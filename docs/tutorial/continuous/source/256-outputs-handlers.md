# services/document/internal/handler/document/outputs_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/256-outputs-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`RegisterCollaboration`、`EditSessions`、`NewEditSessions`、`Create`、`Bootstrap`、`Outputs`、`NewOutputs`、`Bind`、`RegisterAccount`、`Settled`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"net/http"
	"strings"
	"time"
)

// collaboration_output.go：保留原实现，按业务合并文件。

func (h *Outputs) RegisterCollaboration(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Admission string `json:"admission_id"`
		Grant     string `json:"grant_id"`
		Handler   string `json:"handler_id"`
	}
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.Handler) {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	lease, err := h.usecase.RegisterCollaboration(r.Context(), domain.CollaborationOutput{Admission: input.Admission, Grant: input.Grant, Handler: input.Handler})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "OPEN", "lease_id": lease})
}

// edit_sessions.go：保留原实现，按业务合并文件。

type EditSessions struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.EditSessions
}

func NewEditSessions(identity *Identity, requests *Transactions, usecase *logic.EditSessions) *EditSessions {
	return &EditSessions{identity: identity, requests: requests, usecase: usecase}
}
func (h *EditSessions) Create(w http.ResponseWriter, r *http.Request) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var input dto.EditSessionCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidOperationID(input.ClientSessionId) || (input.WorkspaceKind != "BRANCH" && input.WorkspaceKind != "MAIN") {
		httpx.Fail(w, 400, "EDIT_SESSION_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "EDIT_SESSION", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Create(r.Context(), actor, document, unit, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "EDIT_SESSION", input, 200, run, present)
}
func (h *EditSessions) Bootstrap(w http.ResponseWriter, r *http.Request) {
	// 服务证书只授权这个内部入口，实际准入/来源/工作区仍由业务用例持锁核验。
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Admission string `json:"admission_id"`
		Grant     string `json:"grant_id"`
	}
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	result, err := h.usecase.Bootstrap(r.Context(), input.Admission, input.Grant)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

// outputs.go：保留原实现，按业务合并文件。

type Outputs struct{ usecase *logic.Outputs }

func NewOutputs(usecase *logic.Outputs) *Outputs { return &Outputs{usecase: usecase} }

// Bind登记真实取消/完成回调。用例在COMMIT前调用，传输层随后才发送正文。
func (h *Outputs) Bind(w http.ResponseWriter, registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) *ResponseOutput {
	output := &ResponseOutput{m: h.usecase, local: registration.Local, edge: registration.Edge, done: make(chan struct{}), w: w, stopped: make(chan struct{})}
	h.usecase.Local().Add(output.local.ID, sources, func() {
		output.once.Do(func() { close(output.stopped); _ = http.NewResponseController(w).SetWriteDeadline(time.Now()) })
	}, output.done)
	return output
}
func (h *Outputs) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Handler string               `json:"handler_id"`
		Sources []domain.LeaseSource `json:"sources"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 65536))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || !domain.ValidOperationID(input.Handler) || len(input.Sources) == 0 || len(input.Sources) > 512 {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	sources := make([]*pb.SourceEpoch, 0, len(input.Sources))
	for _, wire := range input.Sources {
		epoch, err := domain.CanonicalID(wire.Epoch)
		if err != nil || !platform.ValidAccountOutputScope(wire.Key) {
			httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
			return
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: wire.Key, Epoch: epoch})
	}
	if err := h.usecase.RegisterAccount(r.Context(), input.Handler, sources); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "OPEN"})
}
func (h *Outputs) Settled(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "edge") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		ID       string `json:"lease_id"`
		Instance string `json:"instance_id"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&input) != nil || input.Instance != strings.TrimSuffix(h.usecase.Self().ID, "-document")+"-edge" {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	if err := h.usecase.Settle(r.Context(), input.ID, input.Instance); err != nil {
		httpx.Fail(w, 503, "OUTPUT_SETTLEMENT_UNKNOWN")
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "SETTLED"})
}
````

UTF-8/LF全文SHA-256：`3aecff8d90b24275fbaa1e9339a6b9352297c86d95f3b92e1369768ccd933a4a`。[参考实现](../../../../services/document/internal/handler/document/outputs_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
