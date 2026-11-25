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
