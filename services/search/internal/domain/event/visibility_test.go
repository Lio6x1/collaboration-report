package event

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCanalVisibilityEventAcceptsCurrentReleaseOrUnpublished(t *testing.T) {
	// 可见性不等于再次发布：未发布允许null，当前正式指针允许完整uint64；发布代次保持原事实。
	for _, test := range []struct {
		name    string
		release any
		want    uint64
	}{
		{"unpublished", nil, 0},
		{"current-release", "18446744073709551615", math.MaxUint64},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := visibilityCanalRecord(t, test.release)
			events, err := ParseCanal(raw)
			if err != nil || len(events) != 1 {
				t.Fatalf("visibility event rejected: %v", err)
			}
			got := events[0]
			if got.Type != "DOCUMENT_VISIBILITY_CHANGED" || got.Document != math.MaxUint64 ||
				got.Sequence != math.MaxUint64 || got.PublicationEpoch != math.MaxUint64 || got.Release != test.want {
				t.Fatalf("visibility facts lost precision or changed: %+v", got)
			}
			// 私有记账恢复必须沿同一严格解析规则，不能把新事件当未知后跳过。
			restored, err := RestoreEvent(got.Original)
			if err != nil || restored.Type != got.Type || restored.Release != got.Release ||
				restored.RawDigest != got.RawDigest || string(restored.Original) != string(got.Original) {
				t.Fatalf("visibility original cannot be restored: %+v, %v", restored, err)
			}
			withGTID := append([]byte(`{"gtid":"24bc7850-2c16-11e6-a073-0242ac110002:17",`), raw[1:]...)
			repeated, err := ParseCanal(withGTID)
			if err != nil || len(repeated) != 1 || repeated[0].RawDigest != got.RawDigest {
				t.Fatalf("GTID redelivery changed visibility origin: %v", err)
			}
		})
	}
}

func TestCanalVisibilityEventRetainsStrictPublicReferenceBinding(t *testing.T) {
	// 扩展类型不能放宽八键、原行绑定和规范正数；拒绝毒消息时不能返回部分已成功事件。
	valid := visibilityCanalRecord(t, nil)
	cases := map[string][]byte{
		"unknown-type": mutateVisibilityRecord(t, valid, func(row map[string]string, body map[string]any) {
			row["event_type"], body["event_type"] = "MAIN_PRIVATE_UPDATED", "MAIN_PRIVATE_UPDATED"
		}),
		"unknown-schema": mutateVisibilityRecord(t, valid, func(row map[string]string, body map[string]any) {
			row["schema_version"], body["schema_version"] = "2", "2"
		}),
		"row-payload-type-mismatch": mutateVisibilityRecord(t, valid, func(_ map[string]string, body map[string]any) {
			body["event_type"] = "DOCUMENT_UNPUBLISHED"
		}),
		"row-payload-document-mismatch": mutateVisibilityRecord(t, valid, func(row map[string]string, _ map[string]any) {
			row["aggregate_id"] = "17"
		}),
		"missing-release": mutateVisibilityRecord(t, valid, func(_ map[string]string, body map[string]any) {
			delete(body, "release_id")
		}),
		"extra-policy": mutateVisibilityRecord(t, valid, func(_ map[string]string, body map[string]any) {
			body["visibility_policy"] = "PUBLIC"
		}),
		"duplicate-payload-key": []byte(strings.Replace(string(valid), `\"release_id\":null`, `\"release_id\":null,\"release_id\":null`, 1)),
		"duplicate-row-key":     []byte(strings.Replace(string(valid), `"schema_version":"1"`, `"schema_version":"1","schema_version":"1"`, 1)),
		"duplicate-GTID":        append([]byte(`{"gtid":"","gtid":"",`), valid[1:]...),
		"numeric-GTID":          append([]byte(`{"gtid":17,`), valid[1:]...),
	}
	for _, field := range []string{"release_id", "publication_epoch"} {
		for _, value := range []any{"0", "01", "+1", "-1", "1e3", "", "18446744073709551616", float64(1)} {
			key := field + "-" + string(mustVisibilityJSON(t, value))
			cases[key] = mutateVisibilityRecord(t, valid, func(_ map[string]string, body map[string]any) {
				body[field] = value
			})
		}
	}
	cases["null-publication-epoch"] = mutateVisibilityRecord(t, valid, func(_ map[string]string, body map[string]any) {
		body["publication_epoch"] = nil
	})
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if events, err := ParseCanal(raw); err == nil || len(events) != 0 {
				t.Fatalf("invalid visibility event accepted: %+v, %v", events, err)
			}
		})
	}
}

func TestCanalVisibilitySupportKeepsOriginalPublicationReleaseRules(t *testing.T) {
	// 新类型的可选指针不能让原发布缺指针或原撤下带指针通过。
	for name, test := range map[string]struct {
		kind    string
		release any
	}{
		"published-null":      {"DOCUMENT_PUBLISHED", nil},
		"unpublished-release": {"DOCUMENT_UNPUBLISHED", "29"},
	} {
		t.Run(name, func(t *testing.T) {
			raw := mutateVisibilityRecord(t, visibilityCanalRecord(t, test.release), func(row map[string]string, body map[string]any) {
				row["event_type"], body["event_type"] = test.kind, test.kind
			})
			if _, err := ParseCanal(raw); err == nil || err.Error() != "CANAL_RELEASE_BINDING_INVALID" {
				t.Fatalf("original publication release constraint changed: %v", err)
			}
		})
	}
}

func visibilityCanalRecord(t *testing.T, release any) []byte {
	t.Helper()
	raw := canalRecord(t, "18446744073709551615", "18446744073709551615", "29")
	return mutateVisibilityRecord(t, raw, func(row map[string]string, body map[string]any) {
		row["event_type"], body["event_type"] = "DOCUMENT_VISIBILITY_CHANGED", "DOCUMENT_VISIBILITY_CHANGED"
		body["release_id"], body["publication_epoch"] = release, "18446744073709551615"
	})
}

func mutateVisibilityRecord(t *testing.T, raw []byte, change func(map[string]string, map[string]any)) []byte {
	t.Helper()
	var envelope map[string]json.RawMessage
	var rows []map[string]string
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope["data"], &rows); err != nil || len(rows) != 1 {
		t.Fatalf("invalid fixture rows: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(rows[0]["payload"]), &body); err != nil {
		t.Fatal(err)
	}
	change(rows[0], body)
	rows[0]["payload"] = string(mustVisibilityJSON(t, body))
	envelope["data"] = mustVisibilityJSON(t, rows)
	return mustVisibilityJSON(t, envelope)
}

func mustVisibilityJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
