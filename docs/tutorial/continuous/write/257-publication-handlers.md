# 分步编写 `services/document/internal/handler/document/publication_handlers.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/257-publication-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供当前正式文档、发布历史、创建/切换/撤销发布及可见性修改入口。

当前公开阅读允许真正匿名，发布管理与历史仍走已认证用户与固定意图。

写之前先按这条执行路线理解各段的连接：

1. PublicReading.Current取OptionalActor并绑定输出
2. Publications.Create/Change解析不同意图
3. Visibilities.Change校验政策并调用撤权投影
4. KnowledgeBases.Delete登记关闭。

本文件需要守住的边界：

- 有畸形Cookie不能降级匿名
- 改变可见性不等于创建正式版
- 公开与历史不能共用宽松鉴权。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/publication_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/publication_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/publication_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/publication_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:510 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"database/sql"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// public_reading.go：保留原实现，按业务合并文件。

````

## 第 2 步：PublicReading

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type PublicReading struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |
| <code>usecase</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。 |

<!-- write-step: 2 bytes=510:635 -->
````go
type PublicReading struct {
	identity *Identity
	requests *Transactions
	outputs  *Outputs
	usecase  *logic.PublicReading
}

````

## 第 3 步：NewPublicReading

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func NewPublicReading(identity *Identity, requests *Transactions, outputs *Outputs, usecase *logic.PublicReading) *PublicReading`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>usecase</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PublicReading</code> | 实际返回表达式：<code>&amp;PublicReading{identity: identity, requests: requests, outputs: outputs, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PublicReading{identity: identity, requests: requests, outputs: outputs, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=635:868 -->
````go
func NewPublicReading(identity *Identity, requests *Transactions, outputs *Outputs, usecase *logic.PublicReading) *PublicReading {
	return &PublicReading{identity: identity, requests: requests, outputs: outputs, usecase: usecase}
}

````

## 第 4 步：PublicReading.Current

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (h *PublicReading) Current(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.outputs.Bind(w, registration, sources)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.OptionalActor(r, &quot;READ_PUBLIC_DOCUMENT&quot;, id)</code>；<code>r.Context</code>；<code>r.Header</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。

<!-- write-step: 4 bytes=868:1716 -->
````go
func (h *PublicReading) Current(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_PUBLIC_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	// URL中的旧release提示不参与选择；用例只取当前正式指针。
	value, err := h.usecase.Current(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.SendBounded(r.Context(), http.StatusOK, value, 32<<20); err != nil {
		httpx.Failure(w, err)
	}
}

````

## 第 5 步：PublicReading.History

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func (h *PublicReading) History(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_RELEASE_HISTORY&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_RELEASE_HISTORY&quot;, id)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1716:2277 -->
````go
func (h *PublicReading) History(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RELEASE_HISTORY", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.History(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_RELEASE_HISTORY", run)
}

````

## 第 6 步：PublicReading.Historical

插入位置：第 5 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func (h *PublicReading) Historical(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadBounded(w, r, actor, sources, id, &quot;READ_RELEASE_HISTORY&quot;, run, 32&lt;&lt;20)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_RELEASE_HISTORY&quot;, id)</code>；<code>r.Context</code>；<code>h.requests.ReadBounded(w, r, actor, sources, id, &quot;READ_RELEASE_HISTORY&quot;, run, 32&lt;&lt;20)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2277:2997 -->
````go
func (h *PublicReading) Historical(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	id, err := domain.CanonicalID(vars["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	release, err := domain.CanonicalID(vars["releaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_RELEASE_HISTORY", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Historical(r.Context(), actor, id, release)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadBounded(w, r, actor, sources, id, "READ_RELEASE_HISTORY", run, 32<<20)
}

// publication.go：保留原实现，按业务合并文件。

````

## 第 7 步：Publications

插入位置：第 6 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`type Publications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。 |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 7 bytes=2997:3156 -->
````go
type Publications struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.Publications
	otherOperations http.HandlerFunc
}

````

## 第 8 步：NewPublications

插入位置：第 7 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`func NewPublications(identity *Identity, requests *Transactions, usecase *logic.Publications, other http.HandlerFunc) *Publications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>otherOperations: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Publications</code> | 实际返回表达式：<code>&amp;Publications{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Publications{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3156:3397 -->
````go
func NewPublications(identity *Identity, requests *Transactions, usecase *logic.Publications, other http.HandlerFunc) *Publications {
	return &Publications{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

````

## 第 9 步：Publications.Create

插入位置：第 8 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func (h *Publications) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3397:3684 -->
````go
func (h *Publications) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.ReleaseCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseReleaseCreate(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

````

## 第 10 步：Publications.Change

插入位置：第 9 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`func (h *Publications) Change(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3684:3967 -->
````go
func (h *Publications) Change(w http.ResponseWriter, r *http.Request) {
	var input dto.Publication
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParsePublication(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

````

## 第 11 步：Publications.change

插入位置：第 10 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func (h *Publications) change(w http.ResponseWriter, r *http.Request, intent domain.PublicationIntent, input any)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;PUBLICATION&quot;, document)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.PublicationIntent</code> | 使用 <code>domain.PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>在此声明中的实际使用：<code>h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)</code>；<code>intent.Operation</code>；<code>intent.Kind</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)</code>；<code>h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3967:4724 -->
````go
func (h *Publications) change(w http.ResponseWriter, r *http.Request, intent domain.PublicationIntent, input any) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PUBLICATION", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	initial := map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}
	h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)
}

````

## 第 12 步：Publications.Progress

插入位置：第 11 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func (h *Publications) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.otherOperations(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Document, &quot;PUBLICATION_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.otherOperations(w, r)</code>；<code>h.identity.Actor(r, false, &quot;PUBLICATION_RESULT&quot;, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.otherOperations != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4724:5451 -->
````go
func (h *Publications) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	fixed, err := h.usecase.Lookup(r.Context(), operation)
	if err == sql.ErrNoRows && h.otherOperations != nil {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "PUBLICATION_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "PUBLICATION_RESULT", run)
}

// visibility.go：保留原实现，按业务合并文件。

````

## 第 13 步：Visibilities

插入位置：第 12 步代码之后，紧接追加；完成稿约第 183 行。

本步声明：`type Visibilities struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Visibilities</code> | 使用 <code>logic.Visibilities</code> 类型；其定义处的职责：协调可见政策、撤权关闭任务与搜索索引事件的一次业务提交。 |
| <code>authorizations</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |

<!-- write-step: 13 bytes=5451:5611 -->
````go
type Visibilities struct {
	identity       *Identity
	requests       *Transactions
	usecase        *logic.Visibilities
	authorizations *logic.Authorizations
}

````

## 第 14 步：NewVisibilities

插入位置：第 13 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func NewVisibilities(identity *Identity, requests *Transactions, usecase *logic.Visibilities, authorizations *logic.Authorizations) *Visibilities`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Visibilities</code> | 使用 <code>logic.Visibilities</code> 类型；其定义处的职责：协调可见政策、撤权关闭任务与搜索索引事件的一次业务提交。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>authorizations</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>authorizations: authorizations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Visibilities</code> | 实际返回表达式：<code>&amp;Visibilities{identity: identity, requests: requests, usecase: usecase, authorizations: authorizations}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Visibilities{identity: identity, requests: requests, usecase: usecase, authorizations: authorizations}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5611:5873 -->
````go
func NewVisibilities(identity *Identity, requests *Transactions, usecase *logic.Visibilities, authorizations *logic.Authorizations) *Visibilities {
	return &Visibilities{identity: identity, requests: requests, usecase: usecase, authorizations: authorizations}
}
````

## 第 15 步：Visibilities.Change

插入位置：第 14 步代码之后，紧接追加；完成稿约第 193 行。

本步声明：`func (h *Visibilities) Change(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Fail(w, 400, &quot;VISIBILITY_POLICY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;CHANGE_VISIBILITY&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidVisibility(input.Policy)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>isKnowledgeBase</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor))</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5873:6987 -->
````go
func (h *Visibilities) Change(w http.ResponseWriter, r *http.Request) {
	var input dto.VisibilityChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidVisibility(input.Policy) {
		httpx.Fail(w, 400, "VISIBILITY_POLICY_INVALID")
		return
	}
	isKnowledgeBase := pathvar.Vars(r)["knowledgeBaseId"] != ""
	key, kind := "documentId", "DOCUMENT_VISIBILITY"
	if isKnowledgeBase {
		key = "knowledgeBaseId"
		kind = "KB_VISIBILITY"
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)[key])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "CHANGE_VISIBILITY", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.LookupReceiptFirst, h.authorizations.Presentation(actor)) {
		return
	}
	actor, run, err := h.usecase.Prepare(r.Context(), actor, id, isKnowledgeBase, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, 202, run, h.authorizations.VisibilityPresentation(actor, input.OperationId))
}
````

## 第 16 步：KnowledgeBases.Delete

插入位置：第 15 步代码之后，紧接追加；完成稿约第 228 行。

本步声明：`func (h *KnowledgeBases) Delete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;KB_DELETE&quot;, input, 202, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;DELETE_KB&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6987:7720 -->
````go
func (h *KnowledgeBases) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DELETE_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_DELETE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Delete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_DELETE", input, 202, run, nil)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/publication_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
