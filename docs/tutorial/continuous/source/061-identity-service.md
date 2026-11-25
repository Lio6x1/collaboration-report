# services/account/internal/logic/account/identity_service.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/061-identity-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`AuthorizationResults`、`NewAuthorizationResults`、`authorizationInvalid`、`authorizationUnavailable`、`facts`、`Fence`、`Get`、`authorizationQueryTarget`、`canonicalAuthorizationJSON`、`sameAuthorizationJSON`、`authorizationProof`、`validAuthorizationTaskState`、`validAuthorizationOriginal`、`validAccountCloseAuthorization`、`Cursors`、`NewCursors`、`Decode`、`Encode`、`Identity`、`NewIdentity`、`admissionBinding`、`identityDenied`、`identityUnavailable`、`AcquireIdentityAdmission`、`RevalidateIdentityAdmission`、`DocumentCollaborator`、`revalidateIdentityAdmission`、`currentSources`、`lockIdentityGates`、`sameSources`、`RenewOperationAdmission`、`MutationBody`、`Mutations`、`NewMutations`、`CheckSession`、`Run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// authorization_result.go：保留原实现，按业务合并文件。

type AuthorizationResults struct {
	repository *repository.AuthorizationResults
}

func NewAuthorizationResults(db *sql.DB) *AuthorizationResults {
	return &AuthorizationResults{repository: repository.NewAuthorizationResults(db)}
}

func authorizationInvalid() error {
	return domain.RuleViolation{Kind: domain.InvalidRequest, Code: "AUTHORIZATION_REQUEST_BINDING_INVALID"}
}

func authorizationUnavailable() error {
	return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "AUTHORIZATION_PROOF_UNAVAILABLE"}
}

func (s *AuthorizationResults) facts(ctx context.Context, operation string) (repository.AuthorizationResultFacts, *pb.FenceRequest, error) {
	if !domain.ValidOperationID(operation) {
		return repository.AuthorizationResultFacts{}, nil, authorizationInvalid()
	}
	f, err := s.repository.Read(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return f, nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "AUTHORIZATION_CHANGE_NOT_FOUND"}
	}
	if err != nil {
		return f, nil, authorizationUnavailable()
	}
	request, err := authorizationProof(operation, f)
	return f, request, err
}

// Fence只反向证明Account原关闭范围，绝不登记新范围或再调用Doc Fence。
// 真实Doc ACK未落库时返回PROCESSING，由原Account持久协调器继续恢复。
func (s *AuthorizationResults) Fence(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error) {
	if !platform.ValidBootstrapFence(input) {
		return nil, authorizationInvalid()
	}
	f, original, err := s.facts(ctx, input.RevocationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(original, input) {
		return nil, authorizationInvalid()
	}
	if f.State != "SUCCEEDED" {
		return &pb.FenceResult{State: pb.FenceResult_PROCESSING, RevocationId: original.RevocationId, ClosedScopes: original.ClosedScopes, Generation: original.Generation}, nil
	}
	// Account摘要只在原持久关闭和实际参与者ACK已核验后签出，不把空参与者当成功。
	return platform.BootstrapAck(original, "account"), nil
}

func (s *AuthorizationResults) Get(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error) {
	if input == nil || input.Target == nil || len(input.RequestFingerprint) != sha256.Size {
		return nil, authorizationInvalid()
	}
	f, _, err := s.facts(ctx, input.OperationId)
	if err != nil {
		return nil, err
	}
	target, err := authorizationQueryTarget(f)
	if err != nil {
		return nil, authorizationUnavailable()
	}
	if !proto.Equal(input.Target, target) || !bytes.Equal(input.RequestFingerprint, f.Fingerprint) {
		return nil, authorizationInvalid()
	}
	state := "PROCESSING"
	result := &pb.OperationResult{OperationId: input.OperationId, State: pb.OperationResult_PROCESSING, Generation: 1}
	if f.State == "SUCCEEDED" {
		state = "SUCCEEDED"
		result.State = pb.OperationResult_SUCCEEDED
	}
	raw, err := json.Marshal(struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}{input.OperationId, state, f.Scope, f.Epoch})
	if err != nil {
		return nil, authorizationUnavailable()
	}
	if result.State == pb.OperationResult_PROCESSING {
		result.CurrentStateJsonUtf8 = raw
		return result, nil
	}
	// 原业务协议generation恒为1，不读可变worker租约代次。授权表无独立revision：
	// terminal_revision=1定义为这个不可变范围的OUTPUT_STOPPED固定版本，仅真实持久终态赋值。
	result.TerminalRevision, result.FixedResultJsonUtf8 = 1, raw
	digest := sha256.Sum256(raw)
	result.ResultDigest = digest[:]
	return result, nil
}

// 冻结Target只有文档字段，Account采用明确固定映射：所有文档ID为0，workspace_epoch
// 是原来源epoch，purpose为前缀+scope+两个原完整JSON目标的规范摘要。不能猜文档ID。
// 子撤权没有自己的receipt，指纹和operation_target明确取原ACCOUNT_CLOSE父回执。
func authorizationQueryTarget(f repository.AuthorizationResultFacts) (*pb.Target, error) {
	authorization, err := canonicalAuthorizationJSON(f.Target)
	if err != nil {
		return nil, err
	}
	operation, err := canonicalAuthorizationJSON(f.ReceiptTarget)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(struct {
		Operation     string          `json:"receipt_operation_id"`
		Authorization json.RawMessage `json:"authorization_target"`
		Target        json.RawMessage `json:"operation_target"`
	}{f.TaskOperation, authorization, operation})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &pb.Target{WorkspaceEpoch: f.Epoch, Purpose: "ACCOUNT_AUTHORIZATION_CHANGE:" + f.Scope + ":" + hex.EncodeToString(digest[:])}, nil
}

func canonicalAuthorizationJSON(raw []byte) ([]byte, error) {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF || value == nil {
		return nil, errors.New("AUTHORIZATION_TARGET_INVALID")
	}
	return json.Marshal(value)
}

func sameAuthorizationJSON(first, second []byte) bool {
	a, err := canonicalAuthorizationJSON(first)
	if err != nil {
		return false
	}
	b, err := canonicalAuthorizationJSON(second)
	return err == nil && bytes.Equal(a, b)
}

func authorizationProof(operation string, f repository.AuthorizationResultFacts) (*pb.FenceRequest, error) {
	var participants []string
	if !platform.ValidAccountOutputScope(f.Scope) || f.Epoch == 0 || f.Actor != f.ReceiptActor || len(f.Fingerprint) != sha256.Size ||
		(f.State != "FENCING" && f.State != "SUCCEEDED") || (f.ReceiptState != "PREPARING" && f.ReceiptState != "SUCCEEDED") ||
		json.Unmarshal(f.Participants, &participants) != nil || len(participants) != 1 || participants[0] != "document" || !validAuthorizationTaskState(f.TaskState) {
		return nil, authorizationUnavailable()
	}
	if !validAuthorizationOriginal(operation, f) {
		return nil, authorizationUnavailable()
	}
	// 普通关闭在同一事务结束receipt；账号/组织关闭的来源ACK先于全局业务终态，
	// 不能把它们仍在清理的父回执误当缺少来源证明。
	if (f.State == "FENCING" && f.ReceiptState != "PREPARING") ||
		(f.State == "SUCCEEDED" && f.TaskKind != domain.AccountCloseFenceKind && f.TaskKind != repository.OrganizationCloseRecoveryKind && (f.ReceiptState != "SUCCEEDED" || !f.ReceiptTerminal)) {
		return nil, authorizationUnavailable()
	}
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: f.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(f.Scope, f.Epoch)}
	if strings.HasPrefix(f.Scope, "session:") {
		request.TargetDigest = platform.LogoutTargetDigest(strings.TrimPrefix(f.Scope, "session:"))
	}
	if !platform.ValidBootstrapFence(request) {
		return nil, authorizationUnavailable()
	}
	// 同代gate须仍是原关闭点；更高代次只允许带真实终态ACK的旧范围重读。
	// 新身份来源可重新OPEN，但不能抹掉已完成的旧epoch或替旧未知范围补证明。
	if f.GateEpoch < f.Epoch || (f.GateEpoch == f.Epoch && (f.GateState != "BLOCKED" || f.GateRevocation != operation)) ||
		(f.GateEpoch > f.Epoch && (f.State != "SUCCEEDED" || (f.GateState != "OPEN" && f.GateState != "BLOCKED"))) {
		return nil, authorizationUnavailable()
	}
	if f.State == "SUCCEEDED" {
		if f.AckEpoch != f.Epoch || f.AckPhase != "OUTPUT_STOPPED" || !f.AckCompleted || !bytes.Equal(f.AckDigest, platform.BootstrapDigest(request, "document")) ||
			(f.TaskKind != repository.OrganizationCloseRecoveryKind && f.TaskState != "SUCCEEDED") {
			return nil, authorizationUnavailable()
		}
	} else if len(f.AckDigest) != 0 || f.TaskState == "SUCCEEDED" {
		return nil, authorizationUnavailable()
	}
	return request, nil
}

func validAuthorizationTaskState(state string) bool {
	switch state {
	case "QUEUED", "RUNNING", "RETRY", "UNKNOWN", "SUCCEEDED":
		return true
	}
	return false
}

func validAuthorizationOriginal(operation string, f repository.AuthorizationResultFacts) bool {
	if f.TaskKind == domain.AccountCloseFenceKind {
		return validAccountCloseAuthorization(operation, f)
	}
	if f.TaskOperation != operation {
		return false
	}
	if f.TaskKind == repository.OrganizationCloseRecoveryKind {
		original, err := domain.DecodeOrganizationClosePlan(f.Target)
		payload, payloadErr := domain.DecodeOrganizationClosePlan(f.Payload)
		current, currentErr := domain.DecodeOrganizationClosePlan(f.OrganizationPlan)
		b := original.Binding
		return err == nil && payloadErr == nil && currentErr == nil && payload.Binding == b && current.Binding == b && sameAuthorizationJSON(f.Payload, f.Target) &&
			b.Operation == operation && b.User == f.Actor && b.Organization == f.Organization && f.OrganizationActor == f.Actor && b.Scope() == f.Scope && b.Epoch == f.Epoch && f.ReceiptKind == "ORG_CLOSE" &&
			((f.State == "FENCING" && f.OrganizationState == "FENCING") || (f.State == "SUCCEEDED" && (f.OrganizationState == "APPLIED" || f.OrganizationState == "SUCCEEDED")))
	}
	var payload domain.SessionClosure
	decoder := json.NewDecoder(bytes.NewReader(f.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || payload.Scope != f.Scope || payload.Epoch != strconv.FormatUint(f.Epoch, 10) {
		return false
	}
	if f.TaskKind == "SESSION_LOGOUT" {
		var target map[string]string
		return f.Actor > 0 && f.ReceiptKind == "SESSION_LOGOUT" && payload.Kind == "" && payload.UserID == "" && payload.OrganizationID == "" && payload.Role == "" && f.Scope == "session:"+payload.SessionHash &&
			sameAuthorizationJSON(f.Target, f.ReceiptTarget) && json.Unmarshal(f.Target, &target) == nil && len(target) == 2 && target["scope_key"] == f.Scope && target["session_hash"] == payload.SessionHash
	}
	if !domain.ValidEntityID(payload.UserID) || payload.SessionHash != "" {
		return false
	}
	if f.TaskKind == "CREDENTIAL_CHANGE" {
		var target map[string]string
		return f.Actor > 0 && payload.UserID == strconv.FormatUint(f.Actor, 10) && f.Scope == "credential:"+payload.UserID && f.ReceiptKind == payload.Kind &&
			(payload.Kind == "CHANGE_PASSWORD" || payload.Kind == "RESET_PASSWORD" || payload.Kind == "CHANGE_EMAIL") && payload.OrganizationID == "" && payload.Role == "" && sameAuthorizationJSON(f.Target, f.ReceiptTarget) &&
			json.Unmarshal(f.Target, &target) == nil && len(target) == 3 && target["user_id"] == payload.UserID && target["scope_key"] == f.Scope
	}
	if !sameAuthorizationJSON(f.Target, f.Payload) {
		return false
	}
	if f.TaskKind == "PLATFORM_GOVERNANCE_CHANGE" {
		var target GovernanceOperatorInput
		return f.Actor == 0 && payload.Kind == "PLATFORM_GOVERNANCE_REVOKE" && payload.OrganizationID == "" && payload.Role == "" && f.Scope == "platform-admin:"+payload.UserID && f.ReceiptKind == "PLATFORM_GOVERNANCE_OPERATOR" &&
			json.Unmarshal(f.ReceiptTarget, &target) == nil && target.Operation == operation && strconv.FormatUint(target.User, 10) == payload.UserID && target.Action == "REVOKE"
	}
	if f.TaskKind != "ORGANIZATION_CHANGE" || f.Actor == 0 || !domain.ValidEntityID(payload.OrganizationID) {
		return false
	}
	prefix, kind := "org-member:", payload.Kind
	switch payload.Kind {
	case "ORG_ROLE", "ORG_REMOVE", "ORG_LEAVE":
	case "ORG_OWNER_TRANSFER":
		prefix = "org-owner:"
	case "ORG_CREATE_REVOKE":
		prefix = "org-create:"
		kind = "ORG_CREATION_GRANT"
	default:
		return false
	}
	return f.ReceiptKind == kind && f.Scope == prefix+payload.OrganizationID+":"+payload.UserID
}

func validAccountCloseAuthorization(operation string, f repository.AuthorizationResultFacts) bool {
	p, err := domain.DecodeAccountCloseFence(f.Payload)
	c, captureErr := domain.DecodeAccountCloseCapture(f.Capture)
	var target struct {
		Operation string `json:"account_close_operation"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}
	var receipt map[string]string
	if err != nil || captureErr != nil || c.Binding == nil || p.Operation != f.TaskOperation || p.Revocation != operation || p.User != f.Actor || f.CloseUser != f.Actor || p.Scope != f.Scope || p.Epoch != f.Epoch ||
		f.ReceiptKind != "ACCOUNT_CLOSE" || c.Binding.Operation != p.Operation || c.Binding.User != p.User ||
		(f.CloseState != "FROZEN" && f.CloseState != "PLANNED" && f.CloseState != "CLEANING" && f.CloseState != "SUCCEEDED") || (f.State == "FENCING" && f.CloseState != "FROZEN") || json.Unmarshal(f.Target, &target) != nil || target.Operation != p.Operation || target.Scope != p.Scope || target.Epoch != p.Epoch ||
		json.Unmarshal(f.ReceiptTarget, &receipt) != nil || len(receipt) != 2 || receipt["user_id"] != strconv.FormatUint(p.User, 10) || receipt["session_hash"] != c.SessionHash {
		return false
	}
	matched := 0
	for _, scope := range c.Scopes {
		if scope.Scope == p.Scope || scope.Revocation == p.Revocation {
			if scope.Scope != p.Scope || scope.Revocation != p.Revocation || scope.Epoch != p.Epoch {
				return false
			}
			matched++
		}
	}
	return matched == 1
}

// cursor.go：保留原实现，按业务合并文件。

type Cursors struct{ secrets *domain.Secrets }

func NewCursors(secrets *domain.Secrets) *Cursors { return &Cursors{secrets: secrets} }

func (s *Cursors) Decode(token, binding string) (string, bool) {
	if token == "" {
		return "0", true
	}
	if len(token) > 2048 {
		return "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	var cursor domain.CursorPayload
	// 原签名、所属账号/组织及目录修订号一起核验，不能把游标用于另一个权限范围。
	if err != nil || s.secrets.Digest("cursor\n"+parts[0]) != parts[1] || json.Unmarshal(raw, &cursor) != nil || cursor.Binding != binding || cursor.Expires < time.Now().Unix() || !domain.ValidEntityID(cursor.Last) {
		return "", false
	}
	return cursor.Last, true
}

func (s *Cursors) Encode(binding, last string) *string {
	raw, _ := json.Marshal(domain.CursorPayload{Binding: binding, Last: last, Expires: time.Now().Add(5 * time.Minute).Unix()})
	body := base64.RawURLEncoding.EncodeToString(raw)
	token := body + "." + s.secrets.Digest("cursor\n"+body)
	return &token
}

// identity.go：保留原实现，按业务合并文件。

type Identity struct {
	db         *sql.DB
	repository *repository.Identity
}

func NewIdentity(db *sql.DB, repository *repository.Identity) *Identity {
	return &Identity{db: db, repository: repository}
}

type admissionBinding struct {
	Proof []byte `json:"proof"`
}

func identityDenied() error      { return domain.ErrIdentityDenied }
func identityUnavailable() error { return domain.ErrIdentityUnavailable }

func (s *Identity) AcquireIdentityAdmission(ctx context.Context, req *pb.IdentityRequest) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.OperationId) || len(req.SessionToken) != 32 || !domain.ValidIdentityPurpose(req.Purpose) || req.Target == nil || req.Target.Purpose != req.Purpose || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	hash := sha256.Sum256(req.SessionToken)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	// 先只定位actor，真正资格在固定锁顺序user→session→组织→member→gate中核验。
	var actor uint64
	if s.repository.SessionActor(ctx, tx, hash[:], &actor) != nil {
		return nil, identityDenied()
	}
	sources, err := s.admissionSources(ctx, tx, actor, hash[:], req.RequiredIdentityCapability, req.IdentityTarget, req.Purpose)
	if err != nil {
		return nil, err
	}
	if err = s.lockIdentityGates(ctx, tx, sources); err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(30 * time.Second)
	// 准入寿命取实际会话与协议上限的较小值，不能在会话自然到期后继续发出旧资格。
	var sessionExpiry time.Time
	if s.repository.SessionExpiry(ctx, tx, hash[:], &sessionExpiry) != nil {
		return nil, identityUnavailable()
	}
	if sessionExpiry.Before(expires) {
		expires = sessionExpiry
	}
	proof := &pb.IdentityAdmission{
		AdmissionId:    platform.NewTraceID(),
		ActorId:        actor,
		SessionRef:     hash[:],
		Sources:        sources,
		ExpiresUnixMs:  expires.UnixMilli(),
		Purpose:        req.Purpose,
		Target:         req.Target,
		IdentityTarget: req.IdentityTarget,
		Capability:     req.RequiredIdentityCapability,
		OperationId:    req.OperationId,
	}
	raw, _ := proto.Marshal(proof)
	binding, _ := json.Marshal(admissionBinding{Proof: raw})
	if err = s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, actor, hash[:], req.OperationId, req.Purpose, string(binding), expires); err != nil {
		return nil, identityUnavailable()
	}
	for _, source := range sources {
		if err = s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch); err != nil {
			return nil, identityUnavailable()
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	return s.revalidateIdentityAdmission(ctx, req, nil)
}

// DocumentCollaborator只解析Doc已选定的协作者、提及候选或申请人，不提供任意用户目录。
// 原证明绑定真实会话、文档、用途和目标；姓名与重验共用已锁定的账号事务，邮箱不参与查询。
func (s *Identity) DocumentCollaborator(ctx context.Context, req *pb.IdentityRevalidation) (dto.DocumentCollaborator, error) {
	var result dto.DocumentCollaborator
	// 明确限定这三个读取用途，不能让普通活跃账号证明变成任意账号姓名查询。
	if req == nil || (req.Purpose != "DOCUMENT_COLLABORATORS" && req.Purpose != "DOCUMENT_MENTION_CANDIDATES" && req.Purpose != "DOCUMENT_APPLICATION_NAMES") || req.Target == nil || req.Target.Purpose != req.Purpose || req.Target.DocumentId == 0 ||
		req.Target.UnitId != 0 || req.Target.BranchId != 0 || req.Target.StructureId != 0 || req.Target.WorkspaceEpoch != 0 ||
		req.IdentityTarget == nil || req.IdentityTarget.Kind != pb.IdentityTarget_USER || req.IdentityTarget.UserId == 0 || req.IdentityTarget.OrganizationId != 0 || req.IdentityTarget.SpaceId != 0 {
		return result, domain.ErrIdentityBinding
	}
	_, err := s.revalidateIdentityAdmission(ctx, req, func(ctx context.Context, tx *sql.Tx, proof *pb.IdentityAdmission) error {
		if proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT {
			return identityDenied()
		}
		name, err := s.repository.TargetDisplayName(ctx, tx, proof.IdentityTarget.UserId)
		if err != nil || strings.TrimSpace(name) == "" {
			return identityUnavailable()
		}
		result = dto.DocumentCollaborator{UserId: strconv.FormatUint(proof.IdentityTarget.UserId, 10), DisplayName: name}
		return nil
	})
	return result, err
}

func (s *Identity) revalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation, read func(context.Context, *sql.Tx, *pb.IdentityAdmission) error) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.AdmissionId) || req.Target == nil || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	var raw []byte
	var state string
	var expires time.Time
	if s.repository.Admission(ctx, s.db, req.AdmissionId, &raw, &state, &expires) != nil {
		return nil, identityDenied()
	}
	var binding admissionBinding
	proof := &pb.IdentityAdmission{}
	if json.Unmarshal(raw, &binding) != nil || proto.Unmarshal(binding.Proof, proof) != nil || state != "OPEN" || !expires.After(time.Now()) || proof.OperationId != req.OperationId || proof.Purpose != req.Purpose || !proto.Equal(proof.Target, req.Target) || !proto.Equal(proof.IdentityTarget, req.IdentityTarget) {
		return nil, identityDenied()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	current, err := s.admissionSources(ctx, tx, proof.ActorId, proof.SessionRef, proof.Capability, proof.IdentityTarget, proof.Purpose)
	if err != nil {
		return nil, err
	}
	if !sameSources(proof.Sources, current) {
		return nil, identityDenied()
	}
	if err = s.lockIdentityGates(ctx, tx, current); err != nil {
		return nil, err
	}
	// 关闭事务也先锁相同权威行；此处锁admission防止查询后被并发关闭。
	if s.repository.LockAdmission(ctx, tx, req.AdmissionId, &state, &expires) != nil || state != "OPEN" || !expires.After(time.Now()) {
		return nil, identityDenied()
	}
	if read != nil {
		if err = read(ctx, tx, proof); err != nil {
			return nil, err
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
func (s *Identity) currentSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget) ([]*pb.SourceEpoch, error) {
	var state string
	var credential, identity uint64
	if s.repository.LockUser(ctx, tx, actor, &state, &credential, &identity) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	var sessionActor, sessionCredential, sessionEpoch uint64
	var expiry time.Time
	if s.repository.LockSession(ctx, tx, session, &sessionActor, &state, &sessionCredential, &sessionEpoch, &expiry) != nil || sessionActor != actor || state != "ACTIVE" || sessionCredential != credential || !expiry.After(time.Now()) {
		return nil, identityDenied()
	}
	user := strconv.FormatUint(actor, 10)
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:" + user, Epoch: identity}, {ScopeKey: "credential:" + user, Epoch: credential}, {ScopeKey: "session:" + hex.EncodeToString(session), Epoch: sessionEpoch}}
	if capability == pb.IdentityCapability_ACTIVE_ACCOUNT || capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER {
		if target.Kind != pb.IdentityTarget_USER || target.UserId == 0 || target.OrganizationId != 0 || target.SpaceId != 0 || (capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER && target.UserId != actor) {
			return nil, identityDenied()
		}
		// 文档管理者的个人协作授权需要核验实际目标账号；这里只证明活跃资格，
		// 文档管理权限仍由文档服务负责。证明携带目标身份来源，关闭账号会使它失效。
		if target.UserId != actor {
			var targetEpoch uint64
			if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		}
		return sources, nil
	}
	if target.Kind != pb.IdentityTarget_ORGANIZATION || target.OrganizationId == 0 || target.SpaceId != 0 {
		return nil, identityDenied()
	}
	var targetIdentityEpoch uint64
	if target.UserId != 0 && target.UserId != actor {
		if capability != pb.IdentityCapability_ORGANIZATION_ADMIN && capability != pb.IdentityCapability_ORGANIZATION_OWNER && capability != pb.IdentityCapability_ORGANIZATION_MEMBER {
			return nil, identityDenied()
		}
		// 目标user先于组织锁，与注销固定组织集合使用同一顺序，
		// 避免目标注销持user等组织、目标证明持组织等user的反向等待。
		if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetIdentityEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
	}
	org := strconv.FormatUint(target.OrganizationId, 10)
	var owner uint64
	if s.repository.LockOrganization(ctx, tx, target.OrganizationId, &owner, &state) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	var role string
	var memberEpoch uint64
	if s.repository.LockMember(ctx, tx, target.OrganizationId, actor, &role, &state, &memberEpoch) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + user, Epoch: memberEpoch})
	switch capability {
	case pb.IdentityCapability_ORGANIZATION_MEMBER:
	case pb.IdentityCapability_ORGANIZATION_ADMIN:
		if owner != actor && role != "ADMIN" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_OWNER:
		if owner != actor {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-owner:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE:
		if owner == actor || role == "ADMIN" {
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
		} else {
			var grantState string
			var revision uint64
			if s.repository.LockCreationGrant(ctx, tx, target.OrganizationId, actor, &grantState, &revision) != nil || grantState != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-create:" + org + ":" + user, Epoch: revision})
		}
	default:
		return nil, identityDenied()
	}
	// 任免目标也必须是当前成员；目标来源写入依赖，防止检查后被移除。
	if target.UserId != 0 && target.UserId != actor {
		var targetEpoch uint64
		if s.repository.LockTargetMember(ctx, tx, target.OrganizationId, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		// 目标必须活跃本身也是权限依赖；否则注销预留封住identity后，
		// 另一管理员仍可能凭旧目标证明把新的KB职责授给待注销账号。
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetIdentityEpoch})
	}
	return sources, nil
}
func (s *Identity) lockIdentityGates(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error {
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	for _, source := range sources {
		if err := s.repository.EnsureGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
			return identityUnavailable()
		}
		var epoch uint64
		var state string
		if s.repository.LockGate(ctx, tx, source.ScopeKey, &epoch, &state) != nil {
			return identityUnavailable()
		}
		if source.Epoch < epoch || (source.Epoch == epoch && state != "OPEN") {
			return identityDenied()
		}
		// 只有重新核验权威来源后才可导入更高代次；旧关闭水位留存，不靠TTL删除。
		if source.Epoch > epoch {
			if err := s.repository.AdvanceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return identityUnavailable()
			}
		}
	}
	return nil
}
func sameSources(a, b []*pb.SourceEpoch) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i].ScopeKey < a[j].ScopeKey })
	sort.Slice(b, func(i, j int) bool { return b[i].ScopeKey < b[j].ScopeKey })
	for i := range a {
		if a[i].ScopeKey != b[i].ScopeKey || a[i].Epoch != b[i].Epoch {
			return false
		}
	}
	return true
}

// identity_renewal.go：保留原实现，按业务合并文件。

// RenewOperationAdmission只用于白名单内已登记操作的后台续接。它签发新准入，绝不让旧过期证明重新有效。
// 调用方的document服务身份由独立HTTP入口核验；真实会话和原来源仍在本事务中重查。
func (s *Identity) RenewOperationAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.AdmissionId) || !domain.ValidOperationID(req.OperationId) || req.Target == nil || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	switch req.Purpose {
	case "SUBMISSION", "MERGE", "MAIN_CONTROL", "PUBLICATION", "PULL_MAIN", "STRUCTURE_PREVIEW", "DISCARD_PREVIEW", "DISCARD_DRAFT", "REVERSE", "CONFLICT_RESOLUTION":
		// 人工修复同样依赖原号证明续期；下方仍逐项核验原会话、目标和来源，不能借续期换权限。
	default:
		return nil, identityDenied()
	}
	if req.Target.Purpose != req.Purpose {
		return nil, identityDenied()
	}
	var raw []byte
	var state string
	var oldExpiry time.Time
	if s.repository.Admission(ctx, s.db, req.AdmissionId, &raw, &state, &oldExpiry) != nil {
		return nil, identityDenied()
	}
	var binding admissionBinding
	original := &pb.IdentityAdmission{}
	if json.Unmarshal(raw, &binding) != nil || proto.Unmarshal(binding.Proof, original) != nil || state != "OPEN" ||
		original.OperationId != req.OperationId || original.Purpose != req.Purpose ||
		!proto.Equal(original.Target, req.Target) || !proto.Equal(original.IdentityTarget, req.IdentityTarget) || len(original.SessionRef) != 32 {
		return nil, identityDenied()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	// 与登出/凭据/角色变化共用user→session→组织→成员→gate锁顺序，不能把旧来源换成新权限。
	current, err := s.admissionSources(ctx, tx, original.ActorId, original.SessionRef, original.Capability, original.IdentityTarget, original.Purpose)
	if err != nil {
		return nil, err
	}
	if !sameSources(original.Sources, current) {
		return nil, identityDenied()
	}
	if err = s.lockIdentityGates(ctx, tx, current); err != nil {
		return nil, err
	}
	if s.repository.LockAdmission(ctx, tx, req.AdmissionId, &state, &oldExpiry) != nil || state != "OPEN" {
		return nil, identityDenied()
	}
	var sessionExpiry time.Time
	if s.repository.SessionExpiry(ctx, tx, original.SessionRef, &sessionExpiry) != nil {
		return nil, identityUnavailable()
	}
	expires := time.Now().UTC().Add(30 * time.Second)
	if sessionExpiry.Before(expires) {
		expires = sessionExpiry
	}
	proof := proto.Clone(original).(*pb.IdentityAdmission)
	proof.AdmissionId, proof.ExpiresUnixMs = platform.NewTraceID(), expires.UnixMilli()
	proof.Sources = current
	encoded, err := proto.Marshal(proof)
	if err != nil {
		return nil, identityUnavailable()
	}
	newBinding, err := json.Marshal(admissionBinding{Proof: encoded})
	if err != nil {
		return nil, identityUnavailable()
	}
	if s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, proof.ActorId, proof.SessionRef, proof.OperationId, proof.Purpose, string(newBinding), expires) != nil {
		return nil, identityUnavailable()
	}
	for _, source := range current {
		if s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch) != nil {
			return nil, identityUnavailable()
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}

// mutation.go：保留原实现，按业务合并文件。

// MutationBody使用同一私有库事务，不能另开池。成员离开等明确需要跨服务互斥的
// 用例仍在来源锁内调用有deadline的内部检查；调用本身不能代替本地持久关闭事实。
type MutationBody func(context.Context, *sql.Tx, domain.Session) (map[string]any, error)

type Mutations struct {
	db         *sql.DB
	repository *repository.Mutation
}

func NewMutations(db *sql.DB, repo *repository.Mutation) *Mutations {
	return &Mutations{db: db, repository: repo}
}

func (s *Mutations) CheckSession(ctx context.Context, tx *sql.Tx, session domain.Session) error {
	user, err := s.repository.LockUser(ctx, tx, session.UserID)
	if err != nil || user.State != "ACTIVE" || user.Epoch != session.CredentialEpoch {
		return domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	current, err := s.repository.LockSession(ctx, tx, session.ID)
	if err != nil || current.State != "ACTIVE" || !current.Expires.After(time.Now()) {
		return domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	return nil
}

func (s *Mutations) Run(ctx context.Context, input domain.Mutation, body MutationBody) (json.RawMessage, error) {
	if !domain.ValidOperationID(input.OperationID) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 原user→session→receipt锁顺序和提交边界不变；同号重试也核验当前会话。
	if err := s.CheckSession(ctx, tx, input.Session); err != nil {
		return nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, input.OperationID)
	if err == nil {
		if receipt.ActorID != input.Session.UserID || receipt.Kind != input.Kind || !bytes.Equal(receipt.Fingerprint, input.Fingerprint) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		if len(receipt.Result) == 0 {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "OPERATION_PENDING"}
		}
		return receipt.Result, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	value, err := body(ctx, tx, input.Session)
	if err != nil {
		return nil, err
	}
	result, err := platform.EncodeOperationReceipt(input.OperationID, value)
	if err != nil {
		return nil, err
	}
	target, err := json.Marshal(map[string]string{"path": input.Path})
	if err != nil {
		return nil, err
	}
	if s.repository.InsertReceipt(ctx, tx, input, result.State, result.JSON, target) != nil {
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	if err := s.repository.InsertAudit(ctx, tx, input, target); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	return result.JSON, nil
}
````

UTF-8/LF全文SHA-256：`7f6ba8098475dc8e09aeb8ef6ad6015455b36edf3c117764157e6a3a4dea01b0`。[参考实现](../../../../services/account/internal/logic/account/identity_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
