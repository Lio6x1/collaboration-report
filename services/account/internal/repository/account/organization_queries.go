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
	"slices"
	"strconv"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

type Applications struct{ db *sql.DB }

func NewApplications(db *sql.DB) *Applications { return &Applications{db: db} }

func (r *Applications) Pending(ctx context.Context, tx *sql.Tx, organization, actor string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(id AS CHAR) FROM organization_application
		WHERE organization_id=? AND applicant_id=? AND state='PENDING'
		ORDER BY id LIMIT 1 FOR UPDATE`, organization, actor).Scan(&id)
	return id, err
}

func (r *Applications) Insert(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (int64, error) {
	var result sql.Result
	var err error
	if invitation == "" {
		result, err = tx.ExecContext(ctx, `
			INSERT INTO organization_application(organization_id,applicant_id,reason,state)
			VALUES(?,?,?,'PENDING')`, organization, actor, reason)
	} else {
		result, err = tx.ExecContext(ctx, `
			INSERT INTO organization_application
			(organization_id,applicant_id,source_invitation_id,invitation_valid_at_submit,invitation_validated_at,reason,state)
			VALUES(?,?,?,TRUE,UTC_TIMESTAMP(6),?,'PENDING')`, organization, actor, invitation, reason)
	}
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func (r *Applications) Identity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error) {
	var application domain.OrganizationApplication
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR)
		FROM organization_application WHERE id=?`, id).Scan(&application.OrganizationID, &application.ApplicantID)
	return application, err
}

func (r *Applications) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationApplication, error) {
	var application domain.OrganizationApplication
	err := tx.QueryRowContext(ctx, `
		SELECT state,CAST(revision AS CHAR) FROM organization_application
		WHERE id=? FOR UPDATE`, id).Scan(&application.State, &application.Revision)
	return application, err
}

func (r *Applications) AdmitMember(ctx context.Context, tx *sql.Tx, organization, applicant string) error {
	return execute(ctx, tx, []statement{
		{`INSERT INTO organization_member(organization_id,user_id,role,state)
		  VALUES(?,?,'MEMBER','ACTIVE')
		  ON DUPLICATE KEY UPDATE state='ACTIVE',role='MEMBER',epoch=epoch+1,revision=revision+1`, []any{organization, applicant}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}

func (r *Applications) Decide(ctx context.Context, tx *sql.Tx, id, actor, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_application SET state=?,reviewer_id=?,revision=revision+1 WHERE id=?", state, actor, id)
	return err
}

func (r *Applications) List(ctx context.Context, actor, organization, last string, limit int) ([]domain.OrganizationApplication, error) {
	condition, key := "applicant_id=?", actor
	if organization != "" {
		condition, key = "organization_id=?", organization
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR),CAST(applicant_id AS CHAR),
		       reason,state,CAST(revision AS CHAR),CAST(source_invitation_id AS CHAR),invitation_validated_at
		FROM organization_application WHERE `+condition+" AND id>? ORDER BY id LIMIT ?", key, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationApplication{}
	for rows.Next() {
		var application domain.OrganizationApplication
		var invitation sql.NullString
		var validated sql.NullTime
		if rows.Scan(&application.ID, &application.OrganizationID, &application.ApplicantID, &application.Reason, &application.State, &application.Revision, &invitation, &validated) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		if invitation.Valid {
			application.SourceInvitationID = &invitation.String
		}
		if validated.Valid {
			application.InvitationValidatedAt = &validated.Time
		}
		values = append(values, application)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// invitation.go：保留原实现，按业务合并文件。

type Invitations struct{ db *sql.DB }

func NewInvitations(db *sql.DB) *Invitations { return &Invitations{db: db} }

func (r *Invitations) Insert(ctx context.Context, tx *sql.Tx, organization, actor string, hash, encrypted []byte, expires time.Time) (int64, error) {
	row, err := tx.ExecContext(ctx, `
		INSERT INTO organization_invitation
		(organization_id,inviter_id,token_hash,token_ciphertext,token_key_id,state,expires_at)
		VALUES(?,?,?,?,'dev-invitation-v1','OPEN',?)`, organization, actor, hash, encrypted, expires)
	if err != nil {
		return 0, err
	}
	id, _ := row.LastInsertId()
	return id, nil
}

func (r *Invitations) Share(ctx context.Context, id, organization string) ([]byte, time.Time, error) {
	var encrypted []byte
	var expires time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT token_ciphertext, expires_at FROM organization_invitation
		WHERE id=? AND organization_id=?`, id, organization).Scan(&encrypted, &expires)
	return encrypted, expires, err
}

func (r *Invitations) Organization(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var organization string
	err := tx.QueryRowContext(ctx, "SELECT CAST(organization_id AS CHAR) FROM organization_invitation WHERE id=?", id).Scan(&organization)
	return organization, err
}

func (r *Invitations) LockStatus(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error) {
	var value domain.OrganizationInvitation
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(revision AS CHAR) FROM organization_invitation WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Revision)
	return value, err
}

func (r *Invitations) Close(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization_invitation SET state='CLOSED',revision=revision+1 WHERE id=?", id)
	return err
}

func (r *Invitations) IdentityByToken(ctx context.Context, tx *sql.Tx, hash []byte) (domain.InvitationIdentity, error) {
	var value domain.InvitationIdentity
	err := tx.QueryRowContext(ctx, `
		SELECT CAST(id AS CHAR),CAST(organization_id AS CHAR)
		FROM organization_invitation WHERE token_hash=?`, hash).Scan(&value.ID, &value.OrganizationID)
	return value, err
}

func (r *Invitations) LockValidity(ctx context.Context, tx *sql.Tx, id string) (domain.OrganizationInvitation, error) {
	var value domain.OrganizationInvitation
	err := tx.QueryRowContext(ctx, "SELECT state,expires_at FROM organization_invitation WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Expires)
	return value, err
}

func (r *Invitations) List(ctx context.Context, organization, last string, limit int) ([]domain.OrganizationInvitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT CAST(id AS CHAR),state,expires_at,CAST(revision AS CHAR)
		FROM organization_invitation WHERE organization_id=? AND id>?
		ORDER BY id LIMIT ?`, organization, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationInvitation{}
	for rows.Next() {
		value := domain.OrganizationInvitation{OrganizationID: organization}
		if rows.Scan(&value.ID, &value.State, &value.Expires, &value.Revision) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// organization_close.go：保留原实现，按业务合并文件。

const OrganizationCloseRecoveryKind = "ORG_CLOSE_RECOVER"

type OrganizationCloseRecord struct {
	Organization, Actor uint64
	State               string
	Plan                []byte
}

func ReadOrganizationClose(ctx context.Context, q Queryer, operation string, lock bool) (OrganizationCloseRecord, error) {
	var record OrganizationCloseRecord
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT organization_id,actor_id,state,fixed_plan FROM organization_transition WHERE operation_id=? AND kind='CLOSE'"+suffix, operation).Scan(&record.Organization, &record.Actor, &record.State, &record.Plan)
	return record, err
}

// Begin 在已持 user/session/receipt 锁后锁组织及全部成员；整个关闭意图同事务提交。
func BeginOrganizationClose(ctx context.Context, tx *sql.Tx, operation, organization, actor, expected string) (domain.OrganizationClosePlan, error) {
	var empty domain.OrganizationClosePlan
	org, err := ReadOrganization(ctx, tx, organization, true)
	if err != nil {
		return empty, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE", organization)
	if err != nil {
		return empty, err
	}
	live := 0
	var own domain.OrganizationMember
	for rows.Next() {
		var member domain.OrganizationMember
		if err = rows.Scan(&member.UserID, &member.Role, &member.State, &member.Epoch); err != nil {
			break
		}
		if member.State != "CLOSED" {
			live++
		}
		if member.UserID == actor {
			own = member
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return empty, err
	}
	if err = domain.OrganizationCloseEligible(org, actor, expected, own, live); err != nil {
		return empty, err
	}
	orgID, _ := strconv.ParseUint(organization, 10, 64)
	user, _ := strconv.ParseUint(actor, 10, 64)
	revision, _ := strconv.ParseUint(org.Revision, 10, 64)
	members, _ := strconv.ParseUint(org.MemberRevision, 10, 64)
	plan := domain.OrganizationClosePlan{Binding: platform.OrganizationClosureBinding{Operation: operation, Organization: orgID, User: user, Revision: revision, MemberRevision: members, Epoch: own.Epoch}}
	if !plan.Binding.Valid() || revision == ^uint64(0) || members == ^uint64(0) {
		return empty, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
	 VALUES(?,?,?,?,'CLOSE',?,'FENCING',?)`, operation, orgID, user, user, revision, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1 WHERE id=?", orgID); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1 WHERE organization_id=? AND user_id=?", orgID, user); err != nil {
		return empty, err
	}
	scope := plan.Binding.Scope()
	if _, err = tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?)
	 ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1`, scope, own.Epoch, operation); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=?", scope, own.Epoch); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
	 VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, operation, user, scope, own.Epoch, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, OrganizationCloseRecoveryKind, operation, operation, string(raw)); err != nil {
		return empty, err
	}
	return plan, nil
}

func ClaimOrganizationClose(ctx context.Context, tx *sql.Tx, instance string) (domain.AccountCloseFenceTask, error) {
	claimed, err := claimClosureTask(ctx, tx, []string{OrganizationCloseRecoveryKind})
	if err != nil {
		return domain.AccountCloseFenceTask{}, err
	}
	task, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, "SELECT "+accountCloseFenceTaskColumns+" FROM durable_task t WHERE id=? AND kind=? FOR UPDATE", claimed.ID, OrganizationCloseRecoveryKind))
	if err != nil || instance == "" {
		return task, errors.New("ORGANIZATION_CLOSE_CLAIM_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state='RUNNING',instance_id=?,generation=generation+1,
	 lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=IF(attempts<4294967295,attempts+1,attempts)
	 WHERE id=? AND kind=? AND generation=? AND generation<18446744073709551615`, instance, task.ID, OrganizationCloseRecoveryKind, task.Generation)
	if err = accountCloseChanged(result, err); err != nil {
		return task, err
	}
	task.Generation++
	task.State, task.Instance, task.LeaseValid = "RUNNING", instance, true
	return task, nil
}

func ValidateOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask) error {
	current, err := scanAccountCloseFenceTask(tx.QueryRowContext(ctx, "SELECT "+accountCloseFenceTaskColumns+" FROM durable_task t WHERE id=? AND kind=? FOR UPDATE", task.ID, OrganizationCloseRecoveryKind))
	if err != nil {
		return err
	}
	if current.Operation != task.Operation || current.Revocation != task.Operation || current.Generation != task.Generation || current.Instance != task.Instance || current.State != "RUNNING" || !current.LeaseValid || !bytes.Equal(current.Payload, task.Payload) {
		return errors.New("ORGANIZATION_CLOSE_OWNERSHIP_CHANGED")
	}
	return nil
}

func RetryOrganizationClose(ctx context.Context, db *sql.DB, task domain.AccountCloseFenceTask, code string) {
	_, _ = db.ExecContext(ctx, `UPDATE durable_task SET state='RETRY',instance_id=NULL,lease_until=NULL,last_error=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING'`, code, task.ID, OrganizationCloseRecoveryKind, task.Generation, task.Instance)
}

func FinishOrganizationCloseTask(ctx context.Context, tx *sql.Tx, task domain.AccountCloseFenceTask, terminal bool) error {
	state := "QUEUED"
	if terminal {
		state = "SUCCEEDED"
	}
	result, err := tx.ExecContext(ctx, `UPDATE durable_task SET state=?,instance_id=NULL,lease_until=NULL,last_error=NULL,retry_at=UTC_TIMESTAMP(6)
	 WHERE id=? AND kind=? AND generation=? AND instance_id=? AND state='RUNNING' AND lease_until>UTC_TIMESTAMP(6)`, state, task.ID, OrganizationCloseRecoveryKind, task.Generation, task.Instance)
	return accountCloseChanged(result, err)
}

// Proof核验原授权、gate、固定目标与真实Doc ACK；单纯组织CLOSING不能授予清理资格。
func OrganizationCloseProof(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding, requireACK bool) (bool, error) {
	var state, scope, gateState, revocation string
	var actor, epoch, maxEpoch uint64
	var target, participants []byte
	err := q.QueryRowContext(ctx, `SELECT a.actor_id,a.state,a.scope_key,a.epoch,a.target_permission,a.participant_plan,g.max_epoch,g.state,COALESCE(g.last_revocation_id,'')
	 FROM authorization_change a JOIN access_gate g ON g.scope_key=a.scope_key WHERE a.revocation_id=?`, binding.Operation).Scan(&actor, &state, &scope, &epoch, &target, &participants, &maxEpoch, &gateState, &revocation)
	if err != nil {
		return false, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(target)
	var peers []string
	if err != nil || plan.Binding != binding || actor != binding.User || scope != binding.Scope() || epoch != binding.Epoch || maxEpoch != epoch || gateState != "BLOCKED" || revocation != binding.Operation || json.Unmarshal(participants, &peers) != nil || !slices.Equal(peers, []string{"document"}) {
		return false, errors.New("ORGANIZATION_CLOSE_PROOF_CHANGED")
	}
	if !requireACK {
		return state == "FENCING", nil
	}
	if state != "SUCCEEDED" {
		return false, nil
	}
	var digest []byte
	err = q.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'`, binding.Operation, scope, epoch).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	request := OrganizationCloseFenceRequest(binding)
	if !bytes.Equal(digest, platform.BootstrapDigest(request, "document")) {
		return false, errors.New("ORGANIZATION_CLOSE_ACK_CHANGED")
	}
	return true, nil
}

func OrganizationCloseFenceRequest(b platform.OrganizationClosureBinding) *pb.FenceRequest {
	return &pb.FenceRequest{RevocationId: b.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: b.Scope(), Epoch: b.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(b.Scope(), b.Epoch)}
}

func SaveOrganizationCloseFence(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, digest []byte) error {
	if !bytes.Equal(digest, platform.BootstrapDigest(OrganizationCloseFenceRequest(b), "document")) {
		return errors.New("ORGANIZATION_CLOSE_ACK_INVALID")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at)
	 VALUES(?,'document',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE revocation_id=revocation_id`, b.Operation, b.Scope(), b.Epoch, digest); err != nil {
		return err
	}
	var prior []byte
	if err := tx.QueryRowContext(ctx, `SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant='document' AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'`, b.Operation, b.Scope(), b.Epoch).Scan(&prior); err != nil {
		return err
	}
	if !bytes.Equal(prior, digest) {
		return errors.New("ORGANIZATION_CLOSE_ACK_CONFLICT")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED',retry_at=NULL,last_error=NULL WHERE revocation_id=? AND state='FENCING'", b.Operation); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE organization_transition SET state='APPLIED',generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='FENCING'", b.Operation)
	return accountCloseChanged(result, err)
}

func SaveOrganizationCloseCursor(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE organization_transition SET fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='APPLIED'", string(raw), plan.Binding.Operation)
	return accountCloseChanged(result, err)
}

func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, result []byte) error {
	statements := []statement{
		{"UPDATE organization_member SET state='CLOSED' WHERE organization_id=? AND user_id=? AND state='REVOKING' AND epoch=?", []any{b.Organization, b.User, b.Epoch + 1}},
		{"UPDATE organization SET lifecycle='CLOSED',closed_at=UTC_TIMESTAMP(6) WHERE id=? AND lifecycle='CLOSING'", []any{b.Organization}},
		{"UPDATE organization_invitation SET state='CLOSED' WHERE organization_id=? AND state='OPEN'", []any{b.Organization}},
		{"UPDATE organization_application SET state='CLOSED' WHERE organization_id=? AND state='OPEN'", []any{b.Organization}},
		{"UPDATE organization_creation_grant SET state='CLOSED',revision=revision+1 WHERE organization_id=? AND state<>'CLOSED'", []any{b.Organization}},
		{"UPDATE organization_transition SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='CLOSE' AND state='APPLIED'", []any{b.Operation}},
		{"UPDATE operation_receipt SET state='SUCCEEDED',result_json=?,terminal_at=UTC_TIMESTAMP(6),generation=generation+1 WHERE operation_id=? AND kind='ORG_CLOSE' AND state='PREPARING'", []any{string(result), b.Operation}},
	}
	return execute(ctx, tx, statements)
}

// Ready全量分页检查本流程未终结任务，既不接管账号任务，也不靠lease过期猜终态。
func OrganizationCloseReady(ctx context.Context, db *sql.DB) error {
	var after uint64
	for {
		rows, err := db.QueryContext(ctx, `SELECT t.id,t.operation_id,t.business_key,t.payload,c.organization_id,c.actor_id,c.state,c.fixed_plan,COALESCE(o.kind,''),COALESCE(o.state,'')
		 FROM durable_task t LEFT JOIN organization_transition c ON c.operation_id=t.operation_id AND c.kind='CLOSE'
		 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id WHERE t.kind=? AND t.state NOT IN ('SUCCEEDED','REJECTED') AND t.id>? ORDER BY t.id LIMIT 100`, OrganizationCloseRecoveryKind, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var id uint64
			var operation, key, state, kind, receipt string
			var org, user sql.NullInt64
			var payload, raw []byte
			if err = rows.Scan(&id, &operation, &key, &payload, &org, &user, &state, &raw, &kind, &receipt); err != nil {
				break
			}
			p, pErr := domain.DecodeOrganizationClosePlan(payload)
			c, cErr := domain.DecodeOrganizationClosePlan(raw)
			if pErr != nil || cErr != nil || p.Binding != c.Binding || p.Binding.Operation != operation || key != operation || !org.Valid || uint64(org.Int64) != p.Binding.Organization || !user.Valid || uint64(user.Int64) != p.Binding.User || (state != "FENCING" && state != "APPLIED") || kind != "ORG_CLOSE" || receipt != "PREPARING" {
				err = errors.New("ORGANIZATION_CLOSE_TASK_INVALID")
				break
			}
			after, count = id, count+1
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if count < 100 {
			return nil
		}
	}
}

// organization_read.go：保留原实现，按业务合并文件。

type Organizations struct{ db *sql.DB }

func NewOrganizations(db *sql.DB) *Organizations { return &Organizations{db: db} }

func ReadOrganization(ctx context.Context, query Queryer, id string, lock bool) (domain.Organization, error) {
	var value domain.Organization
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := query.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),name,CAST(owner_id AS CHAR),lifecycle,CAST(revision AS CHAR),CAST(member_revision AS CHAR) FROM organization WHERE id=?"+suffix, id).Scan(&value.ID, &value.Name, &value.OwnerID, &value.Lifecycle, &value.Revision, &value.MemberRevision)
	return value, err
}

func ReadOrganizationMember(ctx context.Context, query Queryer, organization, actor string, lock bool) (domain.OrganizationMember, error) {
	var value domain.OrganizationMember
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := query.QueryRowContext(ctx, "SELECT role,state,epoch,CAST(revision AS CHAR) FROM organization_member WHERE organization_id=? AND user_id=?"+suffix, organization, actor).Scan(&value.Role, &value.State, &value.Epoch, &value.Revision)
	return value, err
}

func (r *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.OrganizationEntry, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT CAST(o.id AS CHAR),o.name,CAST(o.owner_id AS CHAR),o.lifecycle,CAST(o.revision AS CHAR),CAST(o.member_revision AS CHAR),m.epoch FROM organization o JOIN organization_member m ON m.organization_id=o.id AND m.user_id=? AND m.state='ACTIVE' WHERE o.lifecycle='ACTIVE' AND o.id>? ORDER BY o.id LIMIT ?", actor, last, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationEntry{}
	for rows.Next() {
		var value domain.OrganizationEntry
		org := &value.Organization
		if rows.Scan(&org.ID, &org.Name, &org.OwnerID, &org.Lifecycle, &org.Revision, &org.MemberRevision, &value.MemberEpoch) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

func (r *Organizations) Members(ctx context.Context, organization, last string, limit int, search ...string) ([]domain.OrganizationMember, error) {
	query := ""
	if len(search) > 0 {
		query = search[0]
	}
	identity := strings.TrimLeft(query, "0")
	// 与Identity的建库规则一致：所有者/管理员由角色获得资格，普通成员读取独立授权。
	// 同一SQL读取，避免逐个成员查询造成状态混杂或N+1请求；分页仍绑定member_revision。
	rows, err := r.db.QueryContext(ctx, `SELECT CAST(m.user_id AS CHAR),u.display_name,m.role,m.state,
		CAST(m.revision AS CHAR),m.epoch,u.identity_epoch,
		CASE WHEN m.state<>'ACTIVE' THEN 'NONE'
		     WHEN o.owner_id=m.user_id OR m.role='ADMIN' THEN 'ROLE'
		     WHEN g.state='ACTIVE' THEN 'GRANTED' ELSE 'NONE' END
		FROM organization_member m JOIN user u ON u.id=m.user_id
		JOIN organization o ON o.id=m.organization_id
		LEFT JOIN organization_creation_grant g ON g.organization_id=m.organization_id
		  AND g.user_id=m.user_id AND g.capability='CREATE_KNOWLEDGE_BASE'
		WHERE m.organization_id=? AND m.state<>'CLOSED' AND m.user_id>?
		AND (?='' OR LOCATE(?,LOWER(u.display_name))>0 OR CAST(m.user_id AS CHAR)=?)
		ORDER BY m.user_id LIMIT ?`, organization, last, query, query, identity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.OrganizationMember{}
	for rows.Next() {
		var value domain.OrganizationMember
		if rows.Scan(&value.UserID, &value.DisplayName, &value.Role, &value.State, &value.Revision, &value.Epoch, &value.IdentityEpoch, &value.KnowledgeBaseCreation) != nil {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
		}
		values = append(values, value)
	}
	if rows.Err() != nil {
		return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "PAGE_UNAVAILABLE"}
	}
	return values, nil
}

// organization_transition.go：保留原实现，按业务合并文件。

func (r *Organizations) LockTransitionMember(ctx context.Context, tx *sql.Tx, organization, user string) (domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := tx.QueryRowContext(ctx, `
		SELECT role, state, CAST(revision AS CHAR), epoch
		FROM organization_member
		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, user).
		Scan(&member.Role, &member.State, &member.Revision, &member.Epoch)
	return member, err
}

func (r *Organizations) PrepareMemberTransition(ctx context.Context, tx *sql.Tx, organization, user string) error {
	return execute(ctx, tx, []statement{
		{`UPDATE organization_member
		  SET state='REVOKING', epoch=epoch+1, revision=revision+1
		  WHERE organization_id=? AND user_id=?`, []any{organization, user}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}

func (r *Organizations) InsertMemberTransition(ctx context.Context, tx *sql.Tx, id, organization, actor, user, kind, expected string, plan []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO organization_transition
		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
		VALUES(?,?,?,?,?,?,'FENCING',?)`, id, organization, actor, user, kind, expected, string(plan))
	return err
}

func (r *Organizations) LockUserState(ctx context.Context, tx *sql.Tx, user string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", user).Scan(&state)
	return state, err
}

func (r *Organizations) LockOwnerMember(ctx context.Context, tx *sql.Tx, organization, owner string) (domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := tx.QueryRowContext(ctx, `
		SELECT epoch, role FROM organization_member
		WHERE organization_id=? AND user_id=? FOR UPDATE`, organization, owner).
		Scan(&member.Epoch, &member.Role)
	return member, err
}

func (r *Organizations) PrepareOwnerTransfer(ctx context.Context, tx *sql.Tx, organization, owner, target string) error {
	return execute(ctx, tx, []statement{
		{`UPDATE organization SET owner_id=?,revision=revision+1,member_revision=member_revision+1
		  WHERE id=?`, []any{target, organization}},
		{`UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1
		  WHERE organization_id=? AND user_id=?`, []any{organization, owner}},
	})
}

func (r *Organizations) InsertOwnerTransfer(ctx context.Context, tx *sql.Tx, id, organization, owner, target, expected string, plan []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO organization_transition
		(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
		VALUES(?,?,?,?,'TRANSFER_OWNER',?,'FENCING',?)`, id, organization, owner, target, expected, string(plan))
	return err
}

// organization_write.go：保留原实现，按业务合并文件。

func (r *Organizations) Insert(ctx context.Context, tx *sql.Tx, name, owner string) (int64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,'ACTIVE')", name, owner)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	_, err = tx.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", id, owner)
	return id, err
}

func (r *Organizations) Rename(ctx context.Context, tx *sql.Tx, id, name string) error {
	_, err := tx.ExecContext(ctx, "UPDATE organization SET name=?,revision=revision+1 WHERE id=?", name, id)
	return err
}

func (r *Organizations) LockMemberState(ctx context.Context, tx *sql.Tx, id, user string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", id, user).Scan(&state)
	return state, err
}

func (r *Organizations) LockCreationGrant(ctx context.Context, tx *sql.Tx, id, user string) (uint64, string, error) {
	var revision uint64
	var state string
	err := tx.QueryRowContext(ctx, "SELECT revision,state FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability='CREATE_KNOWLEDGE_BASE' FOR UPDATE", id, user).Scan(&revision, &state)
	return revision, state, err
}

func (r *Organizations) WriteCreationGrant(ctx context.Context, tx *sql.Tx, id, user, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO organization_creation_grant(organization_id,user_id,capability,state) VALUES(?,?,'CREATE_KNOWLEDGE_BASE',?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", id, user, state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE organization SET member_revision=member_revision+1 WHERE id=?", id)
	return err
}
