# services/account/internal/repository/account/authentication_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/048-authentication-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Authentication`、`NewAuthentication`、`LoginUser`、`LockCredential`、`InsertSession`、`Session`、`Credential`、`User`、`Challenges`、`NewChallenges`、`ResetAccount`、`Issue`、`Meta`、`Verify`、`Credentials`、`NewCredentials`、`Hash`、`Receipt`、`LockUser`、`ConsumeProof`、`LockSession`、`Prepare`、`LoginLimit`、`NewLoginLimit`、`Allow`、`Signups`、`NewSignups`、`DuplicateKey`、`PublicReceipt`、`InsertUser`、`Consume`、`InsertReceipt`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

// authentication.go：保留原实现，按业务合并文件。

type Authentication struct {
	conn sqlx.SqlConn
	db   *sql.DB
}

func NewAuthentication(conn sqlx.SqlConn, db *sql.DB) *Authentication {
	// 两个适配必须来自Store的同一连接池，不能为事务里的查询另开池。
	return &Authentication{conn: conn, db: db}
}

func (r *Authentication) LoginUser(ctx context.Context, email string) (domain.LoginUser, error) {
	var row struct {
		ID              string `db:"id"`
		DisplayName     string `db:"display_name"`
		Hash            []byte `db:"password_hash"`
		Algorithm       string `db:"password_algorithm"`
		State           string `db:"state"`
		CredentialEpoch string `db:"credential_epoch"`
	}
	err := r.conn.QueryRowCtx(ctx, &row, "SELECT CAST(id AS CHAR) AS id,display_name,COALESCE(password_hash,X'') AS password_hash,COALESCE(password_algorithm,'') AS password_algorithm,state,CAST(credential_epoch AS CHAR) AS credential_epoch FROM user WHERE email_key=?", []byte(email))
	return domain.LoginUser{
		ID: row.ID, DisplayName: row.DisplayName, Hash: row.Hash,
		Algorithm: row.Algorithm, State: row.State, CredentialEpoch: row.CredentialEpoch,
	}, err
}

func (r *Authentication) LockCredential(ctx context.Context, tx *sql.Tx, userID string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE", userID).Scan(&value.Hash, &value.Epoch, &value.State)
	return value, err
}

func (r *Authentication) InsertSession(ctx context.Context, tx *sql.Tx, value domain.Session) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO session(id,user_id,csrf_hash,credential_epoch,state,expires_at,last_seen_at) VALUES(?,?,?,?,'ACTIVE',?,UTC_TIMESTAMP(6))", value.ID, value.UserID, value.CSRF, value.CredentialEpoch, value.Expires)
	return err
}

func (r *Authentication) Session(ctx context.Context, id []byte) (domain.Session, error) {
	value := domain.Session{ID: id}
	err := r.db.QueryRowContext(ctx, "SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?", id).Scan(&value.UserID, &value.CSRF, &value.Epoch, &value.CredentialEpoch, &value.State, &value.Expires)
	return value, err
}

func (r *Authentication) Credential(ctx context.Context, userID string) (domain.Credential, error) {
	var value domain.Credential
	err := r.db.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?", userID).Scan(&value.State, &value.Epoch, &value.IdentityEpoch)
	return value, err
}

func (r *Authentication) User(ctx context.Context, userID string) (domain.User, error) {
	var value domain.User
	err := r.db.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),COALESCE(email,''),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?", userID).Scan(&value.ID, &value.Email, &value.DisplayName, &value.State, &value.CredentialEpoch, &value.IdentityEpoch)
	return value, err
}

// challenge.go：保留原实现，按业务合并文件。

type Challenges struct {
	db    *sql.DB
	redis *redis.Redis
}

func NewChallenges(db *sql.DB, limiter *redis.Redis) *Challenges {
	return &Challenges{db: db, redis: limiter}
}

const issueChallengeScript = `
local total=redis.call('INCR',KEYS[1]); if total==1 then redis.call('EXPIRE',KEYS[1],60) end
if total>100 then return 0 end
if not redis.call('SET',KEYS[2],'1','NX','EX',60) then return 0 end
redis.call('HSET',KEYS[3],'digest',ARGV[1],'meta',ARGV[2],'attempts',0)
redis.call('EXPIRE',KEYS[3],600)
return 1`

const verifyChallengeScript = `
local digest=redis.call('HGET',KEYS[1],'digest')
if not digest then return 0 end
local attempts=tonumber(redis.call('HGET',KEYS[1],'attempts') or '5')
if attempts>=5 then return 0 end
if digest~=ARGV[1] then redis.call('HINCRBY',KEYS[1],'attempts',1); return 0 end
return 1`

func (r *Challenges) ResetAccount(ctx context.Context, email string) (domain.ChallengeMeta, string, error) {
	meta := domain.ChallengeMeta{UserID: "0", Epoch: "1"}
	var state string
	err := r.db.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(credential_epoch AS CHAR),state FROM user WHERE email_key=?", []byte(email)).Scan(&meta.UserID, &meta.Epoch, &state)
	return meta, state, err
}

func (r *Challenges) Issue(ctx context.Context, id string, meta domain.ChallengeMeta, digest string, raw []byte) (any, error) {
	keys := []string{"acct:challenge:global", "acct:challenge:cooldown:" + meta.Purpose + ":" + meta.Subject, "acct:challenge:" + id}
	return r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))
}

func (r *Challenges) Meta(ctx context.Context, id string) (any, error) {
	return r.redis.EvalCtx(ctx, `return redis.call('HGET',KEYS[1],'meta') or ''`, []string{"acct:challenge:" + id})
}

func (r *Challenges) Verify(ctx context.Context, id, digest string) (any, error) {
	return r.redis.EvalCtx(ctx, verifyChallengeScript, []string{"acct:challenge:" + id}, digest)
}

// credential.go：保留原实现，按业务合并文件。

type Credentials struct{ db *sql.DB }

func NewCredentials(db *sql.DB) *Credentials { return &Credentials{db: db} }

func (r *Credentials) Hash(ctx context.Context, user string) ([]byte, error) {
	var hash []byte
	err := r.db.QueryRowContext(ctx, "SELECT password_hash FROM user WHERE id=?", user).Scan(&hash)
	return hash, err
}

func (r *Credentials) Receipt(ctx context.Context, query Queryer, id string, lock bool) (domain.CredentialReceipt, error) {
	var value domain.CredentialReceipt
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := query.QueryRowContext(ctx, `
		SELECT CAST(actor_id AS CHAR),kind,state,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),''),request_fingerprint
		FROM operation_receipt WHERE operation_id=?`+suffix, id).
		Scan(&value.ActorID, &value.Kind, &value.State, &value.SessionHash, &value.Fingerprint)
	return value, err
}

func (r *Credentials) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE", user).Scan(&value.State, &value.Epoch, &value.Hash)
	return value, err
}

func (r *Credentials) ConsumeProof(ctx context.Context, tx *sql.Tx, change domain.CredentialChange, epoch uint64, subject []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at)
		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.ChallengeID, change.Proof.Purpose, subject, change.UserID, epoch)
	return err
}

func (r *Credentials) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.CredentialEpoch, &value.Expires)
	return value, err
}

// credential_prepare.go：保留原实现，按业务合并文件。

func (r *Credentials) Prepare(ctx context.Context, tx *sql.Tx, plan domain.CredentialPreparation) error {
	change := plan.Change
	err := execute(ctx, tx, []statement{
		{`INSERT INTO operation_receipt
		  (operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at)
		  VALUES(?,?,?,?,?,'PREPARING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY))`, []any{change.ID, change.UserID, change.Kind, string(plan.Target), change.Fingerprint, string(plan.Pending)}},
		{"UPDATE user SET state='REVOKING',credential_epoch=credential_epoch+1 WHERE id=?", []any{change.UserID}},
		{"UPDATE session SET state='REVOKING',epoch=epoch+1 WHERE user_id=? AND credential_epoch<=? AND state='ACTIVE'", []any{change.UserID, plan.Epoch}},
		{"UPDATE identity_admission SET state='CLOSED' WHERE actor_id=?", []any{change.UserID}},
		{`INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id)
		  VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),
		  state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1`, []any{plan.Scope, plan.Epoch, change.ID}},
		{`INSERT INTO authorization_change
		  (revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
		  VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, []any{change.ID, change.UserID, plan.Scope, plan.Epoch, string(plan.Target)}},
		{`INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		  VALUES('CREDENTIAL_CHANGE',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, []any{change.ID, change.ID, string(plan.Payload)}},
	})
	if err != nil {
		return errors.New("CREDENTIAL_COMMIT_UNKNOWN")
	}
	if len(change.NewHash) > 0 {
		if _, err := tx.ExecContext(ctx, "UPDATE user SET password_hash=?,password_algorithm='bcrypt' WHERE id=?", change.NewHash, change.UserID); err != nil {
			return errors.New("CREDENTIAL_COMMIT_UNKNOWN")
		}
	}
	if change.NewEmail != "" {
		_, err := tx.ExecContext(ctx, "UPDATE user SET email=?,email_key=?,email_verified_at=UTC_TIMESTAMP(6) WHERE id=?", change.NewEmail, []byte(change.NewEmail), change.UserID)
		if err != nil {
			// 只有邮箱更新语句的唯一冲突是确定409；准备步骤的错误仍保持UNKNOWN。
			if DuplicateKey(err) {
				return domain.RuleViolation{Kind: domain.StateConflict, Code: "EMAIL_ALREADY_REGISTERED"}
			}
			return errors.New("CREDENTIAL_COMMIT_UNKNOWN")
		}
	}
	return nil
}

// login_limit.go：保留原实现，按业务合并文件。

const loginRateScript = `
local total=redis.call('INCR',KEYS[1]); if total==1 then redis.call('EXPIRE',KEYS[1],60) end
if total>100 then return 0 end
local count=redis.call('INCR',KEYS[2]); if count==1 then redis.call('EXPIRE',KEYS[2],60) end
if count>10 then return 0 end
return 1`

type LoginLimit struct{ redis *redis.Redis }

func NewLoginLimit(client *redis.Redis) *LoginLimit { return &LoginLimit{redis: client} }

func (r *LoginLimit) Allow(ctx context.Context, email string) (bool, error) {
	// 同一Lua完成计数与过期；总量先拒绝，随机邮箱不能无限创建限流键。
	digest := sha256.Sum256([]byte(email))
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	value, err := r.redis.EvalCtx(ctx, loginRateScript, []string{"acct:login:global", "acct:login:email:" + hex.EncodeToString(digest[:])})
	if err != nil {
		return false, errors.New("LOGIN_LIMITER_UNAVAILABLE")
	}
	count, ok := value.(int64)
	return ok && count == 1, nil
}

// signup.go：保留原实现，按业务合并文件。

type Signups struct{ db *sql.DB }

func NewSignups(db *sql.DB) *Signups { return &Signups{db: db} }

func DuplicateKey(err error) bool {
	var driver *mysql.MySQLError
	return errors.As(err, &driver) && driver.Number == 1062
}

func (r *Signups) PublicReceipt(ctx context.Context, id string) (domain.PublicReceipt, error) {
	var value domain.PublicReceipt
	err := r.db.QueryRowContext(ctx, "SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?", id).Scan(&value.Kind, &value.Fingerprint, &value.Result)
	return value, err
}

func (r *Signups) InsertUser(ctx context.Context, tx *sql.Tx, email, displayName string, hash []byte) (int64, error) {
	row, err := tx.ExecContext(ctx, `
		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at)
		VALUES(?,?,?,?,'bcrypt','ACTIVE',UTC_TIMESTAMP(6))`, email, []byte(email), displayName, hash)
	if err != nil {
		return 0, err
	}
	id, _ := row.LastInsertId()
	return id, nil
}

func (r *Signups) Consume(ctx context.Context, tx *sql.Tx, challenge string, subject []byte, user int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO verification_consumption
		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at)
		VALUES(?,'SIGNUP',?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)
	return err
}

func (r *Signups) InsertReceipt(ctx context.Context, tx *sql.Tx, id, challenge string, user int64, fingerprint, result []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO operation_receipt
		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at)
		VALUES(?,?,'SIGNUP',?,?,'SUCCEEDED',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),UTC_TIMESTAMP(6))`, id, user, fmt.Sprintf(`{"challenge_id":%q}`, challenge), fingerprint, string(result))
	return err
}
````

UTF-8/LF全文SHA-256：`5971c574ce65e08fe8b5cdc130788963cc4979c4c4b6f76221a232f3651ed321`。[参考实现](../../../../services/account/internal/repository/account/authentication_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
