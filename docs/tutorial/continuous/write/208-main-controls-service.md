# 分步编写 `services/document/internal/logic/document/main_controls_service.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/208-main-controls-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。

控制变更必须基于可靠MAIN固定切面，并保护文档指针与协作房间同时向同一版本推进。

写之前先按这条执行路线理解各段的连接：

1. Prepare固定意图与门闩generation
2. captureMain取得可靠内容
3. recoverControl准备屏障
4. commit追加清单/样式并保存安装
5. reconcile核安装ACK后收尾。

本文件需要守住的边界：

- MAIN_CUT_CHANGED可以推进捕获attempt但不能换业务原号
- 未知ACK保留待恢复状态
- 错误日志只保留稳定码。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/main_controls_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/main_controls_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/main_controls_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/main_controls_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:687 -->
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
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// main_control_commit.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControls.commit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (s *MainControls) commit(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, main *pb.MainCaptureReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, style, locked, result, install)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Fixed</code>；<code>recovery.Attempt</code> |
| <code>main</code> | <code>*pb.MainCaptureReply</code> | 在此声明中的实际使用：<code>main.Cut</code>；<code>main.Units</code> |
| <code>prepared</code> | <code>*pb.MainBarrierReply</code> | 在此声明中的实际使用：<code>prepared == nil</code>；<code>prepared.State</code>；<code>prepared.OperationId</code>；<code>prepared.BarrierGeneration</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepared == nil &#124;&#124; 		prepared.State != pb.MainBarrierReply_PREPARED &#124;&#124; 		prepared.OperationId != task.Operation &#124;&#124; 		prepared.BarrierGeneration != fixed.Generation &#124;&#124; 		!proto.Equa …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(fixed.Manifest) &#124;&#124; 		current.CurrentStructureId != domain.TextID(fixed.Structure) &#124;&#124; 		current.CurrentStyleId != domain.TextID(fixed.Style)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != task.Operation &#124;&#124; barrier.Generation != fixed.Generation &#124;&#124; barrier.Phase != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=687:2621 -->
````go
func (s *MainControls) commit(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, main *pb.MainCaptureReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	cut, err := codec.Canonical(domain.CutBinding(main.Cut))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(cut)
	if prepared == nil ||
		prepared.State != pb.MainBarrierReply_PREPARED ||
		prepared.OperationId != task.Operation ||
		prepared.BarrierGeneration != fixed.Generation ||
		!proto.Equal(prepared.PreparedCut, main.Cut) ||
		!bytes.Equal(prepared.Digest, digest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
	// RPC已在锁外完成。最终事务按operation→来源→Doc→屏障→worker复核，任何一步失败整体回滚。
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
	if current.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Structure) ||
		current.CurrentStyleId != domain.TextID(fixed.Style) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
````

## 第 3 步：MainControls.commit（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 73 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(entries) != len(main.Units)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!structureManifestUnitSetMatches(sourceStructure, entries)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;STYLE_CHANGE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;MAIN_LOCK&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2621:4419 -->
````go
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return err
	}
	entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, fixed.Document)
	if err != nil {
		return err
	}
	if len(entries) != len(main.Units) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	sourceStructure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return err
	}
	if !structureManifestUnitSetMatches(sourceStructure, entries) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	captured := map[uint64]*pb.FixedContent{}
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, content := range main.Units {
		captured[content.UnitId] = content
	}
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, main.Units, "MERGE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	style := fixed.Style
	locked := current.MainLocked
	// 样式与清单均追加新修订；历史轮次保留原指针，可靠MAIN尾部与新控制指针在同一事务切换。
	if fixed.Intent.Kind == "STYLE_CHANGE" {
		style, err = repository.InsertControlledStyle(ctx, tx, fixed.Document, fixed.Style, task.Operation, fixed.Intent.StyleJSON)
		if err != nil {
			return err
		}
	} else if fixed.Intent.Kind == "MAIN_LOCK" {
		locked = fixed.Intent.Locked
	}
	layout, err := prepareControlledLayout(ctx, tx, current, fixed)
	if err != nil {
		return err
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Manifest, layout.Structure, style, task.Operation, fixed.Intent.Kind, controlAttemptKey(task.Operation, recovery.Attempt))
	if err != nil {
````

## 第 4 步：MainControls.commit（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 118 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>layout.Chapter != 0</code>。
- 检查 <code>fixed.Intent.Kind == &quot;STRUCTURE_CHANGE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 4 bytes=4419:5749 -->
````go
		return err
	}
	control := fixed.Control + 1
	units, err := materializeControlledUnits(ctx, tx, fixed, layout, entries, captured, manifest, control)
	if err != nil {
		return err
	}
	entity := style
	if layout.Chapter != 0 {
		entity = layout.Chapter
	}
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" {
		entity = layout.Structure
	}
	manifestDigest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{
		Context: fixed.Context, Generation: fixed.Generation,
		Manifest: manifest, Units: units, Digest: manifestDigest[:],
		Structure: layout.Structure, Style: style, Control: control, Entity: entity,
	})
	if err != nil {
		return err
	}
	result := map[string]any{
		"operation_id": task.Operation, "state": "SUCCEEDED",
		"result": map[string]string{
			"manifest_id": domain.TextID(manifest), "entity_id": domain.TextID(entity),
			"main_sync_state": "SYNCING",
		},
	}
	if err = repository.PointToControlledStructure(ctx, tx, fixed.Document, layout.Structure); err != nil {
		return err
	}
	if err = repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, style, locked, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_compute.go：保留原实现，按业务合并文件。

````

## 第 5 步：controlAttemptKey

插入位置：第 4 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func controlAttemptKey(operation string, attempt uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation + &quot;:&quot;</code> |
| <code>attempt</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(attempt)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>operation + &quot;:&quot; + domain.TextID(attempt)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return operation + &quot;:&quot; + domain.TextID(attempt)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=5749:5867 -->
````go
func controlAttemptKey(operation string, attempt uint64) string {
	return operation + ":" + domain.TextID(attempt)
}

````

## 第 6 步：MainControls.captureMain

插入位置：第 5 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func (s *MainControls) captureMain(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) (*pb.MainCaptureReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;MAIN_CONTROL_MAIN_FIXED&quot;, key)</code>；<code>s.client.CaptureMain(ctx, input)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Fixed</code>；<code>recovery.Attempt</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.MainCaptureReply</code> | 实际返回表达式：<code>nil</code>；<code>&amp;main</code>；<code>main</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNKNOWN&quot;)</code>；<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_BINDING_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(raw, &amp;main) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;main, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>main == nil &#124;&#124; 		main.State != pb.CaptureReply_FIXED &#124;&#124; 		main.Cut == nil &#124;&#124; 		main.Cut.DocumentId != fixed.Document &#124;&#124; 		main.Cut.StructureId != fixed.Structure &#124;&#124; 		main.Cut.Styl …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>content == nil &#124;&#124; 			content.UnitId == 0 &#124;&#124; 			seen&#91;content.UnitId&#93; &#124;&#124; 			content.BranchId != 0 &#124;&#124; 			content.WorkspaceEpoch != fixed.Control &#124;&#124; 			content.OperationId != task.Operation</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=5867:7727 -->
````go
func (s *MainControls) captureMain(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) (*pb.MainCaptureReply, error) {
	fixed := recovery.Fixed
	key := controlAttemptKey(task.Operation, recovery.Attempt)
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_CONTROL_MAIN_FIXED", key)
	if err == nil {
		var main pb.MainCaptureReply
		if json.Unmarshal(raw, &main) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
		}
		return &main, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	input := &pb.CaptureMainRequest{Context: fixed.Context, ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure, ExpectedStyleId: fixed.Style, ExpectedControlRevision: fixed.Control, Usage: "MAIN_CONTROL:" + domain.TextID(recovery.Attempt)}
	main, err := s.client.CaptureMain(ctx, input)
	if err != nil {
		return nil, err
	}
	if main == nil ||
		main.State != pb.CaptureReply_FIXED ||
		main.Cut == nil ||
		main.Cut.DocumentId != fixed.Document ||
		main.Cut.StructureId != fixed.Structure ||
		main.Cut.StyleId != fixed.Style ||
		main.Cut.ControlRevision != fixed.Control {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
	}
	if err = validateMainCaptureDigest(main); err != nil {
		return nil, err
	}
	seen := map[uint64]bool{}
	for _, content := range main.Units {
		if content == nil ||
			content.UnitId == 0 ||
			seen[content.UnitId] ||
			content.BranchId != 0 ||
			content.WorkspaceEpoch != fixed.Control ||
			content.OperationId != task.Operation {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		seen[content.UnitId] = true
		if err = validateFixedMergeContent(s.validator, content, fixed.Document, content.UnitId, fixed.Structure); err != nil {
			return nil, err
		}
	}
````

## 第 7 步：MainControls.captureMain（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 212 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(prior, &amp;known) != nil &#124;&#124; !bytes.Equal(known.ResultDigest, main.ResultDigest)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CAPTURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return main, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 7 bytes=7727:8785 -->
````go
	payload, err := json.Marshal(main)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return nil, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return nil, err
	}
	if prior, err := repository.MergeArtifact(ctx, tx, "MAIN_CONTROL_MAIN_FIXED", key); err == nil {
		// 另一领取代次若已固定，则不能以这次RPC结果覆盖；必须取得完全相同的原输入。
		var known pb.MainCaptureReply
		if json.Unmarshal(prior, &known) != nil || !bytes.Equal(known.ResultDigest, main.ResultDigest) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		main = &known
	} else if err != sql.ErrNoRows {
		return nil, err
	} else if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_MAIN_FIXED", key, task.Operation, payload, "SUCCEEDED"); err != nil {
		return nil, err
	}
	return main, tx.Commit()
}

````

## 第 8 步：MainControls.advanceAttempt

插入位置：第 7 步代码之后，紧接追加；完成稿约第 242 行。

本步声明：`func (s *MainControls) advanceAttempt(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.AdvanceMainControlAttempt(ctx, tx, task, raw)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.AdvanceMainControlAttempt(ctx, tx, task, raw)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Attempt</code>；<code>json.Marshal(recovery)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CUT_CHANGED_RETRY&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CUT_CHANGED_RETRY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=8785:9573 -->
````go
func (s *MainControls) advanceAttempt(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error {
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
	recovery.Attempt++
	raw, err := json.Marshal(recovery)
	if err != nil {
		return err
	}
	if err = repository.AdvanceMainControlAttempt(ctx, tx, task, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CUT_CHANGED_RETRY")
}

// main_control_install.go：保留原实现，按业务合并文件。

````

## 第 9 步：MainControls.CommittedSource

插入位置：第 8 步代码之后，紧接追加；完成稿约第 270 行。

本步声明：`func (s *MainControls) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;MAIN_COMMIT_FIXED&quot;, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>query.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{ 		&quot;generation&quot;: domain.TextID(fixed.Generation), &quot;manifest_id&quot;: domain.TextID(fixed.Manifest), 		&quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;: domain. …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_NOT_TERMINAL&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>result.State != pb.OperationResult_SUCCEEDED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_NOT_TERMINAL&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_COMMIT_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;any{ 		&quot;generation&quot;: domain.TextID(fixed.Generation), &quot;manifest_id&quot;: domain.TextID(fixed.Manifest), 		&quot;structure_id&quot;: domain.TextID(fixed.Structure), &quot;style_id&quot;:  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=9573:11067 -->
````go
func (s *MainControls) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
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
	transition, err := structureInstallationWire(fixed.Transition)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"generation": domain.TextID(fixed.Generation), "manifest_id": domain.TextID(fixed.Manifest),
		"structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style),
		"control_revision": domain.TextID(fixed.Control), "manifest_digest": base64.StdEncoding.EncodeToString(fixed.Digest),
		"units": units, "lineage": nil, "structure_transition": transition,
	}, nil
}

````

## 第 10 步：MainControls.reconcile

插入位置：第 9 步代码之后，紧接追加；完成稿约第 306 行。

本步声明：`func (s *MainControls) reconcile(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, &amp;pb.OperationQuery{OperationId: task.Operation, Target: fixed.Context.Target, RequestFingerprint: fixed.Context.RequestFingerprint})</code>；<code>s.validateControlInstallation(ctx, fixed)</code>；<code>s.client.ReconcileMain(ctx, &amp;pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, Man …</code>（长表达式，按本步完整代码继续）；<code>s.db.BeginTx(ctx, nil)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_INSTALL_RESULT_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RECONCILE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != pb.OperationResult_SUCCEEDED &amp;&amp; result.State != pb.OperationResult_REJECTED &amp;&amp; result.State != pb.OperationResult_CANCELLED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Context.OperationId != task.Operation &#124;&#124; !validMainInstallReply(fixed, result.State == pb.OperationResult_SUCCEEDED, reply)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_INSTALL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fixed.Transition != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!done</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_OLD_OUTPUTS_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=11067:13560 -->
````go
func (s *MainControls) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	// 这是已固定终态的系统收尾，不依赖原用户在线或旧短期准入；Node须再次查询Go权威结果。
	var fixed mainInstallContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RECONCILE_BINDING_UNKNOWN")
	}
	result, err := s.OperationResult(ctx, &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Context.Target, RequestFingerprint: fixed.Context.RequestFingerprint})
	if err != nil {
		return err
	}
	if result.State != pb.OperationResult_SUCCEEDED && result.State != pb.OperationResult_REJECTED && result.State != pb.OperationResult_CANCELLED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RESULT_UNKNOWN")
	}
	if result.State == pb.OperationResult_SUCCEEDED {
		// 在调用Node前先验证实际清单完整性，避免错误的空候选使Node退役仍存在的MAIN房间。
		if err = s.validateControlInstallation(ctx, fixed); err != nil {
			return err
		}
	}
	reply, err := s.client.ReconcileMain(ctx, &pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, ManifestDigest: fixed.Digest, Units: fixed.Units})
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
	if fixed.Transition != nil {
		scope := "doc-acl:" + domain.TextID(fixed.Context.Target.DocumentId)
		fence := &pb.FenceRequest{RevocationId: task.Operation, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope, fixed.Transition.ACLEpoch), ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: fixed.Transition.ACLEpoch}}}
		done, err := repository.StructureAuthorizationComplete(ctx, tx, task.Operation, fixed.Context.Target.DocumentId, fixed.Transition.ACLEpoch, platform.BootstrapDigest(fence, "edge"), platform.BootstrapDigest(fence, "collaboration"))
		if err != nil {
			return err
		}
		if !done {
````

## 第 11 步：MainControls.reconcile（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 351 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_OLD_OUTPUTS_PENDING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 11 bytes=13560:14202 -->
````go
			return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_OLD_OUTPUTS_PENDING")
		}
		branches := make([]uint64, 0, len(fixed.Transition.Branches))
		for _, branch := range fixed.Transition.Branches {
			branches = append(branches, branch.Branch)
		}
		if err = repository.CompleteStructureBranches(ctx, tx, fixed.Context.Target.DocumentId, branches); err != nil {
			return err
		}
	}
	if err = repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_prepare.go：保留原实现，按业务合并文件。

````

## 第 12 步：mainControlPending

插入位置：第 11 步代码之后，紧接追加；完成稿约第 369 行。

本步声明：`func mainControlPending(operation string) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: operation</code>；<code>&quot;/v1/operations/&quot; + operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + operation}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + operation}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=14202:14379 -->
````go
func mainControlPending(operation string) map[string]any {
	return map[string]any{"operation_id": operation, "state": "PREPARING", "poll_url": "/v1/operations/" + operation}
}

````

## 第 13 步：MainControls.Prepare

插入位置：第 12 步代码之后，紧接追加；完成稿约第 373 行。

本步声明：`func (s *MainControls) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.MainControlIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;MAIN_CONTROL&quot;)</code>；<code>repository.MainControlResult(ctx, s.db, intent.Operation)</code>；<code>s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;MAIN_CONTROL&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;MAIN_CONTROL_NOT_ALLOWED&quot;)</code>；<code>actor.Sources</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;MAIN_CONTROL&quot;)</code>；<code>DocumentId: document</code>；<code>repository.LockedMergeBarrier(ctx, tx, document)</code>；<code>InsertAdmission(ctx, tx, actor, access.Sources, document, &quot;MAIN_CONTROL&quot;, admission)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.MainControlIntent</code> | 使用 <code>domain.MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>在此声明中的实际使用：<code>intent.Operation</code>；<code>intent.Kind</code>；<code>intent.Control</code>；<code>intent.Chapter</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;MAIN_CONTROL_NOT_ALLOWED&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;MAIN_CONTROL_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, present, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;CHAPTER_CREATE&quot;</code>。
- 检查 <code>intent.Chapter == nil &#124;&#124; intent.Chapter.ExpectedStructureId != access.Document.CurrentStructureId</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;STRUCTURE_CHANGE&quot; &amp;&amp; (intent.Structure == nil &#124;&#124; intent.Structure.ExpectedStructureId != access.Document.CurrentStructureId)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=14379:16601 -->
````go
func (s *MainControls) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.MainControlIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "MAIN_CONTROL")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "MAIN_CONTROL_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	present := s.presentation(actor, access, intent.Operation)
	if _, err = repository.MainControlResult(ctx, s.db, intent.Operation); err == nil {
		// 同号回执读取仍受当前授权，但不会因旧期待修订再次运行已经完成的业务。
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	structure, _ := domain.CanonicalID(access.Document.CurrentStructureId)
	control, _ := domain.CanonicalRevision(access.Document.ControlRevision)
	if intent.Kind == "CHAPTER_CREATE" {
		intent.Control = control
		if intent.Chapter == nil || intent.Chapter.ExpectedStructureId != access.Document.CurrentStructureId {
			return actor, nil, nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
	}
	if intent.Kind == "STRUCTURE_CHANGE" && (intent.Structure == nil || intent.Structure.ExpectedStructureId != access.Document.CurrentStructureId) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, StructureId: structure, WorkspaceEpoch: control, Purpose: "MAIN_CONTROL"}
	proofs := access.Proofs
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, nil, nil, err
	}
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
````

## 第 14 步：MainControls.Prepare（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 418 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Chapter != nil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Structure != nil</code>。
- 在本段的匿名函数内：检查 <code>domain.HasStructureRangeChange(*intent.Structure)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=16601:18850 -->
````go
		if err = domain.ValidateMainControl(access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, intent); err != nil {
			return nil, err
		}
		if intent.Chapter != nil {
			if _, err = validateChapterAnchor(ctx, tx, current, *intent.Chapter); err != nil {
				return nil, err
			}
		}
		var previewBinding *structureConfirmationBinding
		if intent.Structure != nil {
			if domain.HasStructureRangeChange(*intent.Structure) {
				confirmation, err := s.readStructureConfirmation(ctx, tx, actor, access, *intent.Structure)
				if err != nil {
					return nil, err
				}
				previewBinding = &structureConfirmationBinding{Reference: confirmation.Preview.Intent.Operation, Digest: append([]byte(nil), confirmation.Digest...), Expires: confirmation.Expires}
			} else {
				if _, err = stableControlledEntries(ctx, tx, current, *intent.Structure); err != nil {
					return nil, err
				}
			}
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if barrier.Phase != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "MAIN_CONTROL", admission); err != nil {
			return nil, err
		}
		rawTarget, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		fixed := mainControlContext{
			Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs,
			Document: document, Generation: barrier.Generation + 1,
			Intent: intent, Access: access,
			Manifest: manifest, Structure: structure, Style: style, Control: control,
			StructurePreview: previewBinding,
			Context: &pb.OperationContext{
				OperationId: intent.Operation, ActorId: actor.ID,
				IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission,
				Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:],
			},
		}
````

## 第 15 步：MainControls.Prepare（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 472 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return mainControlPending(intent.Operation), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=18850:19369 -->
````go
		original, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		recovery, err := json.Marshal(mainControlRecovery{Fixed: fixed, Attempt: 1})
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterMainControl(ctx, tx, document, fixed.Generation, intent.Operation, original, recovery); err != nil {
			return nil, err
		}
		return mainControlPending(intent.Operation), nil
	}
	return actor, run, present, nil
}

// main_control_readiness.go：保留原实现，按业务合并文件。

````

## 第 16 步：MainControls.Ready

插入位置：第 15 步代码之后，紧接追加；完成稿约第 490 行。

本步声明：`func (s *MainControls) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingMainControlBindings(ctx, s.db)</code>；<code>s.validateControlInstallation(ctx, fixed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;MAIN_CONTROL_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;MAIN_CONTROL_MANIFEST_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;fixed) != nil &#124;&#124; s.validateControlInstallation(ctx, fixed) != nil</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_MANIFEST_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=19369:19933 -->
````go
func (s *MainControls) Ready(ctx context.Context) error {
	facts, err := repository.PendingMainControlBindings(ctx, s.db)
	if err != nil {
		return errors.New("MAIN_CONTROL_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingMainControlBinding(fact); err != nil {
			return err
		}
		if fact.State == "SUCCEEDED" {
			var fixed mainInstallContext
			if json.Unmarshal(fact.Payload, &fixed) != nil || s.validateControlInstallation(ctx, fixed) != nil {
				return errors.New("MAIN_CONTROL_MANIFEST_UNKNOWN")
			}
		}
	}
	return nil
}

````

## 第 17 步：validatePendingMainControlBinding

插入位置：第 16 步代码之后，紧接追加；完成稿约第 509 行。

本步声明：`func validatePendingMainControlBinding(fact repository.PendingMainControlBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingMainControlBinding</code> | 使用 <code>repository.PendingMainControlBinding</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Target</code>；<code>validMainControlContext(original, fact)</code>；<code>fact.Fingerprint</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MAIN_CONTROL_RECOVERY_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>；<code>errors.New(&quot;MAIN_CONTROL_TASK_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;original) != nil &#124;&#124; json.Unmarshal(fact.Target, &amp;target) != nil &#124;&#124; !validMainControlContext(original, fact)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.Intent.Kind == &quot;STYLE_CHANGE&quot;</code>。
- 检查 <code>original.Intent.Kind == &quot;CHAPTER_CREATE&quot;</code>。
- 检查 <code>original.Intent.Kind == &quot;STRUCTURE_CHANGE&quot;</code>。
- 检查 <code>target.Path != path</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;MAIN_CONTROL_RECOVER&quot;</code>。
- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; json.Unmarshal(fact.Payload, &amp;recovery) != nil &#124;&#124; recovery.Attempt == 0 &#124;&#124; !sameMainControlContext(recovery.Fixed, original)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_RECOVERY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;MAIN_CONTROL_RECONCILE&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;install) != nil &#124;&#124; 			!proto.Equal(install.Context, original.Context) &#124;&#124; 			install.Generation != original.Generation &#124;&#124; 			install.Lineage != nil &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>!validStructureInstallation(original, install)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=19933:22103 -->
````go
func validatePendingMainControlBinding(fact repository.PendingMainControlBinding) error {
	var original mainControlContext
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Target, &target) != nil || !validMainControlContext(original, fact) {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	path := "/v1/documents/" + domain.TextID(original.Document) + "/main-lock"
	if original.Intent.Kind == "STYLE_CHANGE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/styles"
	}
	if original.Intent.Kind == "CHAPTER_CREATE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/chapters"
	}
	if original.Intent.Kind == "STRUCTURE_CHANGE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/structure-changes"
	}
	if target.Path != path {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	// 既检查领取载荷与原事实完全一致，也从原意图重建HTTP摘要；两份意图一起被改仍不能就绪。
	fingerprint, err := mainControlIntentFingerprint(original.Intent, path)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	if fact.Kind == "MAIN_CONTROL_RECOVER" {
		var recovery mainControlRecovery
		if fact.State != "PREPARING" || json.Unmarshal(fact.Payload, &recovery) != nil || recovery.Attempt == 0 || !sameMainControlContext(recovery.Fixed, original) {
			return errors.New("MAIN_CONTROL_RECOVERY_UNKNOWN")
		}
	} else if fact.Kind == "MAIN_CONTROL_RECONCILE" {
		var install mainInstallContext
		if json.Unmarshal(fact.Payload, &install) != nil ||
			!proto.Equal(install.Context, original.Context) ||
			install.Generation != original.Generation ||
			install.Lineage != nil ||
			(fact.State != "SUCCEEDED" && fact.State != "REJECTED" && fact.State != "CANCELLED") {
			return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
		}
		if fact.State == "SUCCEEDED" {
			if !validStructureInstallation(original, install) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
			var result struct {
				Result map[string]string `json:"result"`
			}
````

## 第 18 步：validatePendingMainControlBinding（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 556 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Result, &amp;result) != nil &#124;&#124; 				install.Manifest == 0 &#124;&#124; 				result.Result&#91;&quot;manifest_id&quot;&#93; != domain.TextID(install.Manifest) &#124;&#124; 				result.Result&#91;&quot;entity_id&quot;&#93; != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>unit == nil &#124;&#124; 					unit.UnitId == 0 &#124;&#124; 					unit.ContentVersionId == 0 &#124;&#124; 					seen&#91;unit.UnitId&#93; &#124;&#124; 					content == nil &#124;&#124; 					content.DocumentId != original.Document &#124;&#124; 					con …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(digest&#91;:&#93;, install.Digest)</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>install.Manifest != 0 &#124;&#124; len(install.Units) != 0 &#124;&#124; len(install.Digest) != 0</code>。条件成立时执行 <code>return errors.New(&quot;MAIN_CONTROL_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;MAIN_CONTROL_TASK_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=22103:23688 -->
````go
			if json.Unmarshal(fact.Result, &result) != nil ||
				install.Manifest == 0 ||
				result.Result["manifest_id"] != domain.TextID(install.Manifest) ||
				result.Result["entity_id"] != domain.TextID(mainControlResultEntity(install)) ||
				install.Style == 0 ||
				!validControlledStructure(original, install) ||
				install.Control != original.Control+1 ||
				(len(install.Units) == 0 && (!validEmptyControlledManifest(install) || original.Intent.Kind == "CHAPTER_CREATE")) ||
				(original.Intent.Kind == "MAIN_LOCK" && install.Style != original.Style) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
			seen := map[uint64]bool{}
			for _, unit := range install.Units {
				content := unit.GetContent()
				if unit == nil ||
					unit.UnitId == 0 ||
					unit.ContentVersionId == 0 ||
					seen[unit.UnitId] ||
					content == nil ||
					content.DocumentId != original.Document ||
					content.UnitId != unit.UnitId ||
					content.StructureId != install.Structure ||
					content.BranchId != 0 ||
					content.WorkspaceEpoch != install.Control {
					return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
				}
				seen[unit.UnitId] = true
			}
			digest, err := domain.MainManifestDigest(install.Manifest, install.Units)
			if err != nil || !bytes.Equal(digest[:], install.Digest) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
		} else if install.Manifest != 0 || len(install.Units) != 0 || len(install.Digest) != 0 {
			return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
		}
	} else {
		return errors.New("MAIN_CONTROL_TASK_UNKNOWN")
	}
	return nil
}

````

## 第 19 步：sameMainControlContext

插入位置：第 18 步代码之后，紧接追加；完成稿约第 597 行。

本步声明：`func sameMainControlContext(a, b mainControlContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>json.Marshal(a)</code> |
| <code>b</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>json.Marshal(b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>leftErr == nil &amp;&amp; rightErr == nil &amp;&amp; bytes.Equal(left, right)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return leftErr == nil &amp;&amp; rightErr == nil &amp;&amp; bytes.Equal(left, right)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=23688:23891 -->
````go
func sameMainControlContext(a, b mainControlContext) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

````

## 第 20 步：mainControlIntentFingerprint

插入位置：第 19 步代码之后，紧接追加；完成稿约第 603 行。

本步声明：`func mainControlIntentFingerprint(intent domain.MainControlIntent, path string) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>intent</code> | <code>domain.MainControlIntent</code> | 使用 <code>domain.MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>在此声明中的实际使用：<code>intent.Kind</code>；<code>intent.Structure</code>；<code>intent.Chapter</code>；<code>intent.Style</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, *intent.Structure)</code>；<code>domain.OperationFingerprint(path, *intent.Chapter)</code>；<code>domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})</code>；<code>domain.OperationFingerprint(path, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 由 <code>domain.OperationFingerprint(path, *intent.Structure)</code> 的第 1 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, *intent.Chapter)</code> 的第 1 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})</code> 的第 1 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, input)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>&#91;32&#93;byte{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.OperationFingerprint(path, *intent.Structure)</code> 的第 2 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, *intent.Chapter)</code> 的第 2 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})</code> 的第 2 个返回项原样转交。<br>由 <code>domain.OperationFingerprint(path, input)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>intent.Kind == &quot;STRUCTURE_CHANGE&quot;</code>。
- 检查 <code>intent.Structure == nil &#124;&#124; intent.Chapter != nil &#124;&#124; intent.Style != 0 &#124;&#124; len(intent.StyleJSON) != 0 &#124;&#124; intent.Locked</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; parsed.Operation != intent.Operation &#124;&#124; parsed.Control != intent.Control</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.OperationFingerprint(path, *intent.Structure)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>intent.Structure != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;CHAPTER_CREATE&quot;</code>。
- 检查 <code>intent.Chapter == nil &#124;&#124; intent.Chapter.OperationId != intent.Operation &#124;&#124; intent.Style != 0 &#124;&#124; len(intent.StyleJSON) != 0 &#124;&#124; intent.Locked</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.OperationFingerprint(path, *intent.Chapter)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>intent.Chapter != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;MAIN_LOCK&quot;</code>。
- 检查 <code>intent.Style != 0 &#124;&#124; len(intent.StyleJSON) != 0</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.Unmarshal(intent.StyleJSON, &amp;input.Style) != nil</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, errors.New(&quot;MAIN_CONTROL_INTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.OperationFingerprint(path, input)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=23891:25785 -->
````go
func mainControlIntentFingerprint(intent domain.MainControlIntent, path string) ([32]byte, error) {
	if intent.Kind == "STRUCTURE_CHANGE" {
		if intent.Structure == nil || intent.Chapter != nil || intent.Style != 0 || len(intent.StyleJSON) != 0 || intent.Locked {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		parsed, err := domain.ParseStructureChange(*intent.Structure)
		if err != nil || parsed.Operation != intent.Operation || parsed.Control != intent.Control {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		return domain.OperationFingerprint(path, *intent.Structure)
	}
	if intent.Structure != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	if intent.Kind == "CHAPTER_CREATE" {
		if intent.Chapter == nil || intent.Chapter.OperationId != intent.Operation || intent.Style != 0 || len(intent.StyleJSON) != 0 || intent.Locked {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		if _, err := domain.ParseChapterCreate(*intent.Chapter); err != nil {
			return [32]byte{}, err
		}
		return domain.OperationFingerprint(path, *intent.Chapter)
	}
	if intent.Chapter != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	if intent.Kind == "MAIN_LOCK" {
		if intent.Style != 0 || len(intent.StyleJSON) != 0 {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		return domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})
	}
	input := dto.StyleChange{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), ExpectedStyleId: domain.TextID(intent.Style)}
	if json.Unmarshal(intent.StyleJSON, &input.Style) != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	return domain.OperationFingerprint(path, input)
}

````

## 第 21 步：validMainControlContext

插入位置：第 20 步代码之后，紧接追加；完成稿约第 642 行。

本步声明：`func validMainControlContext(fixed mainControlContext, fact repository.PendingMainControlBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Document</code>；<code>fixed.Generation</code>；<code>fixed.SessionExpires</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>fact</code> | <code>repository.PendingMainControlBinding</code> | 使用 <code>repository.PendingMainControlBinding</code> 类型；其定义处的职责：持久化主线锁定、样式和结构控制的原绑定、重试代次、结果及安装上下文。<br>在此声明中的实际使用：<code>fact.Actor</code>；<code>fact.Operation</code>；<code>fact.ReceiptKind</code>；<code>fact.Fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>fixed.Access.Proofs&#91;0&#93;.AdmissionId == ctx.IdentityAdmissionRef</code> |

**跟着本段代码走一遍：**

- 检查 <code>ctx == nil &#124;&#124; 		ctx.Target == nil &#124;&#124; 		fixed.Document == 0 &#124;&#124; 		fixed.Generation == 0 &#124;&#124; 		fixed.SessionExpires &lt;= 0 &#124;&#124; 		fixed.Actor != fact.Actor &#124;&#124; 		ctx.ActorId != fact.Actor &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;CHAPTER_CREATE&quot; &amp;&amp; (fixed.Intent.Chapter == nil &#124;&#124; fixed.Intent.Chapter.ExpectedStructureId != domain.TextID(fixed.Structure))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;STRUCTURE_CHANGE&quot; &amp;&amp; (fixed.Intent.Structure == nil &#124;&#124; fixed.Intent.Structure.ExpectedStructureId != domain.TextID(fixed.Structure) &#124;&#124; fixed.Intent.Structure. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Structure != nil &amp;&amp; domain.HasStructureRangeChange(*fixed.Intent.Structure)</code>。
- 检查 <code>err != nil &#124;&#124; fixed.StructurePreview == nil &#124;&#124; fixed.StructurePreview.Reference != reference &#124;&#124; len(fixed.StructurePreview.Digest) != 32 &#124;&#124; fixed.StructurePreview.Expires &lt;= 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>fixed.StructurePreview != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>ctx.Target.Purpose != &quot;MAIN_CONTROL&quot; &#124;&#124; 		ctx.Target.DocumentId != fixed.Document &#124;&#124; 		ctx.Target.UnitId != 0 &#124;&#124; 		ctx.Target.BranchId != 0 &#124;&#124; 		ctx.Target.StructureId != fixed.Str …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=25785:28218 -->
````go
func validMainControlContext(fixed mainControlContext, fact repository.PendingMainControlBinding) bool {
	ctx := fixed.Context
	if ctx == nil ||
		ctx.Target == nil ||
		fixed.Document == 0 ||
		fixed.Generation == 0 ||
		fixed.SessionExpires <= 0 ||
		fixed.Actor != fact.Actor ||
		ctx.ActorId != fact.Actor ||
		ctx.OperationId != fact.Operation ||
		fixed.Intent.Operation != fact.Operation ||
		fixed.Intent.Kind != fact.ReceiptKind ||
		(fixed.Intent.Kind != "MAIN_LOCK" && fixed.Intent.Kind != "STYLE_CHANGE" && fixed.Intent.Kind != "CHAPTER_CREATE" && fixed.Intent.Kind != "STRUCTURE_CHANGE") ||
		!bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) ||
		len(fact.Fingerprint) != 32 {
		return false
	}
	if fixed.Intent.Kind == "CHAPTER_CREATE" && (fixed.Intent.Chapter == nil || fixed.Intent.Chapter.ExpectedStructureId != domain.TextID(fixed.Structure)) {
		return false
	}
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" && (fixed.Intent.Structure == nil || fixed.Intent.Structure.ExpectedStructureId != domain.TextID(fixed.Structure) || fixed.Intent.Structure.ExpectedControlRevision != domain.TextID(fixed.Control)) {
		return false
	}
	if fixed.Intent.Structure != nil && domain.HasStructureRangeChange(*fixed.Intent.Structure) {
		reference, err := domain.StructureConfirmationReference(*fixed.Intent.Structure)
		if err != nil || fixed.StructurePreview == nil || fixed.StructurePreview.Reference != reference || len(fixed.StructurePreview.Digest) != 32 || fixed.StructurePreview.Expires <= 0 {
			return false
		}
	} else if fixed.StructurePreview != nil {
		return false
	}
	if ctx.Target.Purpose != "MAIN_CONTROL" ||
		ctx.Target.DocumentId != fixed.Document ||
		ctx.Target.UnitId != 0 ||
		ctx.Target.BranchId != 0 ||
		ctx.Target.StructureId != fixed.Structure ||
		ctx.Target.WorkspaceEpoch != fixed.Control ||
		fixed.Control != fixed.Intent.Control ||
		fixed.Manifest == 0 ||
		fixed.Style == 0 ||
		fixed.Structure == 0 ||
		fixed.Access.Document.Id != domain.TextID(fixed.Document) ||
		fixed.Access.Document.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		fixed.Access.Document.CurrentStructureId != domain.TextID(fixed.Structure) ||
		fixed.Access.Document.CurrentStyleId != domain.TextID(fixed.Style) ||
		fixed.Access.Document.ControlRevision != domain.TextID(fixed.Control) ||
		!fixed.Access.Permissions.Manage ||
		(fixed.Intent.Kind == "STYLE_CHANGE" && fixed.Intent.Style != fixed.Style) {
````

## 第 22 步：validMainControlContext（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 690 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.IdentityAdmissionRef == &quot;&quot; &#124;&#124; 		ctx.DocumentAdmissionRef == &quot;&quot; &#124;&#124; 		len(ctx.Sources) == 0 &#124;&#124; 		!proto.Equal(&amp;pb.OperationContext{Sources: ctx.Sources}, &amp;pb.OperationContext{Sou …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; 			proof.ActorId != fixed.Actor &#124;&#124; 			proof.OperationId != fact.Operation &#124;&#124; 			proof.Purpose != &quot;MAIN_CONTROL&quot; &#124;&#124; 			!proto.Equal(proof.Target, ctx.Target)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return fixed.Access.Proofs&#91;0&#93;.AdmissionId == ctx.IdentityAdmissionRef</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=28218:28826 -->
````go
		return false
	}
	if ctx.IdentityAdmissionRef == "" ||
		ctx.DocumentAdmissionRef == "" ||
		len(ctx.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: fixed.Access.Sources}) ||
		len(fixed.Access.Proofs) == 0 {
		return false
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil ||
			proof.ActorId != fixed.Actor ||
			proof.OperationId != fact.Operation ||
			proof.Purpose != "MAIN_CONTROL" ||
			!proto.Equal(proof.Target, ctx.Target) {
			return false
		}
	}
	return fixed.Access.Proofs[0].AdmissionId == ctx.IdentityAdmissionRef
}

````

## 第 23 步：mainControlResultEntity

插入位置：第 22 步代码之后，紧接追加；完成稿约第 711 行。

本步声明：`func mainControlResultEntity(install mainInstallContext) uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>install</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>install.Entity</code>；<code>install.Style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>install.Entity</code>；<code>install.Style</code> |

**跟着本段代码走一遍：**

- 检查 <code>install.Entity != 0</code>。条件成立时执行 <code>return install.Entity</code>。当前函数在这里结束。
- 此处通过 <code>return install.Style</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=28826:29057 -->
````go
func mainControlResultEntity(install mainInstallContext) uint64 {
	if install.Entity != 0 {
		return install.Entity
	}
	// 既有锁/样式回执没有此字段，保留不可变结果与原样式身份。
	return install.Style
}

````

## 第 24 步：validControlledStructure

插入位置：第 23 步代码之后，紧接追加；完成稿约第 719 行。

本步声明：`func validControlledStructure(original mainControlContext, install mainInstallContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>original</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>original.Intent</code>；<code>original.Structure</code>；<code>original.Style</code> |
| <code>install</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>install.Structure</code>；<code>install.Entity</code>；<code>install.Style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>install.Structure != 0 &amp;&amp; install.Structure != original.Structure &amp;&amp; install.Entity == install.Structure &amp;&amp; install.Style == original.Style</code>；<code>install.Structure != 0 &amp;&amp; install.Structure != original.Structure &amp;&amp; install.Entity != 0 &amp;&amp; install.Style == original.Style</code>；<code>install.Structure == original.Structure</code> |

**跟着本段代码走一遍：**

- 检查 <code>original.Intent.Kind == &quot;STRUCTURE_CHANGE&quot;</code>。条件成立时执行 <code>return install.Structure != 0 &amp;&amp; install.Structure != original.Structure &amp;&amp; install.Entity == install.Structure &amp;&amp; install.Style == original.Style</code>。当前函数在这里结束。
- 检查 <code>original.Intent.Kind == &quot;CHAPTER_CREATE&quot;</code>。条件成立时执行 <code>return install.Structure != 0 &amp;&amp; install.Structure != original.Structure &amp;&amp; install.Entity != 0 &amp;&amp; install.Style == original.Style</code>。当前函数在这里结束。
- 此处通过 <code>return install.Structure == original.Structure</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=29057:29660 -->
````go
func validControlledStructure(original mainControlContext, install mainInstallContext) bool {
	if original.Intent.Kind == "STRUCTURE_CHANGE" {
		return install.Structure != 0 && install.Structure != original.Structure && install.Entity == install.Structure && install.Style == original.Style
	}
	if original.Intent.Kind == "CHAPTER_CREATE" {
		return install.Structure != 0 && install.Structure != original.Structure && install.Entity != 0 && install.Style == original.Style
	}
	return install.Structure == original.Structure
}

// main_control_readings.go：保留原实现，按业务合并文件。

````

## 第 25 步：MainControls.fixedContext

插入位置：第 24 步代码之后，紧接追加；完成稿约第 731 行。

本步声明：`func (s *MainControls) fixedContext(ctx context.Context, operation string) (mainControlContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;MAIN_CONTROL_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;MAIN_CONTROL_CONTEXT&quot;, operation)</code>；<code>fixed.Context.OperationId != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mainControlContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.OperationId != operation</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=29660:30178 -->
````go
func (s *MainControls) fixedContext(ctx context.Context, operation string) (mainControlContext, error) {
	var fixed mainControlContext
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_CONTROL_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
	}
	return fixed, nil
}

````

## 第 26 步：MainControls.Lookup

插入位置：第 25 步代码之后，紧接追加；完成稿约第 743 行。

本步声明：`func (s *MainControls) Lookup(ctx context.Context, operation string) (mainControlContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.fixedContext(ctx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mainControlContext</code> | 由 <code>s.fixedContext(ctx, operation)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.fixedContext(ctx, operation)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.fixedContext(ctx, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=30178:30319 -->
````go
func (s *MainControls) Lookup(ctx context.Context, operation string) (mainControlContext, error) {
	return s.fixedContext(ctx, operation)
}

````

## 第 27 步：MainControls.presentation

插入位置：第 26 步代码之后，紧接追加；完成稿约第 747 行。

本步声明：`func (s *MainControls) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation`。


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

<!-- write-step: 27 bytes=30319:30636 -->
````go
func (s *MainControls) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}

````

## 第 28 步：MainControls.Progress

插入位置：第 27 步代码之后，紧接追加；完成稿约第 754 行。

本步声明：`func (s *MainControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, operation)</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;MAIN_CONTROL_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;MAIN_CONTROL_RESULT&quot;)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.fixedContext(ctx, operation)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;MAIN_CONTROL_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;MAIN_CONTROL_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return s.readResult(ctx, tx, actor, access, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=30636:31250 -->
````go
func (s *MainControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.fixedContext(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "MAIN_CONTROL_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "MAIN_CONTROL_RESULT")
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}

````

## 第 29 步：MainControls.readResult

插入位置：第 28 步代码之后，紧接追加；完成稿约第 771 行。

本步声明：`func (s *MainControls) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.MainControlResult(ctx, tx, operation)</code>；<code>repository.MainInstallationCompleted(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.MainControlResult(ctx, tx, operation)</code>；<code>repository.MainInstallationCompleted(ctx, tx, operation)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MainControlResult(ctx, tx, operation)</code>；<code>repository.MainInstallationCompleted(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;MAIN_CONTROL_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_RESULT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != actor.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;MAIN_CONTROL_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>ready</code>。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=31250:32353 -->
````go
func (s *MainControls) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	fact, err := repository.MainControlResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if fact.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "MAIN_CONTROL_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_RESULT_UNKNOWN")
	}
	if fact.State == "SUCCEEDED" {
		ready, err := repository.MainInstallationCompleted(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		sync := "SYNCING"
		if ready {
			sync = "READY"
		}
		// 与MERGE一致：HTTP只投影安装状态，持久系统终态字节与摘要保持原值。
		value["result"].(map[string]any)["main_sync_state"] = sync
	}
	return value, nil
}

````

## 第 30 步：MainControls.OperationResult

插入位置：第 29 步代码之后，紧接追加；完成稿约第 804 行。

本步声明：`func (s *MainControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, query.OperationId)</code>；<code>repository.MainControlResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_CONTROL_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>text != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 30 bytes=32353:34245 -->
````go
func (s *MainControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.MainControlResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
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
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_RESULT_UNKNOWN")
	}
	for _, key := range []string{"manifest_id", "entity_id"} {
		if text := value.Result[key]; text != "" {
			id, err := domain.CanonicalID(text)
			if err != nil {
				return nil, err
			}
			result.ResultIds = append(result.ResultIds, id)
		}
	}
````

## 第 31 步：MainControls.OperationResult（接着实现）

插入位置：第 30 步代码之后，紧接追加；完成稿约第 850 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=34245:34382 -->
````go
	result.BusinessCode = value.Error.Code
	return result, nil
}

// main_control_recovery.go：保留原实现，按业务合并文件。

````

## 第 32 步：MainControls.RecoverOne

插入位置：第 31 步代码之后，紧接追加；完成稿约第 856 行。

本步声明：`func (s *MainControls) RecoverOne(parent context.Context)`。


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
- 检查 <code>task.Kind == &quot;MAIN_CONTROL_RECONCILE&quot;</code>。
- 检查 <code>json.Unmarshal(task.Payload, &amp;recovery) != nil &#124;&#124; recovery.Fixed.Context == nil &#124;&#124; recovery.Attempt == 0</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; 				violation.Kind == domain.EntityNotFound &#124;&#124; 				violation.Kind == domain.StateConflict)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 32 bytes=34382:35590 -->
````go
func (s *MainControls) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimMainControlTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "MAIN_CONTROL_RECONCILE" {
		err = s.reconcile(ctx, task)
	} else {
		var recovery mainControlRecovery
		if json.Unmarshal(task.Payload, &recovery) != nil || recovery.Fixed.Context == nil || recovery.Attempt == 0 {
			err = domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
		} else {
			err = s.recoverControl(ctx, task, recovery)
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied ||
				violation.Kind == domain.EntityNotFound ||
				violation.Kind == domain.StateConflict) {
				err = s.reject(ctx, task, recovery.Fixed, violation.Code)
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

## 第 33 步：mainControlRecoveryCode

插入位置：第 32 步代码之后，紧接追加；完成稿约第 894 行。

任务只记录稳定错误类别；底层语句和远端详情可能带有私有数据，不进入持久状态。

本步声明：`func mainControlRecoveryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>errors.As(err, &amp;database)</code>；<code>status.FromError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>violation.Code</code>；<code>fmt.Sprintf(&quot;MAIN_CONTROL_SQL_%d&quot;, database.Number)</code>；<code>&quot;MAIN_CONTROL_RPC_&quot; + rpc.Code().String()</code>；<code>&quot;MAIN_CONTROL_RECOVERY_UNKNOWN&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return violation.Code</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;database)</code>。条件成立时执行 <code>return fmt.Sprintf(&quot;MAIN_CONTROL_SQL_%d&quot;, database.Number)</code>。当前函数在这里结束。
- 检查 <code>ok</code>。条件成立时执行 <code>return &quot;MAIN_CONTROL_RPC_&quot; + rpc.Code().String()</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;MAIN_CONTROL_RECOVERY_UNKNOWN&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=35590:36121 -->
````go
// 任务只记录稳定错误类别；底层语句和远端详情可能带有私有数据，不进入持久状态。
func mainControlRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("MAIN_CONTROL_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "MAIN_CONTROL_RPC_" + rpc.Code().String()
	}
	return "MAIN_CONTROL_RECOVERY_UNKNOWN"
}

````

## 第 34 步：MainControls.recoverControl

插入位置：第 33 步代码之后，紧接追加；完成稿约第 910 行。

本步声明：`func (s *MainControls) recoverControl(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resume(ctx, recovery.Fixed)</code>；<code>s.recoverStructure(ctx, task, recovery, proofs)</code>；<code>s.captureMain(ctx, task, recovery)</code>；<code>s.client.PrepareMain(ctx, &amp;pb.PrepareMainCommitRequest{Context: recovery.Fixed.Context, BarrierGeneration: recovery.Fixed.Generation, ExpectedCut: main.Cut})</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.recoverStructure(ctx, task, recovery, proofs)</code>；<code>s.captureMain(ctx, task, recovery)</code>；<code>s.advanceAttempt(ctx, task, recovery)</code>；<code>s.commit(ctx, task, recovery, main, prepared, proofs)</code> |
| <code>recovery</code> | <code>mainControlRecovery</code> | 在此声明中的实际使用：<code>recovery.Fixed</code>；<code>s.recoverStructure(ctx, task, recovery, proofs)</code>；<code>s.captureMain(ctx, task, recovery)</code>；<code>s.advanceAttempt(ctx, task, recovery)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.recoverStructure(ctx, task, recovery, proofs)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code>；<code>s.advanceAttempt(ctx, task, recovery)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>recovery.Fixed.StructurePreview != nil</code>。条件成立时执行 <code>return s.recoverStructure(ctx, task, recovery, proofs)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>prepared == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State == pb.MainBarrierReply_CHANGED</code>。条件成立时执行 <code>return s.advanceAttempt(ctx, task, recovery)</code>。当前函数在这里结束。
- 检查 <code>prepared.State != pb.MainBarrierReply_PREPARED</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_PREPARE_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.commit(ctx, task, recovery, main, prepared, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=36121:37131 -->
````go
func (s *MainControls) recoverControl(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error {
	proofs, err := s.resume(ctx, recovery.Fixed)
	if err != nil {
		return err
	}
	if recovery.Fixed.StructurePreview != nil {
		return s.recoverStructure(ctx, task, recovery, proofs)
	}
	main, err := s.captureMain(ctx, task, recovery)
	if err != nil {
		return err
	}
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: recovery.Fixed.Context, BarrierGeneration: recovery.Fixed.Generation, ExpectedCut: main.Cut})
	if err != nil {
		return err
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return s.advanceAttempt(ctx, task, recovery)
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commit(ctx, task, recovery, main, prepared, proofs)
}

````

## 第 35 步：MainControls.reject

插入位置：第 34 步代码之后，紧接追加；完成稿约第 938 行。

本步声明：`func (s *MainControls) reject(ctx context.Context, task domain.SubmissionTask, fixed mainControlContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Document, true)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Generation</code>；<code>fixed.Context</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


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
- 检查 <code>barrier.Operation == task.Operation &amp;&amp; barrier.Generation == fixed.Generation</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 35 bytes=37131:38779 -->
````go
func (s *MainControls) reject(ctx context.Context, task domain.SubmissionTask, fixed mainControlContext, code string) error {
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
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if barrier.Operation == task.Operation && barrier.Generation == fixed.Generation {
		payload, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: fixed.Generation})
		if err != nil {
			return err
		}
		// 即使拒绝也要以原号查Go终态并解除自己的Node准备；未知RPC结果不直接开放旧MAIN。
		if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECONCILE", task.Operation, task.Operation, payload, "QUEUED"); err != nil {
			return err
		}
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_source.go：保留原实现，按业务合并文件。

````

## 第 36 步：MainControls.resume

插入位置：第 35 步代码之后，紧接追加；完成稿约第 982 行。

本步声明：`func (s *MainControls) resume(ctx context.Context, fixed mainControlContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=38779:39289 -->
````go
func (s *MainControls) resume(ctx context.Context, fixed mainControlContext) ([]*pb.IdentityAdmission, error) {
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

## 第 37 步：validateMainControlProofs

插入位置：第 36 步代码之后，紧接追加；完成稿约第 997 行。

本步声明：`func validateMainControlProofs(fixed mainControlContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>len(proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(proofs) == 0 &#124;&#124; len(proofs) != len(fixed.Access.Proofs)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=39289:39889 -->
````go
func validateMainControlProofs(fixed mainControlContext, proofs []*pb.IdentityAdmission) error {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(proofs) == 0 || len(proofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
	}
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

````

## 第 38 步：MainControls.MainSource

插入位置：第 37 步代码之后，紧接追加；完成稿约第 1012 行。

本步声明：`func (s *MainControls) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.CaptureMainRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>input.ExpectedManifestId</code>；<code>input.ExpectedStructureId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlRevision …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

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
- 检查 <code>current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) &#124;&#124; 		current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) &#124;&#124; 		current.CurrentStyleId != doma …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != fixed.Intent.Operation &#124;&#124; barrier.Generation != fixed.Generation &#124;&#124; barrier.Phase != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 38 bytes=39889:41708 -->
````go
func (s *MainControls) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
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
	if err = domain.ValidateMainControl(fixed.Access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, fixed.Intent); err != nil {
		return nil, err
	}
	if current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) ||
		current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) ||
		current.CurrentStyleId != domain.TextID(input.ExpectedStyleId) ||
		current.ControlRevision != domain.TextID(input.ExpectedControlRevision) {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	if barrier.Operation != fixed.Intent.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return nil, err
	}
````

## 第 39 步：MainControls.MainSource（接着实现）

插入位置：第 38 步代码之后，紧接追加；完成稿约第 1058 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlR …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=41708:42250 -->
````go
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, current.CurrentManifestId, input.ExpectedControlRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

// main_control_units.go：保留原实现，按业务合并文件。

````

## 第 40 步：materializeControlledUnits

插入位置：第 39 步代码之后，紧接追加；完成稿约第 1070 行。

本步声明：`func materializeControlledUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, layout mainControlLayout, entries []map[string]string, captured map[uint64]*pb.FixedContent, manifest, control uint64) ([]*pb.ManifestUnit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, version, fixed.Document)</code>；<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId)</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)</code>；<code>initialUnitVersion(ctx, tx, fixed.Document, layout.Unit, layout.Structure, manifest, fixed.Intent.Operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, version, fixed.Document)</code>；<code>repository.InsertMergeVersion(ctx, tx, content, &quot;MAIN_CAPTURE&quot;, content.CaptureId)</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)</code>；<code>initialUnitVersion(ctx, tx, fixed.Document, layout.Unit, layout.Structure, manifest, fixed.Intent.Operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Structure</code>；<code>fixed.Intent</code> |
| <code>layout</code> | <code>mainControlLayout</code> | 在此声明中的实际使用：<code>layout.Structure</code>；<code>layout.Unit</code> |
| <code>entries</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>len(entries)</code> |
| <code>captured</code> | <code>map&#91;uint64&#93;*pb.FixedContent</code> | 在此声明中的实际使用：<code>content := captured&#91;unit&#93;</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)</code>；<code>initialUnitVersion(ctx, tx, fixed.Document, layout.Unit, layout.Structure, manifest, fixed.Intent.Operation)</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, layout.Unit, version, control, content)</code> |
| <code>control</code> | <code>uint64</code> | 在此声明中的实际使用：<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)</code>；<code>controlledMainUnit(ctx, tx, fixed.Document, manifest, layout.Unit, version, control, content)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>content == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>layout.Structure == fixed.Structure &amp;&amp; bytes.Equal(old.BodyHash, content.BodyHash)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>layout.Unit != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 40 bytes=42250:44083 -->
````go
func materializeControlledUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, layout mainControlLayout, entries []map[string]string, captured map[uint64]*pb.FixedContent, manifest, control uint64) ([]*pb.ManifestUnit, error) {
	units := make([]*pb.ManifestUnit, 0, len(entries)+1)
	for _, entry := range entries {
		unit, _ := domain.CanonicalID(entry["unit_id"])
		version, _ := domain.CanonicalID(entry["content_version_id"])
		content := captured[unit]
		if content == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
		}
		old, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return nil, err
		}
		if layout.Structure == fixed.Structure && bytes.Equal(old.BodyHash, content.BodyHash) {
			if err = canonicalVersion(old, version); err != nil {
				return nil, err
			}
			content = old
		} else {
			// 即使正文相同，结构上下文变化也须创建新版本；不改写旧版本的structure_id。
			content = proto.Clone(content).(*pb.FixedContent)
			content.StructureId = layout.Structure
			version, err = repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId)
			if err != nil {
				return nil, err
			}
		}
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)
		if err != nil {
			return nil, err
		}
		units = append(units, installed)
	}
	if layout.Unit != 0 {
		version, err := initialUnitVersion(ctx, tx, fixed.Document, layout.Unit, layout.Structure, manifest, fixed.Intent.Operation)
		if err != nil {
			return nil, err
		}
		content, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return nil, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return nil, err
		}
````

## 第 41 步：materializeControlledUnits（接着实现）

插入位置：第 40 步代码之后，紧接追加；完成稿约第 1115 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return units, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=44083:44298 -->
````go
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, layout.Unit, version, control, content)
		if err != nil {
			return nil, err
		}
		units = append(units, installed)
	}
	return units, nil
}

````

## 第 42 步：controlledMainUnit

插入位置：第 41 步代码之后，紧接追加；完成稿约第 1124 行。

本步声明：`func controlledMainUnit(ctx context.Context, tx *sql.Tx, document, manifest, unit, version, control uint64, content *pb.FixedContent) (*pb.ManifestUnit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code>；<code>collaborationRoom(ctx, tx, &quot;MAIN&quot;, unit, unit, control)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code>；<code>collaborationRoom(ctx, tx, &quot;MAIN&quot;, unit, unit, control)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code>；<code>collaborationRoom(ctx, tx, &quot;MAIN&quot;, unit, unit, control)</code>；<code>UnitId: unit</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version)</code>；<code>ContentVersionId: version</code> |
| <code>control</code> | <code>uint64</code> | 在此声明中的实际使用：<code>collaborationRoom(ctx, tx, &quot;MAIN&quot;, unit, unit, control)</code>；<code>content.BranchId, content.WorkspaceEpoch = 0, control</code>；<code>domain.TextID(control)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.BranchId</code>；<code>content.WorkspaceEpoch</code>；<code>content.CutJsonUtf8</code>；<code>Content: content</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.ManifestUnit</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.ManifestUnit{UnitId: unit, ContentVersionId: version, Content: content}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;pb.ManifestUnit{UnitId: unit, ContentVersionId: version, Content: content}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=44298:45102 -->
````go
func controlledMainUnit(ctx context.Context, tx *sql.Tx, document, manifest, unit, version, control uint64, content *pb.FixedContent) (*pb.ManifestUnit, error) {
	if err := repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version); err != nil {
		return nil, err
	}
	room, err := collaborationRoom(ctx, tx, "MAIN", unit, unit, control)
	if err != nil {
		return nil, err
	}
	content.BranchId, content.WorkspaceEpoch = 0, control
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(control), "durable_seq": "0"})
	if err != nil {
		return nil, err
	}
	return &pb.ManifestUnit{UnitId: unit, ContentVersionId: version, Content: content}, nil
}

// main_controls.go：保留原实现，按业务合并文件。

````

## 第 43 步：MainControls

插入位置：第 42 步代码之后，紧接追加；完成稿约第 1142 行。

本步声明：`type MainControls struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>MergeClient</code> | 使用 <code>MergeClient</code> 类型；其定义处的职责：合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 43 bytes=45102:45260 -->
````go
type MainControls struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

````

## 第 44 步：mainControlContext

插入位置：第 43 步代码之后，紧接追加；完成稿约第 1151 行。

本步声明：`type mainControlContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Document</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;document&quot;`</code> |
| <code>Generation</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |
| <code>Intent</code> | <code>domain.MainControlIntent</code> | 使用 <code>domain.MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>序列化/校验标记：<code>`json:&quot;intent&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 本次操作固定的主线清单，包含各单元版本引用。 |
| <code>Structure</code> | <code>uint64</code> | 固定的目录结构版本。 |
| <code>Style</code> | <code>uint64</code> | 固定的文档样式版本。 |
| <code>Control</code> | <code>uint64</code> | 固定的主线控制修订上下文；后续准备/安装必须匹配，不能从当前值覆盖原值。 |
| <code>StructurePreview</code> | <code>*structureConfirmationBinding</code> | 序列化/校验标记：<code>`json:&quot;structure_preview,omitempty&quot;`</code> |

<!-- write-step: 44 bytes=45260:46005 -->
````go
type mainControlContext struct {
	Actor                               uint64                   `json:"actor"`
	SessionExpires                      int64                    `json:"session_expires"`
	Document                            uint64                   `json:"document"`
	Generation                          uint64                   `json:"generation"`
	Intent                              domain.MainControlIntent `json:"intent"`
	Access                              DocumentAccess           `json:"access"`
	Context                             *pb.OperationContext     `json:"context"`
	Manifest, Structure, Style, Control uint64
	StructurePreview                    *structureConfirmationBinding `json:"structure_preview,omitempty"`
}

````

## 第 45 步：mainControlRecovery

插入位置：第 44 步代码之后，紧接追加；完成稿约第 1163 行。

本步声明：`type mainControlRecovery struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Fixed</code> | <code>mainControlContext</code> | 序列化/校验标记：<code>`json:&quot;fixed&quot;`</code> |
| <code>Attempt</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;attempt&quot;`</code> |

<!-- write-step: 45 bytes=46005:46130 -->
````go
type mainControlRecovery struct {
	Fixed   mainControlContext `json:"fixed"`
	Attempt uint64             `json:"attempt"`
}

````

## 第 46 步：NewMainControls

插入位置：第 45 步代码之后，紧接追加；完成稿约第 1168 行。

本步声明：`func NewMainControls(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *MainControls`。


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
| <code>(匿名/未命名)</code> | <code>*MainControls</code> | 实际返回表达式：<code>&amp;MainControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;MainControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 46 bytes=46130:46406 -->
````go
func NewMainControls(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *MainControls {
	return &MainControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/main_controls_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
