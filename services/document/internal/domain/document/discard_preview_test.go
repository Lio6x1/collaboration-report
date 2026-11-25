package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

func TestDiscardPreviewIntentRequiresOriginalBranchAndStructure(t *testing.T) {
	input := dto.DiscardPreviewCreate{OperationId: "discard-preview-original", ExpectedBranchId: "3", ExpectedStructureId: "4"}
	intent, err := ParseDiscardPreview(input)
	if err != nil || intent.Operation != input.OperationId || intent.Branch != 3 || intent.Structure != 4 {
		t.Fatal("valid fixed preview intent rejected", err)
	}
	for _, changed := range []dto.DiscardPreviewCreate{
		{OperationId: "bad", ExpectedBranchId: "3", ExpectedStructureId: "4"},
		{OperationId: input.OperationId, ExpectedBranchId: "0", ExpectedStructureId: "4"},
		{OperationId: input.OperationId, ExpectedBranchId: "03", ExpectedStructureId: "4"},
		{OperationId: input.OperationId, ExpectedBranchId: "3", ExpectedStructureId: "0"},
	} {
		if _, err := ParseDiscardPreview(changed); err == nil {
			t.Fatal("unbound discard preview accepted")
		}
	}
}
