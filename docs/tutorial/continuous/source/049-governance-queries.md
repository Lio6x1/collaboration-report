# services/account/internal/repository/account/governance_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/049-governance-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`LockGovernanceGrant`、`GovernanceGrant`、`GovernanceUserActive`、`GovernanceGrantRow`、`GrantGovernance`、`RevokeGovernance`、`CompleteGovernanceRevocation`、`InsertGovernanceOperatorReceipt`、`GovernanceRecoveryReady`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"context"
	"database/sql"
	"errors"
)

// governance_identity.go：保留原实现，按业务合并文件。

func (r *Identity) LockGovernanceGrant(ctx context.Context, tx *sql.Tx, actor uint64) (string, uint64, error) {
	var state string
	var epoch uint64
	err := tx.QueryRowContext(ctx, "SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE", actor).Scan(&state, &epoch)
	return state, epoch, err
}

// governance_operator.go：保留原实现，按业务合并文件。

type GovernanceGrant struct {
	State, Operation string
	Epoch, Revision  uint64
}

func GovernanceUserActive(ctx context.Context, tx *sql.Tx, user uint64) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
	return state == "ACTIVE", err
}

func GovernanceGrantRow(ctx context.Context, q Queryer, user uint64, lock bool) (GovernanceGrant, error) {
	var grant GovernanceGrant
	query := "SELECT state,epoch,revision,changed_operation_id FROM platform_governance_grant WHERE user_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, user).Scan(&grant.State, &grant.Epoch, &grant.Revision, &grant.Operation)
	return grant, err
}

func GrantGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string, previous GovernanceGrant) error {
	if previous.Revision == 0 {
		_, err := tx.ExecContext(ctx, "INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,'ACTIVE',?)", user, operation)
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='ACTIVE',epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?", operation, user)
	return err
}

func RevokeGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='REVOKING',epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?", operation, user)
	return err
}

func CompleteGovernanceRevocation(ctx context.Context, tx *sql.Tx, user, epoch uint64, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='CLOSED',updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state='REVOKING' AND epoch=? AND changed_operation_id=?", user, epoch, operation)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("PLATFORM_GOVERNANCE_BINDING_CHANGED")
	}
	return nil
}

func InsertGovernanceOperatorReceipt(ctx context.Context, tx *sql.Tx, operation string, fingerprint [32]byte, target, result []byte, state string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at)
	 VALUES(?,0,'PLATFORM_GOVERNANCE_OPERATOR',?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))`, operation, string(target), fingerprint[:], state, string(result), state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,'PLATFORM_GOVERNANCE_OPERATOR',?,?)", operation, string(target), string(result))
	return err
}

// governance_readiness.go：保留原实现，按业务合并文件。

// 管理员撤权可在重启后继续，但必须保留角色、固定payload和来源关闭的一致绑定。
func GovernanceRecoveryReady(ctx context.Context, db *sql.DB) error {
	var invalid uint64
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task t
	 LEFT JOIN platform_governance_grant p ON CAST(p.user_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.user_id'))
	 LEFT JOIN access_gate g ON g.scope_key=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.scope_key'))
	 LEFT JOIN authorization_change c ON c.revocation_id=t.operation_id
	 WHERE t.kind='PLATFORM_GOVERNANCE_CHANGE' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
	 p.user_id IS NULL OR p.state<>'REVOKING' OR p.changed_operation_id<>t.operation_id OR
	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.kind')),'')<>'PLATFORM_GOVERNANCE_REVOKE' OR
	 JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.scope_key'))<>CONCAT('platform-admin:',CAST(p.user_id AS CHAR)) OR
	 p.epoch<=1 OR CAST(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.closed_epoch')),'0') AS UNSIGNED)<>p.epoch-1 OR
	 g.scope_key IS NULL OR g.state<>'BLOCKED' OR g.max_epoch<>p.epoch-1 OR g.last_revocation_id<>t.operation_id OR
	 c.revocation_id IS NULL OR c.scope_key<>g.scope_key OR c.epoch<>g.max_epoch OR COALESCE(c.actor_id,1)<>0 OR
	 t.business_key<>t.operation_id)`).Scan(&invalid)
	if err != nil {
		return err
	}
	if invalid != 0 {
		return errors.New("PLATFORM_GOVERNANCE_RECOVERY_INVALID")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`666f3ddefd02aba3010b0c99180346b16ea3ccdd7b7103da3a34f11d994c7b74`。[参考实现](../../../../services/account/internal/repository/account/governance_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
