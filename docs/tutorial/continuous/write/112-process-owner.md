# 分步编写 `services/document/internal/client/process_owner.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/112-process-owner.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证旧输出实例对应的指定PID、绝对入口和run-owner是否仍存在。

PID可能被复用，只知道某PID存在或不存在不足以判断原Document实例。

写之前先按这条执行路线理解各段的连接：

1. ProcessOwner.Verify调用平台probeProcess取得事实，再比对role、owner与绝对入口
2. 确认原进程已退出才返回ABSENT。

本文件需要守住的边界：

- 探测超时是OUTPUT_EXIT_PROOF_DEADLINE，不是退出
- 其他用户进程不能被当作本任务实例处理。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/process_owner.go') -and (Get-Item -LiteralPath 'services/document/internal/client/process_owner.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/process_owner.go')) { New-Item -ItemType File -Path 'services/document/internal/client/process_owner.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:96 -->
````go
package client

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

````

## 第 2 步：ProcessOwner

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type ProcessOwner struct{ root string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>string</code> | 项目运行根绝对路径，用于构造预期Doc/Edge入口路径核验进程归属。 |

<!-- write-step: 2 bytes=96:137 -->
````go
type ProcessOwner struct{ root string }

````

## 第 3 步：processFacts

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type processFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>Command</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;commandLine&quot;`</code> |
| <code>Args</code> | <code>&#91;&#93;string</code> | Linux内核提供的argv数组，按独立参数精确核对入口及--run-owner，不能只做owner前缀匹配。 |

<!-- write-step: 3 bytes=137:253 -->
````go
type processFacts struct {
	State   string `json:"state"`
	Command string `json:"commandLine"`
	Args    []string
}

````

## 第 4 步：NewProcessOwner

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func NewProcessOwner(root string) *ProcessOwner`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>root</code> | <code>string</code> | 在此声明中的实际使用：<code>root: root</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ProcessOwner</code> | 实际返回表达式：<code>&amp;ProcessOwner{root: root}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ProcessOwner{root: root}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=253:339 -->
````go
func NewProcessOwner(root string) *ProcessOwner { return &ProcessOwner{root: root} }

````

## 第 5 步：ProcessOwner.Verify

插入位置：第 4 步代码之后，紧接追加；完成稿约第 22 行。

Verify只接受指定PID、绝对入口和run-owner的实际证据；查询超时不等于旧进程已退出。

本步声明：`func (c *ProcessOwner) Verify(ctx context.Context, pid uint64, role, owner string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, 3*time.Second)</code> |
| <code>pid</code> | <code>uint64</code> | 在此声明中的实际使用：<code>pid == 0</code>；<code>pid &gt; 2147483647</code>；<code>c.probeProcess(probeCtx, pid)</code> |
| <code>role</code> | <code>string</code> | 在此声明中的实际使用：<code>role != &quot;document&quot;</code>；<code>role != &quot;edge&quot;</code>；<code>role == &quot;edge&quot;</code> |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>owner == &quot;&quot;</code>；<code>&quot;--run-owner &quot;+owner</code>；<code>facts.Args&#91;i+1&#93; == owner</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>facts.State</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_EXIT_PROOF_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;OUTPUT_EXIT_PROOF_DEADLINE&quot;)</code>；<code>errors.New(&quot;OLD_OUTPUT_PROCESS_STILL_RUNNING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>pid == 0 &#124;&#124; pid &gt; 2147483647 &#124;&#124; owner == &quot;&quot; &#124;&#124; (role != &quot;document&quot; &amp;&amp; role != &quot;edge&quot;)</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;OUTPUT_EXIT_PROOF_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 检查 <code>err != nil &#124;&#124; probeError != nil</code>。
- 检查 <code>probeError == context.DeadlineExceeded</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;OUTPUT_EXIT_PROOF_DEADLINE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;, errors.New(&quot;OUTPUT_EXIT_PROOF_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>runtime.GOOS == &quot;linux&quot;</code>。
- 检查 <code>role == &quot;edge&quot;</code>。
- 检查 <code>runtime.GOOS == &quot;linux&quot;</code>。
- 检查 <code>arg == &quot;--run-owner&quot; &amp;&amp; i+1 &lt; len(facts.Args) &amp;&amp; facts.Args&#91;i+1&#93; == owner</code>。
- 检查 <code>facts.State == &quot;PRESENT&quot; &amp;&amp; matched</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;OLD_OUTPUT_PROCESS_STILL_RUNNING&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.State != &quot;PRESENT&quot; &amp;&amp; facts.State != &quot;ABSENT&quot;</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;OUTPUT_EXIT_PROOF_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return facts.State, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=339:2204 -->
````go
// Verify只接受指定PID、绝对入口和run-owner的实际证据；查询超时不等于旧进程已退出。
func (c *ProcessOwner) Verify(ctx context.Context, pid uint64, role, owner string) (string, error) {
	if pid == 0 || pid > 2147483647 || owner == "" || (role != "document" && role != "edge") {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	facts, err := c.probeProcess(probeCtx, pid)
	// 先读取再取消，避免主动 cancel 把真实的探测超时原因覆盖掉；失败仍不能当退出证明。
	probeError := probeCtx.Err()
	cancel()
	if err != nil || probeError != nil {
		if probeError == context.DeadlineExceeded {
			return "", errors.New("OUTPUT_EXIT_PROOF_DEADLINE")
		}
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	entry := filepath.Join(c.root, ".runtime/bin/document.exe")
	if runtime.GOOS == "linux" {
		entry = filepath.Join(c.root, ".runtime/bin/document")
	}
	if role == "edge" {
		entry = filepath.Join(c.root, "scripts/edge.mjs")
	}
	matched := strings.Contains(strings.ToLower(facts.Command), strings.ToLower(entry)) && strings.Contains(facts.Command, "--run-owner "+owner)
	if runtime.GOOS == "linux" {
		// procfs给出独立argv，必须精确匹配入口与参数值，owner前缀不能代表原实例。
		entryFound, ownerFound := false, false
		for i, arg := range facts.Args {
			entryFound = entryFound || arg == entry
			if arg == "--run-owner" && i+1 < len(facts.Args) && facts.Args[i+1] == owner {
				ownerFound = true
			}
		}
		matched = entryFound && ownerFound
	}
	if facts.State == "PRESENT" && matched {
		return "", errors.New("OLD_OUTPUT_PROCESS_STILL_RUNNING")
	}
	if facts.State != "PRESENT" && facts.State != "ABSENT" {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}

	return facts.State, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/process_owner.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
