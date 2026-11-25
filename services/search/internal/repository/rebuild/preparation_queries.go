// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	events "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
)

// binding.go：保留原实现，按业务合并文件。

type Binding struct {
	Prepare  rebuildmeta.Prepare
	Envelope []byte
	State    string
}

// Find由已持有代次锁的用例调用。恢复核数据库原接受摘要，不按当前证书期限重新签名。
func Find(ctx context.Context, tx *sql.Tx, operation string) (Binding, bool, error) {
	var row Binding
	var digest, signer []byte
	var alias, source, target, sourceGeneration, sourceRevision, targetGeneration string
	err := tx.QueryRowContext(ctx, `SELECT prepare_envelope,prepare_digest,prepare_signer_digest,state,alias_name,source_index,target_index,
		CAST(source_generation AS CHAR),CAST(source_revision AS CHAR),CAST(target_generation AS CHAR)
		FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&row.Envelope, &digest, &signer, &row.State, &alias, &source, &target, &sourceGeneration, &sourceRevision, &targetGeneration)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	if len(digest) != 32 || len(signer) != 32 {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	var expected, identity [32]byte
	copy(expected[:], digest)
	copy(identity[:], signer)
	accepted, err := rebuildartifact.RestoreAccepted(row.Envelope, rebuildartifact.PrepareV1, expected, identity)
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	row.Prepare, err = rebuildmeta.ParsePrepare(accepted.Payload)
	p := row.Prepare
	if err != nil || p.Operation != operation || p.Alias != alias || p.Source != source || p.Target != target || p.SourceGeneration != sourceGeneration || p.SourceRevision != sourceRevision || p.TargetGeneration != targetGeneration {
		return Binding{}, false, errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	return row, true, nil
}

func CheckAvailable(ctx context.Context, tx *sql.Tx, alias string) error {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b LEFT JOIN rebuild_job j ON j.id=b.job_id
		WHERE b.alias_name=? AND (b.state IN ('PREPARED','IMPORTED') OR
		(b.state='ACCEPTED' AND (j.id IS NULL OR j.state NOT IN ('SUCCEEDED','REJECTED')))) LIMIT 1 FOR UPDATE`, alias).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	return errors.New("SEARCH_REBUILD_ALREADY_PENDING")
}

func Insert(ctx context.Context, tx *sql.Tx, p rebuildmeta.Prepare, envelope []byte, signer string) error {
	if _, err := p.Bytes(); err != nil {
		return err
	}
	identity, err := hex.DecodeString(signer)
	if err != nil || len(identity) != 32 || len(envelope) == 0 || len(envelope) > rebuildartifact.MaxEnvelopeBytes {
		return errors.New("SEARCH_REBUILD_BINDING_INVALID")
	}
	digest := sha256.Sum256(envelope)
	_, err = tx.ExecContext(ctx, `INSERT INTO rebuild_binding(operation_id,alias_name,source_index,source_generation,source_revision,target_index,target_generation,
		prepare_digest,prepare_signer_digest,prepare_envelope,state) VALUES(?,?,?,?,?,?,?,?,?,?,'PREPARED')`,
		p.Operation, p.Alias, p.Source, p.SourceGeneration, p.SourceRevision, p.Target, p.TargetGeneration, digest[:], identity, envelope)
	if err != nil {
		return errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	return nil
}

// frontier.go：保留原实现，按业务合并文件。

// Frontier是切换前沿的不可变版本化载荷。offset使用十进制字符串，
// 避免JSON数字经过其他语言时损失精度；L仍只从原prepare读取。
type Frontier struct {
	Version int                           `json:"version"`
	Ends    [events.PartitionCount]string `json:"ends"`
}

func EncodeFrontier(ends [events.PartitionCount]int64) ([]byte, error) {
	f := Frontier{Version: 1}
	for p, end := range ends {
		if end < 0 {
			return nil, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		f.Ends[p] = strconv.FormatInt(end, 10)
	}
	return json.Marshal(f)
}

func DecodeFrontier(raw []byte) ([events.PartitionCount]int64, error) {
	var ends [events.PartitionCount]int64
	var f Frontier
	if json.Unmarshal(raw, &f) != nil || f.Version != 1 {
		return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
	}
	for p, value := range f.Ends {
		end, err := strconv.ParseInt(value, 10, 64)
		if err != nil || end < 0 || strconv.FormatInt(end, 10) != value {
			return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		ends[p] = end
	}
	canonical, _ := EncodeFrontier(ends)
	if !bytes.Equal(canonical, raw) {
		return ends, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
	}
	return ends, nil
}

// SealFrontier由持有generation独占锁→原binding/job锁的业务事务调用。
// H与SWITCHING必须一起提交；未知结果只能重读原载荷，不能重新捕获H覆盖它。
func SealFrontier(ctx context.Context, tx *sql.Tx, id uint64, ends [events.PartitionCount]int64) error {
	payload, err := EncodeFrontier(ends)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET frontier_payload=?,revision=revision+1
		WHERE job_id=? AND frontier_payload IS NULL AND validation_digest IS NULL AND revision<18446744073709551615`, payload, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_FRONTIER_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_job SET state='SWITCHING',last_error=NULL WHERE id=? AND state='REPLAYING'`, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_FRONTIER_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}

func LoadFrontier(ctx context.Context, tx *sql.Tx, id uint64) ([events.PartitionCount]int64, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT frontier_payload FROM rebuild_progress WHERE job_id=? FOR UPDATE`, id).Scan(&raw); err != nil {
		return [events.PartitionCount]int64{}, errors.New("SEARCH_REBUILD_FRONTIER_READ_FAILED")
	}
	return DecodeFrontier(raw)
}

// jobs.go：保留原实现，按业务合并文件。

// JobBinding只承载Search私有库事实；调用者先取得generation及binding锁。
type JobBinding struct {
	Fingerprint []byte
	Revision    uint64
	JobID       sql.NullString
}

func LoadJobBinding(ctx context.Context, tx *sql.Tx, operation string) (JobBinding, error) {
	var row JobBinding
	err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,revision,CAST(job_id AS CHAR) FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&row.Fingerprint, &row.Revision, &row.JobID)
	if err != nil {
		return row, errors.New("SEARCH_REBUILD_BINDING_READ_FAILED")
	}
	return row, nil
}

type Job struct {
	ID, Generation, SnapshotAfter      uint64
	State, SnapshotRef, Target, Source string
	Watermark                          []byte
}

func LoadJob(ctx context.Context, tx *sql.Tx, id uint64) (Job, error) {
	var j Job
	err := tx.QueryRowContext(ctx, `SELECT j.id,j.state,j.snapshot_ref,j.watermark,j.target_index,j.source_index,j.generation,p.snapshot_after
		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).
		Scan(&j.ID, &j.State, &j.SnapshotRef, &j.Watermark, &j.Target, &j.Source, &j.Generation, &j.SnapshotAfter)
	if err != nil {
		return j, errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return j, nil
}

// ConsumerName以数据库任务主键隔离重放账本，最长28字节，满足003的40字节限制。
func ConsumerName(id uint64) string { return "rebuild-" + strconv.FormatUint(id, 10) }

// AcceptJob的任务、三分区原L及请求绑定必须在同一个业务事务提交。
// 不创建Kafka group，也绝不借用普通消费者的SQL检查点。
func AcceptJob(ctx context.Context, tx *sql.Tx, operation string, fingerprint []byte, j Job, topic string, starts [events.PartitionCount]string) (uint64, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO rebuild_job(state,snapshot_ref,watermark,target_index,source_index,generation) VALUES('SNAPSHOT',?,?,?,?,?)`, j.SnapshotRef, j.Watermark, j.Target, j.Source, j.Generation)
	if err != nil {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	// LAST_INSERT_ID绑定当前事务连接，直接扫描uint64避免经过driver的int64接口。
	var id uint64
	if err := tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id); err != nil || id == 0 {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO rebuild_progress(job_id) VALUES(?)`, id); err != nil {
		return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
	}
	for partition, start := range starts {
		if _, err = tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) VALUES(?,?,?,?)`, ConsumerName(id), topic, partition, start); err != nil {
			return 0, errors.New("SEARCH_REBUILD_JOB_WRITE_FAILED")
		}
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_binding SET state='ACCEPTED',request_fingerprint=?,job_id=?,revision=revision+1 WHERE operation_id=? AND state='IMPORTED' AND revision<18446744073709551615`, fingerprint, id, operation)
	if err != nil {
		return 0, errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return 0, errors.New("SEARCH_REBUILD_BINDING_WRITE_FAILED")
	}
	return id, nil
}

// validation.go：保留原实现，按业务合并文件。

type Validation struct {
	Version  int    `json:"version"`
	Phase    string `json:"phase"`
	Count    string `json:"count"`
	Digest   string `json:"digest"`
	After    uint64 `json:"-"`
	Complete bool   `json:"-"`
}

func LoadValidation(ctx context.Context, tx *sql.Tx, id uint64) (Validation, error) {
	var v Validation
	var raw, digest []byte
	var after uint64
	if tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.validation_after,p.validation_digest FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).Scan(&raw, &after, &digest) != nil {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_READ_FAILED")
	}
	if len(raw) == 0 || string(raw) == "null" {
		v = Validation{Version: 1, Phase: "SNAPSHOT", Count: "0", Digest: hex.EncodeToString(make([]byte, 32))}
	} else if json.Unmarshal(raw, &v) != nil {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	_, err := strconv.ParseUint(v.Count, 10, 64)
	hash, hashErr := hex.DecodeString(v.Digest)
	if v.Version != 1 || (v.Phase != "SNAPSHOT" && v.Phase != "APPLICATION") || err != nil || hashErr != nil || len(hash) != 32 {
		return v, errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	v.After, v.Complete = after, len(digest) == 32
	return v, nil
}

// SaveValidation与调用者持有的原job锁同事务提交分页进度；完成摘要只封存一次。
func SaveValidation(ctx context.Context, tx *sql.Tx, id uint64, v Validation, complete bool) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
	}
	var digest any
	if complete {
		digest, err = hex.DecodeString(v.Digest)
		if err != nil {
			return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET validation_after=?,validation_digest=?,revision=revision+1 WHERE job_id=? AND frontier_payload IS NOT NULL AND validation_digest IS NULL AND revision<18446744073709551615`, v.After, digest, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_WRITE_FAILED")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE rebuild_job SET cursor_json=? WHERE id=? AND state='SWITCHING'`, raw, id); err != nil {
		return errors.New("SEARCH_REBUILD_VALIDATION_WRITE_FAILED")
	}
	return nil
}

type Application struct {
	Document uint64
	State    index.State
}

func ValidationApplications(ctx context.Context, tx *sql.Tx, physical string, after uint64) ([]Application, error) {
	rows, err := tx.QueryContext(ctx, `SELECT document_id,applied_seq,authority_digest,COALESCE(release_id,0),visibility,visibility_policy,revision FROM index_application WHERE physical_index=? AND document_id>? ORDER BY document_id LIMIT 32`, physical, after)
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_APPLICATION_READ_FAILED")
	}
	defer rows.Close()
	var result []Application
	for rows.Next() {
		var a Application
		var digest []byte
		var visibility string
		if rows.Scan(&a.Document, &a.State.Sequence, &digest, &a.State.Release, &visibility, &a.State.Policy, &a.State.Revision) != nil || len(digest) != 32 {
			return nil, errors.New("SEARCH_REBUILD_APPLICATION_INVALID")
		}
		copy(a.State.Digest[:], digest)
		a.State.Visible = visibility == "PUBLIC"
		if a.Document == 0 || a.State.Revision == 0 || (visibility != "PUBLIC" && visibility != "HIDDEN") {
			return nil, errors.New("SEARCH_REBUILD_APPLICATION_INVALID")
		}
		result = append(result, a)
	}
	if rows.Err() != nil {
		return nil, errors.New("SEARCH_REBUILD_APPLICATION_READ_FAILED")
	}
	return result, nil
}
