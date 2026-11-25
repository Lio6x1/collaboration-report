# services/document/internal/server/dependency_error.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/269-dependency-error.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`dependencyError`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package server

import (
	"context"
	"errors"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var publicRPCCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,95}$`)

// dependencyError 是传输边界，不改变领域拒绝、原操作状态或关闭 ACK。
// SQL/网络错误可能带连接和请求细节，只允许固定协议码离开服务。
func dependencyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "REQUEST_CANCELLED")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "DEPENDENCY_TIMEOUT")
	}
	var encoded interface{ GRPCStatus() *status.Status }
	if errors.As(err, &encoded) {
		original := encoded.GRPCStatus()
		if original != nil && original.Code() != codes.Unknown && original.Code() != codes.OK {
			// 从底层 status 取码，避免 fmt 包装层把私有前缀附到拒绝原因上。
			if publicRPCCode.MatchString(original.Message()) {
				return original.Err()
			}
			if original.Code() != codes.Unavailable {
				return status.Error(original.Code(), "DEPENDENCY_FAILURE")
			}
		}
	}
	return status.Error(codes.Unavailable, "SERVICE_UNAVAILABLE")
}
````

UTF-8/LF全文SHA-256：`7c370b48032b4338db2113688d03acd3ecfb7c6d22f060a899882ce090a3a06f`。[参考实现](../../../../services/document/internal/server/dependency_error.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
