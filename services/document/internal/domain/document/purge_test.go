package document

import (
	dto "collaboration/contracts/httpdto"
	"math"
	"testing"
	"time"
)

func TestPurgeRequiresConfirmationCurrentTrashAndMonotonicSources(t *testing.T) {
	f := TrashFacts{Document: dto.Document{Id: "9", SpaceId: "3", Lifecycle: "TRASHED", ControlRevision: "4", AclRevision: "5", PublicationEpoch: "6", VisibilityRevision: "7"}, Sequence: 8}
	if ValidatePurgeIntent(f, false, "4", false) == nil {
		t.Fatal("unconfirmed deletion accepted")
	}
	if err := ValidatePurgeIntent(f, true, "4", false); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"ACTIVE", "CLOSING", "PURGED", "PUBLIC_RETAINED"} {
		changed := f
		changed.Document.Lifecycle = state
		if ValidatePurgeIntent(changed, true, "4", false) == nil {
			t.Fatalf("ordinary purge accepted %s", state)
		}
	}
	now := time.Now().UTC()
	unexpired := now.Add(time.Hour)
	f.Now, f.PurgeAfter = now, &unexpired
	if ValidatePurgeIntent(f, true, "4", true) == nil {
		t.Fatal("automatic purge before deadline")
	}
	expired := now.Add(-time.Second)
	f.PurgeAfter = &expired
	if err := ValidatePurgeIntent(f, true, "4", true); err != nil {
		t.Fatal(err)
	}
	p, err := NewPurgePlan("purge_original_0001", 7, false, f, 0, 10, 20)
	if err != nil || p.ACL != 11 || p.Publication != 21 || p.ClosingControl != 5 || !p.Valid() {
		t.Fatalf("fixed plan: %+v %v", p, err)
	}
	if _, err := NewPurgePlan(p.Operation, 7, false, f, 0, math.MaxUint64, 20); err == nil {
		t.Fatal("closed high water wrapped")
	}
	copy := f
	copy.Now = f.Now.Add(time.Hour)
	if PurgeExpiryOperation(f) != PurgeExpiryOperation(copy) || !ValidOperationID(PurgeExpiryOperation(f)) {
		t.Fatal("expiry original ID not stable")
	}
	copy.Document.ControlRevision = "5"
	if PurgeExpiryOperation(f) == PurgeExpiryOperation(copy) {
		t.Fatal("later trash cycle reused old expiry original")
	}
	// 关闭KB禁止恢复，但永久清理仍必须接受；真实性由事务内原关闭ACK核验。
	for _, lifecycle := range []string{"ACTIVE", "CLOSED"} {
		if err := ValidatePurgeParent(dto.KnowledgeBase{Lifecycle: lifecycle}); err != nil {
			t.Fatal(err)
		}
	}
	for _, lifecycle := range []string{"CLOSING", "PUBLIC_RETAINED", "unknown"} {
		if ValidatePurgeParent(dto.KnowledgeBase{Lifecycle: lifecycle}) == nil {
			t.Fatalf("unqualified parent accepted: %s", lifecycle)
		}
	}
	changed := p
	changed.Scopes = append([]PurgeScope(nil), p.Scopes...)
	changed.Scopes[0].Operation = "other_revocation_0001"
	if changed.Valid() {
		t.Fatal("different closure accepted for original purge")
	}
}
