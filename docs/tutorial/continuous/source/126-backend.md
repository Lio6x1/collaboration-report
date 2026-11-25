# services/document/internal/storage/local/backend.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/126-backend.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`VerifyLegacyAbsence`、`BindBackend`、`RequireLocalBackend`、`readBackend`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`392cff50ba8ec4f944f02bf07b56a4a0c73397c0d053e6127e63530115419788`。[参考实现](../../../../services/document/internal/storage/local/backend.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
