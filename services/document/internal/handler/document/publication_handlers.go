// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"database/sql"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// public_reading.go：保留原实现，按业务合并文件。

type PublicReading struct {
	identity *Identity
	requests *Transactions
	outputs  *Outputs
	usecase  *logic.PublicReading
}

func NewPublicReading(identity *Identity, requests *Transactions, outputs *Outputs, usecase *logic.PublicReading) *PublicReading {
	return &PublicReading{identity: identity, requests: requests, outputs: outputs, usecase: usecase}
}

func (h *PublicReading) Current(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_PUBLIC_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	// URL中的旧release提示不参与选择；用例只取当前正式指针。
	value, err := h.usecase.Current(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.SendBounded(r.Context(), http.StatusOK, value, 32<<20); err != nil {
		httpx.Failure(w, err)
	}
}

func (h *PublicReading) History(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RELEASE_HISTORY", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.History(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_RELEASE_HISTORY", run)
}

func (h *PublicReading) Historical(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	id, err := domain.CanonicalID(vars["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	release, err := domain.CanonicalID(vars["releaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RELEASE_HISTORY", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Historical(r.Context(), actor, id, release)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadBounded(w, r, actor, sources, id, "READ_RELEASE_HISTORY", run, 32<<20)
}

// publication.go：保留原实现，按业务合并文件。

type Publications struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.Publications
	otherOperations http.HandlerFunc
}

func NewPublications(identity *Identity, requests *Transactions, usecase *logic.Publications, other http.HandlerFunc) *Publications {
	return &Publications{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

func (h *Publications) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ReleaseCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseReleaseCreate(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

func (h *Publications) Change(w http.ResponseWriter, r *http.Request) {
	var input dto.Publication
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParsePublication(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

func (h *Publications) change(w http.ResponseWriter, r *http.Request, intent domain.PublicationIntent, input any) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PUBLICATION", document)
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

func (h *Publications) Progress(w http.ResponseWriter, r *http.Request) {
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
	actor, err := h.identity.Actor(r, false, "PUBLICATION_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "PUBLICATION_RESULT", run)
}

// visibility.go：保留原实现，按业务合并文件。

type Visibilities struct {
	identity       *Identity
	requests       *Transactions
	usecase        *logic.Visibilities
	authorizations *logic.Authorizations
}

func NewVisibilities(identity *Identity, requests *Transactions, usecase *logic.Visibilities, authorizations *logic.Authorizations) *Visibilities {
	return &Visibilities{identity: identity, requests: requests, usecase: usecase, authorizations: authorizations}
}
func (h *Visibilities) Change(w http.ResponseWriter, r *http.Request) {
	var input dto.VisibilityChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidVisibility(input.Policy) {
		httpx.Fail(w, 400, "VISIBILITY_POLICY_INVALID")
		return
	}
	isKnowledgeBase := pathvar.Vars(r)["knowledgeBaseId"] != ""
	key, kind := "documentId", "DOCUMENT_VISIBILITY"
	if isKnowledgeBase {
		key = "knowledgeBaseId"
		kind = "KB_VISIBILITY"
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)[key])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "CHANGE_VISIBILITY", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor)) {
		return
	}
	actor, run, err := h.usecase.Prepare(r.Context(), actor, id, isKnowledgeBase, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, 202, run, h.authorizations.VisibilityPresentation(actor, input.OperationId))
}
func (h *KnowledgeBases) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DELETE_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_DELETE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Delete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_DELETE", input, 202, run, nil)
}
