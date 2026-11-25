// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

// public_facts.go：保留原实现，按业务合并文件。

// 这里保留实际生命周期，供权威快照明确返回不可见状态；不放宽内部DocumentRow的ACTIVE要求。
// lock=true的最终读取使用当前读，不能在等待来源锁后复用事务早期的可见策略。
func PublicationFactsRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.PublicationFacts, error) {
	var facts domain.PublicationFacts
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	document, sequence, err := scanPublicationDocument(q.QueryRowContext(ctx,
		"SELECT "+publicationDocumentColumns+" FROM document WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return facts, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	if err != nil {
		return facts, err
	}
	facts.Document, facts.Sequence = document, sequence
	d := &facts.Document
	if d.KnowledgeBaseId != nil {
		kb, err := scanPublicationKnowledgeBase(q.QueryRowContext(ctx,
			"SELECT "+publicationKnowledgeBaseColumns+" FROM knowledge_base WHERE id=?"+suffix, *d.KnowledgeBaseId))
		if err != nil || kb.SpaceId != d.SpaceId {
			return facts, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
		}
		facts.KnowledgeBase = &kb
	}
	facts.Space, err = scanPublicationSpace(q.QueryRowContext(ctx,
		"SELECT "+publicationSpaceColumns+" FROM space WHERE id=?"+suffix, d.SpaceId))
	if err != nil {
		return facts, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
	}
	return facts, nil
}

// public_facts_scan.go：保留原实现，按业务合并文件。

// 单项当前读和搜索批量快照共享列与扫描顺序，避免优化查询时漏掉权限或正式水位。
// 这里仅转换私有SQL结果，不判断ACTIVE/留存资格，也不创建或提交事务。
const publicationDocumentColumns = `CAST(id AS CHAR),title,CAST(space_id AS CHAR),
	CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,
	CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(current_manifest_id AS CHAR),
	CAST(current_release_id AS CHAR),publication_state,CAST(control_revision AS CHAR),
	CAST(acl_revision AS CHAR),CAST(publication_epoch AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR),event_seq`

const publicationKnowledgeBaseColumns = `CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,
	CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR)`

const publicationSpaceColumns = `CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),
	CAST(organization_id AS CHAR),lifecycle`

type publicationScanner interface {
	Scan(...any) error
}

func scanPublicationDocument(row publicationScanner) (dto.Document, uint64, error) {
	var d dto.Document
	var sequence uint64
	var structure, style, manifest sql.NullString
	err := row.Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId,
		&d.Lifecycle, &d.MainLocked, &structure, &style, &manifest,
		&d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision,
		&d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision, &sequence)
	// PURGED/初始化对象合法没有正文指针。公开与搜索先按真实生命周期拒绝，
	// 不能因NULL扫描失败把确定不可见变成依赖未知，也不补造任何固定版本号。
	d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId = structure.String, style.String, manifest.String
	return d, sequence, err
}

func scanPublicationKnowledgeBase(row publicationScanner) (dto.KnowledgeBase, error) {
	var kb dto.KnowledgeBase
	err := row.Scan(&kb.Id, &kb.SpaceId, &kb.Name, &kb.Lifecycle, &kb.AclRevision,
		&kb.VisibilityPolicy, &kb.VisibilityRevision)
	return kb, err
}

func scanPublicationSpace(row publicationScanner) (dto.Space, error) {
	var space dto.Space
	err := row.Scan(&space.Id, &space.Kind, &space.OwnerUserId, &space.OrganizationId, &space.Lifecycle)
	space.Actions = []string{}
	return space, err
}
