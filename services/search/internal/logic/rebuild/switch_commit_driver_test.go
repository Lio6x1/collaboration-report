package rebuild

import (
	"bytes"
	"collaboration/platform"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// 只丢真实MySQL Commit成功后的交付，不声称TCP断线。命中条件为本原job的
// CompleteSwitch终态UPDATE、完整operation/fingerprint/proof及实际一行成功，
// 不能按事务个数猜窗口，不能误伤封存H/验证分页或另一任务。
type switchCommitFault struct {
	mu       sync.Mutex
	metadata switchProcessMetadata
	lost     int
	cursor   []byte
}

func (f *switchCommitFault) committed(cursor []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(cursor) == 0 || f.lost != 0 {
		return nil
	}
	f.lost++
	f.cursor = append([]byte(nil), cursor...)
	return errors.New("ACTUAL_REBUILD_TERMINAL_COMMIT_RECEIPT_LOST")
}

func (f *switchCommitFault) evidence() (int, []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lost, append([]byte(nil), f.cursor...)
}

func switchCommitDatabase(t *testing.T, cfg platform.Config, fault *switchCommitFault) *sql.DB {
	t.Helper()
	parsed, err := mysql.ParseDSN(cfg.DSN)
	switchBoundaryMust(t, "ACTUAL_SWITCH_DRIVER_DSN_INVALID", err)
	delegate, err := mysql.NewConnector(parsed)
	switchBoundaryMust(t, "ACTUAL_SWITCH_DRIVER_CONNECTOR_INVALID", err)
	db := sql.OpenDB(&switchCommitConnector{delegate: delegate, fault: fault})
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	t.Cleanup(func() { db.Close() })
	return db
}

type switchCommitConnector struct {
	delegate driver.Connector
	fault    *switchCommitFault
}

func (c *switchCommitConnector) Driver() driver.Driver { return c.delegate.Driver() }
func (c *switchCommitConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.delegate.Connect(ctx)
	if err != nil {
		return nil, errors.New("ACTUAL_SWITCH_DRIVER_CONNECT_FAILED")
	}
	delegate, ok := conn.(switchCommitDelegateConn)
	if !ok {
		conn.Close()
		return nil, errors.New("ACTUAL_SWITCH_DRIVER_CAPABILITIES_UNKNOWN")
	}
	return &switchCommitConn{switchCommitDelegateConn: delegate, fault: c.fault}, nil
}

// 原Rows/Result及所有可选driver接口交给固定MySQL驱动；不增加成功stub。
// 同时覆盖带参数Exec返回ErrSkip后走真实Prepare/Stmt的默认路径。
type switchCommitDelegateConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.Execer
	driver.ExecerContext
	driver.Queryer
	driver.QueryerContext
	driver.NamedValueChecker
	driver.Pinger
	driver.SessionResetter
	driver.Validator
}
type switchCommitConn struct {
	switchCommitDelegateConn
	fault  *switchCommitFault
	mu     sync.Mutex
	active *switchCommitTx
}

func (c *switchCommitConn) wrapTx(tx driver.Tx, err error) (driver.Tx, error) {
	if err != nil {
		return nil, err
	}
	value := &switchCommitTx{delegate: tx, connection: c}
	c.mu.Lock()
	c.active = value
	c.mu.Unlock()
	return value, nil
}
func (c *switchCommitConn) Begin() (driver.Tx, error) {
	return c.wrapTx(c.switchCommitDelegateConn.Begin())
}
func (c *switchCommitConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.wrapTx(c.switchCommitDelegateConn.BeginTx(ctx, options))
}
func (c *switchCommitConn) wrapStmt(query string, stmt driver.Stmt, err error) (driver.Stmt, error) {
	if err != nil {
		return nil, err
	}
	delegate, ok := stmt.(switchCommitDelegateStmt)
	if !ok {
		stmt.Close()
		return nil, errors.New("ACTUAL_SWITCH_DRIVER_STATEMENT_CAPABILITIES_UNKNOWN")
	}
	return &switchCommitStmt{switchCommitDelegateStmt: delegate, connection: c, query: query}, nil
}
func (c *switchCommitConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.switchCommitDelegateConn.Prepare(query)
	return c.wrapStmt(query, stmt, err)
}
func (c *switchCommitConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	stmt, err := c.switchCommitDelegateConn.PrepareContext(ctx, query)
	return c.wrapStmt(query, stmt, err)
}
func switchNamedValues(values []driver.Value) []driver.NamedValue {
	args := make([]driver.NamedValue, len(values))
	for i, value := range values {
		args[i] = driver.NamedValue{Ordinal: i + 1, Value: value}
	}
	return args
}
func (c *switchCommitConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	result, err := c.switchCommitDelegateConn.Exec(query, args)
	c.observeTerminal(query, switchNamedValues(args), result, err)
	return result, err
}
func (c *switchCommitConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.switchCommitDelegateConn.ExecContext(ctx, query, args)
	c.observeTerminal(query, args, result, err)
	return result, err
}

func switchDriverUint64(value any) (uint64, bool) {
	switch value := value.(type) {
	case uint64:
		return value, true
	case int64:
		return uint64(value), value >= 0
	case string:
		n, err := strconv.ParseUint(value, 10, 64)
		return n, err == nil && strconv.FormatUint(n, 10) == value
	}
	return 0, false
}

func (c *switchCommitConn) observeTerminal(query string, args []driver.NamedValue, result driver.Result, err error) {
	const terminalUpdate = "UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id JOIN rebuild_binding b ON b.job_id=j.id SET j.state='SUCCEEDED',j.cursor_json=?,j.last_error=NULL WHERE j.id=? AND j.state='SWITCHING' AND j.source_index=? AND j.target_index=? AND j.generation=? AND b.operation_id=? AND b.state='ACCEPTED' AND b.request_fingerprint=? AND p.frontier_payload IS NOT NULL AND p.validation_digest=?"
	if err != nil || result == nil || strings.Join(strings.Fields(query), " ") != terminalUpdate || len(args) != 8 {
		return
	}
	job, jobOK := switchDriverUint64(args[1].Value)
	generation, generationOK := switchDriverUint64(args[4].Value)
	operation, operationOK := args[5].Value.(string)
	source, sourceOK := args[2].Value.(string)
	target, targetOK := args[3].Value.(string)
	fingerprint, fingerprintOK := args[6].Value.([]byte)
	cursor, cursorOK := args[0].Value.([]byte)
	m := c.fault.metadata
	if !jobOK || !generationOK || !operationOK || !sourceOK || !targetOK || !fingerprintOK || !cursorOK || job != m.JobID || generation != 2 || operation != m.Operation || source != m.Source || target != m.Target || !bytes.Equal(fingerprint, m.Fingerprint[:]) {
		return
	}
	var envelope terminalEnvelope
	var proof terminalProof
	if json.Unmarshal(cursor, &envelope) != nil || envelope.Version != 1 || json.Unmarshal(envelope.Fixed, &proof) != nil || proof.State != "SUCCEEDED" || proof.Operation != m.Operation || proof.JobID != strconv.FormatUint(m.JobID, 10) || proof.Target != m.Target || proof.Fingerprint != hex.EncodeToString(m.Fingerprint[:]) {
		return
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil {
		c.active.cursor = append([]byte(nil), cursor...)
	}
}

type switchCommitDelegateStmt interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
	driver.NamedValueChecker
	driver.ColumnConverter
}
type switchCommitStmt struct {
	switchCommitDelegateStmt
	connection *switchCommitConn
	query      string
}

func (s *switchCommitStmt) Exec(args []driver.Value) (driver.Result, error) {
	result, err := s.switchCommitDelegateStmt.Exec(args)
	s.connection.observeTerminal(s.query, switchNamedValues(args), result, err)
	return result, err
}
func (s *switchCommitStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	result, err := s.switchCommitDelegateStmt.ExecContext(ctx, args)
	s.connection.observeTerminal(s.query, args, result, err)
	return result, err
}

type switchCommitTx struct {
	delegate   driver.Tx
	connection *switchCommitConn
	cursor     []byte
}

func (tx *switchCommitTx) detach() []byte {
	tx.connection.mu.Lock()
	defer tx.connection.mu.Unlock()
	if tx.connection.active == tx {
		tx.connection.active = nil
	}
	return tx.cursor
}
func (tx *switchCommitTx) Commit() error {
	err := tx.delegate.Commit()
	cursor := tx.detach()
	if err != nil {
		return err
	}
	return tx.connection.fault.committed(cursor)
}
func (tx *switchCommitTx) Rollback() error {
	err := tx.delegate.Rollback()
	tx.detach()
	return err
}

var (
	_ driver.Connector         = (*switchCommitConnector)(nil)
	_ switchCommitDelegateConn = (*switchCommitConn)(nil)
	_ switchCommitDelegateStmt = (*switchCommitStmt)(nil)
	_ driver.Tx                = (*switchCommitTx)(nil)
)
