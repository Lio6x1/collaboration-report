# 分步编写 `services/document/internal/bootstrap/document.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/280-document.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

集中构造Document依赖、绑定RPC与HTTP并启动和停止worker。

共享实例身份、输出恢复及依赖就绪必须在对外Serve前落实。

写之前先按这条执行路线理解各段的连接：

1. RegisterDocument返回装配函数
2. 建立Account/Search等peer
3. 构造用例和handler
4. 运行Ready检查
5. Bind RPC
6. RegisterDocument路由
7. 注册worker与关闭回调。

本文件需要守住的边界：

- 装配失败不能仍Serve
- 连接与worker需要随服务生命周期关闭，不能散落handler临时创建。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/bootstrap' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/bootstrap/document.go') -and (Get-Item -LiteralPath 'services/document/internal/bootstrap/document.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/bootstrap/document.go')) { New-Item -ItemType File -Path 'services/document/internal/bootstrap/document.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:481 -->
````go
package bootstrap

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	"collaboration/document/internal/handler"
	document "collaboration/document/internal/handler/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/document/internal/server"
	"collaboration/document/internal/svc"
	"collaboration/document/internal/worker"
	"collaboration/platform"
	"context"
	"errors"
	"github.com/zeromicro/go-zero/rest"
)

````

## 第 2 步：RegisterDocument

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

RegisterDocument集中组合依赖与生命周期；业务资格、事务、SQL和运输细节分别留在所属层。

本步声明：`func RegisterDocument(rpc *server.Document) func(*rest.Server, *platform.Store, platform.Config) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rpc</code> | <code>*server.Document</code> | 使用 <code>server.Document</code> 类型；其定义处的职责：Document聚合已有RPC适配器，未实施的方法仍沿生成契约返回UNIMPLEMENTED。<br>在此声明中的实际使用：<code>rpc.Bind</code>；<code>rpc.BindLifecycle</code>；<code>rpc.BindCandidateCleanup</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func(*rest.Server, *platform.Store, platform.Config) error</code> | 实际返回表达式：<code>func(httpServer *rest.Server, store *platform.Store, cfg platform.Config) error { 		outputs, err := svc.NewOutputs(store, cfg) 		if err != nil { 			return err 		} 		outputHTTP := d …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(httpServer *rest.Server, store *platform.Store, cfg platform.Config) error { 		outputs, err := svc.NewOutputs(store, cfg) 		if err != nil { 			return err 		} 		outputHT …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：登记 <code>func() { 			if !assembled { 				accountPeer.Close() 			} 		}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!assembled</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：登记 <code>func() { 			if !assembled { 				searchPeer.Close() 			} 		}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!assembled</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=481:2234 -->
````go
// RegisterDocument集中组合依赖与生命周期；业务资格、事务、SQL和运输细节分别留在所属层。
func RegisterDocument(rpc *server.Document) func(*rest.Server, *platform.Store, platform.Config) error {
	return func(httpServer *rest.Server, store *platform.Store, cfg platform.Config) error {
		outputs, err := svc.NewOutputs(store, cfg)
		if err != nil {
			return err
		}
		outputHTTP := document.NewOutputs(outputs)
		fences := svc.NewFences(store, cfg, outputs)
		authorizations := svc.NewAuthorizations(store, fences, outputs)
		store.ReadyExtra = outputs.Ready
		accountPeer, err := platform.DialPeer(cfg, "account")
		if err != nil {
			return err
		}
		// 组合失败关闭已创建的RPC连接；成功后由共用恢复worker的退出负责释放。
		assembled := false
		defer func() {
			if !assembled {
				accountPeer.Close()
			}
		}()
		searchPeer, err := platform.DialPeer(cfg, "search")
		if err != nil {
			return err
		}
		// Doc以自己的真实mTLS角色调用Search；连接装配失败关闭，成功随store退出释放。
		// 不把Search数据库/ES凭据带入Doc，也不在SQL事务里建立跨服务连接。
		defer func() {
			if !assembled {
				searchPeer.Close()
			}
		}()
		accountHTTP, err := platform.HTTPPeer(cfg, "account")
		if err != nil {
			return err
		}
		collaborationHTTP, err := platform.HTTPPeer(cfg, "collaboration")
		if err != nil {
			return err
		}
		departures := svc.NewDepartures(store, cfg, accountHTTP)
		accountClosure := client.NewAccountClosure(cfg.Peers["accountHttp"], accountHTTP)
		retirements := logic.NewAccountRetirements(store.DB, accountClosure)
		documentCleaner := client.NewAccountDocumentCleanup(cfg.Peers["collaborationHttp"], collaborationHTTP)
````

## 第 3 步：RegisterDocument（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 63 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!personalTransfers.Ready()</code>。条件成立时执行 <code>return errors.New(&quot;PERSONAL_TRANSFER_DEPENDENCY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2234:4129 -->
````go
		retentions, err := svc.NewAccountRetentions(store.DB, accountClosure, documentCleaner)
		if err != nil {
			return err
		}
		organizationClosures, err := svc.NewOrganizationClosures(store.DB, client.NewOrganizationCloseAuthority(cfg.Peers["accountHttp"], accountHTTP), documentCleaner)
		if err != nil {
			return err
		}
		identity := svc.NewIdentity(pb.NewIdentityInternalClient(accountPeer), cfg.Peers["accountHttp"], accountHTTP)
		access := svc.NewAccess(store.DB, identity)
		authentication := document.NewIdentity(identity)
		requests := document.NewTransactions(svc.NewTransactions(store, identity, outputs), outputHTTP)
		pagination := svc.NewPagination(cfg)
		publicReading, err := svc.NewPublicReading(store, identity, access, outputs, pagination)
		if err != nil {
			return err
		}
		searchPages, err := svc.NewSearchPages(store, cfg, identity, access, outputs, pb.NewSearchInternalClient(searchPeer))
		if err != nil {
			return err
		}
		directoryClient := svc.NewDirectory(cfg, accountHTTP)
		tokens := client.NewInvitationTokens(cfg.KeyFile)
		editSessions := svc.NewEditSessions(store, cfg, access, tokens)
		submissions, err := svc.NewSubmissions(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		reviews, err := svc.NewReviews(store, access, pagination, directoryClient)
		if err != nil {
			return err
		}
		contentVersions, err := svc.NewContentVersions(store, access)
		if err != nil {
			return err
		}
		resources, err := svc.NewResources(store, cfg, access)
		if err != nil {
			return err
		}
		if err = resources.InitializeQuotas(store.Context); err != nil {
			return err
		}
		personalTransfers := svc.NewPersonalTransfers(store, identity, access, pagination, fences, client.NewPersonalTransferStorage(cfg.Peers["collaborationHttp"], collaborationHTTP), resources.SpaceQuotaLimit(), outputs.Self().ID)
		if !personalTransfers.Ready() {
````

## 第 4 步：RegisterDocument（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 108 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;PERSONAL_TRANSFER_DEPENDENCY_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=4129:5460 -->
````go
			return errors.New("PERSONAL_TRANSFER_DEPENDENCY_UNAVAILABLE")
		}
		exports, err := svc.NewExports(store, cfg, identity, access, publicReading, outputs, resources)
		if err != nil {
			return err
		}
		merges, err := svc.NewMerges(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		controls, err := svc.NewMainControls(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		publications, err := svc.NewPublications(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		mainReadViews, err := svc.NewMainReadViews(store, cfg, identity)
		if err != nil {
			return err
		}
		readings, err := svc.NewReadings(access, mainReadViews)
		if err != nil {
			return err
		}
		pulls, err := svc.NewPullMain(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		structurePreviews, err := svc.NewStructurePreviews(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		draftControls, err := svc.NewDraftControls(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		reverseChanges, err := svc.NewReverseChanges(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		resolutions, err := svc.NewResolutions(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
````

## 第 5 步：RegisterDocument（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 154 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return resolutions.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=5460:7420 -->
````go
		store.ManagedTasks = []string{"SUBMISSION_RECOVER", "CAPTURE_FINALIZE", "REVIEW_NOTIFICATION", "REVIEW_ACTION_NOTIFICATION", "MERGE_RECOVER", "MAIN_RECONCILE", "MERGE_NOTIFICATION", "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_RECONCILE", "PUBLICATION_RECOVER", "PULL_MAIN_RECOVER", "PULL_MAIN_RECONCILE", "STRUCTURE_PREVIEW_RECOVER", "DISCARD_PREVIEW_RECOVER", "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_RECONCILE", "REVERSE_RECOVER", "REVERSE_RECONCILE", "RESOLUTION_RECOVER", "RESOLUTION_RECONCILE"}
		comments := logic.NewComments(store.DB, access, reviews, pagination)
		notificationRecipients := client.NewNotificationRecipients(cfg.Peers["accountHttp"], accountHTTP)
		notificationDelivery := logic.NewNotificationDelivery(store.DB, comments, notificationRecipients)
		store.ManagedTasks = append(store.ManagedTasks, "COMMENT_NOTIFICATION", "RESPONSIBLE_DEPARTURE_NOTIFICATION")
		store.ReadyExtra = func(ctx context.Context) error {
			if err := organizationClosures.Ready(ctx); err != nil {
				return err
			}
			if err := notificationDelivery.Ready(ctx); err != nil {
				return err
			}
			if err := outputs.Ready(ctx); err != nil {
				return err
			}
			if err := submissions.Ready(ctx); err != nil {
				return err
			}
			if err := reviews.Ready(ctx); err != nil {
				return err
			}
			if err := merges.Ready(ctx); err != nil {
				return err
			}
			if err := controls.Ready(ctx); err != nil {
				return err
			}
			if err := publications.Ready(ctx); err != nil {
				return err
			}
			if err := pulls.Ready(ctx); err != nil {
				return err
			}
			if err := structurePreviews.Ready(ctx); err != nil {
				return err
			}
			if err := draftControls.Ready(ctx); err != nil {
				return err
			}
			if err := reverseChanges.Ready(ctx); err != nil {
				return err
			}
			return resolutions.Ready(ctx)
		}
		sharing := document.NewSharing(authentication, requests, svc.NewSharing(store, cfg, identity, access, pagination, tokens, directoryClient))
````

## 第 6 步：RegisterDocument（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 199 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

<!-- write-step: 6 bytes=7420:13056 -->
````go
		previewHTTP := document.NewStructurePreviews(authentication, requests, structurePreviews, sharing.OperationProgress)
		reverseHTTP := document.NewReverseChanges(authentication, requests, reverseChanges, previewHTTP.Progress)
		resolutionHTTP := document.NewConflictResolutions(authentication, requests, reviews, resolutions, reverseHTTP.Progress)
		draftHTTP := document.NewDraftControls(authentication, requests, draftControls, resolutionHTTP.Progress)
		pullHTTP := document.NewPullMain(authentication, requests, pulls, draftHTTP.Progress)
		controlHTTP := document.NewMainControls(authentication, requests, controls, pullHTTP.Progress)
		publicationHTTP := document.NewPublications(authentication, requests, publications, controlHTTP.Progress)
		mergeHTTP := document.NewMerges(authentication, requests, reviews, merges, publicationHTTP.Progress)
		knowledgeBases := svc.NewKnowledgeBases(store, identity, access)
		trash := logic.NewTrash(store.DB, identity, access, pagination)
		purges := svc.NewPurges(store, trash, documentCleaner, outputs.Self().ID)
		retainedGovernance := svc.NewRetainedGovernance(store, identity, documentCleaner, resources, exports, outputs.Self().ID)
		retainedHTTP := document.NewRetainedGovernance(authentication, requests, retainedGovernance, mergeHTTP.Progress)
		retirementHTTP := document.NewAccountRetirements(retirements, retentions)
		retirementHTTP.SetPurges(purges)
		retirementHTTP.SetRetainedGovernance(retainedGovernance)
		store.ManagedTasks = append(store.ManagedTasks, "PURGE_RECOVER", "RETAINED_GOVERNANCE_RECOVER", "PERSONAL_TRANSFER_RECOVER")

		// 平台在这个回调成功后才开放HTTP/RPC；所有适配器复用唯一已装配用例。
		draftResources := svc.NewDraftResources(store, cfg, collaborationHTTP)
		rpc.Bind(server.NewFence(fences), server.NewEditSessions(editSessions), server.NewDraftResources(draftResources), server.NewOperationResults(submissions, merges, controls, pulls, structurePreviews, draftControls, reverseChanges, resolutions, publications), server.NewPublicSnapshot(publicReading))
		rpc.BindLifecycle(logic.NewLifecyclePlans(store.DB, accountClosure, retirements, retentions))
		rpc.BindCandidateCleanup(logic.NewCandidateCleanup(store.DB))
		handler.RegisterDocument(httpServer, handler.Routes{
			DraftBodyCapacity:    document.NewDraftBodyCapacity(draftResources),
			Outputs:              outputHTTP,
			Departures:           document.NewDepartures(departures),
			AccountRetirements:   retirementHTTP,
			OrganizationClosures: document.NewOrganizationClosures(organizationClosures),
			Sharing:              sharing,
			Authorizations:       document.NewAuthorizations(authentication, requests, svc.NewAuthorizationChanges(store, identity, access), authorizations),
			Management:           document.NewManagement(authentication, requests, svc.NewManagement(store, identity, access)),
			Trash:                document.NewTrash(authentication, requests, trash),
			Purges:               document.NewPurges(authentication, requests, purges),
			RetainedGovernance:   retainedHTTP,
			PersonalTransfers:    document.NewPersonalTransfers(authentication, requests, personalTransfers),
			Exports:              document.NewExports(authentication, outputHTTP, exports, cfg.Origin),
			KnowledgeBases:       document.NewKnowledgeBases(authentication, requests, knowledgeBases),
			Directory:            document.NewDirectory(authentication, requests, logic.NewDirectory(store.DB, identity, access, knowledgeBases, pagination, directoryClient)),
			Visibilities:         document.NewVisibilities(authentication, requests, svc.NewVisibilities(knowledgeBases, access), authorizations),
			Documents:            document.NewDocuments(authentication, requests, svc.NewDocuments(store, identity, access, resources.SpaceQuotaLimit())),
			Readings:             document.NewReadings(authentication, requests, readings),
			ContentVersions:      document.NewContentVersions(authentication, requests, contentVersions),
			Resources:            document.NewResources(authentication, requests, resources, logic.NewResourceReading(resources, publicReading, reviews, outputs)),
			Comments:             document.NewComments(authentication, requests, comments),
			Notifications:        document.NewNotifications(authentication, requests, logic.NewNotifications(store.DB, access, comments, pagination, notificationRecipients)),
			Spaces:               document.NewSpaces(authentication, requests, svc.NewSpaces(identity, directoryClient, pagination)),
			EditableUnits:        document.NewEditableUnits(authentication, requests, logic.NewEditableUnits(access, pagination)),
			EditSessions:         document.NewEditSessions(authentication, requests, editSessions),
			Submissions:          document.NewSubmissions(authentication, requests, submissions, retainedHTTP.Progress),
			Reviews:              document.NewReviews(authentication, requests, reviews),
			Merges:               mergeHTTP,
			MainControls:         controlHTTP,
			Publications:         publicationHTTP,
			PublicReading:        document.NewPublicReading(authentication, requests, outputHTTP, publicReading),
			SearchPages:          document.NewSearchPages(authentication, outputHTTP, searchPages),
			MainSources:          document.NewMainSources(merges, controls, mainReadViews, publications),
			PullMain:             pullHTTP,
			DraftControls:        draftHTTP,
			ReverseChanges:       reverseHTTP,
			ConflictResolutions:  resolutionHTTP,
			DraftSources:         document.NewDraftSources(pulls, draftControls, reverseChanges, resolutions),
			StructurePreviews:    previewHTTP,
		})
````

## 第 7 步：RegisterDocument（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 263 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunSubmissions(store.Context, submissions)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunMerges(store.Context, merges)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunMainControls(store.Context, controls)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunPublications(store.Context, publications)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunResources(store.Context, resources)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunExports(store.Context, exports)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunPurges(store.Context, purges)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunRetainedGovernance(store.Context, retainedGovernance)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunPersonalTransfers(store.Context, personalTransfers)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunNotifications(store.Context, notificationDelivery)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunAccountNotifications(store.Context, logic.NewAccountNotificationImport(store.DB, notificationRecipients))</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunPullMain(store.Context, pulls)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunStructurePreviews(store.Context, structurePreviews)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunDraftControls(store.Context, draftControls)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunReverseChanges(store.Context, reverseChanges)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>worker.RunResolutions(store.Context, resolutions)</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：用独立 goroutine 执行 <code>func() { 			defer accountPeer.Close() 			defer searchPeer.Close() 			worker.RunAuthorizations(store.Context, authorizations, departures.Recover) 		}()</code>；调用点不会等待其完成，需要下方代码中的生命周期控制。
- 在本段的匿名函数内：登记 <code>accountPeer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：登记 <code>searchPeer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=13056:14213 -->
````go
		assembled = true
		go worker.RunSubmissions(store.Context, submissions)
		go worker.RunMerges(store.Context, merges)
		go worker.RunMainControls(store.Context, controls)
		go worker.RunPublications(store.Context, publications)
		go worker.RunResources(store.Context, resources)
		go worker.RunExports(store.Context, exports)
		go worker.RunPurges(store.Context, purges)
		go worker.RunRetainedGovernance(store.Context, retainedGovernance)
		go worker.RunPersonalTransfers(store.Context, personalTransfers)
		go worker.RunNotifications(store.Context, notificationDelivery)
		go worker.RunAccountNotifications(store.Context, logic.NewAccountNotificationImport(store.DB, notificationRecipients))
		go worker.RunPullMain(store.Context, pulls)
		go worker.RunStructurePreviews(store.Context, structurePreviews)
		go worker.RunDraftControls(store.Context, draftControls)
		go worker.RunReverseChanges(store.Context, reverseChanges)
		go worker.RunResolutions(store.Context, resolutions)
		go func() {
			defer accountPeer.Close()
			defer searchPeer.Close()
			worker.RunAuthorizations(store.Context, authorizations, departures.Recover)
		}()
		return nil
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/bootstrap/document.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
