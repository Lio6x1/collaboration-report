# D12｜回收恢复、永久清理与个人转移

## 这节课要实现什么

用户误删文档后希望从回收站恢复，也可能永久清理或把个人文档交给另一人。本课实现这些生命周期操作，保留权限和容量的一致性。

## 做完后能看到什么

接通后，回收可恢复文档内容，但不会自动重新公开旧正式版；永久清理要等输出和存储确认。个人转移在对方接受并满足权限与容量条件后才改变归属，失败不会先丢掉原文档。

回收站与个人转移历史各按20条展示，翻页替换当前列表并重读当前资格。接收转移时选择目标知识库仍单独按需加载，不与历史列表混用游标。

## 先想清楚怎样实现

1. 先固定操作范围。
2. 分别处理回收恢复的当前权限、永久清理的持久计划和转移邀约。
3. 协调来源关闭、资源引用及双方容量，再确认最终结果。

## 怎样判断这一课完成

D14后运行规则，06分别验证回收恢复、清理和接受转移，包含容量不足与重复请求；不要拿永久删除验证可撤销回收。

## 开始写之前

继续[D11](./11-output-fence.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。回收和迁移计划先固定原号、来源和代次，后续恢复不能猜测成功。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/retirement_models.go**，用途是回收和迁移计划先固定原号、来源和代次，后续恢复不能猜测成功。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先固定回收、清理和转移计划

先写retirement_models.go及trash_models.go。TrashFacts保存真实Document、Sequence、PurgeAfter和数据库Now，恢复期限不能用客户端时间猜。PurgePlan固定Operation、Actor/Automatic、Document/Space/KnowledgeBase、ExpectedControl/ClosingControl、ACL/Publication/Sequence及两条Scopes。Scopes只能是本Doc的doc-acl/publication关闭来源，不能借父KB或Account旧计划代替。

<!-- lesson-code: services/document/internal/domain/document/trash_models.go :: NewPurgePlan -->
````go
func NewPurgePlan(operation string, actor uint64, automatic bool, facts TrashFacts, kb, aclHighWater, publicationHighWater uint64) (PurgePlan, error) {
	var p PurgePlan
	next, err := PlanTrashRestore(facts, aclHighWater, publicationHighWater)
	if err != nil {
		return p, err
	}
	doc, err := CanonicalID(facts.Document.Id)
	if err != nil {
		return p, err
	}
	space, err := CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return p, err
	}
	control, err := CanonicalRevision(facts.Document.ControlRevision)
	if err != nil {
		return p, err
	}
	p = PurgePlan{Version: 1, Operation: operation, Actor: actor, Automatic: automatic,
		Document: doc, Space: space, KnowledgeBase: kb, ExpectedControl: control,
		ClosingControl: next.Control, ACL: next.ACL, Publication: next.Publication, Sequence: next.Sequence}
	for _, scope := range []struct {
		key   string
		epoch uint64
	}{{"doc-acl:" + TextID(doc), p.ACL}, {"publication:" + TextID(doc), p.Publication}} {
		p.Scopes = append(p.Scopes, PurgeScope{Key: scope.key, Epoch: scope.epoch, Operation: PurgeRevocation(operation, scope.key)})
	}
	if !p.Valid() {
		return PurgePlan{}, Violation(InvalidRequest, "PURGE_PLAN_INVALID")
	}
	return p, nil
}
````
<!-- /lesson-code -->

NewPurgePlan从当前回收事实和真实高水位计算下一代，派生稳定子关闭号；Valid要求恰好两条原来源且修订单调。Automatic只能由到期扫描以actor=0注册，人工必须明确确认与当前control；最大值不能回绕。PersonalTransferPlan另固定原owner/space、接收target空间/KB、配额/资源计量及原来源，双方同意前不能迁移所有权。

在personal_transfer.go先写PersonalTransferStorageReader。MeasurePersonalTransfer(ctx,plan)返回原operation/document、真实持久字节计量与digest；它在旧来源已实际关闭后读取Node原存储，不接受浏览器估计“我只写了10字”。

## 3. 回收和恢复重新取得当前管理资格

先写trash.go及personal_transfer_queries.go，随后写Trash的spaceAuthority/documentAuthority/validate、List、Restore及原号重放。普通Access只接受ACTIVE，所以回收入口必须专门读取TRASHED事实，但仍核当前空间所有者/组织管理员/知识库管理员/当前负责人。离组已清NULL的旧负责人不能复活；普通编辑授权不授予恢复。

<!-- lesson-code: services/document/internal/logic/document/trash_service.go :: Trash.Restore -->
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
<!-- /lesson-code -->

Restore返回actor/run/presentation，仍交Transactions持原来源/operation事务提交。锁序保留来源→KB→Doc→Space；先锁KB防止与可见性fanout倒序。预检后再比较当前KB ACL、space生命周期、doc ACL/负责人/库归属及期限。恢复推进新的ACL/发布水位，文档回ACTIVE但保持私有未发布，旧编辑票据与旧公开下载不能复用。

具体输入：D原先公开R1，回收时撤旧资格且发隐藏索引事件；恢复后MAIN/草稿历史仍保留，但匿名不再看到R1。用户A失去管理后重放原restore，只能读属于A/原kind/原target的固定回执，不能借同号给另一文档发新权限。

## 4. 永久清理、转移和留存治理逐段写

purge_queries.go/purge.go/purge_service.go先固定原计划、登记关闭任务，等D11真实ACK与待写任务排空，再取得Node原清理回执，删除关系历史、释放引用、确认物理资源并归还容量。数据库提交、Node删除、对象删除是分步恢复协议；某一步未知只重送原号，不把sql.ErrNoRows或TTL当所有层已删除。

个人转移先写Offer，再写接受及personal_transfer_recovery.go。Offer只生成PENDING邀约，target必须是真实活跃账号，转自己拒绝；接受者选择当前有权目标空间/KB并持久预留容量。原owner直到真正接受完成前仍有归属；恢复器先关闭旧来源，取得真实Node存储计量，按数值稳定顺序锁source/target quota及资源分摊，短事务迁移元数据、调整容量/引用、推进新ACL，再确认原任务。目标容量不足应保持原归属，不能先删source后再报余额不够。

transferred_resource.go保存跨空间资源分摊证据，不能把同一个真实对象按每个引用重复收费，也不能因旧space释放就删除新space仍引用的对象。worker/personal_transfer.go只驱动原任务，不从当前目录重新生成计划。

retained_governance_queries.go及三个retained_governance logic文件处理PUBLIC_RETAINED孤儿公开对象的下架/永久清理。先由真实管理authority授予治理证明，登记审计计划；恢复不需要管理员继续在线，但只能完成该原计划。永久治理先排空exports、取Node清理回执、删除历史及真实资源，不能复用过去Account关闭授权清新文档。

用户恢复→trash_handlers.go→Trash.Restore→Transactions；用户转移→retirement_handlers.go→PersonalTransfers.Offer/接受→RunPersonalTransfers→原计划恢复。D14的lifecycle_wiring.go注入AccountDocumentCleaner、Fences、真实Node计量及Resources/Exports，lifecycle_workers.go驱动Purges/RetainedGovernance。

### 4.1 复用20条历史展示，目标选择保持独立

前端复用 `web/src/features/trash/useTrash.ts`、`TrashPanel.vue` 与 `web/src/features/transfers/usePersonalTransfers.ts`、`PersonalTransfersPanel.vue`。回收站读取原 `GET /v1/spaces/:spaceId/trash?limit=20`，转移历史读取原 `GET /v1/personal-transfers?direction=...&limit=20`；下一页仅增加服务端cursor，范围与权限仍按原入口核验。20条是显示页大小，不是原固定转移计划、清理文档数或容量计量边界。

下一页替换当前列表，上一页重新HTTP读取；只有一页时隐藏分页栏。空间、转移方向筛选或账号变化回第1页并丢弃迟到响应，不能把旧空间的回收文档装进新空间。历史翻页不改原转移操作号、归属/版本期待或已选详情；最终恢复、接收、永久清理仍核本课固定计划和当前资格，不能用“在列表里看见”代替可执行动作。

接收目标picker另外读取本人的空间/知识库目录，按需加载更多候选；历史第2页不能推进它的cursor，也不能为了做分页预取所有目标库。目标必须真实选择并在接受事务中重新核验，不把另一个账号的库ID填进缓存绕过空间与容量规则。

## 5. 规则与实际验收

复用前端后，可先在自己的Windows学习工程运行 `node --no-maglev --test --test-concurrency=1 tests/unit/history-pagination.test.mjs`。回收和转移用例通过替换外部HTTP响应驱动真实逻辑，检查20条替换、上一页重读、换空间迟到响应丢弃，以及转移历史与接收目标picker独立；fail/skip须为0。它不证明真实恢复、跨空间容量或物理清理。下面的实际生命周期测试仍必须在专用学习实例完成，不能把组件测试记为已经删除/恢复了真实夹具。

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestTrashManagementUsesCurrentAuthority|TestTrashRestoreReplayBindsOriginalActorKindAndTarget|TestTrashRestoreRejectsClosedParentsExpiredAndExhausted|TestPurgeRequiresConfirmationCurrentTrashAndMonotonicSources|TestPersonalTransferCapacityCannotWrap|TestRetainedGovernanceOnlyClosesOrphanedPublicContent)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^TestRetainedGovernanceRejectsSubstitutedAuthorityProof$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '回收/迁移/治理规则失败' }
~~~

源为trash_test.go/purge_test.go/personal_transfer_test.go/retained_governance_test.go，核当前权限、原回执、期限、单调容量与治理证明；并未真的删库/对象。

06 READY后在专用学习实例串行：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/lifecycle/trash-restore.test.mjs tests/lifecycle/personal-transfer.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实回收恢复或转移失败' }
~~~

published documents enter trash and restore privately with current authority and original-operation recovery 应恢复私有；personal transfer retains ownership until actual acceptance, preserves fixed data and rejects insufficient persistent storage 应保留原归属与正文。永久清理另跑tests/lifecycle/purge.test.mjs，测试 purge recovers its original task and automatic expiry physically removes only the fixed documents 会真实清它创建的夹具，不能指向体验owner。资格冲突查当前管理与expected revision，清理等待查原关闭ACK/写者/对象删除，不能清卷或手改SUCCEEDED。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/retirement_models.go | [逐声明/函数带写](../../write/100-retirement-models.md) |
| 2 | services/document/internal/domain/document/trash_models.go | [逐声明/函数带写](../../write/104-trash-models.md) |
| 3 | services/document/internal/repository/document/personal_transfer_queries.go | [逐声明/函数带写](../../write/164-personal-transfer-queries.md) |
| 4 | services/document/internal/repository/document/purge_queries.go | [逐声明/函数带写](../../write/168-purge-queries.md) |
| 5 | services/document/internal/repository/document/retained_governance_queries.go | [逐声明/函数带写](../../write/172-retained-governance-queries.md) |
| 6 | services/document/internal/repository/document/transferred_resource.go | [逐声明/函数带写](../../write/182-transferred-resource.md) |
| 7 | services/document/internal/repository/document/trash.go | [逐声明/函数带写](../../write/183-trash.md) |
| 8 | services/document/internal/logic/document/personal_transfer.go | [逐声明/函数带写](../../write/217-personal-transfer.md) |
| 9 | services/document/internal/logic/document/personal_transfer_recovery.go | [逐声明/函数带写](../../write/218-personal-transfer-recovery.md) |
| 10 | services/document/internal/logic/document/purge.go | [逐声明/函数带写](../../write/222-purge.md) |
| 11 | services/document/internal/logic/document/purge_service.go | [逐声明/函数带写](../../write/223-purge-service.md) |
| 12 | services/document/internal/logic/document/retained_governance_recovery.go | [逐声明/函数带写](../../write/230-retained-governance-recovery.md) |
| 13 | services/document/internal/logic/document/retained_governance_resources.go | [逐声明/函数带写](../../write/231-retained-governance-resources.md) |
| 14 | services/document/internal/logic/document/retained_governance_service.go | [逐声明/函数带写](../../write/232-retained-governance-service.md) |
| 15 | services/document/internal/logic/document/trash_service.go | [逐声明/函数带写](../../write/244-trash-service.md) |
| 16 | services/document/internal/handler/document/retirement_handlers.go | [逐声明/函数带写](../../write/260-retirement-handlers.md) |
| 17 | services/document/internal/handler/document/trash_handlers.go | [逐声明/函数带写](../../write/264-trash-handlers.md) |
| 18 | services/document/internal/worker/personal_transfer.go | [逐声明/函数带写](../../write/274-personal-transfer.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D13](./13-account-retirement.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/retirement_models.go',
    'services/document/internal/domain/document/trash_models.go',
    'services/document/internal/repository/document/personal_transfer_queries.go',
    'services/document/internal/repository/document/purge_queries.go',
    'services/document/internal/repository/document/retained_governance_queries.go',
    'services/document/internal/repository/document/transferred_resource.go',
    'services/document/internal/repository/document/trash.go',
    'services/document/internal/logic/document/personal_transfer.go',
    'services/document/internal/logic/document/personal_transfer_recovery.go',
    'services/document/internal/logic/document/purge.go',
    'services/document/internal/logic/document/purge_service.go',
    'services/document/internal/logic/document/retained_governance_recovery.go',
    'services/document/internal/logic/document/retained_governance_resources.go',
    'services/document/internal/logic/document/retained_governance_service.go',
    'services/document/internal/logic/document/trash_service.go',
    'services/document/internal/handler/document/retirement_handlers.go',
    'services/document/internal/handler/document/trash_handlers.go',
    'services/document/internal/worker/personal_transfer.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
