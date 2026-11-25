package account

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"google.golang.org/protobuf/proto"
)

const authorizationOriginalID = "authorization-original-001"

// 只读SQL模型拒绝Exec/Prepare；所有绑定与状态判断走真实用例及repository SELECT。
// 单次联查模拟同一提交快照，防止新增RPC靠写表或反向网络调用完成证明。
type authorizationResultModel struct {
	facts repository.AuthorizationResultFacts
	reads int
}

type authorizationResultConnector struct{ model *authorizationResultModel }

func (c authorizationResultConnector) Connect(context.Context) (driver.Conn, error) {
	return &authorizationResultConn{c.model}, nil
}
func (authorizationResultConnector) Driver() driver.Driver { return operationResultDriver{} }

type authorizationResultConn struct{ model *authorizationResultModel }

func (*authorizationResultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("AUTHORIZATION_TEST_WRITE_DENIED")
}
func (*authorizationResultConn) Close() error { return nil }
func (*authorizationResultConn) Begin() (driver.Tx, error) {
	return nil, errors.New("AUTHORIZATION_TEST_TRANSACTION_DENIED")
}
func (c *authorizationResultConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "FROM authorization_change a") || !strings.Contains(query, "LEFT JOIN authorization_ack") {
		return nil, errors.New("AUTHORIZATION_TEST_QUERY_UNEXPECTED")
	}
	c.model.reads++
	if args[0].Value != authorizationOriginalID {
		return &operationResultRows{}, nil
	}
	f := c.model.facts
	return &operationResultRows{values: []driver.Value{
		f.Actor, f.Scope, f.Epoch, f.State, f.Target, f.Participants, f.GateEpoch, f.GateState, f.GateRevocation,
		f.TaskKind, f.TaskOperation, f.TaskState, f.Payload, f.ReceiptActor, f.ReceiptKind, f.ReceiptState, f.ReceiptTarget, f.Fingerprint, f.ReceiptTerminal,
		f.CloseUser, f.CloseState, f.Capture, f.Organization, f.OrganizationActor, f.OrganizationState, f.OrganizationPlan,
		f.AckEpoch, f.AckPhase, f.AckDigest, f.AckCompleted,
	}}, nil
}

func authorizationFixture(t *testing.T) (*AuthorizationResults, *authorizationResultModel, *pb.FenceRequest, *pb.OperationQuery) {
	t.Helper()
	f := repository.AuthorizationResultFacts{
		Actor: 7, ReceiptActor: 7, Epoch: 2, Scope: "credential:7", State: "SUCCEEDED", GateEpoch: 2, GateState: "BLOCKED", GateRevocation: authorizationOriginalID,
		TaskKind: "CREDENTIAL_CHANGE", TaskOperation: authorizationOriginalID, TaskState: "SUCCEEDED", ReceiptKind: "CHANGE_PASSWORD", ReceiptState: "SUCCEEDED", ReceiptTerminal: true,
		Participants: []byte(`["document"]`), Fingerprint: bytes.Repeat([]byte{3}, 32),
		Target:  []byte(`{"user_id":"7","scope_key":"credential:7","session_hash":""}`),
		Payload: []byte(`{"kind":"CHANGE_PASSWORD","user_id":"7","session_hash":"","scope_key":"credential:7","closed_epoch":"2"}`),
	}
	f.ReceiptTarget = append([]byte(nil), f.Target...)
	request := &pb.FenceRequest{RevocationId: authorizationOriginalID, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: 2}}, TargetDigest: platform.SourceClosureDigest(f.Scope, 2)}
	f.AckEpoch, f.AckPhase, f.AckCompleted, f.AckDigest = 2, "OUTPUT_STOPPED", true, platform.BootstrapDigest(request, "document")
	m := &authorizationResultModel{facts: f}
	db := sql.OpenDB(authorizationResultConnector{m})
	t.Cleanup(func() { _ = db.Close() })
	// 固定映射金样：完整规范目标均参与摘要，结构字段顺序是协议定义的一部分。
	raw := []byte(`{"receipt_operation_id":"` + authorizationOriginalID + `","authorization_target":{"scope_key":"credential:7","session_hash":"","user_id":"7"},"operation_target":{"scope_key":"credential:7","session_hash":"","user_id":"7"}}`)
	digest := sha256.Sum256(raw)
	query := &pb.OperationQuery{OperationId: authorizationOriginalID, RequestFingerprint: append([]byte(nil), f.Fingerprint...), Target: &pb.Target{WorkspaceEpoch: 2, Purpose: "ACCOUNT_AUTHORIZATION_CHANGE:credential:7:" + hex.EncodeToString(digest[:])}}
	return NewAuthorizationResults(db), m, request, query
}

func TestAuthorizationRPCOriginalProofAndFixedQueryBinding(t *testing.T) {
	// 回执/真实ACK/协议目标任何一项失配都不能成功；旧epoch终态可在新来源OPEN后重读。
	s, m, fence, query := authorizationFixture(t)
	ack, err := s.Fence(context.Background(), fence)
	if err != nil || !platform.MatchesBootstrapAck(fence, ack, "account") {
		t.Fatalf("原真实ACK不能反向证明: %v %v", ack, err)
	}
	first, err := s.Get(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != pb.OperationResult_SUCCEEDED || first.Generation != 1 || first.TerminalRevision != 1 || len(first.CurrentStateJsonUtf8) != 0 {
		t.Fatal("真实授权终态未固定")
	}
	digest := sha256.Sum256(first.FixedResultJsonUtf8)
	if !bytes.Equal(first.ResultDigest, digest[:]) {
		t.Fatal("固定结果摘要失配")
	}
	m.facts.GateEpoch, m.facts.GateState, m.facts.GateRevocation = 3, "OPEN", ""
	second, err := s.Get(context.Background(), query)
	if err != nil || !proto.Equal(first, second) {
		t.Fatal("新epoch改写了旧关闭结果")
	}
	if _, err = s.Fence(context.Background(), fence); err != nil {
		t.Fatal("旧终态反向Fence不能重读", err)
	}
	for name, change := range map[string]func(*pb.OperationQuery){
		"document": func(q *pb.OperationQuery) { q.Target.DocumentId = 1 }, "unit": func(q *pb.OperationQuery) { q.Target.UnitId = 1 },
		"branch": func(q *pb.OperationQuery) { q.Target.BranchId = 1 }, "structure": func(q *pb.OperationQuery) { q.Target.StructureId = 1 },
		"epoch": func(q *pb.OperationQuery) { q.Target.WorkspaceEpoch++ }, "purpose": func(q *pb.OperationQuery) { q.Target.Purpose += "changed" },
		"fingerprint": func(q *pb.OperationQuery) { q.RequestFingerprint[0]++ },
	} {
		t.Run(name, func(t *testing.T) {
			q := proto.Clone(query).(*pb.OperationQuery)
			change(q)
			if _, err := s.Get(context.Background(), q); err == nil {
				t.Fatal("输入字段被忽略")
			}
		})
	}
	bad := proto.Clone(fence).(*pb.FenceRequest)
	bad.Generation++
	if _, err = s.Fence(context.Background(), bad); err == nil {
		t.Fatal("错误generation签出ACK")
	}
	bad = proto.Clone(fence).(*pb.FenceRequest)
	bad.ClosedScopes[0].Epoch++
	bad.TargetDigest = platform.SourceClosureDigest(m.facts.Scope, bad.ClosedScopes[0].Epoch)
	if _, err = s.Fence(context.Background(), bad); err == nil {
		t.Fatal("新epoch覆盖原范围")
	}
	bad = proto.Clone(fence).(*pb.FenceRequest)
	bad.RevocationId = "authorization-never-existed"
	if _, err = s.Fence(context.Background(), bad); err == nil {
		t.Fatal("未知范围凭输入签出ACK")
	}
}

func TestAuthorizationRPCPendingAndCorruptEvidenceNeverAcknowledge(t *testing.T) {
	// 未完成原Doc屏障只是PROCESSING；新gate、空参与者、假摘要或未完成ACK都不能补成功。
	s, m, fence, query := authorizationFixture(t)
	original := m.facts
	m.facts.State, m.facts.TaskState, m.facts.ReceiptState, m.facts.ReceiptTerminal = "FENCING", "RUNNING", "PREPARING", false
	m.facts.AckEpoch, m.facts.AckPhase, m.facts.AckCompleted, m.facts.AckDigest = 0, "", false, nil
	ack, err := s.Fence(context.Background(), fence)
	result, getErr := s.Get(context.Background(), query)
	if err != nil || getErr != nil || ack.State != pb.FenceResult_PROCESSING || len(ack.ResultDigest) != 0 || result.State != pb.OperationResult_PROCESSING || result.TerminalRevision != 0 || len(result.ResultDigest) != 0 || len(result.FixedResultJsonUtf8) != 0 {
		t.Fatal("待定原范围被签成终态")
	}
	m.facts.GateEpoch++
	if _, err = s.Fence(context.Background(), fence); err == nil {
		t.Fatal("待定范围借新gate成功")
	}
	for name, change := range map[string]func(*repository.AuthorizationResultFacts){
		"ack_missing":          func(f *repository.AuthorizationResultFacts) { f.AckDigest = nil },
		"ack_wrong":            func(f *repository.AuthorizationResultFacts) { f.AckDigest = bytes.Repeat([]byte{9}, 32) },
		"ack_uncompleted":      func(f *repository.AuthorizationResultFacts) { f.AckCompleted = false },
		"empty_plan":           func(f *repository.AuthorizationResultFacts) { f.Participants = []byte(`[]`) },
		"receipt_not_terminal": func(f *repository.AuthorizationResultFacts) { f.ReceiptTerminal = false },
		"unknown_task":         func(f *repository.AuthorizationResultFacts) { f.TaskKind = "BOOTSTRAP_FENCE" },
		"payload_changed": func(f *repository.AuthorizationResultFacts) {
			f.Payload = bytes.ReplaceAll(f.Payload, []byte(`"2"`), []byte(`"3"`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			m.facts = original
			change(&m.facts)
			if _, err = s.Fence(context.Background(), fence); err == nil {
				t.Fatal("损坏持久事实仍签出ACK")
			}
			if _, err = s.Get(context.Background(), query); err == nil {
				t.Fatal("损坏持久事实仍固定成功")
			}
		})
	}
}

func TestAuthorizationRPCAccountCloseChildUsesOriginalParentAndScope(t *testing.T) {
	// 子范围已ACK并不代表账号注销成功；其权限/指纹须回溯原父capture，不能套用别人父号。
	s, m, _, _ := authorizationFixture(t)
	parent := "account-close-parent-original"
	capture := domain.AccountCloseCapture{SessionHash: strings.Repeat("a", 64), CredentialEpoch: 2, IdentityEpoch: 4,
		Binding: &platform.AccountClosureBinding{Operation: parent, User: 7, IdentityEpoch: 4, ClosingOrganizations: []uint64{}},
		Scopes:  []domain.AccountCloseScope{{Revocation: authorizationOriginalID, Scope: "identity:7", Epoch: 4}}}
	f := m.facts
	f.TaskKind, f.TaskOperation, f.ReceiptKind, f.ReceiptState, f.ReceiptTerminal = domain.AccountCloseFenceKind, parent, "ACCOUNT_CLOSE", "PREPARING", false
	f.Scope, f.Epoch, f.GateEpoch, f.CloseUser, f.CloseState = "identity:7", 4, 4, 7, "CLEANING"
	f.Capture, _ = json.Marshal(capture)
	f.Payload, _ = json.Marshal(domain.AccountCloseFencePayload{Operation: parent, User: 7, Revocation: authorizationOriginalID, Scope: f.Scope, Epoch: f.Epoch})
	f.Target = []byte(`{"account_close_operation":"` + parent + `","scope_key":"identity:7","epoch":4}`)
	f.ReceiptTarget = []byte(`{"user_id":"7","session_hash":"` + capture.SessionHash + `"}`)
	fence := &pb.FenceRequest{RevocationId: authorizationOriginalID, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: f.Epoch}}, TargetDigest: platform.SourceClosureDigest(f.Scope, f.Epoch)}
	f.AckEpoch, f.AckDigest = f.Epoch, platform.BootstrapDigest(fence, "document")
	m.facts = f
	target, err := authorizationQueryTarget(f)
	if err != nil {
		t.Fatal(err)
	}
	query := &pb.OperationQuery{OperationId: authorizationOriginalID, Target: target, RequestFingerprint: f.Fingerprint}
	result, err := s.Get(context.Background(), query)
	if err != nil || result.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("原子范围被错误绑定父全局终态", err)
	}
	if _, err = s.Fence(context.Background(), fence); err != nil {
		t.Fatal(err)
	}
	m.facts.CloseUser = 8
	if _, err = s.Fence(context.Background(), fence); err == nil {
		t.Fatal("换用他人账号关闭记录签出ACK")
	}
	m.facts = f
	capture.Scopes[0].Epoch++
	m.facts.Capture, _ = json.Marshal(capture)
	if _, err = s.Get(context.Background(), query); err == nil {
		t.Fatal("capture原代次失配仍读取终态")
	}
	if m.reads != 4 {
		t.Fatalf("查询未保持每次单条快照: %d", m.reads)
	}
}

func TestAuthorizationRPCExistingSourceFamiliesKeepOriginalBindings(t *testing.T) {
	// 原授权主体不总是被撤权者；组织关闭ACK也先于清理终态。覆盖这些实际分派边界，
	// 防止凭据规则被错误套在其他原持久任务上。
	for _, family := range []string{"session", "organization_member", "governance", "organization_close"} {
		t.Run(family, func(t *testing.T) {
			s, m, _, _ := authorizationFixture(t)
			f := m.facts
			f.ReceiptTarget = []byte(`{"path":"/v1/account/organizations/11/members/8"}`)
			switch family {
			case "session":
				hash := strings.Repeat("a", 64)
				f.Scope, f.TaskKind, f.ReceiptKind = "session:"+hash, "SESSION_LOGOUT", "SESSION_LOGOUT"
				f.Payload, _ = json.Marshal(domain.SessionClosure{SessionHash: hash, Scope: f.Scope, Epoch: "2"})
				f.Target = []byte(`{"session_hash":"` + hash + `","scope_key":"` + f.Scope + `"}`)
				f.ReceiptTarget = f.Target
			case "organization_member":
				f.Scope, f.TaskKind, f.ReceiptKind = "org-member:11:8", "ORGANIZATION_CHANGE", "ORG_ROLE"
				f.Payload, _ = json.Marshal(domain.SessionClosure{Kind: "ORG_ROLE", UserID: "8", OrganizationID: "11", Role: "MEMBER", Scope: f.Scope, Epoch: "2"})
				f.Target = f.Payload
			case "governance":
				f.Actor, f.ReceiptActor = 0, 0
				f.Scope, f.TaskKind, f.ReceiptKind = "platform-admin:8", "PLATFORM_GOVERNANCE_CHANGE", "PLATFORM_GOVERNANCE_OPERATOR"
				f.Payload, _ = json.Marshal(domain.SessionClosure{Kind: "PLATFORM_GOVERNANCE_REVOKE", UserID: "8", Scope: f.Scope, Epoch: "2"})
				f.Target = f.Payload
				f.ReceiptTarget, _ = json.Marshal(GovernanceOperatorInput{Operation: authorizationOriginalID, User: 8, Action: "REVOKE", Expected: 2})
			case "organization_close":
				f.Scope, f.TaskKind, f.ReceiptKind = "org-member:11:7", repository.OrganizationCloseRecoveryKind, "ORG_CLOSE"
				f.Organization, f.OrganizationActor, f.OrganizationState = 11, 7, "APPLIED"
				f.TaskState, f.ReceiptState, f.ReceiptTerminal = "RUNNING", "PREPARING", false
				plan := domain.OrganizationClosePlan{Binding: platform.OrganizationClosureBinding{Operation: authorizationOriginalID, Organization: 11, User: 7, Revision: 2, MemberRevision: 3, Epoch: 2}}
				f.Payload, _ = json.Marshal(plan)
				f.Target = f.Payload
				plan.PlanDigest = strings.Repeat("b", 64)
				f.OrganizationPlan, _ = json.Marshal(plan)
			}
			fence := &pb.FenceRequest{RevocationId: authorizationOriginalID, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: f.Epoch}}, TargetDigest: platform.SourceClosureDigest(f.Scope, f.Epoch)}
			if family == "session" {
				fence.TargetDigest = platform.LogoutTargetDigest(strings.TrimPrefix(f.Scope, "session:"))
			}
			f.AckDigest = platform.BootstrapDigest(fence, "document")
			m.facts = f
			target, err := authorizationQueryTarget(f)
			if err != nil {
				t.Fatal(err)
			}
			ack, err := s.Fence(context.Background(), fence)
			if err != nil || !platform.MatchesBootstrapAck(fence, ack, "account") {
				t.Fatal("原来源ACK被错误拒绝", err)
			}
			result, err := s.Get(context.Background(), &pb.OperationQuery{OperationId: authorizationOriginalID, Target: target, RequestFingerprint: f.Fingerprint})
			if err != nil || result.State != pb.OperationResult_SUCCEEDED {
				t.Fatal("原来源查询无法重读", err)
			}
		})
	}
}
