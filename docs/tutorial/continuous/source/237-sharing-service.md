# services/document/internal/logic/document/sharing_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/237-sharing-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Apply`、`Decide`、`Applications`、`OperationProgress`、`CreateInvitation`、`CancelInvitation`、`Invitations`、`InvitationCipher`、`Sharing`、`NewSharing`、`sharingFilter`、`Archive`、`personalManagement`、`validateInvitationGrants`、`invitationSnapshot`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// applications.go：保留原实现，按业务合并文件。

func (s *Sharing) Apply(ctx context.Context, actor Actor, hash []byte, input dto.DocumentApplicationCreate) (Actor, MutationBody, error) {
	invitation, document, err := repository.InvitationByHash(ctx, s.db, hash)
	if err != nil {
		return actor, nil, err
	}
	snapshot, err := repository.DocumentRow(ctx, s.db, document, false)
	if err != nil {
		return actor, nil, err
	}
	epoch, _ := domain.CanonicalID(snapshot.AclRevision)
	actor.Sources = append(append([]*pb.SourceEpoch{}, actor.Proof.Sources...), &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(document), Epoch: epoch})
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := repository.DocumentRow(ctx, tx, document, true)
		if err != nil {
			return nil, err
		}
		if current.AclRevision != snapshot.AclRevision {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_ACCESS_CHANGED")
		}
		invite, _, err := invitationSnapshot(ctx, tx, invitation, true)
		if err != nil {
			return nil, err
		}
		if invite.State != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_NOT_CURRENT")
		}
		// 同一文档锁串行不同operation的申请，避免同一个申请者生成多个待审批实体。
		existing, err := repository.PendingApplication(ctx, tx, document, actor.ID)
		if err == nil {
			return OperationResult(input.OperationId, "SUCCEEDED", existing), nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		raw, _ := json.Marshal(invite.RequestedGrants)
		id, err := repository.InsertApplication(ctx, tx, document, invitation, actor.ID, string(raw), input.Reason)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
func (s *Sharing) Decide(ctx context.Context, actor Actor, id uint64, input dto.ApplicationReview) (Actor, MutationBody, error) {
	application, err := repository.ApplicationRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	document, _ := domain.CanonicalID(application.DocumentId)
	access, err := s.personalManagement(ctx, actor, document, "DECIDE_DOCUMENT_APPLICATION")
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	if input.Decision == "ACCEPT" {
		target, _ := domain.CanonicalID(application.ApplicantId)
		proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, "DECIDE_DOCUMENT_APPLICATION", document)
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
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		current, err := repository.ApplicationRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.State != "PENDING" || current.Revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "APPLICATION_CHANGED")
		}
		state := "REJECTED"
		if input.Decision == "ACCEPT" {
			if err = validateInvitationGrants(ctx, tx, document, current.RequestedGrants); err != nil {
				return nil, err
			}
			for _, grant := range current.RequestedGrants {
				if err = repository.ApproveInvitationGrant(ctx, tx, document, current.ApplicantId, grant); err != nil {
					return nil, err
				}
			}
			state = "ACCEPTED"
			if err = repository.AdvanceDocumentACL(ctx, tx, document); err != nil {
				return nil, err
			}
			epoch, _ := domain.CanonicalID(d.AclRevision)
			if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(document), Epoch: epoch, Entity: document, Action: "ACL"}); err != nil {
				return nil, err
			}
		}
		// 提交时有效性已持久固定；审批不重新检查邀请当前期限或OPEN状态。
		if err = repository.DecideApplication(ctx, tx, id, actor.ID, state); err != nil {
			return nil, err
		}
		resultState := "SUCCEEDED"
		if state == "ACCEPTED" {
			resultState = "PREPARING"
		}
		return OperationResult(input.OperationId, resultState, id), nil
	}
	return actor, run, nil
}
func (s *Sharing) Applications(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	filter, err := sharingFilter(filters, "PENDING", "PROCESSED")
	if err != nil {
		return nil, nil, err
	}
	var access DocumentAccess
	sources := actor.Proof.Sources
	binding := "doc-applications:own:" + domain.TextID(actor.ID)
	if document != 0 {
		access, err = s.personalManagement(ctx, actor, document, "LIST_DOCUMENT_APPLICATIONS")
		if err != nil {
			return nil, nil, err
		}
		sources = access.Sources
		binding = "doc-applications:" + domain.TextID(actor.ID) + ":" + domain.TextID(document) + ":" + access.Document.AclRevision
	}
	binding += ":newest:" + filter
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	key := actor.ID
	if document != 0 {
		key = document
	}
	ids, err := repository.ApplicationIDs(ctx, s.db, key, document != 0, last, limit+1, filter)
	if err != nil {
		return nil, nil, err
	}
	page := dto.DocumentApplicationPage{Items: []dto.DocumentApplication{}}
	if len(ids) > limit {
		page.HasMore = true
		ids = ids[:limit]
		page.NextCursor = s.pagination.Next(binding, domain.TextID(ids[limit-1]))
	}
	for _, id := range ids {
		application, err := repository.ApplicationRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		name := "已停用账号"
		target, _ := domain.CanonicalID(application.ApplicantId)
		doc, _ := domain.CanonicalID(application.DocumentId)
		proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, "DOCUMENT_APPLICATION_NAMES", doc)
		if err != nil {
			var violation domain.RuleViolation
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return nil, nil, err
			}
		} else {
			person, err := s.directory.Collaborator(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
			if err != nil {
				return nil, nil, err
			}
			if person.UserId != application.ApplicantId || person.DisplayName == "" {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			name = person.DisplayName
			sources, err = MergeSources(&pb.IdentityAdmission{Sources: sources}, proof)
			if err != nil {
				return nil, nil, err
			}
		}
		application.ApplicantDisplayName = &name
		page.Items = append(page.Items, application)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if document != 0 {
			if _, err := ValidateAccess(ctx, tx, access); err != nil {
				return nil, err
			}
		}
		// 姓名解析在事务外完成，输出登记前复核原记录，避免审批/归档后返回旧状态。
		for _, application := range page.Items {
			id, _ := domain.CanonicalID(application.Id)
			current, err := repository.ApplicationRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			if current.Revision != application.Revision {
				return nil, domain.Violation(domain.StateConflict, "APPLICATION_CHANGED")
			}
		}
		return page, nil
	}
	return sources, run, nil
}
func (s *Sharing) OperationProgress(actor Actor, operation string) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		raw, err := repository.ActorOperationResult(ctx, tx, operation, actor.ID)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if json.Unmarshal(raw, &result) != nil {
			return nil, errors.New("OPERATION_RESULT_UNAVAILABLE")
		}
		return result, nil
	}
}

// invitations.go：保留原实现，按业务合并文件。

func (s *Sharing) CreateInvitation(ctx context.Context, actor Actor, id uint64, hours int64, input dto.DocumentInvitationCreate) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.personalManagement(ctx, actor, id, "CREATE_DOCUMENT_INVITATION")
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if d.AclRevision != input.ExpectedAclRevision {
			return nil, domain.Violation(domain.StateConflict, "ACL_REVISION_MISMATCH")
		}
		if err = validateInvitationGrants(ctx, tx, id, input.RequestedGrants); err != nil {
			return nil, err
		}
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			return nil, err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256(raw)
		encrypted, err := s.tokens.Seal(token, domain.TextID(id))
		if err != nil {
			return nil, err
		}
		grants, _ := json.Marshal(input.RequestedGrants)
		invite, err := repository.InsertInvitation(ctx, tx, id, actor.ID, hash[:], encrypted, string(grants), time.Now().UTC().Add(time.Duration(hours)*time.Hour))
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", invite), nil
	}
	// 创建与原号重放都重新核验敏感输出资格；receipt只保存实体引用，令牌由绑定密文恢复。
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		raw, _ := json.Marshal(value["result"])
		var ref struct {
			Entity string `json:"entity_id"`
		}
		if json.Unmarshal(raw, &ref) != nil {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
		inviteID, err := domain.CanonicalID(ref.Entity)
		if err != nil {
			return nil, nil, err
		}
		invite, encrypted, err := invitationSnapshot(ctx, tx, inviteID, true)
		if err != nil || invite.DocumentId != domain.TextID(id) {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
		token, err := s.tokens.Open(encrypted, domain.TextID(id))
		if err != nil {
			return nil, nil, err
		}
		return dto.DocumentInvitationCreated{Invitation: invite, ShareUrl: s.origin + "/join-document?invitation_token=" + token, OperationId: input.OperationId}, access.Sources, nil
	}
	return actor, run, present, nil
}
func (s *Sharing) CancelInvitation(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	invite, _, err := invitationSnapshot(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	document, _ := domain.CanonicalID(invite.DocumentId)
	access, err := s.personalManagement(ctx, actor, document, "CANCEL_DOCUMENT_INVITATION")
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, _, err := invitationSnapshot(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.Revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_REVISION_MISMATCH")
		}
		if current.State != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_NOT_CURRENT")
		}
		if err = repository.CloseInvitation(ctx, tx, id); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
func (s *Sharing) Invitations(ctx context.Context, actor Actor, id uint64, limitText, cursor string, filters ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	filter, err := sharingFilter(filters, "ACTIVE", "INACTIVE")
	if err != nil {
		return nil, nil, err
	}
	access, err := s.personalManagement(ctx, actor, id, "LIST_DOCUMENT_INVITATIONS")
	if err != nil {
		return nil, nil, err
	}
	binding := "doc-invitations:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision + ":newest:" + filter
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		ids, err := repository.InvitationIDs(ctx, tx, id, last, limit+1, filter)
		if err != nil {
			return nil, err
		}
		page := dto.DocumentInvitationPage{Items: []dto.DocumentInvitation{}}
		for _, key := range ids {
			value, _, err := invitationSnapshot(ctx, tx, key, false)
			if err != nil {
				return nil, err
			}
			page.Items = append(page.Items, value)
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.HasMore = true
			page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
		}
		return page, nil
	}
	return access.Sources, run, nil
}

// sharing.go：保留原实现，按业务合并文件。

type InvitationCipher interface {
	Seal(string, string) ([]byte, error)
	Open([]byte, string) (string, error)
}
type Sharing struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
	tokens     InvitationCipher
	origin     string
	directory  CollaboratorDirectoryReader
}

func NewSharing(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, tokens InvitationCipher, origin string, directory CollaboratorDirectoryReader) *Sharing {
	return &Sharing{db: db, identity: identity, access: access, pagination: pagination, tokens: tokens, origin: origin, directory: directory}
}

func sharingFilter(filters []string, first, second string) (string, error) {
	value := "ALL"
	if len(filters) > 0 && filters[0] != "" {
		value = filters[0]
	}
	if value != "ALL" && value != first && value != second {
		return "", domain.Violation(domain.InvalidRequest, "SHARING_FILTER_INVALID")
	}
	return value, nil
}

func (s *Sharing) Archive(ctx context.Context, actor Actor, id uint64, kind string, input dto.VersionedAction) (Actor, MutationBody, error) {
	if kind != "INVITATION" && kind != "APPLICATION" {
		return actor, nil, domain.Violation(domain.InvalidRequest, "SHARING_RECORD_INVALID")
	}
	var document string
	var lookupError error
	if kind == "INVITATION" {
		value, _, err := invitationSnapshot(ctx, s.db, id, false)
		lookupError = err
		document = value.DocumentId
	} else {
		value, err := repository.ApplicationRow(ctx, s.db, id, false)
		lookupError = err
		document = value.DocumentId
	}
	if lookupError != nil {
		var violation domain.RuleViolation
		if errors.As(lookupError, &violation) && violation.Kind == domain.EntityNotFound {
			// 不存在与无管理资格同样拒绝，避免裸记录编号成为存在性探测接口。
			return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		return actor, nil, lookupError
	}
	doc, _ := domain.CanonicalID(document)
	access, err := s.personalManagement(ctx, actor, doc, "ARCHIVE_DOCUMENT_SHARING")
	if err != nil {
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound) {
			return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		var revision, state string
		if kind == "INVITATION" {
			value, _, err := invitationSnapshot(ctx, tx, id, true)
			if err != nil {
				return nil, err
			}
			revision, state = value.Revision, value.State
		} else {
			value, err := repository.ApplicationRow(ctx, tx, id, true)
			if err != nil {
				return nil, err
			}
			revision, state = value.Revision, value.State
		}
		if revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "SHARING_RECORD_CHANGED")
		}
		if state == "OPEN" || state == "PENDING" {
			return nil, domain.Violation(domain.StateConflict, "SHARING_RECORD_ACTIVE")
		}
		if err := repository.ArchiveSharingRecord(ctx, tx, kind, id, doc, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
func (s *Sharing) personalManagement(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return access, err
	}
	if !access.Permissions.Manage {
		return access, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return access, err
	}
	if space.Kind != "PERSONAL" {
		return access, domain.Violation(domain.PermissionDenied, "PERSONAL_DOCUMENT_REQUIRED")
	}
	return access, nil
}

// 邀请只能授予当前有效单元的EDIT或当前文档的REVIEW/COMMENT，不能扩成管理权限。
func validateInvitationGrants(ctx context.Context, tx *sql.Tx, document uint64, grants []dto.InvitationGrant) error {
	if len(grants) < 1 || len(grants) > 100 {
		return domain.Violation(domain.InvalidRequest, "INVITATION_GRANTS_INVALID")
	}
	for _, grant := range grants {
		id, err := domain.CanonicalID(grant.ScopeId)
		if err != nil {
			return err
		}
		if grant.Capability == "EDIT" {
			if grant.ScopeKind != "UNIT" && grant.ScopeKind != "CHAPTER" {
				return domain.Violation(domain.InvalidRequest, "EDIT_SCOPE_INVALID")
			}
			if repository.EditGrantScope(ctx, tx, grant.ScopeKind, id, document) != nil {
				return domain.Violation(domain.InvalidRequest, "GRANT_SCOPE_NOT_IN_DOCUMENT")
			}
		} else if (grant.Capability != "REVIEW" && grant.Capability != "COMMENT") || grant.ScopeKind != "DOCUMENT" || id != document {
			return domain.Violation(domain.InvalidRequest, "INVITATION_CAPABILITY_INVALID")
		}
	}
	return nil
}
func invitationSnapshot(ctx context.Context, q repository.Queryer, id uint64, lock bool) (dto.DocumentInvitation, []byte, error) {
	fact, err := repository.InvitationRow(ctx, q, id, lock)
	if err != nil {
		return fact.Invitation, nil, err
	}
	v := fact.Invitation
	v.ExpiresAt = fact.ExpiresAt.UTC().Format(time.RFC3339Nano)
	if v.State == "OPEN" && !fact.ExpiresAt.After(time.Now()) {
		v.State = "EXPIRED"
	}
	return v, fact.Ciphertext, nil
}
````

UTF-8/LF全文SHA-256：`069ec65055c58fc04779496ac07b035774b301bfffb81130c73cf7d4e77eb6c8`。[参考实现](../../../../services/document/internal/logic/document/sharing_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
