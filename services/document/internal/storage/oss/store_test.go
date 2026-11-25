package oss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/crc64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// HTTP 边界仍经过官方 SDK 的真实签名/序列化/错误解码。只代替外部网络，
// 验证新适配器必须保证的未知结果和完整性，不冒充实际阿里云验收。
type objectHTTP struct {
	t                                                    *testing.T
	objects                                              map[string][]byte
	version                                              string
	putLost, deleteLost, deleteRetained, getLostAfterPut bool
	putCount, deleteCount                                int
}

func (h *objectHTTP) Do(request *http.Request) (*http.Response, error) {
	h.t.Helper()
	if request.URL.Host != "collaboration-report.oss-cn-shenzhen.aliyuncs.com" ||
		!strings.HasPrefix(request.Header.Get("Authorization"), "OSS4-HMAC-SHA256 ") {
		h.t.Fatal("SDK host or V4 signing missing")
	}
	if _, ok := request.URL.Query()["versioning"]; ok {
		return response(request, 200, []byte("<VersioningConfiguration><Status>"+h.version+"</Status></VersioningConfiguration>")), nil
	}
	key := strings.TrimPrefix(request.URL.Path, "/")
	if !strings.HasPrefix(key, "collabreport-tests/unit/") {
		h.t.Fatal("request escaped independent prefix")
	}
	switch request.Method {
	case http.MethodPut:
		if request.Header.Get("X-Oss-Forbid-Overwrite") != "true" || request.Header.Get("X-Oss-Object-Acl") != "private" {
			h.t.Fatal("immutable/private upload headers missing")
		}
		h.putCount++
		if _, exists := h.objects[key]; exists {
			return errorResponse(request, 409, "FileAlreadyExists"), nil
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		h.objects[key] = body
		if h.putLost {
			h.putLost = false
			return nil, errors.New("injected response loss after object commit")
		}
		result := response(request, 200, nil)
		result.Header.Set("X-Oss-Hash-Crc64ecma", strconv.FormatUint(crc64.Checksum(body, crc64.MakeTable(crc64.ECMA)), 10))
		return result, nil
	case http.MethodGet:
		if h.getLostAfterPut && h.putCount > 0 {
			return nil, errors.New("injected unavailable verification")
		}
		body, exists := h.objects[key]
		if !exists {
			return errorResponse(request, 404, "NoSuchKey"), nil
		}
		return response(request, 200, body), nil
	case http.MethodDelete:
		h.deleteCount++
		if !h.deleteRetained {
			delete(h.objects, key)
		}
		if h.deleteLost {
			h.deleteLost = false
			return nil, errors.New("injected unknown delete")
		}
		return response(request, 204, nil), nil
	default:
		h.t.Fatal("unexpected SDK method")
		return nil, errors.New("unexpected method")
	}
}

func response(request *http.Request, status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Length": {strconv.Itoa(len(body))}},
		ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Request: request}
}

func errorResponse(request *http.Request, status int, code string) *http.Response {
	return response(request, status, []byte("<Error><Code>"+code+"</Code><Message>private diagnostic</Message><RequestId>test</RequestId></Error>"))
}

func fixture(t *testing.T, root string) (*Store, *objectHTTP, string, []byte, [32]byte) {
	t.Helper()
	h := &objectHTTP{t: t, objects: make(map[string][]byte)}
	cfg := Config{Region: "cn-shenzhen", Endpoint: "https://oss-cn-shenzhen.aliyuncs.com", Bucket: "collaboration-report", Prefix: "collabreport-tests/unit/"}
	c := sdk.NewConfig().WithRegion(cfg.Region).WithEndpoint(cfg.Endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider("unit-access-key-id", "unit-secret-for-signing")).WithRetryMaxAttempts(1)
	c.HttpClient = h
	c.LogLevel = sdk.Ptr(sdk.LogOff)
	s, err := newStore(root, cfg, sdk.NewClient(c))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("完整资源正文与原键恢复")
	digest := sha256.Sum256(body)
	return s, h, hex.EncodeToString(digest[:]), body, digest
}

func TestOfficialSDKObjectRecovery(t *testing.T) {
	t.Run("PUT reply loss verifies original object and same key never overwrites", func(t *testing.T) {
		root := t.TempDir()
		s, h, key, body, digest := fixture(t, root)
		ctx := context.Background()
		release, err := s.Acquire(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		h.putLost = true
		if err := s.Put(ctx, key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
			t.Fatal(err)
		}
		if err := s.Put(ctx, key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
			t.Fatal(err)
		}
		if h.putCount != 1 {
			t.Fatal("original object was uploaded again")
		}
		changed := []byte("另一份内容")
		if err := s.Put(ctx, key, bytes.NewReader(changed), uint64(len(changed)), sha256.Sum256(changed)); err != local.ErrConflict {
			t.Fatalf("changed object: %v", err)
		}
		if err := s.Put(ctx, key, bytes.NewReader(changed), uint64(len(body)), digest); err != local.ErrIntegrity {
			t.Fatalf("invalid incoming retry: %v", err)
		}
		reader, err := s.Open(ctx, key, uint64(len(body)), digest)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := io.ReadAll(reader)
		if err != nil || !bytes.Equal(actual, body) {
			t.Fatal("verified read changed bytes")
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		assertNoTemporary(t, root)
	})
	t.Run("unavailable PUT verification retains unknown and original retry recovers", func(t *testing.T) {
		s, h, key, body, digest := fixture(t, t.TempDir())
		h.getLostAfterPut = true
		if err := s.Put(context.Background(), key, bytes.NewReader(body), uint64(len(body)), digest); err != local.ErrUnavailable {
			t.Fatalf("unknown was accepted: %v", err)
		}
		h.getLostAfterPut = false
		if err := s.Put(context.Background(), key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
			t.Fatal(err)
		}
		if h.putCount != 1 {
			t.Fatal("recovery used a different object/write")
		}
	})
	t.Run("corrupt object fails before any output", func(t *testing.T) {
		root := t.TempDir()
		s, h, key, body, digest := fixture(t, root)
		corrupt := bytes.Clone(body)
		corrupt[0] ^= 1
		h.objects[s.config.Prefix+key] = corrupt
		reader, err := s.Open(context.Background(), key, uint64(len(body)), digest)
		if reader != nil || err != local.ErrIntegrity {
			t.Fatalf("corrupt output exposed: %v", err)
		}
		assertNoTemporary(t, root)
	})
	t.Run("DELETE unknown is success only after original object is absent", func(t *testing.T) {
		s, h, key, body, _ := fixture(t, t.TempDir())
		h.objects[s.config.Prefix+key] = body
		h.deleteLost, h.deleteRetained = true, true
		if err := s.Delete(context.Background(), key); err != local.ErrUnavailable {
			t.Fatalf("remaining object accepted: %v", err)
		}
		h.deleteLost, h.deleteRetained = true, false
		if err := s.Delete(context.Background(), key); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(context.Background(), key); err != nil {
			t.Fatal(err)
		}
		if len(h.objects) != 0 {
			t.Fatal("object retained after proof")
		}
	})
	t.Run("versioned bucket cannot mutate and non object 404 is not absence", func(t *testing.T) {
		s, h, key, body, digest := fixture(t, t.TempDir())
		h.version = "Suspended"
		if err := s.Put(context.Background(), key, bytes.NewReader(body), uint64(len(body)), digest); err != ErrVersioning {
			t.Fatalf("versioned PUT: %v", err)
		}
		if err := s.Delete(context.Background(), key); err != ErrVersioning {
			t.Fatalf("versioned DELETE: %v", err)
		}
		if h.putCount != 0 || h.deleteCount != 0 {
			t.Fatal("versioned bucket mutated")
		}
		if storageError(context.Background(), &sdk.ServiceError{StatusCode: 404, Code: "NoSuchBucket"}) != local.ErrUnavailable {
			t.Fatal("bucket absence treated as object absence")
		}
	})
}

func TestLegacyObjectsAndBackendBinding(t *testing.T) {
	root := t.TempDir()
	legacy, err := local.New(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("原本机对象")
	digest := sha256.Sum256(body)
	key := hex.EncodeToString(digest[:])
	if err := legacy.Put(context.Background(), key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	s, _, _, _, _ := fixture(t, root)
	reader, err := s.Open(context.Background(), key, uint64(len(body)), digest)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("old object lost")
	}
	if err := legacy.RequireLocalBackend(context.Background()); err != local.ErrConflict {
		t.Fatal("silent local fallback accepted")
	}
	changed := s.config
	changed.Prefix = "collabreport-tests/other/"
	if _, err := newStore(root, changed, s.client); err != local.ErrConflict {
		t.Fatalf("changed backend accepted: %v", err)
	}
	release, err := s.AcquireRead(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := legacy.Acquire(context.Background(), key); err != local.ErrBusy {
		if other != nil {
			other()
		}
		t.Fatalf("legacy and OSS lock diverged: %v", err)
	}
	release()
	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Open(context.Background(), key, uint64(len(body)), digest); err != local.ErrNotFound {
		t.Fatal("legacy physical object not removed")
	}
}

func TestProtectedCredentialEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "本机 凭据.json")
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"accessKeyId":"unit-access-key-id","accessKeySecret":"unit-secret-for-signing","securityToken":""}`)...)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCredentials(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"accessKeyId":"请填写","accessKeySecret":"私密值"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCredentials(path); err != ErrCredentials {
		t.Fatal("placeholder credentials accepted")
	}
}

func assertNoTemporary(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".upload-") {
			t.Fatal("temporary file leaked")
		}
	}
}
