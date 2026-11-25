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
