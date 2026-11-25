package elastic

import (
	"collaboration/platform"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualSwitchAliasAndInspectOriginalTarget(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil || config.Search == nil {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	client, err := New(config.Search)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	alias := fmt.Sprintf("actual-rebuild-alias-%d", time.Now().UnixNano())
	source, target := alias+"-g1", alias+"-g2"
	// 只清理由本场景创建的精确owner索引；生产alias及普通消费者保持运行。
	defer func() {
		for _, name := range []string{source, target} {
			status, _, err := client.request(ctx, "DELETE", "/"+name+"/_alias/"+alias, nil, 1<<20)
			if err != nil || (status != 200 && status != 404) {
				t.Error("ACTUAL_SWITCH_ALIAS_CLEANUP_FAILED")
				continue
			}
			if err := client.DeleteOwnedIndex(ctx, name); err != nil {
				t.Error(err)
			}
		}
	}()
	if err := client.EnsureInitialAlias(ctx, alias, source); err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureIndex(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := client.SwitchAlias(ctx, alias, source, target); err != nil {
		t.Fatal(err)
	}
	state, err := client.InspectAliasSwitch(ctx, alias, source, target)
	if err != nil || state != AliasAtTarget {
		t.Fatal("ACTUAL_SWITCH_TARGET_UNBOUND")
	}
	// 再次调用不能冒充又执行了一次切换，协调器须完成原SQL任务的恢复。
	if err := client.SwitchAlias(ctx, alias, source, target); err == nil || err.Error() != "SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED" {
		t.Fatal("ACTUAL_SWITCH_REPLAY_NOT_RECONCILED")
	}
}
