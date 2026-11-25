// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/base64"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"strings"
)

// applications.go：保留原实现，按业务合并文件。

func (h *Sharing) Apply(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentApplicationCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	hash, err := domain.TokenHash(input.InvitationToken)
	if err != nil || len([]rune(input.Reason)) > 2000 {
		httpx.Fail(w, 400, "APPLICATION_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "APPLY_DOCUMENT_INVITATION", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_LINK_APPLY", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Apply(r.Context(), actor, hash, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_LINK_APPLY", input, 202, run, nil)
}
func (h *Sharing) Decide(w http.ResponseWriter, r *http.Request) {
	var input dto.ApplicationReview
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["applicationId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if (input.Decision != "ACCEPT" && input.Decision != "REJECT") || len([]rune(input.Reason)) > 2000 {
		httpx.Fail(w, 400, "APPLICATION_DECISION_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "DECIDE_DOCUMENT_APPLICATION", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_APPLICATION_DECIDE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Decide(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_APPLICATION_DECIDE", input, 202, run, nil)
}
func (h *Sharing) Applications(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "LIST_DOCUMENT_APPLICATIONS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document := uint64(0)
	if value := pathvar.Vars(r)["documentId"]; value != "" {
		document, err = domain.CanonicalID(value)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
	}
	sources, run, err := h.usecase.Applications(r.Context(), actor, document, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "LIST_DOCUMENT_APPLICATIONS", run)
}
func (h *Sharing) OperationProgress(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "OPERATION_PROGRESS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	id := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(id) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, "OPERATION_PROGRESS", h.usecase.OperationProgress(actor, id))
}

// authorization.go：保留原实现，按业务合并文件。

type Authorizations struct {
	identity       *Identity
	requests       *Transactions
	changes        *logic.AuthorizationChanges
	authorizations *logic.Authorizations
}

func NewAuthorizations(identity *Identity, requests *Transactions, changes *logic.AuthorizationChanges, authorizations *logic.Authorizations) *Authorizations {
	return &Authorizations{identity: identity, requests: requests, changes: changes, authorizations: authorizations}
}
func (h *Authorizations) Change(w http.ResponseWriter, r *http.Request) {
	var input dto.AuthorizationChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if len(input.Changes) == 0 || len(input.Changes) > 100 {
		httpx.Fail(w, 400, "AUTHORIZATION_CHANGES_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CHANGE_AUTHORIZATION", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_AUTHORIZATION", input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor)) {
		return
	}
	actor, run, err := h.changes.Prepare(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_AUTHORIZATION", input, 202, run, h.authorizations.Presentation(actor))
}
func (h *Authorizations) Progress(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "OPERATION_PROGRESS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, "OPERATION_PROGRESS", h.authorizations.Progress(actor, operation))
}

// identity.go：保留原实现，按业务合并文件。

type Identity struct{ usecase *logic.Identity }

func NewIdentity(usecase *logic.Identity) *Identity { return &Identity{usecase: usecase} }

func (h *Identity) OptionalActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error) {
	return h.optionalActor(r, false, purpose, document)
}

// 带Cookie的PUBLIC导出仍是用户变更，必须验证Origin/CSRF；无Cookie才走匿名准入。
func (h *Identity) OptionalMutationActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error) {
	return h.optionalActor(r, true, purpose, document)
}

func (h *Identity) optionalActor(r *http.Request, mutation bool, purpose string, document uint64) (*logic.Actor, error) {
	count := 0
	for _, header := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(header, ";") {
			name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if strings.TrimSpace(name) == "__Host-collabSession" {
				count++
			}
		}
	}
	if count == 0 {
		return nil, nil
	}
	// net/http会忽略畸形Cookie。原始头中有会话名时必须走真实身份核验，
	// 重复、空值和不可解析值均不能退回匿名资格。
	if count != 1 {
		return nil, domain.Violation(domain.SessionRequired, "SESSION_INVALID")
	}
	actor, err := h.Actor(r, mutation, purpose, document)
	if err != nil {
		return nil, err
	}
	return &actor, nil
}
func (h *Identity) Actor(r *http.Request, mutation bool, purpose string, document uint64) (logic.Actor, error) {
	cookie, err := r.Cookie("__Host-collabSession")
	if err != nil {
		return logic.Actor{}, domain.Violation(domain.SessionRequired, "SESSION_INVALID")
	}
	token, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(token) != 32 || base64.RawURLEncoding.EncodeToString(token) != cookie.Value {
		return logic.Actor{}, domain.Violation(domain.SessionRequired, "SESSION_INVALID")
	}
	return h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get("Cookie"), Origin: r.Header.Get("Origin"), CSRF: r.Header.Get("X-CSRF-Token"), Mutation: mutation}, purpose, document)
}

// invitations.go：保留原实现，按业务合并文件。

type Sharing struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Sharing
}

func NewSharing(identity *Identity, requests *Transactions, usecase *logic.Sharing) *Sharing {
	return &Sharing{identity: identity, requests: requests, usecase: usecase}
}
func (h *Sharing) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentInvitationCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	hours := int64(48)
	if input.ExpiresInHours != nil {
		hours = *input.ExpiresInHours
	}
	if hours < 1 || hours > 720 {
		httpx.Fail(w, 400, "INVITATION_EXPIRY_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CREATE_DOCUMENT_INVITATION", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 链接是敏感结果，原号重试仍须经过当前管理资格与输出核验。
	actor, run, present, err := h.usecase.CreateInvitation(r.Context(), actor, id, hours, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_INVITATION_CREATE", input, 200, run, present)
}
func (h *Sharing) CancelInvitation(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["invitationId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "CANCEL_DOCUMENT_INVITATION", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_INVITATION_CANCEL", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.CancelInvitation(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_INVITATION_CANCEL", input, 200, run, nil)
}
func (h *Sharing) Invitations(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "LIST_DOCUMENT_INVITATIONS", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Invitations(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "LIST_DOCUMENT_INVITATIONS", run)
}

func (h *Sharing) ArchiveInvitation(w http.ResponseWriter, r *http.Request) {
	h.archive(w, r, "INVITATION", "invitationId")
}
func (h *Sharing) ArchiveApplication(w http.ResponseWriter, r *http.Request) {
	h.archive(w, r, "APPLICATION", "applicationId")
}
func (h *Sharing) archive(w http.ResponseWriter, r *http.Request, kind, parameter string) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)[parameter])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	operation := "DOCUMENT_" + kind + "_ARCHIVE"
	actor, err := h.identity.Actor(r, true, operation, 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, operation, input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Archive(r.Context(), actor, id, kind, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, operation, input, 200, run, nil)
}
