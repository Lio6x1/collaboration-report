# 分步编写 `services/document/internal/client/process_probe_windows.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/115-process-probe-windows.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

使用只读PowerShell查询Windows进程启动与命令身份。

Windows退出证明需要原PID和启动/入口标记，不能以端口关闭或心跳超时代替。

写之前先按这条执行路线理解各段的连接：

1. probeProcess执行受上下文期限控制的查询，解析processFacts并拒绝损坏响应。

本文件需要守住的边界：

- 查询返回不完整数据必须PROCESS_FACTS_INVALID
- 后台查询窗口由hideProbeWindow隐藏。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/process_probe_windows.go') -and (Get-Item -LiteralPath 'services/document/internal/client/process_probe_windows.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/process_probe_windows.go')) { New-Item -ItemType File -Path 'services/document/internal/client/process_probe_windows.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:125 -->
````go
//go:build windows

package client

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
)

````

## 第 2 步：ProcessOwner.probeProcess

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

Windows继续通过既有只读PowerShell查询，保持进程启动标记和超时语义。

本步声明：`func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>exec.CommandContext(ctx, &quot;powershell&quot;, &quot;-NoProfile&quot;, &quot;-NonInteractive&quot;, &quot;-File&quot;, filepath.Join(c.root, &quot;scripts/process-owner.ps1&quot;), &quot;-ProcessId&quot;, strconv.FormatUint(pid, 10))</code> |
| <code>pid</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(pid, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>processFacts</code> | 实际返回表达式：<code>processFacts{}</code>；<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PROCESS_FACTS_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return processFacts{}, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;facts) != nil</code>。条件成立时执行 <code>return processFacts{}, errors.New(&quot;PROCESS_FACTS_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=125:751 -->
````go
// Windows继续通过既有只读PowerShell查询，保持进程启动标记和超时语义。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	probe := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-File", filepath.Join(c.root, "scripts/process-owner.ps1"), "-ProcessId", strconv.FormatUint(pid, 10))
	hideProbeWindow(probe)
	raw, err := probe.Output()
	if err != nil {
		return processFacts{}, err
	}
	var facts processFacts
	if json.Unmarshal(raw, &facts) != nil {
		return processFacts{}, errors.New("PROCESS_FACTS_INVALID")
	}
	return facts, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/process_probe_windows.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
