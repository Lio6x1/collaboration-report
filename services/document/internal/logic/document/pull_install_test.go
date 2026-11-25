package document

import (
	pb "collaboration/contracts/internalv1"
	"testing"
)

func TestCommittedPullRejectsMissingContextWithoutPanic(t *testing.T) {
	_, fact := pendingPullFixture(t)
	fixed, _ := pendingPullFixture(t)
	query := &pb.OperationQuery{OperationId: fact.Operation, Target: fixed.Context.Target, RequestFingerprint: fact.Fingerprint}
	if err := validatePullCommittedBinding(draftInstall{}, query); err == nil {
		t.Fatal("missing context accepted")
	}
	if err := validatePullCommittedBinding(draftInstall{Context: fixed.Context}, query); err != nil {
		t.Fatal(err)
	}
	query.RequestFingerprint = make([]byte, 32)
	if err := validatePullCommittedBinding(draftInstall{Context: fixed.Context}, query); err == nil {
		t.Fatal("another original fingerprint accepted")
	}
}
