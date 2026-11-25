# 分步编写 `services/document/internal/logic/document/submissions_service.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/242-submissions-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。

提交不是浏览器当前显示的文本，而是指定工作区已可靠保存且来源可验证的固定内容。

写之前先按这条执行路线理解各段的连接：

1. Prepare绑定原workspace/lineage/guard
2. RecoverOne续原证明并捕获
3. commitSubmission复核内容、资源及任务代次后写轮次
4. finalizeSubmission确认捕获最终状态
5. Progress投影原结果。

本文件需要守住的边界：

- 未知RPC必须重试原号
- 无变化、资源未就绪或会话过期按实际规则处理
- 后台JSON不得保存浏览器token。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/submissions_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/submissions_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/submissions_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/submissions_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:627 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// submission_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：Submissions.commitSubmission

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (s *Submissions) commitSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, content *pb.FixedContent, currentProofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 17 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateSubmissionTask(ctx, tx, task)</code>；<code>task.Payload</code>；<code>repository.FinishSubmissionTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>fixed</code> | <code>submissionContext</code> | 在此声明中的实际使用：<code>fixed.Capture</code>；<code>fixed.Workspace</code>；<code>fixed.Room</code>；<code>fixed.LineageProvenance</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>domain.ValidateSubmissionCapture(s.validator, content, domain.CaptureExpectation{Request: fixed.Capture, Workspace: fixed.Workspace, Room: fixed.Room, LineageProvenance: fixed.LineageProvenance})</code>；<code>content.BodyHash</code>；<code>repository.InsertSubmissionVersion(ctx, tx, content)</code>；<code>repository.CreateSubmissionRound(ctx, tx, request, fixed.Actor, version, content, workspace)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>currentProofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>len(currentProofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_GUARD_SUPERSEDED&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; branch != target.BranchId</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; workspace != fixed.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>guard.Submission != task.Operation &#124;&#124; guard.Generation != fixed.Capture.GuardGeneration</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_GUARD_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(currentProofs) != len(fixed.Access.Proofs)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_RESUME_PROOF_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=627:2649 -->
````go
func (s *Submissions) commitSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, content *pb.FixedContent, currentProofs []*pb.IdentityAdmission) error {
	resources, err := domain.ValidateSubmissionCapture(s.validator, content, domain.CaptureExpectation{Request: fixed.Capture, Workspace: fixed.Workspace, Room: fixed.Room, LineageProvenance: fixed.LineageProvenance})
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	target := fixed.Capture.Context.Target
	branch, err := repository.LockedActiveBranch(ctx, tx, target.UnitId, target.DocumentId)
	if err != nil || branch != target.BranchId {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedEditWorkspace(ctx, tx, branch, target.DocumentId, target.UnitId)
	if err != nil || workspace != fixed.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, target.UnitId)
	if err != nil {
		return err
	}
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if guard.Submission != task.Operation || guard.Generation != fixed.Capture.GuardGeneration {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_GUARD_SUPERSEDED")
	}
	// RPC 与等锁可能跨过自然期限，最终业务事务仍须检查本次实际续接证明和原会话的期限。
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(currentProofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_RESUME_PROOF_INVALID")
	}
````

## 第 3 步：Submissions.commitSubmission（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 71 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_ADMISSION_RETRY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;PREPARING&quot; &amp;&amp; fact.State != &quot;CAPTURED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_ALREADY_TERMINAL&quot;)</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Mode == &quot;RESUBMIT&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESUBMIT_POSITION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Mode == &quot;NEW&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>baseline.UnitId != target.UnitId</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>bytes.Equal(baseHash&#91;:&#93;, content.BodyHash)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SUBMISSION_NO_CHANGES&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2649:4349 -->
````go
	for _, proof := range currentProofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_ADMISSION_RETRY_REQUIRED")
		}
	}
	fact, err := repository.SubmissionRow(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if fact.State != "PREPARING" && fact.State != "CAPTURED" {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_ALREADY_TERMINAL")
	}
	var pointer *domain.ReviewPointer
	if fixed.Intent.Mode == "RESUBMIT" {
		value, err := repository.LockedReviewPointer(ctx, tx, fixed.Intent.Request)
		if err != nil {
			return domain.Violation(domain.StateConflict, "RESUBMIT_POSITION_CHANGED")
		}
		pointer = &value
	}
	position := guard
	position.Submission = ""
	if err = domain.ValidateSubmissionPosition(fixed.Intent, position, pointer, target.DocumentId, target.UnitId); err != nil {
		return err
	}
	if fixed.Intent.Mode == "NEW" {
		baseline, err := repository.VersionContent(ctx, tx, workspace.BaseVersion, target.DocumentId)
		if err != nil {
			return err
		}
		if err = canonicalVersion(baseline, workspace.BaseVersion); err != nil {
			return err
		}
		if baseline.UnitId != target.UnitId {
			return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_BASE_UNKNOWN")
		}
		// MAIN独立字体不属于可送审的支线变化；与同一真实基线的BRANCH投影比较，不能虚构NEW。
		projected, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
		if err != nil {
			return err
		}
		baseHash := sha256.Sum256(projected)
		if bytes.Equal(baseHash[:], content.BodyHash) {
			return domain.Violation(domain.StateConflict, "SUBMISSION_NO_CHANGES")
		}
	}
````

## 第 4 步：Submissions.commitSubmission（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 117 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SUBMISSION_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Mode == &quot;NEW&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=4349:6413 -->
````go
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, id := range resources {
		state, err := repository.LockedDocumentResource(ctx, tx, id, target.DocumentId, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "SUBMISSION_RESOURCE_NOT_READY")
		}
	}
	version, err := repository.InsertSubmissionVersion(ctx, tx, content)
	if err != nil {
		return err
	}
	for _, id := range resources {
		if err = repository.InsertSubmissionResource(ctx, tx, id, target.DocumentId, version); err != nil {
			return err
		}
	}
	request := fixed.Intent.Request
	if fixed.Intent.Mode == "NEW" {
		request, err = repository.CreateReviewRequest(ctx, tx, target.DocumentId, target.UnitId, branch)
		if err != nil {
			return err
		}
	}
	round, err := repository.CreateSubmissionRound(ctx, tx, request, fixed.Actor, version, content, workspace)
	if err != nil {
		return err
	}
	// 轮次目录由 structure_id 固定，样式另外存不可变上下文，避免读取时偷换成文档当前样式。
	// 同时保留原可靠切点，后续接纳来源无需依赖原提交者仍在线或重新读取可变房间。
	roundContext, err := json.Marshal(map[string]any{"style_id": fixed.Style, "document_title": fixed.Access.Document.Title, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_ROUND_CONTEXT", domain.TextID(round), task.Operation, roundContext, "SUCCEEDED"); err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)}}
	if err = repository.CompleteSubmission(ctx, tx, task.Operation, fixed.Actor, target.UnitId, request, round, result); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "CAPTURE_FINALIZE", task.Operation, task.Operation, task.Payload, "QUEUED"); err != nil {
		return err
	}
````

## 第 5 步：Submissions.commitSubmission（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 163 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=6413:6914 -->
````go
	notice, err := json.Marshal(map[string]string{"document_id": domain.TextID(target.DocumentId), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "kind": "SUBMISSION"})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
	if err = repository.FinishSubmissionTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 6 步：Submissions.rejectSubmission

插入位置：第 5 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func (s *Submissions) rejectSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.LockedReviewGuard(ctx, tx, fixed.Capture.Context.Target.UnitId)</code>；<code>repository.ValidateSubmissionTask(ctx, tx, task)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateSubmissionTask(ctx, tx, task)</code>；<code>repository.RejectSubmission(ctx, tx, task, fixed.Capture.Context.Target.UnitId, result, code)</code> |
| <code>fixed</code> | <code>submissionContext</code> | 在此声明中的实际使用：<code>fixed.Capture</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code>；<code>repository.RejectSubmission(ctx, tx, task, fixed.Capture.Context.Target.UnitId, result, code)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_GUARD_SUPERSEDED&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>guard.Submission != task.Operation &#124;&#124; guard.Generation != fixed.Capture.GuardGeneration</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_GUARD_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 6 bytes=6914:8046 -->
````go
func (s *Submissions) rejectSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, fixed.Capture.Context.Target.UnitId)
	if err != nil {
		return err
	}
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if guard.Submission != task.Operation || guard.Generation != fixed.Capture.GuardGeneration {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_GUARD_SUPERSEDED")
	}
	result := map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}}
	if err = repository.RejectSubmission(ctx, tx, task, fixed.Capture.Context.Target.UnitId, result, code); err != nil {
		return err
	}
	return tx.Commit()
}

// submission_prepare.go：保留原实现，按业务合并文件。

````

## 第 7 步：Submissions.Prepare

插入位置：第 6 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`func (s *Submissions) Prepare(ctx context.Context, actor Actor, document, unit uint64, intent domain.SubmissionIntent, path string) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;SUBMISSION&quot;)</code>；<code>repository.SubmissionRow(ctx, s.db, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, intent.Operation, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;SUBMISSION&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;SUBMISSION&quot;)</code>；<code>s.submissionPresentation(actor, access, document, unit, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>DocumentId: document</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>!access.Permissions.Edit&#91;unit&#93;</code>；<code>s.submissionPresentation(actor, access, document, unit, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>UnitId: unit</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.SubmissionIntent</code> | 使用 <code>domain.SubmissionIntent</code> 类型；其定义处的职责：SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。<br>在此声明中的实际使用：<code>intent.Operation</code>；<code>intent.Branch</code>；<code>intent.Structure</code>；<code>intent.Canonical</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, json.RawMessage(intent.Canonical))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, present, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(&amp;pb.OperationContext{Sources: proof.Sources}, &amp;pb.OperationContext{Sources: bound.Sources})</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=8046:10282 -->
````go
func (s *Submissions) Prepare(ctx context.Context, actor Actor, document, unit uint64, intent domain.SubmissionIntent, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "SUBMISSION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	present := s.submissionPresentation(actor, access, document, unit, intent.Operation)
	// 原号结果先读原回执，不能因后来的工作区变化改成一次新提交。
	if _, err = repository.SubmissionRow(ctx, s.db, intent.Operation); err == nil {
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: workspace.Epoch, Purpose: "SUBMISSION"}
	original := access.Proofs
	access.Proofs = make([]*pb.IdentityAdmission, 0, len(original))
	for _, proof := range original {
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
	fingerprint, err := domain.OperationFingerprint(path, json.RawMessage(intent.Canonical))
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
````

## 第 8 步：Submissions.Prepare（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 249 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>current.CurrentStructureId != domain.TextID(intent.Structure)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; branch != intent.Branch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; locked != workspace</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Mode == &quot;RESUBMIT&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RESUBMIT_POSITION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=10282:11988 -->
````go
		if current.CurrentStructureId != domain.TextID(intent.Structure) {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		branch, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil || branch != intent.Branch {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
		}
		locked, err := repository.LockedEditWorkspace(ctx, tx, branch, document, unit)
		if err != nil || locked != workspace {
			return nil, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, unit)
		if err != nil {
			return nil, err
		}
		var pointer *domain.ReviewPointer
		if intent.Mode == "RESUBMIT" {
			value, err := repository.LockedReviewPointer(ctx, tx, intent.Request)
			if err != nil {
				return nil, domain.Violation(domain.StateConflict, "RESUBMIT_POSITION_CHANGED")
			}
			pointer = &value
		}
		if err := domain.ValidateSubmissionPosition(intent, guard, pointer, document, unit); err != nil {
			return nil, err
		}
		_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, branch)
		if err != nil {
			return nil, err
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err := InsertAdmission(ctx, tx, actor, access.Sources, document, "SUBMISSION", admission); err != nil {
			return nil, err
		}
		targetJSON, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err := repository.ConfigureSubmissionAdmission(ctx, tx, admission, unit, intent.Operation, targetJSON, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
````

## 第 9 步：Submissions.Prepare（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 294 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: intent.Operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + intent.Operation}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=11988:13031 -->
````go
		capture := &pb.CaptureSubmissionRequest{Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}, FlushReceiptId: intent.Flush, GuardGeneration: guard.Generation + 1, LineageRef: domain.TextID(workspace.Lineage)}
		fixed := submissionContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Capture: capture, Workspace: workspace, Room: room, LineageProvenance: provenance, Style: current.CurrentStyleId}
		raw, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err := repository.RegisterSubmission(ctx, tx, actor.ID, document, unit, intent, workspace, capture.GuardGeneration, raw); err != nil {
			return nil, err
		}
		return map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}, nil
	}
	return actor, run, present, nil
}

````

## 第 10 步：Submissions.submissionPresentation

插入位置：第 9 步代码之后，紧接追加；完成稿约第 308 行。

本步声明：`func (s *Submissions) submissionPresentation(actor Actor, access DocumentAccess, document, unit uint64, operation string) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>access.Permissions</code>；<code>access.Sources</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fact.Document != document</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>!access.Permissions.Edit&#91;unit&#93;</code>；<code>fact.Unit != unit</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.SubmissionRow(ctx, tx, operation)</code>；<code>repository.SubmissionResult(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, value map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, nil, err 	 …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, value map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, nil …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; fact.Actor != actor.ID &#124;&#124; fact.Document != document &#124;&#124; fact.Unit != unit</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;SUBMISSION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!result.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, access.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=13031:14179 -->
````go
func (s *Submissions) submissionPresentation(actor Actor, access DocumentAccess, document, unit uint64, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		if !access.Permissions.Edit[unit] {
			return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
		}
		fact, err := repository.SubmissionRow(ctx, tx, operation)
		if err != nil || fact.Actor != actor.ID || fact.Document != document || fact.Unit != unit {
			return nil, nil, domain.Violation(domain.EntityNotFound, "SUBMISSION_NOT_FOUND")
		}
		result, err := repository.SubmissionResult(ctx, tx, operation)
		if err != nil {
			return nil, nil, err
		}
		if !result.Expires.After(time.Now()) {
			return nil, nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
		}
		if err := json.Unmarshal(result.Result, &value); err != nil {
			return nil, nil, err
		}
		return value, access.Sources, nil
	}
}

// submission_readiness.go：保留原实现，按业务合并文件。

````

## 第 11 步：Submissions.Ready

插入位置：第 10 步代码之后，紧接追加；完成稿约第 336 行。

本步声明：`func (s *Submissions) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingSubmissionContexts(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SUBMISSION_RECOVERY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SUBMISSION_RECOVERY_INVALID&quot;)</code>；<code>errors.New(&quot;SUBMISSION_RECOVERY_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;REVIEW_NOTIFICATION_FACT_UNKNOWN&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;facts) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_RECOVERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Actor == 0 &#124;&#124; fact.Document == 0 &#124;&#124; fact.Unit == 0 &#124;&#124; fact.Branch == 0 &#124;&#124; fact.Operation == &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_RECOVERY_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;REVIEW_NOTIFICATION&quot;</code>。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &#124;&#124; fact.ReceiptState != &quot;SUCCEEDED&quot; &#124;&#124; fact.Round == 0</code>。条件成立时执行 <code>return errors.New(&quot;REVIEW_NOTIFICATION_FACT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;fixed) != nil &#124;&#124; fixed.Capture == nil &#124;&#124; fixed.Capture.Context == nil &#124;&#124; fixed.Capture.Context.Target == nil</code>。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_RECOVERY_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>binding.OperationId != fact.Operation &#124;&#124; binding.ActorId != fact.Actor &#124;&#124; target.DocumentId != fact.Document &#124;&#124; target.UnitId != fact.Unit &#124;&#124; target.BranchId != fact.Branch &#124;&#124; fixe …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_RECOVERY_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;SUBMISSION_RECOVER&quot;</code>。
- 检查 <code>fact.ReceiptState != &quot;PREPARING&quot; &#124;&#124; (fact.State != &quot;PREPARING&quot; &amp;&amp; fact.State != &quot;CAPTURED&quot;) &#124;&#124; fact.HeldOperation == nil &#124;&#124; *fact.HeldOperation != fact.Operation &#124;&#124; fact.HeldGeneration != fact.GuardGeneration</code>。条件成立时执行 <code>return errors.New(&quot;SUBMISSION_GUARD_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &#124;&#124; fact.ReceiptState != &quot;SUCCEEDED&quot; &#124;&#124; fact.Round == 0</code>。条件成立时执行 <code>return errors.New(&quot;CAPTURE_FINALIZE_FACT_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=14179:16650 -->
````go
func (s *Submissions) Ready(ctx context.Context) error {
	raw, err := repository.PendingSubmissionContexts(ctx, s.db)
	if err != nil {
		return errors.New("SUBMISSION_RECOVERY_UNAVAILABLE")
	}
	var facts []struct {
		Kind                          string          `json:"kind"`
		Operation                     string          `json:"operation"`
		Payload                       json.RawMessage `json:"payload"`
		Actor, Document, Unit, Branch uint64
		GuardGeneration               uint64  `json:"guard_generation"`
		State                         string  `json:"state"`
		ReceiptState                  string  `json:"receipt_state"`
		HeldOperation                 *string `json:"held_operation"`
		HeldGeneration                uint64  `json:"held_generation"`
		Round                         uint64  `json:"round"`
	}
	if json.Unmarshal(raw, &facts) != nil {
		return errors.New("SUBMISSION_RECOVERY_INVALID")
	}
	for _, fact := range facts {
		if fact.Actor == 0 || fact.Document == 0 || fact.Unit == 0 || fact.Branch == 0 || fact.Operation == "" {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		if fact.Kind == "REVIEW_NOTIFICATION" {
			// 通知意图是已提交轮次的后续投递；Task 9 接管投递，存在合法积压不冻结整个文档服务。
			if fact.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Round == 0 {
				return errors.New("REVIEW_NOTIFICATION_FACT_UNKNOWN")
			}
			continue
		}
		var fixed submissionContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil || fixed.Capture.Context.Target == nil {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		binding := fixed.Capture.Context
		target := binding.Target
		if binding.OperationId != fact.Operation || binding.ActorId != fact.Actor || target.DocumentId != fact.Document || target.UnitId != fact.Unit || target.BranchId != fact.Branch || fixed.Capture.GuardGeneration != fact.GuardGeneration {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		if fact.Kind == "SUBMISSION_RECOVER" {
			if fact.ReceiptState != "PREPARING" || (fact.State != "PREPARING" && fact.State != "CAPTURED") || fact.HeldOperation == nil || *fact.HeldOperation != fact.Operation || fact.HeldGeneration != fact.GuardGeneration {
				return errors.New("SUBMISSION_GUARD_RECOVERY_REQUIRED")
			}
		} else if fact.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Round == 0 {
````

## 第 12 步：Submissions.Ready（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 381 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;CAPTURE_FINALIZE_FACT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=16650:16798 -->
````go
			return errors.New("CAPTURE_FINALIZE_FACT_UNKNOWN")
		}
	}
	return nil
}

// submission_recovery.go：保留原实现，按业务合并文件。

````

## 第 13 步：Submissions.RecoverOne

插入位置：第 12 步代码之后，紧接追加；完成稿约第 389 行。

本步声明：`func (s *Submissions) RecoverOne(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 2500*time.Millisecond)</code>；<code>s.retrySubmission(parent, task, domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_CONTEXT_INVALID&quot;))</code>；<code>s.retrySubmission(parent, task, err)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 2500*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Capture == nil &#124;&#124; fixed.Capture.Context == nil &#124;&#124; fixed.Capture.Context.Target == nil &#124;&#124; len(fixed.Access.Proofs) == 0</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>task.Kind == &quot;CAPTURE_FINALIZE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>err == nil &amp;&amp; fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>reply == nil &#124;&#124; reply.State != pb.CaptureReply_FIXED</code>。

<!-- write-step: 13 bytes=16798:18513 -->
````go
func (s *Submissions) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimSubmissionTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if tx.Commit() != nil {
		return
	}
	var fixed submissionContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil || fixed.Capture.Context.Target == nil || len(fixed.Access.Proofs) == 0 {
		s.retrySubmission(parent, task, domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_CONTEXT_INVALID"))
		return
	}
	if task.Kind == "CAPTURE_FINALIZE" {
		err = s.finalizeSubmission(ctx, task, fixed)
	} else {
		currentProofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
		// 证明续接发生在 SQL 锁外；只接受原会话、原操作、原来源，不重新选择现在的权限。
		for _, original := range fixed.Access.Proofs {
			var current *pb.IdentityAdmission
			if current, err = s.identity.Resume(ctx, original); err != nil {
				break
			}
			currentProofs = append(currentProofs, current)
		}
		if err == nil && fixed.SessionExpires <= time.Now().UnixMilli() {
			err = domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
		}
		var reply *pb.CaptureReply
		if err == nil {
			reply, err = s.captures.Capture(ctx, fixed.Capture)
		}
		if err == nil {
			if reply == nil || reply.State != pb.CaptureReply_FIXED {
				err = domain.Violation(domain.TemporarilyUnavailable, "CAPTURE_RESULT_UNKNOWN")
			} else {
				err = s.commitSubmission(ctx, task, fixed, reply.Content, currentProofs)
			}
		}
````

## 第 14 步：Submissions.RecoverOne（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 435 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound &#124;&#124; (violation.Kind == domain.StateConflict &amp;&amp; violation.Code != &quot;SCOPE_TRANSITION_PENDING&quot;))</code>。
- 检查 <code>err != nil</code>。

<!-- write-step: 14 bytes=18513:18897 -->
````go
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || (violation.Kind == domain.StateConflict && violation.Code != "SCOPE_TRANSITION_PENDING")) {
			err = s.rejectSubmission(ctx, task, fixed, violation.Code)
		}
	}
	if err != nil {
		s.retrySubmission(parent, task, err)
	}
}

````

## 第 15 步：Submissions.retrySubmission

插入位置：第 14 步代码之后，紧接追加；完成稿约第 445 行。

本步声明：`func (s *Submissions) retrySubmission(parent context.Context, task domain.SubmissionTask, cause error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, time.Second)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>repository.RetrySubmissionTask(ctx, s.db, task, submissionRecoveryCode(cause))</code> |
| <code>cause</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>submissionRecoveryCode(cause)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 15 bytes=18897:19162 -->
````go
func (s *Submissions) retrySubmission(parent context.Context, task domain.SubmissionTask, cause error) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = repository.RetrySubmissionTask(ctx, s.db, task, submissionRecoveryCode(cause))
}

````

## 第 16 步：submissionRecoveryCode

插入位置：第 15 步代码之后，紧接追加；完成稿约第 451 行。

本步声明：`func submissionRecoveryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>errors.As(err, &amp;database)</code>；<code>status.FromError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>violation.Code</code>；<code>fmt.Sprintf(&quot;SUBMISSION_SQL_%d&quot;, database.Number)</code>；<code>&quot;SUBMISSION_RPC_&quot; + rpc.Code().String()</code>；<code>&quot;SUBMISSION_RECOVERY_UNKNOWN&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return violation.Code</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;database)</code>。条件成立时执行 <code>return fmt.Sprintf(&quot;SUBMISSION_SQL_%d&quot;, database.Number)</code>。当前函数在这里结束。
- 检查 <code>ok</code>。条件成立时执行 <code>return &quot;SUBMISSION_RPC_&quot; + rpc.Code().String()</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;SUBMISSION_RECOVERY_UNKNOWN&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=19162:19568 -->
````go
func submissionRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("SUBMISSION_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "SUBMISSION_RPC_" + rpc.Code().String()
	}
	return "SUBMISSION_RECOVERY_UNKNOWN"
}

````

## 第 17 步：Submissions.finalizeSubmission

插入位置：第 16 步代码之后，紧接追加；完成稿约第 466 行。

本步声明：`func (s *Submissions) finalizeSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>s.captures.Finalize(ctx, &amp;pb.FinalizeCaptureRequest{Context: fixed.Capture.Context, CaptureId: task.Operation, AuthoritativeResult: authority})</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.ValidateSubmissionTask(ctx, tx, task)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateSubmissionTask(ctx, tx, task)</code>；<code>repository.FinishSubmissionTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>fixed</code> | <code>submissionContext</code> | 在此声明中的实际使用：<code>fixed.Capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;CAPTURE_FINALIZE_UNKNOWN&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SUBMISSION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result == nil &#124;&#124; result.State != pb.OperationResult_SUCCEEDED &#124;&#124; result.OperationId != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;CAPTURE_FINALIZE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 17 bytes=19568:20953 -->
````go
func (s *Submissions) finalizeSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext) error {
	// 轮次已提交。收尾是系统事实核对，不再要求当事人的浏览器或旧短期证明保持在线。
	query := &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Capture.Context.Target, RequestFingerprint: fixed.Capture.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if authority.State != pb.OperationResult_SUCCEEDED {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_RESULT_UNKNOWN")
	}
	result, err := s.captures.Finalize(ctx, &pb.FinalizeCaptureRequest{Context: fixed.Capture.Context, CaptureId: task.Operation, AuthoritativeResult: authority})
	if err != nil {
		return err
	}
	if result == nil || result.State != pb.OperationResult_SUCCEEDED || result.OperationId != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "CAPTURE_FINALIZE_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishSubmissionTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// submission_result.go：保留原实现，按业务合并文件。

````

## 第 18 步：Submissions.OperationResult

插入位置：第 17 步代码之后，紧接追加；完成稿约第 499 行。

本步声明：`func (s *Submissions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.SubmissionPayload(ctx, s.db, query.OperationId)</code>；<code>repository.SubmissionResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_BINDING_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Capture == nil &#124;&#124; fixed.Capture.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Capture.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Capture.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;body) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>text != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=20953:22881 -->
````go
func (s *Submissions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	raw, err := repository.SubmissionPayload(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed submissionContext
	if json.Unmarshal(raw, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_BINDING_UNKNOWN")
	}
	if !proto.Equal(query.Target, fixed.Capture.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Capture.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.SubmissionResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	result.FixedResultJsonUtf8 = fact.Result
	digest := sha256.Sum256(fact.Result)
	result.ResultDigest = digest[:]
	result.TerminalRevision = fact.Generation
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
	} else {
		result.State = pb.OperationResult_REJECTED
	}
	var body struct {
		Result map[string]string `json:"result"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &body) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_RESULT_UNKNOWN")
	}
	for _, key := range []string{"request_id", "round_id", "content_version_id"} {
		if text := body.Result[key]; text != "" {
			id, err := domain.CanonicalID(text)
			if err != nil {
````

## 第 19 步：Submissions.OperationResult（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 544 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=22881:23026 -->
````go
				return nil, err
			}
			result.ResultIds = append(result.ResultIds, id)
		}
	}
	result.BusinessCode = body.Error.Code
	return result, nil
}

````

## 第 20 步：Submissions.Lookup

插入位置：第 19 步代码之后，紧接追加；完成稿约第 553 行。

本步声明：`func (s *Submissions) Lookup(ctx context.Context, id string) (domain.SubmissionFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.UserSubmissionRow(ctx, s.db, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.UserSubmissionRow(ctx, s.db, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.SubmissionFact</code> | 使用 <code>domain.SubmissionFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>由 <code>repository.UserSubmissionRow(ctx, s.db, id)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.UserSubmissionRow(ctx, s.db, id)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.UserSubmissionRow(ctx, s.db, id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=23026:23175 -->
````go
func (s *Submissions) Lookup(ctx context.Context, id string) (domain.SubmissionFact, error) {
	return repository.UserSubmissionRow(ctx, s.db, id)
}

````

## 第 21 步：Submissions.Progress

插入位置：第 20 步代码之后，紧接追加；完成稿约第 557 行。

本步声明：`func (s *Submissions) Progress(ctx context.Context, actor Actor, fact domain.SubmissionFact) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, fact.Document, &quot;SUBMISSION_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fact.Document, &quot;SUBMISSION_RESULT&quot;)</code> |
| <code>fact</code> | <code>domain.SubmissionFact</code> | 使用 <code>domain.SubmissionFact</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>fact.Actor</code>；<code>fact.Document</code>；<code>fact.Unit</code>；<code>fact.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;SUBMISSION_NOT_FOUND&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;SUBMISSION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;fact.Unit&#93;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!result.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=23175:24307 -->
````go
func (s *Submissions) Progress(ctx context.Context, actor Actor, fact domain.SubmissionFact) ([]*pb.SourceEpoch, ReadBody, error) {
	if fact.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "SUBMISSION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fact.Document, "SUBMISSION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Edit[fact.Unit] {
		return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		result, err := repository.SubmissionResult(ctx, tx, fact.Operation)
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
		return value, nil
	}
	return access.Sources, run, nil
}

// submissions.go：保留原实现，按业务合并文件。

````

## 第 22 步：SubmissionCaptureClient

插入位置：第 21 步代码之后，紧接追加；完成稿约第 590 行。

本步声明：`type SubmissionCaptureClient interface {`。

**这一段为什么这样写：** 送审先取得可靠固定正文，再由Doc事务创建审核轮次，最后结算Node采样。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Capture</code> | <code>(context.Context, *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error)</code> | ctx与送审采样请求输入，首次固定cut及正文；相同操作后续复用这份采样。 |
| <code>Finalize</code> | <code>(context.Context, *pb.FinalizeCaptureRequest) (*pb.OperationResult, error)</code> | ctx与原采样结算请求输入，将已被业务提交/拒绝的采样收口，返回原操作结果。 |

<!-- write-step: 22 bytes=24307:24517 -->
````go
type SubmissionCaptureClient interface {
	Capture(context.Context, *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error)
	Finalize(context.Context, *pb.FinalizeCaptureRequest) (*pb.OperationResult, error)
}

````

## 第 23 步：Submissions

插入位置：第 22 步代码之后，紧接追加；完成稿约第 595 行。

本步声明：`type Submissions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>captures</code> | <code>SubmissionCaptureClient</code> | 使用 <code>SubmissionCaptureClient</code> 类型；其定义处的职责：送审先取得可靠固定正文，再由Doc事务创建审核轮次，最后结算Node采样。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 23 bytes=24517:24686 -->
````go
type Submissions struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	captures  SubmissionCaptureClient
	validator *codec.Validator
	instance  string
}

````

## 第 24 步：submissionContext

插入位置：第 23 步代码之后，紧接追加；完成稿约第 604 行。

持久上下文只含已核验事实；浏览器 token 不进入 JSON，也不能供后台重新选择权限。

本步声明：`type submissionContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Intent</code> | <code>domain.SubmissionIntent</code> | 使用 <code>domain.SubmissionIntent</code> 类型；其定义处的职责：SubmissionIntent是从生成union DTO解析的业务身份；保留规范公开输入，禁止浏览器提供正文或内部切点。<br>序列化/校验标记：<code>`json:&quot;intent&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Capture</code> | <code>*pb.CaptureSubmissionRequest</code> | 序列化/校验标记：<code>`json:&quot;capture&quot;`</code> |
| <code>Workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>序列化/校验标记：<code>`json:&quot;workspace&quot;`</code> |
| <code>Room</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;room&quot;`</code> |
| <code>LineageProvenance</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;lineage_provenance&quot;`</code> |
| <code>Style</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;style&quot;`</code> |

<!-- write-step: 24 bytes=24686:25433 -->
````go
// 持久上下文只含已核验事实；浏览器 token 不进入 JSON，也不能供后台重新选择权限。
type submissionContext struct {
	Actor             uint64                       `json:"actor"`
	SessionExpires    int64                        `json:"session_expires"`
	Intent            domain.SubmissionIntent      `json:"intent"`
	Access            DocumentAccess               `json:"access"`
	Capture           *pb.CaptureSubmissionRequest `json:"capture"`
	Workspace         domain.EditWorkspace         `json:"workspace"`
	Room              uint64                       `json:"room"`
	LineageProvenance []byte                       `json:"lineage_provenance"`
	Style             string                       `json:"style"`
}

````

## 第 25 步：NewSubmissions

插入位置：第 24 步代码之后，紧接追加；完成稿约第 617 行。

本步声明：`func NewSubmissions(db *sql.DB, identity *Identity, access *Access, captures SubmissionCaptureClient, validator *codec.Validator, instance string) *Submissions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>captures</code> | <code>SubmissionCaptureClient</code> | 使用 <code>SubmissionCaptureClient</code> 类型；其定义处的职责：送审先取得可靠固定正文，再由Doc事务创建审核轮次，最后结算Node采样。<br>在此声明中的实际使用：<code>captures: captures</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Submissions</code> | 实际返回表达式：<code>&amp;Submissions{db: db, identity: identity, access: access, captures: captures, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Submissions{db: db, identity: identity, access: access, captures: captures, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=25433:25724 -->
````go
func NewSubmissions(db *sql.DB, identity *Identity, access *Access, captures SubmissionCaptureClient, validator *codec.Validator, instance string) *Submissions {
	return &Submissions{db: db, identity: identity, access: access, captures: captures, validator: validator, instance: instance}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/submissions_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
