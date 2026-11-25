# D02｜创建工作区与可靠正文阅读

## 这节课要实现什么

用户进入个人或组织空间，创建知识库和文档，再打开正文继续工作。本课实现工作区目录、创建操作以及可靠正文读取所需的后端基础。

## 做完后能看到什么

接通后，新知识库同时拥有初始管理员，新文档有完整的初始结构；创建失败不会留下无人管理的半成品。打开正文会读取已确认版本，而不是凭页面缓存猜最新内容。

目录采用替换式分页：知识库卡片12个、文档20篇、个人协作申请20条、知识库成员20人。知识库按名称、成员按昵称或补零完整账号查询当前有权目录；后页选中的知识库仍能从地址恢复，且不会混进首页卡片。

## 先想清楚怎样实现

1. 先定义空间、知识库和文档事实。
2. 把创建对象与初始权限放进同一事务。
3. 再接目录查询、编辑入口所需资格和固定正文读取。

## 怎样判断这一课完成

D14包齐后执行相关规则，06实际新建并重新打开文档；同时验证无建库资格与事务失败时不会遗留半成品。

## 开始写之前

继续[D01](./01-identity-access.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。文档的空基线必须先有模型，再能在一笔事务中创建结构、样式、正文及清单。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/workspace_models.go**，用途是文档的空基线必须先有模型，再能在一笔事务中创建结构、样式、正文及清单。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 定义工作区的不可变根与消费者

先在 workspace_models.go 按操作页写 InitialContent、CreationKnowledgeBase、InitialStyle、BuildInitialContent。InitialContent.Version/Lineage 是正文版本和谱系编号；Canonical 是规范正文JSON；Provenance记录来源，Hash只对真实规范字节计算。Version不是“当前可读”的通行证；ContentVersionScope还必须绑定 Document、Unit、Structure。

然后在 workspace_management_service.go 的目录部分写 DirectoryReader。它从账号服务取当前空间、人员名字和组织成员；名字仅展示，目标资格仍按ID和当前组织证明判断。在 main_readings_service.go，MainReadViews声明前建立最小协作消费者：

<!-- lesson-code: services/document/internal/logic/document/main_readings_service.go :: MainReadClient -->
````go
type MainReadClient interface {
	CaptureMain(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)
	MainRevision(context.Context, uint64) (string, error)
}
````
<!-- /lesson-code -->

CaptureMain接收固定上下文、manifest/structure/style/control期待，返回协作服务的完整可靠切点；MainRevision取得当前MAIN修订。ctx传取消，error保留网络未知。Go只消费业务JSON和切点，不自己解析Yjs或读Node私有库。

## 3. 在同一事务创建一个完整空文档

依次完成 workspace_queries.go 的文档/知识库/结构/样式/清单初始化SQL，再完成 editing_queries.go 的原单元、支线及票据查询。需要参与当前写入的repository函数都接受 *sql.Tx；Queryer接口只抽取共有查询能力，不能在函数内部换DB池提交一半数据。

在 workspace_management_service.go 的 NewDocuments 后输入：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: Documents.Create -->
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
````
<!-- /lesson-code -->

Create(ctx,actor,spaceID,input) 返回更新后的actor、MutationBody和error。这个方法先做真实空间身份与知识库资格检查，返回的run闭包才在 Transactions.Mutation 传入的原事务内执行。input是生成的dto.DocumentCreate，含操作号、目标空间/库及标题等；不能另手抄一份DTO。

例子：组织普通成员7想在库20建文档。成员身份本身不等于“允许建文档”：该用户需要库管理员或当前ACTIVE创建资格；组织管理员可走其管理来源。KB属于其他空间、已关闭或ACL在请求间变化，必须分别拒绝。事务顺序是空间锁→身份/KB来源锁→KB资格当前读→配额锁→初始化。空正文也占容量，不能只给图片计费。

在 NewEntityID 后输入 InitializeDocument：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: InitializeDocument -->
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
<!-- /lesson-code -->

它先建根对象，再追加初始structure、style、manifest、chapter、unit、结构entry、空正文version、manifest entry，最后ActivateDocument。任一步失败都由外层Rollback整笔撤回；若先ACTIVE再补正文，其他读请求会看见一个无法装配的文档。初始支线带固定base version与lineage，后续可靠编辑从真实基线起步。

### 3.1 先写知识库准入模型及两张互不替代的资格表

空文档创建会检查知识库，现在把这个前向依赖补齐。仍在本课已创建的 workspace_management_service.go；在 `type KnowledgeBases struct` 之前输入 KnowledgeBaseAccess，在 NewKnowledgeBases 后输入Manage与ValidateKnowledgeBase。正文与[247操作页](../../write/247-workspace-management-service.md)是同一声明，只输入一次；Target/bindTarget按该页相邻完整段继续填写，不另建文件。

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBaseAccess -->
````go
type KnowledgeBaseAccess struct {
	KB      dto.KnowledgeBase
	Space   dto.Space
	Proof   *pb.IdentityAdmission
	Sources []*pb.SourceEpoch
}
````
<!-- /lesson-code -->

KnowledgeBaseAccess.KB是当前dto.KnowledgeBase，Id/SpaceId限定真实归属，Lifecycle表示是否仍可用，AclRevision用于管理并发，VisibilityPolicy/VisibilityRevision是公开范围的另一组状态；Space保存PERSONAL或ORGANIZATION及所有者/组织编号。Proof是这次purpose的账号准入，Sources合并会话/成员来源并加kb-acl代次；这些字段将事务外判断带到原事务复核，不能只返回一个bool“能管理”。

先在 workspace_queries.go 的 AvailableKBAdministrators/KBAdministratorReservations 后写SetKBRole、SetKBCreationGrant、AdvanceKnowledgeBaseACL，再按操作页补相邻RenameKnowledgeBase。

<!-- lesson-code: services/document/internal/repository/document/workspace_queries.go :: SetKBRole -->
````go
func SetKBRole(ctx context.Context, tx *sql.Tx, id, user uint64, role, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1", id, user, role, state)
	return err
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/document/internal/repository/document/workspace_queries.go :: SetKBCreationGrant -->
````go
func SetKBCreationGrant(ctx context.Context, tx *sql.Tx, id, user uint64, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", id, user, state)
	return err
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/document/internal/repository/document/workspace_queries.go :: AdvanceKnowledgeBaseACL -->
````go
func AdvanceKnowledgeBaseACL(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?", id)
	return err
}
````
<!-- /lesson-code -->

SetKBRole(ctx,tx,id,user,role,state)的id/user是库和账号uint64，role是ADMIN/MEMBER，state是ACTIVE/CLOSED；返回error，写knowledge_base_member。SetKBCreationGrant(ctx,tx,id,user,state)仅写knowledge_base_creation_grant，Allowed映射为ACTIVE/CLOSED；这条资格允许建文档，绝不产生管理能力。两者用ON DUPLICATE KEY推进已有行revision，参数用占位符，不能拼接用户输入。AdvanceKnowledgeBaseACL只推进库ACL；业务必须把资格变更、ACL推进和关闭意图都放进同一个tx，不能在库外更新一部分。

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.Manage -->
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
<!-- /lesson-code -->

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: ValidateKnowledgeBase -->
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
<!-- /lesson-code -->

Manage(ctx,actor,id,purpose,appoint)返回KnowledgeBaseAccess/error。id是知识库uint64，purpose区分改名、创建资格、任免等用途；appoint是**任免准入开关**，不是调用者自报的“我是管理员”。false时先尝试组织管理员，只有明确PermissionDenied才退回当前组织成员加库ADMIN；身份RPC未知不应作为降级理由。个人空间仍由SpaceIdentity核所有权。true先要求空间ORGANIZATION，并只接受组织管理员，绝不退回库ADMIN，因此库管理员能管库却不能给同事任命库管理员。

ValidateKnowledgeBase(ctx,tx,access,expected)返回锁定的当前dto.KnowledgeBase或error。它同时比较当前ACL与准入快照、请求期待；例如Manage看见库20 ACL=2，另一请求推进到3，即使客户端也写expected="2"，仍报409 KB_ACL_REVISION_MISMATCH。不能只比较用户期待而忽略准入快照，也不能用事务外那份KB直接更新。

### 3.2 建库与首任管理员在原事务一起成立

在 workspace_management_service.go 的knowledge_base部分、Rename之前输入Create；在workspace_queries.go的CreateKnowledgeBase后完成InsertInitialKBAdministrator，完整段按185操作页填写。

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.Create -->
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
<!-- /lesson-code -->

Create(ctx,actor,space,input dto.KBCreate)返回更新Sources的Actor、MutationBody、error。space从路径spaceId规范解析为uint64；input.OperationId绑定创建，Name是已由handler验证的最多160字符名称，VisibilityPolicy是可选公开范围，默认交给domain.Visibility。库ID由数据库生成，不能把客户端给的ID当新库。个人所有者用个人空间资格，组织成员则必须取得ORGANIZATION_CREATE_KNOWLEDGE_BASE来源；组织管理员身份不能越到别人的个人空间。

具体输入：用户7有组织空间10的建库资格，POST /v1/spaces/10/knowledge-bases，名称“设计资料”，一个有效operation_id。先取得并重验账号proof，合并来源；通用Mutation持有这些来源gate，再进入run锁空间。CreateKnowledgeBase插入ACTIVE库20，InsertInitialKBAdministrator插入(20,7,ADMIN,ACTIVE)，最后返回SUCCEEDED实体20，handler给201。如果首管理员SQL失败，整笔回滚，不留下无人管理的ACTIVE库。成员退出准备必须等来源gate释放，所以不能在建库提交与初始管理员事实之间穿过注销/退出。

### 3.3 改名也推进ACL并关闭旧目录输出

在Create紧后输入Rename：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.Rename -->
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
<!-- /lesson-code -->

Rename(ctx,actor,id,input dto.OrganizationEdit)复用生成的OrganizationEdit结构，名称虽叫OrganizationEdit，当前契约确实用于库改名，不手抄KB版DTO。OperationId绑定动作，ExpectedRevision在此指库AclRevision，Name是新名称；返回Actor/run/error。Manage(...,false)允许组织管理员或当前库ADMIN；只有创建资格的普通成员仍403。

用户7把库20“设计资料”改为“开发资料”，页面ACL=1：run ValidateKnowledgeBase("1")锁当前库，RenameKnowledgeBase同时改name并把acl_revision从1推进到2，QueueClosure登记kb-acl:20的旧代次1、Action=RENAME，业务结果PREPARING。目录与成员/子文档输出依赖库来源，因此SQL更新成功还需D11关闭旧来源再完成。当前Rename handler的HTTP状态是200，业务state仍可PREPARING；用waitAuthorization判断终态，不能把200等同于所有旧输出已关闭。另一张旧页面继续expected_revision="1"得到409，刷新到2后用新operation提交。

### 3.4 单独发放创建文档资格，不把成员升为库管理员

在Rename紧后输入CreationGrant：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.CreationGrant -->
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
````
<!-- /lesson-code -->

CreationGrant(ctx,actor,id,user,input dto.CreationGrant)里的id/user是handler分别从库路径、input.UserId解析的uint64。OperationId是动作号，ExpectedRevision是库ACL，Allowed决定ACTIVE/CLOSED；返回Actor/run/error。Manage(...,false)检查操作者管理库的资格，空间必须ORGANIZATION；bindTarget先向账号服务验证目标当前属于该库真实组织，并把目标来源并入锁集合。不能在拿到Doc业务锁后反向调用账号RPC。

用户8是组织10普通成员，最初向库20创建文档403 DOCUMENT_CREATION_NOT_ALLOWED。管理者发 `{expected_revision:"1",user_id:"8",allowed:true}`：run锁库确认ACL1，SetKBCreationGrant写(20,8,ACTIVE)，AdvanceKnowledgeBaseACL到2，QueueClosure旧代次1的ACL关闭任务，结果PREPARING；handler当前返回200，仍须等待同operation终态。随后用户8调用本节前面的Documents.Create：事务内检查ADMIN角色不足时读取ACTIVE创建资格，所以能建文档201；用户8再改库名仍403。allowed=false关闭这条创建资格，不改变角色表；若用户8本身另有库ADMIN/组织ADMIN，其其他创建来源仍然有效。

### 3.5 任免只给组织管理员，并保护最后一名可用管理员

在workspace_management_service.go的knowledge_base_role部分、Management类型之前输入Role：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.Role -->
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
		epoch, _ := domain.CanonicalID(kb.AclRevision)
		if err = QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: "kb-acl:" + domain.TextID(id), Epoch: epoch, Entity: id, Action: "ACL"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}
````
<!-- /lesson-code -->

Role(ctx,actor,id,user,input dto.KBRole)返回Actor/run/error。dto的ExpectedAclRevision和前两种请求的ExpectedRevision名字不同，必须使用生成字段；UserId是目标账号，Role只允许ADMIN/MEMBER/REMOVE，OperationId绑定任免。Manage(...,true)强制组织管理员，再由bindTarget验证目标仍是该组织成员。Role=REMOVE在存储中映射MEMBER+CLOSED，不能直接删除行丢掉历史revision。

组织所有者7把成员8任命为库20的ADMIN，expected_acl_revision="1"：写knowledge_base_member、ACL推进到2、登记旧kb-acl关闭任务，一笔提交，HTTP202/PREPARING。终态后8能建文档、改名，但8再任命成员9仍403；7任命组织外账号10也403。不能因为账号10有效或拥有别的组织成员身份就通过Target。

若改为MEMBER/REMOVE且目标现为ADMIN，AvailableKBAdministrators在库锁下做锁定当前读；count<=1返回409 LAST_KB_ADMIN_REQUIRED。它必须扣除注销流程已RESERVED的管理员，不能按事务较早RR快照把待退出者算作接任者。再查目标的KBAdministratorReservations，只要RESERVED就报409 KB_ADMIN_RETIREMENT_RESERVED，避免任免破坏D13已冻结的交接。查询失败返回KB_ADMIN_UNAVAILABLE，不能当作“数量很多”放行。例：库里7、8两名ADMIN，但8已预留退出，此时降级7仍会被最后管理员约束挡住；先完成有效接任再重试。

### 3.6 删除库先冻结整个范围，保留发布与转移约束

在knowledge_base_deletion部分、Role之前输入Delete：

<!-- lesson-code: services/document/internal/logic/document/workspace_management_service.go :: KnowledgeBases.Delete -->
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
````
<!-- /lesson-code -->

Delete(ctx,actor,id,input dto.VersionedAction)返回Actor/run/error；OperationId绑定删除，ExpectedRevision指库ACL。先Manage(...,false)取得准入，再快照当前ACTIVE子文档ID及其ACL，将每个doc-acl来源加入actor.Sources；超过400个子文档报409 LIFECYCLE_PLAN_REQUIRED，转走D12的完整生命周期计划，不能只处理前400个。

库20下有文档30(ACL4)、40(ACL6)时，通用Mutation先按统一来源锁顺序锁kb/doc来源，再进run锁库，避免与普通文档变更逆序等待。事务重新读取子文档当前集合：多一个、少一个或任一ACL改变均报KB_CHILDREN_CHANGED；有个人转移待完成报PERSONAL_TRANSFER_PENDING；任一当前current_release_id不为空报PUBLISHED_LIFECYCLE_REQUIRED。这里检查的是当前正式版指针，不能把它误讲成直接删光所有历史release与blob。

全部通过才把库与ACTIVE子文档一起置CLOSING、推进相关revision，登记Action=CLOSE_KB的旧kb-acl关闭任务，返回202/PREPARING。没有物理删除正式内容或绕过回收保留约束；D11关闭输出、D12/D13负责后续生命周期与交接。若关库成功而关子文档SQL失败，原事务整体回滚，不能出现目录消失但子文档仍能继续写的半关闭状态。完成关闭终态后，目录与子文档读取都应404，原operation的receipt仍可用于追踪结果。

### 3.7 把知识库按钮接到真实handler与恢复流程

D14的svc/workspace_wiring.go中NewKnowledgeBases使用同一个store.DB、Identity、Access构造logic.NewKnowledgeBases，再给workspace_handlers.go的handler.NewKnowledgeBases；handler负责解DTO、规范路径/用户ID、取actor、绑定操作号，logic返回run，PresentedMutation持来源锁执行。Delete这个handler方法实际在publication_handlers.go，仍是同一个KnowledgeBases接收者；它在D06完成，不要在本文件重复造一份Delete。

| 用户动作 | 当前routes.go登记 | handler→本节方法 |
| --- | --- | --- |
| 新建库 | POST /v1/spaces/:spaceId/knowledge-bases | workspace_handlers.KnowledgeBases.Create→Create，201/SUCCEEDED |
| 改库名 | PATCH /v1/knowledge-bases/:knowledgeBaseId | workspace_handlers.KnowledgeBases.Rename→Rename，200/PREPARING |
| 开关建文档资格 | POST /v1/knowledge-bases/:knowledgeBaseId/creation-grants | workspace_handlers.KnowledgeBases.CreationGrant→CreationGrant，200/PREPARING |
| 任免库角色 | POST /v1/knowledge-bases/:knowledgeBaseId/roles | workspace_handlers.KnowledgeBases.Role→Role，202/PREPARING |
| 删除库 | DELETE /v1/knowledge-bases/:knowledgeBaseId | publication_handlers.KnowledgeBases.Delete→Delete，202/PREPARING |
| 查当前库/成员/文档 | GET /v1/knowledge-bases/:knowledgeBaseId；后缀/members或/documents | workspace_handlers.Directory→workspace_directory_service，校当前资格及库来源 |

带PREPARING的动作均由D11的QueueClosure→Authorizations.RecoverOne→关闭输出/协作来源→操作终态恢复串起来；前端及真实测试使用GET /v1/authorization-changes/:operationId，不凭HTTP成功码推断关闭完成。

## 4. 从打开文档到可靠编辑与读取

继续按操作页完成 draft_access_service.go、editing_service.go、main_readings_service.go、readings_service.go、workspace_directory_service.go。editing_service.go 的 EditSessions.Create 绑定当前 document/unit/branch/epoch、调用者来源与票据；READ并不授予任意单元EDIT。ContentVersions.Read先按scope鉴权，再在最终读取事务中重验同一版本的归属；REVIEW/COMMENT只能读取已共享的固定历史，不能借一个版本ID偷读未送审草稿。

main_readings_service.go 固定只读MAIN视图，main_readings_queries.go保存这个视图实际依赖的manifest与正文引用。用户先读S1、再有人保存S2，原只读视图仍是S1；新的打开动作才取得新切点。该过程可以追加阅读快照，却不能顺便推进业务control、锁主线或批准审核。

浏览器动作链是 workspace_handlers.go 的创建/目录入口→Identity验证→Documents.Create返回run→Transactions.Mutation→InitializeDocument/同事务SQL。正文链是 read.go→Readings/MainReadViews→真实MainReadClient→固定切点→readings_queries.go读取不可变正文→登记输出；编辑票据链再到04协作服务WSS兑换。HTTP创建票据成功不等于编辑内容已保存；只有真实UPDATE持久确认及FLUSH能供D05送审。

D14在workspace/content_workflows两组wiring中注入AccountDirectory、真实协作HTTP客户端、validator和同一个store.DB。目录合作者/提及名单要先验证管理/评论资格，再向账号服务取候选，不能先把人名拉下来后再过滤。

### 4.1 先在服务端筛选，再组成当前有权目录页

继续在本课已有的 `workspace_queries.go`、`workspace_directory_service.go`、`workspace_handlers.go` 中补齐读取链。原因是只筛当前12张卡片会把后页真实存在的知识库误报为“没有匹配”；取完所有页再前端切片又会增加请求，并把过去获得的授权结果长期当作当前结果。

| 既有GET入口 | URL字段与产品显示大小 |
| --- | --- |
| `/v1/spaces/:spaceId/knowledge-bases` | `limit=12`、可选 `cursor/q`；q按知识库名称子串匹配。 |
| `/v1/knowledge-bases/:knowledgeBaseId/members` | `limit=20`、可选 `cursor/q`；q按昵称子串或完整账号匹配，允许输入补零展示号。 |
| `/v1/spaces/:spaceId/documents` 或 `/v1/knowledge-bases/:knowledgeBaseId/documents` | `limit=20`、可选 `cursor`；由当前空间或当前库确定范围，本轮未增加文档标题q。 |
| `/v1/document-applications` | `limit=20`、可选 `cursor`；返回本人协作申请，不由页面提交applicant身份。 |

`Directory.KnowledgeBases(..., search ...string)`、`KnowledgeBaseMembers(..., search ...string)` 与仓储 `KnowledgeBaseCandidates(..., search ...string)` 的可变参数只消费首项，未传时兼容原无搜索调用。`directorySearch(search []string)` 统一 trim、转小写，按Unicode码点限制100字符，超长返回 `DIRECTORY_QUERY_INVALID` 而不是静默截断；q为空不筛选，`%/_` 保持字面意义。它是普通只读查询参数，不需要另造请求体DTO或可写搜索状态表。

知识库名称的实际 SQL 先执行 `space_id=? AND lifecycle='ACTIVE' AND id>? AND (?='' OR LOCATE(?,LOWER(name))>0)`，再 `ORDER BY id LIMIT ?`；参数依次来自真实空间、原游标边界、规范q及扫描预算。候选预算是 `limit*10+1`，与最终可见12项分开。随后每库核显式/继承/派生可见来源，组页后在原 `ReadBody` 事务再次锁库核 ACL 与可见性；名称命中不授予任何读取或创建权限。

知识库成员姓名由 Account 权威目录提供，不能把 Account 私库联进 Document SQL。当前实现先取得知识库管理准入，通过Account当前组织目录解析昵称，再按用户ID有界扫描本库ACTIVE成员候选；q匹配小写昵称子串，或去前导零后的完整user_id。每个返回目标还需当前组织身份与成员来源证明，最后核本库成员ACTIVE及revision，才计入20个可见项。服务端先完成筛选和授权再按**可见项 limit**组页；候选预算用尽时可以是空页加 `next_cursor`，这不表示目录已结束。Account姓名目录解析也有最多100页预算，故障或预算不足要明确失败，不能填入未验证姓名或假空结果。

两个cursor绑定分别含 `actor/space/q` 与 `actor/kb/acl_revision/q`；q用同一规范值参与签名，改条件不能沿用旧cursor。handler从原URL取q，建立Identity准入后调用用例，返回sources/run交 `Transactions.Read`，原输出停止协议仍保留。组织成员的对应搜索与本人 `current_member` 见A03，不能从知识库当前成员页猜本人组织管理资格。

### 4.2 复用分页前端，把选中知识库与展示页分开

支持包的 `WorkspacePanel.vue`、`KnowledgeBasePanel.vue` 复用 `web/src/lib/pagination.ts` 和 `PaginationControls.vue`：`createCursorPage` 建立第1页，`pageCursor`只取目标入口，成功后 `acceptPage` 安装本页并截断旧后续历史，范围变更用 `resetPage`。界面显示从1开始的页码，内部索引从0开始。下一页替换当前列表，上一页同样HTTP重读；只有一页时隐藏分页栏，读取中禁用翻页。搜索提交回第1页，空间、知识库或账号变化立即清除相关旧页，响应序号和上下文代次一起阻止迟到回包覆盖新结果。

`currentKB` 单独保存已重新授权读取的当前库详情与动作摘要；恢复地址里来自后页的库时，不要求它出现在当前卡片页，也不把它追加到首页网格。创建文档仍以这份真实详情、空间归属及 `CREATE_DOCUMENT` 动作为入口，不能靠卡片缓存推断资格。成员授权用的组织候选picker与显示成员页分开，仍按需加载；空间选择及目录选择弹窗同样保留按需续读，不能为了分页预取所有正文或全部候选。

## 5. 现有测试与排错

完成支持包复用后，先在自己的Windows学习工程根目录核组件行为，再在06 READY后核真实目录SQL与权限：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/unit/list-pagination.test.mjs tests/unit/workspace-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '工作台分页组件行为失败' }
# 下面这组需要自己的四服务、迁移和开发SMTP夹具已就绪。
node --no-maglev --test --test-concurrency=1 tests/workspace/list-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实目录搜索与游标绑定失败' }
~~~

第一组编译真实组件逻辑，但HTTP边界由测试替换，验证12/20条替换、上一页重读、搜索回首页、旧响应失效以及后页当前库不混入首页；不证明真实SQL。第二组请求学习实例，创建真实组织/知识库夹具，证明补零账号与库名称在全目录查询、过滤条件绑定签名cursor，KB成员超长q被拒；须逐项运行、fail/skip为0。搜索不到先核当前空间、规范q与当前权限；空页有next仍可续读；400/409游标冲突回到同条件首页重读，不能改签名或把旧列表继续显示为有效结果。其它初始化、票据与正文测试继续按下面执行，分页测试不代替它们。

包齐后离线执行：

~~~powershell
Set-Location -LiteralPath $lessonRoot
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestBranchBaselineProjectsOnlyMainFontsAndKeepsFormalBytes|TestChapterInsertKeepsSplitChildrenAndSiblingOrder)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestDocumentCollaboratorsRequireManagementBeforeCandidateLookup|TestDocumentCollaboratorsBindTargetsCursorAndTransactionalACL)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '工作区规则失败' }
~~~

前两个规则分别在 branch_baseline_test.go/chapter_test.go；后两个在workspace_directory_test.go，用测试依赖检查先鉴权、目标绑定、游标和事务ACL，不证明真实Account/SQL已接通。每条命令均须退出0且精确用例RUN/PASS。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/workspace/initialization.test.mjs tests/workspace/edit-sessions.test.mjs tests/review/main-read-views.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实初始化/编辑/阅读失败' }
~~~

personal document becomes ACTIVE only with a complete immutable structure style and manifest 核整套空基线；a real edit session is scoped to the current unit branch and permission rather than just document reading 核编辑范围。空文档404查初始化事务是否整体提交；票据拒绝查unit归属/epoch/权限；MAIN仍旧先判断是否正在查看原固定视图，不要改旧正文使它“追上当前”。

知识库角色与创建资格需要真实账号/SQL/关闭worker，在同一06 READY前置下再运行现有功能测试；这些不是上面的离线规则用例：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/workspace/permissions.test.mjs tests/workspace/sharing.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实知识库资格与关闭流程失败' }
~~~

核对以下实际存在的用例，完整命令应退出0、TAP无fail/skip：

| 精确用例名与位置 | 按本节链路判断结果 |
| --- | --- |
| `knowledge-base admin cannot appoint peers and targets must currently belong to its organization`（permissions.test.mjs第128行起） | 所有者把成员任命ADMIN并waitAuthorization；该库ADMIN任命同级403；所有者任命组织外账号403；库ADMIN创建文档201 |
| `creation eligibility is independent from management or organization admin roles`（permissions.test.mjs第189行起） | 普通成员建文档403→所有者发创建资格并waitAuthorization→成员建文档成功→该成员改库名403；组织所有者去成员个人空间建库403 |
| `knowledge base deletion fences its children and keeps the original receipt available`（sharing.test.mjs第111行起） | 有子文档的库DELETE202→waitAuthorization→库读取404、子文档读取404。用例名提到receipt；当前断言实际只覆盖关闭与404，没有再重放receipt，不能把receipt重放记成此例实测结果 |

任命403先分操作者是不是组织管理员、目标是否仍属于当前组织，勿直接修改角色表绕过账号证明；创建403同时查组织成员来源与knowledge_base_creation_grant，不把有READ误当有创建资格；409核库ACL、最后管理员、预留退出及子文档快照，不能自动用旧operation改请求体重试。PREPARING迟迟不结束沿D11查关闭工作，而不是把状态改SUCCEEDED。这两份现有功能测试没有直接覆盖末管理员/注销预留、400上限或当前正式版删除拒绝分支；本节已按真实代码推演，仍需D12/D13相应验收，不能将本命令的通过扩大为所有生命周期分支已验证。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/workspace_models.go | [逐声明/函数带写](../../write/105-workspace-models.md) |
| 2 | services/document/internal/repository/document/editing_queries.go | [逐声明/函数带写](../../write/153-editing-queries.md) |
| 3 | services/document/internal/repository/document/main_readings_queries.go | [逐声明/函数带写](../../write/158-main-readings-queries.md) |
| 4 | services/document/internal/repository/document/readings_queries.go | [逐声明/函数带写](../../write/169-readings-queries.md) |
| 5 | services/document/internal/repository/document/workspace_queries.go | [逐声明/函数带写](../../write/185-workspace-queries.md) |
| 6 | services/document/internal/logic/document/draft_access_service.go | [逐声明/函数带写](../../write/203-draft-access-service.md) |
| 7 | services/document/internal/logic/document/editing_service.go | [逐声明/函数带写](../../write/204-editing-service.md) |
| 8 | services/document/internal/logic/document/main_readings_service.go | [逐声明/函数带写](../../write/209-main-readings-service.md) |
| 9 | services/document/internal/logic/document/readings_service.go | [逐声明/函数带写](../../write/224-readings-service.md) |
| 10 | services/document/internal/logic/document/workspace_directory_service.go | [逐声明/函数带写](../../write/246-workspace-directory-service.md) |
| 11 | services/document/internal/logic/document/workspace_management_service.go | [逐声明/函数带写](../../write/247-workspace-management-service.md) |
| 12 | services/document/internal/handler/document/read.go | [逐声明/函数带写](../../write/258-read.md) |
| 13 | services/document/internal/handler/document/workspace_handlers.go | [逐声明/函数带写](../../write/265-workspace-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D03](./03-main-draft.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/workspace_models.go',
    'services/document/internal/repository/document/editing_queries.go',
    'services/document/internal/repository/document/main_readings_queries.go',
    'services/document/internal/repository/document/readings_queries.go',
    'services/document/internal/repository/document/workspace_queries.go',
    'services/document/internal/logic/document/draft_access_service.go',
    'services/document/internal/logic/document/editing_service.go',
    'services/document/internal/logic/document/main_readings_service.go',
    'services/document/internal/logic/document/readings_service.go',
    'services/document/internal/logic/document/workspace_directory_service.go',
    'services/document/internal/logic/document/workspace_management_service.go',
    'services/document/internal/handler/document/read.go',
    'services/document/internal/handler/document/workspace_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
