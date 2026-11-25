# 分步编写 `services/document/internal/repository/document/conflict_resolutions_queries.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/149-conflict-resolutions-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存人工冲突修复的独立支线控制、固定候选、新lineage与安装状态。

修复不等于送审，也不能借送审外键实现互斥；事务同时登记原上下文与控制占位。

写之前先按这条执行路线理解各段的连接：

1. RegisterResolution占支线
2. ClaimResolutionTask领取
3. InsertResolvedLineage写新基线
4. CompleteResolution保存安装
5. CompleteResolutionInstallation核代次后收尾。

本文件需要守住的边界：

- 同名版本存在不够，ResolutionLineage还要核真实分支和来源
- 未知安装不得释放控制。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/conflict_resolutions_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/conflict_resolutions_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/conflict_resolutions_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/conflict_resolutions_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:294 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// conflict_resolution_readiness.go：保留原实现，按业务合并文件。

````

## 第 2 步：PendingResolutionBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type PendingResolutionBinding struct {`。


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

<!-- write-step: 2 bytes=294:510 -->
````go
type PendingResolutionBinding struct {
	Operation, Kind, State, Owner                     string
	Actor                                             uint64
	Original, Payload, Target, Fingerprint, Committed []byte
}

````

## 第 3 步：PendingResolutionBindings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func PendingResolutionBindings(ctx context.Context, q Queryer, operation string) ([]PendingResolutionBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),  COALESCE(original.payload,&#x27;{}&#x27;),t.payload,COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),  COALESCE(original.payload,&#x27;{}&#x27;),t.payload,COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingResolutionBinding</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),  COALESCE(original.payload,&#x27;{}&#x27;),t.payload,COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.Operation, &amp;item.Kind, &amp;item.State, &amp;item.Actor, &amp;item.Original, &amp;item.Payload, &amp;item.Target, &amp;item.Fingerprint, &amp;item.Owner, &amp;item.Committed)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=510:2191 -->
````go
func PendingResolutionBindings(ctx context.Context, q Queryer, operation string) ([]PendingResolutionBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,''),COALESCE(r.actor_id,0),
 COALESCE(original.payload,'{}'),t.payload,COALESCE(r.target_json,'{}'),COALESCE(r.request_fingerprint,''),
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}')
 FROM durable_task t LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='CONFLICT_RESOLUTION'
 LEFT JOIN durable_task original ON original.kind='RESOLUTION_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id AND original.state='SUCCEEDED'
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Scope.Workspace.Workspace'))
 LEFT JOIN durable_task committed ON committed.kind='RESOLUTION_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id AND committed.state='SUCCEEDED'
 WHERE t.kind IN ('RESOLUTION_RECOVER','RESOLUTION_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (?='' OR t.operation_id=?)`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []PendingResolutionBinding{}
	for rows.Next() {
		var item PendingResolutionBinding
		if err = rows.Scan(&item.Operation, &item.Kind, &item.State, &item.Actor, &item.Original, &item.Payload, &item.Target, &item.Fingerprint, &item.Owner, &item.Committed); err != nil {
			return nil, err
		}
		values = append(values, item)
	}
	return values, rows.Err()
}

````

## 第 4 步：ClaimedResolutionBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func ClaimedResolutionBinding(ctx context.Context, q Queryer, operation string) (PendingResolutionBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>PendingResolutionBindings(ctx, q, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>PendingResolutionBindings(ctx, q, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>PendingResolutionBindings(ctx, q, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PendingResolutionBinding</code> | 实际返回表达式：<code>PendingResolutionBinding{}</code>；<code>facts&#91;0&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return PendingResolutionBinding{}, err</code>。当前函数在这里结束。
- 检查 <code>len(facts) != 1</code>。条件成立时执行 <code>return PendingResolutionBinding{}, sql.ErrNoRows</code>。当前函数在这里结束。
- 此处通过 <code>return facts&#91;0&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2191:2602 -->
````go
func ClaimedResolutionBinding(ctx context.Context, q Queryer, operation string) (PendingResolutionBinding, error) {
	facts, err := PendingResolutionBindings(ctx, q, operation)
	if err != nil {
		return PendingResolutionBinding{}, err
	}
	if len(facts) != 1 {
		return PendingResolutionBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

````

## 第 5 步：ResolutionResult

插入位置：第 4 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func ResolutionResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;CONFLICT_RESOLUTION&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;CONFLICT_RESOLUTION&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 使用 <code>MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;CONFLICT_RESOLUTION&#x27;&quot; …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;CONFLICT_RESOLUTION&#x27;&quot;, operation)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2602:3073 -->
````go
func ResolutionResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var value MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='CONFLICT_RESOLUTION'", operation).Scan(&value.Actor, &value.Kind, &value.State, &value.Fingerprint, &value.Result, &value.Generation, &value.Expires)
	return value, err
}

````

## 第 6 步：RegisterResolution

插入位置：第 5 步代码之后，紧接追加；完成稿约第 61 行。

修复是独立支线动作，不写有真实SUBMISSION外键的submission_operation_id。
原上下文、恢复任务及支线占位全部由调用方同一TX保存。

本步声明：`func RegisterResolution(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_CONTEXT&quot;, operation, operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECOVER&quot;, operation, operation, raw, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3073:3677 -->
````go
// 修复是独立支线动作，不写有真实SUBMISSION外键的submission_operation_id。
// 原上下文、恢复任务及支线占位全部由调用方同一TX保存。
func RegisterResolution(ctx context.Context, tx *sql.Tx, branch uint64, operation string, raw []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_CONTEXT", operation, operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "RESOLUTION_RECOVER", operation, operation, raw, "QUEUED")
}

````

## 第 7 步：ClaimResolutionTask

插入位置：第 6 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func ClaimResolutionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;RESOLUTION_RECOVER&quot;, &quot;RESOLUTION_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3677:3880 -->
````go
func ClaimResolutionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "RESOLUTION_RECOVER", "RESOLUTION_RECONCILE")
}

````

## 第 8 步：InsertResolvedLineage

插入位置：第 7 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func InsertResolvedLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>previous</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>capture</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>provenance</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(provenance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3880:4443 -->
````go
func InsertResolvedLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema)
 VALUES(?,?,?,?,?,'RESTORED',?,?,?,?,?,'provenance-v1')`, id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

````

## 第 9 步：CompleteResolution

插入位置：第 8 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func CompleteResolution(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, state, result)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4443:5021 -->
````go
func CompleteResolution(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, state, result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

````

## 第 10 步：ResolutionInstalled

插入位置：第 9 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func ResolutionInstalled(ctx context.Context, q Queryer, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;RESOLUTION_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;RESOLUTION_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;RESOLUTION_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;RESOLUTION_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5021:5324 -->
````go
func ResolutionInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='RESOLUTION_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

````

## 第 11 步：ResolvedLineage

插入位置：第 10 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`type ResolvedLineage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Branch</code> | <code>uint64</code> | 解决冲突后推进的支线。 |
| <code>Document</code> | <code>uint64</code> | 支线所属文档。 |
| <code>Unit</code> | <code>uint64</code> | 支线所属协作单元。 |
| <code>Previous</code> | <code>uint64</code> | 上一条支线谱系记录，形成可追溯链。 |
| <code>Manifest</code> | <code>uint64</code> | 新谱系绑定的基线清单。 |
| <code>Version</code> | <code>uint64</code> | 新谱系绑定的基线正文版本。 |
| <code>Operation</code> | <code>string</code> | 产生本谱系的冲突解决操作号。 |
| <code>Capture</code> | <code>string</code> | Node固定候选采样的引用。 |
| <code>Kind</code> | <code>string</code> | 谱系事件种类。 |
| <code>Provenance</code> | <code>&#91;&#93;byte</code> | 具体来源映射JSON，用于追溯内容从哪里来。 |

<!-- write-step: 11 bytes=5324:5537 -->
````go
type ResolvedLineage struct {
	Branch, Document, Unit, Previous, Manifest, Version uint64
	Operation, Capture, Kind                            string
	Provenance                                          []byte
}

````

## 第 12 步：ResolutionLineage

插入位置：第 11 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`func ResolutionLineage(ctx context.Context, q Queryer, lineage uint64) (ResolvedLineage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lineage WHERE id=?&quot;, lineage)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lineage WHERE id=?&quot;, lineage)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ResolvedLineage</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lin …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lineage WHERE id=?&quot;, lineage)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5537:6065 -->
````go
func ResolutionLineage(ctx context.Context, q Queryer, lineage uint64) (ResolvedLineage, error) {
	var value ResolvedLineage
	err := q.QueryRowContext(ctx, "SELECT branch_id,document_id,unit_id,previous_id,base_manifest_id,base_version_id,source_operation_id,capture_ref,event_kind,provenance_json FROM branch_lineage WHERE id=?", lineage).Scan(&value.Branch, &value.Document, &value.Unit, &value.Previous, &value.Manifest, &value.Version, &value.Operation, &value.Capture, &value.Kind, &value.Provenance)
	return value, err
}

````

## 第 13 步：CompleteResolutionInstallation

插入位置：第 12 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func CompleteResolutionInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_INSTALLED&quot;, task.Operation, task.Operation, &#91;&#93;byte(`{}`), &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_INSTALLED&quot;, task.Operation, task.Operation, &#91;&#93;byte(`{}`), &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code> |
| <code>succeeded</code> | <code>bool</code> | true表示已成功安装对应工作区，将同代次RESTORING支线激活；false不激活。随后清除控制门并登记安装阶段完成，不能用本参数代替Node结果核验。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_WORKSPACE_UNKNOWN&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>succeeded</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_WORKSPACE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6065:6869 -->
````go
func CompleteResolutionInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error {
	if succeeded {
		result, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_WORKSPACE_UNKNOWN")
		}
	}
	if err := SetBranchControl(ctx, tx, branch, ""); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "RESOLUTION_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/conflict_resolutions_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
