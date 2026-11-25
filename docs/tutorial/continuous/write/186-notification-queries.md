# 分步编写 `services/document/internal/repository/notification/notification_queries.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/186-notification-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存仅含跳转标识的通知，校验真实业务回执并领取/推进投递任务。

任务JSON不能自证业务成功，通知也不能永久携带用户撤权后仍可读的标题正文。

写之前先按这条执行路线理解各段的连接：

1. ReviewBinding/DepartureBinding校验原成功事实
2. CommentRecipients取得全量去重候选
3. LockedTask复核generation
4. Insert与FinishTask同事务
5. MarkRead只记首次时间。

本文件需要守住的边界：

- 候选名单不是当前投递许可
- 墓碑作者仍是参与者
- 超时重试CAS不能把已完成任务改回排队。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/notification' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/notification/notification_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/notification/notification_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/notification/notification_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/notification/notification_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:384 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package notification

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// comment_binding.go：保留原实现，按业务合并文件。

````

## 第 2 步：ValidateCommentReceipt

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func ValidateCommentReceipt(ctx context.Context, tx *sql.Tx, task Task, input domain.CommentNotification) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,result_json FROM operation_receipt WHERE operation_id=?&quot;, task.Operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.Operation</code> |
| <code>input</code> | <code>domain.CommentNotification</code> | 使用 <code>domain.CommentNotification</code> 类型；其定义处的职责：CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。<br>在此声明中的实际使用：<code>input.Actor</code>；<code>input.Comment</code>；<code>input.Thread</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;COMMENT_NOTIFICATION_RECEIPT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,result_json FROM operation_receipt WHERE operation_id=?&quot;, task.Operation).Scan(&amp;actor, &amp;kind, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,kind,state,result_json FROM operation_receipt WHERE operation_id=?&quot;, task.Operation)</code>。
- 检查 <code>json.Unmarshal(raw, &amp;result) != nil &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; result.State != &quot;SUCCEEDED&quot; &#124;&#124; actor != input.Actor</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;COMMENT_THREAD_CREATE&quot;</code>。
- 检查 <code>kind != &quot;COMMENT_REPLY&quot;</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Result.ID != domain.TextID(expected)</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=384:1385 -->
````go
func ValidateCommentReceipt(ctx context.Context, tx *sql.Tx, task Task, input domain.CommentNotification) error {
	var actor uint64
	var kind, state string
	var raw []byte
	if err := tx.QueryRowContext(ctx, "SELECT actor_id,kind,state,result_json FROM operation_receipt WHERE operation_id=?", task.Operation).Scan(&actor, &kind, &state, &raw); err != nil {
		return err
	}
	var result struct {
		State  string `json:"state"`
		Result struct {
			ID string `json:"entity_id"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &result) != nil || state != "SUCCEEDED" || result.State != "SUCCEEDED" || actor != input.Actor {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	expected := input.Comment
	if kind == "COMMENT_THREAD_CREATE" {
		expected = input.Thread
	} else if kind != "COMMENT_REPLY" {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	if result.Result.ID != domain.TextID(expected) {
		return errors.New("COMMENT_NOTIFICATION_RECEIPT_INVALID")
	}
	return nil
}

````

## 第 3 步：AdvanceTask

插入位置：第 2 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func AdvanceTask(ctx context.Context, tx *sql.Tx, task Task, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET payload=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6) 		WHERE id=? AND generation=? AND state=? AND kind=? AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.State</code>；<code>task.Kind</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET payload=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6) 		WHERE id=? AND generation=? AND state=? AND kind=? AND generation&lt;1844674 …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count != 1</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1385:1935 -->
````go
func AdvanceTask(ctx context.Context, tx *sql.Tx, task Task, payload []byte) error {
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET payload=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6)
		WHERE id=? AND generation=? AND state=? AND kind=? AND generation<18446744073709551615`, string(payload), task.ID, task.Generation, task.State, task.Kind)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("NOTIFICATION_TASK_CHANGED")
	}
	return nil
}

````

## 第 4 步：InvalidCommentTask

插入位置：第 3 步代码之后，紧接追加；完成稿约第 63 行。

已接管的排队任务可恢复，但绑定损坏不能通过就绪屏障。依据原回执和真实线程/评论，
不以“存在一个同名任务”或租约时间代替业务成功事实。

本步声明：`func InvalidCommentTask(ctx context.Context, db *sql.DB) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		WHERE t.kind=&#x27;COMMENT_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND 		(t.state NOT IN (&#x27;QUEUED …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>invalid</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		WHERE t.kind=&#x27;COMMENT_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND 		(t.state NOT IN (&#x27;QUEUED …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		WHERE t.kind=&#x27;COMMENT_NOTIFICATION&#x27; AND t.state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) AND 		(t.state NOT IN (&#x27;QUEUED …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return invalid, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1935:3533 -->
````go
// 已接管的排队任务可恢复，但绑定损坏不能通过就绪屏障。依据原回执和真实线程/评论，
// 不以“存在一个同名任务”或租约时间代替业务成功事实。
func InvalidCommentTask(ctx context.Context, db *sql.DB) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t
		WHERE t.kind='COMMENT_NOTIFICATION' AND t.state NOT IN ('SUCCEEDED','REJECTED') AND
		(t.state NOT IN ('QUEUED','RETRY','UNKNOWN') OR NOT EXISTS(
		SELECT 1 FROM operation_receipt o JOIN comment c ON c.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.comment_id'))
		JOIN comment_thread h ON h.id=c.thread_id
		WHERE o.operation_id=t.operation_id AND o.state='SUCCEEDED' AND t.business_key=o.operation_id
		AND o.actor_id=c.author_id AND c.author_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor_id'))
		AND h.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.thread_id'))
		AND h.document_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document_id'))
		AND h.context_kind=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context_kind'))
		AND h.context_ref=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context_id'))
		AND h.content_version_id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.content_version_id'))
		AND ((o.kind='COMMENT_THREAD_CREATE' AND JSON_UNQUOTE(JSON_EXTRACT(o.result_json,'$.result.entity_id'))=CAST(h.id AS CHAR))
		OR (o.kind='COMMENT_REPLY' AND JSON_UNQUOTE(JSON_EXTRACT(o.result_json,'$.result.entity_id'))=CAST(c.id AS CHAR))))))`).Scan(&invalid)
	return invalid, err
}

// departure_binding.go：保留原实现，按业务合并文件。

````

## 第 5 步：validateDepartureReceipt

插入位置：第 4 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func validateDepartureReceipt(task Task, businessKey string, actor uint64, kind, state string, plan []byte) (domain.DepartureNotification, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.Payload</code>；<code>task.Kind</code>；<code>task.Operation</code> |
| <code>businessKey</code> | <code>string</code> | 在此声明中的实际使用：<code>businessKey != event.Event</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor != 0</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind != &quot;OWNER_CHANGE&quot;</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>state != &quot;SUCCEEDED&quot;</code> |
| <code>plan</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>domain.DecodeDepartureNotification(plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.DepartureNotification</code> | 使用 <code>domain.DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>实际返回表达式：<code>event</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DEPARTURE_NOTIFICATION_RECEIPT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 检查 <code>task.Kind != domain.DepartureNotificationKind &#124;&#124; task.Operation != event.Event &#124;&#124; businessKey != event.Event &#124;&#124; actor != 0 &#124;&#124; kind != &quot;OWNER_CHANGE&quot; &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; fixe …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return event, errors.New(&quot;DEPARTURE_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return event, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3533:4233 -->
````go
func validateDepartureReceipt(task Task, businessKey string, actor uint64, kind, state string, plan []byte) (domain.DepartureNotification, error) {
	event, err := domain.DecodeDepartureNotification(task.Payload)
	if err != nil {
		return event, err
	}
	fixed, err := domain.DecodeDepartureNotification(plan)
	if err != nil {
		return event, err
	}
	if task.Kind != domain.DepartureNotificationKind || task.Operation != event.Event || businessKey != event.Event || actor != 0 || kind != "OWNER_CHANGE" || state != "SUCCEEDED" || fixed.DeliveryAfter != 0 || !domain.SameDepartureNotification(event, fixed) {
		return event, errors.New("DEPARTURE_NOTIFICATION_RECEIPT_INVALID")
	}
	return event, nil
}

````

## 第 6 步：DepartureBinding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 101 行。

DepartureBinding 只依赖原子提交的不可变离组回执，不依赖会再次打开的门禁，
也不要求负责人至今仍为空；当前文档范围和收件资格由投递事务单独复核。

本步声明：`func DepartureBinding(ctx context.Context, q Queryer, task Task) (domain.DepartureNotification, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT t.business_key,COALESCE(o.actor_id,0),o.kind,o.state,o.fixed_plan 		FROM durable_task t JOIN lifecycle_operation o ON o.operation_id=t.operation_id 	 …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Kind</code>；<code>task.Operation</code>；<code>task.Generation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.DepartureNotification</code> | 使用 <code>domain.DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>由 <code>validateDepartureReceipt(task, businessKey, actor, kind, state, plan)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>domain.DepartureNotification{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>validateDepartureReceipt(task, businessKey, actor, kind, state, plan)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT t.business_key,COALESCE(o.actor_id,0),o.kind,o.state,o.fixed_plan 		FROM durable_task t JOIN lifecycle_operation o ON o.operation_id=t.operation_id 	 …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.DepartureNotification{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return validateDepartureReceipt(task, businessKey, actor, kind, state, plan)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4233:5129 -->
````go
// DepartureBinding 只依赖原子提交的不可变离组回执，不依赖会再次打开的门禁，
// 也不要求负责人至今仍为空；当前文档范围和收件资格由投递事务单独复核。
func DepartureBinding(ctx context.Context, q Queryer, task Task) (domain.DepartureNotification, error) {
	var actor uint64
	var businessKey, kind, state string
	var plan []byte
	err := q.QueryRowContext(ctx, `SELECT t.business_key,COALESCE(o.actor_id,0),o.kind,o.state,o.fixed_plan
		FROM durable_task t JOIN lifecycle_operation o ON o.operation_id=t.operation_id
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.generation=?`, task.ID, task.Kind, task.Operation, task.Generation).
		Scan(&businessKey, &actor, &kind, &state, &plan)
	if err != nil {
		return domain.DepartureNotification{}, err
	}
	return validateDepartureReceipt(task, businessKey, actor, kind, state, plan)
}

````

## 第 7 步：InvalidDepartureTasks

插入位置：第 6 步代码之后，紧接追加；完成稿约第 117 行。

InvalidDepartureTasks 仅检查新通知类型，以有界批次联查不可变回执；
不逐条往返数据库，也不扫描历史上尚未接管的其他任务类型。

本步声明：`func InvalidDepartureTasks(ctx context.Context, db *sql.DB) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT t.id,t.generation,t.kind,COALESCE(t.operation_id,&#x27;&#x27;),t.state,t.payload, 			t.business_key,COALESCE(o.actor_id,0),COALESCE(o.kind,&#x27;&#x27;),COALESCE(o.state,&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT t.id,t.generation,t.kind,COALESCE(t.operation_id,&#x27;&#x27;),t.state,t.payload, 			t.business_key,COALESCE(o.actor_id,0),COALESCE(o.kind,&#x27;&#x27;),COALESCE(o.state,&#x27; …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;task.ID, &amp;task.Generation, &amp;task.Kind, &amp;task.Operation, &amp;task.State, &amp;task.Payload, &amp;businessKey, &amp;actor, &amp;kind, &amp;state, &amp;plan)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>task.State != &quot;QUEUED&quot; &amp;&amp; task.State != &quot;RETRY&quot; &amp;&amp; task.State != &quot;UNKNOWN&quot;</code>。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>count &lt; 100</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=5129:6729 -->
````go
// InvalidDepartureTasks 仅检查新通知类型，以有界批次联查不可变回执；
// 不逐条往返数据库，也不扫描历史上尚未接管的其他任务类型。
func InvalidDepartureTasks(ctx context.Context, db *sql.DB) (bool, error) {
	after := uint64(0)
	for {
		rows, err := db.QueryContext(ctx, `SELECT t.id,t.generation,t.kind,COALESCE(t.operation_id,''),t.state,t.payload,
			t.business_key,COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),o.fixed_plan
			FROM durable_task t LEFT JOIN lifecycle_operation o ON o.operation_id=t.operation_id
			WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>?
			ORDER BY t.id LIMIT 100`, domain.DepartureNotificationKind, after)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var task Task
			var actor uint64
			var businessKey, kind, state string
			var plan []byte
			if err = rows.Scan(&task.ID, &task.Generation, &task.Kind, &task.Operation, &task.State, &task.Payload, &businessKey, &actor, &kind, &state, &plan); err != nil {
				rows.Close()
				return false, err
			}
			if task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN" {
				rows.Close()
				return true, nil
			}
			if _, err = validateDepartureReceipt(task, businessKey, actor, kind, state, plan); err != nil {
				rows.Close()
				return true, nil
			}
			after = task.ID
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count < 100 {
			return false, nil
		}
	}
}

// notifications.go：保留原实现，按业务合并文件。

````

## 第 8 步：Target

插入位置：第 7 步代码之后，紧接追加；完成稿约第 164 行。

Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。

本步声明：`type Target struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Organization</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;organization_id,omitempty&quot;`</code> |
| <code>KnowledgeBase</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;knowledge_base_id,omitempty&quot;`</code> |
| <code>Document</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Thread</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;thread_id,omitempty&quot;`</code> |
| <code>Request</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;request_id,omitempty&quot;`</code> |
| <code>Round</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;round_id,omitempty&quot;`</code> |

<!-- write-step: 8 bytes=6729:7178 -->
````go
// Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。
type Target struct {
	Organization  string `json:"organization_id,omitempty"`
	KnowledgeBase string `json:"knowledge_base_id,omitempty"`
	Document      string `json:"document_id"`
	Thread        string `json:"thread_id,omitempty"`
	Request       string `json:"request_id,omitempty"`
	Round         string `json:"round_id,omitempty"`
}

````

## 第 9 步：Entry

插入位置：第 8 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`type Entry struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 通知主键。 |
| <code>Recipient</code> | <code>uint64</code> | 这条通知的接收账号；读取/已读操作须核验归属。 |
| <code>Event</code> | <code>string</code> | 来源事件标识，配合接收者等键去重投递。 |
| <code>Kind</code> | <code>string</code> | 通知业务种类，决定展示与目标解释。 |
| <code>Target</code> | <code>Target</code> | 使用 <code>Target</code> 类型；其定义处的职责：Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。 |
| <code>ReadAt</code> | <code>*time.Time</code> | 已读时间，nil表示尚未读。 |

<!-- write-step: 9 bytes=7178:7293 -->
````go
type Entry struct {
	ID, Recipient uint64
	Event, Kind   string
	Target        Target
	ReadAt        *time.Time
}

````

## 第 10 步：Queryer

插入位置：第 9 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`type Queryer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>QueryRowContext</code> | <code>(context.Context, string, ...any) *sql.Row</code> | 依次接收ctx、带占位符的SQL和占位符参数；返回单行句柄，实际查询/无行错误在Scan时处理。接口让连接池或原事务都可注入，但涉及锁时必须沿用原事务。 |
| <code>QueryContext</code> | <code>(context.Context, string, ...any) (*sql.Rows, error)</code> | 依次接收ctx、SQL和占位符参数，返回多行游标及错误；调用方关闭Rows、逐行Scan，并在循环后检查Rows.Err。 |

<!-- write-step: 10 bytes=7293:7446 -->
````go
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

````

## 第 11 步：scanner

插入位置：第 10 步代码之后，紧接追加；完成稿约第 186 行。

本步声明：`type scanner interface{ Scan(...any) error }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scan</code> | <code>(...any) error</code> | 按SELECT列顺序传入接收变量的指针；返回解码/查询错误。抽象Row与Rows共用的扫描能力，不负责开启或提交事务。 |

<!-- write-step: 11 bytes=7446:7492 -->
````go
type scanner interface{ Scan(...any) error }

````

## 第 12 步：columns

插入位置：第 11 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`const columns = "id,recipient_id,event_id,kind,target_json,read_at"`。

<!-- write-step: 12 bytes=7492:7561 -->
````go
const columns = "id,recipient_id,event_id,kind,target_json,read_at"

````

## 第 13 步：scan

插入位置：第 12 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func scan(row scanner) (Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>scanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Entry</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 执行 <code>row.Scan(&amp;value.ID, &amp;value.Recipient, &amp;value.Event, &amp;value.Kind, &amp;raw, &amp;read)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>json.Unmarshal(raw, &amp;value.Target) != nil &#124;&#124; !validTarget(value.Kind, value.Target)</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>read.Valid</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7561:8004 -->
````go
func scan(row scanner) (Entry, error) {
	var value Entry
	var raw []byte
	var read sql.NullTime
	if err := row.Scan(&value.ID, &value.Recipient, &value.Event, &value.Kind, &raw, &read); err != nil {
		return value, err
	}
	if json.Unmarshal(raw, &value.Target) != nil || !validTarget(value.Kind, value.Target) {
		return value, errors.New("NOTIFICATION_TARGET_INVALID")
	}
	if read.Valid {
		value.ReadAt = &read.Time
	}
	return value, nil
}

````

## 第 14 步：validTarget

插入位置：第 13 步代码之后，紧接追加；完成稿约第 206 行。

组织事件与文档事件有不同的授权来源，不能用空文档或混合目标绕过校验。

本步声明：`func validTarget(kind string, target Target) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;ORG_OWNER_TRANSFER&quot;</code>；<code>kind == &quot;RESPONSIBLE_DEPARTURE&quot;</code> |
| <code>target</code> | <code>Target</code> | 使用 <code>Target</code> 类型；其定义处的职责：Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。<br>在此声明中的实际使用：<code>target.Organization</code>；<code>target.Document</code>；<code>target.KnowledgeBase</code>；<code>target.Thread</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>target.Organization != &quot;&quot; &amp;&amp; target.Document == &quot;&quot; &amp;&amp; target.KnowledgeBase == &quot;&quot; &amp;&amp; target.Thread == &quot;&quot; &amp;&amp; target.Request == &quot;&quot; &amp;&amp; target.Round == &quot;&quot;</code>；<code>target.Document != &quot;&quot; &amp;&amp; target.KnowledgeBase != &quot;&quot; &amp;&amp; target.Organization == &quot;&quot; &amp;&amp; target.Thread == &quot;&quot; &amp;&amp; target.Request == &quot;&quot; &amp;&amp; target.Round == &quot;&quot;</code>；<code>target.Document != &quot;&quot; &amp;&amp; target.Organization == &quot;&quot; &amp;&amp; target.KnowledgeBase == &quot;&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>kind == &quot;ORG_OWNER_TRANSFER&quot;</code>。条件成立时执行 <code>return target.Organization != &quot;&quot; &amp;&amp; target.Document == &quot;&quot; &amp;&amp; target.KnowledgeBase == &quot;&quot; &amp;&amp; target.Thread == &quot;&quot; &amp;&amp; target.Request == &quot;&quot; &amp;&amp; target.Round == &quot;&quot;</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;RESPONSIBLE_DEPARTURE&quot;</code>。条件成立时执行 <code>return target.Document != &quot;&quot; &amp;&amp; target.KnowledgeBase != &quot;&quot; &amp;&amp; target.Organization == &quot;&quot; &amp;&amp; target.Thread == &quot;&quot; &amp;&amp; target.Request == &quot;&quot; &amp;&amp; target.Round == &quot;&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return target.Document != &quot;&quot; &amp;&amp; target.Organization == &quot;&quot; &amp;&amp; target.KnowledgeBase == &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8004:8651 -->
````go
// 组织事件与文档事件有不同的授权来源，不能用空文档或混合目标绕过校验。
func validTarget(kind string, target Target) bool {
	if kind == "ORG_OWNER_TRANSFER" {
		return target.Organization != "" && target.Document == "" && target.KnowledgeBase == "" && target.Thread == "" && target.Request == "" && target.Round == ""
	}
	if kind == "RESPONSIBLE_DEPARTURE" {
		return target.Document != "" && target.KnowledgeBase != "" && target.Organization == "" && target.Thread == "" && target.Request == "" && target.Round == ""
	}
	return target.Document != "" && target.Organization == "" && target.KnowledgeBase == ""
}

````

## 第 15 步：Row

插入位置：第 14 步代码之后，紧接追加；完成稿约第 217 行。

本步声明：`func Row(ctx context.Context, q Queryer, id, recipient uint64, lock bool) (Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id, recipient)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id, recipient)</code> |
| <code>recipient</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, query, id, recipient)</code> |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Entry</code> | 由 <code>scan(q.QueryRowContext(ctx, query, id, recipient))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scan(q.QueryRowContext(ctx, query, id, recipient))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 此处通过 <code>return scan(q.QueryRowContext(ctx, query, id, recipient))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>q.QueryRowContext(ctx, query, id, recipient)</code>。

<!-- write-step: 15 bytes=8651:8926 -->
````go
func Row(ctx context.Context, q Queryer, id, recipient uint64, lock bool) (Entry, error) {
	query := "SELECT " + columns + " FROM notification WHERE id=? AND recipient_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scan(q.QueryRowContext(ctx, query, id, recipient))
}

````

## 第 16 步：Candidates

插入位置：第 15 步代码之后，紧接追加；完成稿约第 225 行。

本步声明：`func Candidates(ctx context.Context, q Queryer, recipient, after uint64, limit int) ([]Entry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM notification WHERE recipient_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, recipient, after, limit)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>recipient</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM notification WHERE recipient_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, recipient, after, limit)</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM notification WHERE recipient_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, recipient, after, limit)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM notification WHERE recipient_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, recipient, after, limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;Entry</code> | 实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT &quot;+columns+&quot; FROM notification WHERE recipient_id=? AND id&gt;? ORDER BY id LIMIT ?&quot;, recipient, after, limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return items, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8926:9404 -->
````go
func Candidates(ctx context.Context, q Queryer, recipient, after uint64, limit int) ([]Entry, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+columns+" FROM notification WHERE recipient_id=? AND id>? ORDER BY id LIMIT ?", recipient, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

````

## 第 17 步：MarkRead

插入位置：第 16 步代码之后，紧接追加；完成稿约第 242 行。

已读时间只在第一次操作落定；原号重试及另一个已读操作都不改写时间。

本步声明：`func MarkRead(ctx context.Context, tx *sql.Tx, id, recipient uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?&quot;, id, recipient)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?&quot;, id, recipient)</code> |
| <code>recipient</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?&quot;, id, recipient)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?&quot;, id, recipient)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9404:9808 -->
````go
// 已读时间只在第一次操作落定；原号重试及另一个已读操作都不改写时间。
func MarkRead(ctx context.Context, tx *sql.Tx, id, recipient uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE notification SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE id=? AND recipient_id=?", id, recipient)
	return err
}

// recipients.go：保留原实现，按业务合并文件。

````

## 第 18 步：Facts

插入位置：第 17 步代码之后，紧接追加；完成稿约第 250 行。

Facts只读取调用方传入的同一连接/事务，不能用另一个池拼接不同时刻的权限来源。
组织管理员来自调用方已核验的身份服务结果，本仓储不跨私有库猜测组织角色。

本步声明：`func Facts(ctx context.Context, q Queryer, document dto.Document, space dto.Space, user uint64, orgAdmin bool) (domain.PermissionFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m 			JOIN knowledge_base k ON k.id=m.knowledge_base_id 			WHERE m.knowledge_base_id=? AND m.user_id=? AND  …</code>（长表达式，按本步完整代码继续）；<code>grants(ctx, q, id, user)</code>；<code>units(ctx, q, id, document.CurrentStructureId)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code>；<code>grants(ctx, q, id, user)</code>；<code>units(ctx, q, id, document.CurrentStructureId)</code> |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>document.SpaceId</code>；<code>document.Id</code>；<code>document.ResponsibleUserId</code>；<code>document.KnowledgeBaseId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>space</code> | <code>dto.Space</code> | 在此声明中的实际使用：<code>space.Id</code>；<code>space.Kind</code>；<code>space.OwnerUserId</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Actor: user</code>；<code>user == 0</code>；<code>domain.TextID(user)</code>；<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m 			JOIN knowledge_base k ON k.id=m.knowledge_base_id 			WHERE m.knowledge_base_id=? AND m.user_id=? AND  …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>orgAdmin</code> | <code>bool</code> | 在此声明中的实际使用：<code>space.Kind == &quot;ORGANIZATION&quot; &amp;&amp; orgAdmin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PermissionFacts</code> | 使用 <code>domain.PermissionFacts</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_PERMISSION_TARGET_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>user == 0 &#124;&#124; document.SpaceId != space.Id</code>。条件成立时执行 <code>return facts, errors.New(&quot;NOTIFICATION_PERMISSION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 检查 <code>document.KnowledgeBaseId != nil</code>。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m 			JOIN knowledge_base k ON k.id=m.knowledge_base_id 			WHERE m.knowledge_base_id=? AND m.user_id=? AND  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m 			JOIN knowledge_base k ON k.id=m.knowledge_base_id 			WHERE m.knowledge_base_id=? AND m.user_id=? AND  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=9808:11336 -->
````go
// Facts只读取调用方传入的同一连接/事务，不能用另一个池拼接不同时刻的权限来源。
// 组织管理员来自调用方已核验的身份服务结果，本仓储不跨私有库猜测组织角色。
func Facts(ctx context.Context, q Queryer, document dto.Document, space dto.Space, user uint64, orgAdmin bool) (domain.PermissionFacts, error) {
	facts := domain.PermissionFacts{Actor: user}
	if user == 0 || document.SpaceId != space.Id {
		return facts, errors.New("NOTIFICATION_PERMISSION_TARGET_INVALID")
	}
	id, err := domain.CanonicalID(document.Id)
	if err != nil {
		return facts, err
	}
	actor := domain.TextID(user)
	facts.Responsible = document.ResponsibleUserId != nil && *document.ResponsibleUserId == actor
	facts.PersonalOwner = space.Kind == "PERSONAL" && space.OwnerUserId != nil && *space.OwnerUserId == actor
	facts.OrganizationAdmin = space.Kind == "ORGANIZATION" && orgAdmin
	if document.KnowledgeBaseId != nil {
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_base_member m
			JOIN knowledge_base k ON k.id=m.knowledge_base_id
			WHERE m.knowledge_base_id=? AND m.user_id=? AND m.state='ACTIVE' AND m.role='ADMIN'
			AND k.space_id=? AND k.lifecycle='ACTIVE')`, *document.KnowledgeBaseId, user, space.Id).Scan(&facts.KnowledgeBaseAdmin)
		if err != nil {
			return facts, err
		}
	}
	facts.Grants, err = grants(ctx, q, id, user)
	if err != nil {
		return facts, err
	}
	facts.Units, err = units(ctx, q, id, document.CurrentStructureId)
	return facts, err
}

````

## 第 19 步：grants

插入位置：第 18 步代码之后，紧接追加；完成稿约第 282 行。

本步声明：`func grants(ctx context.Context, q Queryer, document, user uint64) ([]domain.GrantFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant 		WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id`, document, user)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant 		WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id`, document, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant 		WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id`, document, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.GrantFact</code> | 使用 <code>domain.GrantFact</code> 类型；其定义处的职责：权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant 		WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27; ORDER BY id`, document, user)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.User, &amp;value.Kind, &amp;value.Scope, &amp;value.Capability)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=11336:11939 -->
````go
func grants(ctx context.Context, q Queryer, document, user uint64) ([]domain.GrantFact, error) {
	rows, err := q.QueryContext(ctx, `SELECT user_id,scope_kind,scope_id,capability FROM content_grant
		WHERE document_id=? AND user_id=? AND state='ACTIVE' ORDER BY id`, document, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.GrantFact{}
	for rows.Next() {
		var value domain.GrantFact
		if err = rows.Scan(&value.User, &value.Kind, &value.Scope, &value.Capability); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

````

## 第 20 步：units

插入位置：第 19 步代码之后，紧接追加；完成稿约第 300 行。

本步声明：`func units(ctx context.Context, q Queryer, document uint64, structure string) ([]domain.UnitFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u 		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? 		W …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u 		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? 		W …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u 		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? 		W …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.UnitFact</code> | 使用 <code>domain.UnitFact</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u 		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? 		W …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;value.ID, &amp;value.Chapter)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11939:12620 -->
````go
func units(ctx context.Context, q Queryer, document uint64, structure string) ([]domain.UnitFact, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT u.id,u.chapter_id FROM collaboration_unit u
		JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=?
		WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id`, structure, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.UnitFact{}
	for rows.Next() {
		var value domain.UnitFact
		if err = rows.Scan(&value.ID, &value.Chapter); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

````

## 第 21 步：CommentRecipients

插入位置：第 20 步代码之后，紧接追加；完成稿约第 319 行。

返回真实参与者全量去重集合；墓碑仍保留作者身份，不静默截断而导致后面的参与者永久漏通知。
当前读取资格由上层逐人重新核验，此处的参与事实本身不等于最终投递许可。

本步声明：`func CommentRecipients(ctx context.Context, q Queryer, thread, actor uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT author_id FROM ( 		SELECT author_id FROM comment_thread WHERE id=? 		UNION SELECT author_id FROM comment WHERE thread_id=? 	) participants WHERE author_ …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>thread</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT author_id FROM ( 		SELECT author_id FROM comment_thread WHERE id=? 		UNION SELECT author_id FROM comment WHERE thread_id=? 	) participants WHERE author_ …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT author_id FROM ( 		SELECT author_id FROM comment_thread WHERE id=? 		UNION SELECT author_id FROM comment WHERE thread_id=? 	) participants WHERE author_ …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>values</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENT_INVALID&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT author_id FROM ( 		SELECT author_id FROM comment_thread WHERE id=? 		UNION SELECT author_id FROM comment WHERE thread_id=? 	) participants WHERE author_ …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;user)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>user == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;NOTIFICATION_RECIPIENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return values, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=12620:13601 -->
````go
// 返回真实参与者全量去重集合；墓碑仍保留作者身份，不静默截断而导致后面的参与者永久漏通知。
// 当前读取资格由上层逐人重新核验，此处的参与事实本身不等于最终投递许可。
func CommentRecipients(ctx context.Context, q Queryer, thread, actor uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT author_id FROM (
		SELECT author_id FROM comment_thread WHERE id=?
		UNION SELECT author_id FROM comment WHERE thread_id=?
	) participants WHERE author_id<>? ORDER BY author_id`, thread, thread, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []uint64{}
	for rows.Next() {
		var user uint64
		if err = rows.Scan(&user); err != nil {
			return nil, err
		}
		if user == 0 {
			return nil, errors.New("NOTIFICATION_RECIPIENT_INVALID")
		}
		values = append(values, user)
	}
	return values, rows.Err()
}

// review_binding.go：保留原实现，按业务合并文件。

````

## 第 22 步：ReviewEvent

插入位置：第 21 步代码之后，紧接追加；完成稿约第 346 行。

ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。

本步声明：`type ReviewEvent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 审核事件所属文档。 |
| <code>Request</code> | <code>uint64</code> | 审核请求编号。 |
| <code>Round</code> | <code>uint64</code> | 事件发生的固定轮次。 |
| <code>Unit</code> | <code>uint64</code> | 审核对应协作单元。 |
| <code>Actor</code> | <code>uint64</code> | 执行审核动作的账号。 |
| <code>Kind</code> | <code>string</code> | 审核事件种类。 |
| <code>Payload</code> | <code>map&#91;string&#93;string</code> | 事件扩展字段；须与真实审核动作、回执互相核对后投递。 |

<!-- write-step: 22 bytes=13601:13891 -->
````go
// ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。
type ReviewEvent struct {
	Document, Request, Round, Unit, Actor uint64
	Kind                                  string
	Payload                               map[string]string
}

````

## 第 23 步：readIDs

插入位置：第 22 步代码之后，紧接追加；完成稿约第 353 行。

本步声明：`func readIDs(rows *sql.Rows) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rows</code> | <code>*sql.Rows</code> | 在此声明中的实际使用：<code>rows.Close</code>；<code>rows.Next</code>；<code>rows.Scan</code>；<code>rows.Err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_CANDIDATE_INVALID&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>id == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;NOTIFICATION_CANDIDATE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=13891:14211 -->
````go
func readIDs(rows *sql.Rows) ([]uint64, error) {
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, errors.New("NOTIFICATION_CANDIDATE_INVALID")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

````

## 第 24 步：ReviewBinding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 369 行。

本步声明：`func ReviewBinding(ctx context.Context, q Queryer, task Task) (ReviewEvent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT unit_id,submitter_id,round_no,submission_operation_id FROM review_round 		WHERE id=? AND request_id=? AND document_id=?`, event.Round, event.Request, event.Document)</code>；<code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state FROM operation_receipt WHERE operation_id=?`, task.Operation)</code>；<code>q.QueryRowContext(ctx, `SELECT actor_id,kind FROM review_action WHERE id=? AND request_id=? AND round_id=? AND operation_id=? AND item_no=0`, action, event.Request, event.Round, task.Operation)</code>；<code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merge_operation m JOIN merge_record r ON r.id=m.result_merge_id 			AND r.operation_id=m.operation_id AND r.request_id=m.request_ …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.Payload</code>；<code>task.Kind</code>；<code>task.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReviewEvent</code> | 使用 <code>ReviewEvent</code> 类型；其定义处的职责：ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。<br>实际返回表达式：<code>event</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REVIEW_NOTIFICATION_PAYLOAD_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;REVIEW_NOTIFICATION_RECEIPT_INVALID&quot;)</code>；<code>errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;event.Payload) != nil</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_PAYLOAD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>task.Kind == &quot;REVIEW_NOTIFICATION&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, `SELECT unit_id,submitter_id,round_no,submission_operation_id FROM review_round 		WHERE id=? AND request_id=? AND document_id=?`, event.Round, event.Request, event.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state FROM operation_receipt WHERE operation_id=?`, task.Operation).Scan(&amp;actor, &amp;receiptKind, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind,state FROM operation_receipt WHERE operation_id=?`, task.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;SUCCEEDED&quot; &#124;&#124; actor == 0</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>p&#91;&quot;kind&quot;&#93; != &quot;SUBMISSION&quot; &#124;&#124; submission != task.Operation &#124;&#124; submitter != actor &#124;&#124; (receiptKind != &quot;SUBMISSION&quot; &amp;&amp; receiptKind != &quot;REVERSE&quot;)</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>roundNo &gt; 1</code>。

<!-- write-step: 24 bytes=14211:15993 -->
````go
func ReviewBinding(ctx context.Context, q Queryer, task Task) (ReviewEvent, error) {
	var event ReviewEvent
	if json.Unmarshal(task.Payload, &event.Payload) != nil {
		return event, errors.New("REVIEW_NOTIFICATION_PAYLOAD_INVALID")
	}
	p := event.Payload
	docKey, requestKey, roundKey := "document", "request", "round"
	if task.Kind == "REVIEW_NOTIFICATION" {
		docKey, requestKey, roundKey = "document_id", "request_id", "round_id"
	}
	var err error
	if event.Document, err = domain.CanonicalID(p[docKey]); err != nil {
		return event, err
	}
	if event.Request, err = domain.CanonicalID(p[requestKey]); err != nil {
		return event, err
	}
	if event.Round, err = domain.CanonicalID(p[roundKey]); err != nil {
		return event, err
	}
	var submitter, roundNo uint64
	var submission string
	err = q.QueryRowContext(ctx, `SELECT unit_id,submitter_id,round_no,submission_operation_id FROM review_round
		WHERE id=? AND request_id=? AND document_id=?`, event.Round, event.Request, event.Document).
		Scan(&event.Unit, &submitter, &roundNo, &submission)
	if err != nil {
		return event, err
	}
	var actor uint64
	var receiptKind, state string
	err = q.QueryRowContext(ctx, `SELECT actor_id,kind,state FROM operation_receipt WHERE operation_id=?`, task.Operation).Scan(&actor, &receiptKind, &state)
	if err != nil {
		return event, err
	}
	if state != "SUCCEEDED" || actor == 0 {
		return event, errors.New("REVIEW_NOTIFICATION_RECEIPT_INVALID")
	}
	event.Actor = actor
	switch task.Kind {
	case "REVIEW_NOTIFICATION":
		if p["kind"] != "SUBMISSION" || submission != task.Operation || submitter != actor || (receiptKind != "SUBMISSION" && receiptKind != "REVERSE") {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		event.Kind = "REVIEW_SUBMITTED"
		if roundNo > 1 {
````

## 第 25 步：ReviewBinding（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 414 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind FROM review_action WHERE id=? AND request_id=? AND round_id=? AND operation_id=? AND item_no=0`, action, event.Request, event.Round, ta …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT actor_id,kind FROM review_action WHERE id=? AND request_id=? AND round_id=? AND operation_id=? AND item_no=0`, action, event.Request, event.Round, task.Operation)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 检查 <code>actualActor != actor &#124;&#124; p&#91;&quot;actor&quot;&#93; != domain.TextID(actor) &#124;&#124; actualKind != p&#91;&quot;event&quot;&#93;</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>actualKind == &quot;WITHDRAW&quot;</code>。
- 检查 <code>actualKind == &quot;ARCHIVE&quot;</code>。
- 检查 <code>receiptKind != expectedKind</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_RECEIPT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merge_operation m JOIN merge_record r ON r.id=m.result_merge_id 			AND r.operation_id=m.operation_id AND r.request_id=m.request_ …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merge_operation m JOIN merge_record r ON r.id=m.result_merge_id 			AND r.operation_id=m.operation_id AND r.request_id=m.request_ …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return event, err</code>。当前函数在这里结束。
- 检查 <code>!valid &#124;&#124; receiptKind != &quot;MERGE&quot; &#124;&#124; p&#91;&quot;event&quot;&#93; != &quot;APPROVE_AND_MERGE&quot; &#124;&#124; p&#91;&quot;actor&quot;&#93; != domain.TextID(actor)</code>。条件成立时执行 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return event, errors.New(&quot;REVIEW_NOTIFICATION_KIND_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=15993:17875 -->
````go
			event.Kind = "REVIEW_RESUBMITTED"
		}
	case "REVIEW_ACTION_NOTIFICATION":
		var action uint64
		if action, err = domain.CanonicalID(p["action"]); err != nil {
			return event, err
		}
		var actualActor uint64
		var actualKind string
		err = q.QueryRowContext(ctx, `SELECT actor_id,kind FROM review_action WHERE id=? AND request_id=? AND round_id=? AND operation_id=? AND item_no=0`, action, event.Request, event.Round, task.Operation).Scan(&actualActor, &actualKind)
		if err != nil {
			return event, err
		}
		if actualActor != actor || p["actor"] != domain.TextID(actor) || actualKind != p["event"] {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		expectedKind := actualKind
		if actualKind == "WITHDRAW" {
			expectedKind = "REVIEW_WITHDRAW"
		}
		if actualKind == "ARCHIVE" {
			expectedKind = "DISCARD_DRAFT"
		}
		if receiptKind != expectedKind {
			return event, errors.New("REVIEW_NOTIFICATION_RECEIPT_INVALID")
		}
		event.Kind = "REVIEW_ACTION"
	case "MERGE_NOTIFICATION":
		var record uint64
		if record, err = domain.CanonicalID(p["record"]); err != nil {
			return event, err
		}
		var valid bool
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM merge_operation m JOIN merge_record r ON r.id=m.result_merge_id
			AND r.operation_id=m.operation_id AND r.request_id=m.request_id AND r.round_id=m.round_id
			WHERE m.operation_id=? AND m.request_id=? AND m.round_id=? AND m.actor_id=? AND m.state='SUCCEEDED' AND r.id=?)`, task.Operation, event.Request, event.Round, actor, record).Scan(&valid)
		if err != nil {
			return event, err
		}
		if !valid || receiptKind != "MERGE" || p["event"] != "APPROVE_AND_MERGE" || p["actor"] != domain.TextID(actor) {
			return event, errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
		}
		event.Kind = "REVIEW_MERGED"
	default:
		return event, errors.New("REVIEW_NOTIFICATION_KIND_INVALID")
	}
````

## 第 26 步：ReviewBinding（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 460 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return event, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=17875:17897 -->
````go
	return event, nil
}

````

## 第 27 步：ReviewCandidates

插入位置：第 26 步代码之后，紧接追加；完成稿约第 463 行。

候选与资格分开：这里只读Doc自己的真实角色/参与记录，组织管理候选由Account提供。
首次领取后固化整个集合，后续分页恢复不把新加入的人混入旧投递。

本步声明：`func ReviewCandidates(ctx context.Context, q Queryer, event ReviewEvent) ([]uint64, []uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT user_id FROM ( 		SELECT responsible_user_id AS user_id FROM document WHERE id=? 		UNION SELECT s.account_owner_id FROM space s JOIN document d ON d.spac …</code>（长表达式，按本步完整代码继续）；<code>q.QueryContext(ctx, `SELECT user_id FROM ( 			SELECT user_id FROM request_submitter WHERE request_id=? 			UNION SELECT actor_id FROM review_action WHERE request_id=? 		) participan …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>event</code> | <code>ReviewEvent</code> | 使用 <code>ReviewEvent</code> 类型；其定义处的职责：ReviewEvent来自成功业务事实；任务JSON只用于核对绑定，不能凭任务自己声称成功。<br>在此声明中的实际使用：<code>event.Document</code>；<code>event.Actor</code>；<code>event.Kind</code>；<code>event.Request</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>participants</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT user_id FROM ( 		SELECT responsible_user_id AS user_id FROM document WHERE id=? 		UNION SELECT s.account_owner_id FROM space s JOIN document d ON d.spac …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>event.Kind == &quot;REVIEW_ACTION&quot; &#124;&#124; event.Kind == &quot;REVIEW_MERGED&quot;</code>。
- 执行 <code>q.QueryContext(ctx, `SELECT user_id FROM ( 			SELECT user_id FROM request_submitter WHERE request_id=? 			UNION SELECT actor_id FROM review_action WHERE request_id=? 		) participan …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return ids, participants, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=17897:19569 -->
````go
// 候选与资格分开：这里只读Doc自己的真实角色/参与记录，组织管理候选由Account提供。
// 首次领取后固化整个集合，后续分页恢复不把新加入的人混入旧投递。
func ReviewCandidates(ctx context.Context, q Queryer, event ReviewEvent) ([]uint64, []uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT user_id FROM (
		SELECT responsible_user_id AS user_id FROM document WHERE id=?
		UNION SELECT s.account_owner_id FROM space s JOIN document d ON d.space_id=s.id WHERE d.id=?
		UNION SELECT user_id FROM content_grant WHERE document_id=? AND state='ACTIVE' AND scope_kind='DOCUMENT' AND capability IN ('REVIEW','MANAGE')
		UNION SELECT m.user_id FROM knowledge_base_member m JOIN document d ON d.knowledge_base_id=m.knowledge_base_id
		WHERE d.id=? AND m.state='ACTIVE' AND m.role='ADMIN'
	) candidates WHERE user_id IS NOT NULL AND user_id<>? ORDER BY user_id`, event.Document, event.Document, event.Document, event.Document, event.Actor)
	if err != nil {
		return nil, nil, err
	}
	ids, err := readIDs(rows)
	if err != nil {
		return nil, nil, err
	}
	participants := []uint64{}
	if event.Kind == "REVIEW_ACTION" || event.Kind == "REVIEW_MERGED" {
		rows, err = q.QueryContext(ctx, `SELECT user_id FROM (
			SELECT user_id FROM request_submitter WHERE request_id=?
			UNION SELECT actor_id FROM review_action WHERE request_id=?
		) participants WHERE user_id<>? ORDER BY user_id`, event.Request, event.Request, event.Actor)
		if err != nil {
			return nil, nil, err
		}
		participants, err = readIDs(rows)
	}
	return ids, participants, err
}

// review_readiness.go：保留原实现，按业务合并文件。

````

## 第 28 步：InvalidReviewTask

插入位置：第 27 步代码之后，紧接追加；完成稿约第 496 行。

全部待投递审核任务须有真实成功回执与原轮次/动作/合并记录。候选快照的增加
不改变这些不可变字段；合法积压允许恢复，绑定损坏必须阻止就绪。

本步声明：`func InvalidReviewTask(ctx context.Context, db *sql.DB) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN review_round rr ON rr.id=COALESCE(J …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>invalid</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN review_round rr ON rr.id=COALESCE(J …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN review_round rr ON rr.id=COALESCE(J …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return invalid, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=19569:22157 -->
````go
// 全部待投递审核任务须有真实成功回执与原轮次/动作/合并记录。候选快照的增加
// 不改变这些不可变字段；合法积压允许恢复，绑定损坏必须阻止就绪。
func InvalidReviewTask(ctx context.Context, db *sql.DB) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM durable_task t
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		LEFT JOIN review_round rr ON rr.id=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.round')))
		LEFT JOIN review_action a ON a.id=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.action')) AND a.operation_id=t.operation_id AND a.item_no=0
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN merge_record r ON r.id=m.result_merge_id AND r.operation_id=m.operation_id
		WHERE t.kind IN ('REVIEW_NOTIFICATION','REVIEW_ACTION_NOTIFICATION','MERGE_NOTIFICATION')
		AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
		t.state NOT IN ('QUEUED','RETRY','UNKNOWN') OR COALESCE(
		t.business_key=o.operation_id AND o.state='SUCCEEDED' AND o.actor_id>0
		AND CAST(rr.document_id AS CHAR)=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.document')))
		AND CAST(rr.request_id AS CHAR)=COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request_id')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.request')))
		AND ((t.kind='REVIEW_NOTIFICATION' AND rr.submission_operation_id=o.operation_id AND rr.submitter_id=o.actor_id
		AND o.kind IN ('SUBMISSION','REVERSE') AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.kind'))='SUBMISSION')
		OR (t.kind='REVIEW_ACTION_NOTIFICATION' AND a.request_id=rr.request_id AND a.round_id=rr.id AND a.actor_id=o.actor_id
		AND CAST(a.actor_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))
		AND a.kind=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))
		AND o.kind=CASE a.kind WHEN 'WITHDRAW' THEN 'REVIEW_WITHDRAW' WHEN 'ARCHIVE' THEN 'DISCARD_DRAFT' ELSE a.kind END)
		OR (t.kind='MERGE_NOTIFICATION' AND o.kind='MERGE' AND m.state='SUCCEEDED' AND m.actor_id=o.actor_id
		AND m.request_id=rr.request_id AND m.round_id=rr.id AND r.request_id=m.request_id AND r.round_id=m.round_id
		AND CAST(r.id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.record'))
		AND CAST(o.actor_id AS CHAR)=JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.actor'))
		AND JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.event'))='APPROVE_AND_MERGE')),0)=0))`).Scan(&invalid)
	return invalid, err
}

// tasks.go：保留原实现，按业务合并文件。

````

## 第 29 步：Task

插入位置：第 28 步代码之后，紧接追加；完成稿约第 528 行。

本步声明：`type Task struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 待处理通知持久任务主键。 |
| <code>Generation</code> | <code>uint64</code> | 任务认领代次，防止旧worker覆盖后来处理者。 |
| <code>Kind</code> | <code>string</code> | 通知任务种类。 |
| <code>Operation</code> | <code>string</code> | 生成通知的原业务操作号。 |
| <code>State</code> | <code>string</code> | 任务队列中的当前状态。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 固定事件载荷，不直接把它当作仍有效权限。 |

<!-- write-step: 29 bytes=22157:22272 -->
````go
type Task struct {
	ID, Generation         uint64
	Kind, Operation, State string
	Payload                []byte
}

````

## 第 30 步：taskColumns

插入位置：第 29 步代码之后，紧接追加；完成稿约第 534 行。

本步声明：`const taskColumns = "id,generation,kind,COALESCE(operation_id,''),state,payload"`。

<!-- write-step: 30 bytes=22272:22354 -->
````go
const taskColumns = "id,generation,kind,COALESCE(operation_id,''),state,payload"

````

## 第 31 步：scanTask

插入位置：第 30 步代码之后，紧接追加；完成稿约第 536 行。

本步声明：`func scanTask(row scanner) (Task, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>scanner</code> | 在此声明中的实际使用：<code>row.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Task</code> | 实际返回表达式：<code>task</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>row.Scan(&amp;task.ID, &amp;task.Generation, &amp;task.Kind, &amp;task.Operation, &amp;task.State, &amp;task.Payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return task, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=22354:22535 -->
````go
func scanTask(row scanner) (Task, error) {
	var task Task
	err := row.Scan(&task.ID, &task.Generation, &task.Kind, &task.Operation, &task.State, &task.Payload)
	return task, err
}

````

## 第 32 步：NextTask

插入位置：第 31 步代码之后，紧接追加；完成稿约第 542 行。

此处只读最早候选，不提前持锁跨服务查询收件资格；最终事务必须LockedTask并复核原代次和状态。
显式kind集合避免尚未装配的通知处理器误取其他持久任务。

本步声明：`func NextTask(ctx context.Context, db *sql.DB, kinds []string, recent bool) (Task, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, query, args...)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |
| <code>kinds</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(kinds)</code> |
| <code>recent</code> | <code>bool</code> | true按ID降序优先取新任务，false升序先取旧任务；两者都只选达到重试时间的待执行状态。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Task</code> | 由 <code>scanTask(db.QueryRowContext(ctx, query, args...))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>Task{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanTask(db.QueryRowContext(ctx, query, args...))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>sql.ErrNoRows</code>；<code>errors.New(&quot;NOTIFICATION_TASK_KIND_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(kinds) == 0</code>。条件成立时执行 <code>return Task{}, sql.ErrNoRows</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;&quot;</code>。条件成立时执行 <code>return Task{}, errors.New(&quot;NOTIFICATION_TASK_KIND_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>recent</code>。
- 此处通过 <code>return scanTask(db.QueryRowContext(ctx, query, args...))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>db.QueryRowContext(ctx, query, args...)</code>。

<!-- write-step: 32 bytes=22535:23443 -->
````go
// 此处只读最早候选，不提前持锁跨服务查询收件资格；最终事务必须LockedTask并复核原代次和状态。
// 显式kind集合避免尚未装配的通知处理器误取其他持久任务。
func NextTask(ctx context.Context, db *sql.DB, kinds []string, recent bool) (Task, error) {
	if len(kinds) == 0 {
		return Task{}, sql.ErrNoRows
	}
	args := make([]any, len(kinds))
	marks := make([]string, len(kinds))
	for i, kind := range kinds {
		if kind == "" {
			return Task{}, errors.New("NOTIFICATION_TASK_KIND_INVALID")
		}
		args[i], marks[i] = kind, "?"
	}
	order := "ASC"
	if recent {
		order = "DESC"
	}
	query := "SELECT " + taskColumns + " FROM durable_task WHERE kind IN (" + strings.Join(marks, ",") + ") AND state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6) ORDER BY id " + order + " LIMIT 1"
	return scanTask(db.QueryRowContext(ctx, query, args...))
}

````

## 第 33 步：LockedTask

插入位置：第 32 步代码之后，紧接追加；完成稿约第 564 行。

本步声明：`func LockedTask(ctx context.Context, tx *sql.Tx, id uint64) (Task, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Task</code> | 由 <code>scanTask(tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>scanTask(tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return scanTask(tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+taskColumns+&quot; FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code>。

<!-- write-step: 33 bytes=23443:23634 -->
````go
func LockedTask(ctx context.Context, tx *sql.Tx, id uint64) (Task, error) {
	return scanTask(tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM durable_task WHERE id=? FOR UPDATE", id))
}

````

## 第 34 步：RetryTask

插入位置：第 33 步代码之后，紧接追加；完成稿约第 568 行。

失败只安排原任务重试，避免一个暂时不可用的目标饿死后面的通知；提交未知
若实际上已经推进代次/完成，条件更新不会把它退回排队状态。

本步声明：`func RetryTask(ctx context.Context, db *sql.DB, task Task) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 SECOND), 		attempts=IF(attempts&lt;4294967295,attempts+1,attempts),last_error= …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.ExecContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.Generation</code>；<code>task.State</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 SECOND), 		attempts=IF(attempts&lt;4294967295,attempts+1,attempts),last_error= …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=23634:24217 -->
````go
// 失败只安排原任务重试，避免一个暂时不可用的目标饿死后面的通知；提交未知
// 若实际上已经推进代次/完成，条件更新不会把它退回排队状态。
func RetryTask(ctx context.Context, db *sql.DB, task Task) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 SECOND),
		attempts=IF(attempts<4294967295,attempts+1,attempts),last_error='NOTIFICATION_DELIVERY_RETRY'
		WHERE id=? AND generation=? AND state=?`, task.ID, task.Generation, task.State)
	return err
}

````

## 第 35 步：FinishTask

插入位置：第 34 步代码之后，紧接追加；完成稿约第 577 行。

与各收件人的Insert使用同一业务事务；提交未知可重新执行，终态CAS防止第二次推进原任务。

本步声明：`func FinishTask(ctx context.Context, tx *sql.Tx, task Task) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL 		WHERE id=? AND generation=? AND state=? AND kind=? AND COALESCE(operation_id,&#x27;&#x27;)=? …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>task</code> | <code>Task</code> | 在此声明中的实际使用：<code>task.ID</code>；<code>task.State</code>；<code>task.Generation</code>；<code>task.Kind</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>task.ID == 0 &#124;&#124; (task.State != &quot;QUEUED&quot; &amp;&amp; task.State != &quot;RETRY&quot; &amp;&amp; task.State != &quot;UNKNOWN&quot;)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL 		WHERE id=? AND generation=? AND state=? AND kind=? AND COALESCE(operation_id,&#x27;&#x27;)=? …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return count == 1, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=24217:24910 -->
````go
// 与各收件人的Insert使用同一业务事务；提交未知可重新执行，终态CAS防止第二次推进原任务。
func FinishTask(ctx context.Context, tx *sql.Tx, task Task) (bool, error) {
	if task.ID == 0 || (task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN") {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL
		WHERE id=? AND generation=? AND state=? AND kind=? AND COALESCE(operation_id,'')=?`, task.ID, task.Generation, task.State, task.Kind, task.Operation)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

````

## 第 36 步：Insert

插入位置：第 35 步代码之后，紧接追加；完成稿约第 591 行。

本步声明：`func Insert(ctx context.Context, tx *sql.Tx, recipient uint64, event, kind string, target Target) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?) 		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>recipient</code> | <code>uint64</code> | 在此声明中的实际使用：<code>recipient == 0</code>；<code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?) 		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind)</code> |
| <code>event</code> | <code>string</code> | 在此声明中的实际使用：<code>event == &quot;&quot;</code>；<code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?) 		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;&quot;</code>；<code>validTarget(kind, target)</code>；<code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?) 		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>Target</code> | 使用 <code>Target</code> 类型；其定义处的职责：Target 只保存跳转标识，不复制评论、正文或文档标题，防止旧通知持有失权内容。<br>在此声明中的实际使用：<code>target.Document</code>；<code>target.Thread</code>；<code>target.Request</code>；<code>target.Round</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_IDENTITY_INVALID&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;NOTIFICATION_TARGET_CONFLICT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>recipient == 0 &#124;&#124; event == &quot;&quot; &#124;&#124; kind == &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_IDENTITY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!validTarget(kind, target)</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?) 		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind).Scan(&amp;existingKind, &amp;existingRaw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification 		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind)</code>。
- 检查 <code>decoder.Decode(&amp;existing) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; existingKind != kind &#124;&#124; existing != target</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TARGET_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=24910:26456 -->
````go
func Insert(ctx context.Context, tx *sql.Tx, recipient uint64, event, kind string, target Target) error {
	if recipient == 0 || event == "" || kind == "" {
		return errors.New("NOTIFICATION_IDENTITY_INVALID")
	}
	for _, id := range []string{target.Document, target.Thread, target.Request, target.Round, target.Organization, target.KnowledgeBase} {
		if id != "" {
			if _, err := domain.CanonicalID(id); err != nil {
				return errors.New("NOTIFICATION_TARGET_INVALID")
			}
		}
	}
	if !validTarget(kind, target) {
		return errors.New("NOTIFICATION_TARGET_INVALID")
	}
	raw, err := json.Marshal(target)
	if err != nil {
		return err
	}
	// 唯一键只负责幂等占位，重复投递绝不改写目标或read_at；随后持锁比较完整目标。
	_, err = tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,?,?)
		ON DUPLICATE KEY UPDATE id=id`, recipient, event, kind, string(raw))
	if err != nil {
		return err
	}
	var existingRaw []byte
	var existingKind string
	if err = tx.QueryRowContext(ctx, `SELECT kind,target_json FROM notification
		WHERE recipient_id=? AND event_id=? AND kind=? FOR UPDATE`, recipient, event, kind).Scan(&existingKind, &existingRaw); err != nil {
		return err
	}
	var existing Target
	decoder := json.NewDecoder(bytes.NewReader(existingRaw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&existing) != nil || decoder.Decode(new(any)) != io.EOF || existingKind != kind || existing != target {
		return errors.New("NOTIFICATION_TARGET_CONFLICT")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/notification/notification_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
