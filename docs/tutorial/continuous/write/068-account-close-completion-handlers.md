# 分步编写 `services/account/internal/handler/account/account_close_completion_handlers.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/068-account-close-completion-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

接入Document读取/确认注销的内部接口，并严格识别可公开续取的最小注销结果。

冻结会话应继续看自己的注销状态，但不能借恢复特例返回姓名、目录或正文。

写之前先按这条执行路线理解各段的连接：

1. AccountCloseDecisions.Read/Confirm要求document服务证书并校验原绑定。
2. MinimalClosureReceipt同时检查精确路径、操作号、字段集合和规范JSON字节。

本文件需要守住的边界：

- 不能只见state字段就绕过输出保护；额外字段或不同路由必须拒绝最小回执豁免。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/account_close_completion_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/account_close_completion_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/account_close_completion_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/account_close_completion_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:473 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"encoding/json"
	"net/http"
	"strings"
)

// account_close_decision.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountCloseDecisions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`type AccountCloseDecisions struct{ usecase *logic.AccountClosures }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountClosures</code> | 使用 <code>logic.AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。 |

<!-- write-step: 2 bytes=473:542 -->
````go
type AccountCloseDecisions struct{ usecase *logic.AccountClosures }

````

## 第 3 步：NewAccountCloseDecisions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func NewAccountCloseDecisions(usecase *logic.AccountClosures) *AccountCloseDecisions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountClosures</code> | 使用 <code>logic.AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseDecisions</code> | 实际返回表达式：<code>&amp;AccountCloseDecisions{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseDecisions{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=542:681 -->
````go
func NewAccountCloseDecisions(usecase *logic.AccountClosures) *AccountCloseDecisions {
	return &AccountCloseDecisions{usecase: usecase}
}

````

## 第 4 步：AccountCloseDecisions.Read

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func (h *AccountCloseDecisions) Read(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>httpx.Failure(w, 400, &quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;binding)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!platform.ValidateAccountClosureBinding(binding)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=681:1331 -->
````go
func (h *AccountCloseDecisions) Read(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.AccountClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	if !platform.ValidateAccountClosureBinding(binding) {
		httpx.Failure(w, 400, "ACCOUNT_CLOSURE_BINDING_INVALID")
		return
	}
	rejected, err := h.usecase.Rejected(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountClosureDecision{Rejected: rejected})
}

````

## 第 5 步：AccountCloseDecisions.Confirm

插入位置：第 4 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (h *AccountCloseDecisions) Confirm(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>httpx.Failure(w, 400, &quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;binding)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!platform.ValidateAccountClosureBinding(binding)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1331:2065 -->
````go
func (h *AccountCloseDecisions) Confirm(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.AccountClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	if !platform.ValidateAccountClosureBinding(binding) {
		httpx.Failure(w, 400, "ACCOUNT_CLOSURE_BINDING_INVALID")
		return
	}
	confirmed, err := h.usecase.Confirmed(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountClosureConfirmation{Confirmed: confirmed})
}

// account_close_output.go：保留原实现，按业务合并文件。

````

## 第 6 步：MinimalClosureReceipt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 70 行。

专用恢复输出不包含姓名、正文或目录；冻结原会话仍须能读取这一条最小计划。
不能扩大全局操作回执豁免：精确路由、原号、四字段和规范字节同时成立才放行。

本步声明：`func MinimalClosureReceipt(path string, raw []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>path != &quot;/v1/account/closures&quot;</code>；<code>path != &quot;/v1/account/closures/&quot;+plan.OperationId</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;plan)</code>；<code>string(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; bytes.Equal(canonical, &#91;&#93;byte(strings.TrimSuffix(string(raw), &quot;\n&quot;)))</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 2&lt;&lt;20</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;plan) != nil &#124;&#124; !domain.ValidOperationID(plan.OperationId) &#124;&#124; plan.RequiredSuccessions == nil &#124;&#124; plan.RetainedReleaseIds == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>path != &quot;/v1/account/closures&quot; &amp;&amp; path != &quot;/v1/account/closures/&quot;+plan.OperationId</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEntityID(item.TargetId) &#124;&#124; (item.Kind != &quot;ORGANIZATION_OWNER&quot; &amp;&amp; item.Kind != &quot;ORGANIZATION_ADMIN&quot; &amp;&amp; item.Kind != &quot;KNOWLEDGE_BASE_ADMIN&quot;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEntityID(release)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(canonical, &#91;&#93;byte(strings.TrimSuffix(string(raw), &quot;\n&quot;)))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2065:3405 -->
````go
// 专用恢复输出不包含姓名、正文或目录；冻结原会话仍须能读取这一条最小计划。
// 不能扩大全局操作回执豁免：精确路由、原号、四字段和规范字节同时成立才放行。
func MinimalClosureReceipt(path string, raw []byte) bool {
	if len(raw) > 2<<20 {
		return false
	}
	var plan dto.AccountClosePlan
	if json.Unmarshal(raw, &plan) != nil || !domain.ValidOperationID(plan.OperationId) || plan.RequiredSuccessions == nil || plan.RetainedReleaseIds == nil {
		return false
	}
	if path != "/v1/account/closures" && path != "/v1/account/closures/"+plan.OperationId {
		return false
	}
	switch plan.State {
	case "PREPARING", "FROZEN", "PLANNED", "CLEANING", "SUCCEEDED", "REJECTED":
	default:
		return false
	}
	for _, item := range plan.RequiredSuccessions {
		if !domain.ValidEntityID(item.TargetId) || (item.Kind != "ORGANIZATION_OWNER" && item.Kind != "ORGANIZATION_ADMIN" && item.Kind != "KNOWLEDGE_BASE_ADMIN") {
			return false
		}
	}
	for _, release := range plan.RetainedReleaseIds {
		if !domain.ValidEntityID(release) {
			return false
		}
	}
	canonical, err := json.Marshal(plan)
	// Encoder允许一次LF；额外字段、重复键或非规范编码不能夹带未解析前缀。
	return err == nil && bytes.Equal(canonical, []byte(strings.TrimSuffix(string(raw), "\n")))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/account_close_completion_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
