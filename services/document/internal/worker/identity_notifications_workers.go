// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"log/slog"
	"time"
)

// account_notifications.go：保留原实现，按业务合并文件。

func RunAccountNotifications(ctx context.Context, importer *logic.AccountNotificationImport) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 每轮只有一个最多100条的批次，网络和事务共用4秒预算，不并发积压请求。
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := importer.Recover(attempt)
			cancel()
			if err != nil && ctx.Err() == nil {
				// 不打印网络错误、SQL参数或原始事件，避免把内部身份和目标写入日志。
				slog.Warn("ACCOUNT_NOTIFICATION_IMPORT_RETRY")
			}
		}
	}
}

// authorization.go：保留原实现，按业务合并文件。

// 调度器只决定何时恢复；事务、关闭证明和业务终态由用例协调器统一维护。
func RunAuthorizations(ctx context.Context, authorizations *logic.Authorizations, recoverDeparture func(context.Context)) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			authorizations.RecoverOne(ctx)
			recoverDeparture(ctx)
		}
	}
}

// notifications.go：保留原实现，按业务合并文件。

func RunNotifications(ctx context.Context, delivery *logic.NotificationDelivery) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			// 同一4秒预算内分批清理积压；空队列立即退出，不用高频空查询消耗数据库。
			var err error
			for count := 0; count < 64 && attempt.Err() == nil; count++ {
				failure := delivery.Recover(attempt)
				if errors.Is(failure, logic.ErrNotificationIdle) {
					break
				}
				if failure != nil {
					// 失败任务已登记下一次重试时间，继续本轮其余候选，避免旧故障长期挡队。
					err = failure
				}
			}
			cancel()
			if err != nil && !errors.Is(err, logic.ErrNotificationIdle) && ctx.Err() == nil {
				slog.Warn("NOTIFICATION_DELIVERY_RETRY")
			}
		}
	}
}
