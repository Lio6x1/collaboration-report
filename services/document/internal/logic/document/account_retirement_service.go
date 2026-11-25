// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// account_retirement.go：保留原实现，按业务合并文件。

type AccountRetirementAuthority interface {
	Rejected(context.Context, platform.AccountClosureBinding) (bool, error)
}

type AccountRetirements struct {
	db        *sql.DB
	authority AccountRetirementAuthority
}

func NewAccountRetirements(db *sql.DB, authority AccountRetirementAuthority) *AccountRetirements {
	return &AccountRetirements{db: db, authority: authority}
}

// Prepare是跨服务注销准备的一部分，不是注销完成。调用者必须是已验证原操作的
// Account控制通道；当前不作为用户HTTP入口，不能接收用户自行声明的关闭组织集合。
func (s *AccountRetirements) Prepare(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementResult, error) {
	result := domain.AccountRetirementResult{RequiredKnowledgeBases: []uint64{}, ReservedKnowledgeBases: []uint64{}}
	if !binding.Valid() {
		return result, errors.New("ACCOUNT_RETIREMENT_BINDING_INVALID")
	}
	// 不存在的原操作不需要范围锁：RR的缺行间隙锁会与随后知识库锁反向等待。
	// RC下仍显式锁定身份来源、知识库、成员和退休事实；操作唯一键处理跨用户撞号。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// 与任命目标身份、本人建库使用同一来源锁，预留后不能迟到获得新的管理员职责。
	gate, err := repository.LockSourceGate(ctx, tx, "identity:"+domain.TextID(binding.User), binding.IdentityEpoch)
	if err != nil {
		return result, err
	}
	record, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err == nil {
		plan, decodeErr := domain.DecodeAccountRetirementPlan(record.Plan)
		if decodeErr != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
		}
		if record.State == "REJECTED" {
			result.State = "REJECTED"
			return result, nil
		}
		matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
		if err != nil {
			return result, err
		}
		if !matches {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
		}
		result.State, result.ReservedKnowledgeBases = record.State, plan.ReservedKnowledgeBases
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if gate.Epoch > binding.IdentityEpoch || (gate.Epoch == binding.IdentityEpoch && gate.State == "BLOCKED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	candidates, err := repository.LockAdministratorRetirementCandidates(ctx, tx, binding.User)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		// 自有个人空间及权威明确将关闭的组织不属于继续存续的库。
		if candidate.PersonalOwner == binding.User || slices.Contains(binding.ClosingOrganizations, candidate.Organization) {
			continue
		}
		if candidate.Lifecycle != "ACTIVE" || candidate.MemberState != "ACTIVE" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_TRANSITION_PENDING")
		}
		if candidate.ReservationState == "RESERVED" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
		}
		if candidate.Available <= 1 {
			result.RequiredKnowledgeBases = append(result.RequiredKnowledgeBases, candidate.KnowledgeBase)
		}
		result.ReservedKnowledgeBases = append(result.ReservedKnowledgeBases, candidate.KnowledgeBase)
	}
	if len(result.RequiredKnowledgeBases) > 0 {
		// 不提交任何来源或部分预留，用户仍可正常交接后重新发起。
		result.State, result.ReservedKnowledgeBases = "HANDOFF_REQUIRED", []uint64{}
		return result, nil
	}
	for _, kb := range result.ReservedKnowledgeBases {
		if err = repository.ReserveAdministratorRetirement(ctx, tx, kb, binding.User, binding.Operation); err != nil {
			return result, err
		}
	}
	plan, err := json.Marshal(domain.AccountRetirementPlan{Binding: binding, ReservedKnowledgeBases: result.ReservedKnowledgeBases})
	if err != nil {
		return result, err
	}
	if err = repository.RecordAccountRetirement(ctx, tx, binding, plan); err != nil {
		return result, err
	}
	if err = repository.FreezeAccountRetirement(ctx, tx, binding); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, errors.New("ACCOUNT_RETIREMENT_PREPARE_UNKNOWN")
	}
	result.State = "FROZEN"
	return result, nil
}

// account_retirement_cancel.go：保留原实现，按业务合并文件。

// Cancel仅接受Account对原固定计划的持久REJECTED证明。超时、查无记录和租约到期
// 都不是回滚证明；已开始实际排空的操作也不能通过这个准备阶段接口撤销。
func (s *AccountRetirements) Cancel(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.authority == nil {
		return errors.New("ACCOUNT_RETIREMENT_AUTHORITY_REQUIRED")
	}
	rejected, err := s.authority.Rejected(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return err
	}
	if !rejected {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_CANCEL_UNCONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = repository.LockSourceGate(ctx, tx, "identity:"+domain.TextID(binding.User), binding.IdentityEpoch); err != nil {
		return err
	}
	record, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if errors.Is(err, sql.ErrNoRows) {
		// Prepare可能尚未到达。权威拒绝仍须留下同号墓碑，禁止迟到请求重新封门；
		// 不触碰其他操作的来源状态或预留，也不把已开始的排空伪装为未发生。
		if _, err = repository.LockedFence(ctx, tx, binding.Operation); err == nil {
			return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = repository.RecordRejectedAccountRetirement(ctx, tx, binding); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return errors.New("ACCOUNT_RETIREMENT_CANCEL_UNKNOWN")
		}
		return nil
	}
	if err != nil {
		return err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
	}
	if record.State == "REJECTED" {
		return nil
	}
	if record.State != "FROZEN" {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_ALREADY_ADVANCED")
	}
	matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
	if err != nil {
		return err
	}
	if !matches {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	if _, err = repository.LockedFence(ctx, tx, binding.Operation); err == nil {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = repository.CancelAccountRetirement(ctx, tx, plan); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETIREMENT_CANCEL_UNKNOWN")
	}
	return nil
}
