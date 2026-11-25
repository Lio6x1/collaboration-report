package indexing

import (
	manual "collaboration/search/internal/client/kafka"
	repository "collaboration/search/internal/repository/event"
	"context"
	"strconv"
	"testing"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

func TestActualSQLSuccessBeforeKafkaCommitReplaysAndSynchronouslyCommits(t *testing.T) {
	f := prepareActualCase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	configuration := *f.config.Search
	configuration.Group, configuration.Topic = f.group, "actual-"+f.group
	transport := &kafka.Transport{DialTimeout: 2 * time.Second}
	defer transport.CloseIdleConnections()
	admin := &kafka.Client{Addr: kafka.TCP(configuration.Brokers...), Transport: transport, Timeout: 5 * time.Second}
	created, err := admin.CreateTopics(ctx, &kafka.CreateTopicsRequest{Topics: []kafka.TopicConfig{{Topic: configuration.Topic, NumPartitions: 3, ReplicationFactor: 1}}})
	if err != nil || created.Errors[configuration.Topic] != nil {
		t.Fatal("ACTUAL_KAFKA_TEST_TOPIC_UNAVAILABLE")
	}
	// 只清理本测试刚创建的独立topic；生产原topic、数据卷和组位置完全保留。
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		deleted, err := admin.DeleteTopics(cleanup, &kafka.DeleteTopicsRequest{Topics: []string{configuration.Topic}})
		if err != nil || deleted.Errors[configuration.Topic] != nil {
			t.Error("ACTUAL_KAFKA_TEST_TOPIC_CLEANUP_FAILED")
		}
	})
	writer := &kafka.Writer{Addr: kafka.TCP(configuration.Brokers...), Topic: configuration.Topic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, WriteTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second}
	defer writer.Close()
	// 原Canal业务行不改写；复制到独立真实topic只为从offset0验证组提交边界。
	if writer.WriteMessages(ctx, kafka.Message{Key: []byte(strconv.FormatUint(f.event.Document, 10)), Value: f.record}) != nil {
		t.Fatal("ACTUAL_KAFKA_FIXTURE_WRITE_FAILED")
	}
	first, err := manual.New(&configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := first.Window(ctx); err != nil {
		t.Fatal(err)
	}
	message, err := first.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if message.Offset != 0 {
		t.Fatalf("ACTUAL_KAFKA_ORIGINAL_NOT_FETCHED partition=%d offset=%d bytes=%d", message.Partition, message.Offset, len(message.Value))
	}
	position := repository.Position{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}
	// 一个未完成记录存在时，既不能Fetch后面的记录，也不能提交调用方改过的位置或内容。
	if _, err := first.Fetch(ctx); err == nil {
		t.Fatal("PENDING_KAFKA_RECORD_SKIPPED")
	}
	changed := message
	changed.Offset++
	if err := first.Commit(ctx, changed); err == nil {
		t.Fatal("UNFETCHED_HIGHER_KAFKA_OFFSET_COMMITTED")
	}
	changed = message
	changed.Value = append(append([]byte(nil), message.Value...), byte(' '))
	if err := first.Commit(ctx, changed); err == nil {
		t.Fatal("CHANGED_ORIGINAL_KAFKA_RECORD_COMMITTED")
	}
	processor := New(f.store.DB, f.source, f.sink, f.group)
	if err := processor.Record(ctx, f.physical, position, message.Value); err != nil {
		t.Fatal(err)
	}
	window, err := first.Window(ctx)
	if err != nil || window[message.Partition].Committed != -1 {
		t.Fatal("KAFKA_AUTO_COMMITTED_BEFORE_EXPLICIT_COMMIT")
	}
	var next, applicationRevision uint64
	if f.store.DB.QueryRowContext(ctx, "SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?", f.group, position.Topic, position.Partition).Scan(&next) != nil || next != 1 {
		t.Fatal("SQL_SUCCESS_NOT_PERSISTED_BEFORE_KAFKA")
	}
	if f.store.DB.QueryRowContext(ctx, "SELECT revision FROM index_application WHERE physical_index=? AND document_id=?", f.physical, f.event.Document).Scan(&applicationRevision) != nil {
		t.Fatal("ACTUAL_SQL_APPLICATION_UNAVAILABLE")
	}
	// SQL已成功但Kafka尚未提交：关闭消费者，再由同一个组真实重读原消息。
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := manual.New(&configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	replayed, err := second.Fetch(ctx)
	if err != nil || replayed.Partition != message.Partition || replayed.Offset != message.Offset {
		t.Fatal("ORIGINAL_UNCOMMITTED_KAFKA_RECORD_NOT_REPLAYED")
	}
	if err := processor.Record(ctx, f.physical, position, replayed.Value); err != nil {
		t.Fatal(err)
	}
	var recoveredRevision uint64
	if f.store.DB.QueryRowContext(ctx, "SELECT revision FROM index_application WHERE physical_index=? AND document_id=?", f.physical, f.event.Document).Scan(&recoveredRevision) != nil || recoveredRevision != applicationRevision {
		t.Fatal("REPLAY_MUTATED_COMMITTED_APPLICATION")
	}
	if err := second.Commit(ctx, replayed); err != nil {
		t.Fatal(err)
	}
	window, err = second.Window(ctx)
	if err != nil || window[message.Partition].Committed != 1 {
		t.Fatal("KAFKA_SYNCHRONOUS_COMMIT_NOT_CONFIRMED")
	}
}
