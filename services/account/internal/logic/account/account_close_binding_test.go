package account

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	domain "collaboration/account/internal/domain/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
)

// 只读SQL模型给出一个已变代的当前账号，原Capture仍固定epoch=41。
// 测试实际BindingForDocument用例，防止新增入口从当前身份或声明参数拼造范围。
type accountLifecycleBindingModel struct {
	state   string
	binding *platform.AccountClosureBinding
}
type accountLifecycleBindingConnector struct{ model *accountLifecycleBindingModel }

func (c accountLifecycleBindingConnector) Connect(context.Context) (driver.Conn, error) {
	return &accountLifecycleBindingConn{c.model}, nil
}
func (accountLifecycleBindingConnector) Driver() driver.Driver {
	return accountLifecycleBindingDriver{}
}

type accountLifecycleBindingDriver struct{}

func (accountLifecycleBindingDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("MODEL_OPEN_UNSUPPORTED")
}

type accountLifecycleBindingConn struct{ model *accountLifecycleBindingModel }

func (*accountLifecycleBindingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("MODEL_WRITE_FORBIDDEN")
}
func (*accountLifecycleBindingConn) Close() error { return nil }
func (*accountLifecycleBindingConn) Begin() (driver.Tx, error) {
	return accountLifecycleBindingTx{}, nil
}
func (c *accountLifecycleBindingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

type accountLifecycleBindingTx struct{}

func (accountLifecycleBindingTx) Commit() error   { return nil }
func (accountLifecycleBindingTx) Rollback() error { return nil }

type accountLifecycleBindingRows struct {
	values []driver.Value
	read   bool
}

func (r *accountLifecycleBindingRows) Columns() []string { return make([]string, len(r.values)) }
func (*accountLifecycleBindingRows) Close() error        { return nil }
func (r *accountLifecycleBindingRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	copy(values, r.values)
	r.read = true
	return nil
}

func (c *accountLifecycleBindingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	m := c.model
	var values []driver.Value
	switch {
	case strings.HasPrefix(query, "SELECT user_id FROM account_close"):
		values = []driver.Value{uint64(11)}
	case strings.Contains(query, "FROM user WHERE id=? FOR UPDATE"):
		values = []driver.Value{[]byte{}, "9", uint64(99), "CLOSED"}
	case strings.Contains(query, "FROM operation_receipt"):
		plan := dto.AccountClosePlan{OperationId: "lifecycle-binding-original-01", State: m.state, RetainedReleaseIds: []string{}, RequiredSuccessions: []struct {
			Kind     string `json:"kind"`
			TargetId string `json:"target_id"`
		}{}}
		raw, err := json.Marshal(plan)
		if err != nil {
			return nil, err
		}
		state := "PREPARING"
		if m.state == "SUCCEEDED" || m.state == "REJECTED" {
			state = m.state
		}
		values = []driver.Value{uint64(11), "ACCOUNT_CLOSE", state, strings.Repeat("ab", 32), []byte{}, raw}
	case strings.Contains(query, "FROM account_close WHERE operation_id=?"):
		capture := domain.AccountCloseCapture{SessionHash: strings.Repeat("ab", 32), CredentialEpoch: 2, IdentityEpoch: 41, Binding: m.binding}
		raw, err := json.Marshal(capture)
		if err != nil {
			return nil, err
		}
		values = []driver.Value{uint64(11), m.state, raw}
	default:
		return nil, errors.New("UNEXPECTED_BINDING_MODEL_QUERY")
	}
	return &accountLifecycleBindingRows{values: values}, nil
}

func TestAccountLifecycleBindingUsesOnlyOriginalPersistentScope(t *testing.T) {
	// 即使当前epoch为99，原操作仍只返回41；未固定/已拒绝的原操作不能取得新预留资格。
	binding := &platform.AccountClosureBinding{Operation: "lifecycle-binding-original-01", User: 11, IdentityEpoch: 41, ClosingOrganizations: []uint64{21, 22}}
	m := &accountLifecycleBindingModel{state: "FROZEN", binding: binding}
	db := sql.OpenDB(accountLifecycleBindingConnector{m})
	defer db.Close()
	s := NewAccountClosures(db, nil)
	result, err := s.BindingForDocument(context.Background(), binding.Operation)
	if err != nil || !platform.EqualAccountClosureBinding(result, *binding) {
		t.Fatalf("原绑定被当前身份替换: %v %v", result, err)
	}
	m.binding = nil
	if _, err = s.BindingForDocument(context.Background(), binding.Operation); err == nil {
		t.Fatal("未固定操作被补造绑定")
	}
	m.binding = binding
	m.state = "REJECTED"
	if _, err = s.BindingForDocument(context.Background(), binding.Operation); err == nil {
		t.Fatal("已拒绝操作获得新预留资格")
	}
}
