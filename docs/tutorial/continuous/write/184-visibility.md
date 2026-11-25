# 分步编写 `services/document/internal/repository/document/visibility.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/184-visibility.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

修改文档或知识库可见政策，并取得需要重新索引的固定文档集合。

知识库级修改影响多篇已发布文档；在同一门闩与快照下确定集合才能避免漏发索引事件。

写之前先按这条执行路线理解各段的连接：

1. VisibilityDocumentIDs限定KB内当前可处理文档
2. LockedVisibilityIndexFacts锁定并复核归属
3. AdvanceVisibilitySequence以previous条件推进索引水位。

本文件需要守住的边界：

- 可见性变化不改当前release指针与publication_epoch
- CAS更新不是一行时必须报冲突。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/visibility.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/visibility.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/visibility.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/visibility.go' | Out-Null }
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

## 第 2 步：UpdateVisibility

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`func UpdateVisibility(ctx context.Context, tx *sql.Tx, id uint64, isKnowledgeBase bool, policy string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, policy, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, policy, id)</code> |
| <code>isKnowledgeBase</code> | <code>bool</code> | true更新knowledge_base，false更新document；两条路径都同时推进可见性修订和ACL修订。 |
| <code>policy</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, policy, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>isKnowledgeBase</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?&quot;, policy, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=115:482 -->
````go
func UpdateVisibility(ctx context.Context, tx *sql.Tx, id uint64, isKnowledgeBase bool, policy string) error {
	table := "document"
	if isKnowledgeBase {
		table = "knowledge_base"
	}
	_, err := tx.ExecContext(ctx, "UPDATE "+table+" SET visibility_policy=?,visibility_revision=visibility_revision+1,acl_revision=acl_revision+1 WHERE id=?", policy, id)
	return err
}

````

## 第 3 步：VisibilityDocumentIDs

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

调用方已经持有kb-acl排他gate及KB行锁，且尚未做一致性快照读取。
创建/发布/生命周期变更同样先取得该gate，因此此时首次快照固定完整范围，
后续再按数值主键逐个当前读加锁；不能用二级索引的扫描锁序冒充文档主键锁序。
非ACTIVE/未发布行仍带KB修订的权威摘要，不能筛掉它们或使用生命周期入口的400条限制。

本步声明：`func VisibilityDocumentIDs(ctx context.Context, tx *sql.Tx, knowledgeBase uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document WHERE knowledge_base_id=? ORDER BY id&quot;, knowledgeBase)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>knowledgeBase</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT id FROM document WHERE knowledge_base_id=? ORDER BY id&quot;, knowledgeBase)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id FROM document WHERE knowledge_base_id=? ORDER BY id&quot;, knowledgeBase)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=482:1369 -->
````go
// 调用方已经持有kb-acl排他gate及KB行锁，且尚未做一致性快照读取。
// 创建/发布/生命周期变更同样先取得该gate，因此此时首次快照固定完整范围，
// 后续再按数值主键逐个当前读加锁；不能用二级索引的扫描锁序冒充文档主键锁序。
// 非ACTIVE/未发布行仍带KB修订的权威摘要，不能筛掉它们或使用生命周期入口的400条限制。
func VisibilityDocumentIDs(ctx context.Context, tx *sql.Tx, knowledgeBase uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM document WHERE knowledge_base_id=? ORDER BY id", knowledgeBase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0)
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

## 第 4 步：VisibilityIndexFacts

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`type VisibilityIndexFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 受可见范围更新影响的文档。 |
| <code>Sequence</code> | <code>uint64</code> | 文档索引事件序号，保障消费端按次序拒绝旧事件。 |
| <code>Release</code> | <code>uint64</code> | 当前正式版编号。 |
| <code>PublicationEpoch</code> | <code>uint64</code> | 当前发布代次，用来区分撤下/重发后的可见性事实。 |

<!-- write-step: 4 bytes=1369:1461 -->
````go
type VisibilityIndexFacts struct {
	Document, Sequence, Release, PublicationEpoch uint64
}

````

## 第 5 步：LockedVisibilityIndexFacts

插入位置：第 4 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func LockedVisibilityIndexFacts(ctx context.Context, tx *sql.Tx, document, knowledgeBase uint64) (VisibilityIndexFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT event_seq,CAST(current_release_id AS CHAR),publication_epoch,CAST(knowledge_base_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Document: document</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT event_seq,CAST(current_release_id AS CHAR),publication_epoch,CAST(knowledge_base_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |
| <code>knowledgeBase</code> | <code>uint64</code> | 在此声明中的实际使用：<code>knowledgeBase != 0</code>；<code>domain.TextID(knowledgeBase)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>VisibilityIndexFacts</code> | 实际返回表达式：<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;VISIBILITY_DOCUMENT_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT event_seq,CAST(current_release_id AS CHAR),publication_epoch,CAST(knowledge_base_id AS CHAR) FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 检查 <code>knowledgeBase != 0 &amp;&amp; (!kb.Valid &#124;&#124; kb.String != domain.TextID(knowledgeBase))</code>。条件成立时执行 <code>return facts, domain.Violation(domain.StateConflict, &quot;VISIBILITY_DOCUMENT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>release.Valid</code>。
- 此处通过 <code>return facts, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1461:2244 -->
````go
func LockedVisibilityIndexFacts(ctx context.Context, tx *sql.Tx, document, knowledgeBase uint64) (VisibilityIndexFacts, error) {
	facts := VisibilityIndexFacts{Document: document}
	var release, kb sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT event_seq,CAST(current_release_id AS CHAR),publication_epoch,CAST(knowledge_base_id AS CHAR) FROM document WHERE id=? FOR UPDATE", document).
		Scan(&facts.Sequence, &release, &facts.PublicationEpoch, &kb)
	if err != nil {
		return facts, err
	}
	if knowledgeBase != 0 && (!kb.Valid || kb.String != domain.TextID(knowledgeBase)) {
		return facts, domain.Violation(domain.StateConflict, "VISIBILITY_DOCUMENT_BINDING_CHANGED")
	}
	if release.Valid {
		facts.Release, err = domain.CanonicalID(release.String)
	}
	return facts, err
}

````

## 第 6 步：AdvanceVisibilitySequence

插入位置：第 5 步代码之后，紧接追加；完成稿约第 60 行。

只推进索引水位，保留当前正式指针和publication_epoch；调用方仍持有同一文档行锁。

本步声明：`func AdvanceVisibilitySequence(ctx context.Context, tx *sql.Tx, document, previous, next uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET event_seq=? WHERE id=? AND event_seq=?&quot;, next, document, previous)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET event_seq=? WHERE id=? AND event_seq=?&quot;, next, document, previous)</code> |
| <code>previous</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET event_seq=? WHERE id=? AND event_seq=?&quot;, next, document, previous)</code> |
| <code>next</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET event_seq=? WHERE id=? AND event_seq=?&quot;, next, document, previous)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;VISIBILITY_EVENT_SEQUENCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET event_seq=? WHERE id=? AND event_seq=?&quot;, next, document, previous)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;VISIBILITY_EVENT_SEQUENCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2244:2809 -->
````go
// 只推进索引水位，保留当前正式指针和publication_epoch；调用方仍持有同一文档行锁。
func AdvanceVisibilitySequence(ctx context.Context, tx *sql.Tx, document, previous, next uint64) error {
	result, err := tx.ExecContext(ctx, "UPDATE document SET event_seq=? WHERE id=? AND event_seq=?", next, document, previous)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.StateConflict, "VISIBILITY_EVENT_SEQUENCE_CHANGED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/visibility.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
