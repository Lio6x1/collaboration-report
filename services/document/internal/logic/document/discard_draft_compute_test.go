package document

import (
	"encoding/base64"
	"testing"
)

func TestDiscardDerivedContentRejectsMalformedImmutableData(t *testing.T) {
	// 损坏的恢复记录应阻止Ready并保留原任务，不让panic打断整个恢复进程。
	for _, value := range []map[string]any{
		nil,
		{"provenance_json_utf8": 7},
		{"provenance_json_utf8": base64.StdEncoding.EncodeToString([]byte(`[]`))},
		{"provenance_json_utf8": base64.StdEncoding.EncodeToString([]byte(`{"block_sources":{}}`))},
	} {
		if _, err := deriveDiscardContent(value, "discard-preview-original", "discard-confirm-original", ":candidate"); err == nil {
			t.Fatal("malformed original accepted")
		}
	}
}
