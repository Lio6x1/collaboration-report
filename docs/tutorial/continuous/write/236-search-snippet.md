# 分步编写 `services/document/internal/logic/document/search_snippet.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/236-search-snippet.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从已验证content-v1正文提取有界纯文本搜索摘要。

摘要投影不应读取MAIN、ES或provenance，也不应该复制另一套schema校验逻辑。

写之前先按这条执行路线理解各段的连接：

1. searchSnippet创建snippetBuffer
2. visit按节点递归取可见文本
3. append控制输出预算。

本文件需要守住的边界：

- 摘要成功不表示获得读取权限
- 调用方先验证固定release、资源、hash和正文预算。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/search_snippet.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/search_snippet.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/search_snippet.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/search_snippet.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:78 -->
````go
package document

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

````

## 第 2 步：searchSnippetCharacters

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`const searchSnippetCharacters = 2000`。

<!-- write-step: 2 bytes=78:116 -->
````go
const searchSnippetCharacters = 2000

````

## 第 3 步：errSearchSnippetInvalid

插入位置：第 2 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`var errSearchSnippetInvalid = errors.New("SEARCH_SNIPPET_INVALID")`。

<!-- write-step: 3 bytes=116:184 -->
````go
var errSearchSnippetInvalid = errors.New("SEARCH_SNIPPET_INVALID")

````

## 第 4 步：searchSnippet

插入位置：第 3 步代码之后，紧接追加；完成稿约第 14 行。

searchSnippet只投影已由共享content-v1校验的单元正文，不读取MAIN、ES或provenance。
release/资源绑定、完整schema/hash及正文预算由调用方先验证；这里不复制整套validator。
摘要成功不代表阅读授权，当前权限及固定正式版事实仍由整页用例复核。

本步声明：`func searchSnippet(validatedBody any) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validatedBody</code> | <code>any</code> | 在此声明中的实际使用：<code>root, ok := validatedBody.(map&#91;string&#93;any)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>strings.TrimSpace(output.text.String())</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errSearchSnippetInvalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok &#124;&#124; root&#91;&quot;type&quot;&#93; != &quot;doc&quot;</code>。条件成立时执行 <code>return &quot;&quot;, errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 此处通过 <code>return strings.TrimSpace(output.text.String()), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=184:929 -->
````go
// searchSnippet只投影已由共享content-v1校验的单元正文，不读取MAIN、ES或provenance。
// release/资源绑定、完整schema/hash及正文预算由调用方先验证；这里不复制整套validator。
// 摘要成功不代表阅读授权，当前权限及固定正式版事实仍由整页用例复核。
func searchSnippet(validatedBody any) (string, error) {
	root, ok := validatedBody.(map[string]any)
	if !ok || root["type"] != "doc" {
		return "", errSearchSnippetInvalid
	}
	var output snippetBuffer
	if err := output.visit(root, 0); err != nil {
		// 即使合法前缀已装满，后续坏节点也不能变成一个成功的截断摘要。
		return "", err
	}
	return strings.TrimSpace(output.text.String()), nil
}

````

## 第 5 步：snippetBuffer

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type snippetBuffer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>text</code> | <code>strings.Builder</code> | 逐步构造结果摘要的字符串缓冲，避免反复拼接分配。 |
| <code>characters</code> | <code>int</code> | 当前已写入的字符计数；摘要上限按字符而非UTF-8字节截断。 |

<!-- write-step: 5 bytes=929:1004 -->
````go
type snippetBuffer struct {
	text       strings.Builder
	characters int
}

````

## 第 6 步：snippetBuffer.append

插入位置：第 5 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func (s *snippetBuffer) append(value string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>utf8.ValidString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errSearchSnippetInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!utf8.ValidString(value)</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>s.characters == searchSnippetCharacters</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>s.characters == 0 &amp;&amp; unicode.IsSpace(character)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1004:1444 -->
````go
func (s *snippetBuffer) append(value string) error {
	// Go字符串允许非法UTF-8；禁止range静默把损坏字节改成替换字符。
	if !utf8.ValidString(value) {
		return errSearchSnippetInvalid
	}
	for _, character := range value {
		if s.characters == searchSnippetCharacters {
			break
		}
		if s.characters == 0 && unicode.IsSpace(character) {
			continue
		}
		s.text.WriteRune(character)
		s.characters++
	}
	return nil
}

````

## 第 7 步：snippetBuffer.visit

插入位置：第 6 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func (s *snippetBuffer) visit(value any, depth int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>node, ok := value.(map&#91;string&#93;any)</code> |
| <code>depth</code> | <code>int</code> | 在此声明中的实际使用：<code>depth &gt; 128</code>；<code>depth+1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errSearchSnippetInvalid</code>；<code>s.append(text)</code>；<code>err</code>；<code>s.append(&quot;\n&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>depth &gt; 128</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; text == &quot;&quot;</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return s.append(text)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>kind == &quot;formula_block&quot;</code>。条件成立时执行 <code>return s.append(&quot;\n&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.append(&quot; &quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=1444:2715 -->
````go
func (s *snippetBuffer) visit(value any, depth int) error {
	// 正常树先经共享Decode的128层限制；此保护也让错误调用的循环树有界失败。
	if depth > 128 {
		return errSearchSnippetInvalid
	}
	node, ok := value.(map[string]any)
	if !ok {
		return errSearchSnippetInvalid
	}
	kind, ok := node["type"].(string)
	if !ok {
		return errSearchSnippetInvalid
	}
	switch kind {
	case "text":
		text, ok := node["text"].(string)
		if !ok || text == "" {
			return errSearchSnippetInvalid
		}
		return s.append(text)
	case "formula", "formula_block":
		attrs, ok := node["attrs"].(map[string]any)
		if !ok {
			return errSearchSnippetInvalid
		}
		latex, ok := attrs["latex"].(string)
		if !ok {
			return errSearchSnippetInvalid
		}
		// 沿Search固定文本语义加原子间隔，不格式化公式中的数字或读取atomLink。
		s.append(" ")
		if err := s.append(latex); err != nil {
			return err
		}
		if kind == "formula_block" {
			return s.append("\n")
		}
		return s.append(" ")
	case "image":
		attrs, ok := node["attrs"].(map[string]any)
		if !ok {
			return errSearchSnippetInvalid
		}
		// 白名单仅取用户说明；resourceId、链接目标及尺寸等属性都不进入输出。
		for _, field := range []string{"alt", "title"} {
````

## 第 8 步：snippetBuffer.visit（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 98 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!exists</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>value == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>!ok</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return s.append(&quot;\n&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!exists</code>。
- 检查 <code>!optional</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return s.append(&quot;\n&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok &#124;&#124; (!optional &amp;&amp; len(items) == 0)</code>。条件成立时执行 <code>return errSearchSnippetInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.append(&quot;\n&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return errSearchSnippetInvalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2715:3952 -->
````go
			value, exists := attrs[field]
			if !exists {
				return errSearchSnippetInvalid
			}
			if value == nil {
				continue
			}
			description, ok := value.(string)
			if !ok {
				return errSearchSnippetInvalid
			}
			s.append(" ")
			if err := s.append(description); err != nil {
				return err
			}
		}
		return nil
	case "hard_break", "horizontal_rule":
		return s.append("\n")
	case "doc", "paragraph", "heading", "code_block", "blockquote", "bullet_list", "ordered_list", "list_item", "table", "table_row", "table_cell", "table_header":
		children, exists := node["content"]
		optional := kind == "paragraph" || kind == "heading" || kind == "code_block"
		if !exists {
			if !optional {
				return errSearchSnippetInvalid
			}
			return s.append("\n")
		}
		items, ok := children.([]any)
		if !ok || (!optional && len(items) == 0) {
			return errSearchSnippetInvalid
		}
		for _, child := range items {
			if err := s.visit(child, depth+1); err != nil {
				return err
			}
		}
		// 保留段落、列表和表格的阅读分隔，不能把相邻块黏成新的词。
		return s.append("\n")
	default:
		// 新节点需明确选择可见语义，不能把未知结构当成合法空文本。
		return errSearchSnippetInvalid
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/search_snippet.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
