# 分步编写 `services/document/internal/handler/document/exports.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/253-exports.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

暴露导出创建、进度和附件下载HTTP接口。

PUBLIC导出可能匿名，但带会话Cookie时仍是用户变更，需当前Origin与CSRF语义。

写之前先按这条执行路线理解各段的连接：

1. Create核Origin并用OptionalMutationActor
2. ParseExportInput校验union
3. Job/Download用当前可选身份
4. 下载交给受控SendExport。

本文件需要守住的边界：

- 不能以PUBLIC为由跳过有Cookie用户的CSRF
- job ID本身不是下载授权。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/exports.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/exports.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/exports.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/exports.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:347 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

````

## 第 2 步：Exports

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type Exports struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |
| <code>usecase</code> | <code>*logic.Exports</code> | 使用 <code>logic.Exports</code> 类型；其定义处的职责：Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，导出创建等浏览器动作检查来源，避免任意外站代用户触发。 |

<!-- write-step: 2 bytes=347:453 -->
````go
type Exports struct {
	identity *Identity
	outputs  *Outputs
	usecase  *logic.Exports
	origin   string
}

````

## 第 3 步：NewExports

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func NewExports(identity *Identity, outputs *Outputs, usecase *logic.Exports, origin string) *Exports`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>usecase</code> | <code>*logic.Exports</code> | 使用 <code>logic.Exports</code> 类型；其定义处的职责：Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Exports</code> | 实际返回表达式：<code>&amp;Exports{identity: identity, outputs: outputs, usecase: usecase, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Exports{identity: identity, outputs: outputs, usecase: usecase, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=453:649 -->
````go
func NewExports(identity *Identity, outputs *Outputs, usecase *logic.Exports, origin string) *Exports {
	return &Exports{identity: identity, outputs: outputs, usecase: usecase, origin: origin}
}

````

## 第 4 步：Exports.Create

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (h *Exports) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.DecodeBody(w, r, &amp;body)</code>；<code>httpx.Fail(w, 403, &quot;ORIGIN_REQUIRED&quot;)</code>；<code>h.outputs.Bind(w, registration, sources)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>httpx.DecodeBody(w, r, &amp;body)</code>；<code>r.Header</code>；<code>h.identity.OptionalMutationActor(r, &quot;CREATE_EXPORT&quot;, id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;body)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>r.Header.Get(&quot;Origin&quot;) != h.origin</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。

<!-- write-step: 4 bytes=649:1824 -->
````go
func (h *Exports) Create(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var body json.RawMessage
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	input, err := domain.ParseExportInput(dto.ExportCreate(body))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 匿名POST也来自同源产品入口；存在会话Cookie时不能回退匿名绕过CSRF。
	if r.Header.Get("Origin") != h.origin {
		httpx.Fail(w, 403, "ORIGIN_REQUIRED")
		return
	}
	actor, err := h.identity.OptionalMutationActor(r, "CREATE_EXPORT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	value, err := h.usecase.Create(r.Context(), actor, id, input, dto.ExportCreate(body), r.URL.Path, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.Send(r.Context(), http.StatusAccepted, value); err != nil {
		httpx.Failure(w, err)
	}
}

````

## 第 5 步：Exports.Job

插入位置：第 4 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func (h *Exports) Job(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.outputs.Bind(w, registration, sources)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.OptionalActor(r, &quot;READ_EXPORT&quot;, 0)</code>；<code>r.Context</code>；<code>r.Header</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。

<!-- write-step: 5 bytes=1824:2553 -->
````go
func (h *Exports) Job(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["exportJobId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_EXPORT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	value, err := h.usecase.Job(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err = output.Send(r.Context(), http.StatusOK, value); err != nil {
		httpx.Failure(w, err)
	}
}

````

## 第 6 步：Exports.Download

插入位置：第 5 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func (h *Exports) Download(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.outputs.Bind(w, registration, sources)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.OptionalActor(r, &quot;DOWNLOAD_EXPORT&quot;, 0)</code>；<code>r.Context</code>；<code>r.Header</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &amp;&amp; !started</code>。

<!-- write-step: 6 bytes=2553:3338 -->
````go
func (h *Exports) Download(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["exportJobId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "DOWNLOAD_EXPORT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	file, name, err := h.usecase.Download(r.Context(), actor, id, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	defer file.Close()
	started, err := output.SendExport(r.Context(), file, name)
	if err != nil && !started {
		httpx.Failure(w, err)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/exports.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
