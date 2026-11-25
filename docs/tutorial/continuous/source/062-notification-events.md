# services/account/internal/logic/account/notification_events.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/062-notification-events.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`NotificationEvents`、`Ready`、`NewNotificationEvents`、`Events`、`Acknowledge`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package account

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
)

var ErrNotificationEventsInput = errors.New("NOTIFICATION_EVENTS_INVALID")

type NotificationEvents struct{ db *sql.DB }

func (s *NotificationEvents) Ready(ctx context.Context) error {
	return repository.NotificationEventsReady(ctx, s.db)
}

func NewNotificationEvents(db *sql.DB) *NotificationEvents { return &NotificationEvents{db: db} }

func (s *NotificationEvents) Events(ctx context.Context) (platform.AccountNotificationEventsResponse, error) {
	items, err := repository.PendingNotificationEvents(ctx, s.db)
	return platform.AccountNotificationEventsResponse{Events: items}, err
}

func (s *NotificationEvents) Acknowledge(ctx context.Context, input platform.AccountNotificationAckRequest) error {
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		return ErrNotificationEventsInput
	}
	ids := append([]uint64(nil), input.IDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] == id) {
			return ErrNotificationEventsInput
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 按稳定ID顺序加锁；任何不存在或不匹配的确认都回滚整批，不丢弃其他事件。
	for _, id := range ids {
		if err = repository.AcknowledgeNotificationEvent(ctx, tx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, repository.ErrNotificationEventBinding) {
				return ErrNotificationEventsInput
			}
			return err
		}
	}
	return tx.Commit()
}
````

UTF-8/LF全文SHA-256：`399ba4b345653c26630e17cac2e107660ac226290656258456bf1fd904cf51ea`。[参考实现](../../../../services/account/internal/logic/account/notification_events.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
