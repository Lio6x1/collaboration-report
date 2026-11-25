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

func pendingPullFixture(t *testing.T) (pullContext, repository.PendingPullBinding) {
	t.Helper()
	target := &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 5, StructureId: 6, WorkspaceEpoch: 1, Purpose: "PULL_MAIN"}
	input := dto.PullMain{OperationId: "original-pull-operation", ExpectedBranchId: "5", ExpectedLineageId: "11", ExpectedStructureId: "6"}
	path := "/v1/documents/3/units/4/pull-main"
	fp, err := domain.OperationFingerprint(path, input)
	if err != nil {
		t.Fatal(err)
	}
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 1}}
	proof := &pb.IdentityAdmission{AdmissionId: "original-identity-proof", ActorId: 7, SessionRef: make([]byte, 32), Sources: sources, ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Purpose: "PULL_MAIN", Target: target, OperationId: input.OperationId}
	fixed := pullContext{Actor: 7, SessionExpires: time.Now().Add(time.Hour).UnixMilli(), Document: 3, Unit: 4, Room: 9, Input: input, Workspace: domain.EditWorkspace{Workspace: 5, Epoch: 1, Lineage: 11, BaseManifest: 10, BaseVersion: 12}, Manifest: 13, Structure: 6, Style: 14, Control: 2,
		Access:  DocumentAccess{Permissions: domain.Permissions{Edit: map[uint64]bool{4: true}}, Sources: sources, Proofs: []*pb.IdentityAdmission{proof}},
		Context: &pb.OperationContext{OperationId: input.OperationId, ActorId: 7, Target: target, Sources: sources, IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "original-document-admission", RequestFingerprint: fp[:]}}
	raw, _ := json.Marshal(fixed)
	targetRaw, _ := json.Marshal(map[string]string{"path": path})
	gate, _ := json.Marshal(map[string]string{"operation_id": input.OperationId})
	admissionTarget, _ := json.Marshal(target)
	return fixed, repository.PendingPullBinding{Kind: "PULL_MAIN_RECOVER", Operation: input.OperationId, State: "PREPARING", Actor: 7, Epoch: 1, Lineage: 11, Lifecycle: "ACTIVE", Original: raw, Payload: raw, Fingerprint: fp[:], Target: targetRaw, Gate: gate,
		Admission: repository.OriginalAdmissionBinding{Actor: 7, Document: 3, Unit: 4, Identity: proof.AdmissionId, Operation: input.OperationId, Purpose: "PULL_MAIN", Target: admissionTarget, Session: proof.SessionRef, SessionExpires: fixed.SessionExpires}}
}

func TestPendingPullRejectsAnotherOriginalAdmissionActor(t *testing.T) {
	_, fact := pendingPullFixture(t)
	if err := validatePendingPullBinding(fact); err != nil {
		t.Fatal(err)
	}
	fact.Admission.Actor = 8
	if err := validatePendingPullBinding(fact); err == nil {
		t.Fatal("another actor admission reported recoverable")
	}
}

func TestPendingPullRejectsMissingOriginalDocumentAdmission(t *testing.T) {
	fixed, fact := pendingPullFixture(t)
	// 过期的短期证明仍可原号续接，不能把它误判成损坏；准入引用缺失则无法证明所属Doc原事务。
	if err := validatePendingPullBinding(fact); err != nil {
		t.Fatal(err)
	}
	fixed.Context.DocumentAdmissionRef = ""
	raw, _ := json.Marshal(fixed)
	fact.Original, fact.Payload = raw, raw
	if err := validatePendingPullBinding(fact); err == nil {
		t.Fatal("missing document admission reported recoverable")
	}
}
