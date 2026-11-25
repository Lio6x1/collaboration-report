# 分步编写 `services/document/internal/handler/document/account_lifecycle_handlers.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/250-account-lifecycle-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供Account控制通道的准备、计划、关系、分步清理与完成入口。

浏览器、Account与Node拥有不同控制职责，清理资格不能由任意调用者代选文档。

写之前先按这条执行路线理解各段的连接：

1. accountRetirementInput解权威绑定
2. Prepare/Cancel处理准备
3. Plan/Progress给固定范围
4. ReleaseDraftResources/CleanHistory/CleanPayloads处理原单Doc
5. DocumentCleanupContext仅给验证过的Node。

本文件需要守住的边界：

- Account只能请求原计划里的文档
- Doc使用自身Node证书取得真实回执，不接受浏览器自报完成。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/account_lifecycle_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/account_lifecycle_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/account_lifecycle_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/account_lifecycle_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:405 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"net/http"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetirements.DocumentCleanupContext

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

清理资格只提供给已验Node服务证书，不接受浏览器或Account调用者代选文档。

本步声明：`func (h *AccountRetirements) DocumentCleanupContext(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Fail(w, 400, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>w.Header</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>h.retainedGovernanceCleanupContext(w, r, request)</code>；<code>h.purgeCleanupContext(w, r, request)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!request.Valid()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.retainedGovernanceCleanupContext(w, r, request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>h.purgeCleanupContext(w, r, request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=405:1261 -->
````go
// 清理资格只提供给已验Node服务证书，不接受浏览器或Account调用者代选文档。
func (h *AccountRetirements) DocumentCleanupContext(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	if !request.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if h.retainedGovernanceCleanupContext(w, r, request) {
		return
	}
	if h.purgeCleanupContext(w, r, request) {
		return
	}
	result, err := h.retentions.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, result)
}

````

## 第 3 步：AccountRetirements.ReleaseDraftResources

插入位置：第 2 步代码之后，紧接追加；完成稿约第 43 行。

Account只能请求固定计划中的单文档；Doc用自己的Node证书取得原清理回执。

本步声明：`func (h *AccountRetirements) ReleaseDraftResources(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Fail(w, 400, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>w.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!request.Valid()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1261:2089 -->
````go
// Account只能请求固定计划中的单文档；Doc用自己的Node证书取得原清理回执。
func (h *AccountRetirements) ReleaseDraftResources(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	if !request.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.ReleaseDraftResources(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "DRAFT_RESOURCES_RELEASED"})
}

````

## 第 4 步：AccountRetirements.CleanHistory

插入位置：第 3 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func (h *AccountRetirements) CleanHistory(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Fail(w, 400, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>w.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!request.Valid()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2089:2788 -->
````go
func (h *AccountRetirements) CleanHistory(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	if !request.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.CleanHistory(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "HISTORY_REMOVED"})
}

````

## 第 5 步：AccountRetirements.CleanPayloads

插入位置：第 4 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func (h *AccountRetirements) CleanPayloads(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Fail(w, 400, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>w.Header</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!request.Valid()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2788:3561 -->
````go
func (h *AccountRetirements) CleanPayloads(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	if !request.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.retentions.CleanPayloads(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "PAYLOADS_REMOVED"})
}

// account_retirement.go：保留原实现，按业务合并文件。

````

## 第 6 步：AccountRetirements

插入位置：第 5 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`type AccountRetirements struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountRetirements</code> | 使用 <code>logic.AccountRetirements</code> 类型；其定义处的职责：为Account原注销操作准备管理员预留和冻结，并处理有证明的取消。 |
| <code>retentions</code> | <code>*logic.AccountRetentions</code> | 使用 <code>logic.AccountRetentions</code> 类型；其定义处的职责：固定账号原公开正式版依赖，并在各文档清理完成后启用留存。 |
| <code>purges</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。 |
| <code>retainedGovernance</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。 |

<!-- write-step: 6 bytes=3561:3768 -->
````go
type AccountRetirements struct {
	usecase            *logic.AccountRetirements
	retentions         *logic.AccountRetentions
	purges             *logic.Purges
	retainedGovernance *logic.RetainedGovernance
}

````

## 第 7 步：NewAccountRetirements

插入位置：第 6 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`func NewAccountRetirements(usecase *logic.AccountRetirements, retentions *logic.AccountRetentions) *AccountRetirements`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountRetirements</code> | 使用 <code>logic.AccountRetirements</code> 类型；其定义处的职责：为Account原注销操作准备管理员预留和冻结，并处理有证明的取消。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>retentions</code> | <code>*logic.AccountRetentions</code> | 使用 <code>logic.AccountRetentions</code> 类型；其定义处的职责：固定账号原公开正式版依赖，并在各文档清理完成后启用留存。<br>在此声明中的实际使用：<code>retentions: retentions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountRetirements</code> | 实际返回表达式：<code>&amp;AccountRetirements{usecase: usecase, retentions: retentions}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountRetirements{usecase: usecase, retentions: retentions}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3768:3962 -->
````go
func NewAccountRetirements(usecase *logic.AccountRetirements, retentions *logic.AccountRetentions) *AccountRetirements {
	return &AccountRetirements{usecase: usecase, retentions: retentions}
}

````

## 第 8 步：AccountRetirements.Plan

插入位置：第 7 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`func (h *AccountRetirements) Plan(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: &quot;PLANNED&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3962:4292 -->
````go
func (h *AccountRetirements) Plan(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if _, err := h.retentions.Plan(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "PLANNED"})
}

````

## 第 9 步：AccountRetirements.Relations

插入位置：第 8 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func (h *AccountRetirements) Relations(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: &quot;CLEANING&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4292:4635 -->
````go
func (h *AccountRetirements) Relations(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.retentions.ApplyRelations(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "CLEANING"})
}

````

## 第 10 步：AccountRetirements.Complete

插入位置：第 9 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func (h *AccountRetirements) Complete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: &quot;SUCCEEDED&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4635:4972 -->
````go
func (h *AccountRetirements) Complete(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.retentions.Complete(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureRetention{State: "SUCCEEDED"})
}

````

## 第 11 步：AccountRetirements.Progress

插入位置：第 10 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func (h *AccountRetirements) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Failure(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4972:5452 -->
````go
func (h *AccountRetirements) Progress(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountClosurePageRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.retentions.Progress(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

````

## 第 12 步：accountRetirementInput

插入位置：第 11 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`func accountRetirementInput(w http.ResponseWriter, r *http.Request) (domain.AccountRetirementBinding, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;binding)</code>；<code>httpx.Fail(w, 400, &quot;ACCOUNT_RETIREMENT_BINDING_INVALID&quot;)</code>；<code>w.Header</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>实际返回表达式：<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;binding)</code>。条件成立时执行 <code>return binding, false</code>。当前函数在这里结束。
- 检查 <code>!binding.Valid()</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return binding, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5452:5971 -->
````go
func accountRetirementInput(w http.ResponseWriter, r *http.Request) (domain.AccountRetirementBinding, bool) {
	var binding domain.AccountRetirementBinding
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return binding, false
	}
	if !httpx.DecodeBody(w, r, &binding) {
		return binding, false
	}
	if !binding.Valid() {
		httpx.Fail(w, 400, "ACCOUNT_RETIREMENT_BINDING_INVALID")
		return binding, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return binding, true
}

````

## 第 13 步：AccountRetirements.Prepare

插入位置：第 12 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`func (h *AccountRetirements) Prepare(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.AccountClosurePreparation{State: result.State, RequiredKnowledgeBases: result.RequiredKnowledgeBases, ReservedKnowledgeBases: result.ReservedKnowledgeBases})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5971:6424 -->
````go
func (h *AccountRetirements) Prepare(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	result, err := h.usecase.Prepare(r.Context(), binding)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosurePreparation{State: result.State, RequiredKnowledgeBases: result.RequiredKnowledgeBases, ReservedKnowledgeBases: result.ReservedKnowledgeBases})
}

````

## 第 14 步：AccountRetirements.Cancel

插入位置：第 13 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`func (h *AccountRetirements) Cancel(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.AccountClosureDecision{Rejected: true})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>accountRetirementInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6424:6748 -->
````go
func (h *AccountRetirements) Cancel(w http.ResponseWriter, r *http.Request) {
	binding, ok := accountRetirementInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Cancel(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.AccountClosureDecision{Rejected: true})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/account_lifecycle_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
