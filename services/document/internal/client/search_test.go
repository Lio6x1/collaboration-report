package client

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 未使用的冻结RPC显式失败，不借成功stub掩盖客户端调用了错误方法。
type searchPeerFixture struct {
	query func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	close func(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}

func (s *searchPeerFixture) SearchCandidates(ctx context.Context, input *pb.SearchCandidatesRequest, _ ...grpc.CallOption) (*pb.SearchCandidatesReply, error) {
	if s.query == nil {
		return nil, errors.New("unexpected query")
	}
	return s.query(ctx, input)
}

func (s *searchPeerFixture) CloseCursor(ctx context.Context, input *pb.CloseCursorRequest, _ ...grpc.CallOption) (*pb.OperationResult, error) {
	if s.close == nil {
		return nil, errors.New("unexpected close")
	}
	return s.close(ctx, input)
}

func (*searchPeerFixture) StartRebuild(context.Context, *pb.RebuildRequest, ...grpc.CallOption) (*pb.OperationResult, error) {
	return nil, errors.New("unexpected rebuild")
}

func (*searchPeerFixture) GetRebuild(context.Context, *pb.OperationQuery, ...grpc.CallOption) (*pb.OperationResult, error) {
	return nil, errors.New("unexpected rebuild query")
}

func searchRPCRequest() *pb.SearchCandidatesRequest {
	query := "中文 e\u0301  原词法"
	digest := sha256.Sum256([]byte(query))
	return &pb.SearchCandidatesRequest{Query: query, QueryDigest: hex.EncodeToString(digest[:]), PageSize: 100,
		Scope: &pb.SearchScope{Anonymous: true, ScopeHash: strings.Repeat("1", 64), SpaceIds: []uint64{1, ^uint64(0)}}}
}

func searchRPCReply(count int, more bool) *pb.SearchCandidatesReply {
	reply := &pb.SearchCandidatesReply{Complete: true, HasMore: more, IndexGeneration: ^uint64(0), PhysicalIndex: "owned-search-g1", ExpiresUnixMs: time.Now().Add(30 * time.Second).UnixMilli(), Continuation: []byte{0xff, 'o', 'p', 'a', 'q', 'u', 'e'}}
	for i := 0; i < count; i++ {
		id := ^uint64(0) - uint64(i)
		reply.Candidates = append(reply.Candidates, &pb.SearchCandidate{DocumentId: id, ReleaseId: ^uint64(0), AggregateSeq: ^uint64(0), PhysicalIndex: reply.PhysicalIndex,
			SortJsonUtf8: []byte(fmt.Sprintf(`[1.00e+0,"%d",18446744073709551615]`, id))})
	}
	return reply
}

func requireSearchRPCError(t *testing.T, err error, kind domain.FailureKind, code string) {
	t.Helper()
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Kind != kind || violation.Code != code {
		t.Fatalf("error = %v; want fixed code %s", err, code)
	}
}

func TestSearchRPCValidFrozenReplyAndIndependentBytes(t *testing.T) {
	for _, count := range []int{0, 1, 100} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			input, reply := searchRPCRequest(), searchRPCReply(count, count == 100)
			var peerInput *pb.SearchCandidatesRequest
			peer := &searchPeerFixture{query: func(_ context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
				peerInput = request
				if !proto.Equal(request, input) {
					t.Fatal("client rewrote identity, query or opaque cursor")
				}
				return reply, nil
			}}
			got, err := NewSearch(peer).SearchCandidates(context.Background(), input)
			if err != nil || !proto.Equal(got, reply) {
				t.Fatalf("valid reply rejected: %v", err)
			}
			got.Continuation[0] = 7
			if reply.Continuation[0] != 0xff {
				t.Fatal("returned anchor aliases peer bytes")
			}
			peerInput.Scope.SpaceIds[0] = 7
			if input.Scope.SpaceIds[0] != 1 {
				t.Fatal("peer request aliases caller scope")
			}
			if count > 0 {
				got.Candidates[0].SortJsonUtf8[0] = 'x'
				if reply.Candidates[0].SortJsonUtf8[0] != '[' {
					t.Fatal("returned candidate aliases peer sort")
				}
			}
		})
	}
}

func TestSearchRPCRejectsIncompleteAndMalformedReplies(t *testing.T) {
	cases := map[string]func(*pb.SearchCandidatesReply){
		"partial":          func(r *pb.SearchCandidatesReply) { r.Complete = false },
		"over count":       func(r *pb.SearchCandidatesReply) { r.Candidates = searchRPCReply(101, false).Candidates },
		"more short batch": func(r *pb.SearchCandidatesReply) { r.HasMore = true },
		"nil candidate":    func(r *pb.SearchCandidatesReply) { r.Candidates[0] = nil },
		"zero doc":         func(r *pb.SearchCandidatesReply) { r.Candidates[0].DocumentId = 0 },
		"zero release":     func(r *pb.SearchCandidatesReply) { r.Candidates[0].ReleaseId = 0 },
		"zero seq":         func(r *pb.SearchCandidatesReply) { r.Candidates[0].AggregateSeq = 0 },
		"duplicate doc": func(r *pb.SearchCandidatesReply) {
			r.Candidates = append(r.Candidates, proto.Clone(r.Candidates[0]).(*pb.SearchCandidate))
		},
		"wrong physical":     func(r *pb.SearchCandidatesReply) { r.Candidates[0].PhysicalIndex = "owned-other" },
		"illegal physical":   func(r *pb.SearchCandidatesReply) { r.PhysicalIndex = "bad/index" },
		"large physical":     func(r *pb.SearchCandidatesReply) { r.PhysicalIndex = strings.Repeat("a", 161) },
		"UTF8 physical":      func(r *pb.SearchCandidatesReply) { r.PhysicalIndex = string([]byte{0xff}) },
		"zero generation":    func(r *pb.SearchCandidatesReply) { r.IndexGeneration = 0 },
		"zero expires":       func(r *pb.SearchCandidatesReply) { r.ExpiresUnixMs = 0 },
		"excess expires":     func(r *pb.SearchCandidatesReply) { r.ExpiresUnixMs = time.Now().Add(3 * time.Minute).UnixMilli() },
		"missing anchor":     func(r *pb.SearchCandidatesReply) { r.Continuation = nil },
		"large anchor":       func(r *pb.SearchCandidatesReply) { r.Continuation = bytes.Repeat([]byte{1}, 769) },
		"unframeable anchor": func(r *pb.SearchCandidatesReply) { r.Continuation = bytes.Repeat([]byte{1}, 762) },
		"missing sort":       func(r *pb.SearchCandidatesReply) { r.Candidates[0].SortJsonUtf8 = nil },
		"large sort":         func(r *pb.SearchCandidatesReply) { r.Candidates[0].SortJsonUtf8 = bytes.Repeat([]byte{' '}, 129) },
		"UTF8 sort":          func(r *pb.SearchCandidatesReply) { r.Candidates[0].SortJsonUtf8 = []byte{0xff} },
		"extra sort": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`[1,"18446744073709551615",0,1]`)
		},
		"wrong sort doc":   func(r *pb.SearchCandidatesReply) { r.Candidates[0].SortJsonUtf8 = []byte(`[1,"2",0]`) },
		"escaped sort doc": func(r *pb.SearchCandidatesReply) { r.Candidates[0].SortJsonUtf8 = []byte(`[1,"\u0031",0]`) },
		"nonfinite score": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`[1e999,"18446744073709551615",0]`)
		},
		"string score": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`["1","18446744073709551615",0]`)
		},
		"fractional shard": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`[1,"18446744073709551615",1.0]`)
		},
		"overflow shard": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`[1,"18446744073709551615",18446744073709551616]`)
		},
		"negative shard": func(r *pb.SearchCandidatesReply) {
			r.Candidates[0].SortJsonUtf8 = []byte(`[1,"18446744073709551615",-1]`)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			reply := searchRPCReply(1, false)
			change(reply)
			got, err := NewSearch(&searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
				return reply, nil
			}}).SearchCandidates(context.Background(), searchRPCRequest())
			if got != nil {
				t.Fatal("malformed peer data returned")
			}
			requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
		})
	}
	got, err := NewSearch(&searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) { return nil, nil }}).SearchCandidates(context.Background(), searchRPCRequest())
	if got != nil {
		t.Fatal("nil peer data accepted")
	}
	requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

func TestSearchRPCDeadlineAndCancellation(t *testing.T) {
	type marker struct{}
	for _, original := range []time.Duration{time.Second, 9 * time.Second} {
		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), marker{}, "same request"), original)
		peer := &searchPeerFixture{query: func(child context.Context, _ *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
			deadline, ok := child.Deadline()
			parentDeadline, _ := ctx.Deadline()
			if !ok || deadline.After(parentDeadline) || deadline.After(time.Now().Add(5*time.Second)) || child.Value(marker{}) != "same request" {
				t.Fatal("RPC expanded deadline or lost parent context")
			}
			return searchRPCReply(0, false), nil
		}}
		_, err := NewSearch(peer).SearchCandidates(ctx, searchRPCRequest())
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	peer := &searchPeerFixture{query: func(child context.Context, _ *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
		cancel()
		if child.Err() == nil {
			t.Fatal("RPC lost caller cancellation")
		}
		return searchRPCReply(0, false), nil
	}}
	got, err := NewSearch(peer).SearchCandidates(ctx, searchRPCRequest())
	if got != nil {
		t.Fatal("cancelled result accepted")
	}
	requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

func TestSearchRPCFixedErrorMapping(t *testing.T) {
	cases := []struct {
		wire          codes.Code
		message, code string
		kind          domain.FailureKind
	}{
		{codes.FailedPrecondition, "SEARCH_CURSOR_EXPIRED", "CURSOR_EXPIRED", domain.StateConflict},
		{codes.FailedPrecondition, "SEARCH_CURSOR_STALE", "CURSOR_STALE", domain.StateConflict},
		{codes.FailedPrecondition, "SEARCH_CURSOR_CLOSED", "CURSOR_STALE", domain.StateConflict},
		{codes.FailedPrecondition, "SEARCH_CURSOR_REPLAY_CHANGED", "CURSOR_STALE", domain.StateConflict},
		{codes.FailedPrecondition, "SEARCH_CURSOR_BINDING_MISMATCH", "SEARCH_BINDING_INVALID", domain.StateConflict},
		{codes.InvalidArgument, "SEARCH_CURSOR_ANCHOR_INVALID", "CURSOR_INVALID", domain.InvalidRequest},
		{codes.InvalidArgument, "SEARCH_SCOPE_INVALID", "SEARCH_BINDING_INVALID", domain.InvalidRequest},
		{codes.Unavailable, "SQL password=secret ES /private", "SEARCH_INCOMPLETE", domain.TemporarilyUnavailable},
		{codes.Unavailable, "SEARCH_CURSOR_EXPIRED", "SEARCH_INCOMPLETE", domain.TemporarilyUnavailable},
		{codes.FailedPrecondition, "SEARCH_CURSOR_EXPIRED private", "SEARCH_INCOMPLETE", domain.TemporarilyUnavailable},
		{codes.ResourceExhausted, "SEARCH_CURSOR_BUSY", "SEARCH_INCOMPLETE", domain.TemporarilyUnavailable},
		{codes.Unimplemented, "UNIMPLEMENTED", "SEARCH_INCOMPLETE", domain.TemporarilyUnavailable},
	}
	for _, test := range cases {
		t.Run(test.message, func(t *testing.T) {
			wireError := status.Error(test.wire, test.message)
			peer := &searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
				return searchRPCReply(0, false), wireError
			}, close: func(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error) { return nil, wireError }}
			got, err := NewSearch(peer).SearchCandidates(context.Background(), searchRPCRequest())
			if got != nil {
				t.Fatal("error reply leaked")
			}
			requireSearchRPCError(t, err, test.kind, test.code)
			_, err = NewSearch(peer).CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: []byte{1}, Scope: searchRPCRequest().Scope})
			requireSearchRPCError(t, err, test.kind, test.code)
		})
	}
}

func TestSearchRPCRejectsInvalidCallerBindings(t *testing.T) {
	cases := map[string]func(*pb.SearchCandidatesRequest){
		"bad digest":                   func(r *pb.SearchCandidatesRequest) { r.QueryDigest = strings.Repeat("2", 64) },
		"blank query":                  func(r *pb.SearchCandidatesRequest) { r.Query = " " },
		"UTF8 query":                   func(r *pb.SearchCandidatesRequest) { r.Query = string([]byte{0xff}) },
		"long query":                   func(r *pb.SearchCandidatesRequest) { r.Query = strings.Repeat("中", 501) },
		"HTTP page size":               func(r *pb.SearchCandidatesRequest) { r.PageSize = 20 },
		"missing scope":                func(r *pb.SearchCandidatesRequest) { r.Scope = nil },
		"zero scope":                   func(r *pb.SearchCandidatesRequest) { r.Scope.ScopeHash = strings.Repeat("0", 64) },
		"anonymous session":            func(r *pb.SearchCandidatesRequest) { r.Scope.SessionRef = bytes.Repeat([]byte{1}, 32) },
		"missing real session":         func(r *pb.SearchCandidatesRequest) { r.Scope.Anonymous = false },
		"zero session":                 func(r *pb.SearchCandidatesRequest) { r.Scope.Anonymous = false; r.Scope.SessionRef = make([]byte, 32) },
		"space order":                  func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = []uint64{2, 1} },
		"space cap":                    func(r *pb.SearchCandidatesRequest) { r.Scope.SpaceIds = make([]uint64, 1025) },
		"opaque request missing frame": func(r *pb.SearchCandidatesRequest) { r.Cursor = []byte{1} },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			input := searchRPCRequest()
			change(input)
			peer := &searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
				t.Fatal("invalid caller reached peer")
				return nil, nil
			}}
			_, err := NewSearch(peer).SearchCandidates(context.Background(), input)
			if err == nil {
				t.Fatal("invalid caller binding accepted")
			}
		})
	}
	_, err := NewSearch(nil).SearchCandidates(context.Background(), searchRPCRequest())
	requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	_, err = NewSearch(&searchPeerFixture{}).SearchCandidates(context.Background(), nil)
	if err == nil {
		t.Fatal("nil request accepted")
	}
}

func TestSearchRPCCloseAcknowledgment(t *testing.T) {
	for _, state := range []pb.OperationResult_State{pb.OperationResult_PROCESSING, pb.OperationResult_SUCCEEDED} {
		code := "CURSOR_CLOSING"
		if state == pb.OperationResult_SUCCEEDED {
			code = "CURSOR_CLOSED"
		}
		ack := &pb.OperationResult{State: state, BusinessCode: code, Generation: ^uint64(0), OperationId: strings.Repeat("a", 64)}
		peer := &searchPeerFixture{close: func(ctx context.Context, input *pb.CloseCursorRequest) (*pb.OperationResult, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 5*time.Second {
				t.Fatal("close deadline missing")
			}
			return ack, nil
		}}
		got, err := NewSearch(peer).CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: []byte{0xff, 1}, Scope: searchRPCRequest().Scope})
		if err != nil || !proto.Equal(got, ack) {
			t.Fatalf("valid close acknowledgment rejected: %v", err)
		}
	}
	for _, ack := range []*pb.OperationResult{nil, {State: pb.OperationResult_UNKNOWN}, {State: pb.OperationResult_SUCCEEDED, BusinessCode: "CURSOR_CLOSING", Generation: 1, OperationId: strings.Repeat("a", 64)}, {State: pb.OperationResult_SUCCEEDED, BusinessCode: "CURSOR_CLOSED secret", Generation: 1, OperationId: strings.Repeat("a", 64)}} {
		peer := &searchPeerFixture{close: func(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error) { return ack, nil }}
		got, err := NewSearch(peer).CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: []byte{1}, Scope: searchRPCRequest().Scope})
		if got != nil {
			t.Fatal("invalid close reply accepted")
		}
		requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
	}
}

func TestSearchRPCMaximumBoundsAndRealSessionArePreserved(t *testing.T) {
	for _, replay := range []bool{false, true} {
		input := searchRPCRequest()
		input.Scope.Anonymous = false
		input.Scope.SessionRef = bytes.Repeat([]byte{3}, 32)
		var err error
		input.Cursor, err = domain.EnvelopeSearchAnchor(bytes.Repeat([]byte{0xff}, 761), replay)
		if err != nil {
			t.Fatal(err)
		}
		reply := searchRPCReply(100, false)
		reply.Continuation = bytes.Repeat([]byte{0xff}, 761)
		reply.PhysicalIndex = strings.Repeat("a", 160)
		for _, candidate := range reply.Candidates {
			candidate.PhysicalIndex = reply.PhysicalIndex
			candidate.SortJsonUtf8 = append(candidate.SortJsonUtf8, bytes.Repeat([]byte{' '}, 128-len(candidate.SortJsonUtf8))...)
		}
		peer := &searchPeerFixture{query: func(_ context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
			if !proto.Equal(request, input) {
				t.Fatal("real session, original sort or action rewritten")
			}
			return reply, nil
		}}
		got, err := NewSearch(peer).SearchCandidates(context.Background(), input)
		if err != nil || !proto.Equal(got, reply) {
			t.Fatalf("fixed maximum bounds rejected: %v", err)
		}
	}
}

func TestSearchRPCAlreadyExpiredReplyIsExplicit(t *testing.T) {
	reply := searchRPCReply(0, false)
	reply.ExpiresUnixMs = time.Now().Add(-time.Millisecond).UnixMilli()
	peer := &searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
		return reply, nil
	}}
	got, err := NewSearch(peer).SearchCandidates(context.Background(), searchRPCRequest())
	if got != nil {
		t.Fatal("already expired continuation accepted")
	}
	requireSearchRPCError(t, err, domain.StateConflict, "CURSOR_EXPIRED")
}

func TestSearchRPCMaximumTTLStartsAtServerAllocation(t *testing.T) {
	// 合法资源可能在RPC发出之后才预留；不能以客户端发出时刻压短冻结的2分钟最大TTL。
	peer := &searchPeerFixture{query: func(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
		time.Sleep(3 * time.Millisecond)
		reply := searchRPCReply(0, false)
		reply.ExpiresUnixMs = time.Now().Add(2 * time.Minute).UnixMilli()
		return reply, nil
	}}
	if _, err := NewSearch(peer).SearchCandidates(context.Background(), searchRPCRequest()); err != nil {
		t.Fatalf("valid allocation TTL rejected: %v", err)
	}
}

func TestSearchRPCCloseCallerBoundAndCancellation(t *testing.T) {
	for name, request := range map[string]*pb.CloseCursorRequest{
		"nil":         nil,
		"no anchor":   {Scope: searchRPCRequest().Scope},
		"over anchor": {Cursor: bytes.Repeat([]byte{1}, 769), Scope: searchRPCRequest().Scope},
		"no scope":    {Cursor: []byte{1}},
	} {
		t.Run(name, func(t *testing.T) {
			peer := &searchPeerFixture{close: func(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error) {
				t.Fatal("invalid close reached peer")
				return nil, nil
			}}
			_, err := NewSearch(peer).CloseCursor(context.Background(), request)
			requireSearchRPCError(t, err, domain.InvalidRequest, "SEARCH_BINDING_INVALID")
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	input := &pb.CloseCursorRequest{Cursor: bytes.Repeat([]byte{0xff}, 768), Scope: searchRPCRequest().Scope}
	peer := &searchPeerFixture{close: func(child context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error) {
		deadline, _ := child.Deadline()
		parentDeadline, _ := ctx.Deadline()
		if deadline != parentDeadline || !proto.Equal(request, input) {
			t.Fatal("close expanded HTTP deadline or changed opaque anchor")
		}
		request.Cursor[0] = 7
		cancel()
		return &pb.OperationResult{State: pb.OperationResult_SUCCEEDED, BusinessCode: "CURSOR_CLOSED", Generation: 1, OperationId: strings.Repeat("a", 64)}, nil
	}}
	got, err := NewSearch(peer).CloseCursor(ctx, input)
	if got != nil || input.Cursor[0] != 0xff {
		t.Fatal("cancelled close accepted or input buffer mutated")
	}
	requireSearchRPCError(t, err, domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}
