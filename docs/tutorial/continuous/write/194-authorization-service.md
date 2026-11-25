# 分步编写 `services/document/internal/logic/document/authorization_service.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/194-authorization-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。

事务外鉴权便于调用远端，事务内ValidateAccess与LockSources负责堵住鉴权后撤权的竞争窗口。

写之前先按这条执行路线理解各段的连接：

1. Identity.Authenticate取得账号权威证明
2. Access.Authorize合并空间/KB/文档事实
3. AuthorizationChanges.Prepare生成变更闭包
4. QueueClosure冻结旧来源
5. RecoverOne等两参与者停止输出后完成回执。

本文件需要守住的边界：

- 恢复必须保留原actor、会话、目标、操作号和来源集合
- 同代BLOCKED不得重开
- 未知ACK不能当作成功。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/authorization_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/authorization_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/authorization_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/authorization_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:603 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// access.go：保留原实现，按业务合并文件。

````

## 第 2 步：Access

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type Access struct {`。

**这一段为什么这样写：** 把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>*repository.Access</code> | 使用 <code>repository.Access</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。 |
| <code>identity</code> | <code>*Identity</code> | Doc身份核验用例，负责取得并核验Account会话/成员准入证明。 |

<!-- write-step: 2 bytes=603:680 -->
````go
type Access struct {
	repository *repository.Access
	identity   *Identity
}

````

## 第 3 步：NewAccess

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewAccess(repository *repository.Access, identity *Identity) *Access`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>*repository.Access</code> | 使用 <code>repository.Access</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>repository: repository</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>实际返回表达式：<code>&amp;Access{repository: repository, identity: identity}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Access{repository: repository, identity: identity}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=680:819 -->
````go
func NewAccess(repository *repository.Access, identity *Identity) *Access {
	return &Access{repository: repository, identity: identity}
}

````

## 第 4 步：DocumentAccess

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type DocumentAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>dto.Document</code> | 已核验可访问的文档元数据。 |
| <code>Permissions</code> | <code>domain.Permissions</code> | 使用 <code>domain.Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。 |
| <code>Sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 本次交付实际依赖的权限来源和代次；交付前须全部复核。 |
| <code>Proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | Account核验得到的身份/成员准入证明集合。 |
| <code>KBRevision</code> | <code>uint64</code> | 核验时的知识库ACL修订，防止库权限变更后沿用旧判断。 |

<!-- write-step: 4 bytes=819:997 -->
````go
type DocumentAccess struct {
	Document    dto.Document
	Permissions domain.Permissions
	Sources     []*pb.SourceEpoch
	Proofs      []*pb.IdentityAdmission
	KBRevision  uint64
}

````

## 第 5 步：Access.Authorize

插入位置：第 4 步代码之后，紧接追加；完成稿约第 41 行。

Authorize先核验当前来源；业务事务内仍须持锁复核，不能把事务外快照当作最终准入。

本步声明：`func (s *Access) Authorize(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Document(ctx, id)</code>；<code>s.repository.Space(ctx, spaceID)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.repository.Document(ctx, id)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)</code>；<code>s.repository.Grants(ctx, id, actor.ID)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)</code>；<code>s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_READ_NOT_ALLOWED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>!errors.As(err, &amp;business) &#124;&#124; business.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>document.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return access, domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=997:3098 -->
````go
// Authorize先核验当前来源；业务事务内仍须持锁复核，不能把事务外快照当作最终准入。
func (s *Access) Authorize(ctx context.Context, actor Actor, id uint64, purpose string) (DocumentAccess, error) {
	var access DocumentAccess
	document, err := s.repository.Document(ctx, id)
	if err != nil {
		return access, err
	}
	access.Document = document
	spaceID, _ := domain.CanonicalID(document.SpaceId)
	space, err := s.repository.Space(ctx, spaceID)
	if err != nil {
		return access, err
	}
	facts := domain.PermissionFacts{Actor: actor.ID, Responsible: document.ResponsibleUserId != nil && *document.ResponsibleUserId == domain.TextID(actor.ID), PersonalOwner: space.OwnerUserId != nil && *space.OwnerUserId == domain.TextID(actor.ID)}
	proofs := []*pb.IdentityAdmission{actor.Proof}
	if space.Kind == "ORGANIZATION" {
		_, proof, err := s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, id)
		if err == nil {
			facts.OrganizationAdmin = true
			proofs = append(proofs, proof)
		} else {
			var business domain.RuleViolation
			if !errors.As(err, &business) || business.Kind != domain.PermissionDenied {
				return access, err
			}
			_, proof, err = s.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, id)
			if err != nil {
				return access, err
			}
			proofs = append(proofs, proof)
		}
	}
	if document.KnowledgeBaseId != nil {
		kb, _ := domain.CanonicalID(*document.KnowledgeBaseId)

		state, revision, err := s.repository.KnowledgeBase(ctx, kb, spaceID)
		access.KBRevision = revision
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return access, err
		}
		if errors.Is(err, sql.ErrNoRows) || state != "ACTIVE" {
			return access, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
		}
		if role, err := s.repository.KnowledgeBaseRole(ctx, kb, actor.ID); err == nil {
			facts.KnowledgeBaseAdmin = role == "ADMIN"
		} else if !errors.Is(err, sql.ErrNoRows) {
			// 没有KB角色是确定资格不足；数据库未知不能当作“普通成员”继续授权。
````

## 第 6 步：Access.Authorize（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 88 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return access, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Read</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_READ_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>document.KnowledgeBaseId != nil</code>。
- 此处通过 <code>return access, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3098:4126 -->
````go
			return access, err
		}
	}
	facts.Grants, err = s.repository.Grants(ctx, id, actor.ID)
	if err != nil {
		return access, err
	}
	facts.Units, err = s.repository.Units(ctx, document.CurrentStructureId, id)
	if err != nil {
		return access, err
	}
	access.Permissions = domain.ResolvePermissions(facts)
	if !access.Permissions.Read {
		return access, domain.Violation(domain.PermissionDenied, "DOCUMENT_READ_NOT_ALLOWED")
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return access, err
		}
	}
	access.Proofs = proofs
	access.Sources, err = MergeSources(proofs...)
	if err != nil {
		return access, err
	}
	revision, _ := domain.CanonicalID(document.AclRevision)
	access.Sources = append(access.Sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(id), Epoch: revision})
	if document.KnowledgeBaseId != nil {
		access.Sources = append(access.Sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *document.KnowledgeBaseId, Epoch: access.KBRevision})
	}
	return access, nil
}
````

## 第 7 步：ValidateAccess

插入位置：第 6 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func ValidateAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess) (dto.Document, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.DocumentRow(ctx, tx, id, true)</code>；<code>repository.LockedKnowledgeBase(ctx, tx, *current.KnowledgeBaseId)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.DocumentRow(ctx, tx, id, true)</code>；<code>repository.LockedKnowledgeBase(ctx, tx, *current.KnowledgeBaseId)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 在此声明中的实际使用：<code>access.Document</code>；<code>access.KBRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_ACCESS_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>current.AclRevision != access.Document.AclRevision &#124;&#124; current.CurrentStructureId != access.Document.CurrentStructureId &#124;&#124; current.ResponsibleUserId == nil != (access.Document.ResponsibleUserId == nil)</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;DOCUMENT_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil &amp;&amp; !errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; state != &quot;ACTIVE&quot; &#124;&#124; revision != access.KBRevision</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4126:5194 -->
````go
func ValidateAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess) (dto.Document, error) {
	id, _ := domain.CanonicalID(access.Document.Id)
	current, err := repository.DocumentRow(ctx, tx, id, true)
	if err != nil {
		return current, err
	}
	if current.AclRevision != access.Document.AclRevision || current.CurrentStructureId != access.Document.CurrentStructureId || current.ResponsibleUserId == nil != (access.Document.ResponsibleUserId == nil) {
		return current, domain.Violation(domain.StateConflict, "DOCUMENT_ACCESS_CHANGED")
	}
	if current.KnowledgeBaseId != nil {
		state, revision, err := repository.LockedKnowledgeBase(ctx, tx, *current.KnowledgeBaseId)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			// SQL未知与已确认资格变化不同；不能把超时/断连改成可重试的页面冲突。
			return current, err
		}
		if errors.Is(err, sql.ErrNoRows) || state != "ACTIVE" || revision != access.KBRevision {
			return current, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
	}
	return current, nil
}
````

## 第 8 步：Access.SpaceIdentity

插入位置：第 7 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`func (s *Access) SpaceIdentity(ctx context.Context, actor Actor, spaceID uint64, capability pb.IdentityCapability, purpose string, document uint64) (dto.Space, *pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Space(ctx, spaceID)</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code>；<code>s.identity.Acquire(ctx, actor, capability, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code>；<code>s.identity.Acquire(ctx, actor, capability, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>spaceID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.repository.Space(ctx, spaceID)</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>s.identity.Acquire(ctx, actor, capability, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code>；<code>s.identity.Acquire(ctx, actor, capability, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code>；<code>s.identity.Acquire(ctx, actor, capability, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PERSONAL_SPACE_NOT_OWNED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return space, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind == &quot;PERSONAL&quot;</code>。
- 检查 <code>space.OwnerUserId == nil &#124;&#124; *space.OwnerUserId != domain.TextID(actor.ID)</code>。条件成立时执行 <code>return space, nil, domain.Violation(domain.PermissionDenied, &quot;PERSONAL_SPACE_NOT_OWNED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return space, proof, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5194:6223 -->
````go
func (s *Access) SpaceIdentity(ctx context.Context, actor Actor, spaceID uint64, capability pb.IdentityCapability, purpose string, document uint64) (dto.Space, *pb.IdentityAdmission, error) {
	space, err := s.repository.Space(ctx, spaceID)
	if err != nil {
		return space, nil, err
	}
	if space.Kind == "PERSONAL" {
		if space.OwnerUserId == nil || *space.OwnerUserId != domain.TextID(actor.ID) {
			return space, nil, domain.Violation(domain.PermissionDenied, "PERSONAL_SPACE_NOT_OWNED")
		}
		proof, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)
		return space, proof, err
	}
	org, _ := domain.CanonicalID(*space.OrganizationId)
	proof, err := s.identity.Acquire(ctx, actor, capability, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: org, UserId: actor.ID}, purpose, document, 0)
	return space, proof, err
}

// authorization.go：保留原实现，按业务合并文件。

````

## 第 9 步：Authorizations

插入位置：第 8 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`type Authorizations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Authorizations</code> | 使用 <code>repository.Authorizations</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。 |
| <code>fences</code> | <code>*Fences</code> | 使用 <code>Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |

<!-- write-step: 9 bytes=6223:6355 -->
````go
type Authorizations struct {
	db         *sql.DB
	repository *repository.Authorizations
	fences     *Fences
	outputs    *Outputs
}

````

## 第 10 步：NewAuthorizations

插入位置：第 9 步代码之后，紧接追加；完成稿约第 167 行。

本步声明：`func NewAuthorizations(db *sql.DB, repository *repository.Authorizations, fences *Fences, outputs *Outputs) *Authorizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repository</code> | <code>*repository.Authorizations</code> | 使用 <code>repository.Authorizations</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>repository: repository</code> |
| <code>fences</code> | <code>*Fences</code> | 使用 <code>Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>fences: fences</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authorizations</code> | 实际返回表达式：<code>&amp;Authorizations{db: db, repository: repository, fences: fences, outputs: outputs}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authorizations{db: db, repository: repository, fences: fences, outputs: outputs}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6355:6574 -->
````go
func NewAuthorizations(db *sql.DB, repository *repository.Authorizations, fences *Fences, outputs *Outputs) *Authorizations {
	return &Authorizations{db: db, repository: repository, fences: fences, outputs: outputs}
}

````

## 第 11 步：QueueClosure

插入位置：第 10 步代码之后，紧接追加；完成稿约第 171 行。

QueueClosure与业务事实使用同一事务：先固定关闭计划，再冻结旧来源并登记可恢复任务。

本步声明：`func QueueClosure(ctx context.Context, tx *sql.Tx, operation string, actor uint64, closure domain.Closure) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding)</code>；<code>repository.BlockFenceScope(ctx, tx, closure.Scope, closure.Epoch, operation)</code>；<code>repository.InsertAuthorizationTask(ctx, tx, operation, payload)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding)</code>；<code>repository.BlockFenceScope(ctx, tx, closure.Scope, closure.Epoch, operation)</code>；<code>repository.InsertAuthorizationTask(ctx, tx, operation, payload)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding)</code>；<code>repository.BlockFenceScope(ctx, tx, closure.Scope, closure.Epoch, operation)</code>；<code>repository.InsertAuthorizationTask(ctx, tx, operation, payload)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding)</code> |
| <code>closure</code> | <code>domain.Closure</code> | 使用 <code>domain.Closure</code> 类型；其定义处的职责：Closure固定已提交的关闭计划；恢复必须继续原编号、原范围及原代次。<br>在此声明中的实际使用：<code>closure.Scope</code>；<code>closure.Epoch</code>；<code>json.Marshal(closure)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>repository.InsertAuthorizationTask(ctx, tx, operation, payload)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return repository.InsertAuthorizationTask(ctx, tx, operation, payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6574:7442 -->
````go
// QueueClosure与业务事实使用同一事务：先固定关闭计划，再冻结旧来源并登记可恢复任务。
func QueueClosure(ctx context.Context, tx *sql.Tx, operation string, actor uint64, closure domain.Closure) error {
	digest := platform.SourceClosureDigest(closure.Scope, closure.Epoch)
	binding, _ := json.Marshal(map[string]string{"bootstrap_target_digest": hex.EncodeToString(digest), "generation": "1"})
	if err := repository.InsertAuthorization(ctx, tx, operation, actor, closure.Scope, closure.Epoch, binding); err != nil {
		return err
	}
	if err := repository.BlockFenceScope(ctx, tx, closure.Scope, closure.Epoch, operation); err != nil {
		return err
	}
	payload, _ := json.Marshal(closure)
	return repository.InsertAuthorizationTask(ctx, tx, operation, payload)
}

// authorization_change.go：保留原实现，按业务合并文件。

````

## 第 12 步：AuthorizationChanges

插入位置：第 11 步代码之后，紧接追加；完成稿约第 187 行。

本步声明：`type AuthorizationChanges struct {`。

**这一段为什么这样写：** 先确认操作者可管理文档并绑定受影响身份，再准备授权变更的事务执行体；权限撤销和来源关闭需沿用同一操作的恢复边界。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | Doc身份核验用例，负责取得并核验Account会话/成员准入证明。 |
| <code>access</code> | <code>*Access</code> | 当前文档权限核验用例，修改前重新读取并验证真实授权来源。 |

<!-- write-step: 12 bytes=7442:7536 -->
````go
type AuthorizationChanges struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

````

## 第 13 步：NewAuthorizationChanges

插入位置：第 12 步代码之后，紧接追加；完成稿约第 193 行。

本步声明：`func NewAuthorizationChanges(db *sql.DB, identity *Identity, access *Access) *AuthorizationChanges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AuthorizationChanges</code> | 使用 <code>AuthorizationChanges</code> 类型；其定义处的职责：先确认操作者可管理文档并绑定受影响身份，再准备授权变更的事务执行体；权限撤销和来源关闭需沿用同一操作的恢复边界。<br>实际返回表达式：<code>&amp;AuthorizationChanges{db: db, identity: identity, access: access}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AuthorizationChanges{db: db, identity: identity, access: access}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7536:7714 -->
````go
func NewAuthorizationChanges(db *sql.DB, identity *Identity, access *Access) *AuthorizationChanges {
	return &AuthorizationChanges{db: db, identity: identity, access: access}
}

````

## 第 14 步：AuthorizationChanges.Prepare

插入位置：第 13 步代码之后，紧接追加；完成稿约第 197 行。

Prepare绑定实际文档及每个受影响身份；最终写入仍在共享变更事务中持锁重核。

本步声明：`func (s *AuthorizationChanges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.AuthorizationChange) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;CHANGE_AUTHORIZATION&quot;)</code>；<code>repository.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.identity.Target(ctx, actor, capability, target, &quot;CHANGE_AUTHORIZATION&quot;, id)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;CHANGE_AUTHORIZATION&quot;)</code>；<code>return actor, nil, err</code>；<code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;CHANGE_AUTHORIZATION&quot;)</code>；<code>s.identity.Target(ctx, actor, capability, target, &quot;CHANGE_AUTHORIZATION&quot;, id)</code>；<code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, scope, id)</code>；<code>scope != id</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.AuthorizationChange</code> | 在此声明中的实际使用：<code>input.Changes</code>；<code>input.ExpectedAclRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=7714:9574 -->
````go
// Prepare绑定实际文档及每个受影响身份；最终写入仍在共享变更事务中持锁重核。
func (s *AuthorizationChanges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.AuthorizationChange) (Actor, MutationBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "CHANGE_AUTHORIZATION")
	if err != nil {
		return actor, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	actor.Sources = access.Sources
	var targets []*pb.IdentityAdmission
	for _, change := range input.Changes {
		user, err := domain.CanonicalID(change.Grant.UserId)
		if err != nil {
			return actor, nil, err
		}
		spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
		space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
		if err != nil {
			return actor, nil, err
		}
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}
		capability := pb.IdentityCapability_ACTIVE_ACCOUNT
		if space.Kind == "ORGANIZATION" {
			organization, _ := domain.CanonicalID(*space.OrganizationId)
			target = &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}
			capability = pb.IdentityCapability_ORGANIZATION_MEMBER
		}
		// 不能借用另一组织的member证明，目标身份必须绑定当前文档所属空间。
		proof, err := s.identity.Target(ctx, actor, capability, target, "CHANGE_AUTHORIZATION", id)
		if err != nil {
			return actor, nil, err
		}
		targets = append(targets, proof)
	}
	for _, proof := range targets {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return actor, nil, err
		}
	}
	sources, err := MergeSources(append(access.Proofs, targets...)...)
	if err != nil {
		return actor, nil, err
	}
	baseSources, _ := MergeSources(access.Proofs...)
````

## 第 15 步：AuthorizationChanges.Prepare（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 242 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.AclRevision != input.ExpectedAclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;ACL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>change.Action != &quot;GRANT&quot; &amp;&amp; change.Action != &quot;REVOKE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;GRANT_ACTION_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>grant.Capability == &quot;EDIT&quot;</code>。
- 在本段的匿名函数内：检查 <code>grant.ScopeKind != &quot;UNIT&quot; &amp;&amp; grant.ScopeKind != &quot;CHAPTER&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;EDIT_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>repository.EditGrantScope(ctx, tx, grant.ScopeKind, scope, id) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;GRANT_SCOPE_NOT_IN_DOCUMENT&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>(grant.Capability != &quot;MANAGE&quot; &amp;&amp; grant.Capability != &quot;REVIEW&quot; &amp;&amp; grant.Capability != &quot;COMMENT&quot;) &#124;&#124; grant.ScopeKind != &quot;DOCUMENT&quot; &#124;&#124; scope != id</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;DOCUMENT_CAPABILITY_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>change.Action == &quot;REVOKE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=9574:11558 -->
````go
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.AclRevision != input.ExpectedAclRevision {
			return nil, domain.Violation(domain.StateConflict, "ACL_REVISION_MISMATCH")
		}
		for _, change := range input.Changes {
			grant := change.Grant
			user, _ := domain.CanonicalID(grant.UserId)
			scope, err := domain.CanonicalID(grant.ScopeId)
			if err != nil {
				return nil, err
			}
			if change.Action != "GRANT" && change.Action != "REVOKE" {
				return nil, domain.Violation(domain.InvalidRequest, "GRANT_ACTION_INVALID")
			}
			if grant.Capability == "EDIT" {
				if grant.ScopeKind != "UNIT" && grant.ScopeKind != "CHAPTER" {
					return nil, domain.Violation(domain.InvalidRequest, "EDIT_SCOPE_INVALID")
				}
				if repository.EditGrantScope(ctx, tx, grant.ScopeKind, scope, id) != nil {
					return nil, domain.Violation(domain.InvalidRequest, "GRANT_SCOPE_NOT_IN_DOCUMENT")
				}
			} else if (grant.Capability != "MANAGE" && grant.Capability != "REVIEW" && grant.Capability != "COMMENT") || grant.ScopeKind != "DOCUMENT" || scope != id {
				return nil, domain.Violation(domain.InvalidRequest, "DOCUMENT_CAPABILITY_SCOPE_INVALID")
			}
			state := "ACTIVE"
			if change.Action == "REVOKE" {
				state = "CLOSED"
			}
			fact := domain.GrantFact{User: user, Kind: grant.ScopeKind, Scope: scope, Capability: grant.Capability}
			if err = repository.SetContentGrant(ctx, tx, id, fact, state); err != nil {
				return nil, err
			}
		}
		if err = repository.AdvanceDocumentACL(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
````

## 第 16 步：AuthorizationChanges.Prepare（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 287 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=11558:11729 -->
````go
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// authorization_recovery.go：保留原实现，按业务合并文件。

````

## 第 17 步：Authorizations.RecoverOne

插入位置：第 16 步代码之后，紧接追加；完成稿约第 294 行。

RecoverOne先持久领取，再执行跨服务关闭；未知ACK只能以原编号重试，不能落业务终态。

本步声明：`func (s *Authorizations) RecoverOne(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 2500*time.Millisecond)</code>；<code>s.retry(parent, task.ID, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 2500*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>repository.RunAuthorizationTask(ctx, tx, task.ID, instance) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>json.Unmarshal(task.Payload, &amp;closure) != nil &#124;&#124; !platform.ValidDocumentScope(closure.Scope)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; !platform.MatchesBootstrapAck(request, ack, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; owner.Instance != instance &#124;&#124; owner.State != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>closure.Action == &quot;TRASH_DOCUMENT&quot;</code>。
- 检查 <code>closure.Action == &quot;CLOSE_KB&quot;</code>。

<!-- write-step: 17 bytes=11729:13454 -->
````go
// RecoverOne先持久领取，再执行跨服务关闭；未知ACK只能以原编号重试，不能落业务终态。
func (s *Authorizations) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimAuthorization(ctx, tx)
	if err != nil {
		return
	}
	instance := s.outputs.Self().ID
	if repository.RunAuthorizationTask(ctx, tx, task.ID, instance) != nil {
		return
	}
	if tx.Commit() != nil {
		return
	}
	var closure domain.Closure
	if json.Unmarshal(task.Payload, &closure) != nil || !platform.ValidDocumentScope(closure.Scope) {
		s.retry(parent, task.ID, instance)
		return
	}
	request := &pb.FenceRequest{RevocationId: task.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: closure.Scope, Epoch: closure.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(closure.Scope, closure.Epoch)}
	ack, err := s.fences.Close(ctx, request)
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		s.retry(parent, task.ID, instance)
		return
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	owner, err := repository.LockedTaskOwnership(ctx, tx, task.ID)
	if err != nil || owner.Instance != instance || owner.State != "RUNNING" {
		return
	}
	// 实例租约只允许领取任务；业务终态仍要求当前任务所有者和真实关闭ACK。
	if closure.Action == "TRASH_DOCUMENT" {
		err = repository.TrashClosingDocument(ctx, tx, closure.Entity)
	} else if closure.Action == "CLOSE_KB" {
		err = repository.TrashClosingKnowledgeBaseDocuments(ctx, tx, closure.Entity)
````

## 第 18 步：Authorizations.RecoverOne（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 339 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 18 bytes=13454:13769 -->
````go
		if err == nil {
			err = repository.CloseKnowledgeBase(ctx, tx, closure.Entity)
		}
	}
	if err == nil {
		err = repository.CompleteAuthorizationReceipt(ctx, tx, task.Operation)
	}
	if err == nil {
		err = repository.CompleteAuthorizationTask(ctx, tx, task.ID, instance)
	}
	if err == nil {
		_ = tx.Commit()
	}
}
````

## 第 19 步：Authorizations.retry

插入位置：第 18 步代码之后，紧接追加；完成稿约第 353 行。

本步声明：`func (s *Authorizations) retry(parent context.Context, id uint64, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, time.Second)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.repository.Retry(ctx, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Retry(ctx, id, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 19 bytes=13769:14042 -->
````go
func (s *Authorizations) retry(parent context.Context, id uint64, instance string) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	s.repository.Retry(ctx, id, instance)
}

// authorization_status.go：保留原实现，按业务合并文件。

````

## 第 20 步：Authorizations.Presentation

插入位置：第 19 步代码之后，紧接追加；完成稿约第 361 行。

本步声明：`func (s *Authorizations) Presentation(actor Actor) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, operation map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		id, ok := operation&#91;&quot;operation_id&quot;&#93;.(string) 		if !ok { 			return nil, nil, erro …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, operation map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		id, ok := operation&#91;&quot;operation_id&quot;&#93;.(string) 		if !ok { 			return nil, ni …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return nil, nil, errors.New(&quot;AUTHORIZATION_RESULT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>value.State == &quot;SUCCEEDED&quot; &amp;&amp; operation&#91;&quot;state&quot;&#93; != &quot;SUCCEEDED&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>count == 0</code>。
- 在本段的匿名函数内：此处通过 <code>return value, actor.Proof.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=14042:15068 -->
````go
func (s *Authorizations) Presentation(actor Actor) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, operation map[string]any) (any, []*pb.SourceEpoch, error) {
		id, ok := operation["operation_id"].(string)
		if !ok {
			return nil, nil, errors.New("AUTHORIZATION_RESULT_UNAVAILABLE")
		}
		facts, err := repository.AuthorizationProjection(ctx, tx, id, actor.ID)
		if err != nil {
			return nil, nil, err
		}
		value := dto.AuthorizationResult{OperationId: id, State: facts.State, ScopeKey: facts.Scope, Epoch: facts.Epoch, MissingParticipants: []string{}}
		if value.State == "SUCCEEDED" && operation["state"] != "SUCCEEDED" {
			value.State = "FENCING"
		}
		for _, participant := range []string{"edge", "collaboration"} {
			count, err := repository.ProjectionAcknowledgment(ctx, tx, id, participant)
			if err != nil {
				return nil, nil, err
			}
			if count == 0 {
				value.MissingParticipants = append(value.MissingParticipants, participant)
			}
		}
		return value, actor.Proof.Sources, nil
	}
}
````

## 第 21 步：Authorizations.Progress

插入位置：第 20 步代码之后，紧接追加；完成稿约第 387 行。

本步声明：`func (s *Authorizations) Progress(actor Actor, operation string) ReadBody`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.AuthorizationProgress(ctx, tx, operation, actor.ID)</code>；<code>OperationId: operation</code>；<code>repository.ProgressAcknowledgment(ctx, tx, operation, participant, value.ScopeKey, value.Epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		facts, err := repository.AuthorizationProgress(ctx, tx, operation, actor.ID) 		if err != nil { 			return nil, domai …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		facts, err := repository.AuthorizationProgress(ctx, tx, operation, actor.ID) 		if err != nil { 			return nil …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>value.State == &quot;SUCCEEDED&quot; &amp;&amp; facts.ReceiptState != &quot;SUCCEEDED&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;AUTHORIZATION_PROGRESS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>count == 0</code>。
- 在本段的匿名函数内：此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=15068:16214 -->
````go
func (s *Authorizations) Progress(actor Actor, operation string) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		facts, err := repository.AuthorizationProgress(ctx, tx, operation, actor.ID)
		if err != nil {
			return nil, domain.Violation(domain.EntityNotFound, "OPERATION_NOT_FOUND")
		}
		value := dto.AuthorizationResult{OperationId: operation, State: facts.State, ScopeKey: facts.Scope, Epoch: facts.Epoch, MissingParticipants: []string{}}
		// 输出ACK只证明旧输出停止；最终业务事务尚未提交时仍对外显示FENCING。
		if value.State == "SUCCEEDED" && facts.ReceiptState != "SUCCEEDED" {
			value.State = "FENCING"
		}
		for _, participant := range []string{"edge", "collaboration"} {
			count, err := repository.ProgressAcknowledgment(ctx, tx, operation, participant, value.ScopeKey, value.Epoch)
			if err != nil {
				return nil, errors.New("AUTHORIZATION_PROGRESS_UNAVAILABLE")
			}
			if count == 0 {
				value.MissingParticipants = append(value.MissingParticipants, participant)
			}
		}
		return value, nil
	}
}

// departure.go：保留原实现，按业务合并文件。

````

## 第 22 步：DepartureAuthority

插入位置：第 21 步代码之后，紧接追加；完成稿约第 413 行。

本步声明：`type DepartureAuthority interface {`。

**这一段为什么这样写：** 修复组织成员来源门前，先取得Account的原退出结果，不能仅靠超时重开门。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>RolledBack</code> | <code>(context.Context, domain.DepartureObservation) (bool, error)</code> | ctx和退出观察事实用于查询Account权威结果；true是原退出已回滚，error表示结果仍不能确定。 |

<!-- write-step: 22 bytes=16214:16324 -->
````go
type DepartureAuthority interface {
	RolledBack(context.Context, domain.DepartureObservation) (bool, error)
}
````

## 第 23 步：Departures

插入位置：第 22 步代码之后，紧接追加；完成稿约第 416 行。

本步声明：`type Departures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Departures</code> | 使用 <code>repository.Departures</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。 |
| <code>authority</code> | <code>DepartureAuthority</code> | 使用 <code>DepartureAuthority</code> 类型；其定义处的职责：修复组织成员来源门前，先取得Account的原退出结果，不能仅靠超时重开门。 |

<!-- write-step: 23 bytes=16324:16438 -->
````go
type Departures struct {
	db         *sql.DB
	repository *repository.Departures
	authority  DepartureAuthority
}

````

## 第 24 步：NewDepartures

插入位置：第 23 步代码之后，紧接追加；完成稿约第 422 行。

本步声明：`func NewDepartures(db *sql.DB, repository *repository.Departures, authority DepartureAuthority) *Departures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repository</code> | <code>*repository.Departures</code> | 使用 <code>repository.Departures</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>repository: repository</code> |
| <code>authority</code> | <code>DepartureAuthority</code> | 使用 <code>DepartureAuthority</code> 类型；其定义处的职责：修复组织成员来源门前，先取得Account的原退出结果，不能仅靠超时重开门。<br>在此声明中的实际使用：<code>authority: authority</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Departures</code> | 实际返回表达式：<code>&amp;Departures{db: db, repository: repository, authority: authority}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Departures{db: db, repository: repository, authority: authority}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=16438:16624 -->
````go
func NewDepartures(db *sql.DB, repository *repository.Departures, authority DepartureAuthority) *Departures {
	return &Departures{db: db, repository: repository, authority: authority}
}
````

## 第 25 步：Departures.Eligibility

插入位置：第 24 步代码之后，紧接追加；完成稿约第 425 行。

本步声明：`func (s *Departures) Eligibility(ctx context.Context, binding domain.DepartureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.EnsureDepartureGate(ctx, tx, binding)</code>；<code>repository.LockedDepartureGate(ctx, tx, binding.Scope())</code>；<code>repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>repository.EnsureDepartureGate(ctx, tx, binding)</code>；<code>binding.Scope</code>；<code>binding.Epoch</code>；<code>binding.ID</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>admin</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DEPARTURE_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DEPARTURE_PREPARE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; binding.Epoch &#124;&#124; (gate.Epoch == binding.Epoch &amp;&amp; gate.State == &quot;BLOCKED&quot; &amp;&amp; gate.Operation != binding.ID)</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;DEPARTURE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!admin</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return false, domain.Violation(domain.TemporarilyUnavailable, &quot;DEPARTURE_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return admin, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=16624:17738 -->
````go
func (s *Departures) Eligibility(ctx context.Context, binding domain.DepartureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// 任命/建库也先锁相同成员来源；取得门闩之后才能判断已提交的管理员事实。
	if err = repository.EnsureDepartureGate(ctx, tx, binding); err != nil {
		return false, err
	}
	gate, err := repository.LockedDepartureGate(ctx, tx, binding.Scope())
	if err != nil {
		return false, err
	}
	if gate.Epoch > binding.Epoch || (gate.Epoch == binding.Epoch && gate.State == "BLOCKED" && gate.Operation != binding.ID) {
		return false, domain.Violation(domain.StateConflict, "DEPARTURE_SOURCE_CHANGED")
	}
	admin, err := repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)
	if err != nil {
		return false, err
	}
	if !admin {
		err = repository.BlockMemberDeparture(ctx, tx, binding)
	}
	if err != nil {
		return false, err
	}
	if tx.Commit() != nil {
		return false, domain.Violation(domain.TemporarilyUnavailable, "DEPARTURE_PREPARE_UNKNOWN")
	}
	return admin, nil
}
````

## 第 26 步：Departures.Close

插入位置：第 25 步代码之后，紧接追加；完成稿约第 457 行。

本步声明：`func (s *Departures) Close(ctx context.Context, binding domain.DepartureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockedDepartureGate(ctx, tx, binding.Scope())</code>；<code>repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)</code>；<code>repository.QueueDepartureNotifications(ctx, tx, binding)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>binding.Scope</code>；<code>binding.Epoch</code>；<code>binding.ID</code>；<code>binding.Org</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DEPARTURE_PREPARE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ADMIN_RETIREMENT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DEPARTURE_CLOSE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; gate.Epoch != binding.Epoch &#124;&#124; gate.State != &quot;BLOCKED&quot; &#124;&#124; gate.Operation != binding.ID</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DEPARTURE_PREPARE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; admin</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ADMIN_RETIREMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;DEPARTURE_CLOSE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=17738:18832 -->
````go
func (s *Departures) Close(ctx context.Context, binding domain.DepartureBinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gate, err := repository.LockedDepartureGate(ctx, tx, binding.Scope())
	if err != nil || gate.Epoch != binding.Epoch || gate.State != "BLOCKED" || gate.Operation != binding.ID {
		return domain.Violation(domain.StateConflict, "DEPARTURE_PREPARE_REQUIRED")
	}
	admin, err := repository.HasKnowledgeBaseAdministration(ctx, tx, binding.Org, binding.User)
	if err != nil || admin {
		return domain.Violation(domain.StateConflict, "ADMIN_RETIREMENT_REQUIRED")
	}
	// 只关闭指定组织的旧成员事实；来源冻结阻止迟到授权，个人空间协作继续独立存在。
	if err = repository.QueueDepartureNotifications(ctx, tx, binding); err != nil {
		return err
	}
	if err = repository.CloseOrganizationMemberFacts(ctx, tx, binding.Org, binding.User); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "DEPARTURE_CLOSE_UNKNOWN")
	}
	return nil
}
````

## 第 27 步：Departures.Recover

插入位置：第 26 步代码之后，紧接追加；完成稿约第 483 行。

本步声明：`func (s *Departures) Recover(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 1500*time.Millisecond)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 1500*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !rolledBack</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=18832:19366 -->
````go
func (s *Departures) Recover(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 1500*time.Millisecond)
	defer cancel()
	observed, err := s.repository.Pending(ctx)
	if err != nil {
		return
	}
	rolledBack, err := s.authority.RolledBack(ctx, observed)
	if err != nil || !rolledBack {
		return
	}
	// 原编号/代次/冻结状态仍一致才恢复；不以租约到期或普通SELECT缺行推断回滚。
	s.repository.Reopen(ctx, observed)
}

// governance_authority.go：保留原实现，按业务合并文件。

````

## 第 28 步：Identity.AcquireGovernance

插入位置：第 27 步代码之后，紧接追加；完成稿约第 500 行。

普通登录仅建立actor；治理必须另外取得Account专用purpose的真实平台角色准入。

本步声明：`func (s *Identity) AcquireGovernance(ctx context.Context, actor Actor, operation string, document uint64) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code>；<code>actor.ID</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentId: document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 由 <code>s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, 		&amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation, 		&amp;pb.Target{DocumentId: document, Purpose: &quot;RETAINED_GOVERNANCE&quot;})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=19366:19898 -->
````go
// 普通登录仅建立actor；治理必须另外取得Account专用purpose的真实平台角色准入。
func (s *Identity) AcquireGovernance(ctx context.Context, actor Actor, operation string, document uint64) (*pb.IdentityAdmission, error) {
	return s.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT,
		&pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, operation,
		&pb.Target{DocumentId: document, Purpose: "RETAINED_GOVERNANCE"})
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 29 步：Actor

插入位置：第 28 步代码之后，紧接追加；完成稿约第 509 行。

Actor保存已核验身份与跨服务证明；请求对象不进入用例。

本步声明：`type Actor struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 当前已认证账号的稳定编号。 |
| <code>Token</code> | <code>&#91;&#93;byte</code> | 当前会话凭据字节，只传给受控身份核验路径，不记录到教程或日志。 |
| <code>Proof</code> | <code>*pb.IdentityAdmission</code> | Account身份准入证明，后续还要核验具体文档权限。 |
| <code>Sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 当前账号/会话相关权限来源及代次。 |
| <code>SessionExpiresUnixMs</code> | <code>int64</code> | 原会话到期时间（Unix毫秒），下游准入不能延长它。 |

<!-- write-step: 29 bytes=19898:20171 -->
````go
// Actor保存已核验身份与跨服务证明；请求对象不进入用例。
type Actor struct {
	ID                   uint64
	Token                []byte
	Proof                *pb.IdentityAdmission
	Sources              []*pb.SourceEpoch
	SessionExpiresUnixMs int64
}
````

## 第 30 步：SessionVerifier

插入位置：第 29 步代码之后，紧接追加；完成稿约第 517 行。

本步声明：`type SessionVerifier interface {`。

**这一段为什么这样写：** 将会话凭据核验留在账号服务，Doc只消费核验结果并形成文档准入。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>VerifySession</code> | <code>(context.Context, domain.SessionVerification) (domain.VerifiedSession, error)</code> | ctx加会话校验请求，返回Account核验的账号/会话事实或错误；不能信任客户端自报身份。 |

<!-- write-step: 30 bytes=20171:20298 -->
````go
type SessionVerifier interface {
	VerifySession(context.Context, domain.SessionVerification) (domain.VerifiedSession, error)
}
````

## 第 31 步：AdmissionRenewer

插入位置：第 30 步代码之后，紧接追加；完成稿约第 520 行。

本步声明：`type AdmissionRenewer interface {`。

**这一段为什么这样写：** 恢复原业务时重新核验身份事实，避免长期持久任务沿用已经失效的旧准入。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>RenewAdmission</code> | <code>(context.Context, *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)</code> | ctx加原身份再验证请求，返回重新核验的准入证明；不延长原会话期限，也不变更原业务目标。 |

<!-- write-step: 31 bytes=20298:20424 -->
````go
type AdmissionRenewer interface {
	RenewAdmission(context.Context, *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)
}
````

## 第 32 步：Identity

插入位置：第 31 步代码之后，紧接追加；完成稿约第 523 行。

本步声明：`type Identity struct {`。

**这一段为什么这样写：** 封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>pb.IdentityInternalClient</code> | Account内部身份RPC客户端，负责取得准入/再验证事实。 |
| <code>verifier</code> | <code>SessionVerifier</code> | 使用 <code>SessionVerifier</code> 类型；其定义处的职责：将会话凭据核验留在账号服务，Doc只消费核验结果并形成文档准入。 |
| <code>renewer</code> | <code>AdmissionRenewer</code> | 使用 <code>AdmissionRenewer</code> 类型；其定义处的职责：恢复原业务时重新核验身份事实，避免长期持久任务沿用已经失效的旧准入。 |

<!-- write-step: 32 bytes=20424:20539 -->
````go
type Identity struct {
	identity pb.IdentityInternalClient
	verifier SessionVerifier
	renewer  AdmissionRenewer
}

````

## 第 33 步：NewIdentity

插入位置：第 32 步代码之后，紧接追加；完成稿约第 529 行。

本步声明：`func NewIdentity(peer pb.IdentityInternalClient, verifier SessionVerifier, renewer AdmissionRenewer) *Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peer</code> | <code>pb.IdentityInternalClient</code> | 在此声明中的实际使用：<code>identity: peer</code> |
| <code>verifier</code> | <code>SessionVerifier</code> | 使用 <code>SessionVerifier</code> 类型；其定义处的职责：将会话凭据核验留在账号服务，Doc只消费核验结果并形成文档准入。<br>在此声明中的实际使用：<code>verifier: verifier</code> |
| <code>renewer</code> | <code>AdmissionRenewer</code> | 使用 <code>AdmissionRenewer</code> 类型；其定义处的职责：恢复原业务时重新核验身份事实，避免长期持久任务沿用已经失效的旧准入。<br>在此声明中的实际使用：<code>renewer: renewer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>实际返回表达式：<code>&amp;Identity{identity: peer, verifier: verifier, renewer: renewer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Identity{identity: peer, verifier: verifier, renewer: renewer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=20539:20726 -->
````go
func NewIdentity(peer pb.IdentityInternalClient, verifier SessionVerifier, renewer AdmissionRenewer) *Identity {
	return &Identity{identity: peer, verifier: verifier, renewer: renewer}
}
````

## 第 34 步：Identity.Authenticate

插入位置：第 33 步代码之后，紧接追加；完成稿约第 532 行。

本步声明：`func (s *Identity) Authenticate(ctx context.Context, token []byte, request domain.SessionVerification, purpose string, document uint64) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.verifier.VerifySession(ctx, request)</code>；<code>s.Acquire(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>token</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>Token: token</code> |
| <code>request</code> | <code>domain.SessionVerification</code> | 使用 <code>domain.SessionVerification</code> 类型；其定义处的职责：SessionVerification只携带账号权威验证所需元数据，业务层不保存HTTP请求。<br>在此声明中的实际使用：<code>s.verifier.VerifySession(ctx, request)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.SessionRequired, &quot;SESSION_EXPIRED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 检查 <code>session.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return actor, domain.Violation(domain.SessionRequired, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=20726:21410 -->
````go
func (s *Identity) Authenticate(ctx context.Context, token []byte, request domain.SessionVerification, purpose string, document uint64) (Actor, error) {
	actor := Actor{Token: token}
	session, err := s.verifier.VerifySession(ctx, request)
	if err != nil {
		return actor, err
	}
	actor.ID = session.ActorID
	if session.ExpiresUnixMs <= time.Now().UnixMilli() {
		return actor, domain.Violation(domain.SessionRequired, "SESSION_EXPIRED")
	}
	actor.SessionExpiresUnixMs = session.ExpiresUnixMs
	actor.Proof, err = s.Acquire(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, purpose, document, 0)
	return actor, err
}

````

## 第 35 步：Identity.Acquire

插入位置：第 34 步代码之后，紧接追加；完成稿约第 547 行。

本步声明：`func (s *Identity) Acquire(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document, unit uint64) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> |
| <code>target</code> | <code>*pb.IdentityTarget</code> | 在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>Purpose: purpose</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentId: document</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>UnitId: unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 由 <code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &amp;pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=21410:21751 -->
````go
func (s *Identity) Acquire(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document, unit uint64) (*pb.IdentityAdmission, error) {
	return s.AcquireBound(ctx, actor, capability, target, platform.NewTraceID(), &pb.Target{DocumentId: document, UnitId: unit, Purpose: purpose})
}

````

## 第 36 步：Identity.BindProofs

插入位置：第 35 步代码之后，紧接追加；完成稿约第 551 行。

后台操作保存的证明必须绑定业务原号和完整工作区；随机请求号只适用于普通短期入口。
BindProofs 把已鉴权证明绑定到本次操作，来源集合必须仍与原证明一致。
空集合的业务含义由调用方决定，不能在公共机制里改变各入口的资格检查顺序。

本步声明：`func (s *Identity) BindProofs(ctx context.Context, actor Actor, originals []*pb.IdentityAdmission, operation string, target *pb.Target) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)</code> |
| <code>originals</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>len(originals)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)</code> |
| <code>target</code> | <code>*pb.Target</code> | 在此声明中的实际使用：<code>s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>bound</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>original == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(&amp;pb.OperationContext{Sources: original.Sources}, &amp;pb.OperationContext{Sources: proof.Sources})</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return bound, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=21751:22859 -->
````go
// 后台操作保存的证明必须绑定业务原号和完整工作区；随机请求号只适用于普通短期入口。
// BindProofs 把已鉴权证明绑定到本次操作，来源集合必须仍与原证明一致。
// 空集合的业务含义由调用方决定，不能在公共机制里改变各入口的资格检查顺序。
func (s *Identity) BindProofs(ctx context.Context, actor Actor, originals []*pb.IdentityAdmission, operation string, target *pb.Target) ([]*pb.IdentityAdmission, error) {
	bound := make([]*pb.IdentityAdmission, 0, len(originals))
	for _, original := range originals {
		if original == nil {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		proof, err := s.AcquireBound(ctx, actor, original.Capability, original.IdentityTarget, operation, target)
		if err != nil {
			return nil, err
		}
		if !proto.Equal(&pb.OperationContext{Sources: original.Sources}, &pb.OperationContext{Sources: proof.Sources}) {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		bound = append(bound, proof)
	}
	return bound, nil
}

````

## 第 37 步：Identity.AcquireBound

插入位置：第 36 步代码之后，紧接追加；完成稿约第 572 行。

本步声明：`func (s *Identity) AcquireBound(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, operation string, businessTarget *pb.Target) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>s.identity.AcquireIdentityAdmission(ctx, request)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Token</code>；<code>actor.ID</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>RequiredIdentityCapability: capability</code>；<code>proof.Capability != capability</code> |
| <code>target</code> | <code>*pb.IdentityTarget</code> | 在此声明中的实际使用：<code>IdentityTarget: target</code>；<code>proto.Equal(proof.IdentityTarget, target)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationId: operation</code> |
| <code>businessTarget</code> | <code>*pb.Target</code> | 在此声明中的实际使用：<code>businessTarget.Purpose</code>；<code>Target: businessTarget</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>；<code>errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;IDENTITY_PROOF_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。
- 检查 <code>status.Code(err) == codes.PermissionDenied</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil, errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>proof == nil &#124;&#124; proof.ActorId != actor.ID &#124;&#124; proof.OperationId != request.OperationId &#124;&#124; proof.Purpose != request.Purpose &#124;&#124; proof.Capability != capability &#124;&#124; !proto.Equal(proof.Ta …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=22859:24016 -->
````go
func (s *Identity) AcquireBound(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, operation string, businessTarget *pb.Target) (*pb.IdentityAdmission, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	request := &pb.IdentityRequest{OperationId: operation, SessionToken: actor.Token, Purpose: businessTarget.Purpose, Target: businessTarget, RequiredIdentityCapability: capability, IdentityTarget: target}
	proof, err := s.identity.AcquireIdentityAdmission(ctx, request)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
		}
		return nil, errors.New("IDENTITY_UNAVAILABLE")
	}
	if proof == nil || proof.ActorId != actor.ID || proof.OperationId != request.OperationId || proof.Purpose != request.Purpose || proof.Capability != capability || !proto.Equal(proof.Target, request.Target) || !proto.Equal(proof.IdentityTarget, target) || proof.ExpiresUnixMs <= time.Now().UnixMilli() || len(proof.SessionRef) != 32 {
		return nil, errors.New("IDENTITY_PROOF_INVALID")
	}
	return proof, nil
}
````

## 第 38 步：Identity.Revalidate

插入位置：第 37 步代码之后，紧接追加；完成稿约第 588 行。

本步声明：`func (s *Identity) Revalidate(ctx context.Context, proof *pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>s.identity.RevalidateIdentityAdmission(ctx, &amp;pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})</code> |
| <code>proof</code> | <code>*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>proof.AdmissionId</code>；<code>proof.OperationId</code>；<code>proof.Purpose</code>；<code>proof.Target</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>；<code>errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;IDENTITY_PROOF_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。
- 检查 <code>status.Code(err) == codes.PermissionDenied</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!proto.Equal(proof, current)</code>。条件成立时执行 <code>return errors.New(&quot;IDENTITY_PROOF_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=24016:24701 -->
````go
func (s *Identity) Revalidate(ctx context.Context, proof *pb.IdentityAdmission) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	current, err := s.identity.RevalidateIdentityAdmission(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
		}
		return errors.New("IDENTITY_UNAVAILABLE")
	}
	if !proto.Equal(proof, current) {
		return errors.New("IDENTITY_PROOF_CHANGED")
	}
	return nil
}
````

## 第 39 步：MergeSources

插入位置：第 38 步代码之后，紧接追加；完成稿约第 603 行。

本步声明：`func MergeSources(proofs ...*pb.IdentityAdmission) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>proofs</code> | <code>...*pb.IdentityAdmission</code> | 此次请求使用的多份身份准入证明；合并其来源时核对相同来源的代次，不能择一绕过撤权。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;IDENTITY_SOURCES_INVALID&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>p == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_SOURCES_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>epoch != source.Epoch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=24701:25342 -->
````go
func MergeSources(proofs ...*pb.IdentityAdmission) ([]*pb.SourceEpoch, error) {
	current := map[string]uint64{}
	var result []*pb.SourceEpoch
	for _, p := range proofs {
		if p == nil {
			continue
		}
		for _, source := range p.Sources {
			if source == nil || source.Epoch == 0 {
				return nil, errors.New("IDENTITY_SOURCES_INVALID")
			}
			if epoch, exists := current[source.ScopeKey]; exists {
				if epoch != source.Epoch {
					return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
				}
				continue
			}
			current[source.ScopeKey] = source.Epoch
			result = append(result, source)
		}
	}
	return result, nil
}
````

## 第 40 步：Identity.Target

插入位置：第 39 步代码之后，紧接追加；完成稿约第 626 行。

本步声明：`func (s *Identity) Target(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document uint64) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |
| <code>target</code> | <code>*pb.IdentityTarget</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 由 <code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.Acquire(ctx, actor, capability, target, purpose, document, 0)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=25342:25604 -->
````go
func (s *Identity) Target(ctx context.Context, actor Actor, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string, document uint64) (*pb.IdentityAdmission, error) {
	return s.Acquire(ctx, actor, capability, target, purpose, document, 0)
}

````

## 第 41 步：Identity.Resume

插入位置：第 40 步代码之后，紧接追加；完成稿约第 630 行。

Resume只供明确列出的已登记操作续接；新短期证明必须保留原actor/会话/目标/来源，不能换用后来取得的权限。

本步声明：`func (s *Identity) Resume(ctx context.Context, original *pb.IdentityAdmission) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Revalidate(ctx, original)</code>；<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>s.renewer.RenewAdmission(ctx, &amp;pb.IdentityRevalidation{AdmissionId: original.AdmissionId, OperationId: original.OperationId, Purpose: original.Purpose, Target: original.Target, IdentityTarget: original.IdentityTarget})</code> |
| <code>original</code> | <code>*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>original == nil</code>；<code>original.Purpose</code>；<code>original.ExpiresUnixMs</code>；<code>s.Revalidate(ctx, original)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>original</code>；<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;IDENTITY_RESUME_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code>；<code>errors.New(&quot;IDENTITY_RESUME_PROOF_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>original == nil &#124;&#124; s.renewer == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_RESUME_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.ExpiresUnixMs &gt; time.Now().Add(2*time.Second).UnixMilli()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return original, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>current == nil &#124;&#124; current.AdmissionId == original.AdmissionId &#124;&#124; current.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_RESUME_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(expected, current)</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_RESUME_PROOF_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=25604:27275 -->
````go
// Resume只供明确列出的已登记操作续接；新短期证明必须保留原actor/会话/目标/来源，不能换用后来取得的权限。
func (s *Identity) Resume(ctx context.Context, original *pb.IdentityAdmission) (*pb.IdentityAdmission, error) {
	if original == nil || s.renewer == nil {
		return nil, errors.New("IDENTITY_RESUME_BINDING_INVALID")
	}
	switch original.Purpose {
	case "SUBMISSION", "MERGE", "MAIN_CONTROL", "PUBLICATION", "PULL_MAIN", "STRUCTURE_PREVIEW", "DISCARD_PREVIEW", "DISCARD_DRAFT", "REVERSE", "CONFLICT_RESOLUTION":
	default:
		return nil, errors.New("IDENTITY_RESUME_BINDING_INVALID")
	}
	if original.ExpiresUnixMs > time.Now().Add(2*time.Second).UnixMilli() {
		if err := s.Revalidate(ctx, original); err != nil {
			return nil, err
		}
		return original, nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	current, err := s.renewer.RenewAdmission(ctx, &pb.IdentityRevalidation{AdmissionId: original.AdmissionId, OperationId: original.OperationId, Purpose: original.Purpose, Target: original.Target, IdentityTarget: original.IdentityTarget})
	if err != nil {
		return nil, err
	}
	if current == nil || current.AdmissionId == original.AdmissionId || current.ExpiresUnixMs <= time.Now().UnixMilli() {
		return nil, errors.New("IDENTITY_RESUME_PROOF_INVALID")
	}
	expected := proto.Clone(original).(*pb.IdentityAdmission)
	expected.AdmissionId, expected.ExpiresUnixMs = current.AdmissionId, current.ExpiresUnixMs
	if !proto.Equal(expected, current) {
		return nil, errors.New("IDENTITY_RESUME_PROOF_CHANGED")
	}
	return current, nil
}

// responsibility.go：保留原实现，按业务合并文件。

````

## 第 42 步：Management.Transfer

插入位置：第 41 步代码之后，紧接追加；完成稿约第 665 行。

本步声明：`func (s *Management) Transfer(ctx context.Context, actor Actor, id, user uint64, input dto.OwnerChange) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;TRANSFER_RESPONSIBLE&quot;)</code>；<code>repository.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, &quot;TRANSFER_RESPONSIBLE&quot;, id)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, access, err := s.managed(ctx, actor, id, &quot;TRANSFER_RESPONSIBLE&quot;)</code>；<code>s.managed(ctx, actor, id, &quot;TRANSFER_RESPONSIBLE&quot;)</code>；<code>return actor, nil, err</code>；<code>return actor, nil, domain.Violation(domain.StateConflict, &quot;PERSONAL_TRANSFER_REQUIRES_ACCEPTANCE&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;TRANSFER_RESPONSIBLE&quot;)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, &quot;TRANSFER_RESPONSIBLE&quot;, id)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code>；<code>repository.TransferDocumentResponsibility(ctx, tx, id, user)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>UserId: user</code>；<code>repository.TransferDocumentResponsibility(ctx, tx, id, user)</code>；<code>repository.NotifyResponsibilityTransfer(ctx, tx, user, input.OperationId, target)</code> |
| <code>input</code> | <code>dto.OwnerChange</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.TargetUserId</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PERSONAL_TRANSFER_REQUIRES_ACCEPTANCE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind != &quot;ORGANIZATION&quot;</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.StateConflict, &quot;PERSONAL_TRANSFER_REQUIRES_ACCEPTANCE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.ControlRevision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.ResponsibleUserId != nil &amp;&amp; *document.ResponsibleUserId == input.TargetUserId</code>。条件成立时执行 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 42 bytes=27275:29323 -->
````go
func (s *Management) Transfer(ctx context.Context, actor Actor, id, user uint64, input dto.OwnerChange) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "TRANSFER_RESPONSIBLE")
	if err != nil {
		return actor, nil, err
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return actor, nil, err
	}
	if space.Kind != "ORGANIZATION" {
		return actor, nil, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_REQUIRES_ACCEPTANCE")
	}
	organization, _ := domain.CanonicalID(*space.OrganizationId)
	proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, "TRANSFER_RESPONSIBLE", id)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	sources, err := MergeSources(append(access.Proofs, proof)...)
	if err != nil {
		return actor, nil, err
	}
	baseSources, _ := MergeSources(access.Proofs...)
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if document.ResponsibleUserId != nil && *document.ResponsibleUserId == input.TargetUserId {
			return OperationResult(input.OperationId, "SUCCEEDED", id), nil
		}
		if err = repository.TransferDocumentResponsibility(ctx, tx, id, user); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "RESPONSIBLE_TRANSFER"}); err != nil {
			return nil, err
		}
````

## 第 43 步：Management.Transfer（接着实现）

插入位置：第 42 步代码之后，紧接追加；完成稿约第 710 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=29323:29688 -->
````go
		target, _ := json.Marshal(map[string]string{"document_id": document.Id})
		if err = repository.NotifyResponsibilityTransfer(ctx, tx, user, input.OperationId, target); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// source_gate.go：保留原实现，按业务合并文件。

````

## 第 44 步：LockSources

插入位置：第 43 步代码之后，紧接追加；完成稿约第 721 行。

来源只来自当前身份证明或已锁定ACL；同代BLOCKED拒绝，只有更高已核验代次才能重开。

本步声明：`func LockSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>append(&#91;&#93;*pb.SourceEpoch(nil), sources...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return list&#91;i&#93;.ScopeKey &lt; list&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; seen&#91;source.ScopeKey&#93;</code>。条件成立时执行 <code>return errors.New(&quot;SOURCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>platform.ValidDocumentScope(source.ScopeKey)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; source.Epoch &#124;&#124; (gate.Epoch == source.Epoch &amp;&amp; gate.State == &quot;BLOCKED&quot;)</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &lt; source.Epoch</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=29688:31065 -->
````go
// 来源只来自当前身份证明或已锁定ACL；同代BLOCKED拒绝，只有更高已核验代次才能重开。
func LockSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error {
	list := append([]*pb.SourceEpoch(nil), sources...)
	sort.Slice(list, func(i, j int) bool { return list[i].ScopeKey < list[j].ScopeKey })
	seen := map[string]bool{}
	for _, source := range list {
		if source == nil || source.Epoch == 0 || seen[source.ScopeKey] {
			return errors.New("SOURCE_BINDING_INVALID")
		}
		seen[source.ScopeKey] = true
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		// 取得gate锁后再查询过渡事实，避免拿旧快照等锁后越过仍未完成的关闭。
		if platform.ValidDocumentScope(source.ScopeKey) {
			pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
			if err != nil {
				return err
			}
			if pending {
				return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
			}
		}
		if gate.Epoch > source.Epoch || (gate.Epoch == source.Epoch && gate.State == "BLOCKED") {
			return domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
		}
		if gate.Epoch < source.Epoch {
			if err := repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return err
			}
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/authorization_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
