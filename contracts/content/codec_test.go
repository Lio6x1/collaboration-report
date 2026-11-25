package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestCanonicalNodeGoVectors(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	type vector struct {
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		Canonical string          `json:"canonical_utf8"`
		SHA256    string          `json:"sha256"`
		Size      int             `json:"byte_size"`
	}
	var vectors []vector
	// 五个冻结样本原文件不变；新增格式样本单独保存，两个语言使用合并后的同组证据。
	for _, file := range []string{"canonical-vectors.json", "paragraph-formatting-vectors.json"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct {
			Vectors []vector `json:"vectors"`
		}
		if json.Unmarshal(raw, &fixture) != nil {
			t.Fatal("vectors invalid")
		}
		vectors = append(vectors, fixture.Vectors...)
	}
	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			value, err := Decode(vector.Input)
			if err != nil {
				t.Fatal(err)
			}
			if err := validator.Body(value); err != nil {
				t.Fatal(err)
			}
			canonical, err := Canonical(value)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(canonical)
			if string(canonical) != vector.Canonical || len(canonical) != vector.Size || hex.EncodeToString(digest[:]) != vector.SHA256 {
				t.Fatal("Node/Go canonical bytes differ")
			}
		})
	}
}

// Go标准JSON解码会替换非法Unicode；固定正文需要在解码前拒绝，不能悄悄变成另一个正文。
func TestStrictJSONRejectsInvalidIdentityOfBytes(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"text":"\ud800"}`), []byte(`{"text":"\udc00"}`), []byte(`{"text":"\ud800\u0041"}`),
		{'"', 0xff, '"'}, []byte(`{"id":"first","id":"second"}`), []byte(`{"width":2147483648}`),
	} {
		if _, err := Decode(raw); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	for _, raw := range [][]byte{[]byte(`{"text":"\ud83d\ude00"}`), []byte(`{"text":"\\ud800"}`), []byte(`{"text":"�"}`)} {
		if _, err := Decode(raw); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSchemaUsesOriginalContractAndRejectsUnknownFields(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	valid, err := Decode([]byte(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[]}]}`))
	if err != nil || validator.Body(valid) != nil {
		t.Fatal("valid contract rejected")
	}
	valid.(map[string]any)["unexpected"] = true
	if validator.Body(valid) == nil {
		t.Fatal("unknown content field accepted")
	}
	if validator.Provenance(map[string]any{"schema_version": "provenance-v1"}) == nil {
		t.Fatal("incomplete provenance accepted")
	}
}

func TestCanonicalUsesNodeUTF16KeyOrder(t *testing.T) {
	value, err := Decode([]byte(`{"\ue000":1,"😀":2}`))
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := Canonical(value)
	if err != nil || string(bytes) != "{\"😀\":2,\"\ue000\":1}" {
		t.Fatal("key order differs from Node")
	}
}

// 新格式只能用于段落/标题的固定属性；不能以支持CSS为由放开任意attrs或字符串。
func TestParagraphFormattingContractRemainsStrict(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, kind, attrs string
		valid             bool
	}{
		{"old-paragraph", "paragraph", `"id":"p"`, true},
		{"center-indent", "paragraph", `"id":"p","align":"center","firstLineIndent":true`, true},
		{"right-heading", "heading", `"id":"p","level":2,"align":"right","firstLineIndent":true`, true},
		{"explicit-defaults", "paragraph", `"id":"p","align":"left","firstLineIndent":false`, true},
		{"invalid-align", "paragraph", `"id":"p","align":"justify"`, false},
		{"css-injection", "paragraph", `"id":"p","align":"center;color:red"`, false},
		{"invalid-indent", "paragraph", `"id":"p","firstLineIndent":2`, false},
		{"unknown-attribute", "paragraph", `"id":"p","unknown":true`, false},
		{"unsupported-block", "code_block", `"id":"p","language":null,"align":"center"`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := Decode([]byte(`{"type":"doc","content":[{"type":"` + test.kind + `","attrs":{` + test.attrs + `},"content":[]}]} `))
			if err != nil {
				t.Fatal(err)
			}
			if valid := validator.Body(body) == nil; valid != test.valid {
				t.Fatalf("accepted=%v, want %v", valid, test.valid)
			}
		})
	}
}

// 约束扩展入口本身，避免后续把任意schema补丁误当成已确认的段落格式功能。
func TestParagraphFormattingExtensionCannotReplaceFrozenRules(t *testing.T) {
	read := func(name string) any {
		t.Helper()
		raw, err := schemas.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		value, err := Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	baseline := read("content.schema.json")
	extension := read("paragraph-formatting.extension.json")
	before, err := Canonical(baseline)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeParagraphFormattingExtension(baseline, extension)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Canonical(baseline)
	if err != nil || string(before) != string(after) {
		t.Fatal("extension mutated the frozen schema tree")
	}
	if _, err := mergeParagraphFormattingExtension(merged, extension); err == nil {
		t.Fatal("extension replaced an existing paragraph attribute")
	}
	for _, mutate := range []func(map[string]any){
		func(value map[string]any) { value["nodes"] = []any{"paragraph", "code_block"} },
		func(value map[string]any) { value["required"] = []any{"align"} },
		func(value map[string]any) {
			value["attributes"].(map[string]any)["id"] = map[string]any{"type": "string"}
		},
		func(value map[string]any) {
			value["attributes"].(map[string]any)["align"] = map[string]any{"type": "string"}
		},
	} {
		invalid := read("paragraph-formatting.extension.json").(map[string]any)
		mutate(invalid)
		if _, err := mergeParagraphFormattingExtension(baseline, invalid); err == nil {
			t.Fatal("unapproved paragraph schema extension accepted")
		}
	}
}
