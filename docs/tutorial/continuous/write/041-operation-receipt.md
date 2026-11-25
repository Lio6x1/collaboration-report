# 分步编写 `services/account/internal/domain/account/operation_receipt.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/041-operation-receipt.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达原操作回执、退出会话选择和只含状态的最小进度。

旧会话可证明自己发起的退出，却不能因此重新获得用户资料读取权。

写之前先按这条执行路线理解各段的连接：

1. OperationReceipt 固定actor/kind/state/sessionHash；LogoutSelection 标记是否已有回执。
2. OperationProgress 在旧会话分支只携带State，在有效会话分支可携带已授权的Result。

本文件需要守住的边界：

- Token仅用于恢复Cookie交付，不得存入SessionHash。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/operation_receipt.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/operation_receipt.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/operation_receipt.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/operation_receipt.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:17 -->
````go
package account

````

## 第 2 步：OperationReceipt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`type OperationReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ActorID</code> | <code>string</code> | 原操作发起账号，防止读取他人的操作状态。 |
| <code>Kind</code> | <code>string</code> | 原操作类别，防止不同业务共用同一个回执。 |
| <code>State</code> | <code>string</code> | 原操作持久状态，不以HTTP请求是否结束替代。 |
| <code>SessionHash</code> | <code>string</code> | 会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。 |

<!-- write-step: 2 bytes=17:131 -->
````go
type OperationReceipt struct {
	ActorID     string
	Kind        string
	State       string
	SessionHash string
}

````

## 第 3 步：LogoutSelection

插入位置：第 2 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type LogoutSelection struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Session</code> | <code>Session</code> | 已认回的原登录会话。 |
| <code>Token</code> | <code>string</code> | 本次请求提供的会话原文，仅在请求处理内使用。 |
| <code>Registered</code> | <code>bool</code> | 原退出是否已登记为持久关闭操作，决定继续登记还是恢复。 |
| <code>ReceiptState</code> | <code>string</code> | 原操作回执所处状态，决定是返回既有结果、继续原操作还是等待核验；未知状态不能伪报成功。 |

<!-- write-step: 3 bytes=131:247 -->
````go
type LogoutSelection struct {
	Session      Session
	Token        string
	Registered   bool
	ReceiptState string
}

````

## 第 4 步：OperationProgress

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

OperationProgress只携带持久结果或原关闭操作的状态，URL由HTTP适配层生成。

本步声明：`type OperationProgress struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 原关闭操作当前持久阶段。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 已经持久的业务结果字节；HTTP状态链接在handler生成。 |

<!-- write-step: 4 bytes=247:411 -->
````go
// OperationProgress只携带持久结果或原关闭操作的状态，URL由HTTP适配层生成。
type OperationProgress struct {
	State  string
	Result []byte
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/operation_receipt.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
