package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pitTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{endpoint: server.URL, http: server.Client()}
}

func pitHit(document string, shard int) map[string]any {
	return map[string]any{"_index": "owned-g1", "_id": document,
		"_source": map[string]any{"document_id": document, "release_id": "18446744073709551615", "sequence": "18446744073709551615"},
		"sort":    json.RawMessage(fmt.Sprintf(`[1.2500e+2,"%s",%d]`, document, shard))}
}

func pitResponse(hits []any) map[string]any {
	return map[string]any{"pit_id": "latest", "timed_out": false,
		"_shards": map[string]any{"total": 2, "successful": 2, "skipped": 0, "failed": 0},
		"hits":    map[string]any{"hits": hits}}
}

func TestPITCandidatesLookaheadLatestHandleAndOriginalSort(t *testing.T) {
	// 第101项只探测More；保存第100项的位置，下一批才能真正返回原探测项。
	client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/_search" || r.URL.Query().Get("allow_partial_search_results") != "false" {
			t.Error("候选请求必须禁用partial且不经alias")
		}
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("无效请求")
		}
		if string(body["size"]) != "101" || string(body["track_total_hits"]) != "false" ||
			string(body["_source"]) != `["document_id","release_id","sequence"]` ||
			string(body["sort"]) != `[{"_score":"desc"},{"document_id":"asc"},{"_shard_doc":"asc"}]` {
			t.Error("候选最小source/固定排序/B+1被改变")
		}
		// 字段名keep_alive需要显式绑定，不能把PIT请求当成普通索引查询。
		var pit struct {
			ID        string `json:"id"`
			KeepAlive string `json:"keep_alive"`
		}
		if json.Unmarshal(body["pit"], &pit) != nil || pit.KeepAlive != "1999ms" {
			t.Error("keep_alive必须向下取整且不续绝对期限")
		}
		hits := []any{}
		if len(body["search_after"]) == 0 {
			if pit.ID != "initial" {
				t.Error("首次PIT错误")
			}
			for i := 1; i <= 101; i++ {
				hits = append(hits, pitHit(fmt.Sprintf("%03d", i+100), i))
			}
		} else {
			if pit.ID != "latest" || string(body["search_after"]) != `[1.2500e+2,"200",100]` {
				t.Error("未使用确认尾部/最新PIT")
			}
			hits = append(hits, pitHit("201", 101), pitHit("18446744073709551615", 102))
		}
		_ = json.NewEncoder(w).Encode(pitResponse(hits))
	})
	first, err := client.Candidates(context.Background(), "initial", "标题", nil, 100, 1999*time.Millisecond+999*time.Microsecond)
	if err != nil || first.PIT != "latest" || len(first.Candidates) != 100 || !first.More || string(first.End) != `[1.2500e+2,"200",100]` {
		t.Fatalf("首批错误: %v %+v", err, first)
	}
	second, err := client.Candidates(context.Background(), first.PIT, "标题", first.End, 100, 1999*time.Millisecond+999*time.Microsecond)
	if err != nil || second.More || len(second.Candidates) != 2 || second.Candidates[0].DocumentId != 201 ||
		second.Candidates[1].DocumentId != ^uint64(0) || second.Candidates[1].ReleaseId != ^uint64(0) ||
		second.Candidates[1].AggregateSeq != ^uint64(0) || string(second.Candidates[1].SortJsonUtf8) != `[1.2500e+2,"18446744073709551615",102]` {
		t.Fatalf("探测被吞/uint64或词法损坏: %v %+v", err, second)
	}
	first.Candidates[99].SortJsonUtf8[0] = 'x'
	if first.End[0] != '[' {
		t.Fatal("End与候选slice共享可变内存")
	}
}

func TestPITCandidatesRejectPartialAndBrokenBindingsIncludingProbe(t *testing.T) {
	cases := map[string]func(map[string]any){
		"timeout":         func(v map[string]any) { v["timed_out"] = true },
		"missing timeout": func(v map[string]any) { delete(v, "timed_out") },
		"terminated":      func(v map[string]any) { v["terminated_early"] = true },
		"missing PIT":     func(v map[string]any) { delete(v, "pit_id") },
		"missing shards":  func(v map[string]any) { delete(v, "_shards") },
		"failed shards":   func(v map[string]any) { v["_shards"].(map[string]any)["failed"] = 1 },
		"partial shards":  func(v map[string]any) { v["_shards"].(map[string]any)["successful"] = 1 },
		"missing hits":    func(v map[string]any) { delete(v, "hits") },
		"null hits":       func(v map[string]any) { v["hits"] = map[string]any{"hits": nil} },
		"probe binding":   func(v map[string]any) { v["hits"].(map[string]any)["hits"].([]any)[1].(map[string]any)["_id"] = "3" },
		"different physical": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"].([]any)[1].(map[string]any)["_index"] = "owned-g2"
		},
		"zero sequence": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"].([]any)[0].(map[string]any)["_source"].(map[string]any)["sequence"] = "0"
		},
		"noncanonical release": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"].([]any)[0].(map[string]any)["_source"].(map[string]any)["release_id"] = "01"
		},
		"sort doc mismatch": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"].([]any)[0].(map[string]any)["sort"] = json.RawMessage(`[1,"9",0]`)
		},
		"sort shard overflow": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"].([]any)[0].(map[string]any)["sort"] = json.RawMessage(`[1,"1",18446744073709551616]`)
		},
		"duplicate doc": func(v map[string]any) { v["hits"].(map[string]any)["hits"] = []any{pitHit("1", 0), pitHit("1", 0)} },
		"too many hits": func(v map[string]any) {
			v["hits"].(map[string]any)["hits"] = []any{pitHit("1", 0), pitHit("2", 1), pitHit("3", 2)}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				response := pitResponse([]any{pitHit("1", 0), pitHit("2", 1)})
				mutate(response)
				_ = json.NewEncoder(w).Encode(response)
			})
			batch, err := client.Candidates(context.Background(), "initial", "词", nil, 1, time.Second)
			if err == nil || len(batch.Candidates) != 0 || batch.More || len(batch.End) != 0 {
				t.Fatal("不完整响应被当成半批成功")
			}
			if name != "missing PIT" && batch.PIT != "latest" {
				t.Fatal("已收到新句柄却丢失关闭线索")
			}
		})
	}
}

func TestPITOpenCloseAndFiniteRemainingBudget(t *testing.T) {
	calls := 0
	client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.Method + " " + r.URL.Path {
		case "POST /owned-g1/_pit":
			if r.URL.Query().Get("keep_alive") != "1000ms" || r.URL.Query().Get("allow_partial_search_results") != "false" {
				t.Error("Open TTL/partial错误")
			}
			_, _ = w.Write([]byte(`{"id":"initial","_shards":{"total":2,"successful":2,"skipped":0,"failed":0}}`))
		case "DELETE /_pit":
			_, _ = w.Write([]byte(`{"succeeded":true,"num_freed":2}`))
		default:
			t.Error("意外请求")
		}
	})
	pit, err := client.OpenPIT(context.Background(), "owned-g1", time.Second+999*time.Microsecond)
	if err != nil || pit != "initial" {
		t.Fatal(err)
	}
	if err = client.ClosePIT(context.Background(), pit); err != nil {
		t.Fatal(err)
	}
	if _, err = client.OpenPIT(context.Background(), "owned-g1", time.Microsecond); err == nil {
		t.Fatal("不足1ms仍分配PIT")
	}
	if _, err = client.Candidates(context.Background(), pit, "词", nil, 100, 0); err == nil {
		t.Fatal("过期仍查询")
	}
	if _, err = client.OpenPIT(context.Background(), "bad/name", time.Second); err == nil {
		t.Fatal("非法索引")
	}
	if calls != 2 {
		t.Fatal("无效参数仍发起HTTP")
	}
}

func TestPITResponse24MiBBoundAndEmptyEndCopy(t *testing.T) {
	base, _ := json.Marshal(pitResponse([]any{}))
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(base)
				_, _ = w.Write([]byte(strings.Repeat(" ", (24<<20)-len(base)+extra)))
			})
			after := json.RawMessage(`[1,"1",0]`)
			batch, err := client.Candidates(context.Background(), "initial", "词", after, 100, 5*time.Second)
			if extra == 1 {
				if err == nil {
					t.Fatal("超过24MiB仍接受")
				}
				return
			}
			if err != nil || batch.More || len(batch.Candidates) != 0 || string(batch.End) != string(after) {
				t.Fatal("空批位置错误", err)
			}
			after[0] = 'x'
			if batch.End[0] != '[' {
				t.Fatal("空批End引用输入")
			}
		})
	}
}

func TestPITUnknownCloseAndOpenPartialAreNotSuccessful(t *testing.T) {
	for _, body := range []string{`{}`, `{"succeeded":false,"num_freed":1}`, `{"succeeded":true}`, `{"succeeded":true,"num_freed":-1}`} {
		client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		if err := client.ClosePIT(context.Background(), "initial"); err == nil {
			t.Fatal("未知关闭被确认", body)
		}
	}
	for _, body := range []string{`{"id":"pit"}`, `{"id":"pit","_shards":{"total":2,"successful":1,"skipped":0,"failed":1}}`} {
		client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		if _, err := client.OpenPIT(context.Background(), "owned-g1", time.Second); err == nil {
			t.Fatal("partial open被接受")
		}
	}
}

func TestPITShardSummaryAllowsOmittedSkippedButRequiresCompleteness(t *testing.T) {
	// Open/refresh不一定输出skipped；total==successful且failed==0才是完整性依据。
	for _, omitted := range []string{"skipped", "total", "successful", "failed"} {
		t.Run(omitted, func(t *testing.T) {
			client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				response := pitResponse([]any{})
				response["id"] = "initial"
				delete(response["_shards"].(map[string]any), omitted)
				_ = json.NewEncoder(w).Encode(response)
			})
			pit, err := client.OpenPIT(context.Background(), "owned-g1", time.Second)
			if omitted == "skipped" {
				if err != nil || pit != "initial" {
					t.Fatal("缺可选skipped被误判", err)
				}
			} else if err == nil {
				t.Fatal("缺完整性必需字段仍成功")
			}
		})
	}
}

func TestPITCancelledAndInvalidRequestsNeverReturnSuccessfulBatch(t *testing.T) {
	client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("取消/非法参数仍调用HTTP") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Candidates(ctx, "initial", "词", nil, 100, time.Second); err == nil {
		t.Fatal("取消被忽略")
	}
	for _, size := range []int{0, 101} {
		if _, err := client.Candidates(context.Background(), "initial", "词", nil, size, time.Second); err == nil {
			t.Fatal("非法size")
		}
	}
	if _, err := client.Candidates(context.Background(), "initial", "词", json.RawMessage(`null`), 100, time.Second); err == nil {
		t.Fatal("非法after")
	}
	if _, err := client.Candidates(context.Background(), "", "词", nil, 100, time.Second); err == nil {
		t.Fatal("空PIT")
	}
}
