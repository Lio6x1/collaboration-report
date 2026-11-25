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
