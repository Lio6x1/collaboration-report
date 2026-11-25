# 分步编写 `services/document/internal/repository/document/historical_ranges.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/155-historical-ranges.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

沿旧单元读取历史迁移映射，并取原结构动作终态与manifest绑定证据。

历史阅读资格不能仅信映射表声明，必须能回到真实成功动作、实际源目标版本。

写之前先按这条执行路线理解各段的连接：

1. HistoricalOutgoingMappings只沿指定单元取出边
2. HistoricalStructureBinding读取原终态与工件
3. HistoricalManifestBinding核文档、结构、清单和操作号。

本文件需要守住的边界：

- 跨文档坏边也应返回给核验器，不能通过JOIN悄悄丢掉反例造成错误证明。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/historical_ranges.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/historical_ranges.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/historical_ranges.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/historical_ranges.go' | Out-Null }
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

## 第 2 步：HistoricalUnitMapping

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`type HistoricalUnitMapping struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>UnitMapping</code> | <code>UnitMapping</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>SourceDocument</code> | <code>uint64</code> | 来源单元所属文档。 |
| <code>TargetDocument</code> | <code>uint64</code> | 目标单元所属文档；不能借跨文档映射越权。 |
| <code>Chapter</code> | <code>uint64</code> | 来源单元的章节归属。 |
| <code>TargetChapter</code> | <code>uint64</code> | 目标单元的章节归属，拆分仍须保留章节层次。 |
| <code>SourceRetired</code> | <code>uint64</code> | 来源单元退役时的结构版本。 |
| <code>TargetCreated</code> | <code>uint64</code> | 目标单元创建时的结构版本。 |
| <code>TargetKind</code> | <code>string</code> | 目标正文版本种类。 |
| <code>SourceKind</code> | <code>string</code> | 来源正文版本种类。 |
| <code>VersionKind</code> | <code>string</code> | 读取映射时查询到的版本种类，用于历史范围核验。 |

<!-- write-step: 2 bytes=57:298 -->
````go
type HistoricalUnitMapping struct {
	UnitMapping
	SourceDocument, TargetDocument, Chapter, TargetChapter uint64
	SourceRetired, TargetCreated                           uint64
	TargetKind, SourceKind, VersionKind                    string
}

````

## 第 3 步：HistoricalOutgoingMappings

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

沿指定旧单元读取出边，不遍历整库；跨文档边也必须显式返回给核验器，不能靠JOIN丢掉反例。

本步声明：`func HistoricalOutgoingMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]HistoricalUnitMapping, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,  l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.s …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,  l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.s …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,  l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.s …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;HistoricalUnitMapping</code> | 实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,  l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.s …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.Source, &amp;item.Target, &amp;item.SourceVersion, &amp;item.TargetVersion, &amp;item.Operation, &amp;item.Schema, &amp;item.Mapping, 			&amp;item.SourceHash, &amp;item.TargetHash, &amp;item.SourceStr …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return items, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=298:2129 -->
````go
// 沿指定旧单元读取出边，不遍历整库；跨文档边也必须显式返回给核验器，不能靠JOIN丢掉反例。
func HistoricalOutgoingMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]HistoricalUnitMapping, error) {
	rows, err := tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,
 l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.structure_id,t.structure_id,
 l.source_document_id,l.target_document_id,u.chapter_id,v.chapter_id,COALESCE(u.retired_structure_id,0),v.created_structure_id,
 v.kind,s.source_kind,t.source_kind
 FROM unit_lineage l
 JOIN collaboration_unit u ON u.id=l.source_unit_id AND u.document_id=l.source_document_id
 JOIN collaboration_unit v ON v.id=l.target_unit_id AND v.document_id=l.target_document_id
 JOIN content_version s ON s.id=l.source_version_id AND s.unit_id=l.source_unit_id AND s.document_id=l.source_document_id
 JOIN content_version t ON t.id=l.target_version_id AND t.unit_id=l.target_unit_id AND t.document_id=l.target_document_id
 WHERE l.source_document_id=? AND l.source_unit_id=? ORDER BY l.id LIMIT 10001`, document, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HistoricalUnitMapping{}
	for rows.Next() {
		var item HistoricalUnitMapping
		if err = rows.Scan(&item.Source, &item.Target, &item.SourceVersion, &item.TargetVersion, &item.Operation, &item.Schema, &item.Mapping,
			&item.SourceHash, &item.TargetHash, &item.SourceStructure, &item.TargetStructure, &item.SourceDocument, &item.TargetDocument,
			&item.Chapter, &item.TargetChapter, &item.SourceRetired, &item.TargetCreated, &item.TargetKind, &item.SourceKind, &item.VersionKind); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

````

## 第 4 步：HistoricalStructureBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

读取真实终态和两个原固定工件；不伪造恢复状态、不依赖原操作者当前在线，也不隐式提交。

本步声明：`func HistoricalStructureBinding(ctx context.Context, tx *sql.Tx, operation string) (PendingMainControlBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,  original.payload,fixed.payload FROM operation_receipt o  JOIN durable_ …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>Operation: operation</code>；<code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,  original.payload,fixed.payload FROM operation_receipt o  JOIN durable_ …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PendingMainControlBinding</code> | 使用 <code>PendingMainControlBinding</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,  original.payload,fixed.payload FROM operation_receipt o  JOIN durable_ …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,  original.payload,fixed.payload FROM operation_receipt o  JOIN durable_ …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2129:3205 -->
````go
// 读取真实终态和两个原固定工件；不伪造恢复状态、不依赖原操作者当前在线，也不隐式提交。
func HistoricalStructureBinding(ctx context.Context, tx *sql.Tx, operation string) (PendingMainControlBinding, error) {
	fact := PendingMainControlBinding{Kind: "MAIN_CONTROL_RECONCILE", Operation: operation}
	err := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json,o.result_json,
 original.payload,fixed.payload FROM operation_receipt o
 JOIN durable_task original ON original.kind='MAIN_CONTROL_CONTEXT' AND original.business_key=o.operation_id AND original.operation_id=o.operation_id AND original.state='SUCCEEDED'
 JOIN durable_task fixed ON fixed.kind='MAIN_COMMIT_FIXED' AND fixed.business_key=o.operation_id AND fixed.operation_id=o.operation_id AND fixed.state='SUCCEEDED'
 WHERE o.operation_id=? AND o.kind='STRUCTURE_CHANGE'`, operation).Scan(
		&fact.Actor, &fact.ReceiptKind, &fact.State, &fact.Fingerprint, &fact.Target, &fact.Result, &fact.Original, &fact.Payload)
	return fact, err
}

````

## 第 5 步：HistoricalManifestBinding

插入位置：第 4 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func HistoricalManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure uint64, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续） |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续） |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m  JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id  WHERE m.id=? AND m.document_id=? AN …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3205:3713 -->
````go
func HistoricalManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure uint64, operation string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest m
 JOIN structure_revision s ON s.id=m.structure_id AND s.document_id=m.document_id
 WHERE m.id=? AND m.document_id=? AND m.structure_id=? AND m.source_operation_id=? AND s.source_operation_id=?`, manifest, document, structure, operation, operation).Scan(&count)
	return count == 1, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/historical_ranges.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
