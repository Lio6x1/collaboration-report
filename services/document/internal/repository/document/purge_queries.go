// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

// purge.go：保留原实现，按业务合并文件。

type PurgeRecord struct {
	Plan  domain.PurgePlan
	State string
}

func PurgeRow(ctx context.Context, q Queryer, operation string, lock bool) (PurgeRecord, error) {
	var record PurgeRecord
	var raw []byte
	var kind string
	var actor, revision uint64
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT kind,state,COALESCE(actor_id,0),expected_revision,fixed_plan FROM lifecycle_operation WHERE operation_id=?"+suffix, operation).
		Scan(&kind, &record.State, &actor, &revision, &raw)
	if err != nil {
		return record, err
	}
	record.Plan, err = domain.DecodePurgePlan(raw)
	if err != nil {
		return record, err
	}
	if kind != "PURGE" || record.Plan.Operation != operation || record.Plan.Actor != actor || record.Plan.ExpectedControl != revision {
		return record, domain.Violation(domain.StateConflict, "PURGE_BINDING_CHANGED")
	}
	return record, nil
}

func LifecycleKind(ctx context.Context, q Queryer, operation string) (string, error) {
	var kind string
	err := q.QueryRowContext(ctx, "SELECT kind FROM lifecycle_operation WHERE operation_id=?", operation).Scan(&kind)
	return kind, err
}

func RegisterPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	var actor any
	if plan.Actor != 0 {
		actor = plan.Actor
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) VALUES(?,?,'PURGE','PREPARING',?,?)", plan.Operation, actor, plan.ExpectedControl, string(raw)); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "PURGE_RECOVER", plan.Operation, plan.Operation, raw, "QUEUED")
}

func BeginPurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	result, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',control_revision=?,acl_revision=?,
	 publication_epoch=?,event_seq=?,visibility_policy='COLLABORATORS',visibility_revision=visibility_revision+1,
	 current_release_id=NULL,publication_state='UNPUBLISHED' WHERE id=? AND lifecycle='TRASHED' AND control_revision=?`,
		plan.ClosingControl, plan.ACL, plan.Publication, plan.Sequence, plan.Document, plan.ExpectedControl)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	return nil
}

func SetPurgeCleaning(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING' WHERE operation_id=? AND kind='PURGE' AND state='PREPARING'", operation)
	return err
}

func CompletePurgeDocument(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	result, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',purge_after=NULL,main_locked=TRUE WHERE id=? AND lifecycle='CLOSING' AND acl_revision=? AND publication_epoch=? AND control_revision=?", plan.Document, plan.ACL, plan.Publication, plan.ClosingControl)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	result, err = tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1,last_error=NULL WHERE operation_id=? AND kind='PURGE' AND state='CLEANING'", plan.Operation)
	if err != nil {
		return err
	}
	changed, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_ORIGINAL_CHANGED")
	}
	return nil
}

// 原用户/系统回执必须存在并属于这个原计划；缺失回执不能被伪造成清理成功。
func CompletePurgeReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, result []byte) error {
	updated, err := tx.ExecContext(ctx, `UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
	 WHERE operation_id=? AND actor_id=? AND kind='DOCUMENT_PURGE' AND state='PREPARING'`, string(result), plan.Operation, plan.Actor)
	if err != nil {
		return err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_ORIGINAL_RECEIPT_CHANGED")
	}
	return nil
}

// purge_drains.go：保留原实现，按业务合并文件。

// 原任务、来源gate、业务绑定及两参与者摘要缺一不可；不按COUNT/TTL猜关闭成功。
func PurgeDrained(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) (bool, error) {
	for _, scope := range plan.Scopes {
		var operation, taskState, authState, key, gateState, revocation string
		var epoch, gateEpoch, actor uint64
		var raw, target, participants []byte
		err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,
	 g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
	 FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
	 WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, scope.Operation).
			Scan(&operation, &taskState, &raw, &authState, &key, &epoch, &target, &gateEpoch, &gateState, &revocation, &actor, &participants)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var closure domain.Closure
		var binding map[string]string
		var expectedParticipants []string
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		if json.Unmarshal(raw, &closure) != nil || json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &expectedParticipants) != nil ||
			operation != scope.Operation || closure.Scope != scope.Key || closure.Epoch != scope.Epoch || closure.Entity != plan.Document || closure.Action != "PURGE_DOCUMENT" ||
			actor != plan.Actor || key != scope.Key || epoch != scope.Epoch || !slices.Equal(expectedParticipants, []string{"edge", "collaboration"}) ||
			gateEpoch != scope.Epoch || gateState != "BLOCKED" || revocation != scope.Operation || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) {
			return false, domain.Violation(domain.StateConflict, "PURGE_DRAIN_BINDING_CHANGED")
		}
		if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
			return false, nil
		}
		for _, participant := range []string{"edge", "collaboration"} {
			var digest []byte
			err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", scope.Operation, participant, scope.Key, scope.Epoch).Scan(&digest)
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
				return false, domain.Violation(domain.StateConflict, "PURGE_ACK_CHANGED")
			}
		}
	}
	return true, nil
}

// purge_expiry.go：保留原实现，按业务合并文件。

// 自动到期是本服务系统动作，不借用用户/Account注销身份。actor_id=0只表示
// 系统原号；lifecycle.actor_id仍NULL，audit与固定plan明确automatic=true。
func RegisterPurgeExpiryReceipt(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan, fingerprint [32]byte, result []byte) error {
	target, err := json.Marshal(map[string]any{"path": "/v1/documents/" + domain.TextID(plan.Document) + "/purge", "automatic": true})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)
 VALUES(?,0,'DOCUMENT_PURGE',?,?,'PREPARING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, plan.Operation, string(target), fingerprint[:], string(result))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json,after_json) VALUES(NULL,?,1,'DOCUMENT_PURGE',?,?)", plan.Operation, string(target), string(result))
	return err
}

// purge_parent.go：保留原实现，按业务合并文件。

// CLOSED不是ACK。调用方已持原kb gate，按其last_revocation定位真实关闭
// 任务和两端摘要；不对父KB重开、不关闭同库其它文档，也不按TTL推定成功。
func ClosedPurgeParentDrained(ctx context.Context, tx *sql.Tx, kb, epoch uint64) (bool, error) {
	scope := "kb-acl:" + domain.TextID(kb)
	var operation, taskOperation, taskState, authState string
	var actualEpoch uint64
	var payload, bindingRaw, participantsRaw []byte
	err := tx.QueryRowContext(ctx, `SELECT g.last_revocation_id,t.operation_id,t.state,t.payload,c.state,c.epoch,c.target_permission,c.participant_plan
 FROM access_gate g JOIN authorization_change c ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key
 JOIN durable_task t ON t.kind='AUTHORIZATION_CHANGE' AND t.business_key=c.revocation_id
 WHERE g.scope_key=? AND g.max_epoch=? AND g.state='BLOCKED' FOR UPDATE`, scope, epoch).
		Scan(&operation, &taskOperation, &taskState, &payload, &authState, &actualEpoch, &bindingRaw, &participantsRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var closure domain.Closure
	var binding map[string]string
	var participants []string
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope, epoch)}
	if !domain.ValidOperationID(operation) || taskOperation != operation || actualEpoch != epoch ||
		json.Unmarshal(payload, &closure) != nil || closure.Scope != scope || closure.Epoch != epoch || closure.Entity != kb || closure.Action != "CLOSE_KB" ||
		json.Unmarshal(bindingRaw, &binding) != nil || binding["generation"] != "1" || binding["bootstrap_target_digest"] != hex.EncodeToString(request.TargetDigest) ||
		json.Unmarshal(participantsRaw, &participants) != nil || !slices.Equal(participants, []string{"edge", "collaboration"}) {
		return false, domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_BINDING_CHANGED")
	}
	if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
		return false, nil
	}
	for _, participant := range participants {
		var digest []byte
		err = tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, scope, epoch).Scan(&digest)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
			return false, domain.Violation(domain.StateConflict, "PURGE_PARENT_ACK_CHANGED")
		}
	}
	return true, nil
}

// purge_tasks.go：保留原实现，按业务合并文件。

type PurgeTask struct {
	ID, Generation uint64
	Operation      string
}

// 小批公平领取：行锁不能让一个过期RUNNING任务占住整个队列；执行租约只用于
// worker ownership，绝不替代原ACK、Node CLEANED或正文UNKNOWN的终态证明。
func ClaimPurge(ctx context.Context, tx *sql.Tx, instance string) (PurgeTask, error) {
	var task PurgeTask
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,generation FROM durable_task
	 WHERE kind='PURGE_RECOVER' AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6))
	 OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6))) ORDER BY retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&task.ID, &task.Operation, &task.Generation)
	if err != nil {
		return task, err
	}
	if task.Generation == ^uint64(0) {
		return task, domain.Violation(domain.StateConflict, "PURGE_GENERATION_EXHAUSTED")
	}
	task.Generation++
	_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,generation=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, task.Generation, task.ID)
	return task, err
}

func LockPurgeOwnership(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	var generation uint64
	var state, owner string
	err := tx.QueryRowContext(ctx, "SELECT generation,state,COALESCE(instance_id,'') FROM durable_task WHERE id=? AND kind='PURGE_RECOVER' AND operation_id=? FOR UPDATE", task.ID, task.Operation).Scan(&generation, &state, &owner)
	if err != nil {
		return err
	}
	if generation != task.Generation || state != "RUNNING" || owner != instance {
		return domain.Violation(domain.StateConflict, "PURGE_WORKER_CHANGED")
	}
	return nil
}

func FinishPurgeTask(ctx context.Context, tx *sql.Tx, task PurgeTask, instance string) error {
	result, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=? AND generation=? AND state='RUNNING'", task.ID, instance, task.Generation)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.Violation(domain.StateConflict, "PURGE_WORKER_CHANGED")
	}
	return nil
}

func RetryPurge(ctx context.Context, db *sql.DB, task PurgeTask, instance string) error {
	_, err := db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='PURGE_PENDING' WHERE id=? AND generation=? AND instance_id=? AND state='RUNNING'", task.ID, task.Generation, instance)
	return err
}

func ExpiredTrashCandidates(ctx context.Context, q Queryer, after uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT d.id FROM document d WHERE d.id>? AND d.lifecycle='TRASHED' AND d.purge_after<=UTC_TIMESTAMP(6)
	 AND EXISTS(SELECT 1 FROM space s WHERE s.id=d.space_id AND s.lifecycle='ACTIVE') ORDER BY d.id LIMIT 16`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
