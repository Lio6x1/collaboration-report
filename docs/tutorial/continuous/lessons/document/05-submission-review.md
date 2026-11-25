# D05｜可靠送审与固定轮次

## 这节课要实现什么

协作者把当前已保存的修改交给审核者，但送审后还可能继续写。本课实现固定一份内容形成审核轮次，让审核不会随支线继续编辑而变化。

## 做完后能看到什么

接通后，审核看到的是本轮捕获的固定正文，之后保存的内容留给下一轮；没有正文变化时不创建空审核。撤回、要求修改和重新送审都对应明确轮次。

审核请求、历史轮次和修改要求各显示20条；翻页只替换相应列表，已经选定的请求、固定轮次正文及尚未提交的修改要求/冲突处理草稿保持原对象，不会被新页自动替换。

## 先想清楚怎样实现

1. 先验证送审输入与保存证明。
2. 在首次捕获时固定正文。
3. 事务内保存版本、轮次、引用和回执，再登记收尾与通知任务。
4. 补齐轮次读取和短审核动作。

## 怎样判断这一课完成

D14后执行送审规则，06结合C04在送审前后继续编辑，确认捕获前的已保存修改会纳入本轮、捕获后的修改不会混入。

## 开始写之前

继续[D04](./04-structure.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。送审输入、固定正文期待和轮次规则先定义，消费者才知道要核哪些真实事实。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/reviews_models.go**，用途是送审输入、固定正文期待和轮次规则先定义，消费者才知道要核哪些真实事实。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 从生成union解析真实送审输入

先写reviews_models.go的SubmissionIntent、ParseSubmission、CaptureExpectation、ReviewPointer/ReviewGuard及全部纯规则。SubmissionIntent.Operation标识本次业务操作；Flush必须是协作服务实际FLUSH回执；Mode只有NEW或RESUBMIT；Branch/Structure绑定原支线和目录；RESUBMIT额外绑定Request及ExpectedRound；Canonical保留规范公开输入供指纹核对。

<!-- lesson-code: services/document/internal/domain/document/reviews_models.go :: ParseSubmission -->
````go
func ParseSubmission(input dto.Submission) (SubmissionIntent, error) {
	var intent SubmissionIntent
	invalid := func() (SubmissionIntent, error) { return intent, Violation(InvalidRequest, "SUBMISSION_INVALID") }
	value, err := codec.Decode(input)
	if err != nil {
		return invalid()
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return invalid()
	}
	text := func(key string) string { value, _ := fields[key].(string); return value }
	intent.Operation, intent.Flush, intent.Mode = text("operation_id"), text("flush_receipt_id"), text("mode")
	if !ValidOperationID(intent.Operation) || !ValidOperationID(intent.Flush) {
		return invalid()
	}
	if intent.Mode != "NEW" && intent.Mode != "RESUBMIT" {
		return invalid()
	}
	allowed := map[string]bool{"operation_id": true, "flush_receipt_id": true, "mode": true, "branch_id": true, "expected_structure_id": true}
	if intent.Mode == "RESUBMIT" {
		allowed["request_id"], allowed["expected_round_id"] = true, true
	}
	if len(fields) != len(allowed) {
		return invalid()
	}
	for key := range fields {
		if !allowed[key] {
			return invalid()
		}
	}
	intent.Branch, err = CanonicalID(text("branch_id"))
	if err != nil {
		return invalid()
	}
	intent.Structure, err = CanonicalID(text("expected_structure_id"))
	if err != nil {
		return invalid()
	}
	if intent.Mode == "RESUBMIT" {
		intent.Request, err = CanonicalID(text("request_id"))
		if err != nil {
			return invalid()
		}
		intent.ExpectedRound, err = CanonicalID(text("expected_round_id"))
		if err != nil {
			return invalid()
		}
	}
	intent.Canonical, err = codec.Canonical(fields)
	if err != nil {
		return invalid()
	}
	return intent, nil
}
````
<!-- /lesson-code -->

dto.Submission是生成的union表示，所以解析时检查字段集合、类型、规范ID与NEW/RESUBMIT分支。NEW带request_id或浏览器夹带body_json都拒绝，正文只能来自协作服务固定输入。

在submissions_service.go的Submissions声明前，写消费者接口：

<!-- lesson-code: services/document/internal/logic/document/submissions_service.go :: SubmissionCaptureClient -->
````go
type SubmissionCaptureClient interface {
	Capture(context.Context, *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error)
	Finalize(context.Context, *pb.FinalizeCaptureRequest) (*pb.OperationResult, error)
}
````
<!-- /lesson-code -->

Capture(ctx, CaptureSubmissionRequest)返回固定捕获状态/正文，Finalize确认Doc已持久接住原捕获。两次调用不是同一事务；超时不能推断Node没固定，也不能立即释放原guard重开任务。

## 3. 注册、捕获、提交、收尾分四步写

先输入submissions_queries.go中的原操作/guard/task/round SQL，再输入reviews_queries.go的当前pointer、历次submitter和change查询。轮次只追加；request保存当前轮次指针，revision只负责并发，不能让旧round页面“自动升级”为新一轮授权。

在Submissions构造前后的操作页定位Prepare；完整输入它，然后依次写RecoverOne、commitSubmission、finalizeSubmission和失败分类。Prepare返回actor、MutationBody、MutationPresentation，短事务固定actor/会话/来源/target/原workspace/guard generation并排恢复任务；worker在锁外调用Capture。

<!-- lesson-code: services/document/internal/domain/document/reviews_models.go :: ValidateSubmissionCapture -->
````go
// Go校验固定业务JSON与来源，不解析Yjs。错误依赖结果不能被当成用户操作失败释放guard。
func ValidateSubmissionCapture(validator *codec.Validator, content *pb.FixedContent, expected CaptureExpectation) ([]uint64, error) {
	invalid := func() ([]uint64, error) { return nil, Violation(TemporarilyUnavailable, "CAPTURE_CONTENT_INVALID") }
	if content == nil || expected.Request == nil || expected.Request.Context == nil || expected.Request.Context.Target == nil || validator == nil {
		return invalid()
	}
	context, target := expected.Request.Context, expected.Request.Context.Target
	if content.DocumentId != target.DocumentId || content.UnitId != target.UnitId || content.BranchId != target.BranchId ||
		content.StructureId != target.StructureId || content.WorkspaceEpoch != target.WorkspaceEpoch || content.LineageRef != TextID(expected.Workspace.Lineage) ||
		content.OperationId != context.OperationId || content.CaptureId != context.OperationId || content.SchemaVersion != "content-v1" ||
		content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" || content.ProvenanceSchema != "provenance-v1" ||
		len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
		return invalid()
	}
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) {
		return invalid()
	}
	body, err := canonicalValue(content.BodyJsonUtf8)
	if err != nil || validator.Body(body) != nil {
		return invalid()
	}
	cutValue, err := canonicalValue(content.CutJsonUtf8)
	if err != nil {
		return invalid()
	}
	cut, ok := cutValue.(map[string]any)
	if !ok || len(cut) != 3 || cut["room_id"] != TextID(expected.Room) || cut["workspace_epoch"] != TextID(expected.Workspace.Epoch) {
		return invalid()
	}
	sequence, ok := cut["durable_seq"].(string)
	if !ok {
		return invalid()
	}
	if sequence != "0" {
		if _, err := CanonicalID(sequence); err != nil {
			return invalid()
		}
	}
	provenanceValue, err := canonicalValue(content.ProvenanceJsonUtf8)
	if err != nil || validator.Provenance(provenanceValue) != nil {
		return invalid()
	}
	provenance := provenanceValue.(map[string]any)
	if provenance["lineage_id"] != TextID(expected.Workspace.Lineage) || provenance["base_manifest_id"] != TextID(expected.Workspace.BaseManifest) ||
		provenance["base_content_version_id"] != TextID(expected.Workspace.BaseVersion) || provenance["source_operation_id"] != context.OperationId {
		return invalid()
	}
	originalValue, err := codec.Decode(expected.LineageProvenance)
	if err != nil || validator.Provenance(originalValue) != nil {
		return invalid()
	}
	original := originalValue.(map[string]any)
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return invalid()
	}
	originalAccepted, err := codec.Canonical(original["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, originalAccepted) {
		return invalid()
	}
	resources, blocks := map[uint64]bool{}, map[string]bool{}
	count := 0
	var inspect func(map[string]any, int) bool
	inspect = func(node map[string]any, depth int) bool {
		count++
		if depth > 64 || count > 100000 {
			return false
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return false
				}
				blocks[id] = true
			}
			if node["type"] == "image" {
				text, ok := attrs["resourceId"].(string)
				if !ok {
					return false
				}
				id, err := CanonicalID(text)
				if err != nil {
					return false
				}
				resources[id] = true
			}
		}
		if marks, ok := node["marks"].([]any); ok {
			for _, mark := range marks {
				if mark.(map[string]any)["type"] == "font" {
					return false
				}
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				if !inspect(child.(map[string]any), depth+1) {
					return false
				}
			}
		}
		return true
	}
	if !inspect(body.(map[string]any), 0) {
		return invalid()
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != len(content.ResourceIds) {
		return invalid()
	}
	for index, id := range ids {
		if id != content.ResourceIds[index] {
			return invalid()
		}
	}
	return ids, nil
}
````
<!-- /lesson-code -->

这个检查把正文和原请求逐项相连：document/unit/branch/structure/epoch、lineage、operation/capture、schema/canonicalization/hash版本、实际字节数及SHA-256、真实cut/provenance/resource集合。JSON通过schema只表示形状合法；另一支线的合法JSON仍必须拒绝。16MiB正文上限和gRPC大消息配置必须同时保留，不能截断到默认4MiB后算“成功”。

具体输入：FLUSH证明此前更新已持久；首次Capture写入CUT_FIXED时固定S1，随后用户继续写并收到S2的SAVED，审核轮次仍保存S1。FLUSH本身不固定送审正文：若S2在首次Capture前已持久，首次Capture会取当时的last_durable_seq，将S2纳入该轮。commitSubmission持operation gate、guard及来源锁复核，插入不可变version和资源引用、创建/追加round、保存原style/title/cut、完成原receipt，同时登记CAPTURE_FINALIZE和REVIEW_NOTIFICATION，然后Commit。S2留在支线供下一轮。NEW若正文与真实MAIN的BRANCH投影hash相同，报SUBMISSION_NO_CHANGES，不能造空审核。MAIN字体会在支线基线投影中去掉，所以字体差异不冒充正文修改。

Capture明确拒绝无效FLUSH可让原任务REJECTED并释放自己的guard；网络超时、损坏依赖响应、提交未知必须保留原号与占用，由恢复器查询。不能因一个503新开另一次送审。

## 4. 写短审核动作并接线

reviews_service.go完整处理RequestChanges、Withdraw、Resolve及固定轮次读。ValidateReviewRound要求当前非归档REVIEWING轮次；withdraw保留修改意见，重新送审后旧轮次动作返回REVIEW_ROUND_CHANGED。解除意见只允许管理者，或从未提交过该请求且本人提出意见的reviewer；后来得到REVIEW也不会洗掉曾提交身份。

用户点击送审→reviews_handlers.go中的Submissions.Create→ParseSubmission→Submissions.Prepare→Transactions.Mutation→恢复worker→真实SubmissionCaptures→commitSubmission；页面轮询原operation得到request_id、round_id、content_version_id。D14用NewSubmissions注入validator、HTTP/mTLS协作客户端和实例ID，RunSubmissions驱动恢复及Finalize。普通批注入口在D09，不写正式change_request表。

### 4.1 把20条展示页与完整审核资格分开

前端复用 `web/src/features/review/api.ts`、`useReviews.ts`、`ReviewPanel.vue`，以及公共游标状态/分页控件；本课继续亲写上述后端事务，不再为页面另造一套审核状态。已有请求、轮次、修改要求GET分别在 `/v1/documents/:documentId/review-requests`、`/v1/review-requests/:requestId/rounds`、`/v1/review-requests/:requestId/change-requests` 接受 `limit=20/cursor`。三种游标各属自己的列表，不能共享扫描位置。

下一页替换当前20条，上一页也重新HTTP核当前权限；只有一页时隐藏分页栏，换文档/请求或会话立即重置相关页并丢弃旧响应。请求列表翻页不清除已选请求详情；轮次列表翻页不把 `selectedRound/fixed` 改成新页第一项；修改要求翻页不清掉正在输入的说明。固定选择和写入输入继续绑定原请求/轮次，不能因为当前页没显示它就偷偷换目标。

初次打开或原请求归档后，`findDefaultRequest` 单独扫描有权请求元数据，最多20页、每页100条，定位当前未归档请求；这不是把展示页扩大到100条。当前请求落在第21条时仍精确读取它的详情，展示页保持20条。重复游标或预算耗尽时明确提示手动选择，不拿已扫描的末条冒充全局当前请求，也不先读取任意候选正文。手动刷新请求、轮次或修改要求时各自清除过期游标链回首页，保留仍有效的固定选择和本机意见。

资格判断有独立读取：非管理者是否“从未提交过该请求”，必须从原请求的轮次首页开始，直到找到本人提交记录，或确认整个历史结束；不能用当前20条里没有本人来放开解除意见。完整核验未结束或失败时保持未知并关闭依赖该判断的入口。修改要求的整体阻塞数来自当前服务端请求事实，不能用本页 `changes.length` 或本页未解决项数改写。以上读取是为了一个明确的权限事实，与让用户按页浏览历史分开；后端短动作仍会再次核验当前资格和原轮次。

## 5. 现有测试

复用前端支持后可先跑离线组件行为：

~~~powershell
Set-Location -LiteralPath $lessonRoot
node --no-maglev --test --test-concurrency=1 tests/unit/history-pagination.test.mjs
if ($LASTEXITCODE -ne 0) { throw '审核与历史分页组件行为失败' }
~~~

其中审核用例用替换的HTTP响应驱动真实组件逻辑，验证20条替换、固定选择和两类本机草稿保留、本人提交资格独立核完后页，以及全请求阻塞数不随本页改变。它不调用真实协作捕获或数据库，不能替代下方送审/审核接口测试。翻页后选中对象消失先查是否错误地随列表清空detail；错误放开解除入口先查完整本人提交历史是否仍未知，不能降低后端资格断言来迁就页面。

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/domain/document -run '^(TestSubmissionInputHasCanonicalBusinessIdentity|TestSubmissionCaptureRejectsSchemaValidBranchViolations|TestSubmissionCaptureRequiresExactFixedBytesAndBinding)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '上一组文档规则检查失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestReviewSummaryUsesFixedRoundWithinOriginalReadTransaction|TestReviewSummaryRejectsUnauthorizedAndMissingFixedRound)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '固定送审规则失败' }
~~~

测试源为submission_test.go、submission_capture_test.go、reviews_summary_test.go；应RUN/PASS且未使用真实Node。重点是另一支线的合法捕获被拒、摘要仍在原读事务中从原round取得。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/review/submission.test.mjs tests/review/actions.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实送审/审核失败' }
~~~

actual HTTP submission creates one fixed round and replays the original result 必须只新增一轮；real withdraw and resubmit preserve blocking changes and reject actions on the previous round 必须保留意见并拒绝旧round。FLUSH无效查actor/room/epoch；PREPARING查原捕获与worker，不能只看HTTP202；4MiB附近失败核双端消息上限，不能删大正文测试。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/reviews_models.go | [逐声明/函数带写](../../write/101-reviews-models.md) |
| 2 | services/document/internal/repository/document/reviews_queries.go | [逐声明/函数带写](../../write/174-reviews-queries.md) |
| 3 | services/document/internal/repository/document/submissions_queries.go | [逐声明/函数带写](../../write/181-submissions-queries.md) |
| 4 | services/document/internal/logic/document/reviews_service.go | [逐声明/函数带写](../../write/234-reviews-service.md) |
| 5 | services/document/internal/logic/document/submissions_service.go | [逐声明/函数带写](../../write/242-submissions-service.md) |
| 6 | services/document/internal/handler/document/reviews_handlers.go | [逐声明/函数带写](../../write/261-reviews-handlers.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D06](./06-merge-conflict.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/reviews_models.go',
    'services/document/internal/repository/document/reviews_queries.go',
    'services/document/internal/repository/document/submissions_queries.go',
    'services/document/internal/logic/document/reviews_service.go',
    'services/document/internal/logic/document/submissions_service.go',
    'services/document/internal/handler/document/reviews_handlers.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
