package account

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"collaboration/platform"
)

var ErrNotificationEventBinding = errors.New("NOTIFICATION_EVENT_BINDING_INVALID")

// 接管排队通知仍须证明任务来自成功的真实转移；坏绑定不能被后台队列豁免掩盖。
func NotificationEventsReady(ctx context.Context, db *sql.DB) error {
	var broken int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task d
		LEFT JOIN notification n ON d.business_key=CAST(n.id AS CHAR)
		LEFT JOIN organization_transition t ON t.operation_id=d.operation_id
		LEFT JOIN operation_receipt r ON r.operation_id=d.operation_id
		WHERE d.kind='ACCOUNT_NOTICE_DELIVERY' AND d.state NOT IN ('SUCCEEDED','REJECTED')
		AND NOT COALESCE(d.state='QUEUED' AND n.kind='ORG_OWNER_TRANSFER'
		AND t.kind='TRANSFER_OWNER' AND t.state='SUCCEEDED'
		AND r.kind='ORG_OWNER_TRANSFER' AND r.state='SUCCEEDED' AND r.actor_id=t.actor_id
		AND n.recipient_id=t.target_user_id
		AND JSON_UNQUOTE(JSON_EXTRACT(n.target_json,'$.organization_id'))=CAST(t.organization_id AS CHAR)
		AND n.event_id=CONCAT('account-owner:',LOWER(SHA2(t.operation_id,256)))
		AND d.payload=JSON_OBJECT('id',n.id,'recipient_id',n.recipient_id,'organization_id',t.organization_id,'event_id',n.event_id,'kind',n.kind),FALSE)`).Scan(&broken)
	if err != nil {
		return err
	}
	if broken != 0 {
		return ErrNotificationEventBinding
	}
	return nil
}

// 成功回执、真实转移记录和通知使用调用方的同一事务，避免终态已可见但事件丢失。
func (r *Closure) CreateOwnerTransferNotice(ctx context.Context, tx *sql.Tx, operation, organization, actor string) error {
	var recipient uint64
	err := tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t
		JOIN operation_receipt r ON r.operation_id=t.operation_id
		WHERE t.operation_id=? AND t.organization_id=? AND t.actor_id=?
		AND t.kind='TRANSFER_OWNER' AND t.state='SUCCEEDED'
		AND r.kind='ORG_OWNER_TRANSFER' AND r.state='SUCCEEDED' FOR UPDATE`, operation, organization, actor).Scan(&recipient)
	if err != nil {
		return err
	}
	// 所有操作号统一散列，避免短原号恰好等于另一长原号摘要时撞到同一事件编号。
	digest := sha256.Sum256([]byte(operation))
	eventID := "account-owner:" + hex.EncodeToString(digest[:])
	target, err := json.Marshal(map[string]string{"organization_id": organization})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json)
		VALUES(?,?,'ORG_OWNER_TRANSFER',?) ON DUPLICATE KEY UPDATE id=id`, recipient, eventID, string(target)); err != nil {
		return err
	}
	var id uint64
	var original []byte
	if err = tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind='ORG_OWNER_TRANSFER' FOR UPDATE`, recipient, eventID).Scan(&id, &original); err != nil {
		return err
	}
	var saved struct {
		OrganizationID string `json:"organization_id"`
	}
	if json.Unmarshal(original, &saved) != nil || saved.OrganizationID != organization {
		return ErrNotificationEventBinding
	}
	orgID, err := strconv.ParseUint(organization, 10, 64)
	if err != nil || orgID == 0 || recipient == 0 {
		return ErrNotificationEventBinding
	}
	event := platform.AccountNotificationEvent{ID: id, RecipientID: recipient, OrganizationID: orgID, EventID: eventID, Kind: "ORG_OWNER_TRANSFER"}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_NOTICE_DELIVERY',?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, strconv.FormatUint(id, 10), operation, string(payload))
	return err
}

// 每次查询未确认任务，不采用ID高水位，避免并发事务按不同顺序提交时漏投。
func PendingNotificationEvents(ctx context.Context, db *sql.DB) ([]platform.AccountNotificationEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload
		FROM durable_task d JOIN notification n ON d.business_key=CAST(n.id AS CHAR)
		WHERE d.kind='ACCOUNT_NOTICE_DELIVERY' AND d.state='QUEUED' ORDER BY d.id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []platform.AccountNotificationEvent{}
	for rows.Next() {
		var event platform.AccountNotificationEvent
		var target, payload []byte
		if err = rows.Scan(&event.ID, &event.RecipientID, &event.EventID, &event.Kind, &target, &payload); err != nil {
			return nil, err
		}
		if err = verifyNotificationEvent(&event, target, payload); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func verifyNotificationEvent(event *platform.AccountNotificationEvent, target, payload []byte) error {
	var saved struct {
		OrganizationID string `json:"organization_id"`
	}
	var marker platform.AccountNotificationEvent
	if json.Unmarshal(target, &saved) != nil || json.Unmarshal(payload, &marker) != nil {
		return ErrNotificationEventBinding
	}
	org, err := strconv.ParseUint(saved.OrganizationID, 10, 64)
	if err != nil || org == 0 {
		return ErrNotificationEventBinding
	}
	event.OrganizationID = org
	if event.ID == 0 || event.RecipientID == 0 || event.EventID == "" || len(event.EventID) > 96 || event.Kind != "ORG_OWNER_TRANSFER" || *event != marker {
		return ErrNotificationEventBinding
	}
	return nil
}

func AcknowledgeNotificationEvent(ctx context.Context, tx *sql.Tx, id uint64) error {
	var event platform.AccountNotificationEvent
	var target, payload []byte
	var taskID uint64
	var state string
	// 重新锁定原始事件及对应任务，不接受调用者提供的收件人或事件内容替代绑定。
	err := tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state
		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS CHAR) AND d.kind='ACCOUNT_NOTICE_DELIVERY'
		WHERE n.id=? FOR UPDATE`, id).Scan(&event.ID, &event.RecipientID, &event.EventID, &event.Kind, &target, &payload, &taskID, &state)
	if err != nil {
		return err
	}
	if err = verifyNotificationEvent(&event, target, payload); err != nil {
		return err
	}
	if state != "QUEUED" && state != "SUCCEEDED" {
		return ErrNotificationEventBinding
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=?`, taskID)
	return err
}
