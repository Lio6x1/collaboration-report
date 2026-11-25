# 分步编写 `services/account/internal/logic/account/authentication_service.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/059-authentication-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排登录、会话验证、验证码签发、注册和凭据变更。

昂贵的 bcrypt 放在行锁外，提交前重读凭据；改密不能只改一行密码，还必须持久关闭旧来源并支持原号恢复。

写之前先按这条执行路线理解各段的连接：

1. Login 规范化邮箱、限流、锁外校验密码，然后在 user 行锁内比较状态/epoch/hash，写会话并提交后交付原文令牌。
2. Authenticate 区分查无会话与基础设施失败，再检查会话有效期、用户状态和凭据代次。
3. Credentials.Begin 先锁user再查原回执，消费证明并重验会话后写入关闭计划；重复原号先返回旧结果。
4. Signups.Register 同事务创建user、消费挑战和写SUCCEEDED回执；失败整体回滚。

本文件需要守住的边界：

- Commit 错误属于未知结果，不能向浏览器发成功登录Cookie，也不能更换操作号盲目重试。
- RESET_PASSWORD 对未知邮箱仍返回相同挑战形状；不能通过投递通道提示泄露存在性。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/authentication_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/authentication_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/authentication_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/authentication_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:555 -->
````go
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

````

## 第 2 步：Limiter

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

Limiter是用例实际需要的能力，不暴露Redis的通用命令接口。

本步声明：`type Limiter interface {`。

**这一段为什么这样写：** 认证用例只问这次邮箱是否允许尝试，不暴露Redis键和Lua细节。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Allow</code> | <code>(context.Context, string) (bool, error)</code> | 第1项context传取消/1秒预算，第2项string是已小写去空白的邮箱；bool=true才允许登录，false/nil代表限流，error代表限流设施不可用，不能失败放行。 |

<!-- write-step: 2 bytes=555:711 -->
````go
// Limiter是用例实际需要的能力，不暴露Redis的通用命令接口。
type Limiter interface {
	Allow(context.Context, string) (bool, error)
}

````

## 第 3 步：AuthenticationStore

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

AuthenticationStore由消费者声明，只暴露认证所需查询；事务连接仍由用例传入，
适配器不能为锁内查询另开连接。小接口也让“查无会话”和“数据库不可用”可独立验证。

本步声明：`type AuthenticationStore interface {`。

**这一段为什么这样写：** 六个方法正好覆盖登录及会话重验；接口放在消费者一侧，测试可分别模拟不存在与查询失败。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>LoginUser</code> | <code>(context.Context, string) (domain.LoginUser, error)</code> | 输入context和规范邮箱；返回该邮箱的LoginUser（含bcrypt哈希、算法、状态、凭据代次）及查询error，sql.ErrNoRows由logic映射统一登录失败。 |
| <code>LockCredential</code> | <code>(context.Context, *sql.Tx, string) (domain.Credential, error)</code> | 输入context、调用者开启的原sql.Tx、用户ID字符串；对user FOR UPDATE，返回当前hash/epoch/state与error，供锁内核对锁外密码验证仍有效。 |
| <code>InsertSession</code> | <code>(context.Context, *sql.Tx, domain.Session) error</code> | 输入context、原sql.Tx、待保存domain.Session；仅插入摘要/用户/凭据代次/有效期，error=nil表示语句成功，事务提交仍由Login负责。 |
| <code>Session</code> | <code>(context.Context, &#91;&#93;byte) (domain.Session, error)</code> | 输入context、32字节令牌SHA256摘要；返回Session与error；查不到保留sql.ErrNoRows，数据库故障不可伪装成无效会话。 |
| <code>Credential</code> | <code>(context.Context, string) (domain.Credential, error)</code> | 输入context、会话所属用户ID；返回用户当前state、credential epoch和identity epoch及error，用于使尚未到期的旧凭据会话失效。 |
| <code>User</code> | <code>(context.Context, string) (domain.User, error)</code> | 输入context、会话所属用户ID；返回含展示名/邮箱/状态/代次的User与error；CurrentUser随后再检查ACTIVE与凭据代次。 |

<!-- write-step: 3 bytes=711:1361 -->
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

## 第 4 步：Authentication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 42 行。

Authentication编排登录与会话校验；SQL负责权威状态，限流器负责请求频率。
它只返回业务结果，Cookie和HTTP状态由handler转换，便于其它入口复用同一规则。

本步声明：`type Authentication struct {`。

**这一段为什么这样写：** 把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | Account私有库连接池，只用于开启登录事务；repo锁内方法须使用该事务。 |
| <code>repository</code> | <code>AuthenticationStore</code> | 满足AuthenticationStore的查询适配器，执行实际user/session查询。 |
| <code>limiter</code> | <code>Limiter</code> | 按规范邮箱控制尝试频率的能力；出错不能当允许。 |
| <code>dummyHash</code> | <code>&#91;&#93;byte</code> | 启动时生成的同成本bcrypt假哈希；邮箱不存在或算法不支持时仍做比较，降低邮箱枚举侧信道。 |

<!-- write-step: 4 bytes=1361:1694 -->
````go
// Authentication编排登录与会话校验；SQL负责权威状态，限流器负责请求频率。
// 它只返回业务结果，Cookie和HTTP状态由handler转换，便于其它入口复用同一规则。
type Authentication struct {
	db         *sql.DB
	repository AuthenticationStore
	limiter    Limiter
	dummyHash  []byte
}

````

## 第 5 步：NewAuthentication

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

NewAuthentication显式接收SQL连接、查询适配器和限流能力，业务不直接依赖Redis命令。
dummyHash由装配层预先生成，未知邮箱也走bcrypt校验；不能每次请求临时生成它。

本步声明：`func NewAuthentication(db *sql.DB, repo AuthenticationStore, limiter Limiter, dummyHash []byte) *Authentication`。

**这一段为什么这样写：** 显式注入依赖并复用启动生成的dummyHash，避免每次登录额外生成bcrypt。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | Store.DB，即本服务事务所属连接池。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>AuthenticationStore</code> | 与db同池的AuthenticationStore实现。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>limiter</code> | <code>Limiter</code> | Redis Lua的窄接口适配器或测试替身。<br>在此声明中的实际使用：<code>limiter: limiter</code> |
| <code>dummyHash</code> | <code>&#91;&#93;byte</code> | 预先生成cost=10的有效bcrypt字节，不是用户凭据。<br>在此声明中的实际使用：<code>dummyHash: dummyHash</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authentication</code> | 已装配的认证业务服务；此函数不查询数据库或创建会话。<br>实际返回表达式：<code>&amp;Authentication{db: db, repository: repo, limiter: limiter, dummyHash: dummyHash}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authentication{db: db, repository: repo, limiter: limiter, dummyHash: dummyHash}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1694:2123 -->
````go
// NewAuthentication显式接收SQL连接、查询适配器和限流能力，业务不直接依赖Redis命令。
// dummyHash由装配层预先生成，未知邮箱也走bcrypt校验；不能每次请求临时生成它。
func NewAuthentication(db *sql.DB, repo AuthenticationStore, limiter Limiter, dummyHash []byte) *Authentication {
	return &Authentication{db: db, repository: repo, limiter: limiter, dummyHash: dummyHash}
}

````

## 第 6 步：Authentication.Login

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

Login负责登录用例，不设置HTTP Cookie：handler只在本用例确认提交后发送凭据。
流程为输入/限流→锁外密码校验→锁内凭据重检与会话写入→提交→返回原文token。

本步声明：`func (s *Authentication) Login(ctx context.Context, email, password string) (domain.LoginResult, error)`。

**这一段为什么这样写：** 锁外做昂贵bcrypt，锁内重验hash与epoch，兼顾并发性能和改密竞争安全。

1. 先校验邮箱和1至72字节密码；Limiter.Allow=false返回ErrLoginLimited，限流器故障继续报错。
2. LoginUser查不到时使用dummyHash仍做一次bcrypt；不存在、密码错、非ACTIVE、非bcrypt统一ErrLoginInvalid。
3. 独立生成session与CSRF两份随机令牌，只保留摘要用于SQL；开启事务并defer Rollback。
4. LockCredential取user行锁后要求state/epoch/hash仍与锁外结果一致，再InsertSession。
5. Commit失败报告SESSION_COMMIT_UNKNOWN，不设置Cookie；成功才返回两份原文令牌。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次登录取消/截止时间，传递限流及全部SQL。<br>在此声明中的实际使用：<code>s.limiter.Allow(ctx, email)</code>；<code>s.repository.LoginUser(ctx, email)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.LockCredential(ctx, tx, user.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>email</code> | <code>string</code> | 用户输入邮箱；只对邮箱TrimSpace并ToLower用于email_key。<br>在此声明中的实际使用：<code>email = strings.ToLower(strings.TrimSpace(email))</code>；<code>strings.TrimSpace(email)</code>；<code>domain.ValidEmail(email)</code>；<code>s.limiter.Allow(ctx, email)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>password</code> | <code>string</code> | 用户输入密码原文，按UTF8字节检查后直接交bcrypt，不去空白也不改大小写。<br>在此声明中的实际使用：<code>domain.ValidPassword(password)</code>；<code>&#91;&#93;byte(password)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.LoginResult</code> | 仅Commit成功时含用户ID、展示名、原文SessionToken/CSRFToken及12小时到期；失败返回空值。<br>实际返回表达式：<code>result</code>；<code>domain.LoginResult{UserID: user.ID, DisplayName: user.DisplayName, SessionToken: token, CSRFToken: csrf, Expires: expires}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 输入非法/限流/统一凭据失败/基础设施失败，或SESSION_COMMIT_UNKNOWN；未知提交仍不返回令牌。<br>实际返回表达式：<code>domain.ErrLoginInput</code>；<code>err</code>；<code>domain.ErrLoginLimited</code>；<code>domain.ErrLoginInvalid</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidEmail(email) &#124;&#124; !domain.ValidPassword(password)</code>。条件成立时执行 <code>return result, domain.ErrLoginInput</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return result, domain.ErrLoginLimited</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; user.Algorithm == &quot;bcrypt&quot;</code>。
- 检查 <code>err != nil &#124;&#124; !match &#124;&#124; user.State != &quot;ACTIVE&quot; &#124;&#124; user.Algorithm != &quot;bcrypt&quot;</code>。条件成立时执行 <code>return result, domain.ErrLoginInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2123:3900 -->
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
````

## 第 7 步：Authentication.Login（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 103 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;LOGIN_RECHECK_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.State != &quot;ACTIVE&quot; &#124;&#124; current.Epoch != user.CredentialEpoch &#124;&#124; !bytes.Equal(current.Hash, user.Hash)</code>。条件成立时执行 <code>return result, domain.ErrLoginInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SESSION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return domain.LoginResult{UserID: user.ID, DisplayName: user.DisplayName, SessionToken: token, CSRFToken: csrf, Expires: expires}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3900:4920 -->
````go
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

## 第 8 步：Authentication.Session

插入位置：第 7 步代码之后，紧接追加；完成稿约第 124 行。

Session先验证并摘要化Cookie，再查库；token原文不成为SQL中的持久身份字段。

本步声明：`func (s *Authentication) Session(ctx context.Context, token string) (domain.Session, error)`。

**这一段为什么这样写：** 先把规范原文Cookie转换为摘要，避免原文作为数据库身份或查询参数长期传播。

1. TokenHash检查编码可往返与长度，再算SHA256。
2. repository.Session只查事实；状态/到期检查由Authenticate决定，恢复流程可复用未认证读取。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 会话查找请求上下文。<br>在此声明中的实际使用：<code>s.repository.Session(ctx, id)</code> |
| <code>token</code> | <code>string</code> | 浏览器持有的无padding Base64URL会话秘密，解码必须恰好32字节。<br>在此声明中的实际使用：<code>domain.TokenHash(token)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 按摘要查到的原会话事实；本方法允许读到CLOSED/REVOKING供原操作恢复。<br>由 <code>s.repository.Session(ctx, id)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>domain.Session{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 格式错为ErrSessionInvalid；查询错误保留，不能由本方法吞成认证失败。<br>由 <code>s.repository.Session(ctx, id)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.ErrSessionInvalid</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Session{}, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return s.repository.Session(ctx, id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4920:5266 -->
````go
// Session先验证并摘要化Cookie，再查库；token原文不成为SQL中的持久身份字段。
func (s *Authentication) Session(ctx context.Context, token string) (domain.Session, error) {
	id, err := domain.TokenHash(token)
	if err != nil {
		return domain.Session{}, domain.ErrSessionInvalid
	}
	return s.repository.Session(ctx, id)
}

````

## 第 9 步：Authentication.Authenticate

插入位置：第 8 步代码之后，紧接追加；完成稿约第 133 行。

Authenticate将Cookie原文转换为有效会话及当前身份代次；过期、停用或凭据变化统一视为无效。
返回的身份代次用于后续准入，不能替代目标业务在提交前的权限重检。

本步声明：`func (s *Authentication) Authenticate(ctx context.Context, token string) (domain.Authentication, error)`。

**这一段为什么这样写：** 把原会话事实和用户当前凭据连接起来，拒绝旧密码代次和已关闭账号。

1. Session失败仅对sql.ErrNoRows和ErrSessionInvalid转会话无效，其余故障保留原因。
2. 要求session.State=ACTIVE且Expires晚于现在。
3. Credential读取user当前状态/凭据代次并比较；相等后返回IdentityEpoch供后续准入和输出捕获。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次会话鉴权上下文，SQL失败须仍可辨识。<br>在此声明中的实际使用：<code>s.Session(ctx, token)</code>；<code>s.repository.Credential(ctx, value.UserID)</code> |
| <code>token</code> | <code>string</code> | 主会话Cookie原文，不是CSRF或恢复后的新身份。<br>在此声明中的实际使用：<code>s.Session(ctx, token)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Authentication</code> | Session及当前IdentityEpoch；只在error=nil时是当前可用身份。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 不存在、非ACTIVE、过期或凭据代次不匹配为ErrSessionInvalid；数据库故障包装原error供上层503。<br>实际返回表达式：<code>domain.ErrSessionInvalid</code>；<code>fmt.Errorf(&quot;read authentication session: %w&quot;, err)</code>；<code>fmt.Errorf(&quot;read authentication credential: %w&quot;, err)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; errors.Is(err, domain.ErrSessionInvalid)</code>。条件成立时执行 <code>return result, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, fmt.Errorf(&quot;read authentication session: %w&quot;, err)</code>。当前函数在这里结束。
- 检查 <code>value.State != &quot;ACTIVE&quot; &#124;&#124; !value.Expires.After(time.Now())</code>。条件成立时执行 <code>return result, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, fmt.Errorf(&quot;read authentication credential: %w&quot;, err)</code>。当前函数在这里结束。
- 检查 <code>credential.State != &quot;ACTIVE&quot; &#124;&#124; credential.Epoch != value.CredentialEpoch</code>。条件成立时执行 <code>return result, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5266:6773 -->
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

## 第 10 步：Authentication.CurrentUser

插入位置：第 9 步代码之后，紧接追加；完成稿约第 164 行。

CurrentUser读取当前账号资料并重检状态/凭据代次，避免旧会话返回改密前的有效身份。

本步声明：`func (s *Authentication) CurrentUser(ctx context.Context, session domain.Session) (domain.User, error)`。

**这一段为什么这样写：** 读资料后再次检查用户状态与凭据代次，避免改密竞争返回旧有效身份。

1. repository.User按session.UserID查本人。
2. 核ACTIVE与CredentialEpoch匹配后返回；此处不赋予组织角色。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 资料读取上下文。<br>在此声明中的实际使用：<code>s.repository.User(ctx, session.UserID)</code> |
| <code>session</code> | <code>domain.Session</code> | 已从真实Cookie读取的会话，UserID定位本人，CredentialEpoch用于资料返回前重检。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.CredentialEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.User</code> | 当前User资料；仅error=nil可输出。<br>实际返回表达式：<code>user</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 查无账号或状态/代次失效为ErrSessionInvalid；其他数据库error保留。<br>实际返回表达式：<code>domain.ErrSessionInvalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return user, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return user, err</code>。当前函数在这里结束。
- 检查 <code>user.State != &quot;ACTIVE&quot; &#124;&#124; user.CredentialEpoch != session.CredentialEpoch</code>。条件成立时执行 <code>return user, domain.ErrSessionInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return user, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6773:7373 -->
````go
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

````

## 第 11 步：ChallengeMailer

插入位置：第 10 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`type ChallengeMailer interface {`。

**这一段为什么这样写：** 验证码业务只关心邮件投递与固定通道枚举。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Send</code> | <code>(context.Context, string, string, string) error</code> | context控制取消；三个string依次是目标邮箱、challenge ID、6位验证码原文；error=nil仅表示SMTP已确认DATA。 |
| <code>DeliveryMode</code> | <code>() string</code> | 无输入，返回SMTP、DEVELOPMENT或空串，不能反映某邮箱是否实际存在。 |

<!-- write-step: 11 bytes=7373:7485 -->
````go
type ChallengeMailer interface {
	Send(context.Context, string, string, string) error
	DeliveryMode() string
}

````

## 第 12 步：ChallengeStore

插入位置：第 11 步代码之后，紧接追加；完成稿约第 186 行。

ChallengeStore限制验证码存储依赖；业务不接收Redis通用客户端，也不隐藏SQL/Redis故障。

本步声明：`type ChallengeStore interface {`。

**这一段为什么这样写：** 验证码存储接口保持SQL账户查询和Redis挑战状态各自的故障类别。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ResetAccount</code> | <code>(context.Context, string) (domain.ChallengeMeta, string, error)</code> | context与规范邮箱输入；输出ChallengeMeta中的用户/凭据代次、账号state和查询error；无账号由业务统一隐藏。 |
| <code>Issue</code> | <code>(context.Context, string, domain.ChallengeMeta, string, &#91;&#93;byte) (any, error)</code> | 输入context、挑战id、绑定meta、HMAC摘要、meta JSON；输出Redis int64(1)或拒绝值及error。 |
| <code>Meta</code> | <code>(context.Context, string) (any, error)</code> | context、挑战ID输入；输出Redis meta字符串或空串，以及基础设施error。 |
| <code>Verify</code> | <code>(context.Context, string, string) (any, error)</code> | context、挑战ID、待比对HMAC输入；输出int64(1)表示匹配，否则拒绝，error表示存储故障。 |

<!-- write-step: 12 bytes=7485:7894 -->
````go
// ChallengeStore限制验证码存储依赖；业务不接收Redis通用客户端，也不隐藏SQL/Redis故障。
type ChallengeStore interface {
	ResetAccount(context.Context, string) (domain.ChallengeMeta, string, error)
	Issue(context.Context, string, domain.ChallengeMeta, string, []byte) (any, error)
	Meta(context.Context, string) (any, error)
	Verify(context.Context, string, string) (any, error)
}

````

## 第 13 步：Challenges

插入位置：第 12 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`type Challenges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>ChallengeStore</code> | 使用 <code>ChallengeStore</code> 类型；其定义处的职责：验证码存储接口保持SQL账户查询和Redis挑战状态各自的故障类别。 |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。 |
| <code>mailer</code> | <code>ChallengeMailer</code> | 使用 <code>ChallengeMailer</code> 类型；其定义处的职责：验证码业务只关心邮件投递与固定通道枚举。 |

<!-- write-step: 13 bytes=7894:8005 -->
````go
type Challenges struct {
	repository ChallengeStore
	secrets    *domain.Secrets
	mailer     ChallengeMailer
}

````

## 第 14 步：NewChallenges

插入位置：第 13 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func NewChallenges(repo ChallengeStore, secrets *domain.Secrets, mailer ChallengeMailer) *Challenges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repo</code> | <code>ChallengeStore</code> | 使用 <code>ChallengeStore</code> 类型；其定义处的职责：验证码存储接口保持SQL账户查询和Redis挑战状态各自的故障类别。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>secrets: secrets</code> |
| <code>mailer</code> | <code>ChallengeMailer</code> | 使用 <code>ChallengeMailer</code> 类型；其定义处的职责：验证码业务只关心邮件投递与固定通道枚举。<br>在此声明中的实际使用：<code>mailer: mailer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Challenges</code> | 实际返回表达式：<code>&amp;Challenges{repository: repo, secrets: secrets, mailer: mailer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Challenges{repository: repo, secrets: secrets, mailer: mailer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8005:8183 -->
````go
func NewChallenges(repo ChallengeStore, secrets *domain.Secrets, mailer ChallengeMailer) *Challenges {
	return &Challenges{repository: repo, secrets: secrets, mailer: mailer}
}

````

## 第 15 步：Challenges.DeliveryMode

插入位置：第 14 步代码之后，紧接追加；完成稿约第 204 行。

只暴露实例配置的通道枚举；不能根据本次是否实际投递来透露找回邮箱是否注册。

本步声明：`func (s *Challenges) DeliveryMode() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>mode</code> |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.mailer == nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 检查 <code>mode == &quot;SMTP&quot; &#124;&#124; mode == &quot;DEVELOPMENT&quot;</code>。条件成立时执行 <code>return mode</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8183:8502 -->
````go
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

````

## 第 16 步：Challenges.Issue

插入位置：第 15 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`func (s *Challenges) Issue(ctx context.Context, input dto.ChallengeCreate, session *domain.Session) (domain.IssuedChallenge, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.ResetAccount(ctx, input.Email)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>s.mailer.Send(ctx, input.Email, id, code)</code> |
| <code>input</code> | <code>dto.ChallengeCreate</code> | 在此声明中的实际使用：<code>input.Purpose</code>；<code>input.Email</code> |
| <code>session</code> | <code>*domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.CredentialEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.IssuedChallenge</code> | 使用 <code>domain.IssuedChallenge</code> 类型；其定义处的职责：签发完成后只需把挑战编号和到期时间交给调用方；验证码原文由投递链路处理，不放进这个返回模型。<br>实际返回表达式：<code>domain.IssuedChallenge{}</code>；<code>domain.IssuedChallenge{ID: id, Expires: time.Now().Add(10 * time.Minute).UTC()}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.ErrChallengeStoreUnavailable</code>；<code>domain.ErrChallengeRateLimited</code>；<code>domain.ErrSMTPDeliveryFailed</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>input.Purpose == &quot;CHANGE_EMAIL&quot;</code>。
- 检查 <code>input.Purpose == &quot;RESET_PASSWORD&quot;</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return domain.IssuedChallenge{}, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.IssuedChallenge{}, err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.IssuedChallenge{}, domain.ErrChallengeStoreUnavailable</code>。当前函数在这里结束。
- 检查 <code>result != int64(1)</code>。条件成立时执行 <code>return domain.IssuedChallenge{}, domain.ErrChallengeRateLimited</code>。当前函数在这里结束。
- 检查 <code>input.Purpose != &quot;RESET_PASSWORD&quot; &#124;&#124; meta.UserID != &quot;0&quot;</code>。
- 检查 <code>s.mailer.Send(ctx, input.Email, id, code) != nil</code>。条件成立时执行 <code>return domain.IssuedChallenge{}, domain.ErrSMTPDeliveryFailed</code>。当前函数在这里结束。
- 此处通过 <code>return domain.IssuedChallenge{ID: id, Expires: time.Now().Add(10 * time.Minute).UTC()}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8502:10189 -->
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

## 第 17 步：Challenges.Verify

插入位置：第 16 步代码之后，紧接追加；完成稿约第 255 行。

本步声明：`func (s *Challenges) Verify(ctx context.Context, id, code, purpose, subject string) (domain.ChallengeMeta, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>s.repository.Meta(ctx, id)</code>；<code>s.repository.Verify(ctx, id, s.secrets.Digest(id+&quot;\n&quot;+code))</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(id)</code>；<code>s.repository.Meta(ctx, id)</code>；<code>s.repository.Verify(ctx, id, s.secrets.Digest(id+&quot;\n&quot;+code))</code>；<code>id+&quot;\n&quot;</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>len(code)</code>；<code>strings.IndexFunc(code, func(r rune) bool { return r &lt; &#x27;0&#x27; &#124;&#124; r &gt; &#x27;9&#x27; })</code>；<code>id+&quot;\n&quot;+code</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>meta.Purpose != purpose</code> |
| <code>subject</code> | <code>string</code> | 在此声明中的实际使用：<code>subject != &quot;&quot;</code>；<code>meta.Subject != subject</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ChallengeMeta</code> | 使用 <code>domain.ChallengeMeta</code> 类型；其定义处的职责：把验证码和用途、接收对象、账号代次及发起人一起绑定；只比较验证码数字会让原本有效的证明被挪到另一项操作。<br>实际返回表达式：<code>meta</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrChallengeInvalid</code>；<code>domain.ErrChallengeStoreUnavailable</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(id) &#124;&#124; len(code) != 6 &#124;&#124; strings.IndexFunc(code, func(r rune) bool { return r &lt; &#x27;0&#x27; &#124;&#124; r &gt; &#x27;9&#x27; }) &gt;= 0</code>。条件成立时执行 <code>return meta, domain.ErrChallengeInvalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return r &lt; &#x27;0&#x27; &#124;&#124; r &gt; &#x27;9&#x27;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return meta, domain.ErrChallengeStoreUnavailable</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; json.Unmarshal(&#91;&#93;byte(value), &amp;meta) != nil &#124;&#124; meta.Purpose != purpose &#124;&#124; (subject != &quot;&quot; &amp;&amp; meta.Subject != subject)</code>。条件成立时执行 <code>return meta, domain.ErrChallengeInvalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return meta, domain.ErrChallengeStoreUnavailable</code>。当前函数在这里结束。
- 检查 <code>result != int64(1)</code>。条件成立时执行 <code>return meta, domain.ErrChallengeInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return meta, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=10189:11200 -->
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

// credential.go：保留原实现，按业务合并文件。

````

## 第 18 步：Credentials

插入位置：第 17 步代码之后，紧接追加；完成稿约第 282 行。

本步声明：`type Credentials struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Credentials</code> | 使用 <code>repository.Credentials</code> 类型；其定义处的职责：实现认证 SQL、Redis 登录限流与验证码脚本、凭据准备和注册落库。 |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。 |
| <code>challenges</code> | <code>*Challenges</code> | 验证码业务依赖，校验挑战用途、主体与结果；密码/注册用例不能只检查输入长度就绕过验证码。 |

<!-- write-step: 18 bytes=11200:11353 -->
````go
type Credentials struct {
	db             *sql.DB
	repository     *repository.Credentials
	authentication *Authentication
	challenges     *Challenges
}

````

## 第 19 步：NewCredentials

插入位置：第 18 步代码之后，紧接追加；完成稿约第 289 行。

本步声明：`func NewCredentials(db *sql.DB, repo *repository.Credentials, authentication *Authentication, challenges *Challenges) *Credentials`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>*repository.Credentials</code> | 使用 <code>repository.Credentials</code> 类型；其定义处的职责：实现认证 SQL、Redis 登录限流与验证码脚本、凭据准备和注册落库。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>challenges</code> | <code>*Challenges</code> | 在此声明中的实际使用：<code>challenges: challenges</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Credentials</code> | 实际返回表达式：<code>&amp;Credentials{db: db, repository: repo, authentication: authentication, challenges: challenges}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Credentials{db: db, repository: repo, authentication: authentication, challenges: challenges}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=11353:11592 -->
````go
func NewCredentials(db *sql.DB, repo *repository.Credentials, authentication *Authentication, challenges *Challenges) *Credentials {
	return &Credentials{db: db, repository: repo, authentication: authentication, challenges: challenges}
}

````

## 第 20 步：Credentials.Retry

插入位置：第 19 步代码之后，紧接追加；完成稿约第 293 行。

本步声明：`func (s *Credentials) Retry(ctx context.Context, id, kind string, fingerprint []byte, token string) (domain.CredentialRetry, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Receipt(ctx, s.db, id, false)</code>；<code>s.authentication.Session(ctx, token)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Receipt(ctx, s.db, id, false)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>receipt.Kind != kind</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(fingerprint, receipt.Fingerprint)</code> |
| <code>token</code> | <code>string</code> | 在此声明中的实际使用：<code>s.authentication.Session(ctx, token)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CredentialRetry</code> | 使用 <code>domain.CredentialRetry</code> 类型；其定义处的职责：改密会关闭原会话，但原请求仍可能因断网需要查结果；因此保留原会话事实和操作状态用于恢复核验，不能把恢复路径当成重新登录。<br>实际返回表达式：<code>domain.CredentialRetry{}</code>；<code>domain.CredentialRetry{Session: session, State: receipt.State}</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return domain.CredentialRetry{}, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.CredentialRetry{}, true, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; receipt.ActorID != session.UserID &#124;&#124; receipt.Kind != kind &#124;&#124; receipt.SessionHash != hex.EncodeToString(session.ID) &#124;&#124; !bytes.Equal(fingerprint, receipt.Fingerprint)</code>。条件成立时执行 <code>return domain.CredentialRetry{}, true, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return domain.CredentialRetry{Session: session, State: receipt.State}, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11592:12479 -->
````go
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

````

## 第 21 步：Credentials.Begin

插入位置：第 20 步代码之后，紧接追加；完成稿约第 309 行。

本步声明：`func (s *Credentials) Begin(ctx context.Context, change domain.CredentialChange) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.LockUser(ctx, tx, change.UserID)</code>；<code>s.repository.Receipt(ctx, tx, change.ID, true)</code>；<code>s.repository.ConsumeProof(ctx, tx, change, oldEpoch, subject)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>change</code> | <code>domain.CredentialChange</code> | 使用 <code>domain.CredentialChange</code> 类型；其定义处的职责：将事务前验证过的密码哈希、会话或验证码证明与预期代次合成一次变更输入；事务内仍要检查这些事实是否已被并发操作改变。<br>在此声明中的实际使用：<code>change.ExpectedEpoch</code>；<code>change.UserID</code>；<code>change.ID</code>；<code>change.Kind</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>receipt.State</code>；<code>&quot;PREPARING&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CREDENTIAL_EPOCH_INVALID&quot;}</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CREDENTIAL_INVALID&quot;}</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!valid</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CREDENTIAL_EPOCH_INVALID&quot;}</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CREDENTIAL_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>receipt.ActorID != change.UserID &#124;&#124; receipt.Kind != change.Kind &#124;&#124; !bytes.Equal(receipt.Fingerprint, change.Fingerprint) &#124;&#124; (change.Session != nil &amp;&amp; receipt.SessionHash != hex.EncodeToString(change.Session.ID))</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return receipt.State, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>change.Proof != nil</code>。
- 检查 <code>s.repository.ConsumeProof(ctx, tx, change, oldEpoch, subject) != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;CHALLENGE_ALREADY_CONSUMED&quot;}</code>。当前函数在这里结束。
- 检查 <code>credential.State != &quot;ACTIVE&quot; &#124;&#124; credential.Epoch != change.ExpectedEpoch &#124;&#124; (len(change.OldHash) &gt; 0 &amp;&amp; !bytes.Equal(credential.Hash, change.OldHash))</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;CREDENTIAL_CHANGED&quot;}</code>。当前函数在这里结束。
- 检查 <code>change.Session != nil</code>。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;ACTIVE&quot; &#124;&#124; current.CredentialEpoch != credential.Epoch &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=12479:14782 -->
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
````

## 第 22 步：Credentials.Begin（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 354 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;CREDENTIAL_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return &quot;PREPARING&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=14782:15447 -->
````go
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

````

## 第 23 步：Credentials.ResetPassword

插入位置：第 22 步代码之后，紧接追加；完成稿约第 368 行。

本步声明：`func (s *Credentials) ResetPassword(ctx context.Context, input dto.ResetPassword, fingerprint []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.challenges.Verify(ctx, input.ChallengeId, input.Code, &quot;RESET_PASSWORD&quot;, &quot;&quot;)</code>；<code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;RESET_PASSWORD&quot;, 		UserID:        proof.UserID, 		ExpectedEpoch: proof.Epoch, 		Proof:   …</code>（长表达式，按本步完整代码继续） |
| <code>input</code> | <code>dto.ResetPassword</code> | 在此声明中的实际使用：<code>input.NewPassword</code>；<code>input.ChallengeId</code>；<code>input.Code</code>；<code>input.OperationId</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>Fingerprint:   fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;RESET_PASSWORD&quot;, 		UserID:        proof.UserID, 		ExpectedEpoch: proof.Epoch, 		Proof:   …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&quot;&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;RESET_PASSWORD&quot;, 		UserID:        proof.UserID, 		ExpectedEpoch: proof.Epoch, 		Proof:   …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CHALLENGE_INVALID&quot;}</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidNewPassword(input.NewPassword)</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>proof.UserID == &quot;0&quot;</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CHALLENGE_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;RESET_PASSWORD&quot;, 		UserID:        proof.UserID, 		ExpectedEpoch: proof.Epoch, 		P …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=15447:16446 -->
````go
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

````

## 第 24 步：Credentials.ChangePassword

插入位置：第 23 步代码之后，紧接追加；完成稿约第 395 行。

本步声明：`func (s *Credentials) ChangePassword(ctx context.Context, session domain.Session, input dto.PasswordChange, fingerprint []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Hash(ctx, session.UserID)</code>；<code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_PASSWORD&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCreden …</code>（长表达式，按本步完整代码继续） |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>&amp;session</code> |
| <code>input</code> | <code>dto.PasswordChange</code> | 在此声明中的实际使用：<code>input.NewPassword</code>；<code>input.CurrentPassword</code>；<code>input.OperationId</code>；<code>input.ExpectedCredentialEpoch</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>Fingerprint:   fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_PASSWORD&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCreden …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&quot;&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_PASSWORD&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCreden …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>；<code>errors.New(&quot;CREDENTIAL_UNAVAILABLE&quot;)</code>；<code>domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;CURRENT_PASSWORD_INVALID&quot;}</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidNewPassword(input.NewPassword)</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;CREDENTIAL_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>bcrypt.CompareHashAndPassword(hash, &#91;&#93;byte(input.CurrentPassword)) != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;CURRENT_PASSWORD_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_PASSWORD&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.Expecte …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=16446:17642 -->
````go
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

````

## 第 25 步：Credentials.ChangeEmail

插入位置：第 24 步代码之后，紧接追加；完成稿约第 423 行。

本步声明：`func (s *Credentials) ChangeEmail(ctx context.Context, session domain.Session, input dto.EmailChange, fingerprint []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.challenges.Verify(ctx, input.ChallengeId, input.Code, &quot;CHANGE_EMAIL&quot;, domain.SubjectDigest(input.NewEmail))</code>；<code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_EMAIL&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCredentia …</code>（长表达式，按本步完整代码继续） |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.CredentialEpoch</code>；<code>&amp;session</code> |
| <code>input</code> | <code>dto.EmailChange</code> | 在此声明中的实际使用：<code>input.ChallengeId</code>；<code>input.Code</code>；<code>input.NewEmail</code>；<code>input.OperationId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>Fingerprint:   fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_EMAIL&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCredentia …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&quot;&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_EMAIL&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCredentia …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CHALLENGE_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>proof.ActorID != session.UserID &#124;&#124; proof.Epoch != session.CredentialEpoch</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;CHALLENGE_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return s.Begin(ctx, domain.CredentialChange{ 		ID:            input.OperationId, 		Kind:          &quot;CHANGE_EMAIL&quot;, 		UserID:        session.UserID, 		ExpectedEpoch: input.ExpectedCr …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=17642:18530 -->
````go
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

````

## 第 26 步：Signups

插入位置：第 25 步代码之后，紧接追加；完成稿约第 446 行。

本步声明：`type Signups struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Signups</code> | 使用 <code>repository.Signups</code> 类型；其定义处的职责：实现认证 SQL、Redis 登录限流与验证码脚本、凭据准备和注册落库。 |
| <code>challenges</code> | <code>*Challenges</code> | 验证码业务依赖，校验挑战用途、主体与结果；密码/注册用例不能只检查输入长度就绕过验证码。 |

<!-- write-step: 26 bytes=18530:18631 -->
````go
type Signups struct {
	db         *sql.DB
	repository *repository.Signups
	challenges *Challenges
}

````

## 第 27 步：NewSignups

插入位置：第 26 步代码之后，紧接追加；完成稿约第 452 行。

本步声明：`func NewSignups(db *sql.DB, repo *repository.Signups, challenges *Challenges) *Signups`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>*repository.Signups</code> | 使用 <code>repository.Signups</code> 类型；其定义处的职责：实现认证 SQL、Redis 登录限流与验证码脚本、凭据准备和注册落库。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>challenges</code> | <code>*Challenges</code> | 在此声明中的实际使用：<code>challenges: challenges</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Signups</code> | 实际返回表达式：<code>&amp;Signups{db: db, repository: repo, challenges: challenges}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Signups{db: db, repository: repo, challenges: challenges}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=18631:18790 -->
````go
func NewSignups(db *sql.DB, repo *repository.Signups, challenges *Challenges) *Signups {
	return &Signups{db: db, repository: repo, challenges: challenges}
}

````

## 第 28 步：Signups.PublicResult

插入位置：第 27 步代码之后，紧接追加；完成稿约第 456 行。

本步声明：`func (s *Signups) PublicResult(ctx context.Context, id, kind string, fingerprint []byte) (json.RawMessage, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.PublicReceipt(ctx, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.PublicReceipt(ctx, id)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>receipt.Kind != kind</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>hmac.Equal(receipt.Fingerprint, fingerprint)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>receipt.Result</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;OPERATION_PENDING&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, true, err</code>。当前函数在这里结束。
- 检查 <code>receipt.Kind != kind &#124;&#124; !hmac.Equal(receipt.Fingerprint, fingerprint)</code>。条件成立时执行 <code>return nil, true, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 检查 <code>len(receipt.Result) == 0</code>。条件成立时执行 <code>return nil, true, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;OPERATION_PENDING&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return receipt.Result, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=18790:19419 -->
````go
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

````

## 第 29 步：Signups.Register

插入位置：第 28 步代码之后，紧接追加；完成稿约第 473 行。

本步声明：`func (s *Signups) Register(ctx context.Context, input dto.Signup, fingerprint []byte) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.challenges.Verify(ctx, input.ChallengeId, input.Code, &quot;SIGNUP&quot;, domain.SubjectDigest(input.Email))</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.InsertUser(ctx, tx, input.Email, input.DisplayName, hash)</code>；<code>s.PublicResult(ctx, input.OperationId, &quot;SIGNUP&quot;, fingerprint)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.Signup</code> | 在此声明中的实际使用：<code>input.Password</code>；<code>input.ChallengeId</code>；<code>input.Code</code>；<code>input.Email</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.PublicResult(ctx, input.OperationId, &quot;SIGNUP&quot;, fingerprint)</code>；<code>s.repository.InsertReceipt(ctx, tx, input.OperationId, input.ChallengeId, user, fingerprint, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EMAIL_ALREADY_REGISTERED&quot;}</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidNewPassword(input.Password)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_POLICY_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PASSWORD_INVALID&quot;}</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>repository.DuplicateKey(err)</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>found</code>。条件成立时执行 <code>return raw, err</code>。当前函数在这里结束。
- 此处通过 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EMAIL_ALREADY_REGISTERED&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>repository.DuplicateKey(err)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;CHALLENGE_ALREADY_CONSUMED&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>s.repository.InsertReceipt(ctx, tx, input.OperationId, input.ChallengeId, user, fingerprint, raw) != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SIGNUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 29 bytes=19419:21546 -->
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
````

## 第 30 步：Signups.Register（接着实现）

插入位置：第 29 步代码之后，紧接追加；完成稿约第 519 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return raw, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=21546:21565 -->
````go
	return raw, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/authentication_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
