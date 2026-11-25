# 分步编写 `services/document/internal/handler/document/authorization_handlers.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/251-authorization-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。

入口统一验证DTO与规范ID，使业务层只接收明确意图；输出仍须经过Transactions登记。

写之前先按这条执行路线理解各段的连接：

1. Identity.Actor验证会话令牌
2. optionalActor先检查原始Cookie是否存在/重复
3. Authorizations.Change先查原号再准备变更
4. Sharing各入口把业务闭包交给PresentedMutation或Read。

本文件需要守住的边界：

- 畸形或重复会话Cookie不能降级为匿名
- PUBLIC导出带Cookie也属于需要CSRF的变更。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/authorization_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/authorization_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/authorization_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/authorization_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:481 -->
````go
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

````

## 第 2 步：Sharing.Apply

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func (h *Sharing) Apply(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Fail(w, 400, &quot;APPLICATION_INPUT_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_LINK_APPLY&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.identity.Actor(r, true, &quot;APPLY_DOCUMENT_INVITATION&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_LINK_APPLY&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(&#91;&#93;rune(input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_LINK_APPLY&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=481:1302 -->
````go
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
````

## 第 3 步：Sharing.Decide

插入位置：第 2 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func (h *Sharing) Decide(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;APPLICATION_DECISION_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_APPLICATION_DECIDE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;DECIDE_DOCUMENT_APPLICATION&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_APPLICATION_DECIDE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>(input.Decision != &quot;ACCEPT&quot; &amp;&amp; input.Decision != &quot;REJECT&quot;) &#124;&#124; len(&#91;&#93;rune(input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_APPLICATION_DECIDE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1302:2248 -->
````go
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
````

## 第 4 步：Sharing.Applications

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (h *Sharing) Applications(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;LIST_DOCUMENT_APPLICATIONS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;LIST_DOCUMENT_APPLICATIONS&quot;, 0)</code>；<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>value != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2248:2934 -->
````go
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
````

## 第 5 步：Sharing.OperationProgress

插入位置：第 4 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func (h *Sharing) OperationProgress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, &quot;OPERATION_PROGRESS&quot;, h.usecase.OperationProgress(actor, id))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;OPERATION_PROGRESS&quot;, 0)</code>；<code>pathvar.Vars(r)</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, &quot;OPERATION_PROGRESS&quot;, h.usecase.OperationProgress(actor, id))</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidOperationID(id)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2934:3447 -->
````go
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

````

## 第 6 步：Authorizations

插入位置：第 5 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`type Authorizations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | HTTP身份适配器，从当前请求核验会话并取得Actor；业务授权仍由logic处理。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>changes</code> | <code>*logic.AuthorizationChanges</code> | 使用 <code>logic.AuthorizationChanges</code> 类型；其定义处的职责：先确认操作者可管理文档并绑定受影响身份，再准备授权变更的事务执行体；权限撤销和来源关闭需沿用同一操作的恢复边界。 |
| <code>authorizations</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |

<!-- write-step: 6 bytes=3447:3617 -->
````go
type Authorizations struct {
	identity       *Identity
	requests       *Transactions
	changes        *logic.AuthorizationChanges
	authorizations *logic.Authorizations
}

````

## 第 7 步：NewAuthorizations

插入位置：第 6 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`func NewAuthorizations(identity *Identity, requests *Transactions, changes *logic.AuthorizationChanges, authorizations *logic.Authorizations) *Authorizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>changes</code> | <code>*logic.AuthorizationChanges</code> | 使用 <code>logic.AuthorizationChanges</code> 类型；其定义处的职责：先确认操作者可管理文档并绑定受影响身份，再准备授权变更的事务执行体；权限撤销和来源关闭需沿用同一操作的恢复边界。<br>在此声明中的实际使用：<code>changes: changes</code> |
| <code>authorizations</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>authorizations: authorizations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authorizations</code> | 实际返回表达式：<code>&amp;Authorizations{identity: identity, requests: requests, changes: changes, authorizations: authorizations}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authorizations{identity: identity, requests: requests, changes: changes, authorizations: authorizations}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3617:3893 -->
````go
func NewAuthorizations(identity *Identity, requests *Transactions, changes *logic.AuthorizationChanges, authorizations *logic.Authorizations) *Authorizations {
	return &Authorizations{identity: identity, requests: requests, changes: changes, authorizations: authorizations}
}
````

## 第 8 步：Authorizations.Change

插入位置：第 7 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func (h *Authorizations) Change(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;AUTHORIZATION_CHANGES_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_AUTHORIZATION&quot;, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;CHANGE_AUTHORIZATION&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_AUTHORIZATION&quot;, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(input.Changes) == 0 &#124;&#124; len(input.Changes) &gt; 100</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_AUTHORIZATION&quot;, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3893:4850 -->
````go
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
````

## 第 9 步：Authorizations.Progress

插入位置：第 8 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func (h *Authorizations) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, &quot;OPERATION_PROGRESS&quot;, h.authorizations.Progress(actor, operation))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;OPERATION_PROGRESS&quot;, 0)</code>；<code>pathvar.Vars(r)</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, &quot;OPERATION_PROGRESS&quot;, h.authorizations.Progress(actor, operation))</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidOperationID(operation)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4850:5375 -->
````go
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

````

## 第 10 步：Identity

插入位置：第 9 步代码之后，紧接追加；完成稿约第 163 行。

本步声明：`type Identity struct{ usecase *logic.Identity }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |

<!-- write-step: 10 bytes=5375:5424 -->
````go
type Identity struct{ usecase *logic.Identity }

````

## 第 11 步：NewIdentity

插入位置：第 10 步代码之后，紧接追加；完成稿约第 165 行。

本步声明：`func NewIdentity(usecase *logic.Identity) *Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Identity</code> | 实际返回表达式：<code>&amp;Identity{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Identity{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5424:5516 -->
````go
func NewIdentity(usecase *logic.Identity) *Identity { return &Identity{usecase: usecase} }

````

## 第 12 步：Identity.OptionalActor

插入位置：第 11 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`func (h *Identity) OptionalActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.optionalActor(r, false, purpose, document)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.optionalActor(r, false, purpose, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.optionalActor(r, false, purpose, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>h.optionalActor(r, false, purpose, document)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>h.optionalActor(r, false, purpose, document)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return h.optionalActor(r, false, purpose, document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5516:5679 -->
````go
func (h *Identity) OptionalActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error) {
	return h.optionalActor(r, false, purpose, document)
}

````

## 第 13 步：Identity.OptionalMutationActor

插入位置：第 12 步代码之后，紧接追加；完成稿约第 171 行。

带Cookie的PUBLIC导出仍是用户变更，必须验证Origin/CSRF；无Cookie才走匿名准入。

本步声明：`func (h *Identity) OptionalMutationActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.optionalActor(r, true, purpose, document)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.optionalActor(r, true, purpose, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.optionalActor(r, true, purpose, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>h.optionalActor(r, true, purpose, document)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>h.optionalActor(r, true, purpose, document)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return h.optionalActor(r, true, purpose, document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5679:5954 -->
````go
// 带Cookie的PUBLIC导出仍是用户变更，必须验证Origin/CSRF；无Cookie才走匿名准入。
func (h *Identity) OptionalMutationActor(r *http.Request, purpose string, document uint64) (*logic.Actor, error) {
	return h.optionalActor(r, true, purpose, document)
}

````

## 第 14 步：Identity.optionalActor

插入位置：第 13 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func (h *Identity) optionalActor(r *http.Request, mutation bool, purpose string, document uint64) (*logic.Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Header</code>；<code>h.Actor(r, mutation, purpose, document)</code> |
| <code>mutation</code> | <code>bool</code> | 在此声明中的实际使用：<code>h.Actor(r, mutation, purpose, document)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.Actor(r, mutation, purpose, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.Actor(r, mutation, purpose, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>nil</code>；<code>&amp;actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>domain.Violation(domain.SessionRequired, &quot;SESSION_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>strings.TrimSpace(name) == &quot;__Host-collabSession&quot;</code>。
- 检查 <code>count == 0</code>。条件成立时执行 <code>return nil, nil</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return nil, domain.Violation(domain.SessionRequired, &quot;SESSION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5954:6742 -->
````go
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
````

## 第 15 步：Identity.Actor

插入位置：第 14 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func (h *Identity) Actor(r *http.Request, mutation bool, purpose string, document uint64) (logic.Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Cookie</code>；<code>r.Context</code>；<code>r.Header</code> |
| <code>mutation</code> | <code>bool</code> | 在此声明中的实际使用：<code>Mutation: mutation</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get(&quot;Cookie&quot;), Origin: r.Header.Get(&quot;Origin&quot;), CSRF: r.Header.Get(&quot;X-CSRF-Token&quot;), Mutation: mutation}, purpose, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get(&quot;Cookie&quot;), Origin: r.Header.Get(&quot;Origin&quot;), CSRF: r.Header.Get(&quot;X-CSRF-Token&quot;), Mutation: mutation}, purpose, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get(&quot;Cookie&quot;), Origin: r.Header.Get(&quot;Origin&quot;), CSRF: r.Header.Get(&quot;X-CSRF-Token&quot;), Mutation: mutation}, purpose, document)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>logic.Actor{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get(&quot;Cookie&quot;), Origin: r.Header.Get(&quot;Origin&quot;), CSRF: r.Header.Get(&quot;X-CSRF-Token&quot;), Mutation: mutation}, purpose, document)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.SessionRequired, &quot;SESSION_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return logic.Actor{}, domain.Violation(domain.SessionRequired, &quot;SESSION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(token) != 32 &#124;&#124; base64.RawURLEncoding.EncodeToString(token) != cookie.Value</code>。条件成立时执行 <code>return logic.Actor{}, domain.Violation(domain.SessionRequired, &quot;SESSION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return h.usecase.Authenticate(r.Context(), token, domain.SessionVerification{Cookie: r.Header.Get(&quot;Cookie&quot;), Origin: r.Header.Get(&quot;Origin&quot;), CSRF: r.Header.Get(&quot;X-CSRF-Token&quot;), Mutation: mutation}, purpose, document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6742:7544 -->
````go
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

````

## 第 16 步：Sharing

插入位置：第 15 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`type Sharing struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | HTTP身份适配器，从当前请求核验会话并取得Actor；业务授权仍由logic处理。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Sharing</code> | 使用 <code>logic.Sharing</code> 类型；其定义处的职责：实现个人文档邀请、申请、审批、状态分页与安全归档。 |

<!-- write-step: 16 bytes=7544:7638 -->
````go
type Sharing struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Sharing
}

````

## 第 17 步：NewSharing

插入位置：第 16 步代码之后，紧接追加；完成稿约第 220 行。

本步声明：`func NewSharing(identity *Identity, requests *Transactions, usecase *logic.Sharing) *Sharing`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Sharing</code> | 使用 <code>logic.Sharing</code> 类型；其定义处的职责：实现个人文档邀请、申请、审批、状态分页与安全归档。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Sharing</code> | 实际返回表达式：<code>&amp;Sharing{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Sharing{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7638:7810 -->
````go
func NewSharing(identity *Identity, requests *Transactions, usecase *logic.Sharing) *Sharing {
	return &Sharing{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 18 步：Sharing.CreateInvitation

插入位置：第 17 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func (h *Sharing) CreateInvitation(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;INVITATION_EXPIRY_INVALID&quot;)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;DOCUMENT_INVITATION_CREATE&quot;, input, 200, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;CREATE_DOCUMENT_INVITATION&quot;, id)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.ExpiresInHours != nil</code>。
- 检查 <code>hours &lt; 1 &#124;&#124; hours &gt; 720</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7810:8758 -->
````go
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
````

## 第 19 步：Sharing.CancelInvitation

插入位置：第 18 步代码之后，紧接追加；完成稿约第 254 行。

本步声明：`func (h *Sharing) CancelInvitation(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_INVITATION_CANCEL&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;DOCUMENT_INVITATION_CANCEL&quot;, input, 200, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;CANCEL_DOCUMENT_INVITATION&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_INVITATION_CANCEL&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_INVITATION_CANCEL&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8758:9552 -->
````go
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
````

## 第 20 步：Sharing.Invitations

插入位置：第 19 步代码之后，紧接追加；完成稿约第 279 行。

本步声明：`func (h *Sharing) Invitations(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;LIST_DOCUMENT_INVITATIONS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;LIST_DOCUMENT_INVITATIONS&quot;, id)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=9552:10154 -->
````go
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

````

## 第 21 步：Sharing.ArchiveInvitation

插入位置：第 20 步代码之后，紧接追加；完成稿约第 298 行。

本步声明：`func (h *Sharing) ArchiveInvitation(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.archive(w, r, &quot;INVITATION&quot;, &quot;invitationId&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.archive(w, r, &quot;INVITATION&quot;, &quot;invitationId&quot;)</code> |

<!-- write-step: 21 bytes=10154:10281 -->
````go
func (h *Sharing) ArchiveInvitation(w http.ResponseWriter, r *http.Request) {
	h.archive(w, r, "INVITATION", "invitationId")
}
````

## 第 22 步：Sharing.ArchiveApplication

插入位置：第 21 步代码之后，紧接追加；完成稿约第 301 行。

本步声明：`func (h *Sharing) ArchiveApplication(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.archive(w, r, &quot;APPLICATION&quot;, &quot;applicationId&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.archive(w, r, &quot;APPLICATION&quot;, &quot;applicationId&quot;)</code> |

<!-- write-step: 22 bytes=10281:10411 -->
````go
func (h *Sharing) ArchiveApplication(w http.ResponseWriter, r *http.Request) {
	h.archive(w, r, "APPLICATION", "applicationId")
}
````

## 第 23 步：Sharing.archive

插入位置：第 22 步代码之后，紧接追加；完成稿约第 304 行。

本步声明：`func (h *Sharing) archive(w http.ResponseWriter, r *http.Request, kind, parameter string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, operation, input, 200, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, operation, input, 200, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, operation, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, operation, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;DOCUMENT_&quot; + kind</code>；<code>h.usecase.Archive(r.Context(), actor, id, kind, input)</code> |
| <code>parameter</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.CanonicalID(pathvar.Vars(r)&#91;parameter&#93;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, operation, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=10411:11201 -->
````go
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
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/authorization_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
