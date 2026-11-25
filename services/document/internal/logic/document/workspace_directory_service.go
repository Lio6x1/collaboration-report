// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// directory_access.go：保留原实现，按业务合并文件。

type Directory struct {
	db             *sql.DB
	identity       *Identity
	access         *Access
	knowledgeBases *KnowledgeBases
	pagination     *Pagination
	directory      CollaboratorDirectoryReader
}

type CollaboratorDirectoryReader interface {
	DirectoryReader
	Collaborator(context.Context, *pb.IdentityRevalidation) (dto.DocumentCollaborator, error)
}

func NewDirectory(db *sql.DB, identity *Identity, access *Access, knowledgeBases *KnowledgeBases, pagination *Pagination, directory CollaboratorDirectoryReader) *Directory {
	return &Directory{db: db, identity: identity, access: access, knowledgeBases: knowledgeBases, pagination: pagination, directory: directory}
}
func (s *Directory) Identity(ctx context.Context, actor Actor, spaceID uint64, purpose string) (dto.Space, *pb.IdentityAdmission, bool, error) {
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)
	if err == nil {
		return space, proof, true, nil
	}
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
		return space, nil, false, err
	}
	space, proof, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)
	return space, proof, false, err
}
func KnowledgeBaseVisibilitySources(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, []*pb.SourceEpoch, error) {
	if inherited {
		return true, nil, nil
	}
	visible, err := repository.ExplicitKBVisibility(ctx, q, kb, actor)
	if err != nil || visible {
		return visible, nil, err
	}
	// 库名也可能凭文档授权可见；输出必须绑定每条实际Doc ACL，不能仅登记KB来源。
	facts, err := repository.DerivedDirectorySources(ctx, q, kb, actor)
	var sources []*pb.SourceEpoch
	for _, fact := range facts {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(fact.ID), Epoch: fact.Epoch})
	}
	if len(sources) > 400 {
		return false, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_SOURCE_BUDGET_EXCEEDED")
	}
	return len(sources) > 0, sources, err
}
func knowledgeBaseVisible(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, error) {
	visible, _, err := KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)
	return visible, err
}

// directory_documents.go：保留原实现，按业务合并文件。

func (s *Directory) Documents(ctx context.Context, actor Actor, spaceID, kbID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	if kbID != 0 {
		kb, err := repository.KnowledgeBaseRow(ctx, s.db, kbID, false)
		if err != nil {
			return nil, nil, err
		}
		spaceID, _ = domain.CanonicalID(kb.SpaceId)
	}
	_, proof, _, err := s.Identity(ctx, actor, spaceID, "LIST_DOCUMENTS")
	if err != nil {
		return nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	binding := "documents:" + domain.TextID(actor.ID) + ":" + domain.TextID(spaceID) + ":" + domain.TextID(kbID)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	candidates, err := repository.DocumentCandidates(ctx, s.db, spaceID, kbID, last, limit*10+1)
	if err != nil {
		return nil, nil, err
	}
	page := dto.DocumentPage{Items: []dto.Document{}}
	sources, _ := MergeSources(actor.Proof, proof)
	bySource := map[string]bool{}
	for _, source := range sources {
		bySource[source.ScopeKey] = true
	}
	var accesses []DocumentAccess
	scanned := last
	for index, id := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(id)
		access, err := s.access.Authorize(ctx, actor, id, "LIST_DOCUMENTS")
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound) {
				continue
			}
			return nil, nil, err
		}
		if len(page.Items) == limit {
			page.HasMore = true
			scanned = page.Items[limit-1].Id
			break
		}
		page.Items = append(page.Items, access.Document)
		accesses = append(accesses, access)
		for _, source := range access.Sources {
			if !bySource[source.ScopeKey] {
				bySource[source.ScopeKey] = true
				sources = append(sources, source)
			}
		}
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		for _, access := range accesses {
			if _, err := ValidateAccess(ctx, tx, access); err != nil {
				return nil, err
			}
		}
		return page, nil
	}
	return sources, run, nil
}

// directory_knowledge_base.go：保留原实现，按业务合并文件。

type KnowledgeBaseReading struct {
	KnowledgeBase dto.KnowledgeBase
	Actions       []string
}

func (s *Directory) KnowledgeBase(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
	if err != nil {
		return nil, nil, err
	}
	spaceID, _ := domain.CanonicalID(kb.SpaceId)
	_, proof, inherited, err := s.Identity(ctx, actor, spaceID, "READ_KB")
	if err != nil {
		return nil, nil, err
	}
	visible, derived, err := KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)
	if err != nil {
		return nil, nil, err
	}
	if !visible {
		return nil, nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return nil, nil, err
	}
	revision, _ := domain.CanonicalID(kb.AclRevision)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: revision})
	sources = append(sources, derived...)
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.AclRevision != kb.AclRevision {
			return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
		visible, err := knowledgeBaseVisible(ctx, tx, id, actor.ID, inherited)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
		}
		actions := []string{"READ"}
		managed := inherited
		if !managed {
			role, err := repository.KnowledgeBaseRole(ctx, tx, id, actor.ID)
			managed = err == nil && role == "ADMIN"
		}
		if managed {
			actions = append(actions, "CREATE_DOCUMENT", "RENAME", "CHANGE_CREATION_GRANTS", "DELETE", "CHANGE_VISIBILITY", "VIEW_MEMBERS")
		} else {
			allowed, err := repository.KBCreationAllowed(ctx, tx, id, actor.ID)
			if err != nil {
				return nil, err
			}
			if allowed {
				actions = append(actions, "CREATE_DOCUMENT")
			}
		}
		if accessSpace, err := repository.SpaceRow(ctx, tx, spaceID, false); err == nil && accessSpace.Kind == "PERSONAL" {
			filtered := actions[:0]
			for _, action := range actions {
				if action != "CHANGE_CREATION_GRANTS" {
					filtered = append(filtered, action)
				}
			}
			actions = filtered
		} else if inherited {
			actions = append(actions, "CHANGE_ROLES")
		}
		return KnowledgeBaseReading{KnowledgeBase: current, Actions: actions}, nil
	}
	return sources, run, nil
}
func (s *Directory) KnowledgeBases(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	query, err := directorySearch(search)
	if err != nil {
		return nil, nil, err
	}
	_, proof, inherited, err := s.Identity(ctx, actor, spaceID, "LIST_KB")
	if err != nil {
		return nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	binding := "kb-list:" + domain.TextID(actor.ID) + ":" + domain.TextID(spaceID) + ":" + url.QueryEscape(query)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	candidates, err := repository.KnowledgeBaseCandidates(ctx, s.db, spaceID, last, limit*10+1, query)
	if err != nil {
		return nil, nil, err
	}
	page := dto.KnowledgeBasePage{Items: []dto.KnowledgeBase{}}
	sources, _ := MergeSources(actor.Proof, proof)
	scanned := last
	for index, id := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(id)
		visible, derived, err := KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)
		if err != nil {
			return nil, nil, err
		}
		if !visible {
			continue
		}
		kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		if len(page.Items) == limit {
			page.HasMore = true
			scanned = page.Items[limit-1].Id
			break
		}
		page.Items = append(page.Items, kb)
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: epoch})
		sources = append(sources, derived...)
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		for _, kb := range page.Items {
			id, _ := domain.CanonicalID(kb.Id)
			current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
			if err != nil || current.AclRevision != kb.AclRevision {
				return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
			}
			visible, err := knowledgeBaseVisible(ctx, tx, id, actor.ID, inherited)
			if err != nil {
				return nil, err
			}
			if !visible {
				return nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
			}
		}
		return page, nil
	}
	return sources, run, nil
}

// directory_members.go：保留原实现，按业务合并文件。

// DocumentCollaborators只为当前文档管理者提供已有协作者候选；实际授权/转交仍走各自严格的写入口。
func (s *Directory) DocumentCollaborators(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, "DOCUMENT_COLLABORATORS")
}

// DocumentMentionCandidates只向当前有COMMENT资格的用户展示文档内可提及候选。
// 候选不代表收件资格；通知投递仍独立核验目标对评论原正文的当前读取权限。
func (s *Directory) DocumentMentionCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, "DOCUMENT_MENTION_CANDIDATES")
}

func (s *Directory) documentCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText, purpose string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return nil, nil, err
	}
	var directoryKind string
	switch purpose {
	case "DOCUMENT_COLLABORATORS":
		if !access.Permissions.Manage {
			return nil, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		directoryKind = "document-collaborators"
	case "DOCUMENT_MENTION_CANDIDATES":
		if !access.Permissions.Comment {
			return nil, nil, domain.Violation(domain.PermissionDenied, "COMMENT_REQUIRED")
		}
		directoryKind = "document-mention-candidates"
	default:
		return nil, nil, domain.Violation(domain.InvalidRequest, "DIRECTORY_PURPOSE_INVALID")
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return nil, nil, err
	}
	// ACL变化后不能继续翻旧目录；同一数字ID按稳定keyset扫描，不以昵称或数组下标分页。
	// 提及和管理目录分别绑定用途，COMMENT游标不能拿来续读管理候选。
	binding := directoryKind + ":" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	names := map[string]string{}
	var candidates []uint64
	if space.Kind == "ORGANIZATION" {
		cursor := ""
		for index := 0; index < 100; index++ {
			var members dto.MemberPage
			if err = s.directory.Read(ctx, cookie, "/v1/organizations/"+*space.OrganizationId+"/members?limit=100&cursor="+url.QueryEscape(cursor), &members); err != nil {
				return nil, nil, err
			}
			for _, member := range members.Items {
				if member.State == "ACTIVE" {
					names[member.UserId] = member.DisplayName
				}
			}
			if !members.HasMore {
				break
			}
			if members.NextCursor == nil || index == 99 {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_BUDGET_EXCEEDED")
			}
			cursor = *members.NextCursor
		}
		// 当前组织成员都是已加入的协作者候选；不能要求其先有文档grant，
		// 否则仅凭继承权限管理的新文档将永远无法发出首次授权。
		lastID, err := strconv.ParseUint(last, 10, 64)
		if err != nil {
			return nil, nil, domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		}
		for text := range names {
			user, err := domain.CanonicalID(text)
			if err != nil {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			if user > lastID {
				candidates = append(candidates, user)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		if len(candidates) > limit*10+1 {
			candidates = candidates[:limit*10+1]
		}
	} else {
		candidates, err = repository.DocumentCollaboratorCandidates(ctx, s.db, id, last, limit*10+1)
		if err != nil {
			return nil, nil, err
		}
	}
	page := dto.DocumentCollaboratorPage{Items: []dto.DocumentCollaborator{}}
	scanned := last
	for index, user := range candidates {
		if index >= limit*10 || len(page.Items) == limit {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(user)
		name := names[scanned]
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}
		capability := pb.IdentityCapability_ACTIVE_ACCOUNT
		if space.Kind == "ORGANIZATION" {
			// 旧授权记录仍在也不能把已离开组织的人继续列为可选目标。
			if _, ok := names[scanned]; !ok {
				continue
			}
			organization, _ := domain.CanonicalID(*space.OrganizationId)
			target = &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}
			capability = pb.IdentityCapability_ORGANIZATION_MEMBER
		}
		proof, err := s.identity.Target(ctx, actor, capability, target, purpose, id)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Kind == domain.PermissionDenied {
				continue
			}
			return nil, nil, err
		}
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
		if space.Kind == "PERSONAL" {
			// 候选已由本库真实关系和本用途的管理/评论资格确定；Account只接收其原身份准入绑定。
			// 目标注销或原证明失效时关闭本次目录，不能退回角色称呼冒充姓名。
			collaborator, err := s.directory.Collaborator(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
			if err != nil {
				return nil, nil, err
			}
			if collaborator.UserId != scanned || collaborator.DisplayName == "" {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			name = collaborator.DisplayName
		}
		// 每个返回账号都绑定真实身份/组织成员代次，使注销或退出组织可以关闭在途输出。
		access.Sources, err = MergeSources(&pb.IdentityAdmission{Sources: access.Sources}, proof)
		if err != nil {
			return nil, nil, err
		}
		if name == "" {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		page.Items = append(page.Items, dto.DocumentCollaborator{UserId: scanned, DisplayName: name})
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 输出登记事务与本用途资格使用相同ACL快照；撤权/转交不能夹在快照与交付之间。
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		return page, nil
	}
	return access.Sources, run, nil
}

func (s *Directory) KnowledgeBaseMembers(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	query, err := directorySearch(search)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.knowledgeBases.Manage(ctx, actor, id, "KB_MEMBERS", false)
	if err != nil {
		return nil, nil, err
	}
	binding := "kb-members:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.KB.AclRevision + ":" + url.QueryEscape(query)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	names := map[string]string{}
	if access.Space.Kind == "ORGANIZATION" {
		cursor := ""
		for index := 0; index < 100; index++ {
			var members dto.MemberPage
			if err = s.directory.Read(ctx, cookie, "/v1/organizations/"+*access.Space.OrganizationId+"/members?limit=100&cursor="+url.QueryEscape(cursor), &members); err != nil {
				return nil, nil, err
			}
			for _, member := range members.Items {
				if member.State == "ACTIVE" {
					names[member.UserId] = member.DisplayName
				}
			}
			if !members.HasMore {
				break
			}
			if members.NextCursor == nil || index == 99 {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_BUDGET_EXCEEDED")
			}
			cursor = *members.NextCursor
		}
	} else {
		var me dto.User
		if err = s.directory.Read(ctx, cookie, "/v1/account/me", &me); err != nil {
			return nil, nil, err
		}
		names[domain.TextID(actor.ID)] = me.DisplayName
	}
	candidates, err := repository.KBMemberCandidates(ctx, s.db, id, last, limit*10+1)
	if err != nil {
		return nil, nil, err
	}
	page := dto.MemberPage{Items: []dto.Member{}}
	scanned := last
	// 稳定扫描后再筛掉历史成员；预算耗尽仍返回游标，空的可见页不等于目录结束。
	for index, member := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = member.UserId
		name, ok := names[member.UserId]
		if !ok || (query != "" && !strings.Contains(strings.ToLower(name), query) && strings.TrimLeft(query, "0") != member.UserId) {
			continue
		}
		if len(page.Items) == limit {
			page.HasMore = true
			scanned = page.Items[limit-1].UserId
			break
		}
		if access.Space.Kind == "ORGANIZATION" {
			user, _ := domain.CanonicalID(member.UserId)
			proof, err := s.knowledgeBases.Target(ctx, actor, access, user, "KB_MEMBERS")
			if err != nil {
				var violation domain.RuleViolation
				if errors.As(err, &violation) && violation.Kind == domain.PermissionDenied {
					continue
				}
				return nil, nil, err
			}
			if err = s.identity.Revalidate(ctx, proof); err != nil {
				return nil, nil, err
			}
			// 成员名也依赖该目标的真实身份/成员代次，不能仅用查看者自己的证明守护。
			access.Sources, err = MergeSources(&pb.IdentityAdmission{Sources: access.Sources}, proof)
			if err != nil {
				return nil, nil, err
			}
		}
		member.DisplayName = name
		page.Items = append(page.Items, member)
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateKnowledgeBase(ctx, tx, access, access.KB.AclRevision); err != nil {
			return nil, err
		}
		for _, member := range page.Items {
			revision, state, err := repository.LockedKBMember(ctx, tx, id, member.UserId)
			if err != nil || state != "ACTIVE" || revision != member.Revision {
				return nil, domain.Violation(domain.StateConflict, "KB_MEMBER_IDENTITY_CHANGED")
			}
		}
		return page, nil
	}
	return access.Sources, run, nil
}

// 限制搜索长度，字面查询避免百分号被当作SQL通配符。
func directorySearch(search []string) (string, error) {
	query := ""
	if len(search) > 0 {
		query = strings.ToLower(strings.TrimSpace(search[0]))
	}
	if len([]rune(query)) > 100 {
		return "", domain.Violation(domain.InvalidRequest, "DIRECTORY_QUERY_INVALID")
	}
	return query, nil
}
