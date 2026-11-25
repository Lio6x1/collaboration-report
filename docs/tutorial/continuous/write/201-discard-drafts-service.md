# 分步编写 `services/document/internal/logic/document/discard_drafts_service.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/201-discard-drafts-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把已确认的固定丢弃预览变成新支线基线，并恢复安装及审核归档。

确认必须消费用户已看到的预览，不允许临时重新采样正文形成另一候选。

写之前先按这条执行路线理解各段的连接：

1. RegisterDiscard核预览/审核/占位
2. expectedDiscard和deriveDiscardContent只重绑本次操作来源
3. commitDiscard提交lineage与install
4. reconcileDiscard等真实ACK后归档。

本文件需要守住的边界：

- 原预览过期、审核意见变更或workspace/lineage变化必须拒绝
- 未知安装不能提前释放控制。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/discard_drafts_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/discard_drafts_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/discard_drafts_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/discard_drafts_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:615 -->
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
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// discard_draft.go：保留原实现，按业务合并文件。

````

## 第 2 步：discardPreviewReview

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type discardPreviewReview struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Guard</code> | <code>domain.ReviewGuard</code> | 使用 <code>domain.ReviewGuard</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。 |
| <code>Pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。 |

<!-- write-step: 2 bytes=615:711 -->
````go
type discardPreviewReview struct {
	Guard   domain.ReviewGuard
	Pointer domain.ReviewPointer
}

````

## 第 3 步：discardDraftContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type discardDraftContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 确认舍弃草稿的账号。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |
| <code>Intent</code> | <code>domain.DiscardDraftIntent</code> | 使用 <code>domain.DiscardDraftIntent</code> 类型；其定义处的职责：DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。 |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。 |
| <code>Scope</code> | <code>draftSourceScope</code> | 原支线工作区及主线结构范围。 |
| <code>Preview</code> | <code>discardPreviewContext</code> | 用户此前查看的舍弃预览上下文。 |
| <code>PreviewResult</code> | <code>dto.DiscardPreview</code> | 已固定且展示给用户的预览结果。 |
| <code>PreviewDigest</code> | <code>&#91;&#93;byte</code> | 这份预览的摘要；确认必须匹配原预览。 |
| <code>Review</code> | <code>discardPreviewReview</code> | 舍弃前固定的审核守卫与审核指针，防止干扰正在审核的提交。 |

<!-- write-step: 3 bytes=711:1069 -->
````go
type discardDraftContext struct {
	Actor          uint64
	SessionExpires int64
	Intent         domain.DiscardDraftIntent
	Access         DocumentAccess
	Context        *pb.OperationContext
	Scope          draftSourceScope
	Preview        discardPreviewContext
	PreviewResult  dto.DiscardPreview
	PreviewDigest  []byte
	Review         discardPreviewReview
}

````

## 第 4 步：discardDraftInstall

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`type discardDraftInstall struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>draftInstall</code> | <code>draftInstall</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>Fixed</code> | <code>discardDraftContext</code> | SQL提交的舍弃草稿上下文；Node安装和恢复沿用，不再重新采样。 |

<!-- write-step: 4 bytes=1069:1147 -->
````go
type discardDraftInstall struct {
	draftInstall
	Fixed discardDraftContext
}

````

## 第 5 步：DraftControls.LookupDiscard

插入位置：第 4 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func (s *DraftControls) LookupDiscard(ctx context.Context, operation string) (discardDraftContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_DRAFT_CONTEXT&quot;, operation)</code>；<code>fixed.Context.OperationId != operation</code>；<code>fixed.Intent.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardDraftContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.OperationId != operation &#124;&#124; fixed.Intent.Operation != operation &#124;&#124; fixed. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1147:1828 -->
````go
func (s *DraftControls) LookupDiscard(ctx context.Context, operation string) (discardDraftContext, error) {
	var fixed discardDraftContext
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_DRAFT_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation || fixed.Context.Target.Purpose != "DISCARD_DRAFT" {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	return fixed, nil
}

// discard_draft_commit.go：保留原实现，按业务合并文件。

````

## 第 6 步：DraftControls.commitDiscard

插入位置：第 5 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func (s *DraftControls) commitDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, prepared draftPrepared, proofs, original []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>s.validateDiscard(ctx, tx, fixed, proofs, original)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteDiscardDraft(ctx, tx, task, &quot;SUCCEEDED&quot;, result, encoded)</code> |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>s.validateDiscard(ctx, tx, fixed, proofs, original)</code>；<code>fixed.Scope</code>；<code>fixed.Access</code>；<code>Fixed: fixed</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>draftPrepared</code> | 在此声明中的实际使用：<code>prepared.Main</code>；<code>prepared.Candidate</code>；<code>prepared.Preview</code>；<code>prepared.Digest</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validateDiscard(ctx, tx, fixed, proofs, original)</code> |
| <code>original</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validateDiscard(ctx, tx, fixed, proofs, original)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>e</code>；<code>domain.Violation(domain.StateConflict, &quot;DISCARD_RESOURCE_NOT_READY&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DISCARD_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1828:3385 -->
````go
func (s *DraftControls) commitDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, prepared draftPrepared, proofs, original []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validateDiscard(ctx, tx, fixed, proofs, original); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	scope := fixed.Scope
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resources := map[uint64]bool{}
	for _, content := range []*pb.FixedContent{prepared.Main, prepared.Candidate} {
		for _, id := range content.ResourceIds {
			resources[id] = true
		}
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, e := repository.LockedDocumentResource(ctx, tx, id, scope.Document, space)
		if e != nil {
			return e
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "DISCARD_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	version, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
	if err != nil {
		return err
	}
````

## 第 7 步：DraftControls.commitDiscard（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 107 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=3385:5666 -->
````go
	lineage, err := NewEntityID()
	if err != nil {
		return err
	}
	decoded, err := codec.Decode(prepared.Candidate.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := decoded.(map[string]any)
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(scope.Manifest), domain.TextID(version)
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertDiscardLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, scope.Manifest, version, task.Operation, prepared.Preview, raw); err != nil {
		return err
	}
	epoch := scope.Workspace.Epoch + 1
	room, err := collaborationRoom(ctx, tx, "BRANCH", scope.Workspace.Workspace, scope.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch, content.LineageRef, content.ProvenanceJsonUtf8 = epoch, domain.TextID(lineage), raw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := discardDraftInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(scope.Manifest), "base_version_id": domain.TextID(version), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
		return err
	}
	// MAIN正式指针保持原值，意见和resumable也保持；安装责任与成功事实在同一事务登记。
````

## 第 8 步：DraftControls.commitDiscard（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 152 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 8 bytes=5666:6003 -->
````go
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompleteDiscardDraft(ctx, tx, task, "SUCCEEDED", result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_compute.go：保留原实现，按业务合并文件。

````

## 第 9 步：deriveDiscardContent

插入位置：第 8 步代码之后，紧接追加；完成稿约第 163 行。

派生规则与Node相同且可逐字节核验：只重绑本次操作来源，不采样、不改原预览、不改旧接纳来源。

本步声明：`func deriveDiscardContent(value map[string]any, old, operation, suffix string) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>encodedProvenance, ok := value&#91;&quot;provenance_json_utf8&quot;&#93;.(string)</code>；<code>value&#91;&quot;operation_id&quot;&#93;, value&#91;&quot;capture_id&quot;&#93;, value&#91;&quot;provenance_json_utf8&quot;&#93; = operation, operation+suffix, base64.StdEncoding.EncodeToString(encoded)</code>；<code>json.Marshal(value)</code> |
| <code>old</code> | <code>string</code> | 在此声明中的实际使用：<code>source&#91;&quot;origin_capture_ref&quot;&#93; == old</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>provenance&#91;&quot;source_operation_id&quot;&#93; = operation</code>；<code>source&#91;&quot;origin_capture_ref&quot;&#93; = operation</code>；<code>value&#91;&quot;operation_id&quot;&#93;, value&#91;&quot;capture_id&quot;&#93;, value&#91;&quot;provenance_json_utf8&quot;&#93; = operation, operation+suffix, base64.StdEncoding.EncodeToString(encoded)</code>；<code>operation+suffix</code> |
| <code>suffix</code> | <code>string</code> | 在此声明中的实际使用：<code>operation+suffix</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 由 <code>decodePullFixedContent(contentRaw)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>decodePullFixedContent(contentRaw)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>invalid</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>source&#91;&quot;origin_capture_ref&quot;&#93; == old</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return decodePullFixedContent(contentRaw)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6003:7386 -->
````go
// 派生规则与Node相同且可逐字节核验：只重绑本次操作来源，不采样、不改原预览、不改旧接纳来源。
func deriveDiscardContent(value map[string]any, old, operation, suffix string) (*pb.FixedContent, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	encodedProvenance, ok := value["provenance_json_utf8"].(string)
	if !ok {
		return nil, invalid
	}
	raw, err := base64.StdEncoding.DecodeString(encodedProvenance)
	if err != nil {
		return nil, err
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	provenance, ok := decoded.(map[string]any)
	if !ok {
		return nil, invalid
	}
	provenance["source_operation_id"] = operation
	sources, ok := provenance["block_sources"].([]any)
	if !ok {
		return nil, invalid
	}
	for _, item := range sources {
		source, ok := item.(map[string]any)
		if !ok {
			return nil, invalid
		}
		if source["origin_capture_ref"] == old {
			source["origin_capture_ref"] = operation
		}
	}
	encoded, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	value["operation_id"], value["capture_id"], value["provenance_json_utf8"] = operation, operation+suffix, base64.StdEncoding.EncodeToString(encoded)
	contentRaw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return decodePullFixedContent(contentRaw)
}

````

## 第 10 步：DraftControls.expectedDiscard

插入位置：第 9 步代码之后，紧接追加；完成稿约第 208 行。

本步声明：`func (s *DraftControls) expectedDiscard(ctx context.Context, fixed discardDraftContext) (map[string]any, draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_PREVIEW_FIXED&quot;, fixed.Intent.Preview)</code> |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.PreviewDigest</code>；<code>fixed.Context</code>；<code>fixed.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>expected</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>invalid</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_FIXED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_CUT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, expected, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, expected, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(digest, fixed.PreviewDigest)</code>。条件成立时执行 <code>return nil, expected, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, expected, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(hash&#91;:&#93;, digest)</code>。条件成立时执行 <code>return nil, expected, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, expected, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, expected, invalid</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=7386:9289 -->
````go
func (s *DraftControls) expectedDiscard(ctx context.Context, fixed discardDraftContext) (map[string]any, draftPrepared, error) {
	var expected draftPrepared
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_PREVIEW_FIXED", fixed.Intent.Preview)
	if err != nil {
		return nil, expected, err
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		return nil, expected, err
	}
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	value, ok := decoded.(map[string]any)
	if !ok {
		return nil, expected, invalid
	}
	digestText, ok := value["input_digest"].(string)
	if !ok {
		return nil, expected, invalid
	}
	digest, err := base64.StdEncoding.DecodeString(digestText)
	if err != nil || !bytes.Equal(digest, fixed.PreviewDigest) {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	delete(value, "input_digest")
	original, err := codec.Canonical(value)
	if err != nil {
		return nil, expected, err
	}
	hash := sha256.Sum256(original)
	if !bytes.Equal(hash[:], digest) {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return nil, expected, err
	}
	value["preview_binding"] = value["binding"]
	value["binding"] = map[string]any{"context": wire, "expected_branch_id": domain.TextID(fixed.Scope.Workspace.Workspace), "expected_structure_id": domain.TextID(fixed.Scope.Structure), "expected_lineage_id": domain.TextID(fixed.Scope.Workspace.Lineage)}
	value["preview_input_digest"] = base64.StdEncoding.EncodeToString(digest)
	value["expected_round_id"] = domain.TextID(fixed.Intent.Round)
	for key, suffix := range map[string]string{"draft": "", "main": ":main", "candidate": ":candidate"} {
		part, ok := value[key].(map[string]any)
		if !ok {
			return nil, expected, invalid
		}
````

## 第 11 步：DraftControls.expectedDiscard（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 253 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, expected, err</code>。当前函数在这里结束。
- 检查 <code>key == &quot;main&quot;</code>。
- 检查 <code>key == &quot;candidate&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(cutRaw, &amp;expected.Cut) != nil</code>。条件成立时执行 <code>return nil, expected, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, expected, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=9289:10147 -->
````go
		content, err := deriveDiscardContent(part, fixed.Intent.Preview, fixed.Intent.Operation, suffix)
		if err != nil {
			return nil, expected, err
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); err != nil {
			return nil, expected, err
		}
		if key == "main" {
			expected.Main = content
		}
		if key == "candidate" {
			expected.Candidate = content
		}
	}
	encoded, err := codec.Canonical(value)
	if err != nil {
		return nil, expected, err
	}
	derived := sha256.Sum256(encoded)
	expected.Digest = derived[:]
	expected.Preview = fixed.Intent.Preview
	cutRaw, _ := json.Marshal(value["draft_cut"])
	if json.Unmarshal(cutRaw, &expected.Cut) != nil {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_CUT_UNKNOWN")
	}
	return value, expected, nil
}

````

## 第 12 步：DraftControls.prepareDiscard

插入位置：第 11 步代码之后，紧接追加；完成稿约第 281 行。

本步声明：`func (s *DraftControls) prepareDiscard(ctx context.Context, fixed discardDraftContext) (draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.expectedDiscard(ctx, fixed)</code>；<code>s.client.Prepare(ctx, map&#91;string&#93;any{&quot;context&quot;: wire, &quot;expected_lineage_id&quot;: domain.TextID(fixed.Scope.Workspace.Lineage), &quot;expected_round_id&quot;: domain.TextID(fixed.Intent.Round), &quot; …</code>（长表达式，按本步完整代码继续） |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>s.expectedDiscard(ctx, fixed)</code>；<code>fixed.Context</code>；<code>fixed.Scope</code>；<code>fixed.Intent</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>expected</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, result.Code)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARED_BINDING_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;parts) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;state&quot;&#93;, &amp;result.State) != nil</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.State == &quot;CHANGED&quot;</code>。
- 检查 <code>json.Unmarshal(parts&#91;&quot;code&quot;&#93;, &amp;result.Code) != nil &#124;&#124; result.Code == &quot;&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, domain.Violation(domain.StateConflict, result.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>result.State != &quot;PREPARED&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(parts&#91;&quot;preview_cut_ref&quot;&#93;, &amp;result.Preview) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;input_digest&quot;&#93;, &amp;result.Digest) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;draft_cut&quot;&#93;, &amp;result.Cut)  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(observed, wanted)</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=10147:12443 -->
````go
func (s *DraftControls) prepareDiscard(ctx context.Context, fixed discardDraftContext) (draftPrepared, error) {
	_, expected, err := s.expectedDiscard(ctx, fixed)
	if err != nil {
		return expected, err
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err := s.client.Prepare(ctx, map[string]any{"context": wire, "expected_lineage_id": domain.TextID(fixed.Scope.Workspace.Lineage), "expected_round_id": domain.TextID(fixed.Intent.Round), "expected_attempt_id": fixed.Intent.Preview, "expected_input_digest": fixed.PreviewDigest})
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	var result draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &result.State) != nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
	}
	if result.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &result.Code) != nil || result.Code == "" {
			return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
		}
		return result, domain.Violation(domain.StateConflict, result.Code)
	}
	if result.State != "PREPARED" {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
	}
	result.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return result, err
	}
	result.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return result, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &result.Preview) != nil || json.Unmarshal(parts["input_digest"], &result.Digest) != nil || json.Unmarshal(parts["draft_cut"], &result.Cut) != nil || result.Preview != expected.Preview || !bytes.Equal(result.Digest, expected.Digest) || !proto.Equal(result.Candidate, expected.Candidate) || !proto.Equal(result.Main, expected.Main) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARED_BINDING_UNKNOWN")
	}
	observed, _ := codec.Canonical(result.Cut)
	wanted, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(observed, wanted) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARED_BINDING_UNKNOWN")
	}
	return result, nil
}

// discard_draft_install.go：保留原实现，按业务合并文件。

````

## 第 13 步：DraftControls.DiscardCommittedSource

插入位置：第 12 步代码之后，紧接追加；完成稿约第 329 行。

本步声明：`func (s *DraftControls) DiscardCommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DiscardOperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;DISCARD_DRAFT_COMMITTED&quot;, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.DiscardOperationResult(ctx, query)</code>；<code>query.OperationId</code>；<code>validatePullCommittedBinding(install.draftInstall, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED &amp;&amp; authority.State != pb.OperationResult_REJECTED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;install) != nil &#124;&#124; validatePullCommittedBinding(install.draftInstall, query) != nil &#124;&#124; install.Fixed.Intent.Operation != query.OperationId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=12443:13828 -->
````go
func (s *DraftControls) DiscardCommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.DiscardOperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_DRAFT_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install discardDraftInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest), "expected_round_id": domain.TextID(install.Fixed.Intent.Round)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

````

## 第 14 步：DraftControls.reconcileDiscard

插入位置：第 13 步代码之后，紧接追加；完成稿约第 356 行。

本步声明：`func (s *DraftControls) reconcileDiscard(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DiscardOperationResult(ctx, query)</code>；<code>s.client.Apply(ctx, &amp;pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_ACK_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;install) != nil &#124;&#124; install.Context == nil &#124;&#124; install.Fixed.Intent.Operation != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply == nil &#124;&#124; reply.OperationId != task.Operation &#124;&#124; reply.State != authority.State &#124;&#124; !bytes.Equal(reply.ResultDigest, authority.ResultDigest) &#124;&#124; !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(reply.CurrentStateJsonUtf8, &amp;current) != nil &#124;&#124; current.Sync != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED &amp;&amp; (current.Room != domain.TextID(install.NewRoom) &#124;&#124; current.Epoch != domain.TextID(install.Epoch) &#124;&#124; current.Lineage != domain.Tex …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=13828:16151 -->
````go
func (s *DraftControls) reconcileDiscard(ctx context.Context, task domain.SubmissionTask) error {
	var install discardDraftInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.DiscardOperationResult(ctx, query)
	if err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, query.Target.DocumentId, true); err != nil {
		return err
	}
	owner, err := repository.LockedBranchControl(ctx, tx, install.Branch)
	if err != nil {
		return err
	}
````

## 第 15 步：DraftControls.reconcileDiscard（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 401 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>guard != install.Fixed.Review.Guard</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ARCHIVE_FACT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>install.Fixed.Review.Pointer.ID != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pointer != install.Fixed.Review.Pointer</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ARCHIVE_FACT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 15 bytes=16151:17869 -->
````go
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if authority.State == pb.OperationResult_SUCCEEDED {
		guard, err := repository.LockedReviewGuard(ctx, tx, install.Fixed.Scope.Unit)
		if err != nil {
			return err
		}
		if guard != install.Fixed.Review.Guard {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
		}
		if install.Fixed.Review.Pointer.ID != 0 {
			pointer, err := repository.LockedReviewPointer(ctx, tx, install.Fixed.Review.Pointer.ID)
			if err != nil {
				return err
			}
			if pointer != install.Fixed.Review.Pointer {
				return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
			}
		}
		// 此时只核验成功时固定事实，不恢复旧用户证明；注销不能取消已经成功的系统安装责任。
		if err = repository.ArchiveDiscardedReview(ctx, tx, task.Operation, install.Fixed.Actor, install.Fixed.Review.Pointer, install.Fixed.Intent.Changes); err != nil {
			return err
		}
		if err = repository.ActivateDiscardedBranch(ctx, tx, install.Branch, install.Epoch); err != nil {
			return err
		}
	}
	if err = repository.SetBranchControl(ctx, tx, install.Branch, ""); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_readiness.go：保留原实现，按业务合并文件。

````

## 第 16 步：validatePendingDiscardDraftBinding

插入位置：第 15 步代码之后，紧接追加；完成稿约第 446 行。

本步声明：`func validatePendingDiscardDraftBinding(fact repository.PendingDiscardDraftBinding) (discardDraftContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingDiscardDraftBinding</code> | 使用 <code>repository.PendingDiscardDraftBinding</code> 类型；其定义处的职责：登记丢弃草稿控制、派生lineage、安装任务与审核归档动作。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Actor</code>；<code>fact.Operation</code>；<code>fact.Owner</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardDraftContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;fixed) != nil</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; fixed.Actor != fact.Actor &#124;&#124; ctx.ActorId != fact.Actor &#124;&#124; ctx.OperationId != fact.Operation &#124;&#124; fixed.Intent.Operation != fact.Operation &#124;&#124; fact.O …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>scope.Document == 0 &#124;&#124; scope.Unit == 0 &#124;&#124; scope.Room == 0 &#124;&#124; scope.Manifest == 0 &#124;&#124; scope.Structure == 0 &#124;&#124; scope.Style == 0 &#124;&#124; scope.Control == 0 &#124;&#124; scope.Workspace.Workspace == 0 …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parsed.Operation != fact.Operation &#124;&#124; parsed.Preview != fixed.Intent.Preview &#124;&#124; parsed.Branch != scope.Workspace.Workspace &#124;&#124; parsed.Structure != scope.Structure &#124;&#124; parsed.Round != fixed.Intent.Round</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; json.Unmarshal(fact.Target, &amp;httpTarget) != nil &#124;&#124; httpTarget.Path != path &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Access.Document.Id != domain.TextID(scope.Document) &#124;&#124; fixed.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) &#124;&#124; fixed.Access.Document.CurrentStructureId != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != 0</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != fixed.Actor &#124;&#124; proof.OperationId != fact.Operation &#124;&#124; proof.Purpose != &quot;DISCARD_DRAFT&quot; &#124;&#124; !proto.Equal(proof.Target, expected) &#124;&#124; len(proof.Session …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=17869:21158 -->
````go
func validatePendingDiscardDraftBinding(fact repository.PendingDiscardDraftBinding) (discardDraftContext, error) {
	var fixed discardDraftContext
	invalid := errors.New("DISCARD_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &fixed) != nil {
		return fixed, invalid
	}
	ctx, scope := fixed.Context, fixed.Scope
	if ctx == nil || ctx.Target == nil || fixed.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || fixed.Intent.Operation != fact.Operation || fact.Owner != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Manage || len(fixed.PreviewDigest) != 32 {
		return fixed, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "DISCARD_DRAFT"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || scope.Workspace.Lineage == 0 || !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" {
		return fixed, invalid
	}
	parsed, err := domain.ParseDiscardDraft(fixed.Intent.Input)
	if err != nil || parsed.Operation != fact.Operation || parsed.Preview != fixed.Intent.Preview || parsed.Branch != scope.Workspace.Workspace || parsed.Structure != scope.Structure || parsed.Round != fixed.Intent.Round {
		return fixed, invalid
	}
	a, _ := json.Marshal(parsed.Changes)
	b, _ := json.Marshal(fixed.Intent.Changes)
	if !bytes.Equal(a, b) {
		return fixed, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-draft"
	var httpTarget struct {
		Path string `json:"path"`
	}
	fingerprint, err := domain.OperationFingerprint(path, fixed.Intent.Input)
	if err != nil || json.Unmarshal(fact.Target, &httpTarget) != nil || httpTarget.Path != path || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return fixed, invalid
	}
	if fixed.Access.Document.Id != domain.TextID(scope.Document) || fixed.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) || fixed.Access.Document.CurrentStructureId != domain.TextID(scope.Structure) || fixed.Access.Document.CurrentStyleId != domain.TextID(scope.Style) || fixed.Access.Document.ControlRevision != domain.TextID(scope.Control) || !equalPullSources(ctx.Sources, fixed.Access.Sources) {
		return fixed, invalid
	}
	available := map[string]uint64{}
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return fixed, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fact.Operation || proof.Purpose != "DISCARD_DRAFT" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) {
			return fixed, invalid
		}
````

## 第 17 步：validatePendingDiscardDraftBinding（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 491 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(proof.SessionRef, fixed.Access.Proofs&#91;0&#93;.SessionRef)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; available&#91;source.ScopeKey&#93; != source.Epoch &#124;&#124; source.Epoch == 0</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Access.Proofs&#91;0&#93;.AdmissionId != ctx.IdentityAdmissionRef &#124;&#124; fixed.Preview.Context == nil &#124;&#124; len(fixed.Preview.Access.Proofs) == 0 &#124;&#124; fixed.Preview.Access.Proofs&#91;0&#93; == nil &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>public.PreviewCutRef != fixed.Intent.Preview &#124;&#124; public.DocumentId != domain.TextID(scope.Document) &#124;&#124; public.UnitId != domain.TextID(scope.Unit) &#124;&#124; public.BranchId != domain.TextID …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;DISCARD_DRAFT_RECOVER&quot;</code>。
- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; !bytes.Equal(fact.Original, fact.Payload)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;DISCARD_DRAFT_RECONCILE&quot;</code>。
- 检查 <code>(fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;) &#124;&#124; !bytes.Equal(fact.Payload, fact.Committed)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;install) != nil &#124;&#124; install.Context == nil &#124;&#124; install.Branch != scope.Workspace.Workspace &#124;&#124; install.OldRoom != scope.Room &#124;&#124; !proto.Equal(install.Context, ctx)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(raw, originalFields)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot; &amp;&amp; (install.NewRoom == 0 &#124;&#124; install.NewRoom == scope.Room &#124;&#124; install.Epoch != scope.Workspace.Epoch+1 &#124;&#124; install.Lineage == 0 &#124;&#124; install.Content == nil &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=21158:24133 -->
````go
		if !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return fixed, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || available[source.ScopeKey] != source.Epoch || source.Epoch == 0 {
				return fixed, invalid
			}
		}
	}
	if fixed.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef || fixed.Preview.Context == nil || len(fixed.Preview.Access.Proofs) == 0 || fixed.Preview.Access.Proofs[0] == nil || !bytes.Equal(fixed.Access.Proofs[0].SessionRef, fixed.Preview.Access.Proofs[0].SessionRef) || fixed.Preview.Scope != scope || fixed.Preview.Actor != fixed.Actor || fixed.Preview.Intent.Operation != fixed.Intent.Preview {
		return fixed, invalid
	}
	previewRaw, _ := json.Marshal(fixed.Preview)
	previewTarget, _ := json.Marshal(map[string]string{"path": "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-previews"})
	if _, err = validateOriginalDiscardPreviewBinding(repository.PendingDiscardPreviewBinding{Operation: fixed.Intent.Preview, Actor: fixed.Actor, Original: previewRaw, Target: previewTarget, Fingerprint: fixed.Preview.Context.RequestFingerprint}); err != nil {
		return fixed, invalid
	}
	public := fixed.PreviewResult
	if public.PreviewCutRef != fixed.Intent.Preview || public.DocumentId != domain.TextID(scope.Document) || public.UnitId != domain.TextID(scope.Unit) || public.BranchId != domain.TextID(scope.Workspace.Workspace) || public.WorkspaceEpoch != domain.TextID(scope.Workspace.Epoch) || public.LineageId != domain.TextID(scope.Workspace.Lineage) || public.MainManifestId != domain.TextID(scope.Manifest) || public.StructureId != domain.TextID(scope.Structure) {
		return fixed, invalid
	}
	if _, err = time.Parse(time.RFC3339Nano, public.ExpiresAt); err != nil {
		return fixed, invalid
	}
	if fact.Kind == "DISCARD_DRAFT_RECOVER" {
		if fact.State != "PREPARING" || !bytes.Equal(fact.Original, fact.Payload) {
			return fixed, invalid
		}
	} else if fact.Kind == "DISCARD_DRAFT_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, invalid
		}
		var install discardDraftInstall
		if json.Unmarshal(fact.Payload, &install) != nil || install.Context == nil || install.Branch != scope.Workspace.Workspace || install.OldRoom != scope.Room || !proto.Equal(install.Context, ctx) {
			return fixed, invalid
		}
		raw, _ := json.Marshal(install.Fixed)
		originalFields, _ := json.Marshal(fixed)
		// MySQL JSON会调整字段顺序；核验完整已解析绑定，不把原字节的合法数据库编码误判为篡改。
		if !bytes.Equal(raw, originalFields) {
			return fixed, invalid
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == scope.Room || install.Epoch != scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Content == nil || install.Content.OperationId != fact.Operation) {
			return fixed, invalid
		}
	} else {
````

## 第 18 步：validatePendingDiscardDraftBinding（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 537 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fixed, invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=24133:24182 -->
````go
		return fixed, invalid
	}
	return fixed, nil
}

````

## 第 19 步：validateDiscardAdmission

插入位置：第 18 步代码之后，紧接追加；完成稿约第 542 行。

本步声明：`func validateDiscardAdmission(fixed discardDraftContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code>；<code>fixed.Scope</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Scope.Document &#124;&#124; actual.Unit != fixed.Scope.Unit &#124;&#124; actual.Identity != fixed.Context.IdentityAdmissionRef &#124;&#124; actual.Operati …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, fixed.Context.Target)</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=24182:25044 -->
````go
func validateDiscardAdmission(fixed discardDraftContext, actual repository.OriginalAdmissionBinding) error {
	if fixed.Context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "DISCARD_DRAFT" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	return nil
}

````

## 第 20 步：DraftControls.readyDiscards

插入位置：第 19 步代码之后，紧接追加；完成稿约第 556 行。

本步声明：`func (s *DraftControls) readyDiscards(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingDiscardDraftBindings(ctx, s.db)</code>；<code>repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code>；<code>s.LookupPreview(ctx, fixed.Intent.Preview)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DISCARD_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>；<code>errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateDiscardAdmission(fixed, admission) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateDiscardPreviewAdmission(original, previewAdmission) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(previewSources, original.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;DISCARD_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=25044:26747 -->
````go
func (s *DraftControls) readyDiscards(ctx context.Context) error {
	facts, err := repository.PendingDiscardDraftBindings(ctx, s.db)
	if err != nil {
		return errors.New("DISCARD_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingDiscardDraftBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardAdmission(fixed, admission) != nil {
			return errors.New("DISCARD_ADMISSION_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("DISCARD_ADMISSION_UNKNOWN")
		}
		original, err := s.LookupPreview(ctx, fixed.Intent.Preview)
		if err != nil {
			return err
		}
		a, _ := json.Marshal(original)
		b, _ := json.Marshal(fixed.Preview)
		if !bytes.Equal(a, b) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		previewAdmission, err := repository.OriginalAdmission(ctx, s.db, original.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardPreviewAdmission(original, previewAdmission) != nil {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		previewSources, err := repository.AdmissionSources(ctx, s.db, original.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(previewSources, original.Access.Sources) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		if _, _, err = s.expectedDiscard(ctx, fixed); err != nil {
			return err
		}
	}
	return nil
}

// discard_draft_readings.go：保留原实现，按业务合并文件。

````

## 第 21 步：DraftControls.DiscardProgress

插入位置：第 20 步代码之后，紧接追加；完成稿约第 600 行。

本步声明：`func (s *DraftControls) DiscardProgress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupDiscard(ctx, operation)</code>；<code>s.previewResultAccess(ctx, actor, fixed.Scope.Document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.previewResultAccess(ctx, actor, fixed.Scope.Document)</code>；<code>actor.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.LookupDiscard(ctx, operation)</code>；<code>repository.DiscardDraftResult(ctx, tx, operation)</code>；<code>repository.DiscardDraftInstalled(ctx, tx, operation)</code>；<code>&quot;operation_id&quot;: operation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		fact, err := repository.Discard …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		fact, er …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;DISCARD_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;PREPARING&quot; &amp;&amp; !installed</code>。条件成立时执行 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;WAITING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + operation}, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=26747:28188 -->
````go
func (s *DraftControls) DiscardProgress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupDiscard(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.previewResultAccess(ctx, actor, fixed.Scope.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fact, err := repository.DiscardDraftResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.Actor != actor.ID {
			return nil, domain.Violation(domain.EntityNotFound, "DISCARD_NOT_FOUND")
		}
		if !fact.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
		}
		var value map[string]any
		if json.Unmarshal(fact.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
		}
		installed, err := repository.DiscardDraftInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.State != "PREPARING" && !installed {
			// 公共结果到真实安装/归档完成后才终态；私有系统原结果保持不变供可靠收尾。
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

````

## 第 22 步：DraftControls.DiscardOperationResult

插入位置：第 21 步代码之后，紧接追加；完成稿约第 639 行。

本步声明：`func (s *DraftControls) DiscardOperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupDiscard(ctx, query.OperationId)</code>；<code>repository.DiscardDraftResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DISCARD_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DISCARD_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor &#124;&#124; !bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=28188:29955 -->
````go
func (s *DraftControls) DiscardOperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_QUERY_INVALID")
	}
	fixed, err := s.LookupDiscard(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.DiscardDraftResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, State: pb.OperationResult_PROCESSING, Generation: fact.Generation}
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
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// discard_draft_recovery.go：保留原实现，按业务合并文件。

````

## 第 23 步：DraftControls.RecoverDiscardOne

插入位置：第 22 步代码之后，紧接追加；完成稿约第 681 行。

本步声明：`func (s *DraftControls) RecoverDiscardOne(parent context.Context)`。


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
- 检查 <code>bindingError != nil &#124;&#124; fact.Kind != task.Kind &#124;&#124; !bytes.Equal(fact.Payload, task.Payload)</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>task.Kind == &quot;DISCARD_DRAFT_RECONCILE&quot;</code>。
- 检查 <code>err == nil</code>。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err == nil</code>。
- 执行 <code>check.Rollback()</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 23 bytes=29955:31334 -->
````go
func (s *DraftControls) RecoverDiscardOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimDiscardDraftTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindingError := repository.ClaimedDiscardDraftBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingDiscardDraftBinding(fact)
	if bindingError != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	if err != nil {
		_ = repository.RetryMergeTask(ctx, s.db, task, "DISCARD_BINDING_UNKNOWN")
		return
	}
	if task.Kind == "DISCARD_DRAFT_RECONCILE" {
		err = s.reconcileDiscard(ctx, task)
	} else {
		proofs, original, resumeError := s.resumeDiscard(ctx, fixed)
		err = resumeError
		if err == nil {
			check, e := s.db.BeginTx(ctx, nil)
			err = e
			if err == nil {
				err = s.validateDiscard(ctx, check, fixed, proofs, original)
				_ = check.Rollback()
			}
		}
		if err == nil {
			prepared, e := s.prepareDiscard(ctx, fixed)
			err = e
			if err == nil {
				err = s.commitDiscard(ctx, task, fixed, prepared, proofs, original)
			}
		}
		var violation domain.RuleViolation
````

## 第 24 步：DraftControls.RecoverDiscardOne（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 726 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.StateConflict &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 24 bytes=31334:31743 -->
````go
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
			err = s.rejectDiscard(ctx, task, fixed, violation.Code)
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

````

## 第 25 步：DraftControls.rejectDiscard

插入位置：第 24 步代码之后，紧接追加；完成稿约第 737 行。

本步声明：`func (s *DraftControls) rejectDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Scope.Document, true)</code>；<code>repository.LockedBranchControl(ctx, tx, fixed.Scope.Workspace.Workspace)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteDiscardDraft(ctx, tx, task, &quot;REJECTED&quot;, result, install)</code> |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.Scope</code>；<code>Fixed: fixed</code>；<code>fixed.Context</code>；<code>fixed.Intent</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 25 bytes=31743:33254 -->
````go
func (s *DraftControls) rejectDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, fixed.Scope.Document, true); err != nil {
		return err
	}
	owner, err := repository.LockedBranchControl(ctx, tx, fixed.Scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	// 未完成prepare也登记原号系统收尾；不能因网络超时猜测Node从未BLOCKED。
	install, _ := json.Marshal(discardDraftInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: fixed.Intent.Preview}})
	if err = repository.CompleteDiscardDraft(ctx, tx, task, "REJECTED", result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_register.go：保留原实现，按业务合并文件。

````

## 第 26 步：DraftControls.RegisterDiscard

插入位置：第 25 步代码之后，紧接追加；完成稿约第 770 行。

本步声明：`func (s *DraftControls) RegisterDiscard(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardDraftIntent, path string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_DRAFT&quot;)</code>；<code>repository.DiscardDraftResult(ctx, s.db, intent.Operation)</code>；<code>s.LookupPreview(ctx, intent.Preview)</code>；<code>s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)</code><br>另有 14 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_DRAFT&quot;)</code>；<code>return actor, err</code>；<code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>actor.ID</code><br>另有 14 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;DISCARD_DRAFT&quot;)</code>；<code>preview.Scope.Document != document</code>；<code>DocumentId: document</code>；<code>InsertAdmission(ctx, tx, actor, access.Sources, document, &quot;DISCARD_DRAFT&quot;, admission)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>preview.Scope.Unit != unit</code>；<code>UnitId: unit</code>；<code>s.readDiscardReview(ctx, tx, unit, intent, fixed)</code>；<code>repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, unit, rawTarget, actor.SessionExpiresUnixMs)</code> |
| <code>intent</code> | <code>domain.DiscardDraftIntent</code> | 使用 <code>domain.DiscardDraftIntent</code> 类型；其定义处的职责：DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。<br>在此声明中的实际使用：<code>intent.Input</code>；<code>intent.Operation</code>；<code>intent.Preview</code>；<code>intent.Branch</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, intent.Input)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>fact.Actor != actor.ID &#124;&#124; !bytes.Equal(fact.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>preview.Actor != actor.ID &#124;&#124; preview.Scope.Document != document &#124;&#124; preview.Scope.Unit != unit &#124;&#124; preview.Scope.Workspace.Workspace != intent.Branch &#124;&#124; preview.Scope.Structure != in …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。

<!-- write-step: 26 bytes=33254:35296 -->
````go
func (s *DraftControls) RegisterDiscard(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardDraftIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_DRAFT")
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
	if fact, e := repository.DiscardDraftResult(ctx, s.db, intent.Operation); e == nil {
		if fact.Actor != actor.ID || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	preview, err := s.LookupPreview(ctx, intent.Preview)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_REQUIRED")
	}
	if preview.Actor != actor.ID || preview.Scope.Document != document || preview.Scope.Unit != unit || preview.Scope.Workspace.Workspace != intent.Branch || preview.Scope.Structure != intent.Structure || len(access.Proofs) == 0 || len(preview.Access.Proofs) == 0 || !bytes.Equal(access.Proofs[0].SessionRef, preview.Access.Proofs[0].SessionRef) {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_BINDING_CHANGED")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: preview.Scope.Workspace.Epoch, Purpose: "DISCARD_DRAFT"}
	var bound []*pb.IdentityAdmission
	bound, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = bound
	actor.Proof = bound[0]
	actor.Sources = access.Sources
	originalProofs, err := s.resumePreview(ctx, preview)
	if err != nil {
		return actor, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
````

## 第 27 步：DraftControls.RegisterDiscard（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 815 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>prior.ActorID != actor.ID &#124;&#124; prior.Kind != &quot;DISCARD_DRAFT&quot; &#124;&#124; !bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &#124;&#124; json.Unmarshal(fact.Result, &amp;public) != nil &#124;&#124; public.PreviewCutRef != intent.Preview</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !expires.After(time.Now())</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;snapshot) != nil &#124;&#124; len(snapshot.Digest) != 32</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=35296:37192 -->
````go
	if err = repository.LockOperationGate(ctx, tx, intent.Operation); err != nil {
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if prior, e := repository.LockedOperation(ctx, tx, intent.Operation); e == nil {
		if prior.ActorID != actor.ID || prior.Kind != "DISCARD_DRAFT" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	if err = s.validatePreview(ctx, tx, preview, originalProofs); err != nil {
		return actor, err
	}
	fact, err := repository.DiscardPreviewResult(ctx, tx, intent.Preview)
	if err != nil {
		return actor, err
	}
	var public dto.DiscardPreview
	if fact.State != "SUCCEEDED" || json.Unmarshal(fact.Result, &public) != nil || public.PreviewCutRef != intent.Preview {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_REQUIRED")
	}
	expires, err := time.Parse(time.RFC3339Nano, public.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	raw, err := repository.MergeArtifact(ctx, tx, "DISCARD_PREVIEW_FIXED", intent.Preview)
	if err != nil {
		return actor, err
	}
	var snapshot struct {
		Digest []byte `json:"input_digest"`
	}
	if json.Unmarshal(raw, &snapshot) != nil || len(snapshot.Digest) != 32 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	access.Document = current
	fixed := discardDraftContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Scope: preview.Scope, Preview: preview, PreviewResult: public, PreviewDigest: snapshot.Digest}
````

## 第 28 步：DraftControls.RegisterDiscard（接着实现）

插入位置：第 27 步代码之后，紧接追加；完成稿约第 860 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 28 bytes=37192:38491 -->
````go
	fixed.Review, err = s.readDiscardReview(ctx, tx, unit, intent, fixed)
	if err != nil {
		return actor, err
	}
	command := OperationCommand{ID: intent.Operation, Kind: "DISCARD_DRAFT", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "DISCARD_DRAFT", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	encoded, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterDiscardDraft(ctx, tx, intent.Branch, intent.Operation, encoded); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// discard_draft_review.go：保留原实现，按业务合并文件。

````

## 第 29 步：DraftControls.readDiscardReview

插入位置：第 28 步代码之后，紧接追加；完成稿约第 889 行。

本步声明：`func (s *DraftControls) readDiscardReview(ctx context.Context, tx *sql.Tx, unit uint64, intent domain.DiscardDraftIntent, preview discardDraftContext) (discardPreviewReview, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;DISCARD_PREVIEW_REVIEW&quot;, intent.Preview)</code>；<code>repository.LockedReviewGuard(ctx, tx, unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, result.Guard.Resumable)</code>；<code>repository.LockedActiveMerge(ctx, tx, p.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;DISCARD_PREVIEW_REVIEW&quot;, intent.Preview)</code>；<code>repository.LockedReviewGuard(ctx, tx, unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, result.Guard.Resumable)</code>；<code>repository.LockedActiveMerge(ctx, tx, p.ID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit)</code>；<code>p.Unit != unit</code>；<code>repository.DiscardPreviewBlockingChanges(ctx, tx, unit)</code> |
| <code>intent</code> | <code>domain.DiscardDraftIntent</code> | 使用 <code>domain.DiscardDraftIntent</code> 类型；其定义处的职责：DTO保留冻结接口；领域意图只负责规范ID、理由和集合唯一性，实际意见归属由事务核验。<br>在此声明中的实际使用：<code>intent.Preview</code>；<code>intent.Round</code>；<code>intent.Changes</code>；<code>intent.Branch</code> |
| <code>preview</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>preview.PreviewResult</code>；<code>preview.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>discardPreviewReview</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_REVIEW_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;SUBMISSION_PENDING&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;REVIEW_WITHDRAW_REQUIRED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;original) != nil</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_REVIEW_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>result.Guard.Submission != &quot;&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;SUBMISSION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Guard.Ongoing != 0</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;REVIEW_WITHDRAW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Guard.Resumable != original.Pointer.ID</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;DISCARD_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Guard.Resumable == 0</code>。
- 检查 <code>intent.Round != 0 &#124;&#124; len(intent.Changes) != 0 &#124;&#124; len(preview.PreviewResult.BlockingChanges) != 0</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;DISCARD_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>p.Archived &#124;&#124; p.Status != &quot;WITHDRAWN&quot; &#124;&#124; p.Round != intent.Round &#124;&#124; p.Round != original.Pointer.Round &#124;&#124; p.Branch != intent.Branch &#124;&#124; p.Unit != unit &#124;&#124; p.Document != preview.Scope.Document</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;DISCARD_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>active != &quot;&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;MERGE_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 29 bytes=38491:40435 -->
````go
func (s *DraftControls) readDiscardReview(ctx context.Context, tx *sql.Tx, unit uint64, intent domain.DiscardDraftIntent, preview discardDraftContext) (discardPreviewReview, error) {
	var result discardPreviewReview
	raw, err := repository.MergeArtifact(ctx, tx, "DISCARD_PREVIEW_REVIEW", intent.Preview)
	if err != nil {
		return result, err
	}
	var original discardPreviewReview
	if json.Unmarshal(raw, &original) != nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_REVIEW_UNKNOWN")
	}
	result.Guard, err = repository.LockedReviewGuard(ctx, tx, unit)
	if err != nil {
		return result, err
	}
	if result.Guard.Submission != "" {
		return result, domain.Violation(domain.StateConflict, "SUBMISSION_PENDING")
	}
	if result.Guard.Ongoing != 0 {
		return result, domain.Violation(domain.StateConflict, "REVIEW_WITHDRAW_REQUIRED")
	}
	if result.Guard.Resumable != original.Pointer.ID {
		return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	if result.Guard.Resumable == 0 {
		if intent.Round != 0 || len(intent.Changes) != 0 || len(preview.PreviewResult.BlockingChanges) != 0 {
			return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
		}
		return result, nil
	}
	result.Pointer, err = repository.LockedReviewPointer(ctx, tx, result.Guard.Resumable)
	if err != nil {
		return result, err
	}
	p := result.Pointer
	if p.Archived || p.Status != "WITHDRAWN" || p.Round != intent.Round || p.Round != original.Pointer.Round || p.Branch != intent.Branch || p.Unit != unit || p.Document != preview.Scope.Document {
		return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	active, err := repository.LockedActiveMerge(ctx, tx, p.ID)
	if err != nil {
		return result, err
	}
	if active != "" {
		return result, domain.Violation(domain.StateConflict, "MERGE_PENDING")
	}
	blocks, err := repository.DiscardPreviewBlockingChanges(ctx, tx, unit)
````

## 第 30 步：DraftControls.readDiscardReview（接着实现）

插入位置：第 29 步代码之后，紧接追加；完成稿约第 934 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(blocks) != len(expected) &#124;&#124; len(changes) != len(expected)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;DISCARD_CHANGES_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>expected&#91;item&#91;&quot;id&quot;&#93;&#93; != item&#91;&quot;revision&quot;&#93; &#124;&#124; changes&#91;item&#91;&quot;id&quot;&#93;&#93; != item&#91;&quot;revision&quot;&#93;</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;DISCARD_CHANGES_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=40435:41139 -->
````go
	if err != nil {
		return result, err
	}
	expected := map[string]string{}
	for _, item := range preview.PreviewResult.BlockingChanges {
		expected[item.Id] = item.Revision
	}
	changes := map[string]string{}
	for _, item := range intent.Changes {
		changes[domain.TextID(item.ID)] = domain.TextID(item.Revision)
	}
	if len(blocks) != len(expected) || len(changes) != len(expected) {
		return result, domain.Violation(domain.StateConflict, "DISCARD_CHANGES_CHANGED")
	}
	for _, item := range blocks {
		if expected[item["id"]] != item["revision"] || changes[item["id"]] != item["revision"] {
			return result, domain.Violation(domain.StateConflict, "DISCARD_CHANGES_CHANGED")
		}
	}
	return result, nil
}

````

## 第 31 步：DraftControls.validateDiscardReview

插入位置：第 30 步代码之后，紧接追加；完成稿约第 956 行。

本步声明：`func (s *DraftControls) validateDiscardReview(ctx context.Context, tx *sql.Tx, fixed discardDraftContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.readDiscardReview(ctx, tx, fixed.Scope.Unit, fixed.Intent, fixed)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.readDiscardReview(ctx, tx, fixed.Scope.Unit, fixed.Intent, fixed)</code> |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>s.readDiscardReview(ctx, tx, fixed.Scope.Unit, fixed.Intent, fixed)</code>；<code>fixed.Review</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DISCARD_REVIEW_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_EXPIRY_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_EXPIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current != fixed.Review</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DISCARD_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_PREVIEW_EXPIRY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!expires.After(time.Now())</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DISCARD_PREVIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=41139:41865 -->
````go
func (s *DraftControls) validateDiscardReview(ctx context.Context, tx *sql.Tx, fixed discardDraftContext) error {
	current, err := s.readDiscardReview(ctx, tx, fixed.Scope.Unit, fixed.Intent, fixed)
	if err != nil {
		return err
	}
	if current != fixed.Review {
		return domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	expires, err := time.Parse(time.RFC3339Nano, fixed.PreviewResult.ExpiresAt)
	if err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_EXPIRY_UNKNOWN")
	}
	if !expires.After(time.Now()) {
		return domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	return nil
}

// discard_draft_source.go：保留原实现，按业务合并文件。

````

## 第 32 步：DraftControls.resumeDiscard

插入位置：第 31 步代码之后，紧接追加；完成稿约第 976 行。

本步声明：`func (s *DraftControls) resumeDiscard(ctx context.Context, fixed discardDraftContext) ([]*pb.IdentityAdmission, []*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code>；<code>s.resumePreview(ctx, fixed.Preview)</code> |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code>；<code>fixed.Preview</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, original, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=41865:42483 -->
````go
func (s *DraftControls) resumeDiscard(ctx context.Context, fixed discardDraftContext) ([]*pb.IdentityAdmission, []*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, nil, err
		}
		proofs = append(proofs, proof)
	}
	original, err := s.resumePreview(ctx, fixed.Preview)
	return proofs, original, err
}

````

## 第 33 步：DraftControls.validateDiscard

插入位置：第 32 步代码之后，紧接追加；完成稿约第 992 行。

本步声明：`func (s *DraftControls) validateDiscard(ctx context.Context, tx *sql.Tx, fixed discardDraftContext, proofs, original []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>s.validatePreviewForControl(ctx, tx, fixed.Preview, original, fixed.Intent.Operation)</code>；<code>repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>s.validatePreviewForControl(ctx, tx, fixed.Preview, original, fixed.Intent.Operation)</code>；<code>repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>discardDraftContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Preview</code>；<code>fixed.Intent</code>；<code>fixed.Context</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |
| <code>original</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validatePreviewForControl(ctx, tx, fixed.Preview, original, fixed.Intent.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DISCARD_ADMISSION_CLOSED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Manage</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(fixed.Access.Proofs) == 0 &#124;&#124; actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Scope.Document &#124;&#124; actual.Unit != fixed.Scope.Unit &#124;&#124; actual.Identity != fixed.Context.Ident …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, fixed.Context.Target)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;DISCARD_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DISCARD_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.validateDiscardReview(ctx, tx, fixed)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=42483:44675 -->
````go
func (s *DraftControls) validateDiscard(ctx context.Context, tx *sql.Tx, fixed discardDraftContext, proofs, original []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	if _, err := ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	if err := s.validatePreviewForControl(ctx, tx, fixed.Preview, original, fixed.Intent.Operation); err != nil {
		return err
	}
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	if len(fixed.Access.Proofs) == 0 || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "DISCARD_DRAFT" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "DISCARD_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs); err != nil {
		return err
	}
	return s.validateDiscardReview(ctx, tx, fixed)
}

````

## 第 34 步：DraftControls.DiscardSource

插入位置：第 33 步代码之后，紧接追加；完成稿约第 1036 行。

Node仅得到已登记确认的原预览引用/摘要；正文仍从Node自己的不可变原预览取，Go不让调用方传新候选。

本步声明：`func (s *DraftControls) DiscardSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.LookupDiscard(ctx, input.Context.OperationId)</code>；<code>s.resumeDiscard(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validateDiscard(ctx, tx, fixed, proofs, original)</code> |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(input, expected)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;purpose&quot;: &quot;DISCARD_DRAFT&quot;, &quot;preview_cut_ref&quot;: fixed.Intent.Preview, &quot;preview_input_digest&quot;: fixed.PreviewDigest, &quot;preview_context&quot;: previewBinding, &quot;expected_round_id&quot;: domain.TextID(fixed.Intent.Round)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DISCARD_SOURCE_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DISCARD_SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input, expected)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;any{&quot;purpose&quot;: &quot;DISCARD_DRAFT&quot;, &quot;preview_cut_ref&quot;: fixed.Intent.Preview, &quot;preview_input_digest&quot;: fixed.PreviewDigest, &quot;preview_context&quot;: previewBinding, &quot;expected …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=44675:46097 -->
````go
// Node仅得到已登记确认的原预览引用/摘要；正文仍从Node自己的不可变原预览取，Go不让调用方传新候选。
func (s *DraftControls) DiscardSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_SOURCE_INVALID")
	}
	fixed, err := s.LookupDiscard(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	expected := &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
	if !proto.Equal(input, expected) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, original, err := s.resumeDiscard(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.validateDiscard(ctx, tx, fixed, proofs, original); err != nil {
		return nil, err
	}
	previewBinding, err := contextWire(fixed.Preview.Context)
	if err != nil {
		return nil, err
	}
	return map[string]any{"purpose": "DISCARD_DRAFT", "preview_cut_ref": fixed.Intent.Preview, "preview_input_digest": fixed.PreviewDigest, "preview_context": previewBinding, "expected_round_id": domain.TextID(fixed.Intent.Round)}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/discard_drafts_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
