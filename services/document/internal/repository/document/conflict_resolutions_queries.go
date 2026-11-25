// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// conflict_resolution_readiness.go：保留原实现，按业务合并文件。

type PendingResolutionBinding struct {
	Operation, Kind, State, Owner                     string
	Actor                                             uint64
	Original, Payload, Target, Fingerprint, Committed []byte
}

func PendingResolutionBindings(ctx context.Context, q Queryer, operation string) ([]PendingResolutionBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,''),COALESCE(r.actor_id,0),
 COALESCE(original.payload,'{}'),t.payload,COALESCE(r.target_json,'{}'),COALESCE(r.request_fingerprint,''),
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}')
 FROM durable_task t LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='CONFLICT_RESOLUTION'
 LEFT JOIN durable_task original ON original.kind='RESOLUTION_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id AND original.state='SUCCEEDED'
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Scope.Workspace.Workspace'))
 LEFT JOIN durable_task committed ON committed.kind='RESOLUTION_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id AND committed.state='SUCCEEDED'
 WHERE t.kind IN ('RESOLUTION_RECOVER','RESOLUTION_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (?='' OR t.operation_id=?)`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []PendingResolutionBinding{}
	for rows.Next() {
		var item PendingResolutionBinding
		if err = rows.Scan(&item.Operation, &item.Kind, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Target, &item.Fingerprint, &item.Owner, &item.Committed); err != nil {
			return nil, err
		}
		values = append(values, item)
	}
	return values, rows.Err()
}

func ClaimedResolutionBinding(ctx context.Context, q Queryer, operation string) (PendingResolutionBinding, error) {
	facts, err := PendingResolutionBindings(ctx, q, operation)
	if err != nil {
		return PendingResolutionBinding{}, err
	}
	if len(facts) != 1 {
		return PendingResolutionBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

func ResolutionResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var value MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='CONFLICT_RESOLUTION'", operation).Scan(&value.Actor, &value.Kind, &value.State, &value.Fingerprint, &value.Result, &value.Generation, &value.Expires)
	return value, err
}

// 修复是独立支线动作，不写有真实SUBMISSION外键的submission_operation_id。
// 原上下文、恢复任务及支线占位全部由调用方同一TX保存。
func RegisterResolution(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "RESOLUTION_RECOVER", operation, operation, raw, "QUEUED")
}

func ClaimResolutionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "RESOLUTION_RECOVER", "RESOLUTION_RECONCILE")
}

func InsertResolvedLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema)
 VALUES(?,?,?,?,?,'RESTORED',?,?,?,?,?,'provenance-v1')`, id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

func CompleteResolution(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, state, result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

func ResolutionInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='RESOLUTION_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

type ResolvedLineage struct {
	Branch, Document, Unit, Previous, Manifest, Version uint64
	Operation, Capture, Kind                            string
	Provenance                                          []byte
}

func ResolutionLineage(ctx context.Context, q Queryer, lineage uint64) (ResolvedLineage, error) {
	var value ResolvedLineage
	err := q.QueryRowContext(ctx, "SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lineage WHERE id=?", lineage).Scan(&value.Branch, &value.Document, &value.Unit, &value.Previous, &value.Manifest, &value.Version, &value.Operation, &value.Capture, &value.Kind, &value.Provenance)
	return value, err
}

func CompleteResolutionInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error {
	if succeeded {
		result, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_WORKSPACE_UNKNOWN")
		}
	}
	if err := SetBranchControl(ctx, tx, branch, ""); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
