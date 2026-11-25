# services/document/internal/repository/document/merges_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/160-merges-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`MergeBarrier`、`MergeAttempt`、`LockedMergeBarrier`、`LatestMergeAttempt`、`ReserveMergeAttempt`、`MergeArtifact`、`SupersedeMergeAttempt`、`StoreMergeConflict`、`FixMergeAttempt`、`FixProjectedMergeAttempt`、`CompleteMainInstallation`、`InsertMergeVersion`、`InsertReadingVersion`、`insertFixedVersion`、`InsertMergedManifest`、`InsertMergeRecord`、`InsertMergeApproval`、`InsertMergedResolution`、`InsertAcceptedLineage`、`CompleteMerge`、`MergeRoundInput`、`MergeRoundInputs`、`VersionContent`、`PendingMergeBinding`、`PendingMergeBindings`、`MainInstallationCompleted`、`ClaimMergeTask`、`claimOperationTask`、`lockClaimableOperationTask`、`lockDueTask`、`ValidateMergeTask`、`FinishMergeTask`、`RetryMergeTask`、`RejectMerge`、`MergeRow`、`LockedActiveMerge`、`LockedReviewChanges`、`RegisterMerge`、`MergePayload`、`MergeResult`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// merge_attempts.go：保留原实现，按业务合并文件。

type MergeBarrier struct {
	Operation, Phase string
	Generation       uint64
}
type MergeAttempt struct {
	ID, Number, Manifest, Structure, Style, Lineage uint64
	State                                           string
}

func LockedMergeBarrier(ctx context.Context, tx *sql.Tx, document uint64) (MergeBarrier, error) {
	var value MergeBarrier
	if _, err := tx.ExecContext(ctx, "INSERT INTO main_commit_barrier(document_id,phase) VALUES(?,'OPEN') ON DUPLICATE KEY UPDATE document_id=VALUES(document_id)", document); err != nil {
		return value, err
	}
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(operation_id,''),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE", document).Scan(&value.Operation, &value.Phase, &value.Generation)
	return value, err
}

func LatestMergeAttempt(ctx context.Context, tx *sql.Tx, operation string) (MergeAttempt, error) {
	var value MergeAttempt
	err := tx.QueryRowContext(ctx, "SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR UPDATE", operation).Scan(&value.ID, &value.Number, &value.Manifest, &value.Structure, &value.Style, &value.Lineage, &value.State)
	return value, err
}

func ReserveMergeAttempt(ctx context.Context, tx *sql.Tx, fixed domain.MergeIntent, document, number, manifest, structure, style, lineage, base, generation uint64) (uint64, error) {
	// 先保留真实attempt身份以固定Node采样usage；完整输入摘要稍后仅在同一FIXED阶段登记一次。
	result, err := tx.ExecContext(ctx, "INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_digest,state) VALUES(?,?,?,?,?,? ,?,?,?,?,'FIXED')", fixed.Operation, number, base, fixed.Version, manifest, fixed.Operation+":"+domain.TextID(number), structure, style, lineage, make([]byte, 32))
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE main_commit_barrier SET operation_id=?,generation=?,phase='PREPARING',expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WHERE document_id=?", fixed.Operation, generation, document)
	return uint64(id), err
}

func MergeArtifact(ctx context.Context, q Queryer, kind, key string) ([]byte, error) {
	var payload []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind=? AND business_key=?", kind, key).Scan(&payload)
	return payload, err
}

func SupersedeMergeAttempt(ctx context.Context, tx *sql.Tx, operation string, attempt, document, generation uint64) error {
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='SUPERSEDED' WHERE id=? AND operation_id=? AND state IN ('FIXED','COMPUTED')", attempt, operation); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='OPEN',operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", document, operation, generation)
	return err
}

func StoreMergeConflict(ctx context.Context, tx *sql.Tx, attempt uint64, conflicts []byte) error {
	_, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='CONFLICT',conflict_json=? WHERE id=? AND state='FIXED'", string(conflicts), attempt)
	return err
}

func FixMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, digest, payload []byte, key, operation string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET input_digest=? WHERE id=? AND state='FIXED'", digest, id); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_FIXED", key, operation, payload, "SUCCEEDED")
}

// 只追加旧生产者混结构输入的派生版本；原FIXED记录不改，准确旧摘要与状态隔离其他计算。
func FixProjectedMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, original, digest, payload []byte, key, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET input_digest=? WHERE id=? AND state='FIXED' AND input_digest=?", digest, id, original)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_BINDING_UNKNOWN")
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key, operation, payload, "SUCCEEDED")
}

// merge_commit.go：保留原实现，按业务合并文件。

func CompleteMainInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation uint64, reply *pb.InstallReply) error {
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='OPEN',operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", document, task.Operation, generation); err != nil {
		return err
	}
	if reply.State == pb.InstallReply_READY {
		raw, err := json.Marshal(map[string]string{"manifest_id": domain.TextID(reply.ManifestId)})
		if err != nil {
			return err
		}
		if err = InsertSubmissionTask(ctx, tx, "MAIN_INSTALL_COMPLETED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}

func InsertMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string) (uint64, error) {
	return insertFixedVersion(ctx, tx, content, kind, ref, true)
}

// 阅读副本保留可靠正文及资源引用，但不因 GET 请求消耗用户新增空间。
// 实际字节在独立运营账中登记，后续清理与转让只能统计可计费业务版本。
func InsertReadingVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error) {
	return insertFixedVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId, false)
}

func insertFixedVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string, billable bool) (uint64, error) {
	if billable {
		if err := LockDocumentCapacity(ctx, tx, content.DocumentId); err != nil {
			return 0, err
		}
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", content.DocumentId, content.UnitId, content.StructureId, string(content.BodyJsonUtf8), content.BodyJsonUtf8, content.BodyHash, content.SchemaVersion, content.CanonicalizationVersion, content.ByteSize, content.OperationId, kind, ref, string(content.ProvenanceJsonUtf8), content.ProvenanceSchema)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if billable {
		err = ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)
	} else {
		_, err = tx.ExecContext(ctx, "INSERT INTO reading_body_capacity(version_id,document_id,bytes) VALUES(?,?,?)", id, content.DocumentId, content.ByteSize)
	}
	if err != nil {
		return 0, err
	}
	for _, resource := range content.ResourceIds {
		if err = InsertSubmissionResource(ctx, tx, resource, content.DocumentId, uint64(id)); err != nil {
			return 0, err
		}
	}
	return uint64(id), nil
}

func InsertMergedManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, cutRef string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, 'MERGE',?)", document, parent, structure, style, operation, cutRef)
}

func InsertMergeRecord(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, attempt, input, output, version, lineage uint64, capture string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALUES(?,?,?,?,?,?,?,?,?)", operation, pointer.ID, pointer.Round, attempt, input, output, version, capture, lineage)
}

func InsertMergeApproval(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, submitted, record uint64) (uint64, error) {
	// 外键环按record(approval NULL)→action(record)→record.approval提交；只更新授权允许的指针列。
	action, err := InsertID(ctx, tx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,'APPROVE_AND_MERGE',?,0,?,?)", pointer.ID, pointer.Round, actor, operation, submitted, record)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE merge_record SET approval_action_id=? WHERE id=?", action, record)
	return action, err
}

func InsertMergedResolution(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, change uint64, item int) error {
	reason := "在通过并合并本轮时明确解除本人修改要求。"
	action, err := InsertID(ctx, tx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,'RESOLVE_CHANGES',?,?,?,?)", pointer.ID, pointer.Round, actor, operation, item, reason, change)
	if err != nil {
		return err
	}
	return ResolveChange(ctx, tx, change, action)
}

func InsertAcceptedLineage(ctx context.Context, tx *sql.Tx, id uint64, pointer domain.ReviewPointer, previous, manifest, submitted uint64, operation, capture string, provenance []byte) error {
	if _, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'ACCEPTED_CUT',?,?,?,?,?,'provenance-v1')", id, pointer.Branch, pointer.Unit, pointer.Document, previous, operation, manifest, submitted, capture, string(provenance)); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,revision=revision+1 WHERE id=?", id, pointer.Branch)
	return err
}

func CompleteMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, pointer domain.ReviewPointer, actor, record, manifest, attempt, generation uint64, result map[string]any, install []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document SET current_manifest_id=?,control_revision=control_revision+1 WHERE id=?", manifest, pointer.Document); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='MERGED',active_merge_operation_id=NULL,revision=revision+1 WHERE id=?", pointer.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND ongoing_request_id=?", pointer.Unit, pointer.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_operation SET state='SUCCEEDED',result_merge_id=? WHERE operation_id=?", record, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='COMMITTED' WHERE id=?", attempt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='INSTALLING',target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", manifest, pointer.Document, task.Operation, generation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", raw); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MAIN_COMMIT_FIXED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	notice, err := json.Marshal(map[string]string{"event": "APPROVE_AND_MERGE", "document": domain.TextID(pointer.Document), "request": domain.TextID(pointer.ID), "round": domain.TextID(pointer.Round), "record": domain.TextID(record), "actor": domain.TextID(actor)})
	if err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MERGE_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}

// merge_inputs.go：保留原实现，按业务合并文件。

type MergeRoundInput struct {
	BaseManifest, BaseVersion, SubmittedVersion, Structure, Lineage uint64
	Capture                                                         string
	Submission                                                      string
}

func MergeRoundInputs(ctx context.Context, q Queryer, request, round uint64) (MergeRoundInput, error) {
	var value MergeRoundInput
	err := q.QueryRowContext(ctx, "SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?", round, request).Scan(&value.BaseManifest, &value.BaseVersion, &value.SubmittedVersion, &value.Structure, &value.Lineage, &value.Capture, &value.Submission)
	return value, err
}

func VersionContent(ctx context.Context, tx Queryer, version, document uint64) (*pb.FixedContent, error) {
	content := &pb.FixedContent{HashAlgorithm: "SHA-256", ResourceIds: []uint64{}}
	err := tx.QueryRowContext(ctx, "SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenance_json,provenance_schema FROM content_version WHERE id=? AND document_id=?", version, document).Scan(&content.DocumentId, &content.UnitId, &content.StructureId, &content.BodyJsonUtf8, &content.BodyHash, &content.SchemaVersion, &content.CanonicalizationVersion, &content.ByteSize, &content.OperationId, &content.CaptureId, &content.ProvenanceJsonUtf8, &content.ProvenanceSchema)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=CAST(? AS CHAR) AND purpose='CONTENT' ORDER BY resource_id", document, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var resource uint64
		if err = rows.Scan(&resource); err != nil {
			return nil, err
		}
		content.ResourceIds = append(content.ResourceIds, resource)
	}
	return content, rows.Err()
}

// merge_readiness.go：保留原实现，按业务合并文件。

type PendingMergeBinding struct {
	Kind, Operation, ReceiptState  string
	Payload, Original, Fingerprint []byte
	ReceiptActor, Record, Manifest uint64
	Fact                           domain.MergeFact
}

func PendingMergeBindings(ctx context.Context, db *sql.DB) ([]PendingMergeBinding, error) {
	// 同时读取原恢复上下文和权威receipt；单凭任务自己的payload不能证明它属于这个操作。
	rows, err := db.QueryContext(ctx, `
		SELECT t.kind,t.operation_id,t.payload,COALESCE(o.state,''),
		       COALESCE(m.actor_id,0),COALESCE(m.request_id,0),COALESCE(m.round_id,0),
		       COALESCE(m.content_version_id,0),COALESCE(m.state,''),
		       COALESCE(r.document_id,0),COALESCE(r.unit_id,0),COALESCE(r.branch_id,0),
		       original.payload,o.request_fingerprint,COALESCE(o.actor_id,0),
		       COALESCE(m.result_merge_id,0),COALESCE(record.output_manifest_id,0)
		FROM durable_task t
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='MERGE'
		LEFT JOIN review_request r ON r.id=m.request_id
		LEFT JOIN durable_task original ON original.kind='MERGE_RECOVER'
		     AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
		LEFT JOIN merge_record record ON record.id=m.result_merge_id AND record.request_id=m.request_id
		WHERE t.kind IN ('MERGE_RECOVER','MAIN_RECONCILE','MERGE_NOTIFICATION')
		  AND t.state NOT IN ('SUCCEEDED','REJECTED')
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingMergeBinding
	for rows.Next() {
		var value PendingMergeBinding
		f := &value.Fact
		if err = rows.Scan(&value.Kind, &value.Operation, &value.Payload, &value.ReceiptState, &f.Actor, &f.Request, &f.Round, &f.Version, &f.State, &f.Document, &f.Unit, &f.Branch, &value.Original, &value.Fingerprint, &value.ReceiptActor, &value.Record, &value.Manifest); err != nil {
			return nil, err
		}
		facts = append(facts, value)
	}
	return facts, rows.Err()
}

func MainInstallationCompleted(ctx context.Context, q Queryer, operation string) (bool, error) {
	var ready bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind='MAIN_INSTALL_COMPLETED' AND business_key=? AND operation_id=? AND state='SUCCEEDED')", operation, operation).Scan(&ready)
	return ready, err
}

// merge_tasks.go：保留原实现，按业务合并文件。

// 租约只授予这次执行资格；所有跨服务步骤在领取事务提交后进行，终态仍检查generation/instance。
func ClaimMergeTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "MERGE_RECOVER", "MAIN_RECONCILE")
}

// 两种已装配的任务分别领取，种类是内部常量且使用SQL参数，控制任务不会冒用MERGE预约。
func claimOperationTask(ctx context.Context, tx *sql.Tx, instance, preparation, reconciliation string) (domain.SubmissionTask, error) {
	task, err := lockClaimableOperationTask(ctx, tx, preparation, reconciliation)
	if err != nil {
		return task, err
	}
	task.Generation++
	task.Instance = instance
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=attempts+1 WHERE id=?", task.Generation, instance, task.ID)
	return task, err
}

func lockClaimableOperationTask(ctx context.Context, tx *sql.Tx, preparation, reconciliation string) (domain.SubmissionTask, error) {
	return lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)
}

const operationTaskDue = "kind IN (?,?) AND retry_at<=UTC_TIMESTAMP(6) AND (state IN ('QUEUED','RETRY') OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"
const authorizationTaskDue = "kind='AUTHORIZATION_CHANGE' AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"

// predicate只接受上面的内部常量；业务值始终通过参数传入。
func lockDueTask(ctx context.Context, tx *sql.Tx, predicate string, args ...any) (domain.SubmissionTask, error) {
	// 002的kind/state覆盖索引先排除其他业务和已完成历史；旧库仍可选原状态索引。
	// 此步不加锁：RR范围锁会把OUTPUT_LEASE等无关种类也挡住。
	// 只排小ID，避免把多MiB JSON放进filesort；候选窗口允许跳过其他执行器正在持有的行。
	rows, err := tx.QueryContext(ctx, "SELECT id FROM durable_task WHERE "+predicate+" ORDER BY id LIMIT 32", args...)
	if err != nil {
		return domain.SubmissionTask{}, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return domain.SubmissionTask{}, err
	}
	for _, id := range ids {
		var task domain.SubmissionTask
		lockedArgs := append([]any{id}, args...)
		// 精确主键锁的当前读重新检查种类、状态和到期时间；候选快照本身不授予领取资格。
		err = tx.QueryRowContext(ctx, "SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND ("+predicate+") FOR UPDATE SKIP LOCKED", lockedArgs...).Scan(&task.ID, &task.Kind, &task.Operation, &task.Generation)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return task, err
		}
		// 代次更新、原payload读取与提交共用这份行锁和事务；不能换连接读取正文。
		err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE id=?", id).Scan(&task.Payload)
		return task, err
	}
	return domain.SubmissionTask{}, sql.ErrNoRows
}

func ValidateMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error {
	var generation uint64
	var state, instance string
	if err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? FOR UPDATE", task.ID).Scan(&generation, &state, &instance); err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || instance != task.Instance {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_WORKER_SUPERSEDED")
	}
	return nil
}

func FinishMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", state, task.ID, task.Generation, task.Instance)
	return err
}

func RetryMergeTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", code, task.ID, task.Generation, task.Instance)
	return err
}

func RejectMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, request uint64, code string, result map[string]any) error {
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET active_merge_operation_id=NULL,revision=revision+1 WHERE id=? AND active_merge_operation_id=?", request, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state NOT IN ('SUCCEEDED','REJECTED','CANCELLED')", code, task.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", raw); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "REJECTED")
}

// merges.go：保留原实现，按业务合并文件。

func MergeRow(ctx context.Context, q Queryer, operation string) (domain.MergeFact, error) {
	var fact domain.MergeFact
	err := q.QueryRowContext(ctx, "SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation m JOIN review_request r ON r.id=m.request_id WHERE m.operation_id=?", operation).Scan(&fact.Operation, &fact.Actor, &fact.Request, &fact.Round, &fact.Version, &fact.State, &fact.Generation, &fact.Document, &fact.Unit, &fact.Branch)
	return fact, err
}

func LockedActiveMerge(ctx context.Context, tx *sql.Tx, request uint64) (string, error) {
	var operation string
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(active_merge_operation_id,'') FROM review_request WHERE id=? FOR UPDATE", request).Scan(&operation)
	return operation, err
}

func LockedReviewChanges(ctx context.Context, tx *sql.Tx, request uint64) ([]domain.ChangeFact, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE request_id=? ORDER BY id FOR UPDATE", request)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var changes []domain.ChangeFact
	for rows.Next() {
		var change domain.ChangeFact
		if err := rows.Scan(&change.ID, &change.Request, &change.Round, &change.Author, &change.Revision, &change.Status); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func RegisterMerge(ctx context.Context, tx *sql.Tx, actor, request uint64, intent domain.MergeIntent, payload []byte) error {
	resolutions, err := json.Marshal(intent.Resolutions)
	if err != nil {
		return err
	}
	// receipt先由通用事务登记；请求仅保存进行中的操作指针，批准动作必须留到正式合并事务。
	if _, err = tx.ExecContext(ctx, "INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,'PREPARING',UTC_TIMESTAMP(6))", intent.Operation, actor, request, intent.Round, intent.Version, string(resolutions)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE review_request SET active_merge_operation_id=?,revision=revision+1 WHERE id=?", intent.Operation, request); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_RECOVER", intent.Operation, intent.Operation, payload, "QUEUED")
}

func MergePayload(ctx context.Context, q Queryer, operation string) ([]byte, error) {
	var payload []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='MERGE_RECOVER' AND business_key=?", operation).Scan(&payload)
	return payload, err
}

func MergeResult(ctx context.Context, q Queryer, operation string) (domain.SubmissionResultFact, error) {
	var fact domain.SubmissionResultFact
	err := q.QueryRowContext(ctx, "SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='MERGE'", operation).Scan(&fact.State, &fact.Generation, &fact.Fingerprint, &fact.Result, &fact.Expires)
	return fact, err
}
````

UTF-8/LF全文SHA-256：`e05a9488fe0a8acbf1ca07377b70c79cd52d968881b1c3abaa6fc92c6c12046d`。[参考实现](../../../../services/document/internal/repository/document/merges_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
