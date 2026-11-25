# services/document/internal/logic/document/output_control_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/215-output-control-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`RegisterCollaboration`、`FencePeer`、`Fences`、`NewFences`、`Close`、`prepare`、`finish`、`stopEdgeAndSettle`、`localFlow`、`LocalOutputs`、`NewLocalOutputs`、`Add`、`Remove`、`Has`、`Stop`、`recoverExited`、`Ready`、`OutputRecoveryCode`、`ResponseRegistration`、`EdgeInstance`、`SourcesWire`、`Register`、`RegisterPublication`、`registerResponse`、`writeResponseRegistration`、`RegisterAccount`、`EdgeOutput`、`ProcessVerifier`、`Outputs`、`NewOutputs`、`Self`、`Local`、`EdgeCall`、`Initialize`、`Settle`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// collaboration_output.go：保留原实现，按业务合并文件。

// 长连接来源从已兑换准入读取，不能接受Node自报更少的来源绕过关闭。
func (m *Outputs) RegisterCollaboration(ctx context.Context, request domain.CollaborationOutput) (string, error) {
	sources, err := m.repository.AdmissionSources(ctx, request.Admission)
	if err != nil {
		return "", err
	}
	edge, err := m.EdgeInstance(ctx, request.Handler)
	if err != nil || edge == nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return "", err
	}
	current, err := repository.LockedAdmission(ctx, tx, request.Admission)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return "", domain.Violation(domain.PermissionDenied, "ADMISSION_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != request.Grant || binding.Target == nil {
		return "", domain.Violation(domain.PermissionDenied, "GRANT_BINDING_INVALID")
	}
	if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
		return "", err
	}
	lease := domain.OutputLease{ID: platform.NewTraceID(), Instance: edge.ID, Handler: request.Handler, Sources: SourcesWire(sources), Document: binding.Target.DocumentId, Admission: request.Admission, Authority: "collaboration"}
	if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
		return "", err
	}
	if err := repository.MarkWebSocketDelivery(ctx, tx, lease.ID, current.Expires); err != nil {
		return "", err
	}
	if tx.Commit() != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	// Node自己的socket停止不足以证明Edge缓冲排空；开放也必须取得真实Edge确认。
	var ack struct {
		State string `json:"state"`
		ID    string `json:"instance_id"`
	}
	if m.edge.Call(ctx, "/infra/output/open", lease, &ack) != nil || ack.State != "OPEN" || ack.ID != edge.ID {
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return lease.ID, nil
}

// fence.go：保留原实现，按业务合并文件。

type FencePeer interface {
	Close(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
}
type Fences struct {
	db         *sql.DB
	repository *repository.Fences
	outputs    *Outputs
	node       FencePeer
}

func NewFences(db *sql.DB, repository *repository.Fences, outputs *Outputs, node FencePeer) *Fences {
	return &Fences{db: db, repository: repository, outputs: outputs, node: node}
}

// Close先关闭准入，再等待真实两段输出与Node ACK，持久确认之前不返回成功。
func (s *Fences) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	done, err := s.prepare(ctx, request)
	if err != nil {
		return nil, err
	}
	if done {
		return platform.BootstrapAck(request, "document"), nil
	}
	if err := s.outputs.Local().Stop(ctx, request.ClosedScopes[0]); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LOCAL_OUTPUT_STOP_UNKNOWN")
	}
	if err := s.stopEdgeAndSettle(ctx, request); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_STOP_UNKNOWN")
	}
	ack, err := s.node.Close(ctx, request)
	if err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "NODE_FENCE_UNAVAILABLE")
	}
	if err := s.finish(ctx, request, ack); err != nil {
		return nil, err
	}
	return platform.BootstrapAck(request, "document"), nil
}
func (s *Fences) prepare(ctx context.Context, request *pb.FenceRequest) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	closed := request.ClosedScopes[0]
	// 新准入与关闭器都遵循gate→authorization_change，不能交叉持锁。
	if err := repository.BlockFenceScope(ctx, tx, closed.ScopeKey, closed.Epoch, request.RevocationId); err != nil {
		return false, err
	}
	// 原Account操作已权威拒绝并撤回预留时，迟到的同号Fence必须整体回滚，
	// 不能再次封住已恢复的身份来源。常规会话/凭据关闭不受该专项终态影响。
	cancelled, err := repository.AccountRetirementFenceCancelled(ctx, tx, request.RevocationId, closed.ScopeKey, closed.Epoch)
	if err != nil {
		return false, err
	}
	if cancelled {
		return false, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_CANCELLED")
	}
	binding := map[string]string{"bootstrap_target_digest": hex.EncodeToString(request.TargetDigest), "generation": "1"}
	current, err := repository.LockedFence(ctx, tx, request.RevocationId)
	if err == nil {
		var prior map[string]string
		if json.Unmarshal(current.TargetPermission, &prior) != nil || prior["bootstrap_target_digest"] != binding["bootstrap_target_digest"] || prior["generation"] != "1" || current.Scope != closed.ScopeKey || current.Epoch != closed.Epoch {
			return false, domain.Violation(domain.StateConflict, "FENCE_ID_CONFLICT")
		}
		if current.State == "SUCCEEDED" {
			return true, nil
		}
	} else if err != sql.ErrNoRows {
		return false, err
	} else {
		encoded, _ := json.Marshal(binding)
		if err := repository.InsertFence(ctx, tx, request.RevocationId, closed.ScopeKey, closed.Epoch, encoded); err != nil {
			return false, err
		}
	}
	if err := repository.CloseSourceAdmissions(ctx, tx, closed.ScopeKey, closed.Epoch); err != nil {
		return false, err
	}
	return false, tx.Commit()
}
func (s *Fences) finish(ctx context.Context, request *pb.FenceRequest, node *pb.FenceResult) error {
	if !platform.MatchesBootstrapAck(request, node, "collaboration") {
		return domain.Violation(domain.TemporarilyUnavailable, "NODE_FENCE_NOT_CONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := request.ClosedScopes[0]
	if err := repository.RecordNodeAck(ctx, tx, request.RevocationId, scope.ScopeKey, scope.Epoch, node.ResultDigest); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_UNAVAILABLE")
	}
	if err := repository.CompleteFence(ctx, tx, request.RevocationId); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_COMMIT_UNKNOWN")
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return nil
}

// fence_outputs.go：保留原实现，按业务合并文件。

// 即使没有在线handler，固定Edge参与者也接收关闭水位，阻止提交前登记而迟到的open。
func (s *Fences) stopEdgeAndSettle(ctx context.Context, req *pb.FenceRequest) error {
	closed := req.ClosedScopes[0]
	records, err := s.repository.Unsettled(ctx)
	if err != nil {
		return err
	}
	var matched []domain.OutputLease
	for _, record := range records {
		var lease domain.OutputLease
		if json.Unmarshal(record.Payload, &lease) != nil || lease.ID != record.ID {
			return errors.New("OUTPUT_RECORD_INVALID")
		}
		for _, source := range lease.Sources {
			epoch, err := strconv.ParseUint(source.Epoch, 10, 64)
			if err != nil {
				return err
			}
			if source.Key == closed.ScopeKey && epoch <= closed.Epoch {
				matched = append(matched, lease)
				break
			}
		}
	}
	edgeID := strings.TrimSuffix(s.outputs.Self().ID, "-document") + "-edge"
	for _, lease := range matched {
		if lease.Instance != edgeID && (lease.Instance != s.outputs.Self().ID || !s.outputs.Local().Has(lease.ID)) {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
	}
	// 固定入口参与者即使当前没有在线handler也接收关闭水位，阻止commit前已登记但迟到的open。
	var ack struct {
		State      string `json:"state"`
		ID         string `json:"instance_id"`
		Scope      string `json:"scope_key"`
		Epoch      string `json:"epoch"`
		Revocation string `json:"revocation_id"`
		Digest     string `json:"target_digest"`
	}
	input := map[string]string{"instance_id": edgeID, "scope_key": closed.ScopeKey, "epoch": strconv.FormatUint(closed.Epoch, 10), "revocation_id": req.RevocationId, "target_digest": hex.EncodeToString(req.TargetDigest)}
	if err = s.outputs.EdgeCall(ctx, "/infra/output/fence", input, &ack); err != nil {
		return err
	}
	if ack.State != "OUTPUT_STOPPED" || ack.ID != edgeID || ack.Scope != closed.ScopeKey || ack.Epoch != input["epoch"] || ack.Revocation != req.RevocationId || ack.Digest != input["target_digest"] {
		return errors.New("EDGE_ACK_BINDING_INVALID")
	}
	for _, lease := range matched {
		if err = s.outputs.Settle(ctx, lease.ID, lease.Instance); err != nil {
			return err
		}
		if lease.Instance == s.outputs.Self().ID {
			s.outputs.Local().Remove(lease.ID)
		}
	}
	digest := platform.BootstrapDigest(req, "edge")
	return s.repository.RecordEdgeAck(ctx, req.RevocationId, closed.ScopeKey, closed.Epoch, digest)
}

// local_outputs.go：保留原实现，按业务合并文件。

// Stop必须等待真实done；发出取消信号以后，输出仍可能持有缓冲或正阻塞写入。
type localFlow struct {
	sources []*pb.SourceEpoch
	cancel  func()
	done    <-chan struct{}
}
type LocalOutputs struct {
	mu    sync.Mutex
	flows map[string]localFlow
}

func NewLocalOutputs() *LocalOutputs { return &LocalOutputs{flows: map[string]localFlow{}} }
func (m *LocalOutputs) Add(id string, sources []*pb.SourceEpoch, cancel func(), done <-chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flows[id] = localFlow{sources, cancel, done}
}
func (m *LocalOutputs) Remove(id string) { m.mu.Lock(); defer m.mu.Unlock(); delete(m.flows, id) }
func (m *LocalOutputs) Has(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.flows[id]
	return ok
}
func (m *LocalOutputs) Stop(ctx context.Context, closed *pb.SourceEpoch) error {
	m.mu.Lock()
	var pending []localFlow
	for _, f := range m.flows {
		for _, s := range f.sources {
			if s.ScopeKey == closed.ScopeKey && s.Epoch <= closed.Epoch {
				pending = append(pending, f)
				break
			}
		}
	}
	m.mu.Unlock()
	for _, f := range pending {
		f.cancel()
	}
	for _, f := range pending {
		select {
		case <-f.done:
		case <-ctx.Done():
			return errors.New("OUTPUT_STOP_UNKNOWN")
		}
	}
	return nil
}

// output_recovery.go：保留原实现，按业务合并文件。

// 旧实例只能凭指定PID归属证据完成恢复；不以心跳、租约或TTL推测停止。
func (m *Outputs) recoverExited(ctx context.Context) error {
	instances, err := m.repository.ActiveInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if instance.ID == m.self.ID {
			continue
		}
		role := ""
		for _, candidate := range []string{"document", "edge"} {
			if strings.HasSuffix(instance.ID, "-"+candidate) {
				role = candidate
			}
		}
		if role == "" {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
		owner := strings.TrimSuffix(instance.ID, "-"+role)
		state, err := m.process.Verify(ctx, instance.PID, role, owner)
		if err != nil {
			return err
		}
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := m.repository.ConfirmExit(ctx, tx, instance.ID, state); err != nil {
			tx.Rollback()
			return err
		}
		if tx.Commit() != nil {
			return errors.New("OUTPUT_RECOVERY_COMMIT_UNKNOWN")
		}
	}
	return nil
}
func (m *Outputs) Ready(ctx context.Context) error {
	count, err := m.repository.UnrecoveredCount(ctx)
	if err != nil {
		return errors.New("OUTPUT_RECOVERY_UNAVAILABLE")
	}
	if count != 0 {
		return errors.New("OUTPUT_RECOVERY_REQUIRED")
	}
	pending, err := m.repository.Pending(ctx)
	if err != nil {
		return err
	}
	edgeID := strings.TrimSuffix(m.self.ID, "-document") + "-edge"
	for _, output := range pending {
		if output.Instance == m.self.ID {
			if !m.local.Has(output.ID) {
				// Pending之后可能完成持久结算并移除handler。旧快照不能把正常结束
				// 当成丢失处理器；只有原记录的提交成功事实才允许通过此竞争窗口。
				settled, err := m.repository.Settled(ctx, output.ID, output.Instance)
				if err != nil || !settled {
					return errors.New("OUTPUT_HANDLER_UNKNOWN")
				}
			}
		} else if output.Instance != edgeID {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
	}
	return nil
}

// 恢复日志只输出白名单边界码，避免驱动错误带出私有配置或SQL。
func OutputRecoveryCode(err error) string {
	switch err.Error() {
	case "OUTPUT_EXIT_PROOF_UNAVAILABLE", "OUTPUT_EXIT_PROOF_DEADLINE", "OLD_OUTPUT_PROCESS_STILL_RUNNING", "OUTPUT_INSTANCE_UNKNOWN", "OUTPUT_RECOVERY_INVALID", "OUTPUT_RECOVERY_COMMIT_UNKNOWN":
		return err.Error()
	default:
		return "OUTPUT_RECOVERY_UNKNOWN"
	}
}

// output_registration.go：保留原实现，按业务合并文件。

type ResponseRegistration struct {
	Local domain.OutputLease
	Edge  *domain.OutputLease
}

func (m *Outputs) EdgeInstance(ctx context.Context, handler string) (*domain.OutputInstance, error) {
	if handler == "" {
		return nil, nil
	}
	var instance domain.OutputInstance
	if err := m.edge.Call(ctx, "/infra/output/info", nil, &instance); err != nil {
		return nil, err
	}
	if instance.ID != strings.TrimSuffix(m.self.ID, "-document")+"-edge" {
		return nil, errors.New("EDGE_OUTPUT_INSTANCE_CHANGED")
	}
	return &instance, nil
}
func SourcesWire(sources []*pb.SourceEpoch) []domain.LeaseSource {
	list := make([]domain.LeaseSource, 0, len(sources))
	for _, s := range sources {
		list = append(list, domain.LeaseSource{Key: s.ScopeKey, Epoch: strconv.FormatUint(s.Epoch, 10)})
	}
	return list
}

// 租约与业务读取/变更共用调用方的原tx；commit之前不开放入口输出。
func (m *Outputs) Register(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64, admission string, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)
}

func (m *Outputs) RegisterPublication(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document, epoch, release uint64, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if document == 0 || epoch == 0 || release == 0 || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_OUTPUT_BINDING_INVALID")
	}
	matched := false
	for _, source := range sources {
		if source != nil && source.ScopeKey == "publication:"+domain.TextID(document) && source.Epoch == epoch {
			matched = true
		}
	}
	if !matched {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_OUTPUT_BINDING_INVALID")
	}
	return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)
}

func (m *Outputs) registerResponse(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	return m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)
}

// 只登记原事务内的租约。普通路径自行锁来源；搜索路径必须持有私有锁证明。
func (m *Outputs) writeResponseRegistration(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	id := platform.NewTraceID()
	local := template
	local.ID, local.Instance, local.Handler, local.Sources = id, m.self.ID, id, SourcesWire(sources)
	o := &ResponseRegistration{Local: local}
	if err := repository.InsertOutputLease(ctx, tx, o.Local); err != nil {
		return nil, err
	}
	if edge != nil {
		if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
			return nil, err
		}
		lease := template
		lease.ID, lease.Instance, lease.Handler, lease.Sources = platform.NewTraceID(), edge.ID, handler, SourcesWire(sources)
		if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
			return nil, err
		}
		o.Edge = &lease
	}
	return o, nil
}

func (m *Outputs) RegisterAccount(ctx context.Context, handler string, sources []*pb.SourceEpoch) error {
	edge, err := m.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return err
	}
	if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
		return err
	}
	lease := domain.OutputLease{ID: platform.NewTraceID(), Instance: edge.ID, Handler: handler, Sources: SourcesWire(sources), Authority: "account"}
	if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	var ack struct {
		State string `json:"state"`
		ID    string `json:"instance_id"`
	}
	if m.edge.Call(ctx, "/infra/output/open", lease, &ack) != nil || ack.State != "OPEN" || ack.ID != edge.ID {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return nil
}

// outputs.go：保留原实现，按业务合并文件。

type EdgeOutput interface {
	Call(context.Context, string, any, any) error
}
type ProcessVerifier interface {
	Verify(context.Context, uint64, string, string) (string, error)
}

// Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。
type Outputs struct {
	db         *sql.DB
	repository *repository.Outputs
	local      *LocalOutputs
	self       domain.OutputInstance
	edge       EdgeOutput
	process    ProcessVerifier
}

func NewOutputs(db *sql.DB, repository *repository.Outputs, self domain.OutputInstance, edge EdgeOutput, process ProcessVerifier) *Outputs {
	return &Outputs{db: db, repository: repository, local: NewLocalOutputs(), self: self, edge: edge, process: process}
}
func (m *Outputs) Self() domain.OutputInstance { return m.self }
func (m *Outputs) Local() *LocalOutputs        { return m.local }
func (m *Outputs) EdgeCall(ctx context.Context, path string, input, result any) error {
	return m.edge.Call(ctx, path, input, result)
}
func (m *Outputs) Initialize(ctx context.Context) error {
	if err := m.recoverExited(ctx); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := repository.EnsureInstance(ctx, tx, m.self); err != nil {
		return err
	}
	return tx.Commit()
}
func (m *Outputs) Settle(ctx context.Context, id, instance string) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := m.repository.Settle(ctx, tx, id, instance); err != nil {
		return err
	}
	return tx.Commit()
}
````

UTF-8/LF全文SHA-256：`0e263b425d4b86c6766218b360515ec3f67a57309cb3fde044ee5d758ca2386b`。[参考实现](../../../../services/document/internal/logic/document/output_control_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
