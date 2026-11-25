# D04｜结构预览、拆章、合章与历史范围

## 这节课要实现什么

一章写长后，作者希望按二级标题拆成子节，或调整章节顺序。有人仍在落后的支线编辑时，也不能把他的内容丢掉。本课实现结构预览与确认流程。

## 做完后能看到什么

接通后，拆分保留原章节归属，标题前内容仍有位置，表格不会跑到别章；落后支线可按迁移证据处理，不要求先手动补相同标题。存在无法安全处理的改动时应提示处理，不能直接覆盖。

## 先想清楚怎样实现

1. 先固定主线结构和拆分意图。
2. 生成各章节及支线的完整迁移预览。
3. 确认原预览并协调在线编辑者保存，提交新结构后安装对应正文，旧位置保留追溯依据。

## 怎样判断这一课完成

D14后检查结构规则，06结合C05实际验证表格归属、落后支线和在线编辑者；预览和确认必须使用同一意图。

## 开始写之前

继续[D03](./03-main-draft.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。结构变化要先声明意图及不变量，不能从修改目录表开始。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/structure_models.go**，用途是结构变化要先声明意图及不变量，不能从修改目录表开始。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 把目录动作和正文迁移分开定义

在structure_models.go先写 MainControlIntent、MainWorkspaceFact、StructurePreviewIntent及后续结构类型。MainControlIntent.Operation绑定原动作，Kind区分结构/样式/锁定等用例；Structure保存生成dto.StructureChange；Control/Style为页面期待值，不能由浏览器提交新的数据库版本号。MainWorkspaceFact将业务metadata control与实际安装的物理MAIN代次分开：改标题未必替换物理MAIN，因此不能每次用最新control猜房间epoch。

按操作页在MainWorkspaceFact后写ResolveMainWorkspace。INITIAL只允许真实空基线；其他产品必须有固定、成功、已安装并匹配原manifest/structure/style的证据，缺一项返回未知。然后输入结构规则及预览规则的完整段；结构的rename/move必须移动整个章组，不能把它的二级子节留在原位置。

## 3. 先做预览，再确认同一份完整意图

repository依次写structure_controls.go、structure_previews_queries.go、structure_confirmation.go、structure_transitions.go及manifest_context_queries.go。预览固定整份文档的MAIN与受影响真实支线，confirmation读取原SUCCEEDED预览；不能伪造PREPARING以通过恢复就绪校验。事务中再读当前unit/workspace，逐项比较Unit、Chapter、Branch、epoch、lineage与基线。

在 structure_previews_service.go 的guard区，输入完整阻止规则：

<!-- lesson-code: services/document/internal/logic/document/structure_previews_service.go :: structurePreviewBlockingUnits -->
````go
// 预览报告阻止单元，确认/最终提交则须拒绝。两者使用同一当前guard规则，不撤回或归档原请求。
func structurePreviewBlockingUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) ([]map[string]string, error) {
	blocks := []map[string]string{}
	affected := map[uint64]bool{}
	for _, change := range fixed.Intent.Input.Changes {
		if change.Type == "SPLIT_LEVEL2" || change.Type == "JOIN_CHAPTER" || change.Type == "REMOVE" {
			id, _ := domain.CanonicalID(change.ChapterId)
			affected[id] = true
		}
	}
	for _, unit := range fixed.Units {
		if !affected[unit.Chapter] {
			continue
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, unit.Unit)
		if err != nil {
			return nil, err
		}
		reason := ""
		if guard.Submission != "" {
			reason = "SUBMISSION_PENDING"
		} else if guard.Ongoing != 0 {
			reason = "ACTIVE_REVIEW"
		} else if guard.Resumable != 0 {
			changes, err := repository.LockedReviewChanges(ctx, tx, guard.Resumable)
			if err != nil {
				return nil, err
			}
			for _, change := range changes {
				if change.Status == "OPEN" {
					reason = "OPEN_CHANGE_REQUEST"
					break
				}
			}
		}
		if reason != "" {
			blocks = append(blocks, map[string]string{"unit_id": domain.TextID(unit.Unit), "reason": reason})
		}
	}
	return blocks, nil
}
````
<!-- /lesson-code -->

它只对SPLIT_LEVEL2/JOIN_CHAPTER/REMOVE涉及的章检查review guard。存在提交中任务→SUBMISSION_PENDING；正在审核→ACTIVE_REVIEW；可恢复请求仍有OPEN意见→OPEN_CHANGE_REQUEST。预览可以报告阻止项供页面解释，最终确认必须拒绝；不能靠结构动作顺便撤审或解除意见。

具体输入：章C正文“前言P→H2甲→段A→H2乙→表T”，拆二级后产生INTRODUCTION及两个SECTION；新unit需要真实新编号，chapter身份保持。若表格属于H2乙，其整体结构不能因扫描到表格里的标题文本而错切。MAIN已有两个H2而落后支线尚无相同H2，仍应根据固定主线切点和支线迁移证据处理；不能要求支线先人为补标题。

在 structure_confirmation_service.go 输入：

<!-- lesson-code: services/document/internal/logic/document/structure_confirmation_service.go :: validateStructureConfirmationActor -->
````go
func validateStructureConfirmationActor(actor Actor, access DocumentAccess, input dto.StructureChange, fact repository.PendingStructurePreviewBinding, now time.Time) (structurePreviewContext, error) {
	var preview structurePreviewContext
	if fact.State != "SUCCEEDED" {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_NOT_FIXED")
	}
	preview, err := validateOriginalStructurePreviewBinding(fact)
	if err != nil {
		return preview, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	if err = domain.ValidateStructureConfirmation(input, preview.Intent.Input); err != nil {
		return preview, err
	}
	if actor.ID != preview.Actor || access.Document.Id != domain.TextID(preview.Document) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ACTOR_MISMATCH")
	}
	if !access.Permissions.Manage {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	if actor.SessionExpiresUnixMs <= now.UnixMilli() || preview.SessionExpires <= now.UnixMilli() {
		return preview, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if actor.SessionExpiresUnixMs != preview.SessionExpires || len(access.Proofs) == 0 {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
	}
	expected := &pb.Target{DocumentId: preview.Document, StructureId: preview.Structure, WorkspaceEpoch: preview.Control, Purpose: "MAIN_CONTROL"}
	for _, proof := range access.Proofs {
		// 原短期证明可以已经过期；当前确认必须取得同一会话的新真实证明，不能用另一登录会话接管预览。
		if proof == nil || proof.ActorId != actor.ID || proof.OperationId != input.OperationId || proof.Purpose != "MAIN_CONTROL" || !proto.Equal(proof.Target, expected) || proof.ExpiresUnixMs <= now.UnixMilli() || len(proof.SessionRef) != 32 || !bytes.Equal(proof.SessionRef, preview.Access.Proofs[0].SessionRef) {
			return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
		}
	}
	if !equalPullSources(access.Sources, preview.Access.Sources) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SOURCE_CHANGED")
	}
	if access.Document.MainLocked || access.Document.CurrentManifestId != domain.TextID(preview.Manifest) || access.Document.CurrentStructureId != domain.TextID(preview.Structure) || access.Document.CurrentStyleId != domain.TextID(preview.Style) || access.Document.ControlRevision != domain.TextID(preview.Control) {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CONTEXT_CHANGED")
	}
	return preview, nil
}
````
<!-- /lesson-code -->

actor必须是原预览actor，原session/来源/整份StructureChange意图必须匹配，输入不能只保留changes而丢expected_structure/control或preview摘要。自然过期的短期准入可续接原绑定；换会话、换actor、换来源代次是另一操作，不能借“恢复”升级资格。

接着按操作页写 structure_transitions_service.go 与 structure_control.go：候选正文和资源先经过完整验证，短事务追加结构、units、lineage、版本及manifest；持久成功后安装，再真实确认。新草稿块是尚未接纳的原预览输入，结构迁移不能把它变成已批准MAIN。

## 4. 历史权限不能只看现存块

historical_ranges.go/historical_ranges_service.go 建立原版本→真实拆分/合并后代的范围图，operations_models.go提供纯规则。旧章U拆为U1/U2后，只有U1的EDIT不能读取整个U；U1和U2都有才覆盖。合并后再拆分，要沿真实目标区间组合，不能只见“块ID还在”就授权。遇到环、未知边、含糊范围直接拒绝，限制递归深度防止坏历史拖垮读取。

浏览器结构预览→structure_handlers.go→StructurePreviews.Prepare→真实DraftTransitions客户端→固定完整cut及digest；确认→MainControls.Structure→readStructureConfirmation→loadStructureConfirmation→同事务追加新布局→worker安装。D14接NewStructurePreviews、MainControls及对应恢复器，03文件中已有的workspace操作继续保留。

## 5. 测试与预期

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestStructureRenameMoveKeepsTheWholeChapterGroup|TestStructureTransitionAllocatesNewUnitsAndRetainsChapterIdentity|TestHistoricalRangeRequiresEverySplitDescendant|TestHistoricalRangeComposesActualJoinIntervalsWithLaterSplit)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestStructureConfirmationCannotTransferOriginalSessionOrSource|TestStructurePreviewCaptureErrorKeepsUnknownOutcome|TestVirtuallyAlignedSplitCandidatesMaterializeWithoutAcceptingPendingDraft)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '结构规则失败' }
~~~

这些用例分别在structure_test.go、structure_transition_test.go、historical_range_graph_test.go、structure_confirmation_test.go、structure_preview_recovery_test.go、structure_transition_content_test.go。它们证明完整范围、原session、未知结果和未接纳草稿规则。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/structure-previews.test.mjs tests/review/structure-migration-preview.test.mjs tests/review/structure-transition.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实结构迁移失败' }
~~~

actual structure preview fixes split candidates from reliable MAIN and S2 with the original digest 应固定MAIN和可靠S2；changed reliable draft boundaries reject the same preview without an endless retry or MAIN mutation 应拒绝旧预览且MAIN不变。scope冲突查完整结构/支线绑定；一直未知查固定cut/候选是否已有持久事实；不能删除OPEN审核绕过结构阻止。

### 5.1 在页面验证新的章节入口

自己的学习实例完整READY且已构建现成前端后，打开文档的“文档设置”。目录按章列出，章前内容和二级小节缩进在原父章之下；右上“新增章节”打开单独弹窗，各章右侧“⋯”提供重命名、移动、拆分、合章及删除。主线锁定时先解锁再做已有章的调整。新增位置和移动位置在各自弹窗内选择，不再在主界面反复选择章节和动作。

拆分、合章和删除仍先“生成固定目录预览”，核对受影响正文、支线和阻止原因，再勾选确认。预览是固定输入；期间又有可靠保存会明确要求重做预览，不会静默改为新正文。在线编辑者必须完成原保存握手，不能因为页面改成弹窗就跳过这一协议。

断网后先在原弹窗点击“确认操作结果”。恢复复用同一operation_id与固定输入，成功后刷新目录并自动关闭；原结果未知时不能关闭并发起第二个动作。结构安装推进ACL时的短暂DOCUMENT_ACCESS_CHANGED/SCOPE_TRANSITION_PENDING，在原响应已未知的恢复链上继续保留待确认；首次收到的确定冲突或其它冲突仍明确失败。

```powershell
# 工作目录：自己的lessonRoot；完整READY、默认开发SMTP、Edge可用，串行运行。
node --no-maglev --test --test-concurrency=1 tests/browser/structure-stable.test.mjs tests/browser/structure-range.test.mjs
if ($LASTEXITCODE -ne 0) { throw '章节入口、原号恢复或范围迁移验收失败' }
```

预期3例通过且fail/skip为0：新增/改名/移动保留原单元与固定审核轮次；拆分/合章/删除保持可靠MAIN及独立S2正文；丢预览响应按原号恢复，之后的新可靠保存拒绝旧确认。测试主动退出不实现保存握手的夹具会话后才确认范围，不以等待超时假装编辑者已应答。这个命令不代替C05的真实多人握手矩阵。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/structure_models.go | [逐声明/函数带写](../../write/103-structure-models.md) |
| 2 | services/document/internal/repository/document/historical_ranges.go | [逐声明/函数带写](../../write/155-historical-ranges.md) |
| 3 | services/document/internal/repository/document/manifest_context_queries.go | [逐声明/函数带写](../../write/159-manifest-context-queries.md) |
| 4 | services/document/internal/repository/document/structure_confirmation.go | [逐声明/函数带写](../../write/177-structure-confirmation.md) |
| 5 | services/document/internal/repository/document/structure_controls.go | [逐声明/函数带写](../../write/178-structure-controls.md) |
| 6 | services/document/internal/repository/document/structure_previews_queries.go | [逐声明/函数带写](../../write/179-structure-previews-queries.md) |
| 7 | services/document/internal/repository/document/structure_transitions.go | [逐声明/函数带写](../../write/180-structure-transitions.md) |
| 8 | services/document/internal/logic/document/historical_ranges_service.go | [逐声明/函数带写](../../write/206-historical-ranges-service.md) |
| 9 | services/document/internal/logic/document/structure_confirmation_service.go | [逐声明/函数带写](../../write/238-structure-confirmation-service.md) |
| 10 | services/document/internal/logic/document/structure_control.go | [逐声明/函数带写](../../write/239-structure-control.md) |
| 11 | services/document/internal/logic/document/structure_previews_service.go | [逐声明/函数带写](../../write/240-structure-previews-service.md) |
| 12 | services/document/internal/logic/document/structure_transitions_service.go | [逐声明/函数带写](../../write/241-structure-transitions-service.md) |
| 13 | services/document/internal/handler/document/structure_handlers.go | [逐声明/函数带写](../../write/263-structure-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D05](./05-submission-review.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/structure_models.go',
    'services/document/internal/repository/document/historical_ranges.go',
    'services/document/internal/repository/document/manifest_context_queries.go',
    'services/document/internal/repository/document/structure_confirmation.go',
    'services/document/internal/repository/document/structure_controls.go',
    'services/document/internal/repository/document/structure_previews_queries.go',
    'services/document/internal/repository/document/structure_transitions.go',
    'services/document/internal/logic/document/historical_ranges_service.go',
    'services/document/internal/logic/document/structure_confirmation_service.go',
    'services/document/internal/logic/document/structure_control.go',
    'services/document/internal/logic/document/structure_previews_service.go',
    'services/document/internal/logic/document/structure_transitions_service.go',
    'services/document/internal/handler/document/structure_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
