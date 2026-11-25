# services/document/internal/storage/local/objects_storage.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/130-objects-storage.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`Acquire`、`AcquireRead`、`acquire`、`CleanupUpload`、`Temporary`、`Stage`、`Close`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"strings"
	"sync"
)

// guard.go：保留原实现，按业务合并文件。

// Acquire 非阻塞获取同一资源的跨进程排他锁。调用方必须在 SQL 预检前获取，
// 并覆盖上传、清理（含 Clear）及数据库最终确认；Put/Delete/CleanupUpload 不重复加锁。
// 锁文件永久保留：删除后重建会产生不同 inode，导致两个进程同时认为自己持锁。
// 本适配器要求各进程共享支持文件锁的本地目录；OSS 也复用此目录，仍是单机部署。
func (s *Store) Acquire(ctx context.Context, key string) (func(), error) {
	return s.acquire(ctx, key, true)
}

// 下载之间共享锁，删除/上传仍需排他锁；多个读者不能互相造成假冲突。
func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error) {
	return s.acquire(ctx, key, false)
}

func (s *Store) acquire(ctx context.Context, key string, exclusive bool) (func(), error) {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := ".lock-" + key
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if os.IsExist(err) {
		info, statErr := root.Lstat(name)
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, ErrIntegrity
		}
		file, err = root.OpenFile(name, os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	opened, err := file.Stat()
	current, pathErr := root.Lstat(name)
	if err != nil || pathErr != nil || !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		file.Close()
		return nil, ErrIntegrity
	}
	if err := lockFile(file, exclusive); err != nil {
		file.Close()
		return nil, err
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			unlockFile(file)
			file.Close()
		})
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// CleanupUpload 要求调用方持有 Acquire 锁，仅清理精确属于该键的普通临时文件。
// 不遍历子目录、不跟随符号链接，也不触碰正式文件、锁文件或其他资源的临时文件。
func (s *Store) CleanupUpload(ctx context.Context, key string) error {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return ErrUnavailable
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return ErrUnavailable
	}
	prefix := ".upload-" + key + "-"
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		nonce := strings.TrimPrefix(name, prefix)
		if len(nonce) != 32 || strings.ToLower(nonce) != nonce {
			continue
		}
		if _, err := hex.DecodeString(nonce); err != nil {
			continue
		}
		info, err := root.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return ErrUnavailable
		}
		if !info.Mode().IsRegular() {
			return ErrIntegrity
		}
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return ErrUnavailable
		}
	}
	return nil
}

// temporary.go：保留原实现，按业务合并文件。

// Temporary 是已经完整校验并回到起点的私有临时文件。OSS 使用可 Seek 的文件
// 上传，读取也先落盘校验再输出，避免把半份或摘要错误的云对象交给 HTTP。
// 创建/关闭期间调用者持有对应资源锁；进程退出后的残留沿 CleanupUpload 清理。
type Temporary struct {
	*os.File
	root *os.Root
	name string
	once sync.Once
	err  error
}

func (s *Store) Stage(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) (*Temporary, error) {
	if body == nil || size > 32<<20 || size >= math.MaxInt64 {
		return nil, ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		root.Close()
		return nil, ErrUnavailable
	}
	name := ".upload-" + key + "-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		root.Close()
		return nil, ErrUnavailable
	}
	temporary := &Temporary{File: file, root: root, name: name}
	valid := false
	defer func() {
		if !valid {
			temporary.Close()
		}
	}()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, body}, int64(size)+1))
	if err != nil {
		return nil, readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), digest[:]) {
		return nil, ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if file.Sync() != nil {
		return nil, ErrUnavailable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, ErrUnavailable
	}
	valid = true
	return temporary, nil
}

func (t *Temporary) Close() error {
	t.once.Do(func() {
		opened, statErr := t.File.Stat()
		current, pathErr := t.root.Lstat(t.name)
		if t.File.Close() != nil {
			t.err = ErrUnavailable
		}
		// 不删除被替换成目录/链接或另一个文件的路径。未知清理留待原键重试。
		if statErr != nil || pathErr != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
			t.err = ErrIntegrity
		} else if err := t.root.Remove(t.name); err != nil && !os.IsNotExist(err) {
			t.err = ErrUnavailable
		}
		t.root.Close()
	})
	return t.err
}
````

UTF-8/LF全文SHA-256：`af8f216f61322080f0b0721808007f6a96355ba26181205a288f6e76a27f2d90`。[参考实现](../../../../services/document/internal/storage/local/objects_storage.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
