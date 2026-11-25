package cursor

import (
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	Open    = "OPEN"
	Closing = "CLOSING"
	Closed  = "CLOSED"

	maxOpenChains = 128
	maxRows       = 1024
	rowColumns    = "cursor_hash,pit_handle,physical_index,index_generation,scope_hash,query_hash,session_ref,page_size,expires_at,state,last_sort"
)

// Record是单链的持久事实；绑定字段不随NEXT/REPLAY或重试更新，Expires也是原绝对期限。
type Record struct {
	Hash       [32]byte
	PIT        string
	Physical   string
	Generation uint64
	ScopeHash  [32]byte
	QueryHash  [32]byte
	Session    []byte
	PageSize   uint32
	Expires    time.Time
	State      string
	Control    domain.Control
}

var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

func validateRecord(record Record) ([]byte, error) {
	invalid := errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	zero := [32]byte{}
	if record.Hash == zero || record.ScopeHash == zero || record.QueryHash == zero ||
		!physicalName.MatchString(record.Physical) || record.Generation == 0 ||
		record.PageSize == 0 || record.PageSize > 100 || record.Expires.IsZero() ||
		record.Expires.UnixMilli() <= 0 || record.Expires.Year() > 9999 ||
		record.Expires.Nanosecond()%int(time.Millisecond) != 0 || !utf8.ValidString(record.PIT) {
		return nil, invalid
	}
	if len(record.Session) != 0 {
		if len(record.Session) != 32 {
			return nil, invalid
		}
		var session [32]byte
		copy(session[:], record.Session)
		if session == zero {
			return nil, invalid
		}
	}
	switch record.State {
	case Open, Closing, Closed:
	default:
		return nil, invalid
	}
	if record.PIT == "" {
		// 未知初次open或轮换句柄仍占配额，关闭领取时可转INFLIGHT但不能捏造PIT。
		if record.State != Closed && !(record.State == Open && record.Control.Phase == domain.PhaseOpening) &&
			!(record.State == Closing && record.Control.Uncertain) {
			return nil, invalid
		}
	} else if len(record.PIT) > 64*1024 || strings.IndexFunc(record.PIT, func(r rune) bool { return r <= ' ' || r == 127 }) >= 0 {
		return nil, invalid
	}
	control, err := domain.EncodeControl(record.Control)
	if err != nil || len(control) > domain.MaxControlBytes ||
		record.Control.Deadline > record.Expires.UnixMilli() || record.Control.RetryAt > record.Expires.UnixMilli() {
		return nil, invalid
	}
	return control, nil
}

type scanner interface {
	Scan(...any) error
}

func scanRecord(source scanner) (Record, error) {
	var record Record
	var hash, scope, query, session, control []byte
	err := source.Scan(&hash, &record.PIT, &record.Physical, &record.Generation, &scope, &query,
		&session, &record.PageSize, &record.Expires, &record.State, &control)
	if err != nil {
		return Record{}, err
	}
	if len(hash) != 32 || len(scope) != 32 || len(query) != 32 || (session != nil && len(session) != 32) {
		return Record{}, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	copy(record.Hash[:], hash)
	copy(record.ScopeHash[:], scope)
	copy(record.QueryHash[:], query)
	record.Session = append([]byte(nil), session...)
	record.Expires = record.Expires.UTC()
	record.Control, err = domain.DecodeControl(control)
	if err != nil {
		return Record{}, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	if _, err := validateRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Create要求调用方已持有当前代次排他锁。配额使用当前锁定读，不能沿旧一致快照
// 漏掉其它代次已登记的链；OPENING仍是SQL OPEN，CLOSING也尚未释放配额。
func Create(ctx context.Context, tx *sql.Tx, record *Record, openLimit, rowLimit int) error {
	if tx == nil || record == nil || openLimit <= 0 || openLimit > maxOpenChains || rowLimit <= 0 || rowLimit > maxRows {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	control, err := validateRecord(*record)
	if err != nil {
		return err
	}
	var open, total int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(state IN ('OPEN','CLOSING')),0),COUNT(*) FROM cursor_registry FOR UPDATE").Scan(&open, &total); err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	if open >= openLimit || total >= rowLimit {
		return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
	}
	var session any
	if len(record.Session) != 0 {
		session = record.Session
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cursor_registry(cursor_hash,pit_handle,physical_index,index_generation,
		scope_hash,query_hash,session_ref,last_sort,page_size,expires_at,state) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		record.Hash[:], record.PIT, record.Physical, record.Generation, record.ScopeHash[:], record.QueryHash[:],
		session, control, record.PageSize, record.Expires.UTC(), record.State)
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
	}
	record.Expires = record.Expires.UTC()
	return nil
}

// Lock只锁登记行，不跨ES调用持有事务；用例在锁内核对nonce和当前状态后再保存确认。
func Lock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error) {
	if tx == nil || hash == [32]byte{} {
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	record, err := scanRecord(tx.QueryRowContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry WHERE cursor_hash=? FOR UPDATE", hash[:]))
	if err == sql.ErrNoRows {
		return Record{}, false, nil
	}
	if err != nil {
		if err.Error() == "SEARCH_CURSOR_REGISTRY_INVALID" {
			return Record{}, false, err
		}
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return record, true, nil
}

// TryLock供关闭worker跳过正在使用的链，避免首个被锁候选耗尽整批恢复预算。
// false仅表示此次未领取（可能不存在，也可能被其它事务锁住），不能用来判定链
// 不存在或准许关闭完成；普通业务核验仍使用Lock及其明确的存在语义。
func TryLock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error) {
	if tx == nil || hash == [32]byte{} {
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	record, err := scanRecord(tx.QueryRowContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry WHERE cursor_hash=? FOR UPDATE SKIP LOCKED", hash[:]))
	if err == sql.ErrNoRows {
		return Record{}, false, nil
	}
	if err != nil {
		if err.Error() == "SEARCH_CURSOR_REGISTRY_INVALID" {
			return Record{}, false, err
		}
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return record, true, nil
}

// Check供Ready逐项证明持久事实可恢复，不用进程初始化标志代替数据库关口。
// 多读一行仅用于发现总量违反1024上限，检查过程不领取链或修改任何事实。
func Check(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry ORDER BY cursor_hash LIMIT 1025")
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	defer rows.Close()
	count := 0
	unreleased := 0
	for rows.Next() {
		count++
		if count > maxRows {
			return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
		}
		record, err := scanRecord(rows)
		if err != nil {
			return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		if record.State == Open || record.State == Closing {
			unreleased++
			if unreleased > maxOpenChains {
				return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
			}
		}
	}
	if rows.Err() != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return nil
}
