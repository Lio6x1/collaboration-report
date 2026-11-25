# S04｜先封存L，再导入签名快照并种子回填

## 这节课要实现什么

需要更换或重建索引时，不能凭一个任意时间导出的文件覆盖正在使用的搜索数据。本课实现重建准备、固定快照导入和新索引初始回填。

## 做完后能看到什么

接通后，新索引从已验证快照开始，原日志起点和任务身份保持固定；损坏或不属于本任务的制品被拒绝。当前查询仍使用旧索引，本课不切换搜索入口。

## 先想清楚怎样实现

1. 先封存需要后续重放的日志起点。
2. 核验文档服务签名快照并受理同一个重建任务。
3. 逐项向新索引回填当前权威内容，同时持久保存进度。

## 怎样判断这一课完成

S05包齐后检查规则，06按本课真实Prepare、Import和回填顺序执行；完成后保留原任务给S05继续，不能另建任务绕过失败。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/search/internal/repository/rebuild/preparation_queries.go` | 持久保存prepare、封存三分区frontier、受理job和原验证进度；统一generation→binding→job锁序。 | [逐段带写](../../write/390-preparation-queries.md) |
| 2 | `services/search/internal/repository/rebuild/installation_queries.go` | 保存导入seal、摘要、原制品和快照进度；进度与种子实际应用同事务推进。 | [逐段带写](../../write/389-installation-queries.md) |
| 3 | `services/search/internal/logic/rebuild/preparation_service.go` | 实现Prepare、Start/Get、屏障与协调器：先固定原日志L和目标代次，原号重放原envelope。 | [逐段带写](../../write/398-preparation-service.md) |
| 4 | `services/search/internal/logic/rebuild/installation_service.go` | 完整验证Doc签名seal及entries流，先发布完整文件再接受SQL，按原任务分批回填目标。 | [逐段带写](../../write/397-installation-service.md) |
| 5 | `services/search/cmd/search-rebuild-prepare/main.go` | 独立CLI只加载Search自己的配置，导出已提交prepare到受控outbox。 | [逐段带写](../../write/409-main.md) |
| 6 | `services/search/cmd/search-rebuild-import/main.go` | 独立CLI从Search受控inbox验证并导入Doc快照引用，不读Doc私有数据库。 | [逐段带写](../../write/408-main.md) |

## 2. 先把L、S、H三个边界分开

重建不是清空旧索引再重跑所有消息。L是Search先封存的每分区下一末端offset，S是Doc在看到原prepare以后封存的完整条目快照，H是稍后切换屏障封存的有限末端。先L再S保证扫描期间变化能由[L,H)覆盖；H不能随不断发布无限后移。三个分区各自有offset，没有跨分区总序。

例如L=[100,200,300]。Doc扫描S期间partition0收到100与101，快照可能已经含其中某个文档更新，也可能不含；先seed当前Doc权威，再从原L重放，按文档水位/摘要幂等覆盖，不遗漏。若先S后L，S扫描后至L之间发生更新，会既没入快照也没被重放，这是必须保留Prepare顺序的原因。

本课 `preparation_service.go` 同时保存最终Barrier/Coordinator声明，引用S05的Replayer/终态函数；`installation_service.go` 与它属于同一rebuild包。本课先按操作页写真实声明，等S05同包完整再build，不删掉前向引用、不填假函数体。S01-S03独立包仍可验证。

## 3. Prepare的模型、接口与数据库事务

先在repository/preparation_queries写Binding、Find、CheckAvailable、Insert，再写Frontier、Job/JobBinding、AcceptJob与验证进度；这些SQL全部使用外层tx。binding保存原prepare envelope及SignerFingerprint，job保存source/target/generation/snapshot_ref/state/snapshot_after。锁序统一generation→binding→job，不能持共享锁后升级为排他。

Preparer字段DB、Logs、Aliases、Signer/LoadSigner、Verifier、Owner/Alias。Logs.Capture返回原Cluster/TopicUUID/三分区Window；Aliases.ResolveAlias核当前真实指向。`Prepare(ctx,operation,target)` 返回已签名envelope字节与error。新操作要求target=source.Generation+1，source/revision未溢出且无SWITCHING/另一个在途任务，ES alias与SQL源一致。捕获Window之后将Starts固定为各partition.Last，保存nonce、source/revision、确定target physical名称，签名后先自验再Insert并COMMIT。

<!-- lesson-code: services/search/internal/logic/rebuild/preparation_service.go :: Preparer -->
````go
type Preparer struct {
	DB           *sql.DB
	Logs         Logs
	Aliases      Aliases
	Signer       *rebuildartifact.Signer
	LoadSigner   func() (*rebuildartifact.Signer, error)
	Verifier     *rebuildartifact.Verifier
	Owner, Alias string
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/logic/rebuild/preparation_service.go :: Preparer.Prepare -->
````go
// Prepare先封存Kafka原L再允许Doc建立S。代次锁只覆盖有界准备，不跨Doc的整库扫描。
func (p *Preparer) Prepare(ctx context.Context, operation string, target uint64) ([]byte, error) {
	if p.DB == nil || p.Logs == nil || p.Aliases == nil || !rebuildmeta.OperationValid(operation) || target == 0 {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := p.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	source, exists, err := generation.Find(ctx, tx, p.Alias, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("SEARCH_GENERATION_MISSING")
	}
	original, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if found {
		if original.Prepare.Owner != p.Owner || original.Prepare.Alias != p.Alias || original.Prepare.TargetGeneration != strconv.FormatUint(target, 10) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		if original.State == "REJECTED" {
			return nil, errors.New("SEARCH_REBUILD_PREPARE_REJECTED")
		}
		return original.Envelope, nil
	}
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || target != source.Generation+1 {
		return nil, errors.New("SEARCH_REBUILD_GENERATION_INVALID")
	}
	if err := generation.CheckSwitching(ctx, tx); err != nil {
		return nil, err
	}
	if err := repository.CheckAvailable(ctx, tx, p.Alias); err != nil {
		return nil, err
	}
	actual, present, err := p.Aliases.ResolveAlias(ctx, p.Alias)
	if err != nil {
		return nil, err
	}
	if !present || actual != source.Physical {
		return nil, errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
	}
	window, err := p.Logs.Capture(ctx, nil)
	if err != nil {
		return nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errors.New("SEARCH_REBUILD_NONCE_UNAVAILABLE")
	}
	name := sha256.Sum256([]byte(p.Owner + "\x00" + p.Alias + "\x00" + operation))
	input := rebuildmeta.Prepare{Version: 1, Operation: operation, Owner: p.Owner, Alias: p.Alias, Source: source.Physical, SourceGeneration: strconv.FormatUint(source.Generation, 10), SourceRevision: strconv.FormatUint(source.Revision, 10),
		Target: "rebuild-" + hex.EncodeToString(name[:]), TargetGeneration: strconv.FormatUint(target, 10), Cluster: window.Identity.Cluster, Topic: window.Identity.Topic, TopicID: hex.EncodeToString(window.Identity.TopicID[:]), Nonce: hex.EncodeToString(nonce[:])}
	for i, part := range window.Partitions {
		input.Starts[i] = strconv.FormatInt(part.Last, 10)
	}
	payload, err := input.Bytes()
	if err != nil {
		return nil, err
	}
	signer := p.Signer
	if signer == nil && p.LoadSigner != nil {
		signer, err = p.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil || p.Verifier == nil {
		return nil, errors.New("SEARCH_REBUILD_SIGNER_UNAVAILABLE")
	}
	envelope, err := signer.Sign(rebuildartifact.PrepareV1, payload)
	if err != nil {
		return nil, err
	}
	verified, err := p.Verifier.Verify(envelope, rebuildartifact.PrepareV1)
	if err != nil {
		return nil, err
	}
	if err := repository.Insert(ctx, tx, input, envelope, verified.SignerFingerprint); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN")
	}
	return envelope, nil
}
````
<!-- /lesson-code -->

同operation再次Prepare读取原envelope，不再次Capture；即使Kafka末端已变102/203/304，原L仍100/200/300。同号换target是 `SEARCH_REBUILD_OPERATION_CONFLICT`。COMMIT未知返回 `SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN`，保持原operation查询/重试；LoadSigner仅用于新任务，已接受原prepare恢复不因当前叶证书过期而被重新签名。

## 4. Import完整验证后才受理，不让SQL指向半文件

先写installation仓储Imported/FindImport/SaveImport/PendingSnapshot/AdvanceSnapshot/FinishSnapshot，再写Importer。参数 `Import(ctx,operation,reference)` 只接受内容地址 `rebuild-snapshot-v1:<摘要>` 对应的snapshot.json，受控Inbox/Accepted来自本服务根目录。先短事务检查原binding和已imported；锁外读Doc签名seal并核PrepareRef，entries.ndjson必须是普通文件，完整流Scan计数和摘要与seal一致。

<!-- lesson-code: services/search/internal/logic/rebuild/installation_service.go :: Importer.Import -->
````go
// Import先恢复原绑定，再在事务外验证并发布完整流，最后短事务复核原绑定后接受。
// 文件先于SQL发布：崩溃最多留下内容地址孤儿，不能留下指向半份文件的IMPORTED。
func (s *Importer) Import(ctx context.Context, operation, reference string) ([]byte, error) {
	name, err := rebuildartifact.FileName(reference)
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || err != nil || !strings.HasSuffix(name, ".snapshot.json") || s.Inbox == "" || s.Accepted == "" {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	binding, original, found, err := s.inspect(ctx, operation)
	if err != nil {
		return nil, err
	}
	var row repository.Imported
	if found {
		actual, _ := rebuildartifact.Reference(original.Envelope, rebuildartifact.SnapshotV1)
		if actual != reference {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		row = original
	} else {
		raw, err := rebuildartifact.ReadFile(s.Inbox, reference)
		if err != nil {
			return nil, err
		}
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		verified, err := s.Verifier.Verify(raw, rebuildartifact.SnapshotV1)
		if err != nil {
			return nil, err
		}
		row.Seal, err = rebuildmeta.ParseSeal(verified.Payload)
		if err != nil {
			return nil, err
		}
		row.Envelope = raw
		row.Digest = sha256.Sum256(raw)
		signer, err := hex.DecodeString(verified.SignerFingerprint)
		if err != nil || len(signer) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		copy(row.SignerDigest[:], signer)
	}
	prepareRef, _ := rebuildartifact.Reference(binding.Envelope, rebuildartifact.PrepareV1)
	if row.Seal.PrepareRef != prepareRef {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	entriesName := strings.TrimSuffix(name, ".snapshot.json") + ".entries.ndjson"
	// 已接受副本优先恢复；丢失时允许从相同inbox重建，但仍核完整原摘要，绝不重签。
	inputDir := s.Inbox
	if found {
		if _, err := os.Lstat(filepath.Join(s.Accepted, entriesName)); err == nil {
			inputDir = s.Accepted
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
	}
	if err := rebuildartifact.PublishFile(s.Accepted, entriesName, func(w io.Writer) error {
		info, err := os.Lstat(filepath.Join(inputDir, entriesName))
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		f, err := os.Open(filepath.Join(inputDir, entriesName))
		if err != nil {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		defer f.Close()
		count, digest, err := rebuildmeta.Scan(io.TeeReader(&importReader{ctx: ctx, reader: f}, w), nil)
		if err != nil {
			return err
		}
		if strconv.FormatUint(count, 10) != row.Seal.Count || hex.EncodeToString(digest[:]) != row.Seal.EntriesDigest {
			return errors.New("SEARCH_REBUILD_ENTRIES_MISMATCH")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := rebuildartifact.PublishFile(s.Accepted, name, func(w io.Writer) error { _, err := io.Copy(w, bytes.NewReader(row.Envelope)); return err }); err != nil {
		return nil, err
	}
	// 全流IO已退出；此处只复核不可变绑定，不在代次锁内重扫文件。
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(short, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, accepted, has, err := s.lock(short, tx, operation)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(binding.Envelope, current.Envelope) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if has {
		if !bytes.Equal(accepted.Envelope, row.Envelope) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		return accepted.Envelope, nil
	}
	if err := repository.SaveImport(short, tx, operation, row); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN")
	}
	return row.Envelope, nil
}
````
<!-- /lesson-code -->

完整文件先发布到Accepted，再短事务重核原binding并SaveImport。若进程在发布文件后、SQL前退出，最多留下内容地址孤儿；反过来会产生指向半文件的IMPORTED，不能这么写。原accepted副本遗失可以从同摘要Inbox重生，但仍完整核摘要，不能重签或接受不同内容。stream尾坏即拒绝，不能在Scan回调中提前写ES后才发现最终hash错。

`JobService.StartRebuild` 接生成pb.RebuildRequest的OperationId/Generation/SnapshotRef/WatermarkJsonUtf8，RequestFingerprint由完整descriptor算。它核import已受理、原prepare/source/revision、owner/alias、受控普通entries文件可用，然后AcceptJob并为独立consumer=`rebuild-<jobID>`写0/1/2三检查点=L。GetRebuild的pb.OperationQuery必须原fingerprint，Target只能空，不借文档target改变整站重建含义。PROCESSING返回current state/after；state字符串SUCCEEDED本身还不算成功，S05必须有固定终态proof。

## 5. 种子写真实当前Doc快照，进度随同事务推进

再写SeedWriter/SeedIndex/Seeder。SeedWriter只暴露 `SeedInTransaction(ctx,tx,physical,entry)`，SeedIndex只负责EnsureIndex。`Seeder.Step` 锁外验证原日志身份与保留窗口、完整entries流和有界snapshot page，确认owned目标索引；每条entry在generation/binding/job锁下重核当前job及after，再调用S02的SeedInTransaction，将实际投影账本与snapshot_after同tx提交。没有原事件身份，不创建伪Kafka成功receipt。

<!-- lesson-code: services/search/internal/logic/rebuild/installation_service.go :: Seeder.apply -->
````go
func (s *Seeder) apply(parent context.Context, operation string, id, after uint64, entry *rebuildmeta.Entry) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if current.job.ID != id || current.job.State != "SNAPSHOT" || current.job.SnapshotAfter != after {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	if entry == nil {
		err = repository.FinishSnapshot(ctx, tx, id)
	} else {
		if entry.Document <= after {
			return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
		}
		if err = s.Writer.SeedInTransaction(ctx, tx, current.job.Target, *entry); err == nil {
			err = repository.AdvanceSnapshot(ctx, tx, id, after, entry.Document)
		}
	}
	if err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}
````
<!-- /lesson-code -->

条目document=10、minimum=7、release=100，Doc当前已经seq=9/release=102，就seed9；entry.Document必须大于原after，顺序错或重复不能推进。ES已写9而SQL commit未知，after没有可证明推进就继续原条目，ES幂等。整流完成后job由SNAPSHOT转REPLAYING；此时alias仍旧physical，目标已填充不等于可以服务搜索。若当前保留First>L，原窗口已经丢失，重建失败并保留原任务，不将L改成First。

## 6. CLI实际参数与06检查顺序

两个main分别写flag/run和进程入口，flag错误输出固定码而不回显参数/路径。prepare需要 `--config` 指Search自己的配置、`--operation` 原操作号、`--generation` 目标代次；import需要同原operation与 `--snapshot-ref` Doc原seal引用。它们分别把原prepare输出到Search outbox、从Search inbox导入；Search不读Doc私有库。Doc的封存和start CLI在03已写，使用受控目录在服务间传制品。

当前不执行prepare/import：S05尚未写齐且06还没有实际owner配置/Kafka/ES/Doc。S05会给build检查。06完成完整READY后，以下三个已有测试依赖前一文件生成的原操作记录，必须显式顺序、串行执行，不能按目录通配符乱序：

```powershell
Set-Location -LiteralPath $lessonRoot
$lessonRebuildSteps = @(
    'tests/search/rebuild-prepare.test.mjs',
    'tests/search/rebuild-start.test.mjs',
    'tests/search/rebuild-seed.test.mjs'
)
foreach ($lessonRebuildStep in $lessonRebuildSteps) {
    node --no-maglev --test --test-reporter=tap --test-concurrency=1 $lessonRebuildStep
    if ($LASTEXITCODE -ne 0) { throw ('重建原任务步骤失败：' + $lessonRebuildStep) }
}
```

精确测试 `真实准备与Doc封存：原号不换L，原条目可重生且不混入后来文档`、`真实导入与Start/Get绑定原操作，独立检查点固定原L`、`原重建任务完成真实快照种子，应用与进度落定但尚不切alias`。前置是06自己的独立owner、全中间件READY、真实注册/Doc/发布权限；脚本在 `artifacts/batches/06/rebuild-prepare-operation.json` 保存无秘密原引用用于重试。第三步预算较大，仅需要深入重建验收时执行，未运行不报告PASS。预期fail/skip=0，prepare重放原字节、import同引用、三分区next等原L、种子完成仍旧alias。

错误 `SEARCH_REBUILD_ENTRIES_MISMATCH` 核原seal与entries完整副本，不能删hash验证；`SEARCH_REBUILD_NOT_IMPORTED` 先完成同号import；`SEARCH_REBUILD_GENERATION_CHANGED` 核原source/revision；`...COMMIT_UNKNOWN` 保留原operation。没有原操作记录时回第一步，不临时伪造state JSON让后续测试运行。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/search/internal/repository/rebuild/preparation_queries.go`：[本课完成文件](../../source/390-preparation-queries.md)。
- `services/search/internal/repository/rebuild/installation_queries.go`：[本课完成文件](../../source/389-installation-queries.md)。
- `services/search/internal/logic/rebuild/preparation_service.go`：[本课完成文件](../../source/398-preparation-service.md)。
- `services/search/internal/logic/rebuild/installation_service.go`：[本课完成文件](../../source/397-installation-service.md)。
- `services/search/cmd/search-rebuild-prepare/main.go`：[本课完成文件](../../source/409-main.md)。
- `services/search/cmd/search-rebuild-import/main.go`：[本课完成文件](../../source/408-main.md)。

下一节：[ S05 固定窗口重放、完整核验与别名恢复切换 ](05-cutover-wiring.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/search/internal/repository/rebuild/preparation_queries.go',
    'services/search/internal/repository/rebuild/installation_queries.go',
    'services/search/internal/logic/rebuild/preparation_service.go',
    'services/search/internal/logic/rebuild/installation_service.go',
    'services/search/cmd/search-rebuild-prepare/main.go',
    'services/search/cmd/search-rebuild-import/main.go'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
