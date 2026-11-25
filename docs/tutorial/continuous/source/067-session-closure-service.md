# services/account/internal/logic/account/session_closure_service.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/067-session-closure-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`ClosurePeer`、`Closure`、`NewClosure`、`RecoverOne`、`finish`、`EnqueueSourceClosure`、`Logout`、`NewLogout`、`Resolve`、`Prepare`、`EnqueueExpired`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
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
	"log/slog"
	"strconv"
	"time"
)

// closure.go：保留原实现，按业务合并文件。

type ClosurePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
	CloseDepartedGrants(context.Context, domain.SessionClosure, string) error
	FailureCode(error) string
}

type Closure struct {
	db         *sql.DB
	repository *repository.Closure
	peer       ClosurePeer
}

func NewClosure(db *sql.DB, repo *repository.Closure, peer ClosurePeer) *Closure {
	return &Closure{db: db, repository: repo, peer: peer}
}

func (s *Closure) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	// 先提交固定任务的持久租约，网络调用在事务外；未知结果仍重取原任务。
	task, err := s.repository.Claim(ctx, tx)
	if err != nil {
		return
	}
	if s.repository.MarkRunning(ctx, tx, task.ID, instance) != nil || tx.Commit() != nil {
		return
	}
	var binding domain.SessionClosure
	var request *pb.FenceRequest
	if json.Unmarshal(task.Payload, &binding) == nil {
		epoch, epochError := strconv.ParseUint(binding.Epoch, 10, 64)
		session, sessionError := hex.DecodeString(binding.SessionHash)
		if epochError == nil && ((sessionError == nil && len(session) == 32) || binding.UserID != "") {
			request = &pb.FenceRequest{
				RevocationId: task.OperationID,
				ClosedScopes: []*pb.SourceEpoch{{ScopeKey: binding.Scope, Epoch: epoch}},
				Generation:   1,
			}
			if binding.UserID != "" {
				request.TargetDigest = platform.SourceClosureDigest(binding.Scope, epoch)
			} else {
				request.TargetDigest = platform.LogoutTargetDigest(binding.SessionHash)
			}
		}
	}
	if request == nil {
		s.repository.Retry(ctx, task.ID, instance, "LOGOUT_PAYLOAD_INVALID")
		return
	}
	// 离开关系才关闭组织下属授权；降级保留独立授权。持久授权关闭先于排空ACK。
	if binding.Kind == "ORG_LEAVE" || binding.Kind == "ORG_REMOVE" {
		if s.peer.CloseDepartedGrants(ctx, binding, task.OperationID) != nil {
			s.repository.Retry(ctx, task.ID, instance, "DEPARTURE_GRANTS_UNKNOWN")
			return
		}
	}
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		// 日志只记录稳定码，不输出私有payload、原始令牌和依赖驱动详情。
		slog.Warn("logout_fence_pending", "operation_id", task.OperationID, "rpc_code", s.peer.FailureCode(err))
		s.repository.Retry(ctx, task.ID, instance, "FENCE_NOT_CONFIRMED")
		return
	}
	s.finish(ctx, task, instance, binding, ack.ResultDigest)
}

func (s *Closure) finish(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	if binding.Kind == "PLATFORM_GOVERNANCE_REVOKE" {
		s.finishGovernance(ctx, task, instance, binding, digest)
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	isCredential := binding.Kind == "CHANGE_PASSWORD" || binding.Kind == "RESET_PASSWORD" || binding.Kind == "CHANGE_EMAIL"
	var oldEpoch uint64
	if isCredential || binding.OrganizationID != "" {
		oldEpoch, err = strconv.ParseUint(binding.Epoch, 10, 64)
		if err != nil || oldEpoch == ^uint64(0) {
			return
		}
	}
	session, _ := hex.DecodeString(binding.SessionHash)
	poll := "/v1/account/operations/" + task.OperationID
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED", PollUrl: &poll})
	if s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal) != nil {
		return
	}
	if isCredential && s.repository.FinishCredential(ctx, tx, binding.UserID, oldEpoch) != nil {
		return
	}
	if binding.OrganizationID != "" {
		switch binding.Kind {
		case "ORG_ROLE", "ORG_OWNER_TRANSFER":
			if s.repository.FinishMemberRole(ctx, tx, binding.OrganizationID, binding.UserID, binding.Role, oldEpoch+1) != nil {
				return
			}
		case "ORG_REMOVE", "ORG_LEAVE":
			if s.repository.FinishMemberDeparture(ctx, tx, binding.OrganizationID, binding.UserID, oldEpoch+1) != nil {
				return
			}
		}
		if s.repository.FinishOrganizationTransition(ctx, tx, task.OperationID) != nil {
			return
		}
		if binding.Kind == "ORG_OWNER_TRANSFER" {
			if s.repository.CreateOwnerTransferNotice(ctx, tx, task.OperationID, binding.OrganizationID, binding.UserID) != nil {
				return
			}
		}
	}
	// commit响应丢失仍是未知事实；下轮恢复读原记录，不能补写假的终态。
	_ = tx.Commit()
}

// closure_enqueue.go：保留原实现，按业务合并文件。

func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure) error {
	target, _ := json.Marshal(payload)
	return repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)
}

// logout.go：保留原实现，按业务合并文件。

type Logout struct {
	db             *sql.DB
	authentication *Authentication
	receipts       *repository.OperationReceipts
	mutations      *repository.Mutation
	closures       *repository.Closure
}

func NewLogout(db *sql.DB, authentication *Authentication, receipts *repository.OperationReceipts, mutations *repository.Mutation, closures *repository.Closure) *Logout {
	return &Logout{
		db:             db,
		authentication: authentication,
		receipts:       receipts,
		mutations:      mutations,
		closures:       closures,
	}
}

func (s *Logout) Resolve(ctx context.Context, id, primaryToken, recoveryToken string) (domain.LogoutSelection, error) {
	session, sessionError := s.authentication.Session(ctx, primaryToken)
	selection := domain.LogoutSelection{Session: session, Token: primaryToken}
	receipt, lookup := s.receipts.Lookup(ctx, id)
	if lookup == nil {
		// 只选择原已登记操作的会话证明；关闭的会话不能通过此分支发起新修改。
		if sessionError != nil || hex.EncodeToString(session.ID) != receipt.SessionHash {
			selection.Token = recoveryToken
			session, sessionError = s.authentication.Session(ctx, recoveryToken)
			selection.Session = session
		}
		if sessionError != nil {
			return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
		}
		if receipt.ActorID != session.UserID || receipt.Kind != "SESSION_LOGOUT" || receipt.SessionHash != hex.EncodeToString(session.ID) {
			return selection, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		selection.Registered = true
		selection.ReceiptState = receipt.State
		return selection, nil
	}
	if lookup != sql.ErrNoRows {
		return selection, lookup
	}
	if sessionError != nil || session.State != "ACTIVE" || !session.Expires.After(time.Now()) {
		return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	return selection, nil
}

func (s *Logout) Prepare(ctx context.Context, id string, session domain.Session) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// 与登录/凭据修改相同，先锁用户再锁会话，防止相反锁顺序导致死锁。
	credential, err := s.mutations.LockUser(ctx, tx, session.UserID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	current, err := s.closures.LockExpirySession(ctx, tx, session.ID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	if credential.State != "ACTIVE" || credential.Epoch != session.CredentialEpoch || !current.Expires.After(time.Now()) {
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	if current.State != "ACTIVE" {
		// 初次查询没有receipt的同号请求，等锁后必须重读持久绑定。
		bound, state, err := s.receipts.ConcurrentLogout(ctx, tx, id)
		if err == nil && bound == hex.EncodeToString(session.ID) {
			return state, nil
		}
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	digest := hex.EncodeToString(session.ID)
	scope := "session:" + digest
	target, _ := json.Marshal(map[string]string{"session_hash": digest, "scope_key": scope})
	fingerprint := sha256.Sum256([]byte("SESSION_LOGOUT\n" + session.UserID + "\n" + digest))
	payload, _ := json.Marshal(map[string]string{
		"session_hash": digest,
		"scope_key":    scope,
		"closed_epoch": strconv.FormatUint(current.Epoch, 10),
	})
	preparation := domain.SessionClosePreparation{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Epoch:       current.Epoch,
		OperationID: id,
		Scope:       scope,
		Target:      target,
		Payload:     payload,
		Fingerprint: fingerprint[:],
	}
	if s.closures.PrepareSessionClose(ctx, tx, preparation) != nil || tx.Commit() != nil {
		return "", errors.New("LOGOUT_COMMIT_UNKNOWN")
	}
	return "PREPARING", nil
}

// session_expiry.go：保留原实现，按业务合并文件。

// 浏览器可能断电；会话自然到期也必须形成持久关闭，不能等待用户点击退出。
func (s *Closure) EnqueueExpired(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	session, err := s.repository.ExpiredSession(ctx)
	if err != nil {
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	// 与显式退出一样先锁用户再锁会话；池外发现过期不代表事务内仍可关闭。
	if s.repository.LockExpiryUser(ctx, tx, session.UserID) != nil {
		return
	}
	current, err := s.repository.LockExpirySession(ctx, tx, session.ID)
	if err != nil || current.State != "ACTIVE" || current.Expires.After(time.Now()) || current.Epoch == ^uint64(0) {
		return
	}
	digest := hex.EncodeToString(session.ID)
	scope := "session:" + digest
	operationID := "expiry-" + digest
	target, _ := json.Marshal(map[string]string{"session_hash": digest, "scope_key": scope})
	payload, _ := json.Marshal(domain.SessionClosure{SessionHash: digest, Scope: scope, Epoch: strconv.FormatUint(current.Epoch, 10)})
	fingerprint := sha256.Sum256([]byte("SESSION_EXPIRY\n" + digest))
	value := domain.SessionClosePreparation{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Epoch:       current.Epoch,
		OperationID: operationID,
		Scope:       scope,
		Target:      target,
		Payload:     payload,
		Fingerprint: fingerprint[:],
	}
	if s.repository.PrepareSessionClose(ctx, tx, value) != nil {
		return
	}
	_ = tx.Commit()
}
````

UTF-8/LF全文SHA-256：`1a8e0104e7bad28bd3932a0c7f8b910ebdfc889ccff3cb729ad34aba48adfc54`。[参考实现](../../../../services/account/internal/logic/account/session_closure_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
