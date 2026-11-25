package snapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
)

type Job struct {
	Operation, State                           string
	Prepare, Seal                              []byte
	PrepareDigest, SignerDigest, EntriesDigest [32]byte
	Count, Revision                            uint64
}

var operationName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$`)

var ErrNotFound = errors.New("REBUILD_SNAPSHOT_NOT_FOUND")

// Reserve只接受逻辑层已经验签的Search准备制品。相同原号必须复用完全相同的字节，
// PSS重新签名得到另一引用也属于冲突，不能悄悄换L。事务的提交由业务用例负责。
func Reserve(ctx context.Context, tx *sql.Tx, operation string, prepare []byte, signer [32]byte) (Job, error) {
	if !operationName.MatchString(operation) || len(prepare) == 0 || len(prepare) > 2<<20 || signer == [32]byte{} {
		return Job{}, errors.New("REBUILD_PREPARE_INVALID")
	}
	digest := sha256.Sum256(prepare)
	_, err := tx.ExecContext(ctx, `INSERT INTO search_snapshot_job(operation_id,prepare_digest,prepare_envelope,state,signer_digest)
		VALUES(?,?,?,'WRITING',?) ON DUPLICATE KEY UPDATE operation_id=search_snapshot_job.operation_id`, operation, digest[:], prepare, signer[:])
	if err != nil {
		return Job{}, errors.New("REBUILD_SNAPSHOT_RESERVE_FAILED")
	}
	job, err := Lock(ctx, tx, operation)
	if err != nil {
		return Job{}, err
	}
	if job.PrepareDigest != digest || job.SignerDigest != signer || !bytes.Equal(job.Prepare, prepare) {
		return Job{}, errors.New("REBUILD_OPERATION_CONFLICT")
	}
	return job, nil
}

func Lock(ctx context.Context, tx *sql.Tx, operation string) (Job, error) {
	var job Job
	var prepareDigest, signerDigest, entriesDigest []byte
	// count使用十进制字符串扫描，避免把合法无符号计数经过int64中转。
	var rawCount sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT operation_id,state,prepare_digest,prepare_envelope,signer_digest,
		CAST(entry_count AS CHAR),entries_digest,seal_envelope,revision FROM search_snapshot_job WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&job.Operation, &job.State, &prepareDigest, &job.Prepare, &signerDigest, &rawCount, &entriesDigest, &job.Seal, &job.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return job, errors.New("REBUILD_SNAPSHOT_READ_FAILED")
	}
	if len(prepareDigest) != 32 || len(signerDigest) != 32 || job.Revision == 0 || len(job.Prepare) == 0 || len(job.Prepare) > 2<<20 {
		return Job{}, errors.New("REBUILD_SNAPSHOT_UNBOUND")
	}
	copy(job.PrepareDigest[:], prepareDigest)
	copy(job.SignerDigest[:], signerDigest)
	if sha256.Sum256(job.Prepare) != job.PrepareDigest {
		return Job{}, errors.New("REBUILD_SNAPSHOT_UNBOUND")
	}
	if job.State == "SEALED" {
		if !rawCount.Valid || len(entriesDigest) != 32 || len(job.Seal) == 0 || len(job.Seal) > 2<<20 {
			return Job{}, errors.New("REBUILD_SNAPSHOT_UNBOUND")
		}
		job.Count, err = strconv.ParseUint(rawCount.String, 10, 64)
		if err != nil {
			return Job{}, errors.New("REBUILD_SNAPSHOT_UNBOUND")
		}
		copy(job.EntriesDigest[:], entriesDigest)
	} else if job.State != "WRITING" || rawCount.Valid || len(entriesDigest) != 0 || len(job.Seal) != 0 {
		return Job{}, errors.New("REBUILD_SNAPSHOT_UNBOUND")
	}
	return job, nil
}

func Seal(ctx context.Context, tx *sql.Tx, operation string, count uint64, digest [32]byte, envelope []byte) error {
	if digest == [32]byte{} || len(envelope) == 0 || len(envelope) > 2<<20 {
		return errors.New("REBUILD_SEAL_INVALID")
	}
	var actual uint64
	if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM search_snapshot_entry WHERE operation_id=?`, operation).Scan(&actual) != nil || actual != count {
		return errors.New("REBUILD_SNAPSHOT_INCOMPLETE")
	}
	result, err := tx.ExecContext(ctx, `UPDATE search_snapshot_job SET state='SEALED',entry_count=?,entries_digest=?,seal_envelope=?,revision=revision+1
		WHERE operation_id=? AND state='WRITING' AND revision<18446744073709551615`, count, digest[:], envelope, operation)
	if err != nil {
		return errors.New("REBUILD_SEAL_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("REBUILD_SEAL_CHANGED")
	}
	return nil
}
