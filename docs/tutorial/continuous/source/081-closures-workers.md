# services/account/internal/worker/closures_workers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/081-closures-workers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`accountCloseRecovery`、`RunAccountCloseRecovery`、`RunOrganizationCloseRecovery`、`sessionClosures`、`RunSessionClosures`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`dcf6fa99f710139c7f79b98b1b569bcb4f471f5246e4f5c3977baaf84367d849`。[参考实现](../../../../services/account/internal/worker/closures_workers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
