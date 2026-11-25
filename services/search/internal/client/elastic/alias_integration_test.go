package elastic

import (
	"collaboration/platform"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualInitialAliasBindsOnlyExpectedOwnedPhysicalIndex(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(config.Search)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	alias := fmt.Sprintf("actual-alias-%d", time.Now().UnixNano())
	physical, other := alias+"-g1", alias+"-g2"
	t.Cleanup(func() {
		// test-only清理只解除本测试刚建立的确切绑定，没有通配符或remove_index。
		status, _, err := client.request(ctx, "DELETE", "/"+physical+"/_alias/"+alias, nil, 1<<20)
		if err != nil || (status != 200 && status != 404) {
			t.Error("ACTUAL_ALIAS_CLEANUP_FAILED")
		}
		for _, name := range []string{physical, other} {
			if err := client.DeleteOwnedIndex(ctx, name); err != nil {
				t.Error(err)
			}
		}
	})
	if err := client.EnsureInitialAlias(ctx, alias, physical); err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureInitialAlias(ctx, alias, physical); err != nil {
		t.Fatal(err)
	}
	actual, exists, err := client.ResolveAlias(ctx, alias)
	if err != nil || !exists || actual != physical {
		t.Fatal("ACTUAL_INITIAL_ALIAS_UNBOUND")
	}
	if err := client.EnsureInitialAlias(ctx, alias, other); err == nil {
		t.Fatal("EXISTING_ALIAS_SILENTLY_REBOUND")
	}
	actual, exists, err = client.ResolveAlias(ctx, alias)
	if err != nil || !exists || actual != physical {
		t.Fatal("REFUSED_ADOPTION_CHANGED_ALIAS")
	}
}
