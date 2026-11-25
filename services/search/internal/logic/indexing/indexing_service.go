// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package indexing

import (
	"collaboration/platform/rebuildmeta"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	repository "collaboration/search/internal/repository/event"
	"context"
	"database/sql"
	"errors"
	"time"
)

// record.go：保留原实现，按业务合并文件。

// Record把Kafka的一条记录作为提交单位。全部原行解析完成后才应用，
// 全部应用成功之后才提交SQL下一offset；实际Kafka同步提交属于外层消费者。
func (p *Processor) Record(ctx context.Context, physical string, position repository.Position, raw []byte) error {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	events, err := domain.ParseCanal(raw)
	if err != nil {
		return err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	next, err := repository.Checkpoint(ctx, tx, p.consumer, position.Topic, position.Partition)
	tx.Rollback()
	if err != nil {
		return err
	}
	if position.Offset < 0 || position.Offset > next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	for _, event := range events {
		if err := p.Event(ctx, physical, event, position); err != nil {
			return err
		}
	}
	return p.completeRecord(ctx, position, events)
}

func (p *Processor) completeRecord(ctx context.Context, position repository.Position, events []domain.Event) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err := repository.AdvanceCheckpoint(ctx, tx, p.consumer, position, events); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_CHECKPOINT_COMMIT_UNKNOWN")
	}
	return nil
}

// seed.go：保留原实现，按业务合并文件。

// Seed用于独立应用一个快照条目；重建任务需要原子推进进度时应使用SeedInTransaction。
// 快照条目没有原事件身份，不能伪造event_origin或consumed_event成功回执。
func (p *Processor) Seed(ctx context.Context, physical string, entry rebuildmeta.Entry) error {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err = p.SeedInTransaction(ctx, tx, physical, entry); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_APPLICATION_COMMIT_UNKNOWN")
	}
	return nil
}

// SeedInTransaction沿用调用方的代次、任务锁和事务，使应用水位与快照进度一起提交。
// 此处不提交事务；ES成功但SQL提交未知时，调用方必须保留原条目重试，不能跳过进度。
func (p *Processor) SeedInTransaction(ctx context.Context, tx *sql.Tx, physical string, entry rebuildmeta.Entry) error {
	if tx == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Event仅复用快照查询参数；Minimum可以为零，未发布文档的隐藏权威同样要落盘。
	authority, err := p.source.Current(ctx, domain.Event{Document: entry.Document, Sequence: entry.Minimum, Release: entry.Release})
	if err != nil {
		return publicError(err, "SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	if entry.Document == 0 || authority.Document != entry.Document || authority.Sequence < entry.Minimum {
		return errors.New("SEARCH_CURRENT_AUTHORITY_UNBOUND")
	}
	actual, err := repository.LockApplication(ctx, tx, physical, entry.Document)
	if err != nil {
		return err
	}
	apply, err := index.ShouldApply(actual, authority)
	if err != nil || !apply {
		return err
	}
	actual, err = p.sink.Apply(ctx, physical, authority)
	if err != nil {
		return publicError(err, "SEARCH_ES_APPLICATION_UNKNOWN")
	}
	if actual.Sequence < authority.Sequence ||
		(actual.Sequence == authority.Sequence && (actual.Digest != authority.Digest ||
			actual.Visible != authority.Visible || actual.Release != authority.Release || actual.Policy != authority.Policy)) {
		return errors.New("SEARCH_ES_APPLICATION_UNBOUND")
	}
	// SaveApplication再次校验ES返回的实际高水位，允许并发后继权威，禁止回退或同序换正文。
	return repository.SaveApplication(ctx, tx, physical, entry.Document, actual)
}
