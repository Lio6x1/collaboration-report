# 分步编写 `services/account/internal/handler/account/account_close_preparation_handlers.go`

先按 [A06 · 账号注销准备、职责交接与来源冻结](../lessons/account/06-account-close-preparation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/069-account-close-preparation-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

处理用户账号注销发起/进度，以及向Document交付原绑定。

发起必须有效主会话与当前密码，冻结后恢复仅有最小进度权限。

写之前先按这条执行路线理解各段的连接：

1. Start先检查Origin、严格DTO及HMAC指纹，Resolve区分新操作和已登记原操作，再执行Start。
2. Progress从路径取原号，使用主/恢复Cookie查询；Binding仅document证书可调用。

本文件需要守住的边界：

- 恢复Cookie不能发起新的注销；原号查询不能返回任意他人的固定组织集合。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/account_close_preparation_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/account_close_preparation_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/account_close_preparation_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/account_close_preparation_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:480 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountClosures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type AccountClosures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountClosures</code> | 使用 <code>logic.AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。 |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，用于核对浏览器请求来源/构建本源链接；不能信任任意Host或输入URL。 |

<!-- write-step: 2 bytes=480:586 -->
````go
type AccountClosures struct {
	usecase *logic.AccountClosures
	secrets *domain.Secrets
	origin  string
}

````

## 第 3 步：NewAccountClosures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewAccountClosures(usecase *logic.AccountClosures, secrets *domain.Secrets, origin string) *AccountClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.AccountClosures</code> | 使用 <code>logic.AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>secrets: secrets</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountClosures</code> | 实际返回表达式：<code>&amp;AccountClosures{usecase: usecase, secrets: secrets, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountClosures{usecase: usecase, secrets: secrets, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=586:781 -->
````go
func NewAccountClosures(usecase *logic.AccountClosures, secrets *domain.Secrets, origin string) *AccountClosures {
	return &AccountClosures{usecase: usecase, secrets: secrets, origin: origin}
}

````

## 第 4 步：AccountClosures.Start

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (h *AccountClosures) Start(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.WriteError(w, err)</code>；<code>httpx.Failure(w, 403, &quot;CSRF_INVALID&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ValidCSRF(r, selection.Session)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=781:1733 -->
````go
func (h *AccountClosures) Start(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.AccountClose
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	selection, err := h.usecase.Resolve(r.Context(), input.OperationId, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !httpx.ValidCSRF(r, selection.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return
	}
	plan, err := h.usecase.Start(r.Context(), selection.Session, input, h.secrets.Fingerprint(input))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	// 准备期保留主会话以便交接；原秘密另存恢复Cookie，身份冻结后只读本操作。
	httpx.SetCookie(w, httpx.RecoveryCookie, selection.Token, time.Now().UTC().Add(24*time.Hour), true)
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 202, plan)
}

````

## 第 5 步：AccountClosures.Progress

插入位置：第 4 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func (h *AccountClosures) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>w.Header</code>；<code>platform.WriteJSON(w, 200, plan)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code>；<code>httpx.CookieValue(r, httpx.RecoveryCookie)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1733:2313 -->
````go
func (h *AccountClosures) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	plan, err := h.usecase.Progress(r.Context(), operation, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, plan)
}

// account_close_binding.go：保留原实现，按业务合并文件。

````

## 第 6 步：AccountCloseDecisions.Binding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 72 行。

原关闭组织集合只向Doc服务控制通道交付；浏览器/其他服务不能按operation号查范围。

本步声明：`func (h *AccountCloseDecisions) Binding(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;query)</code>；<code>httpx.WriteError(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;query)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;query)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2313:2906 -->
````go
// 原关闭组织集合只向Doc服务控制通道交付；浏览器/其他服务不能按operation号查范围。
func (h *AccountCloseDecisions) Binding(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var query dto.Action
	if !httpx.ReadJSON(w, r, &query) {
		return
	}
	binding, err := h.usecase.BindingForDocument(r.Context(), query.OperationId)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, binding)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/account_close_preparation_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/06-account-close-preparation.md)。
