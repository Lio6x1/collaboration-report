# 分步编写 `services/document/internal/repository/document/readings_queries.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/169-readings-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取不可变结构、样式、清单和正文，并登记固定阅读视图。

一页目录及后续单元正文必须属于同一manifest，否则用户会看到混合版本。

写之前先按这条执行路线理解各段的连接：

1. ManifestRevision按document与manifest取得依赖
2. StructureRow/StyleRow/ContentRow保持文档归属
3. InsertReadView固定阅读
4. ReadViewVersion按固定manifest定位单元版本。

本文件需要守住的边界：

- 所有依赖都需校验document
- 持当前来源与文档锁可防止读取过程中历史被清理。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/readings_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/readings_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/readings_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/readings_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:360 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// manifest_read.go：保留原实现，按业务合并文件。

````

## 第 2 步：ManifestRevision

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

清单、结构、样式均不可变；调用方持当前文档和来源锁，历史清理与本次
查询共享同一事务边界。不能只按可猜到的manifest ID跨文档取依赖。

本步声明：`func ManifestRevision(ctx context.Context, tx *sql.Tx, document, manifest uint64) (structure, style string, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?&quot;, manifest, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?&quot;, manifest, document)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?&quot;, manifest, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>structure</code> | <code>string</code> | 实际返回表达式：<code>structure</code> |
| <code>style</code> | <code>string</code> | 实际返回表达式：<code>style</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?&quot;, manifest, document).Scan(&amp;structure, &amp;style)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?&quot;, manifest, document)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=360:929 -->
````go
// 清单、结构、样式均不可变；调用方持当前文档和来源锁，历史清理与本次
// 查询共享同一事务边界。不能只按可猜到的manifest ID跨文档取依赖。
func ManifestRevision(ctx context.Context, tx *sql.Tx, document, manifest uint64) (structure, style string, err error) {
	err = tx.QueryRowContext(ctx, "SELECT CAST(structure_id AS CHAR),CAST(style_id AS CHAR) FROM main_manifest WHERE id=? AND document_id=?", manifest, document).Scan(&structure, &style)
	return
}

// read.go：保留原实现，按业务合并文件。

````

## 第 3 步：StructureRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func StructureRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StructureRevision, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document)</code>；<code>tx.QueryContext(ctx, &quot;SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.QueryContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document)</code>；<code>tx.QueryContext(ctx, &quot;SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document)</code>；<code>tx.QueryContext(ctx, &quot;SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.StructureRevision</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;STRUCTURE_NOT_FOUND&quot;)</code>；<code>err</code>；<code>errors.New(&quot;STRUCTURE_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document).Scan(&amp;value.Id, &amp;valu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return value, errors.New(&quot;STRUCTURE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document).Scan(&amp;value.Id, &amp;value.DocumentId, &amp;value.SchemaVersion)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?&quot;, id, document)</code>。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;entry.NodeId, &amp;entry.ParentNodeId, &amp;entry.ChapterId, &amp;entry.UnitId, &amp;entry.Kind, &amp;entry.Title, &amp;entry.SortOrder, &amp;entry.SplitLevel2) != nil</code>。条件成立时执行 <code>return value, errors.New(&quot;STRUCTURE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;entry.NodeId, &amp;entry.ParentNodeId, &amp;entry.ChapterId, &amp;entry.UnitId, &amp;entry.Kind, &amp;entry.Title, &amp;entry.SortOrder, &amp;entry.SplitLevel2)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return value, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=929:2078 -->
````go
func StructureRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StructureRevision, error) {
	var value dto.StructureRevision
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version FROM structure_revision WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.SchemaVersion) != nil {
		return value, errors.New("STRUCTURE_NOT_FOUND")
	}
	value.Entries = []dto.StructureEntry{}
	rows, err := tx.QueryContext(ctx, "SELECT CAST(node_id AS CHAR),CAST(parent_node_id AS CHAR),CAST(chapter_id AS CHAR),CAST(unit_id AS CHAR),kind,title,sort_order,split_mode FROM structure_entry WHERE structure_id=? AND document_id=? ORDER BY parent_sort_key,sort_order", id, document)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry dto.StructureEntry
		if rows.Scan(&entry.NodeId, &entry.ParentNodeId, &entry.ChapterId, &entry.UnitId, &entry.Kind, &entry.Title, &entry.SortOrder, &entry.SplitLevel2) != nil {
			return value, errors.New("STRUCTURE_UNAVAILABLE")
		}
		value.Entries = append(value.Entries, entry)
	}
	return value, rows.Err()
}
````

## 第 4 步：StyleRow

插入位置：第 3 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func StyleRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StyleRevision, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document)</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.StyleRevision</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;STYLE_NOT_FOUND&quot;)</code>；<code>json.Unmarshal(raw, &amp;value.Style)</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document).Scan(&amp;value.Id …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return value, errors.New(&quot;STYLE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document).Scan(&amp;value.Id …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?&quot;, id, document)</code>。
- 此处通过 <code>return value, json.Unmarshal(raw, &amp;value.Style)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2078:2562 -->
````go
func StyleRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.StyleRevision, error) {
	var value dto.StyleRevision
	var raw []byte
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),schema_version,style_json FROM style_revision WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.SchemaVersion, &raw) != nil {
		return value, errors.New("STYLE_NOT_FOUND")
	}
	return value, json.Unmarshal(raw, &value.Style)
}
````

## 第 5 步：ContentRow

插入位置：第 4 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func ContentRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=? AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.QueryContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=? AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, id)</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, &quot;SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=? AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.FixedContent</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CONTENT_NOT_FOUND&quot;)</code>；<code>errors.New(&quot;CONTENT_SCHEMA_INVALID&quot;)</code>；<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return value, errors.New(&quot;CONTENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_h …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>json.Unmarshal(body, &amp;value.BodyJson) != nil &#124;&#124; json.Unmarshal(provenance, &amp;value.Provenance) != nil</code>。条件成立时执行 <code>return value, errors.New(&quot;CONTENT_SCHEMA_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=? AND purpose=&#x27;CONTENT&#x27; ORDER BY resource_id&quot;, document, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;resource)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return value, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2562:4078 -->
````go
func ContentRow(ctx context.Context, tx *sql.Tx, id, document string) (dto.FixedContent, error) {
	var value dto.FixedContent
	var body, hash, provenance []byte
	if tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(unit_id AS CHAR),CAST(structure_id AS CHAR),schema_version,canonicalization_version,body_json,body_hash,CAST(byte_size AS CHAR),provenance_json FROM content_version WHERE id=? AND document_id=?", id, document).Scan(&value.Id, &value.DocumentId, &value.UnitId, &value.StructureId, &value.SchemaVersion, &value.CanonicalizationVersion, &body, &hash, &value.ByteSize, &provenance) != nil {
		return value, errors.New("CONTENT_NOT_FOUND")
	}
	if json.Unmarshal(body, &value.BodyJson) != nil || json.Unmarshal(provenance, &value.Provenance) != nil {
		return value, errors.New("CONTENT_SCHEMA_INVALID")
	}
	value.BodyHash = hex.EncodeToString(hash)
	value.ResourceIds = []string{}
	// 引用属于不可变 VERSION，不能从当前草稿、主线清单或资源上传者反推固定正文的引用。
	rows, err := tx.QueryContext(ctx, "SELECT CAST(resource_id AS CHAR) FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=? AND purpose='CONTENT' ORDER BY resource_id", document, id)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var resource string
		if err = rows.Scan(&resource); err != nil {
			return value, err
		}
		value.ResourceIds = append(value.ResourceIds, resource)
	}
	return value, rows.Err()
}
````

## 第 6 步：ManifestUnits

插入位置：第 5 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func ManifestUnits(ctx context.Context, tx *sql.Tx, manifest string, document uint64) ([]map[string]string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id&quot;, manifest, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>manifest</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id&quot;, manifest, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id&quot;, manifest, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;MANIFEST_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id&quot;, manifest, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;unit, &amp;version) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;MANIFEST_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;unit, &amp;version)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return units, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4078:4743 -->
````go
func ManifestUnits(ctx context.Context, tx *sql.Tx, manifest string, document uint64) ([]map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT CAST(unit_id AS CHAR),CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? ORDER BY unit_id", manifest, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := []map[string]string{}
	for rows.Next() {
		var unit, version string
		if rows.Scan(&unit, &version) != nil {
			return nil, errors.New("MANIFEST_UNAVAILABLE")
		}
		units = append(units, map[string]string{"unit_id": unit, "content_version_id": version})
	}
	return units, rows.Err()
}
````

## 第 7 步：InsertReadView

插入位置：第 6 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`func InsertReadView(ctx context.Context, tx *sql.Tx, view domain.ReadView) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO read_view(view_id,document_id,manifest_id,admission_id,expires_at,state) VALUES(?,?,?,?,?,&#x27;OPEN&#x27;)&quot;, view.ID, view.Document, view.Manifest, view.Admission, view.Expires)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>view</code> | <code>domain.ReadView</code> | 使用 <code>domain.ReadView</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>在此声明中的实际使用：<code>view.ID</code>；<code>view.Document</code>；<code>view.Manifest</code>；<code>view.Admission</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO read_view(view_id,document_id,manifest_id,admission_id,expires_at,state) VALUES(?,?,?,?,?,&#x27;OPEN&#x27;)&quot;, view.ID, view.Document, view.Manifest, view.Admission, view.Expires)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4743:5053 -->
````go
func InsertReadView(ctx context.Context, tx *sql.Tx, view domain.ReadView) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO read_view(view_id,document_id,manifest_id,admission_id,expires_at,state) VALUES(?,?,?,?,?,'OPEN')", view.ID, view.Document, view.Manifest, view.Admission, view.Expires)
	return err
}
````

## 第 8 步：LockedReadView

插入位置：第 7 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`func LockedReadView(ctx context.Context, tx *sql.Tx, view string, document uint64) (domain.FixedReadView, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>view</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.FixedReadView</code> | 使用 <code>domain.FixedReadView</code> 类型；其定义处的职责：定义工作区初始正文、固定阅读视图及章节插入算法。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5053:5597 -->
````go
func LockedReadView(ctx context.Context, tx *sql.Tx, view string, document uint64) (domain.FixedReadView, error) {
	var value domain.FixedReadView
	err := tx.QueryRowContext(ctx, "SELECT v.manifest_id,a.actor_id,a.session_ref,v.state,a.state,LEAST(v.expires_at,a.expires_at) FROM read_view v JOIN document_admission a ON a.admission_id=v.admission_id WHERE v.view_id=? AND v.document_id=? FOR UPDATE", view, document).Scan(&value.Manifest, &value.Actor, &value.Session, &value.State, &value.AdmissionState, &value.Expires)
	return value, err
}
````

## 第 9 步：ReadViewVersion

插入位置：第 8 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func ReadViewVersion(ctx context.Context, tx *sql.Tx, manifest, document, unit uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit).Scan(&amp;version)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?&quot;, manifest, document, unit)</code>。
- 此处通过 <code>return version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5597:5933 -->
````go
func ReadViewVersion(ctx context.Context, tx *sql.Tx, manifest, document, unit uint64) (string, error) {
	var version string
	err := tx.QueryRowContext(ctx, "SELECT CAST(content_version_id AS CHAR) FROM manifest_entry WHERE manifest_id=? AND document_id=? AND unit_id=?", manifest, document, unit).Scan(&version)
	return version, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/readings_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
