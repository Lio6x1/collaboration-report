package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type blockingExportQueue struct {
	active  atomic.Int32
	started chan struct{}
}

func (*blockingExportQueue) WorkerCount() int { return 2 }

func (q *blockingExportQueue) Recover(ctx context.Context) error {
	q.active.Add(1)
	defer q.active.Add(-1)
	q.started <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}

// 长转换必须真的并行，但只允许配置数量的常驻worker；取消后等待它们退出。
func TestExportPoolBoundsConcurrencyAndJoinsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := &blockingExportQueue{started: make(chan struct{}, 8)}
	done := make(chan struct{})
	go func() {
		RunExports(ctx, queue)
		close(done)
	}()
	for range 2 {
		select {
		case <-queue.started:
		case <-time.After(3 * time.Second):
			t.Fatal("two workers did not enter conversion concurrently")
		}
	}
	if got := queue.active.Load(); got != 2 {
		t.Fatalf("active workers = %d, want 2", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pool did not join cancelled workers")
	}
	if got := queue.active.Load(); got != 0 {
		t.Fatalf("active after shutdown = %d", got)
	}
	if len(queue.started) != 0 {
		t.Fatal("pool started more than the configured workers")
	}
}
