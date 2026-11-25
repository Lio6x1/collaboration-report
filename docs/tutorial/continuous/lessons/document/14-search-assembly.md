# D14｜搜索资格、快照重建与文档服务总装

## 这节课要实现什么

用户搜索正式文档时，只应该看到自己现在有权阅读的结果。本课完成文档服务一侧的搜索权限过滤、重建快照和全部文档功能接线。

## 做完后能看到什么

本课结束可构建完整文档服务；06接通搜索后，索引候选还要经过当前权限检查才展示正文摘要，内部草稿不会因为进入索引就公开。重建工具输出绑定原任务的固定材料。

## 先想清楚怎样实现

1. 先生成正式版索引事件并请求搜索候选。
2. 再依据当前文档事实过滤和重建有权摘要。
3. 补齐重建快照及运维入口，最后登记文档HTTP、RPC与后台任务。

## 怎样判断这一课完成

本课执行文档全包构建及离线测试，06与S01至S05联调搜索，同时回到D01至D13的真实功能关口验收。

## 开始写之前

继续[D13](./13-account-retirement.md)，在00建立的同一个 **$lessonRoot** 里亲写；不取历史checkout。索引事件先限定为正式引用；搜索和重建从Doc权威事实建立真实正文输出。

本课先建立模型和消费者接口，再按业务步骤填SQL与logic，最后写真实适配。**功能课会引用后课同包声明；整个domain/document或logic/document包未齐时，暂不执行该包测试/构建，不填假桩。** D14才是文档模块首次整体构建点；离线规则测试命令列在各课，满足包齐前置后回到对应课执行。转换器exporter引用logic，也要等logic齐全。

## 1. 理解文件分工，再逐份创建

先读下面的文件职责与功能推演。讲到某份文件时，打开它的“逐段带写”页，按该页创建命令建立文件，再从第1步录入。**不用现在一次创建全部文件，也不用先执行章末的批量脚本。** 同一声明只写一次；课末核对是否完成本课全部文件。

先创建 **services/document/internal/domain/document/search_models.go**，用途是索引事件先限定为正式引用；搜索和重建从Doc权威事实建立真实正文输出。逐份创建时使用对应分步操作页中的命令，保留自己已经写过的内容。$lessonRoot沿用00的学习工程绝对路径，不能设成教师现成产品目录。

每份操作页从package/import开始，给出准确插入边界及该步完整代码。先写下文说明的模型/接口段，再按下文用例顺序补仓储、logic与适配；本课末表列出必须写完的每个文件。正文代码块与操作页中的同名声明是同一步，**只输入一次**。同文件其他完整声明继续按操作页顺序填，不能留下省略函数体或另造DTO。

## 2. 先写正式索引引用与候选消费者

search_models.go先写IndexEvent、编码、水位及SearchQuery/SearchPageCursor/SearchCursorCodec等全部声明。IndexEvent只携带event ID/type、document、sequence、release/publication epoch和发生时间；不含正文、策略或身份证明。event_seq与publication_epoch分开：KB可见性变化会发索引事件，但不是再次发布。VisibilityIndexEventID绑定原operation和document，fanout不能为所有文档复用同一outbox主键。

SearchCandidatesPeer只返回Search候选及关闭游标接口：

<!-- lesson-code: services/document/internal/logic/document/search_service.go :: SearchCandidatesPeer -->
````go
type SearchCandidatesPeer interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	CloseCursor(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}
````
<!-- /lesson-code -->

SearchCandidates(ctx,pb request)返回候选页及error，不能授予阅读；CloseCursor释放原候选查询链。真实适配在client/search.go；Doc用自身mTLS身份调用Search，不带Search数据库或ES凭据。

## 3. 候选扫描后由Doc重建有权摘要

先写search_queries.go的当前PublicationFacts、固定release/manifest/正文metadata，再写search_snippet.go的完整语义正文提取，最后完成search_service.go的scan、batch评估、fixed summary与最终输出事务。

<!-- lesson-code: services/document/internal/logic/document/search_service.go :: SearchPages.Page -->
````go
func (s *SearchPages) Page(ctx context.Context, actor *Actor, query domain.SearchQuery, handler string, bind BindOutput) (dto.SearchResultPage, error) {
	fail := func(err error) (dto.SearchResultPage, error) { return dto.SearchResultPage{}, searchIncomplete(err) }
	if s == nil || s.db == nil || s.peer == nil || s.cursors == nil || s.validator == nil || s.outputs == nil || s.identity == nil || s.access == nil {
		return fail(nil)
	}
	// 在任何SQL读取前准入，保留原8连接池给消费、关闭与其他业务；不等持锁后再排队。
	select {
	case s.admission <- struct{}{}:
		defer func() { <-s.admission }()
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	var session []byte
	if actor != nil {
		if actor.Proof == nil || actor.ID == 0 || actor.Proof.ActorId != actor.ID {
			return fail(nil)
		}
		session = actor.Proof.SessionRef
		if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return fail(err)
		}
	}
	binding, err := query.Binding(actor == nil, session)
	if err != nil {
		return fail(err)
	}
	scope := &pb.SearchScope{ScopeHash: hex.EncodeToString(binding[:]), Anonymous: actor == nil, SessionRef: append([]byte(nil), session...)}
	if query.SpaceID != 0 {
		scope.SpaceIds = []uint64{query.SpaceID}
	}
	var start *domain.SearchPageCursor
	if query.Cursor != "" {
		cursor, err := s.cursors.Open(query.Cursor, binding, time.Now())
		if err != nil {
			return fail(err)
		}
		start = &cursor
	}
	var bodyBytes uint64
	audience := newSearchAudience(s.identity, s.access, actor)
	collection, err := collectSearchPage(ctx, s.peer, query, scope, binding, start, func(ctx context.Context, candidates []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		facts, err := s.batchFacts(ctx, candidates)
		if err != nil {
			return nil, err
		}
		if actor != nil {
			if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
				return nil, err
			}
		}
		return func(ctx context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
			current, present := facts[candidate.DocumentId]
			if !present {
				return nil, nil
			}
			policy, visible, err := qualifySearchCandidate(candidate, current, query.SpaceID)
			if err != nil || !visible {
				return nil, err
			}
			access, sources, allowed, err := audience.authorize(ctx, current, policy)
			if err != nil || !allowed {
				return nil, err
			}
			summary, nextBytes, err := s.fixedSearchSummary(ctx, current, candidate.ReleaseId, query.Text, bodyBytes)
			if err != nil {
				return nil, err
			}
			bodyBytes = nextBytes
			return &searchPageEntry{Summary: summary, Facts: current, Access: access, Sources: sources}, nil
		}, nil
	})
	if err != nil {
		return fail(err)
	}
	page := dto.SearchResultPage{Items: make([]dto.SearchResult, 0, query.Limit), HasMore: collection.Next != nil}
	for i, entry := range collection.Entries {
		if i < query.Limit {
			page.Items = append(page.Items, entry.Summary)
		}
	}
	if collection.Next != nil {
		cursor, err := s.cursors.Seal(*collection.Next)
		if err != nil {
			return fail(err)
		}
		page.NextCursor = &cursor
	}
	// 完全耗尽的匿名空页没有标题、正文或游标，不捏造账户/单Doc输出准入。
	if actor == nil && len(collection.Entries) == 0 && collection.Next == nil {
		return page, nil
	}
	if err = s.commitSearchPage(ctx, actor, collection.Entries, audience.finalProofs(), collection.Until, handler, bind); err != nil {
		return fail(err)
	}
	// 正常EOF保留Search原链至原期限，支持HTTP回复丢失后的同锚点重试。
	// 不在最后一批已输出时提前Close，从而破坏终批后缀或前页重试。
	return page, nil
}
````
<!-- /lesson-code -->

Page(ctx,actor可空,query,handler,bind)返回生成SearchResultPage及error。Search只给排序候选；Doc逐批一致读取当前正式事实，按PUBLIC/SPACE_MEMBERS/COLLABORATORS核资格，再从原固定release正文提取标题、章节与Unicode摘要。不能用ES缓存摘要直接输出，也不能把release_id当旧历史阅读授权。

输入推演：候选D是R1/seq10，Doc已切R2/seq11，qualifySearchCandidate比较当前正式指针/水位后直接丢弃该旧候选，不读R1正文，也不拿MAIN补摘要。一个候选明确无权可以跳过，但Account/SQL/RPC未知、游标过期、读取中资格改变都要整页失败，不能输出已收集的半页。lookahead用于判断has_more，最后一批不能吞掉；大于1000个无权候选后仍须继续真实扫描，不能人工截断当耗尽。

生成摘要前先按metadata预留整个文档正文预算，再读body；不能读超额大正文后才拒绝。UTF-8/Unicode、公式、表格、图片语义沿真实内容契约，私有attrs不进摘要。cursor用加密认证、随机nonce隐藏候选排序锚点，绑定actor/查询/过滤/绝对期限，翻页不续命。最终持所有来源锁，重核正式水位及资格，再登记两段输出；中途撤权不能继续交付旧摘要。

## 4. 固定重建快照与三个运维入口

repository/snapshot/job.go存原prepare/签名者摘要/条目摘要/seal，entries.go提供同一事务的CurrentPage和已封存StoredPage。logic/snapshot/capture.go先写Service及Capture：

<!-- lesson-code: services/document/internal/logic/snapshot/capture.go :: Service.Capture -->
````go
// Capture 在一次 RR 事务内同时保存完整条目和签名。任何扫描/签名错误均回滚；
// commit 回执未知时不导出本次内存结果，调用者只能带完全相同的原号和输入重试。
func (s *Service) Capture(ctx context.Context, operation string, prepareEnvelope []byte) ([]byte, error) {
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || len(prepareEnvelope) == 0 || len(prepareEnvelope) > rebuildartifact.MaxEnvelopeBytes {
		return nil, ErrInput
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()

	// 锁定读取不建立业务表的一致视图；先检查持久接受事实，证书到期不能使
	// 已接受的原操作失去恢复能力。只有明确不存在才允许按当前信任链首次准入。
	job, err := repo.Lock(ctx, tx, operation)
	var accepted rebuildartifact.Verified
	if errors.Is(err, repo.ErrNotFound) {
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		accepted, err = s.Verifier.Verify(prepareEnvelope, rebuildartifact.PrepareV1)
		if err != nil {
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
		fingerprint, err := hex.DecodeString(accepted.SignerFingerprint)
		if err != nil || len(fingerprint) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		var signerDigest [32]byte
		copy(signerDigest[:], fingerprint)
		job, err = repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if job.Operation != operation || !bytes.Equal(job.Prepare, prepareEnvelope) || sha256.Sum256(prepareEnvelope) != job.PrepareDigest {
			return nil, ErrConflict
		}
		accepted, err = rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
		if err != nil {
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
	}
	if job.State == "SEALED" {
		// 只返回已经提交的原签名；回滚释放读取锁即可，不重签也不重扫业务表。
		return append([]byte(nil), job.Seal...), nil
	}
	// 当前签名身份仅用于新封存。延迟加载让证书轮换/过期与原制品恢复分离，
	// 同时在整库扫描前报告配置错误，避免已知无法签名仍持有长事务。
	signer := s.Signer
	if signer == nil && s.LoadSigner != nil {
		signer, err = s.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil {
		return nil, rebuildartifact.ErrConfig
	}

	hash := sha256.New()
	var count, after uint64
	for {
		// 首次普通 SELECT 在 prepare 已验签并绑定后建立 S，后续所有页仍在同一事务。
		entries, err := repo.CurrentPage(ctx, tx, after, 500)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		for _, entry := range entries {
			if entry.Document <= after || count == ^uint64(0) {
				return nil, ErrIncomplete
			}
			line, err := entry.Line()
			if err != nil {
				return nil, err
			}
			hash.Write(line)
			after, count = entry.Document, count+1
		}
		if err := repo.InsertEntries(ctx, tx, operation, entries); err != nil {
			return nil, err
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	payload, err := (rebuildmeta.Seal{Version: 1, PrepareRef: accepted.Reference,
		Count: strconv.FormatUint(count, 10), EntriesDigest: hex.EncodeToString(digest[:])}).Bytes()
	if err != nil {
		return nil, err
	}
	envelope, err := signer.Sign(rebuildartifact.SnapshotV1, payload)
	if err != nil {
		return nil, err
	}
	if err := repo.Seal(ctx, tx, operation, count, digest, envelope); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ErrCommitUnknown
	}
	return envelope, nil
}
````
<!-- /lesson-code -->

Capture(ctx,operation,prepareEnvelope)在一个RR事务内验原prepare并绑定operation，首次普通SELECT建立一致视图S，按document递增分页保存完整条目、算摘要和计数、用真实Doc签名封存再Commit。已SEALED重试返回原签名字节，不能重扫或重新签；Commit未知不导出内存seal，只能以相同operation/prepare重试。

snapshot_service.go的Export只导出原封存条目并核count/digest，部分Write不是可导入制品；调用者必须受控临时写入、Sync后发布entries，最后发布原seal。RebuildRequest从原已提交seal还原generation、L/descriptor、ref与fingerprint，CLI不让人重新填代次或L。

cmd/search-snapshot按 -config、-operation、-prepare-ref从本服务受控inbox读内容地址并输出reference；cmd/search-rebuild按 -config、-operation、-snapshot-ref、-action start/status调用Search原重建任务；RPC丢回复继续同号status/start。它们用本学习实例 .runtime/services/document.json，prepare/snapshot引用来自05实际生成交接流程，不手工编造。

cmd/resourcesnapshot属于D08冷备份工具，只在监督器已证全部应用写者退出且Canal停止后读真实对象清单，目标在本owner .runtime/backups内；它不是搜索快照，也不是在线上传接口。本轮只写入口，不执行运维动作。

## 5. 最后把路由、RPC和所有worker接齐

按表完成四组svc文件，再写bootstrap/document.go、handler/routes.go及main.go。workspace_wiring构造Identity/Access、目录与事务；content_workflows_wiring构造送审、审核、合并、主线/支线/结构/冲突消费者；resources_outputs_wiring构造真实对象存储、公开阅读、导出和输出；lifecycle_wiring构造关闭、清理、迁移与治理。构造函数返回error必须逐层处理，不能nil依赖也启动HTTP。

bootstrap.RegisterDocument先建outputs/fences及Account/Search连接，失败关闭已创建连接；构造各用例、InitializeQuotas、注册ManagedTasks；ReadyExtra同时核输出与原恢复绑定，再给rpc各字段赋真实实现、注册handler.Routes，最后以store.Context启动worker。Ready不只是端口能连；缺原绑定必须阻止就绪，不能清原任务逃过检查。

三个worker文件完整保留：content_workflows_workers驱动Submissions/Merges/MainControls/PullMain/StructurePreviews/DraftControls/ReverseChanges/Resolutions；identity_notifications_workers驱动Authorizations/Departures、Notifications、AccountNotifications；publication_outputs_workers驱动Publications、Resources和有限Exports；lifecycle_workers驱动Purges/RetainedGovernance，personal_transfer.go在D12。main创建server.NewDocument，注册生成DocumentInternalServer，再把bootstrap.RegisterDocument传platform.Main。

用户搜索→GET /v1/search→handler.SearchPages.Page→logic.Page→真实SearchCandidates RPC→Doc当前私有SQL/固定正文→最终输出。用户送审/导出的恢复链也必须在routes和bootstrap找到原handler、svc及worker；仅写logic不等于功能可达。路由表全部按操作页输入，不缩成示范三条。

## 6. 首次整体构建与分层验证

到本课文件全部完成、01/02契约与平台模块已齐时，首次允许构建整个文档模块：

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
go -C services/document build -mod=readonly ./...
if ($LASTEXITCODE -ne 0) { throw '文档模块未构建成功' }
go -C services/document test -mod=readonly ./internal/domain/document -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '文档领域规则失败' }
go -C services/document test -mod=readonly ./internal/logic/document -run '^(TestSearchCollectionUnknownAndCancellationHaveNoHalfPage|TestSearchCollectionPreservesTerminalLookahead|TestSearchSnippetUnicodeLimit|TestSearchQualificationUsesCurrentReleaseSequenceAndPolicy)$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '搜索资格/完整页规则失败' }
python $coursePrepare --target $lessonRoot --check 03
if ($LASTEXITCODE -ne 0) { throw '196个文档文件未齐' }
~~~

精确离线用例在search_candidates_test.go、search_snippet_test.go、search_qualification_test.go。构建失败先核包内声明是否都输入、import和构造签名，尤其exporter→logic及svc跨功能前向引用；不要删函数/加nil假桩。03清单检查只核文件完整，不证明写入顺序可实际连续复原。

完成04协作与05搜索后按06启动自己的独立实例，再执行：

~~~powershell
node --no-maglev --test --test-reporter=tap --test-concurrency=1 tests/search/indexing.test.mjs tests/search/pages-route.test.mjs tests/search/visibility.test.mjs
if ($LASTEXITCODE -ne 0) { throw '真实索引/搜索/可见性失败' }
~~~

产品原Canal主题进入实际Go消费者，切版与撤下保留单调水位和隐藏墓碑 核真实Kafka/ES/SQL；search HTTP route returns an exhausted anonymous page and rejects forged bindings 核真实HTTP。重建prepare→seed→start→replay→switch测试有固定artifact和前课原任务前置，不能单独拿rebuild-switch.test.mjs在新实例运行：它读取artifacts/batches/06/rebuild-prepare-operation.json，应按05重建操作流程先产生同owner原事实。切换验收“原重建任务完整核验后切换，固定结果可重复查询且新发布进入新代次”应确认原generation而非手改alias。

离线全包测试、真实模块和tests/deployment/full-workflow.test.mjs整站验收分别记录命令/退出码/skip。搜索503查原RPC/Doc事实/固定正文预算，游标409查查询与actor/绝对期限，重建COMMIT_UNKNOWN保留原号重试，不重扫当前数据冒充原S。

## 7. 完成本课全部逐文件步骤

下表是本课文件的唯一归属清单。每页的全部操作段都要完成；小接口先按第2节写，再补仓储、业务与适配。同包前向引用在后课补齐，所以完成本表表示本用例代码已写全，不等于服务已启动。

| 次序 | 学习工程中的文件 | 完整分段操作 |
| --- | --- | --- |
| 1 | services/document/internal/domain/document/search_models.go | [逐声明/函数带写](../../write/102-search-models.md) |
| 2 | services/document/internal/repository/document/search_queries.go | [逐声明/函数带写](../../write/175-search-queries.md) |
| 3 | services/document/internal/repository/snapshot/entries.go | [逐声明/函数带写](../../write/188-entries.md) |
| 4 | services/document/internal/repository/snapshot/job.go | [逐声明/函数带写](../../write/189-job.md) |
| 5 | services/document/internal/logic/document/search_service.go | [逐声明/函数带写](../../write/235-search-service.md) |
| 6 | services/document/internal/logic/document/search_snippet.go | [逐声明/函数带写](../../write/236-search-snippet.md) |
| 7 | services/document/internal/logic/snapshot/capture.go | [逐声明/函数带写](../../write/248-capture.md) |
| 8 | services/document/internal/logic/snapshot/snapshot_service.go | [逐声明/函数带写](../../write/249-snapshot-service.md) |
| 9 | services/document/internal/client/search.go | [逐声明/函数带写](../../write/117-search.md) |
| 10 | services/document/internal/handler/document/search.go | [逐声明/函数带写](../../write/262-search.md) |
| 11 | services/document/internal/handler/routes.go | [逐声明/函数带写](../../write/268-routes.md) |
| 12 | services/document/internal/worker/content_workflows_workers.go | [逐声明/函数带写](../../write/271-content-workflows-workers.md) |
| 13 | services/document/internal/worker/identity_notifications_workers.go | [逐声明/函数带写](../../write/272-identity-notifications-workers.md) |
| 14 | services/document/internal/worker/lifecycle_workers.go | [逐声明/函数带写](../../write/273-lifecycle-workers.md) |
| 15 | services/document/internal/worker/publication_outputs_workers.go | [逐声明/函数带写](../../write/275-publication-outputs-workers.md) |
| 16 | services/document/internal/svc/content_workflows_wiring.go | [逐声明/函数带写](../../write/276-content-workflows-wiring.md) |
| 17 | services/document/internal/svc/lifecycle_wiring.go | [逐声明/函数带写](../../write/277-lifecycle-wiring.md) |
| 18 | services/document/internal/svc/resources_outputs_wiring.go | [逐声明/函数带写](../../write/278-resources-outputs-wiring.md) |
| 19 | services/document/internal/svc/workspace_wiring.go | [逐声明/函数带写](../../write/279-workspace-wiring.md) |
| 20 | services/document/internal/bootstrap/document.go | [逐声明/函数带写](../../write/280-document.md) |
| 21 | services/document/cmd/search-rebuild/main.go | [逐声明/函数带写](../../write/282-main.md) |
| 22 | services/document/cmd/search-snapshot/main.go | [逐声明/函数带写](../../write/283-main.md) |
| 23 | services/document/main.go | [逐声明/函数带写](../../write/284-main.md) |

## 8. 章末核对

先检查所有操作段已输入、生成契约仍从01支持包取得、没有假成功实现。只在最后需要比较整个文件时打开[03完整源码答案索引](../../03-files.md)。本课未在新空目录实际连续复原或运行真实依赖；上述预期是验收要求，不是本轮执行结果。完成后继续[04协作服务](../../04-collaboration.md)；真实运行统一在[06](../../06-integration.md)自己的独立实例执行。

## 可选快捷准备：一次创建本课空文件

第一次学习可跳过本节，按正文逐份创建即可。已经理解本课分工、希望减少手动建目录操作时，才使用下面脚本；它只创建缺失目录和空文件，不写业务代码，也不覆盖已有文件。

脚本先用Set-Location进入00阶段设置的学习工程；路径数组列出本课文件；foreach依次处理，Split-Path取父目录，New-Item创建目录或空文件，Test-Path让已存在文件跳过，Out-Null仅隐藏命令输出。批量创建后仍从正文第1节学习，不代表完成任何业务功能。

~~~powershell
Set-Location -LiteralPath $lessonRoot
$env:GOWORK = 'off'
$documentLessonFiles = @(
    'services/document/internal/domain/document/search_models.go',
    'services/document/internal/repository/document/search_queries.go',
    'services/document/internal/repository/snapshot/entries.go',
    'services/document/internal/repository/snapshot/job.go',
    'services/document/internal/logic/document/search_service.go',
    'services/document/internal/logic/document/search_snippet.go',
    'services/document/internal/logic/snapshot/capture.go',
    'services/document/internal/logic/snapshot/snapshot_service.go',
    'services/document/internal/client/search.go',
    'services/document/internal/handler/document/search.go',
    'services/document/internal/handler/routes.go',
    'services/document/internal/worker/content_workflows_workers.go',
    'services/document/internal/worker/identity_notifications_workers.go',
    'services/document/internal/worker/lifecycle_workers.go',
    'services/document/internal/worker/publication_outputs_workers.go',
    'services/document/internal/svc/content_workflows_wiring.go',
    'services/document/internal/svc/lifecycle_wiring.go',
    'services/document/internal/svc/resources_outputs_wiring.go',
    'services/document/internal/svc/workspace_wiring.go',
    'services/document/internal/bootstrap/document.go',
    'services/document/cmd/search-rebuild/main.go',
    'services/document/cmd/search-snapshot/main.go',
    'services/document/main.go'
)
foreach ($documentLessonFile in $documentLessonFiles) {
    $documentLessonPath = Join-Path $lessonRoot $documentLessonFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $documentLessonPath) | Out-Null
    if (-not (Test-Path -LiteralPath $documentLessonPath)) {
        New-Item -ItemType File -Path $documentLessonPath | Out-Null
    }
}
~~~
