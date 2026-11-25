package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"errors"
	"testing"
)

func searchPublishedFacts() domain.PublicationFacts {
	release := "9"
	return domain.PublicationFacts{Document: dto.Document{Id: "5", SpaceId: "6", VisibilityPolicy: "PUBLIC", Lifecycle: "ACTIVE", PublicationState: "PUBLISHED", CurrentReleaseId: &release}, Space: dto.Space{Id: "6", Lifecycle: "ACTIVE"}, Sequence: 10}
}

func TestSearchQualificationUsesCurrentReleaseSequenceAndPolicy(t *testing.T) {
	candidate := &pb.SearchCandidate{DocumentId: 5, ReleaseId: 9, AggregateSeq: 10}
	for _, test := range []struct {
		name    string
		mutate  func(*domain.PublicationFacts)
		space   uint64
		visible bool
		policy  string
	}{
		{"current", func(*domain.PublicationFacts) {}, 0, true, "PUBLIC"},
		{"old release", func(f *domain.PublicationFacts) { v := "11"; f.Document.CurrentReleaseId = &v }, 0, false, ""},
		{"old seq", func(f *domain.PublicationFacts) { f.Sequence++ }, 0, false, ""},
		{"wrong space", func(*domain.PublicationFacts) {}, 7, false, ""},
		{"unpublished", func(f *domain.PublicationFacts) { f.Document.PublicationState = "UNPUBLISHED" }, 0, false, ""},
		{"retained public", func(f *domain.PublicationFacts) { f.Document.Lifecycle = "PUBLIC_RETAINED" }, 0, true, "PUBLIC"},
		{"retained member", func(f *domain.PublicationFacts) {
			f.Document.Lifecycle = "PUBLIC_RETAINED"
			f.Document.VisibilityPolicy = "SPACE_MEMBERS"
		}, 0, false, ""},
		{"KB intersection", func(f *domain.PublicationFacts) {
			f.KnowledgeBase = &dto.KnowledgeBase{VisibilityPolicy: "COLLABORATORS", Lifecycle: "ACTIVE"}
		}, 0, true, "COLLABORATORS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := searchPublishedFacts()
			test.mutate(&f)
			policy, visible, err := qualifySearchCandidate(candidate, f, test.space)
			if err != nil || visible != test.visible || policy != test.policy {
				t.Fatalf("qualification: %s %v %v", policy, visible, err)
			}
		})
	}
	f := searchPublishedFacts()
	f.Document.VisibilityPolicy = "UNKNOWN"
	if _, _, err := qualifySearchCandidate(candidate, f, 0); err == nil {
		t.Fatal("unknown policy became deterministic rejection")
	}
}

func TestSearchUnionRejectsConflictingEpochsAndPreservesLookahead(t *testing.T) {
	actor := &pb.SourceEpoch{ScopeKey: "session:real-session", Epoch: 2}
	returned := &pb.SourceEpoch{ScopeKey: "publication:5", Epoch: 1}
	lookahead := &pb.SourceEpoch{ScopeKey: "publication:6", Epoch: 9}
	union, err := searchSourceUnion([]*pb.SourceEpoch{actor, returned}, []*pb.SourceEpoch{actor, lookahead})
	if err != nil || len(union) != 3 {
		t.Fatalf("whole page union: %+v %v", union, err)
	}
	union[0].Epoch = 999
	if actor.Epoch != 2 || returned.Epoch != 1 || lookahead.Epoch != 9 {
		t.Fatal("union aliases input")
	}
	for _, bad := range []*pb.SourceEpoch{{ScopeKey: returned.ScopeKey, Epoch: 2}, {ScopeKey: "", Epoch: 1}, {ScopeKey: "publication:7", Epoch: 0}, nil} {
		values, err := searchSourceUnion([]*pb.SourceEpoch{actor, returned}, []*pb.SourceEpoch{bad})
		var failure domain.RuleViolation
		if values != nil || !errors.As(err, &failure) || failure.Code != "PAGE_CHANGED_RETRY" {
			t.Fatal("bad union accepted or partially returned")
		}
	}
}
