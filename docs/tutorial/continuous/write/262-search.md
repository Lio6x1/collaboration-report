# 分步编写 `services/document/internal/handler/document/search.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/262-search.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将HTTP搜索参数和可选真实身份送入整页搜索用例。

匿名和登录查询绑定不同，坏Cookie不能降级匿名续旧游标。

写之前先按这条执行路线理解各段的连接：

1. SearchPages.Page使用ParseSearchQuery，OptionalActor建立身份，传入真实handler与Outputs.Bind，然后发送完整搜索页。

本文件需要守住的边界：

- 不能在handler直接输出Search候选或自行按标题拼摘要。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/search.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/search.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/search.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/search.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:333 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"net/http"
	"net/url"
)

````

## 第 2 步：searchPageUsecase

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type searchPageUsecase interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Page</code> | <code>(context.Context, *logic.Actor, domain.SearchQuery, string, logic.BindOutput) (dto.SearchResultPage, error)</code> | 参数依次为ctx、可为空的Actor（匿名）、已校验查询、Edge handler标识、输出绑定回调；返回经过权限复核的搜索页或错误。 |

<!-- write-step: 2 bytes=333:484 -->
````go
type searchPageUsecase interface {
	Page(context.Context, *logic.Actor, domain.SearchQuery, string, logic.BindOutput) (dto.SearchResultPage, error)
}

````

## 第 3 步：SearchPages

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type SearchPages struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |
| <code>usecase</code> | <code>searchPageUsecase</code> | 完整搜索页业务接口，handler只解析请求与绑定响应，不直接把Search候选发送给用户。 |

<!-- write-step: 3 bytes=484:580 -->
````go
type SearchPages struct {
	identity *Identity
	outputs  *Outputs
	usecase  searchPageUsecase
}

````

## 第 4 步：NewSearchPages

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func NewSearchPages(identity *Identity, outputs *Outputs, usecase searchPageUsecase) *SearchPages`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>usecase</code> | <code>searchPageUsecase</code> | 在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*SearchPages</code> | 实际返回表达式：<code>&amp;SearchPages{identity: identity, outputs: outputs, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;SearchPages{identity: identity, outputs: outputs, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=580:760 -->
````go
func NewSearchPages(identity *Identity, outputs *Outputs, usecase searchPageUsecase) *SearchPages {
	return &SearchPages{identity: identity, outputs: outputs, usecase: usecase}
}

````

## 第 5 步：SearchPages.Page

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func (h *SearchPages) Page(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, domain.Violation(domain.InvalidRequest, &quot;SEARCH_PARAMETERS_INVALID&quot;))</code>；<code>httpx.Failure(w, err)</code>；<code>h.outputs.Bind(w, registration, sources)</code>；<code>httpx.Failure(w, domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_OUTPUT_REQUIRED&quot;))</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.URL</code>；<code>h.identity.OptionalActor(r, &quot;READ_PUBLIC_DOCUMENT&quot;, 0)</code>；<code>r.Context</code>；<code>r.Header</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>h.outputs == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>output == nil</code>。
- 检查 <code>actor != nil &#124;&#124; len(page.Items) != 0 &#124;&#124; page.HasMore &#124;&#124; page.NextCursor != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。

<!-- write-step: 5 bytes=760:2182 -->
````go
func (h *SearchPages) Page(w http.ResponseWriter, r *http.Request) {
	// URL.Query会隐藏畸形百分号/分号解析错误；必须先核完整原参数集合。
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		httpx.Failure(w, domain.Violation(domain.InvalidRequest, "SEARCH_PARAMETERS_INVALID"))
		return
	}
	query, err := domain.ParseSearchQuery(values)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.OptionalActor(r, "READ_PUBLIC_DOCUMENT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var output *ResponseOutput
	page, err := h.usecase.Page(r.Context(), actor, query, r.Header.Get("X-Edge-Handler"), func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		if h.outputs == nil {
			return nil
		}
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if output == nil {
		// 仅确实耗尽的匿名空页可普通输出；其他页没有真实双进程租约就是失败。
		if actor != nil || len(page.Items) != 0 || page.HasMore || page.NextCursor != nil {
			httpx.Failure(w, domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_REQUIRED"))
			return
		}
		platform.WriteJSON(w, http.StatusOK, page)
		return
	}
	if err = output.SendBounded(r.Context(), http.StatusOK, page, 2<<20); err != nil {
		httpx.Failure(w, err)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/search.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
