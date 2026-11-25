# 分步编写 `services/account/internal/domain/account/authentication_models.go`

先按 [A01 · 安全登录、验证码、注册与凭据变更](../lessons/account/01-authentication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/034-authentication-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义验证码绑定、凭据变更计划以及密码和令牌的基础规则。

验证码验证、密码变更和重试需要共享同一组明确类型；将纯校验放在领域层使 HTTP 与后台调用遵守相同边界。

写之前先按这条执行路线理解各段的连接：

1. ChallengeMeta 同时绑定用途、邮箱摘要、用户、凭据代次和发起人，防止换邮箱或换用途复用验证码。
2. RandomToken 生成32随机字节，返回浏览器原文和数据库摘要；TokenHash 反向严格检查规范编码。
3. ValidNewPassword 在 bcrypt 的72字节边界内检查至少8字符和两类字符；旧密码验证只用 ValidPassword，避免规则升级锁住旧账号。

本文件需要守住的边界：

- 不能把 SessionToken、验证码或密码原文写进回执；Fingerprint 使用 HMAC。
- CredentialEpoch 拒绝0、最大uint64和带前导零的数字，给后续 epoch+1 留空间。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/authentication_models.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/authentication_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/authentication_models.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/authentication_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:366 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// challenge.go：保留原实现，按业务合并文件。

````

## 第 2 步：ChallengeMeta

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type ChallengeMeta struct {`。

**这一段为什么这样写：** 把验证码和用途、接收对象、账号代次及发起人一起绑定；只比较验证码数字会让原本有效的证明被挪到另一项操作。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Purpose</code> | <code>string</code> | SIGNUP、RESET_PASSWORD或CHANGE_EMAIL，限制验证码使用场景。<br>序列化/校验标记：<code>`json:&quot;purpose&quot;`</code> |
| <code>Subject</code> | <code>string</code> | 规范邮箱SHA256十六进制摘要，不是邮箱原文。<br>序列化/校验标记：<code>`json:&quot;subject&quot;`</code> |
| <code>UserID</code> | <code>string</code> | 验证码绑定账号；未知找回对象用0避免枚举。<br>序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 签发时credential_epoch，后续改密会使证明过时。<br>序列化/校验标记：<code>`json:&quot;credential_epoch&quot;`</code> |
| <code>ActorID</code> | <code>string</code> | 换邮箱时的真实发起者，必须等于当前会话用户。<br>序列化/校验标记：<code>`json:&quot;actor_id&quot;`</code> |

<!-- write-step: 2 bytes=366:572 -->
````go
type ChallengeMeta struct {
	Purpose string `json:"purpose"`
	Subject string `json:"subject"`
	UserID  string `json:"user_id"`
	Epoch   string `json:"credential_epoch"`
	ActorID string `json:"actor_id"`
}

````

## 第 3 步：IssuedChallenge

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type IssuedChallenge struct {`。

**这一段为什么这样写：** 签发完成后只需把挑战编号和到期时间交给调用方；验证码原文由投递链路处理，不放进这个返回模型。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 挑战唯一号，同时参与HMAC(id+换行+code)。 |
| <code>Expires</code> | <code>time.Time</code> | 挑战约10分钟后到期的UTC时间。 |

<!-- write-step: 3 bytes=572:640 -->
````go
type IssuedChallenge struct {
	ID      string
	Expires time.Time
}

````

## 第 4 步：PublicReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`type PublicReceipt struct {`。

**这一段为什么这样写：** 公开恢复入口不能只凭操作编号取结果；同时保存操作类型和输入指纹，让重试先证明自己对应原请求，再返回原持久结果。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Kind</code> | <code>string</code> | SIGNUP/RESET_PASSWORD等公开可恢复操作类型。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 包含敏感输入的HMAC，公开读取必须相等。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 原操作持久结果JSON，空值表示未结束。 |

<!-- write-step: 4 bytes=640:794 -->
````go
type PublicReceipt struct {
	Kind        string
	Fingerprint []byte
	Result      []byte
}

// credential.go：保留原实现，按业务合并文件。

````

## 第 5 步：CredentialChange

插入位置：第 4 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`type CredentialChange struct {`。

**这一段为什么这样写：** 将事务前验证过的密码哈希、会话或验证码证明与预期代次合成一次变更输入；事务内仍要检查这些事实是否已被并发操作改变。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 凭据变更原操作号。 |
| <code>Kind</code> | <code>string</code> | CHANGE_PASSWORD、RESET_PASSWORD或CHANGE_EMAIL。 |
| <code>UserID</code> | <code>string</code> | 目标本人账号ID。 |
| <code>ExpectedEpoch</code> | <code>string</code> | 请求者观察到的原credential_epoch，事务内比较。 |
| <code>Session</code> | <code>*Session</code> | 已登录修改时绑定的真实会话；密码找回可为空。 |
| <code>Proof</code> | <code>*ChallengeMeta</code> | 经Redis校验的挑战元数据，需MySQL消费唯一键。 |
| <code>ChallengeID</code> | <code>string</code> | 将被INSERT到verification_consumption的验证码编号。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 原输入HMAC，用于同号重试核对。 |
| <code>NewHash</code> | <code>&#91;&#93;byte</code> | 锁外计算的新bcrypt哈希，非空才更新密码。 |
| <code>OldHash</code> | <code>&#91;&#93;byte</code> | 改密前成功验证的旧hash，锁内必须仍相等。 |
| <code>NewEmail</code> | <code>string</code> | 规范化的新邮箱，非空才更新email/email_key。 |

<!-- write-step: 5 bytes=794:1080 -->
````go
type CredentialChange struct {
	ID            string
	Kind          string
	UserID        string
	ExpectedEpoch string
	Session       *Session
	Proof         *ChallengeMeta
	ChallengeID   string
	Fingerprint   []byte
	NewHash       []byte
	OldHash       []byte
	NewEmail      string
}

````

## 第 6 步：CredentialReceipt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`type CredentialReceipt struct {`。

**这一段为什么这样写：** 在通用操作回执上增加凭据输入指纹，使同一个操作号不能被另一组新密码或新邮箱输入复用。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OperationReceipt</code> | <code>OperationReceipt</code> | 继承原actor/kind/state/sessionHash绑定。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 原凭据变更输入HMAC。 |

<!-- write-step: 6 bytes=1080:1153 -->
````go
type CredentialReceipt struct {
	OperationReceipt
	Fingerprint []byte
}

````

## 第 7 步：CredentialRetry

插入位置：第 6 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`type CredentialRetry struct {`。

**这一段为什么这样写：** 改密会关闭原会话，但原请求仍可能因断网需要查结果；因此保留原会话事实和操作状态用于恢复核验，不能把恢复路径当成重新登录。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Session</code> | <code>Session</code> | 原会话事实，可能已失效，仅供原操作及CSRF核验。 |
| <code>State</code> | <code>string</code> | 原凭据操作当前持久状态。 |

<!-- write-step: 7 bytes=1153:1219 -->
````go
type CredentialRetry struct {
	Session Session
	State   string
}

````

## 第 8 步：CredentialPreparation

插入位置：第 7 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`type CredentialPreparation struct {`。

**这一段为什么这样写：** 在关闭旧凭据来源前固定本次变更的范围、旧代次和恢复载荷；后台恢复必须沿用这份准备结果，不能临时重新选择目标。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Change</code> | <code>CredentialChange</code> | 已核验凭据变更输入与证明。 |
| <code>Epoch</code> | <code>uint64</code> | 被关闭的旧credential代次，绝非递增后代次。 |
| <code>Scope</code> | <code>string</code> | credential:用户ID来源键。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 固定目标JSON，含user_id/session_hash/scope_key。 |
| <code>Pending</code> | <code>&#91;&#93;byte</code> | PREPARING操作DTO的JSON，供轮询。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | SessionClosure JSON，供worker按原范围恢复。 |

<!-- write-step: 8 bytes=1219:1364 -->
````go
type CredentialPreparation struct {
	Change  CredentialChange
	Epoch   uint64
	Scope   string
	Target  []byte
	Pending []byte
	Payload []byte
}

````

## 第 9 步：CredentialEpoch

插入位置：第 8 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func CredentialEpoch(value string) (uint64, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(epoch, 10) == value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>epoch</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; epoch != 0 &amp;&amp; epoch != ^uint64(0) &amp;&amp; strconv.FormatUint(epoch, 10) == value</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return epoch, err == nil &amp;&amp; epoch != 0 &amp;&amp; epoch != ^uint64(0) &amp;&amp; strconv.FormatUint(epoch, 10) == value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1364:1632 -->
````go
func CredentialEpoch(value string) (uint64, bool) {
	epoch, err := strconv.ParseUint(value, 10, 64)
	return epoch, err == nil && epoch != 0 && epoch != ^uint64(0) && strconv.FormatUint(epoch, 10) == value
}

// secrets.go：保留原实现，按业务合并文件。

````

## 第 10 步：Secrets

插入位置：第 9 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`type Secrets struct{ key []byte }`。

**这一段为什么这样写：** 集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 构造时复制的HMAC密钥字节，保护验证码摘要、请求指纹和游标。 |

<!-- write-step: 10 bytes=1632:1667 -->
````go
type Secrets struct{ key []byte }

````

## 第 11 步：NewSecrets

插入位置：第 10 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func NewSecrets(key []byte) *Secrets`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>key: append(&#91;&#93;byte(nil), key...)</code>；<code>append(&#91;&#93;byte(nil), key...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Secrets</code> | 使用 <code>Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>实际返回表达式：<code>&amp;Secrets{key: append(&#91;&#93;byte(nil), key...)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Secrets{key: append(&#91;&#93;byte(nil), key...)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=1667:1759 -->
````go
func NewSecrets(key []byte) *Secrets { return &Secrets{key: append([]byte(nil), key...)} }

````

## 第 12 步：SubjectDigest

插入位置：第 11 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func SubjectDigest(email string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>email</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(email)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(value&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(value&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=1759:1880 -->
````go
func SubjectDigest(email string) string {
	value := sha256.Sum256([]byte(email))
	return hex.EncodeToString(value[:])
}

````

## 第 13 步：Secrets.Digest

插入位置：第 12 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func (s *Secrets) Digest(value string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>hex.EncodeToString(mac.Sum(nil))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(mac.Sum(nil))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=1880:2041 -->
````go
func (s *Secrets) Digest(value string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

````

## 第 14 步：Secrets.Fingerprint

插入位置：第 13 步代码之后，紧接追加；完成稿约第 98 行。

Fingerprint保持原JSON/HMAC编码，密码和验证码不能以明文写入receipt。

本步声明：`func (s *Secrets) Fingerprint(input any) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>digest</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return digest</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=2041:2343 -->
````go
// Fingerprint保持原JSON/HMAC编码，密码和验证码不能以明文写入receipt。
func (s *Secrets) Fingerprint(input any) []byte {
	raw, _ := json.Marshal(input)
	digest, _ := hex.DecodeString(s.Digest(string(raw)))
	return digest
}

// token.go：保留原实现，按业务合并文件。

````

## 第 15 步：RandomToken

插入位置：第 14 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`func RandomToken() (string, []byte, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>token</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>hash&#91;:&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return token, hash&#91;:&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=2343:2592 -->
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

## 第 16 步：TokenHash

插入位置：第 15 步代码之后，紧接追加；完成稿约第 117 行。

本步声明：`func TokenHash(value string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>base64.RawURLEncoding.DecodeString(value)</code>；<code>base64.RawURLEncoding.EncodeToString(raw) != value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>hash&#91;:&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SESSION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(raw) != 32 &#124;&#124; base64.RawURLEncoding.EncodeToString(raw) != value</code>。条件成立时执行 <code>return nil, errors.New(&quot;SESSION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return hash&#91;:&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=2592:2945 -->
````go
func TokenHash(value string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
		return nil, errors.New("SESSION_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}

// validation.go：保留原实现，按业务合并文件。

````

## 第 17 步：ValidPassword

插入位置：第 16 步代码之后，紧接追加；完成稿约第 128 行。

bcrypt按UTF-8字节限制密码，不能以字符数检查或静默截断中文密码。

本步声明：`func ValidPassword(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>len(&#91;&#93;byte(value)) &gt;= 1 &amp;&amp; len(&#91;&#93;byte(value)) &lt;= 72</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return len(&#91;&#93;byte(value)) &gt;= 1 &amp;&amp; len(&#91;&#93;byte(value)) &lt;= 72</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=2945:3141 -->
````go
// bcrypt按UTF-8字节限制密码，不能以字符数检查或静默截断中文密码。
func ValidPassword(value string) bool {
	return len([]byte(value)) >= 1 && len([]byte(value)) <= 72
}

````

## 第 18 步：ValidNewPassword

插入位置：第 17 步代码之后，紧接追加；完成稿约第 133 行。

新密码单独执行强度规则，登录/当前密码仍按原哈希验证，避免锁住已有账号。
大小写均属于英文字母类，但原文直接交给bcrypt，不能转小写或去除首尾字符。

本步声明：`func ValidNewPassword(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>ValidPassword(value)</code>；<code>utf8.ValidString(value)</code>；<code>utf8.RuneCountInString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>letters &amp;&amp; digits &#124;&#124; letters &amp;&amp; symbols &#124;&#124; digits &amp;&amp; symbols</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidPassword(value) &#124;&#124; !utf8.ValidString(value) &#124;&#124; utf8.RuneCountInString(value) &lt; 8</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return letters &amp;&amp; digits &#124;&#124; letters &amp;&amp; symbols &#124;&#124; digits &amp;&amp; symbols</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=3141:4027 -->
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

## 第 19 步：ValidEmail

插入位置：第 18 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`func ValidEmail(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>value != strings.TrimSpace(value)</code>；<code>strings.TrimSpace(value)</code>；<code>mail.ParseAddress(value)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; parsed.Address == value</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(value) &gt; 254 &#124;&#124; value != strings.TrimSpace(value)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; parsed.Address == value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=4027:4231 -->
````go
func ValidEmail(value string) bool {
	if len(value) > 254 || value != strings.TrimSpace(value) {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/authentication_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/01-authentication.md)。
