# 分步编写 `packages/platform/store_diagnostics.go`

先按 [F03 · 配置、连接池、就绪与安全出口](../lessons/foundation/03-runtime-and-security.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/032-store-diagnostics.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按显式开关输出连接池聚合诊断，帮助定位Windows连接耗尽。

观测复用Store.DB.Stats，不创建另一池或暴露SQL/DSN/用户载荷。

写之前先按这条执行路线理解各段的连接：

1. 仅COLLAB_SQL_POOL_DIAGNOSTICS=1启动goroutine；每10秒记录open/in_use/idle/wait及关闭计数。
2. context取消后停止ticker并退出。

本文件需要守住的边界：

- 诊断不能改连接上限、期限或记录私密连接串，未开开关不启动采样。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/store_diagnostics.go') -and (Get-Item -LiteralPath 'packages/platform/store_diagnostics.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/store_diagnostics.go')) { New-Item -ItemType File -Path 'packages/platform/store_diagnostics.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:67 -->
````go
package platform

import (
	"context"
	"log/slog"
	"os"
	"time"
)

````

## 第 2 步：Store.runPoolDiagnostics

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

只在明确排查连接耗尽时输出连接池聚合量，不记录DSN、SQL、用户或业务载荷。
数据库仍由Store统一持有；观测不能建立另一个连接池或影响事务期限。

本步声明：`func (s *Store) runPoolDiagnostics(ctx context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code> |

**跟着本段代码走一遍：**

- 检查 <code>os.Getenv(&quot;COLLAB_SQL_POOL_DIAGNOSTICS&quot;) != &quot;1&quot;</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 用独立 goroutine 执行 <code>func() { 		ticker := time.NewTicker(10 * time.Second) 		defer ticker.Stop() 		for { 			select { 			case &lt;-ctx.Done(): 				return 			case &lt;-ticker.C: 				stats := s.DB.Stats() 				s …</code>（长表达式，按本步完整代码继续）；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 2 bytes=67:881 -->
````go
// 只在明确排查连接耗尽时输出连接池聚合量，不记录DSN、SQL、用户或业务载荷。
// 数据库仍由Store统一持有；观测不能建立另一个连接池或影响事务期限。
func (s *Store) runPoolDiagnostics(ctx context.Context) {
	if os.Getenv("COLLAB_SQL_POOL_DIAGNOSTICS") != "1" {
		return
	}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := s.DB.Stats()
				slog.Info("sql_pool", "role", s.Role,
					"open", stats.OpenConnections, "in_use", stats.InUse, "idle", stats.Idle,
					"wait_count", stats.WaitCount, "idle_closed", stats.MaxIdleClosed,
					"idle_time_closed", stats.MaxIdleTimeClosed, "lifetime_closed", stats.MaxLifetimeClosed)
			}
		}
	}()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/store_diagnostics.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/03-runtime-and-security.md)。
