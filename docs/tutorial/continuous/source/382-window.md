# services/search/internal/client/logsource/window.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/382-window.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`Identity`、`Bounds`、`Window`、`Client`、`New`、`Close`、`Capture`、`identity`、`checkedIdentity`、`offsets`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// Package logsource提供重建专用的只读日志入口，不加入普通消费组，也不提交其offset。
package logsource

import (
	events "collaboration/search/internal/domain/event"
	"context"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/kversion"
)

// Identity在准备阶段封存。名字相同但TopicID不同的日志不能续接原任务。
// leader不属于持久身份，正常选举允许刷新路由；分区集合固定为0、1、2。
type Identity struct {
	Cluster string
	Topic   string
	TopicID [16]byte
}

type Bounds struct{ First, Last int64 }
type Window struct {
	Identity   Identity
	Partitions [events.PartitionCount]Bounds
}

type Client struct {
	api   *kgo.Client
	topic string
}

var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

func New(brokers []string, topic string) (*Client, error) {
	if len(brokers) == 0 || len(brokers) > 8 || !topicName.MatchString(topic) {
		return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
	}
	for _, address := range brokers {
		host, port, err := net.SplitHostPort(address)
		value, parseErr := strconv.ParseUint(port, 10, 16)
		if err != nil || parseErr != nil || value == 0 || host == "" || strings.ContainsAny(host, " /\\@?#\t\r\n") || strconv.FormatUint(value, 10) != port {
			return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
		}
	}
	maximum := kversion.V3_7_0()
	minimum := new(kversion.Versions)
	// Metadata v10开始返回TopicID；Fetch v13才按TopicID读取。禁止协议降级。
	for key, version := range map[int16]int16{3: 10, 2: 5, 1: 13} {
		minimum.SetMaxKeyVersion(key, version)
		maximum.SetMaxKeyVersion(key, version)
	}
	api, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ClientID("collab-rebuild"),
		kgo.MinVersions(minimum), kgo.MaxVersions(maximum), kgo.DialTimeout(2*time.Second),
		kgo.RequestTimeoutOverhead(3*time.Second), kgo.BrokerMaxReadBytes(8<<20), kgo.FetchMaxBytes(4<<20), kgo.RequestRetries(0))
	if err != nil {
		return nil, errors.New("SEARCH_LOG_CONFIG_INVALID")
	}
	return &Client{api: api, topic: topic}, nil
}

func (c *Client) Close() { c.api.Close() }

// Capture先后核验身份，再读取窗口。它证明观察期间身份一致，不声称几个broker请求原子执行。
// expected在恢复时必须传原准备身份；调用方不得把新捕获的身份覆盖原任务。
func (c *Client) Capture(ctx context.Context, expected *Identity) (Window, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	before, err := c.identity(ctx)
	if err != nil {
		return Window{}, err
	}
	if expected != nil && before != *expected {
		return Window{}, errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	first, err := c.offsets(ctx, -2)
	if err != nil {
		return Window{}, err
	}
	last, err := c.offsets(ctx, -1)
	if err != nil {
		return Window{}, err
	}
	after, err := c.identity(ctx)
	if err != nil {
		return Window{}, err
	}
	if before != after {
		return Window{}, errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	window := Window{Identity: before}
	for partition := range window.Partitions {
		if last[partition] < first[partition] {
			return Window{}, errors.New("SEARCH_LOG_WINDOW_INVALID")
		}
		window.Partitions[partition] = Bounds{First: first[partition], Last: last[partition]}
	}
	return window, nil
}

func (c *Client) identity(ctx context.Context) (Identity, error) {
	request := kmsg.NewPtrMetadataRequest()
	request.Topics = []kmsg.MetadataRequestTopic{{Topic: &c.topic}}
	request.AllowAutoTopicCreation = false
	response, err := request.RequestWith(ctx, c.api)
	if err != nil {
		return Identity{}, errors.New("SEARCH_LOG_IDENTITY_UNAVAILABLE")
	}
	return checkedIdentity(c.topic, response)
}

func checkedIdentity(topic string, response *kmsg.MetadataResponse) (Identity, error) {
	invalid := errors.New("SEARCH_LOG_IDENTITY_INVALID")
	if response == nil || response.Version < 10 || response.ClusterID == nil || *response.ClusterID == "" || len(response.Topics) != 1 {
		return Identity{}, invalid
	}
	value := response.Topics[0]
	if value.ErrorCode != 0 || value.Topic == nil || *value.Topic != topic || value.TopicID == [16]byte{} || len(value.Partitions) != events.PartitionCount {
		return Identity{}, invalid
	}
	var seen [events.PartitionCount]bool
	for _, partition := range value.Partitions {
		if partition.Partition < 0 || partition.Partition >= events.PartitionCount || seen[partition.Partition] || partition.ErrorCode != 0 || partition.Leader < 0 {
			return Identity{}, invalid
		}
		seen[partition.Partition] = true
	}
	return Identity{Cluster: *response.ClusterID, Topic: topic, TopicID: value.TopicID}, nil
}

func (c *Client) offsets(ctx context.Context, timestamp int64) ([events.PartitionCount]int64, error) {
	request := kmsg.NewPtrListOffsetsRequest()
	topic := kmsg.NewListOffsetsRequestTopic()
	topic.Topic = c.topic
	for i := int32(0); i < events.PartitionCount; i++ {
		partition := kmsg.NewListOffsetsRequestTopicPartition()
		partition.Partition, partition.Timestamp = i, timestamp
		topic.Partitions = append(topic.Partitions, partition)
	}
	request.Topics = []kmsg.ListOffsetsRequestTopic{topic}
	response, err := request.RequestWith(ctx, c.api)
	if err != nil || response == nil || len(response.Topics) != 1 || response.Topics[0].Topic != c.topic || len(response.Topics[0].Partitions) != events.PartitionCount {
		return [events.PartitionCount]int64{}, errors.New("SEARCH_LOG_WINDOW_UNAVAILABLE")
	}
	var result [events.PartitionCount]int64
	var seen [events.PartitionCount]bool
	for _, partition := range response.Topics[0].Partitions {
		if partition.Partition < 0 || partition.Partition >= events.PartitionCount || seen[partition.Partition] || partition.ErrorCode != 0 || partition.Offset < 0 {
			return [events.PartitionCount]int64{}, errors.New("SEARCH_LOG_WINDOW_INVALID")
		}
		seen[partition.Partition] = true
		result[partition.Partition] = partition.Offset
	}
	return result, nil
}
````

UTF-8/LF全文SHA-256：`0598306e0f4fabdf2581ea59e0efde955c8c2358b2b9a443fed4a4c4c3e7dedc`。[参考实现](../../../../services/search/internal/client/logsource/window.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
