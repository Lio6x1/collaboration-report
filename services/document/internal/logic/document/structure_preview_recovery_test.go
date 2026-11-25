package document

import (
	domain "collaboration/document/internal/domain/document"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestStructurePreviewCaptureErrorKeepsUnknownOutcome(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal} {
		original := status.Error(code, "DRAFT_STRUCTURE_CHANGED")
		if structurePreviewCaptureError(original) != original {
			t.Fatal("未知结果不能仅凭错误文字转为明确拒绝")
		}
	}
	unknown := status.Error(codes.FailedPrecondition, "STRUCTURE_SCOPE_UNKNOWN")
	if structurePreviewCaptureError(unknown) != unknown {
		t.Fatal("范围缺失须保留原号恢复，不能冒充业务未提交")
	}
	for _, original := range []error{
		status.Error(codes.FailedPrecondition, "DRAFT_STRUCTURE_CHANGED"),
		status.Error(codes.FailedPrecondition, "STRUCTURE_DRAFT_CONFLICT"),
		status.Error(codes.ResourceExhausted, "STRUCTURE_TRANSFER_LIMIT"),
	} {
		var violation domain.RuleViolation
		if !errors.As(structurePreviewCaptureError(original), &violation) || violation.Kind != domain.StateConflict {
			t.Fatal("明确前置条件必须停止重试并拒绝原预览")
		}
	}
}

func TestStructurePreparationErrorRejectsOnlyKnownEditorReadiness(t *testing.T) {
	known := status.Error(codes.FailedPrecondition, "STRUCTURE_EDITORS_NOT_READY")
	var violation domain.RuleViolation
	if !errors.As(structurePreparationError(known), &violation) || violation.Kind != domain.StateConflict || violation.Code != "STRUCTURE_EDITORS_NOT_READY" {
		t.Fatal("明确未就绪须拒绝原确认，释放准备流程")
	}
	for _, unknown := range []error{
		status.Error(codes.Unavailable, "STRUCTURE_EDITORS_NOT_READY"),
		status.Error(codes.DeadlineExceeded, "STRUCTURE_EDITORS_NOT_READY"),
		status.Error(codes.FailedPrecondition, "STRUCTURE_PREPARATION_UNKNOWN"),
	} {
		if structurePreparationError(unknown) != unknown {
			t.Fatal("传输失败或未知前置条件须保留恢复，不能按错误文字结束原号")
		}
	}
}
