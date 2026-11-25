# 分步编写 `services/document/internal/repository/document/account_retirement_queries.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/143-account-retirement-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存账号注销准备、冻结、拒绝及显式取消的持久事实。

计划、管理员预留与封门必须一起提交，回复丢失不能擅自释放预留。

写之前先按这条执行路线理解各段的连接：

1. RecordAccountRetirement固定绑定
2. FreezeAccountRetirement在identity来源锁下封门
3. CancelAccountRetirement按原计划回退
4. AccountRetirementFenceCancelled阻止迟到Fence重新关闭已取消来源。

本文件需要守住的边界：

- 取消必须真实持久拒绝证据
- 不存在或超时不是回滚成功。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/account_retirement_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/account_retirement_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/account_retirement_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/account_retirement_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:310 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// account_retirement.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetirementRecord

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type AccountRetirementRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 发起这次账号退役操作的账号。 |
| <code>Epoch</code> | <code>uint64</code> | 这次退役绑定的身份代次。 |
| <code>Kind</code> | <code>string</code> | 生命周期操作种类，避免把其他操作当作账号退役恢复。 |
| <code>State</code> | <code>string</code> | 生命周期操作当前状态。 |
| <code>Plan</code> | <code>&#91;&#93;byte</code> | 原退役计划的序列化字节；恢复沿用原计划而不是重新枚举后换一份。 |

<!-- write-step: 2 bytes=310:414 -->
````go
type AccountRetirementRecord struct {
	Actor, Epoch uint64
	Kind, State  string
	Plan         []byte
}

````

## 第 3 步：AccountRetirementSnapshot

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func AccountRetirementSnapshot(ctx context.Context, q Queryer, operation string) (AccountRetirementRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetirementRecord</code> | 实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code>。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=414:831 -->
````go
func AccountRetirementSnapshot(ctx context.Context, q Queryer, operation string) (AccountRetirementRecord, error) {
	var record AccountRetirementRecord
	err := q.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan
		FROM lifecycle_operation WHERE operation_id=?`, operation).
		Scan(&record.Actor, &record.Kind, &record.State, &record.Epoch, &record.Plan)
	return record, err
}

````

## 第 4 步：LockedAccountRetirement

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func LockedAccountRetirement(ctx context.Context, tx *sql.Tx, operation string) (AccountRetirementRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetirementRecord</code> | 实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan 		FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=831:1259 -->
````go
func LockedAccountRetirement(ctx context.Context, tx *sql.Tx, operation string) (AccountRetirementRecord, error) {
	var record AccountRetirementRecord
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan
		FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&record.Actor, &record.Kind, &record.State, &record.Epoch, &record.Plan)
	return record, err
}

````

## 第 5 步：RecordAccountRetirement

插入位置：第 4 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func RecordAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding, plan []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,&#x27;FROZEN&#x27;,?,?)`, binding.Operation,  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code> |
| <code>plan</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,&#x27;FROZEN&#x27;,?,?)`, binding.Operation,  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1259:1640 -->
````go
func RecordAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding, plan []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
		VALUES(?,?,'ACCOUNT_CLOSE','FROZEN',?,?)`, binding.Operation, binding.User, binding.IdentityEpoch, string(plan))
	return err
}

````

## 第 6 步：FreezeAccountRetirement

插入位置：第 5 步代码之后，紧接追加；完成稿约第 42 行。

必须先持有同一identity来源锁。固定计划、管理员预留和新操作封门在同一事务提交，
提交回复未知时不能释放预留；由原Account注销操作继续确认或执行显式取消协议。

本步声明：`func FreezeAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 		WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, &quot;identity:&quot;+domain.TextID(binding.User))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.IdentityEpoch</code>；<code>binding.Operation</code>；<code>binding.User</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 		WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, &quot;identity:&quot;+domain.TextID(binding.User))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1640:2225 -->
````go
// 必须先持有同一identity来源锁。固定计划、管理员预留和新操作封门在同一事务提交，
// 提交回复未知时不能释放预留；由原Account注销操作继续确认或执行显式取消协议。
func FreezeAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error {
	_, err := tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1
		WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, "identity:"+domain.TextID(binding.User))
	return err
}

````

## 第 7 步：AccountRetirementGateMatches

插入位置：第 6 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func AccountRetirementGateMatches(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? 		FROM access_gate WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, &quot;identity:&quot;+domain.TextID(binding.User))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.IdentityEpoch</code>；<code>binding.Operation</code>；<code>binding.User</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>matches</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? 		FROM access_gate WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, &quot;identity: …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? 		FROM access_gate WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, &quot;identity:&quot;+domain.TextID(binding.User))</code>。
- 此处通过 <code>return matches, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2225:2703 -->
````go
func AccountRetirementGateMatches(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) (bool, error) {
	var matches bool
	err := tx.QueryRowContext(ctx, `SELECT max_epoch=? AND state='BLOCKED' AND last_revocation_id=?
		FROM access_gate WHERE scope_key=?`, binding.IdentityEpoch, binding.Operation, "identity:"+domain.TextID(binding.User)).Scan(&matches)
	return matches, err
}

// account_retirement_cancel.go：保留原实现，按业务合并文件。

````

## 第 8 步：RecordRejectedAccountRetirement

插入位置：第 7 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func RecordRejectedAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan,last_error) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,&#x27;REJECTED&#x27;,?,?,&#x27;ACCOUNT_ …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>Binding: binding</code>；<code>binding.Operation</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan,last_error) 		VALUES(?,?,&#x27;ACCOUNT_CLOSE&#x27;,&#x27;REJECTED&#x27;,?,?,&#x27;ACCOUNT_ …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 8 bytes=2703:3252 -->
````go
func RecordRejectedAccountRetirement(ctx context.Context, tx *sql.Tx, binding domain.AccountRetirementBinding) error {
	raw, err := json.Marshal(domain.AccountRetirementPlan{Binding: binding, ReservedKnowledgeBases: []uint64{}})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan,last_error)
		VALUES(?,?,'ACCOUNT_CLOSE','REJECTED',?,?,'ACCOUNT_REJECTED')`, binding.Operation, binding.User, binding.IdentityEpoch, string(raw))
	return err
}

````

## 第 9 步：CancelAccountRetirement

插入位置：第 8 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func CancelAccountRetirement(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockedKnowledgeBase(ctx, tx, domain.TextID(kb))</code>；<code>tx.ExecContext(ctx, `UPDATE admin_retirement SET state=&#x27;CANCELLED&#x27; 			WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state=&#x27;RESERVED&#x27;`, kb, plan.Binding.User, plan.Binding.Operation)</code>；<code>tx.ExecContext(ctx, `UPDATE lifecycle_operation SET state=&#x27;REJECTED&#x27;,last_error=&#x27;ACCOUNT_REJECTED&#x27; 		WHERE operation_id=? AND state=&#x27;FROZEN&#x27;`, plan.Binding.Operation)</code>；<code>ReopenSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockedKnowledgeBase(ctx, tx, domain.TextID(kb))</code>；<code>tx.ExecContext</code>；<code>ReopenSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.ReservedKnowledgeBases</code>；<code>plan.Binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETIREMENT_RESERVATION_CHANGED&quot;)</code>；<code>ReopenSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE admin_retirement SET state=&#x27;CANCELLED&#x27; 			WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state=&#x27;RESERVED&#x27;`, kb, plan.Binding.User, plan.Binding.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_RESERVATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE lifecycle_operation SET state=&#x27;REJECTED&#x27;,last_error=&#x27;ACCOUNT_REJECTED&#x27; 		WHERE operation_id=? AND state=&#x27;FROZEN&#x27;`, plan.Binding.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return ReopenSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3252:4219 -->
````go
func CancelAccountRetirement(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error {
	for _, kb := range plan.ReservedKnowledgeBases {
		if _, _, err := LockedKnowledgeBase(ctx, tx, domain.TextID(kb)); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE admin_retirement SET state='CANCELLED'
			WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state='RESERVED'`, kb, plan.Binding.User, plan.Binding.Operation)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return errors.New("ACCOUNT_RETIREMENT_RESERVATION_CHANGED")
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE lifecycle_operation SET state='REJECTED',last_error='ACCOUNT_REJECTED'
		WHERE operation_id=? AND state='FROZEN'`, plan.Binding.Operation); err != nil {
		return err
	}
	return ReopenSourceGate(ctx, tx, "identity:"+domain.TextID(plan.Binding.User), plan.Binding.IdentityEpoch)
}

````

## 第 10 步：AccountRetirementFenceCancelled

插入位置：第 9 步代码之后，紧接追加；完成稿约第 91 行。

调用方已锁定来源门禁；取消也必须先取得该锁，因此这里的首次一致读能观察
刚完成的取消且不需要对不存在的操作取得间隙锁。迟到Fence不能重新封住已取消来源。

本步声明：`func AccountRetirementFenceCancelled(ctx context.Context, tx *sql.Tx, operation, scope string, epoch uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>scope != &quot;identity:&quot;+domain.TextID(actor)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>expected != epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>state == &quot;REJECTED&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;FENCE_ID_CONFLICT&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision 		FROM lifecycle_operation WHERE operation_id=?`, operation).Scan(&amp;actor, &amp;kind, &amp;state, &amp;expected)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision 		FROM lifecycle_operation WHERE operation_id=?`, operation)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>kind != &quot;ACCOUNT_CLOSE&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>expected != epoch &#124;&#124; scope != &quot;identity:&quot;+domain.TextID(actor)</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;FENCE_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return state == &quot;REJECTED&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4219:5150 -->
````go
// 调用方已锁定来源门禁；取消也必须先取得该锁，因此这里的首次一致读能观察
// 刚完成的取消且不需要对不存在的操作取得间隙锁。迟到Fence不能重新封住已取消来源。
func AccountRetirementFenceCancelled(ctx context.Context, tx *sql.Tx, operation, scope string, epoch uint64) (bool, error) {
	var actor, expected uint64
	var kind, state string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision
		FROM lifecycle_operation WHERE operation_id=?`, operation).Scan(&actor, &kind, &state, &expected)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if kind != "ACCOUNT_CLOSE" {
		return false, nil
	}
	if expected != epoch || scope != "identity:"+domain.TextID(actor) {
		return false, domain.Violation(domain.StateConflict, "FENCE_ID_CONFLICT")
	}
	return state == "REJECTED", nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/account_retirement_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
