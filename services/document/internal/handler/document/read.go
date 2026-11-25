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
