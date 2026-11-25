# 分步编写 `services/document/internal/handler/document/workspace_handlers.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/265-workspace-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。

所有变更统一经过严格DTO、当前身份和幂等事务入口；各操作保留自己的expected revision条件。 搜索沿原只读路由转交业务层，handler只取路径/URL，不因增加q绕开Identity或受控输出。

写之前先按这条执行路线理解各段的连接：

1. Directory转送limit/cursor
2. Documents.Create和KnowledgeBases.Create准备创建闭包
3. Management.Transfer解析目标用户
4. MainControls.Chapter把章节输入转为固定控制意图。
5. Directory.KnowledgeBases把URL limit/cursor/q传入原用例；Directory.KnowledgeBaseMembers额外带当前Cookie给Account目录适配。
6. 两条搜索路线仍经Identity.Actor→业务准入→Transactions.Read，不能在handler直接读仓储后WriteJSON。

本文件需要守住的边界：

- 知识库角色、创建资格和文档负责人不是同一权限
- 不要把所有入口简化成仅登录可调用。
- q属于URL只读参数，不是另造的请求体模型；长度与规范化在directorySearch统一处理。
- 上一页也是新的GET；无论页码或q如何变化，真实身份/KB资格与输出来源检查都保留。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/workspace_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/workspace_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/workspace_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/workspace_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:464 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
)

// chapters.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControls.Chapter

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func (h *MainControls) Chapter(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.change(w, r, intent, input)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.change(w, r, intent, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=464:821 -->
````go
func (h *MainControls) Chapter(w http.ResponseWriter, r *http.Request) {
	var input dto.ChapterCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	intent, err := domain.ParseChapterCreate(input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.change(w, r, intent, input)
}

// content_versions.go：保留原实现，按业务合并文件。

````

## 第 3 步：ContentVersions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`type ContentVersions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.ContentVersions</code> | 使用 <code>logic.ContentVersions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。 |

<!-- write-step: 3 bytes=821:931 -->
````go
type ContentVersions struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.ContentVersions
}

````

## 第 4 步：NewContentVersions

插入位置：第 3 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func NewContentVersions(identity *Identity, requests *Transactions, usecase *logic.ContentVersions) *ContentVersions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.ContentVersions</code> | 使用 <code>logic.ContentVersions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ContentVersions</code> | 实际返回表达式：<code>&amp;ContentVersions{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ContentVersions{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=931:1136 -->
````go
func NewContentVersions(identity *Identity, requests *Transactions, usecase *logic.ContentVersions) *ContentVersions {
	return &ContentVersions{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 5 步：ContentVersions.Read

插入位置：第 4 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func (h *ContentVersions) Read(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, scope.Document, &quot;READ_CONTENT_VERSION&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.identity.Actor(r, false, &quot;READ_CONTENT_VERSION&quot;, scope.Document)</code>；<code>h.requests.Read(w, r, actor, sources, scope.Document, &quot;READ_CONTENT_VERSION&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1136:1934 -->
````go
func (h *ContentVersions) Read(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["contentVersionId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	scope, err := h.usecase.Lookup(r.Context(), id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_CONTENT_VERSION", scope.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Read(r.Context(), actor, scope)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 沿统一HTTP输出登记与关闭租约交付，版本ID不能成为撤权后的下载后门。
	h.requests.Read(w, r, actor, sources, scope.Document, "READ_CONTENT_VERSION", run)
}

// directory.go：保留原实现，按业务合并文件。

````

## 第 6 步：Directory

插入位置：第 5 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`type Directory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Directory</code> | 使用 <code>logic.Directory</code> 类型；其定义处的职责：按当前权限分页展示空间目录、知识库、成员及文档协作候选。 |

<!-- write-step: 6 bytes=1934:2032 -->
````go
type Directory struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Directory
}

````

## 第 7 步：NewDirectory

插入位置：第 6 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func NewDirectory(identity *Identity, requests *Transactions, usecase *logic.Directory) *Directory`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Directory</code> | 使用 <code>logic.Directory</code> 类型；其定义处的职责：按当前权限分页展示空间目录、知识库、成员及文档协作候选。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Directory</code> | 实际返回表达式：<code>&amp;Directory{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Directory{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2032:2212 -->
````go
func NewDirectory(identity *Identity, requests *Transactions, usecase *logic.Directory) *Directory {
	return &Directory{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 8 步：Directory.KnowledgeBase

插入位置：第 7 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func (h *Directory) KnowledgeBase(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.ReadProjected(w, r, actor, sources, 0, &quot;READ_KB&quot;, run, func(value any) any { 		reading := value.(logic.KnowledgeBaseReading) 		actions, _ := json.Marshal(reading.Actions …</code>（长表达式，按本步完整代码继续）；<code>w.Header</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;READ_KB&quot;, 0)</code>；<code>r.Context</code>；<code>h.requests.ReadProjected(w, r, actor, sources, 0, &quot;READ_KB&quot;, run, func(value any) any { 		reading := value.(logic.KnowledgeBaseReading) 		actions, _ := json.Marshal(reading.Actions …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return reading.KnowledgeBase</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2212:2917 -->
````go
func (h *Directory) KnowledgeBase(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "READ_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBase(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.ReadProjected(w, r, actor, sources, 0, "READ_KB", run, func(value any) any {
		reading := value.(logic.KnowledgeBaseReading)
		actions, _ := json.Marshal(reading.Actions)
		w.Header().Set("X-Available-Actions", string(actions))
		return reading.KnowledgeBase
	})
}
````

## 第 9 步：Directory.KnowledgeBases

插入位置：第 8 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func (h *Directory) KnowledgeBases(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 把可选q与原分页参数转交同一目录用例，最后仍走统一受控读取。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 输出原KnowledgeBasePage或MemberPage及明确错误。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;LIST_KB&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 路径spaceId；URL limit/cursor/q；当前身份仍由Identity.Actor取得。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;LIST_KB&quot;, 0)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2917:3480 -->
````go
func (h *Directory) KnowledgeBases(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "LIST_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBases(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("q"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "LIST_KB", run)
}
````

## 第 10 步：Directory.Documents

插入位置：第 9 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func (h *Directory) Documents(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;LIST_DOCUMENTS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;LIST_DOCUMENTS&quot;, 0)</code>；<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>pathvar.Vars(r)&#91;&quot;knowledgeBaseId&quot;&#93; != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3480:4182 -->
````go
func (h *Directory) Documents(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "LIST_DOCUMENTS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var space, kb uint64
	if pathvar.Vars(r)["knowledgeBaseId"] != "" {
		kb, err = domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	} else {
		space, err = domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Documents(r.Context(), actor, space, kb, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "LIST_DOCUMENTS", run)
}
````

## 第 11 步：Directory.KnowledgeBaseMembers

插入位置：第 10 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`func (h *Directory) KnowledgeBaseMembers(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 把可选q与原分页参数转交同一目录用例，最后仍走统一受控读取。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 输出原KnowledgeBasePage或MemberPage及明确错误。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;KB_MEMBERS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 路径knowledgeBaseId；URL limit/cursor/q；Cookie只交Account目录解析适配，不作为自报用户身份。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;KB_MEMBERS&quot;, 0)</code>；<code>r.Context</code>；<code>r.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4182:4796 -->
````go
func (h *Directory) KnowledgeBaseMembers(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "KB_MEMBERS", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.KnowledgeBaseMembers(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), r.URL.Query().Get("q"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "KB_MEMBERS", run)
}

````

## 第 12 步：Directory.DocumentCollaborators

插入位置：第 11 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`func (h *Directory) DocumentCollaborators(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;DOCUMENT_COLLABORATORS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;DOCUMENT_COLLABORATORS&quot;, id)</code>；<code>r.Context</code>；<code>r.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4796:5409 -->
````go
func (h *Directory) DocumentCollaborators(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DOCUMENT_COLLABORATORS", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DocumentCollaborators(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "DOCUMENT_COLLABORATORS", run)
}

````

## 第 13 步：Directory.DocumentMentionCandidates

插入位置：第 12 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func (h *Directory) DocumentMentionCandidates(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;DOCUMENT_MENTION_CANDIDATES&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;DOCUMENT_MENTION_CANDIDATES&quot;, id)</code>；<code>r.Context</code>；<code>r.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5409:6110 -->
````go
func (h *Directory) DocumentMentionCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "DOCUMENT_MENTION_CANDIDATES", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.DocumentMentionCandidates(r.Context(), actor, id, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "DOCUMENT_MENTION_CANDIDATES", run)
}

// document_creation.go：保留原实现，按业务合并文件。

````

## 第 14 步：Documents

插入位置：第 13 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`type Documents struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Documents</code> | 使用 <code>logic.Documents</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。 |

<!-- write-step: 14 bytes=6110:6208 -->
````go
type Documents struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Documents
}

````

## 第 15 步：NewDocuments

插入位置：第 14 步代码之后，紧接追加；完成稿约第 206 行。

本步声明：`func NewDocuments(identity *Identity, requests *Transactions, usecase *logic.Documents) *Documents`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Documents</code> | 使用 <code>logic.Documents</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Documents</code> | 实际返回表达式：<code>&amp;Documents{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Documents{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6208:6388 -->
````go
func NewDocuments(identity *Identity, requests *Transactions, usecase *logic.Documents) *Documents {
	return &Documents{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 16 步：Documents.Create

插入位置：第 15 步代码之后，紧接追加；完成稿约第 209 行。

本步声明：`func (h *Documents) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Fail(w, 400, &quot;DOCUMENT_INPUT_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.identity.Actor(r, true, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !domain.ValidName(input.Title, 240) &#124;&#124; !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy))</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6388:7312 -->
````go
func (h *Documents) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	space, err := domain.CanonicalID(input.SpaceId)
	if err != nil || !domain.ValidName(input.Title, 240) || !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy)) {
		httpx.Fail(w, 400, "DOCUMENT_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CREATE_DOCUMENT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_CREATE", input, 201, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Create(r.Context(), actor, space, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_CREATE", input, 201, run, nil)
}

// editable.go：保留原实现，按业务合并文件。

````

## 第 17 步：EditableUnits

插入位置：第 16 步代码之后，紧接追加；完成稿约第 237 行。

本步声明：`type EditableUnits struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.EditableUnits</code> | 使用 <code>logic.EditableUnits</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。 |

<!-- write-step: 17 bytes=7312:7418 -->
````go
type EditableUnits struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.EditableUnits
}

````

## 第 18 步：NewEditableUnits

插入位置：第 17 步代码之后，紧接追加；完成稿约第 243 行。

本步声明：`func NewEditableUnits(identity *Identity, requests *Transactions, usecase *logic.EditableUnits) *EditableUnits`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.EditableUnits</code> | 使用 <code>logic.EditableUnits</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EditableUnits</code> | 实际返回表达式：<code>&amp;EditableUnits{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EditableUnits{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7418:7614 -->
````go
func NewEditableUnits(identity *Identity, requests *Transactions, usecase *logic.EditableUnits) *EditableUnits {
	return &EditableUnits{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 19 步：EditableUnits.List

插入位置：第 18 步代码之后，紧接追加；完成稿约第 246 行。

本步声明：`func (h *EditableUnits) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, id, &quot;EDITABLE_UNITS&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;EDITABLE_UNITS&quot;, id)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=7614:8227 -->
````go
func (h *EditableUnits) List(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "EDITABLE_UNITS", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Prepare(r.Context(), actor, id, r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, id, "EDITABLE_UNITS", run)
}

// knowledge_base.go：保留原实现，按业务合并文件。

````

## 第 20 步：KnowledgeBases

插入位置：第 19 步代码之后，紧接追加；完成稿约第 267 行。

本步声明：`type KnowledgeBases struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.KnowledgeBases</code> | 使用 <code>logic.KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。 |

<!-- write-step: 20 bytes=8227:8335 -->
````go
type KnowledgeBases struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.KnowledgeBases
}

````

## 第 21 步：NewKnowledgeBases

插入位置：第 20 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`func NewKnowledgeBases(identity *Identity, requests *Transactions, usecase *logic.KnowledgeBases) *KnowledgeBases`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.KnowledgeBases</code> | 使用 <code>logic.KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*KnowledgeBases</code> | 实际返回表达式：<code>&amp;KnowledgeBases{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;KnowledgeBases{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=8335:8535 -->
````go
func NewKnowledgeBases(identity *Identity, requests *Transactions, usecase *logic.KnowledgeBases) *KnowledgeBases {
	return &KnowledgeBases{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 22 步：KnowledgeBases.Create

插入位置：第 21 步代码之后，紧接追加；完成稿约第 276 行。

本步声明：`func (h *KnowledgeBases) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;KNOWLEDGE_BASE_INPUT_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KNOWLEDGE_BASE_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;CREATE_KNOWLEDGE_BASE&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KNOWLEDGE_BASE_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidName(input.Name, 160) &#124;&#124; !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy))</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KNOWLEDGE_BASE_CREATE&quot;, input, 201, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=8535:9465 -->
````go
func (h *KnowledgeBases) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.KBCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["spaceId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Name, 160) || !domain.ValidVisibility(domain.Visibility(input.VisibilityPolicy)) {
		httpx.Fail(w, 400, "KNOWLEDGE_BASE_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "CREATE_KNOWLEDGE_BASE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KNOWLEDGE_BASE_CREATE", input, 201, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Create(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KNOWLEDGE_BASE_CREATE", input, 201, run, nil)
}
````

## 第 23 步：KnowledgeBases.Rename

插入位置：第 22 步代码之后，紧接追加；完成稿约第 305 行。

本步声明：`func (h *KnowledgeBases) Rename(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;KB_NAME_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RENAME_KB&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidName(input.Name, 160)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=9465:10292 -->
````go
func (h *KnowledgeBases) Rename(w http.ResponseWriter, r *http.Request) {
	var input dto.OrganizationEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Name, 160) {
		httpx.Fail(w, 400, "KB_NAME_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "RENAME_KB", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_RENAME", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Rename(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_RENAME", input, 200, run, nil)
}
````

## 第 24 步：KnowledgeBases.CreationGrant

插入位置：第 23 步代码之后，紧接追加；完成稿约第 334 行。

本步声明：`func (h *KnowledgeBases) CreationGrant(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_CREATION_GRANT&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;KB_CREATION_GRANT&quot;, input, 200, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;KB_CREATION_GRANT&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_CREATION_GRANT&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_CREATION_GRANT&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10292:11167 -->
````go
func (h *KnowledgeBases) CreationGrant(w http.ResponseWriter, r *http.Request) {
	var input dto.CreationGrant
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.UserId)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "KB_CREATION_GRANT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_CREATION_GRANT", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.CreationGrant(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_CREATION_GRANT", input, 200, run, nil)
}
````

## 第 25 步：KnowledgeBases.Role

插入位置：第 24 步代码之后，紧接追加；完成稿约第 364 行。

本步声明：`func (h *KnowledgeBases) Role(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;KB_ROLE_INPUT_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_ROLE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;APPOINT_KB_ROLE&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_ROLE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; (input.Role != &quot;ADMIN&quot; &amp;&amp; input.Role != &quot;MEMBER&quot; &amp;&amp; input.Role != &quot;REMOVE&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;KB_ROLE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=11167:12160 -->
````go
func (h *KnowledgeBases) Role(w http.ResponseWriter, r *http.Request) {
	var input dto.KBRole
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["knowledgeBaseId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.UserId)
	if err != nil || (input.Role != "ADMIN" && input.Role != "MEMBER" && input.Role != "REMOVE") {
		httpx.Fail(w, 400, "KB_ROLE_INPUT_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "APPOINT_KB_ROLE", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "KB_ROLE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Role(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "KB_ROLE", input, 202, run, nil)
}

// management.go：保留原实现，按业务合并文件。

````

## 第 26 步：Management

插入位置：第 25 步代码之后，紧接追加；完成稿约第 397 行。

本步声明：`type Management struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Management</code> | 使用 <code>logic.Management</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。 |

<!-- write-step: 26 bytes=12160:12260 -->
````go
type Management struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Management
}

````

## 第 27 步：NewManagement

插入位置：第 26 步代码之后，紧接追加；完成稿约第 403 行。

本步声明：`func NewManagement(identity *Identity, requests *Transactions, usecase *logic.Management) *Management`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Management</code> | 使用 <code>logic.Management</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Management</code> | 实际返回表达式：<code>&amp;Management{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Management{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=12260:12444 -->
````go
func NewManagement(identity *Identity, requests *Transactions, usecase *logic.Management) *Management {
	return &Management{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 28 步：Management.Rename

插入位置：第 27 步代码之后，紧接追加；完成稿约第 406 行。

本步声明：`func (h *Management) Rename(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;DOCUMENT_TITLE_INVALID&quot;)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;RENAME_DOCUMENT&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidName(input.Title, 240)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_RENAME&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=12444:13285 -->
````go
func (h *Management) Rename(w http.ResponseWriter, r *http.Request) {
	var input dto.DocumentEdit
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidName(input.Title, 240) {
		httpx.Fail(w, 400, "DOCUMENT_TITLE_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "RENAME_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_RENAME", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Rename(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_RENAME", input, 200, run, nil)
}
````

## 第 29 步：Management.Transfer

插入位置：第 28 步代码之后，紧接追加；完成稿约第 435 行。

本步声明：`func (h *Management) Transfer(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;RESPONSIBLE_TRANSFER&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;RESPONSIBLE_TRANSFER&quot;, input, 200, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;TRANSFER_RESPONSIBLE&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;RESPONSIBLE_TRANSFER&quot;, input, 200, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;RESPONSIBLE_TRANSFER&quot;, input, 200, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=13285:14259 -->
````go
func (h *Management) Transfer(w http.ResponseWriter, r *http.Request) {
	var input dto.OwnerChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	user, err := domain.CanonicalID(input.TargetUserId)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "TRANSFER_RESPONSIBLE", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 转交后发起人可以读取原号最小结果；TryReplay不会重新执行新的转交动作。
	if h.requests.Replay(w, r, actor, input.OperationId, "RESPONSIBLE_TRANSFER", input, 200, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Transfer(r.Context(), actor, id, user, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "RESPONSIBLE_TRANSFER", input, 200, run, nil)
}
````

## 第 30 步：Management.Delete

插入位置：第 29 步代码之后，紧接追加；完成稿约第 466 行。

本步声明：`func (h *Management) Delete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;DOCUMENT_DELETE&quot;, input, 202, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;DELETE_DOCUMENT&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;DOCUMENT_DELETE&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=14259:15061 -->
````go
func (h *Management) Delete(w http.ResponseWriter, r *http.Request) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "DELETE_DOCUMENT", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "DOCUMENT_DELETE", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Delete(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "DOCUMENT_DELETE", input, 202, run, nil)
}

// space.go：保留原实现，按业务合并文件。

````

## 第 31 步：Spaces

插入位置：第 30 步代码之后，紧接追加；完成稿约第 494 行。

本步声明：`type Spaces struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.Spaces</code> | 使用 <code>logic.Spaces</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。 |

<!-- write-step: 31 bytes=15061:15153 -->
````go
type Spaces struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.Spaces
}

````

## 第 32 步：NewSpaces

插入位置：第 31 步代码之后，紧接追加；完成稿约第 500 行。

本步声明：`func NewSpaces(identity *Identity, requests *Transactions, usecase *logic.Spaces) *Spaces`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.Spaces</code> | 使用 <code>logic.Spaces</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Spaces</code> | 实际返回表达式：<code>&amp;Spaces{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Spaces{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=15153:15321 -->
````go
func NewSpaces(identity *Identity, requests *Transactions, usecase *logic.Spaces) *Spaces {
	return &Spaces{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 33 步：Spaces.List

插入位置：第 32 步代码之后，紧接追加；完成稿约第 503 行。

本步声明：`func (h *Spaces) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;SPACE_DIRECTORY&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;SPACE_DIRECTORY&quot;, 0)</code>；<code>r.Context</code>；<code>r.Header</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=15321:15764 -->
````go
func (h *Spaces) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "SPACE_DIRECTORY", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Prepare(r.Context(), actor, r.Header.Get("Cookie"), r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "SPACE_DIRECTORY", run)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/workspace_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
