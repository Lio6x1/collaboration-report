package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 多个下载可以并行，但最后一个下载释放前，清理不能取得独占锁。
func TestReadGuardsExcludeCleanup(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, key := context.Background(), strings.Repeat("c", 64)
	first, err := store.AcquireRead(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	second, err := store.AcquireRead(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	assertBusy := func() {
		t.Helper()
		release, err := store.Acquire(ctx, key)
		if release != nil {
			release()
		}
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("下载期间清理必须返回忙: %v", err)
		}
	}
	assertBusy()
	first()
	assertBusy()
	second()
	cleanup, err := store.Acquire(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
}

// 两个独立句柄模拟并行实例：忙时不能进入，释放后可以恢复，恢复清理不能误删其他键。
func TestResourceGuardAndOrphanCleanup(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	first, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	key, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	release, err := first.Acquire(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if unexpected, err := second.Acquire(ctx, key); !errors.Is(err, ErrBusy) {
		if unexpected != nil {
			unexpected()
		}
		t.Fatalf("独立句柄同键必须立即返回忙: %v", err)
	}
	orphan := ".upload-" + key + "-" + strings.Repeat("1", 32)
	retained := []string{key, ".upload-" + other + "-" + strings.Repeat("2", 32), ".upload-" + key + "-invalid"}
	for _, name := range append([]string{orphan}, retained...) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("保留边界"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	lockBefore, err := os.Stat(filepath.Join(directory, ".lock-"+key))
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
	recovered, err := second.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("释放后另一实例应能恢复: %v", err)
	}
	defer recovered()
	if err := second.CleanupUpload(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, orphan)); !os.IsNotExist(err) {
		t.Fatalf("当前资源孤儿临时文件应被清除: %v", err)
	}
	for _, name := range retained {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("不得误删 %s: %v", name, err)
		}
	}
	lockAfter, err := os.Stat(filepath.Join(directory, ".lock-"+key))
	if err != nil || !os.SameFile(lockBefore, lockAfter) {
		t.Fatalf("释放与清理必须保留同一锁文件: %v", err)
	}
}
