package consumer

import (
	domain "collaboration/search/internal/domain/event"
	repository "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// Broker仅暴露消费窗口、单条读取与确认；业务不依赖具体Kafka客户端。
type Broker interface {
	Window(context.Context) (map[int]domain.PartitionWindow, error)
	Fetch(context.Context) (domain.Record, error)
	Commit(context.Context, domain.Record) error
}

// Generations把一次投影及其确认绑定到同一物理索引，避免重建切换时写错代次。
type Generations interface {
	Initialize(context.Context) error
	Ready(context.Context) error
	Do(context.Context, func(context.Context, generation.Target) error) error
}

// Indexer按消息位置及内容记录投影，内部维护索引写入与SQL回执的可恢复顺序。
type Indexer interface {
	Record(context.Context, string, repository.Position, []byte) error
}

// Consumer持有当前未确认消息及恢复状态；Step由互斥锁串行执行，Ready可并发读取。
type Consumer struct {
	database     *sql.DB
	broker       Broker
	generation   Generations
	indexer      Indexer
	group, topic string
	stepMu       sync.Mutex
	current      *domain.Record
	stateMu      sync.RWMutex
	initialized  bool
	frontier     map[int]int64
	failure      error
}

func New(database *sql.DB, broker Broker, generations Generations, indexer Indexer, group, topic string) *Consumer {
	return &Consumer{database: database, broker: broker, generation: generations, indexer: indexer, group: group, topic: topic}
}

// Step串行拥有一条在途记录。任何失败都留下原current，下一次Step重做原记录，
// 不Fetch后面的记录，也不根据Reader预取的最高offset批量提交。
func (c *Consumer) Step(ctx context.Context) (outcome error) {
	c.stepMu.Lock()
	defer c.stepMu.Unlock()
	defer func() { c.stateMu.Lock(); c.failure = outcome; c.stateMu.Unlock() }()
	if !c.initialized {
		if err := c.initialize(ctx); err != nil {
			return err
		}
	}
	if c.current == nil {
		poll, done := context.WithTimeout(ctx, 2*time.Second)
		record, err := c.broker.Fetch(poll)
		done()
		if err != nil {
			if ctx.Err() == nil && err.Error() == "SEARCH_KAFKA_FETCH_CANCELED" {
				return c.inspect(ctx)
			}
			return err
		}
		c.current = &record
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, false)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	// 本条消息绑定当前物理索引代次：索引投影、SQL回执和Kafka提交是独立步骤。
	// Record内部按原序号/摘要允许已成功部分重放，不能把Kafka提交当跨系统事务。
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		if err := c.indexer.Record(ctx, target.Physical, c.current.Position, c.current.Value); err != nil {
			return err
		}
		// SQL整条记录已成功后才能同步Kafka；这个独立动作超时仍保留原记录重试。
		return c.broker.Commit(ctx, *c.current)
	})
	if err != nil {
		return err
	}
	// 两个步骤都确认成功才丢掉原记录；否则下一轮继续原消息，不越过失败位置。
	c.current = nil
	return nil
}

func (c *Consumer) initialize(ctx context.Context) error {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()
	if err := c.generation.Initialize(ctx); err != nil {
		return err
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, true)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	frontier := make(map[int]int64, 3)
	for partition, value := range window {
		frontier[partition] = value.Last
	}
	c.stateMu.Lock()
	c.frontier = frontier
	c.initialized = true
	c.stateMu.Unlock()
	return nil
}

// Ready同时检查初始化、最近失败、消息保留窗口与启动时的消费水位。
// 进程存活或Kafka连接成功不足以证明索引已完成恢复，失败时不能报告可查询。
func (c *Consumer) Ready(ctx context.Context) error {
	c.stateMu.RLock()
	initialized, failure := c.initialized, c.failure
	c.stateMu.RUnlock()
	if !initialized {
		return errors.New("SEARCH_CONSUMER_RECOVERY_REQUIRED")
	}
	if failure != nil {
		return errors.New("SEARCH_CONSUMER_RECOVERY_REQUIRED")
	}
	return c.inspect(ctx)
}

func (c *Consumer) inspect(ctx context.Context) error {
	if err := c.generation.Ready(ctx); err != nil {
		return err
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, false)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	for partition, offset := range c.frontier {
		if positions[partition] < offset {
			return errors.New("SEARCH_CONSUMER_CATCHUP_REQUIRED")
		}
	}
	return nil
}

func (c *Consumer) checkpoints(ctx context.Context, audit bool) (map[int]int64, error) {
	tx, err := c.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if audit {
		if err := repository.CheckRecovery(ctx, tx, c.group, c.topic); err != nil {
			return nil, err
		}
	}
	result := make(map[int]int64, 3)
	for partition := 0; partition < domain.PartitionCount; partition++ {
		next, err := repository.Checkpoint(ctx, tx, c.group, c.topic, partition)
		if err != nil {
			return nil, err
		}
		result[partition] = next
	}
	return result, nil
}

func validateWindow(window map[int]domain.PartitionWindow, positions map[int]int64) error {
	if len(window) != domain.PartitionCount || len(positions) != domain.PartitionCount {
		return errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
	}
	for partition := 0; partition < domain.PartitionCount; partition++ {
		value, exists := window[partition]
		next, present := positions[partition]
		if !exists || !present || next < 0 || value.First < 0 || value.Last < value.First || next < value.First ||
			next > value.Last || value.Committed < -1 || value.Committed > next ||
			(value.Committed >= 0 && value.Committed < value.First) {
			return errors.New("SEARCH_KAFKA_WINDOW_LOST")
		}
	}
	return nil
}
