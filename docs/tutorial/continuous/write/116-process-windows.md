# 分步编写 `services/document/internal/client/process_windows.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/116-process-windows.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

隐藏Windows后台进程归属探测的PowerShell窗口。

恢复周期反复查询进程不应打断用户桌面操作。

写之前先按这条执行路线理解各段的连接：

1. hideProbeWindow在exec.Cmd.SysProcAttr上设置HideWindow=true。

本文件需要守住的边界：

- 只隐藏窗口不改变探测期限与退出证明规则。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/process_windows.go') -and (Get-Item -LiteralPath 'services/document/internal/client/process_windows.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/process_windows.go')) { New-Item -ItemType File -Path 'services/document/internal/client/process_windows.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:70 -->
````go
//go:build windows

package client

import (
	"os/exec"
	"syscall"
)

````

## 第 2 步：hideProbeWindow

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

PID归属检查是后台恢复的一部分，不应在Windows桌面反复弹出PowerShell窗口。

本步声明：`func hideProbeWindow(command *exec.Cmd)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>command</code> | <code>*exec.Cmd</code> | 在此声明中的实际使用：<code>command.SysProcAttr</code> |

<!-- write-step: 2 bytes=70:277 -->
````go
// PID归属检查是后台恢复的一部分，不应在Windows桌面反复弹出PowerShell窗口。
func hideProbeWindow(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/process_windows.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
