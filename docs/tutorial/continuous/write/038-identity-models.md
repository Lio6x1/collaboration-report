# 分步编写 `services/account/internal/domain/account/identity_models.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/038-identity-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达会话事实、登录交付结果、身份快照、业务拒绝与幂等写入绑定。

数据库保存摘要，浏览器拿原文；将 Session 与 LoginResult 分开能避免误把可登录秘密持久化。

写之前先按这条执行路线理解各段的连接：

1. Session 携带会话自身 Epoch 与 CredentialEpoch，Authenticate 还返回用户当前 IdentityEpoch。
2. Mutation 固定操作号、类型、路径、指纹和真实会话；MutationReceipt 用于核对重试是否为同一请求。

本文件需要守住的边界：

- 会话 Epoch、CredentialEpoch、IdentityEpoch 是不同撤销范围，不能互相代用。
- RuleViolation.Kind 决定传输错误类别，Code 保留具体业务拒绝原因。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/identity_models.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/identity_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/identity_models.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/identity_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:193 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"time"
)

// cursor.go：保留原实现，按业务合并文件。

````

## 第 2 步：CursorPayload

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type CursorPayload struct {`。

**这一段为什么这样写：** 游标内容既绑定目录范围又限制重放时间。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>string</code> | 账号、组织和目录修订组成的查询绑定。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Last</code> | <code>string</code> | 上一页最后一个规范十进制ID，下页使用id>Last。<br>序列化/校验标记：<code>`json:&quot;last&quot;`</code> |
| <code>Expires</code> | <code>int64</code> | 游标过期Unix秒，签发时+5分钟。<br>序列化/校验标记：<code>`json:&quot;expires&quot;`</code> |

<!-- write-step: 2 bytes=193:378 -->
````go
type CursorPayload struct {
	Binding string `json:"binding"`
	Last    string `json:"last"`
	Expires int64  `json:"expires"`
}

// model.go：保留原实现，按业务合并文件。

````

## 第 3 步：Session

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

Session是已持久化的会话事实；ID和CSRF只保存摘要，不包含浏览器持有的原始令牌。

本步声明：`type Session struct {`。

**这一段为什么这样写：** 把浏览器秘密与持久会话事实分离。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>&#91;&#93;byte</code> | 32字节会话令牌SHA256摘要，数据库session主键。 |
| <code>UserID</code> | <code>string</code> | 所属账号的十进制字符串ID。 |
| <code>CSRF</code> | <code>&#91;&#93;byte</code> | 独立CSRF令牌的SHA256摘要，用来比较X-CSRF-Token。 |
| <code>Epoch</code> | <code>uint64</code> | 此会话来源session:hash的撤权代次，退出时递增。 |
| <code>CredentialEpoch</code> | <code>string</code> | 创建会话时用户的凭据代次；改密/换邮箱后旧会话不再匹配。 |
| <code>State</code> | <code>string</code> | ACTIVE可认证；REVOKING正在停止旧输出；CLOSED仅供受限恢复。 |
| <code>Expires</code> | <code>time.Time</code> | 会话绝对失效时间，登录时设为UTC现在+12小时。 |

<!-- write-step: 3 bytes=378:687 -->
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

## 第 4 步：LoginUser

插入位置：第 3 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type LoginUser struct {`。

**这一段为什么这样写：** 只装登录第一轮bcrypt所需的用户快照。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 邮箱查得的真实用户ID。 |
| <code>DisplayName</code> | <code>string</code> | 成功登录后返回浏览器的展示名。 |
| <code>Hash</code> | <code>&#91;&#93;byte</code> | 该账号持久bcrypt密码哈希。 |
| <code>Algorithm</code> | <code>string</code> | 密码算法标识，当前只接受bcrypt。 |
| <code>State</code> | <code>string</code> | 用户状态，必须ACTIVE才可登录。 |
| <code>CredentialEpoch</code> | <code>string</code> | 锁外密码读取时的凭据代次，锁内必须仍相等。 |

<!-- write-step: 4 bytes=687:858 -->
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

## 第 5 步：Credential

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`type Credential struct {`。

**这一段为什么这样写：** 按调用目的承载密码或状态代次快照。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Hash</code> | <code>&#91;&#93;byte</code> | LockCredential/LockUser填充的密码哈希，供锁内核对；只查当前状态的方法可不填。 |
| <code>Epoch</code> | <code>string</code> | 用户credential_epoch的规范十进制字符串。 |
| <code>State</code> | <code>string</code> | 用户ACTIVE/REVOKING/CLOSED状态。 |
| <code>IdentityEpoch</code> | <code>uint64</code> | 用户identity_epoch，Authenticate成功时用于输出来源。 |

<!-- write-step: 5 bytes=858:974 -->
````go
type Credential struct {
	Hash          []byte
	Epoch         string
	State         string
	IdentityEpoch uint64
}

````

## 第 6 步：User

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`type User struct {`。

**这一段为什么这样写：** 向本人资料查询返回可见信息和状态版本。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 账号ID字符串。 |
| <code>Email</code> | <code>string</code> | 当前邮箱；关闭账号数据库NULL在查询中映射空串。 |
| <code>DisplayName</code> | <code>string</code> | 用户展示名。 |
| <code>State</code> | <code>string</code> | 当前账号生命周期。 |
| <code>CredentialEpoch</code> | <code>string</code> | 资料读取后需与Session一致的凭据代次。 |
| <code>IdentityEpoch</code> | <code>string</code> | 账号身份撤销代次的字符串表达。 |

<!-- write-step: 6 bytes=974:1140 -->
````go
type User struct {
	ID              string
	Email           string
	DisplayName     string
	State           string
	CredentialEpoch string
	IdentityEpoch   string
}

````

## 第 7 步：LoginResult

插入位置：第 6 步代码之后，紧接追加；完成稿约第 54 行。

LoginResult只在会话提交成功后交给传输层；令牌不能写入数据库或日志。

本步声明：`type LoginResult struct {`。

**这一段为什么这样写：** 仅表示已提交登录，允许handler交付一次原文凭据。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>UserID</code> | <code>string</code> | 已建立会话所属用户ID。 |
| <code>DisplayName</code> | <code>string</code> | 登录用户展示名。 |
| <code>SessionToken</code> | <code>string</code> | 会话随机字节的Base64URL原文，只交Session Cookie。 |
| <code>CSRFToken</code> | <code>string</code> | 独立随机CSRF原文，交可读Cookie和DTO供请求头使用。 |
| <code>Expires</code> | <code>time.Time</code> | 两份Cookie与持久会话共享的绝对到期。 |

<!-- write-step: 7 bytes=1140:1376 -->
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

## 第 8 步：Authentication

插入位置：第 7 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`type Authentication struct {`。

**这一段为什么这样写：** 将已核验会话与当前账号身份来源一起返回。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Session</code> | <code>Session</code> | 已检查ACTIVE、未过期且凭据代次匹配的会话。 |
| <code>IdentityEpoch</code> | <code>uint64</code> | 同次读取用户credential取得的当前身份代次，不是Session.Epoch。 |

<!-- write-step: 8 bytes=1376:1514 -->
````go
type Authentication struct {
	Session       Session
	IdentityEpoch uint64
}

// mutation.go：保留原实现，按业务合并文件。

````

## 第 9 步：RuleFailure

插入位置：第 8 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`type RuleFailure uint8`。

<!-- write-step: 9 bytes=1514:1538 -->
````go
type RuleFailure uint8

````

## 第 10 步：InvalidRequest、SessionRequired、PermissionDenied、EntityNotFound、StateConflict、TemporarilyUnavailable

插入位置：第 9 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`const (`。

<!-- write-step: 10 bytes=1538:1674 -->
````go
const (
	InvalidRequest RuleFailure = iota
	SessionRequired
	PermissionDenied
	EntityNotFound
	StateConflict
	TemporarilyUnavailable
)

````

## 第 11 步：RuleViolation

插入位置：第 10 步代码之后，紧接追加；完成稿约第 81 行。

RuleViolation表达业务拒绝的类别，传输层决定HTTP状态或RPC错误码。

本步声明：`type RuleViolation struct {`。

**这一段为什么这样写：** 让同一业务拒绝跨HTTP/RPC使用。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>RuleFailure</code> | InvalidRequest等领域分类，传输层映射400/401/403/404/409/503。 |
| <code>Code</code> | <code>string</code> | 可安全对外的具体稳定错误码。 |

<!-- write-step: 11 bytes=1674:1826 -->
````go
// RuleViolation表达业务拒绝的类别，传输层决定HTTP状态或RPC错误码。
type RuleViolation struct {
	Kind RuleFailure
	Code string
}

````

## 第 12 步：RuleViolation.Error

插入位置：第 11 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func (e RuleViolation) Error() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>e.Code</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return e.Code</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=1826:1883 -->
````go
func (e RuleViolation) Error() string { return e.Code }

````

## 第 13 步：Mutation

插入位置：第 12 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`type Mutation struct {`。

**这一段为什么这样写：** 固定一笔账号写入的真实身份与请求内容。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OperationID</code> | <code>string</code> | 客户端原操作号，用作唯一幂等回执键。 |
| <code>Kind</code> | <code>string</code> | 当前业务动作，如ORG_APPLY，用于防同号跨动作复用。 |
| <code>Path</code> | <code>string</code> | 实际请求路径，参与请求指纹及审计target。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 路径与规范输入的HMAC摘要，不保存密码原文。 |
| <code>Session</code> | <code>Session</code> | 真实会话事实，事务内重新锁user/session核验。 |

<!-- write-step: 13 bytes=1883:2010 -->
````go
type Mutation struct {
	OperationID string
	Kind        string
	Path        string
	Fingerprint []byte
	Session     Session
}

````

## 第 14 步：MutationReceipt

插入位置：第 13 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`type MutationReceipt struct {`。

**这一段为什么这样写：** 让重复请求返回原事实而非再次执行body。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ActorID</code> | <code>string</code> | 原操作的真实账号，必须等于当前session.UserID。 |
| <code>Kind</code> | <code>string</code> | 原业务类型，必须与重试相等。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 原请求HMAC，必须逐字节相同。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 持久JSON结果；空结果意味着尚待恢复，不执行第二次业务。 |

<!-- write-step: 14 bytes=2010:2122 -->
````go
type MutationReceipt struct {
	ActorID     string
	Kind        string
	Fingerprint []byte
	Result      []byte
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/identity_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
