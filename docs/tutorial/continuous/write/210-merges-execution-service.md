# 分步编写 `services/document/internal/logic/document/merges_execution_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/210-merges-execution-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

执行合并计算、冲突落定、最终主线提交与安装收尾。

计算期间不长期锁审核请求，最终提交必须再次确认轮次、意见、来源与主线屏障。

写之前先按这条执行路线理解各段的连接：

1. recoverMerge恢复固定输入并计算
2. persistMergeConflicts记录确定冲突
3. commitMerge原子写批准/版本/清单/lineage
4. reconcileMain核完整安装ACK后解除屏障。

本文件需要守住的边界：

- 只有SUCCEEDED枚举不足够，validMainInstallReply还核原房间、epoch、完整清单与零切点
- 过期worker不能完成新代次。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/merges_execution_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/merges_execution_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/merges_execution_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/merges_execution_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:574 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// merge_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：mainInstallContext

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type mainInstallContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Generation</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;manifest&quot;`</code> |
| <code>Units</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 序列化/校验标记：<code>`json:&quot;units&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;digest&quot;`</code> |
| <code>Lineage</code> | <code>map&#91;string&#93;any</code> | 序列化/校验标记：<code>`json:&quot;lineage&quot;`</code> |
| <code>Structure</code> | <code>uint64</code> | 新清单安装绑定的结构版本。 |
| <code>Style</code> | <code>uint64</code> | 新清单安装绑定的样式版本。 |
| <code>Control</code> | <code>uint64</code> | 此次安装绑定的控制上下文值，Node据此拒绝旧安装覆盖新状态。 |
| <code>Entity</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;entity,omitempty&quot;`</code> |
| <code>Transition</code> | <code>*structureInstallation</code> | 序列化/校验标记：<code>`json:&quot;structure_transition,omitempty&quot;`</code> |

<!-- write-step: 2 bytes=574:1201 -->
````go
type mainInstallContext struct {
	Context                   *pb.OperationContext `json:"context"`
	Generation                uint64               `json:"generation"`
	Manifest                  uint64               `json:"manifest"`
	Units                     []*pb.ManifestUnit   `json:"units"`
	Digest                    []byte               `json:"digest"`
	Lineage                   map[string]any       `json:"lineage"`
	Structure, Style, Control uint64
	Entity                    uint64                 `json:"entity,omitempty"`
	Transition                *structureInstallation `json:"structure_transition,omitempty"`
}

````

## 第 3 步：Merges.commitMerge

插入位置：第 2 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func (s *Merges) commitMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, inputs mergeFixedInputs, candidate *pb.MergeCandidateReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 19 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteMerge(ctx, tx, task, pointer, fixed.Actor, record, manifest, attempt.ID, attempt.Generation, result, install)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Pointer</code>；<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>inputs</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>inputs.Attempt</code>；<code>inputs.Input</code>；<code>inputs.Main</code>；<code>s.acceptMergeLineage(ctx, tx, fixed, inputs, workspace, record, manifest)</code> |
| <code>candidate</code> | <code>*pb.MergeCandidateReply</code> | 在此声明中的实际使用：<code>candidate.GetCandidate</code>；<code>candidate == nil</code>；<code>candidate.State</code>；<code>candidate.Conflicts</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>*pb.MainBarrierReply</code> | 在此声明中的实际使用：<code>prepared == nil</code>；<code>prepared.State</code>；<code>prepared.OperationId</code>；<code>prepared.BarrierGeneration</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMergeProofs(fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_BINDING_INVALID&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMISSION_CUT_CHANGED&quot;)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>candidate == nil &#124;&#124; candidate.State != pb.CaptureReply_FIXED &#124;&#124; len(candidate.Conflicts) != 0 &#124;&#124; !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) &#124;&#124; content == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>content.OperationId != task.Operation &#124;&#124; content.CaptureId != task.Operation+&quot;-merge-&quot;+domain.TextID(attempt.ID) &#124;&#124; content.BranchId != fixed.Pointer.Branch &#124;&#124; content.WorkspaceEpo …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(content.CutJsonUtf8, inputs.Input.Submitted.CutJsonUtf8) &#124;&#124; !validProjectedSubmissionCut(content.CutJsonUtf8, attempt)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_SUBMISSION_CUT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepared == nil &#124;&#124; prepared.State != pb.MainBarrierReply_PREPARED &#124;&#124; prepared.OperationId != task.Operation &#124;&#124; prepared.BarrierGeneration != attempt.Generation &#124;&#124; !proto.Equal(prep …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(attempt.Main.ExpectedManifestId) &#124;&#124; current.CurrentStructureId != domain.TextID(attempt.Main.ExpectedStructureId) &#124;&#124; current.CurrentStyle …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=1201:3959 -->
````go
func (s *Merges) commitMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, inputs mergeFixedInputs, candidate *pb.MergeCandidateReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	attempt := inputs.Attempt
	content := candidate.GetCandidate()
	if candidate == nil || candidate.State != pb.CaptureReply_FIXED || len(candidate.Conflicts) != 0 || !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) || content == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_UNKNOWN")
	}
	if err := validateFixedMergeContent(s.validator, content, fixed.Pointer.Document, fixed.Pointer.Unit, attempt.Main.ExpectedStructureId); err != nil {
		return err
	}
	if content.OperationId != task.Operation || content.CaptureId != task.Operation+"-merge-"+domain.TextID(attempt.ID) || content.BranchId != fixed.Pointer.Branch || content.WorkspaceEpoch != fixed.Context.Target.WorkspaceEpoch || content.LineageRef != domain.TextID(attempt.Round.Lineage) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_BINDING_INVALID")
	}
	if !bytes.Equal(content.CutJsonUtf8, inputs.Input.Submitted.CutJsonUtf8) || !validProjectedSubmissionCut(content.CutJsonUtf8, attempt) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMISSION_CUT_CHANGED")
	}
	if err := validateMergeCandidateProvenance(content, attempt); err != nil {
		return err
	}
	cutBytes, err := codec.Canonical(domain.CutBinding(inputs.Main.Cut))
	if err != nil {
		return err
	}
	cutDigest := sha256.Sum256(cutBytes)
	if prepared == nil || prepared.State != pb.MainBarrierReply_PREPARED || prepared.OperationId != task.Operation || prepared.BarrierGeneration != attempt.Generation || !proto.Equal(prepared.PreparedCut, inputs.Main.Cut) || !bytes.Equal(prepared.Digest, cutDigest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
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
	if current.CurrentManifestId != domain.TextID(attempt.Main.ExpectedManifestId) || current.CurrentStructureId != domain.TextID(attempt.Main.ExpectedStructureId) || current.CurrentStyleId != domain.TextID(attempt.Main.ExpectedStyleId) || current.ControlRevision != domain.TextID(attempt.Main.ExpectedControlRevision) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
````

## 第 4 步：Merges.commitMerge（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 80 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != task.Operation &#124;&#124; barrier.Generation != attempt.Generation &#124;&#124; barrier.Phase != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; branch != fixed.Pointer.Branch</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; workspace.Epoch != fixed.Context.Target.WorkspaceEpoch</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;WORKSPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>latest.ID != attempt.ID &#124;&#124; latest.State != &quot;FIXED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_ATTEMPT_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(entries) != len(inputs.Main.Units)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=3959:5829 -->
````go
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != attempt.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	branch, err := repository.LockedActiveBranch(ctx, tx, fixed.Pointer.Unit, fixed.Pointer.Document)
	if err != nil || branch != fixed.Pointer.Branch {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedEditWorkspace(ctx, tx, branch, fixed.Pointer.Document, fixed.Pointer.Unit)
	if err != nil || workspace.Epoch != fixed.Context.Target.WorkspaceEpoch {
		return domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	pointer, err := validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return err
	}
	latest, err := repository.LatestMergeAttempt(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if latest.ID != attempt.ID || latest.State != "FIXED" {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_ATTEMPT_SUPERSEDED")
	}
	entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, pointer.Document)
	if err != nil {
		return err
	}
	if len(entries) != len(inputs.Main.Units) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	main := map[uint64]*pb.FixedContent{}
	for _, unit := range inputs.Main.Units {
		main[unit.UnitId] = unit
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	// candidate 和 MAIN 可靠尾部引用精确当前 Doc 的计费分配，保留原 resourceId。
	contents := append(append([]*pb.FixedContent(nil), inputs.Main.Units...), content)
````

## 第 5 步：Merges.commitMerge（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 125 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>captured == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>unit == pointer.Unit</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>bytes.Equal(old.BodyHash, captured.BodyHash)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=5829:7532 -->
````go
	if err = lockFixedContentResources(ctx, tx, fixed.Pointer.Document, space, contents, "MERGE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	version, err := repository.InsertMergeVersion(ctx, tx, content, "MERGE", content.CaptureId)
	if err != nil {
		return err
	}
	manifest, err := repository.InsertMergedManifest(ctx, tx, pointer.Document, attempt.Main.ExpectedManifestId, attempt.Main.ExpectedStructureId, attempt.Main.ExpectedStyleId, task.Operation, attemptKey(task.Operation, attempt.ID))
	if err != nil {
		return err
	}
	control := attempt.Main.ExpectedControlRevision + 1
	units := make([]*pb.ManifestUnit, 0, len(entries))
	for _, entry := range entries {
		unit, _ := domain.CanonicalID(entry["unit_id"])
		oldVersion, _ := domain.CanonicalID(entry["content_version_id"])
		captured := main[unit]
		if captured == nil {
			return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
		}
		chosen := oldVersion
		var body *pb.FixedContent
		if unit == pointer.Unit {
			chosen, body = version, proto.Clone(content).(*pb.FixedContent)
		} else {
			old, err := repository.VersionContent(ctx, tx, oldVersion, pointer.Document)
			if err != nil {
				return err
			}
			if bytes.Equal(old.BodyHash, captured.BodyHash) {
				if err = canonicalVersion(old, oldVersion); err != nil {
					return err
				}
				body = old
			} else {
				chosen, err = repository.InsertMergeVersion(ctx, tx, captured, "MAIN_CAPTURE", captured.CaptureId)
				if err != nil {
					return err
				}
				body = proto.Clone(captured).(*pb.FixedContent)
			}
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, pointer.Document, manifest, unit, chosen); err != nil {
			return err
		}
````

## 第 6 步：Merges.commitMerge（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 170 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 6 bytes=7532:9687 -->
````go
		room, err := collaborationRoom(ctx, tx, "MAIN", unit, unit, control)
		if err != nil {
			return err
		}
		body.BranchId, body.WorkspaceEpoch = 0, control
		body.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(control), "durable_seq": "0"})
		if err != nil {
			return err
		}
		units = append(units, &pb.ManifestUnit{UnitId: unit, ContentVersionId: chosen, Content: body})
	}
	record, err := repository.InsertMergeRecord(ctx, tx, task.Operation, pointer, attempt.ID, attempt.Main.ExpectedManifestId, manifest, version, attempt.Round.Lineage, attempt.Round.Capture)
	if err != nil {
		return err
	}
	if _, err = repository.InsertMergeApproval(ctx, tx, task.Operation, pointer, fixed.Actor, fixed.Intent.Version, record); err != nil {
		return err
	}
	for i, resolution := range fixed.Intent.Resolutions {
		if err = repository.InsertMergedResolution(ctx, tx, task.Operation, pointer, fixed.Actor, resolution.ID, i+1); err != nil {
			return err
		}
	}
	lineage, err := s.acceptMergeLineage(ctx, tx, fixed, inputs, workspace, record, manifest)
	if err != nil {
		return err
	}
	digest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: attempt.Generation, Manifest: manifest, Units: units, Digest: digest[:], Lineage: lineage, Structure: attempt.Main.ExpectedStructureId, Style: attempt.Main.ExpectedStyleId, Control: control})
	if err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"merge_record_id": domain.TextID(record), "manifest_id": domain.TextID(manifest), "content_version_id": domain.TextID(version), "request_id": domain.TextID(pointer.ID), "round_id": domain.TextID(pointer.Round), "main_sync_state": "SYNCING"}}
	if err = repository.CompleteMerge(ctx, tx, task, pointer, fixed.Actor, record, manifest, attempt.ID, attempt.Generation, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// merge_install.go：保留原实现，按业务合并文件。

````

## 第 7 步：Merges.CommittedSource

插入位置：第 6 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`func (s *Merges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;MAIN_COMMIT_FIXED&quot;, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>query.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;generation&quot;: domain.TextID(fixed.Generation), &quot;manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;: domain.TextI …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_NOT_TERMINAL&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>result.State != pb.OperationResult_SUCCEEDED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_NOT_TERMINAL&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;any{&quot;generation&quot;: domain.TextID(fixed.Generation), &quot;manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;: domai …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=9687:11035 -->
````go
func (s *Merges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	result, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.State != pb.OperationResult_SUCCEEDED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_NOT_TERMINAL")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_COMMIT_FIXED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed mainInstallContext
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_BINDING_UNKNOWN")
	}
	units := make([]any, 0, len(fixed.Units))
	for _, unit := range fixed.Units {
		content, err := fixedWire(unit.Content)
		if err != nil {
			return nil, err
		}
		units = append(units, map[string]any{"unit_id": domain.TextID(unit.UnitId), "content_version_id": domain.TextID(unit.ContentVersionId), "content": content})
	}
	return map[string]any{"generation": domain.TextID(fixed.Generation), "manifest_id": domain.TextID(fixed.Manifest), "structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style), "control_revision": domain.TextID(fixed.Control), "manifest_digest": base64.StdEncoding.EncodeToString(fixed.Digest), "units": units, "lineage": fixed.Lineage}, nil
}

````

## 第 8 步：Merges.reconcileMain

插入位置：第 7 步代码之后，紧接追加；完成稿约第 241 行。

本步声明：`func (s *Merges) reconcileMain(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>s.client.ReconcileMain(ctx, &amp;pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, Uni …</code>（长表达式，按本步完整代码继续）；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_INSTALL_RESULT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != pb.OperationResult_SUCCEEDED &amp;&amp; result.State != pb.OperationResult_REJECTED &amp;&amp; result.State != pb.OperationResult_CANCELLED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Context.OperationId != task.Operation &#124;&#124; !validMainInstallReply(fixed, result.State == pb.OperationResult_SUCCEEDED, reply)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_INSTALL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 8 bytes=11035:12783 -->
````go
func (s *Merges) reconcileMain(ctx context.Context, task domain.SubmissionTask) error {
	var fixed mainInstallContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RECONCILE_BINDING_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Context.Target, RequestFingerprint: fixed.Context.RequestFingerprint}
	result, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if result.State != pb.OperationResult_SUCCEEDED && result.State != pb.OperationResult_REJECTED && result.State != pb.OperationResult_CANCELLED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RESULT_UNKNOWN")
	}
	reply, err := s.client.ReconcileMain(ctx, &pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, Units: fixed.Units, ManifestDigest: fixed.Digest})
	if err != nil {
		return err
	}
	if fixed.Context.OperationId != task.Operation || !validMainInstallReply(fixed, result.State == pb.OperationResult_SUCCEEDED, reply) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_INSTALL_RESULT_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply); err != nil {
		return err
	}
	return tx.Commit()
}

// merge_install_ack.go：保留原实现，按业务合并文件。

````

## 第 9 步：validMainInstallReply

插入位置：第 8 步代码之后，紧接追加；完成稿约第 280 行。

安装成功枚举不足以解除屏障：必须逐项确认原完整清单安装到原房间代次与零切点。

本步声明：`func validMainInstallReply(fixed mainInstallContext, succeeded bool, reply *pb.InstallReply) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Generation</code>；<code>fixed.Units</code>；<code>fixed.Transition</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>succeeded</code> | <code>bool</code> | 在此声明中的实际使用：<code>!succeeded</code> |
| <code>reply</code> | <code>*pb.InstallReply</code> | 在此声明中的实际使用：<code>reply == nil</code>；<code>reply.OperationId</code>；<code>reply.BarrierGeneration</code>；<code>reply.State</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>reply.State == pb.InstallReply_ABORTED &amp;&amp; reply.ManifestId == 0 &amp;&amp; len(reply.InstalledEpochs) == 0 &amp;&amp; len(reply.ManifestDigest) == 0</code>；<code>len(expected) == 0</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; reply == nil &#124;&#124; reply.OperationId != fixed.Context.OperationId &#124;&#124; reply.BarrierGeneration != fixed.Generation</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!succeeded</code>。条件成立时执行 <code>return reply.State == pb.InstallReply_ABORTED &amp;&amp; reply.ManifestId == 0 &amp;&amp; len(reply.InstalledEpochs) == 0 &amp;&amp; len(reply.ManifestDigest) == 0</code>。当前函数在这里结束。
- 检查 <code>fixed.Transition != nil</code>。
- 检查 <code>reply.State != pb.InstallReply_READY &#124;&#124; reply.ManifestId != fixed.Manifest &#124;&#124; len(fixed.Digest) != 32 &#124;&#124; 		!bytes.Equal(reply.ManifestDigest, fixed.Digest) &#124;&#124; len(reply.InstalledEp …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>unit == nil &#124;&#124; unit.Content == nil &#124;&#124; json.Unmarshal(unit.Content.CutJsonUtf8, &amp;cut) != nil &#124;&#124; 			cut&#91;&quot;room_id&quot;&#93; == &quot;&quot; &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; == &quot;&quot; &#124;&#124; cut&#91;&quot;durable_seq&quot;&#93; != &quot;0&quot; &#124;&#124; expected&#91;cut&#91;&quot;room_id&quot;&#93;&#93; != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>fixed.Transition != nil</code>。
- 检查 <code>branch.Content == nil &#124;&#124; json.Unmarshal(branch.Content.CutJsonUtf8, &amp;cut) != nil &#124;&#124; cut&#91;&quot;room_id&quot;&#93; != domain.TextID(branch.Room) &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(branch.E …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>installed == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(installed.WorkspaceEpoch) &#124;&#124; installed.DurableSeq != 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=12783:14920 -->
````go
// 安装成功枚举不足以解除屏障：必须逐项确认原完整清单安装到原房间代次与零切点。
func validMainInstallReply(fixed mainInstallContext, succeeded bool, reply *pb.InstallReply) bool {
	if fixed.Context == nil || reply == nil || reply.OperationId != fixed.Context.OperationId || reply.BarrierGeneration != fixed.Generation {
		return false
	}
	if !succeeded {
		return reply.State == pb.InstallReply_ABORTED && reply.ManifestId == 0 && len(reply.InstalledEpochs) == 0 && len(reply.ManifestDigest) == 0
	}
	expectedCount := len(fixed.Units)
	if fixed.Transition != nil {
		expectedCount += len(fixed.Transition.Branches)
	}
	if reply.State != pb.InstallReply_READY || reply.ManifestId != fixed.Manifest || len(fixed.Digest) != 32 ||
		!bytes.Equal(reply.ManifestDigest, fixed.Digest) || len(reply.InstalledEpochs) != expectedCount || (len(fixed.Units) == 0 && !validEmptyControlledManifest(fixed)) {
		return false
	}
	expected := map[string]map[string]string{}
	for _, unit := range fixed.Units {
		var cut map[string]string
		if unit == nil || unit.Content == nil || json.Unmarshal(unit.Content.CutJsonUtf8, &cut) != nil ||
			cut["room_id"] == "" || cut["workspace_epoch"] == "" || cut["durable_seq"] != "0" || expected[cut["room_id"]] != nil {
			return false
		}
		expected[cut["room_id"]] = cut
	}
	if fixed.Transition != nil {
		for _, branch := range fixed.Transition.Branches {
			var cut map[string]string
			if branch.Content == nil || json.Unmarshal(branch.Content.CutJsonUtf8, &cut) != nil || cut["room_id"] != domain.TextID(branch.Room) || cut["workspace_epoch"] != domain.TextID(branch.Epoch) || cut["durable_seq"] != "0" || branch.Room == 0 || branch.Epoch == 0 || expected[cut["room_id"]] != nil {
				return false
			}
			expected[cut["room_id"]] = cut
		}
	}
	for _, installed := range reply.InstalledEpochs {
		if installed == nil {
			return false
		}
		key := domain.TextID(installed.RoomId)
		cut, exists := expected[key]
		if !exists || cut["workspace_epoch"] != domain.TextID(installed.WorkspaceEpoch) || installed.DurableSeq != 0 {
			return false
		}
		delete(expected, key)
	}
````

## 第 10 步：validMainInstallReply（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 325 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return len(expected) == 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=14920:15017 -->
````go
	return len(expected) == 0
}

// merge_recovery.go：保留原实现，按业务合并文件。

````

## 第 11 步：Merges.RecoverOne

插入位置：第 10 步代码之后，紧接追加；完成稿约第 330 行。

本步声明：`func (s *Merges) RecoverOne(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 15*time.Second)</code>；<code>context.WithTimeout(parent, time.Second)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 15*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>task.Kind == &quot;MAIN_RECONCILE&quot;</code>。
- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound &#124;&#124; violation.Kind == domain.StateConflict)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 11 bytes=15017:16132 -->
````go
func (s *Merges) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimMergeTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "MAIN_RECONCILE" {
		err = s.reconcileMain(ctx, task)
	} else {
		var fixed mergeContext
		if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
			err = domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
		} else {
			err = s.recoverMerge(ctx, task, fixed)
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				err = s.rejectMerge(ctx, task, fixed, violation.Code)
			}
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mergeRecoveryCode(err))
	}
}

````

## 第 12 步：Merges.recoverMerge

插入位置：第 11 步代码之后，紧接追加；完成稿约第 366 行。

本步声明：`func (s *Merges) recoverMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>s.ensureSubmissionCut(ctx, task, fixed)</code>；<code>s.reserveAttempt(ctx, task, fixed, proofs)</code>；<code>s.fixInputs(ctx, task, fixed, attempt)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.ensureSubmissionCut(ctx, task, fixed)</code>；<code>s.reserveAttempt(ctx, task, fixed, proofs)</code>；<code>s.fixInputs(ctx, task, fixed, attempt)</code>；<code>s.persistMergeConflicts(ctx, task, attempt, candidate)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>s.ensureSubmissionCut(ctx, task, fixed)</code>；<code>s.reserveAttempt(ctx, task, fixed, proofs)</code>；<code>s.fixInputs(ctx, task, fixed, attempt)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MERGE_CONFLICT_REQUIRES_RESOLUTION&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>candidate == nil &#124;&#124; candidate.State != pb.CaptureReply_FIXED &#124;&#124; !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MERGE_CANDIDATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(candidate.Conflicts) != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;MERGE_CONFLICT_REQUIRES_RESOLUTION&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepared == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State == pb.MainBarrierReply_CHANGED</code>。条件成立时执行 <code>return s.supersedeAttempt(ctx, task, fixed, attempt)</code>。当前函数在这里结束。
- 检查 <code>prepared.State != pb.MainBarrierReply_PREPARED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.commitMerge(ctx, task, fixed, inputs, candidate, prepared, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=16132:17733 -->
````go
func (s *Merges) recoverMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	if err = s.ensureSubmissionCut(ctx, task, fixed); err != nil {
		return err
	}
	attempt, err := s.reserveAttempt(ctx, task, fixed, proofs)
	if err != nil {
		return err
	}
	inputs, err := s.fixInputs(ctx, task, fixed, attempt)
	if err != nil {
		return err
	}
	candidate, err := s.client.Compute(ctx, inputs.Input)
	if err != nil {
		return err
	}
	if candidate == nil || candidate.State != pb.CaptureReply_FIXED || !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_UNKNOWN")
	}
	if len(candidate.Conflicts) != 0 {
		if err = s.persistMergeConflicts(ctx, task, attempt, candidate); err != nil {
			return err
		}
		return domain.Violation(domain.StateConflict, "MERGE_CONFLICT_REQUIRES_RESOLUTION")
	}
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: fixed.Context, BarrierGeneration: attempt.Generation, ExpectedCut: inputs.Main.Cut})
	if err != nil {
		return err
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return s.supersedeAttempt(ctx, task, fixed, attempt)
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commitMerge(ctx, task, fixed, inputs, candidate, prepared, proofs)
}

````

## 第 13 步：Merges.supersedeAttempt

插入位置：第 12 步代码之后，紧接追加；完成稿约第 411 行。

本步声明：`func (s *Merges) supersedeAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Pointer</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.Generation</code>；<code>attempt.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CUT_CHANGED_RETRY&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != task.Operation &#124;&#124; barrier.Generation != attempt.Generation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CUT_CHANGED_RETRY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=17733:18997 -->
````go
func (s *Merges) supersedeAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) error {
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
	if _, err = ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != attempt.Generation {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 只有Node明确CHANGED（未准备）才允许下一次尝试重新采尾；超时保持原attempt与屏障。
	if err = repository.SupersedeMergeAttempt(ctx, tx, task.Operation, attempt.ID, fixed.Pointer.Document, attempt.Generation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CUT_CHANGED_RETRY")
}

````

## 第 14 步：Merges.persistMergeConflicts

插入位置：第 13 步代码之后，紧接追加；完成稿约第 446 行。

本步声明：`func (s *Merges) persistMergeConflicts(ctx context.Context, task domain.SubmissionTask, attempt mergeAttemptContext, candidate *pb.MergeCandidateReply) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.StoreMergeConflict(ctx, tx, attempt.ID, raw)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>attempt</code> | <code>mergeAttemptContext</code> | 在此声明中的实际使用：<code>attempt.ID</code> |
| <code>candidate</code> | <code>*pb.MergeCandidateReply</code> | 在此声明中的实际使用：<code>candidate.Conflicts</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 14 bytes=18997:19623 -->
````go
func (s *Merges) persistMergeConflicts(ctx context.Context, task domain.SubmissionTask, attempt mergeAttemptContext, candidate *pb.MergeCandidateReply) error {
	raw, err := json.Marshal(candidate.Conflicts)
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
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.StoreMergeConflict(ctx, tx, attempt.ID, raw); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 15 步：Merges.rejectMerge

插入位置：第 14 步代码之后，紧接追加；完成稿约第 468 行。

本步声明：`func (s *Merges) rejectMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Pointer.Document, true)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.RejectMerge(ctx, tx, task, fixed.Pointer.ID, code, result)</code> |
| <code>fixed</code> | <code>mergeContext</code> | 在此声明中的实际使用：<code>fixed.Pointer</code>；<code>fixed.Context</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code>；<code>repository.RejectMerge(ctx, tx, task, fixed.Pointer.ID, code, result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; attempt.State == &quot;CONFLICT&quot;</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation == task.Operation &amp;&amp; barrier.Phase != &quot;OPEN&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=19623:21379 -->
````go
func (s *Merges) rejectMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	// 原来源已关闭时也必须完成原号拒绝；这条事务只撤销自己的预约，不写正文/意见/批准。
	if _, err = repository.DocumentRow(ctx, tx, fixed.Pointer.Document, true); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if _, err = repository.LockedReviewGuard(ctx, tx, fixed.Pointer.Unit); err != nil {
		return err
	}
	if _, err = repository.LockedReviewPointer(ctx, tx, fixed.Pointer.ID); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 持久终态也使用公开Error契约。原操作号同时作为追踪标识，重试读取不会产生另一份错误事实。
	result := map[string]any{
		"operation_id": task.Operation,
		"state":        "REJECTED",
		"error": map[string]string{
			"code": code, "message": code,
			"trace_id": task.Operation, "retry_action": "NONE",
		},
	}
	if attempt, err := repository.LatestMergeAttempt(ctx, tx, task.Operation); err == nil && attempt.State == "CONFLICT" {
		result["result"] = map[string]string{"conflict_context_ref": task.Operation}
	} else if err != nil && err != sql.ErrNoRows {
		return err
	}
	if barrier.Operation == task.Operation && barrier.Phase != "OPEN" {
		payload, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: barrier.Generation})
		if err != nil {
			return err
		}
````

## 第 16 步：Merges.rejectMerge（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 513 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 16 bytes=21379:21668 -->
````go
		if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_RECONCILE", task.Operation, task.Operation, payload, "QUEUED"); err != nil {
			return err
		}
	}
	if err = repository.RejectMerge(ctx, tx, task, fixed.Pointer.ID, code, result); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 17 步：mergeRecoveryCode

插入位置：第 16 步代码之后，紧接追加；完成稿约第 523 行。

本步声明：`func mergeRecoveryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>status.FromError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>violation.Code</code>；<code>&quot;MERGE_RPC_&quot; + rpc.Code().String()</code>；<code>&quot;MERGE_RECOVERY_UNAVAILABLE&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return violation.Code</code>。当前函数在这里结束。
- 检查 <code>ok</code>。条件成立时执行 <code>return &quot;MERGE_RPC_&quot; + rpc.Code().String()</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;MERGE_RECOVERY_UNAVAILABLE&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=21668:21936 -->
````go
func mergeRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	if rpc, ok := status.FromError(err); ok {
		return "MERGE_RPC_" + rpc.Code().String()
	}
	return "MERGE_RECOVERY_UNAVAILABLE"
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/merges_execution_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
