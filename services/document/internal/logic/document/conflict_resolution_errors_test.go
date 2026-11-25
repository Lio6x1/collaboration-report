package document

import (
	domain "collaboration/document/internal/domain/document"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestResolutionRejectsOnlyDefiniteProposalOrTransferFailures(t *testing.T) {
	for _, failure := range []struct {
		code           codes.Code
		remote, public string
	}{
		{codes.InvalidArgument, "RESOLUTION_CONTENT_INVALID", "RESOLUTION_CONTENT_INVALID"},
		{codes.ResourceExhausted, "DISCARD_TRANSFER_LIMIT", "RESOLUTION_TRANSFER_LIMIT"},
	} {
		var violation domain.RuleViolation
		converted := resolutionPreviewError(status.Error(failure.code, failure.remote))
		if !errors.As(converted, &violation) || violation.Kind != domain.StateConflict || violation.Code != failure.public {
			t.Fatal("definite input failure left unknown")
		}
	}
	// 未认识的RPC详情、坏固定来源与网络失败都必须保留未知，不能猜测prepare从未发生。
	for _, err := range []error{
		status.Error(codes.InvalidArgument, "FIXED_CONTENT_INVALID"),
		status.Error(codes.ResourceExhausted, "COMPUTE_BUSY"),
		status.Error(codes.Unavailable, "SERVICE_UNAVAILABLE"),
		status.Error(codes.DeadlineExceeded, "RESOLUTION_CONTENT_INVALID"),
		errors.New("connection failure"),
	} {
		if resolutionPreviewError(err) != err {
			t.Fatal("unknown failure incorrectly releases control")
		}
	}
}
