# 分步编写 `services/document/internal/logic/document/conflict_resolutions_execution_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/197-conflict-resolutions-execution-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

登记并提交人工冲突修复，安装新支线并验证原固定冲突证据。

恢复payload互相一致仍可能同时错误，安装必须与实际不可变版本、lineage及原候选交叉核验。

写之前先按这条执行路线理解各段的连接：

1. Register核冲突完整集合与当前权限
2. original读取原attempt
3. resolutionBaseline投影原冲突MAIN
4. commit写新工作区
5. validateInstallSource核实际工件
6. reconcile等待安装。

本文件需要守住的边界：

- 新登记受公共冲突TTL约束，已登记系统恢复继续核原事实
- 不能把后来MAIN尾部擅自提升为已接纳基线。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_execution_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_execution_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_execution_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/conflict_resolutions_execution_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:622 -->
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
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sort"
	"time"
)

// conflict_resolution_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：Resolutions.commit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (s *Resolutions) commit(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>lockResolutionOperations(ctx, tx, fixed)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>task.Operation</code>；<code>repository.CompleteResolution(ctx, tx, task, &quot;SUCCEEDED&quot;, result, encoded)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>lockResolutionOperations(ctx, tx, fixed)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>s.fixedPreview(ctx, tx, raw, fixed)</code>；<code>resolutionBaseline(ctx, tx, fixed)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>draftPrepared</code> | 在此声明中的实际使用：<code>prepared.Digest</code>；<code>prepared.Candidate</code>；<code>prepared.Main</code>；<code>prepared.Preview</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARED_BINDING_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_BASELINE_UNKNOWN&quot;)</code>；<code>e</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(expected.Digest, prepared.Digest) &#124;&#124; !proto.Equal(expected.Candidate, prepared.Candidate) &#124;&#124; !proto.Equal(expected.Main, prepared.Main)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !matched</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_BASELINE_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=622:2267 -->
````go
func (s *Resolutions) commit(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return err
	}
	original, err := s.validate(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	raw, err := repository.MergeArtifact(ctx, tx, "RESOLUTION_FIXED", task.Operation)
	if err != nil {
		return err
	}
	expected, err := s.fixedPreview(ctx, tx, raw, fixed)
	if err != nil {
		return err
	}
	if !bytes.Equal(expected.Digest, prepared.Digest) || !proto.Equal(expected.Candidate, prepared.Candidate) || !proto.Equal(expected.Main, prepared.Main) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARED_BINDING_UNKNOWN")
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return err
	}
	stored, err := repository.VersionContent(ctx, tx, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	matched, err := conflictVersionMatches(stored, baseline, fixed.BaselineVersion)
	if err != nil || !matched {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BASELINE_UNKNOWN")
	}
	scope := fixed.Scope
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resourceSet := map[uint64]bool{}
	for _, part := range []*pb.FixedContent{baseline, prepared.Candidate} {
		for _, id := range part.ResourceIds {
			resourceSet[id] = true
		}
	}
````

## 第 3 步：Resolutions.commit（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 72 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOLUTION_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PROVENANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>epoch == 0</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;WORKSPACE_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2267:3915 -->
````go
	ids := make([]uint64, 0, len(resourceSet))
	for id := range resourceSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, e := repository.LockedDocumentResource(ctx, tx, id, scope.Document, space)
		if e != nil {
			return e
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "RESOLUTION_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	lineage, err := NewEntityID()
	if err != nil {
		return err
	}
	decoded, err := codec.Decode(prepared.Candidate.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance, ok := decoded.(map[string]any)
	if !ok {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PROVENANCE_UNKNOWN")
	}
	manifest := original.Attempt.Main.ExpectedManifestId
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(manifest), domain.TextID(fixed.BaselineVersion)
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	provenanceRaw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertResolvedLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, manifest, fixed.BaselineVersion, task.Operation, prepared.Preview, provenanceRaw); err != nil {
		return err
	}
	epoch := scope.Workspace.Epoch + 1
	if epoch == 0 {
		return domain.Violation(domain.StateConflict, "WORKSPACE_EPOCH_EXHAUSTED")
	}
````

## 第 4 步：Resolutions.commit（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 117 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 4 bytes=3915:6000 -->
````go
	room, err := collaborationRoom(ctx, tx, "BRANCH", scope.Workspace.Workspace, scope.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch, content.LineageRef, content.ProvenanceJsonUtf8 = epoch, domain.TextID(lineage), provenanceRaw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	version, err := repository.InsertMergeVersion(ctx, tx, content, "RESTORE", content.CaptureId)
	if err != nil {
		return err
	}
	branchBody, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	body, err := codec.Decode(branchBody)
	if err != nil {
		return err
	}
	install := resolutionInstall{Fixed: fixed, Version: version, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(fixed.BaselineVersion), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
		return err
	}
	// 不改变正式MAIN、原轮次或意见。新谱系、可靠修复版本、RESTORING及系统安装责任同事务提交。
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompleteResolution(ctx, tx, task, "SUCCEEDED", result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_install.go：保留原实现，按业务合并文件。

````

## 第 5 步：Resolutions.validateInstallSource

插入位置：第 4 步代码之后，紧接追加；完成稿约第 160 行。

系统安装读取实际版本、谱系与原固定候选交叉核验；两份可变恢复payload相互一致不等于可信。

本步声明：`func (s *Resolutions) validateInstallSource(ctx context.Context, q *sql.Tx, install resolutionInstall) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, q, &quot;RESOLUTION_CONTEXT&quot;, install.Context.OperationId)</code>；<code>repository.ResolutionResult(ctx, q, fixed.Intent.Operation)</code>；<code>repository.MergeArtifact(ctx, q, &quot;RESOLUTION_FIXED&quot;, fixed.Intent.Operation)</code>；<code>s.fixedPreview(ctx, q, previewRaw, fixed)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>q</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, q, &quot;RESOLUTION_CONTEXT&quot;, install.Context.OperationId)</code>；<code>repository.ResolutionResult(ctx, q, fixed.Intent.Operation)</code>；<code>repository.MergeArtifact(ctx, q, &quot;RESOLUTION_FIXED&quot;, fixed.Intent.Operation)</code>；<code>s.fixedPreview(ctx, q, previewRaw, fixed)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>install</code> | <code>resolutionInstall</code> | 在此声明中的实际使用：<code>install.Context</code>；<code>install.Fixed</code>；<code>install.Content</code>；<code>install.Version</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ORIGINAL_COPY_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_BINDING_UNKNOWN&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>install.Context == nil &#124;&#124; install.Context.Target == nil</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !sameResolutionContext(given, fixed) &#124;&#124; !proto.Equal(install.Context, fixed.Context) &#124;&#124; install.Content == nil &#124;&#124; install.Version == 0</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ORIGINAL_COPY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;SUCCEEDED&quot; &#124;&#124; receipt.Actor != fixed.Actor &#124;&#124; !bytes.Equal(receipt.Fingerprint, fixed.Context.RequestFingerprint) &#124;&#124; json.Unmarshal(receipt.Result, &amp;result) != ni …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(preview.Digest, install.Digest)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=6000:8265 -->
````go
// 系统安装读取实际版本、谱系与原固定候选交叉核验；两份可变恢复payload相互一致不等于可信。
func (s *Resolutions) validateInstallSource(ctx context.Context, q *sql.Tx, install resolutionInstall) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	if install.Context == nil || install.Context.Target == nil {
		return unknown
	}
	raw, err := repository.MergeArtifact(ctx, q, "RESOLUTION_CONTEXT", install.Context.OperationId)
	if err != nil {
		return err
	}
	fixed, err := decodeResolutionContext(raw)
	if err != nil {
		return err
	}
	givenRaw, _ := json.Marshal(install.Fixed)
	given, err := decodeResolutionContext(givenRaw)
	if err != nil || !sameResolutionContext(given, fixed) || !proto.Equal(install.Context, fixed.Context) || install.Content == nil || install.Version == 0 {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ORIGINAL_COPY_UNKNOWN")
	}
	// 固定成功结果也参与交叉核验；恢复payload保存正确正文不能掩盖结果指向另一真实旧版本。
	receipt, err := repository.ResolutionResult(ctx, q, fixed.Intent.Operation)
	if err != nil {
		return err
	}
	var result struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Result    struct {
			Lineage string `json:"entity_id"`
			Version string `json:"content_version_id"`
		} `json:"result"`
	}
	if receipt.State != "SUCCEEDED" || receipt.Actor != fixed.Actor || !bytes.Equal(receipt.Fingerprint, fixed.Context.RequestFingerprint) || json.Unmarshal(receipt.Result, &result) != nil || result.Operation != fixed.Intent.Operation || result.State != "SUCCEEDED" || result.Result.Lineage != domain.TextID(install.Lineage) || result.Result.Version != domain.TextID(install.Version) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_BINDING_UNKNOWN")
	}
	previewRaw, err := repository.MergeArtifact(ctx, q, "RESOLUTION_FIXED", fixed.Intent.Operation)
	if err != nil {
		return err
	}
	preview, err := s.fixedPreview(ctx, q, previewRaw, fixed)
	if err != nil {
		return err
	}
	if !bytes.Equal(preview.Digest, install.Digest) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	}
````

## 第 6 步：Resolutions.validateInstallSource（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 206 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !baseMatches</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_BASELINE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !proto.Equal(expected, install.Content)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CANDIDATE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !matched</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_VERSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>lineage.Branch != fixed.Scope.Workspace.Workspace &#124;&#124; lineage.Document != fixed.Scope.Document &#124;&#124; lineage.Unit != fixed.Scope.Unit &#124;&#124; lineage.Previous != fixed.Scope.Workspace.Linea …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_LINEAGE_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=8265:10542 -->
````go
	baseline, err := resolutionBaseline(ctx, q, fixed)
	if err != nil {
		return err
	}
	baseVersion, err := repository.VersionContent(ctx, q, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	baseMatches, err := conflictVersionMatches(baseVersion, baseline, fixed.BaselineVersion)
	if err != nil || !baseMatches {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BASELINE_UNKNOWN")
	}
	expected := proto.Clone(preview.Candidate).(*pb.FixedContent)
	provenanceValue, err := codec.Decode(expected.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance, ok := provenanceValue.(map[string]any)
	if !ok {
		return unknown
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(install.Lineage), install.LineageContext["base_manifest_id"], domain.TextID(fixed.BaselineVersion)
	expected.WorkspaceEpoch, expected.LineageRef = install.Epoch, domain.TextID(install.Lineage)
	expected.ProvenanceJsonUtf8, err = codec.Canonical(provenance)
	if err != nil {
		return err
	}
	expected.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(install.NewRoom), "workspace_epoch": domain.TextID(install.Epoch), "durable_seq": "0"})
	if err != nil || !proto.Equal(expected, install.Content) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CANDIDATE_UNKNOWN")
	}
	stored, err := repository.VersionContent(ctx, q, install.Version, fixed.Scope.Document)
	if err != nil {
		return err
	}
	matched, err := conflictVersionMatches(stored, install.Content, install.Version)
	if err != nil || !matched {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSION_UNKNOWN")
	}
	lineage, err := repository.ResolutionLineage(ctx, q, install.Lineage)
	if err != nil {
		return err
	}
	if lineage.Branch != fixed.Scope.Workspace.Workspace || lineage.Document != fixed.Scope.Document || lineage.Unit != fixed.Scope.Unit || lineage.Previous != fixed.Scope.Workspace.Lineage || lineage.Version != fixed.BaselineVersion || lineage.Operation != fixed.Intent.Operation || lineage.Capture != fixed.Intent.Operation || lineage.Kind != "RESTORED" {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_LINEAGE_UNKNOWN")
	}
````

## 第 7 步：Resolutions.validateInstallSource（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 252 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; lineage.Manifest != original.Attempt.Main.ExpectedManifestId</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, install.Content.ProvenanceJsonUtf8)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PROVENANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>e1 != nil &#124;&#124; e2 != nil &#124;&#124; !bytes.Equal(a, b)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_LINEAGE_CONTEXT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(roomRaw, &amp;room) != nil &#124;&#124; room&#91;&quot;room_id&quot;&#93; != domain.TextID(install.NewRoom) &#124;&#124; install.Branch != fixed.Scope.Workspace.Workspace &#124;&#124; install.OldRoom != fixed.Scope.Ro …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ROOM_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=10542:12459 -->
````go
	original, _, err := s.original(ctx, q, fixed.Origin, fixed.Intent, false)
	if err != nil || lineage.Manifest != original.Attempt.Main.ExpectedManifestId {
		return unknown
	}
	provenanceDecoded, err := codec.Decode(lineage.Provenance)
	if err != nil {
		return unknown
	}
	p, ok := provenanceDecoded.(map[string]any)
	if !ok {
		return unknown
	}
	canonical, err := codec.Canonical(p)
	if err != nil || !bytes.Equal(canonical, install.Content.ProvenanceJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PROVENANCE_UNKNOWN")
	}
	body, err := repository.VersionContent(ctx, q, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	projected, err := domain.BranchBaselineBody(body.BodyJsonUtf8)
	if err != nil {
		return err
	}
	expectedBody, err := codec.Decode(projected)
	if err != nil {
		return err
	}
	expectedLineage := map[string]any{"lineage_id": domain.TextID(install.Lineage), "base_manifest_id": domain.TextID(lineage.Manifest), "base_version_id": domain.TextID(fixed.BaselineVersion), "body": expectedBody, "provenance": p}
	a, e1 := codec.Canonical(expectedLineage)
	b, e2 := codec.Canonical(install.LineageContext)
	if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_LINEAGE_CONTEXT_UNKNOWN")
	}
	roomRaw, err := repository.MergeArtifact(ctx, q, "COLLABORATION_ROOM", "BRANCH:"+domain.TextID(install.Branch)+":"+domain.TextID(fixed.Scope.Unit)+":"+domain.TextID(install.Epoch))
	if err != nil {
		return err
	}
	var room map[string]string
	if json.Unmarshal(roomRaw, &room) != nil || room["room_id"] != domain.TextID(install.NewRoom) || install.Branch != fixed.Scope.Workspace.Workspace || install.OldRoom != fixed.Scope.Room || install.Epoch != fixed.Scope.Workspace.Epoch+1 {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ROOM_UNKNOWN")
	}
	return nil
}

````

## 第 8 步：Resolutions.CommittedSource

插入位置：第 7 步代码之后，紧接追加；完成稿约第 297 行。

本步声明：`func (s *Resolutions) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;RESOLUTION_COMMITTED&quot;, query.OperationId)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validateInstallSource(ctx, tx, install)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>query.OperationId</code>；<code>validatePullCommittedBinding(install.draftInstall, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_UNKNOWN&quot;)</code>；<code>beginError</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED &amp;&amp; authority.State != pb.OperationResult_REJECTED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>decodeError != nil &#124;&#124; validatePullCommittedBinding(install.draftInstall, query) != nil &#124;&#124; install.Fixed.Intent.Operation != query.OperationId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>beginError != nil</code>。条件成立时执行 <code>return nil, beginError</code>。当前函数在这里结束。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return nil, e</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=12459:14037 -->
````go
func (s *Resolutions) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "RESOLUTION_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	install, decodeError := decodeResolutionInstall(raw)
	if decodeError != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest), "expected_round_id": domain.TextID(install.Fixed.Intent.Round)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		tx, beginError := s.db.BeginTx(ctx, nil)
		if beginError != nil {
			return nil, beginError
		}
		err = s.validateInstallSource(ctx, tx, install)
		_ = tx.Rollback()
		if err != nil {
			return nil, err
		}
		content, e := fixedWire(install.Content)
		if e != nil {
			return nil, e
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

````

## 第 9 步：Resolutions.reconcile

插入位置：第 8 步代码之后，紧接追加；完成稿约第 333 行。

本步声明：`func (s *Resolutions) reconcile(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>s.CommittedSource(ctx, query)</code>；<code>s.client.Apply(ctx, &amp;pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteResolutionInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_ACK_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CONTROL_UNKNOWN&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>decodeError != nil &#124;&#124; install.Context == nil &#124;&#124; install.Fixed.Intent.Operation != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply == nil &#124;&#124; reply.OperationId != task.Operation &#124;&#124; reply.State != authority.State &#124;&#124; !bytes.Equal(reply.ResultDigest, authority.ResultDigest) &#124;&#124; !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(reply.CurrentStateJsonUtf8, &amp;current) != nil &#124;&#124; current.Sync != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED &amp;&amp; (current.Room != domain.TextID(install.NewRoom) &#124;&#124; current.Epoch != domain.TextID(install.Epoch) &#124;&#124; current.Lineage != domain.Tex …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=14037:16394 -->
````go
func (s *Resolutions) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	install, decodeError := decodeResolutionInstall(task.Payload)
	if decodeError != nil || install.Context == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if _, err = s.CommittedSource(ctx, query); err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, install.Fixed); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, query.Target.DocumentId, true); err != nil {
		return err
	}
	owner, err := repository.LockedBranchControl(ctx, tx, install.Branch)
````

## 第 10 步：Resolutions.reconcile（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 378 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>pointer != install.Fixed.Review &#124;&#124; guard != install.Fixed.Guard</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 10 bytes=16394:17484 -->
````go
	if err != nil {
		return err
	}
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 这里只完成成功时登记的系统责任；注销不能取消已经提交的修复，也不触发意见解除或请求归档。
	if authority.State == pb.OperationResult_SUCCEEDED {
		if err = s.validateInstallSource(ctx, tx, install); err != nil {
			return err
		}
		pointer, guard, e := resolutionReview(ctx, tx, install.Fixed.Origin, install.Fixed.Intent.Round)
		if e != nil {
			return e
		}
		if pointer != install.Fixed.Review || guard != install.Fixed.Guard {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_REVIEW_CHANGED")
		}
	}
	if err = repository.CompleteResolutionInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_original.go：保留原实现，按业务合并文件。

````

## 第 11 步：Resolutions.original

插入位置：第 10 步代码之后，紧接追加；完成稿约第 408 行。

登记时要求公共冲突仍有效；已经登记的系统恢复继续核验原固定事实，不依赖公共页面的TTL。

本步声明：`func (s *Resolutions) original(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, intent domain.ConflictResolutionIntent, fresh bool) (mergeFixedInputs, conflictVersions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeRow(ctx, tx, origin.Operation)</code>；<code>repository.MergeResult(ctx, tx, origin.Operation)</code>；<code>repository.ConflictAttemptRow(ctx, tx, origin.Operation)</code>；<code>readConflictFixed(ctx, tx, s.validator, origin, attempt)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeRow(ctx, tx, origin.Operation)</code>；<code>repository.MergeResult(ctx, tx, origin.Operation)</code>；<code>repository.ConflictAttemptRow(ctx, tx, origin.Operation)</code>；<code>readConflictFixed(ctx, tx, s.validator, origin, attempt)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>origin</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>origin.Operation</code>；<code>reflect.DeepEqual(actual, origin)</code>；<code>origin.Round</code>；<code>readConflictFixed(ctx, tx, s.validator, origin, attempt)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.ConflictResolutionIntent</code> | 使用 <code>domain.ConflictResolutionIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>intent.Attempt</code>；<code>intent.Digest</code>；<code>intent.Round</code>；<code>intent.ValidateConflictSet</code> |
| <code>fresh</code> | <code>bool</code> | 在此声明中的实际使用：<code>fresh &amp;&amp; !receipt.Expires.After(time.Now())</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 实际返回表达式：<code>empty</code>；<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>conflictVersions</code> | 实际返回表达式：<code>conflictVersions{}</code>；<code>versions</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;CONFLICT_ATTEMPT_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !reflect.DeepEqual(actual, origin)</code>。条件成立时执行 <code>return empty, conflictVersions{}, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, conflictVersions{}, err</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;REJECTED&quot; &#124;&#124; (fresh &amp;&amp; !receipt.Expires.After(time.Now()))</code>。条件成立时执行 <code>return empty, conflictVersions{}, domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, conflictVersions{}, err</code>。当前函数在这里结束。
- 检查 <code>attempt.State != &quot;CONFLICT&quot; &#124;&#124; attempt.ID != intent.Attempt &#124;&#124; !bytes.Equal(attempt.Digest, intent.Digest) &#124;&#124; origin.Round != intent.Round</code>。条件成立时执行 <code>return empty, conflictVersions{}, domain.Violation(domain.StateConflict, &quot;CONFLICT_ATTEMPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(attempt.Conflicts, &amp;conflicts) != nil</code>。条件成立时执行 <code>return empty, conflictVersions{}, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, conflictVersions{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, conflictVersions{}, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint)</code>。条件成立时执行 <code>return fixed, conflictVersions{}, unknown</code>。当前函数在这里结束。
- 检查 <code>fresh</code>。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return fixed, versions, e</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=17484:19641 -->
````go
// 登记时要求公共冲突仍有效；已经登记的系统恢复继续核验原固定事实，不依赖公共页面的TTL。
func (s *Resolutions) original(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, intent domain.ConflictResolutionIntent, fresh bool) (mergeFixedInputs, conflictVersions, error) {
	var empty mergeFixedInputs
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	actual, err := repository.MergeRow(ctx, tx, origin.Operation)
	if err != nil || !reflect.DeepEqual(actual, origin) {
		return empty, conflictVersions{}, unknown
	}
	receipt, err := repository.MergeResult(ctx, tx, origin.Operation)
	if err != nil {
		return empty, conflictVersions{}, err
	}
	if receipt.State != "REJECTED" || (fresh && !receipt.Expires.After(time.Now())) {
		return empty, conflictVersions{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	attempt, err := repository.ConflictAttemptRow(ctx, tx, origin.Operation)
	if err != nil {
		return empty, conflictVersions{}, err
	}
	if attempt.State != "CONFLICT" || attempt.ID != intent.Attempt || !bytes.Equal(attempt.Digest, intent.Digest) || origin.Round != intent.Round {
		return empty, conflictVersions{}, domain.Violation(domain.StateConflict, "CONFLICT_ATTEMPT_CHANGED")
	}
	var conflicts []dto.Conflict
	if json.Unmarshal(attempt.Conflicts, &conflicts) != nil {
		return empty, conflictVersions{}, unknown
	}
	if err = intent.ValidateConflictSet(len(conflicts)); err != nil {
		return empty, conflictVersions{}, err
	}
	fixed, err := readConflictFixed(ctx, tx, s.validator, origin, attempt)
	if err != nil {
		return fixed, conflictVersions{}, err
	}
	if !bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint) {
		return fixed, conflictVersions{}, unknown
	}
	var versions conflictVersions
	if fresh {
		versions, err = ensureConflictVersions(ctx, tx, origin, fixed)
	} else {
		raw, e := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(origin.Operation, attempt.ID))
		if e != nil {
			return fixed, versions, e
		}
		versions, err = validateConflictVersions(ctx, tx, raw, fixed)
	}
````

## 第 12 步：Resolutions.original（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 454 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return fixed, versions, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=19641:19673 -->
````go
	return fixed, versions, err
}

````

## 第 13 步：lockResolutionOperations

插入位置：第 12 步代码之后，紧接追加；完成稿约第 457 行。

本步声明：`func lockResolutionOperations(ctx context.Context, tx *sql.Tx, fixed resolutionContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockOperationGate(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockOperationGate(ctx, tx, operation)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=19673:20133 -->
````go
func lockResolutionOperations(ctx context.Context, tx *sql.Tx, fixed resolutionContext) error {
	// 两个操作互斥都先于来源与文档锁；GET物化旧冲突也采用这个方向，避免交叉等待。
	operations := []string{fixed.Intent.Operation, fixed.Intent.Conflict}
	sort.Strings(operations)
	for _, operation := range operations {
		if err := repository.LockOperationGate(ctx, tx, operation); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 14 步：resolutionReview

插入位置：第 13 步代码之后，紧接追加；完成稿约第 469 行。

本步声明：`func resolutionReview(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, round uint64) (domain.ReviewPointer, domain.ReviewGuard, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, origin.Unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, origin.Request)</code>；<code>repository.LockedActiveMerge(ctx, tx, origin.Request)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedReviewGuard(ctx, tx, origin.Unit)</code>；<code>repository.LockedReviewPointer(ctx, tx, origin.Request)</code>；<code>repository.LockedActiveMerge(ctx, tx, origin.Request)</code> |
| <code>origin</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>origin.Unit</code>；<code>origin.Request</code>；<code>origin.Document</code>；<code>origin.Branch</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>pointer.Round != round</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>domain.ReviewPointer{}</code>；<code>pointer</code> |
| <code>(匿名/未命名)</code> | <code>domain.ReviewGuard</code> | 使用 <code>domain.ReviewGuard</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>guard</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;REVIEW_WITHDRAW_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOLUTION_REVIEW_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.ReviewPointer{}, guard, err</code>。当前函数在这里结束。
- 检查 <code>guard.Ongoing != 0</code>。条件成立时执行 <code>return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, &quot;REVIEW_WITHDRAW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>guard.Submission != &quot;&quot; &#124;&#124; guard.Resumable != origin.Request</code>。条件成立时执行 <code>return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, &quot;RESOLUTION_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return pointer, guard, err</code>。当前函数在这里结束。
- 检查 <code>pointer.Archived &#124;&#124; pointer.Status != &quot;WITHDRAWN&quot; &#124;&#124; pointer.Round != round &#124;&#124; pointer.Document != origin.Document &#124;&#124; pointer.Unit != origin.Unit &#124;&#124; pointer.Branch != origin.Branch</code>。条件成立时执行 <code>return pointer, guard, domain.Violation(domain.StateConflict, &quot;RESOLUTION_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; active != &quot;&quot;</code>。
- 此处通过 <code>return pointer, guard, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=20133:21327 -->
````go
func resolutionReview(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, round uint64) (domain.ReviewPointer, domain.ReviewGuard, error) {
	guard, err := repository.LockedReviewGuard(ctx, tx, origin.Unit)
	if err != nil {
		return domain.ReviewPointer{}, guard, err
	}
	if guard.Ongoing != 0 {
		return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, "REVIEW_WITHDRAW_REQUIRED")
	}
	if guard.Submission != "" || guard.Resumable != origin.Request {
		return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
	pointer, err := repository.LockedReviewPointer(ctx, tx, origin.Request)
	if err != nil {
		return pointer, guard, err
	}
	if pointer.Archived || pointer.Status != "WITHDRAWN" || pointer.Round != round || pointer.Document != origin.Document || pointer.Unit != origin.Unit || pointer.Branch != origin.Branch {
		return pointer, guard, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
	active, err := repository.LockedActiveMerge(ctx, tx, origin.Request)
	if err == nil && active != "" {
		err = domain.Violation(domain.StateConflict, "MERGE_PENDING")
	}
	return pointer, guard, err
}

````

## 第 15 步：resolutionBody

插入位置：第 14 步代码之后，紧接追加；完成稿约第 494 行。

本步声明：`func resolutionBody(validator *codec.Validator, raw []byte) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator.Body</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code>；<code>domain.BranchBaselineBody(raw)</code>；<code>bytes.Equal(projected, raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; validator.Body(body) != nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(projected, raw)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>blocks&#91;id&#93;</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node&#91;&quot;type&quot;&#93; == &quot;image&quot;</code>。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>e != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=21327:22593 -->
````go
func resolutionBody(validator *codec.Validator, raw []byte) ([]uint64, error) {
	invalid := domain.Violation(domain.InvalidRequest, "RESOLUTION_CONTENT_INVALID")
	body, err := codec.Decode(raw)
	if err != nil || validator.Body(body) != nil {
		return nil, invalid
	}
	projected, err := domain.BranchBaselineBody(raw)
	if err != nil || !bytes.Equal(projected, raw) {
		return nil, invalid
	}
	seen := map[uint64]bool{}
	blocks := map[string]bool{}
	var walk func(any) error
	walk = func(value any) error {
		node, ok := value.(map[string]any)
		if !ok {
			return invalid
		}
		// Schema只验证单节点属性。稳定块ID在整篇唯一，行内图片可重复引用同一资源。
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return invalid
				}
				blocks[id] = true
			}
		}
		if node["type"] == "image" {
			attrs, ok := node["attrs"].(map[string]any)
			if !ok {
				return invalid
			}
			text, ok := attrs["resourceId"].(string)
			if !ok {
				return invalid
			}
			id, e := domain.CanonicalID(text)
			if e != nil {
				return invalid
			}
			seen[id] = true
		}
		children, _ := node["content"].([]any)
		for _, child := range children {
			if e := walk(child); e != nil {
````

## 第 16 步：resolutionBody（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 539 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return e</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ids, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=22593:22860 -->
````go
				return e
			}
		}
		return nil
	}
	if err = walk(body); err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

````

## 第 17 步：resolutionBaseline

插入位置：第 16 步代码之后，紧接追加；完成稿约第 555 行。

从原冲突真实MAIN版本投影当前仍完整的稳定单元；不会把后来MAIN尾部提升成已接纳基线。

本步声明：`func resolutionBaseline(ctx context.Context, tx *sql.Tx, fixed resolutionContext) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, fixed.Versions.Main, fixed.Scope.Document)</code>；<code>projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, fixed.Versions.Main, fixed.Scope.Document)</code>；<code>projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Versions</code>；<code>fixed.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 由 <code>projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=22860:23500 -->
````go
// 从原冲突真实MAIN版本投影当前仍完整的稳定单元；不会把后来MAIN尾部提升成已接纳基线。
func resolutionBaseline(ctx context.Context, tx *sql.Tx, fixed resolutionContext) (*pb.FixedContent, error) {
	baseline, err := repository.VersionContent(ctx, tx, fixed.Versions.Main, fixed.Scope.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(baseline, fixed.Versions.Main); err != nil {
		return nil, err
	}
	return projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)
}

// conflict_resolution_recovery.go：保留原实现，按业务合并文件。

````

## 第 18 步：Resolutions.RecoverOne

插入位置：第 17 步代码之后，紧接追加；完成稿约第 569 行。

本步声明：`func (s *Resolutions) RecoverOne(parent context.Context)`。


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
- 检查 <code>bindErr != nil &#124;&#124; fact.Kind != task.Kind &#124;&#124; !bytes.Equal(fact.Payload, task.Payload)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>task.Kind == &quot;RESOLUTION_RECONCILE&quot;</code>。
- 检查 <code>err == nil</code>。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err == nil</code>。
- 执行 <code>check.Rollback()</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 18 bytes=23500:24811 -->
````go
func (s *Resolutions) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimResolutionTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindErr := repository.ClaimedResolutionBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingResolutionBinding(fact)
	if bindErr != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	if err == nil {
		if task.Kind == "RESOLUTION_RECONCILE" {
			err = s.reconcile(ctx, task)
		} else {
			proofs, resumeErr := s.resume(ctx, fixed)
			err = resumeErr
			if err == nil {
				check, e := s.db.BeginTx(ctx, nil)
				err = e
				if err == nil {
					err = lockResolutionOperations(ctx, check, fixed)
					if err == nil {
						_, err = s.validate(ctx, check, fixed, proofs)
					}
					_ = check.Rollback()
				}
			}
			if err == nil {
				prepared, e := s.compute(ctx, task, fixed)
				err = e
				if err == nil {
					err = s.commit(ctx, task, fixed, prepared, proofs)
				}
			}
			var violation domain.RuleViolation
````

## 第 19 步：Resolutions.RecoverOne（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 614 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.StateConflict &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 19 bytes=24811:25220 -->
````go
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
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

## 第 20 步：Resolutions.reject

插入位置：第 19 步代码之后，紧接追加；完成稿约第 626 行。

本步声明：`func (s *Resolutions) reject(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>lockResolutionOperations(ctx, tx, fixed)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Scope.Document, true)</code>；<code>repository.LockedBranchControl(ctx, tx, fixed.Scope.Workspace.Workspace)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteResolution(ctx, tx, task, &quot;REJECTED&quot;, result, install)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>lockResolutionOperations(ctx, tx, fixed)</code>；<code>fixed.Scope</code>；<code>Fixed: fixed</code>；<code>fixed.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CONTROL_UNKNOWN&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 20 bytes=25220:26741 -->
````go
func (s *Resolutions) reject(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
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
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	install, _ := json.Marshal(resolutionInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: fixed.Intent.Operation}})
	// 无论是否已收到prepare响应，都由原号权威终态驱动Node收尾，不通过超时猜测屏障未建立。
	if err = repository.CompleteResolution(ctx, tx, task, "REJECTED", result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_register.go：保留原实现，按业务合并文件。

````

## 第 21 步：Resolutions.Register

插入位置：第 20 步代码之后，紧接追加；完成稿约第 659 行。

本步声明：`func (s *Resolutions) Register(ctx context.Context, actor Actor, origin domain.MergeFact, intent domain.ConflictResolutionIntent, path string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, origin.Document, &quot;CONFLICT_RESOLUTION&quot;)</code>；<code>repository.ResolutionResult(ctx, s.db, intent.Operation)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, origin.Branch, origin.Document, origin.Unit)</code>；<code>s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)</code><br>另有 22 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, origin.Document, &quot;CONFLICT_RESOLUTION&quot;)</code>；<code>return actor, err</code>；<code>return actor, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>actor.ID</code><br>另有 17 处使用，继续按下方完整代码的顺序处理。 |
| <code>origin</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>origin.Document</code>；<code>origin.Unit</code>；<code>origin.Branch</code>；<code>Origin: origin</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.ConflictResolutionIntent</code> | 使用 <code>domain.ConflictResolutionIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>domain.ConflictResolutionFingerprint(path, intent)</code>；<code>intent.Operation</code>；<code>intent.Body</code>；<code>Intent: intent</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ConflictResolutionFingerprint(path, intent)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;origin.Unit&#93;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>prior.Actor != actor.ID &#124;&#124; !bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>len(bound) == 0</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_IDENTITY_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=26741:28693 -->
````go
func (s *Resolutions) Register(ctx context.Context, actor Actor, origin domain.MergeFact, intent domain.ConflictResolutionIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, origin.Document, "CONFLICT_RESOLUTION")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Edit[origin.Unit] {
		return actor, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	fingerprint, err := domain.ConflictResolutionFingerprint(path, intent)
	if err != nil {
		return actor, err
	}
	if prior, e := repository.ResolutionResult(ctx, s.db, intent.Operation); e == nil {
		if prior.Actor != actor.ID || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	resources, err := resolutionBody(s.validator, intent.Body)
	if err != nil {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, origin.Branch, origin.Document, origin.Unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, err
	}
	target := &pb.Target{DocumentId: origin.Document, UnitId: origin.Unit, BranchId: origin.Branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "CONFLICT_RESOLUTION"}
	var bound []*pb.IdentityAdmission
	bound, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	if len(bound) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_IDENTITY_UNKNOWN")
	}
	access.Proofs = bound
	actor.Proof = bound[0]
	actor.Sources = access.Sources
	fixed := resolutionContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Origin: origin, Access: access}
````

## 第 22 步：Resolutions.Register（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 704 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>prior.ActorID != actor.ID &#124;&#124; prior.Kind != &quot;CONFLICT_RESOLUTION&quot; &#124;&#124; !bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentStructureId != domain.TextID(structure)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; active != origin.Branch</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; actual != workspace</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;WORKSPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 22 bytes=28693:30391 -->
````go
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if prior, e := repository.LockedOperation(ctx, tx, intent.Operation); e == nil {
		if prior.ActorID != actor.ID || prior.Kind != "CONFLICT_RESOLUTION" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
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
	if current.CurrentStructureId != domain.TextID(structure) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, origin.Document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, origin.Unit, origin.Document)
	if err != nil || active != origin.Branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	actual, err := repository.LockedControlledWorkspace(ctx, tx, active, origin.Document, origin.Unit)
	if err != nil || actual != workspace {
		return actor, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	if err = repository.RequireBranchControlOpen(ctx, tx, active); err != nil {
		return actor, err
	}
	fixed.Review, fixed.Guard, err = resolutionReview(ctx, tx, origin, intent.Round)
````

## 第 23 步：Resolutions.Register（接着实现）

插入位置：第 22 步代码之后，紧接追加；完成稿约第 749 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>baseline.StructureId != original.Input.Main.StructureId</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;RESOLUTION_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=30391:32190 -->
````go
	if err != nil {
		return actor, err
	}
	original, versions, err := s.original(ctx, tx, origin, intent, true)
	if err != nil {
		return actor, err
	}
	if err = requireStableReviewRange(ctx, tx, origin.Document, origin.Unit, original.Input.Main.StructureId, structure); err != nil {
		return actor, err
	}
	fixed.Versions = versions
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalRevision(current.ControlRevision)
	room, err := collaborationRoom(ctx, tx, "BRANCH", active, origin.Unit, actual.Epoch)
	if err != nil {
		return actor, err
	}
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, origin.Document)
	if err != nil {
		return actor, err
	}
	fixed.Scope = draftSourceScope{Document: origin.Document, Unit: origin.Unit, Room: room, Manifest: manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: actual}
	fixed.Access.Document = current
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return actor, err
	}
	fixed.BaselineVersion = versions.Main
	if baseline.StructureId != original.Input.Main.StructureId {
		fixed.BaselineVersion, err = repository.InsertMergeVersion(ctx, tx, baseline, "RESTORE", baseline.CaptureId)
		if err != nil {
			return actor, err
		}
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, resource := range resources {
		state, e := repository.LockedDocumentResource(ctx, tx, resource, origin.Document, space)
		if e != nil {
			return actor, e
		}
		if state != "READY" {
			return actor, domain.Violation(domain.StateConflict, "RESOLUTION_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, resource, origin.Document, intent.Operation, 1); err != nil {
````

## 第 24 步：Resolutions.Register（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 794 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 24 bytes=32190:33341 -->
````go
			return actor, err
		}
	}
	command := OperationCommand{ID: intent.Operation, Kind: "CONFLICT_RESOLUTION", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, origin.Document, "CONFLICT_RESOLUTION", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, origin.Unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterResolution(ctx, tx, active, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/conflict_resolutions_execution_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
