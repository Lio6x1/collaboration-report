package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type searchMetadataFixture struct {
	db                            *sql.DB
	conn                          *sql.Conn
	tx                            *sql.Tx
	ctx                           context.Context
	space, kb, document, other    uint64
	missingDoc, missingKB         uint64
	missingSpace, manifest        uint64
	unit, version, missingVersion uint64
	structure, style              uint64
	operation                     string
	body                          []byte
}

func metadataInsertID(t *testing.T, ctx context.Context, tx *sql.Tx, query string, args ...any) uint64 {
	t.Helper()
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		// 只记录固定表步骤与驱动错误类别，避免DSN、参数和原始SQL错误正文泄漏。
		label := "ACTUAL_SEARCH_METADATA_FIXTURE_INSERT"
		for _, table := range []string{"space", "knowledge_base", "document", "structure_revision", "style_revision", "chapter", "collaboration_unit", "main_manifest", "release", "content_version"} {
			if strings.HasPrefix(query, "INSERT INTO "+table+"(") {
				label += "_" + table
				break
			}
		}
		metadataSQLFailure(t, label, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_FIXTURE_LAST_ID", err)
	}
	if id <= 0 {
		t.Fatal("ACTUAL_SEARCH_METADATA_FIXTURE_ID_INVALID")
	}
	return uint64(id)
}

func metadataSQLFailure(t *testing.T, label string, err error) {
	t.Helper()
	var sqlError *mysql.MySQLError
	if errors.As(err, &sqlError) {
		t.Fatalf("%s mysql_number=%d sql_state=%s", label, sqlError.Number, string(sqlError.SQLState[:]))
	}
	t.Fatalf("%s non_mysql_error=true", label)
}

func searchMetadataActual(t *testing.T) *searchMetadataFixture {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_METADATA_CONFIG_INVALID")
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal("ACTUAL_SEARCH_METADATA_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { store.DB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	conn, err := store.DB.Conn(ctx)
	if err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_CONNECTION", err)
	}
	// 会话变量不随Rollback回滚。固定独占连接保证异常夹具不能污染池内别的产品连接；
	// 即使Fatal或ctx超时，回滚后仍持有独占连接，恢复外键检查再归还连接。
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		_, restoreErr := conn.ExecContext(cleanupCtx, "SET SESSION FOREIGN_KEY_CHECKS=1")
		if restoreErr != nil {
			// 恢复失败时丢弃物理连接，绝不让外键检查关闭的会话重回连接池。
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
		if restoreErr != nil {
			metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_CLEANUP_FOREIGN_KEYS", restoreErr)
		}
	})
	var checks int
	if err := conn.QueryRowContext(ctx, "SELECT @@SESSION.foreign_key_checks").Scan(&checks); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_INITIAL_FOREIGN_KEYS", err)
	}
	if checks != 1 {
		t.Fatal("ACTUAL_SEARCH_METADATA_INITIAL_FOREIGN_KEYS_NOT_ENABLED")
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_TRANSACTION", err)
	}
	t.Cleanup(func() { tx.Rollback() })
	f := &searchMetadataFixture{db: store.DB, conn: conn, tx: tx, ctx: ctx, operation: fmt.Sprintf("search-metadata-%d", time.Now().UnixNano()), body: []byte(`{"type":"doc"}`)}
	// 直接构造私有SQL读取夹具，不声称经过创建/发布HTTP或真实Account准入；所有行
	// 均未提交，不改变产品原行。AUTO_INCREMENT回滚留下的号码空洞不作为业务水位。
	f.space = metadataInsertID(t, ctx, tx, "INSERT INTO space(kind,account_owner_id,lifecycle) VALUES('PERSONAL',1,'ACTIVE')")
	f.missingSpace = metadataInsertID(t, ctx, tx, "INSERT INTO space(kind,account_owner_id,lifecycle) VALUES('PERSONAL',1,'ACTIVE')")
	if _, err := tx.ExecContext(ctx, "DELETE FROM space WHERE id=?", f.missingSpace); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_OWN_MISSING_SPACE", err)
	}
	f.kb = metadataInsertID(t, ctx, tx, "INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,'metadata-kb','ACTIVE','PUBLIC')", f.space)
	f.missingKB = metadataInsertID(t, ctx, tx, "INSERT INTO knowledge_base(space_id,name,lifecycle) VALUES(?,'metadata-missing-kb','ACTIVE')", f.space)
	if _, err := tx.ExecContext(ctx, "DELETE FROM knowledge_base WHERE id=?", f.missingKB); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_OWN_MISSING_KB", err)
	}
	f.document = f.insertDocument(t, f.kb)
	f.other = f.insertDocument(t, 0)
	f.missingDoc = metadataInsertID(t, ctx, tx, "INSERT INTO document(space_id,title,lifecycle,publication_state) VALUES(?,'metadata-missing-document','INITIALIZING','UNPUBLISHED')", f.space)
	if _, err := tx.ExecContext(ctx, "DELETE FROM document WHERE id=?", f.missingDoc); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_OWN_MISSING_DOCUMENT", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT current_structure_id,current_style_id,current_manifest_id FROM document WHERE id=?", f.document).Scan(&f.structure, &f.style, &f.manifest); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_CURRENT_POINTERS", err)
	}
	chapter := metadataInsertID(t, ctx, tx, "INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)", f.document, f.structure)
	f.unit = metadataInsertID(t, ctx, tx, "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,'CHAPTER',?)", f.document, chapter, f.structure)
	hash := sha256.Sum256(f.body)
	f.version = metadataInsertID(t, ctx, tx, `INSERT INTO content_version(document_id,unit_id,structure_id,
		body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,
		source_kind,source_ref,provenance_json,provenance_schema)
		VALUES(?,?,?,'{}',?,?,'content-v1','canonical-v1',?,?,'INITIAL','metadata-version','{}','provenance-v1')`,
		f.document, f.unit, f.structure, f.body, hash[:], uint64(len(f.body)), f.operation)
	f.missingVersion = metadataInsertID(t, ctx, tx, `INSERT INTO content_version(document_id,unit_id,structure_id,
		body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,
		source_kind,source_ref,provenance_json,provenance_schema)
		VALUES(?,?,?,'{}',?,?,'content-v1','canonical-v1',?,?,'INITIAL','metadata-missing-version','{}','provenance-v1')`,
		f.document, f.unit, f.structure, f.body, hash[:], uint64(len(f.body)), f.operation)
	if _, err := tx.ExecContext(ctx, "DELETE FROM content_version WHERE id=?", f.missingVersion); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_OWN_MISSING_VERSION", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.document, f.unit, f.version); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_MANIFEST_ENTRY", err)
	}
	return f
}

func (f *searchMetadataFixture) insertDocument(t *testing.T, kb uint64) uint64 {
	t.Helper()
	var knowledgeBase any
	if kb != 0 {
		knowledgeBase = kb
	}
	// 当前指针是(id,document_id)复合外键，不能借用号码1或其它产品Doc的历史。
	// 先构造自有不可变图，再安装正式指针；这些SQL夹具不模拟HTTP发布流程。
	id := metadataInsertID(t, f.ctx, f.tx, `INSERT INTO document(space_id,knowledge_base_id,title,visibility_policy,lifecycle,publication_state)
		VALUES(?,?,'metadata-document','PUBLIC','INITIALIZING','UNPUBLISHED')`, f.space, knowledgeBase)
	operation := fmt.Sprintf("%s-%d", f.operation, id)
	structure := metadataInsertID(t, f.ctx, f.tx, "INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,'structure-v1')", id, operation)
	style := metadataInsertID(t, f.ctx, f.tx, "INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,'{}','style-v1',?)", id, operation)
	manifest := metadataInsertID(t, f.ctx, f.tx, "INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'SEARCH_METADATA','metadata-cut')", id, structure, style, operation)
	release := metadataInsertID(t, f.ctx, f.tx, "INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,'metadata-release',1,?)", id, manifest, operation)
	if _, err := f.tx.ExecContext(f.ctx, "UPDATE document SET lifecycle='ACTIVE',current_structure_id=?,current_style_id=?,current_manifest_id=?,current_release_id=?,publication_state='PUBLISHED',event_seq=1 WHERE id=?", structure, style, manifest, release, id); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_INSTALL_CURRENT_POINTERS", err)
	}
	return id
}

func (f *searchMetadataFixture) corrupt(t *testing.T, label, query string, args ...any) {
	t.Helper()
	f.withForeignKeyFault(t, func() { metadataExec(t, f.ctx, f.tx, label, query, args...) })
}

func metadataExec(t *testing.T, ctx context.Context, tx *sql.Tx, label, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		metadataSQLFailure(t, label, err)
	}
}

func (f *searchMetadataFixture) withForeignKeyFault(t *testing.T, action func()) {
	t.Helper()
	// 故障夹具需越过实际复合FK才能验证未知事实分支。只关闭这个测试独占会话，
	// 仅写本事务刚创建的行；读取前立即恢复，所有异常事实最终随原事务回滚。
	if _, err := f.conn.ExecContext(f.ctx, "SET SESSION FOREIGN_KEY_CHECKS=0"); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_DISABLE_FOREIGN_KEYS", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := f.conn.ExecContext(ctx, "SET SESSION FOREIGN_KEY_CHECKS=1"); err != nil {
			_ = f.conn.Raw(func(any) error { return driver.ErrBadConn })
			metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_RESTORE_FOREIGN_KEYS", err)
		}
	}()
	action()
}

func (f *searchMetadataFixture) replaceEntryVersion(t *testing.T, version uint64) {
	t.Helper()
	action := func() {
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_DELETE_OWN_ENTRY", "DELETE FROM manifest_entry WHERE manifest_id=? AND unit_id=?", f.manifest, f.unit)
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_INSERT_OWN_ENTRY", "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.document, f.unit, version)
	}
	if version != f.version {
		f.withForeignKeyFault(t, action)
	} else {
		action()
	}
}

func (f *searchMetadataFixture) replaceStoredVersion(t *testing.T, document, unit, size uint64) {
	t.Helper()
	// 服务账号只准INSERT/DELETE不可变历史，不准UPDATE。替换仅限尚未提交的自有
	// 夹具版本；合法size夹具先去掉自有引用，因此完全保持外键检查启用。
	action := func() {
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_DELETE_OWN_ENTRY", "DELETE FROM manifest_entry WHERE manifest_id=? AND unit_id=?", f.manifest, f.unit)
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_DELETE_OWN_VERSION", "DELETE FROM content_version WHERE id=?", f.version)
		hash := sha256.Sum256(f.body)
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_REINSERT_OWN_VERSION", `INSERT INTO content_version(id,document_id,unit_id,structure_id,
			body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,
			source_kind,source_ref,provenance_json,provenance_schema)
			VALUES(?,?,?,?,'{}',?,?,'content-v1','canonical-v1',?,?,'INITIAL','metadata-version','{}','provenance-v1')`,
			f.version, document, unit, f.structure, f.body, hash[:], size, f.operation)
		metadataExec(t, f.ctx, f.tx, "ACTUAL_METADATA_REINSERT_OWN_ENTRY", "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.document, f.unit, f.version)
	}
	if document != f.document || unit != f.unit {
		f.withForeignKeyFault(t, action)
	} else {
		action()
	}
}

func metadataViolation(t *testing.T, err error, code string) {
	t.Helper()
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Kind != domain.TemporarilyUnavailable || violation.Code != code {
		t.Fatal("SEARCH_METADATA_FAILURE_NOT_EXPLICIT_UNKNOWN")
	}
}

func TestActualSearchPublicationFactsMissingIsDistinctFromUnknown(t *testing.T) {
	f := searchMetadataActual(t)
	values, err := SearchPublicationFacts(f.ctx, f.tx, []uint64{f.document, f.missingDoc, f.other})
	if err != nil || len(values) != 2 || values[f.document].Document.Id != domain.TextID(f.document) || values[f.other].Document.Id != domain.TextID(f.other) {
		t.Fatal("BATCH_FACTS_DROPPED_KNOWN_DOCUMENT_OR_KEPT_MISSING")
	}
	if _, exists := values[f.missingDoc]; exists {
		t.Fatal("BATCH_FACTS_FABRICATED_MISSING_DOCUMENT")
	}
	for _, mutation := range []struct {
		name, query, restore, code string
		args, restoreArgs          []any
	}{
		{"missing-kb", "UPDATE document SET knowledge_base_id=? WHERE id=?", "UPDATE document SET knowledge_base_id=? WHERE id=?", "PUBLIC_KNOWLEDGE_BASE_UNKNOWN", []any{f.missingKB, f.document}, []any{f.kb, f.document}},
		{"cross-space-kb", "UPDATE knowledge_base SET space_id=? WHERE id=?", "UPDATE knowledge_base SET space_id=? WHERE id=?", "PUBLIC_KNOWLEDGE_BASE_UNKNOWN", []any{f.missingSpace, f.kb}, []any{f.space, f.kb}},
		{"missing-space", "UPDATE document SET space_id=? WHERE id=?", "UPDATE document SET space_id=? WHERE id=?", "PUBLIC_SPACE_UNKNOWN", []any{f.missingSpace, f.other}, []any{f.space, f.other}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			f.corrupt(t, "ACTUAL_SEARCH_FACTS_OWN_MUTATION", mutation.query, mutation.args...)
			partial, err := SearchPublicationFacts(f.ctx, f.tx, []uint64{f.missingDoc, f.document, f.other})
			metadataViolation(t, err, mutation.code)
			if partial != nil {
				t.Fatal("BATCH_FACTS_UNKNOWN_RETURNED_PARTIAL_MAP")
			}
			f.corrupt(t, "ACTUAL_SEARCH_FACTS_OWN_RESTORE", mutation.restore, mutation.restoreArgs...)
		})
	}
}

func TestActualSearchPublicationFactsKeeps100AndRetainedLifecycle(t *testing.T) {
	f := searchMetadataActual(t)
	ids := []uint64{f.document, f.other}
	for len(ids) < 100 {
		ids = append(ids, f.insertDocument(t, 0))
	}
	for _, tableID := range []struct {
		table string
		id    uint64
	}{{"document", f.document}, {"knowledge_base", f.kb}, {"space", f.space}} {
		if _, err := f.tx.ExecContext(f.ctx, "UPDATE "+tableID.table+" SET lifecycle='PUBLIC_RETAINED' WHERE id=?", tableID.id); err != nil {
			metadataSQLFailure(t, "ACTUAL_RETAINED_FIXTURE", err)
		}
	}
	values, err := SearchPublicationFacts(f.ctx, f.tx, ids)
	if err != nil || len(values) != 100 {
		t.Fatal("BATCH_FACTS_TRUNCATED_EXACT_100")
	}
	retained := values[f.document]
	policy, err := domain.PublicPolicy(retained)
	if err != nil || !domain.HasCurrentPublication(retained, policy) || retained.Document.Lifecycle != "PUBLIC_RETAINED" || retained.KnowledgeBase.Lifecycle != "PUBLIC_RETAINED" || retained.Space.Lifecycle != "PUBLIC_RETAINED" {
		t.Fatal("SEARCH_FACTS_USED_ACTIVE_ONLY_INTERNAL_READER")
	}
	for _, invalid := range [][]uint64{{0}, {f.document, f.document}, append(ids, f.missingDoc)} {
		if values, err := SearchPublicationFacts(f.ctx, f.tx, invalid); err == nil || values != nil {
			t.Fatal("INVALID_BATCH_IDENTIFIERS_ACCEPTED")
		}
	}
	if values, err := SearchPublicationFacts(f.ctx, f.tx, nil); err != nil || len(values) != 0 || values == nil {
		t.Fatal("VALID_EMPTY_CANDIDATE_BATCH_REJECTED")
	}
}

func TestActualSearchPublicationFactsBoundedSQLRoundTrips(t *testing.T) {
	f := searchMetadataActual(t)
	ids := []uint64{f.document, f.other}
	for len(ids) < 100 {
		ids = append(ids, f.insertDocument(t, 0))
	}
	maximum := uint64(^uint64(0))
	if _, err := f.tx.ExecContext(f.ctx, "UPDATE document SET event_seq=? WHERE id=?", maximum, f.document); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_BATCH_UNSIGNED_SEQUENCE", err)
	}
	selectCount := func() uint64 {
		var name string
		var count uint64
		if err := f.tx.QueryRowContext(f.ctx, "SHOW SESSION STATUS LIKE 'Com_select'").Scan(&name, &count); err != nil || name != "Com_select" {
			t.Fatal("ACTUAL_SEARCH_SELECT_COUNT_UNKNOWN")
		}
		return count
	}
	// 在原真实连接观察SELECT计数，准备图和SHOW本身不算候选读取。
	// 一批100项只能分别读Doc/KB/Space，不能每项增加两三个数据库往返。
	before := selectCount()
	facts, err := SearchPublicationFacts(f.ctx, f.tx, ids)
	after := selectCount()
	if err != nil || len(facts) != 100 || after < before || facts[f.document].Sequence != maximum {
		t.Fatal("ACTUAL_SEARCH_BATCH_READ_INCOMPLETE")
	}
	t.Logf("actual candidate batch=100 SQL SELECT round trips=%d", after-before)
	if after-before > 3 {
		t.Fatal("ACTUAL_SEARCH_BATCH_N_PLUS_ONE_READS")
	}
}

func TestActualSearchManifestMetadataRejectsFullCountAndOwnerMismatch(t *testing.T) {
	f := searchMetadataActual(t)
	units, err := SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1)
	if err != nil || len(units) != 1 || units[0].Unit != f.unit || units[0].Version != f.version || units[0].ByteSize != uint64(len(f.body)) {
		t.Fatal("ACTUAL_METADATA_ORIGINAL_BINDING_INVALID")
	}
	empty := metadataInsertID(t, f.ctx, f.tx, `INSERT INTO main_manifest(document_id,structure_id,style_id,
		source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'SEARCH_METADATA','metadata-empty-cut')`, f.document, f.structure, f.style, f.operation+"-empty")
	if units, err := SearchManifestMetadata(f.ctx, f.tx, empty, f.document, 0); err != nil || units == nil || len(units) != 0 {
		t.Fatal("VALID_EMPTY_MANIFEST_METADATA_REJECTED")
	}
	if _, err := f.tx.ExecContext(f.ctx, "DELETE FROM main_manifest WHERE id=?", empty); err != nil {
		metadataSQLFailure(t, "ACTUAL_SEARCH_METADATA_OWN_MISSING_MANIFEST", err)
	}
	_, err = SearchManifestMetadata(f.ctx, f.tx, empty, f.document, 0)
	metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
	for _, input := range []struct {
		manifest, document uint64
		count              int
	}{{f.manifest, f.other, 1}, {0, f.document, 1}, {f.manifest, 0, 1}, {f.manifest, f.document, -1}, {f.manifest, f.document, 10001}, {f.manifest, f.document, 0}, {f.manifest, f.document, 2}} {
		units, err := SearchManifestMetadata(f.ctx, f.tx, input.manifest, input.document, input.count)
		metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
		if units != nil {
			t.Fatal("INVALID_METADATA_RETURNED_PARTIAL_UNITS")
		}
	}
	// 同manifest中额外的跨Doc行不能被WHERE document_id过滤掉而冒充完整清单。
	f.corrupt(t, "ACTUAL_EXTRA_MANIFEST_ENTRY", "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", f.manifest, f.other, f.unit+1, f.version)
	_, err = SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1)
	metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
	_, err = SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 2)
	metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
}

func TestActualSearchManifestMetadataRejectsMissingAndCrossBoundVersions(t *testing.T) {
	f := searchMetadataActual(t)
	for _, mutation := range []struct {
		name           string
		apply, restore func(t *testing.T)
	}{
		{"missing-version", func(t *testing.T) { f.replaceEntryVersion(t, f.missingVersion) }, func(t *testing.T) { f.replaceEntryVersion(t, f.version) }},
		{"cross-doc-version", func(t *testing.T) { f.replaceStoredVersion(t, f.other, f.unit, uint64(len(f.body))) }, func(t *testing.T) { f.replaceStoredVersion(t, f.document, f.unit, uint64(len(f.body))) }},
		{"cross-unit-version", func(t *testing.T) { f.replaceStoredVersion(t, f.document, f.unit+1, uint64(len(f.body))) }, func(t *testing.T) { f.replaceStoredVersion(t, f.document, f.unit, uint64(len(f.body))) }},
		{"zero-byte-size", func(t *testing.T) { f.replaceStoredVersion(t, f.document, f.unit, 0) }, func(t *testing.T) { f.replaceStoredVersion(t, f.document, f.unit, uint64(len(f.body))) }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutation.apply(t)
			units, err := SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1)
			metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
			if units != nil {
				t.Fatal("BROKEN_METADATA_RETURNED_PARTIAL_UNITS")
			}
			mutation.restore(t)
		})
	}
}

func TestActualSearchManifestMetadataPreservesStoredSizeAndMaxUint64(t *testing.T) {
	f := searchMetadataActual(t)
	for _, size := range []uint64{uint64(len(f.body)) + 7, math.MaxUint64} {
		f.replaceStoredVersion(t, f.document, f.unit, size)
		units, err := SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1)
		if err != nil || len(units) != 1 || units[0].ByteSize != size {
			t.Fatal("METADATA_REWROTE_OR_TRUNCATED_UNSIGNED_BYTE_SIZE")
		}
		var actual uint64
		if err := f.tx.QueryRowContext(f.ctx, "SELECT OCTET_LENGTH(canonical_body) FROM content_version WHERE id=?", f.version).Scan(&actual); err != nil {
			metadataSQLFailure(t, "ACTUAL_METADATA_BODY_LENGTH", err)
		}
		if actual != uint64(len(f.body)) || actual == units[0].ByteSize {
			t.Fatal("ACTUAL_METADATA_SIZE_MISMATCH_FIXTURE_NOT_PROVEN")
		}
	}
	// 此层只提供存储元数据。64MiB/24MiB预留与正文严格len/hash/schema验证仍由
	// 用例和纯预算规则处理，不能为了让元数据读取通过而改成len或提前读取正文。
}

func TestActualSearchFactsAndMetadataKeepCallerRollbackAndSQLUnknown(t *testing.T) {
	f := searchMetadataActual(t)
	if _, err := SearchPublicationFacts(f.ctx, f.tx, []uint64{f.document}); err != nil {
		t.Fatal("ACTUAL_SEARCH_FACTS_READ_FAILED")
	}
	if _, err := SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1); err != nil {
		t.Fatal("ACTUAL_SEARCH_METADATA_READ_FAILED")
	}
	// 返回之后原事务仍能继续写自有行；repository不得隐式Commit或另开池连接读取。
	if _, err := f.tx.ExecContext(f.ctx, "UPDATE document SET title='caller-still-owns-tx' WHERE id=?", f.document); err != nil {
		metadataSQLFailure(t, "ACTUAL_METADATA_CALLER_CONTINUE", err)
	}
	if err := f.tx.Rollback(); err != nil {
		metadataSQLFailure(t, "ACTUAL_METADATA_CALLER_ROLLBACK", err)
	}
	for _, row := range []struct {
		table string
		id    uint64
	}{{"document", f.document}, {"document", f.other}, {"space", f.space}, {"knowledge_base", f.kb}, {"main_manifest", f.manifest}, {"content_version", f.version}} {
		var count int
		if err := f.db.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM "+row.table+" WHERE id=?", row.id).Scan(&count); err != nil {
			metadataSQLFailure(t, "ACTUAL_METADATA_VERIFY_ROLLBACK", err)
		}
		if count != 0 {
			t.Fatal("SEARCH_METADATA_FIXTURE_ESCAPED_ROLLBACK")
		}
	}
	partial, err := SearchPublicationFacts(f.ctx, f.tx, []uint64{f.document})
	metadataViolation(t, err, "SEARCH_PUBLICATION_FACTS_UNKNOWN")
	if partial != nil {
		t.Fatal("SQL_UNKNOWN_BECAME_MISSING_CANDIDATE")
	}
	_, err = SearchManifestMetadata(f.ctx, f.tx, f.manifest, f.document, 1)
	metadataViolation(t, err, "SEARCH_FIXED_METADATA_INVALID")
}
