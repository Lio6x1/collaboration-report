# 分步编写 `services/document/internal/repository/document/purge_queries.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/168-purge-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久永久删除计划、关闭证据、清理阶段与系统到期任务。

任务数量、BLOCKED或TTL不能证明已停止输入输出。

写之前先按这条执行路线理解各段的连接：

1. RegisterPurge登记
2. PurgeDrained核原task与两端摘要
3. SetPurgeCleaning进入清理
4. CompletePurgeDocument/Receipt落终态
5. ClaimPurge核generation。

本文件需要守住的边界：

- 自动清理actor=0仅代表系统原号
- 缺原回执不能伪造成功
- 父KB不能只凭CLOSED枚举放行。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/purge_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/purge_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/purge_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/purge_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:399 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

// purge.go：保留原实现，按业务合并文件。

````

## 第 2 步：PurgeRecord

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type PurgeRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。 |
| <code>State</code> | <code>string</code> | 文档清理记录状态；物理清理未确认时不能提前标成功。 |

<!-- write-step: 2 bytes=399:466 -->
````go
type PurgeRecord struct {
	Plan  domain.PurgePlan
	State string
}

````

## 第 3 步：PurgeRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func PurgeRow(ctx context.Context, q Queryer, operation string, lock bool) (PurgeRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT kind,state,COALESCE(actor_id,0),expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=?&quot;+suffix, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT kind,state,COALESCE(actor_id,0),expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=?&quot;+suffix, operation)</code>；<code>record.Plan.Operation != operation</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgeRecord</code> | 实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_BINDING_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT kind,state,COALESCE(actor_id,0),expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=?&quot;+suffix, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, err</code>。当前函数在这里结束。
- 检查 <code>kind != &quot;PURGE&quot; &#124;&#124; record.Plan.Operation != operation &#124;&#124; record.Plan.Actor != actor &#124;&#124; record.Plan.ExpectedControl != revision</code>。条件成立时执行 <code>return record, domain.Violation(domain.StateConflict, &quot;PURGE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return record, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=466:1293 -->
````go
func PurgeRow(ctx context.Context, q Queryer, operation string, lock bool) (PurgeRecord, error) {
	var record PurgeRecord
	var raw []byte
	var kind string
	var actor, revision uint64
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT kind,state,COALESCE(actor_id,0),expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=?"+suffix, operation).
		Scan(&kind, &record.State, &actor, &revision, &raw)
	if err != nil {
		return record, err
	}
	record.Plan, err = domain.DecodePurgePlan(raw)
	if err != nil {
		return record, err
	}
	if kind != "PURGE" || record.Plan.Operation != operation || record.Plan.Actor != actor || record.Plan.ExpectedControl != revision {
		return record, domain.Violation(domain.StateConflict, "PURGE_BINDING_CHANGED")
	}
	return record, nil
}

````

## 第 4 步：LifecycleKind

插入位置：第 3 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func LifecycleKind(ctx context.Context, q Queryer, operation string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT kind FROM lifecycle_operation WHERE operation_id=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT kind FROM lifecycle_operation WHERE operation_id=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>kind</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT kind FROM lifecycle_operation WHERE operation_id=?&quot;, operation).Scan(&amp;kind)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT kind FROM lifecycle_operation WHERE operation_id=?&quot;, operation)</code>。
- 此处通过 <code>return kind, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1293:1533 -->
````go
func LifecycleKind(ctx context.Context, q Queryer, operation string) (string, error) {
	var kind string
	err := q.QueryRowContext(ctx, "SELECT kind FROM lifecycle_operation WHERE operation_id=?", operation).Scan(&kind)
	return kind, err
}

````

## 第 5 步：RegisterPurge

插入位置：第 4 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func RegisterPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) VALUES(?,?,&#x27;PURGE&#x27;,&#x27;PREPARING&#x27;,?,?)&quot;, plan.Operation, actor, plan.ExpectedControl, string(raw))</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PURGE_RECOVER&quot;, plan.Operation, plan.Operation, raw, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PURGE_RECOVER&quot;, plan.Operation, plan.Operation, raw, &quot;QUEUED&quot;)</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>json.Marshal(plan)</code>；<code>plan.Actor</code>；<code>plan.Operation</code>；<code>plan.ExpectedControl</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PURGE_RECOVER&quot;, plan.Operation, plan.Operation, raw, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.Actor != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) VALUES(?,?,&#x27;PURGE&#x27;,&#x27;PREPARING&#x27;,?,?)&quot;, plan.Operation, actor, plan.ExpectedControl, string(raw))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;PURGE_RECOVER&quot;, plan.Operation, plan.Operation, raw, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1533:2102 -->
````go
func RegisterPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	var actor any
	if plan.Actor != 0 {
		actor = plan.Actor
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) VALUES(?,?,'PURGE','PREPARING',?,?)", plan.Operation, actor, plan.ExpectedControl, string(raw)); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "PURGE_RECOVER", plan.Operation, plan.Operation, raw, "QUEUED")
}

````

## 第 6 步：BeginPurgeDocument

插入位置：第 5 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func BeginPurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=?,acl_revision=?, 	 publication_epoch=?,event_seq=?,visibility_policy=&#x27;COLLABORATORS&#x27;,visibility_revis …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>plan.ClosingControl</code>；<code>plan.ACL</code>；<code>plan.Publication</code>；<code>plan.Sequence</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,control_revision=?,acl_revision=?, 	 publication_epoch=?,event_seq=?,visibility_policy=&#x27;COLLABORATORS&#x27;,visibility_revis …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2102:2850 -->
````go
func BeginPurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',control_revision=?,acl_revision=?,
	 publication_epoch=?,event_seq=?,visibility_policy='COLLABORATORS',visibility_revision=visibility_revision+1,
	 current_release_id=NULL,publication_state='UNPUBLISHED' WHERE id=? AND lifecycle='TRASHED' AND control_revision=?`,
		plan.ClosingControl, plan.ACL, plan.Publication, plan.Sequence, plan.Document, plan.ExpectedControl)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	return nil
}

````

## 第 7 步：SetPurgeCleaning

插入位置：第 6 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func SetPurgeCleaning(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27; WHERE operation_id=? AND kind=&#x27;PURGE&#x27; AND state=&#x27;PREPARING&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27; WHERE operation_id=? AND kind=&#x27;PURGE&#x27; AND state=&#x27;PREPARING&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27; WHERE operation_id=? AND kind=&#x27;PURGE&#x27; AND state=&#x27;PREPARING&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2850:3099 -->
````go
func SetPurgeCleaning(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING' WHERE operation_id=? AND kind='PURGE' AND state='PREPARING'", operation)
	return err
}

````

## 第 8 步：CompletePurgeDocument

插入位置：第 7 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func CompletePurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,purge_after=NULL,main_locked=TRUE WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND acl_revision=? AND publication_epoch=? AND cont …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1,last_error=NULL WHERE operation_id=? AND kind=&#x27;PURGE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, plan.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>plan.Document</code>；<code>plan.ACL</code>；<code>plan.Publication</code>；<code>plan.ClosingControl</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_ORIGINAL_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,purge_after=NULL,main_locked=TRUE WHERE id=? AND lifecycle=&#x27;CLOSING&#x27; AND acl_revision=? AND publication_epoch=? AND cont …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1,last_error=NULL WHERE operation_id=? AND kind=&#x27;PURGE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, plan.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_ORIGINAL_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3099:4085 -->
````go
func CompletePurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	result, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',purge_after=NULL,main_locked=TRUE WHERE id=? AND lifecycle='CLOSING' AND acl_revision=? AND publication_epoch=? AND control_revision=?", plan.Document, plan.ACL, plan.Publication, plan.ClosingControl)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	result, err = tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1,last_error=NULL WHERE operation_id=? AND kind='PURGE' AND state='CLEANING'", plan.Operation)
	if err != nil {
		return err
	}
	changed, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_ORIGINAL_CHANGED")
	}
	return nil
}

````

## 第 9 步：CompletePurgeReceipt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

原用户/系统回执必须存在并属于这个原计划；缺失回执不能被伪造成清理成功。

本步声明：`func CompletePurgeReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 	 WHERE operation_id=? AND actor_id=? AND kind=&#x27;DOCUMENT_PURGE&#x27; AND s …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>plan.Operation</code>；<code>plan.Actor</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_ORIGINAL_RECEIPT_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) 	 WHERE operation_id=? AND actor_id=? AND kind=&#x27;DOCUMENT_PURGE&#x27; AND s …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_ORIGINAL_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4085:4850 -->
````go
// 原用户/系统回执必须存在并属于这个原计划；缺失回执不能被伪造成清理成功。
func CompletePurgeReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, result []byte) error {
	updated, err := tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
	 WHERE operation_id=? AND actor_id=? AND kind='DOCUMENT_PURGE' AND state='PREPARING'`, string(result), plan.Operation, plan.Actor)
	if err != nil {
		return err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_ORIGINAL_RECEIPT_CHANGED")
	}
	return nil
}

// purge_drains.go：保留原实现，按业务合并文件。

````

## 第 10 步：PurgeDrained

插入位置：第 9 步代码之后，紧接追加；完成稿约第 137 行。

原任务、来源gate、业务绑定及两参与者摘要缺一不可；不按COUNT/TTL猜关闭成功。

本步声明：`func PurgeDrained(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission, 	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operation, participant, scope.Key, scope.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>plan.Scopes</code>；<code>plan.Document</code>；<code>plan.Actor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_DRAIN_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_ACK_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission, 	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;closure) != nil &#124;&#124; json.Unmarshal(target, &amp;binding) != nil &#124;&#124; json.Unmarshal(participants, &amp;expectedParticipants) != nil &#124;&#124; 			operation != scope.Operation &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;PURGE_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>taskState != &quot;SUCCEEDED&quot; &#124;&#124; authState != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operatio …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operation, participant, scope.Key, scope.Epoch)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(request, participant))</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;PURGE_ACK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4850:7758 -->
````go
// 原任务、来源gate、业务绑定及两参与者摘要缺一不可；不按COUNT/TTL猜关闭成功。
func PurgeDrained(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) (bool, error) {
	for _, scope := range plan.Scopes {
		var operation, taskState, authState, key, gateState, revocation string
		var epoch, gateEpoch, actor uint64
		var raw, target, participants []byte
		err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,
	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
	 FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
	 WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, scope.Operation).
			Scan(&operation, &taskState, &raw, &authState, &key, &epoch, &target, &gateEpoch, &gateState, &revocation, &actor, &participants)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var closure domain.Closure
		var binding map[string]string
		var expectedParticipants []string
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		if json.Unmarshal(raw, &closure) != nil || json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &expectedParticipants) != nil ||
			operation != scope.Operation || closure.Scope != scope.Key || closure.Epoch != scope.Epoch || closure.Entity != plan.Document || closure.Action != "PURGE_DOCUMENT" ||
			actor != plan.Actor || key != scope.Key || epoch != scope.Epoch || !slices.Equal(expectedParticipants, []string{"edge", "collaboration"}) ||
			gateEpoch != scope.Epoch || gateState != "BLOCKED" || revocation != scope.Operation || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) {
			return false, domain.Violation(domain.StateConflict, "PURGE_DRAIN_BINDING_CHANGED")
		}
		if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
			return false, nil
		}
		for _, participant := range []string{"edge", "collaboration"} {
			var digest []byte
			err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", scope.Operation, participant, scope.Key, scope.Epoch).Scan(&digest)
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
				return false, domain.Violation(domain.StateConflict, "PURGE_ACK_CHANGED")
			}
		}
	}
	return true, nil
}

// purge_expiry.go：保留原实现，按业务合并文件。

````

## 第 11 步：RegisterPurgeExpiryReceipt

插入位置：第 10 步代码之后，紧接追加；完成稿约第 186 行。

自动到期是本服务系统动作，不借用用户/Account注销身份。actor_id=0只表示
系统原号；lifecycle.actor_id仍NULL，audit与固定plan明确automatic=true。

本步声明：`func RegisterPurgeExpiryReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, fingerprint [32]byte, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)  VALUES(?,0,&#x27;DOCUMENT_PURGE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,&#x27;DOCUMENT_PURGE&#x27;,?,?)&quot;, plan.Operation, string(target), string(result))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>plan.Document</code>；<code>plan.Operation</code> |
| <code>fingerprint</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)  VALUES(?,0,&#x27;DOCUMENT_PURGE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续） |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)  VALUES(?,0,&#x27;DOCUMENT_PURGE&#x27;,?,?,&#x27;PREPAR …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,&#x27;DOCUMENT_PURGE&#x27;,?,?)&quot;, plan.Operation, string(target), string(result))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 11 bytes=7758:8865 -->
````go
// 自动到期是本服务系统动作，不借用用户/Account注销身份。actor_id=0只表示
// 系统原号；lifecycle.actor_id仍NULL，audit与固定plan明确automatic=true。
func RegisterPurgeExpiryReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, fingerprint [32]byte, result []byte) error {
	target, err := json.Marshal(map[string]any{"path": "/v1/documents/" + domain.TextID(plan.Document) + "/purge", "automatic": true})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)
 VALUES(?,0,'DOCUMENT_PURGE',?,?,'PREPARING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, plan.Operation, string(target), fingerprint[:], string(result))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,'DOCUMENT_PURGE',?,?)", plan.Operation, string(target), string(result))
	return err
}

// purge_parent.go：保留原实现，按业务合并文件。

````

## 第 12 步：ClosedPurgeParentDrained

插入位置：第 11 步代码之后，紧接追加；完成稿约第 204 行。

CLOSED不是ACK。调用方已持原kb gate，按其last_revocation定位真实关闭
任务和两端摘要；不对父KB重开、不关闭同库其它文档，也不按TTL推定成功。

本步声明：`func ClosedPurgeParentDrained(ctx context.Context, tx *sql.Tx, kb, epoch uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT g.last_revocation_id,t.operation_id,t.state,t.payload,c.state,c.epoch,c.target_permission,c.participant_plan  FROM access_gate g JOIN authorization_ …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(kb)</code>；<code>closure.Entity != kb</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT g.last_revocation_id,t.operation_id,t.state,t.payload,c.state,c.epoch,c.target_permission,c.participant_plan  FROM access_gate g JOIN authorization_ …</code>（长表达式，按本步完整代码继续）；<code>Epoch: epoch</code>；<code>platform.SourceClosureDigest(scope, epoch)</code>；<code>actualEpoch != epoch</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_DRAIN_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_ACK_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT g.last_revocation_id,t.operation_id,t.state,t.payload,c.state,c.epoch,c.target_permission,c.participant_plan  FROM access_gate g JOIN authorization_ …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(operation) &#124;&#124; taskOperation != operation &#124;&#124; actualEpoch != epoch &#124;&#124; 		json.Unmarshal(payload, &amp;closure) != nil &#124;&#124; closure.Scope != scope &#124;&#124; closure.Epoch ! …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>taskState != &quot;SUCCEEDED&quot; &#124;&#124; authState != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch).Scan(&amp;digest)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(request, participant))</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_ACK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=8865:11626 -->
````go
// CLOSED不是ACK。调用方已持原kb gate，按其last_revocation定位真实关闭
// 任务和两端摘要；不对父KB重开、不关闭同库其它文档，也不按TTL推定成功。
func ClosedPurgeParentDrained(ctx context.Context, tx *sql.Tx, kb, epoch uint64) (bool, error) {
	scope := "kb-acl:" + domain.TextID(kb)
	var operation, taskOperation, taskState, authState string
	var actualEpoch uint64
	var payload, bindingRaw, participantsRaw []byte
	err := tx.QueryRowContext(ctx, `SELECT g.last_revocation_id,t.operation_id,t.state,t.payload,c.state,c.epoch,c.target_permission,c.participant_plan
 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key
 JOIN durable_task t ON t.kind='AUTHORIZATION_CHANGE' AND t.business_key=c.revocation_id
 WHERE g.scope_key=? AND g.max_epoch=? AND g.state='BLOCKED' FOR UPDATE`, scope, epoch).
		Scan(&operation, &taskOperation, &taskState, &payload, &authState, &actualEpoch, &bindingRaw, &participantsRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var closure domain.Closure
	var binding map[string]string
	var participants []string
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope, epoch)}
	if !domain.ValidOperationID(operation) || taskOperation != operation || actualEpoch != epoch ||
		json.Unmarshal(payload, &closure) != nil || closure.Scope != scope || closure.Epoch != epoch || closure.Entity != kb || closure.Action != "CLOSE_KB" ||
		json.Unmarshal(bindingRaw, &binding) != nil || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) ||
		json.Unmarshal(participantsRaw, &participants) != nil || !slices.Equal(participants, []string{"edge", "collaboration"}) {
		return false, domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_BINDING_CHANGED")
	}
	if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
		return false, nil
	}
	for _, participant := range participants {
		var digest []byte
		err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, scope, epoch).Scan(&digest)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
			return false, domain.Violation(domain.StateConflict, "PURGE_PARENT_ACK_CHANGED")
		}
	}
	return true, nil
}

// purge_tasks.go：保留原实现，按业务合并文件。

````

## 第 13 步：PurgeTask

插入位置：第 12 步代码之后，紧接追加；完成稿约第 253 行。

本步声明：`type PurgeTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 清理持久任务的主键。 |
| <code>Generation</code> | <code>uint64</code> | worker认领的任务代次，写回时需比较以防过期执行者提交。 |
| <code>Operation</code> | <code>string</code> | 该任务绑定的原清理操作号。 |

<!-- write-step: 13 bytes=11626:11699 -->
````go
type PurgeTask struct {
	ID, Generation uint64
	Operation      string
}

````

## 第 14 步：ClaimPurge

插入位置：第 13 步代码之后，紧接追加；完成稿约第 258 行。

小批公平领取：行锁不能让一个过期RUNNING任务占住整个队列；执行租约只用于
worker ownership，绝不替代原ACK、Node CLEANED或正文UNKNOWN的终态证明。

本步声明：`func ClaimPurge(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task 	 WHERE kind=&#x27;PURGE_RECOVER&#x27; AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6) …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgeTask</code> | 实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_GENERATION_EXHAUSTED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task 	 WHERE kind=&#x27;PURGE_RECOVER&#x27; AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6) …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task 	 WHERE kind=&#x27;PURGE_RECOVER&#x27; AND ((state IN (&#x27;QUEUED&#x27;,&#x27;RETRY&#x27;,&#x27;UNKNOWN&#x27;) AND retry_at&lt;=UTC_TIMESTAMP(6) …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 检查 <code>task.Generation == ^uint64(0)</code>。条件成立时执行 <code>return task, domain.Violation(domain.StateConflict, &quot;PURGE_GENERATION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, task.Generation, task.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11699:12781 -->
````go
// 小批公平领取：行锁不能让一个过期RUNNING任务占住整个队列；执行租约只用于
// worker ownership，绝不替代原ACK、Node CLEANED或正文UNKNOWN的终态证明。
func ClaimPurge(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task
	 WHERE kind='PURGE_RECOVER' AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6))
	 OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6))) ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, domain.Violation(domain.StateConflict, "PURGE_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, task.Generation, task.ID)
	return task, err
}

````

## 第 15 步：LockPurgeOwnership

插入位置：第 14 步代码之后，紧接追加；完成稿约第 276 行。

本步声明：`func LockPurgeOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=&#x27;PURGE_RECOVER&#x27; AND operation_id=? FOR UPDATE&quot;, task.ID, task.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>PurgeTask</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Operation</code>；<code>task.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>owner != instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_WORKER_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=&#x27;PURGE_RECOVER&#x27; AND operation_id=? FOR UPDATE&quot;, task.ID, task.Operation).Scan(&amp;generation, &amp;state, &amp;owner)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? AND kind=&#x27;PURGE_RECOVER&#x27; AND operation_id=? FOR UPDATE&quot;, task.ID, task.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>generation != task.Generation &#124;&#124; state != &quot;RUNNING&quot; &#124;&#124; owner != instance</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_WORKER_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=12781:13356 -->
````go
func LockPurgeOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	var generation uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind='PURGE_RECOVER' AND operation_id=? FOR UPDATE", task.ID, task.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || owner != instance {
		return domain.Violation(domain.StateConflict, "PURGE_WORKER_CHANGED")
	}
	return nil
}

````

## 第 16 步：FinishPurgeTask

插入位置：第 15 步代码之后，紧接追加；完成稿约第 289 行。

本步声明：`func FinishPurgeTask(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=? AND generation=? AND state=&#x27;RUNNING&#x27;&quot;, task.ID, instance, task.Generation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>PurgeTask</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=? AND generation=? AND state=&#x27;RUNNING&#x27;&quot;, task.ID, instance, task.Generation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_WORKER_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=? AND generation=? AND state=&#x27;RUNNING&#x27;&quot;, task.ID, instance, task.Generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_WORKER_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=13356:13877 -->
````go
func FinishPurgeTask(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	result, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=? AND generation=? AND state='RUNNING'", task.ID, instance, task.Generation)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_WORKER_CHANGED")
	}
	return nil
}

````

## 第 17 步：RetryPurge

插入位置：第 16 步代码之后，紧接追加；完成稿约第 304 行。

本步声明：`func RetryPurge(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;PURGE_PENDING&#x27; WHERE id=? AND generati …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>PurgeTask</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;PURGE_PENDING&#x27; WHERE id=? AND generati …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;PURGE_PENDING&#x27; WHERE id=? AND generati …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=13877:14254 -->
````go
func RetryPurge(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='PURGE_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", task.ID, task.Generation, instance)
	return err
}

````

## 第 18 步：ExpiredTrashCandidates

插入位置：第 17 步代码之后，紧接追加；完成稿约第 309 行。

本步声明：`func ExpiredTrashCandidates(ctx context.Context, q Queryer, after uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT d.id FROM document d WHERE d.id&gt;? AND d.lifecycle=&#x27;TRASHED&#x27; AND d.purge_after&lt;=UTC_TIMESTAMP(6) 	 AND EXISTS(SELECT 1 FROM space s WHERE s.id=d.space_id …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT d.id FROM document d WHERE d.id&gt;? AND d.lifecycle=&#x27;TRASHED&#x27; AND d.purge_after&lt;=UTC_TIMESTAMP(6) 	 AND EXISTS(SELECT 1 FROM space s WHERE s.id=d.space_id …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT d.id FROM document d WHERE d.id&gt;? AND d.lifecycle=&#x27;TRASHED&#x27; AND d.purge_after&lt;=UTC_TIMESTAMP(6) 	 AND EXISTS(SELECT 1 FROM space s WHERE s.id=d.space_id …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=14254:14830 -->
````go
func ExpiredTrashCandidates(ctx context.Context, q Queryer, after uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT d.id FROM document d WHERE d.id>? AND d.lifecycle='TRASHED' AND d.purge_after<=UTC_TIMESTAMP(6)
	 AND EXISTS(SELECT 1 FROM space s WHERE s.id=d.space_id AND s.lifecycle='ACTIVE') ORDER BY d.id LIMIT 16`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/purge_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
