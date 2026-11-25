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
