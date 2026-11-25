# services/document/internal/handler/document/workspace_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/265-workspace-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Chapter`、`ContentVersions`、`NewContentVersions`、`Read`、`Directory`、`NewDirectory`、`KnowledgeBase`、`KnowledgeBases`、`Documents`、`KnowledgeBaseMembers`、`DocumentCollaborators`、`DocumentMentionCandidates`、`NewDocuments`、`Create`、`EditableUnits`、`NewEditableUnits`、`List`、`NewKnowledgeBases`、`Rename`、`CreationGrant`、`Role`、`Management`、`NewManagement`、`Transfer`、`Delete`、`Spaces`、`NewSpaces`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// chapters.go：保留原实现，按业务合并文件。

func (h *MainControls) Chapter(w http.ResponseWriter, r *http.Request) {
	var input dto.ChapterCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseChapterCreate(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

// content_versions.go：保留原实现，按业务合并文件。

type ContentVersions struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.ContentVersions
}

func NewContentVersions(identity *Identity, requests *Transactions, usecase *logic.ContentVersions) *ContentVersions {
	return &ContentVersions{identity: identity, requests: requests, usecase: usecase}
}

func (h *ContentVersions) Read(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["contentVersionId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	scope, err := h.usecase.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_CONTENT_VERSION", scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Read(r.Context(), actor, scope)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 沿统一HTTP输出登记与关闭租约交付，版本ID不能成为撤权后的下载后门。
	h.requests.Read(w, r, actor, sources, scope.Document, "READ_CONTENT_VERSION", run)
}

// directory.go：保留原实现，按业务合并文件。

type Directory struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Directory
}

func NewDirectory(identity *Identity, requests *Transactions, usecase *logic.Directory) *Directory {
	return &Directory{identity: identity, requests: requests, usecase: usecase}
}
func (h *Directory) KnowledgeBase(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBase(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadProjected(w, r, actor, sources, 0, "READ_KB", run, func(value any) any {
		reading := value.(logic.KnowledgeBaseReading)
		actions, _ := json.Marshal(reading.Actions)
		w.Header().Set("X-Available-Actions", string(actions))
		return reading.KnowledgeBase
	})
}
func (h *Directory) KnowledgeBases(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "LIST_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBases(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("q"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "LIST_KB", run)
}
func (h *Directory) Documents(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "LIST_DOCUMENTS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var space, kb uint64
	if pathvar.Vars(r)["knowledgeBaseId"] != "" {
		kb, err = domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	} else {
		space, err = domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Documents(r.Context(), actor, space, kb, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "LIST_DOCUMENTS", run)
}
func (h *Directory) KnowledgeBaseMembers(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "KB_MEMBERS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBaseMembers(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("q"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "KB_MEMBERS", run)
}

func (h *Directory) DocumentCollaborators(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DOCUMENT_COLLABORATORS", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DocumentCollaborators(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "DOCUMENT_COLLABORATORS", run)
}

func (h *Directory) DocumentMentionCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DOCUMENT_MENTION_CANDIDATES", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DocumentMentionCandidates(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "DOCUMENT_MENTION_CANDIDATES", run)
}

// document_creation.go：保留原实现，按业务合并文件。

type Documents struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Documents
}

func NewDocuments(identity *Identity, requests *Transactions, usecase *logic.Documents) *Documents {
	return &Documents{identity: identity, requests: requests, usecase: usecase}
}
func (h *Documents) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	space, err := domain.CanonicalID(input.SpaceId)
	if err != nil || !domain.ValidName(input.Title, 240) || !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy)) {
		httpx.Fail(w, 400, "DOCUMENT_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CREATE_DOCUMENT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_CREATE", input, 201, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Create(r.Context(), actor, space, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_CREATE", input, 201, run, nil)
}

// editable.go：保留原实现，按业务合并文件。

type EditableUnits struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.EditableUnits
}

func NewEditableUnits(identity *Identity, requests *Transactions, usecase *logic.EditableUnits) *EditableUnits {
	return &EditableUnits{identity: identity, requests: requests, usecase: usecase}
}
func (h *EditableUnits) List(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "EDITABLE_UNITS", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Prepare(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "EDITABLE_UNITS", run)
}

// knowledge_base.go：保留原实现，按业务合并文件。

type KnowledgeBases struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.KnowledgeBases
}

func NewKnowledgeBases(identity *Identity, requests *Transactions, usecase *logic.KnowledgeBases) *KnowledgeBases {
	return &KnowledgeBases{identity: identity, requests: requests, usecase: usecase}
}
func (h *KnowledgeBases) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.KBCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Name, 160) || !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy)) {
		httpx.Fail(w, 400, "KNOWLEDGE_BASE_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CREATE_KNOWLEDGE_BASE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KNOWLEDGE_BASE_CREATE", input, 201, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Create(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KNOWLEDGE_BASE_CREATE", input, 201, run, nil)
}
func (h *KnowledgeBases) Rename(w http.ResponseWriter, r *http.Request) {
	var input dto.OrganizationEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Name, 160) {
		httpx.Fail(w, 400, "KB_NAME_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "RENAME_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_RENAME", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Rename(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_RENAME", input, 200, run, nil)
}
func (h *KnowledgeBases) CreationGrant(w http.ResponseWriter, r *http.Request) {
	var input dto.CreationGrant
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.UserId)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "KB_CREATION_GRANT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_CREATION_GRANT", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.CreationGrant(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_CREATION_GRANT", input, 200, run, nil)
}
func (h *KnowledgeBases) Role(w http.ResponseWriter, r *http.Request) {
	var input dto.KBRole
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.UserId)
	if err != nil || (input.Role != "ADMIN" && input.Role != "MEMBER" && input.Role != "REMOVE") {
		httpx.Fail(w, 400, "KB_ROLE_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "APPOINT_KB_ROLE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_ROLE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Role(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_ROLE", input, 202, run, nil)
}

// management.go：保留原实现，按业务合并文件。

type Management struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Management
}

func NewManagement(identity *Identity, requests *Transactions, usecase *logic.Management) *Management {
	return &Management{identity: identity, requests: requests, usecase: usecase}
}
func (h *Management) Rename(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Title, 240) {
		httpx.Fail(w, 400, "DOCUMENT_TITLE_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "RENAME_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_RENAME", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Rename(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_RENAME", input, 200, run, nil)
}
func (h *Management) Transfer(w http.ResponseWriter, r *http.Request) {
	var input dto.OwnerChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.TargetUserId)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "TRANSFER_RESPONSIBLE", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 转交后发起人可以读取原号最小结果；TryReplay不会重新执行新的转交动作。
	if h.requests.Replay(w, r, actor, input.OperationId, "RESPONSIBLE_TRANSFER", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Transfer(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESPONSIBLE_TRANSFER", input, 200, run, nil)
}
func (h *Management) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DELETE_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_DELETE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Delete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_DELETE", input, 202, run, nil)
}

// space.go：保留原实现，按业务合并文件。

type Spaces struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Spaces
}

func NewSpaces(identity *Identity, requests *Transactions, usecase *logic.Spaces) *Spaces {
	return &Spaces{identity: identity, requests: requests, usecase: usecase}
}
func (h *Spaces) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "SPACE_DIRECTORY", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Prepare(r.Context(), actor, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "SPACE_DIRECTORY", run)
}
````

UTF-8/LF全文SHA-256：`a8d4cf9dc3ef4d6bbcd16f41f6b70d0cab88c37fa8f341a36c47682bfa7c1965`。[参考实现](../../../../services/document/internal/handler/document/workspace_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
