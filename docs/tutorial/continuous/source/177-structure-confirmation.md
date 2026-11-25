# services/document/internal/repository/document/structure_confirmation.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/177-structure-confirmation.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`StructureConfirmationFact`、`LockedStructureConfirmation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"context"
	"database/sql"
	"time"
)

type StructureConfirmationFact struct {
	Binding        PendingStructurePreviewBinding
	Snapshot       []byte
	ReceiptExpires time.Time
}

// 确认读取实际原回执、原上下文与不可变快照；没有任何一步换池或用调用者伪造的PREPARING状态。
func LockedStructureConfirmation(ctx context.Context, tx *sql.Tx, operation string) (StructureConfirmationFact, error) {
	var fact StructureConfirmationFact
	fact.Binding.Operation = operation
	err := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,
 original.payload,fixed.payload FROM operation_receipt o
 LEFT JOIN durable_task original ON original.kind='STRUCTURE_PREVIEW_CONTEXT' AND original.business_key=o.operation_id AND original.operation_id=o.operation_id
 LEFT JOIN durable_task fixed ON fixed.kind='STRUCTURE_PREVIEW_FIXED' AND fixed.business_key=o.operation_id AND fixed.operation_id=o.operation_id
 WHERE o.operation_id=? AND o.kind='STRUCTURE_PREVIEW' FOR UPDATE`, operation).Scan(
		&fact.Binding.Actor, &fact.Binding.State, &fact.Binding.Fingerprint, &fact.Binding.Target,
		&fact.ReceiptExpires, &fact.Binding.Original, &fact.Snapshot,
	)
	return fact, err
}
````

UTF-8/LF全文SHA-256：`e48da6d24071956f0242c81f16d813c259a4f11161444a2207deada9658215c6`。[参考实现](../../../../services/document/internal/repository/document/structure_confirmation.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
