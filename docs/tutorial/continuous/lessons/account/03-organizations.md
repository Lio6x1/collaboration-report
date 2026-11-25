# A03｜创建组织、申请入会，再把降权和关闭写成可靠流程

## 这节课要实现什么

团队需要创建组织，邀请新人或审批加入申请，并管理成员职责。本课实现从组织创建、邀请入会到成员变更、所有者交接和关闭组织的后端流程。

## 做完后能看到什么

接通后，符合条件的新人成为成员；没有管理权的人不能审批或修改职责。所有者交接会改变实际归属，关闭组织则必须满足成员与交接条件，不能一次点击就跳过清理。

组织、成员、待审批申请、邀请和个人入组申请各按20条显示。成员搜索会查当前有权组织的完整目录；本人即使不在当前成员页中，管理入口和退出所需的本人版本仍来自组织详情。

## 先想清楚怎样实现

1. 先保存组织、成员和邀请申请事实。
2. 再按当前身份判断每个动作是否允许。
3. 把审批与角色变化写进事务，涉及撤权或关闭时登记原任务并等待真实确认。

## 怎样判断这一课完成

A08完成后先检查组织规则，06再用不同账号实际申请、审批、交接及验证旧权限失效；创建文件不会自动生成组织。

## 开始写之前

本课继续 A02 的真实会话与 Mutation。用户创建组织、分享邀请、提交申请、审批入会，最后可转移所有者/撤销建库资格/关闭组织。正常输入都包含自己生成且重试不变的 operation_id；版本不符必须返回冲突，旧管理员不能在降权窗口继续取得目录输出。

功能讲义与分步页讲的是同一份代码，同名声明只输入一次。先理解本课功能推演，再沿分步页的完整顺序录入；章末全文只作核对。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

按[模型操作页](../../write/042-organization-models.md)、[仓储操作页](../../write/054-organization-queries.md)、[用例操作页](../../write/065-organization-service.md)、[HTTP操作页](../../write/076-organization-handlers.md)逐声明写入。文件分别持有规则/SQL/事务编排/生成DTO适配，不能把历史拆分路径重新建出来。A07 的 SessionClosure、EnqueueSourceClosure、statement/execute 与 A08 的来源捕获尚待完成，当前是包内积累。

## 2. 从真实字段决定版本与权限

<!-- lesson-code: services/account/internal/domain/account/organization_models.go :: Organization -->
````go
type Organization struct {
	CurrentMember  *OrganizationMember
	ID             string
	Name           string
	OwnerID        string
	Lifecycle      string
	Revision       string
	MemberRevision string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/organization_models.go :: OrganizationMember -->
````go
type OrganizationMember struct {
	UserID        string
	DisplayName   string
	Role          string
	State         string
	Revision      string
	Epoch         uint64
	IdentityEpoch uint64
	// 仅用于成员目录的状态说明，实际建库仍由Identity重新准入。
	KnowledgeBaseCreation string
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/domain/account/organization_models.go :: OrganizationApplication -->
````go
type OrganizationApplication struct {
	ID                    string
	OrganizationID        string
	ApplicantID           string
	Reason                string
	State                 string
	Revision              string
	SourceInvitationID    *string
	InvitationValidatedAt *time.Time
}
````
<!-- /lesson-code -->

Organization 的 OwnerID 表示唯一所有者；Lifecycle 是 ACTIVE/CLOSING/CLOSED；Revision 保护名称/所有者等实体修改，MemberRevision 保护成员目录与资格，两个版本不能混传。Member.Role 为 ADMIN/MEMBER，State 管理 ACTIVE/REVOKING/CLOSED；Revision 用于乐观变更，Epoch 用于撤销来源；IdentityEpoch 来自该用户身份。KnowledgeBaseCreation 只为目录说明，真正建库仍由 A02 身份准入核验。

`Organization.CurrentMember *OrganizationMember` 是本次详情读取附带的本人关系，不是新增数据库列。`AccessOrganization` 用真实会话账号读取并核验 ACTIVE 关系，补上 `member.UserID=actor`，再设置 `org.CurrentMember`。`OrganizationDTO` 仅把它投影成可选的 `current_member`，含 `user_id/role/state/revision`；不把来源代次暴露成浏览器可提交的权限。组织列表没有填这个字段时，不能据此猜测本人无权，应读取该组织详情。

Application 记录 ApplicantID、Reason、State、Revision，以及可空 SourceInvitationID/InvitationValidatedAt。邀请在提交时有效会固定为审批依据，之后邀请关闭不能抹去原申请依据。InvitationShare.Token 是分享原文，OrganizationInvitation 本身不携带它。

先写 InvitationCodec：AES-GCM 随机 nonce，organization ID 是附加认证数据；同一密文搬到另一组织不能解密。数据库保存 token hash 用于接受入口查询，另外保存加密原文供当前管理员分享；操作回执只存实体ID，不能把分享秘密放进审计结果。

具体仓储使用 `*repository.Applications/Invitations/Organizations`，没有另造的仓储接口。跨服务消费者契约真实存在于用例：

<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: DeparturePeer -->
````go
type DeparturePeer interface {
	Check(context.Context, string, string, string, uint64) error
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: OrganizationClosePeer -->
````go
type OrganizationClosePeer interface {
	Plan(context.Context, platform.OrganizationClosureBinding) error
	Page(context.Context, platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)
	CleanDocument(context.Context, platform.OrganizationClosureBinding, string) error
	Complete(context.Context, platform.OrganizationClosureBinding) error
}
````
<!-- /lesson-code -->

Departure.Check(ctx,organization,user,operation,epoch) 在成员退出/移除前确认 Document 是否要求职责交接，error 时不得继续关闭。OrganizationClosePeer 的 Plan/Page/CleanDocument/Complete 分别固定清理计划、获取绑定页、确认单文档载荷移除与终态；AccountCloseFencePeer 提供来源栅栏 ACK。Page 的请求/返回复用平台协议，包含原绑定、计划摘要、游标，不能只返回一个成功布尔。

## 3. 写创建、读目录与权限入口

先写 ReadOrganization/ReadOrganizationMember 的可选 `FOR UPDATE` 查询与 Organizations.Insert，再在逻辑文件写 OrganizationAccess/AccessOrganization、Organizations 和 OrganizationActions。AccessOrganization 同时检查组织 ACTIVE 与操作者成员 ACTIVE，再区分 MEMBER/ADMIN/OWNER，并捕获真实成员/管理员/所有者来源供 A08 输出闸门使用。

<!-- lesson-code: services/account/internal/repository/account/organization_queries.go :: Organizations.Insert -->
````go
func (r *Organizations) Insert(ctx context.Context, tx *sql.Tx, name, owner string) (int64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO organization(name,owner_id,lifecycle) VALUES(?,?,'ACTIVE')", name, owner)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	_, err = tx.ExecContext(ctx, "INSERT INTO organization_member(organization_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", id, owner)
	return id, err
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: OrganizationActions.Create -->
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
<!-- /lesson-code -->

例：用户17输入名称 `"  教研组  "`，handler 先 NormalizeOrganizationName 得到“教研组”（1–160字符），再 Bind Mutation。Create 设置 ORG_CREATE，Run 锁当前 user/session，Insert 在同 tx 写 organization 与所有者 ADMIN 成员，回执携带新实体ID，Commit 后才由创建handler回201。同号相同请求返回同一ID；名称变了却复用操作号会冲突。Edit 要当前管理员以及 ExpectedRevision=当前 organization.Revision，Rename 同时 revision+1，旧版本不能覆盖并发改名。

目录查询先 OpenMembers/OpenDirectory 得到目录修订，再以用户/组织/修订绑定游标，SQL 多取 limit+1 得到 more；handler 根据真实最后一项构造 next。成员目录还将规范化后的 `q` 加入绑定，实际范围为 `members:user:org:member_revision:QueryEscape(q)`。旧游标不能在新成员集合或新筛选条件下继续分页。所有目录来源捕获不可漏掉，否则降权发生在查询后输出前会泄露旧投影。

### 3.1 在服务端搜索成员，把本人资格留在详情读取里

在本课已有的四个文件中补齐这条链，不另建一个“前端成员身份”接口：`organization_models.go` 保存 CurrentMember；`organization_queries.go` 写筛选 SQL；`organization_service.go` 返回本人关系并转发条件；`organization_handlers.go` 适配生成 DTO 和 URL。原因是20条展示页并不等于整个成员集合：搜索后没有本人行、翻到后页，都不能取消原本有效的管理员资格；显示了一名管理员也不能授予查看者管理权。

| 输入/输出 | 逐项含义 |
| --- | --- |
| `GET /v1/organizations/:organizationId` | 重新验证当前成员，详情中的 `current_member` 表示本人，`owner_id` 另表示组织所有者。 |
| `GET /v1/organizations/:organizationId/members?limit=20&q=...` | `q` 是昵称子串或完整账号；前后空白去除、转小写，最多100个Unicode字符。`q=00000022` 按去前导零后的账号22匹配，不截短长编号，也不按账号尾号模糊匹配。 |
| `cursor` | 原服务端签名字符串；首页不传。它同时绑定当前账号、组织、成员集合修订及规范 `q`。 |
| `items/has_more/next_cursor` | 本次已授权成员页、是否可续读和下页入口；不是总成员人数，也不能由本页推断唯一成员或最后管理员。 |

仓储的 `Members(..., search ...string)` 与目录的 `List(..., query ...string)` 使用 Go 可变参数保持原无搜索调用有效：不传得到空条件，当前只消费首项；不是把多个参数合成多个 OR 搜索。handler 完成规范化后转发同一个值，SQL 的 `WHERE` 先限定组织、非 CLOSED 成员、`user_id>last`，再执行 `LOCATE(q,LOWER(display_name))` 或完整账号等值匹配，最后 `ORDER BY user_id LIMIT limit+1`。昵称来自本服务的 user 表，成员状态、创建资格和来源代次仍由原 JOIN 一次取得。`LOCATE` 中 `%` 和 `_` 是字面字符；用户值走占位符，不能拼 SQL。

服务端去掉多取的探测项，捕获每个返回对象的成员与身份来源，最后才输出。前端复用 `OrganizationPanel.vue` 与公共分页状态，只保存页入口游标；下一页替换当前20条，上一页也重新发 GET，而不是显示上次缓存。只有一页时不显示分页栏；提交新搜索回第1页并使旧请求失效。审批、邀请、组织列表及个人申请分别管理自己的20条页，不能串用成员游标。当前本人资格读 `current_member`，不为了找到本人下载全部成员；后续角色/离开写入仍由服务端核验本人关系与修订。

## 4. 写邀请与申请审批的事务

依次写 Invitations 的 Insert/Share/LockValidity/List 查询与 Create/Share/Cancel 用例，再写 Applications 的 create/Apply/AcceptInvitation/Decide。字段和 helper 都按操作页补齐。

<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: Applications.AcceptInvitation -->
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
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: Applications.Decide -->
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
		if err := s.repository.Decide(ctx, tx, id, session.UserID, next); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	})
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/organization_queries.go :: Applications.AdmitMember -->
````go
func (r *Applications) AdmitMember(ctx context.Context, tx *sql.Tx, organization, applicant string) error {
	return execute(ctx, tx, []statement{
		{`INSERT INTO organization_member(organization_id,user_id,role,state)
		  VALUES(?,?,'MEMBER','ACTIVE')
		  ON DUPLICATE KEY UPDATE state='ACTIVE',role='MEMBER',epoch=epoch+1,revision=revision+1`, []any{organization, applicant}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}
````
<!-- /lesson-code -->

用组织9、邀请31、申请用户22推演：管理员 Create 检查 ExpectedMemberRevision，生成 hash/密文和过期时间，写邀请后提交。用户接受时 token→摘要→IdentityByToken，锁组织9，再 LockValidity 证实 OPEN 且未过期；create 拒绝已经 ACTIVE/REVOKING 的成员，有 PENDING 申请则复用该申请，否则插入并固定邀请有效时刻。接受邀请仍是申请，不会绕过管理员审批。

管理员批准申请61：Mutation 先锁自身 user/session；Identity 读取申请所属组织和申请人；ACCEPT 分支先锁申请人 user，随后锁组织/当前管理员、申请状态。必须是 PENDING、revision 等于 ExpectedRevision，申请人 ACTIVE，成员不能 REVOKING。AdmitMember 新建 MEMBER，或恢复已关闭成员时 epoch/revision+1，再递增 organization.member_revision；Decide 写 ACCEPTED/审阅人/revision+1，与回执同 tx 提交。另一审批拿旧 revision 得 APPLICATION_CHANGED；目标正在注销得到 APPLICANT_NOT_ACTIVE，外键只证明“人存在”，不能证明“仍可加入”。REJECT 不创建成员。

## 5. 写成员变更与所有者交接：先暂停旧来源

<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: OrganizationTransitions.ChangeMember -->
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
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/organization_queries.go :: Organizations.PrepareMemberTransition -->
````go
func (r *Organizations) PrepareMemberTransition(ctx context.Context, tx *sql.Tx, organization, user string) error {
	return execute(ctx, tx, []statement{
		{`UPDATE organization_member
		  SET state='REVOKING', epoch=epoch+1, revision=revision+1
		  WHERE organization_id=? AND user_id=?`, []any{organization, user}},
		{"UPDATE organization SET member_revision=member_revision+1 WHERE id=?", []any{organization}},
	})
}
````
<!-- /lesson-code -->

移除组织9用户22、ExpectedRevision=`3`：AccessOrganization 当前管理员且持锁→禁止操作 owner→LockTransitionMember 取 ACTIVE、revision3、epoch5→有界 Departure.Check（A07 客户端1.2秒期限）确认无需知识库职责交接→PrepareMemberTransition 把成员 REVOKING、epoch6、revision4，组织成员修订+1→EnqueueSourceClosure 固定 org-member:9:22/旧epoch5、关 gate/准入并写持久任务→InsertMemberTransition FENCING→提交 PREPARING。

离开时当前管理员必须先处理职责，直接得到 ADMIN_RETIREMENT_REQUIRED；owner 不能经 remove/demote 绕开交接。角色已等于 nextRole 时返回原事实 SUCCEEDED。真正降权/移除由 A07 获得旧输出停止确认后，才能最终写新角色 ACTIVE 或 CLOSED。

TransferOwner 在组织锁之前锁目标用户，确认 ACTIVE、目标当前成员、原 organization.Revision 匹配；更新 owner_id 并暂停旧 owner 来源，持久 org-owner 关闭计划，返回 PREPARING。目标正在注销或已不是成员不能接任。ChangeCreationGrant 的 allowed=false 若原 grant ACTIVE，固定 org-create/原revision 关闭任务；WriteCreationGrant 更新资格和成员修订；撤销不能只把布尔设 false 而让旧准入继续提交建库。

## 6. 写唯一成员组织关闭及后台恢复

<!-- lesson-code: services/account/internal/domain/account/organization_models.go :: OrganizationCloseEligible -->
````go
// 唯一存续成员包括撤权中的成员，不能趁撤权未知期间清除他人数据。
func OrganizationCloseEligible(org Organization, actor, expected string, member OrganizationMember, liveMembers int) error {
	if org.Lifecycle != "ACTIVE" || member.State != "ACTIVE" {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_CLOSE_PENDING"}
	}
	if org.OwnerID != actor {
		return RuleViolation{Kind: PermissionDenied, Code: "ORGANIZATION_OWNER_REQUIRED"}
	}
	if org.Revision != expected {
		return RuleViolation{Kind: StateConflict, Code: "REVISION_CHANGED"}
	}
	if liveMembers != 1 {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_OWNER_HANDOFF_REQUIRED"}
	}
	if member.Epoch == 0 || member.Epoch == ^uint64(0) {
		return RuleViolation{Kind: StateConflict, Code: "EPOCH_EXHAUSTED"}
	}
	return nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/repository/account/organization_queries.go :: BeginOrganizationClose -->
````go
// Begin 在已持 user/session/receipt 锁后锁组织及全部成员；整个关闭意图同事务提交。
func BeginOrganizationClose(ctx context.Context, tx *sql.Tx, operation, organization, actor, expected string) (domain.OrganizationClosePlan, error) {
	var empty domain.OrganizationClosePlan
	org, err := ReadOrganization(ctx, tx, organization, true)
	if err != nil {
		return empty, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT CAST(user_id AS CHAR),role,state,epoch FROM organization_member WHERE organization_id=? ORDER BY user_id FOR UPDATE", organization)
	if err != nil {
		return empty, err
	}
	live := 0
	var own domain.OrganizationMember
	for rows.Next() {
		var member domain.OrganizationMember
		if err = rows.Scan(&member.UserID, &member.Role, &member.State, &member.Epoch); err != nil {
			break
		}
		if member.State != "CLOSED" {
			live++
		}
		if member.UserID == actor {
			own = member
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return empty, err
	}
	if err = domain.OrganizationCloseEligible(org, actor, expected, own, live); err != nil {
		return empty, err
	}
	orgID, _ := strconv.ParseUint(organization, 10, 64)
	user, _ := strconv.ParseUint(actor, 10, 64)
	revision, _ := strconv.ParseUint(org.Revision, 10, 64)
	members, _ := strconv.ParseUint(org.MemberRevision, 10, 64)
	plan := domain.OrganizationClosePlan{Binding: platform.OrganizationClosureBinding{Operation: operation, Organization: orgID, User: user, Revision: revision, MemberRevision: members, Epoch: own.Epoch}}
	if !plan.Binding.Valid() || revision == ^uint64(0) || members == ^uint64(0) {
		return empty, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_transition(operation_id,organization_id,actor_id,target_user_id,kind,expected_revision,state,fixed_plan)
	 VALUES(?,?,?,?,'CLOSE',?,'FENCING',?)`, operation, orgID, user, user, revision, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization SET lifecycle='CLOSING',revision=revision+1,member_revision=member_revision+1 WHERE id=?", orgID); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE organization_member SET state='REVOKING',epoch=epoch+1,revision=revision+1 WHERE organization_id=? AND user_id=?", orgID, user); err != nil {
		return empty, err
	}
	scope := plan.Binding.Scope()
	if _, err = tx.ExecContext(ctx, `INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?)
	 ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state='BLOCKED',last_revocation_id=VALUES(last_revocation_id),revision=revision+1`, scope, own.Epoch, operation); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE identity_admission i JOIN admission_source s ON s.admission_id=i.admission_id SET i.state='CLOSED' WHERE s.scope_key=? AND s.epoch<=?", scope, own.Epoch); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan,retry_at)
	 VALUES(?,?,?,?,?,'FENCING','["document"]',UTC_TIMESTAMP(6))`, operation, user, scope, own.Epoch, string(raw)); err != nil {
		return empty, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
	 VALUES(?,?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, OrganizationCloseRecoveryKind, operation, operation, string(raw)); err != nil {
		return empty, err
	}
	return plan, nil
}
````
<!-- /lesson-code -->
<!-- lesson-code: services/account/internal/logic/account/organization_service.go :: OrganizationClosures.advance -->
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
		return terminal, err
	})
}
````
<!-- /lesson-code -->

Begin 在 Mutation 内锁组织和按 user_id 排序的全部成员。liveMembers 统计所有非 CLOSED 成员，REVOKING 也算存续；只有当前 ACTIVE owner、revision吻合、live=1、epoch可递增才允许关闭。它固定 Binding（操作号、组织、user、组织/成员修订、原epoch），写 CLOSE transition/FENCING，将组织 CLOSING、唯一成员 REVOKING，关闭 gate/identity admission，写 authorization_change 与 ORG_CLOSE_RECOVER，回 PREPARING。

worker 的 advance 先读原计划：FENCING 时向 Document FenceAccess，严格匹配绑定 ACK 后在 save 事务中重核原计划和任务ownership，保存 ACK，进入 APPLIED。接着 Plan→Page；每次仅处理该页第一份文档，CleanDocument 明确确认后才推进 AfterDocument。空页还要 Done/NextAfter一致，Complete 成功后再 Page 读取同摘要 SUCCEEDED，才可 CompleteOrganizationClose。任何网络错误、错误页、游标/计划变化或提交未知都保留原号重试，不能把“请求发出”当载荷已经删除。

`OrganizationClosePlan.PlanDigest/AfterDocument` 保存在原 transition。claim/ownership 限制 worker 提交权，与业务 binding 分开；租约更换不是新的关闭操作。Ready 查询会拒绝未完成恢复或损坏证据，不能以健康接口简单存活代替。

## 7. HTTP 与装配、真实验收边界

organization_handlers.go 使用生成 DTO：Create/Edit/CreationGrant/InvitationCreate/ApplicationCreate/ApplicationReview/OwnerChange 等由 httpx.ReadJSON 严格读取。链路分别是 `/v1/organizations`、`/:organizationId/members`、`/invitations`、`/applications`、成员 role/DELETE/leave、owner-transfers、closures → 对应 handler → MutationRequests.Bind → 用例 → 同 tx SQL/回执。A08 的 RegisterOrganizationReads/Transitions/Closures/Actions/RegisterMembership 完整登记，A07 的 organization_lifecycle_wiring 传同一 Store、Mutation、mTLS peer 与 CaptureSource。

包齐后先跑真实存在的纯规则测试：

```powershell
Set-Location -LiteralPath (Join-Path $lessonRoot 'services/account')
go test ./internal/domain/account -run '^TestOrganizationCloseRequiresSoleActiveOwner$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

应出现该测试 RUN/PASS，覆盖非owner、非唯一存续成员（包括 REVOKING）、版本不符及正常单owner。它不验证真实 SQL 或跨服务清理。本次编写未运行；没有专门组织 CRUD 离线测试时不捏造名字。

复用支持代码后，在自己的 `$lessonRoot` 运行组件行为测试，检查替换页、上一页重读、本人不在当前页时仍保留管理与退出依据，以及搜索迟到响应不回填：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/unit/list-pagination.test.mjs tests/unit/organization-management.test.mjs
if ($LASTEXITCODE -ne 0) { throw '组织分页组件行为失败' }
~~~

这里编译真实 Vue setup 并替换外部 HTTP 边界，不启动数据库，不证明真实 SQL 已通过。06 的学习实例 READY 后，再运行 `node --no-maglev --test --test-concurrency=1 tests/workspace/list-pagination.test.mjs`；它创建真实组织夹具，验证 current_member 独立于成员页、补零账号能直接命中后页成员，以及改 `q` 后原签名游标被拒绝。失败先核规范 q、成员修订和当前会话，不能把旧 cursor 搬到新查询。模拟 API 的页面验证另见[分页功能测试安排](../../../functional-testing.md)，不要记成真实接口验收。

06四服务齐备后实际做：创建→邀请→用户申请→管理员批准→成员目录→改角色→撤建库资格→转移owner→职责交接后退出→唯一owner关闭。自己的开发SMTP和Edge依赖齐备后，在学习工程根目录串行运行：

```powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/account/organization.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
node --no-maglev --test --test-reporter=tap --test-concurrency=1 --test-name-pattern='^independent organization closure preserves account and personal work while cleaning its fixed scope$' tests/lifecycle/organization-close.test.mjs
if ($LASTEXITCODE -ne 0) { throw '本关口失败；保留输出，按下面的前置和排错检查' }
```

第一份精确名字为 `organization creates one owner, only owner appoints admins, and transfer needs a current member`、`invitation means application only, cancellation after application does not invalidate approval`、`member removal closes old identity sources and creation grants do not become organization admin`。预期全部PASS且skip0，核真实唯一owner、邀请申请语义、旧来源失效与建库资格不等于管理员；第二份还核组织关闭后本人账号/个人空间继续工作。它们会创建真实夹具，第二份涉及WSS/浏览器与载荷清理，本次未运行。检查同号重试实体ID不变、旧revision冲突、非管理员拒绝、旧来源被挡、关闭后数据清理事实。409 重新读实体/成员修订，原未知操作先续取；ADMIN_RETIREMENT_REQUIRED 回知识库交接流程；PREPARING 长时间不结束检查原 durable_task/authorization_change/Document ACK，不手改状态成功。

## 8. 章末核对

[模型答案](../../source/042-organization-models.md)、[SQL答案](../../source/054-organization-queries.md)、[用例答案](../../source/065-organization-service.md)、[HTTP答案](../../source/076-organization-handlers.md)。

原因题：为什么审批时不重新检查邀请过期？提交事务已固定有效事实，后来关闭邀请不能追溯抹去合法申请。为什么组织关闭把 REVOKING 算成员？其撤权/交接结果仍未知，不能趁未知期间清理他人数据。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
Set-Location -LiteralPath $lessonRoot
$paths = @(
 'services/account/internal/domain/account/organization_models.go',
 'services/account/internal/repository/account/organization_queries.go',
 'services/account/internal/logic/account/organization_service.go',
 'services/account/internal/handler/account/organization_handlers.go'
)
foreach ($path in $paths) {
 New-Item -ItemType Directory -Force (Split-Path $path) | Out-Null
 if (-not (Test-Path -LiteralPath $path)) { New-Item -ItemType File $path | Out-Null }
}
```
