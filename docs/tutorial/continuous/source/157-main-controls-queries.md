# services/document/internal/repository/document/main_controls_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/157-main-controls-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PendingMainControlBinding`、`PendingMainControlBindings`、`MainControlReceipt`、`MainControlResult`、`RegisterMainControl`、`ClaimMainControlTask`、`AdvanceMainControlAttempt`、`InsertControlledManifest`、`InsertControlledStyle`、`ConfigureMainOperationAdmission`、`CompleteMainControl`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// main_control_readiness.go：保留原实现，按业务合并文件。

type PendingMainControlBinding struct {
	Kind, Operation, ReceiptKind, State            string
	Payload, Original, Fingerprint, Target, Result []byte
	Actor                                          uint64
}

func PendingMainControlBindings(ctx context.Context, db *sql.DB) ([]PendingMainControlBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.kind,t.operation_id,t.payload,original.payload,
		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json
		FROM durable_task t
		LEFT JOIN durable_task original ON original.kind='MAIN_CONTROL_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		WHERE t.kind IN ('MAIN_CONTROL_RECOVER','MAIN_CONTROL_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingMainControlBinding
	for rows.Next() {
		var fact PendingMainControlBinding
		if err = rows.Scan(&fact.Kind, &fact.Operation, &fact.Payload, &fact.Original, &fact.Actor, &fact.ReceiptKind, &fact.State, &fact.Fingerprint, &fact.Target, &fact.Result); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

// main_controls.go：保留原实现，按业务合并文件。

type MainControlReceipt struct {
	Actor               uint64
	Kind, State         string
	Fingerprint, Result []byte
	Generation          uint64
	Expires             time.Time
}

func MainControlResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE')", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

func RegisterMainControl(ctx context.Context, tx *sql.Tx, document, generation uint64, operation string, original, recovery []byte) error {
	// 原绑定不可变，领取进度可变化；两者和Doc屏障均属于调用方的同一预约事务。
	if err := InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_CONTEXT", operation, operation, original, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECOVER", operation, operation, recovery, "QUEUED"); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='PREPARING',operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WHERE document_id=?", operation, generation, document)
	return err
}

func ClaimMainControlTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_RECONCILE")
}

func AdvanceMainControlAttempt(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, payload []byte) error {
	// 只有明确CHANGED才能替换领取进度；原MAIN_CONTROL_CONTEXT与已固定输入始终保留。
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", string(payload), task.ID, task.Generation, task.Instance)
	return err
}

func InsertControlledManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, kind, cut string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)", document, parent, structure, style, operation, kind, cut)
}

func InsertControlledStyle(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string, style []byte) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,'style-v1',?)", document, parent, string(style), operation)
}

func ConfigureMainOperationAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte, expires int64) error {
	// 全篇控制没有虚构的unit=0实体：SQL外键列保持NULL，完整protobuf目标仍明确unit/branch为0。
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?", operation, string(target), expires, admission)
	return err
}

func CompleteMainControl(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation, manifest, style uint64, locked bool, result map[string]any, install []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?", locked, style, manifest, document); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='INSTALLING',target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", manifest, document, task.Operation, generation); err != nil {
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
	if err = InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
````

UTF-8/LF全文SHA-256：`d068c2e827648e107b05e8c27d93fd9580dbd1973d96ccfe522e2952eb161506`。[参考实现](../../../../services/document/internal/repository/document/main_controls_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
