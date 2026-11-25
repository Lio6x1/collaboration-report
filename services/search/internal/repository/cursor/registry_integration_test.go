package cursor

import (
	"collaboration/platform"
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func actualCursorDB(t *testing.T) *sql.DB {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	return store.DB
}

func actualCursorTx(t *testing.T, db *sql.DB) (context.Context, *sql.Tx) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal("ACTUAL_CURSOR_TRANSACTION_FAILED")
	}
	t.Cleanup(func() { tx.Rollback() })
	// 新链配额由同一代次排他锁串行化；连代次夹具也不提交，避免遗留业务事实。
	alias := fmt.Sprintf("actual-cursor-%d", time.Now().UnixNano())
	if _, err := tx.ExecContext(ctx, "INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)", alias, alias+"-g1"); err != nil {
		t.Fatal("ACTUAL_CURSOR_GENERATION_FIXTURE_FAILED")
	}
	var locked string
	if err := tx.QueryRowContext(ctx, "SELECT physical_index FROM index_generation WHERE alias_name=? FOR UPDATE", alias).Scan(&locked); err != nil {
		t.Fatal("ACTUAL_CURSOR_GENERATION_LOCK_FAILED")
	}
	return ctx, tx
}

func cursorFixture(t *testing.T, label string, expires time.Time) Record {
	t.Helper()
	id := fmt.Sprintf("%s/%s/%d", t.Name(), label, time.Now().UnixNano())
	return Record{
		Hash: sha256.Sum256([]byte(id)), Physical: "actual-cursor-g1", Generation: 1,
		ScopeHash: sha256.Sum256([]byte(id + "/scope")), QueryHash: sha256.Sum256([]byte(id + "/query")),
		PageSize: 100, Expires: expires.UTC().Truncate(time.Millisecond), State: Open,
		Control: domain.Control{Version: 1, Phase: domain.PhaseOpening, Nonce: strings.Repeat("a", 64), Deadline: expires.UnixMilli()},
	}
}

func registryCounts(t *testing.T, ctx context.Context, tx *sql.Tx) (int, int) {
	t.Helper()
	var open, total int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(state IN ('OPEN','CLOSING')),0),COUNT(*) FROM cursor_registry").Scan(&open, &total); err != nil {
		t.Fatal("ACTUAL_CURSOR_BASELINE_FAILED")
	}
	return open, total
}

func TestActualCursorCreateKeepsCallerRollback(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	record := cursorFixture(t, "rollback", time.Now().Add(30*time.Second))
	if err := Create(ctx, tx, &record, 128, 1024); err != nil {
		t.Fatal(err)
	}
	if got, found, err := Lock(ctx, tx, record.Hash); err != nil || !found || got.Hash != record.Hash {
		t.Fatal("ACTUAL_CURSOR_CREATED_ROW_UNREADABLE")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", record.Hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("CURSOR_CREATE_COMMITTED_CALLER_TRANSACTION")
	}
}

func TestActualCursorSaveKeepsOneRowAndRejectsImmutableChanges(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	record := cursorFixture(t, "save", time.Now().Add(30*time.Second))
	record.Generation = math.MaxUint64
	record.Expires = record.Expires.In(time.FixedZone("actual-offset", 8*60*60))
	record.Session = []byte(strings.Repeat("s", 32))
	if err := Create(ctx, tx, &record, 128, 1024); err != nil {
		t.Fatal(err)
	}
	if record.Expires.Location() != time.UTC {
		t.Fatal("CURSOR_EXPIRY_NOT_CANONICAL_UTC")
	}
	record.PIT = "actual-first-pit"
	// MySQL JSON会重写数组数字，持久tail须保留原sort字符串，包含指数词法及最大uint64。
	record.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Tail: []byte(`[1.2500e+2,"18446744073709551615",18446744073709551615]`)}
	if err := Save(ctx, tx, record); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name string
		fn   func(*Record)
	}{
		{"physical", func(r *Record) { r.Physical += "-other" }},
		{"generation", func(r *Record) { r.Generation-- }},
		{"scope", func(r *Record) { r.ScopeHash[0] ^= 1 }},
		{"query", func(r *Record) { r.QueryHash[0] ^= 1 }},
		{"session", func(r *Record) { r.Session = []byte(strings.Repeat("t", 32)) }},
		{"page", func(r *Record) { r.PageSize-- }},
		{"expires", func(r *Record) { r.Expires = r.Expires.Add(time.Millisecond) }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := record
			mutation.fn(&changed)
			if err := Save(ctx, tx, changed); err == nil {
				t.Fatal("IMMUTABLE_CURSOR_BINDING_CHANGED")
			}
		})
	}
	for i := range 3 {
		record.PIT = fmt.Sprintf("actual-next-pit-%d", i)
		if err := Save(ctx, tx, record); err != nil {
			t.Fatal(err)
		}
	}
	got, found, err := Lock(ctx, tx, record.Hash)
	if err != nil || !found || got.PIT != record.PIT || string(got.Control.Tail) != string(record.Control.Tail) || !got.Expires.Equal(record.Expires) {
		t.Fatal("CURSOR_PROGRESS_LOST_ORIGINAL_BINDING")
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", record.Hash[:]).Scan(&count); err != nil || count != 1 {
		t.Fatal("CURSOR_CHAIN_CREATED_PER_BATCH_ROWS")
	}
	if err := Create(ctx, tx, &record, 128, 1024); err == nil {
		t.Fatal("DUPLICATE_CHAIN_CREATED_NEW_ROW")
	}
	record.State = Closing
	if err := Save(ctx, tx, record); err != nil {
		t.Fatal(err)
	}
	record.State = Open
	if err := Save(ctx, tx, record); err == nil {
		t.Fatal("CLOSING_CURSOR_REOPENED")
	}
	record.State = Closing
	record.PIT = ""
	record.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Uncertain: true}
	if err := Save(ctx, tx, record); err != nil {
		t.Fatal("UNKNOWN_PIT_CLOSING_FACT_REJECTED", err)
	}
	record.Control.Phase = domain.PhaseInflight
	record.Control.Nonce = strings.Repeat("b", 64)
	record.Control.Deadline = record.Expires.UnixMilli()
	if err := Save(ctx, tx, record); err != nil {
		t.Fatal("UNKNOWN_PIT_CLOSE_CLAIM_REJECTED", err)
	}
	record.Control.Uncertain = false
	if err := Save(ctx, tx, record); err == nil {
		t.Fatal("EMPTY_PIT_WITHOUT_UNKNOWN_FACT_ACCEPTED")
	}
}

func TestActualCursorQuotaIncludesOtherGenerationsAndClosing(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	baselineOpen, baselineRows := registryCounts(t, ctx, tx)
	if baselineOpen+2 > 128 || baselineRows+3 > 1024 {
		t.Fatal("ACTUAL_CURSOR_QUOTA_FIXTURE_CAPACITY_REQUIRED")
	}
	first := cursorFixture(t, "opening", time.Now().Add(30*time.Second))
	second := cursorFixture(t, "other-generation-closing", first.Expires)
	second.Generation = 2
	second.Physical = "actual-cursor-g2"
	for _, record := range []*Record{&first, &second} {
		if err := Create(ctx, tx, record, baselineOpen+2, baselineRows+3); err != nil {
			t.Fatal(err)
		}
	}
	second.State = Closing
	second.Control.Uncertain = true
	if err := Save(ctx, tx, second); err != nil {
		t.Fatal(err)
	}
	third := cursorFixture(t, "blocked-unreleased", first.Expires)
	if err := Create(ctx, tx, &third, baselineOpen+2, baselineRows+3); err == nil {
		t.Fatal("OPENING_OR_CLOSING_OTHER_GENERATION_ESCAPED_QUOTA")
	}
	first.State = Closed
	first.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
	if err := Save(ctx, tx, first); err != nil {
		t.Fatal(err)
	}
	if err := Create(ctx, tx, &third, baselineOpen+2, baselineRows+3); err != nil {
		t.Fatal(err)
	}
	third.State = Closed
	third.Control = first.Control
	if err := Save(ctx, tx, third); err != nil {
		t.Fatal(err)
	}
	fourth := cursorFixture(t, "blocked-total", first.Expires)
	if err := Create(ctx, tx, &fourth, baselineOpen+2, baselineRows+3); err == nil {
		t.Fatal("CLOSED_ROWS_ESCAPED_TOTAL_REGISTRY_QUOTA")
	}
	for _, limits := range [][2]int{{0, 1024}, {129, 1024}, {128, 0}, {128, 1025}} {
		if err := Create(ctx, tx, &fourth, limits[0], limits[1]); err == nil {
			t.Fatal("INVALID_QUOTA_LIMIT_ACCEPTED")
		}
	}
	open, total := registryCounts(t, ctx, tx)
	if open != baselineOpen+1 || total != baselineRows+3 {
		t.Fatal("REJECTED_CREATE_CHANGED_REGISTRY")
	}
}

func TestActualCursorDueSkipsHealthyClaimsAndRetryDoesNotStarve(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	now := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	failed := cursorFixture(t, "retry", now.Add(-3*time.Second))
	failed.State = Closing
	failed.PIT = "actual-retry-pit"
	failed.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
	stale := cursorFixture(t, "stale-opening", now.Add(time.Second))
	stale.Control.Deadline = now.Add(-time.Millisecond).UnixMilli()
	healthy := cursorFixture(t, "healthy-closing", now.Add(500*time.Millisecond))
	healthy.State = Closing
	healthy.Control.Uncertain = true
	healthy.Control.Deadline = now.Add(250 * time.Millisecond).UnixMilli()
	for _, record := range []*Record{&failed, &stale, &healthy} {
		if err := Create(ctx, tx, record, 128, 1024); err != nil {
			t.Fatal(err)
		}
	}
	// 原绝对期限已到期的行必须优先；健康领取中的CLOSING不能在ES在途被worker再领。
	due, err := Due(ctx, tx, now, 1)
	if err != nil || len(due) != 1 || due[0] != failed.Hash {
		t.Fatal("DUE_ORDER_OR_HEALTHY_CLAIM_INCORRECT")
	}
	// 用尚未过绝对期限的失败链验证retry公平性；已过期限的链应立即终结而不是续期。
	failed.State = Closed
	if err := Save(ctx, tx, failed); err != nil {
		t.Fatal(err)
	}
	retry := cursorFixture(t, "future-retry", now.Add(100*time.Millisecond))
	retry.State = Closing
	retry.PIT = "actual-future-retry-pit"
	retry.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, RetryAt: now.Add(50 * time.Millisecond).UnixMilli()}
	if err := Create(ctx, tx, &retry, 128, 1024); err != nil {
		t.Fatal(err)
	}
	due, err = Due(ctx, tx, now, 1)
	if err != nil || len(due) != 1 || due[0] != stale.Hash {
		t.Fatal("FAILED_FIRST_CHAIN_STARVED_LATER_DUE_CHAIN")
	}
	due, err = Due(ctx, tx, now.Add(time.Second), 16)
	if err != nil || !containsHash(due, healthy.Hash) || !containsHash(due, retry.Hash) {
		t.Fatal("ABSOLUTE_EXPIRY_OR_STALE_CLAIM_NOT_RECOVERED")
	}
	if _, err := Due(ctx, tx, now, 17); err == nil {
		t.Fatal("UNBOUNDED_CLEANUP_LIMIT_ACCEPTED")
	}
}

func containsHash(hashes [][32]byte, hash [32]byte) bool {
	for _, candidate := range hashes {
		if candidate == hash {
			return true
		}
	}
	return false
}

func TestActualCursorDeleteClosedIsBoundedAndRollbackable(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	now := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	var closed []Record
	for i := range 17 {
		record := cursorFixture(t, fmt.Sprintf("closed-%d", i), now.Add(time.Duration(i-20)*time.Second))
		record.State = Closed
		record.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
		if err := Create(ctx, tx, &record, 128, 1024); err != nil {
			t.Fatal(err)
		}
		closed = append(closed, record)
	}
	protected := []Record{cursorFixture(t, "open-expired", now.Add(-time.Second)), cursorFixture(t, "closing-expired", now.Add(-time.Second)), cursorFixture(t, "closed-live", now.Add(time.Second))}
	protected[1].State = Closing
	protected[1].Control.Uncertain = true
	protected[2].State = Closed
	protected[2].Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
	for i := range protected {
		if err := Create(ctx, tx, &protected[i], 128, 1024); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteClosed(ctx, tx, now, 16); err != nil {
		t.Fatal(err)
	}
	for i, record := range closed {
		_, found, err := Lock(ctx, tx, record.Hash)
		if err != nil || found != (i == 16) {
			t.Fatal("CLOSED_DELETE_NOT_EXACTLY_BOUNDED")
		}
	}
	for _, record := range protected {
		if _, found, err := Lock(ctx, tx, record.Hash); err != nil || !found {
			t.Fatal("CLOSED_DELETE_REMOVED_LIVE_OR_UNRELEASED_CHAIN")
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// 清理和创建共享原事务，回滚后不能留下测试链或删除已存在的产品链。
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", closed[16].Hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("CLEANUP_COMMITTED_CALLER_TRANSACTION")
	}
}

func TestActualCursorCheckRejectsUnrecoverablePersistedFacts(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	record := cursorFixture(t, "check", time.Now().Add(30*time.Second))
	if err := Create(ctx, tx, &record, 128, 1024); err != nil {
		t.Fatal(err)
	}
	if err := Check(ctx, tx); err != nil {
		t.Fatal(err)
	}
	originalControl, err := domain.EncodeControl(record.Control)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name  string
		query string
		value any
	}{
		{"unknown-version", "UPDATE cursor_registry SET last_sort=? WHERE cursor_hash=?", `{"version":2,"phase":"OPENING"}`},
		{"zero-scope", "UPDATE cursor_registry SET scope_hash=? WHERE cursor_hash=?", make([]byte, 32)},
		{"zero-query", "UPDATE cursor_registry SET query_hash=? WHERE cursor_hash=?", make([]byte, 32)},
		{"zero-session", "UPDATE cursor_registry SET session_ref=? WHERE cursor_hash=?", make([]byte, 32)},
		{"zero-generation", "UPDATE cursor_registry SET index_generation=? WHERE cursor_hash=?", uint64(0)},
		{"bad-physical", "UPDATE cursor_registry SET physical_index=? WHERE cursor_hash=?", "INVALID_PHYSICAL"},
		{"microsecond-expiry", "UPDATE cursor_registry SET expires_at=? WHERE cursor_hash=?", record.Expires.Add(time.Microsecond)},
		{"deadline-past-original-bound", "UPDATE cursor_registry SET last_sort=? WHERE cursor_hash=?", fmt.Sprintf(`{"version":1,"phase":"OPENING","nonce":"%s","deadline":%d}`, strings.Repeat("a", 64), record.Expires.UnixMilli()+1)},
		{"empty-confirmed-pit", "UPDATE cursor_registry SET last_sort=? WHERE cursor_hash=?", `{"version":1,"phase":"IDLE"}`},
		{"oversize-pit", "UPDATE cursor_registry SET pit_handle=? WHERE cursor_hash=?", strings.Repeat("p", 64*1024+1)},
		{"whitespace-pit", "UPDATE cursor_registry SET pit_handle=? WHERE cursor_hash=?", "actual pit"},
		{"control-pit", "UPDATE cursor_registry SET pit_handle=? WHERE cursor_hash=?", "actual\x7fpit"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := tx.ExecContext(ctx, mutation.query, mutation.value, record.Hash[:]); err != nil {
				t.Fatal(err)
			}
			if err := Check(ctx, tx); err == nil {
				t.Fatal("READY_ACCEPTED_UNRECOVERABLE_CURSOR_FACT")
			}
			if _, _, err := Lock(ctx, tx, record.Hash); err == nil {
				t.Fatal("LOCK_ACCEPTED_UNRECOVERABLE_CURSOR_FACT")
			}
			// 只恢复本事务内的自有夹具，不触碰任何既存产品行。
			if _, err := tx.ExecContext(ctx, `UPDATE cursor_registry SET scope_hash=?,query_hash=?,session_ref=NULL,pit_handle='',
				index_generation=?,physical_index=?,expires_at=?,last_sort=? WHERE cursor_hash=?`,
				record.ScopeHash[:], record.QueryHash[:], record.Generation, record.Physical, record.Expires, originalControl, record.Hash[:]); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActualCursorTryLockSkipsOtherConnectionAndKeepsCallerRollback(t *testing.T) {
	db := actualCursorDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked := cursorFixture(t, "locked-committed", time.Now().Add(30*time.Second))
	probe := cursorFixture(t, "uncommitted-probe", locked.Expires)
	// 第二连接必须锁到真正已提交的行；读不到另一事务未提交的INSERT不是SKIP LOCKED证据。
	// 本项仅显式提交这一条自有夹具，结束时按精确hash删除；不清理其它库行。
	t.Cleanup(func() {
		cleanup, finish := context.WithTimeout(context.Background(), 3*time.Second)
		defer finish()
		if _, err := db.ExecContext(cleanup, "DELETE FROM cursor_registry WHERE cursor_hash IN (?,?)", locked.Hash[:], probe.Hash[:]); err != nil {
			t.Error("ACTUAL_CURSOR_TRY_LOCK_FIXTURE_CLEANUP_FAILED")
		}
	})
	control, err := domain.EncodeControl(locked.Control)
	if err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO cursor_registry(cursor_hash,pit_handle,physical_index,index_generation,
		scope_hash,query_hash,session_ref,last_sort,page_size,expires_at,state) VALUES(?,?,?,?,?,?,NULL,?,?,?,?)`
	if _, err := db.ExecContext(ctx, insert, locked.Hash[:], locked.PIT, locked.Physical, locked.Generation,
		locked.ScopeHash[:], locked.QueryHash[:], control, locked.PageSize, locked.Expires, locked.State); err != nil {
		t.Fatal("ACTUAL_CURSOR_TRY_LOCK_FIXTURE_FAILED")
	}
	holder, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, found, err := Lock(ctx, holder, locked.Hash); err != nil || !found {
		t.Fatal("ACTUAL_CURSOR_OTHER_CONNECTION_LOCK_FAILED")
	}
	caller, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer caller.Rollback()
	probeControl, err := domain.EncodeControl(probe.Control)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := caller.ExecContext(ctx, insert, probe.Hash[:], probe.PIT, probe.Physical, probe.Generation,
		probe.ScopeHash[:], probe.QueryHash[:], probeControl, probe.PageSize, probe.Expires, probe.State); err != nil {
		t.Fatal(err)
	}
	// 首行被其它连接锁住时须立即跳过，不能消耗worker每项2秒的业务领取预算。
	attempt, endAttempt := context.WithTimeout(ctx, 500*time.Millisecond)
	start := time.Now()
	got, claimed, err := TryLock(attempt, caller, locked.Hash)
	elapsed := time.Since(start)
	endAttempt()
	if err != nil || claimed || got.Hash != [32]byte{} || elapsed >= 500*time.Millisecond {
		t.Fatal("TRY_LOCK_WAITED_FOR_OTHER_CONNECTION_OR_CLAIMED_LOCKED_CHAIN")
	}
	if _, found, err := Lock(ctx, caller, probe.Hash); err != nil || !found {
		t.Fatal("TRY_LOCK_ENDED_CALLER_TRANSACTION")
	}
	if err := holder.Rollback(); err != nil {
		t.Fatal(err)
	}
	got, claimed, err = TryLock(ctx, caller, locked.Hash)
	if err != nil || !claimed || got.Hash != locked.Hash {
		t.Fatal("TRY_LOCK_COULD_NOT_CLAIM_RELEASED_EXISTING_CHAIN")
	}
	if err := caller.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", probe.Hash[:]).Scan(&count); err != nil || count != 0 {
		t.Fatal("TRY_LOCK_COMMITTED_CALLER_PROBE")
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", locked.Hash[:]).Scan(&count); err != nil || count != 1 {
		t.Fatal("TRY_LOCK_DELETED_LOCKED_EXISTING_CHAIN")
	}
	t.Logf("locked exact-hash claim skipped in %dms; holder rollback permits same caller to claim", elapsed.Milliseconds())
}

func lockedCapacityBaseline(t *testing.T, ctx context.Context, tx *sql.Tx) (int, int) {
	t.Helper()
	var open, total int
	// 容量测试会暂时锁住现存登记事实，避免worker在基线与填充之间释放产品链导致
	// 满界断言漂移；只读这些产品行，不提交、不修改或删除它们。
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(state IN ('OPEN','CLOSING')),0),COUNT(*) FROM cursor_registry FOR UPDATE").Scan(&open, &total); err != nil {
		t.Fatal("ACTUAL_CURSOR_CAPACITY_BASELINE_FAILED")
	}
	if open > 128 || total > 1024 || open > total {
		t.Fatal("ACTUAL_CURSOR_BASELINE_ALREADY_EXCEEDS_HARD_QUOTA")
	}
	return open, total
}

func assertCapacityRollback(t *testing.T, ctx context.Context, db *sql.DB, tx *sql.Tx, physical string) {
	t.Helper()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE physical_index IN (?,?)", physical, physical+"-g2").Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("CAPACITY_FIXTURES_ESCAPED_CALLER_ROLLBACK")
	}
}

func TestActualCursorHard128UnreleasedBoundaryKeepsRollback(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	baselineOpen, baselineRows := lockedCapacityBaseline(t, ctx, tx)
	needed := 128 - baselineOpen
	if needed < 1 || baselineRows+needed > 1024 {
		// 至少需要一个自有空位才能独立证明其它代次的CLOSING也计入128；不删产品
		// 行腾位，更不把无法构造的满界场景Skip为成功。
		t.Fatal("ACTUAL_CURSOR_128_BOUNDARY_FIXTURE_CAPACITY_REQUIRED")
	}
	physical := fmt.Sprintf("actual-cursor-cap128-%d", time.Now().UnixNano())
	expires := time.Now().UTC().Add(30 * time.Second).Truncate(time.Millisecond)
	start := time.Now()
	var otherGeneration Record
	for i := range needed {
		record := cursorFixture(t, fmt.Sprintf("hard128-%d", i), expires)
		record.Physical = physical
		if i == 0 {
			record.Physical = physical + "-g2"
			record.Generation = 2
		}
		if err := Create(ctx, tx, &record, 128, 1024); err != nil {
			t.Fatal("HARD_128_FILL_FAILED", err)
		}
		if i == 0 {
			// 真实经历OPENING→CLOSING但未知句柄仍占额度；其它代次不能被当前代次
			// 的配额查询漏掉，全部事实仍在这一笔未提交事务内。
			record.State = Closing
			record.Control.Uncertain = true
			if err := Save(ctx, tx, record); err != nil {
				t.Fatal(err)
			}
			otherGeneration = record
		}
	}
	open, total := registryCounts(t, ctx, tx)
	if open != 128 || total != baselineRows+needed {
		t.Fatal("UNRELEASED_QUOTA_DID_NOT_REACH_EXACT_128")
	}
	blocked := cursorFixture(t, "hard128-rejected", expires)
	blocked.Physical = physical
	if err := Create(ctx, tx, &blocked, 128, 1024); err == nil || err.Error() != "SEARCH_CURSOR_QUOTA_EXCEEDED" {
		t.Fatal("129TH_UNRELEASED_CHAIN_NOT_REJECTED_BY_QUOTA")
	}
	afterOpen, afterRows := registryCounts(t, ctx, tx)
	if afterOpen != open || afterRows != total {
		t.Fatal("REJECTED_129TH_CHAIN_CHANGED_REGISTRY_COUNTS")
	}
	if _, found, err := Lock(ctx, tx, blocked.Hash); err != nil || found {
		t.Fatal("REJECTED_129TH_CHAIN_WAS_REGISTERED")
	}
	if err := Check(ctx, tx); err != nil {
		t.Fatal("EXACT_128_VALID_REGISTRY_REJECTED", err)
	}
	// 只释放自有其它代次链，验证恢复一个空位后同一被拒输入可原样登记；不借
	// 删除行释放额度，关闭行仍应计入1024总量。
	otherGeneration.State = Closed
	otherGeneration.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
	if err := Save(ctx, tx, otherGeneration); err != nil {
		t.Fatal(err)
	}
	if total < 1024 {
		if err := Create(ctx, tx, &blocked, 128, 1024); err != nil {
			t.Fatal("RELEASED_OTHER_GENERATION_SLOT_NOT_REUSABLE", err)
		}
		afterOpen, afterRows = registryCounts(t, ctx, tx)
		if afterOpen != 128 || afterRows != total+1 {
			t.Fatal("REUSED_SLOT_DID_NOT_KEEP_CLOSED_ROW_IN_TOTAL")
		}
	}
	assertCapacityRollback(t, ctx, db, tx, physical)
	t.Logf("hard128 baseline_unreleased=%d baseline_rows=%d fixture_rows=%d fixture_ms=%d; all capacity facts rolled back", baselineOpen, baselineRows, needed, time.Since(start).Milliseconds())
}

func TestActualCursorHard1024RegistryBoundaryKeepsRollback(t *testing.T) {
	db := actualCursorDB(t)
	ctx, tx := actualCursorTx(t, db)
	baselineOpen, baselineRows := lockedCapacityBaseline(t, ctx, tx)
	needed := 1024 - baselineRows
	if needed < 2 || baselineOpen > 126 {
		// 需要一个其它代次CLOSING以及一个临时OPENING空位，逐链经过真实Create/Save
		// 后保留CLOSED行；不伪造批量SQL、不清理现存产品链以腾容量。
		t.Fatal("ACTUAL_CURSOR_1024_BOUNDARY_FIXTURE_CAPACITY_REQUIRED")
	}
	physical := fmt.Sprintf("actual-cursor-cap1024-%d", time.Now().UnixNano())
	expires := time.Now().UTC().Add(30 * time.Second).Truncate(time.Millisecond)
	start := time.Now()
	for i := range needed {
		record := cursorFixture(t, fmt.Sprintf("hard1024-%d", i), expires)
		record.Physical = physical
		if i == 0 {
			record.Physical = physical + "-g2"
			record.Generation = 2
		}
		if err := Create(ctx, tx, &record, 128, 1024); err != nil {
			t.Fatal("HARD_1024_FILL_FAILED", err)
		}
		if i == 0 {
			record.State = Closing
			record.Control.Uncertain = true
		} else {
			record.State = Closed
			record.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
		}
		if err := Save(ctx, tx, record); err != nil {
			t.Fatal(err)
		}
	}
	open, total := registryCounts(t, ctx, tx)
	if open != baselineOpen+1 || total != 1024 {
		t.Fatal("TOTAL_REGISTRY_DID_NOT_REACH_EXACT_1024")
	}
	// 未释放链数严格低于128，此拒绝只能来自1024总行界，CLOSED也不能被漏计。
	blocked := cursorFixture(t, "hard1024-rejected", expires)
	blocked.Physical = physical
	if err := Create(ctx, tx, &blocked, 128, 1024); err == nil || err.Error() != "SEARCH_CURSOR_QUOTA_EXCEEDED" {
		t.Fatal("1025TH_REGISTRY_ROW_NOT_REJECTED_BY_TOTAL_QUOTA")
	}
	afterOpen, afterRows := registryCounts(t, ctx, tx)
	if afterOpen != open || afterRows != total {
		t.Fatal("REJECTED_1025TH_ROW_CHANGED_REGISTRY_COUNTS")
	}
	if _, found, err := Lock(ctx, tx, blocked.Hash); err != nil || found {
		t.Fatal("REJECTED_1025TH_ROW_WAS_REGISTERED")
	}
	if err := Check(ctx, tx); err != nil {
		t.Fatal("EXACT_1024_VALID_REGISTRY_REJECTED", err)
	}
	assertCapacityRollback(t, ctx, db, tx, physical)
	t.Logf("hard1024 baseline_unreleased=%d baseline_rows=%d fixture_rows=%d fixture_ms=%d; all capacity facts rolled back", baselineOpen, baselineRows, needed, time.Since(start).Milliseconds())
}
