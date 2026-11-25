// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"net/url"
)

// chapter_control.go：保留原实现，按业务合并文件。

type mainControlLayout struct{ Structure, Chapter, Unit uint64 }

func validateChapterAnchor(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.ChapterCreate) (dto.StructureRevision, error) {
	if current.CurrentStructureId != input.ExpectedStructureId {
		return dto.StructureRevision{}, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return structure, err
	}
	if len(structure.Entries) >= 10000 {
		return structure, domain.Violation(domain.StateConflict, "STRUCTURE_ENTRY_LIMIT")
	}
	if input.AfterChapterId != nil {
		for _, entry := range structure.Entries {
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && entry.ChapterId == *input.AfterChapterId {
				return structure, nil
			}
		}
		return structure, domain.Violation(domain.StateConflict, "CHAPTER_ANCHOR_CHANGED")
	}
	return structure, nil
}

// 新章没有旧草稿要迁移；旧单元的支线和审核事实继续保留，正式结构/版本/清单只在本事务追加。
func prepareControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error) {
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" {
		return prepareStableControlledLayout(ctx, tx, current, fixed)
	}
	layout := mainControlLayout{Structure: fixed.Structure}
	if fixed.Intent.Kind != "CHAPTER_CREATE" {
		return layout, nil
	}
	if fixed.Intent.Chapter == nil {
		return layout, domain.Violation(domain.TemporarilyUnavailable, "CHAPTER_INTENT_UNKNOWN")
	}
	old, err := validateChapterAnchor(ctx, tx, current, *fixed.Intent.Chapter)
	if err != nil {
		return layout, err
	}
	if layout.Structure, err = repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation); err != nil {
		return layout, err
	}
	if layout.Chapter, err = repository.CreateInitialChapter(ctx, tx, fixed.Document, layout.Structure); err != nil {
		return layout, err
	}
	if layout.Unit, err = repository.CreateInitialUnit(ctx, tx, fixed.Document, layout.Chapter, layout.Structure); err != nil {
		return layout, err
	}
	entries, err := domain.InsertChapterEntry(old.Entries, *fixed.Intent.Chapter, domain.TextID(layout.Chapter), domain.TextID(layout.Unit))
	if err != nil {
		return layout, err
	}
	err = repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)
	return layout, err
}

// document_creation.go：保留原实现，按业务合并文件。

type Documents struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	quotaLimit uint64
}

func NewDocuments(db *sql.DB, identity *Identity, access *Access, quotaLimit uint64) *Documents {
	return &Documents{db: db, identity: identity, access: access, quotaLimit: quotaLimit}
}
func (s *Documents) Create(ctx context.Context, actor Actor, spaceID uint64, input dto.DocumentCreate) (Actor, MutationBody, error) {
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, "CREATE_DOCUMENT", 0)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	kb := uint64(0)
	if input.KnowledgeBaseId != nil {
		kb, err = domain.CanonicalID(*input.KnowledgeBaseId)
		if err != nil {
			return actor, nil, err
		}
	}
	var admin *pb.IdentityAdmission
	if space.Kind == "ORGANIZATION" {
		if kb == 0 {
			return actor, nil, domain.Violation(domain.PermissionDenied, "ORGANIZATION_DOCUMENT_REQUIRES_KB")
		}
		_, admin, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, "CREATE_DOCUMENT", 0)
		if err != nil {
			var violation domain.RuleViolation
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return actor, nil, err
			}
			admin = nil
		}
		if admin != nil {
			if err = s.identity.Revalidate(ctx, admin); err != nil {
				return actor, nil, err
			}
		}
	}
	actor.Sources, err = MergeSources(actor.Proof, proof, admin)
	if err != nil {
		return actor, nil, err
	}
	var snapshot dto.KnowledgeBase
	if kb != 0 {
		snapshot, err = repository.KnowledgeBaseRow(ctx, s.db, kb, false)
		if err != nil || snapshot.SpaceId != input.SpaceId {
			return actor, nil, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
		}
		revision, _ := domain.CanonicalID(snapshot.AclRevision)
		actor.Sources = append(actor.Sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: revision})
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := repository.SpaceRow(ctx, tx, spaceID, true); err != nil {
			return nil, err
		}
		sources, _ := MergeSources(proof, admin)
		if err := LockSources(ctx, tx, sources); err != nil {
			return nil, err
		}
		if kb != 0 {
			current, err := repository.LockedCreationKnowledgeBase(ctx, tx, kb)
			if err != nil || current.Space != spaceID || current.Lifecycle != "ACTIVE" {
				return nil, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
			}
			if domain.TextID(current.Revision) != snapshot.AclRevision {
				return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
			}
			if admin == nil && space.Kind != "PERSONAL" {
				role, err := repository.KnowledgeBaseRole(ctx, tx, kb, actor.ID)
				if err != nil || role != "ADMIN" {
					allowed, err := repository.KnowledgeBaseCreationState(ctx, tx, kb, actor.ID)
					if err != nil || allowed != "ACTIVE" {
						return nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_CREATION_NOT_ALLOWED")
					}
				}
			}
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: current.Revision}}); err != nil {
				return nil, err
			}
		}
		if _, err := resources.LockQuota(ctx, tx, spaceID, s.quotaLimit); err != nil {
			return nil, err
		}
		id, err := InitializeDocument(ctx, tx, spaceID, kb, actor.ID, input.Title, domain.Visibility(input.VisibilityPolicy), input.OperationId, space.Kind == "ORGANIZATION")
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

// initialization.go：保留原实现，按业务合并文件。

func NewEntityID() (uint64, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data[:])&0x3fffffffffffffff | 0x2000000000000000, nil
}

// 根对象与初始空主线全部属于调用方事务；最后ACTIVE才使完整清单可见。
func InitializeDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility, operation string, locked bool) (uint64, error) {
	id, err := repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)
	if err != nil {
		return 0, err
	}
	structure, err := repository.CreateInitialStructure(ctx, tx, id, operation)
	if err != nil {
		return 0, err
	}
	style, err := repository.CreateInitialStyle(ctx, tx, id, domain.InitialStyle, operation)
	if err != nil {
		return 0, err
	}
	manifest, err := repository.CreateInitialManifest(ctx, tx, id, structure, style, operation)
	if err != nil {
		return 0, err
	}
	chapter, err := repository.CreateInitialChapter(ctx, tx, id, structure)
	if err != nil {
		return 0, err
	}
	unit, err := repository.CreateInitialUnit(ctx, tx, id, chapter, structure)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialStructureEntry(ctx, tx, id, structure, chapter, unit); err != nil {
		return 0, err
	}
	version, err := initialUnitVersion(ctx, tx, id, unit, structure, manifest, operation)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialManifestEntry(ctx, tx, id, manifest, unit, version); err != nil {
		return 0, err
	}
	err = repository.ActivateDocument(ctx, tx, id, structure, style, manifest)
	return id, err
}
func initialUnitVersion(ctx context.Context, tx *sql.Tx, document, unit, structure, manifest uint64, operation string) (uint64, error) {
	version, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	lineage, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	content, err := domain.BuildInitialContent(version, lineage, unit, manifest, operation)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content); err != nil {
		return 0, err
	}
	branch, err := repository.CreateInitialBranch(ctx, tx, document, unit, version)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content); err != nil {
		return 0, err
	}
	if err = repository.ActivateInitialBranch(ctx, tx, branch, lineage); err != nil {
		return 0, err
	}
	err = repository.BindInitialBranch(ctx, tx, document, unit, branch)
	return version, err
}

// knowledge_base.go：保留原实现，按业务合并文件。

func (s *KnowledgeBases) Create(ctx context.Context, actor Actor, space uint64, input dto.KBCreate) (Actor, MutationBody, error) {
	_, proof, err := s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, "CREATE_KNOWLEDGE_BASE", 0)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	actor.Sources, err = MergeSources(actor.Proof, proof)
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 成员来源gate一直持有至提交；退出准备必须等待建库和初始管理员事实提交。
		if _, err := repository.SpaceRow(ctx, tx, space, true); err != nil {
			return nil, err
		}
		kb, err := repository.CreateKnowledgeBase(ctx, tx, space, input.Name, domain.Visibility(input.VisibilityPolicy))
		if err != nil {
			return nil, err
		}
		if err = repository.InsertInitialKBAdministrator(ctx, tx, kb, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", kb), nil
	}
	return actor, run, nil
}
func (s *KnowledgeBases) Rename(ctx context.Context, actor Actor, id uint64, input dto.OrganizationEdit) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "RENAME_KB", false)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision); err != nil {
			return nil, err
		}
		if err := repository.RenameKnowledgeBase(ctx, tx, id, input.Name); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(access.KB.AclRevision)
		if err := QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "RENAME"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}
func (s *KnowledgeBases) CreationGrant(ctx context.Context, actor Actor, id, user uint64, input dto.CreationGrant) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "KB_CREATION_GRANT", false)
	if err != nil {
		return actor, nil, err
	}
	if access.Space.Kind != "ORGANIZATION" {
		return actor, nil, domain.Violation(domain.PermissionDenied, "ORGANIZATION_CREATION_GRANT_REQUIRED")
	}
	actor, err = s.bindTarget(ctx, actor, access, user, "KB_CREATION_GRANT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		kb, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		state := "CLOSED"
		if input.Allowed {
			state = "ACTIVE"
		}
		if err = repository.SetKBCreationGrant(ctx, tx, id, user, state); err != nil {
			return nil, err
		}
		if err = repository.AdvanceKnowledgeBaseACL(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// knowledge_base_access.go：保留原实现，按业务合并文件。

type KnowledgeBaseAccess struct {
	KB      dto.KnowledgeBase
	Space   dto.Space
	Proof   *pb.IdentityAdmission
	Sources []*pb.SourceEpoch
}
type KnowledgeBases struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

func NewKnowledgeBases(db *sql.DB, identity *Identity, access *Access) *KnowledgeBases {
	return &KnowledgeBases{db: db, identity: identity, access: access}
}
func (s *KnowledgeBases) Manage(ctx context.Context, actor Actor, id uint64, purpose string, appoint bool) (KnowledgeBaseAccess, error) {
	var access KnowledgeBaseAccess
	kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
	if err != nil {
		return access, err
	}
	spaceID, _ := domain.CanonicalID(kb.SpaceId)
	if appoint {
		space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
		if err != nil {
			return access, err
		}
		// 任免要求组织管理员；个人所有权不能凭空产生组织身份。
		if space.Kind != "ORGANIZATION" {
			return access, domain.Violation(domain.PermissionDenied, "ORGANIZATION_ROLE_REQUIRED")
		}
	}
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)
	if err != nil && !appoint {
		var violation domain.RuleViolation
		if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
			return access, err
		}
		space, proof, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)
		if err == nil {
			role, roleErr := repository.KnowledgeBaseRole(ctx, s.db, id, actor.ID)
			if roleErr != nil || role != "ADMIN" {
				return access, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_MANAGEMENT_REQUIRED")
			}
		}
	}
	if err != nil {
		return access, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return access, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return access, err
	}
	epoch, _ := domain.CanonicalID(kb.AclRevision)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: epoch})
	return KnowledgeBaseAccess{KB: kb, Space: space, Proof: proof, Sources: sources}, nil
}
func ValidateKnowledgeBase(ctx context.Context, tx *sql.Tx, access KnowledgeBaseAccess, expected string) (dto.KnowledgeBase, error) {
	id, _ := domain.CanonicalID(access.KB.Id)
	current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
	if err != nil {
		return current, err
	}
	if current.AclRevision != access.KB.AclRevision || current.AclRevision != expected {
		return current, domain.Violation(domain.StateConflict, "KB_ACL_REVISION_MISMATCH")
	}
	return current, nil
}
func (s *KnowledgeBases) Target(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (*pb.IdentityAdmission, error) {
	if access.Space.Kind == "PERSONAL" {
		return s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)
	}
	organization, _ := domain.CanonicalID(*access.Space.OrganizationId)
	return s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)
}
func (s *KnowledgeBases) bindTarget(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (Actor, error) {
	target, err := s.Target(ctx, actor, access, user, purpose)
	if err != nil {
		return actor, err
	}
	if err = s.identity.Revalidate(ctx, target); err != nil {
		return actor, err
	}
	sources, err := MergeSources(actor.Proof, access.Proof, target)
	if err != nil {
		return actor, err
	}
	baseSources, _ := MergeSources(actor.Proof, access.Proof)
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	return actor, nil
}

// knowledge_base_deletion.go：保留原实现，按业务合并文件。

func (s *KnowledgeBases) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "DELETE_KB", false)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	// 先按统一顺序锁子文档来源，再取得库/文档业务锁，避免与普通文档变更逆序等待。
	snapshot, err := repository.KnowledgeBaseChildren(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	children := map[uint64]uint64{}
	for _, child := range snapshot {
		children[child.ID] = child.Epoch
		actor.Sources = append(actor.Sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(child.ID), Epoch: child.Epoch})
	}
	if len(children) > 400 {
		return actor, nil, domain.Violation(domain.StateConflict, "LIFECYCLE_PLAN_REQUIRED")
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		pending, err := repository.PersonalTransferKnowledgeBasePending(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if pending {
			return nil, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_PENDING")
		}
		locked, err := repository.KnowledgeBaseChildren(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		for _, child := range locked {
			if child.HasRelease {
				return nil, domain.Violation(domain.StateConflict, "PUBLISHED_LIFECYCLE_REQUIRED")
			}
			if children[child.ID] != child.Epoch {
				return nil, domain.Violation(domain.StateConflict, "KB_CHILDREN_CHANGED")
			}
		}
		if len(locked) != len(children) {
			return nil, domain.Violation(domain.StateConflict, "KB_CHILDREN_CHANGED")
		}
		if err = repository.CloseKnowledgeBaseForDeletion(ctx, tx, id); err != nil {
			return nil, err
		}
		if err = repository.CloseKnowledgeBaseChildren(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(current.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "CLOSE_KB"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// knowledge_base_role.go：保留原实现，按业务合并文件。

func (s *KnowledgeBases) Role(ctx context.Context, actor Actor, id, user uint64, input dto.KBRole) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "APPOINT_KB_ROLE", true)
	if err != nil {
		return actor, nil, err
	}
	actor, err = s.bindTarget(ctx, actor, access, user, "APPOINT_KB_ROLE")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 目标member来源已在通用变更事务持锁；持有Doc业务锁后不再反向调用账号RPC。
		kb, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedAclRevision)
		if err != nil {
			return nil, err
		}
		if input.Role == "MEMBER" || input.Role == "REMOVE" {
			role, _ := repository.KnowledgeBaseRole(ctx, tx, id, user)
			if role == "ADMIN" {
				count, err := repository.AvailableKBAdministrators(ctx, tx, id)
				if err != nil {
					return nil, errors.New("KB_ADMIN_UNAVAILABLE")
				}
				if count <= 1 {
					return nil, domain.Violation(domain.StateConflict, "LAST_KB_ADMIN_REQUIRED")
				}
			}
		}
		reserved, err := repository.KBAdministratorReservations(ctx, tx, id, user)
		if err != nil {
			return nil, errors.New("KB_ADMIN_UNAVAILABLE")
		}
		if reserved != 0 {
			return nil, domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
		}
		role, state := input.Role, "ACTIVE"
		if input.Role == "REMOVE" {
			role = "MEMBER"
			state = "CLOSED"
		}
		if err = repository.SetKBRole(ctx, tx, id, user, role, state); err != nil {
			return nil, err
		}
		if err = repository.AdvanceKnowledgeBaseACL(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// management.go：保留原实现，按业务合并文件。

type Management struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

func NewManagement(db *sql.DB, identity *Identity, access *Access) *Management {
	return &Management{db: db, identity: identity, access: access}
}
func (s *Management) managed(ctx context.Context, actor Actor, id uint64, purpose string) (Actor, DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return actor, access, err
	}
	if !access.Permissions.Manage {
		return actor, access, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	actor.Sources = access.Sources
	return actor, access, nil
}
func (s *Management) Rename(ctx context.Context, actor Actor, id uint64, input dto.DocumentEdit) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "RENAME_DOCUMENT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedControlRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if err = repository.RenameDocument(ctx, tx, id, input.Title); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
func (s *Management) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "DELETE_DOCUMENT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if err = repository.CloseDocument(ctx, tx, id); err != nil {
			return nil, err
		}
		// 已发布文档同样可移入回收站。先停止当前正式指针，并在这一事务
		// 推进不可见索引事件；旧正文输出仍由下方doc-acl实际ACK排空。
		if err = queueVisibilityIndexEvents(ctx, tx, input.OperationId, []uint64{id}, 0); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		// 删除先冻结准入；恢复器核验旧输出停止后才设置TRASHED及清理期限。
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "TRASH_DOCUMENT"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// space.go：保留原实现，按业务合并文件。

type DirectoryReader interface {
	Read(context.Context, string, string, any) error
}
type Spaces struct {
	identity   *Identity
	directory  DirectoryReader
	pagination *Pagination
}

func NewSpaces(identity *Identity, directory DirectoryReader, pagination *Pagination) *Spaces {
	return &Spaces{identity: identity, directory: directory, pagination: pagination}
}

// 既定DDL没有owner/org唯一索引；持久唯一gate保证并发首次打开只物化一个空间。
func MaterializeSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (dto.Space, error) {
	key := "space-user:" + domain.TextID(owner)
	if kind == "ORGANIZATION" {
		key = "space-org:" + domain.TextID(owner)
	}
	if err := repository.EnsureSpaceGate(ctx, tx, key); err != nil {
		return dto.Space{}, err
	}
	if err := repository.LockSpaceGate(ctx, tx, key); err != nil {
		return dto.Space{}, err
	}
	existing, err := repository.ExistingSpace(ctx, tx, kind, owner)
	if err == sql.ErrNoRows {
		existing, err = repository.CreateSpace(ctx, tx, kind, owner)
		if err != nil {
			return dto.Space{}, err
		}
	} else if err != nil {
		return dto.Space{}, err
	}
	return repository.SpaceRow(ctx, tx, existing, true)
}
func (s *Spaces) Prepare(ctx context.Context, actor Actor, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "spaces:" + domain.TextID(actor.ID)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	// limit是单页上限，不承诺填满。每个组织都要核验成员/建库资格并登记撤权来源，
	// 大页会耗尽短请求预算；用完整cursor续接小页，不能跳过证明或放宽失效边界。
	if limit > 5 {
		limit = 5
	}
	personal := last == ""
	accountCursor := last
	if last == "@org" {
		accountCursor = ""
	}
	directoryLimit := limit
	if personal && limit > 1 {
		directoryLimit--
	}
	var directory dto.OrganizationPage
	if err = s.directory.Read(ctx, cookie, "/v1/organizations?limit="+domain.TextID(uint64(directoryLimit))+"&cursor="+url.QueryEscape(accountCursor), &directory); err != nil {
		return nil, nil, err
	}
	proofs := []*pb.IdentityAdmission{actor.Proof}
	firstOnly := personal && limit == 1
	moreAfterPersonal := firstOnly && len(directory.Items) > 0
	if firstOnly {
		directory.Items = nil
	}
	canCreate := map[string]bool{}
	for _, organization := range directory.Items {
		id, _ := domain.CanonicalID(organization.Id)
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}
		proof, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, target, "SPACE_DIRECTORY", 0, 0)
		if err != nil {
			return nil, nil, err
		}
		proofs = append(proofs, proof)
		creation, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, target, "SPACE_DIRECTORY", 0, 0)
		if err == nil {
			canCreate[organization.Id] = true
			proofs = append(proofs, creation)
		} else {
			var violation domain.RuleViolation
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return nil, nil, err
			}
		}
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
	}
	sources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		page := dto.SpacePage{Items: []dto.Space{}}
		if personal {
			space, err := MaterializeSpace(ctx, tx, "PERSONAL", actor.ID)
			if err != nil {
				return nil, err
			}
			space.Actions = []string{"CREATE_KNOWLEDGE_BASE", "CREATE_DOCUMENT"}
			page.Items = append(page.Items, space)
		}
		for _, organization := range directory.Items {
			id, _ := domain.CanonicalID(organization.Id)
			space, err := MaterializeSpace(ctx, tx, "ORGANIZATION", id)
			if err != nil {
				return nil, err
			}
			space.Actions = []string{}
			if canCreate[organization.Id] {
				space.Actions = append(space.Actions, "CREATE_KNOWLEDGE_BASE")
			}
			page.Items = append(page.Items, space)
		}
		page.HasMore = directory.HasMore
		if firstOnly {
			page.HasMore = moreAfterPersonal
			if page.HasMore {
				page.NextCursor = s.pagination.Next(binding, "@org")
			}
			return page, nil
		}
		if directory.NextCursor != nil {
			page.NextCursor = s.pagination.Next(binding, *directory.NextCursor)
		}
		return page, nil
	}
	return sources, run, nil
}
