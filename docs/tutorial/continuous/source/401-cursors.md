# services/search/internal/worker/cursors.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/401-cursors.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`Cursors`、`RunCursors`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package worker

import (
	"context"
	"log/slog"
	"time"
)

type Cursors interface {
	Cleanup(context.Context) error
	Close()
}

func RunCursors(ctx context.Context, cursors Cursors) {
	defer cursors.Close()
	var previous string
	for ctx.Err() == nil {
		// 轮次串行且只领16条；前轮尚未结束不会被ticker再次启动，未知结果有原SQL重试事实。
		step, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := cursors.Cleanup(step)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			code := err.Error()
			if !safeCode.MatchString(code) {
				code = "SEARCH_CURSOR_RECOVERY_UNAVAILABLE"
			}
			if code != previous {
				slog.Warn("search_cursor_retry", "code", code)
				previous = code
			}
		} else if previous != "" {
			slog.Info("search_cursor_recovered")
			previous = ""
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
````

UTF-8/LF全文SHA-256：`12255206240b14f38e7e922b4dc0766e2605f25f6c64c4fc79bc92b549ca33e6`。[参考实现](../../../../services/search/internal/worker/cursors.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
