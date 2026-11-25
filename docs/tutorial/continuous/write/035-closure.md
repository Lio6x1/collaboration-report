# 分步编写 `services/account/internal/domain/account/closure.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/035-closure.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。

关闭范围和旧epoch在准备时固定，worker重试只推进这份计划。

写之前先按这条执行路线理解各段的连接：

1. SessionClosure以Kind选择后续收尾，Scope/Epoch表示应停止输出的旧来源。
2. SessionClosePreparation把目标JSON、载荷JSON、指纹和会话摘要一并交给repository准备事务。

本文件需要守住的边界：

- 关闭旧epoch与任务租约是两种代次，不能用当前用户epoch替换载荷中的Epoch。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/closure.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/closure.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/closure.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/closure.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:17 -->
````go
package account

````

## 第 2 步：SessionClosure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。

本步声明：`type SessionClosure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | 关闭类别，决定是会话、成员还是其他来源关闭。<br>序列化/校验标记：<code>`json:&quot;kind,omitempty&quot;`</code> |
| <code>UserID</code> | <code>string</code> | 账号的稳定标识，用于查询和关系关联；昵称相同不代表同一个账号。<br>序列化/校验标记：<code>`json:&quot;user_id,omitempty&quot;`</code> |
| <code>OrganizationID</code> | <code>string</code> | 组织的稳定标识，关联成员、角色和组织生命周期；显示名称改动不改变这个关联。<br>序列化/校验标记：<code>`json:&quot;organization_id,omitempty&quot;`</code> |
| <code>Role</code> | <code>string</code> | 成员关闭相关角色事实，不能跨角色复用任务。<br>序列化/校验标记：<code>`json:&quot;role,omitempty&quot;`</code> |
| <code>SessionHash</code> | <code>string</code> | 会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。<br>序列化/校验标记：<code>`json:&quot;session_hash&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 原关闭的规范来源键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 原关闭固定的十进制代次，重试不推进成新的代次。<br>序列化/校验标记：<code>`json:&quot;closed_epoch&quot;`</code> |

<!-- write-step: 2 bytes=17:484 -->
````go
// SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。
type SessionClosure struct {
	Kind           string `json:"kind,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	Role           string `json:"role,omitempty"`
	SessionHash    string `json:"session_hash"`
	Scope          string `json:"scope_key"`
	Epoch          string `json:"closed_epoch"`
}

````

## 第 3 步：ClosureTask

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type ClosureTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 后台关闭任务主键。 |
| <code>OperationID</code> | <code>string</code> | 一次业务意图的幂等操作号；重试需要沿用它，不能用新号绕过原操作的未知结果。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 原SessionClosure JSON载荷，恢复先解码核验。 |

<!-- write-step: 3 bytes=484:573 -->
````go
type ClosureTask struct {
	ID          uint64
	OperationID string
	Payload     []byte
}

````

## 第 4 步：TaskOwnership

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type TaskOwnership struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Instance</code> | <code>string</code> | 正在持有原任务的执行实例。 |
| <code>State</code> | <code>string</code> | 任务当前状态，配合实例确认执行所有权。 |

<!-- write-step: 4 bytes=573:638 -->
````go
type TaskOwnership struct {
	Instance string
	State    string
}

````

## 第 5 步：SessionClosePreparation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type SessionClosePreparation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SessionID</code> | <code>&#91;&#93;byte</code> | 原会话令牌摘要字节，用于锁定会话。 |
| <code>UserID</code> | <code>string</code> | 账号的稳定标识，用于查询和关系关联；昵称相同不代表同一个账号。 |
| <code>Epoch</code> | <code>uint64</code> | 要关闭的原会话来源代次。 |
| <code>OperationID</code> | <code>string</code> | 一次业务意图的幂等操作号；重试需要沿用它，不能用新号绕过原操作的未知结果。 |
| <code>Scope</code> | <code>string</code> | 规范会话来源键，下游围栏按此关闭。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原操作目标的规范JSON，用于回执匹配。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 发给持久关闭任务的固定载荷。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |

<!-- write-step: 5 bytes=638:838 -->
````go
type SessionClosePreparation struct {
	SessionID   []byte
	UserID      string
	Epoch       uint64
	OperationID string
	Scope       string
	Target      []byte
	Payload     []byte
	Fingerprint []byte
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/closure.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
