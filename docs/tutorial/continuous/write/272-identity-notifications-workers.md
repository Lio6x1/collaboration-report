# 分步编写 `services/document/internal/worker/identity_notifications_workers.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/272-identity-notifications-workers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

调度授权关闭、成员离组恢复和通知导入/投递。

授权关闭和通知都需服务重启后继续原持久任务，但业务终态仍由各用例决定。

写之前先按这条执行路线理解各段的连接：

1. RunAuthorizations调撤权及recoverDeparture
2. RunAccountNotifications导入账号事件
3. RunNotifications推进投递。

本文件需要守住的边界：

- 循环频率不构成停止证明
- 失败不能丢原事件或以新号重建通知。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/worker/identity_notifications_workers.go') -and (Get-Item -LiteralPath 'services/document/internal/worker/identity_notifications_workers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/worker/identity_notifications_workers.go')) { New-Item -ItemType File -Path 'services/document/internal/worker/identity_notifications_workers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:296 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"log/slog"
	"time"
)

// account_notifications.go：保留原实现，按业务合并文件。

````

## 第 2 步：RunAccountNotifications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func RunAccountNotifications(ctx context.Context, importer *logic.AccountNotificationImport)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>ctx.Err</code> |
| <code>importer</code> | <code>*logic.AccountNotificationImport</code> | 使用 <code>logic.AccountNotificationImport</code> 类型；其定义处的职责：将Account服务的组织事件幂等导入Document通知库并回ACK。<br>在此声明中的实际使用：<code>importer.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; ctx.Err() == nil</code>。

<!-- write-step: 2 bytes=296:1011 -->
````go
func RunAccountNotifications(ctx context.Context, importer *logic.AccountNotificationImport) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 每轮只有一个最多100条的批次，网络和事务共用4秒预算，不并发积压请求。
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := importer.Recover(attempt)
			cancel()
			if err != nil && ctx.Err() == nil {
				// 不打印网络错误、SQL参数或原始事件，避免把内部身份和目标写入日志。
				slog.Warn("ACCOUNT_NOTIFICATION_IMPORT_RETRY")
			}
		}
	}
}

// authorization.go：保留原实现，按业务合并文件。

````

## 第 3 步：RunAuthorizations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 36 行。

调度器只决定何时恢复；事务、关闭证明和业务终态由用例协调器统一维护。

本步声明：`func RunAuthorizations(ctx context.Context, authorizations *logic.Authorizations, recoverDeparture func(context.Context))`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>authorizations.RecoverOne(ctx)</code>；<code>recoverDeparture(ctx)</code> |
| <code>authorizations</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>authorizations.RecoverOne</code> |
| <code>recoverDeparture</code> | <code>func(context.Context)</code> | 在此声明中的实际使用：<code>recoverDeparture(ctx)</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1011:1515 -->
````go
// 调度器只决定何时恢复；事务、关闭证明和业务终态由用例协调器统一维护。
func RunAuthorizations(ctx context.Context, authorizations *logic.Authorizations, recoverDeparture func(context.Context)) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			authorizations.RecoverOne(ctx)
			recoverDeparture(ctx)
		}
	}
}

// notifications.go：保留原实现，按业务合并文件。

````

## 第 4 步：RunNotifications

插入位置：第 3 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func RunNotifications(ctx context.Context, delivery *logic.NotificationDelivery)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>ctx.Err</code> |
| <code>delivery</code> | <code>*logic.NotificationDelivery</code> | 使用 <code>logic.NotificationDelivery</code> 类型；其定义处的职责：按真实评论/审核/离组任务核收件资格并可靠投递。<br>在此声明中的实际使用：<code>delivery.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>errors.Is(failure, logic.ErrNotificationIdle)</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>failure != nil</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, logic.ErrNotificationIdle) &amp;&amp; ctx.Err() == nil</code>。

<!-- write-step: 4 bytes=1515:2414 -->
````go
func RunNotifications(ctx context.Context, delivery *logic.NotificationDelivery) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			// 同一4秒预算内分批清理积压；空队列立即退出，不用高频空查询消耗数据库。
			var err error
			for count := 0; count < 64 && attempt.Err() == nil; count++ {
				failure := delivery.Recover(attempt)
				if errors.Is(failure, logic.ErrNotificationIdle) {
					break
				}
				if failure != nil {
					// 失败任务已登记下一次重试时间，继续本轮其余候选，避免旧故障长期挡队。
					err = failure
				}
			}
			cancel()
			if err != nil && !errors.Is(err, logic.ErrNotificationIdle) && ctx.Err() == nil {
				slog.Warn("NOTIFICATION_DELIVERY_RETRY")
			}
		}
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/worker/identity_notifications_workers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
