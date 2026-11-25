package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

func TestDiscardIntentBindsExplicitChanges(t *testing.T) {
	input := dto.DiscardDraft{OperationId: "discard-confirm-original-01", ExpectedBranchId: "11", ExpectedStructureId: "12", PreviewCutRef: "discard-preview-original-01"}
	if _, err := ParseDiscardDraft(input); err != nil {
		t.Fatal(err)
	}
	// 零轮次与空处理理由不能作为管理员绕过未解除意见的许可。
	zero := "0"
	input.ExpectedRoundId = &zero
	if _, err := ParseDiscardDraft(input); err == nil {
		t.Fatal("zero round accepted")
	}
	input.ExpectedRoundId = nil
	input.ChangesToResolve = append(input.ChangesToResolve, struct {
		Id       string `json:"id"`
		Revision string `json:"revision"`
		Reason   string `json:"reason"`
	}{Id: "19", Revision: "2", Reason: " "})
	if _, err := ParseDiscardDraft(input); err == nil {
		t.Fatal("empty reason accepted")
	}
	input.ChangesToResolve[0].Reason = "已明确保留旧稿并结束此项工作"
	if _, err := ParseDiscardDraft(input); err != nil {
		t.Fatal(err)
	}
	input.ChangesToResolve = append(input.ChangesToResolve, input.ChangesToResolve[0])
	if _, err := ParseDiscardDraft(input); err == nil {
		t.Fatal("duplicate change accepted")
	}
}
