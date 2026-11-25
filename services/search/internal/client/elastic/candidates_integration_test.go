package elastic

import (
	"collaboration/platform"
	index "collaboration/search/internal/domain/index"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"
)

func actualPITFixture(t *testing.T) (*Client, string) {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	// 只通过统一配置加载器读取owner/endpoint，不输出私有配置。
	configuration, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(configuration.Search)
	if err != nil {
		t.Fatal(err)
	}
	physical := fmt.Sprintf("search-pit-%d-%d", os.Getpid(), time.Now().UnixNano())
	if err := client.EnsureIndex(context.Background(), physical); err != nil {
		client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// 删除前仍校验owner/schema且禁止已绑定alias；只清理本测试精确名称。
		if err := client.DeleteOwnedIndex(context.Background(), physical); err != nil {
			t.Error(err)
		}
		client.Close()
	})
	return client, physical
}

func actualPITRefresh(t *testing.T, client *Client, physical string) {
	t.Helper()
	status, data, err := client.request(context.Background(), "POST", "/"+physical+"/_refresh", nil, 1<<20)
	var response struct {
		Shards *pitShards `json:"_shards"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &response) != nil || !response.Shards.complete() {
		t.Fatal("ACTUAL_PIT_FIXTURE_REFRESH_INCOMPLETE")
	}
}

func TestActualPITChinesePhraseMatchesWholeSearchTerm(t *testing.T) {
	client, physical := actualPITFixture(t)
	ctx := context.Background()
	values := []index.Authority{
		{Document: 1, Title: "正文命中", Text: "先检查设定，设定在同一文档出现多次"},
		{Document: 2, Title: "固定版本", Text: "固定正文和约定"},
		{Document: 3, Title: "设想", Text: "设想与固定版本相互独立"},
		{Document: 4, Title: "设定说明", Text: "关键词位于标题"},
		{Document: 5, Title: "章节命中", Text: "正文", Chapters: []index.Chapter{{Unit: 1, Title: "设定与范围", Text: "正文"}}},
	}
	for _, value := range values {
		value.Sequence, value.Release, value.Visible, value.Policy = 1, value.Document+100, true, "PUBLIC"
		value.Digest = [32]byte{byte(value.Document)}
		if _, err := client.Apply(ctx, physical, value); err != nil {
			t.Fatal(err)
		}
	}
	actualPITRefresh(t, client, physical)
	pit, err := client.OpenPIT(ctx, physical, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { actualPITCleanup(t, client, pit, "") })
	batch, err := client.Candidates(ctx, pit, "设定", nil, 20, 10*time.Second)
	if batch.PIT != "" {
		pit = batch.PIT
	}
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uint64, 0, len(batch.Candidates))
	for _, value := range batch.Candidates {
		ids = append(ids, value.DocumentId)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	// 中文标准分词拆出“设”“定”后，不能因只命中“固定”的一个字就返回。
	// 关键词可以命中正式标题、正文或章节；同一文档出现多次仍只返回一次。
	if !reflect.DeepEqual(ids, []uint64{1, 4, 5}) || batch.More {
		t.Fatalf("完整中文关键词应只命中三篇独立文档，实际=%v more=%v", ids, batch.More)
	}
}

func actualPITApply(t *testing.T, client *Client, physical string, document, sequence uint64, visible bool) {
	t.Helper()
	authority := index.Authority{Document: document, Sequence: sequence, Policy: "PUBLIC", Digest: [32]byte{1}, Visible: visible}
	if visible {
		authority.Release = ^uint64(0)
		authority.Title = "pitfixtureword"
		authority.Text = "pitfixtureword"
		authority.Chapters = []index.Chapter{{Unit: 1, Title: "pitfixtureword", Text: "chapter"}}
	}
	if _, err := client.Apply(context.Background(), physical, authority); err != nil {
		t.Fatal(err)
	}
}

func actualPITDefinitelyMissing(t *testing.T, client *Client, pit, deletedPhysical string) bool {
	t.Helper()
	// 404本身不能证明原句柄已不存在：必须是ES明确的search_context_missing，
	// 其他代理404/未知结果不能伪装成主动close succeeded。
	status, data, err := client.request(context.Background(), "POST", "/_search?allow_partial_search_results=false", map[string]any{
		"size": 0, "pit": map[string]any{"id": pit, "keep_alive": "1ms"},
	}, 1<<20)
	var response struct {
		Error struct {
			Type   string `json:"type"`
			Index  string `json:"index"`
			Causes []struct {
				Type  string `json:"type"`
				Index string `json:"index"`
			} `json:"root_cause"`
		} `json:"error"`
	}
	if err != nil || json.Unmarshal(data, &response) != nil {
		return false
	}
	// 只记录固定协议状态/错误类型，不能把PIT句柄或完整ES错误正文写入证据。
	t.Logf("PIT absence probe status=%d type=%s", status, response.Error.Type)
	for _, cause := range response.Error.Causes {
		t.Logf("PIT absence root type=%s", cause.Type)
	}
	if status != 404 {
		return false
	}
	// 已由DeleteOwnedIndex核验归属并确认删除的精确来源，ES以index_not_found回复原PIT。
	// 必须绑定同一物理名称与所有root；其他404/尚未确认的删除仍是未知。
	if deletedPhysical != "" && response.Error.Type == "index_not_found_exception" &&
		response.Error.Index == deletedPhysical && len(response.Error.Causes) > 0 {
		for _, cause := range response.Error.Causes {
			if cause.Type != "index_not_found_exception" || cause.Index != deletedPhysical {
				return false
			}
		}
		return true
	}
	if response.Error.Type == "search_context_missing_exception" {
		return true
	}
	if response.Error.Type != "search_phase_execution_exception" || len(response.Error.Causes) == 0 {
		return false
	}
	for _, cause := range response.Error.Causes {
		if cause.Type != "search_context_missing_exception" {
			return false
		}
	}
	return true
}

func actualPITCleanup(t *testing.T, client *Client, pit, deletedPhysical string) {
	t.Helper()
	if pit == "" {
		return
	}
	if err := client.ClosePIT(context.Background(), pit); err != nil && !actualPITDefinitelyMissing(t, client, pit, deletedPhysical) {
		t.Error("ACTUAL_PIT_CLOSE_OR_CONFIRMED_ABSENCE_REQUIRED", err)
	}
}

func TestActualPITFullTraversalReplayAndSnapshotWithoutSwallowingProbe(t *testing.T) {
	client, physical := actualPITFixture(t)
	expected := []string{}
	for document := uint64(1); document <= 205; document++ {
		actualPITApply(t, client, physical, document, uint64(1)<<63, true)
		expected = append(expected, strconv.FormatUint(document, 10))
	}
	actualPITApply(t, client, physical, ^uint64(0), ^uint64(0), true)
	expected = append(expected, strconv.FormatUint(^uint64(0), 10))
	// keyword document_id按十进制字符串排序，而非数值排序；与冻结ES排序一致。
	sort.Strings(expected)
	actualPITRefresh(t, client, physical)
	deadline := time.Now().Add(30 * time.Second)
	pit, err := client.OpenPIT(context.Background(), physical, time.Until(deadline))
	if err != nil {
		if pit != "" {
			actualPITCleanup(t, client, pit, "")
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		actualPITCleanup(t, client, pit, "")
	})
	// PIT打开之后的新正式版/隐藏墓碑不能改写这条链的快照；新开链才看到它们。
	actualPITApply(t, client, physical, 1, ^uint64(0), false)
	actualPITApply(t, client, physical, 999, uint64(1)<<63, true)
	actualPITRefresh(t, client, physical)
	var after json.RawMessage
	actual := []string{}
	seen := map[uint64]bool{}
	for page := 0; ; page++ {
		if page > len(expected)/100+1 {
			t.Fatal("ACTUAL_PIT_TRAVERSAL_DID_NOT_FINISH")
		}
		batch, err := client.Candidates(context.Background(), pit, "pitfixtureword", after, 100, time.Until(deadline))
		if batch.PIT != "" {
			pit = batch.PIT
		}
		if err != nil {
			t.Fatal(err)
		}
		if page == 0 {
			if len(batch.Candidates) != 100 || !batch.More {
				t.Fatal("ACTUAL_B_PLUS_ONE_NOT_PROBED")
			}
			replay, err := client.Candidates(context.Background(), pit, "pitfixtureword", nil, 100, time.Until(deadline))
			if replay.PIT != "" {
				pit = replay.PIT
			}
			if err != nil || !reflect.DeepEqual(batch.Candidates, replay.Candidates) || string(batch.End) != string(replay.End) || batch.More != replay.More {
				t.Fatal("ACTUAL_PIT_SAME_ANCHOR_REPLAY_CHANGED", err)
			}
		}
		for _, candidate := range batch.Candidates {
			if seen[candidate.DocumentId] || candidate.PhysicalIndex != physical || candidate.ReleaseId != ^uint64(0) ||
				(candidate.DocumentId == ^uint64(0) && candidate.AggregateSeq != ^uint64(0)) {
				t.Fatal("ACTUAL_PIT_BINDING_DUPLICATE_OR_UINT64_LOSS")
			}
			seen[candidate.DocumentId] = true
			actual = append(actual, strconv.FormatUint(candidate.DocumentId, 10))
		}
		after = batch.End
		if !batch.More {
			break
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("ACTUAL_PIT_SNAPSHOT_TRAVERSAL_LOST_OR_ADDED_DOCUMENTS")
	}
	eof, err := client.Candidates(context.Background(), pit, "pitfixtureword", after, 100, time.Until(deadline))
	if eof.PIT != "" {
		pit = eof.PIT
	}
	if err != nil || len(eof.Candidates) != 0 || eof.More || string(eof.End) != string(after) {
		t.Fatal("ACTUAL_PIT_EMPTY_EOF_CHANGED_POSITION", err)
	}
	if err := client.ClosePIT(context.Background(), pit); err != nil {
		t.Fatal(err)
	}
	closed := pit
	pit = ""
	if _, err := client.Candidates(context.Background(), closed, "pitfixtureword", nil, 100, time.Second); err == nil {
		t.Fatal("ACTUAL_CLOSED_PIT_QUERY_SUCCEEDED")
	}
}

func TestActualPITDeletedPhysicalAndCancelledRequestRejectSuccess(t *testing.T) {
	client, physical := actualPITFixture(t)
	actualPITApply(t, client, physical, 1, 1, true)
	actualPITRefresh(t, client, physical)
	pit, err := client.OpenPIT(context.Background(), physical, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	deletedPhysical := ""
	t.Cleanup(func() {
		actualPITCleanup(t, client, pit, deletedPhysical)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if batch, err := client.Candidates(ctx, pit, "pitfixtureword", nil, 100, time.Second); err == nil || len(batch.Candidates) != 0 {
		t.Fatal("ACTUAL_CANCELLED_PIT_QUERY_SUCCEEDED")
	}
	// 删除精确隔离索引使该PIT来源失效；不能退回alias/新索引，也不能把分片失败变成空EOF。
	if err := client.DeleteOwnedIndex(context.Background(), physical); err != nil {
		t.Fatal(err)
	}
	deletedPhysical = physical
	if batch, err := client.Candidates(context.Background(), pit, "pitfixtureword", nil, 100, time.Second); err == nil || len(batch.Candidates) != 0 {
		t.Fatal("ACTUAL_DELETED_PIT_SOURCE_RETURNED_SUCCESS")
	}
}

func TestActualPITNaturalExpirationWithoutRenewingLifetime(t *testing.T) {
	client, physical := actualPITFixture(t)
	actualPITApply(t, client, physical, 1, 1, true)
	actualPITRefresh(t, client, physical)
	pit, err := client.OpenPIT(context.Background(), physical, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		actualPITCleanup(t, client, pit, "")
	})
	// ES自然清理由节点reaper异步执行；等待超过默认轮次，不轮询查询给原PIT续命。
	// 此处只是隔离资源的观察等待，不提高HTTP/RPC期限或修改实际节点配置。
	time.Sleep(90 * time.Second)
	if batch, err := client.Candidates(context.Background(), pit, "pitfixtureword", nil, 100, time.Second); err == nil || len(batch.Candidates) != 0 {
		t.Fatal("ACTUAL_NATURALLY_EXPIRED_PIT_RETURNED_SUCCESS")
	}
	if !actualPITDefinitelyMissing(t, client, pit, "") {
		t.Fatal("ACTUAL_NATURAL_EXPIRY_NOT_CONFIRMED_BY_ES")
	}
}
