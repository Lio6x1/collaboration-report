# 分步编写 `services/account/internal/repository/account/governance_queries.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/049-governance-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现平台治理授权的行锁、授予、撤销和恢复一致性检查。

平台管理员是显式授权事实，不从普通注册或组织管理员推导。

写之前先按这条执行路线理解各段的连接：

1. GovernanceGrantRow 读取state/epoch/revision/原操作；GrantGovernance 激活资格，RevokeGovernance标记REVOKING。
2. InsertGovernanceOperatorReceipt 以actor=0记录运维操作及审计；GovernanceRecoveryReady联查角色、gate、授权和任务的原绑定。

本文件需要守住的边界：

- 撤销成功须等待document屏障；不能在REVOKING时再次覆盖授予。
- epoch和revision递增前必须防溢出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/governance_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/governance_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/governance_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/governance_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:235 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"context"
	"database/sql"
	"errors"
)

// governance_identity.go：保留原实现，按业务合并文件。

````

## 第 2 步：Identity.LockGovernanceGrant

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func (r *Identity) LockGovernanceGrant(ctx context.Context, tx *sql.Tx, actor uint64) (string, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE&quot;, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE&quot;, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>epoch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE&quot;, actor).Scan(&amp;state, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE&quot;, actor)</code>。
- 此处通过 <code>return state, epoch, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=235:624 -->
````go
func (r *Identity) LockGovernanceGrant(ctx context.Context, tx *sql.Tx, actor uint64) (string, uint64, error) {
	var state string
	var epoch uint64
	err := tx.QueryRowContext(ctx, "SELECT state,epoch FROM platform_governance_grant WHERE user_id=? FOR UPDATE", actor).Scan(&state, &epoch)
	return state, epoch, err
}

// governance_operator.go：保留原实现，按业务合并文件。

````

## 第 3 步：GovernanceGrant

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type GovernanceGrant struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 平台治理授权当前状态。 |
| <code>Operation</code> | <code>string</code> | 改变此治理授权的原操作号。 |
| <code>Epoch</code> | <code>uint64</code> | 治理权限来源代次，旧证明不能继续用。 |
| <code>Revision</code> | <code>uint64</code> | 授权记录的并发修订号。 |

<!-- write-step: 3 bytes=624:707 -->
````go
type GovernanceGrant struct {
	State, Operation string
	Epoch, Revision  uint64
}

````

## 第 4 步：GovernanceUserActive

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func GovernanceUserActive(ctx context.Context, tx *sql.Tx, user uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>state == &quot;ACTIVE&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code>。
- 此处通过 <code>return state == &quot;ACTIVE&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=707:946 -->
````go
func GovernanceUserActive(ctx context.Context, tx *sql.Tx, user uint64) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
	return state == "ACTIVE", err
}

````

## 第 5 步：GovernanceGrantRow

插入位置：第 4 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func GovernanceGrantRow(ctx context.Context, q Queryer, user uint64, lock bool) (GovernanceGrant, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, user)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, user)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>GovernanceGrant</code> | 实际返回表达式：<code>grant</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, query, user).Scan(&amp;grant.State, &amp;grant.Epoch, &amp;grant.Revision, &amp;grant.Operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, query, user)</code>。
- 此处通过 <code>return grant, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=946:1361 -->
````go
func GovernanceGrantRow(ctx context.Context, q Queryer, user uint64, lock bool) (GovernanceGrant, error) {
	var grant GovernanceGrant
	query := "SELECT state,epoch,revision,changed_operation_id FROM platform_governance_grant WHERE user_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, user).Scan(&grant.State, &grant.Epoch, &grant.Revision, &grant.Operation)
	return grant, err
}

````

## 第 6 步：GrantGovernance

插入位置：第 5 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func GrantGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string, previous GovernanceGrant) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,&#x27;ACTIVE&#x27;,?)&quot;, user, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;ACTIVE&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,&#x27;ACTIVE&#x27;,?)&quot;, user, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;ACTIVE&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,&#x27;ACTIVE&#x27;,?)&quot;, user, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;ACTIVE&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |
| <code>previous</code> | <code>GovernanceGrant</code> | 在此声明中的实际使用：<code>previous.Revision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>previous.Revision == 0</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,&#x27;ACTIVE&#x27;,?)&quot;, user, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;ACTIVE&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1361:1891 -->
````go
func GrantGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string, previous GovernanceGrant) error {
	if previous.Revision == 0 {
		_, err := tx.ExecContext(ctx, "INSERT INTO platform_governance_grant(user_id,state,changed_operation_id) VALUES(?,'ACTIVE',?)", user, operation)
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='ACTIVE',epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?", operation, user)
	return err
}

````

## 第 7 步：RevokeGovernance

插入位置：第 6 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func RevokeGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;REVOKING&#x27;,epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?&quot;, operation, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1891:2206 -->
````go
func RevokeGovernance(ctx context.Context, tx *sql.Tx, user uint64, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='REVOKING',epoch=epoch+1,revision=revision+1,changed_operation_id=?,updated_at=UTC_TIMESTAMP(6) WHERE user_id=?", operation, user)
	return err
}

````

## 第 8 步：CompleteGovernanceRevocation

插入位置：第 7 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func CompleteGovernanceRevocation(ctx context.Context, tx *sql.Tx, user, epoch uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;CLOSED&#x27;,updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=? AND changed_operation_id=?&quot;, user, epoch, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;CLOSED&#x27;,updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=? AND changed_operation_id=?&quot;, user, epoch, operation)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;CLOSED&#x27;,updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=? AND changed_operation_id=?&quot;, user, epoch, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;CLOSED&#x27;,updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=? AND changed_operation_id=?&quot;, user, epoch, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PLATFORM_GOVERNANCE_BINDING_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE platform_governance_grant SET state=&#x27;CLOSED&#x27;,updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=? AND changed_operation_id=?&quot;, user, epoch, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>n != 1</code>。条件成立时执行 <code>return errors.New(&quot;PLATFORM_GOVERNANCE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2206:2727 -->
````go
func CompleteGovernanceRevocation(ctx context.Context, tx *sql.Tx, user, epoch uint64, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE platform_governance_grant SET state='CLOSED',updated_at=UTC_TIMESTAMP(6) WHERE user_id=? AND state='REVOKING' AND epoch=? AND changed_operation_id=?", user, epoch, operation)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("PLATFORM_GOVERNANCE_BINDING_CHANGED")
	}
	return nil
}

````

## 第 9 步：InsertGovernanceOperatorReceipt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func InsertGovernanceOperatorReceipt(ctx context.Context, tx *sql.Tx, operation string, fingerprint [32]byte, target, result []byte, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 	 VALUES(?,0,&#x27;PLATFORM_GOVER …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,&#x27;PLATFORM_GOVERNANCE_OPERATOR&#x27;,?,?)&quot;, operation, string(target), string(result))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 	 VALUES(?,0,&#x27;PLATFORM_GOVER …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,&#x27;PLATFORM_GOVERNANCE_OPERATOR&#x27;,?,?)&quot;, operation, string(target), string(result))</code> |
| <code>fingerprint</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 	 VALUES(?,0,&#x27;PLATFORM_GOVER …</code>（长表达式，按本步完整代码继续） |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 	 VALUES(?,0,&#x27;PLATFORM_GOVER …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 	 VALUES(?,0,&#x27;PLATFORM_GOVER …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,&#x27;PLATFORM_GOVERNANCE_OPERATOR&#x27;,?,?)&quot;, operation, string(target), string(result))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 9 bytes=2727:3587 -->
````go
func InsertGovernanceOperatorReceipt(ctx context.Context, tx *sql.Tx, operation string, fingerprint [32]byte, target, result []byte, state string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at)
	 VALUES(?,0,'PLATFORM_GOVERNANCE_OPERATOR',?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))`, operation, string(target), fingerprint[:], state, string(result), state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,'PLATFORM_GOVERNANCE_OPERATOR',?,?)", operation, string(target), string(result))
	return err
}

// governance_readiness.go：保留原实现，按业务合并文件。

````

## 第 10 步：GovernanceRecoveryReady

插入位置：第 9 步代码之后，紧接追加；完成稿约第 83 行。

管理员撤权可在重启后继续，但必须保留角色、固定payload和来源关闭的一致绑定。

本步声明：`func GovernanceRecoveryReady(ctx context.Context, db *sql.DB) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task t 	 LEFT JOIN platform_governance_grant p ON CAST(p.user_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,&#x27;$.user_id&#x27;)) 	  …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PLATFORM_GOVERNANCE_RECOVERY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task t 	 LEFT JOIN platform_governance_grant p ON CAST(p.user_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,&#x27;$.user_id&#x27;)) 	  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task t 	 LEFT JOIN platform_governance_grant p ON CAST(p.user_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,&#x27;$.user_id&#x27;)) 	  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>invalid != 0</code>。条件成立时执行 <code>return errors.New(&quot;PLATFORM_GOVERNANCE_RECOVERY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3587:5043 -->
````go
// 管理员撤权可在重启后继续，但必须保留角色、固定payload和来源关闭的一致绑定。
func GovernanceRecoveryReady(ctx context.Context, db *sql.DB) error {
	var invalid uint64
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task t
	 LEFT JOIN platform_governance_grant p ON CAST(p.user_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.user_id'))
	 LEFT JOIN access_gate g ON g.scope_key=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.scope_key'))
	 LEFT JOIN authorization_change c ON c.revocation_id=t.operation_id
	 WHERE t.kind='PLATFORM_GOVERNANCE_CHANGE' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
	 p.user_id IS NULL OR p.state<>'REVOKING' OR p.changed_operation_id<>t.operation_id OR
	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.kind')),'')<>'PLATFORM_GOVERNANCE_REVOKE' OR
	 JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.scope_key'))<>CONCAT('platform-admin:',CAST(p.user_id AS CHAR)) OR
	 p.epoch<=1 OR CAST(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.closed_epoch')),'0') AS UNSIGNED)<>p.epoch-1 OR
	 g.scope_key IS NULL OR g.state<>'BLOCKED' OR g.max_epoch<>p.epoch-1 OR g.last_revocation_id<>t.operation_id OR
	 c.revocation_id IS NULL OR c.scope_key<>g.scope_key OR c.epoch<>g.max_epoch OR COALESCE(c.actor_id,1)<>0 OR
	 t.business_key<>t.operation_id)`).Scan(&invalid)
	if err != nil {
		return err
	}
	if invalid != 0 {
		return errors.New("PLATFORM_GOVERNANCE_RECOVERY_INVALID")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/governance_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
