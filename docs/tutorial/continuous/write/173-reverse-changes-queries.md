# 分步编写 `services/document/internal/repository/document/reverse_changes_queries.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/173-reverse-changes-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

通过审核、合并、轮次外键链定位反向范围，并存储新反向操作及审计。

反向动作撤销的是某次已合并贡献，不能从客户端现存块ID猜归属。

写之前先按这条执行路线理解各段的连接：

1. ReverseMerge取得真实输入输出版本
2. RegisterReverse固定旧merge及当前工作区
3. InsertReverseLineage与InsertReverseAudit记录新事实
4. CompleteReverseTask保存安装。

本文件需要守住的边界：

- 旧合并和旧审核请求保持不可变
- 反向产生新的审核事实而不是回滚旧记录。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/reverse_changes_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/reverse_changes_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/reverse_changes_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/reverse_changes_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:297 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// reverse_changes.go：保留原实现，按业务合并文件。

````

## 第 2 步：ReverseMergeRecord

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。

本步声明：`type ReverseMergeRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 被反向处理的原合并记录编号。 |
| <code>Document</code> | <code>uint64</code> | 由审核外键链确认的原文档。 |
| <code>Unit</code> | <code>uint64</code> | 原合并对应的协作单元。 |
| <code>Branch</code> | <code>uint64</code> | 原审核来源支线。 |
| <code>Request</code> | <code>uint64</code> | 原审核请求编号。 |
| <code>Round</code> | <code>uint64</code> | 原审核轮次编号。 |
| <code>Attempt</code> | <code>uint64</code> | 原合并尝试编号。 |
| <code>InputManifest</code> | <code>uint64</code> | 原合并之前的主线清单。 |
| <code>OutputManifest</code> | <code>uint64</code> | 原合并之后的主线清单。 |
| <code>Version</code> | <code>uint64</code> | 原合并产出的正文版本。 |
| <code>Lineage</code> | <code>uint64</code> | 原合并关联的支线谱系。 |
| <code>Structure</code> | <code>uint64</code> | 原合并尝试固定的结构。 |
| <code>Operation</code> | <code>string</code> | 原合并操作号，不是本次反向操作号。 |
| <code>Capture</code> | <code>string</code> | 原合并接受的固定采样引用。 |
| <code>State</code> | <code>string</code> | 原合并操作状态，必须先确认原操作成功。 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 原合并输入摘要，用于固定反向处理的来源。 |

<!-- write-step: 2 bytes=297:717 -->
````go
// 归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。
type ReverseMergeRecord struct {
	ID, Document, Unit, Branch, Request, Round, Attempt        uint64
	InputManifest, OutputManifest, Version, Lineage, Structure uint64
	Operation, Capture, State                                  string
	Digest                                                     []byte
}

````

## 第 3 步：ReverseActiveBranch

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func ReverseActiveBranch(ctx context.Context, q Queryer, unit, document uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, unit, document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, unit, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, unit, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>branch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, unit, document).Scan(&amp;branch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, unit, document)</code>。
- 此处通过 <code>return branch, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=717:1031 -->
````go
func ReverseActiveBranch(ctx context.Context, q Queryer, unit, document uint64) (uint64, error) {
	var branch uint64
	err := q.QueryRowContext(ctx, "SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL", unit, document).Scan(&branch)
	return branch, err
}

````

## 第 4 步：ReverseMerge

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func ReverseMerge(ctx context.Context, q Queryer, document, record uint64) (ReverseMergeRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,  m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineag …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,  m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineag …</code>（长表达式，按本步完整代码继续） |
| <code>record</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,  m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineag …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReverseMergeRecord</code> | 使用 <code>ReverseMergeRecord</code> 类型；其定义处的职责：归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,  m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineag …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,  m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineag …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1031:2036 -->
````go
func ReverseMerge(ctx context.Context, q Queryer, document, record uint64) (ReverseMergeRecord, error) {
	var value ReverseMergeRecord
	err := q.QueryRowContext(ctx, `SELECT m.id,r.document_id,r.unit_id,r.branch_id,m.request_id,m.round_id,m.attempt_id,
 m.input_manifest_id,m.output_manifest_id,m.result_version_id,m.lineage_id,a.structure_id,m.operation_id,m.accepted_capture_ref,o.state,a.input_digest
 FROM merge_record m JOIN review_request r ON r.id=m.request_id JOIN review_round v ON v.id=m.round_id AND v.request_id=r.id
 JOIN merge_attempt a ON a.id=m.attempt_id AND a.operation_id=m.operation_id JOIN merge_operation o ON o.operation_id=m.operation_id
 WHERE m.id=? AND r.document_id=?`, record, document).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Request, &value.Round, &value.Attempt,
		&value.InputManifest, &value.OutputManifest, &value.Version, &value.Lineage, &value.Structure, &value.Operation, &value.Capture, &value.State, &value.Digest)
	return value, err
}

````

## 第 5 步：ReverseResult

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func ReverseResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;REVERSE&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;REVERSE&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 使用 <code>MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;REVERSE&#x27;&quot;, operation).Scan …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;REVERSE&#x27;&quot;, operation)</code>。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2036:2466 -->
````go
func ReverseResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='REVERSE'", operation).Scan(&fact.Actor, &fact.State, &fact.Generation, &fact.Fingerprint, &fact.Result, &fact.Expires)
	return fact, err
}

````

## 第 6 步：RegisterReverse

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func RegisterReverse(ctx context.Context, tx *sql.Tx, record ReverseMergeRecord, actor uint64, workspace domain.EditWorkspace, generation uint64, operation string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, workspace.Workspace, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)  VALUES(?,?, …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, operation, generation, record.Unit)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, workspace.Workspace, operation)</code>；<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>record</code> | <code>ReverseMergeRecord</code> | 使用 <code>ReverseMergeRecord</code> 类型；其定义处的职责：归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。<br>在此声明中的实际使用：<code>record.Document</code>；<code>record.Unit</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)  VALUES(?,?, …</code>（长表达式，按本步完整代码继续） |
| <code>workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>在此声明中的实际使用：<code>workspace.Workspace</code>；<code>workspace.Lineage</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)  VALUES(?,?, …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, operation, generation, record.Unit)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, workspace.Workspace, operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)  VALUES(?,?, …</code>（长表达式，按本步完整代码继续）；<code>&quot;reverse:&quot;+operation</code>；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, operation, generation, record.Unit)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)  VALUES(?,?, …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?&quot;, operation, generation, record.Unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2466:3698 -->
````go
func RegisterReverse(ctx context.Context, tx *sql.Tx, record ReverseMergeRecord, actor uint64, workspace domain.EditWorkspace, generation uint64, operation string, payload []byte) error {
	if err := SetBranchControl(ctx, tx, workspace.Workspace, operation); err != nil {
		return err
	}
	// flush_receipt_ref是既定必填文本，不具有FLUSH外键；服务器来源使用明确命名，不伪造用户保存确认。
	_, err := tx.ExecContext(ctx, `INSERT INTO submission_operation(operation_id,actor_id,document_id,unit_id,branch_id,state,guard_generation,flush_receipt_ref,lineage_id,retry_at)
 VALUES(?,?,?,?,?,'PREPARING',?,?,?,UTC_TIMESTAMP(6))`, operation, actor, record.Document, record.Unit, workspace.Workspace, generation, "reverse:"+operation, workspace.Lineage)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=?,generation=? WHERE unit_id=?", operation, generation, record.Unit); err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "REVERSE_CONTEXT", operation, operation, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "REVERSE_RECOVER", operation, operation, payload, "QUEUED")
}

````

## 第 7 步：ClaimReverseTask

插入位置：第 6 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func ClaimReverseTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;REVERSE_RECOVER&quot;, &quot;REVERSE_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3698:3892 -->
````go
func ClaimReverseTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "REVERSE_RECOVER", "REVERSE_RECONCILE")
}

````

## 第 8 步：CompleteReverseTask

插入位置：第 7 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func CompleteReverseTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>FinishMergeTask(ctx, tx, task, state)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVERSE_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, state)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3892:4354 -->
````go
func CompleteReverseTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string, install []byte) error {
	if err := InsertSubmissionTask(ctx, tx, "REVERSE_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "REVERSE_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, state)
}

````

## 第 9 步：InsertReverseLineage

插入位置：第 8 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func InsertReverseLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error`。


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

<!-- write-step: 9 bytes=4354:4916 -->
````go
func InsertReverseLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema)
 VALUES(?,?,?,?,?,'RESTORED',?,?,?,?,?,'provenance-v1')`, id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}

````

## 第 10 步：ReverseInstalled

插入位置：第 9 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func ReverseInstalled(ctx context.Context, q Queryer, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;REVERSE_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;REVERSE_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>n == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;REVERSE_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation).Scan(&amp;n)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;REVERSE_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>。
- 此处通过 <code>return n == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4916:5201 -->
````go
func ReverseInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var n uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='REVERSE_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&n)
	return n == 1, err
}

````

## 第 11 步：InsertReverseAudit

插入位置：第 10 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func InsertReverseAudit(ctx context.Context, tx *sql.Tx, operation string, actor, merge, request, round, version uint64, reason string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,&#x27;REVERSE&#x27;,?,?)&quot;, actor, operation, string(target), reason)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,&#x27;REVERSE&#x27;,?,?)&quot;, actor, operation, string(target), reason)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,&#x27;REVERSE&#x27;,?,?)&quot;, actor, operation, string(target), reason)</code> |
| <code>merge</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(merge)</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(request)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(round)</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(version)</code> |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,&#x27;REVERSE&#x27;,?,?)&quot;, actor, operation, string(target), reason)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,&#x27;REVERSE&#x27;,?,?)&quot;, actor, operation, string(target), reason)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 11 bytes=5201:5782 -->
````go
func InsertReverseAudit(ctx context.Context, tx *sql.Tx, operation string, actor, merge, request, round, version uint64, reason string) error {
	target, err := json.Marshal(map[string]string{"merge_record_id": domain.TextID(merge), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,reason) VALUES(?,?,0,'REVERSE',?,?)", actor, operation, string(target), reason)
	return err
}

````

## 第 12 步：RejectReverseSource

插入位置：第 11 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func RejectReverseSource(ctx context.Context, tx *sql.Tx, operation string, unit uint64, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, code, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, code, operation)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, operation)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, code, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?&quot;, unit, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE submission_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, code, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 12 bytes=5782:6321 -->
````go
func RejectReverseSource(ctx context.Context, tx *sql.Tx, operation string, unit uint64, code string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET submission_operation_id=NULL WHERE unit_id=? AND submission_operation_id=?", unit, operation); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE submission_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state='PREPARING'", code, operation)
	return err
}

// reverse_readiness.go：保留原实现，按业务合并文件。

````

## 第 13 步：PendingReverseBinding

插入位置：第 12 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`type PendingReverseBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>Kind</code> | <code>string</code> | 持久恢复任务种类，区分计算、安装与核验阶段。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Owner</code> | <code>string</code> | 支线控制门中持有控制权的操作号；不是worker实例名。 |
| <code>SourceState</code> | <code>string</code> | submission_operation的状态。 |
| <code>HeldOperation</code> | <code>string</code> | 当前单元审核守卫持有的提交号。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |
| <code>SourceActor</code> | <code>uint64</code> | submission_operation中原反向提交的操作者，需匹配回执actor。 |
| <code>SourceDocument</code> | <code>uint64</code> | 原反向提交绑定文档。 |
| <code>SourceUnit</code> | <code>uint64</code> | 原反向提交绑定单元。 |
| <code>SourceBranch</code> | <code>uint64</code> | 原反向提交绑定支线。 |
| <code>SourceLineage</code> | <code>uint64</code> | 原反向提交绑定谱系。 |
| <code>SourceGeneration</code> | <code>uint64</code> | 原反向提交登记的审核守卫代次。 |
| <code>HeldGeneration</code> | <code>uint64</code> | 当前守卫代次，必须与原提交代次相符。 |
| <code>Round</code> | <code>uint64</code> | 原反向提交生成的审核轮次。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Committed</code> | <code>&#91;&#93;byte</code> | SQL已提交阶段保存的安装载荷，用于后续Node安装核验。 |

<!-- write-step: 13 bytes=6321:6735 -->
````go
type PendingReverseBinding struct {
	Operation, Kind, State, Owner, SourceState, HeldOperation                                                            string
	Actor, SourceActor, SourceDocument, SourceUnit, SourceBranch, SourceLineage, SourceGeneration, HeldGeneration, Round uint64
	Original, Payload, Target, Fingerprint, Committed                                                                    []byte
}

````

## 第 14 步：PendingReverseBindings

插入位置：第 13 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func PendingReverseBindings(ctx context.Context, q Queryer) ([]PendingReverseBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>reverseBindings(ctx, q, &quot;&quot;)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>reverseBindings(ctx, q, &quot;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingReverseBinding</code> | 由 <code>reverseBindings(ctx, q, &quot;&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>reverseBindings(ctx, q, &quot;&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return reverseBindings(ctx, q, &quot;&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6735:6869 -->
````go
func PendingReverseBindings(ctx context.Context, q Queryer) ([]PendingReverseBinding, error) {
	return reverseBindings(ctx, q, "")
}

````

## 第 15 步：ClaimedReverseBinding

插入位置：第 14 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func ClaimedReverseBinding(ctx context.Context, q Queryer, operation string) (PendingReverseBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>reverseBindings(ctx, q, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>reverseBindings(ctx, q, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>reverseBindings(ctx, q, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PendingReverseBinding</code> | 实际返回表达式：<code>PendingReverseBinding{}</code>；<code>facts&#91;0&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return PendingReverseBinding{}, err</code>。当前函数在这里结束。
- 检查 <code>len(facts) != 1</code>。条件成立时执行 <code>return PendingReverseBinding{}, sql.ErrNoRows</code>。当前函数在这里结束。
- 此处通过 <code>return facts&#91;0&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6869:7185 -->
````go
func ClaimedReverseBinding(ctx context.Context, q Queryer, operation string) (PendingReverseBinding, error) {
	facts, err := reverseBindings(ctx, q, operation)
	if err != nil {
		return PendingReverseBinding{}, err
	}
	if len(facts) != 1 {
		return PendingReverseBinding{}, sql.ErrNoRows
	}
	return facts[0], nil
}

````

## 第 16 步：reverseBindings

插入位置：第 15 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func reverseBindings(ctx context.Context, q Queryer, operation string) ([]PendingReverseBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),COALESCE(original.payload,&#x27;{}&#x27;),t.payload,  COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),COALESCE(original.payload,&#x27;{}&#x27;),t.payload,  COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingReverseBinding</code> | 实际返回表达式：<code>nil</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,&#x27;&#x27;),COALESCE(r.actor_id,0),COALESCE(original.payload,&#x27;{}&#x27;),t.payload,  COALESCE(r.target_json,&#x27;{}&#x27;),COALESCE(r.re …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;fact.Operation, &amp;fact.Kind, &amp;fact.State, &amp;fact.Actor, &amp;fact.Original, &amp;fact.Payload, &amp;fact.Target, &amp;fact.Fingerprint, &amp;fact.Owner, &amp;fact.Committed, 			&amp;fact.SourceActor, …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return facts, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7185:9357 -->
````go
func reverseBindings(ctx context.Context, q Queryer, operation string) ([]PendingReverseBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.operation_id,t.kind,COALESCE(r.state,''),COALESCE(r.actor_id,0),COALESCE(original.payload,'{}'),t.payload,
 COALESCE(r.target_json,'{}'),COALESCE(r.request_fingerprint,''),COALESCE(JSON_UNQUOTE(JSON_EXTRACT(gate.payload,'$.operation_id')),''),COALESCE(committed.payload,'{}'),
 COALESCE(s.actor_id,0),COALESCE(s.document_id,0),COALESCE(s.unit_id,0),COALESCE(s.branch_id,0),COALESCE(s.lineage_id,0),COALESCE(s.guard_generation,0),COALESCE(s.state,''),COALESCE(s.result_round_id,0),COALESCE(g.submission_operation_id,''),COALESCE(g.generation,0)
 FROM durable_task t LEFT JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='REVERSE'
 LEFT JOIN durable_task original ON original.kind='REVERSE_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.Scope.Workspace.Workspace'))
 LEFT JOIN durable_task committed ON committed.kind='REVERSE_COMMITTED' AND committed.business_key=t.operation_id AND committed.operation_id=t.operation_id
 LEFT JOIN submission_operation s ON s.operation_id=t.operation_id LEFT JOIN unit_review_guard g ON g.unit_id=s.unit_id
 WHERE t.kind IN ('REVERSE_RECOVER','REVERSE_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (?='' OR t.operation_id=?) ORDER BY t.id`, operation, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facts := []PendingReverseBinding{}
	for rows.Next() {
		var fact PendingReverseBinding
		if err = rows.Scan(&fact.Operation, &fact.Kind, &fact.State, &fact.Actor, &fact.Original, &fact.Payload, &fact.Target, &fact.Fingerprint, &fact.Owner, &fact.Committed,
			&fact.SourceActor, &fact.SourceDocument, &fact.SourceUnit, &fact.SourceBranch, &fact.SourceLineage, &fact.SourceGeneration, &fact.SourceState, &fact.Round, &fact.HeldOperation, &fact.HeldGeneration); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/reverse_changes_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
