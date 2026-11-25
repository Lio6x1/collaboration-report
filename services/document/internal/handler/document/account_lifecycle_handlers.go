// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"net/http"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

// 清理资格只提供给已验Node服务证书，不接受浏览器或Account调用者代选文档。
func (h *AccountRetirements) DocumentCleanupContext(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Cache-Control", "no-store")
	if h.retainedGovernanceCleanupContext(w, r, request) {
		return
	}
	if h.purgeCleanupContext(w, r, request) {
		return
	}
	result, err := h.retentions.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, result)
}

// Account只能请求固定计划中的单文档；Doc用自己的Node证书取得原清理回执。
func (h *AccountRetirements) ReleaseDraftResources(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
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
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.ReleaseDraftResources(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "DRAFT_RESOURCES_RELEASED"})
}

func (h *AccountRetirements) CleanHistory(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
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
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.CleanHistory(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "HISTORY_REMOVED"})
}

func (h *AccountRetirements) CleanPayloads(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
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
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.CleanPayloads(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "PAYLOADS_REMOVED"})
}

// account_retirement.go：保留原实现，按业务合并文件。

type AccountRetirements struct {
	usecase            *logic.AccountRetirements
	retentions         *logic.AccountRetentions
	purges             *logic.Purges
	retainedGovernance *logic.RetainedGovernance
}

func NewAccountRetirements(usecase *logic.AccountRetirements, retentions *logic.AccountRetentions) *AccountRetirements {
	return &AccountRetirements{usecase: usecase, retentions: retentions}
}

func (h *AccountRetirements) Plan(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if _, err := h.retentions.Plan(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "PLANNED"})
}

func (h *AccountRetirements) Relations(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.retentions.ApplyRelations(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "CLEANING"})
}

func (h *AccountRetirements) Complete(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.retentions.Complete(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "SUCCEEDED"})
}

func (h *AccountRetirements) Progress(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountClosurePageRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.retentions.Progress(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

func accountRetirementInput(w http.ResponseWriter, r *http.Request) (domain.AccountRetirementBinding, bool) {
	var binding domain.AccountRetirementBinding
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return binding, false
	}
	if !httpx.DecodeBody(w, r, &binding) {
		return binding, false
	}
	if !binding.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_RETIREMENT_BINDING_INVALID")
		return binding, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return binding, true
}

func (h *AccountRetirements) Prepare(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	result, err := h.usecase.Prepare(r.Context(), binding)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosurePreparation{State: result.State, RequiredKnowledgeBases: result.RequiredKnowledgeBases, ReservedKnowledgeBases: result.ReservedKnowledgeBases})
}

func (h *AccountRetirements) Cancel(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Cancel(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureDecision{Rejected: true})
}
