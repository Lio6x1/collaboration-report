# D11｜输出登记、撤权与真实排空

## 这节课要实现什么

管理员撤销一个人的阅读权限时，对方可能正在下载文件或接收正文。本课实现输出登记与关闭，保证撤权完成意味着原输出实际停止。

## 做完后能看到什么

接通后，旧资格不能开启新的输出；已经开始的响应或连接要停止并得到确认，才完成关闭。请求发出、等待超时或租约到期都不能自动当成排空成功。

## 先想清楚怎样实现

1. 先为实际进程和响应登记所用权限来源。
2. 关闭时阻止旧来源继续准入。
3. 通知持有者停止真实输出，核对确认后再写入原关闭任务的结果。

## 怎样判断这一课完成

D14后运行关闭规则，06结合账号与协作服务在实际输出过程中撤权，检查连接停止及原结果恢复。

## 开始写之前

继续[D10](./10-exports.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。租约先声明来源、进程和handler身份，关闭过程才能准确找到真正需要停止的输出。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/outputs_models.go**，用途是租约先声明来源、进程和handler身份，关闭过程才能准确找到真正需要停止的输出。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 定义实际输出身份和关闭消费者

先在outputs_models.go写EditWorkspace/EditingAdmission/EditingTicket、FenceState、OutputInstance、LeaseSource、OutputLease、CollaborationOutput。OutputInstance.ID/PID/Boot同时辨认本次实例、操作系统进程和启动摘要，PID会复用，不能单凭PID断言原进程已退出。OutputLease还固定Handler、Sources、Document、Admission/Authority；PUBLIC输出单独记录PublicationEpoch/Release，用十进制字符串保全uint64，不伪造admission。

在output_control_service.go写小接口FencePeer、EdgeOutput、ProcessVerifier及LocalOutputs。FencePeer.Close返回pb.FenceResult和error；必须与原request摘要/参与者匹配。ProcessVerifier判断真正旧实例是否已退出，不能把查询失败当“不存在”；EdgeOutput调用受控infra协议关闭边缘段。

## 3. 先登记，再交付；先关闭准入，再排空输出

完成output_control_queries.go的实例、lease、source gate、关闭水位及ACK SQL。来源锁统一按固定顺序取得，旧准入与关闭器遵循gate→authorization_change，防止交叉持锁。输出登记在原业务事务中，Commit成功后才允许写正文；事务失败调用Abort。

在output_control_service.go定位Fences.Close，输入完整方法：

<!-- lesson-code: services/document/internal/logic/document/output_control_service.go :: Fences.Close -->
````go
// Close先关闭准入，再等待真实两段输出与Node ACK，持久确认之前不返回成功。
func (s *Fences) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	done, err := s.prepare(ctx, request)
	if err != nil {
		return nil, err
	}
	if done {
		return platform.BootstrapAck(request, "document"), nil
	}
	if err := s.outputs.Local().Stop(ctx, request.ClosedScopes[0]); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LOCAL_OUTPUT_STOP_UNKNOWN")
	}
	if err := s.stopEdgeAndSettle(ctx, request); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_STOP_UNKNOWN")
	}
	ack, err := s.node.Close(ctx, request)
	if err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "NODE_FENCE_UNAVAILABLE")
	}
	if err := s.finish(ctx, request, ack); err != nil {
		return nil, err
	}
	return platform.BootstrapAck(request, "document"), nil
}
````
<!-- /lesson-code -->

prepare先持久BLOCK来源、固定原fence绑定并关闭旧admissions；之后锁外等待LocalOutputs.Stop真实handler done、Edge缓冲段stop/settle、Node真实Close；finish持久记录原Node ACK摘要后才返回document ACK。同号不同scope/epoch/digest必须冲突。空在线handler集合也要把关闭水位发给Edge，防止提交前登记、提交后迟到open的输出复活。

具体推演：source=credential:7/epoch4正在下载，注销把来源关闭。发cancel只证明提出停止，LocalOutputs还要等待done；如果done在50ms后关闭，ACK只能在此后；等待超时返回unknown，不返回成功。租约expires到期也不证明旧进程退出、socket/内核缓冲排空或文件读取结束。

再输入RegisterCollaboration及其全部辅助：

<!-- lesson-code: services/document/internal/logic/document/output_control_service.go :: Outputs.RegisterCollaboration -->
````go
// 长连接来源从已兑换准入读取，不能接受Node自报更少的来源绕过关闭。
func (m *Outputs) RegisterCollaboration(ctx context.Context, request domain.CollaborationOutput) (string, error) {
	sources, err := m.repository.AdmissionSources(ctx, request.Admission)
	if err != nil {
		return "", err
	}
	edge, err := m.EdgeInstance(ctx, request.Handler)
	if err != nil || edge == nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return "", err
	}
	current, err := repository.LockedAdmission(ctx, tx, request.Admission)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return "", domain.Violation(domain.PermissionDenied, "ADMISSION_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != request.Grant || binding.Target == nil {
		return "", domain.Violation(domain.PermissionDenied, "GRANT_BINDING_INVALID")
	}
	if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
		return "", err
	}
	lease := domain.OutputLease{ID: platform.NewTraceID(), Instance: edge.ID, Handler: request.Handler, Sources: SourcesWire(sources), Document: binding.Target.DocumentId, Admission: request.Admission, Authority: "collaboration"}
	if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
		return "", err
	}
	if err := repository.MarkWebSocketDelivery(ctx, tx, lease.ID, current.Expires); err != nil {
		return "", err
	}
	if tx.Commit() != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	// Node自己的socket停止不足以证明Edge缓冲排空；开放也必须取得真实Edge确认。
	var ack struct {
		State string `json:"state"`
		ID    string `json:"instance_id"`
	}
	if m.edge.Call(ctx, "/infra/output/open", lease, &ack) != nil || ack.State != "OPEN" || ack.ID != edge.ID {
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return lease.ID, nil
}
````
<!-- /lesson-code -->

长连接Sources从原已兑换admission读取，不让Node自报更少来源；事务内核admission仍OPEN、grant/target匹配、Edge实例真实，保存lease并MarkWebSocketDelivery。Commit后Edge需返回同实例OPEN确认；响应未知不能当作没登记，原lease仍需要恢复/关闭。

## 4. 将Go、Edge、Node接成两段输出

client/edge_output.go实现真实Edge infra调用；process_owner.go加平台process/probe文件验证实例归属，Windows判断不能弹控制台；所有build tags保留。handler/outputs_handlers.go给每个响应登记handler生命周期，在正常结束或abort时释放；server/dependency_error.go把依赖未知映射为稳定状态，rpc_server.go实现DocumentInternal生成接口，把fence、资源保护、原工作流查询交给真实logic。

用户退出/撤权→Account或Doc原关闭任务→DocumentInternal/Fences.Close→Doc本地handler排空→Edge输出段排空→Node ACK→Doc持久ACK→上游完成。WSS断开界面只是这一链某个外在现象，不能替代持久ACK；对PUBLIC下载同样按publication来源关闭。D14的NewOutputs/NewFences、真实Edge与Node客户端、identity_notifications_workers.go的RunAuthorizations把整条链连起来。

## 5. 测试与诊断

包齐后离线：

~~~powershell
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestLocalFenceWaitsForActualHandler|TestLocalFenceTimeoutIsUnknown)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '输出排空规则失败' }
~~~

output_test.go显式控制stop与done两个channel；第一个确保stop发出但done未结束时没有ACK，第二个确保超时是未知。它不证明Edge/Node实际关闭。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/workspace/output.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实输出关闭失败' }
~~~

logout closes real document admissions, read views and both persisted output segments 核真实两段输出及准入；untrusted browser proxy identities are stripped and an invented direct handler cannot release content 核浏览器无法伪造内部身份。FENCE_*_UNKNOWN查原scope/epoch、handler done、Edge实例、Node ACK分别到哪步；PROCESS_OWNER_MISMATCH保持现场并核入口/boot，不能批量终止所有node.exe或把超时当成功。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/outputs_models.go | [逐声明/函数带写](../../write/097-outputs-models.md) |
| 2 | services/document/internal/repository/document/output_control_queries.go | [逐声明/函数带写](../../write/163-output-control-queries.md) |
| 3 | services/document/internal/logic/document/output_control_service.go | [逐声明/函数带写](../../write/215-output-control-service.md) |
| 4 | services/document/internal/client/edge_output.go | [逐声明/函数带写](../../write/110-edge-output.md) |
| 5 | services/document/internal/client/process_other.go | [逐声明/函数带写](../../write/111-process-other.md) |
| 6 | services/document/internal/client/process_owner.go | [逐声明/函数带写](../../write/112-process-owner.md) |
| 7 | services/document/internal/client/process_probe_linux.go | [逐声明/函数带写](../../write/113-process-probe-linux.md) |
| 8 | services/document/internal/client/process_probe_other.go | [逐声明/函数带写](../../write/114-process-probe-other.md) |
| 9 | services/document/internal/client/process_probe_windows.go | [逐声明/函数带写](../../write/115-process-probe-windows.md) |
| 10 | services/document/internal/client/process_windows.go | [逐声明/函数带写](../../write/116-process-windows.md) |
| 11 | services/document/internal/handler/document/outputs_handlers.go | [逐声明/函数带写](../../write/256-outputs-handlers.md) |
| 12 | services/document/internal/server/dependency_error.go | [逐声明/函数带写](../../write/269-dependency-error.md) |
| 13 | services/document/internal/server/rpc_server.go | [逐声明/函数带写](../../write/270-rpc-server.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D12](./12-trash-transfer.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/outputs_models.go',
    'services/document/internal/repository/document/output_control_queries.go',
    'services/document/internal/logic/document/output_control_service.go',
    'services/document/internal/client/edge_output.go',
    'services/document/internal/client/process_other.go',
    'services/document/internal/client/process_owner.go',
    'services/document/internal/client/process_probe_linux.go',
    'services/document/internal/client/process_probe_other.go',
    'services/document/internal/client/process_probe_windows.go',
    'services/document/internal/client/process_windows.go',
    'services/document/internal/handler/document/outputs_handlers.go',
    'services/document/internal/server/dependency_error.go',
    'services/document/internal/server/rpc_server.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
