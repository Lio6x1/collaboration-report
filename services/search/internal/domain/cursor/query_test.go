package cursor

import (
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func validQueryRequest() *pb.SearchCandidatesRequest {
	q := "协作 项目"
	digest := sha256.Sum256([]byte(q))
	return &pb.SearchCandidatesRequest{Query: q, QueryDigest: hex.EncodeToString(digest[:]),
		PageSize: 100, Scope: &pb.SearchScope{ScopeHash: strings.Repeat("a", 64), Anonymous: true}}
}

func TestQueryBindingPreservesExactUnicodeAndCopiesScope(t *testing.T) {
	r := validQueryRequest()
	r.Scope.Anonymous = false
	r.Scope.SessionRef = []byte(strings.Repeat("s", 32))
	r.Scope.SpaceIds = []uint64{1, 18446744073709551615}
	q, err := ValidateQuery(r)
	if err != nil || q.Query != r.Query || q.PageSize != 100 || q.QueryHash != r.QueryDigest {
		t.Fatalf("query binding: %+v, %v", q, err)
	}
	r.Scope.SessionRef[0], r.Scope.SpaceIds[0] = 'x', 2
	if q.Scope.SessionRef[0] != 's' || q.Scope.SpaceIds[0] != 1 {
		t.Fatal("scope retained caller-owned mutable bytes")
	}
	// 不修整查询：不同空格/UTF8词法就是另一查询，不能悄悄续接原PIT。
	if QueryDigest(" 协作 ") == QueryDigest("协作") {
		t.Fatal("query silently normalized")
	}
}

func TestQueryRejectsUnboundOrUnboundedRPC(t *testing.T) {
	cases := map[string]func(*pb.SearchCandidatesRequest){
		"empty":            func(r *pb.SearchCandidatesRequest) { r.Query = "" },
		"whitespace":       func(r *pb.SearchCandidatesRequest) { r.Query = " \t\n" },
		"invalidUTF8":      func(r *pb.SearchCandidatesRequest) { r.Query = string([]byte{0xff}) },
		"over500runes":     func(r *pb.SearchCandidatesRequest) { r.Query = strings.Repeat("字", 501) },
		"wrongQueryDigest": func(r *pb.SearchCandidatesRequest) { r.QueryDigest = strings.Repeat("b", 64) },
		"uppercaseDigest":  func(r *pb.SearchCandidatesRequest) { r.QueryDigest = strings.ToUpper(r.QueryDigest) },
		"missingScope":     func(r *pb.SearchCandidatesRequest) { r.Scope = nil },
		"scopeHash":        func(r *pb.SearchCandidatesRequest) { r.Scope.ScopeHash = "client" },
		"zeroScopeHash":    func(r *pb.SearchCandidatesRequest) { r.Scope.ScopeHash = strings.Repeat("0", 64) },
		"anonymousSession": func(r *pb.SearchCandidatesRequest) { r.Scope.SessionRef = make([]byte, 32) },
		"missingSession":   func(r *pb.SearchCandidatesRequest) { r.Scope.Anonymous = false },
		"zeroSession":      func(r *pb.SearchCandidatesRequest) { r.Scope.Anonymous = false; r.Scope.SessionRef = make([]byte, 32) },
		"zeroSpace":        func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = []uint64{0} },
		"duplicateSpace":   func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = []uint64{1, 1} },
		"unorderedSpace":   func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = []uint64{2, 1} },
		"unboundedSpace":   func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = make([]uint64, 1025) },
		"pageZero":         func(r *pb.SearchCandidatesRequest) { r.PageSize = 0 },
		"pageOver100":      func(r *pb.SearchCandidatesRequest) { r.PageSize = 101 },
		"cursorOversize":   func(r *pb.SearchCandidatesRequest) { r.Cursor = make([]byte, 769) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := validQueryRequest()
			change(r)
			if _, err := ValidateQuery(r); err == nil {
				t.Fatal("invalid RPC binding accepted")
			}
		})
	}
	if _, err := ValidateQuery(nil); err == nil {
		t.Fatal("nil request accepted")
	}
	r := validQueryRequest()
	r.Query = strings.Repeat("字", 500)
	r.QueryDigest = QueryDigest(r.Query)
	if _, err := ValidateQuery(r); err != nil {
		t.Fatal(err)
	}
}
