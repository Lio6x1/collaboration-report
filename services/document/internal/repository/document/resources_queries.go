// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

// resource_allocation_cleanup.go：保留原实现，按业务合并文件。

// 调用方已锁清理 Doc/KB；必须在任何资源锁之前调用。原上传墓碑不是消费引用，
// 但已转入且没有剩余引用的 allocation 仍须发现，以便未知提交后的原号恢复。
func LockDocumentResourceAllocationCleanup(ctx context.Context, tx *sql.Tx, document uint64) ([]domain.TransferResource, error) {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&space); err != nil {
		return nil, err
	}
	// Account/组织清理时空间已 CLOSING/PUBLIC_RETAINED；它由上层原清理范围
	// 授权，不能复用只接纳 ACTIVE 空间的普通正文写入口。
	if _, err := TrashSpaceRow(ctx, tx, space, true); err != nil {
		return nil, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&limit); err != nil {
		return nil, err
	}
	if _, err := resources.LockQuota(ctx, tx, space, limit); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT id FROM (
 SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')
 UNION SELECT j.id FROM resource_protection p JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(id BIGINT UNSIGNED PATH '$')) j WHERE p.document_id=? AND p.state<>'RELEASED'
 UNION SELECT resource_id id FROM document_resource_allocation WHERE document_id=? AND state='ACTIVE') x ORDER BY id`, document, document, document)
	if err != nil {
		return nil, err
	}
	var items []domain.TransferResource
	for rows.Next() {
		var r domain.TransferResource
		if err = rows.Scan(&r.ID); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		if _, _, err = LockedResourceSpace(ctx, tx, r.ID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// 仅真实 Node CLEANED 后的历史/保留 pin 清理调用；事务为 READ COMMITTED。
func ReconcileCleanedResourceAllocations(ctx context.Context, tx *sql.Tx, document, space uint64, items []domain.TransferResource) error {
	var limit uint64
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes FROM quota_account WHERE space_id=?`, space).Scan(&limit); err != nil {
		return err
	}
	return resources.ReleaseUnusedDocumentAllocations(ctx, tx, document, space, limit, items)
}

// resource_protection.go：保留原实现，按业务合并文件。

func ResourceProtectionBinding(ctx context.Context, q Queryer, operation string) ([]byte, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='RESOURCE_PROTECTION_BINDING' AND business_key=?", operation).Scan(&raw)
	return raw, err
}

func LockedProtectionAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error) {
	var current domain.EditingAdmission
	err := tx.QueryRowContext(ctx, "SELECT actor_id,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.Actor, &current.TargetJSON, &current.State, &current.Expires)
	return current, err
}
func LockedResourceProtection(ctx context.Context, tx *sql.Tx, operation string) (domain.ProtectionState, error) {
	var current domain.ProtectionState
	err := tx.QueryRowContext(ctx, "SELECT request_fingerprint,state FROM resource_protection WHERE operation_id=? FOR UPDATE", operation).Scan(&current.Fingerprint, &current.State)
	return current, err
}
func LockedResourceProtectionState(ctx context.Context, tx *sql.Tx, operation string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM resource_protection WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
	return state, err
}
func LockedResourceDocument(ctx context.Context, tx *sql.Tx, document uint64) (uint64, string, error) {
	var space uint64
	var lifecycle string
	err := tx.QueryRowContext(ctx, "SELECT space_id,lifecycle FROM document WHERE id=? FOR UPDATE", document).Scan(&space, &lifecycle)
	return space, lifecycle, err
}
func LockedResourceSpace(ctx context.Context, tx *sql.Tx, resource uint64) (uint64, string, error) {
	var space uint64
	var state string
	err := tx.QueryRowContext(ctx, "SELECT space_id,state FROM resource WHERE id=? FOR UPDATE", resource).Scan(&space, &state)
	return space, state, err
}
func LockedResourceState(ctx context.Context, tx *sql.Tx, resource uint64) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM resource WHERE id=? FOR UPDATE", resource).Scan(&state)
	return state, err
}
func InsertResourceProtection(ctx context.Context, tx *sql.Tx, record domain.ProtectionRecord) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,generation,request_fingerprint,resource_ids,source_epochs,state) VALUES(?,?,?,?,?,?,?,?,?,'PROTECTED')", record.Operation, record.Document, record.Room, record.Grant, record.Epoch, record.Generation, record.Fingerprint, record.ResourceJSON, record.SourceJSON)
	return err
}
func InsertResourceProtectionBinding(ctx context.Context, tx *sql.Tx, operation string, binding []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('RESOURCE_PROTECTION_BINDING',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))", operation, string(binding))
	return err
}
func InsertOperationResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, operation string, generation uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'OPERATION',?,'DRAFT_UPDATE',?)", resource, document, operation, generation)
	return err
}
func InsertDraftResourceRef(ctx context.Context, tx *sql.Tx, resource, document uint64, owner string, generation uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) VALUES(?,?,'DRAFT',?,'ROOM_DRAFT',?) ON DUPLICATE KEY UPDATE generation=GREATEST(generation,VALUES(generation))", resource, document, owner, generation)
	return err
}
func DeleteOperationResourceRefs(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE owner_kind='OPERATION' AND owner_ref=? AND purpose='DRAFT_UPDATE'", operation)
	return err
}
func FinalizeResourceProtection(ctx context.Context, tx *sql.Tx, operation, state, receipt string, seq uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE resource_protection SET state=?,node_receipt_ref=?,durable_seq=? WHERE operation_id=?", state, receipt, seq, operation)
	return err
}

func ResourceState(ctx context.Context, q Queryer, resource uint64) (string, error) {
	var state string
	err := q.QueryRowContext(ctx, "SELECT state FROM resource WHERE id=?", resource).Scan(&state)
	return state, err
}
