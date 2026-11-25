# 分步编写 `services/document/internal/repository/document/merges_queries.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/160-merges-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

存储合并尝试、固定输入、冲突、版本清单、接纳lineage和安装恢复任务。

一次业务操作可有多个计算attempt，但正式合并只能有一组权威结果；任务租约必须配合generation防旧执行者写入。

写之前先按这条执行路线理解各段的连接：

1. ReserveMergeAttempt预约
2. FixMergeAttempt固定输入摘要
3. InsertMergeVersion/Manifest/Record追加结果
4. InsertAcceptedLineage记录接纳
5. CompleteMerge与后续CompleteMainInstallation分阶段落定。

本文件需要守住的边界：

- RPC在领取事务提交后进行
- 阅读副本不占用户新增额度但计运营字节
- 旧FIXED记录不得原地修改。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/merges_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/merges_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/merges_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/merges_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:347 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// merge_attempts.go：保留原实现，按业务合并文件。

````

## 第 2 步：MergeBarrier

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type MergeBarrier struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 正在占用此文档主线提交屏障的操作号。 |
| <code>Phase</code> | <code>string</code> | 主线屏障阶段，OPEN以外不能让另一提交任意穿过。 |
| <code>Generation</code> | <code>uint64</code> | 屏障的占用代次；释放时核对原号及代次。 |

<!-- write-step: 2 bytes=347:426 -->
````go
type MergeBarrier struct {
	Operation, Phase string
	Generation       uint64
}
````

## 第 3 步：MergeAttempt

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type MergeAttempt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 一次固定合并输入的尝试主键。 |
| <code>Number</code> | <code>uint64</code> | 同一操作下的尝试序号；主线变化可能产生新的尝试。 |
| <code>Manifest</code> | <code>uint64</code> | 本次尝试固定的主线清单。 |
| <code>Structure</code> | <code>uint64</code> | 本次尝试对应的目录结构。 |
| <code>Style</code> | <code>uint64</code> | 本次尝试对应的样式版本。 |
| <code>Lineage</code> | <code>uint64</code> | 本次尝试依据的支线谱系。 |
| <code>State</code> | <code>string</code> | 尝试从固定到计算、冲突等阶段的状态。 |

<!-- write-step: 3 bytes=426:568 -->
````go
type MergeAttempt struct {
	ID, Number, Manifest, Structure, Style, Lineage uint64
	State                                           string
}

````

## 第 4 步：LockedMergeBarrier

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func LockedMergeBarrier(ctx context.Context, tx *sql.Tx, document uint64) (MergeBarrier, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO main_commit_barrier(document_id,phase) VALUES(?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE document_id=VALUES(document_id)&quot;, document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(operation_id,&#x27;&#x27;),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO main_commit_barrier(document_id,phase) VALUES(?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE document_id=VALUES(document_id)&quot;, document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(operation_id,&#x27;&#x27;),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MergeBarrier</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO main_commit_barrier(document_id,phase) VALUES(?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE document_id=VALUES(document_id)&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(operation_id,&#x27;&#x27;),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE&quot;, document).Scan(&amp;value.Operation, &amp;value.Phase, &amp;value.Generation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(operation_id,&#x27;&#x27;),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE&quot;, document)</code>。

<!-- write-step: 4 bytes=568:1127 -->
````go
func LockedMergeBarrier(ctx context.Context, tx *sql.Tx, document uint64) (MergeBarrier, error) {
	var value MergeBarrier
	if _, err := tx.ExecContext(ctx, "INSERT INTO main_commit_barrier(document_id,phase) VALUES(?,'OPEN') ON DUPLICATE KEY UPDATE document_id=VALUES(document_id)", document); err != nil {
		return value, err
	}
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(operation_id,''),phase,generation FROM main_commit_barrier WHERE document_id=? FOR UPDATE", document).Scan(&value.Operation, &value.Phase, &value.Generation)
	return value, err
}

````

## 第 5 步：LatestMergeAttempt

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func LatestMergeAttempt(ctx context.Context, tx *sql.Tx, operation string) (MergeAttempt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MergeAttempt</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR U …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR UPDATE&quot;, operation)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1127:1589 -->
````go
func LatestMergeAttempt(ctx context.Context, tx *sql.Tx, operation string) (MergeAttempt, error) {
	var value MergeAttempt
	err := tx.QueryRowContext(ctx, "SELECT id,attempt_no,main_manifest_id,structure_id,style_id,lineage_id,state FROM merge_attempt WHERE operation_id=? ORDER BY attempt_no DESC LIMIT 1 FOR UPDATE", operation).Scan(&value.ID, &value.Number, &value.Manifest, &value.Structure, &value.Style, &value.Lineage, &value.State)
	return value, err
}

````

## 第 6 步：ReserveMergeAttempt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func ReserveMergeAttempt(ctx context.Context, tx *sql.Tx, fixed domain.MergeIntent, document, number, manifest, structure, style, lineage, base, generation uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET operation_id=?,generation=?,phase=&#x27;PREPARING&#x27;,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>fixed</code> | <code>domain.MergeIntent</code> | 使用 <code>domain.MergeIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fixed.Operation</code>；<code>fixed.Version</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET operation_id=?,generation=?,phase=&#x27;PREPARING&#x27;,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |
| <code>number</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续）；<code>domain.TextID(number)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续） |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续） |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续） |
| <code>base</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续） |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET operation_id=?,generation=?,phase=&#x27;PREPARING&#x27;,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_dig …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET operation_id=?,generation=?,phase=&#x27;PREPARING&#x27;,expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WH …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1589:2665 -->
````go
func ReserveMergeAttempt(ctx context.Context, tx *sql.Tx, fixed domain.MergeIntent, document, number, manifest, structure, style, lineage, base, generation uint64) (uint64, error) {
	// 先保留真实attempt身份以固定Node采样usage；完整输入摘要稍后仅在同一FIXED阶段登记一次。
	result, err := tx.ExecContext(ctx, "INSERT INTO merge_attempt(operation_id,attempt_no,base_version_id,submitted_version_id,main_manifest_id,main_cut_ref,structure_id,style_id,lineage_id,input_digest,state) VALUES(?,?,?,?,?,? ,?,?,?,?,'FIXED')", fixed.Operation, number, base, fixed.Version, manifest, fixed.Operation+":"+domain.TextID(number), structure, style, lineage, make([]byte, 32))
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE main_commit_barrier SET operation_id=?,generation=?,phase='PREPARING',expected_cut=NULL,prepared_cut=NULL,target_manifest_id=NULL,revision=revision+1 WHERE document_id=?", fixed.Operation, generation, document)
	return uint64(id), err
}

````

## 第 7 步：MergeArtifact

插入位置：第 6 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func MergeArtifact(ctx context.Context, q Queryer, kind, key string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=? AND business_key=?&quot;, kind, key)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=? AND business_key=?&quot;, kind, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=? AND business_key=?&quot;, kind, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>payload</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=? AND business_key=?&quot;, kind, key).Scan(&amp;payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=? AND business_key=?&quot;, kind, key)</code>。
- 此处通过 <code>return payload, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2665:2921 -->
````go
func MergeArtifact(ctx context.Context, q Queryer, kind, key string) ([]byte, error) {
	var payload []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind=? AND business_key=?", kind, key).Scan(&payload)
	return payload, err
}

````

## 第 8 步：SupersedeMergeAttempt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func SupersedeMergeAttempt(ctx context.Context, tx *sql.Tx, operation string, attempt, document, generation uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;SUPERSEDED&#x27; WHERE id=? AND operation_id=? AND state IN (&#x27;FIXED&#x27;,&#x27;COMPUTED&#x27;)&quot;, attempt, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, operation, generation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;SUPERSEDED&#x27; WHERE id=? AND operation_id=? AND state IN (&#x27;FIXED&#x27;,&#x27;COMPUTED&#x27;)&quot;, attempt, operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, operation, generation)</code> |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;SUPERSEDED&#x27; WHERE id=? AND operation_id=? AND state IN (&#x27;FIXED&#x27;,&#x27;COMPUTED&#x27;)&quot;, attempt, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, operation, generation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, operation, generation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;SUPERSEDED&#x27; WHERE id=? AND operation_id=? AND state IN (&#x27;FIXED&#x27;,&#x27;COMPUTED&#x27;)&quot;, attempt, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, operation, generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 8 bytes=2921:3460 -->
````go
func SupersedeMergeAttempt(ctx context.Context, tx *sql.Tx, operation string, attempt, document, generation uint64) error {
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='SUPERSEDED' WHERE id=? AND operation_id=? AND state IN ('FIXED','COMPUTED')", attempt, operation); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='OPEN',operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", document, operation, generation)
	return err
}

````

## 第 9 步：StoreMergeConflict

插入位置：第 8 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func StoreMergeConflict(ctx context.Context, tx *sql.Tx, attempt uint64, conflicts []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;CONFLICT&#x27;,conflict_json=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, string(conflicts), attempt)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;CONFLICT&#x27;,conflict_json=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, string(conflicts), attempt)</code> |
| <code>conflicts</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(conflicts)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;CONFLICT&#x27;,conflict_json=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, string(conflicts), attempt)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3460:3723 -->
````go
func StoreMergeConflict(ctx context.Context, tx *sql.Tx, attempt uint64, conflicts []byte) error {
	_, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='CONFLICT',conflict_json=? WHERE id=? AND state='FIXED'", string(conflicts), attempt)
	return err
}

````

## 第 10 步：FixMergeAttempt

插入位置：第 9 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func FixMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, digest, payload []byte, key, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, digest, id)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, digest, id)</code> |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, digest, id)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27;&quot;, digest, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3723:4093 -->
````go
func FixMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, digest, payload []byte, key, operation string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET input_digest=? WHERE id=? AND state='FIXED'", digest, id); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_FIXED", key, operation, payload, "SUCCEEDED")
}

````

## 第 11 步：FixProjectedMergeAttempt

插入位置：第 10 步代码之后，紧接追加；完成稿约第 79 行。

只追加旧生产者混结构输入的派生版本；原FIXED记录不改，准确旧摘要与状态隔离其他计算。

本步声明：`func FixProjectedMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, original, digest, payload []byte, key, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27; AND input_digest=?&quot;, digest, id, original)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27; AND input_digest=?&quot;, digest, id, original)</code> |
| <code>original</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27; AND input_digest=?&quot;, digest, id, original)</code> |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27; AND input_digest=?&quot;, digest, id, original)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_RANGE_BINDING_UNKNOWN&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET input_digest=? WHERE id=? AND state=&#x27;FIXED&#x27; AND input_digest=?&quot;, digest, id, original)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_RANGE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key, operation, payload, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4093:4895 -->
````go
// 只追加旧生产者混结构输入的派生版本；原FIXED记录不改，准确旧摘要与状态隔离其他计算。
func FixProjectedMergeAttempt(ctx context.Context, tx *sql.Tx, id uint64, original, digest, payload []byte, key, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET input_digest=? WHERE id=? AND state='FIXED' AND input_digest=?", digest, id, original)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_BINDING_UNKNOWN")
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key, operation, payload, "SUCCEEDED")
}

// merge_commit.go：保留原实现，按业务合并文件。

````

## 第 12 步：CompleteMainInstallation

插入位置：第 11 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func CompleteMainInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation uint64, reply *pb.InstallReply) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, task.Operation, generation)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_INSTALL_COMPLETED&quot;, task.Operation, task.Operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_INSTALL_COMPLETED&quot;, task.Operation, task.Operation, raw, &quot;SUCCEEDED&quot;)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, task.Operation, generation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, task.Operation, generation)</code> |
| <code>reply</code> | <code>*pb.InstallReply</code> | 在此声明中的实际使用：<code>reply.State</code>；<code>reply.ManifestId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;OPEN&#x27;,operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, document, task.Operation, generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>reply.State == pb.InstallReply_READY</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4895:5670 -->
````go
func CompleteMainInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, document, generation uint64, reply *pb.InstallReply) error {
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='OPEN',operation_id=NULL,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", document, task.Operation, generation); err != nil {
		return err
	}
	if reply.State == pb.InstallReply_READY {
		raw, err := json.Marshal(map[string]string{"manifest_id": domain.TextID(reply.ManifestId)})
		if err != nil {
			return err
		}
		if err = InsertSubmissionTask(ctx, tx, "MAIN_INSTALL_COMPLETED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}

````

## 第 13 步：InsertMergeVersion

插入位置：第 12 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func InsertMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> |
| <code>ref</code> | <code>string</code> | 在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>insertFixedVersion(ctx, tx, content, kind, ref, true)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return insertFixedVersion(ctx, tx, content, kind, ref, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5670:5854 -->
````go
func InsertMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string) (uint64, error) {
	return insertFixedVersion(ctx, tx, content, kind, ref, true)
}

````

## 第 14 步：InsertReadingVersion

插入位置：第 13 步代码之后，紧接追加；完成稿约第 117 行。

阅读副本保留可靠正文及资源引用，但不因 GET 请求消耗用户新增空间。
实际字节在独立运营账中登记，后续清理与转让只能统计可计费业务版本。

本步声明：`func InsertReadingVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code>；<code>content.CaptureId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return insertFixedVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5854:6249 -->
````go
// 阅读副本保留可靠正文及资源引用，但不因 GET 请求消耗用户新增空间。
// 实际字节在独立运营账中登记，后续清理与转让只能统计可计费业务版本。
func InsertReadingVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent) (uint64, error) {
	return insertFixedVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId, false)
}

````

## 第 15 步：insertFixedVersion

插入位置：第 14 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`func insertFixedVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string, billable bool) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockDocumentCapacity(ctx, tx, content.DocumentId)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续）；<code>ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)</code>；<code>tx.ExecContext(ctx, &quot;INSERT INTO reading_body_capacity(version_id,document_id,bytes) VALUES(?,?,?)&quot;, id, content.DocumentId, content.ByteSize)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockDocumentCapacity(ctx, tx, content.DocumentId)</code>；<code>tx.ExecContext</code>；<code>ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)</code>；<code>InsertSubmissionResource(ctx, tx, resource, content.DocumentId, uint64(id))</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.StructureId</code>；<code>content.BodyJsonUtf8</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续） |
| <code>ref</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续） |
| <code>billable</code> | <code>bool</code> | true时先锁文档容量并记账此固定版本；false登记reading_body_capacity，区分读取派生副本的容量。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>billable</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operat …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>billable</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO reading_body_capacity(version_id,document_id,bytes) VALUES(?,?,?)&quot;, id, content.DocumentId, content.ByteSize)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6249:7712 -->
````go
func insertFixedVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, kind, ref string, billable bool) (uint64, error) {
	if billable {
		if err := LockDocumentCapacity(ctx, tx, content.DocumentId); err != nil {
			return 0, err
		}
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO content_version(document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", content.DocumentId, content.UnitId, content.StructureId, string(content.BodyJsonUtf8), content.BodyJsonUtf8, content.BodyHash, content.SchemaVersion, content.CanonicalizationVersion, content.ByteSize, content.OperationId, kind, ref, string(content.ProvenanceJsonUtf8), content.ProvenanceSchema)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if billable {
		err = ChargeStoredVersion(ctx, tx, content.DocumentId, content.ByteSize, content.OperationId)
	} else {
		_, err = tx.ExecContext(ctx, "INSERT INTO reading_body_capacity(version_id,document_id,bytes) VALUES(?,?,?)", id, content.DocumentId, content.ByteSize)
	}
	if err != nil {
		return 0, err
	}
	for _, resource := range content.ResourceIds {
		if err = InsertSubmissionResource(ctx, tx, resource, content.DocumentId, uint64(id)); err != nil {
			return 0, err
		}
	}
	return uint64(id), nil
}

````

## 第 16 步：InsertMergedManifest

插入位置：第 15 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func InsertMergedManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, cutRef string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>parent</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |
| <code>cutRef</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, document,  …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, &#x27;MERGE&#x27;,?)&quot;, doc …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7712:8094 -->
````go
func InsertMergedManifest(ctx context.Context, tx *sql.Tx, document, parent, structure, style uint64, operation, cutRef string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,parent_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,?, 'MERGE',?)", document, parent, structure, style, operation, cutRef)
}

````

## 第 17 步：InsertMergeRecord

插入位置：第 16 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func InsertMergeRecord(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, attempt, input, output, version, lineage uint64, capture string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Round</code> |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>input</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>output</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |
| <code>capture</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALU …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_i …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=8094:8585 -->
````go
func InsertMergeRecord(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, attempt, input, output, version, lineage uint64, capture string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO merge_record(operation_id,request_id,round_id,attempt_id,input_manifest_id,output_manifest_id,result_version_id,accepted_capture_ref,lineage_id) VALUES(?,?,?,?,?,?,?,?,?)", operation, pointer.ID, pointer.Round, attempt, input, output, version, capture, lineage)
}

````

## 第 18 步：InsertMergeApproval

插入位置：第 17 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`func InsertMergeApproval(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, submitted, record uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE merge_record SET approval_action_id=? WHERE id=?&quot;, action, record)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续） |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Round</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续） |
| <code>submitted</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续） |
| <code>record</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,&#x27;APPROVE_AND_MERGE&#x27;,?,0,?,?)&quot;,  …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE merge_record SET approval_action_id=? WHERE id=?&quot;, action, record)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>action</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_record SET approval_action_id=? WHERE id=?&quot;, action, record)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return action, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8585:9284 -->
````go
func InsertMergeApproval(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, submitted, record uint64) (uint64, error) {
	// 外键环按record(approval NULL)→action(record)→record.approval提交；只更新授权允许的指针列。
	action, err := InsertID(ctx, tx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,content_version_id,merge_record_id) VALUES(?,?,?,'APPROVE_AND_MERGE',?,0,?,?)", pointer.ID, pointer.Round, actor, operation, submitted, record)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE merge_record SET approval_action_id=? WHERE id=?", action, record)
	return action, err
}

````

## 第 19 步：InsertMergedResolution

插入位置：第 18 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func InsertMergedResolution(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, change uint64, item int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续）；<code>ResolveChange(ctx, tx, change, action)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续）；<code>ResolveChange(ctx, tx, change, action)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续） |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Round</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续） |
| <code>change</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续）；<code>ResolveChange(ctx, tx, change, action)</code> |
| <code>item</code> | <code>int</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,&#x27;RESOLVE_CHANGES&#x27;,?,?,?,?)&quot;, pointer.ID, p …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ResolveChange(ctx, tx, change, action)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return ResolveChange(ctx, tx, change, action)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=9284:9840 -->
````go
func InsertMergedResolution(ctx context.Context, tx *sql.Tx, operation string, pointer domain.ReviewPointer, actor, change uint64, item int) error {
	reason := "在通过并合并本轮时明确解除本人修改要求。"
	action, err := InsertID(ctx, tx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,target_change_id) VALUES(?,?,?,'RESOLVE_CHANGES',?,?,?,?)", pointer.ID, pointer.Round, actor, operation, item, reason, change)
	if err != nil {
		return err
	}
	return ResolveChange(ctx, tx, change, action)
}

````

## 第 20 步：InsertAcceptedLineage

插入位置：第 19 步代码之后，紧接追加；完成稿约第 180 行。

本步声明：`func InsertAcceptedLineage(ctx context.Context, tx *sql.Tx, id uint64, pointer domain.ReviewPointer, previous, manifest, submitted uint64, operation, capture string, provenance []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,revision=revision+1 WHERE id=?&quot;, id, pointer.Branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,revision=revision+1 WHERE id=?&quot;, id, pointer.Branch)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Branch</code>；<code>pointer.Unit</code>；<code>pointer.Document</code> |
| <code>previous</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>submitted</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>capture</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续） |
| <code>provenance</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(provenance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_ …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,revision=revision+1 WHERE id=?&quot;, id, pointer.Branch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 20 bytes=9840:10604 -->
````go
func InsertAcceptedLineage(ctx context.Context, tx *sql.Tx, id uint64, pointer domain.ReviewPointer, previous, manifest, submitted uint64, operation, capture string, provenance []byte) error {
	if _, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'ACCEPTED_CUT',?,?,?,?,?,'provenance-v1')", id, pointer.Branch, pointer.Unit, pointer.Document, previous, operation, manifest, submitted, capture, string(provenance)); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,revision=revision+1 WHERE id=?", id, pointer.Branch)
	return err
}

````

## 第 21 步：CompleteMerge

插入位置：第 20 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`func CompleteMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, pointer domain.ReviewPointer, actor, record, manifest, attempt, generation uint64, result map[string]any, install []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, manifest, pointer.Document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;MERGED&#x27;,active_merge_operation_id=NULL,revision=revision+1 WHERE id=?&quot;, pointer.ID)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND ongoing_request_id=?&quot;, pointer.Unit, pointer.ID)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;SUCCEEDED&#x27;,result_merge_id=? WHERE operation_id=?&quot;, record, task.Operation)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;SUCCEEDED&quot;, raw)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_COMMIT_FIXED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Document</code>；<code>pointer.ID</code>；<code>pointer.Unit</code>；<code>pointer.Round</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(actor)</code> |
| <code>record</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;SUCCEEDED&#x27;,result_merge_id=? WHERE operation_id=?&quot;, record, task.Operation)</code>；<code>domain.TextID(record)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, manifest, pointer.Document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest,  …</code>（长表达式，按本步完整代码继续） |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;COMMITTED&#x27; WHERE id=?&quot;, attempt)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest,  …</code>（长表达式，按本步完整代码继续） |
| <code>result</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>json.Marshal(result)</code> |
| <code>install</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_COMMIT_FIXED&quot;, task.Operation, task.Operation, install, &quot;SUCCEEDED&quot;)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MAIN_RECONCILE&quot;, task.Operation, task.Operation, install, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET current_manifest_id=?,control_revision=control_revision+1 WHERE id=?&quot;, manifest, pointer.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;MERGED&#x27;,active_merge_operation_id=NULL,revision=revision+1 WHERE id=?&quot;, pointer.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND ongoing_request_id=?&quot;, pointer.Unit, pointer.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;SUCCEEDED&#x27;,result_merge_id=? WHERE operation_id=?&quot;, record, task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_attempt SET state=&#x27;COMMITTED&#x27; WHERE id=?&quot;, attempt)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE main_commit_barrier SET phase=&#x27;INSTALLING&#x27;,target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?&quot;, manifest,  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=10604:13008 -->
````go
func CompleteMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, pointer domain.ReviewPointer, actor, record, manifest, attempt, generation uint64, result map[string]any, install []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document SET current_manifest_id=?,control_revision=control_revision+1 WHERE id=?", manifest, pointer.Document); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='MERGED',active_merge_operation_id=NULL,revision=revision+1 WHERE id=?", pointer.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=NULL,generation=generation+1 WHERE unit_id=? AND ongoing_request_id=?", pointer.Unit, pointer.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_operation SET state='SUCCEEDED',result_merge_id=? WHERE operation_id=?", record, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_attempt SET state='COMMITTED' WHERE id=?", attempt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE main_commit_barrier SET phase='INSTALLING',target_manifest_id=?,revision=revision+1 WHERE document_id=? AND operation_id=? AND generation=?", manifest, pointer.Document, task.Operation, generation); err != nil {
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
	if err = InsertSubmissionTask(ctx, tx, "MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	notice, err := json.Marshal(map[string]string{"event": "APPROVE_AND_MERGE", "document": domain.TextID(pointer.Document), "request": domain.TextID(pointer.ID), "round": domain.TextID(pointer.Round), "record": domain.TextID(record), "actor": domain.TextID(actor)})
	if err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, "MERGE_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}

// merge_inputs.go：保留原实现，按业务合并文件。

````

## 第 22 步：MergeRoundInput

插入位置：第 21 步代码之后，紧接追加；完成稿约第 232 行。

本步声明：`type MergeRoundInput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>BaseManifest</code> | <code>uint64</code> | 审核轮次固定的共同基线清单。 |
| <code>BaseVersion</code> | <code>uint64</code> | 审核轮次的共同基线正文。 |
| <code>SubmittedVersion</code> | <code>uint64</code> | 用户提交审核的固定正文版本。 |
| <code>Structure</code> | <code>uint64</code> | 提交时绑定的结构版本。 |
| <code>Lineage</code> | <code>uint64</code> | 提交来源支线谱系。 |
| <code>Capture</code> | <code>string</code> | 提交正文的固定采样引用。 |
| <code>Submission</code> | <code>string</code> | 产生此轮次的提交操作号。 |

<!-- write-step: 22 bytes=13008:13257 -->
````go
type MergeRoundInput struct {
	BaseManifest, BaseVersion, SubmittedVersion, Structure, Lineage uint64
	Capture                                                         string
	Submission                                                      string
}

````

## 第 23 步：MergeRoundInputs

插入位置：第 22 步代码之后，紧接追加；完成稿约第 238 行。

本步声明：`func MergeRoundInputs(ctx context.Context, q Queryer, request, round uint64) (MergeRoundInput, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MergeRoundInput</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND requ …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=13257:13765 -->
````go
func MergeRoundInputs(ctx context.Context, q Queryer, request, round uint64) (MergeRoundInput, error) {
	var value MergeRoundInput
	err := q.QueryRowContext(ctx, "SELECT base_manifest_id,base_version_id,content_version_id,structure_id,lineage_id,capture_ref,submission_operation_id FROM review_round WHERE id=? AND request_id=?", round, request).Scan(&value.BaseManifest, &value.BaseVersion, &value.SubmittedVersion, &value.Structure, &value.Lineage, &value.Capture, &value.Submission)
	return value, err
}

````

## 第 24 步：VersionContent

插入位置：第 23 步代码之后，紧接追加；完成稿约第 244 行。

本步声明：`func VersionContent(ctx context.Context, tx Queryer, version, document uint64) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenanc …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=CAST(? AS CHAR) AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, version)</code> |
| <code>tx</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.QueryContext</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenanc …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=CAST(? AS CHAR) AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, version)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenanc …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=CAST(? AS CHAR) AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>content</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenanc …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenanc …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=CAST(? AS CHAR) AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, version)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;resource)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return content, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=13765:15106 -->
````go
func VersionContent(ctx context.Context, tx Queryer, version, document uint64) (*pb.FixedContent, error) {
	content := &pb.FixedContent{HashAlgorithm: "SHA-256", ResourceIds: []uint64{}}
	err := tx.QueryRowContext(ctx, "SELECT document_id,unit_id,structure_id,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_ref,provenance_json,provenance_schema FROM content_version WHERE id=? AND document_id=?", version, document).Scan(&content.DocumentId, &content.UnitId, &content.StructureId, &content.BodyJsonUtf8, &content.BodyHash, &content.SchemaVersion, &content.CanonicalizationVersion, &content.ByteSize, &content.OperationId, &content.CaptureId, &content.ProvenanceJsonUtf8, &content.ProvenanceSchema)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT resource_id FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=CAST(? AS CHAR) AND purpose='CONTENT' ORDER BY resource_id", document, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var resource uint64
		if err = rows.Scan(&resource); err != nil {
			return nil, err
		}
		content.ResourceIds = append(content.ResourceIds, resource)
	}
	return content, rows.Err()
}

// merge_readiness.go：保留原实现，按业务合并文件。

````

## 第 25 步：PendingMergeBinding

插入位置：第 24 步代码之后，紧接追加；完成稿约第 267 行。

本步声明：`type PendingMergeBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | 合并恢复任务种类。 |
| <code>Operation</code> | <code>string</code> | 原合并操作号。 |
| <code>ReceiptState</code> | <code>string</code> | 原操作回执所处状态，决定是返回既有结果、继续原操作还是等待核验；未知状态不能伪报成功。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 当前任务恢复载荷。 |
| <code>Original</code> | <code>&#91;&#93;byte</code> | 原合并上下文载荷。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>ReceiptActor</code> | <code>uint64</code> | 原操作回执中的操作者，不能只信任务内的actor。 |
| <code>Record</code> | <code>uint64</code> | 已生成的合并记录编号。 |
| <code>Manifest</code> | <code>uint64</code> | 已提交合并结果的输出清单编号。 |
| <code>Fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。 |

<!-- write-step: 25 bytes=15106:15309 -->
````go
type PendingMergeBinding struct {
	Kind, Operation, ReceiptState  string
	Payload, Original, Fingerprint []byte
	ReceiptActor, Record, Manifest uint64
	Fact                           domain.MergeFact
}

````

## 第 26 步：PendingMergeBindings

插入位置：第 25 步代码之后，紧接追加；完成稿约第 274 行。

本步声明：`func PendingMergeBindings(ctx context.Context, db *sql.DB) ([]PendingMergeBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, ` 		SELECT t.kind,t.operation_id,t.payload,COALESCE(o.state,&#x27;&#x27;), 		       COALESCE(m.actor_id,0),COALESCE(m.request_id,0),COALESCE(m.round_id,0), 		       COAL …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingMergeBinding</code> | 实际返回表达式：<code>nil</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, ` 		SELECT t.kind,t.operation_id,t.payload,COALESCE(o.state,&#x27;&#x27;), 		       COALESCE(m.actor_id,0),COALESCE(m.request_id,0),COALESCE(m.round_id,0), 		       COAL …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.Kind, &amp;value.Operation, &amp;value.Payload, &amp;value.ReceiptState, &amp;f.Actor, &amp;f.Request, &amp;f.Round, &amp;f.Version, &amp;f.State, &amp;f.Document, &amp;f.Unit, &amp;f.Branch, &amp;value.Original …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return facts, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=15309:17135 -->
````go
func PendingMergeBindings(ctx context.Context, db *sql.DB) ([]PendingMergeBinding, error) {
	// 同时读取原恢复上下文和权威receipt；单凭任务自己的payload不能证明它属于这个操作。
	rows, err := db.QueryContext(ctx, `
		SELECT t.kind,t.operation_id,t.payload,COALESCE(o.state,''),
		       COALESCE(m.actor_id,0),COALESCE(m.request_id,0),COALESCE(m.round_id,0),
		       COALESCE(m.content_version_id,0),COALESCE(m.state,''),
		       COALESCE(r.document_id,0),COALESCE(r.unit_id,0),COALESCE(r.branch_id,0),
		       original.payload,o.request_fingerprint,COALESCE(o.actor_id,0),
		       COALESCE(m.result_merge_id,0),COALESCE(record.output_manifest_id,0)
		FROM durable_task t
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='MERGE'
		LEFT JOIN review_request r ON r.id=m.request_id
		LEFT JOIN durable_task original ON original.kind='MERGE_RECOVER'
		     AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
		LEFT JOIN merge_record record ON record.id=m.result_merge_id AND record.request_id=m.request_id
		WHERE t.kind IN ('MERGE_RECOVER','MAIN_RECONCILE','MERGE_NOTIFICATION')
		  AND t.state NOT IN ('SUCCEEDED','REJECTED')
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingMergeBinding
	for rows.Next() {
		var value PendingMergeBinding
		f := &value.Fact
		if err = rows.Scan(&value.Kind, &value.Operation, &value.Payload, &value.ReceiptState, &f.Actor, &f.Request, &f.Round, &f.Version, &f.State, &f.Document, &f.Unit, &f.Branch, &value.Original, &value.Fingerprint, &value.ReceiptActor, &value.Record, &value.Manifest); err != nil {
			return nil, err
		}
		facts = append(facts, value)
	}
	return facts, rows.Err()
}

````

## 第 27 步：MainInstallationCompleted

插入位置：第 26 步代码之后，紧接追加；完成稿约第 309 行。

本步声明：`func MainInstallationCompleted(ctx context.Context, q Queryer, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind=&#x27;MAIN_INSTALL_COMPLETED&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;)&quot;, operation, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind=&#x27;MAIN_INSTALL_COMPLETED&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;)&quot;, operation, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>ready</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind=&#x27;MAIN_INSTALL_COMPLETED&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;)&quot;, operation, operation).Scan(&amp;ready)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind=&#x27;MAIN_INSTALL_COMPLETED&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;)&quot;, operation, operation)</code>。
- 此处通过 <code>return ready, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=17135:17541 -->
````go
func MainInstallationCompleted(ctx context.Context, q Queryer, operation string) (bool, error) {
	var ready bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM durable_task WHERE kind='MAIN_INSTALL_COMPLETED' AND business_key=? AND operation_id=? AND state='SUCCEEDED')", operation, operation).Scan(&ready)
	return ready, err
}

// merge_tasks.go：保留原实现，按业务合并文件。

````

## 第 28 步：ClaimMergeTask

插入位置：第 27 步代码之后，紧接追加；完成稿约第 317 行。

租约只授予这次执行资格；所有跨服务步骤在领取事务提交后进行，终态仍检查generation/instance。

本步声明：`func ClaimMergeTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return claimOperationTask(ctx, tx, instance, &quot;MERGE_RECOVER&quot;, &quot;MAIN_RECONCILE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=17541:17859 -->
````go
// 租约只授予这次执行资格；所有跨服务步骤在领取事务提交后进行，终态仍检查generation/instance。
func ClaimMergeTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "MERGE_RECOVER", "MAIN_RECONCILE")
}

````

## 第 29 步：claimOperationTask

插入位置：第 28 步代码之后，紧接追加；完成稿约第 322 行。

两种已装配的任务分别领取，种类是内部常量且使用SQL参数，控制任务不会冒用MERGE预约。

本步声明：`func claimOperationTask(ctx context.Context, tx *sql.Tx, instance, preparation, reconciliation string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, preparation, reconciliation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, preparation, reconciliation)</code>；<code>tx.ExecContext</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>task.Instance = instance</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code> |
| <code>preparation</code> | <code>string</code> | 在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, preparation, reconciliation)</code> |
| <code>reconciliation</code> | <code>string</code> | 在此声明中的实际使用：<code>lockClaimableOperationTask(ctx, tx, preparation, reconciliation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=attempts+1 WHERE id=?&quot;, task.Generation, instance, task.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 29 bytes=17859:18527 -->
````go
// 两种已装配的任务分别领取，种类是内部常量且使用SQL参数，控制任务不会冒用MERGE预约。
func claimOperationTask(ctx context.Context, tx *sql.Tx, instance, preparation, reconciliation string) (domain.SubmissionTask, error) {
	task, err := lockClaimableOperationTask(ctx, tx, preparation, reconciliation)
	if err != nil {
		return task, err
	}
	task.Generation++
	task.Instance = instance
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',generation=?,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=attempts+1 WHERE id=?", task.Generation, instance, task.ID)
	return task, err
}

````

## 第 30 步：lockClaimableOperationTask

插入位置：第 29 步代码之后，紧接追加；完成稿约第 334 行。

本步声明：`func lockClaimableOperationTask(ctx context.Context, tx *sql.Tx, preparation, reconciliation string) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> |
| <code>preparation</code> | <code>string</code> | 在此声明中的实际使用：<code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> |
| <code>reconciliation</code> | <code>string</code> | 在此声明中的实际使用：<code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=18527:18740 -->
````go
func lockClaimableOperationTask(ctx context.Context, tx *sql.Tx, preparation, reconciliation string) (domain.SubmissionTask, error) {
	return lockDueTask(ctx, tx, operationTaskDue, preparation, reconciliation)
}

````

## 第 31 步：operationTaskDue

插入位置：第 30 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`const operationTaskDue = "kind IN (?,?) AND retry_at<=UTC_TIMESTAMP(6) AND (state IN ('QUEUED','RETRY') OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"`。

<!-- write-step: 31 bytes=18740:18900 -->
````go
const operationTaskDue = "kind IN (?,?) AND retry_at<=UTC_TIMESTAMP(6) AND (state IN ('QUEUED','RETRY') OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"
````

## 第 32 步：authorizationTaskDue

插入位置：第 31 步代码之后，紧接追加；完成稿约第 339 行。

本步声明：`const authorizationTaskDue = "kind='AUTHORIZATION_CHANGE' AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"`。

<!-- write-step: 32 bytes=18900:19091 -->
````go
const authorizationTaskDue = "kind='AUTHORIZATION_CHANGE' AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<UTC_TIMESTAMP(6)))"

````

## 第 33 步：lockDueTask

插入位置：第 32 步代码之后，紧接追加；完成稿约第 341 行。

predicate只接受上面的内部常量；业务值始终通过参数传入。

本步声明：`func lockDueTask(ctx context.Context, tx *sql.Tx, predicate string, args ...any) (domain.SubmissionTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM durable_task WHERE &quot;+predicate+&quot; ORDER BY id LIMIT 32&quot;, args...)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.QueryRowContext</code> |
| <code>predicate</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;SELECT id FROM durable_task WHERE &quot;+predicate</code>；<code>&quot;SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND (&quot;+predicate</code> |
| <code>args</code> | <code>...any</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM durable_task WHERE &quot;+predicate+&quot; ORDER BY id LIMIT 32&quot;, args...)</code>；<code>append(&#91;&#93;any{id}, args...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>domain.SubmissionTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>sql.ErrNoRows</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id FROM durable_task WHERE &quot;+predicate+&quot; ORDER BY id LIMIT 32&quot;, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.SubmissionTask{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.SubmissionTask{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...).Scan(&amp;task.ID, &amp;task.Kind, &amp;task.Operation, &amp;task.Generation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND (&quot;+predicate+&quot;) FOR UPDATE SKIP LOCKED&quot;, lockedArgs...)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return task, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE id=?&quot;, id).Scan(&amp;task.Payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return domain.SubmissionTask{}, sql.ErrNoRows</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=19091:20834 -->
````go
// predicate只接受上面的内部常量；业务值始终通过参数传入。
func lockDueTask(ctx context.Context, tx *sql.Tx, predicate string, args ...any) (domain.SubmissionTask, error) {
	// 002的kind/state覆盖索引先排除其他业务和已完成历史；旧库仍可选原状态索引。
	// 此步不加锁：RR范围锁会把OUTPUT_LEASE等无关种类也挡住。
	// 只排小ID，避免把多MiB JSON放进filesort；候选窗口允许跳过其他执行器正在持有的行。
	rows, err := tx.QueryContext(ctx, "SELECT id FROM durable_task WHERE "+predicate+" ORDER BY id LIMIT 32", args...)
	if err != nil {
		return domain.SubmissionTask{}, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return domain.SubmissionTask{}, err
	}
	for _, id := range ids {
		var task domain.SubmissionTask
		lockedArgs := append([]any{id}, args...)
		// 精确主键锁的当前读重新检查种类、状态和到期时间；候选快照本身不授予领取资格。
		err = tx.QueryRowContext(ctx, "SELECT id,kind,operation_id,generation FROM durable_task WHERE id=? AND ("+predicate+") FOR UPDATE SKIP LOCKED", lockedArgs...).Scan(&task.ID, &task.Kind, &task.Operation, &task.Generation)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return task, err
		}
		// 代次更新、原payload读取与提交共用这份行锁和事务；不能换连接读取正文。
		err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE id=?", id).Scan(&task.Payload)
		return task, err
	}
	return domain.SubmissionTask{}, sql.ErrNoRows
}

````

## 第 34 步：ValidateMergeTask

插入位置：第 33 步代码之后，紧接追加；完成稿约第 383 行。

本步声明：`func ValidateMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.Instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_WORKER_SUPERSEDED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID).Scan(&amp;generation, &amp;state, &amp;instance)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT generation,state,COALESCE(instance_id,&#x27;&#x27;) FROM durable_task WHERE id=? FOR UPDATE&quot;, task.ID)</code>。
- 检查 <code>generation != task.Generation &#124;&#124; state != &quot;RUNNING&quot; &#124;&#124; instance != task.Instance</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_WORKER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=20834:21369 -->
````go
func ValidateMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask) error {
	var generation uint64
	var state, instance string
	if err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? FOR UPDATE", task.ID).Scan(&generation, &state, &instance); err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || instance != task.Instance {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_WORKER_SUPERSEDED")
	}
	return nil
}

````

## 第 35 步：FinishMergeTask

插入位置：第 34 步代码之后，紧接追加；完成稿约第 395 行。

本步声明：`func FinishMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error`。


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

<!-- write-step: 35 bytes=21369:21685 -->
````go
func FinishMergeTask(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state=?,lease_until=NULL WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", state, task.ID, task.Generation, task.Instance)
	return err
}

````

## 第 36 步：RetryMergeTask

插入位置：第 35 步代码之后，紧接追加；完成稿约第 400 行。

本步声明：`func RetryMergeTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error`。


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

<!-- write-step: 36 bytes=21685:22071 -->
````go
func RetryMergeTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", code, task.ID, task.Generation, task.Instance)
	return err
}

````

## 第 37 步：RejectMerge

插入位置：第 36 步代码之后，紧接追加；完成稿约第 405 行。

本步声明：`func RejectMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, request uint64, code string, result map[string]any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=NULL,revision=revision+1 WHERE id=? AND active_merge_operation_id=?&quot;, request, task.Operation)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;,&#x27;CANCELLED&#x27;)&quot;, code, task.Operation)</code>；<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;REJECTED&quot;, raw)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>UpdateOperationResult(ctx, tx, task.Operation, &quot;REJECTED&quot;, raw)</code>；<code>FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=NULL,revision=revision+1 WHERE id=? AND active_merge_operation_id=?&quot;, request, task.Operation)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;,&#x27;CANCELLED&#x27;)&quot;, code, task.Operation)</code> |
| <code>result</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>json.Marshal(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=NULL,revision=revision+1 WHERE id=? AND active_merge_operation_id=?&quot;, request, task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE merge_operation SET state=&#x27;REJECTED&#x27;,last_error=? WHERE operation_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;,&#x27;CANCELLED&#x27;)&quot;, code, task.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=22071:22926 -->
````go
func RejectMerge(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, request uint64, code string, result map[string]any) error {
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET active_merge_operation_id=NULL,revision=revision+1 WHERE id=? AND active_merge_operation_id=?", request, task.Operation); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_operation SET state='REJECTED',last_error=? WHERE operation_id=? AND state NOT IN ('SUCCEEDED','REJECTED','CANCELLED')", code, task.Operation); err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", raw); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "REJECTED")
}

// merges.go：保留原实现，按业务合并文件。

````

## 第 38 步：MergeRow

插入位置：第 37 步代码之后，紧接追加；完成稿约第 424 行。

本步声明：`func MergeRow(ctx context.Context, q Queryer, operation string) (domain.MergeFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation  …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation  …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=22926:23486 -->
````go
func MergeRow(ctx context.Context, q Queryer, operation string) (domain.MergeFact, error) {
	var fact domain.MergeFact
	err := q.QueryRowContext(ctx, "SELECT m.operation_id,m.actor_id,m.request_id,m.round_id,m.content_version_id,m.state,m.generation,r.document_id,r.unit_id,r.branch_id FROM merge_operation m JOIN review_request r ON r.id=m.request_id WHERE m.operation_id=?", operation).Scan(&fact.Operation, &fact.Actor, &fact.Request, &fact.Round, &fact.Version, &fact.State, &fact.Generation, &fact.Document, &fact.Unit, &fact.Branch)
	return fact, err
}

````

## 第 39 步：LockedActiveMerge

插入位置：第 38 步代码之后，紧接追加；完成稿约第 430 行。

本步声明：`func LockedActiveMerge(ctx context.Context, tx *sql.Tx, request uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(active_merge_operation_id,&#x27;&#x27;) FROM review_request WHERE id=? FOR UPDATE&quot;, request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(active_merge_operation_id,&#x27;&#x27;) FROM review_request WHERE id=? FOR UPDATE&quot;, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>operation</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(active_merge_operation_id,&#x27;&#x27;) FROM review_request WHERE id=? FOR UPDATE&quot;, request).Scan(&amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(active_merge_operation_id,&#x27;&#x27;) FROM review_request WHERE id=? FOR UPDATE&quot;, request)</code>。
- 此处通过 <code>return operation, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=23486:23773 -->
````go
func LockedActiveMerge(ctx context.Context, tx *sql.Tx, request uint64) (string, error) {
	var operation string
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(active_merge_operation_id,'') FROM review_request WHERE id=? FOR UPDATE", request).Scan(&operation)
	return operation, err
}

````

## 第 40 步：LockedReviewChanges

插入位置：第 39 步代码之后，紧接追加；完成稿约第 436 行。

本步声明：`func LockedReviewChanges(ctx context.Context, tx *sql.Tx, request uint64) ([]domain.ChangeFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE request_id=? ORDER BY id FOR UPDATE&quot;, request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE request_id=? ORDER BY id FOR UPDATE&quot;, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.ChangeFact</code> | 使用 <code>domain.ChangeFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>nil</code>；<code>changes</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE request_id=? ORDER BY id FOR UPDATE&quot;, request)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;change.ID, &amp;change.Request, &amp;change.Round, &amp;change.Author, &amp;change.Revision, &amp;change.Status)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return changes, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=23773:24414 -->
````go
func LockedReviewChanges(ctx context.Context, tx *sql.Tx, request uint64) ([]domain.ChangeFact, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE request_id=? ORDER BY id FOR UPDATE", request)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var changes []domain.ChangeFact
	for rows.Next() {
		var change domain.ChangeFact
		if err := rows.Scan(&change.ID, &change.Request, &change.Round, &change.Author, &change.Revision, &change.Status); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

````

## 第 41 步：RegisterMerge

插入位置：第 40 步代码之后，紧接追加；完成稿约第 453 行。

本步声明：`func RegisterMerge(ctx context.Context, tx *sql.Tx, actor, request uint64, intent domain.MergeIntent, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,&#x27;PREPARING&#x27;, …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=?,revision=revision+1 WHERE id=?&quot;, intent.Operation, request)</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,&#x27;PREPARING&#x27;, …</code>（长表达式，按本步完整代码继续） |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,&#x27;PREPARING&#x27;, …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=?,revision=revision+1 WHERE id=?&quot;, intent.Operation, request)</code> |
| <code>intent</code> | <code>domain.MergeIntent</code> | 使用 <code>domain.MergeIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>intent.Resolutions</code>；<code>intent.Operation</code>；<code>intent.Round</code>；<code>intent.Version</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;MERGE_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,&#x27;PREPARING&#x27;, …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET active_merge_operation_id=?,revision=revision+1 WHERE id=?&quot;, intent.Operation, request)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;MERGE_RECOVER&quot;, intent.Operation, intent.Operation, payload, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=24414:25372 -->
````go
func RegisterMerge(ctx context.Context, tx *sql.Tx, actor, request uint64, intent domain.MergeIntent, payload []byte) error {
	resolutions, err := json.Marshal(intent.Resolutions)
	if err != nil {
		return err
	}
	// receipt先由通用事务登记；请求仅保存进行中的操作指针，批准动作必须留到正式合并事务。
	if _, err = tx.ExecContext(ctx, "INSERT INTO merge_operation(operation_id,actor_id,request_id,round_id,content_version_id,own_changes_to_resolve,state,retry_at) VALUES(?,?,?,?,?,?,'PREPARING',UTC_TIMESTAMP(6))", intent.Operation, actor, request, intent.Round, intent.Version, string(resolutions)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE review_request SET active_merge_operation_id=?,revision=revision+1 WHERE id=?", intent.Operation, request); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "MERGE_RECOVER", intent.Operation, intent.Operation, payload, "QUEUED")
}

````

## 第 42 步：MergePayload

插入位置：第 41 步代码之后，紧接追加；完成稿约第 468 行。

本步声明：`func MergePayload(ctx context.Context, q Queryer, operation string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_RECOVER&#x27; AND business_key=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_RECOVER&#x27; AND business_key=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>payload</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_RECOVER&#x27; AND business_key=?&quot;, operation).Scan(&amp;payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;MERGE_RECOVER&#x27; AND business_key=?&quot;, operation)</code>。
- 此处通过 <code>return payload, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=25372:25641 -->
````go
func MergePayload(ctx context.Context, q Queryer, operation string) ([]byte, error) {
	var payload []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='MERGE_RECOVER' AND business_key=?", operation).Scan(&payload)
	return payload, err
}

````

## 第 43 步：MergeResult

插入位置：第 42 步代码之后，紧接追加；完成稿约第 474 行。

本步声明：`func MergeResult(ctx context.Context, q Queryer, operation string) (domain.SubmissionResultFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;MERGE&#x27;&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;MERGE&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionResultFact</code> | 使用 <code>domain.SubmissionResultFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;MERGE&#x27;&quot;, operation).Scan(&amp;fact.Stat …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind=&#x27;MERGE&#x27;&quot;, operation)</code>。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=25641:26062 -->
````go
func MergeResult(ctx context.Context, q Queryer, operation string) (domain.SubmissionResultFact, error) {
	var fact domain.SubmissionResultFact
	err := q.QueryRowContext(ctx, "SELECT state,generation,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind='MERGE'", operation).Scan(&fact.State, &fact.Generation, &fact.Fingerprint, &fact.Result, &fact.Expires)
	return fact, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/merges_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
