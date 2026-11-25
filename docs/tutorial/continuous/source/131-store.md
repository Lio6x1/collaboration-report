# services/document/internal/storage/local/store.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/131-store.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`Store`、`New`、`checkDirectories`、`validKey`、`openRoot`、`Put`、`Open`、`verifiedOpen`、`Delete`、`contextReader`、`Read`、`readError`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// Package local 实现可由对象存储替换的不可变资源存储；权限与配额由调用方负责。
package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
)

// 固定错误不包含主机路径，业务层可以稳定映射为对外错误码。
var (
	ErrInvalidKey  = errors.New("RESOURCE_STORAGE_INVALID_KEY")
	ErrInvalidSize = errors.New("RESOURCE_STORAGE_INVALID_SIZE")
	ErrIntegrity   = errors.New("RESOURCE_STORAGE_INTEGRITY")
	ErrConflict    = errors.New("RESOURCE_STORAGE_CONFLICT")
	ErrNotFound    = errors.New("RESOURCE_STORAGE_NOT_FOUND")
	ErrUnavailable = errors.New("RESOURCE_STORAGE_UNAVAILABLE")
	ErrBusy        = errors.New("RESOURCE_STORAGE_BUSY")
)

type Store struct {
	path     string
	identity os.FileInfo
}

// New 仅接受专用绝对目录；拒绝任何现有符号链接祖先，避免配置绕过存储边界。
func New(root string) (*Store, error) {
	if !filepath.IsAbs(root) {
		return nil, ErrUnavailable
	}
	root = filepath.Clean(root)
	if err := checkDirectories(root); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, ErrUnavailable
	}
	if err := checkDirectories(root); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &Store{path: root, identity: info}, nil
}

func checkDirectories(path string) error {
	for {
		info, err := os.Lstat(path)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return ErrUnavailable
		}
		if err != nil && !os.IsNotExist(err) {
			return ErrUnavailable
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func validKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for _, c := range key {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (s *Store) openRoot(ctx context.Context, key string) (*os.Root, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validKey(key) {
		return nil, ErrInvalidKey
	}
	if err := checkDirectories(s.path); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return nil, ErrUnavailable
	}
	info, err := root.Stat(".")
	if err != nil || !os.SameFile(s.identity, info) {
		root.Close()
		return nil, ErrUnavailable
	}
	return root, nil
}

// Put 先流式验证并同步临时文件，再排他发布；同键重试必须内容完全相同。
// 生产调用方必须先 Acquire，并持有到数据库确认结束，避免清理与上传交错。
func (s *Store) Put(ctx context.Context, key string, input io.Reader, size uint64, checksum [32]byte) error {
	if size >= math.MaxInt64 || input == nil {
		return ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(key); err == nil {
		// 原PUT可能已成功但响应丢失；先核对不可变对象，再校验重放正文，
		// 不为同一预留额另写完整临时副本。生产调用方持Acquire锁覆盖此路径。
		existing, err := verifiedOpen(ctx, root, key, size, checksum)
		if errors.Is(err, ErrIntegrity) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if err := existing.Close(); err != nil {
			return ErrUnavailable
		}
		hash := sha256.New()
		count, err := io.Copy(hash, io.LimitReader(contextReader{ctx, input}, int64(size)+1))
		if err != nil {
			return readError(ctx)
		}
		if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
			return ErrIntegrity
		}
		return ctx.Err()
	} else if !os.IsNotExist(err) {
		return ErrUnavailable
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ErrUnavailable
	}
	temporary := ".upload-" + key + "-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrUnavailable
	}
	defer root.Remove(temporary)
	defer file.Close()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, input}, int64(size)+1))
	if err != nil {
		return readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
		return ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return ErrUnavailable
	}
	if err := file.Close(); err != nil {
		return ErrUnavailable
	}
	// Rename 在部分系统会覆盖旧文件；硬链接创建在 Windows/Unix 都保留排他语义。
	if err := root.Link(temporary, key); err == nil {
		return nil
	}
	existing, err := verifiedOpen(ctx, root, key, size, checksum)
	if err != nil {
		if errors.Is(err, ErrIntegrity) {
			return ErrConflict
		}
		if errors.Is(err, ErrNotFound) {
			return ErrUnavailable
		}
		return err
	}
	if err := existing.Close(); err != nil {
		return ErrUnavailable
	}
	return nil
}

// Open 返回已校验且回到起点的文件句柄，不将完整资源载入内存。
func (s *Store) Open(ctx context.Context, key string, size uint64, checksum [32]byte) (io.ReadCloser, error) {
	if size >= math.MaxInt64 {
		return nil, ErrInvalidSize
	}
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return verifiedOpen(ctx, root, key, size, checksum)
}

func verifiedOpen(ctx context.Context, root *os.Root, key string, size uint64, checksum [32]byte) (*os.File, error) {
	info, err := root.Lstat(key)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return nil, ErrIntegrity
	}
	file, err := root.Open(key)
	if err != nil {
		return nil, ErrUnavailable
	}
	valid := false
	defer func() {
		if !valid {
			file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() < 0 || uint64(opened.Size()) != size {
		return nil, ErrIntegrity
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(contextReader{ctx, file}, int64(size)+1))
	if err != nil {
		return nil, readError(ctx)
	}
	if uint64(count) != size || !bytes.Equal(hash.Sum(nil), checksum[:]) {
		return nil, ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, ErrUnavailable
	}
	valid = true
	return file, nil
}

// Delete 只删除精确的普通文件名，缺失视为幂等成功，绝不递归删除目录。
// 生产调用方必须持有 Acquire 返回的锁，覆盖数据库预检、删除与最终确认。
func (s *Store) Delete(ctx context.Context, key string) error {
	root, err := s.openRoot(ctx, key)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(key)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Remove(key); err != nil && !os.IsNotExist(err) {
		return ErrUnavailable
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

func readError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrUnavailable
}
````

UTF-8/LF全文SHA-256：`fecb3fc3ab4dd20b0d27c1defcc46e950473ba8bf2bcdb7eba205dd65796f937`。[参考实现](../../../../services/document/internal/storage/local/store.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
