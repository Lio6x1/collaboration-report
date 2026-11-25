// Package rebuildartifact 提供重建 CLI 的离线身份签名。调用方须持久化首次生成的
// 原 envelope；RSA-PSS 带随机盐，重试重签会改变内容地址，不能替代原操作输入。
package rebuildartifact

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"time"
)

const (
	PrepareV1           = "prepare-v1"
	SnapshotV1          = "snapshot-v1"
	MaxPayloadBytes     = 1 << 20
	MaxEnvelopeBytes    = 2 << 20
	maxCertificateBytes = 32 << 10
	maxChainLength      = 8
)

// 固定错误避免将本服务密钥位置、证书内容及底层解析错误传播到 CLI 输出。
var (
	ErrConfig    = errors.New("REBUILD_ARTIFACT_CONFIG_INVALID")
	ErrEnvelope  = errors.New("REBUILD_ARTIFACT_ENVELOPE_INVALID")
	ErrIdentity  = errors.New("REBUILD_ARTIFACT_IDENTITY_INVALID")
	ErrSignature = errors.New("REBUILD_ARTIFACT_SIGNATURE_INVALID")
)

type envelope struct {
	Purpose      string   `json:"purpose"`
	Certificates [][]byte `json:"certificates"`
	Payload      []byte   `json:"payload"`
	Signature    []byte   `json:"signature"`
}

type Signer struct {
	key          *rsa.PrivateKey
	certificates [][]byte
	role         string
	verifier     *Verifier
}

type Verifier struct{ roots *x509.CertPool }

type Verified struct {
	Payload           []byte
	Reference         string
	SignerFingerprint string
	Role              string
}

// NewSigner 只读取显式配置的本服务 PEM 文件；不从制品中选择信任根或私钥。
// certPath 的 PEM 顺序是叶证书在前，其余为中间证书。
func NewSigner(certPath, keyPath, caPath, role string) (*Signer, error) {
	if role != "search" && role != "document" {
		return nil, ErrConfig
	}
	v, err := NewVerifier(caPath)
	if err != nil {
		return nil, err
	}
	certPEM, err := readBounded(certPath)
	if err != nil {
		return nil, ErrConfig
	}
	certs, err := parseCertificates(certPEM)
	if err != nil {
		return nil, ErrConfig
	}
	keyPEM, err := readBounded(keyPath)
	if err != nil {
		return nil, ErrConfig
	}
	block, rest := pem.Decode(keyPEM)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrConfig
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = parsed.(*rsa.PrivateKey)
	default:
		return nil, ErrConfig
	}
	if err != nil || key == nil || key.Validate() != nil {
		return nil, ErrConfig
	}
	pub, _, err := v.identity(certs, role)
	if err != nil {
		return nil, err
	}
	if !pub.Equal(&key.PublicKey) {
		return nil, ErrIdentity
	}
	return &Signer{key: key, certificates: certs, role: role, verifier: v}, nil
}

func NewVerifier(caPath string) (*Verifier, error) {
	data, err := readBounded(caPath)
	if err != nil {
		return nil, ErrConfig
	}
	certs, err := parseCertificates(data)
	if err != nil {
		return nil, ErrConfig
	}
	roots := x509.NewCertPool()
	for _, der := range certs {
		cert, err := x509.ParseCertificate(der)
		if err != nil || !cert.IsCA {
			return nil, ErrConfig
		}
		roots.AddCert(cert)
	}
	return &Verifier{roots: roots}, nil
}

func (s *Signer) Sign(purpose string, payload []byte) ([]byte, error) {
	role := purposeRole(purpose)
	if role == "" || len(payload) > MaxPayloadBytes {
		return nil, ErrEnvelope
	}
	if s == nil || s.role != role {
		return nil, ErrIdentity
	}
	// 长寿命 CLI 也可能跨过证书到期点，不能仅在构造时校验。
	if _, _, err := s.verifier.identity(s.certificates, role); err != nil {
		return nil, err
	}
	digest := signingDigest(purpose, payload)
	sig, err := rsa.SignPSS(rand.Reader, s.key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		return nil, ErrSignature
	}
	result, err := json.Marshal(envelope{Purpose: purpose, Certificates: s.certificates, Payload: append([]byte{}, payload...), Signature: sig})
	if err != nil || len(result) > MaxEnvelopeBytes {
		return nil, ErrEnvelope
	}
	return result, nil
}

// Verify 用当前可信时钟验证首次接受的制品，不接受签名者自报时间。
// 已接受制品的长期恢复应由调用方核对持久化接受事实与原始摘要，而非重新签名。
func (v *Verifier) Verify(raw []byte, purpose string) (Verified, error) {
	role := purposeRole(purpose)
	if role == "" {
		return Verified{}, ErrEnvelope
	}
	e, err := decodeEnvelope(raw)
	if err != nil || e.Purpose != purpose {
		return Verified{}, ErrEnvelope
	}
	pub, fingerprint, err := v.identity(e.Certificates, role)
	if err != nil {
		return Verified{}, err
	}
	if len(e.Signature) != pub.Size() {
		return Verified{}, ErrSignature
	}
	digest := signingDigest(purpose, e.Payload)
	if rsa.VerifyPSS(pub, crypto.SHA256, digest[:], e.Signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) != nil {
		return Verified{}, ErrSignature
	}
	hash := sha256.Sum256(raw)
	return Verified{Payload: e.Payload, Reference: "rebuild-" + purpose + ":" + hex.EncodeToString(hash[:]), SignerFingerprint: fingerprint, Role: role}, nil
}

// RestoreAccepted仅恢复本服务数据库已经通过Verify并持久接受的原字节。
// 两个期望摘要必须来自该原操作的可信持久行，不能来自CLI输入；它不是首次验签接口。
// 原证书过期不改变已接受事实，也不允许重新签名、换证书或换payload续接原任务。
func RestoreAccepted(raw []byte, purpose string, expectedEnvelope, expectedSigner [32]byte) (Verified, error) {
	role := purposeRole(purpose)
	if role == "" || expectedEnvelope == ([32]byte{}) || expectedSigner == ([32]byte{}) ||
		len(raw) > MaxEnvelopeBytes || sha256.Sum256(raw) != expectedEnvelope {
		return Verified{}, ErrEnvelope
	}
	e, err := decodeEnvelope(raw)
	if err != nil || e.Purpose != purpose || len(e.Certificates) == 0 || sha256.Sum256(e.Certificates[0]) != expectedSigner {
		return Verified{}, ErrEnvelope
	}
	return Verified{Payload: e.Payload, Reference: "rebuild-" + purpose + ":" + hex.EncodeToString(expectedEnvelope[:]),
		SignerFingerprint: hex.EncodeToString(expectedSigner[:]), Role: role}, nil
}

func purposeRole(purpose string) string {
	switch purpose {
	case PrepareV1:
		return "search"
	case SnapshotV1:
		return "document"
	}
	return ""
}

func signingDigest(purpose string, payload []byte) [32]byte {
	h := sha256.New()
	// 每一段带长度，避免拼接歧义；固定域隔离 TLS 和其他协议的签名字节。
	for _, part := range [][]byte{[]byte("collaboration-report/rebuild-artifact/v1"), []byte(purpose), payload} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		h.Write(size[:])
		h.Write(part)
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

func (v *Verifier) identity(ders [][]byte, role string) (*rsa.PublicKey, string, error) {
	if v == nil || v.roots == nil || len(ders) == 0 || len(ders) > maxChainLength {
		return nil, "", ErrIdentity
	}
	var leaf *x509.Certificate
	intermediates := x509.NewCertPool()
	for i, der := range ders {
		if len(der) == 0 || len(der) > maxCertificateBytes {
			return nil, "", ErrIdentity
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, "", ErrIdentity
		}
		if i == 0 {
			leaf = cert
		} else {
			intermediates.AddCert(cert)
		}
	}
	pub, ok := leaf.PublicKey.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < 2048 || pub.N.BitLen() > 8192 || leaf.IsCA || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, "", ErrIdentity
	}
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://collaboration-report/"+role {
		return nil, "", ErrIdentity
	}
	// Verify 的 Any EKU 兼容规则不能替代本协议显式 clientAuth 准入。
	clientAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			clientAuth = true
		}
	}
	if !clientAuth {
		return nil, "", ErrIdentity
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: v.roots, Intermediates: intermediates, CurrentTime: time.Now(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return nil, "", ErrIdentity
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	return pub, hex.EncodeToString(fingerprint[:]), nil
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrConfig
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxEnvelopeBytes+1))
	if err != nil || len(data) > MaxEnvelopeBytes {
		return nil, ErrConfig
	}
	return data, nil
}

func parseCertificates(data []byte) ([][]byte, error) {
	var certs [][]byte
	for len(bytes.TrimSpace(data)) > 0 {
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Bytes) > maxCertificateBytes || len(certs) >= maxChainLength {
			return nil, ErrConfig
		}
		certs = append(certs, block.Bytes)
		data = rest
	}
	if len(certs) == 0 {
		return nil, ErrConfig
	}
	return certs, nil
}

func decodeEnvelope(raw []byte) (envelope, error) {
	var e envelope
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytes {
		return e, ErrEnvelope
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return e, ErrEnvelope
	}
	seen := make(map[string]bool, 4)
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return e, ErrEnvelope
		}
		seen[name] = true
		switch name {
		case "purpose":
			err = d.Decode(&e.Purpose)
		case "certificates":
			var values []string
			err = d.Decode(&values)
			if err == nil && (len(values) == 0 || len(values) > maxChainLength) {
				err = ErrEnvelope
			}
			if err == nil {
				for _, value := range values {
					var der []byte
					der, err = decodeBase64(value, maxCertificateBytes)
					if err != nil {
						break
					}
					e.Certificates = append(e.Certificates, der)
				}
			}
		case "payload", "signature":
			var value *string
			err = d.Decode(&value)
			if err == nil && value == nil {
				err = ErrEnvelope
			}
			if err == nil {
				if name == "payload" {
					e.Payload, err = decodeBase64(*value, MaxPayloadBytes)
				} else {
					e.Signature, err = decodeBase64(*value, 1024)
				}
			}
		default:
			return e, ErrEnvelope
		}
		if err != nil {
			return e, ErrEnvelope
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(seen) != 4 {
		return e, ErrEnvelope
	}
	if _, err := d.Token(); err != io.EOF {
		return e, ErrEnvelope
	}
	if e.Payload == nil || len(e.Payload) > MaxPayloadBytes || len(e.Signature) < 256 || len(e.Signature) > 1024 || len(e.Certificates) == 0 || len(e.Certificates) > maxChainLength {
		return e, ErrEnvelope
	}
	return e, nil
}

func decodeBase64(value string, maximum int) ([]byte, error) {
	if len(value) > base64.StdEncoding.EncodedLen(maximum) {
		return nil, ErrEnvelope
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	// 标准解码器容忍换行；制品要求标准唯一表示，故再核对编码结果。
	if err != nil || len(decoded) > maximum || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, ErrEnvelope
	}
	return decoded, nil
}
