# 分步编写 `services/document/internal/exporter/process_linux.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/122-process-linux.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在Linux为转换器与浏览器创建可整体清理的独立进程组。

仅终止Pandoc或Node父进程会留下孙进程继续写原RUNNING制品。

写之前先按这条执行路线理解各段的连接：

1. runProgram绑定独立进程组
2. 上下文取消调用killProgramGroup终止整组并等待执行结束。

本文件需要守住的边界：

- 进程组退出前不能释放job guard或把任务宣称已排空。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/process_linux.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/process_linux.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/process_linux.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/process_linux.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:68 -->
````go
package exporter

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

````

## 第 2 步：runProgram

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

转换器与浏览器必须在自己的进程组中，context取消时清理整组，不能
只停止Pandoc/Node父进程而让孙进程继续写原RUNNING作业的制品。

本步声明：`func runProgram(cmd *exec.Cmd) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cmd</code> | <code>*exec.Cmd</code> | 在此声明中的实际使用：<code>cmd.SysProcAttr</code>；<code>cmd.Cancel</code>；<code>killProgramGroup(cmd)</code>；<code>cmd.Start</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>cmd.Wait()</code> |

**跟着本段代码走一遍：**

- 检查 <code>cmd.SysProcAttr == nil</code>。
- 检查 <code>cmd.Cancel != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return killProgramGroup(cmd)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>killProgramGroup(cmd)</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return cmd.Wait()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=68:841 -->
````go
// 转换器与浏览器必须在自己的进程组中，context取消时清理整组，不能
// 只停止Pandoc/Node父进程而让孙进程继续写原RUNNING作业的制品。
func runProgram(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
	// Pdeathsig只覆盖直接子进程。Doc强制退出后的完整后代清理依赖正式
	// 部署的独立容器PID空间，不能把此选项当作宿主机全进程树保证。
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { return killProgramGroup(cmd) }
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer killProgramGroup(cmd)
	return cmd.Wait()
}

````

## 第 3 步：killProgramGroup

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func killProgramGroup(cmd *exec.Cmd) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cmd</code> | <code>*exec.Cmd</code> | 在此声明中的实际使用：<code>cmd.Process</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>os.ErrProcessDone</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, syscall.ESRCH)</code>。条件成立时执行 <code>return os.ErrProcessDone</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=841:1022 -->
````go
func killProgramGroup(cmd *exec.Cmd) error {
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/process_linux.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
