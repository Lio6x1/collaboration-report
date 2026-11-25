# 分步编写 `services/document/main.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/284-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

启动Document进程并把生成RPC契约接入统一平台启动器。

平台统一处理配置、私有存储、监听与退出，Document入口只选择自身角色和装配函数。

写之前先按这条执行路线理解各段的连接：

1. 创建server.Document
2. platform.Main指定document角色
3. 回调RegisterDocumentInternalServer
4. bootstrap.RegisterDocument装配业务。

本文件需要守住的边界：

- RPC登记早于Bind，但平台须在装配成功后才Serve
- 不要在main重复构建另一套业务对象。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/main.go') -and (Get-Item -LiteralPath 'services/document/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/main.go')) { New-Item -ItemType File -Path 'services/document/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:206 -->
````go
package main

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/bootstrap"
	"collaboration/document/internal/server"
	"collaboration/platform"
	"google.golang.org/grpc"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`func main()`。

<!-- write-step: 2 bytes=206:438 -->
````go
func main() {
	rpc := server.NewDocument()
	platform.Main("document", func(grpcServer *grpc.Server, _ *platform.Store, _ platform.Config) {
		pb.RegisterDocumentInternalServer(grpcServer, rpc)
	}, bootstrap.RegisterDocument(rpc))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
