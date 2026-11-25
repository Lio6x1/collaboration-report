# 分步编写 `services/document/internal/repository/document/exports_queries.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/154-exports-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存固定导出作业、资源pin、转换预算、代次及终态制品。

排队作业与pin同事务，运行租约过期不能证明转换器退出或文件已删除。

写之前先按这条执行路线理解各段的连接：

1. InsertExport/Receipt固定输入
2. PinExportResources保留附件
3. ClaimExport占临时预算并推进generation
4. FinishExport以代次CAS落终态
5. RemoveExpiredExport在真实清理后释放记录。

本文件需要守住的边界：

- ExpireClosedDocumentExports仅让终态制品过期，不能把运行中转换改CANCELLED冒充退出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/exports_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/exports_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/exports_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/exports_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:428 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// export.go：保留原实现，按业务合并文件。

````

## 第 2 步：ExportError

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func ExportError(code string) *dto.Error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>Code: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*dto.Error</code> | 实际返回表达式：<code>&amp;dto.Error{Code: code, Message: &quot;导出未完成，请按错误码核对格式、权限或临时额度后重新创建&quot;, TraceId: platform.NewTraceID(), RetryAction: &quot;NONE&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;dto.Error{Code: code, Message: &quot;导出未完成，请按错误码核对格式、权限或临时额度后重新创建&quot;, TraceId: platform.NewTraceID(), RetryAction: &quot;NONE&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=428:655 -->
````go
func ExportError(code string) *dto.Error {
	return &dto.Error{Code: code, Message: "导出未完成，请按错误码核对格式、权限或临时额度后重新创建", TraceId: platform.NewTraceID(), RetryAction: "NONE"}
}

````

## 第 3 步：exportColumns

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`const exportColumns = \`j.id,COALESCE(j.requester_id,0),j.document_id,j.manifest_id,j.release_id,j.target_kind,j.format,j.state,j.operation_id,j.generation,COALESCE(j.last_error,''),c.fingerprint,c.snapshot,COALESCE(c.artifact_key,''),c.artifact_size,c.artifact_checksum,c.expires_at\``。

<!-- write-step: 3 bytes=655:940 -->
````go
const exportColumns = `j.id,COALESCE(j.requester_id,0),j.document_id,j.manifest_id,j.release_id,j.target_kind,j.format,j.state,j.operation_id,j.generation,COALESCE(j.last_error,''),c.fingerprint,c.snapshot,COALESCE(c.artifact_key,''),c.artifact_size,c.artifact_checksum,c.expires_at`

````

## 第 4 步：ExportRow

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func ExportRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.ExportRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>由 <code>scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 此处通过 <code>return scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?&quot;+suffix, id)</code>。

<!-- write-step: 4 bytes=940:1248 -->
````go
func ExportRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.ExportRecord, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	return scanExport(q.QueryRowContext(ctx, "SELECT "+exportColumns+" FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?"+suffix, id))
}

````

## 第 5 步：ExportOperation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func ExportOperation(ctx context.Context, q Queryer, operation string) (domain.ExportRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>由 <code>scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return scanExport(q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT &quot;+exportColumns+&quot; FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?&quot;, operation)</code>。

<!-- write-step: 5 bytes=1248:1515 -->
````go
func ExportOperation(ctx context.Context, q Queryer, operation string) (domain.ExportRecord, error) {
	return scanExport(q.QueryRowContext(ctx, "SELECT "+exportColumns+" FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?", operation))
}

````

## 第 6 步：scanExport

插入位置：第 5 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func scanExport(row *sql.Row) (domain.ExportRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>*sql.Row</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EXPORT_RECORD_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;value.Job.Id, &amp;value.Requester, &amp;value.Job.DocumentId, &amp;value.Job.ManifestId, &amp;value.Job.ReleaseId, &amp;target, &amp;value.Job.Format, &amp;value.Job.State, &amp;value.Operation, &amp;value …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>len(fingerprint) != 32 &#124;&#124; (len(checksum) != 0 &amp;&amp; len(checksum) != 32)</code>。条件成立时执行 <code>return value, errors.New(&quot;EXPORT_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>lastError != &quot;&quot;</code>。
- 检查 <code>err == nil &amp;&amp; (target != value.Snapshot.Target &#124;&#124; value.Job.Format != value.Snapshot.Format &#124;&#124; value.Job.DocumentId != domain.TextID(value.Snapshot.Document) &#124;&#124; value.Job.ManifestI …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1515:2666 -->
````go
func scanExport(row *sql.Row) (domain.ExportRecord, error) {
	var value domain.ExportRecord
	var fingerprint, snapshot, checksum []byte
	var target, lastError string
	err := row.Scan(&value.Job.Id, &value.Requester, &value.Job.DocumentId, &value.Job.ManifestId, &value.Job.ReleaseId, &target, &value.Job.Format, &value.Job.State, &value.Operation, &value.Generation, &lastError, &fingerprint, &snapshot, &value.ArtifactKey, &value.Size, &checksum, &value.Expires)
	if err != nil {
		return value, err
	}
	if len(fingerprint) != 32 || (len(checksum) != 0 && len(checksum) != 32) {
		return value, errors.New("EXPORT_RECORD_INVALID")
	}
	copy(value.Fingerprint[:], fingerprint)
	copy(value.Checksum[:], checksum)
	value.Snapshot, err = domain.DecodeExportSnapshot(snapshot)
	if lastError != "" {
		value.Job.LastError = ExportError(lastError)
	}
	if err == nil && (target != value.Snapshot.Target || value.Job.Format != value.Snapshot.Format || value.Job.DocumentId != domain.TextID(value.Snapshot.Document) || value.Job.ManifestId != domain.TextID(value.Snapshot.Manifest)) {
		err = errors.New("EXPORT_FIXED_BINDING_CHANGED")
	}
	return value, err
}

````

## 第 7 步：InsertExport

插入位置：第 6 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func InsertExport(ctx context.Context, tx *sql.Tx, requester uint64, operation string, fingerprint [32]byte, fixed domain.ExportSnapshot) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO export_job(requester_id,document_id,manifest_id,release_id,target_kind,format,state,operation_id) VALUES(?,?,?,?,?,?,&#x27;QUEUED&#x27;,?)`, actor, fixed.Doc …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO export_control(job_id,fingerprint,snapshot,expires_at) VALUES(?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))`, id, fingerprint&#91;:&#93;, raw)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>requester</code> | <code>uint64</code> | 在此声明中的实际使用：<code>requester != 0</code>；<code>actor = requester</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO export_job(requester_id,document_id,manifest_id,release_id,target_kind,format,state,operation_id) VALUES(?,?,?,?,?,?,&#x27;QUEUED&#x27;,?)`, actor, fixed.Doc …</code>（长表达式，按本步完整代码继续） |
| <code>fingerprint</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO export_control(job_id,fingerprint,snapshot,expires_at) VALUES(?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))`, id, fingerprint&#91;:&#93;, raw)</code> |
| <code>fixed</code> | <code>domain.ExportSnapshot</code> | 使用 <code>domain.ExportSnapshot</code> 类型；其定义处的职责：固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。<br>在此声明中的实际使用：<code>fixed.Release</code>；<code>fixed.Document</code>；<code>fixed.Manifest</code>；<code>fixed.Target</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>requester != 0</code>。
- 检查 <code>fixed.Release != 0</code>。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO export_job(requester_id,document_id,manifest_id,release_id,target_kind,format,state,operation_id) VALUES(?,?,?,?,?,?,&#x27;QUEUED&#x27;,?)`, actor, fixed.Doc …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO export_control(job_id,fingerprint,snapshot,expires_at) VALUES(?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))`, id, fingerprint&#91;:&#93;, raw)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2666:3595 -->
````go
func InsertExport(ctx context.Context, tx *sql.Tx, requester uint64, operation string, fingerprint [32]byte, fixed domain.ExportSnapshot) (uint64, error) {
	var actor, release any
	if requester != 0 {
		actor = requester
	}
	if fixed.Release != 0 {
		release = fixed.Release
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO export_job(requester_id,document_id,manifest_id,release_id,target_kind,format,state,operation_id) VALUES(?,?,?,?,?,?,'QUEUED',?)`, actor, fixed.Document, fixed.Manifest, release, fixed.Target, fixed.Format, operation)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO export_control(job_id,fingerprint,snapshot,expires_at) VALUES(?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))`, id, fingerprint[:], raw)
	return uint64(id), err
}

````

## 第 8 步：InsertExportReceipt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func InsertExportReceipt(ctx context.Context, tx *sql.Tx, requester uint64, identityRef, operation, path string, fingerprint [32]byte, job uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,&#x27;EXPORT_CREATE&#x27;,?,?)`, auditActor, operation, target, result)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>requester</code> | <code>uint64</code> | 在此声明中的实际使用：<code>requester != 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）；<code>auditActor = requester</code> |
| <code>identityRef</code> | <code>string</code> | 在此声明中的实际使用：<code>identity = identityRef</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: operation</code>；<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,&#x27;EXPORT_CREATE&#x27;,?,?)`, auditActor, operation, target, result)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;path&quot;: path</code> |
| <code>fingerprint</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续） |
| <code>job</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(job)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>requester != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>requester != 0</code>。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,&#x27;EXPORT_CREATE&#x27;,?,?)`, auditActor, operation, target, result)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3595:5060 -->
````go
func InsertExportReceipt(ctx context.Context, tx *sql.Tx, requester uint64, identityRef, operation, path string, fingerprint [32]byte, job uint64) error {
	var identity any
	if requester != 0 {
		identity = identityRef
	}
	target, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return err
	}
	// SUCCEEDED表示登记已经提交；作业本身另有QUEUED/RUNNING状态，转换未知不伪装成功。
	result, err := json.Marshal(map[string]any{"operation_id": operation, "state": "SUCCEEDED", "result": map[string]string{"export_job_id": domain.TextID(job)}})
	if err != nil {
		return err
	}
	// 冻结receipt.actor_id非空，匿名主体使用不可登录的保留编号0；job.requester_id仍为NULL。
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUES(?,?,'EXPORT_CREATE',?,?,'SUCCEEDED',?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),UTC_TIMESTAMP(6))`, operation, requester, target, fingerprint[:], identity, result)
	if err != nil {
		return err
	}
	var auditActor any
	if requester != 0 {
		auditActor = requester
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,'EXPORT_CREATE',?,?)`, auditActor, operation, target, result)
	return err
}

// export_drain.go：保留原实现，按业务合并文件。

````

## 第 9 步：ExpireClosedDocumentExports

插入位置：第 8 步代码之后，紧接追加；完成稿约第 111 行。

清理调用方必须先完成文档来源关闭；这里再次核生命周期与真实未终结作业。
只能让终态制品提前到期，不能把仍在运行的转换器改成CANCELLED冒充已退出。

本步声明：`func ExpireClosedDocumentExports(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;))&quot;, document)</code>；<code>tx.ExecContext(ctx, `UPDATE export_control c JOIN export_job j ON j.id=c.job_id 	 SET c.expires_at=LEAST(c.expires_at,UTC_TIMESTAMP(6)) WHERE j.document_id=?`, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;))&quot;, document)</code>；<code>tx.ExecContext(ctx, `UPDATE export_control c JOIN export_job j ON j.id=c.job_id 	 SET c.expires_at=LEAST(c.expires_at,UTC_TIMESTAMP(6)) WHERE j.document_id=?`, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_NOT_CLOSED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_DRAIN_PENDING&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document).Scan(&amp;lifecycle)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 检查 <code>lifecycle != &quot;CLOSING&quot; &amp;&amp; lifecycle != &quot;PURGED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_NOT_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;))&quot;, document).Scan(&amp;pending)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;))&quot;, document)</code>。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCUMENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE export_control c JOIN export_job j ON j.id=c.job_id 	 SET c.expires_at=LEAST(c.expires_at,UTC_TIMESTAMP(6)) WHERE j.document_id=?`, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5060:6170 -->
````go
// 清理调用方必须先完成文档来源关闭；这里再次核生命周期与真实未终结作业。
// 只能让终态制品提前到期，不能把仍在运行的转换器改成CANCELLED冒充已退出。
func ExpireClosedDocumentExports(ctx context.Context, tx *sql.Tx, document uint64) error {
	var lifecycle string
	if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM document WHERE id=? FOR UPDATE", document).Scan(&lifecycle); err != nil {
		return err
	}
	if lifecycle != "CLOSING" && lifecycle != "PURGED" {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_NOT_CLOSED")
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN ('QUEUED','RUNNING'))", document).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_DRAIN_PENDING")
	}
	_, err := tx.ExecContext(ctx, `UPDATE export_control c JOIN export_job j ON j.id=c.job_id
	 SET c.expires_at=LEAST(c.expires_at,UTC_TIMESTAMP(6)) WHERE j.document_id=?`, document)
	return err
}

````

## 第 10 步：DocumentExportControlsPending

插入位置：第 9 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func DocumentExportControlsPending(ctx context.Context, q Queryer, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c 	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c 	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>pending</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c 	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document).Scan(&amp;pending)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c 	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document)</code>。
- 此处通过 <code>return pending, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6170:6542 -->
````go
func DocumentExportControlsPending(ctx context.Context, q Queryer, document uint64) (bool, error) {
	var pending bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c
	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document).Scan(&pending)
	return pending, err
}

// export_expiry.go：保留原实现，按业务合并文件。

````

## 第 11 步：ExpiredExport

插入位置：第 10 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`type ExpiredExport struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 待回收导出任务编号。 |
| <code>Size</code> | <code>uint64</code> | 已经产出的文件字节数。 |
| <code>Reserved</code> | <code>uint64</code> | 该任务尚占用的预留容量。 |
| <code>Key</code> | <code>string</code> | 导出文件的存储键；回收文件与释放额度需按原任务核对。 |

<!-- write-step: 11 bytes=6542:6627 -->
````go
type ExpiredExport struct {
	ID, Size, Reserved uint64
	Key                string
}

````

## 第 12 步：ExpiredExports

插入位置：第 11 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func ExpiredExports(ctx context.Context, q Queryer) ([]ExpiredExport, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT c.job_id,COALESCE(c.artifact_key,&#x27;&#x27;),c.artifact_size,c.reserved_bytes 	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id 	 WHERE (j.id IS N …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;ExpiredExport</code> | 实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT c.job_id,COALESCE(c.artifact_key,&#x27;&#x27;),c.artifact_size,c.reserved_bytes 	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id 	 WHERE (j.id IS N …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.ID, &amp;item.Key, &amp;item.Size, &amp;item.Reserved)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return items, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6627:7437 -->
````go
func ExpiredExports(ctx context.Context, q Queryer) ([]ExpiredExport, error) {
	// 清理历史已删除job也精确回收其独立制品；不会留下注销后的临时副本等一小时。
	rows, err := q.QueryContext(ctx, `SELECT c.job_id,COALESCE(c.artifact_key,''),c.artifact_size,c.reserved_bytes
	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id
	 WHERE (j.id IS NULL OR c.expires_at<=UTC_TIMESTAMP(6)) AND (j.id IS NULL OR j.state NOT IN ('QUEUED','RUNNING')) ORDER BY c.job_id LIMIT 16`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ExpiredExport
	for rows.Next() {
		var item ExpiredExport
		if err = rows.Scan(&item.ID, &item.Key, &item.Size, &item.Reserved); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

````

## 第 13 步：RemoveExpiredExport

插入位置：第 12 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`func RemoveExpiredExport(ctx context.Context, tx *sql.Tx, expected ExpiredExport) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext(ctx, `SELECT COALESCE(c.artifact_key,&#x27;&#x27;),c.artifact_size,c.reserved_bytes,(j.id IS NULL OR c.expires_at&lt;=UTC_TIMESTAMP(6)) 	 FROM export_control c LEFT JOIN expo …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET used_bytes=used_bytes-?,reserved_bytes=reserved_bytes-? WHERE id=1&quot;, size, own)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=&#x27;CANCELLED&#x27;,last_error=&#x27;EXPORT_EXPIRED&#x27; WHERE id=?&quot;, expected.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>expected</code> | <code>ExpiredExport</code> | 在此声明中的实际使用：<code>expected.ID</code>；<code>expected.Key</code>；<code>expected.Size</code>；<code>expected.Reserved</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EXPORT_EXPIRY_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(c.artifact_key,&#x27;&#x27;),c.artifact_size,c.reserved_bytes,(j.id IS NULL OR c.expires_at&lt;=UTC_TIMESTAMP(6)) 	 FROM export_control c LEFT JOIN expo …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(c.artifact_key,&#x27;&#x27;),c.artifact_size,c.reserved_bytes,(j.id IS NULL OR c.expires_at&lt;=UTC_TIMESTAMP(6)) 	 FROM export_control c LEFT JOIN expo …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!expired &#124;&#124; key != expected.Key &#124;&#124; size != expected.Size &#124;&#124; own != expected.Reserved &#124;&#124; size &gt; used &#124;&#124; own &gt; reserved</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_EXPIRY_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET used_bytes=used_bytes-?,reserved_bytes=reserved_bytes-? WHERE id=1&quot;, size, own)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=&#x27;CANCELLED&#x27;,last_error=&#x27;EXPORT_EXPIRED&#x27; WHERE id=?&quot;, expected.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM export_control WHERE job_id=?&quot;, expected.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 13 bytes=7437:8664 -->
````go
func RemoveExpiredExport(ctx context.Context, tx *sql.Tx, expected ExpiredExport) error {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return err
	}
	var key string
	var size, own uint64
	var expired bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(c.artifact_key,''),c.artifact_size,c.reserved_bytes,(j.id IS NULL OR c.expires_at<=UTC_TIMESTAMP(6))
	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id WHERE c.job_id=? FOR UPDATE`, expected.ID).Scan(&key, &size, &own, &expired)
	if err != nil {
		return err
	}
	if !expired || key != expected.Key || size != expected.Size || own != expected.Reserved || size > used || own > reserved {
		return errors.New("EXPORT_EXPIRY_BINDING_CHANGED")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET used_bytes=used_bytes-?,reserved_bytes=reserved_bytes-? WHERE id=1", size, own); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_job SET state='CANCELLED',last_error='EXPORT_EXPIRED' WHERE id=?", expected.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM export_control WHERE job_id=?", expected.ID)
	return err
}

// export_fixed.go：保留原实现，按业务合并文件。

````

## 第 14 步：ExportMainView

插入位置：第 13 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func ExportMainView(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8664:8872 -->
````go
func ExportMainView(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error) {
	return resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)
}

````

## 第 15 步：ExportManifestFacts

插入位置：第 14 步代码之后，紧接追加；完成稿约第 199 行。

本步声明：`func ExportManifestFacts(ctx context.Context, q Queryer, document, manifest uint64) (domain.ExportSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Document: document</code>；<code>q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Manifest: manifest</code>；<code>q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportSnapshot</code> | 使用 <code>domain.ExportSnapshot</code> 类型；其定义处的职责：固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document).Scan(&amp;value.Structure, &amp;value.Style)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8872:9251 -->
````go
func ExportManifestFacts(ctx context.Context, q Queryer, document, manifest uint64) (domain.ExportSnapshot, error) {
	value := domain.ExportSnapshot{Document: document, Manifest: manifest}
	err := q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document).Scan(&value.Structure, &value.Style)
	return value, err
}

````

## 第 16 步：PinExportResources

插入位置：第 15 步代码之后，紧接追加；完成稿约第 205 行。

资源pin与排队作业同事务；引用来自固定版本，不能把活跃草稿或任意URL放进导出。

本步声明：`func PinExportResources(ctx context.Context, tx *sql.Tx, id uint64, fixed domain.ExportSnapshot) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) 	 SELECT DISTINCT r.resource_id,e.document_id,&#x27;EXPORT&#x27;,?,&#x27;CONTENT&#x27; FROM ma …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(id)</code> |
| <code>fixed</code> | <code>domain.ExportSnapshot</code> | 使用 <code>domain.ExportSnapshot</code> 类型；其定义处的职责：固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。<br>在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Manifest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) 	 SELECT DISTINCT r.resource_id,e.document_id,&#x27;EXPORT&#x27;,?,&#x27;CONTENT&#x27; FROM ma …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9251:9950 -->
````go
// 资源pin与排队作业同事务；引用来自固定版本，不能把活跃草稿或任意URL放进导出。
func PinExportResources(ctx context.Context, tx *sql.Tx, id uint64, fixed domain.ExportSnapshot) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose)
	 SELECT DISTINCT r.resource_id,e.document_id,'EXPORT',?,'CONTENT' FROM manifest_entry e
	 JOIN resource_reference r ON r.document_id=e.document_id AND r.owner_kind='VERSION' AND r.owner_ref=CAST(e.content_version_id AS CHAR) AND r.purpose='CONTENT'
	 WHERE e.document_id=? AND e.manifest_id=?`, domain.TextID(id), fixed.Document, fixed.Manifest)
	return err
}

````

## 第 17 步：ReleaseExportResources

插入位置：第 16 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`func ReleaseExportResources(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE owner_kind=&#x27;EXPORT&#x27; AND owner_ref=?&quot;, domain.TextID(id))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE owner_kind=&#x27;EXPORT&#x27; AND owner_ref=?&quot;, domain.TextID(id))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9950:10237 -->
````go
func ReleaseExportResources(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE owner_kind='EXPORT' AND owner_ref=?", domain.TextID(id))
	return err
}

// export_worker.go：保留原实现，按业务合并文件。

````

## 第 18 步：ExportTemporaryLimit

插入位置：第 17 步代码之后，紧接追加；完成稿约第 221 行。

本步声明：`const ExportTemporaryLimit uint64 = 256 << 20`。

<!-- write-step: 18 bytes=10237:10283 -->
````go
const ExportTemporaryLimit uint64 = 256 << 20
````

## 第 19 步：ExportWorkReservation

插入位置：第 18 步代码之后，紧接追加；完成稿约第 222 行。

本步声明：`const ExportWorkReservation uint64 = 192 << 20`。

<!-- write-step: 19 bytes=10283:10331 -->
````go
const ExportWorkReservation uint64 = 192 << 20

````

## 第 20 步：lockExportBudget

插入位置：第 19 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func lockExportBudget(ctx context.Context, tx *sql.Tx) (uint64, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO export_budget(id) VALUES(1)&quot;)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes,used_bytes FROM export_budget WHERE id=1 FOR UPDATE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>reserved</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>used</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT IGNORE INTO export_budget(id) VALUES(1)&quot;)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes,used_bytes FROM export_budget WHERE id=1 FOR UPDATE&quot;).Scan(&amp;reserved, &amp;used)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes,used_bytes FROM export_budget WHERE id=1 FOR UPDATE&quot;)</code>。
- 此处通过 <code>return reserved, used, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10331:10722 -->
````go
func lockExportBudget(ctx context.Context, tx *sql.Tx) (uint64, uint64, error) {
	if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO export_budget(id) VALUES(1)"); err != nil {
		return 0, 0, err
	}
	var reserved, used uint64
	err := tx.QueryRowContext(ctx, "SELECT reserved_bytes,used_bytes FROM export_budget WHERE id=1 FOR UPDATE").Scan(&reserved, &used)
	return reserved, used, err
}

````

## 第 21 步：CheckExportQueue

插入位置：第 20 步代码之后，紧接追加；完成稿约第 233 行。

本步声明：`func CheckExportQueue(ctx context.Context, tx *sql.Tx) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM export_job WHERE state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EXPORT_QUEUE_FULL&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM export_job WHERE state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;).Scan(&amp;jobs)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM export_job WHERE state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;)</code>。
- 检查 <code>jobs &gt;= 32</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;EXPORT_QUEUE_FULL&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=10722:11136 -->
````go
func CheckExportQueue(ctx context.Context, tx *sql.Tx) error {
	if _, _, err := lockExportBudget(ctx, tx); err != nil {
		return err
	}
	var jobs uint64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM export_job WHERE state IN ('QUEUED','RUNNING')").Scan(&jobs); err != nil {
		return err
	}
	if jobs >= 32 {
		return domain.Violation(domain.TemporarilyUnavailable, "EXPORT_QUEUE_FULL")
	}
	return nil
}

````

## 第 22 步：NextExports

插入位置：第 21 步代码之后，紧接追加；完成稿约第 247 行。

本步声明：`func NextExports(ctx context.Context, q Queryer) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT id FROM export_job WHERE state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;) AND (retry_at IS NULL OR retry_at&lt;=UTC_TIMESTAMP(6)) ORDER BY id LIMIT 32`)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT id FROM export_job WHERE state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;) AND (retry_at IS NULL OR retry_at&lt;=UTC_TIMESTAMP(6)) ORDER BY id LIMIT 32`)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=11136:11711 -->
````go
func NextExports(ctx context.Context, q Queryer) ([]uint64, error) {
	// 与持久队列上限一致；候选不是领取证明，业务层还须取得对应job文件锁。
	rows, err := q.QueryContext(ctx, `SELECT id FROM export_job WHERE state IN ('QUEUED','RUNNING') AND (retry_at IS NULL OR retry_at<=UTC_TIMESTAMP(6)) ORDER BY id LIMIT 32`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
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

## 第 23 步：ClaimExport

插入位置：第 22 步代码之后，紧接追加；完成稿约第 265 行。

跨进程文件guard在调用前已取得；RUNNING恢复仍沿原job，不靠TTL判断转换成功。

本步声明：`func ClaimExport(ctx context.Context, tx *sql.Tx, id, temporaryLimit uint64) (domain.ExportRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>ExportRow(ctx, tx, id, true)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes FROM export_control WHERE job_id=?&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET reserved_bytes=reserved_bytes+? WHERE id=1&quot;, ExportWorkReservation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>ExportRow(ctx, tx, id, true)</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ExportRow(ctx, tx, id, true)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes FROM export_control WHERE job_id=?&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_control SET reserved_bytes=? WHERE job_id=?&quot;, ExportWorkReservation, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=&#x27;RUNNING&#x27;,generation=generation+1,retry_at=NULL,last_error=NULL WHERE id=?&quot;, id)</code> |
| <code>temporaryLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>used &gt; temporaryLimit</code>；<code>temporaryLimit-used</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>实际返回表达式：<code>domain.ExportRecord{}</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_STATE_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EXPORT_TEMPORARY_QUOTA_EXCEEDED&quot;)</code>；<code>errors.New(&quot;EXPORT_RESERVATION_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.ExportRecord{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>value.Job.State != &quot;QUEUED&quot; &amp;&amp; value.Job.State != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return value, domain.Violation(domain.StateConflict, &quot;EXPORT_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes FROM export_control WHERE job_id=?&quot;, id).Scan(&amp;own)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT reserved_bytes FROM export_control WHERE job_id=?&quot;, id)</code>。
- 检查 <code>own == 0</code>。
- 检查 <code>used &gt; temporaryLimit &#124;&#124; reserved &gt; temporaryLimit-used &#124;&#124; ExportWorkReservation &gt; temporaryLimit-used-reserved</code>。条件成立时执行 <code>return value, domain.Violation(domain.TemporarilyUnavailable, &quot;EXPORT_TEMPORARY_QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET reserved_bytes=reserved_bytes+? WHERE id=1&quot;, ExportWorkReservation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_control SET reserved_bytes=? WHERE job_id=?&quot;, ExportWorkReservation, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>own != ExportWorkReservation</code>。条件成立时执行 <code>return value, errors.New(&quot;EXPORT_RESERVATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Generation == ^uint64(0)</code>。条件成立时执行 <code>return value, errors.New(&quot;EXPORT_GENERATION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=&#x27;RUNNING&#x27;,generation=generation+1,retry_at=NULL,last_error=NULL WHERE id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=11711:13402 -->
````go
// 跨进程文件guard在调用前已取得；RUNNING恢复仍沿原job，不靠TTL判断转换成功。
func ClaimExport(ctx context.Context, tx *sql.Tx, id, temporaryLimit uint64) (domain.ExportRecord, error) {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return domain.ExportRecord{}, err
	}
	value, err := ExportRow(ctx, tx, id, true)
	if err != nil {
		return value, err
	}
	if value.Job.State != "QUEUED" && value.Job.State != "RUNNING" {
		return value, domain.Violation(domain.StateConflict, "EXPORT_STATE_CHANGED")
	}
	var own uint64
	if err = tx.QueryRowContext(ctx, "SELECT reserved_bytes FROM export_control WHERE job_id=?", id).Scan(&own); err != nil {
		return value, err
	}
	if own == 0 {
		if used > temporaryLimit || reserved > temporaryLimit-used || ExportWorkReservation > temporaryLimit-used-reserved {
			return value, domain.Violation(domain.TemporarilyUnavailable, "EXPORT_TEMPORARY_QUOTA_EXCEEDED")
		}
		if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET reserved_bytes=reserved_bytes+? WHERE id=1", ExportWorkReservation); err != nil {
			return value, err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE export_control SET reserved_bytes=? WHERE job_id=?", ExportWorkReservation, id); err != nil {
			return value, err
		}
	} else if own != ExportWorkReservation {
		return value, errors.New("EXPORT_RESERVATION_CHANGED")
	}
	if value.Generation == ^uint64(0) {
		return value, errors.New("EXPORT_GENERATION_EXHAUSTED")
	}
	_, err = tx.ExecContext(ctx, "UPDATE export_job SET state='RUNNING',generation=generation+1,retry_at=NULL,last_error=NULL WHERE id=?", id)
	value.Generation++
	value.Job.State = "RUNNING"
	return value, err
}

````

## 第 24 步：FinishExport

插入位置：第 23 步代码之后，紧接追加；完成稿约第 304 行。

本步声明：`func FinishExport(ctx context.Context, tx *sql.Tx, id, generation uint64, state, code, key string, size uint64, checksum [32]byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET reserved_bytes=reserved_bytes-?,used_bytes=used_bytes+? WHERE id=1&quot;, own, size)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_control SET reserved_bytes=0,artifact_key=?,artifact_size=?,artifact_checksum=? WHERE job_id=?&quot;, artifact, size, digest, id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockExportBudget(ctx, tx)</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code>；<code>ReleaseExportResources(ctx, tx, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_control SET reserved_bytes=0,artifact_key=?,artifact_size=?,artifact_checksum=? WHERE job_id=?&quot;, artifact, size, digest, id)</code>；<code>ReleaseExportResources(ctx, tx, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?&quot;, state, lastError, id, generation)</code> |
| <code>generation</code> | <code>uint64</code> | 在此声明中的实际使用：<code>current != generation</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?&quot;, state, lastError, id, generation)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?&quot;, state, lastError, id, generation)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>code != &quot;&quot;</code>；<code>lastError = code</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>key != &quot;&quot;</code>；<code>artifact = key</code> |
| <code>size</code> | <code>uint64</code> | 在此声明中的实际使用：<code>size &gt; 32&lt;&lt;20</code>；<code>size &gt; own</code>；<code>size &gt; ^uint64(0)-used</code>；<code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET reserved_bytes=reserved_bytes-?,used_bytes=used_bytes+? WHERE id=1&quot;, own, size)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>checksum</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>digest = checksum&#91;:&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EXPORT_FINISH_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;EXPORT_ALREADY_TERMINAL&quot;)</code>；<code>errors.New(&quot;EXPORT_BUDGET_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE&quot;, id).Scan(&amp;current, &amp;oldState, &amp;own, &amp;oldSize)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE&quot;, id)</code>。
- 检查 <code>current != generation &#124;&#124; oldSize != 0</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_FINISH_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>oldState != &quot;QUEUED&quot; &amp;&amp; oldState != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_ALREADY_TERMINAL&quot;)</code>。当前函数在这里结束。
- 检查 <code>own &gt; reserved &#124;&#124; size &gt; 32&lt;&lt;20 &#124;&#124; size &gt; own &#124;&#124; size &gt; ^uint64(0)-used</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_BUDGET_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>key != &quot;&quot;</code>。
- 检查 <code>code != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_budget SET reserved_bytes=reserved_bytes-?,used_bytes=used_bytes+? WHERE id=1&quot;, own, size)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_control SET reserved_bytes=0,artifact_key=?,artifact_size=?,artifact_checksum=? WHERE job_id=?&quot;, artifact, size, digest, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?&quot;, state, lastError, id, generation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 24 bytes=13402:15198 -->
````go
func FinishExport(ctx context.Context, tx *sql.Tx, id, generation uint64, state, code, key string, size uint64, checksum [32]byte) error {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return err
	}
	var current, own, oldSize uint64
	var oldState string
	if err = tx.QueryRowContext(ctx, "SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE", id).Scan(&current, &oldState, &own, &oldSize); err != nil {
		return err
	}
	if current != generation || oldSize != 0 {
		return errors.New("EXPORT_FINISH_BINDING_CHANGED")
	}
	if oldState != "QUEUED" && oldState != "RUNNING" {
		return errors.New("EXPORT_ALREADY_TERMINAL")
	}
	// 完成用已持久预留换成更小的制品，不能再次申请空间。
	// 配置调低后仍允许原RUNNING恢复/取消并归还预留，避免旧预算永远卡死队列。
	if own > reserved || size > 32<<20 || size > own || size > ^uint64(0)-used {
		return errors.New("EXPORT_BUDGET_CHANGED")
	}
	var artifact, digest, lastError any
	if key != "" {
		artifact = key
		digest = checksum[:]
	}
	if code != "" {
		lastError = code
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET reserved_bytes=reserved_bytes-?,used_bytes=used_bytes+? WHERE id=1", own, size); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_control SET reserved_bytes=0,artifact_key=?,artifact_size=?,artifact_checksum=? WHERE job_id=?", artifact, size, digest, id); err != nil {
		return err
	}
	if err = ReleaseExportResources(ctx, tx, id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?", state, lastError, id, generation)
	return err
}

````

## 第 25 步：DeferExport

插入位置：第 24 步代码之后，紧接追加；完成稿约第 346 行。

本步声明：`func DeferExport(ctx context.Context, db *sql.DB, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE export_job SET retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;, id)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>db.ExecContext(ctx, &quot;UPDATE export_job SET retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, &quot;UPDATE export_job SET retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=? AND state IN (&#x27;QUEUED&#x27;,&#x27;RUNNING&#x27;)&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=15198:15440 -->
````go
func DeferExport(ctx context.Context, db *sql.DB, id uint64) error {
	_, err := db.ExecContext(ctx, "UPDATE export_job SET retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=? AND state IN ('QUEUED','RUNNING')", id)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/exports_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
