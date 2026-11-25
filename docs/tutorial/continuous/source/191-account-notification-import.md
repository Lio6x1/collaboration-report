# services/document/internal/logic/document/account_notification_import.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/191-account-notification-import.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`AccountNotificationSource`、`AccountNotificationImport`、`NewAccountNotificationImport`、`Recover`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
)

// AccountNotificationSource只传递服务间事件与确认，不承担用户请求的身份授权。
type AccountNotificationSource interface {
	Events(context.Context) ([]platform.AccountNotificationEvent, error)
	AckEvents(context.Context, []uint64) error
}

type AccountNotificationImport struct {
	db     *sql.DB
	source AccountNotificationSource
}

func NewAccountNotificationImport(db *sql.DB, source AccountNotificationSource) *AccountNotificationImport {
	return &AccountNotificationImport{db: db, source: source}
}

// Recover每轮最多导入一批；Account原始通知行号只用于内部ACK，不成为前端通知ID。
func (s *AccountNotificationImport) Recover(ctx context.Context) error {
	events, err := s.source.Events(ctx)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	if len(events) > 100 {
		return errors.New("ACCOUNT_NOTIFICATION_BATCH_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]uint64, 0, len(events))
	for _, event := range events {
		if err = notification.Insert(ctx, tx, event.RecipientID, event.EventID, event.Kind, notification.Target{Organization: domain.TextID(event.OrganizationID)}); err != nil {
			return err
		}
		ids = append(ids, event.ID)
	}
	// 整批先持久化再ACK。提交未知不能确认；下次重放由收件人/事件/类型唯一键
	// 与完整目标比较收敛，仓储不会覆盖用户已经设置的read_at。
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.source.AckEvents(ctx, ids)
}
````

UTF-8/LF全文SHA-256：`3228e09b2e1a95744abcaed299761576d561aad917f0fb316071249a22e3bbed`。[参考实现](../../../../services/document/internal/logic/document/account_notification_import.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
