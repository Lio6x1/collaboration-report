package rebuildartifact

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 同一夹具覆盖真实链签名及核心准入反例，避免每个辅助函数单独镜像测试。
func TestArtifactIdentityAndOriginalBytes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	intermediate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "intermediate"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediate, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	intermediate, _ = x509.ParseCertificate(intermediateDER)
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	caPath := write("ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	keyPath := write("key.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	leaf := func(role string, expired bool) []byte {
		t.Helper()
		uri, _ := url.Parse("spiffe://collaboration-report/" + role)
		template := &x509.Certificate{SerialNumber: big.NewInt(3), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), URIs: []*url.URL{uri}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		if expired {
			template.NotAfter = now.Add(-time.Minute)
		}
		der, err := x509.CreateCertificate(rand.Reader, template, intermediate, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		return der
	}
	searchDER := leaf("search", false)
	certPath := write("search.pem", append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: searchDER}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER})...))
	signer, err := NewSigner(certPath, keyPath, caPath, "search")
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewVerifier(caPath)
	if err != nil {
		t.Fatal(err)
	}
	// 空白、键顺序都属于原 payload 字节，不能经 JSON 重排后验签。
	payload := []byte("{ \"b\": 2, \"a\": 1 }\n")
	raw, err := signer.Sign(PrepareV1, payload)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifier.Verify(raw, PrepareV1)
	if err != nil || !bytes.Equal(verified.Payload, payload) {
		t.Fatalf("verify: %+v %v", verified, err)
	}
	digest := sha256.Sum256(raw)
	if verified.Reference != "rebuild-prepare-v1:"+hex.EncodeToString(digest[:]) {
		t.Fatal("reference must bind original envelope")
	}
	if verified.Role != "search" || len(verified.SignerFingerprint) != 64 {
		t.Fatal("missing accepted identity")
	}
	// 恢复只认可持久接受时的两个摘要，不因重新签名而改变原内容地址。
	acceptedDigest := sha256.Sum256(raw)
	acceptedSigner := sha256.Sum256(searchDER)
	restored, err := RestoreAccepted(raw, PrepareV1, acceptedDigest, acceptedSigner)
	if err != nil || restored.Reference != verified.Reference || !bytes.Equal(restored.Payload, payload) {
		t.Fatal("accepted artifact not restored")
	}
	wrongSigner := acceptedSigner
	wrongSigner[0] ^= 1
	if _, err := RestoreAccepted(raw, PrepareV1, acceptedDigest, wrongSigner); err == nil {
		t.Fatal("restore accepted a different persisted signer")
	}
	var original envelope
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*envelope)
		want   error
	}{
		{"payload tampering", func(e *envelope) { e.Payload = []byte("changed") }, ErrSignature},
		{"signature tampering", func(e *envelope) { e.Signature = append([]byte{}, e.Signature...); e.Signature[0] ^= 1 }, ErrSignature},
		{"wrong role", func(e *envelope) { e.Certificates = [][]byte{leaf("document", false), intermediateDER} }, ErrIdentity},
		{"expired", func(e *envelope) { e.Certificates = [][]byte{leaf("search", true), intermediateDER} }, ErrIdentity},
		{"missing intermediate", func(e *envelope) { e.Certificates = [][]byte{searchDER} }, ErrIdentity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := original
			tc.mutate(&e)
			changed, _ := json.Marshal(e)
			if _, err := verifier.Verify(changed, PrepareV1); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
	for _, prefix := range []string{`{"purpose":"prepare-v1",`, `{"unknown":1,`} {
		if _, err := verifier.Verify(append([]byte(prefix), raw[1:]...), PrepareV1); !errors.Is(err, ErrEnvelope) {
			t.Fatalf("non-strict JSON accepted: %v", err)
		}
	}
	if _, err := verifier.Verify(raw, SnapshotV1); !errors.Is(err, ErrEnvelope) {
		t.Fatalf("wrong purpose accepted: %v", err)
	}
	if _, err := signer.Sign(SnapshotV1, payload); !errors.Is(err, ErrIdentity) {
		t.Fatalf("wrong signer role accepted: %v", err)
	}
	// 另一信任根不能因为 envelope 携带了叶证书/中间链而被隐式信任。
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherRoot := *root
	otherRoot.PublicKey = &otherKey.PublicKey
	otherDER, err := x509.CreateCertificate(rand.Reader, &otherRoot, &otherRoot, &otherKey.PublicKey, otherKey)
	if err != nil {
		t.Fatal(err)
	}
	otherPath := write("other.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: otherDER}))
	otherVerifier, err := NewVerifier(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherVerifier.Verify(raw, PrepareV1); !errors.Is(err, ErrIdentity) {
		t.Fatalf("wrong CA accepted: %v", err)
	}
	otherKeyPath := write("other-key.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(otherKey)}))
	if _, err := NewSigner(certPath, otherKeyPath, caPath, "search"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("unbound private key accepted: %v", err)
	}
}
