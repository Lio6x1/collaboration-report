# services/search/internal/logic/indexing/processor.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/395-processor.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`SnapshotSource`、`IndexSink`、`Processor`、`New`、`Event`、`receive`、`verifyReplay`、`apply`、`publicError`、`retry`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package indexing

import (
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	repository "collaboration/search/internal/repository/event"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

// 接口由索引用例定义：客户端负责真实协议，用例负责三个系统之间的提交顺序。
type SnapshotSource interface {
	Current(context.Context, domain.Event) (index.Authority, error)
}

type IndexSink interface {
	Apply(context.Context, string, index.Authority) (index.State, error)
}

type Processor struct {
	database *sql.DB
	source   SnapshotSource
	sink     IndexSink
	consumer string
}

func New(database *sql.DB, source SnapshotSource, sink IndexSink, consumer string) *Processor {
	return &Processor{database: database, source: source, sink: sink, consumer: consumer}
}

// Event不提交Kafka。先持久保存原行，再取Doc当前权威，最后在ES成功之后
// 同事务保存实际应用水位与成功回执；中间任何未知结果都允许沿原消息重读。
func (p *Processor) Event(ctx context.Context, physical string, event domain.Event, position repository.Position) (outcome error) {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := p.receive(ctx, event, position)
	if err != nil {
		return err
	}
	if receipt.State == "REJECTED" {
		return errors.New("SEARCH_ORIGINAL_EVENT_REJECTED")
	}
	defer func() {
		if outcome != nil {
			p.retry(event, position, outcome)
		}
	}()
	if receipt.State == "SUCCEEDED" && receipt.Physical == physical {
		return p.verifyReplay(ctx, physical, event, position)
	}
	// 网络请求不占用原事件事务；Doc不可达或未知不能被合成为删除墓碑。
	authority, err := p.source.Current(ctx, event)
	if err != nil {
		return publicError(err, "SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	if authority.Document != event.Document || authority.Sequence < event.Sequence {
		return errors.New("SEARCH_CURRENT_AUTHORITY_UNBOUND")
	}
	return p.apply(ctx, physical, event, position, authority)
}

func (p *Processor) receive(ctx context.Context, event domain.Event, position repository.Position) (repository.Receipt, error) {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return repository.Receipt{}, errors.New("SEARCH_LEDGER_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return receipt, err
	}
	if receipt.State != "SUCCEEDED" && receipt.State != "REJECTED" {
		if err = repository.SetApplying(ctx, tx, p.consumer, event.ID); err != nil {
			return receipt, err
		}
	}
	if tx.Commit() != nil {
		return receipt, errors.New("SEARCH_ORIGIN_COMMIT_UNKNOWN")
	}
	return receipt, nil
}

func (p *Processor) verifyReplay(ctx context.Context, physical string, event domain.Event, position repository.Position) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_REPLAY_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return err
	}
	actual, err := repository.LockApplication(ctx, tx, physical, event.Document)
	if err != nil || receipt.State != "SUCCEEDED" || receipt.Physical != physical ||
		actual.Digest == ([32]byte{}) || actual.Sequence < receipt.Applied.Sequence ||
		(actual.Sequence == receipt.Applied.Sequence && actual.Digest != receipt.Applied.Digest) {
		return errors.New("SEARCH_REPLAY_APPLICATION_UNBOUND")
	}
	// 只读核验后回滚，不能因重读增加应用revision或提交缺失状态的零摘要占位。
	return nil
}

func (p *Processor) apply(ctx context.Context, physical string, event domain.Event, position repository.Position, authority index.Authority) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return err
	}
	if receipt.State == "REJECTED" {
		return errors.New("SEARCH_ORIGINAL_EVENT_REJECTED")
	}
	actual, err := repository.LockApplication(ctx, tx, physical, event.Document)
	if err != nil {
		return err
	}
	apply, err := index.ShouldApply(actual, authority)
	if err != nil {
		return err
	}
	if apply {
		actual, err = p.sink.Apply(ctx, physical, authority)
		if err != nil {
			return publicError(err, "SEARCH_ES_APPLICATION_UNKNOWN")
		}
		if actual.Sequence < authority.Sequence ||
			(actual.Sequence == authority.Sequence && (actual.Digest != authority.Digest ||
				actual.Visible != authority.Visible || actual.Release != authority.Release || actual.Policy != authority.Policy)) {
			return errors.New("SEARCH_ES_APPLICATION_UNBOUND")
		}
		if err = repository.SaveApplication(ctx, tx, physical, event.Document, actual); err != nil {
			return err
		}
	}
	// 后来事件已持久应用时使用实际高水位；重建到另一物理索引也不改写
	// 原事件第一次成功绑定的物理代次和权威摘要。
	if receipt.State != "SUCCEEDED" {
		if err = repository.Complete(ctx, tx, p.consumer, event.ID, physical, actual); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_APPLICATION_COMMIT_UNKNOWN")
	}
	return nil
}

var safeCode = regexp.MustCompile(`^SEARCH_[A-Z0-9_]{1,56}$`)

func publicError(err error, fallback string) error {
	if safeCode.MatchString(err.Error()) {
		return err
	}
	return errors.New(fallback)
}

func (p *Processor) retry(event domain.Event, position repository.Position, cause error) {
	// 主任务取消后只做有界的失败记账。记账本身未知时仍保留APPLYING；
	// 既有SUCCEEDED不能被迟到的错误回写为RETRY。
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil || receipt.State == "SUCCEEDED" || receipt.State == "REJECTED" {
		return
	}
	code := publicError(cause, "SEARCH_INDEXING_RETRY_REQUIRED").Error()
	if repository.SetRetry(ctx, tx, p.consumer, event.ID, code) == nil {
		tx.Commit()
	}
}
````

UTF-8/LF全文SHA-256：`22b922df61200d697bca97233d4c4b2341777e7223f4107d811f3ca9be66b262`。[参考实现](../../../../services/search/internal/logic/indexing/processor.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
