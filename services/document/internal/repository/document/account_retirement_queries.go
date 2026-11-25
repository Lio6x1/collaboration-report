// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// account_retirement.go：保留原实现，按业务合并文件。

type AccountRetirementRecord struct {
	Actor, Epoch uint64
	Kind, State  string
	Plan         []byte
}

func AccountRetirementSnapshot(ctx context.Context, q Queryer, operation string) (AccountRetirementRecord, error) {
	var record AccountRetirementRecord
	err := q.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan
		FROM lifecycle_operation WHERE operation_id=?`, operation).
		Scan(&record.Actor, &record.Kind, &record.State, &record.Epoch, &record.Plan)
	return record, err
}

func LockedAccountRetirement(ctx context.Context, tx *sql.Tx, operation string) (AccountRetirementRecord, error) {
	var record AccountRetirementRecord
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan
		FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&record.Actor, &record.Kind, &record.State, &record.Epoch, &record.Plan)
	return record, err
}

func RecordAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding, plan []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
		VALUES(?,?,'ACCOUNT_CLOSE','FROZEN',?,?)`, binding.Operation, binding.User, binding.IdentityEpoch, string(plan))
	return err
}

// 必须先持有同一identity来源锁。固定计划、管理员预留和新操作封门在同一事务提交，
// 提交回复未知时不能释放预留；由原Account注销操作继续确认或执行显式取消协议。
func FreezeAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error {
	_, err := tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1
		WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, "identity:"+domain.TextID(binding.User))
	return err
}

func AccountRetirementGateMatches(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) (bool, error) {
	var matches bool
	err := tx.QueryRowContext(ctx, `SELECT max_epoch=? AND state='BLOCKED' AND last_revocation_id=?
		FROM access_gate WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, "identity:"+domain.TextID(binding.User)).Scan(&matches)
	return matches, err
}

// account_retirement_cancel.go：保留原实现，按业务合并文件。

func RecordRejectedAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error {
	raw, err := json.Marshal(domain.AccountRetirementPlan{Binding: binding, ReservedKnowledgeBases: []uint64{}})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan,last_error)
		VALUES(?,?,'ACCOUNT_CLOSE','REJECTED',?,?,'ACCOUNT_REJECTED')`, binding.Operation, binding.User, binding.IdentityEpoch, string(raw))
	return err
}

func CancelAccountRetirement(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error {
	for _, kb := range plan.ReservedKnowledgeBases {
		if _, _, err := LockedKnowledgeBase(ctx, tx, domain.TextID(kb)); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE admin_retirement SET state='CANCELLED'
			WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state='RESERVED'`, kb, plan.Binding.User, plan.Binding.Operation)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return errors.New("ACCOUNT_RETIREMENT_RESERVATION_CHANGED")
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE lifecycle_operation SET state='REJECTED',last_error='ACCOUNT_REJECTED'
		WHERE operation_id=? AND state='FROZEN'`, plan.Binding.Operation); err != nil {
		return err
	}
	return ReopenSourceGate(ctx, tx, "identity:"+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)
}

// 调用方已锁定来源门禁；取消也必须先取得该锁，因此这里的首次一致读能观察
// 刚完成的取消且不需要对不存在的操作取得间隙锁。迟到Fence不能重新封住已取消来源。
func AccountRetirementFenceCancelled(ctx context.Context, tx *sql.Tx, operation, scope string, epoch uint64) (bool, error) {
	var actor, expected uint64
	var kind, state string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision
		FROM lifecycle_operation WHERE operation_id=?`, operation).Scan(&actor, &kind, &state, &expected)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if kind != "ACCOUNT_CLOSE" {
		return false, nil
	}
	if expected != epoch || scope != "identity:"+domain.TextID(actor) {
		return false, domain.Violation(domain.StateConflict, "FENCE_ID_CONFLICT")
	}
	return state == "REJECTED", nil
}
