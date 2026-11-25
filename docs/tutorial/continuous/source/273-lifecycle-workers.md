# services/document/internal/worker/lifecycle_workers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/273-lifecycle-workers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`RunPurges`、`RunRetainedGovernance`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"github.com/zeromicro/go-zero/core/logx"
	"log/slog"
	"time"
)

// purges.go：保留原实现，按业务合并文件。

// 一轮一原号，重启后领取原持久任务；调度独立于svc的依赖装配。
// 登记与恢复分别有界，失败只输出固定阶段，不打印数据库原文或正文。
func RunPurges(ctx context.Context, purges *logic.Purges) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
		_, err := purges.RegisterExpired(attempt)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("PURGE_EXPIRY_PENDING")
		}
		attempt, cancel = context.WithTimeout(ctx, 4*time.Second)
		err = purges.Recover(attempt)
		cancel()
		if err != nil && !errors.Is(err, logic.ErrPurgeIdle) && ctx.Err() == nil {
			slog.Warn("PURGE_RECOVERY_PENDING")
		}
	}
}

// retained_governance.go：保留原实现，按业务合并文件。

// 与普通PURGE分开领取，避免共享任务筛选吞掉另一业务恢复。bootstrap显式
// 启动这个有限worker；用户不需再次DELETE才能完成已登记的原号。
func RunRetainedGovernance(ctx context.Context, usecase *logic.RetainedGovernance) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := usecase.Recover(attempt)
			cancel()
			if err != nil && !errors.Is(err, logic.ErrRetainedGovernanceIdle) && ctx.Err() == nil {
				logx.Error("RETAINED_GOVERNANCE_RECOVERY_PENDING")
			}
		}
	}
}
````

UTF-8/LF全文SHA-256：`2f200f6e98b9fdbbe11b8aa5a6536258d278c6e394909812ce2f318fd693623b`。[参考实现](../../../../services/document/internal/worker/lifecycle_workers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
