# D08｜资源上传、配额、保护与真实对象存储

## 这节课要实现什么

用户上传图片并插入正文，希望保存后再次打开仍能看到；空间容量不足或文件无效时也要有明确结果。本课实现上传、资源引用和对象存储管理。

## 做完后能看到什么

接通后，完成核验的资源才可进入正文；上传中断、容量不足或无权限不会留下可随意读取的资源。图片在草稿或固定版本仍被引用时不能误删，暂时上传成功也不等于正文已保存。

## 先想清楚怎样实现

1. 先预留容量并验证上传。
2. 核对真实存储对象后标记可用。
3. 将临时保护交接给正文和版本引用，最后在无保护、无引用且满足清理条件时释放对象与容量。

## 怎样判断这一课完成

D14后运行资源规则，06实际上传、保存、重开和检查对象存储；测试内存替身不能证明图片已持久保存。

## 开始写之前

继续[D07](./07-publication.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。资源计量和保护回执先固定身份，文件适配器随后实现这份约定。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/resources_models.go**，用途是资源计量和保护回执先固定身份，文件适配器随后实现这份约定。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先定义计量、保护及文件消费者

resources_models.go 的DraftBodyMeasure保存实际规范正文Bytes/Hash；ParseDraftBodyMeasure要求规范十进制size、规范小写SHA-256、上限16MiB。ProtectionRecord固定Operation/Document/Room/Epoch/Generation、Grant、Fingerprint、ResourceJSON/SourceJSON；NodeUpdateProof保存真正Node持久更新的seq/hash/protection/grant及body计量。资源READY和正文已保存是两个事实，不能混用。

在resources_storage_service.go声明区先写ResourceFiles：

<!-- lesson-code: services/document/internal/logic/document/resources_storage_service.go :: ResourceFiles -->
````go
type ResourceFiles interface {
	Acquire(context.Context, string) (func(), error)
	AcquireRead(context.Context, string) (func(), error)
	CleanupUpload(context.Context, string) error
	Put(context.Context, string, io.Reader, uint64, [32]byte) error
	Open(context.Context, string, uint64, [32]byte) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
````
<!-- /lesson-code -->

Acquire/AcquireRead返回必须调用的释放函数；Put接收key、流、预期size/[32]byte hash；Open核size/hash并返回需Close的reader；CleanupUpload只清半文件；Delete删除真实对象。ctx用于取消等待和IO。接口属于消费方logic，local/OSS实现不需知道HTTP或SQL；文件写完不能擅自授予READY。

随后写resource_queries.go的Quota/Upload/Queryer、LockQuota、ReserveQuota、SettleQuota等完整声明。Quota把Used与Reserved分开；上传创建时持配额锁预留，完成确认才结算。used还包含初始正文和版本容量，不能断言“总used等于图片字节”。

## 3. 写真实存储，再编排上传

依次写local/store.go、objects_storage.go、各平台guard及backend.go。路径key受约束，不能让浏览器传任意主机文件名；临时写入核字节数/校验和并Sync，再独占发布。Acquire与清理共用进程间锁，取消或TTL到期不证明写者结束。Windows/Unix/unsupported文件保留build tags，不拼成一份跨平台文件。

OSS使用官方SDK及受保护凭据，credentials_windows.go/credentials_unix.go分别按系统保护读取，不能把密钥放公开示例。objects_storage.go核真实对象，legacy_snapshot.go只在可证明边界处理历史本地对象；BindBackend固定非秘密bucket/prefix身份，改变后拒绝启动，不能把原SQL key静默指向另一个桶。storage/snapshot/resources.go只在全部写者退出的冷边界复制真实对象及缺失事实，COMPLETE不代表MissingReady=0，更不能填假字节修复缺失。

在resources_storage_service.go输入WriteUpload完整方法：

<!-- lesson-code: services/document/internal/logic/document/resources_storage_service.go :: Resources.WriteUpload -->
````go
func (s *Resources) WriteUpload(ctx context.Context, actor Actor, id uint64, body io.Reader) (Actor, ReadBody, error) {
	release, err := s.HoldUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	defer release()
	// 等锁之前取得的状态不能用于继续写入；清理可能已经将原号关闭。
	u, access, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	if u.State != "UPLOADING" || !u.Expires.After(time.Now()) {
		return actor, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	if err := s.validateUpload(ctx, actor, access, u); err != nil {
		return actor, nil, err
	}
	// 持锁后可以清除前一进程遗留的半文件；预算在创建上传时已持久预留。
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return actor, nil, err
	}
	if err := s.files.Put(ctx, u.Key, body, u.Size, u.Checksum); err != nil {
		return actor, nil, err
	}
	// 文件落盘不授予READY，且写入期间的撤权必须在确认响应前重新核验。
	_, access, err = s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := validateUploadInTransaction(ctx, tx, access, u); err != nil {
			return nil, err
		}
		return resourceDTO(u), nil
	}
	return actor, run, nil
}
````
<!-- /lesson-code -->

顺序是取上传归属→Acquire→重读状态→确认仍UPLOADING且未过期→重验当前身份/ACL→清原半文件→Put真实字节→再核权限→返回原事务中的ReadBody。落盘本身不设置READY。完成确认另以同事务核上传revision、对象存在/校验和、配额预留，再标READY/结算。

例子：预留1000字节，上传了800字节就断开，Put必须失败，预留保留到有锁的恢复器持久关闭原号、删半文件并确认后才退款。若正在写1000字节时TTL过期，清理Acquire拿不到锁应等/跳过，不可立刻退款让另一上传复用预算。重复完成原操作不能重复加Used或重复退款。

## 4. 让引用先接住，再释放临时保护

resources_protection_service.go定义NodeOutcomeReader并验证真实grant、目标room/epoch、generation、更新hash及资源READY；先持久pin后才能允许Node保存引用。resource_finalize.go输入Finalize：

<!-- lesson-code: services/document/internal/logic/document/resource_finalize.go :: DraftResources.Finalize -->
````go
func (s *DraftResources) Finalize(ctx context.Context, in *pb.DraftResourceFinalize, release bool) (*pb.DraftResourceReceipt, error) {
	if in == nil {
		return nil, domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_FINALIZE_INVALID")
	}
	req, err := s.queryProtection(ctx, in.Query)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(req.UpdateHash, in.UpdateHash) {
		return nil, domain.RejectProtection(domain.ProtectionConflict, "UPDATE_HASH_CONFLICT")
	}
	proof, err := s.outcomes.UpdateOutcome(ctx, req)
	if err != nil {
		return nil, err
	}
	// 自报COMMITTED或seq不构成保存证据，必须逐项匹配实际Node日志的原保护、grant和hash。
	committed := proof.State == "COMMITTED" && proof.Protection == req.Context.OperationId && proof.Grant == req.GrantId && proof.Hash == hex.EncodeToString(req.UpdateHash) && proof.Seq == domain.TextID(in.DurableSeq) && in.NodeReceiptRef == domain.TextID(req.RoomId)+":"+domain.TextID(req.WorkspaceEpoch)+":"+proof.Seq
	if !committed || in.ProvenNodeOutcome != "COMMITTED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "NODE_OUTCOME_NOT_PROVEN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	space, lifecycle, err := repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" {
		return nil, domain.RejectProtection(domain.ProtectionForbidden, "DOCUMENT_NOT_CURRENT")
	}
	if err = repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId); err != nil {
		return nil, err
	}
	state, err := repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_UNKNOWN")
	}
	if state == "RELEASED" {
		if release {
			return protectionReceipt(req, state), nil
		}
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "PROTECTION_RELEASED")
	}
	if release && state != "CONFIRMED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "SAFE_TRANSFER_REQUIRED")
	}
	limit, err := resources.QuotaLimit(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	q, err := resources.LockQuota(ctx, tx, space, limit)
	if err != nil {
		return nil, err
	}
	if err = resources.SettleDraftBody(ctx, tx, q, req.Context.OperationId, proof.BodyBytes, proof.BodyHash); err != nil {
		return nil, err
	}
	ids := append([]uint64{}, req.ResourceIds...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		current, err := repository.LockedResourceState(ctx, tx, id)
		if err != nil || current != "READY" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
		owner := domain.TextID(req.RoomId) + ":" + domain.TextID(req.WorkspaceEpoch)
		if err = repository.InsertDraftResourceRef(ctx, tx, id, req.Context.Target.DocumentId, owner, req.Generation); err != nil {
			return nil, err
		}
	}
	// 同一事务先让DRAFT引用接住，再删临时pin；未知结果只多保留，不造成资源悬空。
	if err = repository.DeleteOperationResourceRefs(ctx, tx, req.Context.OperationId); err != nil {
		return nil, err
	}
	next := "CONFIRMED"
	if release {
		next = "RELEASED"
	}
	if err = repository.FinalizeResourceProtection(ctx, tx, req.Context.OperationId, next, in.NodeReceiptRef, in.DurableSeq); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_COMMIT_UNKNOWN")
	}
	return protectionReceipt(req, next), nil
}
````
<!-- /lesson-code -->

Finalize(ctx,in,release)不信调用者自报COMMITTED。它向Node查原UpdateOutcome，逐项匹配protection、grant、hash、durable_seq及receipt ref；在同一事务结算正文、写DRAFT引用，然后才删临时operation pin。提交未知只多保留保护，不能让正文引用悬空；release还要求已CONFIRMED的安全转移。

resources_reading_service.go/resource_conflict_context.go按MAIN/BRANCH/REVIEW/PUBLIC真实上下文证明某资源确在固定正文中，然后登记输出、持read guard打开字节。拥有resource_id不是下载资格。

用户选图→resources_handlers.go申请上传/PUT字节/完成→Resources→resource repository与真实local/OSS；正文插图UPDATE→04Node→Doc RPC保护/Finalize→Node持久日志→DRAFT引用。D14的NewResources注入同一storage backend，InitializeQuotas建立账户；RunResources分别恢复上传与无引用READY对象，资源冷快照入口是cmd/resourcesnapshot/main.go，不能在活跃实例随手运行。

## 5. 两层测试

独立存储包写齐即可先运行；logic命令需整个logic包齐：

~~~powershell
go -C services/document test -mod=readonly ./internal/storage/local -run '^(TestImmutableResourceLifecycle|TestReadGuardsExcludeCleanup|TestResourceGuardAndOrphanCleanup)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '本地对象与互斥规则失败' }
~~~

store_test.go/guard_test.go使用临时目录验证真实文件，但不访问业务MySQL或OSS。OSS的 TestOfficialSDKObjectRecovery 经过官方SDK的真实签名/编码，并用测试HTTP适配器替代外部网络；TestCloudOriginalObjectRecovery 需要显式云配置，不能当离线通过或为了通过抄体验实例凭据。

06 READY后：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/resources/upload.test.mjs tests/resources/content.test.mjs tests/resources/conflict-content.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实资源/正文保护失败' }
~~~

真实资源上传、幂等结算及在途过期清理互斥 应核并发锁和精确增量。checksum错误查真实上传字节；READY但无法读取查backend绑定/实际对象/上下文资格；预留不退先查写者及删除确认，不能手改quota为0。

## 6. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/resources_models.go | [逐声明/函数带写](../../write/099-resources-models.md) |
| 2 | services/document/internal/repository/document/resources_queries.go | [逐声明/函数带写](../../write/171-resources-queries.md) |
| 3 | services/document/internal/repository/resource/resource_queries.go | [逐声明/函数带写](../../write/187-resource-queries.md) |
| 4 | services/document/internal/logic/document/resource_conflict_context.go | [逐声明/函数带写](../../write/225-resource-conflict-context.md) |
| 5 | services/document/internal/logic/document/resource_finalize.go | [逐声明/函数带写](../../write/226-resource-finalize.md) |
| 6 | services/document/internal/logic/document/resources_protection_service.go | [逐声明/函数带写](../../write/227-resources-protection-service.md) |
| 7 | services/document/internal/logic/document/resources_reading_service.go | [逐声明/函数带写](../../write/228-resources-reading-service.md) |
| 8 | services/document/internal/logic/document/resources_storage_service.go | [逐声明/函数带写](../../write/229-resources-storage-service.md) |
| 9 | services/document/internal/storage/local/backend.go | [逐声明/函数带写](../../write/126-backend.md) |
| 10 | services/document/internal/storage/local/guard_unix.go | [逐声明/函数带写](../../write/127-guard-unix.md) |
| 11 | services/document/internal/storage/local/guard_unsupported.go | [逐声明/函数带写](../../write/128-guard-unsupported.md) |
| 12 | services/document/internal/storage/local/guard_windows.go | [逐声明/函数带写](../../write/129-guard-windows.md) |
| 13 | services/document/internal/storage/local/objects_storage.go | [逐声明/函数带写](../../write/130-objects-storage.md) |
| 14 | services/document/internal/storage/local/store.go | [逐声明/函数带写](../../write/131-store.md) |
| 15 | services/document/internal/storage/oss/credentials.go | [逐声明/函数带写](../../write/132-credentials.md) |
| 16 | services/document/internal/storage/oss/credentials_unix.go | [逐声明/函数带写](../../write/133-credentials-unix.md) |
| 17 | services/document/internal/storage/oss/credentials_windows.go | [逐声明/函数带写](../../write/134-credentials-windows.md) |
| 18 | services/document/internal/storage/oss/legacy_snapshot.go | [逐声明/函数带写](../../write/135-legacy-snapshot.md) |
| 19 | services/document/internal/storage/oss/objects_storage.go | [逐声明/函数带写](../../write/136-objects-storage.md) |
| 20 | services/document/internal/storage/oss/store.go | [逐声明/函数带写](../../write/137-store.md) |
| 21 | services/document/internal/storage/snapshot/resources.go | [逐声明/函数带写](../../write/138-resources.md) |
| 22 | services/document/internal/handler/document/resources_handlers.go | [逐声明/函数带写](../../write/259-resources-handlers.md) |
| 23 | services/document/cmd/resourcesnapshot/main.go | [逐声明/函数带写](../../write/281-main.md) |

## 7. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[D09](./09-comments-notifications.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/resources_models.go',
    'services/document/internal/repository/document/resources_queries.go',
    'services/document/internal/repository/resource/resource_queries.go',
    'services/document/internal/logic/document/resource_conflict_context.go',
    'services/document/internal/logic/document/resource_finalize.go',
    'services/document/internal/logic/document/resources_protection_service.go',
    'services/document/internal/logic/document/resources_reading_service.go',
    'services/document/internal/logic/document/resources_storage_service.go',
    'services/document/internal/storage/local/backend.go',
    'services/document/internal/storage/local/guard_unix.go',
    'services/document/internal/storage/local/guard_unsupported.go',
    'services/document/internal/storage/local/guard_windows.go',
    'services/document/internal/storage/local/objects_storage.go',
    'services/document/internal/storage/local/store.go',
    'services/document/internal/storage/oss/credentials.go',
    'services/document/internal/storage/oss/credentials_unix.go',
    'services/document/internal/storage/oss/credentials_windows.go',
    'services/document/internal/storage/oss/legacy_snapshot.go',
    'services/document/internal/storage/oss/objects_storage.go',
    'services/document/internal/storage/oss/store.go',
    'services/document/internal/storage/snapshot/resources.go',
    'services/document/internal/handler/document/resources_handlers.go',
    'services/document/cmd/resourcesnapshot/main.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
