# 分步编写 `services/document/internal/worker/personal_transfer.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/274-personal-transfer.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

循环恢复已接受转移并记录安全重试码。

响应丢失和服务退出不应要求用户再次接受。

写之前先按这条执行路线理解各段的连接：

1. RunPersonalTransfers响应context退出并调用Recover
2. transferRetryCode保留固定业务码或SQL编号。

本文件需要守住的边界：

- 日志不带身份证明、连接信息与SQL参数
- 重试不能生成新邀约。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/worker/personal_transfer.go') -and (Get-Item -LiteralPath 'services/document/internal/worker/personal_transfer.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/worker/personal_transfer.go')) { New-Item -ItemType File -Path 'services/document/internal/worker/personal_transfer.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:238 -->
````go
package worker

import (
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"github.com/go-sql-driver/mysql"
	"log/slog"
	"regexp"
	"strconv"
	"time"
)

````

## 第 2 步：RunPersonalTransfers

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

服务装配显式启动原号恢复；HTTP 丢回复和实例退出不依赖用户重新接受。

本步声明：`func RunPersonalTransfers(ctx context.Context, usecase *logic.PersonalTransfers)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>ctx.Err</code> |
| <code>usecase</code> | <code>*logic.PersonalTransfers</code> | 使用 <code>logic.PersonalTransfers</code> 类型；其定义处的职责：提供个人文档邀约、参与者列表、取消拒绝和接受登记。<br>在此声明中的实际使用：<code>usecase.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, logic.ErrPersonalTransferIdle) &amp;&amp; ctx.Err() == nil</code>。
- 检查 <code>code != lastCode</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 2 bytes=238:979 -->
````go
// 服务装配显式启动原号恢复；HTTP 丢回复和实例退出不依赖用户重新接受。
func RunPersonalTransfers(ctx context.Context, usecase *logic.PersonalTransfers) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastCode := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := usecase.Recover(attempt)
			cancel()
			if err != nil && !errors.Is(err, logic.ErrPersonalTransferIdle) && ctx.Err() == nil {
				code := transferRetryCode(err)
				if code != lastCode {
					slog.Warn("PERSONAL_TRANSFER_RECOVERY_PENDING", "code", code)
					lastCode = code
				}
			} else if err == nil {
				lastCode = ""
			}
		}
	}
}

````

## 第 3 步：stableTransferError

插入位置：第 2 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`var stableTransferError = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{1,95}$\`)`。

<!-- write-step: 3 bytes=979:1051 -->
````go
var stableTransferError = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,95}$`)

````

## 第 4 步：transferRetryCode

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

只暴露稳定业务码/SQL错误编号，不能把固定身份、连接信息或SQL参数写入日志。

本步声明：`func transferRetryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>errors.As(err, &amp;sqlError)</code>；<code>err.Error</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>violation.Code</code>；<code>&quot;SQL_ERROR_&quot; + strconv.Itoa(int(sqlError.Number))</code>；<code>err.Error()</code>；<code>&quot;PERSONAL_TRANSFER_RETRY&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return violation.Code</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;sqlError)</code>。条件成立时执行 <code>return &quot;SQL_ERROR_&quot; + strconv.Itoa(int(sqlError.Number))</code>。当前函数在这里结束。
- 检查 <code>stableTransferError.MatchString(err.Error())</code>。条件成立时执行 <code>return err.Error()</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;PERSONAL_TRANSFER_RETRY&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1051:1537 -->
````go
// 只暴露稳定业务码/SQL错误编号，不能把固定身份、连接信息或SQL参数写入日志。
func transferRetryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var sqlError *mysql.MySQLError
	if errors.As(err, &sqlError) {
		return "SQL_ERROR_" + strconv.Itoa(int(sqlError.Number))
	}
	if stableTransferError.MatchString(err.Error()) {
		return err.Error()
	}
	return "PERSONAL_TRANSFER_RETRY"
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/worker/personal_transfer.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
