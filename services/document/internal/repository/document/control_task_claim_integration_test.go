package document

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestActualControlTaskClaimsLargePayloadInSameTransaction(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Skip("需要显式指定本项目隔离文档数据库")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	ctx := context.Background()
	key := fmt.Sprintf("control-claim-%d", time.Now().UnixNano())
	const kind = "CONTROL_CLAIM_TEST"
	raw, err := json.Marshal(map[string]string{"body": strings.Repeat("a", 4<<20)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))", kind, key, key, string(raw)); err != nil {
		t.Fatal(err)
	}
	// 只清理本测试自己创建的无业务来源任务；不改产品任务、版本、租约或任何安装事实。
	defer store.DB.ExecContext(ctx, "DELETE FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?", kind, key, key)
	var original []byte
	if err = store.DB.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind=? AND business_key=?", kind, key).Scan(&original); err != nil {
		t.Fatal(err)
	}
	secondKey := key + "-second"
	if _, err = store.DB.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,'{}','QUEUED',UTC_TIMESTAMP(6))", kind, secondKey, secondKey); err != nil {
		t.Fatal(err)
	}
	defer store.DB.ExecContext(ctx, "DELETE FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?", kind, secondKey, secondKey)
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// 巨大正文不能进入按ID排序的工作缓冲；原任务、代次和payload仍必须在同一事务精确领取。
	task, err := claimOperationTask(ctx, tx, "large-payload-test", kind, kind)
	if err != nil {
		t.Fatal(err)
	}
	if task.Operation != key || task.Kind != kind || !bytes.Equal(task.Payload, original) || task.Generation != 2 {
		t.Fatal("claimed source, generation or payload changed")
	}

	// 领取任务只应锁自己的事实。另一个请求的实际OUTPUT_LEASE登记不能等这份大正文事务结束。
	// 第二个事务始终回滚，因此不会留下没有真实handler的输出租约。
	other, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback()
	writeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	_, err = other.ExecContext(writeCtx, "INSERT INTO durable_task(kind,business_key,payload,state,instance_id,lease_until,retry_at) VALUES('OUTPUT_LEASE',?,'{}','RUNNING','claim-isolation-test',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),UTC_TIMESTAMP(6))", key+"-output")
	if err != nil {
		t.Fatal("unrelated output registration waits for the claimed task", err)
	}
	// 外部快照仍能看见第一行QUEUED；当前读必须跳过其实际行锁并领取第二行，不能重复领取。
	second, err := claimOperationTask(writeCtx, other, "second-claim-test", kind, kind)
	if err != nil {
		t.Fatal("locked candidate prevented progress", err)
	}
	if second.Operation != secondKey || second.Generation != 2 || !bytes.Equal(second.Payload, []byte("{}")) {
		t.Fatal("claimed the locked task or changed the second source")
	}
}
