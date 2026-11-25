package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

func structureChangeForTest(t *testing.T, changes string) dto.StructureChange {
	t.Helper()
	var input dto.StructureChange
	if err := json.Unmarshal([]byte(`{"operation_id":"structure-original","expected_structure_id":"2","expected_control_revision":"1","changes":`+changes+`}`), &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestStructureRenameMoveKeepsTheWholeChapterGroup(t *testing.T) {
	parent, unit := "3", "7"
	entries := []dto.StructureEntry{
		{NodeId: "3", ChapterId: "3", Kind: "CHAPTER", Title: "旧标题", SortOrder: 1, SplitLevel2: true},
		{NodeId: "4", ChapterId: "3", ParentNodeId: &parent, UnitId: &unit, Kind: "SECTION", SortOrder: 1},
		{NodeId: "5", ChapterId: "5", Kind: "CHAPTER", SortOrder: 2},
	}
	input := structureChangeForTest(t, `[{"type":"RENAME","chapter_id":"3","title":"新标题"},{"type":"MOVE","chapter_id":"3","after_chapter_id":"5"}]`)
	result, err := ApplyStableStructureChanges(entries, input)
	if err != nil {
		t.Fatal(err)
	}
	// 移动整章及其孩子，仅修改根节点顺序；旧历史快照绝不能被写穿。
	if len(result) != 3 || result[0].NodeId != "5" || result[1].Title != "新标题" || result[2].NodeId != "4" || result[2].SortOrder != 1 {
		t.Fatalf("wrong grouped move: %#v", result)
	}
	if entries[0].Title != "旧标题" || entries[0].SortOrder != 1 {
		t.Fatal("old snapshot mutated")
	}
	for _, changes := range []string{
		`[{"type":"MOVE","chapter_id":"3","after_chapter_id":"3"}]`,
		`[{"type":"MOVE","chapter_id":"3","after_chapter_id":"999"}]`,
		`[{"type":"RENAME","chapter_id":"999","title":"不存在"}]`,
		`[{"type":"RENAME","chapter_id":"3","title":"  "}]`,
	} {
		if _, err = ApplyStableStructureChanges(entries, structureChangeForTest(t, changes)); err == nil {
			t.Fatalf("invalid change accepted: %s", changes)
		}
	}
}
