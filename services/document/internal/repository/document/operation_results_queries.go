// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
)

// operation.go：保留原实现，按业务合并文件。

type Operations struct{ db *sql.DB }

func NewOperations(db *sql.DB) *Operations { return &Operations{db: db} }
func (r *Operations) Exists(ctx context.Context, id string) (bool, error) {
	return OperationExists(ctx, r.db, id)
}
func OperationExists(ctx context.Context, q Queryer, id string) (bool, error) {
	var marker int
	err := q.QueryRowContext(ctx, "SELECT 1 FROM operation_receipt WHERE operation_id=?", id).Scan(&marker)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
func LockOperationGate(ctx context.Context, tx *sql.Tx, id string) error {
	// 同号空receipt也需串行：唯一gate行提供互斥，不能依赖不存在行的间隙锁。
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", "operation:"+id)
	if err != nil {
		return err
	}
	var revision uint64
	if tx.QueryRowContext(ctx, "SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE", "operation:"+id).Scan(&revision) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "OPERATION_UNAVAILABLE")
	}
	return nil
}
func LockedOperation(ctx context.Context, tx *sql.Tx, id string) (domain.OperationReceipt, error) {
	var receipt domain.OperationReceipt
	err := tx.QueryRowContext(ctx, "SELECT actor_id,kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE", id).Scan(&receipt.ActorID, &receipt.Kind, &receipt.Fingerprint, &receipt.Result)
	return receipt, err
}
func InsertOperation(ctx context.Context, tx *sql.Tx, record domain.OperationRecord) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,identity_admission_ref,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))", record.ID, record.ActorID, record.Kind, string(record.TargetJSON), record.Fingerprint, record.State, record.IdentityAdmissionRef, string(record.ResultJSON), record.State)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(?,?,1,?,?,?)", record.ActorID, record.ID, record.Kind, string(record.TargetJSON), string(record.ResultJSON))
	return err
}
func InsertReadAdmission(ctx context.Context, tx *sql.Tx, record domain.ReadAdmission) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO document_admission(admission_id,identity_admission_ref,actor_id,session_ref,document_id,purpose,capabilities,target_json,operation_id,state,expires_at) VALUES(?,?,?,?,?,?,?,?,?,'OPEN',FROM_UNIXTIME(?/1000))", record.ID, record.IdentityAdmissionRef, record.ActorID, record.SessionRef, record.DocumentID, record.Purpose, string(record.CapabilitiesJSON), string(record.TargetJSON), record.OperationID, record.ExpiresUnixMs)
	return err
}

func UpdateOperationResult(ctx context.Context, tx *sql.Tx, id, state string, result []byte) error {
	_, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state=?,result_json=?,terminal_at=IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL) WHERE operation_id=?", state, string(result), state, id)
	return err
}
func InsertAdmissionSource(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)", id, key, epoch)
	return err
}

// outbox.go：保留原实现，按业务合并文件。

// 只写调用方的原业务事务。发送由Canal读取已提交binlog完成，不能在提交前发送Kafka。
func InsertIndexEvent(ctx context.Context, tx *sql.Tx, document, sequence uint64, eventID, eventType string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO outbox(event_id,aggregate_kind,aggregate_id,aggregate_seq,event_type,schema_version,payload) VALUES(?,'DOCUMENT',?,?,?,'1',?)", eventID, document, sequence, eventType, string(payload))
	return err
}

// rows.go：保留原实现，按业务合并文件。

// Queryer让快照与事务复核使用原连接；repository不创建或提交事务。
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func DocumentRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Document, error) {
	var d dto.Document
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),title,CAST(space_id AS CHAR),CAST(knowledge_base_id AS CHAR),CAST(responsible_user_id AS CHAR),lifecycle,main_locked,CAST(current_structure_id AS CHAR),CAST(current_style_id AS CHAR),CAST(current_manifest_id AS CHAR),CAST(current_release_id AS CHAR),publication_state,CAST(control_revision AS CHAR),CAST(acl_revision AS CHAR),CAST(publication_epoch AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM document WHERE id=?"+suffix, id).Scan(&d.Id, &d.Title, &d.SpaceId, &d.KnowledgeBaseId, &d.ResponsibleUserId, &d.Lifecycle, &d.MainLocked, &d.CurrentStructureId, &d.CurrentStyleId, &d.CurrentManifestId, &d.CurrentReleaseId, &d.PublicationState, &d.ControlRevision, &d.AclRevision, &d.PublicationEpoch, &d.VisibilityPolicy, &d.VisibilityRevision)
	if err != nil || d.Lifecycle != "ACTIVE" {
		return d, domain.Violation(domain.EntityNotFound, "DOCUMENT_NOT_FOUND")
	}
	return d, nil
}
func SpaceRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.Space, error) {
	var value dto.Space
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),kind,CAST(account_owner_id AS CHAR),CAST(organization_id AS CHAR),lifecycle FROM space WHERE id=?"+suffix, id).Scan(&value.Id, &value.Kind, &value.OwnerUserId, &value.OrganizationId, &value.Lifecycle)
	value.Actions = []string{}
	if err != nil || value.Lifecycle != "ACTIVE" {
		return value, domain.Violation(domain.EntityNotFound, "SPACE_NOT_FOUND")
	}
	return value, nil
}
