package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

type retainedAuthorityFixture struct{ proof *pb.IdentityAdmission }

func (f retainedAuthorityFixture) AcquireGovernance(context.Context, Actor, string, uint64) (*pb.IdentityAdmission, error) {
	return f.proof, nil
}

func TestRetainedGovernanceRejectsSubstitutedAuthorityProof(t *testing.T) {
	operation := "retained-governance-12345678"
	session := make([]byte, 32)
	actor := Actor{ID: 17, Proof: &pb.IdentityAdmission{SessionRef: session, Sources: []*pb.SourceEpoch{
		{ScopeKey: "identity:17", Epoch: 2}, {ScopeKey: "credential:17", Epoch: 3}, {ScopeKey: "session:fixture", Epoch: 4},
	}}}
	proof := &pb.IdentityAdmission{ActorId: actor.ID, OperationId: operation, SessionRef: session, Purpose: domain.RetainedGovernancePurpose,
		Capability: pb.IdentityCapability_ACTIVE_ACCOUNT, Target: &pb.Target{DocumentId: 29, Purpose: domain.RetainedGovernancePurpose},
		IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, ExpiresUnixMs: time.Now().Add(time.Minute).UnixMilli(),
		Sources: append(append([]*pb.SourceEpoch(nil), actor.Proof.Sources...), &pb.SourceEpoch{ScopeKey: "platform-admin:17", Epoch: 5})}
	// 这些反例检查的是跨服务权限边界，不模拟SQL或逐行镜像实现。即便Account
	// 基础capability仍是ACTIVE_ACCOUNT，也只有专用purpose和真实角色来源可用。
	for _, scenario := range []struct {
		name   string
		change func(*pb.IdentityAdmission)
	}{
		{"普通活跃证明", func(p *pb.IdentityAdmission) { p.Purpose = "DOCUMENT_READ" }},
		{"组织管理员来源", func(p *pb.IdentityAdmission) { p.Sources[3].ScopeKey = "org-admin:1:17" }},
		{"另一原号", func(p *pb.IdentityAdmission) { p.OperationId = "retained-other-12345678" }},
		{"另一文档", func(p *pb.IdentityAdmission) { p.Target.DocumentId = 30 }},
		{"遗漏会话关闭来源", func(p *pb.IdentityAdmission) { p.Sources = append(p.Sources[:2], p.Sources[3]) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			current := proto.Clone(proof).(*pb.IdentityAdmission)
			scenario.change(current)
			s := NewRetainedGovernance(nil, retainedAuthorityFixture{current}, nil, nil, nil, "")
			if _, err := s.Authorize(context.Background(), actor, operation, 29); err == nil {
				t.Fatal("替换或缺失权威绑定仍取得平台治理权限")
			}
		})
	}
	s := NewRetainedGovernance(nil, retainedAuthorityFixture{proof}, nil, nil, nil, "")
	if authorized, err := s.Authorize(context.Background(), actor, operation, 29); err != nil || authorized.Proof != proof {
		t.Fatal("完整真实来源证明未被接受", err)
	}
}
