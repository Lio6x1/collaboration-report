# services/document/internal/handler/document/exports.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/253-exports.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Exports`、`NewExports`、`Create`、`Job`、`Download`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

type Exports struct {
	identity *Identity
	outputs  *Outputs
	usecase  *logic.Exports
	origin   string
}

func NewExports(identity *Identity, outputs *Outputs, usecase *logic.Exports, origin string) *Exports {
	return &Exports{identity: identity, outputs: outputs, usecase: usecase, origin: origin}
}

func (h *Exports) Create(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var body json.RawMessage
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	input, err := domain.ParseExportInput(dto.ExportCreate(body))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 匿名POST也来自同源产品入口；存在会话Cookie时不能回退匿名绕过CSRF。
	if r.Header.Get("Origin") != h.origin {
		httpx.Fail(w, 403, "ORIGIN_REQUIRED")
		return
	}
	actor, err := h.identity.OptionalMutationActor(r, "CREATE_EXPORT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	value, err := h.usecase.Create(r.Context(), actor, id, input, dto.ExportCreate(body), r.URL.Path, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.Send(r.Context(), http.StatusAccepted, value); err != nil {
		httpx.Failure(w, err)
	}
}

func (h *Exports) Job(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["exportJobId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_EXPORT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	value, err := h.usecase.Job(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.Send(r.Context(), http.StatusOK, value); err != nil {
		httpx.Failure(w, err)
	}
}

func (h *Exports) Download(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["exportJobId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "DOWNLOAD_EXPORT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	file, name, err := h.usecase.Download(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer file.Close()
	started, err := output.SendExport(r.Context(), file, name)
	if err != nil && !started {
		httpx.Failure(w, err)
	}
}
````

UTF-8/LF全文SHA-256：`4468f88eb337068b2f9dd2a8442f69d006920666b1a9db385bbca27c2ba7fa9f`。[参考实现](../../../../services/document/internal/handler/document/exports.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
