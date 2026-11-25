# 分步编写 `services/document/internal/logic/document/discard_previews_service.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/202-discard-previews-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定草稿、主线及阻塞审核意见，生成可重放的丢弃预览。

预览是有期限的事实，不改变工作区，却必须让后续确认能验证原内容和原用户权限。

写之前先按这条执行路线理解各段的连接：

1. RegisterPreview持久原绑定
2. RecoverOne续接原证明并捕获
3. validateDiscardPreviewSnapshot校验内容/来源
4. commitPreview落固定结果
5. ReadPreview重新鉴权展示。

本文件需要守住的边界：

- 普通预览只允许OPEN控制门
- 恢复证明过期不同于原会话过期
- 旧worker不得拒绝新代次任务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/discard_previews_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/discard_previews_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/discard_previews_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/discard_previews_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:735 -->
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

// discard_preview_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：DraftControls.commitPreview

插入位置：第 1 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (s *DraftControls) commitPreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext, snapshot discardPreviewSnapshot, reply *pb.DraftPreviewReply, raw []byte, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>s.validatePreview(ctx, tx, fixed, proofs)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>s.validatePreview(ctx, tx, fixed, proofs)</code>；<code>fixed.Scope</code>；<code>fixed.Access</code> |
| <code>snapshot</code> | <code>discardPreviewSnapshot</code> | 在此声明中的实际使用：<code>snapshot.MainVersion</code>；<code>snapshot.LineageProvenance</code>；<code>snapshot.Draft</code>；<code>snapshot.Candidate</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>reply</code> | <code>*pb.DraftPreviewReply</code> | 在此声明中的实际使用：<code>reply.ExpiresUnixMs</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>repository.InsertSubmissionTask(ctx, tx, &quot;DISCARD_PREVIEW_FIXED&quot;, task.Operation, task.Operation, raw, &quot;SUCCEEDED&quot;)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validatePreview(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_MAIN_VERSION_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_LINEAGE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ACCEPTED_CAPTURES_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; version != snapshot.MainVersion</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_MAIN_VERSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, snapshot.LineageProvenance)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_LINEAGE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(accepted, expected)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ACCEPTED_CAPTURES_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=735:2778 -->
````go
func (s *DraftControls) commitPreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext, snapshot discardPreviewSnapshot, reply *pb.DraftPreviewReply, raw []byte, proofs []*pb.IdentityAdmission) error {
	// 网络与正文校验在SQL锁外；最终落库仍在来源锁下复核原权限、实际谱系和领取代次。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validatePreview(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	version, err := repository.MainUnitVersion(ctx, tx, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Manifest)
	if err != nil || version != snapshot.MainVersion {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_MAIN_VERSION_UNKNOWN")
	}
	_, _, prior, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	lineage, err := codec.Decode(prior)
	if err != nil {
		return err
	}
	canonical, err := codec.Canonical(lineage)
	if err != nil || !bytes.Equal(canonical, snapshot.LineageProvenance) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_LINEAGE_UNKNOWN")
	}
	for _, content := range []*pb.FixedContent{snapshot.Draft, snapshot.Candidate} {
		value, _ := codec.Decode(content.ProvenanceJsonUtf8)
		accepted, _ := codec.Canonical(value.(map[string]any)["accepted_captures"])
		expected, _ := codec.Canonical(lineage.(map[string]any)["accepted_captures"])
		if !bytes.Equal(accepted, expected) {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ACCEPTED_CAPTURES_CHANGED")
		}
	}
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resources := map[uint64]bool{}
	for _, content := range []*pb.FixedContent{snapshot.Draft, snapshot.Main} {
		for _, id := range content.ResourceIds {
			resources[id] = true
		}
	}
````

## 第 3 步：DraftControls.commitPreview（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DISCARD_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>request == 0</code>。
- 检查 <code>request != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2778:4167 -->
````go
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, err := repository.LockedDocumentResource(ctx, tx, id, fixed.Scope.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "DISCARD_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, fixed.Scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	blocks, err := repository.DiscardPreviewBlockingChanges(ctx, tx, fixed.Scope.Unit)
	if err != nil {
		return err
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, fixed.Scope.Unit)
	if err != nil {
		return err
	}
	review := discardPreviewReview{Guard: guard}
	request := guard.Ongoing
	if request == 0 {
		request = guard.Resumable
	}
	if request != 0 {
		review.Pointer, err = repository.LockedReviewPointer(ctx, tx, request)
		if err != nil {
			return err
		}
	}
	reviewRaw, err := json.Marshal(review)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_REVIEW", task.Operation, task.Operation, reviewRaw, "SUCCEEDED"); err != nil {
		return err
	}
	oldDraft, err := repository.InsertMergeVersion(ctx, tx, snapshot.Draft, "RESTORE", task.Operation)
	if err != nil {
````

## 第 4 步：DraftControls.commitPreview（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 120 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 4 bytes=4167:5459 -->
````go
		return err
	}
	scope, workspace := fixed.Scope, fixed.Scope.Workspace
	public := map[string]any{"preview_cut_ref": task.Operation, "document_id": domain.TextID(scope.Document), "unit_id": domain.TextID(scope.Unit), "branch_id": domain.TextID(workspace.Workspace), "workspace_epoch": domain.TextID(workspace.Epoch), "lineage_id": domain.TextID(workspace.Lineage), "base_manifest_id": domain.TextID(workspace.BaseManifest), "draft_content_version_id": domain.TextID(oldDraft), "main_manifest_id": domain.TextID(scope.Manifest), "structure_id": domain.TextID(scope.Structure), "cut_digest": hex.EncodeToString(snapshot.Cut.DependencyDigest), "expires_at": time.UnixMilli(reply.ExpiresUnixMs).UTC().Format(time.RFC3339Nano), "blocking_changes": blocks}
	result, err := json.Marshal(public)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
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

// discard_preview_readiness.go：保留原实现，按业务合并文件。

````

## 第 5 步：validatePendingDiscardPreviewBinding

插入位置：第 4 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`func validatePendingDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingDiscardPreviewBinding</code> | 使用 <code>repository.PendingDiscardPreviewBinding</code> 类型；其定义处的职责：保存丢弃预览原上下文和可恢复捕获任务，并读取阻塞意见。<br>在此声明中的实际使用：<code>fact.State</code>；<code>fact.Original</code>；<code>fact.Payload</code>；<code>validateOriginalDiscardPreviewBinding(fact)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardPreviewContext</code> | 由 <code>validateOriginalDiscardPreviewBinding(fact)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>validateOriginalDiscardPreviewBinding(fact)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalid</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; json.Unmarshal(fact.Original, &amp;original) != nil &#124;&#124; json.Unmarshal(fact.Payload, &amp;progress) != nil</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return validateOriginalDiscardPreviewBinding(fact)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=5459:6021 -->
````go
func validatePendingDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error) {
	var original, progress discardPreviewContext
	invalid := errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
	if fact.State != "PREPARING" || json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &progress) != nil {
		return original, invalid
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(progress)
	if !bytes.Equal(a, b) {
		return original, invalid
	}
	return validateOriginalDiscardPreviewBinding(fact)
}

````

## 第 6 步：validateOriginalDiscardPreviewBinding

插入位置：第 5 步代码之后，紧接追加；完成稿约第 156 行。

启动检查只检查可恢复的原绑定；证明自然过期不等于原会话过期，也不能用重启生成新目标。

本步声明：`func validateOriginalDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingDiscardPreviewBinding</code> | 使用 <code>repository.PendingDiscardPreviewBinding</code> 类型；其定义处的职责：保存丢弃预览原上下文和可恢复捕获任务，并读取阻塞意见。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Actor</code>；<code>fact.Operation</code>；<code>fact.Fingerprint</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardPreviewContext</code> | 实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;original) != nil</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; original.Actor == 0 &#124;&#124; scope.Document == 0 &#124;&#124; scope.Unit == 0 &#124;&#124; scope.Room == 0 &#124;&#124; scope.Structure == 0 &#124;&#124; scope.Manifest == 0 &#124;&#124; scope.Style == …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(ctx.Target, expected) &#124;&#124; ctx.IdentityAdmissionRef == &quot;&quot; &#124;&#124; ctx.DocumentAdmissionRef == &quot;&quot; &#124;&#124; len(original.Access.Proofs) == 0 &#124;&#124; original.Access.Proofs&#91;0&#93; == nil</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parsed.Operation != fact.Operation &#124;&#124; parsed.Branch != w.Workspace &#124;&#124; parsed.Structure != scope.Structure</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Target, &amp;httpTarget) != nil &#124;&#124; httpTarget.Path != path</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint)</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != 0</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != original.Actor &#124;&#124; proof.OperationId != fact.Operation &#124;&#124; proof.Purpose != &quot;DISCARD_PREVIEW&quot; &#124;&#124; !proto.Equal(proof.Target, expected) &#124;&#124; 			len(proof …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=6021:9533 -->
````go
// 启动检查只检查可恢复的原绑定；证明自然过期不等于原会话过期，也不能用重启生成新目标。
func validateOriginalDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error) {
	var original discardPreviewContext
	invalid := errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &original) != nil {
		return original, invalid
	}
	ctx, scope := original.Context, original.Scope
	w := scope.Workspace
	if ctx == nil || ctx.Target == nil || original.Actor == 0 || scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Structure == 0 || scope.Manifest == 0 || scope.Style == 0 || scope.Control == 0 || original.SessionExpires <= 0 ||
		w.Workspace == 0 || w.Epoch == 0 || w.Lineage == 0 || w.BaseManifest == 0 || w.BaseVersion == 0 || original.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation ||
		original.Intent.Operation != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || !original.Access.Permissions.Manage ||
		original.Intent.Branch != w.Workspace || original.Intent.Structure != scope.Structure || original.Access.Document.Id != domain.TextID(scope.Document) || original.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) ||
		original.Access.Document.CurrentStructureId != domain.TextID(scope.Structure) || original.Access.Document.CurrentStyleId != domain.TextID(scope.Style) || original.Access.Document.ControlRevision != domain.TextID(scope.Control) ||
		len(ctx.Sources) == 0 || !proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: original.Access.Sources}) {
		return original, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: w.Workspace, StructureId: scope.Structure, WorkspaceEpoch: w.Epoch, Purpose: "DISCARD_PREVIEW"}
	if !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || len(original.Access.Proofs) == 0 || original.Access.Proofs[0] == nil {
		return original, invalid
	}
	parsed, err := domain.ParseDiscardPreview(original.Intent.Input)
	if err != nil || parsed.Operation != fact.Operation || parsed.Branch != w.Workspace || parsed.Structure != scope.Structure {
		return original, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-previews"
	var httpTarget struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Target, &httpTarget) != nil || httpTarget.Path != path {
		return original, invalid
	}
	fingerprint, err := domain.OperationFingerprint(path, original.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return original, invalid
	}
	available := map[string]uint64{}
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return original, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range original.Access.Proofs {
		if proof == nil || proof.ActorId != original.Actor || proof.OperationId != fact.Operation || proof.Purpose != "DISCARD_PREVIEW" || !proto.Equal(proof.Target, expected) ||
			len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, original.Access.Proofs[0].SessionRef) {
````

## 第 7 步：validateOriginalDiscardPreviewBinding（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 202 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return original, invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != source.Epoch</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 检查 <code>original.Access.Proofs&#91;0&#93;.AdmissionId != ctx.IdentityAdmissionRef</code>。条件成立时执行 <code>return original, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return original, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=9533:9863 -->
````go
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
	return original, nil
}

````

## 第 8 步：DraftControls.Ready

插入位置：第 7 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`func (s *DraftControls) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingDiscardPreviewBindings(ctx, s.db)</code>；<code>repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code>；<code>s.readyDiscards(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DISCARD_PREVIEW_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>s.readyDiscards(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateDiscardPreviewAdmission(fixed, admission) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.readyDiscards(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=9863:10770 -->
````go
func (s *DraftControls) Ready(ctx context.Context) error {
	facts, err := repository.PendingDiscardPreviewBindings(ctx, s.db)
	if err != nil {
		return errors.New("DISCARD_PREVIEW_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingDiscardPreviewBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardPreviewAdmission(fixed, admission) != nil {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
	}
	return s.readyDiscards(ctx)
}

// discard_preview_readings.go：保留原实现，按业务合并文件。

````

## 第 9 步：DraftControls.WaitPreview

插入位置：第 8 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func (s *DraftControls) WaitPreview(ctx context.Context, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.DiscardPreviewResult(ctx, s.db, operation)</code>；<code>ctx.Done</code>；<code>ctx.Err</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.DiscardPreviewResult(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>ctx.Err()</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_PREPARING&quot;)</code> |

**跟着本段代码走一遍：**

- 登记 <code>deadline.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return ctx.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_PREPARING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=10770:11338 -->
````go
func (s *DraftControls) WaitPreview(ctx context.Context, operation string) error {
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		fact, err := repository.DiscardPreviewResult(ctx, s.db, operation)
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
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_PREPARING")
		case <-ticker.C:
		}
	}
}

````

## 第 10 步：DraftControls.previewResultAccess

插入位置：第 9 步代码之后，紧接追加；完成稿约第 263 行。

本步声明：`func (s *DraftControls) previewResultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW_RESULT&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW_RESULT&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; !access.Permissions.Manage</code>。
- 此处通过 <code>return access, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=11338:11690 -->
````go
func (s *DraftControls) previewResultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_PREVIEW_RESULT")
	if err == nil && !access.Permissions.Manage {
		err = domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	return access, err
}

````

## 第 11 步：DraftControls.visiblePreview

插入位置：第 10 步代码之后，紧接追加；完成稿约第 271 行。

本步声明：`func (s *DraftControls) visiblePreview(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.DiscardPreviewResult(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.DiscardPreviewResult(ctx, tx, operation)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.DiscardPreviewResult(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.MainControlReceipt</code> | 使用 <code>repository.MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>实际返回表达式：<code>repository.MainControlReceipt{}</code>；<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;DISCARD_PREVIEW_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.MainControlReceipt{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;DISCARD_PREVIEW_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=11690:12325 -->
````go
func (s *DraftControls) visiblePreview(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return repository.MainControlReceipt{}, err
	}
	fact, err := repository.DiscardPreviewResult(ctx, tx, operation)
	if err != nil {
		return fact, err
	}
	if fact.Actor != actor.ID {
		return fact, domain.Violation(domain.EntityNotFound, "DISCARD_PREVIEW_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return fact, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	return fact, nil
}

````

## 第 12 步：discardPreviewRejection

插入位置：第 11 步代码之后，紧接追加；完成稿约第 288 行。

本步声明：`func discardPreviewRejection(fact repository.MainControlReceipt) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.MainControlReceipt</code> | 使用 <code>repository.MainControlReceipt</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>在此声明中的实际使用：<code>fact.State</code>；<code>fact.Result</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, rejected.Error.Code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.State != &quot;REJECTED&quot; &#124;&#124; json.Unmarshal(fact.Result, &amp;rejected) != nil &#124;&#124; rejected.Error.Code == &quot;&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, rejected.Error.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=12325:12763 -->
````go
func discardPreviewRejection(fact repository.MainControlReceipt) error {
	var rejected struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
	}
	return domain.Violation(domain.StateConflict, rejected.Error.Code)
}

````

## 第 13 步：DraftControls.ReadPreview

插入位置：第 12 步代码之后，紧接追加；完成稿约第 300 行。

本步声明：`func (s *DraftControls) ReadPreview(ctx context.Context, actor Actor, document, unit uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupPreview(ctx, operation)</code>；<code>s.previewResultAccess(ctx, actor, document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.previewResultAccess(ctx, actor, document)</code>；<code>s.visiblePreview(ctx, tx, actor, access, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fixed.Scope.Document != document</code>；<code>s.previewResultAccess(ctx, actor, document)</code>；<code>domain.TextID(document)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>fixed.Scope.Unit != unit</code>；<code>domain.TextID(unit)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.LookupPreview(ctx, operation)</code>；<code>s.visiblePreview(ctx, tx, actor, access, operation)</code>；<code>value.PreviewCutRef != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visiblePreview(ctx, tx, actor, access, operation) 		if err != nil { 			return nil, err 		} 		if fact …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;DISCARD_PREVIEW_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Scope.Document != document &#124;&#124; fixed.Scope.Unit != unit &#124;&#124; fixed.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;DISCARD_PREVIEW_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visiblePreview(ctx, tx, actor, access, operation) 		if err != nil { 			return …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_PREPARING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil, discardPreviewRejection(fact)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil &#124;&#124; value.PreviewCutRef != operation &#124;&#124; value.DocumentId != domain.TextID(document) &#124;&#124; value.UnitId != domain.TextID(unit)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=12763:14032 -->
````go
func (s *DraftControls) ReadPreview(ctx context.Context, actor Actor, document, unit uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupPreview(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Scope.Document != document || fixed.Scope.Unit != unit || fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "DISCARD_PREVIEW_NOT_FOUND")
	}
	access, err := s.previewResultAccess(ctx, actor, document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visiblePreview(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		if fact.State == "PREPARING" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_PREPARING")
		}
		if fact.State != "SUCCEEDED" {
			return nil, discardPreviewRejection(fact)
		}
		var value dto.DiscardPreview
		if json.Unmarshal(fact.Result, &value) != nil || value.PreviewCutRef != operation || value.DocumentId != domain.TextID(document) || value.UnitId != domain.TextID(unit) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		return value, nil
	}, nil
}

````

## 第 14 步：DraftControls.Progress

插入位置：第 13 步代码之后，紧接追加；完成稿约第 331 行。

公共Operation严格投影冻结字段。完整预览仅从原POST回放取得，私有RPC另核验完整原目标与指纹。

本步声明：`func (s *DraftControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupPreview(ctx, operation)</code>；<code>s.previewResultAccess(ctx, actor, fixed.Scope.Document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.previewResultAccess(ctx, actor, fixed.Scope.Document)</code>；<code>s.visiblePreview(ctx, tx, actor, access, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.LookupPreview(ctx, operation)</code>；<code>s.visiblePreview(ctx, tx, actor, access, operation)</code>；<code>&quot;operation_id&quot;: operation</code>；<code>&quot;/v1/operations/&quot; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visiblePreview(ctx, tx, actor, access, operation) 		if err != nil { 			return nil, err 		} 		result  …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		fact, err := s.visiblePreview(ctx, tx, actor, access, operation) 		if err != nil { 			return …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;PREPARING&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;preview) != nil &#124;&#124; preview.DraftContentVersionId == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;REJECTED&quot; &#124;&#124; json.Unmarshal(fact.Result, &amp;rejected) != nil &#124;&#124; rejected&#91;&quot;error&quot;&#93; == nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14032:15540 -->
````go
// 公共Operation严格投影冻结字段。完整预览仅从原POST回放取得，私有RPC另核验完整原目标与指纹。
func (s *DraftControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupPreview(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.previewResultAccess(ctx, actor, fixed.Scope.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visiblePreview(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"operation_id": operation, "state": fact.State}
		if fact.State == "PREPARING" {
			result["poll_url"] = "/v1/operations/" + operation
			return result, nil
		}
		if fact.State == "SUCCEEDED" {
			var preview dto.DiscardPreview
			if json.Unmarshal(fact.Result, &preview) != nil || preview.DraftContentVersionId == "" {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
			}
			result["result"] = map[string]any{"entity_id": preview.DraftContentVersionId}
			return result, nil
		}
		if err = discardPreviewRejection(fact); err != nil {
			var rejected map[string]any
			if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected["error"] == nil {
				return nil, err
			}
			result["error"] = rejected["error"]
		}
		return result, nil
	}, nil
}

````

## 第 15 步：DraftControls.OperationResult

插入位置：第 14 步代码之后，紧接追加；完成稿约第 370 行。

本步声明：`func (s *DraftControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.DiscardPreviewResult(ctx, s.db, query.OperationId)</code>；<code>s.DiscardOperationResult(ctx, query)</code>；<code>s.LookupPreview(ctx, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>s.DiscardOperationResult(ctx, query)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.DiscardOperationResult(ctx, query)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DISCARD_PREVIEW_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DISCARD_PREVIEW_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return s.DiscardOperationResult(ctx, query)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;preview) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>!ok &#124;&#124; violation.Kind != domain.StateConflict</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=15540:17676 -->
````go
func (s *DraftControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_PREVIEW_QUERY_INVALID")
	}
	fact, err := repository.DiscardPreviewResult(ctx, s.db, query.OperationId)
	if err == sql.ErrNoRows {
		return s.DiscardOperationResult(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	fixed, err := s.LookupPreview(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if fact.Actor != fixed.Actor {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	if fact.State == "SUCCEEDED" {
		var preview dto.DiscardPreview
		if json.Unmarshal(fact.Result, &preview) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		version, err := domain.CanonicalID(preview.DraftContentVersionId)
		if err != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		result.State, result.ResultIds = pb.OperationResult_SUCCEEDED, []uint64{version}
		return result, nil
	}
	if err = discardPreviewRejection(fact); err != nil {
		violation, ok := err.(domain.RuleViolation)
		if !ok || violation.Kind != domain.StateConflict {
			return nil, err
		}
		result.State, result.BusinessCode = pb.OperationResult_REJECTED, violation.Code
	}
````

## 第 16 步：DraftControls.OperationResult（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 416 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=17676:17776 -->
````go
	return result, nil
}

// discard_preview_recovery.go：保留原实现，按业务合并文件。

````

## 第 17 步：DraftControls.RecoverOne

插入位置：第 16 步代码之后，紧接追加；完成稿约第 421 行。

本步声明：`func (s *DraftControls) RecoverOne(parent context.Context)`。


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
- 检查 <code>!bytes.Equal(a, b)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.StateConflict &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 17 bytes=17776:19150 -->
````go
func (s *DraftControls) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimDiscardPreviewTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed discardPreviewContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	} else {
		original, readErr := s.LookupPreview(ctx, task.Operation)
		err = readErr
		if err == nil {
			a, _ := json.Marshal(original)
			b, _ := json.Marshal(fixed)
			if !bytes.Equal(a, b) {
				err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
			}
		}
		if err == nil {
			err = s.capturePreview(ctx, task, fixed)
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

## 第 18 步：DraftControls.capturePreview

插入位置：第 17 步代码之后，紧接追加；完成稿约第 464 行。

本步声明：`func (s *DraftControls) capturePreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resumePreview(ctx, fixed)</code>；<code>s.validateBeforePreviewCapture(ctx, fixed, proofs)</code>；<code>s.client.Preview(ctx, input)</code>；<code>s.client.FixedDraft(ctx, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.commitPreview(ctx, task, fixed, snapshot, reply, raw, proofs)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>s.resumePreview(ctx, fixed)</code>；<code>s.validateBeforePreviewCapture(ctx, fixed, proofs)</code>；<code>discardPreviewRequest(fixed)</code>；<code>validateDiscardPreviewSnapshot(s.validator, raw, fixed, reply)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>discardPreviewCaptureError(err)</code>；<code>domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_EXPIRED&quot;)</code>；<code>s.commitPreview(ctx, task, fixed, snapshot, reply, raw, proofs)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return discardPreviewCaptureError(err)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.commitPreview(ctx, task, fixed, snapshot, reply, raw, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=19150:19984 -->
````go
func (s *DraftControls) capturePreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext) error {
	proofs, err := s.resumePreview(ctx, fixed)
	if err != nil {
		return err
	}
	if err = s.validateBeforePreviewCapture(ctx, fixed, proofs); err != nil {
		return err
	}
	input := discardPreviewRequest(fixed)
	reply, err := s.client.Preview(ctx, input)
	if err != nil {
		return discardPreviewCaptureError(err)
	}
	raw, err := s.client.FixedDraft(ctx, input)
	if err != nil {
		return err
	}
	snapshot, err := validateDiscardPreviewSnapshot(s.validator, raw, fixed, reply)
	if err != nil {
		return err
	}
	if reply.ExpiresUnixMs <= time.Now().UnixMilli() {
		return domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	return s.commitPreview(ctx, task, fixed, snapshot, reply, raw, proofs)
}

````

## 第 19 步：discardPreviewCaptureError

插入位置：第 18 步代码之后，紧接追加；完成稿约第 491 行。

本步声明：`func discardPreviewCaptureError(err error) error`。


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
- 检查 <code>rpc.Code() == codes.FailedPrecondition &amp;&amp; (rpc.Message() == &quot;WORKSPACE_CHANGED&quot; &#124;&#124; rpc.Message() == &quot;LINEAGE_CHANGED&quot;)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, rpc.Message())</code>。当前函数在这里结束。
- 检查 <code>rpc.Code() == codes.ResourceExhausted &amp;&amp; rpc.Message() == &quot;DISCARD_TRANSFER_LIMIT&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, rpc.Message())</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=19984:20456 -->
````go
func discardPreviewCaptureError(err error) error {
	rpc, ok := status.FromError(err)
	if !ok {
		return err
	}
	if rpc.Code() == codes.FailedPrecondition && (rpc.Message() == "WORKSPACE_CHANGED" || rpc.Message() == "LINEAGE_CHANGED") {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	if rpc.Code() == codes.ResourceExhausted && rpc.Message() == "DISCARD_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	return err
}

````

## 第 20 步：rejectPreviewTask

插入位置：第 19 步代码之后，紧接追加；完成稿约第 505 行。

拒绝结果和任务终态在同一事务落定；先复核原任务代次，避免旧 worker 拒绝新的预约。

本步声明：`func rejectPreviewTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.UpdateOperationResult(ctx, tx, task.Operation, &quot;REJECTED&quot;, result)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.BeginTx</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 20 bytes=20456:21447 -->
````go
// 拒绝结果和任务终态在同一事务落定；先复核原任务代次，避免旧 worker 拒绝新的预约。
func rejectPreviewTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	tx, err := db.BeginTx(ctx, nil)
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
	result, err := json.Marshal(map[string]any{"error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_preview_register.go：保留原实现，按业务合并文件。

````

## 第 21 步：DraftControls.RegisterPreview

插入位置：第 20 步代码之后，紧接追加；完成稿约第 533 行。

本步声明：`func (s *DraftControls) RegisterPreview(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardPreviewIntent, path string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW&quot;)</code>；<code>repository.DiscardPreviewResult(ctx, s.db, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)</code><br>另有 14 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW&quot;)</code>；<code>return actor, err</code>；<code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 15 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_PREVIEW&quot;)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>DocumentId: document</code>；<code>repository.LockedMergeBarrier(ctx, tx, document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)</code>；<code>UnitId: unit</code>；<code>repository.LockedActiveBranch(ctx, tx, unit, document)</code>；<code>repository.LockedEditWorkspace(ctx, tx, intent.Branch, document, unit)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.DiscardPreviewIntent</code> | 使用 <code>domain.DiscardPreviewIntent</code> 类型；其定义处的职责：预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。<br>在此声明中的实际使用：<code>intent.Input</code>；<code>intent.Operation</code>；<code>intent.Branch</code>；<code>intent.Structure</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, intent.Input)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>fact.Actor != actor.ID &#124;&#124; !bytes.Equal(fact.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>access.Document.CurrentStructureId != domain.TextID(intent.Structure)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(access.Proofs) == 0</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_IDENTITY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=21447:23373 -->
````go
func (s *DraftControls) RegisterPreview(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardPreviewIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_PREVIEW")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Manage {
		return actor, domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	fingerprint, err := domain.OperationFingerprint(path, intent.Input)
	if err != nil {
		return actor, err
	}
	actor.Sources = access.Sources
	if fact, err := repository.DiscardPreviewResult(ctx, s.db, intent.Operation); err == nil {
		if fact.Actor != actor.ID || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if err != sql.ErrNoRows {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	if access.Document.CurrentStructureId != domain.TextID(intent.Structure) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: workspace.Epoch, Purpose: "DISCARD_PREVIEW"}
	if len(access.Proofs) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_IDENTITY_UNKNOWN")
	}
	var boundProofs []*pb.IdentityAdmission
	boundProofs, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = boundProofs
	actor.Proof = access.Proofs[0]
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, intent.Operation); err != nil {
````

## 第 22 步：DraftControls.RegisterPreview（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 578 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>fact.ActorID != actor.ID &#124;&#124; fact.Kind != &quot;DISCARD_PREVIEW&quot; &#124;&#124; !bytes.Equal(fact.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; active != intent.Branch</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>actual != workspace &#124;&#124; current.CurrentStructureId != domain.TextID(intent.Structure)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;DRAFT_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 22 bytes=23373:25202 -->
````go
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if fact, err := repository.LockedOperation(ctx, tx, intent.Operation); err == nil {
		if fact.ActorID != actor.ID || fact.Kind != "DISCARD_PREVIEW" || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if err != sql.ErrNoRows {
		return actor, err
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, unit, document)
	if err != nil || active != intent.Branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	actual, err := repository.LockedEditWorkspace(ctx, tx, intent.Branch, document, unit)
	if err != nil {
		return actor, err
	}
	if actual != workspace || current.CurrentStructureId != domain.TextID(intent.Structure) {
		return actor, domain.Violation(domain.StateConflict, "DRAFT_CONTEXT_CHANGED")
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", workspace.Workspace, unit, workspace.Epoch)
	if err != nil {
		return actor, err
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalID(current.ControlRevision)
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	// 授权到锁定文档之间MAIN可合法推进。权限已按ACL/结构复核，原上下文须保存同一最终文档快照。
````

## 第 23 步：DraftControls.RegisterPreview（接着实现）

插入位置：第 22 步代码之后，紧接追加；完成稿约第 625 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 23 bytes=25202:26757 -->
````go
	access.Document = current
	fixed := discardPreviewContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Scope: draftSourceScope{Document: document, Unit: unit, Room: room, Manifest: manifest, Structure: intent.Structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: workspace}}
	command := OperationCommand{ID: intent.Operation, Kind: "DISCARD_PREVIEW", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, map[string]any{"operation_id": intent.Operation, "state": "PREPARING"}); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "DISCARD_PREVIEW", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterDiscardPreview(ctx, tx, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// discard_preview_snapshot.go：保留原实现，按业务合并文件。

````

## 第 24 步：discardPreviewSnapshot

插入位置：第 23 步代码之后，紧接追加；完成稿约第 652 行。

本步声明：`type discardPreviewSnapshot struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Draft</code> | <code>*pb.FixedContent</code> | 固定的原草稿正文，供预览舍弃损失。 |
| <code>Main</code> | <code>*pb.FixedContent</code> | 固定的主线正文。 |
| <code>Candidate</code> | <code>*pb.FixedContent</code> | 将替换草稿的固定候选正文。 |
| <code>Cut</code> | <code>*pb.CutVector</code> | 采样时各房间的持久序号向量，确认前需复核。 |
| <code>MainVersion</code> | <code>uint64</code> | 主线内容对应的固定正文版本号。 |
| <code>LineageProvenance</code> | <code>&#91;&#93;byte</code> | 预览使用的支线谱系来源映射字节。 |

<!-- write-step: 24 bytes=26757:26938 -->
````go
type discardPreviewSnapshot struct {
	Draft, Main, Candidate *pb.FixedContent
	Cut                    *pb.CutVector
	MainVersion            uint64
	LineageProvenance      []byte
}

````

## 第 25 步：validateDiscardPreviewSnapshot

插入位置：第 24 步代码之后，紧接追加；完成稿约第 659 行。

本步声明：`func validateDiscardPreviewSnapshot(validator *codec.Validator, raw []byte, fixed discardPreviewContext, reply *pb.DraftPreviewReply) (discardPreviewSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator == nil</code>；<code>validator.Provenance</code>；<code>validateFixedMergeContent(validator, content, scope.Document, scope.Unit, scope.Structure)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;envelope)</code>；<code>codec.Decode(raw)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>discardPreviewRequest(fixed)</code> |
| <code>reply</code> | <code>*pb.DraftPreviewReply</code> | 在此声明中的实际使用：<code>reply == nil</code>；<code>reply.State</code>；<code>reply.PreviewCutRef</code>；<code>reply.MainManifestId</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardPreviewSnapshot</code> | 实际返回表达式：<code>snapshot</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>epoch == 0 &#124;&#124; epoch &gt; scope.Control</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>validator == nil &#124;&#124; reply == nil &#124;&#124; reply.State != pb.CaptureReply_FIXED &#124;&#124; reply.PreviewCutRef != operation &#124;&#124; reply.MainManifestId != scope.Manifest &#124;&#124; reply.ObservedCut == nil &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;envelope) != nil &#124;&#124; envelope.Room != domain.TextID(scope.Room) &#124;&#124; envelope.Preview != operation &#124;&#124; envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(envelope.Binding, binding) != nil &#124;&#124; !proto.Equal(binding, discardPreviewRequest(fixed))</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=26938:29463 -->
````go
func validateDiscardPreviewSnapshot(validator *codec.Validator, raw []byte, fixed discardPreviewContext, reply *pb.DraftPreviewReply) (discardPreviewSnapshot, error) {
	var snapshot discardPreviewSnapshot
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	scope, operation := fixed.Scope, fixed.Intent.Operation
	if epoch := draftMainEpoch(scope); epoch == 0 || epoch > scope.Control {
		return snapshot, invalid
	}
	if validator == nil || reply == nil || reply.State != pb.CaptureReply_FIXED || reply.PreviewCutRef != operation || reply.MainManifestId != scope.Manifest || reply.ObservedCut == nil || reply.ExpiresUnixMs <= 0 || len(reply.InputDigest) != 32 || len(raw) > 24<<20 {
		return snapshot, invalid
	}
	var envelope struct {
		Binding                json.RawMessage   `json:"binding"`
		Room                   string            `json:"room_id"`
		DraftCut               json.RawMessage   `json:"draft_cut"`
		MainCut                []json.RawMessage `json:"main_cut"`
		Draft, Main, Candidate json.RawMessage
		Conflicts              []json.RawMessage `json:"conflicts"`
		Digest                 []byte            `json:"input_digest"`
		Preview                string            `json:"preview_cut_ref"`
		Expires                string            `json:"expires_unix_ms"`
		Source                 struct {
			Purpose   string `json:"purpose"`
			Room      string `json:"room_id"`
			Manifest  string `json:"main_manifest_id"`
			Version   string `json:"main_version_id"`
			Structure string `json:"structure_id"`
			Style     string `json:"style_id"`
			Control   string `json:"control_revision"`
			MainEpoch string `json:"workspace_epoch"`
			Lineage   struct {
				ID         string          `json:"lineage_id"`
				Manifest   string          `json:"base_manifest_id"`
				Version    string          `json:"base_version_id"`
				Provenance json.RawMessage `json:"provenance"`
			} `json:"lineage"`
		} `json:"source"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Room != domain.TextID(scope.Room) || envelope.Preview != operation || envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs)) || !bytes.Equal(envelope.Digest, reply.InputDigest) || len(envelope.Conflicts) != 0 || len(envelope.MainCut) > 1 {
		return snapshot, invalid
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(envelope.Binding, binding) != nil || !proto.Equal(binding, discardPreviewRequest(fixed)) {
		return snapshot, invalid
	}
	value, err := codec.Decode(raw)
````

## 第 26 步：validateDiscardPreviewSnapshot（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 704 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest&#91;:&#93;, reply.InputDigest)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>source.MainEpoch != domain.TextID(draftMainEpoch(scope)) &amp;&amp; !(scope.MainEpoch == 0 &amp;&amp; source.MainEpoch == &quot;&quot;)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>source.Purpose != &quot;DISCARD_PREVIEW&quot; &#124;&#124; source.Room != domain.TextID(scope.Room) &#124;&#124; source.Manifest != domain.TextID(scope.Manifest) &#124;&#124; source.Structure != domain.TextID(scope.Struc …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validator.Provenance(lineage) != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(envelope.DraftCut, draftCut) != nil &#124;&#124; draftCut.RoomId != scope.Room &#124;&#124; draftCut.WorkspaceEpoch != scope.Workspace.Epoch</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>len(envelope.MainCut) == 1</code>。
- 检查 <code>protojson.Unmarshal(envelope.MainCut&#91;0&#93;, mainCut) != nil &#124;&#124; mainCut.RoomId == 0 &#124;&#124; mainCut.RoomId == scope.Room &#124;&#124; mainCut.WorkspaceEpoch != draftMainEpoch(scope)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。

<!-- write-step: 26 bytes=29463:31676 -->
````go
	if err != nil {
		return snapshot, invalid
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return snapshot, invalid
	}
	delete(fields, "input_digest")
	canonical, err := codec.Canonical(fields)
	if err != nil {
		return snapshot, invalid
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(digest[:], reply.InputDigest) {
		return snapshot, invalid
	}
	source := envelope.Source
	// 旧固定包缺省workspace_epoch仍按其原control解释；新增原包必须保存实际物理来源。
	if source.MainEpoch != domain.TextID(draftMainEpoch(scope)) && !(scope.MainEpoch == 0 && source.MainEpoch == "") {
		return snapshot, invalid
	}
	if source.Purpose != "DISCARD_PREVIEW" || source.Room != domain.TextID(scope.Room) || source.Manifest != domain.TextID(scope.Manifest) || source.Structure != domain.TextID(scope.Structure) || source.Style != domain.TextID(scope.Style) || source.Control != domain.TextID(scope.Control) ||
		source.Lineage.ID != domain.TextID(scope.Workspace.Lineage) || source.Lineage.Manifest != domain.TextID(scope.Workspace.BaseManifest) || source.Lineage.Version != domain.TextID(scope.Workspace.BaseVersion) {
		return snapshot, invalid
	}
	if snapshot.MainVersion, err = domain.CanonicalID(source.Version); err != nil {
		return snapshot, invalid
	}
	lineage, err := codec.Decode(source.Lineage.Provenance)
	if err != nil || validator.Provenance(lineage) != nil {
		return snapshot, invalid
	}
	snapshot.LineageProvenance, err = codec.Canonical(lineage)
	if err != nil {
		return snapshot, invalid
	}
	draftCut := &pb.RoomCut{}
	if protojson.Unmarshal(envelope.DraftCut, draftCut) != nil || draftCut.RoomId != scope.Room || draftCut.WorkspaceEpoch != scope.Workspace.Epoch {
		return snapshot, invalid
	}
	snapshot.Cut = &pb.CutVector{DocumentId: scope.Document, StructureId: scope.Structure, StyleId: scope.Style, ControlRevision: scope.Control, Rooms: []*pb.RoomCut{draftCut}}
	mainCut := &pb.RoomCut{WorkspaceEpoch: draftMainEpoch(scope)}
	if len(envelope.MainCut) == 1 {
		if protojson.Unmarshal(envelope.MainCut[0], mainCut) != nil || mainCut.RoomId == 0 || mainCut.RoomId == scope.Room || mainCut.WorkspaceEpoch != draftMainEpoch(scope) {
			return snapshot, invalid
		}
````

## 第 27 步：validateDiscardPreviewSnapshot（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 750 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(snapshot.Cut, reply.ObservedCut)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>protojson.Unmarshal(raw, content) != nil &#124;&#124; content.OperationId != operation &#124;&#124; content.CaptureId != ref &#124;&#124; content.BranchId != branch &#124;&#124; content.WorkspaceEpoch != epoch</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !bytes.Equal(projected, content.BodyJsonUtf8)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !bytes.Equal(cutBytes, content.CutJsonUtf8)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok &#124;&#124; provenance&#91;&quot;source_operation_id&quot;&#93; != operation &#124;&#124; provenance&#91;&quot;lineage_id&quot;&#93; != content.LineageRef</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>main</code>。
- 在本段的匿名函数内：检查 <code>content.LineageRef != domain.TextID(scope.Workspace.Lineage)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>provenance&#91;&quot;base_manifest_id&quot;&#93; != domain.TextID(baseManifest) &#124;&#124; provenance&#91;&quot;base_content_version_id&quot;&#93; != domain.TextID(baseVersion)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return content, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=31676:33855 -->
````go
		snapshot.Cut.Rooms = append(snapshot.Cut.Rooms, mainCut)
	}
	rooms, err := codec.Canonical(domain.CutBinding(snapshot.Cut)["rooms"])
	if err != nil {
		return snapshot, invalid
	}
	roomDigest := sha256.Sum256(rooms)
	snapshot.Cut.DependencyDigest = roomDigest[:]
	if !proto.Equal(snapshot.Cut, reply.ObservedCut) {
		return snapshot, invalid
	}
	decode := func(raw json.RawMessage, ref string, branch, epoch uint64, cut *pb.RoomCut, main bool) (*pb.FixedContent, error) {
		content := &pb.FixedContent{}
		if protojson.Unmarshal(raw, content) != nil || content.OperationId != operation || content.CaptureId != ref || content.BranchId != branch || content.WorkspaceEpoch != epoch {
			return nil, invalid
		}
		if err := validateFixedMergeContent(validator, content, scope.Document, scope.Unit, scope.Structure); err != nil {
			return nil, err
		}
		projected, err := domain.BranchBaselineBody(content.BodyJsonUtf8)
		if err != nil || !bytes.Equal(projected, content.BodyJsonUtf8) {
			return nil, invalid
		}
		cutBytes, err := codec.Canonical(map[string]any{"room_id": domain.TextID(cut.RoomId), "workspace_epoch": domain.TextID(cut.WorkspaceEpoch), "durable_seq": domain.TextID(cut.DurableSeq)})
		if err != nil || !bytes.Equal(cutBytes, content.CutJsonUtf8) {
			return nil, invalid
		}
		provenanceValue, _ := codec.Decode(content.ProvenanceJsonUtf8)
		provenance, ok := provenanceValue.(map[string]any)
		if !ok || provenance["source_operation_id"] != operation || provenance["lineage_id"] != content.LineageRef {
			return nil, invalid
		}
		baseManifest, baseVersion := scope.Workspace.BaseManifest, scope.Workspace.BaseVersion
		if main {
			baseManifest, baseVersion = scope.Manifest, snapshot.MainVersion
		} else if content.LineageRef != domain.TextID(scope.Workspace.Lineage) {
			return nil, invalid
		}
		if provenance["base_manifest_id"] != domain.TextID(baseManifest) || provenance["base_content_version_id"] != domain.TextID(baseVersion) {
			return nil, invalid
		}
		return content, nil
	}
	if snapshot.Draft, err = decode(envelope.Draft, operation, scope.Workspace.Workspace, scope.Workspace.Epoch, draftCut, false); err != nil {
		return snapshot, err
	}
````

## 第 28 步：validateDiscardPreviewSnapshot（接着实现）

插入位置：第 27 步代码之后，紧接追加；完成稿约第 796 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!proto.Equal(snapshot.Draft, reply.Draft)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return snapshot, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(snapshot.Candidate.BodyJsonUtf8, snapshot.Main.BodyJsonUtf8) &#124;&#124; !bytes.Equal(snapshot.Candidate.BodyHash, snapshot.Main.BodyHash)</code>。条件成立时执行 <code>return snapshot, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return snapshot, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=33855:34668 -->
````go
	if !proto.Equal(snapshot.Draft, reply.Draft) {
		return snapshot, invalid
	}
	if snapshot.Main, err = decode(envelope.Main, operation+":main", 0, scope.Control, mainCut, true); err != nil {
		return snapshot, err
	}
	if snapshot.Candidate, err = decode(envelope.Candidate, operation+":candidate", scope.Workspace.Workspace, scope.Workspace.Epoch, draftCut, false); err != nil {
		return snapshot, err
	}
	// 丢弃候选只能来自这一份可靠MAIN；即使整个包重新计算了摘要，也不能夹带另一份有效正文。
	if !bytes.Equal(snapshot.Candidate.BodyJsonUtf8, snapshot.Main.BodyJsonUtf8) || !bytes.Equal(snapshot.Candidate.BodyHash, snapshot.Main.BodyHash) {
		return snapshot, invalid
	}
	return snapshot, nil
}

// discard_preview_source.go：保留原实现，按业务合并文件。

````

## 第 29 步：DraftControls.LookupPreview

插入位置：第 28 步代码之后，紧接追加；完成稿约第 814 行。

本步声明：`func (s *DraftControls) LookupPreview(ctx context.Context, operation string) (discardPreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_PREVIEW_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_PREVIEW_CONTEXT&quot;, operation)</code>；<code>fixed.Context.OperationId != operation</code>；<code>fixed.Intent.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardPreviewContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.OperationId != operation &#124;&#124; fixed.Intent.Operation != operation</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=34668:35239 -->
````go
func (s *DraftControls) LookupPreview(ctx context.Context, operation string) (discardPreviewContext, error) {
	var fixed discardPreviewContext
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_PREVIEW_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return fixed, nil
}

````

## 第 30 步：DraftControls.resumePreview

插入位置：第 29 步代码之后，紧接追加；完成稿约第 826 行。

本步声明：`func (s *DraftControls) resumePreview(ctx context.Context, fixed discardPreviewContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=35239:35760 -->
````go
func (s *DraftControls) resumePreview(ctx context.Context, fixed discardPreviewContext) ([]*pb.IdentityAdmission, error) {
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

## 第 31 步：validateDiscardPreviewAdmission

插入位置：第 30 步代码之后，紧接追加；完成稿约第 841 行。

本步声明：`func validateDiscardPreviewAdmission(fixed discardPreviewContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code>；<code>fixed.Scope</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>ctx == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil &#124;&#124; actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Scope.Document &#124;&#124; actual.Unit != fixed.Scope …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, ctx.Target)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=35760:36651 -->
````go
func validateDiscardPreviewAdmission(fixed discardPreviewContext, actual repository.OriginalAdmissionBinding) error {
	ctx := fixed.Context
	if ctx == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit ||
		actual.Identity != ctx.IdentityAdmissionRef || actual.Operation != ctx.OperationId || actual.Purpose != "DISCARD_PREVIEW" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, ctx.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return nil
}

````

## 第 32 步：DraftControls.validatePreview

插入位置：第 31 步代码之后，紧接追加；完成稿约第 854 行。

本步声明：`func (s *DraftControls) validatePreview(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.validatePreviewForControl(ctx, tx, fixed, proofs, &quot;&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=36651:36860 -->
````go
func (s *DraftControls) validatePreview(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error {
	return s.validatePreviewForControl(ctx, tx, fixed, proofs, "")
}

````

## 第 33 步：DraftControls.validatePreviewForControl

插入位置：第 32 步代码之后，紧接追加；完成稿约第 858 行。

确认只可在自己的业务占位下复核原预览；普通预览必须看到OPEN，不能借用其他原号。

本步声明：`func (s *DraftControls) validatePreviewForControl(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission, owner string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Scope</code>；<code>fixed.Context</code>；<code>validateDiscardPreviewAdmission(fixed, admission)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |
| <code>owner</code> | <code>string</code> | 在此声明中的实际使用：<code>control != owner</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Manage</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(scope.Manifest) &#124;&#124; current.CurrentStructureId != domain.TextID(scope.Structure) &#124;&#124; current.CurrentStyleId != domain.TextID(scope.Style) &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; active != scope.Workspace.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>workspace != scope.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>control != owner</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_CONTROL_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 33 bytes=36860:38878 -->
````go
// 确认只可在自己的业务占位下复核原预览；普通预览必须看到OPEN，不能借用其他原号。
func (s *DraftControls) validatePreviewForControl(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission, owner string) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	scope := fixed.Scope
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	// 丢弃预览只读取可靠主线；主线锁定是允许状态，但原结构/清单/control不可被换掉。
	if current.CurrentManifestId != domain.TextID(scope.Manifest) || current.CurrentStructureId != domain.TextID(scope.Structure) || current.CurrentStyleId != domain.TextID(scope.Style) || current.ControlRevision != domain.TextID(scope.Control) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, scope.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)
	if err != nil || active != scope.Workspace.Workspace {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, active, scope.Document, scope.Unit)
	if err != nil {
		return err
	}
	if workspace != scope.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	control, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return err
	}
	if control != owner {
		return domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
````

## 第 34 步：DraftControls.validatePreviewForControl（接着实现）

插入位置：第 33 步代码之后，紧接追加；完成稿约第 904 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;DISCARD_PREVIEW_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=38878:39712 -->
````go
	if err = validateDiscardPreviewAdmission(fixed, admission); err != nil {
		return err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "DISCARD_PREVIEW_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

````

## 第 35 步：DraftControls.PreviewSource

插入位置：第 34 步代码之后，紧接追加；完成稿约第 924 行。

本步声明：`func (s *DraftControls) PreviewSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupPreview(ctx, input.Context.OperationId)</code>；<code>s.resumePreview(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validatePreview(ctx, tx, fixed, proofs)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(input, discardPreviewRequest(fixed))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input, discardPreviewRequest(fixed))</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=39712:40587 -->
````go
func (s *DraftControls) PreviewSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.LookupPreview(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, discardPreviewRequest(fixed)) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resumePreview(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.validatePreview(ctx, tx, fixed, proofs); err != nil {
		return nil, err
	}
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err == nil {
		value["purpose"] = "DISCARD_PREVIEW"
	}
	return value, err
}

````

## 第 36 步：DraftControls.validateBeforePreviewCapture

插入位置：第 35 步代码之后，紧接追加；完成稿约第 954 行。

本步声明：`func (s *DraftControls) validateBeforePreviewCapture(ctx context.Context, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validatePreview(ctx, tx, fixed, proofs)</code> |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>s.validatePreview(ctx, tx, fixed, proofs)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validatePreview(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.validatePreview(ctx, tx, fixed, proofs)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return s.validatePreview(ctx, tx, fixed, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=40587:40999 -->
````go
func (s *DraftControls) validateBeforePreviewCapture(ctx context.Context, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error {
	// Go已能确定的失效不依赖Node在线。这里只短期核验后回滚释放锁，RPC和最终提交仍各自复核。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.validatePreview(ctx, tx, fixed, proofs)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/discard_previews_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
