# 分步编写 `services/document/internal/repository/comment/comment_queries.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/139-comment-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存线程、评论、回复墓碑、版本上下文和通知意图。

删除正文不应破坏回复链或分页位置；线程与首评必须同事务出现。 线程上下文筛选在SQL WHERE中先执行，避免其它正文版本占满当前页后再被客户端丢掉。

写之前先按这条执行路线理解各段的连接：

1. InsertThread加InsertComment创建讨论
2. DeleteComment清空正文保留ID/reply_to
3. ContextVersion校验BRANCH/MAIN/REVIEW归属
4. QueueMentions随回执落库
5. UpdateThreadState执行CAS。
6. ThreadFilter保存可选Kind/Context/Version；ListThreads只从固定列名列表附加context_kind/context_ref/content_version_id的?条件，再ORDER BY id LIMIT。
7. 空筛选兼容原文档全部线程；只有当前服务层验证过的非空字段才参与查询，删除线程仍被排除。

本文件需要守住的边界：

- 同单元任意版本不是BRANCH上下文
- 旧revision不能覆盖别人的解决/重开
- 重复通知键不能覆盖原目标。
- filters ...ThreadFilter仅消费首个过滤对象，缺省为不筛选；不是任意SQL列名或多对象OR。
- Context/Version仍是原固定身份；筛选不代替validateContext权限检查，不能把旧锚点强行解释为最新正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/comment' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/comment/comment_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/comment/comment_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/comment/comment_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/comment/comment_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:420 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package comment

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	notices "collaboration/document/internal/repository/notification"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
)

// comments.go：保留原实现，按业务合并文件。

````

## 第 2 步：Entry

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`type Entry struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 单条评论的主键。 |
| <code>Thread</code> | <code>uint64</code> | 评论所属讨论串；回复也必须属于此串。 |
| <code>Author</code> | <code>uint64</code> | 发表此条评论的账号。 |
| <code>Revision</code> | <code>uint64</code> | 评论的并发修订号，修改或删除前比较它，避免覆盖他人的更改。 |
| <code>Body</code> | <code>string</code> | 评论正文；不是被引用的文档正文。 |
| <code>Deleted</code> | <code>bool</code> | 软删除标记；保留评论关系，但界面不能再展示已删除正文。 |

<!-- write-step: 2 bytes=420:552 -->
````go
type Entry struct {
	ID, Thread, Author, Revision uint64
	Body                         string
	Deleted                      bool
}

````

## 第 3 步：commentColumns

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`const commentColumns = \`id,thread_id,author_id,revision,IF(deleted_at IS NULL,body,''),deleted_at IS NOT NULL\``。

<!-- write-step: 3 bytes=552:664 -->
````go
const commentColumns = `id,thread_id,author_id,revision,IF(deleted_at IS NULL,body,''),deleted_at IS NOT NULL`

````

## 第 4 步：scanComment

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func scanComment(row scanner) (Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>scanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Entry</code> | 实际返回表达式：<code>entry</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>readError(err)</code>；<code>errors.New(&quot;COMMENT_ENTRY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return entry, readError(err)</code>。当前函数在这里结束。
- 执行 <code>row.Scan(&amp;entry.ID, &amp;entry.Thread, &amp;entry.Author, &amp;entry.Revision, &amp;entry.Body, &amp;entry.Deleted)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>entry.ID == 0 &#124;&#124; entry.Thread == 0 &#124;&#124; entry.Author == 0 &#124;&#124; entry.Revision == 0</code>。条件成立时执行 <code>return entry, errors.New(&quot;COMMENT_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return entry, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=664:1045 -->
````go
func scanComment(row scanner) (Entry, error) {
	var entry Entry
	if err := row.Scan(&entry.ID, &entry.Thread, &entry.Author, &entry.Revision, &entry.Body, &entry.Deleted); err != nil {
		return entry, readError(err)
	}
	if entry.ID == 0 || entry.Thread == 0 || entry.Author == 0 || entry.Revision == 0 {
		return entry, errors.New("COMMENT_ENTRY_INVALID")
	}
	return entry, nil
}

````

## 第 5 步：CommentRow

插入位置：第 4 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func CommentRow(ctx context.Context, q Queryer, id uint64, lock bool) (Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Entry</code> | 由 <code>scanComment(q.QueryRowContext(ctx, query, id))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanComment(q.QueryRowContext(ctx, query, id))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 此处通过 <code>return scanComment(q.QueryRowContext(ctx, query, id))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, query, id)</code>。

<!-- write-step: 5 bytes=1045:1295 -->
````go
func CommentRow(ctx context.Context, q Queryer, id uint64, lock bool) (Entry, error) {
	query := "SELECT " + commentColumns + " FROM comment WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scanComment(q.QueryRowContext(ctx, query, id))
}

````

## 第 6 步：InsertComment

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

回复关系只能指向同线程已有评论，包括墓碑；删除作者正文不应破坏讨论结构。
调用方先锁线程，避免插入回复与线程删除各自只看见一半的状态。

本步声明：`func InsertComment(ctx context.Context, tx *sql.Tx, thread, actor uint64, body string, reply *uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread)</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code>；<code>insertedID(ctx, tx)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code>；<code>insertedID(ctx, tx)</code> |
| <code>thread</code> | <code>uint64</code> | 在此声明中的实际使用：<code>thread == 0</code>；<code>tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread)</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code> |
| <code>body</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code> |
| <code>reply</code> | <code>*uint64</code> | 在此声明中的实际使用：<code>reply != nil</code>；<code>tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread)</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>insertedID(ctx, tx)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>insertedID(ctx, tx)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_ENTRY_INVALID&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;COMMENT_REPLY_NOT_FOUND&quot;)</code>；<code>readError(err)</code>；<code>errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>thread == 0 &#124;&#124; actor == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_ENTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>reply != nil</code>。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return 0, domain.Violation(domain.EntityNotFound, &quot;COMMENT_REPLY_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, readError(err)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return insertedID(ctx, tx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1295:2269 -->
````go
// 回复关系只能指向同线程已有评论，包括墓碑；删除作者正文不应破坏讨论结构。
// 调用方先锁线程，避免插入回复与线程删除各自只看见一半的状态。
func InsertComment(ctx context.Context, tx *sql.Tx, thread, actor uint64, body string, reply *uint64) (uint64, error) {
	if thread == 0 || actor == 0 {
		return 0, errors.New("COMMENT_ENTRY_INVALID")
	}
	if reply != nil {
		var id uint64
		err := tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.Violation(domain.EntityNotFound, "COMMENT_REPLY_NOT_FOUND")
		}
		if err != nil {
			return 0, readError(err)
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)
	if err != nil {
		return 0, errors.New("COMMENT_WRITE_FAILED")
	}
	return insertedID(ctx, tx)
}

````

## 第 7 步：EditComment

插入位置：第 6 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func EditComment(ctx context.Context, tx *sql.Tx, entry Entry, body string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE comment SET body=?,revision=revision+1 		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision&lt;18446744073709551615`, body, entry.ID, entry.Thread, entry.Revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>entry</code> | <code>Entry</code> | 在此声明中的实际使用：<code>entry.Deleted</code>；<code>entry.Revision</code>；<code>entry.ID</code>；<code>entry.Thread</code> |
| <code>body</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE comment SET body=?,revision=revision+1 		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision&lt;18446744073709551615`, body, entry.ID, entry.Thread, entry.Revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>revisionChanged()</code>；<code>changed(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>entry.Deleted &#124;&#124; entry.Revision == ^uint64(0)</code>。条件成立时执行 <code>return revisionChanged()</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE comment SET body=?,revision=revision+1 		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision&lt;18446744073709551615`, body, entry.ID, entry.Thread, entry.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return changed(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2269:2699 -->
````go
func EditComment(ctx context.Context, tx *sql.Tx, entry Entry, body string) error {
	if entry.Deleted || entry.Revision == ^uint64(0) {
		return revisionChanged()
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment SET body=?,revision=revision+1
		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision<18446744073709551615`, body, entry.ID, entry.Thread, entry.Revision)
	return changed(result, err)
}

````

## 第 8 步：DeleteComment

插入位置：第 7 步代码之后，紧接追加；完成稿约第 77 行。

墓碑保留主键与 reply_to_id，正文清空；后续分页仍返回此位置，避免游标与回复断裂。

本步声明：`func DeleteComment(ctx context.Context, tx *sql.Tx, entry Entry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE comment SET body=&#x27;&#x27;,deleted_at=UTC_TIMESTAMP(6),revision=revision+1 		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision&lt;1844 …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>entry</code> | <code>Entry</code> | 在此声明中的实际使用：<code>entry.Deleted</code>；<code>entry.Revision</code>；<code>entry.ID</code>；<code>entry.Thread</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>revisionChanged()</code>；<code>changed(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>entry.Deleted &#124;&#124; entry.Revision == ^uint64(0)</code>。条件成立时执行 <code>return revisionChanged()</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE comment SET body=&#x27;&#x27;,deleted_at=UTC_TIMESTAMP(6),revision=revision+1 		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision&lt;1844 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return changed(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2699:3259 -->
````go
// 墓碑保留主键与 reply_to_id，正文清空；后续分页仍返回此位置，避免游标与回复断裂。
func DeleteComment(ctx context.Context, tx *sql.Tx, entry Entry) error {
	if entry.Deleted || entry.Revision == ^uint64(0) {
		return revisionChanged()
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment SET body='',deleted_at=UTC_TIMESTAMP(6),revision=revision+1
		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision<18446744073709551615`, entry.ID, entry.Thread, entry.Revision)
	return changed(result, err)
}

````

## 第 9 步：ListComments

插入位置：第 8 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func ListComments(ctx context.Context, tx *sql.Tx, thread, after uint64, limit int) ([]Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT &quot;+commentColumns+&quot; FROM comment WHERE thread_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, thread, after, pageLimit(limit))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>thread</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT &quot;+commentColumns+&quot; FROM comment WHERE thread_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, thread, after, pageLimit(limit))</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT &quot;+commentColumns+&quot; FROM comment WHERE thread_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, thread, after, pageLimit(limit))</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>pageLimit(limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;Entry</code> | 实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>readError(err)</code>；<code>err</code>；<code>readError(rows.Err())</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT &quot;+commentColumns+&quot; FROM comment WHERE thread_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, thread, after, pageLimit(limit))</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, readError(err)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return items, readError(rows.Err())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3259:3776 -->
````go
func ListComments(ctx context.Context, tx *sql.Tx, thread, after uint64, limit int) ([]Entry, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+commentColumns+" FROM comment WHERE thread_id=? AND id>? ORDER BY id LIMIT ?", thread, after, pageLimit(limit))
	if err != nil {
		return nil, readError(err)
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		entry, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	return items, readError(rows.Err())
}

````

## 第 10 步：Entry.DTO

插入位置：第 9 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func (entry Entry) DTO() dto.Comment`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Comment</code> | 实际返回表达式：<code>dto.Comment{Id: domain.TextID(entry.ID), ThreadId: domain.TextID(entry.Thread), AuthorId: domain.TextID(entry.Author), Body: body, Revision: domain.TextID(entry.Revision)}</code> |

**跟着本段代码走一遍：**

- 检查 <code>entry.Deleted</code>。
- 此处通过 <code>return dto.Comment{Id: domain.TextID(entry.ID), ThreadId: domain.TextID(entry.Thread), AuthorId: domain.TextID(entry.Author), Body: body, Revision: domain.TextID(entry.Revision)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3776:4113 -->
````go
func (entry Entry) DTO() dto.Comment {
	body := entry.Body
	if entry.Deleted {
		body = ""
	}
	return dto.Comment{Id: domain.TextID(entry.ID), ThreadId: domain.TextID(entry.Thread), AuthorId: domain.TextID(entry.Author), Body: body, Revision: domain.TextID(entry.Revision)}
}

// context.go：保留原实现，按业务合并文件。

````

## 第 11 步：ContextVersion

插入位置：第 10 步代码之后，紧接追加；完成稿约第 114 行。

ContextVersion 仅证明固定版本属于指定上下文，不代替当前或历史访问权限。
BRANCH 只接受该分支真实谱系或送审记录，不能因版本属于同一单元就扩大可读范围。

本步声明：`func ContextVersion(ctx context.Context, tx *sql.Tx, document uint64, kind string, contextID, version uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, query, document, contextID, version)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, query, document, contextID, version)</code> |
| <code>kind</code> | <code>string</code> | 引用类别：MAIN查询清单、REVIEW查询轮次、BRANCH沿支线谱系及审核外键核验固定版本归属；不能只凭版本存在就允许评论。 |
| <code>contextID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, query, document, contextID, version)</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, query, document, contextID, version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>found</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_CONTEXT_INVALID&quot;)</code>；<code>readError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return false, errors.New(&quot;COMMENT_CONTEXT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, query, document, contextID, version).Scan(&amp;found)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, query, document, contextID, version)</code>。
- 此处通过 <code>return found, readError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4113:6018 -->
````go
// ContextVersion 仅证明固定版本属于指定上下文，不代替当前或历史访问权限。
// BRANCH 只接受该分支真实谱系或送审记录，不能因版本属于同一单元就扩大可读范围。
func ContextVersion(ctx context.Context, tx *sql.Tx, document uint64, kind string, contextID, version uint64) (bool, error) {
	var query string
	switch kind {
	case "MAIN":
		query = `SELECT EXISTS (SELECT 1 FROM manifest_entry e
			JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
			WHERE e.document_id=? AND e.manifest_id=? AND v.id=?)`
	case "REVIEW":
		query = `SELECT EXISTS (SELECT 1 FROM review_round r
			JOIN content_version v ON v.document_id=r.document_id AND v.unit_id=r.unit_id
				AND v.id IN (r.content_version_id,r.base_version_id)
			WHERE r.document_id=? AND r.id=? AND v.id=?)`
	case "BRANCH":
		// review_round 没有 branch_id，必须经 review_request 绑定真实分支，
		// 同时逐层核对文档与单元，避免历史分支间借用另一个送审版本。
		query = `SELECT EXISTS (SELECT 1 FROM branch b
			JOIN content_version v ON v.document_id=b.document_id AND v.unit_id=b.unit_id
			WHERE b.document_id=? AND b.id=? AND v.id=? AND (
				EXISTS (SELECT 1 FROM branch_lineage l WHERE l.branch_id=b.id
					AND l.document_id=b.document_id AND l.unit_id=b.unit_id AND l.base_version_id=v.id)
				OR EXISTS (SELECT 1 FROM review_request r JOIN review_round rr
					ON rr.request_id=r.id AND rr.document_id=r.document_id AND rr.unit_id=r.unit_id
					WHERE r.branch_id=b.id AND r.document_id=b.document_id AND r.unit_id=b.unit_id
					AND v.id IN (rr.content_version_id,rr.base_version_id))))`
	default:
		return false, errors.New("COMMENT_CONTEXT_INVALID")
	}
	var found bool
	err := tx.QueryRowContext(ctx, query, document, contextID, version).Scan(&found)
	return found, readError(err)
}

````

## 第 12 步：BranchScope

插入位置：第 11 步代码之后，紧接追加；完成稿约第 148 行。

BranchScope 保留历史分支身份，但单独返回是否仍为该单元当前活动分支。
创建新线程由业务层要求 active；历史线程读取不能误用此标志抹去已有事实。

本步声明：`func BranchScope(ctx context.Context, tx *sql.Tx, document, branch uint64) (unit uint64, active bool, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id, 		(b.lifecycle=&#x27;ACTIVE&#x27; AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id) 		FROM branch b JOIN collaboration_un …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id, 		(b.lifecycle=&#x27;ACTIVE&#x27; AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id) 		FROM branch b JOIN collaboration_un …</code>（长表达式，按本步完整代码继续） |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT b.unit_id, 		(b.lifecycle=&#x27;ACTIVE&#x27; AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id) 		FROM branch b JOIN collaboration_un …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>unit</code> | <code>uint64</code> | 实际返回表达式：<code>unit</code> |
| <code>active</code> | <code>bool</code> | 实际返回表达式：<code>active</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>readError(err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.unit_id, 		(b.lifecycle=&#x27;ACTIVE&#x27; AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id) 		FROM branch b JOIN collaboration_un …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT b.unit_id, 		(b.lifecycle=&#x27;ACTIVE&#x27; AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id) 		FROM branch b JOIN collaboration_un …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return unit, active, readError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6018:6757 -->
````go
// BranchScope 保留历史分支身份，但单独返回是否仍为该单元当前活动分支。
// 创建新线程由业务层要求 active；历史线程读取不能误用此标志抹去已有事实。
func BranchScope(ctx context.Context, tx *sql.Tx, document, branch uint64) (unit uint64, active bool, err error) {
	err = tx.QueryRowContext(ctx, `SELECT b.unit_id,
		(b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id)
		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id
		WHERE b.document_id=? AND b.id=?`, document, branch).Scan(&unit, &active)
	return unit, active, readError(err)
}

// events.go：保留原实现，按业务合并文件。

````

## 第 13 步：QueueMentions

插入位置：第 12 步代码之后，紧接追加；完成稿约第 160 行。

QueueMentions 与评论和通用操作回执同事务持久化通知意图，避免评论提交后
进程退出丢失提醒。这里只记录原固定目标；实际收件资格由 worker 重新核验。
原操作只在首次业务执行时插入，重复键不得覆盖已经绑定的正文目标或收件人。

本步声明：`func QueueMentions(ctx context.Context, tx *sql.Tx, operation string, thread Thread, comment, actor uint64, mentions []uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.CommentRecipients(ctx, tx, thread.ID, actor)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;COMMENT_NOTIFICATION&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(raw))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>notices.CommentRecipients(ctx, tx, thread.ID, actor)</code>；<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;COMMENT_NOTIFICATION&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(raw))</code> |
| <code>thread</code> | <code>Thread</code> | 在此声明中的实际使用：<code>thread.ID</code>；<code>thread.Document</code>；<code>thread.Kind</code>；<code>thread.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>comment</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Comment: comment</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>notices.CommentRecipients(ctx, tx, thread.ID, actor)</code>；<code>Actor: actor</code> |
| <code>mentions</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>Recipients: mentions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;COMMENT_NOTIFICATION_PAYLOAD_INVALID&quot;)</code>；<code>errors.New(&quot;COMMENT_NOTIFICATION_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_PAYLOAD_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;COMMENT_NOTIFICATION&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))`, operation, operation, string(raw))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6757:8156 -->
````go
// QueueMentions 与评论和通用操作回执同事务持久化通知意图，避免评论提交后
// 进程退出丢失提醒。这里只记录原固定目标；实际收件资格由 worker 重新核验。
// 原操作只在首次业务执行时插入，重复键不得覆盖已经绑定的正文目标或收件人。
func QueueMentions(ctx context.Context, tx *sql.Tx, operation string, thread Thread, comment, actor uint64, mentions []uint64) error {
	participants, err := notices.CommentRecipients(ctx, tx, thread.ID, actor)
	if err != nil {
		return err
	}
	// 无@的回复也需要提醒原参与者；是否有合格收件人由投递事务判断。
	payload := domain.CommentNotification{
		Document: thread.Document, Thread: thread.ID, Comment: comment,
		ContextKind: thread.Kind, ContextID: thread.Context, Version: thread.Version,
		Actor: actor, Recipients: mentions, ReplyRecipients: participants,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return errors.New("COMMENT_NOTIFICATION_PAYLOAD_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('COMMENT_NOTIFICATION',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, operation, operation, string(raw))
	if err != nil {
		return errors.New("COMMENT_NOTIFICATION_WRITE_FAILED")
	}
	return nil
}

// threads.go：保留原实现，按业务合并文件。

````

## 第 14 步：Queryer

插入位置：第 13 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`type Queryer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>QueryRowContext</code> | <code>(context.Context, string, ...any) *sql.Row</code> | 依次接收ctx、带占位符的SQL和占位符参数；返回单行句柄，实际查询/无行错误在Scan时处理。接口让连接池或原事务都可注入，但涉及锁时必须沿用原事务。 |

<!-- write-step: 14 bytes=8156:8243 -->
````go
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

````

## 第 15 步：Thread

插入位置：第 14 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`type Thread struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 讨论串主键。 |
| <code>Document</code> | <code>uint64</code> | 该讨论所属的文档。 |
| <code>Author</code> | <code>uint64</code> | 发起讨论的账号。 |
| <code>Revision</code> | <code>uint64</code> | 讨论串并发修订号，用于状态更新。 |
| <code>Version</code> | <code>uint64</code> | 讨论引用的固定正文版本，正文更新不会自动改写此引用。 |
| <code>Kind</code> | <code>string</code> | 引用上下文的类别，决定按主线、审核等哪条路径核验。 |
| <code>Context</code> | <code>string</code> | 对应类别下的上下文标识，须和Version一起验证归属。 |
| <code>Status</code> | <code>string</code> | 讨论进行中或已解决等状态。 |
| <code>Anchor</code> | <code>dto.Anchor</code> | 引用的文本/块位置；由原版本解释，不能把相同偏移直接套到新正文。 |

<!-- write-step: 15 bytes=8243:8415 -->
````go
type Thread struct {
	ID, Document, Author, Revision, Version uint64
	Kind, Context, Status                   string
	Anchor                                  dto.Anchor
}

````

## 第 16 步：scanner

插入位置：第 15 步代码之后，紧接追加；完成稿约第 198 行。

本步声明：`type scanner interface{ Scan(...any) error }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scan</code> | <code>(...any) error</code> | 按SELECT列顺序传入接收变量的指针；返回解码/查询错误。抽象Row与Rows共用的扫描能力，不负责开启或提交事务。 |

<!-- write-step: 16 bytes=8415:8461 -->
````go
type scanner interface{ Scan(...any) error }

````

## 第 17 步：threadColumns

插入位置：第 16 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`const threadColumns = \`id,document_id,author_id,revision,COALESCE(content_version_id,0),context_kind,context_ref,status,anchor_json,anchor_schema\``。

<!-- write-step: 17 bytes=8461:8609 -->
````go
const threadColumns = `id,document_id,author_id,revision,COALESCE(content_version_id,0),context_kind,context_ref,status,anchor_json,anchor_schema`

````

## 第 18 步：scanThread

插入位置：第 17 步代码之后，紧接追加；完成稿约第 202 行。

锚点正文与关系列必须共同指向同一固定版本，损坏元数据不能降级成当前正文锚点。

本步声明：`func scanThread(row scanner) (Thread, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>scanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Thread</code> | 实际返回表达式：<code>t</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>readError(err)</code>；<code>errors.New(&quot;COMMENT_ANCHOR_INVALID&quot;)</code>；<code>errors.New(&quot;COMMENT_THREAD_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return t, readError(err)</code>。当前函数在这里结束。
- 执行 <code>row.Scan(&amp;t.ID, &amp;t.Document, &amp;t.Author, &amp;t.Revision, &amp;t.Version, &amp;t.Kind, &amp;t.Context, &amp;t.Status, &amp;raw, &amp;schema)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return t, errors.New(&quot;COMMENT_ANCHOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != io.EOF</code>。条件成立时执行 <code>return t, errors.New(&quot;COMMENT_ANCHOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; contextErr != nil &#124;&#124; version != t.Version &#124;&#124; schema == &quot;&quot; &#124;&#124; schema != t.Anchor.SchemaVersion &#124;&#124; t.ID == 0 &#124;&#124; t.Document == 0 &#124;&#124; t.Author == 0 &#124;&#124; t.Revision == 0</code>。条件成立时执行 <code>return t, errors.New(&quot;COMMENT_THREAD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>(t.Kind != &quot;MAIN&quot; &amp;&amp; t.Kind != &quot;BRANCH&quot; &amp;&amp; t.Kind != &quot;REVIEW&quot;) &#124;&#124; (t.Status != &quot;OPEN&quot; &amp;&amp; t.Status != &quot;RESOLVED&quot; &amp;&amp; t.Status != &quot;DELETED&quot;)</code>。条件成立时执行 <code>return t, errors.New(&quot;COMMENT_THREAD_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return t, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8609:9840 -->
````go
// 锚点正文与关系列必须共同指向同一固定版本，损坏元数据不能降级成当前正文锚点。
func scanThread(row scanner) (Thread, error) {
	var t Thread
	var raw []byte
	var schema string
	if err := row.Scan(&t.ID, &t.Document, &t.Author, &t.Revision, &t.Version, &t.Kind, &t.Context, &t.Status, &raw, &schema); err != nil {
		return t, readError(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&t.Anchor); err != nil {
		return t, errors.New("COMMENT_ANCHOR_INVALID")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return t, errors.New("COMMENT_ANCHOR_INVALID")
	}
	version, err := domain.CanonicalID(t.Anchor.ContentVersionId)
	_, contextErr := domain.CanonicalID(t.Context)
	if err != nil || contextErr != nil || version != t.Version || schema == "" || schema != t.Anchor.SchemaVersion || t.ID == 0 || t.Document == 0 || t.Author == 0 || t.Revision == 0 {
		return t, errors.New("COMMENT_THREAD_INVALID")
	}
	if (t.Kind != "MAIN" && t.Kind != "BRANCH" && t.Kind != "REVIEW") || (t.Status != "OPEN" && t.Status != "RESOLVED" && t.Status != "DELETED") {
		return t, errors.New("COMMENT_THREAD_INVALID")
	}
	return t, nil
}

````

## 第 19 步：ThreadRow

插入位置：第 18 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func ThreadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Thread, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Thread</code> | 由 <code>scanThread(q.QueryRowContext(ctx, query, id))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanThread(q.QueryRowContext(ctx, query, id))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 此处通过 <code>return scanThread(q.QueryRowContext(ctx, query, id))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, query, id)</code>。

<!-- write-step: 19 bytes=9840:10095 -->
````go
func ThreadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Thread, error) {
	query := "SELECT " + threadColumns + " FROM comment_thread WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scanThread(q.QueryRowContext(ctx, query, id))
}

````

## 第 20 步：InsertThread

插入位置：第 19 步代码之后，紧接追加；完成稿约第 237 行。

InsertThread 与首条评论、操作回执共享业务事务；此处不提交，避免空线程可见。

本步声明：`func InsertThread(ctx context.Context, tx *sql.Tx, document, actor uint64, kind, contextRef string, anchor dto.Anchor) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续）；<code>insertedID(ctx, tx)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>insertedID(ctx, tx)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续） |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind != &quot;MAIN&quot;</code>；<code>kind != &quot;BRANCH&quot;</code>；<code>kind != &quot;REVIEW&quot;</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续） |
| <code>contextRef</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.CanonicalID(contextRef)</code>；<code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续） |
| <code>anchor</code> | <code>dto.Anchor</code> | 在此声明中的实际使用：<code>anchor.ContentVersionId</code>；<code>anchor.SchemaVersion</code>；<code>json.Marshal(anchor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>insertedID(ctx, tx)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>insertedID(ctx, tx)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_THREAD_INVALID&quot;)</code>；<code>errors.New(&quot;COMMENT_ANCHOR_INVALID&quot;)</code>；<code>errors.New(&quot;COMMENT_THREAD_WRITE_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; contextErr != nil &#124;&#124; document == 0 &#124;&#124; actor == 0 &#124;&#124; anchor.SchemaVersion == &quot;&quot; &#124;&#124; (kind != &quot;MAIN&quot; &amp;&amp; kind != &quot;BRANCH&quot; &amp;&amp; kind != &quot;REVIEW&quot;)</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_THREAD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_ANCHOR_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status) 		VALUES(?,?,?,?,?,?,?,&#x27;OPEN&#x27;)`, …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_THREAD_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return insertedID(ctx, tx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10095:11141 -->
````go
// InsertThread 与首条评论、操作回执共享业务事务；此处不提交，避免空线程可见。
func InsertThread(ctx context.Context, tx *sql.Tx, document, actor uint64, kind, contextRef string, anchor dto.Anchor) (uint64, error) {
	version, err := domain.CanonicalID(anchor.ContentVersionId)
	_, contextErr := domain.CanonicalID(contextRef)
	if err != nil || contextErr != nil || document == 0 || actor == 0 || anchor.SchemaVersion == "" || (kind != "MAIN" && kind != "BRANCH" && kind != "REVIEW") {
		return 0, errors.New("COMMENT_THREAD_INVALID")
	}
	raw, err := json.Marshal(anchor)
	if err != nil {
		return 0, errors.New("COMMENT_ANCHOR_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status)
		VALUES(?,?,?,?,?,?,?,'OPEN')`, document, kind, contextRef, version, string(raw), anchor.SchemaVersion, actor)
	if err != nil {
		return 0, errors.New("COMMENT_THREAD_WRITE_FAILED")
	}
	return insertedID(ctx, tx)
}

````

## 第 21 步：UpdateThreadState

插入位置：第 20 步代码之后，紧接追加；完成稿约第 256 行。

CAS 与文档/线程锁配合，旧修订号不能覆盖另一条已完成的解决或重开动作。

本步声明：`func UpdateThreadState(ctx context.Context, tx *sql.Tx, t Thread, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE comment_thread SET status=?,revision=revision+1 		WHERE id=? AND revision=? AND status&lt;&gt;&#x27;DELETED&#x27; AND revision&lt;18446744073709551615`, state, t.ID, t.Revision)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>t</code> | <code>Thread</code> | 在此声明中的实际使用：<code>t.Revision</code>；<code>t.Status</code>；<code>t.ID</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>state != &quot;OPEN&quot;</code>；<code>state != &quot;RESOLVED&quot;</code>；<code>state != &quot;DELETED&quot;</code>；<code>tx.ExecContext(ctx, `UPDATE comment_thread SET status=?,revision=revision+1 		WHERE id=? AND revision=? AND status&lt;&gt;&#x27;DELETED&#x27; AND revision&lt;18446744073709551615`, state, t.ID, t.Revision)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>revisionChanged()</code>；<code>errors.New(&quot;COMMENT_THREAD_STATE_INVALID&quot;)</code>；<code>changed(result, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>t.Revision == ^uint64(0) &#124;&#124; t.Status == &quot;DELETED&quot;</code>。条件成立时执行 <code>return revisionChanged()</code>。当前函数在这里结束。
- 检查 <code>state != &quot;OPEN&quot; &amp;&amp; state != &quot;RESOLVED&quot; &amp;&amp; state != &quot;DELETED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_THREAD_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE comment_thread SET status=?,revision=revision+1 		WHERE id=? AND revision=? AND status&lt;&gt;&#x27;DELETED&#x27; AND revision&lt;18446744073709551615`, state, t.ID, t.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return changed(result, err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=11141:11777 -->
````go
// CAS 与文档/线程锁配合，旧修订号不能覆盖另一条已完成的解决或重开动作。
func UpdateThreadState(ctx context.Context, tx *sql.Tx, t Thread, state string) error {
	if t.Revision == ^uint64(0) || t.Status == "DELETED" {
		return revisionChanged()
	}
	if state != "OPEN" && state != "RESOLVED" && state != "DELETED" {
		return errors.New("COMMENT_THREAD_STATE_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment_thread SET status=?,revision=revision+1
		WHERE id=? AND revision=? AND status<>'DELETED' AND revision<18446744073709551615`, state, t.ID, t.Revision)
	return changed(result, err)
}

````

## 第 22 步：ThreadFilter

插入位置：第 21 步代码之后，紧接追加；完成稿约第 269 行。

本步声明：`type ThreadFilter struct{ Kind, Context, Version string }`。

**这一段为什么这样写：** 把三个可选只读筛选值传过handler、业务验证和仓储；同一对象也纳入签名游标绑定。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | 对应context_kind；空或MAIN/BRANCH/REVIEW，限制线程所属上下文类别。 |
| <code>Context</code> | <code>string</code> | 对应公开context_id和SQL context_ref；非空必须是规范非零ID，且必须同时给Kind。 |
| <code>Version</code> | <code>string</code> | 对应content_version_id；非空是规范固定正文版本ID，SQL直接等值筛选，不能随当前正文自动更新。 |

<!-- write-step: 22 bytes=11777:11836 -->
````go
type ThreadFilter struct{ Kind, Context, Version string }

````

## 第 23 步：ListThreads

插入位置：第 22 步代码之后，紧接追加；完成稿约第 271 行。

本步声明：`func ListThreads(ctx context.Context, tx *sql.Tx, document, after uint64, limit int, filters ...ThreadFilter) ([]Thread, error)`。

**这一段为什么这样写：** 先限定文档、未删除线程、after和固定上下文条件，再稳定取一个有界扫描批次；显示页与权限裁剪由服务层继续完成。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 原受控读事务的取消上下文。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, statement, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 统一输出读取使用的原事务，不能换连接绕过来源锁。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 原真实文档ID。<br>在此声明中的实际使用：<code>args := &#91;&#93;any{document, after}</code> |
| <code>after</code> | <code>uint64</code> | 上一已扫描线程ID，首页0。<br>在此声明中的实际使用：<code>args := &#91;&#93;any{document, after}</code> |
| <code>limit</code> | <code>int</code> | 本次仓储扫描批大小，经pageLimit约束，和最终显示页大小分开。<br>在此声明中的实际使用：<code>pageLimit(limit)</code> |
| <code>filters</code> | <code>...ThreadFilter</code> | 可选ThreadFilter切片，只读首项；非空字段各追加一个固定列名的?等值条件。<br>在此声明中的实际使用：<code>len(filters)</code>；<code>f := filters&#91;0&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;Thread</code> | 按id升序的匹配线程，仍需逐项validateContext当前权限。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL或损坏线程元数据错误，不能降级返回部分未核验结果。<br>实际返回表达式：<code>readError(err)</code>；<code>err</code>；<code>readError(rows.Err())</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(filters) &gt; 0</code>。
- 检查 <code>c.value != &quot;&quot;</code>。
- 执行 <code>tx.QueryContext(ctx, statement, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, readError(err)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return items, readError(rows.Err())</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=11836:12795 -->
````go
func ListThreads(ctx context.Context, tx *sql.Tx, document, after uint64, limit int, filters ...ThreadFilter) ([]Thread, error) {
	statement := "SELECT " + threadColumns + " FROM comment_thread WHERE document_id=? AND id>? AND status<>'DELETED'"
	args := []any{document, after}
	if len(filters) > 0 {
		f := filters[0]
		for _, c := range []struct{ column, value string }{{"context_kind", f.Kind}, {"context_ref", f.Context}, {"content_version_id", f.Version}} {
			if c.value != "" {
				statement += " AND " + c.column + "=?"
				args = append(args, c.value)
			}
		}
	}
	statement += " ORDER BY id LIMIT ?"
	args = append(args, pageLimit(limit))
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, readError(err)
	}
	defer rows.Close()
	items := []Thread{}
	for rows.Next() {
		item, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, readError(rows.Err())
}

````

## 第 24 步：Thread.DTO

插入位置：第 23 步代码之后，紧接追加；完成稿约第 301 行。

本步声明：`func (t Thread) DTO() dto.CommentThread`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.CommentThread</code> | 实际返回表达式：<code>dto.CommentThread{Id: domain.TextID(t.ID), DocumentId: domain.TextID(t.Document), ContextKind: t.Kind, ContextId: t.Context, Anchor: t.Anchor, AuthorId: domain.TextID(t.Author), St …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return dto.CommentThread{Id: domain.TextID(t.ID), DocumentId: domain.TextID(t.Document), ContextKind: t.Kind, ContextId: t.Context, Anchor: t.Anchor, AuthorId: domain.TextID(t.Auth …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=12795:13081 -->
````go
func (t Thread) DTO() dto.CommentThread {
	return dto.CommentThread{Id: domain.TextID(t.ID), DocumentId: domain.TextID(t.Document), ContextKind: t.Kind, ContextId: t.Context, Anchor: t.Anchor, AuthorId: domain.TextID(t.Author), Status: t.Status, Revision: domain.TextID(t.Revision)}
}

````

## 第 25 步：readError

插入位置：第 24 步代码之后，紧接追加；完成稿约第 305 行。

本步声明：`func readError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>err == sql.ErrNoRows</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;COMMENT_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &#124;&#124; err == sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;COMMENT_READ_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=13081:13217 -->
````go
func readError(err error) error {
	if err == nil || err == sql.ErrNoRows {
		return err
	}
	return errors.New("COMMENT_READ_FAILED")
}

````

## 第 26 步：insertedID

插入位置：第 25 步代码之后，紧接追加；完成稿约第 312 行。

本步声明：`func insertedID(ctx context.Context, tx *sql.Tx) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;).Scan(&amp;id) != nil &#124;&#124; id == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT LAST_INSERT_ID()&quot;)</code>。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=13217:13453 -->
````go
func insertedID(ctx context.Context, tx *sql.Tx) (uint64, error) {
	var id uint64
	if tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id) != nil || id == 0 {
		return 0, errors.New("COMMENT_WRITE_FAILED")
	}
	return id, nil
}

````

## 第 27 步：revisionChanged

插入位置：第 26 步代码之后，紧接追加；完成稿约第 320 行。

本步声明：`func revisionChanged() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;COMMENT_REVISION_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;COMMENT_REVISION_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=13453:13562 -->
````go
func revisionChanged() error {
	return domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
}

````

## 第 28 步：changed

插入位置：第 27 步代码之后，紧接追加；完成稿约第 324 行。

本步声明：`func changed(result sql.Result, err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>result</code> | <code>sql.Result</code> | 在此声明中的实际使用：<code>result.RowsAffected</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err != nil</code>；<code>n, err := result.RowsAffected()</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code>；<code>revisionChanged()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>n != 1</code>。条件成立时执行 <code>return revisionChanged()</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=13562:13832 -->
````go
func changed(result sql.Result, err error) error {
	if err != nil {
		return errors.New("COMMENT_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return errors.New("COMMENT_WRITE_FAILED")
	}
	if n != 1 {
		return revisionChanged()
	}
	return nil
}

````

## 第 29 步：pageLimit

插入位置：第 28 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`func pageLimit(limit int) int`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &lt; 1</code>；<code>limit &gt; 101</code>；<code>return limit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>1</code>；<code>101</code>；<code>limit</code> |

**跟着本段代码走一遍：**

- 检查 <code>limit &lt; 1</code>。条件成立时执行 <code>return 1</code>。当前函数在这里结束。
- 检查 <code>limit &gt; 101</code>。条件成立时执行 <code>return 101</code>。当前函数在这里结束。
- 此处通过 <code>return limit</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=13832:14006 -->
````go
func pageLimit(limit int) int {
	if limit < 1 {
		return 1
	}
	// 允许业务层多取一项判定是否存在下一页。
	if limit > 101 {
		return 101
	}
	return limit
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/comment/comment_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
