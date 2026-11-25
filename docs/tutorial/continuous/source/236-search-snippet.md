# services/document/internal/logic/document/search_snippet.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/236-search-snippet.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`searchSnippet`、`snippetBuffer`、`append`、`visit`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const searchSnippetCharacters = 2000

var errSearchSnippetInvalid = errors.New("SEARCH_SNIPPET_INVALID")

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

type snippetBuffer struct {
	text       strings.Builder
	characters int
}

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

UTF-8/LF全文SHA-256：`7c126c9ed4ab384e57feb73c81080b03a742b837c5f93ac3c006e6675e539a62`。[参考实现](../../../../services/document/internal/logic/document/search_snippet.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
