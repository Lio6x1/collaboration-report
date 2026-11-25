# 分步编写 `services/account/internal/domain/account/identity.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/037-identity.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

校验跨服务和幂等操作使用的操作号与身份用途字符串。

操作号会进入回执、任务和来源绑定；规范格式防止不同入口接受不同身份字符串。

写之前先按这条执行路线理解各段的连接：

1. ValidOperationID 接受16至96位 ASCII 字母数字、下划线或短横线，首位只能是字母数字。
2. ValidIdentityPurpose 要求大写字母开头，后续仅大写字母、数字或下划线，长度最多40。

本文件需要守住的边界：

- 不要用随机数是否唯一替代格式检查；错误的 purpose 不能继续签发准入。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/identity.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/identity.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/identity.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/identity.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:49 -->
````go
package account

import (
	"errors"
	"regexp"
)

````

## 第 2 步：operationID、identityPurpose、ErrIdentityBinding、ErrIdentityDenied、ErrIdentityUnavailable

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`var (`。

<!-- write-step: 2 bytes=49:398 -->
````go
var (
	operationID            = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)
	identityPurpose        = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,39}$`)
	ErrIdentityBinding     = errors.New("IDENTITY_BINDING_INVALID")
	ErrIdentityDenied      = errors.New("IDENTITY_NOT_CURRENT")
	ErrIdentityUnavailable = errors.New("IDENTITY_UNAVAILABLE")
)

````

## 第 3 步：ValidOperationID

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func ValidOperationID(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>operationID.MatchString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>operationID.MatchString(value)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return operationID.MatchString(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=398:485 -->
````go
func ValidOperationID(value string) bool     { return operationID.MatchString(value) }
````

## 第 4 步：ValidIdentityPurpose

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func ValidIdentityPurpose(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>identityPurpose.MatchString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>identityPurpose.MatchString(value)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return identityPurpose.MatchString(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=485:576 -->
````go
func ValidIdentityPurpose(value string) bool { return identityPurpose.MatchString(value) }
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/identity.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
