// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"sort"
	"strconv"
	"strings"
)

// public_reading.go：保留原实现，按业务合并文件。

type PublicReading struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	outputs    *Outputs
	pagination *Pagination
	validator  *codec.Validator
}

func NewPublicReading(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, pagination *Pagination, validator *codec.Validator) *PublicReading {
	return &PublicReading{db: db, identity: identity, access: access, outputs: outputs, pagination: pagination, validator: validator}
}

func (s *PublicReading) facts(ctx context.Context, id uint64) (domain.PublicationFacts, error) {
	// 乐观快照只决定需要哪些真实证明。最终输出仍按来源→Doc→KB→Space当前读复核。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.PublicationFacts{}, err
	}
	defer tx.Rollback()
	facts, err := repository.PublicationFactsRow(ctx, tx, id, false)
	if err != nil {
		return facts, err
	}
	return facts, tx.Commit()
}

func publicationSources(facts domain.PublicationFacts) ([]*pb.SourceEpoch, error) {
	document := facts.Document
	acl, err := domain.CanonicalID(document.AclRevision)
	if err != nil {
		return nil, err
	}
	epoch, err := domain.CanonicalID(document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	sources := []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + document.Id, Epoch: acl}, {ScopeKey: "publication:" + document.Id, Epoch: epoch}}
	if facts.KnowledgeBase != nil {
		revision, err := domain.CanonicalID(facts.KnowledgeBase.AclRevision)
		if err != nil {
			return nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + facts.KnowledgeBase.Id, Epoch: revision})
	}
	return sources, nil
}

func (s *PublicReading) audience(ctx context.Context, actor *Actor, facts domain.PublicationFacts, policy string) ([]*pb.IdentityAdmission, []*pb.SourceEpoch, error) {
	proofs := []*pb.IdentityAdmission{}
	member, internal := false, false
	if actor != nil {
		proofs = append(proofs, actor.Proof)
		switch policy {
		case "SPACE_MEMBERS":
			space, err := domain.CanonicalID(facts.Space.Id)
			if err != nil {
				return nil, nil, err
			}
			_, proof, err := s.access.SpaceIdentity(ctx, *actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_PUBLIC_DOCUMENT", 0)
			if err != nil {
				return nil, nil, err
			}
			proofs = append(proofs, proof)
			member = true
		case "COLLABORATORS":
			id, _ := domain.CanonicalID(facts.Document.Id)
			access, err := s.access.Authorize(ctx, *actor, id, "READ_PUBLIC_DOCUMENT")
			if err != nil {
				return nil, nil, err
			}
			proofs = append(proofs, access.Proofs...)
			internal = access.Permissions.Read
		}
	}
	if err := domain.ValidatePublicAudience(policy, actor != nil, member, internal); err != nil {
		return nil, nil, err
	}
	identitySources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, err
	}
	sources, err := publicationSources(facts)
	if err != nil {
		return nil, nil, err
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
	}
	return proofs, append(identitySources, sources...), nil
}

func (s *PublicReading) Current(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (any, error) {
	if handler == "" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_REQUIRED")
	}
	facts, err := s.facts(ctx, id)
	if err != nil {
		return nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, err
	}
	if !domain.HasCurrentPublication(facts, policy) {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLIC_DOCUMENT_NOT_FOUND")
	}
	_, sources, err := s.audience(ctx, actor, facts, policy)
	if err != nil {
		return nil, err
	}
	// Edge实例查询是跨进程调用，必须位于SQL锁外；缺失/变化不能用匿名路径绕过。
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	current, err := repository.PublicationFactsRow(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if publicFactsBinding(current) != publicFactsBinding(facts) {
		return nil, domain.Violation(domain.StateConflict, "PUBLIC_DOCUMENT_CHANGED")
	}
	if !domain.HasCurrentPublication(current, policy) {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLIC_DOCUMENT_NOT_FOUND")
	}
	release, err := domain.CanonicalID(*current.Document.CurrentReleaseId)
	if err != nil {
		return nil, err
	}
	epoch, err := domain.CanonicalID(current.Document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	value, _, err := s.fixed(ctx, tx, current.Document, release)
	if err != nil {
		return nil, err
	}
	registration, err := s.outputs.RegisterPublication(ctx, tx, sources, id, epoch, release, handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return value, nil
}

// public_reading_binding.go：保留原实现，按业务合并文件。

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// 只绑定正式指针与当前授权事实。管理标题、MAIN/结构/control随后变化不改写固定正式版。
func publicFactsBinding(facts domain.PublicationFacts) string {
	d := facts.Document
	parts := []string{d.Id, d.SpaceId, optionalText(d.KnowledgeBaseId), d.Lifecycle, d.AclRevision, d.VisibilityPolicy, d.VisibilityRevision, d.PublicationEpoch, d.PublicationState, optionalText(d.CurrentReleaseId), domain.TextID(facts.Sequence), facts.Space.Lifecycle, facts.Space.Kind, optionalText(facts.Space.OwnerUserId), optionalText(facts.Space.OrganizationId)}
	if facts.KnowledgeBase != nil {
		kb := facts.KnowledgeBase
		parts = append(parts, kb.Id, kb.SpaceId, kb.Lifecycle, kb.AclRevision, kb.VisibilityPolicy, kb.VisibilityRevision)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(digest[:])
}

// public_reading_fixed.go：保留原实现，按业务合并文件。

// 所有元数据来自release自己的manifest。阅读不创建新版本，也不调用Node读取活跃MAIN。
func (s *PublicReading) fixed(ctx context.Context, tx *sql.Tx, document dto.Document, release uint64) (dto.PublicDocument, []*pb.ManifestUnit, error) {
	value := dto.PublicDocument{DocumentId: document.Id, PublicationEpoch: document.PublicationEpoch, Units: []dto.FixedContent{}}
	id, err := domain.CanonicalID(document.Id)
	if err != nil {
		return value, nil, err
	}
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, release)
	if err != nil {
		return value, nil, err
	}
	value.ReleaseId, value.Title, value.ManifestId = fixed.Release.Id, fixed.Title, fixed.Release.ManifestId
	value.Structure, err = repository.StructureRow(ctx, tx, fixed.Structure, document.Id)
	if err != nil {
		return value, nil, err
	}
	value.Style, err = repository.StyleRow(ctx, tx, fixed.Style, document.Id)
	if err != nil {
		return value, nil, err
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return value, nil, err
	}
	if err = validateReadingUnits(value.Structure, units); err != nil {
		return value, nil, err
	}
	wire := []*pb.ManifestUnit{}
	var total uint64
	structure, _ := domain.CanonicalID(fixed.Structure)
	for _, unit := range units {
		unitID, err := domain.CanonicalID(unit["unit_id"])
		if err != nil {
			return value, nil, err
		}
		version, err := domain.CanonicalID(unit["content_version_id"])
		if err != nil {
			return value, nil, err
		}
		content, err := repository.VersionContent(ctx, tx, version, id)
		if err != nil {
			return value, nil, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return value, nil, err
		}
		if err = validateFixedMergeContent(s.validator, content, id, unitID, structure); err != nil {
			return value, nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_FIXED_CONTENT_INVALID")
		}
		// 总预算在加法前检查；即使未来历史引用异常，也不能整数回绕或无限装配正文。
		if content.ByteSize > 24<<20 || total > (24<<20)-content.ByteSize {
			return value, nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_FIXED_CONTENT_LIMIT")
		}
		total += content.ByteSize
		body := dto.FixedContent{Id: domain.TextID(version), DocumentId: document.Id, UnitId: domain.TextID(unitID), StructureId: domain.TextID(content.StructureId), SchemaVersion: content.SchemaVersion, CanonicalizationVersion: content.CanonicalizationVersion, BodyHash: hex.EncodeToString(content.BodyHash), ByteSize: domain.TextID(content.ByteSize), ResourceIds: []string{}}
		if err = json.Unmarshal(content.BodyJsonUtf8, &body.BodyJson); err != nil {
			return value, nil, err
		}
		if err = json.Unmarshal(content.ProvenanceJsonUtf8, &body.Provenance); err != nil {
			return value, nil, err
		}
		for _, resource := range content.ResourceIds {
			body.ResourceIds = append(body.ResourceIds, domain.TextID(resource))
		}
		value.Units = append(value.Units, body)
		wire = append(wire, &pb.ManifestUnit{UnitId: unitID, ContentVersionId: version, Content: content})
	}
	return value, wire, nil
}

// public_reading_history.go：保留原实现，按业务合并文件。

func (s *PublicReading) History(ctx context.Context, actor Actor, id uint64, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_RELEASE_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	limit, _, err := s.pagination.Parse(limitText, "", "")
	if err != nil {
		return nil, nil, err
	}
	sources := SourcesWire(access.Sources)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Key < sources[j].Key })
	// 每次新证明的随机admission号不属于游标；真实会话/来源、页大小和发布代次必须属于绑定。
	raw, _ := json.Marshal([]any{domain.TextID(actor.ID), hex.EncodeToString(actor.Proof.SessionRef), domain.TextID(id), strconv.Itoa(limit), access.Document.PublicationEpoch, sources})
	digest := sha256.Sum256(raw)
	binding := "release-history:" + hex.EncodeToString(digest[:])
	_, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var before uint64
	if last != "" {
		before, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.PublicationEpoch != access.Document.PublicationEpoch {
			return nil, domain.Violation(domain.StateConflict, "RELEASE_HISTORY_CHANGED")
		}
		items, err := repository.ReleaseRows(ctx, tx, id, before, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.ReleasePage{Items: items, HasMore: len(items) > limit}
		if page.HasMore {
			page.Items = items[:limit]
			page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
		}
		return page, nil
	}
	return access.Sources, run, nil
}

func (s *PublicReading) Historical(ctx context.Context, actor Actor, id, release uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_RELEASE_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		value, _, err := s.fixed(ctx, tx, current, release)
		return value, err
	}
	return access.Sources, run, nil
}

// public_snapshot.go：保留原实现，按业务合并文件。

// Search是受信任的候选索引者，visible表示当前正式版可索引，COLLABORATORS仍须最后由Doc授权。
// 请求release/seq只是旧事件提示；绝不能用它选择旧历史正文或把未知Doc当成删除。
func (s *PublicReading) Snapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	if input == nil || input.DocumentId == 0 {
		return nil, domain.Violation(domain.InvalidRequest, "PUBLIC_SNAPSHOT_QUERY_INVALID")
	}
	facts, err := s.facts(ctx, input.DocumentId)
	if err != nil {
		return nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, err
	}
	visible := domain.HasCurrentPublication(facts, policy)
	var options *sql.TxOptions
	if !visible {
		// 隐藏回复没有正文，也不登记用户输出；同一只读一致快照足以绑定已提交事实。
		// 不抢Doc/KB行锁，避免与持有KB→Doc锁序的政策fanout形成循环等待。
		// 旧快照若低于消息最低水位，Search仍拒绝并重读；刚变可见则沿下面绑定检查重试。
		options = &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if visible {
		sources, err := publicationSources(facts)
		if err != nil {
			return nil, err
		}
		if err = LockSources(ctx, tx, sources); err != nil {
			return nil, err
		}
	}
	current, err := repository.PublicationFactsRow(ctx, tx, input.DocumentId, visible)
	if err != nil {
		return nil, err
	}
	currentPolicy, err := domain.PublicPolicy(current)
	if err != nil {
		return nil, err
	}
	// 无正文的不可见事实可在关闭中返回。若刚转成可见，须重试完整来源路径，不能倒置锁序。
	if publicFactsBinding(current) != publicFactsBinding(facts) || (!visible && domain.HasCurrentPublication(current, currentPolicy)) {
		return nil, domain.Violation(domain.StateConflict, "PUBLIC_SNAPSHOT_CHANGED")
	}
	epoch, err := domain.CanonicalID(current.Document.PublicationEpoch)
	if err != nil {
		return nil, err
	}
	visibility, err := domain.CanonicalID(current.Document.VisibilityRevision)
	if err != nil {
		return nil, err
	}
	result := &pb.PublicSnapshot{DocumentId: input.DocumentId, AggregateSeq: current.Sequence, PublicationEpoch: epoch, VisibilityPolicy: currentPolicy, VisibilityRevision: visibility, Units: []*pb.ManifestUnit{}}
	if current.KnowledgeBase != nil {
		result.KnowledgeBaseVisibilityRevision, err = domain.CanonicalID(current.KnowledgeBase.VisibilityRevision)
		if err != nil {
			return nil, err
		}
	}
	if visible {
		release, err := domain.CanonicalID(*current.Document.CurrentReleaseId)
		if err != nil {
			return nil, err
		}
		fixed, units, err := s.fixed(ctx, tx, current.Document, release)
		if err != nil {
			return nil, err
		}
		structure, err := domain.CanonicalID(fixed.Structure.Id)
		if err != nil {
			return nil, err
		}
		result.Visible, result.ReleaseId, result.Title, result.StructureId, result.Units = true, release, fixed.Title, structure, units
	}
	if proto.Size(result) > 32<<20 {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SNAPSHOT_TRANSFER_LIMIT")
	}
	return result, tx.Commit()
}
