# 分步编写 `packages/platform/authority_rpc_protocol.go`

先按 [F04 · 原操作、资格来源与跨服务身份](../lessons/foundation/04-original-operation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/019-authority-rpc-protocol.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。

这些协议只传固定编号、摘要和资格来源，不能复制正文或替代冻结protobuf授权契约。

写之前先按这条执行路线理解各段的连接：

1. AccountDocumentCleanupContext把原操作、文档、doc-acl旧epoch和revocation纳入域隔离摘要。
2. 通知快照携带全部source及期限；组织关闭分页同时校验固定binding/摘要/游标。
3. PersonalTransferStorageMeasurement返回固定原号/代次/实际字节摘要及SourceClosed，Doc据此计费。

本文件需要守住的边界：

- JS边界的64位ID和字节数用规范字符串；CLEANED只代表Node该文档私有工作区清理，不是账号全局成功。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/authority_rpc_protocol.go') -and (Get-Item -LiteralPath 'packages/platform/authority_rpc_protocol.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/authority_rpc_protocol.go')) { New-Item -ItemType File -Path 'packages/platform/authority_rpc_protocol.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:265 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountDocumentCleanupRequest

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。

本步声明：`type AccountDocumentCleanupRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原账号或组织清理所属操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Document</code> | <code>string</code> | 待清Node私有工作区的规范文档ID字符串。<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |

<!-- write-step: 2 bytes=265:498 -->
````go
// Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。
type AccountDocumentCleanupRequest struct {
	Operation string `json:"operation_id"`
	Document  string `json:"document_id"`
}

````

## 第 3 步：AccountDocumentCleanupContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type AccountDocumentCleanupContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AccountDocumentCleanupRequest</code> | <code>AccountDocumentCleanupRequest</code> | 继承原操作号与精确文档ID。 |
| <code>Scope</code> | <code>string</code> | 固定为doc-acl:文档ID的来源关闭键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 被关闭的旧doc-acl代次，规范十进制字符串。<br>序列化/校验标记：<code>`json:&quot;closed_epoch&quot;`</code> |
| <code>Revocation</code> | <code>string</code> | 执行该文档来源屏障的原撤销操作号。<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |
| <code>Digest</code> | <code>string</code> | 对操作/文档/scope/epoch/revocation作域隔离SHA256后的hex。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |

<!-- write-step: 3 bytes=498:737 -->
````go
type AccountDocumentCleanupContext struct {
	AccountDocumentCleanupRequest
	Scope      string `json:"scope_key"`
	Epoch      string `json:"closed_epoch"`
	Revocation string `json:"revocation_id"`
	Digest     string `json:"plan_digest"`
}

````

## 第 4 步：AccountDocumentCleanupReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

CLEANED是Node已持久清除私有工作区的回执，不等同整个账号注销完成。

本步声明：`type AccountDocumentCleanupReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AccountDocumentCleanupContext</code> | <code>AccountDocumentCleanupContext</code> | 原清理上下文完整绑定，返回必须精确匹配。 |
| <code>State</code> | <code>string</code> | CLEANED表示Node已持久清除此文档私有工作区，并非全账号注销终态。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |

<!-- write-step: 4 bytes=737:940 -->
````go
// CLEANED是Node已持久清除私有工作区的回执，不等同整个账号注销完成。
type AccountDocumentCleanupReceipt struct {
	AccountDocumentCleanupContext
	State string `json:"state"`
}

````

## 第 5 步：canonicalCleanupID

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func canonicalCleanupID(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>strconv.FormatUint(id, 10) == value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>err == nil &amp;&amp; id != 0 &amp;&amp; strconv.FormatUint(id, 10) == value</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err == nil &amp;&amp; id != 0 &amp;&amp; strconv.FormatUint(id, 10) == value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=940:1102 -->
````go
func canonicalCleanupID(value string) bool {
	id, err := strconv.ParseUint(value, 10, 64)
	return err == nil && id != 0 && strconv.FormatUint(id, 10) == value
}

````

## 第 6 步：AccountDocumentCleanupRequest.Valid

插入位置：第 5 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func (r AccountDocumentCleanupRequest) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>accountClosureOperation.MatchString(r.Operation) &amp;&amp; canonicalCleanupID(r.Document)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return accountClosureOperation.MatchString(r.Operation) &amp;&amp; canonicalCleanupID(r.Document)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1102:1250 -->
````go
func (r AccountDocumentCleanupRequest) Valid() bool {
	return accountClosureOperation.MatchString(r.Operation) && canonicalCleanupID(r.Document)
}

````

## 第 7 步：NewAccountDocumentCleanupContext

插入位置：第 6 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func NewAccountDocumentCleanupContext(r AccountDocumentCleanupRequest, epoch uint64, revocation string) AccountDocumentCleanupContext`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>AccountDocumentCleanupRequest</code> | 使用 <code>AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>AccountDocumentCleanupRequest: r</code>；<code>r.Document</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(epoch, 10)</code> |
| <code>revocation</code> | <code>string</code> | 在此声明中的实际使用：<code>Revocation: revocation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountDocumentCleanupContext</code> | 实际返回表达式：<code>c</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1250:1758 -->
````go
func NewAccountDocumentCleanupContext(r AccountDocumentCleanupRequest, epoch uint64, revocation string) AccountDocumentCleanupContext {
	c := AccountDocumentCleanupContext{AccountDocumentCleanupRequest: r, Scope: "doc-acl:" + r.Document, Epoch: strconv.FormatUint(epoch, 10), Revocation: revocation}
	digest := sha256.Sum256([]byte("ACCOUNT_DOCUMENT_CLEANUP\n" + c.Operation + "\n" + c.Document + "\n" + c.Scope + "\n" + c.Epoch + "\n" + c.Revocation))
	c.Digest = hex.EncodeToString(digest[:])
	return c
}

````

## 第 8 步：AccountDocumentCleanupContext.Valid

插入位置：第 7 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (c AccountDocumentCleanupContext) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>c == expected</code> |

**跟着本段代码走一遍：**

- 检查 <code>!c.AccountDocumentCleanupRequest.Valid() &#124;&#124; !canonicalCleanupID(c.Epoch) &#124;&#124; !accountClosureOperation.MatchString(c.Revocation)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return c == expected</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1758:2209 -->
````go
func (c AccountDocumentCleanupContext) Valid() bool {
	if !c.AccountDocumentCleanupRequest.Valid() || !canonicalCleanupID(c.Epoch) || !accountClosureOperation.MatchString(c.Revocation) {
		return false
	}
	epoch, _ := strconv.ParseUint(c.Epoch, 10, 64)
	expected := NewAccountDocumentCleanupContext(c.AccountDocumentCleanupRequest, epoch, c.Revocation)
	return c == expected
}

// account_notification.go：保留原实现，按业务合并文件。

````

## 第 9 步：AccountNotificationEvent

插入位置：第 8 步代码之后，紧接追加；完成稿约第 60 行。

Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。

本步声明：`type AccountNotificationEvent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | Account.notification主键，ACK按此原事件定位。<br>序列化/校验标记：<code>`json:&quot;id&quot;`</code> |
| <code>RecipientID</code> | <code>uint64</code> | 所有者转移成功后的目标用户ID。<br>序列化/校验标记：<code>`json:&quot;recipient_id&quot;`</code> |
| <code>OrganizationID</code> | <code>uint64</code> | 发生所有者转移的组织ID。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>EventID</code> | <code>string</code> | account-owner:加原操作SHA256的稳定去重身份。<br>序列化/校验标记：<code>`json:&quot;event_id&quot;`</code> |
| <code>Kind</code> | <code>string</code> | 当前受支持ORG_OWNER_TRANSFER，决定Doc投递语义。<br>序列化/校验标记：<code>`json:&quot;kind&quot;`</code> |

<!-- write-step: 9 bytes=2209:2546 -->
````go
// Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。
type AccountNotificationEvent struct {
	ID             uint64 `json:"id"`
	RecipientID    uint64 `json:"recipient_id"`
	OrganizationID uint64 `json:"organization_id"`
	EventID        string `json:"event_id"`
	Kind           string `json:"kind"`
}

````

## 第 10 步：AccountNotificationEventsResponse

插入位置：第 9 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`type AccountNotificationEventsResponse struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Events</code> | <code>&#91;&#93;AccountNotificationEvent</code> | 尚未确认的Account事件批次，Doc成功投递后再ACK。<br>序列化/校验标记：<code>`json:&quot;events&quot;`</code> |

<!-- write-step: 10 bytes=2546:2648 -->
````go
type AccountNotificationEventsResponse struct {
	Events []AccountNotificationEvent `json:"events"`
}

````

## 第 11 步：AccountNotificationAckRequest

插入位置：第 10 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`type AccountNotificationAckRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>IDs</code> | <code>&#91;&#93;uint64</code> | 原通知主键集合，长度1至100，必须非零且无重复。<br>序列化/校验标记：<code>`json:&quot;ids&quot;`</code> |

<!-- write-step: 11 bytes=2648:2722 -->
````go
type AccountNotificationAckRequest struct {
	IDs []uint64 `json:"ids"`
}

````

## 第 12 步：AccountNotificationAckResponse

插入位置：第 11 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`type AccountNotificationAckResponse struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Acknowledged</code> | <code>bool</code> | true表示这一批ACK已由Account提交，网络未知仍可重试同批。<br>序列化/校验标记：<code>`json:&quot;acknowledged&quot;`</code> |

<!-- write-step: 12 bytes=2722:2883 -->
````go
type AccountNotificationAckResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// draft_body_capacity.go：保留原实现，按业务合并文件。

````

## 第 13 步：DraftBodyProtection

插入位置：第 12 步代码之后，紧接追加；完成稿约第 83 行。

DraftBodyProtection 是内部实施扩展；原 protobuf 请求仍由 protojson 解码，
不复制 EDIT 授权契约，也不改动冻结的 proto 字节。计量只由 Node 隔离候选产生。

本步声明：`type DraftBodyProtection struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Protection</code> | <code>json.RawMessage</code> | 冻结protobuf保护请求的原protojson，保持原权限契约。<br>序列化/校验标记：<code>`json:&quot;protection&quot;`</code> |
| <code>BodyBytes</code> | <code>string</code> | Node隔离候选canonical正文的规范字节数，不由浏览器自行计量。<br>序列化/校验标记：<code>`json:&quot;body_bytes&quot;`</code> |
| <code>BodyHash</code> | <code>string</code> | 同一候选正文的SHA256，绑定预留与实际写入内容。<br>序列化/校验标记：<code>`json:&quot;body_hash&quot;`</code> |

<!-- write-step: 13 bytes=2883:3346 -->
````go
// DraftBodyProtection 是内部实施扩展；原 protobuf 请求仍由 protojson 解码，
// 不复制 EDIT 授权契约，也不改动冻结的 proto 字节。计量只由 Node 隔离候选产生。
type DraftBodyProtection struct {
	Protection json.RawMessage `json:"protection"`
	BodyBytes  string          `json:"body_bytes"`
	BodyHash   string          `json:"body_hash"`
}

// notification_administrators.go：保留原实现，按业务合并文件。

````

## 第 14 步：NotificationAdministratorsRequest

插入位置：第 13 步代码之后，紧接追加；完成稿约第 93 行。

管理员目录仅产生候选人；投递前必须重新调用接收资格核验，不能把目录当授权凭据。

本步声明：`type NotificationAdministratorsRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OrganizationID</code> | <code>uint64</code> | 要枚举当前管理候选的组织。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>After</code> | <code>uint64</code> | 上一页最后用户ID，首批0，使用严格大于游标查询。<br>序列化/校验标记：<code>`json:&quot;after&quot;`</code> |

<!-- write-step: 14 bytes=3346:3604 -->
````go
// 管理员目录仅产生候选人；投递前必须重新调用接收资格核验，不能把目录当授权凭据。
type NotificationAdministratorsRequest struct {
	OrganizationID uint64 `json:"organization_id"`
	After          uint64 `json:"after"`
}

````

## 第 15 步：NotificationAdministratorsResponse

插入位置：第 14 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`type NotificationAdministratorsResponse struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OrganizationID</code> | <code>uint64</code> | 回显请求组织，调用方核对批次绑定。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>UserIDs</code> | <code>&#91;&#93;uint64</code> | 最多100个当前ACTIVE owner/ADMIN候选，投递前仍需Check。<br>序列化/校验标记：<code>`json:&quot;user_ids&quot;`</code> |
| <code>NextAfter</code> | <code>uint64</code> | 还有下一页时的最后用户ID；0表示无下一页。<br>序列化/校验标记：<code>`json:&quot;next_after&quot;`</code> |

<!-- write-step: 15 bytes=3604:3869 -->
````go
type NotificationAdministratorsResponse struct {
	OrganizationID uint64   `json:"organization_id"`
	UserIDs        []uint64 `json:"user_ids"`
	NextAfter      uint64   `json:"next_after"`
}

// notification_recipient.go：保留原实现，按业务合并文件。

````

## 第 16 步：NotificationRecipientsRequest

插入位置：第 15 步代码之后，紧接追加；完成稿约第 107 行。

通知收件资格只在服务间传递，不是用户会话，也不能用于发起用户修改。

本步声明：`type NotificationRecipientsRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OrganizationID</code> | <code>uint64</code> | 非零时同时要求组织成员资格；0仅核账号资格。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>UserIDs</code> | <code>&#91;&#93;uint64</code> | 1至100个实际收件候选，Account按升序锁定并逐项返回。<br>序列化/校验标记：<code>`json:&quot;user_ids&quot;`</code> |

<!-- write-step: 16 bytes=3869:4112 -->
````go
// 通知收件资格只在服务间传递，不是用户会话，也不能用于发起用户修改。
type NotificationRecipientsRequest struct {
	OrganizationID uint64   `json:"organization_id"`
	UserIDs        []uint64 `json:"user_ids"`
}

````

## 第 17 步：NotificationRecipientSource

插入位置：第 16 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`type NotificationRecipientSource struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ScopeKey</code> | <code>string</code> | identity/credential/org-member/org-admin/org-owner具体撤权来源键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 该来源快照时的旧代次，Doc提交投递前必须锁定并核验。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 17 bytes=4112:4225 -->
````go
type NotificationRecipientSource struct {
	ScopeKey string `json:"scope_key"`
	Epoch    uint64 `json:"epoch"`
}

````

## 第 18 步：NotificationRecipient

插入位置：第 17 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`type NotificationRecipient struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>UserID</code> | <code>uint64</code> | 本条候选收件账号。<br>序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>Eligible</code> | <code>bool</code> | 当前锁内事实是否满足收件条件，false不产生授权来源。<br>序列化/校验标记：<code>`json:&quot;eligible&quot;`</code> |
| <code>IdentityEpoch</code> | <code>uint64</code> | 账号identity撤权代次。<br>序列化/校验标记：<code>`json:&quot;identity_epoch&quot;`</code> |
| <code>CredentialEpoch</code> | <code>uint64</code> | 账号凭据撤权代次，改密会关闭它。<br>序列化/校验标记：<code>`json:&quot;credential_epoch&quot;`</code> |
| <code>MemberEpoch</code> | <code>uint64</code> | 该组织成员关系代次。<br>序列化/校验标记：<code>`json:&quot;member_epoch&quot;`</code> |
| <code>Role</code> | <code>string</code> | 实际成员MEMBER或ADMIN，不能来自调用者自报。<br>序列化/校验标记：<code>`json:&quot;role&quot;`</code> |
| <code>OrgOwner</code> | <code>bool</code> | 是否等于组织owner_id。<br>序列化/校验标记：<code>`json:&quot;org_owner&quot;`</code> |
| <code>Sources</code> | <code>&#91;&#93;NotificationRecipientSource</code> | 全部实际资格依赖，Doc须在有效期内整体锁定。<br>序列化/校验标记：<code>`json:&quot;sources&quot;`</code> |

<!-- write-step: 18 bytes=4225:4797 -->
````go
type NotificationRecipient struct {
	UserID          uint64                        `json:"user_id"`
	Eligible        bool                          `json:"eligible"`
	IdentityEpoch   uint64                        `json:"identity_epoch"`
	CredentialEpoch uint64                        `json:"credential_epoch"`
	MemberEpoch     uint64                        `json:"member_epoch"`
	Role            string                        `json:"role"`
	OrgOwner        bool                          `json:"org_owner"`
	Sources         []NotificationRecipientSource `json:"sources"`
}

````

## 第 19 步：NotificationRecipientsResponse

插入位置：第 18 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`type NotificationRecipientsResponse struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OrganizationID</code> | <code>uint64</code> | 本批核验所属组织或0。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>Recipients</code> | <code>&#91;&#93;NotificationRecipient</code> | 逐候选返回的资格和来源快照。<br>序列化/校验标记：<code>`json:&quot;recipients&quot;`</code> |
| <code>ExpiresUnixMS</code> | <code>int64</code> | Account在释放事实锁前确定的5秒绝对期限，传输不得延长。<br>序列化/校验标记：<code>`json:&quot;expires_unix_ms&quot;`</code> |

<!-- write-step: 19 bytes=4797:5106 -->
````go
type NotificationRecipientsResponse struct {
	OrganizationID uint64                  `json:"organization_id"`
	Recipients     []NotificationRecipient `json:"recipients"`
	ExpiresUnixMS  int64                   `json:"expires_unix_ms"`
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 20 步：OrganizationClosureBinding

插入位置：第 19 步代码之后，紧接追加；完成稿约第 137 行。

OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。
Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。

本步声明：`type OrganizationClosureBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原独立组织关闭操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Organization</code> | <code>uint64</code> | 锁内确定的待关闭组织ID。<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>User</code> | <code>uint64</code> | 唯一存续成员且为owner的发起者。<br>序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>Revision</code> | <code>uint64</code> | 准备时组织资料/控制修订，防目标变化。<br>序列化/校验标记：<code>`json:&quot;revision&quot;`</code> |
| <code>MemberRevision</code> | <code>uint64</code> | 准备时成员集合修订，防加入/离开变化。<br>序列化/校验标记：<code>`json:&quot;member_revision&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 发起者原org-member代次，关闭组织不等于关闭账号身份。<br>序列化/校验标记：<code>`json:&quot;member_epoch&quot;`</code> |

<!-- write-step: 20 bytes=5106:5612 -->
````go
// OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。
// Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。
type OrganizationClosureBinding struct {
	Operation      string `json:"operation_id"`
	Organization   uint64 `json:"organization_id"`
	User           uint64 `json:"user_id"`
	Revision       uint64 `json:"revision"`
	MemberRevision uint64 `json:"member_revision"`
	Epoch          uint64 `json:"member_epoch"`
}

````

## 第 21 步：OrganizationClosureBinding.Valid

插入位置：第 20 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func (b OrganizationClosureBinding) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>accountClosureOperation.MatchString(b.Operation) &amp;&amp; b.Organization &gt; 0 &amp;&amp; b.User &gt; 0 &amp;&amp; b.Revision &gt; 0 &amp;&amp; b.Revision &lt; ^uint64(0) &amp;&amp; b.MemberRevision &gt; 0 &amp;&amp; b.MemberRevision &lt; ^uin …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return accountClosureOperation.MatchString(b.Operation) &amp;&amp; b.Organization &gt; 0 &amp;&amp; b.User &gt; 0 &amp;&amp; b.Revision &gt; 0 &amp;&amp; b.Revision &lt; ^uint64(0) &amp;&amp; b.MemberRevision &gt; 0 &amp;&amp; b.MemberRevision …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=5612:5900 -->
````go
func (b OrganizationClosureBinding) Valid() bool {
	return accountClosureOperation.MatchString(b.Operation) && b.Organization > 0 && b.User > 0 && b.Revision > 0 && b.Revision < ^uint64(0) && b.MemberRevision > 0 && b.MemberRevision < ^uint64(0) && b.Epoch > 0 && b.Epoch < ^uint64(0)
}

````

## 第 22 步：OrganizationClosureBinding.Scope

插入位置：第 21 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func (b OrganizationClosureBinding) Scope() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;org-member:&quot; + strconv.FormatUint(b.Organization, 10) + &quot;:&quot; + strconv.FormatUint(b.User, 10)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;org-member:&quot; + strconv.FormatUint(b.Organization, 10) + &quot;:&quot; + strconv.FormatUint(b.User, 10)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=5900:6058 -->
````go
func (b OrganizationClosureBinding) Scope() string {
	return "org-member:" + strconv.FormatUint(b.Organization, 10) + ":" + strconv.FormatUint(b.User, 10)
}

````

## 第 23 步：OrganizationClosureConfirmation

插入位置：第 22 步代码之后，紧接追加；完成稿约第 156 行。

本步声明：`type OrganizationClosureConfirmation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Confirmed</code> | <code>bool</code> | 同一组织来源屏障已经取得实际Doc ACK的确认。<br>序列化/校验标记：<code>`json:&quot;confirmed&quot;`</code> |

<!-- write-step: 23 bytes=6058:6141 -->
````go
type OrganizationClosureConfirmation struct {
	Confirmed bool `json:"confirmed"`
}
````

## 第 24 步：OrganizationClosureState

插入位置：第 23 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`type OrganizationClosureState struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 组织关闭控制步骤的持久状态，调用方须核期望值而不是只看HTTP200。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |

<!-- write-step: 24 bytes=6141:6211 -->
````go
type OrganizationClosureState struct {
	State string `json:"state"`
}
````

## 第 25 步：OrganizationClosurePageRequest

插入位置：第 24 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`type OrganizationClosurePageRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>OrganizationClosureBinding</code> | 固定组织关闭范围，必须来自Account。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>PlanDigest</code> | <code>string</code> | 不可变文档清理计划摘要，续页必须沿用。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |
| <code>AfterDocument</code> | <code>string</code> | 上一篇已确认清理完成的文档ID。<br>序列化/校验标记：<code>`json:&quot;after_document&quot;`</code> |

<!-- write-step: 25 bytes=6211:6446 -->
````go
type OrganizationClosurePageRequest struct {
	Binding       OrganizationClosureBinding `json:"binding"`
	PlanDigest    string                     `json:"plan_digest"`
	AfterDocument string                     `json:"after_document"`
}
````

## 第 26 步：OrganizationClosurePage

插入位置：第 25 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`type OrganizationClosurePage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>OrganizationClosureBinding</code> | 响应绑定的原组织关闭计划。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>State</code> | <code>string</code> | 仅CLEANING或SUCCEEDED可进入页面匹配。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>PlanDigest</code> | <code>string</code> | 固定清理集合SHA256。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |
| <code>Documents</code> | <code>&#91;&#93;AccountClosureDocument</code> | 递增且最多100个文档/正式版标识。<br>序列化/校验标记：<code>`json:&quot;documents&quot;`</code> |
| <code>NextAfter</code> | <code>string</code> | 精确等于本页最后文档或空页原游标。<br>序列化/校验标记：<code>`json:&quot;next_after&quot;`</code> |
| <code>Done</code> | <code>bool</code> | 是否最后一页；非末页必须满100条防截断。<br>序列化/校验标记：<code>`json:&quot;done&quot;`</code> |

<!-- write-step: 26 bytes=6446:6827 -->
````go
type OrganizationClosurePage struct {
	Binding    OrganizationClosureBinding `json:"binding"`
	State      string                     `json:"state"`
	PlanDigest string                     `json:"plan_digest"`
	Documents  []AccountClosureDocument   `json:"documents"`
	NextAfter  string                     `json:"next_after"`
	Done       bool                       `json:"done"`
}

````

## 第 27 步：OrganizationClosurePage.Matches

插入位置：第 26 步代码之后，紧接追加；完成稿约第 176 行。

固定摘要、严格递增文档号和已保存游标必须同时匹配；分页不接受截断集合。

本步声明：`func (p OrganizationClosurePage) Matches(r OrganizationClosurePageRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>r</code> | <code>OrganizationClosurePageRequest</code> | 在此声明中的实际使用：<code>r.Binding</code>；<code>r.PlanDigest</code>；<code>r.AfterDocument</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>p.NextAfter == next &amp;&amp; (p.Done &#124;&#124; len(p.Documents) == 100)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!r.Binding.Valid() &#124;&#124; p.Binding != r.Binding &#124;&#124; err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != p.PlanDigest &#124;&#124; (r.PlanDigest != &quot;&quot; &amp;&amp; r.PlanDigest != p.PlanDigest …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>r.AfterDocument != &quot;&quot;</code>。
- 检查 <code>!canonicalCleanupID(r.AfterDocument) &#124;&#124; r.PlanDigest == &quot;&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!canonicalCleanupID(item.Document)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>id &lt;= after &#124;&#124; (item.Release != &quot;0&quot; &amp;&amp; !canonicalCleanupID(item.Release))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return p.NextAfter == next &amp;&amp; (p.Done &#124;&#124; len(p.Documents) == 100)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=6827:8024 -->
````go
// 固定摘要、严格递增文档号和已保存游标必须同时匹配；分页不接受截断集合。
func (p OrganizationClosurePage) Matches(r OrganizationClosurePageRequest) bool {
	digest, err := hex.DecodeString(p.PlanDigest)
	if !r.Binding.Valid() || p.Binding != r.Binding || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != p.PlanDigest || (r.PlanDigest != "" && r.PlanDigest != p.PlanDigest) || p.Documents == nil || len(p.Documents) > 100 || (p.State != "CLEANING" && p.State != "SUCCEEDED") {
		return false
	}
	var after uint64
	if r.AfterDocument != "" {
		if !canonicalCleanupID(r.AfterDocument) || r.PlanDigest == "" {
			return false
		}
		after, _ = strconv.ParseUint(r.AfterDocument, 10, 64)
	}
	next := r.AfterDocument
	for _, item := range p.Documents {
		if !canonicalCleanupID(item.Document) {
			return false
		}
		id, _ := strconv.ParseUint(item.Document, 10, 64)
		if id <= after || (item.Release != "0" && !canonicalCleanupID(item.Release)) {
			return false
		}
		after, next = id, item.Document
	}
	return p.NextAfter == next && (p.Done || len(p.Documents) == 100)
}

// personal_transfer_storage.go：保留原实现，按业务合并文件。

````

## 第 28 步：PersonalTransferStorageClosure

插入位置：第 27 步代码之后，紧接追加；完成稿约第 205 行。

专用内部计量只跨mTLS传固定原号、关闭来源和字节事实；不会转移正文或发出写入权限。

本步声明：`type PersonalTransferStorageClosure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Revocation</code> | <code>string</code> | 转让中实际已登记的来源关闭号。<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 须已关闭的精确权限来源键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 该来源关闭的旧代次字符串。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 28 bytes=8024:8308 -->
````go
// 专用内部计量只跨mTLS传固定原号、关闭来源和字节事实；不会转移正文或发出写入权限。
type PersonalTransferStorageClosure struct {
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      string `json:"epoch"`
}

````

## 第 29 步：PersonalTransferStorageRequest

插入位置：第 28 步代码之后，紧接追加；完成稿约第 212 行。

本步声明：`type PersonalTransferStorageRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原个人文档转让操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Document</code> | <code>string</code> | 转让的精确文档ID字符串。<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Generation</code> | <code>string</code> | 原转让协议代次，防混用另一轮测量。<br>序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |
| <code>Closures</code> | <code>&#91;&#93;PersonalTransferStorageClosure</code> | Node测量前必须确认关闭的全部固定来源。<br>序列化/校验标记：<code>`json:&quot;closures&quot;`</code> |

<!-- write-step: 29 bytes=8308:8617 -->
````go
type PersonalTransferStorageRequest struct {
	Operation  string                           `json:"operation_id"`
	Document   string                           `json:"document_id"`
	Generation string                           `json:"generation"`
	Closures   []PersonalTransferStorageClosure `json:"closures"`
}

````

## 第 30 步：PersonalTransferStorageMeasurement

插入位置：第 29 步代码之后，紧接追加；完成稿约第 219 行。

本步声明：`type PersonalTransferStorageMeasurement struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 回显原转让操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Document</code> | <code>string</code> | 本次计量所属原文档。<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Generation</code> | <code>string</code> | 回显原协议代次。<br>序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |
| <code>Bytes</code> | <code>string</code> | Node固定工作区实际存储字节的规范十进制字符串。<br>序列化/校验标记：<code>`json:&quot;storage_bytes&quot;`</code> |
| <code>Digest</code> | <code>string</code> | 这份固定测量快照摘要，防重试换计量内容。<br>序列化/校验标记：<code>`json:&quot;snapshot_digest&quot;`</code> |
| <code>SourceClosed</code> | <code>bool</code> | 实际固定来源是否已停止，false不能据此推进转让。<br>序列化/校验标记：<code>`json:&quot;source_closed&quot;`</code> |

<!-- write-step: 30 bytes=8617:8928 -->
````go
type PersonalTransferStorageMeasurement struct {
	Operation    string `json:"operation_id"`
	Document     string `json:"document_id"`
	Generation   string `json:"generation"`
	Bytes        string `json:"storage_bytes"`
	Digest       string `json:"snapshot_digest"`
	SourceClosed bool   `json:"source_closed"`
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/authority_rpc_protocol.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/04-original-operation.md)。
