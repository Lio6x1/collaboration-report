package logsource

import (
	"collaboration/platform"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	broker "github.com/segmentio/kafka-go"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestActualReadOriginalUUIDRangeAndRejectRecreatedTopic(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	config, err := platform.LoadConfig(file)
	if err != nil || config.Search == nil {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	topic := fmt.Sprintf("actual-rebuild-read-%d", time.Now().UnixNano())
	transport := &broker.Transport{DialTimeout: 2 * time.Second}
	defer transport.CloseIdleConnections()
	admin := &broker.Client{Addr: broker.TCP(config.Search.Brokers...), Transport: transport, Timeout: 3 * time.Second}
	create := func() {
		t.Helper()
		result, err := admin.CreateTopics(ctx, &broker.CreateTopicsRequest{Topics: []broker.TopicConfig{{Topic: topic, NumPartitions: 3, ReplicationFactor: 1}}})
		if err != nil || result == nil || result.Errors[topic] != nil {
			t.Fatal("ACTUAL_READ_CREATE_FAILED")
		}
	}
	remove := func() {
		t.Helper()
		result, err := admin.DeleteTopics(ctx, &broker.DeleteTopicsRequest{Topics: []string{topic}})
		if err != nil || result == nil || result.Errors[topic] != nil {
			t.Fatal("ACTUAL_READ_DELETE_FAILED")
		}
	}
	create()
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		result, err := admin.DeleteTopics(cleanup, &broker.DeleteTopicsRequest{Topics: []string{topic}})
		if err != nil || result == nil || result.Errors[topic] != nil {
			t.Error("ACTUAL_READ_CLEANUP_FAILED")
		}
	}()
	client, err := New(config.Search.Brokers, topic)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	waitWindow := func(previous *Identity) Window {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for {
			value, err := client.Capture(ctx, nil)
			if err == nil && (previous == nil || value.Identity != *previous) {
				return value
			}
			if time.Now().After(deadline) || ctx.Err() != nil {
				t.Fatal("ACTUAL_READ_TOPIC_NOT_READY")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	original := waitWindow(nil)
	// 用同一真实主题覆盖五种broker批次；它们是传输夹具，不伪装成Canal事件。
	for index, codec := range []broker.Compression{0, broker.Gzip, broker.Snappy, broker.Lz4, broker.Zstd} {
		result, err := admin.Produce(ctx, &broker.ProduceRequest{Topic: topic, Partition: 0, RequiredAcks: broker.RequireAll, Compression: codec,
			Records: broker.NewRecordReader(broker.Record{Value: broker.NewBytes([]byte(fmt.Sprintf("record-%d-a", index)))}, broker.Record{Value: broker.NewBytes([]byte(fmt.Sprintf("record-%d-b", index)))})})
		if err != nil || result == nil || result.Error != nil || result.BaseOffset != int64(index*2) {
			t.Fatal("ACTUAL_READ_PRODUCE_FAILED")
		}
	}
	// H落在最后批次中间：只交付[1,9)，不能吞掉H处记录，也不能回到批次起点0。
	next := int64(1)
	for next < 9 {
		records, after, err := client.Read(ctx, original, 0, next, 9)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) == 0 || after <= next || after > 9 {
			t.Fatal("ACTUAL_READ_FRONTIER_INVALID")
		}
		for _, record := range records {
			if record.Offset != next || record.Topic != topic || record.Partition != 0 {
				t.Fatal("ACTUAL_READ_RECORD_UNBOUND")
			}
			suffix := "a"
			if next%2 == 1 {
				suffix = "b"
			}
			if string(record.Value) != fmt.Sprintf("record-%d-%s", next/2, suffix) {
				t.Fatal("ACTUAL_READ_VALUE_MISMATCH")
			}
			next++
		}
		if next != after {
			t.Fatal("ACTUAL_READ_CHECKPOINT_MISMATCH")
		}
	}
	// 即使next已前进，原L落出保留窗口也须拒绝原job，不能偷偷把L改为next。
	trim := kmsg.NewPtrDeleteRecordsRequest()
	trim.TimeoutMillis = 2000
	trim.Topics = []kmsg.DeleteRecordsRequestTopic{{Topic: topic, Partitions: []kmsg.DeleteRecordsRequestTopicPartition{{Partition: 0, Offset: 2}}}}
	trimmed, err := trim.RequestWith(ctx, client.api)
	if err != nil || trimmed == nil || len(trimmed.Topics) != 1 || len(trimmed.Topics[0].Partitions) != 1 || trimmed.Topics[0].Partitions[0].ErrorCode != 0 || trimmed.Topics[0].Partitions[0].LowWatermark < 2 {
		t.Fatal("ACTUAL_READ_TRIM_FAILED")
	}
	if _, _, err := client.Read(ctx, original, 0, 9, 10); err == nil || err.Error() != "SEARCH_LOG_WINDOW_LOST" {
		t.Fatal("ACTUAL_READ_ACCEPTED_LOST_ORIGINAL_WINDOW")
	}
	remove()
	// Kafka删除完成后同名重建，原prepare仍必须拒绝；不修改产品主题或普通组。
	create()
	_ = waitWindow(&original.Identity)
	if _, _, err := client.Read(ctx, original, 0, 1, 9); err == nil || err.Error() != "SEARCH_LOG_IDENTITY_CHANGED" {
		t.Fatal("ACTUAL_READ_ACCEPTED_RECREATED_TOPIC")
	}
}
