package platform

import (
	"context"
	"errors"
	"testing"
)

func TestReadyDelegatesOnlyRegisteredDurableWorkflow(t *testing.T) {
	store := integrationStore(t)
	key := "managed-ready-" + NewTraceID()
	if _, err := store.DB.Exec("INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('MANAGED_READY_TEST',?,'{}','QUEUED',UTC_TIMESTAMP(6))", key); err != nil {
		t.Fatal(err)
	}
	defer store.DB.Exec("DELETE FROM durable_task WHERE kind='MANAGED_READY_TEST' AND business_key=?", key)
	// 正常异步队列无需清空；但必须有已装配的恢复屏障，不能忽略未知任务或屏障的失败。
	store.ManagedTasks = []string{"MANAGED_READY_TEST"}
	store.ReadyExtra = func(context.Context) error { return nil }
	if err := store.CheckReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	store.ReadyExtra = func(context.Context) error { return errors.New("RECOVERY_BINDING_INVALID") }
	if store.CheckReady(context.Background()) == nil {
		t.Fatal("failed recovery barrier ignored")
	}
	store.ReadyExtra = nil
	if store.CheckReady(context.Background()) == nil {
		t.Fatal("unassembled recovery accepted")
	}
	store.ManagedTasks = nil
	if store.CheckReady(context.Background()) == nil {
		t.Fatal("unknown durable work accepted")
	}
}
