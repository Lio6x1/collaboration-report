# D07｜正式发布与当前受众

## 这节课要实现什么

作者想把目前的文档发布给指定受众，同时继续内部编辑；之后也可能撤下公开版本。本课实现正式发布、当前正式版阅读和撤下流程。

## 做完后能看到什么

接通后，正式版保持发布时的正文、标题与样式，后续内部修改不改变它；读者只在当前受众资格允许时读取当前正式版，撤下后不能继续通过旧公开入口取得正文。

正式版历史每页20条。翻页可以浏览更多发布记录，已经打开的固定预览和已经确认选择的原发布/切换目标保持原版本，不能随历史页自动变化。

## 先想清楚怎样实现

1. 先区分内部主线权限与正式版受众。
2. 发布时固定整篇版本并保存引用。
3. 阅读时核当前发布与受众，撤下时关闭旧资格并通知索引更新。

## 怎样判断这一课完成

D14后检查发布规则，06用内部作者、受众用户和匿名访问分别验证，发布后再修改主线检查正式版是否保持不变。

## 开始写之前

继续[D06](./06-merge-conflict.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。发布意图固定页面看到的水位，公开资格和内部正文权限必须先分开。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/publication_models.go**，用途是发布意图固定页面看到的水位，公开资格和内部正文权限必须先分开。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 定义发布意图和两类阅读资格

先写publication_models.go中的PublicationIntent、PublicationFacts、FixedRelease。Operation是原动作；Kind区分RELEASE_CREATE/SWITCH/UNPUBLISH；Publication是页面期待的发布水位，Control是创建新正式版时期待的主线控制修订；Release只在SWITCH绑定真实历史正式版。发布输入不携带正文。

<!-- lesson-code: services/document/internal/domain/document/publication_models.go :: ValidatePublication -->
````go
func ValidatePublication(permissions Permissions, current dto.Document, intent PublicationIntent) error {
	// 先检查管理资格，不能借一个过期发布页面探知当前版本或推进新水位。
	if !permissions.Manage {
		return Violation(PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	if current.Lifecycle != "ACTIVE" {
		return Violation(StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	if current.PublicationEpoch != TextID(intent.Publication) {
		return Violation(StateConflict, "PUBLICATION_CHANGED")
	}
	if intent.Publication == math.MaxUint64 {
		return Violation(StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
	switch intent.Kind {
	case "RELEASE_CREATE":
		if current.ControlRevision != TextID(intent.Control) {
			return Violation(StateConflict, "MAIN_CONTROL_CHANGED")
		}
	case "SWITCH":
		if intent.Release == 0 {
			return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
		}
		if current.PublicationState == "PUBLISHED" && current.CurrentReleaseId != nil && *current.CurrentReleaseId == TextID(intent.Release) {
			return Violation(StateConflict, "PUBLICATION_ALREADY_CURRENT")
		}
	case "UNPUBLISH":
		if current.PublicationState != "PUBLISHED" || current.CurrentReleaseId == nil {
			return Violation(StateConflict, "PUBLICATION_NOT_PUBLISHED")
		}
	default:
		return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	return nil
}
````
<!-- /lesson-code -->

先核当前MANAGE，再核ACTIVE、发布水位和control等条件；耗尽不能回绕。PUBLIC、SPACE_MEMBERS、COLLABORATORS是正式版受众，不给匿名或空间成员MAIN/草稿/历史权限。PublicPolicy将KB与文档策略取更严格者，HasCurrentPublication只接受当前正式指针；回收/关闭不能退回旧正式版继续展示。PUBLIC_RETAINED也只允许已固定公开版本。

logic沿用MergeClient做可靠MAIN捕获，不另外发明“发布正文接口”。publicationContext固定Actor/Document/SessionExpires/Intent/Access/Main/Path；Main含原manifest/structure/style/control及完整units/versions。这些字段让恢复器辨认原操作，而不是读取此刻最新MAIN。

## 3. 先写固定版本SQL，再写发布事务

完成publication.go、releases.go、public_readings_queries.go。正式release只引用不可变manifest、style与结构；索引outbox同事务只存公开引用及水位，不夹正文、策略或身份token。

在publication_service.go的publicationContext附近按操作页先写publicationRequest与validatePublicationContext：

<!-- lesson-code: services/document/internal/logic/document/publication_service.go :: validatePublicationContext -->
````go
func validatePublicationContext(fixed publicationContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	main, ctx := fixed.Main, fixed.Main.Context
	if epoch := reliableMainEpoch(main); epoch == 0 || epoch > main.Control {
		return invalid
	}
	if ctx == nil || ctx.Target == nil || fixed.Actor == 0 || fixed.Document == 0 ||
		fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil ||
		!domain.ValidOperationID(fixed.Intent.Operation) || !fixed.Access.Permissions.Manage ||
		ctx.OperationId != fixed.Intent.Operation || ctx.ActorId != fixed.Actor || ctx.DocumentAdmissionRef == "" {
		return invalid
	}
	if domain.ValidatePublication(fixed.Access.Permissions, fixed.Access.Document, fixed.Intent) != nil {
		return invalid
	}
	document := fixed.Access.Document
	target := &pb.Target{DocumentId: fixed.Document, StructureId: main.Structure, WorkspaceEpoch: main.Control, Purpose: "PUBLICATION"}
	if !proto.Equal(ctx.Target, target) || document.Id != domain.TextID(fixed.Document) ||
		main.Manifest == 0 || main.Structure == 0 || main.Style == 0 ||
		document.CurrentManifestId != domain.TextID(main.Manifest) || document.CurrentStructureId != domain.TextID(main.Structure) ||
		document.CurrentStyleId != domain.TextID(main.Style) || document.ControlRevision != domain.TextID(main.Control) {
		return invalid
	}
	first := fixed.Access.Proofs[0]
	if first.AdmissionId != ctx.IdentityAdmissionRef || len(first.SessionRef) != 32 {
		return invalid
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.IdentityTarget == nil || proof.ActorId != fixed.Actor || proof.OperationId != ctx.OperationId ||
			proof.Purpose != "PUBLICATION" || !proto.Equal(proof.Target, target) || !bytes.Equal(proof.SessionRef, first.SessionRef) ||
			proof.ExpiresUnixMs <= 0 || proof.ExpiresUnixMs > fixed.SessionExpires {
			return invalid
		}
	}
	sources, err := MergeSources(fixed.Access.Proofs...)
	if err != nil {
		return invalid
	}
	acl, err := domain.CanonicalID(document.AclRevision)
	if err != nil {
		return invalid
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + document.Id, Epoch: acl})
	if document.KnowledgeBaseId != nil {
		if _, err := domain.CanonicalID(*document.KnowledgeBaseId); err != nil || fixed.Access.KBRevision == 0 {
			return invalid
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *document.KnowledgeBaseId, Epoch: fixed.Access.KBRevision})
	}
	if !equalPullSources(sources, fixed.Access.Sources) || !equalPullSources(ctx.Sources, sources) {
		return invalid
	}
	path := "/v1/documents/" + document.Id + "/publication"
	if fixed.Intent.Kind == "RELEASE_CREATE" {
		path = "/v1/documents/" + document.Id + "/releases"
	}
	fingerprint, err := domain.OperationFingerprint(path, publicationInput(fixed.Intent))
	if err != nil || fixed.Path != path || !bytes.Equal(fingerprint[:], ctx.RequestFingerprint) {
		return invalid
	}
	if len(main.Units) != len(main.Versions) {
		return invalid
	}
	seen := make(map[uint64]bool, len(main.Units))
	for _, unit := range main.Units {
		if unit == 0 || seen[unit] || main.Versions[unit] == 0 {
			return invalid
		}
		seen[unit] = true
	}
	if fixed.Intent.Kind == "RELEASE_CREATE" && len(main.Units) == 0 {
		return invalid
	}
	return nil
}
````
<!-- /lesson-code -->

原actor/session/target/purpose、原来源、Path与规范输入指纹、Main物理epoch、结构样式清单、每个unit版本都需匹配。自然准入过期只能续原绑定，不能把新的身份epoch当旧发布恢复。

之后逐段写Prepare、worker捕获、提交及恢复方法。RELEASE_CREATE固定可靠MAIN；SWITCH验证release确属当前document；UNPUBLISH不接受release_id。最终短事务锁来源、文档及旧发布水位，追加不可变版本/manifest/release或切换指针、推进publication_epoch/event_seq、写outbox及完成原operation，一起Commit。不要在Commit后再补outbox，崩溃会让搜索永远漏通知。

输入推演：原正式R1正文S1，MAIN继续保存S2。匿名读必须仍是R1/S1；管理者另建R2后才可公开S2。发布捕获已经固定但SQL未提交时断连，保留原任务恢复；不能把未知转成REJECTED后新建第二个release。event_seq达到uint64最大值时，release、版本、manifest、pointer、outbox及receipt都应整体回滚，不能只拒绝发事件。

## 4. 当前公开读与撤下

public_readings_service.go的Current先取得Doc一致PublicationFacts并判断当前正式与策略，按actor可空的受众规则取身份/内部协作资格，最后在输出事务里重核当前事实并登记真实publication来源。Historical也要按当前资格判断；release_id存在不等于可读。

用户点击发布→publication_handlers.go→ParseReleaseCreate→Publications.Prepare→Transactions登记→RunPublications→真实MAIN捕获→同事务固定release/outbox；匿名访问GET /v1/public/documents/:documentId→PublicReading.Current→PublicPolicy/ValidatePublicAudience→FixedReleaseRow→登记输出。D14的resources_outputs_wiring.go注入NewPublications/NewPublicReading，publication_outputs_workers.go驱动RecoverOne。Search只接受D14描述的正式权威投影，不读实时草稿。

### 4.1 复用20条发布历史，固定预览与原写入选择不跟页走

`web/src/features/publication/api.ts`、`usePublication.ts` 和 `PublicationPanel.vue` 属于复用前端。历史入口仍是原文档 `/releases?limit=20&cursor=...`；首页不传cursor。公共分页状态只记每页入口，下一页替换20条历史，上一页重新HTTP读取当前可见记录，只有一页时隐藏分页栏；切文档/账号清旧页与迟到请求。

历史页、固定预览和确认动作分开：翻页只改变 `releases`，不清除正在阅读的 `fixedRelease/fixed`；第一次打开确认框时保存的 `choice.document/choice.release` 继续绑定原版本与原修订。若服务端在此后改变发布事实，提交应得到明确冲突并重新核对，不能换成后来读取的修订后自动重发。当前有权读历史不等于有权发布或切换，最终写入口与原操作恢复边界仍按本课执行。

## 5. 现有验证

复用支持代码后，在自己的Windows学习工程执行 `node --no-maglev --test --test-concurrency=1 tests/unit/history-pagination.test.mjs`，应实际运行发布历史分页相关用例、fail/skip为0。它通过模拟HTTP验证替换页、固定预览和原确认选择保留，以及切文档后旧预览不回填；不证明发布捕获、真实受众权限或SQL/outbox。前端页面若翻页后预览消失，先查是否把预览清理绑到了历史列表；出现原号续接时不要因换页重新选择正文或新建操作号。真实边界继续由下方命令验收。

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestPublicationParsesCanonicalIntent|TestPublicationRequiresCurrentManagementAndPosition|TestPublicationSequencesNeverWrap)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^TestPublicationCaptureUnknownErrorsRetainOriginalRecovery$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '发布规则失败' }
~~~

publication_test.go及publication_capture_errors_test.go应逐项RUN/PASS，确认规范意图、当前管理与水位及未知保留；不证明真实SQL/outbox/RPC。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/publication/releases.test.mjs tests/publication/public-snapshot.test.mjs tests/publication/publication-sequences.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实发布/公开快照失败' }
~~~

actual Search mTLS snapshots use current authority and never disclose MAIN or fall back to old releases 核公开边界；event sequence exhaustion rolls back release, captured versions, manifest, pointer and outbox together 核整笔回滚。页面202查原operation；公开404查当前指针/生命周期；搜索迟到查原outbox/消费水位，不能改旧release正文或手动让搜索读取MAIN。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/publication_models.go | [逐声明/函数带写](../../write/098-publication-models.md) |
| 2 | services/document/internal/repository/document/public_readings_queries.go | [逐声明/函数带写](../../write/165-public-readings-queries.md) |
| 3 | services/document/internal/repository/document/publication.go | [逐声明/函数带写](../../write/166-publication.md) |
| 4 | services/document/internal/repository/document/releases.go | [逐声明/函数带写](../../write/170-releases.md) |
| 5 | services/document/internal/logic/document/public_readings_service.go | [逐声明/函数带写](../../write/219-public-readings-service.md) |
| 6 | services/document/internal/logic/document/publication_service.go | [逐声明/函数带写](../../write/220-publication-service.md) |
| 7 | services/document/internal/handler/document/publication_handlers.go | [逐声明/函数带写](../../write/257-publication-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D08](./08-resources-storage.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/publication_models.go',
    'services/document/internal/repository/document/public_readings_queries.go',
    'services/document/internal/repository/document/publication.go',
    'services/document/internal/repository/document/releases.go',
    'services/document/internal/logic/document/public_readings_service.go',
    'services/document/internal/logic/document/publication_service.go',
    'services/document/internal/handler/document/publication_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
