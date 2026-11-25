package candidates

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/search/internal/domain/cursor"
	"collaboration/search/internal/logic/generation"
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestActualCommittedConfirmationReceiptLostRetainsOriginalNextRetry(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 105)
	request := fixtureRequest()
	first, err := f.usecase.SearchCandidates(context.Background(), request)
	if err != nil || first == nil || !first.Complete || !first.HasMore || len(first.Candidates) != 100 {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_FIRST_BATCH_REQUIRED")
	}
	hash := f.remember(t, first)
	next := requestAfter(t, request, domain.ActionNext, first)
	remaining := make(map[uint64]bool, 105)
	for id := uint64(1); id <= 105; id++ {
		remaining[id] = true
	}
	for _, candidate := range first.Candidates {
		if !remaining[candidate.DocumentId] {
			t.Fatal("ACTUAL_FIRST_BATCH_BINDING_OR_DUPLICATE_INVALID")
		}
		delete(remaining, candidate.DocumentId)
	}
	if len(remaining) != 5 {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_SUFFIX_FIXTURE_INVALID")
	}

	// SQL/ES始终真实执行。仅替换database/sql driver的结果交付边界：delegate
	// Commit已成功之后返回一次固定harness错误；这不声称真实网络断连。
	database, fault := commitReceiptDatabase(t)
	guard, err := generation.New(database, f.sink, f.alias, f.physical)
	if err != nil {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_GENERATION_UNAVAILABLE")
	}
	usecase, err := New(database, guard, f.sink, f.alias, f.key, 30*time.Second)
	if err != nil {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_USECASE_UNAVAILABLE")
	}
	// 初批完全成功后明确arm原hash；识别Save的实参OPEN+IDLE且其前一个真实提交
	// 为同hash INFLIGHT，避免按事务序号误伤reserve/beginQuery/代次只读事务。
	fault.arm(hash)
	unknown, err := usecase.SearchCandidates(context.Background(), next)
	if unknown != nil || err == nil || err.Error() != "SEARCH_CURSOR_COMMIT_UNKNOWN" {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_UNKNOWN_RETURNED_PARTIAL_OR_SUCCESS")
	}
	evidence := fault.snapshot()
	if evidence.InflightCommits != 1 || evidence.IdleCommits != 1 || evidence.LostReceipts != 1 || evidence.Armed || len(evidence.Tail) == 0 {
		t.Fatal("COMMIT_RECEIPT_HARNESS_DID_NOT_CONFIRM_EXACT_TARGET_TRANSACTION")
	}
	var state string
	var controlJSON []byte
	var expiry time.Time
	if err := f.store.DB.QueryRow("SELECT state,last_sort,expires_at FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &controlJSON, &expiry); err != nil {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_CONFIRMED_SQL_UNAVAILABLE")
	}
	control, err := domain.DecodeControl(controlJSON)
	if err != nil || state != "OPEN" || control.Phase != domain.PhaseIdle || control.Nonce != "" || control.Deadline != 0 || control.Uncertain ||
		!bytes.Equal(control.Tail, evidence.Tail) || expiry.UnixMilli() != first.ExpiresUnixMs {
		t.Fatal("COMMITTED_IDLE_FACT_LOST_OR_COMPENSATION_CLOSED_CONFIRMED_CHAIN")
	}
	// 这是已提交确认、仅丢回执的分支：c.fail读到IDLE的空nonce，与原nonce不同而
	// 保留原确认事实。未提交/无法复核的在途事实收紧至CLOSING由另一真实锁超时项覆盖。
	retry, err := usecase.SearchCandidates(context.Background(), next)
	if err != nil || retry == nil || !retry.Complete || retry.HasMore || len(retry.Candidates) != 5 || len(retry.Continuation) == 0 ||
		retry.ExpiresUnixMs != first.ExpiresUnixMs || retry.PhysicalIndex != f.physical || retry.IndexGeneration != first.IndexGeneration {
		t.Fatal("ACTUAL_COMMITTED_ORIGINAL_NEXT_RETRY_LOST_LAST_FIVE")
	}
	for _, candidate := range retry.Candidates {
		if !remaining[candidate.DocumentId] || candidate.ReleaseId != ^uint64(0) || candidate.AggregateSeq != ^uint64(0) || candidate.PhysicalIndex != f.physical {
			t.Fatal("ACTUAL_COMMITTED_RETRY_DUPLICATED_OR_CHANGED_SUFFIX_BINDING")
		}
		delete(remaining, candidate.DocumentId)
	}
	if len(remaining) != 0 || !bytes.Equal(retry.Candidates[len(retry.Candidates)-1].SortJsonUtf8, evidence.Tail) {
		t.Fatal("ACTUAL_COMMITTED_RETRY_SWALLOWED_OR_CHANGED_CONFIRMED_TAIL")
	}
	secondRetry, err := usecase.SearchCandidates(context.Background(), next)
	if err != nil || !proto.Equal(retry, secondRetry) {
		t.Fatal("ACTUAL_COMMITTED_ORIGINAL_NEXT_NOT_DETERMINISTIC")
	}
	if after := fault.snapshot(); after.LostReceipts != 1 || after.Armed {
		t.Fatal("COMMIT_RECEIPT_HARNESS_INJECTED_MORE_THAN_ONCE")
	}
	var rows int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE physical_index=?", f.physical).Scan(&rows); err != nil || rows != 1 {
		t.Fatal("COMMIT_RECEIPT_RETRY_REGISTERED_ANOTHER_CHAIN")
	}
	if err := f.store.DB.QueryRow("SELECT expires_at FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&expiry); err != nil || expiry.UnixMilli() != first.ExpiresUnixMs {
		t.Fatal("COMMIT_RECEIPT_RETRY_EXTENDED_ORIGINAL_SQL_EXPIRY")
	}
	closed, err := usecase.CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: retry.Continuation, Scope: request.Scope})
	if err != nil || closed == nil || closed.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("ACTUAL_COMMIT_RECEIPT_CURSOR_CLOSE_FAILED")
	}
	t.Log("real MySQL delegate Commit succeeded; harness discarded one confirmation receipt; original NEXT retried the same five candidates")
}
