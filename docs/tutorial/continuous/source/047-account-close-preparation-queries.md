# services/account/internal/repository/account/account_close_preparation_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/047-account-close-preparation-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`AccountClosures`、`NewAccountClosures`、`Hash`、`LockUser`、`LockSession`、`Receipt`、`Record`、`HasPending`、`Prepare`、`Bind`、`Reject`、`accountCloseChanged`、`AccountCloseFences`、`NewAccountCloseFences`、`scanAccountCloseFenceTask`、`Claim`、`accountCloseFenceFacts`、`accountCloseFenceFactDestinations`、`validateAccountCloseFence`、`Binding`、`Retry`、`Finish`、`InvalidTasks`、`SavePreparation`、`Freeze`、`validateAccountCloseFreeze`、`freezeAccountCloseSessions`、`freezeAccountCloseScope`、`UserForOperation`、`LockOrganizations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

// account_close.go：保留原实现，按业务合并文件。

type AccountClosures struct{ db *sql.DB }

func NewAccountClosures(db *sql.DB) *AccountClosures { return &AccountClosures{db: db} }

func (r *AccountClosures) Hash(ctx context.Context, user uint64) ([]byte, error) {
	var hash []byte
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(password_hash,X'') FROM user WHERE id=?", user).Scan(&hash)
	return hash, err
}

func (r *AccountClosures) LockUser(ctx context.Context, tx *sql.Tx, user uint64) (domain.Credential, error) {
	var credential domain.Credential
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(password_hash,X''),CAST(credential_epoch AS CHAR),identity_epoch,state
		FROM user WHERE id=? FOR UPDATE`, user).Scan(&credential.Hash, &credential.Epoch, &credential.IdentityEpoch, &credential.State)
	return credential, err
}

func (r *AccountClosures) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	session := domain.Session{ID: id}
	err := tx.QueryRowContext(ctx, `SELECT CAST(user_id AS CHAR),state,CAST(credential_epoch AS CHAR),epoch,expires_at
		FROM session WHERE id=? FOR UPDATE`, id).Scan(&session.UserID, &session.State, &session.CredentialEpoch, &session.Epoch, &session.Expires)
	return session, err
}

func (r *AccountClosures) Receipt(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseReceipt, error) {
	var receipt domain.AccountCloseReceipt
	err := tx.QueryRowContext(ctx, `SELECT actor_id,kind,state,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),''),request_fingerprint,result_json
		FROM operation_receipt WHERE operation_id=? FOR UPDATE`, operation).
		Scan(&receipt.User, &receipt.Kind, &receipt.State, &receipt.SessionHash, &receipt.Fingerprint, &receipt.Result)
	return receipt, err
}

func (r *AccountClosures) Record(ctx context.Context, tx *sql.Tx, operation string) (domain.AccountCloseRecord, error) {
	var record domain.AccountCloseRecord
	err := tx.QueryRowContext(ctx, "SELECT user_id,state,retention_plan FROM account_close WHERE operation_id=? FOR UPDATE", operation).
		Scan(&record.User, &record.State, &record.Capture)
	return record, err
}

func (r *AccountClosures) HasPending(ctx context.Context, tx *sql.Tx, user uint64) (bool, error) {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT operation_id FROM account_close WHERE user_id=?
		AND state NOT IN ('SUCCEEDED','REJECTED') ORDER BY operation_id LIMIT 1 FOR UPDATE`, user).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Prepare只写入准备记录和原回执，同一事务回滚时两者一起撤销；这里不冻结身份来源。
func (r *AccountClosures) Prepare(ctx context.Context, tx *sql.Tx, user uint64, operation string, capture, target, fingerprint, result []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)
		VALUES(?,?,'ACCOUNT_CLOSE',?,?,'PREPARING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, operation, user, string(target), fingerprint, string(result))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_close(operation_id,user_id,state,retention_plan,retry_at)
		VALUES(?,?,'PREPARING',?,UTC_TIMESTAMP(6))`, operation, user, string(capture))
	return err
}

func (r *AccountClosures) Bind(ctx context.Context, tx *sql.Tx, operation string, capture []byte) error {
	result, err := tx.ExecContext(ctx, "UPDATE account_close SET retention_plan=? WHERE operation_id=? AND state='PREPARING'", string(capture), operation)
	return accountCloseChanged(result, err)
}

func (r *AccountClosures) Reject(ctx context.Context, tx *sql.Tx, operation, reason string, result []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET state='REJECTED',last_error=?,retry_at=NULL
		WHERE operation_id=? AND state='PREPARING'`, reason, operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET state='REJECTED',result_json=?,terminal_at=UTC_TIMESTAMP(6)
		WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING'`, string(result), operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	record, err := r.Record(ctx, tx, operation)
	if err != nil {
		return err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return errors.New("ACCOUNT_CLOSE_BINDING_REQUIRED")
	}
	if capture.Binding == nil {
		// 未向Doc固定任何绑定时只有本地准备记录，拒绝即可终结，不登记虚构取消范围。
		return nil
	}
	payload, err := json.Marshal(capture.Binding)
	if err != nil {
		return err
	}
	// 拒绝证明与取消意图一起提交。进程退出或Doc回复丢失后仍有原号任务，
	// 不依赖用户再次点击，也不靠超时猜测下游预留是否存在。
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_CLOSE_CANCEL',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, operation, operation, string(payload))
	return err
}

func accountCloseChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("ACCOUNT_CLOSE_CHANGED")
	}
	return nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

type AccountCloseFences struct{ db *sql.DB }

func NewAccountCloseFences(db *sql.DB) *AccountCloseFences { return &AccountCloseFences{db: db} }

const accountCloseFenceTaskColumns = `t.id,t.generation,COALESCE(t.operation_id,''),t.business_key,t.state,
	COALESCE(t.instance_id,''),t.payload,COALESCE(t.lease_until>UTC_TIMESTAMP(6),FALSE)`

func scanAccountCloseFenceTask(row interface{ Scan(...any) error }) (domain.AccountCloseFenceTask, error) {
	var task domain.AccountCloseFenceTask
	err := row.Scan(&task.ID, &task.Generation, &task.Operation, &task.Revocation, &task.State, &task.Instance, &task.Payload, &task.LeaseValid)
	return task, err
}

func (r *AccountCloseFences) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	if instance == "" {
		return domain.AccountCloseFenceTask{}, errors.New("ACCOUNT_CLOSE_FENCE_INSTANCE_INVALID")
	}
	claimed, err := claimClosureTask(ctx, tx, []string{domain.AccountCloseFenceKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, claimed.ID, domain.AccountCloseFenceKind))
	if err != nil {
		return task, err
	}
	// 每次领取推进代次，旧worker即使复用实例编号也不能提交新租约的结果。
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
		WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, domain.AccountCloseFenceKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

type accountCloseFenceFacts struct {
	User, Actor, AuthorizationActor, Epoch, GateEpoch                                           uint64
	CloseState, ReceiptKind, ReceiptState, AuthorizationState, Scope, GateState, GateRevocation string
	Capture, Target, Participants                                                               []byte
}

const accountCloseFenceFactsColumns = `COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
	COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),COALESCE(a.actor_id,0),COALESCE(a.scope_key,''),COALESCE(a.epoch,0),
	COALESCE(a.state,''),a.target_permission,a.participant_plan,COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,'')`

const accountCloseFenceFactsJoins = ` LEFT JOIN account_close c ON c.operation_id=t.operation_id
	LEFT JOIN operation_receipt o ON o.operation_id=c.operation_id
	LEFT JOIN authorization_change a ON a.revocation_id=t.business_key
	LEFT JOIN access_gate g ON g.scope_key=a.scope_key `

func accountCloseFenceFactDestinations(f *accountCloseFenceFacts) []any {
	return []any{&f.User, &f.CloseState, &f.Capture, &f.Actor, &f.ReceiptKind, &f.ReceiptState, &f.AuthorizationActor, &f.Scope, &f.Epoch, &f.AuthorizationState, &f.Target, &f.Participants, &f.GateEpoch, &f.GateState, &f.GateRevocation}
}

func validateAccountCloseFence(task domain.AccountCloseFenceTask, facts accountCloseFenceFacts) (domain.AccountCloseFencePayload, error) {
	payload, err := domain.DecodeAccountCloseFence(task.Payload)
	if err != nil {
		return payload, err
	}
	invalid := errors.New("ACCOUNT_CLOSE_FENCE_BINDING_INVALID")
	if task.Operation != payload.Operation || task.Revocation != payload.Revocation || facts.User != payload.User || facts.Actor != payload.User || facts.AuthorizationActor != payload.User || facts.CloseState != "FROZEN" || facts.ReceiptKind != "ACCOUNT_CLOSE" || facts.ReceiptState != "PREPARING" || facts.AuthorizationState != "FENCING" || facts.Scope != payload.Scope || facts.Epoch != payload.Epoch || facts.GateEpoch != payload.Epoch || facts.GateState != "BLOCKED" || facts.GateRevocation != payload.Revocation {
		return payload, invalid
	}
	capture, err := domain.DecodeAccountCloseCapture(facts.Capture)
	if err != nil || capture.Binding == nil || capture.Binding.Operation != payload.Operation || capture.Binding.User != payload.User {
		return payload, invalid
	}
	matched := 0
	for _, scope := range capture.Scopes {
		if scope.Scope == payload.Scope || scope.Revocation == payload.Revocation {
			if scope.Scope != payload.Scope || scope.Revocation != payload.Revocation || scope.Epoch != payload.Epoch {
				return payload, invalid
			}
			matched++
		}
	}
	var target struct {
		Operation string `json:"account_close_operation"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}
	var participants []string
	if matched != 1 || json.Unmarshal(facts.Target, &target) != nil || target.Operation != payload.Operation || target.Scope != payload.Scope || target.Epoch != payload.Epoch || json.Unmarshal(facts.Participants, &participants) != nil || len(participants) != 1 || participants[0] != "document" {
		return payload, invalid
	}
	return payload, nil
}

func (r *AccountCloseFences) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (domain.AccountCloseFencePayload, error) {
	var facts accountCloseFenceFacts
	err := q.QueryRowContext(ctx, `SELECT `+accountCloseFenceFactsColumns+` FROM durable_task t `+accountCloseFenceFactsJoins+`
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business_key=? AND t.generation=? AND t.state='RUNNING' AND t.instance_id=? AND t.lease_until>UTC_TIMESTAMP(6)`, task.ID, domain.AccountCloseFenceKind, task.Operation, task.Revocation, task.Generation, task.Instance).
		Scan(accountCloseFenceFactDestinations(&facts)...)
	if err != nil {
		return domain.AccountCloseFencePayload{}, err
	}
	return validateAccountCloseFence(task, facts)
}

func (r *AccountCloseFences) Retry(ctx context.Context, task domain.AccountCloseFenceTask, code string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',lease_until=NULL,instance_id=NULL,
		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error=? WHERE id=? AND kind=? AND generation=?
		AND operation_id=? AND business_key=? AND instance_id=? AND state='RUNNING'`, code, task.ID, domain.AccountCloseFenceKind, task.Generation, task.Operation, task.Revocation, task.Instance)
	return err
}

// Finish锁序沿用user→receipt→account_close→task。RPC期间不持SQL锁，提交前重新核验
// 租约代次和固定绑定；这里只结束一个来源屏障，不结束账号注销或全局操作回执。
func (r *AccountCloseFences) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, payload domain.AccountCloseFencePayload, digest []byte) error {
	closures := NewAccountClosures(r.db)
	if _, err := closures.LockUser(ctx, tx, payload.User); err != nil {
		return err
	}
	if _, err := closures.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err := closures.Record(ctx, tx, task.Operation); err != nil {
		return err
	}
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE t.id=? AND t.kind=? FOR UPDATE`, task.ID, domain.AccountCloseFenceKind))
	if err != nil {
		return err
	}
	if current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || current.Operation != task.Operation || current.Revocation != task.Revocation || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_FENCE_OWNERSHIP_CHANGED")
	}
	actual, err := r.Binding(ctx, tx, current)
	if err != nil {
		return err
	}
	if actual != payload || len(digest) != 32 {
		return errors.New("ACCOUNT_CLOSE_FENCE_ACK_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at)
		VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE revocation_id=revocation_id`, payload.Revocation, payload.Scope, payload.Epoch, digest)
	if err != nil {
		return err
	}
	var existing []byte
	err = tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED' FOR UPDATE`, payload.Revocation, payload.Scope, payload.Epoch).Scan(&existing)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, digest) {
		return errors.New("ACCOUNT_CLOSE_FENCE_ACK_CONFLICT")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE authorization_change SET state='SUCCEEDED',last_error=NULL,retry_at=NULL
		WHERE revocation_id=? AND actor_id=? AND scope_key=? AND epoch=? AND state='FENCING'`, payload.Revocation, payload.User, payload.Scope, payload.Epoch)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,instance_id=NULL,last_error=NULL
		WHERE id=? AND kind=? AND operation_id=? AND business_key=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, task.ID, domain.AccountCloseFenceKind, task.Operation, task.Revocation, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

// InvalidTasks只分页联查本worker接管的未完成任务，既不放过绑定损坏，也不逐行查询旧任务队列。
func (r *AccountCloseFences) InvalidTasks(ctx context.Context) (bool, error) {
	after := uint64(0)
	for {
		rows, err := r.db.QueryContext(ctx, `SELECT `+accountCloseFenceTaskColumns+`,`+accountCloseFenceFactsColumns+`
			FROM durable_task t `+accountCloseFenceFactsJoins+` WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, domain.AccountCloseFenceKind, after)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var task domain.AccountCloseFenceTask
			var facts accountCloseFenceFacts
			destinations := []any{&task.ID, &task.Generation, &task.Operation, &task.Revocation, &task.State, &task.Instance, &task.Payload, &task.LeaseValid}
			destinations = append(destinations, accountCloseFenceFactDestinations(&facts)...)
			if err = rows.Scan(destinations...); err != nil {
				rows.Close()
				return false, err
			}
			if task.State != "QUEUED" && task.State != "RETRY" && task.State != "UNKNOWN" && task.State != "RUNNING" {
				rows.Close()
				return true, nil
			}
			if _, err = validateAccountCloseFence(task, facts); err != nil {
				rows.Close()
				return true, nil
			}
			after, count = task.ID, count+1
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count < 100 {
			return false, nil
		}
	}
}

// account_close_freeze.go：保留原实现，按业务合并文件。

// SavePreparation允许在保持PREPARING的前提下持久化交接要求或下游固定计划。
// 代次每次推进让相同JSON重存仍有明确CAS结果，不把MySQL的零变更误判为不存在。
func (r *AccountClosures) SavePreparation(ctx context.Context, tx *sql.Tx, operation string, captureJSON, resultJSON []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET retention_plan=?,generation=generation+1
		WHERE operation_id=? AND state='PREPARING' AND generation<18446744073709551615`, string(captureJSON), operation)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
		WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(resultJSON), operation)
	return accountCloseChanged(changed, err)
}

// Freeze必须由已复核凭据/组织快照、且已确认Doc同一固定计划FROZEN的用例调用。
// 全部来源冻结、通知下游的持久任务与状态推进使用同一事务；不在此宣称注销完成。
func (r *AccountClosures) Freeze(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture, result []byte) error {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='REVOKING',identity_epoch=identity_epoch+1
		WHERE id=? AND state='ACTIVE' AND identity_epoch=? AND credential_epoch=? AND identity_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch, capture.CredentialEpoch)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	if err = freezeAccountCloseSessions(ctx, tx, binding.User); err != nil {
		return err
	}
	closing := map[uint64]bool{}
	for _, id := range binding.ClosingOrganizations {
		closing[id] = true
	}
	for _, org := range capture.Organizations {
		if org.MemberState != "ACTIVE" {
			return errors.New("ACCOUNT_CLOSE_MEMBERSHIP_CHANGED")
		}
		changed, err = tx.ExecContext(ctx, `UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1
			WHERE organization_id=? AND user_id=? AND role=? AND state='ACTIVE' AND epoch=? AND revision=?
			AND epoch<18446744073709551615 AND revision<18446744073709551615`, org.ID, binding.User, org.Role, org.Epoch, org.MembershipRevision)
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
		if closing[org.ID] {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=?
				AND revision<18446744073709551615 AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		} else {
			changed, err = tx.ExecContext(ctx, `UPDATE organization SET member_revision=member_revision+1
				WHERE id=? AND owner_id=? AND lifecycle=? AND revision=? AND member_revision=? AND member_revision<18446744073709551615`, org.ID, org.Owner, org.Lifecycle, org.Revision, org.MemberRevision)
		}
		if err = accountCloseChanged(changed, err); err != nil {
			return err
		}
	}
	for _, scope := range capture.Scopes {
		if err = freezeAccountCloseScope(ctx, tx, binding, scope); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE account_close SET state='FROZEN',retention_plan=?,generation=generation+1,retry_at=UTC_TIMESTAMP(6)
		WHERE operation_id=? AND user_id=? AND state='PREPARING' AND generation<18446744073709551615`, string(raw), binding.Operation, binding.User)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
		WHERE operation_id=? AND actor_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(result), binding.Operation, binding.User)
	return accountCloseChanged(changed, err)
}

func validateAccountCloseFreeze(binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error {
	invalid := errors.New("ACCOUNT_CLOSE_FREEZE_BINDING_INVALID")
	if !platform.ValidateAccountClosureBinding(binding) || capture.Binding == nil || !platform.EqualAccountClosureBinding(*capture.Binding, binding) || capture.IdentityEpoch != binding.IdentityEpoch || capture.CredentialEpoch == 0 {
		return invalid
	}
	user := strconv.FormatUint(binding.User, 10)
	expected := map[string]uint64{"identity:" + user: binding.IdentityEpoch}
	organizations := map[uint64]bool{}
	previous := uint64(0)
	for _, org := range capture.Organizations {
		if org.ID <= previous || org.Epoch == 0 || org.Revision == 0 || org.MemberRevision == 0 || org.MembershipRevision == 0 || org.MemberState != "ACTIVE" || (org.Lifecycle != "ACTIVE" && org.Lifecycle != "CLOSING") {
			return invalid
		}
		previous = org.ID
		organizations[org.ID] = true
		expected["org-member:"+strconv.FormatUint(org.ID, 10)+":"+user] = org.Epoch
	}
	for _, id := range binding.ClosingOrganizations {
		if !organizations[id] {
			return invalid
		}
	}
	if len(capture.Scopes) != len(expected) {
		return invalid
	}
	revocations := map[string]bool{}
	for _, scope := range capture.Scopes {
		if !domain.ValidOperationID(scope.Revocation) || revocations[scope.Revocation] || expected[scope.Scope] == 0 || expected[scope.Scope] != scope.Epoch {
			return invalid
		}
		if scope.Scope == "identity:"+user && scope.Revocation != binding.Operation {
			return invalid
		}
		revocations[scope.Revocation] = true
		delete(expected, scope.Scope)
	}
	return nil
}

func freezeAccountCloseSessions(ctx context.Context, tx *sql.Tx, user uint64) error {
	rows, err := tx.QueryContext(ctx, "SELECT epoch FROM session WHERE user_id=? AND state='ACTIVE' ORDER BY id FOR UPDATE", user)
	if err != nil {
		return err
	}
	count := int64(0)
	for rows.Next() {
		var epoch uint64
		if err = rows.Scan(&epoch); err != nil {
			rows.Close()
			return err
		}
		if epoch == ^uint64(0) {
			rows.Close()
			return errors.New("ACCOUNT_CLOSE_SESSION_EPOCH_EXHAUSTED")
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, "UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE user_id=? AND state='ACTIVE' AND epoch<18446744073709551615", user)
	if err != nil {
		return err
	}
	actual, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if actual != count {
		return errors.New("ACCOUNT_CLOSE_SESSION_CHANGED")
	}
	return nil
}

func freezeAccountCloseScope(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, scope domain.AccountCloseScope) error {
	// 先建立门禁占位再当前读核对，绝不盲目覆盖另一个相同代次的冻结操作。
	_, err := tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN')
		ON DUPLICATE KEY UPDATE scope_key=scope_key`, scope.Scope, scope.Epoch)
	if err != nil {
		return err
	}
	var epoch uint64
	var state, revocation string
	err = tx.QueryRowContext(ctx, "SELECT max_epoch,state,COALESCE(last_revocation_id,'') FROM access_gate WHERE scope_key=? FOR UPDATE", scope.Scope).Scan(&epoch, &state, &revocation)
	if err != nil {
		return err
	}
	if epoch > scope.Epoch || (epoch == scope.Epoch && state == "BLOCKED" && revocation != scope.Revocation) {
		return errors.New("ACCOUNT_CLOSE_SOURCE_CHANGED")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1
		WHERE scope_key=? AND revision<18446744073709551615`, scope.Epoch, scope.Revocation, scope.Scope)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	// 反向来源覆盖作为操作者和作为目标人的证明；目标身份来源的登记由准入路径负责。
	_, err = tx.ExecContext(ctx, `UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id
		SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=? AND i.state='OPEN'`, scope.Scope, scope.Epoch)
	if err != nil {
		return err
	}
	target, err := json.Marshal(map[string]any{"account_close_operation": binding.Operation, "scope_key": scope.Scope, "epoch": scope.Epoch})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
		VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, scope.Revocation, binding.User, scope.Scope, scope.Epoch, string(target))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"operation_id": binding.Operation, "user_id": binding.User, "revocation_id": scope.Revocation, "scope_key": scope.Scope, "epoch": scope.Epoch})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_CLOSE_FENCE',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, scope.Revocation, binding.Operation, string(payload))
	return err
}

// account_close_organization.go：保留原实现，按业务合并文件。

// UserForOperation仅用于找到串行锁的用户编号；调用方持user锁后必须重新读取原关闭记录。
func (r *AccountClosures) UserForOperation(ctx context.Context, operation string) (uint64, error) {
	var user uint64
	err := r.db.QueryRowContext(ctx, "SELECT user_id FROM account_close WHERE operation_id=?", operation).Scan(&user)
	return user, err
}

// LockOrganizations由已持有user锁的调用方使用。发现阶段不锁成员行，随后逐个
// 按组织编号取组织锁，避免两个成员分别持有自身成员行后再交叉等待组织锁。
func (r *AccountClosures) LockOrganizations(ctx context.Context, tx *sql.Tx, user uint64) ([]domain.AccountCloseOrganization, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM organization WHERE lifecycle IN ('ACTIVE','CLOSING') AND
		(owner_id=? OR id IN (SELECT organization_id FROM organization_member WHERE user_id=? AND state IN ('ACTIVE','REVOKING')))
		ORDER BY id`, user, user)
	if err != nil {
		return nil, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	organizations := []domain.AccountCloseOrganization{}
	for _, id := range ids {
		org := domain.AccountCloseOrganization{ID: id}
		err = tx.QueryRowContext(ctx, `SELECT owner_id,revision,member_revision,lifecycle FROM organization WHERE id=? FOR UPDATE`, id).
			Scan(&org.Owner, &org.Revision, &org.MemberRevision, &org.Lifecycle)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if org.Lifecycle != "ACTIVE" && org.Lifecycle != "CLOSING" {
			continue
		}
		// 当前锁定读不受发现阶段RR快照影响，等待组织锁后必须看到刚提交的角色与成员状态。
		rows, err = tx.QueryContext(ctx, `SELECT user_id,role,state,epoch,revision FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var member, epoch, revision uint64
			var role, state string
			if err = rows.Scan(&member, &role, &state, &epoch, &revision); err != nil {
				rows.Close()
				return nil, err
			}
			if state == "ACTIVE" || state == "REVOKING" {
				org.LiveMembers++
			}
			if state == "ACTIVE" && (role == "ADMIN" || member == org.Owner) {
				org.AvailableAdministrators++
			}
			if member == user {
				org.Role, org.MemberState, org.Epoch, org.MembershipRevision = role, state, epoch, revision
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if org.Owner == user || org.MemberState == "ACTIVE" || org.MemberState == "REVOKING" {
			organizations = append(organizations, org)
		}
	}
	return organizations, nil
}
````

UTF-8/LF全文SHA-256：`33dd6f6b45c3fe6d1a86ec263ac556cdbe41f7fed724e6fd83e3db5d8e3c3ed3`。[参考实现](../../../../services/account/internal/repository/account/account_close_preparation_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
