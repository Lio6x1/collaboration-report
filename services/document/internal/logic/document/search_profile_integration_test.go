//go:build search_profile

package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	client "collaboration/document/internal/client"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
	"time"
)

type measuredSearchPeer struct {
	peer    SearchCandidatesPeer
	calls   int
	elapsed time.Duration
}

func (p *measuredSearchPeer) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	start := time.Now()
	reply, err := p.peer.SearchCandidates(ctx, request)
	p.calls++
	p.elapsed += time.Since(start)
	return reply, err
}

// 该独立标签只作真实阶段诊断，不代表HTTP/Edge/最终输出验收，也不在常规模块SKIP。
// 使用同一三秒读取预算和当前自有规模夹具，输出累计阶段时间，不输出Doc/PIT/正文。
func TestActualSearchPageScalePhaseProfile(t *testing.T) {
	config, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG"))
	if err != nil || config.Role != "document" {
		t.Fatal("SEARCH_PROFILE_DOCUMENT_CONFIG_REQUIRED")
	}
	raw, err := os.ReadFile(filepath.Join(config.Root, ".runtime", "search-page-scale-profile.json"))
	var input struct {
		Query string `json:"query"`
	}
	if err != nil || json.Unmarshal(raw, &input) != nil || !regexp.MustCompile(`^searchscale[a-f0-9]{32}sample$`).MatchString(input.Query) {
		t.Fatal("SEARCH_PROFILE_OWN_SCALE_QUERY_REQUIRED")
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_DATABASE_UNAVAILABLE")
	}
	t.Cleanup(func() { store.DB.Close() })
	connection, err := platform.DialPeer(config, "search")
	if err != nil {
		t.Fatal("SEARCH_PROFILE_PEER_UNAVAILABLE")
	}
	t.Cleanup(func() { connection.Close() })
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal("SEARCH_PROFILE_VALIDATOR_UNAVAILABLE")
	}
	s := &SearchPages{db: store.DB, validator: validator}
	peer := &measuredSearchPeer{peer: client.NewSearch(pb.NewSearchInternalClient(connection))}
	query := domain.SearchQuery{Text: input.Query, Limit: 92}
	binding, err := query.Binding(true, nil)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_BINDING_INVALID")
	}
	scope := &pb.SearchScope{Anonymous: true, ScopeHash: hex.EncodeToString(binding[:])}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	var factTime, fixedTime, sourceTime, finalFactsTime time.Duration
	var batches, scanned, fixed int
	defer func() {
		t.Logf("actual profile excludes output lease/Edge: total=%s RPC calls=%d time=%s fact batches=%d time=%s candidates=%d fixed=%d time=%s source locks=%s final facts=%s", time.Since(start), peer.calls, peer.elapsed, batches, factTime, scanned, fixed, fixedTime, sourceTime, finalFactsTime)
	}()
	var used uint64
	page, err := collectSearchPage(ctx, peer, query, scope, binding, nil, func(ctx context.Context, candidates []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		started := time.Now()
		facts, err := s.batchFacts(ctx, candidates)
		batches++
		factTime += time.Since(started)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
			scanned++
			current, present := facts[candidate.DocumentId]
			if !present {
				return nil, nil
			}
			policy, visible, err := qualifySearchCandidate(candidate, current, 0)
			if err != nil || !visible || policy != "PUBLIC" {
				return nil, err
			}
			started := time.Now()
			summary, next, err := s.fixedSearchSummary(ctx, current, candidate.ReleaseId, input.Query, used)
			fixed++
			fixedTime += time.Since(started)
			if err != nil {
				return nil, err
			}
			used = next
			return &searchPageEntry{Summary: summary, Facts: current}, nil
		}, nil
	})
	if err != nil || len(page.Entries) != 93 {
		t.Fatal("SEARCH_PROFILE_ORIGINAL_COLLECTION_BUDGET_NOT_MET")
	}
	groups := make([][]*pb.SourceEpoch, 0, len(page.Entries))
	for _, entry := range page.Entries {
		sources, err := publicationSources(entry.Facts)
		if err != nil {
			t.Fatal("SEARCH_PROFILE_SOURCE_BINDING_INVALID")
		}
		groups = append(groups, sources)
	}
	sources, err := searchSourceUnion(groups...)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_SOURCE_UNION_INVALID")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_FINAL_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	started := time.Now()
	_, err = lockSearchOutputSources(ctx, tx, sources)
	sourceTime = time.Since(started)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_FIRST_SOURCE_LOCK_BUDGET_NOT_MET")
	}
	sort.Slice(page.Entries, func(i, j int) bool {
		left, _ := domain.CanonicalID(page.Entries[i].Facts.Document.Id)
		right, _ := domain.CanonicalID(page.Entries[j].Facts.Document.Id)
		return left < right
	})
	started = time.Now()
	for _, entry := range page.Entries {
		id, _ := domain.CanonicalID(entry.Facts.Document.Id)
		_, err = repository.PublicationFactsRow(ctx, tx, id, true)
		if err != nil {
			break
		}
	}
	finalFactsTime = time.Since(started)
	if err != nil {
		t.Fatal("SEARCH_PROFILE_FINAL_FACTS_BUDGET_NOT_MET")
	}
}
