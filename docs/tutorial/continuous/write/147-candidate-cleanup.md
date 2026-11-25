# 分步编写 `services/document/internal/repository/document/candidate_cleanup.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/147-candidate-cleanup.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取并保存候选清理许可，核业务终态、系统安装及canonical副本。

登记任务SUCCEEDED不等于正文已复制或MAIN已安装，提前删Node候选会失去恢复来源。

写之前先按这条执行路线理解各段的连接：

1. SubmissionCleanupCopy查真实送审复制
2. MergeCleanupCopy查指定attempt
3. CandidateCleanupTerminal核原动作终态
4. SaveCandidateCleanupPermit固定许可。

本文件需要守住的边界：

- 许可必须同时具备完整副本与收尾证据，不能仅以任务租约到期批准。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/candidate_cleanup.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/candidate_cleanup.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/candidate_cleanup.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/candidate_cleanup.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:57 -->
````go
package document

import (
	"context"
	"database/sql"
)

````

## 第 2 步：CandidateCleanupPermit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`func CandidateCleanupPermit(ctx context.Context, q Queryer, reference string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANUP_PERMIT&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;`, reference)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>reference</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANUP_PERMIT&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;`, reference)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANUP_PERMIT&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;`, reference).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind=&#x27;CANDIDATE_CLEANUP_PERMIT&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;`, reference)</code>。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=57:357 -->
````go
func CandidateCleanupPermit(ctx context.Context, q Queryer, reference string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind='CANDIDATE_CLEANUP_PERMIT' AND business_key=? AND state='SUCCEEDED'`, reference).Scan(&raw)
	return raw, err
}

````

## 第 3 步：SaveCandidateCleanupPermit

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func SaveCandidateCleanupPermit(ctx context.Context, tx *sql.Tx, reference, operation string, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |
| <code>reference</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;CANDIDATE_CLEANUP_PERMIT&quot;, reference, operation, raw, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=357:580 -->
````go
func SaveCandidateCleanupPermit(ctx context.Context, tx *sql.Tx, reference, operation string, raw []byte) error {
	return InsertSubmissionTask(ctx, tx, "CANDIDATE_CLEANUP_PERMIT", reference, operation, raw, "SUCCEEDED")
}

````

## 第 4 步：SubmissionCleanupCopy

插入位置：第 3 步代码之后，紧接追加；完成稿约第 18 行。

只接受原业务完成、系统收尾完成以及完整canonical副本同时成立的候选。
受理/登记任务的SUCCEEDED不能替代正文已复制或MAIN已经安装的证明。

本步声明：`func SubmissionCleanupCopy(ctx context.Context, tx *sql.Tx, operation string) (uint64, string, uint64, []byte, []byte, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;CAPTU …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;CAPTU …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>generation</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>digest</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>actual</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>finalized</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;CAPTU …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;CAPTU …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return generation, state, version, digest, actual, finalized, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=580:1633 -->
````go
// 只接受原业务完成、系统收尾完成以及完整canonical副本同时成立的候选。
// 受理/登记任务的SUCCEEDED不能替代正文已复制或MAIN已经安装的证明。
func SubmissionCleanupCopy(ctx context.Context, tx *sql.Tx, operation string) (uint64, string, uint64, []byte, []byte, bool, error) {
	var generation, version uint64
	var state string
	var digest, actual []byte
	var finalized bool
	err := tx.QueryRowContext(ctx, `SELECT s.guard_generation,s.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)),
	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='CAPTURE_FINALIZE' AND t.business_key=s.operation_id AND t.operation_id=s.operation_id AND t.state='SUCCEEDED')
	 FROM submission_operation s LEFT JOIN content_version v ON v.source_operation_id=s.operation_id AND v.source_kind='SUBMISSION' AND v.source_ref=s.operation_id
	 WHERE s.operation_id=?`, operation).Scan(&generation, &state, &version, &digest, &actual, &finalized)
	return generation, state, version, digest, actual, finalized, err
}

````

## 第 5 步：MergeCleanupCopy

插入位置：第 4 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func MergeCleanupCopy(ctx context.Context, tx *sql.Tx, operation string, attempt uint64) (string, uint64, []byte, []byte, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;MAIN_INSTALL_COMPLETED&#x27;  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;MAIN_INSTALL_COMPLETED&#x27;  …</code>（长表达式，按本步完整代码继续） |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;MAIN_INSTALL_COMPLETED&#x27;  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>digest</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>actual</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>installed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;MAIN_INSTALL_COMPLETED&#x27;  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)), 	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;MAIN_INSTALL_COMPLETED&#x27;  …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return state, version, digest, actual, installed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1633:2507 -->
````go
func MergeCleanupCopy(ctx context.Context, tx *sql.Tx, operation string, attempt uint64) (string, uint64, []byte, []byte, bool, error) {
	var state string
	var version uint64
	var digest, actual []byte
	var installed bool
	err := tx.QueryRowContext(ctx, `SELECT a.state,COALESCE(v.id,0),v.body_hash,UNHEX(SHA2(v.canonical_body,256)),
	 EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='MAIN_INSTALL_COMPLETED' AND t.business_key=a.operation_id AND t.operation_id=a.operation_id AND t.state='SUCCEEDED')
	 FROM merge_attempt a LEFT JOIN merge_record m ON m.attempt_id=a.id AND m.operation_id=a.operation_id
	 LEFT JOIN content_version v ON v.id=m.result_version_id AND v.source_operation_id=a.operation_id
	 WHERE a.id=? AND a.operation_id=?`, attempt, operation).Scan(&state, &version, &digest, &actual, &installed)
	return state, version, digest, actual, installed, err
}

````

## 第 6 步：CandidateCleanupTerminal

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func CandidateCleanupTerminal(ctx context.Context, tx *sql.Tx, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state=&#x27;SUCCEEDED&#x27; AND terminal_at IS NOT NULL) 	 AND NOT EXISTS(SELECT 1 FROM durabl …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state=&#x27;SUCCEEDED&#x27; AND terminal_at IS NOT NULL) 	 AND NOT EXISTS(SELECT 1 FROM durabl …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>complete</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state=&#x27;SUCCEEDED&#x27; AND terminal_at IS NOT NULL) 	 AND NOT EXISTS(SELECT 1 FROM durabl …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state=&#x27;SUCCEEDED&#x27; AND terminal_at IS NOT NULL) 	 AND NOT EXISTS(SELECT 1 FROM durabl …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return complete, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2507:3033 -->
````go
func CandidateCleanupTerminal(ctx context.Context, tx *sql.Tx, operation string) (bool, error) {
	var complete bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM operation_receipt WHERE operation_id=? AND state='SUCCEEDED' AND terminal_at IS NOT NULL)
	 AND NOT EXISTS(SELECT 1 FROM durable_task WHERE operation_id=? AND kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','MERGE_RECOVER','MAIN_RECONCILE') AND state NOT IN ('SUCCEEDED','REJECTED'))`, operation, operation).Scan(&complete)
	return complete, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/candidate_cleanup.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
