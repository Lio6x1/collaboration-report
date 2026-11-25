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
