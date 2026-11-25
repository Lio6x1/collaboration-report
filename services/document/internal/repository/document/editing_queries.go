// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
	"errors"
	"time"
)

// content_capacity.go：保留原实现，按业务合并文件。

// 调用方已锁 Doc/KB。先锁空间与额度，再锁资源；不可在资源之后首次取得额度锁。
// runtime 配置只在创建/首次空间物化传入，历史写入使用空间实际持久上限。
func LockDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT space_id FROM document WHERE id=?", document).Scan(&space); err != nil {
		return err
	}
	if _, err := SpaceRow(ctx, tx, space, true); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", space).Scan(&limit); err != nil {
		return errors.New("DOCUMENT_QUOTA_NOT_INITIALIZED")
	}
	_, err := resources.LockQuota(ctx, tx, space, limit)
	return err
}

// 与真实 INSERT 使用同一事务；结果未知的原恢复器先核对原版本，不生成第二笔费用。
func ChargeStoredVersion(ctx context.Context, tx *sql.Tx, document, bytes uint64, operation string) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?", document).Scan(&space, &limit); err != nil {
		return err
	}
	return resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)
}

// Node 的真实原 CLEANED 已由上层确认。保留公开版本继续计费，删除的历史和
// 已排空的 Node 存储才退款；整份删除时同事务移除容量行。
func ReconcileCleanedDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?", document).Scan(&space, &limit); err != nil {
		return err
	}
	q, err := resources.LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var paid uint64
	err = tx.QueryRowContext(ctx, "SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE", document, space).Scan(&paid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	var remaining uint64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL", document).Scan(&remaining); err != nil {
		return err
	}
	if remaining > paid || paid-remaining > q.Used {
		return errors.New("DOCUMENT_CAPACITY_INCONSISTENT")
	}
	if paid == 0 && remaining == 0 {
		return nil
	}
	changed, err := tx.ExecContext(ctx, "UPDATE quota_account SET used_bytes=used_bytes-?,revision=revision+1 WHERE space_id=? AND revision=? AND revision<18446744073709551615", paid-remaining, space, q.Revision)
	if err != nil {
		return err
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return errors.New("DOCUMENT_CAPACITY_REVISION_CHANGED")
	}
	if remaining == 0 {
		_, err = tx.ExecContext(ctx, "DELETE FROM document_capacity WHERE document_id=?", document)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE document_capacity SET bytes=?,node_bytes=0 WHERE document_id=?", remaining, document)
	}
	return err
}

// content_versions.go：保留原实现，按业务合并文件。

func ContentVersionScope(ctx context.Context, q Queryer, id uint64) (domain.ContentVersionScope, error) {
	var scope domain.ContentVersionScope
	err := q.QueryRowContext(ctx, "SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?", id).Scan(&scope.Version, &scope.Document, &scope.Unit, &scope.Structure)
	return scope, err
}

// 只从实际业务引用证明这是共享审核/主线上下文，source_kind或可猜到的ID不授予草稿访问。
func SharedContentVersion(ctx context.Context, q Queryer, scope domain.ContentVersionScope) (bool, error) {
	var shared bool
	err := q.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM review_round WHERE document_id=? AND unit_id=? AND content_version_id=?)
		OR EXISTS(SELECT 1 FROM manifest_entry WHERE document_id=? AND unit_id=? AND content_version_id=?)`,
		scope.Document, scope.Unit, scope.Version, scope.Document, scope.Unit, scope.Version).Scan(&shared)
	return shared, err
}

// edit_bootstrap.go：保留原实现，按业务合并文件。

func LockedEditingStructure(ctx context.Context, tx *sql.Tx, document uint64) (string, uint64, error) {
	var lifecycle string
	var structure uint64
	err := tx.QueryRowContext(ctx, "SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE", document).Scan(&lifecycle, &structure)
	return lifecycle, structure, err
}
func BranchBootstrapBody(ctx context.Context, tx *sql.Tx, workspace, document, unit, epoch uint64) ([]byte, uint64, error) {
	var body []byte
	var version uint64
	err := tx.QueryRowContext(ctx, "SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_version_id AND v.document_id=b.document_id JOIN collaboration_unit u ON u.active_branch_id=b.id AND u.id=b.unit_id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.workspace_epoch=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL", workspace, document, unit, epoch).Scan(&body, &version)
	return body, version, err
}
func MainBootstrapBody(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]byte, uint64, error) {
	var body []byte
	var version uint64
	err := tx.QueryRowContext(ctx, "SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e.content_version_id WHERE d.id=? AND d.main_locked=0", unit, document).Scan(&body, &version)
	return body, version, err
}

// 来源上下文随当前编辑基线交付，Node不能自行编造base或已接纳来源。
func BranchBootstrapLineage(ctx context.Context, tx *sql.Tx, branch uint64) (uint64, uint64, []byte, error) {
	var lineage, manifest uint64
	var provenance []byte
	err := tx.QueryRowContext(ctx, "SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.lifecycle='ACTIVE' FOR UPDATE", branch).Scan(&lineage, &manifest, &provenance)
	return lineage, manifest, provenance, err
}

// edit_session.go：保留原实现，按业务合并文件。

func EnsureCollaborationRoom(ctx context.Context, tx *sql.Tx, key string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('COLLABORATION_ROOM',?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)", key, string(payload))
	return err
}
func LockedCollaborationRoom(ctx context.Context, tx *sql.Tx, key string) ([]byte, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='COLLABORATION_ROOM' AND business_key=? FOR UPDATE", key).Scan(&raw)
	return raw, err
}
func LockedActiveBranch(ctx context.Context, tx *sql.Tx, unit, document uint64) (uint64, error) {
	var branch uint64
	err := tx.QueryRowContext(ctx, "SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE", unit, document).Scan(&branch)
	return branch, err
}
func LockedEditWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error) {

	current, err := LockedControlledWorkspace(ctx, tx, branch, document, unit)
	if err != nil {
		return current, err
	}
	return current, RequireBranchControlOpen(ctx, tx, branch)
}

// 已登记控制用例读取自己的旧工作区；普通编辑/送审/合并必须再检查控制占位。
func LockedControlledWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error) {
	var current domain.EditWorkspace
	err := tx.QueryRowContext(ctx, "SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.lifecycle='ACTIVE' FOR UPDATE", branch, document, unit).Scan(&current.Workspace, &current.Epoch, &current.Lineage, &current.BaseManifest, &current.BaseVersion)
	return current, err
}
func ConfigureEditAdmission(ctx context.Context, tx *sql.Tx, unit uint64, admission string, target []byte, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=?,target_json=?,capabilities='[\"EDIT\",\"READ\"]',expires_at=? WHERE admission_id=?", unit, string(target), expires, admission)
	return err
}
func InsertEditTicket(ctx context.Context, tx *sql.Tx, hash []byte, admission, client, kind string, workspace, epoch uint64, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,'OPEN',?)", hash, admission, client, kind, workspace, epoch, expires)
	return err
}
func InsertEditSessionResult(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('EDIT_SESSION',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))", operation, string(payload))
	return err
}
func EditSessionResult(ctx context.Context, tx *sql.Tx, operation string) ([]byte, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='EDIT_SESSION' AND business_key=?", operation).Scan(&raw)
	return raw, err
}
func TicketAdmission(ctx context.Context, q Queryer, hash []byte) (string, error) {
	var admission string
	err := q.QueryRowContext(ctx, "SELECT admission_id FROM collaboration_ticket WHERE token_hash=?", hash).Scan(&admission)
	return admission, err
}
func LockedEditingTicket(ctx context.Context, tx *sql.Tx, hash []byte) (domain.EditingTicket, error) {
	var ticket domain.EditingTicket
	a := &ticket.Admission
	err := tx.QueryRowContext(ctx, "SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collaboration_ticket t ON t.admission_id=a.admission_id WHERE t.token_hash=? FOR UPDATE", hash).Scan(&a.Actor, &a.SessionRef, &a.TargetJSON, &a.State, &ticket.Purpose, &a.Expires, &ticket.Client, &ticket.State, &ticket.Expires)
	return ticket, err
}
func BindEditingGrant(ctx context.Context, tx *sql.Tx, admission, grant string, hash, target []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document_admission SET target_json=? WHERE admission_id=?", string(target), admission); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE collaboration_ticket SET state='REDEEMED',redeemed_grant_ref=? WHERE token_hash=?", grant, hash)
	return err
}
func LockedEditingAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error) {
	var current domain.EditingAdmission
	err := tx.QueryRowContext(ctx, "SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.Actor, &current.SessionRef, &current.TargetJSON, &current.State, &current.Expires)
	return current, err
}

// editable.go：保留原实现，按业务合并文件。

// 按原顺序逐条投影，业务达到页上限即结束；不为分层提前扫描整个可编辑目录。
func VisitEditableUnits(ctx context.Context, tx *sql.Tx, structure string, document uint64, last string, visit func(domain.EditableUnitFact) bool) error {
	rows, err := tx.QueryContext(ctx, "SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? LEFT JOIN branch b ON b.id=u.active_branch_id AND b.unit_id=u.id AND b.document_id=u.document_id WHERE u.document_id=? AND u.id>? AND u.retired_structure_id IS NULL ORDER BY u.id", structure, document, last)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var fact domain.EditableUnitFact
		if rows.Scan(&fact.ID, &fact.Title, &fact.Branch, &fact.Lifecycle, &fact.Epoch, &fact.Lineage) != nil {
			return errors.New("EDITABLE_UNITS_UNAVAILABLE")
		}
		if !visit(fact) {
			break
		}
	}
	return rows.Err()
}
