# 分步编写 `services/document/internal/exporter/content_export.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/119-content-export.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把已授权的规范正文树转换成Pandoc原生AST。

保留可编辑表格、公式、图片和格式需要语义节点，不能把内容变成截图或原始HTML绕过验证。

写之前先按这条执行路线理解各段的连接：

1. buildAST按结构排列单元
2. inlines处理文字标记
3. blocks处理段落/标题/列表
4. table保留合并单元格
5. paragraphBookmark用unit与block生成固定ASCII标识。

本文件需要守住的边界：

- 不同单元可重用块ID，书签必须包含unit
- 不支持的节点要明确失败，不能悄悄丢内容。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/exporter' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/exporter/content_export.go') -and (Get-Item -LiteralPath 'services/document/internal/exporter/content_export.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/exporter/content_export.go')) { New-Item -ItemType File -Path 'services/document/internal/exporter/content_export.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:506 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package exporter

import (
	domain "collaboration/document/internal/domain/document"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ast.go：保留原实现，按业务合并文件。
// Package exporter 将已经鉴权并完整校验的固定正文转成成熟转换器的原生AST。
// 不接受任意URL、主机文件或HTML；权限、作业和SQL不属于这个适配层。

````

## 第 2 步：node

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type node struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>T</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;t&quot;`</code> |
| <code>C</code> | <code>any</code> | 序列化/校验标记：<code>`json:&quot;c,omitempty&quot;`</code> |

<!-- write-step: 2 bytes=506:580 -->
````go
type node struct {
	T string `json:"t"`
	C any    `json:"c,omitempty"`
}

````

## 第 3 步：n

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func n(kind string, value any) node`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>T: kind</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>C: value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>node</code> | 实际返回表达式：<code>node{T: kind, C: value}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return node{T: kind, C: value}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=580:651 -->
````go
func n(kind string, value any) node { return node{T: kind, C: value} }
````

## 第 4 步：attr

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func attr(id string) []any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>return &#91;&#93;any{id, &#91;&#93;string{}, &#91;&#93;&#91;2&#93;string{}}</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>&#91;&#93;any{id, &#91;&#93;string{}, &#91;&#93;&#91;2&#93;string{}}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &#91;&#93;any{id, &#91;&#93;string{}, &#91;&#93;&#91;2&#93;string{}}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=651:736 -->
````go
func attr(id string) []any          { return []any{id, []string{}, [][2]string{}} }

````

## 第 5 步：converter

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type converter struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>assets</code> | <code>map&#91;string&#93;string</code> | 已下载且校验的资源编号到本次导出本地资源路径映射，转换不允许自行访问任意URL。 |
| <code>unit</code> | <code>string</code> | 当前正在转换的协作单元编号，用于生成作用域内标识并保持块归属。 |

<!-- write-step: 5 bytes=736:804 -->
````go
type converter struct {
	assets map[string]string
	unit   string
}

````

## 第 6 步：buildAST

插入位置：第 5 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func buildAST(document domain.ExportDocument, assets map[string]string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>domain.ExportDocument</code> | 使用 <code>domain.ExportDocument</code> 类型；其定义处的职责：固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。<br>在此声明中的实际使用：<code>validateDocument(document)</code>；<code>document.Target</code>；<code>document.Manifest</code>；<code>document.Captured</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>assets</code> | <code>map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>assets: assets</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(map&#91;string&#93;any{&quot;pandoc-api-version&quot;: &#91;&#93;int{1, 23, 1}, &quot;meta&quot;: map&#91;string&#93;any{&quot;title&quot;: n(&quot;MetaInlines&quot;, text(document.Title)), &quot;lang&quot;: n(&quot;MetaString&quot;, &quot;zh-CN&quot;)}, &quot;blocks&quot;: blocks})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(map&#91;string&#93;any{&quot;pandoc-api-version&quot;: &#91;&#93;int{1, 23, 1}, &quot;meta&quot;: map&#91;string&#93;any{&quot;title&quot;: n(&quot;MetaInlines&quot;, text(document.Title)), &quot;lang&quot;: n(&quot;MetaString&quot;, &quot;zh-CN&quot;)}, &quot;blocks&quot;: blocks})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_STRUCTURE_INCOMPLETE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>document.Target == &quot;MAIN&quot;</code>。
- 检查 <code>document.Release != &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>entry.ParentNodeId != nil</code>。
- 在本段的匿名函数内：检查 <code>key != parent</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：检查 <code>depth &gt; 6</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_STRUCTURE_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>entry.Title != &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>entry.UnitId != nil</code>。
- 在本段的匿名函数内：检查 <code>seen&#91;id&#93; &#124;&#124; units&#91;id&#93; == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_STRUCTURE_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=804:2671 -->
````go
func buildAST(document domain.ExportDocument, assets map[string]string) ([]byte, error) {
	if err := validateDocument(document); err != nil {
		return nil, err
	}
	c := converter{assets: assets}
	label := "正式发布版"
	if document.Target == "MAIN" {
		label = "主线稿 · 未正式发布"
	}
	blocks := []node{n("Para", text(label+"；固定清单 "+document.Manifest+"；版本时间 "+document.Captured.UTC().Format("2006-01-02 15:04:05 UTC")))}
	if document.Release != "" {
		blocks = append(blocks, n("Para", text("正式版编号 "+document.Release)))
	}
	blocks = append(blocks, n("Para", text("转换说明：正文、表格、图片与公式来自该固定版本；字体以本机安装字体呈现，动画图片采用首帧。Markdown保留字体标记的语义，但阅读器可能采用自身样式。")))
	units := map[string]any{}
	for _, unit := range document.Units {
		units[unit.ID] = unit.Body
	}
	seen := map[string]bool{}
	// 结构原顺序已经由仓库固定。递归按parent关系展开，不能按自增unit编号拼篇。
	var walk func(string, int) error
	walk = func(parent string, depth int) error {
		for _, entry := range document.Structure.Entries {
			key := ""
			if entry.ParentNodeId != nil {
				key = *entry.ParentNodeId
			}
			if key != parent {
				continue
			}
			if depth > 6 {
				return domain.Violation(domain.StateConflict, "EXPORT_STRUCTURE_INVALID")
			}
			if entry.Title != "" {
				blocks = append(blocks, n("Header", []any{depth, attr("chapter-" + entry.NodeId), text(entry.Title)}))
			}
			if entry.UnitId != nil {
				id := *entry.UnitId
				if seen[id] || units[id] == nil {
					return domain.Violation(domain.StateConflict, "EXPORT_STRUCTURE_INVALID")
				}
				root, ok := units[id].(map[string]any)
				if !ok {
					return domain.Violation(domain.StateConflict, "EXPORT_CONTENT_INVALID")
				}
````

## 第 7 步：buildAST（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(seen) != len(units)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_STRUCTURE_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(map&#91;string&#93;any{&quot;pandoc-api-version&quot;: &#91;&#93;int{1, 23, 1}, &quot;meta&quot;: map&#91;string&#93;any{&quot;title&quot;: n(&quot;MetaInlines&quot;, text(document.Title)), &quot;lang&quot;: n(&quot;MetaString&quot;, &quot;zh-CN&quot;)}, &quot;blocks&quot;: blocks})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2671:3308 -->
````go
				c.unit = id
				body, err := c.blocks(root["content"])
				if err != nil {
					return err
				}
				blocks = append(blocks, body...)
				seen[id] = true
			}
			if err := walk(entry.NodeId, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk("", 1); err != nil {
		return nil, err
	}
	if len(seen) != len(units) {
		return nil, domain.Violation(domain.StateConflict, "EXPORT_STRUCTURE_INCOMPLETE")
	}
	return json.Marshal(map[string]any{"pandoc-api-version": []int{1, 23, 1}, "meta": map[string]any{"title": n("MetaInlines", text(document.Title)), "lang": n("MetaString", "zh-CN")}, "blocks": blocks})
}

````

## 第 8 步：text

插入位置：第 7 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func text(value string) []node`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>n(&quot;Str&quot;, value&#91;start:index&#93;)</code>；<code>len(value)</code>；<code>n(&quot;Str&quot;, value&#91;start:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;node</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 检查 <code>r != &#x27; &#x27; &amp;&amp; r != &#x27;\n&#x27; &amp;&amp; r != &#x27;\t&#x27;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>index &gt; start</code>。
- 检查 <code>start &lt; len(value)</code>。
- 此处通过 <code>return result</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3308:3854 -->
````go
func text(value string) []node {
	result := []node{}
	start := 0
	for index, r := range value {
		if r != ' ' && r != '\n' && r != '\t' {
			continue
		}
		if index > start {
			result = append(result, n("Str", value[start:index]))
		}
		switch r {
		case ' ':
			result = append(result, n("Space", nil))
		case '\n':
			result = append(result, n("LineBreak", nil))
		case '\t':
			result = append(result, n("Str", "\t"))
		}
		start = index + 1
	}
	if start < len(value) {
		result = append(result, n("Str", value[start:]))
	}
	return result
}

````

## 第 9 步：converter.inlines

插入位置：第 8 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`func (c converter) inlines(raw any) ([]node, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>any</code> | 在此声明中的实际使用：<code>items, _ := raw.(&#91;&#93;any)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;node</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_UNSUPPORTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_RESOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>width &gt; 0</code>。
- 检查 <code>height &gt; 0</code>。
- 此处通过 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_UNSUPPORTED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>kind == &quot;text&quot;</code>。
- 检查 <code>kind == &quot;formula&quot; &#124;&#124; kind == &quot;image&quot;</code>。

<!-- write-step: 9 bytes=3854:5577 -->
````go
func (c converter) inlines(raw any) ([]node, error) {
	result := []node{}
	items, _ := raw.([]any)
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_INVALID")
		}
		kind, _ := item["type"].(string)
		attrs, _ := item["attrs"].(map[string]any)
		var content []node
		switch kind {
		case "text":
			content = text(stringValue(item["text"]))
		case "hard_break":
			content = []node{n("LineBreak", nil)}
		case "formula":
			content = []node{n("Math", []any{n("InlineMath", nil), stringValue(attrs["latex"])})}
		case "image":
			path, exists := c.assets[stringValue(attrs["resourceId"])]
			if !exists {
				return nil, domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
			}
			properties := [][2]string{}
			if width := integer(attrs["width"]); width > 0 {
				properties = append(properties, [2]string{"width", fmt.Sprintf("%dpx", width)})
			}
			if height := integer(attrs["height"]); height > 0 {
				properties = append(properties, [2]string{"height", fmt.Sprintf("%dpx", height)})
			}
			content = []node{n("Image", []any{[]any{"", []string{}, properties}, text(stringValue(attrs["alt"])), []string{path, stringValue(attrs["title"])}})}
		default:
			return nil, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_UNSUPPORTED")
		}
		if kind == "text" {
			marks, _ := item["marks"].([]any)
			for _, mark := range marks {
				m, _ := mark.(map[string]any)
				a, _ := m["attrs"].(map[string]any)
				content = applyMark(stringValue(m["type"]), a, content)
			}
		} else if kind == "formula" || kind == "image" {
			content = applyAtom(attrs, content)
		}
		result = append(result, content...)
	}
````

## 第 10 步：converter.inlines（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 170 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5577:5600 -->
````go
	return result, nil
}

````

## 第 11 步：applyMark

插入位置：第 10 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func applyMark(kind string, attrs map[string]any, value []node) []node`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 内联格式种类，switch转换为对应Pandoc节点/属性；普通文本、行内代码和代码块的结构不同。 |
| <code>attrs</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>stringValue(attrs&#91;&quot;href&quot;&#93;)</code>；<code>stringValue(attrs&#91;&quot;title&quot;&#93;)</code>；<code>stringValue(attrs&#91;&quot;family&quot;&#93;)</code> |
| <code>value</code> | <code>&#91;&#93;node</code> | 在此声明中的实际使用：<code>n(&quot;Strong&quot;, value)</code>；<code>n(&quot;Emph&quot;, value)</code>；<code>n(&quot;Strikeout&quot;, value)</code>；<code>n(&quot;Span&quot;, &#91;&#93;any{&#91;&#93;any{&quot;&quot;, &#91;&#93;string{&quot;code&quot;}, &#91;&#93;&#91;2&#93;string{{&quot;custom-style&quot;, &quot;ExportCode&quot;}}}, value})</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;node</code> | 实际返回表达式：<code>&#91;&#93;node{n(&quot;Strong&quot;, value)}</code>；<code>&#91;&#93;node{n(&quot;Emph&quot;, value)}</code>；<code>&#91;&#93;node{n(&quot;Strikeout&quot;, value)}</code>；<code>&#91;&#93;node{n(&quot;Span&quot;, &#91;&#93;any{&#91;&#93;any{&quot;&quot;, &#91;&#93;string{&quot;code&quot;}, &#91;&#93;&#91;2&#93;string{{&quot;custom-style&quot;, &quot;ExportCode&quot;}}}, value})}</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return &#91;&#93;node{n(&quot;Strong&quot;, value)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;node{n(&quot;Emph&quot;, value)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;node{n(&quot;Strikeout&quot;, value)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;node{n(&quot;Span&quot;, &#91;&#93;any{&#91;&#93;any{&quot;&quot;, &#91;&#93;string{&quot;code&quot;}, &#91;&#93;&#91;2&#93;string{{&quot;custom-style&quot;, &quot;ExportCode&quot;}}}, value})}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;node{n(&quot;Link&quot;, &#91;&#93;any{attr(&quot;&quot;), value, &#91;&#93;string{stringValue(attrs&#91;&quot;href&quot;&#93;), stringValue(attrs&#91;&quot;title&quot;&#93;)}})}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &#91;&#93;node{n(&quot;Span&quot;, &#91;&#93;any{&#91;&#93;any{&quot;&quot;, &#91;&#93;string{}, &#91;&#93;&#91;2&#93;string{{&quot;custom-style&quot;, &quot;Export&quot; + strings.ReplaceAll(stringValue(attrs&#91;&quot;family&quot;&#93;), &quot; &quot;, &quot;&quot;)}}}, value})}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5600:6301 -->
````go
func applyMark(kind string, attrs map[string]any, value []node) []node {
	switch kind {
	case "strong":
		return []node{n("Strong", value)}
	case "em":
		return []node{n("Emph", value)}
	case "strike":
		return []node{n("Strikeout", value)}
	case "code":
		return []node{n("Span", []any{[]any{"", []string{"code"}, [][2]string{{"custom-style", "ExportCode"}}}, value})}
	case "link":
		return []node{n("Link", []any{attr(""), value, []string{stringValue(attrs["href"]), stringValue(attrs["title"])}})}
	case "font":
		return []node{n("Span", []any{[]any{"", []string{}, [][2]string{{"custom-style", "Export" + strings.ReplaceAll(stringValue(attrs["family"]), " ", "")}}}, value})}
	}
	return value
}

````

## 第 12 步：applyAtom

插入位置：第 11 步代码之后，紧接追加；完成稿约第 191 行。

本步声明：`func applyAtom(attrs map[string]any, value []node) []node`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>attrs</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>enabled, _ := attrs&#91;mark.flag&#93;.(bool)</code>；<code>link, ok := attrs&#91;&quot;atomLink&quot;&#93;.(map&#91;string&#93;any)</code> |
| <code>value</code> | <code>&#91;&#93;node</code> | 在此声明中的实际使用：<code>value = applyMark(mark.kind, nil, value)</code>；<code>applyMark(mark.kind, nil, value)</code>；<code>value = applyMark(&quot;link&quot;, link, value)</code>；<code>applyMark(&quot;link&quot;, link, value)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;node</code> | 实际返回表达式：<code>value</code> |

**跟着本段代码走一遍：**

- 检查 <code>enabled</code>。
- 检查 <code>ok</code>。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6301:6730 -->
````go
func applyAtom(attrs map[string]any, value []node) []node {
	for _, mark := range []struct{ flag, kind string }{{"atomStrong", "strong"}, {"atomEm", "em"}, {"atomCode", "code"}, {"atomStrike", "strike"}} {
		if enabled, _ := attrs[mark.flag].(bool); enabled {
			value = applyMark(mark.kind, nil, value)
		}
	}
	if link, ok := attrs["atomLink"].(map[string]any); ok {
		value = applyMark("link", link, value)
	}
	return value
}

````

## 第 13 步：stringValue

插入位置：第 12 步代码之后，紧接追加；完成稿约第 203 行。

本步声明：`func stringValue(value any) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>v, _ := value.(string)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>v</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return v</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6730:6802 -->
````go
func stringValue(value any) string { v, _ := value.(string); return v }
````

## 第 14 步：integer

插入位置：第 13 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`func integer(value any) int`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>v := value.(type)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>int(v)</code>；<code>v</code>；<code>int(i)</code>；<code>0</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return int(v)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return v</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return int(i)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6802:7043 -->
````go
func integer(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	}
	return 0
}

// blocks.go：保留原实现，按业务合并文件。

````

## 第 15 步：paragraphFormat

插入位置：第 14 步代码之后，紧接追加；完成稿约第 219 行。

同一受控格式同时进入Word样式和HTML CSS，不能仅在编辑器里改变视觉。
未设置格式的旧块沿用既有AST，不为默认值创造新的导出语义。

本步声明：`func paragraphFormat(kind string, attrs map[string]any) (string, string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;heading&quot;</code> |
| <code>attrs</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>stringValue(attrs&#91;&quot;align&quot;&#93;)</code>；<code>indent, _ := attrs&#91;&quot;firstLineIndent&quot;&#93;.(bool)</code>；<code>integer(attrs&#91;&quot;level&quot;&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>name</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>&quot;text-align:&quot; + align + &quot;;text-indent:&quot; + indentCSS</code> |

**跟着本段代码走一遍：**

- 检查 <code>align != &quot;center&quot; &amp;&amp; align != &quot;right&quot;</code>。
- 检查 <code>align == &quot;left&quot; &amp;&amp; !indent</code>。条件成立时执行 <code>return &quot;&quot;, &quot;&quot;</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;heading&quot;</code>。
- 检查 <code>indent</code>。
- 此处通过 <code>return name, &quot;text-align:&quot; + align + &quot;;text-indent:&quot; + indentCSS</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7043:7852 -->
````go
// 同一受控格式同时进入Word样式和HTML CSS，不能仅在编辑器里改变视觉。
// 未设置格式的旧块沿用既有AST，不为默认值创造新的导出语义。
func paragraphFormat(kind string, attrs map[string]any) (string, string) {
	align := stringValue(attrs["align"])
	if align != "center" && align != "right" {
		align = "left"
	}
	indent, _ := attrs["firstLineIndent"].(bool)
	if align == "left" && !indent {
		return "", ""
	}
	name := "ExportParagraph"
	if kind == "heading" {
		name = fmt.Sprintf("ExportHeading%d", integer(attrs["level"]))
	}
	name += map[string]string{"left": "Left", "center": "Center", "right": "Right"}[align]
	indentCSS := "0"
	if indent {
		name += "Indent"
		indentCSS = "2em"
	}
	return name, "text-align:" + align + ";text-indent:" + indentCSS
}

````

## 第 16 步：paragraphBookmark

插入位置：第 15 步代码之后，紧接追加；完成稿约第 243 行。

Word书签由本单元和块身份的摘要生成，固定ASCII可避免Word截断长ID；不同单元可复用块ID。

本步声明：`func paragraphBookmark(unit, id string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>unit</code> | <code>string</code> | 在此声明中的实际使用：<code>unit + &quot;\n&quot;</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>unit + &quot;\n&quot; + id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;exportformat&quot; + hex.EncodeToString(digest&#91;:12&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;exportformat&quot; + hex.EncodeToString(digest&#91;:12&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7852:8135 -->
````go
// Word书签由本单元和块身份的摘要生成，固定ASCII可避免Word截断长ID；不同单元可复用块ID。
func paragraphBookmark(unit, id string) string {
	digest := sha256.Sum256([]byte(unit + "\n" + id))
	return "exportformat" + hex.EncodeToString(digest[:12])
}

````

## 第 17 步：converter.blocks

插入位置：第 16 步代码之后，紧接追加；完成稿约第 249 行。

本步声明：`func (c converter) blocks(raw any) ([]node, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>any</code> | 在此声明中的实际使用：<code>items, _ := raw.(&#91;&#93;any)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;node</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_UNSUPPORTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;heading&quot;</code>。
- 检查 <code>format != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=8135:9509 -->
````go
func (c converter) blocks(raw any) ([]node, error) {
	result := []node{}
	items, _ := raw.([]any)
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_INVALID")
		}
		attrs, _ := item["attrs"].(map[string]any)
		kind := stringValue(item["type"])
		var block node
		switch kind {
		case "paragraph", "heading":
			body, err := c.inlines(item["content"])
			if err != nil {
				return nil, err
			}
			block = n("Para", body)
			format, css := paragraphFormat(kind, attrs)
			if kind == "heading" {
				id := stringValue(attrs["id"])
				if format != "" {
					id = paragraphBookmark(c.unit, id)
				}
				block = n("Header", []any{integer(attrs["level"]), attr(id), body})
			}
			if format != "" {
				properties := [][2]string{{"custom-style", format}, {"style", css}, {"data-paragraph-format", "true"}}
				block = n("Div", []any{[]any{"", []string{}, properties}, []node{block}})
			}
		case "horizontal_rule":
			block = n("HorizontalRule", nil)
		case "blockquote":
			body, err := c.blocks(item["content"])
			if err != nil {
				return nil, err
			}
			block = n("BlockQuote", body)
		case "code_block":
			children, _ := item["content"].([]any)
			code := ""
			for _, v := range children {
				child, _ := v.(map[string]any)
				code += stringValue(child["text"])
			}
````

## 第 18 步：converter.blocks（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 294 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>lang != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;ordered_list&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return nil, domain.Violation(domain.StateConflict, &quot;EXPORT_CONTENT_UNSUPPORTED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9509:10612 -->
````go
			classes := []string{}
			if lang := stringValue(attrs["language"]); lang != "" {
				classes = append(classes, lang)
			}
			block = n("CodeBlock", []any{[]any{stringValue(attrs["id"]), classes, [][2]string{}}, code})
		case "bullet_list", "ordered_list":
			children, _ := item["content"].([]any)
			list := [][]node{}
			for _, v := range children {
				child, _ := v.(map[string]any)
				body, err := c.blocks(child["content"])
				if err != nil {
					return nil, err
				}
				list = append(list, body)
			}
			block = n("BulletList", list)
			if kind == "ordered_list" {
				block = n("OrderedList", []any{[]any{integer(attrs["order"]), n("Decimal", nil), n("Period", nil)}, list})
			}
		case "formula_block":
			block = n("Para", applyAtom(attrs, []node{n("Math", []any{n("DisplayMath", nil), stringValue(attrs["latex"])})}))
		case "table":
			table, err := c.table(item)
			if err != nil {
				return nil, err
			}
			block = table
		default:
			return nil, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_UNSUPPORTED")
		}
		result = append(result, block)
	}
	return result, nil
}

````

## 第 19 步：converter.table

插入位置：第 18 步代码之后，紧接追加；完成稿约第 330 行。

Pandoc原生Table保留合并行列与块内容；不将用户HTML塞进RawBlock或依赖远程渲染。

本步声明：`func (c converter) table(item map[string]any) (node, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>item</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>attrs, _ := item&#91;&quot;attrs&quot;&#93;.(map&#91;string&#93;any)</code>；<code>input, _ := item&#91;&quot;content&quot;&#93;.(&#91;&#93;any)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>node</code> | 实际返回表达式：<code>node{}</code>；<code>n(&quot;Table&quot;, &#91;&#93;any{attr(stringValue(attrs&#91;&quot;id&quot;&#93;)), &#91;&#93;any{nil, &#91;&#93;any{}}, spec, head, &#91;&#93;any{body}, &#91;&#93;any{attr(&quot;&quot;), &#91;&#93;any{}}})</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;EXPORT_TABLE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return node{}, err</code>。当前函数在这里结束。
- 检查 <code>rowspan &lt; 1 &#124;&#124; colspan &lt; 1</code>。条件成立时执行 <code>return node{}, domain.Violation(domain.StateConflict, &quot;EXPORT_TABLE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>width &gt; columns</code>。
- 检查 <code>columns == 0 &#124;&#124; columns &gt; 1000</code>。条件成立时执行 <code>return node{}, domain.Violation(domain.StateConflict, &quot;EXPORT_TABLE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(input) &gt; 0</code>。

<!-- write-step: 19 bytes=10612:12254 -->
````go
// Pandoc原生Table保留合并行列与块内容；不将用户HTML塞进RawBlock或依赖远程渲染。
func (c converter) table(item map[string]any) (node, error) {
	attrs, _ := item["attrs"].(map[string]any)
	input, _ := item["content"].([]any)
	rows := []any{}
	columns := 0
	for _, v := range input {
		row, _ := v.(map[string]any)
		rowAttrs, _ := row["attrs"].(map[string]any)
		children, _ := row["content"].([]any)
		cells := []any{}
		width := 0
		for _, v := range children {
			cell, _ := v.(map[string]any)
			a, _ := cell["attrs"].(map[string]any)
			body, err := c.blocks(cell["content"])
			if err != nil {
				return node{}, err
			}
			rowspan, colspan := integer(a["rowspan"]), integer(a["colspan"])
			if rowspan < 1 || colspan < 1 {
				return node{}, domain.Violation(domain.StateConflict, "EXPORT_TABLE_INVALID")
			}
			width += colspan
			cells = append(cells, []any{attr(stringValue(a["id"])), n("AlignDefault", nil), rowspan, colspan, body})
		}
		if width > columns {
			columns = width
		}
		rows = append(rows, []any{attr(stringValue(rowAttrs["id"])), cells})
	}
	if columns == 0 || columns > 1000 {
		return node{}, domain.Violation(domain.StateConflict, "EXPORT_TABLE_INVALID")
	}
	spec := []any{}
	for i := 0; i < columns; i++ {
		spec = append(spec, []node{n("AlignDefault", nil), n("ColWidthDefault", nil)})
	}
	head := []any{attr(""), []any{}}
	// 首行全为header时进入表头；普通首行保持原正文，不借位置伪造header语义。
	if len(input) > 0 {
		first, _ := input[0].(map[string]any)
		cells, _ := first["content"].([]any)
		header := len(cells) > 0
		for _, v := range cells {
````

## 第 20 步：converter.table（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 375 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>header</code>。
- 此处通过 <code>return n(&quot;Table&quot;, &#91;&#93;any{attr(stringValue(attrs&#91;&quot;id&quot;&#93;)), &#91;&#93;any{nil, &#91;&#93;any{}}, spec, head, &#91;&#93;any{body}, &#91;&#93;any{attr(&quot;&quot;), &#91;&#93;any{}}}), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=12254:12619 -->
````go
			cell, _ := v.(map[string]any)
			header = header && stringValue(cell["type"]) == "table_header"
		}
		if header {
			head = []any{attr(""), []any{rows[0]}}
			rows = rows[1:]
		}
	}
	body := []any{attr(""), 0, []any{}, rows}
	return n("Table", []any{attr(stringValue(attrs["id"])), []any{nil, []any{}}, spec, head, []any{body}, []any{attr(""), []any{}}}), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/exporter/content_export.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
