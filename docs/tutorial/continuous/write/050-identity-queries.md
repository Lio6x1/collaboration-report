# 分步编写 `services/account/internal/repository/account/identity_queries.go`

先按 [A02 · 短期身份准入、重验与幂等写入](../lessons/account/02-identity-admission.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/050-identity-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。

多个表代表同一次关闭；AuthorizationResults.Read 用单SELECT读取一致快照，避免把不同提交阶段拼成成功证明。

写之前先按这条执行路线理解各段的连接：

1. Identity 按 user/session/organization/member 的既定顺序提供 FOR UPDATE 查询；admission_source 记录准入依赖的每个scope/epoch。
2. EnsureGate 只补缺行，AdvanceGate 仅由已经重验权威来源的logic引入更高代次。
3. Mutation.InsertReceipt 与 InsertAudit 使用调用方同一tx；Read 检查最多两条任务，歧义即报错。

本文件需要守住的边界：

- Queryer 可以是池或原事务；需要行锁时不得传池。
- 任务租约generation不能充当授权协议代次，孤立或多重绑定任务不能证明关闭完成。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/identity_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/identity_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/identity_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/identity_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:300 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
	"errors"
	"time"
)

// authorization_result.go：保留原实现，按业务合并文件。

````

## 第 2 步：AuthorizationResultFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。

本步声明：`type AuthorizationResultFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | authorization_change中的原操作者。 |
| <code>Epoch</code> | <code>uint64</code> | 权限关闭协议代次；与任务租约generation不同。 |
| <code>GateEpoch</code> | <code>uint64</code> | 来源门的最高代次。 |
| <code>ReceiptActor</code> | <code>uint64</code> | 原业务回执的操作者，需与授权记录一致。 |
| <code>CloseUser</code> | <code>uint64</code> | 账号注销记录中的账号（仅对应注销任务）。 |
| <code>Organization</code> | <code>uint64</code> | 组织关闭记录中的组织编号。 |
| <code>OrganizationActor</code> | <code>uint64</code> | 组织关闭记录的发起者。 |
| <code>Scope</code> | <code>string</code> | 关闭的权限来源键。 |
| <code>State</code> | <code>string</code> | authorization_change当前状态。 |
| <code>GateState</code> | <code>string</code> | access_gate当前状态。 |
| <code>GateRevocation</code> | <code>string</code> | 门记录的最后关闭操作号。 |
| <code>TaskKind</code> | <code>string</code> | 实际持久任务种类，限定为支持的业务路径。 |
| <code>TaskOperation</code> | <code>string</code> | 持久任务绑定的操作号。 |
| <code>TaskState</code> | <code>string</code> | 持久任务状态。 |
| <code>ReceiptKind</code> | <code>string</code> | 原回执业务种类。 |
| <code>ReceiptState</code> | <code>string</code> | 原操作回执所处状态，决定是返回既有结果、继续原操作还是等待核验；未知状态不能伪报成功。 |
| <code>CloseState</code> | <code>string</code> | 账号注销记录状态。 |
| <code>OrganizationState</code> | <code>string</code> | 组织关闭记录状态。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 授权记录的固定权限目标JSON。 |
| <code>Participants</code> | <code>&#91;&#93;byte</code> | 授权记录的参与者计划。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 真实业务持久任务的载荷。 |
| <code>ReceiptTarget</code> | <code>&#91;&#93;byte</code> | 原业务回执目标JSON，需与授权固定目标一致。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Capture</code> | <code>&#91;&#93;byte</code> | 注销留存计划字节。 |
| <code>OrganizationPlan</code> | <code>&#91;&#93;byte</code> | 组织关闭固定计划字节。 |
| <code>AckEpoch</code> | <code>uint64</code> | Doc参与者ACK确认的来源代次。 |
| <code>AckPhase</code> | <code>string</code> | Doc ACK阶段，完成关闭要求OUTPUT_STOPPED。 |
| <code>AckDigest</code> | <code>&#91;&#93;byte</code> | Doc对原关闭请求返回的结果摘要。 |
| <code>AckCompleted</code> | <code>bool</code> | ACK是否具有完成时间，缺失时不能承认输出已停。 |
| <code>ReceiptTerminal</code> | <code>bool</code> | 原回执是否具有终态时间；与状态和ACK共同核验。 |

<!-- write-step: 2 bytes=300:1297 -->
````go
// AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。
type AuthorizationResultFacts struct {
	Actor, Epoch, GateEpoch, ReceiptActor, CloseUser, Organization, OrganizationActor    uint64
	Scope, State, GateState, GateRevocation, TaskKind, TaskOperation, TaskState          string
	ReceiptKind, ReceiptState, CloseState, OrganizationState                             string
	Target, Participants, Payload, ReceiptTarget, Fingerprint, Capture, OrganizationPlan []byte
	AckEpoch                                                                             uint64
	AckPhase                                                                             string
	AckDigest                                                                            []byte
	AckCompleted                                                                         bool
	ReceiptTerminal                                                                      bool
}

````

## 第 3 步：AuthorizationResults

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type AuthorizationResults struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 3 bytes=1297:1345 -->
````go
type AuthorizationResults struct{ db *sql.DB }

````

## 第 4 步：NewAuthorizationResults

插入位置：第 3 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewAuthorizationResults(db *sql.DB) *AuthorizationResults`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AuthorizationResults</code> | 实际返回表达式：<code>&amp;AuthorizationResults{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AuthorizationResults{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1345:1450 -->
````go
func NewAuthorizationResults(db *sql.DB) *AuthorizationResults { return &AuthorizationResults{db: db} }

````

## 第 5 步：AuthorizationResults.Read

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func (r *AuthorizationResults) Read(ctx context.Context, operation string) (AuthorizationResultFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT COALESCE(a.actor_id,0),a.scope_key,a.epoch,a.state,a.target_permission,a.participant_plan, 	 COALESCE(g.max_epoch,0),COALESCE(g.state,&#x27;&#x27;),COALESCE(g. …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, `SELECT COALESCE(a.actor_id,0),a.scope_key,a.epoch,a.state,a.target_permission,a.participant_plan, 	 COALESCE(g.max_epoch,0),COALESCE(g.state,&#x27;&#x27;),COALESCE(g. …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AuthorizationResultFacts</code> | 使用 <code>AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>实际返回表达式：<code>AuthorizationResultFacts{}</code>；<code>f</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code>；<code>errors.New(&quot;AUTHORIZATION_TASK_BINDING_AMBIGUOUS&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, `SELECT COALESCE(a.actor_id,0),a.scope_key,a.epoch,a.state,a.target_permission,a.participant_plan, 	 COALESCE(g.max_epoch,0),COALESCE(g.state,&#x27;&#x27;),COALESCE(g. …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return AuthorizationResultFacts{}, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>!rows.Next()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return f, err</code>。当前函数在这里结束。
- 此处通过 <code>return f, sql.ErrNoRows</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>rows.Scan(&amp;f.Actor, &amp;f.Scope, &amp;f.Epoch, &amp;f.State, &amp;f.Target, &amp;f.Participants, 		&amp;f.GateEpoch, &amp;f.GateState, &amp;f.GateRevocation, &amp;f.TaskKind, &amp;f.TaskOperation, &amp;f.TaskState, &amp;f.Paylo …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return f, err</code>。当前函数在这里结束。
- 检查 <code>rows.Next()</code>。条件成立时执行 <code>return f, errors.New(&quot;AUTHORIZATION_TASK_BINDING_AMBIGUOUS&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return f, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1450:4118 -->
````go
func (r *AuthorizationResults) Read(ctx context.Context, operation string) (AuthorizationResultFacts, error) {
	// 单条SELECT在同一InnoDB语句快照中读全部事实，不能拼接并发提交前后的状态/ACK。
	// 限定真实业务任务种类，不接管历史底座或仅写入授权表的未知记录。
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(a.actor_id,0),a.scope_key,a.epoch,a.state,a.target_permission,a.participant_plan,
	 COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,''),
	 COALESCE(t.kind,''),COALESCE(t.operation_id,''),COALESCE(t.state,''),t.payload,
	 COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),o.target_json,o.request_fingerprint,COALESCE(o.terminal_at IS NOT NULL,FALSE),
	 COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
	 COALESCE(z.organization_id,0),COALESCE(z.actor_id,0),COALESCE(z.state,''),z.fixed_plan,
	 COALESCE(k.epoch,0),COALESCE(k.phase,''),k.result_digest,COALESCE(k.completed_at IS NOT NULL,FALSE)
	 FROM authorization_change a
	 LEFT JOIN access_gate g ON g.scope_key=a.scope_key
	 LEFT JOIN durable_task t ON t.business_key=a.revocation_id AND t.kind IN
	 ('SESSION_LOGOUT','CREDENTIAL_CHANGE','ORGANIZATION_CHANGE','PLATFORM_GOVERNANCE_CHANGE','ACCOUNT_CLOSE_FENCE','ORG_CLOSE_RECOVER')
	 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
	 LEFT JOIN account_close c ON c.operation_id=t.operation_id AND t.kind='ACCOUNT_CLOSE_FENCE'
	 LEFT JOIN organization_transition z ON z.operation_id=t.operation_id AND z.kind='CLOSE' AND t.kind='ORG_CLOSE_RECOVER'
	 LEFT JOIN authorization_ack k ON k.revocation_id=a.revocation_id AND k.participant='document' AND k.scope_key=a.scope_key
	 WHERE a.revocation_id=? LIMIT 2`, operation)
	if err != nil {
		return AuthorizationResultFacts{}, err
	}
	defer rows.Close()
	var f AuthorizationResultFacts
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return f, err
		}
		return f, sql.ErrNoRows
	}
	err = rows.Scan(&f.Actor, &f.Scope, &f.Epoch, &f.State, &f.Target, &f.Participants,
		&f.GateEpoch, &f.GateState, &f.GateRevocation, &f.TaskKind, &f.TaskOperation, &f.TaskState, &f.Payload,
		&f.ReceiptActor, &f.ReceiptKind, &f.ReceiptState, &f.ReceiptTarget, &f.Fingerprint, &f.ReceiptTerminal,
		&f.CloseUser, &f.CloseState, &f.Capture, &f.Organization, &f.OrganizationActor, &f.OrganizationState, &f.OrganizationPlan,
		&f.AckEpoch, &f.AckPhase, &f.AckDigest, &f.AckCompleted)
	if err != nil {
		return f, err
	}
	if rows.Next() {
		return f, errors.New("AUTHORIZATION_TASK_BINDING_AMBIGUOUS")
	}
	return f, rows.Err()
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 6 步：Queryer

插入位置：第 5 步代码之后，紧接追加；完成稿约第 77 行。

Queryer使只读查询可以明确使用原池或调用方事务；repository不会自行开启事务。

本步声明：`type Queryer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>QueryRowContext</code> | <code>(context.Context, string, ...any) *sql.Row</code> | 依次接收ctx、带占位符的SQL和占位符参数；返回单行句柄，实际查询/无行错误在Scan时处理。接口让连接池或原事务都可注入，但涉及锁时必须沿用原事务。 |

<!-- write-step: 6 bytes=4118:4313 -->
````go
// Queryer使只读查询可以明确使用原池或调用方事务；repository不会自行开启事务。
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

````

## 第 7 步：Identity

插入位置：第 6 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`type Identity struct{}`。

<!-- write-step: 7 bytes=4313:4337 -->
````go
type Identity struct{}

````

## 第 8 步：NewIdentity

插入位置：第 7 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func NewIdentity() *Identity`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Identity</code> | 实际返回表达式：<code>&amp;Identity{}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Identity{}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4337:4390 -->
````go
func NewIdentity() *Identity { return &Identity{} }

````

## 第 9 步：Identity.SessionActor

插入位置：第 8 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func (r *Identity) SessionActor(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session)</code> |
| <code>actor</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session).Scan(actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session).Scan(actor)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session).Scan(actor)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session).Scan(actor)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT user_id FROM session WHERE id=?&quot;, session)</code>。

<!-- write-step: 9 bytes=4390:4592 -->
````go
func (r *Identity) SessionActor(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT user_id FROM session WHERE id=?", session).Scan(actor)
}

````

## 第 10 步：Identity.SessionExpiry

插入位置：第 9 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func (r *Identity) SessionExpiry(ctx context.Context, tx *sql.Tx, session []byte, expires *time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session)</code> |
| <code>expires</code> | <code>*time.Time</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session).Scan(expires)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session).Scan(expires)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session).Scan(expires)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session).Scan(expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT expires_at FROM session WHERE id=?&quot;, session)</code>。

<!-- write-step: 10 bytes=4592:4805 -->
````go
func (r *Identity) SessionExpiry(ctx context.Context, tx *sql.Tx, session []byte, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT expires_at FROM session WHERE id=?", session).Scan(expires)
}

````

## 第 11 步：Identity.CreateAdmission

插入位置：第 10 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`func (r *Identity) CreateAdmission(ctx context.Context, tx *sql.Tx, id string, actor uint64, session []byte, operation, purpose, binding string, expires time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OPEN&#x27;,?)&quot;, id, actor, se …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4805:5228 -->
````go
func (r *Identity) CreateAdmission(ctx context.Context, tx *sql.Tx, id string, actor uint64, session []byte, operation, purpose, binding string, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,'OPEN',?)", id, actor, session, operation, purpose, binding, expires)
	return err
}

````

## 第 12 步：Identity.AttachSource

插入位置：第 11 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`func (r *Identity) AttachSource(ctx context.Context, tx *sql.Tx, admission, scope string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, admission, scope, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, admission, scope, epoch)</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, admission, scope, epoch)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, admission, scope, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)&quot;, admission, scope, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5228:5487 -->
````go
func (r *Identity) AttachSource(ctx context.Context, tx *sql.Tx, admission, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)", admission, scope, epoch)
	return err
}

````

## 第 13 步：Identity.Admission

插入位置：第 12 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func (r *Identity) Admission(ctx context.Context, q Queryer, id string, raw *[]byte, state *string, expires *time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id)</code> |
| <code>raw</code> | <code>*&#91;&#93;byte</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code> |
| <code>expires</code> | <code>*time.Time</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id).Scan(raw, state, expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?&quot;, id)</code>。

<!-- write-step: 13 bytes=5487:5763 -->
````go
func (r *Identity) Admission(ctx context.Context, q Queryer, id string, raw *[]byte, state *string, expires *time.Time) error {
	return q.QueryRowContext(ctx, "SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?", id).Scan(raw, state, expires)
}

````

## 第 14 步：Identity.LockAdmission

插入位置：第 13 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`func (r *Identity) LockAdmission(ctx context.Context, tx *sql.Tx, id string, state *string, expires *time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id).Scan(state, expires)</code> |
| <code>expires</code> | <code>*time.Time</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id).Scan(state, expires)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id).Scan(state, expires)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id).Scan(state, expires)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id).Scan(state, expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE&quot;, id)</code>。

<!-- write-step: 14 bytes=5763:6026 -->
````go
func (r *Identity) LockAdmission(ctx context.Context, tx *sql.Tx, id string, state *string, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE", id).Scan(state, expires)
}

````

## 第 15 步：Identity.LockUser

插入位置：第 14 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func (r *Identity) LockUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string, credential, identity *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code> |
| <code>credential</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code> |
| <code>identity</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, credential, identity)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code>。

<!-- write-step: 15 bytes=6026:6310 -->
````go
func (r *Identity) LockUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string, credential, identity *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE", actor).Scan(state, credential, identity)
}

````

## 第 16 步：Identity.LockSession

插入位置：第 15 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func (r *Identity) LockSession(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64, state *string, credential, epoch *uint64, expires *time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session)</code> |
| <code>actor</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |
| <code>credential</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |
| <code>epoch</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |
| <code>expires</code> | <code>*time.Time</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session).Scan(actor, state, credential, epoch, expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE&quot;, session)</code>。

<!-- write-step: 16 bytes=6310:6659 -->
````go
func (r *Identity) LockSession(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64, state *string, credential, epoch *uint64, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE", session).Scan(actor, state, credential, epoch, expires)
}

````

## 第 17 步：Identity.LockTargetIdentity

插入位置：第 16 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func (r *Identity) LockTargetIdentity(ctx context.Context, tx *sql.Tx, actor uint64, state *string, identity *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, identity)</code> |
| <code>identity</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, identity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, identity)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, identity)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state, identity)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE&quot;, actor)</code>。

<!-- write-step: 17 bytes=6659:6912 -->
````go
func (r *Identity) LockTargetIdentity(ctx context.Context, tx *sql.Tx, actor uint64, state *string, identity *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE", actor).Scan(state, identity)
}

````

## 第 18 步：Identity.TargetDisplayName

插入位置：第 17 步代码之后，紧接追加；完成稿约第 124 行。

仅在目标身份已通过同一事务重验后读取；不复制账号库，也不把邮箱带出服务边界。

本步声明：`func (r *Identity) TargetDisplayName(ctx context.Context, tx *sql.Tx, user uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT display_name FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT display_name FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>name</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT display_name FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, user).Scan(&amp;name)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT display_name FROM user WHERE id=? AND state=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, user)</code>。
- 此处通过 <code>return name, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6912:7293 -->
````go
// 仅在目标身份已通过同一事务重验后读取；不复制账号库，也不把邮箱带出服务边界。
func (r *Identity) TargetDisplayName(ctx context.Context, tx *sql.Tx, user uint64) (string, error) {
	var name string
	err := tx.QueryRowContext(ctx, "SELECT display_name FROM user WHERE id=? AND state='ACTIVE' FOR UPDATE", user).Scan(&name)
	return name, err
}

````

## 第 19 步：Identity.LockOrganization

插入位置：第 18 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func (r *Identity) LockOrganization(ctx context.Context, tx *sql.Tx, organization uint64, owner *uint64, state *string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code> |
| <code>owner</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(owner, state)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(owner, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(owner, state)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(owner, state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(owner, state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code>。

<!-- write-step: 19 bytes=7293:7558 -->
````go
func (r *Identity) LockOrganization(ctx context.Context, tx *sql.Tx, organization uint64, owner *uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE", organization).Scan(owner, state)
}

````

## 第 20 步：Identity.LockMember

插入位置：第 19 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func (r *Identity) LockMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, role, state *string, epoch *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>role</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code> |
| <code>epoch</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(role, state, epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code>。

<!-- write-step: 20 bytes=7558:7875 -->
````go
func (r *Identity) LockMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, role, state *string, epoch *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, actor).Scan(role, state, epoch)
}

````

## 第 21 步：Identity.LockCreationGrant

插入位置：第 20 步代码之后，紧接追加；完成稿约第 139 行。

本步声明：`func (r *Identity) LockCreationGrant(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, revision *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor).Scan(state, revision)</code> |
| <code>revision</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor).Scan(state, revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor).Scan(state, revision)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor).Scan(state, revision)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor).Scan(state, revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability=&#x27;CREATE_KNOWLEDGE_BASE&#x27; FOR UPDATE&quot;, organization, actor)</code>。

<!-- write-step: 21 bytes=7875:8238 -->
````go
func (r *Identity) LockCreationGrant(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, revision *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability='CREATE_KNOWLEDGE_BASE' FOR UPDATE", organization, actor).Scan(state, revision)
}

````

## 第 22 步：Identity.LockTargetUser

插入位置：第 21 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`func (r *Identity) LockTargetUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor).Scan(state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM user WHERE id=? FOR UPDATE&quot;, actor)</code>。

<!-- write-step: 22 bytes=8238:8444 -->
````go
func (r *Identity) LockTargetUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", actor).Scan(state)
}

````

## 第 23 步：Identity.LockTargetMember

插入位置：第 22 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func (r *Identity) LockTargetMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, epoch *uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(state, epoch)</code> |
| <code>epoch</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(state, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(state, epoch)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(state, epoch)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor).Scan(state, epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, actor)</code>。

<!-- write-step: 23 bytes=8444:8816 -->
````go
func (r *Identity) LockTargetMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, epoch *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, actor).Scan(state, epoch)
}

// identity_gate.go：保留原实现，按业务合并文件。

````

## 第 24 步：Identity.EnsureGate

插入位置：第 23 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func (r *Identity) EnsureGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;)&quot;, scope, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;)&quot;, scope, epoch)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;)&quot;, scope, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;)&quot;, scope, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=8816:9055 -->
````go
func (r *Identity) EnsureGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN')", scope, epoch)
	return err
}

````

## 第 25 步：Identity.LockGate

插入位置：第 24 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`func (r *Identity) LockGate(ctx context.Context, tx *sql.Tx, scope string, epoch *uint64, state *string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code> |
| <code>epoch</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(epoch, state)</code> |
| <code>state</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(epoch, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(epoch, state)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(epoch, state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(epoch, state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code>。

<!-- write-step: 25 bytes=9055:9301 -->
````go
func (r *Identity) LockGate(ctx context.Context, tx *sql.Tx, scope string, epoch *uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", scope).Scan(epoch, state)
}

````

## 第 26 步：Identity.AdvanceGate

插入位置：第 25 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func (r *Identity) AdvanceGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?&quot;, epoch, scope)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?&quot;, epoch, scope)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?&quot;, epoch, scope)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?&quot;, epoch, scope)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=9301:9635 -->
````go
func (r *Identity) AdvanceGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='OPEN',last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?", epoch, scope)
	return err
}

// mutation.go：保留原实现，按业务合并文件。

````

## 第 27 步：Mutation

插入位置：第 26 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`type Mutation struct{}`。

<!-- write-step: 27 bytes=9635:9659 -->
````go
type Mutation struct{}

````

## 第 28 步：NewMutation

插入位置：第 27 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func NewMutation() *Mutation`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Mutation</code> | 实际返回表达式：<code>&amp;Mutation{}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Mutation{}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=9659:9712 -->
````go
func NewMutation() *Mutation { return &Mutation{} }

````

## 第 29 步：Mutation.LockUser

插入位置：第 28 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func (r *Mutation) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 使用 <code>domain.Credential</code> 类型；其定义处的职责：按调用目的承载密码或状态代次快照。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;value.State, &amp;value.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE&quot;, user)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=9712:10016 -->
````go
func (r *Mutation) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE", user).Scan(&value.State, &value.Epoch)
	return value, err
}

````

## 第 30 步：Mutation.LockSession

插入位置：第 29 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func (r *Mutation) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.State, &amp;value.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=10016:10298 -->
````go
func (r *Mutation) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Expires)
	return value, err
}

````

## 第 31 步：Mutation.Receipt

插入位置：第 30 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`func (r *Mutation) Receipt(ctx context.Context, tx *sql.Tx, id string) (domain.MutationReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.MutationReceipt</code> | 使用 <code>domain.MutationReceipt</code> 类型；其定义处的职责：让重复请求返回原事实而非再次执行body。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id).Scan(&amp;value.ActorID, &amp;value …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=10298:10689 -->
````go
func (r *Mutation) Receipt(ctx context.Context, tx *sql.Tx, id string) (domain.MutationReceipt, error) {
	var value domain.MutationReceipt
	err := tx.QueryRowContext(ctx, "SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE", id).Scan(&value.ActorID, &value.Kind, &value.Fingerprint, &value.Result)
	return value, err
}

````

## 第 32 步：Mutation.InsertReceipt

插入位置：第 31 步代码之后，紧接追加；完成稿约第 191 行。

本步声明：`func (r *Mutation) InsertReceipt(ctx context.Context, tx *sql.Tx, input domain.Mutation, state string, result, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,DATE_AD …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>input</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>input.OperationID</code>；<code>input.Session</code>；<code>input.Kind</code>；<code>input.Fingerprint</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,DATE_AD …</code>（长表达式，按本步完整代码继续） |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,DATE_AD …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=10689:11225 -->
````go
func (r *Mutation) InsertReceipt(ctx context.Context, tx *sql.Tx, input domain.Mutation, state string, result, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))", input.OperationID, input.Session.UserID, input.Kind, string(target), input.Fingerprint, state, string(result), state)
	return err
}

````

## 第 33 步：Mutation.InsertAudit

插入位置：第 32 步代码之后，紧接追加；完成稿约第 196 行。

本步声明：`func (r *Mutation) InsertAudit(ctx context.Context, tx *sql.Tx, input domain.Mutation, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(?,?,1,?,?)&quot;, input.Session.UserID, input.OperationID, input.Kind, string(target))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>input</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>input.Session</code>；<code>input.OperationID</code>；<code>input.Kind</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(?,?,1,?,?)&quot;, input.Session.UserID, input.OperationID, input.Kind, string(target))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=11225:11542 -->
````go
func (r *Mutation) InsertAudit(ctx context.Context, tx *sql.Tx, input domain.Mutation, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(?,?,1,?,?)", input.Session.UserID, input.OperationID, input.Kind, string(target))
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/identity_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/02-identity-admission.md)。
