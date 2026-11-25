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
