# services/search/internal/svc/consumer_wiring.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/404-consumer-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`boundBroker`、`capture`、`Window`、`Fetch`、`Commit`、`Close`、`Consumer`、`NewConsumer`、`Step`、`Ready`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"bytes"
	"collaboration/platform"
	snapshots "collaboration/search/internal/client/document"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/kafka"
	"collaboration/search/internal/client/logsource"
	domain "collaboration/search/internal/domain/event"
	logic "collaboration/search/internal/logic/consumer"
	"collaboration/search/internal/logic/generation"
	"collaboration/search/internal/logic/indexing"
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// bound_broker.go：保留原实现，按业务合并文件。

// boundBroker保留成熟消费组实现，但所有交付给索引器的原字节必须由UUID Fetch再次确认。
// 持久绑定由服务自己的SQL用例提供；失败时缓存原在途记录，绝不越过它继续Fetch。
type boundBroker struct {
	group      *kafka.Client
	logs       *logsource.Client
	bind       func(context.Context, logsource.Identity) error
	mu         sync.Mutex
	identityMu sync.Mutex
	identity   *logsource.Identity
	pending    *domain.Record
	committed  *domain.Record
}

func (b *boundBroker) capture(ctx context.Context) (logsource.Window, error) {
	b.identityMu.Lock()
	defer b.identityMu.Unlock()
	w, err := b.logs.Capture(ctx, b.identity)
	if err != nil {
		return w, err
	}
	if b.identity == nil {
		if err := b.bind(ctx, w.Identity); err != nil {
			return w, err
		}
		identity := w.Identity
		b.identity = &identity
	}
	return w, nil
}

func (b *boundBroker) Window(ctx context.Context) (map[int]domain.PartitionWindow, error) {
	// Ready的窗口检查不等待最多两秒的组Fetch；只有首次身份绑定需要独立串行。
	// kafka.Client.Window使用元数据连接，不读取reader/pending状态。
	if _, err := b.capture(ctx); err != nil {
		return nil, err
	}
	w, err := b.group.Window(ctx)
	if err != nil {
		return nil, err
	}
	// 组API按名称读取，前后身份检查防止将同名新主题的窗口误认为原日志。
	if _, err := b.capture(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func (b *boundBroker) Fetch(ctx context.Context) (domain.Record, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w, err := b.capture(ctx)
	if err != nil {
		return domain.Record{}, err
	}
	if b.pending == nil {
		record, err := b.group.Fetch(ctx)
		if err != nil {
			return domain.Record{}, err
		}
		b.pending = &record
	}
	record := *b.pending
	if record.Offset == int64(^uint64(0)>>1) {
		return domain.Record{}, errors.New("SEARCH_LOG_RANGE_INVALID")
	}
	// 原窗口的L取当前保留起点，仅用于此条逐字节核对；持久消费next仍独立由SQL校验。
	for p, bounds := range w.Partitions {
		w.Partitions[p].Last = bounds.First
	}
	rows, _, err := b.logs.Read(ctx, w, record.Partition, record.Offset, record.Offset+1)
	if err != nil {
		return domain.Record{}, err
	}
	if len(rows) != 1 || rows[0].Position != record.Position || !bytes.Equal(rows[0].Value, record.Value) {
		return domain.Record{}, errors.New("SEARCH_KAFKA_RECORD_IDENTITY_CHANGED")
	}
	b.pending = nil
	return record, nil
}

func (b *boundBroker) Commit(ctx context.Context, record domain.Record) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.capture(ctx); err != nil {
		return err
	}
	if b.committed != nil {
		if b.committed.Position != record.Position || !bytes.Equal(b.committed.Value, record.Value) {
			return errors.New("SEARCH_KAFKA_COMMIT_UNBOUND")
		}
		b.committed = nil
		return nil
	}
	if err := b.group.Commit(ctx, record); err != nil {
		return err
	}
	// 提交仍是名称协议，不声称与元数据原子；变化后拒绝继续使用原SQL进度。
	b.committed = &record
	_, err := b.capture(ctx)
	if err == nil {
		b.committed = nil
	}
	return err
}

func (b *boundBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logs.Close()
	return b.group.Close()
}

// consumer.go：保留原实现，按业务合并文件。

// svc组合本服务的私有库和客户端生命周期，业务提交顺序留给logic。
type Consumer struct {
	logic   *logic.Consumer
	kafka   *boundBroker
	elastic *elastic.Client
	closed  atomic.Bool
}

func NewConsumer(store *platform.Store, config platform.Config) (*Consumer, error) {
	if store == nil || store.DB == nil || store.Role != "search" || config.Role != "search" || config.Search == nil {
		return nil, errors.New("SEARCH_CONSUMER_CONFIG_INVALID")
	}
	source, err := snapshots.NewSnapshots(config)
	if err != nil {
		return nil, err
	}
	sink, err := elastic.New(config.Search)
	if err != nil {
		return nil, err
	}
	assembled := false
	defer func() {
		if !assembled {
			sink.Close()
		}
	}()
	broker, err := kafka.New(config.Search)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !assembled {
			broker.Close()
		}
	}()
	guard, err := generation.New(store.DB, sink, config.Search.Alias, config.Search.InitialIndex)
	if err != nil {
		return nil, err
	}
	processor := indexing.New(store.DB, source, sink, config.Search.Group)
	logs, err := logsource.New(config.Search.Brokers, config.Search.Topic)
	if err != nil {
		return nil, err
	}
	bound := &boundBroker{group: broker, logs: logs, bind: func(ctx context.Context, identity logsource.Identity) error {
		return bindConsumerLog(ctx, store.DB, config.Search, identity)
	}}
	consumer := logic.New(store.DB, bound, guard, processor, config.Search.Group, config.Search.Topic)
	assembled = true
	return &Consumer{logic: consumer, kafka: bound, elastic: sink}, nil
}

func (c *Consumer) Step(ctx context.Context) error { return c.logic.Step(ctx) }
func (c *Consumer) Ready(ctx context.Context) error {
	if c.closed.Load() {
		return errors.New("SEARCH_CONSUMER_STOPPED")
	}
	return c.logic.Ready(ctx)
}
func (c *Consumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	defer c.elastic.Close()
	return c.kafka.Close()
}
````

UTF-8/LF全文SHA-256：`737ba277fb7d97a3106d0e57e235a244e54a7fd7faa838cef444990e935dff74`。[参考实现](../../../../services/search/internal/svc/consumer_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
