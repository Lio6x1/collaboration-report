# 分步编写 `services/account/internal/domain/account/organization_models.go`

先按 [A03 · 组织、邀请申请、成员变更与组织关闭](../lessons/account/03-organizations.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/042-organization-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。

邀请原文需要重新分享但不能明文入库；AES-GCM用组织ID作附加认证数据，把密文与组织绑定。 组织详情还需单独携带当前操作者的已核验成员事实，让管理资格不依赖当前成员展示页是否恰好包含本人。

写之前先按这条执行路线理解各段的连接：

1. InvitationCodec.Encrypt 将随机nonce前置，Decrypt 用同一组织ID验证密文。
2. OrganizationCloseEligible 要求ACTIVE、本人为owner、修订一致且存续成员恰好1人。
3. DecodeOrganizationClosePlan 严格检查固定binding、32字节摘要及文档游标。
4. Organization.CurrentMember由AccessOrganization读取当前会话本人ACTIVE成员后设置，OrganizationDTO只投影user_id/role/state/revision；列表未填此字段不能反向推导本人无权。

本文件需要守住的边界：

- REVOKING成员仍算存续成员，不能在撤权未知期间清理其数据。
- MemberRevision是目录集合版本，成员Epoch是撤权来源代次，两者不能混同。
- CurrentMember不是浏览器提交的角色，也不是成员目录第一页里的某一行；退出/改角色仍须服务端按当前事实与成员revision检查。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/organization_models.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/organization_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/organization_models.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/organization_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:350 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	"collaboration/platform"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：OrganizationApplication

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type OrganizationApplication struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 加入申请的稳定编号。 |
| <code>OrganizationID</code> | <code>string</code> | 组织的稳定标识，关联成员、角色和组织生命周期；显示名称改动不改变这个关联。 |
| <code>ApplicantID</code> | <code>string</code> | 申请加入组织的账号。 |
| <code>Reason</code> | <code>string</code> | 申请人填写的加入理由。 |
| <code>State</code> | <code>string</code> | 申请当前待审/已处理等状态。 |
| <code>Revision</code> | <code>string</code> | 申请修订号，防止重复或过期审批覆盖新状态。 |
| <code>SourceInvitationID</code> | <code>*string</code> | 可选邀请来源；直接申请时可以为空。 |
| <code>InvitationValidatedAt</code> | <code>*time.Time</code> | 提交申请时完成邀请有效性验证的时间，审批沿用固定来源事实。 |

<!-- write-step: 2 bytes=350:699 -->
````go
type OrganizationApplication struct {
	ID                    string
	OrganizationID        string
	ApplicantID           string
	Reason                string
	State                 string
	Revision              string
	SourceInvitationID    *string
	InvitationValidatedAt *time.Time
}

// invitation.go：保留原实现，按业务合并文件。

````

## 第 3 步：OrganizationInvitation

插入位置：第 2 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`type OrganizationInvitation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 邀请记录编号。 |
| <code>OrganizationID</code> | <code>string</code> | 组织的稳定标识，关联成员、角色和组织生命周期；显示名称改动不改变这个关联。 |
| <code>State</code> | <code>string</code> | 邀请当前是否可用或已撤销。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |
| <code>Revision</code> | <code>string</code> | 邀请修订号，管理修改需防止覆盖并发变化。 |

<!-- write-step: 3 bytes=699:857 -->
````go
type OrganizationInvitation struct {
	ID             string
	OrganizationID string
	State          string
	Expires        time.Time
	Revision       string
}

````

## 第 4 步：InvitationShare

插入位置：第 3 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`type InvitationShare struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 邀请记录编号。 |
| <code>Token</code> | <code>string</code> | 授权后可分享的邀请原文，数据库另存摘要/密文；不能写进普通日志。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 4 bytes=857:941 -->
````go
type InvitationShare struct {
	ID      string
	Token   string
	Expires time.Time
}

````

## 第 5 步：InvitationIdentity

插入位置：第 4 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`type InvitationIdentity struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 通过分享令牌核验后定位的邀请编号。 |
| <code>OrganizationID</code> | <code>string</code> | 组织的稳定标识，关联成员、角色和组织生命周期；显示名称改动不改变这个关联。 |

<!-- write-step: 5 bytes=941:1092 -->
````go
type InvitationIdentity struct {
	ID             string
	OrganizationID string
}

// invitation_codec.go：保留原实现，按业务合并文件。

````

## 第 6 步：InvitationCodec

插入位置：第 5 步代码之后，紧接追加；完成稿约第 55 行。

InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。

本步声明：`type InvitationCodec struct{ key []byte }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 从实例秘密配置注入并复制的AES密钥；组织ID作为附加认证数据，不能串组织解密。 |

<!-- write-step: 6 bytes=1092:1231 -->
````go
// InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。
type InvitationCodec struct{ key []byte }

````

## 第 7 步：NewInvitationCodec

插入位置：第 6 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func NewInvitationCodec(key []byte) *InvitationCodec`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>key: append(&#91;&#93;byte(nil), key...)</code>；<code>append(&#91;&#93;byte(nil), key...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*InvitationCodec</code> | 使用 <code>InvitationCodec</code> 类型；其定义处的职责：InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。<br>实际返回表达式：<code>&amp;InvitationCodec{key: append(&#91;&#93;byte(nil), key...)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;InvitationCodec{key: append(&#91;&#93;byte(nil), key...)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1231:1348 -->
````go
func NewInvitationCodec(key []byte) *InvitationCodec {
	return &InvitationCodec{key: append([]byte(nil), key...)}
}

````

## 第 8 步：InvitationCodec.cipher

插入位置：第 7 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func (c *InvitationCodec) cipher() (cipher.AEAD, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>cipher.AEAD</code> | 由 <code>cipher.NewGCM(block)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>cipher.NewGCM(block)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return cipher.NewGCM(block)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1348:1512 -->
````go
func (c *InvitationCodec) cipher() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

````

## 第 9 步：InvitationCodec.Encrypt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func (c *InvitationCodec) Encrypt(token, organization string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(token)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(organization)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>aead.Seal(nonce, nonce, &#91;&#93;byte(token), &#91;&#93;byte(organization))</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return aead.Seal(nonce, nonce, &#91;&#93;byte(token), &#91;&#93;byte(organization)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1512:1839 -->
````go
func (c *InvitationCodec) Encrypt(token, organization string) ([]byte, error) {
	aead, err := c.cipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(token), []byte(organization)), nil
}

````

## 第 10 步：InvitationCodec.Decrypt

插入位置：第 9 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func (c *InvitationCodec) Decrypt(value []byte, organization string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>aead.Open(nil, value&#91;:aead.NonceSize()&#93;, value&#91;aead.NonceSize():&#93;, &#91;&#93;byte(organization))</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(organization)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>string(plain)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>RuleViolation{Kind: TemporarilyUnavailable, Code: &quot;INVITATION_KEY_UNAVAILABLE&quot;}</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(value) &lt; aead.NonceSize()</code>。条件成立时执行 <code>return &quot;&quot;, RuleViolation{Kind: TemporarilyUnavailable, Code: &quot;INVITATION_KEY_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return string(plain), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=1839:2296 -->
````go
func (c *InvitationCodec) Decrypt(value []byte, organization string) (string, error) {
	aead, err := c.cipher()
	if err != nil || len(value) < aead.NonceSize() {
		return "", RuleViolation{Kind: TemporarilyUnavailable, Code: "INVITATION_KEY_UNAVAILABLE"}
	}
	plain, err := aead.Open(nil, value[:aead.NonceSize()], value[aead.NonceSize():], []byte(organization))
	return string(plain), err
}

// organization.go：保留原实现，按业务合并文件。

````

## 第 11 步：Organization

插入位置：第 10 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`type Organization struct {`。

**这一段为什么这样写：** 区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>CurrentMember</code> | <code>*OrganizationMember</code> | 可空的*OrganizationMember，由当前详情准入填入本人UserID/Role/State/Revision；不新增数据库列，不把Epoch或IdentityEpoch放进公开成员身份投影。 |
| <code>ID</code> | <code>string</code> | 组织稳定编号，展示时可补零但存储关系不改。 |
| <code>Name</code> | <code>string</code> | 组织可读名称，可以修改，不能做权限主键。 |
| <code>OwnerID</code> | <code>string</code> | 组织所有者账号，交接所有者与修改普通角色走不同规则。 |
| <code>Lifecycle</code> | <code>string</code> | ACTIVE/CLOSING/CLOSED等存续阶段。 |
| <code>Revision</code> | <code>string</code> | 组织控制信息修订号。 |
| <code>MemberRevision</code> | <code>string</code> | 组织成员集合修订号，识别成员列表是否变化。 |

<!-- write-step: 11 bytes=2296:2500 -->
````go
type Organization struct {
	CurrentMember  *OrganizationMember
	ID             string
	Name           string
	OwnerID        string
	Lifecycle      string
	Revision       string
	MemberRevision string
}

````

## 第 12 步：OrganizationEntry

插入位置：第 11 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`type OrganizationEntry struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Organization</code> | <code>Organization</code> | 组织目录中的基础组织信息。 |
| <code>MemberEpoch</code> | <code>uint64</code> | 当前浏览者在这个组织中的成员来源代次。 |

<!-- write-step: 12 bytes=2500:2583 -->
````go
type OrganizationEntry struct {
	Organization Organization
	MemberEpoch  uint64
}

````

## 第 13 步：OrganizationMember

插入位置：第 12 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`type OrganizationMember struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>UserID</code> | <code>string</code> | 该成员的原始十进制账号ID；公开补零展示不能改变它。 |
| <code>DisplayName</code> | <code>string</code> | Account权威昵称，目录从user表读取；单独本人准入不需要依赖该昵称。 |
| <code>Role</code> | <code>string</code> | 当前成员角色MEMBER/ADMIN等，所有者身份另由组织OwnerID判断。 |
| <code>State</code> | <code>string</code> | 成员关系状态，撤权中不能视为已经不存在。 |
| <code>Revision</code> | <code>string</code> | 这条成员关系修订号。 |
| <code>Epoch</code> | <code>uint64</code> | 此成员来源代次，重验和撤权围栏按此认回。 |
| <code>IdentityEpoch</code> | <code>uint64</code> | 姓名和身份投影依赖的当前用户来源代次，输出捕获后在闸门重验。 |
| <code>KnowledgeBaseCreation</code> | <code>string</code> | ROLE/GRANTED/NONE目录说明，不能代替下一次真实建库准入。 |

<!-- write-step: 13 bytes=2583:2888 -->
````go
type OrganizationMember struct {
	UserID        string
	DisplayName   string
	Role          string
	State         string
	Revision      string
	Epoch         uint64
	IdentityEpoch uint64
	// 仅用于成员目录的状态说明，实际建库仍由Identity重新准入。
	KnowledgeBaseCreation string
}

````

## 第 14 步：ValidEntityID

插入位置：第 13 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func ValidEntityID(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(number, 10) == value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; number &gt; 0 &amp;&amp; strconv.FormatUint(number, 10) == value</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err == nil &amp;&amp; number &gt; 0 &amp;&amp; strconv.FormatUint(number, 10) == value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=2888:3127 -->
````go
func ValidEntityID(value string) bool {
	number, err := strconv.ParseUint(value, 10, 64)
	return err == nil && number > 0 && strconv.FormatUint(number, 10) == value
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 15 步：OrganizationClosePlan

插入位置：第 14 步代码之后，紧接追加；完成稿约第 127 行。

Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。

本步声明：`type OrganizationClosePlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>platform.OrganizationClosureBinding</code> | 原组织关闭绑定，包含固定操作者、组织、操作与代次。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>PlanDigest</code> | <code>string</code> | 预览/执行计划的内容摘要，确认时据此绑定原计划；不能拿新计划冒充用户此前确认的内容。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |
| <code>AfterDocument</code> | <code>string</code> | 只有Doc确认原文档清理证明后才推进的游标。<br>序列化/校验标记：<code>`json:&quot;after_document&quot;`</code> |

<!-- write-step: 15 bytes=3127:3472 -->
````go
// Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。
type OrganizationClosePlan struct {
	Binding       platform.OrganizationClosureBinding `json:"binding"`
	PlanDigest    string                              `json:"plan_digest"`
	AfterDocument string                              `json:"after_document"`
}

````

## 第 16 步：DecodeOrganizationClosePlan

插入位置：第 15 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OrganizationClosePlan</code> | 使用 <code>OrganizationClosePlan</code> 类型；其定义处的职责：Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>d.Decode(&amp;plan) != nil &#124;&#124; d.Decode(new(any)) != io.EOF &#124;&#124; !plan.Binding.Valid() &#124;&#124; (plan.AfterDocument != &quot;&quot; &amp;&amp; (!ValidEntityID(plan.AfterDocument) &#124;&#124; plan.PlanDigest == &quot;&quot;))</code>。条件成立时执行 <code>return plan, errors.New(&quot;ORGANIZATION_CLOSE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.PlanDigest != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != plan.PlanDigest</code>。条件成立时执行 <code>return plan, errors.New(&quot;ORGANIZATION_CLOSE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=3472:4155 -->
````go
func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error) {
	var plan OrganizationClosePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&plan) != nil || d.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || (plan.AfterDocument != "" && (!ValidEntityID(plan.AfterDocument) || plan.PlanDigest == "")) {
		return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
	}
	if plan.PlanDigest != "" {
		digest, err := hex.DecodeString(plan.PlanDigest)
		if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != plan.PlanDigest {
			return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
		}
	}
	return plan, nil
}

````

## 第 17 步：OrganizationCloseEligible

插入位置：第 16 步代码之后，紧接追加；完成稿约第 150 行。

唯一存续成员包括撤权中的成员，不能趁撤权未知期间清除他人数据。

本步声明：`func OrganizationCloseEligible(org Organization, actor, expected string, member OrganizationMember, liveMembers int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>org</code> | <code>Organization</code> | 使用 <code>Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。<br>在此声明中的实际使用：<code>org.Lifecycle</code>；<code>org.OwnerID</code>；<code>org.Revision</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>org.OwnerID != actor</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>org.Revision != expected</code> |
| <code>member</code> | <code>OrganizationMember</code> | 在此声明中的实际使用：<code>member.State</code>；<code>member.Epoch</code> |
| <code>liveMembers</code> | <code>int</code> | 在此声明中的实际使用：<code>liveMembers != 1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>RuleViolation{Kind: StateConflict, Code: &quot;ORGANIZATION_CLOSE_PENDING&quot;}</code>；<code>RuleViolation{Kind: PermissionDenied, Code: &quot;ORGANIZATION_OWNER_REQUIRED&quot;}</code>；<code>RuleViolation{Kind: StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>；<code>RuleViolation{Kind: StateConflict, Code: &quot;ORGANIZATION_OWNER_HANDOFF_REQUIRED&quot;}</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>org.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; member.State != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return RuleViolation{Kind: StateConflict, Code: &quot;ORGANIZATION_CLOSE_PENDING&quot;}</code>。当前函数在这里结束。
- 检查 <code>org.OwnerID != actor</code>。条件成立时执行 <code>return RuleViolation{Kind: PermissionDenied, Code: &quot;ORGANIZATION_OWNER_REQUIRED&quot;}</code>。当前函数在这里结束。
- 检查 <code>org.Revision != expected</code>。条件成立时执行 <code>return RuleViolation{Kind: StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 检查 <code>liveMembers != 1</code>。条件成立时执行 <code>return RuleViolation{Kind: StateConflict, Code: &quot;ORGANIZATION_OWNER_HANDOFF_REQUIRED&quot;}</code>。当前函数在这里结束。
- 检查 <code>member.Epoch == 0 &#124;&#124; member.Epoch == ^uint64(0)</code>。条件成立时执行 <code>return RuleViolation{Kind: StateConflict, Code: &quot;EPOCH_EXHAUSTED&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=4155:5064 -->
````go
// 唯一存续成员包括撤权中的成员，不能趁撤权未知期间清除他人数据。
func OrganizationCloseEligible(org Organization, actor, expected string, member OrganizationMember, liveMembers int) error {
	if org.Lifecycle != "ACTIVE" || member.State != "ACTIVE" {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_CLOSE_PENDING"}
	}
	if org.OwnerID != actor {
		return RuleViolation{Kind: PermissionDenied, Code: "ORGANIZATION_OWNER_REQUIRED"}
	}
	if org.Revision != expected {
		return RuleViolation{Kind: StateConflict, Code: "REVISION_CHANGED"}
	}
	if liveMembers != 1 {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_OWNER_HANDOFF_REQUIRED"}
	}
	if member.Epoch == 0 || member.Epoch == ^uint64(0) {
		return RuleViolation{Kind: StateConflict, Code: "EPOCH_EXHAUSTED"}
	}
	return nil
}

// organization_name.go：保留原实现，按业务合并文件。

````

## 第 18 步：NormalizeOrganizationName

插入位置：第 17 步代码之后，紧接追加；完成稿约第 172 行。

本步声明：`func NormalizeOrganizationName(name string) (string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>name</code> | <code>string</code> | 在此声明中的实际使用：<code>name = strings.TrimSpace(name)</code>；<code>strings.TrimSpace(name)</code>；<code>&#91;&#93;rune(name)</code>；<code>return name, length &gt;= 1 &amp;&amp; length &lt;= 160</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>name</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>length &gt;= 1 &amp;&amp; length &lt;= 160</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return name, length &gt;= 1 &amp;&amp; length &lt;= 160</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=5064:5231 -->
````go
func NormalizeOrganizationName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	length := len([]rune(name))
	return name, length >= 1 && length <= 160
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/organization_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/03-organizations.md)。
