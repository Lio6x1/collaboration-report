# 分步编写 `services/document/internal/handler/document/merge_review_handlers.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/254-merge-review-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。

这些动作拥有不同固定意图与幂等原号，不能用一个修改正文接口绕过预览、审核或工作区条件。

写之前先按这条执行路线理解各段的连接：

1. 各Create入口解析对应领域意图后登记
2. Merges.Approve先定位review再准备
3. ConflictResolutions接收严格大正文
4. Progress投影公开操作
5. 私有Source/Committed核查询契约。

本文件需要守住的边界：

- 202表示登记或同步中，不能直接宣称Node已安装
- 公开冲突引用要保留原attempt和序号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/merge_review_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/merge_review_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/merge_review_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/merge_review_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:588 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
)

// conflict_contexts.go：保留原实现，按业务合并文件。

````

## 第 2 步：Reviews.Conflict

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func (h *Reviews) Conflict(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fact.Document, &quot;CONFLICT_CONTEXT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;REVIEW_CONTEXT&quot;, fact.Document)</code>；<code>h.requests.Read(w, r, actor, sources, fact.Document, &quot;CONFLICT_CONTEXT&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=588:1197 -->
````go
func (h *Reviews) Conflict(w http.ResponseWriter, r *http.Request) {
	fact, err := h.usecase.LookupConflict(r.Context(), pathvar.Vars(r)["conflictRef"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "REVIEW_CONTEXT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Conflict(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "CONFLICT_CONTEXT", run)
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

````

## 第 3 步：ConflictResolutions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`type ConflictResolutions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>reviews</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。 |
| <code>usecase</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。 |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 3 bytes=1197:1359 -->
````go
type ConflictResolutions struct {
	identity *Identity
	requests *Transactions
	reviews  *logic.Reviews
	usecase  *logic.Resolutions
	other    http.HandlerFunc
}

````

## 第 4 步：NewConflictResolutions

插入位置：第 3 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func NewConflictResolutions(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Resolutions, other http.HandlerFunc) *ConflictResolutions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>reviews</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>在此声明中的实际使用：<code>reviews: reviews</code> |
| <code>usecase</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>other: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ConflictResolutions</code> | 实际返回表达式：<code>&amp;ConflictResolutions{identity: identity, requests: requests, reviews: reviews, usecase: usecase, other: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ConflictResolutions{identity: identity, requests: requests, reviews: reviews, usecase: usecase, other: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1359:1652 -->
````go
func NewConflictResolutions(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Resolutions, other http.HandlerFunc) *ConflictResolutions {
	return &ConflictResolutions{identity: identity, requests: requests, reviews: reviews, usecase: usecase, other: other}
}

````

## 第 5 步：ConflictResolutions.Create

插入位置：第 4 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (h *ConflictResolutions) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBodyLimit(w, r, &amp;input, httpx.ContentBodyLimit)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadAccepted(w, r, actor, sources, origin.Document, &quot;CONFLICT_RESOLUTION_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBodyLimit(w, r, &amp;input, httpx.ContentBodyLimit)</code>；<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, true, &quot;CONFLICT_RESOLUTION&quot;, origin.Document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBodyLimit(w, r, &amp;input, httpx.ContentBodyLimit)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
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

<!-- write-step: 5 bytes=1652:2609 -->
````go
func (h *ConflictResolutions) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ConflictResolution
	if !httpx.DecodeBodyLimit(w, r, &input, httpx.ContentBodyLimit) {
		return
	}
	intent, err := domain.ParseConflictResolution(pathvar.Vars(r)["conflictRef"], input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	origin, err := h.reviews.LookupConflict(r.Context(), intent.Conflict)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "CONFLICT_RESOLUTION", origin.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, origin, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, origin.Document, "CONFLICT_RESOLUTION_RESULT", run)
}

````

## 第 6 步：ConflictResolutions.Progress

插入位置：第 5 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func (h *ConflictResolutions) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Scope.Document, &quot;CONFLICT_RESOLUTION_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.other(w, r)</code>；<code>h.identity.Actor(r, false, &quot;CONFLICT_RESOLUTION_RESULT&quot;, fixed.Scope.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2609:3355 -->
````go
func (h *ConflictResolutions) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "CONFLICT_RESOLUTION_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "CONFLICT_RESOLUTION_RESULT", run)
}

// draft_controls.go：保留原实现，按业务合并文件。

````

## 第 7 步：DraftControls

插入位置：第 6 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`type DraftControls struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。 |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 7 bytes=3355:3488 -->
````go
type DraftControls struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.DraftControls
	other    http.HandlerFunc
}

````

## 第 8 步：NewDraftControls

插入位置：第 7 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func NewDraftControls(identity *Identity, requests *Transactions, usecase *logic.DraftControls, other http.HandlerFunc) *DraftControls`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>other: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftControls</code> | 实际返回表达式：<code>&amp;DraftControls{identity: identity, requests: requests, usecase: usecase, other: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftControls{identity: identity, requests: requests, usecase: usecase, other: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3488:3723 -->
````go
func NewDraftControls(identity *Identity, requests *Transactions, usecase *logic.DraftControls, other http.HandlerFunc) *DraftControls {
	return &DraftControls{identity: identity, requests: requests, usecase: usecase, other: other}
}

````

## 第 9 步：DraftControls.Preview

插入位置：第 8 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`func (h *DraftControls) Preview(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;DISCARD_PREVIEW_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;DISCARD_PREVIEW&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
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
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3723:4839 -->
````go
func (h *DraftControls) Preview(w http.ResponseWriter, r *http.Request) {
	var input dto.DiscardPreviewCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseDiscardPreview(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
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
	actor, err := h.identity.Actor(r, true, "DISCARD_PREVIEW", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.RegisterPreview(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = h.usecase.WaitPreview(r.Context(), intent.Operation); err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.ReadPreview(r.Context(), actor, document, unit, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "DISCARD_PREVIEW_RESULT", run)
}

````

## 第 10 步：DraftControls.Progress

插入位置：第 9 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func (h *DraftControls) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.DiscardProgress(w, r)</code>；<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Scope.Document, &quot;DISCARD_PREVIEW_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.DiscardProgress(w, r)</code>；<code>h.other(w, r)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4839:5576 -->
````go
func (h *DraftControls) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.LookupPreview(r.Context(), operation)
	if err == sql.ErrNoRows {
		h.DiscardProgress(w, r)
		return
	}
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DISCARD_PREVIEW_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "DISCARD_PREVIEW_RESULT", run)
}

````

## 第 11 步：DraftControls.Discard

插入位置：第 10 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`func (h *DraftControls) Discard(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadAccepted(w, r, actor, sources, document, &quot;DISCARD_DRAFT_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;DISCARD_DRAFT&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
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

<!-- write-step: 11 bytes=5576:6561 -->
````go
func (h *DraftControls) Discard(w http.ResponseWriter, r *http.Request) {
	var input dto.DiscardDraft
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseDiscardDraft(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
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
	actor, err := h.identity.Actor(r, true, "DISCARD_DRAFT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.RegisterDiscard(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DiscardProgress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, document, "DISCARD_DRAFT_RESULT", run)
}

````

## 第 12 步：DraftControls.DiscardProgress

插入位置：第 11 步代码之后，紧接追加；完成稿约第 230 行。

本步声明：`func (h *DraftControls) DiscardProgress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Scope.Document, &quot;DISCARD_DRAFT_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.other(w, r)</code>；<code>h.identity.Actor(r, false, &quot;DISCARD_DRAFT_RESULT&quot;, fixed.Scope.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6561:7302 -->
````go
func (h *DraftControls) DiscardProgress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.LookupDiscard(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DISCARD_DRAFT_RESULT", fixed.Scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DiscardProgress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Scope.Document, "DISCARD_DRAFT_RESULT", run)
}

// merges.go：保留原实现，按业务合并文件。

````

## 第 13 步：Merges

插入位置：第 12 步代码之后，紧接追加；完成稿约第 256 行。

本步声明：`type Merges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>reviews</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。 |
| <code>usecase</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。 |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 13 bytes=7302:7481 -->
````go
type Merges struct {
	identity        *Identity
	requests        *Transactions
	reviews         *logic.Reviews
	usecase         *logic.Merges
	otherOperations http.HandlerFunc
}

````

## 第 14 步：NewMerges

插入位置：第 13 步代码之后，紧接追加；完成稿约第 264 行。

本步声明：`func NewMerges(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Merges, other http.HandlerFunc) *Merges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>reviews</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>在此声明中的实际使用：<code>reviews: reviews</code> |
| <code>usecase</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>otherOperations: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Merges</code> | 实际返回表达式：<code>&amp;Merges{identity: identity, requests: requests, reviews: reviews, usecase: usecase, otherOperations: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Merges{identity: identity, requests: requests, reviews: reviews, usecase: usecase, otherOperations: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7481:7740 -->
````go
func NewMerges(identity *Identity, requests *Transactions, reviews *logic.Reviews, usecase *logic.Merges, other http.HandlerFunc) *Merges {
	return &Merges{identity: identity, requests: requests, reviews: reviews, usecase: usecase, otherOperations: other}
}

````

## 第 15 步：Merges.Approve

插入位置：第 14 步代码之后，紧接追加；完成稿约第 268 行。

本步声明：`func (h *Merges) Approve(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PreparedMutation(w, r, actor, input.OperationId, &quot;MERGE&quot;, input, 202, initial, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, true, &quot;MERGE&quot;, pointer.Document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7740:8621 -->
````go
func (h *Merges) Approve(w http.ResponseWriter, r *http.Request) {
	var input dto.ApproveMerge
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["requestId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, err := h.reviews.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "MERGE", pointer.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, pointer, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": input.OperationId, "state": "PREPARING", "poll_url": "/v1/merge-operations/" + input.OperationId}
	h.requests.PreparedMutation(w, r, actor, input.OperationId, "MERGE", input, 202, initial, run, present)
}

````

## 第 16 步：Merges.Progress

插入位置：第 15 步代码之后，紧接追加；完成稿约第 297 行。

本步声明：`func (h *Merges) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.otherOperations(w, r)</code>；<code>httpx.Fail(w, 404, &quot;MERGE_NOT_FOUND&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>r.URL</code>；<code>h.otherOperations(w, r)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.otherOperations != nil &amp;&amp; r.URL.Path == &quot;/v1/operations/&quot;+operation</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == sql.ErrNoRows</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8621:9544 -->
````go
func (h *Merges) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	fact, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.otherOperations != nil && r.URL.Path == "/v1/operations/"+operation {
		h.otherOperations(w, r)
		return
	}
	if err == sql.ErrNoRows {
		httpx.Fail(w, 404, "MERGE_NOT_FOUND")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "MERGE_RESULT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "MERGE_RESULT", run)
}

// pull_main.go：保留原实现，按业务合并文件。

````

## 第 17 步：PullMain

插入位置：第 16 步代码之后，紧接追加；完成稿约第 331 行。

本步声明：`type PullMain struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。 |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 17 bytes=9544:9667 -->
````go
type PullMain struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.PullMain
	other    http.HandlerFunc
}

````

## 第 18 步：NewPullMain

插入位置：第 17 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`func NewPullMain(identity *Identity, requests *Transactions, usecase *logic.PullMain, other http.HandlerFunc) *PullMain`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>other: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PullMain</code> | 实际返回表达式：<code>&amp;PullMain{identity: identity, requests: requests, usecase: usecase, other: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PullMain{identity: identity, requests: requests, usecase: usecase, other: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9667:9881 -->
````go
func NewPullMain(identity *Identity, requests *Transactions, usecase *logic.PullMain, other http.HandlerFunc) *PullMain {
	return &PullMain{identity: identity, requests: requests, usecase: usecase, other: other}
}
````

## 第 19 步：PullMain.Create

插入位置：第 18 步代码之后，紧接追加；完成稿约第 341 行。

本步声明：`func (h *PullMain) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PreparedMutation(w, r, actor, input.OperationId, &quot;PULL_MAIN&quot;, input, 202, map&#91;string&#93;any{&quot;operation_id&quot;: input.OperationId, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/operat …</code>（长表达式，按本步完整代码继续） |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;PULL_MAIN&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=9881:10755 -->
````go
func (h *PullMain) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.PullMain
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
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
	actor, err := h.identity.Actor(r, true, "PULL_MAIN", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, unit, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PreparedMutation(w, r, actor, input.OperationId, "PULL_MAIN", input, 202, map[string]any{"operation_id": input.OperationId, "state": "PREPARING", "poll_url": "/v1/operations/" + input.OperationId}, run, present)
}
````

## 第 20 步：PullMain.Progress

插入位置：第 19 步代码之后，紧接追加；完成稿约第 368 行。

本步声明：`func (h *PullMain) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Document, &quot;PULL_MAIN_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.other(w, r)</code>；<code>h.identity.Actor(r, false, &quot;PULL_MAIN_RESULT&quot;, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10755:11390 -->
````go
func (h *PullMain) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "PULL_MAIN_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "PULL_MAIN_RESULT", run)
}
````

## 第 21 步：PullMain.Source

插入位置：第 20 步代码之后，紧接追加；完成稿约第 391 行。

本步声明：`func (h *PullMain) Source(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_SOURCE_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(raw, input) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=11390:11802 -->
````go
func (h *PullMain) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(raw, input) != nil {
		httpx.Fail(w, 400, "DRAFT_SOURCE_INVALID")
		return
	}
	value, err := h.usecase.DraftSource(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}
````

## 第 22 步：PullMain.Committed

插入位置：第 21 步代码之后，紧接追加；完成稿约第 408 行。

本步声明：`func (h *PullMain) Committed(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_COMMIT_QUERY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(raw, query) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=11802:12291 -->
````go
func (h *PullMain) Committed(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if protojson.Unmarshal(raw, query) != nil {
		httpx.Fail(w, 400, "DRAFT_COMMIT_QUERY_INVALID")
		return
	}
	value, err := h.usecase.CommittedSource(r.Context(), query)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// reverse_changes.go：保留原实现，按业务合并文件。

````

## 第 23 步：ReverseChanges

插入位置：第 22 步代码之后，紧接追加；完成稿约第 428 行。

本步声明：`type ReverseChanges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。 |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 23 bytes=12291:12426 -->
````go
type ReverseChanges struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.ReverseChanges
	other    http.HandlerFunc
}

````

## 第 24 步：NewReverseChanges

插入位置：第 23 步代码之后，紧接追加；完成稿约第 435 行。

本步声明：`func NewReverseChanges(identity *Identity, requests *Transactions, usecase *logic.ReverseChanges, other http.HandlerFunc) *ReverseChanges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>other: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ReverseChanges</code> | 实际返回表达式：<code>&amp;ReverseChanges{identity: identity, requests: requests, usecase: usecase, other: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ReverseChanges{identity: identity, requests: requests, usecase: usecase, other: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=12426:12665 -->
````go
func NewReverseChanges(identity *Identity, requests *Transactions, usecase *logic.ReverseChanges, other http.HandlerFunc) *ReverseChanges {
	return &ReverseChanges{identity: identity, requests: requests, usecase: usecase, other: other}
}

````

## 第 25 步：ReverseChanges.Create

插入位置：第 24 步代码之后，紧接追加；完成稿约第 439 行。

本步声明：`func (h *ReverseChanges) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadAccepted(w, r, actor, sources, document, &quot;REVERSE_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;REVERSE&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
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

<!-- write-step: 25 bytes=12665:13507 -->
````go
func (h *ReverseChanges) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ReverseChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseReverseChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "REVERSE", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, document, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadAccepted(w, r, actor, sources, document, "REVERSE_RESULT", run)
}

````

## 第 26 步：ReverseChanges.Progress

插入位置：第 25 步代码之后，紧接追加；完成稿约第 472 行。

本步声明：`func (h *ReverseChanges) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;REVERSE_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.other(w, r)</code>；<code>h.identity.Actor(r, false, &quot;REVERSE_RESULT&quot;, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=13507:14146 -->
````go
func (h *ReverseChanges) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	document, err := h.usecase.OperationDocument(r.Context(), operation)
	if err == sql.ErrNoRows && h.other != nil {
		h.other(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "REVERSE_RESULT", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "REVERSE_RESULT", run)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/merge_review_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
