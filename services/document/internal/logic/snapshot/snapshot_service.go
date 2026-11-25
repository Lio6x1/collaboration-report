// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package snapshot

import (
	pb "collaboration/contracts/internalv1"
	repo "collaboration/document/internal/repository/snapshot"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
)

// export.go：保留原实现，按业务合并文件。

// Export 只把原封存条目写入调用者提供的流。调用者须写受控临时文件，等本方法
// 成功及文件 Flush/Sync 完成后再发布；流的部分写入不能视为一个可导入制品。
// 返回原 seal 字节供最后发布，文件丢失后仍可从相同 operation 精确重生。
func (s *Service) Export(ctx context.Context, operation string, output io.Writer) ([]byte, error) {
	if s == nil || s.DB == nil || output == nil || !rebuildmeta.OperationValid(operation) {
		return nil, ErrInput
	}
	// Lock 使用 FOR UPDATE，因此不用数据库只读事务；此事务没有数据写入。
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()
	job, err := repo.Lock(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if job.State != "SEALED" || job.Operation != operation {
		return nil, ErrIncomplete
	}
	accepted, err := rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
	if err != nil {
		return nil, err
	}
	if err := checkOperation(accepted, operation); err != nil {
		return nil, err
	}

	hash := sha256.New()
	var after, count uint64
	for {
		entries, err := repo.StoredPage(ctx, tx, operation, after, 500)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		for _, entry := range entries {
			if entry.Document <= after || count == ^uint64(0) {
				return nil, ErrIncomplete
			}
			line, err := entry.Line()
			if err != nil {
				return nil, err
			}
			n, err := output.Write(line)
			if err != nil || n != len(line) {
				// 不传播底层文件路径，也不把短写或磁盘错误吞掉后返回原 seal。
				return nil, errors.New("REBUILD_SNAPSHOT_OUTPUT_FAILED")
			}
			hash.Write(line)
			after, count = entry.Document, count+1
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	if count != job.Count || digest != job.EntriesDigest {
		return nil, ErrIncomplete
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_EXPORT_FINISH_FAILED")
	}
	return append([]byte(nil), job.Seal...), nil
}

// request.go：保留原实现，按业务合并文件。

// RebuildRequest只从Doc已提交的封存记录还原原请求。CLI不让操作者重填代次或L，
// Start回执未知时，后续Start/Get仍使用完全相同的输入与指纹。
func RebuildRequest(ctx context.Context, db *sql.DB, operation, snapshotRef string) (*pb.RebuildRequest, [32]byte, error) {
	var empty [32]byte
	if db == nil || !rebuildmeta.OperationValid(operation) {
		return nil, empty, ErrInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, empty, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()
	job, err := repo.Lock(ctx, tx, operation)
	if err != nil {
		return nil, empty, err
	}
	if job.State != "SEALED" {
		return nil, empty, ErrIncomplete
	}
	ref, err := rebuildartifact.Reference(job.Seal, rebuildartifact.SnapshotV1)
	if err != nil || ref != snapshotRef {
		return nil, empty, ErrConflict
	}
	accepted, err := rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
	if err != nil {
		return nil, empty, err
	}
	p, err := rebuildmeta.ParsePrepare(accepted.Payload)
	if err != nil || p.Operation != operation {
		return nil, empty, ErrConflict
	}
	d, err := rebuildmeta.DescriptorFor(p, accepted.Reference, ref)
	if err != nil {
		return nil, empty, err
	}
	watermark, err := d.Bytes()
	if err != nil {
		return nil, empty, err
	}
	generation, err := rebuildmeta.Decimal(p.TargetGeneration, true)
	if err != nil {
		return nil, empty, err
	}
	fingerprint, err := rebuildmeta.RequestFingerprint(operation, generation, ref, watermark)
	if err != nil {
		return nil, empty, err
	}
	// 释放本地读取锁后才允许调用Search，避免跨服务RPC占有Doc事务。
	if err := tx.Rollback(); err != nil {
		return nil, empty, errors.New("REBUILD_REQUEST_RELEASE_FAILED")
	}
	return &pb.RebuildRequest{OperationId: operation, Generation: generation, SnapshotRef: ref, WatermarkJsonUtf8: watermark}, fingerprint, nil
}
