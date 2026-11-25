package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"google.golang.org/protobuf/proto"
)

const lifecycleRPCOperation = "lifecycle-rpc-original-01"
const lifecycleRPCReservedJSON = `{"binding":{"operation_id":"lifecycle-rpc-original-01","user_id":11,"identity_epoch":41,"closing_organizations":[21]},"reserved_knowledge_bases":[31]}`
const lifecycleRPCPreparedJSON = `{"binding":{"operation_id":"lifecycle-rpc-original-01","user_id":11,"identity_epoch":41,"closing_organizations":[21]},"reserved_knowledge_bases":[31],"retention":{"space_ids":[],"knowledge_base_ids":[],"documents":[],"scopes":[]}}`

// SQL模型只提供协议登记/原计划边界，不连接产品数据库。复杂预留与保留业务已有
// 实际集成关口；这里用同接口边界观察是否在错摘要/代次时错误调用原业务用例。
type lifecycleRPCModel struct {
	registration, plan                                    []byte
	phase                                                 string
	commitUnknown                                         bool
	reserveCalls, planCalls, relationCalls, completeCalls int
	canComplete, authorityUnavailable                     bool
}
type lifecycleRPCConnector struct{ model *lifecycleRPCModel }

func (c lifecycleRPCConnector) Connect(context.Context) (driver.Conn, error) {
	return &lifecycleRPCConn{model: c.model}, nil
}
func (lifecycleRPCConnector) Driver() driver.Driver { return lifecycleRPCDriver{} }

type lifecycleRPCDriver struct{}

func (lifecycleRPCDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("MODEL_OPEN_UNSUPPORTED")
}

type lifecycleRPCConn struct{ model *lifecycleRPCModel }

func (*lifecycleRPCConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("MODEL_PREPARE_UNSUPPORTED")
}
func (*lifecycleRPCConn) Close() error                { return nil }
func (c *lifecycleRPCConn) Begin() (driver.Tx, error) { return &lifecycleRPCTx{model: c.model}, nil }
func (c *lifecycleRPCConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}

type lifecycleRPCTx struct{ model *lifecycleRPCModel }

func (t *lifecycleRPCTx) Commit() error {
	if t.model.commitUnknown {
		t.model.commitUnknown = false
		return errors.New("MODEL_COMMITTED_REPLY_UNKNOWN")
	}
	return nil
}
func (*lifecycleRPCTx) Rollback() error { return nil }

type lifecycleRPCRows struct {
	values []driver.Value
	read   bool
}

func (r *lifecycleRPCRows) Columns() []string { return make([]string, len(r.values)) }
func (*lifecycleRPCRows) Close() error        { return nil }
func (r *lifecycleRPCRows) Next(values []driver.Value) error {
	if r.read || r.values == nil {
		return io.EOF
	}
	copy(values, r.values)
	r.read = true
	return nil
}
func (c *lifecycleRPCConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	m := c.model
	switch {
	case strings.Contains(query, "FROM durable_task"):
		if m.registration == nil {
			return &lifecycleRPCRows{}, nil
		}
		return &lifecycleRPCRows{values: []driver.Value{bytes.Clone(m.registration), lifecycleRPCOperation, "SUCCEEDED"}}, nil
	case strings.Contains(query, "FROM lifecycle_operation"):
		if m.plan == nil {
			return &lifecycleRPCRows{}, nil
		}
		return &lifecycleRPCRows{values: []driver.Value{uint64(11), "ACCOUNT_CLOSE", m.phase, uint64(41), bytes.Clone(m.plan)}}, nil
	default:
		return nil, errors.New("UNEXPECTED_LIFECYCLE_MODEL_QUERY")
	}
}
func (c *lifecycleRPCConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.HasPrefix(query, "INSERT INTO durable_task"):
		if c.model.registration == nil {
			c.model.registration = []byte(args[2].Value.(string))
		}
	case strings.HasPrefix(query, "UPDATE durable_task SET payload"):
		c.model.registration = []byte(args[0].Value.(string))
	default:
		return nil, errors.New("UNEXPECTED_LIFECYCLE_MODEL_WRITE")
	}
	return driver.RowsAffected(1), nil
}
func (m *lifecycleRPCModel) Binding(context.Context, string) (platform.AccountClosureBinding, error) {
	if m.authorityUnavailable {
		return platform.AccountClosureBinding{}, errors.New("ACCOUNT_NOT_CURRENT_ANYMORE")
	}
	return platform.AccountClosureBinding{Operation: lifecycleRPCOperation, User: 11, IdentityEpoch: 41, ClosingOrganizations: []uint64{21}}, nil
}
func (m *lifecycleRPCModel) Prepare(_ context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementResult, error) {
	if binding.IdentityEpoch != 41 {
		return domain.AccountRetirementResult{}, errors.New("IDENTITY_EPOCH_INFERRED_FROM_GENERATION")
	}
	m.reserveCalls++
	m.plan = []byte(lifecycleRPCReservedJSON)
	m.phase = "FROZEN"
	return domain.AccountRetirementResult{State: "FROZEN", ReservedKnowledgeBases: []uint64{31}}, nil
}

type lifecycleRPCRetention struct{ model *lifecycleRPCModel }

func (r lifecycleRPCRetention) Plan(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error) {
	r.model.planCalls++
	r.model.phase = "PLANNED"
	r.model.plan = []byte(lifecycleRPCPreparedJSON)
	return domain.DecodeAccountRetirementPlan(r.model.plan)
}
func (r lifecycleRPCRetention) ApplyRelations(context.Context, domain.AccountRetirementBinding) error {
	r.model.relationCalls++
	r.model.phase = "CLEANING"
	return nil
}
func (r lifecycleRPCRetention) Complete(context.Context, domain.AccountRetirementBinding) error {
	r.model.completeCalls++
	if !r.model.canComplete {
		return domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
	}
	r.model.phase = "SUCCEEDED"
	return nil
}

func lifecycleRPCFixture(t *testing.T) (*LifecyclePlans, *lifecycleRPCModel, *pb.LifecycleRequest) {
	t.Helper()
	m := &lifecycleRPCModel{}
	db := sql.OpenDB(lifecycleRPCConnector{m})
	t.Cleanup(func() { _ = db.Close() })
	return NewLifecyclePlans(db, m, m, lifecycleRPCRetention{m}), m, &pb.LifecycleRequest{OperationId: lifecycleRPCOperation, UserId: 11, ClosingOrganizationIds: []uint64{21}, Generation: 7}
}
func lifecycleRPCWant(t *testing.T, result *pb.LifecyclePlan, raw, phase string) {
	t.Helper()
	digest := sha256.Sum256([]byte(raw))
	if result.OperationId != lifecycleRPCOperation || result.Generation != 7 || result.Phase != phase || string(result.PlanJsonUtf8) != raw || !bytes.Equal(result.PlanDigest, digest[:]) {
		t.Fatalf("原完整计划/独立协议代次/真实阶段失配: %v", result)
	}
}

func TestLifecycleRPCOriginalBindingAndRegistrationUnknown(t *testing.T) {
	// 错user/关闭组织不能取得预留；登记提交回复未知不意味着预留业务已完成。
	s, m, q := lifecycleRPCFixture(t)
	wrong := proto.Clone(q).(*pb.LifecycleRequest)
	wrong.UserId = 12
	if _, err := s.Reserve(context.Background(), wrong); err == nil || len(m.registration) > 0 || m.reserveCalls != 0 {
		t.Fatal("声明的他人绑定被接受")
	}
	wrong = proto.Clone(q).(*pb.LifecycleRequest)
	wrong.ClosingOrganizationIds = []uint64{22}
	if _, err := s.Reserve(context.Background(), wrong); err == nil || len(m.registration) > 0 {
		t.Fatal("浏览器式自选关闭组织被登记")
	}
	m.commitUnknown = true
	if _, err := s.Reserve(context.Background(), q); err == nil || m.reserveCalls != 0 || len(m.registration) == 0 {
		t.Fatal("登记未知被当作预留成功")
	}
	result, err := s.Reserve(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCReservedJSON, "FROZEN")
	wrong = proto.Clone(q).(*pb.LifecycleRequest)
	wrong.Generation = 41
	if _, err := s.Reserve(context.Background(), wrong); err == nil || m.reserveCalls != 1 {
		t.Fatal("身份epoch替换了固定协议generation")
	}
}

func TestLifecycleRPCPrepareReplaysOriginalInputDigestAndRejectsChanges(t *testing.T) {
	// Prepare扩充一次Retention后，旧输入摘要同号仍可重读；错digest/代次不能再调用业务。
	s, m, q := lifecycleRPCFixture(t)
	reserved, err := s.Reserve(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	q = proto.Clone(q).(*pb.LifecycleRequest)
	q.FixedPlanDigest = bytes.Clone(reserved.PlanDigest)
	wrong := proto.Clone(q).(*pb.LifecycleRequest)
	wrong.FixedPlanDigest = bytes.Repeat([]byte{0}, 32)
	if _, err = s.Prepare(context.Background(), wrong); err == nil || m.planCalls != 0 {
		t.Fatal("错摘要启动了保留业务")
	}
	prepared, err := s.Prepare(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, prepared, lifecycleRPCPreparedJSON, "PLANNED")
	replayed, err := s.Prepare(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, replayed, lifecycleRPCPreparedJSON, "PLANNED")
	wrong = proto.Clone(q).(*pb.LifecycleRequest)
	wrong.FixedPlanDigest = bytes.Clone(prepared.PlanDigest)
	if _, err = s.Prepare(context.Background(), wrong); err == nil || m.planCalls != 1 {
		t.Fatal("同号换摘要替换了原Prepare请求")
	}
}

func TestLifecycleRPCApplyWaitsForExistingCleanupAndReplaysTerminal(t *testing.T) {
	// Relations只到CLEANING；缺单文档清理回执不会提前成功，终态重放不重新依赖当前Account身份。
	s, m, q := lifecycleRPCFixture(t)
	reserved, err := s.Reserve(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	prepare := proto.Clone(q).(*pb.LifecycleRequest)
	prepare.FixedPlanDigest = bytes.Clone(reserved.PlanDigest)
	prepared, err := s.Prepare(context.Background(), prepare)
	if err != nil {
		t.Fatal(err)
	}
	apply := proto.Clone(q).(*pb.LifecycleRequest)
	apply.FixedPlanDigest = bytes.Clone(prepared.PlanDigest)
	wrong := proto.Clone(apply).(*pb.LifecycleRequest)
	wrong.Generation++
	if _, err = s.Apply(context.Background(), wrong); err == nil || m.relationCalls != 0 {
		t.Fatal("错代次推进了关系")
	}
	result, err := s.Apply(context.Background(), apply)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "CLEANING")
	if m.completeCalls != 0 {
		t.Fatal("一次Apply同步跨越了两个阶段")
	}
	result, err = s.Apply(context.Background(), apply)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "CLEANING")
	m.canComplete = true
	result, err = s.Apply(context.Background(), apply)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "SUCCEEDED")
	m.authorityUnavailable = true
	// 三种方法仍须各自原请求；终态不能接受空摘要或换摘要作为成功查询。
	result, err = s.Reserve(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "SUCCEEDED")
	result, err = s.Prepare(context.Background(), prepare)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "SUCCEEDED")
	result, err = s.Apply(context.Background(), apply)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRPCWant(t, result, lifecycleRPCPreparedJSON, "SUCCEEDED")
	if m.reserveCalls != 1 || m.planCalls != 1 || m.relationCalls != 1 || m.completeCalls != 2 {
		t.Fatal("终态重放重新执行了业务阶段")
	}
}
