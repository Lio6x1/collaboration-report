# 分步编写 `services/search/internal/worker/consumer.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/400-consumer.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

驱动串行消费循环，失败有界退避且只记录稳定错误码。

一个坏记录不应被跳过或被日志原文泄漏；worker负责再次调用Step，原记录由Consumer持有。

写之前先按这条执行路线理解各段的连接：

1. Run defer Close，循环Step
2. 错误码改变时才Warn，恢复后仅一次Info
3. 固定1秒timer可被ctx取消
4. 非稳定错误替换SEARCH_CONSUMER_UNAVAILABLE

本文件需要守住的边界：

- 不得在worker丢弃current或手动前移offset
- 不能输出Kafka原Value或第三方错误文本

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/worker/consumer.go') -and (Get-Item -LiteralPath 'services/search/internal/worker/consumer.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/worker/consumer.go')) { New-Item -ItemType File -Path 'services/search/internal/worker/consumer.go' | Out-Null }
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

## 第 2 步：Consumer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type Consumer interface {`。

**这一段为什么这样写：** worker只拥有循环与关闭两个操作。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Step</code> | <code>(context.Context) error</code> | ctx→一次恢复/处理原在途记录，失败下轮仍是原记录 |
| <code>Close</code> | <code>() error</code> | 释放组客户端等资源并报告关闭错误 |

<!-- write-step: 2 bytes=69:142 -->
````go
type Consumer interface {
	Step(context.Context) error
	Close() error
}

````

## 第 3 步：safeCode

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var safeCode = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{1,63}$\`)`。

<!-- write-step: 3 bytes=142:203 -->
````go
var safeCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

````

## 第 4 步：Run

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

每次失败继续原在途记录；退避有上限并受生命周期context取消。
只在错误码变化、恢复或退出时写固定日志，不输出消息正文或第三方错误。

本步声明：`func Run(ctx context.Context, consumer Consumer)`。

**这一段为什么这样写：** 在生命周期内重试原工作，不改变业务提交顺序。

1. 调用Step
2. 净化状态日志
3. 失败等待可取消timer
4. 退出关闭资源


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 整个服务停止信号<br>在此声明中的实际使用：<code>ctx.Err</code>；<code>consumer.Step(ctx)</code>；<code>ctx.Done</code> |
| <code>consumer</code> | <code>Consumer</code> | 持有真实在途状态的消费服务<br>在此声明中的实际使用：<code>consumer.Close</code>；<code>consumer.Step</code> |

**跟着本段代码走一遍：**

- 登记 <code>consumer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>previous != &quot;&quot;</code>。
- 检查 <code>!safeCode.MatchString(code)</code>。
- 检查 <code>code != previous</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=203:1030 -->
````go
// 每次失败继续原在途记录；退避有上限并受生命周期context取消。
// 只在错误码变化、恢复或退出时写固定日志，不输出消息正文或第三方错误。
func Run(ctx context.Context, consumer Consumer) {
	defer consumer.Close()
	var previous string
	for ctx.Err() == nil {
		err := consumer.Step(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			if previous != "" {
				slog.Info("search_consumer_recovered")
				previous = ""
			}
			continue
		}
		code := err.Error()
		if !safeCode.MatchString(code) {
			code = "SEARCH_CONSUMER_UNAVAILABLE"
		}
		if code != previous {
			slog.Warn("search_consumer_retry", "code", code)
			previous = code
		}
		timer := time.NewTimer(time.Second)
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
gofmt -w 'services/search/internal/worker/consumer.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
