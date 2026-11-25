// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"slices"
	"strconv"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

type AccountClosures struct {
	db             *sql.DB
	repository     *repository.AccountClosures
	authentication *Authentication
}

func NewAccountClosures(db *sql.DB, authentication *Authentication) *AccountClosures {
	return &AccountClosures{db: db, repository: repository.NewAccountClosures(db), authentication: authentication}
}

func accountCloseConflict(code string) error {
	return domain.RuleViolation{Kind: domain.StateConflict, Code: code}
}

func (s *AccountClosures) Begin(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error) {
	return s.beginClose(ctx, session, input, fingerprint, false)
}

// Start是完整用户用例；原操作与自动恢复意图同事务写入。内部Begin仍用于
// 已有控制协议阶段验证，不将内部准备调用误当成用户已发起整条不可逆流程。
func (s *AccountClosures) Start(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte) (dto.AccountClosePlan, error) {
	return s.beginClose(ctx, session, input, fingerprint, true)
}

func (s *AccountClosures) beginClose(ctx context.Context, session domain.Session, input dto.AccountClose, fingerprint []byte, automatic bool) (dto.AccountClosePlan, error) {
	user, err := strconv.ParseUint(session.UserID, 10, 64)
	epoch, valid := domain.CredentialEpoch(input.ExpectedIdentityEpoch)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != session.UserID || !valid || !domain.ValidOperationID(input.OperationId) || len(session.ID) != 32 || len(fingerprint) != 32 {
		return dto.AccountClosePlan{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ACCOUNT_CLOSE_INPUT_INVALID"}
	}
	// 原会话只允许读取完全相同的原操作。先查回执再验密码，身份进入REVOKING后
	// 仍能恢复原请求；另一个操作编号必须重新通过当前凭据与会话资格检查。
	plan, found, err := s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, nil, automatic)
	if err != nil || found {
		return plan, err
	}
	hash, err := s.repository.Hash(ctx, user)
	if err != nil {
		return plan, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(input.CurrentPassword)) != nil {
		return plan, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "CURRENT_PASSWORD_INVALID"}
	}
	// bcrypt在事务外运行；取得user锁之后比较原hash及两个代次，防止旧密码验证穿越凭据变更。
	plan, _, err = s.begin(ctx, user, epoch, session, input.OperationId, fingerprint, hash, automatic)
	return plan, err
}

func (s *AccountClosures) begin(ctx context.Context, user, epoch uint64, session domain.Session, operation string, fingerprint, verifiedHash []byte, automatic bool) (dto.AccountClosePlan, bool, error) {
	var plan dto.AccountClosePlan
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return plan, false, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err == nil {
		if receipt.User != user || receipt.Kind != "ACCOUNT_CLOSE" || receipt.SessionHash != hex.EncodeToString(session.ID) || !bytes.Equal(receipt.Fingerprint, fingerprint) {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		record, err := s.repository.Record(ctx, tx, operation)
		if err != nil {
			return plan, true, err
		}
		capture, err := domain.DecodeAccountCloseCapture(record.Capture)
		if err != nil {
			return plan, true, err
		}
		if record.User != user || capture.SessionHash != receipt.SessionHash || capture.IdentityEpoch != epoch || strconv.FormatUint(capture.CredentialEpoch, 10) != session.CredentialEpoch {
			return plan, true, accountCloseConflict("OPERATION_ID_CONFLICT")
		}
		if capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation) {
			return plan, true, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
		}
		plan, err = accountCloseSavedPlan(operation, record, receipt)
		if err == nil && automatic {
			if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err == nil {
				err = tx.Commit()
			}
		}
		return plan, true, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return plan, false, err
	}
	if verifiedHash == nil {
		return plan, false, nil
	}
	if credential.State != "ACTIVE" || credential.IdentityEpoch != epoch || credential.Epoch != session.CredentialEpoch || !bytes.Equal(credential.Hash, verifiedHash) {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IDENTITY_CHANGED")
	}
	current, err := s.repository.LockSession(ctx, tx, session.ID)
	if err != nil {
		return plan, false, err
	}
	if current.UserID != session.UserID || current.State != "ACTIVE" || current.CredentialEpoch != credential.Epoch || current.Epoch != session.Epoch || !current.Expires.After(time.Now()) {
		return plan, false, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	pending, err := s.repository.HasPending(ctx, tx, user)
	if err != nil {
		return plan, false, err
	}
	if pending {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_IN_PROGRESS")
	}
	credentialEpoch, valid := domain.CredentialEpoch(credential.Epoch)
	if !valid {
		return plan, false, accountCloseConflict("ACCOUNT_CLOSE_CREDENTIAL_INVALID")
	}
	capture := domain.AccountCloseCapture{SessionHash: hex.EncodeToString(session.ID), CredentialEpoch: credentialEpoch, IdentityEpoch: epoch}
	plan = dto.AccountClosePlan{OperationId: operation, State: "PREPARING", RetainedReleaseIds: []string{}, RequiredSuccessions: []struct {
		Kind     string `json:"kind"`
		TargetId string `json:"target_id"`
	}{}}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, false, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, false, err
	}
	target, err := json.Marshal(map[string]string{"user_id": session.UserID, "session_hash": capture.SessionHash})
	if err != nil {
		return plan, false, err
	}
	if err = s.repository.Prepare(ctx, tx, user, operation, rawCapture, target, fingerprint, rawPlan); err != nil {
		return plan, false, err
	}
	if automatic {
		if err = repository.QueueAccountCloseRecovery(ctx, tx, operation, user); err != nil {
			return plan, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return plan, false, errors.New("ACCOUNT_CLOSE_COMMIT_UNKNOWN")
	}
	return plan, true, nil
}

func accountCloseSavedPlan(operation string, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) (dto.AccountClosePlan, error) {
	var plan dto.AccountClosePlan
	if receipt.Kind != "ACCOUNT_CLOSE" || receipt.User != record.User || json.Unmarshal(receipt.Result, &plan) != nil || plan.OperationId != operation || plan.State != record.State || plan.RequiredSuccessions == nil || plan.RetainedReleaseIds == nil {
		return plan, errors.New("ACCOUNT_CLOSE_RECEIPT_INVALID")
	}
	expected := "PREPARING"
	if record.State == "SUCCEEDED" || record.State == "REJECTED" {
		expected = record.State
	}
	if receipt.State != expected {
		return plan, errors.New("ACCOUNT_CLOSE_RECEIPT_INVALID")
	}
	return plan, nil
}

// lockBinding沿用user→receipt→account_close顺序。原关闭组织资格由编排器在固定下游
// 计划前核验，本层仅维护原用户/身份代次以及绑定不可替换的持久约束。
func (s *AccountClosures) lockBinding(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error) {
	var record domain.AccountCloseRecord
	var capture domain.AccountCloseCapture
	var plan dto.AccountClosePlan
	if !platform.ValidateAccountClosureBinding(binding) {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_INVALID")
	}
	if _, err := s.repository.LockUser(ctx, tx, binding.User); err != nil {
		return record, capture, plan, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, binding.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	record, err = s.repository.Record(ctx, tx, binding.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	capture, err = domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return record, capture, plan, err
	}
	if record.User != binding.User || receipt.User != binding.User || receipt.SessionHash != capture.SessionHash || capture.IdentityEpoch != binding.IdentityEpoch {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if capture.Binding != nil && !platform.EqualAccountClosureBinding(*capture.Binding, binding) {
		return record, capture, plan, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	plan, err = accountCloseSavedPlan(binding.Operation, record, receipt)
	return record, capture, plan, err
}

func (s *AccountClosures) Bind(ctx context.Context, binding platform.AccountClosureBinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	if err != nil {
		return err
	}
	if record.State != "PREPARING" {
		return accountCloseConflict("ACCOUNT_CLOSE_NOT_PREPARING")
	}
	if capture.Binding != nil {
		return nil
	}
	capture.Binding = &binding
	raw, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	if err = s.repository.Bind(ctx, tx, binding.Operation, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_CLOSE_BIND_UNKNOWN")
	}
	return nil
}

func (s *AccountClosures) Reject(ctx context.Context, binding platform.AccountClosureBinding, reason string) error {
	if !domain.ValidIdentityPurpose(reason) {
		return domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ACCOUNT_CLOSE_REASON_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, capture, plan, err := s.lockBinding(ctx, tx, binding)
	if err != nil {
		return err
	}
	if capture.Binding == nil {
		return accountCloseConflict("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	if record.State == "REJECTED" {
		return nil
	}
	// FROZEN及之后阶段可能已产生不可逆事实，不能用准备期拒绝释放下游预留。
	if record.State != "PREPARING" {
		return accountCloseConflict("ACCOUNT_CLOSE_NOT_PREPARING")
	}
	plan.State = "REJECTED"
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = s.repository.Reject(ctx, tx, binding.Operation, reason, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_CLOSE_REJECTION_UNKNOWN")
	}
	return nil
}

func (s *AccountClosures) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	// 不存在只表示没有拒绝证明；查询失败、超时及失配不能推断为已拒绝。
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if capture.Binding == nil {
		return false, nil
	}
	return record.State == "REJECTED", nil
}

// account_close_binding.go：保留原实现，按业务合并文件。

// BindingForDocument只交付原操作已经持久固定的关闭范围；不能用当前身份epoch或
// 当前组织成员图补造绑定。它不授权注销成功，也不创建/推进/取消任何操作。
func (s *AccountClosures) BindingForDocument(ctx context.Context, operation string) (platform.AccountClosureBinding, error) {
	var empty platform.AccountClosureBinding
	if !domain.ValidOperationID(operation) {
		return empty, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	user, err := s.repository.UserForOperation(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return empty, accountCloseNotFound()
	}
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if _, err = s.repository.LockUser(ctx, tx, user); err != nil {
		return empty, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err != nil {
		return empty, err
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return empty, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return empty, err
	}
	if _, err = accountCloseSavedPlan(operation, record, receipt); err != nil {
		return empty, err
	}
	if capture.Binding == nil {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	binding := *capture.Binding
	if binding.Operation != operation || binding.User != user || record.User != user || receipt.User != user ||
		receipt.SessionHash != capture.SessionHash || binding.IdentityEpoch != capture.IdentityEpoch {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if record.State == "REJECTED" {
		return empty, accountCloseConflict("ACCOUNT_CLOSE_REJECTED")
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_CLOSE_BINDING_READ_UNKNOWN")
	}
	return binding, nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

type AccountCloseFencePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
}

type AccountCloseFences struct {
	db         *sql.DB
	repository *repository.AccountCloseFences
	peer       AccountCloseFencePeer
}

func NewAccountCloseFences(db *sql.DB, peer AccountCloseFencePeer) *AccountCloseFences {
	return &AccountCloseFences{db: db, repository: repository.NewAccountCloseFences(db), peer: peer}
}

func (s *AccountCloseFences) Ready(ctx context.Context) error {
	invalid, err := s.repository.InvalidTasks(ctx)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
	}
	return nil
}

func (s *AccountCloseFences) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	task, err := s.repository.Claim(ctx, tx, instance)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	payload, err := s.repository.Binding(ctx, s.db, task)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
		return
	}
	request := &pb.FenceRequest{RevocationId: payload.Revocation, Generation: 1,
		ClosedScopes: []*pb.SourceEpoch{{ScopeKey: payload.Scope, Epoch: payload.Epoch}},
		TargetDigest: platform.SourceClosureDigest(payload.Scope, payload.Epoch)}
	// 持久租约已提交后才调用真实Doc屏障。协议generation固定为1，不能把任务领取代次
	// 填入原屏障请求，否则恢复会悄悄变成另一份业务操作。
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_NOT_CONFIRMED")
		return
	}
	finished, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = s.repository.Finish(ctx, finished, task, payload, ack.ResultDigest); err != nil {
		_ = finished.Rollback()
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
		return
	}
	if err = finished.Commit(); err != nil {
		// 条件重试不会改写已提交成功的任务；提交响应丢失仍按未知结果处理。
		_ = s.repository.Retry(ctx, task, "ACCOUNT_CLOSE_FENCE_FINISH_UNKNOWN")
	}
}

// account_close_prepare.go：保留原实现，按业务合并文件。

type AccountClosurePreparer interface {
	Prepare(context.Context, platform.AccountClosureBinding) (platform.AccountClosurePreparation, error)
	Cancel(context.Context, platform.AccountClosureBinding) error
}

// PrepareClose先固定Account组织事实，再取得Doc预留，最后复核同一快照并原子冻结。
// 网络调用不持SQL锁；中间事实改变时只能拒绝原计划，不能静默替换关闭组织集合。
func (s *AccountClosures) PrepareClose(ctx context.Context, operation string, peer AccountClosurePreparer) (dto.AccountClosePlan, error) {
	plan, binding, err := s.prepareCloseStage(ctx, operation, nil)
	if err != nil || binding == nil {
		return plan, err
	}
	if peer == nil {
		return plan, errors.New("ACCOUNT_CLOSE_CONTROL_REQUIRED")
	}
	if plan.State == "REJECTED" {
		return plan, peer.Cancel(ctx, *binding)
	}
	prepared, err := peer.Prepare(ctx, *binding)
	if err != nil {
		// 回复未知仍保持同一PREPARING记录，重试原号向Doc取回已保存的预留。
		return plan, err
	}
	plan, binding, err = s.prepareCloseStage(ctx, operation, &prepared)
	if err == nil && plan.State == "REJECTED" && binding != nil {
		err = peer.Cancel(ctx, *binding)
	}
	return plan, err
}

func (s *AccountClosures) prepareCloseStage(ctx context.Context, operation string, prepared *platform.AccountClosurePreparation) (dto.AccountClosePlan, *platform.AccountClosureBinding, error) {
	var plan dto.AccountClosePlan
	user, err := s.repository.UserForOperation(ctx, operation)
	if err != nil {
		return plan, nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return plan, nil, err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return plan, nil, err
	}
	plan, err = accountCloseSavedPlan(operation, record, receipt)
	if err != nil {
		return plan, nil, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return plan, nil, err
	}
	if record.User != user || receipt.SessionHash != capture.SessionHash || (capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != operation)) {
		return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_BINDING_CONFLICT")
	}
	if record.State == "REJECTED" {
		return plan, capture.Binding, nil
	}
	if record.State != "PREPARING" {
		return plan, nil, nil
	}
	organizations, err := s.repository.LockOrganizations(ctx, tx, user)
	if err != nil {
		return plan, nil, err
	}
	identityMatches := credential.State == "ACTIVE" && credential.IdentityEpoch == capture.IdentityEpoch && credential.Epoch == strconv.FormatUint(capture.CredentialEpoch, 10)
	if !identityMatches || (capture.Binding != nil && !slices.Equal(capture.Organizations, organizations)) {
		plan.State = "REJECTED"
		raw, marshalErr := json.Marshal(plan)
		if marshalErr != nil {
			return plan, nil, marshalErr
		}
		if err = s.repository.Reject(ctx, tx, operation, "PREPARATION_CHANGED", raw); err != nil {
			return plan, nil, err
		}
		if err = tx.Commit(); err != nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_REJECTION_UNKNOWN")
		}
		return plan, capture.Binding, nil
	}
	plan.RequiredSuccessions = plan.RequiredSuccessions[:0]
	closing := []uint64{}
	for _, org := range organizations {
		if org.Lifecycle != "ACTIVE" || org.MemberState != "ACTIVE" {
			return plan, nil, accountCloseConflict("ACCOUNT_CLOSE_ORGANIZATION_PENDING")
		}
		if org.Owner == user {
			if org.LiveMembers == 1 {
				closing = append(closing, org.ID)
			} else {
				appendAccountCloseSuccession(&plan, "ORGANIZATION_OWNER", org.ID)
			}
		} else if org.Role == "ADMIN" && org.AvailableAdministrators <= 1 {
			appendAccountCloseSuccession(&plan, "ORGANIZATION_ADMIN", org.ID)
		}
	}
	if capture.Binding == nil && len(plan.RequiredSuccessions) == 0 {
		capture.Binding = &platform.AccountClosureBinding{Operation: operation, User: user, IdentityEpoch: capture.IdentityEpoch, ClosingOrganizations: closing}
		capture.Organizations = organizations
	}
	freeze := false
	if prepared != nil {
		if capture.Binding == nil || prepared.RequiredKnowledgeBases == nil || prepared.ReservedKnowledgeBases == nil {
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
		}
		switch prepared.State {
		case "HANDOFF_REQUIRED":
			if len(prepared.RequiredKnowledgeBases) == 0 || len(prepared.ReservedKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			for _, kb := range prepared.RequiredKnowledgeBases {
				if kb == 0 {
					return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
				}
				appendAccountCloseSuccession(&plan, "KNOWLEDGE_BASE_ADMIN", kb)
			}
		case "FROZEN":
			if len(plan.RequiredSuccessions) != 0 || len(prepared.RequiredKnowledgeBases) != 0 {
				return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_INVALID")
			}
			freeze = true
			plan.State = "FROZEN"
			capture.Scopes = accountCloseScopes(*capture.Binding, organizations)
		default:
			return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_CONFLICT")
		}
	}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return plan, nil, err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return plan, nil, err
	}
	if freeze {
		err = s.repository.Freeze(ctx, tx, *capture.Binding, capture, rawPlan)
	} else {
		err = s.repository.SavePreparation(ctx, tx, operation, rawCapture, rawPlan)
	}
	if err != nil {
		return plan, nil, err
	}
	if err = tx.Commit(); err != nil {
		return plan, nil, errors.New("ACCOUNT_CLOSE_PREPARATION_UNKNOWN")
	}
	return plan, capture.Binding, nil
}

func appendAccountCloseSuccession(plan *dto.AccountClosePlan, kind string, target uint64) {
	plan.RequiredSuccessions = append(plan.RequiredSuccessions, struct {
		Kind     string `json:"kind"`
		TargetId string `json:"target_id"`
	}{Kind: kind, TargetId: strconv.FormatUint(target, 10)})
}

func accountCloseScopes(binding platform.AccountClosureBinding, organizations []domain.AccountCloseOrganization) []domain.AccountCloseScope {
	user := strconv.FormatUint(binding.User, 10)
	scopes := []domain.AccountCloseScope{{Revocation: binding.Operation, Scope: "identity:" + user, Epoch: binding.IdentityEpoch}}
	for _, org := range organizations {
		scope := "org-member:" + strconv.FormatUint(org.ID, 10) + ":" + user
		// 子撤权号固定且有界，不因原操作号接近最大长度而截断产生碰撞。
		digest := sha256.Sum256([]byte(binding.Operation + "\n" + scope))
		scopes = append(scopes, domain.AccountCloseScope{Revocation: "account-close-" + hex.EncodeToString(digest[:]), Scope: scope, Epoch: org.Epoch})
	}
	return scopes
}
