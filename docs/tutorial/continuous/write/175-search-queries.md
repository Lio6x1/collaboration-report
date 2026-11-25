# 分步编写 `services/document/internal/repository/document/search_queries.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/175-search-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

批量读取候选的正式权威事实、清单字节元数据，并按排序锁联合来源。

逐候选换事务会拼接不同时刻的权限；一页最终核验需要同一快照与统一锁序。

写之前先按这条执行路线理解各段的连接：

1. SearchPublicationFacts批查Doc/KB/Space
2. SearchManifestMetadata仅取大小和归属
3. LockSourceGatesBatch取得排他gate
4. PendingSourceTransitionsBatch在锁后当前读。

本文件需要守住的边界：

- 未知SQL不能降级成拒绝候选
- 不能INSERT IGNORE触发共享锁升级
- 来源超过SQL预算整页失败不截断。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/search_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/search_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/search_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/search_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:371 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// search_facts.go：保留原实现，按业务合并文件。

````

## 第 2 步：SearchPublicationFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

SearchPublicationFacts只使用调用方原事务。一批候选由用例在同一只读一致快照下
加载；不能逐项另开事务，也不能把未知KB/Space/SQL事实降级为确定拒绝候选。

本步声明：`func SearchPublicationFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>searchPublicationFactsBatch(ctx, tx, ids)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>searchPublicationFactsBatch(ctx, tx, ids)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>searchPublicationFactsBatch(ctx, tx, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;domain.PublicationFacts</code> | 由 <code>searchPublicationFactsBatch(ctx, tx, ids)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>searchPublicationFactsBatch(ctx, tx, ids)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalid</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; len(ids) &gt; 100</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; seen&#91;id&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return searchPublicationFactsBatch(ctx, tx, ids)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=371:1254 -->
````go
// SearchPublicationFacts只使用调用方原事务。一批候选由用例在同一只读一致快照下
// 加载；不能逐项另开事务，也不能把未知KB/Space/SQL事实降级为确定拒绝候选。
func SearchPublicationFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error) {
	invalid := domain.Violation(domain.InvalidRequest, "SEARCH_PUBLICATION_FACTS_INVALID")
	if tx == nil || len(ids) > 100 {
		return nil, invalid
	}
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			return nil, invalid
		}
		seen[id] = true
	}
	// 保留同一原快照及完整事实，只将N项Doc/KB/Space往返合并为至多三次查询。
	// 最终授权事务仍逐行当前读/锁定；这里的批量读取不能替代最终资格复核。
	return searchPublicationFactsBatch(ctx, tx, ids)
}

````

## 第 3 步：SearchManifestMetadata

插入位置：第 2 步代码之后，紧接追加；完成稿约第 36 行。

SearchManifestMetadata不读取canonical_body/body_json/provenance。用例必须先以
固定structure及ManifestUnits证明清单完整，再将这里的ByteSize交给预算规则；
预留成功后才读正文并严格验证实际len/hash/schema，不能让错误byte_size绕过预算。

本步声明：`func SearchManifestMetadata(ctx context.Context, tx *sql.Tx, manifest, document uint64, expectedUnits int) ([]domain.SearchManifestUnit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?&quot;, manifest)</code>；<code>tx.QueryContext(ctx, `SELECT CAST(e.document_id AS CHAR),CAST(e.unit_id AS CHAR), 		CAST(e.content_version_id AS CHAR),CAST(v.document_id AS CHAR),CAST(v.unit_id AS CHAR),CAST(v.by …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryRowContext</code>；<code>tx.QueryContext</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>manifest == 0</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?&quot;, manifest)</code>；<code>tx.QueryContext(ctx, `SELECT CAST(e.document_id AS CHAR),CAST(e.unit_id AS CHAR), 		CAST(e.content_version_id AS CHAR),CAST(v.document_id AS CHAR),CAST(v.unit_id AS CHAR),CAST(v.by …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>ownerID != document</code>；<code>entryDocument != document</code>；<code>contentDocument != document</code> |
| <code>expectedUnits</code> | <code>int</code> | 在此声明中的实际使用：<code>expectedUnits &lt; 0</code>；<code>expectedUnits &gt; domain.SearchManifestMaxUnits</code>；<code>expectedUnits+1</code>；<code>make(&#91;&#93;domain.SearchManifestUnit, 0, expectedUnits)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.SearchManifestUnit</code> | 使用 <code>domain.SearchManifestUnit</code> 类型；其定义处的职责：SearchManifestUnit是Doc私有固定清单的元数据；ByteSize用于读取正文之前预留预算。 它不包含正文/摘要或RPC DTO，读取后仍须验证实际字节、hash、schema与完整单位绑定。<br>实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; manifest == 0 &#124;&#124; document == 0 &#124;&#124; expectedUnits &lt; 0 &#124;&#124; expectedUnits &gt; domain.SearchManifestMaxUnits</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?&quot;, manifest).Scan(&amp;owner)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?&quot;, manifest)</code>。
- 检查 <code>err != nil &#124;&#124; ownerID != document</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT CAST(e.document_id AS CHAR),CAST(e.unit_id AS CHAR), 		CAST(e.content_version_id AS CHAR),CAST(v.document_id AS CHAR),CAST(v.unit_id AS CHAR),CAST(v.by …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>len(units) &gt;= expectedUnits</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>rows.Scan(&amp;entryDoc, &amp;entryUnit, &amp;entryVersion, &amp;versionDoc, &amp;versionUnit, &amp;versionBytes) != nil &#124;&#124; 			!versionDoc.Valid &#124;&#124; !versionUnit.Valid &#124;&#124; !versionBytes.Valid</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;entryDoc, &amp;entryUnit, &amp;entryVersion, &amp;versionDoc, &amp;versionUnit, &amp;versionBytes)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。

<!-- write-step: 3 bytes=1254:3784 -->
````go
// SearchManifestMetadata不读取canonical_body/body_json/provenance。用例必须先以
// 固定structure及ManifestUnits证明清单完整，再将这里的ByteSize交给预算规则；
// 预留成功后才读正文并严格验证实际len/hash/schema，不能让错误byte_size绕过预算。
func SearchManifestMetadata(ctx context.Context, tx *sql.Tx, manifest, document uint64, expectedUnits int) ([]domain.SearchManifestUnit, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_METADATA_INVALID")
	if tx == nil || manifest == 0 || document == 0 || expectedUnits < 0 || expectedUnits > domain.SearchManifestMaxUnits {
		return nil, invalid
	}
	// 即使固定结构为空也核验manifest归属；缺manifest不能成为零单位成功。
	var owner string
	if err := tx.QueryRowContext(ctx, "SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?", manifest).Scan(&owner); err != nil {
		return nil, invalid
	}
	ownerID, err := domain.CanonicalID(owner)
	if err != nil || ownerID != document {
		return nil, invalid
	}
	// 不按e.document_id筛选，否则同manifest的异常跨Doc额外行会被隐藏；LEFT JOIN
	// 保留缺失版本事实。多读一条即可证明全清单超数，异常清单仍不无限加载。
	rows, err := tx.QueryContext(ctx, `SELECT CAST(e.document_id AS CHAR),CAST(e.unit_id AS CHAR),
		CAST(e.content_version_id AS CHAR),CAST(v.document_id AS CHAR),CAST(v.unit_id AS CHAR),CAST(v.byte_size AS CHAR)
		FROM manifest_entry e LEFT JOIN content_version v ON v.id=e.content_version_id
		WHERE e.manifest_id=? ORDER BY e.unit_id LIMIT ?`, manifest, expectedUnits+1)
	if err != nil {
		return nil, invalid
	}
	defer rows.Close()
	units := make([]domain.SearchManifestUnit, 0, expectedUnits)
	seen := make(map[uint64]bool, expectedUnits)
	for rows.Next() {
		if len(units) >= expectedUnits {
			return nil, invalid
		}
		var entryDoc, entryUnit, entryVersion string
		var versionDoc, versionUnit, versionBytes sql.NullString
		if rows.Scan(&entryDoc, &entryUnit, &entryVersion, &versionDoc, &versionUnit, &versionBytes) != nil ||
			!versionDoc.Valid || !versionUnit.Valid || !versionBytes.Valid {
			return nil, invalid
		}
		entryDocument, e1 := domain.CanonicalID(entryDoc)
		unit, e2 := domain.CanonicalID(entryUnit)
		version, e3 := domain.CanonicalID(entryVersion)
		contentDocument, e4 := domain.CanonicalID(versionDoc.String)
		contentUnit, e5 := domain.CanonicalID(versionUnit.String)
		byteSize, e6 := domain.CanonicalID(versionBytes.String)
````

## 第 4 步：SearchManifestMetadata（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 81 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>e1 != nil &#124;&#124; e2 != nil &#124;&#124; e3 != nil &#124;&#124; e4 != nil &#124;&#124; e5 != nil &#124;&#124; e6 != nil &#124;&#124; 			entryDocument != document &#124;&#124; contentDocument != document &#124;&#124; contentUnit != unit &#124;&#124; seen&#91;unit&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil &#124;&#124; len(units) != expectedUnits</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return units, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3784:4290 -->
````go
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil ||
			entryDocument != document || contentDocument != document || contentUnit != unit || seen[unit] {
			return nil, invalid
		}
		seen[unit] = true
		units = append(units, domain.SearchManifestUnit{Unit: unit, Version: version, ByteSize: byteSize})
	}
	if rows.Err() != nil || len(units) != expectedUnits {
		return nil, invalid
	}
	return units, nil
}

// search_facts_batch.go：保留原实现，按业务合并文件。

````

## 第 5 步：searchFactRows

插入位置：第 4 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func searchFactRows(ctx context.Context, tx *sql.Tx, table, columns string, ids []uint64) (*sql.Rows, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM &quot;+table+&quot; WHERE id IN (&quot;+marks+&quot;) ORDER BY id&quot;, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>table</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;SELECT &quot;+columns+&quot; FROM &quot;+table</code> |
| <code>columns</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;SELECT &quot;+columns</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*sql.Rows</code> | 由 <code>tx.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM &quot;+table+&quot; WHERE id IN (&quot;+marks+&quot;) ORDER BY id&quot;, args...)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>tx.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM &quot;+table+&quot; WHERE id IN (&quot;+marks+&quot;) ORDER BY id&quot;, args...)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM &quot;+table+&quot; WHERE id IN (&quot;+marks+&quot;) ORDER BY id&quot;, args...)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM &quot;+table+&quot; WHERE id IN (&quot;+marks+&quot;) ORDER BY id&quot;, args...)</code>。

<!-- write-step: 5 bytes=4290:4766 -->
````go
func searchFactRows(ctx context.Context, tx *sql.Tx, table, columns string, ids []uint64) (*sql.Rows, error) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	// table/columns只取本文件固定常量；所有候选与关联ID用参数绑定，最多原批100项。
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	return tx.QueryContext(ctx, "SELECT "+columns+" FROM "+table+" WHERE id IN ("+marks+") ORDER BY id", args...)
}

````

## 第 6 步：sortedSearchFactIDs

插入位置：第 5 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func sortedSearchFactIDs(ids map[uint64]bool) []uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ids</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>len(ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>ordered</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93; &lt; ordered&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ordered</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4766:5017 -->
````go
func sortedSearchFactIDs(ids map[uint64]bool) []uint64 {
	ordered := make([]uint64, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered
}

````

## 第 7 步：searchPublicationFactsBatch

插入位置：第 6 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`func searchPublicationFactsBatch(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;document&quot;, publicationDocumentColumns, ids)</code>；<code>searchKnowledgeBaseFacts(ctx, tx, sortedSearchFactIDs(kbIDs))</code>；<code>searchSpaceFacts(ctx, tx, sortedSearchFactIDs(spaceIDs))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;document&quot;, publicationDocumentColumns, ids)</code>；<code>searchKnowledgeBaseFacts(ctx, tx, sortedSearchFactIDs(kbIDs))</code>；<code>searchSpaceFacts(ctx, tx, sortedSearchFactIDs(spaceIDs))</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>searchFactRows(ctx, tx, &quot;document&quot;, publicationDocumentColumns, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;domain.PublicationFacts</code> | 实际返回表达式：<code>facts</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>unknown</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>；<code>err</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>len(ids) == 0</code>。条件成立时执行 <code>return facts, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; e1 != nil &#124;&#124; e2 != nil</code>。
- 此处通过 <code>return nil, unknown</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>d.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_KNOWLEDGE_BASE_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>readErr != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=5017:6482 -->
````go
func searchPublicationFactsBatch(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error) {
	facts := make(map[uint64]domain.PublicationFacts, len(ids))
	if len(ids) == 0 {
		return facts, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_PUBLICATION_FACTS_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "document", publicationDocumentColumns, ids)
	if err != nil {
		return nil, unknown
	}
	kbIDs, spaceIDs := make(map[uint64]bool), make(map[uint64]bool)
	for rows.Next() {
		d, sequence, err := scanPublicationDocument(rows)
		id, e1 := domain.CanonicalID(d.Id)
		space, e2 := domain.CanonicalID(d.SpaceId)
		if err != nil || e1 != nil || e2 != nil {
			rows.Close()
			return nil, unknown
		}
		facts[id] = domain.PublicationFacts{Document: d, Sequence: sequence}
		spaceIDs[space] = true
		if d.KnowledgeBaseId != nil {
			kb, err := domain.CanonicalID(*d.KnowledgeBaseId)
			if err != nil {
				rows.Close()
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
			}
			kbIDs[kb] = true
		}
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, unknown
	}
	kbs, err := searchKnowledgeBaseFacts(ctx, tx, sortedSearchFactIDs(kbIDs))
	if err != nil {
		return nil, err
	}
	spaces, err := searchSpaceFacts(ctx, tx, sortedSearchFactIDs(spaceIDs))
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		value, present := facts[id]
````

## 第 8 步：searchPublicationFactsBatch（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 160 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!present</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>value.Document.KnowledgeBaseId != nil</code>。
- 检查 <code>!present &#124;&#124; kb.SpaceId != value.Document.SpaceId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!present</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_SPACE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6482:7191 -->
````go
		if !present {
			continue // 确定缺Doc仅排除该候选；关联缺失/SQL未知使整批失败。
		}
		if value.Document.KnowledgeBaseId != nil {
			kbID, _ := domain.CanonicalID(*value.Document.KnowledgeBaseId)
			kb, present := kbs[kbID]
			if !present || kb.SpaceId != value.Document.SpaceId {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
			}
			value.KnowledgeBase = &kb
		}
		spaceID, _ := domain.CanonicalID(value.Document.SpaceId)
		space, present := spaces[spaceID]
		if !present {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
		}
		value.Space = space
		facts[id] = value
	}
	return facts, nil
}

````

## 第 9 步：searchKnowledgeBaseFacts

插入位置：第 8 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func searchKnowledgeBaseFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;knowledge_base&quot;, publicationKnowledgeBaseColumns, ids)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;knowledge_base&quot;, publicationKnowledgeBaseColumns, ids)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>searchFactRows(ctx, tx, &quot;knowledge_base&quot;, publicationKnowledgeBaseColumns, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;dto.KnowledgeBase</code> | 实际返回表达式：<code>values</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>unknown</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(ids) == 0</code>。条件成立时执行 <code>return values, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; invalid != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7191:7932 -->
````go
func searchKnowledgeBaseFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.KnowledgeBase, error) {
	values := make(map[uint64]dto.KnowledgeBase, len(ids))
	if len(ids) == 0 {
		return values, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "knowledge_base", publicationKnowledgeBaseColumns, ids)
	if err != nil {
		return nil, unknown
	}
	defer rows.Close()
	for rows.Next() {
		value, err := scanPublicationKnowledgeBase(rows)
		id, invalid := domain.CanonicalID(value.Id)
		if err != nil || invalid != nil {
			return nil, unknown
		}
		values[id] = value
	}
	if rows.Err() != nil {
		return nil, unknown
	}
	return values, nil
}

````

## 第 10 步：searchSpaceFacts

插入位置：第 9 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`func searchSpaceFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;space&quot;, publicationSpaceColumns, ids)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>searchFactRows(ctx, tx, &quot;space&quot;, publicationSpaceColumns, ids)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>searchFactRows(ctx, tx, &quot;space&quot;, publicationSpaceColumns, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;dto.Space</code> | 实际返回表达式：<code>values</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>unknown</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(ids) == 0</code>。条件成立时执行 <code>return values, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; invalid != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return values, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=7932:8682 -->
````go
func searchSpaceFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.Space, error) {
	values := make(map[uint64]dto.Space, len(ids))
	if len(ids) == 0 {
		return values, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "space", publicationSpaceColumns, ids)
	if err != nil {
		return nil, unknown
	}
	defer rows.Close()
	for rows.Next() {
		value, err := scanPublicationSpace(rows)
		id, invalid := domain.CanonicalID(value.Id)
		if err != nil || invalid != nil {
			return nil, unknown
		}
		values[id] = value
	}
	if rows.Err() != nil {
		return nil, unknown
	}
	return values, nil
}

// search_sources.go：保留原实现，按业务合并文件。

````

## 第 11 步：maxSearchOutputSources

插入位置：第 10 步代码之后，紧接追加；完成稿约第 234 行。

一页最多100项及一个lookahead；来源为Doc/KB和身份的去重并集。
上限只约束SQL包大小，超限整页失败，不截断任何权限来源。

本步声明：`const maxSearchOutputSources = 1024`。

<!-- write-step: 11 bytes=8682:8889 -->
````go
// 一页最多100项及一个lookahead；来源为Doc/KB和身份的去重并集。
// 上限只约束SQL包大小，超限整页失败，不截断任何权限来源。
const maxSearchOutputSources = 1024

````

## 第 12 步：validOrderedSearchSources

插入位置：第 11 步代码之后，紧接追加；完成稿约第 238 行。

本步声明：`func validOrderedSearchSources(sources []domain.LeaseSource) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>sources</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>len(sources)</code>；<code>sources&#91;i-1&#93;.Key</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(sources) == 0 &#124;&#124; len(sources) &gt; maxSearchOutputSources</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; (!platform.ValidDocumentScope(source.Key) &amp;&amp; !platform.ValidAccountOutputScope(source.Key))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>i &gt; 0 &amp;&amp; sources&#91;i-1&#93;.Key &gt;= source.Key</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=8889:9337 -->
````go
func validOrderedSearchSources(sources []domain.LeaseSource) bool {
	if len(sources) == 0 || len(sources) > maxSearchOutputSources {
		return false
	}
	for i, source := range sources {
		if _, err := domain.CanonicalID(source.Epoch); err != nil || (!platform.ValidDocumentScope(source.Key) && !platform.ValidAccountOutputScope(source.Key)) {
			return false
		}
		if i > 0 && sources[i-1].Key >= source.Key {
			return false
		}
	}
	return true
}

````

## 第 13 步：LockSourceGatesBatch

插入位置：第 12 步代码之后，紧接追加；完成稿约第 253 行。

调用方原事务持有整批排他gate锁；按与旧逐项路径相同的scope顺序取得锁。
不能用INSERT IGNORE（共享锁升级），也不能覆盖已有BLOCKED或更高epoch。

本步声明：`func LockSourceGatesBatch(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource) (map[string]SourceGate, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES&quot;+strings.Join(values, &quot;,&quot;)+&quot; ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, args...)</code>；<code>tx.QueryContext(ctx, &quot;SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (&quot;+marks+&quot;) ORDER BY scope_key FOR UPDATE&quot;, keys...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.ExecContext</code>；<code>tx.QueryContext</code> |
| <code>sources</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>validOrderedSearchSources(sources)</code>；<code>len(sources)</code>；<code>sources&#91;len(gates)&#93;.Key</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;SourceGate</code> | 实际返回表达式：<code>nil</code>；<code>gates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;GATE_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; !validOrderedSearchSources(sources)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES&quot;+strings.Join(values, &quot;,&quot;)+&quot; ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, args...)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN (&quot;+marks+&quot;) ORDER BY scope_key FOR UPDATE&quot;, keys...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;key, &amp;gate.Epoch, &amp;gate.State) != nil &#124;&#124; len(gates) &gt;= len(sources) &#124;&#124; key != sources&#91;len(gates)&#93;.Key &#124;&#124; gate.Epoch == 0 &#124;&#124; (gate.State != &quot;OPEN&quot; &amp;&amp; gate.State != &quot;BLOCKED&quot;)</code>。条件成立时执行 <code>return nil, errors.New(&quot;GATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;key, &amp;gate.Epoch, &amp;gate.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>rows.Err() != nil &#124;&#124; len(gates) != len(sources)</code>。条件成立时执行 <code>return nil, errors.New(&quot;GATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return gates, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9337:11039 -->
````go
// 调用方原事务持有整批排他gate锁；按与旧逐项路径相同的scope顺序取得锁。
// 不能用INSERT IGNORE（共享锁升级），也不能覆盖已有BLOCKED或更高epoch。
func LockSourceGatesBatch(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource) (map[string]SourceGate, error) {
	if tx == nil || !validOrderedSearchSources(sources) {
		return nil, errors.New("SOURCE_BINDING_INVALID")
	}
	values, keys := make([]string, len(sources)), make([]any, len(sources))
	args := make([]any, 0, len(sources)*2)
	for i, source := range sources {
		values[i] = "(?,?,'OPEN')"
		args = append(args, source.Key, source.Epoch)
		keys[i] = source.Key
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES"+strings.Join(values, ",")+" ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", args...); err != nil {
		return nil, err
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	rows, err := tx.QueryContext(ctx, "SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN ("+marks+") ORDER BY scope_key FOR UPDATE", keys...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	gates := make(map[string]SourceGate, len(sources))
	for rows.Next() {
		var key string
		var gate SourceGate
		if rows.Scan(&key, &gate.Epoch, &gate.State) != nil || len(gates) >= len(sources) || key != sources[len(gates)].Key || gate.Epoch == 0 || (gate.State != "OPEN" && gate.State != "BLOCKED") {
			return nil, errors.New("GATE_UNAVAILABLE")
		}
		gates[key] = gate
	}
	if rows.Err() != nil || len(gates) != len(sources) {
		return nil, errors.New("GATE_UNAVAILABLE")
	}
	return gates, nil
}

````

## 第 14 步：PendingSourceTransitionsBatch

插入位置：第 13 步代码之后，紧接追加；完成稿约第 290 行。

必须在全部gate排他锁之后调用。使用scope索引当前读，不能读事务旧快照，
也不能让优化器走state索引先锁无关过渡；RR仍可能取得gap/next-key锁。

本步声明：`func PendingSourceTransitionsBatch(ctx context.Context, tx *sql.Tx, keys []string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change FORCE INDEX(auth_scope_epoch) WHERE scope_key IN (&quot;+marks+&quot;) AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY scop …</code>（长表达式，按本步完整代码继续）；<code>PersonalTransferSourcesPending(ctx, tx, keys)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryRowContext</code>；<code>PersonalTransferSourcesPending(ctx, tx, keys)</code> |
| <code>keys</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(keys)</code>；<code>keys&#91;i-1&#93; &gt;= key</code>；<code>PersonalTransferSourcesPending(ctx, tx, keys)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>PersonalTransferSourcesPending(ctx, tx, keys)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>false</code>；<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>PersonalTransferSourcesPending(ctx, tx, keys)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>；<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; len(keys) &gt; maxSearchOutputSources</code>。条件成立时执行 <code>return false, errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(keys) == 0</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>!platform.ValidDocumentScope(key) &#124;&#124; (i &gt; 0 &amp;&amp; keys&#91;i-1&#93; &gt;= key)</code>。条件成立时执行 <code>return false, errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change FORCE INDEX(auth_scope_epoch) WHERE scope_key IN (&quot;+marks+&quot;) AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY scop …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change FORCE INDEX(auth_scope_epoch) WHERE scope_key IN (&quot;+marks+&quot;) AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) ORDER BY scop …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return PersonalTransferSourcesPending(ctx, tx, keys)</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11039:12138 -->
````go
// 必须在全部gate排他锁之后调用。使用scope索引当前读，不能读事务旧快照，
// 也不能让优化器走state索引先锁无关过渡；RR仍可能取得gap/next-key锁。
func PendingSourceTransitionsBatch(ctx context.Context, tx *sql.Tx, keys []string) (bool, error) {
	if tx == nil || len(keys) > maxSearchOutputSources {
		return false, errors.New("SOURCE_BINDING_INVALID")
	}
	if len(keys) == 0 {
		return false, nil
	}
	args := make([]any, len(keys))
	for i, key := range keys {
		if !platform.ValidDocumentScope(key) || (i > 0 && keys[i-1] >= key) {
			return false, errors.New("SOURCE_BINDING_INVALID")
		}
		args[i] = key
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM authorization_change FORCE INDEX(auth_scope_epoch) WHERE scope_key IN ("+marks+") AND state NOT IN ('SUCCEEDED','REJECTED') ORDER BY scope_key,epoch LIMIT 1 FOR UPDATE", args...).Scan(&state)
	if err == sql.ErrNoRows {
		return PersonalTransferSourcesPending(ctx, tx, keys)
	}
	return err == nil, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/search_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
