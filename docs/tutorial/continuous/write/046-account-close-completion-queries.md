# 分步编写 `services/account/internal/repository/account/account_close_completion_queries.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/046-account-close-completion-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存注销取消、来源ACK汇总、最终个人资料清理和自动恢复任务。

必须确认全部固定来源和Doc清理证据后才能删除凭据，不能把缺失任务忽略为成功。

写之前先按这条执行路线理解各段的连接：

1. ConfirmedScopes逐页按原business_key查任务与ACK，校验固定绑定及摘要。
2. FinalizeAccountCloseData重锁原组织集合，存续组织仅关闭本人关系；用户最终CLOSED并清空邮箱/密码。
3. CompleteAccountCloseReceipt同事务落终态与审计，恢复任务依赖generation/instance/lease CAS收尾。

本文件需要守住的边界：

- 不能用INNER JOIN隐藏孤立恢复任务。
- 组织仍有他人时不得关组织或清他人内容；任何源ACK缺失继续等待。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/account_close_completion_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/account_close_completion_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/account_close_completion_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/account_close_completion_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:420 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
)

// account_close_cancel.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountCloseCancelKind

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`const AccountCloseCancelKind = "ACCOUNT_CLOSE_CANCEL"`。

<!-- write-step: 2 bytes=420:475 -->
````go
const AccountCloseCancelKind = "ACCOUNT_CLOSE_CANCEL"

````

## 第 3 步：AccountCloseCancellations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type AccountCloseCancellations struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 3 bytes=475:528 -->
````go
type AccountCloseCancellations struct{ db *sql.DB }

````

## 第 4 步：NewAccountCloseCancellations

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func NewAccountCloseCancellations(db *sql.DB) *AccountCloseCancellations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseCancellations</code> | 实际返回表达式：<code>&amp;AccountCloseCancellations{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseCancellations{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=528:649 -->
````go
func NewAccountCloseCancellations(db *sql.DB) *AccountCloseCancellations {
	return &AccountCloseCancellations{db: db}
}

````

## 第 5 步：AccountCloseCancellations.Claim

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func (r *AccountCloseCancellations) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{AccountCloseCancelKind})</code>；<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? FOR UPDATE`, claimed.ID)</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{AccountCloseCancelKind})</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance == &quot;&quot;</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）；<code>task.Instance, task.State, task.LeaseValid = instance, &quot;RUNNING&quot;, true</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>domain.AccountCloseFenceTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CANCEL_CLAIM_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.AccountCloseFenceTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? FOR UPDATE`, claimed.ID)</code>。
- 检查 <code>err != nil &#124;&#124; instance == &quot;&quot;</code>。条件成立时执行 <code>return task, errors.New(&quot;ACCOUNT_CLOSE_CANCEL_CLAIM_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 此处通过 <code>return task, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=649:1747 -->
````go
func (r *AccountCloseCancellations) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{AccountCloseCancelKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? FOR UPDATE`, claimed.ID))
	if err != nil || instance == "" {
		return task, errors.New("ACCOUNT_CLOSE_CANCEL_CLAIM_INVALID")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
		WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, AccountCloseCancelKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

````

## 第 6 步：cancellationBinding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func cancellationBinding(task domain.AccountCloseFenceTask, user uint64, state, receiptKind, receiptState string, actor uint64, rawCapture []byte) (platform.AccountClosureBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>task.Revocation</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>binding.User != user</code>；<code>actor != user</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>state != &quot;REJECTED&quot;</code> |
| <code>receiptKind</code> | <code>string</code> | 在此声明中的实际使用：<code>receiptKind != &quot;ACCOUNT_CLOSE&quot;</code> |
| <code>receiptState</code> | <code>string</code> | 在此声明中的实际使用：<code>receiptState != &quot;REJECTED&quot;</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor != user</code> |
| <code>rawCapture</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>domain.DecodeAccountCloseCapture(rawCapture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>实际返回表达式：<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;binding) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; !platform.ValidateAccountClosureBinding(binding)</code>。条件成立时执行 <code>return binding, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; capture.Binding == nil &#124;&#124; !platform.EqualAccountClosureBinding(*capture.Binding, binding) &#124;&#124; binding.Operation != task.Operation &#124;&#124; task.Revocation != task.Operation  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return binding, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return binding, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1747:2721 -->
````go
func cancellationBinding(task domain.AccountCloseFenceTask, user uint64, state, receiptKind, receiptState string, actor uint64, rawCapture []byte) (platform.AccountClosureBinding, error) {
	var binding platform.AccountClosureBinding
	decoder := json.NewDecoder(bytes.NewReader(task.Payload))
	decoder.DisallowUnknownFields()
	invalid := errors.New("ACCOUNT_CLOSE_CANCEL_BINDING_INVALID")
	if decoder.Decode(&binding) != nil || decoder.Decode(new(any)) != io.EOF || !platform.ValidateAccountClosureBinding(binding) {
		return binding, invalid
	}
	capture, err := domain.DecodeAccountCloseCapture(rawCapture)
	if err != nil || capture.Binding == nil || !platform.EqualAccountClosureBinding(*capture.Binding, binding) || binding.Operation != task.Operation || task.Revocation != task.Operation || binding.User != user || actor != user || state != "REJECTED" || receiptKind != "ACCOUNT_CLOSE" || receiptState != "REJECTED" {
		return binding, invalid
	}
	return binding, nil
}

````

## 第 7 步：AccountCloseCancellations.Binding

插入位置：第 6 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func (r *AccountCloseCancellations) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (platform.AccountClosureBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT c.user_id,c.state,c.retention_plan,o.actor_id,o.kind,o.state 		FROM durable_task t JOIN account_close c ON c.operation_id=t.operation_id 		JOIN opera …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Revocation</code>；<code>task.Generation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>由 <code>cancellationBinding(task, user, state, kind, receiptState, actor, capture)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>platform.AccountClosureBinding{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>cancellationBinding(task, user, state, kind, receiptState, actor, capture)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT c.user_id,c.state,c.retention_plan,o.actor_id,o.kind,o.state 		FROM durable_task t JOIN account_close c ON c.operation_id=t.operation_id 		JOIN opera …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return platform.AccountClosureBinding{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return cancellationBinding(task, user, state, kind, receiptState, actor, capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2721:3676 -->
````go
func (r *AccountCloseCancellations) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (platform.AccountClosureBinding, error) {
	var user, actor uint64
	var state, kind, receiptState string
	var capture []byte
	err := q.QueryRowContext(ctx, `SELECT c.user_id,c.state,c.retention_plan,o.actor_id,o.kind,o.state
		FROM durable_task t JOIN account_close c ON c.operation_id=t.operation_id
		JOIN operation_receipt o ON o.operation_id=c.operation_id
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business_key=? AND t.generation=?
		AND t.state='RUNNING' AND t.instance_id=? AND t.lease_until>UTC_TIMESTAMP(6)`, task.ID, AccountCloseCancelKind, task.Operation, task.Revocation, task.Generation, task.Instance).
		Scan(&user, &state, &capture, &actor, &kind, &receiptState)
	if err != nil {
		return platform.AccountClosureBinding{}, err
	}
	return cancellationBinding(task, user, state, kind, receiptState, actor, capture)
}

````

## 第 8 步：AccountCloseCancellations.Retry

插入位置：第 7 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`func (r *AccountCloseCancellations) Retry(ctx context.Context, task domain.AccountCloseFenceTask)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL, 		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=&#x27;ACCOUNT_CLOSE_CA …</code>（长表达式，按本步完整代码继续） |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Generation</code>；<code>task.Instance</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL, 		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=&#x27;ACCOUNT_CLOSE_CA …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 8 bytes=3676:4166 -->
````go
func (r *AccountCloseCancellations) Retry(ctx context.Context, task domain.AccountCloseFenceTask) {
	_, _ = r.db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,
		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error='ACCOUNT_CLOSE_CANCEL_UNCONFIRMED'
		WHERE id=? AND kind=? AND operation_id=? AND generation=? AND instance_id=? AND state='RUNNING'`, task.ID, AccountCloseCancelKind, task.Operation, task.Generation, task.Instance)
}

````

## 第 9 步：AccountCloseCancellations.Finish

插入位置：第 8 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func (r *AccountCloseCancellations) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, binding platform.AccountClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, binding.User)</code>；<code>closures.Receipt(ctx, tx, binding.Operation)</code>；<code>closures.Record(ctx, tx, binding.Operation)</code>；<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseCancelKind)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, binding.User)</code>；<code>closures.Receipt(ctx, tx, binding.Operation)</code>；<code>closures.Record(ctx, tx, binding.Operation)</code>；<code>tx.QueryRowContext</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code>；<code>task.Payload</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.User</code>；<code>binding.Operation</code>；<code>platform.EqualAccountClosureBinding(actual, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CANCEL_OWNERSHIP_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CANCEL_BINDING_CHANGED&quot;)</code>；<code>accountCloseChanged(changed, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseCancelKind)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.Generation != task.Generation &#124;&#124; current.Instance != task.Instance &#124;&#124; current.State != &quot;RUNNING&quot; &#124;&#124; !current.LeaseValid &#124;&#124; !bytes.Equal(current.Payload, task.Payload)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_CANCEL_OWNERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!platform.EqualAccountClosureBinding(actual, binding)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_CANCEL_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,instance_id=NULL,lease_until=NULL,last_error=NULL 		WHERE id=? AND kind=? AND generation=? AND instance_id=? AND stat …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4166:5655 -->
````go
func (r *AccountCloseCancellations) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, binding platform.AccountClosureBinding) error {
	closures := NewAccountClosures(r.db)
	if _, err := closures.LockUser(ctx, tx, binding.User); err != nil {
		return err
	}
	if _, err := closures.Receipt(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if _, err := closures.Record(ctx, tx, binding.Operation); err != nil {
		return err
	}
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseCancelKind))
	if err != nil {
		return err
	}
	if current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_CANCEL_OWNERSHIP_CHANGED")
	}
	actual, err := r.Binding(ctx, tx, current)
	if err != nil {
		return err
	}
	if !platform.EqualAccountClosureBinding(actual, binding) {
		return errors.New("ACCOUNT_CLOSE_CANCEL_BINDING_CHANGED")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',instance_id=NULL,lease_until=NULL,last_error=NULL
		WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, task.ID, AccountCloseCancelKind, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

````

## 第 10 步：AccountCloseCancellations.Ready

插入位置：第 9 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func (r *AccountCloseCancellations) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload, 			COALESCE(c.user_id,0),COALESCE(c.state,&#x27;&#x27;),c.retention_plan,COALESCE(o.actor_id,0),COAL …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload, 			COALESCE(c.user_id,0),COALESCE(c.state,&#x27;&#x27;),c.retention_plan,COALESCE(o.actor_id,0),COAL …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;task.ID, &amp;task.Operation, &amp;task.Revocation, &amp;task.Payload, &amp;user, &amp;state, &amp;capture, &amp;actor, &amp;kind, &amp;receiptState)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; count &lt; 100</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=5655:6969 -->
````go
func (r *AccountCloseCancellations) Ready(ctx context.Context) error {
	after := uint64(0)
	for {
		rows, err := r.db.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,''),t.business_key,t.payload,
			COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,'')
			FROM durable_task t LEFT JOIN account_close c ON c.operation_id=t.operation_id LEFT JOIN operation_receipt o ON o.operation_id=c.operation_id
			WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, AccountCloseCancelKind, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var task domain.AccountCloseFenceTask
			var user, actor uint64
			var state, kind, receiptState string
			var capture []byte
			err = rows.Scan(&task.ID, &task.Operation, &task.Revocation, &task.Payload, &user, &state, &capture, &actor, &kind, &receiptState)
			if err == nil {
				_, err = cancellationBinding(task, user, state, kind, receiptState, actor, capture)
			}
			if err != nil {
				rows.Close()
				return err
			}
			after, count = task.ID, count+1
		}
		err = rows.Err()
		rows.Close()
		if err != nil || count < 100 {
			return err
		}
	}
}

// account_close_confirmation.go：保留原实现，按业务合并文件。

````

## 第 11 步：AccountClosures.ConfirmedScopes

插入位置：第 10 步代码之后，紧接追加；完成稿约第 152 行。

用户锁先于本方法：屏障完成事务也锁同一用户，确认读取不会把不同提交拼成ACK。
每页仅按种类和精确business_key访问本计划的任务，避免扫描历史队列。

本步声明：`func (r *AccountClosures) ConfirmedScopes(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload,t.state, 			COALESCE(a.actor_id,0),COALESCE(a.scope_key,&#x27;&#x27;),COALESCE(a.epoch,0),COALESCE(a.state,&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>validateAccountCloseFreeze(binding, capture)</code>；<code>binding.Operation</code>；<code>binding.User</code> |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>validateAccountCloseFreeze(binding, capture)</code>；<code>capture.Scopes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>len(expected) == 0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_CONFIRMATION_ACK_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>end &gt; len(capture.Scopes)</code>。
- 执行 <code>tx.QueryContext(ctx, `SELECT COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload,t.state, 			COALESCE(a.actor_id,0),COALESCE(a.scope_key,&#x27;&#x27;),COALESCE(a.epoch,0),COALESCE(a.state,&#x27; …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;operation, &amp;revocation, &amp;payload, &amp;taskState, &amp;actor, &amp;scope, &amp;epoch, &amp;authorizationState, &amp;target, &amp;participants, &amp;gateEpoch, &amp;gateState, &amp;gateRevocation, &amp;digest, &amp;phase)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!found &#124;&#124; decodeErr != nil &#124;&#124; operation != binding.Operation &#124;&#124; parsed.Operation != binding.Operation &#124;&#124; parsed.User != binding.User &#124;&#124; parsed.Revocation != revocation &#124;&#124; parsed.Sc …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return false, errors.New(&quot;ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6969:9804 -->
````go
// 用户锁先于本方法：屏障完成事务也锁同一用户，确认读取不会把不同提交拼成ACK。
// 每页仅按种类和精确business_key访问本计划的任务，避免扫描历史队列。
func (r *AccountClosures) ConfirmedScopes(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) (bool, error) {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return false, err
	}
	expected := make(map[string]domain.AccountCloseScope, len(capture.Scopes))
	for _, source := range capture.Scopes {
		expected[source.Revocation] = source
	}
	for offset := 0; offset < len(capture.Scopes); offset += 100 {
		end := offset + 100
		if end > len(capture.Scopes) {
			end = len(capture.Scopes)
		}
		args := []any{domain.AccountCloseFenceKind}
		for _, source := range capture.Scopes[offset:end] {
			args = append(args, source.Revocation)
		}
		rows, err := tx.QueryContext(ctx, `SELECT COALESCE(t.operation_id,''),t.business_key,t.payload,t.state,
			COALESCE(a.actor_id,0),COALESCE(a.scope_key,''),COALESCE(a.epoch,0),COALESCE(a.state,''),a.target_permission,a.participant_plan,
			COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,''),ack.result_digest,
			COALESCE(ack.phase,'')
			FROM durable_task t LEFT JOIN authorization_change a ON a.revocation_id=t.business_key
			LEFT JOIN access_gate g ON g.scope_key=a.scope_key
			LEFT JOIN authorization_ack ack ON ack.revocation_id=a.revocation_id AND ack.participant='document' AND ack.scope_key=a.scope_key AND ack.epoch=a.epoch
			WHERE t.kind=? AND t.business_key IN (`+strings.TrimSuffix(strings.Repeat("?,", end-offset), ",")+`)`, args...)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var operation, revocation, taskState, scope, authorizationState, gateState, gateRevocation, phase string
			var actor, epoch, gateEpoch uint64
			var payload, target, participants, digest []byte
			err = rows.Scan(&operation, &revocation, &payload, &taskState, &actor, &scope, &epoch, &authorizationState, &target, &participants, &gateEpoch, &gateState, &gateRevocation, &digest, &phase)
			if err != nil {
				rows.Close()
				return false, err
			}
			source, found := expected[revocation]
			parsed, decodeErr := domain.DecodeAccountCloseFence(payload)
			if !found || decodeErr != nil || operation != binding.Operation || parsed.Operation != binding.Operation || parsed.User != binding.User || parsed.Revocation != revocation || parsed.Scope != source.Scope || parsed.Epoch != source.Epoch || actor != binding.User || scope != source.Scope || epoch != source.Epoch || gateEpoch != epoch || gateState != "BLOCKED" || gateRevocation != revocation {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
````

## 第 12 步：AccountClosures.ConfirmedScopes（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 198 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(target, &amp;targetBinding) != nil &#124;&#124; targetBinding.Operation != operation &#124;&#124; targetBinding.Scope != scope &#124;&#124; targetBinding.Epoch != epoch &#124;&#124; json.Unmarshal(participants …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return false, errors.New(&quot;ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>taskState != &quot;SUCCEEDED&quot; &#124;&#124; authorizationState != &quot;SUCCEEDED&quot;</code>。
- 此处通过 <code>return false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>phase != &quot;OUTPUT_STOPPED&quot; &#124;&#124; !bytes.Equal(digest, platform.BootstrapDigest(request, &quot;document&quot;))</code>。
- 此处通过 <code>return false, errors.New(&quot;ACCOUNT_CLOSE_CONFIRMATION_ACK_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>count != end-offset</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return len(expected) == 0, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9804:11175 -->
````go
			var targetBinding struct {
				Operation string `json:"account_close_operation"`
				Scope     string `json:"scope_key"`
				Epoch     uint64 `json:"epoch"`
			}
			var participantPlan []string
			if json.Unmarshal(target, &targetBinding) != nil || targetBinding.Operation != operation || targetBinding.Scope != scope || targetBinding.Epoch != epoch || json.Unmarshal(participants, &participantPlan) != nil || len(participantPlan) != 1 || participantPlan[0] != "document" {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
			if taskState != "SUCCEEDED" || authorizationState != "SUCCEEDED" {
				rows.Close()
				return false, nil
			}
			request := &pb.FenceRequest{RevocationId: revocation, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}}, TargetDigest: platform.SourceClosureDigest(scope, epoch)}
			if phase != "OUTPUT_STOPPED" || !bytes.Equal(digest, platform.BootstrapDigest(request, "document")) {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_ACK_INVALID")
			}
			delete(expected, revocation)
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count != end-offset {
			return false, nil
		}
	}
	return len(expected) == 0, nil
}

// account_close_finalize.go：保留原实现，按业务合并文件。

````

## 第 13 步：FinalizeAccountCloseData

插入位置：第 12 步代码之后，紧接追加；完成稿约第 234 行。

调用方已持原user/receipt/account_close锁且核对Doc完成证明和全部原ACK。
组织按原编号锁定，存续组织只关闭本人关系，不删除其他人的内容或分配负责人。

本步声明：`func FinalizeAccountCloseData(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE&quot;, organization.ID)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization.ID, binding.User)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id&lt;&gt;? AND state&lt;&gt;&#x27;CLOSED&#x27;)&quot;, organization.ID, binding.User)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=&#x27;已关闭组织&#x27;,lifecycle=&#x27;CLOSED&#x27;,closed_at=UTC_TIMESTAMP(6),member_revision=member_revision+1 WHERE id=?&quot;, organization.ID)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.ClosingOrganizations</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code> |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>capture.Organizations</code>；<code>capture.CredentialEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_ORGANIZATION_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_MEMBERSHIP_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_ORGANIZATION_NOT_EMPTY&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE&quot;, organization.ID).Scan(&amp;owner, &amp;state, &amp;memberRevision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE&quot;, organization.ID)</code>。
- 检查 <code>memberRevision == math.MaxUint64 &#124;&#124; (closing &amp;&amp; (state != &quot;CLOSING&quot; &#124;&#124; owner != binding.User)) &#124;&#124; (!closing &amp;&amp; (state != &quot;ACTIVE&quot; &#124;&#124; owner == binding.User))</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_ORGANIZATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization.ID, binding.User).Scan(&amp;membership, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization.ID, binding.User)</code>。
- 检查 <code>organization.Epoch == math.MaxUint64 &#124;&#124; membership != &quot;REVOKING&quot; &#124;&#124; epoch != organization.Epoch+1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_MEMBERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>closing</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id&lt;&gt;? AND state&lt;&gt;&#x27;CLOSED&#x27;)&quot;, organization.ID, binding.User).Scan(&amp;others)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id&lt;&gt;? AND state&lt;&gt;&#x27;CLOSED&#x27;)&quot;, organization.ID, binding.User)</code>。
- 检查 <code>others</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_ORGANIZATION_NOT_EMPTY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization SET name=&#x27;已关闭组织&#x27;,lifecycle=&#x27;CLOSED&#x27;,closed_at=UTC_TIMESTAMP(6),member_revision=member_revision+1 WHERE id=?&quot;, organization.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization SET member_revision=member_revision+1 WHERE id=?&quot;, organization.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=?&quot;, organization.ID, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; AND revision=18446744073709551615) 		 OR E …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; AND revision=18446744073709551615) 		 OR E …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>saturated</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=11175:14063 -->
````go
// 调用方已持原user/receipt/account_close锁且核对Doc完成证明和全部原ACK。
// 组织按原编号锁定，存续组织只关闭本人关系，不删除其他人的内容或分配负责人。
func FinalizeAccountCloseData(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error {
	for _, organization := range capture.Organizations {
		var owner, memberRevision uint64
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE", organization.ID).Scan(&owner, &state, &memberRevision); err != nil {
			return err
		}
		closing := slices.Contains(binding.ClosingOrganizations, organization.ID)
		if memberRevision == math.MaxUint64 || (closing && (state != "CLOSING" || owner != binding.User)) || (!closing && (state != "ACTIVE" || owner == binding.User)) {
			return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_CHANGED")
		}
		var epoch uint64
		var membership string
		if err := tx.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization.ID, binding.User).Scan(&membership, &epoch); err != nil {
			return err
		}
		if organization.Epoch == math.MaxUint64 || membership != "REVOKING" || epoch != organization.Epoch+1 {
			return errors.New("ACCOUNT_CLOSE_FINAL_MEMBERSHIP_CHANGED")
		}
		if closing {
			var others bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id<>? AND state<>'CLOSED')", organization.ID, binding.User).Scan(&others); err != nil {
				return err
			}
			if others {
				return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_NOT_EMPTY")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE organization SET name='已关闭组织',lifecycle='CLOSED',closed_at=UTC_TIMESTAMP(6),member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, "UPDATE organization SET member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=?", organization.ID, binding.User); err != nil {
			return err
		}
		var saturated bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision=18446744073709551615)
		 OR EXISTS(SELECT 1 FROM organization_invitation WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision=18446744073709551615)`, organization.ID, binding.User, organization.ID, binding.User).Scan(&saturated); err != nil {
			return err
		}
		if saturated {
			return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
		}
````

## 第 14 步：FinalizeAccountCloseData（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 280 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_creation_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE organization_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; AND revision&lt;18446744073709551615&quot;, organization.ID, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_invitation SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE organization_id=? AND inviter_id=? AND state=&#x27;OPEN&#x27; AND revision&lt;18446744073709551615&quot;, organization.ID, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM session WHERE user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, binding.User).Scan(&amp;active)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM session WHERE user_id=? AND state=&#x27;ACTIVE&#x27;)&quot;, binding.User)</code>。
- 检查 <code>active</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_SESSION_NOT_FROZEN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND state=&#x27;REVOKING&#x27;&quot;, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM organization_application WHERE applicant_id=? AND state=&#x27;PENDING&#x27; AND revision=18446744073709551615)&quot;, binding.User).Scan(&amp;saturated)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM organization_application WHERE applicant_id=? AND state=&#x27;PENDING&#x27; AND revision=18446744073709551615)&quot;, binding.User)</code>。
- 检查 <code>saturated</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_application SET state=&#x27;CANCELLED&#x27;,revision=revision+1 WHERE applicant_id=? AND state=&#x27;PENDING&#x27; AND revision&lt;18446744073709551615&quot;, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, `UPDATE user SET state=&#x27;CLOSED&#x27;,email=NULL,email_key=NULL,password_hash=NULL,password_algorithm=NULL, 	 display_name=&#x27;已注销用户&#x27;,email_verified_at=NULL,closed_at=UT …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14063:16058 -->
````go
		if _, err := tx.ExecContext(ctx, "UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_invitation SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
	}
	var active bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM session WHERE user_id=? AND state='ACTIVE')", binding.User).Scan(&active); err != nil {
		return err
	}
	if active {
		return errors.New("ACCOUNT_CLOSE_FINAL_SESSION_NOT_FROZEN")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE session SET state='CLOSED' WHERE user_id=? AND state='REVOKING'", binding.User); err != nil {
		return err
	}
	var saturated bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_application WHERE applicant_id=? AND state='PENDING' AND revision=18446744073709551615)", binding.User).Scan(&saturated); err != nil {
		return err
	}
	if saturated {
		return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE organization_application SET state='CANCELLED',revision=revision+1 WHERE applicant_id=? AND state='PENDING' AND revision<18446744073709551615", binding.User); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='CLOSED',email=NULL,email_key=NULL,password_hash=NULL,password_algorithm=NULL,
	 display_name='已注销用户',email_verified_at=NULL,closed_at=UTC_TIMESTAMP(6),credential_epoch=credential_epoch+1
	 WHERE id=? AND state='REVOKING' AND identity_epoch=? AND credential_epoch=? AND credential_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch+1, capture.CredentialEpoch)
	return accountCloseChanged(changed, err)
}

````

## 第 15 步：CompleteAccountCloseReceipt

插入位置：第 14 步代码之后，紧接追加；完成稿约第 313 行。

本步声明：`func CompleteAccountCloseReceipt(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, digest string, plan []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND stat …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(NULL,?,1,&#x27;ACCOUNT_CLOSE_COMPLETE&#x27;,?)&quot;, binding.Operation, string(target))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>binding.User</code> |
| <code>digest</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;document_plan_digest&quot;: digest</code> |
| <code>plan</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND stat …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(NULL,?,1,&#x27;ACCOUNT_CLOSE_COMPLETE&#x27;,?)&quot;, binding.Operation, string(target))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 15 bytes=16058:16943 -->
````go
func CompleteAccountCloseReceipt(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, digest string, plan []byte) error {
	changed, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING'", string(plan), binding.Operation, binding.User)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	target, err := json.Marshal(map[string]any{"user_id": binding.User, "document_plan_digest": digest})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(NULL,?,1,'ACCOUNT_CLOSE_COMPLETE',?)", binding.Operation, string(target))
	return err
}

// account_close_progress.go：保留原实现，按业务合并文件。

````

## 第 16 步：AccountClosures.ProgressBinding

插入位置：第 15 步代码之后，紧接追加；完成稿约第 328 行。

这里只取原会话绑定；对外进度仍由用例在user→receipt→account_close锁内重检。

本步声明：`func (r *AccountClosures) ProgressBinding(ctx context.Context, operation string) (domain.AccountCloseReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 	 FROM operation_receipt WHERE operation_id=?`, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 	 FROM operation_receipt WHERE operation_id=?`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseReceipt</code> | 使用 <code>domain.AccountCloseReceipt</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 	 FROM operation_receipt WHERE operation_id=?`, operation).Sca …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;) 	 FROM operation_receipt WHERE operation_id=?`, operation)</code>。
- 此处通过 <code>return receipt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=16943:17549 -->
````go
// 这里只取原会话绑定；对外进度仍由用例在user→receipt→account_close锁内重检。
func (r *AccountClosures) ProgressBinding(ctx context.Context, operation string) (domain.AccountCloseReceipt, error) {
	var receipt domain.AccountCloseReceipt
	err := r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),'')
	 FROM operation_receipt WHERE operation_id=?`, operation).Scan(&receipt.User, &receipt.Kind, &receipt.SessionHash)
	return receipt, err
}

// account_close_recovery.go：保留原实现，按业务合并文件。

````

## 第 17 步：AccountCloseRecoveryKind

插入位置：第 16 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`const AccountCloseRecoveryKind = "ACCOUNT_CLOSE_RECOVER"`。

<!-- write-step: 17 bytes=17549:17607 -->
````go
const AccountCloseRecoveryKind = "ACCOUNT_CLOSE_RECOVER"

````

## 第 18 步：AccountCloseRecoveryPayload

插入位置：第 17 步代码之后，紧接追加；完成稿约第 340 行。

本步声明：`type AccountCloseRecoveryPayload struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>User</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |

<!-- write-step: 18 bytes=17607:17727 -->
````go
type AccountCloseRecoveryPayload struct {
	Operation string `json:"operation_id"`
	User      uint64 `json:"user_id"`
}

````

## 第 19 步：QueueAccountCloseRecovery

插入位置：第 18 步代码之后，紧接追加；完成稿约第 345 行。

本步声明：`func QueueAccountCloseRecovery(ctx context.Context, tx *sql.Tx, operation string, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(?,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_ke …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>Operation: operation</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(?,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_ke …</code>（长表达式，按本步完整代码继续） |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>User: user</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 	 VALUES(?,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_ke …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 19 bytes=17727:18243 -->
````go
func QueueAccountCloseRecovery(ctx context.Context, tx *sql.Tx, operation string, user uint64) error {
	raw, err := json.Marshal(AccountCloseRecoveryPayload{Operation: operation, User: user})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)`, AccountCloseRecoveryKind, operation, operation, string(raw))
	return err
}

````

## 第 20 步：ClaimAccountCloseRecovery

插入位置：第 19 步代码之后，紧接追加；完成稿约第 355 行。

本步声明：`func ClaimAccountCloseRecovery(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{AccountCloseRecoveryKind})</code>；<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, claimed.ID, AccountCloseRecoveryKind)</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{AccountCloseRecoveryKind})</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance == &quot;&quot;</code>；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）；<code>task.Instance, task.State, task.LeaseValid = instance, &quot;RUNNING&quot;, true</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>domain.AccountCloseFenceTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_CLAIM_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.AccountCloseFenceTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, claimed.ID, AccountCloseRecoveryKind)</code>。
- 检查 <code>err != nil &#124;&#124; instance == &quot;&quot;</code>。条件成立时执行 <code>return task, errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_CLAIM_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=generation+1, 	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts&lt; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 此处通过 <code>return task, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=18243:19373 -->
````go
func ClaimAccountCloseRecovery(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{AccountCloseRecoveryKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, claimed.ID, AccountCloseRecoveryKind))
	if err != nil || instance == "" {
		return task, errors.New("ACCOUNT_CLOSE_RECOVERY_CLAIM_INVALID")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
	 WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, AccountCloseRecoveryKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

````

## 第 21 步：ValidateAccountCloseRecovery

插入位置：第 20 步代码之后，紧接追加；完成稿约第 375 行。

只在调用方user→receipt→account_close锁之后核对任务租约。网络结果不能
允许已过租约的旧领取者推进本地游标；下一领取者仍按原Doc回执重放。

本步声明：`func ValidateAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseRecoveryKind)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Generation</code>；<code>task.Instance</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_OWNERSHIP_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseRecoveryKind)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.Operation != task.Operation &#124;&#124; current.Revocation != task.Operation &#124;&#124; current.Generation != task.Generation &#124;&#124; current.Instance != task.Instance &#124;&#124; current.State != &quot;RUNNI …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_OWNERSHIP_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=19373:20254 -->
````go
// 只在调用方user→receipt→account_close锁之后核对任务租约。网络结果不能
// 允许已过租约的旧领取者推进本地游标；下一领取者仍按原Doc回执重放。
func ValidateAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error {
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseRecoveryKind))
	if err != nil {
		return err
	}
	if current.Operation != task.Operation || current.Revocation != task.Operation || current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_OWNERSHIP_CHANGED")
	}
	return nil
}

````

## 第 22 步：FinishAccountCloseRecovery

插入位置：第 21 步代码之后，紧接追加；完成稿约第 388 行。

本步声明：`func FinishAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6) 	 WHERE id=? AND kind=? AND generation=? AND instan …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>terminal</code> | <code>bool</code> | true表示恢复已到终态，任务置SUCCEEDED；false表示本轮仅推进了一部分，重新置QUEUED。两种情况都按任务ID、代次、实例与有效租约条件写回。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>accountCloseChanged(changed, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>terminal</code>。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6) 	 WHERE id=? AND kind=? AND generation=? AND instan …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=20254:20815 -->
````go
func FinishAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error {
	state := "QUEUED"
	if terminal {
		state = "SUCCEEDED"
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, state, task.ID, AccountCloseRecoveryKind, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

````

## 第 23 步：RetryAccountCloseRecovery

插入位置：第 22 步代码之后，紧接追加；完成稿约第 398 行。

本步声明：`func RetryAccountCloseRecovery(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) 	 WHERE id=? AND kin …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=20815:21275 -->
````go
func RetryAccountCloseRecovery(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING'`, code, task.ID, AccountCloseRecoveryKind, task.Generation, task.Instance)
	return err
}

````

## 第 24 步：SaveAccountCloseRetention

插入位置：第 23 步代码之后，紧接追加；完成稿约第 404 行。

本步声明：`func SaveAccountCloseRetention(ctx context.Context, tx *sql.Tx, operation, oldState, newState string, capture, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1 	 WHERE operation_id=? AND state=? AND generation&lt;18446744073709551615`, newState, string(capture), operation, oldState)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 	 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1 	 WHERE operation_id=? AND state=? AND generation&lt;18446744073709551615`, newState, string(capture), operation, oldState)</code>；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 	 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续） |
| <code>oldState</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1 	 WHERE operation_id=? AND state=? AND generation&lt;18446744073709551615`, newState, string(capture), operation, oldState)</code> |
| <code>newState</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1 	 WHERE operation_id=? AND state=? AND generation&lt;18446744073709551615`, newState, string(capture), operation, oldState)</code> |
| <code>capture</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(capture)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>accountCloseChanged(changed, err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1 	 WHERE operation_id=? AND state=? AND generation&lt;18446744073709551615`, newState, string(capture), operation, oldState)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1 	 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PREPARING&#x27; AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return accountCloseChanged(changed, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=21275:22000 -->
````go
func SaveAccountCloseRetention(ctx context.Context, tx *sql.Tx, operation, oldState, newState string, capture, result []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1
	 WHERE operation_id=? AND state=? AND generation<18446744073709551615`, newState, string(capture), operation, oldState)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
	 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(result), operation)
	return accountCloseChanged(changed, err)
}

````

## 第 25 步：RecoveryPlanRow

插入位置：第 24 步代码之后，紧接追加；完成稿约第 415 行。

RecoveryPlanRow 保留孤立任务的 LEFT JOIN 结果，让业务校验拒绝损坏绑定。

本步声明：`type RecoveryPlanRow struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。 |
| <code>Record</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。 |
| <code>Receipt</code> | <code>domain.AccountCloseReceipt</code> | 使用 <code>domain.AccountCloseReceipt</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。 |

<!-- write-step: 25 bytes=22000:22239 -->
````go
// RecoveryPlanRow 保留孤立任务的 LEFT JOIN 结果，让业务校验拒绝损坏绑定。
type RecoveryPlanRow struct {
	Task    domain.AccountCloseFenceTask
	Record  domain.AccountCloseRecord
	Receipt domain.AccountCloseReceipt
}

````

## 第 26 步：AccountCloseRecoveryPage

插入位置：第 25 步代码之后，紧接追加；完成稿约第 422 行。

本步声明：`func AccountCloseRecoveryPage(ctx context.Context, q *sql.DB, after uint64) ([]RecoveryPlanRow, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload, 		 COALESCE(c.user_id,0),COALESCE(c.state,&#x27;&#x27;),c.retention_plan, 		 COALESCE(o.actor_id,0),COA …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload, 		 COALESCE(c.user_id,0),COALESCE(c.state,&#x27;&#x27;),c.retention_plan, 		 COALESCE(o.actor_id,0),COA …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;RecoveryPlanRow</code> | 使用 <code>RecoveryPlanRow</code> 类型；其定义处的职责：RecoveryPlanRow 保留孤立任务的 LEFT JOIN 结果，让业务校验拒绝损坏绑定。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,&#x27;&#x27;),t.business_key,t.payload, 		 COALESCE(c.user_id,0),COALESCE(c.state,&#x27;&#x27;),c.retention_plan, 		 COALESCE(o.actor_id,0),COA …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;task.ID, &amp;task.Operation, &amp;task.Revocation, &amp;task.Payload, &amp;record.User, &amp;record.State, &amp;record.Capture, &amp;receipt.User, &amp;receipt.Kind, &amp;receipt.State, &amp;receipt.SessionHash, &amp;receipt.Result)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=22239:23469 -->
````go
func AccountCloseRecoveryPage(ctx context.Context, q *sql.DB, after uint64) ([]RecoveryPlanRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,''),t.business_key,t.payload,
		 COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
		 COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),
		 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.session_hash')),''),o.result_json
		 FROM durable_task t LEFT JOIN account_close c ON c.operation_id=t.operation_id
		 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		 WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, AccountCloseRecoveryKind, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RecoveryPlanRow, 0, 100)
	for rows.Next() {
		var row RecoveryPlanRow
		task, record, receipt := &row.Task, &row.Record, &row.Receipt
		if err := rows.Scan(&task.ID, &task.Operation, &task.Revocation, &task.Payload, &record.User, &record.State, &record.Capture, &receipt.User, &receipt.Kind, &receipt.State, &receipt.SessionHash, &receipt.Result); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/account_close_completion_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
