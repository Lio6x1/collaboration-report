package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

func TestStructureTransitionAllocatesNewUnitsAndRetainsChapterIdentity(t *testing.T) {
	old := []dto.StructureEntry{{NodeId: "1", ChapterId: "1", Kind: "CHAPTER", Title: "原章", SortOrder: 1, UnitId: strptr("2")}, {NodeId: "3", ChapterId: "3", Kind: "CHAPTER", Title: "另一章", SortOrder: 2, UnitId: strptr("4")}}
	var input dto.StructureChange
	if err := json.Unmarshal([]byte(`{"operation_id":"confirm-original","expected_structure_id":"1","expected_control_revision":"1","changes":[{"type":"SPLIT_LEVEL2","chapter_id":"1","preview_cut_ref":"preview-original"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	split := StructureMigration{Changes: []StructureMigrationChange{{Type: "SPLIT_LEVEL2", Chapter: "1", Sources: []string{"2"}, Targets: []StructureMigrationTarget{{Key: "first", Kind: "SECTION", Title: "一"}, {Key: "second", Kind: "SECTION", Title: "二"}}}}}
	allocated := map[string]StructureMigrationIdentity{"first": {Unit: "10", Node: "20"}, "second": {Unit: "11", Node: "21"}}
	entries, err := ApplyStructureMigration(old, input, split, allocated)
	if err != nil || len(entries) != 4 || entries[0].NodeId != "1" || entries[0].UnitId != nil || !entries[0].SplitLevel2 || *entries[1].UnitId != "10" || *entries[2].ParentNodeId != "1" || *old[0].UnitId != "2" {
		t.Fatalf("split: %+v %v", entries, err)
	}
	input.Changes[0].Type = "JOIN_CHAPTER"
	join := StructureMigration{Changes: []StructureMigrationChange{{Type: "JOIN_CHAPTER", Chapter: "1", Sources: []string{"10", "11"}, Targets: []StructureMigrationTarget{{Key: "joined", Kind: "CHAPTER"}}}}}
	entries, err = ApplyStructureMigration(entries, input, join, map[string]StructureMigrationIdentity{"joined": {Unit: "12", Node: "22"}})
	if err != nil || len(entries) != 2 || *entries[0].UnitId != "12" || entries[0].SplitLevel2 || entries[0].Title != "原章" {
		t.Fatalf("join: %+v %v", entries, err)
	}
	input.Changes[0].Type = "REMOVE"
	remove := StructureMigration{Changes: []StructureMigrationChange{{Type: "REMOVE", Chapter: "1", Sources: []string{"12"}}}}
	entries, err = ApplyStructureMigration(entries, input, remove, nil)
	if err != nil || len(entries) != 1 || entries[0].ChapterId != "3" || entries[0].SortOrder != 1 {
		t.Fatalf("remove: %+v %v", entries, err)
	}
}

func strptr(s string) *string { return &s }
