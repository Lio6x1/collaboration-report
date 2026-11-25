# 分步编写 `services/account/internal/handler/account/common_handlers.go`

先按 [A02 · 短期身份准入、重验与幂等写入](../lessons/account/02-identity-admission.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/071-common-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

统一分页参数与供 Document 使用的实际会话校验出口。

分页游标需绑定查询权限范围；内部服务不能直读Account私有数据库。

写之前先按这条执行路线理解各段的连接：

1. Pagination.Page 默认20条，只允许1至100，随后用binding验证签名cursor。
2. Authentication.VerifySession 只接收document证书，按 X-Internal-Mutation 决定是否校验写入Origin/CSRF，返回actor与会话绝对期限。

本文件需要守住的边界：

- 短期准入期限不是会话寿命；不能用它延长Session。
- 分页非法时已经写错误响应，调用方必须检查返回bool并停止。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/common_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/common_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/common_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/common_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:340 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"net/http"
	"strconv"
)

// pagination.go：保留原实现，按业务合并文件。

````

## 第 2 步：Pagination

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type Pagination struct{ cursors *logic.Cursors }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cursors</code> | <code>*logic.Cursors</code> | 使用 <code>logic.Cursors</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |

<!-- write-step: 2 bytes=340:390 -->
````go
type Pagination struct{ cursors *logic.Cursors }

````

## 第 3 步：NewPagination

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func NewPagination(cursors *logic.Cursors) *Pagination`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cursors</code> | <code>*logic.Cursors</code> | 使用 <code>logic.Cursors</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>cursors: cursors</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Pagination</code> | 实际返回表达式：<code>&amp;Pagination{cursors: cursors}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Pagination{cursors: cursors}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=390:487 -->
````go
func NewPagination(cursors *logic.Cursors) *Pagination { return &Pagination{cursors: cursors} }

````

## 第 4 步：Pagination.Page

插入位置：第 3 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func (h *Pagination) Page(w http.ResponseWriter, r *http.Request, binding string) (int, string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 400, &quot;PAGE_LIMIT_INVALID&quot;)</code>；<code>httpx.Failure(w, 400, &quot;CURSOR_INVALID&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.URL</code> |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>h.cursors.Decode(r.URL.Query().Get(&quot;cursor&quot;), binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>0</code>；<code>limit</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>last</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>value != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; parsed &lt; 1 &#124;&#124; parsed &gt; 100</code>。
- 此处通过 <code>return 0, &quot;&quot;, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!valid</code>。
- 此处通过 <code>return 0, &quot;&quot;, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return limit, last, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=487:1018 -->
````go
func (h *Pagination) Page(w http.ResponseWriter, r *http.Request, binding string) (int, string, bool) {
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			httpx.Failure(w, 400, "PAGE_LIMIT_INVALID")
			return 0, "", false
		}
		limit = parsed
	}
	last, valid := h.cursors.Decode(r.URL.Query().Get("cursor"), binding)
	if !valid {
		httpx.Failure(w, 400, "CURSOR_INVALID")
		return 0, "", false
	}
	return limit, last, true
}

````

## 第 5 步：Pagination.Next

插入位置：第 4 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func (h *Pagination) Next(binding, last string) *string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>h.cursors.Encode(binding, last)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>h.cursors.Encode(binding, last)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*string</code> | 实际返回表达式：<code>h.cursors.Encode(binding, last)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return h.cursors.Encode(binding, last)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1018:1185 -->
````go
func (h *Pagination) Next(binding, last string) *string { return h.cursors.Encode(binding, last) }

// verify_session.go：保留原实现，按业务合并文件。

````

## 第 6 步：Authentication.VerifySession

插入位置：第 5 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func (h *Authentication) VerifySession(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>h.Authenticate(w, r, r.Header.Get(&quot;X-Internal-Mutation&quot;) == &quot;true&quot;)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;any{&quot;actor_id&quot;: session.UserID, &quot;session_expires_unix_ms&quot;: session.Expires.UnixMilli()})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>h.Authenticate(w, r, r.Header.Get(&quot;X-Internal-Mutation&quot;) == &quot;true&quot;)</code>；<code>r.Header</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1185:1821 -->
````go
func (h *Authentication) VerifySession(w http.ResponseWriter, r *http.Request) {
	// 文档服务不能读账号库；证书保护端点只核验真实会话/CSRF，组织资格仍由固定RPC核验。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	session, ok := h.Authenticate(w, r, r.Header.Get("X-Internal-Mutation") == "true")
	if !ok {
		return
	}
	// grant须早于会话绝对到期关闭；短期身份准入不能冒充会话寿命。
	platform.WriteJSON(w, 200, map[string]any{"actor_id": session.UserID, "session_expires_unix_ms": session.Expires.UnixMilli()})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/common_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/02-identity-admission.md)。
