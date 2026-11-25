package event

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func canalRecord(t *testing.T, document, sequence, release string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"event_id": "release-create:fixture", "event_type": "DOCUMENT_PUBLISHED", "schema_version": "1",
		"document_id": document, "aggregate_seq": sequence, "release_id": release,
		"publication_epoch": "2", "occurred_at": "2026-10-07T03:04:05.123Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"id": 7, "database": "document_db", "table": "outbox", "pkNames": []string{"event_id"},
		"isDdl": false, "type": "INSERT", "es": 1791335045123, "ts": 1791335045133, "sql": "",
		"sqlType": map[string]int{"aggregate_id": -5}, "mysqlType": map[string]string{"aggregate_id": "bigint unsigned"},
		"data": []map[string]string{{
			"event_id": "release-create:fixture", "aggregate_kind": "DOCUMENT", "aggregate_id": document,
			"aggregate_seq": sequence, "event_type": "DOCUMENT_PUBLISHED", "schema_version": "1",
			"payload": string(payload), "created_at": "2026-10-07 03:04:05.123000",
		}}, "old": nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCanalPreservesFullUnsignedIdentifiersAndBindsPayload(t *testing.T) {
	raw := canalRecord(t, "18446744073709551615", "18446744073709551615", "18446744073709551615")
	events, err := ParseCanal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Document != math.MaxUint64 || events[0].Sequence != math.MaxUint64 ||
		events[0].Release != math.MaxUint64 || events[0].PublicationEpoch != 2 || events[0].ID != "release-create:fixture" {
		t.Fatal("CANAL_EVENT_BINDING_CHANGED")
	}
	if events[0].RawDigest == (sha256.Sum256(nil)) || events[0].RawDigest == ([32]byte{}) {
		t.Fatal("ORIGINAL_EVENT_DIGEST_MISSING")
	}
	// 消息重批次、重投时间不属于业务事件原内容；摘要不能因此改变而误判原事件冲突。
	repeated := []byte(strings.Replace(string(raw), `"id":7`, `"id":8`, 1))
	again, err := ParseCanal(repeated)
	if err != nil || again[0].RawDigest != events[0].RawDigest {
		t.Fatal("REDELIVERY_DIGEST_CHANGED")
	}
}

func TestCanalAcceptsGTIDMetadataWithoutChangingOriginalEvent(t *testing.T) {
	// 固定Canal版本真实flatMessage携带gtid；复制批次元数据不属于业务原行，
	// 补齐它不能放开未知字段，也不能令同一原事件重投时改变摘要。
	raw := canalRecord(t, "17", "3", "29")
	original, err := ParseCanal(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{`""`, `"24bc7850-2c16-11e6-a073-0242ac110002:17"`} {
		withGTID := append([]byte(`{"gtid":`+metadata+`,`), raw[1:]...)
		events, err := ParseCanal(withGTID)
		if err != nil || len(events) != 1 || events[0].Document != 17 || events[0].Sequence != 3 ||
			events[0].RawDigest != original[0].RawDigest || string(events[0].Original) != string(original[0].Original) {
			t.Fatal("REAL_CANAL_GTID_METADATA_REJECTED_OR_CHANGED_ORIGIN")
		}
	}
	for _, prefix := range []string{`{"gtid":17,`, `{"future_private_field":"x",`} {
		if _, err := ParseCanal(append([]byte(prefix), raw[1:]...)); err == nil {
			t.Fatal("UNKNOWN_OR_MALFORMED_CANAL_METADATA_ACCEPTED")
		}
	}
}

func TestCanalRejectsUnboundMalformedAndUnknownEvents(t *testing.T) {
	valid := string(canalRecord(t, "17", "3", "29"))
	cases := map[string]string{
		"wrong-table":           strings.Replace(valid, `"table":"outbox"`, `"table":"document"`, 1),
		"ddl":                   strings.Replace(valid, `"isDdl":false`, `"isDdl":true`, 1),
		"update":                strings.Replace(valid, `"type":"INSERT"`, `"type":"UPDATE"`, 1),
		"unknown-schema":        strings.ReplaceAll(valid, `\"schema_version\":\"1\"`, `\"schema_version\":\"2\"`),
		"row-payload-mismatch":  strings.Replace(valid, `"aggregate_id":"17"`, `"aggregate_id":"18"`, 1),
		"unknown-event":         strings.ReplaceAll(valid, "DOCUMENT_PUBLISHED", "MAIN_PRIVATE_UPDATED"),
		"duplicate-key":         strings.Replace(valid, `"table":"outbox"`, `"table":"outbox","table":"document"`, 1),
		"extra-document":        valid + "{}",
		"missing-release-field": strings.Replace(valid, `\"release_id\":\"29\",`, "", 1),
		"missing-ddl-field":     strings.Replace(valid, `"isDdl":false,`, "", 1),
	}
	for _, number := range []string{"0", "01", "+1", "-1", "1e3", "18446744073709551616"} {
		cases["invalid-id-"+number] = string(canalRecord(t, number, "3", "29"))
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCanal([]byte(value)); err == nil {
				t.Fatal("INVALID_CANAL_RECORD_ACCEPTED")
			}
		})
	}
	if _, err := ParseCanal([]byte(strings.Repeat("x", 2<<20+1))); err == nil {
		t.Fatal("UNBOUNDED_MESSAGE_ACCEPTED")
	}
}

func TestHiddenEventRequiresNoReleaseAndKeepsOriginDigestDistinct(t *testing.T) {
	published := string(canalRecord(t, "17", "3", "29"))
	hidden := strings.ReplaceAll(published, "DOCUMENT_PUBLISHED", "DOCUMENT_UNPUBLISHED")
	hidden = strings.Replace(hidden, `\"release_id\":\"29\"`, `\"release_id\":null`, 1)
	events, err := ParseCanal([]byte(hidden))
	if err != nil || len(events) != 1 || events[0].Release != 0 {
		t.Fatal("HIDDEN_EVENT_BINDING_INVALID")
	}
	visible, err := ParseCanal([]byte(published))
	if err != nil || visible[0].RawDigest == events[0].RawDigest {
		t.Fatal("DISTINCT_ORIGIN_CONTENT_HAS_SAME_DIGEST")
	}
	wrong := strings.ReplaceAll(published, "DOCUMENT_PUBLISHED", "DOCUMENT_UNPUBLISHED")
	if _, err = ParseCanal([]byte(wrong)); err == nil {
		t.Fatal("HIDDEN_RELEASE_ACCEPTED")
	}
	missing := strings.Replace(hidden, `\"release_id\":null,`, "", 1)
	if _, err = ParseCanal([]byte(missing)); err == nil {
		t.Fatal("HIDDEN_REQUIRED_RELEASE_FIELD_MISSING")
	}
}
