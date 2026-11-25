# services/search/internal/worker/consumer.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/400-consumer.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`Consumer`、`Run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package worker

import (
	"context"
	"log/slog"
	"regexp"
	"time"
)

type Consumer interface {
	Step(context.Context) error
	Close() error
}

var safeCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// 每次失败继续原在途记录；退避有上限并受生命周期context取消。
// 只在错误码变化、恢复或退出时写固定日志，不输出消息正文或第三方错误。
func Run(ctx context.Context, consumer Consumer) {
	defer consumer.Close()
	var previous string
	for ctx.Err() == nil {
		err := consumer.Step(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			if previous != "" {
				slog.Info("search_consumer_recovered")
				previous = ""
			}
			continue
		}
		code := err.Error()
		if !safeCode.MatchString(code) {
			code = "SEARCH_CONSUMER_UNAVAILABLE"
		}
		if code != previous {
			slog.Warn("search_consumer_retry", "code", code)
			previous = code
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
````

UTF-8/LF全文SHA-256：`999724eb556baf281b58fa277391c6650c36643bc120e5a2739db914268005ba`。[参考实现](../../../../services/search/internal/worker/consumer.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
