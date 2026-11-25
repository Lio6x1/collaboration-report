# 分步编写 `services/document/internal/handler/document/structure_handlers.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/263-structure-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

暴露主线锁、样式、结构预览/确认和私有MAIN来源接口。

普通用户动作与内部捕获复用业务事实但身份入口不同，handler必须按purpose路由。

写之前先按这条执行路线理解各段的连接：

1. MainControls.change提交固定意图
2. StructurePreviews.Create登记后读取预览
3. MainSources.MainSource分别调用读取、发布或控制来源
4. ConfirmationSource只接受固定确认查询。

本文件需要守住的边界：

- 通用进度不应扩展未知DTO字段泄漏私有cut
- 不能把READ_MAIN准入拿去执行管理控制。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/structure_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/structure_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/structure_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/structure_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:601 -->
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
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
)

// main_controls.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControls

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type MainControls struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。 |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 2 bytes=601:760 -->
````go
type MainControls struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.MainControls
	otherOperations http.HandlerFunc
}

````

## 第 3 步：NewMainControls

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func NewMainControls(identity *Identity, requests *Transactions, usecase *logic.MainControls, other http.HandlerFunc) *MainControls`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>otherOperations: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*MainControls</code> | 实际返回表达式：<code>&amp;MainControls{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;MainControls{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=760:1001 -->
````go
func NewMainControls(identity *Identity, requests *Transactions, usecase *logic.MainControls, other http.HandlerFunc) *MainControls {
	return &MainControls{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

````

## 第 4 步：MainControls.Lock

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func (h *MainControls) Lock(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1001:1276 -->
````go
func (h *MainControls) Lock(w http.ResponseWriter, r *http.Request) {
	var input dto.MainLock
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseMainLock(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

````

## 第 5 步：MainControls.Style

插入位置：第 4 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (h *MainControls) Style(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1276:1558 -->
````go
func (h *MainControls) Style(w http.ResponseWriter, r *http.Request) {
	var input dto.StyleChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStyleChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

````

## 第 6 步：MainControls.change

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (h *MainControls) change(w http.ResponseWriter, r *http.Request, intent domain.MainControlIntent, input any)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;MAIN_CONTROL&quot;, document)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.MainControlIntent</code> | 使用 <code>domain.MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>在此声明中的实际使用：<code>h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)</code>；<code>intent.Operation</code>；<code>intent.Kind</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>h.usecase.Prepare(r.Context(), actor, document, intent, input, r.URL.Path)</code>；<code>h.requests.PreparedMutation(w, r, actor, intent.Operation, intent.Kind, input, 202, initial, run, present)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1558:2316 -->
````go
func (h *MainControls) change(w http.ResponseWriter, r *http.Request, intent domain.MainControlIntent, input any) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "MAIN_CONTROL", document)
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

## 第 7 步：MainControls.Progress

插入位置：第 6 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func (h *MainControls) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.otherOperations(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Document, &quot;MAIN_CONTROL_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.otherOperations(w, r)</code>；<code>h.identity.Actor(r, false, &quot;MAIN_CONTROL_RESULT&quot;, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.otherOperations != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2316:3047 -->
````go
func (h *MainControls) Progress(w http.ResponseWriter, r *http.Request) {
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
	actor, err := h.identity.Actor(r, false, "MAIN_CONTROL_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "MAIN_CONTROL_RESULT", run)
}

// main_sources.go：保留原实现，按业务合并文件。

````

## 第 8 步：mainSourceInput

插入位置：第 7 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func mainSourceInput(w http.ResponseWriter, r *http.Request) json.RawMessage`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;raw)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>raw</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;raw)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return raw</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3047:3331 -->
````go
func mainSourceInput(w http.ResponseWriter, r *http.Request) json.RawMessage {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return nil
	}
	var raw json.RawMessage
	if !httpx.DecodeBody(w, r, &raw) {
		return nil
	}
	return raw
}

````

## 第 9 步：MainSources

插入位置：第 8 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`type MainSources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>merges</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。 |
| <code>controls</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。 |
| <code>readViews</code> | <code>*logic.MainReadViews</code> | 使用 <code>logic.MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。 |
| <code>publications</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。 |

<!-- write-step: 9 bytes=3331:3491 -->
````go
type MainSources struct {
	merges       *logic.Merges
	controls     *logic.MainControls
	readViews    *logic.MainReadViews
	publications *logic.Publications
}

````

## 第 10 步：NewMainSources

插入位置：第 9 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`func NewMainSources(merges *logic.Merges, controls *logic.MainControls, readViews *logic.MainReadViews, publications *logic.Publications) *MainSources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>merges</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。<br>在此声明中的实际使用：<code>merges: merges</code> |
| <code>controls</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。<br>在此声明中的实际使用：<code>controls: controls</code> |
| <code>readViews</code> | <code>*logic.MainReadViews</code> | 使用 <code>logic.MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。<br>在此声明中的实际使用：<code>readViews: readViews</code> |
| <code>publications</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。<br>在此声明中的实际使用：<code>publications: publications</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*MainSources</code> | 实际返回表达式：<code>&amp;MainSources{merges: merges, controls: controls, readViews: readViews, publications: publications}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;MainSources{merges: merges, controls: controls, readViews: readViews, publications: publications}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3491:3754 -->
````go
func NewMainSources(merges *logic.Merges, controls *logic.MainControls, readViews *logic.MainReadViews, publications *logic.Publications) *MainSources {
	return &MainSources{merges: merges, controls: controls, readViews: readViews, publications: publications}
}

````

## 第 11 步：MainSources.MainSource

插入位置：第 10 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`func (h *MainSources) MainSource(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3754:4603 -->
````go
func (h *MainSources) MainSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.CaptureMainRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "MAIN_SOURCE_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	switch input.Context.GetTarget().GetPurpose() {
	case "MAIN_CONTROL":
		value, err = h.controls.MainSource(r.Context(), input)
	case "READ_MAIN":
		value, err = h.readViews.MainSource(r.Context(), input)
	case "PUBLICATION":
		value, err = h.publications.MainSource(r.Context(), input)
	case "MERGE":
		value, err = h.merges.MainSource(r.Context(), input)
	default:
		httpx.Fail(w, 400, "MAIN_SOURCE_QUERY_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

````

## 第 12 步：MainSources.CommittedSource

插入位置：第 11 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`func (h *MainSources) CommittedSource(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;MAIN_COMMIT_QUERY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>query.GetTarget().GetPurpose() == &quot;MAIN_CONTROL&quot;</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4603:5214 -->
````go
func (h *MainSources) CommittedSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if err := protojson.Unmarshal(raw, query); err != nil {
		httpx.Fail(w, 400, "MAIN_COMMIT_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	if query.GetTarget().GetPurpose() == "MAIN_CONTROL" {
		value, err = h.controls.CommittedSource(r.Context(), query)
	} else {
		value, err = h.merges.CommittedSource(r.Context(), query)
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

````

## 第 13 步：MainSources.ConfirmationSource

插入位置：第 12 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func (h *MainSources) ConfirmationSource(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;STRUCTURE_CONFIRMATION_QUERY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5214:5749 -->
````go
func (h *MainSources) ConfirmationSource(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.PrepareMainCommitRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "STRUCTURE_CONFIRMATION_QUERY_INVALID")
		return
	}
	value, err := h.controls.ConfirmationSource(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// manifest_read.go：保留原实现，按业务合并文件。

````

## 第 14 步：Readings.Manifest

插入位置：第 13 步代码之后，紧接追加；完成稿约第 202 行。

本步声明：`func (h *Readings) Manifest(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;READ_MANIFEST&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_MANIFEST&quot;, document)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;READ_MANIFEST&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5749:6456 -->
````go
func (h *Readings) Manifest(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	document, err := domain.CanonicalID(vars["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	manifest, err := domain.CanonicalID(vars["manifestId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MANIFEST", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Manifest(r.Context(), actor, document, manifest)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, document, "READ_MANIFEST", run)
}

// structure.go：保留原实现，按业务合并文件。

````

## 第 15 步：MainControls.Structure

插入位置：第 14 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func (h *MainControls) Structure(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6456:6821 -->
````go
func (h *MainControls) Structure(w http.ResponseWriter, r *http.Request) {
	var input dto.StructureChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStructureChange(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

// structure_previews.go：保留原实现，按业务合并文件。

````

## 第 16 步：StructurePreviews

插入位置：第 15 步代码之后，紧接追加；完成稿约第 244 行。

本步声明：`type StructurePreviews struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。 |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 16 bytes=6821:6962 -->
````go
type StructurePreviews struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.StructurePreviews
	other    http.HandlerFunc
}

````

## 第 17 步：NewStructurePreviews

插入位置：第 16 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func NewStructurePreviews(identity *Identity, requests *Transactions, usecase *logic.StructurePreviews, other http.HandlerFunc) *StructurePreviews`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>other: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*StructurePreviews</code> | 实际返回表达式：<code>&amp;StructurePreviews{identity: identity, requests: requests, usecase: usecase, other: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;StructurePreviews{identity: identity, requests: requests, usecase: usecase, other: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6962:7213 -->
````go
func NewStructurePreviews(identity *Identity, requests *Transactions, usecase *logic.StructurePreviews, other http.HandlerFunc) *StructurePreviews {
	return &StructurePreviews{identity: identity, requests: requests, usecase: usecase, other: other}
}

````

## 第 18 步：StructurePreviews.Progress

插入位置：第 17 步代码之后，紧接追加；完成稿约第 255 行。

本步声明：`func (h *StructurePreviews) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.other(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, fixed.Document, &quot;STRUCTURE_PREVIEW_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.other(w, r)</code>；<code>h.identity.Actor(r, false, &quot;STRUCTURE_PREVIEW_RESULT&quot;, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows &amp;&amp; h.other != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7213:7874 -->
````go
func (h *StructurePreviews) Progress(w http.ResponseWriter, r *http.Request) {
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
	actor, err := h.identity.Actor(r, false, "STRUCTURE_PREVIEW_RESULT", fixed.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Progress(r.Context(), actor, operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, fixed.Document, "STRUCTURE_PREVIEW_RESULT", run)
}

````

## 第 19 步：StructurePreviews.Create

插入位置：第 18 步代码之后，紧接追加；完成稿约第 279 行。

本步声明：`func (h *StructurePreviews) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, document, &quot;STRUCTURE_PREVIEW_RESULT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;STRUCTURE_PREVIEW&quot;, document)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

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

<!-- write-step: 19 bytes=7874:8966 -->
````go
func (h *StructurePreviews) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.StructurePreviewCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseStructurePreview(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "STRUCTURE_PREVIEW", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Register(r.Context(), actor, document, intent, r.URL.Path)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = h.usecase.Wait(r.Context(), intent.Operation); err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Read(r.Context(), actor, document, intent.Operation)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 写意图先持久化，RPC/等待均在SQL锁外；最终200由当前授权和真实输出租约保护。
	h.requests.Read(w, r, actor, sources, document, "STRUCTURE_PREVIEW_RESULT", run)
}

````

## 第 20 步：StructurePreviews.Source

插入位置：第 19 步代码之后，紧接追加；完成稿约第 317 行。

本步声明：`func (h *StructurePreviews) Source(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;STRUCTURE_PREVIEW_QUERY_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, value)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8966:9405 -->
````go
func (h *StructurePreviews) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if err := protojson.Unmarshal(raw, input); err != nil {
		httpx.Fail(w, 400, "STRUCTURE_PREVIEW_QUERY_INVALID")
		return
	}
	value, err := h.usecase.Source(r.Context(), input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/structure_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
