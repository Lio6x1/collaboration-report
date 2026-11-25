# 分步编写 `services/account/internal/domain/account/errors.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/036-errors.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

声明登录、会话、验证码和投递故障的稳定领域错误。

同一个认证用例可从 HTTP 或 RPC 调用，领域层不应把拒绝原因固定为某个 HTTP 状态。

写之前先按这条执行路线理解各段的连接：

1. 错误以 errors.New 的哨兵值定义，外层用 errors.Is 穿透包装。
2. 区分 ErrChallengeInvalid、ErrChallengeRateLimited 与存储/SMTP不可用，供 handler 分别输出400、429、503。

本文件需要守住的边界：

- Redis/SMTP 原始报错可能包含部署信息，不能直接返回或记录其原文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/errors.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/errors.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/errors.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/errors.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:34 -->
````go
package account

import "errors"

````

## 第 2 步：ErrLoginInput、ErrLoginLimited、ErrLoginInvalid、ErrSessionInvalid、ErrChallengeStoreUnavailable、ErrSMTPDeliveryFailed、ErrChallengeInvalid、ErrChallengeRateLimited

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

领域错误不包含HTTP状态；同一用例可以由HTTP、RPC和后台流程调用。

本步声明：`var (`。

<!-- write-step: 2 bytes=34:749 -->
````go
// 领域错误不包含HTTP状态；同一用例可以由HTTP、RPC和后台流程调用。
var (
	ErrLoginInput     = errors.New("LOGIN_INPUT_INVALID")
	ErrLoginLimited   = errors.New("LOGIN_RATE_LIMITED")
	ErrLoginInvalid   = errors.New("LOGIN_INVALID")
	ErrSessionInvalid = errors.New("SESSION_INVALID")
	// 已知基础设施故障用哨兵错误跨层传递；不能靠驱动正文判断，也不能丢失可解释的失败类别。
	ErrChallengeStoreUnavailable = errors.New("CHALLENGE_STORE_UNAVAILABLE")
	ErrSMTPDeliveryFailed        = errors.New("SMTP_DELIVERY_FAILED")
	ErrChallengeInvalid          = errors.New("CHALLENGE_INVALID")
	ErrChallengeRateLimited      = errors.New("CHALLENGE_RATE_LIMITED")
)
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/errors.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
