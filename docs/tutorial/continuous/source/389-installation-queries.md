# services/search/internal/repository/rebuild/installation_queries.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/389-installation-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Imported`、`FindImport`、`SaveImport`、`PendingSnapshot`、`AdvanceSnapshot`、`FinishSnapshot`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
)

// import.go：保留原实现，按业务合并文件。

type Imported struct {
	Seal                 rebuildmeta.Seal
	Envelope             []byte
	Digest, SignerDigest [32]byte
}

// FindImport只在代次、原binding依次锁定后调用；已接受签名按持久摘要恢复，证书到期不改变原事实。
func FindImport(ctx context.Context, tx *sql.Tx, operation string) (Imported, bool, error) {
	var row Imported
	var digest, signer, entries []byte
	var count sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT snapshot_envelope,snapshot_digest,signer_digest,CAST(snapshot_count AS CHAR),entries_digest FROM rebuild_binding WHERE operation_id=? FOR UPDATE`, operation).Scan(&row.Envelope, &digest, &signer, &count, &entries)
	if err != nil {
		return row, false, errors.New("SEARCH_REBUILD_IMPORT_READ_FAILED")
	}
	if len(row.Envelope) == 0 && digest == nil && signer == nil && !count.Valid && entries == nil {
		return row, false, nil
	}
	if len(digest) != 32 || len(signer) != 32 || len(entries) != 32 || !count.Valid {
		return row, false, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	copy(row.Digest[:], digest)
	copy(row.SignerDigest[:], signer)
	verified, err := rebuildartifact.RestoreAccepted(row.Envelope, rebuildartifact.SnapshotV1, row.Digest, row.SignerDigest)
	if err != nil {
		return row, false, err
	}
	row.Seal, err = rebuildmeta.ParseSeal(verified.Payload)
	if err != nil || row.Seal.Count != count.String || row.Seal.EntriesDigest != hex.EncodeToString(entries) {
		return Imported{}, false, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	return row, true, nil
}

// SaveImport不提交事务。完整副本必须已经发布；未知提交只能恢复同一原号。
func SaveImport(ctx context.Context, tx *sql.Tx, operation string, row Imported) error {
	if _, err := row.Seal.Bytes(); err != nil {
		return err
	}
	entries, _ := hex.DecodeString(row.Seal.EntriesDigest)
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_binding SET snapshot_digest=?,snapshot_envelope=?,snapshot_count=?,entries_digest=?,signer_digest=?,state='IMPORTED',revision=revision+1 WHERE operation_id=? AND state='PREPARED' AND revision<18446744073709551615`, row.Digest[:], row.Envelope, row.Seal.Count, entries, row.SignerDigest[:], operation)
	if err != nil {
		return errors.New("SEARCH_REBUILD_IMPORT_WRITE_FAILED")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("SEARCH_REBUILD_IMPORT_CONFLICT")
	}
	return nil
}

// seed.go：保留原实现，按业务合并文件。

func PendingSnapshot(ctx context.Context, db *sql.DB) (string, error) {
	var operation string
	err := db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state='ACCEPTED' AND j.state='SNAPSHOT' ORDER BY j.id LIMIT 1`).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return operation, nil
}

// 与该文档index_application同事务推进；ES或提交未知时必须保留原after重做。
func AdvanceSnapshot(ctx context.Context, tx *sql.Tx, id, before, after uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_progress SET snapshot_after=?,revision=revision+1 WHERE job_id=? AND snapshot_after=? AND revision<18446744073709551615`, after, id, before)
	if err != nil {
		return errors.New("SEARCH_REBUILD_PROGRESS_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}

func FinishSnapshot(ctx context.Context, tx *sql.Tx, id uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE rebuild_job SET state='REPLAYING',last_error=NULL WHERE id=? AND state='SNAPSHOT'`, id)
	if err != nil {
		return errors.New("SEARCH_REBUILD_PROGRESS_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`db322828cb5cbf6997adf1257040ec10578fab8beb7a3102cb1bc96142f24ab3`。[参考实现](../../../../services/search/internal/repository/rebuild/installation_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
