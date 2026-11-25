# 分步编写 `services/account/internal/repository/account/notifications_queries.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/052-notifications-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取通知管理员候选、锁定收件资格事实，并查询成员离开准备结果。

候选目录只是候选，实际投递必须由同事务权威行锁确认；未知SQL错误不能当作用户不存在。

写之前先按这条执行路线理解各段的连接：

1. NotificationAdministrators 用单SELECT JOIN组织、成员和user，多取第101项确定下一页。
2. NotificationRecipientUser/Org/Member 均在调用方tx内FOR UPDATE读取。
3. Departure 查询按组织→成员→回执锁定，供Doc确认离开原事务结果。

本文件需要守住的边界：

- 目录查询不签发永久资格；不能把 ACTIVE 候选直接当作稍后投递依据。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/notifications_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/notifications_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/notifications_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/notifications_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:278 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
)

// departure_status.go：保留原实现，按业务合并文件。

````

## 第 2 步：Organizations.LockDepartureOrganization

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func (r *Organizations) LockDepartureOrganization(ctx context.Context, tx *sql.Tx, organization string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(&amp;id)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(&amp;id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization).Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE&quot;, organization)</code>。

<!-- write-step: 2 bytes=278:531 -->
````go
func (r *Organizations) LockDepartureOrganization(ctx context.Context, tx *sql.Tx, organization string) error {
	var id string
	return tx.QueryRowContext(ctx, "SELECT CAST(id AS CHAR) FROM organization WHERE id=? FOR UPDATE", organization).Scan(&id)
}

````

## 第 3 步：Organizations.LockDepartureMember

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func (r *Organizations) LockDepartureMember(ctx context.Context, tx *sql.Tx, organization, user string) (string, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>epoch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, user).Scan(&amp;state, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE&quot;, organization, user)</code>。
- 此处通过 <code>return state, epoch, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=531:898 -->
````go
func (r *Organizations) LockDepartureMember(ctx context.Context, tx *sql.Tx, organization, user string) (string, string, error) {
	var state, epoch string
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(epoch AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, user).Scan(&state, &epoch)
	return state, epoch, err
}

````

## 第 4 步：Organizations.LockDepartureReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func (r *Organizations) LockDepartureReceipt(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;state)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;state)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE&quot;, operation)</code>。

<!-- write-step: 4 bytes=898:1230 -->
````go
func (r *Organizations) LockDepartureReceipt(ctx context.Context, tx *sql.Tx, operation string) error {
	var state string
	return tx.QueryRowContext(ctx, "SELECT state FROM operation_receipt WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
}

// notification_administrators.go：保留原实现，按业务合并文件。

````

## 第 5 步：NotificationAdministrators

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

单条本库查询保持候选目录的语句级一致性；资格锁及撤权来源仍由后续Check负责。
多取一条明确告知还有下一页，避免管理员超过100人时静默遗漏通知。

本步声明：`func NotificationAdministrators(ctx context.Context, db *sql.DB, organization, after uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT DISTINCT m.user_id 		FROM organization o 		JOIN organization_member m ON m.organization_id=o.id 		JOIN user u ON u.id=m.user_id 		WHERE o.id=? AND o.li …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT DISTINCT m.user_id 		FROM organization o 		JOIN organization_member m ON m.organization_id=o.id 		JOIN user u ON u.id=m.user_id 		WHERE o.id=? AND o.li …</code>（长表达式，按本步完整代码继续） |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT DISTINCT m.user_id 		FROM organization o 		JOIN organization_member m ON m.organization_id=o.id 		JOIN user u ON u.id=m.user_id 		WHERE o.id=? AND o.li …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT DISTINCT m.user_id 		FROM organization o 		JOIN organization_member m ON m.organization_id=o.id 		JOIN user u ON u.id=m.user_id 		WHERE o.id=? AND o.li …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return ids, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1230:2323 -->
````go
// 单条本库查询保持候选目录的语句级一致性；资格锁及撤权来源仍由后续Check负责。
// 多取一条明确告知还有下一页，避免管理员超过100人时静默遗漏通知。
func NotificationAdministrators(ctx context.Context, db *sql.DB, organization, after uint64) ([]uint64, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT m.user_id
		FROM organization o
		JOIN organization_member m ON m.organization_id=o.id
		JOIN user u ON u.id=m.user_id
		WHERE o.id=? AND o.lifecycle='ACTIVE' AND m.state='ACTIVE' AND u.state='ACTIVE'
		AND (m.role='ADMIN' OR m.user_id=o.owner_id) AND m.user_id>?
		ORDER BY m.user_id LIMIT ?`, organization, after, domain.NotificationAdministratorPageSize+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uint64, 0, domain.NotificationAdministratorPageSize+1)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// notification_recipients.go：保留原实现，按业务合并文件。

````

## 第 6 步：NotificationRecipientUser

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

所有资格事实来自同一事务权威行锁，SQL未知交给业务层报不可用，不能等同不存在。

本步声明：`func NotificationRecipientUser(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientIdentity, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>ID: id</code>；<code>tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.NotificationRecipientIdentity</code> | 使用 <code>domain.NotificationRecipientIdentity</code> 类型；其定义处的职责：表达通知收件人的账号/组织/成员权威快照，并规范批量锁顺序。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id).Scan(&amp;value.State, &amp;value.IdentityEpoch, &amp;value.CredentialEpoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2323:2822 -->
````go
// 所有资格事实来自同一事务权威行锁，SQL未知交给业务层报不可用，不能等同不存在。
func NotificationRecipientUser(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientIdentity, error) {
	value := domain.NotificationRecipientIdentity{ID: id}
	err := tx.QueryRowContext(ctx, `SELECT state,identity_epoch,credential_epoch FROM user WHERE id=? FOR UPDATE`, id).Scan(&value.State, &value.IdentityEpoch, &value.CredentialEpoch)
	return value, err
}

````

## 第 7 步：NotificationRecipientOrg

插入位置：第 6 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func NotificationRecipientOrg(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientOrganization, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.NotificationRecipientOrganization</code> | 使用 <code>domain.NotificationRecipientOrganization</code> 类型；其定义处的职责：表达通知收件人的账号/组织/成员权威快照，并规范批量锁顺序。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).Scan(&amp;value.Owner, &amp;value.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2822:3160 -->
````go
func NotificationRecipientOrg(ctx context.Context, tx *sql.Tx, id uint64) (domain.NotificationRecipientOrganization, error) {
	var value domain.NotificationRecipientOrganization
	err := tx.QueryRowContext(ctx, `SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).Scan(&value.Owner, &value.State)
	return value, err
}

````

## 第 8 步：NotificationRecipientMember

插入位置：第 7 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func NotificationRecipientMember(ctx context.Context, tx *sql.Tx, organization, user uint64) (domain.NotificationRecipientMembership, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.NotificationRecipientMembership</code> | 使用 <code>domain.NotificationRecipientMembership</code> 类型；其定义处的职责：表达通知收件人的账号/组织/成员权威快照，并规范批量锁顺序。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user).Scan(&amp;value.Role, &amp;value.State, &amp;value.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3160:3573 -->
````go
func NotificationRecipientMember(ctx context.Context, tx *sql.Tx, organization, user uint64) (domain.NotificationRecipientMembership, error) {
	var value domain.NotificationRecipientMembership
	err := tx.QueryRowContext(ctx, `SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user).Scan(&value.Role, &value.State, &value.Epoch)
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/notifications_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
