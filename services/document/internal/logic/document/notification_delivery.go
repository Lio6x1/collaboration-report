package document

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	notices "collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync/atomic"
	"time"
)

type NotificationRecipientResolver interface {
	Resolve(context.Context, uint64, []uint64) (platform.NotificationRecipientsResponse, error)
	Administrators(context.Context, uint64, uint64) (platform.NotificationAdministratorsResponse, error)
}

type NotificationDelivery struct {
	db         *sql.DB
	comments   *Comments
	recipients NotificationRecipientResolver
	turn       atomic.Uint64
}

var ErrNotificationIdle = errors.New("NOTIFICATION_QUEUE_IDLE")

func (s *NotificationDelivery) Ready(ctx context.Context) error {
	invalid, err := notices.InvalidCommentTask(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	invalid, err = notices.InvalidReviewTask(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
	}
	invalid, err = notices.InvalidDepartureTasks(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	}
	return nil
}

func NewNotificationDelivery(db *sql.DB, comments *Comments, recipients NotificationRecipientResolver) *NotificationDelivery {
	return &NotificationDelivery{db: db, comments: comments, recipients: recipients}
}

func decodeCommentNotification(raw []byte) (domain.CommentNotification, error) {
	var value domain.CommentNotification
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || value.Document == 0 || value.Thread == 0 || value.Comment == 0 || value.Version == 0 || value.Actor == 0 || len(value.Recipients) > 100 {
		return value, errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	if _, err := domain.CanonicalID(value.ContextID); err != nil {
		return value, err
	}
	return value, nil
}

func commentNotificationRecipients(value domain.CommentNotification) ([]uint64, map[uint64]bool, error) {
	all := map[uint64]bool{}
	mentions := map[uint64]bool{}
	for _, id := range value.Recipients {
		if id == 0 {
			return nil, nil, errors.New("COMMENT_NOTIFICATION_RECIPIENT_INVALID")
		}
		mentions[id] = true
		all[id] = true
	}
	for _, id := range value.ReplyRecipients {
		if id == 0 {
			return nil, nil, errors.New("COMMENT_NOTIFICATION_RECIPIENT_INVALID")
		}
		all[id] = true
	}
	ids := []uint64{}
	for id := range all {
		if id != value.Actor && id > value.DeliveryAfter {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, mentions, nil
}

// 外部身份查询发生在SQL锁外；短期结果最终与Doc权威权限、来源关闭高水位同事务核验。
func (s *NotificationDelivery) Recover(ctx context.Context) (failure error) {
	// 实际旧队列积压数千条时，新事件仍应及时可见；每8次保留1次给最新事件，
	// 其余继续最老任务。此选择不更改持久进度，重启不会影响去重或任务恢复。
	recent := s.turn.Add(1)%8 == 1
	task, err := notices.NextTask(ctx, s.db, []string{"COMMENT_NOTIFICATION", "REVIEW_NOTIFICATION", "REVIEW_ACTION_NOTIFICATION", "MERGE_NOTIFICATION", "RESPONSIBLE_DEPARTURE_NOTIFICATION"}, recent)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotificationIdle
	}
	if err != nil {
		return err
	}
	defer func() {
		if failure != nil {
			retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			// 原失败仍返回给worker，重试登记失败也不能把本次当成成功。
			_ = notices.RetryTask(retry, s.db, task)
		}
	}()
	if task.Kind == "RESPONSIBLE_DEPARTURE_NOTIFICATION" {
		return s.deliverDeparture(ctx, task)
	}
	if task.Kind != "COMMENT_NOTIFICATION" {
		return s.deliverReview(ctx, task)
	}
	input, err := decodeCommentNotification(task.Payload)
	if err != nil {
		return err
	}
	ids, mentions, err := commentNotificationRecipients(input)
	if err != nil {
		return err
	}
	more := len(ids) > 100
	if more {
		ids = ids[:100]
	}
	resolved, err := s.deliveryAccess(ctx, input.Document, ids)
	if err != nil {
		return err
	}
	doc, space, proof, sources, kbRevision := resolved.doc, resolved.space, resolved.proof, resolved.sources, resolved.kbRevision
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return err
	}
	current, err := documents.DocumentRow(ctx, tx, input.Document, true)
	if err != nil {
		return err
	}
	if current.AclRevision != doc.AclRevision || current.CurrentStructureId != doc.CurrentStructureId || current.KnowledgeBaseId == nil != (doc.KnowledgeBaseId == nil) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
	}
	t, err := comments.ThreadRow(ctx, tx, input.Thread, true)
	if err != nil {
		return err
	}
	entry, err := comments.CommentRow(ctx, tx, input.Comment, true)
	if err != nil {
		return err
	}
	if t.Document != input.Document || t.Kind != input.ContextKind || t.Context != input.ContextID || t.Version != input.Version || entry.Thread != t.ID || entry.Author != input.Actor {
		return errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.Generation != task.Generation || locked.State != task.State || !bytes.Equal(locked.Payload, task.Payload) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_TASK_CHANGED")
	}
	if err = notices.ValidateCommentReceipt(ctx, tx, task, input); err != nil {
		return err
	}
	if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if current.Lifecycle == "ACTIVE" && t.Status != "DELETED" && !entry.Deleted {
		for _, recipient := range proof.Recipients {
			if !recipient.Eligible {
				continue
			}
			facts, err := notices.Facts(ctx, tx, current, space, recipient.UserID, recipient.OrgOwner || recipient.Role == "ADMIN")
			if err != nil {
				return err
			}
			access := DocumentAccess{Document: current, Permissions: domain.ResolvePermissions(facts), KBRevision: kbRevision}
			if !access.Permissions.Read {
				continue
			}
			// 这里只复用纯业务上下文核验，没有伪造接收者会话或对外输出准入。
			if err = s.comments.validateContext(ctx, tx, Actor{ID: recipient.UserID}, access, t, false); err != nil {
				if notificationHidden(err) {
					continue
				}
				return err
			}
			kind := "COMMENT_REPLY"
			if mentions[recipient.UserID] {
				kind = "COMMENT_MENTION"
			}
			if err = notices.Insert(ctx, tx, recipient.UserID, task.Operation, kind, notices.Target{Document: doc.Id, Thread: domain.TextID(t.ID)}); err != nil {
				return err
			}
		}
	}
	if more {
		input.DeliveryAfter = ids[len(ids)-1]
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
			return err
		}
	} else {
		finished, err := notices.FinishTask(ctx, tx, task)
		if err != nil {
			return err
		}
		if !finished {
			return errors.New("NOTIFICATION_TASK_CHANGED")
		}
	}
	if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	// 插入与进度同提交；结果未知保留原任务，唯一键及绑定复核阻止重复提醒。
	return tx.Commit()
}
