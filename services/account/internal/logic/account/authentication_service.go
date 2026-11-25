// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"math/big"
	"strings"
	"time"
)

// authentication.go：保留原实现，按业务合并文件。

// Limiter是用例实际需要的能力，不暴露Redis的通用命令接口。
type Limiter interface {
	Allow(context.Context, string) (bool, error)
}

// AuthenticationStore由消费者声明，只暴露认证所需查询；事务连接仍由用例传入，
// 适配器不能为锁内查询另开连接。小接口也让“查无会话”和“数据库不可用”可独立验证。
type AuthenticationStore interface {
	LoginUser(context.Context, string) (domain.LoginUser, error)
	LockCredential(context.Context, *sql.Tx, string) (domain.Credential, error)
	InsertSession(context.Context, *sql.Tx, domain.Session) error
	Session(context.Context, []byte) (domain.Session, error)
	Credential(context.Context, string) (domain.Credential, error)
	User(context.Context, string) (domain.User, error)
}

// Authentication编排登录与会话校验；SQL负责权威状态，限流器负责请求频率。
// 它只返回业务结果，Cookie和HTTP状态由handler转换，便于其它入口复用同一规则。
type Authentication struct {
	db         *sql.DB
	repository AuthenticationStore
	limiter    Limiter
	dummyHash  []byte
}

// NewAuthentication显式接收SQL连接、查询适配器和限流能力，业务不直接依赖Redis命令。
// dummyHash由装配层预先生成，未知邮箱也走bcrypt校验；不能每次请求临时生成它。
func NewAuthentication(db *sql.DB, repo AuthenticationStore, limiter Limiter, dummyHash []byte) *Authentication {
	return &Authentication{db: db, repository: repo, limiter: limiter, dummyHash: dummyHash}
}

// Login负责登录用例，不设置HTTP Cookie：handler只在本用例确认提交后发送凭据。
// 流程为输入/限流→锁外密码校验→锁内凭据重检与会话写入→提交→返回原文token。
func (s *Authentication) Login(ctx context.Context, email, password string) (domain.LoginResult, error) {
	var result domain.LoginResult
	email = strings.ToLower(strings.TrimSpace(email))
	if !domain.ValidEmail(email) || !domain.ValidPassword(password) {
		return result, domain.ErrLoginInput
	}
	allowed, err := s.limiter.Allow(ctx, email)
	if err != nil {
		return result, err
	}
	if !allowed {
		return result, domain.ErrLoginLimited
	}

	user, err := s.repository.LoginUser(ctx, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	hash := s.dummyHash
	if err == nil && user.Algorithm == "bcrypt" {
		hash = user.Hash
	}
	// 不存在的账号也执行同成本校验，避免查无此人的分支暴露邮箱存在性。
	match := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if err != nil || !match || user.State != "ACTIVE" || user.Algorithm != "bcrypt" {
		return result, domain.ErrLoginInvalid
	}

	// 原文token交给浏览器，摘要id存入数据库；数据库泄露不应直接给出可用Cookie。
	// CSRF另用独立随机值，不能把会话秘密交给前端脚本充当CSRF字段。
	token, id, err := domain.RandomToken()
	if err != nil {
		return result, err
	}
	csrf, csrfHash, err := domain.RandomToken()
	if err != nil {
		return result, err
	}
	expires := time.Now().UTC().Add(12 * time.Hour)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	// 任何提前返回都回滚；成功Commit后Rollback只返回事务已结束，不撤销已提交会话。
	defer tx.Rollback()

	// 哈希计算在事务外；持有同一用户行锁后重检，旧密码校验不能越过凭据代次变化。
	current, err := s.repository.LockCredential(ctx, tx, user.ID)
	if err != nil {
		return result, errors.New("LOGIN_RECHECK_FAILED")
	}
	if current.State != "ACTIVE" || current.Epoch != user.CredentialEpoch || !bytes.Equal(current.Hash, user.Hash) {
		return result, domain.ErrLoginInvalid
	}
	value := domain.Session{ID: id, UserID: user.ID, CSRF: csrfHash, CredentialEpoch: current.Epoch, Expires: expires}
	if err := s.repository.InsertSession(ctx, tx, value); err != nil {
		return result, errors.New("SESSION_COMMIT_UNKNOWN")
	}
	// Commit报错可能发生在数据库已提交之后，只能报告未知；此时不返回成功登录凭据。
	if err := tx.Commit(); err != nil {
		return result, errors.New("SESSION_COMMIT_UNKNOWN")
	}
	return domain.LoginResult{UserID: user.ID, DisplayName: user.DisplayName, SessionToken: token, CSRFToken: csrf, Expires: expires}, nil
}

// Session先验证并摘要化Cookie，再查库；token原文不成为SQL中的持久身份字段。
func (s *Authentication) Session(ctx context.Context, token string) (domain.Session, error) {
	id, err := domain.TokenHash(token)
	if err != nil {
		return domain.Session{}, domain.ErrSessionInvalid
	}
	return s.repository.Session(ctx, id)
}

// Authenticate将Cookie原文转换为有效会话及当前身份代次；过期、停用或凭据变化统一视为无效。
// 返回的身份代次用于后续准入，不能替代目标业务在提交前的权限重检。
func (s *Authentication) Authenticate(ctx context.Context, token string) (domain.Authentication, error) {
	value, err := s.Session(ctx, token)
	result := domain.Authentication{Session: value}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrSessionInvalid) {
		return result, domain.ErrSessionInvalid
	}
	if err != nil {
		// 故障只证明本次无法核验，不能把它解释为撤销并让浏览器清除仍有效的登录态。
		return result, fmt.Errorf("read authentication session: %w", err)
	}
	if value.State != "ACTIVE" || !value.Expires.After(time.Now()) {
		return result, domain.ErrSessionInvalid
	}
	// 会话尚未到期也可能因改密码或账号状态改变而失效，需要比对当前凭据代次。
	// 这次读取不替代后续业务的事务内准入/来源校验，不能据此永久放行旧请求。
	credential, err := s.repository.Credential(ctx, value.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return result, domain.ErrSessionInvalid
	}
	if err != nil {
		return result, fmt.Errorf("read authentication credential: %w", err)
	}
	if credential.State != "ACTIVE" || credential.Epoch != value.CredentialEpoch {
		return result, domain.ErrSessionInvalid
	}
	result.IdentityEpoch = credential.IdentityEpoch
	return result, nil
}

// CurrentUser读取当前账号资料并重检状态/凭据代次，避免旧会话返回改密前的有效身份。
func (s *Authentication) CurrentUser(ctx context.Context, session domain.Session) (domain.User, error) {
	user, err := s.repository.User(ctx, session.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return user, domain.ErrSessionInvalid
	}
	if err != nil {
		return user, err
	}
	if user.State != "ACTIVE" || user.CredentialEpoch != session.CredentialEpoch {
		return user, domain.ErrSessionInvalid
	}
	return user, nil
}

// challenge.go：保留原实现，按业务合并文件。

type ChallengeMailer interface {
	Send(context.Context, string, string, string) error
	DeliveryMode() string
}

// ChallengeStore限制验证码存储依赖；业务不接收Redis通用客户端，也不隐藏SQL/Redis故障。
type ChallengeStore interface {
	ResetAccount(context.Context, string) (domain.ChallengeMeta, string, error)
	Issue(context.Context, string, domain.ChallengeMeta, string, []byte) (any, error)
	Meta(context.Context, string) (any, error)
	Verify(context.Context, string, string) (any, error)
}

type Challenges struct {
	repository ChallengeStore
	secrets    *domain.Secrets
	mailer     ChallengeMailer
}

func NewChallenges(repo ChallengeStore, secrets *domain.Secrets, mailer ChallengeMailer) *Challenges {
	return &Challenges{repository: repo, secrets: secrets, mailer: mailer}
}

// 只暴露实例配置的通道枚举；不能根据本次是否实际投递来透露找回邮箱是否注册。
func (s *Challenges) DeliveryMode() string {
	if s == nil || s.mailer == nil {
		return ""
	}
	mode := s.mailer.DeliveryMode()
	if mode == "SMTP" || mode == "DEVELOPMENT" {
		return mode
	}
	return ""
}

func (s *Challenges) Issue(ctx context.Context, input dto.ChallengeCreate, session *domain.Session) (domain.IssuedChallenge, error) {
	meta := domain.ChallengeMeta{Purpose: input.Purpose, Subject: domain.SubjectDigest(input.Email), UserID: "0", Epoch: "1"}
	if input.Purpose == "CHANGE_EMAIL" {
		meta.ActorID, meta.UserID, meta.Epoch = session.UserID, session.UserID, session.CredentialEpoch
	} else if input.Purpose == "RESET_PASSWORD" {
		account, state, err := s.repository.ResetAccount(ctx, input.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.IssuedChallenge{}, err
		}
		meta.UserID, meta.Epoch = account.UserID, account.Epoch
		if state != "ACTIVE" {
			meta.UserID = "0"
		}
	}
	id := platform.NewTraceID()
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return domain.IssuedChallenge{}, err
	}
	code := fmt.Sprintf("%06d", number.Int64())
	raw, _ := json.Marshal(meta)
	storeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	result, err := s.repository.Issue(storeCtx, id, meta, s.secrets.Digest(id+"\n"+code), raw)
	if err != nil {
		return domain.IssuedChallenge{}, domain.ErrChallengeStoreUnavailable
	}
	if result != int64(1) {
		return domain.IssuedChallenge{}, domain.ErrChallengeRateLimited
	}
	// 未知账号的找回仍返回相同挑战形状，避免邮箱枚举；实际已知账号才投递SMTP。
	if input.Purpose != "RESET_PASSWORD" || meta.UserID != "0" {
		if s.mailer.Send(ctx, input.Email, id, code) != nil {
			return domain.IssuedChallenge{}, domain.ErrSMTPDeliveryFailed
		}
	}
	return domain.IssuedChallenge{ID: id, Expires: time.Now().Add(10 * time.Minute).UTC()}, nil
}

func (s *Challenges) Verify(ctx context.Context, id, code, purpose, subject string) (domain.ChallengeMeta, error) {
	var meta domain.ChallengeMeta
	if !domain.ValidOperationID(id) || len(code) != 6 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return meta, domain.ErrChallengeInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	raw, err := s.repository.Meta(ctx, id)
	if err != nil {
		return meta, domain.ErrChallengeStoreUnavailable
	}
	value, ok := raw.(string)
	if !ok || json.Unmarshal([]byte(value), &meta) != nil || meta.Purpose != purpose || (subject != "" && meta.Subject != subject) {
		return meta, domain.ErrChallengeInvalid
	}
	result, err := s.repository.Verify(ctx, id, s.secrets.Digest(id+"\n"+code))
	if err != nil {
		return meta, domain.ErrChallengeStoreUnavailable
	}
	if result != int64(1) {
		return meta, domain.ErrChallengeInvalid
	}
	return meta, nil
}

// credential.go：保留原实现，按业务合并文件。

type Credentials struct {
	db             *sql.DB
	repository     *repository.Credentials
	authentication *Authentication
	challenges     *Challenges
}

func NewCredentials(db *sql.DB, repo *repository.Credentials, authentication *Authentication, challenges *Challenges) *Credentials {
	return &Credentials{db: db, repository: repo, authentication: authentication, challenges: challenges}
}

func (s *Credentials) Retry(ctx context.Context, id, kind string, fingerprint []byte, token string) (domain.CredentialRetry, bool, error) {
	receipt, err := s.repository.Receipt(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		return domain.CredentialRetry{}, false, nil
	}
	if err != nil {
		return domain.CredentialRetry{}, true, err
	}
	// 已失效的原会话只证明这一个持久操作，不能据此创建新的凭据变更。
	session, err := s.authentication.Session(ctx, token)
	if err != nil || receipt.ActorID != session.UserID || receipt.Kind != kind || receipt.SessionHash != hex.EncodeToString(session.ID) || !bytes.Equal(fingerprint, receipt.Fingerprint) {
		return domain.CredentialRetry{}, true, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
	}
	return domain.CredentialRetry{Session: session, State: receipt.State}, true, nil
}

func (s *Credentials) Begin(ctx context.Context, change domain.CredentialChange) (string, error) {
	oldEpoch, valid := domain.CredentialEpoch(change.ExpectedEpoch)
	if !valid {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "CREDENTIAL_EPOCH_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	credential, err := s.repository.LockUser(ctx, tx, change.UserID)
	if err != nil {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "CREDENTIAL_INVALID"}
	}
	// 等user串行锁后先重读receipt，再消费挑战及检查旧epoch，避免同号并发误报409。
	receipt, err := s.repository.Receipt(ctx, tx, change.ID, true)
	if err == nil {
		if receipt.ActorID != change.UserID || receipt.Kind != change.Kind || !bytes.Equal(receipt.Fingerprint, change.Fingerprint) || (change.Session != nil && receipt.SessionHash != hex.EncodeToString(change.Session.ID)) {
			return "", domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		return receipt.State, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	if change.Proof != nil {
		subject, _ := hex.DecodeString(change.Proof.Subject)
		// 消费唯一键与凭据写在同一MySQL事务；Redis回放不能产生第二份消费。
		if s.repository.ConsumeProof(ctx, tx, change, oldEpoch, subject) != nil {
			return "", domain.RuleViolation{Kind: domain.StateConflict, Code: "CHALLENGE_ALREADY_CONSUMED"}
		}
	}
	if credential.State != "ACTIVE" || credential.Epoch != change.ExpectedEpoch || (len(change.OldHash) > 0 && !bytes.Equal(credential.Hash, change.OldHash)) {
		return "", domain.RuleViolation{Kind: domain.StateConflict, Code: "CREDENTIAL_CHANGED"}
	}
	sessionHash := ""
	if change.Session != nil {
		current, err := s.repository.LockSession(ctx, tx, change.Session.ID)
		if err != nil || current.State != "ACTIVE" || current.CredentialEpoch != credential.Epoch || !current.Expires.After(time.Now()) {
			return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
		}
		sessionHash = hex.EncodeToString(change.Session.ID)
	}
	scope := "credential:" + change.UserID
	target, _ := json.Marshal(map[string]string{"user_id": change.UserID, "session_hash": sessionHash, "scope_key": scope})
	poll := "/v1/account/operations/" + change.ID
	pending, _ := json.Marshal(dto.Operation{OperationId: change.ID, State: "PREPARING", PollUrl: &poll})
	payload, _ := json.Marshal(domain.SessionClosure{Kind: change.Kind, UserID: change.UserID, Scope: scope, Epoch: credential.Epoch})
	if err := s.repository.Prepare(ctx, tx, domain.CredentialPreparation{Change: change, Epoch: oldEpoch, Scope: scope, Target: target, Pending: pending, Payload: payload}); err != nil {
		return "", err
	}
	if tx.Commit() != nil {
		return "", errors.New("CREDENTIAL_COMMIT_UNKNOWN")
	}
	return "PREPARING", nil
}

// credential_change.go：保留原实现，按业务合并文件。

func (s *Credentials) ResetPassword(ctx context.Context, input dto.ResetPassword, fingerprint []byte) (string, error) {
	if !domain.ValidNewPassword(input.NewPassword) {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_POLICY_INVALID"}
	}
	proof, err := s.challenges.Verify(ctx, input.ChallengeId, input.Code, "RESET_PASSWORD", "")
	if err != nil {
		return "", err
	}
	if proof.UserID == "0" {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "CHALLENGE_INVALID"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)
	if err != nil {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_INVALID"}
	}
	return s.Begin(ctx, domain.CredentialChange{
		ID:            input.OperationId,
		Kind:          "RESET_PASSWORD",
		UserID:        proof.UserID,
		ExpectedEpoch: proof.Epoch,
		Proof:         &proof,
		ChallengeID:   input.ChallengeId,
		Fingerprint:   fingerprint,
		NewHash:       hash,
	})
}

func (s *Credentials) ChangePassword(ctx context.Context, session domain.Session, input dto.PasswordChange, fingerprint []byte) (string, error) {
	if !domain.ValidNewPassword(input.NewPassword) {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_POLICY_INVALID"}
	}
	hash, err := s.repository.Hash(ctx, session.UserID)
	if err != nil {
		return "", errors.New("CREDENTIAL_UNAVAILABLE")
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(input.CurrentPassword)) != nil {
		return "", domain.RuleViolation{Kind: domain.PermissionDenied, Code: "CURRENT_PASSWORD_INVALID"}
	}
	// bcrypt在事务外计算，事务内仍核对原hash/epoch，避免持锁计算拖慢并发登录。
	next, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)
	if err != nil {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_INVALID"}
	}
	return s.Begin(ctx, domain.CredentialChange{
		ID:            input.OperationId,
		Kind:          "CHANGE_PASSWORD",
		UserID:        session.UserID,
		ExpectedEpoch: input.ExpectedCredentialEpoch,
		Session:       &session,
		Fingerprint:   fingerprint,
		OldHash:       hash,
		NewHash:       next,
	})
}

func (s *Credentials) ChangeEmail(ctx context.Context, session domain.Session, input dto.EmailChange, fingerprint []byte) (string, error) {
	proof, err := s.challenges.Verify(ctx, input.ChallengeId, input.Code, "CHANGE_EMAIL", domain.SubjectDigest(input.NewEmail))
	if err != nil {
		return "", err
	}
	if proof.ActorID != session.UserID || proof.Epoch != session.CredentialEpoch {
		return "", domain.RuleViolation{Kind: domain.InvalidRequest, Code: "CHALLENGE_INVALID"}
	}
	return s.Begin(ctx, domain.CredentialChange{
		ID:            input.OperationId,
		Kind:          "CHANGE_EMAIL",
		UserID:        session.UserID,
		ExpectedEpoch: input.ExpectedCredentialEpoch,
		Session:       &session,
		Proof:         &proof,
		ChallengeID:   input.ChallengeId,
		Fingerprint:   fingerprint,
		NewEmail:      input.NewEmail,
	})
}

// signup.go：保留原实现，按业务合并文件。

type Signups struct {
	db         *sql.DB
	repository *repository.Signups
	challenges *Challenges
}

func NewSignups(db *sql.DB, repo *repository.Signups, challenges *Challenges) *Signups {
	return &Signups{db: db, repository: repo, challenges: challenges}
}

func (s *Signups) PublicResult(ctx context.Context, id, kind string, fingerprint []byte) (json.RawMessage, bool, error) {
	receipt, err := s.repository.PublicReceipt(ctx, id)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	if receipt.Kind != kind || !hmac.Equal(receipt.Fingerprint, fingerprint) {
		return nil, true, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
	}
	if len(receipt.Result) == 0 {
		return nil, true, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "OPERATION_PENDING"}
	}
	return receipt.Result, true, nil
}

func (s *Signups) Register(ctx context.Context, input dto.Signup, fingerprint []byte) (json.RawMessage, error) {
	// handler先续取原操作回执，再校验新登记，升级规则不会阻断历史成功操作的恢复。
	if !domain.ValidNewPassword(input.Password) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_POLICY_INVALID"}
	}
	meta, err := s.challenges.Verify(ctx, input.ChallengeId, input.Code, "SIGNUP", domain.SubjectDigest(input.Email))
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PASSWORD_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	subject, _ := hex.DecodeString(meta.Subject)
	// consumption只准INSERT；先创建用户，再一次写完整消费终态，任一步失败一起回滚。
	user, err := s.repository.InsertUser(ctx, tx, input.Email, input.DisplayName, hash)
	if err != nil {
		if repository.DuplicateKey(err) {
			_ = tx.Rollback()
			raw, found, err := s.PublicResult(ctx, input.OperationId, "SIGNUP", fingerprint)
			if found {
				return raw, err
			}
			// 只有完成邮箱验证码校验后才明确告知占用；唯一约束继续处理并发注册，不能改成覆盖原用户。
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "EMAIL_ALREADY_REGISTERED"}
		}
		return nil, err
	}
	if err := s.repository.Consume(ctx, tx, input.ChallengeId, subject, user); err != nil {
		if repository.DuplicateKey(err) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "CHALLENGE_ALREADY_CONSUMED"}
		}
		return nil, err
	}
	raw, _ := json.Marshal(OperationResult(input.OperationId, "SUCCEEDED", fmt.Sprint(user)))
	if s.repository.InsertReceipt(ctx, tx, input.OperationId, input.ChallengeId, user, fingerprint, raw) != nil {
		return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
	}
	if tx.Commit() != nil {
		return nil, errors.New("SIGNUP_COMMIT_UNKNOWN")
	}
	return raw, nil
}
