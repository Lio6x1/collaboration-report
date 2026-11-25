# services/document/internal/logic/document/authorization_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/194-authorization-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Access`、`NewAccess`、`DocumentAccess`、`Authorize`、`ValidateAccess`、`SpaceIdentity`、`Authorizations`、`NewAuthorizations`、`QueueClosure`、`AuthorizationChanges`、`NewAuthorizationChanges`、`Prepare`、`RecoverOne`、`retry`、`Presentation`、`Progress`、`DepartureAuthority`、`Departures`、`NewDepartures`、`Eligibility`、`Close`、`Recover`、`AcquireGovernance`、`Actor`、`SessionVerifier`、`AdmissionRenewer`、`Identity`、`NewIdentity`、`Authenticate`、`Acquire`、`BindProofs`、`AcquireBound`、`Revalidate`、`MergeSources`、`Target`、`Resume`、`Transfer`、`LockSources`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// access.go：保留原实现，按业务合并文件。

type Access struct {
	repository *repository.Access
	identity   *Identity
}

func NewAccess(repository *repository.Access, identity *Identity) *Access {
	return &Access{repository: repository, identity: identity}
}

type DocumentAccess struct {
	Document    dto.Document
	Permissions domain.Permissions
	Sources     []*pb.SourceEpoch
	Proofs      []*pb.IdentityAdmission
	KBRevision  uint64
}

// Authorize先核验当前来源；业务事务内仍须持锁复核，不能把事务外快照当作最终准入。
func (s *Access) Authorize(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error) {
	var access DocumentAccess
	document, err := s.repository.Document(ctx, id)
	if err != nil {
		return access, err
	}
	access.Document = document
	spaceID, _ := domain.CanonicalID(document.SpaceId)
	space, err := s.repository.Space(ctx, spaceID)
	if err != nil {
		return access, err
	}
	facts := domain.PermissionFacts{Actor: actor.ID, Responsible: document.ResponsibleUserId != nil && *document.ResponsibleUserId == domain.TextID(actor.ID), PersonalOwner: space.OwnerUserId != nil && *space.OwnerUserId == domain.TextID(actor.ID)}
	proofs := []*pb.IdentityAdmission{actor.Proof}
	if space.Kind == "ORGANIZATION" {
		_, proof, err := s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)
		if err == nil {
			facts.OrganizationAdmin = true
			proofs = append(proofs, proof)
		} else {
			var business domain.RuleViolation
			if !errors.As(err, &business) || business.Kind != domain.PermissionDenied {
				return access, err
			}
			_, proof, err = s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)
			if err != nil {
				return access, err
			}
			proofs = append(proofs, proof)
		}
	}
	if document.KnowledgeBaseId != nil {
		kb, _ := domain.CanonicalID(*document.KnowledgeBaseId)

		state, revision, err := s.repository.KnowledgeBase(ctx, kb, spaceID)
		access.KBRevision = revision
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return access, err
		}
		if errors.Is(err, sql.ErrNoRows) || state != "ACTIVE" {
			return access, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
		}
		if role, err := s.repository.KnowledgeBaseRole(ctx, kb, actor.ID); err == nil {
			facts.KnowledgeBaseAdmin = role == "ADMIN"
		} else if !errors.Is(err, sql.ErrNoRows) {
			// 没有KB角色是确定资格不足；数据库未知不能当作“普通成员”继续授权。
			return access, err
		}
	}
	facts.Grants, err = s.repository.Grants(ctx, id, actor.ID)
	if err != nil {
		return access, err
	}
	facts.Units, err = s.repository.Units(ctx, document.CurrentStructureId, id)
	if err != nil {
		return access, err
	}
	access.Permissions = domain.ResolvePermissions(facts)
	if !access.Permissions.Read {
		return access, domain.Violation(domain.PermissionDenied, "DOCUMENT_READ_NOT_ALLOWED")
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return access, err
		}
	}
	access.Proofs = proofs
	access.Sources, err = MergeSources(proofs...)
	if err != nil {
		return access, err
	}
	revision, _ := domain.CanonicalID(document.AclRevision)
	access.Sources = append(access.Sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(id), Epoch: revision})
	if document.KnowledgeBaseId != nil {
		access.Sources = append(access.Sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *document.KnowledgeBaseId, Epoch: access.KBRevision})
	}
	return access, nil
}
func ValidateAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess) (dto.Document, error) {
	id, _ := domain.CanonicalID(access.Document.Id)
	current, err := repository.DocumentRow(ctx, tx, id, true)
	if err != nil {
		return current, err
	}
	if current.AclRevision != access.Document.AclRevision || current.CurrentStructureId != access.Document.CurrentStructureId || current.ResponsibleUserId == nil != (access.Document.ResponsibleUserId == nil) {
		return current, domain.Violation(domain.StateConflict, "DOCUMENT_ACCESS_CHANGED")
	}
	if current.KnowledgeBaseId != nil {
		state, revision, err := repository.LockedKnowledgeBase(ctx, tx, *current.KnowledgeBaseId)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			// SQL未知与已确认资格变化不同；不能把超时/断连改成可重试的页面冲突。
			return current, err
		}
		if errors.Is(err, sql.ErrNoRows) || state != "ACTIVE" || revision != access.KBRevision {
			return current, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
	}
	return current, nil
}
func (s *Access) SpaceIdentity(ctx context.Context, actor Actor, spaceID uint64, capability pb.IdentityCapability, purpose string, document uint64) (dto.Space, *pb.IdentityAdmission, error) {
	space, err := s.repository.Space(ctx, spaceID)
	if err != nil {
		return space, nil, err
	}
	if space.Kind == "PERSONAL" {
		if space.OwnerUserId == nil || *space.OwnerUserId != domain.TextID(actor.ID) {
			return space, nil, domain.Violation(domain.PermissionDenied, "PERSONAL_SPACE_NOT_OWNED")
		}
		proof, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)
		return space, proof, err
	}
	org, _ := domain.CanonicalID(*space.OrganizationId)
	proof, err := s.identity.Acquire(ctx, actor, capability, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)
	return space, proof, err
}

// authorization.go：保留原实现，按业务合并文件。

type Authorizations struct {
	db         *sql.DB
	repository *repository.Authorizations
	fences     *Fences
	outputs    *Outputs
}

func NewAuthorizations(db *sql.DB, repository *repository.Authorizations, fences *Fences, outputs *Outputs) *Authorizations {
	return &Authorizations{db: db, repository: repository, fences: fences, outputs: outputs}
}

// QueueClosure与业务事实使用同一事务：先固定关闭计划，再冻结旧来源并登记可恢复任务。
func QueueClosure(ctx context.Context, tx *sql.Tx, operation string, actor uint64, closure domain.Closure) error {
	digest := platform.SourceClosureDigest(closure.Scope, closure.Epoch)
	binding, _ := json.Marshal(map[string]string{"bootstrap_target_digest": hex.EncodeToString(digest), "generation": "1"})
	if err := repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding); err != nil {
		return err
	}
	if err := repository.BlockFenceScope(ctx, tx, closure.Scope, closure.Epoch, operation); err != nil {
		return err
	}
	payload, _ := json.Marshal(closure)
	return repository.InsertAuthorizationTask(ctx, tx, operation, payload)
}

// authorization_change.go：保留原实现，按业务合并文件。

type AuthorizationChanges struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

func NewAuthorizationChanges(db *sql.DB, identity *Identity, access *Access) *AuthorizationChanges {
	return &AuthorizationChanges{db: db, identity: identity, access: access}
}

// Prepare绑定实际文档及每个受影响身份；最终写入仍在共享变更事务中持锁重核。
func (s *AuthorizationChanges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.AuthorizationChange) (Actor, MutationBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "CHANGE_AUTHORIZATION")
	if err != nil {
		return actor, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	actor.Sources = access.Sources
	var targets []*pb.IdentityAdmission
	for _, change := range input.Changes {
		user, err := domain.CanonicalID(change.Grant.UserId)
		if err != nil {
			return actor, nil, err
		}
		spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
		space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
		if err != nil {
			return actor, nil, err
		}
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}
		capability := pb.IdentityCapability_ACTIVE_ACCOUNT
		if space.Kind == "ORGANIZATION" {
			organization, _ := domain.CanonicalID(*space.OrganizationId)
			target = &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}
			capability = pb.IdentityCapability_ORGANIZATION_MEMBER
		}
		// 不能借用另一组织的member证明，目标身份必须绑定当前文档所属空间。
		proof, err := s.identity.Target(ctx, actor, capability, target, "CHANGE_AUTHORIZATION", id)
		if err != nil {
			return actor, nil, err
		}
		targets = append(targets, proof)
	}
	for _, proof := range targets {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return actor, nil, err
		}
	}
	sources, err := MergeSources(append(access.Proofs, targets...)...)
	if err != nil {
		return actor, nil, err
	}
	baseSources, _ := MergeSources(access.Proofs...)
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.AclRevision != input.ExpectedAclRevision {
			return nil, domain.Violation(domain.StateConflict, "ACL_REVISION_MISMATCH")
		}
		for _, change := range input.Changes {
			grant := change.Grant
			user, _ := domain.CanonicalID(grant.UserId)
			scope, err := domain.CanonicalID(grant.ScopeId)
			if err != nil {
				return nil, err
			}
			if change.Action != "GRANT" && change.Action != "REVOKE" {
				return nil, domain.Violation(domain.InvalidRequest, "GRANT_ACTION_INVALID")
			}
			if grant.Capability == "EDIT" {
				if grant.ScopeKind != "UNIT" && grant.ScopeKind != "CHAPTER" {
					return nil, domain.Violation(domain.InvalidRequest, "EDIT_SCOPE_INVALID")
				}
				if repository.EditGrantScope(ctx, tx, grant.ScopeKind, scope, id) != nil {
					return nil, domain.Violation(domain.InvalidRequest, "GRANT_SCOPE_NOT_IN_DOCUMENT")
				}
			} else if (grant.Capability != "MANAGE" && grant.Capability != "REVIEW" && grant.Capability != "COMMENT") || grant.ScopeKind != "DOCUMENT" || scope != id {
				return nil, domain.Violation(domain.InvalidRequest, "DOCUMENT_CAPABILITY_SCOPE_INVALID")
			}
			state := "ACTIVE"
			if change.Action == "REVOKE" {
				state = "CLOSED"
			}
			fact := domain.GrantFact{User: user, Kind: grant.ScopeKind, Scope: scope, Capability: grant.Capability}
			if err = repository.SetContentGrant(ctx, tx, id, fact, state); err != nil {
				return nil, err
			}
		}
		if err = repository.AdvanceDocumentACL(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// authorization_recovery.go：保留原实现，按业务合并文件。

// RecoverOne先持久领取，再执行跨服务关闭；未知ACK只能以原编号重试，不能落业务终态。
func (s *Authorizations) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimAuthorization(ctx, tx)
	if err != nil {
		return
	}
	instance := s.outputs.Self().ID
	if repository.RunAuthorizationTask(ctx, tx, task.ID, instance) != nil {
		return
	}
	if tx.Commit() != nil {
		return
	}
	var closure domain.Closure
	if json.Unmarshal(task.Payload, &closure) != nil || !platform.ValidDocumentScope(closure.Scope) {
		s.retry(parent, task.ID, instance)
		return
	}
	request := &pb.FenceRequest{RevocationId: task.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: closure.Scope, Epoch: closure.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(closure.Scope, closure.Epoch)}
	ack, err := s.fences.Close(ctx, request)
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		s.retry(parent, task.ID, instance)
		return
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	owner, err := repository.LockedTaskOwnership(ctx, tx, task.ID)
	if err != nil || owner.Instance != instance || owner.State != "RUNNING" {
		return
	}
	// 实例租约只允许领取任务；业务终态仍要求当前任务所有者和真实关闭ACK。
	if closure.Action == "TRASH_DOCUMENT" {
		err = repository.TrashClosingDocument(ctx, tx, closure.Entity)
	} else if closure.Action == "CLOSE_KB" {
		err = repository.TrashClosingKnowledgeBaseDocuments(ctx, tx, closure.Entity)
		if err == nil {
			err = repository.CloseKnowledgeBase(ctx, tx, closure.Entity)
		}
	}
	if err == nil {
		err = repository.CompleteAuthorizationReceipt(ctx, tx, task.Operation)
	}
	if err == nil {
		err = repository.CompleteAuthorizationTask(ctx, tx, task.ID, instance)
	}
	if err == nil {
		_ = tx.Commit()
	}
}
func (s *Authorizations) retry(parent context.Context, id uint64, instance string) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	s.repository.Retry(ctx, id, instance)
}

// authorization_status.go：保留原实现，按业务合并文件。

func (s *Authorizations) Presentation(actor Actor) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, operation map[string]any) (any, []*pb.SourceEpoch, error) {
		id, ok := operation["operation_id"].(string)
		if !ok {
			return nil, nil, errors.New("AUTHORIZATION_RESULT_UNAVAILABLE")
		}
		facts, err := repository.AuthorizationProjection(ctx, tx, id, actor.ID)
		if err != nil {
			return nil, nil, err
		}
		value := dto.AuthorizationResult{OperationId: id, State: facts.State, ScopeKey: facts.Scope, Epoch: facts.Epoch, MissingParticipants: []string{}}
		if value.State == "SUCCEEDED" && operation["state"] != "SUCCEEDED" {
			value.State = "FENCING"
		}
		for _, participant := range []string{"edge", "collaboration"} {
			count, err := repository.ProjectionAcknowledgment(ctx, tx, id, participant)
			if err != nil {
				return nil, nil, err
			}
			if count == 0 {
				value.MissingParticipants = append(value.MissingParticipants, participant)
			}
		}
		return value, actor.Proof.Sources, nil
	}
}
func (s *Authorizations) Progress(actor Actor, operation string) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		facts, err := repository.AuthorizationProgress(ctx, tx, operation, actor.ID)
		if err != nil {
			return nil, domain.Violation(domain.EntityNotFound, "OPERATION_NOT_FOUND")
		}
		value := dto.AuthorizationResult{OperationId: operation, State: facts.State, ScopeKey: facts.Scope, Epoch: facts.Epoch, MissingParticipants: []string{}}
		// 输出ACK只证明旧输出停止；最终业务事务尚未提交时仍对外显示FENCING。
		if value.State == "SUCCEEDED" && facts.ReceiptState != "SUCCEEDED" {
			value.State = "FENCING"
		}
		for _, participant := range []string{"edge", "collaboration"} {
			count, err := repository.ProgressAcknowledgment(ctx, tx, operation, participant, value.ScopeKey, value.Epoch)
			if err != nil {
				return nil, errors.New("AUTHORIZATION_PROGRESS_UNAVAILABLE")
			}
			if count == 0 {
				value.MissingParticipants = append(value.MissingParticipants, participant)
			}
		}
		return value, nil
	}
}

// departure.go：保留原实现，按业务合并文件。

type DepartureAuthority interface {
	RolledBack(context.Context, domain.DepartureObservation) (bool, error)
}
type Departures struct {
	db         *sql.DB
	repository *repository.Departures
	authority  DepartureAuthority
}

func NewDepartures(db *sql.DB, repository *repository.Departures, authority DepartureAuthority) *Departures {
	return &Departures{db: db, repository: repository, authority: authority}
}
func (s *Departures) Eligibility(ctx context.Context, binding domain.DepartureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// 任命/建库也先锁相同成员来源；取得门闩之后才能判断已提交的管理员事实。
	if err = repository.EnsureDepartureGate(ctx, tx, binding); err != nil {
		return false, err
	}
	gate, err := repository.LockedDepartureGate(ctx, tx, binding.Scope())
	if err != nil {
		return false, err
	}
	if gate.Epoch > binding.Epoch || (gate.Epoch == binding.Epoch && gate.State == "BLOCKED" && gate.Operation != binding.ID) {
		return false, domain.Violation(domain.StateConflict, "DEPARTURE_SOURCE_CHANGED")
	}
	admin, err := repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)
	if err != nil {
		return false, err
	}
	if !admin {
		err = repository.BlockMemberDeparture(ctx, tx, binding)
	}
	if err != nil {
		return false, err
	}
	if tx.Commit() != nil {
		return false, domain.Violation(domain.TemporarilyUnavailable, "DEPARTURE_PREPARE_UNKNOWN")
	}
	return admin, nil
}
func (s *Departures) Close(ctx context.Context, binding domain.DepartureBinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gate, err := repository.LockedDepartureGate(ctx, tx, binding.Scope())
	if err != nil || gate.Epoch != binding.Epoch || gate.State != "BLOCKED" || gate.Operation != binding.ID {
		return domain.Violation(domain.StateConflict, "DEPARTURE_PREPARE_REQUIRED")
	}
	admin, err := repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)
	if err != nil || admin {
		return domain.Violation(domain.StateConflict, "ADMIN_RETIREMENT_REQUIRED")
	}
	// 只关闭指定组织的旧成员事实；来源冻结阻止迟到授权，个人空间协作继续独立存在。
	if err = repository.QueueDepartureNotifications(ctx, tx, binding); err != nil {
		return err
	}
	if err = repository.CloseOrganizationMemberFacts(ctx, tx, binding.Org, binding.User); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "DEPARTURE_CLOSE_UNKNOWN")
	}
	return nil
}
func (s *Departures) Recover(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 1500*time.Millisecond)
	defer cancel()
	observed, err := s.repository.Pending(ctx)
	if err != nil {
		return
	}
	rolledBack, err := s.authority.RolledBack(ctx, observed)
	if err != nil || !rolledBack {
		return
	}
	// 原编号/代次/冻结状态仍一致才恢复；不以租约到期或普通SELECT缺行推断回滚。
	s.repository.Reopen(ctx, observed)
}

// governance_authority.go：保留原实现，按业务合并文件。

// 普通登录仅建立actor；治理必须另外取得Account专用purpose的真实平台角色准入。
func (s *Identity) AcquireGovernance(ctx context.Context, actor Actor, operation string, document uint64) (*pb.IdentityAdmission, error) {
	return s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT,
		&pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation,
		&pb.Target{DocumentId: document, Purpose: "RETAINED_GOVERNANCE"})
}

// identity.go：保留原实现，按业务合并文件。

// Actor保存已核验身份与跨服务证明；请求对象不进入用例。
type Actor struct {
	ID                   uint64
	Token                []byte
	Proof                *pb.IdentityAdmission
	Sources              []*pb.SourceEpoch
	SessionExpiresUnixMs int64
}
type SessionVerifier interface {
	VerifySession(context.Context, domain.SessionVerification) (domain.VerifiedSession, error)
}
type AdmissionRenewer interface {
	RenewAdmission(context.Context, *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)
}
type Identity struct {
	identity pb.IdentityInternalClient
	verifier SessionVerifier
	renewer  AdmissionRenewer
}

func NewIdentity(peer pb.IdentityInternalClient, verifier SessionVerifier, renewer AdmissionRenewer) *Identity {
	return &Identity{identity: peer, verifier: verifier, renewer: renewer}
}
func (s *Identity) Authenticate(ctx context.Context, token []byte, request domain.SessionVerification, purpose string, document uint64) (Actor, error) {
	actor := Actor{Token: token}
	session, err := s.verifier.VerifySession(ctx, request)
	if err != nil {
		return actor, err
	}
	actor.ID = session.ActorID
	if session.ExpiresUnixMs <= time.Now().UnixMilli() {
		return actor, domain.Violation(domain.SessionRequired, "SESSION_EXPIRED")
	}
	actor.SessionExpiresUnixMs = session.ExpiresUnixMs
	actor.Proof, err = s.Acquire(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)
	return actor, err
}

func (s *Identity) Acquire(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document, unit uint64) (*pb.IdentityAdmission, error) {
	return s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})
}

// 后台操作保存的证明必须绑定业务原号和完整工作区；随机请求号只适用于普通短期入口。
// BindProofs 把已鉴权证明绑定到本次操作，来源集合必须仍与原证明一致。
// 空集合的业务含义由调用方决定，不能在公共机制里改变各入口的资格检查顺序。
func (s *Identity) BindProofs(ctx context.Context, actor Actor, originals []*pb.IdentityAdmission, operation string, target *pb.Target) ([]*pb.IdentityAdmission, error) {
	bound := make([]*pb.IdentityAdmission, 0, len(originals))
	for _, original := range originals {
		if original == nil {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		proof, err := s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)
		if err != nil {
			return nil, err
		}
		if !proto.Equal(&pb.OperationContext{Sources: original.Sources}, &pb.OperationContext{Sources: proof.Sources}) {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		bound = append(bound, proof)
	}
	return bound, nil
}

func (s *Identity) AcquireBound(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, operation string, businessTarget *pb.Target) (*pb.IdentityAdmission, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	request := &pb.IdentityRequest{OperationId: operation, SessionToken: actor.Token, Purpose: businessTarget.Purpose, Target: businessTarget, RequiredIdentityCapability: capability, IdentityTarget: target}
	proof, err := s.identity.AcquireIdentityAdmission(ctx, request)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
		}
		return nil, errors.New("IDENTITY_UNAVAILABLE")
	}
	if proof == nil || proof.ActorId != actor.ID || proof.OperationId != request.OperationId || proof.Purpose != request.Purpose || proof.Capability != capability || !proto.Equal(proof.Target, request.Target) || !proto.Equal(proof.IdentityTarget, target) || proof.ExpiresUnixMs <= time.Now().UnixMilli() || len(proof.SessionRef) != 32 {
		return nil, errors.New("IDENTITY_PROOF_INVALID")
	}
	return proof, nil
}
func (s *Identity) Revalidate(ctx context.Context, proof *pb.IdentityAdmission) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	current, err := s.identity.RevalidateIdentityAdmission(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
		}
		return errors.New("IDENTITY_UNAVAILABLE")
	}
	if !proto.Equal(proof, current) {
		return errors.New("IDENTITY_PROOF_CHANGED")
	}
	return nil
}
func MergeSources(proofs ...*pb.IdentityAdmission) ([]*pb.SourceEpoch, error) {
	current := map[string]uint64{}
	var result []*pb.SourceEpoch
	for _, p := range proofs {
		if p == nil {
			continue
		}
		for _, source := range p.Sources {
			if source == nil || source.Epoch == 0 {
				return nil, errors.New("IDENTITY_SOURCES_INVALID")
			}
			if epoch, exists := current[source.ScopeKey]; exists {
				if epoch != source.Epoch {
					return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
				}
				continue
			}
			current[source.ScopeKey] = source.Epoch
			result = append(result, source)
		}
	}
	return result, nil
}
func (s *Identity) Target(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document uint64) (*pb.IdentityAdmission, error) {
	return s.Acquire(ctx, actor, capability, target, purpose, document, 0)
}

// Resume只供明确列出的已登记操作续接；新短期证明必须保留原actor/会话/目标/来源，不能换用后来取得的权限。
func (s *Identity) Resume(ctx context.Context, original *pb.IdentityAdmission) (*pb.IdentityAdmission, error) {
	if original == nil || s.renewer == nil {
		return nil, errors.New("IDENTITY_RESUME_BINDING_INVALID")
	}
	switch original.Purpose {
	case "SUBMISSION", "MERGE", "MAIN_CONTROL", "PUBLICATION", "PULL_MAIN", "STRUCTURE_PREVIEW", "DISCARD_PREVIEW", "DISCARD_DRAFT", "REVERSE", "CONFLICT_RESOLUTION":
	default:
		return nil, errors.New("IDENTITY_RESUME_BINDING_INVALID")
	}
	if original.ExpiresUnixMs > time.Now().Add(2*time.Second).UnixMilli() {
		if err := s.Revalidate(ctx, original); err != nil {
			return nil, err
		}
		return original, nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	current, err := s.renewer.RenewAdmission(ctx, &pb.IdentityRevalidation{AdmissionId: original.AdmissionId, OperationId: original.OperationId, Purpose: original.Purpose, Target: original.Target, IdentityTarget: original.IdentityTarget})
	if err != nil {
		return nil, err
	}
	if current == nil || current.AdmissionId == original.AdmissionId || current.ExpiresUnixMs <= time.Now().UnixMilli() {
		return nil, errors.New("IDENTITY_RESUME_PROOF_INVALID")
	}
	expected := proto.Clone(original).(*pb.IdentityAdmission)
	expected.AdmissionId, expected.ExpiresUnixMs = current.AdmissionId, current.ExpiresUnixMs
	if !proto.Equal(expected, current) {
		return nil, errors.New("IDENTITY_RESUME_PROOF_CHANGED")
	}
	return current, nil
}

// responsibility.go：保留原实现，按业务合并文件。

func (s *Management) Transfer(ctx context.Context, actor Actor, id, user uint64, input dto.OwnerChange) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "TRANSFER_RESPONSIBLE")
	if err != nil {
		return actor, nil, err
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return actor, nil, err
	}
	if space.Kind != "ORGANIZATION" {
		return actor, nil, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_REQUIRES_ACCEPTANCE")
	}
	organization, _ := domain.CanonicalID(*space.OrganizationId)
	proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, "TRANSFER_RESPONSIBLE", id)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	sources, err := MergeSources(append(access.Proofs, proof)...)
	if err != nil {
		return actor, nil, err
	}
	baseSources, _ := MergeSources(access.Proofs...)
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if document.ResponsibleUserId != nil && *document.ResponsibleUserId == input.TargetUserId {
			return OperationResult(input.OperationId, "SUCCEEDED", id), nil
		}
		if err = repository.TransferDocumentResponsibility(ctx, tx, id, user); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "RESPONSIBLE_TRANSFER"}); err != nil {
			return nil, err
		}
		target, _ := json.Marshal(map[string]string{"document_id": document.Id})
		if err = repository.NotifyResponsibilityTransfer(ctx, tx, user, input.OperationId, target); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// source_gate.go：保留原实现，按业务合并文件。

// 来源只来自当前身份证明或已锁定ACL；同代BLOCKED拒绝，只有更高已核验代次才能重开。
func LockSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error {
	list := append([]*pb.SourceEpoch(nil), sources...)
	sort.Slice(list, func(i, j int) bool { return list[i].ScopeKey < list[j].ScopeKey })
	seen := map[string]bool{}
	for _, source := range list {
		if source == nil || source.Epoch == 0 || seen[source.ScopeKey] {
			return errors.New("SOURCE_BINDING_INVALID")
		}
		seen[source.ScopeKey] = true
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		// 取得gate锁后再查询过渡事实，避免拿旧快照等锁后越过仍未完成的关闭。
		if platform.ValidDocumentScope(source.ScopeKey) {
			pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
			if err != nil {
				return err
			}
			if pending {
				return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
			}
		}
		if gate.Epoch > source.Epoch || (gate.Epoch == source.Epoch && gate.State == "BLOCKED") {
			return domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
		}
		if gate.Epoch < source.Epoch {
			if err := repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return err
			}
		}
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`739356f26ec854bb64647a80d03cfca5108edbcf1cb6f76fccf1fbd4a306c8ef`。[参考实现](../../../../services/document/internal/logic/document/authorization_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
