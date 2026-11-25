# 分步编写 `services/document/internal/worker/lifecycle_workers.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/273-lifecycle-workers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

有界调度普通永久删除与孤立留存治理恢复。

两类任务资格不同，必须分开领取以免共享筛选器吞掉另一路恢复。

写之前先按这条执行路线理解各段的连接：

1. RunPurges先有界登记到期项再恢复
2. RunRetainedGovernance只推进原治理任务
3. context停止循环。

本文件需要守住的边界：

- 只打印固定失败阶段
- 不能为了清理方便打印数据库原文或正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/worker/lifecycle_workers.go') -and (Get-Item -LiteralPath 'services/document/internal/worker/lifecycle_workers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/worker/lifecycle_workers.go')) { New-Item -ItemType File -Path 'services/document/internal/worker/lifecycle_workers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:323 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"errors"
	"github.com/zeromicro/go-zero/core/logx"
	"log/slog"
	"time"
)

// purges.go：保留原实现，按业务合并文件。

````

## 第 2 步：RunPurges

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

一轮一原号，重启后领取原持久任务；调度独立于svc的依赖装配。
登记与恢复分别有界，失败只输出固定阶段，不打印数据库原文或正文。

本步声明：`func RunPurges(ctx context.Context, purges *logic.Purges)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>ctx.Err</code> |
| <code>purges</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。<br>在此声明中的实际使用：<code>purges.RegisterExpired</code>；<code>purges.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; ctx.Err() == nil</code>。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, logic.ErrPurgeIdle) &amp;&amp; ctx.Err() == nil</code>。

<!-- write-step: 2 bytes=323:1202 -->
````go
// 一轮一原号，重启后领取原持久任务；调度独立于svc的依赖装配。
// 登记与恢复分别有界，失败只输出固定阶段，不打印数据库原文或正文。
func RunPurges(ctx context.Context, purges *logic.Purges) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
		_, err := purges.RegisterExpired(attempt)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("PURGE_EXPIRY_PENDING")
		}
		attempt, cancel = context.WithTimeout(ctx, 4*time.Second)
		err = purges.Recover(attempt)
		cancel()
		if err != nil && !errors.Is(err, logic.ErrPurgeIdle) && ctx.Err() == nil {
			slog.Warn("PURGE_RECOVERY_PENDING")
		}
	}
}

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 3 步：RunRetainedGovernance

插入位置：第 2 步代码之后，紧接追加；完成稿约第 43 行。

与普通PURGE分开领取，避免共享任务筛选吞掉另一业务恢复。bootstrap显式
启动这个有限worker；用户不需再次DELETE才能完成已登记的原号。

本步声明：`func RunRetainedGovernance(ctx context.Context, usecase *logic.RetainedGovernance)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>ctx.Err</code> |
| <code>usecase</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。<br>在此声明中的实际使用：<code>usecase.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, logic.ErrRetainedGovernanceIdle) &amp;&amp; ctx.Err() == nil</code>。

<!-- write-step: 3 bytes=1202:1871 -->
````go
// 与普通PURGE分开领取，避免共享任务筛选吞掉另一业务恢复。bootstrap显式
// 启动这个有限worker；用户不需再次DELETE才能完成已登记的原号。
func RunRetainedGovernance(ctx context.Context, usecase *logic.RetainedGovernance) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := usecase.Recover(attempt)
			cancel()
			if err != nil && !errors.Is(err, logic.ErrRetainedGovernanceIdle) && ctx.Err() == nil {
				logx.Error("RETAINED_GOVERNANCE_RECOVERY_PENDING")
			}
		}
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/worker/lifecycle_workers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
