# 分步编写 `services/document/internal/storage/local/guard_unix.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/127-guard-unix.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在Unix实现本地对象锁的系统级加锁与解锁。

多个Document进程必须共享同一文件锁语义，进程内mutex无法阻止另一实例清理。

写之前先按这条执行路线理解各段的连接：

1. lockFile按exclusive选择排他或共享的非阻塞文件锁
2. 冲突映射为忙
3. unlockFile释放同一文件句柄锁。

本文件需要守住的边界：

- build tag限制支持平台
- 不能在持锁期间更换锁文件身份。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/storage/local' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/storage/local/guard_unix.go') -and (Get-Item -LiteralPath 'services/document/internal/storage/local/guard_unix.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/storage/local/guard_unix.go')) { New-Item -ItemType File -Path 'services/document/internal/storage/local/guard_unix.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:142 -->
````go
//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package local

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

````

## 第 2 步：lockFile

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func lockFile(file *os.File, exclusive bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>file</code> | <code>*os.File</code> | 在此声明中的实际使用：<code>file.Fd</code> |
| <code>exclusive</code> | <code>bool</code> | true申请排他文件锁供写入/清理，false申请共享读锁；本机不同进程须锁同一个永久锁文件。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrBusy</code>；<code>ErrUnavailable</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>exclusive</code>。
- 检查 <code>errors.Is(err, unix.EWOULDBLOCK) &#124;&#124; errors.Is(err, unix.EAGAIN)</code>。条件成立时执行 <code>return ErrBusy</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return ErrUnavailable</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=142:461 -->
````go
func lockFile(file *os.File, exclusive bool) error {
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	err := unix.Flock(int(file.Fd()), mode|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrBusy
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

````

## 第 3 步：unlockFile

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func unlockFile(file *os.File)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>file</code> | <code>*os.File</code> | 在此声明中的实际使用：<code>file.Fd</code> |

<!-- write-step: 3 bytes=461:542 -->
````go
func unlockFile(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/storage/local/guard_unix.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
