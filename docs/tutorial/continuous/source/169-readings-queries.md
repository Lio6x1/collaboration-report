# services/document/internal/repository/document/readings_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/169-readings-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`ManifestRevision`、`StructureRow`、`StyleRow`、`ContentRow`、`ManifestUnits`、`InsertReadView`、`LockedReadView`、`ReadViewVersion`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// manifest_read.go：保留原实现，按业务合并文件。

// 清单、结构、样式均不可变；调用方持当前文档和来源锁，历史清理与本次
// 查询共享同一事务边界。不能只按可猜到的manifest ID跨文档取依赖。
func ManifestRevision(ctx context.Context, tx *sql.Tx, document, manifest uint64) (structure, style string, err error) {
	err = tx.QueryRowContext(ctx, "SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?", manifest, document).Scan(&structure, &style)
	return
}

// read.go：保留原实现，按业务合并文件。

func StructureRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StructureRevision, error) {
	var value dto.StructureRevision
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.SchemaVersion) != nil {
		return value, errors.New("STRUCTURE_NOT_FOUND")
	}
	value.Entries = []dto.StructureEntry{}
	rows, err := tx.QueryContext(ctx, "SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry WHERE structure_id=? AND document_id=? ORDER BY parent_sort_key,sort_order", id, document)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry dto.StructureEntry
		if rows.Scan(&entry.NodeId, &entry.ParentNodeId, &entry.ChapterId, &entry.UnitId, &entry.Kind, &entry.Title, &entry.SortOrder, &entry.SplitLevel2) != nil {
			return value, errors.New("STRUCTURE_UNAVAILABLE")
		}
		value.Entries = append(value.Entries, entry)
	}
	return value, rows.Err()
}
func StyleRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StyleRevision, error) {
	var value dto.StyleRevision
	var raw []byte
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.SchemaVersion, &raw) != nil {
		return value, errors.New("STYLE_NOT_FOUND")
	}
	return value, json.Unmarshal(raw, &value.Style)
}
func ContentRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.FixedContent, error) {
	var value dto.FixedContent
	var body, hash, provenance []byte
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_hash,CAST(byte_size AS CHAR),provenance_json FROM content_version WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.UnitId, &value.StructureId, &value.SchemaVersion, &value.CanonicalizationVersion, &body, &hash, &value.ByteSize, &provenance) != nil {
		return value, errors.New("CONTENT_NOT_FOUND")
	}
	if json.Unmarshal(body, &value.BodyJson) != nil || json.Unmarshal(provenance, &value.Provenance) != nil {
		return value, errors.New("CONTENT_SCHEMA_INVALID")
	}
	value.BodyHash = hex.EncodeToString(hash)
	value.ResourceIds = []string{}
	// 引用属于不可变 VERSION，不能从当前草稿、主线清单或资源上传者反推固定正文的引用。
	rows, err := tx.QueryContext(ctx, "SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=? AND purpose='CONTENT' ORDER BY resource_id", document, id)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var resource string
		if err = rows.Scan(&resource); err != nil {
			return value, err
		}
		value.ResourceIds = append(value.ResourceIds, resource)
	}
	return value, rows.Err()
}
func ManifestUnits(ctx context.Context, tx *sql.Tx, manifest string, document uint64) ([]map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id", manifest, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []map[string]string{}
	for rows.Next() {
		var unit, version string
		if rows.Scan(&unit, &version) != nil {
			return nil, errors.New("MANIFEST_UNAVAILABLE")
		}
		units = append(units, map[string]string{"unit_id": unit, "content_version_id": version})
	}
	return units, rows.Err()
}
func InsertReadView(ctx context.Context, tx *sql.Tx, view domain.ReadView) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO read_view(view_id,document_id,manifest_id,admission_id,expires_at,state) VALUES(?,?,?,?,?,'OPEN')", view.ID, view.Document, view.Manifest, view.Admission, view.Expires)
	return err
}
func LockedReadView(ctx context.Context, tx *sql.Tx, view string, document uint64) (domain.FixedReadView, error) {
	var value domain.FixedReadView
	err := tx.QueryRowContext(ctx, "SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id WHERE v.view_id=? AND v.document_id=? FOR UPDATE", view, document).Scan(&value.Manifest, &value.Actor, &value.Session, &value.State, &value.AdmissionState, &value.Expires)
	return value, err
}
func ReadViewVersion(ctx context.Context, tx *sql.Tx, manifest, document, unit uint64) (string, error) {
	var version string
	err := tx.QueryRowContext(ctx, "SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?", manifest, document, unit).Scan(&version)
	return version, err
}
````

UTF-8/LF全文SHA-256：`ff4d9d644658be63608e65a64884fc34fa9316de2b91ed63d0a43a909d017287`。[参考实现](../../../../services/document/internal/repository/document/readings_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
