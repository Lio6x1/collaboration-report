// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// export.go：保留原实现，按业务合并文件。

func ExportError(code string) *dto.Error {
	return &dto.Error{Code: code, Message: "导出未完成，请按错误码核对格式、权限或临时额度后重新创建", TraceId: platform.NewTraceID(), RetryAction: "NONE"}
}

const exportColumns = `j.id,COALESCE(j.requester_id,0),j.document_id,j.manifest_id,j.release_id,j.target_kind,j.format,j.state,j.operation_id,j.generation,COALESCE(j.last_error,''),c.fingerprint,c.snapshot,COALESCE(c.artifact_key,''),c.artifact_size,c.artifact_checksum,c.expires_at`

func ExportRow(ctx context.Context, q Queryer, id uint64, lock bool) (domain.ExportRecord, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	return scanExport(q.QueryRowContext(ctx, "SELECT "+exportColumns+" FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=?"+suffix, id))
}

func ExportOperation(ctx context.Context, q Queryer, operation string) (domain.ExportRecord, error) {
	return scanExport(q.QueryRowContext(ctx, "SELECT "+exportColumns+" FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.operation_id=?", operation))
}

func scanExport(row *sql.Row) (domain.ExportRecord, error) {
	var value domain.ExportRecord
	var fingerprint, snapshot, checksum []byte
	var target, lastError string
	err := row.Scan(&value.Job.Id, &value.Requester, &value.Job.DocumentId, &value.Job.ManifestId, &value.Job.ReleaseId, &target, &value.Job.Format, &value.Job.State, &value.Operation, &value.Generation, &lastError, &fingerprint, &snapshot, &value.ArtifactKey, &value.Size, &checksum, &value.Expires)
	if err != nil {
		return value, err
	}
	if len(fingerprint) != 32 || (len(checksum) != 0 && len(checksum) != 32) {
		return value, errors.New("EXPORT_RECORD_INVALID")
	}
	copy(value.Fingerprint[:], fingerprint)
	copy(value.Checksum[:], checksum)
	value.Snapshot, err = domain.DecodeExportSnapshot(snapshot)
	if lastError != "" {
		value.Job.LastError = ExportError(lastError)
	}
	if err == nil && (target != value.Snapshot.Target || value.Job.Format != value.Snapshot.Format || value.Job.DocumentId != domain.TextID(value.Snapshot.Document) || value.Job.ManifestId != domain.TextID(value.Snapshot.Manifest)) {
		err = errors.New("EXPORT_FIXED_BINDING_CHANGED")
	}
	return value, err
}

func InsertExport(ctx context.Context, tx *sql.Tx, requester uint64, operation string, fingerprint [32]byte, fixed domain.ExportSnapshot) (uint64, error) {
	var actor, release any
	if requester != 0 {
		actor = requester
	}
	if fixed.Release != 0 {
		release = fixed.Release
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO export_job(requester_id,document_id,manifest_id,release_id,target_kind,format,state,operation_id) VALUES(?,?,?,?,?,?,'QUEUED',?)`, actor, fixed.Document, fixed.Manifest, release, fixed.Target, fixed.Format, operation)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO export_control(job_id,fingerprint,snapshot,expires_at) VALUES(?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 HOUR))`, id, fingerprint[:], raw)
	return uint64(id), err
}

func InsertExportReceipt(ctx context.Context, tx *sql.Tx, requester uint64, identityRef, operation, path string, fingerprint [32]byte, job uint64) error {
	var identity any
	if requester != 0 {
		identity = identityRef
	}
	target, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return err
	}
	// SUCCEEDED表示登记已经提交；作业本身另有QUEUED/RUNNING状态，转换未知不伪装成功。
	result, err := json.Marshal(map[string]any{"operation_id": operation, "state": "SUCCEEDED", "result": map[string]string{"export_job_id": domain.TextID(job)}})
	if err != nil {
		return err
	}
	// 冻结receipt.actor_id非空，匿名主体使用不可登录的保留编号0；job.requester_id仍为NULL。
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUES(?,?,'EXPORT_CREATE',?,?,'SUCCEEDED',?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),UTC_TIMESTAMP(6))`, operation, requester, target, fingerprint[:], identity, result)
	if err != nil {
		return err
	}
	var auditActor any
	if requester != 0 {
		auditActor = requester
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,'EXPORT_CREATE',?,?)`, auditActor, operation, target, result)
	return err
}

// export_drain.go：保留原实现，按业务合并文件。

// 清理调用方必须先完成文档来源关闭；这里再次核生命周期与真实未终结作业。
// 只能让终态制品提前到期，不能把仍在运行的转换器改成CANCELLED冒充已退出。
func ExpireClosedDocumentExports(ctx context.Context, tx *sql.Tx, document uint64) error {
	var lifecycle string
	if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM document WHERE id=? FOR UPDATE", document).Scan(&lifecycle); err != nil {
		return err
	}
	if lifecycle != "CLOSING" && lifecycle != "PURGED" {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_NOT_CLOSED")
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN ('QUEUED','RUNNING'))", document).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_DRAIN_PENDING")
	}
	_, err := tx.ExecContext(ctx, `UPDATE export_control c JOIN export_job j ON j.id=c.job_id
	 SET c.expires_at=LEAST(c.expires_at,UTC_TIMESTAMP(6)) WHERE j.document_id=?`, document)
	return err
}

func DocumentExportControlsPending(ctx context.Context, q Queryer, document uint64) (bool, error) {
	var pending bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_control c
	 JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?)`, document).Scan(&pending)
	return pending, err
}

// export_expiry.go：保留原实现，按业务合并文件。

type ExpiredExport struct {
	ID, Size, Reserved uint64
	Key                string
}

func ExpiredExports(ctx context.Context, q Queryer) ([]ExpiredExport, error) {
	// 清理历史已删除job也精确回收其独立制品；不会留下注销后的临时副本等一小时。
	rows, err := q.QueryContext(ctx, `SELECT c.job_id,COALESCE(c.artifact_key,''),c.artifact_size,c.reserved_bytes
	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id
	 WHERE (j.id IS NULL OR c.expires_at<=UTC_TIMESTAMP(6)) AND (j.id IS NULL OR j.state NOT IN ('QUEUED','RUNNING')) ORDER BY c.job_id LIMIT 16`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ExpiredExport
	for rows.Next() {
		var item ExpiredExport
		if err = rows.Scan(&item.ID, &item.Key, &item.Size, &item.Reserved); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func RemoveExpiredExport(ctx context.Context, tx *sql.Tx, expected ExpiredExport) error {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return err
	}
	var key string
	var size, own uint64
	var expired bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(c.artifact_key,''),c.artifact_size,c.reserved_bytes,(j.id IS NULL OR c.expires_at<=UTC_TIMESTAMP(6))
	 FROM export_control c LEFT JOIN export_job j ON j.id=c.job_id WHERE c.job_id=? FOR UPDATE`, expected.ID).Scan(&key, &size, &own, &expired)
	if err != nil {
		return err
	}
	if !expired || key != expected.Key || size != expected.Size || own != expected.Reserved || size > used || own > reserved {
		return errors.New("EXPORT_EXPIRY_BINDING_CHANGED")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET used_bytes=used_bytes-?,reserved_bytes=reserved_bytes-? WHERE id=1", size, own); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_job SET state='CANCELLED',last_error='EXPORT_EXPIRED' WHERE id=?", expected.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM export_control WHERE job_id=?", expected.ID)
	return err
}

// export_fixed.go：保留原实现，按业务合并文件。

func ExportMainView(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error) {
	return resources.MainManifestAccessible(ctx, tx, document, manifest, actor, session)
}

func ExportManifestFacts(ctx context.Context, q Queryer, document, manifest uint64) (domain.ExportSnapshot, error) {
	value := domain.ExportSnapshot{Document: document, Manifest: manifest}
	err := q.QueryRowContext(ctx, `SELECT structure_id,style_id FROM main_manifest WHERE id=? AND document_id=?`, manifest, document).Scan(&value.Structure, &value.Style)
	return value, err
}

// 资源pin与排队作业同事务；引用来自固定版本，不能把活跃草稿或任意URL放进导出。
func PinExportResources(ctx context.Context, tx *sql.Tx, id uint64, fixed domain.ExportSnapshot) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose)
	 SELECT DISTINCT r.resource_id,e.document_id,'EXPORT',?,'CONTENT' FROM manifest_entry e
	 JOIN resource_reference r ON r.document_id=e.document_id AND r.owner_kind='VERSION' AND r.owner_ref=CAST(e.content_version_id AS CHAR) AND r.purpose='CONTENT'
	 WHERE e.document_id=? AND e.manifest_id=?`, domain.TextID(id), fixed.Document, fixed.Manifest)
	return err
}

func ReleaseExportResources(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE owner_kind='EXPORT' AND owner_ref=?", domain.TextID(id))
	return err
}

// export_worker.go：保留原实现，按业务合并文件。

const ExportTemporaryLimit uint64 = 256 << 20
const ExportWorkReservation uint64 = 192 << 20

func lockExportBudget(ctx context.Context, tx *sql.Tx) (uint64, uint64, error) {
	if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO export_budget(id) VALUES(1)"); err != nil {
		return 0, 0, err
	}
	var reserved, used uint64
	err := tx.QueryRowContext(ctx, "SELECT reserved_bytes,used_bytes FROM export_budget WHERE id=1 FOR UPDATE").Scan(&reserved, &used)
	return reserved, used, err
}

func CheckExportQueue(ctx context.Context, tx *sql.Tx) error {
	if _, _, err := lockExportBudget(ctx, tx); err != nil {
		return err
	}
	var jobs uint64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM export_job WHERE state IN ('QUEUED','RUNNING')").Scan(&jobs); err != nil {
		return err
	}
	if jobs >= 32 {
		return domain.Violation(domain.TemporarilyUnavailable, "EXPORT_QUEUE_FULL")
	}
	return nil
}

func NextExports(ctx context.Context, q Queryer) ([]uint64, error) {
	// 与持久队列上限一致；候选不是领取证明，业务层还须取得对应job文件锁。
	rows, err := q.QueryContext(ctx, `SELECT id FROM export_job WHERE state IN ('QUEUED','RUNNING') AND (retry_at IS NULL OR retry_at<=UTC_TIMESTAMP(6)) ORDER BY id LIMIT 32`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// 跨进程文件guard在调用前已取得；RUNNING恢复仍沿原job，不靠TTL判断转换成功。
func ClaimExport(ctx context.Context, tx *sql.Tx, id, temporaryLimit uint64) (domain.ExportRecord, error) {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return domain.ExportRecord{}, err
	}
	value, err := ExportRow(ctx, tx, id, true)
	if err != nil {
		return value, err
	}
	if value.Job.State != "QUEUED" && value.Job.State != "RUNNING" {
		return value, domain.Violation(domain.StateConflict, "EXPORT_STATE_CHANGED")
	}
	var own uint64
	if err = tx.QueryRowContext(ctx, "SELECT reserved_bytes FROM export_control WHERE job_id=?", id).Scan(&own); err != nil {
		return value, err
	}
	if own == 0 {
		if used > temporaryLimit || reserved > temporaryLimit-used || ExportWorkReservation > temporaryLimit-used-reserved {
			return value, domain.Violation(domain.TemporarilyUnavailable, "EXPORT_TEMPORARY_QUOTA_EXCEEDED")
		}
		if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET reserved_bytes=reserved_bytes+? WHERE id=1", ExportWorkReservation); err != nil {
			return value, err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE export_control SET reserved_bytes=? WHERE job_id=?", ExportWorkReservation, id); err != nil {
			return value, err
		}
	} else if own != ExportWorkReservation {
		return value, errors.New("EXPORT_RESERVATION_CHANGED")
	}
	if value.Generation == ^uint64(0) {
		return value, errors.New("EXPORT_GENERATION_EXHAUSTED")
	}
	_, err = tx.ExecContext(ctx, "UPDATE export_job SET state='RUNNING',generation=generation+1,retry_at=NULL,last_error=NULL WHERE id=?", id)
	value.Generation++
	value.Job.State = "RUNNING"
	return value, err
}

func FinishExport(ctx context.Context, tx *sql.Tx, id, generation uint64, state, code, key string, size uint64, checksum [32]byte) error {
	reserved, used, err := lockExportBudget(ctx, tx)
	if err != nil {
		return err
	}
	var current, own, oldSize uint64
	var oldState string
	if err = tx.QueryRowContext(ctx, "SELECT j.generation,j.state,c.reserved_bytes,c.artifact_size FROM export_job j JOIN export_control c ON c.job_id=j.id WHERE j.id=? FOR UPDATE", id).Scan(&current, &oldState, &own, &oldSize); err != nil {
		return err
	}
	if current != generation || oldSize != 0 {
		return errors.New("EXPORT_FINISH_BINDING_CHANGED")
	}
	if oldState != "QUEUED" && oldState != "RUNNING" {
		return errors.New("EXPORT_ALREADY_TERMINAL")
	}
	// 完成用已持久预留换成更小的制品，不能再次申请空间。
	// 配置调低后仍允许原RUNNING恢复/取消并归还预留，避免旧预算永远卡死队列。
	if own > reserved || size > 32<<20 || size > own || size > ^uint64(0)-used {
		return errors.New("EXPORT_BUDGET_CHANGED")
	}
	var artifact, digest, lastError any
	if key != "" {
		artifact = key
		digest = checksum[:]
	}
	if code != "" {
		lastError = code
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_budget SET reserved_bytes=reserved_bytes-?,used_bytes=used_bytes+? WHERE id=1", own, size); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE export_control SET reserved_bytes=0,artifact_key=?,artifact_size=?,artifact_checksum=? WHERE job_id=?", artifact, size, digest, id); err != nil {
		return err
	}
	if err = ReleaseExportResources(ctx, tx, id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE export_job SET state=?,last_error=?,retry_at=NULL WHERE id=? AND generation=?", state, lastError, id, generation)
	return err
}

func DeferExport(ctx context.Context, db *sql.DB, id uint64) error {
	_, err := db.ExecContext(ctx, "UPDATE export_job SET retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id=? AND state IN ('QUEUED','RUNNING')", id)
	return err
}
