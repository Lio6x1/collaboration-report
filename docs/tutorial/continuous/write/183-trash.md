# 分步编写 `services/document/internal/repository/document/trash.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/183-trash.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取真实回收状态、当前管理候选、高水位和原恢复回执。

普通DocumentRow只接ACTIVE，不能为回收放宽普通正文入口。

写之前先按这条执行路线理解各段的连接：

1. TrashCandidates筛责任人/活动KB管理员
2. TrashSourceHighWater读关闭水位
3. TrashRestoreOriginalReceipt预读原号
4. RestoreTrashDocument只改可变控制事实。

本文件需要守住的边界：

- 恢复不改固定正文、结构、样式或manifest
- 旧content_grant不授予回收管理权。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/trash.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/trash.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/trash.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/trash.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:164 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

````

## 第 2 步：TrashDocumentRow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

只读真实回收事实，保留SQL未知；这里不借普通ACTIVE入口，也不隐式开启事务。

本步声明：`func TrashDocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.TrashFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationDocumentColumns+&quot;,purge_after,UTC_TIMESTAMP(6) FROM document WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationDocumentColumns+&quot;,purge_after,UTC_TIMESTAMP(6) FROM document WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.TrashFacts</code> | 使用 <code>domain.TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>实际返回表达式：<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationDocumentColumns+&quot;,purge_after,UTC_TIMESTAMP(6) FROM document WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return facts, domain.Violation(domain.EntityNotFound, &quot;DOCUMENT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 检查 <code>purge.Valid</code>。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=164:1359 -->
````go
// 只读真实回收事实，保留SQL未知；这里不借普通ACTIVE入口，也不隐式开启事务。
func TrashDocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.TrashFacts, error) {
	var facts domain.TrashFacts
	d := &facts.Document
	var structure, style, manifest sql.NullString
	var purge sql.NullTime
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT "+publicationDocumentColumns+",purge_after,UTC_TIMESTAMP(6) FROM document WHERE id=?"+suffix, id).
		Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId, &d.Lifecycle, &d.MainLocked,
			&structure, &style, &manifest, &d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision,
			&d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision, &facts.Sequence, &purge, &facts.Now)
	if errors.Is(err, sql.ErrNoRows) {
		return facts, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	if err != nil {
		return facts, err
	}
	d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId = structure.String, style.String, manifest.String
	if purge.Valid {
		facts.PurgeAfter = &purge.Time
	}
	return facts, nil
}

````

## 第 3 步：TrashKnowledgeBaseRow

插入位置：第 2 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func TrashKnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationKnowledgeBaseColumns+&quot; FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationKnowledgeBaseColumns+&quot; FROM knowledge_base WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.KnowledgeBase</code> | 实际返回表达式：<code>kb</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationKnowledgeBaseColumns+&quot; FROM knowledge_base WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return kb, domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return kb, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1359:1826 -->
````go
func TrashKnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	kb, err := scanPublicationKnowledgeBase(q.QueryRowContext(ctx, "SELECT "+publicationKnowledgeBaseColumns+" FROM knowledge_base WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return kb, domain.Violation(domain.TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
	}
	return kb, err
}

````

## 第 4 步：TrashSpaceRow

插入位置：第 3 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func TrashSpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationSpaceColumns+&quot; FROM space WHERE id=?&quot;+suffix, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationSpaceColumns+&quot; FROM space WHERE id=?&quot;+suffix, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_SPACE_UNKNOWN&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT &quot;+publicationSpaceColumns+&quot; FROM space WHERE id=?&quot;+suffix, id)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return space, domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_SPACE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return space, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1826:2252 -->
````go
func TrashSpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	space, err := scanPublicationSpace(q.QueryRowContext(ctx, "SELECT "+publicationSpaceColumns+" FROM space WHERE id=?"+suffix, id))
	if errors.Is(err, sql.ErrNoRows) {
		return space, domain.Violation(domain.TemporarilyUnavailable, "TRASH_SPACE_UNKNOWN")
	}
	return space, err
}

````

## 第 5 步：TrashKBRole

插入位置：第 4 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func TrashKBRole(ctx context.Context, q Queryer, id, actor uint64, lock bool) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?&quot;+suffix, id, actor)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?&quot;+suffix, id, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?&quot;+suffix, id, actor)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>role</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?&quot;+suffix, id, actor).Scan(&amp;role, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?&quot;+suffix, id, actor)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return &quot;&quot;, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return &quot;&quot;, nil</code>。当前函数在这里结束。
- 此处通过 <code>return role, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2252:2740 -->
````go
func TrashKBRole(ctx context.Context, q Queryer, id, actor uint64, lock bool) (string, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var role, state string
	err := q.QueryRowContext(ctx, "SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=?"+suffix, id, actor).Scan(&role, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if state != "ACTIVE" {
		return "", nil
	}
	return role, nil
}

````

## 第 6 步：TrashCandidates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 81 行。

SQL只选择现今有管理资格的候选，最多一页加一个lookahead；拒绝项不消耗分页位置。
org普通成员仍须是当前Doc负责人或当前ACTIVE KB ADMIN，content_grant不参与这个范围。

本步声明：`func TrashCandidates(ctx context.Context, q Queryer, space, actor uint64, inherited bool, last uint64, budget int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, statement, args...)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{space, last}</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>append(args, actor, actor)</code> |
| <code>inherited</code> | <code>bool</code> | 在此声明中的实际使用：<code>!inherited</code> |
| <code>last</code> | <code>uint64</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{space, last}</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>budget &lt; 2</code>；<code>budget &gt; 101</code>；<code>append(args, budget)</code>；<code>make(&#91;&#93;uint64, 0, budget)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;TRASH_PAGE_BUDGET_INVALID&quot;)</code>；<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>budget &lt; 2 &#124;&#124; budget &gt; 101</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;TRASH_PAGE_BUDGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!inherited</code>。
- 执行 <code>q.QueryContext(ctx, statement, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2740:4055 -->
````go
// SQL只选择现今有管理资格的候选，最多一页加一个lookahead；拒绝项不消耗分页位置。
// org普通成员仍须是当前Doc负责人或当前ACTIVE KB ADMIN，content_grant不参与这个范围。
func TrashCandidates(ctx context.Context, q Queryer, space, actor uint64, inherited bool, last uint64, budget int) ([]uint64, error) {
	if budget < 2 || budget > 101 {
		return nil, domain.Violation(domain.InvalidRequest, "TRASH_PAGE_BUDGET_INVALID")
	}
	statement := "SELECT d.id FROM document d WHERE d.space_id=? AND d.lifecycle='TRASHED' AND d.id>?"
	args := []any{space, last}
	if !inherited {
		statement += " AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id WHERE k.id=d.knowledge_base_id AND k.space_id=d.space_id AND k.lifecycle='ACTIVE' AND m.user_id=? AND m.role='ADMIN' AND m.state='ACTIVE'))"
		args = append(args, actor, actor)
	}
	statement += " ORDER BY d.id LIMIT ?"
	args = append(args, budget)
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0, budget)
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

## 第 7 步：TrashSourceHighWater

插入位置：第 6 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`func TrashSourceHighWater(ctx context.Context, q Queryer, key string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT max_epoch FROM access_gate WHERE scope_key=?&quot;, key)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT max_epoch FROM access_gate WHERE scope_key=?&quot;, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>epoch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT max_epoch FROM access_gate WHERE scope_key=?&quot;, key).Scan(&amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT max_epoch FROM access_gate WHERE scope_key=?&quot;, key)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return 0, nil</code>。当前函数在这里结束。
- 此处通过 <code>return epoch, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4055:4342 -->
````go
func TrashSourceHighWater(ctx context.Context, q Queryer, key string) (uint64, error) {
	var epoch uint64
	err := q.QueryRowContext(ctx, "SELECT max_epoch FROM access_gate WHERE scope_key=?", key).Scan(&epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return epoch, err
}

````

## 第 8 步：TrashRestoreOriginalReceipt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func TrashRestoreOriginalReceipt(ctx context.Context, q Queryer, operation string) (domain.TrashRestoreReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.path&#x27;)) FROM operation_receipt WHERE operation_id=?&quot;, operation)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>Operation: operation</code>；<code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.path&#x27;)) FROM operation_receipt WHERE operation_id=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.TrashRestoreReceipt</code> | 使用 <code>domain.TrashRestoreReceipt</code> 类型；其定义处的职责：预读只决定使用当前来源准备原号回放；不包含另一份fingerprint/结果解析， 真正重放必须交给标准Mutation持原operation gate核验并读取固定receipt。<br>实际返回表达式：<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.path&#x27;)) FROM operation_receipt WHERE operation_id=?&quot;, operation)</code>。
- 此处通过 <code>return receipt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4342:4828 -->
````go
func TrashRestoreOriginalReceipt(ctx context.Context, q Queryer, operation string) (domain.TrashRestoreReceipt, error) {
	receipt := domain.TrashRestoreReceipt{Operation: operation}
	var path sql.NullString
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.path')) FROM operation_receipt WHERE operation_id=?", operation).
		Scan(&receipt.Actor, &receipt.Kind, &receipt.State, &path)
	receipt.Path = path.String
	return receipt, err
}

````

## 第 9 步：RestoreTrashDocument

插入位置：第 8 步代码之后，紧接追加；完成稿约第 129 行。

调用方已按来源gate→KB→Doc→Space顺序锁定且复核当前资格；只改可变控制事实，
MAIN/branch的固定结构、样式、manifest和正文不变。撤下在冻结schema中存为UNPUBLISHED。

本步声明：`func RestoreTrashDocument(ctx context.Context, tx *sql.Tx, id uint64, expected string, next domain.TrashRestoreRevisions) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,visibility_policy=&#x27;COLLABORATORS&#x27;, 		current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,deleted_at=NULL,purge_after …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,visibility_policy=&#x27;COLLABORATORS&#x27;, 		current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,deleted_at=NULL,purge_after …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, id)</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,visibility_policy=&#x27;COLLABORATORS&#x27;, 		current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,deleted_at=NULL,purge_after …</code>（长表达式，按本步完整代码继续） |
| <code>next</code> | <code>domain.TrashRestoreRevisions</code> | 使用 <code>domain.TrashRestoreRevisions</code> 类型；其定义处的职责：定义回收事实、恢复资格和单文档永久删除计划。<br>在此声明中的实际使用：<code>next.Control</code>；<code>next.ACL</code>；<code>next.Publication</code>；<code>next.Visibility</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_RESTORE_STATE_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;ACTIVE&#x27;,visibility_policy=&#x27;COLLABORATORS&#x27;, 		current_release_id=NULL,publication_state=&#x27;UNPUBLISHED&#x27;,deleted_at=NULL,purge_after …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;TRASH_RESTORE_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4828:6129 -->
````go
// 调用方已按来源gate→KB→Doc→Space顺序锁定且复核当前资格；只改可变控制事实，
// MAIN/branch的固定结构、样式、manifest和正文不变。撤下在冻结schema中存为UNPUBLISHED。
func RestoreTrashDocument(ctx context.Context, tx *sql.Tx, id uint64, expected string, next domain.TrashRestoreRevisions) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='ACTIVE',visibility_policy='COLLABORATORS',
		current_release_id=NULL,publication_state='UNPUBLISHED',deleted_at=NULL,purge_after=NULL,
		control_revision=?,acl_revision=?,publication_epoch=?,visibility_revision=?,event_seq=?
		WHERE id=? AND lifecycle='TRASHED' AND control_revision=? AND purge_after>UTC_TIMESTAMP(6)`,
		next.Control, next.ACL, next.Publication, next.Visibility, next.Sequence, id, expected)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "TRASH_RESTORE_STATE_CHANGED")
	}
	// 仍ACTIVE或撤权中的原直接授权全部关闭，绝不把回收前授权重新解释成恢复资格。
	_, err = tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND state<>'CLOSED'", id)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/trash.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
