# 分步编写 `services/account/internal/handler/httpx/http_transport_handlers.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/078-http-transport-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把 Cookie、Origin、CSRF、严格 JSON 和安全错误映射集中为 HTTP 基础适配。

传输校验要在业务入口前统一执行；ResponseCapture 暂存响应，让输出资格检查有机会在真正写字节前拒绝。

写之前先按这条执行路线理解各段的连接：

1. SetCookie 设置 __Host- 所需 Secure、Path=/ 与 SameSiteStrict；CSRF摘要与请求头常量时间比较。
2. ReadJSON 限16KiB、只接受单个对象，按 dto.RequiredFields 区分字段缺失与null，最后拒绝未知字段。
3. WriteError 将稳定领域错误映射状态；ResponseCapture.CopyTo 在批准后复制头、状态和正文。

本文件需要守住的边界：

- 不能把缺失的bool当false执行撤权；不能接受尾随第二份JSON。
- Session Cookie 必须HttpOnly，CSRF Cookie 可供前端读取，二者不能共用秘密。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/httpx' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/httpx/http_transport_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/httpx/http_transport_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/httpx/http_transport_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/httpx/http_transport_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:459 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package httpx

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"reflect"
	"time"
)

// cookie.go：保留原实现，按业务合并文件。

````

## 第 2 步：SessionCookie、CSRFCookie、RecoveryCookie

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=459:591 -->
````go
const (
	SessionCookie  = "__Host-collabSession"
	CSRFCookie     = "__Host-collabCSRF"
	RecoveryCookie = "__Host-collabRecovery"
)

````

## 第 3 步：SetCookie

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func SetCookie(w http.ResponseWriter, name, value string, expires time.Time, httpOnly bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>http.SetCookie(w, &amp;http.Cookie{Name: name, Value: value, Path: &quot;/&quot;, Expires: expires, Secure: true, HttpOnly: httpOnly, SameSite: http.SameSiteStrictMode})</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>Name: name</code> |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>Value: value</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>Expires: expires</code> |
| <code>httpOnly</code> | <code>bool</code> | 在此声明中的实际使用：<code>HttpOnly: httpOnly</code> |

<!-- write-step: 3 bytes=591:845 -->
````go
func SetCookie(w http.ResponseWriter, name, value string, expires time.Time, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Expires: expires, Secure: true, HttpOnly: httpOnly, SameSite: http.SameSiteStrictMode})
}

````

## 第 4 步：ClearCookie

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func ClearCookie(w http.ResponseWriter, name string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>http.SetCookie(w, &amp;http.Cookie{Name: name, Path: &quot;/&quot;, Secure: true, HttpOnly: name != CSRFCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>Name: name</code>；<code>name != CSRFCookie</code> |

<!-- write-step: 4 bytes=845:1050 -->
````go
func ClearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", Secure: true, HttpOnly: name != CSRFCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

````

## 第 5 步：CookieValue

插入位置：第 4 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func CookieValue(r *http.Request, name string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Cookie</code> |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>r.Cookie(name)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>cookie.Value</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return cookie.Value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1050:1193 -->
````go
func CookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

````

## 第 6 步：ValidCSRF

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func ValidCSRF(r *http.Request, session domain.Session) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Header</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.CSRF</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; subtle.ConstantTimeCompare(hash, session.CSRF) == 1</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err == nil &amp;&amp; subtle.ConstantTimeCompare(hash, session.CSRF) == 1</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1193:1394 -->
````go
func ValidCSRF(r *http.Request, session domain.Session) bool {
	hash, err := domain.TokenHash(r.Header.Get("X-CSRF-Token"))
	return err == nil && subtle.ConstantTimeCompare(hash, session.CSRF) == 1
}

````

## 第 7 步：Origin

插入位置：第 6 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func Origin(w http.ResponseWriter, r *http.Request, expected string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>Failure(w, 403, &quot;ORIGIN_FORBIDDEN&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Header</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>r.Header.Get(&quot;Origin&quot;) != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>r.Header.Get(&quot;Origin&quot;) != expected</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1394:1641 -->
````go
func Origin(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Header.Get("Origin") != expected {
		Failure(w, 403, "ORIGIN_FORBIDDEN")
		return false
	}
	return true
}

// error.go：保留原实现，按业务合并文件。

````

## 第 8 步：WriteError

插入位置：第 7 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func WriteError(w http.ResponseWriter, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>Failure(w, http.StatusBadRequest, domain.ErrChallengeInvalid.Error())</code>；<code>Failure(w, http.StatusTooManyRequests, domain.ErrChallengeRateLimited.Error())</code>；<code>Failure(w, http.StatusServiceUnavailable, known.Error())</code>；<code>Failure(w, status, rejected.Code)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, domain.ErrChallengeInvalid)</code>；<code>errors.Is(err, domain.ErrChallengeRateLimited)</code>；<code>errors.Is(err, known)</code>；<code>errors.As(err, &amp;rejected)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>errors.Is(err, known)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>errors.As(err, &amp;rejected)</code>。
- 检查 <code>status != 0</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1641:3046 -->
````go
func WriteError(w http.ResponseWriter, err error) {
	// 通过错误链判断稳定业务类别，外层补充诊断上下文不会把限流/无效验证码误报为503。
	switch {
	case errors.Is(err, domain.ErrChallengeInvalid):
		Failure(w, http.StatusBadRequest, domain.ErrChallengeInvalid.Error())
		return
	case errors.Is(err, domain.ErrChallengeRateLimited):
		Failure(w, http.StatusTooManyRequests, domain.ErrChallengeRateLimited.Error())
		return
	}
	// 仅透传已定义的故障类别，SMTP/Redis的原始报错可能含主机或凭据，日志同样不记录原文。
	for _, known := range []error{domain.ErrChallengeStoreUnavailable, domain.ErrSMTPDeliveryFailed} {
		if errors.Is(err, known) {
			slog.Warn("account_request_unavailable", "code", known.Error())
			Failure(w, http.StatusServiceUnavailable, known.Error())
			return
		}
	}
	var rejected domain.RuleViolation
	if errors.As(err, &rejected) {
		status := map[domain.RuleFailure]int{
			domain.InvalidRequest:         400,
			domain.SessionRequired:        401,
			domain.PermissionDenied:       403,
			domain.EntityNotFound:         404,
			domain.StateConflict:          409,
			domain.TemporarilyUnavailable: 503,
		}[rejected.Kind]
		if status != 0 {
			Failure(w, status, rejected.Code)
			return
		}
	}
	platform.WriteSafeError(w, platform.NewTraceID(), err)
}

// path.go：保留原实现，按业务合并文件。

````

## 第 9 步：PathID

插入位置：第 8 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func PathID(w http.ResponseWriter, r *http.Request, key string) (string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>Failure(w, 400, &quot;ENTITY_ID_INVALID&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>id := pathvar.Vars(r)&#91;key&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidEntityID(id)</code>。
- 此处通过 <code>return &quot;&quot;, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return id, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3046:3328 -->
````go
func PathID(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	id := pathvar.Vars(r)[key]
	if !domain.ValidEntityID(id) {
		Failure(w, 400, "ENTITY_ID_INVALID")
		return "", false
	}
	return id, true
}

// request.go：保留原实现，按业务合并文件。

````

## 第 10 步：ReadJSON

插入位置：第 9 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func ReadJSON(w http.ResponseWriter, r *http.Request, value any) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>Failure(w, 400, &quot;REQUEST_INVALID&quot;)</code>；<code>http.MaxBytesReader(w, r.Body, 16384)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Header</code>；<code>r.Body</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>reflect.ValueOf(value)</code>；<code>strict.Decode(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; kind != &quot;application/json&quot;</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;raw) != nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;extra) != io.EOF</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.Unmarshal(raw, &amp;fields) != nil &#124;&#124; fields == nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!exists &#124;&#124; (!nullable &amp;&amp; bytes.Equal(bytes.TrimSpace(field), &#91;&#93;byte(&quot;null&quot;)))</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>strict.Decode(value) != nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3328:4690 -->
````go
func ReadJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if decoder.Decode(&raw) != nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	// 生成契约是required/nullability的唯一来源，缺失布尔不能误解成撤销请求。
	model := reflect.Indirect(reflect.ValueOf(value)).Type().Name()
	for key, nullable := range dto.RequiredFields[model] {
		field, exists := fields[key]
		if !exists || (!nullable && bytes.Equal(bytes.TrimSpace(field), []byte("null"))) {
			Failure(w, 400, "REQUEST_INVALID")
			return false
		}
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if strict.Decode(value) != nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	return true
}

// response.go：保留原实现，按业务合并文件。

````

## 第 11 步：Failure

插入位置：第 10 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func Failure(w http.ResponseWriter, status int, code string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: &quot;请求未满足当前操作条件&quot;, TraceID: platform.NewTraceID(), RetryAction: retry})</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>status == 401</code>；<code>platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: &quot;请求未满足当前操作条件&quot;, TraceID: platform.NewTraceID(), RetryAction: retry})</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |

**跟着本段代码走一遍：**

- 检查 <code>status == 401</code>。

<!-- write-step: 11 bytes=4690:5050 -->
````go
func Failure(w http.ResponseWriter, status int, code string) {
	retry := "NONE"
	if status == 401 {
		retry = "RELOGIN"
	}
	platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: "请求未满足当前操作条件", TraceID: platform.NewTraceID(), RetryAction: retry})
}

// response_capture.go：保留原实现，按业务合并文件。

````

## 第 12 步：ResponseCapture

插入位置：第 11 步代码之后，紧接追加；完成稿约第 162 行。

ResponseCapture暂存短JSON，输出资格核验成功后才交给真实ResponseWriter。

本步声明：`type ResponseCapture struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>header</code> | <code>http.Header</code> | 暂存下游handler将写出的响应头。 |
| <code>status</code> | <code>int</code> | 暂存HTTP状态码，在最终响应放行前不直接发送。 |
| <code>body</code> | <code>bytes.Buffer</code> | 暂存响应正文的字节缓冲；用来统一包装/核验响应，不作为跨请求缓存。 |

<!-- write-step: 12 bytes=5050:5230 -->
````go
// ResponseCapture暂存短JSON，输出资格核验成功后才交给真实ResponseWriter。
type ResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

````

## 第 13 步：NewResponseCapture

插入位置：第 12 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func NewResponseCapture() *ResponseCapture`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseCapture</code> | 使用 <code>ResponseCapture</code> 类型；其定义处的职责：ResponseCapture暂存短JSON，输出资格核验成功后才交给真实ResponseWriter。<br>实际返回表达式：<code>&amp;ResponseCapture{header: make(http.Header)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ResponseCapture{header: make(http.Header)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5230:5329 -->
````go
func NewResponseCapture() *ResponseCapture { return &ResponseCapture{header: make(http.Header)} }

````

## 第 14 步：ResponseCapture.Header

插入位置：第 13 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func (c *ResponseCapture) Header() http.Header`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>http.Header</code> | 实际返回表达式：<code>c.header</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.header</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5329:5408 -->
````go
func (c *ResponseCapture) Header() http.Header             { return c.header }
````

## 第 15 步：ResponseCapture.WriteHeader

插入位置：第 14 步代码之后，紧接追加；完成稿约第 172 行。

本步声明：`func (c *ResponseCapture) WriteHeader(status int)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>c.status = status</code> |

<!-- write-step: 15 bytes=5408:5489 -->
````go
func (c *ResponseCapture) WriteHeader(status int)          { c.status = status }
````

## 第 16 步：ResponseCapture.Write

插入位置：第 15 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func (c *ResponseCapture) Write(value []byte) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>c.body.Write(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 由 <code>c.body.Write(value)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>c.body.Write(value)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.body.Write(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=5489:5579 -->
````go
func (c *ResponseCapture) Write(value []byte) (int, error) { return c.body.Write(value) }
````

## 第 17 步：ResponseCapture.Body

插入位置：第 16 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func (c *ResponseCapture) Body() []byte`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>c.body.Bytes()</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.body.Bytes()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=5579:5664 -->
````go
func (c *ResponseCapture) Body() []byte                    { return c.body.Bytes() }
````

## 第 18 步：ResponseCapture.Size

插入位置：第 17 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`func (c *ResponseCapture) Size() int`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>c.body.Len()</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.body.Len()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=5664:5748 -->
````go
func (c *ResponseCapture) Size() int                       { return c.body.Len() }

````

## 第 19 步：ResponseCapture.Status

插入位置：第 18 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`func (c *ResponseCapture) Status() int`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>http.StatusOK</code>；<code>c.status</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.status == 0</code>。条件成立时执行 <code>return http.StatusOK</code>。当前函数在这里结束。
- 此处通过 <code>return c.status</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=5748:5855 -->
````go
func (c *ResponseCapture) Status() int {
	if c.status == 0 {
		return http.StatusOK
	}
	return c.status
}

````

## 第 20 步：ResponseCapture.CopyTo

插入位置：第 19 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func (c *ResponseCapture) CopyTo(w http.ResponseWriter)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>w.Header</code>；<code>w.WriteHeader</code>；<code>w.Write</code> |

<!-- write-step: 20 bytes=5855:6041 -->
````go
func (c *ResponseCapture) CopyTo(w http.ResponseWriter) {
	for key, values := range c.header {
		w.Header()[key] = values
	}
	w.WriteHeader(c.Status())
	_, _ = w.Write(c.body.Bytes())
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/httpx/http_transport_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
