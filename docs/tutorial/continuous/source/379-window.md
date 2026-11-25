# services/search/internal/client/kafka/window.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/379-window.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`PartitionWindow`、`Window`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`a265c93d5d234ac4cacad97fcd077d9061e43095bac6023e1a9b00bad435a268`。[参考实现](../../../../services/search/internal/client/kafka/window.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
