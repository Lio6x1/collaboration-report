package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

// 只把Node明确实施的两个容量上限视为最终拒绝；通用ResourceExhausted、
// deadline和普通依赖错误仍是未知，不可猜成已失败再让用户创建另一个原号。
func TestPublicationCaptureKnownLimitsAreTerminal(t *testing.T) {
	for _, sample := range []struct{ rpc, business string }{
		{"MAIN_CONTENT_TOO_LARGE", "PUBLICATION_CONTENT_TOO_LARGE"},
		{"MAIN_VIEW_TRANSFER_LIMIT", "PUBLICATION_TRANSFER_LIMIT"},
	} {
		mapped := publicationCaptureError(status.Error(codes.ResourceExhausted, sample.rpc))
		var violation domain.RuleViolation
		if !errors.As(mapped, &violation) || violation.Kind != domain.StateConflict || violation.Code != sample.business {
			t.Fatal("explicit MAIN capacity must reject the original publication")
		}
	}
}

func TestPublicationCaptureUnknownErrorsRetainOriginalRecovery(t *testing.T) {
	for _, original := range []error{
		context.DeadlineExceeded,
		context.Canceled,
		errors.New("MAIN_VIEW_TRANSFER_LIMIT"),
		status.Error(codes.ResourceExhausted, "UNIT_UNKNOWN_CAPACITY"),
		status.Error(codes.Unavailable, "MAIN_VIEW_TRANSFER_LIMIT"),
		status.Error(codes.DeadlineExceeded, "MAIN_CONTENT_TOO_LARGE"),
	} {
		if publicationCaptureError(original) != original {
			t.Fatal("unknown capture result must preserve its original error")
		}
	}
	if publicationCaptureError(nil) != nil {
		t.Fatal("nil must remain successful")
	}
}
