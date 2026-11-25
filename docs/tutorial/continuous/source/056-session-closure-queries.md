# services/account/internal/repository/account/session_closure_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/056-session-closure-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`EnqueueSourceClosure`、`FinishBase`、`FinishCredential`、`FinishMemberRole`、`FinishMemberDeparture`、`FinishOrganizationTransition`、`Closure`、`NewClosure`、`Claim`、`claimClosureTask`、`MarkRunning`、`LockOwnership`、`Retry`、`statement`、`execute`、`ExpiredSession`、`LockExpiryUser`、`LockExpirySession`、`PrepareSessionClose`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
	"errors"
	"strings"
)

// closure_enqueue.go：保留原实现，按业务合并文件。

// EnqueueSourceClosure只使用用例传入的事务，不能提前提交门闩或恢复任务。
func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure, target []byte) error {
	kind := "ORGANIZATION_CHANGE"
	if payload.Kind == "PLATFORM_GOVERNANCE_REVOKE" {
		kind = "PLATFORM_GOVERNANCE_CHANGE"
	}
	return execute(ctx, tx, []statement{
		{"INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1", []any{payload.Scope, payload.Epoch, id}},
		{"UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=?", []any{payload.Scope, payload.Epoch}},
		{"INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,?,?,?,'FENCING','[\"document\"]',UTC_TIMESTAMP(6))", []any{id, actor, payload.Scope, payload.Epoch, string(target)}},
		{"INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))", []any{kind, id, id, string(target)}},
	})
}

// closure_finish.go：保留原实现，按业务合并文件。

func (r *Closure) FinishBase(ctx context.Context, tx *sql.Tx, task domain.ClosureTask, binding domain.SessionClosure, session, resultDigest, terminal []byte) error {
	return execute(ctx, tx, []statement{
		{"INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", []any{task.OperationID, binding.Scope, binding.Epoch, resultDigest}},
		{"UPDATE session SET state='CLOSED' WHERE id=? AND state='REVOKING'", []any{session}},
		{"UPDATE authorization_change SET state='SUCCEEDED',last_error=NULL,retry_at=NULL WHERE revocation_id=?", []any{task.OperationID}},
		{"UPDATE operation_receipt SET state='SUCCEEDED',result_json=IF(result_json IS NULL,?,JSON_SET(result_json,'$.state','SUCCEEDED')),terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=?", []any{string(terminal), task.OperationID}},
		{"UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=?", []any{task.ID}},
	})
}

func (r *Closure) FinishCredential(ctx context.Context, tx *sql.Tx, userID string, oldEpoch uint64) error {
	return execute(ctx, tx, []statement{
		{"UPDATE session SET state='CLOSED' WHERE user_id=? AND credential_epoch<=? AND state='REVOKING'", []any{userID, oldEpoch}},
		{"UPDATE user SET state='ACTIVE' WHERE id=? AND state='REVOKING' AND credential_epoch=?", []any{userID, oldEpoch + 1}},
	})
}

func (r *Closure) FinishMemberRole(ctx context.Context, tx *sql.Tx, org, user, role string, nextEpoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_member SET state='ACTIVE',role=? WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", role, org, user, nextEpoch)
	return err
}

func (r *Closure) FinishMemberDeparture(ctx context.Context, tx *sql.Tx, org, user string, nextEpoch uint64) error {
	return execute(ctx, tx, []statement{
		{"UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", []any{org, user, nextEpoch}},
		{"UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND user_id=? AND state='ACTIVE'", []any{org, user}},
	})
}

func (r *Closure) FinishOrganizationTransition(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_transition SET state='SUCCEEDED' WHERE operation_id=? AND state='FENCING'", operation)
	return err
}

// closure_task.go：保留原实现，按业务合并文件。

type Closure struct{ db *sql.DB }

func NewClosure(db *sql.DB) *Closure { return &Closure{db: db} }

func (r *Closure) Claim(ctx context.Context, tx *sql.Tx) (domain.ClosureTask, error) {
	return claimClosureTask(ctx, tx, []string{"SESSION_LOGOUT", "CREDENTIAL_CHANGE", "ORGANIZATION_CHANGE", "PLATFORM_GOVERNANCE_CHANGE"})
}

func claimClosureTask(ctx context.Context, tx *sql.Tx, kinds []string) (domain.ClosureTask, error) {
	if len(kinds) == 0 {
		return domain.ClosureTask{}, sql.ErrNoRows
	}
	predicate := "kind IN (" + strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",") + ") AND ((state IN ('QUEUED','RETRY','UNKNOWN') AND retry_at<=UTC_TIMESTAMP(6)) OR (state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6)))"
	args := make([]any, len(kinds))
	for index, kind := range kinds {
		args[index] = kind
	}
	// 排序候选不加范围锁、不搬运载荷。FOR UPDATE的filesort会锁住多个候选，
	// 即使SKIP LOCKED也令其他worker无法领取；到期登记也可能被其间隙锁阻塞。
	rows, err := tx.QueryContext(ctx, "SELECT id FROM durable_task WHERE "+predicate+" ORDER BY id LIMIT 32", args...)
	if err != nil {
		return domain.ClosureTask{}, err
	}
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return domain.ClosureTask{}, err
	}
	for _, id := range ids {
		var task domain.ClosureTask
		lockedArgs := append([]any{id}, args...)
		// 候选快照不授予执行资格；精确主键当前读重新核验种类、状态及期限。
		err = tx.QueryRowContext(ctx, "SELECT id,business_key,payload FROM durable_task WHERE id=? AND ("+predicate+") FOR UPDATE SKIP LOCKED", lockedArgs...).Scan(&task.ID, &task.OperationID, &task.Payload)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		return task, err
	}
	return domain.ClosureTask{}, sql.ErrNoRows
}

func (r *Closure) MarkRunning(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, id)
	return err
}

func (r *Closure) LockOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error) {
	var value domain.TaskOwnership
	err := tx.QueryRowContext(ctx, "SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE", id).Scan(&value.Instance, &value.State)
	return value, err
}

func (r *Closure) Retry(ctx context.Context, id uint64, instance, code string) {
	_, _ = r.db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',lease_until=NULL,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND instance_id=? AND state='RUNNING'", code, id, instance)
}

type statement struct {
	query string
	args  []any
}

func execute(ctx context.Context, tx *sql.Tx, commands []statement) error {
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, command.query, command.args...); err != nil {
			return err
		}
	}
	return nil
}

// session_expiry.go：保留原实现，按业务合并文件。

func (r *Closure) ExpiredSession(ctx context.Context) (domain.Session, error) {
	var value domain.Session
	err := r.db.QueryRowContext(ctx, "SELECT id,CAST(user_id AS CHAR) FROM session WHERE state='ACTIVE' AND expires_at<=UTC_TIMESTAMP(6) ORDER BY expires_at,id LIMIT 1").Scan(&value.ID, &value.UserID)
	return value, err
}

func (r *Closure) LockExpiryUser(ctx context.Context, tx *sql.Tx, user string) error {
	var state string
	return tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
}

func (r *Closure) LockExpirySession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,epoch,expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Epoch, &value.Expires)
	return value, err
}

func (r *Closure) PrepareSessionClose(ctx context.Context, tx *sql.Tx, value domain.SessionClosePreparation) error {
	// 会话状态、代次和持久gate是关闭权威；准入申请、复核及后台续期都在相同
	// user→session锁下核验它们。不能在这里改写会话全部历史准入：长期会话可
	// 积累数万条记录，使短事务反复超时回滚，反而无法提交关闭点及可靠任务。
	// 保留原准入供审计，OPEN也不代表仍有效；会话关闭后原证明不能再次通过核验。
	return execute(ctx, tx, []statement{
		{"INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,expires_at) VALUES(?,?,'SESSION_LOGOUT',?,?,'PREPARING',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))", []any{value.OperationID, value.UserID, string(value.Target), value.Fingerprint}},
		{"UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE id=?", []any{value.SessionID}},
		{"INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1", []any{value.Scope, value.Epoch, value.OperationID}},
		{"INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at) VALUES(?,?,?,?,?,'FENCING','[\"document\"]',UTC_TIMESTAMP(6))", []any{value.OperationID, value.UserID, value.Scope, value.Epoch, string(value.Target)}},
		{"INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('SESSION_LOGOUT',?,?,?,'QUEUED',UTC_TIMESTAMP(6))", []any{value.OperationID, value.OperationID, string(value.Payload)}},
	})
}
````

UTF-8/LF全文SHA-256：`731819d60f12053cebedf6a9e3bd8f115e5afc90e8a9e8190f8bda593bd87832`。[参考实现](../../../../services/account/internal/repository/account/session_closure_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
