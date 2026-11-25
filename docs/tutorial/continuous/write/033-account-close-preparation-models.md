# 分步编写 `services/account/internal/domain/account/account_close_preparation_models.go`

先按 [A06 · 账号注销准备、职责交接与来源冻结](../lessons/account/06-account-close-preparation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/033-account-close-preparation-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。

注销跨多次任务推进，必须保存当时的资格与范围，不能每次重试用当前组织图重算。

写之前先按这条执行路线理解各段的连接：

1. AccountCloseCapture在下游范围固定前Binding为nil；Organizations同时保存集合与本人行修订。
2. DecodeAccountCloseCapture检查会话摘要、正代次、绑定一致和计划摘要；DecodeAccountCloseFence检查原操作/子撤销/scope。

本文件需要守住的边界：

- 空组织集合不能冒充已完成规划；AfterDocument只有在固定摘要存在且前页清理确认后可推进。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/account_close_preparation_models.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/account_close_preparation_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/account_close_preparation_models.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/account_close_preparation_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:287 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	"collaboration/platform"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

// account_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountCloseCapture

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。
Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。

本步声明：`type AccountCloseCapture struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SessionHash</code> | <code>string</code> | 会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。<br>序列化/校验标记：<code>`json:&quot;session_hash&quot;`</code> |
| <code>CredentialEpoch</code> | <code>uint64</code> | 凭据代次；改密码等凭据变化会推进它，旧会话与当前代次不一致时不能继续使用。<br>序列化/校验标记：<code>`json:&quot;credential_epoch&quot;`</code> |
| <code>IdentityEpoch</code> | <code>uint64</code> | 身份状态代次；准入与恢复时比较这份身份事实是否仍对应当前账号状态。<br>序列化/校验标记：<code>`json:&quot;identity_epoch&quot;`</code> |
| <code>Binding</code> | <code>*platform.AccountClosureBinding</code> | 下游已固定的账号关闭范围；规划完成前必须为nil，空集合不能冒充规划完成。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Organizations</code> | <code>&#91;&#93;AccountCloseOrganization</code> | 开始关闭时固定的组织与成员事实，后续需核对是否发生变化。<br>序列化/校验标记：<code>`json:&quot;organizations,omitempty&quot;`</code> |
| <code>Scopes</code> | <code>&#91;&#93;AccountCloseScope</code> | 固定的来源关闭范围与代次，跨服务围栏沿此清单推进。<br>序列化/校验标记：<code>`json:&quot;scopes,omitempty&quot;`</code> |
| <code>Retention</code> | <code>*AccountCloseRetention</code> | Doc保留/清理计划及确认后的分页进度，不存正文或密码。<br>序列化/校验标记：<code>`json:&quot;retention,omitempty&quot;`</code> |

<!-- write-step: 2 bytes=287:1074 -->
````go
// AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。
// Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。
type AccountCloseCapture struct {
	SessionHash     string                          `json:"session_hash"`
	CredentialEpoch uint64                          `json:"credential_epoch"`
	IdentityEpoch   uint64                          `json:"identity_epoch"`
	Binding         *platform.AccountClosureBinding `json:"binding"`
	Organizations   []AccountCloseOrganization      `json:"organizations,omitempty"`
	Scopes          []AccountCloseScope             `json:"scopes,omitempty"`
	Retention       *AccountCloseRetention          `json:"retention,omitempty"`
}

````

## 第 3 步：AccountCloseRetention

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

文档游标只在原PAYLOADS_REMOVED确认后推进；摘要固定Doc原计划，不存正文。

本步声明：`type AccountCloseRetention struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>PlanDigest</code> | <code>string</code> | 预览/执行计划的内容摘要，确认时据此绑定原计划；不能拿新计划冒充用户此前确认的内容。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |
| <code>AfterDocument</code> | <code>string</code> | 只有Doc确认原文档PAYLOADS_REMOVED后才能推进的文档游标。<br>序列化/校验标记：<code>`json:&quot;after_document&quot;`</code> |
| <code>DocCompleted</code> | <code>bool</code> | Doc原固定计划已全部处理的确认，不是仅发出清理请求。<br>序列化/校验标记：<code>`json:&quot;doc_completed&quot;`</code> |

<!-- write-step: 3 bytes=1074:1348 -->
````go
// 文档游标只在原PAYLOADS_REMOVED确认后推进；摘要固定Doc原计划，不存正文。
type AccountCloseRetention struct {
	PlanDigest    string `json:"plan_digest"`
	AfterDocument string `json:"after_document"`
	DocCompleted  bool   `json:"doc_completed"`
}

````

## 第 4 步：AccountCloseOrganization

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位，
两者都固定才能在关闭前发现其他成员加入与本人角色变化。

本步声明：`type AccountCloseOrganization struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 关闭规划时涉及的组织编号。<br>序列化/校验标记：<code>`json:&quot;id&quot;`</code> |
| <code>Owner</code> | <code>uint64</code> | 当时的组织所有者账号编号，用于发现所有者职责。<br>序列化/校验标记：<code>`json:&quot;owner&quot;`</code> |
| <code>Revision</code> | <code>uint64</code> | 组织对象修订号，关闭前要识别组织控制变化。<br>序列化/校验标记：<code>`json:&quot;revision&quot;`</code> |
| <code>MemberRevision</code> | <code>uint64</code> | 整个组织成员集合水位，能发现其他人加入。<br>序列化/校验标记：<code>`json:&quot;member_revision&quot;`</code> |
| <code>MembershipRevision</code> | <code>uint64</code> | 当前账号那一条成员关系修订号，能发现本人角色变化。<br>序列化/校验标记：<code>`json:&quot;membership_revision&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 本人组织成员来源代次，撤回围栏针对这一来源。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |
| <code>Role</code> | <code>string</code> | 本人当前组织角色。<br>序列化/校验标记：<code>`json:&quot;role&quot;`</code> |
| <code>MemberState</code> | <code>string</code> | 本人成员关系状态，撤权中也不能当成不存在。<br>序列化/校验标记：<code>`json:&quot;member_state&quot;`</code> |
| <code>Lifecycle</code> | <code>string</code> | 组织当前ACTIVE/CLOSING/CLOSED等存续状态。<br>序列化/校验标记：<code>`json:&quot;lifecycle&quot;`</code> |
| <code>LiveMembers</code> | <code>int</code> | 包含撤权中成员的存续成员数，判断能否关闭组织。<br>序列化/校验标记：<code>`json:&quot;live_members&quot;`</code> |
| <code>AvailableAdministrators</code> | <code>int</code> | 可接续管理职责的管理员数量，注销不能把组织留成无人管理。<br>序列化/校验标记：<code>`json:&quot;available_administrators&quot;`</code> |

<!-- write-step: 4 bytes=1348:2148 -->
````go
// MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位，
// 两者都固定才能在关闭前发现其他成员加入与本人角色变化。
type AccountCloseOrganization struct {
	ID                      uint64 `json:"id"`
	Owner                   uint64 `json:"owner"`
	Revision                uint64 `json:"revision"`
	MemberRevision          uint64 `json:"member_revision"`
	MembershipRevision      uint64 `json:"membership_revision"`
	Epoch                   uint64 `json:"epoch"`
	Role                    string `json:"role"`
	MemberState             string `json:"member_state"`
	Lifecycle               string `json:"lifecycle"`
	LiveMembers             int    `json:"live_members"`
	AvailableAdministrators int    `json:"available_administrators"`
}

````

## 第 5 步：AccountCloseScope

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`type AccountCloseScope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Revocation</code> | <code>string</code> | 该来源关闭流程的固定撤权号。<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 账号/成员来源的规范范围键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 此次关闭固定的来源代次，重试不得换成后来代次。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 5 bytes=2148:2297 -->
````go
type AccountCloseScope struct {
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      uint64 `json:"epoch"`
}

````

## 第 6 步：AccountCloseRecord

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`type AccountCloseRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>User</code> | <code>uint64</code> | 正在注销的账号。 |
| <code>State</code> | <code>string</code> | account_close持久处理阶段。 |
| <code>Capture</code> | <code>&#91;&#93;byte</code> | 原关闭capture的完整JSON字节，恢复时严格解码。 |

<!-- write-step: 6 bytes=2297:2381 -->
````go
type AccountCloseRecord struct {
	User    uint64
	State   string
	Capture []byte
}

````

## 第 7 步：AccountCloseReceipt

插入位置：第 6 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`type AccountCloseReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>User</code> | <code>uint64</code> | 原关闭操作绑定的账号。 |
| <code>Kind</code> | <code>string</code> | 原操作种类，避免将其他操作回执用于注销。 |
| <code>State</code> | <code>string</code> | 原关闭回执处理阶段。 |
| <code>SessionHash</code> | <code>string</code> | 会话令牌的摘要表示，用于核对会话；不能把可直接登录的原文令牌写入这类身份字段。 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 已经固定的关闭结果字节，重试重放原结果。 |

<!-- write-step: 7 bytes=2381:2517 -->
````go
type AccountCloseReceipt struct {
	User                     uint64
	Kind, State, SessionHash string
	Fingerprint, Result      []byte
}

````

## 第 8 步：DecodeAccountCloseCapture

插入位置：第 7 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func DecodeAccountCloseCapture(raw []byte) (AccountCloseCapture, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountCloseCapture</code> | 使用 <code>AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>实际返回表达式：<code>capture</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;capture) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return capture, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != capture.SessionHash &#124;&#124; capture.CredentialEpoch == 0 &#124;&#124; capture.IdentityEpoch == 0</code>。条件成立时执行 <code>return capture, invalid</code>。当前函数在这里结束。
- 检查 <code>capture.Binding != nil &amp;&amp; (!platform.ValidateAccountClosureBinding(*capture.Binding) &#124;&#124; capture.Binding.IdentityEpoch != capture.IdentityEpoch)</code>。条件成立时执行 <code>return capture, invalid</code>。当前函数在这里结束。
- 检查 <code>capture.Retention != nil</code>。
- 检查 <code>capture.Binding == nil &#124;&#124; err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != capture.Retention.PlanDigest</code>。条件成立时执行 <code>return capture, invalid</code>。当前函数在这里结束。
- 检查 <code>capture.Retention.AfterDocument != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; id == 0 &#124;&#124; strconv.FormatUint(id, 10) != capture.Retention.AfterDocument</code>。条件成立时执行 <code>return capture, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return capture, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2517:3885 -->
````go
func DecodeAccountCloseCapture(raw []byte) (AccountCloseCapture, error) {
	var capture AccountCloseCapture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	invalid := errors.New("ACCOUNT_CLOSE_CAPTURE_INVALID")
	if decoder.Decode(&capture) != nil || decoder.Decode(new(any)) != io.EOF {
		return capture, invalid
	}
	digest, err := hex.DecodeString(capture.SessionHash)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != capture.SessionHash || capture.CredentialEpoch == 0 || capture.IdentityEpoch == 0 {
		return capture, invalid
	}
	if capture.Binding != nil && (!platform.ValidateAccountClosureBinding(*capture.Binding) || capture.Binding.IdentityEpoch != capture.IdentityEpoch) {
		return capture, invalid
	}
	if capture.Retention != nil {
		digest, err := hex.DecodeString(capture.Retention.PlanDigest)
		if capture.Binding == nil || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != capture.Retention.PlanDigest {
			return capture, invalid
		}
		if capture.Retention.AfterDocument != "" {
			id, err := strconv.ParseUint(capture.Retention.AfterDocument, 10, 64)
			if err != nil || id == 0 || strconv.FormatUint(id, 10) != capture.Retention.AfterDocument {
				return capture, invalid
			}
		}
	}
	return capture, nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

````

## 第 9 步：AccountCloseFenceKind

插入位置：第 8 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`const AccountCloseFenceKind = "ACCOUNT_CLOSE_FENCE"`。

<!-- write-step: 9 bytes=3885:3938 -->
````go
const AccountCloseFenceKind = "ACCOUNT_CLOSE_FENCE"

````

## 第 10 步：AccountCloseFencePayload

插入位置：第 9 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`type AccountCloseFencePayload struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 所属账号注销原号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>User</code> | <code>uint64</code> | 被关闭来源的账号编号。<br>序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>Revocation</code> | <code>string</code> | 原撤权流程号。<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 必须符合AccountScope约定的原来源范围。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 此来源需要完成关闭的固定代次。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 10 bytes=3938:4171 -->
````go
type AccountCloseFencePayload struct {
	Operation  string `json:"operation_id"`
	User       uint64 `json:"user_id"`
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      uint64 `json:"epoch"`
}

````

## 第 11 步：AccountCloseFenceTask

插入位置：第 10 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`type AccountCloseFenceTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 持久关闭任务主键。 |
| <code>Generation</code> | <code>uint64</code> | 任务执行代次，防止旧执行者落定新任务。 |
| <code>Operation</code> | <code>string</code> | 原账号关闭操作号。 |
| <code>Revocation</code> | <code>string</code> | 待确认的原撤权号。 |
| <code>State</code> | <code>string</code> | 持久任务阶段，不凭内存状态认定完成。 |
| <code>Instance</code> | <code>string</code> | 当前持有任务的实例标识。 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 完整AccountCloseFencePayload字节。 |
| <code>LeaseValid</code> | <code>bool</code> | 当前租约是否仍有效，需与代次和实例一起检查。 |

<!-- write-step: 11 bytes=4171:4396 -->
````go
type AccountCloseFenceTask struct {
	ID, Generation                         uint64
	Operation, Revocation, State, Instance string
	Payload                                []byte
	LeaseValid                             bool
}

````

## 第 12 步：DecodeAccountCloseFence

插入位置：第 11 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func DecodeAccountCloseFence(raw []byte) (AccountCloseFencePayload, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountCloseFencePayload</code> | 实际返回表达式：<code>payload</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSE_FENCE_PAYLOAD_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;payload) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; !ValidOperationID(payload.Operation) &#124;&#124; !ValidOperationID(payload.Revocation) &#124;&#124; payload.User == 0 &#124;&#124; paylo …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return payload, errors.New(&quot;ACCOUNT_CLOSE_FENCE_PAYLOAD_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return payload, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4396:4934 -->
````go
func DecodeAccountCloseFence(raw []byte) (AccountCloseFencePayload, error) {
	var payload AccountCloseFencePayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || !ValidOperationID(payload.Operation) || !ValidOperationID(payload.Revocation) || payload.User == 0 || payload.Epoch == 0 || !platform.ValidAccountScope(payload.Scope) {
		return payload, errors.New("ACCOUNT_CLOSE_FENCE_PAYLOAD_INVALID")
	}
	return payload, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/account_close_preparation_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/06-account-close-preparation.md)。
