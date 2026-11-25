package elastic

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestSwitchAliasAcknowledgementAndRecovery(t *testing.T) {
	// 同一组协议场景覆盖精确动作、部分失败、最终绑定漂移和回执丢失；
	// 网络未知后的TARGET只能成为原job恢复证据，不能伪装本次请求成功。
	for _, tc := range []struct {
		name, reply, final string
		lost, success      bool
	}{
		{"success", `{"acknowledged":true,"errors":false}`, "new-index", false, true},
		{"complete-actions", `{"acknowledged":true,"errors":false,"action_results":[{"action":{"type":"remove","indices":["old-index"],"aliases":["reports"]},"status":200},{"action":{"type":"add","indices":["new-index"],"aliases":["reports"]},"status":200}]}`, "new-index", false, true},
		{"action-failure", `{"acknowledged":true,"errors":false,"action_results":[{"action":{"type":"remove","indices":["old-index"],"aliases":["reports"]},"status":404,"error":{}},{"action":{"type":"add","indices":["new-index"],"aliases":["reports"]},"status":200}]}`, "new-index", false, false},
		{"unacknowledged", `{"acknowledged":false}`, "new-index", false, false},
		{"wrong-final-target", `{"acknowledged":true}`, "foreign-index", false, false},
		{"lost-response", "", "new-index", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, posts := "old-index", 0
			client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/_alias/reports":
					_ = json.NewEncoder(w).Encode(map[string]any{actual: map[string]any{"aliases": map[string]any{"reports": map[string]any{"is_write_index": true}}}})
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/_mapping"):
					physical := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/_mapping")
					_ = json.NewEncoder(w).Encode(map[string]any{physical: map[string]any{"mappings": map[string]any{
						"dynamic": "strict", "_meta": map[string]any{"application": "collaboration-report", "owner": "owner", "schema": schemaVersion}, "properties": properties(),
					}}})
				case r.Method == "POST" && r.URL.Path == "/_aliases":
					posts++
					var body any
					if json.NewDecoder(r.Body).Decode(&body) != nil {
						t.Error("请求不是JSON")
					}
					var expected any
					_ = json.Unmarshal([]byte(`{"actions":[{"remove":{"index":"old-index","alias":"reports","must_exist":true}},{"add":{"index":"new-index","alias":"reports","is_write_index":true}}]}`), &expected)
					if !reflect.DeepEqual(body, expected) || r.URL.RawQuery != "master_timeout=1s&timeout=1s" {
						t.Error("精确动作或期限改变")
					}
					actual = tc.final
					if tc.lost {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					_, _ = w.Write([]byte(tc.reply))
				default:
					t.Errorf("意外请求 %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			})
			client.owner = "owner"
			err := client.SwitchAlias(context.Background(), "reports", "old-index", "new-index")
			if (err == nil) != tc.success || posts != 1 {
				t.Fatalf("success=%v posts=%d err=%v", tc.success, posts, err)
			}
			if tc.lost {
				state, err := client.InspectAliasSwitch(context.Background(), "reports", "old-index", "new-index")
				if err != nil || state != AliasAtTarget {
					t.Fatalf("不能恢复原目标: %s %v", state, err)
				}
				if err := client.SwitchAlias(context.Background(), "reports", "old-index", "new-index"); err == nil || posts != 1 {
					t.Fatal("恢复不得再次修改alias")
				}
			}
		})
	}
}

func TestSwitchAliasRejectsForeignOwnerBeforeMutation(t *testing.T) {
	client := pitTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("外来owner不得触发修改")
			w.WriteHeader(500)
			return
		}
		if r.URL.Path == "/_alias/reports" {
			_, _ = w.Write([]byte(`{"old-index":{"aliases":{"reports":{"is_write_index":true}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"old-index":{"mappings":{"dynamic":"strict","_meta":{"application":"collaboration-report","owner":"foreign","schema":"publication-index-v1-uint64"}}}}`))
	})
	client.owner = "owner"
	if err := client.SwitchAlias(context.Background(), "reports", "old-index", "new-index"); err == nil {
		t.Fatal("外来owner被接受")
	}
}
