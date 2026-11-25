# D03｜固定主线操作与支线更新

## 这节课要实现什么

作者直接修改主线，协作者在共同支线写作后又需要同步主线更新。本课实现主线固定操作和支线更新流程，保护双方已保存的工作。

## 做完后能看到什么

接通后，主线变化有固定依据；更新支线会产生可核对的预览，冲突不能静默覆盖。重复确认或请求超时沿用原操作，不能重复生成另一份更新。

## 先想清楚怎样实现

1. 先固定本次目标、版本与操作号。
2. 准备主线或支线候选。
3. 确认时复核原意图并提交文档事实，随后让协作服务安装同一份结果，未知状态通过原回执恢复。

## 怎样判断这一课完成

D14后运行规则，06结合C04验证主线保存、支线更新及冲突分支；预览成功和最终安装完成要分别检查。

## 开始写之前

继续[D02](./02-workspace-content.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。历史范围和固定操作的纯规则先确定，随后才注册可恢复的主线和支线任务。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/operations_models.go**，用途是历史范围和固定操作的纯规则先确定，随后才注册可恢复的主线和支线任务。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先规定业务操作怎样重试

operations_models.go 的历史范围规则服务于之后pull和结构迁移；同一个unit编号不自动证明仍是同一完整范围。先把该文件全部类型和函数按操作页输入，再在 transactions_service.go 的声明区写 OperationCommand、MutationBody、MutationPresentation、BindOutput：

<!-- lesson-code: services/document/internal/logic/document/transactions_service.go :: OperationCommand -->
````go
type OperationCommand struct {
	ID, Kind, Path string
	Input          any
	// 有外键依赖原号的流程先在同一事务登记准备态，再执行业务写入；失败仍整体回滚。
	InitialResult map[string]any
}
````
<!-- /lesson-code -->

ID是用户这一次业务动作的稳定编号；Kind区分动作；Path绑定实际路由目标；Input参与规范指纹；InitialResult只供必须先有原操作外键的流程登记准备态。MutationBody(ctx,tx) 返回只含固定结果的map及error；MutationPresentation在同一事务内用当前资格把固定结果投影为可交付响应；BindOutput把输出登记绑定到真正handler，失败可Abort。

例子：operation=main-control-op-0001首次用来改文档10标题；网络丢回复后同参数重发，应读原回执。若用相同main-control-op-0001改文档11或改另一标题，必须 OPERATION_ID_CONFLICT。不能每次点击/网络重试都换操作号。

## 3. 写原事务与固定工作流

先完成 operation_results_queries.go 的operation gate、回执读写。然后在 Transactions 的构造声明附近按操作页位置输入Mutation完整方法：

<!-- lesson-code: services/document/internal/logic/document/transactions_service.go :: Transactions.Mutation -->
````go
// 通用receipt只存固定结果；敏感投影在原事务内重新鉴权后构造，并登记它实际依赖的来源。
func (s *Transactions) Mutation(ctx context.Context, actor Actor, command OperationCommand, handler string, run MutationBody, present MutationPresentation, bind BindOutput) (any, error) {
	if !domain.ValidOperationID(command.ID) {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_ID_INVALID")
	}
	if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil {
		return nil, err
	}
	fingerprint, err := domain.OperationFingerprint(command.Path, command.Input)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := repository.LockOperationGate(ctx, tx, command.ID); err != nil {
		return nil, err
	}
	sources := actor.Sources
	if sources == nil {
		sources = actor.Proof.Sources
	}
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	receipt, err := repository.LockedOperation(ctx, tx, command.ID)
	var value map[string]any
	if err == nil {
		if receipt.ActorID != actor.ID || receipt.Kind != command.Kind || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		if json.Unmarshal(receipt.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_PENDING")
		}
	} else if err == sql.ErrNoRows {
		if command.InitialResult != nil {
			if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, command.InitialResult); err != nil {
				return nil, err
			}
		}
		value, err = run(ctx, tx)
		if err != nil {
			return nil, err
		}
		if command.InitialResult != nil {
			result, err := platform.EncodeOperationReceipt(command.ID, value)
			if err != nil {
				return nil, err
			}
			if err = repository.UpdateOperationResult(ctx, tx, command.ID, result.State, result.JSON); err != nil {
				return nil, err
			}
		} else if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, value); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	var response any = value
	responseSources := actor.Proof.Sources
	if present != nil {
		response, responseSources, err = present(ctx, tx, value)
		if err != nil {
			return nil, err
		}
	}
	registration, err := s.outputs.Register(ctx, tx, responseSources, 0, "", handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, responseSources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return response, nil
}
````
<!-- /lesson-code -->

调用顺序是核原操作号→重验身份→解析真实Edge实例→算Path/Input指纹→BeginTx→锁operation gate→按固定来源顺序锁gate→查原回执。没有回执才运行run；已有回执只在actor/kind/指纹一致时重放。之后在同一事务重建敏感投影、登记实际输出租约并bind；Commit失败调用Abort，不能让handler继续泄露尚未提交的响应。defer Rollback仅清理未提交事务，不会撤销成功Commit。

再完成 main_controls_queries.go、pull_main_queries.go、discard_previews_queries.go、discard_drafts_queries.go：它们把PREPARING、固定输入、业务提交和安装回执分开。main_controls_service.go先固定整份可靠MAIN、Prepare barrier，再在短事务提交新的结构/样式/manifest；网络阶段放在事务锁外。Go已经提交而Node安装超时，原业务结果仍是成功并等待同步，不能回滚成REJECTED。

进入 pull_main_service.go，先建立PullClient，再写Prepare→compute→commit→reconcile→RecoverOne的全部声明。PullClient只描述协作预览、固定正文及安装相关调用，返回的pb类型沿用生成契约。

<!-- lesson-code: services/document/internal/logic/document/pull_main_service.go :: PullClient -->
````go
type PullClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	Prepare(context.Context, any) ([]byte, error)
	Apply(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)
}
````
<!-- /lesson-code -->

具体推演：支线基线B，已送审S1后又保存S2，MAIN接纳了S1并新增M；pull必须计算固定MAIN与可靠S2之间的兼容更新，使新支线保留S2和M。不能把草稿替换成MAIN。预览后又保存S3，旧预览不能继续装入，应拒绝原切点已变。冲突保持两份固定输入且不推进epoch。control_manifest_validation.go核完整manifest绑定，不能从一个单元推断全文成功。

discard是另一条显式危险动作：先固定preview，再在用户确认的原预览/原支线/原结构/原意见集合上恢复MAIN。原草稿仍保留历史；撤回待审并不解除修改意见，所以discard不能偷偷归档仍OPEN的意见。对应两份logic和repository文件全部写齐，不能仅写公开入口。

## 4. 接到真实路由和恢复器

operations_handlers.go 从生成DTO取expected_control_revision、expected_structure_id、preview及operation，交MainControls/PullMain/DraftControls的Prepare，再由Transactions执行run。D14的 content_workflows_wiring.go 注入真实Merges/DraftTransitions客户端，content_workflows_workers.go 分别驱动 MAIN_CONTROL_RECOVER、PULL_MAIN_RECOVER、DISCARD_* 和各自reconcile。

用户点击“拉取主线”→POST /v1/documents/:documentId/units/:unitId/pull-main→PullMain handler→Prepare登记原号→worker固定可靠草稿与MAIN→短事务追加新lineage/epoch→Node真实安装→reconcile登记安装确认。查询原operation而非重复点击新建任务，才能观察丢回复的恢复。

## 5. 检查

同包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestMainControlIntentAndPosition|TestDiscardIntentBindsExplicitChanges|TestMainWorkspaceUsesOriginalCompletedManifest)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestPendingPullRejectsAnotherOriginalAdmissionActor|TestPendingPullRejectsMissingOriginalDocumentAdmission|TestPullFixedContentReadsTheExactOriginalProducerEnvelope)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '固定工作流规则失败' }
~~~

测试文件为main_control_test.go、discard_draft_test.go、main_workspace_test.go及pull_readiness_test.go/pull_wire_test.go。预期原actor/准入缺失被拒，固定生产者信封不能用另一份JSON替换；无真实RPC。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/main-controls.test.mjs tests/review/pull-main.test.mjs tests/review/discard-draft.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实主线/支线操作失败' }
~~~

explicit pull preserves reliable draft and installs a new branch epoch under the original operation 应保留后来草稿；discard rejects a preview followed by a reliable draft edit without changing epoch 应保持旧epoch。PREPARING久不结束查原task/固定输入/协作可达性；SUCCEEDED但syncing查reconcile真实安装；409查原预览是否被后续编辑越过，不手改任务终态。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/operations_models.go | [逐声明/函数带写](../../write/096-operations-models.md) |
| 2 | services/document/internal/repository/document/discard_drafts_queries.go | [逐声明/函数带写](../../write/151-discard-drafts-queries.md) |
| 3 | services/document/internal/repository/document/discard_previews_queries.go | [逐声明/函数带写](../../write/152-discard-previews-queries.md) |
| 4 | services/document/internal/repository/document/main_controls_queries.go | [逐声明/函数带写](../../write/157-main-controls-queries.md) |
| 5 | services/document/internal/repository/document/operation_results_queries.go | [逐声明/函数带写](../../write/161-operation-results-queries.md) |
| 6 | services/document/internal/repository/document/pull_main_queries.go | [逐声明/函数带写](../../write/167-pull-main-queries.md) |
| 7 | services/document/internal/logic/document/control_manifest_validation.go | [逐声明/函数带写](../../write/200-control-manifest-validation.md) |
| 8 | services/document/internal/logic/document/discard_drafts_service.go | [逐声明/函数带写](../../write/201-discard-drafts-service.md) |
| 9 | services/document/internal/logic/document/discard_previews_service.go | [逐声明/函数带写](../../write/202-discard-previews-service.md) |
| 10 | services/document/internal/logic/document/main_controls_service.go | [逐声明/函数带写](../../write/208-main-controls-service.md) |
| 11 | services/document/internal/logic/document/pull_main_service.go | [逐声明/函数带写](../../write/221-pull-main-service.md) |
| 12 | services/document/internal/logic/document/transactions_service.go | [逐声明/函数带写](../../write/243-transactions-service.md) |
| 13 | services/document/internal/handler/document/operations_handlers.go | [逐声明/函数带写](../../write/255-operations-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D04](./04-structure.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/operations_models.go',
    'services/document/internal/repository/document/discard_drafts_queries.go',
    'services/document/internal/repository/document/discard_previews_queries.go',
    'services/document/internal/repository/document/main_controls_queries.go',
    'services/document/internal/repository/document/operation_results_queries.go',
    'services/document/internal/repository/document/pull_main_queries.go',
    'services/document/internal/logic/document/control_manifest_validation.go',
    'services/document/internal/logic/document/discard_drafts_service.go',
    'services/document/internal/logic/document/discard_previews_service.go',
    'services/document/internal/logic/document/main_controls_service.go',
    'services/document/internal/logic/document/pull_main_service.go',
    'services/document/internal/logic/document/transactions_service.go',
    'services/document/internal/handler/document/operations_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
