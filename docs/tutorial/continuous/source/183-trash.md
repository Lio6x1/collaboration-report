# services/document/internal/repository/document/trash.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/183-trash.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`TrashDocumentRow`、`TrashKnowledgeBaseRow`、`TrashSpaceRow`、`TrashKBRole`、`TrashCandidates`、`TrashSourceHighWater`、`TrashRestoreOriginalReceipt`、`RestoreTrashDocument`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

// 只读真实回收事实，保留SQL未知；这里不借普通ACTIVE入口，也不隐式开启事务。
func TrashDocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.TrashFacts, error) {
	var facts domain.TrashFacts
	d := &facts.Document
	var structure, style, manifest sql.NullString
	var purge sql.NullTime
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT "+publicationDocumentColumns+",purge_after,UTC_TIMESTAMP(6) FROM document WHERE id=?"+suffix, id).
		Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId, &d.Lifecycle, &d.MainLocked,
			&structure, &style, &manifest, &d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision,
			&d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision, &facts.Sequence, &purge, &facts.Now)
	if errors.Is(err, sql.ErrNoRows) {
		return facts, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	if err != nil {
		return facts, err
	}
	d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId = structure.String, style.String, manifest.String
	if purge.Valid {
		facts.PurgeAfter = &purge.Time
	}
	return facts, nil
}

func TrashKnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	kb, err := scanPublicationKnowledgeBase(q.QueryRowContext(ctx, "SELECT "+publicationKnowledgeBaseColumns+" FROM knowledge_base WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return kb, domain.Violation(domain.TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
	}
	return kb, err
}

func TrashSpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	space, err := scanPublicationSpace(q.QueryRowContext(ctx, "SELECT "+publicationSpaceColumns+" FROM space WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return space, domain.Violation(domain.TemporarilyUnavailable, "TRASH_SPACE_UNKNOWN")
	}
	return space, err
}

func TrashKBRole(ctx context.Context, q Queryer, id, actor uint64, lock bool) (string, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var role, state string
	err := q.QueryRowContext(ctx, "SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?"+suffix, id, actor).Scan(&role, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if state != "ACTIVE" {
		return "", nil
	}
	return role, nil
}

// SQL只选择现今有管理资格的候选，最多一页加一个lookahead；拒绝项不消耗分页位置。
// org普通成员仍须是当前Doc负责人或当前ACTIVE KB ADMIN，content_grant不参与这个范围。
func TrashCandidates(ctx context.Context, q Queryer, space, actor uint64, inherited bool, last uint64, budget int) ([]uint64, error) {
	if budget < 2 || budget > 101 {
		return nil, domain.Violation(domain.InvalidRequest, "TRASH_PAGE_BUDGET_INVALID")
	}
	statement := "SELECT d.id FROM document d WHERE d.space_id=? AND d.lifecycle='TRASHED' AND d.id>?"
	args := []any{space, last}
	if !inherited {
		statement += " AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id WHERE k.id=d.knowledge_base_id AND k.space_id=d.space_id AND k.lifecycle='ACTIVE' AND m.user_id=? AND m.role='ADMIN' AND m.state='ACTIVE'))"
		args = append(args, actor, actor)
	}
	statement += " ORDER BY d.id LIMIT ?"
	args = append(args, budget)
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0, budget)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func TrashSourceHighWater(ctx context.Context, q Queryer, key string) (uint64, error) {
	var epoch uint64
	err := q.QueryRowContext(ctx, "SELECT max_epoch FROM access_gate WHERE scope_key=?", key).Scan(&epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return epoch, err
}

func TrashRestoreOriginalReceipt(ctx context.Context, q Queryer, operation string) (domain.TrashRestoreReceipt, error) {
	receipt := domain.TrashRestoreReceipt{Operation: operation}
	var path sql.NullString
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.path')) FROM operation_receipt WHERE operation_id=?", operation).
		Scan(&receipt.Actor, &receipt.Kind, &receipt.State, &path)
	receipt.Path = path.String
	return receipt, err
}

// 调用方已按来源gate→KB→Doc→Space顺序锁定且复核当前资格；只改可变控制事实，
// MAIN/branch的固定结构、样式、manifest和正文不变。撤下在冻结schema中存为UNPUBLISHED。
func RestoreTrashDocument(ctx context.Context, tx *sql.Tx, id uint64, expected string, next domain.TrashRestoreRevisions) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='ACTIVE',visibility_policy='COLLABORATORS',
		current_release_id=NULL,publication_state='UNPUBLISHED',deleted_at=NULL,purge_after=NULL,
		control_revision=?,acl_revision=?,publication_epoch=?,visibility_revision=?,event_seq=?
		WHERE id=? AND lifecycle='TRASHED' AND control_revision=? AND purge_after>UTC_TIMESTAMP(6)`,
		next.Control, next.ACL, next.Publication, next.Visibility, next.Sequence, id, expected)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "TRASH_RESTORE_STATE_CHANGED")
	}
	// 仍ACTIVE或撤权中的原直接授权全部关闭，绝不把回收前授权重新解释成恢复资格。
	_, err = tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state<>'CLOSED'", id)
	return err
}
````

UTF-8/LF全文SHA-256：`d3514b7b1c9fd277e27bc47a1b756a25d67bdf82a478bf88dd6bf74c485d236d`。[参考实现](../../../../services/document/internal/repository/document/trash.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
