# 分步编写 `packages/platform/account_closure.go`

先按 [F04 · 原操作、资格来源与跨服务身份](../lessons/foundation/04-original-operation.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/018-account-closure.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义Account与Document之间的固定注销绑定和有界清理分页协议。

用户只请求注销自己，实际关闭组织集合来自Account锁内事实，恢复不能替换或扩大范围。

写之前先按这条执行路线理解各段的连接：

1. ValidateAccountClosureBinding要求非nil且严格升序组织集合；Equal逐字段比较。
2. AccountClosurePage.Matches校验32字节摘要、原binding、最多100项、递增文档号和NextAfter。
3. Confirmed仅证明来源ACK齐全，PLANNED仅证明Doc计划固定；都不等同整个注销完成。

本文件需要守住的边界：

- 非末页必须恰好100项，不能接受截断集合；继续页必须携带原PlanDigest。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/account_closure.go') -and (Get-Item -LiteralPath 'packages/platform/account_closure.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/account_closure.go')) { New-Item -ItemType File -Path 'packages/platform/account_closure.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:77 -->
````go
package platform

import (
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
)

````

## 第 2 步：AccountClosureBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。
固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。

本步声明：`type AccountClosureBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | Account最初登记的注销操作号。<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>User</code> | <code>uint64</code> | 注销账号ID，由Account真实会话确定。<br>序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>IdentityEpoch</code> | <code>uint64</code> | 冻结前的旧身份代次，定义要关闭的identity范围。<br>序列化/校验标记：<code>`json:&quot;identity_epoch&quot;`</code> |
| <code>ClosingOrganizations</code> | <code>&#91;&#93;uint64</code> | 锁内固定、严格升序、非nil的独自存续组织ID集合；空数组表示确认无需关闭组织。<br>序列化/校验标记：<code>`json:&quot;closing_organizations&quot;`</code> |

<!-- write-step: 2 bytes=77:529 -->
````go
// AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。
// 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。
type AccountClosureBinding struct {
	Operation            string   `json:"operation_id"`
	User                 uint64   `json:"user_id"`
	IdentityEpoch        uint64   `json:"identity_epoch"`
	ClosingOrganizations []uint64 `json:"closing_organizations"`
}

````

## 第 3 步：accountClosureOperation

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`var accountClosureOperation = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$\`)`。

<!-- write-step: 3 bytes=529:616 -->
````go
var accountClosureOperation = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)

````

## 第 4 步：ValidateAccountClosureBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func ValidateAccountClosureBinding(binding AccountClosureBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>AccountClosureBinding</code> | 使用 <code>AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>binding.ClosingOrganizations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!accountClosureOperation.MatchString(binding.Operation) &#124;&#124; binding.User == 0 &#124;&#124; binding.IdentityEpoch == 0 &#124;&#124; binding.ClosingOrganizations == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>id &lt;= previous</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=616:1010 -->
````go
func ValidateAccountClosureBinding(binding AccountClosureBinding) bool {
	if !accountClosureOperation.MatchString(binding.Operation) || binding.User == 0 || binding.IdentityEpoch == 0 || binding.ClosingOrganizations == nil {
		return false
	}
	previous := uint64(0)
	for _, id := range binding.ClosingOrganizations {
		if id <= previous {
			return false
		}
		previous = id
	}
	return true
}

````

## 第 5 步：EqualAccountClosureBinding

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func EqualAccountClosureBinding(left, right AccountClosureBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>AccountClosureBinding</code> | 使用 <code>AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>left.Operation</code>；<code>left.User</code>；<code>left.IdentityEpoch</code>；<code>left.ClosingOrganizations</code> |
| <code>right</code> | <code>AccountClosureBinding</code> | 使用 <code>AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>right.Operation</code>；<code>right.User</code>；<code>right.IdentityEpoch</code>；<code>right.ClosingOrganizations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>left.Operation == right.Operation &amp;&amp; left.User == right.User &amp;&amp; left.IdentityEpoch == right.IdentityEpoch &amp;&amp; slices.Equal(left.ClosingOrganizations, right.ClosingOrganizations)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return left.Operation == right.Operation &amp;&amp; left.User == right.User &amp;&amp; left.IdentityEpoch == right.IdentityEpoch &amp;&amp; slices.Equal(left.ClosingOrganizations, right.ClosingOrganizations)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1010:1272 -->
````go
func EqualAccountClosureBinding(left, right AccountClosureBinding) bool {
	return left.Operation == right.Operation && left.User == right.User && left.IdentityEpoch == right.IdentityEpoch && slices.Equal(left.ClosingOrganizations, right.ClosingOrganizations)
}

````

## 第 6 步：AccountClosureDecision

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`type AccountClosureDecision struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Rejected</code> | <code>bool</code> | true仅表示Account已经持久拒绝同一原计划，供Doc释放对应预留。<br>序列化/校验标记：<code>`json:&quot;rejected&quot;`</code> |

<!-- write-step: 6 bytes=1272:1345 -->
````go
type AccountClosureDecision struct {
	Rejected bool `json:"rejected"`
}

````

## 第 7 步：AccountClosureConfirmation

插入位置：第 6 步代码之后，紧接追加；完成稿约第 43 行。

Confirmed只证明同一固定Account计划已经冻结并持久取得全部来源ACK，
不证明Doc公开保留、引用释放或整个账号注销已经完成。

本步声明：`type AccountClosureConfirmation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Confirmed</code> | <code>bool</code> | true说明原Account计划已冻结且全部来源ACK落库，不代表Doc清理结束。<br>序列化/校验标记：<code>`json:&quot;confirmed&quot;`</code> |

<!-- write-step: 7 bytes=1345:1595 -->
````go
// Confirmed只证明同一固定Account计划已经冻结并持久取得全部来源ACK，
// 不证明Doc公开保留、引用释放或整个账号注销已经完成。
type AccountClosureConfirmation struct {
	Confirmed bool `json:"confirmed"`
}

````

## 第 8 步：AccountClosureRetention

插入位置：第 7 步代码之后，紧接追加；完成稿约第 49 行。

PLANNED仅确认Doc固定了保留对象并登记输出屏障，不能解释为清理或注销完成。

本步声明：`type AccountClosureRetention struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | Doc固定保留规划阶段，例如PLANNED；不能当账号全局终态。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |

<!-- write-step: 8 bytes=1595:1772 -->
````go
// PLANNED仅确认Doc固定了保留对象并登记输出屏障，不能解释为清理或注销完成。
type AccountClosureRetention struct {
	State string `json:"state"`
}

````

## 第 9 步：AccountClosurePreparation

插入位置：第 8 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`type AccountClosurePreparation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | Doc对同一binding的准备结果，例如FROZEN或待交接状态。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>RequiredKnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 注销前尚需移交职责的知识库ID集合。<br>序列化/校验标记：<code>`json:&quot;required_knowledge_bases&quot;`</code> |
| <code>ReservedKnowledgeBases</code> | <code>&#91;&#93;uint64</code> | Doc已为原注销计划预留的知识库集合，须与Account复核计划一致。<br>序列化/校验标记：<code>`json:&quot;reserved_knowledge_bases&quot;`</code> |

<!-- write-step: 9 bytes=1772:1997 -->
````go
type AccountClosurePreparation struct {
	State                  string   `json:"state"`
	RequiredKnowledgeBases []uint64 `json:"required_knowledge_bases"`
	ReservedKnowledgeBases []uint64 `json:"reserved_knowledge_bases"`
}

````

## 第 10 步：AccountClosurePageRequest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 60 行。

控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要，
Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。

本步声明：`type AccountClosurePageRequest struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AccountClosureBinding</code> | <code>AccountClosureBinding</code> | 内嵌原AccountClosureBinding，携带操作/用户/旧身份及固定组织集合。 |
| <code>AfterDocument</code> | <code>string</code> | 上一篇已收到PAYLOADS_REMOVED确认的文档ID，初页为空。<br>序列化/校验标记：<code>`json:&quot;after_document&quot;`</code> |
| <code>PlanDigest</code> | <code>string</code> | Doc不可变保留计划SHA256 hex，继续页不可为空或更换。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |

<!-- write-step: 10 bytes=1997:2342 -->
````go
// 控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要，
// Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。
type AccountClosurePageRequest struct {
	AccountClosureBinding
	AfterDocument string `json:"after_document"`
	PlanDigest    string `json:"plan_digest"`
}

````

## 第 11 步：AccountClosureDocument

插入位置：第 10 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`type AccountClosureDocument struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>string</code> | 固定计划中的文档规范十进制ID。<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Release</code> | <code>string</code> | 应保留正式版ID字符串，0表示无正式版引用。<br>序列化/校验标记：<code>`json:&quot;release_id&quot;`</code> |

<!-- write-step: 11 bytes=2342:2457 -->
````go
type AccountClosureDocument struct {
	Document string `json:"document_id"`
	Release  string `json:"release_id"`
}

````

## 第 12 步：AccountClosurePage

插入位置：第 11 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`type AccountClosurePage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>AccountClosureBinding</code> | 响应所属原注销计划，必须与请求逐字段相等。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>State</code> | <code>string</code> | Doc计划的PLANNED/CLEANING/SUCCEEDED状态。<br>序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>PlanDigest</code> | <code>string</code> | 固定计划摘要，不能随分页改变。<br>序列化/校验标记：<code>`json:&quot;plan_digest&quot;`</code> |
| <code>Documents</code> | <code>&#91;&#93;AccountClosureDocument</code> | 最多100项按文档ID递增的固定依赖，不包含正文。<br>序列化/校验标记：<code>`json:&quot;documents&quot;`</code> |
| <code>NextAfter</code> | <code>string</code> | 本页最后文档ID；空页保持请求AfterDocument。<br>序列化/校验标记：<code>`json:&quot;next_after&quot;`</code> |
| <code>Done</code> | <code>bool</code> | true表示固定集合已到尾页，不单独证明每篇清理已完成。<br>序列化/校验标记：<code>`json:&quot;done&quot;`</code> |

<!-- write-step: 12 bytes=2457:2821 -->
````go
type AccountClosurePage struct {
	Binding    AccountClosureBinding    `json:"binding"`
	State      string                   `json:"state"`
	PlanDigest string                   `json:"plan_digest"`
	Documents  []AccountClosureDocument `json:"documents"`
	NextAfter  string                   `json:"next_after"`
	Done       bool                     `json:"done"`
}

````

## 第 13 步：AccountClosurePage.Matches

插入位置：第 12 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func (p AccountClosurePage) Matches(request AccountClosurePageRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>AccountClosurePageRequest</code> | 使用 <code>AccountClosurePageRequest</code> 类型；其定义处的职责：控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要， Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。<br>在此声明中的实际使用：<code>request.AccountClosureBinding</code>；<code>request.PlanDigest</code>；<code>request.AfterDocument</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>p.NextAfter == last &amp;&amp; (p.Done &#124;&#124; len(p.Documents) == 100)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != p.PlanDigest &#124;&#124; !EqualAccountClosureBinding(p.Binding, request.AccountClosureBinding) &#124;&#124; p.Documents == nil &#124;&#124; len( …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return id, err == nil &amp;&amp; (zero &#124;&#124; id != 0) &amp;&amp; strconv.FormatUint(id, 10) == text</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>request.AfterDocument != &quot;&quot;</code>。
- 检查 <code>!valid &#124;&#124; request.PlanDigest == &quot;&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!valid &#124;&#124; !releaseValid &#124;&#124; id &lt;= previous</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return p.NextAfter == last &amp;&amp; (p.Done &#124;&#124; len(p.Documents) == 100)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=2821:4061 -->
````go
func (p AccountClosurePage) Matches(request AccountClosurePageRequest) bool {
	digest, err := hex.DecodeString(p.PlanDigest)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != p.PlanDigest || !EqualAccountClosureBinding(p.Binding, request.AccountClosureBinding) || p.Documents == nil || len(p.Documents) > 100 || (request.PlanDigest != "" && request.PlanDigest != p.PlanDigest) || (p.State != "PLANNED" && p.State != "CLEANING" && p.State != "SUCCEEDED") {
		return false
	}
	canonical := func(text string, zero bool) (uint64, bool) {
		id, err := strconv.ParseUint(text, 10, 64)
		return id, err == nil && (zero || id != 0) && strconv.FormatUint(id, 10) == text
	}
	var previous uint64
	if request.AfterDocument != "" {
		var valid bool
		previous, valid = canonical(request.AfterDocument, false)
		if !valid || request.PlanDigest == "" {
			return false
		}
	}
	last := request.AfterDocument
	for _, document := range p.Documents {
		id, valid := canonical(document.Document, false)
		_, releaseValid := canonical(document.Release, true)
		if !valid || !releaseValid || id <= previous {
			return false
		}
		previous, last = id, document.Document
	}
	return p.NextAfter == last && (p.Done || len(p.Documents) == 100)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/account_closure.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/04-original-operation.md)。
