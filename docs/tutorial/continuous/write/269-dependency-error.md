# 分步编写 `services/document/internal/server/dependency_error.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/269-dependency-error.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把RPC边界的SQL、网络与取消错误变成安全协议码。

依赖错误可能携带连接详情，不能跨服务直接传原字符串。

写之前先按这条执行路线理解各段的连接：

1. dependencyError保留允许的固定协议码，区分context取消/超时与通用依赖失败。

本文件需要守住的边界：

- 错误翻译不能改变领域拒绝、原操作状态或关闭ACK含义。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/server' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/server/dependency_error.go') -and (Get-Item -LiteralPath 'services/document/internal/server/dependency_error.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/server/dependency_error.go')) { New-Item -ItemType File -Path 'services/document/internal/server/dependency_error.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:125 -->
````go
package server

import (
	"context"
	"errors"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

````

## 第 2 步：publicRPCCode

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`var publicRPCCode = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{0,95}$\`)`。

<!-- write-step: 2 bytes=125:191 -->
````go
var publicRPCCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,95}$`)

````

## 第 3 步：dependencyError

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

dependencyError 是传输边界，不改变领域拒绝、原操作状态或关闭 ACK。
SQL/网络错误可能带连接和请求细节，只允许固定协议码离开服务。

本步声明：`func dependencyError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>errors.Is(err, context.Canceled)</code>；<code>errors.Is(err, context.DeadlineExceeded)</code>；<code>errors.As(err, &amp;encoded)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.Canceled, &quot;REQUEST_CANCELLED&quot;)</code>；<code>status.Error(codes.DeadlineExceeded, &quot;DEPENDENCY_TIMEOUT&quot;)</code>；<code>original.Err()</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, context.Canceled)</code>。条件成立时执行 <code>return status.Error(codes.Canceled, &quot;REQUEST_CANCELLED&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, context.DeadlineExceeded)</code>。条件成立时执行 <code>return status.Error(codes.DeadlineExceeded, &quot;DEPENDENCY_TIMEOUT&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;encoded)</code>。
- 检查 <code>original != nil &amp;&amp; original.Code() != codes.Unknown &amp;&amp; original.Code() != codes.OK</code>。
- 检查 <code>publicRPCCode.MatchString(original.Message())</code>。条件成立时执行 <code>return original.Err()</code>。当前函数在这里结束。
- 检查 <code>original.Code() != codes.Unavailable</code>。条件成立时执行 <code>return status.Error(original.Code(), &quot;DEPENDENCY_FAILURE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(codes.Unavailable, &quot;SERVICE_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=191:1240 -->
````go
// dependencyError 是传输边界，不改变领域拒绝、原操作状态或关闭 ACK。
// SQL/网络错误可能带连接和请求细节，只允许固定协议码离开服务。
func dependencyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "REQUEST_CANCELLED")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "DEPENDENCY_TIMEOUT")
	}
	var encoded interface{ GRPCStatus() *status.Status }
	if errors.As(err, &encoded) {
		original := encoded.GRPCStatus()
		if original != nil && original.Code() != codes.Unknown && original.Code() != codes.OK {
			// 从底层 status 取码，避免 fmt 包装层把私有前缀附到拒绝原因上。
			if publicRPCCode.MatchString(original.Message()) {
				return original.Err()
			}
			if original.Code() != codes.Unavailable {
				return status.Error(original.Code(), "DEPENDENCY_FAILURE")
			}
		}
	}
	return status.Error(codes.Unavailable, "SERVICE_UNAVAILABLE")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/server/dependency_error.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
