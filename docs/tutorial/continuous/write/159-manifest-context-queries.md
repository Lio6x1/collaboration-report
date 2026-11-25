# 分步编写 `services/document/internal/repository/document/manifest_context_queries.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/159-manifest-context-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

查询主线清单归属、bootstrap控制上下文和单元结构映射。

编辑房间的structure/style/control必须来自同一权威时刻，不能各自查询后拼装。

写之前先按这条执行路线理解各段的连接：

1. MainControlManifestBinding核不可变清单身份
2. BootstrapMainContext复用文档锁取当前标量
3. StructureUnitMappings读取指定单元的转换关系。

本文件需要守住的边界：

- bootstrap必须使用原事务，不能锁文档后另从连接池取不一致快照。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/manifest_context_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/manifest_context_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/manifest_context_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/manifest_context_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:281 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// control_manifest.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControlManifestBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func MainControlManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure, style uint64, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |
| <code>style</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation)</code>。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=281:753 -->
````go
func MainControlManifestBinding(ctx context.Context, tx *sql.Tx, document, manifest, structure, style uint64, operation string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main_manifest WHERE id=? AND document_id=? AND structure_id=? AND style_id=? AND source_operation_id=?`, manifest, document, structure, style, operation).Scan(&count)
	return count == 1, err
}

// main_context.go：保留原实现，按业务合并文件。

````

## 第 3 步：BootstrapMainContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

与bootstrap原文档锁共用事务，MAIN房间的结构/样式/control必须来自同一权威时刻。

本步声明：`func BootstrapMainContext(ctx context.Context, tx *sql.Tx, document uint64) (map[string]string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code>；<code>MainWorkspaceEpoch(ctx, tx, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>MainWorkspaceEpoch(ctx, tx, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code>；<code>MainWorkspaceEpoch(ctx, tx, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;string</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;string{&quot;structure_id&quot;: structure, &quot;style_id&quot;: style, &quot;control_revision&quot;: control, &quot;manifest_id&quot;: manifest}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHE …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;string{&quot;structure_id&quot;: structure, &quot;style_id&quot;: style, &quot;control_revision&quot;: control, &quot;manifest_id&quot;: manifest}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=753:1773 -->
````go
// 与bootstrap原文档锁共用事务，MAIN房间的结构/样式/control必须来自同一权威时刻。
func BootstrapMainContext(ctx context.Context, tx *sql.Tx, document uint64) (map[string]string, error) {
	var structure, style, control, manifest string
	err := tx.QueryRowContext(ctx, "SELECT CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(control_revision AS CHAR),CAST(current_manifest_id AS CHAR) FROM document WHERE id=? FOR UPDATE", document).Scan(&structure, &style, &control, &manifest)
	if err != nil {
		return nil, err
	}
	epoch, err := MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	// 改名或转移负责人不重播种可靠正文；这里交付真实MAIN物理上下文，管理修订仍留在Doc元数据。
	control = domain.TextID(epoch)
	return map[string]string{"structure_id": structure, "style_id": style, "control_revision": control, "manifest_id": manifest}, err
}

// unit_lineage.go：保留原实现，按业务合并文件。

````

## 第 4 步：UnitMapping

插入位置：第 3 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`type UnitMapping struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Source</code> | <code>uint64</code> | 拆合前的协作单元编号。 |
| <code>Target</code> | <code>uint64</code> | 拆合后的协作单元编号。 |
| <code>SourceVersion</code> | <code>uint64</code> | 映射来源的固定正文版本。 |
| <code>TargetVersion</code> | <code>uint64</code> | 映射目标的固定正文版本。 |
| <code>SourceStructure</code> | <code>uint64</code> | 来源正文所属结构版本。 |
| <code>TargetStructure</code> | <code>uint64</code> | 目标正文所属结构版本。 |
| <code>Operation</code> | <code>string</code> | 产生这条单元映射的结构变更操作号。 |
| <code>Schema</code> | <code>string</code> | 块映射格式版本；解析前核对，不能猜字段布局。 |
| <code>Mapping</code> | <code>&#91;&#93;byte</code> | 块级映射JSON，保留原块到新单元的归属。 |
| <code>SourceHash</code> | <code>&#91;&#93;byte</code> | 来源固定正文摘要，证明映射基于哪个正文。 |
| <code>TargetHash</code> | <code>&#91;&#93;byte</code> | 目标固定正文摘要，避免把映射用于另一份内容。 |

<!-- write-step: 4 bytes=1773:2063 -->
````go
type UnitMapping struct {
	Source, Target, SourceVersion, TargetVersion, SourceStructure, TargetStructure uint64
	Operation, Schema                                                              string
	Mapping, SourceHash, TargetHash                                                []byte
}

````

## 第 5 步：StructureUnitMappings

插入位置：第 4 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func StructureUnitMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]UnitMapping, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.str …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.str …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.str …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;UnitMapping</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.str …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;item.Source, &amp;item.Target, &amp;item.SourceVersion, &amp;item.TargetVersion, &amp;item.Operation, &amp;item.Schema, &amp;item.Mapping, &amp;item.SourceHash, &amp;item.TargetHash, &amp;item.SourceStructure, &amp;item.TargetStructure)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2063:3223 -->
````go
func StructureUnitMappings(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]UnitMapping, error) {
	rows, err := tx.QueryContext(ctx, `SELECT l.source_unit_id,l.target_unit_id,l.source_version_id,l.target_version_id,l.operation_id,l.mapping_schema,l.block_mapping,s.body_hash,t.body_hash,s.structure_id,t.structure_id
 FROM unit_lineage l JOIN content_version s ON s.id=l.source_version_id AND s.unit_id=l.source_unit_id AND s.document_id=l.source_document_id
 JOIN content_version t ON t.id=l.target_version_id AND t.unit_id=l.target_unit_id AND t.document_id=l.target_document_id
 WHERE l.source_document_id=? AND l.target_document_id=? AND l.target_unit_id=? ORDER BY l.id`, document, document, unit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []UnitMapping{}
	for rows.Next() {
		var item UnitMapping
		if err = rows.Scan(&item.Source, &item.Target, &item.SourceVersion, &item.TargetVersion, &item.Operation, &item.Schema, &item.Mapping, &item.SourceHash, &item.TargetHash, &item.SourceStructure, &item.TargetStructure); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/manifest_context_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
