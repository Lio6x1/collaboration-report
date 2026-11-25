package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestActualSearchLockedSourcesKeepCopiesAndOriginalRollback(t *testing.T) {
	f := newActualSearchFixedFixture(t)
	sources := []*pb.SourceEpoch{{ScopeKey: "publication:" + domain.TextID(f.document), Epoch: 1}, {ScopeKey: "doc-acl:" + domain.TextID(f.document), Epoch: 1}}
	proof, err := lockSearchOutputSources(f.ctx, f.tx, sources)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_PROOF_UNAVAILABLE")
	}
	want := SourcesWire(proof.copySources())
	// 外部protobuf指针以及bind取得的副本都不得修改已经锁定的权限集合。
	sources[0].Epoch = 99
	sources[1] = nil
	copy := proof.copySources()
	copy[0].Epoch = 999
	if !reflect.DeepEqual(want, SourcesWire(proof.copySources())) {
		t.Fatal("ACTUAL_SEARCH_SOURCE_PROOF_ALIASED_CALLER")
	}
	self := domain.OutputInstance{ID: f.operation + "-document", PID: 1, Boot: strings.Repeat("01", 32)}
	edge := domain.OutputInstance{ID: f.operation + "-edge", PID: 2, Boot: strings.Repeat("02", 32)}
	// 这里只检验SQL登记/回滚；独立SQL夹具不声称是实际进程、handler或ACK。
	if err := repository.EnsureInstance(f.ctx, f.tx, self); err != nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_SQL_INSTANCE_FIXTURE_FAILED")
	}
	outputs := &Outputs{self: self}
	registration, err := outputs.registerLockedSearch(f.ctx, proof, "fixture-handler", &edge)
	if err != nil || registration == nil || registration.Edge == nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_LEASE_REGISTRATION_FAILED")
	}
	for _, lease := range []domain.OutputLease{registration.Local, *registration.Edge} {
		var raw []byte
		if err := f.tx.QueryRowContext(f.ctx, "SELECT payload FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=?", lease.ID, lease.Instance).Scan(&raw); err != nil {
			t.Fatal("ACTUAL_SEARCH_SOURCE_LEASE_CURRENT_READ_FAILED")
		}
		var current domain.OutputLease
		if json.Unmarshal(raw, &current) != nil || !reflect.DeepEqual(current.Sources, want) || current.Document != 0 || current.Admission != "" {
			t.Fatal("ACTUAL_SEARCH_SOURCE_LEASE_BINDING_CHANGED")
		}
	}
	if _, err := outputs.registerLockedSearch(f.ctx, proof, "fixture-handler", &edge); err == nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_PROOF_REUSED")
	}
	if err := f.tx.Rollback(); err != nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_ROLLBACK_UNKNOWN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var count int
	if err := f.search.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key IN (?,?)", registration.Local.ID, registration.Edge.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("ACTUAL_SEARCH_SOURCE_LEASE_ESCAPED_ORIGINAL_TRANSACTION")
	}
	if value, err := lockSearchOutputSources(ctx, f.tx, proof.copySources()); err == nil || value != nil {
		t.Fatal("ACTUAL_SEARCH_SOURCE_PROOF_SURVIVED_ROLLBACK")
	}
}

func TestActualSearchLockedSourcesPendingCannotReopenHigherEpoch(t *testing.T) {
	f := newActualSearchFixedFixture(t)
	key := "publication:" + domain.TextID(f.document)
	f.exec(t, "ACTUAL_SEARCH_PENDING_GATE_FIXTURE", "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'BLOCKED')", key)
	f.exec(t, "ACTUAL_SEARCH_PENDING_CHANGE_FIXTURE", "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,1,'{}','FENCING','[]')", f.operation, key)
	proof, err := lockSearchOutputSources(f.ctx, f.tx, []*pb.SourceEpoch{{ScopeKey: key, Epoch: 2}})
	var violation domain.RuleViolation
	if proof != nil || !errors.As(err, &violation) || violation.Code != "SCOPE_TRANSITION_PENDING" {
		t.Fatal("ACTUAL_SEARCH_PENDING_PROOF_ACCEPTED")
	}
	var state string
	var epoch uint64
	if f.tx.QueryRowContext(f.ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=?", key).Scan(&epoch, &state) != nil || epoch != 1 || state != "BLOCKED" {
		t.Fatal("ACTUAL_SEARCH_PENDING_GATE_REOPENED")
	}
}

func TestActualSearchSourceBatchCompetesWithLegacyAndClosureGateOrder(t *testing.T) {
	f := newActualSearchFixedFixture(t)
	keys := []string{"doc-acl:" + domain.TextID(f.document), "publication:" + domain.TextID(f.document)}
	// ID由本次自有图分配；先回滚图，再只构造来源锁竞争。正式来源已有
	// gate，与完全缺失gate的首次并发INSERT竞争分别验证；不声称完整关闭器验收。
	if f.tx.Rollback() != nil {
		t.Fatal("ACTUAL_SEARCH_COMPETING_GATE_FIXTURE_ROLLBACK_UNKNOWN")
	}
	if _, err := f.search.db.ExecContext(f.ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN'),(?,1,'OPEN')", keys[0], keys[1]); err != nil {
		searchFixedSQLFailure(t, "ACTUAL_SEARCH_COMPETING_EXISTING_GATE_FIXTURE", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		result, err := f.search.db.ExecContext(ctx, "DELETE FROM access_gate WHERE scope_key IN (?,?) AND max_epoch=1 AND state='OPEN' AND revision=1 AND last_revocation_id IS NULL", keys[0], keys[1])
		if err != nil {
			searchFixedSQLFailure(t, "ACTUAL_SEARCH_COMPETING_GATE_CLEANUP", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 2 {
			t.Fatal("ACTUAL_SEARCH_COMPETING_GATE_CLEANUP_BINDING_UNKNOWN")
		}
	})
	for iteration := 0; iteration < 8; iteration++ {
		start := make(chan struct{})
		results := make(chan error, 3)
		var ready sync.WaitGroup
		ready.Add(3)
		for path := 0; path < 3; path++ {
			go func(path int) {
				ready.Done()
				<-start
				tx, err := f.search.db.BeginTx(f.ctx, nil)
				if err != nil {
					results <- err
					return
				}
				defer tx.Rollback()
				sources := []*pb.SourceEpoch{{ScopeKey: keys[0], Epoch: 1}, {ScopeKey: keys[1], Epoch: 1}}
				switch path {
				case 0:
					_, err = lockSearchOutputSources(f.ctx, tx, sources)
				case 1:
					// 旧路径输入反序，必须仍与bulk遵循相同gate字典序。
					err = LockSources(f.ctx, tx, []*pb.SourceEpoch{sources[1], sources[0]})
				case 2:
					// 关闭器的原gate→过渡锁边界，使用同一真实repository原语。
					_, err = repository.LockSourceGate(f.ctx, tx, keys[0], 1)
					if err == nil {
						_, err = repository.PendingSourceTransition(f.ctx, tx, keys[0])
					}
				}
				if rollbackErr := tx.Rollback(); err == nil {
					err = rollbackErr
				}
				results <- err
			}(path)
		}
		ready.Wait()
		close(start)
		for path := 0; path < 3; path++ {
			if err := <-results; err != nil {
				searchFixedSQLFailure(t, "ACTUAL_SEARCH_COMPETING_SOURCE_LOCK_FAILED", err)
			}
		}
	}
}

func TestActualSearchSourceInitializationDeadlockCannotProduceProof(t *testing.T) {
	f := newActualSearchFixedFixture(t)
	key := "publication:" + domain.TextID(f.document)
	if f.tx.Rollback() != nil {
		t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_FIXTURE_ROLLBACK_UNKNOWN")
	}
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	transactions := make([]*sql.Tx, 2)
	for i := range transactions {
		tx, err := f.search.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_TRANSACTION_UNKNOWN")
		}
		defer tx.Rollback()
		transactions[i] = tx
		var epoch uint64
		// 两个原事务均持有同一缺失key的兼容gap锁，然后竞争首次INSERT。
		// 这是明确故障注入，不声称正常业务会预先这样锁不存在的gate。
		if err := tx.QueryRowContext(ctx, "SELECT max_epoch FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&epoch); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_MISSING_GATE_NOT_CONFIRMED")
		}
	}
	type outcome struct {
		proof *lockedOutputSources
		err   error
	}
	start, results := make(chan struct{}), make(chan outcome, 2)
	for _, tx := range transactions {
		go func(tx *sql.Tx) {
			<-start
			proof, err := lockSearchOutputSources(ctx, tx, []*pb.SourceEpoch{{ScopeKey: key, Epoch: 1}})
			rollbackErr := tx.Rollback()
			if err == nil && rollbackErr != nil {
				err = rollbackErr
			}
			results <- outcome{proof, err}
		}(tx)
	}
	close(start)
	deadlocks, successes := 0, 0
	for range transactions {
		result := <-results
		if result.err == nil {
			if result.proof == nil {
				t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_SUCCESS_WITHOUT_PROOF")
			}
			successes++
			continue
		}
		var failure *mysql.MySQLError
		if result.proof != nil || !errors.As(result.err, &failure) || failure.Number != 1213 {
			t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_FAILED_WITH_PROOF_OR_UNKNOWN_ERROR")
		}
		var violation domain.RuleViolation
		if !errors.As(searchFinalChange(result.err), &violation) || violation.Code != "SEARCH_INCOMPLETE" {
			t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_DEADLOCK_NOT_FAIL_CLOSED")
		}
		deadlocks++
	}
	var remaining int
	if deadlocks != 1 || successes != 1 || f.search.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM access_gate WHERE scope_key=?", key).Scan(&remaining) != nil || remaining != 0 {
		t.Fatal("ACTUAL_SEARCH_INITIAL_SOURCE_DEADLOCK_OR_ROLLBACK_NOT_CONFIRMED")
	}
}
