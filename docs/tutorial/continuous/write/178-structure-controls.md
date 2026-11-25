# 分步编写 `services/document/internal/repository/document/structure_controls.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/178-structure-controls.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

追加不可变结构修订、逐项写结构条目并切换文档结构指针。

历史结构仍支撑旧版本与审核阅读，所以改目录也不能原地UPDATE旧条目。

写之前先按这条执行路线理解各段的连接：

1. InsertControlledStructure建立parent链
2. InsertStructureEntries写新条目
3. PointToControlledStructure只在整体控制事务就绪后切当前指针。

本文件需要守住的边界：

- 不要先切指针再分批写条目
- 所有写入须用同一调用方tx。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/structure_controls.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/structure_controls.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/structure_controls.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/structure_controls.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:96 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	"context"
	"database/sql"
)

````

## 第 2 步：InsertControlledStructure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`func InsertControlledStructure(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> |
| <code>parent</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,&#x27;structure-v1&#x27;)&quot;, document, parent, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=96:400 -->
````go
func InsertControlledStructure(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,'structure-v1')", document, parent, operation)
}

````

## 第 3 步：InsertStructureEntries

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func InsertStructureEntries(ctx context.Context, tx *sql.Tx, document, structure uint64, entries []dto.StructureEntry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,  parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)` …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,  parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)` …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,  parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)` …</code>（长表达式，按本步完整代码继续） |
| <code>entries</code> | <code>&#91;&#93;dto.StructureEntry</code> | 已经验证的新结构条目，逐条写入父节点、章节、排序和单元关系；共享外层事务，避免只插入半棵目录。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,  parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)` …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=400:961 -->
````go
func InsertStructureEntries(ctx context.Context, tx *sql.Tx, document, structure uint64, entries []dto.StructureEntry) error {
	for _, entry := range entries {
		_, err := tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,
 parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			structure, document, entry.NodeId, entry.ParentNodeId, entry.ChapterId, entry.Kind,
			entry.Title, entry.SortOrder, entry.UnitId, entry.SplitLevel2)
		if err != nil {
			return err
		}
	}
	return nil
}

````

## 第 4 步：PointToControlledStructure

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func PointToControlledStructure(ctx context.Context, tx *sql.Tx, document, structure uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=? WHERE id=?&quot;, structure, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=? WHERE id=?&quot;, structure, document)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=? WHERE id=?&quot;, structure, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET current_structure_id=? WHERE id=?&quot;, structure, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=961:1185 -->
````go
func PointToControlledStructure(ctx context.Context, tx *sql.Tx, document, structure uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET current_structure_id=? WHERE id=?", structure, document)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/structure_controls.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
