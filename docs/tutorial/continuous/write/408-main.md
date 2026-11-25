# 分步编写 `services/search/cmd/search-rebuild-import/main.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/408-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提供运维导入入口，只接受Search配置和受控inbox里的原Doc快照引用。

命令行参数和路径可能包含敏感配置，因此flag错误不回显参数，stdout只交付原reference。

写之前先按这条执行路线理解各段的连接：

1. main创建Ctrl+C取消上下文，run解析-config/-operation/-snapshot-ref并拒绝多余参数。
2. 核search角色及绝对Root，创建私有Store和CA验证器。
3. 装配Importer导入原操作；仅允许固定错误码输出，成功打印reference JSON。

本文件需要守住的边界：

- 引用必须是合法.snapshot.json内容地址，不能把任意文件路径当导入目标。
- IMPORT_COMMIT_UNKNOWN要求沿原号恢复；输出失败不能证明前面SQL没提交。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/cmd/search-rebuild-import' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/cmd/search-rebuild-import/main.go') -and (Get-Item -LiteralPath 'services/search/cmd/search-rebuild-import/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/cmd/search-rebuild-import/main.go')) { New-Item -ItemType File -Path 'services/search/cmd/search-rebuild-import/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:362 -->
````go
// search-rebuild-import只读取Search自己的配置和受控inbox中的Doc制品。
package main

import (
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/logic/rebuild"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>code != &quot;&quot;</code>。

<!-- write-step: 2 bytes=362:571 -->
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
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>service.Import(ctx, *operation, *reference)</code>；<code>ctx.Err</code> |
| <code>args</code> | <code>&#91;&#93;string</code> | 不含程序名的命令行参数列表。<br>在此声明中的实际使用：<code>flags.Parse(args)</code> |
| <code>output</code> | <code>io.Writer</code> | 接收成功reference JSON的Writer，测试可注入缓冲区。<br>在此声明中的实际使用：<code>json.NewEncoder(output)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;SEARCH_REBUILD_ARGUMENT_INVALID&quot;</code>；<code>&quot;SEARCH_REBUILD_CONFIG_INVALID&quot;</code>；<code>&quot;SEARCH_REBUILD_DATABASE_UNAVAILABLE&quot;</code>；<code>&quot;REBUILD_ARTIFACT_CONFIG_INVALID&quot;</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>flags.Parse(args) != nil &#124;&#124; flags.NArg() != 0 &#124;&#124; *config == &quot;&quot; &#124;&#124; !rebuildmeta.OperationValid(*operation)</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_ARGUMENT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !strings.HasSuffix(name, &quot;.snapshot.json&quot;)</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_ARGUMENT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; cfg.Role != &quot;search&quot; &#124;&#124; cfg.Search == nil &#124;&#124; cfg.Root == &quot;&quot; &#124;&#124; !filepath.IsAbs(cfg.Root)</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_DATABASE_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_CANCELLED&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return err.Error()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;SEARCH_REBUILD_IMPORT_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.NewEncoder(output).Encode(struct { 		Reference string `json:&quot;reference&quot;` 	}{*reference}) != nil</code>。条件成立时执行 <code>return &quot;SEARCH_REBUILD_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=571:2620 -->
````go
func run(ctx context.Context, args []string, output io.Writer) string {
	flags := flag.NewFlagSet("search-rebuild-import", flag.ContinueOnError)
	// 不回显路径、配置或用户参数，错误出口只允许固定码。
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Search本服务配置")
	operation := flags.String("operation", "", "原操作号")
	reference := flags.String("snapshot-ref", "", "原Doc快照引用")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	name, err := rebuildartifact.FileName(*reference)
	if err != nil || !strings.HasSuffix(name, ".snapshot.json") {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "search" || cfg.Search == nil || cfg.Root == "" || !filepath.IsAbs(cfg.Root) {
		return "SEARCH_REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "SEARCH_REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	root := filepath.Join(cfg.Root, ".runtime", "rebuild", "search")
	service := rebuild.Importer{DB: store.DB, Verifier: verifier, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, Inbox: filepath.Join(root, "inbox"), Accepted: filepath.Join(root, "accepted")}
	_, err = service.Import(ctx, *operation, *reference)
	if err != nil {
		if ctx.Err() != nil {
			return "SEARCH_REBUILD_CANCELLED"
		}
		switch err.Error() {
		case "SEARCH_REBUILD_OPERATION_CONFLICT", "SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN", "SEARCH_REBUILD_PREPARE_MISSING", "SEARCH_REBUILD_GENERATION_INVALID", "SEARCH_REBUILD_ENTRIES_MISMATCH":
			return err.Error()
		default:
			return "SEARCH_REBUILD_IMPORT_FAILED"
		}
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{*reference}) != nil {
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
````

## 第 4 步：run（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2620:2633 -->
````go
	return ""
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/cmd/search-rebuild-import/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
