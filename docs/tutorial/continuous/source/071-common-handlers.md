# services/account/internal/handler/account/common_handlers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/071-common-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Pagination`、`NewPagination`、`Page`、`Next`、`VerifySession`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`a868d78e369a4b4dbd3e503e2a3c32a04ebb90ceedfb90bc65a73ceee7cfbf02`。[参考实现](../../../../services/account/internal/handler/account/common_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
