package document

import (
	"math"
	"testing"
)

func TestSearchBodyBudgetReservesWholeDocumentBeforeRead(t *testing.T) {
	for _, example := range []struct {
		name  string
		used  uint64
		units []uint64
		want  uint64
		valid bool
	}{
		{"first", 0, []uint64{1024, 2048}, 3072, true},
		{"document-exact-limit", 0, []uint64{12 << 20, 12 << 20}, 24 << 20, true},
		{"page-exact-limit", 40 << 20, []uint64{24 << 20}, 64 << 20, true},
		{"lookahead-over-page", 64 << 20, []uint64{1}, 0, false},
		{"document-over-limit", 0, []uint64{24 << 20, 1}, 0, false},
		{"unit-over-limit", 0, []uint64{math.MaxUint64}, 0, false},
		{"sum-cannot-wrap", 0, []uint64{math.MaxUint64, 2}, 0, false},
		{"invalid-page-baseline", math.MaxUint64, []uint64{1}, 0, false},
		{"missing-manifest", 0, nil, 0, false},
		{"zero-body-metadata", 0, []uint64{1, 0}, 0, false},
	} {
		t.Run(example.name, func(t *testing.T) {
			// 全篇元数据先占累计预算，不能读完巨大正文之后才发现lookahead放不下。
			got, err := ReserveSearchBody(example.used, example.units)
			if (err == nil) != example.valid || got != example.want {
				t.Fatal("SEARCH_BODY_BUDGET_INCORRECT", got, err)
			}
		})
	}
}
