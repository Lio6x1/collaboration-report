# services/document/internal/handler/document/merge_review_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/254-merge-review-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Conflict`、`ConflictResolutions`、`NewConflictResolutions`、`Create`、`Progress`、`DraftControls`、`NewDraftControls`、`Preview`、`Discard`、`DiscardProgress`、`Merges`、`NewMerges`、`Approve`、`PullMain`、`NewPullMain`、`Source`、`Committed`、`ReverseChanges`、`NewReverseChanges`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
)

// conflict_contexts.go：保留原实现，按业务合并文件。

func (h *Reviews) Conflict(w http.ResponseWriter, r *http.Request) {
	fact, err := h.usecase.LookupConflict(r.Context(), pathvar.Vars(r)["conflictRef"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "REVIEW_CONTEXT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Conflict(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "CONFLICT_CONTEXT", run)
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

type ConflictResolutions struct {
	identity *Identity
	requests *Transactions
	reviews  *logic.Reviews
	usecase  *logic.Resolutions
	other    http.HandlerFunc
}

func NewConflictResolutions(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Resolutions, other http.HandlerFunc) *ConflictResolutions {
	return &ConflictResolutions{identity: identity, requests: requests, reviews: reviews, usecase: usecase, other: other}
}

func (h *ConflictResolutions) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ConflictResolution
	if !httpx.DecodeBodyLimit(w, r, &input, httpx.ContentBodyLimit) {
		return
	}
	intent, err := domain.ParseConflictResolution(pathvar.Vars(r)["conflictRef"], input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	origin, err := h.reviews.LookupConflict(r.Context(), intent.Conflict)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "CONFLICT_RESOLUTION", origin.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, origin, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, origin.Document, "CONFLICT_RESOLUTION_RESULT", run)
}

func (h *ConflictResolutions) Progress(w http.ResponseWriter, r *http.Request) {
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
	actor, err := h.identity.Actor(r, false, "CONFLICT_RESOLUTION_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "CONFLICT_RESOLUTION_RESULT", run)
}

// draft_controls.go：保留原实现，按业务合并文件。

type DraftControls struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.DraftControls
	other    http.HandlerFunc
}

func NewDraftControls(identity *Identity, requests *Transactions, usecase *logic.DraftControls, other http.HandlerFunc) *DraftControls {
	return &DraftControls{identity: identity, requests: requests, usecase: usecase, other: other}
}

func (h *DraftControls) Preview(w http.ResponseWriter, r *http.Request) {
	var input dto.DiscardPreviewCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseDiscardPreview(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DISCARD_PREVIEW", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.RegisterPreview(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = h.usecase.WaitPreview(r.Context(), intent.Operation); err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.ReadPreview(r.Context(), actor, document, unit, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "DISCARD_PREVIEW_RESULT", run)
}

func (h *DraftControls) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.LookupPreview(r.Context(), operation)
	if err == sql.ErrNoRows {
		h.DiscardProgress(w, r)
		return
	}
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DISCARD_PREVIEW_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "DISCARD_PREVIEW_RESULT", run)
}

func (h *DraftControls) Discard(w http.ResponseWriter, r *http.Request) {
	var input dto.DiscardDraft
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseDiscardDraft(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DISCARD_DRAFT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.RegisterDiscard(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DiscardProgress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, document, "DISCARD_DRAFT_RESULT", run)
}

func (h *DraftControls) DiscardProgress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.LookupDiscard(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DISCARD_DRAFT_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DiscardProgress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "DISCARD_DRAFT_RESULT", run)
}

// merges.go：保留原实现，按业务合并文件。

type Merges struct {
	identity        *Identity
	requests        *Transactions
	reviews         *logic.Reviews
	usecase         *logic.Merges
	otherOperations http.HandlerFunc
}

func NewMerges(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Merges, other http.HandlerFunc) *Merges {
	return &Merges{identity: identity, requests: requests, reviews: reviews, usecase: usecase, otherOperations: other}
}

func (h *Merges) Approve(w http.ResponseWriter, r *http.Request) {
	var input dto.ApproveMerge
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["requestId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, err := h.reviews.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "MERGE", pointer.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, pointer, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": input.OperationId, "state": "PREPARING", "poll_url": "/v1/merge-operations/" + input.OperationId}
	h.requests.PreparedMutation(w, r, actor, input.OperationId, "MERGE", input, 202, initial, run, present)
}

func (h *Merges) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	fact, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.otherOperations != nil && r.URL.Path == "/v1/operations/"+operation {
		h.otherOperations(w, r)
		return
	}
	if err == sql.ErrNoRows {
		httpx.Fail(w, 404, "MERGE_NOT_FOUND")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "MERGE_RESULT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "MERGE_RESULT", run)
}

// pull_main.go：保留原实现，按业务合并文件。

type PullMain struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.PullMain
	other    http.HandlerFunc
}

func NewPullMain(identity *Identity, requests *Transactions, usecase *logic.PullMain, other http.HandlerFunc) *PullMain {
	return &PullMain{identity: identity, requests: requests, usecase: usecase, other: other}
}
func (h *PullMain) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.PullMain
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PULL_MAIN", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, unit, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PreparedMutation(w, r, actor, input.OperationId, "PULL_MAIN", input, 202, map[string]any{"operation_id": input.OperationId, "state": "PREPARING", "poll_url": "/v1/operations/" + input.OperationId}, run, present)
}
func (h *PullMain) Progress(w http.ResponseWriter, r *http.Request) {
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
	actor, err := h.identity.Actor(r, false, "PULL_MAIN_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "PULL_MAIN_RESULT", run)
}
func (h *PullMain) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(raw, input) != nil {
		httpx.Fail(w, 400, "DRAFT_SOURCE_INVALID")
		return
	}
	value, err := h.usecase.DraftSource(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}
func (h *PullMain) Committed(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if protojson.Unmarshal(raw, query) != nil {
		httpx.Fail(w, 400, "DRAFT_COMMIT_QUERY_INVALID")
		return
	}
	value, err := h.usecase.CommittedSource(r.Context(), query)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// reverse_changes.go：保留原实现，按业务合并文件。

type ReverseChanges struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.ReverseChanges
	other    http.HandlerFunc
}

func NewReverseChanges(identity *Identity, requests *Transactions, usecase *logic.ReverseChanges, other http.HandlerFunc) *ReverseChanges {
	return &ReverseChanges{identity: identity, requests: requests, usecase: usecase, other: other}
}

func (h *ReverseChanges) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ReverseChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseReverseChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "REVERSE", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, document, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, document, "REVERSE_RESULT", run)
}

func (h *ReverseChanges) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	document, err := h.usecase.OperationDocument(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "REVERSE_RESULT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "REVERSE_RESULT", run)
}
````

UTF-8/LF全文SHA-256：`6ca5f8b3a2ad46af59fcc6ac5dbbe513630b833b5a0334fa982e814fda7e9a2e`。[参考实现](../../../../services/document/internal/handler/document/merge_review_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
