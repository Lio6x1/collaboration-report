// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// submission.go：保留原实现，按业务合并文件。

func SubmissionWorkspace(ctx context.Context, q Queryer, branch, document, unit uint64) (domain.EditWorkspace, error) {
	var current domain.EditWorkspace
	err := q.QueryRowContext(ctx, "SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN collaboration_unit u ON u.active_branch_id=b.id AND u.id=b.unit_id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL", branch, document, unit).Scan(&current.Workspace, &current.Epoch, &current.Lineage, &current.BaseManifest, &current.BaseVersion)
	return current, err
}

func SubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error) {
	return readSubmissionRow(ctx, q, id, false)
}

// 同一来源表也承载服务器固定的新审核；公开普通送审入口须按真实receipt.kind区分。
func UserSubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error) {
	return readSubmissionRow(ctx, q, id, true)
}

func readSubmissionRow(ctx context.Context, q Queryer, id string, userSubmission bool) (domain.SubmissionFact, error) {
	var value domain.SubmissionFact
	statement := "SELECT operation_id,actor_id,document_id,unit_id,branch_id,guard_generation,COALESCE(request_id,0),COALESCE(result_round_id,0),state FROM submission_operation WHERE operation_id=?"
	if userSubmission {
		statement += " AND EXISTS(SELECT 1 FROM operation_receipt r WHERE r.operation_id=submission_operation.operation_id AND r.kind='SUBMISSION')"
	}
	err := q.QueryRowContext(ctx, statement, id).Scan(&value.Operation, &value.Actor, &value.Document, &value.Unit, &value.Branch, &value.GuardGeneration, &value.Request, &value.Round, &value.State)
	return value, err
}

func LockedReviewGuard(ctx context.Context, tx *sql.Tx, unit uint64) (domain.ReviewGuard, error) {
	var value domain.ReviewGuard
	_, err := tx.ExecContext(ctx, "INSERT INTO unit_review_guard(unit_id) VALUES(?) ON DUPLICATE KEY UPDATE unit_id=VALUES(unit_id)", unit)
	if err != nil {
		return value, err
	}
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,''),generation FROM unit_review_guard WHERE unit_id=? FOR UPDATE", unit).Scan(&value.Ongoing, &value.Resumable, &value.Submission, &value.Generation)
	return value, err
}

func LockedReviewPointer(ctx context.Context, tx *sql.Tx, id uint64) (domain.ReviewPointer, error) {
	var value domain.ReviewPointer
	err := tx.QueryRowContext(ctx, "SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE", id).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Round, &value.Revision, &value.Status, &value.Archived)
	return value, err
}

func RegisterSubmission(ctx context.Context, tx *sql.Tx, actor, document, unit uint64, intent domain.SubmissionIntent, workspace domain.EditWorkspace, generation uint64, payload []byte) error {
	// 实际 DDL 保留 receipt→submission→guard 的外键链；都在一个事务内按依赖顺序插入。
	_, err := tx.ExecContext(ctx, "INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at) VALUES(?,?,?,?,?,NULLIF(?,0),NULLIF(?,0),'PREPARING',?,?,?,UTC_TIMESTAMP(6))", intent.Operation, actor, document, unit, intent.Branch, intent.Request, intent.ExpectedRound, generation, intent.Flush, workspace.Lineage)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?", intent.Operation, generation, unit); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "SUBMISSION_RECOVER", intent.Operation, intent.Operation, payload, "QUEUED")
}

func InsertSubmissionTask(ctx context.Context, tx *sql.Tx, kind, key, operation string, payload []byte, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))", kind, key, operation, string(payload), state)
	return err
}

func ConfigureSubmissionAdmission(ctx context.Context, tx *sql.Tx, admission string, unit uint64, operation string, target []byte, expires int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?", unit, operation, string(target), expires, admission)
	return err
}

func SubmissionResult(ctx context.Context, q Queryer, id string) (domain.SubmissionResultFact, error) {
	var value domain.SubmissionResultFact
	err := q.QueryRowContext(ctx, "SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='SUBMISSION'", id).Scan(&value.State, &value.Generation, &value.Fingerprint, &value.Result, &value.Expires)
	return value, err
}

func SubmissionPayload(ctx context.Context, q Queryer, id string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?", id).Scan(&raw)
	return raw, err
}

func InsertSubmissionVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error) {
	if err := LockDocumentCapacity(ctx, tx, content.DocumentId); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,?,?,?,'SUBMISSION',?,?,?)", content.DocumentId, content.UnitId, content.StructureId, string(content.BodyJsonUtf8), content.BodyJsonUtf8, content.BodyHash, content.SchemaVersion, content.CanonicalizationVersion, content.ByteSize, content.OperationId, content.CaptureId, string(content.ProvenanceJsonUtf8), content.ProvenanceSchema)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err == nil {
		err = ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)
	}
	return uint64(id), err
}

func InsertSubmissionResource(ctx context.Context, tx *sql.Tx, resource, document, version uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'VERSION',?,'CONTENT')", resource, document, domain.TextID(version))
	return err
}

func CreateReviewRequest(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,'REVIEWING')", document, unit, branch)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func CreateSubmissionRound(ctx context.Context, tx *sql.Tx, request, actor, version uint64, fixed *pb.FixedContent, workspace domain.EditWorkspace) (uint64, error) {
	var number uint64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?", request).Scan(&number); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submitter_id,submission_operation_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", request, fixed.DocumentId, fixed.UnitId, number, version, workspace.BaseManifest, workspace.BaseVersion, fixed.StructureId, workspace.Lineage, fixed.CaptureId, actor, fixed.OperationId)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func CompleteSubmission(ctx context.Context, tx *sql.Tx, operation string, actor, unit, request, round uint64, result map[string]any) error {
	// 历次作者是不可变事实，服务角色没有 UPDATE 权限。请求已持锁，已存在作者只读保留。
	if _, err := tx.ExecContext(ctx, "INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)", request, actor, round, request, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='REVIEWING',current_round_id=?,revision=revision+1 WHERE id=?", round, request); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?", request, unit); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='SUCCEEDED',request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?", request, round, operation, operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state='PREPARING'", string(raw), operation)
	return err
}

// submission_readiness.go：保留原实现，按业务合并文件。

func PendingSubmissionContexts(ctx context.Context, db *sql.DB) ([]byte, error) {
	// 聚合仅用于验证持久绑定，不依据租约或排队时间推断业务已成功。
	var raw []byte
	err := db.QueryRowContext(ctx, "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('kind',t.kind,'operation',t.operation_id,'payload',t.payload,'actor',s.actor_id,'document',s.document_id,'unit',s.unit_id,'branch',s.branch_id,'guard_generation',s.guard_generation,'state',s.state,'receipt_state',r.state,'held_operation',g.submission_operation_id,'held_generation',g.generation,'round',s.result_round_id)),JSON_ARRAY()) FROM durable_task t LEFT JOIN submission_operation s ON s.operation_id=t.operation_id LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id LEFT JOIN unit_review_guard g ON g.unit_id=s.unit_id WHERE t.kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','REVIEW_NOTIFICATION') AND t.state NOT IN ('SUCCEEDED','REJECTED')").Scan(&raw)
	return raw, err
}

// submission_tasks.go：保留原实现，按业务合并文件。

// 领取提交后才调用 RPC。租约只是重新领取条件，最终写入还必须验证 generation 与实例。
func ClaimSubmissionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	task, err := lockClaimableOperationTask(ctx, tx, "SUBMISSION_RECOVER", "CAPTURE_FINALIZE")
	if err != nil {
		return task, err
	}
	task.Generation++
	task.Instance = instance
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 10 SECOND),attempts=attempts+1 WHERE id=?", task.Generation, instance, task.ID)
	return task, err
}

func ValidateSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error {
	var generation uint64
	var state, instance string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? FOR UPDATE", task.ID).Scan(&generation, &state, &instance)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || instance != task.Instance {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_WORKER_SUPERSEDED")
	}
	return nil
}

func FinishSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", state, task.ID, task.Generation, task.Instance)
	return err
}

func RetrySubmissionTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	// 只记录稳定业务码；驱动错误可能包含私有 DSN，不能进入日志或持久业务输出。
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", code, task.ID, task.Generation, task.Instance)
	return err
}

func RejectSubmission(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, unit uint64, result map[string]any, code string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?", unit, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state IN ('PREPARING','CAPTURED')", code, task.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state='PREPARING'", string(raw), task.Operation); err != nil {
		return err
	}
	return FinishSubmissionTask(ctx, tx, task, "REJECTED")
}
