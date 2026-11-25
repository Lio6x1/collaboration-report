package elastic

import (
	"collaboration/platform"
	index "collaboration/search/internal/domain/index"
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func TestActualElasticUnsignedCompareTombstoneAndSameSequenceConflict(t *testing.T) {
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
	physical := fmt.Sprintf("search-cas-%d-%d", os.Getpid(), time.Now().UnixNano())
	ctx := context.Background()
	if err = client.EnsureIndex(ctx, physical); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.DeleteOwnedIndex(ctx, physical); err != nil {
			t.Error(err)
		}
	})
	first := index.Authority{Document: math.MaxUint64, Sequence: math.MaxInt64 + 1, Release: 7,
		Visible: true, Policy: "COLLABORATORS", Title: "正式标题", Text: "表格与代码中文检索", Digest: [32]byte{1},
		Chapters: []index.Chapter{{Unit: 29, Title: "章节", Text: "代码"}}}
	state, err := client.Apply(ctx, physical, first)
	if err != nil || state.Sequence != first.Sequence || state.Release != 7 || state.Digest != first.Digest {
		t.Fatal("ACTUAL_UNSIGNED_INDEX_NOT_APPLIED")
	}
	// 高位隐藏也保留墓碑，且整个新文档替换旧正文。旧事件不能借delete后的空ID复活。
	hidden := index.Authority{Document: first.Document, Sequence: math.MaxUint64, Policy: "PUBLIC", Digest: [32]byte{2}}
	state, err = client.Apply(ctx, physical, hidden)
	if err != nil || state.Sequence != math.MaxUint64 || state.Visible || state.Release != 0 || state.Digest != hidden.Digest {
		t.Fatal("ACTUAL_MAX_TOMBSTONE_NOT_APPLIED")
	}
	state, err = client.Apply(ctx, physical, first)
	if err != nil || state.Sequence != math.MaxUint64 || state.Visible {
		t.Fatal("OLDER_VISIBLE_EVENT_REVIVED_TOMBSTONE")
	}
	stored, err := client.Get(ctx, physical, first.Document)
	if err != nil || stored.Title != "" || stored.Text != "" || len(stored.Chapters) != 0 {
		t.Fatal("HIDDEN_INDEX_RETAINED_OLD_BODY")
	}
	if _, err = client.Apply(ctx, physical, hidden); err != nil {
		t.Fatal("IDENTICAL_ES_REPLAY_NOT_IDEMPOTENT")
	}
	hidden.Digest = [32]byte{3}
	if _, err = client.Apply(ctx, physical, hidden); err == nil || err.Error() != "SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT" {
		t.Fatal("ACTUAL_ES_SAME_SEQUENCE_CONFLICT_ACCEPTED")
	}
}
