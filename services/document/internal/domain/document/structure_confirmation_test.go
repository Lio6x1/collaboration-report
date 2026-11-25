package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

func structureConfirmationFixture(t *testing.T) (dto.StructurePreviewCreate, dto.StructureChange) {
	t.Helper()
	var preview dto.StructurePreviewCreate
	raw := `{"operation_id":"structure-preview-original","expected_structure_id":"2","expected_control_revision":"1","changes":[{"type":"RENAME","chapter_id":"3","title":"原预览标题"},{"type":"SPLIT_LEVEL2","chapter_id":"3"},{"type":"REMOVE","chapter_id":"4"}]}`
	if err := json.Unmarshal([]byte(raw), &preview); err != nil {
		t.Fatal(err)
	}
	var confirm dto.StructureChange
	raw = `{"operation_id":"structure-confirm-original","expected_structure_id":"2","expected_control_revision":"1","changes":[{"type":"RENAME","chapter_id":"3","title":"原预览标题"},{"type":"SPLIT_LEVEL2","chapter_id":"3","preview_cut_ref":"structure-preview-original"},{"type":"REMOVE","chapter_id":"4","preview_cut_ref":"structure-preview-original"}]}`
	if err := json.Unmarshal([]byte(raw), &confirm); err != nil {
		t.Fatal(err)
	}
	return preview, confirm
}

func TestStructureConfirmationRequiresTheWholeOriginalIntent(t *testing.T) {
	preview, confirm := structureConfirmationFixture(t)
	if err := ValidateStructureConfirmation(confirm, preview); err != nil {
		t.Fatal(err)
	}
	// 确认引用只允许标识同一原预览；没有引用、不同预览拼接、改动作/顺序/标题均不能重新计算。
	for _, item := range []struct {
		name string
		edit func(*dto.StructureChange)
	}{
		{"missing reference", func(c *dto.StructureChange) { c.Changes[1].PreviewCutRef = nil }},
		{"mixed references", func(c *dto.StructureChange) { ref := "another-preview-original"; c.Changes[2].PreviewCutRef = &ref }},
		{"wrong original", func(c *dto.StructureChange) {
			ref := "another-preview-original"
			c.Changes[1].PreviewCutRef, c.Changes[2].PreviewCutRef = &ref, &ref
		}},
		{"same operation", func(c *dto.StructureChange) { c.OperationId = preview.OperationId }},
		{"structure", func(c *dto.StructureChange) { c.ExpectedStructureId = "99" }},
		{"control", func(c *dto.StructureChange) { c.ExpectedControlRevision = "2" }},
		{"action", func(c *dto.StructureChange) { c.Changes[1].Type = "JOIN_CHAPTER" }},
		{"chapter", func(c *dto.StructureChange) { c.Changes[2].ChapterId = "99" }},
		{"title", func(c *dto.StructureChange) { title := "确认时另换标题"; c.Changes[0].Title = &title }},
		{"order", func(c *dto.StructureChange) { c.Changes[1], c.Changes[2] = c.Changes[2], c.Changes[1] }},
		{"stable action reference", func(c *dto.StructureChange) { ref := preview.OperationId; c.Changes[0].PreviewCutRef = &ref }},
		{"missing action", func(c *dto.StructureChange) { c.Changes = c.Changes[:2] }},
	} {
		t.Run(item.name, func(t *testing.T) {
			original, changed := structureConfirmationFixture(t)
			item.edit(&changed)
			if ValidateStructureConfirmation(changed, original) == nil {
				t.Fatal("different confirmation accepted")
			}
		})
	}
}
