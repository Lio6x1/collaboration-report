# 分步编写 `services/account/internal/handler/account/logout.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/073-logout.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

处理用户退出HTTP与主/恢复Cookie交替。

退出准备后主会话可能已无效，但原秘密仍需要短期留给进度查询和重复请求。

写之前先按这条执行路线理解各段的连接：

1. Logout检查Origin、DTO/操作号，通过Resolve取当前或已登记原会话，再核CSRF。
2. 准备成功后logoutCookies清主会话与CSRF，同时把原秘密放恢复Cookie；ProgressDTO提供原poll_url。

本文件需要守住的边界：

- 只删除Cookie不是可靠退出；必须先持久登记服务端关闭。
- 恢复Cookie不能变成通用登录凭据。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/logout.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/logout.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/logout.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/logout.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:273 -->
````go
package account

import (
	"net/http"
	"time"

	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
)

````

## 第 2 步：Logout

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type Logout struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.Logout</code> | 使用 <code>logic.Logout</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，用于核对浏览器请求来源/构建本源链接；不能信任任意Host或输入URL。 |

<!-- write-step: 2 bytes=273:334 -->
````go
type Logout struct {
	logic  *logic.Logout
	origin string
}

````

## 第 3 步：NewLogout

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func NewLogout(service *logic.Logout, origin string) *Logout`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.Logout</code> | 使用 <code>logic.Logout</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。<br>在此声明中的实际使用：<code>logic: service</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Logout</code> | 实际返回表达式：<code>&amp;Logout{logic: service, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Logout{logic: service, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=334:448 -->
````go
func NewLogout(service *logic.Logout, origin string) *Logout {
	return &Logout{logic: service, origin: origin}
}

````

## 第 4 步：ProgressDTO

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func ProgressDTO(id, state string) dto.Operation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;/v1/account/operations/&quot; + id</code>；<code>OperationId: id</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>State: state</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Operation</code> | 实际返回表达式：<code>dto.Operation{OperationId: id, State: state, PollUrl: &amp;poll}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return dto.Operation{OperationId: id, State: state, PollUrl: &amp;poll}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=448:611 -->
````go
func ProgressDTO(id, state string) dto.Operation {
	poll := "/v1/account/operations/" + id
	return dto.Operation{OperationId: id, State: state, PollUrl: &poll}
}

````

## 第 5 步：Logout.Logout

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (h *Logout) Logout(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ValidCSRF(r, selection.Session)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!selection.Registered</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=611:1526 -->
````go
func (h *Logout) Logout(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Action
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return
	}
	selection, err := h.logic.Resolve(r.Context(), input.OperationId, httpx.CookieValue(r, httpx.SessionCookie), httpx.CookieValue(r, httpx.RecoveryCookie))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !httpx.ValidCSRF(r, selection.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return
	}
	state := selection.ReceiptState
	if !selection.Registered {
		state, err = h.logic.Prepare(r.Context(), input.OperationId, selection.Session)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	logoutCookies(w, r, selection.Token)
	platform.WriteJSON(w, 202, ProgressDTO(input.OperationId, state))
}

````

## 第 6 步：logoutCookies

插入位置：第 5 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func logoutCookies(w http.ResponseWriter, r *http.Request, originalToken string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.ClearCookie(w, httpx.SessionCookie)</code>；<code>httpx.SetCookie(w, httpx.RecoveryCookie, originalToken, time.Now().UTC().Add(24*time.Hour), true)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.CookieValue(r, httpx.SessionCookie)</code> |
| <code>originalToken</code> | <code>string</code> | 在此声明中的实际使用：<code>httpx.CookieValue(r, httpx.SessionCookie) == originalToken</code>；<code>httpx.SetCookie(w, httpx.RecoveryCookie, originalToken, time.Now().UTC().Add(24*time.Hour), true)</code> |

**跟着本段代码走一遍：**

- 检查 <code>httpx.CookieValue(r, httpx.SessionCookie) == originalToken</code>。

<!-- write-step: 6 bytes=1526:1926 -->
````go
func logoutCookies(w http.ResponseWriter, r *http.Request, originalToken string) {
	// 清原登录Cookie，另一个新会话不能被旧重试清掉；恢复Cookie只证明原操作。
	if httpx.CookieValue(r, httpx.SessionCookie) == originalToken {
		httpx.ClearCookie(w, httpx.SessionCookie)
	}
	httpx.SetCookie(w, httpx.RecoveryCookie, originalToken, time.Now().UTC().Add(24*time.Hour), true)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/logout.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
