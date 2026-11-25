package document

import (
	"context"
	"database/sql"
)

// 专用登记不会被原恢复worker领取。SUCCEEDED仅表示协议请求绑定已持久登记，
// 业务阶段必须从lifecycle_operation读取，绝不能凭本task签出清理成功。
const LifecycleRPCBindingKind = "ACCOUNT_LIFECYCLE_RPC"

func ReadLifecycleRPCBinding(ctx context.Context, q Queryer, operation string, locked bool) ([]byte, string, string, error) {
	query := `SELECT payload,operation_id,state FROM durable_task WHERE kind='ACCOUNT_LIFECYCLE_RPC' AND business_key=?`
	if locked {
		query += " FOR UPDATE"
	}
	var raw []byte
	var original, state string
	err := q.QueryRowContext(ctx, query, operation).Scan(&raw, &original, &state)
	return raw, original, state, err
}

func InsertLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES('ACCOUNT_LIFECYCLE_RPC',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))
	 ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)`, operation, operation, string(raw))
	return err
}

func UpdateLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	_, err := tx.ExecContext(ctx, `UPDATE durable_task SET payload=?
	 WHERE kind='ACCOUNT_LIFECYCLE_RPC' AND business_key=? AND operation_id=? AND state='SUCCEEDED'`, string(raw), operation, operation)
	return err
}
