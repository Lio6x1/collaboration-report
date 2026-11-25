# 分步编写 `services/document/internal/logic/document/structure_transitions_service.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/241-structure-transitions-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

提交拆合后的结构、MAIN和未审核草稿，并协调旧输出停止及新安装。

主线和草稿的候选来源不同，结构迁移不能把未审核S2伪装为已接纳主线。

写之前先按这条执行路线理解各段的连接：

1. recoverStructure复核确认并准备屏障
2. materializeStructureUnits分配新身份、版本与范围映射
3. commitStructure原子切指针
4. structureInstallationWire交付安装
5. 按实际ACK恢复支线。

本文件需要守住的边界：

- 只有明确编辑器未准备好才能拒绝确认
- 未知RPC继续原号恢复
- 资源与切点必须匹配原预览。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/structure_transitions_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/structure_transitions_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/structure_transitions_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/structure_transitions_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:650 -->
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
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
)

// structure_transition_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControls.commitStructure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func (s *MainControls) commitStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, confirmation structureConfirmation, cut *pb.CutVector, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 17 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, fixed.Style, current.MainLocked, result, install)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Fixed</code> |
| <code>confirmation</code> | <code>structureConfirmation</code> | 在此声明中的实际使用：<code>confirmation.Raw</code> |
| <code>cut</code> | <code>*pb.CutVector</code> | 在此声明中的实际使用：<code>domain.CutBinding(cut)</code>；<code>proto.Equal(prepared.PreparedCut, cut)</code> |
| <code>prepared</code> | <code>*pb.MainBarrierReply</code> | 在此声明中的实际使用：<code>prepared == nil</code>；<code>prepared.State</code>；<code>prepared.OperationId</code>；<code>prepared.BarrierGeneration</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(fixed, proofs)</code>；<code>s.loadStructureConfirmation(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_FIXED_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepared == nil &#124;&#124; prepared.State != pb.MainBarrierReply_PREPARED &#124;&#124; prepared.OperationId != task.Operation &#124;&#124; prepared.BarrierGeneration != fixed.Generation &#124;&#124; !proto.Equal(prepar …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != task.Operation &#124;&#124; barrier.Generation != fixed.Generation &#124;&#124; barrier.Phase != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=650:2620 -->
````go
func (s *MainControls) commitStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, confirmation structureConfirmation, cut *pb.CutVector, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	raw, err := codec.Canonical(domain.CutBinding(cut))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if prepared == nil || prepared.State != pb.MainBarrierReply_PREPARED || prepared.OperationId != task.Operation || prepared.BarrierGeneration != fixed.Generation || !proto.Equal(prepared.PreparedCut, cut) || !bytes.Equal(prepared.Digest, digest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
	// RPC在锁外，最终事务重新核验原号→来源→Doc→屏障→任务→完整预览与审核guard，追加及退役整体提交。
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
	if err = domain.ValidateMainControl(fixed.Access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, fixed.Intent); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return err
	}
	actual, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
````

## 第 3 步：MainControls.commitStructure（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 70 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!bytes.Equal(actual.Raw, confirmation.Raw)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!affected&#91;domain.TextID(unit.Unit)&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2620:4234 -->
````go
	if !bytes.Equal(actual.Raw, confirmation.Raw) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	}
	affected := map[string]bool{}
	for _, change := range actual.Migration.Changes {
		for _, id := range change.Sources {
			affected[id] = true
		}
	}
	for _, unit := range actual.Preview.Units {
		if !affected[domain.TextID(unit.Unit)] {
			continue
		}
		if err = repository.RequireBranchControlOpen(ctx, tx, unit.Workspace.Workspace); err != nil {
			return err
		}
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	contents := append(append([]*pb.FixedContent{}, actual.Snapshot.Main...), actual.Snapshot.Drafts...)
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, contents, "STRUCTURE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	old, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return err
	}
	structure, err := repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, task.Operation)
	if err != nil {
		return err
	}
	identities := map[string]domain.StructureMigrationIdentity{}
	for _, change := range actual.Migration.Changes {
		chapter, _ := domain.CanonicalID(change.Chapter)
		for _, target := range change.Targets {
			unit, err := repository.CreateStructureUnit(ctx, tx, fixed.Document, chapter, structure, target.Kind)
			if err != nil {
				return err
			}
			node, err := NewEntityID()
			if err != nil {
				return err
			}
			identities[target.Key] = domain.StructureMigrationIdentity{Unit: domain.TextID(unit), Node: domain.TextID(node)}
		}
	}
````

## 第 4 步：MainControls.commitStructure（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 115 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>affected&#91;domain.TextID(unit.Unit)&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;REMOVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=4234:6174 -->
````go
	entries, err := domain.ApplyStructureMigration(old.Entries, *fixed.Intent.Structure, actual.Migration, identities)
	if err != nil {
		return err
	}
	if err = repository.InsertStructureEntries(ctx, tx, fixed.Document, structure, entries); err != nil {
		return err
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Manifest, structure, fixed.Style, task.Operation, fixed.Intent.Kind, actual.Preview.Intent.Operation)
	if err != nil {
		return err
	}
	units, transition, err := s.materializeStructureUnits(ctx, tx, fixed, actual, structure, manifest, identities)
	if err != nil {
		return err
	}
	for _, unit := range actual.Preview.Units {
		if affected[domain.TextID(unit.Unit)] {
			if err = repository.RetireStructureUnit(ctx, tx, fixed.Document, unit.Unit, unit.Workspace.Workspace, structure); err != nil {
				return err
			}
		}
	}
	for _, change := range actual.Migration.Changes {
		if change.Type == "REMOVE" {
			chapter, _ := domain.CanonicalID(change.Chapter)
			if err = repository.RetireStructureChapter(ctx, tx, fixed.Document, chapter, structure); err != nil {
				return err
			}
		}
	}
	transition.ACLEpoch, _ = domain.CanonicalID(current.AclRevision)
	if err = repository.AdvanceDocumentACL(ctx, tx, fixed.Document); err != nil {
		return err
	}
	if err = QueueClosure(ctx, tx, task.Operation, fixed.Actor, domain.Closure{Scope: "doc-acl:" + domain.TextID(fixed.Document), Epoch: transition.ACLEpoch, Entity: fixed.Document, Action: "ACL"}); err != nil {
		return err
	}
	manifestDigest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: fixed.Generation, Manifest: manifest, Units: units, Digest: manifestDigest[:], Structure: structure, Style: fixed.Style, Control: fixed.Control + 1, Entity: structure, Transition: transition})
	if err != nil {
		return err
	}
````

## 第 5 步：MainControls.commitStructure（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 160 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=6174:6790 -->
````go
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"manifest_id": domain.TextID(manifest), "entity_id": domain.TextID(structure), "main_sync_state": "SYNCING"}}
	if err = repository.PointToControlledStructure(ctx, tx, fixed.Document, structure); err != nil {
		return err
	}
	if err = repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, fixed.Style, current.MainLocked, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// structure_transition_content.go：保留原实现，按业务合并文件。

````

## 第 6 步：structureCandidateContext

插入位置：第 5 步代码之后，紧接追加；完成稿约第 172 行。

本步声明：`type structureCandidateContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 迁移候选所属文档。 |
| <code>Unit</code> | <code>uint64</code> | 新结构目标单元。 |
| <code>Structure</code> | <code>uint64</code> | 目标结构版本。 |
| <code>Branch</code> | <code>uint64</code> | 目标支线编号。 |
| <code>Epoch</code> | <code>uint64</code> | 目标支线工作区代次。 |
| <code>Lineage</code> | <code>uint64</code> | 目标支线谱系编号。 |
| <code>Manifest</code> | <code>uint64</code> | 新结构主线清单。 |
| <code>BaseVersion</code> | <code>uint64</code> | 新支线基线正文版本。 |
| <code>Operation</code> | <code>string</code> | 本次结构变更操作号。 |
| <code>Preview</code> | <code>string</code> | 原结构预览引用。 |
| <code>Key</code> | <code>string</code> | 迁移结果中此候选的固定键。 |
| <code>Relation</code> | <code>string</code> | 迁移关系种类，用于标记来源范围如何成为目标范围。 |
| <code>SourceVersions</code> | <code>map&#91;string&#93;uint64</code> | 来源单元到已固定正文版本的映射。 |
| <code>Sources</code> | <code>&#91;&#93;*pb.FixedContent</code> | 构造本候选所用的固定来源正文，不能读取变化中的房间重新计算。 |

<!-- write-step: 6 bytes=6790:7180 -->
````go
type structureCandidateContext struct {
	Document, Unit, Structure, Branch, Epoch, Lineage, Manifest, BaseVersion uint64
	Operation, Preview, Key, Relation                                        string
	SourceVersions                                                           map[string]uint64
	Sources                                                                  []*pb.FixedContent
}

````

## 第 7 步：buildStructureCandidate

插入位置：第 6 步代码之后，紧接追加；完成稿约第 179 行。

MAIN和未审核草稿各自构造固定内容。来源仅引用实际固定版本/原预览，不把S2标为已接纳。

本步声明：`func buildStructureCandidate(validator *codec.Validator, raw json.RawMessage, fixed structureCandidateContext) (*pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator.Body</code>；<code>validator.Provenance</code> |
| <code>raw</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>codec.Decode(raw)</code> |
| <code>fixed</code> | <code>structureCandidateContext</code> | 在此声明中的实际使用：<code>fixed.Sources</code>；<code>fixed.SourceVersions</code>；<code>fixed.Relation</code>；<code>fixed.Preview</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>content</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!seenAccepted&#91;string(key)&#93;</code>。
- 在本段的匿名函数内：检查 <code>node&#91;&quot;type&quot;&#93; == &quot;image&quot;</code>。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>err == nil</code>。

<!-- write-step: 7 bytes=7180:8820 -->
````go
// MAIN和未审核草稿各自构造固定内容。来源仅引用实际固定版本/原预览，不把S2标为已接纳。
func buildStructureCandidate(validator *codec.Validator, raw json.RawMessage, fixed structureCandidateContext) (*pb.FixedContent, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	if err = validator.Body(value); err != nil {
		return nil, err
	}
	body, err := codec.Canonical(value)
	if err != nil {
		return nil, err
	}
	origins := map[string]uint64{}
	accepted := []any{}
	seenAccepted := map[string]bool{}
	for _, source := range fixed.Sources {
		sourceBody, err := codec.Decode(source.BodyJsonUtf8)
		if err != nil {
			return nil, err
		}
		visitStructureBlocks(sourceBody, func(id string, _ map[string]any) { origins[id] = fixed.SourceVersions[domain.TextID(source.UnitId)] })
		prior, err := codec.Decode(source.ProvenanceJsonUtf8)
		if err != nil {
			return nil, err
		}
		for _, capture := range prior.(map[string]any)["accepted_captures"].([]any) {
			key, err := codec.Canonical(capture)
			if err != nil {
				return nil, err
			}
			if !seenAccepted[string(key)] {
				seenAccepted[string(key)] = true
				accepted = append(accepted, capture)
			}
		}
	}
	blocks := []any{}
	resources := map[uint64]bool{}
	visitStructureNodes(value, func(node map[string]any) {
		// 图片是无块ID的行内节点；按共享正文schema的resourceId收集，独立于块谱系。
		attrs, _ := node["attrs"].(map[string]any)
		if node["type"] == "image" {
			if resource, ok := attrs["resourceId"].(string); ok {
				if resourceID, err := domain.CanonicalID(resource); err == nil {
````

## 第 8 步：buildStructureCandidate（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 224 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>version != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return content, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=8820:10655 -->
````go
					resources[resourceID] = true
				}
			}
		}
		id, ok := attrs["id"].(string)
		if !ok {
			return
		}
		item := map[string]any{"block_id": id, "origin_block_id": id, "relation": fixed.Relation, "source_range": map[string]any{"start": json.Number("0"), "end": json.Number("0")}}
		if version := origins[id]; version != 0 {
			item["origin_content_version_id"] = domain.TextID(version)
		} else {
			item["origin_capture_ref"] = fixed.Preview
			item["relation"] = "INSERTED"
		}
		blocks = append(blocks, item)
	})
	provenance := map[string]any{"schema_version": "provenance-v1", "lineage_id": domain.TextID(fixed.Lineage), "base_manifest_id": domain.TextID(fixed.Manifest), "base_content_version_id": domain.TextID(fixed.BaseVersion), "source_operation_id": fixed.Operation, "accepted_captures": accepted, "block_sources": blocks}
	if err = validator.Provenance(provenance); err != nil {
		return nil, err
	}
	proof, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	digest := sha256.Sum256(body)
	content := &pb.FixedContent{OperationId: fixed.Operation, DocumentId: fixed.Document, UnitId: fixed.Unit, StructureId: fixed.Structure, BranchId: fixed.Branch, WorkspaceEpoch: fixed.Epoch, CaptureId: fixed.Operation + ":" + fixed.Key, LineageRef: domain.TextID(fixed.Lineage), BodyJsonUtf8: body, BodyHash: digest[:], SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256", ByteSize: uint64(len(body)), ResourceIds: ids, ProvenanceJsonUtf8: proof, ProvenanceSchema: "provenance-v1"}
	if err = domain.ValidateFixedContentResources(value, content); err != nil {
		return nil, err
	}
	return content, nil
}

````

## 第 9 步：visitStructureBlocks

插入位置：第 8 步代码之后，紧接追加；完成稿约第 262 行。

本步声明：`func visitStructureBlocks(value any, visit func(string, map[string]any))`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>visitStructureNodes(value, func(node map&#91;string&#93;any) { 		if attrs, ok := node&#91;&quot;attrs&quot;&#93;.(map&#91;string&#93;any); ok { 			if id, ok := attrs&#91;&quot;id&quot;&#93;.(string); ok { 				visit(id, node) 			} 		} 	})</code> |
| <code>visit</code> | <code>func(string, map&#91;string&#93;any)</code> | 在此声明中的实际使用：<code>visit(id, node)</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>ok</code>。

<!-- write-step: 9 bytes=10655:10920 -->
````go
func visitStructureBlocks(value any, visit func(string, map[string]any)) {
	visitStructureNodes(value, func(node map[string]any) {
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				visit(id, node)
			}
		}
	})
}

````

## 第 10 步：visitStructureNodes

插入位置：第 9 步代码之后，紧接追加；完成稿约第 272 行。

本步声明：`func visitStructureNodes(value any, visit func(map[string]any))`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>node := value.(type)</code> |
| <code>visit</code> | <code>func(map&#91;string&#93;any)</code> | 在此声明中的实际使用：<code>visit(node)</code>；<code>visitStructureNodes(child, visit)</code> |

**跟着本段代码走一遍：**

- 检查 <code>ok</code>。

<!-- write-step: 10 bytes=10920:11359 -->
````go
func visitStructureNodes(value any, visit func(map[string]any)) {
	switch node := value.(type) {
	case map[string]any:
		visit(node)
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				visitStructureNodes(child, visit)
			}
		}
	case []any:
		for _, child := range node {
			visitStructureNodes(child, visit)
		}
	}
}

// structure_transition_install.go：保留原实现，按业务合并文件。

````

## 第 11 步：structureInstallationWire

插入位置：第 10 步代码之后，紧接追加；完成稿约第 290 行。

本步声明：`func structureInstallationWire(fixed *structureInstallation) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>*structureInstallation</code> | 在此声明中的实际使用：<code>fixed == nil</code>；<code>fixed.Branches</code>；<code>fixed.Preview</code>；<code>fixed.Digest</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;preview_cut_ref&quot;: fixed.Preview, &quot;preview_digest&quot;: base64.StdEncoding.EncodeToString(fixed.Digest), &quot;acl_epoch&quot;: domain.TextID(fixed.ACLEpoch), &quot;branches&quot;: branches}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed == nil</code>。条件成立时执行 <code>return nil, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;any{&quot;preview_cut_ref&quot;: fixed.Preview, &quot;preview_digest&quot;: base64.StdEncoding.EncodeToString(fixed.Digest), &quot;acl_epoch&quot;: domain.TextID(fixed.ACLEpoch), &quot;branches&quot;: branches}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=11359:12268 -->
````go
func structureInstallationWire(fixed *structureInstallation) (any, error) {
	if fixed == nil {
		return nil, nil
	}
	branches := make([]any, 0, len(fixed.Branches))
	for _, branch := range fixed.Branches {
		content, err := fixedWire(branch.Content)
		if err != nil {
			return nil, err
		}
		branches = append(branches, map[string]any{"candidate_key": branch.Key, "unit_id": domain.TextID(branch.Unit), "branch_id": domain.TextID(branch.Branch), "room_id": domain.TextID(branch.Room), "epoch": domain.TextID(branch.Epoch), "lineage_id": domain.TextID(branch.Lineage), "content": content, "lineage_context": branch.Context})
	}
	return map[string]any{"preview_cut_ref": fixed.Preview, "preview_digest": base64.StdEncoding.EncodeToString(fixed.Digest), "acl_epoch": domain.TextID(fixed.ACLEpoch), "branches": branches}, nil
}

// structure_transition_readiness.go：保留原实现，按业务合并文件。

````

## 第 12 步：validStructureInstallation

插入位置：第 11 步代码之后，紧接追加；完成稿约第 307 行。

本步声明：`func validStructureInstallation(original mainControlContext, install mainInstallContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>original</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>original.StructurePreview</code>；<code>original.Access</code>；<code>original.Document</code>；<code>original.Intent</code> |
| <code>install</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>install.Transition</code>；<code>install.Units</code>；<code>install.Structure</code>；<code>install.Manifest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>fixed == nil</code>；<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>original.StructurePreview == nil</code>。条件成立时执行 <code>return fixed == nil</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; fixed == nil &#124;&#124; fixed.ACLEpoch != epoch &#124;&#124; fixed.Preview != original.StructurePreview.Reference &#124;&#124; !bytes.Equal(fixed.Digest, original.StructurePreview.Digest)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>unit == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>branch.Key == &quot;&quot; &#124;&#124; keys&#91;branch.Key&#93; &#124;&#124; branch.Unit == 0 &#124;&#124; units&#91;branch.Unit&#93; == 0 &#124;&#124; branch.Branch == 0 &#124;&#124; branch.Room == 0 &#124;&#124; rooms&#91;branch.Room&#93; &#124;&#124; branch.Epoch != 1 &#124;&#124; branch.L …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(raw, content.BodyJsonUtf8) &#124;&#124; !bytes.Equal(digest&#91;:&#93;, content.BodyHash)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(content.CutJsonUtf8, &amp;cut) != nil &#124;&#124; cut&#91;&quot;room_id&quot;&#93; != domain.TextID(branch.Room) &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; != &quot;1&quot; &#124;&#124; cut&#91;&quot;durable_seq&quot;&#93; != &quot;0&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>branch.Context&#91;&quot;lineage_id&quot;&#93; != domain.TextID(branch.Lineage) &#124;&#124; branch.Context&#91;&quot;base_manifest_id&quot;&#93; != domain.TextID(install.Manifest) &#124;&#124; branch.Context&#91;&quot;base_version_id&quot;&#93; != domain.TextID(units&#91;branch.Unit&#93;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=12268:14550 -->
````go
func validStructureInstallation(original mainControlContext, install mainInstallContext) bool {
	fixed := install.Transition
	if original.StructurePreview == nil {
		return fixed == nil
	}
	epoch, err := domain.CanonicalID(original.Access.Document.AclRevision)
	if err != nil || fixed == nil || fixed.ACLEpoch != epoch || fixed.Preview != original.StructurePreview.Reference || !bytes.Equal(fixed.Digest, original.StructurePreview.Digest) {
		return false
	}
	units := map[uint64]uint64{}
	rooms, keys := map[uint64]bool{}, map[string]bool{}
	for _, unit := range install.Units {
		if unit == nil {
			return false
		}
		units[unit.UnitId] = unit.ContentVersionId
	}
	for _, branch := range fixed.Branches {
		content := branch.Content
		if branch.Key == "" || keys[branch.Key] || branch.Unit == 0 || units[branch.Unit] == 0 || branch.Branch == 0 || branch.Room == 0 || rooms[branch.Room] || branch.Epoch != 1 || branch.Lineage == 0 || content == nil || content.DocumentId != original.Document || content.UnitId != branch.Unit || content.BranchId != branch.Branch || content.WorkspaceEpoch != branch.Epoch || content.StructureId != install.Structure || content.OperationId != original.Intent.Operation || content.LineageRef != domain.TextID(branch.Lineage) || content.HashAlgorithm != "SHA-256" || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
			return false
		}
		body, err := codec.Decode(content.BodyJsonUtf8)
		if err != nil {
			return false
		}
		raw, err := codec.Canonical(body)
		digest := sha256.Sum256(raw)
		if err != nil || !bytes.Equal(raw, content.BodyJsonUtf8) || !bytes.Equal(digest[:], content.BodyHash) {
			return false
		}
		var cut map[string]string
		if json.Unmarshal(content.CutJsonUtf8, &cut) != nil || cut["room_id"] != domain.TextID(branch.Room) || cut["workspace_epoch"] != "1" || cut["durable_seq"] != "0" {
			return false
		}
		if branch.Context["lineage_id"] != domain.TextID(branch.Lineage) || branch.Context["base_manifest_id"] != domain.TextID(install.Manifest) || branch.Context["base_version_id"] != domain.TextID(units[branch.Unit]) {
			return false
		}
		encoded, err := json.Marshal(branch.Context["provenance"])
		if err != nil {
			return false
		}
		decoded, err := codec.Decode(encoded)
		if err != nil {
			return false
		}
````

## 第 13 步：validStructureInstallation（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 353 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !bytes.Equal(proof, content.ProvenanceJsonUtf8)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=14550:14835 -->
````go
		proof, err := codec.Canonical(decoded)
		if err != nil || !bytes.Equal(proof, content.ProvenanceJsonUtf8) {
			return false
		}
		rooms[branch.Room] = true
		keys[branch.Key] = true
	}
	return true
}

// structure_transition_recovery.go：保留原实现，按业务合并文件。

````

## 第 14 步：structurePreparationError

插入位置：第 13 步代码之后，紧接追加；完成稿约第 365 行。

协调服务已确定未准备完成的编辑器才拒绝本次确认；断连和未知RPC仍沿原号恢复，不能误判为安全未提交。

本步声明：`func structurePreparationError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>status.Code(err)</code>；<code>status.Convert(err)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_EDITORS_NOT_READY&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>status.Code(err) == codes.FailedPrecondition &amp;&amp; status.Convert(err).Message() == &quot;STRUCTURE_EDITORS_NOT_READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_EDITORS_NOT_READY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14835:15247 -->
````go
// 协调服务已确定未准备完成的编辑器才拒绝本次确认；断连和未知RPC仍沿原号恢复，不能误判为安全未提交。
func structurePreparationError(err error) error {
	if status.Code(err) == codes.FailedPrecondition && status.Convert(err).Message() == "STRUCTURE_EDITORS_NOT_READY" {
		return domain.Violation(domain.StateConflict, "STRUCTURE_EDITORS_NOT_READY")
	}
	return err
}

````

## 第 15 步：MainControls.recoverStructure

插入位置：第 14 步代码之后，紧接追加；完成稿约第 373 行。

本步声明：`func (s *MainControls) recoverStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Fixed</code>；<code>s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.loadStructureConfirmation(ctx, tx, fixed, proofs)</code>；<code>s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>structurePreparationError(err)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return structurePreparationError(err)</code>。当前函数在这里结束。
- 检查 <code>prepared == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State == pb.MainBarrierReply_CHANGED</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State != pb.MainBarrierReply_PREPARED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=15247:16890 -->
````go
func (s *MainControls) recoverStructure(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	if _, err = ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	confirmation, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	cut, err := structureConfirmationCut(confirmation)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// 确认只消费原预览；CHANGED拒绝该原号，不能沿原号采新cut或推进attempt替换用户确认对象。
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, ExpectedCut: cut})
	if err != nil {
		return structurePreparationError(err)
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CHANGED")
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commitStructure(ctx, task, recovery, confirmation, cut, prepared, proofs)
}

// structure_transition_units.go：保留原实现，按业务合并文件。

````

## 第 16 步：structureInstalledBranch

插入位置：第 15 步代码之后，紧接追加；完成稿约第 419 行。

本步声明：`type structureInstalledBranch struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;candidate_key&quot;`</code> |
| <code>Unit</code> | <code>uint64</code> | 安装后的目标单元。 |
| <code>Branch</code> | <code>uint64</code> | 目标共同支线。 |
| <code>Room</code> | <code>uint64</code> | 安装后用于编辑的新房间。 |
| <code>Epoch</code> | <code>uint64</code> | 新工作区代次，旧连接必须切换。 |
| <code>Lineage</code> | <code>uint64</code> | 安装后绑定的新谱系。 |
| <code>Content</code> | <code>*pb.FixedContent</code> | 序列化/校验标记：<code>`json:&quot;content&quot;`</code> |
| <code>Context</code> | <code>map&#91;string&#93;any</code> | 序列化/校验标记：<code>`json:&quot;lineage_context&quot;`</code> |

<!-- write-step: 16 bytes=16890:17188 -->
````go
type structureInstalledBranch struct {
	Key                                string `json:"candidate_key"`
	Unit, Branch, Room, Epoch, Lineage uint64
	Content                            *pb.FixedContent `json:"content"`
	Context                            map[string]any   `json:"lineage_context"`
}
````

## 第 17 步：structureInstallation

插入位置：第 16 步代码之后，紧接追加；完成稿约第 425 行。

本步声明：`type structureInstallation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Preview</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;preview&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;preview_digest&quot;`</code> |
| <code>ACLEpoch</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;acl_epoch&quot;`</code> |
| <code>Branches</code> | <code>&#91;&#93;structureInstalledBranch</code> | 序列化/校验标记：<code>`json:&quot;branches&quot;`</code> |

<!-- write-step: 17 bytes=17188:17453 -->
````go
type structureInstallation struct {
	Preview  string                     `json:"preview"`
	Digest   []byte                     `json:"preview_digest"`
	ACLEpoch uint64                     `json:"acl_epoch"`
	Branches []structureInstalledBranch `json:"branches"`
}

````

## 第 18 步：MainControls.materializeStructureUnits

插入位置：第 17 步代码之后，紧接追加；完成稿约第 432 行。

本步声明：`func (s *MainControls) materializeStructureUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, confirmation structureConfirmation, structure, manifest uint64, identities map[string]domain.StructureMigrationIdentity) ([]*pb.ManifestUnit, *structureInstallation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, &quot;structure-source:&quot;+id)</code>；<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, &quot;structure-stable:&quot;+domain.TextID(content.UnitId))</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, content.UnitId, version, fixed.Control+1, content)</code>；<code>repository.InsertMergeVersion(ctx, tx, accepted, &quot;STRUCTURE&quot;, target.Key)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, &quot;structure-source:&quot;+id)</code>；<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, &quot;structure-stable:&quot;+domain.TextID(content.UnitId))</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, content.UnitId, version, fixed.Control+1, content)</code>；<code>repository.InsertMergeVersion(ctx, tx, accepted, &quot;STRUCTURE&quot;, target.Key)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.Document</code>；<code>fixed.Control</code>；<code>fixed.Structure</code> |
| <code>confirmation</code> | <code>structureConfirmation</code> | 在此声明中的实际使用：<code>confirmation.Preview</code>；<code>confirmation.Digest</code>；<code>confirmation.Snapshot</code>；<code>confirmation.Migration</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.StructureId = structure</code>；<code>Structure: structure</code>；<code>domain.TextID(structure)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, content.UnitId, version, fixed.Control+1, content)</code>；<code>Manifest: manifest</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, fixed.Control+1, accepted)</code>；<code>repository.InsertInitialLineage(ctx, tx, fixed.Document, unit, branch, manifest, fixed.Intent.Operation, domain.InitialContent{Version: version, Lineage: lineage, Provenance: draft.ProvenanceJsonUtf8})</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>identities</code> | <code>map&#91;string&#93;domain.StructureMigrationIdentity</code> | 在此声明中的实际使用：<code>identities&#91;target.Key&#93;.Unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>*structureInstallation</code> | 实际返回表达式：<code>nil</code>；<code>transition</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!affected&#91;id&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>affected&#91;domain.TextID(original.UnitId)&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=17453:19418 -->
````go
func (s *MainControls) materializeStructureUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, confirmation structureConfirmation, structure, manifest uint64, identities map[string]domain.StructureMigrationIdentity) ([]*pb.ManifestUnit, *structureInstallation, error) {
	units := []*pb.ManifestUnit{}
	transition := &structureInstallation{Preview: confirmation.Preview.Intent.Operation, Digest: confirmation.Digest, Branches: []structureInstalledBranch{}}
	main := map[string]*pb.FixedContent{}
	affected := map[string]bool{}
	for _, content := range confirmation.Snapshot.Main {
		main[domain.TextID(content.UnitId)] = content
	}
	for _, change := range confirmation.Migration.Changes {
		for _, id := range change.Sources {
			affected[id] = true
		}
	}
	// 把受影响整份可靠MAIN留为不可变来源版本，范围边只能引用这些实际落库事实。
	sources := map[string]uint64{}
	for _, original := range confirmation.Snapshot.Main {
		id := domain.TextID(original.UnitId)
		if !affected[id] {
			continue
		}
		content := proto.Clone(original).(*pb.FixedContent)
		content.OperationId = fixed.Intent.Operation
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", "structure-source:"+id)
		if err != nil {
			return nil, nil, err
		}
		sources[id] = version
	}
	for _, original := range confirmation.Snapshot.Main {
		if affected[domain.TextID(original.UnitId)] {
			continue
		}
		content := proto.Clone(original).(*pb.FixedContent)
		content.OperationId = fixed.Intent.Operation
		content.StructureId = structure
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", "structure-stable:"+domain.TextID(content.UnitId))
		if err != nil {
			return nil, nil, err
		}
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, content.UnitId, version, fixed.Control+1, content)
		if err != nil {
			return nil, nil, err
		}
		units = append(units, installed)
	}
````

## 第 19 步：MainControls.materializeStructureUnits（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 477 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>change.Type == &quot;JOIN_CHAPTER&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=19418:21381 -->
````go
	for _, change := range confirmation.Migration.Changes {
		sourceContent := []*pb.FixedContent{}
		for _, id := range change.Sources {
			sourceContent = append(sourceContent, main[id])
		}
		relation := "SPLIT"
		if change.Type == "JOIN_CHAPTER" {
			relation = "JOINED"
		}
		for _, target := range change.Targets {
			unit, _ := domain.CanonicalID(identities[target.Key].Unit)
			lineage, err := NewEntityID()
			if err != nil {
				return nil, nil, err
			}
			candidate := structureCandidateContext{Document: fixed.Document, Unit: unit, Structure: structure, Epoch: fixed.Control + 1, Lineage: lineage, Manifest: manifest, BaseVersion: sources[change.Sources[0]], Operation: fixed.Intent.Operation, Preview: confirmation.Preview.Intent.Operation, Key: target.Key, Relation: relation, SourceVersions: sources, Sources: sourceContent}
			accepted, err := buildStructureCandidate(s.validator, target.Main, candidate)
			if err != nil {
				return nil, nil, err
			}
			version, err := repository.InsertMergeVersion(ctx, tx, accepted, "STRUCTURE", target.Key)
			if err != nil {
				return nil, nil, err
			}
			installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, fixed.Control+1, accepted)
			if err != nil {
				return nil, nil, err
			}
			units = append(units, installed)
			branch, err := repository.CreateInitialBranch(ctx, tx, fixed.Document, unit, version)
			if err != nil {
				return nil, nil, err
			}
			candidate.Branch, candidate.Epoch, candidate.BaseVersion = branch, 1, version
			draft, err := buildStructureCandidate(s.validator, target.Draft, candidate)
			if err != nil {
				return nil, nil, err
			}
			room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, 1)
			if err != nil {
				return nil, nil, err
			}
			draft.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": "1", "durable_seq": "0"})
			if err != nil {
				return nil, nil, err
			}
````

## 第 20 步：MainControls.materializeStructureUnits（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 523 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return units, transition, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=21381:24394 -->
````go
			if err = repository.InsertInitialLineage(ctx, tx, fixed.Document, unit, branch, manifest, fixed.Intent.Operation, domain.InitialContent{Version: version, Lineage: lineage, Provenance: draft.ProvenanceJsonUtf8}); err != nil {
				return nil, nil, err
			}
			if err = repository.RestoreStructureBranch(ctx, tx, branch, lineage); err != nil {
				return nil, nil, err
			}
			if err = repository.BindInitialBranch(ctx, tx, fixed.Document, unit, branch); err != nil {
				return nil, nil, err
			}
			for _, resource := range draft.ResourceIds {
				if err = repository.InsertDraftResourceRef(ctx, tx, resource, fixed.Document, domain.TextID(room)+":1", 1); err != nil {
					return nil, nil, err
				}
			}
			branchBaseline, err := domain.BranchBaselineBody(accepted.BodyJsonUtf8)
			if err != nil {
				return nil, nil, err
			}
			baseline, err := codec.Decode(branchBaseline)
			if err != nil {
				return nil, nil, err
			}
			provenance, err := codec.Decode(draft.ProvenanceJsonUtf8)
			if err != nil {
				return nil, nil, err
			}
			transition.Branches = append(transition.Branches, structureInstalledBranch{Key: target.Key, Unit: unit, Branch: branch, Room: room, Epoch: 1, Lineage: lineage, Content: draft, Context: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(version), "body": baseline, "provenance": provenance}})
			for _, rangeProof := range target.Ranges {
				source, _ := domain.CanonicalID(rangeProof.Source)
				mapping := map[string]any{"schema_version": "full-range-v1", "type": change.Type, "chapter_id": change.Chapter, "operation_id": fixed.Intent.Operation, "source_unit_id": rangeProof.Source, "target_unit_id": domain.TextID(unit), "source_version_id": domain.TextID(sources[rangeProof.Source]), "target_version_id": domain.TextID(version), "source_structure_id": domain.TextID(fixed.Structure), "target_structure_id": domain.TextID(structure), "source_body_hash": main[rangeProof.Source].BodyHash, "target_body_hash": accepted.BodyHash, "source_scope": rangeProof.Scope, "start_boundary": rangeProof.Start, "end_boundary": rangeProof.End, "main_block_ids": rangeProof.MainBlocks, "draft_block_ids": rangeProof.DraftBlocks, "boundary": target.Boundary, "target_kind": target.Kind, "preview_cut_ref": confirmation.Preview.Intent.Operation, "preview_digest": confirmation.Digest}
				mapping["target_key"] = target.Key
				mapping["target_scope"] = rangeProof.TargetScope
				mapping["target_start_boundary"] = rangeProof.TargetStart
				mapping["target_end_boundary"] = rangeProof.TargetEnd
				raw, err := json.Marshal(mapping)
				if err != nil {
					return nil, nil, err
				}
				if err = repository.InsertUnitLineage(ctx, tx, fixed.Intent.Operation, fixed.Document, source, unit, sources[rangeProof.Source], version, raw); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	return units, transition, nil
}

// structure_unit_boundaries.go：保留原实现，按业务合并文件。

````

## 第 21 步：readStructureUnitBoundaries

插入位置：第 20 步代码之后，紧接追加；完成稿约第 572 行。

本步声明：`func readStructureUnitBoundaries(ctx context.Context, tx *sql.Tx, document uint64, entries []dto.StructureEntry) ([]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructureUnitMappings(ctx, tx, document, unit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.StructureUnitMappings(ctx, tx, document, unit)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.StructureUnitMappings(ctx, tx, document, unit)</code> |
| <code>entries</code> | <code>&#91;&#93;dto.StructureEntry</code> | 固定目录条目列表，从中读取各单元标题范围和章节归属，保持结构顺序。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_BOUNDARY_UNKNOWN&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>entry.ParentNodeId == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>entry.UnitId == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_BOUNDARY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(mappings) != 1 &#124;&#124; mappings&#91;0&#93;.Schema != &quot;full-range-v1&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_BOUNDARY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(item.Mapping, &amp;fixed) != nil &#124;&#124; fixed.Schema != item.Schema &#124;&#124; fixed.Type != &quot;SPLIT_LEVEL2&quot; &#124;&#124; fixed.Kind != entry.Kind &#124;&#124; fixed.Source != domain.TextID(item.Source) …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_BOUNDARY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=24394:26579 -->
````go
func readStructureUnitBoundaries(ctx context.Context, tx *sql.Tx, document uint64, entries []dto.StructureEntry) ([]any, error) {
	result := []any{}
	for _, entry := range entries {
		if entry.ParentNodeId == nil {
			continue
		}
		if entry.UnitId == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		unit, _ := domain.CanonicalID(*entry.UnitId)
		mappings, err := repository.StructureUnitMappings(ctx, tx, document, unit)
		if err != nil {
			return nil, err
		}
		if len(mappings) != 1 || mappings[0].Schema != "full-range-v1" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		item := mappings[0]
		var fixed struct {
			Schema        string                            `json:"schema_version"`
			Type          string                            `json:"type"`
			Kind          string                            `json:"target_kind"`
			Source        string                            `json:"source_unit_id"`
			Target        string                            `json:"target_unit_id"`
			SourceVersion string                            `json:"source_version_id"`
			TargetVersion string                            `json:"target_version_id"`
			SourceHash    []byte                            `json:"source_body_hash"`
			TargetHash    []byte                            `json:"target_body_hash"`
			Boundary      domain.StructureMigrationBoundary `json:"boundary"`
		}
		if json.Unmarshal(item.Mapping, &fixed) != nil || fixed.Schema != item.Schema || fixed.Type != "SPLIT_LEVEL2" || fixed.Kind != entry.Kind || fixed.Source != domain.TextID(item.Source) || fixed.Target != *entry.UnitId || fixed.SourceVersion != domain.TextID(item.SourceVersion) || fixed.TargetVersion != domain.TextID(item.TargetVersion) || !bytes.Equal(fixed.SourceHash, item.SourceHash) || !bytes.Equal(fixed.TargetHash, item.TargetHash) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_BOUNDARY_UNKNOWN")
		}
		result = append(result, map[string]any{"unit_id": *entry.UnitId, "heading": fixed.Boundary.Heading, "empty_block_id": fixed.Boundary.EmptyBlock})
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/structure_transitions_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
