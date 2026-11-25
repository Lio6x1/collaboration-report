# services/document/internal/repository/document/structure_previews_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/179-structure-previews-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PendingStructurePreviewBinding`、`PendingStructurePreviewBindings`、`StructurePreviewUnit`、`CurrentStructurePreviewUnits`、`StructurePreviewResult`、`RegisterStructurePreview`、`ClaimStructurePreviewTask`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// structure_preview_readiness.go：保留原实现，按业务合并文件。

type PendingStructurePreviewBinding struct {
	Operation, State                       string
	Actor                                  uint64
	Original, Payload, Fingerprint, Target []byte
}

func PendingStructurePreviewBindings(ctx context.Context, db *sql.DB) ([]PendingStructurePreviewBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.operation_id,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.request_fingerprint,o.target_json FROM durable_task t
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='STRUCTURE_PREVIEW'
 LEFT JOIN durable_task original ON original.kind='STRUCTURE_PREVIEW_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 WHERE t.kind='STRUCTURE_PREVIEW_RECOVER' AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingStructurePreviewBinding{}
	for rows.Next() {
		var item PendingStructurePreviewBinding
		if err = rows.Scan(&item.Operation, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Fingerprint, &item.Target); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// structure_previews.go：保留原实现，按业务合并文件。

type StructurePreviewUnit struct{ Unit, Chapter, Branch uint64 }

func CurrentStructurePreviewUnits(ctx context.Context, tx *sql.Tx, document, structure uint64) ([]StructurePreviewUnit, error) {
	rows, err := tx.QueryContext(ctx, `SELECT u.id,u.chapter_id,COALESCE(u.active_branch_id,0) FROM collaboration_unit u
 JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=?
 WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id`, structure, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StructurePreviewUnit{}
	for rows.Next() {
		var unit StructurePreviewUnit
		if err = rows.Scan(&unit.Unit, &unit.Chapter, &unit.Branch); err != nil {
			return nil, err
		}
		result = append(result, unit)
	}
	return result, rows.Err()
}

func StructurePreviewResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var result MainControlReceipt
	err := q.QueryRowContext(ctx, `SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at
 FROM operation_receipt WHERE operation_id=? AND kind='STRUCTURE_PREVIEW'`, operation).Scan(&result.Actor, &result.Kind, &result.State, &result.Fingerprint, &result.Result, &result.Generation, &result.Expires)
	return result, err
}

func RegisterStructurePreview(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	// 原上下文与后台任务依赖已登记receipt，使用调用方同一事务，不把未知RPC调用塞进SQL锁内。
	if err := InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_RECOVER", operation, operation, raw, "QUEUED")
}

func ClaimStructurePreviewTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "STRUCTURE_PREVIEW_RECOVER", "STRUCTURE_PREVIEW_RECOVER")
}
````

UTF-8/LF全文SHA-256：`c6dfd438da889781bde618284bcd988c956bc817964c9efb575842b422faca60`。[参考实现](../../../../services/document/internal/repository/document/structure_previews_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
