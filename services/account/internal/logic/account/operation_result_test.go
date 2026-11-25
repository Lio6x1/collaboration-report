package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// 离线SQL边界只提供持久事实，权限判断仍执行真实Logout/AccountClosures.Progress。
// 不连接部署私有库；Exec/Prepare全部拒绝，查询不能被测试偷偷变成新操作。
type operationResultModel struct {
	kind, state, phase string
	actor, sessionHash string
	generation         uint64
	result             []byte
	sessions           map[string]domain.Session
	changed            bool
	missingTerminal    bool
	snapshotReads      int
}

type operationResultConnector struct{ model *operationResultModel }

func (c operationResultConnector) Connect(context.Context) (driver.Conn, error) {
	return &operationResultConn{model: c.model}, nil
}
func (operationResultConnector) Driver() driver.Driver { return operationResultDriver{} }

type operationResultDriver struct{}

func (operationResultDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("MODEL_OPEN_UNSUPPORTED")
}

type operationResultConn struct{ model *operationResultModel }

func (*operationResultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("MODEL_WRITE_UNSUPPORTED")
}
func (*operationResultConn) Close() error              { return nil }
func (*operationResultConn) Begin() (driver.Tx, error) { return operationResultTx{}, nil }
func (*operationResultConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return operationResultTx{}, nil
}

type operationResultTx struct{}

func (operationResultTx) Commit() error   { return nil }
func (operationResultTx) Rollback() error { return nil }

type operationResultRows struct {
	values []driver.Value
	read   bool
}

func (r *operationResultRows) Columns() []string {
	columns := make([]string, len(r.values))
	for i := range columns {
		columns[i] = "column"
	}
	return columns
}
func (*operationResultRows) Close() error { return nil }
func (r *operationResultRows) Next(destination []driver.Value) error {
	if r.read || r.values == nil {
		return io.EOF
	}
	copy(destination, r.values)
	r.read = true
	return nil
}

const operationResultID = "account-operation-original-01"

func (c *operationResultConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	m := c.model
	row := func(values ...driver.Value) (driver.Rows, error) { return &operationResultRows{values: values}, nil }
	if strings.Contains(query, "FROM operation_receipt") && args[0].Value != operationResultID {
		return row()
	}
	switch {
	case strings.Contains(query, "FROM operation_receipt") && strings.Contains(query, "terminal_at IS NOT NULL"):
		m.snapshotReads++
		generation := m.generation
		if m.changed && m.snapshotReads > 1 {
			generation++
		}
		return row(m.actor, m.kind, m.state, m.sessionHash, bytes.Repeat([]byte{1}, 32), generation, m.result, m.state != "PREPARING" && !m.missingTerminal)
	case strings.Contains(query, "FROM operation_receipt") && strings.Contains(query, "request_fingerprint"):
		return row(int64(7), m.kind, m.state, m.sessionHash, bytes.Repeat([]byte{1}, 32), m.result)
	case strings.Contains(query, "FROM operation_receipt") && strings.Contains(query, "kind,state"):
		return row(m.actor, m.kind, m.state, m.sessionHash)
	case strings.Contains(query, "FROM operation_receipt") && strings.Contains(query, "actor_id,kind"):
		return row(int64(7), m.kind, m.sessionHash)
	case strings.Contains(query, "SELECT result_json FROM operation_receipt"):
		return row(m.result)
	case strings.Contains(query, "FROM session WHERE id=?"):
		s, ok := m.sessions[hex.EncodeToString(args[0].Value.([]byte))]
		if !ok {
			return row()
		}
		return row(s.UserID, []byte{}, s.Epoch, s.CredentialEpoch, s.State, s.Expires)
	case strings.Contains(query, "FROM user WHERE id=? FOR UPDATE"):
		return row([]byte{}, "2", uint64(3), "CLOSED")
	case strings.Contains(query, "SELECT state,CAST(credential_epoch AS CHAR) FROM user"):
		return row("ACTIVE", "2")
	case strings.Contains(query, "FROM account_close WHERE operation_id=?"):
		capture := []byte(`{"session_hash":"` + m.sessionHash + `","credential_epoch":2,"identity_epoch":3}`)
		return row(int64(7), m.phase, capture)
	default:
		return nil, errors.New("UNEXPECTED_OPERATION_MODEL_QUERY: " + query)
	}
}

func newOperationResultFixture(t *testing.T, kind string) (*OperationResults, *operationResultModel, string, string) {
	t.Helper()
	token, id, err := domain.RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	other, otherID, err := domain.RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	m := &operationResultModel{kind: kind, state: "SUCCEEDED", phase: "SUCCEEDED", actor: "7", sessionHash: hex.EncodeToString(id), generation: 9,
		result: []byte(`{"operation_id":"` + operationResultID + `","state":"SUCCEEDED","result":{"entity_id":"123"}}`),
		sessions: map[string]domain.Session{
			hex.EncodeToString(id):      {ID: id, UserID: "7", State: "CLOSED", CredentialEpoch: "2", Epoch: 1, Expires: time.Now().Add(-time.Hour)},
			hex.EncodeToString(otherID): {ID: otherID, UserID: "8", State: "ACTIVE", CredentialEpoch: "2", Epoch: 1, Expires: time.Now().Add(time.Hour)},
		}}
	if kind == "ACCOUNT_CLOSE" {
		m.result = []byte(`{"operation_id":"` + operationResultID + `","state":"SUCCEEDED","required_successions":[],"retained_release_ids":[]}`)
	}
	db := sql.OpenDB(operationResultConnector{m})
	t.Cleanup(func() { _ = db.Close() })
	auth := NewAuthentication(db, repository.NewAuthentication(nil, db), nil, nil)
	logout := NewLogout(db, auth, repository.NewOperationReceipts(db), nil, nil)
	return NewOperationResults(db, logout, NewAccountClosures(db, auth)), m, token, other
}

func TestAccountOperationResultOriginalRecoveryAndMinimalStableTerminal(t *testing.T) {
	// 漏分派ACCOUNT_CLOSE或忽略recovery_receipt会拒绝已关闭原会话；披露原result则越过最小查询边界。
	for _, kind := range []string{"SESSION_LOGOUT", "ACCOUNT_CLOSE"} {
		t.Run(kind, func(t *testing.T) {
			s, _, original, other := newOperationResultFixture(t, kind)
			query := &pb.AccountOperationQuery{OperationId: operationResultID, SessionToken: []byte(other), RecoveryReceipt: []byte(original)}
			first, err := s.Get(ctxForOperationResult(), query)
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.Get(ctxForOperationResult(), query)
			if err != nil {
				t.Fatal(err)
			}
			minimal := []byte(`{"operation_id":"` + operationResultID + `","state":"SUCCEEDED"}`)
			digest := sha256.Sum256(minimal)
			if first.State != pb.OperationResult_SUCCEEDED || first.Generation != 9 || first.TerminalRevision != 9 || !bytes.Equal(first.FixedResultJsonUtf8, minimal) || !bytes.Equal(first.ResultDigest, digest[:]) || !bytes.Equal(first.ResultDigest, second.ResultDigest) || len(first.ResultIds) != 0 {
				t.Fatalf("原持久终态最小投影失配: %v", first)
			}
			query.RecoveryReceipt = []byte(other)
			var rule domain.RuleViolation
			if _, err = s.Get(ctxForOperationResult(), query); !errors.As(err, &rule) || rule.Kind != domain.EntityNotFound {
				t.Fatal("其他会话读取了原恢复结果")
			}
		})
	}
}

func ctxForOperationResult() context.Context { return context.Background() }

func TestAccountOperationResultRecoveryCannotReplaceCurrentSessionOrMixGeneration(t *testing.T) {
	// 普通修改须当前本人主会话，恢复秘密不得扩权；并发推进须拒绝拼接不同代次。
	s, m, original, other := newOperationResultFixture(t, "ORG_EDIT")
	var rule domain.RuleViolation
	if _, err := s.Get(context.Background(), &pb.AccountOperationQuery{OperationId: operationResultID, SessionToken: []byte(other), RecoveryReceipt: []byte(original)}); !errors.As(err, &rule) || rule.Kind != domain.EntityNotFound {
		t.Fatal("普通修改接受恢复秘密")
	}
	originalID, _ := domain.TokenHash(original)
	m.sessions[hex.EncodeToString(originalID)] = domain.Session{ID: originalID, UserID: "7", State: "ACTIVE", CredentialEpoch: "2", Expires: time.Now().Add(time.Hour)}
	q := &pb.AccountOperationQuery{OperationId: operationResultID, SessionToken: []byte(original)}
	if _, err := s.Get(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	m.changed = true
	m.snapshotReads = 0
	if _, err := s.Get(context.Background(), q); !errors.As(err, &rule) || rule.Kind != domain.TemporarilyUnavailable {
		t.Fatal("并发推进被拼成稳定终态")
	}
}

func TestAccountOperationResultProcessingAndUnprovenTerminalRemainUnfixed(t *testing.T) {
	// CLEANING只有当前阶段；没有terminal_at的成功字样不能形成固定成功证明。
	s, m, original, _ := newOperationResultFixture(t, "ACCOUNT_CLOSE")
	q := &pb.AccountOperationQuery{OperationId: operationResultID, RecoveryReceipt: []byte(original)}
	m.state, m.phase = "PREPARING", "CLEANING"
	m.result = []byte(`{"operation_id":"` + operationResultID + `","state":"CLEANING","required_successions":[],"retained_release_ids":[]}`)
	result, err := s.Get(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != pb.OperationResult_PROCESSING || result.Generation != 9 || result.TerminalRevision != 0 || len(result.FixedResultJsonUtf8) != 0 || len(result.ResultDigest) != 0 || string(result.CurrentStateJsonUtf8) != `{"operation_id":"`+operationResultID+`","state":"CLEANING"}` {
		t.Fatalf("清理阶段被提前固定: %v", result)
	}
	m.state, m.phase, m.missingTerminal = "SUCCEEDED", "SUCCEEDED", true
	m.result = []byte(`{"operation_id":"` + operationResultID + `","state":"SUCCEEDED","required_successions":[],"retained_release_ids":[]}`)
	var rule domain.RuleViolation
	if _, err = s.Get(context.Background(), q); !errors.As(err, &rule) || rule.Kind != domain.TemporarilyUnavailable {
		t.Fatal("缺少持久terminal_at仍签出成功")
	}
	q.OperationId = "account-operation-does-not-exist"
	if _, err = s.Get(context.Background(), q); !errors.As(err, &rule) || rule.Kind != domain.EntityNotFound {
		t.Fatal("恢复秘密查询新操作没有拒绝")
	}
}
