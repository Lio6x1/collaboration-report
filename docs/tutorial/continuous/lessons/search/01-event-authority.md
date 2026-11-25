# S01｜Canal事件与Doc当前正式投影

## 这节课要实现什么

作者发布文档后，它应能进入搜索；撤下正式版后，旧消息不能把它重新搜出来。本课实现正式版事件解析与索引投影基础。

## 做完后能看到什么

接通后，索引内容依据文档服务当前权威事实建立；过时或重复事件不会覆盖更晚结果，已隐藏的内容不会被旧事件复活。未发布草稿不作为正式版搜索正文。

## 先想清楚怎样实现

1. 先严格解析发布变更事件并保留原消息身份。
2. 向文档服务获取当前固定快照。
3. 验证后提取索引字段，按精确版本顺序写入或隐藏。

## 怎样判断这一课完成

本课执行事件与投影离线规则；06实际发布和撤下文档，核对真实消息到达后索引与页面结果。

## 开始写之前

从[04 协作课程](../../04-collaboration.md)继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/search/internal/domain/event/events_models.go` | 定义不可变Event原行与Kafka位置；三分区属于前沿v1协议。 | [逐段带写](../../write/370-events-models.md) |
| 2 | `services/search/internal/domain/event/canal.go` | 严格解析document_db.outbox INSERT，校验外层/内层引用一致并保留Original与RawDigest。 | [逐段带写](../../write/369-canal.md) |
| 3 | `services/search/internal/domain/index/projection_models.go` | 从已验证Doc固定快照提取文本、资源和Authority；完整uint64水位与隐藏墓碑防复活。 | [逐段带写](../../write/371-projection-models.md) |
| 4 | `services/search/internal/client/document/snapshot.go` | 用Search服务证书RPC读取Doc当前正式快照，minimum是下界，旧release不当正文选择器。 | [逐段带写](../../write/372-snapshot.md) |
| 5 | `services/search/internal/client/elastic/client.go` | 统一ES连接、owner配置、有界响应和生命周期；owner通过索引mapping元数据核验。 | [逐段带写](../../write/375-client.md) |
| 6 | `services/search/internal/client/elastic/mapping_client.go` | 创建并核验本owner物理索引映射，水位存20位keyword、正文以固定字段索引。 | [逐段带写](../../write/376-mapping-client.md) |
| 7 | `services/search/internal/client/elastic/projection_client.go` | 以ES原子更新脚本比较水位/摘要，保存隐藏墓碑并返回实际应用状态。 | [逐段带写](../../write/377-projection-client.md) |

## 2. 先定义事件模型：原事件与当前权威分开保存

按表先写events_models的 `PartitionCount/Event/Position/Record/PartitionWindow`，再写严格JSON解码，最后写canal解析。`Event.ID/Type/Document/Sequence/Release/PublicationEpoch/OccurredAt` 是原outbox公开引用；`RawDigest [32]byte` 是规范原行摘要，`Original []byte` 是恢复必须保留的不可变行。Kafka的Topic/Partition/Offset不属于业务事件身份，同一事件可以在不同消息位置重投。

`Position.Offset int64` 遵守Kafka有符号协议；文档、发布、事件序号仍是uint64。`PartitionCount=3` 是前沿v1契约，不能只把broker改四分区就指望旧检查点继续成立。`PartitionWindow.First/Last/Committed` 分别是当前保留起点、下一末端offset、组提交位置；它们在S02判断是否丢失未消费历史。

<!-- lesson-code: services/search/internal/domain/event/events_models.go :: Event -->
````go
// Event只保存outbox公开引用及原内容摘要。消息批次/投递时间不属于业务身份，
// Doc后来返回的当前权威摘要必须另存，不能覆盖RawDigest来伪造原事件一致性。
type Event struct {
	ID               string
	Type             string
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	OccurredAt       time.Time
	RawDigest        [32]byte
	Original         []byte
}
````
<!-- /lesson-code -->

`ParseCanal(raw []byte) ([]Event,error)` 只接受UTF-8、最多2MiB的document_db.outbox INSERT；非DDL、PK只有event_id、data最多1024行。decodeStrict先拒绝重复JSON键与深度超限，再禁止未知字段。每行严格八个字符串字段，payload也严格八项，外层event_id/type/document/seq与payload必须一致。三类事件是DOCUMENT_PUBLISHED、DOCUMENT_UNPUBLISHED、DOCUMENT_VISIBILITY_CHANGED；发布必须release非空，撤下必须空，可见性变化既可绑定当前正式指针也可未发布。时间必须RFC3339Nano并以Z结尾。

<!-- lesson-code: services/search/internal/domain/event/canal.go :: ParseCanal -->
````go
// ParseCanal只接受受限复制表的INSERT，不把DDL、其他私有表或未知schema当可消费成功。
// 一条Kafka记录可包含多行；调用方只有在全部行完成后才能同步提交该记录的offset。
func ParseCanal(raw []byte) ([]Event, error) {
	if len(raw) == 0 || len(raw) > MaxRecordBytes || !utf8.Valid(raw) {
		return nil, errors.New("CANAL_RECORD_SIZE_INVALID")
	}
	var message flatMessage
	if err := decodeStrict(raw, &message); err != nil {
		return nil, err
	}
	if message.Database != "document_db" || message.Table != "outbox" || message.IsDDL == nil || *message.IsDDL ||
		message.Type != "INSERT" || len(message.PKNames) != 1 || message.PKNames[0] != "event_id" ||
		len(message.Data) == 0 || len(message.Data) > 1024 {
		return nil, errors.New("CANAL_RECORD_BINDING_INVALID")
	}
	events := make([]Event, 0, len(message.Data))
	for _, row := range message.Data {
		value, err := parseRow(row)
		if err != nil {
			return nil, err
		}
		events = append(events, value)
	}
	return events, nil
}
````
<!-- /lesson-code -->

具体推演：document_id=`9007199254740993`、aggregate_seq=`18446744073709551615` 是合法原字符串；`090`、JSON数字90、溢出uint64或payload里另写document_id=91都拒绝。Canal外层TS/GTID在重投可变化，所以RawDigest只覆盖规范原outbox行，不能对整个批元数据求hash。`RestoreEvent` 从保存Original恢复时仍走parseRow相同规则，SQL不能提供一个更弱的恢复入口。

## 3. 再写Doc正式快照模型与调用接口

先在projection_models写Chapter/Authority/State，再写Watermark/ShouldApply，最后写FromSnapshot与ExtractChapter。`Authority` 是Doc当前事实：Document/Sequence/Release/PublicationEpoch/Structure/Visible/Policy/Title/Text/Chapters/Digest；`State` 是物理索引已实际应用的Sequence/Release/Visible/Policy/Digest/Revision。Visible表示可索引，不授予浏览器当前读取权。Policy只能PUBLIC/SPACE_MEMBERS/COLLABORATORS。

`FromSnapshot(validator,snapshot,document,minimum)` 必须核文档相等且seq>=minimum、未知proto字段为空、正式结构和内容版本绑定、content-v1规范字节与body_hash/provenance一致、资源集合精确。Hidden必须release/structure/title/units全空；Doc RPC失败或NotFound不能由Search猜成Hidden。每单元正文不超过16MiB，总固定正文/提取文本不超过24MiB。Authority.Digest覆盖完整确定性protobuf当前快照，它不替换原事件RawDigest。

<!-- lesson-code: services/search/internal/domain/index/projection_models.go :: Authority -->
````go
// Authority来自Doc当前正式快照；Visible是可索引状态，不代表当前浏览器匿名可读。
type Authority struct {
	Document         uint64
	Sequence         uint64
	Release          uint64
	PublicationEpoch uint64
	Structure        uint64
	Visible          bool
	Policy           string
	Title            string
	Text             string
	Chapters         []Chapter
	Digest           [32]byte
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/domain/index/projection_models.go :: FromSnapshot -->
````go
// FromSnapshot只消费Doc认证的当前固定版本，不读Yjs，不把旧事件release当正文选择器。
// 权威摘要覆盖完整protobuf固定事实；它与原Canal行摘要独立保存。
func FromSnapshot(validator *codec.Validator, snapshot *pb.PublicSnapshot, document, minimum uint64) (Authority, error) {
	var result Authority
	if validator == nil || snapshot == nil || document == 0 || snapshot.DocumentId != document || snapshot.AggregateSeq < minimum ||
		snapshot.PublicationEpoch == 0 || snapshot.VisibilityRevision == 0 || proto.Size(snapshot) > 32<<20 ||
		len(snapshot.ProtoReflect().GetUnknown()) != 0 || len(snapshot.Units) > 100000 {
		return result, errors.New("SEARCH_CURRENT_SNAPSHOT_INVALID")
	}
	result = Authority{Document: document, Sequence: snapshot.AggregateSeq, Release: snapshot.ReleaseId, PublicationEpoch: snapshot.PublicationEpoch,
		Structure: snapshot.StructureId, Visible: snapshot.Visible, Policy: snapshot.VisibilityPolicy, Title: snapshot.Title, Chapters: []Chapter{}}
	if result.Policy != "PUBLIC" && result.Policy != "SPACE_MEMBERS" && result.Policy != "COLLABORATORS" {
		return result, errors.New("SEARCH_CURRENT_POLICY_INVALID")
	}
	if !result.Visible {
		if result.Release != 0 || result.Structure != 0 || result.Title != "" || len(snapshot.Units) != 0 {
			return result, errors.New("SEARCH_HIDDEN_SNAPSHOT_HAS_BODY")
		}
	} else {
		if result.Release == 0 || result.Structure == 0 || result.Sequence == 0 || !utf8.ValidString(result.Title) || len(result.Title) > 1024 {
			return result, errors.New("SEARCH_VISIBLE_SNAPSHOT_INVALID")
		}
		seen := make(map[uint64]bool)
		var text strings.Builder
		var total uint64
		for _, unit := range snapshot.Units {
			if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || seen[unit.UnitId] {
				return result, errors.New("SEARCH_FIXED_UNIT_INVALID")
			}
			seen[unit.UnitId] = true
			fixed := unit.Content
			if fixed == nil || fixed.DocumentId != document || fixed.UnitId != unit.UnitId || fixed.StructureId != result.Structure ||
				fixed.SchemaVersion != "content-v1" || fixed.CanonicalizationVersion != "canonical-v1" || fixed.HashAlgorithm != "SHA-256" ||
				fixed.ProvenanceSchema != "provenance-v1" || len(fixed.BodyJsonUtf8) > 16<<20 || fixed.ByteSize != uint64(len(fixed.BodyJsonUtf8)) ||
				fixed.ByteSize > 24<<20 || total > (24<<20)-fixed.ByteSize {
				return result, errors.New("SEARCH_FIXED_CONTENT_INVALID")
			}
			total += fixed.ByteSize
			body, err := codec.Decode(fixed.BodyJsonUtf8)
			if err != nil || validator.Body(body) != nil {
				return result, errors.New("SEARCH_FIXED_BODY_INVALID")
			}
			canonical, err := codec.Canonical(body)
			digest := sha256.Sum256(canonical)
			if err != nil || !bytes.Equal(canonical, fixed.BodyJsonUtf8) || !bytes.Equal(digest[:], fixed.BodyHash) {
				return result, errors.New("SEARCH_FIXED_BODY_DIGEST_INVALID")
			}
			provenance, err := codec.Decode(fixed.ProvenanceJsonUtf8)
			if err != nil || validator.Provenance(provenance) != nil {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			canonical, err = codec.Canonical(provenance)
			if err != nil || !bytes.Equal(canonical, fixed.ProvenanceJsonUtf8) {
				return result, errors.New("SEARCH_FIXED_PROVENANCE_INVALID")
			}
			chapter, err := ExtractChapter(body, unit.UnitId, fixed.ResourceIds)
			if err != nil {
				return result, err
			}
			if text.Len()+len(chapter.Text)+1 > 24<<20 {
				return result, errors.New("SEARCH_FIXED_TEXT_LIMIT")
			}
			text.WriteString(chapter.Text)
			text.WriteByte('\n')
			result.Chapters = append(result.Chapters, chapter)
		}
		result.Text = text.String()
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(snapshot)
	if err != nil {
		return result, errors.New("SEARCH_AUTHORITY_DIGEST_INVALID")
	}
	result.Digest = sha256.Sum256(raw)
	return result, nil
}
````
<!-- /lesson-code -->

`ExtractChapter` 提取段落、表格、代码、latex与图片alt/title，不索引链接URL、资源ID、provenance内部身份；它仍核正文出现资源与fixed.ResourceIds相同。`Snapshots.Current(ctx,event)` 调Snapshot(event.Document,event.Sequence,event.Release)，RPC `GetPublicSnapshot` 带minimum及expected release引用，真正返回的是Doc认证的当前正式快照。假如原事件seq=5/release=100，消费时Doc已到seq=7/release=102，取7并保存7的独立Authority；不能强行用100取旧正文。

## 4. 写ES边界、水位脚本与隐藏墓碑

client.go先写Client/New/request/Close，再写mapping，最后projection。ES endpoint不能嵌用户密码或额外路径；请求与响应有界，原网络错误转稳定SEARCH码。EnsureIndex核 `_meta.application/owner/schema` 和strict mapping，遇同名不属于本owner的索引拒绝，不删除重建。

水位不能存在ES long，因为它装不下完整uint64。`Watermark` 生成20位十进制keyword，如9→`00000000000000000009`，10→`00000000000000000010`。`ShouldApply` 比较原uint64：较旧不覆盖；同seq且摘要/可见性/release/policy不同是 `SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT`。ES原子脚本按sequence_key比较，更高替换，相同摘要noop，不同摘要拒绝。Apply成功后重读实际ES State，不能把请求的旧seq冒充现存高水位。

<!-- lesson-code: services/search/internal/domain/index/projection_models.go :: ShouldApply -->
````go
func ShouldApply(current State, next Authority) (bool, error) {
	if next.Digest == ([32]byte{}) ||
		(next.Policy != "PUBLIC" && next.Policy != "SPACE_MEMBERS" && next.Policy != "COLLABORATORS") ||
		(next.Visible && next.Release == 0) || (!next.Visible && next.Release != 0) {
		return false, errors.New("SEARCH_AUTHORITY_INVALID")
	}
	if current.Digest == ([32]byte{}) {
		if current.Sequence != 0 || current.Visible || current.Release != 0 {
			return false, errors.New("SEARCH_INDEX_STATE_INVALID")
		}
		return true, nil
	}
	if next.Sequence < current.Sequence {
		return false, nil
	}
	if next.Sequence == current.Sequence {
		if next.Digest != current.Digest || next.Visible != current.Visible ||
			next.Release != current.Release || next.Policy != current.Policy {
			return false, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return false, nil
	}
	return true, nil
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/client/elastic/projection_client.go :: Client.Apply -->
````go
// Apply以确定Doc ID原子替换投影。成功或noop后重读实际ES水位，
// SQL不能把这次请求的旧水位冒充已经存在的更高水位。
func (c *Client) Apply(ctx context.Context, physical string, value index.Authority) (index.State, error) {
	var result index.State
	if !physicalName.MatchString(physical) {
		return result, errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	projection, err := project(value)
	if err != nil {
		return result, err
	}
	status, data, err := c.request(ctx, "POST", "/"+physical+"/_update/"+strconv.FormatUint(value.Document, 10)+"?retry_on_conflict=3", map[string]any{
		"scripted_upsert": true, "upsert": map[string]any{}, "script": map[string]any{"lang": "painless", "source": applyScript,
			"params": map[string]any{"document": projection}},
	}, 1<<20)
	if err != nil {
		return result, err
	}
	if status != 200 && status != 201 {
		if sameSequenceConflict(data) {
			return result, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return result, errors.New("SEARCH_ES_APPLY_FAILED")
	}
	stored, err := c.getProjection(ctx, physical, value.Document, true)
	if err != nil {
		return result, err
	}
	result, err = stored.state(value.Document)
	if err != nil {
		return result, err
	}
	if result.Sequence < value.Sequence || (result.Sequence == value.Sequence && result.Digest != value.Digest) {
		return result, errors.New("SEARCH_ES_APPLICATION_UNKNOWN")
	}
	return result, nil
}
````
<!-- /lesson-code -->

如果seq=10已经撤下，ES保存visible=false且无正文的seq=10墓碑；后来旧seq=9发布重投必须noop。物理删除墓碑会丢水位并使旧正文复活，因此不能调用delete代替隐藏投影。S02的Processor在ES后才提交SQL应用账本；本课还没有Kafka消费循环。

## 5. 离线规则与06实际投影

```powershell
# 工作目录：自己的lessonRoot；前03的Go底座/共享契约已完成。
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
go -C services/search test -mod=readonly ./internal/domain/event ./internal/domain/index -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Canal原绑定或正式投影规则失败' }
```

精确测试 `TestCanalPreservesFullUnsignedIdentifiersAndBindsPayload`、`TestCanalRejectsUnboundMalformedAndUnknownEvents`、`TestCanalVisibilityEventRetainsStrictPublicReferenceBinding`、`TestSnapshotRejectsUnboundCurrentStateAndHiddenBodyFallback`、`TestFullUnsignedWatermarksSortWithoutSignedWrap`、`TestSameSequenceDifferentAuthorityCannotOverwriteAndOldCannotRevive`；预期全部PASS、没有SKIP。格式错误先核payload和外层字段是否都是规范字符串，隐藏失败先核release/units是否真的为空。

06完整READY后执行 `node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/search/indexing.test.mjs`。精确测试 `产品原Canal主题进入实际Go消费者，切版与撤下保留单调水位和隐藏墓碑` 核真实outbox→Canal→Kafka→Doc→ES→SQL；本课离线规则不证明该链路。ES的 `SEARCH_INDEX_SCHEMA_MISMATCH` 先核本owner与mapping，不能接管旁边索引；Doc未知保持原事件重试，不生成墓碑。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/search/internal/domain/event/events_models.go`：[本课完成文件](../../source/370-events-models.md)。
- `services/search/internal/domain/event/canal.go`：[本课完成文件](../../source/369-canal.md)。
- `services/search/internal/domain/index/projection_models.go`：[本课完成文件](../../source/371-projection-models.md)。
- `services/search/internal/client/document/snapshot.go`：[本课完成文件](../../source/372-snapshot.md)。
- `services/search/internal/client/elastic/client.go`：[本课完成文件](../../source/375-client.md)。
- `services/search/internal/client/elastic/mapping_client.go`：[本课完成文件](../../source/376-mapping-client.md)。
- `services/search/internal/client/elastic/projection_client.go`：[本课完成文件](../../source/377-projection-client.md)。

下一节：[ S02 持久消费、日志身份与代次锁 ](02-consumer-ledger.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/search/internal/domain/event/events_models.go',
    'services/search/internal/domain/event/canal.go',
    'services/search/internal/domain/index/projection_models.go',
    'services/search/internal/client/document/snapshot.go',
    'services/search/internal/client/elastic/client.go',
    'services/search/internal/client/elastic/mapping_client.go',
    'services/search/internal/client/elastic/projection_client.go'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
