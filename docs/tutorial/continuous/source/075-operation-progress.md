# services/account/internal/handler/account/operation_progress.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/075-operation-progress.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Progress`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`bb36e1b371294e3122274d806947913d0e3aef4e909bc8014845584937a0dc68`。[参考实现](../../../../services/account/internal/handler/account/operation_progress.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
