# 分步编写 `services/document/internal/logic/document/workspace_management_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/247-workspace-management-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排文档/知识库创建、管理、章节布局与个人/组织空间物化。

创建最后切ACTIVE保证用户永远看不到缺少初始正文的文档；空间gate补足DDL缺少owner唯一约束的并发保护。

写之前先按这条执行路线理解各段的连接：

1. InitializeDocument按真实ID创建结构样式清单版本支线
2. MaterializeSpace先锁持久gate
3. KnowledgeBases.Role复核管理员保留
4. Management.Delete登记关闭流程。

本文件需要守住的边界：

- 新章仅追加结构，旧单元支线和审核继续保留
- 不能让最后可用管理员被无保护地移除。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/workspace_management_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/workspace_management_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/workspace_management_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/workspace_management_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:546 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"net/url"
)

// chapter_control.go：保留原实现，按业务合并文件。

````

## 第 2 步：mainControlLayout

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type mainControlLayout struct{ Structure, Chapter, Unit uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Structure</code> | <code>uint64</code> | 初始主线布局的结构版本。 |
| <code>Chapter</code> | <code>uint64</code> | 初始章节编号。 |
| <code>Unit</code> | <code>uint64</code> | 初始章节的协作单元编号。 |

<!-- write-step: 2 bytes=546:612 -->
````go
type mainControlLayout struct{ Structure, Chapter, Unit uint64 }

````

## 第 3 步：validateChapterAnchor

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func validateChapterAnchor(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.ChapterCreate) (dto.StructureRevision, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)</code> |
| <code>current</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>current.CurrentStructureId</code>；<code>current.Id</code> |
| <code>input</code> | <code>dto.ChapterCreate</code> | 在此声明中的实际使用：<code>input.ExpectedStructureId</code>；<code>input.AfterChapterId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.StructureRevision</code> | 实际返回表达式：<code>dto.StructureRevision{}</code>；<code>structure</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_ENTRY_LIMIT&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>current.CurrentStructureId != input.ExpectedStructureId</code>。条件成立时执行 <code>return dto.StructureRevision{}, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return structure, err</code>。当前函数在这里结束。
- 检查 <code>len(structure.Entries) &gt;= 10000</code>。条件成立时执行 <code>return structure, domain.Violation(domain.StateConflict, &quot;STRUCTURE_ENTRY_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>input.AfterChapterId != nil</code>。
- 检查 <code>entry.Kind == &quot;CHAPTER&quot; &amp;&amp; entry.ParentNodeId == nil &amp;&amp; entry.ChapterId == *input.AfterChapterId</code>。条件成立时执行 <code>return structure, nil</code>。当前函数在这里结束。
- 此处通过 <code>return structure, domain.Violation(domain.StateConflict, &quot;CHAPTER_ANCHOR_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=612:1504 -->
````go
func validateChapterAnchor(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.ChapterCreate) (dto.StructureRevision, error) {
	if current.CurrentStructureId != input.ExpectedStructureId {
		return dto.StructureRevision{}, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return structure, err
	}
	if len(structure.Entries) >= 10000 {
		return structure, domain.Violation(domain.StateConflict, "STRUCTURE_ENTRY_LIMIT")
	}
	if input.AfterChapterId != nil {
		for _, entry := range structure.Entries {
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && entry.ChapterId == *input.AfterChapterId {
				return structure, nil
			}
		}
		return structure, domain.Violation(domain.StateConflict, "CHAPTER_ANCHOR_CHANGED")
	}
	return structure, nil
}

````

## 第 4 步：prepareControlledLayout

插入位置：第 3 步代码之后，紧接追加；完成稿约第 44 行。

新章没有旧草稿要迁移；旧单元的支线和审核事实继续保留，正式结构/版本/清单只在本事务追加。

本步声明：`func prepareControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>prepareStableControlledLayout(ctx, tx, current, fixed)</code>；<code>validateChapterAnchor(ctx, tx, current, *fixed.Intent.Chapter)</code>；<code>repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)</code>；<code>repository.CreateInitialChapter(ctx, tx, fixed.Document, layout.Structure)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>prepareStableControlledLayout(ctx, tx, current, fixed)</code>；<code>validateChapterAnchor(ctx, tx, current, *fixed.Intent.Chapter)</code>；<code>repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)</code>；<code>repository.CreateInitialChapter(ctx, tx, fixed.Document, layout.Structure)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>current</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>prepareStableControlledLayout(ctx, tx, current, fixed)</code>；<code>validateChapterAnchor(ctx, tx, current, *fixed.Intent.Chapter)</code> |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Intent</code>；<code>prepareStableControlledLayout(ctx, tx, current, fixed)</code>；<code>fixed.Structure</code>；<code>fixed.Document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mainControlLayout</code> | 由 <code>prepareStableControlledLayout(ctx, tx, current, fixed)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>layout</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>prepareStableControlledLayout(ctx, tx, current, fixed)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;CHAPTER_INTENT_UNKNOWN&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Intent.Kind == &quot;STRUCTURE_CHANGE&quot;</code>。条件成立时执行 <code>return prepareStableControlledLayout(ctx, tx, current, fixed)</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind != &quot;CHAPTER_CREATE&quot;</code>。条件成立时执行 <code>return layout, nil</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Chapter == nil</code>。条件成立时执行 <code>return layout, domain.Violation(domain.TemporarilyUnavailable, &quot;CHAPTER_INTENT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return layout, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=1504:3083 -->
````go
// 新章没有旧草稿要迁移；旧单元的支线和审核事实继续保留，正式结构/版本/清单只在本事务追加。
func prepareControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error) {
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" {
		return prepareStableControlledLayout(ctx, tx, current, fixed)
	}
	layout := mainControlLayout{Structure: fixed.Structure}
	if fixed.Intent.Kind != "CHAPTER_CREATE" {
		return layout, nil
	}
	if fixed.Intent.Chapter == nil {
		return layout, domain.Violation(domain.TemporarilyUnavailable, "CHAPTER_INTENT_UNKNOWN")
	}
	old, err := validateChapterAnchor(ctx, tx, current, *fixed.Intent.Chapter)
	if err != nil {
		return layout, err
	}
	if layout.Structure, err = repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation); err != nil {
		return layout, err
	}
	if layout.Chapter, err = repository.CreateInitialChapter(ctx, tx, fixed.Document, layout.Structure); err != nil {
		return layout, err
	}
	if layout.Unit, err = repository.CreateInitialUnit(ctx, tx, fixed.Document, layout.Chapter, layout.Structure); err != nil {
		return layout, err
	}
	entries, err := domain.InsertChapterEntry(old.Entries, *fixed.Intent.Chapter, domain.TextID(layout.Chapter), domain.TextID(layout.Unit))
	if err != nil {
		return layout, err
	}
	err = repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)
	return layout, err
}

// document_creation.go：保留原实现，按业务合并文件。

````

## 第 5 步：Documents

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`type Documents struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>quotaLimit</code> | <code>uint64</code> | 空间容量账户所用字节上限；创建文档时初始化/核验容量。 |

<!-- write-step: 5 bytes=3083:3191 -->
````go
type Documents struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	quotaLimit uint64
}

````

## 第 6 步：NewDocuments

插入位置：第 5 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`func NewDocuments(db *sql.DB, identity *Identity, access *Access, quotaLimit uint64) *Documents`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>quotaLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>quotaLimit: quotaLimit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Documents</code> | 实际返回表达式：<code>&amp;Documents{db: db, identity: identity, access: access, quotaLimit: quotaLimit}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Documents{db: db, identity: identity, access: access, quotaLimit: quotaLimit}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3191:3378 -->
````go
func NewDocuments(db *sql.DB, identity *Identity, access *Access, quotaLimit uint64) *Documents {
	return &Documents{db: db, identity: identity, access: access, quotaLimit: quotaLimit}
}
````

## 第 7 步：Documents.Create

插入位置：第 6 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func (s *Documents) Create(ctx context.Context, actor Actor, spaceID uint64, input dto.DocumentCreate) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>s.identity.Revalidate(ctx, proof)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>s.identity.Revalidate(ctx, admin)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>return actor, nil, err</code>；<code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_DOCUMENT_REQUIRES_KB&quot;)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, &quot;CREATE_DOCUMENT&quot;, 0)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>spaceID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, &quot;CREATE_DOCUMENT&quot;, 0)</code>；<code>repository.SpaceRow(ctx, tx, spaceID, true)</code>；<code>current.Space != spaceID</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.DocumentCreate</code> | 在此声明中的实际使用：<code>input.KnowledgeBaseId</code>；<code>input.SpaceId</code>；<code>input.Title</code>；<code>input.VisibilityPolicy</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_DOCUMENT_REQUIRES_KB&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>input.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>kb == 0</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_DOCUMENT_REQUIRES_KB&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;violation) &#124;&#124; violation.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>admin != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>kb != 0</code>。
- 检查 <code>err != nil &#124;&#124; snapshot.SpaceId != input.SpaceId</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=3378:4981 -->
````go
func (s *Documents) Create(ctx context.Context, actor Actor, spaceID uint64, input dto.DocumentCreate) (Actor, MutationBody, error) {
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, "CREATE_DOCUMENT", 0)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	kb := uint64(0)
	if input.KnowledgeBaseId != nil {
		kb, err = domain.CanonicalID(*input.KnowledgeBaseId)
		if err != nil {
			return actor, nil, err
		}
	}
	var admin *pb.IdentityAdmission
	if space.Kind == "ORGANIZATION" {
		if kb == 0 {
			return actor, nil, domain.Violation(domain.PermissionDenied, "ORGANIZATION_DOCUMENT_REQUIRES_KB")
		}
		_, admin, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, "CREATE_DOCUMENT", 0)
		if err != nil {
			var violation domain.RuleViolation
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return actor, nil, err
			}
			admin = nil
		}
		if admin != nil {
			if err = s.identity.Revalidate(ctx, admin); err != nil {
				return actor, nil, err
			}
		}
	}
	actor.Sources, err = MergeSources(actor.Proof, proof, admin)
	if err != nil {
		return actor, nil, err
	}
	var snapshot dto.KnowledgeBase
	if kb != 0 {
		snapshot, err = repository.KnowledgeBaseRow(ctx, s.db, kb, false)
		if err != nil || snapshot.SpaceId != input.SpaceId {
			return actor, nil, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
		}
		revision, _ := domain.CanonicalID(snapshot.AclRevision)
````

## 第 8 步：Documents.Create（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 134 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kb != 0</code>。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; current.Space != spaceID &#124;&#124; current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;KNOWLEDGE_BASE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>domain.TextID(current.Revision) != snapshot.AclRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KNOWLEDGE_BASE_ACCESS_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>admin == nil &amp;&amp; space.Kind != &quot;PERSONAL&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; role != &quot;ADMIN&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; allowed != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_CREATION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4981:6836 -->
````go
		actor.Sources = append(actor.Sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: revision})
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := repository.SpaceRow(ctx, tx, spaceID, true); err != nil {
			return nil, err
		}
		sources, _ := MergeSources(proof, admin)
		if err := LockSources(ctx, tx, sources); err != nil {
			return nil, err
		}
		if kb != 0 {
			current, err := repository.LockedCreationKnowledgeBase(ctx, tx, kb)
			if err != nil || current.Space != spaceID || current.Lifecycle != "ACTIVE" {
				return nil, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
			}
			if domain.TextID(current.Revision) != snapshot.AclRevision {
				return nil, domain.Violation(domain.StateConflict, "KNOWLEDGE_BASE_ACCESS_CHANGED")
			}
			if admin == nil && space.Kind != "PERSONAL" {
				role, err := repository.KnowledgeBaseRole(ctx, tx, kb, actor.ID)
				if err != nil || role != "ADMIN" {
					allowed, err := repository.KnowledgeBaseCreationState(ctx, tx, kb, actor.ID)
					if err != nil || allowed != "ACTIVE" {
						return nil, domain.Violation(domain.PermissionDenied, "DOCUMENT_CREATION_NOT_ALLOWED")
					}
				}
			}
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: current.Revision}}); err != nil {
				return nil, err
			}
		}
		if _, err := resources.LockQuota(ctx, tx, spaceID, s.quotaLimit); err != nil {
			return nil, err
		}
		id, err := InitializeDocument(ctx, tx, spaceID, kb, actor.ID, input.Title, domain.Visibility(input.VisibilityPolicy), input.OperationId, space.Kind == "ORGANIZATION")
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

// initialization.go：保留原实现，按业务合并文件。

````

## 第 9 步：NewEntityID

插入位置：第 8 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func NewEntityID() (uint64, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>binary.BigEndian.Uint64(data&#91;:&#93;)&amp;0x3fffffffffffffff &#124; 0x2000000000000000</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return binary.BigEndian.Uint64(data&#91;:&#93;)&amp;0x3fffffffffffffff &#124; 0x2000000000000000, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6836:7046 -->
````go
func NewEntityID() (uint64, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data[:])&0x3fffffffffffffff | 0x2000000000000000, nil
}

````

## 第 10 步：InitializeDocument

插入位置：第 9 步代码之后，紧接追加；完成稿约第 187 行。

根对象与初始空主线全部属于调用方事务；最后ACTIVE才使完整清单可见。

本步声明：`func InitializeDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility, operation string, locked bool) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code>；<code>repository.CreateInitialStructure(ctx, tx, id, operation)</code>；<code>repository.CreateInitialStyle(ctx, tx, id, domain.InitialStyle, operation)</code>；<code>repository.CreateInitialManifest(ctx, tx, id, structure, style, operation)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code>；<code>repository.CreateInitialStructure(ctx, tx, id, operation)</code>；<code>repository.CreateInitialStyle(ctx, tx, id, domain.InitialStyle, operation)</code>；<code>repository.CreateInitialManifest(ctx, tx, id, structure, style, operation)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |
| <code>title</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |
| <code>visibility</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.CreateInitialStructure(ctx, tx, id, operation)</code>；<code>repository.CreateInitialStyle(ctx, tx, id, domain.InitialStyle, operation)</code>；<code>repository.CreateInitialManifest(ctx, tx, id, structure, style, operation)</code>；<code>initialUnitVersion(ctx, tx, id, unit, structure, manifest, operation)</code> |
| <code>locked</code> | <code>bool</code> | 在此声明中的实际使用：<code>repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return id, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=7046:8470 -->
````go
// 根对象与初始空主线全部属于调用方事务；最后ACTIVE才使完整清单可见。
func InitializeDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility, operation string, locked bool) (uint64, error) {
	id, err := repository.CreateDocument(ctx, tx, space, kb, actor, title, visibility, locked)
	if err != nil {
		return 0, err
	}
	structure, err := repository.CreateInitialStructure(ctx, tx, id, operation)
	if err != nil {
		return 0, err
	}
	style, err := repository.CreateInitialStyle(ctx, tx, id, domain.InitialStyle, operation)
	if err != nil {
		return 0, err
	}
	manifest, err := repository.CreateInitialManifest(ctx, tx, id, structure, style, operation)
	if err != nil {
		return 0, err
	}
	chapter, err := repository.CreateInitialChapter(ctx, tx, id, structure)
	if err != nil {
		return 0, err
	}
	unit, err := repository.CreateInitialUnit(ctx, tx, id, chapter, structure)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialStructureEntry(ctx, tx, id, structure, chapter, unit); err != nil {
		return 0, err
	}
	version, err := initialUnitVersion(ctx, tx, id, unit, structure, manifest, operation)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialManifestEntry(ctx, tx, id, manifest, unit, version); err != nil {
		return 0, err
	}
	err = repository.ActivateDocument(ctx, tx, id, structure, style, manifest)
	return id, err
}
````

## 第 11 步：initialUnitVersion

插入位置：第 10 步代码之后，紧接追加；完成稿约第 226 行。

本步声明：`func initialUnitVersion(ctx context.Context, tx *sql.Tx, document, unit, structure, manifest uint64, operation string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code>；<code>repository.CreateInitialBranch(ctx, tx, document, unit, version)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code>；<code>repository.ActivateInitialBranch(ctx, tx, branch, lineage)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code>；<code>repository.CreateInitialBranch(ctx, tx, document, unit, version)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code>；<code>repository.ActivateInitialBranch(ctx, tx, branch, lineage)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code>；<code>repository.CreateInitialBranch(ctx, tx, document, unit, version)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code>；<code>repository.BindInitialBranch(ctx, tx, document, unit, branch)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.BuildInitialContent(version, lineage, unit, manifest, operation)</code>；<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code>；<code>repository.CreateInitialBranch(ctx, tx, document, unit, version)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.BuildInitialContent(version, lineage, unit, manifest, operation)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.BuildInitialContent(version, lineage, unit, manifest, operation)</code>；<code>repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content)</code>；<code>repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8470:9514 -->
````go
func initialUnitVersion(ctx context.Context, tx *sql.Tx, document, unit, structure, manifest uint64, operation string) (uint64, error) {
	version, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	lineage, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	content, err := domain.BuildInitialContent(version, lineage, unit, manifest, operation)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialContent(ctx, tx, document, unit, structure, operation, content); err != nil {
		return 0, err
	}
	branch, err := repository.CreateInitialBranch(ctx, tx, document, unit, version)
	if err != nil {
		return 0, err
	}
	if err = repository.InsertInitialLineage(ctx, tx, document, unit, branch, manifest, operation, content); err != nil {
		return 0, err
	}
	if err = repository.ActivateInitialBranch(ctx, tx, branch, lineage); err != nil {
		return 0, err
	}
	err = repository.BindInitialBranch(ctx, tx, document, unit, branch)
	return version, err
}

// knowledge_base.go：保留原实现，按业务合并文件。

````

## 第 12 步：KnowledgeBases.Create

插入位置：第 11 步代码之后，紧接追加；完成稿约第 258 行。

本步声明：`func (s *KnowledgeBases) Create(ctx context.Context, actor Actor, space uint64, input dto.KBCreate) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, &quot;CREATE_KNOWLEDGE_BASE&quot;, 0)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, &quot;CREATE_KNOWLEDGE_BASE&quot;, 0)</code>；<code>return actor, nil, err</code>；<code>actor.Sources</code>；<code>actor.Proof</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, &quot;CREATE_KNOWLEDGE_BASE&quot;, 0)</code>；<code>repository.SpaceRow(ctx, tx, space, true)</code>；<code>repository.CreateKnowledgeBase(ctx, tx, space, input.Name, domain.Visibility(input.VisibilityPolicy))</code> |
| <code>input</code> | <code>dto.KBCreate</code> | 在此声明中的实际使用：<code>input.Name</code>；<code>input.VisibilityPolicy</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, kb), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9514:10659 -->
````go
func (s *KnowledgeBases) Create(ctx context.Context, actor Actor, space uint64, input dto.KBCreate) (Actor, MutationBody, error) {
	_, proof, err := s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, "CREATE_KNOWLEDGE_BASE", 0)
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	actor.Sources, err = MergeSources(actor.Proof, proof)
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 成员来源gate一直持有至提交；退出准备必须等待建库和初始管理员事实提交。
		if _, err := repository.SpaceRow(ctx, tx, space, true); err != nil {
			return nil, err
		}
		kb, err := repository.CreateKnowledgeBase(ctx, tx, space, input.Name, domain.Visibility(input.VisibilityPolicy))
		if err != nil {
			return nil, err
		}
		if err = repository.InsertInitialKBAdministrator(ctx, tx, kb, actor.ID); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", kb), nil
	}
	return actor, run, nil
}
````

## 第 13 步：KnowledgeBases.Rename

插入位置：第 12 步代码之后，紧接追加；完成稿约第 286 行。

本步声明：`func (s *KnowledgeBases) Rename(ctx context.Context, actor Actor, id uint64, input dto.OrganizationEdit) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;RENAME_KB&quot;, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;RENAME_KB&quot;, false)</code>；<code>return actor, nil, err</code>；<code>actor.Sources</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;RENAME_KB&quot;, false)</code>；<code>repository.RenameKnowledgeBase(ctx, tx, id, input.Name)</code>；<code>domain.TextID(id)</code>；<code>Entity: id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.OrganizationEdit</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.Name</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=10659:11574 -->
````go
func (s *KnowledgeBases) Rename(ctx context.Context, actor Actor, id uint64, input dto.OrganizationEdit) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "RENAME_KB", false)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision); err != nil {
			return nil, err
		}
		if err := repository.RenameKnowledgeBase(ctx, tx, id, input.Name); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(access.KB.AclRevision)
		if err := QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "RENAME"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}
````

## 第 14 步：KnowledgeBases.CreationGrant

插入位置：第 13 步代码之后，紧接追加；完成稿约第 307 行。

本步声明：`func (s *KnowledgeBases) CreationGrant(ctx context.Context, actor Actor, id, user uint64, input dto.CreationGrant) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;KB_CREATION_GRANT&quot;, false)</code>；<code>s.bindTarget(ctx, actor, access, user, &quot;KB_CREATION_GRANT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;KB_CREATION_GRANT&quot;, false)</code>；<code>return actor, nil, err</code>；<code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_CREATION_GRANT_REQUIRED&quot;)</code>；<code>actor, err = s.bindTarget(ctx, actor, access, user, &quot;KB_CREATION_GRANT&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;KB_CREATION_GRANT&quot;, false)</code>；<code>repository.SetKBCreationGrant(ctx, tx, id, user, state)</code>；<code>repository.AdvanceKnowledgeBaseACL(ctx, tx, id)</code>；<code>domain.TextID(id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.bindTarget(ctx, actor, access, user, &quot;KB_CREATION_GRANT&quot;)</code>；<code>repository.SetKBCreationGrant(ctx, tx, id, user, state)</code> |
| <code>input</code> | <code>dto.CreationGrant</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.Allowed</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_CREATION_GRANT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>access.Space.Kind != &quot;ORGANIZATION&quot;</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_CREATION_GRANT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Allowed</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=11574:12968 -->
````go
func (s *KnowledgeBases) CreationGrant(ctx context.Context, actor Actor, id, user uint64, input dto.CreationGrant) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "KB_CREATION_GRANT", false)
	if err != nil {
		return actor, nil, err
	}
	if access.Space.Kind != "ORGANIZATION" {
		return actor, nil, domain.Violation(domain.PermissionDenied, "ORGANIZATION_CREATION_GRANT_REQUIRED")
	}
	actor, err = s.bindTarget(ctx, actor, access, user, "KB_CREATION_GRANT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		kb, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		state := "CLOSED"
		if input.Allowed {
			state = "ACTIVE"
		}
		if err = repository.SetKBCreationGrant(ctx, tx, id, user, state); err != nil {
			return nil, err
		}
		if err = repository.AdvanceKnowledgeBaseACL(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// knowledge_base_access.go：保留原实现，按业务合并文件。

````

## 第 15 步：KnowledgeBaseAccess

插入位置：第 14 步代码之后，紧接追加；完成稿约第 345 行。

本步声明：`type KnowledgeBaseAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>KB</code> | <code>dto.KnowledgeBase</code> | 已核验的知识库事实。 |
| <code>Space</code> | <code>dto.Space</code> | 所属空间事实，验证库和空间的真实关系。 |
| <code>Proof</code> | <code>*pb.IdentityAdmission</code> | Account身份准入证明。 |
| <code>Sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 本次知识库读取/修改依赖的权限来源及代次。 |

<!-- write-step: 15 bytes=12968:13108 -->
````go
type KnowledgeBaseAccess struct {
	KB      dto.KnowledgeBase
	Space   dto.Space
	Proof   *pb.IdentityAdmission
	Sources []*pb.SourceEpoch
}
````

## 第 16 步：KnowledgeBases

插入位置：第 15 步代码之后，紧接追加；完成稿约第 351 行。

本步声明：`type KnowledgeBases struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |

<!-- write-step: 16 bytes=13108:13196 -->
````go
type KnowledgeBases struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

````

## 第 17 步：NewKnowledgeBases

插入位置：第 16 步代码之后，紧接追加；完成稿约第 357 行。

本步声明：`func NewKnowledgeBases(db *sql.DB, identity *Identity, access *Access) *KnowledgeBases`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*KnowledgeBases</code> | 实际返回表达式：<code>&amp;KnowledgeBases{db: db, identity: identity, access: access}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;KnowledgeBases{db: db, identity: identity, access: access}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=13196:13355 -->
````go
func NewKnowledgeBases(db *sql.DB, identity *Identity, access *Access) *KnowledgeBases {
	return &KnowledgeBases{db: db, identity: identity, access: access}
}
````

## 第 18 步：KnowledgeBases.Manage

插入位置：第 17 步代码之后，紧接追加；完成稿约第 360 行。

本步声明：`func (s *KnowledgeBases) Manage(ctx context.Context, actor Actor, id uint64, purpose string, appoint bool) (KnowledgeBaseAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, s.db, id, false)</code>；<code>repository.SpaceRow(ctx, s.db, spaceID, false)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code>；<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, s.db, id, false)</code>；<code>repository.KnowledgeBaseRole(ctx, s.db, id, actor.ID)</code>；<code>domain.TextID(id)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)</code>；<code>s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)</code> |
| <code>appoint</code> | <code>bool</code> | 在此声明中的实际使用：<code>!appoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>KnowledgeBaseAccess</code> | 实际返回表达式：<code>access</code>；<code>KnowledgeBaseAccess{KB: kb, Space: space, Proof: proof, Sources: sources}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_ROLE_REQUIRED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_MANAGEMENT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>appoint</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>space.Kind != &quot;ORGANIZATION&quot;</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;ORGANIZATION_ROLE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &amp;&amp; !appoint</code>。
- 检查 <code>!errors.As(err, &amp;violation) &#124;&#124; violation.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>roleErr != nil &#124;&#124; role != &quot;ADMIN&quot;</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;KNOWLEDGE_BASE_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 此处通过 <code>return KnowledgeBaseAccess{KB: kb, Space: space, Proof: proof, Sources: sources}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13355:15120 -->
````go
func (s *KnowledgeBases) Manage(ctx context.Context, actor Actor, id uint64, purpose string, appoint bool) (KnowledgeBaseAccess, error) {
	var access KnowledgeBaseAccess
	kb, err := repository.KnowledgeBaseRow(ctx, s.db, id, false)
	if err != nil {
		return access, err
	}
	spaceID, _ := domain.CanonicalID(kb.SpaceId)
	if appoint {
		space, err := repository.SpaceRow(ctx, s.db, spaceID, false)
		if err != nil {
			return access, err
		}
		// 任免要求组织管理员；个人所有权不能凭空产生组织身份。
		if space.Kind != "ORGANIZATION" {
			return access, domain.Violation(domain.PermissionDenied, "ORGANIZATION_ROLE_REQUIRED")
		}
	}
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, 0)
	if err != nil && !appoint {
		var violation domain.RuleViolation
		if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
			return access, err
		}
		space, proof, err = s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, 0)
		if err == nil {
			role, roleErr := repository.KnowledgeBaseRole(ctx, s.db, id, actor.ID)
			if roleErr != nil || role != "ADMIN" {
				return access, domain.Violation(domain.PermissionDenied, "KNOWLEDGE_BASE_MANAGEMENT_REQUIRED")
			}
		}
	}
	if err != nil {
		return access, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return access, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return access, err
	}
	epoch, _ := domain.CanonicalID(kb.AclRevision)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(id), Epoch: epoch})
	return KnowledgeBaseAccess{KB: kb, Space: space, Proof: proof, Sources: sources}, nil
}
````

## 第 19 步：ValidateKnowledgeBase

插入位置：第 18 步代码之后，紧接追加；完成稿约第 405 行。

本步声明：`func ValidateKnowledgeBase(ctx context.Context, tx *sql.Tx, access KnowledgeBaseAccess, expected string) (dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, tx, id, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.KnowledgeBaseRow(ctx, tx, id, true)</code> |
| <code>access</code> | <code>KnowledgeBaseAccess</code> | 在此声明中的实际使用：<code>access.KB</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>current.AclRevision != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.KnowledgeBase</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;KB_ACL_REVISION_MISMATCH&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>current.AclRevision != access.KB.AclRevision &#124;&#124; current.AclRevision != expected</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;KB_ACL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=15120:15600 -->
````go
func ValidateKnowledgeBase(ctx context.Context, tx *sql.Tx, access KnowledgeBaseAccess, expected string) (dto.KnowledgeBase, error) {
	id, _ := domain.CanonicalID(access.KB.Id)
	current, err := repository.KnowledgeBaseRow(ctx, tx, id, true)
	if err != nil {
		return current, err
	}
	if current.AclRevision != access.KB.AclRevision || current.AclRevision != expected {
		return current, domain.Violation(domain.StateConflict, "KB_ACL_REVISION_MISMATCH")
	}
	return current, nil
}
````

## 第 20 步：KnowledgeBases.Target

插入位置：第 19 步代码之后，紧接追加；完成稿约第 416 行。

本步声明：`func (s *KnowledgeBases) Target(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> |
| <code>access</code> | <code>KnowledgeBaseAccess</code> | 在此声明中的实际使用：<code>access.Space</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>UserId: user</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code>；<code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 由 <code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code> 的第 1 个返回项原样转交。<br>由 <code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code> 的第 2 个返回项原样转交。<br>由 <code>s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 检查 <code>access.Space.Kind == &quot;PERSONAL&quot;</code>。条件成立时执行 <code>return s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)</code>。当前函数在这里结束。
- 此处通过 <code>return s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=15600:16216 -->
````go
func (s *KnowledgeBases) Target(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (*pb.IdentityAdmission, error) {
	if access.Space.Kind == "PERSONAL" {
		return s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: user}, purpose, 0)
	}
	organization, _ := domain.CanonicalID(*access.Space.OrganizationId)
	return s.identity.Target(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: organization, UserId: user}, purpose, 0)
}
````

## 第 21 步：KnowledgeBases.bindTarget

插入位置：第 20 步代码之后，紧接追加；完成稿约第 423 行。

本步声明：`func (s *KnowledgeBases) bindTarget(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (Actor, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Target(ctx, actor, access, user, purpose)</code>；<code>s.identity.Revalidate(ctx, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Target(ctx, actor, access, user, purpose)</code>；<code>return actor, err</code>；<code>actor.Proof</code>；<code>actor.Sources</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>access</code> | <code>KnowledgeBaseAccess</code> | 在此声明中的实际使用：<code>s.Target(ctx, actor, access, user, purpose)</code>；<code>access.Proof</code>；<code>access.Sources</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Target(ctx, actor, access, user, purpose)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Target(ctx, actor, access, user, purpose)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=16216:16875 -->
````go
func (s *KnowledgeBases) bindTarget(ctx context.Context, actor Actor, access KnowledgeBaseAccess, user uint64, purpose string) (Actor, error) {
	target, err := s.Target(ctx, actor, access, user, purpose)
	if err != nil {
		return actor, err
	}
	if err = s.identity.Revalidate(ctx, target); err != nil {
		return actor, err
	}
	sources, err := MergeSources(actor.Proof, access.Proof, target)
	if err != nil {
		return actor, err
	}
	baseSources, _ := MergeSources(actor.Proof, access.Proof)
	actor.Sources = append(sources, access.Sources[len(baseSources):]...)
	return actor, nil
}

// knowledge_base_deletion.go：保留原实现，按业务合并文件。

````

## 第 22 步：KnowledgeBases.Delete

插入位置：第 21 步代码之后，紧接追加；完成稿约第 442 行。

本步声明：`func (s *KnowledgeBases) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;DELETE_KB&quot;, false)</code>；<code>repository.KnowledgeBaseChildren(ctx, s.db, id, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;DELETE_KB&quot;, false)</code>；<code>return actor, nil, err</code>；<code>actor.Sources</code>；<code>return actor, nil, domain.Violation(domain.StateConflict, &quot;LIFECYCLE_PLAN_REQUIRED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;DELETE_KB&quot;, false)</code>；<code>repository.KnowledgeBaseChildren(ctx, s.db, id, false)</code>；<code>repository.PersonalTransferKnowledgeBasePending(ctx, tx, id)</code>；<code>repository.KnowledgeBaseChildren(ctx, tx, id, true)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;LIFECYCLE_PLAN_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>len(children) &gt; 400</code>。条件成立时执行 <code>return actor, nil, domain.Violation(domain.StateConflict, &quot;LIFECYCLE_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>pending</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PERSONAL_TRANSFER_PENDING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>child.HasRelease</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PUBLISHED_LIFECYCLE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>children&#91;child.ID&#93; != child.Epoch</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KB_CHILDREN_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(locked) != len(children)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KB_CHILDREN_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 22 bytes=16875:18651 -->
````go
func (s *KnowledgeBases) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "DELETE_KB", false)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	// 先按统一顺序锁子文档来源，再取得库/文档业务锁，避免与普通文档变更逆序等待。
	snapshot, err := repository.KnowledgeBaseChildren(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	children := map[uint64]uint64{}
	for _, child := range snapshot {
		children[child.ID] = child.Epoch
		actor.Sources = append(actor.Sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(child.ID), Epoch: child.Epoch})
	}
	if len(children) > 400 {
		return actor, nil, domain.Violation(domain.StateConflict, "LIFECYCLE_PLAN_REQUIRED")
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		pending, err := repository.PersonalTransferKnowledgeBasePending(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if pending {
			return nil, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_PENDING")
		}
		locked, err := repository.KnowledgeBaseChildren(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		for _, child := range locked {
			if child.HasRelease {
				return nil, domain.Violation(domain.StateConflict, "PUBLISHED_LIFECYCLE_REQUIRED")
			}
			if children[child.ID] != child.Epoch {
				return nil, domain.Violation(domain.StateConflict, "KB_CHILDREN_CHANGED")
			}
		}
		if len(locked) != len(children) {
			return nil, domain.Violation(domain.StateConflict, "KB_CHILDREN_CHANGED")
		}
````

## 第 23 步：KnowledgeBases.Delete（接着实现）

插入位置：第 22 步代码之后，紧接追加；完成稿约第 488 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=18651:19273 -->
````go
		if err = repository.CloseKnowledgeBaseForDeletion(ctx, tx, id); err != nil {
			return nil, err
		}
		if err = repository.CloseKnowledgeBaseChildren(ctx, tx, id); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(current.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "CLOSE_KB"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// knowledge_base_role.go：保留原实现，按业务合并文件。

````

## 第 24 步：KnowledgeBases.Role

插入位置：第 23 步代码之后，紧接追加；完成稿约第 505 行。

本步声明：`func (s *KnowledgeBases) Role(ctx context.Context, actor Actor, id, user uint64, input dto.KBRole) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;APPOINT_KB_ROLE&quot;, true)</code>；<code>s.bindTarget(ctx, actor, access, user, &quot;APPOINT_KB_ROLE&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;APPOINT_KB_ROLE&quot;, true)</code>；<code>return actor, nil, err</code>；<code>actor, err = s.bindTarget(ctx, actor, access, user, &quot;APPOINT_KB_ROLE&quot;)</code>；<code>s.bindTarget(ctx, actor, access, user, &quot;APPOINT_KB_ROLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.Manage(ctx, actor, id, &quot;APPOINT_KB_ROLE&quot;, true)</code>；<code>repository.KnowledgeBaseRole(ctx, tx, id, user)</code>；<code>repository.AvailableKBAdministrators(ctx, tx, id)</code>；<code>repository.KBAdministratorReservations(ctx, tx, id, user)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.bindTarget(ctx, actor, access, user, &quot;APPOINT_KB_ROLE&quot;)</code>；<code>repository.KnowledgeBaseRole(ctx, tx, id, user)</code>；<code>repository.KBAdministratorReservations(ctx, tx, id, user)</code>；<code>repository.SetKBRole(ctx, tx, id, user, role, state)</code> |
| <code>input</code> | <code>dto.KBRole</code> | 在此声明中的实际使用：<code>input.ExpectedAclRevision</code>；<code>input.Role</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Role == &quot;MEMBER&quot; &#124;&#124; input.Role == &quot;REMOVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>role == &quot;ADMIN&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;KB_ADMIN_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>count &lt;= 1</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;LAST_KB_ADMIN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;KB_ADMIN_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>reserved != 0</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;KB_ADMIN_RETIREMENT_RESERVED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Role == &quot;REMOVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=19273:20904 -->
````go
func (s *KnowledgeBases) Role(ctx context.Context, actor Actor, id, user uint64, input dto.KBRole) (Actor, MutationBody, error) {
	access, err := s.Manage(ctx, actor, id, "APPOINT_KB_ROLE", true)
	if err != nil {
		return actor, nil, err
	}
	actor, err = s.bindTarget(ctx, actor, access, user, "APPOINT_KB_ROLE")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		// 目标member来源已在通用变更事务持锁；持有Doc业务锁后不再反向调用账号RPC。
		kb, err := ValidateKnowledgeBase(ctx, tx, access, input.ExpectedAclRevision)
		if err != nil {
			return nil, err
		}
		if input.Role == "MEMBER" || input.Role == "REMOVE" {
			role, _ := repository.KnowledgeBaseRole(ctx, tx, id, user)
			if role == "ADMIN" {
				count, err := repository.AvailableKBAdministrators(ctx, tx, id)
				if err != nil {
					return nil, errors.New("KB_ADMIN_UNAVAILABLE")
				}
				if count <= 1 {
					return nil, domain.Violation(domain.StateConflict, "LAST_KB_ADMIN_REQUIRED")
				}
			}
		}
		reserved, err := repository.KBAdministratorReservations(ctx, tx, id, user)
		if err != nil {
			return nil, errors.New("KB_ADMIN_UNAVAILABLE")
		}
		if reserved != 0 {
			return nil, domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
		}
		role, state := input.Role, "ACTIVE"
		if input.Role == "REMOVE" {
			role = "MEMBER"
			state = "CLOSED"
		}
		if err = repository.SetKBRole(ctx, tx, id, user, role, state); err != nil {
			return nil, err
		}
		if err = repository.AdvanceKnowledgeBaseACL(ctx, tx, id); err != nil {
			return nil, err
		}
````

## 第 25 步：KnowledgeBases.Role（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 550 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=20904:21306 -->
````go
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// management.go：保留原实现，按业务合并文件。

````

## 第 26 步：Management

插入位置：第 25 步代码之后，紧接追加；完成稿约第 561 行。

本步声明：`type Management struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |

<!-- write-step: 26 bytes=21306:21390 -->
````go
type Management struct {
	db       *sql.DB
	identity *Identity
	access   *Access
}

````

## 第 27 步：NewManagement

插入位置：第 26 步代码之后，紧接追加；完成稿约第 567 行。

本步声明：`func NewManagement(db *sql.DB, identity *Identity, access *Access) *Management`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Management</code> | 实际返回表达式：<code>&amp;Management{db: db, identity: identity, access: access}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Management{db: db, identity: identity, access: access}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=21390:21537 -->
````go
func NewManagement(db *sql.DB, identity *Identity, access *Access) *Management {
	return &Management{db: db, identity: identity, access: access}
}
````

## 第 28 步：Management.managed

插入位置：第 27 步代码之后，紧接追加；完成稿约第 570 行。

本步声明：`func (s *Management) managed(ctx context.Context, actor Actor, id uint64, purpose string) (Actor, DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code>；<code>return actor, access, err</code>；<code>return actor, access, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, purpose)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, access, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, access, domain.Violation(domain.PermissionDenied, &quot;DOCUMENT_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, access, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=21537:21963 -->
````go
func (s *Management) managed(ctx context.Context, actor Actor, id uint64, purpose string) (Actor, DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, id, purpose)
	if err != nil {
		return actor, access, err
	}
	if !access.Permissions.Manage {
		return actor, access, domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
	}
	actor.Sources = access.Sources
	return actor, access, nil
}
````

## 第 29 步：Management.Rename

插入位置：第 28 步代码之后，紧接追加；完成稿约第 581 行。

本步声明：`func (s *Management) Rename(ctx context.Context, actor Actor, id uint64, input dto.DocumentEdit) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;RENAME_DOCUMENT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, access, err := s.managed(ctx, actor, id, &quot;RENAME_DOCUMENT&quot;)</code>；<code>s.managed(ctx, actor, id, &quot;RENAME_DOCUMENT&quot;)</code>；<code>return actor, nil, err</code>；<code>return actor, run, nil</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;RENAME_DOCUMENT&quot;)</code>；<code>repository.RenameDocument(ctx, tx, id, input.Title)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.DocumentEdit</code> | 在此声明中的实际使用：<code>input.ExpectedControlRevision</code>；<code>input.Title</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.ControlRevision != input.ExpectedControlRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=21963:22715 -->
````go
func (s *Management) Rename(ctx context.Context, actor Actor, id uint64, input dto.DocumentEdit) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "RENAME_DOCUMENT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedControlRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if err = repository.RenameDocument(ctx, tx, id, input.Title); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
````

## 第 30 步：Management.Delete

插入位置：第 29 步代码之后，紧接追加；完成稿约第 601 行。

本步声明：`func (s *Management) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;DELETE_DOCUMENT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, access, err := s.managed(ctx, actor, id, &quot;DELETE_DOCUMENT&quot;)</code>；<code>s.managed(ctx, actor, id, &quot;DELETE_DOCUMENT&quot;)</code>；<code>return actor, nil, err</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.managed(ctx, actor, id, &quot;DELETE_DOCUMENT&quot;)</code>；<code>repository.CloseDocument(ctx, tx, id)</code>；<code>queueVisibilityIndexEvents(ctx, tx, input.OperationId, &#91;&#93;uint64{id}, 0)</code>；<code>domain.TextID(id)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>document.ControlRevision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CONTROL_REVISION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=22715:24170 -->
````go
func (s *Management) Delete(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction) (Actor, MutationBody, error) {
	actor, access, err := s.managed(ctx, actor, id, "DELETE_DOCUMENT")
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		if err = repository.CloseDocument(ctx, tx, id); err != nil {
			return nil, err
		}
		// 已发布文档同样可移入回收站。先停止当前正式指针，并在这一事务
		// 推进不可见索引事件；旧正文输出仍由下方doc-acl实际ACK排空。
		if err = queueVisibilityIndexEvents(ctx, tx, input.OperationId, []uint64{id}, 0); err != nil {
			return nil, err
		}
		epoch, _ := domain.CanonicalID(document.AclRevision)
		// 删除先冻结准入；恢复器核验旧输出停止后才设置TRASHED及清理期限。
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "TRASH_DOCUMENT"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// space.go：保留原实现，按业务合并文件。

````

## 第 31 步：DirectoryReader

插入位置：第 30 步代码之后，紧接追加；完成稿约第 634 行。

本步声明：`type DirectoryReader interface {`。

**这一段为什么这样写：** 空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Read</code> | <code>(context.Context, string, string, any) error</code> | 参数顺序为ctx、用户会话令牌、内部目录路径、输出对象；从Account取得当前可访问目录，错误不能当空列表吞掉。 |

<!-- write-step: 31 bytes=24170:24255 -->
````go
type DirectoryReader interface {
	Read(context.Context, string, string, any) error
}
````

## 第 32 步：Spaces

插入位置：第 31 步代码之后，紧接追加；完成稿约第 637 行。

本步声明：`type Spaces struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>directory</code> | <code>DirectoryReader</code> | 使用 <code>DirectoryReader</code> 类型；其定义处的职责：空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |

<!-- write-step: 32 bytes=24255:24353 -->
````go
type Spaces struct {
	identity   *Identity
	directory  DirectoryReader
	pagination *Pagination
}

````

## 第 33 步：NewSpaces

插入位置：第 32 步代码之后，紧接追加；完成稿约第 643 行。

本步声明：`func NewSpaces(identity *Identity, directory DirectoryReader, pagination *Pagination) *Spaces`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>directory</code> | <code>DirectoryReader</code> | 使用 <code>DirectoryReader</code> 类型；其定义处的职责：空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。<br>在此声明中的实际使用：<code>directory: directory</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Spaces</code> | 实际返回表达式：<code>&amp;Spaces{identity: identity, directory: directory, pagination: pagination}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Spaces{identity: identity, directory: directory, pagination: pagination}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=24353:24534 -->
````go
func NewSpaces(identity *Identity, directory DirectoryReader, pagination *Pagination) *Spaces {
	return &Spaces{identity: identity, directory: directory, pagination: pagination}
}

````

## 第 34 步：MaterializeSpace

插入位置：第 33 步代码之后，紧接追加；完成稿约第 647 行。

既定DDL没有owner/org唯一索引；持久唯一gate保证并发首次打开只物化一个空间。

本步声明：`func MaterializeSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.EnsureSpaceGate(ctx, tx, key)</code>；<code>repository.LockSpaceGate(ctx, tx, key)</code>；<code>repository.ExistingSpace(ctx, tx, kind, owner)</code>；<code>repository.CreateSpace(ctx, tx, kind, owner)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.EnsureSpaceGate(ctx, tx, key)</code>；<code>repository.LockSpaceGate(ctx, tx, key)</code>；<code>repository.ExistingSpace(ctx, tx, kind, owner)</code>；<code>repository.CreateSpace(ctx, tx, kind, owner)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;ORGANIZATION&quot;</code>；<code>repository.ExistingSpace(ctx, tx, kind, owner)</code>；<code>repository.CreateSpace(ctx, tx, kind, owner)</code> |
| <code>owner</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(owner)</code>；<code>repository.ExistingSpace(ctx, tx, kind, owner)</code>；<code>repository.CreateSpace(ctx, tx, kind, owner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 由 <code>repository.SpaceRow(ctx, tx, existing, true)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>dto.Space{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.SpaceRow(ctx, tx, existing, true)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>kind == &quot;ORGANIZATION&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return dto.Space{}, err</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return dto.Space{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return repository.SpaceRow(ctx, tx, existing, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=24534:25365 -->
````go
// 既定DDL没有owner/org唯一索引；持久唯一gate保证并发首次打开只物化一个空间。
func MaterializeSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (dto.Space, error) {
	key := "space-user:" + domain.TextID(owner)
	if kind == "ORGANIZATION" {
		key = "space-org:" + domain.TextID(owner)
	}
	if err := repository.EnsureSpaceGate(ctx, tx, key); err != nil {
		return dto.Space{}, err
	}
	if err := repository.LockSpaceGate(ctx, tx, key); err != nil {
		return dto.Space{}, err
	}
	existing, err := repository.ExistingSpace(ctx, tx, kind, owner)
	if err == sql.ErrNoRows {
		existing, err = repository.CreateSpace(ctx, tx, kind, owner)
		if err != nil {
			return dto.Space{}, err
		}
	} else if err != nil {
		return dto.Space{}, err
	}
	return repository.SpaceRow(ctx, tx, existing, true)
}
````

## 第 35 步：Spaces.Prepare

插入位置：第 34 步代码之后，紧接追加；完成稿约第 670 行。

本步声明：`func (s *Spaces) Prepare(ctx context.Context, actor Actor, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations?limit=&quot;+domain.TextID(uint64(directoryLimit))+&quot;&amp;cursor=&quot;+url.QueryEscape(accountCursor), &amp;directory)</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, target, &quot;SPACE_DIRECTORY&quot;, 0, 0)</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, target, &quot;SPACE_DIRECTORY&quot;, 0, 0)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, target, &quot;SPACE_DIRECTORY&quot;, 0, 0)</code>；<code>s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, target, &quot;SPACE_DIRECTORY&quot;, 0, 0)</code> |
| <code>cookie</code> | <code>string</code> | 在此声明中的实际使用：<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations?limit=&quot;+domain.TextID(uint64(directoryLimit))+&quot;&amp;cursor=&quot;+url.QueryEscape(accountCursor), &amp;directory)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>limit &gt; 5</code>。
- 检查 <code>last == &quot;@org&quot;</code>。
- 检查 <code>personal &amp;&amp; limit &gt; 1</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>firstOnly</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。

<!-- write-step: 35 bytes=25365:27227 -->
````go
func (s *Spaces) Prepare(ctx context.Context, actor Actor, cookie, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "spaces:" + domain.TextID(actor.ID)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	// limit是单页上限，不承诺填满。每个组织都要核验成员/建库资格并登记撤权来源，
	// 大页会耗尽短请求预算；用完整cursor续接小页，不能跳过证明或放宽失效边界。
	if limit > 5 {
		limit = 5
	}
	personal := last == ""
	accountCursor := last
	if last == "@org" {
		accountCursor = ""
	}
	directoryLimit := limit
	if personal && limit > 1 {
		directoryLimit--
	}
	var directory dto.OrganizationPage
	if err = s.directory.Read(ctx, cookie, "/v1/organizations?limit="+domain.TextID(uint64(directoryLimit))+"&cursor="+url.QueryEscape(accountCursor), &directory); err != nil {
		return nil, nil, err
	}
	proofs := []*pb.IdentityAdmission{actor.Proof}
	firstOnly := personal && limit == 1
	moreAfterPersonal := firstOnly && len(directory.Items) > 0
	if firstOnly {
		directory.Items = nil
	}
	canCreate := map[string]bool{}
	for _, organization := range directory.Items {
		id, _ := domain.CanonicalID(organization.Id)
		target := &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}
		proof, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_MEMBER, target, "SPACE_DIRECTORY", 0, 0)
		if err != nil {
			return nil, nil, err
		}
		proofs = append(proofs, proof)
		creation, err := s.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE, target, "SPACE_DIRECTORY", 0, 0)
		if err == nil {
			canCreate[organization.Id] = true
			proofs = append(proofs, creation)
		} else {
			var violation domain.RuleViolation
````

## 第 36 步：Spaces.Prepare（接着实现）

插入位置：第 35 步代码之后，紧接追加；完成稿约第 715 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!errors.As(err, &amp;violation) &#124;&#124; violation.Kind != domain.PermissionDenied</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>personal</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>canCreate&#91;organization.Id&#93;</code>。
- 在本段的匿名函数内：检查 <code>firstOnly</code>。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>directory.NextCursor != nil</code>。

<!-- write-step: 36 bytes=27227:28528 -->
````go
			if !errors.As(err, &violation) || violation.Kind != domain.PermissionDenied {
				return nil, nil, err
			}
		}
	}
	for _, proof := range proofs {
		if err = s.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, err
		}
	}
	sources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		page := dto.SpacePage{Items: []dto.Space{}}
		if personal {
			space, err := MaterializeSpace(ctx, tx, "PERSONAL", actor.ID)
			if err != nil {
				return nil, err
			}
			space.Actions = []string{"CREATE_KNOWLEDGE_BASE", "CREATE_DOCUMENT"}
			page.Items = append(page.Items, space)
		}
		for _, organization := range directory.Items {
			id, _ := domain.CanonicalID(organization.Id)
			space, err := MaterializeSpace(ctx, tx, "ORGANIZATION", id)
			if err != nil {
				return nil, err
			}
			space.Actions = []string{}
			if canCreate[organization.Id] {
				space.Actions = append(space.Actions, "CREATE_KNOWLEDGE_BASE")
			}
			page.Items = append(page.Items, space)
		}
		page.HasMore = directory.HasMore
		if firstOnly {
			page.HasMore = moreAfterPersonal
			if page.HasMore {
				page.NextCursor = s.pagination.Next(binding, "@org")
			}
			return page, nil
		}
		if directory.NextCursor != nil {
````

## 第 37 步：Spaces.Prepare（接着实现）

插入位置：第 36 步代码之后，紧接追加；完成稿约第 760 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=28528:28653 -->
````go
			page.NextCursor = s.pagination.Next(binding, *directory.NextCursor)
		}
		return page, nil
	}
	return sources, run, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/workspace_management_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
