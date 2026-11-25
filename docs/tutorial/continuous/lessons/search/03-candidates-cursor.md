# S03｜签名批锚点、PIT与当前权限二次过滤

## 这节课要实现什么

用户输入关键词后翻到下一页，希望结果稳定且不泄露后来失去权限的文档。本课实现搜索候选分页与可验证的翻页凭据。

## 做完后能看到什么

接通后，同一查询的后续页绑定原范围和查询视图，篡改游标或换关键词复用会被拒绝；文档服务仍会过滤当前无权内容，不能把候选数量当作最终可见数量。

## 先想清楚怎样实现

1. 先定义查询范围与排序位置。
2. 建立固定搜索视图并持久登记分页链。
3. 签名绑定每批候选及前后位置，最后交给文档服务二次权限核验后展示。

## 怎样判断这一课完成

S05依赖写齐后运行分页规则及视图测试；06实际搜索、翻页并在途中撤权，检查页面不暴露失效内容。

## 开始写之前

从上一节已经写入的文件继续使用同一个 `$lessonRoot`。本课只输入一份代码：功能讲义用于理解，逐段带写页给出创建命令和完整录入顺序，同名声明不要重复追加。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

| 顺序 | 相对工程根目录的文件 | 本课为什么写它 | 本步代码与插入顺序 |
| --- | --- | --- | --- |
| 1 | `services/search/internal/domain/cursor/pagination_models.go` | 定义Query/Scope校验、精确sort及持久Control状态；SQL尾位置保留原JSON词法。 | [逐段带写](../../write/368-pagination-models.md) |
| 2 | `services/search/internal/domain/cursor/anchor.go` | 签名Chain/Before/After/Count/More/Digest，区分NEXT与REPLAY动作并拒绝帧篡改。 | [逐段带写](../../write/366-anchor.md) |
| 3 | `services/search/internal/domain/cursor/digest.go` | 对完整有序候选与批边界求摘要，绑定physical/document/release/sequence和原sort字节。 | [逐段带写](../../write/367-digest.md) |
| 4 | `services/search/internal/repository/cursor/registry.go` | 保存游标Record状态、scope/session/query/代次/PIT及绝对期限，限定全局OPEN/CLOSING链128与登记行1024配额。 | [逐段带写](../../write/384-registry.md) |
| 5 | `services/search/internal/repository/cursor/pagination_queries.go` | 领取/保存/到期回收单链记录与原Control，查询、关闭和恢复共用SQL事实。 | [逐段带写](../../write/383-pagination-queries.md) |
| 6 | `services/search/internal/client/elastic/candidates.go` | 在固定physical打开PIT，取100候选及lookahead，验证分片完整性和最新PIT句柄。 | [逐段带写](../../write/374-candidates.md) |
| 7 | `services/search/internal/logic/candidates/candidates_service.go` | 编排预留→打开→查询→确认，nonce拒绝迟到结果；未知句柄保持配额至原绝对期限。 | [逐段带写](../../write/391-candidates-service.md) |
| 8 | `services/search/internal/svc/candidates.go` | 从本服务配置装配独立ES/Guard及游标签名密钥，管理Ready与关闭生命周期。 | [逐段带写](../../write/403-candidates.md) |
| 9 | `services/search/internal/worker/cursors.go` | 定时清理到期或未知单链，逐项有界处理坏行，恢复完成才允许READY。 | [逐段带写](../../write/401-cursors.md) |

## 2. 模型先分清“候选”与“有权显示的搜索结果”

先写Query、Control与Sort校验，再写Batch/Codec，最后DigestCandidates。输入使用现成 `pb.SearchCandidatesRequest`，Query含原UTF-8文本、QueryDigest、PageSize与Scope。QueryDigest对原文本求SHA-256，不能trim后另算；文本最多500 Unicode码点，PageSize规则最多100，实际用例固定要求100。ScopeHash、Anonymous、SessionRef和有序SpaceIds由受mTLS保护的Doc传入，scope自报不是权限；非匿名SessionRef须32字节非零，匿名必须空。

Batch字段Chain[32]、Before/After原sort JSON、Count、More、Digest[32]绑定一份完整有序候选批。Query/scope/session/physical/generation/绝对expires存Chain对应SQL行。Codec用32字节非零密钥做用途分离HMAC；Envelope区分ActionNext和ActionReplay，framing最多768字节。Open返回独立切片，不让调用者修改验签原输入。

<!-- lesson-code: services/search/internal/domain/cursor/anchor.go :: Batch -->
````go
// Batch绑定一个已经完整取得的批次，而非可随意指定的ES位置。nil位置表示首批起点。
// Query/scope/session/物理代次/绝对期限由Chain对应SQL行绑定，不重复塞进签名token。
// Digest由上层对完整有序候选计算；此层仅检查非零并签名，不能代替ES完整性验证。
type Batch struct {
	Chain  [32]byte
	Before json.RawMessage
	After  json.RawMessage
	Count  uint8
	More   bool
	Digest [32]byte
}
````
<!-- /lesson-code -->

<!-- lesson-code: services/search/internal/domain/cursor/anchor.go :: Codec.Seal -->
````go
func (c *Codec) Seal(batch Batch) ([]byte, error) {
	if c == nil || len(c.key) != sha256.Size {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	if err := validateBatch(batch); err != nil {
		return nil, err
	}
	data := make([]byte, 0, anchorMinimum+len(batch.Before)+len(batch.After))
	data = append(data, anchorMagic[:]...)
	data = append(data, batch.Chain[:]...)
	more := byte(0)
	if batch.More {
		more = 1
	}
	data = append(data, batch.Count, more)
	data = append(data, batch.Digest[:]...)
	for _, position := range []json.RawMessage{batch.Before, batch.After} {
		data = append(data, byte(len(position)))
		data = append(data, position...)
	}
	return append(data, c.signature(data)...), nil
}
````
<!-- /lesson-code -->

sort只能是 `[score,"规范uint64文档ID",shard_uint64]` 三元组，保留原数值词法和原JSON字节。ID=`9007199254740993` 不能float64重编码。DigestCandidates覆盖before/after/more及每项physical/document/release/sequence/sort；PIT、nonce、时间会轮换，不属于批摘要。篡改release或候选顺序也必须导致重放不一致，不能只签裸页码。

## 3. 数据库单链登记与ES固定PIT

先写registry.Record/scan/validate/Create，再写pagination_queries的Lock/Save/Due/DeleteClosed等。Record绑定Hash、PIT、Physical、Generation、ScopeHash、QueryHash、Session、PageSize、Expires、State与Control。状态为OPEN/CLOSING/CLOSED；Control为OPENING/INFLIGHT/IDLE，带Nonce/Deadline/Tail/RetryAt/Uncertain。Tail外层SQL JSON以字符串保存原sort词法，不能把Control直接Marshal再读float。

Create要求已持generation排他锁，锁内统计全局：OPEN/CLOSING合计最多128、所有登记行最多1024；OPENING和未知CLOSING也占额。Expires是首次分配绝对时间、TTL最多两分钟，后续NEXT不能延长。未知open可能已经创建ES资源，不能因为本地PIT为空就立即归还配额。

<!-- lesson-code: services/search/internal/repository/cursor/registry.go :: Record -->
````go
// Record是单链的持久事实；绑定字段不随NEXT/REPLAY或重试更新，Expires也是原绝对期限。
type Record struct {
	Hash       [32]byte
	PIT        string
	Physical   string
	Generation uint64
	ScopeHash  [32]byte
	QueryHash  [32]byte
	Session    []byte
	PageSize   uint32
	Expires    time.Time
	State      string
	Control    domain.Control
}
````
<!-- /lesson-code -->

ES client `OpenPIT(ctx,physical,remaining)` 只对固定physical打开；`Candidates(ctx,pit,query,after,size,remaining)` 取size+1，固定_score desc/document_id asc/_shard_doc asc，查询phrase且slop=0，visible=true。全部响应包括第101项probe先核合法性、分片完整、timeout=false及最新pit_id，再返回前100项，More=true；End是第100项sort，probe留给下一批，不能吞掉。空终批End复制原after。remaining来自SQL绝对Expires，keep_alive也只取剩余时间。

<!-- lesson-code: services/search/internal/client/elastic/candidates.go :: Client.Candidates -->
````go
func (c *Client) Candidates(ctx context.Context, pit, query string, after json.RawMessage, size int, remaining time.Duration) (CandidateBatch, error) {
	var result CandidateBatch
	if !validPIT(pit) || !utf8.ValidString(query) || strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 500 ||
		size < 1 || size > 100 || (len(after) != 0 && cursor.ValidateSort(after) != nil) {
		return result, errors.New("SEARCH_ES_CANDIDATE_REQUEST_INVALID")
	}
	keepAlive, err := pitRemaining(remaining)
	if err != nil {
		return result, err
	}
	request := map[string]any{
		"size": size + 1, "track_total_hits": false,
		"_source": []string{"document_id", "release_id", "sequence"},
		"pit":     map[string]any{"id": pit, "keep_alive": keepAlive},
		"sort":    []any{map[string]string{"_score": "desc"}, map[string]string{"document_id": "asc"}, map[string]string{"_shard_doc": "asc"}},
		"query": map[string]any{"bool": map[string]any{
			"filter": []any{map[string]any{"term": map[string]any{"visible": true}}},
			// 中文标准分词会拆出单字；默认OR会把“设定”误匹配到只有“固定”的正文。
			// 要求同一字段中连续的词序，复用原索引，不以标题去重掩盖独立文档。
			"must": []any{map[string]any{"multi_match": map[string]any{
				"query": query, "type": "phrase", "slop": 0, "fields": []string{"title", "text", "chapters.title"},
			}}},
		}},
	}
	if len(after) != 0 {
		// RawMessage只保留位置的原JSON词法，不能经float64重新编码高位shard序号。
		request["search_after"] = after
	}
	requestCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	status, data, err := c.request(requestCtx, "POST", "/_search?allow_partial_search_results=false", request, candidateResponseLimit)
	if err != nil {
		return result, err
	}
	var response struct {
		PIT        string     `json:"pit_id"`
		TimedOut   *bool      `json:"timed_out"`
		Terminated bool       `json:"terminated_early"`
		Shards     *pitShards `json:"_shards"`
		Hits       *struct {
			Hits json.RawMessage `json:"hits"`
		} `json:"hits"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !validPIT(response.PIT) {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	result.PIT = response.PIT
	if response.TimedOut == nil || *response.TimedOut || response.Terminated || !response.Shards.complete() ||
		response.Hits == nil || requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	var hits []candidateHit
	if json.Unmarshal(response.Hits.Hits, &hits) != nil || hits == nil || len(hits) > size+1 {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	// 先验证整个响应，包括第B+1项，之后才能交付批次；损坏probe不能伪造成has_more。
	candidates := make([]*pb.SearchCandidate, 0, len(hits))
	seen := make(map[uint64]bool, len(hits))
	physical := ""
	for _, hit := range hits {
		candidate, err := hit.candidate()
		if err != nil || seen[candidate.GetDocumentId()] || (physical != "" && physical != hit.Physical) {
			return result, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
		}
		physical = hit.Physical
		seen[candidate.DocumentId] = true
		candidates = append(candidates, candidate)
	}
	if requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	result.More = len(candidates) > size
	if result.More {
		candidates = candidates[:size]
	}
	result.Candidates = candidates
	result.End = append(json.RawMessage(nil), after...)
	if len(candidates) != 0 {
		result.End = append(json.RawMessage(nil), candidates[len(candidates)-1].SortJsonUtf8...)
	}
	return result, nil
}
````
<!-- /lesson-code -->

## 4. 查询、关闭与未知恢复的实际次序

在Candidates用例先写New/enter/transact/keyLocks，再写reserve/beginQuery/retainOpened/confirmQuery/fail，最后SearchCandidates与关闭/清理。接口Indexes提供OpenPIT/Candidates/ClosePIT；Generations提供Ready/Do。新链先SQL reserve并占配额，再generation.Do核原physical与代次，ES打开PIT→先把句柄存SQL→查候选→求批摘要→签Batch→SQL confirm。每步使用原nonce；Close或期限收紧后迟到查询不能重新OPEN，返回 `SEARCH_CURSOR_CONFIRMATION_STALE`。

NEXT从anchor.After读取下一批；REPLAY从Before重新取同一批，必须Count/More/Digest/After全相等。网络回复丢失时Doc可REPLAY原批重新过滤，不能假定已给浏览器的半页可继续使用。Search返回Candidates、Continuation、IndexGeneration、PhysicalIndex、ExpiresUnixMs、HasMore、Complete；它没有给浏览器的正式标题、摘要或下载资格。

<!-- lesson-code: services/search/internal/logic/candidates/candidates_service.go :: Candidates.SearchCandidates -->
````go
func (c *Candidates) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	query, err := domain.ValidateQuery(request)
	if err != nil {
		return nil, err
	}
	// HTTP page大小属于Doc；Search固定批100，避免小批More与已冻结批锚点出现两套语义。
	if query.PageSize != 100 {
		return nil, errors.New("SEARCH_CANDIDATE_BATCH_SIZE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.enter(ctx); err != nil {
		return nil, err
	}
	defer c.leave()
	var anchor domain.Batch
	var action domain.Action
	var record repository.Record
	newChain := len(request.Cursor) == 0
	if newChain {
		record, anchor.Chain, err = c.reserve(ctx, query)
	} else {
		var signed []byte
		action, signed, err = domain.ParseEnvelope(request.Cursor)
		if err == nil {
			anchor, err = c.codec.Open(signed)
		}
	}
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(anchor.Chain[:])
	release, err := c.queries.acquire(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer release()
	var reply *pb.SearchCandidatesReply
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		var err error
		record, err = c.beginQuery(ctx, hash, query, target, newChain)
		if err != nil {
			return err
		}
		before := anchor.After
		if newChain || action == domain.ActionReplay {
			before = anchor.Before
		}
		if newChain {
			pit, openErr := c.indexes.OpenPIT(ctx, record.Physical, time.Until(record.Expires))
			if openErr != nil {
				c.fail(ctx, record, pit, true)
				return openErr
			}
			record.PIT = pit
			// Open成功的句柄先落SQL再查候选；中间退出仍能沿原行关闭，不重复分配资源。
			if err := c.retainOpened(ctx, record); err != nil {
				c.fail(ctx, record, pit, false)
				return err
			}
		}
		batch, queryErr := c.indexes.Candidates(ctx, record.PIT, query.Query, before, 100, time.Until(record.Expires))
		if batch.PIT != "" {
			record.PIT = batch.PIT
		}
		if queryErr != nil {
			c.fail(ctx, record, record.PIT, true)
			return queryErr
		}
		for _, candidate := range batch.Candidates {
			if candidate.PhysicalIndex != record.Physical {
				c.fail(ctx, record, record.PIT, true)
				return errors.New("SEARCH_CURSOR_PHYSICAL_MISMATCH")
			}
		}
		digest, err := domain.DigestCandidates(before, batch.End, batch.Candidates, batch.More)
		if err != nil {
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		if !newChain && action == domain.ActionReplay && !sameBatch(anchor, batch, digest) {
			c.fail(ctx, record, record.PIT, false)
			return errors.New("SEARCH_CURSOR_REPLAY_CHANGED")
		}
		signed, err := c.codec.Seal(domain.Batch{Chain: anchor.Chain, Before: before, After: batch.End, Count: uint8(len(batch.Candidates)), More: batch.More, Digest: digest})
		if err != nil {
			c.fail(ctx, record, record.PIT, false)
			return err
		}
		if err := c.confirmQuery(ctx, record, batch.End); err != nil {
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		reply = &pb.SearchCandidatesReply{Candidates: batch.Candidates, Continuation: signed, IndexGeneration: record.Generation,
			PhysicalIndex: record.Physical, ExpiresUnixMs: record.Expires.UnixMilli(), HasMore: batch.More, Complete: true}
		return nil
	})
	if err != nil {
		// 代次拒绝发生在ES调用之前时，同样收尾原预留；没有新查询可继续这条链。
		if newChain {
			c.fail(ctx, record, record.PIT, true)
		}
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, errors.New("SEARCH_CANDIDATE_REPLY_UNKNOWN")
	}
	return reply, nil
}
````
<!-- /lesson-code -->

closeHash先准入再短事务领取原claim、调用ES ClosePIT，再短事务核原nonce确认。已知句柄关闭不证明未知轮换句柄不存在，Uncertain保留CLOSING及配额至原Expires；超过期限应用链CLOSED，不承诺ES内部reaper立刻全排空。Cleanup每轮最多16条、每条有独立预算，坏首行不能饿死后续。svc创建独立client/Guard/codec，worker.RunCursors定期Cleanup，recovered后才Ready。

具体输入：旧链绑定query“设定”、scope=A、physical=g1，续接换“固定”或scope=B是 `SEARCH_CURSOR_BINDING_MISMATCH`；别名已经切g2，旧链查询是 `SEARCH_CURSOR_STALE`，但可以沿原PIT关闭。Open超时且无句柄，保留未知占额到原期限；不能开一个新PIT替原链并保持旧签名。

## 5. HTTP实际二次权限过滤在Doc

复用03已写的 `services/document/internal/client/search.go` 与 `internal/logic/document/search_service.go`。浏览器走Doc搜索HTTP，Doc核当前身份/空间，调用本服务SearchCandidates；`collectSearchPage` 对每批候选重新读取当前正式版本/受众/来源，验证原release/seq及固定正文，剔除现在无权的候选，最后联合输出门闩再核资格。ES visible=true和SearchScopeHash都不能代替它。

例如Search一次返回100项，当前用户只可读3项，Doc请求limit=2：显示前2项，第3项做lookahead，HTTP游标保存批锚点和prefix在第3项之前。即使该批HasMore=false，下一HTTP页仍需REPLAY原终批后从prefix重新核当前权限，不能漏掉剩余可读后缀。下一页期间撤权，第3项不显示；任何候选RPC/权限/正文读取未知，整页SEARCH_INCOMPLETE，不回半页或假exhausted。不存在“最多看1000候选就成功空页”的截断。

## 6. 离线与06真实分页检查

```powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
go -C services/search test -mod=readonly ./internal/domain/cursor -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '签名批锚点、Control或scope规则失败' }
```

精确规则名还包括 `TestAnchorRejectsEveryByteMutationTruncationExtraBytesAndOtherPurpose`、`TestSortRequiresExactThreeTypesAndFullCanonicalUint64`、`TestControlTailStringSurvivesOuterSQLJSONReorderAnd128ByteBound`、`TestDigestCandidatesBindsEveryOrderedFrozenFieldAndRawSort`。本课仅运行 cursor 包，预期PASS、skip=0。ES client 包的现成测试还引用 S05 的 aliases_client.go；-run 只筛执行，不跳过同包测试文件的编译，须等 S05 全部实现后再运行下列命令。这批使用测试HTTP服务，不是实际ES集群。

```powershell
# 先完成 S05；工作目录仍是自己的 lessonRoot。
go -C services/search test -mod=readonly ./internal/client/elastic -run '^TestPIT(CandidatesLookaheadLatestHandleAndOriginalSort|CandidatesRejectPartialAndBrokenBindingsIncludingProbe|OpenCloseAndFiniteRemainingBudget|UnknownCloseAndOpenPartialAreNotSuccessful)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'PIT完整响应规则失败' }
```

06完整READY后运行 `node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/search/pages.test.mjs tests/search/pages-recovery.test.mjs`，精确测试 `实际搜索页：固定正式摘要、当前受众、终批后缀与加密绑定` 与 `真实Search进程不可达时HTTP整页失败，恢复同owner后新链成功`。后者有真实进程故障，串行运行。若需要SQL/ES未知句柄边界，设置06自己的COLLAB_TEST_SEARCH_CONFIG后选 `go -C services/search test -mod=readonly ./internal/logic/candidates -run '^TestActualUnknownOpenRetainsQuotaUntilOriginalExpiryThenRecovers$' -count=1 -v`。CURSOR_STALE新建查询链，原链仍应关闭；QUOTA_EXCEEDED检查CLOSING/Expires和worker，不删除登记行；分片不完整保持失败，不能放宽Complete。

## 章末完整答案核对

完成上述小步后再对照本课答案，核package/import、声明次序、函数体及中文原因注释。完整答案不承担创建工程或跳步补业务的职责。

- `services/search/internal/domain/cursor/pagination_models.go`：[本课完成文件](../../source/368-pagination-models.md)。
- `services/search/internal/domain/cursor/anchor.go`：[本课完成文件](../../source/366-anchor.md)。
- `services/search/internal/domain/cursor/digest.go`：[本课完成文件](../../source/367-digest.md)。
- `services/search/internal/repository/cursor/registry.go`：[本课完成文件](../../source/384-registry.md)。
- `services/search/internal/repository/cursor/pagination_queries.go`：[本课完成文件](../../source/383-pagination-queries.md)。
- `services/search/internal/client/elastic/candidates.go`：[本课完成文件](../../source/374-candidates.md)。
- `services/search/internal/logic/candidates/candidates_service.go`：[本课完成文件](../../source/391-candidates-service.md)。
- `services/search/internal/svc/candidates.go`：[本课完成文件](../../source/403-candidates.md)。
- `services/search/internal/worker/cursors.go`：[本课完成文件](../../source/401-cursors.md)。

下一节：[ S04 先封存L，再导入签名快照并种子回填 ](04-rebuild-snapshot.md)。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

```powershell
# 工作目录：00 中已设置的自己的学习工程。不会覆盖已有源码。
Set-Location -LiteralPath $lessonRoot
$lessonFiles = @(
    'services/search/internal/domain/cursor/pagination_models.go',
    'services/search/internal/domain/cursor/anchor.go',
    'services/search/internal/domain/cursor/digest.go',
    'services/search/internal/repository/cursor/registry.go',
    'services/search/internal/repository/cursor/pagination_queries.go',
    'services/search/internal/client/elastic/candidates.go',
    'services/search/internal/logic/candidates/candidates_service.go',
    'services/search/internal/svc/candidates.go',
    'services/search/internal/worker/cursors.go'
)
foreach ($lessonFile in $lessonFiles) {
    $lessonPath = Join-Path $lessonRoot $lessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $lessonPath)) {
        New-Item -ItemType File -Path $lessonPath | Out-Null
    }
}
```
