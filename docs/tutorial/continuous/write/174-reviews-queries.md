# 分步编写 `services/document/internal/repository/document/reviews_queries.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/174-reviews-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读固定轮次与审核历史，并在同一事务记录动作、意见及通知任务。

审计动作要保留当时版本和理由；修改意见解决与通知必须可追溯到真实原动作。

写之前先按这条执行路线理解各段的连接：

1. InsertReviewAction写actor/round/version
2. InsertChange创建意见
3. ResolveChange关联解决动作
4. AdvanceReviewRevision推进并发版本
5. QueueReviewActionNotification登记后续投递。

本文件需要守住的边界：

- 旧轮次样式必须从冻结上下文取得，不能套当前样式
- 全局列表候选不等于正文读取授权。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/reviews_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/reviews_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/reviews_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/reviews_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:364 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// review_actions.go：保留原实现，按业务合并文件。

````

## 第 2 步：ReviewPointerRow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func ReviewPointerRow(ctx context.Context, q Queryer, id uint64) (domain.ReviewPointer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?&quot;, id).Scan(&amp;valu …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=364:817 -->
````go
func ReviewPointerRow(ctx context.Context, q Queryer, id uint64) (domain.ReviewPointer, error) {
	var value domain.ReviewPointer
	err := q.QueryRowContext(ctx, "SELECT id,document_id,unit_id,branch_id,COALESCE(current_round_id,0),revision,status,archived_at IS NOT NULL FROM review_request WHERE id=?", id).Scan(&value.ID, &value.Document, &value.Unit, &value.Branch, &value.Round, &value.Revision, &value.Status, &value.Archived)
	return value, err
}

````

## 第 3 步：ReviewRoundVersion

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func ReviewRoundVersion(ctx context.Context, q Queryer, request, round uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request).Scan(&amp;version)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT content_version_id FROM review_round WHERE id=? AND request_id=?&quot;, round, request)</code>。
- 此处通过 <code>return version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=817:1095 -->
````go
func ReviewRoundVersion(ctx context.Context, q Queryer, request, round uint64) (uint64, error) {
	var version uint64
	err := q.QueryRowContext(ctx, "SELECT content_version_id FROM review_round WHERE id=? AND request_id=?", round, request).Scan(&version)
	return version, err
}

````

## 第 4 步：EverSubmitted

插入位置：第 3 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func EverSubmitted(ctx context.Context, q Queryer, request, actor uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)&quot;, request, actor)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)&quot;, request, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)&quot;, request, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>exists</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)&quot;, request, actor).Scan(&amp;exists)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)&quot;, request, actor)</code>。
- 此处通过 <code>return exists, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1095:1369 -->
````go
func EverSubmitted(ctx context.Context, q Queryer, request, actor uint64) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM request_submitter WHERE request_id=? AND user_id=?)", request, actor).Scan(&exists)
	return exists, err
}

````

## 第 5 步：LockedChange

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func LockedChange(ctx context.Context, tx *sql.Tx, request, id uint64) (domain.ChangeFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE&quot;, id, request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE&quot;, id, request)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE&quot;, id, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ChangeFact</code> | 使用 <code>domain.ChangeFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE&quot;, id, request).Scan(&amp;value.ID, &amp;v …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE&quot;, id, request)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1369:1783 -->
````go
func LockedChange(ctx context.Context, tx *sql.Tx, request, id uint64) (domain.ChangeFact, error) {
	var value domain.ChangeFact
	err := tx.QueryRowContext(ctx, "SELECT id,request_id,origin_round_id,author_id,revision,status FROM change_request WHERE id=? AND request_id=? FOR UPDATE", id, request).Scan(&value.ID, &value.Request, &value.Round, &value.Author, &value.Revision, &value.Status)
	return value, err
}

````

## 第 6 步：InsertReviewAction

插入位置：第 5 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func InsertReviewAction(ctx context.Context, tx *sql.Tx, operation, kind string, request, round, actor, version, change uint64, reason *string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>change</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |
| <code>reason</code> | <code>*string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),N …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1783:2317 -->
````go
func InsertReviewAction(ctx context.Context, tx *sql.Tx, operation, kind string, request, round, actor, version, change uint64, reason *string) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO review_action(request_id,round_id,actor_id,kind,operation_id,item_no,reason,content_version_id,target_change_id) VALUES(?,?,?,?,?,0,?,NULLIF(?,0),NULLIF(?,0))", request, round, actor, kind, operation, reason, version, change)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

````

## 第 7 步：InsertChange

插入位置：第 6 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func InsertChange(ctx context.Context, tx *sql.Tx, request, round, action, actor uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code> |
| <code>action</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,&#x27;OPEN&#x27;)&quot;, request, round, action, actor)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return uint64(id), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2317:2704 -->
````go
func InsertChange(ctx context.Context, tx *sql.Tx, request, round, action, actor uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO change_request(request_id,origin_round_id,created_action_id,author_id,status) VALUES(?,?,?,?,'OPEN')", request, round, action, actor)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

````

## 第 8 步：ResolveChange

插入位置：第 7 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func ResolveChange(ctx context.Context, tx *sql.Tx, change, action uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE change_request SET status=&#x27;RESOLVED&#x27;,resolved_action_id=?,revision=revision+1 WHERE id=?&quot;, action, change)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>change</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE change_request SET status=&#x27;RESOLVED&#x27;,resolved_action_id=?,revision=revision+1 WHERE id=?&quot;, action, change)</code> |
| <code>action</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE change_request SET status=&#x27;RESOLVED&#x27;,resolved_action_id=?,revision=revision+1 WHERE id=?&quot;, action, change)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE change_request SET status=&#x27;RESOLVED&#x27;,resolved_action_id=?,revision=revision+1 WHERE id=?&quot;, action, change)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2704:2948 -->
````go
func ResolveChange(ctx context.Context, tx *sql.Tx, change, action uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE change_request SET status='RESOLVED',resolved_action_id=?,revision=revision+1 WHERE id=?", action, change)
	return err
}

````

## 第 9 步：AdvanceReviewRevision

插入位置：第 8 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func AdvanceReviewRevision(ctx context.Context, tx *sql.Tx, request uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET revision=revision+1 WHERE id=?&quot;, request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET revision=revision+1 WHERE id=?&quot;, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET revision=revision+1 WHERE id=?&quot;, request)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2948:3147 -->
````go
func AdvanceReviewRevision(ctx context.Context, tx *sql.Tx, request uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE review_request SET revision=revision+1 WHERE id=?", request)
	return err
}

````

## 第 10 步：WithdrawReview

插入位置：第 9 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func WithdrawReview(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;WITHDRAWN&#x27;,active_merge_operation_id=NULL,revision=revision+1 WHERE id=?&quot;, pointer.ID)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=?,generation=generation+1 WHERE unit_id=?&quot;, pointer.ID, pointer.Unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE review_request SET status=&#x27;WITHDRAWN&#x27;,active_merge_operation_id=NULL,revision=revision+1 WHERE id=?&quot;, pointer.ID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=?,generation=generation+1 WHERE unit_id=?&quot;, pointer.ID, pointer.Unit)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 10 bytes=3147:3615 -->
````go
func WithdrawReview(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer) error {
	if _, err := tx.ExecContext(ctx, "UPDATE review_request SET status='WITHDRAWN',active_merge_operation_id=NULL,revision=revision+1 WHERE id=?", pointer.ID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE unit_review_guard SET ongoing_request_id=NULL,resumable_request_id=?,generation=generation+1 WHERE unit_id=?", pointer.ID, pointer.Unit)
	return err
}

````

## 第 11 步：QueueReviewActionNotification

插入位置：第 10 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func QueueReviewActionNotification(ctx context.Context, tx *sql.Tx, operation, kind string, pointer domain.ReviewPointer, action, actor uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVIEW_ACTION_NOTIFICATION&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVIEW_ACTION_NOTIFICATION&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertSubmissionTask(ctx, tx, &quot;REVIEW_ACTION_NOTIFICATION&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;event&quot;: kind</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Document</code>；<code>pointer.ID</code>；<code>pointer.Round</code> |
| <code>action</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(action)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>InsertSubmissionTask(ctx, tx, &quot;REVIEW_ACTION_NOTIFICATION&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return InsertSubmissionTask(ctx, tx, &quot;REVIEW_ACTION_NOTIFICATION&quot;, operation, operation, payload, &quot;QUEUED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3615:4271 -->
````go
func QueueReviewActionNotification(ctx context.Context, tx *sql.Tx, operation, kind string, pointer domain.ReviewPointer, action, actor uint64) error {
	// 通知意图与动作同事务；精确收件人与实际投递由 Task 9 读取权威事实后处理。
	payload, err := json.Marshal(map[string]string{"event": kind, "action": domain.TextID(action), "actor": domain.TextID(actor), "document": domain.TextID(pointer.Document), "request": domain.TextID(pointer.ID), "round": domain.TextID(pointer.Round)})
	if err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "REVIEW_ACTION_NOTIFICATION", operation, operation, payload, "QUEUED")
}

````

## 第 12 步：UnboundReviewActionNotifications

插入位置：第 11 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func UnboundReviewActionNotifications(ctx context.Context, q Queryer) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;REVIEW_ACTION_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND NOT EXISTS(SELECT 1 FRO …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>invalid</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;REVIEW_ACTION_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND NOT EXISTS(SELECT 1 FRO …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT EXISTS(SELECT 1 FROM durable_task t WHERE t.kind=&#x27;REVIEW_ACTION_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND NOT EXISTS(SELECT 1 FRO …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return invalid, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4271:5466 -->
````go
func UnboundReviewActionNotifications(ctx context.Context, q Queryer) (bool, error) {
	var invalid bool
	// 缺失或不一致的持久关联阻止启动；合法通知积压不伪装为已投递，也不冻结所有读取。
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM durable_task t WHERE t.kind='REVIEW_ACTION_NOTIFICATION' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND NOT EXISTS(SELECT 1 FROM operation_receipt o JOIN review_action a ON a.operation_id=o.operation_id AND a.item_no=0 JOIN review_request r ON r.id=a.request_id WHERE o.operation_id=t.operation_id AND o.state='SUCCEEDED' AND t.business_key=o.operation_id AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.action'))=CAST(a.id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))=CAST(a.actor_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document'))=CAST(r.document_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request'))=CAST(r.id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round'))=CAST(a.round_id AS CHAR) AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))=a.kind))").Scan(&invalid)
	return invalid, err
}

// review_readings.go：保留原实现，按业务合并文件。

````

## 第 13 步：ReviewRequestRow

插入位置：第 12 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`func ReviewRequestRow(ctx context.Context, q Queryer, id uint64) (dto.ReviewRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.rev …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.rev …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ReviewRequest</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.rev …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.rev …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>archived.Valid</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5466:6348 -->
````go
func ReviewRequestRow(ctx context.Context, q Queryer, id uint64) (dto.ReviewRequest, error) {
	var value dto.ReviewRequest
	var archived sql.NullTime
	var count int64
	err := q.QueryRowContext(ctx, "SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.unit_id AS CHAR),CAST(r.branch_id AS CHAR),r.status,CAST(r.current_round_id AS CHAR),CAST(r.revision AS CHAR),r.archived_at,(SELECT COUNT(*) FROM change_request c WHERE c.request_id=r.id AND c.status='OPEN') FROM review_request r WHERE r.id=?", id).Scan(&value.Id, &value.DocumentId, &value.UnitId, &value.BranchId, &value.Status, &value.CurrentRoundId, &value.Revision, &archived, &count)
	if err != nil {
		return value, err
	}
	if archived.Valid {
		text := archived.Time.UTC().Format(time.RFC3339Nano)
		value.ArchivedAt = &text
	}
	value.BlockingChangeCount = &count
	value.Actions = []string{}
	return value, nil
}

````

## 第 14 步：ReviewRoundRow

插入位置：第 13 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func ReviewRoundRow(ctx context.Context, tx *sql.Tx, request, round uint64, document string) (dto.ReviewRound, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;REVIEW_ROUND_CONTEXT&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;&quot;, value.Id, operation)</code>；<code>validateLegacyReverseStyle(ctx, tx, operation, document, value.ContentVersionId, style)</code>；<code>ContentRow(ctx, tx, value.ContentVersionId, document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>validateLegacyReverseStyle(ctx, tx, operation, document, value.ContentVersionId, style)</code>；<code>ContentRow(ctx, tx, value.ContentVersionId, document)</code>；<code>StructureRow(ctx, tx, value.StructureId, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续） |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续）；<code>validateLegacyReverseStyle(ctx, tx, operation, document, value.ContentVersionId, style)</code>；<code>ContentRow(ctx, tx, value.ContentVersionId, document)</code>；<code>StructureRow(ctx, tx, value.StructureId, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ReviewRound</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_i …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;REVIEW_ROUND_CONTEXT&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;&quot;, value.Id, operation).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;REVIEW_ROUND_CONTEXT&#x27; AND business_key=? AND operation_id=? AND state=&#x27;SUCCEEDED&#x27;&quot;, value.Id, operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>legacy</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=6348:7937 -->
````go
func ReviewRoundRow(ctx context.Context, tx *sql.Tx, request, round uint64, document string) (dto.ReviewRound, error) {
	var value dto.ReviewRound
	var operation string
	err := tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(request_id AS CHAR),CAST(round_no AS CHAR),CAST(content_version_id AS CHAR),CAST(base_manifest_id AS CHAR),CAST(base_version_id AS CHAR),CAST(structure_id AS CHAR),CAST(lineage_id AS CHAR),CAST(submitter_id AS CHAR),submission_operation_id FROM review_round WHERE id=? AND request_id=? AND document_id=?", round, request, document).Scan(&value.Id, &value.RequestId, &value.RoundNo, &value.ContentVersionId, &value.BaseManifestId, &value.BaseVersionId, &value.StructureId, &value.LineageId, &value.SubmitterId, &operation)
	if err != nil {
		return value, err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='REVIEW_ROUND_CONTEXT' AND business_key=? AND operation_id=? AND state='SUCCEEDED'", value.Id, operation).Scan(&raw); err != nil {
		return value, err
	}
	style, legacy, err := frozenRoundStyle(raw)
	if err != nil {
		return value, err
	}
	if legacy {
		if err = validateLegacyReverseStyle(ctx, tx, operation, document, value.ContentVersionId, style); err != nil {
			return value, err
		}
	}
	value.Content, err = ContentRow(ctx, tx, value.ContentVersionId, document)
	if err != nil {
		return value, err
	}
	value.Structure, err = StructureRow(ctx, tx, value.StructureId, document)
	if err != nil {
		return value, err
	}
	value.Style, err = StyleRow(ctx, tx, style, document)
	return value, err
}

````

## 第 15 步：ReviewRequestSummaryRow

插入位置：第 14 步代码之后，紧接追加；完成稿约第 144 行。

本步声明：`func ReviewRequestSummaryRow(ctx context.Context, tx *sql.Tx, request, round, document uint64) (dto.ReviewRequestSummary, []byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续） |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ReviewRequestSummary</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>body</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_r …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, body, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7937:9087 -->
````go
func ReviewRequestSummaryRow(ctx context.Context, tx *sql.Tx, request, round, document uint64) (dto.ReviewRequestSummary, []byte, error) {
	var value dto.ReviewRequestSummary
	var body []byte
	var submitted time.Time
	// 题名和正文必须共同绑定当前固定轮次；不读取MAIN/共同草稿，也不为列表拉样式或资源。
	err := tx.QueryRowContext(ctx, "SELECT e.title,v.body_json,CAST(r.round_no AS CHAR),CAST(r.submitter_id AS CHAR),r.created_at FROM review_request q JOIN review_round r ON r.id=q.current_round_id AND r.request_id=q.id AND r.document_id=q.document_id AND r.unit_id=q.unit_id JOIN content_version v ON v.id=r.content_version_id AND v.document_id=r.document_id AND v.unit_id=r.unit_id AND v.structure_id=r.structure_id JOIN structure_entry e ON e.structure_id=r.structure_id AND e.document_id=r.document_id AND e.unit_id=r.unit_id WHERE q.id=? AND r.id=? AND q.document_id=?", request, round, document).Scan(&value.Title, &body, &value.RoundNo, &value.SubmitterId, &submitted)
	if err != nil {
		return value, nil, err
	}
	value.SubmittedAt = submitted.UTC().Format(time.RFC3339Nano)
	return value, body, nil
}

````

## 第 16 步：ReviewRequestCandidates

插入位置：第 15 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func ReviewRequestCandidates(ctx context.Context, q Queryer, document uint64, last string, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id&gt;? ORDER BY id LIMIT ?&quot;, document, document, last, limit)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id&gt;? ORDER BY id LIMIT ?&quot;, document, document, last, limit)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id&gt;? ORDER BY id LIMIT ?&quot;, document, document, last, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id&gt;? ORDER BY id LIMIT ?&quot;, document, document, last, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id&gt;? ORDER BY id LIMIT ?&quot;, document, document, last, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9087:9723 -->
````go
func ReviewRequestCandidates(ctx context.Context, q Queryer, document uint64, last string, limit int) ([]uint64, error) {
	// 全局候选只返回稳定 ID；访问筛选在业务层执行，禁止把 SQL 的总量作为可见数量。
	rows, err := q.QueryContext(ctx, "SELECT id FROM review_request WHERE (?=0 OR document_id=?) AND id>? ORDER BY id LIMIT ?", document, document, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

````

## 第 17 步：ScopedReviewRequestCandidates

插入位置：第 16 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`func ScopedReviewRequestCandidates(ctx context.Context, q Queryer, actor uint64, organizations []uint64, last string, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, statement, args...)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{last, actor, actor, actor, actor}</code> |
| <code>organizations</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(organizations)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>args := &#91;&#93;any{last, actor, actor, actor, actor}</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>append(args, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(organizations) &gt; 0</code>。
- 执行 <code>q.QueryContext(ctx, statement, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9723:11196 -->
````go
func ScopedReviewRequestCandidates(ctx context.Context, q Queryer, actor uint64, organizations []uint64, last string, limit int) ([]uint64, error) {
	// 私有库只做可能有REVIEW/MANAGE的范围筛选；角色与来源是否仍有效留给逐项权威准入及最终事务。
	statement := `SELECT r.id FROM review_request r JOIN document d ON d.id=r.document_id JOIN space s ON s.id=d.space_id
		WHERE r.id>? AND d.lifecycle='ACTIVE' AND (
		d.responsible_user_id=? OR s.account_owner_id=?
		OR EXISTS(SELECT 1 FROM content_grant g WHERE g.document_id=d.id AND g.user_id=? AND g.state='ACTIVE' AND g.scope_kind='DOCUMENT' AND g.capability IN ('REVIEW','MANAGE'))
		OR EXISTS(SELECT 1 FROM knowledge_base_member k WHERE k.knowledge_base_id=d.knowledge_base_id AND k.user_id=? AND k.state='ACTIVE' AND k.role='ADMIN')`
	args := []any{last, actor, actor, actor, actor}
	if len(organizations) > 0 {
		statement += " OR s.organization_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(organizations)), ",") + ")"
		for _, organization := range organizations {
			args = append(args, organization)
		}
	}
	statement += ") ORDER BY r.id LIMIT ?"
	args = append(args, limit)
	rows, err := q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

````

## 第 18 步：ReviewRoundIDs

插入位置：第 17 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`func ReviewRoundIDs(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_round WHERE request_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, request, last, limit)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_round WHERE request_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, request, last, limit)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_round WHERE request_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, request, last, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT id FROM review_round WHERE request_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, request, last, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT id FROM review_round WHERE request_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, request, last, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=11196:11684 -->
````go
func ReviewRoundIDs(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, "SELECT id FROM review_round WHERE request_id=? AND id>? ORDER BY id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		values = append(values, id)
	}
	return values, rows.Err()
}

````

## 第 19 步：ReviewChanges

插入位置：第 18 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func ReviewChanges(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ChangeRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.re …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.re …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.re …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.re …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.ChangeRequest</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.re …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.Id, &amp;value.RequestId, &amp;value.RoundId, &amp;value.ContentVersionId, &amp;value.AuthorId, &amp;value.Status, &amp;value.Reason, &amp;value.ResolutionReason, &amp;value.Revision, &amp;created)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=11684:12832 -->
````go
func ReviewChanges(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ChangeRequest, error) {
	rows, err := q.QueryContext(ctx, "SELECT CAST(c.id AS CHAR),CAST(c.request_id AS CHAR),CAST(c.origin_round_id AS CHAR),CAST(a.content_version_id AS CHAR),CAST(c.author_id AS CHAR),c.status,a.reason,z.reason,CAST(c.revision AS CHAR),c.created_at FROM change_request c JOIN review_action a ON a.id=c.created_action_id AND a.request_id=c.request_id LEFT JOIN review_action z ON z.id=c.resolved_action_id AND z.request_id=c.request_id WHERE c.request_id=? AND c.id>? ORDER BY c.id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.ChangeRequest{}
	for rows.Next() {
		var value dto.ChangeRequest
		var created time.Time
		if err = rows.Scan(&value.Id, &value.RequestId, &value.RoundId, &value.ContentVersionId, &value.AuthorId, &value.Status, &value.Reason, &value.ResolutionReason, &value.Revision, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}

````

## 第 20 步：ReviewActions

插入位置：第 19 步代码之后，紧接追加；完成稿约第 243 行。

本步声明：`func ReviewActions(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ReviewAction, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST( …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>request</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST( …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST( …</code>（长表达式，按本步完整代码继续） |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST( …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.ReviewAction</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST( …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.Id, &amp;value.RequestId, &amp;value.RoundId, &amp;value.ActorId, &amp;value.Type, &amp;value.ContentVersionId, &amp;value.Reason, &amp;value.ChangeRequestId, &amp;value.MergeRecordId, &amp;created)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=12832:14005 -->
````go
func ReviewActions(ctx context.Context, q Queryer, request uint64, last string, limit int) ([]dto.ReviewAction, error) {
	rows, err := q.QueryContext(ctx, "SELECT CAST(a.id AS CHAR),CAST(a.request_id AS CHAR),CAST(a.round_id AS CHAR),CAST(a.actor_id AS CHAR),a.kind,CAST(a.content_version_id AS CHAR),a.reason,CAST(COALESCE(a.target_change_id,c.id) AS CHAR),CAST(a.merge_record_id AS CHAR),a.created_at FROM review_action a LEFT JOIN change_request c ON c.created_action_id=a.id AND c.request_id=a.request_id WHERE a.request_id=? AND a.id>? ORDER BY a.id LIMIT ?", request, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.ReviewAction{}
	for rows.Next() {
		var value dto.ReviewAction
		var created time.Time
		if err = rows.Scan(&value.Id, &value.RequestId, &value.RoundId, &value.ActorId, &value.Type, &value.ContentVersionId, &value.Reason, &value.ChangeRequestId, &value.MergeRecordId, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}

// review_round_style.go：保留原实现，按业务合并文件。

````

## 第 21 步：frozenRoundStyle

插入位置：第 20 步代码之后，紧接追加；完成稿约第 264 行。

本步声明：`func frozenRoundStyle(raw []byte) (string, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;fixed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>style</code>；<code>domain.TextID(legacy)</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil</code>。条件成立时执行 <code>return &quot;&quot;, false, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fixed.Style, &amp;style) == nil</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return style, false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;, false, invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.Unmarshal(fixed.Style, &amp;legacy) != nil &#124;&#124; legacy == 0</code>。条件成立时执行 <code>return &quot;&quot;, false, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return domain.TextID(legacy), true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=14005:14722 -->
````go
func frozenRoundStyle(raw []byte) (string, bool, error) {
	invalid := errors.New("REVIEW_ROUND_CONTEXT_INVALID")
	var fixed struct {
		Style json.RawMessage `json:"style_id"`
	}
	if json.Unmarshal(raw, &fixed) != nil {
		return "", false, invalid
	}
	var style string
	if json.Unmarshal(fixed.Style, &style) == nil {
		if _, err := domain.CanonicalID(style); err == nil {
			return style, false, nil
		}
		return "", false, invalid
	}
	// 第一版REVERSE生产者写过数值ID。只按uint64精确解析，不经float64，不接受指数、小数或零。
	var legacy uint64
	if json.Unmarshal(fixed.Style, &legacy) != nil || legacy == 0 {
		return "", false, invalid
	}
	return domain.TextID(legacy), true, nil
}

````

## 第 22 步：validateLegacyReverseStyle

插入位置：第 21 步代码之后，紧接追加；完成稿约第 287 行。

本步声明：`func validateLegacyReverseStyle(ctx context.Context, tx *sql.Tx, operation, document, version, style string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续）；<code>original.Context.Operation != operation</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续）；<code>domain.TextID(original.Scope.Document) != document</code> |
| <code>version</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续） |
| <code>style</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.TextID(original.Scope.Style) != style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;REVIEW_ROUND_CONTEXT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t 		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind=&#x27;REVERSE&#x27; AND r.state=&#x27;SUCCEEDED&#x27; 		JOIN conten …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;original) != nil &#124;&#124; original.Context.Operation != operation &#124;&#124; 		domain.TextID(original.Scope.Document) != document &#124;&#124; domain.TextID(original.Scope.Style) != style</code>。条件成立时执行 <code>return errors.New(&quot;REVIEW_ROUND_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=14722:15840 -->
````go
func validateLegacyReverseStyle(ctx context.Context, tx *sql.Tx, operation, document, version, style string) error {
	// 兼容仅限同一原REVERSE操作、真实REVERSE版本和不可变来源中的原样式；不改写旧任务或套用当前样式。
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT t.payload FROM durable_task t
		JOIN operation_receipt r ON r.operation_id=t.operation_id AND r.kind='REVERSE' AND r.state='SUCCEEDED'
		JOIN content_version v ON v.id=? AND v.document_id=? AND v.source_kind='REVERSE' AND v.source_ref=r.operation_id
		WHERE t.kind='REVERSE_CONTEXT' AND t.business_key=? AND t.operation_id=?`, version, document, operation, operation).Scan(&raw)
	if err != nil {
		return err
	}
	var original struct {
		Scope   struct{ Document, Style uint64 }
		Context struct {
			Operation string `json:"operation_id"`
		}
	}
	if json.Unmarshal(raw, &original) != nil || original.Context.Operation != operation ||
		domain.TextID(original.Scope.Document) != document || domain.TextID(original.Scope.Style) != style {
		return errors.New("REVIEW_ROUND_CONTEXT_INVALID")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/reviews_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
