# 分步编写 `services/document/internal/domain/document/merge_review_models.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/095-merge-review-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。

合并输入必须绑定实际base/submitted/MAIN与完整冲突集合；人工修复不能只挑部分冲突宣称完成。

写之前先按这条执行路线理解各段的连接：

1. ParseMerge/ParseConflictResolution规范输入
2. ValidateMergePosition核轮次与guard
3. ValidateConflictSet核原attempt全部序号
4. MergeInputDigest按canonical-v1生成
5. ValidateFixedContentResources逐项检查正文资源。

本文件需要守住的边界：

- RawMessage须规范化后算指纹以抵御MySQL键重排
- resource_ids自报列表不足以证明真实正文引用。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/merge_review_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/merge_review_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/merge_review_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/merge_review_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:435 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// conflict_resolution.go：保留原实现，按业务合并文件。

````

## 第 2 步：ConflictAttemptRef

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

公共引用编码真实attempt和原冲突数组序号；不改旧私有计算输入中的数据库主键。

本步声明：`func ConflictAttemptRef(attempt uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fmt.Sprintf(&quot;merge-attempt-%020d&quot;, attempt)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>fmt.Sprintf(&quot;merge-attempt-%020d&quot;, attempt)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return fmt.Sprintf(&quot;merge-attempt-%020d&quot;, attempt)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=435:652 -->
````go
// 公共引用编码真实attempt和原冲突数组序号；不改旧私有计算输入中的数据库主键。
func ConflictAttemptRef(attempt uint64) string {
	return fmt.Sprintf("merge-attempt-%020d", attempt)
}

````

## 第 3 步：ConflictItemRef

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func ConflictItemRef(attempt uint64, ordinal int) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fmt.Sprintf(&quot;merge-conflict-%020d-%04d&quot;, attempt, ordinal)</code> |
| <code>ordinal</code> | <code>int</code> | 在此声明中的实际使用：<code>fmt.Sprintf(&quot;merge-conflict-%020d-%04d&quot;, attempt, ordinal)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>fmt.Sprintf(&quot;merge-conflict-%020d-%04d&quot;, attempt, ordinal)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return fmt.Sprintf(&quot;merge-conflict-%020d-%04d&quot;, attempt, ordinal)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=652:781 -->
````go
func ConflictItemRef(attempt uint64, ordinal int) string {
	return fmt.Sprintf("merge-conflict-%020d-%04d", attempt, ordinal)
}

````

## 第 4 步：ConflictResolutionIntent

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type ConflictResolutionIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 这次人工修复原号 |
| <code>Conflict</code> | <code>string</code> | 原冲突操作引用 |
| <code>Attempt</code> | <code>uint64</code> | 原冲突实际计算attempt ID |
| <code>Round</code> | <code>uint64</code> | 原冲突固定轮次 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 用户确认的原冲突固定输入摘要 |
| <code>Body</code> | <code>&#91;&#93;byte</code> | 规范化人工修复正文，不直接JSON重复持久化<br>序列化/校验标记：<code>`json:&quot;-&quot;`</code> |
| <code>Conflicts</code> | <code>&#91;&#93;string</code> | 原attempt完整冲突引用集合 |
| <code>Input</code> | <code>dto.ConflictResolution</code> | 冻结HTTP修复DTO供核指纹 |

<!-- write-step: 4 bytes=781:1020 -->
````go
type ConflictResolutionIntent struct {
	Operation, Conflict string
	Attempt, Round      uint64
	Digest              []byte
	Body                []byte `json:"-"`
	Conflicts           []string
	Input               dto.ConflictResolution
}

````

## 第 5 步：ConflictResolutionFingerprint

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

原HTTP DTO中的RawMessage会被MySQL JSON重排键。正文先按冻结编码规范化，
再参与通用指纹；同一正文的键序不会改变原号，而不同正文仍不能复用原号。

本步声明：`func ConflictResolutionFingerprint(path string, intent ConflictResolutionIntent) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationFingerprint(path, input)</code> |
| <code>intent</code> | <code>ConflictResolutionIntent</code> | 在此声明中的实际使用：<code>intent.Input</code>；<code>intent.Body</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 由 <code>OperationFingerprint(path, input)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&#91;32&#93;byte{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>OperationFingerprint(path, input)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return OperationFingerprint(path, input)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1020:1531 -->
````go
// 原HTTP DTO中的RawMessage会被MySQL JSON重排键。正文先按冻结编码规范化，
// 再参与通用指纹；同一正文的键序不会改变原号，而不同正文仍不能复用原号。
func ConflictResolutionFingerprint(path string, intent ConflictResolutionIntent) ([32]byte, error) {
	input := intent.Input
	input.ResolutionContent = nil
	if err := json.Unmarshal(intent.Body, &input.ResolutionContent); err != nil {
		return [32]byte{}, err
	}
	return OperationFingerprint(path, input)
}

````

## 第 6 步：ParseConflictResolution

插入位置：第 5 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func ParseConflictResolution(conflict string, input dto.ConflictResolution) (ConflictResolutionIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>conflict</code> | <code>string</code> | 在此声明中的实际使用：<code>Conflict: conflict</code>；<code>ValidOperationID(conflict)</code>；<code>value.Operation == conflict</code> |
| <code>input</code> | <code>dto.ConflictResolution</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>Input: input</code>；<code>input.ExpectedAttemptId</code>；<code>input.ExpectedRoundId</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ConflictResolutionIntent</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(value.Operation) &#124;&#124; !ValidOperationID(conflict) &#124;&#124; value.Operation == conflict</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; attempt == 0 &#124;&#124; input.ExpectedAttemptId != ConflictAttemptRef(attempt)</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(value.Digest) != 32 &#124;&#124; hex.EncodeToString(value.Digest) != input.ExpectedInputDigest</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>len(input.ResolvedConflictIds) == 0 &#124;&#124; len(input.ResolvedConflictIds) &gt; 10000</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>e != nil &#124;&#124; ordinal &lt; 1 &#124;&#124; ordinal &gt; 10000 &#124;&#124; reference != ConflictItemRef(attempt, ordinal) &#124;&#124; seen&#91;reference&#93;</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>encoder.Encode(input.ResolutionContent) != nil &#124;&#124; len(input.ResolutionContent) == 0</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(value.Body) &gt; 16&lt;&lt;20</code>。条件成立时执行 <code>return value, invalid</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1531:3562 -->
````go
func ParseConflictResolution(conflict string, input dto.ConflictResolution) (ConflictResolutionIntent, error) {
	value := ConflictResolutionIntent{Operation: input.OperationId, Conflict: conflict, Input: input}
	invalid := Violation(InvalidRequest, "CONFLICT_RESOLUTION_INVALID")
	if !ValidOperationID(value.Operation) || !ValidOperationID(conflict) || value.Operation == conflict {
		return value, invalid
	}
	attempt, err := strconv.ParseUint(strings.TrimPrefix(input.ExpectedAttemptId, "merge-attempt-"), 10, 64)
	if err != nil || attempt == 0 || input.ExpectedAttemptId != ConflictAttemptRef(attempt) {
		return value, invalid
	}
	value.Attempt = attempt
	if value.Round, err = CanonicalID(input.ExpectedRoundId); err != nil {
		return value, invalid
	}
	value.Digest, err = hex.DecodeString(input.ExpectedInputDigest)
	if err != nil || len(value.Digest) != 32 || hex.EncodeToString(value.Digest) != input.ExpectedInputDigest {
		return value, invalid
	}
	if len(input.ResolvedConflictIds) == 0 || len(input.ResolvedConflictIds) > 10000 {
		return value, invalid
	}
	seen := map[string]bool{}
	prefix := fmt.Sprintf("merge-conflict-%020d-", attempt)
	for _, reference := range input.ResolvedConflictIds {
		ordinal, e := strconv.Atoi(strings.TrimPrefix(reference, prefix))
		if e != nil || ordinal < 1 || ordinal > 10000 || reference != ConflictItemRef(attempt, ordinal) || seen[reference] {
			return value, invalid
		}
		seen[reference] = true
		value.Conflicts = append(value.Conflicts, reference)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	// <、>、&是合法正文文本。预算按canonical-v1字节计算，不能先用HTML转义膨胀文本。
	encoder.SetEscapeHTML(false)
	if encoder.Encode(input.ResolutionContent) != nil || len(input.ResolutionContent) == 0 {
		return value, invalid
	}
	body, err := codec.Decode(encoded.Bytes())
	if err != nil {
		return value, invalid
	}
	value.Body, err = codec.Canonical(body)
	if err != nil || len(value.Body) > 16<<20 {
		return value, invalid
	}
````

## 第 7 步：ParseConflictResolution（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 96 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3562:3584 -->
````go
	return value, nil
}

````

## 第 8 步：ConflictResolutionIntent.ValidateConflictSet

插入位置：第 7 步代码之后，紧接追加；完成稿约第 99 行。

当前真实attempt的完整集合由事务传入；语法正确的子集不能被当作已完成全部人工修复。

本步声明：`func (value ConflictResolutionIntent) ValidateConflictSet(count int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>count</code> | <code>int</code> | 在此声明中的实际使用：<code>count &lt; 1</code>；<code>count &gt; 10000</code>；<code>len(value.Conflicts) != count</code>；<code>ordinal &lt;= count</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;CONFLICT_SET_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>count &lt; 1 &#124;&#124; count &gt; 10000 &#124;&#124; len(value.Conflicts) != count</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CONFLICT_SET_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!seen&#91;ConflictItemRef(value.Attempt, ordinal)&#93;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CONFLICT_SET_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3584:4260 -->
````go
// 当前真实attempt的完整集合由事务传入；语法正确的子集不能被当作已完成全部人工修复。
func (value ConflictResolutionIntent) ValidateConflictSet(count int) error {
	if count < 1 || count > 10000 || len(value.Conflicts) != count {
		return Violation(StateConflict, "CONFLICT_SET_CHANGED")
	}
	seen := map[string]bool{}
	for _, reference := range value.Conflicts {
		seen[reference] = true
	}
	for ordinal := 1; ordinal <= count; ordinal++ {
		if !seen[ConflictItemRef(value.Attempt, ordinal)] {
			return Violation(StateConflict, "CONFLICT_SET_CHANGED")
		}
	}
	return nil
}

// discard_draft.go：保留原实现，按业务合并文件。

````

## 第 9 步：DiscardChange

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`type DiscardChange struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 待处理修改意见ID |
| <code>Revision</code> | <code>uint64</code> | 用户看到的意见修订 |
| <code>Reason</code> | <code>string</code> | 丢弃时逐条说明的审计理由 |

<!-- write-step: 9 bytes=4260:4333 -->
````go
type DiscardChange struct {
	ID, Revision uint64
	Reason       string
}

````

## 第 10 步：DiscardDraftIntent

插入位置：第 9 步代码之后，紧接追加；完成稿约第 123 行。

DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。

本步声明：`type DiscardDraftIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 丢弃确认原号 |
| <code>Preview</code> | <code>string</code> | 已固定丢弃预览引用 |
| <code>Branch</code> | <code>uint64</code> | 原支线 |
| <code>Structure</code> | <code>uint64</code> | 期待结构 |
| <code>Round</code> | <code>uint64</code> | 预览绑定审核轮次 |
| <code>Changes</code> | <code>&#91;&#93;DiscardChange</code> | 用户明确处理的意见及理由 |
| <code>Input</code> | <code>dto.DiscardDraft</code> | 完整原确认DTO |

<!-- write-step: 10 bytes=4333:4643 -->
````go
// DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。
type DiscardDraftIntent struct {
	Operation, Preview       string
	Branch, Structure, Round uint64
	Changes                  []DiscardChange
	Input                    dto.DiscardDraft
}

````

## 第 11 步：ParseDiscardDraft

插入位置：第 10 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func ParseDiscardDraft(input dto.DiscardDraft) (DiscardDraftIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.DiscardDraft</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.PreviewCutRef</code>；<code>Input: input</code>；<code>input.ExpectedBranchId</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DiscardDraftIntent</code> | 使用 <code>DiscardDraftIntent</code> 类型；其定义处的职责：DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;DISCARD_REFERENCE_INVALID&quot;)</code>；<code>err</code>；<code>e</code>；<code>Violation(InvalidRequest, &quot;DISCARD_CHANGES_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(result.Operation) &#124;&#124; !ValidOperationID(result.Preview) &#124;&#124; result.Operation == result.Preview</code>。条件成立时执行 <code>return result, Violation(InvalidRequest, &quot;DISCARD_REFERENCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>input.ExpectedRoundId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return result, e</code>。当前函数在这里结束。
- 检查 <code>seen&#91;id&#93; &#124;&#124; !ValidName(item.Reason, 2000)</code>。条件成立时执行 <code>return result, Violation(InvalidRequest, &quot;DISCARD_CHANGES_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4643:5952 -->
````go
func ParseDiscardDraft(input dto.DiscardDraft) (DiscardDraftIntent, error) {
	result := DiscardDraftIntent{Operation: input.OperationId, Preview: input.PreviewCutRef, Input: input}
	if !ValidOperationID(result.Operation) || !ValidOperationID(result.Preview) || result.Operation == result.Preview {
		return result, Violation(InvalidRequest, "DISCARD_REFERENCE_INVALID")
	}
	var err error
	if result.Branch, err = CanonicalID(input.ExpectedBranchId); err != nil {
		return result, err
	}
	if result.Structure, err = CanonicalID(input.ExpectedStructureId); err != nil {
		return result, err
	}
	if input.ExpectedRoundId != nil {
		if result.Round, err = CanonicalID(*input.ExpectedRoundId); err != nil {
			return result, err
		}
	}
	seen := map[uint64]bool{}
	for _, item := range input.ChangesToResolve {
		id, e := CanonicalID(item.Id)
		if e != nil {
			return result, e
		}
		revision, e := CanonicalRevision(item.Revision)
		if e != nil {
			return result, e
		}
		if seen[id] || !ValidName(item.Reason, 2000) {
			return result, Violation(InvalidRequest, "DISCARD_CHANGES_INVALID")
		}
		seen[id] = true
		result.Changes = append(result.Changes, DiscardChange{ID: id, Revision: revision, Reason: item.Reason})
	}
	return result, nil
}

// discard_preview.go：保留原实现，按业务合并文件。

````

## 第 12 步：DiscardPreviewIntent

插入位置：第 11 步代码之后，紧接追加；完成稿约第 169 行。

预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。

本步声明：`type DiscardPreviewIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 预览原号 |
| <code>Branch</code> | <code>uint64</code> | 用户指定支线 |
| <code>Structure</code> | <code>uint64</code> | 用户期待结构 |
| <code>Input</code> | <code>dto.DiscardPreviewCreate</code> | 原预览DTO，epoch/lineage由数据库补足 |

<!-- write-step: 12 bytes=5952:6223 -->
````go
// 预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。
type DiscardPreviewIntent struct {
	Operation         string
	Branch, Structure uint64
	Input             dto.DiscardPreviewCreate
}

````

## 第 13 步：ParseDiscardPreview

插入位置：第 12 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func ParseDiscardPreview(input dto.DiscardPreviewCreate) (DiscardPreviewIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.DiscardPreviewCreate</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>Input: input</code>；<code>input.ExpectedBranchId</code>；<code>input.ExpectedStructureId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DiscardPreviewIntent</code> | 使用 <code>DiscardPreviewIntent</code> 类型；其定义处的职责：预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(result.Operation)</code>。条件成立时执行 <code>return result, Violation(InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=6223:6759 -->
````go
func ParseDiscardPreview(input dto.DiscardPreviewCreate) (DiscardPreviewIntent, error) {
	result := DiscardPreviewIntent{Operation: input.OperationId, Input: input}
	if !ValidOperationID(result.Operation) {
		return result, Violation(InvalidRequest, "OPERATION_ID_INVALID")
	}
	var err error
	if result.Branch, err = CanonicalID(input.ExpectedBranchId); err != nil {
		return result, err
	}
	result.Structure, err = CanonicalID(input.ExpectedStructureId)
	return result, err
}

// merge.go：保留原实现，按业务合并文件。

````

## 第 14 步：MergeResolution

插入位置：第 13 步代码之后，紧接追加；完成稿约第 191 行。

本步声明：`type MergeResolution struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 批准时同时解决的本人修改意见ID<br>序列化/校验标记：<code>`json:&quot;id&quot;`</code> |
| <code>Revision</code> | <code>uint64</code> | 该意见期待修订<br>序列化/校验标记：<code>`json:&quot;revision&quot;`</code> |

<!-- write-step: 14 bytes=6759:6856 -->
````go
type MergeResolution struct {
	ID       uint64 `json:"id"`
	Revision uint64 `json:"revision"`
}

````

## 第 15 步：CommittedSubmissionCut

插入位置：第 14 步代码之后，紧接追加；完成稿约第 196 行。

CommittedSubmissionCut只保存既有成功送审的切点证明，不携带或延长原用户的权限。

本步声明：`type CommittedSubmissionCut struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 已成功送审原号<br>序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Version</code> | <code>uint64</code> | 真实固定送审版本<br>序列化/校验标记：<code>`json:&quot;content_version_id,string&quot;`</code> |
| <code>Cut</code> | <code>&#91;&#93;byte</code> | 已提交可靠捕获切点字节<br>序列化/校验标记：<code>`json:&quot;cut_json_utf8&quot;`</code> |
| <code>BodyHash</code> | <code>&#91;&#93;byte</code> | 该送审正文摘要，不是新权限证明<br>序列化/校验标记：<code>`json:&quot;body_hash&quot;`</code> |

<!-- write-step: 15 bytes=6856:7177 -->
````go
// CommittedSubmissionCut只保存既有成功送审的切点证明，不携带或延长原用户的权限。
type CommittedSubmissionCut struct {
	Operation string `json:"operation_id"`
	Version   uint64 `json:"content_version_id,string"`
	Cut       []byte `json:"cut_json_utf8"`
	BodyHash  []byte `json:"body_hash"`
}

````

## 第 16 步：MergeIntent

插入位置：第 15 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`type MergeIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 批准并合并原号<br>序列化/校验标记：<code>`json:&quot;operation&quot;`</code> |
| <code>Round</code> | <code>uint64</code> | 期待审核轮次 |
| <code>Version</code> | <code>uint64</code> | 该轮次固定送审版本 |
| <code>Resolutions</code> | <code>&#91;&#93;MergeResolution</code> | 本审核者随批准解决的本人意见<br>序列化/校验标记：<code>`json:&quot;resolutions&quot;`</code> |

<!-- write-step: 16 bytes=7177:7326 -->
````go
type MergeIntent struct {
	Operation      string `json:"operation"`
	Round, Version uint64
	Resolutions    []MergeResolution `json:"resolutions"`
}

````

## 第 17 步：MergeFact

插入位置：第 16 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`type MergeFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原合并操作号 |
| <code>State</code> | <code>string</code> | 合并业务阶段 |
| <code>Actor</code> | <code>uint64</code> | 批准者 |
| <code>Request</code> | <code>uint64</code> | 审核请求 |
| <code>Round</code> | <code>uint64</code> | 批准的固定轮次 |
| <code>Version</code> | <code>uint64</code> | 被批准送审版本 |
| <code>Document</code> | <code>uint64</code> | 所属文档 |
| <code>Unit</code> | <code>uint64</code> | 所属单元 |
| <code>Branch</code> | <code>uint64</code> | 原支线 |
| <code>Generation</code> | <code>uint64</code> | 合并相关原代次 |

<!-- write-step: 17 bytes=7326:7482 -->
````go
type MergeFact struct {
	Operation, State                   string
	Actor, Request, Round, Version     uint64
	Document, Unit, Branch, Generation uint64
}

````

## 第 18 步：ParseMerge

插入位置：第 17 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`func ParseMerge(input dto.ApproveMerge) (MergeIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.ApproveMerge</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.OwnChangesToResolve</code>；<code>input.RoundId</code>；<code>input.ContentVersionId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MergeIntent</code> | 实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;MERGE_INPUT_INVALID&quot;)</code>；<code>err</code>；<code>Violation(InvalidRequest, &quot;MERGE_RESOLUTION_DUPLICATED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(input.OperationId) &#124;&#124; input.OwnChangesToResolve == nil &#124;&#124; len(input.OwnChangesToResolve) &gt; 100</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;MERGE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>item.Revision != &quot;0&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>seen&#91;id&#93;</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;MERGE_RESOLUTION_DUPLICATED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return intent, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7482:8660 -->
````go
func ParseMerge(input dto.ApproveMerge) (MergeIntent, error) {
	intent := MergeIntent{Operation: input.OperationId, Resolutions: []MergeResolution{}}
	if !ValidOperationID(input.OperationId) || input.OwnChangesToResolve == nil || len(input.OwnChangesToResolve) > 100 {
		return intent, Violation(InvalidRequest, "MERGE_INPUT_INVALID")
	}
	var err error
	if intent.Round, err = CanonicalID(input.RoundId); err != nil {
		return intent, err
	}
	if intent.Version, err = CanonicalID(input.ContentVersionId); err != nil {
		return intent, err
	}
	seen := map[uint64]bool{}
	for _, item := range input.OwnChangesToResolve {
		id, err := CanonicalID(item.Id)
		if err != nil {
			return intent, err
		}
		// revision是非负预期值，与必须为正的实体ID不同；零随后由意见CAS核对。
		var revision uint64
		if item.Revision != "0" {
			revision, err = CanonicalID(item.Revision)
			if err != nil {
				return intent, err
			}
		}
		if seen[id] {
			return intent, Violation(InvalidRequest, "MERGE_RESOLUTION_DUPLICATED")
		}
		seen[id] = true
		intent.Resolutions = append(intent.Resolutions, MergeResolution{ID: id, Revision: revision})
	}
	return intent, nil
}

````

## 第 19 步：CanApproveMerge

插入位置：第 18 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func CanApproveMerge(permissions Permissions, everSubmitted bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>permissions</code> | <code>Permissions</code> | 使用 <code>Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>permissions.Manage</code>；<code>permissions.Review</code> |
| <code>everSubmitted</code> | <code>bool</code> | 在此声明中的实际使用：<code>!everSubmitted</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>permissions.Manage &#124;&#124; (permissions.Review &amp;&amp; !everSubmitted)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return permissions.Manage &#124;&#124; (permissions.Review &amp;&amp; !everSubmitted)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8660:8805 -->
````go
func CanApproveMerge(permissions Permissions, everSubmitted bool) bool {
	return permissions.Manage || (permissions.Review && !everSubmitted)
}

````

## 第 20 步：ValidateMergePosition

插入位置：第 19 步代码之后，紧接追加；完成稿约第 255 行。

首次登记与最终提交复用同一门槛；计算中不会持请求锁，因此后来撤回或换轮次仍能使提交失效。

本步声明：`func ValidateMergePosition(pointer ReviewPointer, guard ReviewGuard, active string, intent MergeIntent, version uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pointer</code> | <code>ReviewPointer</code> | 使用 <code>ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>ValidateReviewRound(pointer, intent.Round)</code>；<code>pointer.ID</code> |
| <code>guard</code> | <code>ReviewGuard</code> | 使用 <code>ReviewGuard</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>guard.Ongoing</code>；<code>guard.Submission</code> |
| <code>active</code> | <code>string</code> | 在此声明中的实际使用：<code>active != &quot;&quot;</code>；<code>active != intent.Operation</code> |
| <code>intent</code> | <code>MergeIntent</code> | 在此声明中的实际使用：<code>intent.Round</code>；<code>intent.Version</code>；<code>intent.Operation</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>version != intent.Version</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>Violation(StateConflict, &quot;REVIEW_CONTENT_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;UNIT_REVIEW_POSITION_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;MERGE_OPERATION_ACTIVE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>version != intent.Version</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;REVIEW_CONTENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>guard.Ongoing != pointer.ID &#124;&#124; guard.Submission != &quot;&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;UNIT_REVIEW_POSITION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>active != &quot;&quot; &amp;&amp; active != intent.Operation</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;MERGE_OPERATION_ACTIVE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8805:9504 -->
````go
// 首次登记与最终提交复用同一门槛；计算中不会持请求锁，因此后来撤回或换轮次仍能使提交失效。
func ValidateMergePosition(pointer ReviewPointer, guard ReviewGuard, active string, intent MergeIntent, version uint64) error {
	if err := ValidateReviewRound(pointer, intent.Round); err != nil {
		return err
	}
	if version != intent.Version {
		return Violation(StateConflict, "REVIEW_CONTENT_CHANGED")
	}
	if guard.Ongoing != pointer.ID || guard.Submission != "" {
		return Violation(StateConflict, "UNIT_REVIEW_POSITION_CHANGED")
	}
	if active != "" && active != intent.Operation {
		return Violation(StateConflict, "MERGE_OPERATION_ACTIVE")
	}
	return nil
}

````

## 第 21 步：ValidateMergeChanges

插入位置：第 20 步代码之后，紧接追加；完成稿约第 272 行。

本步声明：`func ValidateMergeChanges(actor uint64, changes []ChangeFact, resolutions []MergeResolution) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>change.Author != actor</code> |
| <code>changes</code> | <code>&#91;&#93;ChangeFact</code> | 使用 <code>ChangeFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。 |
| <code>resolutions</code> | <code>&#91;&#93;MergeResolution</code> | 用户声明本次一并解决的修改要求编号及修订；只能解决本人提出且仍OPEN、修订一致的要求，其余OPEN要求继续阻挡合并。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;MERGE_RESOLUTION_DUPLICATED&quot;)</code>；<code>Violation(PermissionDenied, &quot;MERGE_RESOLUTION_NOT_OWN&quot;)</code>；<code>Violation(StateConflict, &quot;CHANGE_REQUEST_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;CHANGES_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>duplicate</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;MERGE_RESOLUTION_DUPLICATED&quot;)</code>。当前函数在这里结束。
- 检查 <code>included</code>。
- 检查 <code>change.Author != actor</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;MERGE_RESOLUTION_NOT_OWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Status != &quot;OPEN&quot; &#124;&#124; change.Revision != revision</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CHANGE_REQUEST_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Status == &quot;OPEN&quot;</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CHANGES_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(wanted) != 0</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;CHANGE_REQUEST_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=9504:10450 -->
````go
func ValidateMergeChanges(actor uint64, changes []ChangeFact, resolutions []MergeResolution) error {
	wanted := map[uint64]uint64{}
	for _, item := range resolutions {
		if _, duplicate := wanted[item.ID]; duplicate {
			return Violation(InvalidRequest, "MERGE_RESOLUTION_DUPLICATED")
		}
		wanted[item.ID] = item.Revision
	}
	for _, change := range changes {
		revision, included := wanted[change.ID]
		if included {
			if change.Author != actor {
				return Violation(PermissionDenied, "MERGE_RESOLUTION_NOT_OWN")
			}
			if change.Status != "OPEN" || change.Revision != revision {
				return Violation(StateConflict, "CHANGE_REQUEST_CHANGED")
			}
			delete(wanted, change.ID)
		} else if change.Status == "OPEN" {
			return Violation(StateConflict, "CHANGES_PENDING")
		}
	}
	if len(wanted) != 0 {
		return Violation(StateConflict, "CHANGE_REQUEST_CHANGED")
	}
	return nil
}

// merge_binding.go：保留原实现，按业务合并文件。

````

## 第 22 步：binaryText

插入位置：第 21 步代码之后，紧接追加；完成稿约第 302 行。

本步声明：`func binaryText(value []byte) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>base64.StdEncoding.EncodeToString(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>base64.StdEncoding.EncodeToString(value)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return base64.StdEncoding.EncodeToString(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=10450:10539 -->
````go
func binaryText(value []byte) string { return base64.StdEncoding.EncodeToString(value) }
````

## 第 23 步：binaryDigest

插入位置：第 22 步代码之后，紧接追加；完成稿约第 303 行。

本步声明：`func binaryDigest(value []byte) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>sha256.Sum256(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>binaryText(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return binaryText(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=10539:10645 -->
````go
func binaryDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return binaryText(digest[:])
}

````

## 第 24 步：FixedMergeBinding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 308 行。

与Node merge_protocol使用相同显式字段和canonical-v1，不依赖proto默认值/Go JSON字段名偶然一致。

本步声明：`func FixedMergeBinding(content *pb.FixedContent) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.ResourceIds</code>；<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.BranchId</code><br>另有 13 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>map&#91;string&#93;any{ 		&quot;document_id&quot;:              TextID(content.DocumentId), 		&quot;unit_id&quot;:                  TextID(content.UnitId), 		&quot;branch_id&quot;:                TextID(content.BranchI …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return map&#91;string&#93;any{ 		&quot;document_id&quot;:              TextID(content.DocumentId), 		&quot;unit_id&quot;:                  TextID(content.UnitId), 		&quot;branch_id&quot;:                TextID(content. …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10645:11966 -->
````go
// 与Node merge_protocol使用相同显式字段和canonical-v1，不依赖proto默认值/Go JSON字段名偶然一致。
func FixedMergeBinding(content *pb.FixedContent) map[string]any {
	resources := make([]any, 0, len(content.ResourceIds))
	for _, id := range content.ResourceIds {
		resources = append(resources, TextID(id))
	}
	return map[string]any{
		"document_id":              TextID(content.DocumentId),
		"unit_id":                  TextID(content.UnitId),
		"branch_id":                TextID(content.BranchId),
		"structure_id":             TextID(content.StructureId),
		"workspace_epoch":          TextID(content.WorkspaceEpoch),
		"capture_id":               content.CaptureId,
		"operation_id":             content.OperationId,
		"lineage_ref":              content.LineageRef,
		"schema_version":           content.SchemaVersion,
		"canonicalization_version": content.CanonicalizationVersion,
		"hash_algorithm":           content.HashAlgorithm,
		"byte_size":                TextID(content.ByteSize),
		"body_hash":                binaryText(content.BodyHash),
		"cut_digest":               binaryDigest(content.CutJsonUtf8),
		"provenance_digest":        binaryDigest(content.ProvenanceJsonUtf8),
		"provenance_schema":        content.ProvenanceSchema,
		"resource_ids":             resources,
	}
}

````

## 第 25 步：CutBinding

插入位置：第 24 步代码之后，紧接追加；完成稿约第 335 行。

本步声明：`func CutBinding(cut *pb.CutVector) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cut</code> | <code>*pb.CutVector</code> | 在此声明中的实际使用：<code>cut.Rooms</code>；<code>cut.DocumentId</code>；<code>cut.StructureId</code>；<code>cut.StyleId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>map&#91;string&#93;any{ 		&quot;document_id&quot;:       TextID(cut.DocumentId), 		&quot;structure_id&quot;:      TextID(cut.StructureId), 		&quot;style_id&quot;:          TextID(cut.StyleId), 		&quot;control_revision&quot;:  Te …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return rooms&#91;i&#93;.RoomId &lt; rooms&#91;j&#93;.RoomId</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return map&#91;string&#93;any{ 		&quot;document_id&quot;:       TextID(cut.DocumentId), 		&quot;structure_id&quot;:      TextID(cut.StructureId), 		&quot;style_id&quot;:          TextID(cut.StyleId), 		&quot;control_revisio …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=11966:12696 -->
````go
func CutBinding(cut *pb.CutVector) map[string]any {
	rooms := append([]*pb.RoomCut(nil), cut.Rooms...)
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	values := make([]any, 0, len(rooms))
	for _, room := range rooms {
		values = append(values, map[string]any{"room_id": TextID(room.RoomId), "workspace_epoch": TextID(room.WorkspaceEpoch), "durable_seq": TextID(room.DurableSeq)})
	}
	return map[string]any{
		"document_id":       TextID(cut.DocumentId),
		"structure_id":      TextID(cut.StructureId),
		"style_id":          TextID(cut.StyleId),
		"control_revision":  TextID(cut.ControlRevision),
		"rooms":             values,
		"dependency_digest": binaryText(cut.DependencyDigest),
	}
}

````

## 第 26 步：MergeInputDigest

插入位置：第 25 步代码之后，紧接追加；完成稿约第 352 行。

本步声明：`func MergeInputDigest(input *pb.MergeCandidateRequest) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>*pb.MergeCandidateRequest</code> | 在此声明中的实际使用：<code>input.Context</code>；<code>input.AttemptId</code>；<code>input.Base</code>；<code>input.Submitted</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>sha256.Sum256(raw)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.ScopeKey &lt; sources&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sha256.Sum256(raw), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=12696:14162 -->
````go
func MergeInputDigest(input *pb.MergeCandidateRequest) ([32]byte, error) {
	context := input.Context
	sources := append([]*pb.SourceEpoch(nil), context.Sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	values := make([]any, 0, len(sources))
	for _, source := range sources {
		values = append(values, map[string]any{"scope_key": source.ScopeKey, "epoch": TextID(source.Epoch)})
	}
	target := context.Target
	binding := map[string]any{
		"attempt_id": input.AttemptId,
		"context": map[string]any{
			"operation_id":           context.OperationId,
			"actor_id":               TextID(context.ActorId),
			"identity_admission_ref": context.IdentityAdmissionRef,
			"document_admission_ref": context.DocumentAdmissionRef,
			"target": map[string]any{
				"document_id":     TextID(target.DocumentId),
				"unit_id":         TextID(target.UnitId),
				"branch_id":       TextID(target.BranchId),
				"structure_id":    TextID(target.StructureId),
				"workspace_epoch": TextID(target.WorkspaceEpoch),
				"purpose":         target.Purpose,
			},
			"sources":             values,
			"request_fingerprint": binaryText(context.RequestFingerprint),
		},
		"base":      FixedMergeBinding(input.Base),
		"submitted": FixedMergeBinding(input.Submitted),
		"main":      FixedMergeBinding(input.Main),
		"main_cut":  CutBinding(input.MainCut),
	}
	raw, err := codec.Canonical(binding)
	return sha256.Sum256(raw), err
}

````

## 第 27 步：MainManifestDigest

插入位置：第 26 步代码之后，紧接追加；完成稿约第 388 行。

本步声明：`func MainManifestDigest(manifest uint64, units []*pb.ManifestUnit) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(manifest)</code> |
| <code>units</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 在此声明中的实际使用：<code>len(units)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>sha256.Sum256(raw)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return sha256.Sum256(raw), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=14162:14687 -->
````go
func MainManifestDigest(manifest uint64, units []*pb.ManifestUnit) ([32]byte, error) {
	values := make([]any, 0, len(units))
	for _, unit := range units {
		values = append(values, map[string]any{"unit_id": TextID(unit.UnitId), "content_version_id": TextID(unit.ContentVersionId), "content": FixedMergeBinding(unit.Content)})
	}
	raw, err := codec.Canonical(map[string]any{"manifest_id": TextID(manifest), "units": values})
	return sha256.Sum256(raw), err
}

// merge_content.go：保留原实现，按业务合并文件。

````

## 第 28 步：ValidateFixedContentResources

插入位置：第 27 步代码之后，紧接追加；完成稿约第 399 行。

MAIN允许独立字体覆盖，但引用仍必须逐项与实际正文一致；不能相信Node仅填resource_ids列表。

本步声明：`func ValidateFixedContentResources(body any, content *pb.FixedContent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>any</code> | 在此声明中的实际使用：<code>root, ok := body.(map&#91;string&#93;any)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.BodyJsonUtf8</code>；<code>content.BodyHash</code>；<code>content.ResourceIds</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(digest&#91;:&#93;, content.BodyHash)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>depth &gt; 64 &#124;&#124; count &gt; 100000</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>blocks&#91;id&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node&#91;&quot;type&quot;&#93; == &quot;image&quot;</code>。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>!ok &#124;&#124; !visit(value, depth+1)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=14687:15951 -->
````go
// MAIN允许独立字体覆盖，但引用仍必须逐项与实际正文一致；不能相信Node仅填resource_ids列表。
func ValidateFixedContentResources(body any, content *pb.FixedContent) error {
	invalid := Violation(TemporarilyUnavailable, "MERGE_FIXED_CONTENT_INVALID")
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) {
		return invalid
	}
	seen, blocks := map[uint64]bool{}, map[string]bool{}
	count := 0
	var visit func(map[string]any, int) bool
	visit = func(node map[string]any, depth int) bool {
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
				seen[id] = true
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				value, ok := child.(map[string]any)
				if !ok || !visit(value, depth+1) {
					return false
				}
			}
		}
		return true
	}
	root, ok := body.(map[string]any)
````

## 第 29 步：ValidateFixedContentResources（接着实现）

插入位置：第 28 步代码之后，紧接追加；完成稿约第 444 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!ok &#124;&#124; !visit(root, 0)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(ids) != len(content.ResourceIds)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>content.ResourceIds&#91;i&#93; != id</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=15951:16385 -->
````go
	if !ok || !visit(root, 0) {
		return invalid
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != len(content.ResourceIds) {
		return invalid
	}
	for i, id := range ids {
		if content.ResourceIds[i] != id {
			return invalid
		}
	}
	return nil
}

// reverse_change.go：保留原实现，按业务合并文件。

````

## 第 30 步：ReverseChangeIntent

插入位置：第 29 步代码之后，紧接追加；完成稿约第 465 行。

反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。

本步声明：`type ReverseChangeIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 新反向更改原号 |
| <code>Reason</code> | <code>string</code> | 反向操作审计理由 |
| <code>Manifest</code> | <code>uint64</code> | 当前期待清单 |
| <code>Merge</code> | <code>uint64</code> | 需反向的不可变合并记录ID |
| <code>Input</code> | <code>dto.ReverseChange</code> | 原反向DTO |

<!-- write-step: 30 bytes=16385:16635 -->
````go
// 反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。
type ReverseChangeIntent struct {
	Operation, Reason string
	Manifest, Merge   uint64
	Input             dto.ReverseChange
}

````

## 第 31 步：ParseReverseChange

插入位置：第 30 步代码之后，紧接追加；完成稿约第 472 行。

本步声明：`func ParseReverseChange(input dto.ReverseChange) (ReverseChangeIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.ReverseChange</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Reason</code>；<code>Input: input</code>；<code>input.ExpectedManifestId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReverseChangeIntent</code> | 使用 <code>ReverseChangeIntent</code> 类型；其定义处的职责：反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;REVERSE_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(value.Operation) &#124;&#124; !ValidName(value.Reason, 2000)</code>。条件成立时执行 <code>return value, Violation(InvalidRequest, &quot;REVERSE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。

<!-- write-step: 31 bytes=16635:17146 -->
````go
func ParseReverseChange(input dto.ReverseChange) (ReverseChangeIntent, error) {
	value := ReverseChangeIntent{Operation: input.OperationId, Reason: input.Reason, Input: input}
	if !ValidOperationID(value.Operation) || !ValidName(value.Reason, 2000) {
		return value, Violation(InvalidRequest, "REVERSE_INPUT_INVALID")
	}
	var err error
	if value.Manifest, err = CanonicalID(input.ExpectedManifestId); err != nil {
		return value, err
	}
	value.Merge, err = CanonicalID(input.MergeRecordId)
	return value, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/merge_review_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
