package account

import (
	"net/http"
	"time"

	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
)

type Logout struct {
	logic  *logic.Logout
	origin string
}

func NewLogout(service *logic.Logout, origin string) *Logout {
	return &Logout{logic: service, origin: origin}
}

func ProgressDTO(id, state string) dto.Operation {
	poll := "/v1/account/operations/" + id
	return dto.Operation{OperationId: id, State: state, PollUrl: &poll}
}

func (h *Logout) Logout(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Action
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	selection, err := h.logic.Resolve(r.Context(), input.OperationId, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !httpx.ValidCSRF(r, selection.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return
	}
	state := selection.ReceiptState
	if !selection.Registered {
		state, err = h.logic.Prepare(r.Context(), input.OperationId, selection.Session)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	logoutCookies(w, r, selection.Token)
	platform.WriteJSON(w, 202, ProgressDTO(input.OperationId, state))
}

func logoutCookies(w http.ResponseWriter, r *http.Request, originalToken string) {
	// 清原登录Cookie，另一个新会话不能被旧重试清掉；恢复Cookie只证明原操作。
	if httpx.CookieValue(r, httpx.SessionCookie) == originalToken {
		httpx.ClearCookie(w, httpx.SessionCookie)
	}
	httpx.SetCookie(w, httpx.RecoveryCookie, originalToken, time.Now().UTC().Add(24*time.Hour), true)
}
