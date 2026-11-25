# 分步编写 `services/document/internal/handler/document/read.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/258-read.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供详情、主线、修订摘要和单元正文的HTTP读取入口。

固定阅读的传输适配集中于handler，业务层只返回读取闭包和来源，不持有ResponseWriter。

写之前先按这条执行路线理解各段的连接：

1. Document/Main/Revision解析documentId并认证
2. Unit追加unitId和view绑定
3. 通过Transactions把业务读取结果变成已登记的受控响应。

本文件需要守住的边界：

- 不能跳过usecase直接按URL版本号查询正文
- 整篇结果须遵守传输预算。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/read.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/read.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/read.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/read.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:267 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

````

## 第 2 步：Readings

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type Readings struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Readings</code> | 使用 <code>logic.Readings</code> 类型；其定义处的职责：协调文档详情、MAIN全文、变化摘要和固定视图单元读取。 |

<!-- write-step: 2 bytes=267:363 -->
````go
type Readings struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Readings
}

````

## 第 3 步：NewReadings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func NewReadings(identity *Identity, requests *Transactions, usecase *logic.Readings) *Readings`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Readings</code> | 使用 <code>logic.Readings</code> 类型；其定义处的职责：协调文档详情、MAIN全文、变化摘要和固定视图单元读取。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Readings</code> | 实际返回表达式：<code>&amp;Readings{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Readings{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=363:539 -->
````go
func NewReadings(identity *Identity, requests *Transactions, usecase *logic.Readings) *Readings {
	return &Readings{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 4 步：Readings.Document

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (h *Readings) Document(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadProjected(w, r, actor, sources, id, &quot;READ_DOCUMENT&quot;, run, func(value any) any { 		reading := value.(logic.DocumentReading) 		actions, _ := json.Marshal(reading.Actio …</code>（长表达式，按本步完整代码继续）；<code>w.Header</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_DOCUMENT&quot;, id)</code>；<code>r.Context</code>；<code>h.requests.ReadProjected(w, r, actor, sources, id, &quot;READ_DOCUMENT&quot;, run, func(value any) any { 		reading := value.(logic.DocumentReading) 		actions, _ := json.Marshal(reading.Actio …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return reading.Document</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=539:1232 -->
````go
func (h *Readings) Document(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Document(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadProjected(w, r, actor, sources, id, "READ_DOCUMENT", run, func(value any) any {
		reading := value.(logic.DocumentReading)
		actions, _ := json.Marshal(reading.Actions)
		w.Header().Set("X-Available-Actions", string(actions))
		return reading.Document
	})
}
````

## 第 5 步：Readings.Main

插入位置：第 4 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (h *Readings) Main(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_MAIN&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_MAIN&quot;, id)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_MAIN&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1232:1703 -->
````go
func (h *Readings) Main(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MAIN", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Main(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_MAIN", run)
}

````

## 第 6 步：Readings.Revision

插入位置：第 5 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (h *Readings) Revision(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_MAIN&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_MAIN&quot;, id)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_MAIN&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1703:2181 -->
````go
func (h *Readings) Revision(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_MAIN", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Revision(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_MAIN", run)
}
````

## 第 7 步：Readings.Unit

插入位置：第 6 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`func (h *Readings) Unit(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;READ_VIEW_INVALID&quot;)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_FIXED_UNIT&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_FIXED_UNIT&quot;, id)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;READ_FIXED_UNIT&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidOperationID(view)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2181:2914 -->
````go
func (h *Readings) Unit(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	view := pathvar.Vars(r)["viewId"]
	if !domain.ValidOperationID(view) {
		httpx.Fail(w, 400, "READ_VIEW_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_FIXED_UNIT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Unit(r.Context(), actor, id, unit, view)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "READ_FIXED_UNIT", run)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/read.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
