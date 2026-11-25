# 分步编写 `services/document/internal/client/process_probe_linux.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/113-process-probe-linux.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在Linux同一PID命名空间中读取旧输出进程的真实身份。

容器隔离会让别的命名空间PID不可见，因此必须在共享应用PID范围内探测。

写之前先按这条执行路线理解各段的连接：

1. probeProcess读取指定PID的内核进程参数并构造processFacts，区分真实缺失与参数不可读。

本文件需要守住的边界：

- 另一个容器看不见PID不能视为ABSENT
- 权限或参数读取失败保持未知。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/process_probe_linux.go') -and (Get-Item -LiteralPath 'services/document/internal/client/process_probe_linux.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/process_probe_linux.go')) { New-Item -ItemType File -Path 'services/document/internal/client/process_probe_linux.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:112 -->
````go
//go:build linux

package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

````

## 第 2 步：ProcessOwner.probeProcess

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

Linux发行版把应用放在同一个PID命名空间。另一个容器里看不到PID不能作为退出证据。

本步声明：`func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Err</code> |
| <code>pid</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(pid, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>processFacts</code> | 实际返回表达式：<code>processFacts{}</code>；<code>processFacts{State: &quot;ABSENT&quot;}</code>；<code>processFacts{State: &quot;PRESENT&quot;, Args: strings.Split(string(raw&#91;:len(raw)-1&#93;), &quot;\x00&quot;)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;PROCESS_ARGUMENTS_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return processFacts{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, os.ErrNotExist)</code>。
- 检查 <code>errors.Is(statErr, os.ErrNotExist)</code>。条件成立时执行 <code>return processFacts{State: &quot;ABSENT&quot;}, nil</code>。当前函数在这里结束。
- 此处通过 <code>return processFacts{}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(raw) == 0 &#124;&#124; raw&#91;len(raw)-1&#93; != 0</code>。条件成立时执行 <code>return processFacts{}, errors.New(&quot;PROCESS_ARGUMENTS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return processFacts{State: &quot;PRESENT&quot;, Args: strings.Split(string(raw&#91;:len(raw)-1&#93;), &quot;\x00&quot;)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=112:1206 -->
````go
// Linux发行版把应用放在同一个PID命名空间。另一个容器里看不到PID不能作为退出证据。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	if err := ctx.Err(); err != nil {
		return processFacts{}, err
	}
	// 先证明procfs可读；未挂载或无权限均是未知，绝不返回ABSENT。
	if _, err := os.ReadFile("/proc/self/cmdline"); err != nil {
		return processFacts{}, err
	}
	dir := filepath.Join("/proc", strconv.FormatUint(pid, 10))
	raw, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
				return processFacts{State: "ABSENT"}, nil
			}
		}
		return processFacts{}, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != 0 {
		// 僵尸/退出中的空命令行无法证明身份，等待真正回收后再重试。
		return processFacts{}, errors.New("PROCESS_ARGUMENTS_UNAVAILABLE")
	}
	return processFacts{State: "PRESENT", Args: strings.Split(string(raw[:len(raw)-1]), "\x00")}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/process_probe_linux.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
