package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	"testing"
)

func TestBranchBaselineProjectsOnlyMainFontsAndKeepsFormalBytes(t *testing.T) {
	raw := []byte(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"font-baseline"},"content":[{"type":"text","text":"基线","marks":[{"type":"font","attrs":{"family":"SimSun"}},{"type":"strong"}]}]}]}`)
	original := bytes.Clone(raw)
	projected, err := BranchBaselineBody(raw)
	if err != nil {
		t.Fatal(err)
	}
	value, err := codec.Decode(projected)
	if err != nil {
		t.Fatal(err)
	}
	node := value.(map[string]any)["content"].([]any)[0].(map[string]any)
	text := node["content"].([]any)[0].(map[string]any)
	marks := text["marks"].([]any)
	if len(marks) != 1 || marks[0].(map[string]any)["type"] != "strong" || node["attrs"].(map[string]any)["id"] != "font-baseline" || !bytes.Equal(raw, original) {
		t.Fatal("projection changed formal bytes, identity or permitted formatting")
	}
	if _, err = BranchBaselineBody([]byte(`{"type":`)); err == nil {
		t.Fatal("invalid baseline accepted")
	}
}
