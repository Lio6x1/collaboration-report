package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
)

func pendingMergeFixture(t *testing.T) repository.PendingMergeBinding {
	t.Helper()
	target := &pb.Target{DocumentId: 3, UnitId: 5, BranchId: 7, StructureId: 11, WorkspaceEpoch: 2, Purpose: "MERGE"}
	proof := &pb.IdentityAdmission{AdmissionId: "merge-proof", OperationId: "readiness-merge-operation", ActorId: 9, Purpose: "MERGE", Target: target}
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:9", Epoch: 2}, {ScopeKey: "doc-acl:3", Epoch: 1}}
	fingerprint := make([]byte, 32)
	fingerprint[0] = 31
	fixed := mergeContext{
		Actor: 9, SessionExpires: 1,
		Pointer: domain.ReviewPointer{ID: 13, Document: 3, Unit: 5, Branch: 7, Round: 17},
		Intent:  domain.MergeIntent{Operation: proof.OperationId, Round: 17, Version: 19, Resolutions: []domain.MergeResolution{}},
		Access:  DocumentAccess{Document: dto.Document{Id: "3"}, Sources: sources, Proofs: []*pb.IdentityAdmission{proof}},
		Context: &pb.OperationContext{OperationId: proof.OperationId, ActorId: 9, Target: target, Sources: sources, RequestFingerprint: fingerprint, IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "document-admission"},
	}
	raw, err := json.Marshal(fixed)
	if err != nil {
		t.Fatal(err)
	}
	return repository.PendingMergeBinding{
		Kind: "MERGE_RECOVER", Operation: proof.OperationId, ReceiptState: "PREPARING", ReceiptActor: 9,
		Payload: raw, Original: raw, Fingerprint: fingerprint,
		Fact: domain.MergeFact{Actor: 9, Document: 3, Unit: 5, Branch: 7, Request: 13, Round: 17, Version: 19, State: "PREPARING"},
	}
}

// 就绪检查验证可恢复的固定绑定；过期证明仍允许原号进入恢复流程，由工作者续接或拒绝。
func TestMergeReadinessRejectsChangedTargetAndSources(t *testing.T) {
	good := pendingMergeFixture(t)
	if err := validatePendingMergeBinding(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*mergeContext)
	}{
		{"branch", func(f *mergeContext) { f.Context.Target.BranchId++ }},
		{"purpose", func(f *mergeContext) { f.Context.Target.Purpose = "SUBMISSION" }},
		{"source epoch", func(f *mergeContext) { f.Access.Sources[0].Epoch++ }},
		{"actor", func(f *mergeContext) { f.Context.ActorId++ }},
		{"fingerprint", func(f *mergeContext) { f.Context.RequestFingerprint[0]++ }},
		{"proof operation", func(f *mergeContext) { f.Access.Proofs[0].OperationId = "another-operation" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			fact := pendingMergeFixture(t)
			var fixed mergeContext
			if err := json.Unmarshal(fact.Payload, &fixed); err != nil {
				t.Fatal(err)
			}
			change.edit(&fixed)
			fact.Payload, _ = json.Marshal(fixed)
			fact.Original = fact.Payload
			if validatePendingMergeBinding(fact) == nil {
				t.Fatal("changed persistent binding reported ready")
			}
		})
	}
}

// 通知积压可以继续运行，但不能把另一轮或另一人的通知误认为本次已提交合并的后续事实。
func TestMergeReadinessValidatesNotificationFact(t *testing.T) {
	fact := pendingMergeFixture(t)
	fact.Kind, fact.Fact.State, fact.ReceiptState, fact.Record = "MERGE_NOTIFICATION", "SUCCEEDED", "SUCCEEDED", 23
	notice := map[string]string{"event": "APPROVE_AND_MERGE", "document": "3", "request": "13", "round": "17", "record": "23", "actor": "9"}
	fact.Payload, _ = json.Marshal(notice)
	if err := validatePendingMergeBinding(fact); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"actor", "round", "record", "event"} {
		changed := map[string]string{}
		for key, value := range notice {
			changed[key] = value
		}
		changed[field] = "wrong"
		fact.Payload, _ = json.Marshal(changed)
		if validatePendingMergeBinding(fact) == nil {
			t.Fatalf("changed notification %s reported ready", field)
		}
	}
}
