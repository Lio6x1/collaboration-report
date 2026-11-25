# services/document/internal/repository/document/manifest_context_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/159-manifest-context-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`MainControlManifestBinding`、`BootstrapMainContext`、`UnitMapping`、`StructureUnitMappings`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// control_manifest.go：保留原实现，按业务合并文件。

func MainControlManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure, style uint64, operation string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation).Scan(&count)
	return count == 1, err
}

// main_context.go：保留原实现，按业务合并文件。

// 与bootstrap原文档锁共用事务，MAIN房间的结构/样式/control必须来自同一权威时刻。
func BootstrapMainContext(ctx context.Context, tx *sql.Tx, document uint64) (map[string]string, error) {
	var structure, style, control, manifest string
	err := tx.QueryRowContext(ctx, "SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHERE id=? FOR UPDATE", document).Scan(&structure, &style, &control, &manifest)
	if err != nil {
		return nil, err
	}
	epoch, err := MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	// 改名或转移负责人不重播种可靠正文；这里交付真实MAIN物理上下文，管理修订仍留在Doc元数据。
	control = domain.TextID(epoch)
	return map[string]string{"structure_id": structure, "style_id": style, "control_revision": control, "manifest_id": manifest}, err
}

// unit_lineage.go：保留原实现，按业务合并文件。

type UnitMapping struct {
	Source, Target, SourceVersion, TargetVersion, SourceStructure, TargetStructure uint64
	Operation, Schema                                                              string
	Mapping, SourceHash, TargetHash                                                []byte
}

func StructureUnitMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]UnitMapping, error) {
	rows, err := tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.structure_id,t.structure_id
 FROM unit_lineage l JOIN content_version s ON s.id=l.source_version_id AND s.unit_id=l.source_unit_id AND s.document_id=l.source_document_id
 JOIN content_version t ON t.id=l.target_version_id AND t.unit_id=l.target_unit_id AND t.document_id=l.target_document_id
 WHERE l.source_document_id=? AND l.target_document_id=? AND l.target_unit_id=? ORDER BY l.id`, document, document, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []UnitMapping{}
	for rows.Next() {
		var item UnitMapping
		if err = rows.Scan(&item.Source, &item.Target, &item.SourceVersion, &item.TargetVersion, &item.Operation, &item.Schema, &item.Mapping, &item.SourceHash, &item.TargetHash, &item.SourceStructure, &item.TargetStructure); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
````

UTF-8/LF全文SHA-256：`11000e5b9b6ee3d1533832b4f35636aa8f8daf11d7d3bcac67189747f0492de2`。[参考实现](../../../../services/document/internal/repository/document/manifest_context_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
