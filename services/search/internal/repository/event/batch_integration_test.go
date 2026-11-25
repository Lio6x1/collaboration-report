package event

import (
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"encoding/json"
	"math"
	"testing"
)

func olderFixture(t *testing.T) domain.Event {
	t.Helper()
	value := fixtureEvent(t)
	var row map[string]string
	if json.Unmarshal(value.Original, &row) != nil {
		t.Fatal("FIXTURE_ORIGIN_INVALID")
	}
	var payload map[string]any
	if json.Unmarshal([]byte(row["payload"]), &payload) != nil {
		t.Fatal("FIXTURE_PAYLOAD_INVALID")
	}
	payload["aggregate_seq"], payload["release_id"], payload["publication_epoch"] = "18446744073709551614", "18446744073709551614", "1"
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	row["aggregate_seq"], row["payload"] = "18446744073709551614", string(body)
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	value, err = domain.RestoreEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestActualBatchCannotCommitFirstSuccessfulRowPastSecondPendingRow(t *testing.T) {
	store := actualStore(t)
	ctx := context.Background()
	first, second := olderFixture(t), fixtureEvent(t)
	group := "batch-checkpoint-test"
	topic := first.ID
	physical := "batch-" + first.ID
	position := Position{Topic: topic, Partition: 0, Offset: 0}
	t.Cleanup(func() {
		store.DB.ExecContext(ctx, "DELETE FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=?", group, topic)
		store.DB.ExecContext(ctx, "DELETE FROM index_application WHERE physical_index=?", physical)
		for _, value := range []domain.Event{first, second} {
			store.DB.ExecContext(ctx, "DELETE FROM event_origin WHERE consumer_name=? AND event_id=?", group, value.ID)
			store.DB.ExecContext(ctx, "DELETE FROM consumed_event WHERE consumer_name=? AND event_id=?", group, value.ID)
		}
	})
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, value := range []domain.Event{first, second} {
		if _, err = Receive(ctx, tx, group, value, position); err != nil {
			t.Fatal(err)
		}
	}
	state := index.State{Sequence: math.MaxUint64, Release: math.MaxUint64, Visible: true, Policy: "PUBLIC", Digest: [32]byte{7}}
	if _, err = LockApplication(ctx, tx, physical, first.Document); err != nil {
		t.Fatal(err)
	}
	if err = SaveApplication(ctx, tx, physical, first.Document, state); err != nil {
		t.Fatal(err)
	}
	if err = Complete(ctx, tx, group, first.ID, physical, state); err != nil {
		t.Fatal(err)
	}
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{first, second}); err == nil {
		t.Fatal("SECOND_PENDING_ROW_WAS_SKIPPED")
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=?", group, topic).Scan(&count); err != nil || count != 0 {
		t.Fatal("PARTIAL_BATCH_ADVANCED_CHECKPOINT")
	}
	if err = Complete(ctx, tx, group, second.ID, physical, state); err != nil {
		t.Fatal(err)
	}
	if err = AdvanceCheckpoint(ctx, tx, group, position, []domain.Event{first, second}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
