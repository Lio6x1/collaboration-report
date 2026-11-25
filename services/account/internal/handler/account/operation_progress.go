package account

import (
	"encoding/json"
	"net/http"

	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/rest/pathvar"
)

func (h *Logout) Progress(w http.ResponseWriter, r *http.Request) {
	id := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(id) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	result, err := h.logic.Progress(r.Context(), id, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if result.State != "" {
		platform.WriteJSON(w, 200, ProgressDTO(id, result.State))
		return
	}
	platform.WriteJSON(w, 200, json.RawMessage(result.Result))
}
