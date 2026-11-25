# services/search/internal/logic/generation/guard.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/393-guard.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Indexes`、`Guard`、`New`、`Initialize`、`Do`、`Ready`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package generation

import (
	repository "collaboration/search/internal/repository/generation"
	"context"
	"database/sql"
	"errors"
	"time"
)

type Indexes interface {
	EnsureInitialAlias(context.Context, string, string) error
	ResolveAlias(context.Context, string) (string, bool, error)
}

type Guard struct {
	database *sql.DB
	indexes  Indexes
	alias    string
	initial  string
}

func New(database *sql.DB, indexes Indexes, alias, initial string) (*Guard, error) {
	if database == nil || indexes == nil {
		return nil, errors.New("SEARCH_GENERATION_DEPENDENCIES_UNAVAILABLE")
	}
	if err := repository.ValidateNames(alias, initial); err != nil {
		return nil, err
	}
	return &Guard{database: database, indexes: indexes, alias: alias, initial: initial}, nil
}

// 初代创建是可核验的两系统恢复流程：先建立确切的owned ES绑定，再提交SQL。
// ES已成功而SQL未知时，重启只能核验原配置的同一绑定，不允许覆盖任意alias。
func (g *Guard) Initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := g.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_GENERATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	target, exists, err := repository.Find(ctx, tx, g.alias, true)
	if err != nil {
		return err
	}
	if err := repository.CheckSwitching(ctx, tx); err != nil {
		return err
	}
	if exists {
		actual, present, err := g.indexes.ResolveAlias(ctx, g.alias)
		if err != nil {
			return err
		}
		if !present || actual != target.Physical {
			return errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
		}
	} else {
		if err := g.indexes.EnsureInitialAlias(ctx, g.alias, g.initial); err != nil {
			return err
		}
		if err := repository.InsertInitial(ctx, tx, g.alias, g.initial); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_GENERATION_COMMIT_UNKNOWN")
	}
	return nil
}

// Do的事务只拥有代次共享锁，不写事件账本。事件事务仍各自明确提交，
// 这个跨完整记录的共享锁阻止重建在中途切代次；它不伪装成ES/SQL/Kafka原子事务。
// 回调必须使用传入的有界context，不能在锁已因取消释放后另起后台继续提交。
func (g *Guard) Do(ctx context.Context, work func(context.Context, repository.Target) error) error {
	if work == nil {
		return errors.New("SEARCH_GENERATION_WORK_REQUIRED")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := g.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_GENERATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	target, err := repository.LockCurrent(ctx, tx, g.alias)
	if err != nil {
		return err
	}
	actual, exists, err := g.indexes.ResolveAlias(ctx, g.alias)
	if err != nil {
		return err
	}
	if !exists || actual != target.Physical {
		return errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
	}
	return work(ctx, target)
}

func (g *Guard) Ready(ctx context.Context) error {
	return g.Do(ctx, func(context.Context, repository.Target) error { return nil })
}
````

UTF-8/LF全文SHA-256：`f930f5fb668a518d41bd3b6c8f7bb95e31b85fdaa48701803f389bb6a06f2668`。[参考实现](../../../../services/search/internal/logic/generation/guard.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
