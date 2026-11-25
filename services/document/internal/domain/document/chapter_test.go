package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

func TestChapterInsertKeepsSplitChildrenAndSiblingOrder(t *testing.T) {
	unit := "11"
	parent := "3"
	after := "3"
	entries := []dto.StructureEntry{{NodeId: "3", ChapterId: "3", Kind: "CHAPTER", Title: "旧章", SortOrder: 1, SplitLevel2: true},
		{NodeId: "4", ChapterId: "3", ParentNodeId: &parent, UnitId: &unit, Kind: "SECTION", Title: "子节", SortOrder: 1},
		{NodeId: "5", ChapterId: "5", Kind: "CHAPTER", Title: "后一章", SortOrder: 2}}
	input := dto.ChapterCreate{OperationId: "create-chapter-original", ExpectedStructureId: "2", Title: "新章", AfterChapterId: &after}
	result, err := InsertChapterEntry(entries, input, "7", "13")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 || result[0].SortOrder != 1 || result[1].ParentNodeId == nil || result[1].SortOrder != 1 || result[2].NodeId != "7" || result[2].SortOrder != 2 || result[3].SortOrder != 3 {
		t.Fatalf("wrong structure %#v", result)
	}
	if entries[2].SortOrder != 2 {
		t.Fatal("old immutable structure mutated")
	}
	unknown := "999"
	input.AfterChapterId = &unknown
	if _, err = InsertChapterEntry(entries, input, "7", "13"); err == nil {
		t.Fatal("unknown sibling accepted")
	}
}
