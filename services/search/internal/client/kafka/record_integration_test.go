package kafka

import (
	"bytes"
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	broker "github.com/segmentio/kafka-go"
)

func TestActualInvalidRecordBlocksFurtherFetchAndCommit(t *testing.T) {
	// 反例针对真实Kafka传输记录：坏记录之后还有正常字节，不能将它们抓取或提交。
	// 这些字节不是Canal业务夹具；本测试只证明客户端不会越过传输校验失败位置。
	for _, test := range []struct {
		name  string
		value []byte
	}{
		{name: "empty", value: []byte{}},
		{name: "oversize", value: bytes.Repeat([]byte{'x'}, domain.MaxRecordBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
			if file == "" {
				t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
			}
			config, err := platform.LoadConfig(file)
			if err != nil || config.Search == nil {
				t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
			}
			settings := *config.Search
			settings.Group = fmt.Sprintf("invalid-%d", time.Now().UnixNano())
			settings.Topic = "actual-" + settings.Group
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			transport := &broker.Transport{DialTimeout: 2 * time.Second}
			defer transport.CloseIdleConnections()
			admin := &broker.Client{Addr: broker.TCP(settings.Brokers...), Transport: transport, Timeout: 5 * time.Second}
			created, err := admin.CreateTopics(ctx, &broker.CreateTopicsRequest{Topics: []broker.TopicConfig{{
				Topic: settings.Topic, NumPartitions: 3, ReplicationFactor: 1,
				// 只放宽这个新主题的夹具上限；生产主题及broker配置完全不修改。
				ConfigEntries: []broker.ConfigEntry{{ConfigName: "max.message.bytes", ConfigValue: "4194304"}},
			}}})
			if err != nil || created == nil || created.Errors[settings.Topic] != nil {
				t.Fatal("ACTUAL_INVALID_RECORD_TOPIC_CREATE_FAILED")
			}
			// cleanup在所有测试消费者Close之后执行，只删除精确的新主题和新组。
			defer func() {
				cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
				defer done()
				deletedGroup, err := admin.DeleteGroups(cleanup, &broker.DeleteGroupsRequest{GroupIDs: []string{settings.Group}})
				if err != nil || deletedGroup == nil || (deletedGroup.Errors[settings.Group] != nil && !errors.Is(deletedGroup.Errors[settings.Group], broker.GroupIdNotFound)) {
					t.Error("ACTUAL_INVALID_RECORD_GROUP_CLEANUP_FAILED")
				}
				deletedTopic, err := admin.DeleteTopics(cleanup, &broker.DeleteTopicsRequest{Topics: []string{settings.Topic}})
				if err != nil || deletedTopic == nil || deletedTopic.Errors[settings.Topic] != nil {
					t.Error("ACTUAL_INVALID_RECORD_TOPIC_CLEANUP_FAILED")
				}
			}()
			produce := func(partition int, value []byte, offset int64) {
				t.Helper()
				write, done := context.WithTimeout(ctx, 5*time.Second)
				defer done()
				for {
					response, err := admin.Produce(write, &broker.ProduceRequest{Topic: settings.Topic, Partition: partition, RequiredAcks: broker.RequireAll,
						Records: broker.NewRecordReader(broker.Record{Time: time.Now(), Value: broker.NewBytes(value)})})
					if err == nil && response != nil && response.Error == nil && response.BaseOffset == offset {
						return
					}
					var protocolCode broker.Error
					errors.As(err, &protocolCode)
					if response != nil && response.Error != nil {
						errors.As(response.Error, &protocolCode)
					}
					// 新主题的leader可能尚未可见；只重试明确未写入的协议3/6。
					// 原5秒预算不增加，网络/提交未知绝不盲重发以避免重复夹具。
					if write.Err() != nil || (protocolCode != broker.UnknownTopicOrPartition && protocolCode != broker.NotLeaderForPartition) {
						t.Fatalf("ACTUAL_INVALID_RECORD_FIXTURE_WRITE_FAILED partition=%d bytes=%d transport_error=%t protocol_code=%d", partition, len(value), err != nil, int(protocolCode))
					}
					transport.CloseIdleConnections()
					select {
					case <-write.Done():
						t.Fatal("ACTUAL_INVALID_RECORD_FIXTURE_WRITE_EXPIRED")
					case <-time.After(100 * time.Millisecond):
					}
				}
			}
			produce(0, test.value, 0)
			first, err := New(&settings)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Close()
			if _, err := first.Fetch(ctx); err == nil || err.Error() != "SEARCH_KAFKA_RECORD_INVALID" {
				t.Fatal("ACTUAL_INVALID_ORIGINAL_NOT_REJECTED")
			}
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			second, err := New(&settings)
			if err != nil {
				t.Fatal(err)
			}
			defer second.Close()
			// 仅分区0有原坏记录；同一组的新实例必须重读它，不能因关闭就推进位置。
			if _, err := second.Fetch(ctx); err == nil || err.Error() != "SEARCH_KAFKA_RECORD_INVALID" {
				t.Fatal("ACTUAL_INVALID_ORIGINAL_NOT_REPLAYED")
			}
			produce(0, []byte("later-same-partition"), 1)
			produce(1, []byte("later-other-partition"), 0)
			for attempt := 0; attempt < 2; attempt++ {
				poll, done := context.WithTimeout(ctx, 3*time.Second)
				message, err := second.Fetch(poll)
				done()
				if err == nil || err.Error() != "SEARCH_KAFKA_RECORD_INVALID" {
					t.Errorf("INVALID_RECORD_WAS_SKIPPED attempt=%d fetched=%t partition=%d offset=%d", attempt, err == nil, message.Partition, message.Offset)
				}
				if err := second.Commit(ctx, message); err == nil || err.Error() != "SEARCH_KAFKA_COMMIT_UNBOUND" {
					t.Error("LATER_RECORD_COMMITTED_AFTER_INVALID_ORIGINAL")
				}
			}
			// 显式伪造原坏位置与另分区合法位置也不能成为提交通道。
			for _, message := range []domain.Record{
				{Position: domain.Position{Topic: settings.Topic, Partition: 0, Offset: 0}, Value: test.value},
				{Position: domain.Position{Topic: settings.Topic, Partition: 1, Offset: 0}, Value: []byte("later-other-partition")},
			} {
				if err := second.Commit(ctx, message); err == nil || err.Error() != "SEARCH_KAFKA_COMMIT_UNBOUND" {
					t.Error("INVALID_OR_UNFETCHED_RECORD_COMMITTED")
				}
			}
			window, err := second.Window(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for partition := 0; partition < 3; partition++ {
				if window[partition].Committed != -1 {
					t.Errorf("INVALID_RECORD_ADVANCED_GROUP_OFFSET partition=%d committed=%d", partition, window[partition].Committed)
				}
			}
			t.Logf("REAL_KAFKA_TRANSPORT_ONLY bytes=%d rejected_original=true restart_replayed=true all_partitions_uncommitted=%t", len(test.value), !t.Failed())
		})
	}
}
