package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

func CreateStructureUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64, kind string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)", document, chapter, kind, structure)
}
func RestoreStructureBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,lifecycle='RESTORING' WHERE id=? AND lifecycle='PREPARING'", lineage, branch)
	return err
}
func RetireStructureUnit(ctx context.Context, tx *sql.Tx, document, unit, branch, structure uint64) error {
	// 原身份退役，直接UNIT授权关闭；章级授权由当前真实结构解析，不把旧直接授权扩为整章。
	if _, err := tx.ExecContext(ctx, "UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_structure_id IS NULL", structure, unit, document, branch); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='RETIRED',revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle='ACTIVE'", branch, unit, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND scope_kind='UNIT' AND scope_id=? AND state<>'CLOSED'", document, unit)
	return err
}
func RetireStructureChapter(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) error {
	if _, err := tx.ExecContext(ctx, "UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL", structure, chapter, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND scope_kind='CHAPTER' AND scope_id=? AND state<>'CLOSED'", document, chapter)
	return err
}
func InsertUnitLineage(ctx context.Context, tx *sql.Tx, operation string, document, source, target, sourceVersion, targetVersion uint64, mapping []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,mapping_schema) VALUES(?,?,?,?,?,?,?,?,'full-range-v1')", operation, source, target, document, document, sourceVersion, targetVersion, string(mapping))
	return err
}
func CompleteStructureBranches(ctx context.Context, tx *sql.Tx, document uint64, branches []uint64) error {
	for _, branch := range branches {
		if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE',revision=revision+1 WHERE id=? AND document_id=? AND lifecycle='RESTORING'", branch, document); err != nil {
			return err
		}
	}
	return nil
}
func StructureAuthorizationComplete(ctx context.Context, tx *sql.Tx, operation string, document, epoch uint64, edgeDigest, nodeDigest []byte) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind='AUTHORIZATION_CHANGE'
 WHERE c.revocation_id=? AND c.scope_key=? AND c.epoch=? AND c.state='SUCCEEDED' AND t.state='SUCCEEDED'
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.participant='edge' AND a.phase='OUTPUT_STOPPED' AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.result_digest=?)
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.participant='collaboration' AND a.phase='OUTPUT_STOPPED' AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.result_digest=?)`, operation, "doc-acl:"+domain.TextID(document), epoch, edgeDigest, nodeDigest).Scan(&count)
	return count == 1, err
}
