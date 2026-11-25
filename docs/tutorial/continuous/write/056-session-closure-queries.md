# 分步编写 `services/account/internal/repository/account/session_closure_queries.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/056-session-closure-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化来源关闭、领取恢复任务、保存真实Doc ACK并完成会话或成员收尾。

登出和自然到期都需要广播停止已有输出，单改session状态不足以阻断跨服务旧请求。

写之前先按这条执行路线理解各段的连接：

1. PrepareSessionClose同事务写PREPARING回执、REVOKING会话、BLOCKED gate与SESSION_LOGOUT任务。
2. claimClosureTask先取有界候选再FOR UPDATE SKIP LOCKED，MarkRunning设置5秒租约。
3. FinishBase保存OUTPUT_STOPPED ACK、关闭session、结束授权/回执/任务；具体成员或凭据收尾与它同事务。

本文件需要守住的边界：

- 租约过期仅允许重新领取，不证明业务已完成。
- Retry按instance和RUNNING约束更新，旧worker不得覆盖新worker状态。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/session_closure_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/session_closure_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/session_closure_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/session_closure_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:298 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
	"errors"
	"strings"
)

// closure_enqueue.go：保留原实现，按业务合并文件。

````

## 第 2 步：EnqueueSourceClosure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

EnqueueSourceClosure只使用用例传入的事务，不能提前提交门闩或恢复任务。

本步声明：`func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch …</code>（长表达式，按本步完整代码继续） |
| <code>payload</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>payload.Kind</code>；<code>payload.Scope</code>；<code>payload.Epoch</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>payload.Kind == &quot;PLATFORM_GOVERNANCE_REVOKE&quot;</code>。
- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(ma …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=298:1666 -->
````go
// EnqueueSourceClosure只使用用例传入的事务，不能提前提交门闩或恢复任务。
func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure, target []byte) error {
	kind := "ORGANIZATION_CHANGE"
	if payload.Kind == "PLATFORM_GOVERNANCE_REVOKE" {
		kind = "PLATFORM_GOVERNANCE_CHANGE"
	}
	return execute(ctx, tx, []statement{
		{"INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1", []any{payload.Scope, payload.Epoch, id}},
		{"UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=?", []any{payload.Scope, payload.Epoch}},
		{"INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,?,?,?,'FENCING','[\"document\"]',UTC_TIMESTAMP(6))", []any{id, actor, payload.Scope, payload.Epoch, string(target)}},
		{"INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))", []any{kind, id, id, string(target)}},
	})
}

// closure_finish.go：保留原实现，按业务合并文件。

````

## 第 3 步：Closure.FinishBase

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (r *Closure) FinishBase(ctx context.Context, tx *sql.Tx, task domain.ClosureTask, binding domain.SessionClosure, session, resultDigest, terminal []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>task</code> | <code>domain.ClosureTask</code> | 使用 <code>domain.ClosureTask</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>在此声明中的实际使用：<code>task.OperationID</code>；<code>task.ID</code> |
| <code>binding</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>binding.Scope</code>；<code>binding.Epoch</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>resultDigest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>terminal</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(terminal)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27; …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;document&#x27;,?,?,&#x27;OUTPUT_S …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1666:2741 -->
````go
func (r *Closure) FinishBase(ctx context.Context, tx *sql.Tx, task domain.ClosureTask, binding domain.SessionClosure, session, resultDigest, terminal []byte) error {
	return execute(ctx, tx, []statement{
		{"INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", []any{task.OperationID, binding.Scope, binding.Epoch, resultDigest}},
		{"UPDATE session SET state='CLOSED' WHERE id=? AND state='REVOKING'", []any{session}},
		{"UPDATE authorization_change SET state='SUCCEEDED',last_error=NULL,retry_at=NULL WHERE revocation_id=?", []any{task.OperationID}},
		{"UPDATE operation_receipt SET state='SUCCEEDED',result_json=IF(result_json IS NULL,?,JSON_SET(result_json,'$.state','SUCCEEDED')),terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=?", []any{string(terminal), task.OperationID}},
		{"UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=?", []any{task.ID}},
	})
}

````

## 第 4 步：Closure.FinishCredential

插入位置：第 3 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func (r *Closure) FinishCredential(ctx context.Context, tx *sql.Tx, userID string, oldEpoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user SET sta …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user SET sta …</code>（长表达式，按本步完整代码继续） |
| <code>userID</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user SET sta …</code>（长表达式，按本步完整代码继续） |
| <code>oldEpoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user SET sta …</code>（长表达式，按本步完整代码继续）；<code>oldEpoch + 1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user SET sta …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE session SET state=&#x27;CLOSED&#x27; WHERE user_id=? AND credential_epoch&lt;=? AND state=&#x27;REVOKING&#x27;&quot;, &#91;&#93;any{userID, oldEpoch}}, 		{&quot;UPDATE user  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2741:3143 -->
````go
func (r *Closure) FinishCredential(ctx context.Context, tx *sql.Tx, userID string, oldEpoch uint64) error {
	return execute(ctx, tx, []statement{
		{"UPDATE session SET state='CLOSED' WHERE user_id=? AND credential_epoch<=? AND state='REVOKING'", []any{userID, oldEpoch}},
		{"UPDATE user SET state='ACTIVE' WHERE id=? AND state='REVOKING' AND credential_epoch=?", []any{userID, oldEpoch + 1}},
	})
}

````

## 第 5 步：Closure.FinishMemberRole

插入位置：第 4 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func (r *Closure) FinishMemberRole(ctx context.Context, tx *sql.Tx, org, user, role string, nextEpoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>org</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code> |
| <code>nextEpoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_member SET state=&#x27;ACTIVE&#x27;,role=? WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, role, org, user, nextEpoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3143:3462 -->
````go
func (r *Closure) FinishMemberRole(ctx context.Context, tx *sql.Tx, org, user, role string, nextEpoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_member SET state='ACTIVE',role=? WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", role, org, user, nextEpoch)
	return err
}

````

## 第 6 步：Closure.FinishMemberDeparture

插入位置：第 5 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (r *Closure) FinishMemberDeparture(ctx context.Context, tx *sql.Tx, org, user string, nextEpoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |
| <code>org</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |
| <code>nextEpoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nextEpoch} …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{&quot;UPDATE organization_member SET state=&#x27;CLOSED&#x27; WHERE organization_id=? AND user_id=? AND state=&#x27;REVOKING&#x27; AND epoch=?&quot;, &#91;&#93;any{org, user, nex …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3462:3933 -->
````go
func (r *Closure) FinishMemberDeparture(ctx context.Context, tx *sql.Tx, org, user string, nextEpoch uint64) error {
	return execute(ctx, tx, []statement{
		{"UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", []any{org, user, nextEpoch}},
		{"UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND user_id=? AND state='ACTIVE'", []any{org, user}},
	})
}

````

## 第 7 步：Closure.FinishOrganizationTransition

插入位置：第 6 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func (r *Closure) FinishOrganizationTransition(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET state=&#x27;SUCCEEDED&#x27; WHERE operation_id=? AND state=&#x27;FENCING&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET state=&#x27;SUCCEEDED&#x27; WHERE operation_id=? AND state=&#x27;FENCING&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE organization_transition SET state=&#x27;SUCCEEDED&#x27; WHERE operation_id=? AND state=&#x27;FENCING&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3933:4258 -->
````go
func (r *Closure) FinishOrganizationTransition(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_transition SET state='SUCCEEDED' WHERE operation_id=? AND state='FENCING'", operation)
	return err
}

// closure_task.go：保留原实现，按业务合并文件。

````

## 第 8 步：Closure

插入位置：第 7 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`type Closure struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 8 bytes=4258:4293 -->
````go
type Closure struct{ db *sql.DB }

````

## 第 9 步：NewClosure

插入位置：第 8 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func NewClosure(db *sql.DB) *Closure`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Closure</code> | 实际返回表达式：<code>&amp;Closure{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Closure{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4293:4359 -->
````go
func NewClosure(db *sql.DB) *Closure { return &Closure{db: db} }

````

## 第 10 步：Closure.Claim

插入位置：第 9 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func (r *Closure) Claim(ctx context.Context, tx *sql.Tx) (domain.ClosureTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{&quot;SESSION_LOGOUT&quot;, &quot;CREDENTIAL_CHANGE&quot;, &quot;ORGANIZATION_CHANGE&quot;, &quot;PLATFORM_GOVERNANCE_CHANGE&quot;})</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimClosureTask(ctx, tx, &#91;&#93;string{&quot;SESSION_LOGOUT&quot;, &quot;CREDENTIAL_CHANGE&quot;, &quot;ORGANIZATION_CHANGE&quot;, &quot;PLATFORM_GOVERNANCE_CHANGE&quot;})</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ClosureTask</code> | 使用 <code>domain.ClosureTask</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>由 <code>claimClosureTask(ctx, tx, &#91;&#93;string{&quot;SESSION_LOGOUT&quot;, &quot;CREDENTIAL_CHANGE&quot;, &quot;ORGANIZATION_CHANGE&quot;, &quot;PLATFORM_GOVERNANCE_CHANGE&quot;})</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimClosureTask(ctx, tx, &#91;&#93;string{&quot;SESSION_LOGOUT&quot;, &quot;CREDENTIAL_CHANGE&quot;, &quot;ORGANIZATION_CHANGE&quot;, &quot;PLATFORM_GOVERNANCE_CHANGE&quot;})</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimClosureTask(ctx, tx, &#91;&#93;string{&quot;SESSION_LOGOUT&quot;, &quot;CREDENTIAL_CHANGE&quot;, &quot;ORGANIZATION_CHANGE&quot;, &quot;PLATFORM_GOVERNANCE_CHANGE&quot;})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4359:4585 -->
````go
func (r *Closure) Claim(ctx context.Context, tx *sql.Tx) (domain.ClosureTask, error) {
	return claimClosureTask(ctx, tx, []string{"SESSION_LOGOUT", "CREDENTIAL_CHANGE", "ORGANIZATION_CHANGE", "PLATFORM_GOVERNANCE_CHANGE"})
}

````

## 第 11 步：claimClosureTask

插入位置：第 10 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func claimClosureTask(ctx context.Context, tx *sql.Tx, kinds []string) (domain.ClosureTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM durable_task WHERE &quot;+predicate+&quot; ORDER BY id LIMIT 32&quot;, args...)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT id,business_key,payload FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.QueryRowContext</code> |
| <code>kinds</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(kinds)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ClosureTask</code> | 使用 <code>domain.ClosureTask</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>实际返回表达式：<code>domain.ClosureTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>sql.ErrNoRows</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(kinds) == 0</code>。条件成立时执行 <code>return domain.ClosureTask{}, sql.ErrNoRows</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id FROM durable_task WHERE &quot;+predicate+&quot; ORDER BY id LIMIT 32&quot;, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.ClosureTask{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.ClosureTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,business_key,payload FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...).Scan(&amp;task.ID, &amp;task.OperationID, &amp;task.Payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,business_key,payload FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.ClosureTask{}, sql.ErrNoRows</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4585:6210 -->
````go
func claimClosureTask(ctx context.Context, tx *sql.Tx, kinds []string) (domain.ClosureTask, error) {
	if len(kinds) == 0 {
		return domain.ClosureTask{}, sql.ErrNoRows
	}
	predicate := "kind IN (" + strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",") + ") AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))"
	args := make([]any, len(kinds))
	for index, kind := range kinds {
		args[index] = kind
	}
	// 排序候选不加范围锁、不搬运载荷。FOR UPDATE的filesort会锁住多个候选，
	// 即使SKIP LOCKED也令其他worker无法领取；到期登记也可能被其间隙锁阻塞。
	rows, err := tx.QueryContext(ctx, "SELECT id FROM durable_task WHERE "+predicate+" ORDER BY id LIMIT 32", args...)
	if err != nil {
		return domain.ClosureTask{}, err
	}
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return domain.ClosureTask{}, err
	}
	for _, id := range ids {
		var task domain.ClosureTask
		lockedArgs := append([]any{id}, args...)
		// 候选快照不授予执行资格；精确主键当前读重新核验种类、状态及期限。
		err = tx.QueryRowContext(ctx, "SELECT id,business_key,payload FROM durable_task WHERE id=? AND ("+predicate+") FOR UPDATE SKIP LOCKED", lockedArgs...).Scan(&task.ID, &task.OperationID, &task.Payload)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		return task, err
	}
	return domain.ClosureTask{}, sql.ErrNoRows
}

````

## 第 12 步：Closure.MarkRunning

插入位置：第 11 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func (r *Closure) MarkRunning(ctx context.Context, tx *sql.Tx, id uint64, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6210:6514 -->
````go
func (r *Closure) MarkRunning(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, id)
	return err
}

````

## 第 13 步：Closure.LockOwnership

插入位置：第 12 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`func (r *Closure) LockOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.TaskOwnership</code> | 使用 <code>domain.TaskOwnership</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.Instance, &amp;value.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6514:6816 -->
````go
func (r *Closure) LockOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error) {
	var value domain.TaskOwnership
	err := tx.QueryRowContext(ctx, "SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE", id).Scan(&value.Instance, &value.State)
	return value, err
}

````

## 第 14 步：Closure.Retry

插入位置：第 13 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func (r *Closure) Retry(ctx context.Context, id uint64, instance, code string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, code, id, instance)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, code, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, code, id, instance)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, code, id, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, code, id, instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 14 bytes=6816:7124 -->
````go
func (r *Closure) Retry(ctx context.Context, id uint64, instance, code string) {
	_, _ = r.db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state='RUNNING'", code, id, instance)
}

````

## 第 15 步：statement

插入位置：第 14 步代码之后，紧接追加；完成稿约第 132 行。

本步声明：`type statement struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>query</code> | <code>string</code> | 本次关闭会话步骤的参数化SQL。 |
| <code>args</code> | <code>&#91;&#93;any</code> | 依SQL占位符顺序排列的参数，不能拼入SQL文本。 |

<!-- write-step: 15 bytes=7124:7178 -->
````go
type statement struct {
	query string
	args  []any
}

````

## 第 16 步：execute

插入位置：第 15 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func execute(ctx context.Context, tx *sql.Tx, commands []statement) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, command.query, command.args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>commands</code> | <code>&#91;&#93;statement</code> | 按既定顺序执行的关闭会话SQL列表；任一步失败立即返回，由外层事务统一回滚/核验。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, command.query, command.args...)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7178:7474 -->
````go
func execute(ctx context.Context, tx *sql.Tx, commands []statement) error {
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, command.query, command.args...); err != nil {
			return err
		}
	}
	return nil
}

// session_expiry.go：保留原实现，按业务合并文件。

````

## 第 17 步：Closure.ExpiredSession

插入位置：第 16 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func (r *Closure) ExpiredSession(ctx context.Context) (domain.Session, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT id,CAST(user_id AS CHAR) FROM session WHERE state=&#x27;ACTIVE&#x27; AND expires_at&lt;=UTC_TIMESTAMP(6) ORDER BY expires_at,id LIMIT 1&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT id,CAST(user_id AS CHAR) FROM session WHERE state=&#x27;ACTIVE&#x27; AND expires_at&lt;=UTC_TIMESTAMP(6) ORDER BY expires_at,id LIMIT 1&quot;).Scan(&amp;value.ID, &amp;value.UserID)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT id,CAST(user_id AS CHAR) FROM session WHERE state=&#x27;ACTIVE&#x27; AND expires_at&lt;=UTC_TIMESTAMP(6) ORDER BY expires_at,id LIMIT 1&quot;)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7474:7800 -->
````go
func (r *Closure) ExpiredSession(ctx context.Context) (domain.Session, error) {
	var value domain.Session
	err := r.db.QueryRowContext(ctx, "SELECT id,CAST(user_id AS CHAR) FROM session WHERE state='ACTIVE' AND expires_at<=UTC_TIMESTAMP(6) ORDER BY expires_at,id LIMIT 1").Scan(&value.ID, &value.UserID)
	return value, err
}

````

## 第 18 步：Closure.LockExpiryUser

插入位置：第 17 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`func (r *Closure) LockExpiryUser(ctx context.Context, tx *sql.Tx, user string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;state)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, user)</code>。

<!-- write-step: 18 bytes=7800:8007 -->
````go
func (r *Closure) LockExpiryUser(ctx context.Context, tx *sql.Tx, user string) error {
	var state string
	return tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
}

````

## 第 19 步：Closure.LockExpirySession

插入位置：第 18 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func (r *Closure) LockExpirySession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.State, &amp;value.Epoch, &amp;value.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8007:8314 -->
````go
func (r *Closure) LockExpirySession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Epoch, &value.Expires)
	return value, err
}

````

## 第 20 步：Closure.PrepareSessionClose

插入位置：第 19 步代码之后，紧接追加；完成稿约第 165 行。

本步声明：`func (r *Closure) PrepareSessionClose(ctx context.Context, tx *sql.Tx, value domain.SessionClosePreparation) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,&#x27;SESSION_LOGOUT&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,&#x27;SESSION_LOGOUT&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续） |
| <code>value</code> | <code>domain.SessionClosePreparation</code> | 使用 <code>domain.SessionClosePreparation</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>在此声明中的实际使用：<code>value.OperationID</code>；<code>value.UserID</code>；<code>value.Target</code>；<code>value.Fingerprint</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,&#x27;SESSION_LOGOUT&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return execute(ctx, tx, &#91;&#93;statement{ 		{&quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,&#x27;SESSION_LOGOUT&#x27;,?,?, …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8314:10074 -->
````go
func (r *Closure) PrepareSessionClose(ctx context.Context, tx *sql.Tx, value domain.SessionClosePreparation) error {
	// 会话状态、代次和持久gate是关闭权威；准入申请、复核及后台续期都在相同
	// user→session锁下核验它们。不能在这里改写会话全部历史准入：长期会话可
	// 积累数万条记录，使短事务反复超时回滚，反而无法提交关闭点及可靠任务。
	// 保留原准入供审计，OPEN也不代表仍有效；会话关闭后原证明不能再次通过核验。
	return execute(ctx, tx, []statement{
		{"INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,'SESSION_LOGOUT',?,?,'PREPARING',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))", []any{value.OperationID, value.UserID, string(value.Target), value.Fingerprint}},
		{"UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE id=?", []any{value.SessionID}},
		{"INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1", []any{value.Scope, value.Epoch, value.OperationID}},
		{"INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,?,?,?,'FENCING','[\"document\"]',UTC_TIMESTAMP(6))", []any{value.OperationID, value.UserID, value.Scope, value.Epoch, string(value.Target)}},
		{"INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('SESSION_LOGOUT',?,?,?,'QUEUED',UTC_TIMESTAMP(6))", []any{value.OperationID, value.OperationID, string(value.Payload)}},
	})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/session_closure_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
