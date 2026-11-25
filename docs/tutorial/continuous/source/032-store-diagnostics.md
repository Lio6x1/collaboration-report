# packages/platform/store_diagnostics.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/032-store-diagnostics.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`runPoolDiagnostics`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package platform

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// 只在明确排查连接耗尽时输出连接池聚合量，不记录DSN、SQL、用户或业务载荷。
// 数据库仍由Store统一持有；观测不能建立另一个连接池或影响事务期限。
func (s *Store) runPoolDiagnostics(ctx context.Context) {
	if os.Getenv("COLLAB_SQL_POOL_DIAGNOSTICS") != "1" {
		return
	}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := s.DB.Stats()
				slog.Info("sql_pool", "role", s.Role,
					"open", stats.OpenConnections, "in_use", stats.InUse, "idle", stats.Idle,
					"wait_count", stats.WaitCount, "idle_closed", stats.MaxIdleClosed,
					"idle_time_closed", stats.MaxIdleTimeClosed, "lifetime_closed", stats.MaxLifetimeClosed)
			}
		}
	}()
}
````

UTF-8/LF全文SHA-256：`495783c3ba92e7cbed911c23751a0072046cdf9f931c1b9c63ac0420c25613c0`。[参考实现](../../../../packages/platform/store_diagnostics.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
