# services/document/internal/repository/document/visibility.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/184-visibility.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`UpdateVisibility`、`VisibilityDocumentIDs`、`VisibilityIndexFacts`、`LockedVisibilityIndexFacts`、`AdvanceVisibilitySequence`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

func UpdateVisibility(ctx context.Context, tx *sql.Tx, id uint64, isKnowledgeBase bool, policy string) error {
	table := "document"
	if isKnowledgeBase {
		table = "knowledge_base"
	}
	_, err := tx.ExecContext(ctx, "UPDATE "+table+" SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?", policy, id)
	return err
}

// 调用方已经持有kb-acl排他gate及KB行锁，且尚未做一致性快照读取。
// 创建/发布/生命周期变更同样先取得该gate，因此此时首次快照固定完整范围，
// 后续再按数值主键逐个当前读加锁；不能用二级索引的扫描锁序冒充文档主键锁序。
// 非ACTIVE/未发布行仍带KB修订的权威摘要，不能筛掉它们或使用生命周期入口的400条限制。
func VisibilityDocumentIDs(ctx context.Context, tx *sql.Tx, knowledgeBase uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM document WHERE knowledge_base_id=? ORDER BY id", knowledgeBase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type VisibilityIndexFacts struct {
	Document, Sequence, Release, PublicationEpoch uint64
}

func LockedVisibilityIndexFacts(ctx context.Context, tx *sql.Tx, document, knowledgeBase uint64) (VisibilityIndexFacts, error) {
	facts := VisibilityIndexFacts{Document: document}
	var release, kb sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT event_seq,CAST(current_release_id AS CHAR),publication_epoch,CAST(knowledge_base_id AS CHAR) FROM document WHERE id=? FOR UPDATE", document).
		Scan(&facts.Sequence, &release, &facts.PublicationEpoch, &kb)
	if err != nil {
		return facts, err
	}
	if knowledgeBase != 0 && (!kb.Valid || kb.String != domain.TextID(knowledgeBase)) {
		return facts, domain.Violation(domain.StateConflict, "VISIBILITY_DOCUMENT_BINDING_CHANGED")
	}
	if release.Valid {
		facts.Release, err = domain.CanonicalID(release.String)
	}
	return facts, err
}

// 只推进索引水位，保留当前正式指针和publication_epoch；调用方仍持有同一文档行锁。
func AdvanceVisibilitySequence(ctx context.Context, tx *sql.Tx, document, previous, next uint64) error {
	result, err := tx.ExecContext(ctx, "UPDATE document SET event_seq=? WHERE id=? AND event_seq=?", next, document, previous)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.StateConflict, "VISIBILITY_EVENT_SEQUENCE_CHANGED")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`9d5213cf53ed313d6ce68e07e54f4e8e585a630c97c5aac6668b7dd76aa52cf9`。[参考实现](../../../../services/document/internal/repository/document/visibility.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
