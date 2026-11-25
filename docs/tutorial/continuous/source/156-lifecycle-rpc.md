# services/document/internal/repository/document/lifecycle_rpc.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/156-lifecycle-rpc.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ReadLifecycleRPCBinding`、`InsertLifecycleRPCBinding`、`UpdateLifecycleRPCBinding`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`2fb0d5edcf2fa4178042f16312a9b196304f5329067ba457d07ca79b2ea6c77b`。[参考实现](../../../../services/document/internal/repository/document/lifecycle_rpc.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
