# 分步编写 `services/document/internal/handler/document/trash_handlers.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/264-trash-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供回收列表、恢复、确认永久删除和清理上下文注入。

永久删除需confirmed与控制revision，HTTP负责登记长期任务。

写之前先按这条执行路线理解各段的连接：

1. Purges.Purge核确认
2. Trash.Restore核理由并走标准事务
3. SetPurges显式注入
4. purgeCleanupContext只处理PURGE原号。

本文件需要守住的边界：

- 不用全局服务定位器
- 恢复和清理保持各自资格与重放策略。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/trash_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/trash_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/trash_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/trash_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:470 -->
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

````

## 第 2 步：Purges

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type Purges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。 |

<!-- write-step: 2 bytes=470:562 -->
````go
type Purges struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Purges
}

````

## 第 3 步：NewPurges

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func NewPurges(identity *Identity, requests *Transactions, usecase *logic.Purges) *Purges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Purges</code> | 实际返回表达式：<code>&amp;Purges{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Purges{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=562:731 -->
````go
func NewPurges(identity *Identity, requests *Transactions, usecase *logic.Purges) *Purges {
	return &Purges{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 4 步：Purges.Purge

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func (h *Purges) Purge(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Failure(w, domain.Violation(domain.InvalidRequest, &quot;CONTROL_REVISION_INVALID&quot;))</code>；<code>httpx.Failure(w, domain.Violation(domain.InvalidRequest, &quot;PURGE_CONFIRMATION_REQUIRED&quot;))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;PURGE_DOCUMENT&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_PURGE&quot;, input, http.StatusAccepted, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!input.Confirmed</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_PURGE&quot;, input, http.StatusAccepted, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=731:1987 -->
````go
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

````

## 第 5 步：AccountRetirements.SetPurges

插入位置：第 4 步代码之后，紧接追加；完成稿约第 63 行。

明确依赖注入；旧Account构造/路由仍可独立使用，没有全局服务定位器。

本步声明：`func (h *AccountRetirements) SetPurges(purges *logic.Purges)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purges</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。<br>在此声明中的实际使用：<code>h.purges = purges</code> |

<!-- write-step: 5 bytes=1987:2171 -->
````go
// 明确依赖注入；旧Account构造/路由仍可独立使用，没有全局服务定位器。
func (h *AccountRetirements) SetPurges(purges *logic.Purges) {
	h.purges = purges
}

````

## 第 6 步：AccountRetirements.purgeCleanupContext

插入位置：第 5 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func (h *AccountRetirements) purgeCleanupContext(w http.ResponseWriter, r *http.Request, request platform.AccountDocumentCleanupRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, http.StatusOK, struct { 		platform.AccountDocumentCleanupContext 		CleanupKind string `json:&quot;cleanup_kind&quot;` 	}{AccountDocumentCleanupContext: result, CleanupKind: &quot;PURGE&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Operation</code>；<code>h.purges.DocumentCleanupContext(r.Context(), request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>h.purges == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!handles</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2171:3083 -->
````go
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

````

## 第 7 步：Trash

插入位置：第 6 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`type Trash struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Trash</code> | 使用 <code>logic.Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。 |

<!-- write-step: 7 bytes=3083:3173 -->
````go
type Trash struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Trash
}

````

## 第 8 步：NewTrash

插入位置：第 7 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func NewTrash(identity *Identity, requests *Transactions, usecase *logic.Trash) *Trash`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Trash</code> | 使用 <code>logic.Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Trash</code> | 实际返回表达式：<code>&amp;Trash{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Trash{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3173:3338 -->
````go
func NewTrash(identity *Identity, requests *Transactions, usecase *logic.Trash) *Trash {
	return &Trash{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 9 步：Trash.List

插入位置：第 8 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func (h *Trash) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;LIST_TRASH&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;LIST_TRASH&quot;, 0)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3338:3860 -->
````go
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

````

## 第 10 步：Trash.Restore

插入位置：第 9 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func (h *Trash) Restore(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Failure(w, domain.Violation(domain.InvalidRequest, &quot;CONTROL_REVISION_INVALID&quot;))</code>；<code>httpx.Failure(w, domain.Violation(domain.InvalidRequest, &quot;TRASH_REASON_INVALID&quot;))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_RESTORE&quot;, input, http.StatusAccepted, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.Reason != nil &amp;&amp; len(&#91;&#93;rune(*input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_RESTORE&quot;, input, http.StatusAccepted, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3860:5095 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/trash_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
