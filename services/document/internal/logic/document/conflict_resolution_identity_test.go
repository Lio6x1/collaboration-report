package document

import (
	pb "collaboration/contracts/internalv1"
	"context"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

type resolutionAdmissionRenewer struct {
	original      *pb.IdentityAdmission
	changeSession bool
	called        bool
}

func (s *resolutionAdmissionRenewer) RenewAdmission(_ context.Context, query *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	s.called = true
	proof := proto.Clone(s.original).(*pb.IdentityAdmission)
	proof.AdmissionId = "refreshed-resolution-proof"
	proof.ExpiresUnixMs = time.Now().Add(30 * time.Second).UnixMilli()
	if s.changeSession {
		proof.SessionRef = make([]byte, 32)
	}
	return proof, nil
}

func TestResolutionRenewsExpiredOriginalProofWithoutChangingItsSessionOrTarget(t *testing.T) {
	// 续接需要得到新证明，不能延长旧证明；返回的另一会话即使拥有权限也不能接管原号。
	original := &pb.IdentityAdmission{AdmissionId: "original-resolution-proof", OperationId: "manual-resolution-original", Purpose: "CONFLICT_RESOLUTION", ActorId: 7, SessionRef: []byte("12345678901234567890123456789012"), ExpiresUnixMs: time.Now().Add(-time.Second).UnixMilli(), Target: &pb.Target{DocumentId: 3, UnitId: 4, BranchId: 6, WorkspaceEpoch: 1, Purpose: "CONFLICT_RESOLUTION"}, IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 7}, Sources: []*pb.SourceEpoch{{ScopeKey: "account:user:7", Epoch: 1}}}
	for _, changeSession := range []bool{false, true} {
		renewer := &resolutionAdmissionRenewer{original: original, changeSession: changeSession}
		identity := NewIdentity(nil, nil, renewer)
		proof, err := identity.Resume(context.Background(), original)
		if !renewer.called {
			t.Fatal("registered resolution did not reach bound admission renewal", err)
		}
		if changeSession {
			if err == nil {
				t.Fatal("another login session took over original resolution")
			}
		} else if err != nil || proof.AdmissionId == original.AdmissionId || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			t.Fatal("original resolution did not receive a new current proof", err)
		}
	}
	if original.ExpiresUnixMs > time.Now().UnixMilli() {
		t.Fatal("old proof was extended")
	}
}
