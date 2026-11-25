// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	"collaboration/platform"
	"context"
	"sync"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

type accountCloseRecovery interface{ RecoverOne(context.Context, string) }

// 每种流程一个独立有限领取者。慢取消不阻塞其他账号的排空，两者共用服务退出上下文。
func RunAccountCloseRecovery(ctx context.Context, service accountCloseRecovery) {
	instance := platform.NewTraceID()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.RecoverOne(ctx, instance)
		}
	}
}

// organization_close.go：保留原实现，按业务合并文件。

// 独立恢复器使用原组织任务领取，不与账号注销worker共享任务种类。
func RunOrganizationCloseRecovery(ctx context.Context, service accountCloseRecovery) {
	RunAccountCloseRecovery(ctx, service)
}

// session_closure.go：保留原实现，按业务合并文件。

type sessionClosures interface {
	EnqueueExpired(context.Context)
	RecoverOne(context.Context, string)
}

func RunSessionClosures(ctx context.Context, service sessionClosures) {
	// 到期登记不等待下游关闭：浏览器离线产生的一批原号应先持久入队。
	// 固定4个领取者使用既有SKIP LOCKED、有限租约及真实ACK逻辑，限制SQL/RPC压力；
	// 任一不可达任务只占用一个worker，不能阻塞其他独立范围的恢复。
	var workers sync.WaitGroup
	for n := 0; n < 4; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			instance := platform.NewTraceID()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					service.RecoverOne(ctx, instance)
				}
			}
		}()
	}
	defer workers.Wait()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.EnqueueExpired(ctx)
		}
	}
}
