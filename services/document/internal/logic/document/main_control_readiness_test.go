package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
)

func pendingMainControlFixture(t *testing.T) repository.PendingMainControlBinding {
	t.Helper()
	input := dto.MainLock{OperationId: "readiness-main-control", ExpectedControlRevision: "2", Locked: false}
	intent, err := domain.ParseMainLock(input)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/3/main-lock", input)
	if err != nil {
		t.Fatal(err)
	}
	target := &pb.Target{DocumentId: 3, StructureId: 11, WorkspaceEpoch: 2, Purpose: "MAIN_CONTROL"}
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:9", Epoch: 2}, {ScopeKey: "doc-acl:3", Epoch: 1}}
	proof := &pb.IdentityAdmission{AdmissionId: "main-control-proof", OperationId: intent.Operation, ActorId: 9, Purpose: "MAIN_CONTROL", Target: target, SessionRef: make([]byte, 32), IdentityTarget: &pb.IdentityTarget{UserId: 9}, Sources: sources[:1]}
	fixed := mainControlContext{Actor: 9, SessionExpires: 1, Document: 3, Generation: 4, Intent: intent, Manifest: 13, Structure: 11, Style: 17, Control: 2,
		Access:  DocumentAccess{Document: dto.Document{Id: "3", CurrentManifestId: "13", CurrentStructureId: "11", CurrentStyleId: "17", ControlRevision: "2"}, Permissions: domain.Permissions{Manage: true}, Sources: sources, Proofs: []*pb.IdentityAdmission{proof}},
		Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: 9, Target: target, Sources: sources, RequestFingerprint: fingerprint[:], IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "main-control-admission"},
	}
	original, _ := json.Marshal(fixed)
	payload, _ := json.Marshal(mainControlRecovery{Fixed: fixed, Attempt: 1})
	return repository.PendingMainControlBinding{Kind: "MAIN_CONTROL_RECOVER", Operation: intent.Operation, ReceiptKind: "MAIN_LOCK", State: "PREPARING", Actor: 9, Original: original, Payload: payload, Fingerprint: fingerprint[:], Target: []byte(`{"path":"/v1/documents/3/main-lock"}`)}
}

// 领取代次仅能增加attempt；后台恢复不能改变原锁定方向、期限、权限快照或固定控制代次。
func TestMainControlReadinessPreservesOriginalIntent(t *testing.T) {
	good := pendingMainControlFixture(t)
	if err := validatePendingMainControlBinding(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*mainControlContext)
	}{
		{"locked", func(f *mainControlContext) { f.Intent.Locked = true }},
		{"session", func(f *mainControlContext) { f.SessionExpires++ }},
		{"generation", func(f *mainControlContext) { f.Generation++ }},
		{"permission", func(f *mainControlContext) { f.Access.Permissions.Manage = false }},
		{"manifest snapshot", func(f *mainControlContext) { f.Access.Document.CurrentManifestId = "99" }},
		{"proof", func(f *mainControlContext) {
			f.Access.Proofs[0].AdmissionId = "replacement-proof"
			f.Context.IdentityAdmissionRef = "replacement-proof"
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			fact := pendingMainControlFixture(t)
			var progress mainControlRecovery
			if err := json.Unmarshal(fact.Payload, &progress); err != nil {
				t.Fatal(err)
			}
			change.edit(&progress.Fixed)
			fact.Payload, _ = json.Marshal(progress)
			if validatePendingMainControlBinding(fact) == nil {
				t.Fatal("changed original binding reported ready")
			}
		})
	}
	// 修改原文与领取载荷仍不能绕过HTTP输入摘要；防止一致篡改两份管理意图。
	var progress mainControlRecovery
	json.Unmarshal(good.Payload, &progress)
	progress.Fixed.Intent.Locked = true
	good.Payload, _ = json.Marshal(progress)
	good.Original, _ = json.Marshal(progress.Fixed)
	if validatePendingMainControlBinding(good) == nil {
		t.Fatal("changed intent with original fingerprint reported ready")
	}
}
