# 分步编写 `services/document/internal/domain/document/errors.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/092-errors.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义业务失败分类及稳定错误码，供HTTP与RPC共同翻译。

领域层不知道HTTP状态码；RuleViolation让适配器用errors.As区分规则失败与依赖故障。

写之前先按这条执行路线理解各段的连接：

1. Violation构造Kind和Code
2. Error只返回Code
3. ErrSourcesRequired专门表示准入来源集合缺失。

本文件需要守住的边界：

- 不要把数据库错误包装成PermissionDenied，也不要向客户端暴露SQL细节。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/errors.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/errors.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/errors.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/errors.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:35 -->
````go
package document

import "errors"

````

## 第 2 步：ErrSourcesRequired

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`var ErrSourcesRequired = errors.New("SOURCES_REQUIRED")`。

<!-- write-step: 2 bytes=35:92 -->
````go
var ErrSourcesRequired = errors.New("SOURCES_REQUIRED")

````

## 第 3 步：FailureKind

插入位置：第 2 步代码之后，紧接追加；完成稿约第 7 行。

RuleViolation只表达业务条件，HTTP/RPC适配层决定传输状态。

本步声明：`type FailureKind uint8`。

<!-- write-step: 3 bytes=92:195 -->
````go
// RuleViolation只表达业务条件，HTTP/RPC适配层决定传输状态。
type FailureKind uint8

````

## 第 4 步：InvalidRequest、SessionRequired、PermissionDenied、EntityNotFound、StateConflict、TemporarilyUnavailable

插入位置：第 3 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`const (`。

<!-- write-step: 4 bytes=195:331 -->
````go
const (
	InvalidRequest FailureKind = iota
	SessionRequired
	PermissionDenied
	EntityNotFound
	StateConflict
	TemporarilyUnavailable
)

````

## 第 5 步：RuleViolation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type RuleViolation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>FailureKind</code> | 使用 <code>FailureKind</code> 类型；其定义处的职责：RuleViolation只表达业务条件，HTTP/RPC适配层决定传输状态。 |
| <code>Code</code> | <code>string</code> | 稳定的内部业务错误标识，交给传输层映射HTTP/RPC结果及用户文案，不直接泄漏底层SQL错误。 |

<!-- write-step: 5 bytes=331:393 -->
````go
type RuleViolation struct {
	Kind FailureKind
	Code string
}

````

## 第 6 步：RuleViolation.Error

插入位置：第 5 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func (e RuleViolation) Error() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>e.Code</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return e.Code</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=393:449 -->
````go
func (e RuleViolation) Error() string { return e.Code }
````

## 第 7 步：Violation

插入位置：第 6 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func Violation(kind FailureKind, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>FailureKind</code> | 使用 <code>FailureKind</code> 类型；其定义处的职责：RuleViolation只表达业务条件，HTTP/RPC适配层决定传输状态。<br>在此声明中的实际使用：<code>Kind: kind</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>RuleViolation{Kind: kind, Code: code}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return RuleViolation{Kind: kind, Code: code}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=449:551 -->
````go
func Violation(kind FailureKind, code string) error {
	return RuleViolation{Kind: kind, Code: code}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/errors.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
