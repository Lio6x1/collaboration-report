# A01｜先让一个账号安全地登录，再完成验证码、注册与凭据变更

## 这节课要实现什么

新用户希望用邮箱注册，已有用户希望输入邮箱和密码登录。本课实现注册、登录以及找回密码、修改密码和邮箱的后端流程；先把最基本的登录链路讲清，再逐步加入验证码和凭据变更。

## 做完后能看到什么

接通后，合法验证码和密码可以完成注册，正确凭据登录后获得会话；错误密码、无效验证码或不符合规则的新密码会被拒绝。修改凭据还要关闭旧会话资格，可能先显示处理中，不能收到请求就宣称完成。

先在脑中走一遍这个例子：小林用自己能收邮件的邮箱申请验证码，提交验证码和符合规则的密码注册；之后用同一邮箱和密码登录，再用错误密码尝试登录。前两条正常流程要成功，最后一条必须拒绝，而且不能产生新会话。学习到本课末尾时，你应能指出这三个请求分别在哪里校验、在哪里读写数据库、在哪里返回结果；实际请求等下方说明的联调关口再运行。

## 先想清楚怎样实现

1. 先定义账号、会话与输入规则。
2. 再写读取账号、核对密码、事务内复核并保存会话的链路。
3. 随后接验证码投递与一次消费、注册和凭据变更，最后把结果交给HTTP返回。

## 怎样判断这一课完成

本课先登记各分支的输入与预期；A08写齐账号包后运行构建和离线规则，06具备真实数据库、Redis和SMTP后再实际注册、登录与改密。

## 开始写之前

本课接续 01 阶段：SQL和平台连接池已在01阶段亲写；生成HTTP DTO与RPC由支持步骤准备。继续在自己的 `collaboration-report-learning` 根目录写代码。本课结束能解释为什么登录成功必须晚于事务提交，以及为什么改密码返回 `PREPARING`。真实运行留到 A08 装配完成、06 阶段四服务齐备后；不要在半个 Go 包尚缺声明时把编译错误当作业务失败。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序、文件与分段操作 | 此时创建的原因 |
| --- | --- |
| [errors.go](../../write/036-errors.md) | 先给登录、验证码故障稳定身份，handler 不匹配驱动报错正文。 |
| [identity.go](../../write/037-identity.md) | 操作号与用途格式被注册、改密和后续身份接口共同使用。 |
| [identity_models.go](../../write/038-identity-models.md) | 定义会话、凭据、登录结果及业务拒绝；先有返回类型再写函数。 |
| [authentication_models.go](../../write/034-authentication-models.md) | 定义挑战、凭据变更计划、秘密摘要及密码规则。 |
| [authentication_queries.go](../../write/048-authentication-queries.md) | 将 SQL、Redis Lua、唯一约束查询集中成真实适配器。 |
| [smtp.go](../../write/045-smtp.md) | 补上挑战需要的邮件投递能力；这里成功的含义是 SMTP DATA 被确认。 |
| [authentication_service.go](../../write/059-authentication-service.md) | 用消费者接口编排登录，随后写挑战、注册与改密事务。 |
| [http_transport_handlers.go](../../write/078-http-transport-handlers.md) | Cookie、JSON、Origin、CSRF 与错误映射先备好，HTTP 才能调用用例。 |
| [authentication_handlers.go](../../write/070-authentication-handlers.md) | 接收生成 DTO，完成四条用户入口链路。 |
| [hashcost/main.go](../../write/085-main.md)、[seed/main.go](../../write/087-main.md) | 分别测本机 bcrypt 成本、为自己的隔离实例创建学习账号；不替代注册业务。 |

这是按功能积累一个包。`CredentialReceipt` 引用 A04 的 `OperationReceipt`，SQL 适配器引用 A02 的 `Queryer` 与 A07 的 `execute`；登录 handler 的来源捕获由 A08 定义。此处明确保留这些真实依赖，A08 才运行整个账号包检查，不引入临时空实现。

## 2. 写会话模型：浏览器秘密与数据库身份分开

在 `identity_models.go` 的 package/import 后写下会话、登录查询和返回声明；完整其余声明按对应操作页继续。

<!-- lesson-code: services/account/internal/domain/account/identity_models.go :: Session -->
````go
// Session是已持久化的会话事实；ID和CSRF只保存摘要，不包含浏览器持有的原始令牌。
type Session struct {
	ID              []byte
	UserID          string
	CSRF            []byte
	Epoch           uint64
	CredentialEpoch string
	State           string
	Expires         time.Time
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/identity_models.go :: LoginUser -->
````go
type LoginUser struct {
	ID              string
	DisplayName     string
	Hash            []byte
	Algorithm       string
	State           string
	CredentialEpoch string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/identity_models.go :: Credential -->
````go
type Credential struct {
	Hash          []byte
	Epoch         string
	State         string
	IdentityEpoch uint64
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/identity_models.go :: LoginResult -->
````go
// LoginResult只在会话提交成功后交给传输层；令牌不能写入数据库或日志。
type LoginResult struct {
	UserID       string
	DisplayName  string
	SessionToken string
	CSRFToken    string
	Expires      time.Time
}
````
<!-- /lesson-code -->

`Session.ID` 是令牌摘要，`CSRF` 是另一个令牌的摘要。`UserID` 是字符串，因为 HTTP 契约里的大整数编号不能丢精度。`Epoch` 表示该会话来源代次，`CredentialEpoch` 表示账号密码/邮箱代次，不能互换。`State` 与 `Expires` 分别处理主动撤销和自然过期。`LoginUser.Hash/Algorithm` 来自权威 user 表；`Credential.Hash/Epoch/State` 用于事务内第二次检查，`IdentityEpoch` 用于后面的身份来源证明。`LoginResult` 才拥有发给浏览器的原文 SessionToken/CSRFToken，它不能进入数据库和日志。

在 `authentication_models.go` 先写 `RandomToken` 与 `TokenHash`，再写校验函数。前者返回 `(原文, 摘要, error)`，32 个随机字节编码成无填充 URL Base64；后者拒绝非规范编码、不是 32 字节的值，再计算相同摘要。因此数据库里的 session.id 本身不能拿去当 Cookie 登录。

<!-- lesson-code: services/account/internal/domain/account/authentication_models.go :: RandomToken -->
````go
func RandomToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	return token, hash[:], nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/authentication_models.go :: TokenHash -->
````go
func TokenHash(value string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
		return nil, errors.New("SESSION_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/authentication_models.go :: ValidNewPassword -->
````go
// 新密码单独执行强度规则，登录/当前密码仍按原哈希验证，避免锁住已有账号。
// 大小写均属于英文字母类，但原文直接交给bcrypt，不能转小写或去除首尾字符。
func ValidNewPassword(value string) bool {
	if !ValidPassword(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) < 8 {
		return false
	}
	var letters, digits, symbols bool
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z':
			letters = true
		case character >= '0' && character <= '9':
			digits = true
		case unicode.IsPunct(character) || unicode.IsSymbol(character):
			symbols = true
		}
	}
	// 空格和中文文字可以保留在密码中，但不能冒充英文或特殊字符凑够类别。
	return letters && digits || letters && symbols || digits && symbols
}
````
<!-- /lesson-code -->

`ValidPassword` 只要求 UTF-8 字节长度 1–72，供旧密码登录/当前密码核验。`ValidNewPassword` 另外要求合法 UTF-8、至少 8 个字符、英文/数字/符号中至少两类。`abcdefgh` 不能作为新密码，已有该密码的账号仍可按原 hash 登录；`Abcd1234` 可作新密码。中文和空格保留原文，但不假充英文或符号类别；25 个三字节中文字符超过 bcrypt 的 72 字节上限，必须拒绝，不能截断。

## 3. 先声明消费者接口，再写查询和登录事务

在 `authentication_service.go` 的 imports 后先输入这两个接口。`context.Context` 贯穿取消/超时；仓储的锁内方法显式接收 `*sql.Tx`，所以不会偷偷换连接池。

<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Limiter -->
````go
// Limiter是用例实际需要的能力，不暴露Redis的通用命令接口。
type Limiter interface {
	Allow(context.Context, string) (bool, error)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: AuthenticationStore -->
````go
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
````
<!-- /lesson-code -->

`LoginUser(email)` 返回 hash、状态和凭据代次；查无邮箱是 `sql.ErrNoRows`，数据库故障仍是 error。`LockCredential(tx,userID)` 返回持锁时的权威凭据；`InsertSession(tx,session)` 只写入，不提交。`Session(hash)` 按摘要读会话；`Credential(userID)` 读取当前代次；`User(userID)` 读取 `/me` 的资料。`Allow(ctx,email)` 返回允许与否，Redis 不可用是 error，不能把故障当作允许。

转到仓储文件，按操作页写 `Authentication`、构造器和全部六个方法。重点先完成下面两个锁内方法，再回用例文件写 `Authentication`、`NewAuthentication` 与 `Login`。

<!-- lesson-code: services/account/internal/repository/account/authentication_queries.go :: Authentication.LockCredential -->
````go
func (r *Authentication) LockCredential(ctx context.Context, tx *sql.Tx, userID string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE", userID).Scan(&value.Hash, &value.Epoch, &value.State)
	return value, err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/authentication_queries.go :: Authentication.InsertSession -->
````go
func (r *Authentication) InsertSession(ctx context.Context, tx *sql.Tx, value domain.Session) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO session(id,user_id,csrf_hash,credential_epoch,state,expires_at,last_seen_at) VALUES(?,?,?,?,'ACTIVE',?,UTC_TIMESTAMP(6))", value.ID, value.UserID, value.CSRF, value.CredentialEpoch, value.Expires)
	return err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Authentication.Login -->
````go
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
````
<!-- /lesson-code -->

现在代入 `email=" Learner@Example.Test "`、正确原文密码，数据库用户 `id=17,state=ACTIVE,credential_epoch=4`：

1. 邮箱变成 `learner@example.test`，密码不 trim、不转小写。无效邮箱返回 `LOGIN_INPUT_INVALID`，尚未查数据库。
2. `LoginLimit.Allow` 用 Lua 在一分钟内先限制全局 100 次，再限制邮箱摘要键 10 次；总量先拒绝防止随机邮箱无限造键。执行器有一秒期限，拒绝为 `LOGIN_RATE_LIMITED`，Redis 故障为 `LOGIN_LIMITER_UNAVAILABLE`。
3. `LoginUser` 锁外读用户。不存在也拿装配层预生成的 dummy bcrypt hash 做同成本比较，最终同为 `LOGIN_INVALID`，避免明显时序枚举。
4. 密码正确后才随机生成两个独立 token。开启事务、`defer Rollback`，`LockCredential` 用 `FOR UPDATE` 锁 user 17。必须再次核对 state、epoch=4 和 hash 原字节；若另一改密已把 epoch 改成 5，旧密码比较成功也不能创建会话。
5. 同一 tx 插入摘要、CSRF 摘要、凭据代次及 12 小时到期时间；Commit 成功后返回原文 token。Insert/Commit 报错路径返回 `SESSION_COMMIT_UNKNOWN`，不会发 Cookie。提交回复可能在数据库提交后丢失，所以无法从错误断言“会话绝对未写入”。

随后输入 `Session`、`Authenticate`、`CurrentUser` 的完整方法。`Authenticate` 将查无会话、无效 token、过期、状态关闭、凭据代次变化都归为 `SESSION_INVALID`。普通数据库读取故障保留错误上下文，不能误报 401 让前端丢弃仍有效登录态。`CurrentUser` 再核对 ACTIVE 与同一凭据代次，避免旧会话读到不该继续展示的账号投影。

<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Authentication.Authenticate -->
````go
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
````
<!-- /lesson-code -->

## 4. 第二遍写挑战和注册：Redis 校验与 SQL 一次消费各负其责

先在模型文件写 `ChallengeMeta/IssuedChallenge/PublicReceipt/Secrets`。`Purpose` 绑定 SIGNUP、RESET_PASSWORD 或 CHANGE_EMAIL；`Subject` 是邮箱 SHA-256，`UserID/Epoch` 固定核验时账号，`ActorID` 把换邮箱挑战绑到发起人。`Secrets.Digest` 是带秘密 key 的 HMAC；注册指纹不能存原文密码和验证码。

回用例文件输入 `ChallengeMailer`、`ChallengeStore`、`Challenges` 及构造器。邮件接口 `Send(ctx,email,id,code) error` 只负责投递；`DeliveryMode()` 只返回通道枚举。仓储 `ResetAccount(email)` 同时返回元数据、状态和 error，`Issue` 原子保存摘要和元数据，`Meta/Verify` 用挑战 ID 读取并核验。Redis 客户端的 `any` 在这里必须检查实际 string/int64 类型，不能直接断言成功。

<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: ChallengeStore -->
````go
// ChallengeStore限制验证码存储依赖；业务不接收Redis通用客户端，也不隐藏SQL/Redis故障。
type ChallengeStore interface {
	ResetAccount(context.Context, string) (domain.ChallengeMeta, string, error)
	Issue(context.Context, string, domain.ChallengeMeta, string, []byte) (any, error)
	Meta(context.Context, string) (any, error)
	Verify(context.Context, string, string) (any, error)
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Challenges.Issue -->
````go
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
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Challenges.Verify -->
````go
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
````
<!-- /lesson-code -->

在仓储操作页依次写两段 Lua 和 `Challenges` 方法：全局一分钟 100 次、同用途同邮箱冷却 60 秒，挑战 600 秒过期；错误尝试最多五次。`Verify` 成功不删除 Redis 挑战，它只证明验证码正确；真正“只消费一次”由后面的 MySQL `verification_consumption` 唯一键保证。

例：RESET_PASSWORD 查不到邮箱时仍发同形挑战 ID/到期时间，元数据 UserID=0、不投递邮件。已知账号 SMTP 失败是 `SMTP_DELIVERY_FAILED`，Redis 失败是 `CHALLENGE_STORE_UNAVAILABLE`；错误验证码是 400，限流是 429，依赖故障是 503。换邮箱必须先由 handler 完成当前会话/Origin/CSRF 检查，再把会话传给 Issue；不能直接给 nil session 调用该目的。

现在写 `smtp.go` 全部分段，顺序是配置/本地凭据/TLS 辅助函数→Send→LOGIN Auth。development 只准明确 loopback IP 且不加载凭据；tls/starttls 要核验证书和主机名，再发 AUTH。每命令最多 3 秒，总共最多 6 秒，取消会关闭连接。`writer.Close()` 收到 DATA 最终确认才算接收；随后 QUIT 失败不推翻已确认事实。投递成功只说明邮件服务器接受，不代表用户收件箱可见。

<!-- lesson-code: services/account/internal/client/smtp.go :: SMTP.Send -->
````go
func (s *SMTP) Send(ctx context.Context, email, id, code string) error {
	if s == nil || s.cfg == nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	// 在建立连接之前拒绝完整邮箱与头注入；不能依赖handler永远是唯一调用者。
	if !domain.ValidEmail(email) || id == "" || code == "" || strings.ContainsAny(email+id+code, "\r\n\x00") {
		return errors.New("SMTP_INPUT_INVALID")
	}
	if ctx.Err() != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	cfg := *s.cfg
	host, security, err := smtpConfiguration(cfg)
	if err != nil {
		return err
	}
	var credentials smtpCredentials
	var tlsConfig *tls.Config
	if security != "development" {
		credentials, err = readSMTPCredentials(cfg.CredentialsFile)
		if err != nil {
			return err
		}
		tlsConfig, err = smtpTLSConfig(host, cfg.CAFile)
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, smtpTotalTimeout)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	defer conn.Close()
	// net/smtp没有ctx参数。取消主动关闭底层连接，避免用户退出后仍等到命令期限。
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()
	command := func(step func() error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		deadline := time.Now().Add(smtpCommandTimeout)
		if absolute, ok := ctx.Deadline(); ok && absolute.Before(deadline) {
			deadline = absolute
		}
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
		return step()
	}
	var transport net.Conn = conn
	if security == "tls" {
		encrypted := tls.Client(conn, tlsConfig)
		if command(func() error { return encrypted.HandshakeContext(ctx) }) != nil {
			return errors.New("SMTP_TLS_FAILED")
		}
		transport = encrypted
	}
	var client *smtp.Client
	if command(func() error { client, err = smtp.NewClient(transport, host); return err }) != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	defer client.Close()
	if command(func() error { return client.Hello("localhost") }) != nil {
		return errors.New("SMTP_UNAVAILABLE")
	}
	if security == "starttls" {
		available, _ := client.Extension("STARTTLS")
		// 远程服务器没有升级能力或升级失败必须中止，不能回退明文MAIL/AUTH。
		if !available || command(func() error { return client.StartTLS(tlsConfig) }) != nil {
			return errors.New("SMTP_TLS_FAILED")
		}
	}
	if security != "development" {
		state, encrypted := client.TLSConnectionState()
		if !encrypted || !state.HandshakeComplete || len(state.VerifiedChains) == 0 {
			return errors.New("SMTP_TLS_FAILED")
		}
		available, mechanisms := client.Extension("AUTH")
		var auth smtp.Auth
		for _, mechanism := range strings.Fields(mechanisms) {
			if mechanism == "PLAIN" {
				auth = smtp.PlainAuth("", credentials.Username, credentials.Password, host)
				break
			}
			if mechanism == "LOGIN" {
				auth = &smtpLoginAuth{host: host, credentials: credentials}
			}
		}
		if !available || auth == nil || command(func() error { return client.Auth(auth) }) != nil {
			return errors.New("SMTP_AUTH_FAILED")
		}
	}
	if command(func() error { return client.Mail(cfg.From) }) != nil || command(func() error { return client.Rcpt(email) }) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	var writer io.WriteCloser
	if command(func() error { writer, err = client.Data(); return err }) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	// 真实邮件补齐Date/Message-ID和MIME头；消息身份由本机生成，不能把用户字段拼进头。
	senderDomain := cfg.From[strings.LastIndexByte(cfg.From, '@')+1:]
	message := fmt.Sprintf("Date: %s\r\nMessage-ID: <%s@%s>\r\nFrom: %s\r\nTo: %s\r\nSubject: Collaboration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nChallenge: %s\r\nCode: %s\r\nExpires: 10 minutes\r\n", time.Now().UTC().Format(time.RFC1123Z), platform.NewTraceID(), senderDomain, cfg.From, email, id, code)
	if command(func() error { _, err := io.WriteString(writer, message); return err }) != nil || command(writer.Close) != nil {
		return errors.New("SMTP_DELIVERY_FAILED")
	}
	// Close收到DATA的最终250才确认服务器接收。QUIT失败不推翻已确认事实，
	// 也不重发验证码；所有网络/认证错误仅返回固定码，不泄露服务器原文或授权码。
	_ = command(client.Quit)
	return nil
}
````
<!-- /lesson-code -->

接着完成 `Signups.PublicResult` 与 `Signups.Register` 以及四个 Signups SQL 方法。

<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Signups.Register -->
````go
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
````
<!-- /lesson-code -->

代入 `operation_id=signup-course-0001`、正确 SIGNUP 挑战、邮箱 `new@example.test`、显示名 `小林`、新密码 `Abcd1234`：先验强度和挑战，锁外 bcrypt，再在一个 tx 内 InsertUser→Consume→InsertReceipt→Commit。Consume 是 INSERT，不允许覆盖旧消费。邮箱唯一冲突先续取同号同指纹成功回执；不存在原回执才返回 `EMAIL_ALREADY_REGISTERED`。挑战唯一冲突是 `CHALLENGE_ALREADY_CONSUMED`。回执与用户同事务，使响应丢失后同号请求可返回原成功事实；同号换密码/验证码改变 HMAC 指纹，应为 `OPERATION_ID_CONFLICT`。Commit 未确认是 `SIGNUP_COMMIT_UNKNOWN`，客户端保留原操作号续取，不新造账号。

## 5. 第三遍写改密/邮箱：先封住旧资格，再异步确认关闭

模型 `CredentialChange` 携带 ID、Kind、UserID、ExpectedEpoch、可选原 Session、挑战 Proof、Fingerprint、新/旧 hash、新邮箱；`CredentialPreparation` 汇总 scope、target、pending result 和 worker payload。Epoch 用规范非零十进制字符串解析，拒绝 `04`、零和最大值，避免代次越界或同值异形。

在同一用例文件依次输入 `Credentials`、构造器、Retry、Begin、ResetPassword、ChangePassword、ChangeEmail，仓储对应写 Receipt/LockUser/ConsumeProof/LockSession/Prepare。

<!-- lesson-code: services/account/internal/logic/account/authentication_service.go :: Credentials.Begin -->
````go
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
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/authentication_queries.go :: Credentials.Prepare -->
````go
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
````
<!-- /lesson-code -->

推演已登录用户 17、ExpectedEpoch=`4` 改密：锁外校验当前密码并算新 hash→tx 锁 user→先读同号 receipt→消费证明（有才做）→重检原 epoch/hash→锁原 session→写 PREPARING 回执→user 改 REVOKING 且 credential_epoch+1→旧会话 REVOKING、epoch+1→关 identity_admission→credential:17 gate BLOCKED→authorization_change FENCING→持久 CREDENTIAL_CHANGE task→更新 hash→提交，返回 `PREPARING`。

同号请求等到 user 锁后先读回执，避免第一次已经加 epoch 后，第二次错误得到 `CREDENTIAL_CHANGED`。真换 operation ID 但继续拿 epoch=4 就应冲突。换邮箱只有最终邮箱 UPDATE 的唯一冲突确定映射 409；准备或提交其他故障保留未知语义。这里没有跨服务数据库事务，旧协作输出是否停止要由 A07 worker 获得 Document 确认；提交新 hash 不等于旧权限已经全部收回。

`Retry` 允许已经失效的原会话读取这一个既有操作，绑定 actor、kind、session hash、fingerprint，handler 仍核 CSRF；不能用恢复能力新建另一次改密。ChangePassword handler 先尝试续取，再暂存认证错误、再试一次续取，才决定向浏览器输出，覆盖并发同号请求恰好使旧会话失效的窗口。

## 6. 接到真实 HTTP：DTO、Cookie 与路由分层

先完成 httpx 操作页：`ReadJSON` 限 16384 字节，只收单个对象、拒未知字段、按生成的 `dto.RequiredFields` 检查 required/null；未给布尔不能自动成为 false 撤权。`Origin` 精确匹配配置；`ValidCSRF` 将请求头原文摘要化并常量时间比较。Cookie 都为 Secure、Path=/、SameSite=Strict；Session HttpOnly=true，独立 CSRF Cookie 可被前端读出恢复请求头。正常浏览器联调必须走 HTTPS。

<!-- lesson-code: services/account/internal/handler/account/authentication_handlers.go :: Authentication.Login -->
````go
func (h *Authentication) Login(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Login
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	result, err := h.logic.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	// logic已提交会话；只有此时才交付令牌。CSRF Cookie用于刷新后恢复请求头。
	httpx.SetCookie(w, httpx.SessionCookie, result.SessionToken, result.Expires, true)
	httpx.SetCookie(w, httpx.CSRFCookie, result.CSRFToken, result.Expires, false)
	platform.WriteJSON(w, 200, dto.LoginResult{UserId: result.UserID, DisplayName: result.DisplayName, CsrfToken: result.CSRFToken, ExpiresAt: result.Expires.Format(time.RFC3339)})
}
````
<!-- /lesson-code -->

`POST /v1/account/login` → `Authentication.Login` handler → 生成 `dto.Login` → logic.Login → repository/限流器 → 提交 → 两个 Cookie 和 dto.LoginResult。错误时没有 Cookie。`GET /v1/account/me` 先 Authenticate，再 CurrentUser。`POST /v1/account/challenges`、`/signup` 接前述挑战/注册；`/password-resets`、`/password`、`/email` 接凭据用例。路由完整代码在 A08 写 `RegisterAuthentication/RegisterChallenges/RegisterCredentials`，依赖在 `svc.NewAuthentication/NewChallenges/NewSignups/NewCredentials` 创建，`bootstrap.RegisterAccount` 注册。不要提前手写另一份 DTO 或把 SQL 放入 handler。

## 7. 验证：先登记检查项，包齐后执行

本课只完成输入和代码推演；以下命令在 A08 包齐后执行。命令针对自己的学习工程，不指向体验实例。

```powershell
Set-Location "$lessonRoot\services\account"
go test ./internal/domain/account -run '^(TestPasswordLengthIsUTF8Bytes|TestEmailRejectsDisplayNameAndOversize|TestNewPasswordPolicyKeepsLegacyInputSeparate)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/logic/account -run '^(TestWeakNewPasswordsRejectedBeforeChallengesOrStorage|TestAuthenticationDistinguishesInvalidIdentityFromReadFailure|TestChallengeFailuresKeepTheirCategoryAcrossBusinessEntries)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/handler/httpx -run '^(TestMissingOrNullBooleanCannotBecomeARevoke|TestKnownAccountFailuresKeepTheirCodeWithoutLeakingDriverDetails)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
go test ./internal/client -run '^(TestSMTPEncryptedAuthenticationAndDataConfirmation|TestSMTPRejectsTLSFailureWithoutSendingCredentials|TestSMTPDevelopmentAndCancellation|TestSMTPCommandDeadlineStopsStalledServer)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

前两组是规则/替身用例检查；SMTP 组自己创建本机测试监听器和证书，不需要四业务服务或真实邮箱，但需要本机可监听。每个指定名字应出现 RUN/PASS，`[no tests to run]` 不算通过。应证实弱新密码在接触挑战/存储前拒绝，查无身份是无效而 SQL 故障仍为不可用，验证码分类保留，TLS 失败不发送凭据，DATA 失败不能报告成功。依赖尚未缓存时先完成00阶段锁定依赖安装；不要用删测试避开编译缺声明。

`go run ./cmd/hashcost` 是可选本机成本检查，输出 cost=10、samples=20、p95_ms、预算 500ms 与 PASS/FAIL；慢机器 FAIL 应检查 CPU 占用与环境，不降低密码验证规则。`go run ./cmd/seed --config <自己的账号配置路径>` 只能在06自己的隔离实例迁移好后执行；它生成自己的 `.runtime/login-seed.json`，EXISTS 表示保留既有文件，不能拿体验实例种子反复覆盖。

真实验收等06：验证码→注册→登录→me→改密后旧登录态拒绝→原 operation_id 续取结果→新密码重新登录。自己的四服务READY、开发SMTP收件夹具和锁定Node依赖齐备后，在学习工程根目录执行：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/account/challenge.test.mjs tests/account/credential.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

精确名字包括 `signup uses real SMTP code and consumed challenge cannot register again`、`wrong purpose and bounded guesses never consume an unrelated challenge`、`verified email collision returns a stable conflict without consuming the challenge or closing the old session`、`password reset closes old epoch and Redis restore cannot replay consumed verification`、`authenticated password change, verified email change and session expiry all check current credentials`。预期所有实际用例PASS、skip0，核验证码一次消费、错用途/有界猜测、邮箱占用不消耗挑战或误关会话、凭据代次失效及Redis回放拒绝。本次文档编写未执行。检查 SQL 持久事实和恢复结果，不能以本课替身测试宣称四服务流程通过。401 检查 Cookie/账号状态/代次；403 检查 HTTPS、Origin、CSRF；429 等待实际冷却；503 按稳定故障码区分 Redis、SMTP 与提交未知，保持原操作号重试。

## 8. 章末核对与自测

完整答案：[模型](../../source/034-authentication-models.md)、[会话模型](../../source/038-identity-models.md)、[错误](../../source/036-errors.md)、[编号](../../source/037-identity.md)、[仓储](../../source/048-authentication-queries.md)、[邮件](../../source/045-smtp.md)、[用例](../../source/059-authentication-service.md)、[HTTP工具](../../source/078-http-transport-handlers.md)、[入口](../../source/070-authentication-handlers.md)、[成本命令](../../source/085-main.md)、[种子命令](../../source/087-main.md)。按操作页补齐所有辅助声明后再比对全文，不把正文的重要函数当作文件全部内容。

自测：为什么 bcrypt 不在 user 锁内？答案是昂贵计算拖慢并发，事务内用 hash/epoch 再验保证锁外结果仍适用。为什么 Redis 验证成功后不立即删除挑战？答案是 SQL 消费与业务提交必须同生共死，Redis 删除无法参加该事务。为什么改密没有直接返回 SUCCEEDED？答案是旧输出关闭需要跨服务持久确认，A07 会完成这个闭环。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
  'services/account/internal/domain/account/errors.go',
  'services/account/internal/domain/account/identity.go',
  'services/account/internal/domain/account/identity_models.go',
  'services/account/internal/domain/account/authentication_models.go',
  'services/account/internal/repository/account/authentication_queries.go',
  'services/account/internal/client/smtp.go',
  'services/account/internal/logic/account/authentication_service.go',
  'services/account/internal/handler/httpx/http_transport_handlers.go',
  'services/account/internal/handler/account/authentication_handlers.go',
  'services/account/cmd/hashcost/main.go',
  'services/account/cmd/seed/main.go'
)
foreach ($path in $paths) {
  New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
  if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
