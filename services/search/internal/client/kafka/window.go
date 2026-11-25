package kafka

import (
	domain "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type PartitionWindow = domain.PartitionWindow

// Window同时读取真实保留窗口和真实组位置。SQL下一offset须在窗口内，
// broker位置不能领先SQL；调用方据此拒绝缺失历史，而不是接受自动跳转到最新位置。
func (c *Client) Window(ctx context.Context) (map[int]PartitionWindow, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	topic := c.configuration.Topic
	metadata, err := c.api.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
	if err != nil || metadata == nil || len(metadata.Topics) != 1 || metadata.Topics[0].Name != topic ||
		metadata.Topics[0].Error != nil || len(metadata.Topics[0].Partitions) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
	}
	seen := map[int]bool{}
	for _, partition := range metadata.Topics[0].Partitions {
		if partition.ID < 0 || partition.ID >= domain.PartitionCount || seen[partition.ID] || partition.Error != nil || partition.Leader.Host == "" {
			return nil, errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
		}
		seen[partition.ID] = true
	}
	requests := make([]kafka.OffsetRequest, 0, 6)
	for partition := 0; partition < domain.PartitionCount; partition++ {
		requests = append(requests, kafka.FirstOffsetOf(partition), kafka.LastOffsetOf(partition))
	}
	offsets, err := c.api.ListOffsets(ctx, &kafka.ListOffsetsRequest{Topics: map[string][]kafka.OffsetRequest{topic: requests}})
	if err != nil || offsets == nil || len(offsets.Topics) != 1 || len(offsets.Topics[topic]) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_WINDOW_UNAVAILABLE")
	}
	result := make(map[int]PartitionWindow, 3)
	for _, value := range offsets.Topics[topic] {
		if _, exists := result[value.Partition]; exists || value.Partition < 0 || value.Partition >= domain.PartitionCount || value.Error != nil ||
			value.FirstOffset < 0 || value.LastOffset < value.FirstOffset {
			return nil, errors.New("SEARCH_KAFKA_WINDOW_UNBOUND")
		}
		result[value.Partition] = PartitionWindow{First: value.FirstOffset, Last: value.LastOffset, Committed: -1}
	}
	committed, err := c.api.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: c.configuration.Group, Topics: map[string][]int{topic: {0, 1, 2}}})
	if err != nil || committed == nil || committed.Error != nil || len(committed.Topics) != 1 || len(committed.Topics[topic]) != domain.PartitionCount {
		return nil, errors.New("SEARCH_KAFKA_GROUP_UNAVAILABLE")
	}
	seen = map[int]bool{}
	for _, value := range committed.Topics[topic] {
		window, exists := result[value.Partition]
		if !exists || seen[value.Partition] || value.Error != nil || value.CommittedOffset < -1 || value.CommittedOffset > window.Last {
			return nil, errors.New("SEARCH_KAFKA_GROUP_UNBOUND")
		}
		seen[value.Partition] = true
		window.Committed = value.CommittedOffset
		result[value.Partition] = window
	}
	return result, nil
}
