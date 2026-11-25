# services/document/internal/repository/notification/notification_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/186-notification-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ValidateCommentReceipt`、`AdvanceTask`、`InvalidCommentTask`、`validateDepartureReceipt`、`DepartureBinding`、`InvalidDepartureTasks`、`Target`、`Entry`、`Queryer`、`scanner`、`scan`、`validTarget`、`Row`、`Candidates`、`MarkRead`、`Facts`、`grants`、`units`、`CommentRecipients`、`ReviewEvent`、`readIDs`、`ReviewBinding`、`ReviewCandidates`、`InvalidReviewTask`、`Task`、`scanTask`、`NextTask`、`LockedTask`、`RetryTask`、`FinishTask`、`Insert`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package notification

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// comment_binding.go：保留原实现，按业务合并文件。

func ValidateCommentReceipt(ctx context.Context, tx *sql.Tx, task Task, input domain.CommentNotification) error {
	var actor uint64
	var kind, state string
	var raw []byte
	if err := tx.QueryRowContext(ctx, "SELECT actor_id,kind,state,result_json FROM operation_receipt WHERE operation_id=?", task.Operation).Scan(&actor, &kind, &state, &raw); err != nil {
		return err
	}
	var result struct {
		State  string `json:"state"`
		Result struct {
			ID string `json:"entity_id"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &result) != nil || state != "SUCCEEDED" || result.State != "SUCCEEDED" || actor != input.Actor {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	expected := input.Comment
	if kind == "COMMENT_THREAD_CREATE" {
		expected = input.Thread
	} else if kind != "COMMENT_REPLY" {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	if result.Result.ID != domain.TextID(expected) {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	return nil
}

func AdvanceTask(ctx context.Context, tx *sql.Tx, task Task, payload []byte) error {
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET payload=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6)
		WHERE id=? AND generation=? AND state=? AND kind=? AND generation<18446744073709551615`, string(payload), task.ID, task.Generation, task.State, task.Kind)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("NOTIFICATION_TASK_CHANGED")
	}
	return nil
}

// 已接管的排队任务可恢复，但绑定损坏不能通过就绪屏障。依据原回执和真实线程/评论，
// 不以“存在一个同名任务”或租约时间代替业务成功事实。
func InvalidCommentTask(ctx context.Context, db *sql.DB) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t
		WHERE t.kind='COMMENT_NOTIFICATION' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND
		(t.state NOT IN ('QUEUED','RETRY','UNKNOWN') OR NOT EXISTS(
		SELECT 1 FROM operation_receipt o JOIN comment c ON c.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.comment_id'))
		JOIN comment_thread h ON h.id=c.thread_id
		WHERE o.operation_id=t.operation_id AND o.state='SUCCEEDED' AND t.business_key=o.operation_id
		AND o.actor_id=c.author_id AND c.author_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor_id'))
		AND h.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.thread_id'))
		AND h.document_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document_id'))
		AND h.context_kind=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context_kind'))
		AND h.context_ref=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context_id'))
		AND h.content_version_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.content_version_id'))
		AND ((o.kind='COMMENT_THREAD_CREATE' AND JSON_UNQUOTE(JSON_EXTRACT(o.result_json,'$.result.entity_id'))=CAST(h.id AS CHAR))
		OR (o.kind='COMMENT_REPLY' AND JSON_UNQUOTE(JSON_EXTRACT(o.result_json,'$.result.entity_id'))=CAST(c.id AS CHAR))))))`).Scan(&invalid)
	return invalid, err
}

// departure_binding.go：保留原实现，按业务合并文件。

func validateDepartureReceipt(task Task, businessKey string, actor uint64, kind, state string, plan []byte) (domain.DepartureNotification, error) {
	event, err := domain.DecodeDepartureNotification(task.Payload)
	if err != nil {
		return event, err
	}
	fixed, err := domain.DecodeDepartureNotification(plan)
	if err != nil {
		return event, err
	}
	if task.Kind != domain.DepartureNotificationKind || task.Operation != event.Event || businessKey != event.Event || actor != 0 || kind != "OWNER_CHANGE" || state != "SUCCEEDED" || fixed.DeliveryAfter != 0 || !domain.SameDepartureNotification(event, fixed) {
		return event, errors.New("DEPARTURE_NOTIFICATION_RECEIPT_INVALID")
	}
	return event, nil
}

// DepartureBinding 只依赖原子提交的不可变离组回执，不依赖会再次打开的门禁，
// 也不要求负责人至今仍为空；当前文档范围和收件资格由投递事务单独复核。
func DepartureBinding(ctx context.Context, q Queryer, task Task) (domain.DepartureNotification, error) {
	var actor uint64
	var businessKey, kind, state string
	var plan []byte
	err := q.QueryRowContext(ctx, `SELECT t.business_key,COALESCE(o.actor_id,0),o.kind,o.state,o.fixed_plan
		FROM durable_task t JOIN lifecycle_operation o ON o.operation_id=t.operation_id
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.generation=?`, task.ID, task.Kind, task.Operation, task.Generation).
		Scan(&businessKey, &actor, &kind, &state, &plan)
	if err != nil {
		return domain.DepartureNotification{}, err
	}
	return validateDepartureReceipt(task, businessKey, actor, kind, state, plan)
}

// InvalidDepartureTasks 仅检查新通知类型，以有界批次联查不可变回执；
// 不逐条往返数据库，也不扫描历史上尚未接管的其他任务类型。
func InvalidDepartureTasks(ctx context.Context, db *sql.DB) (bool, error) {
	after := uint64(0)
	for {
		rows, err := db.QueryContext(ctx, `SELECT t.id,t.generation,t.kind,COALESCE(t.operation_id,''),t.state,t.payload,
			t.business_key,COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),o.fixed_plan
			FROM durable_task t LEFT JOIN lifecycle_operation o ON o.operation_id=t.operation_id
			WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>?
			ORDER BY t.id LIMIT 100`, domain.DepartureNotificationKind, after)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var task Task
			var actor uint64
			var businessKey, kind, state string
			var plan []byte
			if err = rows.Scan(&task.ID, &task.Generation, &task.Kind, &task.Operation, &task.State, &task.Payload, &businessKey, &actor, &kind, &state, &plan); err != nil {
				rows.Close()
				return false, err
			}
			if task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN" {
				rows.Close()
				return true, nil
			}
			if _, err = validateDepartureReceipt(task, businessKey, actor, kind, state, plan); err != nil {
				rows.Close()
				return true, nil
			}
			after = task.ID
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count < 100 {
			return false, nil
		}
	}
}

// notifications.go：保留原实现，按业务合并文件。

// Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。
type Target struct {
	Organization  string `json:"organization_id,omitempty"`
	KnowledgeBase string `json:"knowledge_base_id,omitempty"`
	Document      string `json:"document_id"`
	Thread        string `json:"thread_id,omitempty"`
	Request       string `json:"request_id,omitempty"`
	Round         string `json:"round_id,omitempty"`
}

type Entry struct {
	ID, Recipient uint64
	Event, Kind   string
	Target        Target
	ReadAt        *time.Time
}

type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type scanner interface{ Scan(...any) error }

const columns = "id,recipient_id,event_id,kind,target_json,read_at"

func scan(row scanner) (Entry, error) {
	var value Entry
	var raw []byte
	var read sql.NullTime
	if err := row.Scan(&value.ID, &value.Recipient, &value.Event, &value.Kind, &raw, &read); err != nil {
		return value, err
	}
	if json.Unmarshal(raw, &value.Target) != nil || !validTarget(value.Kind, value.Target) {
		return value, errors.New("NOTIFICATION_TARGET_INVALID")
	}
	if read.Valid {
		value.ReadAt = &read.Time
	}
	return value, nil
}

// 组织事件与文档事件有不同的授权来源，不能用空文档或混合目标绕过校验。
func validTarget(kind string, target Target) bool {
	if kind == "ORG_OWNER_TRANSFER" {
		return target.Organization != "" && target.Document == "" && target.KnowledgeBase == "" && target.Thread == "" && target.Request == "" && target.Round == ""
	}
	if kind == "RESPONSIBLE_DEPARTURE" {
		return target.Document != "" && target.KnowledgeBase != "" && target.Organization == "" && target.Thread == "" && target.Request == "" && target.Round == ""
	}
	return target.Document != "" && target.Organization == "" && target.KnowledgeBase == ""
}

func Row(ctx context.Context, q Queryer, id, recipient uint64, lock bool) (Entry, error) {
	query := "SELECT " + columns + " FROM notification WHERE id=? AND recipient_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scan(q.QueryRowContext(ctx, query, id, recipient))
}

func Candidates(ctx context.Context, q Queryer, recipient, after uint64, limit int) ([]Entry, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+columns+" FROM notification WHERE recipient_id=? AND id>? ORDER BY id LIMIT ?", recipient, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 已读时间只在第一次操作落定；原号重试及另一个已读操作都不改写时间。
func MarkRead(ctx context.Context, tx *sql.Tx, id, recipient uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?", id, recipient)
	return err
}

// recipients.go：保留原实现，按业务合并文件。

// Facts只读取调用方传入的同一连接/事务，不能用另一个池拼接不同时刻的权限来源。
// 组织管理员来自调用方已核验的身份服务结果，本仓储不跨私有库猜测组织角色。
func Facts(ctx context.Context, q Queryer, document dto.Document, space dto.Space, user uint64, orgAdmin bool) (domain.PermissionFacts, error) {
	facts := domain.PermissionFacts{Actor: user}
	if user == 0 || document.SpaceId != space.Id {
		return facts, errors.New("NOTIFICATION_PERMISSION_TARGET_INVALID")
	}
	id, err := domain.CanonicalID(document.Id)
	if err != nil {
		return facts, err
	}
	actor := domain.TextID(user)
	facts.Responsible = document.ResponsibleUserId != nil && *document.ResponsibleUserId == actor
	facts.PersonalOwner = space.Kind == "PERSONAL" && space.OwnerUserId != nil && *space.OwnerUserId == actor
	facts.OrganizationAdmin = space.Kind == "ORGANIZATION" && orgAdmin
	if document.KnowledgeBaseId != nil {
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m
			JOIN knowledge_base k ON k.id=m.knowledge_base_id
			WHERE m.knowledge_base_id=? AND m.user_id=? AND m.state='ACTIVE' AND m.role='ADMIN'
			AND k.space_id=? AND k.lifecycle='ACTIVE')`, *document.KnowledgeBaseId, user, space.Id).Scan(&facts.KnowledgeBaseAdmin)
		if err != nil {
			return facts, err
		}
	}
	facts.Grants, err = grants(ctx, q, id, user)
	if err != nil {
		return facts, err
	}
	facts.Units, err = units(ctx, q, id, document.CurrentStructureId)
	return facts, err
}

func grants(ctx context.Context, q Queryer, document, user uint64) ([]domain.GrantFact, error) {
	rows, err := q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant
		WHERE document_id=? AND user_id=? AND state='ACTIVE' ORDER BY id`, document, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.GrantFact{}
	for rows.Next() {
		var value domain.GrantFact
		if err = rows.Scan(&value.User, &value.Kind, &value.Scope, &value.Capability); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func units(ctx context.Context, q Queryer, document uint64, structure string) ([]domain.UnitFact, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u
		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=?
		WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id`, structure, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.UnitFact{}
	for rows.Next() {
		var value domain.UnitFact
		if err = rows.Scan(&value.ID, &value.Chapter); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// 返回真实参与者全量去重集合；墓碑仍保留作者身份，不静默截断而导致后面的参与者永久漏通知。
// 当前读取资格由上层逐人重新核验，此处的参与事实本身不等于最终投递许可。
func CommentRecipients(ctx context.Context, q Queryer, thread, actor uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT author_id FROM (
		SELECT author_id FROM comment_thread WHERE id=?
		UNION SELECT author_id FROM comment WHERE thread_id=?
	) participants WHERE author_id<>? ORDER BY author_id`, thread, thread, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var user uint64
		if err = rows.Scan(&user); err != nil {
			return nil, err
		}
		if user == 0 {
			return nil, errors.New("NOTIFICATION_RECIPIENT_INVALID")
		}
		values = append(values, user)
	}
	return values, rows.Err()
}

// review_binding.go：保留原实现，按业务合并文件。

// ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。
type ReviewEvent struct {
	Document, Request, Round, Unit, Actor uint64
	Kind                                  string
	Payload                               map[string]string
}

func readIDs(rows *sql.Rows) ([]uint64, error) {
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, errors.New("NOTIFICATION_CANDIDATE_INVALID")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func ReviewBinding(ctx context.Context, q Queryer, task Task) (ReviewEvent, error) {
	var event ReviewEvent
	if json.Unmarshal(task.Payload, &event.Payload) != nil {
		return event, errors.New("REVIEW_NOTIFICATION_PAYLOAD_INVALID")
	}
	p := event.Payload
	docKey, requestKey, roundKey := "document", "request", "round"
	if task.Kind == "REVIEW_NOTIFICATION" {
		docKey, requestKey, roundKey = "document_id", "request_id", "round_id"
	}
	var err error
	if event.Document, err = domain.CanonicalID(p[docKey]); err != nil {
		return event, err
	}
	if event.Request, err = domain.CanonicalID(p[requestKey]); err != nil {
		return event, err
	}
	if event.Round, err = domain.CanonicalID(p[roundKey]); err != nil {
		return event, err
	}
	var submitter, roundNo uint64
	var submission string
	err = q.QueryRowContext(ctx, `SELECT unit_id,submitter_id,round_no,submission_operation_id FROM review_round
		WHERE id=? AND request_id=? AND document_id=?`, event.Round, event.Request, event.Document).
		Scan(&event.Unit, &submitter, &roundNo, &submission)
	if err != nil {
		return event, err
	}
	var actor uint64
	var receiptKind, state string
	err = q.QueryRowContext(ctx, `SELECT actor_id,kind,state FROM operation_receipt WHERE operation_id=?`, task.Operation).Scan(&actor, &receiptKind, &state)
	if err != nil {
		return event, err
	}
	if state != "SUCCEEDED" || actor == 0 {
		return event, errors.New("REVIEW_NOTIFICATION_RECEIPT_INVALID")
	}
	event.Actor = actor
	switch task.Kind {
	case "REVIEW_NOTIFICATION":
		if p["kind"] != "SUBMISSION" || submission != task.Operation || submitter != actor || (receiptKind != "SUBMISSION" && receiptKind != "REVERSE") {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		event.Kind = "REVIEW_SUBMITTED"
		if roundNo > 1 {
			event.Kind = "REVIEW_RESUBMITTED"
		}
	case "REVIEW_ACTION_NOTIFICATION":
		var action uint64
		if action, err = domain.CanonicalID(p["action"]); err != nil {
			return event, err
		}
		var actualActor uint64
		var actualKind string
		err = q.QueryRowContext(ctx, `SELECT actor_id,kind FROM review_action WHERE id=? AND request_id=? AND round_id=? AND operation_id=? AND item_no=0`, action, event.Request, event.Round, task.Operation).Scan(&actualActor, &actualKind)
		if err != nil {
			return event, err
		}
		if actualActor != actor || p["actor"] != domain.TextID(actor) || actualKind != p["event"] {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		expectedKind := actualKind
		if actualKind == "WITHDRAW" {
			expectedKind = "REVIEW_WITHDRAW"
		}
		if actualKind == "ARCHIVE" {
			expectedKind = "DISCARD_DRAFT"
		}
		if receiptKind != expectedKind {
			return event, errors.New("REVIEW_NOTIFICATION_RECEIPT_INVALID")
		}
		event.Kind = "REVIEW_ACTION"
	case "MERGE_NOTIFICATION":
		var record uint64
		if record, err = domain.CanonicalID(p["record"]); err != nil {
			return event, err
		}
		var valid bool
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merge_operation m JOIN merge_record r ON r.id=m.result_merge_id
			AND r.operation_id=m.operation_id AND r.request_id=m.request_id AND r.round_id=m.round_id
			WHERE m.operation_id=? AND m.request_id=? AND m.round_id=? AND m.actor_id=? AND m.state='SUCCEEDED' AND r.id=?)`, task.Operation, event.Request, event.Round, actor, record).Scan(&valid)
		if err != nil {
			return event, err
		}
		if !valid || receiptKind != "MERGE" || p["event"] != "APPROVE_AND_MERGE" || p["actor"] != domain.TextID(actor) {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		event.Kind = "REVIEW_MERGED"
	default:
		return event, errors.New("REVIEW_NOTIFICATION_KIND_INVALID")
	}
	return event, nil
}

// 候选与资格分开：这里只读Doc自己的真实角色/参与记录，组织管理候选由Account提供。
// 首次领取后固化整个集合，后续分页恢复不把新加入的人混入旧投递。
func ReviewCandidates(ctx context.Context, q Queryer, event ReviewEvent) ([]uint64, []uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT user_id FROM (
		SELECT responsible_user_id AS user_id FROM document WHERE id=?
		UNION SELECT s.account_owner_id FROM space s JOIN document d ON d.space_id=s.id WHERE d.id=?
		UNION SELECT user_id FROM content_grant WHERE document_id=? AND state='ACTIVE' AND scope_kind='DOCUMENT' AND capability IN ('REVIEW','MANAGE')
		UNION SELECT m.user_id FROM knowledge_base_member m JOIN document d ON d.knowledge_base_id=m.knowledge_base_id
		WHERE d.id=? AND m.state='ACTIVE' AND m.role='ADMIN'
	) candidates WHERE user_id IS NOT NULL AND user_id<>? ORDER BY user_id`, event.Document, event.Document, event.Document, event.Document, event.Actor)
	if err != nil {
		return nil, nil, err
	}
	ids, err := readIDs(rows)
	if err != nil {
		return nil, nil, err
	}
	participants := []uint64{}
	if event.Kind == "REVIEW_ACTION" || event.Kind == "REVIEW_MERGED" {
		rows, err = q.QueryContext(ctx, `SELECT user_id FROM (
			SELECT user_id FROM request_submitter WHERE request_id=?
			UNION SELECT actor_id FROM review_action WHERE request_id=?
		) participants WHERE user_id<>? ORDER BY user_id`, event.Request, event.Request, event.Actor)
		if err != nil {
			return nil, nil, err
		}
		participants, err = readIDs(rows)
	}
	return ids, participants, err
}

// review_readiness.go：保留原实现，按业务合并文件。

// 全部待投递审核任务须有真实成功回执与原轮次/动作/合并记录。候选快照的增加
// 不改变这些不可变字段；合法积压允许恢复，绑定损坏必须阻止就绪。
func InvalidReviewTask(ctx context.Context, db *sql.DB) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		LEFT JOIN review_round rr ON rr.id=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round')))
		LEFT JOIN review_action a ON a.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.action')) AND a.operation_id=t.operation_id AND a.item_no=0
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN merge_record r ON r.id=m.result_merge_id AND r.operation_id=m.operation_id
		WHERE t.kind IN ('REVIEW_NOTIFICATION','REVIEW_ACTION_NOTIFICATION','MERGE_NOTIFICATION')
		AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
		t.state NOT IN ('QUEUED','RETRY','UNKNOWN') OR COALESCE(
		t.business_key=o.operation_id AND o.state='SUCCEEDED' AND o.actor_id>0
		AND CAST(rr.document_id AS CHAR)=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document')))
		AND CAST(rr.request_id AS CHAR)=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request')))
		AND ((t.kind='REVIEW_NOTIFICATION' AND rr.submission_operation_id=o.operation_id AND rr.submitter_id=o.actor_id
		AND o.kind IN ('SUBMISSION','REVERSE') AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.kind'))='SUBMISSION')
		OR (t.kind='REVIEW_ACTION_NOTIFICATION' AND a.request_id=rr.request_id AND a.round_id=rr.id AND a.actor_id=o.actor_id
		AND CAST(a.actor_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))
		AND a.kind=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))
		AND o.kind=CASE a.kind WHEN 'WITHDRAW' THEN 'REVIEW_WITHDRAW' WHEN 'ARCHIVE' THEN 'DISCARD_DRAFT' ELSE a.kind END)
		OR (t.kind='MERGE_NOTIFICATION' AND o.kind='MERGE' AND m.state='SUCCEEDED' AND m.actor_id=o.actor_id
		AND m.request_id=rr.request_id AND m.round_id=rr.id AND r.request_id=m.request_id AND r.round_id=m.round_id
		AND CAST(r.id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.record'))
		AND CAST(o.actor_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))
		AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))='APPROVE_AND_MERGE')),0)=0))`).Scan(&invalid)
	return invalid, err
}

// tasks.go：保留原实现，按业务合并文件。

type Task struct {
	ID, Generation         uint64
	Kind, Operation, State string
	Payload                []byte
}

const taskColumns = "id,generation,kind,COALESCE(operation_id,''),state,payload"

func scanTask(row scanner) (Task, error) {
	var task Task
	err := row.Scan(&task.ID, &task.Generation, &task.Kind, &task.Operation, &task.State, &task.Payload)
	return task, err
}

// 此处只读最早候选，不提前持锁跨服务查询收件资格；最终事务必须LockedTask并复核原代次和状态。
// 显式kind集合避免尚未装配的通知处理器误取其他持久任务。
func NextTask(ctx context.Context, db *sql.DB, kinds []string, recent bool) (Task, error) {
	if len(kinds) == 0 {
		return Task{}, sql.ErrNoRows
	}
	args := make([]any, len(kinds))
	marks := make([]string, len(kinds))
	for i, kind := range kinds {
		if kind == "" {
			return Task{}, errors.New("NOTIFICATION_TASK_KIND_INVALID")
		}
		args[i], marks[i] = kind, "?"
	}
	order := "ASC"
	if recent {
		order = "DESC"
	}
	query := "SELECT " + taskColumns + " FROM durable_task WHERE kind IN (" + strings.Join(marks, ",") + ") AND state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6) ORDER BY id " + order + " LIMIT 1"
	return scanTask(db.QueryRowContext(ctx, query, args...))
}

func LockedTask(ctx context.Context, tx *sql.Tx, id uint64) (Task, error) {
	return scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM durable_task WHERE id=? FOR UPDATE", id))
}

// 失败只安排原任务重试，避免一个暂时不可用的目标饿死后面的通知；提交未知
// 若实际上已经推进代次/完成，条件更新不会把它退回排队状态。
func RetryTask(ctx context.Context, db *sql.DB, task Task) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 SECOND),
		attempts=IF(attempts<4294967295,attempts+1,attempts),last_error='NOTIFICATION_DELIVERY_RETRY'
		WHERE id=? AND generation=? AND state=?`, task.ID, task.Generation, task.State)
	return err
}

// 与各收件人的Insert使用同一业务事务；提交未知可重新执行，终态CAS防止第二次推进原任务。
func FinishTask(ctx context.Context, tx *sql.Tx, task Task) (bool, error) {
	if task.ID == 0 || (task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN") {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL
		WHERE id=? AND generation=? AND state=? AND kind=? AND COALESCE(operation_id,'')=?`, task.ID, task.Generation, task.State, task.Kind, task.Operation)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func Insert(ctx context.Context, tx *sql.Tx, recipient uint64, event, kind string, target Target) error {
	if recipient == 0 || event == "" || kind == "" {
		return errors.New("NOTIFICATION_IDENTITY_INVALID")
	}
	for _, id := range []string{target.Document, target.Thread, target.Request, target.Round, target.Organization, target.KnowledgeBase} {
		if id != "" {
			if _, err := domain.CanonicalID(id); err != nil {
				return errors.New("NOTIFICATION_TARGET_INVALID")
			}
		}
	}
	if !validTarget(kind, target) {
		return errors.New("NOTIFICATION_TARGET_INVALID")
	}
	raw, err := json.Marshal(target)
	if err != nil {
		return err
	}
	// 唯一键只负责幂等占位，重复投递绝不改写目标或read_at；随后持锁比较完整目标。
	_, err = tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?)
		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))
	if err != nil {
		return err
	}
	var existingRaw []byte
	var existingKind string
	if err = tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification
		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind).Scan(&existingKind, &existingRaw); err != nil {
		return err
	}
	var existing Target
	decoder := json.NewDecoder(bytes.NewReader(existingRaw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&existing) != nil || decoder.Decode(new(any)) != io.EOF || existingKind != kind || existing != target {
		return errors.New("NOTIFICATION_TARGET_CONFLICT")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`41d9b5bde3727be549511cfbd18cffde145b78b1d799ca6ef7be30d32d52d236`。[参考实现](../../../../services/document/internal/repository/notification/notification_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
