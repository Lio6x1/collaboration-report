// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package oss

import (
	"collaboration/document/internal/storage/local"
	"context"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"io"
)

// cleanup.go：保留原实现，按业务合并文件。

func (s *Store) Delete(ctx context.Context, key string) error {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return err
	}
	if err := s.requireUnversioned(ctx); err != nil {
		return err
	}
	result, deleteErr := s.client.DeleteObject(ctx, &sdk.DeleteObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if deleteErr == nil && (result == nil || result.DeleteMarker || (result.VersionId != nil && *result.VersionId != "")) {
		return ErrVersioning
	}
	// DELETE 的 204 或连接断开都不能单独结算退款。GET 的 XML 错误码能明确
	// 区分 NoSuchKey 与 NoSuchBucket；HEAD 的无正文 404 不足以做退款证明。
	// 未删除、GET 未知、Bucket 不存在/无权限都让 SQL 保持 DELETING。
	remaining, getErr := s.client.GetObject(ctx, &sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if remaining != nil && remaining.Body != nil {
		remaining.Body.Close()
	}
	if getErr == nil {
		return local.ErrUnavailable
	}
	if err := storageError(ctx, getErr); err != local.ErrNotFound {
		return err
	}
	// 旧本地对象也必须真实删除后才允许退款；失败时保留原任务继续清理。
	return s.local.Delete(ctx, key)
}

// reading.go：保留原实现，按业务合并文件。

func (s *Store) remoteOpen(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if size > 32<<20 {
		return nil, local.ErrInvalidSize
	}
	result, err := s.client.GetObject(ctx, &sdk.GetObjectRequest{Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object)})
	if err != nil {
		return nil, storageError(ctx, err)
	}
	if result == nil || result.Body == nil {
		return nil, local.ErrUnavailable
	}
	defer result.Body.Close()
	if result.ContentLength < 0 || uint64(result.ContentLength) != size ||
		(result.VersionId != nil && *result.VersionId != "") || result.ContentRange != nil {
		return nil, local.ErrIntegrity
	}
	file, err := s.local.Stage(ctx, key, result.Body, size, digest)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Store) Open(ctx context.Context, key string, size uint64, digest [32]byte) (io.ReadCloser, error) {
	file, err := s.remoteOpen(ctx, key, size, digest)
	if err != local.ErrNotFound {
		return file, err
	}
	// 接入云存储前的已验收本地对象留在原目录，仅云端明确 NoSuchKey 才回读。
	// 云端超时、权限错误或损坏不能绕过为本地成功，也不主动搬运旧业务数据。
	legacy, err := s.local.Open(ctx, key, size, digest)
	if err != nil {
		return nil, err
	}
	return legacy, nil
}

// upload.go：保留原实现，按业务合并文件。

func (s *Store) Put(ctx context.Context, key string, body io.Reader, size uint64, digest [32]byte) error {
	object, err := s.objectKey(ctx, key)
	if err != nil {
		return err
	}
	// 先完整检查客户端原流。即使对象已存在，错误的重试正文也不能被冒充成功。
	file, err := s.local.Stage(ctx, key, body, size, digest)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := s.requireUnversioned(ctx); err != nil {
		return err
	}
	existing, err := s.remoteOpen(ctx, key, size, digest)
	if err == nil {
		return existing.Close()
	}
	if err == local.ErrIntegrity {
		return local.ErrConflict
	}
	if err != local.ErrNotFound {
		return err
	}
	_, putErr := s.client.PutObject(ctx, &sdk.PutObjectRequest{
		Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(object), Body: file.File,
		ContentLength: sdk.Ptr(int64(size)), ForbidOverwrite: sdk.Ptr("true"),
		ContentType: sdk.Ptr("application/octet-stream"), Acl: sdk.ObjectACLPrivate,
	})
	// 无论成功、409 还是回复丢失，都核验同一个实际对象的完整长度/SHA。
	// 原 context 已结束时直接保留未知，由业务原号恢复，绝不另起宽期限。
	verified, verifyErr := s.remoteOpen(ctx, key, size, digest)
	if verifyErr == nil {
		return verified.Close()
	}
	if verifyErr == local.ErrIntegrity {
		return local.ErrConflict
	}
	if putErr != nil {
		return storageError(ctx, putErr)
	}
	if verifyErr == local.ErrNotFound {
		return local.ErrUnavailable
	}
	return verifyErr
}
