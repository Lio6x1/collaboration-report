package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// QueueDepartureNotifications 必须在已验证离组门禁的原事务内、清空负责人之前调用。
// 先按知识库顺序持锁，再锁文档，使真实管理员快照与负责人清空原子提交。
func QueueDepartureNotifications(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	if binding.Org == 0 || binding.User == 0 || binding.Epoch == 0 || !domain.ValidOperationID(binding.ID) {
		return errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT k.id FROM knowledge_base k JOIN space s ON s.id=k.space_id
		WHERE s.kind='ORGANIZATION' AND s.organization_id=? ORDER BY k.id FOR UPDATE`, binding.Org)
	if err != nil {
		return err
	}
	knowledgeBases := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		knowledgeBases = append(knowledgeBases, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, kb := range knowledgeBases {
		if err = queueKnowledgeBaseDepartures(ctx, tx, binding, kb); err != nil {
			return err
		}
	}
	return nil
}

func queueKnowledgeBaseDepartures(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding, kb uint64) error {
	rows, err := tx.QueryContext(ctx, `SELECT d.id,d.control_revision FROM document d JOIN space s ON s.id=d.space_id
		JOIN knowledge_base k ON k.id=d.knowledge_base_id AND k.space_id=d.space_id
		WHERE s.kind='ORGANIZATION' AND s.organization_id=? AND d.knowledge_base_id=? AND d.responsible_user_id=?
		AND d.lifecycle IN ('ACTIVE','TRASHED') ORDER BY d.id FOR UPDATE`, binding.Org, kb, binding.User)
	if err != nil {
		return err
	}
	type candidate struct{ id, revision uint64 }
	documents := []candidate{}
	for rows.Next() {
		var doc candidate
		if err = rows.Scan(&doc.id, &doc.revision); err != nil {
			rows.Close()
			return err
		}
		documents = append(documents, doc)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(documents) == 0 {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=?
		AND state='ACTIVE' AND role='ADMIN' AND user_id<>? ORDER BY user_id FOR UPDATE`, kb, binding.User)
	if err != nil {
		return err
	}
	recipients := []uint64{}
	for rows.Next() {
		var recipient uint64
		if err = rows.Scan(&recipient); err != nil {
			rows.Close()
			return err
		}
		if recipient == 0 {
			rows.Close()
			return errors.New("DEPARTURE_NOTIFICATION_RECIPIENT_INVALID")
		}
		if len(recipients) == 0 || recipients[len(recipients)-1] != recipient {
			recipients = append(recipients, recipient)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, doc := range documents {
		event := domain.DepartureNotification{Document: doc.id, KnowledgeBase: kb, Organization: binding.Org,
			Departed: binding.User, Epoch: binding.Epoch, Operation: binding.ID,
			Event: domain.DepartureNotificationEvent(binding.ID, doc.id), Recipients: recipients}
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		// 冲突只占位，不覆盖旧回执；逐字段校验防止同一操作编号绑定到另一次离组。
		// 这是关闭协议执行的系统动作；被移除成员不一定是发起者，不能冒充操作者。
		// 受影响成员已固定在plan.departed_user_id中，实际发起者沿原Account操作查询。
		_, err = tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
			VALUES(?,NULL,'OWNER_CHANGE','SUCCEEDED',?,?) ON DUPLICATE KEY UPDATE operation_id=operation_id`, event.Event, doc.revision, string(raw))
		if err != nil {
			return err
		}
		var actor, revision uint64
		var kind, state string
		var plan []byte
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, event.Event).Scan(&actor, &kind, &state, &revision, &plan); err != nil {
			return err
		}
		fixed, decodeErr := domain.DecodeDepartureNotification(plan)
		if decodeErr != nil || actor != 0 || kind != "OWNER_CHANGE" || state != "SUCCEEDED" || revision != doc.revision || fixed.DeliveryAfter != 0 || !domain.SameDepartureNotification(fixed, event) {
			return errors.New("DEPARTURE_NOTIFICATION_RECEIPT_CONFLICT")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
			VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, domain.DepartureNotificationKind, event.Event, event.Event, string(raw))
		if err != nil {
			return err
		}
		var operation string
		var payload []byte
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(operation_id,''),payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE`, domain.DepartureNotificationKind, event.Event).Scan(&operation, &payload); err != nil {
			return err
		}
		existing, decodeErr := domain.DecodeDepartureNotification(payload)
		if decodeErr != nil || operation != event.Event || !domain.SameDepartureNotification(existing, event) {
			return errors.New("DEPARTURE_NOTIFICATION_TASK_CONFLICT")
		}
	}
	return nil
}
