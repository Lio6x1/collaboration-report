# services/document/internal/repository/document/public_readings_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/165-public-readings-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PublicationFactsRow`、`publicationScanner`、`scanPublicationDocument`、`scanPublicationKnowledgeBase`、`scanPublicationSpace`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`27a321e3ad0d7df1a3f6a85c37acee003c3621e7560a5f8897c87dde883dfac3`。[参考实现](../../../../services/document/internal/repository/document/public_readings_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
