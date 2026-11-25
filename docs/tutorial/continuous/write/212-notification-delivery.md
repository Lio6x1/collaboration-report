# 分步编写 `services/document/internal/logic/document/notification_delivery.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/212-notification-delivery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按真实评论/审核/离组任务核收件资格并可靠投递。

通知不能泄露已失权文档；远端账号短期证明必须与本地来源关闭水位在最终事务核验。

写之前先按这条执行路线理解各段的连接：

1. Recover先取候选任务，锁外Resolve身份
2. 解原CommentNotification
3. 锁任务及来源复核目标权限
4. 插每人通知并推进任务
5. Ready拒绝损坏业务绑定。

本文件需要守住的边界：

- 暂时不可用只安排原任务重试
- 短期收件证明过期必须重取，不能使用服务身份代替用户资格。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/notification_delivery.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/notification_delivery.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/notification_delivery.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/notification_delivery.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:409 -->
````go
package document

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	notices "collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync/atomic"
	"time"
)

````

## 第 2 步：NotificationRecipientResolver

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type NotificationRecipientResolver interface {`。

**这一段为什么这样写：** Doc不跨库读账号成员关系，通过Account返回权威接收人事实再核验内容可见性。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Resolve</code> | <code>(context.Context, uint64, &#91;&#93;uint64) (platform.NotificationRecipientsResponse, error)</code> | ctx、组织编号、候选账号列表交给Account，返回有效账号/成员证明。 |
| <code>Administrators</code> | <code>(context.Context, uint64, uint64) (platform.NotificationAdministratorsResponse, error)</code> | ctx、组织编号、分页起点查询管理员及证明；本轮结果仍需文档权限复核。 |

<!-- write-step: 2 bytes=409:654 -->
````go
type NotificationRecipientResolver interface {
	Resolve(context.Context, uint64, []uint64) (platform.NotificationRecipientsResponse, error)
	Administrators(context.Context, uint64, uint64) (platform.NotificationAdministratorsResponse, error)
}

````

## 第 3 步：NotificationDelivery

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type NotificationDelivery struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>comments</code> | <code>*Comments</code> | 使用 <code>Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。 |
| <code>recipients</code> | <code>NotificationRecipientResolver</code> | 使用 <code>NotificationRecipientResolver</code> 类型；其定义处的职责：Doc不跨库读账号成员关系，通过Account返回权威接收人事实再核验内容可见性。 |
| <code>turn</code> | <code>atomic.Uint64</code> | 原子调度轮数，用于在不同任务扫描顺序间轮换，避免长期只处理一端积压。 |

<!-- write-step: 3 bytes=654:802 -->
````go
type NotificationDelivery struct {
	db         *sql.DB
	comments   *Comments
	recipients NotificationRecipientResolver
	turn       atomic.Uint64
}

````

## 第 4 步：ErrNotificationIdle

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`var ErrNotificationIdle = errors.New("NOTIFICATION_QUEUE_IDLE")`。

<!-- write-step: 4 bytes=802:867 -->
````go
var ErrNotificationIdle = errors.New("NOTIFICATION_QUEUE_IDLE")

````

## 第 5 步：NotificationDelivery.Ready

插入位置：第 4 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (s *NotificationDelivery) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.InvalidCommentTask(ctx, s.db)</code>；<code>notices.InvalidReviewTask(ctx, s.db)</code>；<code>notices.InvalidDepartureTasks(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;COMMENT_NOTIFICATION_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;DEPARTURE_NOTIFICATION_BINDING_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>invalid</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>invalid</code>。条件成立时执行 <code>return errors.New(&quot;REVIEW_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>invalid</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=867:1444 -->
````go
func (s *NotificationDelivery) Ready(ctx context.Context) error {
	invalid, err := notices.InvalidCommentTask(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	invalid, err = notices.InvalidReviewTask(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("REVIEW_NOTIFICATION_BINDING_INVALID")
	}
	invalid, err = notices.InvalidDepartureTasks(ctx, s.db)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	}
	return nil
}

````

## 第 6 步：NewNotificationDelivery

插入位置：第 5 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func NewNotificationDelivery(db *sql.DB, comments *Comments, recipients NotificationRecipientResolver) *NotificationDelivery`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>comments</code> | <code>*Comments</code> | 使用 <code>Comments</code> 类型；其定义处的职责：执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。<br>在此声明中的实际使用：<code>comments: comments</code> |
| <code>recipients</code> | <code>NotificationRecipientResolver</code> | 使用 <code>NotificationRecipientResolver</code> 类型；其定义处的职责：Doc不跨库读账号成员关系，通过Account返回权威接收人事实再核验内容可见性。<br>在此声明中的实际使用：<code>recipients: recipients</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationDelivery</code> | 实际返回表达式：<code>&amp;NotificationDelivery{db: db, comments: comments, recipients: recipients}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationDelivery{db: db, comments: comments, recipients: recipients}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1444:1656 -->
````go
func NewNotificationDelivery(db *sql.DB, comments *Comments, recipients NotificationRecipientResolver) *NotificationDelivery {
	return &NotificationDelivery{db: db, comments: comments, recipients: recipients}
}

````

## 第 7 步：decodeCommentNotification

插入位置：第 6 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func decodeCommentNotification(raw []byte) (domain.CommentNotification, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CommentNotification</code> | 使用 <code>domain.CommentNotification</code> 类型；其定义处的职责：CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_NOTIFICATION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; value.Document == 0 &#124;&#124; value.Thread == 0 &#124;&#124; value.Comment == 0 &#124;&#124; value.Version == 0 &#124;&#124; value.Actor == 0 &#124;&#124; len(value.Recipients) &gt; 100</code>。条件成立时执行 <code>return value, errors.New(&quot;COMMENT_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1656:2250 -->
````go
func decodeCommentNotification(raw []byte) (domain.CommentNotification, error) {
	var value domain.CommentNotification
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || value.Document == 0 || value.Thread == 0 || value.Comment == 0 || value.Version == 0 || value.Actor == 0 || len(value.Recipients) > 100 {
		return value, errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	if _, err := domain.CanonicalID(value.ContextID); err != nil {
		return value, err
	}
	return value, nil
}

````

## 第 8 步：commentNotificationRecipients

插入位置：第 7 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func commentNotificationRecipients(value domain.CommentNotification) ([]uint64, map[uint64]bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.CommentNotification</code> | 使用 <code>domain.CommentNotification</code> 类型；其定义处的职责：CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。<br>在此声明中的实际使用：<code>value.Recipients</code>；<code>value.ReplyRecipients</code>；<code>value.Actor</code>；<code>value.DeliveryAfter</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>map&#91;uint64&#93;bool</code> | 实际返回表达式：<code>nil</code>；<code>mentions</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COMMENT_NOTIFICATION_RECIPIENT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>id == 0</code>。条件成立时执行 <code>return nil, nil, errors.New(&quot;COMMENT_NOTIFICATION_RECIPIENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id != value.Actor &amp;&amp; id &gt; value.DeliveryAfter</code>。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ids, mentions, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2250:2964 -->
````go
func commentNotificationRecipients(value domain.CommentNotification) ([]uint64, map[uint64]bool, error) {
	all := map[uint64]bool{}
	mentions := map[uint64]bool{}
	for _, id := range value.Recipients {
		if id == 0 {
			return nil, nil, errors.New("COMMENT_NOTIFICATION_RECIPIENT_INVALID")
		}
		mentions[id] = true
		all[id] = true
	}
	for _, id := range value.ReplyRecipients {
		if id == 0 {
			return nil, nil, errors.New("COMMENT_NOTIFICATION_RECIPIENT_INVALID")
		}
		all[id] = true
	}
	ids := []uint64{}
	for id := range all {
		if id != value.Actor && id > value.DeliveryAfter {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, mentions, nil
}

````

## 第 9 步：NotificationDelivery.Recover

插入位置：第 8 步代码之后，紧接追加；完成稿约第 102 行。

外部身份查询发生在SQL锁外；短期结果最终与Doc权威权限、来源关闭高水位同事务核验。

本步声明：`func (s *NotificationDelivery) Recover(ctx context.Context) (failure error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>notices.NextTask(ctx, s.db, &#91;&#93;string{&quot;COMMENT_NOTIFICATION&quot;, &quot;REVIEW_NOTIFICATION&quot;, &quot;REVIEW_ACTION_NOTIFICATION&quot;, &quot;MERGE_NOTIFICATION&quot;, &quot;RESPONSIBLE_DEPARTURE_NOTIFICATION&quot;}, recent)</code>；<code>context.WithoutCancel(ctx)</code>；<code>s.deliverDeparture(ctx, task)</code>；<code>s.deliverReview(ctx, task)</code><br>另有 13 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>failure</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotificationIdle</code>；<code>err</code>；<code>s.deliverDeparture(ctx, task)</code>；<code>s.deliverReview(ctx, task)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return ErrNotificationIdle</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>func() { 		if failure != nil { 			retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second) 			defer cancel() 			// 原失败仍返回给worker，重试登记失败也不能把本次当成成功。 			_ = notices.RetryTask(retry, s.db, task) 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>failure != nil</code>。
- 在本段的匿名函数内：执行 <code>context.WithTimeout(context.WithoutCancel(ctx), time.Second)</code>。
- 在本段的匿名函数内：登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>task.Kind == &quot;RESPONSIBLE_DEPARTURE_NOTIFICATION&quot;</code>。条件成立时执行 <code>return s.deliverDeparture(ctx, task)</code>。当前函数在这里结束。
- 检查 <code>task.Kind != &quot;COMMENT_NOTIFICATION&quot;</code>。条件成立时执行 <code>return s.deliverReview(ctx, task)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=2964:4673 -->
````go
// 外部身份查询发生在SQL锁外；短期结果最终与Doc权威权限、来源关闭高水位同事务核验。
func (s *NotificationDelivery) Recover(ctx context.Context) (failure error) {
	// 实际旧队列积压数千条时，新事件仍应及时可见；每8次保留1次给最新事件，
	// 其余继续最老任务。此选择不更改持久进度，重启不会影响去重或任务恢复。
	recent := s.turn.Add(1)%8 == 1
	task, err := notices.NextTask(ctx, s.db, []string{"COMMENT_NOTIFICATION", "REVIEW_NOTIFICATION", "REVIEW_ACTION_NOTIFICATION", "MERGE_NOTIFICATION", "RESPONSIBLE_DEPARTURE_NOTIFICATION"}, recent)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotificationIdle
	}
	if err != nil {
		return err
	}
	defer func() {
		if failure != nil {
			retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			// 原失败仍返回给worker，重试登记失败也不能把本次当成成功。
			_ = notices.RetryTask(retry, s.db, task)
		}
	}()
	if task.Kind == "RESPONSIBLE_DEPARTURE_NOTIFICATION" {
		return s.deliverDeparture(ctx, task)
	}
	if task.Kind != "COMMENT_NOTIFICATION" {
		return s.deliverReview(ctx, task)
	}
	input, err := decodeCommentNotification(task.Payload)
	if err != nil {
		return err
	}
	ids, mentions, err := commentNotificationRecipients(input)
	if err != nil {
		return err
	}
	more := len(ids) > 100
	if more {
		ids = ids[:100]
	}
	resolved, err := s.deliveryAccess(ctx, input.Document, ids)
	if err != nil {
		return err
	}
	doc, space, proof, sources, kbRevision := resolved.doc, resolved.space, resolved.proof, resolved.sources, resolved.kbRevision
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
````

## 第 10 步：NotificationDelivery.Recover（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 147 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.AclRevision != doc.AclRevision &#124;&#124; current.CurrentStructureId != doc.CurrentStructureId &#124;&#124; current.KnowledgeBaseId == nil != (doc.KnowledgeBaseId == nil)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;NOTIFICATION_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>t.Document != input.Document &#124;&#124; t.Kind != input.ContextKind &#124;&#124; t.Context != input.ContextID &#124;&#124; t.Version != input.Version &#124;&#124; entry.Thread != t.ID &#124;&#124; entry.Author != input.Actor</code>。条件成立时执行 <code>return errors.New(&quot;COMMENT_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>locked.Generation != task.Generation &#124;&#124; locked.State != task.State &#124;&#124; !bytes.Equal(locked.Payload, task.Payload)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Lifecycle == &quot;ACTIVE&quot; &amp;&amp; t.Status != &quot;DELETED&quot; &amp;&amp; !entry.Deleted</code>。
- 检查 <code>!recipient.Eligible</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 10 bytes=4673:6283 -->
````go
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return err
	}
	current, err := documents.DocumentRow(ctx, tx, input.Document, true)
	if err != nil {
		return err
	}
	if current.AclRevision != doc.AclRevision || current.CurrentStructureId != doc.CurrentStructureId || current.KnowledgeBaseId == nil != (doc.KnowledgeBaseId == nil) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_ACCESS_CHANGED")
	}
	t, err := comments.ThreadRow(ctx, tx, input.Thread, true)
	if err != nil {
		return err
	}
	entry, err := comments.CommentRow(ctx, tx, input.Comment, true)
	if err != nil {
		return err
	}
	if t.Document != input.Document || t.Kind != input.ContextKind || t.Context != input.ContextID || t.Version != input.Version || entry.Thread != t.ID || entry.Author != input.Actor {
		return errors.New("COMMENT_NOTIFICATION_BINDING_INVALID")
	}
	locked, err := notices.LockedTask(ctx, tx, task.ID)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.Generation != task.Generation || locked.State != task.State || !bytes.Equal(locked.Payload, task.Payload) {
		return domain.Violation(domain.StateConflict, "NOTIFICATION_TASK_CHANGED")
	}
	if err = notices.ValidateCommentReceipt(ctx, tx, task, input); err != nil {
		return err
	}
	if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	if current.Lifecycle == "ACTIVE" && t.Status != "DELETED" && !entry.Deleted {
		for _, recipient := range proof.Recipients {
			if !recipient.Eligible {
				continue
			}
````

## 第 11 步：NotificationDelivery.Recover（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 192 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Read</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。
- 检查 <code>notificationHidden(err)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>mentions&#91;recipient.UserID&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!finished</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_TASK_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof.ExpiresUnixMS &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_EXPIRED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=6283:7771 -->
````go
			facts, err := notices.Facts(ctx, tx, current, space, recipient.UserID, recipient.OrgOwner || recipient.Role == "ADMIN")
			if err != nil {
				return err
			}
			access := DocumentAccess{Document: current, Permissions: domain.ResolvePermissions(facts), KBRevision: kbRevision}
			if !access.Permissions.Read {
				continue
			}
			// 这里只复用纯业务上下文核验，没有伪造接收者会话或对外输出准入。
			if err = s.comments.validateContext(ctx, tx, Actor{ID: recipient.UserID}, access, t, false); err != nil {
				if notificationHidden(err) {
					continue
				}
				return err
			}
			kind := "COMMENT_REPLY"
			if mentions[recipient.UserID] {
				kind = "COMMENT_MENTION"
			}
			if err = notices.Insert(ctx, tx, recipient.UserID, task.Operation, kind, notices.Target{Document: doc.Id, Thread: domain.TextID(t.ID)}); err != nil {
				return err
			}
		}
	}
	if more {
		input.DeliveryAfter = ids[len(ids)-1]
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		if err = notices.AdvanceTask(ctx, tx, task, raw); err != nil {
			return err
		}
	} else {
		finished, err := notices.FinishTask(ctx, tx, task)
		if err != nil {
			return err
		}
		if !finished {
			return errors.New("NOTIFICATION_TASK_CHANGED")
		}
	}
	if proof.ExpiresUnixMS <= time.Now().UnixMilli() {
		return errors.New("NOTIFICATION_RECIPIENT_PROOF_EXPIRED")
	}
	// 插入与进度同提交；结果未知保留原任务，唯一键及绑定复核阻止重复提醒。
````

## 第 12 步：NotificationDelivery.Recover（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 238 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 12 bytes=7771:7793 -->
````go
	return tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/notification_delivery.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
