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
