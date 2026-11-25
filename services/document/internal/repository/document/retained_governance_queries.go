// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	allocations "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
)

// retained_governance.go：保留原实现，按业务合并文件。

const retainedGovernanceTask = "RETAINED_GOVERNANCE_RECOVER"

type RetainedGovernanceRecord struct {
	Plan  domain.RetainedGovernancePlan
	State string
}

func RetainedGovernanceRow(ctx context.Context, q Queryer, operation string, lock bool) (RetainedGovernanceRecord, error) {
	var row RetainedGovernanceRecord
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT payload,state FROM durable_task WHERE kind=? AND business_key=? AND operation_id=?"+suffix, retainedGovernanceTask, operation, operation).Scan(&raw, &row.State)
	if err != nil {
		return row, err
	}
	row.Plan, err = domain.DecodeRetainedGovernancePlan(raw)
	if err == nil && row.Plan.Purge.Operation != operation {
		err = domain.Violation(domain.StateConflict, "RETAINED_PLAN_BINDING_CHANGED")
	}
	return row, err
}

// 发现本Doc全部资源只固定清理范围，不提供物理删除许可。最后由资源行锁
// 重查跨Doc引用和UNKNOWN保护，故共享资源不会随孤立文章误删。
func RetainedDocumentResources(ctx context.Context, q Queryer, document uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, "SELECT DISTINCT resource_id FROM resource_reference WHERE document_id=? ORDER BY resource_id", document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func BeginRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error {
	p := plan.Purge
	resources, err := RetainedDocumentResources(ctx, tx, p.Document)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(resources, plan.Resources) {
		return domain.Violation(domain.StateConflict, "RETAINED_RESOURCES_CHANGED")
	}
	state := "PUBLIC_RETAINED"
	if plan.Action == "PURGE" {
		state = "CLOSING"
	}
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle=?,control_revision=?,acl_revision=?,publication_epoch=?,event_seq=?,
	 current_release_id=NULL,publication_state='UNPUBLISHED',visibility_revision=visibility_revision+1,main_locked=TRUE
	 WHERE id=? AND lifecycle='PUBLIC_RETAINED' AND responsible_user_id IS NULL AND control_revision=?`,
		state, p.ClosingControl, p.ACL, p.Publication, p.Sequence, p.Document, p.ExpectedControl)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = InsertSubmissionTask(ctx, tx, retainedGovernanceTask, p.Operation, p.Operation, raw, "QUEUED"); err != nil {
		return err
	}
	for _, source := range p.Scopes {
		// 关闭编号不可逆派生，额外保存精确父原号用于后续DELETE核验。
		// 两方ACK完成但父治理receipt未终结时，不允许新号抢先推进水位。
		binding, err := json.Marshal(source)
		if err != nil {
			return err
		}
		if err = InsertSubmissionTask(ctx, tx, "RETAINED_GOVERNANCE_FENCE", source.Operation, p.Operation, binding, "SUCCEEDED"); err != nil {
			return err
		}
	}
	// 通用事务会保存item_no=1的原号审计；这里补充治理前后真实版本和范围，
	// 与指针/关闭计划同一事务，失败不会留下“审计成功而文章未撤下”的分裂。
	target, _ := json.Marshal(map[string]string{"document_id": domain.TextID(p.Document)})
	before, _ := json.Marshal(map[string]any{"lifecycle": "PUBLIC_RETAINED", "release_id": domain.TextID(plan.Release), "control_revision": domain.TextID(p.ExpectedControl)})
	after, _ := json.Marshal(map[string]any{"action": plan.Action, "lifecycle": state, "control_revision": domain.TextID(p.ClosingControl), "publication_epoch": domain.TextID(p.Publication)})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,before_json,after_json,reason)
	 VALUES(?,?,2,'RETAINED_PUBLIC_GOVERNANCE',?,?,?,'平台治理孤立公开留存；禁止代写正文')`, p.Actor, p.Operation, string(target), string(before), string(after))
	return err
}

func ClaimRetainedGovernance(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task WHERE kind=?
	 AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))
	 ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, retainedGovernanceTask).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, errors.New("RETAINED_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, task.Generation, task.ID)
	return task, err
}

func LockRetainedGovernanceOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	var generation uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind=? AND operation_id=? FOR UPDATE", task.ID, retainedGovernanceTask, task.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || owner != instance {
		return domain.Violation(domain.StateConflict, "RETAINED_WORKER_CHANGED")
	}
	return nil
}

func RetryRetainedGovernance(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='RETAINED_GOVERNANCE_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", task.ID, task.Generation, instance)
	return err
}

func CompleteRetainedGovernance(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan, result []byte) error {
	p := plan.Purge
	if plan.Action == "PURGE" {
		updated, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',purge_after=NULL,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)) WHERE id=? AND lifecycle='CLOSING' AND control_revision=? AND acl_revision=? AND publication_epoch=?", p.Document, p.ClosingControl, p.ACL, p.Publication)
		if err != nil {
			return err
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
		}
	}
	updated, err := tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
	 WHERE operation_id=? AND actor_id=? AND kind=? AND state='PREPARING'`, string(result), p.Operation, p.Actor, domain.RetainedGovernanceKind(plan.Action))
	if err != nil {
		return err
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return domain.Violation(domain.StateConflict, "RETAINED_RECEIPT_CHANGED")
	}
	return nil
}

// retained_governance_cleanup.go：保留原实现，按业务合并文件。

func RetainedGovernanceCleanupReceipt(ctx context.Context, q Queryer, operation string) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var raw []byte
	var original, state string
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='RETAINED_GOVERNANCE_CLEANED' AND business_key=?", operation).Scan(&original, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if original != operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.Operation != operation {
		return result, false, errors.New("RETAINED_CLEANUP_RECEIPT_CHANGED")
	}
	return result, true, nil
}

func ReleaseRetainedResourcePins(ctx context.Context, tx *sql.Tx, document uint64) error {
	items, err := LockDocumentResourceAllocationCleanup(ctx, tx, document)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE document_id=? AND owner_kind='RETENTION'", document); err != nil {
		return err
	}
	var space uint64
	if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, document).Scan(&space); err != nil {
		return err
	}
	return ReconcileCleanedResourceAllocations(ctx, tx, document, space, items)
}

func RemoveRetainedExportControls(ctx context.Context, tx *sql.Tx, document uint64) error {
	drained, err := RetainedExportArtifactsDrained(ctx, tx, document)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "RETAINED_EXPORT_DRAIN_PENDING")
	}
	_, err = tx.ExecContext(ctx, "DELETE c FROM export_control c JOIN export_job j ON j.id=c.job_id WHERE j.document_id=?", document)
	return err
}

// 每个固定资源须已物理DELETED，或由另一文档的真实引用/确定保护继续保留。
// 原UPLOAD pin只是恢复凭据；它不能把未删且无业务引用的对象伪装为共享保留。
func ValidateRetainedResourceCompletion(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) error {
	for _, id := range plan.Resources {
		var space uint64
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT space_id,state FROM resource WHERE id=? FOR UPDATE", id).Scan(&space, &state); err != nil {
			return err
		}
		if space != plan.Purge.Space {
			known, err := allocations.ResourceTransferEvidence(ctx, tx, id, plan.Purge.Document, plan.Purge.Space)
			if err != nil {
				return err
			}
			if !known {
				return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_BINDING_CHANGED")
			}
		}
		if state == "DELETED" {
			continue
		}
		var own, shared, unknown bool
		err := tx.QueryRowContext(ctx, `SELECT
		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')),
		 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id<>? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD'))
		 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id<>? AND state IN ('PROTECTED','CONFIRMED') AND JSON_CONTAINS(resource_ids,CAST(? AS JSON))),
		 EXISTS(SELECT 1 FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)))`,
			id, plan.Purge.Document, id, plan.Purge.Document, plan.Purge.Document, domain.TextID(id), domain.TextID(id)).Scan(&own, &shared, &unknown)
		if err != nil {
			return err
		}
		if own || unknown || state != "READY" || !shared {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_CLEANUP_PENDING")
		}
	}
	return nil
}

// retained_governance_drains.go：保留原实现，按业务合并文件。

// 不从BLOCKED/任务数量/租约推断结束。必须是原范围、原关闭任务及两端
// OUTPUT_STOPPED摘要同时匹配，才能继续历史清理或已撤下对象的下一次治理。
func RetainedGovernanceDrained(ctx context.Context, tx *sql.Tx, plan domain.RetainedGovernancePlan) (bool, error) {
	for _, source := range plan.Purge.Scopes {
		drained, err := retainedScopeDrained(ctx, tx, source.Key, source.Epoch, source.Operation, plan.Purge.Actor, plan.Purge.Document)
		if err != nil || !drained {
			return false, err
		}
	}
	return true, nil
}

func RetainedClosedSourceDrained(ctx context.Context, tx *sql.Tx, key string, epoch, document uint64) (bool, error) {
	var operation string
	var actor uint64
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0)
	 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id
	 WHERE g.scope_key=? AND g.max_epoch=? AND g.state='BLOCKED'`, key, epoch).Scan(&operation, &actor)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var parent string
	if err = tx.QueryRowContext(ctx, "SELECT operation_id FROM durable_task WHERE kind='RETAINED_GOVERNANCE_FENCE' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&parent); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	original, err := RetainedGovernanceRow(ctx, tx, parent, false)
	if err != nil {
		return false, err
	}
	if original.State != "SUCCEEDED" || original.Plan.Purge.Actor != actor || original.Plan.Purge.Document != document {
		return false, nil
	}
	found := false
	for _, source := range original.Plan.Purge.Scopes {
		found = found || (source.Key == key && source.Epoch == epoch && source.Operation == operation)
	}
	if !found {
		return false, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_BINDING_CHANGED")
	}
	return retainedScopeDrained(ctx, tx, key, epoch, operation, actor, document)
}

func retainedScopeDrained(ctx context.Context, tx *sql.Tx, key string, epoch uint64, operation string, actor, document uint64) (bool, error) {
	var taskOperation, taskState, authState, scope, gateState, revocation string
	var actualEpoch, gateEpoch, actualActor uint64
	var raw, target, participants []byte
	err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,
	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
	 FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
	 WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, operation).
		Scan(&taskOperation, &taskState, &raw, &authState, &scope, &actualEpoch, &target, &gateEpoch, &gateState, &revocation, &actualActor, &participants)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var closure domain.Closure
	var binding map[string]string
	var expectedParticipants []string
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: key, Epoch: epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(key, epoch)}
	if !domain.ValidOperationID(operation) || actor == 0 || taskOperation != operation ||
		json.Unmarshal(raw, &closure) != nil || closure.Scope != key || closure.Epoch != epoch || closure.Entity != document || closure.Action != domain.RetainedGovernancePurpose ||
		json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &expectedParticipants) != nil ||
		actualActor != actor || scope != key || actualEpoch != epoch || !slices.Equal(expectedParticipants, []string{"edge", "collaboration"}) ||
		gateEpoch != epoch || gateState != "BLOCKED" || revocation != operation || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) {
		return false, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_BINDING_CHANGED")
	}
	if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
		return false, nil
	}
	for _, participant := range expectedParticipants {
		var digest []byte
		err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, key, epoch).Scan(&digest)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
			return false, domain.Violation(domain.StateConflict, "RETAINED_ACK_CHANGED")
		}
	}
	return true, nil
}

func RetainedExportArtifactsDrained(ctx context.Context, tx *sql.Tx, document uint64) (bool, error) {
	var pending bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM export_job j JOIN export_control c ON c.job_id=j.id
	 WHERE j.document_id=? AND (c.artifact_key IS NOT NULL OR c.artifact_size<>0 OR c.reserved_bytes<>0))`, document).Scan(&pending)
	return !pending, err
}
