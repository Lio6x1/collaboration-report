// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"database/sql"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// review_readings.go：保留原实现，按业务合并文件。

func (h *Reviews) Request(w http.ResponseWriter, r *http.Request) {
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.Request(r.Context(), actor, pointer)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW", run)
}

func (h *Reviews) Round(w http.ResponseWriter, r *http.Request) {
	round, err := domain.CanonicalID(pathvar.Vars(r)["roundId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.Round(r.Context(), actor, pointer, round)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW_ROUND", run)
}

func (h *Reviews) Rounds(w http.ResponseWriter, r *http.Request)  { h.history(w, r, "rounds") }
func (h *Reviews) Changes(w http.ResponseWriter, r *http.Request) { h.history(w, r, "changes") }
func (h *Reviews) Actions(w http.ResponseWriter, r *http.Request) { h.history(w, r, "actions") }

func (h *Reviews) history(w http.ResponseWriter, r *http.Request, kind string) {
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.History(r.Context(), actor, pointer, kind, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW_HISTORY", run)
}

func (h *Reviews) List(w http.ResponseWriter, r *http.Request) {
	document := uint64(0)
	if text := pathvar.Vars(r)["documentId"]; text != "" {
		id, err := domain.CanonicalID(text)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		document = id
	}
	actor, err := h.identity.Actor(r, false, "LIST_REVIEWS", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.List(r.Context(), actor, document, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "LIST_REVIEWS", run)
}

// reviews.go：保留原实现，按业务合并文件。

type Reviews struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Reviews
}

func NewReviews(identity *Identity, requests *Transactions, usecase *logic.Reviews) *Reviews {
	return &Reviews{identity: identity, requests: requests, usecase: usecase}
}

func (h *Reviews) target(w http.ResponseWriter, r *http.Request, mutation bool) (domain.ReviewPointer, logic.Actor, bool) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["requestId"])
	if err != nil {
		httpx.Failure(w, err)
		return domain.ReviewPointer{}, logic.Actor{}, false
	}
	pointer, err := h.usecase.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return pointer, logic.Actor{}, false
	}
	actor, err := h.identity.Actor(r, mutation, "REVIEW_CONTEXT", pointer.Document)
	if err != nil {
		httpx.Failure(w, err)
		return pointer, actor, false
	}
	return pointer, actor, true
}

func (h *Reviews) RequestChanges(w http.ResponseWriter, r *http.Request) {
	var input dto.RequestChanges
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.RequestChanges(r.Context(), actor, pointer, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "REQUEST_CHANGES", input, 200, run, present)
}

func (h *Reviews) Withdraw(w http.ResponseWriter, r *http.Request) {
	var input dto.Withdrawal
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.Withdraw(r.Context(), actor, pointer, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "REVIEW_WITHDRAW", input, 200, run, present)
}

func (h *Reviews) Resolve(w http.ResponseWriter, r *http.Request) {
	var input dto.ResolveChanges
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	change, err := domain.CanonicalID(pathvar.Vars(r)["changeId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.Resolve(r.Context(), actor, pointer, change, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOLVE_CHANGES", input, 200, run, present)
}

// submissions.go：保留原实现，按业务合并文件。

type Submissions struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.Submissions
	otherOperations http.HandlerFunc
}

func NewSubmissions(identity *Identity, requests *Transactions, usecase *logic.Submissions, otherOperations http.HandlerFunc) *Submissions {
	return &Submissions{identity: identity, requests: requests, usecase: usecase, otherOperations: otherOperations}
}

func (h *Submissions) Create(w http.ResponseWriter, r *http.Request) {
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
	// 生成 oneOf DTO 是命名 RawMessage；先保留 JSON 字节，再交领域规则解析，避免 []byte 被解成 base64。
	var raw json.RawMessage
	if !httpx.DecodeBody(w, r, &raw) {
		return
	}
	intent, err := domain.ParseSubmission(dto.Submission(raw))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "SUBMISSION", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}
	h.requests.PreparedMutation(w, r, actor, intent.Operation, "SUBMISSION", json.RawMessage(intent.Canonical), 202, initial, run, present)
}

func (h *Submissions) Progress(w http.ResponseWriter, r *http.Request) {
	id := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(id) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	fact, err := h.usecase.Lookup(r.Context(), id)
	if err == sql.ErrNoRows && h.otherOperations != nil && r.URL.Path == "/v1/operations/"+id {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		if err == sql.ErrNoRows {
			httpx.Fail(w, 404, "SUBMISSION_NOT_FOUND")
		} else {
			httpx.Failure(w, err)
		}
		return
	}
	actor, err := h.identity.Actor(r, false, "SUBMISSION_RESULT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "SUBMISSION_RESULT", run)
}
