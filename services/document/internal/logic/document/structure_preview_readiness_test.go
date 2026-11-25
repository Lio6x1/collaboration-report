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

func pendingStructurePreviewFixture(t *testing.T) (structurePreviewContext, repository.PendingStructurePreviewBinding) {
	t.Helper()
	var input dto.StructurePreviewCreate
	if err := json.Unmarshal([]byte(`{"operation_id":"structure-preview-original","expected_structure_id":"5","expected_control_revision":"2","changes":[{"type":"RENAME","chapter_id":"9","title":"固定章名"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	intent, err := domain.ParseStructurePreview(input)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/documents/3/structure-previews"
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		t.Fatal(err)
	}
	target := &pb.Target{DocumentId: 3, StructureId: 5, WorkspaceEpoch: 2, Purpose: "STRUCTURE_PREVIEW"}
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 1}, {ScopeKey: "doc-acl:3", Epoch: 1}}
	proof := &pb.IdentityAdmission{AdmissionId: "original-identity-proof", ActorId: 7, SessionRef: make([]byte, 32), Sources: []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 1}}, ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Purpose: "STRUCTURE_PREVIEW", Target: target, OperationId: input.OperationId, Capability: pb.IdentityCapability_ACTIVE_ACCOUNT, IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 7}}
	fixed := structurePreviewContext{Actor: 7, Document: 3, SessionExpires: time.Now().Add(time.Hour).UnixMilli(), Intent: intent, Manifest: 10, Structure: 5, Style: 11, Control: 2,
		Access:  DocumentAccess{Document: dto.Document{Id: "3", CurrentManifestId: "10", CurrentStructureId: "5", CurrentStyleId: "11", ControlRevision: "2"}, Permissions: domain.Permissions{Manage: true}, Sources: sources, Proofs: []*pb.IdentityAdmission{proof}},
		Context: &pb.OperationContext{OperationId: input.OperationId, ActorId: 7, Target: target, Sources: sources, IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "original-document-proof", RequestFingerprint: fingerprint[:]},
		Units:   []structurePreviewUnit{{Unit: 4, Chapter: 9, Room: 12, Workspace: domain.EditWorkspace{Workspace: 6, Epoch: 1, Lineage: 13, BaseManifest: 10, BaseVersion: 14}}}}
	raw, _ := json.Marshal(fixed)
	targetJSON, _ := json.Marshal(map[string]string{"path": path})
	return fixed, repository.PendingStructurePreviewBinding{Operation: input.OperationId, State: "PREPARING", Actor: 7, Original: raw, Payload: raw, Fingerprint: fingerprint[:], Target: targetJSON}
}

func TestPendingStructurePreviewRetainsExpiredProofAndRejectsChangedPersistentBinding(t *testing.T) {
	_, valid := pendingStructurePreviewFixture(t)
	if _, err := validatePendingStructurePreviewBinding(valid); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		edit func(*structurePreviewContext)
	}{
		{"missing session", func(f *structurePreviewContext) { f.Access.Proofs[0].SessionRef = nil }},
		{"missing identity source", func(f *structurePreviewContext) { f.Access.Proofs[0].Sources = nil }},
		{"identity source epoch", func(f *structurePreviewContext) { f.Access.Proofs[0].Sources[0].Epoch = 9 }},
		{"structure", func(f *structurePreviewContext) { f.Context.Target.StructureId++ }},
		{"unit scope", func(f *structurePreviewContext) { f.Context.Target.UnitId = 4 }},
		{"actor", func(f *structurePreviewContext) { f.Actor++ }},
		{"title in both payloads", func(f *structurePreviewContext) { title := "篡改标题"; f.Intent.Input.Changes[0].Title = &title }},
		{"missing branch", func(f *structurePreviewContext) { f.Units[0].Workspace.Workspace = 0 }},
		{"missing original admission", func(f *structurePreviewContext) { f.Context.DocumentAdmissionRef = "" }},
	} {
		t.Run(item.name, func(t *testing.T) {
			fixed, fact := pendingStructurePreviewFixture(t)
			item.edit(&fixed)
			raw, _ := json.Marshal(fixed)
			fact.Original, fact.Payload = raw, raw
			if _, err := validatePendingStructurePreviewBinding(fact); err == nil {
				t.Fatal("changed persistent binding reported ready")
			}
		})
	}
}

func TestStructurePreviewAdmissionRejectsMissingOriginalProofWithoutPanic(t *testing.T) {
	fixed := structurePreviewContext{Actor: 3, Document: 5, SessionExpires: 100, Context: &pb.OperationContext{IdentityAdmissionRef: "proof", OperationId: "structure-preview-original"}, Access: DocumentAccess{Proofs: []*pb.IdentityAdmission{nil}}}
	actual := repository.OriginalAdmissionBinding{Actor: 3, Document: 5, Identity: "proof", Operation: "structure-preview-original", Purpose: "STRUCTURE_PREVIEW", SessionExpires: 100}
	// 私有持久元数据缺失时必须保持UNKNOWN/503，不能用panic终止worker和整个服务。
	if validateStructurePreviewAdmission(fixed, actual) == nil {
		t.Fatal("missing original proof accepted")
	}
}

func TestStructurePreviewAdmissionBindsActualPersistentHeader(t *testing.T) {
	fixed, _ := pendingStructurePreviewFixture(t)
	target, _ := json.Marshal(fixed.Context.Target)
	actual := repository.OriginalAdmissionBinding{Actor: fixed.Actor, Document: fixed.Document, Identity: fixed.Context.IdentityAdmissionRef,
		Operation: fixed.Context.OperationId, Purpose: "STRUCTURE_PREVIEW", Target: target, Session: fixed.Access.Proofs[0].SessionRef, SessionExpires: fixed.SessionExpires}
	if err := validateStructurePreviewAdmission(fixed, actual); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		edit func(*repository.OriginalAdmissionBinding)
	}{
		{"actor", func(a *repository.OriginalAdmissionBinding) { a.Actor++ }},
		{"document", func(a *repository.OriginalAdmissionBinding) { a.Document++ }},
		{"unit", func(a *repository.OriginalAdmissionBinding) { a.Unit = 4 }},
		{"session", func(a *repository.OriginalAdmissionBinding) { a.Session = []byte("other-session") }},
		{"session expiry", func(a *repository.OriginalAdmissionBinding) { a.SessionExpires++ }},
		{"purpose", func(a *repository.OriginalAdmissionBinding) { a.Purpose = "PULL_MAIN" }},
		{"original operation", func(a *repository.OriginalAdmissionBinding) { a.Operation = "another-operation" }},
		{"missing target", func(a *repository.OriginalAdmissionBinding) { a.Target = nil }},
		{"other target", func(a *repository.OriginalAdmissionBinding) {
			a.Target = []byte(`{"document_id":99,"purpose":"STRUCTURE_PREVIEW"}`)
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			changed := actual
			item.edit(&changed)
			if validateStructurePreviewAdmission(fixed, changed) == nil {
				t.Fatal("different actual document admission accepted")
			}
		})
	}
}
