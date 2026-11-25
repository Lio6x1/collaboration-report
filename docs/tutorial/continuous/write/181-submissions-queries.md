# 分步编写 `services/document/internal/repository/document/submissions_queries.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/181-submissions-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存送审guard、可靠捕获版本、审核请求轮次和可恢复任务。

可靠捕获跨服务，领取租约过期不意味着旧worker有权继续落库；generation和instance共同决定所有权。

写之前先按这条执行路线理解各段的连接：

1. RegisterSubmission占单元guard
2. ClaimSubmissionTask提交领取后RPC
3. ValidateSubmissionTask复核代次
4. InsertSubmissionVersion/Resource固定正文
5. CreateSubmissionRound并CompleteSubmission落结果。

本文件需要守住的边界：

- 送审版本、资源引用、轮次和回执必须同事务
- 未知捕获结果不得释放guard让第二次送审越过。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/submissions_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/submissions_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/submissions_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/submissions_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:333 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// submission.go：保留原实现，按业务合并文件。

````

## 第 2 步：SubmissionWorkspace

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func SubmissionWorkspace(ctx context.Context, q Queryer, branch, document, unit uint64) (domain.EditWorkspace, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.i …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=333:1012 -->
````go
func SubmissionWorkspace(ctx context.Context, q Queryer, branch, document, unit uint64) (domain.EditWorkspace, error) {
	var current domain.EditWorkspace
	err := q.QueryRowContext(ctx, "SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN collaboration_unit u ON u.active_branch_id=b.id AND u.id=b.unit_id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL", branch, document, unit).Scan(&current.Workspace, &current.Epoch, &current.Lineage, &current.BaseManifest, &current.BaseVersion)
	return current, err
}

````

## 第 3 步：SubmissionRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func SubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, false)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, false)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionFact</code> | 使用 <code>domain.SubmissionFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>readSubmissionRow(ctx, q, id, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>readSubmissionRow(ctx, q, id, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return readSubmissionRow(ctx, q, id, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1012:1155 -->
````go
func SubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error) {
	return readSubmissionRow(ctx, q, id, false)
}

````

## 第 4 步：UserSubmissionRow

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

同一来源表也承载服务器固定的新审核；公开普通送审入口须按真实receipt.kind区分。

本步声明：`func UserSubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, true)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, true)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>readSubmissionRow(ctx, q, id, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionFact</code> | 使用 <code>domain.SubmissionFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>readSubmissionRow(ctx, q, id, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>readSubmissionRow(ctx, q, id, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return readSubmissionRow(ctx, q, id, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1155:1416 -->
````go
// 同一来源表也承载服务器固定的新审核；公开普通送审入口须按真实receipt.kind区分。
func UserSubmissionRow(ctx context.Context, q Queryer, id string) (domain.SubmissionFact, error) {
	return readSubmissionRow(ctx, q, id, true)
}

````

## 第 5 步：readSubmissionRow

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func readSubmissionRow(ctx context.Context, q Queryer, id string, userSubmission bool) (domain.SubmissionFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, statement, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, statement, id)</code> |
| <code>userSubmission</code> | <code>bool</code> | true额外要求同号SUBMISSION回执，避免把反向生成等其他来源混成用户主动提交。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionFact</code> | 使用 <code>domain.SubmissionFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>userSubmission</code>。
- 执行 <code>q.QueryRowContext(ctx, statement, id).Scan(&amp;value.Operation, &amp;value.Actor, &amp;value.Document, &amp;value.Unit, &amp;value.Branch, &amp;value.GuardGeneration, &amp;value.Request, &amp;value.Round, &amp;value.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, statement, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1416:2150 -->
````go
func readSubmissionRow(ctx context.Context, q Queryer, id string, userSubmission bool) (domain.SubmissionFact, error) {
	var value domain.SubmissionFact
	statement := "SELECT operation_id,actor_id,document_id,unit_id,branch_id,guard_generation,COALESCE(request_id,0),COALESCE(result_round_id,0),state FROM submission_operation WHERE operation_id=?"
	if userSubmission {
		statement += " AND EXISTS(SELECT 1 FROM operation_receipt r WHERE r.operation_id=submission_operation.operation_id AND r.kind='SUBMISSION')"
	}
	err := q.QueryRowContext(ctx, statement, id).Scan(&value.Operation, &value.Actor, &value.Document, &value.Unit, &value.Branch, &value.GuardGeneration, &value.Request, &value.Round, &value.State)
	return value, err
}

````

## 第 6 步：LockedReviewGuard

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func LockedReviewGuard(ctx context.Context, tx *sql.Tx, unit uint64) (domain.ReviewGuard, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_review_guard(unit_id) VALUES(?) ON DUPLICATE KEY UPDATE unit_id=VALUES(unit_id)&quot;, unit)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,&#x27;&#x27;),generation FROM unit_review_guard WHERE unit_id=? FOR UPDATE&quot;, unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_review_guard(unit_id) VALUES(?) ON DUPLICATE KEY UPDATE unit_id=VALUES(unit_id)&quot;, unit)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,&#x27;&#x27;),generation FROM unit_review_guard WHERE unit_id=? FOR UPDATE&quot;, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewGuard</code> | 使用 <code>domain.ReviewGuard</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO unit_review_guard(unit_id) VALUES(?) ON DUPLICATE KEY UPDATE unit_id=VALUES(unit_id)&quot;, unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,&#x27;&#x27;),generation FROM unit_review_guard WHERE unit_id …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,&#x27;&#x27;),generation FROM unit_review_guard WHERE unit_id=? FOR UPDATE&quot;, unit)</code>。

<!-- write-step: 6 bytes=2150:2764 -->
````go
func LockedReviewGuard(ctx context.Context, tx *sql.Tx, unit uint64) (domain.ReviewGuard, error) {
	var value domain.ReviewGuard
	_, err := tx.ExecContext(ctx, "INSERT INTO unit_review_guard(unit_id) VALUES(?) ON DUPLICATE KEY UPDATE unit_id=VALUES(unit_id)", unit)
	if err != nil {
		return value, err
	}
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(ongoing_request_id,0),COALESCE(resumable_request_id,0),COALESCE(submission_operation_id,''),generation FROM unit_review_guard WHERE unit_id=? FOR UPDATE", unit).Scan(&value.Ongoing, &value.Resumable, &value.Submission, &value.Generation)
	return value, err
}

````

## 第 7 步：LockedReviewPointer

插入位置：第 6 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func LockedReviewPointer(ctx context.Context, tx *sql.Tx, id uint64) (domain.ReviewPointer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE&quot;, id …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2764:3233 -->
````go
func LockedReviewPointer(ctx context.Context, tx *sql.Tx, id uint64) (domain.ReviewPointer, error) {
	var value domain.ReviewPointer
	err := tx.QueryRowContext(ctx, "SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=? FOR UPDATE", id).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Round, &value.Revision, &value.Status, &value.Archived)
	return value, err
}

````

## 第 8 步：RegisterSubmission

插入位置：第 7 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func RegisterSubmission(ctx context.Context, tx *sql.Tx, actor, document, unit uint64, intent domain.SubmissionIntent, workspace domain.EditWorkspace, generation uint64, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, intent.Operation, generation, unit)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, intent.Operation, generation, unit)</code> |
| <code>intent</code> | <code>domain.SubmissionIntent</code> | 使用 <code>domain.SubmissionIntent</code> 类型；其定义处的职责：SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。<br>在此声明中的实际使用：<code>intent.Operation</code>；<code>intent.Branch</code>；<code>intent.Request</code>；<code>intent.ExpectedRound</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>在此声明中的实际使用：<code>workspace.Lineage</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, intent.Operation, generation, unit)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,line …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, intent.Operation, generation, unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3233:4302 -->
````go
func RegisterSubmission(ctx context.Context, tx *sql.Tx, actor, document, unit uint64, intent domain.SubmissionIntent, workspace domain.EditWorkspace, generation uint64, payload []byte) error {
	// 实际 DDL 保留 receipt→submission→guard 的外键链；都在一个事务内按依赖顺序插入。
	_, err := tx.ExecContext(ctx, "INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,request_id,expected_round_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at) VALUES(?,?,?,?,?,NULLIF(?,0),NULLIF(?,0),'PREPARING',?,?,?,UTC_TIMESTAMP(6))", intent.Operation, actor, document, unit, intent.Branch, intent.Request, intent.ExpectedRound, generation, intent.Flush, workspace.Lineage)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?", intent.Operation, generation, unit); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "SUBMISSION_RECOVER", intent.Operation, intent.Operation, payload, "QUEUED")
}

````

## 第 9 步：InsertSubmissionTask

插入位置：第 8 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func InsertSubmissionTask(ctx context.Context, tx *sql.Tx, kind, key, operation string, payload []byte, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))&quot;, kind, key, operation, string(payload), state)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4302:4638 -->
````go
func InsertSubmissionTask(ctx context.Context, tx *sql.Tx, kind, key, operation string, payload []byte, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))", kind, key, operation, string(payload), state)
	return err
}

````

## 第 10 步：ConfigureSubmissionAdmission

插入位置：第 9 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func ConfigureSubmissionAdmission(ctx context.Context, tx *sql.Tx, admission string, unit uint64, operation string, target []byte, expires int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |
| <code>expires</code> | <code>int64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?&quot;, unit, operation, string(target), expires, admission)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4638:5017 -->
````go
func ConfigureSubmissionAdmission(ctx context.Context, tx *sql.Tx, admission string, unit uint64, operation string, target []byte, expires int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=?,operation_id=?,target_json=?,expires_at=FROM_UNIXTIME(?/1000) WHERE admission_id=?", unit, operation, string(target), expires, admission)
	return err
}

````

## 第 11 步：SubmissionResult

插入位置：第 10 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func SubmissionResult(ctx context.Context, q Queryer, id string) (domain.SubmissionResultFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;SUBMISSION&#x27;&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;SUBMISSION&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionResultFact</code> | 使用 <code>domain.SubmissionResultFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;SUBMISSION&#x27;&quot;, id).Scan(&amp;value.State …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;SUBMISSION&#x27;&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5017:5442 -->
````go
func SubmissionResult(ctx context.Context, q Queryer, id string) (domain.SubmissionResultFact, error) {
	var value domain.SubmissionResultFact
	err := q.QueryRowContext(ctx, "SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='SUBMISSION'", id).Scan(&value.State, &value.Generation, &value.Fingerprint, &value.Result, &value.Expires)
	return value, err
}

````

## 第 12 步：SubmissionPayload

插入位置：第 11 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func SubmissionPayload(ctx context.Context, q Queryer, id string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;SUBMISSION_RECOVER&#x27; AND business_key=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;SUBMISSION_RECOVER&#x27; AND business_key=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;SUBMISSION_RECOVER&#x27; AND business_key=?&quot;, id).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;SUBMISSION_RECOVER&#x27; AND business_key=?&quot;, id)</code>。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5442:5695 -->
````go
func SubmissionPayload(ctx context.Context, q Queryer, id string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='SUBMISSION_RECOVER' AND business_key=?", id).Scan(&raw)
	return raw, err
}

````

## 第 13 步：InsertSubmissionVersion

插入位置：第 12 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func InsertSubmissionVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockDocumentCapacity(ctx, tx, content.DocumentId)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续）；<code>ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockDocumentCapacity(ctx, tx, content.DocumentId)</code>；<code>tx.ExecContext</code>；<code>ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.StructureId</code>；<code>content.BodyJsonUtf8</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5695:6721 -->
````go
func InsertSubmissionVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error) {
	if err := LockDocumentCapacity(ctx, tx, content.DocumentId); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,?,?,?,'SUBMISSION',?,?,?)", content.DocumentId, content.UnitId, content.StructureId, string(content.BodyJsonUtf8), content.BodyJsonUtf8, content.BodyHash, content.SchemaVersion, content.CanonicalizationVersion, content.ByteSize, content.OperationId, content.CaptureId, string(content.ProvenanceJsonUtf8), content.ProvenanceSchema)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err == nil {
		err = ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)
	}
	return uint64(id), err
}

````

## 第 14 步：InsertSubmissionResource

插入位置：第 13 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func InsertSubmissionResource(ctx context.Context, tx *sql.Tx, resource, document, version uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;VERSION&#x27;,?,&#x27;CONTENT&#x27;)&quot;, resource, document, domain.TextID(version))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>resource</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;VERSION&#x27;,?,&#x27;CONTENT&#x27;)&quot;, resource, document, domain.TextID(version))</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;VERSION&#x27;,?,&#x27;CONTENT&#x27;)&quot;, resource, document, domain.TextID(version))</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,&#x27;VERSION&#x27;,?,&#x27;CONTENT&#x27;)&quot;, resource, document, domain.TextID(version))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6721:7040 -->
````go
func InsertSubmissionResource(ctx context.Context, tx *sql.Tx, resource, document, version uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'VERSION',?,'CONTENT')", resource, document, domain.TextID(version))
	return err
}

````

## 第 15 步：CreateReviewRequest

插入位置：第 14 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func CreateReviewRequest(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,&#x27;REVIEWING&#x27;)&quot;, document, unit, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,&#x27;REVIEWING&#x27;)&quot;, document, unit, branch)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,&#x27;REVIEWING&#x27;)&quot;, document, unit, branch)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,&#x27;REVIEWING&#x27;)&quot;, document, unit, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,&#x27;REVIEWING&#x27;)&quot;, document, unit, branch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7040:7398 -->
````go
func CreateReviewRequest(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO review_request(document_id,unit_id,branch_id,status) VALUES(?,?,?,'REVIEWING')", document, unit, branch)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

````

## 第 16 步：CreateSubmissionRound

插入位置：第 15 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func CreateSubmissionRound(ctx context.Context, tx *sql.Tx, request, actor, version uint64, fixed *pb.FixedContent, workspace domain.EditWorkspace) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?&quot;, request)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submit …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?&quot;, request)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submit …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submit …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submit …</code>（长表达式，按本步完整代码继续） |
| <code>fixed</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>fixed.DocumentId</code>；<code>fixed.UnitId</code>；<code>fixed.StructureId</code>；<code>fixed.CaptureId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>在此声明中的实际使用：<code>workspace.BaseManifest</code>；<code>workspace.BaseVersion</code>；<code>workspace.Lineage</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?&quot;, request).Scan(&amp;number)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?&quot;, request)</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submit …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7398:8294 -->
````go
func CreateSubmissionRound(ctx context.Context, tx *sql.Tx, request, actor, version uint64, fixed *pb.FixedContent, workspace domain.EditWorkspace) (uint64, error) {
	var number uint64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(round_no),0)+1 FROM review_round WHERE request_id=?", request).Scan(&number); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO review_round(request_id,document_id,unit_id,round_no,content_version_id,base_manifest_id,base_version_id,structure_id,lineage_id,capture_ref,submitter_id,submission_operation_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", request, fixed.DocumentId, fixed.UnitId, number, version, workspace.BaseManifest, workspace.BaseVersion, fixed.StructureId, workspace.Lineage, fixed.CaptureId, actor, fixed.OperationId)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

````

## 第 17 步：CompleteSubmission

插入位置：第 16 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func CompleteSubmission(ctx context.Context, tx *sql.Tx, operation string, actor, unit, request, round uint64, result map[string]any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=? …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;REVIEWING&#x27;,current_round_id=?,revision=revision+1 WHERE id=?&quot;, round, request)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?&quot;, request, unit)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;SUCCEEDED&#x27;,request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?&quot;, request, round, operation, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;SUCCEEDED&#x27;,request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?&quot;, request, round, operation, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(raw), operation)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=? …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?&quot;, request, unit)</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=? …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;REVIEWING&#x27;,current_round_id=?,revision=revision+1 WHERE id=?&quot;, round, request)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?&quot;, request, unit)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;SUCCEEDED&#x27;,request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?&quot;, request, round, operation, operation)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=? …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;REVIEWING&#x27;,current_round_id=?,revision=revision+1 WHERE id=?&quot;, round, request)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;SUCCEEDED&#x27;,request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?&quot;, request, round, operation, operation)</code> |
| <code>result</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>json.Marshal(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=? …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;REVIEWING&#x27;,current_round_id=?,revision=revision+1 WHERE id=?&quot;, round, request)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?&quot;, request, unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;SUCCEEDED&#x27;,request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?&quot;, request, round, operation, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(raw), operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 17 bytes=8294:9771 -->
````go
func CompleteSubmission(ctx context.Context, tx *sql.Tx, operation string, actor, unit, request, round uint64, result map[string]any) error {
	// 历次作者是不可变事实，服务角色没有 UPDATE 权限。请求已持锁，已存在作者只读保留。
	if _, err := tx.ExecContext(ctx, "INSERT INTO request_submitter(request_id,user_id,first_round_id) SELECT ?,?,? WHERE NOT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)", request, actor, round, request, actor); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='REVIEWING',current_round_id=?,revision=revision+1 WHERE id=?", round, request); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=?,resumable_request_id=NULL,submission_operation_id=NULL WHERE unit_id=?", request, unit); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='SUCCEEDED',request_id=?,result_round_id=?,capture_ref=? WHERE operation_id=?", request, round, operation, operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state='PREPARING'", string(raw), operation)
	return err
}

// submission_readiness.go：保留原实现，按业务合并文件。

````

## 第 18 步：PendingSubmissionContexts

插入位置：第 17 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func PendingSubmissionContexts(ctx context.Context, db *sql.DB) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, &quot;SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(&#x27;kind&#x27;,t.kind,&#x27;operation&#x27;,t.operation_id,&#x27;payload&#x27;,t.payload,&#x27;actor&#x27;,s.actor_id,&#x27;document&#x27;,s.document_id,&#x27;unit&#x27;,s …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, &quot;SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(&#x27;kind&#x27;,t.kind,&#x27;operation&#x27;,t.operation_id,&#x27;payload&#x27;,t.payload,&#x27;actor&#x27;,s.actor_id,&#x27;document&#x27;,s.document_id,&#x27;unit&#x27;,s …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, &quot;SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(&#x27;kind&#x27;,t.kind,&#x27;operation&#x27;,t.operation_id,&#x27;payload&#x27;,t.payload,&#x27;actor&#x27;,s.actor_id,&#x27;document&#x27;,s.document_id,&#x27;unit&#x27;,s …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9771:10804 -->
````go
func PendingSubmissionContexts(ctx context.Context, db *sql.DB) ([]byte, error) {
	// 聚合仅用于验证持久绑定，不依据租约或排队时间推断业务已成功。
	var raw []byte
	err := db.QueryRowContext(ctx, "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('kind',t.kind,'operation',t.operation_id,'payload',t.payload,'actor',s.actor_id,'document',s.document_id,'unit',s.unit_id,'branch',s.branch_id,'guard_generation',s.guard_generation,'state',s.state,'receipt_state',r.state,'held_operation',g.submission_operation_id,'held_generation',g.generation,'round',s.result_round_id)),JSON_ARRAY()) FROM durable_task t LEFT JOIN submission_operation s ON s.operation_id=t.operation_id LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id LEFT JOIN unit_review_guard g ON g.unit_id=s.unit_id WHERE t.kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','REVIEW_NOTIFICATION') AND t.state NOT IN ('SUCCEEDED','REJECTED')").Scan(&raw)
	return raw, err
}

// submission_tasks.go：保留原实现，按业务合并文件。

````

## 第 19 步：ClaimSubmissionTask

插入位置：第 18 步代码之后，紧接追加；完成稿约第 164 行。

领取提交后才调用 RPC。租约只是重新领取条件，最终写入还必须验证 generation 与实例。

本步声明：`func ClaimSubmissionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, &quot;CAPTURE_FINALIZE&quot;)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 10 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, &quot;SUBMISSION_RECOVER&quot;, &quot;CAPTURE_FINALIZE&quot;)</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>task.Instance = instance</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 10 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 10 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 19 bytes=10804:11453 -->
````go
// 领取提交后才调用 RPC。租约只是重新领取条件，最终写入还必须验证 generation 与实例。
func ClaimSubmissionTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	task, err := lockClaimableOperationTask(ctx, tx, "SUBMISSION_RECOVER", "CAPTURE_FINALIZE")
	if err != nil {
		return task, err
	}
	task.Generation++
	task.Instance = instance
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 10 SECOND),attempts=attempts+1 WHERE id=?", task.Generation, instance, task.ID)
	return task, err
}

````

## 第 20 步：ValidateSubmissionTask

插入位置：第 19 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func ValidateSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_WORKER_SUPERSEDED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID).Scan(&amp;generation, &amp;state, &amp;instance)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>generation != task.Generation &#124;&#124; state != &quot;RUNNING&quot; &#124;&#124; instance != task.Instance</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_WORKER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11453:11998 -->
````go
func ValidateSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error {
	var generation uint64
	var state, instance string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? FOR UPDATE", task.ID).Scan(&generation, &state, &instance)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || instance != task.Instance {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_WORKER_SUPERSEDED")
	}
	return nil
}

````

## 第 21 步：FinishSubmissionTask

插入位置：第 20 步代码之后，紧接追加；完成稿约第 189 行。

本步声明：`func FinishSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, state, task.ID, task.Generation, task.Instance)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, state, task.ID, task.Generation, task.Instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state=&#x27;RUNNING&#x27;&quot;, state, task.ID, task.Generation, task.Instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=11998:12319 -->
````go
func FinishSubmissionTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", state, task.ID, task.Generation, task.Instance)
	return err
}

````

## 第 22 步：RetrySubmissionTask

插入位置：第 21 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`func RetrySubmissionTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND insta …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND insta …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND insta …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=12319:12821 -->
````go
func RetrySubmissionTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	// 只记录稳定业务码；驱动错误可能包含私有 DSN，不能进入日志或持久业务输出。
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", code, task.ID, task.Generation, task.Instance)
	return err
}

````

## 第 23 步：RejectSubmission

插入位置：第 22 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func RejectSubmission(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, unit uint64, result map[string]any, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, task.Operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state IN (&#x27;PREPARING&#x27;,&#x27;CAPTURED&#x27;)&quot;, code, task.Operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(raw), task.Operation)</code>；<code>FinishSubmissionTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>FinishSubmissionTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishSubmissionTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, task.Operation)</code> |
| <code>result</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>json.Marshal(result)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state IN (&#x27;PREPARING&#x27;,&#x27;CAPTURED&#x27;)&quot;, code, task.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishSubmissionTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state IN (&#x27;PREPARING&#x27;,&#x27;CAPTURED&#x27;)&quot;, code, task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;REJECTED&#x27;,result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, string(raw), task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return FinishSubmissionTask(ctx, tx, task, &quot;REJECTED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=12821:13716 -->
````go
func RejectSubmission(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, unit uint64, result map[string]any, code string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?", unit, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state IN ('PREPARING','CAPTURED')", code, task.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND state='PREPARING'", string(raw), task.Operation); err != nil {
		return err
	}
	return FinishSubmissionTask(ctx, tx, task, "REJECTED")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/submissions_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
