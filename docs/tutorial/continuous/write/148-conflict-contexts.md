# 分步编写 `services/document/internal/repository/document/conflict_contexts.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/148-conflict-contexts.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取原合并操作最终冲突attempt及其固定输入。

当前清单或当前审核轮次可能已变化，冲突页面必须展示当时真正发生冲突的三个版本。

写之前先按这条执行路线理解各段的连接：

1. ConflictAttemptRow沿原operation取得attempt、固定输入和冲突数组，供冲突视图与修复共用。

本文件需要守住的边界：

- 禁止按现在MAIN重新计算冲突并冒充原冲突上下文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/conflict_contexts.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/conflict_contexts.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/conflict_contexts.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/conflict_contexts.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:41 -->
````go
package document

import (
	"context"
)

````

## 第 2 步：ConflictAttempt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 7 行。

本步声明：`type ConflictAttempt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 发生冲突的合并尝试编号。 |
| <code>State</code> | <code>string</code> | 尝试当前状态，只有适用状态才能解析冲突。 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 该尝试固定输入的摘要。 |
| <code>Conflicts</code> | <code>&#91;&#93;byte</code> | 固定冲突列表的JSON字节，用户解决的是这份列表。 |

<!-- write-step: 2 bytes=41:152 -->
````go
type ConflictAttempt struct {
	ID                uint64
	State             string
	Digest, Conflicts []byte
}

````

## 第 3 步：ConflictAttemptRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

原操作的最终冲突attempt决定展示输入；不能按现在的轮次或清单重新挑一次计算。

本步声明：`func ConflictAttemptRow(ctx context.Context, q Queryer, operation string) (ConflictAttempt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ConflictAttempt</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1&quot;, operation).Scan(&amp;value.ID, &amp;value.State, &amp;value.Digest, &amp;value.Conflicts)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1&quot;, operation)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=152:638 -->
````go
// 原操作的最终冲突attempt决定展示输入；不能按现在的轮次或清单重新挑一次计算。
func ConflictAttemptRow(ctx context.Context, q Queryer, operation string) (ConflictAttempt, error) {
	var value ConflictAttempt
	err := q.QueryRowContext(ctx, "SELECT id,state,input_digest,conflict_json FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1", operation).Scan(&value.ID, &value.State, &value.Digest, &value.Conflicts)
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/conflict_contexts.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
