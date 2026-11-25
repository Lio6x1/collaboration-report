# D01｜身份、权限与共享准入

## 这节课要实现什么

同一篇文档可能有作者、协作者、知识库管理员和无关访客。本课实现文档权限判断，让后续阅读、编辑、审核和管理知道当前用户到底能做什么。

## 做完后能看到什么

接通后，有效权限允许相应动作，无权限或已被撤销权限的请求被拒绝；显示昵称不会代替身份核验。这里建立准入基础，具体编辑与审核流程在后续课完成。

## 先想清楚怎样实现

1. 先核真实账号身份。
2. 再汇总空间、知识库、文档与章节授权。
3. 将可用能力和权限来源交给后续用例，并在实际写入事务中复核。

## 怎样判断这一课完成

文档包在D14写齐后运行本课规则；06用不同角色验证同一文档的读取与操作边界，不能只观察前端按钮。

## 开始写之前

继续[02账号服务](../../02-account.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。授权模型先把用户、范围和能力拆开；HTTP输入只能从生成契约取得。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/authorization_models.go**，用途是授权模型先把用户、范围和能力拆开；HTTP输入只能从生成契约取得。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

本讲义负责解释模型、接口、SQL与调用过程；下面所有Go代码块都是讲解用的完整声明，不直接作为第二套录入步骤。先读懂一个业务过程，再从第6节文件清单进入对应操作页，严格按操作页从第1步到最后一步录入，该页是唯一落笔顺序。不要先把讲义声明插进文件再回操作页跳过同名代码；每个文件只沿一份清单累计，最后用完整答案核对。

## 2. 先理解权限模型，再理解身份消费者接口

authorization_models.go中的SessionVerification、VerifiedSession、GrantFact、UnitFact、PermissionFacts、Permissions分别保存会话请求、验证结果和权限事实。先看ResolvePermissions如何消费这些模型；实际录入时仍沿该文件操作页的完整顺序：

<!-- lesson-code: services/document/internal/domain/document/authorization_models.go :: ResolvePermissions -->
````go
func ResolvePermissions(f PermissionFacts) Permissions {
	p := Permissions{Edit: map[uint64]bool{}, Manage: f.Responsible || f.PersonalOwner || f.OrganizationAdmin || f.KnowledgeBaseAdmin}
	for _, g := range f.Grants {
		if g.User != f.Actor {
			continue
		}
		switch g.Capability {
		case "MANAGE":
			if g.Kind == "DOCUMENT" {
				p.Manage = true
			}
		case "REVIEW":
			if g.Kind == "DOCUMENT" {
				p.Review = true
				p.Comment = true
				p.Read = true
			}
		case "COMMENT":
			if g.Kind == "DOCUMENT" {
				p.Comment = true
				p.Read = true
			}
		case "EDIT":
			for _, u := range f.Units {
				if (g.Kind == "UNIT" && g.Scope == u.ID) || (g.Kind == "CHAPTER" && g.Scope == u.Chapter) {
					p.Edit[u.ID] = true
					p.Read = true
				}
			}
		}
	}
	if p.Manage {
		p.Read = true
		p.Comment = true
		p.Review = true
		for _, u := range f.Units {
			p.Edit[u.ID] = true
		}
	}
	return p
}
````
<!-- /lesson-code -->

PermissionFacts.Actor 是账号服务验证过的 uint64 主键，不是昵称。四个布尔字段分别表示当前负责人、个人空间所有者、组织管理员、知识库管理员；任一管理来源成立就授予完整管理能力。GrantFact.User 绑定目标用户；Kind 决定 DOCUMENT、CHAPTER 或 UNIT 范围；Scope 保存范围编号；Capability 保存能力。Units 把当前单元映射到章，章授权因此能展开为准确的单元集合。Permissions.Edit 用 map 保存每个单元是否可写，不能用一个 Edit=true 抹平范围。

现在用用户7、章10、单元11/12推演：同时有 CHAPTER/10/EDIT 与 UNIT/11/EDIT 时，移除后者仍可编辑11和12；两份授权是有效能力的并集。只有 EDIT 的人能读整份 MAIN，但不能因此评论或审核；REVIEW 才额外包含 COMMENT 与 READ。CoversHistoricalRange 对历史范围要求证明完整，拆成11/12后仅有11的编辑权限不能读整个旧章。

authorization_service.go中的消费者接口SessionVerifier把身份核验能力与HTTP请求对象分开。先理解下面的方法形状，录入位置统一按该文件操作页：

<!-- lesson-code: services/document/internal/logic/document/authorization_service.go :: SessionVerifier -->
````go
type SessionVerifier interface {
	VerifySession(context.Context, domain.SessionVerification) (domain.VerifiedSession, error)
}
````
<!-- /lesson-code -->

VerifySession(ctx, SessionVerification) 接收 Cookie、Origin、CSRF 和 Mutation 标志，返回账号服务核验后的 ActorID 与会话绝对期限，失败返回 error。读操作和写操作的CSRF要求不同，所以 Mutation 不能丢。logic不保存 *http.Request，才能被HTTP、后台流程和测试分别调用。AdmissionRenewer 专门续接原准入，不能把任意过期读凭证续成管理凭证。

## 3. 从仓储事实推导当前授权

authorization_queries.go以Access/NewAccess及Document、Space、KnowledgeBase、KnowledgeBaseRole、Grants、Units提供这些事实；全部声明在该文件操作页连续录入。Access.Document(ctx,id) 查询当前文档；Grants 返回这名actor的真实授权行；Units(structure,id) 必须限定文档和当前结构。sql.ErrNoRows 在角色查询中表示没有该角色，数据库断连则是未知，不能也解释为“普通成员”。

logic的DocumentAccess汇总当前文档与资格。下面用Authorize推演如何取得这些事实；这是讲解代码，实际文件沿操作页录入：

<!-- lesson-code: services/document/internal/logic/document/authorization_service.go :: Access.Authorize -->
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
<!-- /lesson-code -->

Authorize(ctx, actor, id, purpose) 返回文档、权限、来源代次 Sources、原准入 Proofs 与 KBRevision。过程是查文档→空间→按个人/组织身份取得真实证明→查KB状态和角色→读直接授权与当前单元→计算权限→重验身份→合并来源→附加doc-acl/kb-acl。组织管理员明确被拒时才退回成员资格；RPC超时不能当普通成员继续。purpose限定这次准入用途，防止一份阅读证明被挪去送审。

接着理解ValidateAccess怎样在业务事务内复核原授权；声明的实际顺序仍以操作页为准：

<!-- lesson-code: services/document/internal/logic/document/authorization_service.go :: ValidateAccess -->
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
<!-- /lesson-code -->

想象 Authorize 读到ACL=4，另一请求已撤权推进到5；本次事务再锁文档，比较发现4≠5就报 DOCUMENT_ACCESS_CHANGED。事务外成功不是最终准入。相同来源在多份证明里必须指向同一代次；输出依赖账号、会话、文档ACL等多个来源，任一被关闭就要停止输出。这与上面的“授权能力取并集”是不同问题。

### 3.1 先为个人分享定义令牌与加密消费者

现在推演个人文档“创建链接→申请→审批”。TokenHash与ResolvePermissions同属authorization_models.go，负责令牌的规范身份；它们在同一操作页各出现一次，不在这里另行插入或重写文件。

<!-- lesson-code: services/document/internal/domain/document/authorization_models.go :: TokenHash -->
````go
// TokenHash只接受规范的32字节随机令牌；不同编码不能绕过同一邀请的持久身份。
func TokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, Violation(InvalidRequest, "INVITATION_TOKEN_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
````
<!-- /lesson-code -->

TokenHash(token string) 返回规范令牌对应的32字节摘要或 error。客户端提交的是 URL 中的字符串，数据库查找的是 SHA256(raw)，这里的 raw 是 Base64URL 解码后的32个随机字节，不能对令牌文字直接求摘要。再次编码并比较原字符串，是为了拒绝补 `=`、换行或非规范尾位等编码别名。无效输入返回 INVITATION_TOKEN_INVALID，handler对外报400 APPLICATION_INPUT_INVALID。

先理解sharing_service.go的消费者接口InvitationCipher：Sharing只依赖加密/解密能力。讲解可以先看接口，实际录入仍从该文件操作页第1步顺序推进，到InvitationCipher对应步骤时再写；无需提前插入或之后跳过任何声明。

<!-- lesson-code: services/document/internal/logic/document/sharing_service.go :: InvitationCipher -->
````go
type InvitationCipher interface {
	Seal(string, string) ([]byte, error)
	Open([]byte, string) (string, error)
}
````
<!-- /lesson-code -->

Seal 的两个 string 参数依次是规范 token、真实文档编号的绑定字符串，返回密文字节及 error；Open 的参数是密文字节、同一绑定，返回原 token 或 error。业务只依赖这两个能力。client/account_identity_client.go 的 InvitationTokens 才读取密钥文件、使用AES-GCM；Seal把文档编号作为附加认证数据，因此把文档30的密文挪给文档40、再用绑定“40”解密会失败。读取密钥或认证失败必须返回错误，不能换发一个新token掩盖原邀请丢失。

sharing_queries.go中的InsertInvitation和InsertApplication分别持久化邀请及申请。下面两段用于说明它们的同事务输入与返回，实际位置按该文件操作页：

<!-- lesson-code: services/document/internal/repository/document/sharing_queries.go :: InsertInvitation -->
````go
func InsertInvitation(ctx context.Context, tx *sql.Tx, document, actor uint64, hash, encrypted []byte, grants string, expires time.Time) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_invitation(document_id,inviter_id,token_hash,token_ciphertext,token_key_id,requested_grants,state,expires_at) VALUES(?,?,?,?,'dev-document-invitation-v1',?,'OPEN',?)", document, actor, hash, encrypted, grants, expires)
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/document/internal/repository/document/sharing_queries.go :: InsertApplication -->
````go
func InsertApplication(ctx context.Context, tx *sql.Tx, document, invitation, actor uint64, grants, reason string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO document_application(document_id,invitation_id,applicant_id,requested_grants,invitation_valid_at_submit,invitation_validated_at,reason,state) VALUES(?,?,?,?,TRUE,UTC_TIMESTAMP(6),?,'PENDING')", document, invitation, actor, grants, reason)
}
````
<!-- /lesson-code -->

InsertInvitation 的 document/actor 是真实文档和邀请人 uint64；hash 用来匹配申请，encrypted 用来在有资格时重显链接，grants 是冻结候选权限的JSON，expires 是绝对期限。它返回数据库生成的邀请ID。InsertApplication 的 document/invitation/actor 则分别绑定文档、来源邀请和申请人；SQL在原事务保存 requested_grants、invitation_valid_at_submit=TRUE 与 invitation_validated_at=UTC_TIMESTAMP(6)，返回申请ID。两函数都接收调用方的 *sql.Tx，不自行提交，否则可能留下“邀请已生成但operation没记住”或“申请已存在但提交有效性未固定”的半成品。

### 3.2 创建链接：原号重试也重新核验敏感输出

CreateInvitation组合personalManagement、validateInvitationGrants、invitationSnapshot等协作者完成创建邀请。先沿完整函数推演正常与失败输入；录入时按sharing_service.go操作页一次完成这些声明及其余必需方法。

<!-- lesson-code: services/document/internal/logic/document/sharing_service.go :: Sharing.CreateInvitation -->
````go
func (s *Sharing) CreateInvitation(ctx context.Context, actor Actor, id uint64, hours int64, input dto.DocumentInvitationCreate) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.personalManagement(ctx, actor, id, "CREATE_DOCUMENT_INVITATION")
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if d.AclRevision != input.ExpectedAclRevision {
			return nil, domain.Violation(domain.StateConflict, "ACL_REVISION_MISMATCH")
		}
		if err = validateInvitationGrants(ctx, tx, id, input.RequestedGrants); err != nil {
			return nil, err
		}
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			return nil, err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256(raw)
		encrypted, err := s.tokens.Seal(token, domain.TextID(id))
		if err != nil {
			return nil, err
		}
		grants, _ := json.Marshal(input.RequestedGrants)
		invite, err := repository.InsertInvitation(ctx, tx, id, actor.ID, hash[:], encrypted, string(grants), time.Now().UTC().Add(time.Duration(hours)*time.Hour))
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", invite), nil
	}
	// 创建与原号重放都重新核验敏感输出资格；receipt只保存实体引用，令牌由绑定密文恢复。
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		raw, _ := json.Marshal(value["result"])
		var ref struct {
			Entity string `json:"entity_id"`
		}
		if json.Unmarshal(raw, &ref) != nil {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
		inviteID, err := domain.CanonicalID(ref.Entity)
		if err != nil {
			return nil, nil, err
		}
		invite, encrypted, err := invitationSnapshot(ctx, tx, inviteID, true)
		if err != nil || invite.DocumentId != domain.TextID(id) {
			return nil, nil, errors.New("INVITATION_RESULT_UNAVAILABLE")
		}
		token, err := s.tokens.Open(encrypted, domain.TextID(id))
		if err != nil {
			return nil, nil, err
		}
		return dto.DocumentInvitationCreated{Invitation: invite, ShareUrl: s.origin + "/join-document?invitation_token=" + token, OperationId: input.OperationId}, access.Sources, nil
	}
	return actor, run, present, nil
}
````
<!-- /lesson-code -->

CreateInvitation(ctx, actor, id, hours, input) 的 id 是路径解析后的文档 uint64；actor 是已验证会话；hours 是handler确定的期限小时数，未传 ExpiresInHours 时48，允许1到720；input 是生成的 dto.DocumentInvitationCreate。OperationId 绑定一次创建，ExpectedAclRevision 拒绝旧页面，RequestedGrants 每项用 ScopeKind/ScopeId/Capability 表示准确范围。函数返回更新 Sources 的 Actor、写入闭包 MutationBody、重显闭包 MutationPresentation、error；不是直接把 ShareUrl 当固定操作结果返回。

用个人空间所有者7管理文档30、当前ACL=1、单元31为例：输入 expected_acl_revision="1"，requested_grants=[{scope_kind:"UNIT",scope_id:"31",capability:"EDIT"}]。personalManagement先核MANAGE及空间PERSONAL；管理组织文档也不能走个人分享。run再次锁定文档检查ACL与范围。EDIT只允许该文档当前UNIT/CHAPTER，REVIEW/COMMENT只允许当前DOCUMENT，邀请不能携带MANAGE；其他文档的单元会得到 GRANT_SCOPE_NOT_IN_DOCUMENT。

随后依次生成32随机字节→RawURLEncoding规范token→SHA256(raw)；不是顺序编号，也不是SHA256(token文字)。Seal(token,"30")得到绑定文档30的密文，InsertInvitation写OPEN邀请、摘要、密文、权限和UTC期限。随机源、加密或SQL失败都退出原事务。run只返回 OperationResult 的实体引用，通用Mutation在同一事务存receipt；receipt没有明文token或完整share_url。

present在同一事务 ValidateAccess，解析receipt的 entity_id，持锁读取原邀请并确认 DocumentId="30"，再Open(ciphertext,"30")构造 `origin + "/join-document?invitation_token=" + token`，同时返回 access.Sources 供输出登记。第一次和同operation重试都经过当前personalManagement及present，重试读取原邀请而不再生成随机字节。用户第一次响应丢失后仍能恢复原链接；若管理资格已丢失，或本次重新准入到输出之间ACL又变化，不能凭旧receipt取得敏感链接；单纯历史ACL变化不阻止当前仍有权者恢复原链接。不要在handler开头套普通Replay直接返回以前的明文结果；当前CreateInvitation handler特意始终调用用例与PresentedMutation。

### 3.3 申请时固定有效性，申请成功暂不授予正文能力

下面通过Apply理解申请为什么先按邀请定位文档；代码在sharing_service.go操作页的对应步骤录入：

<!-- lesson-code: services/document/internal/logic/document/sharing_service.go :: Sharing.Apply -->
````go
func (s *Sharing) Apply(ctx context.Context, actor Actor, hash []byte, input dto.DocumentApplicationCreate) (Actor, MutationBody, error) {
	invitation, document, err := repository.InvitationByHash(ctx, s.db, hash)
	if err != nil {
		return actor, nil, err
	}
	snapshot, err := repository.DocumentRow(ctx, s.db, document, false)
	if err != nil {
		return actor, nil, err
	}
	epoch, _ := domain.CanonicalID(snapshot.AclRevision)
	actor.Sources = append(append([]*pb.SourceEpoch{}, actor.Proof.Sources...), &pb.SourceEpoch{ScopeKey: "doc-acl:" + domain.TextID(document), Epoch: epoch})
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := repository.DocumentRow(ctx, tx, document, true)
		if err != nil {
			return nil, err
		}
		if current.AclRevision != snapshot.AclRevision {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_ACCESS_CHANGED")
		}
		invite, _, err := invitationSnapshot(ctx, tx, invitation, true)
		if err != nil {
			return nil, err
		}
		if invite.State != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "INVITATION_NOT_CURRENT")
		}
		// 同一文档锁串行不同operation的申请，避免同一个申请者生成多个待审批实体。
		existing, err := repository.PendingApplication(ctx, tx, document, actor.ID)
		if err == nil {
			return OperationResult(input.OperationId, "SUCCEEDED", existing), nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		raw, _ := json.Marshal(invite.RequestedGrants)
		id, err := repository.InsertApplication(ctx, tx, document, invitation, actor.ID, string(raw), input.Reason)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}
````
<!-- /lesson-code -->

Apply(ctx, actor, hash, input) 的 hash 是handler调用 TokenHash 得到的 []byte，input 的 InvitationToken只用于原请求绑定，Reason保存申请说明，OperationId保持请求幂等。返回 Actor/MutationBody/error。申请人尚无文档READ，因此这里先按邀请摘要定位文档，不能先调用要求已能阅读的 Authorize；actor.Sources仍绑定自己的真实会话来源与当前doc-acl。

继续前例：用户8在邀请OPEN且未过期时申请。事务锁文档核ACL，然后锁邀请；invitationSnapshot会把已到期限的OPEN投影为EXPIRED，非OPEN报409 INVITATION_NOT_CURRENT。只有这时有效才把邀请的 RequestedGrants 复制进申请，保存提交有效性与验证时刻。PendingApplication在同一文档锁下查用户8已有PENDING；不同operation也会指向同一待审批实体，sql.ErrNoRows才允许新建，其他查询错误不能当作“没有申请”。这笔事务没有写 content_grant，所以HTTP202表示申请已收下，用户8读文档仍应403。

随后所有者取消邀请：CancelInvitation核 expected_revision 并把OPEN改CLOSED，阻止新申请。用户9再用同token申请得到409；用户8此前保存的 RequestedGrants 与有效提交时间仍保留。GET /v1/document-applications按actor.ID查自己的申请元数据，用户9看到空页，不能借“进度”读文档正文；管理者列文档申请才走personalManagement。分页binding含actor、文档/ACL及过滤条件，不能把别人的游标复制过来续查。

### 3.4 审批固定申请，ACCEPT等待旧授权来源关闭

再看Decide如何决定固定申请。此处只是调用过程的讲解顺序，文件位置仍遵循sharing_service.go操作页：

<!-- lesson-code: services/document/internal/logic/document/sharing_service.go :: Sharing.Decide -->
````go
func (s *Sharing) Decide(ctx context.Context, actor Actor, id uint64, input dto.ApplicationReview) (Actor, MutationBody, error) {
	application, err := repository.ApplicationRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, err
	}
	document, _ := domain.CanonicalID(application.DocumentId)
	access, err := s.personalManagement(ctx, actor, document, "DECIDE_DOCUMENT_APPLICATION")
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	if input.Decision == "ACCEPT" {
		target, _ := domain.CanonicalID(application.ApplicantId)
		proof, err := s.identity.Target(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, "DECIDE_DOCUMENT_APPLICATION", document)
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
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		d, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		current, err := repository.ApplicationRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.State != "PENDING" || current.Revision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "APPLICATION_CHANGED")
		}
		state := "REJECTED"
		if input.Decision == "ACCEPT" {
			if err = validateInvitationGrants(ctx, tx, document, current.RequestedGrants); err != nil {
				return nil, err
			}
			for _, grant := range current.RequestedGrants {
				if err = repository.ApproveInvitationGrant(ctx, tx, document, current.ApplicantId, grant); err != nil {
					return nil, err
				}
			}
			state = "ACCEPTED"
			if err = repository.AdvanceDocumentACL(ctx, tx, document); err != nil {
				return nil, err
			}
			epoch, _ := domain.CanonicalID(d.AclRevision)
			if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "doc-acl:" + domain.TextID(document), Epoch: epoch, Entity: document, Action: "ACL"}); err != nil {
				return nil, err
			}
		}
		// 提交时有效性已持久固定；审批不重新检查邀请当前期限或OPEN状态。
		if err = repository.DecideApplication(ctx, tx, id, actor.ID, state); err != nil {
			return nil, err
		}
		resultState := "SUCCEEDED"
		if state == "ACCEPTED" {
			resultState = "PREPARING"
		}
		return OperationResult(input.OperationId, resultState, id), nil
	}
	return actor, run, nil
}
````
<!-- /lesson-code -->

Decide(ctx, actor, id, input) 的 id 是applicationId，不是文档ID；dto.ApplicationReview含 OperationId、ExpectedRevision、Decision与Reason。handler只接受ACCEPT/REJECT、理由最多2000字符；当前实现不把审批Reason写入申请表，DecideApplication保存审核人、状态并增加revision，不要把请求字段误说成已持久化审核理由。返回 Actor、同事务MutationBody或error。

管理者7接受用户8的申请：事务外取得该个人文档的当前管理准入，ACCEPT额外核用户8仍是ACTIVE_ACCOUNT并把目标来源并入锁集合。事务内ValidateAccess后锁申请，必须仍为PENDING且Revision等于ExpectedRevision，否则409 APPLICATION_CHANGED。读取的是申请冻结的 RequestedGrants；validateInvitationGrants再次确认这些范围仍属于当前文档，然后逐项写/恢复ACTIVE content_grant，增加文档ACL，登记旧doc-acl代次的QueueClosure，最后将申请改ACCEPTED。所有步骤与receipt在一笔事务里；若第二条授权写失败，不能留下第一条授权与PENDING申请混在一起。

**审批不再查邀请当前OPEN或期限。** 用户8是在取消前有效提交，因此邀请后来CLOSED仍可批准；但目标账号已无效、申请revision被其他审核推进、单元已不属当前文档，仍会失败。REJECT只改申请并返回业务状态SUCCEEDED；ACCEPT返回业务状态PREPARING，即使SQL提交成功也要等D11的Authorizations恢复流程关闭旧来源输出，再转SUCCEEDED。HTTP审批入口统一202，不能只看202就断言新的可读权限已完成。测试的waitAuthorization轮询 GET /v1/authorization-changes/:operationId 到SUCCEEDED后，用户8读MAIN才应200，editable-units包含31。

可见性另按 visibility.go → visibility_service.go 计算父KB与文档限制，PUBLIC不会代替本节的内部READ授权，也不会自动发布正式版。

## 3.1 管理列表分页与删除记录

本节要让管理员在大量邀请和申请中快速找到待处理事项，并能整理过期记录。邀请默认显示“有效”，可切换“已失效/全部”；申请默认“待处理”，处理后进入“已处理”。每页最多20条、最新在前；更换筛选回首页。

先在F02创建 `db/migrations/013.up.sql`，按对应分步页写入归档表。`record_kind + record_id`防止邀请和申请数字ID相同造成混淆；`document_id`绑定管理范围，`actor_id/archived_at`保留操作责任。原申请和邀请行都保留，不删除授权。

接着在本课的 `sharing_queries.go` 编写 `ApplicationIDs`、`InvitationIDs` 和 `ArchiveSharingRecord`。状态与NOT EXISTS归档过滤必须在LIMIT前完成；ID降序、首页last=0、后页id<last，取limit+1判断后页。管理者查询排除归档，本人进度查询保留全部历史。

再在 `sharing_service.go` 编写 `sharingFilter`、`Applications`、`Invitations` 和 `Archive`。分页binding加入newest及筛选状态，因此有效列表游标拿去查询失效列表返回409。申请人昵称由Account的DOCUMENT_APPLICATION_NAMES原证明解析，调用在事务外，姓名来源合入输出来源，输出事务内复核原申请revision；不是从Doc跨库查询用户表。

不存在记录与没有管理资格统一403及DOCUMENT_MANAGEMENT_REQUIRED，不能让裸记录编号探测记录是否存在；查询故障保持服务失败。关闭链接只接受原修订且仍OPEN；已关闭/过期的记录改走管理归档。

归档按“文档当前管理资格→文档锁→原记录锁→expected_revision→状态→归档INSERT+原revision推进”执行。OPEN邀请和PENDING申请返回409；已关闭/过期邀请、已同意/拒绝/撤回申请可以删除管理记录。归档不写content_grant，申请人仍可看到自己的审批结果。handler先查原operation_id回执，未知结果仍用原号恢复，不能另造新号重复执行。

前端支持包中的 `SharingPanel.vue`接管两组列表、分页历史和确认弹窗；`DocumentReader`只负责文档资格与刷新。成员选择在 `MemberPicker.vue` 默认收起，打开后输入姓名，自动继续读取有权候选后页，选中即收起。找不到当前页成员不能直接判定全目录不存在。

## 4. 把用户动作接到实现

完成 account_identity_client.go 的真实账号HTTP/RPC适配；它实现 SessionVerifier/AdmissionRenewer，业务层不跨库读取账号表。httpx/body.go 严格解码生成DTO，httpx/errors.go 将 RuleViolation 映射稳定错误。authorization_handlers.go 获取actor、解析文档编号及 expected_acl_revision，调用 AuthorizationChanges.Prepare，再交通用 Transactions.Mutation 执行返回的事务函数。

D14把真实 svc.NewIdentity → svc.NewAccess → svc.NewAuthorizationChanges 接入routes；当前授权变更入口是 POST /v1/documents/:documentId/authorization-changes，对应 routes.Authorizations.Change。浏览器撤销一个单元授权后，handler到Prepare，Prepare绑定受影响目标真实身份，Mutation持来源锁执行SQL并登记关闭任务。不要只更新grant行后立即返回“旧连接已关闭”；D11负责关闭旧来源输出。

个人分享的真实注册在 handler/routes.go，依赖在D14的svc/workspace_wiring.go：NewSharing将同一个store.DB、Identity、Access、Pagination、InvitationTokens、cfg.Origin和AccountDirectory交给logic.NewSharing，再注入handler.NewSharing。下表按用户动作解释authorization_handlers.go的方法对应关系；录入仍沿该文件操作页顺序，不依表另插方法。

| 用户动作 | 路由 | handler→logic与结果 |
| --- | --- | --- |
| 创建个人分享链接 | POST /v1/documents/:documentId/invitations | Sharing.CreateInvitation→CreateInvitation→PresentedMutation(run,present)，200含重验后的share_url |
| 管理者查看或取消邀请 | GET /v1/documents/:documentId/invitations；POST /v1/document-invitations/:invitationId/cancel | Invitations读元数据；CancelInvitation核VersionedAction.ExpectedRevision并关闭，200 |
| 登录用户申请 | POST /v1/document-invitations/applications | Apply先TokenHash→logic.Apply→PresentedMutation，202仍无READ |
| 申请人看自己的申请 | GET /v1/document-applications | Applications传document=0，按actor绑定；不输出正文 |
| 管理者看文档申请 | GET /v1/documents/:documentId/applications | Applications传路径document，先personalManagement |
| 删除失效邀请/已处理申请记录 | POST /v1/document-invitations/:invitationId/archive；POST /v1/document-applications/:applicationId/archive | ArchiveInvitation/ArchiveApplication→Archive→原号PresentedMutation，200；不撤销授权 |
| 管理者审批 | POST /v1/document-applications/:applicationId/decisions | Decide→logic.Decide→PresentedMutation，ACCEPT为202/PREPARING→D11关闭恢复→进度SUCCEEDED |

## 5. 验证与定位

同包文件齐全后做离线纯规则检查：

~~~powershell
Set-Location -LiteralPath $lessonRoot
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestRemovingDirectUnitGrantRetainsInheritedChapterEdit|TestEditorReadsWholeMainButDoesNotGainComments|TestHistoricalRangeRequiresEveryCurrentDescendant)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '权限规则失败' }
~~~

这些测试在 internal/domain/document/permissions_test.go，应逐个出现RUN/PASS；无MySQL、RPC或WSS。若出现 [no tests to run]，检查名称和支持包中的现有测试文件，不能把它记为通过。

06整站READY后执行 node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/workspace/permissions.test.mjs tests/workspace/sharing.test.mjs。真实用例 scope identifiers belong to the document and revoking a direct edit preserves inherited chapter edit 同时核范围归属与撤销后继承。403先区分缺READ与缺MANAGE，409核ACL/范围是否已变，503沿身份/SQL错误定位；不要把503提示为密码错误。

把本课分享验收单独跑一遍时，前置仍是06已启动自己的四服务与Docker中间件、测试支持文件已装入学习工程；下面不是离线Go测试，不要指向教师体验实例：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/workspace/sharing.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实个人分享/可见性/知识库关闭失败' }
~~~

精确用例 `personal link snapshots validity, grants only after approval, and own progress reveals no private body` 在sharing.test.mjs中：UNIT/EDIT邀请200、默认期限距当前超过47小时→申请202而读403→本人申请列表一条、陌生人零条→取消200→迟到申请409→原有效申请ACCEPT202→waitAuthorization终态→MAIN200且可编辑单元等于邀请单元。逐条断言分别证明“候选权限没有提前生效”“申请元数据只属于本人”“取消只截断新提交”“旧有效提交可继续审批”“跨服务关闭完成后才判新授权生效”。同文件全部四例也须通过，命令退出0、TAP无fail/skip。

新增用例 `sharing filters run before pagination and archive preserves accepted access` 验证倒序有效分页、跨筛选游标409、非法状态400、活跃记录409、昵称、无权归档403、旧修订409、原号重试200、管理列表移除而已有READ及本人历史保留。

若迟到申请却202，查invitationSnapshot的时限/状态和取消revision；若取消后旧申请审批409，先看APPLICATION_CHANGED、目标账号或范围变化，不能补一个邀请OPEN检查。若批准后立刻读被拒，先轮询同operation进度；AUTHORIZATION_NOT_TERMINAL沿D11关闭任务与协作ACK排查，不删等待。敏感链接重显的加密绑定与当前管理检查是本节代码推演要求；这个共享功能测试没有覆盖密文篡改或撤权后的原号链接重显，不能把这两点报成该测试的实测结论。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属与录入清单。按表逐个打开操作页，每页从文件头到最后一步连续完成，讲义中的代码只用于解释和对照。接口在讲义中可以先讲，但不改变操作页声明顺序；不另插同名声明、不跳写辅助函数。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/authorization_models.go | [逐声明/函数带写](../../write/090-authorization-models.md) |
| 2 | services/document/internal/domain/document/errors.go | [逐声明/函数带写](../../write/092-errors.md) |
| 3 | services/document/internal/domain/document/ids.go | [逐声明/函数带写](../../write/094-ids.md) |
| 4 | services/document/internal/repository/document/authorization_queries.go | [逐声明/函数带写](../../write/146-authorization-queries.md) |
| 5 | services/document/internal/repository/document/sharing_queries.go | [逐声明/函数带写](../../write/176-sharing-queries.md) |
| 6 | services/document/internal/repository/document/visibility.go | [逐声明/函数带写](../../write/184-visibility.md) |
| 7 | services/document/internal/logic/document/authorization_service.go | [逐声明/函数带写](../../write/194-authorization-service.md) |
| 8 | services/document/internal/logic/document/pagination.go | [逐声明/函数带写](../../write/216-pagination.md) |
| 9 | services/document/internal/logic/document/sharing_service.go | [逐声明/函数带写](../../write/237-sharing-service.md) |
| 10 | services/document/internal/logic/document/visibility_service.go | [逐声明/函数带写](../../write/245-visibility-service.md) |
| 11 | services/document/internal/client/account_identity_client.go | [逐声明/函数带写](../../write/106-account-identity-client.md) |
| 12 | services/document/internal/handler/document/authorization_handlers.go | [逐声明/函数带写](../../write/251-authorization-handlers.md) |
| 13 | services/document/internal/handler/httpx/body.go | [逐声明/函数带写](../../write/266-body.md) |
| 14 | services/document/internal/handler/httpx/errors.go | [逐声明/函数带写](../../write/267-errors.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D02](./02-workspace-content.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/authorization_models.go',
    'services/document/internal/domain/document/errors.go',
    'services/document/internal/domain/document/ids.go',
    'services/document/internal/repository/document/authorization_queries.go',
    'services/document/internal/repository/document/sharing_queries.go',
    'services/document/internal/repository/document/visibility.go',
    'services/document/internal/logic/document/authorization_service.go',
    'services/document/internal/logic/document/pagination.go',
    'services/document/internal/logic/document/sharing_service.go',
    'services/document/internal/logic/document/visibility_service.go',
    'services/document/internal/client/account_identity_client.go',
    'services/document/internal/handler/document/authorization_handlers.go',
    'services/document/internal/handler/httpx/body.go',
    'services/document/internal/handler/httpx/errors.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~

## 页面验收：分页、筛选和恢复

完成06、完整READY和前端构建后，在“协作 → 邀请链接/加入申请”按[页面验收表](../../reference/page-acceptance.md)操作。邀请每页最多20条，用上一页/下一页替换当前页；状态筛选回到首页。申请先看待处理，处理后在已处理查看昵称、时间、原因和权限范围；只有失效邀请与已处理申请显示“删除记录”。确认弹窗明确已有权限不变。页面不展示邀请token作为列表身份。

```powershell
# 工作目录：自己的lessonRoot；默认开发SMTP、完整READY、Edge可用。
node --no-maglev --test --test-concurrency=1 tests/browser/sharing-management.test.mjs
if ($LASTEXITCODE -ne 0) { throw '分享管理页面验收失败' }
```

预期1例通过：真实创建21条独立邀请，验证20+1分页、有效/失效筛选、申请昵称和已处理状态；归档POST成功但响应丢失后，必须从原弹窗恢复同一编号和输入；390px页面无横向溢出。已有权限及本人历史是否保留由上面的sharing.test.mjs真实用例检查，不能仅凭列表消失得出结论。
