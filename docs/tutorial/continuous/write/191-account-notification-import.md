# 分步编写 `services/document/internal/logic/document/account_notification_import.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/191-account-notification-import.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将Account服务的组织事件幂等导入Document通知库并回ACK。

账号原事件行号只用于内部确认，本地通知ID由Document决定，避免跨服务ID混用。

写之前先按这条执行路线理解各段的连接：

1. Recover每轮取一批事件，校验固定目标后在本地事务Insert，成功后通过AccountNotificationSource确认原事件。

本文件需要守住的边界：

- 先确认远端再提交本地会丢通知
- 来源接口不提供用户登录资格。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/account_notification_import.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/account_notification_import.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/account_notification_import.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/account_notification_import.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:210 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/repository/notification"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
)

````

## 第 2 步：AccountNotificationSource

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

AccountNotificationSource只传递服务间事件与确认，不承担用户请求的身份授权。

本步声明：`type AccountNotificationSource interface {`。

**这一段为什么这样写：** 以拉取加确认接口导入账号事件，两个数据库没有共享事务，依靠事件身份去重和重试连接两端。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Events</code> | <code>(context.Context) (&#91;&#93;platform.AccountNotificationEvent, error)</code> | 传入ctx，获取Account尚未确认投递的事件列表；消费后先在Doc事务中落库。 |
| <code>AckEvents</code> | <code>(context.Context, &#91;&#93;uint64) error</code> | 传入ctx与已持久导入的事件编号，再确认Account来源事件；不能先ACK后写入。 |

<!-- write-step: 2 bytes=210:474 -->
````go
// AccountNotificationSource只传递服务间事件与确认，不承担用户请求的身份授权。
type AccountNotificationSource interface {
	Events(context.Context) ([]platform.AccountNotificationEvent, error)
	AckEvents(context.Context, []uint64) error
}

````

## 第 3 步：AccountNotificationImport

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`type AccountNotificationImport struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>source</code> | <code>AccountNotificationSource</code> | 使用 <code>AccountNotificationSource</code> 类型；其定义处的职责：以拉取加确认接口导入账号事件，两个数据库没有共享事务，依靠事件身份去重和重试连接两端。 |

<!-- write-step: 3 bytes=474:567 -->
````go
type AccountNotificationImport struct {
	db     *sql.DB
	source AccountNotificationSource
}

````

## 第 4 步：NewAccountNotificationImport

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewAccountNotificationImport(db *sql.DB, source AccountNotificationSource) *AccountNotificationImport`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>source</code> | <code>AccountNotificationSource</code> | 使用 <code>AccountNotificationSource</code> 类型；其定义处的职责：以拉取加确认接口导入账号事件，两个数据库没有共享事务，依靠事件身份去重和重试连接两端。<br>在此声明中的实际使用：<code>source: source</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountNotificationImport</code> | 实际返回表达式：<code>&amp;AccountNotificationImport{db: db, source: source}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountNotificationImport{db: db, source: source}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=567:738 -->
````go
func NewAccountNotificationImport(db *sql.DB, source AccountNotificationSource) *AccountNotificationImport {
	return &AccountNotificationImport{db: db, source: source}
}

````

## 第 5 步：AccountNotificationImport.Recover

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

Recover每轮最多导入一批；Account原始通知行号只用于内部ACK，不成为前端通知ID。

本步声明：`func (s *AccountNotificationImport) Recover(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.source.Events(ctx)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>notification.Insert(ctx, tx, event.RecipientID, event.EventID, event.Kind, notification.Target{Organization: domain.TextID(event.OrganizationID)})</code>；<code>s.source.AckEvents(ctx, ids)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;ACCOUNT_NOTIFICATION_BATCH_INVALID&quot;)</code>；<code>s.source.AckEvents(ctx, ids)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(events) == 0</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>len(events) &gt; 100</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_BATCH_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return s.source.AckEvents(ctx, ids)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=738:1779 -->
````go
// Recover每轮最多导入一批；Account原始通知行号只用于内部ACK，不成为前端通知ID。
func (s *AccountNotificationImport) Recover(ctx context.Context) error {
	events, err := s.source.Events(ctx)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	if len(events) > 100 {
		return errors.New("ACCOUNT_NOTIFICATION_BATCH_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]uint64, 0, len(events))
	for _, event := range events {
		if err = notification.Insert(ctx, tx, event.RecipientID, event.EventID, event.Kind, notification.Target{Organization: domain.TextID(event.OrganizationID)}); err != nil {
			return err
		}
		ids = append(ids, event.ID)
	}
	// 整批先持久化再ACK。提交未知不能确认；下次重放由收件人/事件/类型唯一键
	// 与完整目标比较收敛，仓储不会覆盖用户已经设置的read_at。
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.source.AckEvents(ctx, ids)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/account_notification_import.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
