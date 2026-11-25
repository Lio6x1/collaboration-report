# services/document/internal/repository/document/reverse_changes_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/173-reverse-changes-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ReverseMergeRecord`、`ReverseActiveBranch`、`ReverseMerge`、`ReverseResult`、`RegisterReverse`、`ClaimReverseTask`、`CompleteReverseTask`、`InsertReverseLineage`、`ReverseInstalled`、`InsertReverseAudit`、`RejectReverseSource`、`PendingReverseBinding`、`PendingReverseBindings`、`ClaimedReverseBinding`、`reverseBindings`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// reverse_changes.go：保留原实现，按业务合并文件。

// 归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。
type ReverseMergeRecord struct {
	ID, Document, Unit, Branch, Request, Round, Attempt        uint64
	InputManifest, OutputManifest, Version, Lineage, Structure uint64
	Operation, Capture, State                                  string
	Digest                                                     []byte
}

func ReverseActiveBranch(ctx context.Context, q Queryer, unit, document uint64) (uint64, error) {
	var branch uint64
	err := q.QueryRowContext(ctx, "SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL", unit, document).Scan(&branch)
	return branch, err
}

func ReverseMerge(ctx context.Context, q Queryer, document, record uint64) (ReverseMergeRecord, error) {
	var value ReverseMergeRecord
	err := q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,
 m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineage_id,a.structure_id,m.operation_id,m.accepted_capture_ref,o.state,a.input_digest
 FROM merge_record m JOIN review_request r ON r.id=m.request_id JOIN review_round v ON v.id=m.round_id AND v.request_id=r.id
 JOIN merge_attempt a ON a.id=m.attempt_id AND a.operation_id=m.operation_id JOIN merge_operation o ON o.operation_id=m.operation_id
 WHERE m.id=? AND r.document_id=?`, record, document).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Request, &value.Round, &value.Attempt,
		&value.InputManifest, &value.OutputManifest, &value.Version, &value.Lineage, &value.Structure, &value.Operation, &value.Capture, &value.State, &value.Digest)
	return value, err
}

func ReverseResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='REVERSE'", operation).Scan(&fact.Actor, &fact.State, &fact.Generation, &fact.Fingerprint, &fact.Result, &fact.Expires)
	return fact, err
}

func RegisterReverse(ctx context.Context, tx *sql.Tx, record ReverseMergeRecord, actor uint64, workspace domain.EditWorkspace, generation uint64, operation string, payload []byte) error {
	if err := SetBranchControl(ctx, tx, workspace.Workspace, operation); err != nil {
		return err
	}
	// flush_receipt_ref是既定必填文本，不具有FLUSH外键；服务器来源使用明确命名，不伪造用户保存确认。
	_, err := tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)
 VALUES(?,?,?,?,?,'PREPARING',?,?,?,UTC_TIMESTAMP(6))`, operation, actor, record.Document, record.Unit, workspace.Workspace, generation, "reverse:"+operation, workspace.Lineage)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?", operation, generation, record.Unit); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "REVERSE_CONTEXT", operation, operation, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "REVERSE_RECOVER", operation, operation, payload, "QUEUED")
}

func ClaimReverseTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "REVERSE_RECOVER", "REVERSE_RECONCILE")
}

func CompleteReverseTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, install []byte) error {
	if err := InsertSubmissionTask(ctx, tx, "REVERSE_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "REVERSE_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

func InsertReverseLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema)
 VALUES(?,?,?,?,?,'RESTORED',?,?,?,?,?,'provenance-v1')`, id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

func ReverseInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var n uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='REVERSE_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&n)
	return n == 1, err
}

func InsertReverseAudit(ctx context.Context, tx *sql.Tx, operation string, actor, merge, request, round, version uint64, reason string) error {
	target, err := json.Marshal(map[string]string{"merge_record_id": domain.TextID(merge), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,'REVERSE',?,?)", actor, operation, string(target), reason)
	return err
}

func RejectReverseSource(ctx context.Context, tx *sql.Tx, operation string, unit uint64, code string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?", unit, operation); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state='PREPARING'", code, operation)
	return err
}

// reverse_readiness.go：保留原实现，按业务合并文件。

type PendingReverseBinding struct {
	Operation, Kind, State, Owner, SourceState, HeldOperation                                                            string
	Actor, SourceActor, SourceDocument, SourceUnit, SourceBranch, SourceLineage, SourceGeneration, HeldGeneration, Round uint64
	Original, Payload, Target, Fingerprint, Committed                                                                    []byte
}

func PendingReverseBindings(ctx context.Context, q Queryer) ([]PendingReverseBinding, error) {
	return reverseBindings(ctx, q, "")
}

func ClaimedReverseBinding(ctx context.Context, q Queryer, operation string) (PendingReverseBinding, error) {
	facts, err := reverseBindings(ctx, q, operation)
	if err != nil {
		return PendingReverseBinding{}, err
	}
	if len(facts) != 1 {
		return PendingReverseBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

func reverseBindings(ctx context.Context, q Queryer, operation string) ([]PendingReverseBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,''),COALESCE(r.actor_id,0),COALESCE(original.payload,'{}'),t.payload,
 COALESCE(r.target_json,'{}'),COALESCE(r.request_fingerprint,''),COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}'),
 COALESCE(s.actor_id,0),COALESCE(s.document_id,0),COALESCE(s.unit_id,0),COALESCE(s.branch_id,0),COALESCE(s.lineage_id,0),COALESCE(s.guard_generation,0),COALESCE(s.state,''),COALESCE(s.result_round_id,0),COALESCE(g.submission_operation_id,''),COALESCE(g.generation,0)
 FROM durable_task t LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='REVERSE'
 LEFT JOIN durable_task original ON original.kind='REVERSE_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Scope.Workspace.Workspace'))
 LEFT JOIN durable_task committed ON committed.kind='REVERSE_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id
 LEFT JOIN submission_operation s ON s.operation_id=t.operation_id LEFT JOIN unit_review_guard g ON g.unit_id=s.unit_id
 WHERE t.kind IN ('REVERSE_RECOVER','REVERSE_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (?='' OR t.operation_id=?) ORDER BY t.id`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facts := []PendingReverseBinding{}
	for rows.Next() {
		var fact PendingReverseBinding
		if err = rows.Scan(&fact.Operation, &fact.Kind, &fact.State, &fact.Actor, &fact.Original, &fact.Payload, &fact.Target, &fact.Fingerprint, &fact.Owner, &fact.Committed,
			&fact.SourceActor, &fact.SourceDocument, &fact.SourceUnit, &fact.SourceBranch, &fact.SourceLineage, &fact.SourceGeneration, &fact.SourceState, &fact.Round, &fact.HeldOperation, &fact.HeldGeneration); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
````

UTF-8/LF全文SHA-256：`cece7460349c676b2df173ea7bd388c1220045d57f91e51d8498d8a10020a3a7`。[参考实现](../../../../services/document/internal/repository/document/reverse_changes_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
