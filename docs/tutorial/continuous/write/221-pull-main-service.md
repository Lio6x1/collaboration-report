# 分步编写 `services/document/internal/logic/document/pull_main_service.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/221-pull-main-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。

拉取不能覆盖未送审可靠修改，也不能与未固定送审或正在合并跨越支线代次。

写之前先按这条执行路线理解各段的连接：

1. Prepare固定workspace/lineage和MAIN
2. compute要求PREPARED候选
3. validateInputs核cut与来源
4. commit写新lineage
5. reconcile核Node安装和当前分支后释放占位。

本文件需要守住的边界：

- CONFLICTED直接返回拉取冲突
- 固定旧审核轮次可以保留，未固定送审和活动合并必须阻止拉取。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/pull_main_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/pull_main_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/pull_main_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/pull_main_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:662 -->
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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// pull_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：PullMain.commit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func (s *PullMain) commit(ctx context.Context, task domain.SubmissionTask, fixed pullContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompletePull(ctx, tx, task, result, encoded)</code> |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>fixed.Document</code>；<code>fixed.Workspace</code>；<code>fixed.Manifest</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>draftPrepared</code> | 在此声明中的实际使用：<code>prepared.Main</code>；<code>prepared.Candidate</code>；<code>prepared.Preview</code>；<code>prepared.Digest</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_ACCEPTED_CAPTURES_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_PROVENANCE_INVALID&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(accepted, expected)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_ACCEPTED_CAPTURES_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=662:2295 -->
````go
func (s *PullMain) commit(ctx context.Context, task domain.SubmissionTask, fixed pullContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
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
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, []*pb.FixedContent{prepared.Main, prepared.Candidate}, "PULL_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	_, _, prior, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return err
	}
	original, err := codec.Decode(prior)
	if err != nil {
		return err
	}
	value, err := codec.Decode(prepared.Candidate.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := value.(map[string]any)
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return err
	}
	expected, err := codec.Canonical(original.(map[string]any)["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, expected) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_ACCEPTED_CAPTURES_CHANGED")
	}
	// 可靠MAIN采样成为这条支线的固定基线；不切换全篇正式manifest，也不把未拉入的MAIN新增解释成删除。
````

## 第 3 步：PullMain.commit（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 71 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_PROVENANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2295:4568 -->
````go
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	version, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
	if err != nil {
		return err
	}
	lineage, err := NewEntityID()
	if err != nil {
		return err
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(fixed.Manifest), domain.TextID(version)
	provenance["source_operation_id"] = task.Operation
	if err = s.validator.Provenance(provenance); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_PROVENANCE_INVALID")
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertPullLineage(ctx, tx, lineage, fixed.Workspace.Workspace, fixed.Document, fixed.Unit, fixed.Workspace.Lineage, fixed.Manifest, version, task.Operation, prepared.Preview, raw); err != nil {
		return err
	}
	epoch := fixed.Workspace.Epoch + 1
	room, err := collaborationRoom(ctx, tx, "BRANCH", fixed.Workspace.Workspace, fixed.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch = epoch
	content.LineageRef = domain.TextID(lineage)
	content.ProvenanceJsonUtf8 = raw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := draftInstall{Context: fixed.Context, Branch: fixed.Workspace.Workspace, OldRoom: fixed.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(fixed.Manifest), "base_version_id": domain.TextID(version), "body": body, "provenance": provenance}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
````

## 第 4 步：PullMain.commit（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 116 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 4 bytes=4568:4891 -->
````go
		return err
	}
	if err = repository.AdvancePulledBranch(ctx, tx, fixed.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompletePull(ctx, tx, task, result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_compute.go：保留原实现，按业务合并文件。

````

## 第 5 步：PullMain.compute

插入位置：第 4 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func (s *PullMain) compute(ctx context.Context, fixed pullContext) (*pb.DraftPreviewReply, draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.client.Preview(ctx, &amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Workspace.Workspace, ExpectedStructureId: fixed.Structure, ExpectedLineageId: fixed.Workspace.Lineage})</code>；<code>s.client.Prepare(ctx, map&#91;string&#93;any{&quot;context&quot;: wire, &quot;expected_attempt_id&quot;: preview.PreviewCutRef, &quot;expected_input_digest&quot;: preview.InputDigest})</code> |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Workspace</code>；<code>fixed.Structure</code>；<code>fixed.Manifest</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftPreviewReply</code> | 实际返回表达式：<code>nil</code>；<code>preview</code> |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>prepared</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREVIEW_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREPARE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PULL_MAIN_CONFLICT&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, prepared, err</code>。当前函数在这里结束。
- 检查 <code>preview == nil &#124;&#124; preview.State != pb.CaptureReply_FIXED &#124;&#124; preview.MainManifestId != fixed.Manifest &#124;&#124; len(preview.InputDigest) != 32 &#124;&#124; preview.PreviewCutRef != &quot;draft-preview:&quot;+fixed.Input.OperationId</code>。条件成立时执行 <code>return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREVIEW_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, prepared, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;parts) != nil</code>。条件成立时执行 <code>return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, prepared, err</code>。当前函数在这里结束。
- 检查 <code>prepared.State == &quot;CONFLICTED&quot;</code>。条件成立时执行 <code>return nil, prepared, domain.Violation(domain.StateConflict, &quot;PULL_MAIN_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State == &quot;CHANGED&quot;</code>。
- 此处通过 <code>return nil, prepared, domain.Violation(domain.StateConflict, prepared.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prepared.State != &quot;PREPARED&quot;</code>。条件成立时执行 <code>return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, prepared, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(parts&#91;&quot;preview_cut_ref&quot;&#93;, &amp;prepared.Preview) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;input_digest&quot;&#93;, &amp;prepared.Digest) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;draft_cut&quot;&#93;, &amp;prepared …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, prepared, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=4891:7344 -->
````go
func (s *PullMain) compute(ctx context.Context, fixed pullContext) (*pb.DraftPreviewReply, draftPrepared, error) {
	preview, err := s.client.Preview(ctx, &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Workspace.Workspace, ExpectedStructureId: fixed.Structure, ExpectedLineageId: fixed.Workspace.Lineage})
	var prepared draftPrepared
	if err != nil {
		return nil, prepared, err
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.MainManifestId != fixed.Manifest || len(preview.InputDigest) != 32 || preview.PreviewCutRef != "draft-preview:"+fixed.Input.OperationId {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREVIEW_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return nil, prepared, err
	}
	raw, err := s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": preview.PreviewCutRef, "expected_input_digest": preview.InputDigest})
	if err != nil {
		return nil, prepared, err
	}
	var parts map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARE_UNKNOWN")
	}
	if err = json.Unmarshal(parts["state"], &prepared.State); err != nil {
		return nil, prepared, err
	}
	if prepared.State == "CONFLICTED" {
		return nil, prepared, domain.Violation(domain.StateConflict, "PULL_MAIN_CONFLICT")
	}
	if prepared.State == "CHANGED" {
		_ = json.Unmarshal(parts["code"], &prepared.Code)
		return nil, prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State != "PREPARED" {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARE_UNKNOWN")
	}
	if prepared.Candidate, err = decodePullFixedContent(parts["candidate"]); err != nil {
		return nil, prepared, err
	}
	if prepared.Main, err = decodePullFixedContent(parts["main"]); err != nil {
		return nil, prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != preview.PreviewCutRef || !bytes.Equal(prepared.Digest, preview.InputDigest) {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARED_BINDING_UNKNOWN")
	}
	if err = s.validateInputs(fixed, preview, prepared); err != nil {
		return nil, prepared, err
	}
````

## 第 6 步：PullMain.compute（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 175 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return preview, prepared, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=7344:7378 -->
````go
	return preview, prepared, nil
}

````

## 第 7 步：decodePullFixedContent

插入位置：第 6 步代码之后，紧接追加；完成稿约第 178 行。

本步声明：`func decodePullFixedContent(raw json.RawMessage) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;fields)</code>；<code>raw, err = json.Marshal(fields)</code>；<code>protojson.Unmarshal(raw, content)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>content</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_CONTENT_ENVELOPE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>json.Unmarshal(envelope, &amp;purpose) != nil &#124;&#124; purpose != &quot;PULL_MAIN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_CONTENT_ENVELOPE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return content, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=7378:8199 -->
````go
func decodePullFixedContent(raw json.RawMessage) (*pb.FixedContent, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if envelope, exists := fields["purpose"]; exists {
		var purpose string
		if json.Unmarshal(envelope, &purpose) != nil || purpose != "PULL_MAIN" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_CONTENT_ENVELOPE_UNKNOWN")
		}
		// 只兼容首轮已经固定的这一项生产者封装，不使用DiscardUnknown，也不改写Node原候选与摘要。
		delete(fields, "purpose")
		var err error
		raw, err = json.Marshal(fields)
		if err != nil {
			return nil, err
		}
	}
	content := &pb.FixedContent{}
	if err := protojson.Unmarshal(raw, content); err != nil {
		return nil, err
	}
	return content, nil
}
````

## 第 8 步：contextWire

插入位置：第 7 步代码之后，紧接追加；完成稿约第 202 行。

本步声明：`func contextWire(context *pb.OperationContext) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>在此声明中的实际使用：<code>(protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(context)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=8199:8493 -->
````go
func contextWire(context *pb.OperationContext) (map[string]any, error) {
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(context)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	err = json.Unmarshal(raw, &value)
	return value, err
}
````

## 第 9 步：PullMain.validateInputs

插入位置：第 8 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`func (s *PullMain) validateInputs(fixed pullContext, preview *pb.DraftPreviewReply, prepared draftPrepared) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.Room</code>；<code>fixed.Workspace</code>；<code>fixed.Document</code>；<code>fixed.Unit</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>preview</code> | <code>*pb.DraftPreviewReply</code> | 在此声明中的实际使用：<code>preview.Draft</code> |
| <code>prepared</code> | <code>draftPrepared</code> | 在此声明中的实际使用：<code>prepared.Cut</code>；<code>prepared.Candidate</code>；<code>prepared.Main</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_CUT_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_CONTENT_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>prepared.Cut&#91;&quot;room_id&quot;&#93; != domain.TextID(fixed.Room) &#124;&#124; prepared.Cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(fixed.Workspace.Epoch)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>content.OperationId != fixed.Input.OperationId</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_CONTENT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>content.BranchId != fixed.Workspace.Workspace &#124;&#124; content.WorkspaceEpoch != fixed.Workspace.Epoch &#124;&#124; content.LineageRef != domain.TextID(fixed.Workspace.Lineage)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DRAFT_CONTENT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=8493:9571 -->
````go
func (s *PullMain) validateInputs(fixed pullContext, preview *pb.DraftPreviewReply, prepared draftPrepared) error {
	if prepared.Cut["room_id"] != domain.TextID(fixed.Room) || prepared.Cut["workspace_epoch"] != domain.TextID(fixed.Workspace.Epoch) {
		return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CUT_UNKNOWN")
	}
	for _, content := range []*pb.FixedContent{preview.Draft, prepared.Candidate, prepared.Main} {
		if err := validateFixedMergeContent(s.validator, content, fixed.Document, fixed.Unit, fixed.Structure); err != nil {
			return err
		}
		if content.OperationId != fixed.Input.OperationId {
			return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CONTENT_BINDING_UNKNOWN")
		}
	}
	for _, content := range []*pb.FixedContent{preview.Draft, prepared.Candidate} {
		if content.BranchId != fixed.Workspace.Workspace || content.WorkspaceEpoch != fixed.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Workspace.Lineage) {
			return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CONTENT_BINDING_UNKNOWN")
		}
	}
	return nil
}

````

## 第 10 步：PullMain.validateBeforeCompute

插入位置：第 9 步代码之后，紧接追加；完成稿约第 231 行。

本步声明：`func (s *PullMain) validateBeforeCompute(ctx context.Context, fixed pullContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code> |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.validate(ctx, tx, fixed, proofs)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return s.validate(ctx, tx, fixed, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=9571:9892 -->
````go
func (s *PullMain) validateBeforeCompute(ctx context.Context, fixed pullContext, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.validate(ctx, tx, fixed, proofs)
}

// pull_install.go：保留原实现，按业务合并文件。

````

## 第 11 步：PullMain.CommittedSource

插入位置：第 10 步代码之后，紧接追加；完成稿约第 242 行。

本步声明：`func (s *PullMain) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;PULL_MAIN_COMMITTED&quot;, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>query.OperationId</code>；<code>validatePullCommittedBinding(install, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED &amp;&amp; authority.State != pb.OperationResult_REJECTED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;install) != nil &#124;&#124; validatePullCommittedBinding(install, query) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=9892:11111 -->
````go
func (s *PullMain) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "PULL_MAIN_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install draftInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install, query) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"] = content, install.LineageContext
		value["new_room_id"], value["new_epoch"] = domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

````

## 第 12 步：validatePullCommittedBinding

插入位置：第 11 步代码之后，紧接追加；完成稿约第 270 行。

本步声明：`func validatePullCommittedBinding(install draftInstall, query *pb.OperationQuery) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>install</code> | <code>draftInstall</code> | 在此声明中的实际使用：<code>install.Context</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.OperationId</code>；<code>query.Target</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>install.Context == nil &#124;&#124; query == nil &#124;&#124; install.Context.Target == nil &#124;&#124; 		install.Context.OperationId != query.OperationId &#124;&#124; !proto.Equal(install.Context.Target, query.Target)  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=11111:11565 -->
````go
func validatePullCommittedBinding(install draftInstall, query *pb.OperationQuery) error {
	if install.Context == nil || query == nil || install.Context.Target == nil ||
		install.Context.OperationId != query.OperationId || !proto.Equal(install.Context.Target, query.Target) ||
		!bytes.Equal(install.Context.RequestFingerprint, query.RequestFingerprint) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	return nil
}
````

## 第 13 步：PullMain.reconcile

插入位置：第 12 步代码之后，紧接追加；完成稿约第 278 行。

本步声明：`func (s *PullMain) reconcile(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>s.client.Apply(ctx, &amp;pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: preview, ExpectedInputDigest: digest, Resolution: install.Content})</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompletePullInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_ACK_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;install) != nil &#124;&#124; install.Context == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply == nil &#124;&#124; reply.State != authority.State &#124;&#124; reply.OperationId != task.Operation &#124;&#124; !bytes.Equal(reply.ResultDigest, authority.ResultDigest) &#124;&#124; !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(reply.CurrentStateJsonUtf8, &amp;current) != nil &#124;&#124; current.Sync != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED &amp;&amp; (current.Room != domain.TextID(install.NewRoom) &#124;&#124; current.Epoch != domain.TextID(install.Epoch) &#124;&#124; current.Lineage != domain.Tex …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=11565:13905 -->
````go
func (s *PullMain) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	var install draftInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	preview, digest := install.Preview, install.Digest
	if authority.State != pb.OperationResult_SUCCEEDED {
		// 拒绝收尾只使用原号系统终态；原来源关闭或会话失效也必须能解除自己的旧屏障。
		preview = "draft-preview:" + task.Operation
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: preview, ExpectedInputDigest: digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.State != authority.State || reply.OperationId != task.Operation || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
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
````

## 第 14 步：PullMain.reconcile（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 324 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>operation != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 14 bytes=13905:14473 -->
````go
	operation, err := repository.LockedBranchControl(ctx, tx, install.Branch)
	if err != nil {
		return err
	}
	if operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.CompletePullInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_main.go：保留原实现，按业务合并文件。

````

## 第 15 步：PullClient

插入位置：第 14 步代码之后，紧接追加；完成稿约第 342 行。

本步声明：`type PullClient interface {`。

**这一段为什么这样写：** 共同支线拉取主线的Node参与者，预览到安装保持同一原输入，保护正在编辑的内容。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Preview</code> | <code>(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)</code> | ctx与拉取预览请求返回固定主线/草稿输入引用。 |
| <code>Prepare</code> | <code>(context.Context, any) (&#91;&#93;byte, error)</code> | ctx和准备请求返回Node屏障事实字节，仍需按协议解析核验。 |
| <code>Apply</code> | <code>(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)</code> | ctx与固定拉取安装请求返回操作结果；新工作区安装完成后才恢复编辑。 |

<!-- write-step: 15 bytes=14473:14713 -->
````go
type PullClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	Prepare(context.Context, any) ([]byte, error)
	Apply(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)
}
````

## 第 16 步：PullMain

插入位置：第 15 步代码之后，紧接追加；完成稿约第 347 行。

本步声明：`type PullMain struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>PullClient</code> | 使用 <code>PullClient</code> 类型；其定义处的职责：共同支线拉取主线的Node参与者，预览到安装保持同一原输入，保护正在编辑的内容。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 16 bytes=14713:14865 -->
````go
type PullMain struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    PullClient
	validator *codec.Validator
	instance  string
}
````

## 第 17 步：pullContext

插入位置：第 16 步代码之后，紧接追加；完成稿约第 355 行。

本步声明：`type pullContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Document</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;document&quot;`</code> |
| <code>Unit</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;unit&quot;`</code> |
| <code>Room</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;room&quot;`</code> |
| <code>Input</code> | <code>dto.PullMain</code> | 序列化/校验标记：<code>`json:&quot;input&quot;`</code> |
| <code>Workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>序列化/校验标记：<code>`json:&quot;workspace&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 本次操作固定的主线清单，包含各单元版本引用。 |
| <code>Structure</code> | <code>uint64</code> | 固定的目录结构版本。 |
| <code>Style</code> | <code>uint64</code> | 固定的文档样式版本。 |
| <code>Control</code> | <code>uint64</code> | 固定的主线控制修订上下文；后续准备/安装必须匹配，不能从当前值覆盖原值。 |
| <code>MainEpoch</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;main_epoch,omitempty&quot;`</code> |

<!-- write-step: 17 bytes=14865:15687 -->
````go
type pullContext struct {
	Actor                               uint64               `json:"actor"`
	SessionExpires                      int64                `json:"session_expires"`
	Document                            uint64               `json:"document"`
	Unit                                uint64               `json:"unit"`
	Room                                uint64               `json:"room"`
	Input                               dto.PullMain         `json:"input"`
	Workspace                           domain.EditWorkspace `json:"workspace"`
	Access                              DocumentAccess       `json:"access"`
	Context                             *pb.OperationContext `json:"context"`
	Manifest, Structure, Style, Control uint64
	MainEpoch                           uint64 `json:"main_epoch,omitempty"`
}

````

## 第 18 步：NewPullMain

插入位置：第 17 步代码之后，紧接追加；完成稿约第 369 行。

本步声明：`func NewPullMain(db *sql.DB, identity *Identity, access *Access, client PullClient, validator *codec.Validator, instance string) *PullMain`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>PullClient</code> | 使用 <code>PullClient</code> 类型；其定义处的职责：共同支线拉取主线的Node参与者，预览到安装保持同一原输入，保护正在编辑的内容。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PullMain</code> | 实际返回表达式：<code>&amp;PullMain{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PullMain{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=15687:16016 -->
````go
func NewPullMain(db *sql.DB, identity *Identity, access *Access, client PullClient, validator *codec.Validator, instance string) *PullMain {
	return &PullMain{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

// pull_prepare.go：保留原实现，按业务合并文件。

````

## 第 19 步：PullMain.Prepare

插入位置：第 18 步代码之后，紧接追加；完成稿约第 375 行。

本步声明：`func (s *PullMain) Prepare(ctx context.Context, actor Actor, document, unit uint64, input dto.PullMain, path string) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;PULL_MAIN&quot;)</code>；<code>repository.PullResult(ctx, s.db, input.OperationId)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, branch, document, unit)</code>；<code>s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, input.OperationId, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;PULL_MAIN_INPUT_INVALID&quot;)</code>；<code>s.access.Authorize(ctx, actor, document, &quot;PULL_MAIN&quot;)</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;PULL_MAIN&quot;)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, branch, document, unit)</code>；<code>DocumentId: document</code>；<code>repository.LockedMergeBarrier(ctx, tx, document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>!access.Permissions.Edit&#91;unit&#93;</code>；<code>repository.SubmissionWorkspace(ctx, s.db, branch, document, unit)</code>；<code>UnitId: unit</code>；<code>repository.LockedActiveBranch(ctx, tx, unit, document)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.PullMain</code> | 在此声明中的实际使用：<code>input.ExpectedBranchId</code>；<code>input.ExpectedLineageId</code>；<code>input.ExpectedStructureId</code>；<code>input.OperationId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;PULL_MAIN_INPUT_INVALID&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !domain.ValidOperationID(input.OperationId)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;PULL_MAIN_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, present, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(&amp;pb.OperationContext{Sources: proof.Sources}, &amp;pb.OperationContext{Sources: bound.Sources})</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=16016:18162 -->
````go
func (s *PullMain) Prepare(ctx context.Context, actor Actor, document, unit uint64, input dto.PullMain, path string) (Actor, MutationBody, MutationPresentation, error) {
	branch, err := domain.CanonicalID(input.ExpectedBranchId)
	if err != nil {
		return actor, nil, nil, err
	}
	lineage, err := domain.CanonicalID(input.ExpectedLineageId)
	if err != nil {
		return actor, nil, nil, err
	}
	structure, err := domain.CanonicalID(input.ExpectedStructureId)
	if err != nil || !domain.ValidOperationID(input.OperationId) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "PULL_MAIN_INPUT_INVALID")
	}
	access, err := s.access.Authorize(ctx, actor, document, "PULL_MAIN")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	present := s.presentation(actor, access, input.OperationId)
	if _, err = repository.PullResult(ctx, s.db, input.OperationId); err == nil {
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, branch, document, unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "PULL_MAIN"}
	original := access.Proofs
	access.Proofs = nil
	for _, proof := range original {
		bound, err := s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, input.OperationId, target)
		if err != nil {
			return actor, nil, nil, err
		}
		if !proto.Equal(&pb.OperationContext{Sources: proof.Sources}, &pb.OperationContext{Sources: bound.Sources}) {
			return actor, nil, nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		access.Proofs = append(access.Proofs, bound)
	}
````

## 第 20 步：PullMain.Prepare（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 421 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.CurrentStructureId != input.ExpectedStructureId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; active != branch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>actual.Lineage != lineage</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>actual.Epoch != workspace.Epoch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;WORKSPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=18162:19814 -->
````go
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.CurrentStructureId != input.ExpectedStructureId {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if barrier.Phase != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		active, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil || active != branch {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
		}
		actual, err := repository.LockedControlledWorkspace(ctx, tx, branch, document, unit)
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
		}
		if actual.Lineage != lineage {
			return nil, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
		}
		if actual.Epoch != workspace.Epoch {
			return nil, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
		}
		if err = s.validateGuard(ctx, tx, unit); err != nil {
			return nil, err
		}
		if err = repository.RequireBranchControlOpen(ctx, tx, branch); err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "PULL_MAIN", admission); err != nil {
			return nil, err
		}
		rawTarget, _ := json.Marshal(target)
````

## 第 21 步：PullMain.Prepare（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 466 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return mainControlPending(input.OperationId), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=19814:21205 -->
````go
		if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, unit, input.OperationId, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		control, _ := domain.CanonicalRevision(current.ControlRevision)
		mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		fixed := pullContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Document: document, Unit: unit, Room: room, Input: input, Workspace: actual, Access: access, Manifest: manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Context: &pb.OperationContext{OperationId: input.OperationId, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}}
		payload, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterPull(ctx, tx, branch, input.OperationId, payload); err != nil {
			return nil, err
		}
		return mainControlPending(input.OperationId), nil
	}
	return actor, run, present, nil
}

````

## 第 22 步：PullMain.validateGuard

插入位置：第 21 步代码之后，紧接追加；完成稿约第 493 行。

固定旧审核轮次可以继续存在；未固定送审/正在合并则不能与改变支线代次互相越过。

本步声明：`func (s *PullMain) validateGuard(ctx context.Context, tx *sql.Tx, unit uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit)</code>；<code>repository.LockedActiveMerge(ctx, tx, request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit)</code>；<code>repository.LockedActiveMerge(ctx, tx, request)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;SUBMISSION_PENDING&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MERGE_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>guard.Submission != &quot;&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SUBMISSION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>request == 0</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>active != &quot;&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MERGE_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=21205:21984 -->
````go
// 固定旧审核轮次可以继续存在；未固定送审/正在合并则不能与改变支线代次互相越过。
func (s *PullMain) validateGuard(ctx context.Context, tx *sql.Tx, unit uint64) error {
	guard, err := repository.LockedReviewGuard(ctx, tx, unit)
	if err != nil {
		return err
	}
	if guard.Submission != "" {
		return domain.Violation(domain.StateConflict, "SUBMISSION_PENDING")
	}
	for _, request := range []uint64{guard.Ongoing, guard.Resumable} {
		if request == 0 {
			continue
		}
		active, err := repository.LockedActiveMerge(ctx, tx, request)
		if err != nil {
			return err
		}
		if active != "" {
			return domain.Violation(domain.StateConflict, "MERGE_PENDING")
		}
	}
	return nil
}

// pull_readiness.go：保留原实现，按业务合并文件。

````

## 第 23 步：PullMain.Ready

插入位置：第 22 步代码之后，紧接追加；完成稿约第 519 行。

本步声明：`func (s *PullMain) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingPullBindings(ctx, s.db)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PULL_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;PULL_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PULL_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Original, &amp;fixed) != nil</code>。条件成立时执行 <code>return errors.New(&quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>readErr != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=21984:22632 -->
````go
func (s *PullMain) Ready(ctx context.Context) error {
	facts, err := repository.PendingPullBindings(ctx, s.db)
	if err != nil {
		return errors.New("PULL_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err = validatePendingPullBinding(fact); err != nil {
			return err
		}
		var fixed pullContext
		if json.Unmarshal(fact.Original, &fixed) != nil {
			return errors.New("PULL_BINDING_UNKNOWN")
		}
		sources, readErr := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if readErr != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("PULL_BINDING_UNKNOWN")
		}
	}
	return nil
}
````

## 第 24 步：validatePendingPullBinding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 539 行。

本步声明：`func validatePendingPullBinding(fact repository.PendingPullBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingPullBinding</code> | 使用 <code>repository.PendingPullBinding</code> 类型；其定义处的职责：存储支线拉取MAIN的控制占位、lineage、代次与安装进度。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Gate</code>；<code>fact.Target</code>；<code>fact.Actor</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;fixed) != nil &#124;&#124; 		json.Unmarshal(fact.Gate, &amp;gate) != nil &#124;&#124; 		json.Unmarshal(fact.Target, &amp;target) != nil &#124;&#124; 		fixed.Context == nil &#124;&#124; 		fixed.Cont …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(fixed.Context.DocumentAdmissionRef) &#124;&#124; !domain.ValidOperationID(fixed.Context.IdentityAdmissionRef)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>businessTarget.Purpose != &quot;PULL_MAIN&quot; &#124;&#124; 		businessTarget.DocumentId != fixed.Document &#124;&#124; 		businessTarget.UnitId != fixed.Unit &#124;&#124; 		businessTarget.BranchId != fixed.Workspace.Work …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=22632:24633 -->
````go
func validatePendingPullBinding(fact repository.PendingPullBinding) error {
	invalid := errors.New("PULL_BINDING_UNKNOWN")
	var fixed pullContext
	var gate struct {
		Operation string `json:"operation_id"`
	}
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &fixed) != nil ||
		json.Unmarshal(fact.Gate, &gate) != nil ||
		json.Unmarshal(fact.Target, &target) != nil ||
		fixed.Context == nil ||
		fixed.Context.Target == nil ||
		fixed.Actor == 0 ||
		fixed.Actor != fact.Actor ||
		fixed.Context.ActorId != fixed.Actor ||
		fixed.SessionExpires <= 0 ||
		fixed.Document == 0 ||
		fixed.Unit == 0 ||
		fixed.Room == 0 ||
		fixed.Workspace.Workspace == 0 ||
		fixed.Workspace.Lineage == 0 ||
		fixed.Workspace.Epoch == 0 ||
		fixed.Workspace.BaseManifest == 0 ||
		fixed.Workspace.BaseVersion == 0 ||
		fixed.Input.OperationId != fact.Operation ||
		fixed.Context.OperationId != fact.Operation ||
		!domain.ValidOperationID(fact.Operation) ||
		gate.Operation != fact.Operation {
		return invalid
	}
	businessTarget := fixed.Context.Target
	if !domain.ValidOperationID(fixed.Context.DocumentAdmissionRef) || !domain.ValidOperationID(fixed.Context.IdentityAdmissionRef) {
		return invalid
	}
	if businessTarget.Purpose != "PULL_MAIN" ||
		businessTarget.DocumentId != fixed.Document ||
		businessTarget.UnitId != fixed.Unit ||
		businessTarget.BranchId != fixed.Workspace.Workspace ||
		businessTarget.StructureId != fixed.Structure ||
		businessTarget.WorkspaceEpoch != fixed.Workspace.Epoch ||
		fixed.Input.ExpectedBranchId != domain.TextID(fixed.Workspace.Workspace) ||
		fixed.Input.ExpectedLineageId != domain.TextID(fixed.Workspace.Lineage) ||
		fixed.Input.ExpectedStructureId != domain.TextID(fixed.Structure) ||
		!fixed.Access.Permissions.Edit[fixed.Unit] ||
		len(fixed.Access.Proofs) == 0 ||
		len(fixed.Access.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: fixed.Access.Sources}, &pb.OperationContext{Sources: fixed.Context.Sources}) {
````

## 第 25 步：validatePendingPullBinding（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 588 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>proof == nil &#124;&#124; 			proof.ActorId != fixed.Actor &#124;&#124; 			proof.OperationId != fact.Operation &#124;&#124; 			proof.Purpose != &quot;PULL_MAIN&quot; &#124;&#124; 			len(proof.SessionRef) != 32 &#124;&#124; 			!proto.Equal(pr …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>i &gt; 0 &amp;&amp; !bytes.Equal(proof.SessionRef, fixed.Access.Proofs&#91;0&#93;.SessionRef)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>validateOriginalAdmission(fixed, fact.Admission) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; target.Path != path &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint) &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;PULL_MAIN_RECOVER&quot;</code>。
- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; 			fact.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; 			fact.Epoch != fixed.Workspace.Epoch &#124;&#124; 			fact.Lineage != fixed.Workspace.Lineage &#124;&#124; 			json.Unmarshal(fact.Payload, &amp;progress) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(left, right)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;PULL_MAIN_RECONCILE&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;install) != nil &#124;&#124; !proto.Equal(install.Context, fixed.Context) &#124;&#124; install.Branch != fixed.Workspace.Workspace &#124;&#124; install.OldRoom != fixed.Room</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。

<!-- write-step: 25 bytes=24633:26359 -->
````go
		return invalid
	}
	for i, proof := range fixed.Access.Proofs {
		if proof == nil ||
			proof.ActorId != fixed.Actor ||
			proof.OperationId != fact.Operation ||
			proof.Purpose != "PULL_MAIN" ||
			len(proof.SessionRef) != 32 ||
			!proto.Equal(proof.Target, businessTarget) ||
			(i == 0 && proof.AdmissionId != fixed.Context.IdentityAdmissionRef) {
			return invalid
		}
		if i > 0 && !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return invalid
		}
	}
	if validateOriginalAdmission(fixed, fact.Admission) != nil {
		return invalid
	}
	path := "/v1/documents/" + domain.TextID(fixed.Document) + "/units/" + domain.TextID(fixed.Unit) + "/pull-main"
	fingerprint, err := domain.OperationFingerprint(path, fixed.Input)
	if err != nil || target.Path != path || !bytes.Equal(fingerprint[:], fact.Fingerprint) || !bytes.Equal(fingerprint[:], fixed.Context.RequestFingerprint) {
		return invalid
	}
	if fact.Kind == "PULL_MAIN_RECOVER" {
		var progress pullContext
		if fact.State != "PREPARING" ||
			fact.Lifecycle != "ACTIVE" ||
			fact.Epoch != fixed.Workspace.Epoch ||
			fact.Lineage != fixed.Workspace.Lineage ||
			json.Unmarshal(fact.Payload, &progress) != nil {
			return invalid
		}
		left, _ := json.Marshal(progress)
		right, _ := json.Marshal(fixed)
		if !bytes.Equal(left, right) {
			return invalid
		}
	} else if fact.Kind == "PULL_MAIN_RECONCILE" {
		var install draftInstall
		if json.Unmarshal(fact.Payload, &install) != nil || !proto.Equal(install.Context, fixed.Context) || install.Branch != fixed.Workspace.Workspace || install.OldRoom != fixed.Room {
			return invalid
		}
		if fact.State == "SUCCEEDED" {
			var result struct {
				Result map[string]string `json:"result"`
			}
````

## 第 26 步：validatePendingPullBinding（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 635 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Result, &amp;result) != nil &#124;&#124; 				install.NewRoom == 0 &#124;&#124; 				install.Lineage == 0 &#124;&#124; 				install.Epoch != fixed.Workspace.Epoch+1 &#124;&#124; 				install.Content == nil &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;REJECTED&quot;</code>。
- 检查 <code>install.NewRoom != 0 &#124;&#124; install.Content != nil &#124;&#124; fact.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; fact.Epoch != fixed.Workspace.Epoch &#124;&#124; fact.Lineage != fixed.Workspace.Lineage</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=26359:27427 -->
````go
			if json.Unmarshal(fact.Result, &result) != nil ||
				install.NewRoom == 0 ||
				install.Lineage == 0 ||
				install.Epoch != fixed.Workspace.Epoch+1 ||
				install.Content == nil ||
				install.Content.DocumentId != fixed.Document ||
				install.Content.UnitId != fixed.Unit ||
				install.Content.BranchId != fixed.Workspace.Workspace ||
				install.Content.WorkspaceEpoch != install.Epoch ||
				install.Content.LineageRef != domain.TextID(install.Lineage) ||
				len(install.Digest) != 32 ||
				install.Preview != "draft-preview:"+fact.Operation ||
				fact.Lifecycle != "RESTORING" ||
				fact.Epoch != install.Epoch ||
				fact.Lineage != install.Lineage ||
				result.Result["entity_id"] != domain.TextID(install.Lineage) {
				return invalid
			}
		} else if fact.State == "REJECTED" {
			if install.NewRoom != 0 || install.Content != nil || fact.Lifecycle != "ACTIVE" || fact.Epoch != fixed.Workspace.Epoch || fact.Lineage != fixed.Workspace.Lineage {
				return invalid
			}
		} else {
			return invalid
		}
	} else {
		return invalid
	}
	return nil
}

````

## 第 27 步：validateOriginalAdmission

插入位置：第 26 步代码之后，紧接追加；完成稿约第 666 行。

本步声明：`func validateOriginalAdmission(fixed pullContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code>；<code>fixed.Document</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Operation</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Document &#124;&#124; actual.Unit != fixed.Unit &#124;&#124; 		actual.Operation != fixed.Input.OperationId &#124;&#124; actual.Purpose != &quot;PULL_MAIN&quot; &#124;&#124; 	 …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=27427:28376 -->
````go
func validateOriginalAdmission(fixed pullContext, actual repository.OriginalAdmissionBinding) error {
	invalid := errors.New("PULL_BINDING_UNKNOWN")
	if fixed.Context == nil || fixed.Context.Target == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return invalid
	}
	var target pb.Target
	if actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != fixed.Unit ||
		actual.Operation != fixed.Input.OperationId || actual.Purpose != "PULL_MAIN" ||
		actual.Identity != fixed.Context.IdentityAdmissionRef ||
		actual.SessionExpires != fixed.SessionExpires || len(actual.Session) != 32 ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) ||
		json.Unmarshal(actual.Target, &target) != nil || !proto.Equal(&target, fixed.Context.Target) {
		return invalid
	}
	// 会话/来源到期属于业务拒绝或系统收尾；就绪检查只判断原记录有没有被换绑。
	return nil
}

````

## 第 28 步：equalPullSources

插入位置：第 27 步代码之后，紧接追加；完成稿约第 684 行。

本步声明：`func equalPullSources(left, right []*pb.SourceEpoch) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(left)</code>；<code>ordered(left)</code> |
| <code>right</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(right)</code>；<code>ordered(right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>ok &amp;&amp; proto.Equal(&amp;pb.OperationContext{Sources: a}, &amp;pb.OperationContext{Sources: b})</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(left) == 0 &#124;&#124; len(left) != len(right)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>value == nil &#124;&#124; value.ScopeKey == &quot;&quot; &#124;&#124; value.Epoch == 0</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result&#91;i&#93;.ScopeKey &lt; result&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>result&#91;i-1&#93;.ScopeKey == result&#91;i&#93;.ScopeKey</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return ok &amp;&amp; proto.Equal(&amp;pb.OperationContext{Sources: a}, &amp;pb.OperationContext{Sources: b})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=28376:29233 -->
````go
func equalPullSources(left, right []*pb.SourceEpoch) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	ordered := func(values []*pb.SourceEpoch) ([]*pb.SourceEpoch, bool) {
		result := append([]*pb.SourceEpoch(nil), values...)
		for _, value := range result {
			if value == nil || value.ScopeKey == "" || value.Epoch == 0 {
				return nil, false
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].ScopeKey < result[j].ScopeKey })
		for i := 1; i < len(result); i++ {
			if result[i-1].ScopeKey == result[i].ScopeKey {
				return nil, false
			}
		}
		return result, true
	}
	a, ok := ordered(left)
	if !ok {
		return false
	}
	b, ok := ordered(right)
	return ok && proto.Equal(&pb.OperationContext{Sources: a}, &pb.OperationContext{Sources: b})
}

// pull_readings.go：保留原实现，按业务合并文件。

````

## 第 29 步：PullMain.Lookup

插入位置：第 28 步代码之后，紧接追加；完成稿约第 713 行。

本步声明：`func (s *PullMain) Lookup(ctx context.Context, operation string) (pullContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;PULL_MAIN_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;PULL_MAIN_CONTEXT&quot;, operation)</code>；<code>fixed.Input.OperationId != operation</code>；<code>fixed.Context.OperationId != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>pullContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Input.OperationId != operation &#124;&#124; fixed.Context.OperationId != operation</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=29233:29755 -->
````go
func (s *PullMain) Lookup(ctx context.Context, operation string) (pullContext, error) {
	var fixed pullContext
	raw, err := repository.MergeArtifact(ctx, s.db, "PULL_MAIN_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Input.OperationId != operation || fixed.Context.OperationId != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
	return fixed, nil
}
````

## 第 30 步：PullMain.presentation

插入位置：第 29 步代码之后，紧接追加；完成稿约第 724 行。

本步声明：`func (s *PullMain) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code>；<code>access.Sources</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readResult(ctx, tx, actor, access, operation) 		return value, access.Sources, err 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readResult(ctx, tx, actor, access, operation) 		return value, access.Sources, err 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return value, access.Sources, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=29755:30067 -->
````go
func (s *PullMain) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}
````

## 第 31 步：PullMain.Progress

插入位置：第 30 步代码之后，紧接追加；完成稿约第 730 行。

本步声明：`func (s *PullMain) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;PULL_MAIN_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;PULL_MAIN_RESULT&quot;)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PULL_MAIN_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;PULL_MAIN_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return s.readResult(ctx, tx, actor, access, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=30067:30664 -->
````go
func (s *PullMain) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PULL_MAIN_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "PULL_MAIN_RESULT")
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}
````

## 第 32 步：PullMain.readResult

插入位置：第 31 步代码之后，紧接追加；完成稿约第 746 行。

本步声明：`func (s *PullMain) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.PullResult(ctx, tx, operation)</code>；<code>repository.PullInstalled(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.PullResult(ctx, tx, operation)</code>；<code>repository.PullInstalled(ctx, tx, operation)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.PullResult(ctx, tx, operation)</code>；<code>repository.PullInstalled(ctx, tx, operation)</code>；<code>&quot;/v1/operations/&quot; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PULL_MAIN_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;PULL_MAIN_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>!ready</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=30664:32197 -->
````go
func (s *PullMain) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	fact, err := repository.PullResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if fact.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "PULL_MAIN_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	if fact.State == "SUCCEEDED" {
		ready, err := repository.PullInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		result, ok := value["result"].(map[string]any)
		if !ok {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
		}
		// 冻结Operation没有草稿同步字段。HTTP投影WAITING直至系统完成；RPC仍交付不可变SUCCEEDED事实。
		// 首轮已固定回执多带了支线字段，只投影契约已有ID，不改原回执字节或系统摘要。
		ids := map[string]any{"entity_id": result["entity_id"]}
		if version, exists := result["content_version_id"]; exists {
			ids["content_version_id"] = version
		}
		value["result"] = ids
		if !ready {
			value["state"] = "WAITING"
			value["poll_url"] = "/v1/operations/" + operation
		}
	}
	return value, nil
}
````

## 第 33 步：PullMain.OperationResult

插入位置：第 32 步代码之后，紧接追加；完成稿约第 787 行。

本步声明：`func (s *PullMain) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, query.OperationId)</code>；<code>repository.PullResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor &#124;&#124; !bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=32197:33924 -->
````go
func (s *PullMain) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.PullResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// pull_recovery.go：保留原实现，按业务合并文件。

````

## 第 34 步：PullMain.RecoverOne

插入位置：第 33 步代码之后，紧接追加；完成稿约第 829 行。

本步声明：`func (s *PullMain) RecoverOne(parent context.Context)`。


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
- 检查 <code>task.Kind == &quot;PULL_MAIN_RECONCILE&quot;</code>。
- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound &#124;&#124; violation.Kind == domain.StateConflict)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 34 bytes=33924:35328 -->
````go
func (s *PullMain) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimPullTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "PULL_MAIN_RECONCILE" {
		err = s.reconcile(ctx, task)
	} else {
		var fixed pullContext
		if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
			err = domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
		} else {
			proofs, resumeError := s.resume(ctx, fixed)
			err = resumeError
			if err == nil {
				err = s.validateBeforeCompute(ctx, fixed, proofs)
			}
			if err == nil {
				_, prepared, computeError := s.compute(ctx, fixed)
				err = computeError
				if err == nil {
					err = s.commit(ctx, task, fixed, prepared, proofs)
				}
			}
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				err = s.reject(ctx, task, fixed, violation.Code)
			}
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

````

## 第 35 步：PullMain.reject

插入位置：第 34 步代码之后，紧接追加；完成稿约第 876 行。

本步声明：`func (s *PullMain) reject(ctx context.Context, task domain.SubmissionTask, fixed pullContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Document, true)</code>；<code>repository.LockedBranchControl(ctx, tx, fixed.Workspace.Workspace)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Workspace</code>；<code>fixed.Context</code>；<code>fixed.Room</code> |
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
- 检查 <code>operation != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 35 bytes=35328:37123 -->
````go
func (s *PullMain) reject(ctx context.Context, task domain.SubmissionTask, fixed pullContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, fixed.Document, true); err != nil {
		return err
	}
	operation, err := repository.LockedBranchControl(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return err
	}
	if operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	// 拒绝也要原号系统确认后解锁自己的Node屏障；不能把RPC超时当作已恢复OPEN。
	install, _ := json.Marshal(draftInstall{Context: fixed.Context, Branch: fixed.Workspace.Workspace, OldRoom: fixed.Room, Epoch: fixed.Workspace.Epoch})
	if err = repository.InsertSubmissionTask(ctx, tx, "PULL_MAIN_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_source.go：保留原实现，按业务合并文件。

````

## 第 36 步：PullMain.resume

插入位置：第 35 步代码之后，紧接追加；完成稿约第 918 行。

本步声明：`func (s *PullMain) resume(ctx context.Context, fixed pullContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, proof)</code> |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=37123:37585 -->
````go
func (s *PullMain) resume(ctx context.Context, fixed pullContext) ([]*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	var result []*pb.IdentityAdmission
	for _, proof := range fixed.Access.Proofs {
		current, err := s.identity.Resume(ctx, proof)
		if err != nil {
			return nil, err
		}
		result = append(result, current)
	}
	return result, nil
}
````

## 第 37 步：PullMain.validate

插入位置：第 36 步代码之后，紧接追加；完成稿约第 932 行。

本步声明：`func (s *PullMain) validate(ctx context.Context, tx *sql.Tx, fixed pullContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>repository.LockedControlledWorkspace(ctx, tx, fixed.Workspace.Workspace, fixed.Document, fixed.Unit)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>repository.LockedControlledWorkspace(ctx, tx, fixed.Workspace.Workspace, fixed.Document, fixed.Unit)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>pullContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Unit</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Edit&#91;fixed.Unit&#93;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(fixed.Manifest) &#124;&#124; current.CurrentStructureId != domain.TextID(fixed.Structure) &#124;&#124; current.CurrentStyleId != domain.TextID(fixed.Style) &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>workspace != fixed.Workspace</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>operation != fixed.Input.OperationId</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;BRANCH_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateOriginalAdmission(fixed, admission) != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PULL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 37 bytes=37585:39659 -->
````go
func (s *PullMain) validate(ctx context.Context, tx *sql.Tx, fixed pullContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Edit[fixed.Unit] {
		return domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	if current.CurrentManifestId != domain.TextID(fixed.Manifest) || current.CurrentStructureId != domain.TextID(fixed.Structure) || current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, fixed.Workspace.Workspace, fixed.Document, fixed.Unit)
	if err != nil {
		return domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	if workspace != fixed.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	if err = s.validateGuard(ctx, tx, fixed.Unit); err != nil {
		return err
	}
	operation, err := repository.LockedBranchControl(ctx, tx, workspace.Workspace)
	if err != nil {
		return err
	}
	if operation != fixed.Input.OperationId {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil || validateOriginalAdmission(fixed, admission) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
````

## 第 38 步：PullMain.validate（接着实现）

插入位置：第 37 步代码之后，紧接追加；完成稿约第 978 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=39659:39784 -->
````go
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

````

## 第 39 步：PullMain.DraftSource

插入位置：第 38 步代码之后，紧接追加；完成稿约第 981 行。

本步声明：`func (s *PullMain) DraftSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>input.ExpectedBranchId</code>；<code>input.ExpectedLineageId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 由 <code>readDraftSource(ctx, tx, draftSourceScope{Document: fixed.Document, Unit: fixed.Unit, Room: fixed.Room, Manifest: fixed.Manifest, Structure: fixed.Structure, Style: fixed.Style, Co …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>readDraftSource(ctx, tx, draftSourceScope{Document: fixed.Document, Unit: fixed.Unit, Room: fixed.Room, Manifest: fixed.Manifest, Structure: fixed.Structure, Style: fixed.Style, Co …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(fixed.Context, input.Context) &#124;&#124; input.ExpectedBranchId != fixed.Workspace.Workspace &#124;&#124; input.ExpectedLineageId != fixed.Workspace.Lineage &#124;&#124; input.ExpectedStructureId != fixed.Structure</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return readDraftSource(ctx, tx, draftSourceScope{Document: fixed.Document, Unit: fixed.Unit, Room: fixed.Room, Manifest: fixed.Manifest, Structure: fixed.Structure, Style: fixed.St …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=39784:40916 -->
````go
func (s *PullMain) DraftSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) || input.ExpectedBranchId != fixed.Workspace.Workspace || input.ExpectedLineageId != fixed.Workspace.Lineage || input.ExpectedStructureId != fixed.Structure {
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
	return readDraftSource(ctx, tx, draftSourceScope{Document: fixed.Document, Unit: fixed.Unit, Room: fixed.Room, Manifest: fixed.Manifest, Structure: fixed.Structure, Style: fixed.Style, Control: fixed.Control, MainEpoch: fixed.MainEpoch, Workspace: fixed.Workspace})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/pull_main_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
