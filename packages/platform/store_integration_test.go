package platform

import (
	"context"
	"os"
	"testing"
	"time"
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG")
	if file == "" {
		t.Skip("明确配置隔离开发数据库后运行集成测试")
	}
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.DB.Close() })
	return store
}
func TestReadyRefusesChangedMigrationChecksum(t *testing.T) {
	// 真实库已安装，但服务持有另一摘要时必须拒绝ready，不能把连通当成就绪。
	store := integrationStore(t)
	store.ExpectedSQLSHA = "0000000000000000000000000000000000000000000000000000000000000000"
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if store.CheckReady(ctx) == nil {
		t.Fatal("changed schema checksum accepted as ready")
	}
}
func TestStoreCloseDoesNotPoisonNewConnection(t *testing.T) {
	// 一个Store拥有自己的连接池，关闭它以后重新打开必须还能真实查询数据库。
	store := integrationStore(t)
	_ = store.DB.Close()
	cfg, err := LoadConfig(os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	next, err := OpenStore(cfg)
	if err != nil {
		t.Fatal("closed pool was reused", err)
	}
	defer next.DB.Close()
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if next.DB.PingContext(ctx) != nil {
		t.Fatal("new store inherited a closed pool")
	}
}
func TestReadyRefusesUnrecoveredDurableWork(t *testing.T) {
	// 模块尚无恢复worker时，未知任务应保持不可用，不能跳过恢复直接接受新业务。
	store := integrationStore(t)
	ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	key := "bootstrap-ready-test-" + NewTraceID()
	_, err := store.DB.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('BOOTSTRAP_TEST',?,'{}','UNKNOWN',UTC_TIMESTAMP(6))", key)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.ExecContext(context.Background(), "DELETE FROM durable_task WHERE kind='BOOTSTRAP_TEST' AND business_key=?", key)
	if store.CheckReady(ctx) == nil {
		t.Fatal("unrecovered durable work accepted as ready")
	}
}
func TestReadyAcceptsInstalledEmptyBaseline(t *testing.T) {
	store := integrationStore(t)
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if err := store.CheckReady(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestReadyRefusesUnrestoredBlockedGate(t *testing.T) {
	// 不依赖任务表，单独持久化的冻结gate也必须进入恢复屏障。
	store := integrationStore(t)
	key := "bootstrap-gate-test-" + NewTraceID()
	_, err := store.DB.Exec("INSERT INTO access_gate(scope_key,state) VALUES(?,'BLOCKED')", key)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Exec("DELETE FROM access_gate WHERE scope_key=?", key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if store.CheckReady(ctx) == nil {
		t.Fatal("blocked gate accepted before restoration")
	}
}
func TestReadyRestoresAcknowledgedClosedWatermark(t *testing.T) {
	store := integrationStore(t)
	key := "known-closed-" + NewTraceID()
	id := "bootstrap-close-" + NewTraceID()
	_, err := store.DB.Exec("INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?, ?,1,'{}','SUCCEEDED','[]')", id, key)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Exec("DELETE FROM authorization_change WHERE revocation_id=?", id)
	_, err = store.DB.Exec("INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,1,'BLOCKED',?)", key, id)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Exec("DELETE FROM access_gate WHERE scope_key=?", key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = store.CheckReady(ctx); err != nil {
		t.Fatal("terminal durable closure should restore its tombstone", err)
	}
}

func TestReadyRefusesMissingOrChangedAdditionalMigrationExpectation(t *testing.T) {
	store := integrationStore(t)
	configured := store.ExpectedAdditionalMigrations
	if configured[2] == "" {
		t.Fatal("ADDITIONAL_MIGRATION_CONFIGURATION_REQUIRED")
	}
	// 只改变当前测试实例的发布期望，不修改原schema_migration或共享服务配置。
	for _, expected := range []map[int]string{
		nil,
		{2: "0000000000000000000000000000000000000000000000000000000000000000"},
		{2: configured[2], 3: configured[2]},
	} {
		store.ExpectedAdditionalMigrations = expected
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if store.CheckReady(ctx) == nil {
			cancel()
			t.Fatal("unverified additional migration accepted")
		}
		cancel()
	}
	store.ExpectedAdditionalMigrations = configured
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := store.CheckReady(ctx); err != nil {
		t.Fatal("original complete migration expectation must remain ready", err)
	}
}
