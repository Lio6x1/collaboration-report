package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
	"time"
)

func pendingDiscardDraftFixture(t *testing.T) (discardDraftContext, repository.PendingDiscardDraftBinding) {
	t.Helper()
	preview, _ := pendingDiscardPreviewFixture(t)
	input := dto.DiscardDraft{OperationId: "discard-confirm-original", ExpectedBranchId: "6", ExpectedStructureId: "5", PreviewCutRef: preview.Intent.Operation}
	intent, err := domain.ParseDiscardDraft(input)
	if err != nil {
		t.Fatal(err)
	}
	var access DocumentAccess
	copied, _ := json.Marshal(preview.Access)
	if err = json.Unmarshal(copied, &access); err != nil {
		t.Fatal(err)
	}
	target := &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 6, StructureId: 5, WorkspaceEpoch: 1, Purpose: "DISCARD_DRAFT"}
	path := "/v1/documents/3/units/4/discard-draft"
	fingerprint, _ := domain.OperationFingerprint(path, input)
	for _, proof := range access.Proofs {
		proof.OperationId, proof.Purpose, proof.Target = input.OperationId, "DISCARD_DRAFT", target
	}
	fixed := discardDraftContext{Actor: preview.Actor, SessionExpires: preview.SessionExpires, Intent: intent, Scope: preview.Scope, Access: access, Preview: preview, PreviewDigest: make([]byte, 32), PreviewResult: dto.DiscardPreview{PreviewCutRef: preview.Intent.Operation, DocumentId: "3", UnitId: "4", BranchId: "6", WorkspaceEpoch: "1", LineageId: domain.TextID(preview.Scope.Workspace.Lineage), StructureId: "5", MainManifestId: "10", ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)}}
	fixed.Context = &pb.OperationContext{OperationId: input.OperationId, ActorId: preview.Actor, Target: target, Sources: access.Sources, IdentityAdmissionRef: access.Proofs[0].AdmissionId, DocumentAdmissionRef: "discard-original-doc-admission", RequestFingerprint: fingerprint[:]}
	raw, _ := json.Marshal(fixed)
	rawTarget, _ := json.Marshal(map[string]string{"path": path})
	return fixed, repository.PendingDiscardDraftBinding{Operation: intent.Operation, Kind: "DISCARD_DRAFT_RECOVER", State: "PREPARING", Actor: fixed.Actor, Original: raw, Payload: raw, Target: rawTarget, Fingerprint: fingerprint[:], Owner: intent.Operation}
}

func TestDiscardReadinessRequiresIndependentOriginalConfirmation(t *testing.T) {
	_, valid := pendingDiscardDraftFixture(t)
	if _, err := validatePendingDiscardDraftBinding(valid); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*discardDraftContext){
		func(f *discardDraftContext) { f.Context.Target.Purpose = "PULL_MAIN" },
		func(f *discardDraftContext) { f.Scope.Unit++ },
		func(f *discardDraftContext) { f.Intent.Input.PreviewCutRef = "another-discard-preview" },
		func(f *discardDraftContext) { f.Access.Proofs[0].SessionRef = nil },
		func(f *discardDraftContext) { f.Access.Proofs[0].Sources[0].Epoch++ },
		func(f *discardDraftContext) { f.PreviewDigest = nil },
	} {
		fixed, fact := pendingDiscardDraftFixture(t)
		edit(&fixed)
		raw, _ := json.Marshal(fixed)
		fact.Original, fact.Payload = raw, raw
		if _, err := validatePendingDiscardDraftBinding(fact); err == nil {
			t.Fatal("changed confirmation reported ready")
		}
	}
	valid.Owner = "another-operation-original"
	if _, err := validatePendingDiscardDraftBinding(valid); err == nil {
		t.Fatal("another branch owner accepted")
	}
}

func TestDiscardReadinessAcceptsDatabaseJSONFieldOrder(t *testing.T) {
	fixed, fact := pendingDiscardDraftFixture(t)
	var fields map[string]any
	if err := json.Unmarshal(fact.Original, &fields); err != nil {
		t.Fatal(err)
	}
	fact.Original, _ = json.Marshal(fields)
	install := discardDraftInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, NewRoom: 19, Epoch: fixed.Scope.Workspace.Epoch + 1, Lineage: 20, Content: &pb.FixedContent{OperationId: fixed.Intent.Operation}}}
	fact.Payload, _ = json.Marshal(install)
	fact.Committed = fact.Payload
	fact.Kind, fact.State = "DISCARD_DRAFT_RECONCILE", "SUCCEEDED"
	if _, err := validatePendingDiscardDraftBinding(fact); err != nil {
		t.Fatal(err)
	}
}
