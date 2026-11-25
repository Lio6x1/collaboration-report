# 分步编写 `services/document/internal/handler/document/discussion_handlers.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/252-discussion-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供批注与通知HTTP操作，先由持久目标定位真实文档后认证。

客户端自报document_id不能改变现有线程/评论所属范围。 列表入口把context_kind/context_id/content_version_id组成可选ThreadFilter，仍由业务层统一验证并在受控事务中输出。

写之前先按这条执行路线理解各段的连接：

1. threadTarget读取原线程
2. commentChange读取真实评论与线程
3. commentRevision接受规范零供业务CAS
4. Notifications.ReviewRound先按收件人定位原目标。
5. ListThreads先按documentId建立COMMENT_CONTEXT身份，读取URL limit/cursor及三项上下文筛选，传Comments.List后由Transactions.Read输出。
6. ListComments仍从持久thread定位真实document，回复列表只在用户选择/展开线程时按需调用，不根据客户端自报文档绕过threadTarget。

本文件需要守住的边界：

- 零revision不是非法ID，应该进入CAS形成冲突
- 批注删除保留墓碑，响应仍走幂等事务和受控输出。
- 三个筛选参数属于只读URL扩展，不改变原创建锚点请求体；context_id需要context_kind，错误不能静默回退全列表。
- 线程筛选、回复分页和提及候选是三个范围；不能为了显示当前10条线程预取全库回复。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/discussion_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/discussion_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/discussion_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/discussion_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:521 -->
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

````

## 第 2 步：Comments

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type Comments struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Comments</code> | 使用 <code>logic.Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。 |

<!-- write-step: 2 bytes=521:617 -->
````go
type Comments struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Comments
}

````

## 第 3 步：NewComments

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewComments(identity *Identity, requests *Transactions, usecase *logic.Comments) *Comments`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Comments</code> | 使用 <code>logic.Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Comments</code> | 实际返回表达式：<code>&amp;Comments{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Comments{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=617:794 -->
````go
func NewComments(identity *Identity, requests *Transactions, usecase *logic.Comments) *Comments {
	return &Comments{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 4 步：Comments.CreateThread

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (h *Comments) CreateThread(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;COMMENT_THREAD_CREATE&quot;, input, http.StatusCreated, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;COMMENT_CONTEXT&quot;, document)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=794:1464 -->
````go
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

````

## 第 5 步：Comments.ListThreads

插入位置：第 4 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func (h *Comments) ListThreads(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 用同一ThreadFilter将HTTP只读上下文条件送往真实业务验证、签名游标和SQL过滤。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 返回CommentThreadPage或明确参数/权限错误。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;COMMENT_THREADS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 路径documentId；URL limit/cursor/context_kind/context_id/content_version_id；身份仍取当前会话。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;COMMENT_CONTEXT&quot;, document)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1464:2191 -->
````go
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

````

## 第 6 步：Comments.threadTarget

插入位置：第 5 步代码之后，紧接追加；完成稿约第 69 行。

线程、评论入口从持久记录取得原文档后建立身份准入，不能信任客户端另报范围。

本步声明：`func (h *Comments) threadTarget(w http.ResponseWriter, r *http.Request, mutation bool) (comments.Thread, logic.Actor, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, mutation, &quot;COMMENT_CONTEXT&quot;, thread.Document)</code> |
| <code>mutation</code> | <code>bool</code> | 在此声明中的实际使用：<code>h.identity.Actor(r, mutation, &quot;COMMENT_CONTEXT&quot;, thread.Document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>实际返回表达式：<code>comments.Thread{}</code>；<code>thread</code> |
| <code>(匿名/未命名)</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>logic.Actor{}</code>；<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return comments.Thread{}, logic.Actor{}, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return thread, logic.Actor{}, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return thread, actor, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return thread, actor, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2191:2909 -->
````go
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

````

## 第 7 步：Comments.ListComments

插入位置：第 6 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func (h *Comments) ListComments(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.threadTarget(w, r, false)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, thread.Document, &quot;COMMENTS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.threadTarget(w, r, false)</code>；<code>r.Context</code>；<code>r.URL</code>；<code>h.requests.Read(w, r, actor, sources, thread.Document, &quot;COMMENTS&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2909:3314 -->
````go
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

````

## 第 8 步：Comments.ReadThread

插入位置：第 7 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func (h *Comments) ReadThread(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.threadTarget(w, r, false)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, thread.Document, &quot;COMMENT_THREAD&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.threadTarget(w, r, false)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, thread.Document, &quot;COMMENT_THREAD&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3314:3664 -->
````go
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

````

## 第 9 步：Comments.Reply

插入位置：第 8 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`func (h *Comments) Reply(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.threadTarget(w, r, true)</code>；<code>h.change(w, r, actor, thread, command, input, &quot;COMMENT_REPLY&quot;, http.StatusCreated)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.threadTarget(w, r, true)</code>；<code>h.change(w, r, actor, thread, command, input, &quot;COMMENT_REPLY&quot;, http.StatusCreated)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>input.ReplyToId != nil</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=3664:4335 -->
````go
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

````

## 第 10 步：Comments.State

插入位置：第 9 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func (h *Comments) State(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.threadTarget(w, r, true)</code>；<code>h.change(w, r, actor, thread, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;STATE&quot;, State: input.State, Revision: revision}, input, &quot;COMMENT_THREAD_STATE&quot;, http.StatusOK)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.threadTarget(w, r, true)</code>；<code>h.change(w, r, actor, thread, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;STATE&quot;, State: input.State, Revision: revision}, input, &quot;COMMENT_THREAD_STATE&quot;, http.StatusOK)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=4335:4848 -->
````go
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

````

## 第 11 步：Comments.Edit

插入位置：第 10 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`func (h *Comments) Edit(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;EDIT&quot;, Body: input.Body}, input.ExpectedRevision, input, &quot;COMMENT_EDIT&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;EDIT&quot;, Body: input.Body}, input.ExpectedRevision, input, &quot;COMMENT_EDIT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=4848:5148 -->
````go
func (h *Comments) Edit(w http.ResponseWriter, r *http.Request) {
	var input dto.CommentEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: "EDIT", Body: input.Body}, input.ExpectedRevision, input, "COMMENT_EDIT")
}

````

## 第 12 步：Comments.Delete

插入位置：第 11 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func (h *Comments) Delete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;DELETE&quot;}, input.ExpectedRevision, input, &quot;COMMENT_DELETE&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: &quot;DELETE&quot;}, input.ExpectedRevision, input, &quot;COMMENT_DELETE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=5148:5440 -->
````go
func (h *Comments) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	h.commentChange(w, r, logic.CommentCommand{Operation: input.OperationId, Kind: "DELETE"}, input.ExpectedRevision, input, "COMMENT_DELETE")
}

````

## 第 13 步：Comments.commentChange

插入位置：第 12 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func (h *Comments) commentChange(w http.ResponseWriter, r *http.Request, command logic.CommentCommand, expected string, input any, kind string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, actor, thread, command, input, kind, http.StatusOK)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, true, &quot;COMMENT_CONTEXT&quot;, thread.Document)</code>；<code>h.change(w, r, actor, thread, command, input, kind, http.StatusOK)</code> |
| <code>command</code> | <code>logic.CommentCommand</code> | 使用 <code>logic.CommentCommand</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。<br>在此声明中的实际使用：<code>command.Comment</code>；<code>command.Revision</code>；<code>h.change(w, r, actor, thread, command, input, kind, http.StatusOK)</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>commentRevision(expected)</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>h.change(w, r, actor, thread, command, input, kind, http.StatusOK)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>h.change(w, r, actor, thread, command, input, kind, http.StatusOK)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5440:6171 -->
````go
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

````

## 第 14 步：Comments.change

插入位置：第 13 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func (h *Comments) change(w http.ResponseWriter, r *http.Request, actor logic.Actor, thread comments.Thread, command logic.CommentCommand, input any, kind string, status int)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code>；<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, run, present, err := h.usecase.Change(r.Context(), actor, thread, command)</code>；<code>h.usecase.Change(r.Context(), actor, thread, command)</code>；<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |
| <code>thread</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>在此声明中的实际使用：<code>h.usecase.Change(r.Context(), actor, thread, command)</code> |
| <code>command</code> | <code>logic.CommentCommand</code> | 使用 <code>logic.CommentCommand</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。<br>在此声明中的实际使用：<code>h.usecase.Change(r.Context(), actor, thread, command)</code>；<code>command.Operation</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6171:6700 -->
````go
func (h *Comments) change(w http.ResponseWriter, r *http.Request, actor logic.Actor, thread comments.Thread, command logic.CommentCommand, input any, kind string, status int) {
	actor, run, present, err := h.usecase.Change(r.Context(), actor, thread, command)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 同一幂等事务处理状态、评论与通知意图；投影仍走统一输出租约和当前权限复核。
	h.requests.PresentedMutation(w, r, actor, command.Operation, kind, input, status, run, present)
}

````

## 第 15 步：commentRevision

插入位置：第 14 步代码之后，紧接追加；完成稿约第 205 行。

修订号允许规范的零进入业务 CAS 并产生冲突；拒绝前导零、符号与溢出。

本步声明：`func commentRevision(raw string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(raw, 10, 64)</code>；<code>strconv.FormatUint(value, 10) != raw</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_REVISION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(value, 10) != raw</code>。条件成立时执行 <code>return 0, domain.Violation(domain.InvalidRequest, &quot;COMMENT_REVISION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6700:7140 -->
````go
// 修订号允许规范的零进入业务 CAS 并产生冲突；拒绝前导零、符号与溢出。
func commentRevision(raw string) (uint64, error) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != raw {
		return 0, domain.Violation(domain.InvalidRequest, "COMMENT_REVISION_INVALID")
	}
	return value, nil
}

// notification_review_target.go：保留原实现，按业务合并文件。

````

## 第 16 步：Notifications.ReviewRound

插入位置：第 15 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`func (h *Notifications) ReviewRound(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;NOTIFICATION_REVIEW_ROUND&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;NOTIFICATION_REVIEW_ROUND&quot;, 0)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, false, &quot;NOTIFICATION_REVIEW_ROUND&quot;, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7140:8248 -->
````go
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

````

## 第 17 步：Notifications

插入位置：第 16 步代码之后，紧接追加；完成稿约第 253 行。

本步声明：`type Notifications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Notifications</code> | 使用 <code>logic.Notifications</code> 类型；其定义处的职责：处理审核/离组收件快照、通知列表过滤、固定轮次跳转与已读。 |

<!-- write-step: 17 bytes=8248:8354 -->
````go
type Notifications struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Notifications
}

````

## 第 18 步：NewNotifications

插入位置：第 17 步代码之后，紧接追加；完成稿约第 259 行。

本步声明：`func NewNotifications(identity *Identity, requests *Transactions, usecase *logic.Notifications) *Notifications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Notifications</code> | 使用 <code>logic.Notifications</code> 类型；其定义处的职责：处理审核/离组收件快照、通知列表过滤、固定轮次跳转与已读。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Notifications</code> | 实际返回表达式：<code>&amp;Notifications{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Notifications{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8354:8551 -->
````go
func NewNotifications(identity *Identity, requests *Transactions, usecase *logic.Notifications) *Notifications {
	return &Notifications{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 19 步：Notifications.List

插入位置：第 18 步代码之后，紧接追加；完成稿约第 263 行。

本步声明：`func (h *Notifications) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;NOTIFICATIONS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;NOTIFICATIONS&quot;, 0)</code>；<code>r.Context</code>；<code>r.URL</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;NOTIFICATIONS&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8551:8971 -->
````go
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

````

## 第 20 步：Notifications.MarkRead

插入位置：第 19 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`func (h *Notifications) MarkRead(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;NOTIFICATION_READ&quot;, input, http.StatusOK, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;NOTIFICATION_READ&quot;, 0)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;NOTIFICATION_READ&quot;, input, http.StatusOK, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8971:9549 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/discussion_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
