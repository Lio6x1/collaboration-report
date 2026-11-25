# services/document/internal/handler/document/trash_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/264-trash-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Purges`、`NewPurges`、`Purge`、`SetPurges`、`purgeCleanupContext`、`Trash`、`NewTrash`、`List`、`Restore`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// purge.go：保留原实现，按业务合并文件。

type Purges struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Purges
}

func NewPurges(identity *Identity, requests *Transactions, usecase *logic.Purges) *Purges {
	return &Purges{identity: identity, requests: requests, usecase: usecase}
}

func (h *Purges) Purge(w http.ResponseWriter, r *http.Request) {
	var input dto.PurgeAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if _, err = domain.CanonicalRevision(input.ExpectedControlRevision); err != nil {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "CONTROL_REVISION_INVALID"))
		return
	}
	if !input.Confirmed {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "PURGE_CONFIRMATION_REQUIRED"))
		return
	}
	actor, err := h.identity.Actor(r, true, "PURGE_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 最终PURGED后原POST仍能得到精确固定回执；新原号不能借旧receipt获得资格。
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_PURGE", input, http.StatusAccepted, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_PURGE", input, http.StatusAccepted, run, present)
}

// purge_context.go：保留原实现，按业务合并文件。

// 明确依赖注入；旧Account构造/路由仍可独立使用，没有全局服务定位器。
func (h *AccountRetirements) SetPurges(purges *logic.Purges) {
	h.purges = purges
}

func (h *AccountRetirements) purgeCleanupContext(w http.ResponseWriter, r *http.Request, request platform.AccountDocumentCleanupRequest) bool {
	if h.purges == nil {
		return false
	}
	handles, err := h.purges.Handles(r.Context(), request.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	if !handles {
		return false
	}
	result, err := h.purges.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	// cleanup_kind只用于Doc→Node内部authority分派。Node的CLEANED回复必须剥离
	// 此字段，保持既有platform清理context/回执的固定字节契约。
	platform.WriteJSON(w, http.StatusOK, struct {
		platform.AccountDocumentCleanupContext
		CleanupKind string `json:"cleanup_kind"`
	}{AccountDocumentCleanupContext: result, CleanupKind: "PURGE"})
	return true
}

// trash.go：保留原实现，按业务合并文件。

type Trash struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Trash
}

func NewTrash(identity *Identity, requests *Transactions, usecase *logic.Trash) *Trash {
	return &Trash{identity: identity, requests: requests, usecase: usecase}
}

func (h *Trash) List(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "LIST_TRASH", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.List(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "LIST_TRASH", run)
}

func (h *Trash) Restore(w http.ResponseWriter, r *http.Request) {
	var input dto.TrashAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if _, err := domain.CanonicalRevision(input.ExpectedControlRevision); err != nil {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "CONTROL_REVISION_INVALID"))
		return
	}
	if input.Reason != nil && len([]rune(*input.Reason)) > 2000 {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "TRASH_REASON_INVALID"))
		return
	}
	actor, err := h.identity.Actor(r, true, "RESTORE_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 原号只重放最小固定结果；成功恢复后文档已非TRASHED，也不能重复执行恢复。
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_RESTORE", input, http.StatusAccepted, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Restore(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_RESTORE", input, http.StatusAccepted, run, present)
}
````

UTF-8/LF全文SHA-256：`cb59ce0931972f4851766facc6cdc629154d20e5a0d826553f0cf4018abc19f9`。[参考实现](../../../../services/document/internal/handler/document/trash_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
