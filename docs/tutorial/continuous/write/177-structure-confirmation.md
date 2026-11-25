# 分步编写 `services/document/internal/repository/document/structure_confirmation.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/177-structure-confirmation.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在确认事务中锁取预览的原回执、原上下文与固定快照。

确认需要真实SUCCEEDED预览证据，不能为复用恢复代码伪造PREPARING状态。

写之前先按这条执行路线理解各段的连接：

1. LockedStructureConfirmation用同一tx读取固定记录并返回StructureConfirmationFact，供服务比对actor、会话、动作和候选。

本文件需要守住的边界：

- 缺任一原工件都属于绑定未知
- 调用方传入的状态不能替代持久事实。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/structure_confirmation.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/structure_confirmation.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/structure_confirmation.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/structure_confirmation.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:65 -->
````go
package document

import (
	"context"
	"database/sql"
	"time"
)

````

## 第 2 步：StructureConfirmationFact

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`type StructureConfirmationFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>PendingStructurePreviewBinding</code> | 使用 <code>PendingStructurePreviewBinding</code> 类型；其定义处的职责：持久结构预览上下文与恢复任务，列出当前结构的单元和活动支线。 |
| <code>Snapshot</code> | <code>&#91;&#93;byte</code> | 预览时已固定的结构快照字节；确认不能悄悄重新采样成另一份。 |
| <code>ReceiptExpires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 2 bytes=65:204 -->
````go
type StructureConfirmationFact struct {
	Binding        PendingStructurePreviewBinding
	Snapshot       []byte
	ReceiptExpires time.Time
}

````

## 第 3 步：LockedStructureConfirmation

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

确认读取实际原回执、原上下文与不可变快照；没有任何一步换池或用调用者伪造的PREPARING状态。

本步声明：`func LockedStructureConfirmation(ctx context.Context, tx *sql.Tx, operation string) (StructureConfirmationFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,  original.payload,fixed.payload FROM operation_receipt o  LEFT JOIN durable_tas …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>fact.Binding.Operation = operation</code>；<code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,  original.payload,fixed.payload FROM operation_receipt o  LEFT JOIN durable_tas …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>StructureConfirmationFact</code> | 实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,  original.payload,fixed.payload FROM operation_receipt o  LEFT JOIN durable_tas …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,  original.payload,fixed.payload FROM operation_receipt o  LEFT JOIN durable_tas …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=204:1267 -->
````go
// 确认读取实际原回执、原上下文与不可变快照；没有任何一步换池或用调用者伪造的PREPARING状态。
func LockedStructureConfirmation(ctx context.Context, tx *sql.Tx, operation string) (StructureConfirmationFact, error) {
	var fact StructureConfirmationFact
	fact.Binding.Operation = operation
	err := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,
 original.payload,fixed.payload FROM operation_receipt o
 LEFT JOIN durable_task original ON original.kind='STRUCTURE_PREVIEW_CONTEXT' AND original.business_key=o.operation_id AND original.operation_id=o.operation_id
 LEFT JOIN durable_task fixed ON fixed.kind='STRUCTURE_PREVIEW_FIXED' AND fixed.business_key=o.operation_id AND fixed.operation_id=o.operation_id
 WHERE o.operation_id=? AND o.kind='STRUCTURE_PREVIEW' FOR UPDATE`, operation).Scan(
		&fact.Binding.Actor, &fact.Binding.State, &fact.Binding.Fingerprint, &fact.Binding.Target,
		&fact.ReceiptExpires, &fact.Binding.Original, &fact.Snapshot,
	)
	return fact, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/structure_confirmation.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
