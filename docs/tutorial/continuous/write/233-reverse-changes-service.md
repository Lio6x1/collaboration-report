# 分步编写 `services/document/internal/logic/document/reverse_changes_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/233-reverse-changes-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。

反向不是回退主线；应保留当时已兼容的可靠MAIN尾部和之后独立修改。

写之前先按这条执行路线理解各段的连接：

1. original读取原可靠MAIN采样与合并结果
2. compute生成固定反向候选
3. commit追加版本/lineage/审核轮次及理由
4. validateReviewSource交叉核不可变轮次
5. reconcile核安装。

本文件需要守住的边界：

- 不能使用input_manifest中较旧版本当合并前正文
- 两份恢复payload同版本号不构成真实证据。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/reverse_changes_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/reverse_changes_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/reverse_changes_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/reverse_changes_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:638 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
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
	"reflect"
	"sort"
	"time"
)

// reverse_changes.go：保留原实现，按业务合并文件。

````

## 第 2 步：ReverseChanges

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`type ReverseChanges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>DraftControlClient</code> | 使用 <code>DraftControlClient</code> 类型；其定义处的职责：Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 2 bytes=638:805 -->
````go
type ReverseChanges struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

````

## 第 3 步：reverseContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`type reverseContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 发起反向变更的账号。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |
| <code>Intent</code> | <code>domain.ReverseChangeIntent</code> | 使用 <code>domain.ReverseChangeIntent</code> 类型；其定义处的职责：反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。 |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。 |
| <code>Scope</code> | <code>draftSourceScope</code> | 本次反向候选写入的当前支线范围。 |
| <code>Record</code> | <code>repository.ReverseMergeRecord</code> | 使用 <code>repository.ReverseMergeRecord</code> 类型；其定义处的职责：归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。 |
| <code>Before</code> | <code>*pb.FixedContent</code> | 原合并之前的固定正文。 |
| <code>After</code> | <code>*pb.FixedContent</code> | 原合并之后的固定正文，用于计算要反向抵消的变动。 |
| <code>GuardGeneration</code> | <code>uint64</code> | 为本次反向送审占用的审核守卫代次。 |

<!-- write-step: 3 bytes=805:1137 -->
````go
type reverseContext struct {
	Actor           uint64
	SessionExpires  int64
	Intent          domain.ReverseChangeIntent
	Access          DocumentAccess
	Context         *pb.OperationContext
	Scope           draftSourceScope
	Record          repository.ReverseMergeRecord
	Before, After   *pb.FixedContent
	GuardGeneration uint64
}

````

## 第 4 步：reverseInstall

插入位置：第 3 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`type reverseInstall struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>draftInstall</code> | <code>draftInstall</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>Fixed</code> | <code>reverseContext</code> | 序列化/校验标记：<code>`json:&quot;fixed&quot;`</code> |
| <code>Request</code> | <code>uint64</code> | 反向变更创建/关联的审核请求编号。 |
| <code>Round</code> | <code>uint64</code> | 反向变更生成的固定审核轮次。 |
| <code>Version</code> | <code>uint64</code> | 反向候选的固定正文版本，仍按审核流程处理。 |

<!-- write-step: 4 bytes=1137:1270 -->
````go
type reverseInstall struct {
	draftInstall
	Fixed                   reverseContext `json:"fixed"`
	Request, Round, Version uint64
}

````

## 第 5 步：NewReverseChanges

插入位置：第 4 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func NewReverseChanges(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *ReverseChanges`。


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
| <code>(匿名/未命名)</code> | <code>*ReverseChanges</code> | 实际返回表达式：<code>&amp;ReverseChanges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ReverseChanges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1270:1560 -->
````go
func NewReverseChanges(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *ReverseChanges {
	return &ReverseChanges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

````

## 第 6 步：ReverseChanges.lookup

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (s *ReverseChanges) lookup(ctx context.Context, operation string) (reverseContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;REVERSE_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;REVERSE_CONTEXT&quot;, operation)</code>；<code>fixed.Context.OperationId != operation</code>；<code>fixed.Intent.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>reverseContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.OperationId != operation &#124;&#124; fixed.Intent.Operation != operation &#124;&#124; fixed.Context.Target.Purpose != &quot;REVERSE&quot;</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1560:2140 -->
````go
func (s *ReverseChanges) lookup(ctx context.Context, operation string) (reverseContext, error) {
	var fixed reverseContext
	raw, err := repository.MergeArtifact(ctx, s.db, "REVERSE_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation || fixed.Context.Target.Purpose != "REVERSE" {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	return fixed, nil
}

````

## 第 7 步：reversePreviewRequest

插入位置：第 6 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func reversePreviewRequest(fixed reverseContext) *pb.DraftPreviewRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftPreviewRequest</code> | 实际返回表达式：<code>&amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedStructureId: fixed.Scope.Structure, ExpectedLineageId: fixed.Scope.Workspace.Lineage}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedStructureId: fixed.Scope.Structure, ExpectedLineageId: fixed.Scope.Workspace.Lineage}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2140:2486 -->
````go
func reversePreviewRequest(fixed reverseContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedStructureId: fixed.Scope.Structure, ExpectedLineageId: fixed.Scope.Workspace.Lineage}
}

// reverse_commit.go：保留原实现，按业务合并文件。

````

## 第 8 步：ReverseChanges.commit

插入位置：第 7 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func (s *ReverseChanges) commit(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 15 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteReverseTask(ctx, tx, task, &quot;SUCCEEDED&quot;, encoded)</code> |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code>；<code>fixed.Scope</code>；<code>fixed.Access</code>；<code>fixed.Actor</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>prepared</code> | <code>draftPrepared</code> | 在此声明中的实际使用：<code>prepared.Main</code>；<code>prepared.Candidate</code>；<code>prepared.Preview</code>；<code>prepared.Digest</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validate(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>e</code>；<code>domain.Violation(domain.StateConflict, &quot;REVERSE_RESOURCE_NOT_READY&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PROVENANCE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != nil</code>。条件成立时执行 <code>return e</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;REVERSE_RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=2486:4009 -->
````go
func (s *ReverseChanges) commit(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
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
			return domain.Violation(domain.StateConflict, "REVERSE_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	baseVersion, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
	if err != nil {
		return err
	}
````

## 第 9 步：ReverseChanges.commit（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 120 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PROVENANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=4009:6002 -->
````go
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
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PROVENANCE_UNKNOWN")
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"], provenance["source_operation_id"] = domain.TextID(lineage), domain.TextID(scope.Manifest), domain.TextID(baseVersion), task.Operation
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertReverseLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, scope.Manifest, baseVersion, task.Operation, prepared.Preview, raw); err != nil {
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
	version, err := repository.InsertMergeVersion(ctx, tx, content, "REVERSE", content.CaptureId)
	if err != nil {
		return err
	}
	request, err := repository.CreateReviewRequest(ctx, tx, scope.Document, scope.Unit, scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	workspace := scope.Workspace
	workspace.Epoch, workspace.Lineage, workspace.BaseManifest, workspace.BaseVersion = epoch, lineage, scope.Manifest, baseVersion
	round, err := repository.CreateSubmissionRound(ctx, tx, request, fixed.Actor, version, content, workspace)
````

## 第 10 步：ReverseChanges.commit（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 165 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=6002:8833 -->
````go
	if err != nil {
		return err
	}
	roundContext, err := json.Marshal(map[string]any{"style_id": domain.TextID(scope.Style), "document_title": fixed.Access.Document.Title, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_ROUND_CONTEXT", domain.TextID(round), task.Operation, roundContext, "SUCCEEDED"); err != nil {
		return err
	}
	// 服务器固定来源、真实submission_operation和review_round同TX；后续普通批准读取这份新代次零尾cut。
	reviewSource, err := json.Marshal(map[string]any{"operation_id": task.Operation, "merge_record_id": domain.TextID(fixed.Record.ID), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version), "capture_ref": content.CaptureId, "input_digest": prepared.Digest, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_REVIEW_SOURCE", task.Operation, task.Operation, reviewSource, "SUCCEEDED"); err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)}}
	if err = repository.CompleteSubmission(ctx, tx, task.Operation, fixed.Actor, scope.Unit, request, round, result); err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := reverseInstall{Fixed: fixed, Request: request, Round: round, Version: version, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content,
		LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(scope.Manifest), "base_version_id": domain.TextID(baseVersion), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.InsertReverseAudit(ctx, tx, task.Operation, fixed.Actor, fixed.Record.ID, request, round, version, fixed.Intent.Reason); err != nil {
		return err
	}
	notice, err := json.Marshal(map[string]string{"document_id": domain.TextID(scope.Document), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "kind": "SUBMISSION"})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
````

## 第 11 步：ReverseChanges.commit（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 210 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 11 bytes=8833:9032 -->
````go
	if err = repository.CompleteReverseTask(ctx, tx, task, "SUCCEEDED", encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_compute.go：保留原实现，按业务合并文件。

````

## 第 12 步：ReverseChanges.fixedPreview

插入位置：第 11 步代码之后，紧接追加；完成稿约第 218 行。

本步声明：`func (s *ReverseChanges) fixedPreview(raw []byte, fixed reverseContext) (draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>reversePreviewRequest(fixed)</code>；<code>fixed.Scope</code>；<code>fixed.Record</code>；<code>fixed.Intent</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>expected</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>domain.Violation(domain.StateConflict, &quot;REVERSE_CONFLICT&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;REVERSE_NO_CHANGES&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(hash&#91;:&#93;, digest)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(bindingRaw, binding) != nil &#124;&#124; !proto.Equal(binding, reversePreviewRequest(fixed))</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; (source&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(draftMainEpoch(fixed.Scope)) &amp;&amp; !(fixed.Scope.MainEpoch == 0 &amp;&amp; source&#91;&quot;workspace_epoch&quot;&#93; == nil))</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; source&#91;&quot;purpose&quot;&#93; != &quot;REVERSE&quot; &#124;&#124; source&#91;&quot;merge_record_id&quot;&#93; != domain.TextID(fixed.Record.ID) &#124;&#124; source&#91;&quot;reason&quot;&#93; != fixed.Intent.Reason &#124;&#124; source&#91;&quot;main_manifest_id&quot;&#93; != dom …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=9032:10880 -->
````go
func (s *ReverseChanges) fixedPreview(raw []byte, fixed reverseContext) (draftPrepared, error) {
	var expected draftPrepared
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_UNKNOWN")
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
	bindingRaw, err := json.Marshal(value["binding"])
	if err != nil {
		return expected, err
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(bindingRaw, binding) != nil || !proto.Equal(binding, reversePreviewRequest(fixed)) {
		return expected, unknown
	}
	source, ok := value["source"].(map[string]any)
	if !ok || (source["workspace_epoch"] != domain.TextID(draftMainEpoch(fixed.Scope)) && !(fixed.Scope.MainEpoch == 0 && source["workspace_epoch"] == nil)) {
		return expected, unknown
	}
	if !ok || source["purpose"] != "REVERSE" || source["merge_record_id"] != domain.TextID(fixed.Record.ID) || source["reason"] != fixed.Intent.Reason || source["main_manifest_id"] != domain.TextID(fixed.Scope.Manifest) || source["structure_id"] != domain.TextID(fixed.Scope.Structure) || source["control_revision"] != domain.TextID(fixed.Scope.Control) {
		return expected, unknown
	}
	for key, wanted := range map[string]*pb.FixedContent{"reverse_before": fixed.Before, "reverse_after": fixed.After} {
		part, err := json.Marshal(source[key])
````

## 第 13 步：ReverseChanges.fixedPreview（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 263 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !proto.Equal(content, wanted)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>value&#91;&quot;preview_cut_ref&quot;&#93; != fixed.Intent.Operation</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; json.Unmarshal(cutRaw, &amp;expected.Cut) != nil &#124;&#124; expected.Cut&#91;&quot;room_id&quot;&#93; != domain.TextID(fixed.Scope.Room) &#124;&#124; expected.Cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(fixed.Scope.Workspace.Epoch)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>value&#91;&quot;candidate&quot;&#93; == nil</code>。条件成立时执行 <code>return expected, domain.Violation(domain.StateConflict, &quot;REVERSE_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>content.OperationId != fixed.Intent.Operation</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>key == &quot;main&quot;</code>。
- 检查 <code>content.BranchId != 0 &#124;&#124; content.WorkspaceEpoch != fixed.Scope.Control</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>content.BranchId != fixed.Scope.Workspace.Workspace &#124;&#124; content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch &#124;&#124; content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage)</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。
- 检查 <code>key == &quot;candidate&quot;</code>。
- 检查 <code>content.CaptureId != fixed.Intent.Operation</code>。条件成立时执行 <code>return expected, unknown</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=10880:12599 -->
````go
		if err != nil {
			return expected, err
		}
		content, err := decodePullFixedContent(part)
		if err != nil || !proto.Equal(content, wanted) {
			return expected, unknown
		}
	}
	if value["preview_cut_ref"] != fixed.Intent.Operation {
		return expected, unknown
	}
	expected.Preview, expected.Digest = fixed.Intent.Operation, digest
	cutRaw, err := json.Marshal(value["draft_cut"])
	if err != nil || json.Unmarshal(cutRaw, &expected.Cut) != nil || expected.Cut["room_id"] != domain.TextID(fixed.Scope.Room) || expected.Cut["workspace_epoch"] != domain.TextID(fixed.Scope.Workspace.Epoch) {
		return expected, unknown
	}
	if value["candidate"] == nil {
		return expected, domain.Violation(domain.StateConflict, "REVERSE_CONFLICT")
	}
	for key := range map[string]bool{"candidate": true, "main": true, "draft": true} {
		part, err := json.Marshal(value[key])
		if err != nil {
			return expected, err
		}
		content, err := decodePullFixedContent(part)
		if err != nil {
			return expected, err
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); err != nil {
			return expected, err
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
			if content.BranchId != fixed.Scope.Workspace.Workspace || content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage) {
				return expected, unknown
			}
			if key == "candidate" {
				if content.CaptureId != fixed.Intent.Operation {
````

## 第 14 步：ReverseChanges.fixedPreview（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 308 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return expected, unknown</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>bytes.Equal(expected.Candidate.BodyHash, expected.Main.BodyHash)</code>。条件成立时执行 <code>return expected, domain.Violation(domain.StateConflict, &quot;REVERSE_NO_CHANGES&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return expected, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=12599:12859 -->
````go
					return expected, unknown
				}
				expected.Candidate = content
			}
		}
	}
	if bytes.Equal(expected.Candidate.BodyHash, expected.Main.BodyHash) {
		return expected, domain.Violation(domain.StateConflict, "REVERSE_NO_CHANGES")
	}
	return expected, nil
}

````

## 第 15 步：ReverseChanges.compute

插入位置：第 14 步代码之后，紧接追加；完成稿约第 320 行。

本步声明：`func (s *ReverseChanges) compute(ctx context.Context, task domain.SubmissionTask, fixed reverseContext) (draftPrepared, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.client.Preview(ctx, request)</code>；<code>s.client.FixedDraft(ctx, request)</code>；<code>s.persistFixed(ctx, task, raw)</code>；<code>s.client.Prepare(ctx, map&#91;string&#93;any{&quot;context&quot;: wire, &quot;expected_attempt_id&quot;: expected.Preview, &quot;expected_input_digest&quot;: expected.Digest})</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.persistFixed(ctx, task, raw)</code> |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>reversePreviewRequest(fixed)</code>；<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>s.fixedPreview(raw, fixed)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>draftPrepared</code> | 实际返回表达式：<code>draftPrepared{}</code>；<code>expected</code>；<code>prepared</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREVIEW_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREPARE_UNKNOWN&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return draftPrepared{}, err</code>。当前函数在这里结束。
- 检查 <code>preview == nil &#124;&#124; preview.State != pb.CaptureReply_FIXED &#124;&#124; preview.MainManifestId != fixed.Scope.Manifest &#124;&#124; preview.PreviewCutRef != fixed.Intent.Operation</code>。条件成立时执行 <code>return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREVIEW_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return draftPrepared{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;originalParts) != nil</code>。条件成立时执行 <code>return expected, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(preview.InputDigest, expected.Digest) &#124;&#124; !proto.Equal(preview.Draft, draft)</code>。条件成立时执行 <code>return expected, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return expected, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;parts) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;state&quot;&#93;, &amp;prepared.State) != nil</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=12859:14807 -->
````go
func (s *ReverseChanges) compute(ctx context.Context, task domain.SubmissionTask, fixed reverseContext) (draftPrepared, error) {
	request := reversePreviewRequest(fixed)
	preview, err := s.client.Preview(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.MainManifestId != fixed.Scope.Manifest || preview.PreviewCutRef != fixed.Intent.Operation {
		return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_UNKNOWN")
	}
	raw, err := s.client.FixedDraft(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	// Go保留Node完整原固定输入；即使冲突也不抹掉两份正文和真实切点。
	if err = s.persistFixed(ctx, task, raw); err != nil {
		return draftPrepared{}, err
	}
	expected, err := s.fixedPreview(raw, fixed)
	if err != nil {
		return expected, err
	}
	var originalParts map[string]json.RawMessage
	if json.Unmarshal(raw, &originalParts) != nil {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_BINDING_UNKNOWN")
	}
	draft, err := decodePullFixedContent(originalParts["draft"])
	if err != nil {
		return expected, err
	}
	if !bytes.Equal(preview.InputDigest, expected.Digest) || !proto.Equal(preview.Draft, draft) {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_BINDING_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err = s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": expected.Preview, "expected_input_digest": expected.Digest})
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	var prepared draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &prepared.State) != nil {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
	}
````

## 第 16 步：ReverseChanges.compute（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 365 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>prepared.State == &quot;CHANGED&quot;</code>。
- 检查 <code>json.Unmarshal(parts&#91;&quot;code&quot;&#93;, &amp;prepared.Code) != nil &#124;&#124; prepared.Code == &quot;&quot;</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prepared, domain.Violation(domain.StateConflict, prepared.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>prepared.State != &quot;PREPARED&quot;</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return prepared, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(parts&#91;&quot;preview_cut_ref&quot;&#93;, &amp;prepared.Preview) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;input_digest&quot;&#93;, &amp;prepared.Digest) != nil &#124;&#124; json.Unmarshal(parts&#91;&quot;draft_cut&quot;&#93;, &amp;prepared …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_PREPARED_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(actual, wanted)</code>。条件成立时执行 <code>return prepared, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CUT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prepared, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=14807:16185 -->
````go
	if prepared.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &prepared.Code) != nil || prepared.Code == "" {
			return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
		}
		return prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State != "PREPARED" {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
	}
	prepared.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return prepared, err
	}
	prepared.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != expected.Preview || !bytes.Equal(prepared.Digest, expected.Digest) || !proto.Equal(prepared.Main, expected.Main) || !proto.Equal(prepared.Candidate, expected.Candidate) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARED_BINDING_UNKNOWN")
	}
	actual, _ := codec.Canonical(prepared.Cut)
	wanted, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(actual, wanted) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CUT_CHANGED")
	}
	return prepared, nil
}

````

## 第 17 步：ReverseChanges.persistFixed

插入位置：第 16 步代码之后，紧接追加；完成稿约第 393 行。

本步声明：`func (s *ReverseChanges) persistFixed(ctx context.Context, task domain.SubmissionTask, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;REVERSE_FIXED&quot;, task.Operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code>；<code>repository.InsertSubmissionTask(ctx, tx, &quot;REVERSE_FIXED&quot;, task.Operation, task.Operation, raw, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_FIXED_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_FIXED_CHANGED&quot;)</code>；<code>e</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>e1 != nil &#124;&#124; e2 != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_FIXED_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_FIXED_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>e == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return e</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 17 bytes=16185:17409 -->
````go
func (s *ReverseChanges) persistFixed(ctx context.Context, task domain.SubmissionTask, raw []byte) error {
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
	if prior, e := repository.MergeArtifact(ctx, tx, "REVERSE_FIXED", task.Operation); e == nil {
		// MySQL JSON只改变对象键的排列；比较完整规范事实，不改已有payload。
		left, e1 := codec.Decode(prior)
		right, e2 := codec.Decode(raw)
		if e1 != nil || e2 != nil {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_UNKNOWN")
		}
		a, _ := codec.Canonical(left)
		b, _ := codec.Canonical(right)
		if !bytes.Equal(a, b) {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_CHANGED")
		}
	} else if e == sql.ErrNoRows {
		if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	} else {
		return e
	}
	return tx.Commit()
}

// reverse_install.go：保留原实现，按业务合并文件。

````

## 第 18 步：ReverseChanges.CommittedSource

插入位置：第 17 步代码之后，紧接追加；完成稿约第 429 行。

本步声明：`func (s *ReverseChanges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>repository.MergeArtifact(ctx, s.db, &quot;REVERSE_COMMITTED&quot;, query.OperationId)</code>；<code>s.validateReviewSource(ctx, s.db, install, false)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>query.OperationId</code>；<code>validatePullCommittedBinding(install.draftInstall, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_RESULT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>authority.State != pb.OperationResult_SUCCEEDED &amp;&amp; authority.State != pb.OperationResult_REJECTED</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;install) != nil &#124;&#124; validatePullCommittedBinding(install.draftInstall, query) != nil &#124;&#124; install.Fixed.Intent.Operation != query.OperationId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=17409:18804 -->
````go
func (s *ReverseChanges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "REVERSE_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install reverseInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		if err = s.validateReviewSource(ctx, s.db, install, false); err != nil {
			return nil, err
		}
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

````

## 第 19 步：ReverseChanges.reconcile

插入位置：第 18 步代码之后，紧接追加；完成稿约第 459 行。

本步声明：`func (s *ReverseChanges) reconcile(ctx context.Context, task domain.SubmissionTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.OperationResult(ctx, query)</code>；<code>s.client.Apply(ctx, &amp;pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_ACK_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CONTROL_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(task.Payload, &amp;install) != nil &#124;&#124; install.Context == nil &#124;&#124; install.Context.Target == nil &#124;&#124; install.Fixed.Intent.Operation != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reply == nil &#124;&#124; reply.OperationId != task.Operation &#124;&#124; reply.State != authority.State &#124;&#124; !bytes.Equal(reply.ResultDigest, authority.ResultDigest) &#124;&#124; !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(reply.CurrentStateJsonUtf8, &amp;current) != nil &#124;&#124; current.Sync != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED &amp;&amp; (install.Content == nil &#124;&#124; current.Room != domain.TextID(install.NewRoom) &#124;&#124; current.Epoch != domain.TextID(install.Epoch) &#124;&#124; cur …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_INSTALL_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=18804:21123 -->
````go
func (s *ReverseChanges) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	var install reverseInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil || install.Context.Target == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (install.Content == nil || current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
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

## 第 20 步：ReverseChanges.reconcile（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 504 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority.State == pb.OperationResult_SUCCEEDED</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pointer.Document != query.Target.DocumentId &#124;&#124; pointer.Unit != query.Target.UnitId &#124;&#124; pointer.Branch != install.Branch &#124;&#124; pointer.Round != install.Round &#124;&#124; pointer.Status != &quot;REVIE …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_REVIEW_SOURCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 20 bytes=21123:22916 -->
````go
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if authority.State == pb.OperationResult_SUCCEEDED {
		pointer, err := repository.LockedReviewPointer(ctx, tx, install.Request)
		if err != nil {
			return err
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, query.Target.UnitId)
		if err != nil {
			return err
		}
		round, err := repository.MergeRoundInputs(ctx, tx, install.Request, install.Round)
		if err != nil {
			return err
		}
		if pointer.Document != query.Target.DocumentId || pointer.Unit != query.Target.UnitId || pointer.Branch != install.Branch || pointer.Round != install.Round || pointer.Status != "REVIEWING" || pointer.Archived || guard.Ongoing != install.Request || guard.Submission != "" || round.Submission != task.Operation || round.SubmittedVersion != install.Version || round.Lineage != install.Lineage || round.Capture != task.Operation {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
		}
		// 成功后的恢复属于系统责任，注销不重写旧成功，也不重新借用作者权限。
		if err = repository.ActivateDiscardedBranch(ctx, tx, install.Branch, install.Epoch); err != nil {
			return err
		}
	}
	if err = repository.SetBranchControl(ctx, tx, install.Branch, ""); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_original.go：保留原实现，按业务合并文件。

````

## 第 21 步：ReverseChanges.original

插入位置：第 20 步代码之后，紧接追加；完成稿约第 545 行。

合并前正文来自原可靠MAIN采样，而非input_manifest里的更旧正式版本；否则会反向删除当时兼容的MAIN尾部。

本步声明：`func (s *ReverseChanges) original(ctx context.Context, tx *sql.Tx, document, record, structure uint64) (repository.ReverseMergeRecord, *pb.FixedContent, *pb.FixedContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReverseMerge(ctx, tx, document, record)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key)</code>；<code>repository.VersionContent(ctx, tx, value.Version, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.ReverseMerge(ctx, tx, document, record)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key)</code>；<code>repository.VersionContent(ctx, tx, value.Version, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ReverseMerge(ctx, tx, document, record)</code>；<code>target.DocumentId != document</code>；<code>validateFixedMergeContent(s.validator, before, document, value.Unit, value.Structure)</code>；<code>repository.VersionContent(ctx, tx, value.Version, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>record</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ReverseMerge(ctx, tx, document, record)</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>value.Structure != structure</code>；<code>requireStableReviewRange(ctx, tx, document, value.Unit, value.Structure, structure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.ReverseMergeRecord</code> | 使用 <code>repository.ReverseMergeRecord</code> 类型；其定义处的职责：归属由真实审核/合并/轮次外键链确定，不能用客户端给出的现存块ID推断反向范围。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>before</code> |
| <code>(匿名/未命名)</code> | <code>*pb.FixedContent</code> | 实际返回表达式：<code>nil</code>；<code>after</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;REVERSE_MERGE_NOT_FOUND&quot;)</code>；<code>err</code>；<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return value, nil, nil, domain.Violation(domain.StateConflict, &quot;REVERSE_MERGE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>value.State != &quot;SUCCEEDED&quot; &#124;&#124; len(value.Digest) != 32</code>。条件成立时执行 <code>return value, nil, nil, unknown</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Input == nil &#124;&#124; fixed.Input.Context == nil &#124;&#124; fixed.Input.Context.Target == nil &#124;&#124; fixed.Input.Main == nil &#124;&#124; fixed.Attempt.Main == nil</code>。条件成立时执行 <code>return value, nil, nil, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(digest&#91;:&#93;, value.Digest) &#124;&#124; !bytes.Equal(fixed.Input.InputDigest, value.Digest) &#124;&#124; fixed.Attempt.ID != value.Attempt &#124;&#124; fixed.Input.AttemptId != domain.T …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return value, nil, nil, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>after.UnitId != value.Unit &#124;&#124; after.StructureId != value.Structure &#124;&#124; after.OperationId != value.Operation</code>。条件成立时执行 <code>return value, nil, nil, unknown</code>。当前函数在这里结束。
- 检查 <code>value.Structure != structure</code>。

<!-- write-step: 21 bytes=22916:25407 -->
````go
// 合并前正文来自原可靠MAIN采样，而非input_manifest里的更旧正式版本；否则会反向删除当时兼容的MAIN尾部。
func (s *ReverseChanges) original(ctx context.Context, tx *sql.Tx, document, record, structure uint64) (repository.ReverseMergeRecord, *pb.FixedContent, *pb.FixedContent, error) {
	value, err := repository.ReverseMerge(ctx, tx, document, record)
	if err == sql.ErrNoRows {
		return value, nil, nil, domain.Violation(domain.StateConflict, "REVERSE_MERGE_NOT_FOUND")
	}
	if err != nil {
		return value, nil, nil, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_SOURCE_UNKNOWN")
	if value.State != "SUCCEEDED" || len(value.Digest) != 32 {
		return value, nil, nil, unknown
	}
	key := attemptKey(value.Operation, value.Attempt)
	raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key)
	if err == sql.ErrNoRows {
		raw, err = repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key)
	}
	if err != nil {
		return value, nil, nil, err
	}
	var fixed mergeFixedInputs
	if json.Unmarshal(raw, &fixed) != nil || fixed.Input == nil || fixed.Input.Context == nil || fixed.Input.Context.Target == nil || fixed.Input.Main == nil || fixed.Attempt.Main == nil {
		return value, nil, nil, unknown
	}
	target := fixed.Input.Context.Target
	digest, err := domain.MergeInputDigest(fixed.Input)
	if err != nil || !bytes.Equal(digest[:], value.Digest) || !bytes.Equal(fixed.Input.InputDigest, value.Digest) || fixed.Attempt.ID != value.Attempt || fixed.Input.AttemptId != domain.TextID(value.Attempt) || fixed.Input.Context.OperationId != value.Operation || target.DocumentId != document || target.UnitId != value.Unit || target.BranchId != value.Branch || fixed.Attempt.Main.ExpectedManifestId != value.InputManifest || fixed.Attempt.Round.Capture != value.Capture {
		return value, nil, nil, unknown
	}
	before := proto.Clone(fixed.Input.Main).(*pb.FixedContent)
	if err = validateFixedMergeContent(s.validator, before, document, value.Unit, value.Structure); err != nil {
		return value, nil, nil, err
	}
	after, err := repository.VersionContent(ctx, tx, value.Version, document)
	if err != nil {
		return value, nil, nil, err
	}
	if err = canonicalVersion(after, value.Version); err != nil {
		return value, nil, nil, err
	}
	if after.UnitId != value.Unit || after.StructureId != value.Structure || after.OperationId != value.Operation {
		return value, nil, nil, unknown
	}
	if value.Structure != structure {
````

## 第 22 步：ReverseChanges.original（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 590 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, before, after, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=25407:25661 -->
````go
		if err = requireStableReviewRange(ctx, tx, document, value.Unit, value.Structure, structure); err != nil {
			return value, nil, nil, err
		}
	}
	return value, before, after, nil
}

// reverse_readiness.go：保留原实现，按业务合并文件。

````

## 第 23 步：validatePendingReverseBinding

插入位置：第 22 步代码之后，紧接追加；完成稿约第 599 行。

本步声明：`func validatePendingReverseBinding(fact repository.PendingReverseBinding) (reverseContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingReverseBinding</code> | 使用 <code>repository.PendingReverseBinding</code> 类型；其定义处的职责：通过审核、合并、轮次外键链定位反向范围，并存储新反向操作及审计。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Actor</code>；<code>fact.Operation</code>；<code>fact.Owner</code><br>另有 16 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>reverseContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;fixed) != nil</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; fixed.Actor != fact.Actor &#124;&#124; ctx.ActorId != fact.Actor &#124;&#124; ctx.OperationId != fact.Operation &#124;&#124; fixed.Intent.Operation != fact.Operation &#124;&#124; fact.O …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>scope.Document == 0 &#124;&#124; scope.Unit == 0 &#124;&#124; scope.Room == 0 &#124;&#124; scope.Manifest == 0 &#124;&#124; scope.Structure == 0 &#124;&#124; scope.Style == 0 &#124;&#124; scope.Control == 0 &#124;&#124; scope.Workspace.Workspace == 0 …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !reflect.DeepEqual(parsed, fixed.Intent) &#124;&#124; parsed.Manifest != scope.Manifest &#124;&#124; parsed.Merge != fixed.Record.ID</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, fact.Fingerprint) &#124;&#124; json.Unmarshal(fact.Target, &amp;target) != nil &#124;&#124; target.Path != path</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Access.Document.Id != domain.TextID(scope.Document) &#124;&#124; fixed.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) &#124;&#124; fixed.Access.Document.CurrentStructureId != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.ScopeKey == &quot;&quot; &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != 0</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != fixed.Actor &#124;&#124; proof.OperationId != fact.Operation &#124;&#124; proof.Purpose != &quot;REVERSE&quot; &#124;&#124; !proto.Equal(proof.Target, expected) &#124;&#124; len(proof.SessionRef) ! …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; available&#91;source.ScopeKey&#93; != source.Epoch</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=25661:28881 -->
````go
func validatePendingReverseBinding(fact repository.PendingReverseBinding) (reverseContext, error) {
	var fixed reverseContext
	invalid := errors.New("REVERSE_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &fixed) != nil {
		return fixed, invalid
	}
	ctx, scope := fixed.Context, fixed.Scope
	if ctx == nil || ctx.Target == nil || fixed.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || fixed.Intent.Operation != fact.Operation || fact.Owner != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Manage {
		return fixed, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "REVERSE"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || scope.Workspace.Lineage == 0 || !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || fixed.GuardGeneration == 0 {
		return fixed, invalid
	}
	parsed, err := domain.ParseReverseChange(fixed.Intent.Input)
	if err != nil || !reflect.DeepEqual(parsed, fixed.Intent) || parsed.Manifest != scope.Manifest || parsed.Merge != fixed.Record.ID {
		return fixed, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/reverse-changes"
	var target struct {
		Path string `json:"path"`
	}
	fingerprint, err := domain.OperationFingerprint(path, fixed.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) || json.Unmarshal(fact.Target, &target) != nil || target.Path != path {
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
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fact.Operation || proof.Purpose != "REVERSE" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return fixed, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return fixed, invalid
			}
		}
	}
````

## 第 24 步：validatePendingReverseBinding（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 645 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>fixed.Access.Proofs&#91;0&#93;.AdmissionId != ctx.IdentityAdmissionRef</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>record.ID == 0 &#124;&#124; record.Document != scope.Document &#124;&#124; record.Unit != scope.Unit &#124;&#124; record.Request == 0 &#124;&#124; record.Round == 0 &#124;&#124; record.Attempt == 0 &#124;&#124; record.InputManifest == 0 &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>content.DocumentId != scope.Document &#124;&#124; content.UnitId != scope.Unit &#124;&#124; content.StructureId != record.Structure &#124;&#124; content.OperationId != record.Operation &#124;&#124; len(content.BodyHash) != 32</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.SourceActor != fixed.Actor &#124;&#124; fact.SourceDocument != scope.Document &#124;&#124; fact.SourceUnit != scope.Unit &#124;&#124; fact.SourceBranch != scope.Workspace.Workspace &#124;&#124; fact.SourceLineage != …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;REVERSE_RECOVER&quot;</code>。
- 检查 <code>fact.State != &quot;PREPARING&quot; &#124;&#124; fact.SourceState != &quot;PREPARING&quot; &#124;&#124; fact.HeldOperation != fact.Operation &#124;&#124; fact.HeldGeneration != fixed.GuardGeneration &#124;&#124; !bytes.Equal(fact.Payload, fact.Original)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.Kind == &quot;REVERSE_RECONCILE&quot;</code>。
- 检查 <code>(fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;) &#124;&#124; fact.SourceState != fact.State &#124;&#124; !bytes.Equal(fact.Payload, fact.Committed)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;install) != nil &#124;&#124; !proto.Equal(install.Context, ctx) &#124;&#124; install.Branch != scope.Workspace.Workspace &#124;&#124; install.OldRoom != scope.Room &#124;&#124; install.Preview != fact.Operation</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(a, b)</code>。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot; &amp;&amp; (install.NewRoom == 0 &#124;&#124; install.NewRoom == scope.Room &#124;&#124; install.Epoch != scope.Workspace.Epoch+1 &#124;&#124; install.Lineage == 0 &#124;&#124; install.Request == 0 &#124;&#124; i …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=28881:31370 -->
````go
	if fixed.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef {
		return fixed, invalid
	}
	record := fixed.Record
	if record.ID == 0 || record.Document != scope.Document || record.Unit != scope.Unit || record.Request == 0 || record.Round == 0 || record.Attempt == 0 || record.InputManifest == 0 || record.OutputManifest == 0 || record.Version == 0 || record.Structure == 0 || record.State != "SUCCEEDED" || !domain.ValidOperationID(record.Operation) || len(record.Digest) != 32 || fixed.Before == nil || fixed.After == nil {
		return fixed, invalid
	}
	for _, content := range []*pb.FixedContent{fixed.Before, fixed.After} {
		if content.DocumentId != scope.Document || content.UnitId != scope.Unit || content.StructureId != record.Structure || content.OperationId != record.Operation || len(content.BodyHash) != 32 {
			return fixed, invalid
		}
	}
	if fact.SourceActor != fixed.Actor || fact.SourceDocument != scope.Document || fact.SourceUnit != scope.Unit || fact.SourceBranch != scope.Workspace.Workspace || fact.SourceLineage != scope.Workspace.Lineage || fact.SourceGeneration != fixed.GuardGeneration {
		return fixed, invalid
	}
	if fact.Kind == "REVERSE_RECOVER" {
		if fact.State != "PREPARING" || fact.SourceState != "PREPARING" || fact.HeldOperation != fact.Operation || fact.HeldGeneration != fixed.GuardGeneration || !bytes.Equal(fact.Payload, fact.Original) {
			return fixed, invalid
		}
	} else if fact.Kind == "REVERSE_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || fact.SourceState != fact.State || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, invalid
		}
		var install reverseInstall
		if json.Unmarshal(fact.Payload, &install) != nil || !proto.Equal(install.Context, ctx) || install.Branch != scope.Workspace.Workspace || install.OldRoom != scope.Room || install.Preview != fact.Operation {
			return fixed, invalid
		}
		a, _ := json.Marshal(install.Fixed)
		b, _ := json.Marshal(fixed)
		if !bytes.Equal(a, b) {
			return fixed, invalid
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == scope.Room || install.Epoch != scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Request == 0 || install.Round == 0 || install.Version == 0 || install.Round != fact.Round || install.Content == nil || install.Content.OperationId != fact.Operation || install.Content.CaptureId != fact.Operation) {
			return fixed, invalid
		}
	} else {
		return fixed, invalid
	}
	return fixed, nil
}

````

## 第 25 步：validateReverseAdmission

插入位置：第 24 步代码之后，紧接追加；完成稿约第 686 行。

本步声明：`func validateReverseAdmission(fixed reverseContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Access</code>；<code>fixed.Actor</code>；<code>fixed.Scope</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil &#124;&#124; actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Scope.Document &#124;&#124; actual.Unit != f …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, fixed.Context.Target)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=31370:32154 -->
````go
func validateReverseAdmission(fixed reverseContext, actual repository.OriginalAdmissionBinding) error {
	invalid := errors.New("REVERSE_ADMISSION_UNKNOWN")
	if fixed.Context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "REVERSE" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return invalid
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return invalid
	}
	return nil
}

````

## 第 26 步：ReverseChanges.Ready

插入位置：第 25 步代码之后，紧接追加；完成稿约第 698 行。

本步声明：`func (s *ReverseChanges) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingReverseBindings(ctx, s.db)</code>；<code>repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)</code>；<code>repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)</code>；<code>s.db.BeginTx(ctx, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REVERSE_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;REVERSE_ADMISSION_UNKNOWN&quot;)</code>；<code>errors.New(&quot;REVERSE_SOURCE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;REVERSE_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; validateReverseAdmission(fixed, actual) != nil</code>。条件成立时执行 <code>return errors.New(&quot;REVERSE_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return errors.New(&quot;REVERSE_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; !reflect.DeepEqual(record, fixed.Record) &#124;&#124; !proto.Equal(before, fixed.Before) &#124;&#124; !proto.Equal(after, fixed.After)</code>。条件成立时执行 <code>return errors.New(&quot;REVERSE_SOURCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=32154:33462 -->
````go
func (s *ReverseChanges) Ready(ctx context.Context) error {
	facts, err := repository.PendingReverseBindings(ctx, s.db)
	if err != nil {
		return errors.New("REVERSE_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingReverseBinding(fact)
		if err != nil {
			return err
		}
		actual, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateReverseAdmission(fixed, actual) != nil {
			return errors.New("REVERSE_ADMISSION_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("REVERSE_ADMISSION_UNKNOWN")
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		record, before, after, err := s.original(ctx, tx, fixed.Scope.Document, fixed.Intent.Merge, fixed.Scope.Structure)
		if err == nil {
			err = s.validatePendingReviewSource(ctx, tx, fact)
		}
		_ = tx.Rollback()
		if err != nil || !reflect.DeepEqual(record, fixed.Record) || !proto.Equal(before, fixed.Before) || !proto.Equal(after, fixed.After) {
			return errors.New("REVERSE_SOURCE_UNKNOWN")
		}
	}
	return nil
}

// reverse_readings.go：保留原实现，按业务合并文件。

````

## 第 27 步：ReverseChanges.OperationDocument

插入位置：第 26 步代码之后，紧接追加；完成稿约第 734 行。

本步声明：`func (s *ReverseChanges) OperationDocument(ctx context.Context, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.lookup(ctx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>fixed.Scope.Document</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return fixed.Scope.Document, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=33462:33639 -->
````go
func (s *ReverseChanges) OperationDocument(ctx context.Context, operation string) (uint64, error) {
	fixed, err := s.lookup(ctx, operation)
	return fixed.Scope.Document, err
}

````

## 第 28 步：ReverseChanges.Progress

插入位置：第 27 步代码之后，紧接追加；完成稿约第 739 行。

本步声明：`func (s *ReverseChanges) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, operation)</code>；<code>s.access.Authorize(ctx, actor, fixed.Scope.Document, &quot;REVERSE_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fixed.Scope.Document, &quot;REVERSE_RESULT&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.lookup(ctx, operation)</code>；<code>repository.ReverseResult(ctx, tx, operation)</code>；<code>repository.ReverseInstalled(ctx, tx, operation)</code>；<code>&quot;operation_id&quot;: operation</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		fact, err := repository.Reverse …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;REVERSE_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>actor.ID != fixed.Actor</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;REVERSE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if _, err := ValidateAccess(ctx, tx, access); err != nil { 			return nil, err 		} 		fact, er …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.Actor != actor.ID &#124;&#124; !fact.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;REVERSE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.State != &quot;PREPARING&quot; &amp;&amp; !installed</code>。条件成立时执行 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: operation, &quot;state&quot;: &quot;WAITING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + operation}, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=33639:35110 -->
````go
func (s *ReverseChanges) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if actor.ID != fixed.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "REVERSE_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Scope.Document, "REVERSE_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Manage {
		return nil, nil, domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fact, err := repository.ReverseResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.Actor != actor.ID || !fact.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "REVERSE_NOT_FOUND")
		}
		var value map[string]any
		if json.Unmarshal(fact.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
		}
		installed, err := repository.ReverseInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.State != "PREPARING" && !installed {
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

````

## 第 29 步：ReverseChanges.OperationResult

插入位置：第 28 步代码之后，紧接追加；完成稿约第 780 行。

本步声明：`func (s *ReverseChanges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, query.OperationId)</code>；<code>repository.ReverseResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;REVERSE_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;REVERSE_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor &#124;&#124; !bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=35110:37004 -->
````go
func (s *ReverseChanges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "REVERSE_QUERY_INVALID")
	}
	fixed, err := s.lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.ReverseResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// reverse_recovery.go：保留原实现，按业务合并文件。

````

## 第 30 步：ReverseChanges.RecoverOne

插入位置：第 29 步代码之后，紧接追加；完成稿约第 825 行。

本步声明：`func (s *ReverseChanges) RecoverOne(parent context.Context)`。


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
- 检查 <code>task.Kind == &quot;REVERSE_RECONCILE&quot;</code>。
- 检查 <code>err == nil</code>。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err == nil</code>。
- 执行 <code>check.Rollback()</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 30 bytes=37004:38286 -->
````go
func (s *ReverseChanges) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimReverseTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindingError := repository.ClaimedReverseBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingReverseBinding(fact)
	if bindingError != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	if err != nil {
		_ = repository.RetryMergeTask(ctx, s.db, task, "REVERSE_BINDING_UNKNOWN")
		return
	}
	if task.Kind == "REVERSE_RECONCILE" {
		err = s.validatePendingReviewSource(ctx, s.db, fact)
		if err == nil {
			err = s.reconcile(ctx, task)
		}
	} else {
		if err == nil {
			proofs, e := s.resume(ctx, fixed)
			err = e
			if err == nil {
				check, e := s.db.BeginTx(ctx, nil)
				err = e
				if err == nil {
					err = s.validate(ctx, check, fixed, proofs)
					_ = check.Rollback()
				}
			}
			if err == nil {
				prepared, e := s.compute(ctx, task, fixed)
				err = e
				if err == nil {
````

## 第 31 步：ReverseChanges.RecoverOne（接着实现）

插入位置：第 30 步代码之后，紧接追加；完成稿约第 870 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.StateConflict &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 31 bytes=38286:38800 -->
````go
					err = s.commit(ctx, task, fixed, prepared, proofs)
				}
			}
			var violation domain.RuleViolation
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

## 第 32 步：ReverseChanges.reject

插入位置：第 31 步代码之后，紧接追加；完成稿约第 886 行。

本步声明：`func (s *ReverseChanges) reject(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.DocumentRow(ctx, tx, fixed.Scope.Document, true)</code>；<code>repository.LockedBranchControl(ctx, tx, fixed.Scope.Workspace.Workspace)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.CompleteReverseTask(ctx, tx, task, &quot;REJECTED&quot;, install)</code> |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>fixed.Scope</code>；<code>Fixed: fixed</code>；<code>fixed.Context</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.RejectReverseSource(ctx, tx, task.Operation, fixed.Scope.Unit, code)</code>；<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CONTROL_UNKNOWN&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>owner != task.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 32 bytes=38800:40485 -->
````go
func (s *ReverseChanges) reject(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, code string) error {
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
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.RejectReverseSource(ctx, tx, task.Operation, fixed.Scope.Unit, code); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	install, err := json.Marshal(reverseInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: task.Operation}})
	if err != nil {
		return err
	}
	if err = repository.CompleteReverseTask(ctx, tx, task, "REJECTED", install); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_register.go：保留原实现，按业务合并文件。

````

## 第 33 步：ReverseChanges.Register

插入位置：第 32 步代码之后，紧接追加；完成稿约第 930 行。

本步声明：`func (s *ReverseChanges) Register(ctx context.Context, actor Actor, document uint64, intent domain.ReverseChangeIntent, path string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;REVERSE&quot;)</code>；<code>repository.ReverseResult(ctx, s.db, intent.Operation)</code>；<code>repository.ReverseMerge(ctx, s.db, document, intent.Merge)</code>；<code>repository.ReverseActiveBranch(ctx, s.db, record.Unit, document)</code><br>另有 19 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;REVERSE&quot;)</code>；<code>return actor, err</code>；<code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>actor.ID</code><br>另有 19 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;REVERSE&quot;)</code>；<code>repository.ReverseMerge(ctx, s.db, document, intent.Merge)</code>；<code>repository.ReverseActiveBranch(ctx, s.db, record.Unit, document)</code>；<code>repository.SubmissionWorkspace(ctx, s.db, branch, document, record.Unit)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.ReverseChangeIntent</code> | 使用 <code>domain.ReverseChangeIntent</code> 类型；其定义处的职责：反向意图指定不可变合并记录和当前清单；旧请求从不回退，理由属于新操作的审计事实。<br>在此声明中的实际使用：<code>intent.Input</code>；<code>intent.Operation</code>；<code>intent.Manifest</code>；<code>intent.Merge</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, intent.Input)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>prior.Actor != actor.ID &#124;&#124; !bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>access.Document.CurrentManifestId != domain.TextID(intent.Manifest)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;REVERSE_MERGE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;REVERSE_RANGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 33 bytes=40485:42337 -->
````go
func (s *ReverseChanges) Register(ctx context.Context, actor Actor, document uint64, intent domain.ReverseChangeIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "REVERSE")
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
	if prior, e := repository.ReverseResult(ctx, s.db, intent.Operation); e == nil {
		if prior.Actor != actor.ID || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	if access.Document.CurrentManifestId != domain.TextID(intent.Manifest) {
		return actor, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	record, err := repository.ReverseMerge(ctx, s.db, document, intent.Merge)
	if err == sql.ErrNoRows {
		return actor, domain.Violation(domain.StateConflict, "REVERSE_MERGE_NOT_FOUND")
	}
	if err != nil {
		return actor, err
	}
	branch, err := repository.ReverseActiveBranch(ctx, s.db, record.Unit, document)
	if err == sql.ErrNoRows {
		// 原单元已退役或删除，不能猜测其现存后代作为可写范围；真实数据库故障仍保留重试语义。
		return actor, domain.Violation(domain.StateConflict, "REVERSE_RANGE_CHANGED")
	}
	if err != nil {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, branch, document, record.Unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, err
	}
````

## 第 34 步：ReverseChanges.Register（接着实现）

插入位置：第 33 步代码之后，紧接追加；完成稿约第 976 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>e == nil</code>。
- 检查 <code>prior.ActorID != actor.ID &#124;&#124; prior.Kind != &quot;REVERSE&quot; &#124;&#124; !bytes.Equal(prior.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>e != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, e</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>current.CurrentManifestId != domain.TextID(intent.Manifest) &#124;&#124; current.CurrentStructureId != domain.TextID(structure)</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; active != branch</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 34 bytes=42337:44088 -->
````go
	target := &pb.Target{DocumentId: document, UnitId: record.Unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "REVERSE"}
	var proofs []*pb.IdentityAdmission
	proofs, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = proofs
	actor.Proof, actor.Sources = proofs[0], access.Sources
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, intent.Operation); err != nil {
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if prior, e := repository.LockedOperation(ctx, tx, intent.Operation); e == nil {
		if prior.ActorID != actor.ID || prior.Kind != "REVERSE" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
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
	if current.CurrentManifestId != domain.TextID(intent.Manifest) || current.CurrentStructureId != domain.TextID(structure) {
		return actor, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, record.Unit, document)
	if err != nil || active != branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
````

## 第 35 步：ReverseChanges.Register（接着实现）

插入位置：第 34 步代码之后，紧接追加；完成稿约第 1021 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; actual != workspace</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;LINEAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>owner != &quot;&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;BRANCH_CONTROL_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>guard.Ongoing != 0 &#124;&#124; guard.Resumable != 0 &#124;&#124; guard.Submission != &quot;&quot;</code>。条件成立时执行 <code>return actor, domain.Violation(domain.StateConflict, &quot;REVERSE_REVIEW_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。

<!-- write-step: 35 bytes=44088:46354 -->
````go
	actual, err := repository.LockedControlledWorkspace(ctx, tx, branch, document, record.Unit)
	if err != nil || actual != workspace {
		return actor, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	owner, err := repository.LockedBranchControl(ctx, tx, branch)
	if err != nil {
		return actor, err
	}
	if owner != "" {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, record.Unit)
	if err != nil {
		return actor, err
	}
	// 反向是新的审核，不会覆盖另一个进行中或尚可续提的请求；必须先明确结束那项工作。
	if guard.Ongoing != 0 || guard.Resumable != 0 || guard.Submission != "" {
		return actor, domain.Violation(domain.StateConflict, "REVERSE_REVIEW_PENDING")
	}
	record, before, after, err := s.original(ctx, tx, document, intent.Merge, structure)
	if err != nil {
		return actor, err
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: actor.SessionExpiresUnixMs, Access: access}, proofs); err != nil {
		return actor, err
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", branch, record.Unit, workspace.Epoch)
	if err != nil {
		return actor, err
	}
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalRevision(current.ControlRevision)
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	access.Document = current
	fixed := reverseContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Record: record, Before: before, After: after, GuardGeneration: guard.Generation + 1,
		Scope: draftSourceScope{Document: document, Unit: record.Unit, Room: room, Manifest: intent.Manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: workspace}}
	command := OperationCommand{ID: intent.Operation, Kind: "REVERSE", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "REVERSE", admission); err != nil {
````

## 第 36 步：ReverseChanges.Register（接着实现）

插入位置：第 35 步代码之后，紧接追加；完成稿约第 1066 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 36 bytes=46354:47180 -->
````go
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, record.Unit, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	payload, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterReverse(ctx, tx, record, actor.ID, workspace, fixed.GuardGeneration, intent.Operation, payload); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// reverse_review_source.go：保留原实现，按业务合并文件。

````

## 第 37 步：ReverseChanges.validateReviewSource

插入位置：第 36 步代码之后，紧接追加；完成稿约第 1085 行。

恢复元数据与真实不可变轮次交叉核验，不能只因两个任务里的版本号一致就开放Ready或安装。
调用方可传同一事务保持一致读；成功后的系统收尾不重新借用作者登录资格。

本步声明：`func (s *ReverseChanges) validateReviewSource(ctx context.Context, q repository.Queryer, install reverseInstall, pending bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReviewPointerRow(ctx, q, install.Request)</code>；<code>repository.MergeRoundInputs(ctx, q, install.Request, install.Round)</code>；<code>repository.VersionContent(ctx, q, install.Version, target.DocumentId)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.ReviewPointerRow(ctx, q, install.Request)</code>；<code>repository.MergeRoundInputs(ctx, q, install.Request, install.Round)</code>；<code>repository.VersionContent(ctx, q, install.Version, target.DocumentId)</code> |
| <code>install</code> | <code>reverseInstall</code> | 在此声明中的实际使用：<code>install.Context</code>；<code>install.Content</code>；<code>install.Request</code>；<code>install.Round</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>pending</code> | <code>bool</code> | 在此声明中的实际使用：<code>pending &amp;&amp; (pointer.Round != install.Round &#124;&#124; pointer.Status != &quot;REVIEWING&quot; &#124;&#124; pointer.Archived)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>install.Context == nil &#124;&#124; install.Context.Target == nil &#124;&#124; install.Content == nil</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pointer.Document != target.DocumentId &#124;&#124; pointer.Unit != target.UnitId &#124;&#124; pointer.Branch != install.Branch &#124;&#124; round.Submission != install.Context.OperationId &#124;&#124; round.SubmittedVers …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>pending &amp;&amp; (pointer.Round != install.Round &#124;&#124; pointer.Status != &quot;REVIEWING&quot; &#124;&#124; pointer.Archived)</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !proto.Equal(stored, expected)</code>。条件成立时执行 <code>return unknown</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=47180:49455 -->
````go
// 恢复元数据与真实不可变轮次交叉核验，不能只因两个任务里的版本号一致就开放Ready或安装。
// 调用方可传同一事务保持一致读；成功后的系统收尾不重新借用作者登录资格。
func (s *ReverseChanges) validateReviewSource(ctx context.Context, q repository.Queryer, install reverseInstall, pending bool) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
	if install.Context == nil || install.Context.Target == nil || install.Content == nil {
		return unknown
	}
	target := install.Context.Target
	pointer, err := repository.ReviewPointerRow(ctx, q, install.Request)
	if err != nil {
		return err
	}
	round, err := repository.MergeRoundInputs(ctx, q, install.Request, install.Round)
	if err != nil {
		return err
	}
	if pointer.Document != target.DocumentId || pointer.Unit != target.UnitId || pointer.Branch != install.Branch || round.Submission != install.Context.OperationId || round.SubmittedVersion != install.Version || round.Lineage != install.Lineage || round.Capture != install.Context.OperationId || round.Structure != target.StructureId {
		return unknown
	}
	// 未完成安装时当前轮次仍须独占；已完成后的私有原号查询允许请求后来已合并或续提。
	if pending && (pointer.Round != install.Round || pointer.Status != "REVIEWING" || pointer.Archived) {
		return unknown
	}
	stored, err := repository.VersionContent(ctx, q, install.Version, target.DocumentId)
	if err != nil {
		return err
	}
	if err = canonicalVersion(stored, install.Version); err != nil {
		return err
	}
	if err = validateFixedMergeContent(s.validator, stored, target.DocumentId, target.UnitId, target.StructureId); err != nil {
		return err
	}
	// 正文版本用版本引用切点；安装候选用新房间起始切点。只规范这三个传输字段，正文/谱系/资源仍精确比较。
	expected := proto.Clone(install.Content).(*pb.FixedContent)
	expected.BranchId, expected.WorkspaceEpoch = 0, 0
	expected.CutJsonUtf8, err = codec.Canonical(map[string]any{"content_version_id": domain.TextID(install.Version), "source_ref": install.Context.OperationId})
	if err != nil || !proto.Equal(stored, expected) {
		return unknown
	}
	return nil
}

````

## 第 38 步：ReverseChanges.validatePendingReviewSource

插入位置：第 37 步代码之后，紧接追加；完成稿约第 1128 行。

本步声明：`func (s *ReverseChanges) validatePendingReviewSource(ctx context.Context, q repository.Queryer, fact repository.PendingReverseBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.validateReviewSource(ctx, q, install, true)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>s.validateReviewSource(ctx, q, install, true)</code> |
| <code>fact</code> | <code>repository.PendingReverseBinding</code> | 使用 <code>repository.PendingReverseBinding</code> 类型；其定义处的职责：通过审核、合并、轮次外键链定位反向范围，并存储新反向操作及审计。<br>在此声明中的实际使用：<code>fact.Kind</code>；<code>fact.State</code>；<code>fact.Payload</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_REVIEW_SOURCE_UNKNOWN&quot;)</code>；<code>s.validateReviewSource(ctx, q, install, true)</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.Kind != &quot;REVERSE_RECONCILE&quot; &#124;&#124; fact.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Payload, &amp;install) != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_REVIEW_SOURCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.validateReviewSource(ctx, q, install, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=49455:49979 -->
````go
func (s *ReverseChanges) validatePendingReviewSource(ctx context.Context, q repository.Queryer, fact repository.PendingReverseBinding) error {
	if fact.Kind != "REVERSE_RECONCILE" || fact.State != "SUCCEEDED" {
		return nil
	}
	var install reverseInstall
	if json.Unmarshal(fact.Payload, &install) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
	}
	return s.validateReviewSource(ctx, q, install, true)
}

// reverse_source.go：保留原实现，按业务合并文件。

````

## 第 39 步：ReverseChanges.resume

插入位置：第 38 步代码之后，紧接追加；完成稿约第 1141 行。

本步声明：`func (s *ReverseChanges) resume(ctx context.Context, fixed reverseContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=49979:50487 -->
````go
func (s *ReverseChanges) resume(ctx context.Context, fixed reverseContext) ([]*pb.IdentityAdmission, error) {
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

## 第 40 步：ReverseChanges.validate

插入位置：第 39 步代码之后，紧接追加；完成稿约第 1156 行。

本步声明：`func (s *ReverseChanges) validate(ctx context.Context, tx *sql.Tx, fixed reverseContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, scope.Document)</code>；<code>repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>reverseContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Scope</code>；<code>fixed.Intent</code>；<code>fixed.GuardGeneration</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DRAFT_MANAGE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_CONTEXT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

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
- 检查 <code>owner != fixed.Intent.Operation</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_CONTROL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>guard.Ongoing != 0 &#124;&#124; guard.Resumable != 0 &#124;&#124; guard.Submission != fixed.Intent.Operation &#124;&#124; guard.Generation != fixed.GuardGeneration</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_GUARD_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 40 bytes=50487:52362 -->
````go
func (s *ReverseChanges) validate(ctx context.Context, tx *sql.Tx, fixed reverseContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	scope := fixed.Scope
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
	owner, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return err
	}
	if owner != fixed.Intent.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, scope.Unit)
	if err != nil {
		return err
	}
	if guard.Ongoing != 0 || guard.Resumable != 0 || guard.Submission != fixed.Intent.Operation || guard.Generation != fixed.GuardGeneration {
````

## 第 41 步：ReverseChanges.validate（接着实现）

插入位置：第 40 步代码之后，紧接追加；完成稿约第 1201 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_GUARD_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>admission.Actor != fixed.Actor &#124;&#124; admission.Document != scope.Document &#124;&#124; admission.Unit != scope.Unit &#124;&#124; admission.Operation != fixed.Intent.Operation &#124;&#124; admission.Purpose != &quot;REV …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;REVERSE_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_ADMISSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!reflect.DeepEqual(record, fixed.Record) &#124;&#124; !proto.Equal(before, fixed.Before) &#124;&#124; !proto.Equal(after, fixed.After)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;REVERSE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=52362:54258 -->
````go
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_GUARD_UNKNOWN")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	target := &pb.Target{}
	if admission.Actor != fixed.Actor || admission.Document != scope.Document || admission.Unit != scope.Unit || admission.Operation != fixed.Intent.Operation || admission.Purpose != "REVERSE" || admission.Identity != fixed.Context.IdentityAdmissionRef || admission.SessionExpires != fixed.SessionExpires || len(fixed.Access.Proofs) == 0 || !bytes.Equal(admission.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(admission.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "REVERSE_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	record, before, after, err := s.original(ctx, tx, scope.Document, fixed.Intent.Merge, scope.Structure)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(record, fixed.Record) || !proto.Equal(before, fixed.Before) || !proto.Equal(after, fixed.After) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_SOURCE_CHANGED")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

````

## 第 42 步：ReverseChanges.Source

插入位置：第 41 步代码之后，紧接追加；完成稿约第 1235 行。

本步声明：`func (s *ReverseChanges) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lookup(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validate(ctx, tx, fixed, proofs)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(input, reversePreviewRequest(fixed))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DRAFT_SOURCE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input, reversePreviewRequest(fixed))</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=54258:55428 -->
````go
func (s *ReverseChanges) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, reversePreviewRequest(fixed)) {
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
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err != nil {
		return nil, err
	}
	before, err := fixedWire(fixed.Before)
	if err != nil {
		return nil, err
	}
	after, err := fixedWire(fixed.After)
	if err != nil {
		return nil, err
	}
	value["purpose"], value["reverse_before"], value["reverse_after"], value["merge_record_id"], value["reason"] = "REVERSE", before, after, domain.TextID(fixed.Record.ID), fixed.Intent.Reason
	return value, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/reverse_changes_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
