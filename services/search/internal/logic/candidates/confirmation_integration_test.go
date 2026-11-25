package candidates

import (
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"errors"
	"testing"
	"time"
)

func TestActualSQLConfirmationAndCompensationTimeoutRecoverOriginalInflight(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 1)
	fault := &faultIndexes{sink: f.sink}
	t.Cleanup(func() { fault.cleanup(t) })
	var err error
	f.usecase, err = New(f.store.DB, f.guard, fault, f.alias, f.key, 4*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	t.Cleanup(func() {
		select {
		case <-fault.release:
		default:
			close(fault.release)
		}
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("ACTUAL_SQL_UNKNOWN_QUERY_UNFINISHED")
		}
	})
	go func() {
		defer close(finished)
		reply, err := f.usecase.SearchCandidates(ctx, requestAfter(t, request, domain.ActionReplay, first))
		if reply != nil || err == nil {
			err = errors.New("ACTUAL_SQL_UNKNOWN_RETURNED_SUCCESS")
		}
		result <- err
	}()
	select {
	case <-fault.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ACTUAL_ES_CONFIRMATION_WAITPOINT_MISSING")
	}
	// ES已真实成功，另一SQL连接锁住原cursor行，直到查询确认及独立1秒补偿都耗尽。
	// 此项证明真实锁超时后的持久inflight恢复，不声称模拟了COMMIT已成功但回执丢失。
	holder, err := f.store.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	var raw []byte
	if err := holder.QueryRow("SELECT last_sort FROM cursor_registry WHERE cursor_hash=? FOR UPDATE", hash[:]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	control, err := domain.DecodeControl(raw)
	if err != nil || control.Phase != domain.PhaseInflight {
		t.Fatal("ACTUAL_SQL_UNKNOWN_NOT_DURABLY_INFLIGHT", err)
	}
	close(fault.release)
	select {
	case err := <-result:
		if err == nil || err.Error() == "ACTUAL_SQL_UNKNOWN_RETURNED_SUCCESS" {
			t.Fatal("ACTUAL_SQL_UNKNOWN_HALF_BATCH", err)
		}
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("ACTUAL_SQL_UNKNOWN_EXCEEDED_QUERY_AND_COMPENSATION_BUDGET")
	}
	if err := holder.Rollback(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(f.store.DB, f.guard, f.sink, f.alias, f.key, 4*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var state string
	var expiry time.Time
	if err := f.store.DB.QueryRow("SELECT state,expires_at FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &expiry); err != nil || state != "CLOSING" || expiry.UnixMilli() != first.ExpiresUnixMs {
		t.Fatal("ACTUAL_SQL_UNKNOWN_RELEASED_EARLY_OR_RENEWED", err)
	}
	if reply, err := restarted.SearchCandidates(context.Background(), requestAfter(t, request, domain.ActionReplay, first)); err == nil || reply != nil {
		t.Fatal("ACTUAL_SQL_UNKNOWN_REOPENED_CHAIN")
	}
	time.Sleep(time.Until(expiry) + 20*time.Millisecond)
	if err := restarted.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=? AND state<>'CLOSED'", hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("ACTUAL_SQL_UNKNOWN_NOT_RELEASED_AT_ORIGINAL_EXPIRY", err)
	}
}
