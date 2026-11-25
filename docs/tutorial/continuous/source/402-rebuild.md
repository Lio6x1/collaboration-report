# services/search/internal/worker/rebuild.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/402-rebuild.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`rebuilder`、`RunAfterRecovery`、`RunRebuild`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`ae4eefac5fdfea6f7b1e60132c8e449e92ad6d426a661f5cdd1a73a46220cca9`。[参考实现](../../../../services/search/internal/worker/rebuild.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
