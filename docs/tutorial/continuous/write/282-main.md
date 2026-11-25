# 分步编写 `services/document/cmd/search-rebuild/main.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/282-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用真实Document身份启动或查询已导入的原Search重建操作。

Start结果未知需要复用原封存请求，不允许人工重新输入代次覆盖绑定。

写之前先按这条执行路线理解各段的连接：

1. run解析CLI，加载Doc配置与原快照请求，建立受信Search连接并调用冻结Start/Get协议，向output打印安全状态。

本文件需要守住的边界：

- 只操作Doc已提交制品
- 此CLI不访问Search私库或ES。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/cmd/search-rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/cmd/search-rebuild/main.go') -and (Get-Item -LiteralPath 'services/document/cmd/search-rebuild/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/cmd/search-rebuild/main.go')) { New-Item -ItemType File -Path 'services/document/cmd/search-rebuild/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:385 -->
````go
// search-rebuild以真实Doc身份接收/查询已导入的原重建操作。
package main

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/logic/snapshot"
	"collaboration/platform"
	"collaboration/platform/rebuildmeta"
	"context"
	"flag"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>code != &quot;&quot;</code>。

<!-- write-step: 2 bytes=385:594 -->
````go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if code := run(ctx, os.Args[1:], os.Stdout); code != "" {
		fmt.Fprintln(os.Stderr, code)
		os.Exit(1)
	}
}

````

## 第 3 步：run

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func run(ctx context.Context, args []string, output io.Writer) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, 3*time.Second)</code> |
| <code>args</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>flags.Parse(args)</code> |
| <code>output</code> | <code>io.Writer</code> | 接收输出字节的流，写入可能失败或短写；不能只看业务计算完成就宣称文件已输出。<br>在此声明中的实际使用：<code>fmt.Fprintln(output, string(raw))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;REBUILD_ARGUMENT_INVALID&quot;</code>；<code>&quot;REBUILD_CONFIG_INVALID&quot;</code>；<code>&quot;REBUILD_DATABASE_UNAVAILABLE&quot;</code>；<code>&quot;REBUILD_ORIGINAL_REQUEST_UNAVAILABLE&quot;</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>flags.Parse(args) != nil &#124;&#124; flags.NArg() != 0 &#124;&#124; *config == &quot;&quot; &#124;&#124; !rebuildmeta.OperationValid(*operation) &#124;&#124; (*action != &quot;start&quot; &amp;&amp; *action != &quot;status&quot;)</code>。条件成立时执行 <code>return &quot;REBUILD_ARGUMENT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; cfg.Role != &quot;document&quot; &#124;&#124; !filepath.IsAbs(cfg.Root)</code>。条件成立时执行 <code>return &quot;REBUILD_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_DATABASE_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ORIGINAL_REQUEST_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_PEER_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 登记 <code>connection.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>*action == &quot;start&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_RPC_UNKNOWN_RETRY_SAME_OPERATION&quot;</code>。当前函数在这里结束。
- 检查 <code>result == nil &#124;&#124; result.OperationId != *operation &#124;&#124; result.Generation != request.Generation</code>。条件成立时执行 <code>return &quot;REBUILD_REPLY_INVALID&quot;</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=594:2520 -->
````go
func run(ctx context.Context, args []string, output io.Writer) string {
	flags := flag.NewFlagSet("search-rebuild", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Doc本服务配置")
	operation := flags.String("operation", "", "原操作号")
	ref := flags.String("snapshot-ref", "", "原封存引用")
	action := flags.String("action", "status", "start或status")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) || (*action != "start" && *action != "status") {
		return "REBUILD_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "document" || !filepath.IsAbs(cfg.Root) {
		return "REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	local, cancel := context.WithTimeout(ctx, 3*time.Second)
	request, fingerprint, err := snapshot.RebuildRequest(local, store.DB, *operation, *ref)
	cancel()
	if err != nil {
		return "REBUILD_ORIGINAL_REQUEST_UNAVAILABLE"
	}
	connection, err := platform.DialPeer(cfg, "search")
	if err != nil {
		return "REBUILD_PEER_UNAVAILABLE"
	}
	defer connection.Close()
	client := pb.NewSearchInternalClient(connection)
	rpcCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var result *pb.OperationResult
	if *action == "start" {
		result, err = client.StartRebuild(rpcCtx, request)
	} else {
		result, err = client.GetRebuild(rpcCtx, &pb.OperationQuery{OperationId: *operation, RequestFingerprint: fingerprint[:]})
	}
	if err != nil {
		// 包括网络中断在内都不推断未接收；操作者用同一参数status查询或start重试。
		return "REBUILD_RPC_UNKNOWN_RETRY_SAME_OPERATION"
	}
	if result == nil || result.OperationId != *operation || result.Generation != request.Generation {
		return "REBUILD_REPLY_INVALID"
	}
````

## 第 4 步：run（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_REPLY_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2520:2761 -->
````go
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(result)
	if err != nil {
		return "REBUILD_REPLY_INVALID"
	}
	if _, err := fmt.Fprintln(output, string(raw)); err != nil {
		return "REBUILD_OUTPUT_FAILED"
	}
	return ""
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/cmd/search-rebuild/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
