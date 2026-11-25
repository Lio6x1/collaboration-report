# 分步编写 `services/document/internal/logic/document/structure_confirmation_service.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/238-structure-confirmation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证确认者确实消费自己的原成功预览，并提供固定确认source。

浏览器确认只能引用已固定候选，不能在确认时提交新的正文或更换cut。

写之前先按这条执行路线理解各段的连接：

1. validateStructureConfirmationActor核用户、会话、权限与期限
2. readStructureConfirmation锁原工件
3. loadStructureConfirmation恢复原绑定
4. structureConfirmationCut取原切面
5. ConfirmationSource仅返回固定引用。

本文件需要守住的边界：

- 预览被别的用户创建、会话变化、候选缺失或scope变化都要拒绝
- 不能借私有mTLS跳过用户资格。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/structure_confirmation_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/structure_confirmation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/structure_confirmation_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/structure_confirmation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:582 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"strconv"
	"time"
)

// structure_confirmation.go：保留原实现，按业务合并文件。

````

## 第 2 步：structureConfirmationBinding

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type structureConfirmationBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Reference</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;reference&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;digest&quot;`</code> |
| <code>Expires</code> | <code>int64</code> | 序列化/校验标记：<code>`json:&quot;expires&quot;`</code> |

<!-- write-step: 2 bytes=582:734 -->
````go
type structureConfirmationBinding struct {
	Reference string `json:"reference"`
	Digest    []byte `json:"digest"`
	Expires   int64  `json:"expires"`
}

````

## 第 3 步：validateStructureConfirmationActor

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func validateStructureConfirmationActor(actor Actor, access DocumentAccess, input dto.StructureChange, fact repository.PendingStructurePreviewBinding, now time.Time) (structurePreviewContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.SessionExpiresUnixMs</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Document</code>；<code>access.Permissions</code>；<code>access.Proofs</code>；<code>access.Sources</code> |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>domain.ValidateStructureConfirmation(input, preview.Intent.Input)</code>；<code>input.OperationId</code> |
| <code>fact</code> | <code>repository.PendingStructurePreviewBinding</code> | 使用 <code>repository.PendingStructurePreviewBinding</code> 类型；其定义处的职责：持久结构预览上下文与恢复任务，列出当前结构的单元和活动支线。<br>在此声明中的实际使用：<code>fact.State</code>；<code>validateOriginalStructurePreviewBinding(fact)</code> |
| <code>now</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>now.UnixMilli</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structurePreviewContext</code> | 实际返回表达式：<code>preview</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_NOT_FIXED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_ACTOR_MISMATCH&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>fact.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return preview, domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_NOT_FIXED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return preview, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return preview, err</code>。当前函数在这里结束。
- 检查 <code>actor.ID != preview.Actor &#124;&#124; access.Document.Id != domain.TextID(preview.Document)</code>。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_ACTOR_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>actor.SessionExpiresUnixMs &lt;= now.UnixMilli() &#124;&#124; preview.SessionExpires &lt;= now.UnixMilli()</code>。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>actor.SessionExpiresUnixMs != preview.SessionExpires &#124;&#124; len(access.Proofs) == 0</code>。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_SESSION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != actor.ID &#124;&#124; proof.OperationId != input.OperationId &#124;&#124; proof.Purpose != &quot;MAIN_CONTROL&quot; &#124;&#124; !proto.Equal(proof.Target, expected) &#124;&#124; proof.ExpiresUnixM …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_SESSION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>!equalPullSources(access.Sources, preview.Access.Sources)</code>。条件成立时执行 <code>return preview, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>access.Document.MainLocked &#124;&#124; access.Document.CurrentManifestId != domain.TextID(preview.Manifest) &#124;&#124; access.Document.CurrentStructureId != domain.TextID(preview.Structure) &#124;&#124; acce …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return preview, domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return preview, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=734:3493 -->
````go
func validateStructureConfirmationActor(actor Actor, access DocumentAccess, input dto.StructureChange, fact repository.PendingStructurePreviewBinding, now time.Time) (structurePreviewContext, error) {
	var preview structurePreviewContext
	if fact.State != "SUCCEEDED" {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_NOT_FIXED")
	}
	preview, err := validateOriginalStructurePreviewBinding(fact)
	if err != nil {
		return preview, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	if err = domain.ValidateStructureConfirmation(input, preview.Intent.Input); err != nil {
		return preview, err
	}
	if actor.ID != preview.Actor || access.Document.Id != domain.TextID(preview.Document) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ACTOR_MISMATCH")
	}
	if !access.Permissions.Manage {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	if actor.SessionExpiresUnixMs <= now.UnixMilli() || preview.SessionExpires <= now.UnixMilli() {
		return preview, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if actor.SessionExpiresUnixMs != preview.SessionExpires || len(access.Proofs) == 0 {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
	}
	expected := &pb.Target{DocumentId: preview.Document, StructureId: preview.Structure, WorkspaceEpoch: preview.Control, Purpose: "MAIN_CONTROL"}
	for _, proof := range access.Proofs {
		// 原短期证明可以已经过期；当前确认必须取得同一会话的新真实证明，不能用另一登录会话接管预览。
		if proof == nil || proof.ActorId != actor.ID || proof.OperationId != input.OperationId || proof.Purpose != "MAIN_CONTROL" || !proto.Equal(proof.Target, expected) || proof.ExpiresUnixMs <= now.UnixMilli() || len(proof.SessionRef) != 32 || !bytes.Equal(proof.SessionRef, preview.Access.Proofs[0].SessionRef) {
			return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
		}
	}
	if !equalPullSources(access.Sources, preview.Access.Sources) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SOURCE_CHANGED")
	}
	if access.Document.MainLocked || access.Document.CurrentManifestId != domain.TextID(preview.Manifest) || access.Document.CurrentStructureId != domain.TextID(preview.Structure) || access.Document.CurrentStyleId != domain.TextID(preview.Style) || access.Document.ControlRevision != domain.TextID(preview.Control) {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CONTEXT_CHANGED")
	}
	return preview, nil
}

// structure_confirmation_read.go：保留原实现，按业务合并文件。

````

## 第 4 步：structureConfirmation

插入位置：第 3 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`type structureConfirmation struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Preview</code> | <code>structurePreviewContext</code> | 用户确认对应的原预览上下文。 |
| <code>Snapshot</code> | <code>structurePreviewSnapshot</code> | 原预览固定的主线和草稿快照。 |
| <code>Raw</code> | <code>&#91;&#93;byte</code> | 原预览结果的完整序列化字节。 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 原预览字节的摘要，用户确认须匹配。 |
| <code>Expires</code> | <code>int64</code> | 预览确认有效期限；过期需重新预览而不能沿旧确认变更新内容。 |
| <code>Migration</code> | <code>domain.StructureMigration</code> | 使用 <code>domain.StructureMigration</code> 类型；其定义处的职责：这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。 |

<!-- write-step: 4 bytes=3493:3693 -->
````go
type structureConfirmation struct {
	Preview   structurePreviewContext
	Snapshot  structurePreviewSnapshot
	Raw       []byte
	Digest    []byte
	Expires   int64
	Migration domain.StructureMigration
}

````

## 第 5 步：MainControls.readStructureConfirmation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (s *MainControls) readStructureConfirmation(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, input dto.StructureChange) (structureConfirmation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedStructureConfirmation(ctx, tx, reference)</code>；<code>validateStructurePreviewUnits(ctx, tx, result.Preview)</code>；<code>structurePreviewBlockingUnits(ctx, tx, result.Preview)</code>；<code>repository.OriginalAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedStructureConfirmation(ctx, tx, reference)</code>；<code>validateStructurePreviewUnits(ctx, tx, result.Preview)</code>；<code>structurePreviewBlockingUnits(ctx, tx, result.Preview)</code>；<code>repository.OriginalAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>validateStructureConfirmationActor(actor, access, input, fact.Binding, now)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>validateStructureConfirmationActor(actor, access, input, fact.Binding, now)</code> |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>domain.StructureConfirmationReference(input)</code>；<code>validateStructureConfirmationActor(actor, access, input, fact.Binding, now)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structureConfirmation</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGE_BLOCKED&quot;)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return result, domain.Violation(domain.EntityNotFound, &quot;STRUCTURE_PREVIEW_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!fact.ReceiptExpires.After(now)</code>。条件成立时执行 <code>return result, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(blocks) != 0</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGE_BLOCKED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.PermissionDenied, &quot;STRUCTURE_PREVIEW_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3693:5370 -->
````go
func (s *MainControls) readStructureConfirmation(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, input dto.StructureChange) (structureConfirmation, error) {
	var result structureConfirmation
	reference, err := domain.StructureConfirmationReference(input)
	if err != nil {
		return result, err
	}
	fact, err := repository.LockedStructureConfirmation(ctx, tx, reference)
	if err == sql.ErrNoRows {
		return result, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	if err != nil {
		return result, err
	}
	now := time.Now()
	if !fact.ReceiptExpires.After(now) {
		return result, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	result.Preview, err = validateStructureConfirmationActor(actor, access, input, fact.Binding, now)
	if err != nil {
		return result, err
	}
	if err = validateStructurePreviewUnits(ctx, tx, result.Preview); err != nil {
		return result, err
	}
	blocks, err := structurePreviewBlockingUnits(ctx, tx, result.Preview)
	if err != nil {
		return result, err
	}
	if len(blocks) != 0 {
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGE_BLOCKED")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef, true)
	if err != nil {
		return result, err
	}
	if err = validateStructurePreviewAdmission(result.Preview, admission); err != nil {
		return result, err
	}
	state, err := repository.LockedAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef)
	if err != nil {
		return result, err
	}
	if state.State == "CLOSED" {
		return result, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ADMISSION_CLOSED")
	}
````

## 第 6 步：MainControls.readStructureConfirmation（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 124 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!equalPullSources(sources, result.Preview.Access.Sources)</code>。条件成立时执行 <code>return result, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Snapshot, &amp;envelope) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; result.Expires &lt;= 0 &#124;&#124; strconv.FormatInt(result.Expires, 10) != envelope.Expires</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>result.Expires &lt;= now.UnixMilli()</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(envelope.Cut, cut) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(envelope.Migration) == 0</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CANDIDATE_MISSING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=5370:7810 -->
````go
	if state.State != "OPEN" {
		return result, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, result.Preview.Context.DocumentAdmissionRef)
	if err != nil {
		return result, err
	}
	if !equalPullSources(sources, result.Preview.Access.Sources) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	// 同一原快照先校验完整digest/cut/body，再读取候选；引用和到期时间都不能只相信浏览器参数。
	var envelope struct {
		Preview   string          `json:"preview_cut_ref"`
		Expires   string          `json:"expires_unix_ms"`
		Digest    []byte          `json:"input_digest"`
		Cut       json.RawMessage `json:"observed_cut"`
		Migration json.RawMessage `json:"migration"`
		Source    json.RawMessage `json:"source"`
	}
	invalid := domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	if json.Unmarshal(fact.Snapshot, &envelope) != nil {
		return result, invalid
	}
	result.Expires, err = strconv.ParseInt(envelope.Expires, 10, 64)
	if err != nil || result.Expires <= 0 || strconv.FormatInt(result.Expires, 10) != envelope.Expires {
		return result, invalid
	}
	if result.Expires <= now.UnixMilli() {
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_EXPIRED")
	}
	cut := &pb.CutVector{}
	if protojson.Unmarshal(envelope.Cut, cut) != nil {
		return result, invalid
	}
	reply := &pb.DraftPreviewReply{State: pb.CaptureReply_FIXED, PreviewCutRef: envelope.Preview,
		MainManifestId: result.Preview.Manifest, ObservedCut: cut, ExpiresUnixMs: result.Expires, InputDigest: envelope.Digest}
	result.Snapshot, err = validateStructurePreviewSnapshot(s.validator, fact.Snapshot, result.Preview, reply)
	if err != nil {
		return result, err
	}
	if len(envelope.Migration) == 0 {
		// 首轮旧事实只固定正文，没有迁移候选。保留原字节，明确拒绝确认，不能原号内重新算候选。
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CANDIDATE_MISSING")
	}
	var source struct {
		Manifest    string                `json:"manifest_id"`
		StructureID string                `json:"structure_id"`
		Style       string                `json:"style_id"`
		Control     string                `json:"control_revision"`
		Structure   dto.StructureRevision `json:"structure"`
	}
````

## 第 7 步：MainControls.readStructureConfirmation（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 175 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(envelope.Source, &amp;source) != nil &#124;&#124; source.Manifest != domain.TextID(result.Preview.Manifest) &#124;&#124; source.StructureID != domain.TextID(result.Preview.Structure) &#124;&#124; sou …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(actualJSON, sourceJSON)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(envelope.Source, &amp;sourceIntent) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(declared, original)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=7810:9605 -->
````go
	if json.Unmarshal(envelope.Source, &source) != nil || source.Manifest != domain.TextID(result.Preview.Manifest) || source.StructureID != domain.TextID(result.Preview.Structure) || source.Style != domain.TextID(result.Preview.Style) || source.Control != domain.TextID(result.Preview.Control) {
		return result, invalid
	}
	actualStructure, err := repository.StructureRow(ctx, tx, source.StructureID, domain.TextID(result.Preview.Document))
	if err != nil {
		return result, err
	}
	actualJSON, _ := json.Marshal(actualStructure)
	sourceJSON, _ := json.Marshal(source.Structure)
	if !bytes.Equal(actualJSON, sourceJSON) {
		return result, invalid
	}
	// 系统source里的changes也必须是实际原HTTP意图，不能仅验证候选自己声称的动作种类。
	var sourceIntent dto.StructurePreviewCreate
	if json.Unmarshal(envelope.Source, &sourceIntent) != nil {
		return result, invalid
	}
	sourceIntent.OperationId = result.Preview.Intent.Operation
	sourceIntent.ExpectedStructureId = source.StructureID
	sourceIntent.ExpectedControlRevision = source.Control
	declared, _ := json.Marshal(sourceIntent)
	original, _ := json.Marshal(result.Preview.Intent.Input)
	if !bytes.Equal(declared, original) {
		return result, invalid
	}
	scopes := make([]domain.StructureMigrationSource, 0, len(result.Preview.Units))
	for _, unit := range result.Preview.Units {
		scopes = append(scopes, domain.StructureMigrationSource{Unit: unit.Unit, Chapter: unit.Chapter})
	}
	result.Migration, err = domain.ParseStructureMigration(envelope.Migration, result.Preview.Intent.Input, scopes, s.validator)
	if err != nil {
		return result, err
	}
	result.Raw, result.Digest = fact.Snapshot, envelope.Digest
	return result, nil
}

// structure_confirmation_source.go：保留原实现，按业务合并文件。

````

## 第 8 步：MainControls.loadStructureConfirmation

插入位置：第 7 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`func (s *MainControls) loadStructureConfirmation(ctx context.Context, tx *sql.Tx, fixed mainControlContext, proofs []*pb.IdentityAdmission) (structureConfirmation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.readStructureConfirmation(ctx, tx, Actor{ID: fixed.Actor, SessionExpiresUnixMs: fixed.SessionExpires}, access, *fixed.Intent.Structure)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.readStructureConfirmation(ctx, tx, Actor{ID: fixed.Actor, SessionExpiresUnixMs: fixed.SessionExpires}, access, *fixed.Intent.Structure)</code> |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.StructurePreview</code>；<code>fixed.Intent</code>；<code>fixed.Access</code>；<code>fixed.Actor</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>access.Proofs = proofs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>structureConfirmation</code> | 实际返回表达式：<code>empty</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_CONFIRMATION_BINDING_UNKNOWN&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.StructurePreview == nil &#124;&#124; fixed.Intent.Structure == nil</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_CONFIRMATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>fixed.StructurePreview.Reference != result.Preview.Intent.Operation &#124;&#124; fixed.StructurePreview.Expires != result.Expires &#124;&#124; !bytes.Equal(fixed.StructurePreview.Digest, result.Digest)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_CONFIRMATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=9605:10551 -->
````go
func (s *MainControls) loadStructureConfirmation(ctx context.Context, tx *sql.Tx, fixed mainControlContext, proofs []*pb.IdentityAdmission) (structureConfirmation, error) {
	var empty structureConfirmation
	if fixed.StructurePreview == nil || fixed.Intent.Structure == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_CONFIRMATION_BINDING_UNKNOWN")
	}
	access := fixed.Access
	access.Proofs = proofs
	result, err := s.readStructureConfirmation(ctx, tx, Actor{ID: fixed.Actor, SessionExpiresUnixMs: fixed.SessionExpires}, access, *fixed.Intent.Structure)
	if err != nil {
		return empty, err
	}
	if fixed.StructurePreview.Reference != result.Preview.Intent.Operation || fixed.StructurePreview.Expires != result.Expires || !bytes.Equal(fixed.StructurePreview.Digest, result.Digest) {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_CONFIRMATION_BINDING_UNKNOWN")
	}
	return result, nil
}

````

## 第 9 步：structureConfirmationCut

插入位置：第 8 步代码之后，紧接追加；完成稿约第 231 行。

本步声明：`func structureConfirmationCut(fixed structureConfirmation) (*pb.CutVector, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>structureConfirmation</code> | 在此声明中的实际使用：<code>fixed.Migration</code>；<code>fixed.Preview</code>；<code>fixed.Snapshot</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CutVector</code> | 由 <code>domain.StructurePreparationCut(fixed.Snapshot.Cut, all, selected)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.StructurePreparationCut(fixed.Snapshot.Cut, all, selected)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>affected&#91;domain.TextID(unit.Unit)&#93;</code>。
- 此处通过 <code>return domain.StructurePreparationCut(fixed.Snapshot.Cut, all, selected)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=10551:11076 -->
````go
func structureConfirmationCut(fixed structureConfirmation) (*pb.CutVector, error) {
	affected := map[string]bool{}
	for _, change := range fixed.Migration.Changes {
		for _, source := range change.Sources {
			affected[source] = true
		}
	}
	all, selected := []uint64{}, []uint64{}
	for _, unit := range fixed.Preview.Units {
		all = append(all, unit.Room)
		if affected[domain.TextID(unit.Unit)] {
			selected = append(selected, unit.Room)
		}
	}
	return domain.StructurePreparationCut(fixed.Snapshot.Cut, all, selected)
}

````

## 第 10 步：MainControls.ConfirmationSource

插入位置：第 9 步代码之后，紧接追加；完成稿约第 248 行。

此私有查询只返回原确认中的预览绑定。候选留在固定artifact，调用方不能另提交候选或新cut。

本步声明：`func (s *MainControls) ConfirmationSource(ctx context.Context, input *pb.PrepareMainCommitRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, fixed.Access.Sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.PrepareMainCommitRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>input.ExpectedCut</code>；<code>input.BarrierGeneration</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_CONFIRMATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil &#124;&#124; input.ExpectedCut == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;STRUCTURE_CONFIRMATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(fixed.Context, input.Context) &#124;&#124; fixed.Generation != input.BarrierGeneration</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Operation != fixed.Intent.Operation &#124;&#124; barrier.Generation != fixed.Generation &#124;&#124; barrier.Phase != &quot;PREPARING&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_BARRIER_SUPERSEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.StructurePreview != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=11076:12898 -->
````go
// 此私有查询只返回原确认中的预览绑定。候选留在固定artifact，调用方不能另提交候选或新cut。
func (s *MainControls) ConfirmationSource(ctx context.Context, input *pb.PrepareMainCommitRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.ExpectedCut == nil {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_CONFIRMATION_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) || fixed.Generation != input.BarrierGeneration {
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
	value := map[string]any{"structure_preview": nil}
	if fixed.StructurePreview != nil {
		confirmation, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
		if err != nil {
````

## 第 11 步：MainControls.ConfirmationSource（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 293 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(cut, input.ExpectedCut)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_PREVIEW_CUT_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=12898:13470 -->
````go
			return nil, err
		}
		cut, err := structureConfirmationCut(confirmation)
		if err != nil {
			return nil, err
		}
		if !proto.Equal(cut, input.ExpectedCut) {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CUT_CONFLICT")
		}
		value["structure_preview"] = map[string]string{"reference": fixed.StructurePreview.Reference, "digest": base64.StdEncoding.EncodeToString(fixed.StructurePreview.Digest), "expires": domain.TextID(uint64(fixed.StructurePreview.Expires))}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return value, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/structure_confirmation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
