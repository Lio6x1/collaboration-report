package document

import "testing"

// 每种上级管理来源与直接角色叠加都应取能力并集，不能因撤销某一直接授权丢失继承权。
func TestEveryManagementCombinationAndDirectCapabilityMatrix(t *testing.T) {
	units := []UnitFact{{ID: 31, Chapter: 9}, {ID: 32, Chapter: 9}, {ID: 33, Chapter: 10}}
	for mask := 1; mask < 16; mask++ {
		p := ResolvePermissions(PermissionFacts{Actor: 7, Responsible: mask&1 != 0, PersonalOwner: mask&2 != 0, OrganizationAdmin: mask&4 != 0, KnowledgeBaseAdmin: mask&8 != 0, Units: units, Grants: []GrantFact{{User: 7, Kind: "UNIT", Scope: 31, Capability: "EDIT"}}})
		if !p.Read || !p.Comment || !p.Review || !p.Manage || len(p.Edit) != 3 {
			t.Fatalf("management combination %d lost inherited capability", mask)
		}
	}
	cases := []struct {
		name                          string
		grants                        []GrantFact
		read, comment, review, manage bool
		edits                         int
	}{
		{name: "none"},
		{"comment", []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "COMMENT"}}, true, true, false, false, 0},
		{"review", []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "REVIEW"}}, true, true, true, false, 0},
		{"unit", []GrantFact{{User: 7, Kind: "UNIT", Scope: 31, Capability: "EDIT"}}, true, false, false, false, 1},
		{"chapter", []GrantFact{{User: 7, Kind: "CHAPTER", Scope: 9, Capability: "EDIT"}}, true, false, false, false, 2},
		{"comment_and_edit", []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "COMMENT"}, {User: 7, Kind: "UNIT", Scope: 31, Capability: "EDIT"}}, true, true, false, false, 1},
		{"review_and_edit", []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "REVIEW"}, {User: 7, Kind: "UNIT", Scope: 31, Capability: "EDIT"}}, true, true, true, false, 1},
		{"manage", []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "MANAGE"}}, true, true, true, true, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ResolvePermissions(PermissionFacts{Actor: 7, Grants: c.grants, Units: units})
			if p.Read != c.read || p.Comment != c.comment || p.Review != c.review || p.Manage != c.manage || len(p.Edit) != c.edits {
				t.Fatalf("capability matrix mismatch: %+v", p)
			}
		})
	}
}

func TestEditorReadsWholeMainButDoesNotGainComments(t *testing.T) {
	p := ResolvePermissions(PermissionFacts{Actor: 7, Grants: []GrantFact{{User: 7, Kind: "UNIT", Scope: 31, Capability: "EDIT"}}, Units: []UnitFact{{ID: 31, Chapter: 9}, {ID: 32, Chapter: 9}}})
	if !p.Read || !p.Edit[31] || p.Edit[32] || p.Comment || p.Review || p.Manage {
		t.Fatal("editor capability expansion")
	}
}
func TestRemovingDirectUnitGrantRetainsInheritedChapterEdit(t *testing.T) {
	p := ResolvePermissions(PermissionFacts{Actor: 7, Grants: []GrantFact{{User: 7, Kind: "CHAPTER", Scope: 9, Capability: "EDIT"}}, Units: []UnitFact{{ID: 31, Chapter: 9}, {ID: 32, Chapter: 9}}})
	if !p.Edit[31] || !p.Edit[32] {
		t.Fatal("remaining inherited source lost")
	}
}
func TestHistoricalRangeRequiresEveryCurrentDescendant(t *testing.T) {
	p := Permissions{Edit: map[uint64]bool{31: true}}
	if CoversHistoricalRange(p, []uint64{31, 32}, true) {
		t.Fatal("section exposes whole chapter draft")
	}
	p.Edit[32] = true
	if !CoversHistoricalRange(p, []uint64{31, 32}, true) {
		t.Fatal("complete source coverage denied")
	}
	if CoversHistoricalRange(p, nil, false) {
		t.Fatal("unknown lineage treated as covered")
	}
}
func TestReviewerCommentsButCannotReadUnsentDraftHistory(t *testing.T) {
	p := ResolvePermissions(PermissionFacts{Actor: 7, Grants: []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "REVIEW"}}})
	if !p.Read || !p.Comment || !p.Review || CoversHistoricalRange(p, []uint64{31}, true) {
		t.Fatal("reviewer sees unsent draft or loses review")
	}
}
func TestManagementAndNotificationTargetsUseTheirActualScope(t *testing.T) {
	p := ResolvePermissions(PermissionFacts{Actor: 7, KnowledgeBaseAdmin: true, Units: []UnitFact{{ID: 31, Chapter: 9}}})
	if !p.Manage || !p.Edit[31] || !p.Comment {
		t.Fatal("KB management not inherited")
	}
	targets := KnowledgeBaseNotificationTargets([]KBRoleFact{{User: 7, Role: "ADMIN", State: "ACTIVE"}, {User: 8, Role: "MEMBER", State: "ACTIVE"}, {User: 9, Role: "ADMIN", State: "CLOSED"}}, []uint64{10})
	if len(targets) != 1 || targets[0] != 7 {
		t.Fatal("notification expanded to organization admin")
	}
}
