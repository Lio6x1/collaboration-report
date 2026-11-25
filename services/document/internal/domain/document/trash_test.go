package document

import (
	dto "collaboration/contracts/httpdto"
	"math"
	"testing"
	"time"
)

func TestTrashManagementUsesCurrentAuthority(t *testing.T) {
	// 直接MANAGE授权不得让被回收文档重新取得管理入口。当前负责人必须由
	// 调用方用真实成员proof及已锁定Doc负责关系核验，不能采用回收前缓存。
	if CanManageTrash(PermissionFacts{Grants: []GrantFact{{User: 7, Kind: "DOCUMENT", Capability: "MANAGE"}}, Actor: 7}) {
		t.Fatal("old direct document authority revived")
	}
	for _, facts := range []PermissionFacts{{Responsible: true}, {PersonalOwner: true}, {OrganizationAdmin: true}, {KnowledgeBaseAdmin: true}} {
		if !CanManageTrash(facts) {
			t.Fatal("current parent administrator denied")
		}
	}
}

func TestTrashRestoreReplayBindsOriginalActorKindAndTarget(t *testing.T) {
	// 恢复后ACTIVE/新control不能挡原号固定结果；这里只批准原号回放准备，
	// 不生成成功结果，异内容的fingerprint仍由标准Mutation在原tx核验。
	receipt := TrashRestoreReceipt{Operation: "restore_original_0001", Actor: 7, Kind: "DOCUMENT_RESTORE", State: "SUCCEEDED", Path: "/v1/documents/9/restore"}
	if err := ValidateTrashRestoreReceipt(receipt, "restore_original_0001", 7, 9); err != nil {
		t.Fatal(err)
	}
	for _, args := range []struct {
		operation  string
		actor, doc uint64
	}{{"restore_other_id_0001", 7, 9}, {"restore_original_0001", 8, 9}, {"restore_original_0001", 7, 10}} {
		if ValidateTrashRestoreReceipt(receipt, args.operation, args.actor, args.doc) == nil {
			t.Fatal("other original binding replayed")
		}
	}
	wrongKind, pending := receipt, receipt
	wrongKind.Kind = "DOCUMENT_DELETE"
	pending.State = "PREPARING"
	for _, changed := range []TrashRestoreReceipt{wrongKind, pending} {
		if ValidateTrashRestoreReceipt(changed, receipt.Operation, 7, 9) == nil {
			t.Fatal("different action/nonterminal accepted as restored")
		}
	}
}

func TestTrashRestoreRejectsClosedParentsExpiredAndExhausted(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)
	facts := TrashFacts{Document: dto.Document{Id: "9", SpaceId: "3", Lifecycle: "TRASHED", ControlRevision: "4", AclRevision: "5", PublicationEpoch: "6", VisibilityRevision: "7", CurrentStructureId: "11", CurrentStyleId: "12", CurrentManifestId: "13"}, Sequence: 8, PurgeAfter: &until, Now: now}
	space := dto.Space{Id: "3", Lifecycle: "ACTIVE"}
	if err := ValidateTrashRestore(facts, space, nil, "4"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTrashRestore(facts, space, nil, "0"); err == nil || err.Error() != "CONTROL_REVISION_MISMATCH" {
		t.Fatalf("stale canonical revision: %v", err)
	}
	for _, state := range []string{"ACTIVE", "CLOSING", "PUBLIC_RETAINED", "PURGED"} {
		changed := facts
		changed.Document.Lifecycle = state
		if ValidateTrashRestore(changed, space, nil, "4") == nil {
			t.Fatalf("restored %s", state)
		}
	}
	closedKB := dto.KnowledgeBase{Id: "2", SpaceId: "3", Lifecycle: "CLOSED"}
	kbID := "2"
	facts.Document.KnowledgeBaseId = &kbID
	if err := ValidateTrashRestore(facts, space, &closedKB, "4"); err == nil || err.Error() != "KB_NOT_ACTIVE" {
		t.Fatalf("closed KB: %v", err)
	}
	facts.Document.KnowledgeBaseId = nil
	expired, unknown := facts, facts
	expired.PurgeAfter = &now
	unknown.PurgeAfter = nil
	for _, changed := range []TrashFacts{expired, unknown} {
		if ValidateTrashRestore(changed, space, nil, "4") == nil {
			t.Fatal("unknown/expired retention restored")
		}
	}
	space.Lifecycle = "PUBLIC_RETAINED"
	if ValidateTrashRestore(facts, space, nil, "4") == nil {
		t.Fatal("retained space restored")
	}
	// 高水位比原文档counter更大时，恢复也只能继续向前；完整uint64不得转float。
	next, err := PlanTrashRestore(facts, math.MaxUint64-1, 20)
	if err != nil || next.ACL != math.MaxUint64 || next.Publication != 21 || next.Control != 5 || next.Visibility != 8 || next.Sequence != 9 {
		t.Fatalf("monotonic revisions: %+v %v", next, err)
	}
	if _, err := PlanTrashRestore(facts, math.MaxUint64, 20); err == nil {
		t.Fatal("closed high water overflow revived")
	}
}
