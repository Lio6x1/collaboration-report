# 分步编写 `services/account/internal/logic/account/organization_service.go`

先按 [A03 · 组织、邀请申请、成员变更与组织关闭](../lessons/account/03-organizations.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/065-organization-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。

组织资格必须与变更处于同一事务；跨服务职责交接和输出停止不能仅靠本地成员状态猜测完成。 本人当前成员事实从AccessOrganization直接取得；展示页、搜索结果和授权判断因此不会互相替代。

写之前先按这条执行路线理解各段的连接：

1. Applications.Decide 先核申请人和管理员资格、原修订，再录入成员或拒绝申请；邀请只创建申请，不直接加入。
2. Invitations.Share 重新核当前管理资格后解密原邀请；目录读取捕获实际组织成员来源。
3. OrganizationTransitions 将角色变更/离开登记为持久关闭；移交前检查目标当前成员。
4. OrganizationClosures 按原绑定取得Fence ACK，再逐文档清理；Ready检查恢复绑定。
5. AccessOrganization核组织与本人ACTIVE关系后设置member.UserID=actor和org.CurrentMember，并继续捕获org-member/管理员/所有者原来源。
6. OpenMembers每次重新准入；MemberDirectory.List向仓储转发可选q，limit+1探测后截断，逐项捕获org-member和identity来源。

本文件需要守住的边界：

- 负责人离开必须先交接，Doc职责未知不能放行。
- 跨服务调用回复丢失应重读原号和固定计划，不能重新选择关闭范围。
- 当前本人不在20条成员页中不等于没有管理权；CurrentMember也只是本次读出的事实，后续写操作仍要锁当前权限。
- 成员筛选后的页面不能用于判断整个组织只有一人、完整管理员人数或是否允许关闭。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/organization_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/organization_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/organization_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/organization_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:455 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：Applications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type Applications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>repository</code> | <code>*repository.Applications</code> | 使用 <code>repository.Applications</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>organizations</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>invitations</code> | <code>*repository.Invitations</code> | 使用 <code>repository.Invitations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 2 bytes=455:683 -->
````go
type Applications struct {
	db            *sql.DB
	mutations     *Mutations
	repository    *repository.Applications
	organizations *repository.Organizations
	invitations   *repository.Invitations
	capture       CaptureSource
}

````

## 第 3 步：NewApplications

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewApplications(db *sql.DB, mutations *Mutations, repo *repository.Applications, organizations *repository.Organizations, invitations *repository.Invitations, capture CaptureSource) *Applications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db:            db</code> |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>mutations:     mutations</code> |
| <code>repo</code> | <code>*repository.Applications</code> | 使用 <code>repository.Applications</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>repository:    repo</code> |
| <code>organizations</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>organizations: organizations</code> |
| <code>invitations</code> | <code>*repository.Invitations</code> | 使用 <code>repository.Invitations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>invitations:   invitations</code> |
| <code>capture</code> | <code>CaptureSource</code> | 在此声明中的实际使用：<code>capture:       capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Applications</code> | 实际返回表达式：<code>&amp;Applications{ 		db:            db, 		mutations:     mutations, 		repository:    repo, 		organizations: organizations, 		invitations:   invitations, 		capture:       capture, 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Applications{ 		db:            db, 		mutations:     mutations, 		repository:    repo, 		organizations: organizations, 		invitations:   invitations, 		capture:       capture, 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=683:1075 -->
````go
func NewApplications(db *sql.DB, mutations *Mutations, repo *repository.Applications, organizations *repository.Organizations, invitations *repository.Invitations, capture CaptureSource) *Applications {
	return &Applications{
		db:            db,
		mutations:     mutations,
		repository:    repo,
		organizations: organizations,
		invitations:   invitations,
		capture:       capture,
	}
}

````

## 第 4 步：Applications.create

插入位置：第 3 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func (s *Applications) create(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.organizations.LockMemberState(ctx, tx, organization, actor)</code>；<code>s.repository.Pending(ctx, tx, organization, actor)</code>；<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.organizations.LockMemberState(ctx, tx, organization, actor)</code>；<code>s.repository.Pending(ctx, tx, organization, actor)</code>；<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>s.organizations.LockMemberState(ctx, tx, organization, actor)</code>；<code>s.repository.Pending(ctx, tx, organization, actor)</code>；<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.organizations.LockMemberState(ctx, tx, organization, actor)</code>；<code>s.repository.Pending(ctx, tx, organization, actor)</code>；<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |
| <code>reason</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |
| <code>invitation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Insert(ctx, tx, organization, actor, reason, invitation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>existing</code>；<code>fmt.Sprint(id)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;ALREADY_MEMBER_OR_TRANSITIONING&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>state == &quot;ACTIVE&quot; &#124;&#124; state == &quot;REVOKING&quot;</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;ALREADY_MEMBER_OR_TRANSITIONING&quot;}</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return existing, nil</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 此处通过 <code>return fmt.Sprint(id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1075:1804 -->
````go
func (s *Applications) create(ctx context.Context, tx *sql.Tx, organization, actor, reason, invitation string) (string, error) {
	state, err := s.organizations.LockMemberState(ctx, tx, organization, actor)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if state == "ACTIVE" || state == "REVOKING" {
		return "", domain.RuleViolation{Kind: domain.StateConflict, Code: "ALREADY_MEMBER_OR_TRANSITIONING"}
	}
	existing, err := s.repository.Pending(ctx, tx, organization, actor)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id, err := s.repository.Insert(ctx, tx, organization, actor, reason, invitation)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(id), nil
}

````

## 第 5 步：Applications.Apply

插入位置：第 4 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func (s *Applications) Apply(ctx context.Context, mutation domain.Mutation, organization string, input dto.ApplicationCreate) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := repository.ReadOrganization(ctx, tx, organizati …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := repository.ReadOrganization(ctx, tx, organizati …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ReadOrganization(ctx, tx, organization, true)</code>；<code>s.create(ctx, tx, organization, session.UserID, input.Reason, &quot;&quot;)</code> |
| <code>input</code> | <code>dto.ApplicationCreate</code> | 在此声明中的实际使用：<code>input.Reason</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := repository.ReadOrganization(ctx, tx, organizati …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := repository.ReadOrganization(ctx, tx, organizati …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := repository.ReadOrganization(ctx, tx, org …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; org.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ORGANIZATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1804:2525 -->
````go
func (s *Applications) Apply(ctx context.Context, mutation domain.Mutation, organization string, input dto.ApplicationCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_APPLY"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := repository.ReadOrganization(ctx, tx, organization, true)
		if err != nil || org.Lifecycle != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
		}
		id, err := s.create(ctx, tx, organization, session.UserID, input.Reason, "")
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

````

## 第 6 步：Applications.AcceptInvitation

插入位置：第 5 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func (s *Applications) AcceptInvitation(ctx context.Context, mutation domain.Mutation, hash []byte, input dto.InvitationAccept) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.invitations.IdentityByToken(ctx, tx, has …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.invitations.IdentityByToken(ctx, tx, has …</code>（长表达式，按本步完整代码继续） |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.invitations.IdentityByToken(ctx, tx, hash)</code> |
| <code>input</code> | <code>dto.InvitationAccept</code> | 在此声明中的实际使用：<code>input.Reason</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.invitations.IdentityByToken(ctx, tx, has …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.invitations.IdentityByToken(ctx, tx, has …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.invitations.IdentityByToken(ctx,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;INVITATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; org.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ORGANIZATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; invitation.State != &quot;OPEN&quot; &#124;&#124; !invitation.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;INVITATION_NOT_CURRENT&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2525:3818 -->
````go
func (s *Applications) AcceptInvitation(ctx context.Context, mutation domain.Mutation, hash []byte, input dto.InvitationAccept) (json.RawMessage, error) {
	mutation.Kind = "ORG_LINK_APPLY"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		identity, err := s.invitations.IdentityByToken(ctx, tx, hash)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		org, err := repository.ReadOrganization(ctx, tx, identity.OrganizationID, true)
		if err != nil || org.Lifecycle != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
		}
		// 有效性固定在申请提交事务；之后关闭/过期不能抹去原审批依据。
		invitation, err := s.invitations.LockValidity(ctx, tx, identity.ID)
		if err != nil || invitation.State != "OPEN" || !invitation.Expires.After(time.Now()) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "INVITATION_NOT_CURRENT"}
		}
		id, err := s.create(ctx, tx, identity.OrganizationID, session.UserID, input.Reason, identity.ID)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

````

## 第 7 步：Applications.Decide

插入位置：第 6 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func (s *Applications) Decide(ctx context.Context, mutation domain.Mutation, id string, input dto.ApplicationReview) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.repository.Identity(ctx, tx, id) 		if er …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.repository.Identity(ctx, tx, id) 		if er …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Identity(ctx, tx, id)</code>；<code>s.repository.LockStatus(ctx, tx, id)</code>；<code>s.repository.Decide(ctx, tx, id, session.UserID, next)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.ApplicationReview</code> | 在此声明中的实际使用：<code>input.Decision</code>；<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.repository.Identity(ctx, tx, id) 		if er …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.repository.Identity(ctx, tx, id) 		if er …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		identity, err := s.repository.Identity(ctx, tx, id)  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;APPLICATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Decision == &quot;ACCEPT&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;APPLICATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>application.State != &quot;PENDING&quot; &#124;&#124; application.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;APPLICATION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Decision == &quot;ACCEPT&quot;</code>。
- 在本段的匿名函数内：检查 <code>applicantError != nil &#124;&#124; applicantState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;APPLICANT_NOT_ACTIVE&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>memberState == &quot;REVOKING&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;MEMBER_TRANSITION_IN_PROGRESS&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>memberState != &quot;ACTIVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=3818:6198 -->
````go
func (s *Applications) Decide(ctx context.Context, mutation domain.Mutation, id string, input dto.ApplicationReview) (json.RawMessage, error) {
	mutation.Kind = "ORG_APPLICATION_DECIDE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		identity, err := s.repository.Identity(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "APPLICATION_NOT_FOUND"}
		}
		var applicantState string
		var applicantError error
		if input.Decision == "ACCEPT" {
			// 注销以目标user锁固定其全部组织；新增成员也先持同一目标锁，再锁组织。
			// 外键仅保证用户存在，普通快照读ACTIVE不能阻止其等待后已变为REVOKING。
			applicantState, applicantError = s.organizations.LockUserState(ctx, tx, identity.ApplicantID)
		}
		if _, err := AccessOrganization(ctx, tx, identity.OrganizationID, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture); err != nil {
			return nil, err
		}
		application, err := s.repository.LockStatus(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "APPLICATION_NOT_FOUND"}
		}
		if application.State != "PENDING" || application.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "APPLICATION_CHANGED"}
		}
		next := "REJECTED"
		if input.Decision == "ACCEPT" {
			if applicantError != nil || applicantState != "ACTIVE" {
				return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "APPLICANT_NOT_ACTIVE"}
			}
			// 成员状态与账号状态分开读取；不存在的成员行不能沿用账号ACTIVE值。
			memberState, err := s.organizations.LockMemberState(ctx, tx, identity.OrganizationID, identity.ApplicantID)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if memberState == "REVOKING" {
				return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "MEMBER_TRANSITION_IN_PROGRESS"}
			}
			if memberState != "ACTIVE" {
				if err := s.repository.AdmitMember(ctx, tx, identity.OrganizationID, identity.ApplicantID); err != nil {
					return nil, err
				}
			}
			next = "ACCEPTED"
		}
		// 本次只核验当前管理员及目标状态，不重查已固定的提交时邀请有效性。
````

## 第 8 步：Applications.Decide（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 146 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6198:6449 -->
````go
		if err := s.repository.Decide(ctx, tx, id, session.UserID, next); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// application_directory.go：保留原实现，按业务合并文件。

````

## 第 9 步：ApplicationDirectory

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`type ApplicationDirectory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>string</code> | 当前列出申请的账号。 |
| <code>organization</code> | <code>string</code> | 申请列表绑定的组织，防止游标跨组织使用。 |
| <code>revision</code> | <code>string</code> | 列表绑定的组织修订值，后续分页继续核验同一目录上下文。 |
| <code>repository</code> | <code>*repository.Applications</code> | 使用 <code>repository.Applications</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |

<!-- write-step: 9 bytes=6449:6589 -->
````go
type ApplicationDirectory struct {
	actor        string
	organization string
	revision     string
	repository   *repository.Applications
}

````

## 第 10 步：Applications.OpenDirectory

插入位置：第 9 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func (s *Applications) OpenDirectory(ctx context.Context, actor, organization string) (*ApplicationDirectory, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>actor: actor</code>；<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>organization: organization</code>；<code>organization != &quot;&quot;</code>；<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ApplicationDirectory</code> | 实际返回表达式：<code>nil</code>；<code>directory</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>organization != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return directory, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6589:7078 -->
````go
func (s *Applications) OpenDirectory(ctx context.Context, actor, organization string) (*ApplicationDirectory, error) {
	directory := &ApplicationDirectory{actor: actor, organization: organization, repository: s.repository}
	if organization != "" {
		org, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)
		if err != nil {
			return nil, err
		}
		directory.revision = org.MemberRevision
	}
	return directory, nil
}

````

## 第 11 步：ApplicationDirectory.Revision

插入位置：第 10 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func (d *ApplicationDirectory) Revision() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>d.revision</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return d.revision</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7078:7150 -->
````go
func (d *ApplicationDirectory) Revision() string { return d.revision }

````

## 第 12 步：ApplicationDirectory.List

插入位置：第 11 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func (d *ApplicationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationApplication, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>d.repository.List(ctx, d.actor, d.organization, last, limit+1)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>d.repository.List(ctx, d.actor, d.organization, last, limit+1)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit+1</code>；<code>len(items) &gt; limit</code>；<code>items = items&#91;:limit&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationApplication</code> | 使用 <code>domain.OrganizationApplication</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>more</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, false, err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 此处通过 <code>return items, more, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7150:7561 -->
````go
func (d *ApplicationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationApplication, bool, error) {
	items, err := d.repository.List(ctx, d.actor, d.organization, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// invitation.go：保留原实现，按业务合并文件。

````

## 第 13 步：Invitations

插入位置：第 12 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`type Invitations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>repository</code> | <code>*repository.Invitations</code> | 使用 <code>repository.Invitations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>codec</code> | <code>*domain.InvitationCodec</code> | 使用 <code>domain.InvitationCodec</code> 类型；其定义处的职责：InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 13 bytes=7561:7731 -->
````go
type Invitations struct {
	db         *sql.DB
	mutations  *Mutations
	repository *repository.Invitations
	codec      *domain.InvitationCodec
	capture    CaptureSource
}

````

## 第 14 步：NewInvitations

插入位置：第 13 步代码之后，紧接追加；完成稿约第 198 行。

本步声明：`func NewInvitations(db *sql.DB, mutations *Mutations, repo *repository.Invitations, codec *domain.InvitationCodec, capture CaptureSource) *Invitations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>mutations: mutations</code> |
| <code>repo</code> | <code>*repository.Invitations</code> | 使用 <code>repository.Invitations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>codec</code> | <code>*domain.InvitationCodec</code> | 使用 <code>domain.InvitationCodec</code> 类型；其定义处的职责：InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。<br>在此声明中的实际使用：<code>codec: codec</code> |
| <code>capture</code> | <code>CaptureSource</code> | 在此声明中的实际使用：<code>capture: capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Invitations</code> | 实际返回表达式：<code>&amp;Invitations{db: db, mutations: mutations, repository: repo, codec: codec, capture: capture}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Invitations{db: db, mutations: mutations, repository: repo, codec: codec, capture: capture}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7731:7988 -->
````go
func NewInvitations(db *sql.DB, mutations *Mutations, repo *repository.Invitations, codec *domain.InvitationCodec, capture CaptureSource) *Invitations {
	return &Invitations{db: db, mutations: mutations, repository: repo, codec: codec, capture: capture}
}

````

## 第 15 步：Invitations.Create

插入位置：第 14 步代码之后，紧接追加；完成稿约第 202 行。

本步声明：`func (s *Invitations) Create(ctx context.Context, mutation domain.Mutation, organization string, input dto.InvitationCreate) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)</code>；<code>s.codec.Encrypt(token, organization)</code>；<code>s.repository.Insert(ctx, tx, organization, session.UserID, hash, encrypted, expires)</code> |
| <code>input</code> | <code>dto.InvitationCreate</code> | 在此声明中的实际使用：<code>input.ExpectedMemberRevision</code>；<code>input.ExpiresInHours</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>org.MemberRevision != input.ExpectedMemberRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, fmt.Sprint(id)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7988:9246 -->
````go
func (s *Invitations) Create(ctx context.Context, mutation domain.Mutation, organization string, input dto.InvitationCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_INVITE_CREATE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.MemberRevision != input.ExpectedMemberRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		token, hash, err := domain.RandomToken()
		if err != nil {
			return nil, err
		}
		encrypted, err := s.codec.Encrypt(token, organization)
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(time.Duration(input.ExpiresInHours) * time.Hour)
		id, err := s.repository.Insert(ctx, tx, organization, session.UserID, hash, encrypted, expires)
		if err != nil {
			return nil, err
		}
		// receipt只保存实体身份；分享令牌的密文独立保存，审计与操作结果不包含令牌。
		return OperationResult(input.OperationId, "SUCCEEDED", fmt.Sprint(id)), nil
	})
}

````

## 第 16 步：Invitations.Share

插入位置：第 15 步代码之后，紧接追加；完成稿约第 230 行。

本步声明：`func (s *Invitations) Share(ctx context.Context, actor, organization, id string) (domain.InvitationShare, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code>；<code>s.repository.Share(ctx, id, organization)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code>；<code>s.repository.Share(ctx, id, organization)</code>；<code>s.codec.Decrypt(encrypted, organization)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Share(ctx, id, organization)</code>；<code>ID: id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.InvitationShare</code> | 使用 <code>domain.InvitationShare</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>domain.InvitationShare{}</code>；<code>domain.InvitationShare{ID: id, Token: token, Expires: expires}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;INVITATION_RESULT_UNAVAILABLE&quot;}</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;INVITATION_KEY_UNAVAILABLE&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.InvitationShare{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;INVITATION_RESULT_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;INVITATION_KEY_UNAVAILABLE&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return domain.InvitationShare{ID: id, Token: token, Expires: expires}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9246:10052 -->
````go
func (s *Invitations) Share(ctx context.Context, actor, organization, id string) (domain.InvitationShare, error) {
	if _, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture); err != nil {
		return domain.InvitationShare{}, err
	}
	encrypted, expires, err := s.repository.Share(ctx, id, organization)
	if err != nil {
		return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "INVITATION_RESULT_UNAVAILABLE"}
	}
	token, err := s.codec.Decrypt(encrypted, organization)
	if err != nil {
		return domain.InvitationShare{}, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "INVITATION_KEY_UNAVAILABLE"}
	}
	return domain.InvitationShare{ID: id, Token: token, Expires: expires}, nil
}

````

## 第 17 步：Invitations.Cancel

插入位置：第 16 步代码之后，紧接追加；完成稿约第 245 行。

本步声明：`func (s *Invitations) Cancel(ctx context.Context, mutation domain.Mutation, id string, input dto.VersionedAction) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := s.repository.Organization(ctx, tx, id) …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := s.repository.Organization(ctx, tx, id) …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Organization(ctx, tx, id)</code>；<code>s.repository.LockStatus(ctx, tx, id)</code>；<code>s.repository.Close(ctx, tx, id)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.VersionedAction</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := s.repository.Organization(ctx, tx, id) …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := s.repository.Organization(ctx, tx, id) …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := s.repository.Organization(ctx,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;INVITATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;INVITATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>invitation.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>invitation.State == &quot;OPEN&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=10052:11312 -->
````go
func (s *Invitations) Cancel(ctx context.Context, mutation domain.Mutation, id string, input dto.VersionedAction) (json.RawMessage, error) {
	mutation.Kind = "ORG_INVITE_CANCEL"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := s.repository.Organization(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		if _, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture); err != nil {
			return nil, err
		}
		invitation, err := s.repository.LockStatus(ctx, tx, id)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "INVITATION_NOT_FOUND"}
		}
		if invitation.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if invitation.State == "OPEN" {
			if err := s.repository.Close(ctx, tx, id); err != nil {
				return nil, err
			}
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// invitation_directory.go：保留原实现，按业务合并文件。

````

## 第 18 步：InvitationDirectory

插入位置：第 17 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`type InvitationDirectory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>organization</code> | <code>domain.Organization</code> | 使用 <code>domain.Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。 |
| <code>repository</code> | <code>*repository.Invitations</code> | 使用 <code>repository.Invitations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |

<!-- write-step: 18 bytes=11312:11421 -->
````go
type InvitationDirectory struct {
	organization domain.Organization
	repository   *repository.Invitations
}

````

## 第 19 步：Invitations.OpenDirectory

插入位置：第 18 步代码之后，紧接追加；完成稿约第 278 行。

本步声明：`func (s *Invitations) OpenDirectory(ctx context.Context, actor, organization string) (*InvitationDirectory, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)</code>；<code>organization: org</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*InvitationDirectory</code> | 实际返回表达式：<code>nil</code>；<code>&amp;InvitationDirectory{organization: org, repository: s.repository}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;InvitationDirectory{organization: org, repository: s.repository}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=11421:11786 -->
````go
func (s *Invitations) OpenDirectory(ctx context.Context, actor, organization string) (*InvitationDirectory, error) {
	org, err := AccessOrganization(ctx, s.db, organization, actor, OrganizationAccess{Role: OrganizationAdministrator}, s.capture)
	if err != nil {
		return nil, err
	}
	return &InvitationDirectory{organization: org, repository: s.repository}, nil
}

````

## 第 20 步：InvitationDirectory.Revision

插入位置：第 19 步代码之后，紧接追加；完成稿约第 286 行。

本步声明：`func (d *InvitationDirectory) Revision() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>d.organization.MemberRevision</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return d.organization.MemberRevision</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11786:11876 -->
````go
func (d *InvitationDirectory) Revision() string { return d.organization.MemberRevision }

````

## 第 21 步：InvitationDirectory.List

插入位置：第 20 步代码之后，紧接追加；完成稿约第 288 行。

本步声明：`func (d *InvitationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationInvitation, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>d.repository.List(ctx, d.organization.ID, last, limit+1)</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>d.repository.List(ctx, d.organization.ID, last, limit+1)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit+1</code>；<code>len(items) &gt; limit</code>；<code>items = items&#91;:limit&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationInvitation</code> | 使用 <code>domain.OrganizationInvitation</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>more</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, false, err</code>。当前函数在这里结束。
- 检查 <code>items&#91;index&#93;.State == &quot;OPEN&quot; &amp;&amp; !items&#91;index&#93;.Expires.After(time.Now())</code>。
- 检查 <code>more</code>。
- 此处通过 <code>return items, more, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=11876:12435 -->
````go
func (d *InvitationDirectory) List(ctx context.Context, last string, limit int) ([]domain.OrganizationInvitation, bool, error) {
	items, err := d.repository.List(ctx, d.organization.ID, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	for index := range items {
		if items[index].State == "OPEN" && !items[index].Expires.After(time.Now()) {
			items[index].State = "EXPIRED"
		}
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 22 步：OrganizationClosePeer

插入位置：第 21 步代码之后，紧接追加；完成稿约第 307 行。

本步声明：`type OrganizationClosePeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Plan</code> | <code>(context.Context, platform.OrganizationClosureBinding) error</code> | 按原组织关闭绑定让Doc固定留存/清理计划；错误时不推进Account成功状态。 |
| <code>Page</code> | <code>(context.Context, platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)</code> | 参数为ctx和固定计划分页请求，返回本轮文档及继续游标。 |
| <code>CleanDocument</code> | <code>(context.Context, platform.OrganizationClosureBinding, string) error</code> | 参数为ctx、原组织关闭绑定、文档编号；只清理固定计划中的对象。 |
| <code>Complete</code> | <code>(context.Context, platform.OrganizationClosureBinding) error</code> | 请求Doc核验全部文档及关系处理完成，保持原操作绑定。 |

<!-- write-step: 22 bytes=12435:12802 -->
````go
type OrganizationClosePeer interface {
	Plan(context.Context, platform.OrganizationClosureBinding) error
	Page(context.Context, platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)
	CleanDocument(context.Context, platform.OrganizationClosureBinding, string) error
	Complete(context.Context, platform.OrganizationClosureBinding) error
}

````

## 第 23 步：OrganizationClosures

插入位置：第 22 步代码之后，紧接追加；完成稿约第 314 行。

本步声明：`type OrganizationClosures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>fence</code> | <code>AccountCloseFencePeer</code> | 使用 <code>AccountCloseFencePeer</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。 |
| <code>peer</code> | <code>OrganizationClosePeer</code> | 组织关闭的Doc参与者；与账号注销使用各自协议绑定，不能混用操作种类。 |

<!-- write-step: 23 bytes=12802:12947 -->
````go
type OrganizationClosures struct {
	db        *sql.DB
	mutations *Mutations
	fence     AccountCloseFencePeer
	peer      OrganizationClosePeer
}

````

## 第 24 步：NewOrganizationClosures

插入位置：第 23 步代码之后，紧接追加；完成稿约第 321 行。

本步声明：`func NewOrganizationClosures(db *sql.DB, mutations *Mutations, fence AccountCloseFencePeer, peer OrganizationClosePeer) *OrganizationClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>mutations: mutations</code> |
| <code>fence</code> | <code>AccountCloseFencePeer</code> | 使用 <code>AccountCloseFencePeer</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>fence: fence</code> |
| <code>peer</code> | <code>OrganizationClosePeer</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationClosures</code> | 实际返回表达式：<code>&amp;OrganizationClosures{db: db, mutations: mutations, fence: fence, peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationClosures{db: db, mutations: mutations, fence: fence, peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=12947:13180 -->
````go
func NewOrganizationClosures(db *sql.DB, mutations *Mutations, fence AccountCloseFencePeer, peer OrganizationClosePeer) *OrganizationClosures {
	return &OrganizationClosures{db: db, mutations: mutations, fence: fence, peer: peer}
}

````

## 第 25 步：OrganizationClosures.Begin

插入位置：第 24 步代码之后，紧接追加；完成稿约第 325 行。

本步声明：`func (s *OrganizationClosures) Begin(ctx context.Context, mutation domain.Mutation, organization, expected string) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		if _, err := repository.BeginOrganizationClose(ctx, tx, mut …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		if _, err := repository.BeginOrganizationClose(ctx, tx, mut …</code>（长表达式，按本步完整代码继续）；<code>mutation.OperationID</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidEntityID(organization)</code>；<code>repository.BeginOrganizationClose(ctx, tx, mutation.OperationID, organization, session.UserID, expected)</code>；<code>OperationResult(mutation.OperationID, &quot;PREPARING&quot;, organization)</code> |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidEntityID(expected)</code>；<code>repository.BeginOrganizationClose(ctx, tx, mutation.OperationID, organization, session.UserID, expected)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		if _, err := repository.BeginOrganizationClose(ctx, tx, mut …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		if _, err := repository.BeginOrganizationClose(ctx, tx, mut …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ORGANIZATION_CLOSE_INPUT_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidEntityID(organization) &#124;&#124; !domain.ValidEntityID(expected)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;ORGANIZATION_CLOSE_INPUT_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		if _, err := repository.BeginOrganizationClose(ctx,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(mutation.OperationID, &quot;PREPARING&quot;, organization), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=13180:13907 -->
````go
func (s *OrganizationClosures) Begin(ctx context.Context, mutation domain.Mutation, organization, expected string) (json.RawMessage, error) {
	if !domain.ValidEntityID(organization) || !domain.ValidEntityID(expected) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "ORGANIZATION_CLOSE_INPUT_INVALID"}
	}
	mutation.Kind = "ORG_CLOSE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		if _, err := repository.BeginOrganizationClose(ctx, tx, mutation.OperationID, organization, session.UserID, expected); err != nil {
			return nil, err
		}
		return OperationResult(mutation.OperationID, "PREPARING", organization), nil
	})
}

````

## 第 26 步：OrganizationClosures.Ready

插入位置：第 25 步代码之后，紧接追加；完成稿约第 338 行。

本步声明：`func (s *OrganizationClosures) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.OrganizationCloseReady(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED&quot;)</code>；<code>repository.OrganizationCloseReady(ctx, s.db)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.fence == nil &#124;&#124; s.peer == nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return repository.OrganizationCloseReady(ctx, s.db)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=13907:14134 -->
````go
func (s *OrganizationClosures) Ready(ctx context.Context) error {
	if s.fence == nil || s.peer == nil {
		return errors.New("ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED")
	}
	return repository.OrganizationCloseReady(ctx, s.db)
}

````

## 第 27 步：OrganizationClosures.lockOriginal

插入位置：第 26 步代码之后，紧接追加；完成稿约第 345 行。

lockOriginal沿user→receipt→organization→member→transition顺序；后台不要求原会话仍存活。
关闭期间账号仍可改变凭据，也可发起注销，恢复仅依据原组织来源和固定绑定。

本步声明：`func (s *OrganizationClosures) lockOriginal(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding) (repository.OrganizationCloseRecord, domain.OrganizationClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, b.User)</code>；<code>closures.Receipt(ctx, tx, b.Operation)</code>；<code>repository.ReadOrganization(ctx, tx, strconv.FormatUint(b.Organization, 10), true)</code>；<code>repository.ReadOrganizationMember(ctx, tx, org.ID, strconv.FormatUint(b.User, 10), true)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>closures.LockUser(ctx, tx, b.User)</code>；<code>closures.Receipt(ctx, tx, b.Operation)</code>；<code>repository.ReadOrganization(ctx, tx, strconv.FormatUint(b.Organization, 10), true)</code>；<code>repository.ReadOrganizationMember(ctx, tx, org.ID, strconv.FormatUint(b.User, 10), true)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Valid</code>；<code>b.User</code>；<code>b.Operation</code>；<code>b.Organization</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.OrganizationCloseRecord</code> | 使用 <code>repository.OrganizationCloseRecord</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>实际返回表达式：<code>empty</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_TERMINAL_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!b.Valid()</code>。条件成立时执行 <code>return empty, plan, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, plan, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, plan, err</code>。当前函数在这里结束。
- 检查 <code>plan.Binding != b &#124;&#124; record.Actor != b.User &#124;&#124; record.Organization != b.Organization &#124;&#124; receipt.User != b.User &#124;&#124; receipt.Kind != &quot;ORG_CLOSE&quot; &#124;&#124; org.OwnerID != strconv.FormatUint(b …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return record, plan, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>org.Lifecycle != &quot;CLOSED&quot; &#124;&#124; member.State != &quot;CLOSED&quot; &#124;&#124; receipt.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return record, plan, errors.New(&quot;ORGANIZATION_CLOSE_TERMINAL_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>org.Lifecycle != &quot;CLOSING&quot; &#124;&#124; member.State != &quot;REVOKING&quot; &#124;&#124; receipt.State != &quot;PREPARING&quot; &#124;&#124; (record.State != &quot;FENCING&quot; &amp;&amp; record.State != &quot;APPLIED&quot;)</code>。条件成立时执行 <code>return record, plan, errors.New(&quot;ORGANIZATION_CLOSE_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return record, plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=14134:16391 -->
````go
// lockOriginal沿user→receipt→organization→member→transition顺序；后台不要求原会话仍存活。
// 关闭期间账号仍可改变凭据，也可发起注销，恢复仅依据原组织来源和固定绑定。
func (s *OrganizationClosures) lockOriginal(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding) (repository.OrganizationCloseRecord, domain.OrganizationClosePlan, error) {
	var empty repository.OrganizationCloseRecord
	var plan domain.OrganizationClosePlan
	if !b.Valid() {
		return empty, plan, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	closures := repository.NewAccountClosures(s.db)
	if _, err := closures.LockUser(ctx, tx, b.User); err != nil {
		return empty, plan, err
	}
	receipt, err := closures.Receipt(ctx, tx, b.Operation)
	if err != nil {
		return empty, plan, err
	}
	org, err := repository.ReadOrganization(ctx, tx, strconv.FormatUint(b.Organization, 10), true)
	if err != nil {
		return empty, plan, err
	}
	member, err := repository.ReadOrganizationMember(ctx, tx, org.ID, strconv.FormatUint(b.User, 10), true)
	if err != nil {
		return empty, plan, err
	}
	record, err := repository.ReadOrganizationClose(ctx, tx, b.Operation, true)
	if err != nil {
		return record, plan, err
	}
	plan, err = domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
		return record, plan, err
	}
	if plan.Binding != b || record.Actor != b.User || record.Organization != b.Organization || receipt.User != b.User || receipt.Kind != "ORG_CLOSE" || org.OwnerID != strconv.FormatUint(b.User, 10) || org.Revision != strconv.FormatUint(b.Revision+1, 10) || org.MemberRevision != strconv.FormatUint(b.MemberRevision+1, 10) || member.Epoch != b.Epoch+1 {
		return record, plan, errors.New("ORGANIZATION_CLOSE_BINDING_CHANGED")
	}
	if record.State == "SUCCEEDED" {
		if org.Lifecycle != "CLOSED" || member.State != "CLOSED" || receipt.State != "SUCCEEDED" {
			return record, plan, errors.New("ORGANIZATION_CLOSE_TERMINAL_CHANGED")
		}
	} else if org.Lifecycle != "CLOSING" || member.State != "REVOKING" || receipt.State != "PREPARING" || (record.State != "FENCING" && record.State != "APPLIED") {
		return record, plan, errors.New("ORGANIZATION_CLOSE_STATE_CHANGED")
	}
	return record, plan, nil
}

````

## 第 28 步：OrganizationClosures.Confirmed

插入位置：第 27 步代码之后，紧接追加；完成稿约第 390 行。

本步声明：`func (s *OrganizationClosures) Confirmed(ctx context.Context, b platform.OrganizationClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockOriginal(ctx, tx, b)</code>；<code>repository.OrganizationCloseProof(ctx, tx, b, true)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>s.lockOriginal(ctx, tx, b)</code>；<code>repository.OrganizationCloseProof(ctx, tx, b, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>repository.OrganizationCloseProof(ctx, tx, b, true)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.OrganizationCloseProof(ctx, tx, b, true)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;APPLIED&quot; &amp;&amp; record.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return repository.OrganizationCloseProof(ctx, tx, b, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=16391:16898 -->
````go
func (s *OrganizationClosures) Confirmed(ctx context.Context, b platform.OrganizationClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, _, err := s.lockOriginal(ctx, tx, b)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record.State != "APPLIED" && record.State != "SUCCEEDED" {
		return false, nil
	}
	return repository.OrganizationCloseProof(ctx, tx, b, true)
}

````

## 第 29 步：OrganizationClosures.RecoverOne

插入位置：第 28 步代码之后，紧接追加；完成稿约第 409 行。

本步声明：`func (s *OrganizationClosures) RecoverOne(parent context.Context, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ClaimOrganizationClose(ctx, tx, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。

<!-- write-step: 29 bytes=16898:17417 -->
````go
func (s *OrganizationClosures) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	task, err := repository.ClaimOrganizationClose(ctx, tx, instance)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if tx.Commit() != nil {
		return
	}
	if err = s.advance(ctx, task); err != nil {
		repository.RetryOrganizationClose(ctx, s.db, task, "ORGANIZATION_CLOSE_ADVANCE_UNKNOWN")
	}
}

````

## 第 30 步：OrganizationClosures.advance

插入位置：第 29 步代码之后，紧接追加；完成稿约第 429 行。

一次领取最多清理一份文档；网络不持SQL锁，SQL推进前重新核验原号及租约generation。

本步声明：`func (s *OrganizationClosures) advance(ctx context.Context, task domain.AccountCloseFenceTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReadOrganizationClose(ctx, s.db, b.Operation, false)</code>；<code>s.fence.FenceAccess(ctx, request)</code>；<code>s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) { 			if record.State != &quot;FENCING&quot; { 				return  …</code>（长表达式，按本步完整代码继续）；<code>repository.OrganizationCloseProof(ctx, tx, b, false)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) { 			if record.State != &quot;FENCING&quot; { 				return  …</code>（长表达式，按本步完整代码继续）；<code>s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) { 		if record.State != &quot;APPLIED&quot; &#124;&#124; current.Pla …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_TASK_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_PLAN_CHANGED&quot;)</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_FENCE_UNCONFIRMED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; initial.Binding.Operation != task.Operation</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_TASK_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; plan.Binding != b</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;FENCING&quot;</code>。
- 检查 <code>err != nil &#124;&#124; !platform.MatchesBootstrapAck(request, ack, &quot;document&quot;)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_FENCE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) { 			if record.State != &quot;FENCING&quot; { 				 …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>record.State != &quot;FENCING&quot;</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_FENCE_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !valid</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_FENCE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return false, repository.SaveOrganizationCloseFence(ctx, tx, b, ack.ResultDigest)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>record.State != &quot;APPLIED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_RECOVERY_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!page.Matches(request)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 30 bytes=17417:19369 -->
````go
// 一次领取最多清理一份文档；网络不持SQL锁，SQL推进前重新核验原号及租约generation。
func (s *OrganizationClosures) advance(ctx context.Context, task domain.AccountCloseFenceTask) error {
	initial, err := domain.DecodeOrganizationClosePlan(task.Payload)
	if err != nil || initial.Binding.Operation != task.Operation {
		return errors.New("ORGANIZATION_CLOSE_TASK_INVALID")
	}
	b := initial.Binding
	record, err := repository.ReadOrganizationClose(ctx, s.db, b.Operation, false)
	if err != nil {
		return err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil || plan.Binding != b {
		return errors.New("ORGANIZATION_CLOSE_PLAN_CHANGED")
	}
	if record.State == "FENCING" {
		request := repository.OrganizationCloseFenceRequest(b)
		ack, err := s.fence.FenceAccess(ctx, request)
		if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
			return errors.New("ORGANIZATION_CLOSE_FENCE_UNCONFIRMED")
		}
		return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) {
			if record.State != "FENCING" {
				return false, errors.New("ORGANIZATION_CLOSE_FENCE_STATE_CHANGED")
			}
			valid, err := repository.OrganizationCloseProof(ctx, tx, b, false)
			if err != nil || !valid {
				return false, errors.New("ORGANIZATION_CLOSE_FENCE_BINDING_CHANGED")
			}
			return false, repository.SaveOrganizationCloseFence(ctx, tx, b, ack.ResultDigest)
		})
	}
	if record.State != "APPLIED" {
		return errors.New("ORGANIZATION_CLOSE_RECOVERY_STATE_INVALID")
	}
	if err := s.peer.Plan(ctx, b); err != nil {
		return err
	}
	request := platform.OrganizationClosurePageRequest{Binding: b, PlanDigest: plan.PlanDigest, AfterDocument: plan.AfterDocument}
	page, err := s.peer.Page(ctx, request)
	if err != nil {
		return err
	}
	if !page.Matches(request) {
		return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
	}
````

## 第 31 步：OrganizationClosures.advance（接着实现）

插入位置：第 30 步代码之后，紧接追加；完成稿约第 475 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>len(page.Documents) &gt; 0</code>。
- 检查 <code>plan.AfterDocument != &quot;&quot;</code>。
- 检查 <code>parseErr != nil &#124;&#124; id &lt;= after &#124;&#124; !domain.ValidEntityID(item.Document)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!page.Done &#124;&#124; page.NextAfter != plan.AfterDocument</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !proof.Matches(request) &#124;&#124; proof.PlanDigest != page.PlanDigest &#124;&#124; proof.State != &quot;SUCCEEDED&quot; &#124;&#124; !proof.Done &#124;&#124; len(proof.Documents) != 0</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_DOC_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) { 		if record.State != &quot;APPLIED&quot; &#124;&#124; curr …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>record.State != &quot;APPLIED&quot; &#124;&#124; current.PlanDigest != request.PlanDigest &#124;&#124; current.AfterDocument != request.AfterDocument</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_CURSOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !confirmed</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_ACK_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>terminal</code>。

<!-- write-step: 31 bytes=19369:21297 -->
````go
	terminal := false
	if len(page.Documents) > 0 {
		item := page.Documents[0]
		id, parseErr := strconv.ParseUint(item.Document, 10, 64)
		var after uint64
		if plan.AfterDocument != "" {
			after, _ = strconv.ParseUint(plan.AfterDocument, 10, 64)
		}
		if parseErr != nil || id <= after || !domain.ValidEntityID(item.Document) {
			return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
		}
		if err = s.peer.CleanDocument(ctx, b, item.Document); err != nil {
			return err
		}
		plan.AfterDocument = item.Document
	} else {
		if !page.Done || page.NextAfter != plan.AfterDocument {
			return errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
		}
		if err = s.peer.Complete(ctx, b); err != nil {
			return err
		}
		// Complete的成功回复还需原分页读取SUCCEEDED，丢失回复继续原号查询。
		proof, err := s.peer.Page(ctx, request)
		if err != nil || !proof.Matches(request) || proof.PlanDigest != page.PlanDigest || proof.State != "SUCCEEDED" || !proof.Done || len(proof.Documents) != 0 {
			return errors.New("ORGANIZATION_CLOSE_DOC_UNCONFIRMED")
		}
		terminal = true
	}
	plan.PlanDigest = page.PlanDigest
	return s.save(ctx, task, b, func(tx *sql.Tx, record repository.OrganizationCloseRecord, current domain.OrganizationClosePlan) (bool, error) {
		if record.State != "APPLIED" || current.PlanDigest != request.PlanDigest || current.AfterDocument != request.AfterDocument {
			return false, errors.New("ORGANIZATION_CLOSE_CURSOR_CHANGED")
		}
		confirmed, err := repository.OrganizationCloseProof(ctx, tx, b, true)
		if err != nil || !confirmed {
			return false, errors.New("ORGANIZATION_CLOSE_ACK_REQUIRED")
		}
		if err = repository.SaveOrganizationCloseCursor(ctx, tx, plan); err != nil {
			return false, err
		}
		if terminal {
			raw, _ := json.Marshal(OperationResult(b.Operation, "SUCCEEDED", strconv.FormatUint(b.Organization, 10)))
			err = repository.CompleteOrganizationClose(ctx, tx, b, raw)
		}
````

## 第 32 步：OrganizationClosures.advance（接着实现）

插入位置：第 31 步代码之后，紧接追加；完成稿约第 520 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return terminal, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=21297:21327 -->
````go
		return terminal, err
	})
}

````

## 第 33 步：OrganizationClosures.save

插入位置：第 32 步代码之后，紧接追加；完成稿约第 524 行。

本步声明：`func (s *OrganizationClosures) save(ctx context.Context, task domain.AccountCloseFenceTask, b platform.OrganizationClosureBinding, action func(*sql.Tx, repository.OrganizationCloseRecord, domain.OrganizationClosePlan) (bool, error)) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockOriginal(ctx, tx, b)</code>；<code>repository.ValidateOrganizationCloseTask(ctx, tx, task)</code>；<code>repository.FinishOrganizationCloseTask(ctx, tx, task, terminal)</code> |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>repository.ValidateOrganizationCloseTask(ctx, tx, task)</code>；<code>repository.FinishOrganizationCloseTask(ctx, tx, task, terminal)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>s.lockOriginal(ctx, tx, b)</code> |
| <code>action</code> | <code>func(*sql.Tx, repository.OrganizationCloseRecord, domain.OrganizationClosePlan) (bool, error)</code> | 在此声明中的实际使用：<code>action(tx, record, plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=21327:22185 -->
````go
func (s *OrganizationClosures) save(ctx context.Context, task domain.AccountCloseFenceTask, b platform.OrganizationClosureBinding, action func(*sql.Tx, repository.OrganizationCloseRecord, domain.OrganizationClosePlan) (bool, error)) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, plan, err := s.lockOriginal(ctx, tx, b)
	if err != nil {
		return err
	}
	if err = repository.ValidateOrganizationCloseTask(ctx, tx, task); err != nil {
		return err
	}
	terminal, err := action(tx, record, plan)
	if err != nil {
		return err
	}
	if err = repository.FinishOrganizationCloseTask(ctx, tx, task, terminal); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_COMMIT_UNKNOWN")
	}
	return nil
}

// organization_read.go：保留原实现，按业务合并文件。

````

## 第 34 步：CaptureSource

插入位置：第 33 步代码之后，紧接追加；完成稿约第 552 行。

本步声明：`type CaptureSource func(context.Context, string, uint64)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>string</code> | 本次输出使用的权限来源键，例如组织成员来源。 |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 该来源当次核验的代次，输出关闭按键与代次追踪。 |

<!-- write-step: 34 bytes=22185:22243 -->
````go
type CaptureSource func(context.Context, string, uint64)

````

## 第 35 步：OrganizationRole

插入位置：第 34 步代码之后，紧接追加；完成稿约第 554 行。

本步声明：`type OrganizationRole uint8`。

<!-- write-step: 35 bytes=22243:22272 -->
````go
type OrganizationRole uint8

````

## 第 36 步：OrganizationMember、OrganizationAdministrator、OrganizationOwner

插入位置：第 35 步代码之后，紧接追加；完成稿约第 556 行。

本步声明：`const (`。

<!-- write-step: 36 bytes=22272:22373 -->
````go
const (
	OrganizationMember OrganizationRole = iota
	OrganizationAdministrator
	OrganizationOwner
)

````

## 第 37 步：OrganizationAccess

插入位置：第 36 步代码之后，紧接追加；完成稿约第 562 行。

角色要求与行锁是两个独立维度，用命名策略避免调用点出现连续的 true/false。

本步声明：`type OrganizationAccess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Role</code> | <code>OrganizationRole</code> | 此次操作所需的组织角色级别：成员、管理员或所有者。 |
| <code>Lock</code> | <code>bool</code> | true使用事务锁定组织/成员事实，供角色或生命周期修改；普通读取不能替代修改前锁内核验。 |

<!-- write-step: 37 bytes=22373:22551 -->
````go
// 角色要求与行锁是两个独立维度，用命名策略避免调用点出现连续的 true/false。
type OrganizationAccess struct {
	Role OrganizationRole
	Lock bool
}

````

## 第 38 步：AccessOrganization

插入位置：第 37 步代码之后，紧接追加；完成稿约第 568 行。

本步声明：`func AccessOrganization(ctx context.Context, query repository.Queryer, id, actor string, access OrganizationAccess, capture CaptureSource) (domain.Organization, error)`。

**这一段为什么这样写：** 在组织详情和成员目录的共同入口取得当前组织与本人关系，并为本次输出绑定真实权限来源。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次请求上下文，供来源捕获和SQL取消。<br>在此声明中的实际使用：<code>repository.ReadOrganization(ctx, query, id, access.Lock)</code>；<code>repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)</code>；<code>capture(ctx, &quot;org-member:&quot;+id+&quot;:&quot;+actor, member.Epoch)</code>；<code>capture(ctx, &quot;org-owner:&quot;+id+&quot;:&quot;+actor, member.Epoch)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>query</code> | <code>repository.Queryer</code> | 原数据库或原事务Queryer；access.Lock=true时所有后续变更必须沿用同一事务。<br>在此声明中的实际使用：<code>repository.ReadOrganization(ctx, query, id, access.Lock)</code>；<code>repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)</code> |
| <code>id</code> | <code>string</code> | 规范组织ID。<br>在此声明中的实际使用：<code>repository.ReadOrganization(ctx, query, id, access.Lock)</code>；<code>repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)</code>；<code>&quot;org-member:&quot;+id</code>；<code>&quot;org-owner:&quot;+id</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>string</code> | 当前会话真实用户ID，不能来自成员搜索或请求体。<br>在此声明中的实际使用：<code>repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)</code>；<code>member.UserID = actor</code>；<code>&quot;org-member:&quot;+id+&quot;:&quot;+actor</code>；<code>org.OwnerID == actor</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>access</code> | <code>OrganizationAccess</code> | Role决定MEMBER/ADMIN/OWNER门槛，Lock独立决定是否加行锁。<br>在此声明中的实际使用：<code>access.Lock</code>；<code>access.Role</code> |
| <code>capture</code> | <code>CaptureSource</code> | 登记本次实际使用的来源与原epoch，供输出停止协议重验。<br>在此声明中的实际使用：<code>capture(ctx, &quot;org-member:&quot;+id+&quot;:&quot;+actor, member.Epoch)</code>；<code>capture(ctx, &quot;org-owner:&quot;+id+&quot;:&quot;+actor, member.Epoch)</code>；<code>capture(ctx, &quot;org-admin:&quot;+id+&quot;:&quot;+actor, member.Epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Organization</code> | 已核验ACTIVE组织，CurrentMember指向当前操作者成员关系。<br>实际返回表达式：<code>org</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 组织/成员不可访问、管理资格不足或数据库失败。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ORGANIZATION_NOT_FOUND&quot;}</code>；<code>domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;ORGANIZATION_OWNER_REQUIRED&quot;}</code>；<code>domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;ORGANIZATION_ADMIN_REQUIRED&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; org.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ORGANIZATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; member.State != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ORGANIZATION_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 检查 <code>owner &#124;&#124; (admin &amp;&amp; org.OwnerID == actor)</code>。
- 检查 <code>admin</code>。
- 检查 <code>owner &amp;&amp; org.OwnerID != actor</code>。条件成立时执行 <code>return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;ORGANIZATION_OWNER_REQUIRED&quot;}</code>。当前函数在这里结束。
- 检查 <code>admin &amp;&amp; org.OwnerID != actor &amp;&amp; member.Role != &quot;ADMIN&quot;</code>。条件成立时执行 <code>return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;ORGANIZATION_ADMIN_REQUIRED&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return org, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=22551:23877 -->
````go
func AccessOrganization(ctx context.Context, query repository.Queryer, id, actor string, access OrganizationAccess, capture CaptureSource) (domain.Organization, error) {
	org, err := repository.ReadOrganization(ctx, query, id, access.Lock)
	if err != nil || org.Lifecycle != "ACTIVE" {
		return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
	}
	member, err := repository.ReadOrganizationMember(ctx, query, id, actor, access.Lock)
	if err != nil || member.State != "ACTIVE" {
		return org, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ORGANIZATION_NOT_FOUND"}
	}
	member.UserID = actor
	org.CurrentMember = &member
	owner, admin := access.Role == OrganizationOwner, access.Role == OrganizationAdministrator
	capture(ctx, "org-member:"+id+":"+actor, member.Epoch)
	if owner || (admin && org.OwnerID == actor) {
		capture(ctx, "org-owner:"+id+":"+actor, member.Epoch)
	} else if admin {
		capture(ctx, "org-admin:"+id+":"+actor, member.Epoch)
	}
	if owner && org.OwnerID != actor {
		return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "ORGANIZATION_OWNER_REQUIRED"}
	}
	if admin && org.OwnerID != actor && member.Role != "ADMIN" {
		return org, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "ORGANIZATION_ADMIN_REQUIRED"}
	}
	return org, nil
}

````

## 第 39 步：Organizations

插入位置：第 38 步代码之后，紧接追加；完成稿约第 595 行。

本步声明：`type Organizations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 39 bytes=23877:23992 -->
````go
type Organizations struct {
	db         *sql.DB
	repository *repository.Organizations
	capture    CaptureSource
}

````

## 第 40 步：NewOrganizations

插入位置：第 39 步代码之后，紧接追加；完成稿约第 601 行。

本步声明：`func NewOrganizations(db *sql.DB, repo *repository.Organizations, capture CaptureSource) *Organizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>capture</code> | <code>CaptureSource</code> | 在此声明中的实际使用：<code>capture: capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Organizations</code> | 实际返回表达式：<code>&amp;Organizations{db: db, repository: repo, capture: capture}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Organizations{db: db, repository: repo, capture: capture}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=23992:24168 -->
````go
func NewOrganizations(db *sql.DB, repo *repository.Organizations, capture CaptureSource) *Organizations {
	return &Organizations{db: db, repository: repo, capture: capture}
}

````

## 第 41 步：Organizations.Get

插入位置：第 40 步代码之后，紧接追加；完成稿约第 605 行。

本步声明：`func (s *Organizations) Get(ctx context.Context, actor, id string) (domain.Organization, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Organization</code> | 使用 <code>domain.Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。<br>由 <code>AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=24168:24375 -->
````go
func (s *Organizations) Get(ctx context.Context, actor, id string) (domain.Organization, error) {
	return AccessOrganization(ctx, s.db, id, actor, OrganizationAccess{Role: OrganizationMember}, s.capture)
}

````

## 第 42 步：Organizations.List

插入位置：第 41 步代码之后，紧接追加；完成稿约第 609 行。

本步声明：`func (s *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.Organization, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.List(ctx, actor, last, limit+1)</code>；<code>s.capture(ctx, &quot;org-member:&quot;+entry.Organization.ID+&quot;:&quot;+actor, entry.MemberEpoch)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.List(ctx, actor, last, limit+1)</code>；<code>&quot;org-member:&quot;+entry.Organization.ID+&quot;:&quot;+actor</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.List(ctx, actor, last, limit+1)</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit+1</code>；<code>len(items) &gt; limit</code>；<code>items = items&#91;:limit&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.Organization</code> | 使用 <code>domain.Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>more</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, false, err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 此处通过 <code>return items, more, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=24375:24914 -->
````go
func (s *Organizations) List(ctx context.Context, actor, last string, limit int) ([]domain.Organization, bool, error) {
	entries, err := s.repository.List(ctx, actor, last, limit+1)
	if err != nil {
		return nil, false, err
	}
	items := make([]domain.Organization, 0, len(entries))
	for _, entry := range entries {
		s.capture(ctx, "org-member:"+entry.Organization.ID+":"+actor, entry.MemberEpoch)
		items = append(items, entry.Organization)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

````

## 第 43 步：MemberDirectory

插入位置：第 42 步代码之后，紧接追加；完成稿约第 626 行。

MemberDirectory只由当前资格核验产生，不能从浏览器自报的组织对象构造。
它在同一请求里提供绑定member_revision的分页，最终输出仍核验所捕获的来源代次。

本步声明：`type MemberDirectory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>organization</code> | <code>domain.Organization</code> | 使用 <code>domain.Organization</code> 类型；其定义处的职责：区分组织控制版本、成员集合版本与当前操作者的成员快照；页码改变只能改变展示集合，不能改变本人身份。 |
| <code>repository</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 43 bytes=24914:25261 -->
````go
// MemberDirectory只由当前资格核验产生，不能从浏览器自报的组织对象构造。
// 它在同一请求里提供绑定member_revision的分页，最终输出仍核验所捕获的来源代次。
type MemberDirectory struct {
	organization domain.Organization
	repository   *repository.Organizations
	capture      CaptureSource
}

````

## 第 44 步：Organizations.OpenMembers

插入位置：第 43 步代码之后，紧接追加；完成稿约第 634 行。

本步声明：`func (s *Organizations) OpenMembers(ctx context.Context, actor, id string) (*MemberDirectory, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Get(ctx, actor, id)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Get(ctx, actor, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Get(ctx, actor, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*MemberDirectory</code> | 使用 <code>MemberDirectory</code> 类型；其定义处的职责：MemberDirectory只由当前资格核验产生，不能从浏览器自报的组织对象构造。 它在同一请求里提供绑定member_revision的分页，最终输出仍核验所捕获的来源代次。<br>实际返回表达式：<code>nil</code>；<code>&amp;MemberDirectory{organization: org, repository: s.repository, capture: s.capture}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;MemberDirectory{organization: org, repository: s.repository, capture: s.capture}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=25261:25535 -->
````go
func (s *Organizations) OpenMembers(ctx context.Context, actor, id string) (*MemberDirectory, error) {
	org, err := s.Get(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	return &MemberDirectory{organization: org, repository: s.repository, capture: s.capture}, nil
}

````

## 第 45 步：MemberDirectory.Revision

插入位置：第 44 步代码之后，紧接追加；完成稿约第 642 行。

本步声明：`func (d *MemberDirectory) Revision() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>d.organization.MemberRevision</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return d.organization.MemberRevision</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 45 bytes=25535:25621 -->
````go
func (d *MemberDirectory) Revision() string { return d.organization.MemberRevision }

````

## 第 46 步：MemberDirectory.List

插入位置：第 45 步代码之后，紧接追加；完成稿约第 644 行。

本步声明：`func (d *MemberDirectory) List(ctx context.Context, last string, limit int, query ...string) ([]domain.OrganizationMember, bool, error)`。

**这一段为什么这样写：** 在已经取得的当前组织资格和MemberRevision下读取一个筛选页；组页不扫描全目录，也不为找到本人额外续页。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 本次受控读取上下文。<br>在此声明中的实际使用：<code>d.repository.Members(ctx, d.organization.ID, last, limit+1, query...)</code>；<code>d.capture(ctx, &quot;org-member:&quot;+d.organization.ID+&quot;:&quot;+member.UserID, member.Epoch)</code>；<code>d.capture(ctx, &quot;identity:&quot;+member.UserID, member.IdentityEpoch)</code> |
| <code>last</code> | <code>string</code> | 解析出的原user_id扫描边界。<br>在此声明中的实际使用：<code>d.repository.Members(ctx, d.organization.ID, last, limit+1, query...)</code> |
| <code>limit</code> | <code>int</code> | 公开展示页大小；向仓储传limit+1。<br>在此声明中的实际使用：<code>limit+1</code>；<code>len(items) &gt; limit</code>；<code>items = items&#91;:limit&#93;</code> |
| <code>query</code> | <code>...string</code> | 可选已规范化q，原样展开给仓储；无参数兼容原目录调用。<br>在此声明中的实际使用：<code>d.repository.Members(ctx, d.organization.ID, last, limit+1, query...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.OrganizationMember</code> | 剔除探测项后的成员页。<br>实际返回表达式：<code>nil</code>；<code>items</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 是否还有后续匹配项；不表示总人数。<br>实际返回表达式：<code>false</code>；<code>more</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 查询或来源捕获依赖失败。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, false, err</code>。当前函数在这里结束。
- 检查 <code>more</code>。
- 此处通过 <code>return items, more, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 46 bytes=25621:26242 -->
````go
func (d *MemberDirectory) List(ctx context.Context, last string, limit int, query ...string) ([]domain.OrganizationMember, bool, error) {
	items, err := d.repository.Members(ctx, d.organization.ID, last, limit+1, query...)
	if err != nil {
		return nil, false, err
	}
	for _, member := range items {
		d.capture(ctx, "org-member:"+d.organization.ID+":"+member.UserID, member.Epoch)
		d.capture(ctx, "identity:"+member.UserID, member.IdentityEpoch)
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// organization_transition.go：保留原实现，按业务合并文件。

````

## 第 47 步：DeparturePeer

插入位置：第 46 步代码之后，紧接追加；完成稿约第 662 行。

本步声明：`type DeparturePeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Check</code> | <code>(context.Context, string, string, string, uint64) error</code> | 参数顺序为ctx、组织编号、成员编号、退出操作号、成员来源代次；向Doc核验管理交接资格，失败不得退出。 |

<!-- write-step: 47 bytes=26242:26338 -->
````go
type DeparturePeer interface {
	Check(context.Context, string, string, string, uint64) error
}

````

## 第 48 步：OrganizationTransitions

插入位置：第 47 步代码之后，紧接追加；完成稿约第 666 行。

本步声明：`type OrganizationTransitions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>repository</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>departure</code> | <code>DeparturePeer</code> | 退出/移除前的Doc资格核验接口，确认该成员在知识库的管理职责可交接。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 48 bytes=26338:26492 -->
````go
type OrganizationTransitions struct {
	mutations  *Mutations
	repository *repository.Organizations
	departure  DeparturePeer
	capture    CaptureSource
}

````

## 第 49 步：NewOrganizationTransitions

插入位置：第 48 步代码之后，紧接追加；完成稿约第 673 行。

本步声明：`func NewOrganizationTransitions(mutations *Mutations, repo *repository.Organizations, departure DeparturePeer, capture CaptureSource) *OrganizationTransitions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>mutations: mutations</code> |
| <code>repo</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>departure</code> | <code>DeparturePeer</code> | 在此声明中的实际使用：<code>departure: departure</code> |
| <code>capture</code> | <code>CaptureSource</code> | 在此声明中的实际使用：<code>capture: capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationTransitions</code> | 实际返回表达式：<code>&amp;OrganizationTransitions{mutations: mutations, repository: repo, departure: departure, capture: capture}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationTransitions{mutations: mutations, repository: repo, departure: departure, capture: capture}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 49 bytes=26492:26769 -->
````go
func NewOrganizationTransitions(mutations *Mutations, repo *repository.Organizations, departure DeparturePeer, capture CaptureSource) *OrganizationTransitions {
	return &OrganizationTransitions{mutations: mutations, repository: repo, departure: departure, capture: capture}
}

````

## 第 50 步：OrganizationTransitions.ChangeMember

插入位置：第 49 步代码之后，紧接追加；完成稿约第 677 行。

本步声明：`func (s *OrganizationTransitions) ChangeMember(ctx context.Context, mutation domain.Mutation, organization, user, expected, kind, nextRole string) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续）；<code>mutation.OperationID</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: organizationChangeRole(kind), Lock: true}, s.capture)</code>；<code>s.repository.LockTransitionMember(ctx, tx, organization, user)</code>；<code>s.departure.Check(ctx, organization, user, mutation.OperationID, member.Epoch)</code>；<code>OperationResult(mutation.OperationID, &quot;SUCCEEDED&quot;, organization)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>org.OwnerID == user</code>；<code>s.repository.LockTransitionMember(ctx, tx, organization, user)</code>；<code>s.departure.Check(ctx, organization, user, mutation.OperationID, member.Epoch)</code>；<code>UserID:         user</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>expected</code> | <code>string</code> | 在此声明中的实际使用：<code>member.Revision != expected</code>；<code>s.repository.InsertMemberTransition(ctx, tx, mutation.OperationID, organization, session.UserID, user, transition, expected, plan)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>mutation.Kind = kind</code>；<code>organizationChangeRole(kind)</code>；<code>kind == &quot;ORG_LEAVE&quot;</code>；<code>kind == &quot;ORG_REMOVE&quot;</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>nextRole</code> | <code>string</code> | 在此声明中的实际使用：<code>member.Role == nextRole</code>；<code>Role:           nextRole</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization, sessi …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		org, err := AccessOrganization(ctx, tx, organization …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>org.OwnerID == user</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.PermissionDenied, Code: &quot;OWNER_CANNOT_BE_REMOVED_OR_DEMOTED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; member.State != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;TARGET_NOT_MEMBER&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>member.Revision != expected</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>member.Epoch == ^uint64(0)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EPOCH_EXHAUSTED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kind == &quot;ORG_LEAVE&quot; &amp;&amp; member.Role == &quot;ADMIN&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;ADMIN_RETIREMENT_REQUIRED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kind == &quot;ORG_LEAVE&quot; &#124;&#124; kind == &quot;ORG_REMOVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>kind == &quot;ORG_ROLE&quot; &amp;&amp; member.Role == nextRole</code>。条件成立时执行 <code>return OperationResult(mutation.OperationID, &quot;SUCCEEDED&quot;, organization), nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 50 bytes=26769:28996 -->
````go
func (s *OrganizationTransitions) ChangeMember(ctx context.Context, mutation domain.Mutation, organization, user, expected, kind, nextRole string) (json.RawMessage, error) {
	mutation.Kind = kind
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: organizationChangeRole(kind), Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.OwnerID == user {
			return nil, domain.RuleViolation{Kind: domain.PermissionDenied, Code: "OWNER_CANNOT_BE_REMOVED_OR_DEMOTED"}
		}
		member, err := s.repository.LockTransitionMember(ctx, tx, organization, user)
		if err != nil || member.State != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		if member.Revision != expected {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if member.Epoch == ^uint64(0) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "EPOCH_EXHAUSTED"}
		}
		if kind == "ORG_LEAVE" && member.Role == "ADMIN" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "ADMIN_RETIREMENT_REQUIRED"}
		}
		if kind == "ORG_LEAVE" || kind == "ORG_REMOVE" {
			if err := s.departure.Check(ctx, organization, user, mutation.OperationID, member.Epoch); err != nil {
				return nil, err
			}
		}
		if kind == "ORG_ROLE" && member.Role == nextRole {
			return OperationResult(mutation.OperationID, "SUCCEEDED", organization), nil
		}
		payload := domain.SessionClosure{
			Kind:           kind,
			OrganizationID: organization,
			UserID:         user,
			Role:           nextRole,
			Scope:          "org-member:" + organization + ":" + user,
			Epoch:          strconv.FormatUint(member.Epoch, 10),
		}
		// 先暂停旧来源再登记固定关闭计划；完成后普通成员身份以新代次恢复。
		if err := s.repository.PrepareMemberTransition(ctx, tx, organization, user); err != nil {
			return nil, err
		}
		if err := EnqueueSourceClosure(ctx, tx, mutation.OperationID, session.UserID, payload); err != nil {
			return nil, err
		}
````

## 第 51 步：OrganizationTransitions.ChangeMember（接着实现）

插入位置：第 50 步代码之后，紧接追加；完成稿约第 723 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>kind == &quot;ORG_LEAVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(mutation.OperationID, &quot;PREPARING&quot;, organization), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 51 bytes=28996:29453 -->
````go
		transition := "ROLE_CHANGE"
		if kind == "ORG_LEAVE" {
			transition = "LEAVE"
		}
		plan, _ := json.Marshal(payload)
		if err := s.repository.InsertMemberTransition(ctx, tx, mutation.OperationID, organization, session.UserID, user, transition, expected, plan); err != nil {
			return nil, err
		}
		return OperationResult(mutation.OperationID, "PREPARING", organization), nil
	})
}

// organization_write.go：保留原实现，按业务合并文件。

````

## 第 52 步：OrganizationActions

插入位置：第 51 步代码之后，紧接追加；完成稿约第 737 行。

本步声明：`type OrganizationActions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>repository</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。 |
| <code>capture</code> | <code>CaptureSource</code> | 记录本次输出依赖的权限来源键及代次（ctx,key,epoch），以便权限关闭时停止旧输出；不是创建新权限。 |

<!-- write-step: 52 bytes=29453:29577 -->
````go
type OrganizationActions struct {
	mutations  *Mutations
	repository *repository.Organizations
	capture    CaptureSource
}

````

## 第 53 步：NewOrganizationActions

插入位置：第 52 步代码之后，紧接追加；完成稿约第 743 行。

本步声明：`func NewOrganizationActions(mutations *Mutations, repo *repository.Organizations, capture CaptureSource) *OrganizationActions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mutations</code> | <code>*Mutations</code> | 使用 <code>Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>mutations: mutations</code> |
| <code>repo</code> | <code>*repository.Organizations</code> | 使用 <code>repository.Organizations</code> 类型；其定义处的职责：持久化组织申请、邀请、成员/所有者变更以及组织关闭恢复任务。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>capture</code> | <code>CaptureSource</code> | 在此声明中的实际使用：<code>capture: capture</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationActions</code> | 实际返回表达式：<code>&amp;OrganizationActions{mutations: mutations, repository: repo, capture: capture}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationActions{mutations: mutations, repository: repo, capture: capture}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 53 bytes=29577:29795 -->
````go
func NewOrganizationActions(mutations *Mutations, repo *repository.Organizations, capture CaptureSource) *OrganizationActions {
	return &OrganizationActions{mutations: mutations, repository: repo, capture: capture}
}

````

## 第 54 步：OrganizationActions.Create

插入位置：第 53 步代码之后，紧接追加；完成稿约第 747 行。

本步声明：`func (s *OrganizationActions) Create(ctx context.Context, mutation domain.Mutation, input dto.NamedCreate) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		id, err := s.repository.Insert(ctx, tx, input.Name, session …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		id, err := s.repository.Insert(ctx, tx, input.Name, session …</code>（长表达式，按本步完整代码继续） |
| <code>input</code> | <code>dto.NamedCreate</code> | 在此声明中的实际使用：<code>input.Name</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		id, err := s.repository.Insert(ctx, tx, input.Name, session …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		id, err := s.repository.Insert(ctx, tx, input.Name, session …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		id, err := s.repository.Insert(ctx, tx, input.Name,  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, fmt.Sprint(id)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 54 bytes=29795:30282 -->
````go
func (s *OrganizationActions) Create(ctx context.Context, mutation domain.Mutation, input dto.NamedCreate) (json.RawMessage, error) {
	mutation.Kind = "ORG_CREATE"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		id, err := s.repository.Insert(ctx, tx, input.Name, session.UserID)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", fmt.Sprint(id)), nil
	})
}

````

## 第 55 步：OrganizationActions.Edit

插入位置：第 54 步代码之后，紧接追加；完成稿约第 758 行。

本步声明：`func (s *OrganizationActions) Edit(ctx context.Context, mutation domain.Mutation, id string, input dto.OrganizationEdit) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)</code>；<code>s.repository.Rename(ctx, tx, id, input.Name)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id)</code> |
| <code>input</code> | <code>dto.OrganizationEdit</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.Name</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>organization.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 55 bytes=30282:31089 -->
````go
func (s *OrganizationActions) Edit(ctx context.Context, mutation domain.Mutation, id string, input dto.OrganizationEdit) (json.RawMessage, error) {
	mutation.Kind = "ORG_EDIT"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if organization.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if err := s.repository.Rename(ctx, tx, id, input.Name); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

````

## 第 56 步：OrganizationActions.ChangeCreationGrant

插入位置：第 55 步代码之后，紧接追加；完成稿约第 775 行。

本步声明：`func (s *OrganizationActions) ChangeCreationGrant(ctx context.Context, mutation domain.Mutation, id string, input dto.CreationGrant) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)</code>；<code>s.repository.LockMemberState(ctx, tx, id, input.UserId)</code>；<code>s.repository.LockCreationGrant(ctx, tx, id, input.UserId)</code>；<code>OrganizationID: id</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.CreationGrant</code> | 在此声明中的实际使用：<code>input.ExpectedRevision</code>；<code>input.UserId</code>；<code>input.Allowed</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, sessio …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		organization, err := AccessOrganization(ctx, tx, id, …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>organization.MemberRevision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;TARGET_NOT_MEMBER&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.Allowed</code>。
- 在本段的匿名函数内：检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; previousState == &quot;ACTIVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>pending</code>。条件成立时执行 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, id), nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 56 bytes=31089:33129 -->
````go
func (s *OrganizationActions) ChangeCreationGrant(ctx context.Context, mutation domain.Mutation, id string, input dto.CreationGrant) (json.RawMessage, error) {
	mutation.Kind = "ORG_CREATION_GRANT"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		organization, err := AccessOrganization(ctx, tx, id, session.UserID, OrganizationAccess{Role: OrganizationAdministrator, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if organization.MemberRevision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		state, err := s.repository.LockMemberState(ctx, tx, id, input.UserId)
		if err != nil || state != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		state = "CLOSED"
		pending := false
		if input.Allowed {
			state = "ACTIVE"
		} else {
			// 撤销创建资格仍须阻止旧准入提交建库；门闩、变更和恢复任务一起提交。
			revision, previousState, err := s.repository.LockCreationGrant(ctx, tx, id, input.UserId)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil && previousState == "ACTIVE" {
				payload := domain.SessionClosure{
					Kind:           "ORG_CREATE_REVOKE",
					OrganizationID: id,
					UserID:         input.UserId,
					Scope:          "org-create:" + id + ":" + input.UserId,
					Epoch:          strconv.FormatUint(revision, 10),
				}
				if err := EnqueueSourceClosure(ctx, tx, input.OperationId, session.UserID, payload); err != nil {
					return nil, err
				}
				pending = true
			}
		}
		if err := s.repository.WriteCreationGrant(ctx, tx, id, input.UserId, state); err != nil {
			return nil, err
		}
		if pending {
			return OperationResult(input.OperationId, "PREPARING", id), nil
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}

// owner_transfer.go：保留原实现，按业务合并文件。

````

## 第 57 步：OrganizationTransitions.TransferOwner

插入位置：第 56 步代码之后，紧接追加；完成稿约第 825 行。

本步声明：`func (s *OrganizationTransitions) TransferOwner(ctx context.Context, mutation domain.Mutation, organization string, input dto.OwnerChange) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		var targetState string 		var targetError error 		if input.T …</code>（长表达式，按本步完整代码继续） |
| <code>mutation</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>mutation.Kind</code>；<code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		var targetState string 		var targetError error 		if input.T …</code>（长表达式，按本步完整代码继续） |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationOwner, Lock: true}, s.capture)</code>；<code>OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, organization)</code>；<code>s.repository.LockMemberState(ctx, tx, organization, input.TargetUserId)</code>；<code>s.repository.LockOwnerMember(ctx, tx, organization, session.UserID)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.OwnerChange</code> | 在此声明中的实际使用：<code>input.TargetUserId</code>；<code>input.ExpectedRevision</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		var targetState string 		var targetError error 		if input.T …</code>（长表达式，按本步完整代码继续） 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		var targetState string 		var targetError error 		if input.T …</code>（长表达式，按本步完整代码继续） 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map&#91;string&#93;any, error) { 		var targetState string 		var targetError error 		if  …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>input.TargetUserId != session.UserID</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>org.Revision != input.ExpectedRevision</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;REVISION_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.TargetUserId == session.UserID</code>。条件成立时执行 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, organization), nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>targetError != nil &#124;&#124; targetState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;TARGET_NOT_MEMBER&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;TARGET_NOT_MEMBER&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OWNER_CHANGED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>member.Epoch == ^uint64(0)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;EPOCH_EXHAUSTED&quot;}</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 57 bytes=33129:35446 -->
````go
func (s *OrganizationTransitions) TransferOwner(ctx context.Context, mutation domain.Mutation, organization string, input dto.OwnerChange) (json.RawMessage, error) {
	mutation.Kind = "ORG_OWNER_TRANSFER"
	return s.mutations.Run(ctx, mutation, func(ctx context.Context, tx *sql.Tx, session domain.Session) (map[string]any, error) {
		var targetState string
		var targetError error
		if input.TargetUserId != session.UserID {
			// 与账号注销一致，先锁目标身份再锁组织；注销冻结后不能授予新的组织所有权。
			targetState, targetError = s.repository.LockUserState(ctx, tx, input.TargetUserId)
		}
		org, err := AccessOrganization(ctx, tx, organization, session.UserID, OrganizationAccess{Role: OrganizationOwner, Lock: true}, s.capture)
		if err != nil {
			return nil, err
		}
		if org.Revision != input.ExpectedRevision {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "REVISION_CHANGED"}
		}
		if input.TargetUserId == session.UserID {
			return OperationResult(input.OperationId, "SUCCEEDED", organization), nil
		}
		if targetError != nil || targetState != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		state, err := s.repository.LockMemberState(ctx, tx, organization, input.TargetUserId)
		if err != nil || state != "ACTIVE" {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "TARGET_NOT_MEMBER"}
		}
		member, err := s.repository.LockOwnerMember(ctx, tx, organization, session.UserID)
		if err != nil {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OWNER_CHANGED"}
		}
		if member.Epoch == ^uint64(0) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "EPOCH_EXHAUSTED"}
		}
		payload := domain.SessionClosure{
			Kind:           "ORG_OWNER_TRANSFER",
			OrganizationID: organization,
			UserID:         session.UserID,
			Role:           member.Role,
			Scope:          "org-owner:" + organization + ":" + session.UserID,
			Epoch:          strconv.FormatUint(member.Epoch, 10),
		}
		if err := s.repository.PrepareOwnerTransfer(ctx, tx, organization, session.UserID, input.TargetUserId); err != nil {
			return nil, err
		}
		if err := EnqueueSourceClosure(ctx, tx, input.OperationId, session.UserID, payload); err != nil {
````

## 第 58 步：OrganizationTransitions.TransferOwner（接着实现）

插入位置：第 57 步代码之后，紧接追加；完成稿约第 870 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;PREPARING&quot;, organization), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 58 bytes=35446:35777 -->
````go
			return nil, err
		}
		plan, _ := json.Marshal(payload)
		if err := s.repository.InsertOwnerTransfer(ctx, tx, input.OperationId, organization, session.UserID, input.TargetUserId, input.ExpectedRevision, plan); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", organization), nil
	})
}

````

## 第 59 步：organizationChangeRole

插入位置：第 58 步代码之后，紧接追加；完成稿约第 880 行。

本步声明：`func organizationChangeRole(kind string) OrganizationRole`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>kind</code> | <code>string</code> | 组织修改种类：ORG_LEAVE要求成员，ORG_ROLE要求所有者，其他本入口动作要求管理员。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OrganizationRole</code> | 实际返回表达式：<code>OrganizationMember</code>；<code>OrganizationOwner</code>；<code>OrganizationAdministrator</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return OrganizationMember</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return OrganizationOwner</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return OrganizationAdministrator</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 59 bytes=35777:35994 -->
````go
func organizationChangeRole(kind string) OrganizationRole {
	switch kind {
	case "ORG_LEAVE":
		return OrganizationMember
	case "ORG_ROLE":
		return OrganizationOwner
	default:
		return OrganizationAdministrator
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/organization_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/03-organizations.md)。
