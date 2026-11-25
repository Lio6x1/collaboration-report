# services/document/internal/svc/resources_outputs_wiring.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/278-resources-outputs-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewExports`、`NewFences`、`NewOutputs`、`NewPublicReading`、`NewPublications`、`NewResources`、`newResourceFiles`、`resourceFiles`、`Put`、`Open`、`Acquire`、`AcquireRead`、`CleanupUpload`、`Delete`、`resourceFileError`、`NewSearchPages`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/exporter"
	logic "collaboration/document/internal/logic/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/document/internal/storage/local"
	ossstore "collaboration/document/internal/storage/oss"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// exports.go：保留原实现，按业务合并文件。

func NewExports(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, public *logic.PublicReading, outputs *logic.Outputs, resources *logic.Resources) (*logic.Exports, error) {
	c := platform.ExportConfig{Root: filepath.Join(cfg.Root, ".runtime", "exports", "document")}
	if cfg.Exports != nil {
		c = *cfg.Exports
	}
	if c.Root == "" {
		c.Root = filepath.Join(cfg.Root, ".runtime", "exports", "document")
	}
	// 开发环境可发现PATH中的转换器；生产和不同Windows安装位置可以显式配置。
	var err error
	if c.Pandoc == "" {
		c.Pandoc, err = exec.LookPath("pandoc")
		if err != nil {
			return nil, err
		}
	}
	if c.Node == "" {
		c.Node, err = exec.LookPath("node")
		if err != nil {
			return nil, err
		}
	}
	if c.Browser == "" && runtime.GOOS == "windows" {
		c.Browser = "msedge"
	}
	renderer, err := exporter.New(exporter.Config{Root: c.Root, Pandoc: c.Pandoc, Node: c.Node, PDFScript: filepath.Join(cfg.Root, "scripts", "export-pdf.mjs"), Browser: c.Browser})
	if err != nil {
		return nil, err
	}
	return logic.NewExports(store.DB, identity, access, public, outputs, resources, renderer, c.Workers)
}

// fence.go：保留原实现，按业务合并文件。

func NewFences(store *platform.Store, cfg platform.Config, outputs *logic.Outputs) *logic.Fences {
	return logic.NewFences(store.DB, repository.NewFences(store.DB), outputs, client.NewCollaborationFence(cfg))
}

// output.go：保留原实现，按业务合并文件。

// 启动恢复使用同一私有池和既定运行归属，不创建新的数据库或替换退出证明。
func NewOutputs(store *platform.Store, cfg platform.Config) (*logic.Outputs, error) {
	owner := os.Getenv("COLLAB_RUN_OWNER")
	if owner == "" {
		return nil, errors.New("OUTPUT_OWNER_REQUIRED")
	}
	digest := sha256.Sum256([]byte("document\n" + owner))
	httpClient, err := platform.HTTPPeer(cfg, "edge")
	if err != nil {
		return nil, err
	}
	self := domain.OutputInstance{ID: owner + "-document", PID: uint64(os.Getpid()), Boot: hex.EncodeToString(digest[:])}
	manager := logic.NewOutputs(store.DB, repository.NewOutputs(store.DB), self, client.NewEdgeOutput(cfg.Peers["edge"], httpClient), client.NewProcessOwner(cfg.Root))
	if err := manager.Initialize(store.Context); err != nil {
		slog.Warn("output_recovery_blocked", "code", logic.OutputRecoveryCode(err))
		return nil, err
	}
	return manager, nil
}

// public_reading.go：保留原实现，按业务合并文件。

func NewPublicReading(store *platform.Store, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, pagination *logic.Pagination) (*logic.PublicReading, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewPublicReading(store.DB, identity, access, outputs, pagination, validator), nil
}

// publication.go：保留原实现，按业务合并文件。

func NewPublications(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Publications, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewPublications(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// resources.go：保留原实现，按业务合并文件。

func NewResources(store *platform.Store, cfg platform.Config, access *logic.Access) (*logic.Resources, error) {
	// 开发默认值集中在装配层；生产可显式配置目录与预算，不让仓储隐含容量。
	c := platform.ResourceConfig{Root: filepath.Join(cfg.Root, ".runtime", "resources", "document"), SpaceBytes: 1 << 30, UploadBytes: 32 << 20, TemporaryBytes: 256 << 20, UploadTTLSeconds: 3600}
	if cfg.Resources != nil {
		c = *cfg.Resources
	}
	if c.SpaceBytes == 0 || c.UploadBytes == 0 || c.UploadBytes > 32<<20 || c.TemporaryBytes < c.UploadBytes || c.UploadTTLSeconds < 60 || c.UploadTTLSeconds > 86400 {
		return nil, errors.New("RESOURCE_CONFIG_INVALID")
	}
	files, err := newResourceFiles(c)
	if err != nil {
		return nil, err
	}
	return logic.NewResources(store.DB, access, logic.ResourceLimits{SpaceBytes: c.SpaceBytes, UploadBytes: c.UploadBytes, TemporaryBytes: c.TemporaryBytes, UploadTTL: time.Duration(c.UploadTTLSeconds) * time.Second}, cfg.Origin, resourceFiles{files}), nil
}

func newResourceFiles(c platform.ResourceConfig) (logic.ResourceFiles, error) {
	if c.OSS != nil {
		return ossstore.New(c.Root, ossstore.Config{Region: c.OSS.Region, Endpoint: c.OSS.Endpoint,
			Bucket: c.OSS.Bucket, Prefix: c.OSS.Prefix, CredentialsFile: c.OSS.CredentialsFile})
	}
	files, err := local.New(c.Root)
	if err != nil {
		return nil, err
	}
	if err := files.RequireLocalBackend(context.Background()); err != nil {
		return nil, err
	}
	return files, nil
}

// 适配器在装配边界把存储校验失败映射为业务错误，不让业务依赖本地文件实现。
type resourceFiles struct{ backend logic.ResourceFiles }

func (f resourceFiles) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error {
	return resourceFileError(f.backend.Put(ctx, key, body, size, digest))
}
func (f resourceFiles) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	file, err := f.backend.Open(ctx, key, size, digest)
	return file, resourceFileError(err)
}
func (f resourceFiles) Acquire(ctx context.Context, key string) (func(), error) {
	release, err := f.backend.Acquire(ctx, key)
	return release, resourceFileError(err)
}
func (f resourceFiles) AcquireRead(ctx context.Context, key string) (func(), error) {
	release, err := f.backend.AcquireRead(ctx, key)
	return release, resourceFileError(err)
}

func (f resourceFiles) CleanupUpload(ctx context.Context, key string) error {
	return resourceFileError(f.backend.CleanupUpload(ctx, key))
}

func (f resourceFiles) Delete(ctx context.Context, key string) error {
	return resourceFileError(f.backend.Delete(ctx, key))
}
func resourceFileError(err error) error {
	switch {
	case errors.Is(err, local.ErrIntegrity), errors.Is(err, local.ErrConflict):
		return domain.Violation(domain.StateConflict, "RESOURCE_CHECKSUM_MISMATCH")
	case errors.Is(err, local.ErrNotFound):
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_INCOMPLETE")
	case errors.Is(err, local.ErrBusy):
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_BUSY")
	default:
		return err
	}
}

// search.go：保留原实现，按业务合并文件。

// Search连接由bootstrap建立及释放，此处只装配Doc私有依赖；不引入Search库/ES凭据。
// 沿现有本地私钥作为master来源，独立用途派生且不回退到空key或固定开发常量。
func NewSearchPages(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs, peer pb.SearchInternalClient) (*logic.SearchPages, error) {
	raw, err := os.ReadFile(cfg.KeyFile)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("SEARCH_PAGE_KEY_UNAVAILABLE")
	}
	defer clear(raw)
	input := append([]byte("collaboration/document/search-page-master/v1\x00"), raw...)
	key := sha256.Sum256(input)
	clear(input)
	defer clear(key[:])
	cursors, err := domain.NewSearchCursorCodec(key[:])
	if err != nil {
		return nil, err
	}
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewSearchPages(store.DB, identity, access, outputs, client.NewSearch(peer), cursors, validator), nil
}
````

UTF-8/LF全文SHA-256：`0e0966cade67751b4aaa18fedcaed3c072d665e60b187671e036e67792036572`。[参考实现](../../../../services/document/internal/svc/resources_outputs_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
