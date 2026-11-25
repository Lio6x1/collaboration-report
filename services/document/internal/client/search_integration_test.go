package client

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
	"time"
)

func TestActualDocSearchClientMTLSReplayAndBinding(t *testing.T) {
	load := func(name string) platform.Config {
		file := os.Getenv(name)
		if file == "" {
			t.Fatal("ACTUAL_SERVICE_CONFIG_REQUIRED")
		}
		cfg, err := platform.LoadConfig(file)
		if err != nil {
			t.Fatal("ACTUAL_SERVICE_CONFIG_INVALID")
		}
		return cfg
	}
	doc, search := load("COLLAB_TEST_DOCUMENT_CONFIG"), load("COLLAB_TEST_SEARCH_CONFIG")
	if doc.Role != "document" || search.Role != "search" || doc.Root != search.Root || search.Search == nil {
		t.Fatal("ACTUAL_SERVICE_OWNER_BINDING_INVALID")
	}
	// 当前产品Doc尚未装配search peer；测试只给内存副本补当前owner已分配地址。
	// 使用真实Doc私钥/mTLS，不写配置，不读取Search数据库，不冒称HTTP入口已接线。
	peers := make(map[string]string, len(doc.Peers)+1)
	for key, value := range doc.Peers {
		peers[key] = value
	}
	doc.Peers = peers
	doc.Peers["search"] = fmt.Sprintf("localhost:%d", search.RPCPort)
	connection, err := platform.DialPeer(doc, "search")
	if err != nil {
		t.Fatal("ACTUAL_DOC_SEARCH_MTLS_CONNECT_FAILED")
	}
	defer connection.Close()
	peer := NewSearch(pb.NewSearchInternalClient(connection))
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal("ACTUAL_SEARCH_QUERY_NONCE_FAILED")
	}
	query := domain.SearchQuery{Text: "empty-doc-client-" + hex.EncodeToString(nonce[:]), Limit: 3}
	binding, err := query.Binding(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(query.Text))
	scope := &pb.SearchScope{Anonymous: true, ScopeHash: hex.EncodeToString(binding[:])}
	request := &pb.SearchCandidatesRequest{Query: query.Text, QueryDigest: hex.EncodeToString(digest[:]), PageSize: 100, Scope: scope}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := peer.SearchCandidates(ctx, request)
	if err != nil {
		t.Fatalf("ACTUAL_DOC_SEARCH_QUERY_FAILED code=%s", searchClientFailureCode(err))
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		result, err := peer.CloseCursor(cleanup, &pb.CloseCursorRequest{Cursor: first.Continuation, Scope: scope})
		if err != nil || result == nil || result.State != pb.OperationResult_SUCCEEDED {
			t.Errorf("ACTUAL_DOC_SEARCH_CLOSE_NOT_CONFIRMED code=%s", searchClientFailureCode(err))
		}
	}()
	if !first.Complete || len(first.Candidates) != 0 || first.HasMore || len(first.Continuation) == 0 {
		t.Fatal("ACTUAL_DOC_SEARCH_EMPTY_BATCH_INVALID")
	}
	request.Cursor, err = domain.EnvelopeSearchAnchor(first.Continuation, true)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := peer.SearchCandidates(ctx, request)
	if err != nil || replay.ExpiresUnixMs != first.ExpiresUnixMs || replay.IndexGeneration != first.IndexGeneration || replay.PhysicalIndex != first.PhysicalIndex || len(replay.Candidates) != 0 {
		t.Fatal("ACTUAL_DOC_SEARCH_REPLAY_CHANGED")
	}
	changed := proto.Clone(scope).(*pb.SearchScope)
	changed.ScopeHash = hex.EncodeToString(digest[:])
	request.Scope = changed
	_, err = peer.SearchCandidates(ctx, request)
	if searchClientFailureCode(err) != "SEARCH_BINDING_INVALID" {
		t.Fatal("ACTUAL_DOC_SEARCH_BINDING_CHANGE_NOT_REJECTED")
	}
}

func searchClientFailureCode(err error) string {
	if err == nil {
		return "NONE"
	}
	var failure domain.RuleViolation
	if errors.As(err, &failure) {
		return failure.Code
	}
	return "UNKNOWN"
}
