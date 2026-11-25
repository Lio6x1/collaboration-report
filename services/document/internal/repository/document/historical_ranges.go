package document

import (
	"context"
	"database/sql"
)

type HistoricalUnitMapping struct {
	UnitMapping
	SourceDocument, TargetDocument, Chapter, TargetChapter uint64
	SourceRetired, TargetCreated                           uint64
	TargetKind, SourceKind, VersionKind                    string
}

// 沿指定旧单元读取出边，不遍历整库；跨文档边也必须显式返回给核验器，不能靠JOIN丢掉反例。
func HistoricalOutgoingMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]HistoricalUnitMapping, error) {
	rows, err := tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,
 l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.structure_id,t.structure_id,
 l.source_document_id,l.target_document_id,u.chapter_id,v.chapter_id,COALESCE(u.retired_structure_id,0),v.created_structure_id,
 v.kind,s.source_kind,t.source_kind
 FROM unit_lineage l
 JOIN collaboration_unit u ON u.id=l.source_unit_id AND u.document_id=l.source_document_id
 JOIN collaboration_unit v ON v.id=l.target_unit_id AND v.document_id=l.target_document_id
 JOIN content_version s ON s.id=l.source_version_id AND s.unit_id=l.source_unit_id AND s.document_id=l.source_document_id
 JOIN content_version t ON t.id=l.target_version_id AND t.unit_id=l.target_unit_id AND t.document_id=l.target_document_id
 WHERE l.source_document_id=? AND l.source_unit_id=? ORDER BY l.id LIMIT 10001`, document, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HistoricalUnitMapping{}
	for rows.Next() {
		var item HistoricalUnitMapping
		if err = rows.Scan(&item.Source, &item.Target, &item.SourceVersion, &item.TargetVersion, &item.Operation, &item.Schema, &item.Mapping,
			&item.SourceHash, &item.TargetHash, &item.SourceStructure, &item.TargetStructure, &item.SourceDocument, &item.TargetDocument,
			&item.Chapter, &item.TargetChapter, &item.SourceRetired, &item.TargetCreated, &item.TargetKind, &item.SourceKind, &item.VersionKind); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 读取真实终态和两个原固定工件；不伪造恢复状态、不依赖原操作者当前在线，也不隐式提交。
func HistoricalStructureBinding(ctx context.Context, tx *sql.Tx, operation string) (PendingMainControlBinding, error) {
	fact := PendingMainControlBinding{Kind: "MAIN_CONTROL_RECONCILE", Operation: operation}
	err := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,
 original.payload,fixed.payload FROM operation_receipt o
 JOIN durable_task original ON original.kind='MAIN_CONTROL_CONTEXT' AND original.business_key=o.operation_id AND original.operation_id=o.operation_id AND original.state='SUCCEEDED'
 JOIN durable_task fixed ON fixed.kind='MAIN_COMMIT_FIXED' AND fixed.business_key=o.operation_id AND fixed.operation_id=o.operation_id AND fixed.state='SUCCEEDED'
 WHERE o.operation_id=? AND o.kind='STRUCTURE_CHANGE'`, operation).Scan(
		&fact.Actor, &fact.ReceiptKind, &fact.State, &fact.Fingerprint, &fact.Target, &fact.Result, &fact.Original, &fact.Payload)
	return fact, err
}

func HistoricalManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure uint64, operation string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m
 JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id
 WHERE m.id=? AND m.document_id=? AND m.structure_id=? AND m.source_operation_id=? AND s.source_operation_id=?`, manifest, document, structure, operation, operation).Scan(&count)
	return count == 1, err
}
