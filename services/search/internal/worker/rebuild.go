package worker

import (
	"context"
	"log/slog"
	"regexp"
	"time"
)

type rebuilder interface {
	Step(context.Context) error
	Close()
}

// RunAfterRecovery在启动时先等待原SWITCHING完成，再让普通消费者初始化。
// 切换失败持续保留屏障，不通过空成功或另建任务绕过；退出时关闭尚未启动的消费者。
func RunAfterRecovery(ctx context.Context, consumer Consumer, ready func(context.Context) error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		check, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := ready(check)
		cancel()
		if err == nil {
			Run(ctx, consumer)
			return
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	consumer.Close()
}

var rebuildCode = regexp.MustCompile(`^SEARCH_[A-Z0-9_]{1,56}$`)

// 本实例只运行一个重建循环；数据库逐文档/记录状态锁同时保护跨实例重复调用。
// 故障只暂停原游标，不清空任务、不替换快照，也不影响仍服务旧索引的普通消费者。
func RunRebuild(ctx context.Context, service rebuilder) {
	defer service.Close()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := service.Step(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			last = ""
			continue
		}
		code := "SEARCH_REBUILD_RETRY_REQUIRED"
		if rebuildCode.MatchString(err.Error()) {
			code = err.Error()
		}
		if code != last {
			slog.Warn("rebuild_pending", "code", code)
			last = code
		}
	}
}
