package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

func TestRetainedGovernanceOnlyClosesOrphanedPublicContent(t *testing.T) {
	release, kbID := "9", "3"
	base := TrashFacts{Document: dto.Document{Id: "7", SpaceId: "2", KnowledgeBaseId: &kbID,
		Lifecycle: "PUBLIC_RETAINED", MainLocked: true, ControlRevision: "4", AclRevision: "5", PublicationEpoch: "6",
		PublicationState: "PUBLISHED", CurrentReleaseId: &release, VisibilityPolicy: "PUBLIC"}}
	space := dto.Space{Id: "2", Lifecycle: "PUBLIC_RETAINED"}
	kb := dto.KnowledgeBase{Id: "3", SpaceId: "2", Lifecycle: "PUBLIC_RETAINED", VisibilityPolicy: "PUBLIC"}
	// 覆盖批次验收的核心反例：平台身份不能代管ACTIVE正文、恢复负责人，
	// 不能越过已变修订或把受限旧正式版当成孤立公开内容。
	for _, scenario := range []struct {
		name   string
		change func(*TrashFacts, *dto.Space, *dto.KnowledgeBase)
	}{
		{"普通公开文档", func(f *TrashFacts, _ *dto.Space, _ *dto.KnowledgeBase) { f.Document.Lifecycle = "ACTIVE" }},
		{"仍有负责人", func(f *TrashFacts, _ *dto.Space, _ *dto.KnowledgeBase) {
			owner := "12"
			f.Document.ResponsibleUserId = &owner
		}},
		{"存续空间", func(_ *TrashFacts, s *dto.Space, _ *dto.KnowledgeBase) { s.Lifecycle = "ACTIVE" }},
		{"已变修订", func(f *TrashFacts, _ *dto.Space, _ *dto.KnowledgeBase) { f.Document.ControlRevision = "5" }},
		{"原库范围受限", func(_ *TrashFacts, _ *dto.Space, k *dto.KnowledgeBase) { k.VisibilityPolicy = "COLLABORATORS" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f, s, k := base, space, kb
			scenario.change(&f, &s, &k)
			if err := ValidateRetainedGovernance(f, s, &k, "UNPUBLISH", "4"); err == nil {
				t.Fatal("不满足孤立留存资格却允许平台治理")
			}
		})
	}
	if err := ValidateRetainedGovernance(base, space, &kb, "UNPUBLISH", "4"); err != nil {
		t.Fatal(err)
	}
	base.Document.PublicationState, base.Document.CurrentReleaseId = "UNPUBLISHED", nil
	if err := ValidateRetainedGovernance(base, space, &kb, "UNPUBLISH", "4"); err == nil {
		t.Fatal("已撤下对象不能用新号制造再次撤下成功")
	}
	if err := ValidateRetainedGovernance(base, space, &kb, "PURGE", "4"); err != nil {
		t.Fatal("真实已撤下孤立留存仍须可永久删除", err)
	}
}
