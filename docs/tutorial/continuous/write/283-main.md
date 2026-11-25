# 分步编写 `services/document/cmd/search-snapshot/main.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/283-main.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

用Document身份与私库生成或恢复一致快照制品。

Search prepare经受控inbox/outbox交接，恢复要重生原SEALED字节而非重新扫描最新数据。

写之前先按这条执行路线理解各段的连接：

1. run校验参数及原制品，先恢复已封存任务
2. 需要新封存时加载本服务签名身份
3. 写临时文件并完成同步后发布。

本文件需要守住的边界：

- 不可把部分写入文件当完整快照
- 不得使用别的服务证书或直接读Search库。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/cmd/search-snapshot' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/cmd/search-snapshot/main.go') -and (Get-Item -LiteralPath 'services/document/cmd/search-snapshot/main.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/cmd/search-snapshot/main.go')) { New-Item -ItemType File -Path 'services/document/cmd/search-snapshot/main.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:404 -->
````go
// search-snapshot 只使用 Document 本服务身份与私有库；制品交接经受控 inbox/outbox。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"collaboration/document/internal/logic/snapshot"
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
)

````

## 第 2 步：main

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func main()`。

**跟着本段代码走一遍：**

- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>code != &quot;&quot;</code>。

<!-- write-step: 2 bytes=404:613 -->
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

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func run(ctx context.Context, args []string, output io.Writer) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>service.Capture(ctx, *operation, prepare)</code>；<code>ctx.Err</code>；<code>service.Export(ctx, *operation, w)</code> |
| <code>args</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>flags.Parse(args)</code> |
| <code>output</code> | <code>io.Writer</code> | 接收输出字节的流，写入可能失败或短写；不能只看业务计算完成就宣称文件已输出。<br>在此声明中的实际使用：<code>json.NewEncoder(output)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;REBUILD_SNAPSHOT_ARGUMENT_INVALID&quot;</code>；<code>&quot;REBUILD_SNAPSHOT_CONFIG_INVALID&quot;</code>；<code>&quot;REBUILD_SNAPSHOT_INPUT_INVALID&quot;</code>；<code>&quot;REBUILD_SNAPSHOT_DATABASE_UNAVAILABLE&quot;</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>flags.Parse(args) != nil &#124;&#124; flags.NArg() != 0 &#124;&#124; *config == &quot;&quot; &#124;&#124; !rebuildmeta.OperationValid(*operation) &#124;&#124; 		!strings.HasPrefix(*prepareRef, &quot;rebuild-prepare-v1:&quot;)</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_ARGUMENT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; cfg.Role != &quot;document&quot; &#124;&#124; cfg.Root == &quot;&quot; &#124;&#124; !filepath.IsAbs(cfg.Root)</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_INPUT_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_DATABASE_UNAVAILABLE&quot;</code>。当前函数在这里结束。
- 登记 <code>store.DB.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_CONFIG_INVALID&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, &quot;document&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, snapshot.ErrCommitUnknown)</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_COMMIT_UNKNOWN_RETRY_SAME_OPERATION&quot;</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_CANCELLED&quot;</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, snapshot.ErrConflict)</code>。条件成立时执行 <code>return &quot;REBUILD_OPERATION_CONFLICT&quot;</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=613:2526 -->
````go
func run(ctx context.Context, args []string, output io.Writer) string {
	flags := flag.NewFlagSet("search-snapshot", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Document 本服务配置文件")
	operation := flags.String("operation", "", "原操作号")
	prepareRef := flags.String("prepare-ref", "", "受控 inbox 内的 prepare 内容引用")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) ||
		!strings.HasPrefix(*prepareRef, "rebuild-prepare-v1:") {
		return "REBUILD_SNAPSHOT_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "document" || cfg.Root == "" || !filepath.IsAbs(cfg.Root) {
		return "REBUILD_SNAPSHOT_CONFIG_INVALID"
	}
	// prepare-ref 是内容地址而不是路径，ReadFile 同时限制文件名与核对原始字节摘要。
	inbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "document", "inbox")
	prepare, err := rebuildartifact.ReadFile(inbox, *prepareRef)
	if err != nil {
		return "REBUILD_SNAPSHOT_INPUT_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "REBUILD_SNAPSHOT_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	service := snapshot.Service{DB: store.DB, Verifier: verifier,
		LoadSigner: func() (*rebuildartifact.Signer, error) {
			return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "document")
		},
	}
	seal, err := service.Capture(ctx, *operation, prepare)
	if err != nil {
		if errors.Is(err, snapshot.ErrCommitUnknown) {
			return "REBUILD_SNAPSHOT_COMMIT_UNKNOWN_RETRY_SAME_OPERATION"
		}
		if ctx.Err() != nil {
			return "REBUILD_SNAPSHOT_CANCELLED"
		}
		if errors.Is(err, snapshot.ErrConflict) {
			return "REBUILD_OPERATION_CONFLICT"
		}
````

## 第 4 步：run（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 77 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;REBUILD_SNAPSHOT_CAPTURE_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_ENVELOPE_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !strings.HasSuffix(name, &quot;.snapshot.json&quot;)</code>。条件成立时执行 <code>return &quot;REBUILD_ARTIFACT_REFERENCE_INVALID&quot;</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!bytes.Equal(original, seal)</code>。条件成立时执行 <code>return errors.New(&quot;REBUILD_SNAPSHOT_SEAL_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ctx.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; n != len(seal)</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_CANCELLED&quot;</code>。当前函数在这里结束。
- 检查 <code>json.NewEncoder(output).Encode(struct { 		Reference string `json:&quot;reference&quot;` 	}{reference}) != nil</code>。条件成立时执行 <code>return &quot;REBUILD_SNAPSHOT_OUTPUT_FAILED&quot;</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2526:4115 -->
````go
		return "REBUILD_SNAPSHOT_CAPTURE_FAILED"
	}
	reference, err := rebuildartifact.Reference(seal, rebuildartifact.SnapshotV1)
	if err != nil {
		return "REBUILD_ARTIFACT_ENVELOPE_INVALID"
	}
	name, err := rebuildartifact.FileName(reference)
	if err != nil || !strings.HasSuffix(name, ".snapshot.json") {
		return "REBUILD_ARTIFACT_REFERENCE_INVALID"
	}
	outbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "document", "outbox")
	entriesName := strings.TrimSuffix(name, ".snapshot.json") + ".entries.ndjson"
	if err := rebuildartifact.PublishFile(outbox, entriesName, func(w io.Writer) error {
		original, err := service.Export(ctx, *operation, w)
		if err != nil {
			return err
		}
		if !bytes.Equal(original, seal) {
			return errors.New("REBUILD_SNAPSHOT_SEAL_CHANGED")
		}
		return ctx.Err()
	}); err != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
	// 条目先完整校验、Sync 并发布，原 seal 最后发布；中途失败可按原号重生，
	// 不能用重扫 document 或重签名补齐文件，也不能把 SQL 成功冒充文件发布成功。
	if err := rebuildartifact.PublishFile(outbox, name, func(w io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := w.Write(seal)
		if err == nil && n != len(seal) {
			err = io.ErrShortWrite
		}
		return err
	}); err != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
	if ctx.Err() != nil {
		return "REBUILD_SNAPSHOT_CANCELLED"
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{reference}) != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
````

## 第 5 步：run（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 123 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4115:4128 -->
````go
	return ""
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/cmd/search-snapshot/main.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
