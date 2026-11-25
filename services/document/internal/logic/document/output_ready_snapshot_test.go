package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

// 数据库适配器只安排一个可观察竞争：Pending返回原RUNNING行后，真实本地
// 登记在rows.Close时被移除。是否已有持久SUCCEEDED由独立事实决定，不能凭
// 缺失本地handler或过期租约猜测完成；这里不连接产品私有库。
type outputReadyModel struct {
	local        *LocalOutputs
	done         chan struct{}
	settled      bool
	settleOnRead bool
	lookupError  bool
}

type outputReadyConnector struct{ model *outputReadyModel }

func (c outputReadyConnector) Connect(context.Context) (driver.Conn, error) {
	return &outputReadyConn{model: c.model}, nil
}

func (c outputReadyConnector) Driver() driver.Driver { return outputReadyDriver{} }

type outputReadyDriver struct{}

func (outputReadyDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("UNSUPPORTED_MODEL_OPEN")
}

type outputReadyConn struct{ model *outputReadyModel }

func (*outputReadyConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("UNSUPPORTED_MODEL_PREPARE")
}
func (*outputReadyConn) Close() error { return nil }
func (*outputReadyConn) Begin() (driver.Tx, error) {
	return nil, errors.New("UNSUPPORTED_MODEL_TRANSACTION")
}
func (c *outputReadyConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "SELECT COUNT(*)"):
		return &outputReadyRows{columns: []string{"count"}, values: []driver.Value{int64(0)}}, nil
	case strings.Contains(query, "SELECT business_key,instance_id"):
		return &outputReadyRows{
			columns: []string{"business_key", "instance_id"},
			values:  []driver.Value{"original-output", "original-owner-document"},
			onClose: func() {
				if c.model.settleOnRead {
					close(c.model.done)
				}
				c.model.settled = c.model.settleOnRead
				c.model.local.Remove("original-output")
			},
		}, nil
	case strings.Contains(query, "SELECT state"):
		if len(args) != 2 || args[0].Value != "original-output" || args[1].Value != "original-owner-document" {
			return nil, errors.New("OUTPUT_LOOKUP_BINDING_CHANGED")
		}
		if c.model.lookupError {
			return nil, errors.New("OUTPUT_LOOKUP_UNAVAILABLE")
		}
		state := "RUNNING"
		if c.model.settled {
			state = "SUCCEEDED"
		}
		return &outputReadyRows{columns: []string{"state"}, values: []driver.Value{state}}, nil
	default:
		return nil, errors.New("UNEXPECTED_OUTPUT_MODEL_QUERY")
	}
}

type outputReadyRows struct {
	columns []string
	values  []driver.Value
	onClose func()
	read    bool
}

func (r *outputReadyRows) Columns() []string { return r.columns }
func (r *outputReadyRows) Close() error {
	if r.onClose != nil {
		r.onClose()
		r.onClose = nil
	}
	return nil
}
func (r *outputReadyRows) Next(destination []driver.Value) error {
	if r.read {
		return io.EOF
	}
	copy(destination, r.values)
	r.read = true
	return nil
}

func TestOutputReadinessRechecksOriginalDurableSettlementAfterLocalRemoval(t *testing.T) {
	for _, sample := range []struct {
		name        string
		settled     bool
		lookupError bool
		wantReady   bool
	}{
		{"committed-settlement-after-pending-snapshot", true, false, true},
		{"missing-handler-without-settlement", false, false, false},
		{"settlement-read-unavailable", true, true, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			model := &outputReadyModel{done: make(chan struct{}), settleOnRead: sample.settled, lookupError: sample.lookupError}
			db := sql.OpenDB(outputReadyConnector{model})
			defer db.Close()
			outputs := NewOutputs(db, repository.NewOutputs(db), domain.OutputInstance{ID: "original-owner-document"}, nil, nil)
			model.local = outputs.Local()
			outputs.Local().Add("original-output", nil, func() {}, model.done)
			err := outputs.Ready(context.Background())
			if (err == nil) != sample.wantReady {
				t.Fatalf("ready=%t want=%t error=%v", err == nil, sample.wantReady, err)
			}
		})
	}
}
