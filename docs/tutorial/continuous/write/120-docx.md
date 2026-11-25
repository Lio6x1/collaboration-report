# 分步编写 `services/document/internal/exporter/docx.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/120-docx.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

在转换器生成的DOCX包内调整既定版式并保留原生对象。

Word应保持可编辑OMML、表格和图片关系；版式修饰不应重新拼接用户XML。

写之前先按这条执行路线理解各段的连接：

1. styleDOCX打开OOXML包，按固定样式枚举设置字体字号和版式，受包与制品大小预算限制。

本文件需要守住的边界：

- 不要将用户内容直接串接进XML
- 必须保留原生关系而非截图替代。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/docx.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/docx.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/docx.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/docx.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:160 -->
````go
package exporter

import (
	"archive/zip"
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

````

## 第 2 步：styleBlock

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var styleBlock = regexp.MustCompile(\`(?s)<w:style\b[^>]*>.*?</w:style>\`)`。

<!-- write-step: 2 bytes=160:233 -->
````go
var styleBlock = regexp.MustCompile(`(?s)<w:style\b[^>]*>.*?</w:style>`)
````

## 第 3 步：runProperties

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`var runProperties = regexp.MustCompile(\`(?s)<w:rPr>.*?</w:rPr>\`)`。

<!-- write-step: 3 bytes=233:298 -->
````go
var runProperties = regexp.MustCompile(`(?s)<w:rPr>.*?</w:rPr>`)
````

## 第 4 步：fontProperties

插入位置：第 3 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`var fontProperties = regexp.MustCompile(\`<w:(?:rFonts|sz|szCs|color)\b[^>]*/>\`)`。

<!-- write-step: 4 bytes=298:378 -->
````go
var fontProperties = regexp.MustCompile(`<w:(?:rFonts|sz|szCs|color)\b[^>]*/>`)
````

## 第 5 步：sectionProperties

插入位置：第 4 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`var sectionProperties = regexp.MustCompile(\`(?s)<w:sectPr(?:\s[^>]*)?>.*?</w:sectPr>\`)`。

<!-- write-step: 5 bytes=378:466 -->
````go
var sectionProperties = regexp.MustCompile(`(?s)<w:sectPr(?:\s[^>]*)?>.*?</w:sectPr>`)

````

## 第 6 步：styleDOCX

插入位置：第 5 步代码之后，紧接追加；完成稿约第 20 行。

在成熟转换器生成的OOXML上设置既定版式，保留其原生OMML、表格和图片关系。
不用截图伪装可编辑Word，也不把用户内容作为XML拼接；字体和字号来自固定枚举。

本步声明：`func styleDOCX(path string, document domain.ExportDocument) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>zip.OpenReader(path)</code>；<code>os.WriteFile(path, output.Bytes(), 0600)</code> |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>document.Style</code>；<code>document.Units</code>；<code>document.Captured</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_DOCX_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_ARTIFACT_LIMIT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>file.UncompressedSize64 &gt; 64&lt;&lt;20 &#124;&#124; bytesCount &gt; (64&lt;&lt;20)-file.UncompressedSize64</code>。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONVERSION_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!exists</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCX_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return fmt.Sprintf(`&lt;w:rFonts w:ascii=&quot;Times New Roman&quot; w:hAnsi=&quot;Times New Roman&quot; w:eastAsia=&quot;%s&quot; w:cs=&quot;Times New Roman&quot;/&gt;&lt;w:sz w:val=&quot;%d&quot;/&gt;&lt;w:szCs w:val=&quot;%d&quot;/&gt;&lt;w:color w:val=&quot;000000&quot;/&gt;`, family, points*2, points*2)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>len(id) != 2</code>。条件成立时执行 <code>return block</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=466:2109 -->
````go
// 在成熟转换器生成的OOXML上设置既定版式，保留其原生OMML、表格和图片关系。
// 不用截图伪装可编辑Word，也不把用户内容作为XML拼接；字体和字号来自固定枚举。
func styleDOCX(path string, document domain.ExportDocument) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	var bytesCount uint64
	for _, file := range reader.File {
		if file.UncompressedSize64 > 64<<20 || bytesCount > (64<<20)-file.UncompressedSize64 {
			reader.Close()
			return domain.Violation(domain.StateConflict, "EXPORT_CONVERSION_LIMIT")
		}
		bytesCount += file.UncompressedSize64
		body, err := file.Open()
		if err != nil {
			reader.Close()
			return err
		}
		data, err := io.ReadAll(io.LimitReader(body, 64<<20))
		body.Close()
		if err != nil {
			reader.Close()
			return err
		}
		files[file.Name] = data
	}
	reader.Close()
	styles, exists := files["word/styles.xml"]
	if !exists {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	fixed := document.Style.Style
	font := func(family string, points int64) string {
		return fmt.Sprintf(`<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:eastAsia="%s" w:cs="Times New Roman"/><w:sz w:val="%d"/><w:szCs w:val="%d"/><w:color w:val="000000"/>`, family, points*2, points*2)
	}
	changed := styleBlock.ReplaceAllStringFunc(string(styles), func(block string) string {
		id := regexp.MustCompile(`w:styleId="([^"]+)"`).FindStringSubmatch(block)
		if len(id) != 2 {
			return block
		}
		family, points := fixed.BodyFont, int64(12)
		switch id[1] {
		case "Title":
````

## 第 7 步：styleDOCX（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 65 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>id&#91;1&#93; == fmt.Sprintf(&quot;Heading%d&quot;, index+1)</code>。
- 在本段的匿名函数内：检查 <code>!matched</code>。条件成立时执行 <code>return block</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>runProperties.MatchString(block)</code>。条件成立时执行 <code>return runProperties.ReplaceAllStringFunc(block, func(old string) string { 				return strings.Replace(fontProperties.ReplaceAllString(old, &quot;&quot;), `&lt;/w:rPr&gt;`, font(family, points)+`&lt;/w:rPr&gt;`, 1) 			})</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return strings.Replace(fontProperties.ReplaceAllString(old, &quot;&quot;), `&lt;/w:rPr&gt;`, font(family, points)+`&lt;/w:rPr&gt;`, 1)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return strings.Replace(block, `&lt;/w:style&gt;`, properties+`&lt;/w:style&gt;`, 1)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>strings.Contains(block, `w:styleId=&quot;`+name+`&quot;`)</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return block</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>family == &quot;Times New Roman&quot;</code>。

<!-- write-step: 7 bytes=2109:4027 -->
````go
			family = "SimHei"
			points = 22
		case "Normal", "BodyText", "FirstParagraph", "Compact":
		default:
			matched := false
			for index, size := range fixed.HeadingSizesPt {
				if id[1] == fmt.Sprintf("Heading%d", index+1) {
					family = "SimHei"
					points = size
					matched = true
					break
				}
			}
			if !matched {
				return block
			}
		}
		properties := `<w:rPr>` + font(family, points) + `</w:rPr>`
		if runProperties.MatchString(block) {
			// 只替换字体/字号，保留转换器已有的粗体、斜体及其它排版属性。
			return runProperties.ReplaceAllStringFunc(block, func(old string) string {
				return strings.Replace(fontProperties.ReplaceAllString(old, ""), `</w:rPr>`, font(family, points)+`</w:rPr>`, 1)
			})
		}
		return strings.Replace(block, `</w:style>`, properties+`</w:style>`, 1)
	})
	for _, family := range []string{"SimSun", "FangSong", "Times New Roman", "SimHei"} {
		name := "Export" + strings.ReplaceAll(family, " ", "")
		// 删除Pandoc可能自动产生的同名占位，避免两个styleId让Word随机选择。
		changed = styleBlock.ReplaceAllStringFunc(changed, func(block string) string {
			if strings.Contains(block, `w:styleId="`+name+`"`) {
				return ""
			}
			return block
		})
		properties := font(family, 12)
		if family == "Times New Roman" {
			properties = `<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:eastAsia="Times New Roman"/><w:sz w:val="24"/>`
		}
		changed = strings.Replace(changed, `</w:styles>`, `<w:style w:type="character" w:styleId="`+name+`"><w:name w:val="`+name+`"/><w:rPr>`+properties+`</w:rPr></w:style></w:styles>`, 1)
	}
	// Pandoc支持Para的custom-style，但Header始终写HeadingN。先建立继承原正文/
	// 标题样式的段落格式，再按受控书签定位标题，保留大纲级别、字号、公式和图片。
	formats := map[string]string{}
	headingFormats := map[string]string{}
````

## 第 8 步：styleDOCX（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 110 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>kind == &quot;paragraph&quot; &#124;&#124; kind == &quot;heading&quot;</code>。
- 在本段的匿名函数内：检查 <code>name != &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>kind == &quot;heading&quot;</code>。
- 在本段的匿名函数内：检查 <code>align != &quot;center&quot; &amp;&amp; align != &quot;right&quot;</code>。
- 在本段的匿名函数内：检查 <code>indent</code>。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>strings.Contains(block, `w:styleId=&quot;`+name+`&quot;`)</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=4027:5838 -->
````go
	var collect func(string, map[string]any)
	collect = func(unit string, item map[string]any) {
		kind := stringValue(item["type"])
		attrs, _ := item["attrs"].(map[string]any)
		if kind == "paragraph" || kind == "heading" {
			name, _ := paragraphFormat(kind, attrs)
			if name != "" {
				base, size := "BodyText", int64(12)
				if kind == "heading" {
					level := integer(attrs["level"])
					base = fmt.Sprintf("Heading%d", level)
					size = fixed.HeadingSizesPt[level-1]
					headingFormats[paragraphBookmark(unit, stringValue(attrs["id"]))] = name
				}
				align := stringValue(attrs["align"])
				if align != "center" && align != "right" {
					align = "left"
				}
				firstLine, characters := int64(0), 0
				if indent, _ := attrs["firstLineIndent"].(bool); indent {
					firstLine, characters = size*40, 200
				}
				// firstLineChars以百分之一字符计，200即两字符；twip值照顾旧阅读器。
				properties := fmt.Sprintf(`<w:pPr><w:jc w:val="%s"/><w:ind w:firstLine="%d" w:firstLineChars="%d"/></w:pPr>`, align, firstLine, characters)
				formats[name] = `<w:style w:type="paragraph" w:customStyle="1" w:styleId="` + name + `"><w:name w:val="` + name + `"/><w:basedOn w:val="` + base + `"/>` + properties + `</w:style>`
			}
		}
		children, _ := item["content"].([]any)
		for _, child := range children {
			if node, ok := child.(map[string]any); ok {
				collect(unit, node)
			}
		}
	}
	for _, unit := range document.Units {
		collect(unit.ID, unit.Body)
	}
	formatNames := make([]string, 0, len(formats))
	for name := range formats {
		formatNames = append(formatNames, name)
	}
	sort.Strings(formatNames)
	for _, name := range formatNames {
		changed = styleBlock.ReplaceAllStringFunc(changed, func(block string) string {
			if strings.Contains(block, `w:styleId="`+name+`"`) {
````

## 第 9 步：styleDOCX（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return block</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!exists &#124;&#124; !paragraphStyle.MatchString(block)</code>。条件成立时执行 <code>return block</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return paragraphStyle.ReplaceAllString(block, `&lt;w:pStyle w:val=&quot;`+name+`&quot;/&gt;`)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(applied) != len(headingFormats)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCX_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return strings.Replace(properties, `&lt;/w:tblPr&gt;`, borders+`&lt;/w:tblPr&gt;`, 1)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!sectionProperties.MatchString(body)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_DOCX_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=5838:9402 -->
````go
				return ""
			}
			return block
		})
		changed = strings.Replace(changed, `</w:styles>`, formats[name]+`</w:styles>`, 1)
	}
	files["word/styles.xml"] = []byte(changed)
	body := string(files["word/document.xml"])
	paragraphStyle := regexp.MustCompile(`<w:pStyle\b[^>]*/>`)
	markedParagraph := regexp.MustCompile(`(?s)<w:bookmarkStart\b[^>]*w:name="(exportformat[0-9a-f]{24})"[^>]*/>\s*<w:p>.*?</w:p>`)
	applied := map[string]bool{}
	// 一次扫描整篇OOXML，避免正文标题较多时按标题反复扫描整个Word文件。
	body = markedParagraph.ReplaceAllStringFunc(body, func(block string) string {
		bookmark := markedParagraph.FindStringSubmatch(block)[1]
		name, exists := headingFormats[bookmark]
		if !exists || !paragraphStyle.MatchString(block) {
			return block
		}
		applied[bookmark] = true
		return paragraphStyle.ReplaceAllString(block, `<w:pStyle w:val="`+name+`"/>`)
	})
	if len(applied) != len(headingFormats) {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	// Pandoc默认表格只有横线，统一补齐浅灰网格和单元格留白，Word实际排版仍可编辑。
	tableProperties := regexp.MustCompile(`(?s)<w:tblPr>.*?</w:tblPr>`)
	oldTableBorders := regexp.MustCompile(`(?s)<w:tblBorders>.*?</w:tblBorders>`)
	oldTableMargins := regexp.MustCompile(`(?s)<w:tblCellMar>.*?</w:tblCellMar>`)
	borders := `<w:tblBorders>`
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		borders += `<w:` + edge + ` w:val="single" w:sz="4" w:color="D9D9D9"/>`
	}
	borders += `</w:tblBorders><w:tblCellMar><w:top w:w="100" w:type="dxa"/><w:left w:w="100" w:type="dxa"/><w:bottom w:w="100" w:type="dxa"/><w:right w:w="100" w:type="dxa"/></w:tblCellMar>`
	body = tableProperties.ReplaceAllStringFunc(body, func(properties string) string {
		properties = oldTableMargins.ReplaceAllString(oldTableBorders.ReplaceAllString(properties, ""), "")
		return strings.Replace(properties, `</w:tblPr>`, borders+`</w:tblPr>`, 1)
	})
	section := `<w:sectPr><w:footerReference w:type="default" r:id="rIdExportFooter"/><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`
	if !sectionProperties.MatchString(body) {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCX_INVALID")
	}
	files["word/document.xml"] = []byte(sectionProperties.ReplaceAllString(body, section))
	files["word/footer-export.xml"] = []byte(`<?xml version="1.0" encoding="UTF-8"?><w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="20"/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p></w:ftr>`)
	files["word/_rels/document.xml.rels"] = []byte(strings.Replace(string(files["word/_rels/document.xml.rels"]), `</Relationships>`, `<Relationship Id="rIdExportFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer-export.xml"/></Relationships>`, 1))
	files["[Content_Types].xml"] = []byte(strings.Replace(string(files["[Content_Types].xml"]), `</Types>`, `<Override PartName="/word/footer-export.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/></Types>`, 1))
	// 转换时间/机器用户名不进入导出包；固定版本时间已显示在正文。
````

## 第 10 步：styleDOCX（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 201 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>ok</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>output.Len() &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_ARTIFACT_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return os.WriteFile(path, output.Bytes(), 0600)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=9402:10317 -->
````go
	if core, ok := files["docProps/core.xml"]; ok {
		private := regexp.MustCompile(`(?s)<(?:dc:creator|cp:lastModifiedBy)\b[^>]*>.*?</(?:dc:creator|cp:lastModifiedBy)>`)
		files["docProps/core.xml"] = private.ReplaceAll(core, nil)
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(document.Captured.UTC())
		entry, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err = entry.Write(files[name]); err != nil {
			return err
		}
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if output.Len() > 32<<20 {
		return domain.Violation(domain.StateConflict, "EXPORT_ARTIFACT_LIMIT")
	}
	return os.WriteFile(path, output.Bytes(), 0600)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/docx.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
