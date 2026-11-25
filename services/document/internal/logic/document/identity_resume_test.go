package document

import (
	"context"
	"testing"
	"time"

	pb "collaboration/contracts/internalv1"
	"google.golang.org/protobuf/proto"
)

type admissionRenewalStub struct {
	current *pb.IdentityAdmission
}

func TestDiscardPreviewResumeKeepsOriginalControlledUnit(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "discard-old-admission", OperationId: "discard-preview-operation", ActorId: 7, Purpose: "DISCARD_PREVIEW", ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Target: &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 5, StructureId: 6, WorkspaceEpoch: 2, Purpose: "DISCARD_PREVIEW"}}
	current := proto.Clone(original).(*pb.IdentityAdmission)
	current.AdmissionId, current.ExpiresUnixMs = "discard-new-admission", time.Now().Add(30*time.Second).UnixMilli()
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	current.Target.UnitId++
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err == nil {
		t.Fatal("discard preview changed original unit")
	}
}

// 结构预览固定的是全篇原切点；短证明自然过期后仅换准入编号/期限，不能换结构或控制代次。
func TestStructurePreviewResumeRetainsOriginalWholeDocumentBinding(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "old-structure-admission", OperationId: "structure-preview-original", ActorId: 7,
		Purpose: "STRUCTURE_PREVIEW", SessionRef: make([]byte, 32), ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(),
		Target:         &pb.Target{DocumentId: 3, StructureId: 5, WorkspaceEpoch: 2, Purpose: "STRUCTURE_PREVIEW"},
		IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 7}, Capability: pb.IdentityCapability_ACTIVE_ACCOUNT,
		Sources: []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 2}}}
	current := proto.Clone(original).(*pb.IdentityAdmission)
	current.AdmissionId, current.ExpiresUnixMs = "new-structure-admission", time.Now().Add(30*time.Second).UnixMilli()
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*pb.IdentityAdmission)
	}{
		{"structure", func(p *pb.IdentityAdmission) { p.Target.StructureId++ }},
		{"control", func(p *pb.IdentityAdmission) { p.Target.WorkspaceEpoch++ }},
		{"unit", func(p *pb.IdentityAdmission) { p.Target.UnitId = 9 }},
		{"branch", func(p *pb.IdentityAdmission) { p.Target.BranchId = 9 }},
		{"session", func(p *pb.IdentityAdmission) { p.SessionRef[0]++ }},
		{"source", func(p *pb.IdentityAdmission) { p.Sources[0].Epoch++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := proto.Clone(current).(*pb.IdentityAdmission)
			change.edit(changed)
			if _, err := NewIdentity(nil, nil, admissionRenewalStub{changed}).Resume(context.Background(), original); err == nil {
				t.Fatal("changed original preview binding accepted")
			}
		})
	}
}

func TestPullMainResumeKeepsOriginalBranchEpoch(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "old-pull-admission", OperationId: "pull-operation", ActorId: 7, Purpose: "PULL_MAIN", ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Target: &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 5, StructureId: 6, WorkspaceEpoch: 2, Purpose: "PULL_MAIN"}}
	current := proto.Clone(original).(*pb.IdentityAdmission)
	current.AdmissionId, current.ExpiresUnixMs = "new-pull-admission", time.Now().Add(30*time.Second).UnixMilli()
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	current.Target.WorkspaceEpoch++
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err == nil {
		t.Fatal("original pull changed into a later epoch")
	}
}

func TestMainControlResumeRetainsTheWholeDocumentTarget(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "old-control-admission", OperationId: "main-control-operation", ActorId: 7, Purpose: "MAIN_CONTROL", ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Target: &pb.Target{DocumentId: 3, StructureId: 5, WorkspaceEpoch: 1, Purpose: "MAIN_CONTROL"}}
	current := proto.Clone(original).(*pb.IdentityAdmission)
	current.AdmissionId, current.ExpiresUnixMs = "new-control-admission", time.Now().Add(30*time.Second).UnixMilli()
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	current.Target.UnitId = 9
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{current}).Resume(context.Background(), original); err == nil {
		t.Fatal("whole-document control changed into another scope")
	}
}

func TestMergeResumeKeepsOriginalPurposeAndBinding(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "original-merge-admission", OperationId: "original-merge-operation", ActorId: 7,
		Purpose: "MERGE", ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(),
		Target:  &pb.Target{DocumentId: 3, UnitId: 5, Purpose: "MERGE"},
		Sources: []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 2}}}
	current := proto.Clone(original).(*pb.IdentityAdmission)
	current.AdmissionId, current.ExpiresUnixMs = "renewed-merge-admission", time.Now().Add(30*time.Second).UnixMilli()
	service := NewIdentity(nil, nil, admissionRenewalStub{current})
	if _, err := service.Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	changed := proto.Clone(current).(*pb.IdentityAdmission)
	changed.Target.Purpose = "SUBMISSION"
	if _, err := NewIdentity(nil, nil, admissionRenewalStub{changed}).Resume(context.Background(), original); err == nil {
		t.Fatal("purpose changed")
	}
}

func (r admissionRenewalStub) RenewAdmission(context.Context, *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	return r.current, nil
}

// 续接只允许新的期限/准入编号；跨进程客户端不能借返回包变更actor、session或原权限来源。
func TestResumeRejectsChangedPersistentBinding(t *testing.T) {
	original := &pb.IdentityAdmission{AdmissionId: "original-admission", OperationId: "original-operation", ActorId: 7,
		Purpose: "SUBMISSION", SessionRef: []byte("actual-session-reference-32-byte"), ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(),
		Target:         &pb.Target{DocumentId: 3, UnitId: 5, Purpose: "SUBMISSION"},
		IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 7}, Capability: pb.IdentityCapability_ACTIVE_ACCOUNT,
		Sources: []*pb.SourceEpoch{{ScopeKey: "identity:7", Epoch: 2}}}
	valid := proto.Clone(original).(*pb.IdentityAdmission)
	valid.AdmissionId, valid.ExpiresUnixMs = "renewed-admission", time.Now().Add(30*time.Second).UnixMilli()
	service := NewIdentity(nil, nil, admissionRenewalStub{valid})
	if _, err := service.Resume(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*pb.IdentityAdmission)
	}{
		{"actor", func(p *pb.IdentityAdmission) { p.ActorId++ }},
		{"session", func(p *pb.IdentityAdmission) { p.SessionRef[0]++ }},
		{"epoch", func(p *pb.IdentityAdmission) { p.Sources[0].Epoch++ }},
		{"target", func(p *pb.IdentityAdmission) { p.Target.UnitId++ }},
		{"capability", func(p *pb.IdentityAdmission) { p.Capability = pb.IdentityCapability_ORGANIZATION_ADMIN }},
		{"operation", func(p *pb.IdentityAdmission) { p.OperationId = "another-operation" }},
		{"identity target", func(p *pb.IdentityAdmission) { p.IdentityTarget.UserId++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			proof := proto.Clone(valid).(*pb.IdentityAdmission)
			change.edit(proof)
			service := NewIdentity(nil, nil, admissionRenewalStub{proof})
			if _, err := service.Resume(context.Background(), original); err == nil {
				t.Fatal("changed binding accepted")
			}
		})
	}
}
