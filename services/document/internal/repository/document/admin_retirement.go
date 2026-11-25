package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

type AdministratorRetirementCandidate struct {
	KnowledgeBase, Organization, PersonalOwner uint64
	Lifecycle, MemberState                     string
	ReservationOperation, ReservationState     string
	Available                                  int
}

// LockAdministratorRetirementCandidates 使用调用方原事务，并先按知识库编号持锁。
// 其他管理员的预留同样受知识库锁串行化，不能只锁当前成员就分别误判最后一名管理员。
func LockAdministratorRetirementCandidates(ctx context.Context, tx *sql.Tx, user uint64) ([]AdministratorRetirementCandidate, error) {
	if user == 0 {
		return nil, errors.New("ADMIN_RETIREMENT_USER_INVALID")
	}
	// 调用方已锁定目标身份来源，任命路径也使用该来源锁；发现编号本身不取成员锁，
	// 避免两个退出者先分别持有自己的成员行，再竞争同一知识库时形成反向死锁。
	rows, err := tx.QueryContext(ctx, `SELECT k.id
		FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id
		WHERE m.user_id=? AND m.role='ADMIN' AND m.state IN ('ACTIVE','REVOKING')
		AND k.lifecycle IN ('ACTIVE','CLOSING') ORDER BY k.id`, user)
	if err != nil {
		return nil, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	candidates := []AdministratorRetirementCandidate{}
	for _, id := range ids {
		candidate := AdministratorRetirementCandidate{KnowledgeBase: id}
		var space uint64
		// 显式先锁知识库，不依赖JOIN优化器决定加锁顺序。
		if err = tx.QueryRowContext(ctx, `SELECT space_id,lifecycle FROM knowledge_base WHERE id=? FOR UPDATE`, id).Scan(&space, &candidate.Lifecycle); err != nil {
			return nil, err
		}
		if candidate.Lifecycle != "ACTIVE" && candidate.Lifecycle != "CLOSING" {
			continue
		}
		var role string
		err = tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user).Scan(&role, &candidate.MemberState)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if role != "ADMIN" || (candidate.MemberState != "ACTIVE" && candidate.MemberState != "REVOKING") {
			continue
		}
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(organization_id,0),COALESCE(account_owner_id,0) FROM space WHERE id=? FOR SHARE`, space).Scan(&candidate.Organization, &candidate.PersonalOwner); err != nil {
			return nil, err
		}
		if err = lockAdministratorRetirementFacts(ctx, tx, user, &candidate); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func lockAdministratorRetirementFacts(ctx context.Context, tx *sql.Tx, user uint64, candidate *AdministratorRetirementCandidate) error {
	// 全部采用当前锁定读：即使外层事务此前已建立RR快照，等待知识库锁后也必须
	// 看见前一事务刚提交的预留。普通COUNT快照读在这里会错误放行并发退休。
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member
		WHERE knowledge_base_id=? AND role='ADMIN' AND state='ACTIVE' ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)
	if err != nil {
		return err
	}
	available := map[uint64]struct{}{}
	for rows.Next() {
		var administrator uint64
		if err = rows.Scan(&administrator); err != nil {
			rows.Close()
			return err
		}
		available[administrator] = struct{}{}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT user_id,operation_id,state FROM admin_retirement
		WHERE knowledge_base_id=? ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)
	if err != nil {
		return err
	}
	for rows.Next() {
		var administrator uint64
		var operation, state string
		if err = rows.Scan(&administrator, &operation, &state); err != nil {
			rows.Close()
			return err
		}
		if state == "RESERVED" {
			delete(available, administrator)
		}
		if administrator == user {
			candidate.ReservationOperation = operation
			candidate.ReservationState = state
		}
	}
	err = rows.Err()
	rows.Close()
	candidate.Available = len(available)
	return err
}

// ReserveAdministratorRetirement 只负责预留事实；是否允许退休及最后管理员规则由用例层判断。
// 知识库锁是成员/预留行的前置锁，重复同号只返回成功，不能覆盖另一未完成操作。
func ReserveAdministratorRetirement(ctx context.Context, tx *sql.Tx, kb, user uint64, operation string) error {
	if kb == 0 || user == 0 || !domain.ValidOperationID(operation) {
		return errors.New("ADMIN_RETIREMENT_BINDING_INVALID")
	}
	var locked uint64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM knowledge_base WHERE id=? FOR UPDATE", kb).Scan(&locked); err != nil {
		return err
	}
	var existing, state string
	err := tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement
		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user).Scan(&existing, &state)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state)
			VALUES(?,?,?,'RESERVED')`, kb, user, operation)
		return err
	}
	if err != nil {
		return err
	}
	if state == "RESERVED" {
		if existing == operation {
			return nil
		}
		return domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
	}
	if state != "CANCELLED" && state != "APPLIED" {
		return errors.New("ADMIN_RETIREMENT_STATE_INVALID")
	}
	_, err = tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state='RESERVED',created_at=UTC_TIMESTAMP(6)
		WHERE knowledge_base_id=? AND user_id=? AND state IN ('CANCELLED','APPLIED')`, operation, kb, user)
	return err
}
