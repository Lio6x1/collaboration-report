// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

type AccountClosures struct {
	usecase *logic.AccountClosures
	secrets *domain.Secrets
	origin  string
}

func NewAccountClosures(usecase *logic.AccountClosures, secrets *domain.Secrets, origin string) *AccountClosures {
	return &AccountClosures{usecase: usecase, secrets: secrets, origin: origin}
}

func (h *AccountClosures) Start(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.AccountClose
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	selection, err := h.usecase.Resolve(r.Context(), input.OperationId, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !httpx.ValidCSRF(r, selection.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return
	}
	plan, err := h.usecase.Start(r.Context(), selection.Session, input, h.secrets.Fingerprint(input))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	// 准备期保留主会话以便交接；原秘密另存恢复Cookie，身份冻结后只读本操作。
	httpx.SetCookie(w, httpx.RecoveryCookie, selection.Token, time.Now().UTC().Add(24*time.Hour), true)
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 202, plan)
}

func (h *AccountClosures) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	plan, err := h.usecase.Progress(r.Context(), operation, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, plan)
}

// account_close_binding.go：保留原实现，按业务合并文件。

// 原关闭组织集合只向Doc服务控制通道交付；浏览器/其他服务不能按operation号查范围。
func (h *AccountCloseDecisions) Binding(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var query dto.Action
	if !httpx.ReadJSON(w, r, &query) {
		return
	}
	binding, err := h.usecase.BindingForDocument(r.Context(), query.OperationId)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, binding)
}
