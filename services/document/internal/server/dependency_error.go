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
