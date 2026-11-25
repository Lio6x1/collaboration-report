// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// discard_draft_readiness.go：保留原实现，按业务合并文件。

type PendingDiscardDraftBinding struct {
	Operation, Kind, State, Owner                     string
	Actor                                             uint64
	Original, Payload, Target, Fingerprint, Committed []byte
}

func PendingDiscardDraftBindings(ctx context.Context, q Queryer) ([]PendingDiscardDraftBinding, error) {
	return pendingDiscardDraftBindings(ctx, q, "")
}

func ClaimedDiscardDraftBinding(ctx context.Context, q Queryer, operation string) (PendingDiscardDraftBinding, error) {
	facts, err := pendingDiscardDraftBindings(ctx, q, operation)
	if err != nil {
		return PendingDiscardDraftBinding{}, err
	}
	if len(facts) != 1 {
		return PendingDiscardDraftBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

func pendingDiscardDraftBindings(ctx context.Context, q Queryer, operation string) ([]PendingDiscardDraftBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.target_json,o.request_fingerprint,
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}')
 FROM durable_task t
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='DISCARD_DRAFT'
 LEFT JOIN durable_task original ON original.kind='DISCARD_DRAFT_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Intent.Branch'))
 LEFT JOIN durable_task committed ON committed.kind='DISCARD_DRAFT_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id
 WHERE t.kind IN ('DISCARD_DRAFT_RECOVER','DISCARD_DRAFT_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED')
 AND (?='' OR t.operation_id=?) ORDER BY t.id`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingDiscardDraftBinding{}
	for rows.Next() {
		var item PendingDiscardDraftBinding
		if err = rows.Scan(&item.Operation, &item.Kind, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Target, &item.Fingerprint, &item.Owner, &item.Committed); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// discard_drafts.go：保留原实现，按业务合并文件。

func DiscardDraftResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='DISCARD_DRAFT'", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

func RegisterDiscardDraft(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_RECOVER", operation, operation, raw, "QUEUED")
}

func ClaimDiscardDraftTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_RECONCILE")
}

func InsertDiscardLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'DISCARDED',?,?,?,?,?,'provenance-v1')", id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

func CompleteDiscardDraft(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, state, result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

func DiscardDraftInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='DISCARD_DRAFT_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

func ActivateDiscardedBranch(ctx context.Context, tx *sql.Tx, branch, epoch uint64) error {
	result, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_WORKSPACE_UNKNOWN")
	}
	return nil
}

// 安装ACK之后才写这些动作。item_no=0留给ARCHIVE通知，逐项处理使用明确且稳定的序号。
func ArchiveDiscardedReview(ctx context.Context, tx *sql.Tx, operation string, actor uint64, pointer domain.ReviewPointer, changes []domain.DiscardChange) error {
	if pointer.ID == 0 {
		return nil
	}
	version, err := ReviewRoundVersion(ctx, tx, pointer.ID, pointer.Round)
	if err != nil {
		return err
	}
	for index, change := range changes {
		fact, err := LockedChange(ctx, tx, pointer.ID, change.ID)
		if err != nil {
			return err
		}
		if fact.Status != "OPEN" || fact.Revision != change.Revision {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,'RESOLVE_CHANGES',?,?,?,?,?)", pointer.ID, pointer.Round, actor, operation, index+1, change.Reason, version, change.ID)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if err = ResolveChange(ctx, tx, change.ID, uint64(id)); err != nil {
			return err
		}
	}
	reason := "管理员明确丢弃共同草稿，可靠恢复已安装，撤回工作结束"
	action, err := InsertReviewAction(ctx, tx, operation, "ARCHIVE", pointer.ID, pointer.Round, actor, version, 0, &reason)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE review_request SET archived_at=UTC_TIMESTAMP(6),revision=revision+1 WHERE id=? AND status='WITHDRAWN' AND archived_at IS NULL", pointer.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND resumable_request_id=?", pointer.Unit, pointer.ID); err != nil {
		return err
	}
	target, _ := json.Marshal(map[string]string{"document_id": domain.TextID(pointer.Document), "unit_id": domain.TextID(pointer.Unit), "request_id": domain.TextID(pointer.ID), "round_id": domain.TextID(pointer.Round)})
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) VALUES(?,?,2,'ARCHIVE',?,'{\"archived\":false}','{\"archived\":true}',?)", actor, operation, string(target), reason); err != nil {
		return err
	}
	return QueueReviewActionNotification(ctx, tx, operation, "ARCHIVE", pointer, action, actor)
}
