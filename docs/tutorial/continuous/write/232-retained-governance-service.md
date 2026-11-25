# 分步编写 `services/document/internal/logic/document/retained_governance_service.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/232-retained-governance-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

取得Account平台治理资格并登记孤立留存撤下/删除。

旧grant、组织角色和配置名单均不能替代真实平台治理能力。

写之前先按这条执行路线理解各段的连接：

1. Authorize取专用证明
2. 原号走标准Replay
3. Prepare固定新号范围
4. Metadata仅返回治理元数据
5. Progress核原发起人。

本文件需要守住的边界：

- 元数据不创建正文准入
- 来源一次排序锁，避免反向锁序
- 治理不恢复协作。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/retained_governance_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/retained_governance_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:493 -->
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
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 2 步：RetainedGovernanceAuthority

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

实现方必须调用Account真实治理能力，不得根据Doc旧grant、组织角色或
进程配置名单发证；标准事务还会向Account再次Revalidate这个证明。

本步声明：`type RetainedGovernanceAuthority interface {`。

**这一段为什么这样写：** 留存内容治理独立于原账号普通权限，必须重新取得平台治理权威准入。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AcquireGovernance</code> | <code>(context.Context, Actor, string, uint64) (*pb.IdentityAdmission, error)</code> | ctx、当前Actor、治理操作号、目标文档编号输入，返回Account的治理准入证明；普通组织管理员不自动拥有此资格。 |

<!-- write-step: 2 bytes=493:815 -->
````go
// 实现方必须调用Account真实治理能力，不得根据Doc旧grant、组织角色或
// 进程配置名单发证；标准事务还会向Account再次Revalidate这个证明。
type RetainedGovernanceAuthority interface {
	AcquireGovernance(context.Context, Actor, string, uint64) (*pb.IdentityAdmission, error)
}

````

## 第 3 步：RetainedResourceCleaner

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type RetainedResourceCleaner interface {`。

**这一段为什么这样写：** 留存治理的资源清理边界，与普通删除共用引用及容量核验。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>CleanRetainedResource</code> | <code>(context.Context, uint64, uint64, uint64) error</code> | ctx、资源编号、空间编号、文档编号输入，清理该留存文档对应资源分配，不能误删其他文档共享对象。 |

<!-- write-step: 3 bytes=815:929 -->
````go
type RetainedResourceCleaner interface {
	CleanRetainedResource(context.Context, uint64, uint64, uint64) error
}

````

## 第 4 步：RetainedExportDrainer

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`type RetainedExportDrainer interface {`。

**这一段为什么这样写：** 治理清理前先停止依赖旧正文的导出，避免转换器仍在读取被删除的文件。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DrainDocument</code> | <code>(context.Context, uint64) error</code> | ctx和文档编号输入，等待/核验该文档导出任务已排空，返回错误时不能继续清理底层内容。 |

<!-- write-step: 4 bytes=929:1017 -->
````go
type RetainedExportDrainer interface {
	DrainDocument(context.Context, uint64) error
}

````

## 第 5 步：RetainedGovernance

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`type RetainedGovernance struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authority</code> | <code>RetainedGovernanceAuthority</code> | 使用 <code>RetainedGovernanceAuthority</code> 类型；其定义处的职责：留存内容治理独立于原账号普通权限，必须重新取得平台治理权威准入。 |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。 |
| <code>resources</code> | <code>RetainedResourceCleaner</code> | 使用 <code>RetainedResourceCleaner</code> 类型；其定义处的职责：留存治理的资源清理边界，与普通删除共用引用及容量核验。 |
| <code>exports</code> | <code>RetainedExportDrainer</code> | 使用 <code>RetainedExportDrainer</code> 类型；其定义处的职责：治理清理前先停止依赖旧正文的导出，避免转换器仍在读取被删除的文件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 5 bytes=1017:1231 -->
````go
type RetainedGovernance struct {
	db        *sql.DB
	authority RetainedGovernanceAuthority
	cleaner   AccountDocumentCleaner
	resources RetainedResourceCleaner
	exports   RetainedExportDrainer
	instance  string
}

````

## 第 6 步：NewRetainedGovernance

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func NewRetainedGovernance(db *sql.DB, authority RetainedGovernanceAuthority, cleaner AccountDocumentCleaner, resources RetainedResourceCleaner, exports RetainedExportDrainer, instance string) *RetainedGovernance`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>authority</code> | <code>RetainedGovernanceAuthority</code> | 使用 <code>RetainedGovernanceAuthority</code> 类型；其定义处的职责：留存内容治理独立于原账号普通权限，必须重新取得平台治理权威准入。<br>在此声明中的实际使用：<code>authority: authority</code> |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>cleaner: cleaner</code> |
| <code>resources</code> | <code>RetainedResourceCleaner</code> | 使用 <code>RetainedResourceCleaner</code> 类型；其定义处的职责：留存治理的资源清理边界，与普通删除共用引用及容量核验。<br>在此声明中的实际使用：<code>resources: resources</code> |
| <code>exports</code> | <code>RetainedExportDrainer</code> | 使用 <code>RetainedExportDrainer</code> 类型；其定义处的职责：治理清理前先停止依赖旧正文的导出，避免转换器仍在读取被删除的文件。<br>在此声明中的实际使用：<code>exports: exports</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*RetainedGovernance</code> | 实际返回表达式：<code>&amp;RetainedGovernance{db: db, authority: authority, cleaner: cleaner, resources: resources, exports: exports, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;RetainedGovernance{db: db, authority: authority, cleaner: cleaner, resources: resources, exports: exports, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1231:1585 -->
````go
func NewRetainedGovernance(db *sql.DB, authority RetainedGovernanceAuthority, cleaner AccountDocumentCleaner, resources RetainedResourceCleaner, exports RetainedExportDrainer, instance string) *RetainedGovernance {
	return &RetainedGovernance{db: db, authority: authority, cleaner: cleaner, resources: resources, exports: exports, instance: instance}
}

````

## 第 7 步：RetainedGovernance.Authorize

插入位置：第 6 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func (s *RetainedGovernance) Authorize(ctx context.Context, actor Actor, operation string, document uint64) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authority.AcquireGovernance(ctx, actor, operation, document)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code>；<code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;RETAINED_GOVERNANCE_AUTHORITY_REQUIRED&quot;)</code>；<code>s.authority.AcquireGovernance(ctx, actor, operation, document)</code>；<code>return actor, err</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>s.authority.AcquireGovernance(ctx, actor, operation, document)</code>；<code>proof.OperationId != operation</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>s.authority.AcquireGovernance(ctx, actor, operation, document)</code>；<code>proof.Target.DocumentId != document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;RETAINED_GOVERNANCE_AUTHORITY_REQUIRED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;RETAINED_GOVERNANCE_PROOF_INVALID&quot;)</code>；<code>errors.New(&quot;RETAINED_GOVERNANCE_SOURCE_REQUIRED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s.authority == nil &#124;&#124; actor.Proof == nil &#124;&#124; !domain.ValidOperationID(operation) &#124;&#124; document == 0</code>。条件成立时执行 <code>return actor, domain.Violation(domain.TemporarilyUnavailable, &quot;RETAINED_GOVERNANCE_AUTHORITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != actor.ID &#124;&#124; proof.OperationId != operation &#124;&#124; proof.Purpose != domain.RetainedGovernancePurpose &#124;&#124; 		proof.Target == nil &#124;&#124; proof.Target.DocumentId …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return actor, errors.New(&quot;RETAINED_GOVERNANCE_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>source != nil &amp;&amp; source.ScopeKey == &quot;platform-admin:&quot;+domain.TextID(actor.ID) &amp;&amp; source.Epoch != 0</code>。
- 检查 <code>!governance</code>。条件成立时执行 <code>return actor, errors.New(&quot;RETAINED_GOVERNANCE_SOURCE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>len(merged) != len(proof.Sources)</code>。条件成立时执行 <code>return actor, errors.New(&quot;RETAINED_GOVERNANCE_IDENTITY_SOURCES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1585:3439 -->
````go
func (s *RetainedGovernance) Authorize(ctx context.Context, actor Actor, operation string, document uint64) (Actor, error) {
	if s.authority == nil || actor.Proof == nil || !domain.ValidOperationID(operation) || document == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "RETAINED_GOVERNANCE_AUTHORITY_REQUIRED")
	}
	proof, err := s.authority.AcquireGovernance(ctx, actor, operation, document)
	if err != nil {
		return actor, err
	}
	if proof == nil || proof.ActorId != actor.ID || proof.OperationId != operation || proof.Purpose != domain.RetainedGovernancePurpose ||
		proof.Target == nil || proof.Target.DocumentId != document || proof.Target.Purpose != domain.RetainedGovernancePurpose ||
		proof.IdentityTarget == nil || proof.IdentityTarget.Kind != pb.IdentityTarget_USER || proof.IdentityTarget.UserId != actor.ID || proof.IdentityTarget.OrganizationId != 0 || proof.IdentityTarget.SpaceId != 0 ||
		proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT || proof.Target.UnitId != 0 || proof.Target.BranchId != 0 || proof.Target.StructureId != 0 || proof.Target.WorkspaceEpoch != 0 ||
		proof.ExpiresUnixMs <= time.Now().UnixMilli() || !bytes.Equal(proof.SessionRef, actor.Proof.SessionRef) {
		return actor, errors.New("RETAINED_GOVERNANCE_PROOF_INVALID")
	}
	governance := false
	for _, source := range proof.Sources {
		if source != nil && source.ScopeKey == "platform-admin:"+domain.TextID(actor.ID) && source.Epoch != 0 {
			governance = true
		}
	}
	if !governance {
		return actor, errors.New("RETAINED_GOVERNANCE_SOURCE_REQUIRED")
	}
	merged, err := MergeSources(proof, actor.Proof)
	if err != nil {
		return actor, err
	}
	if len(merged) != len(proof.Sources) {
		return actor, errors.New("RETAINED_GOVERNANCE_IDENTITY_SOURCES_REQUIRED")
	}
	actor.Proof, actor.Sources = proof, proof.Sources
	return actor, nil
}

````

## 第 8 步：RetainedGovernance.Prepare

插入位置：第 7 步代码之后，紧接追加；完成稿约第 83 行。

handler先Authorize并标准Replay；Prepare只生成新号的不可变计划。来源锁
在run中一次排序，避免先持session再倒序申请Doc/KB形成反向锁顺序。

本步声明：`func (s *RetainedGovernance) Prepare(ctx context.Context, actor Actor, id uint64, action string, input dto.VersionedAction) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>repository.TrashSpaceRow(ctx, s.db, spaceID, false)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, s.db, kbID, false)</code>；<code>repository.TrashSourceHighWater(ctx, s.db, &quot;doc-acl:&quot;+domain.TextID(id))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>actor.ID</code>；<code>actor.Proof</code>；<code>actor.Sources</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>domain.TextID(id)</code>；<code>repository.RetainedDocumentResources(ctx, s.db, id)</code>；<code>lockRetainedRegistrationSources(ctx, tx, sources, id)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidateRetainedGovernance(facts, space, kb, action, input.ExpectedRevision)</code>；<code>Action: action</code>；<code>domain.ValidateRetainedGovernance(current, currentSpace, currentKB, action, input.ExpectedRevision)</code> |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>facts.Document.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>facts.Document.CurrentReleaseId != nil</code>。

<!-- write-step: 8 bytes=3439:5207 -->
````go
// handler先Authorize并标准Replay；Prepare只生成新号的不可变计划。来源锁
// 在run中一次排序，避免先持session再倒序申请Doc/KB形成反向锁顺序。
func (s *RetainedGovernance) Prepare(ctx context.Context, actor Actor, id uint64, action string, input dto.VersionedAction) (Actor, MutationBody, MutationPresentation, error) {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, nil, err
	}
	space, err := repository.TrashSpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return actor, nil, nil, err
	}
	var kb *dto.KnowledgeBase
	var kbID uint64
	if facts.Document.KnowledgeBaseId != nil {
		kbID, err = domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return actor, nil, nil, err
		}
		value, err := repository.TrashKnowledgeBaseRow(ctx, s.db, kbID, false)
		if err != nil {
			return actor, nil, nil, err
		}
		kb = &value
	}
	if err = domain.ValidateRetainedGovernance(facts, space, kb, action, input.ExpectedRevision); err != nil {
		return actor, nil, nil, err
	}
	acl, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+domain.TextID(id))
	if err != nil {
		return actor, nil, nil, err
	}
	publication, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+domain.TextID(id))
	if err != nil {
		return actor, nil, nil, err
	}
	purge, err := domain.NewPurgePlan(input.OperationId, actor.ID, false, facts, kbID, acl, publication)
	if err != nil {
		return actor, nil, nil, err
	}
	plan := domain.RetainedGovernancePlan{Purge: purge, Action: action, Resources: []uint64{}}
	if facts.Document.CurrentReleaseId != nil {
````

## 第 9 步：RetainedGovernance.Prepare（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 128 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>kb != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kb != nil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=5207:6804 -->
````go
		plan.Release, err = domain.CanonicalID(*facts.Document.CurrentReleaseId)
		if err != nil {
			return actor, nil, nil, err
		}
	}
	plan.Resources, err = repository.RetainedDocumentResources(ctx, s.db, id)
	if err != nil {
		return actor, nil, nil, err
	}
	sources := append([]*pb.SourceEpoch(nil), actor.Proof.Sources...)
	for _, source := range []struct{ key, revision string }{{"doc-acl:" + domain.TextID(id), facts.Document.AclRevision}, {"publication:" + domain.TextID(id), facts.Document.PublicationEpoch}} {
		epoch, err := domain.CanonicalRevision(source.revision)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: source.key, Epoch: epoch})
	}
	if kb != nil {
		epoch, err := domain.CanonicalRevision(kb.AclRevision)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kbID), Epoch: epoch})
		plan.KnowledgeBaseACL = epoch
	}
	actor.Sources = []*pb.SourceEpoch{}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := lockRetainedRegistrationSources(ctx, tx, sources, id); err != nil {
			return nil, err
		}
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		var currentKB *dto.KnowledgeBase
		if kb != nil {
			value, err := repository.TrashKnowledgeBaseRow(ctx, tx, kbID, true)
			if err != nil {
				return nil, err
			}
			currentKB = &value
		}
		currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
		if err != nil {
			return nil, err
		}
````

## 第 10 步：RetainedGovernance.Prepare（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 174 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Sequence != facts.Sequence &#124;&#124; !reflect.DeepEqual(current.Document, facts.Document) &#124;&#124; !reflect.DeepEqual(currentKB, kb)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return retainedGovernanceProgress(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return value, actor.Proof.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6804:8306 -->
````go
		if err = domain.ValidateRetainedGovernance(current, currentSpace, currentKB, action, input.ExpectedRevision); err != nil {
			return nil, err
		}
		if current.Sequence != facts.Sequence || !reflect.DeepEqual(current.Document, facts.Document) || !reflect.DeepEqual(currentKB, kb) {
			return nil, domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
		}
		if err = repository.BeginRetainedGovernance(ctx, tx, plan); err != nil {
			return nil, err
		}
		for _, source := range purge.Scopes {
			if err = QueueClosure(ctx, tx, source.Operation, actor.ID, domain.Closure{Scope: source.Key, Epoch: source.Epoch, Entity: id, Action: domain.RetainedGovernancePurpose}); err != nil {
				return nil, err
			}
		}
		payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: input.OperationId, Type: "DOCUMENT_UNPUBLISHED", Document: id, Sequence: purge.Sequence, PublicationEpoch: purge.Publication, OccurredAt: time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		if err = repository.InsertIndexEvent(ctx, tx, id, purge.Sequence, input.OperationId, "DOCUMENT_UNPUBLISHED", payload); err != nil {
			return nil, err
		}
		return retainedGovernanceProgress(input.OperationId, "PREPARING", id), nil
	}
	present := func(_ context.Context, _ *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		// 只发送最小固定receipt；正文和已关闭Doc来源不被治理身份重新开放。
		return value, actor.Proof.Sources, nil
	}
	return actor, run, present, nil
}

````

## 第 11 步：lockRetainedRegistrationSources

插入位置：第 10 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`func lockRetainedRegistrationSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source})</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.RetainedClosedSourceDrained(ctx, tx, source.ScopeKey, source.Epoch, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source})</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.RetainedClosedSourceDrained(ctx, tx, source.ScopeKey, source.Epoch, document)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>append(&#91;&#93;*pb.SourceEpoch(nil), sources...)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code>；<code>repository.RetainedClosedSourceDrained(ctx, tx, source.ScopeKey, source.Epoch, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93;.ScopeKey &lt; ordered&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>source.ScopeKey != &quot;doc-acl:&quot;+domain.TextID(document) &amp;&amp; source.ScopeKey != &quot;publication:&quot;+domain.TextID(document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch != source.Epoch</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.State == &quot;BLOCKED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.State != &quot;OPEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8306:9694 -->
````go
func lockRetainedRegistrationSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64) error {
	ordered := append([]*pb.SourceEpoch(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ScopeKey < ordered[j].ScopeKey })
	for _, source := range ordered {
		if source.ScopeKey != "doc-acl:"+domain.TextID(document) && source.ScopeKey != "publication:"+domain.TextID(document) {
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{source}); err != nil {
				return err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		if gate.Epoch != source.Epoch {
			return domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
		pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
		if err != nil {
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
		}
		if gate.State == "BLOCKED" {
			drained, err := repository.RetainedClosedSourceDrained(ctx, tx, source.ScopeKey, source.Epoch, document)
			if err != nil {
				return err
			}
			if !drained {
				return domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
			}
		} else if gate.State != "OPEN" {
			return domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
	}
	return nil
}

````

## 第 12 步：retainedGovernanceProgress

插入位置：第 11 步代码之后，紧接追加；完成稿约第 243 行。

本步声明：`func retainedGovernanceProgress(operation, state string, document uint64) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code>；<code>&quot;/v1/operations/&quot; + operation</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code>；<code>state != &quot;SUCCEEDED&quot;</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>value</code> |

**跟着本段代码走一遍：**

- 检查 <code>state != &quot;SUCCEEDED&quot;</code>。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9694:9938 -->
````go
func retainedGovernanceProgress(operation, state string, document uint64) map[string]any {
	value := OperationResult(operation, state, document)
	if state != "SUCCEEDED" {
		value["poll_url"] = "/v1/operations/" + operation
	}
	return value
}

````

## 第 13 步：RetainedGovernance.Lookup

插入位置：第 12 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func (s *RetainedGovernance) Lookup(ctx context.Context, operation string) (repository.RetainedGovernanceRecord, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, s.db, operation, false)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, s.db, operation, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.RetainedGovernanceRecord</code> | 使用 <code>repository.RetainedGovernanceRecord</code> 类型；其定义处的职责：保存留存治理计划及资源和导出排空证据。<br>由 <code>repository.RetainedGovernanceRow(ctx, s.db, operation, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.RetainedGovernanceRow(ctx, s.db, operation, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.RetainedGovernanceRow(ctx, s.db, operation, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9938:10133 -->
````go
func (s *RetainedGovernance) Lookup(ctx context.Context, operation string) (repository.RetainedGovernanceRecord, error) {
	return repository.RetainedGovernanceRow(ctx, s.db, operation, false)
}

````

## 第 14 步：RetainedGovernance.Progress

插入位置：第 13 步代码之后，紧接追加；完成稿约第 255 行。

本步声明：`func (s *RetainedGovernance) Progress(actor Actor, operation string, document uint64) ReadBody`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, tx, operation, false)</code>；<code>repository.LockedOperation(ctx, tx, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>row.Plan.Purge.Document != document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		row, err := repository.RetainedGovernanceRow(ctx, tx, operation, false) 		if err != nil { 			return nil, err 		} 		 …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		row, err := repository.RetainedGovernanceRow(ctx, tx, operation, false) 		if err != nil { 			return nil, err …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>row.Plan.Purge.Actor != actor.ID &#124;&#124; row.Plan.Purge.Document != document</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;RETAINED_ORIGINAL_ACTOR_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>receipt.ActorID != actor.ID &#124;&#124; receipt.Kind != domain.RetainedGovernanceKind(row.Plan.Action) &#124;&#124; json.Unmarshal(receipt.Result, &amp;result) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RETAINED_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10133:11067 -->
````go
func (s *RetainedGovernance) Progress(actor Actor, operation string, document uint64) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		row, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
		if err != nil {
			return nil, err
		}
		if row.Plan.Purge.Actor != actor.ID || row.Plan.Purge.Document != document {
			return nil, domain.Violation(domain.PermissionDenied, "RETAINED_ORIGINAL_ACTOR_REQUIRED")
		}
		receipt, err := repository.LockedOperation(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if receipt.ActorID != actor.ID || receipt.Kind != domain.RetainedGovernanceKind(row.Plan.Action) || json.Unmarshal(receipt.Result, &result) != nil {
			return nil, domain.Violation(domain.StateConflict, "RETAINED_RECEIPT_CHANGED")
		}
		return result, nil
	}
}

// retained_governance_read.go：保留原实现，按业务合并文件。

````

## 第 15 步：RetainedGovernance.Metadata

插入位置：第 14 步代码之后，紧接追加；完成稿约第 278 行。

治理页面只取得孤立留存的元数据和当前控制修订。使用真实平台资格登记输出，
不为已关闭的Doc/KB来源创建正文read admission，也不重新开放协作能力。

本步声明：`func (s *RetainedGovernance) Metadata(document uint64) ReadBody`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, tx, document, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		facts, err := repository.TrashDocumentRow(ctx, tx, document, false) 		if err != nil { 			return nil, err 		} 		spac …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		facts, err := repository.TrashDocumentRow(ctx, tx, document, false) 		if err != nil { 			return nil, err 		} …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>facts.Document.KnowledgeBaseId != nil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return facts.Document, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11067:12420 -->
````go
// 治理页面只取得孤立留存的元数据和当前控制修订。使用真实平台资格登记输出，
// 不为已关闭的Doc/KB来源创建正文read admission，也不重新开放协作能力。
func (s *RetainedGovernance) Metadata(document uint64) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		facts, err := repository.TrashDocumentRow(ctx, tx, document, false)
		if err != nil {
			return nil, err
		}
		spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
		if err != nil {
			return nil, err
		}
		space, err := repository.TrashSpaceRow(ctx, tx, spaceID, false)
		if err != nil {
			return nil, err
		}
		var kb *dto.KnowledgeBase
		if facts.Document.KnowledgeBaseId != nil {
			id, err := domain.CanonicalID(*facts.Document.KnowledgeBaseId)
			if err != nil {
				return nil, err
			}
			value, err := repository.TrashKnowledgeBaseRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			kb = &value
		}
		// PURGE的前置事实包含仍发布及已撤下两种合法留存；ACTIVE/CLOSING/PURGED
		// 均不由这个入口返回。实际写入还会在事务锁下重新比较这份控制修订。
		if err = domain.ValidateRetainedGovernance(facts, space, kb, "PURGE", facts.Document.ControlRevision); err != nil {
			return nil, err
		}
		return facts.Document, nil
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/retained_governance_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
