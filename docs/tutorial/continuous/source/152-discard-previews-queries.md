# services/document/internal/repository/document/discard_previews_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/152-discard-previews-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PendingDiscardPreviewBinding`、`PendingDiscardPreviewBindings`、`DiscardPreviewResult`、`RegisterDiscardPreview`、`ClaimDiscardPreviewTask`、`ConfigureDiscardPreviewAdmission`、`DiscardPreviewBlockingChanges`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`dba4abf5b33096f005fb27abe5c24a72be2560a50d25d90c988432cffc85563f`。[参考实现](../../../../services/document/internal/repository/document/discard_previews_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
