# services/document/internal/logic/document/notifications_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/213-notifications-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`notificationDeliveryAccess`、`deliveryAccess`、`deliverDeparture`、`finishClosedDeparture`、`notificationTaskUnchanged`、`orderedRecipients`、`prepareReviewRecipients`、`decodeReviewRecipients`、`commitReviewProgress`、`deliverReview`、`LookupReviewTarget`、`ReviewRoundTarget`、`Notifications`、`NewNotifications`、`notificationHidden`、`notificationValue`、`validateTarget`、`List`、`MarkRead`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	notices "collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

// notification_delivery_access.go：保留原实现，按业务合并文件。

type notificationDeliveryAccess struct {
	doc        dto.Document
	space      dto.Space
	proof      platform.NotificationRecipientsResponse
	sources    []*pb.SourceEpoch
	kbRevision uint64
}

// 跨服务查询统一放在SQL锁外；每种通知只负责自己的业务绑定和收件范围。
// 返回的来源必须在最终业务事务持锁，短期身份结果不能直接当成投递许可。
func (s *NotificationDelivery) deliveryAccess(ctx context.Context, document uint64, ids []uint64) (notificationDeliveryAccess, error) {
	var value notificationDeliveryAccess
	var err error
	value.doc, err = documents.DocumentRow(ctx, s.db, document, false)
	if err != nil {
		return value, err
	}
	spaceID, err := domain.CanonicalID(value.doc.SpaceId)
	if err != nil {
		return value, err
	}
	value.space, err = documents.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return value, err
	}
	var organization uint64
	if value.space.OrganizationId != nil {
		organization, err = domain.CanonicalID(*value.space.OrganizationId)
		if err != nil {
			return value, err
		}
	}
	value.proof = platform.NotificationRecipientsResponse{ExpiresUnixMS: time.Now().Add(5 * time.Second).UnixMilli()}
	if len(ids) > 0 {
		value.proof, err = s.recipients.Resolve(ctx, organization, ids)
		if err != nil {
			return value, err
		}
	}
	acl, err := domain.CanonicalID(value.doc.AclRevision)
	if err != nil {
		return value, err
	}
	value.sources = []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + value.doc.Id, Epoch: acl}}
	if value.doc.KnowledgeBaseId != nil {
		kb, _ := domain.CanonicalID(*value.doc.KnowledgeBaseId)
		var state string
		state, value.kbRevision, err = s.comments.access.repository.KnowledgeBase(ctx, kb, spaceID)
		if err != nil {
			return value, err
		}
		if state != "ACTIVE" {
			return value, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
		value.sources = append(value.sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *value.doc.KnowledgeBaseId, Epoch: value.kbRevision})
	}
	for _, recipient := range value.proof.Recipients {
		if recipient.Eligible {
			for _, source := range recipient.Sources {
				value.sources = append(value.sources, &pb.SourceEpoch{ScopeKey: source.ScopeKey, Epoch: source.Epoch})
			}
		}
	}
	return value, nil
}

// notification_departure_delivery.go：保留原实现，按业务合并文件。

// 收件集合来自负责人清空时的真实知识库管理员快照，投递时再核验当前身份和KB角色。
// 不把组织ADMIN、负责人或一般COMMENT成员扩充为知识库管理员。
func (s *NotificationDelivery) deliverDeparture(ctx context.Context, task notices.Task) error {
	event, err := notices.DepartureBinding(ctx, s.db, task)
	if err != nil {
		return err
	}
	start := sort.Search(len(event.Recipients), func(i int) bool { return event.Recipients[i] > event.DeliveryAfter })
	end := start + 100
	if end > len(event.Recipients) {
		end = len(event.Recipients)
	}
	ids := event.Recipients[start:end]
	resolved, err := s.deliveryAccess(ctx, event.Document, ids)
	if err != nil {
		// 关闭中的库仍等待排空；只有持锁确认CLOSED，才可把失去目标的提醒结束。
		// 资格查询超时、缺行或其他未知错误不能被当成“无需投递”。
		finished, finishErr := s.finishClosedDeparture(ctx, task, event)
		if finishErr != nil {
			return finishErr
		}
		if finished {
			return nil
		}
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, resolved.sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, DocumentAccess{Document: resolved.doc, KBRevision: resolved.kbRevision})
	if err != nil {
		return err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return err
	}
	if event, err = notices.DepartureBinding(ctx, tx, locked); err != nil {
		return err
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	eligibleContext := current.Lifecycle == "ACTIVE" && current.KnowledgeBaseId != nil && *current.KnowledgeBaseId == domain.TextID(event.KnowledgeBase) && resolved.space.OrganizationId != nil && *resolved.space.OrganizationId == domain.TextID(event.Organization)
	if eligibleContext {
		for _, recipient := range resolved.proof.Recipients {
			if !recipient.Eligible || recipient.UserID == event.Departed {
				continue
			}
			role, err := documents.KnowledgeBaseRole(ctx, tx, event.KnowledgeBase, recipient.UserID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if role != "ADMIN" {
				continue
			}
			target := notices.Target{Document: current.Id, KnowledgeBase: domain.TextID(event.KnowledgeBase)}
			if err = notices.Insert(ctx, tx, recipient.UserID, event.Event, "RESPONSIBLE_DEPARTURE", target); err != nil {
				return err
			}
		}
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	// 通知与批次位置在同一事务提交；未知提交仅重新读取原任务，不改收件集合。
	if end < len(event.Recipients) {
		event.DeliveryAfter = ids[len(ids)-1]
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
			return err
		}
	} else {
		done, err := notices.FinishTask(ctx, tx, task)
		if err != nil {
			return err
		}
		if !done {
			return errors.New("NOTIFICATION_TASK_CHANGED")
		}
	}
	return tx.Commit()
}

// 已关闭知识库没有可进入的管理目标。验证原回执后终结任务，不制造通知，
// 不删除回执或资源；原知识库锁阻止生命周期事实在判断和提交之间改变。
func (s *NotificationDelivery) finishClosedDeparture(ctx context.Context, task notices.Task, event domain.DepartureNotification) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	state, _, err := documents.LockedKnowledgeBase(ctx, tx, domain.TextID(event.KnowledgeBase))
	if err != nil || state != "CLOSED" {
		return false, err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return false, err
	}
	if locked.State == "SUCCEEDED" {
		return true, nil
	}
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return false, err
	}
	if _, err = notices.DepartureBinding(ctx, tx, locked); err != nil {
		return false, err
	}
	done, err := notices.FinishTask(ctx, tx, task)
	if err != nil {
		return false, err
	}
	if !done {
		return false, errors.New("NOTIFICATION_TASK_CHANGED")
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// notification_review_delivery.go：保留原实现，按业务合并文件。

func notificationTaskUnchanged(current, original notices.Task) error {
	if current.Kind != original.Kind || current.Operation != original.Operation || current.Generation != original.Generation || current.State != original.State || !bytes.Equal(current.Payload, original.Payload) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_TASK_CHANGED")
	}
	return nil
}

func orderedRecipients(groups ...[]uint64) []uint64 {
	seen := map[uint64]bool{}
	for _, group := range groups {
		for _, id := range group {
			if id != 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// 首次恢复时冻结完整候选集，原业务JSON字段仍保留，既有恢复绑定检查继续生效。
// Account目录只供候选枚举，后续每批均取得新身份事实并检查当前Doc资格。
func (s *NotificationDelivery) prepareReviewRecipients(ctx context.Context, task notices.Task, event notices.ReviewEvent) error {
	doc, err := documents.DocumentRow(ctx, s.db, event.Document, false)
	if err != nil {
		return err
	}
	spaceID, err := domain.CanonicalID(doc.SpaceId)
	if err != nil {
		return err
	}
	space, err := documents.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return err
	}
	admins := []uint64{}
	if space.OrganizationId != nil {
		organization, err := domain.CanonicalID(*space.OrganizationId)
		if err != nil {
			return err
		}
		var after uint64
		for {
			page, err := s.recipients.Administrators(ctx, organization, after)
			if err != nil {
				return err
			}
			admins = append(admins, page.UserIDs...)
			if page.NextAfter == 0 {
				break
			}
			if page.NextAfter <= after {
				return errors.New("NOTIFICATION_DIRECTORY_CURSOR_INVALID")
			}
			after = page.NextAfter
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = documents.DocumentRow(ctx, tx, event.Document, true); err != nil {
		return err
	}
	current, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if current.State == "SUCCEEDED" {
		return nil
	}
	if err = notificationTaskUnchanged(current, task); err != nil {
		return err
	}
	if event, err = notices.ReviewBinding(ctx, tx, task); err != nil {
		return err
	}
	ids, participants, err := notices.ReviewCandidates(ctx, tx, event)
	if err != nil {
		return err
	}
	all := orderedRecipients(ids, admins, participants)
	filtered := all[:0]
	for _, id := range all {
		if id != event.Actor {
			filtered = append(filtered, id)
		}
	}
	raw, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	event.Payload["delivery_recipients"] = string(raw)
	raw, err = json.Marshal(participants)
	if err != nil {
		return err
	}
	event.Payload["delivery_participants"] = string(raw)
	event.Payload["delivery_after"] = "0"
	return commitReviewProgress(ctx, tx, task, event.Payload)
}

func decodeReviewRecipients(event notices.ReviewEvent) ([]uint64, map[uint64]bool, uint64, error) {
	var ids, participants []uint64
	if json.Unmarshal([]byte(event.Payload["delivery_recipients"]), &ids) != nil || ids == nil || json.Unmarshal([]byte(event.Payload["delivery_participants"]), &participants) != nil || participants == nil {
		return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID")
	}
	var after uint64
	if text := event.Payload["delivery_after"]; text != "0" {
		var err error
		after, err = domain.CanonicalID(text)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	seen := map[uint64]bool{}
	var previous uint64
	for _, id := range ids {
		if id <= previous || id == event.Actor {
			return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_SNAPSHOT_INVALID")
		}
		seen[id] = true
		previous = id
	}
	if after != 0 && !seen[after] {
		return nil, nil, 0, errors.New("NOTIFICATION_RECIPIENT_CURSOR_INVALID")
	}
	participantsSet := map[uint64]bool{}
	for _, id := range participants {
		if !seen[id] || participantsSet[id] {
			return nil, nil, 0, errors.New("NOTIFICATION_PARTICIPANT_SNAPSHOT_INVALID")
		}
		participantsSet[id] = true
	}
	return ids, participantsSet, after, nil
}

func commitReviewProgress(ctx context.Context, tx *sql.Tx, task notices.Task, payload map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *NotificationDelivery) deliverReview(ctx context.Context, task notices.Task) error {
	event, err := notices.ReviewBinding(ctx, s.db, task)
	if err != nil {
		return err
	}
	if _, ready := event.Payload["delivery_recipients"]; !ready {
		return s.prepareReviewRecipients(ctx, task, event)
	}
	all, participants, after, err := decodeReviewRecipients(event)
	if err != nil {
		return err
	}
	start := sort.Search(len(all), func(i int) bool { return all[i] > after })
	end := start + 100
	if end > len(all) {
		end = len(all)
	}
	ids := all[start:end]
	resolved, err := s.deliveryAccess(ctx, event.Document, ids)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, resolved.sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, DocumentAccess{Document: resolved.doc, KBRevision: resolved.kbRevision})
	if err != nil {
		return err
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if err = notificationTaskUnchanged(locked, task); err != nil {
		return err
	}
	if event, err = notices.ReviewBinding(ctx, tx, task); err != nil {
		return err
	}
	pointer, err := documents.ReviewPointerRow(ctx, tx, event.Request)
	if err != nil {
		return err
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if current.Lifecycle == "ACTIVE" {
		for _, recipient := range resolved.proof.Recipients {
			if !recipient.Eligible {
				continue
			}
			facts, err := notices.Facts(ctx, tx, current, resolved.space, recipient.UserID, recipient.OrgOwner || recipient.Role == "ADMIN")
			if err != nil {
				return err
			}
			access := DocumentAccess{Document: current, Permissions: domain.ResolvePermissions(facts), KBRevision: resolved.kbRevision}
			// 提交只提醒当前审核/管理者；动作及合并额外提醒原参与者，但参与不授予读取权。
			if !access.Permissions.Read || (!access.Permissions.Review && !access.Permissions.Manage && !participants[recipient.UserID]) {
				continue
			}
			if err = s.comments.reviews.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
				if notificationHidden(err) {
					continue
				}
				return err
			}
			target := notices.Target{Document: current.Id, Request: domain.TextID(event.Request), Round: domain.TextID(event.Round)}
			if err = notices.Insert(ctx, tx, recipient.UserID, task.Operation, event.Kind, target); err != nil {
				return err
			}
		}
	}
	if resolved.proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if end < len(all) {
		event.Payload["delivery_after"] = domain.TextID(ids[len(ids)-1])
		return commitReviewProgress(ctx, tx, task, event.Payload)
	}
	done, err := notices.FinishTask(ctx, tx, task)
	if err != nil {
		return err
	}
	if !done {
		return errors.New("NOTIFICATION_TASK_CHANGED")
	}
	return tx.Commit()
}

// notification_review_target.go：保留原实现，按业务合并文件。

// 先按真实收件人读取原目标，供handler绑定文档身份准入；公开target_id不含轮次，不能拿当前轮次补齐。
func (s *Notifications) LookupReviewTarget(ctx context.Context, id, recipient uint64) (notices.Entry, error) {
	n, err := notices.Row(ctx, s.db, id, recipient, false)
	if errors.Is(err, sql.ErrNoRows) {
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
	}
	if err != nil {
		return n, err
	}
	switch n.Kind {
	case "REVIEW_SUBMITTED", "REVIEW_RESUBMITTED", "REVIEW_ACTION", "REVIEW_MERGED":
	default:
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
	}
	if n.Target.Thread != "" {
		return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
	}
	for _, value := range []string{n.Target.Document, n.Target.Request, n.Target.Round} {
		if _, err := domain.CanonicalID(value); err != nil {
			return n, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
		}
	}
	return n, nil
}

func (s *Notifications) ReviewRoundTarget(ctx context.Context, actor Actor, n notices.Entry) ([]*pb.SourceEpoch, ReadBody, error) {
	if n.Recipient != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
	}
	document, err := domain.CanonicalID(n.Target.Document)
	if err != nil {
		return nil, nil, err
	}
	request, err := domain.CanonicalID(n.Target.Request)
	if err != nil {
		return nil, nil, err
	}
	round, err := domain.CanonicalID(n.Target.Round)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.access.Authorize(ctx, actor, document, "NOTIFICATION_REVIEW_ROUND")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 最终输出事务先锁定并复核文档/审核资格，再重读原通知；通知本身不替代当前读取资格。
		if err := s.validateTarget(ctx, tx, actor, access, n); err != nil {
			return nil, err
		}
		current, err := notices.Row(ctx, tx, n.ID, actor.ID, false)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
		}
		if err != nil {
			return nil, err
		}
		if current.Target != n.Target || current.Kind != n.Kind || current.Event != n.Event {
			return nil, domain.Violation(domain.StateConflict, "NOTIFICATION_TARGET_CHANGED")
		}
		fixed, err := documents.ReviewRoundRow(ctx, tx, request, round, n.Target.Document)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_REVIEW_TARGET_NOT_FOUND")
		}
		return fixed, err
	}
	return access.Sources, run, nil
}

// notifications.go：保留原实现，按业务合并文件。

type Notifications struct {
	db         *sql.DB
	access     *Access
	comments   *Comments
	pagination *Pagination
	recipients NotificationRecipientResolver
}

func NewNotifications(db *sql.DB, access *Access, comments *Comments, pagination *Pagination, recipients NotificationRecipientResolver) *Notifications {
	return &Notifications{db: db, access: access, comments: comments, pagination: pagination, recipients: recipients}
}

func notificationHidden(err error) bool {
	var rule domain.RuleViolation
	return errors.Is(err, sql.ErrNoRows) || (errors.As(err, &rule) && (rule.Kind == domain.PermissionDenied || rule.Kind == domain.EntityNotFound))
}

// 通知显示通用事件名；文档标题和正文仅在跳转目标的授权接口中读取。
func notificationValue(n notices.Entry) (dto.Notification, error) {
	value := dto.Notification{Id: domain.TextID(n.ID), EventId: n.Event, Kind: n.Kind}
	target := n.Target.Document
	switch n.Kind {
	case "ORG_OWNER_TRANSFER":
		value.Title, target = "组织所有者已转交给你", n.Target.Organization
		// 内部来源命名空间使用冒号，公开event_id沿冻结OperationID字符集呈现。
		// 只转换传输表示，原始事件及唯一键不变，旧已读记录仍对应同一通知。
		if !strings.HasPrefix(n.Event, "account-owner:") {
			return value, errors.New("NOTIFICATION_EVENT_INVALID")
		}
		value.EventId = "account-owner-" + strings.TrimPrefix(n.Event, "account-owner:")
	case "RESPONSIBLE_TRANSFER":
		value.Title = "你已成为文档负责人"
	case "RESPONSIBLE_DEPARTURE":
		value.Title = "文档负责人已离开组织，请安排接任人"
	case "COMMENT_MENTION":
		value.Title, target = "有人在讨论中提到了你", n.Target.Thread
	case "COMMENT_REPLY":
		value.Title, target = "你参与的讨论有新回复", n.Target.Thread
	case "REVIEW_SUBMITTED":
		value.Title, target = "有新的审核提交", n.Target.Request
	case "REVIEW_RESUBMITTED":
		value.Title, target = "审核内容已重新提交", n.Target.Request
	case "REVIEW_ACTION":
		value.Title, target = "审核状态有更新", n.Target.Request
	case "REVIEW_MERGED":
		value.Title, target = "审核内容已合并", n.Target.Request
	default:
		return value, errors.New("NOTIFICATION_KIND_INVALID")
	}
	if _, err := domain.CanonicalID(target); err != nil {
		return value, err
	}
	value.TargetId = &target
	if n.ReadAt != nil {
		read := n.ReadAt.UTC().Format(time.RFC3339Nano)
		value.ReadAt = &read
	}
	return value, nil
}

func (s *Notifications) validateTarget(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, n notices.Entry) error {
	doc, err := ValidateAccess(ctx, tx, a)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.EntityNotFound, "NOTIFICATION_TARGET_NOT_AVAILABLE")
	}
	if n.Kind == "RESPONSIBLE_DEPARTURE" {
		if doc.KnowledgeBaseId == nil || *doc.KnowledgeBaseId != n.Target.KnowledgeBase {
			return domain.Violation(domain.EntityNotFound, "NOTIFICATION_TARGET_NOT_AVAILABLE")
		}
		kb, err := domain.CanonicalID(n.Target.KnowledgeBase)
		if err != nil {
			return err
		}
		role, err := documents.KnowledgeBaseRole(ctx, tx, kb, actor.ID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && role != "ADMIN") {
			return domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ADMIN_REQUIRED")
		}
		if err != nil {
			return err
		}
	}
	if n.Target.Thread != "" {
		id, err := domain.CanonicalID(n.Target.Thread)
		if err != nil {
			return err
		}
		t, err := comments.ThreadRow(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if domain.TextID(t.Document) != doc.Id {
			return errors.New("NOTIFICATION_TARGET_INVALID")
		}
		return s.comments.validateContext(ctx, tx, actor, a, t, false)
	}
	if n.Target.Request != "" {
		id, err := domain.CanonicalID(n.Target.Request)
		if err != nil {
			return err
		}
		pointer, err := documents.ReviewPointerRow(ctx, tx, id)
		if err != nil {
			return err
		}
		if domain.TextID(pointer.Document) != doc.Id {
			return errors.New("NOTIFICATION_TARGET_INVALID")
		}
		return s.comments.reviews.validateReviewReadAccess(ctx, tx, a, pointer)
	}
	return nil
}

func (s *Notifications) List(ctx context.Context, actor Actor, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "notifications:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef)
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	// 有界扫描允许隐藏项形成空页，但必须保留续页位置，不能泄露隐藏总数。
	candidates, err := notices.Candidates(ctx, s.db, actor.ID, after, limit+1)
	if err != nil {
		return nil, nil, err
	}
	accesses := map[uint64]DocumentAccess{}
	organizations := map[string]platform.NotificationRecipientsResponse{}
	hidden := map[uint64]bool{}
	sources, err := MergeSources(actor.Proof)
	if err != nil {
		return nil, nil, err
	}
	epochs := map[string]uint64{}
	for _, source := range sources {
		epochs[source.ScopeKey] = source.Epoch
	}
	for _, n := range candidates {
		if n.Target.Organization != "" {
			if _, exists := organizations[n.Target.Organization]; exists {
				continue
			}
			id, err := domain.CanonicalID(n.Target.Organization)
			if err != nil {
				return nil, nil, err
			}
			proof, err := s.recipients.Resolve(ctx, id, []uint64{actor.ID})
			if err != nil {
				return nil, nil, err
			}
			organizations[n.Target.Organization] = proof
			// 当前组织成员身份决定旧事件是否可见，通知本身不授予访问组织的权利。
			if proof.Recipients[0].Eligible {
				for _, source := range proof.Recipients[0].Sources {
					if old, exists := epochs[source.ScopeKey]; exists && old != source.Epoch {
						return nil, nil, domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
					}
					if _, exists := epochs[source.ScopeKey]; !exists {
						sources = append(sources, &pb.SourceEpoch{ScopeKey: source.ScopeKey, Epoch: source.Epoch})
						epochs[source.ScopeKey] = source.Epoch
					}
				}
			}
			continue
		}
		id, err := domain.CanonicalID(n.Target.Document)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := accesses[id]; exists || hidden[id] {
			continue
		}
		a, err := s.access.Authorize(ctx, actor, id, "NOTIFICATIONS")
		if notificationHidden(err) {
			hidden[id] = true
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		accesses[id] = a
		for _, source := range a.Sources {
			if old, exists := epochs[source.ScopeKey]; exists {
				if old != source.Epoch {
					return nil, nil, domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
				}
			} else {
				sources = append(sources, source)
				epochs[source.ScopeKey] = source.Epoch
			}
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 多文档结果按主键锁定，不能随通知事件的先后形成相反锁序。
		ids := make([]uint64, 0, len(accesses))
		for id := range accesses {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			if _, err := ValidateAccess(ctx, tx, accesses[id]); err != nil {
				return nil, err
			}
		}
		page := dto.NotificationPage{Items: []dto.Notification{}}
		scanned := after
		for index, n := range candidates {
			if index == limit {
				page.HasMore = true
				break
			}
			scanned = n.ID
			if n.Target.Organization != "" {
				proof := organizations[n.Target.Organization]
				if !proof.Recipients[0].Eligible {
					continue
				}
				// 这些来源已由统一Read事务加锁并登记输出租约；短期权威证明不可跨过期点复用。
				if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
					return nil, domain.Violation(domain.TemporarilyUnavailable, "NOTIFICATION_ACCESS_EXPIRED")
				}
			} else {
				id, _ := domain.CanonicalID(n.Target.Document)
				if hidden[id] {
					continue
				}
				if err := s.validateTarget(ctx, tx, actor, accesses[id], n); err != nil {
					if notificationHidden(err) {
						continue
					}
					return nil, err
				}
			}
			current, err := notices.Row(ctx, tx, n.ID, actor.ID, false)
			if err != nil {
				return nil, err
			}
			if current.Target != n.Target || current.Kind != n.Kind || current.Event != n.Event {
				return nil, errors.New("NOTIFICATION_TARGET_CHANGED")
			}
			value, err := notificationValue(current)
			if err != nil {
				return nil, err
			}
			page.Items = append(page.Items, value)
		}
		if page.HasMore {
			page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
		}
		return page, nil
	}
	return sources, run, nil
}

func (s *Notifications) MarkRead(actor Actor, id uint64, operation string) (Actor, MutationBody, MutationPresentation) {
	actor.Sources, _ = MergeSources(actor.Proof)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 冻结契约的已读能力只要求收件人本人；失去目标访问权仍可清理自己的提醒。
		if _, err := notices.Row(ctx, tx, id, actor.ID, true); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.Violation(domain.EntityNotFound, "NOTIFICATION_NOT_FOUND")
			}
			return nil, err
		}
		if err := notices.MarkRead(ctx, tx, id, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(operation, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, result map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := notices.Row(ctx, tx, id, actor.ID, false); err != nil {
			return nil, nil, err
		}
		return result, actor.Sources, nil
	}
	return actor, run, present
}
````

UTF-8/LF全文SHA-256：`e86ace660e3f1aa1822c280b3dd1e6b672233ef355403504904c2ebd797643f7`。[参考实现](../../../../services/document/internal/logic/document/notifications_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
