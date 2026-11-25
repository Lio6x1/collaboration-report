package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
	"time"
)

func FixedReleaseRow(ctx context.Context, tx *sql.Tx, document, release uint64) (domain.FixedRelease, error) {
	var fixed domain.FixedRelease
	var created time.Time
	err := tx.QueryRowContext(ctx, "SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS CHAR),CAST(m.style_id AS CHAR) FROM `release` r JOIN main_manifest m ON m.id=r.manifest_id AND m.document_id=r.document_id WHERE r.document_id=? AND r.id=?", document, release).Scan(&fixed.Release.Id, &fixed.Release.DocumentId, &fixed.Release.ManifestId, &fixed.Release.PublisherId, &created, &fixed.Title, &fixed.Structure, &fixed.Style)
	if errors.Is(err, sql.ErrNoRows) {
		return fixed, domain.Violation(domain.EntityNotFound, "RELEASE_NOT_FOUND")
	}
	if err == nil {
		fixed.Release.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	}
	return fixed, err
}

// 不可变历史按数值ID倒序；游标边界和页大小由用例先验签，SQL只在原输出事务中取一页。
func ReleaseRows(ctx context.Context, tx *sql.Tx, document, before uint64, limit int) ([]dto.Release, error) {
	statement := "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(manifest_id AS CHAR),CAST(publisher_id AS CHAR),created_at FROM `release` WHERE document_id=?"
	args := []any{document}
	if before != 0 {
		statement += " AND id<?"
		args = append(args, before)
	}
	statement += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.Release{}
	for rows.Next() {
		var value dto.Release
		var created time.Time
		if err = rows.Scan(&value.Id, &value.DocumentId, &value.ManifestId, &value.PublisherId, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}
