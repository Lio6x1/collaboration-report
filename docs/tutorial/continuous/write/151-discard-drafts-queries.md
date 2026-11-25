# 分步编写 `services/document/internal/repository/document/discard_drafts_queries.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/151-discard-drafts-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

登记丢弃草稿控制、派生lineage、安装任务与审核归档动作。

数据库提交候选不等于Node已安装，归档意见必须等真实安装ACK。

写之前先按这条执行路线理解各段的连接：

1. RegisterDiscardDraft占支线
2. InsertDiscardLineage追加来源
3. CompleteDiscardDraft保存结果和install
4. ActivateDiscardedBranch切可用
5. ArchiveDiscardedReview用稳定item_no写审核动作。

本文件需要守住的边界：

- item_no=0留给ARCHIVE通知
- 不得在安装前归档审核或释放支线。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/discard_drafts_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/discard_drafts_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/discard_drafts_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/discard_drafts_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:305 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// discard_draft_readiness.go：保留原实现，按业务合并文件。

````

## 第 2 步：PendingDiscardDraftBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type PendingDiscardDraftBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>Kind</code> | <code>string</code> | 持久恢复任务种类，区分计算、安装与核验阶段。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Owner</code> | <code>string</code> | 支线控制门中持有控制权的操作号；不是worker实例名。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Committed</code> | <code>&#91;&#93;byte</code> | SQL已提交阶段保存的安装载荷，用于后续Node安装核验。 |

<!-- write-step: 2 bytes=305:523 -->
````go
type PendingDiscardDraftBinding struct {
	Operation, Kind, State, Owner                     string
	Actor                                             uint64
	Original, Payload, Target, Fingerprint, Committed []byte
}

````

## 第 3 步：PendingDiscardDraftBindings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func PendingDiscardDraftBindings(ctx context.Context, q Queryer) ([]PendingDiscardDraftBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>pendingDiscardDraftBindings(ctx, q, &quot;&quot;)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>pendingDiscardDraftBindings(ctx, q, &quot;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingDiscardDraftBinding</code> | 由 <code>pendingDiscardDraftBindings(ctx, q, &quot;&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pendingDiscardDraftBindings(ctx, q, &quot;&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return pendingDiscardDraftBindings(ctx, q, &quot;&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=523:679 -->
````go
func PendingDiscardDraftBindings(ctx context.Context, q Queryer) ([]PendingDiscardDraftBinding, error) {
	return pendingDiscardDraftBindings(ctx, q, "")
}

````

## 第 4 步：ClaimedDiscardDraftBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func ClaimedDiscardDraftBinding(ctx context.Context, q Queryer, operation string) (PendingDiscardDraftBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>pendingDiscardDraftBindings(ctx, q, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>pendingDiscardDraftBindings(ctx, q, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>pendingDiscardDraftBindings(ctx, q, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PendingDiscardDraftBinding</code> | 实际返回表达式：<code>PendingDiscardDraftBinding{}</code>；<code>facts&#91;0&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return PendingDiscardDraftBinding{}, err</code>。当前函数在这里结束。
- 检查 <code>len(facts) != 1</code>。条件成立时执行 <code>return PendingDiscardDraftBinding{}, sql.ErrNoRows</code>。当前函数在这里结束。
- 此处通过 <code>return facts&#91;0&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=679:1027 -->
````go
func ClaimedDiscardDraftBinding(ctx context.Context, q Queryer, operation string) (PendingDiscardDraftBinding, error) {
	facts, err := pendingDiscardDraftBindings(ctx, q, operation)
	if err != nil {
		return PendingDiscardDraftBinding{}, err
	}
	if len(facts) != 1 {
		return PendingDiscardDraftBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

````

## 第 5 步：pendingDiscardDraftBindings

插入位置：第 4 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func pendingDiscardDraftBindings(ctx context.Context, q Queryer, operation string) ([]PendingDiscardDraftBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.target_json,o.request_fingerprint,  COALESCE(JSON_UNQUO …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.target_json,o.request_fingerprint,  COALESCE(JSON_UNQUO …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingDiscardDraftBinding</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.target_json,o.request_fingerprint,  COALESCE(JSON_UNQUO …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.Operation, &amp;item.Kind, &amp;item.State, &amp;item.Actor, &amp;item.Original, &amp;item.Payload, &amp;item.Target, &amp;item.Fingerprint, &amp;item.Owner, &amp;item.Committed)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1027:2687 -->
````go
func pendingDiscardDraftBindings(ctx context.Context, q Queryer, operation string) ([]PendingDiscardDraftBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.target_json,o.request_fingerprint,
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}')
 FROM durable_task t
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='DISCARD_DRAFT'
 LEFT JOIN durable_task original ON original.kind='DISCARD_DRAFT_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Intent.Branch'))
 LEFT JOIN durable_task committed ON committed.kind='DISCARD_DRAFT_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id
 WHERE t.kind IN ('DISCARD_DRAFT_RECOVER','DISCARD_DRAFT_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED')
 AND (?='' OR t.operation_id=?) ORDER BY t.id`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PendingDiscardDraftBinding{}
	for rows.Next() {
		var item PendingDiscardDraftBinding
		if err = rows.Scan(&item.Operation, &item.Kind, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Target, &item.Fingerprint, &item.Owner, &item.Committed); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// discard_drafts.go：保留原实现，按业务合并文件。

````

## 第 6 步：DiscardDraftResult

插入位置：第 5 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func DiscardDraftResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;DISCARD_DRAFT&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;DISCARD_DRAFT&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 使用 <code>MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;DISCARD_DRAFT&#x27;&quot;, oper …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;DISCARD_DRAFT&#x27;&quot;, operation)</code>。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2687:3145 -->
````go
func DiscardDraftResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='DISCARD_DRAFT'", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

````

## 第 7 步：RegisterDiscardDraft

插入位置：第 6 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func RegisterDiscardDraft(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3145:3582 -->
````go
func RegisterDiscardDraft(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_RECOVER", operation, operation, raw, "QUEUED")
}

````

## 第 8 步：ClaimDiscardDraftTask

插入位置：第 7 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func ClaimDiscardDraftTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;DISCARD_DRAFT_RECOVER&quot;, &quot;DISCARD_DRAFT_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3582:3793 -->
````go
func ClaimDiscardDraftTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_RECONCILE")
}

````

## 第 9 步：InsertDiscardLineage

插入位置：第 8 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func InsertDiscardLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>previous</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>capture</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>provenance</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(provenance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3793:4355 -->
````go
func InsertDiscardLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'DISCARDED',?,?,?,?,?,'provenance-v1')", id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

````

## 第 10 步：CompleteDiscardDraft

插入位置：第 9 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func CompleteDiscardDraft(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;DISCARD_DRAFT_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4355:4941 -->
````go
func CompleteDiscardDraft(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, state, result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

````

## 第 11 步：DiscardDraftInstalled

插入位置：第 10 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func DiscardDraftInstalled(ctx context.Context, q Queryer, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;DISCARD_DRAFT_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;DISCARD_DRAFT_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;DISCARD_DRAFT_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;DISCARD_DRAFT_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4941:5249 -->
````go
func DiscardDraftInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='DISCARD_DRAFT_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

````

## 第 12 步：ActivateDiscardedBranch

插入位置：第 11 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func ActivateDiscardedBranch(ctx context.Context, tx *sql.Tx, branch, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_WORKSPACE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_WORKSPACE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5249:5715 -->
````go
func ActivateDiscardedBranch(ctx context.Context, tx *sql.Tx, branch, epoch uint64) error {
	result, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_WORKSPACE_UNKNOWN")
	}
	return nil
}

````

## 第 13 步：ArchiveDiscardedReview

插入位置：第 12 步代码之后，紧接追加；完成稿约第 121 行。

安装ACK之后才写这些动作。item_no=0留给ARCHIVE通知，逐项处理使用明确且稳定的序号。

本步声明：`func ArchiveDiscardedReview(ctx context.Context, tx *sql.Tx, operation string, actor uint64, pointer domain.ReviewPointer, changes []domain.DiscardChange) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ReviewRoundVersion(ctx, tx, pointer.ID, pointer.Round)</code>；<code>LockedChange(ctx, tx, pointer.ID, change.ID)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,? …</code>（长表达式，按本步完整代码继续）；<code>ResolveChange(ctx, tx, change.ID, uint64(id))</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ReviewRoundVersion(ctx, tx, pointer.ID, pointer.Round)</code>；<code>LockedChange(ctx, tx, pointer.ID, change.ID)</code>；<code>tx.ExecContext</code>；<code>ResolveChange(ctx, tx, change.ID, uint64(id))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,? …</code>（长表达式，按本步完整代码继续）；<code>InsertReviewAction(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer.ID, pointer.Round, actor, version, 0, &amp;reason)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) VALUES(?,?,2,&#x27;ARCHIVE&#x27;,?,&#x27;{\&quot;archived\&quot;:false}&#x27;,&#x27;{\&quot;archi …</code>（长表达式，按本步完整代码继续）；<code>QueueReviewActionNotification(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer, action, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,? …</code>（长表达式，按本步完整代码继续）；<code>InsertReviewAction(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer.ID, pointer.Round, actor, version, 0, &amp;reason)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) VALUES(?,?,2,&#x27;ARCHIVE&#x27;,?,&#x27;{\&quot;archived\&quot;:false}&#x27;,&#x27;{\&quot;archi …</code>（长表达式，按本步完整代码继续）；<code>QueueReviewActionNotification(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer, action, actor)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Round</code>；<code>pointer.Unit</code>；<code>pointer.Document</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>changes</code> | <code>&#91;&#93;domain.DiscardChange</code> | 使用 <code>domain.DiscardChange</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ARCHIVE_FACT_CHANGED&quot;)</code>；<code>QueueReviewActionNotification(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer, action, actor)</code> |

**跟着本段代码走一遍：**

- 检查 <code>pointer.ID == 0</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fact.Status != &quot;OPEN&quot; &#124;&#124; fact.Revision != change.Revision</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ARCHIVE_FACT_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,? …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET archived_at=UTC_TIMESTAMP(6),revision=revision+1 WHERE id=? AND status=&#x27;WITHDRAWN&#x27; AND archived_at IS NULL&quot;, pointer.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND resumable_request_id=?&quot;, pointer.Unit, pointer.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) VALUES(?,?,2,&#x27;ARCHIVE&#x27;,?,&#x27;{\&quot;archived\&quot;:false}&#x27;,&#x27;{\&quot;archi …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return QueueReviewActionNotification(ctx, tx, operation, &quot;ARCHIVE&quot;, pointer, action, actor)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5715:8191 -->
````go
// 安装ACK之后才写这些动作。item_no=0留给ARCHIVE通知，逐项处理使用明确且稳定的序号。
func ArchiveDiscardedReview(ctx context.Context, tx *sql.Tx, operation string, actor uint64, pointer domain.ReviewPointer, changes []domain.DiscardChange) error {
	if pointer.ID == 0 {
		return nil
	}
	version, err := ReviewRoundVersion(ctx, tx, pointer.ID, pointer.Round)
	if err != nil {
		return err
	}
	for index, change := range changes {
		fact, err := LockedChange(ctx, tx, pointer.ID, change.ID)
		if err != nil {
			return err
		}
		if fact.Status != "OPEN" || fact.Revision != change.Revision {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,'RESOLVE_CHANGES',?,?,?,?,?)", pointer.ID, pointer.Round, actor, operation, index+1, change.Reason, version, change.ID)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if err = ResolveChange(ctx, tx, change.ID, uint64(id)); err != nil {
			return err
		}
	}
	reason := "管理员明确丢弃共同草稿，可靠恢复已安装，撤回工作结束"
	action, err := InsertReviewAction(ctx, tx, operation, "ARCHIVE", pointer.ID, pointer.Round, actor, version, 0, &reason)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE review_request SET archived_at=UTC_TIMESTAMP(6),revision=revision+1 WHERE id=? AND status='WITHDRAWN' AND archived_at IS NULL", pointer.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND resumable_request_id=?", pointer.Unit, pointer.ID); err != nil {
		return err
	}
	target, _ := json.Marshal(map[string]string{"document_id": domain.TextID(pointer.Document), "unit_id": domain.TextID(pointer.Unit), "request_id": domain.TextID(pointer.ID), "round_id": domain.TextID(pointer.Round)})
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) VALUES(?,?,2,'ARCHIVE',?,'{\"archived\":false}','{\"archived\":true}',?)", actor, operation, string(target), reason); err != nil {
		return err
	}
	return QueueReviewActionNotification(ctx, tx, operation, "ARCHIVE", pointer, action, actor)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/discard_drafts_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
