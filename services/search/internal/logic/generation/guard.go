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
