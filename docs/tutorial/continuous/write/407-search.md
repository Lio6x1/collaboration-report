# 分步编写 `services/search/internal/bootstrap/search.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/407-search.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在启动边界注册Search RPC并启动消费、游标和重建后台任务。

依赖装配失败必须影响就绪状态；不能仍注册一个表面可用但缺少业务能力的成功服务。

写之前先按这条执行路线理解各段的连接：

1. 依次创建consumer、candidates、rebuild服务；失败时ReadyExtra保留相应错误并关闭已成功创建的资源。
2. 完整重建worker装配成功后，ReadyExtra同时检查消费和候选服务。
3. 后台启动RunAfterRecovery、RunCursors、RunRebuild，全部沿Store.Context退出；最后注册完整RPC。

本文件需要守住的边界：

- 重建worker装配失败时只注册无能力适配器，不能启动部分后台流程。
- 启动goroutine不等于已就绪，真实依赖检查仍必须通过。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/bootstrap/search.go') -and (Get-Item -LiteralPath 'services/search/internal/bootstrap/search.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/bootstrap/search.go')) { New-Item -ItemType File -Path 'services/search/internal/bootstrap/search.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:252 -->
````go
package bootstrap

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/search/internal/server"
	"collaboration/search/internal/svc"
	"collaboration/search/internal/worker"
	"context"
	"google.golang.org/grpc"
)

````

## 第 2 步：RegisterSearch

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

RegisterSearch仅组合当前RPC适配，底座就绪继续依赖本服务私有Store的实际检查。

本步声明：`func RegisterSearch(rpc *grpc.Server, store *platform.Store, configuration platform.Config)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rpc</code> | <code>*grpc.Server</code> | 当前内部gRPC服务器，注册冻结Search接口。<br>在此声明中的实际使用：<code>pb.RegisterSearchInternalServer(rpc, server.NewSearch())</code>；<code>pb.RegisterSearchInternalServer(rpc, server.NewSearchWithRebuild(candidates, rebuild))</code> |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有Store，包含DB、生命周期Context和ReadyExtra钩子。<br>在此声明中的实际使用：<code>svc.NewConsumer(store, configuration)</code>；<code>svc.NewCandidates(store, configuration)</code>；<code>svc.NewRebuild(store, configuration)</code>；<code>store.ReadyExtra</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>configuration</code> | <code>platform.Config</code> | 经平台加载的search角色配置，不从其他服务配置借权限。<br>在此声明中的实际使用：<code>svc.NewConsumer(store, configuration)</code>；<code>svc.NewCandidates(store, configuration)</code>；<code>svc.NewRebuild(store, configuration)</code>；<code>svc.NewRebuildWorker(store, configuration)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>candidates != nil</code>。
- 检查 <code>candidateErr != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return candidateErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>rebuildErr != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return rebuildErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>seedErr != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return seedErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return candidates.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 用独立 goroutine 执行 <code>worker.RunAfterRecovery(store.Context, consumer, rebuildWorker.RecoveryReady)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 用独立 goroutine 执行 <code>worker.RunCursors(store.Context, candidates)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 用独立 goroutine 执行 <code>worker.RunRebuild(store.Context, rebuildWorker)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 检查 <code>err != nil &#124;&#124; candidateErr != nil &#124;&#124; rebuildErr != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=252:1895 -->
````go
// RegisterSearch仅组合当前RPC适配，底座就绪继续依赖本服务私有Store的实际检查。
func RegisterSearch(rpc *grpc.Server, store *platform.Store, configuration platform.Config) {
	consumer, err := svc.NewConsumer(store, configuration)
	candidates, candidateErr := svc.NewCandidates(store, configuration)
	rebuild, rebuildErr := svc.NewRebuild(store, configuration)
	if err != nil {
		store.ReadyExtra = func(context.Context) error { return err }
		if candidates != nil {
			candidates.Close()
		}
	} else if candidateErr != nil {
		store.ReadyExtra = func(context.Context) error { return candidateErr }
		consumer.Close()
	} else if rebuildErr != nil {
		store.ReadyExtra = func(context.Context) error { return rebuildErr }
		consumer.Close()
		candidates.Close()
	} else {
		rebuildWorker, seedErr := svc.NewRebuildWorker(store, configuration)
		if seedErr != nil {
			store.ReadyExtra = func(context.Context) error { return seedErr }
			consumer.Close()
			candidates.Close()
			pb.RegisterSearchInternalServer(rpc, server.NewSearch())
			return
		}
		store.ReadyExtra = func(ctx context.Context) error {
			if err := consumer.Ready(ctx); err != nil {
				return err
			}
			return candidates.Ready(ctx)
		}
		go worker.RunAfterRecovery(store.Context, consumer, rebuildWorker.RecoveryReady)
		go worker.RunCursors(store.Context, candidates)
		go worker.RunRebuild(store.Context, rebuildWorker)
	}
	if err != nil || candidateErr != nil || rebuildErr != nil {
		pb.RegisterSearchInternalServer(rpc, server.NewSearch())
		return
	}
	pb.RegisterSearchInternalServer(rpc, server.NewSearchWithRebuild(candidates, rebuild))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/bootstrap/search.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
