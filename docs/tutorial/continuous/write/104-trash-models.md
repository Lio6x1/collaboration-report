# 分步编写 `services/document/internal/domain/document/trash_models.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/104-trash-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义回收事实、恢复资格和单文档永久删除计划。

TRASHED仍有正文，恢复依据数据库时间及当前管理关系，旧直接授权不能复活文档。

写之前先按这条执行路线理解各段的连接：

1. CanManageTrash核当前责任来源
2. ValidateTrashRestore核父级和保留期
3. PlanTrashRestore越过metadata与gate双高水位
4. NewPurgePlan固定一篇Doc的两条来源。

本文件需要守住的边界：

- CLOSED父KB还需真实ACK
- 代次耗尽拒绝整笔事务，不能回绕。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/trash_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/trash_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/trash_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/trash_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:305 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"time"
)

// purge.go：保留原实现，按业务合并文件。

````

## 第 2 步：ValidatePurgeParent

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

普通已关闭KB中的TRASHED正文仍须按期清理；恢复另有ACTIVE限制。
CLOSED资格还必须由调用方核原BLOCKED关闭与真实ACK，不能单凭枚举发证。

本步声明：`func ValidatePurgeParent(kb dto.KnowledgeBase) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kb</code> | <code>dto.KnowledgeBase</code> | 在此声明中的实际使用：<code>kb.Lifecycle</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>kb.Lifecycle != &quot;ACTIVE&quot; &amp;&amp; kb.Lifecycle != &quot;CLOSED&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=305:681 -->
````go
// 普通已关闭KB中的TRASHED正文仍须按期清理；恢复另有ACTIVE限制。
// CLOSED资格还必须由调用方核原BLOCKED关闭与真实ACK，不能单凭枚举发证。
func ValidatePurgeParent(kb dto.KnowledgeBase) error {
	if kb.Lifecycle != "ACTIVE" && kb.Lifecycle != "CLOSED" {
		return Violation(StateConflict, "PURGE_PARENT_CHANGED")
	}
	return nil
}

````

## 第 3 步：PurgePlan

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用
Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。

本步声明：`type PurgePlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>uint8</code> | 固定计划schema版本，当前为1<br>序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 永久删除原号<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Actor</code> | <code>uint64</code> | 手动发起者，自动清理为0<br>序列化/校验标记：<code>`json:&quot;actor_id&quot;`</code> |
| <code>Automatic</code> | <code>bool</code> | 是否为服务端到期清理<br>序列化/校验标记：<code>`json:&quot;automatic&quot;`</code> |
| <code>Document</code> | <code>uint64</code> | 唯一文档ID<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Space</code> | <code>uint64</code> | 原父空间<br>序列化/校验标记：<code>`json:&quot;space_id&quot;`</code> |
| <code>KnowledgeBase</code> | <code>uint64</code> | 原父KB，不能扩大到整个KB<br>序列化/校验标记：<code>`json:&quot;knowledge_base_id&quot;`</code> |
| <code>ExpectedControl</code> | <code>uint64</code> | 登记前控制修订<br>序列化/校验标记：<code>`json:&quot;expected_control_revision&quot;`</code> |
| <code>ClosingControl</code> | <code>uint64</code> | 登记后关闭控制修订<br>序列化/校验标记：<code>`json:&quot;closing_control_revision&quot;`</code> |
| <code>ACL</code> | <code>uint64</code> | 计划关闭的Doc ACL代次<br>序列化/校验标记：<code>`json:&quot;acl_revision&quot;`</code> |
| <code>Publication</code> | <code>uint64</code> | 计划关闭的发布代次<br>序列化/校验标记：<code>`json:&quot;publication_epoch&quot;`</code> |
| <code>Sequence</code> | <code>uint64</code> | 撤下索引事件水位<br>序列化/校验标记：<code>`json:&quot;event_seq&quot;`</code> |
| <code>Scopes</code> | <code>&#91;&#93;PurgeScope</code> | 精确两条Doc ACL及publication关闭来源<br>序列化/校验标记：<code>`json:&quot;scopes&quot;`</code> |

<!-- write-step: 3 bytes=681:1601 -->
````go
// 普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用
// Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。
type PurgePlan struct {
	Version         uint8        `json:"version"`
	Operation       string       `json:"operation_id"`
	Actor           uint64       `json:"actor_id"`
	Automatic       bool         `json:"automatic"`
	Document        uint64       `json:"document_id"`
	Space           uint64       `json:"space_id"`
	KnowledgeBase   uint64       `json:"knowledge_base_id"`
	ExpectedControl uint64       `json:"expected_control_revision"`
	ClosingControl  uint64       `json:"closing_control_revision"`
	ACL             uint64       `json:"acl_revision"`
	Publication     uint64       `json:"publication_epoch"`
	Sequence        uint64       `json:"event_seq"`
	Scopes          []PurgeScope `json:"scopes"`
}

````

## 第 4 步：PurgeScope

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`type PurgeScope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 关闭来源键<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 计划要关闭的代次<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 由业务原号及scope稳定派生的子撤权号<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |

<!-- write-step: 4 bytes=1601:1740 -->
````go
type PurgeScope struct {
	Key       string `json:"scope_key"`
	Epoch     uint64 `json:"epoch"`
	Operation string `json:"revocation_id"`
}

````

## 第 5 步：purgeDerivedID

插入位置：第 4 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func purgeDerivedID(kind, input string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind + &quot;\n&quot;</code>；<code>kind + &quot;_&quot;</code> |
| <code>input</code> | <code>string</code> | 在此声明中的实际使用：<code>kind + &quot;\n&quot; + input</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>kind + &quot;_&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return kind + &quot;_&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1740:1897 -->
````go
func purgeDerivedID(kind, input string) string {
	digest := sha256.Sum256([]byte(kind + "\n" + input))
	return kind + "_" + hex.EncodeToString(digest[:])
}

````

## 第 6 步：PurgeRevocation

插入位置：第 5 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func PurgeRevocation(operation, scope string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation+&quot;\n&quot;</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>operation+&quot;\n&quot;+scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>purgeDerivedID(&quot;purge_fence&quot;, operation+&quot;\n&quot;+scope)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return purgeDerivedID(&quot;purge_fence&quot;, operation+&quot;\n&quot;+scope)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1897:2015 -->
````go
func PurgeRevocation(operation, scope string) string {
	return purgeDerivedID("purge_fence", operation+"\n"+scope)
}

````

## 第 7 步：PurgeExpiryOperation

插入位置：第 6 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func PurgeExpiryOperation(facts TrashFacts) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.PurgeAfter</code>；<code>facts.Document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>purgeDerivedID(&quot;purge_expiry&quot;, facts.Document.Id+&quot;\n&quot;+facts.Document.ControlRevision+&quot;\n&quot;+deadline)</code> |

**跟着本段代码走一遍：**

- 检查 <code>facts.PurgeAfter != nil</code>。
- 此处通过 <code>return purgeDerivedID(&quot;purge_expiry&quot;, facts.Document.Id+&quot;\n&quot;+facts.Document.ControlRevision+&quot;\n&quot;+deadline)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2015:2310 -->
````go
func PurgeExpiryOperation(facts TrashFacts) string {
	deadline := ""
	if facts.PurgeAfter != nil {
		deadline = facts.PurgeAfter.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return purgeDerivedID("purge_expiry", facts.Document.Id+"\n"+facts.Document.ControlRevision+"\n"+deadline)
}

````

## 第 8 步：ValidatePurgeIntent

插入位置：第 7 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func ValidatePurgeIntent(facts TrashFacts, confirmed bool, expected string, automatic bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Now</code>；<code>facts.PurgeAfter</code> |
| <code>confirmed</code> | <code>bool</code> | 在此声明中的实际使用：<code>!confirmed</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>facts.Document.ControlRevision != expected</code> |
| <code>automatic</code> | <code>bool</code> | 在此声明中的实际使用：<code>automatic &amp;&amp; (facts.Now.IsZero() &#124;&#124; facts.PurgeAfter == nil &#124;&#124; facts.PurgeAfter.After(facts.Now))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;PURGE_CONFIRMATION_REQUIRED&quot;)</code>；<code>Violation(StateConflict, &quot;DOCUMENT_NOT_TRASHED&quot;)</code>；<code>Violation(StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>；<code>Violation(StateConflict, &quot;PURGE_NOT_EXPIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!confirmed</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;PURGE_CONFIRMATION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.Document.Lifecycle != &quot;TRASHED&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;DOCUMENT_NOT_TRASHED&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.Document.ControlRevision != expected</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>automatic &amp;&amp; (facts.Now.IsZero() &#124;&#124; facts.PurgeAfter == nil &#124;&#124; facts.PurgeAfter.After(facts.Now))</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PURGE_NOT_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2310:2893 -->
````go
func ValidatePurgeIntent(facts TrashFacts, confirmed bool, expected string, automatic bool) error {
	if !confirmed {
		return Violation(InvalidRequest, "PURGE_CONFIRMATION_REQUIRED")
	}
	if facts.Document.Lifecycle != "TRASHED" {
		return Violation(StateConflict, "DOCUMENT_NOT_TRASHED")
	}
	if facts.Document.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if automatic && (facts.Now.IsZero() || facts.PurgeAfter == nil || facts.PurgeAfter.After(facts.Now)) {
		return Violation(StateConflict, "PURGE_NOT_EXPIRED")
	}
	return nil
}

````

## 第 9 步：NewPurgePlan

插入位置：第 8 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func NewPurgePlan(operation string, actor uint64, automatic bool, facts TrashFacts, kb, aclHighWater, publicationHighWater uint64) (PurgePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>Operation: operation</code>；<code>PurgeRevocation(operation, scope.key)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Actor: actor</code> |
| <code>automatic</code> | <code>bool</code> | 在此声明中的实际使用：<code>Automatic: automatic</code> |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>PlanTrashRestore(facts, aclHighWater, publicationHighWater)</code>；<code>facts.Document</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>KnowledgeBase: kb</code> |
| <code>aclHighWater</code> | <code>uint64</code> | 在此声明中的实际使用：<code>PlanTrashRestore(facts, aclHighWater, publicationHighWater)</code> |
| <code>publicationHighWater</code> | <code>uint64</code> | 在此声明中的实际使用：<code>PlanTrashRestore(facts, aclHighWater, publicationHighWater)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgePlan</code> | 使用 <code>PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>实际返回表达式：<code>p</code>；<code>PurgePlan{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>Violation(InvalidRequest, &quot;PURGE_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return p, err</code>。当前函数在这里结束。
- 检查 <code>!p.Valid()</code>。条件成立时执行 <code>return PurgePlan{}, Violation(InvalidRequest, &quot;PURGE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2893:4096 -->
````go
func NewPurgePlan(operation string, actor uint64, automatic bool, facts TrashFacts, kb, aclHighWater, publicationHighWater uint64) (PurgePlan, error) {
	var p PurgePlan
	next, err := PlanTrashRestore(facts, aclHighWater, publicationHighWater)
	if err != nil {
		return p, err
	}
	doc, err := CanonicalID(facts.Document.Id)
	if err != nil {
		return p, err
	}
	space, err := CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return p, err
	}
	control, err := CanonicalRevision(facts.Document.ControlRevision)
	if err != nil {
		return p, err
	}
	p = PurgePlan{Version: 1, Operation: operation, Actor: actor, Automatic: automatic,
		Document: doc, Space: space, KnowledgeBase: kb, ExpectedControl: control,
		ClosingControl: next.Control, ACL: next.ACL, Publication: next.Publication, Sequence: next.Sequence}
	for _, scope := range []struct {
		key   string
		epoch uint64
	}{{"doc-acl:" + TextID(doc), p.ACL}, {"publication:" + TextID(doc), p.Publication}} {
		p.Scopes = append(p.Scopes, PurgeScope{Key: scope.key, Epoch: scope.epoch, Operation: PurgeRevocation(operation, scope.key)})
	}
	if !p.Valid() {
		return PurgePlan{}, Violation(InvalidRequest, "PURGE_PLAN_INVALID")
	}
	return p, nil
}

````

## 第 10 步：PurgePlan.Valid

插入位置：第 9 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func (p PurgePlan) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>p.Version != 1 &#124;&#124; !ValidOperationID(p.Operation) &#124;&#124; p.Document == 0 &#124;&#124; p.Space == 0 &#124;&#124; 		(p.Automatic &amp;&amp; p.Actor != 0) &#124;&#124; (!p.Automatic &amp;&amp; p.Actor == 0) &#124;&#124; p.ClosingControl == 0 &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>i == 1</code>。
- 检查 <code>source.Key != key &#124;&#124; source.Epoch != epoch &#124;&#124; source.Operation != PurgeRevocation(p.Operation, key)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4096:4771 -->
````go
func (p PurgePlan) Valid() bool {
	if p.Version != 1 || !ValidOperationID(p.Operation) || p.Document == 0 || p.Space == 0 ||
		(p.Automatic && p.Actor != 0) || (!p.Automatic && p.Actor == 0) || p.ClosingControl == 0 ||
		p.ClosingControl != p.ExpectedControl+1 || p.ACL == 0 || p.Publication == 0 || p.Sequence == 0 || len(p.Scopes) != 2 {
		return false
	}
	for i, source := range p.Scopes {
		key, epoch := "doc-acl:"+TextID(p.Document), p.ACL
		if i == 1 {
			key, epoch = "publication:"+TextID(p.Document), p.Publication
		}
		if source.Key != key || source.Epoch != epoch || source.Operation != PurgeRevocation(p.Operation, key) {
			return false
		}
	}
	return true
}

````

## 第 11 步：DecodePurgePlan

插入位置：第 10 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func DecodePurgePlan(raw []byte) (PurgePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code>；<code>len(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PurgePlan</code> | 使用 <code>PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>实际返回表达式：<code>PurgePlan{}</code>；<code>p</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;PURGE_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 8192 &#124;&#124; d.Decode(&amp;p) != nil &#124;&#124; d.Decode(new(any)) != io.EOF &#124;&#124; !p.Valid()</code>。条件成立时执行 <code>return PurgePlan{}, Violation(TemporarilyUnavailable, &quot;PURGE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4771:5161 -->
````go
func DecodePurgePlan(raw []byte) (PurgePlan, error) {
	var p PurgePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 8192 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || !p.Valid() {
		return PurgePlan{}, Violation(TemporarilyUnavailable, "PURGE_PLAN_INVALID")
	}
	return p, nil
}

// trash.go：保留原实现，按业务合并文件。

````

## 第 12 步：TrashFacts

插入位置：第 11 步代码之后，紧接追加；完成稿约第 146 行。

TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。
恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。

本步声明：`type TrashFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>dto.Document</code> | 含TRASHED等实际状态的文档事实 |
| <code>Sequence</code> | <code>uint64</code> | 当前索引事件水位 |
| <code>PurgeAfter</code> | <code>*time.Time</code> | 数据库保存的永久清理截止时间，可空表示缺证据 |
| <code>Now</code> | <code>time.Time</code> | 本次SQL读取的数据库当前时间，不使用进程时钟猜保留期 |

<!-- write-step: 12 bytes=5161:5469 -->
````go
// TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。
// 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。
type TrashFacts struct {
	Document   dto.Document
	Sequence   uint64
	PurgeAfter *time.Time
	Now        time.Time
}

````

## 第 13 步：TrashRestoreRevisions

插入位置：第 12 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`type TrashRestoreRevisions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Control</code> | <code>uint64</code> | 恢复后控制修订 |
| <code>ACL</code> | <code>uint64</code> | 越过旧gate高水位的新ACL代次 |
| <code>Publication</code> | <code>uint64</code> | 越过旧发布关闭水位的新代次 |
| <code>Visibility</code> | <code>uint64</code> | 恢复后的可见政策修订 |
| <code>Sequence</code> | <code>uint64</code> | 恢复后索引事件水位 |

<!-- write-step: 13 bytes=5469:5564 -->
````go
type TrashRestoreRevisions struct {
	Control, ACL, Publication, Visibility, Sequence uint64
}

````

## 第 14 步：TrashRestoreReceipt

插入位置：第 13 步代码之后，紧接追加；完成稿约第 159 行。

预读只决定使用当前来源准备原号回放；不包含另一份fingerprint/结果解析，
真正重放必须交给标准Mutation持原operation gate核验并读取固定receipt。

本步声明：`type TrashRestoreReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 预读到的原恢复号 |
| <code>Kind</code> | <code>string</code> | 必须为DOCUMENT_RESTORE |
| <code>State</code> | <code>string</code> | 原回执状态，只有SUCCEEDED可按此路径重放 |
| <code>Path</code> | <code>string</code> | 原恢复URL，需精确绑定文档 |
| <code>Actor</code> | <code>uint64</code> | 原恢复发起人 |

<!-- write-step: 14 bytes=5564:5871 -->
````go
// 预读只决定使用当前来源准备原号回放；不包含另一份fingerprint/结果解析，
// 真正重放必须交给标准Mutation持原operation gate核验并读取固定receipt。
type TrashRestoreReceipt struct {
	Operation, Kind, State, Path string
	Actor                        uint64
}

````

## 第 15 步：ValidateTrashRestoreReceipt

插入位置：第 14 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func ValidateTrashRestoreReceipt(receipt TrashRestoreReceipt, operation string, actor, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>receipt</code> | <code>TrashRestoreReceipt</code> | 使用 <code>TrashRestoreReceipt</code> 类型；其定义处的职责：预读只决定使用当前来源准备原号回放；不包含另一份fingerprint/结果解析， 真正重放必须交给标准Mutation持原operation gate核验并读取固定receipt。<br>在此声明中的实际使用：<code>receipt.Operation</code>；<code>receipt.Actor</code>；<code>receipt.Kind</code>；<code>receipt.Path</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>receipt.Operation != operation</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>receipt.Actor != actor</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>Violation(TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>receipt.Operation != operation &#124;&#124; receipt.Actor != actor &#124;&#124; receipt.Kind != &quot;DOCUMENT_RESTORE&quot; &#124;&#124; 		receipt.Path != &quot;/v1/documents/&quot;+TextID(document)+&quot;/restore&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return Violation(TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5871:6342 -->
````go
func ValidateTrashRestoreReceipt(receipt TrashRestoreReceipt, operation string, actor, document uint64) error {
	if receipt.Operation != operation || receipt.Actor != actor || receipt.Kind != "DOCUMENT_RESTORE" ||
		receipt.Path != "/v1/documents/"+TextID(document)+"/restore" {
		return Violation(StateConflict, "OPERATION_ID_CONFLICT")
	}
	if receipt.State != "SUCCEEDED" {
		return Violation(TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}
	return nil
}

````

## 第 16 步：CanManageTrash

插入位置：第 15 步代码之后，紧接追加；完成稿约第 177 行。

回收资格来自现今仍有效的责任关系/上级管理来源，旧文档直接授权不用于复活。
Responsible由调用方在真实组织成员proof下核验当前Doc负责关系；
KnowledgeBaseAdmin还要求当前ACTIVE库，不能把离组或已关闭角色当作管理资格。

本步声明：`func CanManageTrash(facts PermissionFacts) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>PermissionFacts</code> | 使用 <code>PermissionFacts</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>facts.Responsible</code>；<code>facts.PersonalOwner</code>；<code>facts.OrganizationAdmin</code>；<code>facts.KnowledgeBaseAdmin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>facts.Responsible &#124;&#124; facts.PersonalOwner &#124;&#124; facts.OrganizationAdmin &#124;&#124; facts.KnowledgeBaseAdmin</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return facts.Responsible &#124;&#124; facts.PersonalOwner &#124;&#124; facts.OrganizationAdmin &#124;&#124; facts.KnowledgeBaseAdmin</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6342:6801 -->
````go
// 回收资格来自现今仍有效的责任关系/上级管理来源，旧文档直接授权不用于复活。
// Responsible由调用方在真实组织成员proof下核验当前Doc负责关系；
// KnowledgeBaseAdmin还要求当前ACTIVE库，不能把离组或已关闭角色当作管理资格。
func CanManageTrash(facts PermissionFacts) bool {
	return facts.Responsible || facts.PersonalOwner || facts.OrganizationAdmin || facts.KnowledgeBaseAdmin
}

````

## 第 17 步：ValidateTrashRestore

插入位置：第 16 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func ValidateTrashRestore(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, expected string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Now</code>；<code>facts.PurgeAfter</code> |
| <code>space</code> | <code>dto.Space</code> | 在此声明中的实际使用：<code>space.Id</code>；<code>space.Lifecycle</code> |
| <code>kb</code> | <code>*dto.KnowledgeBase</code> | 在此声明中的实际使用：<code>kb == nil</code>；<code>kb.Id</code>；<code>kb.SpaceId</code>；<code>kb.Lifecycle</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>d.ControlRevision != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;DOCUMENT_NOT_TRASHED&quot;)</code>；<code>Violation(StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>；<code>Violation(TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>；<code>Violation(StateConflict, &quot;KB_NOT_ACTIVE&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>d.Lifecycle != &quot;TRASHED&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;DOCUMENT_NOT_TRASHED&quot;)</code>。当前函数在这里结束。
- 检查 <code>space.Id != d.SpaceId &#124;&#124; space.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.KnowledgeBaseId != nil</code>。
- 检查 <code>kb == nil &#124;&#124; kb.Id != *d.KnowledgeBaseId &#124;&#124; kb.SpaceId != d.SpaceId</code>。条件成立时执行 <code>return Violation(TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>kb.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;KB_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.ControlRevision != expected</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.Now.IsZero() &#124;&#124; facts.PurgeAfter == nil</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;TRASH_RETENTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!facts.PurgeAfter.After(facts.Now)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;TRASH_RETENTION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Violation(TemporarilyUnavailable, &quot;TRASH_CONTENT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6801:8077 -->
````go
func ValidateTrashRestore(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, expected string) error {
	d := facts.Document
	if d.Lifecycle != "TRASHED" {
		return Violation(StateConflict, "DOCUMENT_NOT_TRASHED")
	}
	if space.Id != d.SpaceId || space.Lifecycle != "ACTIVE" {
		return Violation(StateConflict, "SPACE_NOT_ACTIVE")
	}
	if d.KnowledgeBaseId != nil {
		if kb == nil || kb.Id != *d.KnowledgeBaseId || kb.SpaceId != d.SpaceId {
			return Violation(TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
		}
		// 冻结接口没有迁移目标，不能把关闭库中的文档私自移入其他库或空间。
		if kb.Lifecycle != "ACTIVE" {
			return Violation(StateConflict, "KB_NOT_ACTIVE")
		}
	}
	if d.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if facts.Now.IsZero() || facts.PurgeAfter == nil {
		return Violation(StateConflict, "TRASH_RETENTION_UNKNOWN")
	}
	if !facts.PurgeAfter.After(facts.Now) {
		return Violation(StateConflict, "TRASH_RETENTION_EXPIRED")
	}
	for _, id := range []string{d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId} {
		if _, err := CanonicalID(id); err != nil {
			return Violation(TemporarilyUnavailable, "TRASH_CONTENT_BINDING_UNKNOWN")
		}
	}
	return nil
}

````

## 第 18 步：PlanTrashRestore

插入位置：第 17 步代码之后，紧接追加；完成稿约第 218 行。

关闭高水位可能已超出旧metadata，恢复必须同时越过两者。耗尽拒绝整笔事务，
不通过回绕或降低gate.max_epoch让迟到的旧输出/协作请求复活。

本步声明：`func PlanTrashRestore(facts TrashFacts, aclHighWater, publicationHighWater uint64) (TrashRestoreRevisions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Sequence</code> |
| <code>aclHighWater</code> | <code>uint64</code> | 在此声明中的实际使用：<code>max(current&#91;1&#93;, aclHighWater)</code> |
| <code>publicationHighWater</code> | <code>uint64</code> | 在此声明中的实际使用：<code>max(current&#91;2&#93;, publicationHighWater)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>TrashRestoreRevisions</code> | 实际返回表达式：<code>next</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;TRASH_REVISION_INVALID&quot;)</code>；<code>Violation(StateConflict, &quot;TRASH_REVISION_EXHAUSTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return next, Violation(TemporarilyUnavailable, &quot;TRASH_REVISION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value == math.MaxUint64</code>。条件成立时执行 <code>return next, Violation(StateConflict, &quot;TRASH_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return next, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8077:9266 -->
````go
// 关闭高水位可能已超出旧metadata，恢复必须同时越过两者。耗尽拒绝整笔事务，
// 不通过回绕或降低gate.max_epoch让迟到的旧输出/协作请求复活。
func PlanTrashRestore(facts TrashFacts, aclHighWater, publicationHighWater uint64) (TrashRestoreRevisions, error) {
	var next TrashRestoreRevisions
	values := []string{facts.Document.ControlRevision, facts.Document.AclRevision, facts.Document.PublicationEpoch, facts.Document.VisibilityRevision}
	current := make([]uint64, len(values))
	for i, text := range values {
		value, err := CanonicalID(text)
		if err != nil {
			return next, Violation(TemporarilyUnavailable, "TRASH_REVISION_INVALID")
		}
		current[i] = value
	}
	current[1] = max(current[1], aclHighWater)
	current[2] = max(current[2], publicationHighWater)
	for _, value := range append(current, facts.Sequence) {
		if value == math.MaxUint64 {
			return next, Violation(StateConflict, "TRASH_REVISION_EXHAUSTED")
		}
	}
	next = TrashRestoreRevisions{
		Control:     current[0] + 1,
		ACL:         current[1] + 1,
		Publication: current[2] + 1,
		Visibility:  current[3] + 1,
		Sequence:    facts.Sequence + 1,
	}
	return next, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/trash_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
