# 分步编写 `services/document/internal/repository/document/lifecycle_rpc.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/156-lifecycle-rpc.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久生命周期RPC的原请求绑定与阶段输入。

协议登记成功与业务清理成功不同，专用任务不能被普通恢复worker误领。

写之前先按这条执行路线理解各段的连接：

1. ReadLifecycleRPCBinding读取原proto绑定
2. InsertLifecycleRPCBinding首次登记
3. UpdateLifecycleRPCBinding记录后续阶段。

本文件需要守住的边界：

- 此task的SUCCEEDED只表示绑定存好，业务状态必须从lifecycle_operation读取。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/lifecycle_rpc.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/lifecycle_rpc.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/lifecycle_rpc.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/lifecycle_rpc.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:57 -->
````go
package document

import (
	"context"
	"database/sql"
)

````

## 第 2 步：LifecycleRPCBindingKind

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

专用登记不会被原恢复worker领取。SUCCEEDED仅表示协议请求绑定已持久登记，
业务阶段必须从lifecycle_operation读取，绝不能凭本task签出清理成功。

本步声明：`const LifecycleRPCBindingKind = "ACCOUNT_LIFECYCLE_RPC"`。

<!-- write-step: 2 bytes=57:310 -->
````go
// 专用登记不会被原恢复worker领取。SUCCEEDED仅表示协议请求绑定已持久登记，
// 业务阶段必须从lifecycle_operation读取，绝不能凭本task签出清理成功。
const LifecycleRPCBindingKind = "ACCOUNT_LIFECYCLE_RPC"

````

## 第 3 步：ReadLifecycleRPCBinding

插入位置：第 2 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func ReadLifecycleRPCBinding(ctx context.Context, q Queryer, operation string, locked bool) ([]byte, string, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, operation)</code> |
| <code>locked</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>locked</code>。
- 执行 <code>q.QueryRowContext(ctx, query, operation).Scan(&amp;raw, &amp;original, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, query, operation)</code>。
- 此处通过 <code>return raw, original, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=310:755 -->
````go
func ReadLifecycleRPCBinding(ctx context.Context, q Queryer, operation string, locked bool) ([]byte, string, string, error) {
	query := `SELECT payload,operation_id,state FROM durable_task WHERE kind='ACCOUNT_LIFECYCLE_RPC' AND business_key=?`
	if locked {
		query += " FOR UPDATE"
	}
	var raw []byte
	var original, state string
	err := q.QueryRowContext(ctx, query, operation).Scan(&raw, &original, &state)
	return raw, original, state, err
}

````

## 第 4 步：InsertLifecycleRPCBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func InsertLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(&#x27;ACCOUNT_LIFECYCLE_RPC&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) 	 ON DUPLI …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(&#x27;ACCOUNT_LIFECYCLE_RPC&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) 	 ON DUPLI …</code>（长表达式，按本步完整代码继续） |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(&#x27;ACCOUNT_LIFECYCLE_RPC&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) 	 ON DUPLI …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=755:1150 -->
````go
func InsertLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES('ACCOUNT_LIFECYCLE_RPC',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))
	 ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)`, operation, operation, string(raw))
	return err
}

````

## 第 5 步：UpdateLifecycleRPCBinding

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func UpdateLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET payload=? 	 WHERE kind=&#x27;ACCOUNT_LIFECYCLE_RPC&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;`, string(raw), operation, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET payload=? 	 WHERE kind=&#x27;ACCOUNT_LIFECYCLE_RPC&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;`, string(raw), operation, operation)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET payload=? 	 WHERE kind=&#x27;ACCOUNT_LIFECYCLE_RPC&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;`, string(raw), operation, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1150:1466 -->
````go
func UpdateLifecycleRPCBinding(ctx context.Context, tx *sql.Tx, operation string, raw []byte) error {
	_, err := tx.ExecContext(ctx, `UPDATE durable_task SET payload=?
	 WHERE kind='ACCOUNT_LIFECYCLE_RPC' AND business_key=? AND operation_id=? AND state='SUCCEEDED'`, string(raw), operation, operation)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/lifecycle_rpc.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
