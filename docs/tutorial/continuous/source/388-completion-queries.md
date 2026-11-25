# services/search/internal/repository/rebuild/completion_queries.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/388-completion-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PendingReplay`、`ReplayCheckpoints`、`TerminalFacts`、`LoadTerminalFacts`、`CheckTerminalFrontier`、`CompleteSwitch`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	events "collaboration/search/internal/domain/event"
	"context"
	"database/sql"
	"errors"
	"math"
)

// replay.go：保留原实现，按业务合并文件。

func PendingReplay(ctx context.Context, db *sql.DB) (string, error) {
	var operation string
	err := db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id
		JOIN rebuild_progress p ON p.job_id=j.id
		WHERE b.state='ACCEPTED' AND (j.state='REPLAYING' OR (j.state='SWITCHING' AND p.validation_digest IS NULL)) ORDER BY j.id LIMIT 1`).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return operation, nil
}

// ReplayCheckpoints要求Start显式建立的三个检查点全部存在；缺行不能沿用普通消费的从0语义。
// 此处只读进度，不持有checkpoint锁跨越Processor自己的事件/整记录提交事务。
func ReplayCheckpoints(ctx context.Context, db *sql.DB, id uint64, topic string, starts, ends [events.PartitionCount]int64) ([events.PartitionCount]int64, error) {
	var result [events.PartitionCount]int64
	for partition := range result {
		var next, revision uint64
		err := db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition).Scan(&next, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			return result, errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if err != nil {
			return result, errors.New("SEARCH_CHECKPOINT_READ_FAILED")
		}
		if next > math.MaxInt64 || revision == 0 || starts[partition] < 0 ||
			int64(next) < starts[partition] || int64(next) > ends[partition] {
			return result, errors.New("SEARCH_REBUILD_CHECKPOINT_UNBOUND")
		}
		result[partition] = int64(next)
	}
	return result, nil
}

// terminal.go：保留原实现，按业务合并文件。

type TerminalFacts struct {
	Cursor, Frontier, Validation []byte
}

func LoadTerminalFacts(ctx context.Context, tx *sql.Tx, id uint64) (TerminalFacts, error) {
	var facts TerminalFacts
	err := tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.frontier_payload,p.validation_digest
		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).
		Scan(&facts.Cursor, &facts.Frontier, &facts.Validation)
	if err != nil {
		return facts, errors.New("SEARCH_REBUILD_TERMINAL_READ_FAILED")
	}
	return facts, nil
}

// CheckTerminalFrontier在同一排他事务内读取原专用检查点；只接受完整到达H，
// 普通消费进度或超过H的不同区间都不能替代本次封存证明。
func CheckTerminalFrontier(ctx context.Context, tx *sql.Tx, id uint64, topic string, ends [events.PartitionCount]int64) error {
	for partition, end := range ends {
		var next, revision uint64
		if err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition).Scan(&next, &revision); err != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_READ_FAILED")
		}
		if end < 0 || next != uint64(end) || revision == 0 {
			return errors.New("SEARCH_REBUILD_FRONTIER_INCOMPLETE")
		}
	}
	return nil
}

// CompleteSwitch不提交事务。调用者持有generation排他锁及原binding/job锁，
// 且已经确认真实alias唯一指向目标；CAS、终态和固定证明必须一起提交。
// 任一语句失败都必须回滚，未知提交结果只能重新读取原任务。
func CompleteSwitch(ctx context.Context, tx *sql.Tx, operation, alias string, job Job, sourceGeneration, sourceRevision uint64, fingerprint, validation, cursor []byte) error {
	if sourceGeneration == 0 || sourceGeneration == ^uint64(0) || sourceRevision == 0 || sourceRevision == ^uint64(0) || job.Generation != sourceGeneration+1 || len(fingerprint) != 32 || len(validation) != 32 || len(cursor) == 0 {
		return errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1
		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`,
		job.Target, job.Generation, alias, job.Source, sourceGeneration, sourceRevision)
	if err != nil {
		return errors.New("SEARCH_REBUILD_GENERATION_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id
		JOIN rebuild_binding b ON b.job_id=j.id SET j.state='SUCCEEDED',j.cursor_json=?,j.last_error=NULL
		WHERE j.id=? AND j.state='SWITCHING' AND j.source_index=? AND j.target_index=? AND j.generation=?
		AND b.operation_id=? AND b.state='ACCEPTED' AND b.request_fingerprint=?
		AND p.frontier_payload IS NOT NULL AND p.validation_digest=?`,
		cursor, job.ID, job.Source, job.Target, job.Generation, operation, fingerprint, validation)
	if err != nil {
		return errors.New("SEARCH_REBUILD_TERMINAL_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`aee59bcf7d31e3fe20376bb1dbc089267dfedde09eff95e3353181366dddea6a`。[参考实现](../../../../services/search/internal/repository/rebuild/completion_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
