package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
)

type searchFixedVersionFixture struct {
	id, unit, chapter uint64
	body, bodyJSON    []byte
	provenance        []byte
	hash              [32]byte
	size              uint64
	schema, canonical string
	provenanceSchema  string
}

type searchFixedFixture struct {
	ctx                                         context.Context
	tx                                          *sql.Tx
	search                                      *SearchPages
	facts                                       domain.PublicationFacts
	space, document, structure, style, manifest uint64
	release, resource                           uint64
	operation                                   string
	versions                                    []searchFixedVersionFixture
	chapterText                                 string
}

func searchFixedSQLFailure(t *testing.T, step string, err error) {
	t.Helper()
	var sqlError *mysql.MySQLError
	if errors.As(err, &sqlError) {
		t.Fatalf("%s mysql_number=%d sql_state=%s", step, sqlError.Number, string(sqlError.SQLState[:]))
	}
	// 不打印DSN、配置、正文、SQL参数或驱动原文，只交付可定位的固定步骤。
	t.Fatalf("%s non_mysql_error=true", step)
}

func (f *searchFixedFixture) exec(t *testing.T, step, query string, args ...any) sql.Result {
	t.Helper()
	result, err := f.tx.ExecContext(f.ctx, query, args...)
	if err != nil {
		searchFixedSQLFailure(t, step, err)
	}
	return result
}

func (f *searchFixedFixture) insert(t *testing.T, step, query string, args ...any) uint64 {
	t.Helper()
	result := f.exec(t, step, query, args...)
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		t.Fatal(step + "_ID_INVALID")
	}
	return uint64(id)
}

func (f *searchFixedFixture) changeOne(t *testing.T, step, query string, args ...any) {
	t.Helper()
	result := f.exec(t, step, query, args...)
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		t.Fatal(step + "_OWN_ROW_COUNT_INVALID")
	}
}

func searchFixedCanonical(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := codec.Canonical(value)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_CANONICAL_FIXTURE_INVALID")
	}
	return raw
}

func searchFixedTextBody(t *testing.T, block, text string, extra ...any) []byte {
	t.Helper()
	children := []any{map[string]any{"type": "text", "text": text}}
	children = append(children, extra...)
	return searchFixedCanonical(t, map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "attrs": map[string]any{"id": block}, "content": children},
	}})
}

func newActualSearchFixedFixture(t *testing.T) *searchFixedFixture {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_CONFIG_INVALID")
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { _ = store.DB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	tx, err := store.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_TRANSACTION_UNAVAILABLE")
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			searchFixedSQLFailure(t, "ACTUAL_SEARCH_FIXED_ROLLBACK_FAILED", err)
		}
	})
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_VALIDATOR_UNAVAILABLE")
	}
	return newSearchFixedGraph(t, ctx, tx, &SearchPages{db: store.DB, validator: validator})
}

func newSearchFixedGraph(t *testing.T, ctx context.Context, tx *sql.Tx, search *SearchPages) *searchFixedFixture {
	t.Helper()
	var entropy [12]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_ENTROPY_UNAVAILABLE")
	}
	f := &searchFixedFixture{ctx: ctx, tx: tx, search: search, operation: "search-fixed-" + hex.EncodeToString(entropy[:])}
	var foreignKeys int
	if err := tx.QueryRowContext(ctx, "SELECT @@SESSION.foreign_key_checks").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatal("ACTUAL_SEARCH_FIXED_FOREIGN_KEYS_NOT_ENABLED")
	}
	// 所有ID由当前事务新建，指针只连接自有图；不关闭外键、不借产品版本，也不提交。
	// account_owner_id/publisher_id只是私有SQL样本，不声称通过真实Account或发布HTTP。
	f.space = f.insert(t, "ACTUAL_SEARCH_FIXED_SPACE", "INSERT INTO space(kind,account_owner_id,lifecycle) VALUES('PERSONAL',1,'ACTIVE')")
	f.document = f.insert(t, "ACTUAL_SEARCH_FIXED_DOCUMENT", "INSERT INTO document(space_id,title,visibility_policy,lifecycle,publication_state) VALUES(?,'管理标题不会成为摘要','PUBLIC','INITIALIZING','UNPUBLISHED')", f.space)
	f.structure = f.insert(t, "ACTUAL_SEARCH_FIXED_STRUCTURE", "INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,'structure-v1')", f.document, f.operation)
	f.style = f.insert(t, "ACTUAL_SEARCH_FIXED_STYLE", "INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,'style-v1',?)", f.document, domain.InitialStyle, f.operation)
	f.manifest = f.insert(t, "ACTUAL_SEARCH_FIXED_MANIFEST", "INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'SEARCH_FIXED','search-fixed-owned-cut')", f.document, f.structure, f.style, f.operation)
	f.resource = f.insert(t, "ACTUAL_SEARCH_FIXED_RESOURCE", "INSERT INTO resource(space_id,uploader_id,storage_key,byte_size,media_type,state,upload_operation_id) VALUES(?,1,?,1,'image/png','READY',?)", f.space, f.operation+"-image", f.operation+"-upload")
	f.chapterText = "命中 " + strings.Repeat("中😀", 1100)
	for index, text := range []string{"首章可见链接", f.chapterText} {
		chapter := f.insert(t, "ACTUAL_SEARCH_FIXED_CHAPTER", "INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)", f.document, f.structure)
		unit := f.insert(t, "ACTUAL_SEARCH_FIXED_UNIT", "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,'CHAPTER',?)", f.document, chapter, f.structure)
		block := fmt.Sprintf("fixed-block-%d", index)
		var extra []any
		if index == 0 {
			extra = []any{
				map[string]any{"type": "formula", "attrs": map[string]any{"latex": "x^2", "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}},
				map[string]any{"type": "image", "attrs": map[string]any{"resourceId": domain.TextID(f.resource), "alt": "图片说明", "title": "图片标题", "width": nil, "height": nil, "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": map[string]any{"href": "https://private.invalid/target"}}},
			}
		}
		body := searchFixedTextBody(t, block, text, extra...)
		decoded, err := codec.Decode(body)
		if err != nil || search.validator.Body(decoded) != nil {
			t.Fatal("ACTUAL_SEARCH_FIXED_VALID_BODY_FIXTURE_INVALID")
		}
		hash := sha256.Sum256(body)
		// 先取得真实自增版本ID；尚未安装引用的自有版本可DELETE后重插正确provenance。
		version := f.insert(t, "ACTUAL_SEARCH_FIXED_VERSION_RESERVE", `INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema)
			VALUES(?,?,?,?,?,?,'content-v1','canonical-v1',?,?,'INITIAL',?,'{}','provenance-v1')`, f.document, unit, f.structure, string(body), body, hash[:], len(body), f.operation, fmt.Sprintf("fixed-version-%d", index))
		provenance := searchFixedCanonical(t, map[string]any{
			"schema_version": "provenance-v1", "lineage_id": domain.TextID(unit), "base_manifest_id": domain.TextID(f.manifest), "base_content_version_id": domain.TextID(version), "source_operation_id": f.operation,
			"accepted_captures": []any{}, "block_sources": []any{map[string]any{"block_id": block, "origin_content_version_id": domain.TextID(version), "origin_block_id": block, "relation": "INSERTED", "source_range": map[string]any{"start": json.Number("0"), "end": json.Number(fmt.Sprint(utf8.RuneCountInString(text)))}}},
		})
		decoded, err = codec.Decode(provenance)
		if err != nil || search.validator.Provenance(decoded) != nil {
			t.Fatal("ACTUAL_SEARCH_FIXED_VALID_PROVENANCE_FIXTURE_INVALID")
		}
		row := searchFixedVersionFixture{id: version, unit: unit, chapter: chapter, body: body, bodyJSON: body, provenance: provenance, hash: hash, size: uint64(len(body)), schema: "content-v1", canonical: "canonical-v1", provenanceSchema: "provenance-v1"}
		f.changeOne(t, "ACTUAL_SEARCH_FIXED_OWN_RESERVED_VERSION_DELETE", "DELETE FROM content_version WHERE id=? AND document_id=? AND unit_id=?", version, f.document, unit)
		f.insertVersion(t, row, index)
		f.versions = append(f.versions, row)
		f.exec(t, "ACTUAL_SEARCH_FIXED_MANIFEST_ENTRY", "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.document, unit, version)
		title := "第一章固定标题"
		if index == 1 {
			title = "第二章目标😀"
		}
		f.exec(t, "ACTUAL_SEARCH_FIXED_STRUCTURE_ENTRY", "INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,'CHAPTER',?,?,?)", f.structure, f.document, chapter, chapter, title, index, unit)
	}
	f.exec(t, "ACTUAL_SEARCH_FIXED_RESOURCE_REFERENCE", "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'VERSION',?,'CONTENT')", f.resource, f.document, domain.TextID(f.versions[0].id))
	f.release = f.insert(t, "ACTUAL_SEARCH_FIXED_RELEASE", "INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,'固定正式标题😀',1,?)", f.document, f.manifest, f.operation)
	f.changeOne(t, "ACTUAL_SEARCH_FIXED_INSTALL_OWN_POINTERS", "UPDATE document SET lifecycle='ACTIVE',current_structure_id=?,current_style_id=?,current_manifest_id=?,current_release_id=?,publication_state='PUBLISHED',event_seq=1 WHERE id=?", f.structure, f.style, f.manifest, f.release, f.document)
	var err error
	f.facts, err = repository.PublicationFactsRow(ctx, tx, f.document, false)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_OWN_FACTS_UNAVAILABLE")
	}
	return f
}

func (f *searchFixedFixture) insertVersion(t *testing.T, row searchFixedVersionFixture, index int) {
	t.Helper()
	f.exec(t, "ACTUAL_SEARCH_FIXED_VERSION_INSERT", `INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,'INITIAL',?,?,?)`, row.id, f.document, row.unit, f.structure, string(row.bodyJSON), row.body, row.hash[:], row.schema, row.canonical, row.size, f.operation, fmt.Sprintf("fixed-version-%d", index), string(row.provenance), row.provenanceSchema)
}

func (f *searchFixedFixture) replaceVersion(t *testing.T, index int, row searchFixedVersionFixture) {
	t.Helper()
	// immutable表账号无UPDATE权限。仅解除自有清单引用、删除/重插精确自有版本后恢复引用。
	f.changeOne(t, "ACTUAL_SEARCH_FIXED_OWN_ENTRY_DELETE", "DELETE FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=? AND content_version_id=?", f.manifest, f.document, row.unit, row.id)
	f.changeOne(t, "ACTUAL_SEARCH_FIXED_OWN_VERSION_DELETE", "DELETE FROM content_version WHERE id=? AND document_id=? AND unit_id=?", row.id, f.document, row.unit)
	f.insertVersion(t, row, index)
	f.exec(t, "ACTUAL_SEARCH_FIXED_OWN_ENTRY_REINSERT", "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.document, row.unit, row.id)
	f.versions[index] = row
}

func (f *searchFixedFixture) summary(query string, used uint64) (dto.SearchResult, uint64, error) {
	return f.search.fixedSearchSummaryRow(f.ctx, f.tx, f.facts, f.release, query, used)
}

func (f *searchFixedFixture) requireValid(t *testing.T) {
	t.Helper()
	// JSON列的读取词法由MySQL决定。先证明实际读回的provenance语义/schema均合法，
	// 只输出词法是否相同的布尔证据，不能把数据库规范化误诊为坏业务正文。
	content, err := repository.VersionContent(f.ctx, f.tx, f.versions[0].id, f.document)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_BASELINE_VERSION_UNAVAILABLE")
	}
	proof, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil || f.search.validator.Provenance(proof) != nil {
		t.Fatal("ACTUAL_SEARCH_FIXED_BASELINE_PROVENANCE_SCHEMA_INVALID")
	}
	canonical := searchFixedCanonical(t, proof)
	if !bytes.Equal(canonical, f.versions[0].provenance) {
		t.Fatal("ACTUAL_SEARCH_FIXED_BASELINE_PROVENANCE_SEMANTICS_CHANGED")
	}
	t.Logf("ACTUAL_SEARCH_FIXED_JSON_COLUMN canonical_bytes_equal=%t semantic_schema_valid=true", bytes.Equal(content.ProvenanceJsonUtf8, canonical))
	value, next, err := f.summary("第二章目标", 0)
	if err != nil {
		t.Fatalf("ACTUAL_SEARCH_FIXED_VALID_BASELINE_FAILED code=%s", searchFixedErrorCode(err))
	}
	want := string([]rune(f.chapterText)[:2000])
	if value.DocumentId != domain.TextID(f.document) || value.ReleaseId != domain.TextID(f.release) || value.Title != "固定正式标题😀" || value.Snippet != want || value.ChapterId == nil || *value.ChapterId != domain.TextID(f.versions[1].chapter) || next != f.totalBytes() {
		t.Fatal("ACTUAL_SEARCH_FIXED_AUTHORITY_OR_UNICODE_MISMATCH")
	}
}

func (f *searchFixedFixture) totalBytes() uint64 {
	var size uint64
	for _, version := range f.versions {
		size += version.size
	}
	return size
}

func searchFixedErrorCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	return "NON_DOMAIN_ERROR"
}

func searchFixedRequireFailure(t *testing.T, value dto.SearchResult, next uint64, err error, code string) {
	t.Helper()
	if err == nil || searchFixedErrorCode(err) != code || value.DocumentId != "" || value.ReleaseId != "" || value.Title != "" || value.Snippet != "" || value.ChapterId != nil || next != 0 {
		t.Fatalf("ACTUAL_SEARCH_FIXED_EXPECTED_WHOLE_FAILURE code=%s want=%s", searchFixedErrorCode(err), code)
	}
}

func TestActualSearchFixedAuthorityTitleChapterAndUnicode(t *testing.T) {
	f := newActualSearchFixedFixture(t)
	f.requireValid(t)
	value, _, err := f.summary("首章", 0)
	if err != nil || value.ChapterId == nil || *value.ChapterId != domain.TextID(f.versions[0].chapter) || value.Snippet != "首章可见链接 x^2  图片说明 图片标题" {
		t.Fatal("ACTUAL_SEARCH_FIXED_ATOM_TEXT_OR_PRIVATE_ATTRIBUTE_MISMATCH")
	}
}

func TestActualSearchFixedRejectsCorruptContentContract(t *testing.T) {
	cases := map[string]func(*searchFixedVersionFixture){
		"hash":                              func(row *searchFixedVersionFixture) { row.hash = [32]byte{} },
		"schema version":                    func(row *searchFixedVersionFixture) { row.schema = "content-v2" },
		"canonicalization version":          func(row *searchFixedVersionFixture) { row.canonical = "canonical-v2" },
		"provenance schema":                 func(row *searchFixedVersionFixture) { row.provenanceSchema = "provenance-v2" },
		"provenance semantic tree":          func(row *searchFixedVersionFixture) { row.provenance = []byte(`{}`) },
		"actual bytes differ from metadata": func(row *searchFixedVersionFixture) { row.size++ },
		"canonical body whitespace": func(row *searchFixedVersionFixture) {
			row.body = append([]byte(" "), row.body...)
			row.size = uint64(len(row.body))
			row.hash = sha256.Sum256(row.body)
		},
		"canonical body invalid UTF8": func(row *searchFixedVersionFixture) {
			row.body = []byte{0xff}
			row.size = 1
			row.hash = sha256.Sum256(row.body)
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newActualSearchFixedFixture(t)
			// 先证明同一真实图成功，随后仅改变目标字段，避免其它损坏令反例虚假通过。
			f.requireValid(t)
			row := f.versions[1]
			corrupt(&row)
			f.replaceVersion(t, 1, row)
			value, next, err := f.summary("首章", 0)
			searchFixedRequireFailure(t, value, next, err, "SEARCH_FIXED_CONTENT_INVALID")
		})
	}
}

func TestActualSearchFixedResourcesExactlyMatchBody(t *testing.T) {
	for _, name := range []string{"missing reference", "extra reference"} {
		t.Run(name, func(t *testing.T) {
			f := newActualSearchFixedFixture(t)
			f.requireValid(t)
			if name == "missing reference" {
				f.changeOne(t, "ACTUAL_SEARCH_FIXED_OWN_RESOURCE_REFERENCE_DELETE", "DELETE FROM resource_reference WHERE resource_id=? AND document_id=? AND owner_kind='VERSION' AND owner_ref=? AND purpose='CONTENT'", f.resource, f.document, domain.TextID(f.versions[0].id))
			} else {
				resource := f.insert(t, "ACTUAL_SEARCH_FIXED_EXTRA_RESOURCE", "INSERT INTO resource(space_id,storage_key,byte_size,media_type,state,upload_operation_id) VALUES(?,?,1,'image/png','READY',?)", f.space, f.operation+"-extra", f.operation+"-extra-upload")
				f.exec(t, "ACTUAL_SEARCH_FIXED_EXTRA_REFERENCE", "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'VERSION',?,'CONTENT')", resource, f.document, domain.TextID(f.versions[0].id))
			}
			value, next, err := f.summary("第二章目标", 0)
			searchFixedRequireFailure(t, value, next, err, "SEARCH_FIXED_CONTENT_INVALID")
		})
	}
}

func TestActualSearchFixedMetadataBudgetPrecedesBodyReads(t *testing.T) {
	for _, name := range []string{"whole document", "whole request"} {
		t.Run(name, func(t *testing.T) {
			f := newActualSearchFixedFixture(t)
			f.requireValid(t)
			row := f.versions[0]
			row.hash = [32]byte{}
			var used uint64
			if name == "whole document" {
				row.size = domain.SearchDocumentBodyLimit
			} else {
				used = domain.SearchPageBodyLimit - f.totalBytes() + 1
			}
			f.replaceVersion(t, 0, row)
			// 若先读取坏hash则得到FIXED_CONTENT_INVALID；必须先读完整元数据拒绝预算。
			value, next, err := f.summary("首章", used)
			searchFixedRequireFailure(t, value, next, err, "SEARCH_INCOMPLETE")
		})
	}
}

func TestActualSearchFixedBadSuffixCannotBecomePartialSummary(t *testing.T) {
	for _, name := range []string{"late unit", "late node past 2000 characters"} {
		t.Run(name, func(t *testing.T) {
			f := newActualSearchFixedFixture(t)
			f.requireValid(t)
			if name == "late unit" {
				// 第一单位已足够形成2000字符摘要，仍必须实际读取并拒绝第二单位。
				first := f.versions[0]
				body, err := codec.Decode(first.body)
				if err != nil {
					t.Fatal("ACTUAL_SEARCH_FIXED_PREFIX_BODY_DECODE_FAILED")
				}
				paragraph := body.(map[string]any)["content"].([]any)[0].(map[string]any)
				paragraph["content"].([]any)[0].(map[string]any)["text"] = "首章 " + strings.Repeat("中😀", 1200)
				first.body = searchFixedCanonical(t, body)
				first.bodyJSON, first.hash, first.size = first.body, sha256.Sum256(first.body), uint64(len(first.body))
				f.replaceVersion(t, 0, first)
				value, _, err := f.summary("首章", 0)
				if err != nil || utf8.RuneCountInString(value.Snippet) != 2000 {
					t.Fatal("ACTUAL_SEARCH_FIXED_VALID_2000_PREFIX_FAILED")
				}
			}
			row := f.versions[1]
			if name == "late unit" {
				row.body = []byte(`{"type":"doc","content":[{"type":"future_private_node"}]}`)
			} else {
				row.body = searchFixedCanonical(t, map[string]any{"type": "doc", "content": []any{
					map[string]any{"type": "paragraph", "attrs": map[string]any{"id": "long-valid-prefix"}, "content": []any{map[string]any{"type": "text", "text": strings.Repeat("中😀", 1200)}}},
					map[string]any{"type": "future_private_node"},
				}})
			}
			row.bodyJSON, row.hash, row.size = row.body, sha256.Sum256(row.body), uint64(len(row.body))
			f.replaceVersion(t, 1, row)
			value, next, err := f.summary("首章", 0)
			searchFixedRequireFailure(t, value, next, err, "SEARCH_FIXED_CONTENT_INVALID")
		})
	}
}

func TestActualSearchFixedAccumulatedBudgetIncludesLookahead(t *testing.T) {
	for _, excess := range []uint64{0, 1} {
		t.Run(fmt.Sprintf("excess-%d", excess), func(t *testing.T) {
			first := newActualSearchFixedFixture(t)
			// 两个自有Doc图共用同一原事务；未提交DELETE/重插可能保留唯一索引间隙锁，
			// 不能用第二连接构造夹具而制造与实际摘要无关的事务互等。
			lookahead := newSearchFixedGraph(t, first.ctx, first.tx, first.search)
			first.requireValid(t)
			lookahead.requireValid(t)
			// seed表示同请求此前已验证的正文；这里真实读取两次固定摘要，不虚构64MiB大正文。
			// 首项和下一合法lookahead共享同一个计数，exact64MiB通过，超1字节整项失败。
			used := domain.SearchPageBodyLimit - first.totalBytes() - lookahead.totalBytes() + excess
			_, firstBytes, err := first.summary("首章", used)
			if err != nil || firstBytes != used+first.totalBytes() {
				t.Fatalf("ACTUAL_SEARCH_FIXED_FIRST_BUDGET_FAILED code=%s", searchFixedErrorCode(err))
			}
			if excess == 1 {
				row := lookahead.versions[0]
				row.hash = [32]byte{}
				lookahead.replaceVersion(t, 0, row)
			}
			value, next, err := lookahead.summary("第二章目标", firstBytes)
			if excess == 0 {
				if err != nil || next != domain.SearchPageBodyLimit || value.DocumentId == "" || !utf8.ValidString(value.Snippet) || utf8.RuneCountInString(value.Snippet) != 2000 {
					t.Fatal("ACTUAL_SEARCH_FIXED_EXACT_PAGE_BUDGET_REJECTED")
				}
			} else {
				searchFixedRequireFailure(t, value, next, err, "SEARCH_INCOMPLETE")
			}
		})
	}
}
