package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

func TestStableUnitRangeProvesFullScopeAcrossAppendOrRename(t *testing.T) {
	unit := "11"
	old := dto.StructureRevision{Id: "2", DocumentId: "1", Entries: []dto.StructureEntry{{NodeId: "3", ChapterId: "3", Kind: "CHAPTER", UnitId: &unit, Title: "旧题", SortOrder: 1}}}
	next := old
	next.Id = "4"
	next.Entries = append([]dto.StructureEntry(nil), old.Entries...)
	next.Entries[0].Title = "新题"
	if !SameStableUnitRange(old, next, unit) {
		t.Fatal("unchanged full unit rejected")
	}
	next.DocumentId = "2"
	if SameStableUnitRange(old, next, unit) {
		t.Fatal("another document reported unchanged")
	}
	next.DocumentId = "1"
	next.Entries[0].SplitLevel2 = true
	if SameStableUnitRange(old, next, unit) {
		t.Fatal("split scope reported unchanged")
	}
	next.Entries[0].SplitLevel2 = false
	next.Entries[0].ChapterId = "99"
	if SameStableUnitRange(old, next, unit) {
		t.Fatal("another chapter reported unchanged")
	}
	next.Entries = nil
	if SameStableUnitRange(old, next, unit) {
		t.Fatal("missing range reported unchanged")
	}
}
