// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

// governance_identity.go：保留原实现，按业务合并文件。

// 冻结RPC已经绑定purpose/Target/原号；基础ACTIVE_ACCOUNT不能代替平台资格。
// 专用purpose在Acquire和Revalidate都核真实角色，并把可撤权代次写入同一准入。
func (s *Identity) admissionSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string) ([]*pb.SourceEpoch, error) {
	if purpose == "RETAINED_GOVERNANCE" && (capability != pb.IdentityCapability_ACTIVE_ACCOUNT || target == nil || target.Kind != pb.IdentityTarget_USER || target.UserId != actor || target.OrganizationId != 0 || target.SpaceId != 0) {
		return nil, identityDenied()
	}
	sources, err := s.currentSources(ctx, tx, actor, session, capability, target)
	if err != nil || purpose != "RETAINED_GOVERNANCE" {
		return sources, err
	}
	state, epoch, err := s.repository.LockGovernanceGrant(ctx, tx, actor)
	if err != nil || state != "ACTIVE" || epoch == 0 {
		return nil, identityDenied()
	}
	return append(sources, &pb.SourceEpoch{ScopeKey: "platform-admin:" + strconv.FormatUint(actor, 10), Epoch: epoch}), nil
}

// governance_operator.go：保留原实现，按业务合并文件。

// 仅由持有Account部署凭据的本机运维入口装配，不注册公共HTTP任免接口。
// 普通注册账号没有隐含管理员；运维原号/目标/预期修订和实际资格都有持久审计。
type GovernanceOperator struct{ db *sql.DB }

func NewGovernanceOperator(db *sql.DB) *GovernanceOperator { return &GovernanceOperator{db: db} }

type GovernanceOperatorInput struct {
	Operation string `json:"operation_id"`
	User      uint64 `json:"user_id"`
	Action    string `json:"action"`
	Expected  uint64 `json:"expected_revision"`
}

func (s *GovernanceOperator) Change(ctx context.Context, input GovernanceOperatorInput) (json.RawMessage, error) {
	if input.User == 0 || !domain.ValidOperationID(input.Operation) || (input.Action != "GRANT" && input.Action != "REVOKE") {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PLATFORM_GOVERNANCE_INPUT_INVALID"}
	}
	raw, _ := json.Marshal(input)
	fingerprint := sha256.Sum256(raw)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	active, err := repository.GovernanceUserActive(ctx, tx, input.User)
	if err != nil || !active {
		return nil, errors.New("PLATFORM_GOVERNANCE_USER_NOT_ACTIVE")
	}
	// 持目标user锁序列化同一角色变更；原号重取不能用后来修订改写原请求。
	receipt, err := repository.NewMutation().Receipt(ctx, tx, input.Operation)
	if err == nil {
		if receipt.ActorID != "0" || receipt.Kind != "PLATFORM_GOVERNANCE_OPERATOR" || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, errors.New("PLATFORM_GOVERNANCE_OPERATION_CONFLICT")
		}
		return receipt.Result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	grant, err := repository.GovernanceGrantRow(ctx, tx, input.User, true)
	if errors.Is(err, sql.ErrNoRows) {
		grant = repository.GovernanceGrant{}
	} else if err != nil {
		return nil, err
	}
	if grant.Revision != input.Expected || grant.State == "REVOKING" {
		return nil, errors.New("PLATFORM_GOVERNANCE_REVISION_CHANGED")
	}
	if grant.Epoch == ^uint64(0) || grant.Revision == ^uint64(0) {
		return nil, errors.New("PLATFORM_GOVERNANCE_EPOCH_EXHAUSTED")
	}
	state := "SUCCEEDED"
	switch input.Action {
	case "GRANT":
		if grant.State != "ACTIVE" {
			if err = repository.GrantGovernance(ctx, tx, input.User, input.Operation, grant); err != nil {
				return nil, err
			}
		}
	case "REVOKE":
		if grant.State == "ACTIVE" {
			if err = repository.RevokeGovernance(ctx, tx, input.User, input.Operation); err != nil {
				return nil, err
			}
			closure := domain.SessionClosure{Kind: "PLATFORM_GOVERNANCE_REVOKE", UserID: strconv.FormatUint(input.User, 10), Scope: "platform-admin:" + strconv.FormatUint(input.User, 10), Epoch: strconv.FormatUint(grant.Epoch, 10)}
			if err = EnqueueSourceClosure(ctx, tx, input.Operation, "0", closure); err != nil {
				return nil, err
			}
			state = "PREPARING"
		}
	}
	result, _ := json.Marshal(map[string]string{"operation_id": input.Operation, "state": state})
	if err = repository.InsertGovernanceOperatorReceipt(ctx, tx, input.Operation, fingerprint, raw, result, state); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("PLATFORM_GOVERNANCE_COMMIT_UNKNOWN")
	}
	return result, nil
}

func (s *GovernanceOperator) Current(ctx context.Context, user uint64) (map[string]string, error) {
	grant, err := repository.GovernanceGrantRow(ctx, s.db, user, false)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]string{"user_id": strconv.FormatUint(user, 10), "state": "CLOSED", "revision": "0"}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]string{"user_id": strconv.FormatUint(user, 10), "state": grant.State, "revision": strconv.FormatUint(grant.Revision, 10), "epoch": strconv.FormatUint(grant.Epoch, 10)}, nil
}

func (s *GovernanceOperator) Ready(ctx context.Context) error {
	return repository.GovernanceRecoveryReady(ctx, s.db)
}

// governance_revocation.go：保留原实现，按业务合并文件。

func (s *Closure) finishGovernance(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	user, err := strconv.ParseUint(binding.UserID, 10, 64)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != binding.UserID || binding.Scope != "platform-admin:"+binding.UserID {
		return
	}
	epoch, err := strconv.ParseUint(binding.Epoch, 10, 64)
	if err != nil || epoch == 0 || epoch == ^uint64(0) || strconv.FormatUint(epoch, 10) != binding.Epoch {
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	// 先角色，再任务ownership，与任免入口保持同一锁顺序；真实Doc ACK已经核验。
	grant, err := repository.GovernanceGrantRow(ctx, tx, user, true)
	if err != nil || grant.State != "REVOKING" || grant.Epoch != epoch+1 || grant.Operation != task.OperationID {
		return
	}
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	if err = repository.CompleteGovernanceRevocation(ctx, tx, user, epoch+1, task.OperationID); err != nil {
		return
	}
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED"})
	if err = s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal); err != nil {
		return
	}
	// 未知提交仍重取原角色/回执；绝不按租约结束补写成功。
	_ = tx.Commit()
}
