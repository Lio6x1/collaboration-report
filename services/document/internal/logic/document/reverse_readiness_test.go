package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
)

func pendingReverseFixture(t *testing.T) (reverseContext, repository.PendingReverseBinding) {
	t.Helper()
	base, _ := pendingPullFixture(t)
	input := dto.ReverseChange{OperationId: "original-reverse-operation", ExpectedManifestId: "13", MergeRecordId: "20", Reason: "创建新审核保留旧事实"}
	intent, err := domain.ParseReverseChange(input)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/documents/3/reverse-changes"
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		t.Fatal(err)
	}
	base.Context.OperationId, base.Context.Target.Purpose, base.Context.RequestFingerprint = input.OperationId, "REVERSE", fingerprint[:]
	base.Access.Proofs[0].OperationId, base.Access.Proofs[0].Purpose = input.OperationId, "REVERSE"
	base.Access.Permissions.Manage = true
	base.Access.Document = dto.Document{Id: "3", CurrentManifestId: "13", CurrentStructureId: "6", CurrentStyleId: "14", ControlRevision: "2"}
	fixed := reverseContext{Actor: base.Actor, SessionExpires: base.SessionExpires, Intent: intent, Access: base.Access, Context: base.Context, GuardGeneration: 2,
		Scope:  draftSourceScope{Document: 3, Unit: 4, Room: 9, Manifest: 13, Structure: 6, Style: 14, Control: 2, Workspace: base.Workspace},
		Record: repository.ReverseMergeRecord{ID: 20, Document: 3, Unit: 4, Branch: 5, Request: 21, Round: 22, Attempt: 23, InputManifest: 10, OutputManifest: 13, Version: 24, Lineage: 11, Structure: 6, Operation: "old-merged-operation", Capture: "old-submitted-operation", State: "SUCCEEDED", Digest: make([]byte, 32)},
		Before: &pb.FixedContent{DocumentId: 3, UnitId: 4, StructureId: 6, OperationId: "old-merged-operation", BodyHash: make([]byte, 32)},
		After:  &pb.FixedContent{DocumentId: 3, UnitId: 4, StructureId: 6, OperationId: "old-merged-operation", BodyHash: make([]byte, 32)}}
	raw, _ := json.Marshal(fixed)
	target, _ := json.Marshal(map[string]string{"path": path})
	return fixed, repository.PendingReverseBinding{Operation: input.OperationId, Kind: "REVERSE_RECOVER", State: "PREPARING", Owner: input.OperationId, Actor: 7, Original: raw, Payload: raw, Target: target, Fingerprint: fingerprint[:], SourceActor: 7, SourceDocument: 3, SourceUnit: 4, SourceBranch: 5, SourceLineage: 11, SourceGeneration: 2, SourceState: "PREPARING", HeldOperation: input.OperationId, HeldGeneration: 2}
}

func TestReverseReadinessRejectsSubstitutedOriginalBinding(t *testing.T) {
	_, fact := pendingReverseFixture(t)
	// 原短证明已自然到期仍是可续接绑定；这里检查归属，实际worker会续接并核验本次期限。
	if _, err := validatePendingReverseBinding(fact); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*reverseContext, *repository.PendingReverseBinding){
		"actor":          func(_ *reverseContext, f *repository.PendingReverseBinding) { f.Actor++ },
		"owner":          func(_ *reverseContext, f *repository.PendingReverseBinding) { f.Owner = "other-operation" },
		"target purpose": func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Context.Target.Purpose = "SUBMISSION" },
		"proof actor":    func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Access.Proofs[0].ActorId++ },
		"proof session":  func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Access.Proofs[0].SessionRef = nil },
		"original unit":  func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Record.Unit++ },
		"original MAIN":  func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Before.OperationId = "another-merge" },
		"source digest":  func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Record.Digest = nil },
		"reason":         func(f *reverseContext, _ *repository.PendingReverseBinding) { f.Intent.Reason = "偷换理由" },
	} {
		t.Run(name, func(t *testing.T) {
			fixed, fact := pendingReverseFixture(t)
			mutate(&fixed, &fact)
			fact.Original, _ = json.Marshal(fixed)
			fact.Payload = fact.Original
			if _, err := validatePendingReverseBinding(fact); err == nil {
				t.Fatal("substituted original binding accepted")
			}
		})
	}
}
