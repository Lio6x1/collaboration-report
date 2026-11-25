package candidates

import (
	"collaboration/platform"
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// commitReceiptFault只丢真实delegate Commit成功后的交付结果，不代替SQL/ES，也不
// 伪称TCP断连。arm严格绑定原hash及Save控制对象，不根据事务个数或连接顺序猜窗口。
type commitReceiptFault struct {
	mu     sync.Mutex
	target [32]byte
	state  commitReceiptEvidence
}

type commitReceiptEvidence struct {
	Armed           bool
	InflightCommits int
	IdleCommits     int
	LostReceipts    int
	Tail            []byte
}

func (f *commitReceiptFault) arm(hash [32]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.target = hash
	f.state = commitReceiptEvidence{Armed: true}
}

func (f *commitReceiptFault) accepts(hash [32]byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state.Armed && f.target == hash
}

func (f *commitReceiptFault) snapshot() commitReceiptEvidence {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := f.state
	result.Tail = append([]byte(nil), f.state.Tail...)
	return result
}

type commitReceiptWrite struct {
	hash    [32]byte
	control domain.Control
}

func (f *commitReceiptFault) committed(write *commitReceiptWrite) error {
	if write == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.state.Armed || f.target != write.hash {
		return nil
	}
	switch write.control.Phase {
	case domain.PhaseInflight:
		f.state.InflightCommits++
	case domain.PhaseIdle:
		// 只命中真正查询确认：同hash已完成一次INFLIGHT登记，OPEN+IDLE携带
		// 非空真实尾sort、清空nonce/deadline且无unknown标志；reserve/补偿不满足。
		if f.state.InflightCommits == 1 && write.control.Nonce == "" && write.control.Deadline == 0 &&
			write.control.RetryAt == 0 && !write.control.Uncertain && len(write.control.Tail) != 0 {
			f.state.IdleCommits++
			f.state.LostReceipts++
			f.state.Tail = append([]byte(nil), write.control.Tail...)
			f.state.Armed = false
			// 普通固定错误避免ErrBadConn触发连接重试；不输出DSN/句柄/控制正文。
			return errors.New("ACTUAL_DRIVER_COMMIT_RECEIPT_LOST")
		}
	}
	return nil
}

func commitReceiptDatabase(t *testing.T) (*sql.DB, *commitReceiptFault) {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal("ACTUAL_COMMIT_DRIVER_CONFIG_UNAVAILABLE")
	}
	mysqlConfig, err := mysql.ParseDSN(config.DSN)
	if err != nil {
		t.Fatal("ACTUAL_COMMIT_DRIVER_DSN_INVALID")
	}
	delegate, err := mysql.NewConnector(mysqlConfig)
	if err != nil {
		t.Fatal("ACTUAL_COMMIT_DRIVER_CONNECTOR_UNAVAILABLE")
	}
	fault := &commitReceiptFault{}
	database := sql.OpenDB(&commitReceiptConnector{delegate: delegate, fault: fault})
	// 保持实际服务原pool参数及DSN。原fixture池仅作独立SQL事实核验，不扩大业务
	// 用例池或期限，也不修改interpolateParams绕过真实prepared statement路径。
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(4)
	database.SetConnMaxLifetime(5 * time.Minute)
	t.Cleanup(func() { database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if database.PingContext(ctx) != nil {
		t.Fatal("ACTUAL_COMMIT_DRIVER_DATABASE_UNAVAILABLE")
	}
	return database, fault
}

type commitReceiptConnector struct {
	delegate driver.Connector
	fault    *commitReceiptFault
}

func (c *commitReceiptConnector) Driver() driver.Driver { return c.delegate.Driver() }

func (c *commitReceiptConnector) Connect(ctx context.Context) (driver.Conn, error) {
	connection, err := c.delegate.Connect(ctx)
	if err != nil {
		return nil, errors.New("ACTUAL_COMMIT_DRIVER_CONNECT_UNAVAILABLE")
	}
	delegate, supported := connection.(commitReceiptDelegateConn)
	if !supported {
		connection.Close()
		return nil, errors.New("ACTUAL_COMMIT_DRIVER_CONNECTION_CAPABILITIES_UNKNOWN")
	}
	return &commitReceiptConn{commitReceiptDelegateConn: delegate, fault: c.fault}, nil
}

// 这些接口全部由固定mysql1.10.0实现。未识别能力明确失败，不用成功stub隐藏缺失，
// 原Rows/Result也直接返回，因此列元数据、uint64转换和结果集能力均由真实driver拥有。
type commitReceiptDelegateConn interface {
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

type commitReceiptConn struct {
	commitReceiptDelegateConn
	fault  *commitReceiptFault
	mu     sync.Mutex
	active *commitReceiptTx
}

func (c *commitReceiptConn) wrapTx(delegate driver.Tx, err error) (driver.Tx, error) {
	if err != nil {
		return nil, err
	}
	transaction := &commitReceiptTx{delegate: delegate, connection: c}
	c.mu.Lock()
	c.active = transaction
	c.mu.Unlock()
	return transaction, nil
}

func (c *commitReceiptConn) Begin() (driver.Tx, error) {
	return c.wrapTx(c.commitReceiptDelegateConn.Begin())
}

func (c *commitReceiptConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.wrapTx(c.commitReceiptDelegateConn.BeginTx(ctx, options))
}

func (c *commitReceiptConn) wrapStmt(query string, statement driver.Stmt, err error) (driver.Stmt, error) {
	if err != nil {
		return nil, err
	}
	delegate, supported := statement.(commitReceiptDelegateStmt)
	if !supported {
		statement.Close()
		return nil, errors.New("ACTUAL_COMMIT_DRIVER_STATEMENT_CAPABILITIES_UNKNOWN")
	}
	return &commitReceiptStmt{commitReceiptDelegateStmt: delegate, connection: c, query: query}, nil
}

func (c *commitReceiptConn) Prepare(query string) (driver.Stmt, error) {
	statement, err := c.commitReceiptDelegateConn.Prepare(query)
	return c.wrapStmt(query, statement, err)
}

func (c *commitReceiptConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	statement, err := c.commitReceiptDelegateConn.PrepareContext(ctx, query)
	return c.wrapStmt(query, statement, err)
}

func namedReceiptValues(values []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(values))
	for index, value := range values {
		named[index] = driver.NamedValue{Ordinal: index + 1, Value: value}
	}
	return named
}

func (c *commitReceiptConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	result, err := c.commitReceiptDelegateConn.Exec(query, args)
	c.observeSave(query, namedReceiptValues(args), result, err)
	return result, err
}

func (c *commitReceiptConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.commitReceiptDelegateConn.ExecContext(ctx, query, args)
	c.observeSave(query, args, result, err)
	return result, err
}

func (c *commitReceiptConn) observeSave(query string, args []driver.NamedValue, result driver.Result, err error) {
	// MySQL默认带参数ExecContext可返回ErrSkip，再由database/sql走Prepare/Stmt；
	// 两条路径都观察实际成功结果，不能在ErrSkip时提前标记未执行的SQL。
	if err != nil || result == nil || strings.Join(strings.Fields(query), " ") !=
		"UPDATE cursor_registry SET pit_handle=?,last_sort=?,state=? WHERE cursor_hash=?" || len(args) != 4 {
		return
	}
	state, stateOK := args[2].Value.(string)
	rawControl, controlOK := args[1].Value.([]byte)
	rawHash, hashOK := args[3].Value.([]byte)
	if !stateOK || state != "OPEN" || !controlOK || !hashOK || len(rawHash) != 32 {
		return
	}
	var hash [32]byte
	copy(hash[:], rawHash)
	if !c.fault.accepts(hash) {
		return
	}
	control, err := domain.DecodeControl(rawControl)
	affected, affectedErr := result.RowsAffected()
	if err != nil || affectedErr != nil || affected != 1 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil {
		c.active.write = &commitReceiptWrite{hash: hash, control: control}
	}
}

type commitReceiptDelegateStmt interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
	driver.NamedValueChecker
	driver.ColumnConverter
}

type commitReceiptStmt struct {
	commitReceiptDelegateStmt
	connection *commitReceiptConn
	query      string
}

func (s *commitReceiptStmt) Exec(args []driver.Value) (driver.Result, error) {
	result, err := s.commitReceiptDelegateStmt.Exec(args)
	s.connection.observeSave(s.query, namedReceiptValues(args), result, err)
	return result, err
}

func (s *commitReceiptStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	result, err := s.commitReceiptDelegateStmt.ExecContext(ctx, args)
	s.connection.observeSave(s.query, args, result, err)
	return result, err
}

type commitReceiptTx struct {
	delegate   driver.Tx
	connection *commitReceiptConn
	write      *commitReceiptWrite
}

func (t *commitReceiptTx) detach() *commitReceiptWrite {
	t.connection.mu.Lock()
	defer t.connection.mu.Unlock()
	if t.connection.active == t {
		t.connection.active = nil
	}
	return t.write
}

func (t *commitReceiptTx) Commit() error {
	err := t.delegate.Commit()
	write := t.detach()
	if err != nil {
		return err
	}
	// 必须先拿到真实mysqlTx.Commit的nil成功结果，才模拟交付丢失。SQL成功事实
	// 由测试另一个实际连接再次读出OPEN+IDLE/tail，不能仅凭此harness计数声称提交。
	return t.connection.fault.committed(write)
}

func (t *commitReceiptTx) Rollback() error {
	err := t.delegate.Rollback()
	t.detach()
	return err
}

var (
	_ driver.Connector          = (*commitReceiptConnector)(nil)
	_ commitReceiptDelegateConn = (*commitReceiptConn)(nil)
	_ commitReceiptDelegateStmt = (*commitReceiptStmt)(nil)
	_ driver.Tx                 = (*commitReceiptTx)(nil)
)
