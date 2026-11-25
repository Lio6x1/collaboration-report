# 分步编写 `services/account/internal/logic/account/notification_events.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/062-notification-events.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排通知队列读取及整批原子确认。

一批ACK中某个ID无效时，整批回滚使调用方可以原样重试。

写之前先按这条执行路线理解各段的连接：

1. Events返回repository中的未确认事件。
2. Acknowledge限制1至100个非零且不重复ID，升序后在同一事务逐个锁定并确认，最后Commit。

本文件需要守住的边界：

- 不存在或坏绑定转换为输入错误；数据库故障继续上抛，不能吞掉后确认其余事件。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/notification_events.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/notification_events.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/notification_events.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/notification_events.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:165 -->
````go
package account

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
)

````

## 第 2 步：ErrNotificationEventsInput

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`var ErrNotificationEventsInput = errors.New("NOTIFICATION_EVENTS_INVALID")`。

<!-- write-step: 2 bytes=165:241 -->
````go
var ErrNotificationEventsInput = errors.New("NOTIFICATION_EVENTS_INVALID")

````

## 第 3 步：NotificationEvents

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type NotificationEvents struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 3 bytes=241:287 -->
````go
type NotificationEvents struct{ db *sql.DB }

````

## 第 4 步：NotificationEvents.Ready

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func (s *NotificationEvents) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.NotificationEventsReady(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>repository.NotificationEventsReady(ctx, s.db)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.NotificationEventsReady(ctx, s.db)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=287:408 -->
````go
func (s *NotificationEvents) Ready(ctx context.Context) error {
	return repository.NotificationEventsReady(ctx, s.db)
}

````

## 第 5 步：NewNotificationEvents

插入位置：第 4 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func NewNotificationEvents(db *sql.DB) *NotificationEvents`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationEvents</code> | 实际返回表达式：<code>&amp;NotificationEvents{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationEvents{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=408:507 -->
````go
func NewNotificationEvents(db *sql.DB) *NotificationEvents { return &NotificationEvents{db: db} }

````

## 第 6 步：NotificationEvents.Events

插入位置：第 5 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func (s *NotificationEvents) Events(ctx context.Context) (platform.AccountNotificationEventsResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingNotificationEvents(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountNotificationEventsResponse</code> | 使用 <code>platform.AccountNotificationEventsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>platform.AccountNotificationEventsResponse{Events: items}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return platform.AccountNotificationEventsResponse{Events: items}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=507:755 -->
````go
func (s *NotificationEvents) Events(ctx context.Context) (platform.AccountNotificationEventsResponse, error) {
	items, err := repository.PendingNotificationEvents(ctx, s.db)
	return platform.AccountNotificationEventsResponse{Events: items}, err
}

````

## 第 7 步：NotificationEvents.Acknowledge

插入位置：第 6 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (s *NotificationEvents) Acknowledge(ctx context.Context, input platform.AccountNotificationAckRequest) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.AcknowledgeNotificationEvent(ctx, tx, id)</code> |
| <code>input</code> | <code>platform.AccountNotificationAckRequest</code> | 使用 <code>platform.AccountNotificationAckRequest</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>input.IDs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotificationEventsInput</code>；<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(input.IDs) == 0 &#124;&#124; len(input.IDs) &gt; 100</code>。条件成立时执行 <code>return ErrNotificationEventsInput</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>id == 0 &#124;&#124; (i &gt; 0 &amp;&amp; ids&#91;i-1&#93; == id)</code>。条件成立时执行 <code>return ErrNotificationEventsInput</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; errors.Is(err, repository.ErrNotificationEventBinding)</code>。条件成立时执行 <code>return ErrNotificationEventsInput</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 7 bytes=755:1667 -->
````go
func (s *NotificationEvents) Acknowledge(ctx context.Context, input platform.AccountNotificationAckRequest) error {
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		return ErrNotificationEventsInput
	}
	ids := append([]uint64(nil), input.IDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] == id) {
			return ErrNotificationEventsInput
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 按稳定ID顺序加锁；任何不存在或不匹配的确认都回滚整批，不丢弃其他事件。
	for _, id := range ids {
		if err = repository.AcknowledgeNotificationEvent(ctx, tx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, repository.ErrNotificationEventBinding) {
				return ErrNotificationEventsInput
			}
			return err
		}
	}
	return tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/notification_events.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
