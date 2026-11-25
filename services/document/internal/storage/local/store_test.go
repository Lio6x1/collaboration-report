package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// 同一资源的失败上传、并发重试及磁盘篡改集中验证不可变存储边界。
func TestImmutableResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	payload := []byte("经过校验的资源正文")
	size, checksum := uint64(len(payload)), sha256.Sum256(payload)
	for _, bad := range [][]byte{payload[:len(payload)-1], bytes.Repeat([]byte("x"), len(payload)), append(append([]byte{}, payload...), 'x')} {
		if err := store.Put(ctx, key, bytes.NewReader(bad), size, checksum); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("错误上传必须被拒绝: %v", err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("失败上传不得留下正式或临时文件: %v, %v", entries, err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := store.Put(ctx, key, bytes.NewReader(payload), size, checksum); err != nil {
				t.Errorf("相同资源并发重试应成功: %v", err)
			}
		}()
	}
	workers.Wait()
	other := []byte("不得覆盖原文件")
	if err := store.Put(ctx, key, bytes.NewReader(other), uint64(len(other)), sha256.Sum256(other)); !errors.Is(err, ErrConflict) {
		t.Fatalf("同键不同内容必须冲突: %v", err)
	}
	reader, err := store.Open(ctx, key, size, checksum)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(actual, payload) {
		t.Fatalf("校验后应从起点读取原文件: %q, %v", actual, err)
	}
	if err := store.Put(ctx, "../"+key, bytes.NewReader(payload), size, checksum); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("路径穿越必须被拒绝: %v", err)
	}
	if err := store.Delete(ctx, "../"+key); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("删除同样不能穿越目录: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, key), bytes.Repeat([]byte("x"), len(payload)), 0600); err != nil {
		t.Fatal(err)
	}
	if reader, err := store.Open(ctx, key, size, checksum); !errors.Is(err, ErrIntegrity) {
		if reader != nil {
			reader.Close()
		}
		t.Fatalf("同长度磁盘篡改也必须在下载前被发现: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := store.Delete(ctx, key); err != nil {
			t.Fatalf("删除应幂等: %v", err)
		}
	}
}
