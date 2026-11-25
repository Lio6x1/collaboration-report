# 分步编写 `services/document/internal/exporter/rendering_export.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/125-rendering-export.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证转换输入预算、工具配置，管理临时目录与最终不可变制品。

正文体积上限不能限制AST展开量，大量空格也会创建海量节点，必须有整作业节点预算。

写之前先按这条执行路线理解各段的连接：

1. validateDocument先估算节点
2. New验证Pandoc及工具
3. Render写资源和AST、调用convert、校验制品并存入Files
4. Cleanup删除本job临时
5. limitedBuffer限制工具输出。

本文件需要守住的边界：

- 制品key绑定作业代次
- 转换失败或超限不能发布半份文件
- 固定工具版本是结果可重复的前提。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/rendering_export.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/rendering_export.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/rendering_export.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/rendering_export.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:471 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package exporter

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/document/internal/storage/local"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// limits.go：保留原实现，按业务合并文件。

````

## 第 2 步：validateDocument

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

在分配Pandoc节点之前估算展开量。正文的字节上限不足以保护转换器：
一串空格会变成大量Space节点，必须先有整个作业共享的节点预算。

本步声明：`func validateDocument(document domain.ExportDocument) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>document.Style</code>；<code>document.Title</code>；<code>document.Structure</code>；<code>document.Units</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;EXPORT_STYLE_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>(style.BodyFont != &quot;SimSun&quot; &amp;&amp; style.BodyFont != &quot;FangSong&quot;) &#124;&#124; style.WesternFont != &quot;Times New Roman&quot; &#124;&#124; style.BodySizePt != 12 &#124;&#124; style.Page != &quot;A4&quot; &#124;&#124; style.MarginCm != &quot;2.54&quot; &#124;&#124; len(style.HeadingSizesPt) != 6</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_STYLE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>size &lt; 1 &#124;&#124; size &gt; 72</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_STYLE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>style.TitleFont != nil &amp;&amp; *style.TitleFont != &quot;SimHei&quot; &#124;&#124; style.TitleSizePt != nil &amp;&amp; *style.TitleSizePt != 22</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_STYLE_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>remaining &lt; 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!visit(child)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return remaining &gt;= 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!visit(document.Title)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>!visit(entry.Title)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=471:2109 -->
````go
// 在分配Pandoc节点之前估算展开量。正文的字节上限不足以保护转换器：
// 一串空格会变成大量Space节点，必须先有整个作业共享的节点预算。
func validateDocument(document domain.ExportDocument) error {
	style := document.Style.Style
	if (style.BodyFont != "SimSun" && style.BodyFont != "FangSong") || style.WesternFont != "Times New Roman" || style.BodySizePt != 12 || style.Page != "A4" || style.MarginCm != "2.54" || len(style.HeadingSizesPt) != 6 {
		return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
	}
	for _, size := range style.HeadingSizesPt {
		if size < 1 || size > 72 {
			return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
		}
	}
	if style.TitleFont != nil && *style.TitleFont != "SimHei" || style.TitleSizePt != nil && *style.TitleSizePt != 22 {
		return domain.Violation(domain.StateConflict, "EXPORT_STYLE_INVALID")
	}
	remaining := 500000
	var visit func(any) bool
	visit = func(value any) bool {
		remaining -= 8
		if remaining < 0 {
			return false
		}
		switch value := value.(type) {
		case string:
			remaining -= 2 * (strings.Count(value, " ") + strings.Count(value, "\n") + strings.Count(value, "\t"))
		case []any:
			for _, child := range value {
				if !visit(child) {
					return false
				}
			}
		case map[string]any:
			for _, child := range value {
				if !visit(child) {
					return false
				}
			}
		}
		return remaining >= 0
	}
	if !visit(document.Title) {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	for _, entry := range document.Structure.Entries {
		if !visit(entry.Title) {
````

## 第 3 步：validateDocument（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 68 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!visit(unit.Body)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2109:2415 -->
````go
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	for _, unit := range document.Units {
		if !visit(unit.Body) {
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	return nil
}

// renderer.go：保留原实现，按业务合并文件。

````

## 第 4 步：Config

插入位置：第 3 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`type Config struct{ Root, Pandoc, Node, PDFScript, Browser string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Root</code> | <code>string</code> | 导出专用存储根绝对路径，临时产物和结果限制在此范围。 |
| <code>Pandoc</code> | <code>string</code> | 服务端Pandoc可执行文件绝对路径。 |
| <code>Node</code> | <code>string</code> | 服务端Node可执行文件绝对路径。 |
| <code>PDFScript</code> | <code>string</code> | 项目PDF渲染脚本绝对路径。 |
| <code>Browser</code> | <code>string</code> | 用于PDF渲染的浏览器路径/配置，交给PDF脚本选择转换环境。 |

<!-- write-step: 4 bytes=2415:2483 -->
````go
type Config struct{ Root, Pandoc, Node, PDFScript, Browser string }
````

## 第 5 步：Renderer

插入位置：第 4 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`type Renderer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>config</code> | <code>Config</code> | 已校验的转换工具与目录配置，浏览器下载用户无需安装这些工具。 |
| <code>files</code> | <code>*local.Store</code> | 使用 <code>local.Store</code> 类型；其定义处的职责：在专用本地目录实现不可变对象Put/Open/Delete。 |

<!-- write-step: 5 bytes=2483:2545 -->
````go
type Renderer struct {
	config Config
	files  *local.Store
}

````

## 第 6 步：New

插入位置：第 5 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func New(config Config) (*Renderer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>config</code> | <code>Config</code> | 在此声明中的实际使用：<code>config.Root</code>；<code>config.Pandoc</code>；<code>config.Node</code>；<code>config.PDFScript</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Renderer</code> | 实际返回表达式：<code>nil</code>；<code>&amp;Renderer{config: config, files: files}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;EXPORT_CONFIG_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;EXPORT_CONVERTER_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;EXPORT_PANDOC_VERSION_REQUIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!filepath.IsAbs(config.Root) &#124;&#124; !filepath.IsAbs(config.Pandoc) &#124;&#124; !filepath.IsAbs(config.Node) &#124;&#124; !filepath.IsAbs(config.PDFScript)</code>。条件成立时执行 <code>return nil, errors.New(&quot;EXPORT_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return nil, errors.New(&quot;EXPORT_CONVERTER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(context.Background(), 3*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; !strings.HasPrefix(output.String(), &quot;pandoc 3.8\n&quot;) &amp;&amp; !strings.HasPrefix(output.String(), &quot;pandoc 3.8\r\n&quot;)</code>。条件成立时执行 <code>return nil, errors.New(&quot;EXPORT_PANDOC_VERSION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Renderer{config: config, files: files}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2545:3566 -->
````go
func New(config Config) (*Renderer, error) {
	if !filepath.IsAbs(config.Root) || !filepath.IsAbs(config.Pandoc) || !filepath.IsAbs(config.Node) || !filepath.IsAbs(config.PDFScript) {
		return nil, errors.New("EXPORT_CONFIG_INVALID")
	}
	files, err := local.New(config.Root)
	if err != nil {
		return nil, err
	}
	for _, path := range []string{config.Pandoc, config.Node, config.PDFScript} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("EXPORT_CONVERTER_UNAVAILABLE")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output limitedBuffer
	cmd := exec.CommandContext(ctx, config.Pandoc, "--version")
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err = runProgram(cmd); err != nil || !strings.HasPrefix(output.String(), "pandoc 3.8\n") && !strings.HasPrefix(output.String(), "pandoc 3.8\r\n") {
		return nil, errors.New("EXPORT_PANDOC_VERSION_REQUIRED")
	}
	return &Renderer{config: config, files: files}, nil
}

````

## 第 7 步：Renderer.Files

插入位置：第 6 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func (r *Renderer) Files() logic.ResourceFiles`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>logic.ResourceFiles</code> | 使用 <code>logic.ResourceFiles</code> 类型；其定义处的职责：统一的资源存储接口，保持本地和OSS都具备锁定、完整性核验、释放与删除语义。<br>实际返回表达式：<code>r.files</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return r.files</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3566:3632 -->
````go
func (r *Renderer) Files() logic.ResourceFiles { return r.files }
````

## 第 8 步：artifactKey

插入位置：第 7 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func artifactKey(value domain.ExportRecord) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>value.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(hash&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(hash&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3632:3793 -->
````go
func artifactKey(value domain.ExportRecord) string {
	hash := sha256.Sum256([]byte("EXPORT_ARTIFACT\n" + value.Operation))
	return hex.EncodeToString(hash[:])
}
````

## 第 9 步：Renderer.directory

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func (r *Renderer) directory(value domain.ExportRecord) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>artifactKey(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>filepath.Join(r.config.Root, &quot;work-&quot;+artifactKey(value))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return filepath.Join(r.config.Root, &quot;work-&quot;+artifactKey(value))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3793:3926 -->
````go
func (r *Renderer) directory(value domain.ExportRecord) string {
	return filepath.Join(r.config.Root, "work-"+artifactKey(value))
}

````

## 第 10 步：Renderer.Cleanup

插入位置：第 9 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`func (r *Renderer) Cleanup(ctx context.Context, value domain.ExportRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.files.Acquire(ctx, key)</code>；<code>r.files.CleanupUpload(ctx, key)</code>；<code>r.files.Delete(ctx, key)</code> |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>artifactKey(value)</code>；<code>r.directory(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EXPORT_TEMPORARY_CLEANUP_FAILED&quot;)</code>；<code>r.files.Delete(ctx, key)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;EXPORT_TEMPORARY_CLEANUP_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return r.files.Delete(ctx, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3926:4467 -->
````go
func (r *Renderer) Cleanup(ctx context.Context, value domain.ExportRecord) error {
	key := artifactKey(value)
	unlock, err := r.files.Acquire(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	// 目录来自固定原号hash，只清本作业；根目录的跨进程身份/符号链接校验由Store完成。
	if err = os.RemoveAll(r.directory(value)); err != nil {
		return errors.New("EXPORT_TEMPORARY_CLEANUP_FAILED")
	}
	if err = r.files.CleanupUpload(ctx, key); err != nil {
		return err
	}
	return r.files.Delete(ctx, key)
}

````

## 第 11 步：Renderer.Render

插入位置：第 10 步代码之后，紧接追加；完成稿约第 139 行。

本步声明：`func (r *Renderer) Render(ctx context.Context, value domain.ExportRecord, document domain.ExportDocument) (string, uint64, [32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.Cleanup(ctx, value)</code>；<code>context.WithTimeout(ctx, 60*time.Second)</code>；<code>r.files.Acquire(ctx, key)</code>；<code>r.files.Put(ctx, key, file, uint64(info.Size()), checksum)</code> |
| <code>value</code> | <code>domain.ExportRecord</code> | 使用 <code>domain.ExportRecord</code> 类型；其定义处的职责：定义导出union输入、固定计划、作业记录和转换器输入正文。<br>在此声明中的实际使用：<code>r.Cleanup(ctx, value)</code>；<code>r.directory(value)</code>；<code>artifactKey(value)</code> |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>document.Assets</code>；<code>buildAST(document, assets)</code>；<code>r.convert(attempt, directory, input, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>key</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>uint64(info.Size())</code> |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>empty</code>；<code>checksum</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EXPORT_TEMPORARY_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>；<code>errors.New(&quot;EXPORT_ARTIFACT_MISSING&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, errors.New(&quot;EXPORT_TEMPORARY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>os.RemoveAll(directory)</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 检查 <code>len(ast) &gt; 48&lt;&lt;20</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 60*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, errors.New(&quot;EXPORT_ARTIFACT_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>info.Size() &lt;= 0 &#124;&#124; info.Size() &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, domain.Violation(domain.StateConflict, &quot;EXPORT_ARTIFACT_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 登记 <code>file.Close()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 11 bytes=4467:5991 -->
````go
func (r *Renderer) Render(ctx context.Context, value domain.ExportRecord, document domain.ExportDocument) (string, uint64, [32]byte, error) {
	var empty [32]byte
	if err := r.Cleanup(ctx, value); err != nil {
		return "", 0, empty, err
	}
	directory := r.directory(value)
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", 0, empty, errors.New("EXPORT_TEMPORARY_UNAVAILABLE")
	}
	defer os.RemoveAll(directory)
	assets, err := writeAssets(directory, document.Assets)
	if err != nil {
		return "", 0, empty, err
	}
	ast, err := buildAST(document, assets)
	if err != nil {
		return "", 0, empty, err
	}
	if len(ast) > 48<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	input := filepath.Join(directory, "source.json")
	if err = os.WriteFile(input, ast, 0600); err != nil {
		return "", 0, empty, err
	}
	// 每次转换共享一个60秒预算，不给每个步骤分别续满时限。
	attempt, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	path, err := r.convert(attempt, directory, input, document)
	if err != nil {
		return "", 0, empty, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, empty, errors.New("EXPORT_ARTIFACT_MISSING")
	}
	if info.Size() <= 0 || info.Size() > 32<<20 {
		return "", 0, empty, domain.Violation(domain.StateConflict, "EXPORT_ARTIFACT_LIMIT")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, empty, err
	}
	defer file.Close()
	digest := sha256.New()
````

## 第 12 步：Renderer.Render（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 184 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 登记 <code>unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, 0, empty, err</code>。当前函数在这里结束。
- 此处通过 <code>return key, uint64(info.Size()), checksum, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5991:6509 -->
````go
	if _, err = io.Copy(digest, file); err != nil {
		return "", 0, empty, err
	}
	var checksum [32]byte
	copy(checksum[:], digest.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", 0, empty, err
	}
	key := artifactKey(value)
	unlock, err := r.files.Acquire(ctx, key)
	if err != nil {
		return "", 0, empty, err
	}
	defer unlock()
	if err = r.files.Put(ctx, key, file, uint64(info.Size()), checksum); err != nil {
		return "", 0, empty, err
	}
	return key, uint64(info.Size()), checksum, nil
}

````

## 第 13 步：limitedBuffer

插入位置：第 12 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`type limitedBuffer struct{ bytes.Buffer }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>bytes.Buffer</code> | <code>bytes.Buffer</code> | 嵌入的类型/接口，组合其字段或方法集合。  |

<!-- write-step: 13 bytes=6509:6552 -->
````go
type limitedBuffer struct{ bytes.Buffer }

````

## 第 14 步：limitedBuffer.Write

插入位置：第 13 步代码之后，紧接追加；完成稿约第 206 行。

本步声明：`func (b *limitedBuffer) Write(data []byte) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(data)</code>；<code>data = data&#91;:left&#93;</code>；<code>b.Buffer.Write(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>n</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>left &gt; 0</code>。
- 检查 <code>left &lt; len(data)</code>。
- 此处通过 <code>return n, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6552:6772 -->
````go
func (b *limitedBuffer) Write(data []byte) (int, error) {
	n := len(data)
	left := (32 << 10) - b.Len()
	if left > 0 {
		if left < len(data) {
			data = data[:left]
		}
		_, _ = b.Buffer.Write(data)
	}
	return n, nil
}

````

## 第 15 步：Renderer.pandoc

插入位置：第 14 步代码之后，紧接追加；完成稿约第 218 行。

本步声明：`func (r *Renderer) pandoc(ctx context.Context, directory string, args ...string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>exec.CommandContext(ctx, r.config.Pandoc, args...)</code> |
| <code>directory</code> | <code>string</code> | 在此声明中的实际使用：<code>cmd.Dir = directory</code> |
| <code>args</code> | <code>...string</code> | 在此声明中的实际使用：<code>exec.CommandContext(ctx, r.config.Pandoc, args...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_FAILED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_FORMULA_UNSUPPORTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>strings.Contains(diagnostic.String(), &quot;Could not convert TeX math&quot;)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_FORMULA_UNSUPPORTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6772:7426 -->
````go
func (r *Renderer) pandoc(ctx context.Context, directory string, args ...string) error {
	var diagnostic limitedBuffer
	cmd := exec.CommandContext(ctx, r.config.Pandoc, args...)
	cmd.Dir = directory
	cmd.Stdout = &diagnostic
	cmd.Stderr = &diagnostic
	if err := runProgram(cmd); err != nil {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_FAILED")
	}
	// texmath无法表示的公式不得被静默丢失；日志可能含用户公式，只保留固定错误码。
	if strings.Contains(diagnostic.String(), "Could not convert TeX math") {
		return domain.Violation(domain.StateConflict, "EXPORT_FORMULA_UNSUPPORTED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/rendering_export.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
