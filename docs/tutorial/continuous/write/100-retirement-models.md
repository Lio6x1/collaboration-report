# 分步编写 `services/document/internal/domain/document/retirement_models.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/100-retirement-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。

关闭范围与计费事实接受后不能因重新入组或目录变化重选。

写之前先按这条执行路线理解各段的连接：

1. DepartureBinding.Scope绑定组织成员
2. DecodeDepartureNotification核升序集合和游标
3. PersonalTransferPlan.Valid核原范围
4. TransferCapacity检查溢出。

本文件需要守住的边界：

- 组织计划不能混入个人空间或身份墓碑
- 治理计划不能恢复可写空间或选择新正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/retirement_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/retirement_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/retirement_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/retirement_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:358 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"time"
)

// departure.go：保留原实现，按业务合并文件。

````

## 第 2 步：DepartureBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

DepartureBinding绑定实际组织、成员、操作编号与原身份代次。

本步声明：`type DepartureBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Org</code> | <code>uint64</code> | 离组组织ID |
| <code>User</code> | <code>uint64</code> | 离组成员ID |
| <code>ID</code> | <code>string</code> | Account原离组操作号 |
| <code>Epoch</code> | <code>uint64</code> | 原成员身份代次 |

<!-- write-step: 2 bytes=358:529 -->
````go
// DepartureBinding绑定实际组织、成员、操作编号与原身份代次。
type DepartureBinding struct {
	Org, User uint64
	ID        string
	Epoch     uint64
}

````

## 第 3 步：DepartureBinding.Scope

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (b DepartureBinding) Scope() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;org-member:&quot; + TextID(b.Org) + &quot;:&quot; + TextID(b.User)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;org-member:&quot; + TextID(b.Org) + &quot;:&quot; + TextID(b.User)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=529:635 -->
````go
func (b DepartureBinding) Scope() string { return "org-member:" + TextID(b.Org) + ":" + TextID(b.User) }

````

## 第 4 步：DepartureGate

插入位置：第 3 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type DepartureGate struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Epoch</code> | <code>uint64</code> | 本地已观察成员来源高水位 |
| <code>State</code> | <code>string</code> | OPEN或BLOCKED |
| <code>Operation</code> | <code>string</code> | 最后关闭该来源的原操作号 |

<!-- write-step: 4 bytes=635:715 -->
````go
type DepartureGate struct {
	Epoch            uint64
	State, Operation string
}
````

## 第 5 步：DepartureObservation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type DepartureObservation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scope</code> | <code>string</code> | 待恢复成员来源 |
| <code>Operation</code> | <code>string</code> | 原离组操作号 |
| <code>Epoch</code> | <code>uint64</code> | 观察到的关闭代次，回滚证据必须一致 |

<!-- write-step: 5 bytes=715:878 -->
````go
type DepartureObservation struct {
	Scope, Operation string
	Epoch            uint64
}

// departure_notification.go：保留原实现，按业务合并文件。

````

## 第 6 步：DepartureNotificationKind

插入位置：第 5 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`const DepartureNotificationKind = "RESPONSIBLE_DEPARTURE_NOTIFICATION"`。

<!-- write-step: 6 bytes=878:950 -->
````go
const DepartureNotificationKind = "RESPONSIBLE_DEPARTURE_NOTIFICATION"

````

## 第 7 步：DepartureNotification

插入位置：第 6 步代码之后，紧接追加；完成稿约第 42 行。

DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、
成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。

本步声明：`type DepartureNotification struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 负责人离开的文档<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>KnowledgeBase</code> | <code>uint64</code> | 原知识库<br>序列化/校验标记：<code>`json:&quot;knowledge_base_id&quot;`</code> |
| <code>Organization</code> | <code>uint64</code> | 原组织<br>序列化/校验标记：<code>`json:&quot;organization_id&quot;`</code> |
| <code>Departed</code> | <code>uint64</code> | 离组用户<br>序列化/校验标记：<code>`json:&quot;departed_user_id&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 原成员来源代次<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 原离组号<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Event</code> | <code>string</code> | 按操作与文档派生的唯一事件号<br>序列化/校验标记：<code>`json:&quot;event_id&quot;`</code> |
| <code>Recipients</code> | <code>&#91;&#93;uint64</code> | 当时真实活动KB管理员升序集合<br>序列化/校验标记：<code>`json:&quot;recipients&quot;`</code> |
| <code>DeliveryAfter</code> | <code>uint64</code> | 已提交投递游标，必须属于原集合<br>序列化/校验标记：<code>`json:&quot;delivery_after&quot;`</code> |

<!-- write-step: 7 bytes=950:1613 -->
````go
// DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、
// 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。
type DepartureNotification struct {
	Document      uint64   `json:"document_id"`
	KnowledgeBase uint64   `json:"knowledge_base_id"`
	Organization  uint64   `json:"organization_id"`
	Departed      uint64   `json:"departed_user_id"`
	Epoch         uint64   `json:"epoch"`
	Operation     string   `json:"operation_id"`
	Event         string   `json:"event_id"`
	Recipients    []uint64 `json:"recipients"`
	DeliveryAfter uint64   `json:"delivery_after"`
}

````

## 第 8 步：DepartureNotificationEvent

插入位置：第 7 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func DepartureNotificationEvent(operation string, document uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;:&quot;</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;departure-&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;departure-&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1613:1814 -->
````go
func DepartureNotificationEvent(operation string, document uint64) string {
	digest := sha256.Sum256([]byte(operation + ":" + TextID(document)))
	return "departure-" + hex.EncodeToString(digest[:])
}

````

## 第 9 步：DecodeDepartureNotification

插入位置：第 8 步代码之后，紧接追加；完成稿约第 61 行。

DecodeDepartureNotification 严格拒绝未知字段、非升序集合及不在原集合中的游标。
数字直接解码为uint64，避免通过浮点数丢失身份精度。

本步声明：`func DecodeDepartureNotification(raw []byte) (DepartureNotification, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DepartureNotification</code> | 使用 <code>DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>实际返回表达式：<code>event</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;event) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return event, invalid</code>。当前函数在这里结束。
- 检查 <code>event.Document == 0 &#124;&#124; event.KnowledgeBase == 0 &#124;&#124; event.Organization == 0 &#124;&#124; event.Departed == 0 &#124;&#124; event.Epoch == 0 &#124;&#124; !ValidOperationID(event.Operation) &#124;&#124; event.Event != Depart …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return event, invalid</code>。当前函数在这里结束。
- 检查 <code>recipient &lt;= previous &#124;&#124; recipient == event.Departed</code>。条件成立时执行 <code>return event, invalid</code>。当前函数在这里结束。
- 检查 <code>!cursorFound</code>。条件成立时执行 <code>return event, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return event, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=1814:3006 -->
````go
// DecodeDepartureNotification 严格拒绝未知字段、非升序集合及不在原集合中的游标。
// 数字直接解码为uint64，避免通过浮点数丢失身份精度。
func DecodeDepartureNotification(raw []byte) (DepartureNotification, error) {
	var event DepartureNotification
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	invalid := errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	if decoder.Decode(&event) != nil || decoder.Decode(new(any)) != io.EOF {
		return event, invalid
	}
	if event.Document == 0 || event.KnowledgeBase == 0 || event.Organization == 0 || event.Departed == 0 || event.Epoch == 0 || !ValidOperationID(event.Operation) || event.Event != DepartureNotificationEvent(event.Operation, event.Document) || event.Recipients == nil {
		return event, invalid
	}
	previous, cursorFound := uint64(0), event.DeliveryAfter == 0
	for _, recipient := range event.Recipients {
		if recipient <= previous || recipient == event.Departed {
			return event, invalid
		}
		previous = recipient
		cursorFound = cursorFound || recipient == event.DeliveryAfter
	}
	if !cursorFound {
		return event, invalid
	}
	return event, nil
}

````

## 第 10 步：SameDepartureNotification

插入位置：第 9 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func SameDepartureNotification(left, right DepartureNotification) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>DepartureNotification</code> | 使用 <code>DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>在此声明中的实际使用：<code>left.DeliveryAfter</code>；<code>reflect.DeepEqual(left, right)</code> |
| <code>right</code> | <code>DepartureNotification</code> | 使用 <code>DepartureNotification</code> 类型；其定义处的职责：DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。<br>在此声明中的实际使用：<code>right.DeliveryAfter</code>；<code>reflect.DeepEqual(left, right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>reflect.DeepEqual(left, right)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return reflect.DeepEqual(left, right)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3006:3240 -->
````go
func SameDepartureNotification(left, right DepartureNotification) bool {
	left.DeliveryAfter, right.DeliveryAfter = 0, 0
	return reflect.DeepEqual(left, right)
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 11 步：OrganizationClosePlan

插入位置：第 10 步代码之后，紧接追加；完成稿约第 95 行。

仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。

本步声明：`type OrganizationClosePlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>platform.OrganizationClosureBinding</code> | Account权威组织关闭绑定，非账号注销绑定<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Retention</code> | <code>*AccountRetentionPlan</code> | 原组织内固定Doc/KB/Space和公开版依赖计划<br>序列化/校验标记：<code>`json:&quot;retention&quot;`</code> |

<!-- write-step: 11 bytes=3240:3514 -->
````go
// 仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。
type OrganizationClosePlan struct {
	Binding   platform.OrganizationClosureBinding `json:"binding"`
	Retention *AccountRetentionPlan               `json:"retention"`
}

````

## 第 12 步：DecodeOrganizationClosePlan

插入位置：第 11 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OrganizationClosePlan</code> | 使用 <code>OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>d.Decode(&amp;plan) != nil &#124;&#124; d.Decode(new(any)) != io.EOF &#124;&#124; !plan.Binding.Valid() &#124;&#124; plan.Retention == nil &#124;&#124; !plan.Retention.Valid(plan.Binding.Operation)</code>。条件成立时执行 <code>return plan, errors.New(&quot;ORGANIZATION_CLOSE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3514:4020 -->
````go
func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error) {
	var plan OrganizationClosePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&plan) != nil || d.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || plan.Retention == nil || !plan.Retention.Valid(plan.Binding.Operation) {
		return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
	}
	return plan, nil
}

// organization_close_document.go：保留原实现，按业务合并文件。

````

## 第 13 步：OrganizationClosePlan.CleanupDocument

插入位置：第 12 步代码之后，紧接追加；完成稿约第 113 行。

CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。

本步声明：`func (p OrganizationClosePlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>cleanupRetentionDocument(p.Retention, document, &quot;ORGANIZATION&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionDocument</code> | 使用 <code>AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ORGANIZATION&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;AccountRetentionScope</code> | 使用 <code>AccountRetentionScope</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ORGANIZATION&quot;)</code> 的第 2 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ORGANIZATION&quot;)</code> 的第 3 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return cleanupRetentionDocument(p.Retention, document, &quot;ORGANIZATION&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4020:4491 -->
````go
// CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
// 拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。
func (p OrganizationClosePlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error) {
	return cleanupRetentionDocument(p.Retention, document, "ORGANIZATION")
}

// personal_transfer.go：保留原实现，按业务合并文件。

````

## 第 14 步：PersonalTransfer

插入位置：第 13 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`type PersonalTransfer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 转移邀约ID |
| <code>Document</code> | <code>uint64</code> | 转移文档 |
| <code>SourceUser</code> | <code>uint64</code> | 当前个人所有者 |
| <code>TargetUser</code> | <code>uint64</code> | 被邀请接收人 |
| <code>SourceSpace</code> | <code>uint64</code> | 原个人空间 |
| <code>ExpectedControl</code> | <code>uint64</code> | 发邀约时期待文档控制修订 |
| <code>Revision</code> | <code>uint64</code> | 邀约并发修订 |
| <code>Title</code> | <code>string</code> | 邀约固定标题 |
| <code>State</code> | <code>string</code> | PENDING、ACCEPTING等转移状态 |
| <code>OfferOperation</code> | <code>string</code> | 创建邀约原号 |
| <code>AcceptOperation</code> | <code>string</code> | 接受邀约原号 |
| <code>Expires</code> | <code>time.Time</code> | 邀约绝对期限 |
| <code>Now</code> | <code>time.Time</code> | 数据库当前时间，用于期限判断 |
| <code>Plan</code> | <code>PersonalTransferPlan</code> | 接受后持久固定的迁移计划 |

<!-- write-step: 14 bytes=4491:4970 -->
````go
type PersonalTransfer struct {
	ID, Document, SourceUser, TargetUser, SourceSpace, ExpectedControl, Revision uint64
	Title, State, OfferOperation, AcceptOperation                                string
	Expires                                                                      time.Time
	Now                                                                          time.Time
	Plan                                                                         PersonalTransferPlan
}

````

## 第 15 步：PersonalTransferPlan

插入位置：第 14 步代码之后，紧接追加；完成稿约第 129 行。

固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。

本步声明：`type PersonalTransferPlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 接受迁移原号<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Transfer</code> | <code>uint64</code> | 邀约ID |
| <code>Document</code> | <code>uint64</code> | 唯一转移文档 |
| <code>SourceUser</code> | <code>uint64</code> | 原所有者 |
| <code>TargetUser</code> | <code>uint64</code> | 接收者 |
| <code>SourceSpace</code> | <code>uint64</code> | 原空间 |
| <code>TargetSpace</code> | <code>uint64</code> | 接收者个人空间 |
| <code>TargetKB</code> | <code>uint64</code> | 目标知识库 |
| <code>ExpectedControl</code> | <code>uint64</code> | 接受前文档控制修订 |
| <code>ClosingControl</code> | <code>uint64</code> | 封闭迁移期间控制修订 |
| <code>ACL</code> | <code>uint64</code> | 需关闭的文档ACL代次 |
| <code>Publication</code> | <code>uint64</code> | 需关闭的发布代次 |
| <code>BodyBytes</code> | <code>uint64</code> | 目标计费的固定历史正文字节 |
| <code>SourceBodyBytes</code> | <code>uint64</code> | 源空间原正文字节计费 |
| <code>ReservedBytes</code> | <code>uint64</code> | 目标预留总额，必须等于正文、资源增量与NodeBytes之和 |
| <code>Resources</code> | <code>&#91;&#93;TransferResource</code> | 固定迁移资源及目标是否新增计费 |
| <code>Scopes</code> | <code>&#91;&#93;PurgeScope</code> | 原Doc ACL与publication两条关闭绑定 |
| <code>SourceIdentity</code> | <code>&#91;&#93;byte</code> | 原所有者权威身份证明字节 |
| <code>ReceiverIdentity</code> | <code>&#91;&#93;byte</code> | 接收者权威身份证明字节 |
| <code>ReceiverSession</code> | <code>&#91;&#93;byte</code> | 接收者真实会话引用，固定32字节 |
| <code>NodeBytes</code> | <code>uint64</code> | 关闭后Node实际存储计量 |
| <code>NodeMeasurement</code> | <code>&#91;&#93;byte</code> | 该原计量摘要，防重试换数据 |

<!-- write-step: 15 bytes=4970:5968 -->
````go
// 固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。
type PersonalTransferPlan struct {
	Operation                                                                                    string `json:"operation_id"`
	Transfer, Document, SourceUser, TargetUser, SourceSpace, TargetSpace, TargetKB               uint64
	ExpectedControl, ClosingControl, ACL, Publication, BodyBytes, SourceBodyBytes, ReservedBytes uint64
	Resources                                                                                    []TransferResource
	Scopes                                                                                       []PurgeScope
	SourceIdentity, ReceiverIdentity, ReceiverSession                                            []byte
	NodeBytes                                                                                    uint64
	NodeMeasurement                                                                              []byte
}

````

## 第 16 步：TransferResource

插入位置：第 15 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`type TransferResource struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 共享资源ID |
| <code>Size</code> | <code>uint64</code> | 物理对象字节数 |
| <code>TargetCharge</code> | <code>bool</code> | 目标空间是否尚无ACTIVE分配，需要新增计费 |

<!-- write-step: 16 bytes=5968:6042 -->
````go
type TransferResource struct {
	ID, Size     uint64
	TargetCharge bool
}

````

## 第 17 步：PersonalTransferFence

插入位置：第 16 步代码之后，紧接追加；完成稿约第 146 行。

本步声明：`func PersonalTransferFence(operation, scope string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;\n&quot;</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;\n&quot; + scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;personal_fence_&quot; + hex.EncodeToString(sum&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;personal_fence_&quot; + hex.EncodeToString(sum&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6042:6217 -->
````go
func PersonalTransferFence(operation, scope string) string {
	sum := sha256.Sum256([]byte(operation + "\n" + scope))
	return "personal_fence_" + hex.EncodeToString(sum[:])
}

````

## 第 18 步：ValidatePersonalTransferAction

插入位置：第 17 步代码之后，紧接追加；完成稿约第 151 行。

本步声明：`func ValidatePersonalTransferAction(offer PersonalTransfer, actor, expected uint64, action string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>offer</code> | <code>PersonalTransfer</code> | 在此声明中的实际使用：<code>offer.Revision</code>；<code>offer.State</code>；<code>offer.Expires</code>；<code>offer.Now</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor != offer.SourceUser</code>；<code>actor != offer.TargetUser</code> |
| <code>expected</code> | <code>uint64</code> | 在此声明中的实际使用：<code>offer.Revision != expected</code> |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>action == &quot;CANCEL&quot;</code>；<code>action != &quot;CANCEL&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;TRANSFER_REVISION_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;TRANSFER_NOT_PENDING&quot;)</code>；<code>Violation(StateConflict, &quot;TRANSFER_EXPIRED&quot;)</code>；<code>Violation(PermissionDenied, &quot;TRANSFER_PARTICIPANT_REQUIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>offer.Revision != expected</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;TRANSFER_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>offer.State != &quot;PENDING&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;TRANSFER_NOT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>!offer.Expires.After(offer.Now)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;TRANSFER_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>(action == &quot;CANCEL&quot; &amp;&amp; actor != offer.SourceUser) &#124;&#124; (action != &quot;CANCEL&quot; &amp;&amp; actor != offer.TargetUser)</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;TRANSFER_PARTICIPANT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6217:6807 -->
````go
func ValidatePersonalTransferAction(offer PersonalTransfer, actor, expected uint64, action string) error {
	if offer.Revision != expected {
		return Violation(StateConflict, "TRANSFER_REVISION_CHANGED")
	}
	if offer.State != "PENDING" {
		return Violation(StateConflict, "TRANSFER_NOT_PENDING")
	}
	if !offer.Expires.After(offer.Now) {
		return Violation(StateConflict, "TRANSFER_EXPIRED")
	}
	if (action == "CANCEL" && actor != offer.SourceUser) || (action != "CANCEL" && actor != offer.TargetUser) {
		return Violation(PermissionDenied, "TRANSFER_PARTICIPANT_REQUIRED")
	}
	return nil
}

````

## 第 19 步：TransferCapacity

插入位置：第 18 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`func TransferCapacity(body, resources uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>uint64</code> | 在此声明中的实际使用：<code>^uint64(0)-body</code>；<code>body + resources</code> |
| <code>resources</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources &gt; ^uint64(0)-body</code>；<code>body + resources</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>body + resources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;TRANSFER_CAPACITY_OVERFLOW&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>resources &gt; ^uint64(0)-body</code>。条件成立时执行 <code>return 0, Violation(StateConflict, &quot;TRANSFER_CAPACITY_OVERFLOW&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return body + resources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=6807:7008 -->
````go
func TransferCapacity(body, resources uint64) (uint64, error) {
	if resources > ^uint64(0)-body {
		return 0, Violation(StateConflict, "TRANSFER_CAPACITY_OVERFLOW")
	}
	return body + resources, nil
}

````

## 第 20 步：PersonalTransferPlan.Valid

插入位置：第 19 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func (p PersonalTransferPlan) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; total == p.ReservedBytes &amp;&amp; (len(p.NodeMeasurement) == 0 &#124;&#124; len(p.NodeMeasurement) == 32)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(p.Operation) &#124;&#124; p.Transfer == 0 &#124;&#124; p.Document == 0 &#124;&#124; p.SourceUser == 0 &#124;&#124; p.TargetUser == 0 &#124;&#124; p.SourceUser == p.TargetUser &#124;&#124; p.SourceSpace == 0 &#124;&#124; p.TargetSpac …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>i == 1</code>。
- 检查 <code>s.Key != key &#124;&#124; s.Epoch != epoch &#124;&#124; s.Operation != PersonalTransferFence(p.Operation, key)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>r.ID == 0 &#124;&#124; r.ID &lt;= previous</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>r.TargetCharge</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; total == p.ReservedBytes &amp;&amp; (len(p.NodeMeasurement) == 0 &#124;&#124; len(p.NodeMeasurement) == 32)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=7008:8518 -->
````go
func (p PersonalTransferPlan) Valid() bool {
	if !ValidOperationID(p.Operation) || p.Transfer == 0 || p.Document == 0 || p.SourceUser == 0 || p.TargetUser == 0 || p.SourceUser == p.TargetUser || p.SourceSpace == 0 || p.TargetSpace == 0 || p.SourceSpace == p.TargetSpace || p.TargetKB == 0 || p.ExpectedControl == ^uint64(0) || p.ClosingControl != p.ExpectedControl+1 || p.ClosingControl == ^uint64(0) || p.ACL == 0 || p.ACL == ^uint64(0) || p.Publication == 0 || p.Publication == ^uint64(0) || len(p.Scopes) != 2 || len(p.ReceiverSession) != 32 || len(p.SourceIdentity) == 0 || len(p.ReceiverIdentity) == 0 {
		return false
	}
	for i, s := range p.Scopes {
		key, epoch := "doc-acl:"+TextID(p.Document), p.ACL
		if i == 1 {
			key, epoch = "publication:"+TextID(p.Document), p.Publication
		}
		if s.Key != key || s.Epoch != epoch || s.Operation != PersonalTransferFence(p.Operation, key) {
			return false
		}
	}
	var previous, total uint64
	for _, r := range p.Resources {
		if r.ID == 0 || r.ID <= previous {
			return false
		}
		previous = r.ID
		if r.TargetCharge {
			var err error
			total, err = TransferCapacity(total, r.Size)
			if err != nil {
				return false
			}
		}
	}
	total, err := TransferCapacity(total, p.BodyBytes)
	if err != nil {
		return false
	}
	total, err = TransferCapacity(total, p.NodeBytes)
	return err == nil && total == p.ReservedBytes && (len(p.NodeMeasurement) == 0 || len(p.NodeMeasurement) == 32)
}

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 21 步：RetainedGovernancePurpose

插入位置：第 20 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`const RetainedGovernancePurpose = "RETAINED_GOVERNANCE"`。

<!-- write-step: 21 bytes=8518:8575 -->
````go
const RetainedGovernancePurpose = "RETAINED_GOVERNANCE"

````

## 第 22 步：RetainedGovernancePlan

插入位置：第 21 步代码之后，紧接追加；完成稿约第 213 行。

平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。
PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。

本步声明：`type RetainedGovernancePlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Purge</code> | <code>PurgePlan</code> | 复用单Doc原号、代次与关闭scope形状<br>序列化/校验标记：<code>`json:&quot;purge&quot;`</code> |
| <code>Action</code> | <code>string</code> | UNPUBLISH或PURGE<br>序列化/校验标记：<code>`json:&quot;action&quot;`</code> |
| <code>Release</code> | <code>uint64</code> | 被治理的固定公开版<br>序列化/校验标记：<code>`json:&quot;release_id&quot;`</code> |
| <code>Resources</code> | <code>&#91;&#93;uint64</code> | 固定待清理资源升序集合<br>序列化/校验标记：<code>`json:&quot;resources&quot;`</code> |
| <code>KnowledgeBaseACL</code> | <code>uint64</code> | 原留存KB ACL代次，复核父级未换<br>序列化/校验标记：<code>`json:&quot;knowledge_base_acl&quot;`</code> |

<!-- write-step: 22 bytes=8575:9065 -->
````go
// 平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。
// PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。
type RetainedGovernancePlan struct {
	Purge            PurgePlan `json:"purge"`
	Action           string    `json:"action"`
	Release          uint64    `json:"release_id"`
	Resources        []uint64  `json:"resources"`
	KnowledgeBaseACL uint64    `json:"knowledge_base_acl"`
}

````

## 第 23 步：ValidateRetainedGovernance

插入位置：第 22 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func ValidateRetainedGovernance(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, action, expected string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>TrashFacts</code> | 使用 <code>TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.Document</code> |
| <code>space</code> | <code>dto.Space</code> | 在此声明中的实际使用：<code>space.Id</code>；<code>space.Lifecycle</code>；<code>Space: space</code> |
| <code>kb</code> | <code>*dto.KnowledgeBase</code> | 在此声明中的实际使用：<code>kb == nil</code>；<code>kb.Id</code>；<code>kb.SpaceId</code>；<code>kb.Lifecycle</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>action != &quot;UNPUBLISH&quot;</code>；<code>action != &quot;PURGE&quot;</code>；<code>action == &quot;PURGE&quot;</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>d.ControlRevision != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;RETAINED_ACTION_INVALID&quot;)</code>；<code>Violation(PermissionDenied, &quot;ISOLATED_PUBLIC_RETAINED_REQUIRED&quot;)</code>；<code>Violation(StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>；<code>err</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>action != &quot;UNPUBLISH&quot; &amp;&amp; action != &quot;PURGE&quot;</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;RETAINED_ACTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.Lifecycle != &quot;PUBLIC_RETAINED&quot; &#124;&#124; d.ResponsibleUserId != nil &#124;&#124; !d.MainLocked &#124;&#124; 		space.Id != d.SpaceId &#124;&#124; space.Lifecycle != &quot;PUBLIC_RETAINED&quot;</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;ISOLATED_PUBLIC_RETAINED_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.KnowledgeBaseId != nil &amp;&amp; (kb == nil &#124;&#124; kb.Id != *d.KnowledgeBaseId &#124;&#124; kb.SpaceId != space.Id &#124;&#124; kb.Lifecycle != &quot;PUBLIC_RETAINED&quot;)</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;ISOLATED_PUBLIC_RETAINED_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.ControlRevision != expected</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.PublicationState == &quot;PUBLISHED&quot; &amp;&amp; d.CurrentReleaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>policy != &quot;PUBLIC&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;RETAINED_PUBLIC_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>action == &quot;PURGE&quot; &amp;&amp; d.PublicationState == &quot;UNPUBLISHED&quot; &amp;&amp; d.CurrentReleaseId == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return Violation(StateConflict, &quot;RETAINED_PUBLICATION_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=9065:10660 -->
````go
func ValidateRetainedGovernance(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, action, expected string) error {
	d := facts.Document
	if action != "UNPUBLISH" && action != "PURGE" {
		return Violation(InvalidRequest, "RETAINED_ACTION_INVALID")
	}
	// 原负责人已解除、父空间已关闭协作能力才是孤立留存；普通ACTIVE文档即使
	// 对外公开，也绝不因平台管理身份获得代替负责人操作的入口。
	if d.Lifecycle != "PUBLIC_RETAINED" || d.ResponsibleUserId != nil || !d.MainLocked ||
		space.Id != d.SpaceId || space.Lifecycle != "PUBLIC_RETAINED" {
		return Violation(PermissionDenied, "ISOLATED_PUBLIC_RETAINED_REQUIRED")
	}
	if d.KnowledgeBaseId != nil && (kb == nil || kb.Id != *d.KnowledgeBaseId || kb.SpaceId != space.Id || kb.Lifecycle != "PUBLIC_RETAINED") {
		return Violation(PermissionDenied, "ISOLATED_PUBLIC_RETAINED_REQUIRED")
	}
	if d.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if d.PublicationState == "PUBLISHED" && d.CurrentReleaseId != nil {
		policy, err := PublicPolicy(PublicationFacts{Document: d, Space: space, KnowledgeBase: kb})
		if err != nil {
			return err
		}
		if policy != "PUBLIC" {
			return Violation(StateConflict, "RETAINED_PUBLIC_BINDING_CHANGED")
		}
		return nil
	}
	// 已撤下的孤立内容仍可永久删除，但不能重新撤下以制造一个新关闭成功。
	if action == "PURGE" && d.PublicationState == "UNPUBLISHED" && d.CurrentReleaseId == nil {
		return nil
	}
	return Violation(StateConflict, "RETAINED_PUBLICATION_CHANGED")
}

````

## 第 24 步：RetainedGovernancePlan.Valid

插入位置：第 23 步代码之后，紧接追加；完成稿约第 257 行。

本步声明：`func (p RetainedGovernancePlan) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!p.Purge.Valid() &#124;&#124; p.Purge.Automatic &#124;&#124; (p.Action != &quot;UNPUBLISH&quot; &amp;&amp; p.Action != &quot;PURGE&quot;) &#124;&#124; 		(p.Action == &quot;UNPUBLISH&quot; &amp;&amp; p.Release == 0) &#124;&#124; p.Resources == nil &#124;&#124; !slices.IsSorted …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; (i &gt; 0 &amp;&amp; p.Resources&#91;i-1&#93; == id)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10660:11109 -->
````go
func (p RetainedGovernancePlan) Valid() bool {
	if !p.Purge.Valid() || p.Purge.Automatic || (p.Action != "UNPUBLISH" && p.Action != "PURGE") ||
		(p.Action == "UNPUBLISH" && p.Release == 0) || p.Resources == nil || !slices.IsSorted(p.Resources) || (p.Purge.KnowledgeBase != 0 && p.KnowledgeBaseACL == 0) {
		return false
	}
	for i, id := range p.Resources {
		if id == 0 || (i > 0 && p.Resources[i-1] == id) {
			return false
		}
	}
	return true
}

````

## 第 25 步：DecodeRetainedGovernancePlan

插入位置：第 24 步代码之后，紧接追加；完成稿约第 270 行。

本步声明：`func DecodeRetainedGovernancePlan(raw []byte) (RetainedGovernancePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code>；<code>len(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>RetainedGovernancePlan</code> | 使用 <code>RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>实际返回表达式：<code>p</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;RETAINED_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 1&lt;&lt;20 &#124;&#124; d.Decode(&amp;p) != nil &#124;&#124; d.Decode(new(any)) != io.EOF &#124;&#124; !p.Valid()</code>。条件成立时执行 <code>return p, Violation(TemporarilyUnavailable, &quot;RETAINED_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=11109:11474 -->
````go
func DecodeRetainedGovernancePlan(raw []byte) (RetainedGovernancePlan, error) {
	var p RetainedGovernancePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 1<<20 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || !p.Valid() {
		return p, Violation(TemporarilyUnavailable, "RETAINED_PLAN_INVALID")
	}
	return p, nil
}

````

## 第 26 步：RetainedGovernanceKind

插入位置：第 25 步代码之后，紧接追加；完成稿约第 280 行。

本步声明：`func RetainedGovernanceKind(action string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;RETAINED_PUBLIC_&quot; + action</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;RETAINED_PUBLIC_&quot; + action</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;RETAINED_PUBLIC_&quot; + action</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=11474:11564 -->
````go
func RetainedGovernanceKind(action string) string {
	return "RETAINED_PUBLIC_" + action
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/retirement_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
