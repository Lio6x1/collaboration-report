// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
)

// account_close_cancel.go：保留原实现，按业务合并文件。

const AccountCloseCancelKind = "ACCOUNT_CLOSE_CANCEL"

type AccountCloseCancellations struct{ db *sql.DB }

func NewAccountCloseCancellations(db *sql.DB) *AccountCloseCancellations {
	return &AccountCloseCancellations{db: db}
}

func (r *AccountCloseCancellations) Claim(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{AccountCloseCancelKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? FOR UPDATE`, claimed.ID))
	if err != nil || instance == "" {
		return task, errors.New("ACCOUNT_CLOSE_CANCEL_CLAIM_INVALID")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
		lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
		WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, AccountCloseCancelKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

func cancellationBinding(task domain.AccountCloseFenceTask, user uint64, state, receiptKind, receiptState string, actor uint64, rawCapture []byte) (platform.AccountClosureBinding, error) {
	var binding platform.AccountClosureBinding
	decoder := json.NewDecoder(bytes.NewReader(task.Payload))
	decoder.DisallowUnknownFields()
	invalid := errors.New("ACCOUNT_CLOSE_CANCEL_BINDING_INVALID")
	if decoder.Decode(&binding) != nil || decoder.Decode(new(any)) != io.EOF || !platform.ValidateAccountClosureBinding(binding) {
		return binding, invalid
	}
	capture, err := domain.DecodeAccountCloseCapture(rawCapture)
	if err != nil || capture.Binding == nil || !platform.EqualAccountClosureBinding(*capture.Binding, binding) || binding.Operation != task.Operation || task.Revocation != task.Operation || binding.User != user || actor != user || state != "REJECTED" || receiptKind != "ACCOUNT_CLOSE" || receiptState != "REJECTED" {
		return binding, invalid
	}
	return binding, nil
}

func (r *AccountCloseCancellations) Binding(ctx context.Context, q Queryer, task domain.AccountCloseFenceTask) (platform.AccountClosureBinding, error) {
	var user, actor uint64
	var state, kind, receiptState string
	var capture []byte
	err := q.QueryRowContext(ctx, `SELECT c.user_id,c.state,c.retention_plan,o.actor_id,o.kind,o.state
		FROM durable_task t JOIN account_close c ON c.operation_id=t.operation_id
		JOIN operation_receipt o ON o.operation_id=c.operation_id
		WHERE t.id=? AND t.kind=? AND t.operation_id=? AND t.business_key=? AND t.generation=?
		AND t.state='RUNNING' AND t.instance_id=? AND t.lease_until>UTC_TIMESTAMP(6)`, task.ID, AccountCloseCancelKind, task.Operation, task.Revocation, task.Generation, task.Instance).
		Scan(&user, &state, &capture, &actor, &kind, &receiptState)
	if err != nil {
		return platform.AccountClosureBinding{}, err
	}
	return cancellationBinding(task, user, state, kind, receiptState, actor, capture)
}

func (r *AccountCloseCancellations) Retry(ctx context.Context, task domain.AccountCloseFenceTask) {
	_, _ = r.db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,
		retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),last_error='ACCOUNT_CLOSE_CANCEL_UNCONFIRMED'
		WHERE id=? AND kind=? AND operation_id=? AND generation=? AND instance_id=? AND state='RUNNING'`, task.ID, AccountCloseCancelKind, task.Operation, task.Generation, task.Instance)
}

func (r *AccountCloseCancellations) Finish(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, binding platform.AccountClosureBinding) error {
	closures := NewAccountClosures(r.db)
	if _, err := closures.LockUser(ctx, tx, binding.User); err != nil {
		return err
	}
	if _, err := closures.Receipt(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if _, err := closures.Record(ctx, tx, binding.Operation); err != nil {
		return err
	}
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseCancelKind))
	if err != nil {
		return err
	}
	if current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_CANCEL_OWNERSHIP_CHANGED")
	}
	actual, err := r.Binding(ctx, tx, current)
	if err != nil {
		return err
	}
	if !platform.EqualAccountClosureBinding(actual, binding) {
		return errors.New("ACCOUNT_CLOSE_CANCEL_BINDING_CHANGED")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='SUCCEEDED',instance_id=NULL,lease_until=NULL,last_error=NULL
		WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, task.ID, AccountCloseCancelKind, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

func (r *AccountCloseCancellations) Ready(ctx context.Context) error {
	after := uint64(0)
	for {
		rows, err := r.db.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,''),t.business_key,t.payload,
			COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,'')
			FROM durable_task t LEFT JOIN account_close c ON c.operation_id=t.operation_id LEFT JOIN operation_receipt o ON o.operation_id=c.operation_id
			WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, AccountCloseCancelKind, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var task domain.AccountCloseFenceTask
			var user, actor uint64
			var state, kind, receiptState string
			var capture []byte
			err = rows.Scan(&task.ID, &task.Operation, &task.Revocation, &task.Payload, &user, &state, &capture, &actor, &kind, &receiptState)
			if err == nil {
				_, err = cancellationBinding(task, user, state, kind, receiptState, actor, capture)
			}
			if err != nil {
				rows.Close()
				return err
			}
			after, count = task.ID, count+1
		}
		err = rows.Err()
		rows.Close()
		if err != nil || count < 100 {
			return err
		}
	}
}

// account_close_confirmation.go：保留原实现，按业务合并文件。

// 用户锁先于本方法：屏障完成事务也锁同一用户，确认读取不会把不同提交拼成ACK。
// 每页仅按种类和精确business_key访问本计划的任务，避免扫描历史队列。
func (r *AccountClosures) ConfirmedScopes(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) (bool, error) {
	if err := validateAccountCloseFreeze(binding, capture); err != nil {
		return false, err
	}
	expected := make(map[string]domain.AccountCloseScope, len(capture.Scopes))
	for _, source := range capture.Scopes {
		expected[source.Revocation] = source
	}
	for offset := 0; offset < len(capture.Scopes); offset += 100 {
		end := offset + 100
		if end > len(capture.Scopes) {
			end = len(capture.Scopes)
		}
		args := []any{domain.AccountCloseFenceKind}
		for _, source := range capture.Scopes[offset:end] {
			args = append(args, source.Revocation)
		}
		rows, err := tx.QueryContext(ctx, `SELECT COALESCE(t.operation_id,''),t.business_key,t.payload,t.state,
			COALESCE(a.actor_id,0),COALESCE(a.scope_key,''),COALESCE(a.epoch,0),COALESCE(a.state,''),a.target_permission,a.participant_plan,
			COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,''),ack.result_digest,
			COALESCE(ack.phase,'')
			FROM durable_task t LEFT JOIN authorization_change a ON a.revocation_id=t.business_key
			LEFT JOIN access_gate g ON g.scope_key=a.scope_key
			LEFT JOIN authorization_ack ack ON ack.revocation_id=a.revocation_id AND ack.participant='document' AND ack.scope_key=a.scope_key AND ack.epoch=a.epoch
			WHERE t.kind=? AND t.business_key IN (`+strings.TrimSuffix(strings.Repeat("?,", end-offset), ",")+`)`, args...)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var operation, revocation, taskState, scope, authorizationState, gateState, gateRevocation, phase string
			var actor, epoch, gateEpoch uint64
			var payload, target, participants, digest []byte
			err = rows.Scan(&operation, &revocation, &payload, &taskState, &actor, &scope, &epoch, &authorizationState, &target, &participants, &gateEpoch, &gateState, &gateRevocation, &digest, &phase)
			if err != nil {
				rows.Close()
				return false, err
			}
			source, found := expected[revocation]
			parsed, decodeErr := domain.DecodeAccountCloseFence(payload)
			if !found || decodeErr != nil || operation != binding.Operation || parsed.Operation != binding.Operation || parsed.User != binding.User || parsed.Revocation != revocation || parsed.Scope != source.Scope || parsed.Epoch != source.Epoch || actor != binding.User || scope != source.Scope || epoch != source.Epoch || gateEpoch != epoch || gateState != "BLOCKED" || gateRevocation != revocation {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
			var targetBinding struct {
				Operation string `json:"account_close_operation"`
				Scope     string `json:"scope_key"`
				Epoch     uint64 `json:"epoch"`
			}
			var participantPlan []string
			if json.Unmarshal(target, &targetBinding) != nil || targetBinding.Operation != operation || targetBinding.Scope != scope || targetBinding.Epoch != epoch || json.Unmarshal(participants, &participantPlan) != nil || len(participantPlan) != 1 || participantPlan[0] != "document" {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_BINDING_INVALID")
			}
			if taskState != "SUCCEEDED" || authorizationState != "SUCCEEDED" {
				rows.Close()
				return false, nil
			}
			request := &pb.FenceRequest{RevocationId: revocation, Generation: 1, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: epoch}}, TargetDigest: platform.SourceClosureDigest(scope, epoch)}
			if phase != "OUTPUT_STOPPED" || !bytes.Equal(digest, platform.BootstrapDigest(request, "document")) {
				rows.Close()
				return false, errors.New("ACCOUNT_CLOSE_CONFIRMATION_ACK_INVALID")
			}
			delete(expected, revocation)
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if count != end-offset {
			return false, nil
		}
	}
	return len(expected) == 0, nil
}

// account_close_finalize.go：保留原实现，按业务合并文件。

// 调用方已持原user/receipt/account_close锁且核对Doc完成证明和全部原ACK。
// 组织按原编号锁定，存续组织只关闭本人关系，不删除其他人的内容或分配负责人。
func FinalizeAccountCloseData(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, capture domain.AccountCloseCapture) error {
	for _, organization := range capture.Organizations {
		var owner, memberRevision uint64
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT owner_id,lifecycle,member_revision FROM organization WHERE id=? FOR UPDATE", organization.ID).Scan(&owner, &state, &memberRevision); err != nil {
			return err
		}
		closing := slices.Contains(binding.ClosingOrganizations, organization.ID)
		if memberRevision == math.MaxUint64 || (closing && (state != "CLOSING" || owner != binding.User)) || (!closing && (state != "ACTIVE" || owner == binding.User)) {
			return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_CHANGED")
		}
		var epoch uint64
		var membership string
		if err := tx.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization.ID, binding.User).Scan(&membership, &epoch); err != nil {
			return err
		}
		if organization.Epoch == math.MaxUint64 || membership != "REVOKING" || epoch != organization.Epoch+1 {
			return errors.New("ACCOUNT_CLOSE_FINAL_MEMBERSHIP_CHANGED")
		}
		if closing {
			var others bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_member WHERE organization_id=? AND user_id<>? AND state<>'CLOSED')", organization.ID, binding.User).Scan(&others); err != nil {
				return err
			}
			if others {
				return errors.New("ACCOUNT_CLOSE_FINAL_ORGANIZATION_NOT_EMPTY")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE organization SET name='已关闭组织',lifecycle='CLOSED',closed_at=UTC_TIMESTAMP(6),member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, "UPDATE organization SET member_revision=member_revision+1 WHERE id=?", organization.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=?", organization.ID, binding.User); err != nil {
			return err
		}
		var saturated bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision=18446744073709551615)
		 OR EXISTS(SELECT 1 FROM organization_invitation WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision=18446744073709551615)`, organization.ID, binding.User, organization.ID, binding.User).Scan(&saturated); err != nil {
			return err
		}
		if saturated {
			return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND user_id=? AND state='ACTIVE' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE organization_invitation SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND inviter_id=? AND state='OPEN' AND revision<18446744073709551615", organization.ID, binding.User); err != nil {
			return err
		}
	}
	var active bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM session WHERE user_id=? AND state='ACTIVE')", binding.User).Scan(&active); err != nil {
		return err
	}
	if active {
		return errors.New("ACCOUNT_CLOSE_FINAL_SESSION_NOT_FROZEN")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE session SET state='CLOSED' WHERE user_id=? AND state='REVOKING'", binding.User); err != nil {
		return err
	}
	var saturated bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organization_application WHERE applicant_id=? AND state='PENDING' AND revision=18446744073709551615)", binding.User).Scan(&saturated); err != nil {
		return err
	}
	if saturated {
		return errors.New("ACCOUNT_CLOSE_FINAL_REVISION_EXHAUSTED")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE organization_application SET state='CANCELLED',revision=revision+1 WHERE applicant_id=? AND state='PENDING' AND revision<18446744073709551615", binding.User); err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE user SET state='CLOSED',email=NULL,email_key=NULL,password_hash=NULL,password_algorithm=NULL,
	 display_name='已注销用户',email_verified_at=NULL,closed_at=UTC_TIMESTAMP(6),credential_epoch=credential_epoch+1
	 WHERE id=? AND state='REVOKING' AND identity_epoch=? AND credential_epoch=? AND credential_epoch<18446744073709551615`, binding.User, binding.IdentityEpoch+1, capture.CredentialEpoch)
	return accountCloseChanged(changed, err)
}

func CompleteAccountCloseReceipt(ctx context.Context, tx *sql.Tx, binding platform.AccountClosureBinding, digest string, plan []byte) error {
	changed, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6) WHERE operation_id=? AND actor_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING'", string(plan), binding.Operation, binding.User)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	target, err := json.Marshal(map[string]any{"user_id": binding.User, "document_plan_digest": digest})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(NULL,?,1,'ACCOUNT_CLOSE_COMPLETE',?)", binding.Operation, string(target))
	return err
}

// account_close_progress.go：保留原实现，按业务合并文件。

// 这里只取原会话绑定；对外进度仍由用例在user→receipt→account_close锁内重检。
func (r *AccountClosures) ProgressBinding(ctx context.Context, operation string) (domain.AccountCloseReceipt, error) {
	var receipt domain.AccountCloseReceipt
	err := r.db.QueryRowContext(ctx, `SELECT actor_id,kind,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),'')
	 FROM operation_receipt WHERE operation_id=?`, operation).Scan(&receipt.User, &receipt.Kind, &receipt.SessionHash)
	return receipt, err
}

// account_close_recovery.go：保留原实现，按业务合并文件。

const AccountCloseRecoveryKind = "ACCOUNT_CLOSE_RECOVER"

type AccountCloseRecoveryPayload struct {
	Operation string `json:"operation_id"`
	User      uint64 `json:"user_id"`
}

func QueueAccountCloseRecovery(ctx context.Context, tx *sql.Tx, operation string, user uint64) error {
	raw, err := json.Marshal(AccountCloseRecoveryPayload{Operation: operation, User: user})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)`, AccountCloseRecoveryKind, operation, operation, string(raw))
	return err
}

func ClaimAccountCloseRecovery(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{AccountCloseRecoveryKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, claimed.ID, AccountCloseRecoveryKind))
	if err != nil || instance == "" {
		return task, errors.New("ACCOUNT_CLOSE_RECOVERY_CLAIM_INVALID")
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
	 WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, AccountCloseRecoveryKind, task.Generation)
	if err = accountCloseChanged(changed, err); err != nil {
		return task, err
	}
	task.Generation++
	task.Instance, task.State, task.LeaseValid = instance, "RUNNING", true
	return task, nil
}

// 只在调用方user→receipt→account_close锁之后核对任务租约。网络结果不能
// 允许已过租约的旧领取者推进本地游标；下一领取者仍按原Doc回执重放。
func ValidateAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error {
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, `SELECT `+accountCloseFenceTaskColumns+` FROM durable_task t WHERE id=? AND kind=? FOR UPDATE`, task.ID, AccountCloseRecoveryKind))
	if err != nil {
		return err
	}
	if current.Operation != task.Operation || current.Revocation != task.Operation || current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_OWNERSHIP_CHANGED")
	}
	return nil
}

func FinishAccountCloseRecovery(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error {
	state := "QUEUED"
	if terminal {
		state = "SUCCEEDED"
	}
	changed, err := tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, state, task.ID, AccountCloseRecoveryKind, task.Generation, task.Instance)
	return accountCloseChanged(changed, err)
}

func RetryAccountCloseRecovery(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string) error {
	_, err := db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING'`, code, task.ID, AccountCloseRecoveryKind, task.Generation, task.Instance)
	return err
}

func SaveAccountCloseRetention(ctx context.Context, tx *sql.Tx, operation, oldState, newState string, capture, result []byte) error {
	changed, err := tx.ExecContext(ctx, `UPDATE account_close SET state=?,retention_plan=?,generation=generation+1
	 WHERE operation_id=? AND state=? AND generation<18446744073709551615`, newState, string(capture), operation, oldState)
	if err = accountCloseChanged(changed, err); err != nil {
		return err
	}
	changed, err = tx.ExecContext(ctx, `UPDATE operation_receipt SET result_json=?,generation=generation+1
	 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PREPARING' AND generation<18446744073709551615`, string(result), operation)
	return accountCloseChanged(changed, err)
}

// RecoveryPlanRow 保留孤立任务的 LEFT JOIN 结果，让业务校验拒绝损坏绑定。
type RecoveryPlanRow struct {
	Task    domain.AccountCloseFenceTask
	Record  domain.AccountCloseRecord
	Receipt domain.AccountCloseReceipt
}

func AccountCloseRecoveryPage(ctx context.Context, q *sql.DB, after uint64) ([]RecoveryPlanRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.id,COALESCE(t.operation_id,''),t.business_key,t.payload,
		 COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
		 COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),
		 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.session_hash')),''),o.result_json
		 FROM durable_task t LEFT JOIN account_close c ON c.operation_id=t.operation_id
		 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		 WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, AccountCloseRecoveryKind, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RecoveryPlanRow, 0, 100)
	for rows.Next() {
		var row RecoveryPlanRow
		task, record, receipt := &row.Task, &row.Record, &row.Receipt
		if err := rows.Scan(&task.ID, &task.Operation, &task.Revocation, &task.Payload, &record.User, &record.State, &record.Capture, &receipt.User, &receipt.Kind, &receipt.State, &receipt.SessionHash, &receipt.Result); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
