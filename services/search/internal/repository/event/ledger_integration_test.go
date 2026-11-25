package event

import (
	"bytes"
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func actualStore(t *testing.T) *platform.Store {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	return store
}

func fixtureEvent(t *testing.T) domain.Event {
	t.Helper()
	// Windows时钟可能连续两次返回相同UnixNano；同批两条不同原事件不能碰成同一个ID。
	// 使用独立随机夹具ID，生产Receive的同号异摘要拒绝规则保持不变。
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		t.Fatal("ACTUAL_EVENT_FIXTURE_ENTROPY_UNAVAILABLE")
	}
	id := fmt.Sprintf("search-ledger-%x", randomID[:])
	payload, err := json.Marshal(map[string]any{"event_id": id, "event_type": "DOCUMENT_PUBLISHED", "schema_version": "1",
		"document_id": "18446744073709551615", "aggregate_seq": "18446744073709551615",
		"release_id": "18446744073709551615", "publication_epoch": "2", "occurred_at": "2026-10-07T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]string{"event_id": id, "event_type": "DOCUMENT_PUBLISHED", "schema_version": "1",
		"aggregate_kind": "DOCUMENT", "aggregate_id": "18446744073709551615", "aggregate_seq": "18446744073709551615",
		"payload": string(payload), "created_at": "2026-10-07 03:04:05"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := domain.RestoreEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestActualOriginConflictRollbackAndIndependentPhysicalState(t *testing.T) {
	store := actualStore(t)
	ctx := context.Background()
	event := fixtureEvent(t)
	group := "ledger-test"
	physical := "ledger-g1-" + event.ID
	second := "ledger-g2-" + event.ID
	// 清理只限本测试自建的无产品业务引用行，先子后父；不修改正式事件或真实checkpoint。
	t.Cleanup(func() {
		store.DB.ExecContext(ctx, "DELETE FROM event_origin WHERE consumer_name=? AND event_id=?", group, event.ID)
		store.DB.ExecContext(ctx, "DELETE FROM consumed_event WHERE consumer_name=? AND event_id=?", group, event.ID)
		store.DB.ExecContext(ctx, "DELETE FROM index_application WHERE physical_index IN (?,?)", physical, second)
	})
	position := Position{Topic: "ledger-test-topic", Partition: 1, Offset: 7}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Receive(ctx, tx, group, event, position); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed := event
	changed.Original = bytes.Replace(event.Original, []byte("03:04:05"), []byte("03:04:06"), -1)
	changed.RawDigest = sha256.Sum256(changed.Original)
	changed.OccurredAt = event.OccurredAt.Add(time.Second)
	if _, err = Receive(ctx, tx, group, changed, position); err == nil {
		tx.Rollback()
		t.Fatal("ORIGINAL_CONTENT_CONFLICT_ACCEPTED")
	}
	tx.Rollback()
	var retained []byte
	if err = store.DB.QueryRowContext(ctx, "SELECT original_row FROM event_origin WHERE consumer_name=? AND event_id=?", group, event.ID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if string(retained) != string(event.Original) {
		t.Fatal("ORIGINAL_EVENT_OVERWRITTEN")
	}
	state := index.State{Sequence: math.MaxUint64, Release: math.MaxUint64, Visible: true, Policy: "PUBLIC", Digest: [32]byte{9}}
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = LockApplication(ctx, tx, physical, event.Document); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = SaveApplication(ctx, tx, physical, event.Document, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = Complete(ctx, tx, group, event.ID, physical, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	// ES成功前的SQL事务如果回滚，原事件仍未成功且没有“已应用”水位。
	tx.Rollback()
	var count int
	if err = store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM index_application WHERE physical_index=?", physical).Scan(&count); err != nil || count != 0 {
		t.Fatal("ROLLED_BACK_APPLICATION_VISIBLE")
	}
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = LockApplication(ctx, tx, physical, event.Document); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = SaveApplication(ctx, tx, physical, event.Document, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = Complete(ctx, tx, group, event.ID, physical, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// 成功行缺真实权威摘要是未知持久事实，不能只凭SUCCEEDED跳过原消息。
	if _, err = tx.ExecContext(ctx, "UPDATE event_origin SET authority_digest=? WHERE consumer_name=? AND event_id=?", make([]byte, 32), group, event.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = Receive(ctx, tx, group, event, position); err == nil {
		tx.Rollback()
		t.Fatal("UNBOUND_SUCCESS_RECEIPT_ACCEPTED")
	}
	tx.Rollback()
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	position.Offset = 9
	receipt, err := Receive(ctx, tx, group, event, position)
	if err != nil || receipt.State != "SUCCEEDED" || receipt.Applied.Sequence != math.MaxUint64 || receipt.Applied.Digest != state.Digest {
		t.Fatal("FULL_UINT64_SUCCESS_NOT_REPLAYABLE")
	}
	newState, err := LockApplication(ctx, tx, second, event.Document)
	if err != nil || newState.Digest != ([32]byte{}) {
		t.Fatal("PHYSICAL_GENERATIONS_OVERLAPPED")
	}
	oldState, err := LockApplication(ctx, tx, physical, event.Document)
	if err != nil || oldState.Sequence != math.MaxUint64 || oldState.Release != math.MaxUint64 || oldState.Digest != state.Digest {
		t.Fatal("ORIGINAL_PHYSICAL_APPLICATION_CHANGED")
	}
}

func TestActualCheckpointRequiresEveryRecordRowSuccessAndNoOffsetGap(t *testing.T) {
	store := actualStore(t)
	ctx := context.Background()
	event := fixtureEvent(t)
	group := "checkpoint-test"
	topic := event.ID
	physical := "checkpoint-" + event.ID
	position := Position{Topic: topic, Partition: 2, Offset: 0}
	t.Cleanup(func() {
		store.DB.ExecContext(ctx, "DELETE FROM index_application WHERE physical_index=?", physical)
		store.DB.ExecContext(ctx, "DELETE FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=?", group, topic)
		store.DB.ExecContext(ctx, "DELETE FROM event_origin WHERE consumer_name=? AND event_id=?", group, event.ID)
		store.DB.ExecContext(ctx, "DELETE FROM consumed_event WHERE consumer_name=? AND event_id=?", group, event.ID)
	})
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Receive(ctx, tx, group, event, position); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{event}); err == nil {
		tx.Rollback()
		t.Fatal("UNFINISHED_ROW_COMMITTED_OFFSET")
	}
	tx.Rollback()
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Receive(ctx, tx, group, event, position); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	state := index.State{Sequence: math.MaxUint64, Digest: [32]byte{4}, Policy: "PUBLIC"}
	if _, err = LockApplication(ctx, tx, physical, event.Document); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = SaveApplication(ctx, tx, physical, event.Document, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = Complete(ctx, tx, group, event.ID, physical, state); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{event}); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// SQL成功、Kafka未提交时原offset可再读，但不能越过未完成的offset1去提交offset2。
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{event}); err != nil {
		t.Fatal(err)
	}
	position.Offset = 2
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{event}); err == nil {
		t.Fatal("MISSING_OFFSET_WAS_SKIPPED")
	}
}
