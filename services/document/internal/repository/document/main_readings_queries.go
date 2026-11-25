// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// main_read.go：保留原实现，按业务合并文件。

// 该记录只表示只读捕获依据已持久化，不是用户控制动作或可公开查询的业务成功回执。
// 读失败不会切MAIN指针；原短期准入到期后，私有source和输出都必须拒绝继续使用。
func InsertMainReadContext(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	return InsertSubmissionTask(ctx, tx, "MAIN_READ_CONTEXT", operation, operation, payload, "SUCCEEDED")
}

func MainReadContext(ctx context.Context, query Queryer, operation string) ([]byte, error) {
	return MergeArtifact(ctx, query, "MAIN_READ_CONTEXT", operation)
}

func BindMainReadAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte) error {
	result, err := tx.ExecContext(ctx, "UPDATE document_admission SET target_json=? WHERE admission_id=? AND operation_id=? AND purpose='READ_MAIN' AND state='OPEN'", string(target), admission, operation)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// main_workspace.go：保留原实现，按业务合并文件。

// MainWorkspaceEpoch复用调用方已经持有的Doc锁和事务，仅取原固定安装的标量。
// 不能为取得一个epoch再次拉出整篇正文，也不能查询协作服务私有数据库。
func MainWorkspaceEpoch(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error) {
	var fact domain.MainWorkspaceFact
	var manifest, structure, style, control sql.NullString
	var fixed, succeeded, installed, matched sql.NullBool
	err := tx.QueryRowContext(ctx, `SELECT m.id,m.structure_id,m.style_id,d.control_revision,m.product_key,m.parent_id IS NOT NULL,
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.manifest')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Structure')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Style')),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.Control')),
		f.state='SUCCEEDED',o.state='SUCCEEDED',
		completed.state='SUCCEEDED' AND JSON_UNQUOTE(JSON_EXTRACT(completed.payload,'$.manifest_id'))=CAST(m.id AS CHAR),
		JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.operation_id'))=m.source_operation_id
		AND JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.target.document_id'))=CAST(d.id AS CHAR)
		AND FROM_BASE64(JSON_UNQUOTE(JSON_EXTRACT(f.payload,'$.context.request_fingerprint')))=o.request_fingerprint
		FROM document d JOIN main_manifest m ON m.id=d.current_manifest_id AND m.document_id=d.id
		LEFT JOIN durable_task f ON f.kind='MAIN_COMMIT_FIXED' AND f.business_key=m.source_operation_id AND f.operation_id=m.source_operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=m.source_operation_id
		LEFT JOIN durable_task completed ON completed.kind='MAIN_INSTALL_COMPLETED' AND completed.business_key=m.source_operation_id AND completed.operation_id=m.source_operation_id
		WHERE d.id=?`, document).Scan(&fact.Manifest, &fact.Structure, &fact.Style, &fact.MetadataControl, &fact.Product, &fact.Parent,
		&manifest, &structure, &style, &control, &fixed, &succeeded, &installed, &matched)
	if err != nil {
		return 0, err
	}
	values := []struct {
		raw    sql.NullString
		target *uint64
	}{
		{manifest, &fact.RecordedManifest}, {structure, &fact.RecordedStructure}, {style, &fact.RecordedStyle}, {control, &fact.Control},
	}
	for _, value := range values {
		if !value.raw.Valid {
			continue
		}
		parsed, err := domain.CanonicalID(value.raw.String)
		if err != nil {
			return 0, domain.Violation(domain.TemporarilyUnavailable, "MAIN_WORKSPACE_BINDING_UNKNOWN")
		}
		*value.target = parsed
	}
	fact.Fixed, fact.Succeeded, fact.Installed, fact.OriginalMatched = fixed.Valid && fixed.Bool, succeeded.Valid && succeeded.Bool, installed.Valid && installed.Bool, matched.Valid && matched.Bool
	return domain.ResolveMainWorkspace(fact)
}
