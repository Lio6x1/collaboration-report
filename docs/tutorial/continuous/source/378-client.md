# services/search/internal/client/kafka/client.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/378-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`recordIdentity`、`Client`、`New`、`newReader`、`Fetch`、`Commit`、`Close`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package kafka

import (
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type recordIdentity struct {
	partition int
	offset    int64
	digest    [32]byte
}

// Client只保留一条在途记录；失败时必须处理或重读原记录，不能继续Fetch后面的记录。
// Kafka提交并不证明SQL成功，业务调用方仍须先完成Record，再调用同步Commit。
type Client struct {
	configuration platform.SearchConfig
	api           *kafka.Client
	transport     *kafka.Transport
	mu            sync.Mutex
	reader        *kafka.Reader
	pending       *recordIdentity
	blocked       error
	closed        bool
}

var groupName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)

func New(configuration *platform.SearchConfig) (*Client, error) {
	if configuration == nil || !groupName.MatchString(configuration.Group) || !topicName.MatchString(configuration.Topic) ||
		len(configuration.Brokers) == 0 || len(configuration.Brokers) > 8 {
		return nil, errors.New("SEARCH_KAFKA_CONFIG_INVALID")
	}
	for _, address := range configuration.Brokers {
		host, port, err := net.SplitHostPort(address)
		number, parsed := strconv.ParseUint(port, 10, 16)
		if err != nil || parsed != nil || host == "" || strings.ContainsAny(host, " /\\@?#\t\r\n") || number == 0 || strconv.FormatUint(number, 10) != port {
			return nil, errors.New("SEARCH_KAFKA_CONFIG_INVALID")
		}
	}
	value := *configuration
	value.Brokers = append([]string(nil), configuration.Brokers...)
	transport := &kafka.Transport{DialTimeout: 2 * time.Second, IdleTimeout: 10 * time.Second, MetadataTopics: []string{value.Topic}, ClientID: value.Group}
	return &Client{configuration: value, transport: transport, api: &kafka.Client{Addr: kafka.TCP(value.Brokers...), Timeout: 3 * time.Second, Transport: transport}}, nil
}

func (c *Client) newReader() *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.configuration.Brokers, Topic: c.configuration.Topic, GroupID: c.configuration.Group,
		Dialer:        &kafka.Dialer{Timeout: 2 * time.Second, ClientID: c.configuration.Group},
		QueueCapacity: 1, MinBytes: 1, MaxBytes: domain.MaxRecordBytes + 64<<10, MaxWait: 250 * time.Millisecond, ReadBatchTimeout: time.Second,
		// 不使用ReadMessage，也不设置异步CommitInterval；初次没有组位置必须从最早记录开始。
		CommitInterval: 0, StartOffset: kafka.FirstOffset, ReadLagInterval: -1,
		HeartbeatInterval: 2 * time.Second, SessionTimeout: 10 * time.Second, RebalanceTimeout: 10 * time.Second, JoinGroupBackoff: 500 * time.Millisecond,
		ReadBackoffMin: 100 * time.Millisecond, ReadBackoffMax: 500 * time.Millisecond, MaxAttempts: 3,
		WatchPartitionChanges: true, PartitionWatchInterval: time.Second, OffsetOutOfRangeError: true,
	})
}

func (c *Client) Fetch(ctx context.Context) (domain.Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return domain.Record{}, errors.New("SEARCH_KAFKA_CLIENT_CLOSED")
	}
	if c.blocked != nil {
		return domain.Record{}, c.blocked
	}
	if c.pending != nil {
		return domain.Record{}, errors.New("SEARCH_KAFKA_RECORD_PENDING")
	}
	if c.reader == nil {
		c.reader = c.newReader()
	}
	message, err := c.reader.FetchMessage(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Record{}, errors.New("SEARCH_KAFKA_FETCH_CANCELED")
		}
		if errors.Is(err, kafka.OffsetOutOfRange) {
			return domain.Record{}, errors.New("SEARCH_KAFKA_WINDOW_LOST")
		}
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			return domain.Record{}, errors.New("SEARCH_KAFKA_TOPIC_UNKNOWN")
		}
		return domain.Record{}, errors.New("SEARCH_KAFKA_FETCH_UNAVAILABLE")
	}
	if message.Topic != c.configuration.Topic || message.Partition < 0 || message.Partition >= domain.PartitionCount || message.Offset < 0 ||
		len(message.Value) == 0 || len(message.Value) > domain.MaxRecordBytes {
		// Reader已取走原记录，校验失败后不能再Fetch而越过它。只保存固定阻断状态，
		// 不保留超限正文；Close后由新实例沿broker尚未提交的原位置重读并再次校验。
		c.blocked = errors.New("SEARCH_KAFKA_RECORD_INVALID")
		return domain.Record{}, c.blocked
	}
	c.pending = &recordIdentity{partition: message.Partition, offset: message.Offset, digest: sha256.Sum256(message.Value)}
	return domain.Record{Position: domain.Position{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}, Value: message.Value}, nil
}

func (c *Client) Commit(ctx context.Context, message domain.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.blocked != nil || c.reader == nil || c.pending == nil || message.Topic != c.configuration.Topic ||
		message.Partition != c.pending.partition || message.Offset != c.pending.offset || sha256.Sum256(message.Value) != c.pending.digest {
		return errors.New("SEARCH_KAFKA_COMMIT_UNBOUND")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 已取消的调用不再入提交队列；真正发送后的超时仍是未知结果，保留原在途身份重试。
	if ctx.Err() != nil || c.reader.CommitMessages(ctx, kafka.Message{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}) != nil {
		return errors.New("SEARCH_KAFKA_COMMIT_UNKNOWN")
	}
	c.pending = nil
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	defer c.transport.CloseIdleConnections()
	if c.reader != nil && c.reader.Close() != nil {
		return errors.New("SEARCH_KAFKA_CLOSE_UNKNOWN")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`c03601d9ea89b5bfcfda2a4fb327b469ab6fc619e149ca1e6e44f6d54d6dfdfe`。[参考实现](../../../../services/search/internal/client/kafka/client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
