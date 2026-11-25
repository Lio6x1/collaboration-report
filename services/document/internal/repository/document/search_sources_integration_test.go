package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestActualSearchSourceGateBatchKeepsOrderedLocksAndBoundedReads(t *testing.T) {
	f := searchMetadataActual(t)
	sources := []domain.LeaseSource{}
	for len(sources) < 200 {
		id := f.insertDocument(t, 0)
		sources = append(sources,
			domain.LeaseSource{Key: "doc-acl:" + domain.TextID(id), Epoch: "1"},
			domain.LeaseSource{Key: "publication:" + domain.TextID(id), Epoch: "1"})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	count := func() uint64 {
		var name string
		var value uint64
		if err := f.tx.QueryRowContext(f.ctx, "SHOW SESSION STATUS LIKE 'Com_select'").Scan(&name, &value); err != nil || name != "Com_select" {
			t.Fatal("ACTUAL_SEARCH_GATE_READ_COUNT_UNKNOWN")
		}
		return value
	}
	before := count()
	gates, err := LockSourceGatesBatch(f.ctx, f.tx, sources)
	if err != nil || len(gates) != len(sources) {
		t.Fatal("ACTUAL_SEARCH_GATE_BATCH_INCOMPLETE")
	}
	keys := make([]string, len(sources))
	for i, source := range sources {
		keys[i] = source.Key
		if gates[source.Key].Epoch != 1 || gates[source.Key].State != "OPEN" {
			t.Fatal("ACTUAL_SEARCH_GATE_FACTS_CHANGED")
		}
	}
	pending, err := PendingSourceTransitionsBatch(f.ctx, f.tx, keys)
	if err != nil || pending {
		t.Fatal("ACTUAL_SEARCH_GATE_TRANSITION_UNKNOWN")
	}
	reads := count() - before
	t.Logf("actual sources=200 SELECT round trips=%d", reads)
	// gate、授权过渡和跨空间父迁移各一次当前读，往返数不随来源数量增长。
	if reads > 3 {
		t.Fatal("ACTUAL_SEARCH_GATE_N_PLUS_ONE_READS")
	}
	// 业务仍拥有原锁及提交：另一真实连接的NOWAIT不得穿过整批中的最后一个gate。
	other, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_GATE_OTHER_TRANSACTION_UNKNOWN")
	}
	defer other.Rollback()
	var epoch uint64
	err = other.QueryRowContext(f.ctx, "SELECT max_epoch FROM access_gate WHERE scope_key=? FOR UPDATE NOWAIT", keys[len(keys)-1]).Scan(&epoch)
	var sqlError *mysql.MySQLError
	if !errors.As(err, &sqlError) || sqlError.Number != 3572 {
		t.Fatal("ACTUAL_SEARCH_GATE_BATCH_DID_NOT_RETAIN_CALLER_LOCKS")
	}
}

func TestActualSearchPendingBatchUsesCurrentReadAfterOldSnapshot(t *testing.T) {
	f := searchMetadataActual(t)
	key, operation := "publication:"+domain.TextID(f.document), f.operation+"-snapshot"
	var before int
	if f.tx.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM authorization_change WHERE scope_key=?", key).Scan(&before) != nil || before != 0 {
		t.Fatal("ACTUAL_SEARCH_OLD_READ_VIEW_UNKNOWN")
	}
	// 单独提交自有过渡事实，使原RR旧ReadView明确看不见它；没有durable_task，
	// 不构造任何实际关闭/ACK任务。清理只按本次operation、scope和epoch绑定删除。
	if _, err := f.db.ExecContext(f.ctx, "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,1,'{}','FENCING','[]',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 DAY))", operation, key); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_OLD_READ_VIEW_CHANGE", err)
	}
	t.Cleanup(func() {
		f.tx.Rollback()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		result, err := f.db.ExecContext(ctx, "DELETE FROM authorization_change WHERE revocation_id=? AND scope_key=? AND epoch=1 AND participant_plan=JSON_ARRAY()", operation, key)
		if err != nil {
			metadataSQLFailure(t, "ACTUAL_SEARCH_OLD_READ_VIEW_CLEANUP", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			t.Fatal("ACTUAL_SEARCH_OLD_READ_VIEW_CLEANUP_BINDING_UNKNOWN")
		}
	})
	if f.tx.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM authorization_change WHERE scope_key=?", key).Scan(&before) != nil || before != 0 {
		t.Fatal("ACTUAL_SEARCH_OLD_READ_VIEW_NOT_ESTABLISHED")
	}
	if _, err := LockSourceGatesBatch(f.ctx, f.tx, []domain.LeaseSource{{Key: key, Epoch: "2"}}); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_OLD_READ_VIEW_GATE", err)
	}
	pending, err := PendingSourceTransitionsBatch(f.ctx, f.tx, []string{key})
	if err != nil || !pending {
		t.Fatal("ACTUAL_SEARCH_PENDING_BATCH_USED_STALE_SNAPSHOT")
	}
}

func TestActualSearchPendingBatchDoesNotScanHeldUnrelatedStateIndex(t *testing.T) {
	f := searchMetadataActual(t)
	key := "publication:" + domain.TextID(f.document)
	if _, err := LockSourceGatesBatch(f.ctx, f.tx, []domain.LeaseSource{{Key: key, Epoch: "1"}}); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_UNRELATED_GATE", err)
	}
	other, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_UNRELATED_TRANSACTION", err)
	}
	defer other.Rollback()
	// 此scope在目标scope范围之前；它故意持有FENCING二级索引记录锁。
	// 查询不得转走auth_due扫描它；不声称RR下任何邻接gap都没有锁等待。
	if _, err := other.ExecContext(f.ctx, "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,1,'{}','FENCING','[]')", f.operation+"-unrelated", "kb-acl:"+domain.TextID(f.kb)); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_UNRELATED_HELD_CHANGE", err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()
	pending, err := PendingSourceTransitionsBatch(ctx, f.tx, []string{key})
	if err != nil || pending {
		t.Fatal("ACTUAL_SEARCH_PENDING_SCANNED_UNRELATED_HELD_STATE")
	}
}

func TestActualSearchSourceGateBatchPreservesBlockedMaxEpochAndPending(t *testing.T) {
	f := searchMetadataActual(t)
	key := "publication:" + domain.TextID(f.document)
	maximum := ^uint64(0)
	if _, err := f.tx.ExecContext(f.ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'BLOCKED')", key, maximum); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_GATE_BLOCKED_FIXTURE", err)
	}
	gates, err := LockSourceGatesBatch(f.ctx, f.tx, []domain.LeaseSource{{Key: key, Epoch: "1"}})
	if err != nil || gates[key].Epoch != maximum || gates[key].State != "BLOCKED" {
		t.Fatal("ACTUAL_SEARCH_GATE_REOPENED_BLOCKED_MAXIMUM")
	}
	if _, err := f.tx.ExecContext(f.ctx, "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,1,'{}','FENCING','[]')", f.operation+"-gate", key); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_GATE_PENDING_FIXTURE", err)
	}
	pending, err := PendingSourceTransitionsBatch(f.ctx, f.tx, []string{key})
	if err != nil || !pending {
		t.Fatal("ACTUAL_SEARCH_GATE_MISSED_ORIGINAL_PENDING")
	}
	for _, invalid := range [][]domain.LeaseSource{nil, {{Key: key, Epoch: "0"}}, {{Key: key, Epoch: "01"}}, {{Key: key, Epoch: "1"}, {Key: key, Epoch: "1"}}} {
		if values, err := LockSourceGatesBatch(f.ctx, f.tx, invalid); err == nil || values != nil {
			t.Fatal("ACTUAL_SEARCH_GATE_INVALID_BINDING_ACCEPTED")
		}
	}
}
