# services/account/internal/handler/account/logout.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/073-logout.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Logout`、`NewLogout`、`ProgressDTO`、`logoutCookies`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`f451678f51d0c750eff7f495afff15f348422b79863190ee06548e547f81cb8a`。[参考实现](../../../../services/account/internal/handler/account/logout.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
