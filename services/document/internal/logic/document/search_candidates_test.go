package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// 此夹具只验证Doc消费前缀，不代替真实Search签名、SQL、ES或权限验收。
// 每个锚点稳定指向一个批次，NEXT与REPLAY选取不同位置，能暴露终批后缀被吞的问题。
type searchBatchPeer struct {
	batches  []*pb.SearchCandidatesReply
	requests []*pb.SearchCandidatesRequest
	failAt   int
}

func (p *searchBatchPeer) SearchCandidates(_ context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	p.requests = append(p.requests, request)
	if p.failAt != 0 && len(p.requests) == p.failAt {
		return nil, errors.New("peer unavailable")
	}
	index := 0
	if len(request.Cursor) > 0 {
		if len(request.Cursor) != 8 {
			return nil, errors.New("unexpected frame")
		}
		index = int(request.Cursor[7])
		if request.Cursor[4] == 1 {
			index++
		}
	}
	if index >= len(p.batches) {
		return nil, errors.New("past EOF")
	}
	return p.batches[index], nil
}

func searchBatchFixture(count int) *searchBatchPeer {
	p := &searchBatchPeer{}
	until := time.Now().Add(30 * time.Second).UnixMilli()
	for offset := 0; offset < count || offset == 0; offset += 100 {
		batch := &pb.SearchCandidatesReply{Complete: true, PhysicalIndex: "owned-g1", IndexGeneration: 1, ExpiresUnixMs: until, Continuation: []byte{byte(offset / 100)}}
		for i := offset; i < count && i < offset+100; i++ {
			batch.Candidates = append(batch.Candidates, &pb.SearchCandidate{DocumentId: uint64(i + 1), ReleaseId: 1, AggregateSeq: 1, PhysicalIndex: batch.PhysicalIndex})
		}
		batch.HasMore = offset+100 < count
		p.batches = append(p.batches, batch)
	}
	return p
}

func searchBatchEligibility(rejectThrough uint64) searchBatchEvaluator {
	return func(_ context.Context, _ []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		return func(_ context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
			if candidate.DocumentId <= rejectThrough {
				return nil, nil
			}
			return &searchPageEntry{Summary: dto.SearchResult{DocumentId: domain.TextID(candidate.DocumentId), ReleaseId: "1", Title: "已验证固定版"}}, nil
		}, nil
	}
}

func TestSearchCollectionPreservesTerminalLookahead(t *testing.T) {
	peer := searchBatchFixture(7)
	query := domain.SearchQuery{Text: "搜索", Limit: 3}
	binding := [32]byte{1}
	scope := &pb.SearchScope{Anonymous: true, ScopeHash: fmt.Sprintf("%064x", 1)}
	first, err := collectSearchPage(context.Background(), peer, query, scope, binding, nil, searchBatchEligibility(0))
	if err != nil || len(first.Entries) != 4 || first.Next == nil || first.Next.Prefix != 3 {
		t.Fatalf("first page: %+v %v", first, err)
	}
	second, err := collectSearchPage(context.Background(), peer, query, scope, binding, first.Next, searchBatchEligibility(0))
	if err != nil || len(second.Entries) != 4 || second.Entries[0].Summary.DocumentId != "4" || second.Next == nil || second.Next.Prefix != 6 {
		t.Fatalf("second page: %+v %v", second, err)
	}
	third, err := collectSearchPage(context.Background(), peer, query, scope, binding, second.Next, searchBatchEligibility(0))
	if err != nil || len(third.Entries) != 1 || third.Entries[0].Summary.DocumentId != "7" || third.Next != nil {
		t.Fatalf("last suffix: %+v %v", third, err)
	}
	if len(peer.requests) != 3 || peer.requests[1].Cursor[4] != 2 || peer.requests[2].Cursor[4] != 2 {
		t.Fatal("terminal suffix must replay, never NEXT/close")
	}
}

func TestSearchCollectionScansBeyondThousandAndCrossesBatch(t *testing.T) {
	peer := searchBatchFixture(1405)
	query := domain.SearchQuery{Text: "搜索", Limit: 100}
	first, err := collectSearchPage(context.Background(), peer, query, &pb.SearchScope{Anonymous: true}, [32]byte{1}, nil, searchBatchEligibility(1200))
	if err != nil || len(first.Entries) != 101 || first.Entries[0].Summary.DocumentId != "1201" || first.Entries[100].Summary.DocumentId != "1301" || first.Next == nil || first.Next.Prefix != 0 {
		t.Fatalf("no candidate cap: %+v %v", first, err)
	}
	if len(peer.requests) != 14 || peer.requests[1].PageSize != 100 {
		t.Fatalf("fixed batch traversal: %d", len(peer.requests))
	}
}

func TestSearchCollectionUnknownAndCancellationHaveNoHalfPage(t *testing.T) {
	for _, name := range []string{"rpc", "facts", "body", "cancel", "changed deadline"} {
		t.Run(name, func(t *testing.T) {
			peer := searchBatchFixture(105)
			query := domain.SearchQuery{Text: "搜索", Limit: 100}
			evaluate := searchBatchEligibility(0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "rpc":
				peer.failAt = 2
			case "facts":
				evaluate = func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
					return nil, errors.New("SQL unknown")
				}
			case "body":
				evaluate = func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
					return func(_ context.Context, c *pb.SearchCandidate) (*searchPageEntry, error) {
						if c.DocumentId == 101 {
							return nil, domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
						}
						return &searchPageEntry{}, nil
					}, nil
				}
			case "cancel":
				cancel()
			case "changed deadline":
				peer.batches[1].ExpiresUnixMs++
			}
			page, err := collectSearchPage(ctx, peer, query, &pb.SearchScope{Anonymous: true}, [32]byte{1}, nil, evaluate)
			if err == nil || len(page.Entries) != 0 || page.Next != nil {
				t.Fatalf("must fail whole page: %+v %v", page, err)
			}
		})
	}
}

func TestSearchCollectionRejectsExpiredAndInvalidReplayPrefix(t *testing.T) {
	peer := searchBatchFixture(7)
	query := domain.SearchQuery{Text: "搜索", Limit: 3}
	for _, start := range []*domain.SearchPageCursor{
		{Anchor: []byte{0}, Prefix: 7, Until: peer.batches[0].ExpiresUnixMs, Binding: [32]byte{1}},
		{Anchor: []byte{0}, Prefix: 0, Until: time.Now().Add(-time.Second).UnixMilli(), Binding: [32]byte{1}},
	} {
		page, err := collectSearchPage(context.Background(), peer, query, &pb.SearchScope{Anonymous: true}, [32]byte{1}, start, searchBatchEligibility(0))
		if err == nil || len(page.Entries) != 0 {
			t.Fatalf("invalid replay accepted: %+v %v", page, err)
		}
	}
}

func TestSearchCollectionCancelledByLastAssessmentDiscardsPage(t *testing.T) {
	// 最后正文完成时HTTP已取消；不能因为下一步正好EOF/lookahead就绕过期限复核。
	for _, count := range []int{1, 2} {
		peer := searchBatchFixture(count)
		ctx, cancel := context.WithCancel(context.Background())
		evaluate := func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
			return func(_ context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
				if candidate.DocumentId == uint64(count) {
					cancel()
				}
				return &searchPageEntry{}, nil
			}, nil
		}
		page, err := collectSearchPage(ctx, peer, domain.SearchQuery{Text: "搜索", Limit: 1}, &pb.SearchScope{Anonymous: true}, [32]byte{1}, nil, evaluate)
		cancel()
		if err == nil || len(page.Entries) != 0 || page.Next != nil {
			t.Fatal("last assessment cancellation emitted a page")
		}
	}
}

func TestSearchCollectionExpiresDuringLastAssessment(t *testing.T) {
	peer := searchBatchFixture(1)
	until := time.Now().Add(25 * time.Millisecond)
	peer.batches[0].ExpiresUnixMs = until.UnixMilli()
	evaluate := func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		return func(context.Context, *pb.SearchCandidate) (*searchPageEntry, error) {
			time.Sleep(time.Until(until.Add(5 * time.Millisecond)))
			return &searchPageEntry{}, nil
		}, nil
	}
	page, err := collectSearchPage(context.Background(), peer, domain.SearchQuery{Text: "搜索", Limit: 1}, &pb.SearchScope{Anonymous: true}, [32]byte{1}, nil, evaluate)
	var failure domain.RuleViolation
	if len(page.Entries) != 0 || !errors.As(err, &failure) || failure.Code != "CURSOR_EXPIRED" {
		t.Fatalf("expired final assessment: %+v %v", page, err)
	}
}

func TestSearchCollectionBindingMismatchIsStale(t *testing.T) {
	peer := searchBatchFixture(1)
	start := &domain.SearchPageCursor{Anchor: []byte{0}, Binding: [32]byte{2}, Until: peer.batches[0].ExpiresUnixMs}
	_, err := collectSearchPage(context.Background(), peer, domain.SearchQuery{Text: "搜索", Limit: 1}, &pb.SearchScope{Anonymous: true}, [32]byte{1}, start, searchBatchEligibility(0))
	var failure domain.RuleViolation
	if !errors.As(err, &failure) || failure.Code != "CURSOR_STALE" || len(peer.requests) != 0 {
		t.Fatal("changed binding reached Search or was called expired")
	}
}
