# services/search/internal/client/logsource/read.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/380-read.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`Read`、`checkRange`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package logsource

import (
	domain "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"time"

	"github.com/twmb/franz-go/pkg/kmsg"
)

// Read从原准备窗口的某分区读取一个有界批次，until为本轮封存的排他前沿。
// 结果只有全部校验成功才交付；调用方应用整条记录后持久推进自己的next，不提交普通组。
func (c *Client) Read(ctx context.Context, original Window, partition int, next, until int64) ([]domain.Record, int64, error) {
	failed := func(code string) ([]domain.Record, int64, error) { return nil, next, errors.New(code) }
	if partition < 0 || partition >= domain.PartitionCount || original.Identity.Topic != c.topic ||
		original.Identity.TopicID == [16]byte{} || original.Partitions[partition].Last < 0 ||
		next < original.Partitions[partition].Last || until < next {
		return failed("SEARCH_LOG_RANGE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	window, err := c.Capture(ctx, &original.Identity)
	if err != nil {
		return nil, next, err
	}
	if err := checkRange(original, window, partition, until); err != nil {
		return nil, next, err
	}
	if next == until {
		return nil, next, nil
	}

	// 只用当前元数据刷新leader路由，Fetch请求仍携带原TopicID，绝不跟随同名新日志。
	metadataRequest := kmsg.NewPtrMetadataRequest()
	metadataRequest.Topics = []kmsg.MetadataRequestTopic{{Topic: &c.topic}}
	metadata, err := metadataRequest.RequestWith(ctx, c.api)
	if err != nil {
		return failed("SEARCH_LOG_ROUTE_UNAVAILABLE")
	}
	identity, err := checkedIdentity(c.topic, metadata)
	if err != nil || identity != original.Identity {
		return failed("SEARCH_LOG_IDENTITY_CHANGED")
	}
	var leader, epoch int32
	for _, route := range metadata.Topics[0].Partitions {
		if route.Partition == int32(partition) {
			leader, epoch = route.Leader, route.LeaderEpoch
		}
	}
	request := kmsg.NewPtrFetchRequest()
	request.MaxWaitMillis, request.MinBytes, request.MaxBytes = 250, 1, 4<<20
	// 无增量fetch session，不产生需要跨重启保存的broker会话。
	request.SessionID, request.SessionEpoch = 0, -1
	part := kmsg.NewFetchRequestTopicPartition()
	part.Partition, part.CurrentLeaderEpoch, part.FetchOffset, part.PartitionMaxBytes = int32(partition), epoch, next, 4<<20
	request.Topics = []kmsg.FetchRequestTopic{{TopicID: original.Identity.TopicID, Partitions: []kmsg.FetchRequestTopicPartition{part}}}
	response, err := request.RequestWith(ctx, c.api.Broker(int(leader)))
	if err != nil {
		return failed("SEARCH_LOG_FETCH_UNAVAILABLE")
	}
	if response == nil || response.Version != 13 || response.ErrorCode != 0 || len(response.Topics) != 1 ||
		response.Topics[0].TopicID != original.Identity.TopicID || len(response.Topics[0].Partitions) != 1 {
		return failed("SEARCH_LOG_FETCH_UNBOUND")
	}
	value := response.Topics[0].Partitions[0]
	if value.ErrorCode == 1 || value.LogStartOffset > original.Partitions[partition].Last {
		return failed("SEARCH_LOG_WINDOW_LOST")
	}
	if value.Partition != int32(partition) || value.ErrorCode != 0 || value.LogStartOffset < 0 ||
		value.HighWatermark < until || len(value.AbortedTransactions) != 0 {
		return failed("SEARCH_LOG_FETCH_UNBOUND")
	}
	records, after, err := decodeRecords(value.RecordBatches, c.topic, partition, next, until)
	if err != nil {
		return nil, next, err
	}
	// 元数据/保留策略变化时也不交付已经解码的前缀；持久原L不能因next前进而改写。
	window, err = c.Capture(ctx, &original.Identity)
	if err != nil {
		return nil, next, err
	}
	if err := checkRange(original, window, partition, until); err != nil {
		return nil, next, err
	}
	return records, after, nil
}

func checkRange(original, current Window, partition int, until int64) error {
	for i := range original.Partitions {
		if original.Partitions[i].Last < current.Partitions[i].First {
			return errors.New("SEARCH_LOG_WINDOW_LOST")
		}
	}
	if until > current.Partitions[partition].Last {
		return errors.New("SEARCH_LOG_FRONTIER_UNAVAILABLE")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`36e4b4bdb52107ef86dafb055fb0842c6f957e3bc4cd4f0b1b8af50b95cb8d29`。[参考实现](../../../../services/search/internal/client/logsource/read.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
