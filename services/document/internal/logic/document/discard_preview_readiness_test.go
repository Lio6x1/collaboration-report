package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
)

func pendingDiscardPreviewFixture(t *testing.T) (discardPreviewContext, repository.PendingDiscardPreviewBinding) {
	t.Helper()
	seed, _ := pendingStructurePreviewFixture(t)
	input := dto.DiscardPreviewCreate{OperationId: "discard-preview-original", ExpectedBranchId: "6", ExpectedStructureId: "5"}
	intent, err := domain.ParseDiscardPreview(input)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/documents/3/units/4/discard-previews"
	fingerprint, _ := domain.OperationFingerprint(path, input)
	target := &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 6, StructureId: 5, WorkspaceEpoch: 1, Purpose: "DISCARD_PREVIEW"}
	seed.Access.Document.MainLocked = true
	proof := seed.Access.Proofs[0]
	proof.OperationId, proof.Purpose, proof.Target = input.OperationId, "DISCARD_PREVIEW", target
	fixed := discardPreviewContext{Actor: seed.Actor, SessionExpires: seed.SessionExpires, Intent: intent, Access: seed.Access,
		Scope:   draftSourceScope{Document: 3, Unit: 4, Room: 12, Manifest: 10, Structure: 5, Style: 11, Control: 2, Workspace: seed.Units[0].Workspace},
		Context: &pb.OperationContext{OperationId: input.OperationId, ActorId: 7, Target: target, Sources: seed.Access.Sources, IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "original-document-proof", RequestFingerprint: fingerprint[:]}}
	raw, _ := json.Marshal(fixed)
	targetJSON, _ := json.Marshal(map[string]string{"path": path})
	return fixed, repository.PendingDiscardPreviewBinding{Operation: input.OperationId, State: "PREPARING", Actor: 7, Original: raw, Payload: raw, Fingerprint: fingerprint[:], Target: targetJSON}
}

func TestDiscardPreviewReadinessBindsOriginalUnitWithoutRejectingLockedMAIN(t *testing.T) {
	_, valid := pendingDiscardPreviewFixture(t)
	// 自然过期准入可按原会话续接；主线锁定不妨碍只读取MAIN的丢弃预览。
	if _, err := validatePendingDiscardPreviewBinding(valid); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*discardPreviewContext){
		func(f *discardPreviewContext) { f.Scope.Room = 0 },
		func(f *discardPreviewContext) { f.Context.Target.UnitId++ },
		func(f *discardPreviewContext) { f.Scope.Workspace.Lineage = 0 },
		func(f *discardPreviewContext) { f.Intent.Input.ExpectedBranchId = "999" },
		func(f *discardPreviewContext) { f.Access.Proofs[0].SessionRef = nil },
		func(f *discardPreviewContext) { f.Access.Proofs[0].Sources[0].Epoch++ },
		func(f *discardPreviewContext) { f.Access.Permissions.Manage = false },
	} {
		fixed, fact := pendingDiscardPreviewFixture(t)
		edit(&fixed)
		raw, _ := json.Marshal(fixed)
		fact.Original, fact.Payload = raw, raw
		if _, err := validatePendingDiscardPreviewBinding(fact); err == nil {
			t.Fatal("changed original discard binding reported ready")
		}
	}
}

func TestDiscardPreviewAdmissionRequiresActualOriginalUnitAndSession(t *testing.T) {
	fixed, _ := pendingDiscardPreviewFixture(t)
	target, _ := json.Marshal(fixed.Context.Target)
	actual := repository.OriginalAdmissionBinding{Actor: fixed.Actor, Document: fixed.Scope.Document, Unit: fixed.Scope.Unit, Identity: fixed.Context.IdentityAdmissionRef, Operation: fixed.Intent.Operation, Purpose: "DISCARD_PREVIEW", Target: target, Session: fixed.Access.Proofs[0].SessionRef, SessionExpires: fixed.SessionExpires}
	if err := validateDiscardPreviewAdmission(fixed, actual); err != nil {
		t.Fatal(err)
	}
	actual.Unit = 0
	if validateDiscardPreviewAdmission(fixed, actual) == nil {
		t.Fatal("whole-document admission accepted for original unit preview")
	}
	fixed.Access.Proofs[0] = nil
	if validateDiscardPreviewAdmission(fixed, actual) == nil {
		t.Fatal("missing original proof accepted")
	}
}
