// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"net/http"
	"strconv"
)

// pagination.go：保留原实现，按业务合并文件。

type Pagination struct{ cursors *logic.Cursors }

func NewPagination(cursors *logic.Cursors) *Pagination { return &Pagination{cursors: cursors} }

func (h *Pagination) Page(w http.ResponseWriter, r *http.Request, binding string) (int, string, bool) {
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			httpx.Failure(w, 400, "PAGE_LIMIT_INVALID")
			return 0, "", false
		}
		limit = parsed
	}
	last, valid := h.cursors.Decode(r.URL.Query().Get("cursor"), binding)
	if !valid {
		httpx.Failure(w, 400, "CURSOR_INVALID")
		return 0, "", false
	}
	return limit, last, true
}

func (h *Pagination) Next(binding, last string) *string { return h.cursors.Encode(binding, last) }

// verify_session.go：保留原实现，按业务合并文件。

func (h *Authentication) VerifySession(w http.ResponseWriter, r *http.Request) {
	// 文档服务不能读账号库；证书保护端点只核验真实会话/CSRF，组织资格仍由固定RPC核验。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	session, ok := h.Authenticate(w, r, r.Header.Get("X-Internal-Mutation") == "true")
	if !ok {
		return
	}
	// grant须早于会话绝对到期关闭；短期身份准入不能冒充会话寿命。
	platform.WriteJSON(w, 200, map[string]any{"actor_id": session.UserID, "session_expires_unix_ms": session.Expires.UnixMilli()})
}
