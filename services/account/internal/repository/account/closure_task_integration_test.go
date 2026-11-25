package account

import (
	"bytes"
	"collaboration/platform"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualClosureClaimsDoNotLockOtherCandidatesOrRegistration(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG")
	if file == "" {
		t.Skip("需要显式指定本项目隔离账号数据库")
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
	const kind = "CLOSURE_CLAIM_TEST"
	key := fmt.Sprintf("closure-claim-%d", time.Now().UnixNano())
	for _, value := range []string{key, key + "-second"} {
		if _, err = store.DB.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(?,?,'{}','QUEUED',UTC_TIMESTAMP(6))", kind, value); err != nil {
			t.Fatal(err)
		}
		defer store.DB.ExecContext(ctx, "DELETE FROM durable_task WHERE kind=? AND business_key=?", kind, value)
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	first, err := claimClosureTask(ctx, tx, []string{kind})
	if err != nil || first.OperationID != key {
		t.Fatal("未领取原首项关闭候选")
	}
	other, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback()
	bounded, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	// 首个领取事务保持实际行锁；另一个worker必须能跳过它领取第二个原号。
	second, err := claimClosureTask(bounded, other, []string{kind})
	if err != nil || second.OperationID != key+"-second" || !bytes.Equal(second.Payload, []byte("{}")) {
		t.Fatal("首项领取锁住了另一个独立候选")
	}
	// 另一种类的实际登记不能等待这个领取事务；写入在本测试事务中全部回滚。
	if _, err = other.ExecContext(bounded, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('CLOSURE_REGISTRATION_TEST',?,'{}','QUEUED',UTC_TIMESTAMP(6))", key+"-registration"); err != nil {
		t.Fatal("独立登记被领取范围锁阻塞")
	}
}
