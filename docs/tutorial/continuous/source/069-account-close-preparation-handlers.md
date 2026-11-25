# services/account/internal/handler/account/account_close_preparation_handlers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/069-account-close-preparation-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`AccountClosures`、`NewAccountClosures`、`Start`、`Progress`、`Binding`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`2556d68333766253e5b16fb39cca1f37c859ce9875f595a9b6fcac3dcb6c8882`。[参考实现](../../../../services/account/internal/handler/account/account_close_preparation_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
