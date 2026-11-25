# 分步编写 `services/document/internal/domain/document/reviews_models.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/101-reviews-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义送审、固定轮次、修改意见及合并按钮条件。

审核对象是不可变轮次；请求revision只是并发条件，不会把旧页面自动升级为新正文的授权。

写之前先按这条执行路线理解各段的连接：

1. ParseSubmission从union DTO提取SUBMIT/RESUBMIT意图
2. ValidateSubmissionPosition核单元guard
3. ValidateSubmissionCapture核正文来源
4. CanResolveChange检查历次提交者
5. CanOfferMerge给提示。

本文件需要守住的边界：

- 普通REVIEW用户曾提交过此请求就受自审限制，重提不会洗掉历史；Manage例外仍须满足轮次与意见规则
- 浏览器不能提交内部cut或送审正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/reviews_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/reviews_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/reviews_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/reviews_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:376 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/json"
	"sort"
	"time"
)

// approval_actions.go：保留原实现，按业务合并文件。

````

## 第 2 步：MergeApprovalFacts

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

提示依据完整请求事实，与浏览器加载了多少页历史无关；实际写入仍由合并用例再次核验。

本步声明：`type MergeApprovalFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>PositionReady</code> | <code>bool</code> | 当前请求/轮次/guard是否允许开始批准 |
| <code>EverSubmitted</code> | <code>bool</code> | 此用户是否曾提交过该请求任何轮次 |
| <code>OwnOpen</code> | <code>uint64</code> | 当前用户自己尚未解决的修改意见数 |
| <code>ForeignOpen</code> | <code>uint64</code> | 其他用户尚未解决的修改意见数 |

<!-- write-step: 2 bytes=376:623 -->
````go
// 提示依据完整请求事实，与浏览器加载了多少页历史无关；实际写入仍由合并用例再次核验。
type MergeApprovalFacts struct {
	PositionReady bool
	EverSubmitted bool
	OwnOpen       uint64
	ForeignOpen   uint64
}

````

## 第 3 步：CanOfferMerge

插入位置：第 2 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func CanOfferMerge(permissions Permissions, facts MergeApprovalFacts) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>permissions</code> | <code>Permissions</code> | 使用 <code>Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>CanApproveMerge(permissions, facts.EverSubmitted)</code> |
| <code>facts</code> | <code>MergeApprovalFacts</code> | 使用 <code>MergeApprovalFacts</code> 类型；其定义处的职责：提示依据完整请求事实，与浏览器加载了多少页历史无关；实际写入仍由合并用例再次核验。<br>在此声明中的实际使用：<code>facts.PositionReady</code>；<code>facts.EverSubmitted</code>；<code>facts.ForeignOpen</code>；<code>facts.OwnOpen</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>facts.PositionReady &amp;&amp; CanApproveMerge(permissions, facts.EverSubmitted) &amp;&amp; 		facts.ForeignOpen == 0 &amp;&amp; facts.OwnOpen &lt;= 100</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return facts.PositionReady &amp;&amp; CanApproveMerge(permissions, facts.EverSubmitted) &amp;&amp; 		facts.ForeignOpen == 0 &amp;&amp; facts.OwnOpen &lt;= 100</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=623:1008 -->
````go
func CanOfferMerge(permissions Permissions, facts MergeApprovalFacts) bool {
	// 一并解除仅限本人要求，最多100项沿用ApproveMerge契约；管理例外只豁免自审身份。
	return facts.PositionReady && CanApproveMerge(permissions, facts.EverSubmitted) &&
		facts.ForeignOpen == 0 && facts.OwnOpen <= 100
}

// review.go：保留原实现，按业务合并文件。

````

## 第 4 步：ChangeFact

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type ChangeFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 正式修改意见ID |
| <code>Request</code> | <code>uint64</code> | 所属审核请求 |
| <code>Round</code> | <code>uint64</code> | 意见产生的固定轮次 |
| <code>Author</code> | <code>uint64</code> | 提出意见的审核者 |
| <code>Revision</code> | <code>uint64</code> | 意见并发版本 |
| <code>Status</code> | <code>string</code> | OPEN或已解决状态 |

<!-- write-step: 4 bytes=1008:1126 -->
````go
type ChangeFact struct {
	ID, Request, Round, Author, Revision uint64
	Status                               string
}

````

## 第 5 步：ValidateReviewRound

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

动作期待的是当前固定轮次；请求修订只控制并发，不能把旧页面升级为新轮次授权。

本步声明：`func ValidateReviewRound(pointer ReviewPointer, round uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pointer</code> | <code>ReviewPointer</code> | 在此声明中的实际使用：<code>pointer.Archived</code>；<code>pointer.Round</code>；<code>pointer.Status</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>pointer.Round != round</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;REVIEW_ROUND_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;REVIEW_REQUEST_NOT_REVIEWING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>pointer.Archived &#124;&#124; pointer.Round != round &#124;&#124; pointer.Round == 0</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;REVIEW_ROUND_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>pointer.Status != &quot;REVIEWING&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;REVIEW_REQUEST_NOT_REVIEWING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1126:1694 -->
````go
// 动作期待的是当前固定轮次；请求修订只控制并发，不能把旧页面升级为新轮次授权。
func ValidateReviewRound(pointer ReviewPointer, round uint64) error {
	if pointer.Archived || pointer.Round != round || pointer.Round == 0 {
		return Violation(StateConflict, "REVIEW_ROUND_CHANGED")
	}
	// 撤回保留修改要求，但所有短审核动作都必须等待重新送审；解除意见也不能绕过状态门槛。
	if pointer.Status != "REVIEWING" {
		return Violation(StateConflict, "REVIEW_REQUEST_NOT_REVIEWING")
	}
	return nil
}

````

## 第 6 步：CanResolveChange

插入位置：第 5 步代码之后，紧接追加；完成稿约第 50 行。

历次提交者限制针对整个请求，重提或取得 REVIEW 授权不会洗掉历史身份。

本步声明：`func CanResolveChange(permissions Permissions, actor uint64, change ChangeFact, everSubmitted bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>permissions</code> | <code>Permissions</code> | 使用 <code>Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>permissions.Manage</code>；<code>permissions.Review</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>change.Author == actor</code> |
| <code>change</code> | <code>ChangeFact</code> | 在此声明中的实际使用：<code>change.Author</code> |
| <code>everSubmitted</code> | <code>bool</code> | 在此声明中的实际使用：<code>!everSubmitted</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>permissions.Manage &#124;&#124; (permissions.Review &amp;&amp; change.Author == actor &amp;&amp; !everSubmitted)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return permissions.Manage &#124;&#124; (permissions.Review &amp;&amp; change.Author == actor &amp;&amp; !everSubmitted)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1694:2064 -->
````go
// 历次提交者限制针对整个请求，重提或取得 REVIEW 授权不会洗掉历史身份。
func CanResolveChange(permissions Permissions, actor uint64, change ChangeFact, everSubmitted bool) bool {
	return permissions.Manage || (permissions.Review && change.Author == actor && !everSubmitted)
}

// submission.go：保留原实现，按业务合并文件。

````

## 第 7 步：SubmissionIntent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 57 行。

SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。

本步声明：`type SubmissionIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 送审业务原号 |
| <code>Flush</code> | <code>string</code> | 可靠保存FLUSH引用，不能用客户端正文替代 |
| <code>Mode</code> | <code>string</code> | SUBMIT或RESUBMIT |
| <code>Branch</code> | <code>uint64</code> | 待送审活动支线ID |
| <code>Structure</code> | <code>uint64</code> | 页面期待结构 |
| <code>Request</code> | <code>uint64</code> | 重提时原审核请求ID |
| <code>ExpectedRound</code> | <code>uint64</code> | 重提时期待当前轮次 |
| <code>Canonical</code> | <code>dto.Submission</code> | 保留原冻结union DTO供一致指纹 |

<!-- write-step: 7 bytes=2064:2393 -->
````go
// SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。
type SubmissionIntent struct {
	Operation, Flush, Mode                    string
	Branch, Structure, Request, ExpectedRound uint64
	Canonical                                 dto.Submission
}

````

## 第 8 步：ParseSubmission

插入位置：第 7 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func ParseSubmission(input dto.Submission) (SubmissionIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.Submission</code> | 在此声明中的实际使用：<code>codec.Decode(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>SubmissionIntent</code> | 使用 <code>SubmissionIntent</code> 类型；其定义处的职责：SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。<br>由 <code>invalid()</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>invalid()</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return intent, Violation(InvalidRequest, &quot;SUBMISSION_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ValidOperationID(intent.Operation) &#124;&#124; !ValidOperationID(intent.Flush)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>intent.Mode != &quot;NEW&quot; &amp;&amp; intent.Mode != &quot;RESUBMIT&quot;</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>intent.Mode == &quot;RESUBMIT&quot;</code>。
- 检查 <code>len(fields) != len(allowed)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!allowed&#91;key&#93;</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>intent.Mode == &quot;RESUBMIT&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=2393:3889 -->
````go
func ParseSubmission(input dto.Submission) (SubmissionIntent, error) {
	var intent SubmissionIntent
	invalid := func() (SubmissionIntent, error) { return intent, Violation(InvalidRequest, "SUBMISSION_INVALID") }
	value, err := codec.Decode(input)
	if err != nil {
		return invalid()
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return invalid()
	}
	text := func(key string) string { value, _ := fields[key].(string); return value }
	intent.Operation, intent.Flush, intent.Mode = text("operation_id"), text("flush_receipt_id"), text("mode")
	if !ValidOperationID(intent.Operation) || !ValidOperationID(intent.Flush) {
		return invalid()
	}
	if intent.Mode != "NEW" && intent.Mode != "RESUBMIT" {
		return invalid()
	}
	allowed := map[string]bool{"operation_id": true, "flush_receipt_id": true, "mode": true, "branch_id": true, "expected_structure_id": true}
	if intent.Mode == "RESUBMIT" {
		allowed["request_id"], allowed["expected_round_id"] = true, true
	}
	if len(fields) != len(allowed) {
		return invalid()
	}
	for key := range fields {
		if !allowed[key] {
			return invalid()
		}
	}
	intent.Branch, err = CanonicalID(text("branch_id"))
	if err != nil {
		return invalid()
	}
	intent.Structure, err = CanonicalID(text("expected_structure_id"))
	if err != nil {
		return invalid()
	}
	if intent.Mode == "RESUBMIT" {
		intent.Request, err = CanonicalID(text("request_id"))
		if err != nil {
			return invalid()
		}
		intent.ExpectedRound, err = CanonicalID(text("expected_round_id"))
````

## 第 9 步：ParseSubmission（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 109 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 此处通过 <code>return intent, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3889:4116 -->
````go
		if err != nil {
			return invalid()
		}
	}
	intent.Canonical, err = codec.Canonical(fields)
	if err != nil {
		return invalid()
	}
	return intent, nil
}

// submission_capture.go：保留原实现，按业务合并文件。

````

## 第 10 步：CaptureExpectation

插入位置：第 9 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`type CaptureExpectation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Request</code> | <code>*pb.CaptureSubmissionRequest</code> | 已登记的真实捕获请求 |
| <code>Workspace</code> | <code>EditWorkspace</code> | 固定支线workspace/epoch/lineage/base |
| <code>Room</code> | <code>uint64</code> | Doc登记的协作房间ID |
| <code>LineageProvenance</code> | <code>&#91;&#93;byte</code> | 原lineage实际来源字节，用于核捕获没有伪造接纳来源 |

<!-- write-step: 10 bytes=4116:4285 -->
````go
type CaptureExpectation struct {
	Request           *pb.CaptureSubmissionRequest
	Workspace         EditWorkspace
	Room              uint64
	LineageProvenance []byte
}

````

## 第 11 步：ValidateSubmissionCapture

插入位置：第 10 步代码之后，紧接追加；完成稿约第 129 行。

Go校验固定业务JSON与来源，不解析Yjs。错误依赖结果不能被当成用户操作失败释放guard。

本步声明：`func ValidateSubmissionCapture(validator *codec.Validator, content *pb.FixedContent, expected CaptureExpectation) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator == nil</code>；<code>validator.Body</code>；<code>validator.Provenance</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content == nil</code>；<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.BranchId</code><br>另有 15 处使用，继续按下方完整代码的顺序处理。 |
| <code>expected</code> | <code>CaptureExpectation</code> | 在此声明中的实际使用：<code>expected.Request</code>；<code>expected.Workspace</code>；<code>expected.Room</code>；<code>expected.LineageProvenance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 由 <code>invalid()</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>invalid()</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, Violation(TemporarilyUnavailable, &quot;CAPTURE_CONTENT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>content == nil &#124;&#124; expected.Request == nil &#124;&#124; expected.Request.Context == nil &#124;&#124; expected.Request.Context.Target == nil &#124;&#124; validator == nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>content.DocumentId != target.DocumentId &#124;&#124; content.UnitId != target.UnitId &#124;&#124; content.BranchId != target.BranchId &#124;&#124; 		content.StructureId != target.StructureId &#124;&#124; content.Workspac …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest&#91;:&#93;, content.BodyHash)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Body(body) != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; len(cut) != 3 &#124;&#124; cut&#91;&quot;room_id&quot;&#93; != TextID(expected.Room) &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; != TextID(expected.Workspace.Epoch)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>sequence != &quot;0&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Provenance(provenanceValue) != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>provenance&#91;&quot;lineage_id&quot;&#93; != TextID(expected.Workspace.Lineage) &#124;&#124; provenance&#91;&quot;base_manifest_id&quot;&#93; != TextID(expected.Workspace.BaseManifest) &#124;&#124; 		provenance&#91;&quot;base_content_version_id …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=4285:6762 -->
````go
// Go校验固定业务JSON与来源，不解析Yjs。错误依赖结果不能被当成用户操作失败释放guard。
func ValidateSubmissionCapture(validator *codec.Validator, content *pb.FixedContent, expected CaptureExpectation) ([]uint64, error) {
	invalid := func() ([]uint64, error) { return nil, Violation(TemporarilyUnavailable, "CAPTURE_CONTENT_INVALID") }
	if content == nil || expected.Request == nil || expected.Request.Context == nil || expected.Request.Context.Target == nil || validator == nil {
		return invalid()
	}
	context, target := expected.Request.Context, expected.Request.Context.Target
	if content.DocumentId != target.DocumentId || content.UnitId != target.UnitId || content.BranchId != target.BranchId ||
		content.StructureId != target.StructureId || content.WorkspaceEpoch != target.WorkspaceEpoch || content.LineageRef != TextID(expected.Workspace.Lineage) ||
		content.OperationId != context.OperationId || content.CaptureId != context.OperationId || content.SchemaVersion != "content-v1" ||
		content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" || content.ProvenanceSchema != "provenance-v1" ||
		len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
		return invalid()
	}
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) {
		return invalid()
	}
	body, err := canonicalValue(content.BodyJsonUtf8)
	if err != nil || validator.Body(body) != nil {
		return invalid()
	}
	cutValue, err := canonicalValue(content.CutJsonUtf8)
	if err != nil {
		return invalid()
	}
	cut, ok := cutValue.(map[string]any)
	if !ok || len(cut) != 3 || cut["room_id"] != TextID(expected.Room) || cut["workspace_epoch"] != TextID(expected.Workspace.Epoch) {
		return invalid()
	}
	sequence, ok := cut["durable_seq"].(string)
	if !ok {
		return invalid()
	}
	if sequence != "0" {
		if _, err := CanonicalID(sequence); err != nil {
			return invalid()
		}
	}
	provenanceValue, err := canonicalValue(content.ProvenanceJsonUtf8)
	if err != nil || validator.Provenance(provenanceValue) != nil {
		return invalid()
	}
	provenance := provenanceValue.(map[string]any)
	if provenance["lineage_id"] != TextID(expected.Workspace.Lineage) || provenance["base_manifest_id"] != TextID(expected.Workspace.BaseManifest) ||
		provenance["base_content_version_id"] != TextID(expected.Workspace.BaseVersion) || provenance["source_operation_id"] != context.OperationId {
````

## 第 12 步：ValidateSubmissionCapture（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 175 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return invalid()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; validator.Provenance(originalValue) != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(accepted, originalAccepted)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>depth &gt; 64 &#124;&#124; count &gt; 100000</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>blocks&#91;id&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node&#91;&quot;type&quot;&#93; == &quot;image&quot;</code>。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>mark.(map&#91;string&#93;any)&#91;&quot;type&quot;&#93; == &quot;font&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=6762:7996 -->
````go
		return invalid()
	}
	originalValue, err := codec.Decode(expected.LineageProvenance)
	if err != nil || validator.Provenance(originalValue) != nil {
		return invalid()
	}
	original := originalValue.(map[string]any)
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return invalid()
	}
	originalAccepted, err := codec.Canonical(original["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, originalAccepted) {
		return invalid()
	}
	resources, blocks := map[uint64]bool{}, map[string]bool{}
	count := 0
	var inspect func(map[string]any, int) bool
	inspect = func(node map[string]any, depth int) bool {
		count++
		if depth > 64 || count > 100000 {
			return false
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return false
				}
				blocks[id] = true
			}
			if node["type"] == "image" {
				text, ok := attrs["resourceId"].(string)
				if !ok {
					return false
				}
				id, err := CanonicalID(text)
				if err != nil {
					return false
				}
				resources[id] = true
			}
		}
		if marks, ok := node["marks"].([]any); ok {
			for _, mark := range marks {
				if mark.(map[string]any)["type"] == "font" {
````

## 第 13 步：ValidateSubmissionCapture（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 220 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>!inspect(child.(map&#91;string&#93;any), depth+1)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!inspect(body.(map&#91;string&#93;any), 0)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(ids) != len(content.ResourceIds)</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 检查 <code>id != content.ResourceIds&#91;index&#93;</code>。条件成立时执行 <code>return invalid()</code>。当前函数在这里结束。
- 此处通过 <code>return ids, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7996:8625 -->
````go
					return false
				}
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				if !inspect(child.(map[string]any), depth+1) {
					return false
				}
			}
		}
		return true
	}
	if !inspect(body.(map[string]any), 0) {
		return invalid()
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != len(content.ResourceIds) {
		return invalid()
	}
	for index, id := range ids {
		if id != content.ResourceIds[index] {
			return invalid()
		}
	}
	return ids, nil
}

````

## 第 14 步：canonicalValue

插入位置：第 13 步代码之后，紧接追加；完成稿约第 252 行。

本步声明：`func canonicalValue(raw []byte) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code>；<code>bytes.Equal(raw, canonical)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>codec.ErrInvalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(raw, canonical)</code>。条件成立时执行 <code>return nil, codec.ErrInvalid</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=8625:8890 -->
````go
func canonicalValue(raw []byte) (any, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	canonical, err := codec.Canonical(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return nil, codec.ErrInvalid
	}
	return value, nil
}

````

## 第 15 步：OperationFingerprint

插入位置：第 14 步代码之后，紧接追加；完成稿约第 264 行。

OperationFingerprint复用现有receipt编码，供持久跨服务上下文使用同一个指纹。

本步声明：`func OperationFingerprint(path string, input any) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>json.Marshal(struct { 		Path  string 		Input any 	}{path, input})</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(struct { 		Path  string 		Input any 	}{path, input})</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>sha256.Sum256(encoded)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return sha256.Sum256(encoded), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8890:9254 -->
````go
// OperationFingerprint复用现有receipt编码，供持久跨服务上下文使用同一个指纹。
func OperationFingerprint(path string, input any) ([32]byte, error) {
	encoded, err := json.Marshal(struct {
		Path  string
		Input any
	}{path, input})
	return sha256.Sum256(encoded), err
}

// submission_state.go：保留原实现，按业务合并文件。

````

## 第 16 步：ReviewGuard

插入位置：第 15 步代码之后，紧接追加；完成稿约第 275 行。

本步声明：`type ReviewGuard struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Ongoing</code> | <code>uint64</code> | 当前正在审核的请求ID |
| <code>Resumable</code> | <code>uint64</code> | 已撤回但须沿原请求重提的ID |
| <code>Generation</code> | <code>uint64</code> | 送审占位代次 |
| <code>Submission</code> | <code>string</code> | 尚未固定完成的送审操作号 |

<!-- write-step: 16 bytes=9254:9361 -->
````go
type ReviewGuard struct {
	Ongoing, Resumable, Generation uint64
	Submission                     string
}

````

## 第 17 步：ReviewPointer

插入位置：第 16 步代码之后，紧接追加；完成稿约第 280 行。

本步声明：`type ReviewPointer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 审核请求ID |
| <code>Document</code> | <code>uint64</code> | 所属文档 |
| <code>Unit</code> | <code>uint64</code> | 原协作单元 |
| <code>Branch</code> | <code>uint64</code> | 原支线 |
| <code>Round</code> | <code>uint64</code> | 当前固定审核轮次 |
| <code>Revision</code> | <code>uint64</code> | 请求并发修订 |
| <code>Status</code> | <code>string</code> | REVIEWING、WITHDRAWN等业务状态 |
| <code>Archived</code> | <code>bool</code> | 是否已归档，归档不等于删除历史 |

<!-- write-step: 17 bytes=9361:9546 -->
````go
type ReviewPointer struct {
	ID, Document, Unit, Branch, Round, Revision uint64
	Status                                      string
	Archived                                    bool
}

````

## 第 18 步：SubmissionFact

插入位置：第 17 步代码之后，紧接追加；完成稿约第 286 行。

本步声明：`type SubmissionFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原送审号 |
| <code>Actor</code> | <code>uint64</code> | 提交者 |
| <code>Document</code> | <code>uint64</code> | 原文档 |
| <code>Unit</code> | <code>uint64</code> | 原单元 |
| <code>Branch</code> | <code>uint64</code> | 原支线 |
| <code>GuardGeneration</code> | <code>uint64</code> | 原送审guard代次 |
| <code>Request</code> | <code>uint64</code> | 固定后的审核请求 |
| <code>Round</code> | <code>uint64</code> | 固定后的审核轮次 |
| <code>State</code> | <code>string</code> | 原送审阶段 |

<!-- write-step: 18 bytes=9546:9791 -->
````go
type SubmissionFact struct {
	Operation                                                      string
	Actor, Document, Unit, Branch, GuardGeneration, Request, Round uint64
	State                                                          string
}

````

## 第 19 步：SubmissionTask

插入位置：第 18 步代码之后，紧接追加；完成稿约第 292 行。

本步声明：`type SubmissionTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | durable_task行号 |
| <code>Generation</code> | <code>uint64</code> | 本次worker领取代次，最终提交必须一致 |
| <code>Operation</code> | <code>string</code> | 被恢复的原业务操作号 |
| <code>Kind</code> | <code>string</code> | 恢复或收尾任务类型 |
| <code>Instance</code> | <code>string</code> | 当前领取实例 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 持久恢复上下文 |

<!-- write-step: 19 bytes=9791:9925 -->
````go
type SubmissionTask struct {
	ID, Generation            uint64
	Operation, Kind, Instance string
	Payload                   []byte
}

````

## 第 20 步：SubmissionResultFact

插入位置：第 19 步代码之后，紧接追加；完成稿约第 298 行。

本步声明：`type SubmissionResultFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 原操作阶段 |
| <code>Generation</code> | <code>uint64</code> | 回执关联业务代次 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 原请求摘要 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 固定结果JSON |
| <code>Expires</code> | <code>time.Time</code> | 原结果可公开查询期限 |

<!-- write-step: 20 bytes=9925:10078 -->
````go
type SubmissionResultFact struct {
	State               string
	Generation          uint64
	Fingerprint, Result []byte
	Expires             time.Time
}

````

## 第 21 步：ValidateSubmissionPosition

插入位置：第 20 步代码之后，紧接追加；完成稿约第 305 行。

一个单元只能有一个正在审核或可续接的请求；撤回不会变成创建新请求的许可。

本步声明：`func ValidateSubmissionPosition(intent SubmissionIntent, guard ReviewGuard, pointer *ReviewPointer, document, unit uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>intent</code> | <code>SubmissionIntent</code> | 使用 <code>SubmissionIntent</code> 类型；其定义处的职责：SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。<br>在此声明中的实际使用：<code>intent.Mode</code>；<code>intent.Request</code>；<code>intent.Branch</code>；<code>intent.ExpectedRound</code> |
| <code>guard</code> | <code>ReviewGuard</code> | 在此声明中的实际使用：<code>guard.Submission</code>；<code>guard.Ongoing</code>；<code>guard.Resumable</code> |
| <code>pointer</code> | <code>*ReviewPointer</code> | 在此声明中的实际使用：<code>pointer == nil</code>；<code>pointer.ID</code>；<code>pointer.Document</code>；<code>pointer.Unit</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>pointer.Document != document</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>pointer.Unit != unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;UNIT_REVIEW_OCCUPIED&quot;)</code>；<code>Violation(StateConflict, &quot;RESUBMIT_REQUIRED&quot;)</code>；<code>nil</code>；<code>Violation(StateConflict, &quot;RESUBMIT_POSITION_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>guard.Submission != &quot;&quot; &#124;&#124; guard.Ongoing != 0</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;UNIT_REVIEW_OCCUPIED&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Mode == &quot;NEW&quot;</code>。
- 检查 <code>guard.Resumable != 0</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;RESUBMIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>guard.Resumable != intent.Request &#124;&#124; pointer == nil &#124;&#124; pointer.ID != intent.Request &#124;&#124; pointer.Document != document &#124;&#124; pointer.Unit != unit &#124;&#124; pointer.Branch != intent.Branch &#124;&#124; po …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return Violation(StateConflict, &quot;RESUBMIT_POSITION_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=10078:10919 -->
````go
// 一个单元只能有一个正在审核或可续接的请求；撤回不会变成创建新请求的许可。
func ValidateSubmissionPosition(intent SubmissionIntent, guard ReviewGuard, pointer *ReviewPointer, document, unit uint64) error {
	if guard.Submission != "" || guard.Ongoing != 0 {
		return Violation(StateConflict, "UNIT_REVIEW_OCCUPIED")
	}
	if intent.Mode == "NEW" {
		if guard.Resumable != 0 {
			return Violation(StateConflict, "RESUBMIT_REQUIRED")
		}
		return nil
	}
	if guard.Resumable != intent.Request || pointer == nil || pointer.ID != intent.Request || pointer.Document != document || pointer.Unit != unit || pointer.Branch != intent.Branch || pointer.Status != "WITHDRAWN" || pointer.Archived || pointer.Round != intent.ExpectedRound {
		return Violation(StateConflict, "RESUBMIT_POSITION_CHANGED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/reviews_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
