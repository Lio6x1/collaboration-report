package svc

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestActualCandidateRPCUsesDocumentMTLSAndPersistsSingleCursor(t *testing.T) {
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
	t.Cleanup(func() { store.DB.Close() })
	doc, err := platform.LoadConfig(filepath.Join(config.Root, ".runtime", "services", "document.json"))
	if err != nil {
		t.Fatal(err)
	}
	// 旧Doc配置尚未接搜索入口。测试使用Search自身已配置的监听地址和真正Doc证书，
	// 只补本地副本的调用目标；正式Doc client装配时再更新其peer生成，不重写运行配置。
	doc.Peers["search"] = net.JoinHostPort(config.Host, strconv.Itoa(config.RPCPort))
	conn, err := platform.DialPeer(doc, "search")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := pb.NewSearchInternalClient(conn)
	var physical string
	var indexGeneration uint64
	if err := store.DB.QueryRow("SELECT physical_index,generation FROM index_generation WHERE alias_name=?", config.Search.Alias).Scan(&physical, &indexGeneration); err != nil {
		t.Fatal(err)
	}
	query := fmt.Sprintf("rpcfixtureuniqueword%d", time.Now().UnixNano())
	scope := sha256.Sum256([]byte("actual-document-rpc-scope"))
	req := &pb.SearchCandidatesRequest{Query: query, QueryDigest: domain.QueryDigest(query), PageSize: 100,
		Scope: &pb.SearchScope{Anonymous: true, ScopeHash: hex.EncodeToString(scope[:])}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	reply, err := client.SearchCandidates(ctx, req)
	cancel()
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_SEARCH_RPC_UNAVAILABLE", err)
	}
	if !reply.Complete || reply.HasMore || len(reply.Candidates) != 0 || reply.PhysicalIndex != physical || reply.IndexGeneration != indexGeneration || len(reply.Continuation) == 0 {
		t.Fatal("ACTUAL_RPC_EMPTY_EOF_NOT_BOUND")
	}
	key, err := hex.DecodeString(config.Search.CursorKey)
	if err != nil {
		t.Fatal("ACTUAL_RPC_KEY_INVALID")
	}
	codec, err := domain.NewCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := codec.Open(reply.Continuation)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(anchor.Chain[:])
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := client.CloseCursor(closeCtx, &pb.CloseCursorRequest{Cursor: reply.Continuation, Scope: req.Scope}); err != nil {
			t.Error("ACTUAL_RPC_CURSOR_CLEANUP_FAILED")
		}
		if _, err := store.DB.Exec("DELETE FROM cursor_registry WHERE cursor_hash=?", hash[:]); err != nil {
			t.Error("ACTUAL_RPC_CURSOR_ROW_CLEANUP_FAILED")
		}
	})
	var count int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM cursor_registry WHERE cursor_hash=? AND state='OPEN'", hash[:]).Scan(&count); err != nil || count != 1 {
		t.Fatal("ACTUAL_RPC_CURSOR_NOT_PERSISTED", err)
	}
	// 只有Doc可调用候选；另真实Account证书不能自报SearchScope绕过method allowlist。
	account, err := platform.LoadConfig(filepath.Join(config.Root, ".runtime", "services", "account.json"))
	if err != nil {
		t.Fatal(err)
	}
	// 测试显式指向同一已配置Search地址，仍使用真正Account证书；不更改任何服务配置。
	account.Peers = map[string]string{"search": doc.Peers["search"]}
	other, err := platform.DialPeer(account, "search")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	_, err = pb.NewSearchInternalClient(other).SearchCandidates(ctx, req)
	cancel()
	if status.Code(err) != codes.PermissionDenied {
		t.Fatal("ACTUAL_NON_DOCUMENT_CANDIDATE_RPC_ACCEPTED", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	closed, err := client.CloseCursor(ctx, &pb.CloseCursorRequest{Cursor: reply.Continuation, Scope: req.Scope})
	cancel()
	if err != nil || closed.State != pb.OperationResult_SUCCEEDED {
		t.Fatal("ACTUAL_RPC_CLOSE_NOT_CONFIRMED", err)
	}
}
