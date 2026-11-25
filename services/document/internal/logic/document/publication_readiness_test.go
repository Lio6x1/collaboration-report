package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
)

func pendingPublicationFixture(t *testing.T) repository.PendingPublicationBinding {
	t.Helper()
	intent := domain.PublicationIntent{Operation: "publication-readiness-operation", Kind: "RELEASE_CREATE", Publication: 7, Control: 9}
	path := "/v1/documents/3/releases"
	fingerprint, err := domain.OperationFingerprint(path, publicationInput(intent))
	if err != nil {
		t.Fatal(err)
	}
	target := &pb.Target{DocumentId: 3, StructureId: 11, WorkspaceEpoch: 9, Purpose: "PUBLICATION"}
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:5", Epoch: 2}, {ScopeKey: "doc-acl:3", Epoch: 4}}
	proof := &pb.IdentityAdmission{AdmissionId: "publication-original-proof", OperationId: intent.Operation, ActorId: 5, Purpose: "PUBLICATION", Target: target,
		SessionRef: make([]byte, 32), IdentityTarget: &pb.IdentityTarget{UserId: 5}, Sources: sources[:1], ExpiresUnixMs: 2000000000000}
	fixed := publicationContext{Actor: 5, Document: 3, SessionExpires: 2100000000000, Intent: intent, Path: path,
		Access: DocumentAccess{Document: dto.Document{Id: "3", Title: "固定标题", Lifecycle: "ACTIVE", PublicationState: "PUBLISHED", CurrentReleaseId: publicationText("19"),
			CurrentManifestId: "13", CurrentStructureId: "11", CurrentStyleId: "17", ControlRevision: "9", PublicationEpoch: "7", AclRevision: "4"},
			Permissions: domain.Permissions{Manage: true, Read: true}, Sources: sources, Proofs: []*pb.IdentityAdmission{proof}},
		Main: reliableMainBinding{Manifest: 13, Structure: 11, Style: 17, Control: 9, Units: []uint64{21}, Versions: map[uint64]uint64{21: 23},
			Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: 5, Target: target, Sources: sources, RequestFingerprint: fingerprint[:], IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "publication-original-admission"}},
	}
	original, _ := json.Marshal(fixed)
	return repository.PendingPublicationBinding{Operation: intent.Operation, State: "PREPARING", Kind: "RELEASE_CREATE", Actor: 5,
		Original: original, Payload: original, Fingerprint: fingerprint[:], Target: []byte(`{"path":"/v1/documents/3/releases"}`)}
}

// 两份持久载荷一致也不够：恢复必须绑定原receipt输入摘要、原准入、完整来源及整篇单元集合。
func TestPublicationReadinessRequiresOriginalWholeBinding(t *testing.T) {
	if err := validatePendingPublicationBinding(pendingPublicationFixture(t)); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*publicationContext)
	}{
		{"actor", func(f *publicationContext) { f.Actor++ }},
		{"document", func(f *publicationContext) { f.Document++ }},
		{"session", func(f *publicationContext) { f.SessionExpires++ }},
		{"permission", func(f *publicationContext) { f.Access.Permissions.Manage = false }},
		{"target", func(f *publicationContext) { f.Main.Context.Target.Purpose = "MAIN_CONTROL" }},
		{"proof", func(f *publicationContext) { f.Access.Proofs[0].Purpose = "READ_MAIN" }},
		{"sources", func(f *publicationContext) {
			f.Access.Sources = append(f.Access.Sources, &pb.SourceEpoch{ScopeKey: "identity:99", Epoch: 1})
		}},
		{"version", func(f *publicationContext) { f.Main.Versions[21] = 0 }},
		{"units", func(f *publicationContext) { f.Main.Units = append(f.Main.Units, 21) }},
		{"fingerprint", func(f *publicationContext) { f.Main.Context.RequestFingerprint[0] ^= 1 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			fact := pendingPublicationFixture(t)
			var fixed publicationContext
			if err := json.Unmarshal(fact.Payload, &fixed); err != nil {
				t.Fatal(err)
			}
			change.edit(&fixed)
			fact.Payload, _ = json.Marshal(fixed)
			if validatePendingPublicationBinding(fact) == nil {
				t.Fatal("changed recovery reported ready")
			}
		})
	}
	fact := pendingPublicationFixture(t)
	var fixed publicationContext
	_ = json.Unmarshal(fact.Original, &fixed)
	fixed.Intent.Control++
	fixed.Main.Control++
	fixed.Access.Document.ControlRevision = "10"
	fixed.Main.Context.Target.WorkspaceEpoch++
	fixed.Access.Proofs[0].Target.WorkspaceEpoch++
	fact.Original, _ = json.Marshal(fixed)
	fact.Payload = fact.Original
	if validatePendingPublicationBinding(fact) == nil {
		t.Fatal("two changed contexts overrode original receipt fingerprint")
	}
}

func publicationText(value string) *string { return &value }
