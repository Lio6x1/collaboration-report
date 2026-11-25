# 分步编写 `services/document/internal/repository/document/departure_notifications.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/150-departure-notifications.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在成员离组清空负责人前固定真实知识库管理员收件集合。

清空之后无法可靠重建当时责任事实，因此通知意图要与离组变更原子提交。

写之前先按这条执行路线理解各段的连接：

1. QueueDepartureNotifications要求已验证离组门禁
2. 按KB再文档锁序读取管理员
3. queueKnowledgeBaseDepartures写不可变回执与任务。

本文件需要守住的边界：

- 不能在事务外另取管理员名单
- 组织ADMIN不自动等于该KB管理员。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/departure_notifications.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/departure_notifications.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/departure_notifications.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/departure_notifications.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:142 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

````

## 第 2 步：QueueDepartureNotifications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

QueueDepartureNotifications 必须在已验证离组门禁的原事务内、清空负责人之前调用。
先按知识库顺序持锁，再锁文档，使真实管理员快照与负责人清空原子提交。

本步声明：`func QueueDepartureNotifications(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT k.id FROM knowledge_base k JOIN space s ON s.id=k.space_id 		WHERE s.kind=&#x27;ORGANIZATION&#x27; AND s.organization_id=? ORDER BY k.id FOR UPDATE`, binding.Org)</code>；<code>queueKnowledgeBaseDepartures(ctx, tx, binding, kb)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>queueKnowledgeBaseDepartures(ctx, tx, binding, kb)</code> |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>binding.Org</code>；<code>binding.User</code>；<code>binding.Epoch</code>；<code>binding.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DEPARTURE_NOTIFICATION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>binding.Org == 0 &#124;&#124; binding.User == 0 &#124;&#124; binding.Epoch == 0 &#124;&#124; !domain.ValidOperationID(binding.ID)</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_NOTIFICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT k.id FROM knowledge_base k JOIN space s ON s.id=k.space_id 		WHERE s.kind=&#x27;ORGANIZATION&#x27; AND s.organization_id=? ORDER BY k.id FOR UPDATE`, binding.Org)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=142:1267 -->
````go
// QueueDepartureNotifications 必须在已验证离组门禁的原事务内、清空负责人之前调用。
// 先按知识库顺序持锁，再锁文档，使真实管理员快照与负责人清空原子提交。
func QueueDepartureNotifications(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	if binding.Org == 0 || binding.User == 0 || binding.Epoch == 0 || !domain.ValidOperationID(binding.ID) {
		return errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	}
	rows, err := tx.QueryContext(ctx, `SELECT k.id FROM knowledge_base k JOIN space s ON s.id=k.space_id
		WHERE s.kind='ORGANIZATION' AND s.organization_id=? ORDER BY k.id FOR UPDATE`, binding.Org)
	if err != nil {
		return err
	}
	knowledgeBases := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		knowledgeBases = append(knowledgeBases, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, kb := range knowledgeBases {
		if err = queueKnowledgeBaseDepartures(ctx, tx, binding, kb); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 3 步：queueKnowledgeBaseDepartures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func queueKnowledgeBaseDepartures(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding, kb uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT d.id,d.control_revision FROM document d JOIN space s ON s.id=d.space_id 		JOIN knowledge_base k ON k.id=d.knowledge_base_id AND k.space_id=d.space_id 	 …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=? 		AND state=&#x27;ACTIVE&#x27; AND role=&#x27;ADMIN&#x27; AND user_id&lt;&gt;? ORDER BY user_id FOR UPDATE`, kb, binding.User)</code>；<code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 			VALUES(?,NULL,&#x27;OWNER_CHANGE&#x27;,&#x27;SUCCEEDED&#x27;,?,?) ON DUPLICATE KE …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, event.Event)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>binding.Org</code>；<code>binding.User</code>；<code>binding.Epoch</code>；<code>binding.ID</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT d.id,d.control_revision FROM document d JOIN space s ON s.id=d.space_id 		JOIN knowledge_base k ON k.id=d.knowledge_base_id AND k.space_id=d.space_id 	 …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=? 		AND state=&#x27;ACTIVE&#x27; AND role=&#x27;ADMIN&#x27; AND user_id&lt;&gt;? ORDER BY user_id FOR UPDATE`, kb, binding.User)</code>；<code>KnowledgeBase: kb</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DEPARTURE_NOTIFICATION_RECIPIENT_INVALID&quot;)</code>；<code>errors.New(&quot;DEPARTURE_NOTIFICATION_RECEIPT_CONFLICT&quot;)</code>；<code>errors.New(&quot;DEPARTURE_NOTIFICATION_TASK_CONFLICT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT d.id,d.control_revision FROM document d JOIN space s ON s.id=d.space_id 		JOIN knowledge_base k ON k.id=d.knowledge_base_id AND k.space_id=d.space_id 	 …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;doc.id, &amp;doc.revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; len(documents) == 0</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=? 		AND state=&#x27;ACTIVE&#x27; AND role=&#x27;ADMIN&#x27; AND user_id&lt;&gt;? ORDER BY user_id FOR UPDATE`, kb, binding.User)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;recipient)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>recipient == 0</code>。
- 此处通过 <code>return errors.New(&quot;DEPARTURE_NOTIFICATION_RECIPIENT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(recipients) == 0 &#124;&#124; recipients&#91;len(recipients)-1&#93; != recipient</code>。

<!-- write-step: 3 bytes=1267:2806 -->
````go
func queueKnowledgeBaseDepartures(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding, kb uint64) error {
	rows, err := tx.QueryContext(ctx, `SELECT d.id,d.control_revision FROM document d JOIN space s ON s.id=d.space_id
		JOIN knowledge_base k ON k.id=d.knowledge_base_id AND k.space_id=d.space_id
		WHERE s.kind='ORGANIZATION' AND s.organization_id=? AND d.knowledge_base_id=? AND d.responsible_user_id=?
		AND d.lifecycle IN ('ACTIVE','TRASHED') ORDER BY d.id FOR UPDATE`, binding.Org, kb, binding.User)
	if err != nil {
		return err
	}
	type candidate struct{ id, revision uint64 }
	documents := []candidate{}
	for rows.Next() {
		var doc candidate
		if err = rows.Scan(&doc.id, &doc.revision); err != nil {
			rows.Close()
			return err
		}
		documents = append(documents, doc)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(documents) == 0 {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member WHERE knowledge_base_id=?
		AND state='ACTIVE' AND role='ADMIN' AND user_id<>? ORDER BY user_id FOR UPDATE`, kb, binding.User)
	if err != nil {
		return err
	}
	recipients := []uint64{}
	for rows.Next() {
		var recipient uint64
		if err = rows.Scan(&recipient); err != nil {
			rows.Close()
			return err
		}
		if recipient == 0 {
			rows.Close()
			return errors.New("DEPARTURE_NOTIFICATION_RECIPIENT_INVALID")
		}
		if len(recipients) == 0 || recipients[len(recipients)-1] != recipient {
			recipients = append(recipients, recipient)
		}
	}
	err = rows.Err()
	rows.Close()
````

## 第 4 步：queueKnowledgeBaseDepartures（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 89 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 			VALUES(?,NULL,&#x27;OWNER_CHANGE&#x27;,&#x27;SUCCEEDED&#x27;,?,?) ON DUPLICATE KE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, event.Event).Scan(&amp;actor, &amp;kind, &amp;state, &amp;revision, &amp;plan)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, event.Event)</code>。
- 检查 <code>decodeErr != nil &#124;&#124; actor != 0 &#124;&#124; kind != &quot;OWNER_CHANGE&quot; &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; revision != doc.revision &#124;&#124; fixed.DeliveryAfter != 0 &#124;&#124; !domain.SameDepartureNotification(fixed, event)</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_NOTIFICATION_RECEIPT_CONFLICT&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 			VALUES(?,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, do …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(operation_id,&#x27;&#x27;),payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE`, domain.DepartureNotificationKind, event.Event).Scan(&amp;operation, &amp;payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(operation_id,&#x27;&#x27;),payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE`, domain.DepartureNotificationKind, event.Event)</code>。
- 检查 <code>decodeErr != nil &#124;&#124; operation != event.Event &#124;&#124; !domain.SameDepartureNotification(existing, event)</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_NOTIFICATION_TASK_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2806:5333 -->
````go
	if err != nil {
		return err
	}
	for _, doc := range documents {
		event := domain.DepartureNotification{Document: doc.id, KnowledgeBase: kb, Organization: binding.Org,
			Departed: binding.User, Epoch: binding.Epoch, Operation: binding.ID,
			Event: domain.DepartureNotificationEvent(binding.ID, doc.id), Recipients: recipients}
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		// 冲突只占位，不覆盖旧回执；逐字段校验防止同一操作编号绑定到另一次离组。
		// 这是关闭协议执行的系统动作；被移除成员不一定是发起者，不能冒充操作者。
		// 受影响成员已固定在plan.departed_user_id中，实际发起者沿原Account操作查询。
		_, err = tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
			VALUES(?,NULL,'OWNER_CHANGE','SUCCEEDED',?,?) ON DUPLICATE KEY UPDATE operation_id=operation_id`, event.Event, doc.revision, string(raw))
		if err != nil {
			return err
		}
		var actor, revision uint64
		var kind, state string
		var plan []byte
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(actor_id,0),kind,state,expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=? FOR UPDATE`, event.Event).Scan(&actor, &kind, &state, &revision, &plan); err != nil {
			return err
		}
		fixed, decodeErr := domain.DecodeDepartureNotification(plan)
		if decodeErr != nil || actor != 0 || kind != "OWNER_CHANGE" || state != "SUCCEEDED" || revision != doc.revision || fixed.DeliveryAfter != 0 || !domain.SameDepartureNotification(fixed, event) {
			return errors.New("DEPARTURE_NOTIFICATION_RECEIPT_CONFLICT")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
			VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, domain.DepartureNotificationKind, event.Event, event.Event, string(raw))
		if err != nil {
			return err
		}
		var operation string
		var payload []byte
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(operation_id,''),payload FROM durable_task WHERE kind=? AND business_key=? FOR UPDATE`, domain.DepartureNotificationKind, event.Event).Scan(&operation, &payload); err != nil {
			return err
		}
		existing, decodeErr := domain.DecodeDepartureNotification(payload)
		if decodeErr != nil || operation != event.Event || !domain.SameDepartureNotification(existing, event) {
			return errors.New("DEPARTURE_NOTIFICATION_TASK_CONFLICT")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/departure_notifications.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
