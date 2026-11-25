package platform

import (
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"testing"
)

func TestNilScopeRejectedWithoutPanic(t *testing.T) {
	if ValidBootstrapFence(&pb.FenceRequest{RevocationId: "logout-0123456789abcdef", Generation: 1, ClosedScopes: []*pb.SourceEpoch{nil}}) {
		t.Fatal("nil scope allowed")
	}
}
func TestCredentialScopeFenceIsBoundToExactEpoch(t *testing.T) {
	scope := &pb.SourceEpoch{ScopeKey: "credential:123", Epoch: 7}
	req := &pb.FenceRequest{RevocationId: "credential-0123456789abcdef", Generation: 1, ClosedScopes: []*pb.SourceEpoch{scope}}
	digest := sha256.Sum256([]byte("AUTHORIZATION_CHANGE\ncredential:123\n7"))
	req.TargetDigest = digest[:]
	if !ValidBootstrapFence(req) {
		t.Fatal("credential closure rejected")
	}
	scope.Epoch = 8
	if ValidBootstrapFence(req) {
		t.Fatal("changed epoch accepted")
	}
}

func TestPublicationScopeFenceRequiresOriginalBinding(t *testing.T) {
	const scope = "publication:18446744073709551615"
	const epoch = uint64(18446744073709551615)
	req := &pb.FenceRequest{
		RevocationId: "publication-fence-operation", Generation: 1,
		ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}},
		TargetDigest: SourceClosureDigest(scope, epoch),
	}
	// 文档发布属于Doc关闭来源；不能通过扩大账号身份用途让它获得账号准入。
	if !ValidDocumentScope(scope) || !ValidFence(req) || ValidAccountScope(scope) || ValidBootstrapFence(req) {
		t.Fatal("publication scope has wrong authority or loses uint64 precision")
	}
	if !MatchesBootstrapAck(req, BootstrapAck(req, "collaboration"), "collaboration") {
		t.Fatal("exact original acknowledgement rejected")
	}
	req.Generation = 2
	if ValidFence(req) {
		t.Fatal("replacement generation accepted")
	}
	req.Generation = 1
	req.ClosedScopes[0].Epoch--
	if ValidFence(req) {
		t.Fatal("changed epoch accepted with original digest")
	}
	req.ClosedScopes[0].Epoch = epoch
	req.TargetDigest[0] ^= 1
	if ValidFence(req) {
		t.Fatal("changed target accepted")
	}
	for _, key := range []string{"publication:0", "publication:01", "publication:+1", "publication:1:2", "publication:18446744073709551616"} {
		if ValidDocumentScope(key) {
			t.Fatalf("ambiguous publication scope accepted: %s", key)
		}
	}
}
