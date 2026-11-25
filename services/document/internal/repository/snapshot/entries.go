package snapshot

import (
	"collaboration/platform/rebuildmeta"
	"context"
	"database/sql"
	"errors"
	"strconv"
)

// CurrentPage必须一直使用建立S的同一个RepeatableRead事务。普通SELECT不取业务行锁，
// 不筛ACTIVE/PUBLIC；隐藏、未发布和零水位也必须进入完整的重建元数据。
func CurrentPage(ctx context.Context, tx *sql.Tx, after uint64, limit int) ([]rebuildmeta.Entry, error) {
	if limit < 1 || limit > 500 {
		return nil, errors.New("REBUILD_PAGE_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id>? ORDER BY id LIMIT ?`, after, limit)
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_READ_FAILED")
	}
	defer rows.Close()
	return scanEntries(rows)
}

// StoredPage只重读已封存任务的原条目，用于文件丢失后重生，不重查可变document表。
// 调用方须先在同事务Lock并确认SEALED；空页本身不能证明任务存在或已经封存。
func StoredPage(ctx context.Context, tx *sql.Tx, operation string, after uint64, limit int) ([]rebuildmeta.Entry, error) {
	if limit < 1 || limit > 500 {
		return nil, errors.New("REBUILD_PAGE_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR)
		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id
		WHERE e.operation_id=? AND j.state='SEALED' AND e.document_id>? ORDER BY e.document_id LIMIT ?`, operation, after, limit)
	if err != nil {
		return nil, errors.New("REBUILD_ENTRIES_READ_FAILED")
	}
	defer rows.Close()
	return scanEntries(rows)
}

func scanEntries(rows *sql.Rows) ([]rebuildmeta.Entry, error) {
	result := make([]rebuildmeta.Entry, 0)
	for rows.Next() {
		var value rebuildmeta.Entry
		var release sql.NullString
		if rows.Scan(&value.Document, &value.Minimum, &release) != nil || value.Document == 0 {
			return nil, errors.New("REBUILD_ENTRY_INVALID")
		}
		if release.Valid {
			parsed, err := strconv.ParseUint(release.String, 10, 64)
			if err != nil || parsed == 0 {
				return nil, errors.New("REBUILD_ENTRY_INVALID")
			}
			value.Release = parsed
		}
		result = append(result, value)
	}
	if rows.Err() != nil {
		return nil, errors.New("REBUILD_ENTRIES_READ_FAILED")
	}
	return result, nil
}

func InsertEntries(ctx context.Context, tx *sql.Tx, operation string, entries []rebuildmeta.Entry) error {
	if len(entries) > 500 {
		return errors.New("REBUILD_PAGE_INVALID")
	}
	for _, value := range entries {
		if value.Document == 0 {
			return errors.New("REBUILD_ENTRY_INVALID")
		}
		var release any
		if value.Release != 0 {
			release = value.Release
		}
		// 不使用INSERT SELECT；写入已读取的值，避免把快照扫描变成当前锁定读取。
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_snapshot_entry(operation_id,document_id,minimum_seq,release_hint) VALUES(?,?,?,?)`, operation, value.Document, value.Minimum, release); err != nil {
			return errors.New("REBUILD_ENTRY_WRITE_FAILED")
		}
	}
	return nil
}
