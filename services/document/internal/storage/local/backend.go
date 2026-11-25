package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"regexp"
)

var legacySegment = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

// VerifyLegacyAbsence 只用于全部写者已退出的备份边界。旧实验元数据可能
// 没有合法业务 key/摘要；先证明目录内确实不存在，不能伪造摘要后当普通对象。
func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error {
	if !legacySegment.MatchString(key) {
		return ErrInvalidKey
	}
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = root.Lstat(key)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrUnavailable
	}
	return ErrIntegrity
}

var backendKey = func() string {
	digest := sha256.Sum256([]byte("RESOURCE_STORAGE_BACKEND_BINDING"))
	return hex.EncodeToString(digest[:])
}()

// BindBackend 把非秘密的对象地址固定在原对象目录。若换 Bucket/prefix 或退回
// 本地模式，已有 SQL key 会失去实际对象；启动时拒绝这类静默切换。
// 初次接入 OSS 可继续读取同一目录的旧本地对象，不搬动或覆盖已有数据。
func (s *Store) BindBackend(ctx context.Context, identity []byte) error {
	if len(identity) == 0 || len(identity) > 4096 {
		return ErrInvalidSize
	}
	release, err := s.Acquire(ctx, backendKey)
	if err != nil {
		return err
	}
	defer release()
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	if current, err := readBackend(root); err == nil {
		if !bytes.Equal(current, identity) {
			return ErrConflict
		}
		return nil
	} else if err != ErrNotFound {
		return err
	}
	digest := sha256.Sum256(identity)
	file, err := s.Stage(ctx, backendKey, bytes.NewReader(identity), uint64(len(identity)), digest)
	if err != nil {
		return err
	}
	defer file.Close()
	// 完整文件写入与 Sync 后再独占发布；崩溃不会留下半份配置绑定。
	if root.Link(file.name, ".oss-binding") != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *Store) RequireLocalBackend(ctx context.Context) error {
	root, err := s.openRoot(ctx, backendKey)
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = readBackend(root)
	if err == ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrConflict
}

func readBackend(root *os.Root) ([]byte, error) {
	info, err := root.Lstat(".oss-binding")
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return nil, ErrIntegrity
	}
	file, err := root.Open(".oss-binding")
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrIntegrity
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(raw) == 0 || len(raw) > 4096 {
		return nil, ErrIntegrity
	}
	return raw, nil
}
