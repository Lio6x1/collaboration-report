package worker

import (
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"github.com/go-sql-driver/mysql"
	"log/slog"
	"regexp"
	"strconv"
	"time"
)

// 服务装配显式启动原号恢复；HTTP 丢回复和实例退出不依赖用户重新接受。
func RunPersonalTransfers(ctx context.Context, usecase *logic.PersonalTransfers) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastCode := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := usecase.Recover(attempt)
			cancel()
			if err != nil && !errors.Is(err, logic.ErrPersonalTransferIdle) && ctx.Err() == nil {
				code := transferRetryCode(err)
				if code != lastCode {
					slog.Warn("PERSONAL_TRANSFER_RECOVERY_PENDING", "code", code)
					lastCode = code
				}
			} else if err == nil {
				lastCode = ""
			}
		}
	}
}

var stableTransferError = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,95}$`)

// 只暴露稳定业务码/SQL错误编号，不能把固定身份、连接信息或SQL参数写入日志。
func transferRetryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var sqlError *mysql.MySQLError
	if errors.As(err, &sqlError) {
		return "SQL_ERROR_" + strconv.Itoa(int(sqlError.Number))
	}
	if stableTransferError.MatchString(err.Error()) {
		return err.Error()
	}
	return "PERSONAL_TRANSFER_RETRY"
}
