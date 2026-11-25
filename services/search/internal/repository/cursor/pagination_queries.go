// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package cursor

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

// cleanup.go：保留原实现，按业务合并文件。

func validCleanup(tx *sql.Tx, now time.Time, limit int) bool {
	return tx != nil && !now.IsZero() && now.UnixMilli() > 0 && limit > 0 && limit <= 16
}

// Due在原事务内跳过被锁行并选择至多16个hash，调用方提交后再访问ES。
// 领取者仍须再Lock复核状态/nonce/期限，不能把调度观察当成关闭授权；
// retry_at让前面的失败链暂时让位，而原绝对期限到期必须结束等待，不续期。
func Due(ctx context.Context, tx *sql.Tx, now time.Time, limit int) ([][32]byte, error) {
	if !validCleanup(tx, now, limit) {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	now = now.UTC().Truncate(time.Millisecond)
	// CLOSING也可能仍有原query/close调用在途，健康deadline前不能再次领取。
	// 控制对象的严格读取留给Lock；这里仅做有界调度，不把JSON字段当业务授权。
	rows, err := tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE
		(state='CLOSING' AND (expires_at<=? OR (
			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.retry_at')) AS SIGNED),0)<=?
			AND NOT (JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.phase')) IN ('OPENING','INFLIGHT')
				AND CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.deadline')) AS SIGNED)>?))))
		OR (state='OPEN' AND (expires_at<=? OR (
			JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.phase')) IN ('OPENING','INFLIGHT')
			AND CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.deadline')) AS SIGNED)<=?)))
		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now, now.UnixMilli(), now.UnixMilli(), now, now.UnixMilli(), limit)
	if err != nil {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	defer rows.Close()
	var hashes [][32]byte
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
		}
		var hash [32]byte
		if len(encoded) != len(hash) {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		copy(hash[:], encoded)
		if hash == [32]byte{} {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		hashes = append(hashes, hash)
	}
	if rows.Err() != nil {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return hashes, nil
}

// DeleteClosed只在原期限已过后有界删除已释放行；仍未知的CLOSING不得靠DELETE释放配额。
func DeleteClosed(ctx context.Context, tx *sql.Tx, now time.Time, limit int) error {
	if !validCleanup(tx, now, limit) {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	// DELETE的排序扫描会等待更早的被锁行，包括尚未释放的OPEN；先跳锁选择，再按精确hash删除。
	// 两步使用同一事务，所选CLOSED行的锁保持到caller提交/回滚，避免资格改变或隐式提交。
	rows, err := tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE state='CLOSED' AND expires_at<=?
		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now.UTC().Truncate(time.Millisecond), limit)
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	var hashes [][]byte
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil || len(hash) != 32 {
			rows.Close()
			return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		hashes = append(hashes, hash)
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	for _, hash := range hashes {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cursor_registry WHERE cursor_hash=? AND state='CLOSED' AND expires_at<=?", hash, now.UTC().Truncate(time.Millisecond)); err != nil {
			return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
		}
	}
	return nil
}

// progress.go：保留原实现，按业务合并文件。

// Save只能推进同一链的最新确认PIT/控制对象/释放状态。再次读取原行，防止调用方
// 把错误query/session/期限的完整Record悄悄当成合法进度写入；任何错误仍留给原事务回滚。
func Save(ctx context.Context, tx *sql.Tx, record Record) error {
	control, err := validateRecord(record)
	if err != nil {
		return err
	}
	current, found, err := Lock(ctx, tx, record.Hash)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("SEARCH_CURSOR_REGISTRY_MISSING")
	}
	if current.Physical != record.Physical || current.Generation != record.Generation ||
		current.ScopeHash != record.ScopeHash || current.QueryHash != record.QueryHash ||
		!bytes.Equal(current.Session, record.Session) || current.PageSize != record.PageSize || !current.Expires.Equal(record.Expires) {
		return errors.New("SEARCH_CURSOR_BINDING_CHANGED")
	}
	// 显式Close已登记未知结果后，迟到的查询确认不得重新开放；nonce是否匹配由用例
	// 在同一行锁下复核，本层仍保留不可逆的关闭状态底线。
	if (current.State == Closing && record.State == Open) || (current.State == Closed && record.State != Closed) {
		return errors.New("SEARCH_CURSOR_STATE_CHANGED")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cursor_registry SET pit_handle=?,last_sort=?,state=? WHERE cursor_hash=?",
		record.PIT, control, record.State, record.Hash[:]); err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
	}
	return nil
}
