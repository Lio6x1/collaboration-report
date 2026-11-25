package document

import (
	pb "collaboration/contracts/internalv1"
	"context"
	"testing"
	"time"
)

// 停止信号不等于输出结束。即使写入handler需要异步退出，关闭也必须等待done证明。
func TestLocalFenceWaitsForActualHandler(t *testing.T) {
	m := NewLocalOutputs()
	done := make(chan struct{})
	stopped := make(chan struct{})
	source := &pb.SourceEpoch{ScopeKey: "credential:7", Epoch: 4}
	m.Add("held", []*pb.SourceEpoch{source}, func() {
		select {
		case <-stopped:
		default:
			close(stopped)
		}
	}, done)
	finished := make(chan error, 1)
	go func() { finished <- m.Stop(context.Background(), source) }()
	<-stopped
	select {
	case <-finished:
		t.Fatal("ACK before handler settled")
	default:
	}
	close(done)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
func TestLocalFenceTimeoutIsUnknown(t *testing.T) {
	m := NewLocalOutputs()
	m.Add("held", []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 2}}, func() {}, make(chan struct{}))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if m.Stop(ctx, &pb.SourceEpoch{ScopeKey: "identity:7", Epoch: 2}) == nil {
		t.Fatal("unknown output cannot acknowledge")
	}
}
