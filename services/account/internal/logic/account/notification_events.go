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
