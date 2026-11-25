# 分步编写 `services/document/internal/repository/document/main_controls_queries.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/157-main-controls-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。

主线控制跨SQL与协作服务，原始意图和可变化的恢复进度必须分开保存。

写之前先按这条执行路线理解各段的连接：

1. RegisterMainControl固定original/recovery
2. AdvanceMainControlAttempt推进捕获尝试
3. InsertControlledManifest追加版本
4. CompleteMainControl同时更新业务指针及安装任务。

本文件需要守住的边界：

- 任务执行者和generation必须匹配
- 不能超时后以新操作号再次修改主线。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/main_controls_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/main_controls_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/main_controls_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/main_controls_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:312 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// main_control_readiness.go：保留原实现，按业务合并文件。

````

## 第 2 步：PendingMainControlBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type PendingMainControlBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | 持久恢复任务种类，区分计算、安装与核验阶段。 |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>ReceiptKind</code> | <code>string</code> | 原回执业务种类，需与任务种类形成合法组合。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 原回执结果JSON，安装恢复据此核对已提交内容。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |

<!-- write-step: 2 bytes=312:520 -->
````go
type PendingMainControlBinding struct {
	Kind, Operation, ReceiptKind, State            string
	Payload, Original, Fingerprint, Target, Result []byte
	Actor                                          uint64
}

````

## 第 3 步：PendingMainControlBindings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func PendingMainControlBindings(ctx context.Context, db *sql.DB) ([]PendingMainControlBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.kind,t.operation_id,t.payload,original.payload, 		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json 		FROM durable_task t 	 …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingMainControlBinding</code> | 实际返回表达式：<code>nil</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.kind,t.operation_id,t.payload,original.payload, 		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json 		FROM durable_task t 	 …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;fact.Kind, &amp;fact.Operation, &amp;fact.Payload, &amp;fact.Original, &amp;fact.Actor, &amp;fact.ReceiptKind, &amp;fact.State, &amp;fact.Fingerprint, &amp;fact.Target, &amp;fact.Result)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return facts, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=520:1657 -->
````go
func PendingMainControlBindings(ctx context.Context, db *sql.DB) ([]PendingMainControlBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.kind,t.operation_id,t.payload,original.payload,
		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json
		FROM durable_task t
		LEFT JOIN durable_task original ON original.kind='MAIN_CONTROL_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		WHERE t.kind IN ('MAIN_CONTROL_RECOVER','MAIN_CONTROL_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingMainControlBinding
	for rows.Next() {
		var fact PendingMainControlBinding
		if err = rows.Scan(&fact.Kind, &fact.Operation, &fact.Payload, &fact.Original, &fact.Actor, &fact.ReceiptKind, &fact.State, &fact.Fingerprint, &fact.Target, &fact.Result); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

// main_controls.go：保留原实现，按业务合并文件。

````

## 第 4 步：MainControlReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`type MainControlReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者账号。 |
| <code>Kind</code> | <code>string</code> | 回执业务种类，避免同号跨业务复用。 |
| <code>State</code> | <code>string</code> | 原操作回执状态，决定返回结果或等待恢复。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 已经保存的结果JSON，不重新执行以制造新结果。 |
| <code>Generation</code> | <code>uint64</code> | 回执代次，恢复提交时防止旧工作覆盖新进度。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 4 bytes=1657:1836 -->
````go
type MainControlReceipt struct {
	Actor               uint64
	Kind, State         string
	Fingerprint, Result []byte
	Generation          uint64
	Expires             time.Time
}

````

## 第 5 步：MainControlResult

插入位置：第 4 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func MainControlResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;MAIN_LOCK&#x27;,&#x27;STYLE …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;MAIN_LOCK&#x27;,&#x27;STYLE …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;MAIN_LOCK&#x27;,&#x27;STYLE …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN (&#x27;MAIN_LOCK&#x27;,&#x27;STYLE …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1836:2345 -->
````go
func MainControlResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE')", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

````

## 第 6 步：RegisterMainControl

插入位置：第 5 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func RegisterMainControl(ctx context.Context, tx *sql.Tx, document, generation uint64, operation string, original, recovery []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECOVER&quot;, operation, operation, recovery, &quot;QUEUED&quot;)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;PREPARING&#x27;,operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECOVER&quot;, operation, operation, recovery, &quot;QUEUED&quot;)</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;PREPARING&#x27;,operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;PREPARING&#x27;,operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECOVER&quot;, operation, operation, recovery, &quot;QUEUED&quot;)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;PREPARING&#x27;,operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>original</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_CONTEXT&quot;, operation, operation, original, &quot;SUCCEEDED&quot;)</code> |
| <code>recovery</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECOVER&quot;, operation, operation, recovery, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;PREPARING&#x27;,operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 6 bytes=2345:3131 -->
````go
func RegisterMainControl(ctx context.Context, tx *sql.Tx, document, generation uint64, operation string, original, recovery []byte) error {
	// 原绑定不可变，领取进度可变化；两者和Doc屏障均属于调用方的同一预约事务。
	if err := InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_CONTEXT", operation, operation, original, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECOVER", operation, operation, recovery, "QUEUED"); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='PREPARING',operation_id=?,generation=?,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WHERE document_id=?", operation, generation, document)
	return err
}

````

## 第 7 步：ClaimMainControlTask

插入位置：第 6 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func ClaimMainControlTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;MAIN_CONTROL_RECOVER&quot;, &quot;MAIN_CONTROL_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3131:3339 -->
````go
func ClaimMainControlTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_RECONCILE")
}

````

## 第 8 步：AdvanceMainControlAttempt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func AdvanceMainControlAttempt(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE id=? AND generation=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, string(payload), task.ID, task.Generation, task.Instance)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE id=? AND generation=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, string(payload), task.ID, task.Generation, task.Instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3339:3769 -->
````go
func AdvanceMainControlAttempt(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, payload []byte) error {
	// 只有明确CHANGED才能替换领取进度；原MAIN_CONTROL_CONTEXT与已固定输入始终保留。
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", string(payload), task.ID, task.Generation, task.Instance)
	return err
}

````

## 第 9 步：InsertControlledManifest

插入位置：第 8 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`func InsertControlledManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, kind, cut string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>parent</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |
| <code>cut</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document, parent, structure, style, operation, kind, cut)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3769:4154 -->
````go
func InsertControlledManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, kind, cut string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?,?,?)", document, parent, structure, style, operation, kind, cut)
}

````

## 第 10 步：InsertControlledStyle

插入位置：第 9 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func InsertControlledStyle(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string, style []byte) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> |
| <code>parent</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> |
| <code>style</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(style)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,&#x27;style-v1&#x27;,?)&quot;, document, parent, string(style), operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4154:4488 -->
````go
func InsertControlledStyle(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string, style []byte) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO style_revision(document_id,parent_id,style_json,schema_version,source_operation_id) VALUES(?,?,?,'style-v1',?)", document, parent, string(style), operation)
}

````

## 第 11 步：ConfigureMainOperationAdmission

插入位置：第 10 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func ConfigureMainOperationAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte, expires int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, operation, string(target), expires, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, operation, string(target), expires, admission)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, operation, string(target), expires, admission)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |
| <code>expires</code> | <code>int64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, operation, string(target), expires, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, operation, string(target), expires, admission)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4488:4966 -->
````go
func ConfigureMainOperationAdmission(ctx context.Context, tx *sql.Tx, admission, operation string, target []byte, expires int64) error {
	// 全篇控制没有虚构的unit=0实体：SQL外键列保持NULL，完整protobuf目标仍明确unit/branch为0。
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=NULL,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?", operation, string(target), expires, admission)
	return err
}

````

## 第 12 步：CompleteMainControl

插入位置：第 11 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`func CompleteMainControl(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation, manifest, style uint64, locked bool, result map[string]any, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest, document, task.Operation, generation)</code>；<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, raw)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_COMMIT_FIXED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, raw)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_COMMIT_FIXED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest, document, task.Operation, generation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest, document, task.Operation, generation)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest, document, task.Operation, generation)</code> |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code> |
| <code>locked</code> | <code>bool</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code> |
| <code>result</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>json.Marshal(result)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_COMMIT_FIXED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_CONTROL_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, locked, style, manifest, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest, document, task.Operation, generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4966:6162 -->
````go
func CompleteMainControl(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation, manifest, style uint64, locked bool, result map[string]any, install []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document SET main_locked=?,current_style_id=?,current_manifest_id=?,control_revision=control_revision+1 WHERE id=?", locked, style, manifest, document); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='INSTALLING',target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", manifest, document, task.Operation, generation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", raw); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MAIN_COMMIT_FIXED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/main_controls_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
