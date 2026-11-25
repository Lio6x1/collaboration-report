// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"encoding/json"
	"net/http"
	"strings"
)

// account_close_decision.go：保留原实现，按业务合并文件。

type AccountCloseDecisions struct{ usecase *logic.AccountClosures }

func NewAccountCloseDecisions(usecase *logic.AccountClosures) *AccountCloseDecisions {
	return &AccountCloseDecisions{usecase: usecase}
}

func (h *AccountCloseDecisions) Read(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.AccountClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	if !platform.ValidateAccountClosureBinding(binding) {
		httpx.Failure(w, 400, "ACCOUNT_CLOSURE_BINDING_INVALID")
		return
	}
	rejected, err := h.usecase.Rejected(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountClosureDecision{Rejected: rejected})
}

func (h *AccountCloseDecisions) Confirm(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.AccountClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	if !platform.ValidateAccountClosureBinding(binding) {
		httpx.Failure(w, 400, "ACCOUNT_CLOSURE_BINDING_INVALID")
		return
	}
	confirmed, err := h.usecase.Confirmed(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountClosureConfirmation{Confirmed: confirmed})
}

// account_close_output.go：保留原实现，按业务合并文件。

// 专用恢复输出不包含姓名、正文或目录；冻结原会话仍须能读取这一条最小计划。
// 不能扩大全局操作回执豁免：精确路由、原号、四字段和规范字节同时成立才放行。
func MinimalClosureReceipt(path string, raw []byte) bool {
	if len(raw) > 2<<20 {
		return false
	}
	var plan dto.AccountClosePlan
	if json.Unmarshal(raw, &plan) != nil || !domain.ValidOperationID(plan.OperationId) || plan.RequiredSuccessions == nil || plan.RetainedReleaseIds == nil {
		return false
	}
	if path != "/v1/account/closures" && path != "/v1/account/closures/"+plan.OperationId {
		return false
	}
	switch plan.State {
	case "PREPARING", "FROZEN", "PLANNED", "CLEANING", "SUCCEEDED", "REJECTED":
	default:
		return false
	}
	for _, item := range plan.RequiredSuccessions {
		if !domain.ValidEntityID(item.TargetId) || (item.Kind != "ORGANIZATION_OWNER" && item.Kind != "ORGANIZATION_ADMIN" && item.Kind != "KNOWLEDGE_BASE_ADMIN") {
			return false
		}
	}
	for _, release := range plan.RetainedReleaseIds {
		if !domain.ValidEntityID(release) {
			return false
		}
	}
	canonical, err := json.Marshal(plan)
	// Encoder允许一次LF；额外字段、重复键或非规范编码不能夹带未解析前缀。
	return err == nil && bytes.Equal(canonical, []byte(strings.TrimSuffix(string(raw), "\n")))
}
