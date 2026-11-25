# 分步编写 `services/search/internal/worker/cursors.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/401-cursors.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

串行驱动持久游标恢复关闭和已过期登记清理。

ticker并发启动多轮会重复领取并放大ES压力，有限轮次和单项预算能避免坏首行堵全部恢复。

写之前先按这条执行路线理解各段的连接：

1. RunCursors每轮10秒调用Cleanup，内部最多16项
2. 按稳定错误码变化记录，恢复后记录一次
3. 轮次结束再等500毫秒，ctx取消即退出并Close服务

本文件需要守住的边界：

- 不能因为一轮超时就删SQL未知链
- 不会重开PIT或延长绝对Expires

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/worker/cursors.go') -and (Get-Item -LiteralPath 'services/search/internal/worker/cursors.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/worker/cursors.go')) { New-Item -ItemType File -Path 'services/search/internal/worker/cursors.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:59 -->
````go
package worker

import (
	"context"
	"log/slog"
	"time"
)

````

## 第 2 步：Cursors

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`type Cursors interface {`。

**这一段为什么这样写：** 关闭worker的最小业务能力。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Cleanup</code> | <code>(context.Context) error</code> | ctx→有界恢复SQL/PIT链并报告首个失败 |
| <code>Close</code> | <code>()</code> | 结束客户端生命周期，不改写业务成功事实 |

<!-- write-step: 2 bytes=59:128 -->
````go
type Cursors interface {
	Cleanup(context.Context) error
	Close()
}

````

## 第 3 步：RunCursors

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func RunCursors(ctx context.Context, cursors Cursors)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code>；<code>context.WithTimeout(ctx, 10*time.Second)</code>；<code>ctx.Done</code> |
| <code>cursors</code> | <code>Cursors</code> | 使用 <code>Cursors</code> 类型；其定义处的职责：关闭worker的最小业务能力。<br>在此声明中的实际使用：<code>cursors.Close</code>；<code>cursors.Cleanup</code> |

**跟着本段代码走一遍：**

- 登记 <code>cursors.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 10*time.Second)</code>。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>!safeCode.MatchString(code)</code>。
- 检查 <code>code != previous</code>。
- 检查 <code>previous != &quot;&quot;</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=128:965 -->
````go
func RunCursors(ctx context.Context, cursors Cursors) {
	defer cursors.Close()
	var previous string
	for ctx.Err() == nil {
		// 轮次串行且只领16条；前轮尚未结束不会被ticker再次启动，未知结果有原SQL重试事实。
		step, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := cursors.Cleanup(step)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			code := err.Error()
			if !safeCode.MatchString(code) {
				code = "SEARCH_CURSOR_RECOVERY_UNAVAILABLE"
			}
			if code != previous {
				slog.Warn("search_cursor_retry", "code", code)
				previous = code
			}
		} else if previous != "" {
			slog.Info("search_cursor_recovered")
			previous = ""
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/worker/cursors.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
