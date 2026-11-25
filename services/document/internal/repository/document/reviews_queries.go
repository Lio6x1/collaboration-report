// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// review_actions.go：保留原实现，按业务合并文件。

func ReviewPointerRow(ctx context.Context, q Queryer, id uint64) (domain.ReviewPointer, error) {
	var value domain.ReviewPointer
	err := q.QueryRowContext(ctx, "SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?", id).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Round, &value.Revision, &value.Status, &value.Archived)
	return value, err
}

func ReviewRoundVersion(ctx context.Context, q Queryer, request, round uint64) (uint64, error) {
	var version uint64
	err := q.QueryRowContext(ctx, "SELECT content_version_id FROM review_round WHERE id=? AND request_id=?", round, request).Scan(&version)
	return version, err
}

func EverSubmitted(ctx context.Context, q Queryer, request, actor uint64) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)", request, actor).Scan(&exists)
	return exists, err
}

func LockedChange(ctx context.Context, tx *sql.Tx, request, id uint64) (domain.ChangeFact, error) {
	var value domain.ChangeFact
	err := tx.QueryRowContext(ctx, "SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE", id, request).Scan(&value.ID, &value.Request, &value.Round, &value.Author, &value.Revision, &value.Status)
	return value, err
}

func InsertReviewAction(ctx context.Context, tx *sql.Tx, operation, kind string, request, round, actor, version, change uint64, reason *string) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),NULLIF(?,0))", request, round, actor, kind, operation, reason, version, change)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func InsertChange(ctx context.Context, tx *sql.Tx, request, round, action, actor uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,'OPEN')", request, round, action, actor)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func ResolveChange(ctx context.Context, tx *sql.Tx, change, action uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE change_request SET status='RESOLVED',resolved_action_id=?,revision=revision+1 WHERE id=?", action, change)
	return err
}

func AdvanceReviewRevision(ctx context.Context, tx *sql.Tx, request uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE review_request SET revision=revision+1 WHERE id=?", request)
	return err
}

func WithdrawReview(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer) error {
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='WITHDRAWN',active_merge_operation_id=NULL,revision=revision+1 WHERE id=?", pointer.ID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=?,generation=generation+1 WHERE unit_id=?", pointer.ID, pointer.Unit)
	return err
}

func QueueReviewActionNotification(ctx context.Context, tx *sql.Tx, operation, kind string, pointer domain.ReviewPointer, action, actor uint64) error {
	// 通知意图与动作同事务；精确收件人与实际投递由 Task 9 读取权威事实后处理。
	payload, err := json.Marshal(map[string]string{"event": kind, "action": domain.TextID(action), "actor": domain.TextID(actor), "document": domain.TextID(pointer.Document), "request": domain.TextID(pointer.ID), "round": domain.TextID(pointer.Round)})
	if err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "REVIEW_ACTION_NOTIFICATION", operation, operation, payload, "QUEUED")
}

func UnboundReviewActionNotifications(ctx context.Context, q Queryer) (bool, error) {
	var invalid bool
	// 缺失或不一致的持久关联阻止启动；合法通知积压不伪装为已投递，也不冻结所有读取。
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='REVIEW_ACTION_NOTIFICATION' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND NOT EXISTS(SELECT 1 FROM operation_receipt o JOIN review_action a ON a.operation_id=o.operation_id AND a.item_no=0 JOIN review_request r ON r.id=a.request_id WHERE o.operation_id=t.operation_id AND o.state='SUCCEEDED' AND t.business_key=o.operation_id AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.action'))=CAST(a.id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))=CAST(a.actor_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document'))=CAST(r.document_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request'))=CAST(r.id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round'))=CAST(a.round_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))=a.kind))").Scan(&invalid)
	return invalid, err
}

// review_readings.go：保留原实现，按业务合并文件。

func ReviewRequestRow(ctx context.Context, q Queryer, id uint64) (dto.ReviewRequest, error) {
	var value dto.ReviewRequest
	var archived sql.NullTime
	var count int64
	err := q.QueryRowContext(ctx, "SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.revision AS CHAR),r.archived_at,(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN') FROM review_request r WHERE r.id=?", id).Scan(&value.Id, &value.DocumentId, &value.UnitId, &value.BranchId, &value.Status, &value.CurrentRoundId, &value.Revision, &archived, &count)
	if err != nil {
		return value, err
	}
	if archived.Valid {
		text := archived.Time.UTC().Format(time.RFC3339Nano)
		value.ArchivedAt = &text
	}
	value.BlockingChangeCount = &count
	value.Actions = []string{}
	return value, nil
}

func ReviewRoundRow(ctx context.Context, tx *sql.Tx, request, round uint64, document string) (dto.ReviewRound, error) {
	var value dto.ReviewRound
	var operation string
	err := tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_id AS CHAR),CAST(structure_id AS CHAR),CAST(lineage_id AS CHAR),CAST(submitter_id AS CHAR),submission_operation_id FROM review_round WHERE id=? AND request_id=? AND document_id=?", round, request, document).Scan(&value.Id, &value.RequestId, &value.RoundNo, &value.ContentVersionId, &value.BaseManifestId, &value.BaseVersionId, &value.StructureId, &value.LineageId, &value.SubmitterId, &operation)
	if err != nil {
		return value, err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=? AND operation_id=? AND state='SUCCEEDED'", value.Id, operation).Scan(&raw); err != nil {
		return value, err
	}
	style, legacy, err := frozenRoundStyle(raw)
	if err != nil {
		return value, err
	}
	if legacy {
		if err = validateLegacyReverseStyle(ctx, tx, operation, document, value.ContentVersionId, style); err != nil {
			return value, err
		}
	}
	value.Content, err = ContentRow(ctx, tx, value.ContentVersionId, document)
	if err != nil {
		return value, err
	}
	value.Structure, err = StructureRow(ctx, tx, value.StructureId, document)
	if err != nil {
		return value, err
	}
	value.Style, err = StyleRow(ctx, tx, style, document)
	return value, err
}

func ReviewRequestSummaryRow(ctx context.Context, tx *sql.Tx, request, round, document uint64) (dto.ReviewRequestSummary, []byte, error) {
	var value dto.ReviewRequestSummary
	var body []byte
	var submitted time.Time
	// 题名和正文必须共同绑定当前固定轮次；不读取MAIN/共同草稿，也不为列表拉样式或资源。
	err := tx.QueryRowContext(ctx, "SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_round_id AND r.request_id=q.id AND r.document_id=q.document_id AND r.unit_id=q.unit_id JOIN content_version v ON v.id=r.content_version_id AND v.document_id=r.document_id AND v.unit_id=r.unit_id AND v.structure_id=r.structure_id JOIN structure_entry e ON e.structure_id=r.structure_id AND e.document_id=r.document_id AND e.unit_id=r.unit_id WHERE q.id=? AND r.id=? AND q.document_id=?", request, round, document).Scan(&value.Title, &body, &value.RoundNo, &value.SubmitterId, &submitted)
	if err != nil {
		return value, nil, err
	}
	value.SubmittedAt = submitted.UTC().Format(time.RFC3339Nano)
	return value, body, nil
}

func ReviewRequestCandidates(ctx context.Context, q Queryer, document uint64, last string, limit int) ([]uint64, error) {
	// 全局候选只返回稳定 ID；访问筛选在业务层执行，禁止把 SQL 的总量作为可见数量。
	rows, err := q.QueryContext(ctx, "SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id>? ORDER BY id LIMIT ?", document, document, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

func ScopedReviewRequestCandidates(ctx context.Context, q Queryer, actor uint64, organizations []uint64, last string, limit int) ([]uint64, error) {
	// 私有库只做可能有REVIEW/MANAGE的范围筛选；角色与来源是否仍有效留给逐项权威准入及最终事务。
	statement := `SELECT r.id FROM review_request r JOIN document d ON d.id=r.document_id JOIN space s ON s.id=d.space_id
		WHERE r.id>? AND d.lifecycle='ACTIVE' AND (
		d.responsible_user_id=? OR s.account_owner_id=?
		OR EXISTS(SELECT 1 FROM content_grant g WHERE g.document_id=d.id AND g.user_id=? AND g.state='ACTIVE' AND g.scope_kind='DOCUMENT' AND g.capability IN ('REVIEW','MANAGE'))
		OR EXISTS(SELECT 1 FROM knowledge_base_member k WHERE k.knowledge_base_id=d.knowledge_base_id AND k.user_id=? AND k.state='ACTIVE' AND k.role='ADMIN')`
	args := []any{last, actor, actor, actor, actor}
	if len(organizations) > 0 {
		statement += " OR s.organization_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(organizations)), ",") + ")"
		for _, organization := range organizations {
			args = append(args, organization)
		}
	}
	statement += ") ORDER BY r.id LIMIT ?"
	args = append(args, limit)
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

func ReviewRoundIDs(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, "SELECT id FROM review_round WHERE request_id=? AND id>? ORDER BY id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

func ReviewChanges(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ChangeRequest, error) {
	rows, err := q.QueryContext(ctx, "SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.reason,z.reason,CAST(c.revision AS CHAR),c.created_at FROM change_request c JOIN review_action a ON a.id=c.created_action_id AND a.request_id=c.request_id LEFT JOIN review_action z ON z.id=c.resolved_action_id AND z.request_id=c.request_id WHERE c.request_id=? AND c.id>? ORDER BY c.id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.ChangeRequest{}
	for rows.Next() {
		var value dto.ChangeRequest
		var created time.Time
		if err = rows.Scan(&value.Id, &value.RequestId, &value.RoundId, &value.ContentVersionId, &value.AuthorId, &value.Status, &value.Reason, &value.ResolutionReason, &value.Revision, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}

func ReviewActions(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ReviewAction, error) {
	rows, err := q.QueryContext(ctx, "SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST(COALESCE(a.target_change_id,c.id) AS CHAR),CAST(a.merge_record_id AS CHAR),a.created_at FROM review_action a LEFT JOIN change_request c ON c.created_action_id=a.id AND c.request_id=a.request_id WHERE a.request_id=? AND a.id>? ORDER BY a.id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.ReviewAction{}
	for rows.Next() {
		var value dto.ReviewAction
		var created time.Time
		if err = rows.Scan(&value.Id, &value.RequestId, &value.RoundId, &value.ActorId, &value.Type, &value.ContentVersionId, &value.Reason, &value.ChangeRequestId, &value.MergeRecordId, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}

// review_round_style.go：保留原实现，按业务合并文件。

func frozenRoundStyle(raw []byte) (string, bool, error) {
	invalid := errors.New("REVIEW_ROUND_CONTEXT_INVALID")
	var fixed struct {
		Style json.RawMessage `json:"style_id"`
	}
	if json.Unmarshal(raw, &fixed) != nil {
		return "", false, invalid
	}
	var style string
	if json.Unmarshal(fixed.Style, &style) == nil {
		if _, err := domain.CanonicalID(style); err == nil {
			return style, false, nil
		}
		return "", false, invalid
	}
	// 第一版REVERSE生产者写过数值ID。只按uint64精确解析，不经float64，不接受指数、小数或零。
	var legacy uint64
	if json.Unmarshal(fixed.Style, &legacy) != nil || legacy == 0 {
		return "", false, invalid
	}
	return domain.TextID(legacy), true, nil
}

func validateLegacyReverseStyle(ctx context.Context, tx *sql.Tx, operation, document, version, style string) error {
	// 兼容仅限同一原REVERSE操作、真实REVERSE版本和不可变来源中的原样式；不改写旧任务或套用当前样式。
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t
		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='REVERSE' AND r.state='SUCCEEDED'
		JOIN content_version v ON v.id=? AND v.document_id=? AND v.source_kind='REVERSE' AND v.source_ref=r.operation_id
		WHERE t.kind='REVERSE_CONTEXT' AND t.business_key=? AND t.operation_id=?`, version, document, operation, operation).Scan(&raw)
	if err != nil {
		return err
	}
	var original struct {
		Scope   struct{ Document, Style uint64 }
		Context struct {
			Operation string `json:"operation_id"`
		}
	}
	if json.Unmarshal(raw, &original) != nil || original.Context.Operation != operation ||
		domain.TextID(original.Scope.Document) != document || domain.TextID(original.Scope.Style) != style {
		return errors.New("REVIEW_ROUND_CONTEXT_INVALID")
	}
	return nil
}
