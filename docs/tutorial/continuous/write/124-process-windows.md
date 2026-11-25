# 分步编写 `services/document/internal/exporter/process_windows.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/124-process-windows.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把真实转换器及浏览器进程树绑定Windows Job Object。

Document退出时OS关闭Job句柄可一起终止子孙，避免遗留进程继续改作业文件。

写之前先按这条执行路线理解各段的连接：

1. runProgram创建并绑定Job Object，运行受控命令并等待，取消或退出时收束整个转换进程树。

本文件需要守住的边界：

- 只有父进程退出不足以证明作业安全排空
- Job所有权必须保持到执行结束。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/process_windows.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/process_windows.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/process_windows.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/process_windows.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:91 -->
````go
package exporter

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

````

## 第 2 步：runProgram

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

真实转换进程树绑定本Go进程持有的Job Object；Doc退出时OS关闭句柄，
Pandoc/Node/浏览器子进程一起终止，原RUNNING不能由遗留转换继续写文件。

本步声明：`func runProgram(cmd *exec.Cmd) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cmd</code> | <code>*exec.Cmd</code> | 在此声明中的实际使用：<code>cmd.SysProcAttr</code>；<code>cmd.Cancel</code>；<code>cmd.Process</code>；<code>cmd.Start</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>cmd.Wait()</code> |

**跟着本段代码走一遍：**

- 检查 <code>cmd.SysProcAttr == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>windows.CloseHandle(job)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>cmd.Cancel != nil</code>。
- 在本段的匿名函数内：检查 <code>inJob</code>。条件成立时执行 <code>return windows.TerminateJobObject(job, 1)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return cmd.Process.Kill()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。

<!-- write-step: 2 bytes=91:2096 -->
````go
// 真实转换进程树绑定本Go进程持有的Job Object；Doc退出时OS关闭句柄，
// Pandoc/Node/浏览器子进程一起终止，原RUNNING不能由遗留转换继续写文件。
func runProgram(cmd *exec.Cmd) error {
	// 后台Doc没有控制台，Windows默认会为Pandoc/Node等控制台程序新建窗口。
	// 不创建控制台窗口并隐藏启动窗口，标准输出/错误仍通过原管道收集；
	// 在既有属性上追加，保留调用方设置和下方Job Object的进程树回收约束。
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	assigned := make(chan struct{})
	inJob := false
	if cmd.Cancel != nil {
		// CommandContext默认只杀直接父进程；后代若继承输出管道，Wait会等EOF，
		// 而defer关闭Job又必须等Wait，形成死等。取消时直接终止整棵Job树。
		cmd.Cancel = func() error {
			// Start成功后watchCtx即可调用Cancel，必须等Assign结果，不能先终止
			// 空Job再把仍存活的转换器加入。close也发布inJob，避免并发读写。
			<-assigned
			if inJob {
				return windows.TerminateJobObject(job, 1)
			}
			return cmd.Process.Kill()
		}
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
````

## 第 3 步：runProgram（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 56 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return cmd.Wait()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2096:2333 -->
````go
	inJob = err == nil
	close(assigned)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	// Wait也等待watchCtx/Cancel结束；之后defer才关闭句柄，取消不能访问已关闭Job。
	return cmd.Wait()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/process_windows.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
