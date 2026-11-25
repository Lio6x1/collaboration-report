# 分步编写 `services/document/internal/handler/document/reviews_handlers.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/261-reviews-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供送审、请求详情、轮次历史、撤回和修改意见动作的HTTP入口。

接口要明确针对哪个固定request/round，不能让路径请求与DTO中的预期轮次脱节。

写之前先按这条执行路线理解各段的连接：

1. target从请求ID找到真实document/unit后认证
2. Create用ParseSubmission校验union
3. RequestChanges/Withdraw/Resolve生成动作闭包
4. History传递kind和分页游标。

本文件需要守住的边界：

- 送审202只说明操作已登记
- 轮次正文与历史响应仍通过受控输出，不可直接WriteJSON。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/reviews_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/reviews_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/reviews_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/reviews_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:487 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"database/sql"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// review_readings.go：保留原实现，按业务合并文件。

````

## 第 2 步：Reviews.Request

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func (h *Reviews) Request(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.target(w, r, false)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.target(w, r, false)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=487:824 -->
````go
func (h *Reviews) Request(w http.ResponseWriter, r *http.Request) {
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.Request(r.Context(), actor, pointer)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW", run)
}

````

## 第 3 步：Reviews.Round

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (h *Reviews) Round(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.target(w, r, false)</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW_ROUND&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.target(w, r, false)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW_ROUND&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=824:1285 -->
````go
func (h *Reviews) Round(w http.ResponseWriter, r *http.Request) {
	round, err := domain.CanonicalID(pathvar.Vars(r)["roundId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.Round(r.Context(), actor, pointer, round)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW_ROUND", run)
}

````

## 第 4 步：Reviews.Rounds

插入位置：第 3 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func (h *Reviews) Rounds(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;rounds&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;rounds&quot;)</code> |

<!-- write-step: 4 bytes=1285:1381 -->
````go
func (h *Reviews) Rounds(w http.ResponseWriter, r *http.Request)  { h.history(w, r, "rounds") }
````

## 第 5 步：Reviews.Changes

插入位置：第 4 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (h *Reviews) Changes(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;changes&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;changes&quot;)</code> |

<!-- write-step: 5 bytes=1381:1478 -->
````go
func (h *Reviews) Changes(w http.ResponseWriter, r *http.Request) { h.history(w, r, "changes") }
````

## 第 6 步：Reviews.Actions

插入位置：第 5 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func (h *Reviews) Actions(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;actions&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.history(w, r, &quot;actions&quot;)</code> |

<!-- write-step: 6 bytes=1478:1576 -->
````go
func (h *Reviews) Actions(w http.ResponseWriter, r *http.Request) { h.history(w, r, "actions") }

````

## 第 7 步：Reviews.history

插入位置：第 6 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (h *Reviews) history(w http.ResponseWriter, r *http.Request, kind string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.target(w, r, false)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW_HISTORY&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.target(w, r, false)</code>；<code>r.Context</code>；<code>r.URL</code>；<code>h.requests.Read(w, r, actor, sources, pointer.Document, &quot;READ_REVIEW_HISTORY&quot;, run)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>h.usecase.History(r.Context(), actor, pointer, kind, r.URL.Query().Get(&quot;limit&quot;), r.URL.Query().Get(&quot;cursor&quot;))</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1576:1997 -->
````go
func (h *Reviews) history(w http.ResponseWriter, r *http.Request, kind string) {
	pointer, actor, ok := h.target(w, r, false)
	if !ok {
		return
	}
	sources, run, err := h.usecase.History(r.Context(), actor, pointer, kind, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, pointer.Document, "READ_REVIEW_HISTORY", run)
}

````

## 第 8 步：Reviews.List

插入位置：第 7 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`func (h *Reviews) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;LIST_REVIEWS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;LIST_REVIEWS&quot;, document)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>text != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1997:2686 -->
````go
func (h *Reviews) List(w http.ResponseWriter, r *http.Request) {
	document := uint64(0)
	if text := pathvar.Vars(r)["documentId"]; text != "" {
		id, err := domain.CanonicalID(text)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		document = id
	}
	actor, err := h.identity.Actor(r, false, "LIST_REVIEWS", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.List(r.Context(), actor, document, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "LIST_REVIEWS", run)
}

// reviews.go：保留原实现，按业务合并文件。

````

## 第 9 步：Reviews

插入位置：第 8 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`type Reviews struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。 |

<!-- write-step: 9 bytes=2686:2780 -->
````go
type Reviews struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Reviews
}

````

## 第 10 步：NewReviews

插入位置：第 9 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func NewReviews(identity *Identity, requests *Transactions, usecase *logic.Reviews) *Reviews`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Reviews</code> | 实际返回表达式：<code>&amp;Reviews{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Reviews{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2780:2953 -->
````go
func NewReviews(identity *Identity, requests *Transactions, usecase *logic.Reviews) *Reviews {
	return &Reviews{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 11 步：Reviews.target

插入位置：第 10 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func (h *Reviews) target(w http.ResponseWriter, r *http.Request, mutation bool) (domain.ReviewPointer, logic.Actor, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, mutation, &quot;REVIEW_CONTEXT&quot;, pointer.Document)</code> |
| <code>mutation</code> | <code>bool</code> | 在此声明中的实际使用：<code>h.identity.Actor(r, mutation, &quot;REVIEW_CONTEXT&quot;, pointer.Document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>domain.ReviewPointer{}</code>；<code>pointer</code> |
| <code>(匿名/未命名)</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>logic.Actor{}</code>；<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return domain.ReviewPointer{}, logic.Actor{}, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return pointer, logic.Actor{}, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return pointer, actor, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return pointer, actor, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2953:3558 -->
````go
func (h *Reviews) target(w http.ResponseWriter, r *http.Request, mutation bool) (domain.ReviewPointer, logic.Actor, bool) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["requestId"])
	if err != nil {
		httpx.Failure(w, err)
		return domain.ReviewPointer{}, logic.Actor{}, false
	}
	pointer, err := h.usecase.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return pointer, logic.Actor{}, false
	}
	actor, err := h.identity.Actor(r, mutation, "REVIEW_CONTEXT", pointer.Document)
	if err != nil {
		httpx.Failure(w, err)
		return pointer, actor, false
	}
	return pointer, actor, true
}

````

## 第 12 步：Reviews.RequestChanges

插入位置：第 11 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`func (h *Reviews) RequestChanges(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.target(w, r, true)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;REQUEST_CHANGES&quot;, input, 200, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.target(w, r, true)</code>；<code>r.Context</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;REQUEST_CHANGES&quot;, input, 200, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3558:4032 -->
````go
func (h *Reviews) RequestChanges(w http.ResponseWriter, r *http.Request) {
	var input dto.RequestChanges
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.RequestChanges(r.Context(), actor, pointer, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "REQUEST_CHANGES", input, 200, run, present)
}

````

## 第 13 步：Reviews.Withdraw

插入位置：第 12 步代码之后，紧接追加；完成稿约第 136 行。

本步声明：`func (h *Reviews) Withdraw(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.target(w, r, true)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;REVIEW_WITHDRAW&quot;, input, 200, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.target(w, r, true)</code>；<code>r.Context</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;REVIEW_WITHDRAW&quot;, input, 200, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4032:4490 -->
````go
func (h *Reviews) Withdraw(w http.ResponseWriter, r *http.Request) {
	var input dto.Withdrawal
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.Withdraw(r.Context(), actor, pointer, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "REVIEW_WITHDRAW", input, 200, run, present)
}

````

## 第 14 步：Reviews.Resolve

插入位置：第 13 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func (h *Reviews) Resolve(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.target(w, r, true)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;RESOLVE_CHANGES&quot;, input, 200, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.target(w, r, true)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4490:5139 -->
````go
func (h *Reviews) Resolve(w http.ResponseWriter, r *http.Request) {
	var input dto.ResolveChanges
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	change, err := domain.CanonicalID(pathvar.Vars(r)["changeId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	pointer, actor, ok := h.target(w, r, true)
	if !ok {
		return
	}
	actor, run, present, err := h.usecase.Resolve(r.Context(), actor, pointer, change, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESOLVE_CHANGES", input, 200, run, present)
}

// submissions.go：保留原实现，按业务合并文件。

````

## 第 15 步：Submissions

插入位置：第 14 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`type Submissions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。 |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 15 bytes=5139:5296 -->
````go
type Submissions struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.Submissions
	otherOperations http.HandlerFunc
}

````

## 第 16 步：NewSubmissions

插入位置：第 15 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func NewSubmissions(identity *Identity, requests *Transactions, usecase *logic.Submissions, otherOperations http.HandlerFunc) *Submissions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>otherOperations: otherOperations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Submissions</code> | 实际返回表达式：<code>&amp;Submissions{identity: identity, requests: requests, usecase: usecase, otherOperations: otherOperations}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Submissions{identity: identity, requests: requests, usecase: usecase, otherOperations: otherOperations}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=5296:5553 -->
````go
func NewSubmissions(identity *Identity, requests *Transactions, usecase *logic.Submissions, otherOperations http.HandlerFunc) *Submissions {
	return &Submissions{identity: identity, requests: requests, usecase: usecase, otherOperations: otherOperations}
}

````

## 第 17 步：Submissions.Create

插入位置：第 16 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`func (h *Submissions) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.DecodeBody(w, r, &amp;raw)</code>；<code>h.requests.PreparedMutation(w, r, actor, intent.Operation, &quot;SUBMISSION&quot;, json.RawMessage(intent.Canonical), 202, initial, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>httpx.DecodeBody(w, r, &amp;raw)</code>；<code>h.identity.Actor(r, true, &quot;SUBMISSION&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;raw)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=5553:6720 -->
````go
func (h *Submissions) Create(w http.ResponseWriter, r *http.Request) {
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
	// 生成 oneOf DTO 是命名 RawMessage；先保留 JSON 字节，再交领域规则解析，避免 []byte 被解成 base64。
	var raw json.RawMessage
	if !httpx.DecodeBody(w, r, &raw) {
		return
	}
	intent, err := domain.ParseSubmission(dto.Submission(raw))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "SUBMISSION", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, unit, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}
	h.requests.PreparedMutation(w, r, actor, intent.Operation, "SUBMISSION", json.RawMessage(intent.Canonical), 202, initial, run, present)
}

````

## 第 18 步：Submissions.Progress

插入位置：第 17 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func (h *Submissions) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.otherOperations(w, r)</code>；<code>httpx.Fail(w, 404, &quot;SUBMISSION_NOT_FOUND&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>r.URL</code>；<code>h.otherOperations(w, r)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(id)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.otherOperations != nil &amp;&amp; r.URL.Path == &quot;/v1/operations/&quot;+id</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>err == sql.ErrNoRows</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6720:7578 -->
````go
func (h *Submissions) Progress(w http.ResponseWriter, r *http.Request) {
	id := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(id) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	fact, err := h.usecase.Lookup(r.Context(), id)
	if err == sql.ErrNoRows && h.otherOperations != nil && r.URL.Path == "/v1/operations/"+id {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		if err == sql.ErrNoRows {
			httpx.Fail(w, 404, "SUBMISSION_NOT_FOUND")
		} else {
			httpx.Failure(w, err)
		}
		return
	}
	actor, err := h.identity.Actor(r, false, "SUBMISSION_RESULT", fact.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, fact)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fact.Document, "SUBMISSION_RESULT", run)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/reviews_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
