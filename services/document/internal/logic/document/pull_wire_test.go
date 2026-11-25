package document

import (
	"encoding/json"
	"testing"
)

func TestPullFixedContentReadsTheExactOriginalProducerEnvelope(t *testing.T) {
	// 首轮真实预览额外带了PULL_MAIN目标purpose；兼容原固定字节，但其他未知字段或其他purpose不能被忽略。
	for _, item := range []struct {
		raw   string
		valid bool
	}{
		{`{"document_id":"3","purpose":"PULL_MAIN"}`, true},
		{`{"document_id":"3","purpose":"MERGE"}`, false},
		{`{"document_id":"3","unexpected":"ignored"}`, false},
	} {
		_, err := decodePullFixedContent(json.RawMessage(item.raw))
		if (err == nil) != item.valid {
			t.Fatalf("valid=%v error=%v", item.valid, err)
		}
	}
}
