package candidates

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func moveFixtureAlias(t *testing.T, alias, from, to string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"actions": []any{
		map[string]any{"remove": map[string]any{"index": from, "alias": alias}},
		map[string]any{"add": map[string]any{"index": to, "alias": alias, "is_write_index": true}},
	}})
	if err != nil {
		t.Fatal("ACTUAL_ALIAS_FIXTURE_ENCODING_FAILED")
	}
	request, err := http.NewRequest("POST", mustElasticEndpoint(t)+"/_aliases", bytes.NewReader(body))
	if err != nil {
		t.Fatal("ACTUAL_ALIAS_FIXTURE_REQUEST_FAILED")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal("ACTUAL_ALIAS_FIXTURE_UNAVAILABLE")
	}
	defer response.Body.Close()
	var receipt struct {
		Acknowledged bool `json:"acknowledged"`
		Errors       bool `json:"errors"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&receipt) != nil || !receipt.Acknowledged || receipt.Errors {
		t.Fatal("ACTUAL_ALIAS_FIXTURE_NOT_CONFIRMED")
	}
}

func TestActualStaleGenerationCannotQueryButCanCloseOriginalPIT(t *testing.T) {
	f := candidateFixture(t)
	f.seed(t, 1)
	request := fixtureRequest()
	first, err := f.usecase.SearchCandidates(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	hash := f.remember(t, first)
	nextPhysical := f.alias + "-g2"
	if err := f.sink.EnsureIndex(context.Background(), nextPhysical); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// 本测试不是完整重建实现；只切自有隔离alias及同库代次，再恢复确切旧绑定。
		physical, found, err := f.sink.ResolveAlias(context.Background(), f.alias)
		if err != nil || !found || (physical != f.physical && physical != nextPhysical) {
			t.Error("ACTUAL_OLD_GENERATION_CLEANUP_UNBOUND")
			return
		}
		if physical == nextPhysical {
			moveFixtureAlias(t, f.alias, nextPhysical, f.physical)
		}
		if _, err := f.store.DB.Exec("UPDATE index_generation SET physical_index=?,generation=1 WHERE alias_name=? AND physical_index IN (?,?)", f.physical, f.alias, f.physical, nextPhysical); err != nil {
			t.Error("ACTUAL_GENERATION_FIXTURE_RESTORE_FAILED")
		}
		if err := f.sink.DeleteOwnedIndex(context.Background(), nextPhysical); err != nil {
			t.Error("ACTUAL_GENERATION_INDEX_CLEANUP_FAILED")
		}
	})
	moveFixtureAlias(t, f.alias, f.physical, nextPhysical)
	result, err := f.store.DB.Exec("UPDATE index_generation SET physical_index=?,generation=2 WHERE alias_name=? AND physical_index=? AND generation=1", nextPhysical, f.alias, f.physical)
	if err != nil {
		t.Fatal("ACTUAL_GENERATION_SWITCH_FIXTURE_FAILED")
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		t.Fatal("ACTUAL_GENERATION_SWITCH_FIXTURE_NOT_EXACT")
	}
	if reply, err := f.usecase.SearchCandidates(context.Background(), requestAfter(t, request, domain.ActionReplay, first)); err == nil || err.Error() != "SEARCH_CURSOR_STALE" || reply != nil {
		t.Fatal("ACTUAL_OLD_GENERATION_QUERY_ACCEPTED", err)
	}
	closed, err := f.usecase.CloseCursor(context.Background(), &pb.CloseCursorRequest{Cursor: first.Continuation, Scope: request.Scope})
	if err != nil || closed.State != pb.OperationResult_SUCCEEDED || closed.Generation != 1 {
		t.Fatal("ACTUAL_OLD_GENERATION_CLOSE_REQUIRED_CURRENT_ALIAS", err)
	}
	var state, physical, pit string
	var generation uint64
	if err := f.store.DB.QueryRow("SELECT state,physical_index,index_generation,pit_handle FROM cursor_registry WHERE cursor_hash=?", hash[:]).Scan(&state, &physical, &generation, &pit); err != nil || state != "CLOSED" || physical != f.physical || generation != 1 || pit != "" {
		t.Fatal("ACTUAL_OLD_GENERATION_CLOSE_CHANGED_IMMUTABLE_BINDING", err)
	}
}
