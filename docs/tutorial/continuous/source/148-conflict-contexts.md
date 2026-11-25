# services/document/internal/repository/document/conflict_contexts.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/148-conflict-contexts.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ConflictAttempt`、`ConflictAttemptRow`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"context"
)

type ConflictAttempt struct {
	ID                uint64
	State             string
	Digest, Conflicts []byte
}

// 原操作的最终冲突attempt决定展示输入；不能按现在的轮次或清单重新挑一次计算。
func ConflictAttemptRow(ctx context.Context, q Queryer, operation string) (ConflictAttempt, error) {
	var value ConflictAttempt
	err := q.QueryRowContext(ctx, "SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1", operation).Scan(&value.ID, &value.State, &value.Digest, &value.Conflicts)
	return value, err
}
````

UTF-8/LF全文SHA-256：`1d5d71ec92667b67cfcd6033b5cf1a3908ce507a8a72df85de91b94cfb33e61d`。[参考实现](../../../../services/document/internal/repository/document/conflict_contexts.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
