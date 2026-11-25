package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

func structureMigrationFixture(t *testing.T) (dto.StructurePreviewCreate, []StructureMigrationSource, []byte, *codec.Validator) {
	t.Helper()
	var input dto.StructurePreviewCreate
	if err := json.Unmarshal([]byte(`{"operation_id":"structure-preview-original","expected_structure_id":"2","expected_control_revision":"1","changes":[{"type":"SPLIT_LEVEL2","chapter_id":"3"},{"type":"REMOVE","chapter_id":"6"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"structure-migration-v1","changes":[{"type":"SPLIT_LEVEL2","chapter_id":"3","source_units":["4"],"targets":[{"key":"3:SPLIT:0","kind":"SECTION","title":"第一节","main_body":{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"source-main"},"content":[{"type":"text","text":"主线正文"}]}]},"draft_body":{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"source-S2"},"content":[{"type":"text","text":"后续草稿"}]}]},"boundary":{"heading":{"type":"heading","attrs":{"id":"heading-one","level":2},"content":[{"type":"text","text":"第一节"}]}},"source_ranges":[{"source_unit_id":"4","source_scope":"WHOLE_UNIT","start_boundary":"heading-one","end_boundary":null,"main_block_ids":["heading-one","source-main"],"draft_block_ids":["heading-one","source-S2"]}]}]},{"type":"REMOVE","chapter_id":"6","source_units":["7"],"targets":[]}]}`)
	return input, []StructureMigrationSource{{Unit: 4, Chapter: 3}, {Unit: 7, Chapter: 6}}, raw, validator
}

func TestStructureMigrationRetainsCanonicalCandidatesAndCompleteSourceScope(t *testing.T) {
	input, sources, raw, validator := structureMigrationFixture(t)
	migration, err := ParseStructureMigration(raw, input, sources, validator)
	if err != nil {
		t.Fatal(err)
	}
	if len(migration.Changes) != 2 || len(migration.Changes[0].Targets) != 1 || len(migration.Changes[1].Targets) != 0 {
		t.Fatal("wrong range candidates")
	}
	for _, edit := range []string{
		`{"schema_version":"unknown","changes":[]}`,
		`{"schema_version":"structure-migration-v1","changes":[]}`,
	} {
		if _, err = ParseStructureMigration([]byte(edit), input, sources, validator); err == nil {
			t.Fatal("unknown or empty migration accepted")
		}
	}
	for _, item := range []struct {
		name string
		edit func(*StructureMigration)
	}{
		{"wrong chapter", func(m *StructureMigration) { m.Changes[0].Chapter = "99" }},
		{"wrong action", func(m *StructureMigration) { m.Changes[0].Type = "JOIN_CHAPTER" }},
		{"missing source", func(m *StructureMigration) { m.Changes[0].Sources = nil }},
		{"foreign source", func(m *StructureMigration) { m.Changes[0].Sources[0] = "7" }},
		{"duplicate source", func(m *StructureMigration) { m.Changes[0].Sources = append(m.Changes[0].Sources, "4") }},
		{"missing targets", func(m *StructureMigration) { m.Changes[0].Targets = nil }},
		{"missing range", func(m *StructureMigration) { m.Changes[0].Targets[0].Ranges = nil }},
		{"foreign range", func(m *StructureMigration) { m.Changes[0].Targets[0].Ranges[0].Source = "7" }},
		{"partial declared scope", func(m *StructureMigration) { m.Changes[0].Targets[0].Ranges[0].Scope = "CURRENT_BLOCKS" }},
		{"missing heading", func(m *StructureMigration) { m.Changes[0].Targets[0].Boundary.Heading = nil }},
		{"invalid body", func(m *StructureMigration) {
			m.Changes[0].Targets[0].Draft = json.RawMessage(`{"type":"doc","content":[]}`)
		}},
		{"removal replacement", func(m *StructureMigration) { m.Changes[1].Targets = m.Changes[0].Targets }},
	} {
		t.Run(item.name, func(t *testing.T) {
			_, _, original, _ := structureMigrationFixture(t)
			var changed StructureMigration
			if err := json.Unmarshal(original, &changed); err != nil {
				t.Fatal(err)
			}
			item.edit(&changed)
			wrong, _ := json.Marshal(changed)
			if _, err := ParseStructureMigration(wrong, input, sources, validator); err == nil {
				t.Fatal("invalid complete source scope accepted")
			}
		})
	}
}

func TestJoinedIntervalsCoverActualManagementBoundariesWithInternalHeadings(t *testing.T) {
	a, b := "a", "b"
	target := StructureMigrationTarget{Main: json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"id":"a","level":2}},{"type":"heading","attrs":{"id":"internal","level":2}},{"type":"heading","attrs":{"id":"b","level":2}}]}`),
		Ranges: []StructureMigrationRange{{TargetScope: "INTERVAL", TargetStart: &a, TargetEnd: &b}, {TargetScope: "INTERVAL", TargetStart: &b}}}
	// 合章中新插入的内部标题不会改变原管理节的范围边界，也不能被误当成另一来源节。
	if !validJoinedTargetIntervals(&target) {
		t.Fatal("internal content heading changed original management intervals")
	}
	for _, change := range []func(*StructureMigrationTarget){
		func(v *StructureMigrationTarget) { v.Ranges[0].TargetEnd = nil },
		func(v *StructureMigrationTarget) { v.Ranges[1].TargetStart = &a },
		func(v *StructureMigrationTarget) { v.Ranges[0].TargetScope = "CURRENT_BLOCKS" },
		func(v *StructureMigrationTarget) { v.Ranges[1].TargetScope = "" },
	} {
		copy := target
		copy.Ranges = append([]StructureMigrationRange{}, target.Ranges...)
		change(&copy)
		if validJoinedTargetIntervals(&copy) {
			t.Fatal("ambiguous target intervals accepted")
		}
	}
}
