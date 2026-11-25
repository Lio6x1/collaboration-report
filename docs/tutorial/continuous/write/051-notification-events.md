# 分步编写 `services/account/internal/repository/account/notification_events.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/051-notification-events.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把所有者转移成功写成可靠通知事件，并实现拉取与确认SQL。

成功回执与通知事件同事务落库，避免转移完成但通知永久丢失。

写之前先按这条执行路线理解各段的连接：

1. CreateOwnerTransferNotice 核对SUCCEEDED transition/receipt，用原操作SHA256形成account-owner事件ID并插入QUEUED任务。
2. PendingNotificationEvents 每次读尚未确认任务，核对notification与payload绑定。
3. AcknowledgeNotificationEvent 重锁原事件及任务，再标SUCCEEDED，重复确认可成功。

本文件需要守住的边界：

- 不能用ID高水位代替未确认队列，并发事务可能晚提交小ID。
- 不存在或绑定不符的ACK必须拒绝，不接受调用者替换收件人。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/notification_events.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/notification_events.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/notification_events.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/notification_events.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:154 -->
````go
package account

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"collaboration/platform"
)

````

## 第 2 步：ErrNotificationEventBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var ErrNotificationEventBinding = errors.New("NOTIFICATION_EVENT_BINDING_INVALID")`。

<!-- write-step: 2 bytes=154:238 -->
````go
var ErrNotificationEventBinding = errors.New("NOTIFICATION_EVENT_BINDING_INVALID")

````

## 第 3 步：NotificationEventsReady

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

接管排队通知仍须证明任务来自成功的真实转移；坏绑定不能被后台队列豁免掩盖。

本步声明：`func NotificationEventsReady(ctx context.Context, db *sql.DB) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task d 		LEFT JOIN notification n ON d.business_key=CAST(n.id AS CHAR) 		LEFT JOIN organization_transition t ON t.operation_id …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrNotificationEventBinding</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task d 		LEFT JOIN notification n ON d.business_key=CAST(n.id AS CHAR) 		LEFT JOIN organization_transition t ON t.operation_id …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task d 		LEFT JOIN notification n ON d.business_key=CAST(n.id AS CHAR) 		LEFT JOIN organization_transition t ON t.operation_id …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>broken != 0</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=238:1481 -->
````go
// 接管排队通知仍须证明任务来自成功的真实转移；坏绑定不能被后台队列豁免掩盖。
func NotificationEventsReady(ctx context.Context, db *sql.DB) error {
	var broken int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_task d
		LEFT JOIN notification n ON d.business_key=CAST(n.id AS CHAR)
		LEFT JOIN organization_transition t ON t.operation_id=d.operation_id
		LEFT JOIN operation_receipt r ON r.operation_id=d.operation_id
		WHERE d.kind='ACCOUNT_NOTICE_DELIVERY' AND d.state NOT IN ('SUCCEEDED','REJECTED')
		AND NOT COALESCE(d.state='QUEUED' AND n.kind='ORG_OWNER_TRANSFER'
		AND t.kind='TRANSFER_OWNER' AND t.state='SUCCEEDED'
		AND r.kind='ORG_OWNER_TRANSFER' AND r.state='SUCCEEDED' AND r.actor_id=t.actor_id
		AND n.recipient_id=t.target_user_id
		AND JSON_UNQUOTE(JSON_EXTRACT(n.target_json,'$.organization_id'))=CAST(t.organization_id AS CHAR)
		AND n.event_id=CONCAT('account-owner:',LOWER(SHA2(t.operation_id,256)))
		AND d.payload=JSON_OBJECT('id',n.id,'recipient_id',n.recipient_id,'organization_id',t.organization_id,'event_id',n.event_id,'kind',n.kind),FALSE)`).Scan(&broken)
	if err != nil {
		return err
	}
	if broken != 0 {
		return ErrNotificationEventBinding
	}
	return nil
}

````

## 第 4 步：Closure.CreateOwnerTransferNotice

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

成功回执、真实转移记录和通知使用调用方的同一事务，避免终态已可见但事件丢失。

本步声明：`func (r *Closure) CreateOwnerTransferNotice(ctx context.Context, tx *sql.Tx, operation, organization, actor string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) 		VALUES(?,?,&#x27;ORG_OWNER_TRANSFER&#x27;,?) ON DUPLICATE KEY UPDATE id=id`, recipient, eventID, string(target))</code>；<code>tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind=&#x27;ORG_OWNER_TRANSFER&#x27; FOR UPDATE`, recipient, eventID)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_NOTICE_DELIVERY&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续）；<code>&#91;&#93;byte(operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_NOTICE_DELIVERY&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续）；<code>&quot;organization_id&quot;: organization</code>；<code>saved.OrganizationID != organization</code>；<code>strconv.ParseUint(organization, 10, 64)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrNotificationEventBinding</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t 		JOIN operation_receipt r ON r.operation_id=t.operation_id 		WHERE t.operation_id=? AND t.organizat …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json) 		VALUES(?,?,&#x27;ORG_OWNER_TRANSFER&#x27;,?) ON DUPLICATE KEY UPDATE id=id`, recipient, eventID, string(target))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind=&#x27;ORG_OWNER_TRANSFER&#x27; FOR UPDATE`, recipient, eventID).Scan(&amp;id, &amp;original)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind=&#x27;ORG_OWNER_TRANSFER&#x27; FOR UPDATE`, recipient, eventID)</code>。
- 检查 <code>json.Unmarshal(original, &amp;saved) != nil &#124;&#124; saved.OrganizationID != organization</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; orgID == 0 &#124;&#124; recipient == 0</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_NOTICE_DELIVERY&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICAT …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 4 bytes=1481:3832 -->
````go
// 成功回执、真实转移记录和通知使用调用方的同一事务，避免终态已可见但事件丢失。
func (r *Closure) CreateOwnerTransferNotice(ctx context.Context, tx *sql.Tx, operation, organization, actor string) error {
	var recipient uint64
	err := tx.QueryRowContext(ctx, `SELECT t.target_user_id FROM organization_transition t
		JOIN operation_receipt r ON r.operation_id=t.operation_id
		WHERE t.operation_id=? AND t.organization_id=? AND t.actor_id=?
		AND t.kind='TRANSFER_OWNER' AND t.state='SUCCEEDED'
		AND r.kind='ORG_OWNER_TRANSFER' AND r.state='SUCCEEDED' FOR UPDATE`, operation, organization, actor).Scan(&recipient)
	if err != nil {
		return err
	}
	// 所有操作号统一散列，避免短原号恰好等于另一长原号摘要时撞到同一事件编号。
	digest := sha256.Sum256([]byte(operation))
	eventID := "account-owner:" + hex.EncodeToString(digest[:])
	target, err := json.Marshal(map[string]string{"organization_id": organization})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO notification(recipient_id,event_id,kind,target_json)
		VALUES(?,?,'ORG_OWNER_TRANSFER',?) ON DUPLICATE KEY UPDATE id=id`, recipient, eventID, string(target)); err != nil {
		return err
	}
	var id uint64
	var original []byte
	if err = tx.QueryRowContext(ctx, `SELECT id,target_json FROM notification WHERE recipient_id=? AND event_id=? AND kind='ORG_OWNER_TRANSFER' FOR UPDATE`, recipient, eventID).Scan(&id, &original); err != nil {
		return err
	}
	var saved struct {
		OrganizationID string `json:"organization_id"`
	}
	if json.Unmarshal(original, &saved) != nil || saved.OrganizationID != organization {
		return ErrNotificationEventBinding
	}
	orgID, err := strconv.ParseUint(organization, 10, 64)
	if err != nil || orgID == 0 || recipient == 0 {
		return ErrNotificationEventBinding
	}
	event := platform.AccountNotificationEvent{ID: id, RecipientID: recipient, OrganizationID: orgID, EventID: eventID, Kind: "ORG_OWNER_TRANSFER"}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_NOTICE_DELIVERY',?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE id=id`, strconv.FormatUint(id, 10), operation, string(payload))
	return err
}

````

## 第 5 步：PendingNotificationEvents

插入位置：第 4 步代码之后，紧接追加；完成稿约第 88 行。

每次查询未确认任务，不采用ID高水位，避免并发事务按不同顺序提交时漏投。

本步声明：`func PendingNotificationEvents(ctx context.Context, db *sql.DB) ([]platform.AccountNotificationEvent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload 		FROM durable_task d JOIN notification n ON d.business_key=CAST(n.id AS CHAR) 		WHERE d. …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;platform.AccountNotificationEvent</code> | 使用 <code>platform.AccountNotificationEvent</code> 类型；其定义处的职责：Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload 		FROM durable_task d JOIN notification n ON d.business_key=CAST(n.id AS CHAR) 		WHERE d. …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;event.ID, &amp;event.RecipientID, &amp;event.EventID, &amp;event.Kind, &amp;target, &amp;payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return items, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3832:4820 -->
````go
// 每次查询未确认任务，不采用ID高水位，避免并发事务按不同顺序提交时漏投。
func PendingNotificationEvents(ctx context.Context, db *sql.DB) ([]platform.AccountNotificationEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload
		FROM durable_task d JOIN notification n ON d.business_key=CAST(n.id AS CHAR)
		WHERE d.kind='ACCOUNT_NOTICE_DELIVERY' AND d.state='QUEUED' ORDER BY d.id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []platform.AccountNotificationEvent{}
	for rows.Next() {
		var event platform.AccountNotificationEvent
		var target, payload []byte
		if err = rows.Scan(&event.ID, &event.RecipientID, &event.EventID, &event.Kind, &target, &payload); err != nil {
			return nil, err
		}
		if err = verifyNotificationEvent(&event, target, payload); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

````

## 第 6 步：verifyNotificationEvent

插入位置：第 5 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func verifyNotificationEvent(event *platform.AccountNotificationEvent, target, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>event</code> | <code>*platform.AccountNotificationEvent</code> | 使用 <code>platform.AccountNotificationEvent</code> 类型；其定义处的职责：Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。<br>在此声明中的实际使用：<code>event.OrganizationID</code>；<code>event.ID</code>；<code>event.RecipientID</code>；<code>event.EventID</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(target, &amp;saved)</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(payload, &amp;marker)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotificationEventBinding</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(target, &amp;saved) != nil &#124;&#124; json.Unmarshal(payload, &amp;marker) != nil</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; org == 0</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 检查 <code>event.ID == 0 &#124;&#124; event.RecipientID == 0 &#124;&#124; event.EventID == &quot;&quot; &#124;&#124; len(event.EventID) &gt; 96 &#124;&#124; event.Kind != &quot;ORG_OWNER_TRANSFER&quot; &#124;&#124; *event != marker</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4820:5535 -->
````go
func verifyNotificationEvent(event *platform.AccountNotificationEvent, target, payload []byte) error {
	var saved struct {
		OrganizationID string `json:"organization_id"`
	}
	var marker platform.AccountNotificationEvent
	if json.Unmarshal(target, &saved) != nil || json.Unmarshal(payload, &marker) != nil {
		return ErrNotificationEventBinding
	}
	org, err := strconv.ParseUint(saved.OrganizationID, 10, 64)
	if err != nil || org == 0 {
		return ErrNotificationEventBinding
	}
	event.OrganizationID = org
	if event.ID == 0 || event.RecipientID == 0 || event.EventID == "" || len(event.EventID) > 96 || event.Kind != "ORG_OWNER_TRANSFER" || *event != marker {
		return ErrNotificationEventBinding
	}
	return nil
}

````

## 第 7 步：AcknowledgeNotificationEvent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func AcknowledgeNotificationEvent(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state 		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS  …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=?`, taskID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state 		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrNotificationEventBinding</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state 		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state 		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;QUEUED&quot; &amp;&amp; state != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return ErrNotificationEventBinding</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=?`, taskID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5535:6559 -->
````go
func AcknowledgeNotificationEvent(ctx context.Context, tx *sql.Tx, id uint64) error {
	var event platform.AccountNotificationEvent
	var target, payload []byte
	var taskID uint64
	var state string
	// 重新锁定原始事件及对应任务，不接受调用者提供的收件人或事件内容替代绑定。
	err := tx.QueryRowContext(ctx, `SELECT n.id,n.recipient_id,n.event_id,n.kind,n.target_json,d.payload,d.id,d.state
		FROM notification n JOIN durable_task d ON d.business_key=CAST(n.id AS CHAR) AND d.kind='ACCOUNT_NOTICE_DELIVERY'
		WHERE n.id=? FOR UPDATE`, id).Scan(&event.ID, &event.RecipientID, &event.EventID, &event.Kind, &target, &payload, &taskID, &state)
	if err != nil {
		return err
	}
	if err = verifyNotificationEvent(&event, target, payload); err != nil {
		return err
	}
	if state != "QUEUED" && state != "SUCCEEDED" {
		return ErrNotificationEventBinding
	}
	_, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=?`, taskID)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/notification_events.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
