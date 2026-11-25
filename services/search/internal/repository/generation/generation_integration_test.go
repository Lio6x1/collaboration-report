package generation

import (
	"collaboration/platform"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualCurrentGenerationLockBlocksCutoverAndRejectsSwitching(t *testing.T) {
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
	ctx := context.Background()
	alias := fmt.Sprintf("actual-generation-%d", time.Now().UnixNano())
	physical := alias + "-g1"
	t.Cleanup(func() {
		if _, err := store.DB.ExecContext(ctx, "DELETE FROM rebuild_job WHERE target_index=?", physical+"-next"); err != nil {
			t.Error("ACTUAL_JOB_CLEANUP_FAILED")
		}
		if _, err := store.DB.ExecContext(ctx, "DELETE FROM index_generation WHERE alias_name=?", alias); err != nil {
			t.Error("ACTUAL_GENERATION_CLEANUP_FAILED")
		}
	})
	if _, err := store.DB.ExecContext(ctx, "INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)", alias, physical); err != nil {
		t.Fatal("ACTUAL_GENERATION_FIXTURE_FAILED")
	}
	active, err := store.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer active.Rollback()
	target, err := LockCurrent(ctx, active, alias)
	if err != nil || target.Physical != physical || target.Generation != 1 {
		t.Fatal("ACTUAL_GENERATION_NOT_BOUND")
	}
	// 消费持有共享代次锁时，切换的独占UPDATE必须真实阻塞，不能只靠进程内mutex。
	cutover, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	// 取消autocommit语句的客户端等待不证明服务端没有后来提交；夹具写入必须
	// 处在明确不提交的事务内，避免放开共享锁后留下未知的测试代次更新。
	writer, err := store.DB.BeginTx(cutover, nil)
	if err != nil {
		t.Fatal("ACTUAL_CUTOVER_TRANSACTION_UNAVAILABLE")
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(cutover, "UPDATE index_generation SET physical_index=?,generation=2,revision=2 WHERE alias_name=?", physical+"-next", alias); err == nil || cutover.Err() == nil {
		t.Fatal("CUTOVER_PASSED_LIVE_CONSUMER_GENERATION_LOCK")
	}
	writer.Rollback()
	active.Rollback()
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO rebuild_job(state,snapshot_ref,watermark,target_index,source_index,generation)
		VALUES('SWITCHING',?,'{}',?,?,2)`, alias, physical+"-next", physical); err != nil {
		t.Fatal("ACTUAL_SWITCHING_FIXTURE_FAILED")
	}
	checking, err := store.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer checking.Rollback()
	if _, err := LockCurrent(ctx, checking, alias); err == nil || err.Error() != "SEARCH_GENERATION_RECOVERY_REQUIRED" {
		t.Fatal("DURABLE_SWITCHING_WAS_TREATED_AS_READY")
	}
}
