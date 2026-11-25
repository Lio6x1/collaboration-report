package document

import (
	codec "collaboration/contracts/content"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func snippetText(value string) map[string]any {
	return map[string]any{"type": "text", "text": value}
}

func snippetBlock(kind, id string, children ...any) map[string]any {
	if children == nil {
		children = []any{}
	}
	return map[string]any{"type": kind, "attrs": map[string]any{"id": id}, "content": children}
}

func snippetBody(children ...any) map[string]any {
	return map[string]any{"type": "doc", "content": children}
}

// 合法样本沿共享解码/schema验证，避免用另造的简化树代替固定正文契约。
func validatedSnippetBody(t *testing.T, body any) any {
	t.Helper()
	raw, err := codec.Canonical(body)
	if err != nil {
		t.Fatal(err)
	}
	value, err := codec.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	validator, err := codec.NewValidator()
	if err != nil || validator.Body(value) != nil {
		t.Fatal("invalid content-v1 fixture", err)
	}
	return value
}

func TestSearchSnippetSemanticTextAndPrivateAttributes(t *testing.T) {
	link := snippetText("可见链接")
	link["marks"] = []any{map[string]any{"type": "link", "attrs": map[string]any{"href": "https://secret.invalid/private-target", "title": "私有链接标题"}}}
	atom := func(kind string) map[string]any {
		return map[string]any{"type": kind, "attrs": map[string]any{"atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": map[string]any{"href": "https://secret.invalid/atom-target"}}}
	}
	formula := atom("formula")
	formula["attrs"].(map[string]any)["latex"] = "x^{18446744073709551615}"
	image := atom("image")
	for key, value := range map[string]any{"resourceId": "18446744073709551615", "alt": "图片说明", "title": "图片标题", "width": nil, "height": nil} {
		image["attrs"].(map[string]any)[key] = value
	}
	body := validatedSnippetBody(t, snippetBody(snippetBlock("paragraph", "private-block-id", link, formula, image, map[string]any{"type": "hard_break"}, snippetText("下一行")), snippetBlock("paragraph", "p2", snippetText("下一段"))))
	got, err := searchSnippet(body)
	want := "可见链接 x^{18446744073709551615}  图片说明 图片标题\n下一行\n下一段"
	if err != nil || got != want {
		t.Fatalf("snippet = %q, %v; want %q", got, err, want)
	}
	for _, private := range []string{"secret.invalid", "private-block-id", "私有链接标题", "atom-target"} {
		if strings.Contains(got, private) {
			t.Fatalf("private attribute exposed: %s", private)
		}
	}
}

func TestSearchSnippetAllContentContainers(t *testing.T) {
	heading := snippetBlock("heading", "heading", snippetText("标题"))
	heading["attrs"].(map[string]any)["level"] = json.Number("2")
	code := snippetBlock("code_block", "code", snippetText("代码\n第二行"))
	code["attrs"].(map[string]any)["language"] = nil
	ordered := snippetBlock("ordered_list", "ordered", snippetBlock("list_item", "item1", snippetBlock("paragraph", "p1", snippetText("有序"))))
	ordered["attrs"].(map[string]any)["order"] = json.Number("2147483647")
	bullet := snippetBlock("bullet_list", "bullet", snippetBlock("list_item", "item2", snippetBlock("paragraph", "p2", snippetText("无序"))))
	cell := func(kind, id, text string) any {
		value := snippetBlock(kind, id, snippetBlock("paragraph", id+"-p", snippetText(text)))
		for key, attr := range map[string]any{"colspan": json.Number("1"), "rowspan": json.Number("1"), "colwidth": nil} {
			value["attrs"].(map[string]any)[key] = attr
		}
		return value
	}
	table := snippetBlock("table", "table", snippetBlock("table_row", "row", cell("table_header", "header", "表头"), cell("table_cell", "cell", "单元格")))
	formula := map[string]any{"type": "formula_block", "attrs": map[string]any{"id": "formula", "latex": "a+b", "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}}
	body := validatedSnippetBody(t, snippetBody(heading, code, snippetBlock("blockquote", "quote", snippetBlock("paragraph", "qp", snippetText("引用"))), ordered, bullet, table, formula, map[string]any{"type": "horizontal_rule", "attrs": map[string]any{"id": "hr"}}))
	before, _ := codec.Canonical(body)
	got, err := searchSnippet(body)
	if err != nil || strings.Join(strings.Fields(got), "|") != "标题|代码|第二行|引用|有序|无序|表头|单元格|a+b" {
		t.Fatalf("container text = %q, %v", got, err)
	}
	after, _ := codec.Canonical(body)
	if string(before) != string(after) {
		t.Fatal("inspection mutated fixed body or numeric attributes")
	}
}

func TestSearchSnippetUnicodeLimit(t *testing.T) {
	// 多字节中文/补充平面字符按Unicode字符计数；不能截断UTF-8或以字节数限制。
	for _, text := range []string{strings.Repeat("中😀", 1000), strings.Repeat("中😀", 1001), strings.Repeat("a", 1999) + "😀尾"} {
		body := validatedSnippetBody(t, snippetBody(snippetBlock("paragraph", "unicode", snippetText(text))))
		got, err := searchSnippet(body)
		want := string([]rune(text)[:2000])
		if err != nil || got != want || !utf8.ValidString(got) || utf8.RuneCountInString(got) != 2000 {
			t.Fatalf("Unicode bound mismatch: runes=%d, error=%v", utf8.RuneCountInString(got), err)
		}
	}
}

func TestSearchSnippetLegitimateEmptyText(t *testing.T) {
	image := map[string]any{"type": "image", "attrs": map[string]any{"resourceId": "99", "alt": nil, "title": nil, "width": nil, "height": nil, "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}}
	for _, body := range []any{snippetBody(snippetBlock("paragraph", "empty")), snippetBody(snippetBlock("paragraph", "image", image)), snippetBody(snippetBlock("paragraph", "space", snippetText(" \n\t ")))} {
		got, err := searchSnippet(validatedSnippetBody(t, body))
		if err != nil || got != "" {
			t.Fatalf("legal empty snippet = %q, %v", got, err)
		}
	}
}

func TestSearchSnippetRejectsInvalidSemanticInput(t *testing.T) {
	invalid := map[string]any{
		"nil":                     nil,
		"array root":              []any{},
		"text root":               snippetText("not a document"),
		"missing type":            map[string]any{"content": []any{}},
		"unknown node":            snippetBody(map[string]any{"type": "future_private_atom"}),
		"invalid child":           snippetBody("not a node"),
		"content wrong type":      map[string]any{"type": "doc", "content": "text"},
		"missing doc content":     map[string]any{"type": "doc"},
		"text wrong type":         snippetBody(snippetBlock("paragraph", "p", map[string]any{"type": "text", "text": json.Number("7")})),
		"empty text node":         snippetBody(snippetBlock("paragraph", "p", snippetText(""))),
		"invalid UTF8 text":       snippetBody(snippetBlock("paragraph", "p", snippetText(string([]byte{0xff})))),
		"formula missing attrs":   snippetBody(snippetBlock("paragraph", "p", map[string]any{"type": "formula"})),
		"formula wrong type":      snippetBody(map[string]any{"type": "formula_block", "attrs": map[string]any{"latex": true}}),
		"image wrong description": snippetBody(snippetBlock("paragraph", "p", map[string]any{"type": "image", "attrs": map[string]any{"alt": 7, "title": nil}})),
		"invalid UTF8 formula":    snippetBody(map[string]any{"type": "formula_block", "attrs": map[string]any{"latex": string([]byte{0xff})}}),
		"invalid UTF8 image":      snippetBody(snippetBlock("paragraph", "p", map[string]any{"type": "image", "attrs": map[string]any{"alt": nil, "title": string([]byte{0xff})}})),
	}
	for name, body := range invalid {
		t.Run(name, func(t *testing.T) {
			got, err := searchSnippet(body)
			if err == nil || got != "" {
				t.Fatalf("invalid tree accepted: snippet=%q, error=%v", got, err)
			}
		})
	}
}

func TestSearchSnippetInspectsBeyondBoundedPrefix(t *testing.T) {
	// 缓冲达到上限不等于验证完成，末尾坏节点仍必须拒绝整项。
	body := snippetBody(snippetBlock("paragraph", "p", snippetText(strings.Repeat("a", 3000))), map[string]any{"type": "unknown"})
	if got, err := searchSnippet(body); err == nil || got != "" {
		t.Fatalf("invalid suffix hidden by truncation: %q, %v", got, err)
	}
	// 跳过前导空白不能耗尽可见字符预算，更不能构造一个无界rune副本。
	valid := validatedSnippetBody(t, snippetBody(snippetBlock("paragraph", "spaces", snippetText(strings.Repeat(" ", 100000)+"可见"))))
	if got, err := searchSnippet(valid); err != nil || got != "可见" {
		t.Fatalf("leading whitespace swallowed visible text: %q, %v", got, err)
	}
}
