package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

func TestStructurePreviewBindsGeneratedInputAndRejectsPrematureConfirmation(t *testing.T) {
	var input dto.StructurePreviewCreate
	if err := json.Unmarshal([]byte(`{"operation_id":"structure-preview-original","expected_structure_id":"7","expected_control_revision":"2","changes":[{"type":"SPLIT_LEVEL2","chapter_id":"3"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	intent, err := ParseStructurePreview(input)
	if err != nil || intent.Structure != 7 || intent.Control != 2 || intent.Operation != input.OperationId {
		t.Fatalf("wrong fixed preview: %#v %v", intent, err)
	}
	// 创建预览不接纳旧preview_cut_ref，避免预览请求悄悄变成结构确认或复用另一操作。
	ref := "another-preview"
	input.Changes[0].PreviewCutRef = &ref
	if _, err = ParseStructurePreview(input); err == nil {
		t.Fatal("preview accepted another confirmation reference")
	}
	input.Changes[0].PreviewCutRef = nil
	input.ExpectedControlRevision = "02"
	if _, err = ParseStructurePreview(input); err == nil {
		t.Fatal("noncanonical control accepted")
	}
}
