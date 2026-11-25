# services/document/internal/storage/oss/store.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/137-store.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`Config`、`objectClient`、`Store`、`New`、`newStore`、`validConfig`、`objectKey`、`Acquire`、`AcquireRead`、`CleanupUpload`、`requireUnversioned`、`storageError`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// Package oss 用官方 SDK 实现单机私有资源存储。业务仍以原 SQL key/摘要为权威，
// Bucket 不公开；本机文件锁覆盖对象 IO 和业务层最后的 SQL 确认。
package oss

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type Config struct {
	Region, Endpoint, Bucket, Prefix, CredentialsFile string
}

type objectClient interface {
	PutObject(context.Context, *sdk.PutObjectRequest, ...func(*sdk.Options)) (*sdk.PutObjectResult, error)
	GetObject(context.Context, *sdk.GetObjectRequest, ...func(*sdk.Options)) (*sdk.GetObjectResult, error)
	HeadObject(context.Context, *sdk.HeadObjectRequest, ...func(*sdk.Options)) (*sdk.HeadObjectResult, error)
	DeleteObject(context.Context, *sdk.DeleteObjectRequest, ...func(*sdk.Options)) (*sdk.DeleteObjectResult, error)
	GetBucketVersioning(context.Context, *sdk.GetBucketVersioningRequest, ...func(*sdk.Options)) (*sdk.GetBucketVersioningResult, error)
}

type Store struct {
	local  *local.Store
	client objectClient
	config Config
}

var (
	ErrConfig      = errors.New("RESOURCE_OSS_CONFIG_INVALID")
	ErrCredentials = errors.New("RESOURCE_OSS_CREDENTIALS_UNAVAILABLE")
	ErrVersioning  = errors.New("RESOURCE_OSS_VERSIONING_UNSUPPORTED")
	regionPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)
	bucketPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
	prefixPattern  = regexp.MustCompile(`^(?:[a-zA-Z0-9_-]+/)+$`)
)

func New(root string, cfg Config) (*Store, error) {
	if !validConfig(cfg) {
		return nil, ErrConfig
	}
	secret, err := readCredentials(cfg.CredentialsFile)
	if err != nil {
		return nil, err
	}
	// 不从环境继承 SDK 调试日志；其中可能含对象地址或签名。一次请求未知后
	// 由本适配器核验原键，再由业务原任务重试，不叠加 SDK 内部长时间退避。
	c := sdk.NewConfig().WithRegion(cfg.Region).WithEndpoint(cfg.Endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(secret.ID, secret.Secret, secret.Token)).
		WithRetryMaxAttempts(1)
	c.LogLevel = sdk.Ptr(sdk.LogOff)
	return newStore(root, cfg, sdk.NewClient(c))
}

func newStore(root string, cfg Config, client objectClient) (*Store, error) {
	if !validConfig(cfg) || client == nil {
		return nil, ErrConfig
	}
	files, err := local.New(root)
	if err != nil {
		return nil, err
	}
	// 配置绑定不含凭据路径，允许轮换本机凭据而不改变对象身份。
	identity, _ := json.Marshal(struct{ Region, Endpoint, Bucket, Prefix string }{cfg.Region, cfg.Endpoint, cfg.Bucket, cfg.Prefix})
	if err := files.BindBackend(context.Background(), identity); err != nil {
		return nil, err
	}
	return &Store{local: files, client: client, config: cfg}, nil
}

func validConfig(cfg Config) bool {
	// 仅接受官方 HTTPS 外网 Endpoint，避免把签名凭据发给配置中的任意主机。
	return len(cfg.Region) <= 64 && regionPattern.MatchString(cfg.Region) &&
		cfg.Endpoint == "https://oss-"+cfg.Region+".aliyuncs.com" &&
		bucketPattern.MatchString(cfg.Bucket) && len(cfg.Prefix) <= 160 && prefixPattern.MatchString(cfg.Prefix)
}

func (s *Store) objectKey(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(key) != 64 || strings.ToLower(key) != key {
		return "", local.ErrInvalidKey
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", local.ErrInvalidKey
	}
	return s.config.Prefix + key, nil
}

func (s *Store) Acquire(ctx context.Context, key string) (func(), error) {
	return s.local.Acquire(ctx, key)
}

func (s *Store) AcquireRead(ctx context.Context, key string) (func(), error) {
	return s.local.AcquireRead(ctx, key)
}

func (s *Store) CleanupUpload(ctx context.Context, key string) error {
	return s.local.CleanupUpload(ctx, key)
}

func (s *Store) requireUnversioned(ctx context.Context) error {
	result, err := s.client.GetBucketVersioning(ctx, &sdk.GetBucketVersioningRequest{Bucket: sdk.Ptr(s.config.Bucket)})
	if err != nil {
		return storageError(ctx, err)
	}
	// Suspended 也有历史版本与 delete marker，不能把 HEAD 404 当物理删除证明。
	// 写入/删除前检查；运维不得在服务运行时改变版本设置。
	if result == nil || (result.VersionStatus != nil && *result.VersionStatus != "") {
		return ErrVersioning
	}
	return nil
}

func storageError(ctx context.Context, err error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var service *sdk.ServiceError
	if errors.As(err, &service) && service.StatusCode == 404 && service.Code == "NoSuchKey" {
		return local.ErrNotFound
	}
	// AccessDenied/NoSuchBucket/连接失败不伪装成不存在，也不输出 SDK 原始错误。
	return local.ErrUnavailable
}
````

UTF-8/LF全文SHA-256：`88fd113e2069f39bbe04ee538991cedcde6dfe254b6530c5603e8bc7f4bc2aa6`。[参考实现](../../../../services/document/internal/storage/oss/store.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
