# services/document/internal/storage/oss/legacy_snapshot.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/135-legacy-snapshot.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`VerifyLegacyAbsence`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`524e496fa404c7ee4e43437195994ec32c3620b8255aa04b1bd4a20c52fe21de`。[参考实现](../../../../services/document/internal/storage/oss/legacy_snapshot.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
