package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type blockedClosures struct {
	started  chan string
	enqueues atomic.Int32
	active   atomic.Int32
	max      atomic.Int32
}

func (s *blockedClosures) EnqueueExpired(context.Context) { s.enqueues.Add(1) }
func (s *blockedClosures) RecoverOne(ctx context.Context, owner string) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	for old := s.max.Load(); active > old && !s.max.CompareAndSwap(old, active); old = s.max.Load() {
	}
	s.started <- owner
	<-ctx.Done()
}

func TestSessionClosureBacklogUsesBoundedIndependentOwners(t *testing.T) {
	// 一项下游未知不能挡住其他独立原号；每个领取者有自己的持久租约身份。
	service := &blockedClosures{started: make(chan string, 16)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunSessionClosures(ctx, service); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("关闭调度器未等待受控worker退出")
		}
	}()
	owners := map[string]bool{}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for len(owners) < 4 {
		select {
		case owner := <-service.started:
			if owner == "" || owners[owner] {
				t.Fatal("并行领取者必须使用独立非空身份")
			}
			owners[owner] = true
		case <-deadline.C:
			t.Fatal("单项关闭阻塞了其他原号的恢复")
		}
	}
	time.Sleep(250 * time.Millisecond)
	if service.max.Load() != 4 || service.enqueues.Load() < 2 {
		t.Fatal("关闭并发必须有界，且原会话到期登记不能被下游未知阻塞")
	}
}
