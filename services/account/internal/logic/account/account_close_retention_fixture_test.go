package account

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

type closeDocumentFixture struct {
	id, release, manifest, version, privateManifest, privateVersion uint64
	structure, style, chapter, unit                                 uint64
	resource, privateResource                                       uint64
	ancestorManifest, ancestorStructure, ancestorStyle              uint64
}

// 私有SQL建立固定正式版及随后主线的样本；生产协议仍使用真实服务mTLS和排空worker。
// 不调用Node制造发布回执，不把这些建图语句算作发布用户流程验收。
func createCloseDocumentFixture(t *testing.T, ctx context.Context, db *sql.DB, space, kb, user uint64, operation, visibility string, published bool) closeDocumentFixture {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	insert := func(statement string, args ...any) uint64 {
		t.Helper()
		r, err := tx.ExecContext(ctx, statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, err := r.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return uint64(id)
	}
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	var knowledgeBase any
	if kb != 0 {
		knowledgeBase = kb
	}
	d := closeDocumentFixture{}
	d.id = insert("INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,publication_state) VALUES(?,?,?,'注销固定版测试',?,'INITIALIZING','UNPUBLISHED')", space, knowledgeBase, user, visibility)
	styleJSON := `{"body_font":"SimSun","western_font":"Times New Roman","body_size_pt":12,"heading_sizes_pt":[22,18,16,14,12,12],"page":"A4","margin_cm":"2.54","title_font":"SimHei","title_size_pt":22}`
	if os.Getenv("COLLAB_TEST_HISTORY_CLEANUP") == "1" {
		d.ancestorStructure = insert("INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,'structure-v1')", d.id, operation+"-ancestor")
		d.ancestorStyle = insert("INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,'style-v1',?)", d.id, styleJSON, operation+"-ancestor")
		d.ancestorManifest = insert("INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'INITIAL',?)", d.id, d.ancestorStructure, d.ancestorStyle, operation+"-ancestor", "INITIAL:"+operation+"-ancestor")
	}
	structure := insert("INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,NULLIF(?,0),?,'structure-v1')", d.id, d.ancestorStructure, operation)
	style := insert(`INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,NULLIF(?,0),?,'style-v1',?)`, d.id, d.ancestorStyle, styleJSON, operation)
	createdStructure := structure
	if d.ancestorStructure != 0 {
		createdStructure = d.ancestorStructure
	}
	chapter := insert("INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)", d.id, createdStructure)
	unit := insert("INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,'CHAPTER',?)", d.id, chapter, structure)
	d.structure, d.style, d.chapter, d.unit = structure, style, chapter, unit
	if d.ancestorStructure != 0 {
		exec("INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order) VALUES(?,?,?,?,'CHAPTER','不应公开的旧标题',1)", d.ancestorStructure, d.id, chapter, chapter)
		exec("INSERT INTO structure_entry(structure_id,document_id,node_id,parent_node_id,chapter_id,kind,title,sort_order) VALUES(?,?,?,?,?,'SECTION','旧标题的子节点',1)", d.ancestorStructure, d.id, chapter+1000000, chapter, chapter)
	}
	if os.Getenv("COLLAB_TEST_RESOURCE_CLEANUP") == "1" {
		// 这是引用/固定版本SQL样本，不声称上传或对象物理GC验收。无配额上传收据，
		// 运行中的GC不会领取它；正式引用在同一建图事务中建立。
		d.resource = insert("INSERT INTO resource(space_id,storage_key,byte_size,media_type,state,upload_operation_id) VALUES(?,?,1,'image/png','READY',?)", space, operation+"-image", operation+"-image")
		d.privateResource = insert("INSERT INTO resource(space_id,storage_key,byte_size,media_type,state,upload_operation_id) VALUES(?,?,1,'image/png','READY',?)", space, operation+"-private-image", operation+"-private-image")
	}
	exec("INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,'CHAPTER','固定正式章节',1,?)", structure, d.id, chapter, chapter, unit)
	makeManifest := func(suffix string) (uint64, uint64) {
		parent := d.ancestorManifest
		if suffix == "-private" {
			parent = d.manifest
		}
		manifest := insert("INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,NULLIF(?,0),?,?,?,?,?)", d.id, parent, structure, style, operation+suffix, "INITIAL", "INITIAL:"+operation+suffix)
		inline := []any{}
		resource := d.resource
		if suffix == "-private" {
			resource = d.privateResource
		}
		if resource != 0 {
			inline = append(inline, map[string]any{"type": "image", "attrs": map[string]any{"resourceId": strconv.FormatUint(resource, 10), "alt": "固定版本图片", "title": nil, "width": 1, "height": 1, "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}})
		}
		body, err := json.Marshal(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "attrs": map[string]string{"id": "initial-" + strconv.FormatUint(unit, 10)}, "content": inline}}})
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(body)
		// 不可变版本没有UPDATE权限。与生产初始化一样，插入前分配固定编号，
		// 正文、摘要和完整provenance一次写入，不能先占位再补写历史。
		identity := sha256.Sum256([]byte(operation + suffix))
		version := binary.BigEndian.Uint64(identity[:8])&0x3fffffffffffffff | 0x2000000000000000
		provenance := map[string]any{"schema_version": "provenance-v1", "lineage_id": strconv.FormatUint(version, 10), "base_manifest_id": strconv.FormatUint(manifest, 10), "base_content_version_id": strconv.FormatUint(version, 10), "source_operation_id": operation + suffix, "accepted_captures": []any{}, "block_sources": []any{map[string]any{"block_id": "initial-" + strconv.FormatUint(unit, 10), "origin_content_version_id": strconv.FormatUint(version, 10), "origin_block_id": "initial-" + strconv.FormatUint(unit, 10), "relation": "INSERTED", "source_range": map[string]int{"start": 0, "end": 0}}}}
		raw, err := json.Marshal(provenance)
		if err != nil {
			t.Fatal(err)
		}
		exec("INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,'content-v1','canonical-v1',?,?,'INITIAL',?,?,'provenance-v1')", version, d.id, unit, structure, string(body), body, hash[:], len(body), operation+suffix, strconv.FormatUint(unit, 10), string(raw))
		exec("INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", manifest, d.id, unit, version)
		if resource != 0 {
			exec("INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'VERSION',?,'CONTENT',1)", resource, d.id, strconv.FormatUint(version, 10))
		}
		return manifest, version
	}
	d.manifest, d.version = makeManifest("-public")
	d.release = insert("INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,'固定正式标题',?,?)", d.id, d.manifest, user, operation+"-release")
	d.privateManifest, d.privateVersion = makeManifest("-private")
	if d.ancestorManifest != 0 {
		exec("INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", d.ancestorManifest, d.id, unit, d.privateVersion)
	}
	var release any
	state := "UNPUBLISHED"
	if published {
		release, state = d.release, "PUBLISHED"
	}
	exec("UPDATE document SET lifecycle='ACTIVE',current_structure_id=?,current_style_id=?,current_manifest_id=?,current_release_id=?,publication_state=? WHERE id=?", structure, style, d.privateManifest, release, state, d.id)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return d
}

func cleanupCloseDocumentFixture(t *testing.T, ctx context.Context, db *sql.DB, d closeDocumentFixture) {
	t.Helper()
	if d.id == 0 {
		return
	}
	// 所有条件都是本样本document主键；先解开指针，再按实际外键顺序释放。
	// outbox和Node关闭审计不可删，且没有指向document的外键；保留原事件证据。
	// content_version/document_id没有前导索引，必须用已捕获的版本主键，不能扫描历史表。
	for _, item := range []struct {
		statement string
		args      []any
	}{
		{"DELETE FROM delivery_session WHERE document_id=? AND state='SETTLED'", []any{d.id}},
		{"UPDATE document SET current_structure_id=NULL,current_style_id=NULL,current_manifest_id=NULL,current_release_id=NULL,publication_state='UNPUBLISHED' WHERE id=?", []any{d.id}},
		{"DELETE FROM resource_protection WHERE document_id=?", []any{d.id}},
		{"DELETE FROM resource_reference WHERE document_id=?", []any{d.id}},
		{"DELETE FROM resource WHERE id IN (?,?)", []any{d.resource, d.privateResource}},
		{"DELETE FROM `release` WHERE id=? AND document_id=?", []any{d.release, d.id}},
		{"DELETE FROM manifest_entry WHERE manifest_id IN (?,?,?) AND document_id=?", []any{d.manifest, d.privateManifest, d.ancestorManifest, d.id}},
		{"DELETE FROM main_manifest WHERE id=? AND document_id=?", []any{d.privateManifest, d.id}},
		{"DELETE FROM main_manifest WHERE id=? AND document_id=?", []any{d.manifest, d.id}},
		{"DELETE FROM main_manifest WHERE id=? AND document_id=?", []any{d.ancestorManifest, d.id}},
		{"DELETE FROM content_version WHERE id IN (?,?) AND document_id=?", []any{d.version, d.privateVersion, d.id}},
		{"DELETE FROM structure_entry WHERE structure_id=? AND document_id=?", []any{d.structure, d.id}},
		{"DELETE FROM structure_entry WHERE structure_id=? AND document_id=? AND parent_node_id IS NOT NULL", []any{d.ancestorStructure, d.id}},
		{"DELETE FROM structure_entry WHERE structure_id=? AND document_id=?", []any{d.ancestorStructure, d.id}},
		{"DELETE FROM collaboration_unit WHERE id=? AND document_id=?", []any{d.unit, d.id}},
		{"DELETE FROM chapter WHERE id=? AND document_id=?", []any{d.chapter, d.id}},
		{"DELETE FROM style_revision WHERE id=? AND document_id=?", []any{d.style, d.id}},
		{"DELETE FROM style_revision WHERE id=? AND document_id=?", []any{d.ancestorStyle, d.id}},
		{"DELETE FROM structure_revision WHERE id=? AND document_id=?", []any{d.structure, d.id}},
		{"DELETE FROM structure_revision WHERE id=? AND document_id=?", []any{d.ancestorStructure, d.id}},
		{"DELETE FROM document WHERE id=?", []any{d.id}},
	} {
		if _, err := db.ExecContext(ctx, item.statement, item.args...); err != nil {
			t.Errorf("OWNED_RETENTION_DOCUMENT_CLEANUP: %v", err)
		}
	}
}
