# services/account/internal/repository/account/identity_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/050-identity-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`AuthorizationResultFacts`、`AuthorizationResults`、`NewAuthorizationResults`、`Read`、`Queryer`、`Identity`、`NewIdentity`、`SessionActor`、`SessionExpiry`、`CreateAdmission`、`AttachSource`、`Admission`、`LockAdmission`、`LockUser`、`LockSession`、`LockTargetIdentity`、`TargetDisplayName`、`LockOrganization`、`LockMember`、`LockCreationGrant`、`LockTargetUser`、`LockTargetMember`、`EnsureGate`、`LockGate`、`AdvanceGate`、`Mutation`、`NewMutation`、`Receipt`、`InsertReceipt`、`InsertAudit`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
	"errors"
	"time"
)

// authorization_result.go：保留原实现，按业务合并文件。

// AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。
type AuthorizationResultFacts struct {
	Actor, Epoch, GateEpoch, ReceiptActor, CloseUser, Organization, OrganizationActor    uint64
	Scope, State, GateState, GateRevocation, TaskKind, TaskOperation, TaskState          string
	ReceiptKind, ReceiptState, CloseState, OrganizationState                             string
	Target, Participants, Payload, ReceiptTarget, Fingerprint, Capture, OrganizationPlan []byte
	AckEpoch                                                                             uint64
	AckPhase                                                                             string
	AckDigest                                                                            []byte
	AckCompleted                                                                         bool
	ReceiptTerminal                                                                      bool
}

type AuthorizationResults struct{ db *sql.DB }

func NewAuthorizationResults(db *sql.DB) *AuthorizationResults { return &AuthorizationResults{db: db} }

func (r *AuthorizationResults) Read(ctx context.Context, operation string) (AuthorizationResultFacts, error) {
	// 单条SELECT在同一InnoDB语句快照中读全部事实，不能拼接并发提交前后的状态/ACK。
	// 限定真实业务任务种类，不接管历史底座或仅写入授权表的未知记录。
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(a.actor_id,0),a.scope_key,a.epoch,a.state,a.target_permission,a.participant_plan,
	 COALESCE(g.max_epoch,0),COALESCE(g.state,''),COALESCE(g.last_revocation_id,''),
	 COALESCE(t.kind,''),COALESCE(t.operation_id,''),COALESCE(t.state,''),t.payload,
	 COALESCE(o.actor_id,0),COALESCE(o.kind,''),COALESCE(o.state,''),o.target_json,o.request_fingerprint,COALESCE(o.terminal_at IS NOT NULL,FALSE),
	 COALESCE(c.user_id,0),COALESCE(c.state,''),c.retention_plan,
	 COALESCE(z.organization_id,0),COALESCE(z.actor_id,0),COALESCE(z.state,''),z.fixed_plan,
	 COALESCE(k.epoch,0),COALESCE(k.phase,''),k.result_digest,COALESCE(k.completed_at IS NOT NULL,FALSE)
	 FROM authorization_change a
	 LEFT JOIN access_gate g ON g.scope_key=a.scope_key
	 LEFT JOIN durable_task t ON t.business_key=a.revocation_id AND t.kind IN
	 ('SESSION_LOGOUT','CREDENTIAL_CHANGE','ORGANIZATION_CHANGE','PLATFORM_GOVERNANCE_CHANGE','ACCOUNT_CLOSE_FENCE','ORG_CLOSE_RECOVER')
	 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
	 LEFT JOIN account_close c ON c.operation_id=t.operation_id AND t.kind='ACCOUNT_CLOSE_FENCE'
	 LEFT JOIN organization_transition z ON z.operation_id=t.operation_id AND z.kind='CLOSE' AND t.kind='ORG_CLOSE_RECOVER'
	 LEFT JOIN authorization_ack k ON k.revocation_id=a.revocation_id AND k.participant='document' AND k.scope_key=a.scope_key
	 WHERE a.revocation_id=? LIMIT 2`, operation)
	if err != nil {
		return AuthorizationResultFacts{}, err
	}
	defer rows.Close()
	var f AuthorizationResultFacts
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return f, err
		}
		return f, sql.ErrNoRows
	}
	err = rows.Scan(&f.Actor, &f.Scope, &f.Epoch, &f.State, &f.Target, &f.Participants,
		&f.GateEpoch, &f.GateState, &f.GateRevocation, &f.TaskKind, &f.TaskOperation, &f.TaskState, &f.Payload,
		&f.ReceiptActor, &f.ReceiptKind, &f.ReceiptState, &f.ReceiptTarget, &f.Fingerprint, &f.ReceiptTerminal,
		&f.CloseUser, &f.CloseState, &f.Capture, &f.Organization, &f.OrganizationActor, &f.OrganizationState, &f.OrganizationPlan,
		&f.AckEpoch, &f.AckPhase, &f.AckDigest, &f.AckCompleted)
	if err != nil {
		return f, err
	}
	if rows.Next() {
		return f, errors.New("AUTHORIZATION_TASK_BINDING_AMBIGUOUS")
	}
	return f, rows.Err()
}

// identity.go：保留原实现，按业务合并文件。

// Queryer使只读查询可以明确使用原池或调用方事务；repository不会自行开启事务。
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Identity struct{}

func NewIdentity() *Identity { return &Identity{} }

func (r *Identity) SessionActor(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT user_id FROM session WHERE id=?", session).Scan(actor)
}

func (r *Identity) SessionExpiry(ctx context.Context, tx *sql.Tx, session []byte, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT expires_at FROM session WHERE id=?", session).Scan(expires)
}

func (r *Identity) CreateAdmission(ctx context.Context, tx *sql.Tx, id string, actor uint64, session []byte, operation, purpose, binding string, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO identity_admission(admission_id,actor_id,session_id,operation_id,purpose,target_json,state,expires_at) VALUES(?,?,?,?,?,?,'OPEN',?)", id, actor, session, operation, purpose, binding, expires)
	return err
}

func (r *Identity) AttachSource(ctx context.Context, tx *sql.Tx, admission, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO admission_source(admission_id,scope_key,epoch) VALUES(?,?,?)", admission, scope, epoch)
	return err
}

func (r *Identity) Admission(ctx context.Context, q Queryer, id string, raw *[]byte, state *string, expires *time.Time) error {
	return q.QueryRowContext(ctx, "SELECT target_json,state,expires_at FROM identity_admission WHERE admission_id=?", id).Scan(raw, state, expires)
}

func (r *Identity) LockAdmission(ctx context.Context, tx *sql.Tx, id string, state *string, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT state,expires_at FROM identity_admission WHERE admission_id=? FOR UPDATE", id).Scan(state, expires)
}

func (r *Identity) LockUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string, credential, identity *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,credential_epoch,identity_epoch FROM user WHERE id=? FOR UPDATE", actor).Scan(state, credential, identity)
}

func (r *Identity) LockSession(ctx context.Context, tx *sql.Tx, session []byte, actor *uint64, state *string, credential, epoch *uint64, expires *time.Time) error {
	return tx.QueryRowContext(ctx, "SELECT user_id,state,credential_epoch,epoch,expires_at FROM session WHERE id=? FOR UPDATE", session).Scan(actor, state, credential, epoch, expires)
}

func (r *Identity) LockTargetIdentity(ctx context.Context, tx *sql.Tx, actor uint64, state *string, identity *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,identity_epoch FROM user WHERE id=? FOR UPDATE", actor).Scan(state, identity)
}

// 仅在目标身份已通过同一事务重验后读取；不复制账号库，也不把邮箱带出服务边界。
func (r *Identity) TargetDisplayName(ctx context.Context, tx *sql.Tx, user uint64) (string, error) {
	var name string
	err := tx.QueryRowContext(ctx, "SELECT display_name FROM user WHERE id=? AND state='ACTIVE' FOR UPDATE", user).Scan(&name)
	return name, err
}

func (r *Identity) LockOrganization(ctx context.Context, tx *sql.Tx, organization uint64, owner *uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT owner_id,lifecycle FROM organization WHERE id=? FOR UPDATE", organization).Scan(owner, state)
}

func (r *Identity) LockMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, role, state *string, epoch *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT role,state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, actor).Scan(role, state, epoch)
}

func (r *Identity) LockCreationGrant(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, revision *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,revision FROM organization_creation_grant WHERE organization_id=? AND user_id=? AND capability='CREATE_KNOWLEDGE_BASE' FOR UPDATE", organization, actor).Scan(state, revision)
}

func (r *Identity) LockTargetUser(ctx context.Context, tx *sql.Tx, actor uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT state FROM user WHERE id=? FOR UPDATE", actor).Scan(state)
}

func (r *Identity) LockTargetMember(ctx context.Context, tx *sql.Tx, organization, actor uint64, state *string, epoch *uint64) error {
	return tx.QueryRowContext(ctx, "SELECT state,epoch FROM organization_member WHERE organization_id=? AND user_id=? FOR UPDATE", organization, actor).Scan(state, epoch)
}

// identity_gate.go：保留原实现，按业务合并文件。

func (r *Identity) EnsureGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT IGNORE INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN')", scope, epoch)
	return err
}

func (r *Identity) LockGate(ctx context.Context, tx *sql.Tx, scope string, epoch *uint64, state *string) error {
	return tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", scope).Scan(epoch, state)
}

func (r *Identity) AdvanceGate(ctx context.Context, tx *sql.Tx, scope string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='OPEN',last_revocation_id=NULL,revision=revision+1 WHERE scope_key=?", epoch, scope)
	return err
}

// mutation.go：保留原实现，按业务合并文件。

type Mutation struct{}

func NewMutation() *Mutation { return &Mutation{} }

func (r *Mutation) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=? FOR UPDATE", user).Scan(&value.State, &value.Epoch)
	return value, err
}

func (r *Mutation) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.Expires)
	return value, err
}

func (r *Mutation) Receipt(ctx context.Context, tx *sql.Tx, id string) (domain.MutationReceipt, error) {
	var value domain.MutationReceipt
	err := tx.QueryRowContext(ctx, "SELECT CAST(actor_id AS CHAR),kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=? FOR UPDATE", id).Scan(&value.ActorID, &value.Kind, &value.Fingerprint, &value.Result)
	return value, err
}

func (r *Mutation) InsertReceipt(ctx context.Context, tx *sql.Tx, input domain.Mutation, state string, result, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO operation_receipt(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) VALUES(?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),IF(?='SUCCEEDED',UTC_TIMESTAMP(6),NULL))", input.OperationID, input.Session.UserID, input.Kind, string(target), input.Fingerprint, state, string(result), state)
	return err
}

func (r *Mutation) InsertAudit(ctx context.Context, tx *sql.Tx, input domain.Mutation, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO audit_event(actor_id,operation_id,item_no,kind,target_json) VALUES(?,?,1,?,?)", input.Session.UserID, input.OperationID, input.Kind, string(target))
	return err
}
````

UTF-8/LF全文SHA-256：`69315c4635313a75ea48177e6f91929c155ee78e9a5d3fef7a9d8ebbecee9045`。[参考实现](../../../../services/account/internal/repository/account/identity_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
