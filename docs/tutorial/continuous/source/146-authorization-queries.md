# services/document/internal/repository/document/authorization_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/146-authorization-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Access`、`NewAccess`、`KnowledgeBase`、`KnowledgeBaseRole`、`Grants`、`Units`、`LockedKnowledgeBase`、`Document`、`Space`、`AdmissionSources`、`AdmissionState`、`LockedAdmission`、`MarkWebSocketDelivery`、`Authorizations`、`NewAuthorizations`、`InsertAuthorization`、`InsertAuthorizationTask`、`AuthorizationProjection`、`AuthorizationProgress`、`ProjectionAcknowledgment`、`ProgressAcknowledgment`、`ClaimAuthorization`、`RunAuthorizationTask`、`LockedTaskOwnership`、`TrashClosingDocument`、`TrashClosingKnowledgeBaseDocuments`、`CloseKnowledgeBase`、`CompleteAuthorizationReceipt`、`CompleteAuthorizationTask`、`Retry`、`EditGrantScope`、`SetContentGrant`、`AdvanceDocumentACL`、`Departures`、`NewDepartures`、`EnsureDepartureGate`、`LockedDepartureGate`、`HasKnowledgeBaseAdministration`、`BlockMemberDeparture`、`CloseOrganizationMemberFacts`、`Pending`、`Reopen`、`OriginalAdmissionBinding`、`OriginalAdmission`、`SourceGate`、`LockSourceGate`、`PendingSourceTransition`、`ReopenSourceGate`、`LockedSourceGate`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
	"time"
)

// access.go：保留原实现，按业务合并文件。

type Access struct{ db *sql.DB }

func NewAccess(db *sql.DB) *Access { return &Access{db: db} }
func (r *Access) KnowledgeBase(ctx context.Context, id, space uint64) (string, uint64, error) {
	var state string
	var revision uint64
	err := r.db.QueryRowContext(ctx, "SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?", id, space).Scan(&state, &revision)
	return state, revision, err
}
func (r *Access) KnowledgeBaseRole(ctx context.Context, id, actor uint64) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, "SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE'", id, actor).Scan(&role)
	return role, err
}
func (r *Access) Grants(ctx context.Context, id, actor uint64) ([]domain.GrantFact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state='ACTIVE'", id, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []domain.GrantFact
	for rows.Next() {
		var grant domain.GrantFact
		if rows.Scan(&grant.User, &grant.Kind, &grant.Scope, &grant.Capability) != nil {
			return nil, errors.New("GRANT_UNAVAILABLE")
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
func (r *Access) Units(ctx context.Context, structure string, id uint64) ([]domain.UnitFact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id", structure, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var units []domain.UnitFact
	for rows.Next() {
		var unit domain.UnitFact
		if rows.Scan(&unit.ID, &unit.Chapter) != nil {
			return nil, errors.New("UNIT_UNAVAILABLE")
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}
func LockedKnowledgeBase(ctx context.Context, tx *sql.Tx, id string) (string, uint64, error) {
	var state string
	var revision uint64
	err := tx.QueryRowContext(ctx, "SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE", id).Scan(&state, &revision)
	return state, revision, err
}

func (r *Access) Document(ctx context.Context, id uint64) (dto.Document, error) {
	return DocumentRow(ctx, r.db, id, false)
}
func (r *Access) Space(ctx context.Context, id uint64) (dto.Space, error) {
	return SpaceRow(ctx, r.db, id, false)
}

// admission.go：保留原实现，按业务合并文件。

func AdmissionSources(ctx context.Context, q Queryer, admission string) ([]*pb.SourceEpoch, error) {
	rows, err := q.QueryContext(ctx, "SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key", admission)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*pb.SourceEpoch
	for rows.Next() {
		var source pb.SourceEpoch
		if err := rows.Scan(&source.ScopeKey, &source.Epoch); err != nil {
			return nil, err
		}
		result = append(result, &source)
	}
	if len(result) == 0 {
		return nil, domain.ErrSourcesRequired
	}
	return result, rows.Err()
}
func (r *Outputs) AdmissionSources(ctx context.Context, admission string) ([]*pb.SourceEpoch, error) {
	return AdmissionSources(ctx, r.db, admission)
}

type AdmissionState struct {
	TargetJSON []byte
	State      string
	Expires    time.Time
}

func LockedAdmission(ctx context.Context, tx *sql.Tx, admission string) (AdmissionState, error) {
	var current AdmissionState
	err := tx.QueryRowContext(ctx, "SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.TargetJSON, &current.State, &current.Expires)
	return current, err
}
func MarkWebSocketDelivery(ctx context.Context, tx *sql.Tx, lease string, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE delivery_session SET target_kind='WSS',expires_at=? WHERE delivery_id=?", expires, lease)
	return err
}

// authorization.go：保留原实现，按业务合并文件。

type Authorizations struct{ db *sql.DB }

func NewAuthorizations(db *sql.DB) *Authorizations { return &Authorizations{db: db} }

// 所有写入接受调用方事务；关闭计划与业务变更不可拆成两次提交。
func InsertAuthorization(ctx context.Context, tx *sql.Tx, operation string, actor uint64, scope string, epoch uint64, binding []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,'FENCING','[\"edge\",\"collaboration\"]')", operation, actor, scope, epoch, string(binding))
	return err
}
func InsertAuthorizationTask(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('AUTHORIZATION_CHANGE',?,?,?,'QUEUED',UTC_TIMESTAMP(6))", operation, operation, string(payload))
	return err
}
func AuthorizationProjection(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error) {
	var result domain.AuthorizationStatus
	err := tx.QueryRowContext(ctx, "SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?", operation, actor).Scan(&result.State, &result.Scope, &result.Epoch)
	return result, err
}
func AuthorizationProgress(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error) {
	var result domain.AuthorizationStatus
	err := tx.QueryRowContext(ctx, "SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.operation_id=? AND r.actor_id=?", operation, actor).Scan(&result.State, &result.Scope, &result.Epoch, &result.ReceiptState)
	return result, err
}
func ProjectionAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase='OUTPUT_STOPPED'", operation, participant).Scan(&count)
	return count, err
}
func ProgressAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant, scope, epoch string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, scope, epoch).Scan(&count)
	return count, err
}
func ClaimAuthorization(ctx context.Context, tx *sql.Tx) (domain.AuthorizationTask, error) {
	// UNKNOWN仍按原撤权协议续接；原5秒执行租约由RunAuthorizationTask登记，不套用控制任务代次。
	task, err := lockDueTask(ctx, tx, authorizationTaskDue)
	return domain.AuthorizationTask{ID: task.ID, Operation: task.Operation, Payload: task.Payload}, err
}
func RunAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, id)
	return err
}
func LockedTaskOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error) {
	var owner domain.TaskOwnership
	err := tx.QueryRowContext(ctx, "SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE", id).Scan(&owner.Instance, &owner.State)
	return owner, err
}
func TrashClosingDocument(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='TRASHED',deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE id=? AND lifecycle='CLOSING'", id)
	return err
}
func TrashClosingKnowledgeBaseDocuments(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='TRASHED',deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE knowledge_base_id=? AND lifecycle='CLOSING'", id)
	return err
}
func CloseKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSED' WHERE id=? AND lifecycle='CLOSING'", id)
	return err
}
func CompleteAuthorizationReceipt(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',terminal_at=UTC_TIMESTAMP(6),result_json=JSON_SET(result_json,'$.state','SUCCEEDED') WHERE operation_id=? AND state='PREPARING'", operation)
	return err
}
func CompleteAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?", id, instance)
	return err
}
func (r *Authorizations) Retry(ctx context.Context, id uint64, instance string) {
	_, _ = r.db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='FENCE_NOT_CONFIRMED' WHERE id=? AND instance_id=? AND state='RUNNING'", id, instance)
}

// authorization_change.go：保留原实现，按业务合并文件。

// 表名来自已验证的内部scope枚举，不接受外部SQL片段。
func EditGrantScope(ctx context.Context, tx *sql.Tx, kind string, scope, document uint64) error {
	table := "collaboration_unit"
	if kind == "CHAPTER" {
		table = "chapter"
	}
	var found uint64
	return tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE id=? AND document_id=? AND retired_structure_id IS NULL", scope, document).Scan(&found)
}
func SetContentGrant(ctx context.Context, tx *sql.Tx, document uint64, grant domain.GrantFact, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", document, grant.User, grant.Kind, grant.Scope, grant.Capability, state)
	return err
}
func AdvanceDocumentACL(ctx context.Context, tx *sql.Tx, document uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET acl_revision=acl_revision+1 WHERE id=?", document)
	return err
}

// departure.go：保留原实现，按业务合并文件。

type Departures struct{ db *sql.DB }

func NewDepartures(db *sql.DB) *Departures { return &Departures{db: db} }

const departureAdminSQL = "SELECT EXISTS(SELECT 1 FROM knowledge_base_member m JOIN knowledge_base k ON k.id=m.knowledge_base_id JOIN space s ON s.id=k.space_id WHERE s.kind='ORGANIZATION' AND s.organization_id=? AND k.lifecycle IN ('ACTIVE','CLOSING') AND m.user_id=? AND m.role='ADMIN' AND m.state IN ('ACTIVE','REVOKING'))"

func EnsureDepartureGate(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", binding.Scope(), binding.Epoch)
	return err
}
func LockedDepartureGate(ctx context.Context, tx *sql.Tx, scope string) (domain.DepartureGate, error) {
	var gate domain.DepartureGate
	var operation sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE", scope).Scan(&gate.Epoch, &gate.State, &operation)
	gate.Operation = operation.String
	return gate, err
}
func HasKnowledgeBaseAdministration(ctx context.Context, tx *sql.Tx, organization, user uint64) (bool, error) {
	var admin bool
	err := tx.QueryRowContext(ctx, departureAdminSQL, organization, user).Scan(&admin)
	return admin, err
}
func BlockMemberDeparture(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1 WHERE scope_key=?", binding.Epoch, binding.ID, binding.Scope())
	return err
}
func CloseOrganizationMemberFacts(ctx context.Context, tx *sql.Tx, organization, user uint64) error {
	commands := []string{
		"UPDATE knowledge_base_member m JOIN knowledge_base k ON k.id=m.knowledge_base_id JOIN space s ON s.id=k.space_id SET m.state='CLOSED',m.revision=m.revision+1 WHERE s.organization_id=? AND m.user_id=? AND m.state<>'CLOSED'",
		"UPDATE knowledge_base_creation_grant g JOIN knowledge_base k ON k.id=g.knowledge_base_id JOIN space s ON s.id=k.space_id SET g.state='CLOSED',g.revision=g.revision+1 WHERE s.organization_id=? AND g.user_id=? AND g.state<>'CLOSED'",
		"UPDATE content_grant g JOIN document d ON d.id=g.document_id JOIN space s ON s.id=d.space_id SET g.state='CLOSED',g.revision=g.revision+1 WHERE s.organization_id=? AND g.user_id=? AND g.state<>'CLOSED'",
		"UPDATE document d JOIN space s ON s.id=d.space_id SET d.responsible_user_id=NULL,d.control_revision=d.control_revision+1 WHERE s.organization_id=? AND d.responsible_user_id=?",
	}
	for _, statement := range commands {
		if _, err := tx.ExecContext(ctx, statement, organization, user); err != nil {
			return err
		}
	}
	return nil
}
func (r *Departures) Pending(ctx context.Context) (domain.DepartureObservation, error) {
	var observed domain.DepartureObservation
	err := r.db.QueryRowContext(ctx, "SELECT g.scope_key,g.last_revocation_id,g.max_epoch FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id WHERE g.scope_key LIKE 'org-member:%' AND g.state='BLOCKED' AND g.last_revocation_id IS NOT NULL AND c.revocation_id IS NULL ORDER BY g.scope_key LIMIT 1").Scan(&observed.Scope, &observed.Operation, &observed.Epoch)
	return observed, err
}
func (r *Departures) Reopen(ctx context.Context, observed domain.DepartureObservation) {
	_, _ = r.db.ExecContext(ctx, "UPDATE access_gate SET state='OPEN',last_revocation_id=NULL,revision=revision+1 WHERE scope_key=? AND max_epoch=? AND state='BLOCKED' AND last_revocation_id=?", observed.Scope, observed.Epoch, observed.Operation)
}

// original_admission.go：保留原实现，按业务合并文件。

type OriginalAdmissionBinding struct {
	Actor, Document, Unit        uint64
	Identity, Operation, Purpose string
	Target, Session              []byte
	SessionExpires               int64
}

// 原准入证明所有持久操作的归属，不能另取新准入替换固定目标。所有字段在同一调用方事务读取。
func OriginalAdmission(ctx context.Context, q Queryer, admission string, locked bool) (OriginalAdmissionBinding, error) {
	var value OriginalAdmissionBinding
	statement := `SELECT actor_id,document_id,COALESCE(unit_id,0),identity_admission_ref,
 operation_id,purpose,target_json,session_ref,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED)
 FROM document_admission WHERE admission_id=?`
	if locked {
		statement += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, statement, admission).Scan(&value.Actor, &value.Document, &value.Unit, &value.Identity, &value.Operation, &value.Purpose, &value.Target, &value.Session, &value.SessionExpires)
	return value, err
}

// source_gate.go：保留原实现，按业务合并文件。

type SourceGate struct {
	Epoch uint64
	State string
}

func LockSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) (SourceGate, error) {
	var gate SourceGate
	// INSERT IGNORE会取得共享锁并造成升级死锁；无值变化的UPSERT直接取得排他锁。
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", key, epoch)
	if err != nil {
		return gate, err
	}
	err = tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&gate.Epoch, &gate.State)
	if err != nil {
		return gate, errors.New("GATE_UNAVAILABLE")
	}
	return gate, nil
}
func PendingSourceTransition(ctx context.Context, tx *sql.Tx, key string) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN ('SUCCEEDED','REJECTED') LIMIT 1 FOR UPDATE", key).Scan(&state)
	if err == sql.ErrNoRows {
		return PersonalTransferSourcePending(ctx, tx, key)
	}
	return err == nil, err
}
func ReopenSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='OPEN',revision=revision+1 WHERE scope_key=?", epoch, key)
	return err
}

// LockedSourceGate 只锁已有来源；缺失记录保留为未知，不能在恢复时补出开放来源。
func LockedSourceGate(ctx context.Context, tx *sql.Tx, key string) (SourceGate, error) {
	var gate SourceGate
	err := tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&gate.Epoch, &gate.State)
	return gate, err
}
````

UTF-8/LF全文SHA-256：`c6f83fce83533d52f106959065180f1dfab54ea7dc442c1d83f4f79798bdda4b`。[参考实现](../../../../services/document/internal/repository/document/authorization_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
