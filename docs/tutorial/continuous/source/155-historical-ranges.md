# services/document/internal/repository/document/historical_ranges.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/155-historical-ranges.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`HistoricalUnitMapping`、`HistoricalOutgoingMappings`、`HistoricalStructureBinding`、`HistoricalManifestBinding`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`2ecfc10c8fe8b83f42dff44caf6908d1fcc03109f94fbf4d70544ec942bb3c61`。[参考实现](../../../../services/document/internal/repository/document/historical_ranges.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
