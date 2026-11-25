# services/document/internal/handler/document/structure_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/263-structure-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`MainControls`、`NewMainControls`、`Lock`、`Style`、`change`、`Progress`、`mainSourceInput`、`MainSources`、`NewMainSources`、`MainSource`、`CommittedSource`、`ConfirmationSource`、`Manifest`、`Structure`、`StructurePreviews`、`NewStructurePreviews`、`Create`、`Source`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
)

// main_controls.go：保留原实现，按业务合并文件。

type MainControls struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.MainControls
	otherOperations http.HandlerFunc
}

func NewMainControls(identity *Identity, requests *Transactions, usecase *logic.MainControls, other http.HandlerFunc) *MainControls {
	return &MainControls{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

func (h *MainControls) Lock(w http.ResponseWriter, r *http.Request) {
	var input dto.MainLock
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseMainLock(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

func (h *MainControls) Style(w http.ResponseWriter, r *http.Request) {
	var input dto.StyleChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStyleChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

func (h *MainControls) change(w http.ResponseWriter, r *http.Request, intent domain.MainControlIntent, input any) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "MAIN_CONTROL", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}
	h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)
}

func (h *MainControls) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.otherOperations != nil {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "MAIN_CONTROL_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "MAIN_CONTROL_RESULT", run)
}

// main_sources.go：保留原实现，按业务合并文件。

func mainSourceInput(w http.ResponseWriter, r *http.Request) json.RawMessage {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return nil
	}
	var raw json.RawMessage
	if !httpx.DecodeBody(w, r, &raw) {
		return nil
	}
	return raw
}

type MainSources struct {
	merges       *logic.Merges
	controls     *logic.MainControls
	readViews    *logic.MainReadViews
	publications *logic.Publications
}

func NewMainSources(merges *logic.Merges, controls *logic.MainControls, readViews *logic.MainReadViews, publications *logic.Publications) *MainSources {
	return &MainSources{merges: merges, controls: controls, readViews: readViews, publications: publications}
}

func (h *MainSources) MainSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.CaptureMainRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "MAIN_SOURCE_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	switch input.Context.GetTarget().GetPurpose() {
	case "MAIN_CONTROL":
		value, err = h.controls.MainSource(r.Context(), input)
	case "READ_MAIN":
		value, err = h.readViews.MainSource(r.Context(), input)
	case "PUBLICATION":
		value, err = h.publications.MainSource(r.Context(), input)
	case "MERGE":
		value, err = h.merges.MainSource(r.Context(), input)
	default:
		httpx.Fail(w, 400, "MAIN_SOURCE_QUERY_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

func (h *MainSources) CommittedSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if err := protojson.Unmarshal(raw, query); err != nil {
		httpx.Fail(w, 400, "MAIN_COMMIT_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	if query.GetTarget().GetPurpose() == "MAIN_CONTROL" {
		value, err = h.controls.CommittedSource(r.Context(), query)
	} else {
		value, err = h.merges.CommittedSource(r.Context(), query)
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

func (h *MainSources) ConfirmationSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.PrepareMainCommitRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "STRUCTURE_CONFIRMATION_QUERY_INVALID")
		return
	}
	value, err := h.controls.ConfirmationSource(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// manifest_read.go：保留原实现，按业务合并文件。

func (h *Readings) Manifest(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	document, err := domain.CanonicalID(vars["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	manifest, err := domain.CanonicalID(vars["manifestId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MANIFEST", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Manifest(r.Context(), actor, document, manifest)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "READ_MANIFEST", run)
}

// structure.go：保留原实现，按业务合并文件。

func (h *MainControls) Structure(w http.ResponseWriter, r *http.Request) {
	var input dto.StructureChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStructureChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

// structure_previews.go：保留原实现，按业务合并文件。

type StructurePreviews struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.StructurePreviews
	other    http.HandlerFunc
}

func NewStructurePreviews(identity *Identity, requests *Transactions, usecase *logic.StructurePreviews, other http.HandlerFunc) *StructurePreviews {
	return &StructurePreviews{identity: identity, requests: requests, usecase: usecase, other: other}
}

func (h *StructurePreviews) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "STRUCTURE_PREVIEW_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "STRUCTURE_PREVIEW_RESULT", run)
}

func (h *StructurePreviews) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.StructurePreviewCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStructurePreview(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "STRUCTURE_PREVIEW", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, document, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = h.usecase.Wait(r.Context(), intent.Operation); err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Read(r.Context(), actor, document, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 写意图先持久化，RPC/等待均在SQL锁外；最终200由当前授权和真实输出租约保护。
	h.requests.Read(w, r, actor, sources, document, "STRUCTURE_PREVIEW_RESULT", run)
}

func (h *StructurePreviews) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "STRUCTURE_PREVIEW_QUERY_INVALID")
		return
	}
	value, err := h.usecase.Source(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}
````

UTF-8/LF全文SHA-256：`08c3430da9f60ef43156c3e1da8c912d5ce0827120c49049b8d48130014bf3c0`。[参考实现](../../../../services/document/internal/handler/document/structure_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
