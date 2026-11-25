# services/document/internal/repository/document/approval_actions.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/145-approval-actions.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`MergeApprovalFacts`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
)

// 同一输出事务读取全部权威事实。不能为GET调用会插入guard的LockedReviewGuard，
// 也不能使用分页轮次或分页意见推断“从未提交”和“没有其他人的要求”。
func MergeApprovalFacts(ctx context.Context, q Queryer, request, actor uint64) (domain.MergeApprovalFacts, error) {
	var value domain.MergeApprovalFacts
	err := q.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM request_submitter s WHERE s.request_id=r.id AND s.user_id=?),
		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN' AND c.author_id=?),
		(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN' AND c.author_id<>?),
		EXISTS(SELECT 1 FROM unit_review_guard g
			JOIN collaboration_unit u ON u.id=g.unit_id
			JOIN branch b ON b.id=r.branch_id AND b.unit_id=u.id AND b.document_id=r.document_id
			WHERE g.unit_id=r.unit_id AND g.ongoing_request_id=r.id
				AND g.submission_operation_id IS NULL AND r.active_merge_operation_id IS NULL
				AND u.retired_structure_id IS NULL AND u.active_branch_id=b.id AND b.lifecycle='ACTIVE')
		FROM review_request r WHERE r.id=?`, actor, actor, actor, request).
		Scan(&value.EverSubmitted, &value.OwnOpen, &value.ForeignOpen, &value.PositionReady)
	return value, err
}
````

UTF-8/LF全文SHA-256：`0905bc0c9e37dba925dabe501fe3f2bfaf09d2be97a84913c1fe2c23aca1e14b`。[参考实现](../../../../services/document/internal/repository/document/approval_actions.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
