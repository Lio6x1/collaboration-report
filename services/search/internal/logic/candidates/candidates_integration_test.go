package candidates

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	domain "collaboration/search/internal/domain/cursor"
	index "collaboration/search/internal/domain/index"
	"collaboration/search/internal/logic/generation"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

type actualFixture struct {
	usecase         *Candidates
	store           *platform.Store
	sink            *elastic.Client
	guard           *generation.Guard
	alias, physical string
	key             []byte
	hashes          [][32]byte
}

func candidateFixture(t *testing.T) *actualFixture {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	sink, err := elastic.New(config.Search)
	if err != nil {
		store.DB.Close()
		t.Fatal(err)
	}
	key, err := hex.DecodeString(config.Search.CursorKey)
	if err != nil {
		t.Fatal("ACTUAL_CURSOR_KEY_INVALID")
	}
	alias := fmt.Sprintf("actual-candidates-%d-%d", os.Getpid(), time.Now().UnixNano())
	fixture := &actualFixture{store: store, sink: sink, alias: alias, physical: alias + "-g1", key: key}
	t.Cleanup(func() {
		// 只删除已由本测试创建并记录的精确链、代次和owned隔离索引；不动产品alias。
		for _, hash := range fixture.hashes {
			if _, err := store.DB.Exec("DELETE FROM cursor_registry WHERE cursor_hash=?", hash[:]); err != nil {
				t.Error("ACTUAL_CURSOR_CLEANUP_FAILED")
			}
		}
		actual, exists, err := sink.ResolveAlias(context.Background(), alias)
		if err != nil || (exists && actual != fixture.physical) {
			t.Error("ACTUAL_ALIAS_CLEANUP_UNBOUND")
			return
		}
		if exists {
			req, _ := http.NewRequest("DELETE", config.Search.ElasticEndpoint+"/"+fixture.physical+"/_alias/"+alias, nil)
			response, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
			if err != nil {
				t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
				return
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
				return
			}
		}
		if _, err := store.DB.Exec("DELETE FROM index_generation WHERE alias_name=?", alias); err != nil {
			t.Error("ACTUAL_GENERATION_CLEANUP_FAILED")
		}
		if err := sink.DeleteOwnedIndex(context.Background(), fixture.physical); err != nil {
			t.Error(err)
		}
		sink.Close()
		store.DB.Close()
	})
	guard, err := generation.New(store.DB, sink, alias, fixture.physical)
	if err != nil {
		t.Fatal(err)
	}
	fixture.guard = guard
	if err := guard.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	fixture.usecase, err = New(store.DB, guard, sink, alias, key, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *actualFixture) seed(t *testing.T, count uint64) {
	t.Helper()
	for id := uint64(1); id <= count; id++ {
		value := index.Authority{Document: id, Release: ^uint64(0), Sequence: ^uint64(0), Visible: true, Policy: "PUBLIC", Title: "candidatefixtureword", Text: "candidatefixtureword", Digest: [32]byte{1}}
		if _, err := f.sink.Apply(context.Background(), f.physical, value); err != nil {
			t.Fatal(err)
		}
	}
	// 不通过sleep猜refresh完成；核验ES明确完整分片响应。
	req, _ := http.NewRequest("POST", mustElasticEndpoint(t)+"/"+f.physical+"/_refresh", nil)
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("ACTUAL_REFRESH_UNAVAILABLE")
	}
	defer response.Body.Close()
	var facts struct {
		Shards struct{ Total, Successful, Failed int } `json:"_shards"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&facts) != nil || facts.Shards.Total < 1 || facts.Shards.Successful != facts.Shards.Total || facts.Shards.Failed != 0 {
		t.Fatal("ACTUAL_REFRESH_INCOMPLETE")
	}
}

func mustElasticEndpoint(t *testing.T) string {
	t.Helper()
	config, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_SEARCH_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	return config.Search.ElasticEndpoint
}

func fixtureRequest() *pb.SearchCandidatesRequest {
	scope := sha256.Sum256([]byte("candidate-actual-scope"))
	return &pb.SearchCandidatesRequest{Query: "candidatefixtureword", QueryDigest: domain.QueryDigest("candidatefixtureword"), PageSize: 100, Scope: &pb.SearchScope{Anonymous: true, ScopeHash: hex.EncodeToString(scope[:])}}
}

func (f *actualFixture) remember(t *testing.T, reply *pb.SearchCandidatesReply) [32]byte {
	t.Helper()
	codec, err := domain.NewCodec(f.key)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := codec.Open(reply.Continuation)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(anchor.Chain[:])
	f.hashes = append(f.hashes, hash)
	return hash
}

func requestAfter(t *testing.T, original *pb.SearchCandidatesRequest, action domain.Action, reply *pb.SearchCandidatesReply) *pb.SearchCandidatesRequest {
	t.Helper()
	req := proto.Clone(original).(*pb.SearchCandidatesRequest)
	var err error
	req.Cursor, err = domain.Envelope(action, reply.Continuation)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestActualCandidateSingleChainReplayNextRetryBindingsAndClose(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 205)
	req := fixtureRequest()
	ctx := context.Background()
	first, err := f.usecase.SearchCandidates(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	hash := f.remember(t, first)
	if !first.Complete || !first.HasMore || len(first.Candidates) != 100 || first.PhysicalIndex != f.physical || first.IndexGeneration != 1 {
		t.Fatal("ACTUAL_FIRST_BATCH_INCOMPLETE")
	}
	replay, err := f.usecase.SearchCandidates(ctx, requestAfter(t, req, domain.ActionReplay, first))
	if err != nil || !proto.Equal(first, replay) {
		t.Fatal("ACTUAL_BATCH_REPLAY_CHANGED", err)
	}
	nextRequest := requestAfter(t, req, domain.ActionNext, first)
	second, err := f.usecase.SearchCandidates(ctx, nextRequest)
	if err != nil || len(second.Candidates) != 100 || !second.HasMore {
		t.Fatal("ACTUAL_SECOND_BATCH_INCOMPLETE", err)
	}
	secondRetry, err := f.usecase.SearchCandidates(ctx, nextRequest)
	if err != nil || !proto.Equal(second, secondRetry) {
		t.Fatal("ACTUAL_ORIGINAL_NEXT_RETRY_SKIPPED_BATCH", err)
	}
	last, err := f.usecase.SearchCandidates(ctx, requestAfter(t, req, domain.ActionNext, second))
	if err != nil || len(last.Candidates) != 5 || last.HasMore || len(last.Continuation) == 0 {
		t.Fatal("ACTUAL_TERMINAL_SUFFIX_LOST", err)
	}
	lastReplay, err := f.usecase.SearchCandidates(ctx, requestAfter(t, req, domain.ActionReplay, last))
	if err != nil || !proto.Equal(last, lastReplay) {
		t.Fatal("ACTUAL_TERMINAL_SUFFIX_NOT_REPLAYABLE", err)
	}
	seen := map[uint64]bool{}
	for _, batch := range []*pb.SearchCandidatesReply{first, second, last} {
		if batch.ExpiresUnixMs != first.ExpiresUnixMs {
			t.Fatal("ACTUAL_CURSOR_RENEWED_ABSOLUTE_EXPIRY")
		}
		for _, item := range batch.Candidates {
			if seen[item.DocumentId] {
				t.Fatal("ACTUAL_NEXT_DUPLICATED_DOCUMENT")
			}
			seen[item.DocumentId] = true
		}
	}
	if len(seen) != 205 {
		t.Fatal("ACTUAL_NEXT_SWALLOWED_PROBE")
	}
	var count int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE physical_index=?", f.physical).Scan(&count); err != nil || count != 1 {
		t.Fatal("ACTUAL_CHAIN_REGISTERED_PER_BATCH")
	}
	for _, change := range []func(*pb.SearchCandidatesRequest){
		func(r *pb.SearchCandidatesRequest) {
			r.Query = "different"
			r.QueryDigest = domain.QueryDigest(r.Query)
		},
		func(r *pb.SearchCandidatesRequest) { r.Scope.ScopeHash = domain.QueryDigest("different-scope") },
		func(r *pb.SearchCandidatesRequest) {
			r.Scope.Anonymous = false
			r.Scope.SessionRef = make([]byte, 32)
			r.Scope.SessionRef[0] = 1
		},
	} {
		changed := requestAfter(t, req, domain.ActionReplay, first)
		change(changed)
		if _, err := f.usecase.SearchCandidates(ctx, changed); err == nil {
			t.Fatal("ACTUAL_UNBOUND_CURSOR_ACCEPTED")
		}
	}
	closeRequest := &pb.CloseCursorRequest{Cursor: last.Continuation, Scope: req.Scope}
	closed, err := f.usecase.CloseCursor(ctx, closeRequest)
	if err != nil || closed.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("ACTUAL_PIT_CLOSE_NOT_CONFIRMED", err)
	}
	closedAgain, err := f.usecase.CloseCursor(ctx, closeRequest)
	if err != nil || closedAgain.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("ACTUAL_CLOSE_NOT_IDEMPOTENT", err)
	}
	var state, pit string
	if err := f.store.DB.QueryRow("SELECT state,pit_handle FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &pit); err != nil || state != "CLOSED" || pit != "" {
		t.Fatal("ACTUAL_CLOSE_NOT_PERSISTED")
	}
	if _, err := f.usecase.SearchCandidates(ctx, nextRequest); err == nil {
		t.Fatal("ACTUAL_CLOSED_CHAIN_CONTINUED")
	}
}

func TestActualCleanupRevalidatesFreshQueryAfterDueSnapshot(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 1)
	first, err := f.usecase.SearchCandidates(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	hash := f.remember(t, first)
	// 模拟Due选到旧在途行后，原查询已在SQL确认成健康IDLE。领取时必须重读资格，
	// 不能用过时的Due快照关闭新完成的链；nil scope代表worker而非用户显式Close。
	record, err := f.usecase.closeHash(context.Background(), hash, nil)
	if err != nil || record.State != "OPEN" {
		t.Fatal("ACTUAL_CLEANUP_CLOSED_NEWLY_CONFIRMED_QUERY", err)
	}
	if _, err := f.usecase.SearchCandidates(context.Background(), requestAfter(t, fixtureRequest(), domain.ActionReplay, first)); err != nil {
		t.Fatal("ACTUAL_HEALTHY_CHAIN_NOT_REPLAYABLE", err)
	}
	closed, err := f.usecase.CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: first.Continuation, Scope: fixtureRequest().Scope})
	if err != nil || closed.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("ACTUAL_TEST_CURSOR_CLOSE_FAILED", err)
	}
}

func TestActualCleanupLockedFirstChainDoesNotStarveNext(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 1)
	var err error
	f.usecase, err = New(f.store.DB, f.guard, f.sink, f.alias, f.key, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.usecase.SearchCandidates(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	firstHash := f.remember(t, first)
	second, err := f.usecase.SearchCandidates(context.Background(), fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	secondHash := f.remember(t, second)
	// 自然应用期限到期后，以第二连接持续锁住排序更早的链。不能让它垄断所有清理轮次。
	blocked, err := f.store.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Rollback()
	var state string
	if err := blocked.QueryRow("SELECT state FROM cursor_registry WHERE cursor_hash=? FOR UPDATE", firstHash[:]).Scan(&state); err != nil {
		t.Fatal(err)
	}
	// 在到期前锁住第一链，防止同时运行的真实产品worker先将它清理，破坏夹具前提。
	time.Sleep(time.Until(time.UnixMilli(second.ExpiresUnixMs)) + 20*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := f.usecase.Cleanup(ctx); err != nil {
		t.Fatal("ACTUAL_LOCKED_CHAIN_BLOCKED_CLEANUP", err)
	}
	// 允许第一链仍OPEN，但后链应已关闭/有界删除；不能靠释放第一锁之后才通过。
	err = f.store.DB.QueryRow("SELECT state FROM cursor_registry WHERE cursor_hash=?", secondHash[:]).Scan(&state)
	if err != sql.ErrNoRows && (err != nil || state != "CLOSED") {
		t.Fatal("ACTUAL_LOCKED_FIRST_CHAIN_STARVED_NEXT", err)
	}
}
