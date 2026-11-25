# 分步编写 `services/account/main.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/088-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

启动Account进程并把共享用例装到RPC与HTTP入口。

单一platform.Main负责连接池、安全传输和生命周期，Account main只声明本服务组合。

写之前先按这条执行路线理解各段的连接：

1. 第一个回调创建Identity RPC并绑定只读授权结果。
2. 第二个回调调用RegisterAccount，同时通过BindOperationResults共享退出/注销结果用例。

本文件需要守住的边界：

- HTTP与RPC必须在依赖装配完成后才对外Serve；不能用两个互不关联的Logout/AccountClosures状态替代共享持久事实。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/main.go') -and (Get-Item -LiteralPath 'services/account/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/main.go')) { New-Item -ItemType File -Path 'services/account/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:280 -->
````go
package main

import (
	"collaboration/account/internal/bootstrap"
	"collaboration/account/internal/server"
	"collaboration/account/internal/svc"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return bootstrap.RegisterAccount(http, store, cfg, identity.BindOperationResults)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=280:767 -->
````go
func main() {
	var identity *server.Identity
	platform.Main("account", func(rpc *grpc.Server, store *platform.Store, _ platform.Config) {
		identity = server.NewIdentity(svc.NewIdentity(store))
		identity.BindAuthorizationResults(svc.NewAuthorizationResults(store))
		pb.RegisterIdentityInternalServer(rpc, identity)
	}, func(http *rest.Server, store *platform.Store, cfg platform.Config) error {
		return bootstrap.RegisterAccount(http, store, cfg, identity.BindOperationResults)
	})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
