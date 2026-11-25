package cursor

import (
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func TestActualCursorSchedulingSkipsSixteenLockedRows(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(fmt.Sprintf("closed=%t", closed), func(t *testing.T) {
			db := actualCursorDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			// 使用未来事实和显式调度时钟，产品worker不会与本反例争抢；没有修改产品期限。
			base := time.Now().Add(time.Minute).UTC().Truncate(time.Millisecond)
			tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			alias := fmt.Sprintf("actual-scheduler-%d", time.Now().UnixNano())
			if _, err := tx.ExecContext(ctx, "INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)", alias, alias+"-g1"); err != nil {
				t.Fatal(err)
			}
			var lockedAlias string
			if err := tx.QueryRowContext(ctx, "SELECT alias_name FROM index_generation WHERE alias_name=? FOR UPDATE", alias).Scan(&lockedAlias); err != nil {
				t.Fatal(err)
			}
			var records []Record
			for i := range 17 {
				r := cursorFixture(t, fmt.Sprintf("scheduler-%d", i), base.Add(time.Duration(i)*time.Millisecond))
				if closed {
					r.State = Closed
					r.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle}
				}
				if err := Create(ctx, tx, &r, 128, 1024); err != nil {
					t.Fatal(err)
				}
				records = append(records, r)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				for _, r := range records {
					if _, err := db.Exec("DELETE FROM cursor_registry WHERE cursor_hash=?", r.Hash[:]); err != nil {
						t.Error("ACTUAL_SCHEDULER_FIXTURE_CLEANUP_FAILED")
					}
				}
				if _, err := db.Exec("DELETE FROM index_generation WHERE alias_name=?", alias); err != nil {
					t.Error("ACTUAL_SCHEDULER_GENERATION_CLEANUP_FAILED")
				}
			})
			holder, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback()
			for _, r := range records[:16] {
				if _, found, err := Lock(ctx, holder, r.Hash); err != nil || !found {
					t.Fatal("ACTUAL_SCHEDULER_LOCK_FAILED", err)
				}
			}
			attempt, finish := context.WithTimeout(ctx, 500*time.Millisecond)
			defer finish()
			caller, err := db.BeginTx(attempt, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer caller.Rollback()
			now := base.Add(time.Second)
			if closed {
				if err := DeleteClosed(attempt, caller, now, 16); err != nil {
					t.Fatal("LOCKED_CLOSED_PREFIX_BLOCKED_LATER_DELETE", err)
				}
				if _, found, err := Lock(attempt, caller, records[16].Hash); err != nil || found {
					t.Fatal("LOCKED_CLOSED_PREFIX_STARVED_SEVENTEENTH", err)
				}
			} else {
				due, err := Due(attempt, caller, now, 16)
				if err != nil || !containsHash(due, records[16].Hash) {
					t.Fatal("LOCKED_DUE_PREFIX_STARVED_SEVENTEENTH", err)
				}
				for _, r := range records[:16] {
					if containsHash(due, r.Hash) {
						t.Fatal("DUE_SELECTED_LOCKED_CHAIN")
					}
				}
			}
			// 调度领取与删除均属于原caller事务，回滚不会删除第17行或释放holder锁。
			if err := caller.Rollback(); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=?", records[16].Hash[:]).Scan(&count); err != nil || count != 1 {
				t.Fatal("SCHEDULER_COMMITTED_CALLER_TRANSACTION", err)
			}
		})
	}
}
