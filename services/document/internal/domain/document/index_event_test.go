package document

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIndexEventSequenceNeverWraps(t *testing.T) {
	// 可见性只推进索引水位；最大值必须拒绝，不能回绕到旧发布事件之前。
	for _, test := range []struct {
		current, want uint64
	}{
		{0, 1},
		{9007199254740993, 9007199254740994},
		{math.MaxUint64 - 1, math.MaxUint64},
	} {
		got, err := NextIndexEventSequence(test.current)
		if err != nil || got != test.want {
			t.Fatalf("sequence %d: got %d, %v; want %d", test.current, got, err, test.want)
		}
	}
	got, err := NextIndexEventSequence(math.MaxUint64)
	assertIndexEventFailure(t, err, StateConflict, "EVENT_SEQUENCE_EXHAUSTED")
	if got != 0 {
		t.Fatalf("exhaustion returned usable sequence %d", got)
	}
}

func TestVisibilityIndexEventIDBindsOriginalOperationAndDocument(t *testing.T) {
	// KB一次原操作对应多个文档；稳定派生保证重试复用原事件，最大输入也不能超出outbox主键。
	operation := strings.Repeat("A", 96)
	first, err := VisibilityIndexEventID(operation, math.MaxUint64)
	if err != nil || len(first) > 96 || !ValidOperationID(first) {
		t.Fatalf("maximum operation not safely derived: %q, %v", first, err)
	}
	if !strings.HasPrefix(first, "visibility_index_") {
		t.Fatalf("event type namespace missing: %q", first)
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(first, "visibility_index_"))
	if err != nil || len(digest) != 32 {
		t.Fatalf("event ID must retain complete SHA256: %q", first)
	}
	repeated, err := VisibilityIndexEventID(operation, math.MaxUint64)
	if err != nil || repeated != first {
		t.Fatalf("original operation changed ID: %q, %v", repeated, err)
	}
	otherDocument, err := VisibilityIndexEventID(operation, math.MaxUint64-1)
	if err != nil || otherDocument == first {
		t.Fatalf("different document shares event: %q, %v", otherDocument, err)
	}
	otherOperation, err := VisibilityIndexEventID(strings.Repeat("B", 96), math.MaxUint64)
	if err != nil || otherOperation == first {
		t.Fatalf("different original operation shares event: %q, %v", otherOperation, err)
	}
}

func TestVisibilityIndexEventIDRejectsInvalidOriginalBinding(t *testing.T) {
	for _, operation := range []string{"", "short", strings.Repeat("A", 97), "visibility operation", "visibility:operation", "可见范围operation0001", "_visibility-operation"} {
		_, err := VisibilityIndexEventID(operation, 1)
		assertIndexEventFailure(t, err, InvalidRequest, "INDEX_EVENT_BINDING_INVALID")
	}
	_, err := VisibilityIndexEventID("visibility-operation", 0)
	assertIndexEventFailure(t, err, InvalidRequest, "INDEX_EVENT_BINDING_INVALID")
}

func TestEncodeIndexEventPreservesEightKeyPublicReferenceContract(t *testing.T) {
	// 全uint64及带本地时区时间进入消息后仍是规范字符串；不能夹带策略、正文或准入身份。
	event := IndexEvent{
		ID: "visibility-operation", Type: "DOCUMENT_VISIBILITY_CHANGED",
		Document: math.MaxUint64, Sequence: math.MaxUint64, Release: math.MaxUint64,
		PublicationEpoch: math.MaxUint64,
		OccurredAt:       time.Date(2026, 10, 7, 12, 34, 56, 123456789, time.FixedZone("CST", 8*60*60)),
	}
	raw, err := EncodeIndexEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	wantKeys := []string{"aggregate_seq", "document_id", "event_id", "event_type", "occurred_at", "publication_epoch", "release_id", "schema_version"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("unexpected bus fields: %v", keys)
	}
	want := map[string]string{
		"event_id": "visibility-operation", "event_type": "DOCUMENT_VISIBILITY_CHANGED", "schema_version": "1",
		"document_id": "18446744073709551615", "aggregate_seq": "18446744073709551615",
		"release_id": "18446744073709551615", "publication_epoch": "18446744073709551615",
		"occurred_at": "2026-10-07T04:34:56.123456789Z",
	}
	for key, expected := range want {
		var value string
		if json.Unmarshal(fields[key], &value) != nil || value != expected {
			t.Fatalf("%s lost canonical reference: %s; want %q", key, fields[key], expected)
		}
	}
}

func TestEncodeIndexEventSeparatesReleaseFromVisibilityChange(t *testing.T) {
	// 未发布也要有可见性事件，但不能借此创建正式指针或推进发布代次。
	for _, test := range []struct {
		kind    string
		release uint64
		want    string
	}{
		{"DOCUMENT_PUBLISHED", 7, `"7"`},
		{"DOCUMENT_UNPUBLISHED", 0, "null"},
		{"DOCUMENT_VISIBILITY_CHANGED", 0, "null"},
		{"DOCUMENT_VISIBILITY_CHANGED", 7, `"7"`},
	} {
		t.Run(test.kind+test.want, func(t *testing.T) {
			event := validIndexEventFixture()
			event.Type, event.Release = test.kind, test.release
			raw, err := EncodeIndexEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["release_id"]) != test.want || string(fields["publication_epoch"]) != `"3"` {
				t.Fatalf("release or publication epoch changed: %s", raw)
			}
		})
	}
}

func TestEncodeIndexEventRejectsInvalidFacts(t *testing.T) {
	// 消息不能接受未知类型、零水位或与发布类型不匹配的指针，避免毒消息提交后阻塞消费。
	for name, change := range map[string]func(*IndexEvent){
		"unknown-type":            func(e *IndexEvent) { e.Type = "MAIN_PRIVATE_UPDATED" },
		"published-no-release":    func(e *IndexEvent) { e.Type, e.Release = "DOCUMENT_PUBLISHED", 0 },
		"unpublished-release":     func(e *IndexEvent) { e.Type, e.Release = "DOCUMENT_UNPUBLISHED", 7 },
		"zero-document":           func(e *IndexEvent) { e.Document = 0 },
		"zero-sequence":           func(e *IndexEvent) { e.Sequence = 0 },
		"zero-publication":        func(e *IndexEvent) { e.PublicationEpoch = 0 },
		"zero-time":               func(e *IndexEvent) { e.OccurredAt = time.Time{} },
		"unrepresentable-time":    func(e *IndexEvent) { e.OccurredAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) },
		"empty-id":                func(e *IndexEvent) { e.ID = "" },
		"short-id":                func(e *IndexEvent) { e.ID = "short" },
		"long-id":                 func(e *IndexEvent) { e.ID = strings.Repeat("A", 97) },
		"unexpected-id-character": func(e *IndexEvent) { e.ID = "visibility:operation" },
	} {
		t.Run(name, func(t *testing.T) {
			event := validIndexEventFixture()
			change(&event)
			raw, err := EncodeIndexEvent(event)
			assertIndexEventFailure(t, err, InvalidRequest, "INDEX_EVENT_INVALID")
			if len(raw) != 0 {
				t.Fatalf("invalid event returned usable payload: %s", raw)
			}
		})
	}
}

func validIndexEventFixture() IndexEvent {
	return IndexEvent{ID: "visibility-operation", Type: "DOCUMENT_VISIBILITY_CHANGED", Document: 1, Sequence: 2,
		PublicationEpoch: 3, OccurredAt: time.Date(2026, 10, 7, 4, 34, 56, 0, time.UTC)}
}

func assertIndexEventFailure(t *testing.T, err error, kind FailureKind, code string) {
	t.Helper()
	var violation RuleViolation
	if !errors.As(err, &violation) || violation.Kind != kind || violation.Code != code {
		t.Fatalf("expected %v/%s, got %v", kind, code, err)
	}
}
