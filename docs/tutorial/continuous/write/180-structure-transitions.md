# 分步编写 `services/document/internal/repository/document/structure_transitions.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/180-structure-transitions.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存拆章合章后的新单元、旧单元退役、lineage映射与支线恢复状态。

范围迁移必须留下可追溯边，旧权限及旧输出也需要确认关闭后才能收尾。

写之前先按这条执行路线理解各段的连接：

1. CreateStructureUnit分配目标
2. RetireStructureUnit/Chapter保留历史退役标记
3. InsertUnitLineage保存源目标版本映射
4. StructureAuthorizationComplete核Edge和Node摘要。

本文件需要守住的边界：

- 不能物理删除旧单元使审核历史失去归属
- 两个参与者关闭证据必须属于原操作和epoch。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/structure_transitions.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/structure_transitions.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/structure_transitions.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/structure_transitions.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:115 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

````

## 第 2 步：CreateStructureUnit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`func CreateStructureUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64, kind string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |
| <code>chapter</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return InsertID(ctx, tx, &quot;INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)&quot;, document, chapter, kind, structure)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=115:405 -->
````go
func CreateStructureUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64, kind string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,?,?)", document, chapter, kind, structure)
}
````

## 第 3 步：RestoreStructureBranch

插入位置：第 2 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func RestoreStructureBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;RESTORING&#x27; WHERE id=? AND lifecycle=&#x27;PREPARING&#x27;&quot;, lineage, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;RESTORING&#x27; WHERE id=? AND lifecycle=&#x27;PREPARING&#x27;&quot;, lineage, branch)</code> |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;RESTORING&#x27; WHERE id=? AND lifecycle=&#x27;PREPARING&#x27;&quot;, lineage, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET current_lineage_id=?,lifecycle=&#x27;RESTORING&#x27; WHERE id=? AND lifecycle=&#x27;PREPARING&#x27;&quot;, lineage, branch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=405:661 -->
````go
func RestoreStructureBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,lifecycle='RESTORING' WHERE id=? AND lifecycle='PREPARING'", lineage, branch)
	return err
}
````

## 第 4 步：RetireStructureUnit

插入位置：第 3 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func RetireStructureUnit(ctx context.Context, tx *sql.Tx, document, unit, branch, structure uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;RETIRED&#x27;,revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, branch, unit, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;UNIT&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;RETIRED&#x27;,revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, branch, unit, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;UNIT&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, unit)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;RETIRED&#x27;,revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, branch, unit, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;UNIT&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, unit)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;RETIRED&#x27;,revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, branch, unit, document)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_ …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;RETIRED&#x27;,revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle=&#x27;ACTIVE&#x27;&quot;, branch, unit, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;UNIT&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 4 bytes=661:1593 -->
````go
func RetireStructureUnit(ctx context.Context, tx *sql.Tx, document, unit, branch, structure uint64) error {
	// 原身份退役，直接UNIT授权关闭；章级授权由当前真实结构解析，不把旧直接授权扩为整章。
	if _, err := tx.ExecContext(ctx, "UPDATE collaboration_unit SET retired_structure_id=?,active_branch_id=NULL,revision=revision+1 WHERE id=? AND document_id=? AND active_branch_id=? AND retired_structure_id IS NULL", structure, unit, document, branch); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='RETIRED',revision=revision+1 WHERE id=? AND unit_id=? AND document_id=? AND lifecycle='ACTIVE'", branch, unit, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND scope_kind='UNIT' AND scope_id=? AND state<>'CLOSED'", document, unit)
	return err
}
````

## 第 5 步：RetireStructureChapter

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func RetireStructureChapter(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, structure, chapter, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;CHAPTER&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, chapter)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, structure, chapter, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;CHAPTER&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, chapter)</code> |
| <code>chapter</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, structure, chapter, document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;CHAPTER&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, chapter)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, structure, chapter, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, structure, chapter, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND scope_kind=&#x27;CHAPTER&#x27; AND scope_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, document, chapter)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 5 bytes=1593:2107 -->
````go
func RetireStructureChapter(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) error {
	if _, err := tx.ExecContext(ctx, "UPDATE chapter SET retired_structure_id=? WHERE id=? AND document_id=? AND retired_structure_id IS NULL", structure, chapter, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND scope_kind='CHAPTER' AND scope_id=? AND state<>'CLOSED'", document, chapter)
	return err
}
````

## 第 6 步：InsertUnitLineage

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func InsertUnitLineage(ctx context.Context, tx *sql.Tx, operation string, document, source, target, sourceVersion, targetVersion uint64, mapping []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>sourceVersion</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>targetVersion</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续） |
| <code>mapping</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(mapping)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,map …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2107:2622 -->
````go
func InsertUnitLineage(ctx context.Context, tx *sql.Tx, operation string, document, source, target, sourceVersion, targetVersion uint64, mapping []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO unit_lineage(operation_id,source_unit_id,target_unit_id,source_document_id,target_document_id,source_version_id,target_version_id,block_mapping,mapping_schema) VALUES(?,?,?,?,?,?,?,?,'full-range-v1')", operation, source, target, document, document, sourceVersion, targetVersion, string(mapping))
	return err
}
````

## 第 7 步：CompleteStructureBranches

插入位置：第 6 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func CompleteStructureBranches(ctx context.Context, tx *sql.Tx, document uint64, branches []uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27;,revision=revision+1 WHERE id=? AND document_id=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27;,revision=revision+1 WHERE id=? AND document_id=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, document)</code> |
| <code>branches</code> | <code>&#91;&#93;uint64</code> | 本次结构变更安装完成的支线编号集合；只把同文档RESTORING支线恢复为ACTIVE。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE branch SET lifecycle=&#x27;ACTIVE&#x27;,revision=revision+1 WHERE id=? AND document_id=? AND lifecycle=&#x27;RESTORING&#x27;&quot;, branch, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2622:2982 -->
````go
func CompleteStructureBranches(ctx context.Context, tx *sql.Tx, document uint64, branches []uint64) error {
	for _, branch := range branches {
		if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE',revision=revision+1 WHERE id=? AND document_id=? AND lifecycle='RESTORING'", branch, document); err != nil {
			return err
		}
	}
	return nil
}
````

## 第 8 步：StructureAuthorizationComplete

插入位置：第 7 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func StructureAuthorizationComplete(ctx context.Context, tx *sql.Tx, operation string, document, epoch uint64, edgeDigest, nodeDigest []byte) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续） |
| <code>edgeDigest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续） |
| <code>nodeDigest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind=&#x27;AUTHORIZATION_CHANGE&#x27;  WHERE c.revocation_id= …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2982:3971 -->
````go
func StructureAuthorizationComplete(ctx context.Context, tx *sql.Tx, operation string, document, epoch uint64, edgeDigest, nodeDigest []byte) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authorization_change c JOIN durable_task t ON t.operation_id=c.revocation_id AND t.kind='AUTHORIZATION_CHANGE'
 WHERE c.revocation_id=? AND c.scope_key=? AND c.epoch=? AND c.state='SUCCEEDED' AND t.state='SUCCEEDED'
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.participant='edge' AND a.phase='OUTPUT_STOPPED' AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.result_digest=?)
 AND EXISTS(SELECT 1 FROM authorization_ack a WHERE a.revocation_id=c.revocation_id AND a.participant='collaboration' AND a.phase='OUTPUT_STOPPED' AND a.scope_key=c.scope_key AND a.epoch=c.epoch AND a.result_digest=?)`, operation, "doc-acl:"+domain.TextID(document), epoch, edgeDigest, nodeDigest).Scan(&count)
	return count == 1, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/structure_transitions.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
