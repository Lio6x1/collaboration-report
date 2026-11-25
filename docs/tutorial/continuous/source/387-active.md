# services/search/internal/repository/generation/active.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/387-active.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Target`、`ValidateNames`、`Find`、`CheckSwitching`、`LockCurrent`、`InsertInitial`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package generation

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
)

type Target struct {
	Physical   string
	Generation uint64
	Revision   uint64
}

var indexName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

func ValidateNames(alias, physical string) error {
	if !indexName.MatchString(alias) || !indexName.MatchString(physical) || alias == physical {
		return errors.New("SEARCH_GENERATION_CONFIG_INVALID")
	}
	return nil
}

// Find由调用方决定共享/独占锁；锁顺序始终先index_generation，再原事件和应用水位。
// 后续重建进入SWITCHING以及切SQL代次也必须先拿这行的独占锁。
func Find(ctx context.Context, tx *sql.Tx, alias string, exclusive bool) (Target, bool, error) {
	var target Target
	if !indexName.MatchString(alias) {
		return target, false, errors.New("SEARCH_ALIAS_INVALID")
	}
	query := "SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=? FOR SHARE"
	if exclusive {
		query = "SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=? FOR UPDATE"
	}
	err := tx.QueryRowContext(ctx, query, alias).Scan(&target.Physical, &target.Generation, &target.Revision)
	if err == sql.ErrNoRows {
		return target, false, nil
	}
	if err != nil {
		return target, false, errors.New("SEARCH_GENERATION_READ_FAILED")
	}
	if ValidateNames(alias, target.Physical) != nil || target.Generation == 0 || target.Revision == 0 {
		return target, false, errors.New("SEARCH_GENERATION_UNBOUND")
	}
	return target, true, nil
}

func CheckSwitching(ctx context.Context, tx *sql.Tx) error {
	var job uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM rebuild_job WHERE state='SWITCHING' ORDER BY id LIMIT 1 FOR SHARE").Scan(&job)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_GENERATION_READ_FAILED")
	}
	return errors.New("SEARCH_GENERATION_RECOVERY_REQUIRED")
}

func LockCurrent(ctx context.Context, tx *sql.Tx, alias string) (Target, error) {
	target, exists, err := Find(ctx, tx, alias, false)
	if err != nil {
		return target, err
	}
	if !exists {
		return target, errors.New("SEARCH_GENERATION_MISSING")
	}
	return target, CheckSwitching(ctx, tx)
}

func InsertInitial(ctx context.Context, tx *sql.Tx, alias, physical string) error {
	if err := ValidateNames(alias, physical); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO index_generation(alias_name,physical_index) VALUES(?,?)", alias, physical); err != nil {
		return errors.New("SEARCH_GENERATION_WRITE_FAILED")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`c397365c29f4aaabca52572af8ca20935502f30d2bb2dd7849e5c7f006c6ee31`。[参考实现](../../../../services/search/internal/repository/generation/active.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
