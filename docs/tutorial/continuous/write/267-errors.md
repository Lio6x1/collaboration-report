# 分步编写 `services/document/internal/handler/httpx/errors.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/267-errors.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将领域错误翻译成安全HTTP响应，并以有限日志信息记录依赖失败。

公开响应不能暴露SQL、文件路径或上游错误文本，排错仍需要稳定码和数据库编号。

写之前先按这条执行路线理解各段的连接：

1. Failure用errors.As识别RuleViolation并映射400/401/403/404/409
2. 未知错误走WriteSafeError
3. safeFailureCode只允许短大写固定码进入日志。

本文件需要守住的边界：

- 不要把所有失败返回200
- Fail的TraceID用于定位请求，不能把原始err.Error直接当公共消息。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/httpx' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/httpx/errors.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/httpx/errors.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/httpx/errors.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/httpx/errors.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:190 -->
````go
package httpx

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-sql-driver/mysql"
)

````

## 第 2 步：Reject

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

Reject兼容正在迁移的HTTP入口；新业务用例直接返回领域类别。

本步声明：`func Reject(status int, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>status</code> | <code>int</code> | 需要表达的HTTP拒绝状态，映射为统一业务错误类别；未知状态按暂时不可用处理。 |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.Violation(kind, code)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(kind, code)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(kind, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=190:632 -->
````go
// Reject兼容正在迁移的HTTP入口；新业务用例直接返回领域类别。
func Reject(status int, code string) error {
	kind := domain.TemporarilyUnavailable
	switch status {
	case 400:
		kind = domain.InvalidRequest
	case 401:
		kind = domain.SessionRequired
	case 403:
		kind = domain.PermissionDenied
	case 404:
		kind = domain.EntityNotFound
	case 409:
		kind = domain.StateConflict
	}
	return domain.Violation(kind, code)
}

````

## 第 3 步：Fail

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func Fail(w http.ResponseWriter, status int, code string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: &quot;请求未满足当前操作条件&quot;, TraceID: platform.NewTraceID(), RetryAction: &quot;NONE&quot;})</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: &quot;请求未满足当前操作条件&quot;, TraceID: platform.NewTraceID(), RetryAction: &quot;NONE&quot;})</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |

<!-- write-step: 3 bytes=632:861 -->
````go
func Fail(w http.ResponseWriter, status int, code string) {
	platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: "请求未满足当前操作条件", TraceID: platform.NewTraceID(), RetryAction: "NONE"})
}

````

## 第 4 步：Failure

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func Failure(w http.ResponseWriter, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>platform.WriteSafeError(w, platform.NewTraceID(), err)</code>；<code>Fail(w, status, violation.Code)</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>err.Error</code>；<code>errors.As(err, &amp;databaseError)</code>；<code>platform.WriteSafeError(w, platform.NewTraceID(), err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!errors.As(err, &amp;violation)</code>。
- 检查 <code>safeFailureCode.MatchString(err.Error())</code>。
- 检查 <code>errors.As(err, &amp;databaseError)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=861:1815 -->
````go
func Failure(w http.ResponseWriter, err error) {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		// 公共响应不泄露依赖细节；日志只保留固定错误码或数据库编号，便于定位真实故障。
		code := "DEPENDENCY_FAILURE"
		if safeFailureCode.MatchString(err.Error()) {
			code = err.Error()
		}
		var databaseError *mysql.MySQLError
		if errors.As(err, &databaseError) {
			slog.Warn("document_request_failed", "code", code, "database_number", databaseError.Number)
		} else {
			slog.Warn("document_request_failed", "code", code)
		}
		platform.WriteSafeError(w, platform.NewTraceID(), err)
		return
	}
	status := 503
	switch violation.Kind {
	case domain.InvalidRequest:
		status = 400
	case domain.SessionRequired:
		status = 401
	case domain.PermissionDenied:
		status = 403
	case domain.EntityNotFound:
		status = 404
	case domain.StateConflict:
		status = 409
	}
	Fail(w, status, violation.Code)
}

````

## 第 5 步：safeFailureCode

插入位置：第 4 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`var safeFailureCode = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{1,63}$\`)`。

<!-- write-step: 5 bytes=1815:1882 -->
````go
var safeFailureCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/httpx/errors.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
