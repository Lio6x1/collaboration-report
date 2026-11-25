package document

import (
	codec "collaboration/contracts/content"
	"slices"
	"testing"
)

func TestResolutionBodyRejectsDuplicateStableBlockIdentities(t *testing.T) {
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	// JSON Schema只能检查单个节点；重复稳定身份会让Node永久拒绝计算，必须在登记控制占位前拒绝。
	for name, raw := range map[string]string{
		"same parent":   `{"content":[{"attrs":{"id":"same"},"content":[{"text":"a","type":"text"}],"type":"paragraph"},{"attrs":{"id":"same"},"content":[{"text":"b","type":"text"}],"type":"paragraph"}],"type":"doc"}`,
		"nested parent": `{"content":[{"attrs":{"id":"same"},"content":[{"text":"a","type":"text"}],"type":"paragraph"},{"attrs":{"id":"quote"},"content":[{"attrs":{"id":"same"},"content":[{"text":"b","type":"text"}],"type":"paragraph"}],"type":"blockquote"}],"type":"doc"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolutionBody(validator, []byte(raw)); err == nil {
				t.Fatal("duplicate identity accepted before durable registration")
			}
		})
	}
}

func TestResolutionBodyPreservesRepeatedInlineResourceReferences(t *testing.T) {
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	image := map[string]any{"type": "image", "attrs": map[string]any{"resourceId": "9", "alt": nil, "title": nil, "width": nil, "height": nil, "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}}
	raw, err := codec.Canonical(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "attrs": map[string]any{"id": "images"}, "content": []any{image, image}}}})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := resolutionBody(validator, raw)
	if err != nil || !slices.Equal(ids, []uint64{9}) {
		t.Fatalf("inline resources changed: %v %v", ids, err)
	}
}
