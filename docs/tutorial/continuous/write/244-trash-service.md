# 分步编写 `services/document/internal/logic/document/trash_service.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/244-trash-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按当前管理资格分页显示回收站并恢复未过期文档。

恢复要越过旧撤权水位且保留固定正文，签名游标不是访问许可。

写之前先按这条执行路线理解各段的连接：

1. spaceAuthority/documentAuthority核资格
2. validate先KB后Doc当前读
3. List登记返回及lookahead来源
4. Restore识别原号并计划更高修订。

本文件需要守住的边界：

- 父级必须ACTIVE且DB保留期未过
- 同号presentation不能登记高于当前metadata的来源。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/trash_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/trash_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/trash_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/trash_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:450 -->
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
	"encoding/hex"
	"errors"
	"time"
)

// trash.go：保留原实现，按业务合并文件。

````

## 第 2 步：Trash

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。
不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。

本步声明：`type Trash struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |

<!-- write-step: 2 bytes=450:765 -->
````go
// Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。
// 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。
type Trash struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
}

````

## 第 3 步：NewTrash

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func NewTrash(db *sql.DB, identity *Identity, access *Access, pagination *Pagination) *Trash`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Trash</code> | 使用 <code>Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。<br>实际返回表达式：<code>&amp;Trash{db: db, identity: identity, access: access, pagination: pagination}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Trash{db: db, identity: identity, access: access, pagination: pagination}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=765:946 -->
````go
func NewTrash(db *sql.DB, identity *Identity, access *Access, pagination *Pagination) *Trash {
	return &Trash{db: db, identity: identity, access: access, pagination: pagination}
}

````

## 第 4 步：trashSpaceAccess

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`type trashSpaceAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Space</code> | <code>dto.Space</code> | 已核验的回收站所属空间事实。 |
| <code>Proof</code> | <code>*pb.IdentityAdmission</code> | Account身份/组织准入证明。 |
| <code>Inherited</code> | <code>bool</code> | 是否依靠继承的组织管理资格访问，决定需要记录的来源。 |

<!-- write-step: 4 bytes=946:1050 -->
````go
type trashSpaceAccess struct {
	Space     dto.Space
	Proof     *pb.IdentityAdmission
	Inherited bool
}

````

## 第 5 步：trashDocumentAccess

插入位置：第 4 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`type trashDocumentAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Facts</code> | <code>domain.TrashFacts</code> | 使用 <code>domain.TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。 |
| <code>KB</code> | <code>*dto.KnowledgeBase</code> | 文档原属知识库事实，可为空；回收站仍要验证原空间与库关系。 |

<!-- write-step: 5 bytes=1050:1138 -->
````go
type trashDocumentAccess struct {
	Facts domain.TrashFacts
	KB    *dto.KnowledgeBase
}

````

## 第 6 步：Trash.spaceAuthority

插入位置：第 5 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func (s *Trash) spaceAuthority(ctx context.Context, actor Actor, id uint64, purpose string, document uint64) (trashSpaceAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashSpaceRow(ctx, s.db, id, false)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)</code>；<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashSpaceRow(ctx, s.db, id, false)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)</code>；<code>s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>trashSpaceAccess</code> | 实际返回表达式：<code>admission</code>；<code>trashSpaceAccess{Space: current, Proof: proof, Inherited: inherited}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_SPACE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return admission, err</code>。当前函数在这里结束。
- 检查 <code>space.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return admission, domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;denied) &#124;&#124; denied.Kind != domain.PermissionDenied &#124;&#124; space.Kind != &quot;ORGANIZATION&quot;</code>。条件成立时执行 <code>return admission, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return admission, err</code>。当前函数在这里结束。
- 检查 <code>current.Id != space.Id &#124;&#124; current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return admission, domain.Violation(domain.StateConflict, &quot;TRASH_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return admission, err</code>。当前函数在这里结束。
- 此处通过 <code>return trashSpaceAccess{Space: current, Proof: proof, Inherited: inherited}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1138:2354 -->
````go
func (s *Trash) spaceAuthority(ctx context.Context, actor Actor, id uint64, purpose string, document uint64) (trashSpaceAccess, error) {
	var admission trashSpaceAccess
	space, err := repository.TrashSpaceRow(ctx, s.db, id, false)
	if err != nil {
		return admission, err
	}
	if space.Lifecycle != "ACTIVE" {
		return admission, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	current, proof, err := s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_ADMIN, purpose, document)
	inherited := err == nil
	if err != nil {
		var denied domain.RuleViolation
		if !errors.As(err, &denied) || denied.Kind != domain.PermissionDenied || space.Kind != "ORGANIZATION" {
			return admission, err
		}
		current, proof, err = s.access.SpaceIdentity(ctx, actor, id, pb.IdentityCapability_ORGANIZATION_MEMBER, purpose, document)
	}
	if err != nil {
		return admission, err
	}
	if current.Id != space.Id || current.Lifecycle != "ACTIVE" {
		return admission, domain.Violation(domain.StateConflict, "TRASH_SPACE_CHANGED")
	}
	if err := s.identity.Revalidate(ctx, proof); err != nil {
		return admission, err
	}
	return trashSpaceAccess{Space: current, Proof: proof, Inherited: inherited}, nil
}

````

## 第 7 步：Trash.documentAuthority

插入位置：第 6 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func (s *Trash) documentAuthority(ctx context.Context, q repository.Queryer, actor Actor, space trashSpaceAccess, facts domain.TrashFacts, lock bool) (trashDocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashKnowledgeBaseRow(ctx, q, id, lock)</code>；<code>repository.TrashKBRole(ctx, q, id, actor.ID, lock)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.TrashKnowledgeBaseRow(ctx, q, id, lock)</code>；<code>repository.TrashKBRole(ctx, q, id, actor.ID, lock)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>space</code> | <code>trashSpaceAccess</code> | 在此声明中的实际使用：<code>space.Space</code>；<code>space.Inherited</code> |
| <code>facts</code> | <code>domain.TrashFacts</code> | 使用 <code>domain.TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>Facts: facts</code>；<code>facts.Document</code> |
| <code>lock</code> | <code>bool</code> | 在此声明中的实际使用：<code>repository.TrashKnowledgeBaseRow(ctx, q, id, lock)</code>；<code>repository.TrashKBRole(ctx, q, id, actor.ID, lock)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>trashDocumentAccess</code> | 实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;TRASH_SPACE_CHANGED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;TRASH_MANAGEMENT_REQUIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>facts.Document.SpaceId != space.Space.Id</code>。条件成立时执行 <code>return access, domain.Violation(domain.StateConflict, &quot;TRASH_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.Document.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>kb.SpaceId != facts.Document.SpaceId</code>。条件成立时执行 <code>return access, domain.Violation(domain.TemporarilyUnavailable, &quot;TRASH_KNOWLEDGE_BASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>!space.Inherited &amp;&amp; kb.Lifecycle == &quot;ACTIVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return access, err</code>。当前函数在这里结束。
- 检查 <code>!domain.CanManageTrash(permissions)</code>。条件成立时执行 <code>return access, domain.Violation(domain.PermissionDenied, &quot;TRASH_MANAGEMENT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return access, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2354:4055 -->
````go
func (s *Trash) documentAuthority(ctx context.Context, q repository.Queryer, actor Actor, space trashSpaceAccess, facts domain.TrashFacts, lock bool) (trashDocumentAccess, error) {
	access := trashDocumentAccess{Facts: facts}
	if facts.Document.SpaceId != space.Space.Id {
		return access, domain.Violation(domain.StateConflict, "TRASH_SPACE_CHANGED")
	}
	permissions := domain.PermissionFacts{PersonalOwner: space.Inherited && space.Space.Kind == "PERSONAL", OrganizationAdmin: space.Inherited && space.Space.Kind == "ORGANIZATION"}
	// spaceAuthority对组织普通成员也取得真实成员proof；负责关系只读当前Doc，
	// 事务内再比较原负责ID及ACL。离组/注销已经清NULL的关系不会复活。
	permissions.Responsible = space.Space.Kind == "ORGANIZATION" && facts.Document.ResponsibleUserId != nil && *facts.Document.ResponsibleUserId == domain.TextID(actor.ID)
	if facts.Document.KnowledgeBaseId != nil {
		id, err := domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return access, err
		}
		kb, err := repository.TrashKnowledgeBaseRow(ctx, q, id, lock)
		if err != nil {
			return access, err
		}
		if kb.SpaceId != facts.Document.SpaceId {
			return access, domain.Violation(domain.TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
		}
		access.KB = &kb
		if !space.Inherited && kb.Lifecycle == "ACTIVE" {
			role, err := repository.TrashKBRole(ctx, q, id, actor.ID, lock)
			if err != nil {
				return access, err
			}
			permissions.KnowledgeBaseAdmin = role == "ADMIN"
		}
	}
	if !domain.CanManageTrash(permissions) {
		return access, domain.Violation(domain.PermissionDenied, "TRASH_MANAGEMENT_REQUIRED")
	}
	return access, nil
}

````

## 第 8 步：trashSources

插入位置：第 7 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`func trashSources(actor Actor, space trashSpaceAccess, access trashDocumentAccess, acl, publication uint64) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code> |
| <code>space</code> | <code>trashSpaceAccess</code> | 在此声明中的实际使用：<code>space.Proof</code> |
| <code>access</code> | <code>trashDocumentAccess</code> | 在此声明中的实际使用：<code>access.KB</code>；<code>access.Facts</code> |
| <code>acl</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Epoch: acl</code> |
| <code>publication</code> | <code>uint64</code> | 在此声明中的实际使用：<code>publication != 0</code>；<code>Epoch: publication</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>access.KB != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>publication != 0</code>。
- 此处通过 <code>return sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4055:4779 -->
````go
func trashSources(actor Actor, space trashSpaceAccess, access trashDocumentAccess, acl, publication uint64) ([]*pb.SourceEpoch, error) {
	sources, err := MergeSources(actor.Proof, space.Proof)
	if err != nil {
		return nil, err
	}
	if access.KB != nil {
		epoch, err := domain.CanonicalID(access.KB.AclRevision)
		if err != nil {
			return nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + access.KB.Id, Epoch: epoch})
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + access.Facts.Document.Id, Epoch: acl})
	if publication != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "publication:" + access.Facts.Document.Id, Epoch: publication})
	}
	return sources, nil
}

````

## 第 9 步：Trash.validate

插入位置：第 8 步代码之后，紧接追加；完成稿约第 127 行。

validate先锁KB再Doc，避免与持kb-acl gate的完整visibility fanout倒序。
Account/角色来源gate已经由外层按scope字典序取得；角色必须用当前读，不能复用RR旧快照。

本步声明：`func (s *Trash) validate(ctx context.Context, tx *sql.Tx, actor Actor, space trashSpaceAccess, original trashDocumentAccess) (domain.TrashFacts, *dto.KnowledgeBase, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.documentAuthority(ctx, tx, actor, space, original.Facts, true)</code>；<code>repository.TrashDocumentRow(ctx, tx, id, true)</code>；<code>repository.TrashSpaceRow(ctx, tx, spaceID, true)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.documentAuthority(ctx, tx, actor, space, original.Facts, true)</code>；<code>repository.TrashDocumentRow(ctx, tx, id, true)</code>；<code>repository.TrashSpaceRow(ctx, tx, spaceID, true)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.documentAuthority(ctx, tx, actor, space, original.Facts, true)</code> |
| <code>space</code> | <code>trashSpaceAccess</code> | 在此声明中的实际使用：<code>s.documentAuthority(ctx, tx, actor, space, original.Facts, true)</code>；<code>space.Space</code> |
| <code>original</code> | <code>trashDocumentAccess</code> | 在此声明中的实际使用：<code>original.Facts</code>；<code>original.KB</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.TrashFacts</code> | 使用 <code>domain.TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>实际返回表达式：<code>domain.TrashFacts{}</code>；<code>current</code> |
| <code>(匿名/未命名)</code> | <code>*dto.KnowledgeBase</code> | 实际返回表达式：<code>nil</code>；<code>qualified.KB</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_KNOWLEDGE_BASE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.TrashFacts{}, nil, err</code>。当前函数在这里结束。
- 检查 <code>(qualified.KB == nil) != (original.KB == nil) &#124;&#124; (qualified.KB != nil &amp;&amp; qualified.KB.AclRevision != original.KB.AclRevision)</code>。条件成立时执行 <code>return domain.TrashFacts{}, nil, domain.Violation(domain.StateConflict, &quot;TRASH_KNOWLEDGE_BASE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, nil, err</code>。当前函数在这里结束。
- 检查 <code>d.Lifecycle != &quot;TRASHED&quot; &#124;&#124; d.SpaceId != before.SpaceId &#124;&#124; d.AclRevision != before.AclRevision &#124;&#124; 		!sameTrashID(d.KnowledgeBaseId, before.KnowledgeBaseId) &#124;&#124; !sameTrashID(d.ResponsibleUserId, before.ResponsibleUserId)</code>。条件成立时执行 <code>return current, nil, domain.Violation(domain.StateConflict, &quot;TRASH_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, nil, err</code>。当前函数在这里结束。
- 检查 <code>currentSpace.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; currentSpace.Kind != space.Space.Kind &#124;&#124; 		!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) &#124;&#124; !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId)</code>。条件成立时执行 <code>return current, nil, domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, qualified.KB, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4779:6596 -->
````go
// validate先锁KB再Doc，避免与持kb-acl gate的完整visibility fanout倒序。
// Account/角色来源gate已经由外层按scope字典序取得；角色必须用当前读，不能复用RR旧快照。
func (s *Trash) validate(ctx context.Context, tx *sql.Tx, actor Actor, space trashSpaceAccess, original trashDocumentAccess) (domain.TrashFacts, *dto.KnowledgeBase, error) {
	qualified, err := s.documentAuthority(ctx, tx, actor, space, original.Facts, true)
	if err != nil {
		return domain.TrashFacts{}, nil, err
	}
	if (qualified.KB == nil) != (original.KB == nil) || (qualified.KB != nil && qualified.KB.AclRevision != original.KB.AclRevision) {
		return domain.TrashFacts{}, nil, domain.Violation(domain.StateConflict, "TRASH_KNOWLEDGE_BASE_CHANGED")
	}
	id, _ := domain.CanonicalID(original.Facts.Document.Id)
	current, err := repository.TrashDocumentRow(ctx, tx, id, true)
	if err != nil {
		return current, nil, err
	}
	d, before := current.Document, original.Facts.Document
	if d.Lifecycle != "TRASHED" || d.SpaceId != before.SpaceId || d.AclRevision != before.AclRevision ||
		!sameTrashID(d.KnowledgeBaseId, before.KnowledgeBaseId) || !sameTrashID(d.ResponsibleUserId, before.ResponsibleUserId) {
		return current, nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
	}
	spaceID, _ := domain.CanonicalID(d.SpaceId)
	currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
	if err != nil {
		return current, nil, err
	}
	if currentSpace.Lifecycle != "ACTIVE" || currentSpace.Kind != space.Space.Kind ||
		!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) || !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId) {
		return current, nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	return current, qualified.KB, nil
}

````

## 第 10 步：sameTrashID

插入位置：第 9 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func sameTrashID(a, b *string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>*string</code> | 在此声明中的实际使用：<code>a == nil</code>；<code>a != nil</code>；<code>*a == *b</code> |
| <code>b</code> | <code>*string</code> | 在此声明中的实际使用：<code>b == nil</code>；<code>b != nil</code>；<code>*a == *b</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>(a == nil &amp;&amp; b == nil) &#124;&#124; (a != nil &amp;&amp; b != nil &amp;&amp; *a == *b)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return (a == nil &amp;&amp; b == nil) &#124;&#124; (a != nil &amp;&amp; b != nil &amp;&amp; *a == *b)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6596:6706 -->
````go
func sameTrashID(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

````

## 第 11 步：trashCursorBinding

插入位置：第 10 步代码之后，紧接追加；完成稿约第 163 行。

Cursor绑定真实会话及空间；列表输出仍登记所有返回项和lookahead依赖的来源。
签名仅证明分页位置，不能代替下一次请求的当前资格复核。

本步声明：`func trashCursorBinding(actor Actor, space uint64) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(space)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;trash:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + domain.TextID(space) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;trash:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + domain.TextID(space) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6706:7146 -->
````go
// Cursor绑定真实会话及空间；列表输出仍登记所有返回项和lookahead依赖的来源。
// 签名仅证明分页位置，不能代替下一次请求的当前资格复核。
func trashCursorBinding(actor Actor, space uint64) string {
	return "trash:" + domain.TextID(actor.ID) + ":" + domain.TextID(space) + ":" + hex.EncodeToString(actor.Proof.SessionRef)
}

// trash_list.go：保留原实现，按业务合并文件。

````

## 第 12 步：Trash.List

插入位置：第 11 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func (s *Trash) List(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.spaceAuthority(ctx, actor, spaceID, &quot;LIST_TRASH&quot;, 0)</code>；<code>repository.TrashCandidates(ctx, s.db, spaceID, actor.ID, space.Inherited, last, limit+1)</code>；<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.documentAuthority(ctx, s.db, actor, space, facts, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.spaceAuthority(ctx, actor, spaceID, &quot;LIST_TRASH&quot;, 0)</code>；<code>trashCursorBinding(actor, spaceID)</code>；<code>actor.ID</code>；<code>actor.Proof</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>spaceID</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.spaceAuthority(ctx, actor, spaceID, &quot;LIST_TRASH&quot;, 0)</code>；<code>trashCursorBinding(actor, spaceID)</code>；<code>repository.TrashCandidates(ctx, s.db, spaceID, actor.ID, space.Inherited, last, limit+1)</code>；<code>repository.TrashSpaceRow(ctx, tx, spaceID, true)</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;TRASH_SOURCE_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>lastText != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>facts.Document.Lifecycle != &quot;TRASHED&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;TRASH_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=7146:8671 -->
````go
func (s *Trash) List(ctx context.Context, actor Actor, spaceID uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	space, err := s.spaceAuthority(ctx, actor, spaceID, "LIST_TRASH", 0)
	if err != nil {
		return nil, nil, err
	}
	binding := trashCursorBinding(actor, spaceID)
	limit, lastText, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	var last uint64
	if lastText != "" {
		last, err = domain.CanonicalID(lastText)
		if err != nil {
			return nil, nil, domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		}
	}
	ids, err := repository.TrashCandidates(ctx, s.db, spaceID, actor.ID, space.Inherited, last, limit+1)
	if err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, space.Proof)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[string]uint64, len(sources))
	for _, source := range sources {
		seen[source.ScopeKey] = source.Epoch
	}
	accesses := make([]trashDocumentAccess, 0, len(ids))
	for _, id := range ids {
		facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
		if err != nil {
			return nil, nil, err
		}
		if facts.Document.Lifecycle != "TRASHED" {
			return nil, nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
		}
		access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
		if err != nil {
			return nil, nil, err
		}
		acl, err := domain.CanonicalID(facts.Document.AclRevision)
		if err != nil {
			return nil, nil, err
		}
````

## 第 13 步：Trash.List（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 217 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>exists</code>。
- 检查 <code>epoch != source.Epoch</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;TRASH_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(accesses) == 0</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>currentSpace.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; currentSpace.Kind != space.Space.Kind &#124;&#124; 				!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) &#124;&#124; !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>index &lt; limit</code>。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8671:10401 -->
````go
		itemSources, err := trashSources(actor, space, access, acl, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, source := range itemSources {
			if epoch, exists := seen[source.ScopeKey]; exists {
				if epoch != source.Epoch {
					return nil, nil, domain.Violation(domain.StateConflict, "TRASH_SOURCE_CHANGED")
				}
				continue
			}
			seen[source.ScopeKey] = source.Epoch
			sources = append(sources, source)
		}
		accesses = append(accesses, access)
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 有结果时validate按KB→Doc→Space当前读；空列表才单独锁Space，避免
		// 先做一致性读建立旧ReadView，也不把空间注销后的快照当作合法空页。
		if len(accesses) == 0 {
			currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
			if err != nil {
				return nil, err
			}
			if currentSpace.Lifecycle != "ACTIVE" || currentSpace.Kind != space.Space.Kind ||
				!sameTrashID(currentSpace.OwnerUserId, space.Space.OwnerUserId) || !sameTrashID(currentSpace.OrganizationId, space.Space.OrganizationId) {
				return nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
			}
		}
		page := dto.DocumentPage{Items: []dto.Document{}, HasMore: len(accesses) > limit}
		for index, access := range accesses {
			current, _, err := s.validate(ctx, tx, actor, space, access)
			if err != nil {
				return nil, err
			}
			if index < limit {
				page.Items = append(page.Items, current.Document)
			}
		}
		if page.HasMore {
			// lookahead只探测，不将其ID写入cursor，否则下一页会吞掉这条真实回收项。
			page.NextCursor = s.pagination.Next(binding, page.Items[len(page.Items)-1].Id)
		}
		return page, nil
	}
````

## 第 14 步：Trash.List（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 262 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10401:10496 -->
````go
	return sources, run, nil
}

// trash_restore.go：保留原实现，按业务合并文件。

````

## 第 15 步：Trash.Restore

插入位置：第 14 步代码之后，紧接追加；完成稿约第 267 行。

与发布Prepare相同，返回必需的presentation：同号竞态即使跳过run，也不能
提交比当前Doc metadata更高的来源。固定结果本身仍由标准Mutation读取。

本步声明：`func (s *Trash) Restore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.prepareRestore(ctx, actor, id, input)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, run, err := s.prepareRestore(ctx, actor, id, input)</code>；<code>s.prepareRestore(ctx, actor, id, input)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, run, s.restorePresentation(actor, id), nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.prepareRestore(ctx, actor, id, input)</code>；<code>s.restorePresentation(actor, id)</code> |
| <code>input</code> | <code>dto.TrashAction</code> | 在此声明中的实际使用：<code>s.prepareRestore(ctx, actor, id, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>s.restorePresentation(actor, id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, run, s.restorePresentation(actor, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=10496:11006 -->
````go
// 与发布Prepare相同，返回必需的presentation：同号竞态即使跳过run，也不能
// 提交比当前Doc metadata更高的来源。固定结果本身仍由标准Mutation读取。
func (s *Trash) Restore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, MutationPresentation, error) {
	actor, run, err := s.prepareRestore(ctx, actor, id, input)
	if err != nil {
		return actor, nil, nil, err
	}
	return actor, run, s.restorePresentation(actor, id), nil
}

````

## 第 16 步：Trash.prepareRestore

插入位置：第 15 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`func (s *Trash) prepareRestore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>s.documentAuthority(ctx, s.db, actor, space, facts, false)</code>；<code>repository.TrashRestoreOriginalReceipt(ctx, s.db, input.OperationId)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>s.documentAuthority(ctx, s.db, actor, space, facts, false)</code>；<code>actor.ID</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>domain.ValidateTrashRestoreReceipt(receipt, input.OperationId, actor.ID, id)</code>；<code>s.prepareOriginalRestore(ctx, actor, id)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.TrashAction</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.ExpectedControlRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>s.prepareOriginalRestore(ctx, actor, id)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.prepareOriginalRestore(ctx, actor, id)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.prepareOriginalRestore(ctx, actor, id)</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return s.prepareOriginalRestore(ctx, actor, id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=11006:12833 -->
````go
func (s *Trash) prepareRestore(ctx context.Context, actor Actor, id uint64, input dto.TrashAction) (Actor, MutationBody, error) {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, err
	}
	space, err := s.spaceAuthority(ctx, actor, spaceID, "RESTORE_DOCUMENT", id)
	if err != nil {
		return actor, nil, err
	}
	access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, err
	}
	if receipt, err := repository.TrashRestoreOriginalReceipt(ctx, s.db, input.OperationId); err == nil {
		if err := domain.ValidateTrashRestoreReceipt(receipt, input.OperationId, actor.ID, id); err != nil {
			return actor, nil, err
		}
		return s.prepareOriginalRestore(ctx, actor, id)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return actor, nil, err
	}
	if err = domain.ValidateTrashRestore(facts, space.Space, access.KB, input.ExpectedControlRevision); err != nil {
		return actor, nil, err
	}
	aclHighWater, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+domain.TextID(id))
	if err != nil {
		return actor, nil, err
	}
	publicationHighWater, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+domain.TextID(id))
	if err != nil {
		return actor, nil, err
	}
	next, err := domain.PlanTrashRestore(facts, aclHighWater, publicationHighWater)
	if err != nil {
		return actor, nil, err
	}
	// 新epoch仅在原Mutation事务内预锁，随资格/状态复核失败一起回滚。
	// Gate高水位在预锁前若又前进，标准LockSources会拒绝本次动作，不降低它。
	actor.Sources, err = trashSources(actor, space, access, next.ACL, next.Publication)
	if err != nil {
		return actor, nil, err
	}
````

## 第 17 步：Trash.prepareRestore（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 323 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>d.PublicationEpoch != before.PublicationEpoch &#124;&#124; d.VisibilityRevision != before.VisibilityRevision &#124;&#124; current.Sequence != facts.Sequence</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;TRASH_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=12833:14188 -->
````go
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, kb, err := s.validate(ctx, tx, actor, space, access)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateTrashRestore(current, space.Space, kb, input.ExpectedControlRevision); err != nil {
			return nil, err
		}
		before, d := facts.Document, current.Document
		if d.PublicationEpoch != before.PublicationEpoch || d.VisibilityRevision != before.VisibilityRevision || current.Sequence != facts.Sequence {
			return nil, domain.Violation(domain.StateConflict, "TRASH_DOCUMENT_CHANGED")
		}
		if err := repository.RestoreTrashDocument(ctx, tx, id, input.ExpectedControlRevision, next); err != nil {
			return nil, err
		}
		payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: input.OperationId, Type: "DOCUMENT_UNPUBLISHED", Document: id, Sequence: next.Sequence, PublicationEpoch: next.Publication, OccurredAt: time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		// 恢复的不可见权威摘要和metadata、权限关闭、原号receipt共同提交，未发送成功不冒称消费。
		if err = repository.InsertIndexEvent(ctx, tx, id, next.Sequence, input.OperationId, "DOCUMENT_UNPUBLISHED", payload); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

````

## 第 18 步：Trash.restorePresentation

插入位置：第 17 步代码之后，紧接追加；完成稿约第 351 行。

本步声明：`func (s *Trash) restorePresentation(actor Actor, id uint64) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Sources</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, tx, id, true)</code>；<code>domain.TextID(id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, value map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		current, err := repository.TrashDocumentRow(ctx, tx, id, true) 		if err != nil { 			 …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, value map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		current, err := repository.TrashDocumentRow(ctx, tx, id, true) 		if err != ni …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>source.ScopeKey == &quot;doc-acl:&quot;+domain.TextID(id)</code>。
- 在本段的匿名函数内：检查 <code>source.ScopeKey == &quot;publication:&quot;+domain.TextID(id)</code>。
- 在本段的匿名函数内：检查 <code>!foundACL &#124;&#124; !foundPublication</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;TRASH_RESTORE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, actor.Proof.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=14188:15481 -->
````go
func (s *Trash) restorePresentation(actor Actor, id uint64) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, nil, err
		}
		acl, err := domain.CanonicalID(current.Document.AclRevision)
		if err != nil {
			return nil, nil, err
		}
		publication, err := domain.CanonicalID(current.Document.PublicationEpoch)
		if err != nil {
			return nil, nil, err
		}
		foundACL, foundPublication := false, false
		for _, source := range actor.Sources {
			if source.ScopeKey == "doc-acl:"+domain.TextID(id) {
				foundACL = source.Epoch == acl
			}
			if source.ScopeKey == "publication:"+domain.TextID(id) {
				foundPublication = source.Epoch == publication
			}
		}
		if !foundACL || !foundPublication {
			// receipt可能由并发原请求刚刚提交。本次不得把预想的next gate留在
			// SQL；明确失败使外层回滚，下次原POST走标准Replay的固定结果。
			return nil, nil, domain.Violation(domain.StateConflict, "TRASH_RESTORE_SOURCE_CHANGED")
		}
		// 最小操作结果只依赖原发起人的当前身份，不把管理数据夹入receipt输出。
		return value, actor.Proof.Sources, nil
	}
}

````

## 第 19 步：Trash.prepareOriginalRestore

插入位置：第 18 步代码之后，紧接追加；完成稿约第 384 行。

本步声明：`func (s *Trash) prepareOriginalRestore(ctx context.Context, actor Actor, id uint64) (Actor, MutationBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>s.documentAuthority(ctx, s.db, actor, space, facts, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, err</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code>；<code>s.documentAuthority(ctx, s.db, actor, space, facts, false)</code>；<code>actor.Sources</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.spaceAuthority(ctx, actor, spaceID, &quot;RESTORE_DOCUMENT&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		// 标准Mutation若发现回执已被清理必须失败，不能把ACTIVE文档假装新恢复成功。 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		// 标准Mutation若发现回执已被清理必须失败，不能把ACTIVE文档假装新恢复成功。 		return nil, domain.Violation(domain.TemporarilyUnavailable …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=15481:16879 -->
````go
func (s *Trash) prepareOriginalRestore(ctx context.Context, actor Actor, id uint64) (Actor, MutationBody, error) {
	// 回执确认之后再取当前事实，避免并发首次提交把前面的TRASHED快照变成
	// 已过期来源；回放不计算next，也不提前开放一个尚未发生的恢复代次。
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, err
	}
	space, err := s.spaceAuthority(ctx, actor, spaceID, "RESTORE_DOCUMENT", id)
	if err != nil {
		return actor, nil, err
	}
	access, err := s.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, err
	}
	acl, err := domain.CanonicalID(facts.Document.AclRevision)
	if err != nil {
		return actor, nil, err
	}
	publication, err := domain.CanonicalID(facts.Document.PublicationEpoch)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources, err = trashSources(actor, space, access, acl, publication)
	if err != nil {
		return actor, nil, err
	}
	return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
		// 标准Mutation若发现回执已被清理必须失败，不能把ACTIVE文档假装新恢复成功。
		return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/trash_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
