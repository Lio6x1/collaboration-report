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

type node struct {
	T string `json:"t"`
	C any    `json:"c,omitempty"`
}

func n(kind string, value any) node { return node{T: kind, C: value} }
func attr(id string) []any          { return []any{id, []string{}, [][2]string{}} }

type converter struct {
	assets map[string]string
	unit   string
}

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
	return result, nil
}

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

func stringValue(value any) string { v, _ := value.(string); return v }
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

// Word书签由本单元和块身份的摘要生成，固定ASCII可避免Word截断长ID；不同单元可复用块ID。
func paragraphBookmark(unit, id string) string {
	digest := sha256.Sum256([]byte(unit + "\n" + id))
	return "exportformat" + hex.EncodeToString(digest[:12])
}

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
