# 分步编写 `services/search/internal/repository/rebuild/installation_queries.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/389-installation-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存已验证的导入封口，并维护快照播种进度。

文件发布与SQL不原子，因此先发布完整校验副本，再在短事务中记录接受事实；播种游标则与每篇文档的物理应用同事务推进。

写之前先按这条执行路线理解各段的连接：

1. FindImport 恢复数据库已接受的封装摘要/签发者摘要，检查封口数量与条目摘要。
2. SaveImport 仅把原 PREPARED 行条件更新为IMPORTED，调用者提交事务。
3. PendingSnapshot 找到待播种原号；AdvanceSnapshot 用before做CAS，FinishSnapshot仅将SNAPSHOT推进为REPLAYING。

本文件需要守住的边界：

- 已接受签名恢复依据持久摘要，不因当前证书到期重签原事实。
- ES写入或SQL提交未知时保留原after，不能猜测已推进。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/rebuild/installation_queries.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/rebuild/installation_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/rebuild/installation_queries.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/rebuild/installation_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:318 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
)

// import.go：保留原实现，按业务合并文件。

````

## 第 2 步：Imported

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type Imported struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Seal</code> | <code>rebuildmeta.Seal</code> | 解析后的Doc快照封口，绑定prepare引用、条目数量和摘要。 |
| <code>Envelope</code> | <code>&#91;&#93;byte</code> | 保留原始签名封装字节，用于原号恢复。 |
| <code>Digest</code> | <code>&#91;32&#93;byte</code> | 整个签名封装的SHA-256摘要。 |
| <code>SignerDigest</code> | <code>&#91;32&#93;byte</code> | 原接受的签发者摘要，恢复不更换签发者。 |

<!-- write-step: 2 bytes=318:443 -->
````go
type Imported struct {
	Seal                 rebuildmeta.Seal
	Envelope             []byte
	Digest, SignerDigest [32]byte
}

````

## 第 3 步：FindImport

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

FindImport只在代次、原binding依次锁定后调用；已接受签名按持久摘要恢复，证书到期不改变原事实。

本步声明：`func FindImport(ctx context.Context, tx *sql.Tx, operation string) (Imported, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 最初准备的重建原号。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Imported</code> | 实际返回表达式：<code>row</code>；<code>Imported{}</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_IMPORT_READ_FAILED&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, o …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, false, errors.New(&quot;SEARCH_REBUILD_IMPORT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(row.Envelope) == 0 &amp;&amp; digest == nil &amp;&amp; signer == nil &amp;&amp; !count.Valid &amp;&amp; entries == nil</code>。条件成立时执行 <code>return row, false, nil</code>。当前函数在这里结束。
- 检查 <code>len(digest) != 32 &#124;&#124; len(signer) != 32 &#124;&#124; len(entries) != 32 &#124;&#124; !count.Valid</code>。条件成立时执行 <code>return row, false, errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return row, false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; row.Seal.Count != count.String &#124;&#124; row.Seal.EntriesDigest != hex.EncodeToString(entries)</code>。条件成立时执行 <code>return Imported{}, false, errors.New(&quot;SEARCH_REBUILD_IMPORT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return row, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=443:1867 -->
````go
// FindImport只在代次、原binding依次锁定后调用；已接受签名按持久摘要恢复，证书到期不改变原事实。
func FindImport(ctx context.Context, tx *sql.Tx, operation string) (Imported, bool, error) {
	var row Imported
	var digest, signer, entries []byte
	var count sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).Scan(&row.Envelope, &digest, &signer, &count, &entries)
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_IMPORT_READ_FAILED")
	}
	if len(row.Envelope) == 0 && digest == nil && signer == nil && !count.Valid && entries == nil {
		return row, false, nil
	}
	if len(digest) != 32 || len(signer) != 32 || len(entries) != 32 || !count.Valid {
		return row, false, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	copy(row.Digest[:], digest)
	copy(row.SignerDigest[:], signer)
	verified, err := rebuildartifact.RestoreAccepted(row.Envelope, rebuildartifact.SnapshotV1, row.Digest, row.SignerDigest)
	if err != nil {
		return row, false, err
	}
	row.Seal, err = rebuildmeta.ParseSeal(verified.Payload)
	if err != nil || row.Seal.Count != count.String || row.Seal.EntriesDigest != hex.EncodeToString(entries) {
		return Imported{}, false, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	return row, true, nil
}

````

## 第 4 步：SaveImport

插入位置：第 3 步代码之后，紧接追加；完成稿约第 49 行。

SaveImport不提交事务。完整副本必须已经发布；未知提交只能恢复同一原号。

本步声明：`func SaveImport(ctx context.Context, tx *sql.Tx, operation string, row Imported) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET snapshot_digest=?,snapshot_envelope=?,snapshot_count=?,entries_digest=?,signer_digest=?,state=&#x27;IMPORTED&#x27;,revision=revision+1 WHERE o …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 最初准备的重建原号。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET snapshot_digest=?,snapshot_envelope=?,snapshot_count=?,entries_digest=?,signer_digest=?,state=&#x27;IMPORTED&#x27;,revision=revision+1 WHERE o …</code>（长表达式，按本步完整代码继续） |
| <code>row</code> | <code>Imported</code> | 已经校验并完整发布到accepted目录的导入封口。<br>在此声明中的实际使用：<code>row.Seal</code>；<code>row.Digest</code>；<code>row.Envelope</code>；<code>row.SignerDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_IMPORT_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_IMPORT_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_binding SET snapshot_digest=?,snapshot_envelope=?,snapshot_count=?,entries_digest=?,signer_digest=?,state=&#x27;IMPORTED&#x27;,revision=revision+1 WHERE o …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_IMPORT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_IMPORT_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1867:2809 -->
````go
// SaveImport不提交事务。完整副本必须已经发布；未知提交只能恢复同一原号。
func SaveImport(ctx context.Context, tx *sql.Tx, operation string, row Imported) error {
	if _, err := row.Seal.Bytes(); err != nil {
		return err
	}
	entries, _ := hex.DecodeString(row.Seal.EntriesDigest)
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_binding SET snapshot_digest=?,snapshot_envelope=?,snapshot_count=?,entries_digest=?,signer_digest=?,state='IMPORTED',revision=revision+1 WHERE operation_id=? AND state='PREPARED' AND revision<18446744073709551615`, row.Digest[:], row.Envelope, row.Seal.Count, entries, row.SignerDigest[:], operation)
	if err != nil {
		return errors.New("SEARCH_REBUILD_IMPORT_WRITE_FAILED")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("SEARCH_REBUILD_IMPORT_CONFLICT")
	}
	return nil
}

// seed.go：保留原实现，按业务合并文件。

````

## 第 5 步：PendingSnapshot

插入位置：第 4 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func PendingSnapshot(ctx context.Context, db *sql.DB) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state=&#x27;SNAPSHOT&#x27; ORDER BY j.id LIMIT 1`)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>operation</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state=&#x27;SNAPSHOT&#x27; ORDER BY j.id LIMIT 1`).Scan(&amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state=&#x27;SNAPSHOT&#x27; ORDER BY j.id LIMIT 1`)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return &quot;&quot;, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return operation, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2809:3266 -->
````go
func PendingSnapshot(ctx context.Context, db *sql.DB) (string, error) {
	var operation string
	err := db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state='ACCEPTED' AND j.state='SNAPSHOT' ORDER BY j.id LIMIT 1`).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return operation, nil
}

````

## 第 6 步：AdvanceSnapshot

插入位置：第 5 步代码之后，紧接追加；完成稿约第 80 行。

与该文档index_application同事务推进；ES或提交未知时必须保留原after重做。

本步声明：`func AdvanceSnapshot(ctx context.Context, tx *sql.Tx, id, before, after uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision&lt;18446744073709551615`, after, id, before)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 原重建job主键。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision&lt;18446744073709551615`, after, id, before)</code> |
| <code>before</code> | <code>uint64</code> | 本轮读到的旧snapshot_after，用作并发CAS。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision&lt;18446744073709551615`, after, id, before)</code> |
| <code>after</code> | <code>uint64</code> | 本篇文档成功应用后的新snapshot_after。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision&lt;18446744073709551615`, after, id, before)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision&lt;18446744073709551615`, after, id, before)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3266:3861 -->
````go
// 与该文档index_application同事务推进；ES或提交未知时必须保留原after重做。
func AdvanceSnapshot(ctx context.Context, tx *sql.Tx, id, before, after uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision<18446744073709551615`, after, id, before)
	if err != nil {
		return errors.New("SEARCH_REBUILD_PROGRESS_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}

````

## 第 7 步：FinishSnapshot

插入位置：第 6 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`func FinishSnapshot(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;REPLAYING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;SNAPSHOT&#x27;`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 原重建job主键。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;REPLAYING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;SNAPSHOT&#x27;`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_job SET state=&#x27;REPLAYING&#x27;,last_error=NULL WHERE id=? AND state=&#x27;SNAPSHOT&#x27;`, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3861:4278 -->
````go
func FinishSnapshot(ctx context.Context, tx *sql.Tx, id uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_job SET state='REPLAYING',last_error=NULL WHERE id=? AND state='SNAPSHOT'`, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_PROGRESS_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/rebuild/installation_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
