# 分步编写 `services/document/internal/repository/document/retained_governance_queries.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/172-retained-governance-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存留存治理计划及资源和导出排空证据。

固定资源清理范围不自动授权删除跨文档共享对象。

写之前先按这条执行路线理解各段的连接：

1. BeginRetainedGovernance固定范围
2. RetainedGovernanceDrained核原scope/ACK
3. ReleaseRetainedResourcePins移除依赖
4. ValidateRetainedResourceCompletion证明删除或真实共享保留。

本文件需要守住的边界：

- UPLOAD pin是恢复凭据，不能当共享消费
- UNKNOWN保护不是清理完成。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/retained_governance_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/retained_governance_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/retained_governance_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/retained_governance_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:491 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	allocations "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
)

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 2 步：retainedGovernanceTask

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`const retainedGovernanceTask = "RETAINED_GOVERNANCE_RECOVER"`。

<!-- write-step: 2 bytes=491:553 -->
````go
const retainedGovernanceTask = "RETAINED_GOVERNANCE_RECOVER"

````

## 第 3 步：RetainedGovernanceRecord

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type RetainedGovernanceRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Plan</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。 |
| <code>State</code> | <code>string</code> | 留存内容治理操作当前状态。 |

<!-- write-step: 3 bytes=553:646 -->
````go
type RetainedGovernanceRecord struct {
	Plan  domain.RetainedGovernancePlan
	State string
}

````

## 第 4 步：RetainedGovernanceRow

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func RetainedGovernanceRow(ctx context.Context, q Queryer, operation string, lock bool) (RetainedGovernanceRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?&quot;+suffix, retainedGovernanceTask, operation, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?&quot;+suffix, retainedGovernanceTask, operation, operation)</code>；<code>row.Plan.Purge.Operation != operation</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>RetainedGovernanceRecord</code> | 实际返回表达式：<code>row</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?&quot;+suffix, retainedGovernanceTask, operation, operation).Scan(&amp;raw, &amp;row.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?&quot;+suffix, retainedGovernanceTask, operation, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; row.Plan.Purge.Operation != operation</code>。
- 此处通过 <code>return row, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=646:1330 -->
````go
func RetainedGovernanceRow(ctx context.Context, q Queryer, operation string, lock bool) (RetainedGovernanceRecord, error) {
	var row RetainedGovernanceRecord
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?"+suffix, retainedGovernanceTask, operation, operation).Scan(&raw, &row.State)
	if err != nil {
		return row, err
	}
	row.Plan, err = domain.DecodeRetainedGovernancePlan(raw)
	if err == nil && row.Plan.Purge.Operation != operation {
		err = domain.Violation(domain.StateConflict, "RETAINED_PLAN_BINDING_CHANGED")
	}
	return row, err
}

````

## 第 5 步：RetainedDocumentResources

插入位置：第 4 步代码之后，紧接追加；完成稿约第 46 行。

发现本Doc全部资源只固定清理范围，不提供物理删除许可。最后由资源行锁
重查跨Doc引用和UNKNOWN保护，故共享资源不会随孤立文章误删。

本步声明：`func RetainedDocumentResources(ctx context.Context, q Queryer, document uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT DISTINCT resource_id FROM resource_reference WHERE document_id=? ORDER BY resource_id&quot;, document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT DISTINCT resource_id FROM resource_reference WHERE document_id=? ORDER BY resource_id&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT DISTINCT resource_id FROM resource_reference WHERE document_id=? ORDER BY resource_id&quot;, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1330:1988 -->
````go
// 发现本Doc全部资源只固定清理范围，不提供物理删除许可。最后由资源行锁
// 重查跨Doc引用和UNKNOWN保护，故共享资源不会随孤立文章误删。
func RetainedDocumentResources(ctx context.Context, q Queryer, document uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, "SELECT DISTINCT resource_id FROM resource_reference WHERE document_id=? ORDER BY resource_id", document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

````

## 第 6 步：BeginRetainedGovernance

插入位置：第 5 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`func BeginRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>RetainedDocumentResources(ctx, tx, p.Document)</code>；<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=?,control_revision=?,acl_revision=?,publication_epoch=?,event_seq=?, 	 current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,vi …</code>（长表达式，按本步完整代码继续）；<code>InsertSubmissionTask(ctx, tx, retainedGovernanceTask, p.Operation, p.Operation, raw, &quot;QUEUED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RETAINED_GOVERNANCE_FENCE&quot;, source.Operation, p.Operation, binding, &quot;SUCCEEDED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>RetainedDocumentResources(ctx, tx, p.Document)</code>；<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, retainedGovernanceTask, p.Operation, p.Operation, raw, &quot;QUEUED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;RETAINED_GOVERNANCE_FENCE&quot;, source.Operation, p.Operation, binding, &quot;SUCCEEDED&quot;)</code> |
| <code>plan</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>在此声明中的实际使用：<code>plan.Purge</code>；<code>plan.Resources</code>；<code>plan.Action</code>；<code>json.Marshal(plan)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCES_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!reflect.DeepEqual(resources, plan.Resources)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCES_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.Action == &quot;PURGE&quot;</code>。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=?,control_revision=?,acl_revision=?,publication_epoch=?,event_seq=?, 	 current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,vi …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1988:4169 -->
````go
func BeginRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error {
	p := plan.Purge
	resources, err := RetainedDocumentResources(ctx, tx, p.Document)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(resources, plan.Resources) {
		return domain.Violation(domain.StateConflict, "RETAINED_RESOURCES_CHANGED")
	}
	state := "PUBLIC_RETAINED"
	if plan.Action == "PURGE" {
		state = "CLOSING"
	}
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle=?,control_revision=?,acl_revision=?,publication_epoch=?,event_seq=?,
	 current_release_id=NULL,publication_state='UNPUBLISHED',visibility_revision=visibility_revision+1,main_locked=TRUE
	 WHERE id=? AND lifecycle='PUBLIC_RETAINED' AND responsible_user_id IS NULL AND control_revision=?`,
		state, p.ClosingControl, p.ACL, p.Publication, p.Sequence, p.Document, p.ExpectedControl)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, retainedGovernanceTask, p.Operation, p.Operation, raw, "QUEUED"); err != nil {
		return err
	}
	for _, source := range p.Scopes {
		// 关闭编号不可逆派生，额外保存精确父原号用于后续DELETE核验。
		// 两方ACK完成但父治理receipt未终结时，不允许新号抢先推进水位。
		binding, err := json.Marshal(source)
		if err != nil {
			return err
		}
		if err = InsertSubmissionTask(ctx, tx, "RETAINED_GOVERNANCE_FENCE", source.Operation, p.Operation, binding, "SUCCEEDED"); err != nil {
			return err
		}
	}
	// 通用事务会保存item_no=1的原号审计；这里补充治理前后真实版本和范围，
	// 与指针/关闭计划同一事务，失败不会留下“审计成功而文章未撤下”的分裂。
	target, _ := json.Marshal(map[string]string{"document_id": domain.TextID(p.Document)})
	before, _ := json.Marshal(map[string]any{"lifecycle": "PUBLIC_RETAINED", "release_id": domain.TextID(plan.Release), "control_revision": domain.TextID(p.ExpectedControl)})
````

## 第 7 步：BeginRetainedGovernance（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 110 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason) 	 VALUES(?,?,2,&#x27;RETAINED_PUBLIC_GOVERNANCE&#x27;,?,?,?,&#x27;平台治理孤立 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4169:4682 -->
````go
	after, _ := json.Marshal(map[string]any{"action": plan.Action, "lifecycle": state, "control_revision": domain.TextID(p.ClosingControl), "publication_epoch": domain.TextID(p.Publication)})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason)
	 VALUES(?,?,2,'RETAINED_PUBLIC_GOVERNANCE',?,?,?,'平台治理孤立公开留存；禁止代写正文')`, p.Actor, p.Operation, string(target), string(before), string(after))
	return err
}

````

## 第 8 步：ClaimRetainedGovernance

插入位置：第 7 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func ClaimRetainedGovernance(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? 	 AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;R …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RETAINED_GENERATION_EXHAUSTED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? 	 AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;R …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=? 	 AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6)) OR (state=&#x27;R …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 检查 <code>task.Generation == ^uint64(0)</code>。条件成立时执行 <code>return task, errors.New(&quot;RETAINED_GENERATION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4682:5565 -->
````go
func ClaimRetainedGovernance(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=?
	 AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))
	 ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, retainedGovernanceTask).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, errors.New("RETAINED_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, task.Generation, task.ID)
	return task, err
}

````

## 第 9 步：LockRetainedGovernanceOwnership

插入位置：第 8 步代码之后，紧接追加；完成稿约第 132 行。

本步声明：`func LockRetainedGovernanceOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE&quot;, task.ID, retainedGovernanceTask, task.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>owner != instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_WORKER_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE&quot;, task.ID, retainedGovernanceTask, t …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE&quot;, task.ID, retainedGovernanceTask, task.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>generation != task.Generation &#124;&#124; state != &quot;RUNNING&quot; &#124;&#124; owner != instance</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_WORKER_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5565:6166 -->
````go
func LockRetainedGovernanceOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	var generation uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE", task.ID, retainedGovernanceTask, task.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || owner != instance {
		return domain.Violation(domain.StateConflict, "RETAINED_WORKER_CHANGED")
	}
	return nil
}

````

## 第 10 步：RetryRetainedGovernance

插入位置：第 9 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func RetryRetainedGovernance(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;RETAINED_GOVERNANCE_PENDING&#x27; WHERE id= …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>PurgeTask</code> | 使用 <code>PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;RETAINED_GOVERNANCE_PENDING&#x27; WHERE id= …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;RETAINED_GOVERNANCE_PENDING&#x27; WHERE id= …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6166:6570 -->
````go
func RetryRetainedGovernance(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='RETAINED_GOVERNANCE_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", task.ID, task.Generation, instance)
	return err
}

````

## 第 11 步：CompleteRetainedGovernance

插入位置：第 10 步代码之后，紧接追加；完成稿约第 150 行。

本步声明：`func CompleteRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,purge_after=NULL,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)) WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND control_revisio …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 	 WHERE operation_id=? AND actor_id=? AND kind=? AND state=&#x27;PREPARING …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>在此声明中的实际使用：<code>plan.Purge</code>；<code>plan.Action</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_RECEIPT_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>plan.Action == &quot;PURGE&quot;</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,purge_after=NULL,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)) WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND control_revisio …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 	 WHERE operation_id=? AND actor_id=? AND kind=? AND state=&#x27;PREPARING …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6570:7789 -->
````go
func CompleteRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan, result []byte) error {
	p := plan.Purge
	if plan.Action == "PURGE" {
		updated, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',purge_after=NULL,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)) WHERE id=? AND lifecycle='CLOSING' AND control_revision=? AND acl_revision=? AND publication_epoch=?", p.Document, p.ClosingControl, p.ACL, p.Publication)
		if err != nil {
			return err
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
		}
	}
	updated, err := tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
	 WHERE operation_id=? AND actor_id=? AND kind=? AND state='PREPARING'`, string(result), p.Operation, p.Actor, domain.RetainedGovernanceKind(plan.Action))
	if err != nil {
		return err
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return domain.Violation(domain.StateConflict, "RETAINED_RECEIPT_CHANGED")
	}
	return nil
}

// retained_governance_cleanup.go：保留原实现，按业务合并文件。

````

## 第 12 步：RetainedGovernanceCleanupReceipt

插入位置：第 11 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func RetainedGovernanceCleanupReceipt(ctx context.Context, q Queryer, operation string) (platform.AccountDocumentCleanupContext, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_CLEANED&#x27; AND business_key=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_CLEANED&#x27; AND business_key=?&quot;, operation)</code>；<code>original != operation</code>；<code>result.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;RETAINED_CLEANUP_RECEIPT_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_CLEANED&#x27; AND business_key=?&quot;, operation).Scan(&amp;original, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_CLEANED&#x27; AND business_key=?&quot;, operation)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, false, err</code>。当前函数在这里结束。
- 检查 <code>original != operation &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; json.Unmarshal(raw, &amp;result) != nil &#124;&#124; !result.Valid() &#124;&#124; result.Operation != operation</code>。条件成立时执行 <code>return result, false, errors.New(&quot;RETAINED_CLEANUP_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7789:8576 -->
````go
func RetainedGovernanceCleanupReceipt(ctx context.Context, q Queryer, operation string) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var raw []byte
	var original, state string
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='RETAINED_GOVERNANCE_CLEANED' AND business_key=?", operation).Scan(&original, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if original != operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.Operation != operation {
		return result, false, errors.New("RETAINED_CLEANUP_RECEIPT_CHANGED")
	}
	return result, true, nil
}

````

## 第 13 步：ReleaseRetainedResourcePins

插入位置：第 12 步代码之后，紧接追加；完成稿约第 191 行。

本步声明：`func ReleaseRetainedResourcePins(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, document)</code>；<code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;RETENTION&#x27;&quot;, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>；<code>ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, document)</code>；<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code>；<code>ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, document)</code>；<code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;RETENTION&#x27;&quot;, document)</code>；<code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>；<code>ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;RETENTION&#x27;&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document)</code>。
- 此处通过 <code>return ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8576:9155 -->
````go
func ReleaseRetainedResourcePins(ctx context.Context, tx *sql.Tx, document uint64) error {
	items, err := LockDocumentResourceAllocationCleanup(ctx, tx, document)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE document_id=? AND owner_kind='RETENTION'", document); err != nil {
		return err
	}
	var space uint64
	if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&space); err != nil {
		return err
	}
	return ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)
}

````

## 第 14 步：RemoveRetainedExportControls

插入位置：第 13 步代码之后，紧接追加；完成稿约第 206 行。

本步声明：`func RemoveRetainedExportControls(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>RetainedExportArtifactsDrained(ctx, tx, document)</code>；<code>tx.ExecContext(ctx, &quot;DELETE c FROM export_control c JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>RetainedExportArtifactsDrained(ctx, tx, document)</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>RetainedExportArtifactsDrained(ctx, tx, document)</code>；<code>tx.ExecContext(ctx, &quot;DELETE c FROM export_control c JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_EXPORT_DRAIN_PENDING&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_EXPORT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE c FROM export_control c JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9155:9592 -->
````go
func RemoveRetainedExportControls(ctx context.Context, tx *sql.Tx, document uint64) error {
	drained, err := RetainedExportArtifactsDrained(ctx, tx, document)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "RETAINED_EXPORT_DRAIN_PENDING")
	}
	_, err = tx.ExecContext(ctx, "DELETE c FROM export_control c JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?", document)
	return err
}

````

## 第 15 步：ValidateRetainedResourceCompletion

插入位置：第 14 步代码之后，紧接追加；完成稿约第 218 行。

每个固定资源须已物理DELETED，或由另一文档的真实引用/确定保护继续保留。
原UPLOAD pin只是恢复凭据；它不能把未删且无业务引用的对象伪装为共享保留。

本步声明：`func ValidateRetainedResourceCompletion(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, id)</code>；<code>allocations.ResourceTransferEvidence(ctx, tx, id, plan.Purge.Document, plan.Purge.Space)</code>；<code>tx.QueryRowContext(ctx, `SELECT 		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)), 		 EXISTS(SE …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>allocations.ResourceTransferEvidence(ctx, tx, id, plan.Purge.Document, plan.Purge.Space)</code> |
| <code>plan</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>在此声明中的实际使用：<code>plan.Resources</code>；<code>plan.Purge</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_CLEANUP_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;space, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,state FROM resource WHERE id=? FOR UPDATE&quot;, id)</code>。
- 检查 <code>space != plan.Purge.Space</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!known</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;DELETED&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)), 		 EXISTS(SE …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)), 		 EXISTS(SE …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>own &#124;&#124; unknown &#124;&#124; state != &quot;READY&quot; &#124;&#124; !shared</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_RESOURCE_CLEANUP_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=9592:11518 -->
````go
// 每个固定资源须已物理DELETED，或由另一文档的真实引用/确定保护继续保留。
// 原UPLOAD pin只是恢复凭据；它不能把未删且无业务引用的对象伪装为共享保留。
func ValidateRetainedResourceCompletion(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error {
	for _, id := range plan.Resources {
		var space uint64
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT space_id,state FROM resource WHERE id=? FOR UPDATE", id).Scan(&space, &state); err != nil {
			return err
		}
		if space != plan.Purge.Space {
			known, err := allocations.ResourceTransferEvidence(ctx, tx, id, plan.Purge.Document, plan.Purge.Space)
			if err != nil {
				return err
			}
			if !known {
				return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_BINDING_CHANGED")
			}
		}
		if state == "DELETED" {
			continue
		}
		var own, shared, unknown bool
		err := tx.QueryRowContext(ctx, `SELECT
		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')),
		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id<>? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD'))
		 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id<>? AND state IN ('PROTECTED','CONFIRMED') AND JSON_CONTAINS(resource_ids,CAST(? AS JSON))),
		 EXISTS(SELECT 1 FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)))`,
			id, plan.Purge.Document, id, plan.Purge.Document, plan.Purge.Document, domain.TextID(id), domain.TextID(id)).Scan(&own, &shared, &unknown)
		if err != nil {
			return err
		}
		if own || unknown || state != "READY" || !shared {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_CLEANUP_PENDING")
		}
	}
	return nil
}

// retained_governance_drains.go：保留原实现，按业务合并文件。

````

## 第 16 步：RetainedGovernanceDrained

插入位置：第 15 步代码之后，紧接追加；完成稿约第 258 行。

不从BLOCKED/任务数量/租约推断结束。必须是原范围、原关闭任务及两端
OUTPUT_STOPPED摘要同时匹配，才能继续历史清理或已撤下对象的下一次治理。

本步声明：`func RetainedGovernanceDrained(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>retainedScopeDrained(ctx, tx, source.Key, source.Epoch, source.Operation, plan.Purge.Actor, plan.Purge.Document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>retainedScopeDrained(ctx, tx, source.Key, source.Epoch, source.Operation, plan.Purge.Actor, plan.Purge.Document)</code> |
| <code>plan</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>在此声明中的实际使用：<code>plan.Purge</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !drained</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=11518:12087 -->
````go
// 不从BLOCKED/任务数量/租约推断结束。必须是原范围、原关闭任务及两端
// OUTPUT_STOPPED摘要同时匹配，才能继续历史清理或已撤下对象的下一次治理。
func RetainedGovernanceDrained(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) (bool, error) {
	for _, source := range plan.Purge.Scopes {
		drained, err := retainedScopeDrained(ctx, tx, source.Key, source.Epoch, source.Operation, plan.Purge.Actor, plan.Purge.Document)
		if err != nil || !drained {
			return false, err
		}
	}
	return true, nil
}

````

## 第 17 步：RetainedClosedSourceDrained

插入位置：第 16 步代码之后，紧接追加；完成稿约第 270 行。

本步声明：`func RetainedClosedSourceDrained(ctx context.Context, tx *sql.Tx, key string, epoch, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.actor_id,0) 	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id 	  …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT operation_id FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_FENCE&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>；<code>RetainedGovernanceRow(ctx, tx, parent, false)</code>；<code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>RetainedGovernanceRow(ctx, tx, parent, false)</code>；<code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.actor_id,0) 	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id 	  …</code>（长表达式，按本步完整代码继续）；<code>source.Key == key</code>；<code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.actor_id,0) 	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id 	  …</code>（长表达式，按本步完整代码继续）；<code>source.Epoch == epoch</code>；<code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>original.Plan.Purge.Document != document</code>；<code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.actor_id,0) 	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id 	  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.actor_id,0) 	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id 	  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT operation_id FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_FENCE&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation).Scan(&amp;parent)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT operation_id FROM durable_task WHERE kind=&#x27;RETAINED_GOVERNANCE_FENCE&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;SUCCEEDED&quot; &#124;&#124; original.Plan.Purge.Actor != actor &#124;&#124; original.Plan.Purge.Document != document</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=12087:13530 -->
````go
func RetainedClosedSourceDrained(ctx context.Context, tx *sql.Tx, key string, epoch, document uint64) (bool, error) {
	var operation string
	var actor uint64
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0)
	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id
	 WHERE g.scope_key=? AND g.max_epoch=? AND g.state='BLOCKED'`, key, epoch).Scan(&operation, &actor)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var parent string
	if err = tx.QueryRowContext(ctx, "SELECT operation_id FROM durable_task WHERE kind='RETAINED_GOVERNANCE_FENCE' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&parent); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	original, err := RetainedGovernanceRow(ctx, tx, parent, false)
	if err != nil {
		return false, err
	}
	if original.State != "SUCCEEDED" || original.Plan.Purge.Actor != actor || original.Plan.Purge.Document != document {
		return false, nil
	}
	found := false
	for _, source := range original.Plan.Purge.Scopes {
		found = found || (source.Key == key && source.Epoch == epoch && source.Operation == operation)
	}
	if !found {
		return false, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_BINDING_CHANGED")
	}
	return retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)
}

````

## 第 18 步：retainedScopeDrained

插入位置：第 17 步代码之后，紧接追加；完成稿约第 305 行。

本步声明：`func retainedScopeDrained(ctx context.Context, tx *sql.Tx, key string, epoch uint64, operation string, actor, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission, 	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, key, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>ScopeKey: key</code>；<code>platform.SourceClosureDigest(key, epoch)</code>；<code>closure.Scope != key</code>；<code>scope != key</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Epoch: epoch</code>；<code>platform.SourceClosureDigest(key, epoch)</code>；<code>closure.Epoch != epoch</code>；<code>actualEpoch != epoch</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission, 	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c …</code>（长表达式，按本步完整代码继续）；<code>RevocationId: operation</code>；<code>domain.ValidOperationID(operation)</code>；<code>taskOperation != operation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor == 0</code>；<code>actualActor != actor</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>closure.Entity != document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_ACK_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission, 	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(operation) &#124;&#124; actor == 0 &#124;&#124; taskOperation != operation &#124;&#124; 		json.Unmarshal(raw, &amp;closure) != nil &#124;&#124; closure.Scope != key &#124;&#124; closure.Epoch != epoch &#124;&#124; closu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>taskState != &quot;SUCCEEDED&quot; &#124;&#124; authState != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, key, epoch).Scan(&amp;digest)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, key, epoch)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(request, participant))</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;RETAINED_ACK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13530:16251 -->
````go
func retainedScopeDrained(ctx context.Context, tx *sql.Tx, key string, epoch uint64, operation string, actor, document uint64) (bool, error) {
	var taskOperation, taskState, authState, scope, gateState, revocation string
	var actualEpoch, gateEpoch, actualActor uint64
	var raw, target, participants []byte
	err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,
	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
	 FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
	 WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, operation).
		Scan(&taskOperation, &taskState, &raw, &authState, &scope, &actualEpoch, &target, &gateEpoch, &gateState, &revocation, &actualActor, &participants)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var closure domain.Closure
	var binding map[string]string
	var expectedParticipants []string
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: key, Epoch: epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(key, epoch)}
	if !domain.ValidOperationID(operation) || actor == 0 || taskOperation != operation ||
		json.Unmarshal(raw, &closure) != nil || closure.Scope != key || closure.Epoch != epoch || closure.Entity != document || closure.Action != domain.RetainedGovernancePurpose ||
		json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &expectedParticipants) != nil ||
		actualActor != actor || scope != key || actualEpoch != epoch || !slices.Equal(expectedParticipants, []string{"edge", "collaboration"}) ||
		gateEpoch != epoch || gateState != "BLOCKED" || revocation != operation || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) {
		return false, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_BINDING_CHANGED")
	}
	if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
		return false, nil
	}
	for _, participant := range expectedParticipants {
		var digest []byte
		err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, key, epoch).Scan(&digest)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
			return false, domain.Violation(domain.StateConflict, "RETAINED_ACK_CHANGED")
		}
	}
	return true, nil
}

````

## 第 19 步：RetainedExportArtifactsDrained

插入位置：第 18 步代码之后，紧接追加；完成稿约第 350 行。

本步声明：`func RetainedExportArtifactsDrained(ctx context.Context, tx *sql.Tx, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id 	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size&lt;&gt;0 OR c.reserved_bytes&lt;&gt;0))`, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id 	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size&lt;&gt;0 OR c.reserved_bytes&lt;&gt;0))`, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>!pending</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id 	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size&lt; …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id 	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size&lt;&gt;0 OR c.reserved_bytes&lt;&gt;0))`, document)</code>。
- 此处通过 <code>return !pending, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=16251:16638 -->
````go
func RetainedExportArtifactsDrained(ctx context.Context, tx *sql.Tx, document uint64) (bool, error) {
	var pending bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id
	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size<>0 OR c.reserved_bytes<>0))`, document).Scan(&pending)
	return !pending, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/retained_governance_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
