// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"net/http"
)

// departure.go：保留原实现，按业务合并文件。

type Departures struct{ usecase *logic.Departures }

func NewDepartures(usecase *logic.Departures) *Departures { return &Departures{usecase: usecase} }
func departureInput(w http.ResponseWriter, r *http.Request) (domain.DepartureBinding, bool) {
	var binding domain.DepartureBinding
	var err error
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return binding, false
	}
	binding.Org, err = domain.CanonicalID(r.URL.Query().Get("organization_id"))
	if err != nil {
		httpx.Failure(w, err)
		return binding, false
	}
	binding.User, err = domain.CanonicalID(r.URL.Query().Get("user_id"))
	if err != nil {
		httpx.Failure(w, err)
		return binding, false
	}
	binding.Epoch, err = domain.CanonicalID(r.URL.Query().Get("epoch"))
	binding.ID = r.URL.Query().Get("operation_id")
	if err != nil || !domain.ValidOperationID(binding.ID) {
		httpx.Fail(w, 400, "DEPARTURE_BINDING_INVALID")
		return binding, false
	}
	return binding, true
}
func (h *Departures) Eligibility(w http.ResponseWriter, r *http.Request) {
	binding, ok := departureInput(w, r)
	if !ok {
		return
	}
	admin, err := h.usecase.Eligibility(r.Context(), binding)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]bool{"requires_handoff": admin})
}
func (h *Departures) Close(w http.ResponseWriter, r *http.Request) {
	binding, ok := departureInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Close(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "CLOSED"})
}

// organization_close.go：保留原实现，按业务合并文件。

type OrganizationClosures struct{ usecase *logic.OrganizationClosures }

func NewOrganizationClosures(usecase *logic.OrganizationClosures) *OrganizationClosures {
	return &OrganizationClosures{usecase: usecase}
}
func organizationCloseInput(w http.ResponseWriter, r *http.Request) (platform.OrganizationClosureBinding, bool) {
	var b platform.OrganizationClosureBinding
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return b, false
	}
	if !httpx.DecodeBody(w, r, &b) {
		return b, false
	}
	if !b.Valid() {
		httpx.Fail(w, 400, "ORGANIZATION_CLOSE_BINDING_INVALID")
		return b, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return b, true
}
func (h *OrganizationClosures) Plan(w http.ResponseWriter, r *http.Request) {
	b, ok := organizationCloseInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Plan(r.Context(), b); err != nil {
		httpx.Failure(w, err)
		return
	}
	// 已有原号返回真实持久阶段，不把成功HTTP解释成新的规划或清理ACK。
	state, err := h.usecase.State(r.Context(), b)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: state})
}
func (h *OrganizationClosures) Progress(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.OrganizationClosurePageRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.Progress(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}
func (h *OrganizationClosures) Complete(w http.ResponseWriter, r *http.Request) {
	b, ok := organizationCloseInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Complete(r.Context(), b); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: "SUCCEEDED"})
}
func (h *OrganizationClosures) CleanPayloads(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.CleanPayloads(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "PAYLOADS_REMOVED"})
}

// 根在现有Node context入口按lifecycle kind分派此用例，不改变Node原协议。
func (h *OrganizationClosures) DocumentCleanupContext(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

// organization_close_dispatch.go：保留原实现，按业务合并文件。

// 路由仅包裹旧context入口：原Node协议完全不变，组织原号由自己的计划发证。
// 其他清理生命周期继续交给现有完整分派器；未知查询失败绝不回退账号证明。
func (h *OrganizationClosures) DispatchCleanupContext(fallback http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.HTTPCaller(r, "collaboration") {
			httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
			return
		}
		var request platform.AccountDocumentCleanupRequest
		if !httpx.DecodeBody(w, r, &request) {
			return
		}
		if !request.Valid() {
			httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
			return
		}
		handles, err := h.usecase.HandlesCleanup(r.Context(), request.Operation)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		if handles {
			result, err := h.usecase.DocumentCleanupContext(r.Context(), request)
			if err != nil {
				httpx.Failure(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			platform.WriteJSON(w, 200, result)
			return
		}
		raw, err := json.Marshal(request)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		fallback(w, r)
	}
}

// personal_transfer.go：保留原实现，按业务合并文件。

type PersonalTransfers struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.PersonalTransfers
}

func NewPersonalTransfers(identity *Identity, requests *Transactions, usecase *logic.PersonalTransfers) *PersonalTransfers {
	return &PersonalTransfers{identity: identity, requests: requests, usecase: usecase}
}

func (h *PersonalTransfers) Offer(w http.ResponseWriter, r *http.Request) {
	var input dto.OwnerChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PERSONAL_TRANSFER_OFFER", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_OFFER", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Offer(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_OFFER", input, 202, run, nil)
}

func (h *PersonalTransfers) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "PERSONAL_TRANSFER_DETAIL", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Detail(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "PERSONAL_TRANSFER_DETAIL", run)
}

func (h *PersonalTransfers) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "PERSONAL_TRANSFER_LIST", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	q := r.URL.Query()
	sources, run, err := h.usecase.List(r.Context(), actor, q.Get("direction"), q.Get("limit"), q.Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "PERSONAL_TRANSFER_LIST", run)
}

func (h *PersonalTransfers) Accept(w http.ResponseWriter, r *http.Request) {
	var input dto.PersonalTransferAccept
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PERSONAL_TRANSFER_ACCEPT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 未知接受只能读取标准 fingerprint 核验后的原 receipt，不再次预留容量。
	if h.requests.Replay(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_ACCEPT", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Accept(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_ACCEPT", input, 202, run, present)
}

func (h *PersonalTransfers) Cancel(w http.ResponseWriter, r *http.Request) { h.decide(w, r, "CANCEL") }
func (h *PersonalTransfers) Reject(w http.ResponseWriter, r *http.Request) { h.decide(w, r, "REJECT") }

func (h *PersonalTransfers) decide(w http.ResponseWriter, r *http.Request, action string) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	kind := "PERSONAL_TRANSFER_" + action
	actor, err := h.identity.Actor(r, true, kind, 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Decide(r.Context(), actor, id, input, action)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, 202, run, nil)
}

// retained_cleanup_context.go：保留原实现，按业务合并文件。

func (h *AccountRetirements) SetRetainedGovernance(usecase *logic.RetainedGovernance) {
	h.retainedGovernance = usecase
}

func (h *AccountRetirements) retainedGovernanceCleanupContext(w http.ResponseWriter, r *http.Request, request platform.AccountDocumentCleanupRequest) bool {
	if h.retainedGovernance == nil {
		return false
	}
	_, err := h.retainedGovernance.Lookup(r.Context(), request.Operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	result, err := h.retainedGovernance.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	// 复用真实PURGE清理墓碑，回执仍沿冻结context；内部kind不进入CLEANED摘要。
	platform.WriteJSON(w, http.StatusOK, struct {
		platform.AccountDocumentCleanupContext
		CleanupKind string `json:"cleanup_kind"`
	}{AccountDocumentCleanupContext: result, CleanupKind: "PURGE"})
	return true
}

// retained_governance.go：保留原实现，按业务合并文件。

type RetainedGovernance struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.RetainedGovernance
	otherOperations http.HandlerFunc
}

func NewRetainedGovernance(identity *Identity, requests *Transactions, usecase *logic.RetainedGovernance, other http.HandlerFunc) *RetainedGovernance {
	return &RetainedGovernance{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

func (h *RetainedGovernance) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "UNPUBLISH")
}

func (h *RetainedGovernance) Delete(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "PURGE")
}

func (h *RetainedGovernance) Metadata(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, domain.RetainedGovernancePurpose, id)
	if err == nil {
		actor, err = h.usecase.Authorize(r.Context(), actor, platform.NewTraceID(), id)
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Metadata(id))
}

func (h *RetainedGovernance) mutate(w http.ResponseWriter, r *http.Request, action string) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidOperationID(input.OperationId) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	if _, err = domain.CanonicalRevision(input.ExpectedRevision); err != nil {
		httpx.Fail(w, 400, "CONTROL_REVISION_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, domain.RetainedGovernancePurpose, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Authorize(r.Context(), actor, input.OperationId, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	kind := domain.RetainedGovernanceKind(action)
	// 原号在最终删除后仍可查询精确最小receipt，但每次HTTP都核验当前
	// 平台角色；普通活跃账号证明、旧组织管理员或注销恢复票都不能代替。
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, http.StatusAccepted, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, id, action, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, http.StatusAccepted, run, present)
}

func (h *RetainedGovernance) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	row, err := h.usecase.Lookup(r.Context(), operation)
	if errors.Is(err, sql.ErrNoRows) && h.otherOperations != nil {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Fail(w, 404, "OPERATION_NOT_FOUND")
		} else {
			httpx.Failure(w, err)
		}
		return
	}
	actor, err := h.identity.Actor(r, false, domain.RetainedGovernancePurpose, row.Plan.Purge.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Authorize(r.Context(), actor, operation, row.Plan.Purge.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 查询只输出原操作者的最小进度，不为已关闭Doc创建正文read admission。
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Progress(actor, operation, row.Plan.Purge.Document))
}
