# 分步编写 `services/account/internal/worker/closures_workers.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/081-closures-workers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

启动有界循环，分别驱动会话关闭、账号注销各阶段和组织关闭恢复。

独立领取者使慢取消不阻塞其他账号的排空；所有循环跟随Store退出context停止。

写之前先按这条执行路线理解各段的连接：

1. RunAccountCloseRecovery为实例生成唯一标识，按ticker调用RecoverOne。
2. RunSessionClosures轮流登记自然到期和恢复关闭；组织关闭使用独立任务种类。

本文件需要守住的边界：

- 不能在worker中无限处理整个账号；不能共享错误任务种类导致一个恢复器领取另一流程的任务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/worker/closures_workers.go') -and (Get-Item -LiteralPath 'services/account/internal/worker/closures_workers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/worker/closures_workers.go')) { New-Item -ItemType File -Path 'services/account/internal/worker/closures_workers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:244 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	"collaboration/platform"
	"context"
	"sync"
	"time"
)

// account_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：accountCloseRecovery

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type accountCloseRecovery interface{ RecoverOne(context.Context, string) }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>RecoverOne</code> | <code>(context.Context, string)</code> | 每次接收ctx和当前worker实例号，领取并推进一项原任务；内部记录重试结果，不一次跑完无界队列。 |

<!-- write-step: 2 bytes=244:320 -->
````go
type accountCloseRecovery interface{ RecoverOne(context.Context, string) }

````

## 第 3 步：RunAccountCloseRecovery

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

每种流程一个独立有限领取者。慢取消不阻塞其他账号的排空，两者共用服务退出上下文。

本步声明：`func RunAccountCloseRecovery(ctx context.Context, service accountCloseRecovery)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>service.RecoverOne(ctx, instance)</code> |
| <code>service</code> | <code>accountCloseRecovery</code> | 在此声明中的实际使用：<code>service.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=320:818 -->
````go
// 每种流程一个独立有限领取者。慢取消不阻塞其他账号的排空，两者共用服务退出上下文。
func RunAccountCloseRecovery(ctx context.Context, service accountCloseRecovery) {
	instance := platform.NewTraceID()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.RecoverOne(ctx, instance)
		}
	}
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 4 步：RunOrganizationCloseRecovery

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

独立恢复器使用原组织任务领取，不与账号注销worker共享任务种类。

本步声明：`func RunOrganizationCloseRecovery(ctx context.Context, service accountCloseRecovery)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>RunAccountCloseRecovery(ctx, service)</code> |
| <code>service</code> | <code>accountCloseRecovery</code> | 在此声明中的实际使用：<code>RunAccountCloseRecovery(ctx, service)</code> |

<!-- write-step: 4 bytes=818:1109 -->
````go
// 独立恢复器使用原组织任务领取，不与账号注销worker共享任务种类。
func RunOrganizationCloseRecovery(ctx context.Context, service accountCloseRecovery) {
	RunAccountCloseRecovery(ctx, service)
}

// session_closure.go：保留原实现，按业务合并文件。

````

## 第 5 步：sessionClosures

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`type sessionClosures interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>EnqueueExpired</code> | <code>(context.Context)</code> | 以ctx扫描到期会话并持久登记关闭任务，不等下游排空。 |
| <code>RecoverOne</code> | <code>(context.Context, string)</code> | 以ctx和worker实例号有限领取一项会话关闭任务，依租约/代次及真实ACK恢复。 |

<!-- write-step: 5 bytes=1109:1215 -->
````go
type sessionClosures interface {
	EnqueueExpired(context.Context)
	RecoverOne(context.Context, string)
}

````

## 第 6 步：RunSessionClosures

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func RunSessionClosures(ctx context.Context, service sessionClosures)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>service.RecoverOne(ctx, instance)</code>；<code>service.EnqueueExpired(ctx)</code> |
| <code>service</code> | <code>sessionClosures</code> | 在此声明中的实际使用：<code>service.RecoverOne</code>；<code>service.EnqueueExpired</code> |

**跟着本段代码走一遍：**

- 用独立 goroutine 执行 <code>func() { 			defer workers.Done() 			instance := platform.NewTraceID() 			ticker := time.NewTicker(100 * time.Millisecond) 			defer ticker.Stop() 			for { 				select { 				case &lt;-ct …</code>（长表达式，按本步完整代码继续）；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：登记 <code>workers.Done()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>workers.Wait()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1215:2141 -->
````go
func RunSessionClosures(ctx context.Context, service sessionClosures) {
	// 到期登记不等待下游关闭：浏览器离线产生的一批原号应先持久入队。
	// 固定4个领取者使用既有SKIP LOCKED、有限租约及真实ACK逻辑，限制SQL/RPC压力；
	// 任一不可达任务只占用一个worker，不能阻塞其他独立范围的恢复。
	var workers sync.WaitGroup
	for n := 0; n < 4; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			instance := platform.NewTraceID()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					service.RecoverOne(ctx, instance)
				}
			}
		}()
	}
	defer workers.Wait()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.EnqueueExpired(ctx)
		}
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/worker/closures_workers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
