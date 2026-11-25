# 分步编写 `services/document/internal/repository/snapshot/entries.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/188-entries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取一致快照完整文档元数据并保存可重放重建条目。

重建需要隐藏、未发布和零水位对象来纠正旧索引，不能只扫描PUBLIC。

写之前先按这条执行路线理解各段的连接：

1. CurrentPage始终使用建立S的同一RR事务
2. InsertEntries保存
3. StoredPage只重读SEALED任务原条目。

本文件需要守住的边界：

- 空页不能证明作业存在或已封存
- 文件丢失后不能重查可变document表生成不同快照。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/snapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/snapshot/entries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/snapshot/entries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/snapshot/entries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/snapshot/entries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:116 -->
````go
package snapshot

import (
	"collaboration/platform/rebuildmeta"
	"context"
	"database/sql"
	"errors"
	"strconv"
)

````

## 第 2 步：CurrentPage

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

CurrentPage必须一直使用建立S的同一个RepeatableRead事务。普通SELECT不取业务行锁，
不筛ACTIVE/PUBLIC；隐藏、未发布和零水位也必须进入完整的重建元数据。

本步声明：`func CurrentPage(ctx context.Context, tx *sql.Tx, after uint64, limit int) ([]rebuildmeta.Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id&gt;? ORDER BY id LIMIT ?`, after, limit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id&gt;? ORDER BY id LIMIT ?`, after, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 1</code>；<code>limit &gt; 500</code>；<code>tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id&gt;? ORDER BY id LIMIT ?`, after, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>由 <code>scanEntries(rows)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanEntries(rows)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>；<code>errors.New(&quot;REBUILD_SNAPSHOT_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 1 &#124;&#124; limit &gt; 500</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id&gt;? ORDER BY id LIMIT ?`, after, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_SNAPSHOT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return scanEntries(rows)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=116:787 -->
````go
// CurrentPage必须一直使用建立S的同一个RepeatableRead事务。普通SELECT不取业务行锁，
// 不筛ACTIVE/PUBLIC；隐藏、未发布和零水位也必须进入完整的重建元数据。
func CurrentPage(ctx context.Context, tx *sql.Tx, after uint64, limit int) ([]rebuildmeta.Entry, error) {
	if limit < 1 || limit > 500 {
		return nil, errors.New("REBUILD_PAGE_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,event_seq,CAST(current_release_id AS CHAR) FROM document WHERE id>? ORDER BY id LIMIT ?`, after, limit)
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_READ_FAILED")
	}
	defer rows.Close()
	return scanEntries(rows)
}

````

## 第 3 步：StoredPage

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

StoredPage只重读已封存任务的原条目，用于文件丢失后重生，不重查可变document表。
调用方须先在同事务Lock并确认SEALED；空页本身不能证明任务存在或已经封存。

本步声明：`func StoredPage(ctx context.Context, tx *sql.Tx, operation string, after uint64, limit int) ([]rebuildmeta.Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR) 		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id 		W …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR) 		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id 		W …</code>（长表达式，按本步完整代码继续） |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR) 		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id 		W …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 1</code>；<code>limit &gt; 500</code>；<code>tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR) 		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id 		W …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>由 <code>scanEntries(rows)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanEntries(rows)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>；<code>errors.New(&quot;REBUILD_ENTRIES_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 1 &#124;&#124; limit &gt; 500</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR) 		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id 		W …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_ENTRIES_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return scanEntries(rows)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=787:1652 -->
````go
// StoredPage只重读已封存任务的原条目，用于文件丢失后重生，不重查可变document表。
// 调用方须先在同事务Lock并确认SEALED；空页本身不能证明任务存在或已经封存。
func StoredPage(ctx context.Context, tx *sql.Tx, operation string, after uint64, limit int) ([]rebuildmeta.Entry, error) {
	if limit < 1 || limit > 500 {
		return nil, errors.New("REBUILD_PAGE_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.document_id,e.minimum_seq,CAST(e.release_hint AS CHAR)
		FROM search_snapshot_entry e JOIN search_snapshot_job j ON j.operation_id=e.operation_id
		WHERE e.operation_id=? AND j.state='SEALED' AND e.document_id>? ORDER BY e.document_id LIMIT ?`, operation, after, limit)
	if err != nil {
		return nil, errors.New("REBUILD_ENTRIES_READ_FAILED")
	}
	defer rows.Close()
	return scanEntries(rows)
}

````

## 第 4 步：scanEntries

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func scanEntries(rows *sql.Rows) ([]rebuildmeta.Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rows</code> | <code>*sql.Rows</code> | 在此声明中的实际使用：<code>rows.Next</code>；<code>rows.Scan</code>；<code>rows.Err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>；<code>errors.New(&quot;REBUILD_ENTRIES_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>rows.Scan(&amp;value.Document, &amp;value.Minimum, &amp;release) != nil &#124;&#124; value.Document == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.Document, &amp;value.Minimum, &amp;release)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>release.Valid</code>。
- 检查 <code>err != nil &#124;&#124; parsed == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REBUILD_ENTRIES_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1652:2323 -->
````go
func scanEntries(rows *sql.Rows) ([]rebuildmeta.Entry, error) {
	result := make([]rebuildmeta.Entry, 0)
	for rows.Next() {
		var value rebuildmeta.Entry
		var release sql.NullString
		if rows.Scan(&value.Document, &value.Minimum, &release) != nil || value.Document == 0 {
			return nil, errors.New("REBUILD_ENTRY_INVALID")
		}
		if release.Valid {
			parsed, err := strconv.ParseUint(release.String, 10, 64)
			if err != nil || parsed == 0 {
				return nil, errors.New("REBUILD_ENTRY_INVALID")
			}
			value.Release = parsed
		}
		result = append(result, value)
	}
	if rows.Err() != nil {
		return nil, errors.New("REBUILD_ENTRIES_READ_FAILED")
	}
	return result, nil
}

````

## 第 5 步：InsertEntries

插入位置：第 4 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func InsertEntries(ctx context.Context, tx *sql.Tx, operation string, entries []rebuildmeta.Entry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO search_snapshot_entry(operation_id,document_id,minimum_seq,release_hint) VALUES(?,?,?,?)`, operation, value.Document, value.Minimum, release)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO search_snapshot_entry(operation_id,document_id,minimum_seq,release_hint) VALUES(?,?,?,?)`, operation, value.Document, value.Minimum, release)</code> |
| <code>entries</code> | <code>&#91;&#93;rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>在此声明中的实际使用：<code>len(entries)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>；<code>errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>；<code>errors.New(&quot;REBUILD_ENTRY_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(entries) &gt; 500</code>。条件成立时执行 <code>return errors.New(&quot;REBUILD_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Document == 0</code>。条件成立时执行 <code>return errors.New(&quot;REBUILD_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;REBUILD_ENTRY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO search_snapshot_entry(operation_id,document_id,minimum_seq,release_hint) VALUES(?,?,?,?)`, operation, value.Document, value.Minimum, release)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2323:3066 -->
````go
func InsertEntries(ctx context.Context, tx *sql.Tx, operation string, entries []rebuildmeta.Entry) error {
	if len(entries) > 500 {
		return errors.New("REBUILD_PAGE_INVALID")
	}
	for _, value := range entries {
		if value.Document == 0 {
			return errors.New("REBUILD_ENTRY_INVALID")
		}
		var release any
		if value.Release != 0 {
			release = value.Release
		}
		// 不使用INSERT SELECT；写入已读取的值，避免把快照扫描变成当前锁定读取。
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_snapshot_entry(operation_id,document_id,minimum_seq,release_hint) VALUES(?,?,?,?)`, operation, value.Document, value.Minimum, release); err != nil {
			return errors.New("REBUILD_ENTRY_WRITE_FAILED")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/snapshot/entries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
