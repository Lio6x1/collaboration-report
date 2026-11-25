# services/document/internal/handler/document/read.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/258-read.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Readings`、`NewReadings`、`Document`、`Main`、`Revision`、`Unit`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

type Readings struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Readings
}

func NewReadings(identity *Identity, requests *Transactions, usecase *logic.Readings) *Readings {
	return &Readings{identity: identity, requests: requests, usecase: usecase}
}
func (h *Readings) Document(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Document(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadProjected(w, r, actor, sources, id, "READ_DOCUMENT", run, func(value any) any {
		reading := value.(logic.DocumentReading)
		actions, _ := json.Marshal(reading.Actions)
		w.Header().Set("X-Available-Actions", string(actions))
		return reading.Document
	})
}
func (h *Readings) Main(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MAIN", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Main(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_MAIN", run)
}

func (h *Readings) Revision(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MAIN", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Revision(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_MAIN", run)
}
func (h *Readings) Unit(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	view := pathvar.Vars(r)["viewId"]
	if !domain.ValidOperationID(view) {
		httpx.Fail(w, 400, "READ_VIEW_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_FIXED_UNIT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Unit(r.Context(), actor, id, unit, view)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_FIXED_UNIT", run)
}
````

UTF-8/LF全文SHA-256：`73aa9f67c7c28ff914809b2458792c7b66f9d9b42e1bc0a3185dced3da0e33b1`。[参考实现](../../../../services/document/internal/handler/document/read.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
