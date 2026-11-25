// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// personal_transfer.go：保留原实现，按业务合并文件。

const PersonalTransferTask = "PERSONAL_TRANSFER_RECOVER"

func PersonalTransferRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PersonalTransfer, error) {
	var p domain.PersonalTransfer
	var raw []byte
	var accept sql.NullString
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, `SELECT t.id,t.document_id,t.document_title,t.source_user_id,t.target_user_id,t.source_space_id,
 t.expected_control_revision,t.revision,t.state,t.offer_operation_id,t.accept_operation_id,t.fixed_plan,t.expires_at,UTC_TIMESTAMP(6)
 FROM personal_transfer t WHERE t.id=?`+suffix, id).Scan(&p.ID, &p.Document, &p.Title, &p.SourceUser, &p.TargetUser, &p.SourceSpace, &p.ExpectedControl, &p.Revision, &p.State, &p.OfferOperation, &accept, &raw, &p.Expires, &p.Now)
	if err != nil {
		return p, err
	}
	p.AcceptOperation = accept.String
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &p.Plan); err != nil {
			return p, errors.New("TRANSFER_PLAN_INVALID")
		}
		if !p.Plan.Valid() {
			return p, errors.New("TRANSFER_PLAN_INVALID")
		}
	}
	return p, nil
}

func InsertPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransfer) (uint64, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO personal_transfer(document_id,document_title,source_user_id,target_user_id,source_space_id,expected_control_revision,state,offer_operation_id,expires_at)
 VALUES(?,?,?,?,?,?,'PENDING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, p.Document, p.Title, p.SourceUser, p.TargetUser, p.SourceSpace, p.ExpectedControl, p.OfferOperation)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func PersonalTransferIDs(ctx context.Context, tx *sql.Tx, user uint64, direction string, last uint64, limit int) ([]uint64, error) {
	field := "target_user_id"
	if direction == "outgoing" {
		field = "source_user_id"
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM personal_transfer WHERE `+field+`=? AND id>? ORDER BY id LIMIT ?`, user, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func ClosePersonalTransfer(ctx context.Context, tx *sql.Tx, id, revision uint64, state string) error {
	result, err := tx.ExecContext(ctx, `UPDATE personal_transfer SET state=?,revision=revision+1 WHERE id=? AND revision=? AND state='PENDING'`, state, id, revision)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_CHANGED")
	}
	return nil
}

func TransferInterlock(ctx context.Context, tx *sql.Tx, document uint64) error {
	pending, err := DocumentWritesPending(ctx, tx, document, "")
	if err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "DOCUMENT_OPERATION_PENDING")
	}
	var blocked int
	// 已登记的未知正文/结构任务必须先结束；不能固定旧版本后让迟到的 COMMIT 改写转让范围。
	err = tx.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state IN ('PROTECTED','UNKNOWN'))
 OR EXISTS(SELECT 1 FROM personal_transfer WHERE document_id=? AND state='ACCEPTING')`, document, document).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked != 0 {
		return domain.Violation(domain.StateConflict, "DOCUMENT_OPERATION_PENDING")
	}
	return nil
}

func TransferBodyCapacity(ctx context.Context, tx *sql.Tx, document uint64) (uint64, uint64, error) {
	var total, paid uint64
	// 版本 byte_size 由 canonical 编码器写入；计入全部历史，不只当前 manifest。
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&total); err != nil {
		return 0, 0, err
	}
	err := tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&paid)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}
	return total, paid, nil
}

func BeginPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	if !p.Valid() {
		return errors.New("TRANSFER_PLAN_INVALID")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE personal_transfer SET state='ACCEPTING',revision=revision+1,accept_operation_id=?,fixed_plan=? WHERE id=? AND state='PENDING'`, p.Operation, string(raw), p.Transfer)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_CHANGED")
	}
	result, err = tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',control_revision=?,acl_revision=?,publication_epoch=?,publication_state='UNPUBLISHED',current_release_id=NULL,
 visibility_policy='COLLABORATORS',visibility_revision=visibility_revision+1
 WHERE id=? AND lifecycle='ACTIVE' AND control_revision=? AND space_id=? AND responsible_user_id=?`, p.ClosingControl, p.ACL+1, p.Publication+1, p.Document, p.ExpectedControl, p.SourceSpace, p.SourceUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "DOCUMENT_CHANGED")
	}
	return InsertSubmissionTask(ctx, tx, PersonalTransferTask, p.Operation, p.Operation, raw, "QUEUED")
}

func ClaimPersonalTransfer(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? AND
 ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))
 ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, PersonalTransferTask).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, errors.New("TRANSFER_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?`, instance, task.Generation, task.ID)
	return task, err
}

func LockPersonalTransferTask(ctx context.Context, tx *sql.Tx, t PurgeTask, instance string) (domain.PersonalTransfer, error) {
	var generation, id uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, `SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE`, t.ID, PersonalTransferTask, t.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return domain.PersonalTransfer{}, err
	}
	if generation != t.Generation || state != "RUNNING" || owner != instance {
		return domain.PersonalTransfer{}, domain.Violation(domain.StateConflict, "TRANSFER_WORKER_CHANGED")
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM personal_transfer WHERE accept_operation_id=?`, t.Operation).Scan(&id); err != nil {
		return domain.PersonalTransfer{}, err
	}
	p, err := PersonalTransferRow(ctx, tx, id, true)
	if err != nil {
		return p, err
	}
	if p.State != "ACCEPTING" || p.Plan.Operation != t.Operation || p.Plan.Transfer != p.ID || p.Plan.Document != p.Document {
		return p, errors.New("TRANSFER_PLAN_CHANGED")
	}
	return p, nil
}

func RetryPersonalTransfer(ctx context.Context, db *sql.DB, t PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='TRANSFER_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'`, t.ID, t.Generation, instance)
	return err
}

func CompletePersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	// 排空与 TransferInterlock 已确保所有正文预留结算。移动高水位归属，
	// 下一次编辑只预留新的差额，不给原房间再次扣整篇正文。
	if _, err := tx.ExecContext(ctx, "UPDATE draft_body_capacity SET space_id=? WHERE document_id=? AND space_id=?", p.TargetSpace, p.Document, p.SourceSpace); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE document SET space_id=?,knowledge_base_id=?,responsible_user_id=?,lifecycle='ACTIVE',control_revision=control_revision+1
 WHERE id=? AND lifecycle='CLOSING' AND space_id=? AND responsible_user_id=? AND control_revision=? AND acl_revision=? AND publication_epoch=?`, p.TargetSpace, p.TargetKB, p.TargetUser, p.Document, p.SourceSpace, p.SourceUser, p.ClosingControl, p.ACL+1, p.Publication+1)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TRANSFER_DOCUMENT_CHANGED")
	}
	for _, statement := range []string{
		`UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state<>'CLOSED'`,
		`UPDATE document_invitation SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state='OPEN'`,
		`UPDATE document_application SET state='CANCELLED',revision=revision+1 WHERE document_id=? AND state='PENDING'`,
	} {
		if _, err = tx.ExecContext(ctx, statement, p.Document); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,node_bytes,operation_id) VALUES(?,?,?,?,?)
 ON DUPLICATE KEY UPDATE space_id=VALUES(space_id),bytes=VALUES(bytes),node_bytes=VALUES(node_bytes),operation_id=VALUES(operation_id)`, p.Document, p.TargetSpace, p.BodyBytes+p.NodeBytes, p.NodeBytes, p.Operation); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='ACCEPTED',revision=revision+1 WHERE id=? AND state='ACCEPTING' AND accept_operation_id=?`, p.Transfer, p.Operation); err != nil {
		return err
	}
	// 旧 owner 的其它申请不随文档归属继续生效；标题快照也不泄漏新 owner 的后续改名。
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='CANCELLED',revision=revision+1 WHERE document_id=? AND state='PENDING'`, p.Document); err != nil {
		return err
	}
	// 负责人通知的既定含义是已经完成；申请阶段不能复用该通知冒充迁移。
	if err = NotifyResponsibilityTransfer(ctx, tx, p.TargetUser, p.Operation, []byte(`{"document_id":"`+domain.TextID(p.Document)+`"}`)); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"operation_id": p.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(p.Document)}})
	result, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='PERSONAL_TRANSFER_ACCEPT' AND state='PREPARING'`, string(raw), p.Operation, p.TargetUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_RECEIPT_CHANGED")
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE kind=? AND operation_id=?`, PersonalTransferTask, p.Operation)
	return err
}

// Account 固定父空间范围之前调用；源或目标的已接受固定关闭都必须先完成真实迁移。
func PersonalTransferAccountPending(ctx context.Context, q Queryer, user uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_transfer WHERE state='ACCEPTING' AND (source_user_id=? OR target_user_id=?)`, user, user).Scan(&n)
	return n != 0, err
}

func TransferFenceConfirmed(ctx context.Context, tx *sql.Tx, scope domain.PurgeScope) error {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c WHERE revocation_id=? AND scope_key=? AND epoch=? AND state='SUCCEEDED'
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.participant='collaboration' AND a.phase='OUTPUT_STOPPED')`, scope.Operation, scope.Key, scope.Epoch).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("TRANSFER_FENCE_NOT_CONFIRMED")
	}
	return nil
}

// personal_transfer_interlock.go：保留原实现，按业务合并文件。

// 父 KB 删除必须同时看原范围与接收计划，不能只看当前 children：接受尚未完成
// 时 Doc 仍在源 KB，接收目标 KB 没有这个子文档。调用方先锁 KB 来源 gate。
func PersonalTransferKnowledgeBasePending(ctx context.Context, q Queryer, kb uint64) (bool, error) {
	var pending bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personal_transfer t JOIN document d ON d.id=t.document_id
 WHERE t.state='ACCEPTING' AND (d.knowledge_base_id=? OR JSON_UNQUOTE(JSON_EXTRACT(t.fixed_plan,'$.TargetKB'))=?))`, kb, kb).Scan(&pending)
	return pending, err
}

// 只有固定 transfer driver 的真实终态才能开放更高 Doc 来源；关闭子任务已经
// SUCCEEDED 不能代表父迁移已经完成。按当前来源锁序调用，以免提前重开旧范围。
func PersonalTransferSourcePending(ctx context.Context, tx *sql.Tx, key string) (bool, error) {
	return PersonalTransferSourcesPending(ctx, tx, []string{key})
}

// 来源 gate 已由调用方排序锁定。提取 Doc 后按 transfer_document 索引做一次
// 当前读，既不让 RR 旧快照遗漏原迁移，也不把搜索的一页退化成逐来源 SQL。
func PersonalTransferSourcesPending(ctx context.Context, tx *sql.Tx, keys []string) (bool, error) {
	documents := map[uint64]bool{}
	for _, key := range keys {
		prefix, number, found := strings.Cut(key, ":")
		if !found || (prefix != "doc-acl" && prefix != "publication") {
			continue
		}
		id, err := domain.CanonicalID(number)
		if err != nil {
			return false, err
		}
		documents[id] = true
	}
	if len(documents) == 0 {
		return false, nil
	}
	ids := make([]uint64, 0, len(documents))
	for id := range documents {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	var state string
	err := tx.QueryRowContext(ctx, `SELECT state FROM personal_transfer FORCE INDEX(transfer_document)
 WHERE document_id IN (`+marks+`) AND state='ACCEPTING' ORDER BY document_id,id LIMIT 1 FOR UPDATE`, args...).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// personal_transfer_terminal.go：保留原实现，按业务合并文件。

func SavePersonalTransferMeasurement(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET fixed_plan=? WHERE id=? AND accept_operation_id=?`, string(raw), p.Transfer, p.Operation)
	return err
}

func RejectPersonalTransfer(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan, reason string) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='ACTIVE',control_revision=control_revision+1 WHERE id=? AND lifecycle='CLOSING' AND space_id=? AND responsible_user_id=? AND control_revision=? AND acl_revision=? AND publication_epoch=?`, p.Document, p.SourceSpace, p.SourceUser, p.ClosingControl, p.ACL+1, p.Publication+1)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_COMPENSATION_CHANGED")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE personal_transfer SET state='REJECTED',revision=revision+1 WHERE id=? AND state='ACCEPTING' AND accept_operation_id=?`, p.Transfer, p.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"operation_id": p.Operation, "state": "REJECTED", "error": map[string]string{"code": reason, "message": "接收空间容量不足，原文档归属已恢复", "trace_id": p.Operation, "operation_id": p.Operation, "retry_action": "REFRESH_STATE"}, "result": map[string]string{"entity_id": domain.TextID(p.Document)}})
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='PERSONAL_TRANSFER_ACCEPT' AND state='PREPARING'`, string(raw), p.Operation, p.TargetUser)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("TRANSFER_COMPENSATION_RECEIPT_CHANGED")
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='REJECTED',last_error=?,lease_until=NULL WHERE kind=? AND operation_id=?`, reason, PersonalTransferTask, p.Operation)
	return err
}

func ActiveTransferTargetACL(ctx context.Context, q Queryer, kb, space uint64) (uint64, error) {
	var revision uint64
	err := q.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=? AND space_id=? AND lifecycle='ACTIVE'", kb, space).Scan(&revision)
	return revision, err
}

// 调用方已先排序锁定来源；文档锁继续按编号递增取得，避免共享资源消费者互锁。
func LockTransferConsumerDocuments(ctx context.Context, tx *sql.Tx, documents []uint64) error {
	ids := append([]uint64{}, documents...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var previous uint64
	for i, id := range ids {
		if i > 0 && id == previous {
			continue
		}
		var current uint64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM document WHERE id=? FOR UPDATE", id).Scan(&current); err != nil {
			return err
		}
		previous = id
	}
	return nil
}
