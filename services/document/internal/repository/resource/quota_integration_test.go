package resource

import (
	"collaboration/platform"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualQuotaReservationSettlementAndLimit(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_INVALID")
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_DATABASE_UNAVAILABLE")
	}
	defer store.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("ACTUAL_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	var space uint64
	if tx.QueryRowContext(ctx, `SELECT s.id FROM space s LEFT JOIN quota_account q ON q.space_id=s.id WHERE q.space_id IS NULL ORDER BY s.id LIMIT 1`).Scan(&space) != nil {
		t.Fatal("ACTUAL_UNMETERED_FIXTURE_SPACE_REQUIRED")
	}
	first := fmt.Sprintf("quota-first-%d", time.Now().UnixNano())
	second := first + "-second"
	lock := func() Quota {
		t.Helper()
		q, err := LockQuota(ctx, tx, space, 100)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	expires := time.Now().Add(time.Minute)
	q := lock()
	must(ReserveQuota(ctx, tx, q, first, 60, q.Revision, expires))
	// 原号重试不能加倍占用；余额不足必须拒绝，不能靠uint64回绕绕过限制。
	q = lock()
	must(ReserveQuota(ctx, tx, q, first, 60, 1, expires))
	if q.Reserved != 60 || q.Used != 0 {
		t.Fatal("QUOTA_RESERVATION_NOT_DURABLE_IN_TRANSACTION")
	}
	if err := ReserveQuota(ctx, tx, q, second, 41, q.Revision, expires); err == nil || err.Error() != "QUOTA_EXCEEDED" {
		t.Fatal("QUOTA_LIMIT_BYPASSED")
	}
	must(SettleQuota(ctx, tx, q, first, true))
	q = lock()
	if q.Used != 60 || q.Reserved != 0 {
		t.Fatal("QUOTA_COMMIT_NOT_SETTLED")
	}
	must(SettleQuota(ctx, tx, q, first, true))
	must(ReserveQuota(ctx, tx, q, second, 40, q.Revision, expires))
	q = lock()
	must(SettleQuota(ctx, tx, q, second, false))
	q = lock()
	if q.Used != 60 || q.Reserved != 0 {
		t.Fatal("QUOTA_RELEASE_NOT_SETTLED")
	}
	if err := ReserveQuota(ctx, tx, q, second, 40, q.Revision, expires); err == nil || err.Error() != "QUOTA_RESERVATION_CLOSED" {
		t.Fatal("CANCELLED_QUOTA_RESERVATION_REVIVED")
	}
	// 本场景只借用现有space外键，所有配额事实回滚，不修改该空间实际容量。
	must(tx.Rollback())
}
