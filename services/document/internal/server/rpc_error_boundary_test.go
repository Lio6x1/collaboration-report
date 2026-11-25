package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	domain "collaboration/document/internal/domain/document"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 使用无敏感信息的独立sentinel证明传输边界；不为了复现泄露而打印真实DSN或驱动错误。
// 本单元只验证适配器，不替代实际票据/UPDATE/资源保护及完整关闭矩阵。
func TestRPCDependencyErrorsUseFixedPublicStatus(t *testing.T) {
	for _, adapter := range []struct {
		name     string
		mapError func(error) error
	}{
		{"editing", EditingError},
		{"protection", ProtectionError},
	} {
		t.Run(adapter.name, func(t *testing.T) {
			if adapter.mapError(nil) != nil {
				t.Fatal("nil must remain successful")
			}
			for _, dependency := range []error{
				errors.New("UNIT_PRIVATE_DEPENDENCY_DETAIL"),
				status.Error(codes.Unknown, "UNIT_PRIVATE_DEPENDENCY_DETAIL"),
			} {
				mapped := status.Convert(adapter.mapError(dependency))
				if mapped.Code() != codes.Unavailable || mapped.Message() != "SERVICE_UNAVAILABLE" {
					t.Fatal("ordinary dependency detail crossed the public RPC boundary")
				}
			}
			for _, deadline := range []struct {
				err     error
				code    codes.Code
				message string
			}{
				{context.Canceled, codes.Canceled, "REQUEST_CANCELLED"},
				{context.DeadlineExceeded, codes.DeadlineExceeded, "DEPENDENCY_TIMEOUT"},
			} {
				mapped := status.Convert(adapter.mapError(fmt.Errorf("UNIT_PRIVATE_PREFIX: %w", deadline.err)))
				if mapped.Code() != deadline.code || mapped.Message() != deadline.message {
					t.Fatal("context cancellation/deadline must preserve its fixed status")
				}
			}
			encoded := status.Error(codes.PermissionDenied, "CURRENT_CAPABILITY_REQUIRED")
			for _, dependency := range []error{encoded, fmt.Errorf("UNIT_PRIVATE_PREFIX: %w", encoded)} {
				mapped := status.Convert(adapter.mapError(dependency))
				if mapped.Code() != codes.PermissionDenied || mapped.Message() != "CURRENT_CAPABILITY_REQUIRED" {
					t.Fatal("encoded denial must remain denial without a private wrapping prefix")
				}
			}
		})
	}
}

func TestRPCDomainDenialsKeepTheirBusinessCode(t *testing.T) {
	editing := status.Convert(EditingError(domain.RuleViolation{Kind: domain.PermissionDenied, Code: "CURRENT_EDIT_REQUIRED"}))
	if editing.Code() != codes.PermissionDenied || editing.Message() != "CURRENT_EDIT_REQUIRED" {
		t.Fatal("editing domain denial lost its business code")
	}
	protection := status.Convert(ProtectionError(domain.ProtectionFailure{Kind: domain.ProtectionForbidden, Code: "CURRENT_RESOURCE_REQUIRED"}))
	if protection.Code() != codes.PermissionDenied || protection.Message() != "CURRENT_RESOURCE_REQUIRED" {
		t.Fatal("resource domain denial lost its business code")
	}
}
