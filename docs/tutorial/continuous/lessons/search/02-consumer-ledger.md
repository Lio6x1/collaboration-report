# S02｜持久消费、日志身份与代次锁

## 这节课要实现什么

搜索服务处理一条发布消息时可能重启，恢复后不能漏掉它，也不能越过失败消息继续消费。本课实现有持久进度的可靠消息处理。

## 做完后能看到什么

接通后，只有索引处理与数据库进度确认后才提交消息位置；重复处理可核对原记录，丢失日志范围或主题身份变化会被拒绝，而不是假装追赶完成。

## 先想清楚怎样实现

1. 先固定日志来源和消费窗口。
2. 一次保留一条待处理记录。
3. 按索引、数据库账本、消息确认顺序推进，重启依据持久进度恢复，并检查启动时待追赶范围。

## 怎样判断这一课完成

搜索包写齐后运行消费水位规则；06验证真实Kafka、SQL和索引恢复，单独看到消费循环运行不能证明没有漏消息。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/search/internal/client/kafka/client.go` | 用成熟Kafka消费组读取单条并同步确认原记录，持有pending防止越过失败消息。 | [逐段带写](../../write/378-client.md) |
| 2 | `services/search/internal/client/kafka/window.go` | 读取三分区保留起点、末尾和组提交位置，向用例提供消费窗口。 | [逐段带写](../../write/379-window.md) |
| 3 | `services/search/internal/client/logsource/window.go` | 按Cluster/TopicUUID捕获三个分区窗口；同名重建topic不能冒充原日志。 | [逐段带写](../../write/382-window.md) |
| 4 | `services/search/internal/client/logsource/records.go` | 严格解码Kafka record batch，核CRC、压缩/长度及连续范围，整响应损坏则整批拒绝。 | [逐段带写](../../write/381-records.md) |
| 5 | `services/search/internal/client/logsource/read.go` | 按原TopicUUID直接读指定[offset,end)区间，不加入组、不提交普通消费offset。 | [逐段带写](../../write/380-read.md) |
| 6 | `services/search/internal/repository/event/validation.go` | 校验consumer/position与原Event合法性，SQL账本入口不能比Canal恢复更宽松。 | [逐段带写](../../write/386-validation.md) |
| 7 | `services/search/internal/repository/event/events_queries.go` | 维护原事件、收到/成功回执、实际应用水位、SQL连续next_offset和日志身份。 | [逐段带写](../../write/385-events-queries.md) |
| 8 | `services/search/internal/repository/generation/active.go` | 统一generation共享/排他锁和SWITCHING检查，读写原alias→physical绑定。 | [逐段带写](../../write/387-active.md) |
| 9 | `services/search/internal/logic/generation/guard.go` | 初始化确切初代alias，并在一整条记录及Kafka提交期间持共享代次锁。 | [逐段带写](../../write/393-guard.md) |
| 10 | `services/search/internal/logic/indexing/processor.go` | 保存原行→Doc权威→ES→SQL实际应用与成功回执；未知结果重读同一原事件。 | [逐段带写](../../write/395-processor.md) |
| 11 | `services/search/internal/logic/indexing/indexing_service.go` | 把多行Kafka记录作为完整检查点单位；种子沿调用方事务保存投影及进度。 | [逐段带写](../../write/394-indexing-service.md) |
| 12 | `services/search/internal/logic/consumer/consumer.go` | 串行拥有current，失败保留原记录；SQL成功后同步Kafka，全部确认后才Fetch下一条。 | [逐段带写](../../write/392-consumer.md) |
| 13 | `services/search/internal/svc/log_identity.go` | 把Cluster/TopicUUID与Search自己的SQL持久绑定，拒绝配置同名日志置换。 | [逐段带写](../../write/405-log-identity.md) |
| 14 | `services/search/internal/svc/consumer_wiring.go` | 组装boundBroker、Guard、Doc快照与ES；组记录必须再经UUID直读逐字节核对。 | [逐段带写](../../write/404-consumer-wiring.md) |
| 15 | `services/search/internal/worker/consumer.go` | 以有界Step循环消费并尊重服务取消，故障保留原current而不自行跳过。 | [逐段带写](../../write/400-consumer.md) |

## 2. 从消费者接口开始，而非先写Kafka循环

先写 `Broker/Generations/Indexer` 和Consumer字段。Broker提供Window、Fetch和Commit；Generations提供Initialize、Ready和Do；Indexer提供Record(ctx,physical,position,raw)。业务只拿Record/Position，不持第三方Reader。Consumer保存原 `current *Record`、stepMu、防并发Ready的stateMu、启动frontier与最近failure。`Step` 的返回error表示本条仍需恢复，不表示可以Fetch下一条。

<!-- lesson-code: services/search/internal/logic/consumer/consumer.go :: Broker -->
````go
// Broker仅暴露消费窗口、单条读取与确认；业务不依赖具体Kafka客户端。
type Broker interface {
	Window(context.Context) (map[int]domain.PartitionWindow, error)
	Fetch(context.Context) (domain.Record, error)
	Commit(context.Context, domain.Record) error
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/logic/indexing/processor.go :: SnapshotSource -->
````go
// 接口由索引用例定义：客户端负责真实协议，用例负责三个系统之间的提交顺序。
type SnapshotSource interface {
	Current(context.Context, domain.Event) (index.Authority, error)
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/logic/indexing/processor.go :: IndexSink -->
````go
type IndexSink interface {
	Apply(context.Context, string, index.Authority) (index.State, error)
}
````
<!-- /lesson-code -->

SnapshotSource.Current返回Doc当前Authority；IndexSink.Apply返回ES实际State。Processor.New接受Search私有database、source、sink及consumer名。它不持Kafka客户端，Event/Record不提交Kafka；外层消费者负责同步确认，避免把不同系统伪装成一个事务。

## 3. 先写日志协议与SQL原始账本

client/kafka先写New/newReader/Fetch/Commit/Close，再写window。成熟组Reader读取原message，pending绑定topic/partition/offset与字节摘要，只有原记录能提交，非法记录不继续Fetch。logsource先写Identity/Bounds/Window与Capture，再写records解码和Read；UUID Fetch要求协议Metadata>=v10、Fetch v13，不降级成仅按主题名。

Identity是Cluster、Topic、16字节TopicID；leader不是身份，选举可刷新路由。Capture前后都核身份，expected是原持久身份，不能用刚观察到的新UUID覆盖。Read读取原UUID的[offset,end)并核CRC/记录范围，任何损坏整个响应失败；它不加入普通组也不提交组offset。

写repository/event时顺序是validatePosition/validateEvent→Receive/transition→LockApplication/SaveApplication/Complete→Checkpoint/AdvanceCheckpoint→BindLogIdentity/CheckRecovery。`event_origin` 保存Original/RawDigest与原位置，`consumed_event` 保存APPLYING/RETRY/SUCCEEDED等原回执，`index_application` 按physical/document保存实际水位，`consumer_checkpoint` 保存每分区下一offset。Receive同event_id但原摘要不同拒绝，不能让重投改写Original。仓储一律收外层tx，不自行commit。

<!-- lesson-code: services/search/internal/repository/event/events_queries.go :: AdvanceCheckpoint -->
````go
// AdvanceCheckpoint核对整条记录所有行成功，才写SQL下一offset；Kafka实际提交仍在tx.Commit之后。
// 原SQL已成功但Kafka未知时允许较早记录重读，不允许越过同分区未完成的中间记录。
func AdvanceCheckpoint(ctx context.Context, tx *sql.Tx, consumer string, position Position, events []domain.Event) error {
	if err := validatePosition(consumer, position); err != nil {
		return err
	}
	if position.Offset == math.MaxInt64 || len(events) == 0 || len(events) > 1024 {
		return errors.New("SEARCH_CHECKPOINT_INPUT_INVALID")
	}
	for _, value := range events {
		if err := validateEvent(value); err != nil {
			return err
		}
		var state, topic string
		var digest []byte
		var partition int
		var offset int64
		if tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset
			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
			WHERE c.consumer_name=? AND c.event_id=? FOR UPDATE`, consumer, value.ID).
			Scan(&state, &digest, &topic, &partition, &offset) != nil || state != "SUCCEEDED" ||
			!bytes.Equal(digest, value.RawDigest[:]) || topic != position.Topic || partition != position.Partition || offset > position.Offset {
			return errors.New("SEARCH_RECORD_ROWS_NOT_COMPLETE")
		}
	}
	var next, revision uint64
	err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition).Scan(&next, &revision)
	if err == sql.ErrNoRows {
		if position.Offset != 0 {
			return errors.New("SEARCH_OFFSET_GAP")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset)
			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition); err != nil {
			return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
		}
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_READ_FAILED")
	}
	if next > uint64(math.MaxInt64) || revision == 0 {
		return errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	if uint64(position.Offset) > next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	if uint64(position.Offset) < next {
		return nil
	}
	if revision == math.MaxUint64 {
		return errors.New("SEARCH_CHECKPOINT_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE consumer_checkpoint SET next_offset=?,revision=?,updated_at=UTC_TIMESTAMP(6)
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? AND revision=?`, position.Offset+1, revision+1,
		consumer, position.Topic, position.Partition, revision)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_CHECKPOINT_CHANGED")
	}
	return nil
}
````
<!-- /lesson-code -->

一条Kafka offset=4含outbox e1/e2两行：e1成功而e2失败，SQL next仍4，Kafka不能提交5。重试原记录，e1核原成功绑定，e2恢复后才AdvanceCheckpoint=5。next=4收到offset=6是 `SEARCH_OFFSET_GAP`，不能跳到6；offset=3重投可重读原事实，不把较旧记录当缺口。

## 4. 索引业务跨系统的完整提交顺序

按Processor操作页写New→Event→receive→verifyReplay→apply→retry，再写indexing_service的Record/completeRecord/Seed/SeedInTransaction。Event第一笔短事务保存原行并标APPLYING，锁外请求Doc当前Authority；第二事务锁原receipt和physical/document应用行，ES原子Apply后核返回State，再写SQL实际应用与成功回执，commit后返回。Doc失败不是撤下，ES成功SQL未知也不跳过。

<!-- lesson-code: services/search/internal/logic/indexing/processor.go :: Processor.Event -->
````go
// Event不提交Kafka。先持久保存原行，再取Doc当前权威，最后在ES成功之后
// 同事务保存实际应用水位与成功回执；中间任何未知结果都允许沿原消息重读。
func (p *Processor) Event(ctx context.Context, physical string, event domain.Event, position repository.Position) (outcome error) {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := p.receive(ctx, event, position)
	if err != nil {
		return err
	}
	if receipt.State == "REJECTED" {
		return errors.New("SEARCH_ORIGINAL_EVENT_REJECTED")
	}
	defer func() {
		if outcome != nil {
			p.retry(event, position, outcome)
		}
	}()
	if receipt.State == "SUCCEEDED" && receipt.Physical == physical {
		return p.verifyReplay(ctx, physical, event, position)
	}
	// 网络请求不占用原事件事务；Doc不可达或未知不能被合成为删除墓碑。
	authority, err := p.source.Current(ctx, event)
	if err != nil {
		return publicError(err, "SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	if authority.Document != event.Document || authority.Sequence < event.Sequence {
		return errors.New("SEARCH_CURRENT_AUTHORITY_UNBOUND")
	}
	return p.apply(ctx, physical, event, position, authority)
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/logic/indexing/indexing_service.go :: Processor.Record -->
````go
// Record把Kafka的一条记录作为提交单位。全部原行解析完成后才应用，
// 全部应用成功之后才提交SQL下一offset；实际Kafka同步提交属于外层消费者。
func (p *Processor) Record(ctx context.Context, physical string, position repository.Position, raw []byte) error {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	events, err := domain.ParseCanal(raw)
	if err != nil {
		return err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	next, err := repository.Checkpoint(ctx, tx, p.consumer, position.Topic, position.Partition)
	tx.Rollback()
	if err != nil {
		return err
	}
	if position.Offset < 0 || position.Offset > next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	for _, event := range events {
		if err := p.Event(ctx, physical, event, position); err != nil {
			return err
		}
	}
	return p.completeRecord(ctx, position, events)
}
````
<!-- /lesson-code -->

输入推演：原e1.seq=5，Doc当前7，ES已应用7；SQL COMMIT回执丢失。下一Step继续e1：若SQL已经SUCCEEDED，verifyReplay核原行和实际physical水位，不新增revision；若SQL没提交，重新取当前权威并Apply，ES相同摘要noop，再补SQL。原事件RawDigest始终不变。retry只在取消后两秒内做失败记账，SUCCEEDED不能被迟到错误写回RETRY。

SeedInTransaction用于S04种子，由调用方持有job/代次锁和tx，ES成功后SaveApplication与快照after同事务提交；它不伪造event_origin/consumed_event，因为快照条目没有Kafka原事件身份。

## 5. 代次Guard、Step与READY接线

写repository/generation的Target/Find/LockCurrent/CheckSwitching/InsertInitial，再写Guard。Initialize先确认本owner确切alias→physical后才提交SQL初代，未知则核同一绑定。Do只持代次共享锁，贯穿一整条Record和Kafka Commit，切换端需排他锁，不能在中途写到新physical。它不替事件事务提交，也不声称ES/SQL/Kafka原子。

Consumer.Step初始化时审计原SQL账本并捕获启动frontier。Fetch后先查当前窗口及SQL next，再在generation.Do里Record→broker.Commit，只有两者成功才清current。SQL成功Kafka提交超时，current仍是原记录，下一次重做并同步确认；没有“已发给broker就成功”。Ready除generation外还检查initialized/failure、窗口未丢与next已追上启动frontier。

<!-- lesson-code: services/search/internal/logic/consumer/consumer.go :: Consumer.Step -->
````go
// Step串行拥有一条在途记录。任何失败都留下原current，下一次Step重做原记录，
// 不Fetch后面的记录，也不根据Reader预取的最高offset批量提交。
func (c *Consumer) Step(ctx context.Context) (outcome error) {
	c.stepMu.Lock()
	defer c.stepMu.Unlock()
	defer func() { c.stateMu.Lock(); c.failure = outcome; c.stateMu.Unlock() }()
	if !c.initialized {
		if err := c.initialize(ctx); err != nil {
			return err
		}
	}
	if c.current == nil {
		poll, done := context.WithTimeout(ctx, 2*time.Second)
		record, err := c.broker.Fetch(poll)
		done()
		if err != nil {
			if ctx.Err() == nil && err.Error() == "SEARCH_KAFKA_FETCH_CANCELED" {
				return c.inspect(ctx)
			}
			return err
		}
		c.current = &record
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, false)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	// 本条消息绑定当前物理索引代次：索引投影、SQL回执和Kafka提交是独立步骤。
	// Record内部按原序号/摘要允许已成功部分重放，不能把Kafka提交当跨系统事务。
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		if err := c.indexer.Record(ctx, target.Physical, c.current.Position, c.current.Value); err != nil {
			return err
		}
		// SQL整条记录已成功后才能同步Kafka；这个独立动作超时仍保留原记录重试。
		return c.broker.Commit(ctx, *c.current)
	})
	if err != nil {
		return err
	}
	// 两个步骤都确认成功才丢掉原记录；否则下一轮继续原消息，不越过失败位置。
	c.current = nil
	return nil
}
````
<!-- /lesson-code -->

窗口例子：First=0、Last=5、Committed=2、SQL next=3合法；First变4而next仍3说明未处理历史已被删除，返回 `SEARCH_KAFKA_WINDOW_LOST`；Committed=4而SQL next=3说明broker领先SQL，也拒绝。不能auto reset latest掩盖缺口。

svc/log_identity把Cluster/UUID持久绑定本group，svc/consumer_wiring的boundBroker把组Fetch所得原字节再经UUID Read核一遍，并在名称式Commit前后检查身份。即使topic同名重建，SQL原next不能套到新日志。worker.Run只对原current做有限退避，固定码变化才日志；服务ctx取消时Close Kafka/ES。S05 bootstrap会实际启动它。

## 6. 精确离线与真实恢复检查

```powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
go -C services/search test -mod=readonly ./internal/logic/consumer -run '^TestWindowRejectsLostHistoryAndBrokerPositionAheadOfSQL$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '消费窗口规则失败' }
go -C services/search test -mod=readonly ./internal/client/logsource -run '^TestDecodeRecords(BoundedWindow|RejectsEntireCorruptResponse|MaximumValue)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'UUID日志记录解码失败' }
```

预期具名测试PASS、没有SKIP；第二条只选纯解码测试，不误跑同包Actual依赖。06准备Kafka/ES/Canal、Doc、Search及本学习数据库后执行 `node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/search/consumer-identity.test.mjs tests/search/indexing-recovery.test.mjs`，精确测试 `持久日志身份消费者完成真实发布，SQL落定后同步提交普通组` 与 `实际ES停止时原消息未提交，Search进程退出后从同一消息恢复` 核真实提交顺序，恢复组会停止本owner依赖，必须串行。

需要定位某一边界时，在06同一终端设置 `$env:COLLAB_TEST_SEARCH_CONFIG=(Resolve-Path '.runtime/services/search.json').Path`，再选现有 `go -C services/search test -mod=readonly ./internal/logic/indexing -run '^TestActualESSuccessBeforeSQLFailureRecoversSameOriginalRecord$' -count=1 -v`。没有配置会明确ACTUAL_SEARCH_CONFIG_REQUIRED，不算离线失败或通过。WINDOW_LOST保留原next和日志身份，排查保留范围；IDENTITY_CHANGED核同名主题是否重建，不能重置账本制造READY。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/search/internal/client/kafka/client.go`：[本课完成文件](../../source/378-client.md)。
- `services/search/internal/client/kafka/window.go`：[本课完成文件](../../source/379-window.md)。
- `services/search/internal/client/logsource/window.go`：[本课完成文件](../../source/382-window.md)。
- `services/search/internal/client/logsource/records.go`：[本课完成文件](../../source/381-records.md)。
- `services/search/internal/client/logsource/read.go`：[本课完成文件](../../source/380-read.md)。
- `services/search/internal/repository/event/validation.go`：[本课完成文件](../../source/386-validation.md)。
- `services/search/internal/repository/event/events_queries.go`：[本课完成文件](../../source/385-events-queries.md)。
- `services/search/internal/repository/generation/active.go`：[本课完成文件](../../source/387-active.md)。
- `services/search/internal/logic/generation/guard.go`：[本课完成文件](../../source/393-guard.md)。
- `services/search/internal/logic/indexing/processor.go`：[本课完成文件](../../source/395-processor.md)。
- `services/search/internal/logic/indexing/indexing_service.go`：[本课完成文件](../../source/394-indexing-service.md)。
- `services/search/internal/logic/consumer/consumer.go`：[本课完成文件](../../source/392-consumer.md)。
- `services/search/internal/svc/log_identity.go`：[本课完成文件](../../source/405-log-identity.md)。
- `services/search/internal/svc/consumer_wiring.go`：[本课完成文件](../../source/404-consumer-wiring.md)。
- `services/search/internal/worker/consumer.go`：[本课完成文件](../../source/400-consumer.md)。

下一节：[ S03 签名批锚点、PIT与当前权限二次过滤 ](03-candidates-cursor.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/search/internal/client/kafka/client.go',
    'services/search/internal/client/kafka/window.go',
    'services/search/internal/client/logsource/window.go',
    'services/search/internal/client/logsource/records.go',
    'services/search/internal/client/logsource/read.go',
    'services/search/internal/repository/event/validation.go',
    'services/search/internal/repository/event/events_queries.go',
    'services/search/internal/repository/generation/active.go',
    'services/search/internal/logic/generation/guard.go',
    'services/search/internal/logic/indexing/processor.go',
    'services/search/internal/logic/indexing/indexing_service.go',
    'services/search/internal/logic/consumer/consumer.go',
    'services/search/internal/svc/log_identity.go',
    'services/search/internal/svc/consumer_wiring.go',
    'services/search/internal/worker/consumer.go'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
