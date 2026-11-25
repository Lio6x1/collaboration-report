package oss

import (
	"context"
	"errors"

	"collaboration/document/internal/storage/local"
	sdk "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

// VerifyLegacyAbsence 不放宽业务 key 规则。仅在停止全部写者的冷备份中，
// 证明旧 metadata-only key 在本地和原云前缀均明确不存在；权限/网络未知失败。
func (s *Store) VerifyLegacyAbsence(ctx context.Context, key string) error {
	if err := s.local.VerifyLegacyAbsence(ctx, key); err != nil {
		return err
	}
	result, err := s.client.GetObject(ctx, &sdk.GetObjectRequest{
		Bucket: sdk.Ptr(s.config.Bucket), Key: sdk.Ptr(s.config.Prefix + key),
	})
	if err != nil {
		if errors.Is(storageError(ctx, err), local.ErrNotFound) {
			return nil
		}
		return local.ErrUnavailable
	}
	if result != nil && result.Body != nil {
		result.Body.Close()
	}
	return local.ErrIntegrity
}
