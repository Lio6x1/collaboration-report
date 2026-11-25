# 分步编写 `services/document/internal/exporter/formats.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/121-formats.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把统一AST转换为DOCX、PDF或含资源的Markdown包。

不同输出格式共享同一固定正文和资源，避免某种格式偷偷读取当前MAIN。

写之前先按这条执行路线理解各段的连接：

1. Renderer.convert按格式调用受控工具
2. markdownPackage用固定过滤器打包本地资源
3. embedPDFAssets把已验证附件嵌入渲染内容。

本文件需要守住的边界：

- 过滤器程序固定且HTML由转换器转义
- 不允许通过图片或超链接导出读取主机任意文件。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/formats.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/formats.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/formats.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/formats.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:211 -->
````go
package exporter

import (
	"archive/zip"
	domain "collaboration/document/internal/domain/document"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

````

## 第 2 步：Renderer.convert

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func (r *Renderer) convert(ctx context.Context, directory, input string, document domain.ExportDocument) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=docx&quot;, &quot;--output=&quot;+path, &quot;--resource-path=&quot;+directory)</code>；<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=markdown&quot;, &quot;--standalone&quot;, &quot;--wrap=none&quot;, &quot;--lua-filter=&quot;+filter, &quot;--output=&quot;+path)</code>；<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=html5&quot;, &quot;--standalone&quot;, &quot;--mathjax&quot;, &quot;--output=&quot;+path)</code>；<code>exec.CommandContext(ctx, r.config.Node, &quot;--no-maglev&quot;, &quot;--max-old-space-size=256&quot;, r.config.PDFScript, directory, r.config.Browser)</code> |
| <code>directory</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.Join(directory, &quot;document.docx&quot;)</code>；<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=docx&quot;, &quot;--output=&quot;+path, &quot;--resource-path=&quot;+directory)</code>；<code>&quot;--resource-path=&quot;+directory</code>；<code>filepath.Join(directory, &quot;document.md&quot;)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>string</code> | 在此声明中的实际使用：<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=docx&quot;, &quot;--output=&quot;+path, &quot;--resource-path=&quot;+directory)</code>；<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=markdown&quot;, &quot;--standalone&quot;, &quot;--wrap=none&quot;, &quot;--lua-filter=&quot;+filter, &quot;--output=&quot;+path)</code>；<code>r.pandoc(ctx, directory, input, &quot;--from=json&quot;, &quot;--to=html5&quot;, &quot;--standalone&quot;, &quot;--mathjax&quot;, &quot;--output=&quot;+path)</code> |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>document.Format</code>；<code>styleDOCX(path, document)</code>；<code>markdownPackage(directory, document)</code>；<code>document.Style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 由 <code>markdownPackage(directory, document)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&quot;&quot;</code>；<code>path</code>；<code>filepath.Join(directory, &quot;document.pdf&quot;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>markdownPackage(directory, document)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_PDF_RENDER_FAILED&quot;)</code>；<code>domain.Violation(domain.InvalidRequest, &quot;EXPORT_FORMAT_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 此处通过 <code>return path, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 此处通过 <code>return markdownPackage(directory, document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.StateConflict, &quot;EXPORT_PDF_RENDER_FAILED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=211:2243 -->
````go
func (r *Renderer) convert(ctx context.Context, directory, input string, document domain.ExportDocument) (string, error) {
	switch document.Format {
	case "DOCX":
		path := filepath.Join(directory, "document.docx")
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=docx", "--output="+path, "--resource-path="+directory); err != nil {
			return "", err
		}
		if err := styleDOCX(path, document); err != nil {
			return "", err
		}
		return path, nil
	case "MARKDOWN":
		path := filepath.Join(directory, "document.md")
		// Markdown本身没有对齐/首行缩进语法。由Pandoc将受控格式块转成完整HTML，
		// 避免fenced Div在普通阅读器里显示为文本，或把标题/强调留在HTML内当作字面量。
		filter := filepath.Join(directory, "paragraph-formatting.lua")
		if err := os.WriteFile(filter, []byte(markdownParagraphFilter), 0600); err != nil {
			return "", err
		}
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=markdown", "--standalone", "--wrap=none", "--lua-filter="+filter, "--output="+path); err != nil {
			return "", err
		}
		return markdownPackage(directory, document)
	case "PDF":
		path := filepath.Join(directory, "document.html")
		if err := r.pandoc(ctx, directory, input, "--from=json", "--to=html5", "--standalone", "--mathjax", "--output="+path); err != nil {
			return "", err
		}
		if err := embedPDFAssets(path, directory); err != nil {
			return "", err
		}
		settings, err := json.Marshal(document.Style.Style)
		if err != nil {
			return "", err
		}
		if err = os.WriteFile(filepath.Join(directory, "style.json"), settings, 0600); err != nil {
			return "", err
		}
		var diagnostic limitedBuffer
		cmd := exec.CommandContext(ctx, r.config.Node, "--no-maglev", "--max-old-space-size=256", r.config.PDFScript, directory, r.config.Browser)
		cmd.Dir = directory
		cmd.Stdout = &diagnostic
		cmd.Stderr = &diagnostic
		if err = runProgram(cmd); err != nil {
			return "", domain.Violation(domain.StateConflict, "EXPORT_PDF_RENDER_FAILED")
		}
````

## 第 3 步：Renderer.convert（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 63 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return filepath.Join(directory, &quot;document.pdf&quot;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;EXPORT_FORMAT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2243:2381 -->
````go
		return filepath.Join(directory, "document.pdf"), nil
	}
	return "", domain.Violation(domain.InvalidRequest, "EXPORT_FORMAT_INVALID")
}

````

## 第 4 步：markdownParagraphFilter

插入位置：第 3 步代码之后，紧接追加；完成稿约第 68 行。

过滤器内容是固定程序，用户只能提供已由共享schema校验的属性；HTML仍由成熟转换器转义。

本步声明：`const markdownParagraphFilter = \`function Div(block)`。

<!-- write-step: 4 bytes=2381:2811 -->
````go
// 过滤器内容是固定程序，用户只能提供已由共享schema校验的属性；HTML仍由成熟转换器转义。
const markdownParagraphFilter = `function Div(block)
  if block.attributes["data-paragraph-format"] == "true" then
    block.attributes["data-paragraph-format"] = nil
    block.attributes["custom-style"] = nil
    return pandoc.RawBlock("html", pandoc.write(pandoc.Pandoc({block}), "html5"))
  end
end
`

````

## 第 5 步：markdownPackage

插入位置：第 4 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func markdownPackage(directory string, document domain.ExportDocument) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>directory</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.Join(directory, &quot;document.zip&quot;)</code>；<code>filepath.Join(directory, &quot;assets&quot;)</code>；<code>filepath.Join(directory, filepath.FromSlash(name))</code> |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>document.Target</code>；<code>document.Manifest</code>；<code>document.Release</code>；<code>document.Captured</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>path</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>writeErr</code>；<code>closeErr</code>；<code>fileErr</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return &quot;&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return &quot;&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>writeErr == nil</code>。
- 检查 <code>writeErr != nil</code>。
- 此处通过 <code>return &quot;&quot;, writeErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>closeErr != nil</code>。条件成立时执行 <code>return &quot;&quot;, closeErr</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=2811:4383 -->
````go
func markdownPackage(directory string, document domain.ExportDocument) (string, error) {
	path := filepath.Join(directory, "document.zip")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	archive := zip.NewWriter(file)
	names := []string{"document.md"}
	items, err := os.ReadDir(filepath.Join(directory, "assets"))
	if err != nil {
		archive.Close()
		file.Close()
		return "", err
	}
	for _, item := range items {
		names = append(names, "assets/"+item.Name())
	}
	for _, name := range names {
		body, err := os.Open(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil {
			archive.Close()
			file.Close()
			return "", err
		}
		entry, writeErr := archive.Create(name)
		if writeErr == nil {
			_, writeErr = io.Copy(entry, body)
		}
		body.Close()
		if writeErr != nil {
			archive.Close()
			file.Close()
			return "", writeErr
		}
	}
	entry, err := archive.Create("README.txt")
	if err == nil {
		_, err = io.WriteString(entry, "固定版本导出\n目标："+document.Target+"\n清单："+document.Manifest+"\n正式版："+document.Release+"\n时间："+document.Captured.UTC().Format("2006-01-02 15:04:05 UTC")+"\n解压整个压缩包后打开document.md；图片只引用同包assets。动画图采用首帧，公式以LaTeX保留。段落对齐与首行缩进保留为受控HTML/CSS；字体与段落格式的实际显示取决于阅读器是否支持HTML/CSS。\n")
	}
	closeErr := archive.Close()
	fileErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
````

## 第 6 步：markdownPackage（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 123 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;&quot;, closeErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>fileErr != nil</code>。条件成立时执行 <code>return &quot;&quot;, fileErr</code>。当前函数在这里结束。
- 此处通过 <code>return path, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4383:4474 -->
````go
		return "", closeErr
	}
	if fileErr != nil {
		return "", fileErr
	}
	return path, nil
}

````

## 第 7 步：localImage

插入位置：第 6 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`var localImage = regexp.MustCompile(\`src="(assets/resource-[1-9][0-9]*\.png)"\`)`。

<!-- write-step: 7 bytes=4474:4555 -->
````go
var localImage = regexp.MustCompile(`src="(assets/resource-[1-9][0-9]*\.png)"`)

````

## 第 8 步：embedPDFAssets

插入位置：第 7 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func embedPDFAssets(path, directory string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>os.ReadFile(path)</code>；<code>os.WriteFile(path, &#91;&#93;byte(result), 0600)</code> |
| <code>directory</code> | <code>string</code> | 在此声明中的实际使用：<code>filepath.Join(directory, filepath.FromSlash(name))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>；<code>os.WriteFile(path, &#91;&#93;byte(result), 0600)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(body) &gt; 48&lt;&lt;20</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>size &gt; 64&lt;&lt;20</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return `src=&quot;` + cache&#91;match&#91;1&#93;&#93; + `&quot;`</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>strings.Contains(result, `src=&quot;assets/`)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return os.WriteFile(path, &#91;&#93;byte(result), 0600)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4555:5894 -->
````go
func embedPDFAssets(path, directory string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(body) > 48<<20 {
		return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
	}
	cache := map[string]string{}
	size := len(body)
	for _, match := range localImage.FindAllSubmatch(body, -1) {
		name := string(match[1])
		data, exists := cache[name]
		if !exists {
			bytes, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			data = "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes)
			cache[name] = data
		}
		size += len(data) - len(name)
		if size > 64<<20 {
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
	}
	result := localImage.ReplaceAllStringFunc(string(body), func(source string) string {
		match := localImage.FindStringSubmatch(source)
		return `src="` + cache[match[1]] + `"`
	})
	// Pandoc的MathJax远程脚本不执行；后续本地KaTeX统一处理，浏览器禁用任意网络。
	scripts := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`)
	result = scripts.ReplaceAllString(result, "")
	if strings.Contains(result, `src="assets/`) {
		return domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
	}
	return os.WriteFile(path, []byte(result), 0600)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/formats.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
