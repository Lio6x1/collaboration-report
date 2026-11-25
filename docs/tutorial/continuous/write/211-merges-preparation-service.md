# 分步编写 `services/document/internal/logic/document/merges_preparation_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/211-merges-preparation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。

审核针对S1固定送审内容，后来S2草稿不能进入本次计算；稳定目录变更可以投影但范围拆合不可以。

写之前先按这条执行路线理解各段的连接：

1. Prepare核批准资格与意见
2. reserveAttempt固定当前主线
3. fixInputs捕获并算摘要
4. projectMergeVersion只投影稳定身份
5. acceptMergeLineage记录实际接纳来源
6. resume在SQL锁外续期。

本文件需要守住的边界：

- 普通REVIEW用户曾提交过该请求便不能自审；Manage例外仍不豁免固定轮次和其他人的未解决意见
- 不能把拆合范围变化走稳定单元快捷路径
- 成功后的系统安装只核原权威事实。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/merges_preparation_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/merges_preparation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/merges_preparation_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/merges_preparation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:640 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"time"
)

// merge_base_range.go：保留原实现，按业务合并文件。

````

## 第 2 步：projectMergeBase

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

这里只投影计算上下文，不更新旧content_version；拆/合导致的范围变化不能走这个稳定身份快捷路径。

本步声明：`func projectMergeBase(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.Main</code>；<code>attempt.Base</code>；<code>attempt.Round</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 由 <code>projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BASE_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>attempt.Main == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=640:1122 -->
````go
// 这里只投影计算上下文，不更新旧content_version；拆/合导致的范围变化不能走这个稳定身份快捷路径。
func projectMergeBase(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext) (*pb.FixedContent, error) {
	if attempt.Main == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BASE_UNKNOWN")
	}
	return projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)
}

````

## 第 3 步：projectMergeSubmitted

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func projectMergeSubmitted(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext, round uint64) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readSubmissionCut(ctx, tx, round)</code>；<code>projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>readSubmissionCut(ctx, tx, round)</code>；<code>projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.Main</code>；<code>attempt.Submitted</code>；<code>attempt.Round</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>readSubmissionCut(ctx, tx, round)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 由 <code>projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>attempt.Submitted</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMITTED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMITTED_BINDING_UNKNOWN&quot;)</code>；<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>attempt.Main == nil &#124;&#124; attempt.Submitted == nil &#124;&#124; attempt.Main.Context == nil &#124;&#124; attempt.Main.Context.Target == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMITTED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>attempt.Submitted.BranchId != target.BranchId &#124;&#124; attempt.Submitted.WorkspaceEpoch != target.WorkspaceEpoch &#124;&#124; attempt.Submitted.LineageRef != domain.TextID(attempt.Round.Lineage)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMITTED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>attempt.Submitted.StructureId == attempt.Main.ExpectedStructureId</code>。条件成立时执行 <code>return attempt.Submitted, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1122:2114 -->
````go
func projectMergeSubmitted(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext, round uint64) (*pb.FixedContent, error) {
	if attempt.Main == nil || attempt.Submitted == nil || attempt.Main.Context == nil || attempt.Main.Context.Target == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMITTED_UNKNOWN")
	}
	target := attempt.Main.Context.Target
	if attempt.Submitted.BranchId != target.BranchId || attempt.Submitted.WorkspaceEpoch != target.WorkspaceEpoch || attempt.Submitted.LineageRef != domain.TextID(attempt.Round.Lineage) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMITTED_BINDING_UNKNOWN")
	}
	if attempt.Submitted.StructureId == attempt.Main.ExpectedStructureId {
		return attempt.Submitted, nil
	}
	cut, err := readSubmissionCut(ctx, tx, round)
	if err != nil {
		return nil, err
	}
	return projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)
}

````

## 第 4 步：Merges.projectStoredMergeBase

插入位置：第 3 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`func (s *Merges) projectStoredMergeBase(ctx context.Context, task domain.SubmissionTask, fixed mergeFixedInputs, key string) (mergeFixedInputs, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.MergeArtifact(ctx, tx, kind, key)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>fixed</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>fixed.Input</code>；<code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>；<code>return fixed, nil</code>；<code>return fixed, err</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, kind, key)</code>；<code>repository.FixProjectedMergeAttempt(ctx, tx, fixed.Attempt.ID, original, digest&#91;:&#93;, raw, key, task.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 实际返回表达式：<code>fixed</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>；<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_RANGE_INPUTS_UNKNOWN&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Input == nil &#124;&#124; fixed.Input.Base == nil</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fixed.Input.Base.StructureId == fixed.Input.Context.GetTarget().GetStructureId()</code>。条件成立时执行 <code>return fixed, nil</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>readErr == nil</code>。
- 检查 <code>json.Unmarshal(raw, &amp;result) != nil</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_RANGE_INPUTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>readErr != sql.ErrNoRows</code>。条件成立时执行 <code>return fixed, readErr</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(fixed.Input.Base, fixed.Attempt.Base)</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BASE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2114:3784 -->
````go
func (s *Merges) projectStoredMergeBase(ctx context.Context, task domain.SubmissionTask, fixed mergeFixedInputs, key string) (mergeFixedInputs, error) {
	if fixed.Input == nil || fixed.Input.Base == nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
	}
	if fixed.Input.Base.StructureId == fixed.Input.Context.GetTarget().GetStructureId() {
		return fixed, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fixed, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return fixed, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return fixed, err
	}
	const kind = "MERGE_ATTEMPT_RANGE_FIXED"
	if raw, readErr := repository.MergeArtifact(ctx, tx, kind, key); readErr == nil {
		var result mergeFixedInputs
		if json.Unmarshal(raw, &result) != nil {
			return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_INPUTS_UNKNOWN")
		}
		return result, nil
	} else if readErr != sql.ErrNoRows {
		return fixed, readErr
	}
	if !proto.Equal(fixed.Input.Base, fixed.Attempt.Base) {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BASE_BINDING_UNKNOWN")
	}
	original := append([]byte(nil), fixed.Input.InputDigest...)
	projected, err := projectMergeBase(ctx, tx, fixed.Attempt)
	if err != nil {
		return fixed, err
	}
	fixed.Input = proto.Clone(fixed.Input).(*pb.MergeCandidateRequest)
	fixed.Input.Base = projected
	digest, err := domain.MergeInputDigest(fixed.Input)
	if err != nil {
		return fixed, err
	}
	fixed.Input.InputDigest = digest[:]
	raw, err := json.Marshal(fixed)
	if err != nil {
````

## 第 5 步：Merges.projectStoredMergeBase（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 95 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fixed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>bytes.Equal(digest&#91;:&#93;, original)</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_RANGE_DIGEST_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=3784:4434 -->
````go
		return fixed, err
	}
	// 旧生产者的混结构输入在Node注册候选前必然InvalidArgument；只追加带范围证据的派生输入。
	// 原MAIN切点、轮次、字节及MERGE_ATTEMPT_FIXED保持，更新仍须原worker代次和准确旧摘要。
	if bytes.Equal(digest[:], original) {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_DIGEST_UNKNOWN")
	}
	if err = repository.FixProjectedMergeAttempt(ctx, tx, fixed.Attempt.ID, original, digest[:], raw, key, task.Operation); err != nil {
		return fixed, err
	}
	return fixed, tx.Commit()
}

// merge_compute.go：保留原实现，按业务合并文件。

````

## 第 6 步：mergeAttemptContext

插入位置：第 5 步代码之后，紧接追加；完成稿约第 110 行。

本步声明：`type mergeAttemptContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 本次已保留的合并尝试编号。 |
| <code>Generation</code> | <code>uint64</code> | 主线提交屏障的本次代次。 |
| <code>Round</code> | <code>repository.MergeRoundInput</code> | 使用 <code>repository.MergeRoundInput</code> 类型；其定义处的职责：存储合并尝试、固定输入、冲突、版本清单、接纳lineage和安装恢复任务。 |
| <code>Main</code> | <code>*pb.CaptureMainRequest</code> | 该尝试向Node发起的固定主线采样请求。 |
| <code>Base</code> | <code>*pb.FixedContent</code> | 三方合并固定共同基线。 |
| <code>Submitted</code> | <code>*pb.FixedContent</code> | 审核轮次固定送审正文，不取后来编辑的草稿。 |
| <code>SubmissionCut</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。 |

<!-- write-step: 6 bytes=4434:4646 -->
````go
type mergeAttemptContext struct {
	ID, Generation  uint64
	Round           repository.MergeRoundInput
	Main            *pb.CaptureMainRequest
	Base, Submitted *pb.FixedContent
	SubmissionCut   json.RawMessage
}

````

## 第 7 步：mergeFixedInputs

插入位置：第 6 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`type mergeFixedInputs struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Attempt</code> | <code>mergeAttemptContext</code> | 原合并尝试上下文。 |
| <code>Main</code> | <code>*pb.MainCaptureReply</code> | Node固定主线采样结果。 |
| <code>Input</code> | <code>*pb.MergeCandidateRequest</code> | 由基线、提交、主线构成的候选计算请求；固定后原号重试复用。 |

<!-- write-step: 7 bytes=4646:4774 -->
````go
type mergeFixedInputs struct {
	Attempt mergeAttemptContext
	Main    *pb.MainCaptureReply
	Input   *pb.MergeCandidateRequest
}

````

## 第 8 步：attemptKey

插入位置：第 7 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`func attemptKey(operation string, id uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;:&quot;</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>operation + &quot;:&quot; + domain.TextID(id)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return operation + &quot;:&quot; + domain.TextID(id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4774:4874 -->
````go
func attemptKey(operation string, id uint64) string { return operation + ":" + domain.TextID(id) }

````

## 第 9 步：Merges.reserveAttempt

插入位置：第 8 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`func (s *Merges) reserveAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, proofs []*pb.IdentityAdmission) (mergeAttemptContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Pointer</code>；<code>fixed.Actor</code>；<code>fixed.Intent</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMergeProofs(fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeAttemptContext</code> | 实际返回表达式：<code>attempt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_PENDING&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_ATTEMPT_UNKNOWN&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot; &amp;&amp; barrier.Operation != task.Operation</code>。条件成立时执行 <code>return attempt, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; prior.State != &quot;SUPERSEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;attempt) != nil</code>。条件成立时执行 <code>return attempt, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_ATTEMPT_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=4874:6544 -->
````go
func (s *Merges) reserveAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, proofs []*pb.IdentityAdmission) (mergeAttemptContext, error) {
	var attempt mergeAttemptContext
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return attempt, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return attempt, err
	}
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return attempt, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return attempt, err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if barrier.Phase != "OPEN" && barrier.Operation != task.Operation {
		return attempt, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_PENDING")
	}
	if _, err = validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true); err != nil {
		return attempt, err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return attempt, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return attempt, err
	}
	prior, err := repository.LatestMergeAttempt(ctx, tx, task.Operation)
	if err != nil && err != sql.ErrNoRows {
		return attempt, err
	}
	if err == nil && prior.State != "SUPERSEDED" {
		raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_CONTEXT", attemptKey(task.Operation, prior.ID))
		if err != nil {
			return attempt, err
		}
		if json.Unmarshal(raw, &attempt) != nil {
			return attempt, domain.Violation(domain.TemporarilyUnavailable, "MERGE_ATTEMPT_UNKNOWN")
		}
````

## 第 10 步：Merges.reserveAttempt（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 171 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return attempt, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 检查 <code>round.SubmittedVersion != fixed.Intent.Version</code>。条件成立时执行 <code>return attempt, domain.Violation(domain.StateConflict, &quot;REVIEW_CONTENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>round.Structure != structure</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=6544:8439 -->
````go
		if err = tx.Commit(); err != nil {
			return attempt, err
		}
		return attempt, nil
	}
	round, err := repository.MergeRoundInputs(ctx, tx, fixed.Pointer.ID, fixed.Intent.Round)
	if err != nil {
		return attempt, err
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	structure, _ := domain.CanonicalID(current.CurrentStructureId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalID(current.ControlRevision)
	if round.SubmittedVersion != fixed.Intent.Version {
		return attempt, domain.Violation(domain.StateConflict, "REVIEW_CONTENT_CHANGED")
	}
	if round.Structure != structure {
		if err = requireStableReviewRange(ctx, tx, fixed.Pointer.Document, fixed.Pointer.Unit, round.Structure, structure); err != nil {
			return attempt, err
		}
	}
	base, err := repository.VersionContent(ctx, tx, round.BaseVersion, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if err = canonicalVersion(base, round.BaseVersion); err != nil {
		return attempt, err
	}
	submitted, err := repository.VersionContent(ctx, tx, round.SubmittedVersion, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if err = canonicalVersion(submitted, round.SubmittedVersion); err != nil {
		return attempt, err
	}
	submitted.BranchId, submitted.WorkspaceEpoch, submitted.LineageRef = fixed.Pointer.Branch, fixed.Context.Target.WorkspaceEpoch, domain.TextID(round.Lineage)
	cut, err := readSubmissionCut(ctx, tx, fixed.Intent.Round)
	if err != nil {
		return attempt, err
	}
	// submitted的切点是原送审确认；不能把S2当前尾部或合并后的MAIN切点替代它。
	submitted.CutJsonUtf8 = cut
	generation := barrier.Generation + 1
	id, err := repository.ReserveMergeAttempt(ctx, tx, fixed.Intent, fixed.Pointer.Document, prior.Number+1, manifest, structure, style, round.Lineage, round.BaseVersion, generation)
	if err != nil {
````

## 第 11 步：Merges.reserveAttempt（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 216 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return attempt, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, err</code>。当前函数在这里结束。
- 此处通过 <code>return attempt, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 11 bytes=8439:9112 -->
````go
		return attempt, err
	}
	attempt = mergeAttemptContext{ID: id, Generation: generation, Round: round, Base: base, Submitted: submitted, SubmissionCut: cut, Main: &pb.CaptureMainRequest{Context: fixed.Context, ExpectedManifestId: manifest, ExpectedControlRevision: control, ExpectedStructureId: structure, ExpectedStyleId: style, Usage: "MERGE_ATTEMPT:" + domain.TextID(id)}}
	payload, err := json.Marshal(attempt)
	if err != nil {
		return attempt, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_CONTEXT", attemptKey(task.Operation, id), task.Operation, payload, "SUCCEEDED"); err != nil {
		return attempt, err
	}
	return attempt, tx.Commit()
}

````

## 第 12 步：Merges.fixInputs

插入位置：第 11 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func (s *Merges) fixInputs(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) (mergeFixedInputs, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;MERGE_ATTEMPT_FIXED&quot;, key)</code>；<code>s.projectStoredMergeBase(ctx, task, result, key)</code>；<code>s.client.CaptureMain(ctx, attempt.Main)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>s.projectStoredMergeBase(ctx, task, result, key)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Pointer</code>；<code>fixed.Context</code>；<code>fixed.Intent</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.ID</code>；<code>attempt.Main</code>；<code>attempt.Base</code>；<code>attempt.Submitted</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 由 <code>s.projectStoredMergeBase(ctx, task, result, key)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.projectStoredMergeBase(ctx, task, result, key)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_INVALID&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(raw, &amp;result) != nil</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.projectStoredMergeBase(ctx, task, result, key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>main == nil &#124;&#124; main.State != pb.CaptureReply_FIXED &#124;&#124; main.Cut == nil &#124;&#124; main.Cut.DocumentId != fixed.Pointer.Document &#124;&#124; main.Cut.StructureId != attempt.Main.ExpectedStructureId &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>content == nil &#124;&#124; seen&#91;content.UnitId&#93;</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>content.BranchId != 0 &#124;&#124; content.WorkspaceEpoch != attempt.Main.ExpectedControlRevision &#124;&#124; content.OperationId != task.Operation</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>content.UnitId == fixed.Pointer.Unit</code>。
- 检查 <code>selected == nil</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNIT_MISSING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=9112:11355 -->
````go
func (s *Merges) fixInputs(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) (mergeFixedInputs, error) {
	var result mergeFixedInputs
	key := attemptKey(task.Operation, attempt.ID)
	raw, err := repository.MergeArtifact(ctx, s.db, "MERGE_ATTEMPT_FIXED", key)
	if err == nil {
		if json.Unmarshal(raw, &result) != nil {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
		}
		return s.projectStoredMergeBase(ctx, task, result, key)
	}
	if err != sql.ErrNoRows {
		return result, err
	}
	main, err := s.client.CaptureMain(ctx, attempt.Main)
	if err != nil {
		return result, err
	}
	if main == nil || main.State != pb.CaptureReply_FIXED || main.Cut == nil || main.Cut.DocumentId != fixed.Pointer.Document || main.Cut.StructureId != attempt.Main.ExpectedStructureId || main.Cut.StyleId != attempt.Main.ExpectedStyleId || main.Cut.ControlRevision != attempt.Main.ExpectedControlRevision {
		return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
	}
	if err = validateMainCaptureDigest(main); err != nil {
		return result, err
	}
	var selected *pb.FixedContent
	seen := map[uint64]bool{}
	for _, content := range main.Units {
		if content == nil || seen[content.UnitId] {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_INVALID")
		}
		seen[content.UnitId] = true
		if content.BranchId != 0 || content.WorkspaceEpoch != attempt.Main.ExpectedControlRevision || content.OperationId != task.Operation {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Pointer.Document, content.UnitId, attempt.Main.ExpectedStructureId); err != nil {
			return result, err
		}
		if content.UnitId == fixed.Pointer.Unit {
			selected = content
		}
	}
	if selected == nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNIT_MISSING")
	}
	input := &pb.MergeCandidateRequest{Context: fixed.Context, AttemptId: domain.TextID(attempt.ID), Base: attempt.Base, Submitted: attempt.Submitted, Main: selected, MainCut: main.Cut}
	digest, err := domain.MergeInputDigest(input)
````

## 第 13 步：Merges.fixInputs（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 274 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(known, &amp;result) != nil</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_INPUTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=11355:12618 -->
````go
	if err != nil {
		return result, err
	}
	input.InputDigest = digest[:]
	result = mergeFixedInputs{Attempt: attempt, Main: main, Input: input}
	payload, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return result, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return result, err
	}
	base, err := projectMergeBase(ctx, tx, attempt)
	if err != nil {
		return result, err
	}
	input.Base = base
	input.Submitted, err = projectMergeSubmitted(ctx, tx, attempt, fixed.Intent.Round)
	if err != nil {
		return result, err
	}
	digest, err = domain.MergeInputDigest(input)
	if err != nil {
		return result, err
	}
	input.InputDigest = digest[:]
	result.Input = input
	payload, err = json.Marshal(result)
	if err != nil {
		return result, err
	}
	if known, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key); err == nil {
		if json.Unmarshal(known, &result) != nil {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
		}
	} else if err != sql.ErrNoRows {
		return result, err
	} else {
````

## 第 14 步：Merges.fixInputs（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 320 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 14 bytes=12618:12869 -->
````go
		if err = repository.FixMergeAttempt(ctx, tx, attempt.ID, digest[:], payload, key, task.Operation); err != nil {
			return result, err
		}
	}
	return result, tx.Commit()
}

// merge_content_validation.go：保留原实现，按业务合并文件。

````

## 第 15 步：validateMainCaptureDigest

插入位置：第 14 步代码之后，紧接追加；完成稿约第 329 行。

跨服务固定字节在事务外校验，未知摘要或不完整正文不能进入正式主线写入。

本步声明：`func validateMainCaptureDigest(main *pb.MainCaptureReply) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>main</code> | <code>*pb.MainCaptureReply</code> | 在此声明中的实际使用：<code>main.Cut</code>；<code>main.Units</code>；<code>main.ResultDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_DEPENDENCY_DIGEST_INVALID&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_INVALID&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_DIGEST_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(dependencies&#91;:&#93;, main.Cut.DependencyDigest)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_DEPENDENCY_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>unit == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest&#91;:&#93;, main.ResultDigest)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=12869:13926 -->
````go
// 跨服务固定字节在事务外校验，未知摘要或不完整正文不能进入正式主线写入。
func validateMainCaptureDigest(main *pb.MainCaptureReply) error {
	cut := domain.CutBinding(main.Cut)
	rooms, err := codec.Canonical(cut["rooms"])
	if err != nil {
		return err
	}
	dependencies := sha256.Sum256(rooms)
	if !bytes.Equal(dependencies[:], main.Cut.DependencyDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_DEPENDENCY_DIGEST_INVALID")
	}
	units := make([]any, 0, len(main.Units))
	for _, unit := range main.Units {
		if unit == nil {
			return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_INVALID")
		}
		wire, err := fixedWire(unit)
		if err != nil {
			return err
		}
		units = append(units, wire)
	}
	raw, err := codec.Canonical(map[string]any{"cut": cut, "units": units})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if !bytes.Equal(digest[:], main.ResultDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_DIGEST_INVALID")
	}
	return nil
}

````

## 第 16 步：validateFixedMergeContent

插入位置：第 15 步代码之后，紧接追加；完成稿约第 362 行。

本步声明：`func validateFixedMergeContent(validator *codec.Validator, content *pb.FixedContent, document, unit, structure uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator.Body</code>；<code>validator.Provenance</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content == nil</code>；<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.StructureId</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.DocumentId != document</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.UnitId != unit</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.StructureId != structure</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>content == nil &#124;&#124; content.DocumentId != document &#124;&#124; content.UnitId != unit &#124;&#124; content.StructureId != structure &#124;&#124; content.SchemaVersion != &quot;content-v1&quot; &#124;&#124; content.CanonicalizationV …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Body(body) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, content.BodyJsonUtf8)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Provenance(provenance) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, content.ProvenanceJsonUtf8)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=13926:15251 -->
````go
func validateFixedMergeContent(validator *codec.Validator, content *pb.FixedContent, document, unit, structure uint64) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MERGE_FIXED_CONTENT_INVALID")
	if content == nil || content.DocumentId != document || content.UnitId != unit || content.StructureId != structure || content.SchemaVersion != "content-v1" || content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" || content.ProvenanceSchema != "provenance-v1" || len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
		return invalid
	}
	body, err := codec.Decode(content.BodyJsonUtf8)
	if err != nil || validator.Body(body) != nil {
		return invalid
	}
	canonical, err := codec.Canonical(body)
	if err != nil || !bytes.Equal(canonical, content.BodyJsonUtf8) {
		return invalid
	}
	provenance, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil || validator.Provenance(provenance) != nil {
		return invalid
	}
	canonical, err = codec.Canonical(provenance)
	if err != nil || !bytes.Equal(canonical, content.ProvenanceJsonUtf8) {
		return invalid
	}
	if err = domain.ValidateFixedContentResources(body, content); err != nil {
		return err
	}
	return nil
}

// merge_lineage.go：保留原实现，按业务合并文件。

````

## 第 17 步：validateMergeCandidateProvenance

插入位置：第 16 步代码之后，紧接追加；完成稿约第 391 行。

本步声明：`func validateMergeCandidateProvenance(content *pb.FixedContent, attempt mergeAttemptContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.ProvenanceJsonUtf8</code>；<code>content.OperationId</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.Submitted</code>；<code>attempt.Round</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_PROVENANCE_BINDING_INVALID&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_ACCEPTED_CAPTURES_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>provenance&#91;&quot;lineage_id&quot;&#93; != domain.TextID(attempt.Round.Lineage) &#124;&#124; provenance&#91;&quot;base_manifest_id&quot;&#93; != domain.TextID(attempt.Round.BaseManifest) &#124;&#124; provenance&#91;&quot;base_content_version_ …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_PROVENANCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(accepted, before)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_ACCEPTED_CAPTURES_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=15251:16313 -->
````go
func validateMergeCandidateProvenance(content *pb.FixedContent, attempt mergeAttemptContext) error {
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	original, err := codec.Decode(attempt.Submitted.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := value.(map[string]any)
	if provenance["lineage_id"] != domain.TextID(attempt.Round.Lineage) || provenance["base_manifest_id"] != domain.TextID(attempt.Round.BaseManifest) || provenance["base_content_version_id"] != domain.TextID(attempt.Round.BaseVersion) || provenance["source_operation_id"] != content.OperationId {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_PROVENANCE_BINDING_INVALID")
	}
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return err
	}
	before, err := codec.Canonical(original.(map[string]any)["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, before) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_ACCEPTED_CAPTURES_CHANGED")
	}
	return nil
}

````

## 第 18 步：Merges.acceptMergeLineage

插入位置：第 17 步代码之后，紧接追加；完成稿约第 415 行。

本步声明：`func (s *Merges) acceptMergeLineage(ctx context.Context, tx *sql.Tx, fixed mergeContext, inputs mergeFixedInputs, workspace domain.EditWorkspace, record, manifest uint64) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.BranchBootstrapLineage(ctx, tx, fixed.Pointer.Branch)</code>；<code>repository.InsertAcceptedLineage(ctx, tx, id, fixed.Pointer, workspace.Lineage, manifest, fixed.Intent.Version, fixed.Intent.Operation, inputs.Attempt.Round.Capture, raw)</code>；<code>collaborationRoom(ctx, tx, &quot;BRANCH&quot;, fixed.Pointer.Branch, fixed.Pointer.Unit, workspace.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.BranchBootstrapLineage(ctx, tx, fixed.Pointer.Branch)</code>；<code>repository.InsertAcceptedLineage(ctx, tx, id, fixed.Pointer, workspace.Lineage, manifest, fixed.Intent.Version, fixed.Intent.Operation, inputs.Attempt.Round.Capture, raw)</code>；<code>collaborationRoom(ctx, tx, &quot;BRANCH&quot;, fixed.Pointer.Branch, fixed.Pointer.Unit, workspace.Epoch)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Pointer</code>；<code>fixed.Intent</code> |
| <code>inputs</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>inputs.Attempt</code> |
| <code>workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>在此声明中的实际使用：<code>workspace.Lineage</code>；<code>workspace.Epoch</code> |
| <code>record</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(record)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(manifest)</code>；<code>repository.InsertAcceptedLineage(ctx, tx, id, fixed.Pointer, workspace.Lineage, manifest, fixed.Intent.Version, fixed.Intent.Operation, inputs.Attempt.Round.Capture, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;room_id&quot;: domain.TextID(room), &quot;epoch&quot;: domain.TextID(workspace.Epoch), &quot;previous_lineage_id&quot;: domain.TextID(workspace.Lineage), &quot;context&quot;: map&#91;string&#93;any{&quot;lineage_ …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;LINEAGE_ACCEPTED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;LINEAGE_ACCEPTED_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;LINEAGE_ACCEPTED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(inputs.Attempt.SubmissionCut, &amp;cut) != nil &#124;&#124; cut&#91;&quot;room_id&quot;&#93; == &quot;&quot; &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; == &quot;&quot; &#124;&#124; cut&#91;&quot;durable_seq&quot;&#93; == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;LINEAGE_ACCEPTED_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=16313:18705 -->
````go
func (s *Merges) acceptMergeLineage(ctx context.Context, tx *sql.Tx, fixed mergeContext, inputs mergeFixedInputs, workspace domain.EditWorkspace, record, manifest uint64) (map[string]any, error) {
	id, err := NewEntityID()
	if err != nil {
		return nil, err
	}
	value, err := codec.Decode(inputs.Attempt.Submitted.ProvenanceJsonUtf8)
	if err != nil {
		return nil, err
	}
	provenance := value.(map[string]any)
	_, _, previousRaw, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Pointer.Branch)
	if err != nil {
		return nil, err
	}
	previous, err := codec.Decode(previousRaw)
	if err != nil {
		return nil, err
	}
	accepted, ok := previous.(map[string]any)["accepted_captures"].([]any)
	if !ok {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LINEAGE_ACCEPTED_UNKNOWN")
	}
	var cut map[string]string
	if json.Unmarshal(inputs.Attempt.SubmissionCut, &cut) != nil || cut["room_id"] == "" || cut["workspace_epoch"] == "" || cut["durable_seq"] == "" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	provenance["lineage_id"] = domain.TextID(id)
	provenance["base_manifest_id"] = domain.TextID(manifest)
	// S1是已接纳的共同草稿基线；未拉入的MAIN兼容新增不能在下一次S2送审时被误判成删除。
	provenance["base_content_version_id"] = domain.TextID(fixed.Intent.Version)
	provenance["source_operation_id"] = fixed.Intent.Operation
	provenance["accepted_captures"] = append(append([]any{}, accepted...), map[string]any{"capture_id": inputs.Attempt.Round.Capture, "submitted_content_version_id": domain.TextID(fixed.Intent.Version), "merge_record_id": domain.TextID(record), "cut": []any{map[string]any{"room_id": cut["room_id"], "epoch": cut["workspace_epoch"], "durable_seq": cut["durable_seq"]}}})
	if err = s.validator.Provenance(provenance); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LINEAGE_ACCEPTED_INVALID")
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	if err = repository.InsertAcceptedLineage(ctx, tx, id, fixed.Pointer, workspace.Lineage, manifest, fixed.Intent.Version, fixed.Intent.Operation, inputs.Attempt.Round.Capture, raw); err != nil {
		return nil, err
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", fixed.Pointer.Branch, fixed.Pointer.Unit, workspace.Epoch)
	if err != nil {
		return nil, err
	}
````

## 第 19 步：Merges.acceptMergeLineage（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 461 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;any{&quot;room_id&quot;: domain.TextID(room), &quot;epoch&quot;: domain.TextID(workspace.Epoch), &quot;previous_lineage_id&quot;: domain.TextID(workspace.Lineage), &quot;context&quot;: map&#91;string&#93;any{&quot;l …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=18705:19241 -->
````go
	body, err := codec.Decode(inputs.Attempt.Submitted.BodyJsonUtf8)
	if err != nil {
		return nil, err
	}
	return map[string]any{"room_id": domain.TextID(room), "epoch": domain.TextID(workspace.Epoch), "previous_lineage_id": domain.TextID(workspace.Lineage), "context": map[string]any{"lineage_id": domain.TextID(id), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(fixed.Intent.Version), "body": body, "provenance": provenance}}, nil
}

// merge_main_source.go：保留原实现，按业务合并文件。

````

## 第 20 步：Merges.fixedContext

插入位置：第 19 步代码之后，紧接追加；完成稿约第 470 行。

本步声明：`func (s *Merges) fixedContext(ctx context.Context, operation string) (mergeContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergePayload(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergePayload(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; len(fixed.Access.Proofs) == 0</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=19241:19708 -->
````go
func (s *Merges) fixedContext(ctx context.Context, operation string) (mergeContext, error) {
	var fixed mergeContext
	payload, err := repository.MergePayload(ctx, s.db, operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(payload, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || len(fixed.Access.Proofs) == 0 {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
	}
	return fixed, nil
}

````

## 第 21 步：Merges.resume

插入位置：第 20 步代码之后，紧接追加；完成稿约第 482 行。

短期证明续接在SQL锁外执行；所有最终写入仍须检查原会话期限、来源和当前文档事实。

本步声明：`func (s *Merges) resume(ctx context.Context, fixed mergeContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=19708:20327 -->
````go
// 短期证明续接在SQL锁外执行；所有最终写入仍须检查原会话期限、来源和当前文档事实。
func (s *Merges) resume(ctx context.Context, fixed mergeContext) ([]*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

````

## 第 22 步：validateMergeProofs

插入位置：第 21 步代码之后，紧接追加；完成稿约第 498 行。

本步声明：`func validateMergeProofs(fixed mergeContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.SessionExpires</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>len(proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(proofs) != len(fixed.Access.Proofs) &#124;&#124; fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=20327:20797 -->
````go
func validateMergeProofs(fixed mergeContext, proofs []*pb.IdentityAdmission) error {
	if len(proofs) != len(fixed.Access.Proofs) || fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

````

## 第 23 步：Merges.MainSource

插入位置：第 22 步代码之后，紧接追加；完成稿约第 510 行。

仅向协作服务提供已登记MERGE原号的全篇固定清单；不能让mTLS调用方任意指定正文或版本。

本步声明：`func (s *Merges) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.CaptureMainRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>input.ExpectedManifestId</code>；<code>input.ExpectedStructureId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlRevision …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil &#124;&#124; input.Context.Target == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(fixed.Context, input.Context)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) &#124;&#124; current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) &#124;&#124; current.CurrentStyleId != domain.T …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlR …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=20797:22833 -->
````go
// 仅向协作服务提供已登记MERGE原号的全篇固定清单；不能让mTLS调用方任意指定正文或版本。
func (s *Merges) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return nil, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return nil, err
	}
	if current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) || current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) || current.CurrentStyleId != domain.TextID(input.ExpectedStyleId) || current.ControlRevision != domain.TextID(input.ExpectedControlRevision) {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	if _, err = validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true); err != nil {
		return nil, err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Pointer.Document, current.CurrentManifestId, input.ExpectedControlRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

````

## 第 24 步：canonicalVersion

插入位置：第 23 步代码之后，紧接追加；完成稿约第 557 行。

本步声明：`func canonicalVersion(content *pb.FixedContent, version uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.BodyJsonUtf8</code>；<code>content.BodyHash</code>；<code>content.ByteSize</code>；<code>content.ProvenanceJsonUtf8</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_VERSION_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_PROVENANCE_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(digest&#91;:&#93;, content.BodyHash) &#124;&#124; content.ByteSize != uint64(len(content.BodyJsonUtf8)) &#124;&#124; len(content.BodyJsonUtf8) &gt; 16&lt;&lt;20</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_VERSION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_PROVENANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=22833:23713 -->
````go
func canonicalVersion(content *pb.FixedContent, version uint64) error {
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) || content.ByteSize != uint64(len(content.BodyJsonUtf8)) || len(content.BodyJsonUtf8) > 16<<20 {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_VERSION_INVALID")
	}
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	content.ProvenanceJsonUtf8, err = codec.Canonical(value)
	if err != nil {
		return err
	}
	provenance, ok := value.(map[string]any)
	if !ok {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_PROVENANCE_INVALID")
	}
	content.LineageRef, _ = provenance["lineage_id"].(string)
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"content_version_id": domain.TextID(version), "source_ref": content.CaptureId})
	return err
}

````

## 第 25 步：fixedWire

插入位置：第 24 步代码之后，紧接追加；完成稿约第 579 行。

本步声明：`func fixedWire(content *pb.FixedContent) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(content)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=23713:24066 -->
````go
func fixedWire(content *pb.FixedContent) (map[string]any, error) {
	raw, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(content)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	err = json.Unmarshal(raw, &value)
	return value, err
}

// merge_prepare.go：保留原实现，按业务合并文件。

````

## 第 26 步：Merges.Prepare

插入位置：第 25 步代码之后，紧接追加；完成稿约第 591 行。

本步声明：`func (s *Merges) Prepare(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.ApproveMerge, path string) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, pointer.Document, &quot;MERGE&quot;)</code>；<code>repository.MergeRow(ctx, s.db, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, pointer.Branch, pointer.Document, pointer.Unit)</code>；<code>s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, intent.Operation, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>s.access.Authorize(ctx, actor, pointer.Document, &quot;MERGE&quot;)</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>actor.Sources</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Document</code>；<code>reviewContextAllowed(access, pointer)</code>；<code>s.mergePresentation(actor, access, pointer, intent.Operation)</code>；<code>pointer.Branch</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.ApproveMerge</code> | 在此声明中的实际使用：<code>domain.ParseMerge(input)</code>；<code>domain.OperationFingerprint(path, input)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!reviewContextAllowed(access, pointer)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, present, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(&amp;pb.OperationContext{Sources: proof.Sources}, &amp;pb.OperationContext{Sources: bound.Sources})</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 26 bytes=24066:26299 -->
````go
func (s *Merges) Prepare(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.ApproveMerge, path string) (Actor, MutationBody, MutationPresentation, error) {
	intent, err := domain.ParseMerge(input)
	if err != nil {
		return actor, nil, nil, err
	}
	access, err := s.access.Authorize(ctx, actor, pointer.Document, "MERGE")
	if err != nil {
		return actor, nil, nil, err
	}
	if !reviewContextAllowed(access, pointer) {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	present := s.mergePresentation(actor, access, pointer, intent.Operation)
	if _, err = repository.MergeRow(ctx, s.db, intent.Operation); err == nil {
		// 回执复用由通用事务执行，不能因现在的轮次/资格改变再次写一次批准。
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, pointer.Branch, pointer.Document, pointer.Unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, nil, nil, err
	}
	target := &pb.Target{DocumentId: pointer.Document, UnitId: pointer.Unit, BranchId: pointer.Branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "MERGE"}
	proofs := access.Proofs
	access.Proofs = make([]*pb.IdentityAdmission, 0, len(proofs))
	for _, proof := range proofs {
		bound, err := s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, intent.Operation, target)
		if err != nil {
			return actor, nil, nil, err
		}
		if !proto.Equal(&pb.OperationContext{Sources: proof.Sources}, &pb.OperationContext{Sources: bound.Sources}) {
			return actor, nil, nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		access.Proofs = append(access.Proofs, bound)
	}
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
````

## 第 27 步：Merges.Prepare（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 636 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return mergePending(intent.Operation), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=26299:27730 -->
````go
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, err := validateMergeReview(ctx, tx, actor.ID, access, pointer, intent, false)
		if err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, pointer.Document, "MERGE", admission); err != nil {
			return nil, err
		}
		targetJSON, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, pointer.Unit, intent.Operation, targetJSON, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		binding := &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
		payload, err := json.Marshal(mergeContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Pointer: current, Intent: intent, Access: access, Context: binding})
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterMerge(ctx, tx, actor.ID, current.ID, intent, payload); err != nil {
			return nil, err
		}
		return mergePending(intent.Operation), nil
	}
	return actor, run, present, nil
}

````

## 第 28 步：mergePending

插入位置：第 27 步代码之后，紧接追加；完成稿约第 671 行。

本步声明：`func mergePending(operation string) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: operation</code>；<code>&quot;/v1/merge-operations/&quot; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/merge-operations/&quot; + operation}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/merge-operations/&quot; + operation}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=27730:27907 -->
````go
func mergePending(operation string) map[string]any {
	return map[string]any{"operation_id": operation, "state": "PREPARING", "poll_url": "/v1/merge-operations/" + operation}
}

````

## 第 29 步：validateMergeReview

插入位置：第 28 步代码之后，紧接追加；完成稿约第 675 行。

本步声明：`func validateMergeReview(ctx context.Context, tx *sql.Tx, actor uint64, access DocumentAccess, pointer domain.ReviewPointer, intent domain.MergeIntent, reserved bool) (domain.ReviewPointer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, pointer.Unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, pointer.ID)</code>；<code>repository.LockedActiveMerge(ctx, tx, current.ID)</code>；<code>repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, pointer.Unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, pointer.ID)</code>；<code>repository.LockedActiveMerge(ctx, tx, current.ID)</code>；<code>repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.EverSubmitted(ctx, tx, current.ID, actor)</code>；<code>domain.ValidateMergeChanges(actor, changes, intent.Resolutions)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Unit</code>；<code>return pointer, err</code>；<code>pointer.ID</code> |
| <code>intent</code> | <code>domain.MergeIntent</code> | 使用 <code>domain.MergeIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>domain.ValidateMergePosition(current, guard, active, intent, version)</code>；<code>intent.Operation</code>；<code>intent.Resolutions</code> |
| <code>reserved</code> | <code>bool</code> | 在此声明中的实际使用：<code>reserved &amp;&amp; active != intent.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>pointer</code>；<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MERGE_RESERVATION_CHANGED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;MERGE_APPROVAL_NOT_ALLOWED&quot;)</code>；<code>domain.ValidateMergeChanges(actor, changes, intent.Resolutions)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return pointer, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>reserved &amp;&amp; active != intent.Operation</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MERGE_RESERVATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>!domain.CanApproveMerge(access.Permissions, ever)</code>。条件成立时执行 <code>return current, domain.Violation(domain.PermissionDenied, &quot;MERGE_APPROVAL_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 此处通过 <code>return current, domain.ValidateMergeChanges(actor, changes, intent.Resolutions)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=27907:29333 -->
````go
func validateMergeReview(ctx context.Context, tx *sql.Tx, actor uint64, access DocumentAccess, pointer domain.ReviewPointer, intent domain.MergeIntent, reserved bool) (domain.ReviewPointer, error) {
	guard, err := repository.LockedReviewGuard(ctx, tx, pointer.Unit)
	if err != nil {
		return pointer, err
	}
	current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
	if err != nil {
		return current, err
	}
	active, err := repository.LockedActiveMerge(ctx, tx, current.ID)
	if err != nil {
		return current, err
	}
	version, err := repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)
	if err != nil {
		return current, err
	}
	if err = domain.ValidateMergePosition(current, guard, active, intent, version); err != nil {
		return current, err
	}
	if reserved && active != intent.Operation {
		return current, domain.Violation(domain.StateConflict, "MERGE_RESERVATION_CHANGED")
	}
	ever, err := repository.EverSubmitted(ctx, tx, current.ID, actor)
	if err != nil {
		return current, err
	}
	if !domain.CanApproveMerge(access.Permissions, ever) {
		return current, domain.Violation(domain.PermissionDenied, "MERGE_APPROVAL_NOT_ALLOWED")
	}
	changes, err := repository.LockedReviewChanges(ctx, tx, current.ID)
	if err != nil {
		return current, err
	}
	return current, domain.ValidateMergeChanges(actor, changes, intent.Resolutions)
}

// merge_range_cut.go：保留原实现，按业务合并文件。

````

## 第 30 步：validProjectedSubmissionCut

插入位置：第 29 步代码之后，紧接追加；完成稿约第 714 行。

本步声明：`func validProjectedSubmissionCut(projected []byte, attempt mergeAttemptContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>projected</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.Equal(projected, original)</code>；<code>json.Unmarshal(projected, &amp;cut)</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.Submitted</code>；<code>attempt.Main</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code>；<code>err == nil &amp;&amp; bytes.Equal(a, b)</code> |

**跟着本段代码走一遍：**

- 检查 <code>attempt.Submitted == nil &#124;&#124; attempt.Main == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>bytes.Equal(projected, original)</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(projected, &amp;cut) != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(cut&#91;&quot;range_projection&quot;&#93;, &amp;proof) != nil &#124;&#124; proof.Source.Id != domain.TextID(attempt.Submitted.StructureId) &#124;&#124; proof.Target.Id != domain.TextID(attempt.Main.ExpectedS …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(a, b)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=29333:30865 -->
````go
func validProjectedSubmissionCut(projected []byte, attempt mergeAttemptContext) bool {
	if attempt.Submitted == nil || attempt.Main == nil {
		return false
	}
	original := attempt.Submitted.CutJsonUtf8
	if bytes.Equal(projected, original) {
		return true
	}
	var cut map[string]json.RawMessage
	if json.Unmarshal(projected, &cut) != nil {
		return false
	}
	var proof struct {
		Source dto.StructureRevision `json:"source_structure"`
		Target dto.StructureRevision `json:"target_structure"`
		Unit   string                `json:"unit_id"`
	}
	if json.Unmarshal(cut["range_projection"], &proof) != nil || proof.Source.Id != domain.TextID(attempt.Submitted.StructureId) || proof.Target.Id != domain.TextID(attempt.Main.ExpectedStructureId) ||
		proof.Source.DocumentId != domain.TextID(attempt.Submitted.DocumentId) || proof.Unit != domain.TextID(attempt.Submitted.UnitId) || !domain.SameStableUnitRange(proof.Source, proof.Target, proof.Unit) {
		return false
	}
	delete(cut, "range_projection")
	raw, err := json.Marshal(cut)
	if err != nil {
		return false
	}
	left, err := codec.Decode(raw)
	if err != nil {
		return false
	}
	right, err := codec.Decode(original)
	if err != nil {
		return false
	}
	a, err := codec.Canonical(left)
	if err != nil {
		return false
	}
	b, err := codec.Canonical(right)
	// 仅允许已证明的范围注解；房间、epoch和durable_seq及任何其它原字段必须完全相同。
	return err == nil && bytes.Equal(a, b)
}

// merge_range_version.go：保留原实现，按业务合并文件。

````

## 第 31 步：requireStableReviewRange

插入位置：第 30 步代码之后，紧接追加；完成稿约第 759 行。

本步声明：`func requireStableReviewRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>stableMergeRange(ctx, tx, document, unit, source, target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=30865:31062 -->
````go
func requireStableReviewRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) error {
	_, _, err := stableMergeRange(ctx, tx, document, unit, source, target)
	return err
}

````

## 第 32 步：stableMergeRange

插入位置：第 31 步代码之后，紧接追加；完成稿约第 764 行。

本步声明：`func stableMergeRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) (any, any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, domain.TextID(source), domain.TextID(document))</code>；<code>repository.StructureRow(ctx, tx, domain.TextID(target), domain.TextID(document))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, domain.TextID(source), domain.TextID(document))</code>；<code>repository.StructureRow(ctx, tx, domain.TextID(target), domain.TextID(document))</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(unit)</code> |
| <code>source</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(source)</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>old</code> |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MERGE_BASE_RANGE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!domain.SameStableUnitRange(old, current, domain.TextID(unit))</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;MERGE_BASE_RANGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return old, current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=31062:31641 -->
````go
func stableMergeRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) (any, any, error) {
	old, err := repository.StructureRow(ctx, tx, domain.TextID(source), domain.TextID(document))
	if err != nil {
		return nil, nil, err
	}
	current, err := repository.StructureRow(ctx, tx, domain.TextID(target), domain.TextID(document))
	if err != nil {
		return nil, nil, err
	}
	if !domain.SameStableUnitRange(old, current, domain.TextID(unit)) {
		return nil, nil, domain.Violation(domain.StateConflict, "MERGE_BASE_RANGE_CHANGED")
	}
	return old, current, nil
}

````

## 第 33 步：projectMergeVersion

插入位置：第 32 步代码之后，紧接追加；完成稿约第 779 行。

基线和送审正文分别绑定各自的实际版本；结构投影不修改正文/来源，也不读取S2当前草稿。

本步声明：`func projectMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, version, target uint64, submittedCut []byte) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>stableMergeRange(ctx, tx, content.DocumentId, content.UnitId, content.StructureId, target)</code>；<code>repository.VersionContent(ctx, tx, version, content.DocumentId)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>stableMergeRange(ctx, tx, content.DocumentId, content.UnitId, content.StructureId, target)</code>；<code>repository.VersionContent(ctx, tx, version, content.DocumentId)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content == nil</code>；<code>content.StructureId</code>；<code>return content, nil</code>；<code>content.DocumentId</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, version, content.DocumentId)</code>；<code>canonicalVersion(stored, version)</code> |
| <code>target</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.StructureId == target</code>；<code>stableMergeRange(ctx, tx, content.DocumentId, content.UnitId, content.StructureId, target)</code>；<code>projected.StructureId = target</code> |
| <code>submittedCut</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>submittedCut != nil</code>；<code>stored.CutJsonUtf8 = submittedCut</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>content</code>；<code>projected</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_VERSION_UNKNOWN&quot;)</code>；<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_VERSION_BINDING_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>content == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_VERSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>content.StructureId == target</code>。条件成立时执行 <code>return content, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>submittedCut != nil</code>。
- 检查 <code>!proto.Equal(stored, content)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_VERSION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_VERSION_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 33 bytes=31641:33436 -->
````go
// 基线和送审正文分别绑定各自的实际版本；结构投影不修改正文/来源，也不读取S2当前草稿。
func projectMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, version, target uint64, submittedCut []byte) (*pb.FixedContent, error) {
	if content == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_UNKNOWN")
	}
	if content.StructureId == target {
		return content, nil
	}
	old, current, err := stableMergeRange(ctx, tx, content.DocumentId, content.UnitId, content.StructureId, target)
	if err != nil {
		return nil, err
	}
	stored, err := repository.VersionContent(ctx, tx, version, content.DocumentId)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(stored, version); err != nil {
		return nil, err
	}
	if submittedCut != nil {
		// 送审的BRANCH绑定来自原审核上下文，切点重新取Doc不可变原SUBMISSION事实。
		stored.BranchId, stored.WorkspaceEpoch, stored.LineageRef = content.BranchId, content.WorkspaceEpoch, content.LineageRef
		stored.CutJsonUtf8 = submittedCut
	}
	if !proto.Equal(stored, content) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_BINDING_UNKNOWN")
	}
	cut, err := codec.Decode(content.CutJsonUtf8)
	if err != nil {
		return nil, err
	}
	value, ok := cut.(map[string]any)
	if !ok {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_CUT_UNKNOWN")
	}
	raw, err := json.Marshal(map[string]any{"source_structure": old, "target_structure": current, "unit_id": domain.TextID(content.UnitId)})
	if err != nil {
		return nil, err
	}
	evidence, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	value["range_projection"] = evidence
	projected := proto.Clone(content).(*pb.FixedContent)
````

## 第 34 步：projectMergeVersion（接着实现）

插入位置：第 33 步代码之后，紧接追加；完成稿约第 824 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return projected, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=33436:33615 -->
````go
	projected.StructureId = target
	projected.CutJsonUtf8, err = codec.Canonical(value)
	return projected, err
}

// merge_readiness.go：保留原实现，按业务合并文件。

````

## 第 35 步：Merges.Ready

插入位置：第 34 步代码之后，紧接追加；完成稿约第 831 行。

本步声明：`func (s *Merges) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingMergeBindings(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;MERGE_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;MERGE_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=33615:33921 -->
````go
func (s *Merges) Ready(ctx context.Context) error {
	facts, err := repository.PendingMergeBindings(ctx, s.db)
	if err != nil {
		return errors.New("MERGE_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingMergeBinding(fact); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 36 步：validatePendingMergeBinding

插入位置：第 35 步代码之后，紧接追加；完成稿约第 844 行。

本步声明：`func validatePendingMergeBinding(fact repository.PendingMergeBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingMergeBinding</code> | 使用 <code>repository.PendingMergeBinding</code> 类型；其定义处的职责：存储合并尝试、固定输入、冲突、版本清单、接纳lineage和安装恢复任务。<br>在此声明中的实际使用：<code>fact.Fact</code>；<code>fact.ReceiptActor</code>；<code>fact.Fingerprint</code>；<code>fact.Original</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;MERGE_RECOVERY_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MERGE_NOTIFICATION_FACT_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MERGE_RECOVERY_KIND_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>f.Actor == 0 &#124;&#124; f.Document == 0 &#124;&#124; f.Unit == 0 &#124;&#124; f.Branch == 0 &#124;&#124; f.Request == 0 &#124;&#124; f.Round == 0 &#124;&#124; f.Version == 0 &#124;&#124; 		fact.ReceiptActor != f.Actor &#124;&#124; len(fact.Fingerprint) != 32 …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MERGE_RECOVERY_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;fixed) != nil &#124;&#124; !validMergeRecoveryContext(fixed, fact) &#124;&#124; 			fact.ReceiptState != &quot;PREPARING&quot; &#124;&#124; (f.State != &quot;PREPARING&quot; &amp;&amp; f.State != &quot;COMPUTING&quot; &amp;&amp; f.State != &quot;PREPARED&quot;)</code>。条件成立时执行 <code>return errors.New(&quot;MERGE_RECOVERY_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;fixed) != nil &#124;&#124; !proto.Equal(fixed.Context, original.Context) &#124;&#124; fixed.Generation == 0 &#124;&#124; 			(fact.ReceiptState != &quot;SUCCEEDED&quot; &amp;&amp; fact.ReceiptState ! …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.ReceiptState == &quot;SUCCEEDED&quot; &amp;&amp; !validMainInstallBinding(fixed, fact)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>f.State != &quot;SUCCEEDED&quot; &#124;&#124; fact.ReceiptState != &quot;SUCCEEDED&quot; &#124;&#124; fact.Record == 0 &#124;&#124; 			json.Unmarshal(fact.Payload, &amp;notice) != nil &#124;&#124; notice&#91;&quot;event&quot;&#93; != &quot;APPROVE_AND_MERGE&quot; &#124;&#124; 			no …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MERGE_NOTIFICATION_FACT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;MERGE_RECOVERY_KIND_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=33921:35838 -->
````go
func validatePendingMergeBinding(fact repository.PendingMergeBinding) error {
	f := fact.Fact
	var original mergeContext
	if f.Actor == 0 || f.Document == 0 || f.Unit == 0 || f.Branch == 0 || f.Request == 0 || f.Round == 0 || f.Version == 0 ||
		fact.ReceiptActor != f.Actor || len(fact.Fingerprint) != 32 || json.Unmarshal(fact.Original, &original) != nil ||
		!validMergeRecoveryContext(original, fact) {
		return errors.New("MERGE_RECOVERY_BINDING_UNKNOWN")
	}
	switch fact.Kind {
	case "MERGE_RECOVER":
		var fixed mergeContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || !validMergeRecoveryContext(fixed, fact) ||
			fact.ReceiptState != "PREPARING" || (f.State != "PREPARING" && f.State != "COMPUTING" && f.State != "PREPARED") {
			return errors.New("MERGE_RECOVERY_BINDING_UNKNOWN")
		}
	case "MAIN_RECONCILE":
		var fixed mainInstallContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || !proto.Equal(fixed.Context, original.Context) || fixed.Generation == 0 ||
			(fact.ReceiptState != "SUCCEEDED" && fact.ReceiptState != "REJECTED" && fact.ReceiptState != "CANCELLED") {
			return errors.New("MAIN_RECONCILE_BINDING_UNKNOWN")
		}
		if fact.ReceiptState == "SUCCEEDED" && !validMainInstallBinding(fixed, fact) {
			return errors.New("MAIN_RECONCILE_BINDING_UNKNOWN")
		}
	case "MERGE_NOTIFICATION":
		var notice map[string]string
		if f.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Record == 0 ||
			json.Unmarshal(fact.Payload, &notice) != nil || notice["event"] != "APPROVE_AND_MERGE" ||
			notice["document"] != domain.TextID(f.Document) || notice["request"] != domain.TextID(f.Request) ||
			notice["round"] != domain.TextID(f.Round) || notice["record"] != domain.TextID(fact.Record) || notice["actor"] != domain.TextID(f.Actor) {
			return errors.New("MERGE_NOTIFICATION_FACT_UNKNOWN")
		}
	default:
		return errors.New("MERGE_RECOVERY_KIND_UNKNOWN")
	}
	return nil
}

````

## 第 37 步：validMergeRecoveryContext

插入位置：第 36 步代码之后，紧接追加；完成稿约第 882 行。

这里校验固定身份而非重新授予权限。原短期证明过期不冻结服务，工作者会在SQL锁外续接。

本步声明：`func validMergeRecoveryContext(fixed mergeContext, fact repository.PendingMergeBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Actor</code>；<code>fixed.Intent</code>；<code>fixed.Pointer</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>fact</code> | <code>repository.PendingMergeBinding</code> | 使用 <code>repository.PendingMergeBinding</code> 类型；其定义处的职责：存储合并尝试、固定输入、冲突、版本清单、接纳lineage和安装恢复任务。<br>在此声明中的实际使用：<code>fact.Fact</code>；<code>fact.Operation</code>；<code>fact.Fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>fixed.Access.Proofs&#91;0&#93;.AdmissionId == binding.IdentityAdmissionRef</code> |

**跟着本段代码走一遍：**

- 检查 <code>binding == nil &#124;&#124; binding.Target == nil &#124;&#124; binding.OperationId != fact.Operation &#124;&#124; binding.ActorId != f.Actor &#124;&#124; 		fixed.Actor != f.Actor &#124;&#124; fixed.Intent.Operation != fact.Operati …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>target.DocumentId != f.Document &#124;&#124; target.UnitId != f.Unit &#124;&#124; target.BranchId != f.Branch &#124;&#124; 		target.StructureId == 0 &#124;&#124; target.WorkspaceEpoch == 0 &#124;&#124; target.Purpose != &quot;MERGE&quot; &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.OperationId != fact.Operation &#124;&#124; proof.ActorId != f.Actor &#124;&#124; proof.Purpose != &quot;MERGE&quot; &#124;&#124; !proto.Equal(proof.Target, target)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return fixed.Access.Proofs&#91;0&#93;.AdmissionId == binding.IdentityAdmissionRef</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=35838:37490 -->
````go
// 这里校验固定身份而非重新授予权限。原短期证明过期不冻结服务，工作者会在SQL锁外续接。
func validMergeRecoveryContext(fixed mergeContext, fact repository.PendingMergeBinding) bool {
	f, binding := fact.Fact, fixed.Context
	if binding == nil || binding.Target == nil || binding.OperationId != fact.Operation || binding.ActorId != f.Actor ||
		fixed.Actor != f.Actor || fixed.Intent.Operation != fact.Operation || fixed.Intent.Round != f.Round || fixed.Intent.Version != f.Version ||
		fixed.Pointer.ID != f.Request || fixed.Pointer.Document != f.Document || fixed.Pointer.Unit != f.Unit || fixed.Pointer.Branch != f.Branch ||
		fixed.Pointer.Round != f.Round || fixed.SessionExpires <= 0 || fixed.Access.Document.Id != domain.TextID(f.Document) ||
		!bytes.Equal(binding.RequestFingerprint, fact.Fingerprint) || binding.IdentityAdmissionRef == "" || binding.DocumentAdmissionRef == "" {
		return false
	}
	target := binding.Target
	if target.DocumentId != f.Document || target.UnitId != f.Unit || target.BranchId != f.Branch ||
		target.StructureId == 0 || target.WorkspaceEpoch == 0 || target.Purpose != "MERGE" || len(binding.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: binding.Sources}, &pb.OperationContext{Sources: fixed.Access.Sources}) || len(fixed.Access.Proofs) == 0 {
		return false
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.OperationId != fact.Operation || proof.ActorId != f.Actor || proof.Purpose != "MERGE" || !proto.Equal(proof.Target, target) {
			return false
		}
	}
	return fixed.Access.Proofs[0].AdmissionId == binding.IdentityAdmissionRef
}

````

## 第 38 步：validMainInstallBinding

插入位置：第 37 步代码之后，紧接追加；完成稿约第 906 行。

本步声明：`func validMainInstallBinding(fixed mainInstallContext, fact repository.PendingMergeBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code>；<code>fixed.Control</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>fact</code> | <code>repository.PendingMergeBinding</code> | 使用 <code>repository.PendingMergeBinding</code> 类型；其定义处的职责：存储合并尝试、固定输入、冲突、版本清单、接纳lineage和安装恢复任务。<br>在此声明中的实际使用：<code>fact.Fact</code>；<code>fact.Record</code>；<code>fact.Manifest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; bytes.Equal(digest&#91;:&#93;, fixed.Digest)</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.Fact.State != &quot;SUCCEEDED&quot; &#124;&#124; fact.Record == 0 &#124;&#124; fixed.Manifest == 0 &#124;&#124; fixed.Manifest != fact.Manifest &#124;&#124; 		fixed.Structure == 0 &#124;&#124; fixed.Style == 0 &#124;&#124; fixed.Control == 0 &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>unit == nil &#124;&#124; unit.UnitId == 0 &#124;&#124; unit.ContentVersionId == 0 &#124;&#124; seen&#91;unit.UnitId&#93; &#124;&#124; unit.Content == nil &#124;&#124; 			unit.Content.UnitId != unit.UnitId &#124;&#124; unit.Content.DocumentId != fac …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(digest&#91;:&#93;, fixed.Digest)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=37490:38486 -->
````go
func validMainInstallBinding(fixed mainInstallContext, fact repository.PendingMergeBinding) bool {
	if fact.Fact.State != "SUCCEEDED" || fact.Record == 0 || fixed.Manifest == 0 || fixed.Manifest != fact.Manifest ||
		fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 || len(fixed.Units) == 0 || len(fixed.Digest) != 32 {
		return false
	}
	seen := map[uint64]bool{}
	for _, unit := range fixed.Units {
		if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || seen[unit.UnitId] || unit.Content == nil ||
			unit.Content.UnitId != unit.UnitId || unit.Content.DocumentId != fact.Fact.Document || unit.Content.StructureId != fixed.Structure ||
			unit.Content.BranchId != 0 || unit.Content.WorkspaceEpoch != fixed.Control {
			return false
		}
		seen[unit.UnitId] = true
	}
	digest, err := domain.MainManifestDigest(fixed.Manifest, fixed.Units)
	return err == nil && bytes.Equal(digest[:], fixed.Digest)
}

// merge_readings.go：保留原实现，按业务合并文件。

````

## 第 39 步：Merges.Lookup

插入位置：第 38 步代码之后，紧接追加；完成稿约第 926 行。

本步声明：`func (s *Merges) Lookup(ctx context.Context, operation string) (domain.MergeFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeRow(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeRow(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>由 <code>repository.MergeRow(ctx, s.db, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.MergeRow(ctx, s.db, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.MergeRow(ctx, s.db, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=38486:38630 -->
````go
func (s *Merges) Lookup(ctx context.Context, operation string) (domain.MergeFact, error) {
	return repository.MergeRow(ctx, s.db, operation)
}

````

## 第 40 步：Merges.Progress

插入位置：第 39 步代码之后，紧接追加；完成稿约第 930 行。

本步声明：`func (s *Merges) Progress(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, fact.Document, &quot;MERGE_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fact.Document, &quot;MERGE_RESULT&quot;)</code>；<code>s.readMergeResult(ctx, tx, actor, access, pointer, fact.Operation)</code> |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Actor</code>；<code>fact.Document</code>；<code>fact.Request</code>；<code>fact.Unit</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;MERGE_NOT_FOUND&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>actor.ID != fact.Actor</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;MERGE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!reviewContextAllowed(access, pointer)</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return s.readMergeResult(ctx, tx, actor, access, pointer, fact.Operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=38630:39401 -->
````go
func (s *Merges) Progress(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error) {
	if actor.ID != fact.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "MERGE_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fact.Document, "MERGE_RESULT")
	if err != nil {
		return nil, nil, err
	}
	pointer := domain.ReviewPointer{ID: fact.Request, Document: fact.Document, Unit: fact.Unit}
	if !reviewContextAllowed(access, pointer) {
		return nil, nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readMergeResult(ctx, tx, actor, access, pointer, fact.Operation)
	}
	return access.Sources, run, nil
}

````

## 第 41 步：Merges.mergePresentation

插入位置：第 40 步代码之后，紧接追加；完成稿约第 948 行。

本步声明：`func (s *Merges) mergePresentation(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.readMergeResult(ctx, tx, actor, access, pointer, operation)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>s.readMergeResult(ctx, tx, actor, access, pointer, operation)</code>；<code>access.Sources</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.readMergeResult(ctx, tx, actor, access, pointer, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.readMergeResult(ctx, tx, actor, access, pointer, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readMergeResult(ctx, tx, actor, access, pointer, operation) 		return value, access.Sources, err 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readMergeResult(ctx, tx, actor, access, pointer, operation) 		return value, access.Sources, err 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return value, access.Sources, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=39401:39761 -->
````go
func (s *Merges) mergePresentation(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readMergeResult(ctx, tx, actor, access, pointer, operation)
		return value, access.Sources, err
	}
}

````

## 第 42 步：Merges.readMergeResult

插入位置：第 41 步代码之后，紧接追加；完成稿约第 955 行。

本步声明：`func (s *Merges) readMergeResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.MergeRow(ctx, tx, operation)</code>；<code>repository.MergeResult(ctx, tx, operation)</code>；<code>repository.ReviewPointerRow(ctx, tx, pointer.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.MergeRow(ctx, tx, operation)</code>；<code>repository.MergeResult(ctx, tx, operation)</code>；<code>repository.ReviewPointerRow(ctx, tx, pointer.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>reviewContextAllowed(access, pointer)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>reviewContextAllowed(access, pointer)</code>；<code>pointer.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeRow(ctx, tx, operation)</code>；<code>repository.MergeResult(ctx, tx, operation)</code>；<code>repository.MainInstallationCompleted(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;MERGE_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!reviewContextAllowed(access, pointer)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; fact.Actor != actor.ID &#124;&#124; fact.Request != pointer.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;MERGE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!result.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>result.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>ready</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=39761:41358 -->
````go
func (s *Merges) readMergeResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	if !reviewContextAllowed(access, pointer) {
		return nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	fact, err := repository.MergeRow(ctx, tx, operation)
	if err != nil || fact.Actor != actor.ID || fact.Request != pointer.ID {
		return nil, domain.Violation(domain.EntityNotFound, "MERGE_NOT_FOUND")
	}
	result, err := repository.MergeResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !result.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if err = json.Unmarshal(result.Result, &value); err != nil {
		return nil, err
	}
	current, err := repository.ReviewPointerRow(ctx, tx, pointer.ID)
	if err != nil {
		return nil, err
	}
	value["current_state"] = map[string]string{"review_state": current.Status, "current_round_id": domain.TextID(current.Round), "revision": domain.TextID(current.Revision)}
	if result.State == "SUCCEEDED" {
		ready, err := repository.MainInstallationCompleted(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		sync := "SYNCING"
		if ready {
			sync = "READY"
		}
		// 契约把安装状态放在result；这里只更新HTTP投影，原receipt/RPC终态摘要和固定版本编号不变。
		value["result"].(map[string]any)["main_sync_state"] = sync
	}
	return value, nil
}

````

## 第 43 步：Merges.OperationResult

插入位置：第 42 步代码之后，紧接追加；完成稿约第 997 行。

系统安装只核对原号/原目标/原摘要对应的权威事实；用户读取仍走上面的当前授权与输出租约。

本步声明：`func (s *Merges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergePayload(ctx, s.db, query.OperationId)</code>；<code>repository.MergeResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BINDING_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>text != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 43 bytes=41358:43316 -->
````go
// 系统安装只核对原号/原目标/原摘要对应的权威事实；用户读取仍走上面的当前授权与输出租约。
func (s *Merges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	payload, err := repository.MergePayload(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed mergeContext
	if json.Unmarshal(payload, &fixed) != nil || fixed.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.MergeResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	result.State = pb.OperationResult_REJECTED
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
	}
	var value struct {
		Result map[string]string `json:"result"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err = json.Unmarshal(fact.Result, &value); err != nil {
		return nil, err
	}
	for _, key := range []string{"merge_record_id", "manifest_id", "content_version_id"} {
		if text := value.Result[key]; text != "" {
			id, err := domain.CanonicalID(text)
			if err != nil {
				return nil, err
			}
````

## 第 44 步：Merges.OperationResult（接着实现）

插入位置：第 43 步代码之后，紧接追加；完成稿约第 1042 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=43316:43510 -->
````go
			result.ResultIds = append(result.ResultIds, id)
		}
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// merge_submission_cut.go：保留原实现，按业务合并文件。

````

## 第 45 步：readSubmissionCut

插入位置：第 44 步代码之后，紧接追加；完成稿约第 1051 行。

本步声明：`func readSubmissionCut(ctx context.Context, q repository.Queryer, round uint64) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, q, &quot;REVIEW_ROUND_CONTEXT&quot;, domain.TextID(round))</code>；<code>repository.MergeArtifact(ctx, q, &quot;REVIEW_CAPTURE_CUT&quot;, domain.TextID(round))</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, q, &quot;REVIEW_ROUND_CONTEXT&quot;, domain.TextID(round))</code>；<code>repository.MergeArtifact(ctx, q, &quot;REVIEW_CAPTURE_CUT&quot;, domain.TextID(round))</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(round)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>original.Cut</code>；<code>fact.Cut</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;original) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(original.Cut) != 0 &amp;&amp; !bytes.Equal(original.Cut, &#91;&#93;byte(&quot;null&quot;))</code>。条件成立时执行 <code>return original.Cut, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fact) != nil &#124;&#124; len(fact.Cut) == 0</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact.Cut, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 45 bytes=43510:44401 -->
````go
func readSubmissionCut(ctx context.Context, q repository.Queryer, round uint64) (json.RawMessage, error) {
	raw, err := repository.MergeArtifact(ctx, q, "REVIEW_ROUND_CONTEXT", domain.TextID(round))
	if err != nil {
		return nil, err
	}
	var original struct {
		Cut json.RawMessage `json:"cut"`
	}
	if json.Unmarshal(raw, &original) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	if len(original.Cut) != 0 && !bytes.Equal(original.Cut, []byte("null")) {
		return original.Cut, nil
	}
	raw, err = repository.MergeArtifact(ctx, q, "REVIEW_CAPTURE_CUT", domain.TextID(round))
	if err != nil {
		return nil, err
	}
	var fact domain.CommittedSubmissionCut
	if json.Unmarshal(raw, &fact) != nil || len(fact.Cut) == 0 {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	return fact.Cut, nil
}

````

## 第 46 步：Merges.ensureSubmissionCut

插入位置：第 45 步代码之后，紧接追加；完成稿约第 1076 行。

本步声明：`func (s *Merges) ensureSubmissionCut(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readSubmissionCut(ctx, s.db, fixed.Intent.Round)</code>；<code>repository.MergeRoundInputs(ctx, s.db, fixed.Pointer.ID, fixed.Intent.Round)</code>；<code>repository.SubmissionPayload(ctx, s.db, round.Submission)</code>；<code>s.client.CommittedCut(ctx, &amp;pb.OperationQuery{OperationId: round.Submission, Target: binding.Target, RequestFingerprint: binding.RequestFingerprint})</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.Pointer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;original) != nil &#124;&#124; original.Capture == nil &#124;&#124; original.Capture.Context == nil &#124;&#124; original.Capture.Context.Target == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Cut, &amp;shape) != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Operation != round.Submission &#124;&#124; fact.Version != round.SubmittedVersion &#124;&#124; !bytes.Equal(fact.BodyHash, content.BodyHash) &#124;&#124; 		binding.Target.DocumentId != fixed.Pointer.Docume …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 46 bytes=44401:46546 -->
````go
func (s *Merges) ensureSubmissionCut(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error {
	if _, err := readSubmissionCut(ctx, s.db, fixed.Intent.Round); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}
	round, err := repository.MergeRoundInputs(ctx, s.db, fixed.Pointer.ID, fixed.Intent.Round)
	if err != nil {
		return err
	}
	raw, err := repository.SubmissionPayload(ctx, s.db, round.Submission)
	if err != nil {
		return err
	}
	var original submissionContext
	if json.Unmarshal(raw, &original) != nil || original.Capture == nil || original.Capture.Context == nil || original.Capture.Context.Target == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	binding := original.Capture.Context
	// RPC在SQL锁外，只按已成功送审的原target/fingerprint读固定系统事实；不续接旧作者的权限。
	fact, err := s.client.CommittedCut(ctx, &pb.OperationQuery{OperationId: round.Submission, Target: binding.Target, RequestFingerprint: binding.RequestFingerprint})
	if err != nil {
		return err
	}
	content, err := repository.VersionContent(ctx, s.db, round.SubmittedVersion, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	var cut struct{ Room, Epoch, Seq string }
	var shape map[string]string
	if json.Unmarshal(fact.Cut, &shape) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	cut.Room, cut.Epoch, cut.Seq = shape["room_id"], shape["workspace_epoch"], shape["durable_seq"]
	if fact.Operation != round.Submission || fact.Version != round.SubmittedVersion || !bytes.Equal(fact.BodyHash, content.BodyHash) ||
		binding.Target.DocumentId != fixed.Pointer.Document || binding.Target.UnitId != fixed.Pointer.Unit || binding.Target.BranchId != fixed.Pointer.Branch ||
		cut.Epoch != domain.TextID(binding.Target.WorkspaceEpoch) || cut.Room == "" || cut.Seq == "" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	payload, err := json.Marshal(fact)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
````

## 第 47 步：Merges.ensureSubmissionCut（接着实现）

插入位置：第 46 步代码之后，紧接追加；完成稿约第 1121 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return tx.Commit()</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 47 bytes=46546:47266 -->
````go
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if _, err = readSubmissionCut(ctx, tx, fixed.Intent.Round); err == nil {
		return tx.Commit()
	} else if err != sql.ErrNoRows {
		return err
	}
	// 追加单独的固定证明，不改旧REVIEW_ROUND_CONTEXT，更不重采当前支线尾部。
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_CAPTURE_CUT", domain.TextID(fixed.Intent.Round), round.Submission, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// merges.go：保留原实现，按业务合并文件。

````

## 第 48 步：MergeClient

插入位置：第 47 步代码之后，紧接追加；完成稿约第 1144 行。

本步声明：`type MergeClient interface {`。

**这一段为什么这样写：** 合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>CommittedCut</code> | <code>(context.Context, *pb.OperationQuery) (domain.CommittedSubmissionCut, error)</code> | ctx与原操作查询取得已提交送审cut，用于核对送审来源。 |
| <code>CaptureMain</code> | <code>(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)</code> | ctx及固定主线采样请求取得本次合并主线输入。 |
| <code>Compute</code> | <code>(context.Context, *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error)</code> | ctx及基线/提交/主线固定输入，返回候选或冲突；计算不直接写主线。 |
| <code>PrepareMain</code> | <code>(context.Context, *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error)</code> | ctx及期望cut请求冻结主线提交屏障，确认输入还适用。 |
| <code>ReconcileMain</code> | <code>(context.Context, *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error)</code> | ctx及已提交安装请求核验Node是否安装原结果；失败继续原操作而非再算新合并。 |

<!-- write-step: 48 bytes=47266:47738 -->
````go
type MergeClient interface {
	CommittedCut(context.Context, *pb.OperationQuery) (domain.CommittedSubmissionCut, error)
	CaptureMain(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)
	Compute(context.Context, *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error)
	PrepareMain(context.Context, *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error)
	ReconcileMain(context.Context, *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error)
}

````

## 第 49 步：Merges

插入位置：第 48 步代码之后，紧接追加；完成稿约第 1152 行。

本步声明：`type Merges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>MergeClient</code> | 使用 <code>MergeClient</code> 类型；其定义处的职责：合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 49 bytes=47738:47890 -->
````go
type Merges struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

````

## 第 50 步：mergeContext

插入位置：第 49 步代码之后，紧接追加；完成稿约第 1161 行。

本步声明：`type mergeContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>序列化/校验标记：<code>`json:&quot;pointer&quot;`</code> |
| <code>Intent</code> | <code>domain.MergeIntent</code> | 使用 <code>domain.MergeIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>序列化/校验标记：<code>`json:&quot;intent&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |

<!-- write-step: 50 bytes=47890:48248 -->
````go
type mergeContext struct {
	Actor          uint64               `json:"actor"`
	SessionExpires int64                `json:"session_expires"`
	Pointer        domain.ReviewPointer `json:"pointer"`
	Intent         domain.MergeIntent   `json:"intent"`
	Access         DocumentAccess       `json:"access"`
	Context        *pb.OperationContext `json:"context"`
}

````

## 第 51 步：NewMerges

插入位置：第 50 步代码之后，紧接追加；完成稿约第 1170 行。

本步声明：`func NewMerges(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Merges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>MergeClient</code> | 使用 <code>MergeClient</code> 类型；其定义处的职责：合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Merges</code> | 实际返回表达式：<code>&amp;Merges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Merges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 51 bytes=48248:48506 -->
````go
func NewMerges(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Merges {
	return &Merges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/merges_preparation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
