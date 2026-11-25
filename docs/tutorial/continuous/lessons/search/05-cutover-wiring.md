# S05｜固定窗口重放、完整核验与别名恢复切换

## 这节课要实现什么

新索引回填期间仍会有人发布文档，切换前必须补齐这些变化；切换响应丢失后也不能猜测正在使用哪份索引。本课完成增量追赶、核验、切换与搜索服务接线。

## 做完后能看到什么

接通后，新索引覆盖固定追赶范围并通过完整核验后才接管查询；切换结果未知时检查真实索引指向并恢复原任务。切换完成后的新发布仍能被正常消费。

## 先想清楚怎样实现

1. 先重放原任务固定日志窗口。
2. 封存结束边界并核验投影与账本。
3. 原子切换查询别名并确认SQL代次，再接入RPC、后台任务和进程入口。

## 怎样判断这一课完成

本课构建搜索服务并运行离线规则；06接续S04的同一任务执行真实追赶与切换，最后用新发布和浏览器查询检查结果。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/search/internal/repository/rebuild/completion_queries.go` | 核原重放检查点与封存end，保存终态proof并原子推进SQL代次。 | [逐段带写](../../write/388-completion-queries.md) |
| 2 | `services/search/internal/logic/rebuild/completion_service.go` | 沿TopicUUID逐条重放原窗口，再用完整终态事实固定成功证明，不伪造原事件回执。 | [逐段带写](../../write/396-completion-service.md) |
| 3 | `services/search/internal/client/elastic/aliases_client.go` | 检查alias只指向本owner源或目标，用一次原子aliases请求切换并核验未知结果。 | [逐段带写](../../write/373-aliases-client.md) |
| 4 | `services/search/internal/worker/rebuild.go` | 先恢复SWITCHING屏障，再启动普通消费；后台按原任务推进种子、重放、核验与切换。 | [逐段带写](../../write/402-rebuild.md) |
| 5 | `services/search/internal/svc/rebuild_wiring.go` | 组装Seeder、Replayer、Barrier、Validator与Coordinator并管理日志/ES客户端。 | [逐段带写](../../write/406-rebuild-wiring.md) |
| 6 | `services/search/internal/server/rpc_server.go` | 将生成SearchInternal RPC适配到候选及重建用例，稳定映射业务错误。 | [逐段带写](../../write/399-rpc-server.md) |
| 7 | `services/search/internal/bootstrap/search.go` | 集中创建consumer/candidates/rebuild，注册RPC并启动三个worker，任一恢复失败阻止READY。 | [逐段带写](../../write/407-search.md) |
| 8 | `services/search/main.go` | 交给platform.Main启动search角色，使用RegisterSearch装配，不另建公开HTTP入口。 | [逐段带写](../../write/410-main.md) |

## 2. 先实现独立日志重放与终态账本

先写completion_queries的PendingReplay/ReplayCheckpoints/TerminalFacts/CheckTerminalFrontier/CompleteSwitch，再写completion_service的ReplayLogs/ReplayProcessor/Replayer、固定终态编码与finalizeRebuild。ReplayLogs.Read以原UUID读取[offset,end)，ReplayProcessor.Record复用S02实际索引流程，consumer名为rebuild-job而非普通组。

`Replayer.Step` 每步最多32条完整记录，在每分区原顺序从SQL next读到当前末端；job进入SWITCHING以后只读到保存H。original.Window.Last始终保留原L，不能用本轮next覆盖原窗口。每条先锁原generation/binding/job再重核state/target/next，已被另一实例推进的旧批不再次请求Doc，否则可能在验证完成后写入更新正文。Record自有SQL应用/事件/检查点事务；job锁只守整记录边界，不伪装跨系统原子事务。

<!-- lesson-code: services/search/internal/logic/rebuild/completion_service.go :: Replayer.Step -->
````go
// Step只推进独立任务的SQL next；当前观察到的末端不是切换H，也不改变普通消费组。
// 每步最多32条完整记录，跨分区没有全局顺序，分区内保留broker原始次序和原字节。
func (r *Replayer) Step(ctx context.Context) error {
	if r == nil || r.Seed == nil || r.Seed.DB == nil || r.Logs == nil || r.Processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation, err := repository.PendingReplay(ctx, r.Seed.DB)
	if err != nil || operation == "" {
		return err
	}
	tx, err := r.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	input, err := r.Seed.lock(ctx, tx, operation)
	var frontier [domain.PartitionCount]int64
	if err == nil && input.job.State == "SWITCHING" {
		frontier, err = repository.LoadFrontier(ctx, tx, input.job.ID)
	}
	tx.Rollback()
	if err != nil {
		return err
	}
	if input.job.State != "REPLAYING" && input.job.State != "SWITCHING" {
		return nil
	}
	original := logsource.Window{Identity: logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(original.Identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(original.Identity.TopicID[:], uuid)
	var starts, ends [domain.PartitionCount]int64
	for partition, raw := range input.prepare.Starts {
		starts[partition], err = strconv.ParseInt(raw, 10, 64)
		if err != nil || starts[partition] < 0 {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_INVALID")
		}
		// Read的原窗口Last始终是持久L，绝不能用恢复next或本轮末端覆盖。
		original.Partitions[partition].Last = starts[partition]
	}
	// 元数据与Fetch都在任务锁之外；Capture/Read核验原Cluster、UUID和严格三个分区。
	window, err := r.Logs.Capture(ctx, &original.Identity)
	if err != nil {
		return err
	}
	if window.Identity != original.Identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	for partition, bounds := range window.Partitions {
		if bounds.First < 0 || bounds.First > starts[partition] || bounds.Last < starts[partition] {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[partition] = bounds.Last
		if input.job.State == "SWITCHING" {
			// 持久屏障之后只补到原H。新发布留给切换后的普通消费，
			// 不能让不断增长的broker末端改写本次完整验证的有限边界。
			if frontier[partition] < starts[partition] || frontier[partition] > bounds.Last {
				return errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
			}
			ends[partition] = frontier[partition]
		}
	}
	next, err := repository.ReplayCheckpoints(ctx, r.Seed.DB, input.job.ID, input.prepare.Topic, starts, ends)
	if err != nil {
		return err
	}
	processor := r.Processor(repository.ConsumerName(input.job.ID))
	if processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	remaining := 32
	for partition := range next {
		if next[partition] == ends[partition] {
			continue
		}
		records, _, err := r.Logs.Read(ctx, original, partition, next[partition], ends[partition])
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.Topic != input.prepare.Topic || record.Partition != partition ||
				record.Offset != next[partition] || record.Offset >= ends[partition] {
				return errors.New("SEARCH_REBUILD_LOG_RECORD_INVALID")
			}
			if err := r.apply(ctx, operation, input, processor, record); err != nil {
				return err
			}
			next[partition]++
			remaining--
			if remaining == 0 {
				return nil
			}
		}
	}
	return nil
}
````
<!-- /lesson-code -->

例如L=[100,200,300]、新H=[105,203,309]，目标检查点必须分别到105/203/309。普通组自己的next不被重建改写；H之后的新发布留给切换完成后的普通消费者。partition0缺offset=102则整记录失败，不能用其他分区追上作补偿。

## 3. 写屏障H与两阶段完整验证

回到S04 preparation_service操作页的Barrier/Coordinator/Validator。`Barrier.Seal` 先拿generation排他锁，等待普通完整记录退出，核原job/alias仍在source。捕获原UUID当前窗口，只有每分区距末端<=32才封存H并把任务置SWITCHING；尚未接近时继续后台重放，旧搜索可用。SWITCHING提交后普通Guard明确拒绝新消费/查询，短暂屏障是固定有限核验的必要边界。

<!-- lesson-code: services/search/internal/logic/rebuild/preparation_service.go :: Barrier.Seal -->
````go
func (b *Barrier) Seal(ctx context.Context, operation string) error {
	if b == nil || b.Seed == nil || b.Seed.DB == nil || b.Seed.Logs == nil || b.Indexes == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := b.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	// 排他锁先等待普通消费和重建的完整记录退出，再读取原任务；
	// 不在持有共享锁后升级，避免两个协调器同时升级产生死锁。
	_, found, err := generation.Find(ctx, tx, b.Seed.Alias, true)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := b.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SWITCHING" {
		_, err = repository.LoadFrontier(ctx, tx, input.job.ID)
		return err
	}
	if input.job.State != "REPLAYING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	state, err := b.Indexes.InspectAliasSwitch(ctx, b.Seed.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state != elastic.AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	identity := logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(identity.TopicID[:], uuid)
	window, err := b.Seed.Logs.Capture(ctx, &identity)
	if err != nil {
		return err
	}
	if window.Identity != identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	var ends [events.PartitionCount]int64
	for p, bounds := range window.Partitions {
		start, err := strconv.ParseInt(input.prepare.Starts[p], 10, 64)
		if err != nil || bounds.First > start || bounds.Last < start {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[p] = bounds.Last
		var next int64
		if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p).Scan(&next) != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if next < start || next > bounds.Last {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_UNBOUND")
		}
		// 尚未追近时继续后台重放，不提前暂停当前搜索。
		if bounds.Last-next > 32 {
			return nil
		}
	}
	if err := repository.SealFrontier(ctx, tx, input.job.ID, ends); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}
````
<!-- /lesson-code -->

`Validator.Step` 先证明三分区next恰好等H，再两阶段分页：SNAPSHOT核原entries每文档应用水位>=minimum；APPLICATION遍历目标全部index_application，逐篇Get ES核Sequence/Digest/Release/Visible/Policy完全一致，隐藏墓碑无正文。然后核原事件账本无pending、成功均绑定目标，RefreshCount等全部应用数，包含空快照的非零封存摘要。每页原job锁和验证progress同事务提交，不只比较“文档条数差不多”。

如果目标缺一篇、同水位摘要不同、多一条外来记录或隐藏正文未清空，必须TARGET_MISMATCH/COUNT_MISMATCH/LOG_COVERAGE_FAILED，仍保持原SWITCHING等待修复；不能跳过某篇让后续alias切换。EOF、完整摘要与实际应用绑定一起成立才progress.complete=true。

## 4. ES原子别名切换和SQL未知恢复

先写aliases_client的ResolveAlias/EnsureInitialAlias/InspectAliasSwitch/SwitchAlias及回执核验。AliasSwitchState区分原alias在source、已在target或冲突；source/target都核本owner/schema。SwitchAlias用一次 `/_aliases` 同时remove source/add target，不分两请求产生无指向窗口；ack必须完整，不把超时解释为未切。

<!-- lesson-code: services/search/internal/client/elastic/aliases_client.go :: Client.SwitchAlias -->
````go
// SwitchAlias只执行一次精确切换。调用方必须已持久化SWITCHING屏障及完整验证。
// ES与SQL不构成分布式事务：请求/回执未知直接返回错误，留给原job重读恢复。
func (c *Client) SwitchAlias(ctx context.Context, alias, source, target string) error {
	state, err := c.InspectAliasSwitch(ctx, alias, source, target)
	if err != nil {
		return err
	}
	if state != AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED")
	}
	status, data, err := c.request(ctx, "POST", "/_aliases?master_timeout=1s&timeout=1s", map[string]any{
		"actions": []any{
			map[string]any{"remove": map[string]any{"index": source, "alias": alias, "must_exist": true}},
			map[string]any{"add": map[string]any{"index": target, "alias": alias, "is_write_index": true}},
		},
	}, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 || !validSwitchAcknowledgement(data, alias, source, target) {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !exists || actual != target {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	return nil
}
````
<!-- /lesson-code -->

`Coordinator.Step` 对REPLAYING尝试Seal，对SWITCHING先Validator，再finish。finish专用恢复入口不调用拒绝SWITCHING的普通Guard：锁generation排他→原job，完整validation后InspectAliasSwitch。若仍source，执行一次切换；已target则只补原SQL终态；任何第三方状态拒绝。finalizeRebuild固定prepare/snapshot/请求fingerprint/原L/H/验证摘要/源目标代次，CompleteSwitch同tx推进SQL当前physical/generation/revision并保存结果。GetRebuild只读原proof，不重建一份成功JSON。

<!-- lesson-code: services/search/internal/logic/rebuild/preparation_service.go :: Coordinator.finish -->
````go
// finish专用恢复路径不调用拒绝SWITCHING的普通Guard。ES已在目标时只完成
// 原SQL事务，绝不再次增加代次；未知回执留在同一任务下由下一步重读。
func (c *Coordinator) finish(parent context.Context, operation string) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	s := c.Barrier.Seed
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if _, found, err := generation.Find(ctx, tx, s.Alias, true); err != nil {
		return err
	} else if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SUCCEEDED" {
		return nil
	}
	if input.job.State != "SWITCHING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	progress, err := repository.LoadValidation(ctx, tx, input.job.ID)
	if err != nil {
		return err
	}
	if !progress.Complete {
		return nil
	}
	state, err := c.Barrier.Indexes.InspectAliasSwitch(ctx, s.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state == elastic.AliasAtSource {
		if err := c.Barrier.Indexes.SwitchAlias(ctx, s.Alias, input.job.Source, input.job.Target); err != nil {
			return err
		}
	} else if state != elastic.AliasAtTarget {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	if _, err := finalizeRebuild(ctx, tx, operation, input); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}
````
<!-- /lesson-code -->

具体推演：ES已经从g1切g2，SQL COMMIT前进程退出。重启原job仍SWITCHING，普通消费者先不启动；coordinator重读ES已target，沿原job补SQL到g2，生成原成功结果一次。不能再增加generation到g3、重新建job或把alias切回g1。SQL提交但回复丢失，重复Get/Start必须返回完全相同fixed_result_json_utf8/result_digest。旧g1游标不能再查询，但S03关闭器仍可以关闭原PIT。

## 5. svc、RPC、三个worker与入口实际装配

写NewRebuild/JobService，再写NewRebuildWorker：Doc Snapshots、ES sink、UUID logsource→SeedWriter Processor→Seeder→Replayer（为每job构造独立Processor）→Barrier/Validator/Coordinator。Step按seed→replay→coordinator顺序；Close关闭logs/index。RecoveryReady只查是否仍有SWITCHING，普通consumer自己的Initialize另核原日志恢复。

在server定义CandidateOperations、RebuildOperations，分别把现成生成RPC SearchCandidates/CloseCursor/StartRebuild/GetRebuild交给用例。候选非法输入映射InvalidArgument、过期/陈旧绑定映射FailedPrecondition、配额映射ResourceExhausted、依赖未知Unavailable；重建COMMIT_UNKNOWN也Unavailable，调用者保持原号，不误认为确定拒绝。

bootstrap.RegisterSearch依次组装consumer/candidates/rebuild/rebuildWorker，失败释放已建client并ReadyExtra返回失败，不注册假成功实现。成功启动三条goroutine：RunAfterRecovery等待原SWITCHING恢复后才Run普通消费；RunCursors清理未知PIT；RunRebuild循环原任务。ReadyExtra必须同时通过consumer.Ready和candidates.Ready。最后注册 `pb.RegisterSearchInternalServer(rpc, server.NewSearchWithRebuild(...))`。

<!-- lesson-code: services/search/internal/bootstrap/search.go :: RegisterSearch -->
````go
// RegisterSearch仅组合当前RPC适配，底座就绪继续依赖本服务私有Store的实际检查。
func RegisterSearch(rpc *grpc.Server, store *platform.Store, configuration platform.Config) {
	consumer, err := svc.NewConsumer(store, configuration)
	candidates, candidateErr := svc.NewCandidates(store, configuration)
	rebuild, rebuildErr := svc.NewRebuild(store, configuration)
	if err != nil {
		store.ReadyExtra = func(context.Context) error { return err }
		if candidates != nil {
			candidates.Close()
		}
	} else if candidateErr != nil {
		store.ReadyExtra = func(context.Context) error { return candidateErr }
		consumer.Close()
	} else if rebuildErr != nil {
		store.ReadyExtra = func(context.Context) error { return rebuildErr }
		consumer.Close()
		candidates.Close()
	} else {
		rebuildWorker, seedErr := svc.NewRebuildWorker(store, configuration)
		if seedErr != nil {
			store.ReadyExtra = func(context.Context) error { return seedErr }
			consumer.Close()
			candidates.Close()
			pb.RegisterSearchInternalServer(rpc, server.NewSearch())
			return
		}
		store.ReadyExtra = func(ctx context.Context) error {
			if err := consumer.Ready(ctx); err != nil {
				return err
			}
			return candidates.Ready(ctx)
		}
		go worker.RunAfterRecovery(store.Context, consumer, rebuildWorker.RecoveryReady)
		go worker.RunCursors(store.Context, candidates)
		go worker.RunRebuild(store.Context, rebuildWorker)
	}
	if err != nil || candidateErr != nil || rebuildErr != nil {
		pb.RegisterSearchInternalServer(rpc, server.NewSearch())
		return
	}
	pb.RegisterSearchInternalServer(rpc, server.NewSearchWithRebuild(candidates, rebuild))
}
````
<!-- /lesson-code -->

main调用 `platform.Main("search", bootstrap.RegisterSearch, nil)`。Search本身提供内部RPC，不给浏览器另开公开搜索HTTP；浏览器HTTP由03的Doc handler负责当前权限与输出边界。platform管理store.Context取消，worker退出Close依赖；后台错误只写固定码变化，不输出消息正文、路径或凭据。

## 6. 全包构建与离线规则关口

```powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
go -C services/search build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '搜索完整包或CLI装配编译失败' }
go -C services/search test -mod=readonly ./internal/domain/... -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '事件、索引或游标规则失败' }
go -C services/search test -mod=readonly ./internal/client/elastic -run '^TestSwitchAlias(AcknowledgementAndRecovery|RejectsForeignOwnerBeforeMutation)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '原子别名回执或owner拒绝失败' }
go -C services/search test -mod=readonly ./internal/logic/consumer -run '^TestWindowRejectsLostHistoryAndBrokerPositionAheadOfSQL$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'SQL/Kafka消费窗口失败' }
python $coursePrepare --target $lessonRoot --check 05
if ($LASTEXITCODE -ne 0) { throw '全量后端文件未齐或原字节不符' }
```

精确alias测试 `TestSwitchAliasAcknowledgementAndRecovery`、`TestSwitchAliasRejectsForeignOwnerBeforeMutation` 使用测试HTTP服务；预期PASS、无SKIP，不能宣称实际alias已切。build没有测试持久状态，check只核源码清单。编译提示undefined finalizeRebuild先核completion_service；Import undefined核S04同包文件；mTLS/RPC契约错误先核共享生成物而非新增DTO。

## 7. 06真实重建收口与限制

06自己完整READY且S04三步真实测试已经通过后，显式继续原任务，不重新Prepare：

```powershell
Set-Location -LiteralPath $lessonRoot
foreach ($lessonRebuildStep in @('tests/search/rebuild-replay.test.mjs','tests/search/rebuild-switch.test.mjs')) {
    node --no-maglev --test --test-reporter=tap --test-concurrency=1 $lessonRebuildStep
    if ($LASTEXITCODE -ne 0) { throw ('原重建任务尚未完成：' + $lessonRebuildStep) }
}
```

精确测试 `封存后的真实发布独立重放到目标，普通消费事实与当前代次保持` 核原L后变化进入目标、普通组未被重建改写；`原重建任务完整核验后切换，固定结果可重复查询且新发布进入新代次` 核真实alias/SQL代次/固定proof与后续正常消费。预期fail/skip=0并进程退出0。此两项依赖S04保存的原操作记录，不用单独造快照替代原job。

如果专门排查未知切换边界，先设置本学习配置，再只选现有具名测试：

```powershell
$env:COLLAB_TEST_SEARCH_CONFIG = (Resolve-Path -LiteralPath '.runtime/services/search.json').Path
go -C services/search test -mod=readonly ./internal/logic/rebuild -run '^TestActualRebuildAliasUnknownRecoversOriginalJob$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'ES切换未知后的原job恢复失败' }
```

该测试使用真实SQL/ES和显式空日志/空快照fixture，只证明ES切换后未知恢复，不替代真实Doc快照/Kafka重放；`TestActualRebuildProcessExitAfterAliasRecoversSameJob` 是更重的进程边界测试，按功能测试指南的独立前置选用，不因当前文档重写重复运行。ALIAS_SWITCH_UNKNOWN先Inspect原source/target；LOG_WINDOW_LOST保留原L/UUID；VALIDATION失败查实际投影/账本，不清除SWITCHING。全站用户验收仍回06的发布/当前权限/浏览器真实搜索关口，教学源码核对通过不代表新学习顺序已独立复原验证。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/search/internal/repository/rebuild/completion_queries.go`：[本课完成文件](../../source/388-completion-queries.md)。
- `services/search/internal/logic/rebuild/completion_service.go`：[本课完成文件](../../source/396-completion-service.md)。
- `services/search/internal/client/elastic/aliases_client.go`：[本课完成文件](../../source/373-aliases-client.md)。
- `services/search/internal/worker/rebuild.go`：[本课完成文件](../../source/402-rebuild.md)。
- `services/search/internal/svc/rebuild_wiring.go`：[本课完成文件](../../source/406-rebuild-wiring.md)。
- `services/search/internal/server/rpc_server.go`：[本课完成文件](../../source/399-rpc-server.md)。
- `services/search/internal/bootstrap/search.go`：[本课完成文件](../../source/407-search.md)。
- `services/search/main.go`：[本课完成文件](../../source/410-main.md)。

继续：[06 接通自己的实例](../../06-integration.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/search/internal/repository/rebuild/completion_queries.go',
    'services/search/internal/logic/rebuild/completion_service.go',
    'services/search/internal/client/elastic/aliases_client.go',
    'services/search/internal/worker/rebuild.go',
    'services/search/internal/svc/rebuild_wiring.go',
    'services/search/internal/server/rpc_server.go',
    'services/search/internal/bootstrap/search.go',
    'services/search/main.go'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
