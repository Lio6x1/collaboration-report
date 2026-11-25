# 分步编写 `services/document/internal/exporter/process_other.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/123-process-other.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

明确未验收平台仅具有直接转换进程运行和取消能力。

Windows Job Object和Linux进程组之外的系统没有同等已验证进程树清理语义。

写之前先按这条执行路线理解各段的连接：

1. build tag排除Windows/Linux
2. runProgram调用cmd.Run，依赖直接进程取消。

本文件需要守住的边界：

- 不能将此fallback描述为可靠清理整个孙进程树或正式支持平台。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/process_other.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/process_other.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/process_other.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/process_other.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:67 -->
````go
//go:build !windows && !linux

package exporter

import "os/exec"

````

## 第 2 步：runProgram

插入位置：第 1 步代码之后，紧接追加；完成稿约第 7 行。

未验收的平台只保留直接进程取消；正式部署支持Windows Job Object与Linux容器。

本步声明：`func runProgram(cmd *exec.Cmd) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cmd</code> | <code>*exec.Cmd</code> | 在此声明中的实际使用：<code>cmd.Run</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>cmd.Run()</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return cmd.Run()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=67:230 -->
````go
// 未验收的平台只保留直接进程取消；正式部署支持Windows Job Object与Linux容器。
func runProgram(cmd *exec.Cmd) error { return cmd.Run() }
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/process_other.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
