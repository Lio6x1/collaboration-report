# 分步编写 `services/document/internal/logic/document/structure_previews_service.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/240-structure-previews-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。

预览报告问题供用户判断；确认与提交则必须拒绝阻塞单元，不能自动撤回审核。

写之前先按这条执行路线理解各段的连接：

1. Register固定真实unit/branch/epoch
2. structurePreviewBlockingUnits列出送审和审核阻塞
3. captureAndCommit取候选
4. validateStructurePreviewSnapshot核内容/来源
5. Read输出固定预览。

本文件需要守住的边界：

- 断连和未知RPC不能当确定失败
- POST给完整预览，通用GET只投影公开状态
- 领取次数不得改原绑定。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/structure_previews_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/structure_previews_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/structure_previews_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/structure_previews_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:737 -->
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
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// structure_preview_guards.go：保留原实现，按业务合并文件。

````

## 第 2 步：validateStructurePreviewUnits

插入位置：第 1 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func validateStructurePreviewUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.CurrentStructurePreviewUnits(ctx, tx, fixed.Document, fixed.Structure)</code>；<code>repository.SubmissionWorkspace(ctx, tx, actual.Branch, fixed.Document, actual.Unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.CurrentStructurePreviewUnits(ctx, tx, fixed.Document, fixed.Structure)</code>；<code>repository.SubmissionWorkspace(ctx, tx, actual.Branch, fixed.Document, actual.Unit)</code> |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Structure</code>；<code>fixed.Units</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_SCOPE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_BRANCH_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(currentUnits) != len(fixed.Units)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_SCOPE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>actual.Unit != unit.Unit &#124;&#124; actual.Chapter != unit.Chapter &#124;&#124; actual.Branch != unit.Workspace.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_SCOPE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; workspace != unit.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=737:1610 -->
````go
func validateStructurePreviewUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) error {
	currentUnits, err := repository.CurrentStructurePreviewUnits(ctx, tx, fixed.Document, fixed.Structure)
	if err != nil {
		return err
	}
	if len(currentUnits) != len(fixed.Units) {
		return domain.Violation(domain.StateConflict, "STRUCTURE_SCOPE_CHANGED")
	}
	for i, unit := range fixed.Units {
		actual := currentUnits[i]
		if actual.Unit != unit.Unit || actual.Chapter != unit.Chapter || actual.Branch != unit.Workspace.Workspace {
			return domain.Violation(domain.StateConflict, "STRUCTURE_SCOPE_CHANGED")
		}
		workspace, err := repository.SubmissionWorkspace(ctx, tx, actual.Branch, fixed.Document, actual.Unit)
		if err != nil || workspace != unit.Workspace {
			return domain.Violation(domain.StateConflict, "STRUCTURE_BRANCH_CHANGED")
		}
	}
	return nil
}

````

## 第 3 步：structurePreviewBlockingUnits

插入位置：第 2 步代码之后，紧接追加；完成稿约第 49 行。

预览报告阻止单元，确认/最终提交则须拒绝。两者使用同一当前guard规则，不撤回或归档原请求。

本步声明：`func structurePreviewBlockingUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) ([]map[string]string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit.Unit)</code>；<code>repository.LockedReviewChanges(ctx, tx, guard.Resumable)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit.Unit)</code>；<code>repository.LockedReviewChanges(ctx, tx, guard.Resumable)</code> |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.Units</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 实际返回表达式：<code>nil</code>；<code>blocks</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>change.Type == &quot;SPLIT_LEVEL2&quot; &#124;&#124; change.Type == &quot;JOIN_CHAPTER&quot; &#124;&#124; change.Type == &quot;REMOVE&quot;</code>。
- 检查 <code>!affected&#91;unit.Chapter&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>guard.Submission != &quot;&quot;</code>。
- 检查 <code>guard.Ongoing != 0</code>。
- 检查 <code>guard.Resumable != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>change.Status == &quot;OPEN&quot;</code>。
- 检查 <code>reason != &quot;&quot;</code>。
- 此处通过 <code>return blocks, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1610:3011 -->
````go
// 预览报告阻止单元，确认/最终提交则须拒绝。两者使用同一当前guard规则，不撤回或归档原请求。
func structurePreviewBlockingUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) ([]map[string]string, error) {
	blocks := []map[string]string{}
	affected := map[uint64]bool{}
	for _, change := range fixed.Intent.Input.Changes {
		if change.Type == "SPLIT_LEVEL2" || change.Type == "JOIN_CHAPTER" || change.Type == "REMOVE" {
			id, _ := domain.CanonicalID(change.ChapterId)
			affected[id] = true
		}
	}
	for _, unit := range fixed.Units {
		if !affected[unit.Chapter] {
			continue
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, unit.Unit)
		if err != nil {
			return nil, err
		}
		reason := ""
		if guard.Submission != "" {
			reason = "SUBMISSION_PENDING"
		} else if guard.Ongoing != 0 {
			reason = "ACTIVE_REVIEW"
		} else if guard.Resumable != 0 {
			changes, err := repository.LockedReviewChanges(ctx, tx, guard.Resumable)
			if err != nil {
				return nil, err
			}
			for _, change := range changes {
				if change.Status == "OPEN" {
					reason = "OPEN_CHANGE_REQUEST"
					break
				}
			}
		}
		if reason != "" {
			blocks = append(blocks, map[string]string{"unit_id": domain.TextID(unit.Unit), "reason": reason})
		}
	}
	return blocks, nil
}

// structure_preview_readiness.go：保留原实现，按业务合并文件。

````

## 第 4 步：validatePendingStructurePreviewBinding

插入位置：第 3 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`func validatePendingStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingStructurePreviewBinding</code> | 使用 <code>repository.PendingStructurePreviewBinding</code> 类型；其定义处的职责：持久结构预览上下文与恢复任务，列出当前结构的单元和活动支线。<br>在此声明中的实际使用：<code>fact.State</code>；<code>fact.Original</code>；<code>fact.Payload</code>；<code>validateOriginalStructurePreviewBinding(fact)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structurePreviewContext</code> | 由 <code>validateOriginalStructurePreviewBinding(fact)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>validateOriginalStructurePreviewBinding(fact)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalid</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; json.Unmarshal(fact.Original, &amp;original) != nil &#124;&#124; json.Unmarshal(fact.Payload, &amp;progress) != nil</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return validateOriginalStructurePreviewBinding(fact)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3011:3585 -->
````go
func validatePendingStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error) {
	var original, progress structurePreviewContext
	invalid := errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	if fact.State != "PREPARING" || json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &progress) != nil {
		return original, invalid
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(progress)
	if !bytes.Equal(a, b) {
		return original, invalid
	}
	return validateOriginalStructurePreviewBinding(fact)
}

````

## 第 5 步：validateOriginalStructurePreviewBinding

插入位置：第 4 步代码之后，紧接追加；完成稿约第 107 行。

原事实核验与可变恢复状态分开：确认只能读取真实SUCCEEDED事实，不能伪造PREPARING复用就绪门槛。

本步声明：`func validateOriginalStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingStructurePreviewBinding</code> | 使用 <code>repository.PendingStructurePreviewBinding</code> 类型；其定义处的职责：持久结构预览上下文与恢复任务，列出当前结构的单元和活动支线。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Actor</code>；<code>fact.Operation</code>；<code>fact.Fingerprint</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structurePreviewContext</code> | 实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;original) != nil</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; original.Actor == 0 &#124;&#124; original.Document == 0 &#124;&#124; original.Structure == 0 &#124;&#124; original.Manifest == 0 &#124;&#124; original.Style == 0 &#124;&#124; original.SessionExpi …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(ctx.Target, expected) &#124;&#124; ctx.IdentityAdmissionRef == &quot;&quot; &#124;&#124; ctx.DocumentAdmissionRef == &quot;&quot; &#124;&#124; len(original.Access.Proofs) == 0</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parsed.Operation != original.Intent.Operation &#124;&#124; parsed.Structure != original.Structure &#124;&#124; parsed.Control != original.Control</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Target, &amp;target) != nil &#124;&#124; target.Path != path</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint)</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != 0</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != original.Actor &#124;&#124; proof.OperationId != fact.Operation &#124;&#124; proof.Purpose != &quot;STRUCTURE_PREVIEW&quot; &#124;&#124; !proto.Equal(proof.Target, expected) &#124;&#124; len(proof. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3585:7023 -->
````go
// 原事实核验与可变恢复状态分开：确认只能读取真实SUCCEEDED事实，不能伪造PREPARING复用就绪门槛。
func validateOriginalStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error) {
	var original structurePreviewContext
	invalid := errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &original) != nil {
		return original, invalid
	}
	ctx := original.Context
	if ctx == nil || ctx.Target == nil || original.Actor == 0 || original.Document == 0 || original.Structure == 0 || original.Manifest == 0 || original.Style == 0 || original.SessionExpires <= 0 ||
		original.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || original.Intent.Operation != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) ||
		original.Structure != original.Intent.Structure || original.Control != original.Intent.Control || len(original.Units) == 0 || !original.Access.Permissions.Manage || original.Access.Document.MainLocked ||
		original.Access.Document.Id != domain.TextID(original.Document) || original.Access.Document.CurrentManifestId != domain.TextID(original.Manifest) || original.Access.Document.CurrentStructureId != domain.TextID(original.Structure) ||
		original.Access.Document.CurrentStyleId != domain.TextID(original.Style) || original.Access.Document.ControlRevision != domain.TextID(original.Control) || len(ctx.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: original.Access.Sources}) {
		return original, invalid
	}
	expected := &pb.Target{DocumentId: original.Document, StructureId: original.Structure, WorkspaceEpoch: original.Control, Purpose: "STRUCTURE_PREVIEW"}
	if !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || len(original.Access.Proofs) == 0 {
		return original, invalid
	}
	parsed, err := domain.ParseStructurePreview(original.Intent.Input)
	if err != nil || parsed.Operation != original.Intent.Operation || parsed.Structure != original.Structure || parsed.Control != original.Control {
		return original, invalid
	}
	path := "/v1/documents/" + domain.TextID(original.Document) + "/structure-previews"
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Target, &target) != nil || target.Path != path {
		return original, invalid
	}
	fingerprint, err := domain.OperationFingerprint(path, original.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return original, invalid
	}
	available := make(map[string]uint64, len(ctx.Sources))
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return original, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range original.Access.Proofs {
		if proof == nil || proof.ActorId != original.Actor || proof.OperationId != fact.Operation || proof.Purpose != "STRUCTURE_PREVIEW" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) {
			return original, invalid
		}
		// 自然过期可续接；缺会话或换来源则不是原操作，启动屏障不能把它当成可恢复任务。
````

## 第 6 步：validateOriginalStructurePreviewBinding（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 154 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(proof.SessionRef, original.Access.Proofs&#91;0&#93;.SessionRef)</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != source.Epoch</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>original.Access.Proofs&#91;0&#93;.AdmissionId != ctx.IdentityAdmissionRef</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>unit.Unit == 0 &#124;&#124; unit.Chapter == 0 &#124;&#124; unit.Room == 0 &#124;&#124; unit.Workspace.Workspace == 0 &#124;&#124; unit.Workspace.Epoch == 0 &#124;&#124; unit.Workspace.Lineage == 0 &#124;&#124; unit.Workspace.BaseManifest == …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return original, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=7023:7799 -->
````go
		if !bytes.Equal(proof.SessionRef, original.Access.Proofs[0].SessionRef) {
			return original, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return original, invalid
			}
		}
	}
	if original.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef {
		return original, invalid
	}
	seen := map[uint64]bool{}
	for _, unit := range original.Units {
		if unit.Unit == 0 || unit.Chapter == 0 || unit.Room == 0 || unit.Workspace.Workspace == 0 || unit.Workspace.Epoch == 0 || unit.Workspace.Lineage == 0 || unit.Workspace.BaseManifest == 0 || unit.Workspace.BaseVersion == 0 || seen[unit.Unit] {
			return original, invalid
		}
		seen[unit.Unit] = true
	}
	return original, nil
}

````

## 第 7 步：StructurePreviews.Ready

插入位置：第 6 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func (s *StructurePreviews) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingStructurePreviewBindings(ctx, s.db)</code>；<code>repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;STRUCTURE_PREVIEW_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;STRUCTURE_PREVIEW_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateStructurePreviewAdmission(fixed, admission) != nil</code>。条件成立时执行 <code>return errors.New(&quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=7799:8707 -->
````go
func (s *StructurePreviews) Ready(ctx context.Context) error {
	facts, err := repository.PendingStructurePreviewBindings(ctx, s.db)
	if err != nil {
		return errors.New("STRUCTURE_PREVIEW_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingStructurePreviewBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateStructurePreviewAdmission(fixed, admission) != nil {
			return errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
		}
	}
	return nil
}

// structure_preview_readings.go：保留原实现，按业务合并文件。

````

## 第 8 步：StructurePreviews.Wait

插入位置：第 7 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func (s *StructurePreviews) Wait(ctx context.Context, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructurePreviewResult(ctx, s.db, operation)</code>；<code>ctx.Done</code>；<code>ctx.Err</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.StructurePreviewResult(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>ctx.Err()</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_PREPARING&quot;)</code> |

**跟着本段代码走一遍：**

- 登记 <code>deadline.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return ctx.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_PREPARING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=8707:9396 -->
````go
func (s *StructurePreviews) Wait(ctx context.Context, operation string) error {
	// 预览依靠持久任务续接，入口只短等；留出边缘4秒预算中的身份核验与错误响应时间。
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		fact, err := repository.StructurePreviewResult(ctx, s.db, operation)
		if err != nil {
			return err
		}
		if fact.State != "PREPARING" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_PREPARING")
		case <-ticker.C:
		}
	}
}

````

## 第 9 步：StructurePreviews.Read

插入位置：第 8 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func (s *StructurePreviews) Read(ctx context.Context, actor Actor, document uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.resultAccess(ctx, actor, document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.resultAccess(ctx, actor, document)</code>；<code>s.visibleResult(ctx, tx, actor, access, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fixed.Document != document</code>；<code>s.resultAccess(ctx, actor, document)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.visibleResult(ctx, tx, actor, access, operation)</code>；<code>sameStructurePreviewRef(value.PreviewCutRef, operation)</code>；<code>value.PreviewCutRef = operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visibleResult(ctx, tx, actor, access, operation) 		if err != nil { 			return nil, err 		} 		if fact. …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Document != document &#124;&#124; fixed.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visibleResult(ctx, tx, actor, access, operation) 		if err != nil { 			return  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_PREPARING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;SUCCEEDED&quot;</code>。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;rejected) != nil &#124;&#124; rejected.Error.Code == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.StateConflict, rejected.Error.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil &#124;&#124; !sameStructurePreviewRef(value.PreviewCutRef, operation)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=9396:10906 -->
````go
func (s *StructurePreviews) Read(ctx context.Context, actor Actor, document uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Document != document || fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	access, err := s.resultAccess(ctx, actor, document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visibleResult(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		if fact.State == "PREPARING" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_PREPARING")
		}
		if fact.State != "SUCCEEDED" {
			var rejected struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
			}
			return nil, domain.Violation(domain.StateConflict, rejected.Error.Code)
		}
		var value dto.StructurePreview
		if json.Unmarshal(fact.Result, &value) != nil || !sameStructurePreviewRef(value.PreviewCutRef, operation) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		value.PreviewCutRef = operation
		return value, nil
	}, nil
}

````

## 第 10 步：StructurePreviews.resultAccess

插入位置：第 9 步代码之后，紧接追加；完成稿约第 264 行。

本步声明：`func (s *StructurePreviews) resultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW_RESULT&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW_RESULT&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; !access.Permissions.Manage</code>。
- 此处通过 <code>return access, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=10906:11261 -->
````go
func (s *StructurePreviews) resultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, document, "STRUCTURE_PREVIEW_RESULT")
	if err == nil && !access.Permissions.Manage {
		err = domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	return access, err
}

````

## 第 11 步：StructurePreviews.visibleResult

插入位置：第 10 步代码之后，紧接追加；完成稿约第 272 行。

本步声明：`func (s *StructurePreviews) visibleResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.StructurePreviewResult(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.StructurePreviewResult(ctx, tx, operation)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.StructurePreviewResult(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.MainControlReceipt</code> | 使用 <code>repository.MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>repository.MainControlReceipt{}</code>；<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.MainControlReceipt{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=11261:11903 -->
````go
func (s *StructurePreviews) visibleResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return repository.MainControlReceipt{}, err
	}
	fact, err := repository.StructurePreviewResult(ctx, tx, operation)
	if err != nil {
		return fact, err
	}
	if fact.Actor != actor.ID {
		return fact, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return fact, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	return fact, nil
}

````

## 第 12 步：StructurePreviews.Progress

插入位置：第 11 步代码之后，紧接追加；完成稿约第 289 行。

POST返回固定预览；通用GET按冻结Operation仅投影状态和已有实体ID，不把私有切点塞入未知字段。

本步声明：`func (s *StructurePreviews) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.resultAccess(ctx, actor, fixed.Document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.resultAccess(ctx, actor, fixed.Document)</code>；<code>s.visibleResult(ctx, tx, actor, access, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.visibleResult(ctx, tx, actor, access, operation)</code>；<code>&quot;operation_id&quot;: operation</code>；<code>&quot;/v1/operations/&quot; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visibleResult(ctx, tx, actor, access, operation) 		if err != nil { 			return nil, err 		} 		result : …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visibleResult(ctx, tx, actor, access, operation) 		if err != nil { 			return  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;PREPARING&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;REJECTED&quot; &#124;&#124; json.Unmarshal(fact.Result, &amp;rejected) != nil &#124;&#124; rejected&#91;&quot;error&quot;&#93; == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=11903:13181 -->
````go
// POST返回固定预览；通用GET按冻结Operation仅投影状态和已有实体ID，不把私有切点塞入未知字段。
func (s *StructurePreviews) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.resultAccess(ctx, actor, fixed.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visibleResult(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"operation_id": operation, "state": fact.State}
		if fact.State == "PREPARING" {
			result["poll_url"] = "/v1/operations/" + operation
			return result, nil
		}
		if fact.State == "SUCCEEDED" {
			result["result"] = map[string]any{"entity_id": domain.TextID(fixed.Structure)}
			return result, nil
		}
		var rejected map[string]any
		if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected["error"] == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		result["error"] = rejected["error"]
		return result, nil
	}, nil
}

````

## 第 13 步：StructurePreviews.OperationResult

插入位置：第 12 步代码之后，紧接追加；完成稿约第 322 行。

本步声明：`func (s *StructurePreviews) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructurePreviewResult(ctx, s.db, query.OperationId)</code>；<code>s.Lookup(ctx, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_PREVIEW_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_PREVIEW_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>fact.State != &quot;REJECTED&quot; &#124;&#124; json.Unmarshal(fact.Result, &amp;rejected) != nil &#124;&#124; rejected.Error.Code == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=13181:15108 -->
````go
func (s *StructurePreviews) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_PREVIEW_QUERY_INVALID")
	}
	fact, err := repository.StructurePreviewResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if fact.Actor != fixed.Actor {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
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
		result.ResultIds = []uint64{fixed.Structure}
	} else {
		var rejected struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		result.BusinessCode = rejected.Error.Code
	}
	return result, nil
}

// structure_preview_recovery.go：保留原实现，按业务合并文件。

````

## 第 14 步：StructurePreviews.RecoverOne

插入位置：第 13 步代码之后，紧接追加；完成稿约第 366 行。

本步声明：`func (s *StructurePreviews) RecoverOne(parent context.Context)`。


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
- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>string(a) != string(b)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.StateConflict &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 14 bytes=15108:16493 -->
````go
func (s *StructurePreviews) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimStructurePreviewTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed structurePreviewContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	} else {
		original, readErr := s.Lookup(ctx, task.Operation)
		err = readErr
		if err == nil {
			a, _ := json.Marshal(original)
			b, _ := json.Marshal(fixed)
			if string(a) != string(b) {
				err = domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
			}
		}
		if err == nil {
			err = s.captureAndCommit(ctx, task, fixed)
		}
	}
	var violation domain.RuleViolation
	if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
		err = rejectPreviewTask(ctx, s.db, task, violation.Code)
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

````

## 第 15 步：StructurePreviews.captureAndCommit

插入位置：第 14 步代码之后，紧接追加；完成稿约第 409 行。

本步声明：`func (s *StructurePreviews) captureAndCommit(ctx context.Context, task domain.SubmissionTask, fixed structurePreviewContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>s.client.Preview(ctx, input)</code>；<code>s.client.FixedStructure(ctx, input)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>fixed.Context</code>；<code>fixed.Structure</code>；<code>validateStructurePreviewSnapshot(s.validator, raw, fixed, reply)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>structurePreviewCaptureError(err)</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_EXPIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_RESOURCE_NOT_READY&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return structurePreviewCaptureError(err)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=16493:17899 -->
````go
func (s *StructurePreviews) captureAndCommit(ctx context.Context, task domain.SubmissionTask, fixed structurePreviewContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	input := &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedStructureId: fixed.Structure}
	reply, err := s.client.Preview(ctx, input)
	if err != nil {
		return structurePreviewCaptureError(err)
	}
	raw, err := s.client.FixedStructure(ctx, input)
	if err != nil {
		return err
	}
	snapshot, err := validateStructurePreviewSnapshot(s.validator, raw, fixed, reply)
	if err != nil {
		return err
	}
	if reply.ExpiresUnixMs <= time.Now().UnixMilli() {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_EXPIRED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validate(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	current, err := repository.DocumentRow(ctx, tx, fixed.Document, true)
	if err != nil {
		return err
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	resources := map[uint64]bool{}
	for _, content := range append(snapshot.Main, snapshot.Drafts...) {
		for _, id := range content.ResourceIds {
			resources[id] = true
		}
	}
````

## 第 16 步：StructurePreviews.captureAndCommit（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 455 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return resourceIDs&#91;i&#93; &lt; resourceIDs&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 16 bytes=17899:19349 -->
````go
	resourceIDs := make([]uint64, 0, len(resources))
	for id := range resources {
		resourceIDs = append(resourceIDs, id)
	}
	sort.Slice(resourceIDs, func(i, j int) bool { return resourceIDs[i] < resourceIDs[j] })
	for _, id := range resourceIDs {
		state, err := repository.LockedDocumentResource(ctx, tx, id, fixed.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "STRUCTURE_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, fixed.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	blocks, err := structurePreviewBlockingUnits(ctx, tx, fixed)
	if err != nil {
		return err
	}
	public := map[string]any{"preview_cut_ref": task.Operation, "structure_id": domain.TextID(fixed.Structure), "cut_digest": hex.EncodeToString(snapshot.Cut.DependencyDigest), "expires_at": time.UnixMilli(reply.ExpiresUnixMs).UTC().Format(time.RFC3339Nano), "blocking_units": blocks}
	result, err := json.Marshal(public)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 17 步：structurePreviewCaptureError

插入位置：第 16 步代码之后，紧接追加；完成稿约第 493 行。

远端只有这些确定的正文前置条件可以终结原号；断连、未知范围和内部异常仍沿持久任务恢复。

本步声明：`func structurePreviewCaptureError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>status.FromError(err)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, rpc.Message())</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>rpc.Code() == codes.FailedPrecondition</code>。
- 此处通过 <code>return domain.Violation(domain.StateConflict, rpc.Message())</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>rpc.Code() == codes.ResourceExhausted &amp;&amp; rpc.Message() == &quot;STRUCTURE_TRANSFER_LIMIT&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, rpc.Message())</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=19349:20243 -->
````go
// 远端只有这些确定的正文前置条件可以终结原号；断连、未知范围和内部异常仍沿持久任务恢复。
func structurePreviewCaptureError(err error) error {
	rpc, ok := status.FromError(err)
	if !ok {
		return err
	}
	if rpc.Code() == codes.FailedPrecondition {
		switch rpc.Message() {
		case "DRAFT_STRUCTURE_CHANGED", "STRUCTURE_DRAFT_CONFLICT", "STRUCTURE_LEVEL2_MISSING", "STRUCTURE_TITLE_INVALID", "STRUCTURE_TITLE_CHANGED", "STRUCTURE_MODE_CHANGED", "STRUCTURE_RANGE_OVERLAP", "STRUCTURE_BLOCK_ID_COLLISION", "STRUCTURE_CONTENT_LIMIT":
			return domain.Violation(domain.StateConflict, rpc.Message())
		}
	}
	if rpc.Code() == codes.ResourceExhausted && rpc.Message() == "STRUCTURE_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	return err
}

// structure_preview_register.go：保留原实现，按业务合并文件。

````

## 第 18 步：StructurePreviews.Register

插入位置：第 17 步代码之后，紧接追加；完成稿约第 513 行。

本步声明：`func (s *StructurePreviews) Register(ctx context.Context, actor Actor, document uint64, intent domain.StructurePreviewIntent, path string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW&quot;)</code>；<code>repository.StructurePreviewResult(ctx, s.db, intent.Operation)</code>；<code>s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 13 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW&quot;)</code>；<code>return actor, err</code>；<code>return actor, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_MANAGE_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 19 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;STRUCTURE_PREVIEW&quot;)</code>；<code>DocumentId: document</code>；<code>repository.LockedMergeBarrier(ctx, tx, document)</code>；<code>Document: document</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.StructurePreviewIntent</code> | 使用 <code>domain.StructurePreviewIntent</code> 类型；其定义处的职责：表达管理意图、拆合迁移候选和历史范围边界，并提供纯结构变换。<br>在此声明中的实际使用：<code>intent.Input</code>；<code>intent.Operation</code>；<code>intent.Structure</code>；<code>intent.Control</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, intent.Input)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>lookupErr == nil</code>。
- 检查 <code>result.Actor != actor.ID &#124;&#124; !bytes.Equal(result.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>lookupErr != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, lookupErr</code>。当前函数在这里结束。
- 检查 <code>access.Document.MainLocked</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_LOCKED&quot;)</code>。当前函数在这里结束。
- 检查 <code>access.Document.CurrentStructureId != domain.TextID(intent.Structure) &#124;&#124; access.Document.ControlRevision != domain.TextID(intent.Control)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(proofs) == 0</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_IDENTITY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=20243:22337 -->
````go
func (s *StructurePreviews) Register(ctx context.Context, actor Actor, document uint64, intent domain.StructurePreviewIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "STRUCTURE_PREVIEW")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Manage {
		return actor, domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	fingerprint, err := domain.OperationFingerprint(path, intent.Input)
	if err != nil {
		return actor, err
	}
	actor.Sources = access.Sources
	if result, lookupErr := repository.StructurePreviewResult(ctx, s.db, intent.Operation); lookupErr == nil {
		if result.Actor != actor.ID || !bytes.Equal(result.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		// 当前MANAGE仍要求成立；原预览的结构/control可以已经过时，重复请求只读取原事实。
		return actor, nil
	} else if lookupErr != sql.ErrNoRows {
		return actor, lookupErr
	}
	// 明确无资格的页面不先占用账号原号；事务内仍再次复核，防止授权后到预约前发生改变。
	if access.Document.MainLocked {
		return actor, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
	}
	if access.Document.CurrentStructureId != domain.TextID(intent.Structure) || access.Document.ControlRevision != domain.TextID(intent.Control) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, StructureId: intent.Structure, WorkspaceEpoch: intent.Control, Purpose: "STRUCTURE_PREVIEW"}
	proofs := access.Proofs
	if len(proofs) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_IDENTITY_UNKNOWN")
	}
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	actor.Proof = access.Proofs[0]
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, intent.Operation); err != nil {
````

## 第 19 步：StructurePreviews.Register（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 558 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>lookupErr == nil</code>。
- 检查 <code>existing.ActorID != actor.ID &#124;&#124; existing.Kind != &quot;STRUCTURE_PREVIEW&quot; &#124;&#124; !bytes.Equal(existing.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>lookupErr != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, lookupErr</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentStructureId != domain.TextID(intent.Structure) &#124;&#124; current.ControlRevision != domain.TextID(intent.Control)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.MainLocked</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_LOCKED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>entry.Kind == &quot;CHAPTER&quot; &amp;&amp; entry.ParentNodeId == nil &amp;&amp; change.AfterChapterId != nil &amp;&amp; entry.ChapterId == *change.AfterChapterId</code>。
- 检查 <code>entry.Kind == &quot;CHAPTER&quot; &amp;&amp; entry.ParentNodeId == nil &amp;&amp; entry.ChapterId == change.ChapterId</code>。
- 检查 <code>change.Type == &quot;SPLIT_LEVEL2&quot; &amp;&amp; entry.SplitLevel2 &#124;&#124; change.Type == &quot;JOIN_CHAPTER&quot; &amp;&amp; !entry.SplitLevel2</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_MODE_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=22337:24251 -->
````go
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if existing, lookupErr := repository.LockedOperation(ctx, tx, intent.Operation); lookupErr == nil {
		if existing.ActorID != actor.ID || existing.Kind != "STRUCTURE_PREVIEW" || !bytes.Equal(existing.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if lookupErr != sql.ErrNoRows {
		return actor, lookupErr
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	if current.CurrentStructureId != domain.TextID(intent.Structure) || current.ControlRevision != domain.TextID(intent.Control) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	if current.MainLocked {
		return actor, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return actor, err
	}
	for _, change := range intent.Input.Changes {
		found := false
		anchorFound := change.AfterChapterId == nil
		for _, entry := range structure.Entries {
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && change.AfterChapterId != nil && entry.ChapterId == *change.AfterChapterId {
				anchorFound = true
			}
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && entry.ChapterId == change.ChapterId {
				found = true
				if change.Type == "SPLIT_LEVEL2" && entry.SplitLevel2 || change.Type == "JOIN_CHAPTER" && !entry.SplitLevel2 {
					return actor, domain.Violation(domain.StateConflict, "STRUCTURE_MODE_CHANGED")
				}
			}
		}
````

## 第 20 步：StructurePreviews.Register（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 606 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!found</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;CHAPTER_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!anchorFound</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;CHAPTER_ANCHOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>len(units) == 0</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_SCOPE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=24251:26569 -->
````go
		if !found {
			return actor, domain.Violation(domain.StateConflict, "CHAPTER_CHANGED")
		}
		if !anchorFound {
			return actor, domain.Violation(domain.StateConflict, "CHAPTER_ANCHOR_CHANGED")
		}
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	fixed := structurePreviewContext{Actor: actor.ID, Document: document, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Manifest: manifest, Structure: intent.Structure, Style: style, Control: intent.Control}
	units, err := repository.CurrentStructurePreviewUnits(ctx, tx, document, intent.Structure)
	if err != nil {
		return actor, err
	}
	if len(units) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_SCOPE_UNKNOWN")
	}
	for _, unit := range units {
		workspace, err := repository.SubmissionWorkspace(ctx, tx, unit.Branch, document, unit.Unit)
		if err != nil {
			return actor, domain.Violation(domain.StateConflict, "STRUCTURE_BRANCH_CHANGED")
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", workspace.Workspace, unit.Unit, workspace.Epoch)
		if err != nil {
			return actor, err
		}
		fixed.Units = append(fixed.Units, structurePreviewUnit{Unit: unit.Unit, Chapter: unit.Chapter, Room: room, Workspace: workspace})
	}
	pending := map[string]any{"operation_id": intent.Operation, "state": "PREPARING"}
	command := OperationCommand{ID: intent.Operation, Kind: "STRUCTURE_PREVIEW", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, pending); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "STRUCTURE_PREVIEW", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
````

## 第 21 步：StructurePreviews.Register（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 652 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 21 bytes=26569:26793 -->
````go
	if err = repository.RegisterStructurePreview(ctx, tx, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// structure_preview_snapshot.go：保留原实现，按业务合并文件。

````

## 第 22 步：structurePreviewSnapshot

插入位置：第 21 步代码之后，紧接追加；完成稿约第 660 行。

本步声明：`type structurePreviewSnapshot struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Main</code> | <code>&#91;&#93;*pb.FixedContent</code> | 原结构中各单元的固定主线正文。 |
| <code>Drafts</code> | <code>&#91;&#93;*pb.FixedContent</code> | 原结构中各单元的固定共同支线正文，可与主线不同。 |
| <code>Cut</code> | <code>*pb.CutVector</code> | 预览固定的主线/支线房间持久序号向量。 |

<!-- write-step: 22 bytes=26793:26896 -->
````go
type structurePreviewSnapshot struct {
	Main, Drafts []*pb.FixedContent
	Cut          *pb.CutVector
}

````

## 第 23 步：validateStructurePreviewSnapshot

插入位置：第 22 步代码之后，紧接追加；完成稿约第 665 行。

本步声明：`func validateStructurePreviewSnapshot(validator *codec.Validator, raw []byte, fixed structurePreviewContext, reply *pb.DraftPreviewReply) (structurePreviewSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validateFixedMergeContent(validator, content, fixed.Document, unit.Unit, fixed.Structure)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;envelope)</code>；<code>codec.Decode(raw)</code> |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.Manifest</code>；<code>fixed.Context</code>；<code>fixed.Structure</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>reply</code> | <code>*pb.DraftPreviewReply</code> | 在此声明中的实际使用：<code>reply == nil</code>；<code>reply.State</code>；<code>reply.PreviewCutRef</code>；<code>reply.MainManifestId</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structurePreviewSnapshot</code> | 实际返回表达式：<code>snapshot</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>reply == nil &#124;&#124; reply.State != pb.CaptureReply_FIXED &#124;&#124; !sameStructurePreviewRef(reply.PreviewCutRef, fixed.Intent.Operation) &#124;&#124; reply.MainManifestId != fixed.Manifest &#124;&#124; reply.Obs …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;envelope) != nil &#124;&#124; envelope.Preview != reply.PreviewCutRef &#124;&#124; envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs)) &#124;&#124; !bytes.Equal(envelope.Digest, reply.InputDigest)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(envelope.Binding, binding) != nil &#124;&#124; !proto.Equal(binding.Context, fixed.Context) &#124;&#124; binding.ExpectedBranchId != 0 &#124;&#124; binding.ExpectedLineageId != 0 &#124;&#124; binding.ExpectedStructureId != fixed.Structure</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest&#91;:&#93;, reply.InputDigest)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(envelope.Cut, snapshot.Cut) != nil &#124;&#124; !proto.Equal(snapshot.Cut, reply.ObservedCut) &#124;&#124; snapshot.Cut.DocumentId != fixed.Document &#124;&#124; snapshot.Cut.StructureId !=  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=26896:29235 -->
````go
func validateStructurePreviewSnapshot(validator *codec.Validator, raw []byte, fixed structurePreviewContext, reply *pb.DraftPreviewReply) (structurePreviewSnapshot, error) {
	var snapshot structurePreviewSnapshot
	invalid := domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	if reply == nil || reply.State != pb.CaptureReply_FIXED || !sameStructurePreviewRef(reply.PreviewCutRef, fixed.Intent.Operation) || reply.MainManifestId != fixed.Manifest || reply.ObservedCut == nil || len(reply.InputDigest) != 32 || len(raw) > 24<<20 {
		return snapshot, invalid
	}
	var envelope struct {
		Binding json.RawMessage   `json:"binding"`
		Main    []json.RawMessage `json:"main_units"`
		Drafts  []json.RawMessage `json:"drafts"`
		Cut     json.RawMessage   `json:"observed_cut"`
		Digest  []byte            `json:"input_digest"`
		Preview string            `json:"preview_cut_ref"`
		Expires string            `json:"expires_unix_ms"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Preview != reply.PreviewCutRef || envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs)) || !bytes.Equal(envelope.Digest, reply.InputDigest) {
		return snapshot, invalid
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(envelope.Binding, binding) != nil || !proto.Equal(binding.Context, fixed.Context) || binding.ExpectedBranchId != 0 || binding.ExpectedLineageId != 0 || binding.ExpectedStructureId != fixed.Structure {
		return snapshot, invalid
	}
	value, err := codec.Decode(raw)
	if err != nil {
		return snapshot, err
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return snapshot, invalid
	}
	delete(fields, "input_digest")
	canonical, err := codec.Canonical(fields)
	if err != nil {
		return snapshot, err
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(digest[:], reply.InputDigest) {
		return snapshot, invalid
	}
	snapshot.Cut = &pb.CutVector{}
	if protojson.Unmarshal(envelope.Cut, snapshot.Cut) != nil || !proto.Equal(snapshot.Cut, reply.ObservedCut) || snapshot.Cut.DocumentId != fixed.Document || snapshot.Cut.StructureId != fixed.Structure || snapshot.Cut.StyleId != fixed.Style || snapshot.Cut.ControlRevision != fixed.Control || len(snapshot.Cut.Rooms) > 10000 {
		return snapshot, invalid
	}
	seenRooms := map[uint64]bool{}
	for _, room := range snapshot.Cut.Rooms {
````

## 第 24 步：validateStructurePreviewSnapshot（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 710 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>room == nil &#124;&#124; room.RoomId == 0 &#124;&#124; room.WorkspaceEpoch == 0 &#124;&#124; seenRooms&#91;room.RoomId&#93;</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(roomDigest&#91;:&#93;, snapshot.Cut.DependencyDigest)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>len(envelope.Main) != len(fixed.Units) &#124;&#124; len(envelope.Drafts) != len(fixed.Units)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>protojson.Unmarshal(item, content) != nil &#124;&#124; seen&#91;content.UnitId&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists &#124;&#124; content.OperationId != fixed.Intent.Operation</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>draft</code>。
- 在本段的匿名函数内：检查 <code>content.BranchId != unit.Workspace.Workspace &#124;&#124; content.WorkspaceEpoch != unit.Workspace.Epoch &#124;&#124; content.LineageRef != domain.TextID(unit.Workspace.Lineage)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>content.BranchId != 0 &#124;&#124; content.WorkspaceEpoch != fixed.Control</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!bytes.Equal(hash&#91;:&#93;, content.BodyHash)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=29235:30931 -->
````go
		if room == nil || room.RoomId == 0 || room.WorkspaceEpoch == 0 || seenRooms[room.RoomId] {
			return snapshot, invalid
		}
		seenRooms[room.RoomId] = true
	}
	rooms, err := codec.Canonical(domain.CutBinding(snapshot.Cut)["rooms"])
	if err != nil {
		return snapshot, err
	}
	roomDigest := sha256.Sum256(rooms)
	if !bytes.Equal(roomDigest[:], snapshot.Cut.DependencyDigest) {
		return snapshot, invalid
	}
	if len(envelope.Main) != len(fixed.Units) || len(envelope.Drafts) != len(fixed.Units) {
		return snapshot, invalid
	}
	expected := map[uint64]structurePreviewUnit{}
	for _, unit := range fixed.Units {
		expected[unit.Unit] = unit
	}
	decode := func(items []json.RawMessage, draft bool) ([]*pb.FixedContent, error) {
		result := make([]*pb.FixedContent, 0, len(items))
		seen := map[uint64]bool{}
		for _, item := range items {
			content := &pb.FixedContent{}
			if protojson.Unmarshal(item, content) != nil || seen[content.UnitId] {
				return nil, invalid
			}
			unit, exists := expected[content.UnitId]
			if !exists || content.OperationId != fixed.Intent.Operation {
				return nil, invalid
			}
			if draft {
				if content.BranchId != unit.Workspace.Workspace || content.WorkspaceEpoch != unit.Workspace.Epoch || content.LineageRef != domain.TextID(unit.Workspace.Lineage) {
					return nil, invalid
				}
			} else if content.BranchId != 0 || content.WorkspaceEpoch != fixed.Control {
				return nil, invalid
			}
			if err := validateFixedMergeContent(validator, content, fixed.Document, unit.Unit, fixed.Structure); err != nil {
				return nil, err
			}
			hash := sha256.Sum256(content.BodyJsonUtf8)
			if !bytes.Equal(hash[:], content.BodyHash) {
				return nil, invalid
			}
````

## 第 25 步：validateStructurePreviewSnapshot（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 756 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=30931:31194 -->
````go
			seen[unit.Unit] = true
			result = append(result, content)
		}
		return result, nil
	}
	if snapshot.Main, err = decode(envelope.Main, false); err != nil {
		return snapshot, err
	}
	snapshot.Drafts, err = decode(envelope.Drafts, true)
	return snapshot, err
}

````

## 第 26 步：sameStructurePreviewRef

插入位置：第 25 步代码之后，紧接追加；完成稿约第 768 行。

只兼容首轮实际固定事实的准确旧前缀，不改其不可变字节/摘要；新公共引用统一为原操作号。

本步声明：`func sameStructurePreviewRef(reference, operation string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>reference</code> | <code>string</code> | 在此声明中的实际使用：<code>reference == operation</code>；<code>reference == &quot;structure-preview:&quot;+operation</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>reference == operation</code>；<code>&quot;structure-preview:&quot;+operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>reference == operation &#124;&#124; reference == &quot;structure-preview:&quot;+operation</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return reference == operation &#124;&#124; reference == &quot;structure-preview:&quot;+operation</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=31194:31548 -->
````go
// 只兼容首轮实际固定事实的准确旧前缀，不改其不可变字节/摘要；新公共引用统一为原操作号。
func sameStructurePreviewRef(reference, operation string) bool {
	return reference == operation || reference == "structure-preview:"+operation
}

// structure_preview_source.go：保留原实现，按业务合并文件。

````

## 第 27 步：StructurePreviews.Lookup

插入位置：第 26 步代码之后，紧接追加；完成稿约第 775 行。

本步声明：`func (s *StructurePreviews) Lookup(ctx context.Context, operation string) (structurePreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;STRUCTURE_PREVIEW_CONTEXT&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structurePreviewContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=31548:32043 -->
````go
func (s *StructurePreviews) Lookup(ctx context.Context, operation string) (structurePreviewContext, error) {
	var fixed structurePreviewContext
	raw, err := repository.MergeArtifact(ctx, s.db, "STRUCTURE_PREVIEW_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return fixed, nil
}

````

## 第 28 步：StructurePreviews.resume

插入位置：第 27 步代码之后，紧接追加；完成稿约第 787 行。

本步声明：`func (s *StructurePreviews) resume(ctx context.Context, fixed structurePreviewContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=32043:32563 -->
````go
func (s *StructurePreviews) resume(ctx context.Context, fixed structurePreviewContext) ([]*pb.IdentityAdmission, error) {
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

## 第 29 步：validateStructurePreviewAdmission

插入位置：第 28 步代码之后，紧接追加；完成稿约第 802 行。

本步声明：`func validateStructurePreviewAdmission(fixed structurePreviewContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code>；<code>fixed.Document</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>context == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil &#124;&#124; actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Document &#124;&#124; actual.Unit != 0 &#124;&#124; actual.I …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, context.Target)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=32563:33463 -->
````go
func validateStructurePreviewAdmission(fixed structurePreviewContext, actual repository.OriginalAdmissionBinding) error {
	context := fixed.Context
	if context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != 0 || actual.Identity != context.IdentityAdmissionRef ||
		actual.Operation != context.OperationId || actual.Purpose != "STRUCTURE_PREVIEW" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return nil
}

````

## 第 30 步：StructurePreviews.validate

插入位置：第 29 步代码之后，紧接追加；完成稿约第 815 行。

本步声明：`func (s *StructurePreviews) validate(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>validateStructurePreviewUnits(ctx, tx, fixed)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>validateStructurePreviewUnits(ctx, tx, fixed)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>structurePreviewContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_ADMISSION_CLOSED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Manage &#124;&#124; current.MainLocked &#124;&#124; current.CurrentManifestId != domain.TextID(fixed.Manifest) &#124;&#124; current.CurrentStructureId != domain.TextID(fixed.Structure) …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=33463:35566 -->
````go
func (s *StructurePreviews) validate(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage || current.MainLocked || current.CurrentManifestId != domain.TextID(fixed.Manifest) || current.CurrentStructureId != domain.TextID(fixed.Structure) || current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	if err = validateStructurePreviewUnits(ctx, tx, fixed); err != nil {
		return err
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	if err = validateStructurePreviewAdmission(fixed, admission); err != nil {
		return err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	// 固定引用只证明原事实归属，不能使已经关闭的原Doc准入重新开放；后台保存明确拒绝再收尾。
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

````

## 第 31 步：StructurePreviews.Source

插入位置：第 30 步代码之后，紧接追加；完成稿约第 861 行。

本步声明：`func (s *StructurePreviews) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>input.ExpectedBranchId</code>；<code>input.ExpectedLineageId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;: domain.TextID(fixed.Style), &quot;control_revision&quot;: domain.Text …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_PREVIEW_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_PREVIEW_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input.Context, fixed.Context) &#124;&#124; input.ExpectedBranchId != 0 &#124;&#124; input.ExpectedLineageId != 0 &#124;&#124; input.ExpectedStructureId != fixed.Structure</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 31 bytes=35566:37246 -->
````go
func (s *StructurePreviews) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_PREVIEW_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input.Context, fixed.Context) || input.ExpectedBranchId != 0 || input.ExpectedLineageId != 0 || input.ExpectedStructureId != fixed.Structure {
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
	if err = s.validate(ctx, tx, fixed, proofs); err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, domain.TextID(fixed.Manifest), fixed.Control)
	if err != nil {
		return nil, err
	}
	structure, err := repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(fixed.Document))
	if err != nil {
		return nil, err
	}
	boundaries, err := readStructureUnitBoundaries(ctx, tx, fixed.Document, structure.Entries)
	if err != nil {
		return nil, err
	}
	drafts := make([]any, 0, len(fixed.Units))
	for _, unit := range fixed.Units {
		baseline, err := repository.VersionContent(ctx, tx, unit.Workspace.BaseVersion, fixed.Document)
		if err != nil {
			return nil, err
		}
		if err = canonicalVersion(baseline, unit.Workspace.BaseVersion); err != nil {
			return nil, err
		}
		_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, unit.Workspace.Workspace)
````

## 第 32 步：StructurePreviews.Source（接着实现）

插入位置：第 31 步代码之后，紧接追加；完成稿约第 906 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;: domain.TextID(fixed.Style), &quot;control_revision&quot;: doma …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=37246:38601 -->
````go
		if err != nil {
			return nil, err
		}
		branchBaseline, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
		if err != nil {
			return nil, err
		}
		body, err := codec.Decode(branchBaseline)
		if err != nil {
			return nil, err
		}
		sources, err := codec.Decode(provenance)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, map[string]any{"unit_id": domain.TextID(unit.Unit), "chapter_id": domain.TextID(unit.Chapter), "branch_id": domain.TextID(unit.Workspace.Workspace), "workspace_epoch": domain.TextID(unit.Workspace.Epoch), "room_id": domain.TextID(unit.Room),
			"lineage": map[string]any{"lineage_id": domain.TextID(unit.Workspace.Lineage), "base_manifest_id": domain.TextID(unit.Workspace.BaseManifest), "base_version_id": domain.TextID(unit.Workspace.BaseVersion), "body": body, "provenance": sources}})
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": domain.TextID(fixed.Manifest), "structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style), "control_revision": domain.TextID(fixed.Control), "workspace_epoch": domain.TextID(epoch), "structure": structure, "changes": fixed.Intent.Input.Changes, "units": units, "drafts": drafts, "unit_boundaries": boundaries}, nil
}

// structure_previews.go：保留原实现，按业务合并文件。

````

## 第 33 步：StructurePreviewClient

插入位置：第 32 步代码之后，紧接追加；完成稿约第 932 行。

本步声明：`type StructurePreviewClient interface {`。

**这一段为什么这样写：** 拆合目录前保留主线与支线的真实范围映射；不要求支线先被主线强行覆盖成一样。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Preview</code> | <code>(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)</code> | ctx与固定结构预览请求输入，返回Node预览引用/摘要。 |
| <code>FixedStructure</code> | <code>(context.Context, *pb.DraftPreviewRequest) (&#91;&#93;byte, error)</code> | 按同一预览请求读取已经固定的结构/主线/草稿快照字节，不重新采样替换原预览。 |

<!-- write-step: 33 bytes=38601:38800 -->
````go
type StructurePreviewClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	FixedStructure(context.Context, *pb.DraftPreviewRequest) ([]byte, error)
}

````

## 第 34 步：StructurePreviews

插入位置：第 33 步代码之后，紧接追加；完成稿约第 937 行。

本步声明：`type StructurePreviews struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>StructurePreviewClient</code> | 使用 <code>StructurePreviewClient</code> 类型；其定义处的职责：拆合目录前保留主线与支线的真实范围映射；不要求支线先被主线强行覆盖成一样。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 34 bytes=38800:38974 -->
````go
type StructurePreviews struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    StructurePreviewClient
	validator *codec.Validator
	instance  string
}

````

## 第 35 步：structurePreviewUnit

插入位置：第 34 步代码之后，紧接追加；完成稿约第 946 行。

本步声明：`type structurePreviewUnit struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 原协作单元编号。 |
| <code>Chapter</code> | <code>uint64</code> | 单元所属章节，拆分子标题仍保留父章节。 |
| <code>Room</code> | <code>uint64</code> | 预览对应的真实协作房间。 |
| <code>Workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。 |

<!-- write-step: 35 bytes=38974:39082 -->
````go
type structurePreviewUnit struct {
	Unit, Chapter, Room uint64
	Workspace           domain.EditWorkspace
}

````

## 第 36 步：structurePreviewContext

插入位置：第 35 步代码之后，紧接追加；完成稿约第 951 行。

原操作包含真实单元/支线代次及生成DTO；领取次数、HTTP超时都不能重写这些固定绑定。

本步声明：`type structurePreviewContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 请求拆合预览的账号。 |
| <code>Document</code> | <code>uint64</code> | 预览所属文档。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |
| <code>Intent</code> | <code>domain.StructurePreviewIntent</code> | 使用 <code>domain.StructurePreviewIntent</code> 类型；其定义处的职责：表达管理意图、拆合迁移候选和历史范围边界，并提供纯结构变换。 |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。 |
| <code>Manifest</code> | <code>uint64</code> | 本次操作固定的主线清单，包含各单元版本引用。 |
| <code>Structure</code> | <code>uint64</code> | 固定的目录结构版本。 |
| <code>Style</code> | <code>uint64</code> | 固定的文档样式版本。 |
| <code>Control</code> | <code>uint64</code> | 固定的主线控制修订上下文；后续准备/安装必须匹配，不能从当前值覆盖原值。 |
| <code>Units</code> | <code>&#91;&#93;structurePreviewUnit</code> | 按原结构固定的单元、章节和支线工作区列表。 |

<!-- write-step: 36 bytes=39082:39611 -->
````go
// 原操作包含真实单元/支线代次及生成DTO；领取次数、HTTP超时都不能重写这些固定绑定。
type structurePreviewContext struct {
	Actor, Document                     uint64
	SessionExpires                      int64
	Intent                              domain.StructurePreviewIntent
	Access                              DocumentAccess
	Context                             *pb.OperationContext
	Manifest, Structure, Style, Control uint64
	Units                               []structurePreviewUnit
}

````

## 第 37 步：NewStructurePreviews

插入位置：第 36 步代码之后，紧接追加；完成稿约第 962 行。

本步声明：`func NewStructurePreviews(db *sql.DB, identity *Identity, access *Access, client StructurePreviewClient, validator *codec.Validator, instance string) *StructurePreviews`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>StructurePreviewClient</code> | 使用 <code>StructurePreviewClient</code> 类型；其定义处的职责：拆合目录前保留主线与支线的真实范围映射；不要求支线先被主线强行覆盖成一样。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*StructurePreviews</code> | 实际返回表达式：<code>&amp;StructurePreviews{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;StructurePreviews{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=39611:39913 -->
````go
func NewStructurePreviews(db *sql.DB, identity *Identity, access *Access, client StructurePreviewClient, validator *codec.Validator, instance string) *StructurePreviews {
	return &StructurePreviews{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/structure_previews_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
