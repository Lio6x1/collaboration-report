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
