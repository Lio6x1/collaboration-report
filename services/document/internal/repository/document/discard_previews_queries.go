// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// discard_preview_readiness.go：保留原实现，按业务合并文件。

type PendingDiscardPreviewBinding struct {
	Operation, State                       string
	Actor                                  uint64
	Original, Payload, Fingerprint, Target []byte
}

func PendingDiscardPreviewBindings(ctx context.Context, db *sql.DB) ([]PendingDiscardPreviewBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.operation_id,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.request_fingerprint,o.target_json FROM durable_task t
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='DISCARD_PREVIEW'
 LEFT JOIN durable_task original ON original.kind='DISCARD_PREVIEW_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 WHERE t.kind='DISCARD_PREVIEW_RECOVER' AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingDiscardPreviewBinding{}
	for rows.Next() {
		var item PendingDiscardPreviewBinding
		if err = rows.Scan(&item.Operation, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Fingerprint, &item.Target); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// discard_previews.go：保留原实现，按业务合并文件。

func DiscardPreviewResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var result MainControlReceipt
	err := q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at
 FROM operation_receipt WHERE operation_id=? AND kind='DISCARD_PREVIEW'`, operation).Scan(&result.Actor, &result.Kind, &result.State, &result.Fingerprint, &result.Result, &result.Generation, &result.Expires)
	return result, err
}

func RegisterDiscardPreview(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_RECOVER", operation, operation, raw, "QUEUED")
}

func ClaimDiscardPreviewTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "DISCARD_PREVIEW_RECOVER", "DISCARD_PREVIEW_RECOVER")
}

func ConfigureDiscardPreviewAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, unit uint64, target []byte, expires int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?", unit, operation, string(target), expires, admission)
	return err
}

// 意见的原理由来自真实REQUEST_CHANGES审计动作，不用当前页面文字或虚构字段替代。
func DiscardPreviewBlockingChanges(ctx context.Context, tx *sql.Tx, unit uint64) ([]map[string]string, error) {
	guard, err := LockedReviewGuard(ctx, tx, unit)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.revision,c.author_id,a.reason FROM change_request c
 JOIN review_action a ON a.id=c.created_action_id AND a.request_id=c.request_id AND a.kind='REQUEST_CHANGES'
 WHERE c.request_id IN (?,?) AND c.status='OPEN' ORDER BY c.id FOR UPDATE`, guard.Ongoing, guard.Resumable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var id, revision, author uint64
		var reason string
		if err = rows.Scan(&id, &revision, &author, &reason); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{"id": domain.TextID(id), "revision": domain.TextID(revision), "author_id": domain.TextID(author), "reason": reason})
	}
	return result, rows.Err()
}
