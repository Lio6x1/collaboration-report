# 分步编写 `services/document/internal/logic/document/workspace_directory_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/246-workspace-directory-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按当前权限分页展示空间目录、知识库、成员及文档协作候选。

目录可见可能来自继承或单篇授权，必须记录真实来源；候选列表不能赋予管理或收件资格。 名称和成员搜索要绑定原查询条件；页内容只是显示结果，不能作为本人资格、完整成员集合或授权候选的替代。

写之前先按这条执行路线理解各段的连接：

1. KnowledgeBaseVisibilitySources收集显式/派生来源
2. Documents逐项过滤
3. DocumentCollaborators要求管理
4. DocumentMentionCandidates要求COMMENT并向账号目录解析姓名。
5. directorySearch对可选首项trim/lower，按Unicode码点限制100字符；空值表示不筛选，原不传参数的调用仍有效。
6. KnowledgeBases将q纳入actor/space签名游标，SQL先按名称筛选候选，再逐库收集显式或派生当前来源，最终事务重验KB ACL及可见性。
7. KnowledgeBaseMembers先Manage，再按actor/kb/acl_revision/q核游标；从Account当前成员目录解析姓名，过滤昵称子串或去前导零后的完整账号，逐项重验目标成员来源，最后按可见limit组页。

本文件需要守住的边界：

- 提及候选不保证通知可送达，投递时必须重验原正文读取权限
- 别提前扫描全部目录再分页。
- KB成员不是跨私库SQL搜索：原候选扫描仍有limit*10+1预算；预算耗尽允许空页加next_cursor，不能提前宣称目录结束。
- 改q、库或ACL版本不能复用原游标；只记页入口cursor，上一页HTTP也必须走当前授权和最终ReadBody事务。
- Account目录解析当前有100页预算，依赖不可达或预算耗尽应明确失败，不能返回未核验姓名或假空目录。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/workspace_directory_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/workspace_directory_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/workspace_directory_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/workspace_directory_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:478 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// directory_access.go：保留原实现，按业务合并文件。

````

## 第 2 步：Directory

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type Directory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>knowledgeBases</code> | <code>*KnowledgeBases</code> | 使用 <code>KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>directory</code> | <code>CollaboratorDirectoryReader</code> | 使用 <code>CollaboratorDirectoryReader</code> 类型；其定义处的职责：协作者列表通过账号服务解析名称/身份，保留稳定账号编号做内部关联。 |

<!-- write-step: 2 bytes=478:683 -->
````go
type Directory struct {
	db             *sql.DB
	identity       *Identity
	access         *Access
	knowledgeBases *KnowledgeBases
	pagination     *Pagination
	directory      CollaboratorDirectoryReader
}

````

## 第 3 步：CollaboratorDirectoryReader

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type CollaboratorDirectoryReader interface {`。

**这一段为什么这样写：** 协作者列表通过账号服务解析名称/身份，保留稳定账号编号做内部关联。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DirectoryReader</code> | <code>DirectoryReader</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>Collaborator</code> | <code>(context.Context, *pb.IdentityRevalidation) (dto.DocumentCollaborator, error)</code> | ctx与固定身份再验证请求输入，取得该协作者展示信息和可核验状态；昵称不能代替账号授权。 |

<!-- write-step: 3 bytes=683:839 -->
````go
type CollaboratorDirectoryReader interface {
	DirectoryReader
	Collaborator(context.Context, *pb.IdentityRevalidation) (dto.DocumentCollaborator, error)
}

````

## 第 4 步：NewDirectory

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func NewDirectory(db *sql.DB, identity *Identity, access *Access, knowledgeBases *KnowledgeBases, pagination *Pagination, directory CollaboratorDirectoryReader) *Directory`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>knowledgeBases</code> | <code>*KnowledgeBases</code> | 使用 <code>KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>knowledgeBases: knowledgeBases</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>directory</code> | <code>CollaboratorDirectoryReader</code> | 使用 <code>CollaboratorDirectoryReader</code> 类型；其定义处的职责：协作者列表通过账号服务解析名称/身份，保留稳定账号编号做内部关联。<br>在此声明中的实际使用：<code>directory: directory</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Directory</code> | 实际返回表达式：<code>&amp;Directory{db: db, identity: identity, access: access, knowledgeBases: knowledgeBases, pagination: pagination, directory: directory}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Directory{db: db, identity: identity, access: access, knowledgeBases: knowledgeBases, pagination: pagination, directory: directory}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=839:1156 -->
````go
func NewDirectory(db *sql.DB, identity *Identity, access *Access, knowledgeBases *KnowledgeBases, pagination *Pagination, directory CollaboratorDirectoryReader) *Directory {
	return &Directory{db: db, identity: identity, access: access, knowledgeBases: knowledgeBases, pagination: pagination, directory: directory}
}
````

## 第 5 步：Directory.Identity

插入位置：第 4 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func (s *Directory) Identity(ctx context.Context, actor Actor, spaceID uint64, purpose string) (dto.Space, *pb.IdentityAdmission, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code> |
| <code>spaceID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 实际返回表达式：<code>space</code> |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>proof</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return space, proof, true, nil</code>。当前函数在这里结束。
- 检查 <code>!errors.As(err, &amp;violation) &#124;&#124; violation.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return space, nil, false, err</code>。当前函数在这里结束。
- 此处通过 <code>return space, proof, false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1156:1779 -->
````go
func (s *Directory) Identity(ctx context.Context, actor Actor, spaceID uint64, purpose string) (dto.Space, *pb.IdentityAdmission, bool, error) {
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)
	if err == nil {
		return space, proof, true, nil
	}
	var violation domain.RuleViolation
	if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
		return space, nil, false, err
	}
	space, proof, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)
	return space, proof, false, err
}
````

## 第 6 步：KnowledgeBaseVisibilitySources

插入位置：第 5 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func KnowledgeBaseVisibilitySources(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, []*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExplicitKBVisibility(ctx, q, kb, actor)</code>；<code>repository.DerivedDirectorySources(ctx, q, kb, actor)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.ExplicitKBVisibility(ctx, q, kb, actor)</code>；<code>repository.DerivedDirectorySources(ctx, q, kb, actor)</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExplicitKBVisibility(ctx, q, kb, actor)</code>；<code>repository.DerivedDirectorySources(ctx, q, kb, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ExplicitKBVisibility(ctx, q, kb, actor)</code>；<code>repository.DerivedDirectorySources(ctx, q, kb, actor)</code> |
| <code>inherited</code> | <code>bool</code> | 本次权限是否来自组织管理资格继承；据此补全来源，不能把库角色与组织角色等同。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>visible</code>；<code>false</code>；<code>len(sources) &gt; 0</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_SOURCE_BUDGET_EXCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>inherited</code>。条件成立时执行 <code>return true, nil, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; visible</code>。条件成立时执行 <code>return visible, nil, err</code>。当前函数在这里结束。
- 检查 <code>len(sources) &gt; 400</code>。条件成立时执行 <code>return false, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_SOURCE_BUDGET_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return len(sources) &gt; 0, sources, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1779:2624 -->
````go
func KnowledgeBaseVisibilitySources(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, []*pb.SourceEpoch, error) {
	if inherited {
		return true, nil, nil
	}
	visible, err := repository.ExplicitKBVisibility(ctx, q, kb, actor)
	if err != nil || visible {
		return visible, nil, err
	}
	// 库名也可能凭文档授权可见；输出必须绑定每条实际Doc ACL，不能仅登记KB来源。
	facts, err := repository.DerivedDirectorySources(ctx, q, kb, actor)
	var sources []*pb.SourceEpoch
	for _, fact := range facts {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(fact.ID), Epoch: fact.Epoch})
	}
	if len(sources) > 400 {
		return false, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_SOURCE_BUDGET_EXCEEDED")
	}
	return len(sources) > 0, sources, err
}
````

## 第 7 步：knowledgeBaseVisible

插入位置：第 6 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func knowledgeBaseVisible(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)</code> |
| <code>inherited</code> | <code>bool</code> | 在此声明中的实际使用：<code>KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>visible</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return visible, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2624:2920 -->
````go
func knowledgeBaseVisible(ctx context.Context, q repository.Queryer, kb, actor uint64, inherited bool) (bool, error) {
	visible, _, err := KnowledgeBaseVisibilitySources(ctx, q, kb, actor, inherited)
	return visible, err
}

// directory_documents.go：保留原实现，按业务合并文件。

````

## 第 8 步：Directory.Documents

插入位置：第 7 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func (s *Directory) Documents(ctx context.Context, actor Actor, spaceID, kbID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, s.db, kbID, false)</code>；<code>s.Identity(ctx, actor, spaceID, &quot;LIST_DOCUMENTS&quot;)</code>；<code>s.identity.Revalidate(ctx, proof)</code>；<code>repository.DocumentCandidates(ctx, s.db, spaceID, kbID, last, limit*10+1)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Identity(ctx, actor, spaceID, &quot;LIST_DOCUMENTS&quot;)</code>；<code>actor.ID</code>；<code>actor.Proof</code>；<code>s.access.Authorize(ctx, actor, id, &quot;LIST_DOCUMENTS&quot;)</code> |
| <code>spaceID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>spaceID, _ = domain.CanonicalID(kb.SpaceId)</code>；<code>s.Identity(ctx, actor, spaceID, &quot;LIST_DOCUMENTS&quot;)</code>；<code>domain.TextID(spaceID)</code>；<code>repository.DocumentCandidates(ctx, s.db, spaceID, kbID, last, limit*10+1)</code> |
| <code>kbID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>kbID != 0</code>；<code>repository.KnowledgeBaseRow(ctx, s.db, kbID, false)</code>；<code>domain.TextID(kbID)</code>；<code>repository.DocumentCandidates(ctx, s.db, spaceID, kbID, last, limit*10+1)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>kbID != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>index &gt;= limit*10</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 8 bytes=2920:4473 -->
````go
func (s *Directory) Documents(ctx context.Context, actor Actor, spaceID, kbID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	if kbID != 0 {
		kb, err := repository.KnowledgeBaseRow(ctx, s.db, kbID, false)
		if err != nil {
			return nil, nil, err
		}
		spaceID, _ = domain.CanonicalID(kb.SpaceId)
	}
	_, proof, _, err := s.Identity(ctx, actor, spaceID, "LIST_DOCUMENTS")
	if err != nil {
		return nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	binding := "documents:" + domain.TextID(actor.ID) + ":" + domain.TextID(spaceID) + ":" + domain.TextID(kbID)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	candidates, err := repository.DocumentCandidates(ctx, s.db, spaceID, kbID, last, limit*10+1)
	if err != nil {
		return nil, nil, err
	}
	page := dto.DocumentPage{Items: []dto.Document{}}
	sources, _ := MergeSources(actor.Proof, proof)
	bySource := map[string]bool{}
	for _, source := range sources {
		bySource[source.ScopeKey] = true
	}
	var accesses []DocumentAccess
	scanned := last
	for index, id := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(id)
		access, err := s.access.Authorize(ctx, actor, id, "LIST_DOCUMENTS")
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound) {
````

## 第 9 步：Directory.Documents（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 120 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(page.Items) == limit</code>。
- 检查 <code>!bySource&#91;source.ScopeKey&#93;</code>。
- 检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4473:5280 -->
````go
				continue
			}
			return nil, nil, err
		}
		if len(page.Items) == limit {
			page.HasMore = true
			scanned = page.Items[limit-1].Id
			break
		}
		page.Items = append(page.Items, access.Document)
		accesses = append(accesses, access)
		for _, source := range access.Sources {
			if !bySource[source.ScopeKey] {
				bySource[source.ScopeKey] = true
				sources = append(sources, source)
			}
		}
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		for _, access := range accesses {
			if _, err := ValidateAccess(ctx, tx, access); err != nil {
				return nil, err
			}
		}
		return page, nil
	}
	return sources, run, nil
}

// directory_knowledge_base.go：保留原实现，按业务合并文件。

````

## 第 10 步：KnowledgeBaseReading

插入位置：第 9 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`type KnowledgeBaseReading struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>KnowledgeBase</code> | <code>dto.KnowledgeBase</code> | 已核验可读的知识库元数据。 |
| <code>Actions</code> | <code>&#91;&#93;string</code> | 服务器按当前权限计算的库操作入口；接口本身仍独立鉴权。 |

<!-- write-step: 10 bytes=5280:5375 -->
````go
type KnowledgeBaseReading struct {
	KnowledgeBase dto.KnowledgeBase
	Actions       []string
}

````

## 第 11 步：Directory.KnowledgeBase

插入位置：第 10 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func (s *Directory) KnowledgeBase(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, s.db, id, false)</code>；<code>s.Identity(ctx, actor, spaceID, &quot;READ_KB&quot;)</code>；<code>KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Identity(ctx, actor, spaceID, &quot;READ_KB&quot;)</code>；<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, s.db, id, false)</code>；<code>KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)</code>；<code>domain.TextID(id)</code>；<code>repository.KnowledgeBaseRow(ctx, tx, id, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ACCESS_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!visible</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ACCESS_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.AclRevision != kb.AclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!visible</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ACCESS_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!managed</code>。

<!-- write-step: 11 bytes=5375:6984 -->
````go
func (s *Directory) KnowledgeBase(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
	if err != nil {
		return nil, nil, err
	}
	spaceID, _ := domain.CanonicalID(kb.SpaceId)
	_, proof, inherited, err := s.Identity(ctx, actor, spaceID, "READ_KB")
	if err != nil {
		return nil, nil, err
	}
	visible, derived, err := KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)
	if err != nil {
		return nil, nil, err
	}
	if !visible {
		return nil, nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return nil, nil, err
	}
	revision, _ := domain.CanonicalID(kb.AclRevision)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: revision})
	sources = append(sources, derived...)
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.AclRevision != kb.AclRevision {
			return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
		}
		visible, err := knowledgeBaseVisible(ctx, tx, id, actor.ID, inherited)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
		}
		actions := []string{"READ"}
		managed := inherited
		if !managed {
````

## 第 12 步：Directory.KnowledgeBase（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 204 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>managed</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>allowed</code>。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; accessSpace.Kind == &quot;PERSONAL&quot;</code>。
- 在本段的匿名函数内：检查 <code>action != &quot;CHANGE_CREATION_GRANTS&quot;</code>。
- 在本段的匿名函数内：检查 <code>inherited</code>。
- 在本段的匿名函数内：此处通过 <code>return KnowledgeBaseReading{KnowledgeBase: current, Actions: actions}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6984:7925 -->
````go
			role, err := repository.KnowledgeBaseRole(ctx, tx, id, actor.ID)
			managed = err == nil && role == "ADMIN"
		}
		if managed {
			actions = append(actions, "CREATE_DOCUMENT", "RENAME", "CHANGE_CREATION_GRANTS", "DELETE", "CHANGE_VISIBILITY", "VIEW_MEMBERS")
		} else {
			allowed, err := repository.KBCreationAllowed(ctx, tx, id, actor.ID)
			if err != nil {
				return nil, err
			}
			if allowed {
				actions = append(actions, "CREATE_DOCUMENT")
			}
		}
		if accessSpace, err := repository.SpaceRow(ctx, tx, spaceID, false); err == nil && accessSpace.Kind == "PERSONAL" {
			filtered := actions[:0]
			for _, action := range actions {
				if action != "CHANGE_CREATION_GRANTS" {
					filtered = append(filtered, action)
				}
			}
			actions = filtered
		} else if inherited {
			actions = append(actions, "CHANGE_ROLES")
		}
		return KnowledgeBaseReading{KnowledgeBase: current, Actions: actions}, nil
	}
	return sources, run, nil
}
````

## 第 13 步：Directory.KnowledgeBases

插入位置：第 12 步代码之后，紧接追加；完成稿约第 233 行。

本步声明：`func (s *Directory) KnowledgeBases(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error)`。

**这一段为什么这样写：** 在空间当前资格下，对全目录名称先筛选再授权组页；它不下载全部对象，也不接纳前端缓存授予权限。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次读取上下文。<br>在此声明中的实际使用：<code>s.Identity(ctx, actor, spaceID, &quot;LIST_KB&quot;)</code>；<code>s.identity.Revalidate(ctx, proof)</code>；<code>repository.KnowledgeBaseCandidates(ctx, s.db, spaceID, last, limit*10+1, query)</code>；<code>KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 已验证会话操作者及原Proof。<br>在此声明中的实际使用：<code>s.Identity(ctx, actor, spaceID, &quot;LIST_KB&quot;)</code>；<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>spaceID</code> | <code>uint64</code> | 目录所属真实空间。<br>在此声明中的实际使用：<code>s.Identity(ctx, actor, spaceID, &quot;LIST_KB&quot;)</code>；<code>domain.TextID(spaceID)</code>；<code>repository.KnowledgeBaseCandidates(ctx, s.db, spaceID, last, limit*10+1, query)</code> |
| <code>limitText</code> | <code>string</code> | URL页大小，由Pagination解析；前端卡片当前传12。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 原opaque签名游标，不是可自由构造的页码或SQL offset。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>search</code> | <code>...string</code> | 可选名称q，仅首项有效；经directorySearch规范化并进入cursor binding。<br>在此声明中的实际使用：<code>directorySearch(search)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际用到的空间、KB与派生Doc来源，交最终输出闸门核验。<br>实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 在原受控读事务里重验当前KB事实后输出KnowledgeBasePage的闭包。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 参数、权限、来源预算或SQL失败。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>index &gt;= limit*10</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!visible</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>len(page.Items) == limit</code>。

<!-- write-step: 13 bytes=7925:9347 -->
````go
func (s *Directory) KnowledgeBases(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	query, err := directorySearch(search)
	if err != nil {
		return nil, nil, err
	}
	_, proof, inherited, err := s.Identity(ctx, actor, spaceID, "LIST_KB")
	if err != nil {
		return nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	binding := "kb-list:" + domain.TextID(actor.ID) + ":" + domain.TextID(spaceID) + ":" + url.QueryEscape(query)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	candidates, err := repository.KnowledgeBaseCandidates(ctx, s.db, spaceID, last, limit*10+1, query)
	if err != nil {
		return nil, nil, err
	}
	page := dto.KnowledgeBasePage{Items: []dto.KnowledgeBase{}}
	sources, _ := MergeSources(actor.Proof, proof)
	scanned := last
	for index, id := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(id)
		visible, derived, err := KnowledgeBaseVisibilitySources(ctx, s.db, id, actor.ID, inherited)
		if err != nil {
			return nil, nil, err
		}
		if !visible {
			continue
		}
		kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		if len(page.Items) == limit {
````

## 第 14 步：Directory.KnowledgeBases（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 278 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; current.AclRevision != kb.AclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!visible</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_ACCESS_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9347:10446 -->
````go
			page.HasMore = true
			scanned = page.Items[limit-1].Id
			break
		}
		page.Items = append(page.Items, kb)
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: epoch})
		sources = append(sources, derived...)
	}
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		for _, kb := range page.Items {
			id, _ := domain.CanonicalID(kb.Id)
			current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
			if err != nil || current.AclRevision != kb.AclRevision {
				return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
			}
			visible, err := knowledgeBaseVisible(ctx, tx, id, actor.ID, inherited)
			if err != nil {
				return nil, err
			}
			if !visible {
				return nil, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_ACCESS_REQUIRED")
			}
		}
		return page, nil
	}
	return sources, run, nil
}

// directory_members.go：保留原实现，按业务合并文件。

````

## 第 15 步：Directory.DocumentCollaborators

插入位置：第 14 步代码之后，紧接追加；完成稿约第 312 行。

DocumentCollaborators只为当前文档管理者提供已有协作者候选；实际授权/转交仍走各自严格的写入口。

本步声明：`func (s *Directory) DocumentCollaborators(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |
| <code>cookie</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> 的第 2 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> 的第 3 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_COLLABORATORS&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=10446:10849 -->
````go
// DocumentCollaborators只为当前文档管理者提供已有协作者候选；实际授权/转交仍走各自严格的写入口。
func (s *Directory) DocumentCollaborators(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, "DOCUMENT_COLLABORATORS")
}

````

## 第 16 步：Directory.DocumentMentionCandidates

插入位置：第 15 步代码之后，紧接追加；完成稿约第 317 行。

DocumentMentionCandidates只向当前有COMMENT资格的用户展示文档内可提及候选。
候选不代表收件资格；通知投递仍独立核验目标对评论原正文的当前读取权限。

本步声明：`func (s *Directory) DocumentMentionCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |
| <code>cookie</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> 的第 2 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> 的第 3 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, &quot;DOCUMENT_MENTION_CANDIDATES&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=10849:11335 -->
````go
// DocumentMentionCandidates只向当前有COMMENT资格的用户展示文档内可提及候选。
// 候选不代表收件资格；通知投递仍独立核验目标对评论原正文的当前读取权限。
func (s *Directory) DocumentMentionCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	return s.documentCandidates(ctx, actor, id, cookie, limitText, cursorText, "DOCUMENT_MENTION_CANDIDATES")
}

````

## 第 17 步：Directory.documentCandidates

插入位置：第 16 步代码之后，紧接追加；完成稿约第 323 行。

本步声明：`func (s *Directory) documentCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText, purpose string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>repository.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations/&quot;+*space.OrganizationId+&quot;/members?limit=100&amp;cursor=&quot;+url.QueryEscape(cursor), &amp;members)</code>；<code>repository.DocumentCollaboratorCandidates(ctx, s.db, id, last, limit*10+1)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>actor.ID</code>；<code>s.identity.Target(ctx, actor, capability, target, purpose, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>domain.TextID(id)</code>；<code>repository.DocumentCollaboratorCandidates(ctx, s.db, id, last, limit*10+1)</code>；<code>s.identity.Target(ctx, actor, capability, target, purpose, id)</code> |
| <code>cookie</code> | <code>string</code> | 在此声明中的实际使用：<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations/&quot;+*space.OrganizationId+&quot;/members?limit=100&amp;cursor=&quot;+url.QueryEscape(cursor), &amp;members)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>s.identity.Target(ctx, actor, capability, target, purpose, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;COMMENT_REQUIRED&quot;)</code>；<code>domain.Violation(domain.InvalidRequest, &quot;DIRECTORY_PURPOSE_INVALID&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Comment</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;COMMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;DIRECTORY_PURPOSE_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=11335:13220 -->
````go
func (s *Directory) documentCandidates(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText, purpose string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return nil, nil, err
	}
	var directoryKind string
	switch purpose {
	case "DOCUMENT_COLLABORATORS":
		if !access.Permissions.Manage {
			return nil, nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		directoryKind = "document-collaborators"
	case "DOCUMENT_MENTION_CANDIDATES":
		if !access.Permissions.Comment {
			return nil, nil, domain.Violation(domain.PermissionDenied, "COMMENT_REQUIRED")
		}
		directoryKind = "document-mention-candidates"
	default:
		return nil, nil, domain.Violation(domain.InvalidRequest, "DIRECTORY_PURPOSE_INVALID")
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return nil, nil, err
	}
	// ACL变化后不能继续翻旧目录；同一数字ID按稳定keyset扫描，不以昵称或数组下标分页。
	// 提及和管理目录分别绑定用途，COMMENT游标不能拿来续读管理候选。
	binding := directoryKind + ":" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	names := map[string]string{}
	var candidates []uint64
	if space.Kind == "ORGANIZATION" {
		cursor := ""
		for index := 0; index < 100; index++ {
			var members dto.MemberPage
			if err = s.directory.Read(ctx, cookie, "/v1/organizations/"+*space.OrganizationId+"/members?limit=100&cursor="+url.QueryEscape(cursor), &members); err != nil {
				return nil, nil, err
			}
			for _, member := range members.Items {
````

## 第 18 步：Directory.documentCandidates（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 368 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>member.State == &quot;ACTIVE&quot;</code>。
- 检查 <code>!members.HasMore</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>members.NextCursor == nil &#124;&#124; index == 99</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_BUDGET_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>user &gt; lastID</code>。
- 在本段的匿名函数内：此处通过 <code>return candidates&#91;i&#93; &lt; candidates&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(candidates) &gt; limit*10+1</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>index &gt;= limit*10 &#124;&#124; len(page.Items) == limit</code>。

<!-- write-step: 18 bytes=13220:14701 -->
````go
				if member.State == "ACTIVE" {
					names[member.UserId] = member.DisplayName
				}
			}
			if !members.HasMore {
				break
			}
			if members.NextCursor == nil || index == 99 {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_BUDGET_EXCEEDED")
			}
			cursor = *members.NextCursor
		}
		// 当前组织成员都是已加入的协作者候选；不能要求其先有文档grant，
		// 否则仅凭继承权限管理的新文档将永远无法发出首次授权。
		lastID, err := strconv.ParseUint(last, 10, 64)
		if err != nil {
			return nil, nil, domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		}
		for text := range names {
			user, err := domain.CanonicalID(text)
			if err != nil {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			if user > lastID {
				candidates = append(candidates, user)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		if len(candidates) > limit*10+1 {
			candidates = candidates[:limit*10+1]
		}
	} else {
		candidates, err = repository.DocumentCollaboratorCandidates(ctx, s.db, id, last, limit*10+1)
		if err != nil {
			return nil, nil, err
		}
	}
	page := dto.DocumentCollaboratorPage{Items: []dto.DocumentCollaborator{}}
	scanned := last
	for index, user := range candidates {
		if index >= limit*10 || len(page.Items) == limit {
			page.HasMore = true
			break
		}
		scanned = domain.TextID(user)
````

## 第 19 步：Directory.documentCandidates（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 413 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; violation.Kind == domain.PermissionDenied</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return nil, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind == &quot;PERSONAL&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>collaborator.UserId != scanned &#124;&#124; collaborator.DisplayName == &quot;&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>name == &quot;&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=14701:16833 -->
````go
		name := names[scanned]
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}
		capability := pb.IdentityCapability_ACTIVE_ACCOUNT
		if space.Kind == "ORGANIZATION" {
			// 旧授权记录仍在也不能把已离开组织的人继续列为可选目标。
			if _, ok := names[scanned]; !ok {
				continue
			}
			organization, _ := domain.CanonicalID(*space.OrganizationId)
			target = &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}
			capability = pb.IdentityCapability_ORGANIZATION_MEMBER
		}
		proof, err := s.identity.Target(ctx, actor, capability, target, purpose, id)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Kind == domain.PermissionDenied {
				continue
			}
			return nil, nil, err
		}
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
		if space.Kind == "PERSONAL" {
			// 候选已由本库真实关系和本用途的管理/评论资格确定；Account只接收其原身份准入绑定。
			// 目标注销或原证明失效时关闭本次目录，不能退回角色称呼冒充姓名。
			collaborator, err := s.directory.Collaborator(ctx, &pb.IdentityRevalidation{AdmissionId: proof.AdmissionId, OperationId: proof.OperationId, Purpose: proof.Purpose, Target: proof.Target, IdentityTarget: proof.IdentityTarget})
			if err != nil {
				return nil, nil, err
			}
			if collaborator.UserId != scanned || collaborator.DisplayName == "" {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
			}
			name = collaborator.DisplayName
		}
		// 每个返回账号都绑定真实身份/组织成员代次，使注销或退出组织可以关闭在途输出。
		access.Sources, err = MergeSources(&pb.IdentityAdmission{Sources: access.Sources}, proof)
		if err != nil {
			return nil, nil, err
		}
		if name == "" {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		page.Items = append(page.Items, dto.DocumentCollaborator{UserId: scanned, DisplayName: name})
	}
````

## 第 20 步：Directory.documentCandidates（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 458 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=16833:17239 -->
````go
	if page.HasMore {
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 输出登记事务与本用途资格使用相同ACL快照；撤权/转交不能夹在快照与交付之间。
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		return page, nil
	}
	return access.Sources, run, nil
}

````

## 第 21 步：Directory.KnowledgeBaseMembers

插入位置：第 20 步代码之后，紧接追加；完成稿约第 471 行。

本步声明：`func (s *Directory) KnowledgeBaseMembers(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error)`。

**这一段为什么这样写：** 在知识库管理资格下显示当前成员，并在服务端匹配Account昵称/完整账号；显示页与组织候选picker互不替代。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次目录上下文。<br>在此声明中的实际使用：<code>s.knowledgeBases.Manage(ctx, actor, id, &quot;KB_MEMBERS&quot;, false)</code>；<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations/&quot;+*access.Space.OrganizationId+&quot;/members?limit=100&amp;cursor=&quot;+url.QueryEscape(cursor), &amp;members)</code>；<code>s.directory.Read(ctx, cookie, &quot;/v1/account/me&quot;, &amp;me)</code>；<code>repository.KBMemberCandidates(ctx, s.db, id, last, limit*10+1)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 真实当前操作者。<br>在此声明中的实际使用：<code>s.knowledgeBases.Manage(ctx, actor, id, &quot;KB_MEMBERS&quot;, false)</code>；<code>actor.ID</code>；<code>s.knowledgeBases.Target(ctx, actor, access, user, &quot;KB_MEMBERS&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 知识库ID，先通过Manage。<br>在此声明中的实际使用：<code>s.knowledgeBases.Manage(ctx, actor, id, &quot;KB_MEMBERS&quot;, false)</code>；<code>domain.TextID(id)</code>；<code>repository.KBMemberCandidates(ctx, s.db, id, last, limit*10+1)</code>；<code>repository.LockedKBMember(ctx, tx, id, member.UserId)</code> |
| <code>cookie</code> | <code>string</code> | 当前请求Cookie，仅交Account适配器解析当前组织姓名与身份，不返回浏览器。<br>在此声明中的实际使用：<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations/&quot;+*access.Space.OrganizationId+&quot;/members?limit=100&amp;cursor=&quot;+url.QueryEscape(cursor), &amp;members)</code>；<code>s.directory.Read(ctx, cookie, &quot;/v1/account/me&quot;, &amp;me)</code> |
| <code>limitText</code> | <code>string</code> | 可见页大小，当前成员界面传20。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 绑定actor/kb/acl_revision/q的原签名cursor。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>search</code> | <code>...string</code> | 可选昵称/补零账号q，最多100个Unicode字符，仅首项有效。<br>在此声明中的实际使用：<code>directorySearch(search)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 查看者与每个返回目标的真实来源集合。<br>实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 在事务内重验KB ACL及每行ACTIVE/revision后输出MemberPage的闭包。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 当前资格、参数、Account依赖、SQL或扫描预算失败。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_BUDGET_EXCEEDED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>access.Space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>member.State == &quot;ACTIVE&quot;</code>。
- 检查 <code>!members.HasMore</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>members.NextCursor == nil &#124;&#124; index == 99</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;DIRECTORY_BUDGET_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=17239:18816 -->
````go
func (s *Directory) KnowledgeBaseMembers(ctx context.Context, actor Actor, id uint64, cookie, limitText, cursorText string, search ...string) ([]*pb.SourceEpoch, ReadBody, error) {
	query, err := directorySearch(search)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.knowledgeBases.Manage(ctx, actor, id, "KB_MEMBERS", false)
	if err != nil {
		return nil, nil, err
	}
	binding := "kb-members:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.KB.AclRevision + ":" + url.QueryEscape(query)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	names := map[string]string{}
	if access.Space.Kind == "ORGANIZATION" {
		cursor := ""
		for index := 0; index < 100; index++ {
			var members dto.MemberPage
			if err = s.directory.Read(ctx, cookie, "/v1/organizations/"+*access.Space.OrganizationId+"/members?limit=100&cursor="+url.QueryEscape(cursor), &members); err != nil {
				return nil, nil, err
			}
			for _, member := range members.Items {
				if member.State == "ACTIVE" {
					names[member.UserId] = member.DisplayName
				}
			}
			if !members.HasMore {
				break
			}
			if members.NextCursor == nil || index == 99 {
				return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_BUDGET_EXCEEDED")
			}
			cursor = *members.NextCursor
		}
	} else {
		var me dto.User
		if err = s.directory.Read(ctx, cookie, "/v1/account/me", &me); err != nil {
			return nil, nil, err
		}
		names[domain.TextID(actor.ID)] = me.DisplayName
	}
````

## 第 22 步：Directory.KnowledgeBaseMembers（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 516 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>index &gt;= limit*10</code>。
- 检查 <code>!ok &#124;&#124; (query != &quot;&quot; &amp;&amp; !strings.Contains(strings.ToLower(name), query) &amp;&amp; strings.TrimLeft(query, &quot;0&quot;) != member.UserId)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>len(page.Items) == limit</code>。
- 检查 <code>access.Space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; violation.Kind == domain.PermissionDenied</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return nil, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>page.HasMore</code>。

<!-- write-step: 22 bytes=18816:20333 -->
````go
	candidates, err := repository.KBMemberCandidates(ctx, s.db, id, last, limit*10+1)
	if err != nil {
		return nil, nil, err
	}
	page := dto.MemberPage{Items: []dto.Member{}}
	scanned := last
	// 稳定扫描后再筛掉历史成员；预算耗尽仍返回游标，空的可见页不等于目录结束。
	for index, member := range candidates {
		if index >= limit*10 {
			page.HasMore = true
			break
		}
		scanned = member.UserId
		name, ok := names[member.UserId]
		if !ok || (query != "" && !strings.Contains(strings.ToLower(name), query) && strings.TrimLeft(query, "0") != member.UserId) {
			continue
		}
		if len(page.Items) == limit {
			page.HasMore = true
			scanned = page.Items[limit-1].UserId
			break
		}
		if access.Space.Kind == "ORGANIZATION" {
			user, _ := domain.CanonicalID(member.UserId)
			proof, err := s.knowledgeBases.Target(ctx, actor, access, user, "KB_MEMBERS")
			if err != nil {
				var violation domain.RuleViolation
				if errors.As(err, &violation) && violation.Kind == domain.PermissionDenied {
					continue
				}
				return nil, nil, err
			}
			if err = s.identity.Revalidate(ctx, proof); err != nil {
				return nil, nil, err
			}
			// 成员名也依赖该目标的真实身份/成员代次，不能仅用查看者自己的证明守护。
			access.Sources, err = MergeSources(&pb.IdentityAdmission{Sources: access.Sources}, proof)
			if err != nil {
				return nil, nil, err
			}
		}
		member.DisplayName = name
		page.Items = append(page.Items, member)
	}
	if page.HasMore {
````

## 第 23 步：Directory.KnowledgeBaseMembers（接着实现）

插入位置：第 22 步代码之后，紧接追加；完成稿约第 561 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; state != &quot;ACTIVE&quot; &#124;&#124; revision != member.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KB_MEMBER_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=20333:20919 -->
````go
		page.NextCursor = s.pagination.Next(binding, scanned)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateKnowledgeBase(ctx, tx, access, access.KB.AclRevision); err != nil {
			return nil, err
		}
		for _, member := range page.Items {
			revision, state, err := repository.LockedKBMember(ctx, tx, id, member.UserId)
			if err != nil || state != "ACTIVE" || revision != member.Revision {
				return nil, domain.Violation(domain.StateConflict, "KB_MEMBER_IDENTITY_CHANGED")
			}
		}
		return page, nil
	}
	return access.Sources, run, nil
}

````

## 第 24 步：directorySearch

插入位置：第 23 步代码之后，紧接追加；完成稿约第 578 行。

限制搜索长度，字面查询避免百分号被当作SQL通配符。

本步声明：`func directorySearch(search []string) (string, error)`。

**这一段为什么这样写：** 在两个知识库只读目录入口统一规范化搜索文字，限制解析开销并让游标绑定同一规范字符串。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>search</code> | <code>&#91;&#93;string</code> | 可选查询切片，仅取首项；trim后转小写，未传或空串即不筛选。<br>在此声明中的实际使用：<code>len(search)</code>；<code>strings.TrimSpace(search&#91;0&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 最多100个Unicode字符的规范q；%/_保留字面意义。<br>实际返回表达式：<code>&quot;&quot;</code>；<code>query</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 超过长度时DIRECTORY_QUERY_INVALID，不截断后静默检索。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;DIRECTORY_QUERY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(search) &gt; 0</code>。
- 检查 <code>len(&#91;&#93;rune(query)) &gt; 100</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;DIRECTORY_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return query, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=20919:21280 -->
````go
// 限制搜索长度，字面查询避免百分号被当作SQL通配符。
func directorySearch(search []string) (string, error) {
	query := ""
	if len(search) > 0 {
		query = strings.ToLower(strings.TrimSpace(search[0]))
	}
	if len([]rune(query)) > 100 {
		return "", domain.Violation(domain.InvalidRequest, "DIRECTORY_QUERY_INVALID")
	}
	return query, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/workspace_directory_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
