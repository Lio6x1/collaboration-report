//go:build osscloud

package oss

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

// 只丢弃官方 SDK 已返回的成功回复；对象 PUT/DELETE 都真实访问阿里云。
// 不伪造 Bucket 状态或对象内容，业务恢复仍重新读取同一 key。
type lostCloudReply struct {
	objectClient
	put, delete bool
}

func (c *lostCloudReply) PutObject(ctx context.Context, request *sdk.PutObjectRequest, options ...func(*sdk.Options)) (*sdk.PutObjectResult, error) {
	result, err := c.objectClient.PutObject(ctx, request, options...)
	if err == nil && c.put {
		c.put = false
		return nil, errors.New("OSS_TEST_PUT_REPLY_LOST")
	}
	return result, err
}

func (c *lostCloudReply) DeleteObject(ctx context.Context, request *sdk.DeleteObjectRequest, options ...func(*sdk.Options)) (*sdk.DeleteObjectResult, error) {
	result, err := c.objectClient.DeleteObject(ctx, request, options...)
	if err == nil && c.delete {
		c.delete = false
		return nil, errors.New("OSS_TEST_DELETE_REPLY_LOST")
	}
	return result, err
}

func TestCloudOriginalObjectRecovery(t *testing.T) {
	settings := os.Getenv("COLLAB_OSS_SETTINGS")
	if !filepath.IsAbs(settings) {
		t.Fatal("OSS_CLOUD_SETTINGS_REQUIRED")
	}
	raw, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal("OSS_CLOUD_SETTINGS_UNAVAILABLE")
	}
	var cfg Config
	if json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), &cfg) != nil ||
		!strings.HasPrefix(cfg.Prefix, "collabreport-tests/") {
		t.Fatal("OSS_CLOUD_TEST_PREFIX_REQUIRED")
	}
	s, err := New(t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	lost := &lostCloudReply{objectClient: s.client, put: true, delete: true}
	s.client = lost
	var identity [32]byte
	if _, err := rand.Read(identity[:]); err != nil {
		t.Fatal("OSS_CLOUD_TEST_RANDOM_FAILED")
	}
	key := hex.EncodeToString(identity[:])
	body := append([]byte("真实 OSS 原键完整校验\n"), identity[:]...)
	digest := sha256.Sum256(body)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	release, err := s.Acquire(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// 失败也只清理本轮随机原 key；任何清理失败保留明确证据，不删整个前缀。
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.Delete(cleanup, key); err != nil {
			t.Error("OSS_CLOUD_ORIGINAL_KEY_CLEANUP_FAILED", err)
		}
	}()
	if err := s.Put(ctx, key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	if lost.put {
		t.Fatal("OSS_CLOUD_REAL_PUT_NOT_OBSERVED")
	}
	reader, err := s.Open(ctx, key, uint64(len(body)), digest)
	if err != nil {
		t.Fatal(err)
	}
	actual, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(actual, body) {
		t.Fatal("OSS_CLOUD_READ_CHANGED")
	}
	if err := s.Put(ctx, key, bytes.NewReader(body), uint64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	changed := []byte("同名不同正文")
	if err := s.Put(ctx, key, bytes.NewReader(changed), uint64(len(changed)), sha256.Sum256(changed)); err != local.ErrConflict {
		t.Fatal("OSS_CLOUD_OVERWRITE_NOT_REJECTED")
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if lost.delete {
		t.Fatal("OSS_CLOUD_REAL_DELETE_NOT_OBSERVED")
	}
	if reader, err := s.Open(ctx, key, uint64(len(body)), digest); err != local.ErrNotFound || reader != nil {
		t.Fatal("OSS_CLOUD_OBJECT_NOT_REMOVED")
	}
	t.Log("OSS_CLOUD_ORIGINAL_PUT_GET_DELETE_RECOVERY_PASS")
}
