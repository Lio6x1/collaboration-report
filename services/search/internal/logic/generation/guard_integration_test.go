package generation

import (
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	repository "collaboration/search/internal/repository/generation"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestActualGenerationGuardInitializesIdempotentlyAndHoldsRecordBarrier(t *testing.T) {
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
	sink, err := elastic.New(config.Search)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	ctx := context.Background()
	alias := fmt.Sprintf("actual-guard-%d", time.Now().UnixNano())
	physical := alias + "-g1"
	t.Cleanup(func() {
		// 精确删除本测试创建的alias；不为生产客户端增加只用于清理的接口。
		request, _ := http.NewRequestWithContext(ctx, "DELETE", config.Search.ElasticEndpoint+"/"+physical+"/_alias/"+alias, nil)
		response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
		if err != nil {
			t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
		} else {
			response.Body.Close()
			if response.StatusCode != 200 && response.StatusCode != 404 {
				t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
			}
		}
		if _, err := store.DB.ExecContext(ctx, "DELETE FROM index_generation WHERE alias_name=?", alias); err != nil {
			t.Error("ACTUAL_GENERATION_CLEANUP_FAILED")
		}
		if err := sink.DeleteOwnedIndex(ctx, physical); err != nil {
			t.Error(err)
		}
	})
	guard, err := New(store.DB, sink, alias, physical)
	if err != nil {
		t.Fatal(err)
	}
	if guard.Ready(ctx) == nil {
		t.Fatal("MISSING_GENERATION_WAS_READY")
	}
	if err := guard.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err := guard.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err := guard.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if err := guard.Do(ctx, func(ctx context.Context, target repository.Target) error {
		if target.Physical != physical || target.Generation != 1 {
			t.Fatal("ACTIVE_GUARD_TARGET_UNBOUND")
		}
		cutover, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		writer, err := store.DB.BeginTx(cutover, nil)
		if err != nil {
			t.Fatal("ACTUAL_CUTOVER_TRANSACTION_UNAVAILABLE")
		}
		defer writer.Rollback()
		if _, err := writer.ExecContext(cutover, "UPDATE index_generation SET generation=2,revision=2 WHERE alias_name=?", alias); err == nil || cutover.Err() == nil {
			t.Fatal("GUARD_RELEASED_GENERATION_BEFORE_RECORD_FINISHED")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var generation, revision uint64
	if store.DB.QueryRowContext(ctx, "SELECT generation,revision FROM index_generation WHERE alias_name=?", alias).Scan(&generation, &revision) != nil || generation != 1 || revision != 1 {
		t.Fatal("IDEMPOTENT_INITIALIZATION_OR_RECORD_CHANGED_GENERATION")
	}
}
