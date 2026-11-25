# 分步编写 `services/document/internal/domain/document/publication_models.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/098-publication-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义发布意图、正式指针水位及PUBLIC/SPACE_MEMBERS/COLLABORATORS受众判定。

正式版本受众独立于内部READ，设置PUBLIC不应开放MAIN、草稿或历史。

写之前先按这条执行路线理解各段的连接：

1. EffectiveVisibility计算KB和文档政策
2. HasCurrentPublication核生命周期与正式状态
3. ValidatePublicAudience核匿名/成员/内部资格
4. NextPublicationSequences同时推进epoch与事件序号。

本文件需要守住的边界：

- 留存只能继续显示已固定公开版
- 任一序号耗尽必须让整个发布动作失败。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/publication_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/publication_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/publication_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/publication_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:241 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"math"
)

// public_reading.go：保留原实现，按业务合并文件。

````

## 第 2 步：PublicationFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格，
PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。

本步声明：`type PublicationFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>dto.Document</code> | 文档含真实生命周期与正式指针 |
| <code>Space</code> | <code>dto.Space</code> | 当前父空间事实 |
| <code>KnowledgeBase</code> | <code>*dto.KnowledgeBase</code> | 可选父KB及可见政策 |
| <code>Sequence</code> | <code>uint64</code> | 独立索引事件水位 |

<!-- write-step: 2 bytes=241:563 -->
````go
// PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格，
// PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。
type PublicationFacts struct {
	Document      dto.Document
	Space         dto.Space
	KnowledgeBase *dto.KnowledgeBase
	Sequence      uint64
}

````

## 第 3 步：PublicPolicy

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func PublicPolicy(facts PublicationFacts) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>PublicationFacts</code> | 使用 <code>PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.KnowledgeBase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>policy</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;PUBLIC_POLICY_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return value == &quot;PUBLIC&quot; &#124;&#124; value == &quot;SPACE_MEMBERS&quot; &#124;&#124; value == &quot;COLLABORATORS&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!valid(policy)</code>。条件成立时执行 <code>return &quot;&quot;, Violation(TemporarilyUnavailable, &quot;PUBLIC_POLICY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.KnowledgeBase != nil</code>。
- 检查 <code>!valid(facts.KnowledgeBase.VisibilityPolicy)</code>。条件成立时执行 <code>return &quot;&quot;, Violation(TemporarilyUnavailable, &quot;PUBLIC_POLICY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return policy, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=563:1149 -->
````go
func PublicPolicy(facts PublicationFacts) (string, error) {
	valid := func(value string) bool {
		return value == "PUBLIC" || value == "SPACE_MEMBERS" || value == "COLLABORATORS"
	}
	policy := facts.Document.VisibilityPolicy
	if !valid(policy) {
		return "", Violation(TemporarilyUnavailable, "PUBLIC_POLICY_UNKNOWN")
	}
	if facts.KnowledgeBase != nil {
		if !valid(facts.KnowledgeBase.VisibilityPolicy) {
			return "", Violation(TemporarilyUnavailable, "PUBLIC_POLICY_UNKNOWN")
		}
		policy = EffectiveVisibility(facts.KnowledgeBase.VisibilityPolicy, policy)
	}
	return policy, nil
}

````

## 第 4 步：HasCurrentPublication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 37 行。

留存对象只可能继续展示已经固定的公开版；其他关闭、回收站或未知状态都不回退历史。

本步声明：`func HasCurrentPublication(facts PublicationFacts, policy string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>facts</code> | <code>PublicationFacts</code> | 使用 <code>PublicationFacts</code> 类型；其定义处的职责：PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格， PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>facts.Space</code>；<code>facts.KnowledgeBase</code> |
| <code>policy</code> | <code>string</code> | 在此声明中的实际使用：<code>policy == &quot;PUBLIC&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>!retained &#124;&#124; policy == &quot;PUBLIC&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>document.PublicationState != &quot;PUBLISHED&quot; &#124;&#124; document.CurrentReleaseId == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>facts.KnowledgeBase != nil</code>。
- 检查 <code>state == &quot;PUBLIC_RETAINED&quot;</code>。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return !retained &#124;&#124; policy == &quot;PUBLIC&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1149:1833 -->
````go
// 留存对象只可能继续展示已经固定的公开版；其他关闭、回收站或未知状态都不回退历史。
func HasCurrentPublication(facts PublicationFacts, policy string) bool {
	document := facts.Document
	if document.PublicationState != "PUBLISHED" || document.CurrentReleaseId == nil {
		return false
	}
	retained := false
	states := []string{document.Lifecycle, facts.Space.Lifecycle}
	if facts.KnowledgeBase != nil {
		states = append(states, facts.KnowledgeBase.Lifecycle)
	}
	for _, state := range states {
		if state == "PUBLIC_RETAINED" {
			retained = true
		} else if state != "ACTIVE" {
			return false
		}
	}
	return !retained || policy == "PUBLIC"
}

````

## 第 5 步：ValidatePublicAudience

插入位置：第 4 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func ValidatePublicAudience(policy string, authenticated, member, internal bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>policy</code> | <code>string</code> | 在此声明中的实际使用：<code>policy == &quot;PUBLIC&quot;</code>；<code>policy == &quot;SPACE_MEMBERS&quot;</code>；<code>policy == &quot;COLLABORATORS&quot;</code> |
| <code>authenticated</code> | <code>bool</code> | 在此声明中的实际使用：<code>!authenticated</code> |
| <code>member</code> | <code>bool</code> | 在此声明中的实际使用：<code>policy == &quot;SPACE_MEMBERS&quot; &amp;&amp; member</code> |
| <code>internal</code> | <code>bool</code> | 在此声明中的实际使用：<code>policy == &quot;COLLABORATORS&quot; &amp;&amp; internal</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>Violation(SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>；<code>Violation(PermissionDenied, &quot;PUBLIC_DOCUMENT_NOT_ALLOWED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>policy == &quot;PUBLIC&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>!authenticated</code>。条件成立时执行 <code>return Violation(SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>(policy == &quot;SPACE_MEMBERS&quot; &amp;&amp; member) &#124;&#124; (policy == &quot;COLLABORATORS&quot; &amp;&amp; internal)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return Violation(PermissionDenied, &quot;PUBLIC_DOCUMENT_NOT_ALLOWED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1833:2216 -->
````go
func ValidatePublicAudience(policy string, authenticated, member, internal bool) error {
	if policy == "PUBLIC" {
		return nil
	}
	if !authenticated {
		return Violation(SessionRequired, "SESSION_REQUIRED")
	}
	if (policy == "SPACE_MEMBERS" && member) || (policy == "COLLABORATORS" && internal) {
		return nil
	}
	return Violation(PermissionDenied, "PUBLIC_DOCUMENT_NOT_ALLOWED")
}

````

## 第 6 步：FixedRelease

插入位置：第 5 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`type FixedRelease struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Release</code> | <code>dto.Release</code> | 不可变正式版DTO |
| <code>Title</code> | <code>string</code> | 此正式版固定标题 |
| <code>Structure</code> | <code>string</code> | release自有manifest关联结构ID |
| <code>Style</code> | <code>string</code> | release自有样式ID |

<!-- write-step: 6 bytes=2216:2379 -->
````go
type FixedRelease struct {
	Release                 dto.Release
	Title, Structure, Style string
}

// publication.go：保留原实现，按业务合并文件。

````

## 第 7 步：PublicationIntent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 78 行。

PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖，
发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。

本步声明：`type PublicationIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 发布业务原号<br>序列化/校验标记：<code>`json:&quot;operation&quot;`</code> |
| <code>Kind</code> | <code>string</code> | RELEASE_CREATE、SWITCH或UNPUBLISH<br>序列化/校验标记：<code>`json:&quot;kind&quot;`</code> |
| <code>Publication</code> | <code>uint64</code> | 页面期待发布epoch<br>序列化/校验标记：<code>`json:&quot;publication&quot;`</code> |
| <code>Control</code> | <code>uint64</code> | 页面期待控制修订<br>序列化/校验标记：<code>`json:&quot;control&quot;`</code> |
| <code>Release</code> | <code>uint64</code> | 切换目标历史release，需证明同文档<br>序列化/校验标记：<code>`json:&quot;release&quot;`</code> |

<!-- write-step: 7 bytes=2379:2805 -->
````go
// PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖，
// 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。
type PublicationIntent struct {
	Operation   string `json:"operation"`
	Kind        string `json:"kind"`
	Publication uint64 `json:"publication"`
	Control     uint64 `json:"control"`
	Release     uint64 `json:"release"`
}

````

## 第 8 步：ParseReleaseCreate

插入位置：第 7 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func ParseReleaseCreate(input dto.ReleaseCreate) (PublicationIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.ReleaseCreate</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.ExpectedPublicationRevision</code>；<code>input.ExpectedControlRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PublicationIntent</code> | 使用 <code>PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(intent.Operation)</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=2805:3314 -->
````go
func ParseReleaseCreate(input dto.ReleaseCreate) (PublicationIntent, error) {
	intent := PublicationIntent{Operation: input.OperationId, Kind: "RELEASE_CREATE"}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	var err error
	if intent.Publication, err = CanonicalRevision(input.ExpectedPublicationRevision); err != nil {
		return intent, err
	}
	intent.Control, err = CanonicalRevision(input.ExpectedControlRevision)
	return intent, err
}

````

## 第 9 步：ParsePublication

插入位置：第 8 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func ParsePublication(input dto.Publication) (PublicationIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.Publication</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Action</code>；<code>input.ReleaseId</code>；<code>input.ExpectedPublicationRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PublicationIntent</code> | 使用 <code>PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(intent.Operation) &#124;&#124; 		(input.Action != &quot;SWITCH&quot; &amp;&amp; input.Action != &quot;UNPUBLISH&quot;) &#124;&#124; 		(input.Action == &quot;SWITCH&quot; &amp;&amp; input.ReleaseId == nil) &#124;&#124; 		(input.Action == &quot;UNPUBLISH&quot; &amp;&amp; input.ReleaseId != nil)</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>input.Action == &quot;SWITCH&quot;</code>。
- 此处通过 <code>return intent, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3314:4013 -->
````go
func ParsePublication(input dto.Publication) (PublicationIntent, error) {
	intent := PublicationIntent{Operation: input.OperationId, Kind: input.Action}
	if !ValidOperationID(intent.Operation) ||
		(input.Action != "SWITCH" && input.Action != "UNPUBLISH") ||
		(input.Action == "SWITCH" && input.ReleaseId == nil) ||
		(input.Action == "UNPUBLISH" && input.ReleaseId != nil) {
		return intent, Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	var err error
	if intent.Publication, err = CanonicalRevision(input.ExpectedPublicationRevision); err != nil {
		return intent, err
	}
	if input.Action == "SWITCH" {
		intent.Release, err = CanonicalID(*input.ReleaseId)
	}
	return intent, err
}

````

## 第 10 步：ValidatePublication

插入位置：第 9 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`func ValidatePublication(permissions Permissions, current dto.Document, intent PublicationIntent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>permissions</code> | <code>Permissions</code> | 使用 <code>Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>permissions.Manage</code> |
| <code>current</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>current.Lifecycle</code>；<code>current.PublicationEpoch</code>；<code>current.ControlRevision</code>；<code>current.PublicationState</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>PublicationIntent</code> | 使用 <code>PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>在此声明中的实际使用：<code>intent.Publication</code>；<code>intent.Kind</code>；<code>intent.Control</code>；<code>intent.Release</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>；<code>Violation(StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>；<code>Violation(StateConflict, &quot;PUBLICATION_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;PUBLICATION_SEQUENCE_EXHAUSTED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!permissions.Manage</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.PublicationEpoch != TextID(intent.Publication)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PUBLICATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Publication == math.MaxUint64</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PUBLICATION_SEQUENCE_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.ControlRevision != TextID(intent.Control)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;MAIN_CONTROL_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Release == 0</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.PublicationState == &quot;PUBLISHED&quot; &amp;&amp; current.CurrentReleaseId != nil &amp;&amp; *current.CurrentReleaseId == TextID(intent.Release)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PUBLICATION_ALREADY_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.PublicationState != &quot;PUBLISHED&quot; &#124;&#124; current.CurrentReleaseId == nil</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;PUBLICATION_NOT_PUBLISHED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return Violation(InvalidRequest, &quot;PUBLICATION_INPUT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4013:5390 -->
````go
func ValidatePublication(permissions Permissions, current dto.Document, intent PublicationIntent) error {
	// 先检查管理资格，不能借一个过期发布页面探知当前版本或推进新水位。
	if !permissions.Manage {
		return Violation(PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	if current.Lifecycle != "ACTIVE" {
		return Violation(StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	if current.PublicationEpoch != TextID(intent.Publication) {
		return Violation(StateConflict, "PUBLICATION_CHANGED")
	}
	if intent.Publication == math.MaxUint64 {
		return Violation(StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
	switch intent.Kind {
	case "RELEASE_CREATE":
		if current.ControlRevision != TextID(intent.Control) {
			return Violation(StateConflict, "MAIN_CONTROL_CHANGED")
		}
	case "SWITCH":
		if intent.Release == 0 {
			return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
		}
		if current.PublicationState == "PUBLISHED" && current.CurrentReleaseId != nil && *current.CurrentReleaseId == TextID(intent.Release) {
			return Violation(StateConflict, "PUBLICATION_ALREADY_CURRENT")
		}
	case "UNPUBLISH":
		if current.PublicationState != "PUBLISHED" || current.CurrentReleaseId == nil {
			return Violation(StateConflict, "PUBLICATION_NOT_PUBLISHED")
		}
	default:
		return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	return nil
}

````

## 第 11 步：NextPublicationSequences

插入位置：第 10 步代码之后，紧接追加；完成稿约第 155 行。

正式指针和outbox序号在同一事务推进；任何一个耗尽都使整个动作失败，
不能先创建正式版，再因序号溢出留下没有对应事件的业务结果。

本步声明：`func NextPublicationSequences(epoch, event uint64) (uint64, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>epoch == math.MaxUint64</code>；<code>epoch + 1</code> |
| <code>event</code> | <code>uint64</code> | 在此声明中的实际使用：<code>event == math.MaxUint64</code>；<code>event + 1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>epoch + 1</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>event + 1</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;PUBLICATION_SEQUENCE_EXHAUSTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>epoch == math.MaxUint64 &#124;&#124; event == math.MaxUint64</code>。条件成立时执行 <code>return 0, 0, Violation(StateConflict, &quot;PUBLICATION_SEQUENCE_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return epoch + 1, event + 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5390:5892 -->
````go
// 正式指针和outbox序号在同一事务推进；任何一个耗尽都使整个动作失败，
// 不能先创建正式版，再因序号溢出留下没有对应事件的业务结果。
func NextPublicationSequences(epoch, event uint64) (uint64, uint64, error) {
	if epoch == math.MaxUint64 || event == math.MaxUint64 {
		return 0, 0, Violation(StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
	return epoch + 1, event + 1, nil
}

// visibility.go：保留原实现，按业务合并文件。

````

## 第 12 步：EffectiveVisibility

插入位置：第 11 步代码之后，紧接追加；完成稿约第 166 行。

可见范围只限制正式版受众，不产生主线/草稿资格；设置PUBLIC也不能自动发布。

本步声明：`func EffectiveVisibility(knowledgeBase, document string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>knowledgeBase</code> | <code>string</code> | 在此声明中的实际使用：<code>rank&#91;knowledgeBase&#93; &gt; rank&#91;document&#93;</code>；<code>return knowledgeBase</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>rank&#91;knowledgeBase&#93; &gt; rank&#91;document&#93;</code>；<code>return document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>knowledgeBase</code>；<code>document</code> |

**跟着本段代码走一遍：**

- 检查 <code>rank&#91;knowledgeBase&#93; &gt; rank&#91;document&#93;</code>。条件成立时执行 <code>return knowledgeBase</code>。当前函数在这里结束。
- 此处通过 <code>return document</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5892:6234 -->
````go
// 可见范围只限制正式版受众，不产生主线/草稿资格；设置PUBLIC也不能自动发布。
func EffectiveVisibility(knowledgeBase, document string) string {
	rank := map[string]int{"PUBLIC": 0, "SPACE_MEMBERS": 1, "COLLABORATORS": 2}
	if rank[knowledgeBase] > rank[document] {
		return knowledgeBase
	}
	return document
}

````

## 第 13 步：KnowledgeBaseChild

插入位置：第 12 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`type KnowledgeBaseChild struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | KB子文档ID |
| <code>Epoch</code> | <code>uint64</code> | 文档当前ACL来源代次 |
| <code>HasRelease</code> | <code>bool</code> | 是否已有正式版，用于关闭/留存分支 |

<!-- write-step: 13 bytes=6234:6305 -->
````go
type KnowledgeBaseChild struct {
	ID, Epoch  uint64
	HasRelease bool
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/publication_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
