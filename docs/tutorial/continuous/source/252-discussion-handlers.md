# services/document/internal/handler/document/discussion_handlers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/252-discussion-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Comments`、`NewComments`、`CreateThread`、`ListThreads`、`threadTarget`、`ListComments`、`ReadThread`、`Reply`、`State`、`Edit`、`Delete`、`commentChange`、`change`、`commentRevision`、`ReviewRound`、`Notifications`、`NewNotifications`、`List`、`MarkRead`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	comments "collaboration/document/internal/repository/comment"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"strconv"
)

// comments.go：保留原实现，按业务合并文件。

type Comments struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Comments
}

func NewComments(identity *Identity, requests *Transactions, usecase *logic.Comments) *Comments {
	return &Comments{identity: identity, requests: requests, usecase: usecase}
}

func (h *Comments) CreateThread(w http.ResponseWriter, r *http.Request) {
	var input dto.CommentThreadCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "COMMENT_CONTEXT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Create(r.Context(), actor, document, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "COMMENT_THREAD_CREATE", input, http.StatusCreated, run, present)
}

func (h *Comments) ListThreads(w http.ResponseWriter, r *http.Request) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "COMMENT_CONTEXT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.List(r.Context(), actor, document, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), comments.ThreadFilter{Kind: r.URL.Query().Get("context_kind"), Context: r.URL.Query().Get("context_id"), Version: r.URL.Query().Get("content_version_id")})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "COMMENT_THREADS", run)
}

// 线程、评论入口从持久记录取得原文档后建立身份准入，不能信任客户端另报范围。
func (h *Comments) threadTarget(w http.ResponseWriter, r *http.Request, mutation bool) (comments.Thread, logic.Actor, bool) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["threadId"])
	if err != nil {
		httpx.Failure(w, err)
		return comments.Thread{}, logic.Actor{}, false
	}
	thread, err := h.usecase.LookupThread(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return thread, logic.Actor{}, false
	}
	actor, err := h.identity.Actor(r, mutation, "COMMENT_CONTEXT", thread.Document)
	if err != nil {
		httpx.Failure(w, err)
		return thread, actor, false
	}
	return thread, actor, true
}

func (h *Comments) ListComments(w http.ResponseWriter, r *http.Request) {
	thread, actor, ok := h.threadTarget(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.ReadComments(r.Context(), actor, thread, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, thread.Document, "COMMENTS", run)
}

func (h *Comments) ReadThread(w http.ResponseWriter, r *http.Request) {
	thread, actor, ok := h.threadTarget(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.ReadThread(r.Context(), actor, thread)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, thread.Document, "COMMENT_THREAD", run)
}

func (h *Comments) Reply(w http.ResponseWriter, r *http.Request) {
	var input dto.CommentCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	command := logic.CommentCommand{Operation: input.OperationId, Kind: "REPLY", Body: input.Body, Mentions: input.Mentions}
	if input.ReplyToId != nil {
		id, err := domain.CanonicalID(*input.ReplyToId)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		command.Reply = &id
	}
	thread, actor, ok := h.threadTarget(w, r, true)
	if !ok {
		return
	}
	// 冻结 OpenAPI 的回复成功状态为 201，与新评论实体创建一致。
	h.change(w, r, actor, thread, command, input, "COMMENT_REPLY", http.StatusCreated)
}

func (h *Comments) State(w http.ResponseWriter, r *http.Request) {
	var input dto.ThreadState
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	revision, err := commentRevision(input.ExpectedRevision)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	thread, actor, ok := h.threadTarget(w, r, true)
	if !ok {
		return
	}
	h.change(w, r, actor, thread, logic.CommentCommand{Operation: input.OperationId, Kind: "STATE", State: input.State, Revision: revision}, input, "COMMENT_THREAD_STATE", http.StatusOK)
}

func (h *Comments) Edit(w http.ResponseWriter, r *http.Request) {
	var input dto.CommentEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: "EDIT", Body: input.Body}, input.ExpectedRevision, input, "COMMENT_EDIT")
}

func (h *Comments) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: "DELETE"}, input.ExpectedRevision, input, "COMMENT_DELETE")
}

func (h *Comments) commentChange(w http.ResponseWriter, r *http.Request, command logic.CommentCommand, expected string, input any, kind string) {
	revision, err := commentRevision(expected)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["commentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	entry, thread, err := h.usecase.LookupComment(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "COMMENT_CONTEXT", thread.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	command.Comment, command.Revision = entry.ID, revision
	h.change(w, r, actor, thread, command, input, kind, http.StatusOK)
}

func (h *Comments) change(w http.ResponseWriter, r *http.Request, actor logic.Actor, thread comments.Thread, command logic.CommentCommand, input any, kind string, status int) {
	actor, run, present, err := h.usecase.Change(r.Context(), actor, thread, command)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 同一幂等事务处理状态、评论与通知意图；投影仍走统一输出租约和当前权限复核。
	h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)
}

// 修订号允许规范的零进入业务 CAS 并产生冲突；拒绝前导零、符号与溢出。
func commentRevision(raw string) (uint64, error) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != raw {
		return 0, domain.Violation(domain.InvalidRequest, "COMMENT_REVISION_INVALID")
	}
	return value, nil
}

// notification_review_target.go：保留原实现，按业务合并文件。

func (h *Notifications) ReviewRound(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["notificationId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "NOTIFICATION_REVIEW_ROUND", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	target, err := h.usecase.LookupReviewTarget(r.Context(), id, actor.ID)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(target.Target.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 收件人定位完成后为真实原文档重新建立身份准入，不能用document=0的身份来源输出私有正文。
	actor, err = h.identity.Actor(r, false, "NOTIFICATION_REVIEW_ROUND", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.ReviewRoundTarget(r.Context(), actor, target)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "NOTIFICATION_REVIEW_ROUND", run)
}

// notifications.go：保留原实现，按业务合并文件。

type Notifications struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Notifications
}

func NewNotifications(identity *Identity, requests *Transactions, usecase *logic.Notifications) *Notifications {
	return &Notifications{identity: identity, requests: requests, usecase: usecase}
}

func (h *Notifications) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "NOTIFICATIONS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.List(r.Context(), actor, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "NOTIFICATIONS", run)
}

func (h *Notifications) MarkRead(w http.ResponseWriter, r *http.Request) {
	var input dto.Action
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["notificationId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "NOTIFICATION_READ", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present := h.usecase.MarkRead(actor, id, input.OperationId)
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "NOTIFICATION_READ", input, http.StatusOK, run, present)
}
````

UTF-8/LF全文SHA-256：`4c1909dbad61a37e92d82a8a2f5e970b20e106bc10fcd0fd311d953f6b83a78e`。[参考实现](../../../../services/document/internal/handler/document/discussion_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
