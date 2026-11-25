# D13｜账号注销、组织关闭与公开留存

## 这节课要实现什么

账号注销或组织关闭后，私有文档需要清理，约定保留的公开正式版仍需按规则留存。本课实现文档服务参与这些跨服务关闭流程。

## 做完后能看到什么

接通后，清理范围在开始时固定，职责未交接时不会继续；公开留存与私有清理分开处理。原关闭请求重试只推进原计划，不能重新扩大删除范围。

## 先想清楚怎样实现

1. 先核账号服务给出的原操作并预留职责交接。
2. 冻结来源后逐文档执行固定清理。
3. 保存下游确认与留存依据，最后向账号服务报告真实处理结果。

## 怎样判断这一课完成

D14后运行关闭规则，06配合A06、A07和C06验证整条注销链，检查私有内容与公开留存各自结果。

## 开始写之前

继续[D12](./12-trash-transfer.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。跨服务注销先固定不可变清理范围，不能先删除用户再寻找依赖。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/account_lifecycle_models.go**，用途是跨服务注销先固定不可变清理范围，不能先删除用户再寻找依赖。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 定义关闭范围和权威消费者

account_lifecycle_models.go先写AccountRetirementBinding/Plan、AccountRetentionDocument/Scope/Plan、HistoryRevision以及全部解码/规则。AccountRetentionDocument只存被固定的document/space/KB/release/manifest/structure/style、version/resource编号；Release=0表示整份清理，非0只保留该正式版自己的依赖。不能把当前活跃MAIN或整份私有正文塞进注销receipt。

AccountRetirementAuthority.Rejected用于取消准备时查询Account原操作的持久REJECTED；AccountRetentionAuthority提供原Account冻结与最终状态，AccountDocumentCleaner取得Node原单文档清理证明，OrganizationCloseAuthority服务于独立组织关闭。消费者接口写在logic中，具体mTLS HTTP客户端在account_lifecycle_client.go；Doc不能跨库读Account私有表或接受用户自报ClosingOrganizations。

## 3. 先预留管理交接，再冻结身份

依次写admin_retirement.go、account_retirement_queries.go、account_retention_queries.go，再写account_retirement_service.go的Prepare与Cancel：

<!-- lesson-code: services/document/internal/logic/document/account_retirement_service.go :: AccountRetirements.Prepare -->
````go
// Prepare是跨服务注销准备的一部分，不是注销完成。调用者必须是已验证原操作的
// Account控制通道；当前不作为用户HTTP入口，不能接收用户自行声明的关闭组织集合。
func (s *AccountRetirements) Prepare(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementResult, error) {
	result := domain.AccountRetirementResult{RequiredKnowledgeBases: []uint64{}, ReservedKnowledgeBases: []uint64{}}
	if !binding.Valid() {
		return result, errors.New("ACCOUNT_RETIREMENT_BINDING_INVALID")
	}
	// 不存在的原操作不需要范围锁：RR的缺行间隙锁会与随后知识库锁反向等待。
	// RC下仍显式锁定身份来源、知识库、成员和退休事实；操作唯一键处理跨用户撞号。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// 与任命目标身份、本人建库使用同一来源锁，预留后不能迟到获得新的管理员职责。
	gate, err := repository.LockSourceGate(ctx, tx, "identity:"+domain.TextID(binding.User), binding.IdentityEpoch)
	if err != nil {
		return result, err
	}
	record, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err == nil {
		plan, decodeErr := domain.DecodeAccountRetirementPlan(record.Plan)
		if decodeErr != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
		}
		if record.State == "REJECTED" {
			result.State = "REJECTED"
			return result, nil
		}
		matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
		if err != nil {
			return result, err
		}
		if !matches {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
		}
		result.State, result.ReservedKnowledgeBases = record.State, plan.ReservedKnowledgeBases
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if gate.Epoch > binding.IdentityEpoch || (gate.Epoch == binding.IdentityEpoch && gate.State == "BLOCKED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	candidates, err := repository.LockAdministratorRetirementCandidates(ctx, tx, binding.User)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		// 自有个人空间及权威明确将关闭的组织不属于继续存续的库。
		if candidate.PersonalOwner == binding.User || slices.Contains(binding.ClosingOrganizations, candidate.Organization) {
			continue
		}
		if candidate.Lifecycle != "ACTIVE" || candidate.MemberState != "ACTIVE" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_TRANSITION_PENDING")
		}
		if candidate.ReservationState == "RESERVED" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
		}
		if candidate.Available <= 1 {
			result.RequiredKnowledgeBases = append(result.RequiredKnowledgeBases, candidate.KnowledgeBase)
		}
		result.ReservedKnowledgeBases = append(result.ReservedKnowledgeBases, candidate.KnowledgeBase)
	}
	if len(result.RequiredKnowledgeBases) > 0 {
		// 不提交任何来源或部分预留，用户仍可正常交接后重新发起。
		result.State, result.ReservedKnowledgeBases = "HANDOFF_REQUIRED", []uint64{}
		return result, nil
	}
	for _, kb := range result.ReservedKnowledgeBases {
		if err = repository.ReserveAdministratorRetirement(ctx, tx, kb, binding.User, binding.Operation); err != nil {
			return result, err
		}
	}
	plan, err := json.Marshal(domain.AccountRetirementPlan{Binding: binding, ReservedKnowledgeBases: result.ReservedKnowledgeBases})
	if err != nil {
		return result, err
	}
	if err = repository.RecordAccountRetirement(ctx, tx, binding, plan); err != nil {
		return result, err
	}
	if err = repository.FreezeAccountRetirement(ctx, tx, binding); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, errors.New("ACCOUNT_RETIREMENT_PREPARE_UNKNOWN")
	}
	result.State = "FROZEN"
	return result, nil
}
````
<!-- /lesson-code -->

Prepare(ctx,binding)返回AccountRetirementResult及error，属于已验证Account控制通道的一部分，不是用户HTTP直接注销入口。事务用READ COMMITTED，按identity来源gate→当前KB/成员/管理员候选→retirement事实顺序核验；仍显式持记录锁，唯一操作键防撞号。原因是不存在原操作的RR间隙锁可能与后续KB锁反向等待，不能靠进程mutex替代数据库并发保护。

输入推演：A/B是某存续KB仅有的两个管理员，同时注销。每次预留都必须看到其他可用管理人和真实reservation；不能两边都看到“还有对方”然后同时退出。若任一KB Available≤1，返回HANDOFF_REQUIRED及具体库集合，整笔不提交身份冻结或部分预留，用户还能交接。个人空间与Account权威明确会关闭的组织按原计划排除，不能由浏览器随便列入排除集合。

成功固定计划并FreezeAccountRetirement后是FROZEN，仍不是注销完成。Cancel只接受Account对同一binding的持久REJECTED；超时、查无记录、租约到期不是回滚证明，已开始真实排空也不能走准备取消。迟到fence还会识别已权威取消状态，防止重新封住恢复身份。

## 4. 按原计划逐文档清理，最后去身份化

account_retention_service.go先固定Doc实际留存计划与两类来源关闭；account_cleanup_service.go的DocumentCleanupContext从已保存计划取单个文档及真实关闭范围：

<!-- lesson-code: services/document/internal/logic/document/account_cleanup_service.go :: AccountRetentions.DocumentCleanupContext -->
````go
// DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
// 的Account冻结，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。
func (s *AccountRetentions) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	result, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	// 返回给Node之前先结束只读事务，SQL错误不能被成功HTTP掩盖。
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}
````
<!-- /lesson-code -->

它要求原计划CLEANING、该文档确在计划、对应doc-acl/publication/KB屏障与真实ACK匹配、没有pending writes，再在事务结束后向Node发证。一个文档只检查自己的屏障，不因其他无关文档未知全局阻塞；但不能从调用参数拼缺失屏障。

随后写CleanHistory/ReleaseDraftResources、account_payload_cleanup.go、account_cleanup_queries.go、account_retention_complete：先在锁外取Node清理回执，事务内核原回执并删版本/资源关系；操作payload中的私有正文副本也要清，删关系表不代表副本已经消失。HistoryDeletionOrder从真实parent图叶子开始删除，不能按最大ID猜最新；保留当前公开版所需祖先只保留骨架，不能留下祖先私有正文条目。

organization_lifecycle_queries.go/service.go为独立组织关闭固定该组织范围，Account仍活跃，个人空间继续可用。lifecycle_rpc.go给Account/Node提供原计划上下文；candidate_cleanup.go/service.go清未提交候选，未知commit不能删仍可能被采用的候选。

动作链是用户向Account发注销→Account控制通道调用account_lifecycle_handlers.go的Prepare/Plan/Progress/Complete→AccountRetirements/AccountRetentions→原Doc来源关闭→Node原清理→Doc历史/资源/payload清理→Account最终去身份化。D14的lifecycle_wiring.go与bootstrap注入真实AccountClosure、OrganizationCloseAuthority及AccountDocumentCleanup。Doc无权先清账号再回头找文档。

## 5. 验证与前置

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^TestOrganizationCloseCleanupRequiresFixedDocument$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '关闭计划范围规则失败' }
~~~

organization_close_test.go只证明计划中真实文档和来源限定。logic的TestActualAccountRetirementAtomicHandoffAndConcurrentExit在account_retirement_integration_test.go，需要实际数据库测试前置；名称里的Actual不能拿来冒充离线检查。

06全站READY后串行：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/lifecycle/account-close.test.mjs tests/lifecycle/organization-close.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实注销或组织关闭失败' }
~~~

account closure persists and completes original public/private cleanup before deidentification 核清理完成才去身份化；independent organization closure preserves account and personal work while cleaning its fixed scope 核范围隔离。HANDOFF_REQUIRED先按返回库交接；FROZEN/CLEANING久等查原计划、fence ACK及Node单文档回执；不能换号重发或把租约到期当撤销，也不能把公开留存当私有草稿已清。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/account_lifecycle_models.go | [逐声明/函数带写](../../write/089-account-lifecycle-models.md) |
| 2 | services/document/internal/repository/document/account_cleanup_queries.go | [逐声明/函数带写](../../write/140-account-cleanup-queries.md) |
| 3 | services/document/internal/repository/document/account_payload_cleanup.go | [逐声明/函数带写](../../write/141-account-payload-cleanup.md) |
| 4 | services/document/internal/repository/document/account_retention_queries.go | [逐声明/函数带写](../../write/142-account-retention-queries.md) |
| 5 | services/document/internal/repository/document/account_retirement_queries.go | [逐声明/函数带写](../../write/143-account-retirement-queries.md) |
| 6 | services/document/internal/repository/document/admin_retirement.go | [逐声明/函数带写](../../write/144-admin-retirement.md) |
| 7 | services/document/internal/repository/document/candidate_cleanup.go | [逐声明/函数带写](../../write/147-candidate-cleanup.md) |
| 8 | services/document/internal/repository/document/lifecycle_rpc.go | [逐声明/函数带写](../../write/156-lifecycle-rpc.md) |
| 9 | services/document/internal/repository/document/organization_lifecycle_queries.go | [逐声明/函数带写](../../write/162-organization-lifecycle-queries.md) |
| 10 | services/document/internal/logic/document/account_cleanup_service.go | [逐声明/函数带写](../../write/190-account-cleanup-service.md) |
| 11 | services/document/internal/logic/document/account_retention_service.go | [逐声明/函数带写](../../write/192-account-retention-service.md) |
| 12 | services/document/internal/logic/document/account_retirement_service.go | [逐声明/函数带写](../../write/193-account-retirement-service.md) |
| 13 | services/document/internal/logic/document/candidate_cleanup.go | [逐声明/函数带写](../../write/195-candidate-cleanup.md) |
| 14 | services/document/internal/logic/document/lifecycle_rpc.go | [逐声明/函数带写](../../write/207-lifecycle-rpc.md) |
| 15 | services/document/internal/logic/document/organization_lifecycle_service.go | [逐声明/函数带写](../../write/214-organization-lifecycle-service.md) |
| 16 | services/document/internal/client/account_lifecycle_client.go | [逐声明/函数带写](../../write/107-account-lifecycle-client.md) |
| 17 | services/document/internal/handler/document/account_lifecycle_handlers.go | [逐声明/函数带写](../../write/250-account-lifecycle-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D14](./14-search-assembly.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/account_lifecycle_models.go',
    'services/document/internal/repository/document/account_cleanup_queries.go',
    'services/document/internal/repository/document/account_payload_cleanup.go',
    'services/document/internal/repository/document/account_retention_queries.go',
    'services/document/internal/repository/document/account_retirement_queries.go',
    'services/document/internal/repository/document/admin_retirement.go',
    'services/document/internal/repository/document/candidate_cleanup.go',
    'services/document/internal/repository/document/lifecycle_rpc.go',
    'services/document/internal/repository/document/organization_lifecycle_queries.go',
    'services/document/internal/logic/document/account_cleanup_service.go',
    'services/document/internal/logic/document/account_retention_service.go',
    'services/document/internal/logic/document/account_retirement_service.go',
    'services/document/internal/logic/document/candidate_cleanup.go',
    'services/document/internal/logic/document/lifecycle_rpc.go',
    'services/document/internal/logic/document/organization_lifecycle_service.go',
    'services/document/internal/client/account_lifecycle_client.go',
    'services/document/internal/handler/document/account_lifecycle_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
