# 分步编写 `services/search/internal/worker/rebuild.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/402-rebuild.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

运行重建后台循环，并在启动普通消费前等待原SWITCHING恢复。

服务重启不能绕过持久屏障。后台失败只暂停同一任务，不能清空进度或重建另一个任务来制造成功。

写之前先按这条执行路线理解各段的连接：

1. RunAfterRecovery每秒用3秒期限检查原切换屏障，准备好才进入普通Run循环。
2. RunRebuild每200ms推进一次Step，本实例只运行一个重建循环。
3. 取消时关闭消费者/重建依赖；失败日志仅输出白名单固定码且连续相同错误不刷屏。

本文件需要守住的边界：

- ready失败不代表可以启动普通消费者；退出时尚未启动的消费者也要Close。
- nil错误表示本步无故障，不代表整次重建已经成功。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/worker/rebuild.go') -and (Get-Item -LiteralPath 'services/search/internal/worker/rebuild.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/worker/rebuild.go')) { New-Item -ItemType File -Path 'services/search/internal/worker/rebuild.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:69 -->
````go
package worker

import (
	"context"
	"log/slog"
	"regexp"
	"time"
)

````

## 第 2 步：rebuilder

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type rebuilder interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Step</code> | <code>(context.Context) error</code> | 在传入ctx下推进有限工作量，失败保留原进度等待下一次；nil不等于全任务终态。 |
| <code>Close</code> | <code>()</code> | 关闭该worker持有的网络等资源，循环退出时调用。 |

<!-- write-step: 2 bytes=69:137 -->
````go
type rebuilder interface {
	Step(context.Context) error
	Close()
}

````

## 第 3 步：RunAfterRecovery

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

RunAfterRecovery在启动时先等待原SWITCHING完成，再让普通消费者初始化。
切换失败持续保留屏障，不通过空成功或另建任务绕过；退出时关闭尚未启动的消费者。

本步声明：`func RunAfterRecovery(ctx context.Context, consumer Consumer, ready func(context.Context) error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code>；<code>context.WithTimeout(ctx, 3*time.Second)</code>；<code>Run(ctx, consumer)</code>；<code>ctx.Done</code> |
| <code>consumer</code> | <code>Consumer</code> | 尚未启动的普通Kafka消费者，恢复屏障解除后才运行。<br>在此声明中的实际使用：<code>Run(ctx, consumer)</code>；<code>consumer.Close</code> |
| <code>ready</code> | <code>func(context.Context) error</code> | 检查当前持久SWITCHING是否已恢复的函数。<br>在此声明中的实际使用：<code>ready(check)</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=137:761 -->
````go
// RunAfterRecovery在启动时先等待原SWITCHING完成，再让普通消费者初始化。
// 切换失败持续保留屏障，不通过空成功或另建任务绕过；退出时关闭尚未启动的消费者。
func RunAfterRecovery(ctx context.Context, consumer Consumer, ready func(context.Context) error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		check, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := ready(check)
		cancel()
		if err == nil {
			Run(ctx, consumer)
			return
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	consumer.Close()
}

````

## 第 4 步：rebuildCode

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`var rebuildCode = regexp.MustCompile(\`^SEARCH_[A-Z0-9_]{1,56}$\`)`。

<!-- write-step: 4 bytes=761:827 -->
````go
var rebuildCode = regexp.MustCompile(`^SEARCH_[A-Z0-9_]{1,56}$`)

````

## 第 5 步：RunRebuild

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

本实例只运行一个重建循环；数据库逐文档/记录状态锁同时保护跨实例重复调用。
故障只暂停原游标，不清空任务、不替换快照，也不影响仍服务旧索引的普通消费者。

本步声明：`func RunRebuild(ctx context.Context, service rebuilder)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>service.Step(ctx)</code>；<code>ctx.Err</code> |
| <code>service</code> | <code>rebuilder</code> | 具有有界Step及Close能力的重建worker。<br>在此声明中的实际使用：<code>service.Close</code>；<code>service.Step</code> |

**跟着本段代码走一遍：**

- 登记 <code>service.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>rebuildCode.MatchString(err.Error())</code>。
- 检查 <code>code != last</code>。

<!-- write-step: 5 bytes=827:1608 -->
````go
// 本实例只运行一个重建循环；数据库逐文档/记录状态锁同时保护跨实例重复调用。
// 故障只暂停原游标，不清空任务、不替换快照，也不影响仍服务旧索引的普通消费者。
func RunRebuild(ctx context.Context, service rebuilder) {
	defer service.Close()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := service.Step(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			last = ""
			continue
		}
		code := "SEARCH_REBUILD_RETRY_REQUIRED"
		if rebuildCode.MatchString(err.Error()) {
			code = err.Error()
		}
		if code != last {
			slog.Warn("rebuild_pending", "code", code)
			last = code
		}
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/worker/rebuild.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
