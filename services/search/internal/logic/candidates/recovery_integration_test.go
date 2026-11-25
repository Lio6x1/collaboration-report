package candidates

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/search/internal/client/elastic"
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// faultIndexes仍执行真实ES，只在真实成功之后丢弃调用结果/阻塞交付，模拟跨系统未知窗口。
// 它不是替代ES的成功stub；原SQL和实际PIT的创建、查询、关闭均保持。
type faultIndexes struct {
	sink             *elastic.Client
	loseOpen         bool
	started, release chan struct{}
	mu               sync.Mutex
	lastPIT          string
}

func (f *faultIndexes) retain(pit string) { f.mu.Lock(); f.lastPIT = pit; f.mu.Unlock() }
func (f *faultIndexes) OpenPIT(ctx context.Context, physical string, remaining time.Duration) (string, error) {
	pit, err := f.sink.OpenPIT(ctx, physical, remaining)
	if pit != "" {
		f.retain(pit)
	}
	if err == nil && f.loseOpen {
		return "", errors.New("SEARCH_ES_PIT_OPEN_UNKNOWN")
	}
	return pit, err
}
func (f *faultIndexes) Candidates(ctx context.Context, pit, query string, after json.RawMessage, size int, remaining time.Duration) (elastic.CandidateBatch, error) {
	batch, err := f.sink.Candidates(ctx, pit, query, after, size, remaining)
	if batch.PIT != "" {
		f.retain(batch.PIT)
	}
	if err == nil && f.started != nil {
		close(f.started)
		select {
		case <-f.release:
		case <-ctx.Done():
			return elastic.CandidateBatch{}, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
		}
	}
	return batch, err
}
func (f *faultIndexes) ClosePIT(ctx context.Context, pit string) error {
	return f.sink.ClosePIT(ctx, pit)
}

func (f *faultIndexes) cleanup(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	pit := f.lastPIT
	f.mu.Unlock()
	if pit != "" {
		_ = f.sink.ClosePIT(context.Background(), pit)
	}
}

func TestActualUnknownOpenRetainsQuotaUntilOriginalExpiryThenRecovers(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 1)
	fault := &faultIndexes{sink: f.sink, loseOpen: true}
	t.Cleanup(func() { fault.cleanup(t) })
	var err error
	f.usecase, err = New(f.store.DB, f.guard, fault, f.alias, f.key, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if reply, err := f.usecase.SearchCandidates(context.Background(), fixtureRequest()); err == nil || reply != nil {
		t.Fatal("ACTUAL_UNKNOWN_OPEN_RETURNED_SUCCESS")
	}
	var raw []byte
	var state, pit string
	var until time.Time
	if err := f.store.DB.QueryRow("SELECT cursor_hash,state,pit_handle,expires_at FROM cursor_registry WHERE physical_index=?", f.physical).Scan(&raw, &state, &pit, &until); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 32 || state != "CLOSING" || pit != "" {
		t.Fatal("ACTUAL_UNKNOWN_OPEN_NOT_DURABLY_CLOSING")
	}
	var hash [32]byte
	copy(hash[:], raw)
	f.hashes = append(f.hashes, hash)
	if err := f.usecase.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var uncertain bool
	if err := f.store.DB.QueryRow("SELECT state,JSON_EXTRACT(last_sort,'$.uncertain') FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &uncertain); err != nil || state != "CLOSING" || !uncertain {
		t.Fatal("ACTUAL_UNKNOWN_OPEN_RELEASED_QUOTA_EARLY", err)
	}
	time.Sleep(time.Until(until) + 20*time.Millisecond)
	// 新用例模拟进程内状态全部丢失，恢复只沿原SQL未知行，不重开PIT。
	restarted, err := New(f.store.DB, f.guard, f.sink, f.alias, f.key, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=? AND state<>'CLOSED'", hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("ACTUAL_UNKNOWN_OPEN_NOT_RECOVERED", err)
	}
}

func TestActualCloseDuringQueryRejectsLateConfirmationAndRetainsUnknownQuota(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 105)
	fault := &faultIndexes{sink: f.sink}
	t.Cleanup(func() { fault.cleanup(t) })
	var err error
	f.usecase, err = New(f.store.DB, f.guard, fault, f.alias, f.key, 1500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	request := fixtureRequest()
	first, err := f.usecase.SearchCandidates(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	hash := f.remember(t, first)
	fault.started, fault.release = make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	finished := make(chan struct{})
	next := requestAfter(t, request, domain.ActionNext, first)
	t.Cleanup(func() {
		select {
		case <-fault.release:
		default:
			close(fault.release)
		}
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("ACTUAL_QUERY_CLEANUP_UNFINISHED")
		}
	})
	go func() {
		defer close(finished)
		reply, err := f.usecase.SearchCandidates(context.Background(), next)
		if err == nil && reply != nil {
			err = errors.New("ACTUAL_LATE_QUERY_REOPENED_CURSOR")
		}
		result <- err
	}()
	select {
	case <-fault.started:
	case <-time.After(time.Second):
		t.Fatal("ACTUAL_QUERY_WAITPOINT_NOT_REACHED")
	}
	closed, err := f.usecase.CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: first.Continuation, Scope: request.Scope})
	close(fault.release)
	if err != nil || closed.State != pb.OperationResult_PROCESSING {
		t.Fatal("ACTUAL_INFLIGHT_CLOSE_DROPPED_UNKNOWN_QUOTA", err)
	}
	select {
	case err := <-result:
		if err == nil || err.Error() == "ACTUAL_LATE_QUERY_REOPENED_CURSOR" {
			t.Fatal("ACTUAL_LATE_QUERY_CONFIRMED_SUCCESS", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ACTUAL_QUERY_DID_NOT_FINISH")
	}
	var state string
	var until time.Time
	if err := f.store.DB.QueryRow("SELECT state,expires_at FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &until); err != nil || state != "CLOSING" || until.UnixMilli() != first.ExpiresUnixMs {
		t.Fatal("ACTUAL_LATE_REPLY_CHANGED_CLOSED_BINDING", err)
	}
	time.Sleep(time.Until(until) + 20*time.Millisecond)
	if err := f.usecase.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=? AND state<>'CLOSED'", hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("ACTUAL_UNKNOWN_ROTATION_NOT_RELEASED_AFTER_ORIGINAL_EXPIRY", err)
	}
}
