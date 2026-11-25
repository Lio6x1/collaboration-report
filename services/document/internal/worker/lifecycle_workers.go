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
