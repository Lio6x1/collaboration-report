//go:build search_partial

package elastic

import (
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

type partialTopology struct {
	Version  int    `json:"version"`
	Fixture  string `json:"fixture"`
	Owner    string `json:"owner"`
	Project  string `json:"project"`
	Endpoint string `json:"endpoint"`
}

func TestPartialTopologyMetadataAndShardSettings(t *testing.T) {
	// go test的cwd是包目录，不是启动go test的services/search目录。
	base, err := partialTopologyBase()
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("../../../../../")
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil || !strings.EqualFold(cwd, filepath.Join(repository, "services", "search", "internal", "client", "elastic")) ||
		!strings.EqualFold(base, filepath.Join(repository, ".runtime", "search-partial-test")) {
		t.Fatal("PARTIAL_REAL_PACKAGE_CWD_BINDING_INVALID")
	}
	valid := `{"version":1,"fixture":"0123456789abcdef0123456789abcdef","owner":"test-owner","project":"collab-search-partial-0123456789abcdef0123456789abcdef","endpoint":"http://127.0.0.1:19291"}`
	if _, err := decodePartialTopology([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		strings.Replace(valid, `"version":1`, `"version":2`, 1),
		strings.Replace(valid, `collab-search-partial-`, `production-`, 1),
		strings.Replace(valid, `127.0.0.1`, `example.test`, 1),
		strings.Replace(valid, `19291`, `0`, 1),
		strings.Replace(valid, `http://`, `http://user:secret@`, 1),
		strings.TrimSuffix(valid, "}") + `,"secret":"value"}`,
		valid + ` {}`,
	} {
		if _, err := decodePartialTopology([]byte(value)); err == nil {
			t.Fatal("PARTIAL_TOPOLOGY_INVALID_ACCEPTED")
		}
	}
	settings := partialShardSettings()
	// 两个零副本主分片加每节点硬上限，避免B下线后A接管全部分片而失去反例。
	if settings["number_of_shards"] != 2 || settings["number_of_replicas"] != 0 || settings["index.routing.allocation.total_shards_per_node"] != 1 {
		t.Fatal("PARTIAL_SHARD_SETTINGS_INVALID")
	}
}

func TestPartialContextTerminalRequiresCompleteOwnedNodes(t *testing.T) {
	valid := `{"_nodes":{"total":2,"successful":2,"failed":0},"nodes":{"node-a":{"name":"partial-a","indices":{"search":{"open_contexts":0}}},"node-b":{"name":"partial-b","indices":{"search":{"open_contexts":0}}}}}`
	if !partialContextsGone([]byte(valid)) {
		t.Fatal("PARTIAL_CONTEXT_ZERO_PROOF_REJECTED")
	}
	// 普通404、缺失/不完整统计、其他节点、任何剩余context都不能证明终态。
	for _, value := range []string{
		`{"error":{"type":"search_context_missing_exception"},"status":404}`,
		strings.Replace(valid, `"total":2`, `"total":1`, 1),
		strings.Replace(valid, `"successful":2`, `"successful":1`, 1),
		strings.Replace(valid, `"failed":0`, `"failed":1`, 1),
		strings.Replace(valid, `"failed":0`, `"missing":0`, 1),
		strings.Replace(valid, `"open_contexts":0`, `"open_contexts":1`, 1),
		strings.Replace(valid, `"open_contexts":0`, `"open_contexts":null`, 1),
		strings.Replace(valid, `"open_contexts":0`, `"missing":0`, 1),
		strings.Replace(valid, `"partial-b"`, `"partial-a"`, 1),
		strings.Replace(valid, `"partial-b"`, `"production"`, 1),
		valid + ` {}`,
	} {
		if partialContextsGone([]byte(value)) {
			t.Fatal("PARTIAL_CONTEXT_UNKNOWN_ACCEPTED")
		}
	}
}

// 本测试只由search_partial独立标签及隔离环境脚本启动，不属于常规模块环境。
// 显式选择此真实关口时，没有隔离元数据必须失败，不能用SKIP冒充真实验收。
func TestActualPITRejectsRealPartialShardTopology(t *testing.T) {
	directory := os.Getenv("COLLAB_TEST_SEARCH_PARTIAL_DIRECTORY")
	if directory == "" {
		t.Fatal("ACTUAL_PARTIAL_TOPOLOGY_REQUIRED")
	}
	data, err := os.ReadFile(filepath.Join(directory, "topology.json"))
	if err != nil {
		t.Fatal("ACTUAL_PARTIAL_TOPOLOGY_READ_FAILED")
	}
	topology, err := decodePartialTopology(data)
	if err != nil {
		t.Fatal(err)
	}
	base, err := partialTopologyBase()
	if err != nil || !strings.EqualFold(filepath.Dir(directory), base) || filepath.Base(directory) != topology.Fixture {
		t.Fatal("ACTUAL_PARTIAL_DIRECTORY_INVALID")
	}
	client, err := New(&platform.SearchConfig{Owner: topology.Owner, ElasticEndpoint: topology.Endpoint})
	if err != nil {
		t.Fatal(err)
	}
	physical := "search-partial-" + topology.Fixture
	// 每条实际PIT链只保留最新句柄；不能逐个关闭已轮换的旧ID并把重复关闭当失败。
	handles := map[string]string{}
	created := false
	t.Cleanup(func() {
		// 即使断言Fatal，也先恢复精确B；恢复预算属于夹具，不给原PIT续命。
		if err := partialStage(directory, "restore", 95*time.Second); err != nil {
			t.Error(err)
			client.Close()
			return
		}
		if created {
			if err := partialWaitShards(client, physical, true, 30*time.Second); err != nil {
				t.Error(err)
			}
			// 新恢复链必须由生产ClosePIT明确确认。旧故障PIT可能仍携带已丢失的
			// B上下文/缺分片引用；其Close未知与最终context已不存在是两种事实。
			closedHandles := map[string]bool{}
			for _, role := range []string{"recovery", "baseline", "diagnostic", "failed-open"} {
				handle := handles[role]
				if handle == "" {
					continue
				}
				if closedHandles[handle] {
					t.Logf("PIT cleanup role=%s duplicate_latest_handle=true; no second close", role)
					continue
				}
				closedHandles[handle] = true
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := client.ClosePIT(ctx, handle); err != nil {
					t.Logf("PIT cleanup role=%s close=unknown; not an acknowledged close", role)
					if role == "recovery" {
						t.Error("ACTUAL_PARTIAL_RECOVERY_CLOSE_UNKNOWN")
					}
				} else {
					t.Logf("PIT cleanup role=%s close=acknowledged", role)
				}
				cancel()
			}
			// 私有隔离集群仅有本夹具链。完整A/B统计都为零可证明所有旧context
			// 最终消失；不根据通用404、缺统计或关闭错误本身作出终态判断。
			// 90秒仅观察GC/回收，不搜索旧PIT，不续keep_alive或扩大业务30秒期限。
			if err := partialWaitContextsGone(client, 90*time.Second); err != nil {
				t.Error(err)
			} else {
				t.Log("isolated nodes total=2 successful=2 failed=0; partial-a/partial-b open_contexts=0; old handle Close may remain unknown")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := client.DeleteOwnedIndex(ctx, physical); err != nil {
				t.Error(err)
			}
			cancel()
		}
		client.Close()
	})
	status, _, err := client.request(context.Background(), "PUT", "/"+physical, map[string]any{
		"settings": partialShardSettings(),
		"mappings": map[string]any{"dynamic": "strict", "_meta": map[string]any{
			"application": "collaboration-report", "owner": topology.Owner, "schema": schemaVersion,
		}, "properties": properties()},
	}, 1<<20)
	if err != nil || status != 200 {
		t.Fatal("ACTUAL_PARTIAL_INDEX_CREATE_FAILED")
	}
	created = true
	if err := client.EnsureIndex(context.Background(), physical); err != nil {
		t.Fatal(err)
	}
	if err := partialWaitShards(client, physical, true, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	// 默认文档routing就是_id。确定各分片的一项真实匹配文档，不靠随机散列碰运气。
	seen := map[int]bool{}
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer seedCancel()
	for document := uint64(1); document <= 100 && len(seen) < 2; document++ {
		status, data, err := client.request(seedCtx, "GET", "/"+physical+"/_search_shards?routing="+strconv.FormatUint(document, 10), nil, 1<<20)
		var routing struct {
			Shards [][]struct {
				Shard int `json:"shard"`
			} `json:"shards"`
		}
		if err != nil || status != 200 || json.Unmarshal(data, &routing) != nil || len(routing.Shards) != 1 || len(routing.Shards[0]) != 1 {
			t.Fatal("ACTUAL_PARTIAL_ROUTING_INVALID")
		}
		shard := routing.Shards[0][0].Shard
		if shard < 0 || shard > 1 {
			t.Fatal("ACTUAL_PARTIAL_ROUTING_INVALID")
		}
		if !seen[shard] {
			actualPITApply(t, client, physical, document, 1, true)
			seen[shard] = true
		}
	}
	if len(seen) != 2 {
		t.Fatal("ACTUAL_PARTIAL_BOTH_SHARDS_REQUIRED")
	}
	actualPITRefresh(t, client, physical)
	// 故障、诊断和生产调用共用此原30秒绝对期限，不因停止容器或诊断请求重置。
	until := time.Now().Add(30 * time.Second)
	businessCtx, businessCancel := context.WithDeadline(context.Background(), until)
	defer businessCancel()
	pit, err := client.OpenPIT(context.Background(), physical, time.Until(until))
	if pit != "" {
		handles["baseline"] = pit
	}
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := client.Candidates(context.Background(), pit, "pitfixtureword", nil, 10, time.Until(until))
	if baseline.PIT != "" {
		pit = baseline.PIT
		handles["baseline"] = pit
	}
	if err != nil || len(baseline.Candidates) != 2 || baseline.More {
		t.Fatal("ACTUAL_PARTIAL_BASELINE_INVALID")
	}
	if err := partialStage(directory, "fault", time.Until(until)); err != nil {
		t.Fatal(err)
	}
	remaining := time.Until(until)
	if remaining > 5*time.Second {
		remaining = 5 * time.Second
	}
	if err := partialWaitShards(client, physical, false, remaining); err != nil {
		t.Fatal(err)
	}
	broken, err := client.Candidates(context.Background(), pit, "pitfixtureword", nil, 10, time.Until(until))
	if broken.PIT != "" {
		handles["baseline"] = broken.PIT
	}
	if err == nil || err.Error() != "SEARCH_ES_CANDIDATES_INCOMPLETE" || len(broken.Candidates) != 0 || len(broken.End) != 0 || broken.More {
		t.Fatal("ACTUAL_PARTIAL_CANDIDATES_EXPOSED")
	}
	t.Log("original complete PIT rejected after one real data node stopped")
	// true仅是诊断对照，证明健康分片仍真实有结果；这些hits从不作为产品成功输出。
	keepAlive, err := pitRemaining(time.Until(until))
	if err != nil {
		t.Fatal("ACTUAL_PARTIAL_ORIGINAL_BUDGET_EXHAUSTED")
	}
	status, data, err = client.request(businessCtx, "POST", "/"+physical+"/_pit?allow_partial_search_results=true&keep_alive="+url.QueryEscape(keepAlive), nil, 1<<20)
	var partial struct {
		ID     string     `json:"id"`
		Shards *pitShards `json:"_shards"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &partial) != nil || !validPIT(partial.ID) || !partialSummary(partial.Shards) {
		t.Fatal("ACTUAL_PARTIAL_DIAGNOSTIC_OPEN_REQUIRED")
	}
	handles["diagnostic"] = partial.ID
	keepAlive, err = pitRemaining(time.Until(until))
	if err != nil {
		t.Fatal("ACTUAL_PARTIAL_ORIGINAL_BUDGET_EXHAUSTED")
	}
	status, data, err = client.request(businessCtx, "POST", "/_search?allow_partial_search_results=true", map[string]any{
		"size": 10, "pit": map[string]any{"id": partial.ID, "keep_alive": keepAlive},
		"query": map[string]any{"match": map[string]string{"text": "pitfixtureword"}},
	}, 1<<20)
	var diagnostic struct {
		PIT    string     `json:"pit_id"`
		Shards *pitShards `json:"_shards"`
		Hits   struct {
			Hits []json.RawMessage `json:"hits"`
		} `json:"hits"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &diagnostic) != nil || !validPIT(diagnostic.PIT) || !partialSummary(diagnostic.Shards) || len(diagnostic.Hits.Hits) != 1 {
		t.Fatal("ACTUAL_PARTIAL_HEALTHY_HIT_REQUIRED")
	}
	if validPIT(diagnostic.PIT) {
		partial.ID = diagnostic.PIT
		handles["diagnostic"] = partial.ID
	}
	t.Log("real partial diagnostic returned one healthy hit; no response fabrication")
	broken, err = client.Candidates(context.Background(), partial.ID, "pitfixtureword", nil, 10, time.Until(until))
	if broken.PIT != "" {
		handles["diagnostic"] = broken.PIT
	}
	if err == nil || err.Error() != "SEARCH_ES_CANDIDATES_INCOMPLETE" || len(broken.Candidates) != 0 || len(broken.End) != 0 || broken.More {
		t.Fatal("ACTUAL_PARTIAL_DIAGNOSTIC_PIT_EXPOSED")
	}
	opened, err := client.OpenPIT(context.Background(), physical, time.Until(until))
	if opened != "" {
		handles["failed-open"] = opened
	}
	if err == nil || (err.Error() != "SEARCH_ES_PIT_OPEN_UNKNOWN" && err.Error() != "SEARCH_ES_PIT_OPEN_INCOMPLETE") {
		t.Fatal("ACTUAL_PARTIAL_OPEN_SUCCEEDED")
	}
	if time.Now().After(until) {
		t.Fatal("ACTUAL_PARTIAL_ORIGINAL_BUDGET_EXHAUSTED")
	}
	if err := partialStage(directory, "restore", 95*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := partialWaitShards(client, physical, true, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	// 恢复后是新的独立验证链，不能宣称B重启会复原旧PIT上下文。
	recoveredUntil := time.Now().Add(30 * time.Second)
	recovered, err := client.OpenPIT(context.Background(), physical, time.Until(recoveredUntil))
	if recovered != "" {
		handles["recovery"] = recovered
	}
	if err != nil {
		t.Fatal(err)
	}
	batch, err := client.Candidates(context.Background(), recovered, "pitfixtureword", nil, 10, time.Until(recoveredUntil))
	if batch.PIT != "" {
		handles["recovery"] = batch.PIT
	}
	if err != nil || len(batch.Candidates) != 2 || batch.More {
		t.Fatal("ACTUAL_PARTIAL_RESTORE_INVALID")
	}
	t.Log("exact data node restored; fresh PIT returned both original documents")
}

func partialShardSettings() map[string]any {
	return map[string]any{"number_of_shards": 2, "number_of_replicas": 0, "index.routing.allocation.total_shards_per_node": 1}
}

func partialTopologyBase() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", errors.New("PARTIAL_REPOSITORY_ROOT_UNKNOWN")
	}
	// 用正式workspace/contracts/Search模块三哨兵定位根；不依赖调用者的启动cwd，
	// 也不把包级internal目录误当成controller维护的仓库.runtime目录。
	for {
		workspace, workspaceErr := os.Stat(filepath.Join(current, "go.work"))
		contracts, contractsErr := os.ReadFile(filepath.Join(current, "contracts", "go.mod"))
		search, searchErr := os.ReadFile(filepath.Join(current, "services", "search", "go.mod"))
		if workspaceErr == nil && workspace.Mode().IsRegular() && contractsErr == nil && searchErr == nil &&
			strings.TrimSpace(strings.SplitN(string(contracts), "\n", 2)[0]) == "module collaboration/contracts" &&
			strings.TrimSpace(strings.SplitN(string(search), "\n", 2)[0]) == "module collaboration/search" {
			return filepath.Join(current, ".runtime", "search-partial-test"), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("PARTIAL_REPOSITORY_ROOT_UNKNOWN")
		}
		current = parent
	}
}

func partialSummary(shards *pitShards) bool {
	return shards != nil && shards.Total != nil && shards.Successful != nil && shards.Failed != nil &&
		*shards.Total == 2 && *shards.Successful == 1 && *shards.Failed == 1
}

func partialContextsGone(data []byte) bool {
	var statistics struct {
		Summary struct {
			Total      *uint64 `json:"total"`
			Successful *uint64 `json:"successful"`
			Failed     *uint64 `json:"failed"`
		} `json:"_nodes"`
		Nodes map[string]struct {
			Name    string `json:"name"`
			Indices struct {
				Search struct {
					Open *uint64 `json:"open_contexts"`
				} `json:"search"`
			} `json:"indices"`
		} `json:"nodes"`
	}
	if json.Unmarshal(data, &statistics) != nil || statistics.Summary.Total == nil || statistics.Summary.Successful == nil ||
		statistics.Summary.Failed == nil || *statistics.Summary.Total != 2 || *statistics.Summary.Successful != 2 ||
		*statistics.Summary.Failed != 0 || len(statistics.Nodes) != 2 {
		return false
	}
	names := map[string]bool{}
	for id, node := range statistics.Nodes {
		if id == "" || (node.Name != "partial-a" && node.Name != "partial-b") || names[node.Name] ||
			node.Indices.Search.Open == nil || *node.Indices.Search.Open != 0 {
			return false
		}
		names[node.Name] = true
	}
	return names["partial-a"] && names["partial-b"]
}

func partialWaitContextsGone(client *Client, budget time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	for ctx.Err() == nil {
		status, data, err := client.request(ctx, "GET", "/_nodes/stats/indices/search?filter_path=_nodes,nodes.*.name,nodes.*.indices.search.open_contexts", nil, 1<<20)
		if err == nil && status == 200 && partialContextsGone(data) {
			return nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("ACTUAL_PARTIAL_CONTEXT_TERMINAL_ZERO_UNCONFIRMED")
}

func decodePartialTopology(data []byte) (partialTopology, error) {
	var value partialTopology
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if len(data) > 2048 || decoder.Decode(&value) != nil {
		return value, errors.New("PARTIAL_TOPOLOGY_INVALID")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || value.Version != 1 || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(value.Fixture) ||
		!ownerName.MatchString(value.Owner) || value.Project != "collab-search-partial-"+value.Fixture {
		return value, errors.New("PARTIAL_TOPOLOGY_INVALID")
	}
	u, err := url.Parse(value.Endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return value, errors.New("PARTIAL_TOPOLOGY_INVALID")
	}
	host, port, err := net.SplitHostPort(u.Host)
	number, parseErr := strconv.ParseUint(port, 10, 16)
	if err != nil || parseErr != nil || host != "127.0.0.1" || number == 0 || strconv.FormatUint(number, 10) != port {
		return value, errors.New("PARTIAL_TOPOLOGY_INVALID")
	}
	return value, nil
}

func partialStage(directory, name string, budget time.Duration) error {
	if name != "fault" && name != "restore" {
		return errors.New("PARTIAL_STAGE_INVALID")
	}
	if budget <= 0 {
		return errors.New("PARTIAL_STAGE_BUDGET_EXHAUSTED")
	}
	if err := os.WriteFile(filepath.Join(directory, name+".request"), []byte("1\n"), 0600); err != nil {
		return errors.New("PARTIAL_STAGE_WRITE_FAILED")
	}
	until := time.Now().Add(budget)
	for time.Now().Before(until) {
		if data, err := os.ReadFile(filepath.Join(directory, name+".applied")); err == nil && string(data) == "1\n" {
			return nil
		}
		if _, err := os.Stat(filepath.Join(directory, "controller.failed")); err == nil {
			return errors.New("PARTIAL_CONTROLLER_FAILED")
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("PARTIAL_STAGE_BUDGET_EXHAUSTED")
}

func partialWaitShards(client *Client, physical string, complete bool, budget time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	for ctx.Err() == nil {
		status, data, err := client.request(ctx, "GET", "/_cat/shards/"+physical+"?format=json&h=shard,prirep,state,node", nil, 1<<20)
		var shards []struct {
			Shard   string `json:"shard"`
			Primary string `json:"prirep"`
			State   string `json:"state"`
			Node    string `json:"node"`
		}
		if err == nil && status == 200 && json.Unmarshal(data, &shards) == nil && len(shards) == 2 {
			started, missing := 0, 0
			nodes := map[string]bool{}
			ids := map[string]bool{}
			for _, shard := range shards {
				if shard.Primary != "p" || (shard.Shard != "0" && shard.Shard != "1") || ids[shard.Shard] {
					return errors.New("PARTIAL_SHARD_STATE_INVALID")
				}
				ids[shard.Shard] = true
				if shard.State == "STARTED" {
					started++
					nodes[shard.Node] = true
				}
				if shard.State == "UNASSIGNED" {
					missing++
				}
			}
			if complete && started == 2 && len(nodes) == 2 && nodes["partial-a"] && nodes["partial-b"] {
				return nil
			}
			if !complete && started == 1 && missing == 1 && nodes["partial-a"] {
				return nil
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("PARTIAL_SHARD_STATE_BUDGET_EXHAUSTED")
}
