// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/logsource"
	domain "collaboration/search/internal/domain/event"
	event "collaboration/search/internal/repository/event"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// replay.go：保留原实现，按业务合并文件。

type ReplayLogs interface {
	Logs
	Read(context.Context, logsource.Window, int, int64, int64) ([]domain.Record, int64, error)
}

type ReplayProcessor interface {
	Record(context.Context, string, event.Position, []byte) error
}

type Replayer struct {
	Seed      *Seeder
	Logs      ReplayLogs
	Processor func(consumer string) ReplayProcessor
}

// Step只推进独立任务的SQL next；当前观察到的末端不是切换H，也不改变普通消费组。
// 每步最多32条完整记录，跨分区没有全局顺序，分区内保留broker原始次序和原字节。
func (r *Replayer) Step(ctx context.Context) error {
	if r == nil || r.Seed == nil || r.Seed.DB == nil || r.Logs == nil || r.Processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation, err := repository.PendingReplay(ctx, r.Seed.DB)
	if err != nil || operation == "" {
		return err
	}
	tx, err := r.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	input, err := r.Seed.lock(ctx, tx, operation)
	var frontier [domain.PartitionCount]int64
	if err == nil && input.job.State == "SWITCHING" {
		frontier, err = repository.LoadFrontier(ctx, tx, input.job.ID)
	}
	tx.Rollback()
	if err != nil {
		return err
	}
	if input.job.State != "REPLAYING" && input.job.State != "SWITCHING" {
		return nil
	}
	original := logsource.Window{Identity: logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(original.Identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(original.Identity.TopicID[:], uuid)
	var starts, ends [domain.PartitionCount]int64
	for partition, raw := range input.prepare.Starts {
		starts[partition], err = strconv.ParseInt(raw, 10, 64)
		if err != nil || starts[partition] < 0 {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_INVALID")
		}
		// Read的原窗口Last始终是持久L，绝不能用恢复next或本轮末端覆盖。
		original.Partitions[partition].Last = starts[partition]
	}
	// 元数据与Fetch都在任务锁之外；Capture/Read核验原Cluster、UUID和严格三个分区。
	window, err := r.Logs.Capture(ctx, &original.Identity)
	if err != nil {
		return err
	}
	if window.Identity != original.Identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	for partition, bounds := range window.Partitions {
		if bounds.First < 0 || bounds.First > starts[partition] || bounds.Last < starts[partition] {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[partition] = bounds.Last
		if input.job.State == "SWITCHING" {
			// 持久屏障之后只补到原H。新发布留给切换后的普通消费，
			// 不能让不断增长的broker末端改写本次完整验证的有限边界。
			if frontier[partition] < starts[partition] || frontier[partition] > bounds.Last {
				return errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
			}
			ends[partition] = frontier[partition]
		}
	}
	next, err := repository.ReplayCheckpoints(ctx, r.Seed.DB, input.job.ID, input.prepare.Topic, starts, ends)
	if err != nil {
		return err
	}
	processor := r.Processor(repository.ConsumerName(input.job.ID))
	if processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	remaining := 32
	for partition := range next {
		if next[partition] == ends[partition] {
			continue
		}
		records, _, err := r.Logs.Read(ctx, original, partition, next[partition], ends[partition])
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.Topic != input.prepare.Topic || record.Partition != partition ||
				record.Offset != next[partition] || record.Offset >= ends[partition] {
				return errors.New("SEARCH_REBUILD_LOG_RECORD_INVALID")
			}
			if err := r.apply(ctx, operation, input, processor, record); err != nil {
				return err
			}
			next[partition]++
			remaining--
			if remaining == 0 {
				return nil
			}
		}
	}
	return nil
}

func (r *Replayer) apply(parent context.Context, operation string, input seedJob, processor ReplayProcessor, record domain.Record) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	tx, err := r.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, err := r.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if current.job.ID != input.job.ID || current.job.State != input.job.State || current.job.Target != input.job.Target {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	// 另一个实例可能已推进next并开始验证；旧批次不能再次向Doc取更新正文，
	// 否则即使offset已完成也会在验证之后改写目标。必须在原job锁内重新判定。
	var next int64
	if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`,
		repository.ConsumerName(current.job.ID), record.Topic, record.Partition).Scan(&next) != nil {
		return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
	}
	if record.Offset < next {
		return nil
	}
	if record.Offset != next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	// 此事务仅持有generation SHARE→binding/job锁，直到整条Record返回才释放。
	// Processor仍明确拥有各应用/事件/整记录checkpoint事务，不能反向取得job锁；
	// 与普通Guard相同，这不是ES/SQL原子事务。ES或SQL提交未知时保留原任务，
	// 下一步从SQL next重读，由原行摘要、物理应用账本及Record幂等恢复，绝不猜进度。
	return processor.Record(ctx, current.job.Target, record.Position, record.Value)
}

// terminal.go：保留原实现，按业务合并文件。

// terminalProof只含已封存事实；JSON中的uint64用字符串保留完整精度。
// 原请求摘要和job绑定使后来代次/当前alias变化不会改写这次操作的答复。
type terminalProof struct {
	Version     int                           `json:"version"`
	State       string                        `json:"state"`
	Operation   string                        `json:"operation_id"`
	Fingerprint string                        `json:"request_fingerprint"`
	JobID       string                        `json:"job_id"`
	Generation  string                        `json:"generation"`
	Revision    string                        `json:"terminal_revision"`
	Alias       string                        `json:"alias"`
	Source      string                        `json:"source_index"`
	Target      string                        `json:"target_index"`
	Snapshot    string                        `json:"snapshot_ref"`
	Ends        [domain.PartitionCount]string `json:"frontier"`
	Validation  string                        `json:"validation_digest"`
}

type terminalEnvelope struct {
	Version int `json:"version"`
	// []byte序列化为base64，MySQL JSON键序/空白规范化不能改变原固定结果字节。
	Fixed []byte `json:"fixed_result"`
}

func fixedRebuildResult(operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte, facts repository.TerminalFacts) (*pb.OperationResult, error) {
	sourceGeneration, err := rebuildmeta.Decimal(p.SourceGeneration, true)
	if err != nil || sourceGeneration == ^uint64(0) || job.Generation != sourceGeneration+1 || strconv.FormatUint(job.Generation, 10) != p.TargetGeneration {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	revision, err := rebuildmeta.Decimal(p.SourceRevision, true)
	if err != nil || revision == ^uint64(0) || len(fingerprint) != 32 || len(facts.Validation) != 32 || bytes.Equal(facts.Validation, make([]byte, 32)) || job.ID == 0 || p.Operation != operation || job.Source != p.Source || job.Target != p.Target {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	ends, err := repository.DecodeFrontier(facts.Frontier)
	if err != nil {
		return nil, err
	}
	var values [domain.PartitionCount]string
	for partition, end := range ends {
		start, err := strconv.ParseInt(p.Starts[partition], 10, 64)
		if err != nil || end < start {
			return nil, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		values[partition] = strconv.FormatInt(end, 10)
	}
	fixed, err := json.Marshal(terminalProof{1, "SUCCEEDED", operation, hex.EncodeToString(fingerprint), strconv.FormatUint(job.ID, 10), strconv.FormatUint(job.Generation, 10), strconv.FormatUint(revision+1, 10), p.Alias, job.Source, job.Target, job.SnapshotRef, values, hex.EncodeToString(facts.Validation)})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	digest := sha256.Sum256(fixed)
	return &pb.OperationResult{State: pb.OperationResult_SUCCEEDED, OperationId: operation, Generation: job.Generation, TerminalRevision: revision + 1, ResultIds: []uint64{job.ID}, ResultDigest: digest[:], FixedResultJsonUtf8: fixed}, nil
}

func loadTerminalResult(ctx context.Context, tx *sql.Tx, operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte) (*pb.OperationResult, error) {
	if job.State != "SUCCEEDED" {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE")
	}
	facts, err := repository.LoadTerminalFacts(ctx, tx, job.ID)
	if err != nil {
		return nil, err
	}
	result, err := fixedRebuildResult(operation, p, job, fingerprint, facts)
	if err != nil {
		return nil, err
	}
	var envelope terminalEnvelope
	if json.Unmarshal(facts.Cursor, &envelope) != nil || envelope.Version != 1 || !bytes.Equal(envelope.Fixed, result.FixedResultJsonUtf8) {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_INVALID")
	}
	return result, nil
}

// finalizeRebuild只落定SQL；调用者在同一排他事务内先验证原alias已经到目标。
// 返回值只有事务提交成功后才能对外发布，提交未知由原号读取固定证明恢复。
func finalizeRebuild(ctx context.Context, tx *sql.Tx, operation string, input seedJob) (*pb.OperationResult, error) {
	bound, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !bound.JobID.Valid || bound.JobID.String != strconv.FormatUint(input.job.ID, 10) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if input.job.State == "SUCCEEDED" {
		return loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)
	}
	if input.job.State != "SWITCHING" {
		return nil, errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	facts, err := repository.LoadTerminalFacts(ctx, tx, input.job.ID)
	if err != nil {
		return nil, err
	}
	ends, err := repository.DecodeFrontier(facts.Frontier)
	if err != nil {
		return nil, err
	}
	if err := repository.CheckTerminalFrontier(ctx, tx, input.job.ID, input.prepare.Topic, ends); err != nil {
		return nil, err
	}
	// 原watermark是MySQL JSON，先还原规范字节再核对原请求摘要。
	var descriptor rebuildmeta.Descriptor
	if json.Unmarshal(input.job.Watermark, &descriptor) != nil {
		return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	watermark, err := descriptor.Bytes()
	if err != nil {
		return nil, err
	}
	expected, err := rebuildmeta.RequestFingerprint(operation, input.job.Generation, input.job.SnapshotRef, watermark)
	if err != nil || !bytes.Equal(expected[:], bound.Fingerprint) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	result, err := fixedRebuildResult(operation, input.prepare, input.job, bound.Fingerprint, facts)
	if err != nil {
		return nil, err
	}
	cursor, err := json.Marshal(terminalEnvelope{1, result.FixedResultJsonUtf8})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	sourceGeneration, _ := rebuildmeta.Decimal(input.prepare.SourceGeneration, true)
	sourceRevision, _ := rebuildmeta.Decimal(input.prepare.SourceRevision, true)
	if err := repository.CompleteSwitch(ctx, tx, operation, input.prepare.Alias, input.job, sourceGeneration, sourceRevision, bound.Fingerprint, facts.Validation, cursor); err != nil {
		return nil, err
	}
	return result, nil
}
