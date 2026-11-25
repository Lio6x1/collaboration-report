# 分步编写 `services/document/internal/repository/document/pull_main_queries.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/167-pull-main-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

存储支线拉取MAIN的控制占位、lineage、代次与安装进度。

拉取属于支线控制而非送审；独立占位避免破坏submission_operation_id的真实外键。

写之前先按这条执行路线理解各段的连接：

1. RequireBranchControlOpen检查空闲
2. RegisterPull固定上下文
3. InsertPullLineage追加基线来源
4. AdvancePulledBranch推进代次
5. CompletePullInstallation在ACK后完成。

本文件需要守住的边界：

- 不能把拉取操作号写入送审列
- 业务成功与Node安装完成需要分别确认。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/pull_main_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/pull_main_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/pull_main_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/pull_main_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:291 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// pull_main.go：保留原实现，按业务合并文件。

````

## 第 2 步：PullResult

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func PullResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;PULL_MAIN&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;PULL_MAIN&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlReceipt</code> | 使用 <code>MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;PULL_MAIN&#x27;&quot;, operatio …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;PULL_MAIN&#x27;&quot;, operation)</code>。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=291:737 -->
````go
func PullResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='PULL_MAIN'", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

````

## 第 3 步：LockedBranchControl

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

支线控制不写submission_operation_id：该列有真实SUBMISSION外键，不能塞入另一种业务原号。

本步声明：`func LockedBranchControl(ctx context.Context, tx *sql.Tx, branch uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;BRANCH_CONTROL_GATE&#x27;,?,&#x27;{}&#x27;,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)&quot;, key)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;BRANCH_CONTROL_GATE&#x27; AND business_key=? FOR UPDATE&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>value.Operation</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;BRANCH_CONTROL_GATE&#x27;,?,&#x27;{}&#x27;,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)&quot;, key)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;BRANCH_CONTROL_GATE&#x27; AND business_key=? FOR UPDATE&quot;, key).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;BRANCH_CONTROL_GATE&#x27; AND business_key=? FOR UPDATE&quot;, key)</code>。
- 检查 <code>json.Unmarshal(raw, &amp;value) != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value.Operation, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=737:1668 -->
````go
// 支线控制不写submission_operation_id：该列有真实SUBMISSION外键，不能塞入另一种业务原号。
func LockedBranchControl(ctx context.Context, tx *sql.Tx, branch uint64) (string, error) {
	key := domain.TextID(branch)
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('BRANCH_CONTROL_GATE',?,'{}','SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)", key)
	if err != nil {
		return "", err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='BRANCH_CONTROL_GATE' AND business_key=? FOR UPDATE", key).Scan(&raw); err != nil {
		return "", err
	}
	var value struct {
		Operation string `json:"operation_id"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	return value.Operation, nil
}
````

## 第 4 步：RequireBranchControlOpen

插入位置：第 3 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func RequireBranchControlOpen(ctx context.Context, tx *sql.Tx, branch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockedBranchControl(ctx, tx, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockedBranchControl(ctx, tx, branch)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockedBranchControl(ctx, tx, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;BRANCH_CONTROL_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>operation != &quot;&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_CONTROL_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1668:1956 -->
````go
func RequireBranchControlOpen(ctx context.Context, tx *sql.Tx, branch uint64) error {
	operation, err := LockedBranchControl(ctx, tx, branch)
	if err != nil {
		return err
	}
	if operation != "" {
		return domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	return nil
}
````

## 第 5 步：SetBranchControl

插入位置：第 4 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func SetBranchControl(ctx context.Context, tx *sql.Tx, branch uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;BRANCH_CONTROL_GATE&#x27; AND business_key=?&quot;, string(raw), domain.TextID(branch))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(branch)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET payload=? WHERE kind=&#x27;BRANCH_CONTROL_GATE&#x27; AND business_key=?&quot;, string(raw), domain.TextID(branch))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1956:2292 -->
````go
func SetBranchControl(ctx context.Context, tx *sql.Tx, branch uint64, operation string) error {
	raw, _ := json.Marshal(map[string]string{"operation_id": operation})
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE kind='BRANCH_CONTROL_GATE' AND business_key=?", string(raw), domain.TextID(branch))
	return err
}
````

## 第 6 步：RegisterPull

插入位置：第 5 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func RegisterPull(ctx context.Context, tx *sql.Tx, branch uint64, operation string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>SetBranchControl(ctx, tx, branch, operation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_CONTEXT&quot;, operation, operation, payload, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECOVER&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2292:2724 -->
````go
func RegisterPull(ctx context.Context, tx *sql.Tx, branch uint64, operation string, payload []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_CONTEXT", operation, operation, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECOVER", operation, operation, payload, "QUEUED")
}
````

## 第 7 步：ClaimPullTask

插入位置：第 6 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func ClaimPullTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;PULL_MAIN_RECOVER&quot;, &quot;PULL_MAIN_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2724:2918 -->
````go
func ClaimPullTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "PULL_MAIN_RECOVER", "PULL_MAIN_RECONCILE")
}
````

## 第 8 步：MainUnitVersion

插入位置：第 7 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`func MainUnitVersion(ctx context.Context, q Queryer, document, unit, manifest uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit).Scan(&amp;version)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?&quot;, document, manifest, unit)</code>。
- 此处通过 <code>return version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2918:3238 -->
````go
func MainUnitVersion(ctx context.Context, q Queryer, document, unit, manifest uint64) (uint64, error) {
	var version uint64
	err := q.QueryRowContext(ctx, "SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?", document, manifest, unit).Scan(&version)
	return version, err
}
````

## 第 9 步：InsertPullLineage

插入位置：第 8 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func InsertPullLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error`。


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

<!-- write-step: 9 bytes=3238:3796 -->
````go
func InsertPullLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'PULL_MAIN',?,?,?,?,?,'provenance-v1')", id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}
````

## 第 10 步：AdvancePulledBranch

插入位置：第 9 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func AdvancePulledBranch(ctx context.Context, tx *sql.Tx, branch, lineage, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle=&#x27;RESTORING&#x27;,revision=revision+1 WHERE id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, lineage, epoch, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle=&#x27;RESTORING&#x27;,revision=revision+1 WHERE id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, lineage, epoch, branch)</code> |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle=&#x27;RESTORING&#x27;,revision=revision+1 WHERE id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, lineage, epoch, branch)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle=&#x27;RESTORING&#x27;,revision=revision+1 WHERE id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, lineage, epoch, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle=&#x27;RESTORING&#x27;,revision=revision+1 WHERE id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, lineage, epoch, branch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3796:4098 -->
````go
func AdvancePulledBranch(ctx context.Context, tx *sql.Tx, branch, lineage, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle='RESTORING',revision=revision+1 WHERE id=? AND lifecycle='ACTIVE'", lineage, epoch, branch)
	return err
}
````

## 第 11 步：CompletePull

插入位置：第 10 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func CompletePull(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, result, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, result)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, result)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_COMMITTED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4098:4665 -->
````go
func CompletePull(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
````

## 第 12 步：CompletePullInstallation

插入位置：第 11 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func CompletePullInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_INSTALLED&quot;, task.Operation, task.Operation, &#91;&#93;byte(`{}`), &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;PULL_MAIN_INSTALLED&quot;, task.Operation, task.Operation, &#91;&#93;byte(`{}`), &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>；<code>SetBranchControl(ctx, tx, branch, &quot;&quot;)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code> |
| <code>succeeded</code> | <code>bool</code> | true表示已成功安装对应工作区，将同代次RESTORING支线激活；false不激活。随后清除控制门并登记安装阶段完成，不能用本参数代替Node结果核验。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>succeeded</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27; WHERE id=? AND workspace_epoch=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4665:5288 -->
````go
func CompletePullInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error {
	if succeeded {
		if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch); err != nil {
			return err
		}
	}
	if err := SetBranchControl(ctx, tx, branch, ""); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
````

## 第 13 步：PullInstalled

插入位置：第 12 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func PullInstalled(ctx context.Context, q Queryer, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;PULL_MAIN_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;PULL_MAIN_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;PULL_MAIN_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task WHERE kind=&#x27;PULL_MAIN_INSTALLED&#x27; AND business_key=? AND state=&#x27;SUCCEEDED&#x27;&quot;, operation)</code>。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5288:5651 -->
````go
func PullInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='PULL_MAIN_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

// pull_readiness.go：保留原实现，按业务合并文件。

````

## 第 14 步：PendingPullBinding

插入位置：第 13 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`type PendingPullBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | 持久恢复任务种类，区分计算、安装与核验阶段。 |
| <code>Operation</code> | <code>string</code> | 原业务操作号，恢复继续同一号。 |
| <code>State</code> | <code>string</code> | 与此操作关联的回执状态。 |
| <code>Lifecycle</code> | <code>string</code> | 当前支线生命周期，RESTORING与ACTIVE不能混同。 |
| <code>Actor</code> | <code>uint64</code> | 原回执绑定的操作者。 |
| <code>Epoch</code> | <code>uint64</code> | 当前支线工作区代次。 |
| <code>Lineage</code> | <code>uint64</code> | 当前支线谱系编号。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原上下文任务中保存的载荷；不能由当前请求替换。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前恢复任务的载荷，与原上下文互相核对。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原回执的目标JSON。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 原拉取操作的结果JSON。 |
| <code>Gate</code> | <code>&#91;&#93;byte</code> | 支线控制门载荷，核验门仍属于本次拉取。 |
| <code>Admission</code> | <code>OriginalAdmissionBinding</code> | 使用 <code>OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。 |

<!-- write-step: 14 bytes=5651:5949 -->
````go
type PendingPullBinding struct {
	Kind, Operation, State, Lifecycle                    string
	Actor, Epoch, Lineage                                uint64
	Original, Payload, Fingerprint, Target, Result, Gate []byte
	Admission                                            OriginalAdmissionBinding
}

````

## 第 15 步：PendingPullBindings

插入位置：第 14 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`func PendingPullBindings(ctx context.Context, db *sql.DB) ([]PendingPullBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.kind,t.operation_id,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.request_fingerprint,o.target_json,o.result_json,gate.p …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingPullBinding</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.kind,t.operation_id,COALESCE(o.state,&#x27;&#x27;),COALESCE(o.actor_id,0),  original.payload,t.payload,o.request_fingerprint,o.target_json,o.result_json,gate.p …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;row.Kind, &amp;row.Operation, &amp;row.State, &amp;row.Actor, &amp;row.Original, &amp;row.Payload, &amp;row.Fingerprint, &amp;row.Target, &amp;row.Result, &amp;row.Gate, &amp;row.Lifecycle, &amp;row.Epoch, &amp;row.Li …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5949:7973 -->
````go
func PendingPullBindings(ctx context.Context, db *sql.DB) ([]PendingPullBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.kind,t.operation_id,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.request_fingerprint,o.target_json,o.result_json,gate.payload,
 COALESCE(b.lifecycle,''),COALESCE(b.workspace_epoch,0),COALESCE(b.current_lineage_id,0),
 COALESCE(a.actor_id,0),COALESCE(a.document_id,0),COALESCE(a.unit_id,0),
 COALESCE(a.identity_admission_ref,''),COALESCE(a.operation_id,''),COALESCE(a.purpose,''),
 a.target_json,a.session_ref,COALESCE(CAST(UNIX_TIMESTAMP(a.expires_at)*1000 AS UNSIGNED),0)
 FROM durable_task t
 LEFT JOIN durable_task original ON original.kind='PULL_MAIN_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='PULL_MAIN'
 LEFT JOIN branch b ON b.id=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.context.target.branch_id'))
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=CAST(b.id AS CHAR)
 LEFT JOIN document_admission a ON a.admission_id=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.context.document_admission_ref'))
 WHERE t.kind IN ('PULL_MAIN_RECOVER','PULL_MAIN_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingPullBinding
	for rows.Next() {
		var row PendingPullBinding
		if err = rows.Scan(&row.Kind, &row.Operation, &row.State, &row.Actor, &row.Original, &row.Payload, &row.Fingerprint, &row.Target, &row.Result, &row.Gate, &row.Lifecycle, &row.Epoch, &row.Lineage,
			&row.Admission.Actor, &row.Admission.Document, &row.Admission.Unit,
			&row.Admission.Identity, &row.Admission.Operation, &row.Admission.Purpose,
			&row.Admission.Target, &row.Admission.Session, &row.Admission.SessionExpires); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/pull_main_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
