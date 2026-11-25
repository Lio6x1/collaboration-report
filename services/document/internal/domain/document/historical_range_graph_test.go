package document

import "testing"

func ptrBoundary(s string) *string { return &s }

func TestHistoricalRangeRequiresEverySplitDescendant(t *testing.T) {
	edges := []HistoricalRangeEdge{{Source: 1, Target: 2, Kind: "SPLIT_LEVEL2", Order: 0, Start: ptrBoundary("a"), End: ptrBoundary("b"), Heads: []string{"a", "b"}}, {Source: 1, Target: 3, Kind: "SPLIT_LEVEL2", Order: 1, Start: ptrBoundary("b"), Heads: []string{"a", "b"}}}
	if HistoricalRangeCovered(map[uint64]bool{2: true}, 1, map[uint64]bool{2: true, 3: true}, edges) {
		t.Fatal("partial section opened whole old chapter")
	}
	if !HistoricalRangeCovered(map[uint64]bool{2: true, 3: true}, 1, map[uint64]bool{2: true, 3: true}, edges) {
		t.Fatal("complete descendants rejected")
	}
	edges = edges[:1]
	if HistoricalRangeCovered(map[uint64]bool{2: true}, 1, map[uint64]bool{2: true}, edges) {
		t.Fatal("missing range edge proved whole history")
	}
}

func TestHistoricalRangeComposesActualJoinIntervalsWithLaterSplit(t *testing.T) {
	edges := []HistoricalRangeEdge{
		{Source: 1, Target: 2, Kind: "SPLIT_LEVEL2", Order: 0, Start: ptrBoundary("a"), End: ptrBoundary("b"), Heads: []string{"a", "b"}},
		{Source: 1, Target: 3, Kind: "SPLIT_LEVEL2", Order: 1, Start: ptrBoundary("b"), Heads: []string{"a", "b"}},
		{Source: 2, Target: 4, Kind: "JOIN_CHAPTER", TargetInterval: true, TargetStart: ptrBoundary("a"), TargetEnd: ptrBoundary("b")},
		{Source: 3, Target: 4, Kind: "JOIN_CHAPTER", TargetInterval: true, TargetStart: ptrBoundary("b")},
		{Source: 4, Target: 5, Kind: "SPLIT_LEVEL2", Order: 0, Start: ptrBoundary("a"), End: ptrBoundary("b"), Heads: []string{"a", "b"}},
		{Source: 4, Target: 6, Kind: "SPLIT_LEVEL2", Order: 1, Start: ptrBoundary("b"), Heads: []string{"a", "b"}},
	}
	current := map[uint64]bool{5: true, 6: true}
	partial := map[uint64]bool{5: true}
	if !HistoricalRangeCovered(partial, 2, current, edges) {
		t.Fatal("first original section lost its actual complete range")
	}
	if HistoricalRangeCovered(partial, 1, current, edges) {
		t.Fatal("first section widened to whole original chapter")
	}
	if HistoricalRangeCovered(partial, 3, current, edges) {
		t.Fatal("first section widened to second original section")
	}
	if !HistoricalRangeCovered(map[uint64]bool{5: true, 6: true}, 1, current, edges) {
		t.Fatal("whole range failed after split-join-split")
	}
	edges[2].TargetInterval = false
	if HistoricalRangeCovered(partial, 2, current, edges) {
		t.Fatal("missing target interval guessed a partial authorization")
	}
}

func TestHistoricalRangeRejectsCyclesUnknownAndAmbiguousScope(t *testing.T) {
	for _, edges := range [][]HistoricalRangeEdge{
		{{Source: 1, Target: 2, Kind: "JOIN_CHAPTER"}, {Source: 2, Target: 1, Kind: "JOIN_CHAPTER"}},
		{{Source: 1, Target: 1, Kind: "JOIN_CHAPTER"}},
		{{Source: 1, Target: 2, Kind: "UNKNOWN"}},
		{{Source: 1, Target: 2, Kind: "SPLIT_LEVEL2", Heads: []string{"a", "a"}}},
	} {
		if HistoricalRangeCovered(map[uint64]bool{2: true}, 1, map[uint64]bool{}, edges) {
			t.Fatal("unproven scope allowed")
		}
	}
	if HistoricalRangeCovered(nil, 1, nil, nil) {
		t.Fatal("empty graph proved empty whole history")
	}
}
