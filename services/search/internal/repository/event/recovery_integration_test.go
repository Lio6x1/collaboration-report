package event

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestActualRecoveryRejectsReceiptWithoutImmutableOriginal(t *testing.T) {
	store := actualStore(t)
	ctx := context.Background()
	value := fixtureEvent(t)
	group := fmt.Sprintf("recover-%d", time.Now().UnixNano())
	position := Position{Topic: "recovery-test-topic", Partition: 0, Offset: 0}
	t.Cleanup(func() {
		store.DB.ExecContext(ctx, "DELETE FROM event_origin WHERE consumer_name=?", group)
		store.DB.ExecContext(ctx, "DELETE FROM consumed_event WHERE consumer_name=?", group)
	})
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := Receive(ctx, tx, group, value, position); err != nil {
		t.Fatal(err)
	}
	if tx.Commit() != nil {
		t.Fatal("ACTUAL_RECOVERY_FIXTURE_COMMIT_FAILED")
	}
	checking, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckRecovery(ctx, checking, group, position.Topic); err != nil {
		var topicBound, partitionBound, digestBound, binaryJSON, textJSON, received, indexNull, sequenceNull, authorityNull int
		if diagnostic := checking.QueryRowContext(ctx, `SELECT o.kafka_topic=?,o.kafka_partition<=2,
			o.raw_digest=UNHEX(SHA2(o.original_row,256)),JSON_VALID(o.original_row),
			JSON_VALID(CAST(o.original_row AS CHAR CHARACTER SET utf8mb4)),c.state='RECEIVED',
			o.applied_index IS NULL,o.applied_seq IS NULL,o.authority_digest IS NULL
			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
			WHERE c.consumer_name=? AND c.event_id=?`, position.Topic, group, value.ID).
			Scan(&topicBound, &partitionBound, &digestBound, &binaryJSON, &textJSON, &received, &indexNull, &sequenceNull, &authorityNull); diagnostic == nil {
			t.Logf("RECOVERY_PREDICATES topic=%d partition=%d digest=%d binaryJSON=%d textJSON=%d received=%d indexNull=%d sequenceNull=%d authorityNull=%d", topicBound, partitionBound, digestBound, binaryJSON, textJSON, received, indexNull, sequenceNull, authorityNull)
		}
		t.Fatal(err)
	}
	checking.Rollback()
	// 已有原回执缺少原行时不能跳过恢复审计并开始提交后面的Kafka位置。
	if _, err := store.DB.ExecContext(ctx, "DELETE FROM event_origin WHERE consumer_name=? AND event_id=?", group, value.ID); err != nil {
		t.Fatal("ACTUAL_ORIGIN_FAULT_UNAVAILABLE")
	}
	checking, err = store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer checking.Rollback()
	if err := CheckRecovery(ctx, checking, group, position.Topic); err == nil {
		t.Fatal("UNBOUND_RECOVERY_RECEIPT_ACCEPTED")
	}
}
