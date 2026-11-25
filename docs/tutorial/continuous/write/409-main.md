# 分步编写 `services/search/cmd/search-rebuild-prepare/main.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/409-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供运维准备入口，固定原Kafka起点并导出已提交的签名Prepare制品。

新准备签名与已接受原号恢复分开；后者不能因当前叶证书到期而被迫重新签发或更换操作。

写之前先按这条执行路线理解各段的连接：

1. 解析-config/-operation/-generation，目标代次必须是规范非零十进制。
2. 只打开Search私有Store、ES和Kafka客户端，签名器由LoadSigner在新任务时延迟加载。
3. Prepare确认提交后计算内容地址，将原封装发布到search/outbox，最后输出reference JSON。

本文件需要守住的边界：

- 提交未知只报固定码并原号重试；不能把DB失败原文或私钥路径打进日志。
- 制品输出失败发生在业务提交之后，下一次仍恢复原Prepare，不能创建新任务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/cmd/search-rebuild-prepare' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/cmd/search-rebuild-prepare/main.go') -and (Get-Item -LiteralPath 'services/search/cmd/search-rebuild-prepare/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/cmd/search-rebuild-prepare/main.go')) { New-Item -ItemType File -Path 'services/search/cmd/search-rebuild-prepare/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:461 -->
````go
// search-rebuild-prepare 只读取 Search 自己的配置并导出已提交的原准备制品。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	"collaboration/search/internal/logic/rebuild"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>code != &quot;&quot;</code>。

<!-- write-step: 2 bytes=461:670 -->
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

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func run(ctx context.Context, args []string, output io.Writer) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>preparer.Prepare(ctx, *operation, target)</code>；<code>ctx.Err</code> |
| <code>args</code> | <code>&#91;&#93;string</code> | 命令行参数列表，包含search配置、原号和目标代次。<br>在此声明中的实际使用：<code>flags.Parse(args)</code> |
| <code>output</code> | <code>io.Writer</code> | 成功结果reference JSON的输出流。<br>在此声明中的实际使用：<code>json.NewEncoder(output)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;SEARCH_REBUILD_ARGUMENT_INVALID&quot;</code>；<code>&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;</code>；<code>&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;</code>；<code>&quot;SEARCH_REBUILD_DATABASE_UNAVAILABLE&quot;</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>flags.Parse(args) != nil &#124;&#124; flags.NArg() != 0 &#124;&#124; *config == &quot;&quot; &#124;&#124; !rebuildmeta.OperationValid(*operation)</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_ARGUMENT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_GENERATION_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; cfg.Role != &quot;search&quot; &#124;&#124; cfg.Root == &quot;&quot; &#124;&#124; !filepath.IsAbs(cfg.Root) &#124;&#124; cfg.Search == nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_DATABASE_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_LOG_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 登记 <code>logs.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_ES_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 登记 <code>aliases.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, &quot;search&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=670:2630 -->
````go
func run(ctx context.Context, args []string, output io.Writer) string {
	flags := flag.NewFlagSet("search-rebuild-prepare", flag.ContinueOnError)
	// flag 默认错误会回显用户参数；这里只输出固定错误码，配置内容与路径不进日志。
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Search 本服务配置文件")
	operation := flags.String("operation", "", "原操作号")
	generation := flags.String("generation", "", "规范十进制目标代次")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	target, err := rebuildmeta.Decimal(*generation, true)
	if err != nil {
		return "SEARCH_REBUILD_GENERATION_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "search" || cfg.Root == "" || !filepath.IsAbs(cfg.Root) || cfg.Search == nil {
		return "SEARCH_REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "SEARCH_REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	logs, err := logsource.New(cfg.Search.Brokers, cfg.Search.Topic)
	if err != nil {
		return "SEARCH_REBUILD_LOG_CONFIG_INVALID"
	}
	defer logs.Close()
	aliases, err := elastic.New(cfg.Search)
	if err != nil {
		return "SEARCH_REBUILD_ES_CONFIG_INVALID"
	}
	defer aliases.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	preparer := rebuild.Preparer{DB: store.DB, Logs: logs, Aliases: aliases, Verifier: verifier,
		Owner: cfg.Search.Owner, Alias: cfg.Search.Alias,
		// 已接受原 prepare 恢复不应因当前叶证书过期而失败；只在新任务时读私钥。
		LoadSigner: func() (*rebuildartifact.Signer, error) {
			return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "search")
		},
	}
	envelope, err := preparer.Prepare(ctx, *operation, target)
````

## 第 4 步：run（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 76 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>err.Error() == &quot;SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN&quot;</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN&quot;</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CANCELLED&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return err.Error()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;SEARCH_REBUILD_PREPARE_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_ENVELOPE_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_REFERENCE_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CANCELLED&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; n != len(envelope)</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;SEARCH_REBUILD_OUTPUT_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CANCELLED&quot;</code>。当前函数在这里结束。
- 检查 <code>json.NewEncoder(output).Encode(struct { 		Reference string `json:&quot;reference&quot;` 	}{reference}) != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2630:4031 -->
````go
	if err != nil {
		if err.Error() == "SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN" {
			return "SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN"
		}
		if ctx.Err() != nil {
			return "SEARCH_REBUILD_CANCELLED"
		}
		// 已知提交未知必须明确提示原号重试，其他底层原因不回显 DSN/网络响应。
		switch err.Error() {
		case "SEARCH_REBUILD_OPERATION_CONFLICT", "SEARCH_REBUILD_PREPARE_REJECTED", "SEARCH_REBUILD_GENERATION_INVALID":
			return err.Error()
		default:
			return "SEARCH_REBUILD_PREPARE_FAILED"
		}
	}
	reference, err := rebuildartifact.Reference(envelope, rebuildartifact.PrepareV1)
	if err != nil {
		return "REBUILD_ARTIFACT_ENVELOPE_INVALID"
	}
	name, err := rebuildartifact.FileName(reference)
	if err != nil {
		return "REBUILD_ARTIFACT_REFERENCE_INVALID"
	}
	if ctx.Err() != nil {
		return "SEARCH_REBUILD_CANCELLED"
	}
	outbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "outbox")
	if err := rebuildartifact.PublishFile(outbox, name, func(w io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := w.Write(envelope)
		if err == nil && n != len(envelope) {
			err = io.ErrShortWrite
		}
		return err
	}); err != nil {
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
	if ctx.Err() != nil {
		return "SEARCH_REBUILD_CANCELLED"
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{reference}) != nil {
````

## 第 5 步：run（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 121 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;SEARCH_REBUILD_OUTPUT_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4031:4087 -->
````go
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
	return ""
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/cmd/search-rebuild-prepare/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
