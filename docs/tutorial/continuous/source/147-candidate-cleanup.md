# services/document/internal/repository/document/candidate_cleanup.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/147-candidate-cleanup.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`CandidateCleanupPermit`、`SaveCandidateCleanupPermit`、`SubmissionCleanupCopy`、`MergeCleanupCopy`、`CandidateCleanupTerminal`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"context"
	"database/sql"
)

func CandidateCleanupPermit(ctx context.Context, q Queryer, reference string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind='CANDIDATE_CLEANUP_PERMIT' AND business_key=? AND state='SUCCEEDED'`, reference).Scan(&raw)
	return raw, err
}

func SaveCandidateCleanupPermit(ctx context.Context, tx *sql.Tx, reference, operation string, raw []byte) error {
	return InsertSubmissionTask(ctx, tx, "CANDIDATE_CLEANUP_PERMIT", reference, operation, raw, "SUCCEEDED")
}

// 只接受原业务完成、系统收尾完成以及完整canonical副本同时成立的候选。
// 受理/登记任务的SUCCEEDED不能替代正文已复制或MAIN已经安装的证明。
func SubmissionCleanupCopy(ctx context.Context, tx *sql.Tx, operation string) (uint64, string, uint64, []byte, []byte, bool, error) {
	var generation, version uint64
	var state string
	var digest, actual []byte
	var finalized bool
	err := tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)),
	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='CAPTURE_FINALIZE' AND t.business_key=s.operation_id AND t.operation_id=s.operation_id AND t.state='SUCCEEDED')
	 FROM submission_operation s LEFT JOIN content_version v ON v.source_operation_id=s.operation_id AND v.source_kind='SUBMISSION' AND v.source_ref=s.operation_id
	 WHERE s.operation_id=?`, operation).Scan(&generation, &state, &version, &digest, &actual, &finalized)
	return generation, state, version, digest, actual, finalized, err
}

func MergeCleanupCopy(ctx context.Context, tx *sql.Tx, operation string, attempt uint64) (string, uint64, []byte, []byte, bool, error) {
	var state string
	var version uint64
	var digest, actual []byte
	var installed bool
	err := tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)),
	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='MAIN_INSTALL_COMPLETED' AND t.business_key=a.operation_id AND t.operation_id=a.operation_id AND t.state='SUCCEEDED')
	 FROM merge_attempt a LEFT JOIN merge_record m ON m.attempt_id=a.id AND m.operation_id=a.operation_id
	 LEFT JOIN content_version v ON v.id=m.result_version_id AND v.source_operation_id=a.operation_id
	 WHERE a.id=? AND a.operation_id=?`, attempt, operation).Scan(&state, &version, &digest, &actual, &installed)
	return state, version, digest, actual, installed, err
}

func CandidateCleanupTerminal(ctx context.Context, tx *sql.Tx, operation string) (bool, error) {
	var complete bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state='SUCCEEDED' AND terminal_at IS NOT NULL)
	 AND NOT EXISTS(SELECT 1 FROM durable_task WHERE operation_id=? AND kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','MERGE_RECOVER','MAIN_RECONCILE') AND state NOT IN ('SUCCEEDED','REJECTED'))`, operation, operation).Scan(&complete)
	return complete, err
}
````

UTF-8/LF全文SHA-256：`d5b7476e7519cfc37b63f37663da2000b66324467c76a2a36c7e88dea729fee6`。[参考实现](../../../../services/document/internal/repository/document/candidate_cleanup.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
