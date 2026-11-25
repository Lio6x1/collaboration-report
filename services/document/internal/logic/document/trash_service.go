// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// trash.go：保留原实现，按业务合并文件。

// Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。
// 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。
type Trash struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
}

func NewTrash(db *sql.DB, identity *Identity, access *Access, pagination *Pagination) *Trash {
	return &Trash{db: db, identity: identity, access: access, pagination: pagination}
}

type trashSpaceAccess struct {
	Space     dto.Space
	Proof     *pb.IdentityAdmission
	Inherited bool
}

type trashDocumentAccess struct {
	Facts domain.TrashFacts
	KB    *dto.KnowledgeBase
}

func (s *Trash) spaceAuthority(ctx context.Context, actor Actor, id uint64, purpose string, document uint64) (trashSpaceAccess, error) {
	var admission trashSpaceAccess
	space, err := repository.TrashSpaceRow(ctx, s.db, id, false)
	if err != nil {
		return admission, err
	}
	if space.Lifecycle != "ACTIVE" {
		return admission, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	current, proof, err := s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)
	inherited := err == nil
	if err != nil {
		var denied domain.RuleViolation
		if !errors.As(err, &denied) || denied.Kind != domain.PermissionDenied || space.Kind != "ORGANIZATION" {
			return admission, err
		}
		current, proof, err = s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)
	}
	if err != nil {
		return admission, err
	}
	if current.Id != space.Id || current.Lifecycle != "ACTIVE" {
		return admission, domain.Violation(domain.StateConflict, "TRASH_SPACE_CHANGED")
	}
	if err := s.identity.Revalidate(ctx, proof); err != nil {
		return admission, err
	}
	return trashSpaceAccess{Space: current, Proof: proof, Inherited: inherited}, nil
}

func (s *Trash) documentAuthority(ctx context.Context, q repository.Queryer, actor Actor, space trashSpaceAccess, facts domain.TrashFacts, lock bool) (trashDocumentAccess, error) {
	access := trashDocumentAccess{Facts: facts}
	if facts.Document.SpaceId != space.Space.Id {
		return access, domain.Violation(domain.StateConflict, "TRASH_SPACE_CHANGED")
	}
	permissions := domain.PermissionFacts{PersonalOwner: space.Inherited && space.Space.Kind == "PERSONAL", OrganizationAdmin: space.Inherited && space.Space.Kind == "ORGANIZATION"}
	// spaceAuthority对组织普通成员也取得真实成员proof；负责关系只读当前Doc，
	// 事务内再比较原负责ID及ACL。离组/注销已经清NULL的关系不会复活。
	permissions.Responsible = space.Space.Kind == "ORGANIZATION" && facts.Document.ResponsibleUserId != nil && *facts.Document.ResponsibleUserId == domain.TextID(actor.ID)
	if facts.Document.KnowledgeBaseId != nil {
		id, err := domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return access, err
		}
		kb, err := repository.TrashKnowledgeBaseRow(ctx, q, id, lock)
		if err != nil {
			return access, err
		}
		if kb.SpaceId != facts.Document.SpaceId {
			return access, domain.Violation(domain.TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
		}
		access.KB = &kb
		if !space.Inherited && kb.Lifecycle == "ACTIVE" {
			role, err := repository.TrashKBRole(ctx, q, id, actor.ID, lock)
			if err != nil {
				return access, err
			}
			permissions.KnowledgeBaseAdmin = role == "ADMIN"
		}
	}
	if !domain.CanManageTrash(permissions) {
		return access, domain.Violation(domain.PermissionDenied, "TRASH_MANAGEMENT_REQUIRED")
	}
	return access, nil
}

func trashSources(actor Actor, space trashSpaceAccess, access trashDocumentAccess, acl, publication uint64) ([]*pb.SourceEpoch, error) {
	sources, err := MergeSources(actor.Proof, space.Proof)
	if err != nil {
		return nil, err
	}
	if access.KB != nil {
		epoch, err := domain.CanonicalID(access.KB.AclRevision)
		if err != nil {
			return nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + access.KB.Id, Epoch: epoch})
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + access.Facts.Document.Id, Epoch: acl})
	if publication != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "publication:" + access.Facts.Document.Id, Epoch: publication})
	}
	return sources, nil
}

// validate先锁KB再Doc，避免与持kb-acl gate的完整visibility fanout倒序。
// Account/角色来源gate已经由外层按scope字典序取得；角色必须用当前读，不能复用RR旧快照。
func (s *Trash) validate(ctx context.Context, tx *sql.Tx, actor Actor, space trashSpaceAccess, original trashDocumentAccess) (domain.TrashFacts, *dto.KnowledgeBase, error) {
	qualified, err := s.documentAuthority(ctx, tx, actor, space, original.Facts, true)
	if err != nil {
		return domain.TrashFacts{}, nil, err
	}
	if (qualified.KB == nil) != (original.KB == nil) || (qualified.KB != nil && qualified.KB.AclRevision != original.KB.AclRevision) {
		return domain.TrashFacts{}, nil, domain.Violation(domain.StateConflict, "TRASH_KNOWLEDGE_BASE_CHANGED")
	}
	id, _ := domain.CanonicalID(original.Facts.Document.Id)
	current, err := repository.TrashDocumentRow(ctx, tx, id, true)
	if err != nil {
		return current, nil, err
	}
	d, before := current.Document, original.Facts.Document
	if d.Lifecycle != "TRASHED" || d.SpaceId != before.SpaceId || d.AclRevision != before.AclRevision ||
		!sameTrashID(d.KnowledgeBaseId, before.KnowledgeBaseId) || !sameTrashID(d.ResponsibleUserId, before.ResponsibleUserId) {
		return current, nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
	}
	spaceID, _ := domain.CanonicalID(d.SpaceId)
	currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
	if err != nil {
		return current, nil, err
	}
	if currentSpace.Lifecycle != "ACTIVE" || currentSpace.Kind != space.Space.Kind ||
		!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) || !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId) {
		return current, nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	return current, qualified.KB, nil
}

func sameTrashID(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// Cursor绑定真实会话及空间；列表输出仍登记所有返回项和lookahead依赖的来源。
// 签名仅证明分页位置，不能代替下一次请求的当前资格复核。
func trashCursorBinding(actor Actor, space uint64) string {
	return "trash:" + domain.TextID(actor.ID) + ":" + domain.TextID(space) + ":" + hex.EncodeToString(actor.Proof.SessionRef)
}

// trash_list.go：保留原实现，按业务合并文件。

func (s *Trash) List(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	space, err := s.spaceAuthority(ctx, actor, spaceID, "LIST_TRASH", 0)
	if err != nil {
		return nil, nil, err
	}
	binding := trashCursorBinding(actor, spaceID)
	limit, lastText, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	var last uint64
	if lastText != "" {
		last, err = domain.CanonicalID(lastText)
		if err != nil {
			return nil, nil, domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		}
	}
	ids, err := repository.TrashCandidates(ctx, s.db, spaceID, actor.ID, space.Inherited, last, limit+1)
	if err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, space.Proof)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[string]uint64, len(sources))
	for _, source := range sources {
		seen[source.ScopeKey] = source.Epoch
	}
	accesses := make([]trashDocumentAccess, 0, len(ids))
	for _, id := range ids {
		facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		if facts.Document.Lifecycle != "TRASHED" {
			return nil, nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
		}
		access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
		if err != nil {
			return nil, nil, err
		}
		acl, err := domain.CanonicalID(facts.Document.AclRevision)
		if err != nil {
			return nil, nil, err
		}
		itemSources, err := trashSources(actor, space, access, acl, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, source := range itemSources {
			if epoch, exists := seen[source.ScopeKey]; exists {
				if epoch != source.Epoch {
					return nil, nil, domain.Violation(domain.StateConflict, "TRASH_SOURCE_CHANGED")
				}
				continue
			}
			seen[source.ScopeKey] = source.Epoch
			sources = append(sources, source)
		}
		accesses = append(accesses, access)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 有结果时validate按KB→Doc→Space当前读；空列表才单独锁Space，避免
		// 先做一致性读建立旧ReadView，也不把空间注销后的快照当作合法空页。
		if len(accesses) == 0 {
			currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
			if err != nil {
				return nil, err
			}
			if currentSpace.Lifecycle != "ACTIVE" || currentSpace.Kind != space.Space.Kind ||
				!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) || !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId) {
				return nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
			}
		}
		page := dto.DocumentPage{Items: []dto.Document{}, HasMore: len(accesses) > limit}
		for index, access := range accesses {
			current, _, err := s.validate(ctx, tx, actor, space, access)
			if err != nil {
				return nil, err
			}
			if index < limit {
				page.Items = append(page.Items, current.Document)
			}
		}
		if page.HasMore {
			// lookahead只探测，不将其ID写入cursor，否则下一页会吞掉这条真实回收项。
			page.NextCursor = s.pagination.Next(binding, page.Items[len(page.Items)-1].Id)
		}
		return page, nil
	}
	return sources, run, nil
}

// trash_restore.go：保留原实现，按业务合并文件。

// 与发布Prepare相同，返回必需的presentation：同号竞态即使跳过run，也不能
// 提交比当前Doc metadata更高的来源。固定结果本身仍由标准Mutation读取。
func (s *Trash) Restore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, MutationPresentation, error) {
	actor, run, err := s.prepareRestore(ctx, actor, id, input)
	if err != nil {
		return actor, nil, nil, err
	}
	return actor, run, s.restorePresentation(actor, id), nil
}

func (s *Trash) prepareRestore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, error) {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, err
	}
	space, err := s.spaceAuthority(ctx, actor, spaceID, "RESTORE_DOCUMENT", id)
	if err != nil {
		return actor, nil, err
	}
	access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, err
	}
	if receipt, err := repository.TrashRestoreOriginalReceipt(ctx, s.db, input.OperationId); err == nil {
		if err := domain.ValidateTrashRestoreReceipt(receipt, input.OperationId, actor.ID, id); err != nil {
			return actor, nil, err
		}
		return s.prepareOriginalRestore(ctx, actor, id)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return actor, nil, err
	}
	if err = domain.ValidateTrashRestore(facts, space.Space, access.KB, input.ExpectedControlRevision); err != nil {
		return actor, nil, err
	}
	aclHighWater, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+domain.TextID(id))
	if err != nil {
		return actor, nil, err
	}
	publicationHighWater, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+domain.TextID(id))
	if err != nil {
		return actor, nil, err
	}
	next, err := domain.PlanTrashRestore(facts, aclHighWater, publicationHighWater)
	if err != nil {
		return actor, nil, err
	}
	// 新epoch仅在原Mutation事务内预锁，随资格/状态复核失败一起回滚。
	// Gate高水位在预锁前若又前进，标准LockSources会拒绝本次动作，不降低它。
	actor.Sources, err = trashSources(actor, space, access, next.ACL, next.Publication)
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, kb, err := s.validate(ctx, tx, actor, space, access)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateTrashRestore(current, space.Space, kb, input.ExpectedControlRevision); err != nil {
			return nil, err
		}
		before, d := facts.Document, current.Document
		if d.PublicationEpoch != before.PublicationEpoch || d.VisibilityRevision != before.VisibilityRevision || current.Sequence != facts.Sequence {
			return nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
		}
		if err := repository.RestoreTrashDocument(ctx, tx, id, input.ExpectedControlRevision, next); err != nil {
			return nil, err
		}
		payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: input.OperationId, Type: "DOCUMENT_UNPUBLISHED", Document: id, Sequence: next.Sequence, PublicationEpoch: next.Publication, OccurredAt: time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		// 恢复的不可见权威摘要和metadata、权限关闭、原号receipt共同提交，未发送成功不冒称消费。
		if err = repository.InsertIndexEvent(ctx, tx, id, next.Sequence, input.OperationId, "DOCUMENT_UNPUBLISHED", payload); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

func (s *Trash) restorePresentation(actor Actor, id uint64) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, nil, err
		}
		acl, err := domain.CanonicalID(current.Document.AclRevision)
		if err != nil {
			return nil, nil, err
		}
		publication, err := domain.CanonicalID(current.Document.PublicationEpoch)
		if err != nil {
			return nil, nil, err
		}
		foundACL, foundPublication := false, false
		for _, source := range actor.Sources {
			if source.ScopeKey == "doc-acl:"+domain.TextID(id) {
				foundACL = source.Epoch == acl
			}
			if source.ScopeKey == "publication:"+domain.TextID(id) {
				foundPublication = source.Epoch == publication
			}
		}
		if !foundACL || !foundPublication {
			// receipt可能由并发原请求刚刚提交。本次不得把预想的next gate留在
			// SQL；明确失败使外层回滚，下次原POST走标准Replay的固定结果。
			return nil, nil, domain.Violation(domain.StateConflict, "TRASH_RESTORE_SOURCE_CHANGED")
		}
		// 最小操作结果只依赖原发起人的当前身份，不把管理数据夹入receipt输出。
		return value, actor.Proof.Sources, nil
	}
}

func (s *Trash) prepareOriginalRestore(ctx context.Context, actor Actor, id uint64) (Actor, MutationBody, error) {
	// 回执确认之后再取当前事实，避免并发首次提交把前面的TRASHED快照变成
	// 已过期来源；回放不计算next，也不提前开放一个尚未发生的恢复代次。
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, err
	}
	space, err := s.spaceAuthority(ctx, actor, spaceID, "RESTORE_DOCUMENT", id)
	if err != nil {
		return actor, nil, err
	}
	access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, err
	}
	acl, err := domain.CanonicalID(facts.Document.AclRevision)
	if err != nil {
		return actor, nil, err
	}
	publication, err := domain.CanonicalID(facts.Document.PublicationEpoch)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources, err = trashSources(actor, space, access, acl, publication)
	if err != nil {
		return actor, nil, err
	}
	return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
		// 标准Mutation若发现回执已被清理必须失败，不能把ACTIVE文档假装新恢复成功。
		return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}, nil
}
