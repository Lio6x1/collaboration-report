# 分步编写 `services/account/internal/repository/account/authentication_queries.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/048-authentication-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现认证 SQL、Redis 登录限流与验证码脚本、凭据准备和注册落库。

密码计算交给 logic；这里接收原事务执行行锁与写入，使会话、验证码消费和回执保持原子性。

写之前先按这条执行路线理解各段的连接：

1. LoginUser 按 email_key 查 bcrypt 资料；LockCredential 在原 tx 内 FOR UPDATE，InsertSession 只写摘要。
2. LoginLimit.Allow 用一次 Lua 完成全局100次/60秒与邮箱10次/60秒限制；先限总量再创建邮箱键。
3. Challenges 的 Lua 保留挑战600秒、发送冷却60秒、最多5次错误；MySQL verification_consumption 唯一键才负责不可重复消费。
4. Credentials.Prepare 同事务写 PREPARING 回执、REVOKING 用户/会话、BLOCKED gate、authorization_change 和 durable_task。

本文件需要守住的边界：

- conn 与 db 必须来自同一 Store 连接池；锁内查询只走 tx。
- 验证码校验成功不等于已消费，消费必须与账号写入同事务。
- 只有更新邮箱时的1062可解释为 EMAIL_ALREADY_REGISTERED，其余提交不确定不能改成成功。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/repository/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/repository/account/authentication_queries.go') -and (Get-Item -LiteralPath 'services/account/internal/repository/account/authentication_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/repository/account/authentication_queries.go')) { New-Item -ItemType File -Path 'services/account/internal/repository/account/authentication_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:467 -->
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

````

## 第 2 步：Authentication

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type Authentication struct {`。

**这一段为什么这样写：** 把go-zero查询适配与database/sql事务共享到同一个Account连接池。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>conn</code> | <code>sqlx.SqlConn</code> | Store.Conn包装的sqlx.SqlConn，用于LoginUser映射db标签的非事务读。 |
| <code>db</code> | <code>*sql.DB</code> | 与conn同底层池的sql.DB，用于session/user权威查询；事务锁由logic传入tx。 |

<!-- write-step: 2 bytes=467:532 -->
````go
type Authentication struct {
	conn sqlx.SqlConn
	db   *sql.DB
}

````

## 第 3 步：NewAuthentication

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func NewAuthentication(conn sqlx.SqlConn, db *sql.DB) *Authentication`。

**这一段为什么这样写：** 两种查询接口必须指向同一数据库池，避免锁外/锁内访问不同实例。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>conn</code> | <code>sqlx.SqlConn</code> | Store.Conn的go-zero SQL适配器。<br>在此声明中的实际使用：<code>conn: conn</code> |
| <code>db</code> | <code>*sql.DB</code> | 同Store.DB底层池。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authentication</code> | 认证查询仓库；尚未执行任何SQL。<br>实际返回表达式：<code>&amp;Authentication{conn: conn, db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authentication{conn: conn, db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=532:745 -->
````go
func NewAuthentication(conn sqlx.SqlConn, db *sql.DB) *Authentication {
	// 两个适配必须来自Store的同一连接池，不能为事务里的查询另开池。
	return &Authentication{conn: conn, db: db}
}

````

## 第 4 步：Authentication.LoginUser

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (r *Authentication) LoginUser(ctx context.Context, email string) (domain.LoginUser, error)`。

**这一段为什么这样写：** 按规范邮箱键一次读取登录所需的哈希、算法、状态和凭据代次；密码比较留在业务层，数据库读取本身不表示登录成功。

1. SELECT以CAST保留64位ID精度，COALESCE让关闭账号空密码字段可扫描；不在此判断密码正确与否。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 邮箱登录查询的取消信号。<br>在此声明中的实际使用：<code>r.conn.QueryRowCtx(ctx, &amp;row, &quot;SELECT CAST(id AS CHAR) AS id,display_name,COALESCE(password_hash,X&#x27;&#x27;) AS password_hash,COALESCE(password_algorithm,&#x27;&#x27;) AS password_algorithm,state,C …</code>（长表达式，按本步完整代码继续） |
| <code>email</code> | <code>string</code> | 已经ToLower/TrimSpace的邮箱，转[]byte匹配二进制email_key。<br>在此声明中的实际使用：<code>&#91;&#93;byte(email)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.LoginUser</code> | 锁外密码校验所需user ID/显示名/hash/algorithm/state/credential epoch。<br>实际返回表达式：<code>domain.LoginUser{ 		ID: row.ID, DisplayName: row.DisplayName, Hash: row.Hash, 		Algorithm: row.Algorithm, State: row.State, CredentialEpoch: row.CredentialEpoch, 	}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 查询或映射错误，未知邮箱为无行错误。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.LoginUser{ 		ID: row.ID, DisplayName: row.DisplayName, Hash: row.Hash, 		Algorithm: row.Algorithm, State: row.State, CredentialEpoch: row.CredentialEpoch, 	}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=745:1592 -->
````go
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

````

## 第 5 步：Authentication.LockCredential

插入位置：第 4 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (r *Authentication) LockCredential(ctx context.Context, tx *sql.Tx, userID string) (domain.Credential, error)`。

**这一段为什么这样写：** 在调用方传入的原事务中用FOR UPDATE锁住账号凭据；登录后写会话前必须复核锁内代次，避免并发改密后仍创建旧代次会话。

1. 对user WHERE id=? FOR UPDATE，锁一直保留到调用者Commit或Rollback。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 原登录事务的上下文。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE&quot;, userID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | Login开启且尚未提交的原事务。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>userID</code> | <code>string</code> | 锁外LoginUser取得的用户ID。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE&quot;, userID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 锁内当前password_hash、credential_epoch、state。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 查询失败；logic不把失败当成功重验。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE&quot;, userID).Scan(&amp;value.Hash, &amp;value.Epoch, &amp;value.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE&quot;, userID)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1592:1939 -->
````go
func (r *Authentication) LockCredential(ctx context.Context, tx *sql.Tx, userID string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT password_hash,CAST(credential_epoch AS CHAR),state FROM user WHERE id=? FOR UPDATE", userID).Scan(&value.Hash, &value.Epoch, &value.State)
	return value, err
}

````

## 第 6 步：Authentication.InsertSession

插入位置：第 5 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (r *Authentication) InsertSession(ctx context.Context, tx *sql.Tx, value domain.Session) error`。

**这一段为什么这样写：** 用同一个事务写入会话摘要、CSRF摘要和准入时的凭据代次，让凭据复核与新会话落库共同提交。

1. session固定ACTIVE，last_seen_at用数据库UTC时间，epoch使用表默认初始值。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 会话插入上下文。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO session(id,user_id,csrf_hash,credential_epoch,state,expires_at,last_seen_at) VALUES(?,?,?,?,&#x27;ACTIVE&#x27;,?,UTC_TIMESTAMP(6))&quot;, value.ID, value.UserID,  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 已经持有user行锁的登录事务。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>value</code> | <code>domain.Session</code> | 含会话/CSRF摘要、UserID、当前CredentialEpoch和Expires的待写会话。<br>在此声明中的实际使用：<code>value.ID</code>；<code>value.UserID</code>；<code>value.CSRF</code>；<code>value.CredentialEpoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | INSERT语句结果；nil不代表事务已提交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO session(id,user_id,csrf_hash,credential_epoch,state,expires_at,last_seen_at) VALUES(?,?,?,?,&#x27;ACTIVE&#x27;,?,UTC_TIMESTAMP(6))&quot;, value.ID, value.UserID,  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1939:2297 -->
````go
func (r *Authentication) InsertSession(ctx context.Context, tx *sql.Tx, value domain.Session) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO session(id,user_id,csrf_hash,credential_epoch,state,expires_at,last_seen_at) VALUES(?,?,?,?,'ACTIVE',?,UTC_TIMESTAMP(6))", value.ID, value.UserID, value.CSRF, value.CredentialEpoch, value.Expires)
	return err
}

````

## 第 7 步：Authentication.Session

插入位置：第 6 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (r *Authentication) Session(ctx context.Context, id []byte) (domain.Session, error)`。

**这一段为什么这样写：** 按会话摘要读取数据库中的归属、代次、状态和到期时间；返回的是会话事实，调用者还要检查有效性，不能把查到一行当成已授权。

1. 保留传入id并扫描其余列；不过滤ACTIVE，让退出恢复仍可读原会话。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 会话事实查询上下文。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?&quot;, id)</code> |
| <code>id</code> | <code>&#91;&#93;byte</code> | 32字节SHA256会话摘要，与session.id主键完全相同。<br>在此声明中的实际使用：<code>ID: id</code>；<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | Session含ID、归属用户、CSRF摘要、两种代次、状态和绝对到期。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 无行与数据库故障分开保留。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?&quot;, id).Scan(&amp;value.UserID, &amp;value.CSR …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2297:2702 -->
````go
func (r *Authentication) Session(ctx context.Context, id []byte) (domain.Session, error) {
	value := domain.Session{ID: id}
	err := r.db.QueryRowContext(ctx, "SELECT CAST(user_id AS CHAR),csrf_hash,epoch,CAST(credential_epoch AS CHAR),state,expires_at FROM session WHERE id=?", id).Scan(&value.UserID, &value.CSRF, &value.Epoch, &value.CredentialEpoch, &value.State, &value.Expires)
	return value, err
}

````

## 第 8 步：Authentication.Credential

插入位置：第 7 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (r *Authentication) Credential(ctx context.Context, userID string) (domain.Credential, error)`。

**这一段为什么这样写：** 读取账号当前状态、凭据代次和身份代次供会话核验；此处不需要密码哈希，也没有FOR UPDATE，不能代替修改凭据时的加锁读取。

1. 只SELECT状态与两种代次，Authenticate无需再次读取密码哈希。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 当前用户凭据状态的查询上下文。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?&quot;, userID)</code> |
| <code>userID</code> | <code>string</code> | session.UserID，不是客户端自报目标。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?&quot;, userID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 当前用户State/Epoch/IdentityEpoch；此查询不会填充Hash。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | SQL无行或故障，用例按类别处理。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?&quot;, userID).Scan(&amp;value.State, &amp;value.Epoch, &amp;value.IdentityEpoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?&quot;, userID)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2702:3034 -->
````go
func (r *Authentication) Credential(ctx context.Context, userID string) (domain.Credential, error) {
	var value domain.Credential
	err := r.db.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),identity_epoch FROM user WHERE id=?", userID).Scan(&value.State, &value.Epoch, &value.IdentityEpoch)
	return value, err
}

````

## 第 9 步：Authentication.User

插入位置：第 8 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func (r *Authentication) User(ctx context.Context, userID string) (domain.User, error)`。

**这一段为什么这样写：** 读取面向账号资料与身份检查的用户快照，包含昵称、邮箱、状态及两种代次；它不承担密码验证，也不返回密码哈希。

1. ID及代次CAST AS CHAR避免JSON数字精度丢失，CurrentUser随后核状态/代次。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本人资料查询上下文。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),COALESCE(email,&#x27;&#x27;),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?&quot;, userID)</code> |
| <code>userID</code> | <code>string</code> | 已经通过会话绑定的用户ID。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),COALESCE(email,&#x27;&#x27;),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?&quot;, userID)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.User</code> | User资料及两个字符串代次；邮箱NULL映射空字符串。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 查询错误，资料不可在error非空时交付。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),COALESCE(email,&#x27;&#x27;),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?&quot;, userID). …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),COALESCE(email,&#x27;&#x27;),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?&quot;, userID)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3034:3528 -->
````go
func (r *Authentication) User(ctx context.Context, userID string) (domain.User, error) {
	var value domain.User
	err := r.db.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),COALESCE(email,''),display_name,state,CAST(credential_epoch AS CHAR),CAST(identity_epoch AS CHAR) FROM user WHERE id=?", userID).Scan(&value.ID, &value.Email, &value.DisplayName, &value.State, &value.CredentialEpoch, &value.IdentityEpoch)
	return value, err
}

// challenge.go：保留原实现，按业务合并文件。

````

## 第 10 步：Challenges

插入位置：第 9 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`type Challenges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>redis</code> | <code>*redis.Redis</code> | Redis挑战存储客户端，原子记录验证码状态、频率和校验结果；数据库账号事实仍由MySQL保存。 |

<!-- write-step: 10 bytes=3528:3591 -->
````go
type Challenges struct {
	db    *sql.DB
	redis *redis.Redis
}

````

## 第 11 步：NewChallenges

插入位置：第 10 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func NewChallenges(db *sql.DB, limiter *redis.Redis) *Challenges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>limiter</code> | <code>*redis.Redis</code> | 在此声明中的实际使用：<code>redis: limiter</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Challenges</code> | 实际返回表达式：<code>&amp;Challenges{db: db, redis: limiter}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Challenges{db: db, redis: limiter}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3591:3705 -->
````go
func NewChallenges(db *sql.DB, limiter *redis.Redis) *Challenges {
	return &Challenges{db: db, redis: limiter}
}

````

## 第 12 步：issueChallengeScript

插入位置：第 11 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`const issueChallengeScript = \``。

<!-- write-step: 12 bytes=3705:4044 -->
````go
const issueChallengeScript = `
local total=redis.call('INCR',KEYS[1]); if total==1 then redis.call('EXPIRE',KEYS[1],60) end
if total>100 then return 0 end
if not redis.call('SET',KEYS[2],'1','NX','EX',60) then return 0 end
redis.call('HSET',KEYS[3],'digest',ARGV[1],'meta',ARGV[2],'attempts',0)
redis.call('EXPIRE',KEYS[3],600)
return 1`

````

## 第 13 步：verifyChallengeScript

插入位置：第 12 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`const verifyChallengeScript = \``。

<!-- write-step: 13 bytes=4044:4352 -->
````go
const verifyChallengeScript = `
local digest=redis.call('HGET',KEYS[1],'digest')
if not digest then return 0 end
local attempts=tonumber(redis.call('HGET',KEYS[1],'attempts') or '5')
if attempts>=5 then return 0 end
if digest~=ARGV[1] then redis.call('HINCRBY',KEYS[1],'attempts',1); return 0 end
return 1`

````

## 第 14 步：Challenges.ResetAccount

插入位置：第 13 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func (r *Challenges) ResetAccount(ctx context.Context, email string) (domain.ChallengeMeta, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(credential_epoch AS CHAR),state FROM user WHERE email_key=?&quot;, &#91;&#93;byte(email))</code> |
| <code>email</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(email)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ChallengeMeta</code> | 使用 <code>domain.ChallengeMeta</code> 类型；其定义处的职责：把验证码和用途、接收对象、账号代次及发起人一起绑定；只比较验证码数字会让原本有效的证明被挪到另一项操作。<br>实际返回表达式：<code>meta</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(credential_epoch AS CHAR),state FROM user WHERE email_key=?&quot;, &#91;&#93;byte(email)).Scan(&amp;meta.UserID, &amp;meta.Epoch, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT CAST(id AS CHAR),CAST(credential_epoch AS CHAR),state FROM user WHERE email_key=?&quot;, &#91;&#93;byte(email))</code>。
- 此处通过 <code>return meta, state, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4352:4743 -->
````go
func (r *Challenges) ResetAccount(ctx context.Context, email string) (domain.ChallengeMeta, string, error) {
	meta := domain.ChallengeMeta{UserID: "0", Epoch: "1"}
	var state string
	err := r.db.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(credential_epoch AS CHAR),state FROM user WHERE email_key=?", []byte(email)).Scan(&meta.UserID, &meta.Epoch, &state)
	return meta, state, err
}

````

## 第 15 步：Challenges.Issue

插入位置：第 14 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func (r *Challenges) Issue(ctx context.Context, id string, meta domain.ChallengeMeta, digest string, raw []byte) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;acct:challenge:&quot; + id</code> |
| <code>meta</code> | <code>domain.ChallengeMeta</code> | 使用 <code>domain.ChallengeMeta</code> 类型；其定义处的职责：把验证码和用途、接收对象、账号代次及发起人一起绑定；只比较验证码数字会让原本有效的证明被挪到另一项操作。<br>在此声明中的实际使用：<code>meta.Purpose</code>；<code>meta.Subject</code> |
| <code>digest</code> | <code>string</code> | 在此声明中的实际使用：<code>r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 由 <code>r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=4743:5083 -->
````go
func (r *Challenges) Issue(ctx context.Context, id string, meta domain.ChallengeMeta, digest string, raw []byte) (any, error) {
	keys := []string{"acct:challenge:global", "acct:challenge:cooldown:" + meta.Purpose + ":" + meta.Subject, "acct:challenge:" + id}
	return r.redis.EvalCtx(ctx, issueChallengeScript, keys, digest, string(raw))
}

````

## 第 16 步：Challenges.Meta

插入位置：第 15 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func (r *Challenges) Meta(ctx context.Context, id string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.redis.EvalCtx(ctx, `return redis.call(&#x27;HGET&#x27;,KEYS&#91;1&#93;,&#x27;meta&#x27;) or &#x27;&#x27;`, &#91;&#93;string{&quot;acct:challenge:&quot; + id})</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;acct:challenge:&quot; + id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 由 <code>r.redis.EvalCtx(ctx, `return redis.call(&#x27;HGET&#x27;,KEYS&#91;1&#93;,&#x27;meta&#x27;) or &#x27;&#x27;`, &#91;&#93;string{&quot;acct:challenge:&quot; + id})</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>r.redis.EvalCtx(ctx, `return redis.call(&#x27;HGET&#x27;,KEYS&#91;1&#93;,&#x27;meta&#x27;) or &#x27;&#x27;`, &#91;&#93;string{&quot;acct:challenge:&quot; + id})</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return r.redis.EvalCtx(ctx, `return redis.call(&#x27;HGET&#x27;,KEYS&#91;1&#93;,&#x27;meta&#x27;) or &#x27;&#x27;`, &#91;&#93;string{&quot;acct:challenge:&quot; + id})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=5083:5272 -->
````go
func (r *Challenges) Meta(ctx context.Context, id string) (any, error) {
	return r.redis.EvalCtx(ctx, `return redis.call('HGET',KEYS[1],'meta') or ''`, []string{"acct:challenge:" + id})
}

````

## 第 17 步：Challenges.Verify

插入位置：第 16 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func (r *Challenges) Verify(ctx context.Context, id, digest string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.redis.EvalCtx(ctx, verifyChallengeScript, &#91;&#93;string{&quot;acct:challenge:&quot; + id}, digest)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;acct:challenge:&quot; + id</code> |
| <code>digest</code> | <code>string</code> | 在此声明中的实际使用：<code>r.redis.EvalCtx(ctx, verifyChallengeScript, &#91;&#93;string{&quot;acct:challenge:&quot; + id}, digest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 由 <code>r.redis.EvalCtx(ctx, verifyChallengeScript, &#91;&#93;string{&quot;acct:challenge:&quot; + id}, digest)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>r.redis.EvalCtx(ctx, verifyChallengeScript, &#91;&#93;string{&quot;acct:challenge:&quot; + id}, digest)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return r.redis.EvalCtx(ctx, verifyChallengeScript, &#91;&#93;string{&quot;acct:challenge:&quot; + id}, digest)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=5272:5515 -->
````go
func (r *Challenges) Verify(ctx context.Context, id, digest string) (any, error) {
	return r.redis.EvalCtx(ctx, verifyChallengeScript, []string{"acct:challenge:" + id}, digest)
}

// credential.go：保留原实现，按业务合并文件。

````

## 第 18 步：Credentials

插入位置：第 17 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`type Credentials struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 18 bytes=5515:5554 -->
````go
type Credentials struct{ db *sql.DB }

````

## 第 19 步：NewCredentials

插入位置：第 18 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`func NewCredentials(db *sql.DB) *Credentials`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Credentials</code> | 实际返回表达式：<code>&amp;Credentials{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Credentials{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=5554:5632 -->
````go
func NewCredentials(db *sql.DB) *Credentials { return &Credentials{db: db} }

````

## 第 20 步：Credentials.Hash

插入位置：第 19 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func (r *Credentials) Hash(ctx context.Context, user string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT password_hash FROM user WHERE id=?&quot;, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT password_hash FROM user WHERE id=?&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>hash</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT password_hash FROM user WHERE id=?&quot;, user).Scan(&amp;hash)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT password_hash FROM user WHERE id=?&quot;, user)</code>。
- 此处通过 <code>return hash, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=5632:5846 -->
````go
func (r *Credentials) Hash(ctx context.Context, user string) ([]byte, error) {
	var hash []byte
	err := r.db.QueryRowContext(ctx, "SELECT password_hash FROM user WHERE id=?", user).Scan(&hash)
	return hash, err
}

````

## 第 21 步：Credentials.Receipt

插入位置：第 20 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func (r *Credentials) Receipt(ctx context.Context, query Queryer, id string, lock bool) (domain.CredentialReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>query.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint 		FROM o …</code>（长表达式，按本步完整代码继续） |
| <code>query</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>query.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>query.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint 		FROM o …</code>（长表达式，按本步完整代码继续） |
| <code>lock</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CredentialReceipt</code> | 使用 <code>domain.CredentialReceipt</code> 类型；其定义处的职责：在通用操作回执上增加凭据输入指纹，使同一个操作号不能被另一组新密码或新邮箱输入复用。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>lock</code>。
- 执行 <code>query.QueryRowContext(ctx, ` 		SELECT CAST(actor_id AS CHAR),kind,state, 		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,&#x27;$.session_hash&#x27;)),&#x27;&#x27;),request_fingerprint 		FROM o …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=5846:6411 -->
````go
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

````

## 第 22 步：Credentials.LockUser

插入位置：第 21 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func (r *Credentials) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE&quot;, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Credential</code> | 使用 <code>domain.Credential</code> 类型；其定义处的职责：按调用目的承载密码或状态代次快照。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE&quot;, user).Scan(&amp;value.State, &amp;value.Epoch, &amp;value.Hash)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE&quot;, user)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=6411:6745 -->
````go
func (r *Credentials) LockUser(ctx context.Context, tx *sql.Tx, user string) (domain.Credential, error) {
	var value domain.Credential
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),password_hash FROM user WHERE id=? FOR UPDATE", user).Scan(&value.State, &value.Epoch, &value.Hash)
	return value, err
}

````

## 第 23 步：Credentials.ConsumeProof

插入位置：第 22 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`func (r *Credentials) ConsumeProof(ctx context.Context, tx *sql.Tx, change domain.CredentialChange, epoch uint64, subject []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.Ch …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>change</code> | <code>domain.CredentialChange</code> | 使用 <code>domain.CredentialChange</code> 类型；其定义处的职责：将事务前验证过的密码哈希、会话或验证码证明与预期代次合成一次变更输入；事务内仍要检查这些事实是否已被并发操作改变。<br>在此声明中的实际使用：<code>change.ChallengeID</code>；<code>change.Proof</code>；<code>change.UserID</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.Ch …</code>（长表达式，按本步完整代码继续） |
| <code>subject</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.Ch …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.Ch …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=6745:7153 -->
````go
func (r *Credentials) ConsumeProof(ctx context.Context, tx *sql.Tx, change domain.CredentialChange, epoch uint64, subject []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO verification_consumption(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at)
		VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, change.ChallengeID, change.Proof.Purpose, subject, change.UserID, epoch)
	return err
}

````

## 第 24 步：Credentials.LockSession

插入位置：第 23 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`func (r *Credentials) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;value.State, &amp;value.CredentialEpoch, &amp;value.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=7153:7564 -->
````go
func (r *Credentials) LockSession(ctx context.Context, tx *sql.Tx, id []byte) (domain.Session, error) {
	var value domain.Session
	err := tx.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR),expires_at FROM session WHERE id=? FOR UPDATE", id).Scan(&value.State, &value.CredentialEpoch, &value.Expires)
	return value, err
}

// credential_prepare.go：保留原实现，按业务合并文件。

````

## 第 25 步：Credentials.Prepare

插入位置：第 24 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func (r *Credentials) Prepare(ctx context.Context, tx *sql.Tx, plan domain.CredentialPreparation) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO operation_receipt 		  (operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		  VALUES(?,?,?,?,?,&#x27; …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE user SET password_hash=?,password_algorithm=&#x27;bcrypt&#x27; WHERE id=?&quot;, change.NewHash, change.UserID)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE user SET email=?,email_key=?,email_verified_at=UTC_TIMESTAMP(6) WHERE id=?&quot;, change.NewEmail, &#91;&#93;byte(change.NewEmail), change.UserID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>execute(ctx, tx, &#91;&#93;statement{ 		{`INSERT INTO operation_receipt 		  (operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at) 		  VALUES(?,?,?,?,?,&#x27; …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext</code> |
| <code>plan</code> | <code>domain.CredentialPreparation</code> | 使用 <code>domain.CredentialPreparation</code> 类型；其定义处的职责：在关闭旧凭据来源前固定本次变更的范围、旧代次和恢复载荷；后台恢复必须沿用这份准备结果，不能临时重新选择目标。<br>在此声明中的实际使用：<code>plan.Change</code>；<code>plan.Target</code>；<code>plan.Pending</code>；<code>plan.Epoch</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;CREDENTIAL_COMMIT_UNKNOWN&quot;)</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EMAIL_ALREADY_REGISTERED&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;CREDENTIAL_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(change.NewHash) &gt; 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;CREDENTIAL_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE user SET password_hash=?,password_algorithm=&#x27;bcrypt&#x27; WHERE id=?&quot;, change.NewHash, change.UserID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>change.NewEmail != &quot;&quot;</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE user SET email=?,email_key=?,email_verified_at=UTC_TIMESTAMP(6) WHERE id=?&quot;, change.NewEmail, &#91;&#93;byte(change.NewEmail), change.UserID)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>DuplicateKey(err)</code>。条件成立时执行 <code>return domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EMAIL_ALREADY_REGISTERED&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;CREDENTIAL_COMMIT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=7564:10080 -->
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

// login_limit.go：保留原实现，按业务合并文件。

````

## 第 26 步：loginRateScript

插入位置：第 25 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`const loginRateScript = \``。

<!-- write-step: 26 bytes=10080:10364 -->
````go
const loginRateScript = `
local total=redis.call('INCR',KEYS[1]); if total==1 then redis.call('EXPIRE',KEYS[1],60) end
if total>100 then return 0 end
local count=redis.call('INCR',KEYS[2]); if count==1 then redis.call('EXPIRE',KEYS[2],60) end
if count>10 then return 0 end
return 1`

````

## 第 27 步：LoginLimit

插入位置：第 26 步代码之后，紧接追加；完成稿约第 217 行。

本步声明：`type LoginLimit struct{ redis *redis.Redis }`。

**这一段为什么这样写：** 把Redis脚本实现收敛为Allow能力。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>redis</code> | <code>*redis.Redis</code> | 启动时装配的Redis连接，执行全局与邮箱计数的单次Lua。 |

<!-- write-step: 27 bytes=10364:10410 -->
````go
type LoginLimit struct{ redis *redis.Redis }

````

## 第 28 步：NewLoginLimit

插入位置：第 27 步代码之后，紧接追加；完成稿约第 219 行。

本步声明：`func NewLoginLimit(client *redis.Redis) *LoginLimit`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>client</code> | <code>*redis.Redis</code> | 在此声明中的实际使用：<code>redis: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*LoginLimit</code> | 使用 <code>LoginLimit</code> 类型；其定义处的职责：把Redis脚本实现收敛为Allow能力。<br>实际返回表达式：<code>&amp;LoginLimit{redis: client}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;LoginLimit{redis: client}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=10410:10501 -->
````go
func NewLoginLimit(client *redis.Redis) *LoginLimit { return &LoginLimit{redis: client} }

````

## 第 29 步：LoginLimit.Allow

插入位置：第 28 步代码之后，紧接追加；完成稿约第 221 行。

本步声明：`func (r *LoginLimit) Allow(ctx context.Context, email string) (bool, error)`。

**这一段为什么这样写：** 全局先限流可阻止攻击者用随机邮箱无限创建键；Lua将INCR/EXPIRE变为原子操作。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 继承请求取消并再限1秒，防止Redis拖住登录。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>r.redis.EvalCtx(ctx, loginRateScript, &#91;&#93;string{&quot;acct:login:global&quot;, &quot;acct:login:email:&quot; + hex.EncodeToString(digest&#91;:&#93;)})</code> |
| <code>email</code> | <code>string</code> | 已规范化邮箱，仅其SHA256十六进制摘要进入Redis键。<br>在此声明中的实际使用：<code>&#91;&#93;byte(email)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 只有Lua返回int64(1)为true，超全局100或邮箱10次/60秒为false。<br>实际返回表达式：<code>false</code>；<code>ok &amp;&amp; count == 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | Redis异常转换LOGIN_LIMITER_UNAVAILABLE，不能失败放行。<br>实际返回表达式：<code>errors.New(&quot;LOGIN_LIMITER_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;LOGIN_LIMITER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return ok &amp;&amp; count == 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=10501:11120 -->
````go
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

````

## 第 30 步：Signups

插入位置：第 29 步代码之后，紧接追加；完成稿约第 236 行。

本步声明：`type Signups struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 30 bytes=11120:11155 -->
````go
type Signups struct{ db *sql.DB }

````

## 第 31 步：NewSignups

插入位置：第 30 步代码之后，紧接追加；完成稿约第 238 行。

本步声明：`func NewSignups(db *sql.DB) *Signups`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Signups</code> | 实际返回表达式：<code>&amp;Signups{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Signups{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=11155:11221 -->
````go
func NewSignups(db *sql.DB) *Signups { return &Signups{db: db} }

````

## 第 32 步：DuplicateKey

插入位置：第 31 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func DuplicateKey(err error) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;driver)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>errors.As(err, &amp;driver) &amp;&amp; driver.Number == 1062</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.As(err, &amp;driver) &amp;&amp; driver.Number == 1062</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=11221:11347 -->
````go
func DuplicateKey(err error) bool {
	var driver *mysql.MySQLError
	return errors.As(err, &driver) && driver.Number == 1062
}

````

## 第 33 步：Signups.PublicReceipt

插入位置：第 32 步代码之后，紧接追加；完成稿约第 245 行。

本步声明：`func (r *Signups) PublicReceipt(ctx context.Context, id string) (domain.PublicReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PublicReceipt</code> | 使用 <code>domain.PublicReceipt</code> 类型；其定义处的职责：公开恢复入口不能只凭操作编号取结果；同时保存操作类型和输入指纹，让重试先证明自己对应原请求，再返回原持久结果。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?&quot;, id).Scan(&amp;value.Kind, &amp;value.Fingerprint, &amp;value.Result)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?&quot;, id)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=11347:11679 -->
````go
func (r *Signups) PublicReceipt(ctx context.Context, id string) (domain.PublicReceipt, error) {
	var value domain.PublicReceipt
	err := r.db.QueryRowContext(ctx, "SELECT kind,request_fingerprint,result_json FROM operation_receipt WHERE operation_id=?", id).Scan(&value.Kind, &value.Fingerprint, &value.Result)
	return value, err
}

````

## 第 34 步：Signups.InsertUser

插入位置：第 33 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func (r *Signups) InsertUser(ctx context.Context, tx *sql.Tx, email, displayName string, hash []byte) (int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) 		VALUES(?,?,?,?,&#x27;bcrypt&#x27;,&#x27;ACTIVE&#x27;,UTC_TIMESTAMP(6)) …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>email</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) 		VALUES(?,?,?,?,&#x27;bcrypt&#x27;,&#x27;ACTIVE&#x27;,UTC_TIMESTAMP(6)) …</code>（长表达式，按本步完整代码继续）；<code>&#91;&#93;byte(email)</code> |
| <code>displayName</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) 		VALUES(?,?,?,?,&#x27;bcrypt&#x27;,&#x27;ACTIVE&#x27;,UTC_TIMESTAMP(6)) …</code>（长表达式，按本步完整代码继续） |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) 		VALUES(?,?,?,?,&#x27;bcrypt&#x27;,&#x27;ACTIVE&#x27;,UTC_TIMESTAMP(6)) …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO user(email,email_key,display_name,password_hash,password_algorithm,state,email_verified_at) 		VALUES(?,?,?,?,&#x27;bcrypt&#x27;,&#x27;ACTIVE&#x27;,UTC_TIMESTAMP(6)) …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=11679:12119 -->
````go
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

````

## 第 35 步：Signups.Consume

插入位置：第 34 步代码之后，紧接追加；完成稿约第 262 行。

本步声明：`func (r *Signups) Consume(ctx context.Context, tx *sql.Tx, challenge string, subject []byte, user int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption 		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,&#x27;SIGNUP&#x27;,?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>challenge</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption 		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,&#x27;SIGNUP&#x27;,?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)</code> |
| <code>subject</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption 		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,&#x27;SIGNUP&#x27;,?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)</code> |
| <code>user</code> | <code>int64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption 		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,&#x27;SIGNUP&#x27;,?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO verification_consumption 		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at) 		VALUES(?,&#x27;SIGNUP&#x27;,?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=12119:12465 -->
````go
func (r *Signups) Consume(ctx context.Context, tx *sql.Tx, challenge string, subject []byte, user int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO verification_consumption
		(challenge_id,purpose,subject_key,user_id,credential_epoch,consumed_at)
		VALUES(?,'SIGNUP',?,?,1,UTC_TIMESTAMP(6))`, challenge, subject, user)
	return err
}

````

## 第 36 步：Signups.InsertReceipt

插入位置：第 35 步代码之后，紧接追加；完成稿约第 270 行。

本步声明：`func (r *Signups) InsertReceipt(ctx context.Context, tx *sql.Tx, id, challenge string, user int64, fingerprint, result []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO operation_receipt 		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 		VALUES(?,?,&#x27;SIGNUP&#x27;, …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO operation_receipt 		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 		VALUES(?,?,&#x27;SIGNUP&#x27;, …</code>（长表达式，按本步完整代码继续） |
| <code>challenge</code> | <code>string</code> | 在此声明中的实际使用：<code>fmt.Sprintf(`{&quot;challenge_id&quot;:%q}`, challenge)</code> |
| <code>user</code> | <code>int64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO operation_receipt 		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 		VALUES(?,?,&#x27;SIGNUP&#x27;, …</code>（长表达式，按本步完整代码继续） |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, ` 		INSERT INTO operation_receipt 		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 		VALUES(?,?,&#x27;SIGNUP&#x27;, …</code>（长表达式，按本步完整代码继续） |
| <code>result</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, ` 		INSERT INTO operation_receipt 		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at) 		VALUES(?,?,&#x27;SIGNUP&#x27;, …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=12465:12971 -->
````go
func (r *Signups) InsertReceipt(ctx context.Context, tx *sql.Tx, id, challenge string, user int64, fingerprint, result []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO operation_receipt
		(operation_id,actor_id,kind,target_json,request_fingerprint,state,result_json,expires_at,terminal_at)
		VALUES(?,?,'SIGNUP',?,?,'SUCCEEDED',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 7 DAY),UTC_TIMESTAMP(6))`, id, user, fmt.Sprintf(`{"challenge_id":%q}`, challenge), fingerprint, string(result))
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/repository/account/authentication_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
