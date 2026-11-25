# 分步编写 `services/account/internal/handler/account/authentication_handlers.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/070-authentication-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将登录、验证码、注册和凭据变更用例接入浏览器 HTTP。

handler 负责读 DTO、Origin/CSRF 和 Cookie，业务事务仍由 logic 掌握；重试必须先恢复旧操作再判新请求资格。

写之前先按这条执行路线理解各段的连接：

1. Authentication.Login 仅在 logic 成功提交后设置会话和CSRF Cookie。
2. Authenticate 捕获 identity/credential/session 三种输出来源，mutation=true时再检查Origin和CSRF。
3. ChangePassword 暂存首次会话拒绝并再查原回执，避免并发同号请求已改密后误报失效。
4. retry 允许失效原会话证明同一个操作，但仍核CSRF，不允许它发起新操作。

本文件需要守住的边界：

- 不能先发送Cookie再提交会话。
- 公开注册/重置结果读取必须匹配HMAC指纹，只有操作号不足以读取他人结果。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/authentication_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/authentication_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/authentication_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/authentication_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:468 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// authentication.go：保留原实现，按业务合并文件。

````

## 第 2 步：SessionSourceCapture

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type SessionSourceCapture func(context.Context, domain.Session, uint64)`。

**这一段为什么这样写：** 用函数能力将handler认证成功的事实传给输出保护，避免handler依赖保护器内部结构。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 本请求context，内含Protect创建的OutputBinding。 |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 真实Session，提供用户/凭据/会话来源代次与摘要。 |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 当前用户identity_epoch，来自Authenticate的权威查询。 |

<!-- write-step: 2 bytes=468:541 -->
````go
type SessionSourceCapture func(context.Context, domain.Session, uint64)

````

## 第 3 步：Authentication

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type Authentication struct {`。

**这一段为什么这样写：** 只适配HTTP输入输出，复用logic认证并记录响应来源。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.Authentication</code> | 登录/会话校验业务服务，掌握SQL提交。 |
| <code>origin</code> | <code>string</code> | 配置中精确允许的浏览器Origin，写入请求必须相等。 |
| <code>capture</code> | <code>SessionSourceCapture</code> | 会话来源捕获回调，将真实Session及IdentityEpoch写入同请求输出绑定。 |

<!-- write-step: 3 bytes=541:650 -->
````go
type Authentication struct {
	logic   *logic.Authentication
	origin  string
	capture SessionSourceCapture
}

````

## 第 4 步：NewAuthentication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func NewAuthentication(service *logic.Authentication, origin string, capture SessionSourceCapture) *Authentication`。

**这一段为什么这样写：** 装配HTTP层的业务、Origin和来源回调，不能省略capture。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.Authentication</code> | 共享的认证业务服务。<br>在此声明中的实际使用：<code>logic: service</code> |
| <code>origin</code> | <code>string</code> | 配置中允许的Origin。<br>在此声明中的实际使用：<code>origin: origin</code> |
| <code>capture</code> | <code>SessionSourceCapture</code> | 通常为CaptureSessionSources，记录三类输出来源。<br>在此声明中的实际使用：<code>capture: capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authentication</code> | HTTP认证handler集合。<br>实际返回表达式：<code>&amp;Authentication{logic: service, origin: origin, capture: capture}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authentication{logic: service, origin: origin, capture: capture}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=650:844 -->
````go
func NewAuthentication(service *logic.Authentication, origin string, capture SessionSourceCapture) *Authentication {
	return &Authentication{logic: service, origin: origin, capture: capture}
}

````

## 第 5 步：Authentication.Login

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func (h *Authentication) Login(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 只有业务Commit成功才能向浏览器发送可用凭据。

1. Origin检查后严格ReadJSON，失败立即return。
2. logic.Login成功后设置HttpOnly Session Cookie与可读CSRF Cookie，输出DTO中的用户ID/展示名/CSRF/到期时间。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 真正响应器或Outputs.Protect的暂存响应器。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>writeError(w, err)</code>；<code>httpx.SetCookie(w, httpx.SessionCookie, result.SessionToken, result.Expires, true)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前浏览器请求，携带Origin或主会话Cookie。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=844:1602 -->
````go
func (h *Authentication) Login(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Login
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	result, err := h.logic.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	// logic已提交会话；只有此时才交付令牌。CSRF Cookie用于刷新后恢复请求头。
	httpx.SetCookie(w, httpx.SessionCookie, result.SessionToken, result.Expires, true)
	httpx.SetCookie(w, httpx.CSRFCookie, result.CSRFToken, result.Expires, false)
	platform.WriteJSON(w, 200, dto.LoginResult{UserId: result.UserID, DisplayName: result.DisplayName, CsrfToken: result.CSRFToken, ExpiresAt: result.Expires.Format(time.RFC3339)})
}

````

## 第 6 步：Authentication.Authenticate

插入位置：第 5 步代码之后，紧接追加；完成稿约第 50 行。

Authenticate核验账号会话；组织角色仍由对应业务事务锁定并重检。

本步声明：`func (h *Authentication) Authenticate(w http.ResponseWriter, r *http.Request, mutation bool) (domain.Session, bool)`。

**这一段为什么这样写：** 认证与写入CSRF分开，读接口仍捕获来源，写接口额外防跨站。

1. logic.Authenticate读当前会话和IdentityEpoch。
2. capture在同context捕获来源；组织角色仍留给业务事务重检。
3. mutation时要求Origin相等并用摘要常量时间比较CSRF。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 鉴权失败时写安全HTTP错误的响应器。<br>在此声明中的实际使用：<code>writeError(w, err)</code>；<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.Failure(w, 403, &quot;CSRF_INVALID&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前请求；Cookie提供会话，写入时Header提供Origin与X-CSRF-Token。<br>在此声明中的实际使用：<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code>；<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ValidCSRF(r, value.Session)</code> |
| <code>mutation</code> | <code>bool</code> | true执行Origin与CSRF检查；false仅认证并捕获三类来源。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 原Session事实；仅返回bool=true后业务才可使用。<br>实际返回表达式：<code>value.Session</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | true表示认证及所需写入校验通过；false表示已写错误，调用者必须return。<br>实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return value.Session, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>mutation</code>。
- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return value.Session, false</code>。当前函数在这里结束。
- 检查 <code>!httpx.ValidCSRF(r, value.Session)</code>。
- 此处通过 <code>return value.Session, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return value.Session, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1602:2374 -->
````go
// Authenticate核验账号会话；组织角色仍由对应业务事务锁定并重检。
func (h *Authentication) Authenticate(w http.ResponseWriter, r *http.Request, mutation bool) (domain.Session, bool) {
	value, err := h.logic.Authenticate(r.Context(), httpx.CookieValue(r, httpx.SessionCookie))
	if err != nil {
		writeError(w, err)
		return value.Session, false
	}
	// 保留真实输出来源绑定；拆层后不能遗漏身份、凭据或会话的关闭代次。
	h.capture(r.Context(), value.Session, value.IdentityEpoch)
	if mutation {
		if !httpx.Origin(w, r, h.origin) {
			return value.Session, false
		}
		if !httpx.ValidCSRF(r, value.Session) {
			httpx.Failure(w, 403, "CSRF_INVALID")
			return value.Session, false
		}
	}
	return value.Session, true
}

````

## 第 7 步：Authentication.Me

插入位置：第 6 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (h *Authentication) Me(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 展示资料也要先鉴权并捕获撤权来源。

1. Authenticate(mutation=false)后CurrentUser再次核状态/凭据。
2. 以DTO.User输出字符串ID和代次；后续Outputs.Protect仍核来源。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 真正响应器或Outputs.Protect的暂存响应器。<br>在此声明中的实际使用：<code>h.Authenticate(w, r, false)</code>；<code>writeError(w, err)</code>；<code>platform.WriteJSON(w, 200, dto.User{Id: user.ID, Email: &amp;user.Email, DisplayName: user.DisplayName, State: user.State, CredentialEpoch: user.CredentialEpoch, IdentityEpoch: user.IdentityEpoch})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前浏览器请求，携带Origin或主会话Cookie。<br>在此声明中的实际使用：<code>h.Authenticate(w, r, false)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2374:2814 -->
````go
func (h *Authentication) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := h.Authenticate(w, r, false)
	if !ok {
		return
	}
	user, err := h.logic.CurrentUser(r.Context(), session)
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, dto.User{Id: user.ID, Email: &user.Email, DisplayName: user.DisplayName, State: user.State, CredentialEpoch: user.CredentialEpoch, IdentityEpoch: user.IdentityEpoch})
}

````

## 第 8 步：writeError

插入位置：第 7 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func writeError(w http.ResponseWriter, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 400, &quot;LOGIN_INPUT_INVALID&quot;)</code>；<code>httpx.Failure(w, 429, &quot;LOGIN_RATE_LIMITED&quot;)</code>；<code>httpx.Failure(w, 401, &quot;LOGIN_INVALID&quot;)</code>；<code>httpx.Failure(w, 401, &quot;SESSION_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, domain.ErrLoginInput)</code>；<code>errors.Is(err, domain.ErrLoginLimited)</code>；<code>errors.Is(err, domain.ErrLoginInvalid)</code>；<code>errors.Is(err, domain.ErrSessionInvalid)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

<!-- write-step: 8 bytes=2814:3372 -->
````go
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrLoginInput):
		httpx.Failure(w, 400, "LOGIN_INPUT_INVALID")
	case errors.Is(err, domain.ErrLoginLimited):
		httpx.Failure(w, 429, "LOGIN_RATE_LIMITED")
	case errors.Is(err, domain.ErrLoginInvalid):
		httpx.Failure(w, 401, "LOGIN_INVALID")
	case errors.Is(err, domain.ErrSessionInvalid):
		httpx.Failure(w, 401, "SESSION_INVALID")
	default:
		platform.WriteSafeError(w, platform.NewTraceID(), err)
	}
}

// challenge.go：保留原实现，按业务合并文件。

````

## 第 9 步：Challenges

插入位置：第 8 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`type Challenges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>logic</code> | <code>*logic.Challenges</code> | 使用 <code>logic.Challenges</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。 |
| <code>signups</code> | <code>*logic.Signups</code> | 使用 <code>logic.Signups</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。 |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，用于核对浏览器请求来源/构建本源链接；不能信任任意Host或输入URL。 |

<!-- write-step: 9 bytes=3372:3552 -->
````go
type Challenges struct {
	authentication *Authentication
	logic          *logic.Challenges
	signups        *logic.Signups
	secrets        *domain.Secrets
	origin         string
}

````

## 第 10 步：NewChallenges

插入位置：第 9 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func NewChallenges(authentication *Authentication, service *logic.Challenges, signups *logic.Signups, secrets *domain.Secrets, origin string) *Challenges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>service</code> | <code>*logic.Challenges</code> | 使用 <code>logic.Challenges</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>logic: service</code> |
| <code>signups</code> | <code>*logic.Signups</code> | 使用 <code>logic.Signups</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>signups: signups</code> |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>secrets: secrets</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Challenges</code> | 实际返回表达式：<code>&amp;Challenges{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Challenges{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3552:3831 -->
````go
func NewChallenges(authentication *Authentication, service *logic.Challenges, signups *logic.Signups, secrets *domain.Secrets, origin string) *Challenges {
	return &Challenges{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}
}

````

## 第 11 步：Challenges.Issue

插入位置：第 10 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func (h *Challenges) Issue(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;CHALLENGE_INPUT_INVALID&quot;)</code>；<code>h.authentication.Authenticate(w, r, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.authentication.Authenticate(w, r, true)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEmail(input.Email) &#124;&#124; (input.Purpose != &quot;SIGNUP&quot; &amp;&amp; input.Purpose != &quot;RESET_PASSWORD&quot; &amp;&amp; input.Purpose != &quot;CHANGE_EMAIL&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.Purpose == &quot;CHANGE_EMAIL&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>mode != &quot;&quot;</code>。

<!-- write-step: 11 bytes=3831:4949 -->
````go
func (h *Challenges) Issue(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.ChallengeCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !domain.ValidEmail(input.Email) || (input.Purpose != "SIGNUP" && input.Purpose != "RESET_PASSWORD" && input.Purpose != "CHANGE_EMAIL") {
		httpx.Failure(w, 400, "CHALLENGE_INPUT_INVALID")
		return
	}
	var session *domain.Session
	if input.Purpose == "CHANGE_EMAIL" {
		current, ok := h.authentication.Authenticate(w, r, true)
		if !ok {
			return
		}
		session = &current
	}
	issued, err := h.logic.Issue(r.Context(), input, session)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	// 成功仅表示挑战已受理/邮件服务器已确认DATA；头不含SMTP主机、凭据或邮箱存在性。
	if mode := h.logic.DeliveryMode(); mode != "" {
		w.Header().Set("X-Collaboration-Mail-Delivery", mode)
	}
	platform.WriteJSON(w, 200, dto.Challenge{ChallengeId: issued.ID, ExpiresAt: issued.Expires.Format(time.RFC3339), RetryAfterSeconds: 60})
}

````

## 第 12 步：Challenges.Signup

插入位置：第 11 步代码之后，紧接追加；完成稿约第 146 行。

本步声明：`func (h *Challenges) Signup(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;SIGNUP_INPUT_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId) &#124;&#124; !domain.ValidEmail(input.Email) &#124;&#124; !domain.ValidPassword(input.Password) &#124;&#124; len(&#91;&#93;rune(input.DisplayName)) &lt; 1 &#124;&#124; len(&#91;&#93;rune(input.DisplayName)) &gt; 100</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>found</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4949:6116 -->
````go
func (h *Challenges) Signup(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Signup
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidEmail(input.Email) || !domain.ValidPassword(input.Password) || len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 100 {
		httpx.Failure(w, 400, "SIGNUP_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	// 公开重取仅返回原事实；指纹含密码/验证码的HMAC，不能靠操作号读取他人结果。
	raw, found, err := h.signups.PublicResult(r.Context(), input.OperationId, "SIGNUP", fingerprint)
	if found {
		if err != nil {
			httpx.WriteError(w, err)
		} else {
			platform.WriteJSON(w, 200, raw)
		}
		return
	}
	raw, err = h.signups.Register(r.Context(), input, fingerprint)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, raw)
}

// credential.go：保留原实现，按业务合并文件。

````

## 第 13 步：Credentials

插入位置：第 12 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`type Credentials struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>logic</code> | <code>*logic.Credentials</code> | 使用 <code>logic.Credentials</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。 |
| <code>signups</code> | <code>*logic.Signups</code> | 使用 <code>logic.Signups</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。 |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，用于核对浏览器请求来源/构建本源链接；不能信任任意Host或输入URL。 |

<!-- write-step: 13 bytes=6116:6298 -->
````go
type Credentials struct {
	authentication *Authentication
	logic          *logic.Credentials
	signups        *logic.Signups
	secrets        *domain.Secrets
	origin         string
}

````

## 第 14 步：NewCredentials

插入位置：第 13 步代码之后，紧接追加；完成稿约第 189 行。

本步声明：`func NewCredentials(authentication *Authentication, service *logic.Credentials, signups *logic.Signups, secrets *domain.Secrets, origin string) *Credentials`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>service</code> | <code>*logic.Credentials</code> | 使用 <code>logic.Credentials</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>logic: service</code> |
| <code>signups</code> | <code>*logic.Signups</code> | 使用 <code>logic.Signups</code> 类型；其定义处的职责：编排登录、会话验证、验证码签发、注册和凭据变更。<br>在此声明中的实际使用：<code>signups: signups</code> |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>secrets: secrets</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Credentials</code> | 实际返回表达式：<code>&amp;Credentials{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Credentials{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6298:6581 -->
````go
func NewCredentials(authentication *Authentication, service *logic.Credentials, signups *logic.Signups, secrets *domain.Secrets, origin string) *Credentials {
	return &Credentials{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}
}

````

## 第 15 步：Credentials.ResetPassword

插入位置：第 14 步代码之后，紧接追加；完成稿约第 193 行。

本步声明：`func (h *Credentials) ResetPassword(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;RESET_INPUT_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId) &#124;&#124; !domain.ValidPassword(input.NewPassword)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>found</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6581:7337 -->
````go
func (h *Credentials) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.ResetPassword
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidPassword(input.NewPassword) {
		httpx.Failure(w, 400, "RESET_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	raw, found, err := h.signups.PublicResult(r.Context(), input.OperationId, "RESET_PASSWORD", fingerprint)
	if found {
		if err != nil {
			httpx.WriteError(w, err)
		} else {
			platform.WriteJSON(w, 200, raw)
		}
		return
	}
	state, err := h.logic.ResetPassword(r.Context(), input, fingerprint)
	writeCredential(w, 200, input.OperationId, state, err)
}

````

## 第 16 步：Credentials.ChangePassword

插入位置：第 15 步代码之后，紧接追加；完成稿约第 219 行。

本步声明：`func (h *Credentials) ChangePassword(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;PASSWORD_INPUT_INVALID&quot;)</code>；<code>h.retry(w, r, input.OperationId, &quot;CHANGE_PASSWORD&quot;, fingerprint)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.retry(w, r, input.OperationId, &quot;CHANGE_PASSWORD&quot;, fingerprint)</code>；<code>h.authentication.Authenticate(capture, r, true)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId) &#124;&#124; !domain.ValidPassword(input.NewPassword) &#124;&#124; !domain.ValidPassword(input.CurrentPassword)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.retry(w, r, input.OperationId, &quot;CHANGE_PASSWORD&quot;, fingerprint)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。
- 检查 <code>h.retry(w, r, input.OperationId, &quot;CHANGE_PASSWORD&quot;, fingerprint)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;rejected) &amp;&amp; rejected.Code == &quot;CURRENT_PASSWORD_INVALID&quot;</code>。
- 检查 <code>h.retry(w, r, input.OperationId, &quot;CHANGE_PASSWORD&quot;, fingerprint)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=7337:8567 -->
````go
func (h *Credentials) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.PasswordChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidPassword(input.NewPassword) || !domain.ValidPassword(input.CurrentPassword) {
		httpx.Failure(w, 400, "PASSWORD_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
		return
	}
	capture := httpx.NewResponseCapture()
	session, ok := h.authentication.Authenticate(capture, r, true)
	if !ok {
		// 等待另一个同号请求提交后，失效会话仍能续取原结果；暂存身份拒绝不能抢先输出。
		if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
			return
		}
		capture.CopyTo(w)
		return
	}
	state, err := h.logic.ChangePassword(r.Context(), session, input, fingerprint)
	var rejected domain.RuleViolation
	if errors.As(err, &rejected) && rejected.Code == "CURRENT_PASSWORD_INVALID" {
		if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
			return
		}
	}
	writeCredential(w, 202, input.OperationId, state, err)
}

````

## 第 17 步：Credentials.ChangeEmail

插入位置：第 16 步代码之后，紧接追加；完成稿约第 255 行。

本步声明：`func (h *Credentials) ChangeEmail(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;EMAIL_INPUT_INVALID&quot;)</code>；<code>h.retry(w, r, input.OperationId, &quot;CHANGE_EMAIL&quot;, fingerprint)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.Origin(w, r, h.origin)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.retry(w, r, input.OperationId, &quot;CHANGE_EMAIL&quot;, fingerprint)</code>；<code>h.authentication.Authenticate(w, r, true)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.Origin(w, r, h.origin)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId) &#124;&#124; !domain.ValidEmail(input.NewEmail)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.retry(w, r, input.OperationId, &quot;CHANGE_EMAIL&quot;, fingerprint)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=8567:9323 -->
````go
func (h *Credentials) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.EmailChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.NewEmail = strings.ToLower(strings.TrimSpace(input.NewEmail))
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidEmail(input.NewEmail) {
		httpx.Failure(w, 400, "EMAIL_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	if h.retry(w, r, input.OperationId, "CHANGE_EMAIL", fingerprint) {
		return
	}
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return
	}
	state, err := h.logic.ChangeEmail(r.Context(), session, input, fingerprint)
	writeCredential(w, 202, input.OperationId, state, err)
}

````

## 第 18 步：Credentials.retry

插入位置：第 17 步代码之后，紧接追加；完成稿约第 280 行。

本步声明：`func (h *Credentials) retry(w http.ResponseWriter, r *http.Request, id, kind string, fingerprint []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.WriteError(w, err)</code>；<code>httpx.Failure(w, 403, &quot;CSRF_INVALID&quot;)</code>；<code>platform.WriteJSON(w, 202, ProgressDTO(id, value.State))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code>；<code>httpx.CookieValue(r, httpx.SessionCookie)</code>；<code>httpx.ValidCSRF(r, value.Session)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>h.logic.Retry(r.Context(), id, kind, fingerprint, httpx.CookieValue(r, httpx.SessionCookie))</code>；<code>ProgressDTO(id, value.State)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>h.logic.Retry(r.Context(), id, kind, fingerprint, httpx.CookieValue(r, httpx.SessionCookie))</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>h.logic.Retry(r.Context(), id, kind, fingerprint, httpx.CookieValue(r, httpx.SessionCookie))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!found</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ValidCSRF(r, value.Session)</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9323:9814 -->
````go
func (h *Credentials) retry(w http.ResponseWriter, r *http.Request, id, kind string, fingerprint []byte) bool {
	value, found, err := h.logic.Retry(r.Context(), id, kind, fingerprint, httpx.CookieValue(r, httpx.SessionCookie))
	if !found {
		return false
	}
	if err != nil {
		httpx.WriteError(w, err)
		return true
	}
	if !httpx.ValidCSRF(r, value.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return true
	}
	platform.WriteJSON(w, 202, ProgressDTO(id, value.State))
	return true
}

````

## 第 19 步：writeCredential

插入位置：第 18 步代码之后，紧接追加；完成稿约第 297 行。

本步声明：`func writeCredential(w http.ResponseWriter, status int, id, state string, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, status, ProgressDTO(id, state))</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>platform.WriteJSON(w, status, ProgressDTO(id, state))</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>ProgressDTO(id, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>ProgressDTO(id, state)</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err != nil</code>；<code>httpx.WriteError(w, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=9814:10014 -->
````go
func writeCredential(w http.ResponseWriter, status int, id, state string, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, status, ProgressDTO(id, state))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/authentication_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
