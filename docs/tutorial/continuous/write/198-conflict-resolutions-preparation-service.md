# 分步编写 `services/document/internal/logic/document/conflict_resolutions_preparation_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/198-conflict-resolutions-preparation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定修复候选、发起协作准备并验证恢复上下文和来源。

客户端修复正文需经过规范验证并固化一次，网络重试不能产生不同候选。

写之前先按这条执行路线理解各段的连接：

1. compute获取预览
2. fixedPreview核cut和正文
3. persistFixed保存摘要
4. validate核原来源/工作区
5. Source只提供登记过的固定上下文
6. Ready检查待恢复绑定。

本文件需要守住的边界：

- 仅已约定的确定拒绝可终结原号
- 超时、坏来源与未知RPC错误保持未知恢复，不释放控制。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_preparation_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_preparation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/conflict_resolutions_preparation_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/conflict_resolutions_preparation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:671 -->
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"reflect"
	"time"
)

// conflict_resolution_compute.go：保留原实现，按业务合并文件。

````

## 第 2 步：Resolutions.fixedPreview

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (s *Resolutions) fixedPreview(ctx context.Context, tx *sql.Tx, raw []byte, fixed resolutionContext) (draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.original(ctx, tx, fixed.Origin, fixed.Intent, false)</code>；<code>resolutionBaseline(ctx, tx, fixed)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.original(ctx, tx, fixed.Origin, fixed.Intent, false)</code>；<code>resolutionBaseline(ctx, tx, fixed)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>resolutionPreviewRequest(fixed)</code>；<code>fixed.Intent</code>；<code>fixed.Scope</code>；<code>fixed.Origin</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>expected</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOLUTION_CONFLICT&quot;)</code>；<code>e</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(hash&#91;:&#93;, digest)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(bindingRaw, binding) != nil &#124;&#124; !proto.Equal(binding, resolutionPreviewRequest(fixed)) &#124;&#124; value&#91;&quot;preview_cut_ref&quot;&#93; != fixed.Intent.Operation</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; (source&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(draftMainEpoch(fixed.Scope)) &amp;&amp; !(fixed.Scope.MainEpoch == 0 &amp;&amp; source&#91;&quot;workspace_epoch&quot;&#93; == nil))</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; source&#91;&quot;main_manifest_id&quot;&#93; != domain.TextID(fixed.Scope.Manifest) &#124;&#124; source&#91;&quot;structure_id&quot;&#93; != domain.TextID(fixed.Scope.Structure) &#124;&#124; source&#91;&quot;style_id&quot;&#93; != domain.TextID(fi …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>versions.Main != fixed.Versions.Main &#124;&#124; versions.Submitted != fixed.Versions.Submitted &#124;&#124; versions.Base != fixed.Versions.Base</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=671:2641 -->
````go
func (s *Resolutions) fixedPreview(ctx context.Context, tx *sql.Tx, raw []byte, fixed resolutionContext) (draftPrepared, error) {
	var expected draftPrepared
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	decoded, err := codec.Decode(raw)
	if err != nil {
		return expected, err
	}
	value, ok := decoded.(map[string]any)
	if !ok {
		return expected, unknown
	}
	digestText, ok := value["input_digest"].(string)
	if !ok {
		return expected, unknown
	}
	digest, err := base64.StdEncoding.DecodeString(digestText)
	if err != nil || len(digest) != 32 {
		return expected, unknown
	}
	delete(value, "input_digest")
	canonical, err := codec.Canonical(value)
	if err != nil {
		return expected, err
	}
	hash := sha256.Sum256(canonical)
	if !bytes.Equal(hash[:], digest) {
		return expected, unknown
	}
	bindingRaw, _ := json.Marshal(value["binding"])
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(bindingRaw, binding) != nil || !proto.Equal(binding, resolutionPreviewRequest(fixed)) || value["preview_cut_ref"] != fixed.Intent.Operation {
		return expected, unknown
	}
	source, ok := value["source"].(map[string]any)
	if !ok || (source["workspace_epoch"] != domain.TextID(draftMainEpoch(fixed.Scope)) && !(fixed.Scope.MainEpoch == 0 && source["workspace_epoch"] == nil)) {
		return expected, unknown
	}
	if !ok || source["main_manifest_id"] != domain.TextID(fixed.Scope.Manifest) || source["structure_id"] != domain.TextID(fixed.Scope.Structure) || source["style_id"] != domain.TextID(fixed.Scope.Style) || source["control_revision"] != domain.TextID(fixed.Scope.Control) || source["room_id"] != domain.TextID(fixed.Scope.Room) {
		return expected, unknown
	}
	original, versions, err := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
	if err != nil {
		return expected, err
	}
	if versions.Main != fixed.Versions.Main || versions.Submitted != fixed.Versions.Submitted || versions.Base != fixed.Versions.Base {
````

## 第 3 步：Resolutions.fixedPreview（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 70 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return expected, unknown</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>e1 != nil &#124;&#124; e2 != nil &#124;&#124; !bytes.Equal(left, right)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(cutRaw, &amp;expected.Cut) != nil &#124;&#124; expected.Cut&#91;&quot;room_id&quot;&#93; != domain.TextID(fixed.Scope.Room) &#124;&#124; expected.Cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(fixed.Scope.Workspace.Epoch)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>value&#91;&quot;candidate&quot;&#93; == nil</code>。条件成立时执行 <code>return expected, domain.Violation(domain.StateConflict, &quot;RESOLUTION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return expected, e</code>。当前函数在这里结束。
- 检查 <code>content.OperationId != fixed.Intent.Operation</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>key == &quot;main&quot;</code>。
- 检查 <code>content.BranchId != 0 &#124;&#124; content.WorkspaceEpoch != fixed.Scope.Control</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2641:4296 -->
````go
		return expected, unknown
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return expected, err
	}
	wanted := map[string]any{}
	if err = resolutionSourceFields(wanted, fixed, original, baseline); err != nil {
		return expected, err
	}
	for key, expectedValue := range wanted {
		left, e1 := codec.Canonical(source[key])
		right, e2 := codec.Canonical(expectedValue)
		if e1 != nil || e2 != nil || !bytes.Equal(left, right) {
			return expected, unknown
		}
	}
	expected.Preview, expected.Digest = fixed.Intent.Operation, digest
	cutRaw, _ := json.Marshal(value["draft_cut"])
	if json.Unmarshal(cutRaw, &expected.Cut) != nil || expected.Cut["room_id"] != domain.TextID(fixed.Scope.Room) || expected.Cut["workspace_epoch"] != domain.TextID(fixed.Scope.Workspace.Epoch) {
		return expected, unknown
	}
	if _, err = domain.CanonicalRevision(expected.Cut["durable_seq"]); err != nil {
		return expected, unknown
	}
	if value["candidate"] == nil {
		return expected, domain.Violation(domain.StateConflict, "RESOLUTION_CONFLICT")
	}
	for _, key := range []string{"candidate", "main", "draft"} {
		part, _ := json.Marshal(value[key])
		content, e := decodePullFixedContent(part)
		if e != nil {
			return expected, e
		}
		if e = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); e != nil {
			return expected, e
		}
		if content.OperationId != fixed.Intent.Operation {
			return expected, unknown
		}
		if key == "main" {
			if content.BranchId != 0 || content.WorkspaceEpoch != fixed.Scope.Control {
				return expected, unknown
			}
			expected.Main = content
		} else {
````

## 第 4 步：Resolutions.fixedPreview（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 116 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>content.BranchId != fixed.Scope.Workspace.Workspace &#124;&#124; content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch &#124;&#124; content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>key == &quot;candidate&quot;</code>。
- 检查 <code>content.CaptureId != fixed.Intent.Operation+&quot;:candidate&quot;</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return expected, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=4296:4825 -->
````go
			if content.BranchId != fixed.Scope.Workspace.Workspace || content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage) {
				return expected, unknown
			}
			if _, e = resolutionBody(s.validator, content.BodyJsonUtf8); e != nil {
				return expected, unknown
			}
			if key == "candidate" {
				if content.CaptureId != fixed.Intent.Operation+":candidate" {
					return expected, unknown
				}
				expected.Candidate = content
			}
		}
	}
	return expected, nil
}

````

## 第 5 步：Resolutions.persistFixed

插入位置：第 4 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func (s *Resolutions) persistFixed(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>lockResolutionOperations(ctx, tx, fixed)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;RESOLUTION_FIXED&quot;, task.Operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>task.Operation</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>lockResolutionOperations(ctx, tx, fixed)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code>；<code>repository.InsertSubmissionTask(ctx, tx, &quot;RESOLUTION_FIXED&quot;, task.Operation, task.Operation, raw, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_CHANGED&quot;)</code>；<code>e</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>e1 != nil &#124;&#124; e2 != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>e1 != nil &#124;&#124; e2 != nil &#124;&#124; !bytes.Equal(a, b)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>e == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return e</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=4825:5935 -->
````go
func (s *Resolutions) persistFixed(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, raw []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if prior, e := repository.MergeArtifact(ctx, tx, "RESOLUTION_FIXED", task.Operation); e == nil {
		left, e1 := codec.Decode(prior)
		right, e2 := codec.Decode(raw)
		if e1 != nil || e2 != nil {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
		}
		a, e1 := codec.Canonical(left)
		b, e2 := codec.Canonical(right)
		if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_CHANGED")
		}
	} else if e == sql.ErrNoRows {
		if err = repository.InsertSubmissionTask(ctx, tx, "RESOLUTION_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	} else {
		return e
	}
	return tx.Commit()
}

````

## 第 6 步：Resolutions.compute

插入位置：第 5 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func (s *Resolutions) compute(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext) (draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.client.Preview(ctx, request)</code>；<code>s.client.FixedDraft(ctx, request)</code>；<code>s.persistFixed(ctx, task, fixed, raw)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.persistFixed(ctx, task, fixed, raw)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>resolutionPreviewRequest(fixed)</code>；<code>fixed.Intent</code>；<code>fixed.Scope</code>；<code>s.persistFixed(ctx, task, fixed, raw)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>draftPrepared{}</code>；<code>expected</code>；<code>prepared</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>resolutionPreviewError(err)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREVIEW_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_UNKNOWN&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return draftPrepared{}, resolutionPreviewError(err)</code>。当前函数在这里结束。
- 检查 <code>preview == nil &#124;&#124; preview.State != pb.CaptureReply_FIXED &#124;&#124; preview.PreviewCutRef != fixed.Intent.Operation &#124;&#124; preview.MainManifestId != fixed.Scope.Manifest</code>。条件成立时执行 <code>return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREVIEW_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return draftPrepared{}, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return draftPrepared{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;parts) != nil</code>。条件成立时执行 <code>return expected, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !proto.Equal(draft, preview.Draft) &#124;&#124; !bytes.Equal(preview.InputDigest, expected.Digest)</code>。条件成立时执行 <code>return expected, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREVIEW_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;parts) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;state&quot;&#93;, &amp;prepared.State) != nil</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=5935:7894 -->
````go
func (s *Resolutions) compute(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext) (draftPrepared, error) {
	request := resolutionPreviewRequest(fixed)
	preview, err := s.client.Preview(ctx, request)
	if err != nil {
		return draftPrepared{}, resolutionPreviewError(err)
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.PreviewCutRef != fixed.Intent.Operation || preview.MainManifestId != fixed.Scope.Manifest {
		return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREVIEW_UNKNOWN")
	}
	raw, err := s.client.FixedDraft(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	if err = s.persistFixed(ctx, task, fixed, raw); err != nil {
		return draftPrepared{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return draftPrepared{}, err
	}
	expected, err := s.fixedPreview(ctx, tx, raw, fixed)
	_ = tx.Rollback()
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	}
	draft, err := decodePullFixedContent(parts["draft"])
	if err != nil || !proto.Equal(draft, preview.Draft) || !bytes.Equal(preview.InputDigest, expected.Digest) {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREVIEW_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err = s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": expected.Preview, "expected_input_digest": expected.Digest, "expected_round_id": domain.TextID(fixed.Intent.Round)})
	if err != nil {
		return expected, err
	}
	var prepared draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &prepared.State) != nil {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
	}
````

## 第 7 步：Resolutions.compute（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 211 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>prepared.State == &quot;CHANGED&quot;</code>。
- 检查 <code>json.Unmarshal(parts&#91;&quot;code&quot;&#93;, &amp;prepared.Code) != nil &#124;&#124; prepared.Code == &quot;&quot;</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prepared, domain.Violation(domain.StateConflict, prepared.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prepared.State == &quot;CONFLICTED&quot;</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.StateConflict, &quot;RESOLUTION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>prepared.State != &quot;PREPARED&quot;</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return prepared, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(parts&#91;&quot;preview_cut_ref&quot;&#93;, &amp;prepared.Preview) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;input_digest&quot;&#93;, &amp;prepared.Digest) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;draft_cut&quot;&#93;, &amp;prepared …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CUT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prepared, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=7894:9464 -->
````go
	if prepared.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &prepared.Code) != nil || prepared.Code == "" {
			return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
		}
		return prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State == "CONFLICTED" {
		return prepared, domain.Violation(domain.StateConflict, "RESOLUTION_CONFLICT")
	}
	if prepared.State != "PREPARED" {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
	}
	prepared.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return prepared, err
	}
	prepared.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != expected.Preview || !bytes.Equal(prepared.Digest, expected.Digest) || !proto.Equal(prepared.Candidate, expected.Candidate) || !proto.Equal(prepared.Main, expected.Main) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARED_BINDING_UNKNOWN")
	}
	a, _ := codec.Canonical(prepared.Cut)
	b, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(a, b) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CUT_CHANGED")
	}
	return prepared, nil
}

// conflict_resolution_errors.go：保留原实现，按业务合并文件。

````

## 第 8 步：resolutionPreviewError

插入位置：第 7 步代码之后，紧接追加；完成稿约第 244 行。

只接纳已约定的确定拒绝。连接/超时、坏固定来源或未认识的RPC错误继续未知恢复，不能释放控制。

本步声明：`func resolutionPreviewError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>status.FromError(err)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOLUTION_CONTENT_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOLUTION_TRANSFER_LIMIT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>value.Code() == codes.InvalidArgument &amp;&amp; value.Message() == &quot;RESOLUTION_CONTENT_INVALID&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOLUTION_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Code() == codes.ResourceExhausted &amp;&amp; value.Message() == &quot;DISCARD_TRANSFER_LIMIT&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOLUTION_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=9464:10156 -->
````go
// 只接纳已约定的确定拒绝。连接/超时、坏固定来源或未认识的RPC错误继续未知恢复，不能释放控制。
func resolutionPreviewError(err error) error {
	value, ok := status.FromError(err)
	if !ok {
		return err
	}
	if value.Code() == codes.InvalidArgument && value.Message() == "RESOLUTION_CONTENT_INVALID" {
		return domain.Violation(domain.StateConflict, "RESOLUTION_CONTENT_INVALID")
	}
	if value.Code() == codes.ResourceExhausted && value.Message() == "DISCARD_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, "RESOLUTION_TRANSFER_LIMIT")
	}
	return err
}

// conflict_resolution_readiness.go：保留原实现，按业务合并文件。

````

## 第 9 步：validateResolutionContext

插入位置：第 8 步代码之后，紧接追加；完成稿约第 261 行。

本步声明：`func validateResolutionContext(fixed resolutionContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>fixed.Actor</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>c == nil &#124;&#124; c.Target == nil &#124;&#124; c.OperationId != fixed.Intent.Operation &#124;&#124; c.ActorId != fixed.Actor &#124;&#124; fixed.Actor == 0 &#124;&#124; fixed.SessionExpires &lt;= 0 &#124;&#124; c.IdentityAdmissionRef == &quot;&quot;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>scope.Document == 0 &#124;&#124; scope.Unit == 0 &#124;&#124; scope.Room == 0 &#124;&#124; scope.Manifest == 0 &#124;&#124; scope.Structure == 0 &#124;&#124; scope.Style == 0 &#124;&#124; scope.Control == 0 &#124;&#124; scope.Workspace.Lineage == 0 &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>fixed.Origin.Operation != fixed.Intent.Conflict &#124;&#124; fixed.Origin.Document != scope.Document &#124;&#124; fixed.Origin.Unit != scope.Unit &#124;&#124; fixed.Origin.Branch != scope.Workspace.Workspace &#124;&#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>fixed.Review.ID != fixed.Origin.Request &#124;&#124; fixed.Review.Round != fixed.Intent.Round &#124;&#124; fixed.Review.Document != scope.Document &#124;&#124; fixed.Review.Unit != scope.Unit &#124;&#124; fixed.Review.Br …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>d.Id != domain.TextID(scope.Document) &#124;&#124; d.CurrentManifestId != domain.TextID(scope.Manifest) &#124;&#124; d.CurrentStructureId != domain.TextID(scope.Structure) &#124;&#124; d.CurrentStyleId != domai …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, c.RequestFingerprint)</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != 0</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != fixed.Actor &#124;&#124; proof.OperationId != fixed.Intent.Operation &#124;&#124; proof.Purpose != &quot;CONFLICT_RESOLUTION&quot; &#124;&#124; !proto.Equal(proof.Target, target) &#124;&#124; len(p …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != source.Epoch</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>fixed.Access.Proofs&#91;0&#93;.AdmissionId != c.IdentityAdmissionRef</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=10156:13641 -->
````go
func validateResolutionContext(fixed resolutionContext) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	c, scope := fixed.Context, fixed.Scope
	if c == nil || c.Target == nil || c.OperationId != fixed.Intent.Operation || c.ActorId != fixed.Actor || fixed.Actor == 0 || fixed.SessionExpires <= 0 || c.IdentityAdmissionRef == "" || c.DocumentAdmissionRef == "" || len(c.RequestFingerprint) != 32 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Edit[scope.Unit] {
		return unknown
	}
	target := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "CONFLICT_RESOLUTION"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Lineage == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || fixed.BaselineVersion == 0 || !proto.Equal(target, c.Target) || !equalPullSources(c.Sources, fixed.Access.Sources) {
		return unknown
	}
	if fixed.Origin.Operation != fixed.Intent.Conflict || fixed.Origin.Document != scope.Document || fixed.Origin.Unit != scope.Unit || fixed.Origin.Branch != scope.Workspace.Workspace || fixed.Origin.Round != fixed.Intent.Round || fixed.Versions.Attempt != fixed.Intent.Attempt || fixed.Versions.Base == 0 || fixed.Versions.Submitted == 0 || fixed.Versions.Main == 0 || !bytes.Equal(fixed.Versions.Digest, fixed.Intent.Digest) {
		return unknown
	}
	if fixed.Review.ID != fixed.Origin.Request || fixed.Review.Round != fixed.Intent.Round || fixed.Review.Document != scope.Document || fixed.Review.Unit != scope.Unit || fixed.Review.Branch != scope.Workspace.Workspace || fixed.Review.Status != "WITHDRAWN" || fixed.Review.Archived || fixed.Guard.Ongoing != 0 || fixed.Guard.Resumable != fixed.Review.ID || fixed.Guard.Submission != "" {
		return unknown
	}
	d := fixed.Access.Document
	if d.Id != domain.TextID(scope.Document) || d.CurrentManifestId != domain.TextID(scope.Manifest) || d.CurrentStructureId != domain.TextID(scope.Structure) || d.CurrentStyleId != domain.TextID(scope.Style) || d.ControlRevision != domain.TextID(scope.Control) {
		return unknown
	}
	path := "/v1/conflict-contexts/" + fixed.Intent.Conflict + "/resolutions"
	fingerprint, err := domain.ConflictResolutionFingerprint(path, fixed.Intent)
	if err != nil || !bytes.Equal(fingerprint[:], c.RequestFingerprint) {
		return unknown
	}
	available := map[string]uint64{}
	for _, source := range c.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return unknown
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fixed.Intent.Operation || proof.Purpose != "CONFLICT_RESOLUTION" || !proto.Equal(proof.Target, target) || len(proof.SessionRef) != 32 || len(proof.Sources) == 0 || proof.ExpiresUnixMs <= 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return unknown
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return unknown
			}
		}
	}
	if fixed.Access.Proofs[0].AdmissionId != c.IdentityAdmissionRef {
		return unknown
	}
````

## 第 10 步：validateResolutionContext（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 306 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=13641:13656 -->
````go
	return nil
}

````

## 第 11 步：validatePendingResolutionBinding

插入位置：第 10 步代码之后，紧接追加；完成稿约第 309 行。

本步声明：`func validatePendingResolutionBinding(fact repository.PendingResolutionBinding) (resolutionContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingResolutionBinding</code> | 使用 <code>repository.PendingResolutionBinding</code> 类型；其定义处的职责：保存人工冲突修复的独立支线控制、固定候选、新lineage与安装状态。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Operation</code>；<code>fact.Actor</code>；<code>fact.Owner</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>resolutionContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Operation != fact.Operation &#124;&#124; fixed.Actor != fact.Actor &#124;&#124; fact.Owner != fact.Operation &#124;&#124; !bytes.Equal(fixed.Context.RequestFingerprint, fact.Fingerprint) &#124;&#124; json.Un …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;RESOLUTION_RECOVER&quot;</code>。
- 检查 <code>e != nil &#124;&#124; fact.State != &quot;PREPARING&quot; &#124;&#124; !sameResolutionContext(payload, fixed)</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;RESOLUTION_RECONCILE&quot;</code>。
- 检查 <code>(fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;) &#124;&#124; !bytes.Equal(fact.Payload, fact.Committed)</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>decodeError != nil &#124;&#124; !proto.Equal(install.Context, fixed.Context) &#124;&#124; install.Branch != fixed.Scope.Workspace.Workspace &#124;&#124; install.OldRoom != fixed.Scope.Room &#124;&#124; install.Preview != fact.Operation</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>e != nil &#124;&#124; !sameResolutionContext(original, fixed)</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot; &amp;&amp; (install.NewRoom == 0 &#124;&#124; install.NewRoom == fixed.Scope.Room &#124;&#124; install.Epoch != fixed.Scope.Workspace.Epoch+1 &#124;&#124; install.Lineage == 0 &#124;&#124; install.Versi …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=13656:15697 -->
````go
func validatePendingResolutionBinding(fact repository.PendingResolutionBinding) (resolutionContext, error) {
	fixed, err := decodeResolutionContext(fact.Original)
	if err != nil {
		return fixed, err
	}
	if err = validateResolutionContext(fixed); err != nil {
		return fixed, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	var target struct {
		Path string `json:"path"`
	}
	if fixed.Intent.Operation != fact.Operation || fixed.Actor != fact.Actor || fact.Owner != fact.Operation || !bytes.Equal(fixed.Context.RequestFingerprint, fact.Fingerprint) || json.Unmarshal(fact.Target, &target) != nil || target.Path != "/v1/conflict-contexts/"+fixed.Intent.Conflict+"/resolutions" {
		return fixed, unknown
	}
	if fact.Kind == "RESOLUTION_RECOVER" {
		payload, e := decodeResolutionContext(fact.Payload)
		if e != nil || fact.State != "PREPARING" || !sameResolutionContext(payload, fixed) {
			return fixed, unknown
		}
	} else if fact.Kind == "RESOLUTION_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, unknown
		}
		install, decodeError := decodeResolutionInstall(fact.Payload)
		if decodeError != nil || !proto.Equal(install.Context, fixed.Context) || install.Branch != fixed.Scope.Workspace.Workspace || install.OldRoom != fixed.Scope.Room || install.Preview != fact.Operation {
			return fixed, unknown
		}
		raw, _ := json.Marshal(install.Fixed)
		original, e := decodeResolutionContext(raw)
		if e != nil || !sameResolutionContext(original, fixed) {
			return fixed, unknown
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == fixed.Scope.Room || install.Epoch != fixed.Scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Version == 0 || install.Content == nil || install.Content.OperationId != fact.Operation || install.Content.CaptureId != fact.Operation+":candidate") {
			return fixed, unknown
		}
	} else {
		return fixed, unknown
	}
	return fixed, nil
}

````

## 第 12 步：Resolutions.Ready

插入位置：第 11 步代码之后，紧接追加；完成稿约第 351 行。

本步声明：`func (s *Resolutions) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingResolutionBindings(ctx, s.db, &quot;&quot;)</code>；<code>repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>e</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ADMISSION_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>e != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>e == nil &amp;&amp; !reflect.DeepEqual(versions, fixed.Versions)</code>。
- 检查 <code>e == nil &amp;&amp; fact.Kind == &quot;RESOLUTION_RECONCILE&quot; &amp;&amp; fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>decodeError != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=15697:17211 -->
````go
func (s *Resolutions) Ready(ctx context.Context) error {
	facts, err := repository.PendingResolutionBindings(ctx, s.db, "")
	if err != nil {
		return err
	}
	for _, fact := range facts {
		fixed, e := validatePendingResolutionBinding(fact)
		if e != nil {
			return e
		}
		actual, e := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if e != nil {
			return e
		}
		if e = validateResolutionAdmission(fixed, actual); e != nil {
			return e
		}
		sources, e := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if e != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
		}
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		_, versions, e := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
		if e == nil && !reflect.DeepEqual(versions, fixed.Versions) {
			e = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSIONS_UNKNOWN")
		}
		if e == nil && fact.Kind == "RESOLUTION_RECONCILE" && fact.State == "SUCCEEDED" {
			install, decodeError := decodeResolutionInstall(fact.Payload)
			if decodeError != nil {
				e = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
			} else {
				e = s.validateInstallSource(ctx, tx, install)
			}
		}
		_ = tx.Rollback()
		if e != nil {
			return e
		}
	}
	return nil
}

// conflict_resolution_readings.go：保留原实现，按业务合并文件。

````

## 第 13 步：Resolutions.Progress

插入位置：第 12 步代码之后，紧接追加；完成稿约第 398 行。

本步声明：`func (s *Resolutions) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.access.Authorize(ctx, actor, fixed.Scope.Document, &quot;CONFLICT_RESOLUTION_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fixed.Scope.Document, &quot;CONFLICT_RESOLUTION_RESULT&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>repository.ResolutionResult(ctx, tx, operation)</code>；<code>repository.ResolutionInstalled(ctx, tx, operation)</code>；<code>&quot;operation_id&quot;: operation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		receipt, err := repository.Reso …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;RESOLUTION_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>actor.ID != fixed.Actor</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;RESOLUTION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;fixed.Scope.Unit&#93;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		receipt, …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>receipt.Actor != actor.ID &#124;&#124; !receipt.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;RESOLUTION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(receipt.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>receipt.State != &quot;PREPARING&quot; &amp;&amp; !installed</code>。条件成立时执行 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;WAITING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + operation}, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=17211:18734 -->
````go
func (s *Resolutions) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if actor.ID != fixed.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "RESOLUTION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Scope.Document, "CONFLICT_RESOLUTION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Edit[fixed.Scope.Unit] {
		return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		receipt, err := repository.ResolutionResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if receipt.Actor != actor.ID || !receipt.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "RESOLUTION_NOT_FOUND")
		}
		var value map[string]any
		if json.Unmarshal(receipt.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
		}
		installed, err := repository.ResolutionInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if receipt.State != "PREPARING" && !installed {
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

````

## 第 14 步：Resolutions.OperationResult

插入位置：第 13 步代码之后，紧接追加；完成稿约第 439 行。

本步声明：`func (s *Resolutions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, query.OperationId)</code>；<code>repository.ResolutionResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;RESOLUTION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;RESOLUTION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor &#124;&#124; !bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=18734:20650 -->
````go
func (s *Resolutions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "RESOLUTION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.ResolutionResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, State: pb.OperationResult_PROCESSING, Generation: fact.Generation}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// conflict_resolution_source.go：保留原实现，按业务合并文件。

````

## 第 15 步：validateResolutionAdmission

插入位置：第 14 步代码之后，紧接追加；完成稿约第 484 行。

本步声明：`func validateResolutionAdmission(fixed resolutionContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Context</code>；<code>fixed.Actor</code>；<code>fixed.Scope</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil &#124;&#124; fixed.Context == nil</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Scope.Document &#124;&#124; actual.Unit != fixed.Scope.Unit &#124;&#124; actual.Identity != fixed.Context.IdentityAdmissionRef &#124;&#124; actual.Operati …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=20650:21492 -->
````go
func validateResolutionAdmission(fixed resolutionContext, actual repository.OriginalAdmissionBinding) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	if len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || fixed.Context == nil {
		return unknown
	}
	target := &pb.Target{}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "CONFLICT_RESOLUTION" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return unknown
	}
	return nil
}

````

## 第 16 步：Resolutions.resume

插入位置：第 15 步代码之后，紧接追加；完成稿约第 496 行。

本步声明：`func (s *Resolutions) resume(ctx context.Context, fixed resolutionContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=21492:22000 -->
````go
func (s *Resolutions) resume(ctx context.Context, fixed resolutionContext) ([]*pb.IdentityAdmission, error) {
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

## 第 17 步：Resolutions.validate

插入位置：第 16 步代码之后，紧接追加；完成稿约第 511 行。

本步声明：`func (s *Resolutions) validate(ctx context.Context, tx *sql.Tx, fixed resolutionContext, proofs []*pb.IdentityAdmission) (mergeFixedInputs, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>fixed.Origin</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 实际返回表达式：<code>empty</code>；<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Edit&#91;scope.Unit&#93;</code>。条件成立时执行 <code>return empty, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(scope.Manifest) &#124;&#124; current.CurrentStructureId != domain.TextID(scope.Structure) &#124;&#124; current.CurrentStyleId != domain.TextID(scope.Style) &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; active != scope.Workspace.Workspace</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; workspace != scope.Workspace</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;WORKSPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>owner != fixed.Intent.Operation</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>pointer != fixed.Review &#124;&#124; guard != fixed.Guard</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;RESOLUTION_REVIEW_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=22000:24013 -->
````go
func (s *Resolutions) validate(ctx context.Context, tx *sql.Tx, fixed resolutionContext, proofs []*pb.IdentityAdmission) (mergeFixedInputs, error) {
	var empty mergeFixedInputs
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return empty, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return empty, err
	}
	scope := fixed.Scope
	if !fixed.Access.Permissions.Edit[scope.Unit] {
		return empty, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	if current.CurrentManifestId != domain.TextID(scope.Manifest) || current.CurrentStructureId != domain.TextID(scope.Structure) || current.CurrentStyleId != domain.TextID(scope.Style) || current.ControlRevision != domain.TextID(scope.Control) {
		return empty, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, scope.Document)
	if err != nil {
		return empty, err
	}
	if barrier.Phase != "OPEN" {
		return empty, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)
	if err != nil || active != scope.Workspace.Workspace {
		return empty, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, active, scope.Document, scope.Unit)
	if err != nil || workspace != scope.Workspace {
		return empty, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	owner, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return empty, err
	}
	if owner != fixed.Intent.Operation {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	pointer, guard, err := resolutionReview(ctx, tx, fixed.Origin, fixed.Intent.Round)
	if err != nil {
		return empty, err
	}
	if pointer != fixed.Review || guard != fixed.Guard {
		return empty, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
````

## 第 18 步：Resolutions.validate（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 556 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return empty, domain.Violation(domain.PermissionDenied, &quot;RESOLUTION_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.TemporarilyUnavailable, &quot;RESOLUTION_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; !reflect.DeepEqual(versions, fixed.Versions)</code>。
- 此处通过 <code>return original, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=24013:25297 -->
````go
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return empty, err
	}
	if err = validateResolutionAdmission(fixed, actual); err != nil {
		return empty, err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return empty, err
	}
	if state.State == "CLOSED" {
		return empty, domain.Violation(domain.PermissionDenied, "RESOLUTION_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs); err != nil {
		return empty, err
	}
	original, versions, err := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
	if err == nil && !reflect.DeepEqual(versions, fixed.Versions) {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSIONS_UNKNOWN")
	}
	return original, err
}

````

## 第 19 步：resolutionSourceFields

插入位置：第 18 步代码之后，紧接追加；完成稿约第 587 行。

本步声明：`func resolutionSourceFields(value map[string]any, fixed resolutionContext, original mergeFixedInputs, baseline *pb.FixedContent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>value&#91;&quot;purpose&quot;&#93; = &quot;CONFLICT_RESOLUTION&quot;</code>；<code>value&#91;&quot;conflict_ref&quot;&#93;, value&#91;&quot;original_attempt_id&quot;&#93;, value&#91;&quot;original_input_digest&quot;&#93;, value&#91;&quot;expected_round_id&quot;&#93; = fixed.Intent.Conflict, domain.ConflictAttemptRef(fixed.Intent.Atte …</code>（长表达式，按本步完整代码继续）；<code>value&#91;&quot;resolution_content&quot;&#93;, value&#91;&quot;resolution_submitted&quot;&#93;, value&#91;&quot;resolution_baseline&quot;&#93; = body, submitted, base</code>；<code>value&#91;&quot;resolution_base_version_id&quot;&#93;, value&#91;&quot;resolution_base_manifest_id&quot;&#93; = domain.TextID(fixed.BaselineVersion), domain.TextID(original.Attempt.Main.ExpectedManifestId)</code> |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>fixed.BaselineVersion</code> |
| <code>original</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>original.Input</code>；<code>original.Attempt</code> |
| <code>baseline</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>fixedWire(baseline)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=25297:26271 -->
````go
func resolutionSourceFields(value map[string]any, fixed resolutionContext, original mergeFixedInputs, baseline *pb.FixedContent) error {
	submitted, err := fixedWire(original.Input.Submitted)
	if err != nil {
		return err
	}
	base, err := fixedWire(baseline)
	if err != nil {
		return err
	}
	body, err := codec.Decode(fixed.Intent.Body)
	if err != nil {
		return err
	}
	value["purpose"] = "CONFLICT_RESOLUTION"
	value["conflict_ref"], value["original_attempt_id"], value["original_input_digest"], value["expected_round_id"] = fixed.Intent.Conflict, domain.ConflictAttemptRef(fixed.Intent.Attempt), fixed.Intent.Input.ExpectedInputDigest, domain.TextID(fixed.Intent.Round)
	value["resolution_content"], value["resolution_submitted"], value["resolution_baseline"] = body, submitted, base
	value["resolution_base_version_id"], value["resolution_base_manifest_id"] = domain.TextID(fixed.BaselineVersion), domain.TextID(original.Attempt.Main.ExpectedManifestId)
	return nil
}

````

## 第 20 步：Resolutions.Source

插入位置：第 19 步代码之后，紧接追加；完成稿约第 607 行。

本步声明：`func (s *Resolutions) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>lockResolutionOperations(ctx, tx, fixed)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(input, resolutionPreviewRequest(fixed))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;RESOLUTION_SOURCE_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;RESOLUTION_SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input, resolutionPreviewRequest(fixed))</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=26271:27466 -->
````go
func (s *Resolutions) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "RESOLUTION_SOURCE_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, resolutionPreviewRequest(fixed)) {
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
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return nil, err
	}
	original, err := s.validate(ctx, tx, fixed, proofs)
	if err != nil {
		return nil, err
	}
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err != nil {
		return nil, err
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return nil, err
	}
	if err = resolutionSourceFields(value, fixed, original, baseline); err != nil {
		return nil, err
	}
	return value, nil
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

````

## 第 21 步：Resolutions

插入位置：第 20 步代码之后，紧接追加；完成稿约第 650 行。

本步声明：`type Resolutions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>DraftControlClient</code> | 使用 <code>DraftControlClient</code> 类型；其定义处的职责：Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 21 bytes=27466:27630 -->
````go
type Resolutions struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

````

## 第 22 步：resolutionContext

插入位置：第 21 步代码之后，紧接追加；完成稿约第 659 行。

本步声明：`type resolutionContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 发起这次冲突解决的账号。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |
| <code>Intent</code> | <code>domain.ConflictResolutionIntent</code> | 使用 <code>domain.ConflictResolutionIntent</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。 |
| <code>Origin</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。 |
| <code>Versions</code> | <code>conflictVersions</code> | 冲突尝试绑定的基线、提交、主线版本及输入摘要。 |
| <code>BaselineVersion</code> | <code>uint64</code> | 解决后将采用的基线正文版本。 |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Scope</code> | <code>draftSourceScope</code> | 原支线、房间及主线结构上下文，安装时继续核对。 |
| <code>Review</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。 |
| <code>Guard</code> | <code>domain.ReviewGuard</code> | 使用 <code>domain.ReviewGuard</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。 |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。 |

<!-- write-step: 22 bytes=27630:28031 -->
````go
type resolutionContext struct {
	Actor           uint64
	SessionExpires  int64
	Intent          domain.ConflictResolutionIntent
	Origin          domain.MergeFact
	Versions        conflictVersions
	BaselineVersion uint64
	Access          DocumentAccess
	Scope           draftSourceScope
	Review          domain.ReviewPointer
	Guard           domain.ReviewGuard
	Context         *pb.OperationContext
}

````

## 第 23 步：resolutionInstall

插入位置：第 22 步代码之后，紧接追加；完成稿约第 673 行。

本步声明：`type resolutionInstall struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>draftInstall</code> | <code>draftInstall</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>Fixed</code> | <code>resolutionContext</code> | 已固定的冲突解决业务上下文，安装恢复不能用新请求替换。 |
| <code>Version</code> | <code>uint64</code> | 本次解决产出的固定正文版本。 |

<!-- write-step: 23 bytes=28031:28123 -->
````go
type resolutionInstall struct {
	draftInstall
	Fixed   resolutionContext
	Version uint64
}

````

## 第 24 步：NewResolutions

插入位置：第 23 步代码之后，紧接追加；完成稿约第 679 行。

本步声明：`func NewResolutions(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *Resolutions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>DraftControlClient</code> | 使用 <code>DraftControlClient</code> 类型；其定义处的职责：Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Resolutions</code> | 实际返回表达式：<code>&amp;Resolutions{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Resolutions{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=28123:28404 -->
````go
func NewResolutions(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *Resolutions {
	return &Resolutions{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

````

## 第 25 步：decodeResolutionContext

插入位置：第 24 步代码之后，紧接追加；完成稿约第 683 行。

本步声明：`func decodeResolutionContext(raw []byte) (resolutionContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;fixed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>resolutionContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>validateResolutionContext(fixed)</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.Target.Purpose != &quot;CONFLICT_RESOLUTION&quot; &#124;&#124; fixed.Intent.Operation != fixe …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; intent.Operation != fixed.Intent.Operation &#124;&#124; intent.Attempt != fixed.Intent.Attempt &#124;&#124; intent.Round != fixed.Intent.Round &#124;&#124; !bytes.Equal(intent.Digest, fixed.Intent …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, validateResolutionContext(fixed)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=28404:29461 -->
````go
func decodeResolutionContext(raw []byte) (resolutionContext, error) {
	var fixed resolutionContext
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.Target.Purpose != "CONFLICT_RESOLUTION" || fixed.Intent.Operation != fixed.Context.OperationId || fixed.Origin.Operation != fixed.Intent.Conflict {
		return fixed, unknown
	}
	intent, err := domain.ParseConflictResolution(fixed.Intent.Conflict, fixed.Intent.Input)
	if err != nil || intent.Operation != fixed.Intent.Operation || intent.Attempt != fixed.Intent.Attempt || intent.Round != fixed.Intent.Round || !bytes.Equal(intent.Digest, fixed.Intent.Digest) || !reflect.DeepEqual(intent.Conflicts, fixed.Intent.Conflicts) {
		return fixed, unknown
	}
	// 正文只持久化在原HTTP意图里，避免另存一份可独立漂移的大正文；读取时从同一意图规范派生。
	fixed.Intent = intent
	return fixed, validateResolutionContext(fixed)
}

````

## 第 26 步：Resolutions.Lookup

插入位置：第 25 步代码之后，紧接追加；完成稿约第 698 行。

本步声明：`func (s *Resolutions) Lookup(ctx context.Context, operation string) (resolutionContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;RESOLUTION_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;RESOLUTION_CONTEXT&quot;, operation)</code>；<code>fixed.Context.OperationId != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>resolutionContext</code> | 实际返回表达式：<code>resolutionContext{}</code>；<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return resolutionContext{}, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; fixed.Context.OperationId != operation</code>。
- 此处通过 <code>return fixed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=29461:29908 -->
````go
func (s *Resolutions) Lookup(ctx context.Context, operation string) (resolutionContext, error) {
	raw, err := repository.MergeArtifact(ctx, s.db, "RESOLUTION_CONTEXT", operation)
	if err != nil {
		return resolutionContext{}, err
	}
	fixed, err := decodeResolutionContext(raw)
	if err == nil && fixed.Context.OperationId != operation {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	return fixed, err
}

````

## 第 27 步：resolutionPreviewRequest

插入位置：第 26 步代码之后，紧接追加；完成稿约第 710 行。

本步声明：`func resolutionPreviewRequest(fixed resolutionContext) *pb.DraftPreviewRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftPreviewRequest</code> | 实际返回表达式：<code>&amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=29908:30193 -->
````go
func resolutionPreviewRequest(fixed resolutionContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
}

````

## 第 28 步：sameResolutionContext

插入位置：第 27 步代码之后，紧接追加；完成稿约第 714 行。

本步声明：`func sameResolutionContext(left, right resolutionContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>json.Marshal(left)</code> |
| <code>right</code> | <code>resolutionContext</code> | 在此声明中的实际使用：<code>json.Marshal(right)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>first == nil &amp;&amp; second == nil &amp;&amp; bytes.Equal(a, b)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return first == nil &amp;&amp; second == nil &amp;&amp; bytes.Equal(a, b)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=30193:30589 -->
````go
func sameResolutionContext(left, right resolutionContext) bool {
	// protobuf比较会初始化消息内部缓存；这些字段不参与已持久化协议。
	// 比较完整协议编码，保留所有业务/来源字段，避免reflect把缓存状态当成篡改。
	a, first := json.Marshal(left)
	b, second := json.Marshal(right)
	return first == nil && second == nil && bytes.Equal(a, b)
}

````

## 第 29 步：decodeResolutionInstall

插入位置：第 28 步代码之后，紧接追加；完成稿约第 722 行。

本步声明：`func decodeResolutionInstall(raw []byte) (resolutionInstall, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>resolutionInstall</code> | 实际返回表达式：<code>install</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return install, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=30589:30932 -->
````go
func decodeResolutionInstall(raw []byte) (resolutionInstall, error) {
	var install resolutionInstall
	// 跨库JSON中的source_range和标题级别是规范整数；默认float64解码无法再进入canonical-v1。
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&install)
	return install, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/conflict_resolutions_preparation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
