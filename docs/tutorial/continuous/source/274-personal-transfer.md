# services/document/internal/worker/personal_transfer.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/274-personal-transfer.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`RunPersonalTransfers`、`transferRetryCode`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`a3890ce5c49d02ee1704a730262b96cdaa8134277eaa40c0b570c89de6ad1367`。[参考实现](../../../../services/document/internal/worker/personal_transfer.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
