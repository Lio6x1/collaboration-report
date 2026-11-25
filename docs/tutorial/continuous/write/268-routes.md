# 分步编写 `services/document/internal/handler/routes.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/268-routes.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

登记Document所有已装配的HTTP功能入口。

路由是传输映射表，依赖由bootstrap构造，不能在路由注册时隐藏业务状态。

写之前先按这条执行路线理解各段的连接：

1. Routes携带各handler
2. RegisterDocument逐项绑定路径与方法，并为大正文入口使用约定体积限制。

本文件需要守住的边界：

- 遗漏注册会使已实现功能不可达
- 内部控制路由仍须保留原服务身份保护。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/routes.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/routes.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/routes.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/routes.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:176 -->
````go
package handler

import (
	document "collaboration/document/internal/handler/document"
	"collaboration/document/internal/handler/httpx"
	"github.com/zeromicro/go-zero/rest"
)

````

## 第 2 步：Routes

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

Routes只登记传输入口，每个功能由已注入依赖的handler承担，不保存业务状态或SQL池。

本步声明：`type Routes struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DraftBodyCapacity</code> | <code>*document.DraftBodyCapacity</code> | 使用 <code>document.DraftBodyCapacity</code> 类型；其定义处的职责：提供上传预约、字节流、完成确认、下载与正文容量保护入口。 |
| <code>Outputs</code> | <code>*document.Outputs</code> | 使用 <code>document.Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |
| <code>Departures</code> | <code>*document.Departures</code> | 使用 <code>document.Departures</code> 类型；其定义处的职责：适配成员离组、组织关闭、个人转移和留存治理HTTP协议。 |
| <code>AccountRetirements</code> | <code>*document.AccountRetirements</code> | 使用 <code>document.AccountRetirements</code> 类型；其定义处的职责：提供Account控制通道的准备、计划、关系、分步清理与完成入口。 |
| <code>OrganizationClosures</code> | <code>*document.OrganizationClosures</code> | 使用 <code>document.OrganizationClosures</code> 类型；其定义处的职责：适配成员离组、组织关闭、个人转移和留存治理HTTP协议。 |
| <code>Sharing</code> | <code>*document.Sharing</code> | 使用 <code>document.Sharing</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>Authorizations</code> | <code>*document.Authorizations</code> | 使用 <code>document.Authorizations</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>Management</code> | <code>*document.Management</code> | 使用 <code>document.Management</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>Trash</code> | <code>*document.Trash</code> | 使用 <code>document.Trash</code> 类型；其定义处的职责：提供回收列表、恢复、确认永久删除和清理上下文注入。 |
| <code>Exports</code> | <code>*document.Exports</code> | 使用 <code>document.Exports</code> 类型；其定义处的职责：暴露导出创建、进度和附件下载HTTP接口。 |
| <code>Purges</code> | <code>*document.Purges</code> | 使用 <code>document.Purges</code> 类型；其定义处的职责：提供回收列表、恢复、确认永久删除和清理上下文注入。 |
| <code>RetainedGovernance</code> | <code>*document.RetainedGovernance</code> | 使用 <code>document.RetainedGovernance</code> 类型；其定义处的职责：适配成员离组、组织关闭、个人转移和留存治理HTTP协议。 |
| <code>PersonalTransfers</code> | <code>*document.PersonalTransfers</code> | 使用 <code>document.PersonalTransfers</code> 类型；其定义处的职责：适配成员离组、组织关闭、个人转移和留存治理HTTP协议。 |
| <code>KnowledgeBases</code> | <code>*document.KnowledgeBases</code> | 使用 <code>document.KnowledgeBases</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>Directory</code> | <code>*document.Directory</code> | 使用 <code>document.Directory</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>Visibilities</code> | <code>*document.Visibilities</code> | 使用 <code>document.Visibilities</code> 类型；其定义处的职责：提供当前正式文档、发布历史、创建/切换/撤销发布及可见性修改入口。 |
| <code>Documents</code> | <code>*document.Documents</code> | 使用 <code>document.Documents</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>Readings</code> | <code>*document.Readings</code> | 使用 <code>document.Readings</code> 类型；其定义处的职责：提供详情、主线、修订摘要和单元正文的HTTP读取入口。 |
| <code>ContentVersions</code> | <code>*document.ContentVersions</code> | 使用 <code>document.ContentVersions</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>Resources</code> | <code>*document.Resources</code> | 使用 <code>document.Resources</code> 类型；其定义处的职责：提供上传预约、字节流、完成确认、下载与正文容量保护入口。 |
| <code>Comments</code> | <code>*document.Comments</code> | 使用 <code>document.Comments</code> 类型；其定义处的职责：提供批注与通知HTTP操作，先由持久目标定位真实文档后认证。 |
| <code>Notifications</code> | <code>*document.Notifications</code> | 使用 <code>document.Notifications</code> 类型；其定义处的职责：提供批注与通知HTTP操作，先由持久目标定位真实文档后认证。 |
| <code>Spaces</code> | <code>*document.Spaces</code> | 使用 <code>document.Spaces</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>EditableUnits</code> | <code>*document.EditableUnits</code> | 使用 <code>document.EditableUnits</code> 类型；其定义处的职责：连接目录、创建、改名、角色、负责人及章节管理的HTTP接口。 |
| <code>EditSessions</code> | <code>*document.EditSessions</code> | 使用 <code>document.EditSessions</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |
| <code>Submissions</code> | <code>*document.Submissions</code> | 使用 <code>document.Submissions</code> 类型；其定义处的职责：提供送审、请求详情、轮次历史、撤回和修改意见动作的HTTP入口。 |
| <code>Reviews</code> | <code>*document.Reviews</code> | 使用 <code>document.Reviews</code> 类型；其定义处的职责：提供送审、请求详情、轮次历史、撤回和修改意见动作的HTTP入口。 |
| <code>Merges</code> | <code>*document.Merges</code> | 使用 <code>document.Merges</code> 类型；其定义处的职责：连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。 |
| <code>MainControls</code> | <code>*document.MainControls</code> | 使用 <code>document.MainControls</code> 类型；其定义处的职责：暴露主线锁、样式、结构预览/确认和私有MAIN来源接口。 |
| <code>Publications</code> | <code>*document.Publications</code> | 使用 <code>document.Publications</code> 类型；其定义处的职责：提供当前正式文档、发布历史、创建/切换/撤销发布及可见性修改入口。 |
| <code>PublicReading</code> | <code>*document.PublicReading</code> | 使用 <code>document.PublicReading</code> 类型；其定义处的职责：提供当前正式文档、发布历史、创建/切换/撤销发布及可见性修改入口。 |
| <code>SearchPages</code> | <code>*document.SearchPages</code> | 使用 <code>document.SearchPages</code> 类型；其定义处的职责：将HTTP搜索参数和可选真实身份送入整页搜索用例。 |
| <code>MainSources</code> | <code>*document.MainSources</code> | 使用 <code>document.MainSources</code> 类型；其定义处的职责：暴露主线锁、样式、结构预览/确认和私有MAIN来源接口。 |
| <code>PullMain</code> | <code>*document.PullMain</code> | 使用 <code>document.PullMain</code> 类型；其定义处的职责：连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。 |
| <code>DraftControls</code> | <code>*document.DraftControls</code> | 使用 <code>document.DraftControls</code> 类型；其定义处的职责：连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。 |
| <code>ReverseChanges</code> | <code>*document.ReverseChanges</code> | 使用 <code>document.ReverseChanges</code> 类型；其定义处的职责：连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。 |
| <code>ConflictResolutions</code> | <code>*document.ConflictResolutions</code> | 使用 <code>document.ConflictResolutions</code> 类型；其定义处的职责：连接合并批准、冲突展示/修复、拉取、丢弃和反向变更的HTTP接口。 |
| <code>DraftSources</code> | <code>*document.DraftSources</code> | 使用 <code>document.DraftSources</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>StructurePreviews</code> | <code>*document.StructurePreviews</code> | 使用 <code>document.StructurePreviews</code> 类型；其定义处的职责：暴露主线锁、样式、结构预览/确认和私有MAIN来源接口。 |

<!-- write-step: 2 bytes=176:2053 -->
````go
// Routes只登记传输入口，每个功能由已注入依赖的handler承担，不保存业务状态或SQL池。
type Routes struct {
	DraftBodyCapacity    *document.DraftBodyCapacity
	Outputs              *document.Outputs
	Departures           *document.Departures
	AccountRetirements   *document.AccountRetirements
	OrganizationClosures *document.OrganizationClosures
	Sharing              *document.Sharing
	Authorizations       *document.Authorizations
	Management           *document.Management
	Trash                *document.Trash
	Exports              *document.Exports
	Purges               *document.Purges
	RetainedGovernance   *document.RetainedGovernance
	PersonalTransfers    *document.PersonalTransfers
	KnowledgeBases       *document.KnowledgeBases
	Directory            *document.Directory
	Visibilities         *document.Visibilities
	Documents            *document.Documents
	Readings             *document.Readings
	ContentVersions      *document.ContentVersions
	Resources            *document.Resources
	Comments             *document.Comments
	Notifications        *document.Notifications
	Spaces               *document.Spaces
	EditableUnits        *document.EditableUnits
	EditSessions         *document.EditSessions
	Submissions          *document.Submissions
	Reviews              *document.Reviews
	Merges               *document.Merges
	MainControls         *document.MainControls
	Publications         *document.Publications
	PublicReading        *document.PublicReading
	SearchPages          *document.SearchPages
	MainSources          *document.MainSources
	PullMain             *document.PullMain
	DraftControls        *document.DraftControls
	ReverseChanges       *document.ReverseChanges
	ConflictResolutions  *document.ConflictResolutions
	DraftSources         *document.DraftSources
	StructurePreviews    *document.StructurePreviews
}

````

## 第 3 步：RegisterDocument

插入位置：第 2 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func RegisterDocument(server *rest.Server, routes Routes)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>routes</code> | <code>Routes</code> | 使用 <code>Routes</code> 类型；其定义处的职责：Routes只登记传输入口，每个功能由已注入依赖的handler承担，不保存业务状态或SQL池。<br>在此声明中的实际使用：<code>routes.Resources</code>；<code>routes.PersonalTransfers</code>；<code>routes.RetainedGovernance</code>；<code>routes.Exports</code><br>另有 35 处使用，继续按下方完整代码的顺序处理。 |

<!-- write-step: 3 bytes=2053:15873 -->
````go
func RegisterDocument(server *rest.Server, routes Routes) {
	// 二进制上传有独立32MiB请求预算，其他JSON入口保留原框架限制。
	server.AddRoutes([]rest.Route{{Method: "PUT", Path: "/v1/resources/:resourceId/upload", Handler: routes.Resources.UploadBytes}}, rest.WithMaxBytes(32<<20))
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/documents/:documentId/personal-transfers", Handler: routes.PersonalTransfers.Offer},
		{Method: "GET", Path: "/v1/personal-transfers", Handler: routes.PersonalTransfers.List},
		{Method: "GET", Path: "/v1/personal-transfers/:transferId", Handler: routes.PersonalTransfers.Detail},
		{Method: "POST", Path: "/v1/personal-transfers/:transferId/accept", Handler: routes.PersonalTransfers.Accept},
		{Method: "POST", Path: "/v1/personal-transfers/:transferId/reject", Handler: routes.PersonalTransfers.Reject},
		{Method: "POST", Path: "/v1/personal-transfers/:transferId/cancel", Handler: routes.PersonalTransfers.Cancel},
		{Method: "POST", Path: "/v1/retained-public/documents/:documentId/unpublish", Handler: routes.RetainedGovernance.Unpublish},
		{Method: "GET", Path: "/v1/retained-public/documents/:documentId", Handler: routes.RetainedGovernance.Metadata},
		{Method: "DELETE", Path: "/v1/retained-public/documents/:documentId", Handler: routes.RetainedGovernance.Delete},
		{Method: "POST", Path: "/v1/documents/:documentId/exports", Handler: routes.Exports.Create},
		{Method: "GET", Path: "/v1/export-jobs/:exportJobId", Handler: routes.Exports.Job},
		{Method: "GET", Path: "/v1/export-jobs/:exportJobId/download", Handler: routes.Exports.Download},
		{Method: "GET", Path: "/v1/spaces/:spaceId/trash", Handler: routes.Trash.List},
		{Method: "POST", Path: "/v1/documents/:documentId/restore", Handler: routes.Trash.Restore},
		{Method: "POST", Path: "/v1/documents/:documentId/purge", Handler: routes.Purges.Purge},
		{Method: "GET", Path: "/v1/notifications", Handler: routes.Notifications.List},
		{Method: "GET", Path: "/v1/notifications/:notificationId/review-round", Handler: routes.Notifications.ReviewRound},
		{Method: "POST", Path: "/v1/notifications/:notificationId/read", Handler: routes.Notifications.MarkRead},
		{Method: "POST", Path: "/v1/documents/:documentId/comment-threads", Handler: routes.Comments.CreateThread},
		{Method: "GET", Path: "/v1/documents/:documentId/comment-threads", Handler: routes.Comments.ListThreads},
		{Method: "POST", Path: "/v1/comment-threads/:threadId/comments", Handler: routes.Comments.Reply},
		{Method: "GET", Path: "/v1/comment-threads/:threadId/comments", Handler: routes.Comments.ListComments},
		{Method: "GET", Path: "/v1/comment-threads/:threadId", Handler: routes.Comments.ReadThread},
		{Method: "POST", Path: "/v1/comment-threads/:threadId/state", Handler: routes.Comments.State},
		{Method: "PATCH", Path: "/v1/comments/:commentId", Handler: routes.Comments.Edit},
		{Method: "DELETE", Path: "/v1/comments/:commentId", Handler: routes.Comments.Delete},
		{Method: "POST", Path: "/v1/documents/:documentId/resources", Handler: routes.Resources.BeginUpload},
		{Method: "POST", Path: "/v1/resources/:resourceId/complete", Handler: routes.Resources.Complete},
		{Method: "GET", Path: "/v1/spaces/:spaceId/quota", Handler: routes.Resources.Quota},
		{Method: "GET", Path: "/v1/resources/:resourceId/content", Handler: routes.Resources.Content},
		{Method: "POST", Path: "/infra/output/settled", Handler: routes.Outputs.Settled},
		{Method: "POST", Path: "/infra/draft/protect-body", Handler: routes.DraftBodyCapacity.Protect},
		{Method: "POST", Path: "/infra/output/register-account", Handler: routes.Outputs.RegisterAccount},
		{Method: "POST", Path: "/infra/output/register-wss", Handler: routes.Outputs.RegisterCollaboration},
		{Method: "POST", Path: "/infra/collaboration/bootstrap", Handler: routes.EditSessions.Bootstrap},
		{Method: "POST", Path: "/infra/main/context", Handler: routes.MainSources.MainSource},
		{Method: "POST", Path: "/infra/main/committed", Handler: routes.MainSources.CommittedSource},
		{Method: "POST", Path: "/infra/draft/context", Handler: routes.DraftSources.Source},
		{Method: "POST", Path: "/infra/draft/committed", Handler: routes.DraftSources.Committed},
		{Method: "POST", Path: "/infra/structure/context", Handler: routes.StructurePreviews.Source},
		{Method: "POST", Path: "/infra/structure/confirmation", Handler: routes.MainSources.ConfirmationSource},
		{Method: "POST", Path: "/v1/documents/:documentId/structure-previews", Handler: routes.StructurePreviews.Create},
		{Method: "POST", Path: "/v1/documents/:documentId/units/:unitId/pull-main", Handler: routes.PullMain.Create},
		{Method: "POST", Path: "/v1/documents/:documentId/units/:unitId/discard-previews", Handler: routes.DraftControls.Preview},
		{Method: "POST", Path: "/v1/documents/:documentId/units/:unitId/discard-draft", Handler: routes.DraftControls.Discard},
		{Method: "POST", Path: "/v1/documents/:documentId/reverse-changes", Handler: routes.ReverseChanges.Create},
		{Method: "GET", Path: "/v1/conflict-contexts/:conflictRef", Handler: routes.Reviews.Conflict},
		{Method: "POST", Path: "/v1/documents/:documentId/main-lock", Handler: routes.MainControls.Lock},
		{Method: "POST", Path: "/v1/documents/:documentId/releases", Handler: routes.Publications.Create},
		{Method: "GET", Path: "/v1/documents/:documentId/releases", Handler: routes.PublicReading.History},
		{Method: "GET", Path: "/v1/documents/:documentId/releases/:releaseId", Handler: routes.PublicReading.Historical},
		{Method: "GET", Path: "/v1/public/documents/:documentId", Handler: routes.PublicReading.Current},
		{Method: "GET", Path: "/v1/search", Handler: routes.SearchPages.Page},
		{Method: "POST", Path: "/v1/documents/:documentId/publication", Handler: routes.Publications.Change},
		{Method: "POST", Path: "/v1/documents/:documentId/styles", Handler: routes.MainControls.Style},
		{Method: "POST", Path: "/v1/documents/:documentId/chapters", Handler: routes.MainControls.Chapter},
		{Method: "POST", Path: "/v1/documents/:documentId/structure-changes", Handler: routes.MainControls.Structure},
		{Method: "GET", Path: "/infra/lifecycle/member-departure-eligibility", Handler: routes.Departures.Eligibility},
		{Method: "POST", Path: "/infra/lifecycle/member-departure-close", Handler: routes.Departures.Close},
		{Method: "POST", Path: "/infra/lifecycle/account-close-prepare", Handler: routes.AccountRetirements.Prepare},
		{Method: "POST", Path: "/infra/lifecycle/account-close-cancel", Handler: routes.AccountRetirements.Cancel},
		{Method: "POST", Path: "/infra/lifecycle/account-close-plan", Handler: routes.AccountRetirements.Plan},
		{Method: "POST", Path: "/infra/lifecycle/account-close-relations", Handler: routes.AccountRetirements.Relations},
		{Method: "POST", Path: "/infra/lifecycle/account-close-complete", Handler: routes.AccountRetirements.Complete},
		{Method: "POST", Path: "/infra/lifecycle/account-close-progress", Handler: routes.AccountRetirements.Progress},
		{Method: "POST", Path: "/infra/lifecycle/document-close-context", Handler: routes.OrganizationClosures.DispatchCleanupContext(routes.AccountRetirements.DocumentCleanupContext)},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-plan", Handler: routes.OrganizationClosures.Plan},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-progress", Handler: routes.OrganizationClosures.Progress},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-complete", Handler: routes.OrganizationClosures.Complete},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-clean-payloads", Handler: routes.OrganizationClosures.CleanPayloads},
		{Method: "POST", Path: "/infra/lifecycle/document-release-draft-resources", Handler: routes.AccountRetirements.ReleaseDraftResources},
		{Method: "POST", Path: "/infra/lifecycle/document-clean-history", Handler: routes.AccountRetirements.CleanHistory},
		{Method: "POST", Path: "/infra/lifecycle/document-clean-payloads", Handler: routes.AccountRetirements.CleanPayloads},
		{Method: "POST", Path: "/v1/documents/:documentId/units/:unitId/edit-sessions", Handler: routes.EditSessions.Create},
		{Method: "GET", Path: "/v1/operations/:operationId", Handler: routes.Submissions.Progress},
		{Method: "POST", Path: "/v1/documents/:documentId/units/:unitId/submissions", Handler: routes.Submissions.Create},
		{Method: "GET", Path: "/v1/submission-operations/:operationId", Handler: routes.Submissions.Progress},
		{Method: "POST", Path: "/v1/review-requests/:requestId/actions", Handler: routes.Reviews.RequestChanges},
		{Method: "POST", Path: "/v1/review-requests/:requestId/approve-and-merge", Handler: routes.Merges.Approve},
		{Method: "GET", Path: "/v1/merge-operations/:operationId", Handler: routes.Merges.Progress},
		{Method: "POST", Path: "/v1/review-requests/:requestId/withdrawals", Handler: routes.Reviews.Withdraw},
		{Method: "POST", Path: "/v1/review-requests/:requestId/change-requests/:changeId/resolve", Handler: routes.Reviews.Resolve},
		{Method: "GET", Path: "/v1/documents/:documentId/review-requests", Handler: routes.Reviews.List},
		{Method: "GET", Path: "/v1/me/review-requests", Handler: routes.Reviews.List},
		{Method: "GET", Path: "/v1/review-requests/:requestId", Handler: routes.Reviews.Request},
		{Method: "GET", Path: "/v1/review-requests/:requestId/rounds", Handler: routes.Reviews.Rounds},
		{Method: "GET", Path: "/v1/review-requests/:requestId/rounds/:roundId", Handler: routes.Reviews.Round},
		{Method: "GET", Path: "/v1/review-requests/:requestId/change-requests", Handler: routes.Reviews.Changes},
		{Method: "GET", Path: "/v1/review-requests/:requestId/actions", Handler: routes.Reviews.Actions},
		{Method: "POST", Path: "/v1/documents/:documentId/visibility", Handler: routes.Visibilities.Change},
		{Method: "POST", Path: "/v1/knowledge-bases/:knowledgeBaseId/visibility", Handler: routes.Visibilities.Change},
		{Method: "DELETE", Path: "/v1/knowledge-bases/:knowledgeBaseId", Handler: routes.KnowledgeBases.Delete},
		{Method: "POST", Path: "/v1/documents/:documentId/invitations", Handler: routes.Sharing.CreateInvitation},
		{Method: "GET", Path: "/v1/documents/:documentId/invitations", Handler: routes.Sharing.Invitations},
		{Method: "POST", Path: "/v1/document-invitations/:invitationId/cancel", Handler: routes.Sharing.CancelInvitation},
		{Method: "POST", Path: "/v1/document-invitations/:invitationId/archive", Handler: routes.Sharing.ArchiveInvitation},
		{Method: "POST", Path: "/v1/document-applications/:applicationId/archive", Handler: routes.Sharing.ArchiveApplication},
		{Method: "POST", Path: "/v1/document-invitations/applications", Handler: routes.Sharing.Apply},
		{Method: "GET", Path: "/v1/documents/:documentId/applications", Handler: routes.Sharing.Applications},
		{Method: "GET", Path: "/v1/document-applications", Handler: routes.Sharing.Applications},
		{Method: "POST", Path: "/v1/document-applications/:applicationId/decisions", Handler: routes.Sharing.Decide},
		{Method: "PATCH", Path: "/v1/documents/:documentId", Handler: routes.Management.Rename},
		{Method: "DELETE", Path: "/v1/documents/:documentId", Handler: routes.Management.Delete},
		{Method: "POST", Path: "/v1/documents/:documentId/responsible-transfers", Handler: routes.Management.Transfer},
		{Method: "GET", Path: "/v1/spaces/:spaceId/knowledge-bases", Handler: routes.Directory.KnowledgeBases},
		{Method: "GET", Path: "/v1/knowledge-bases/:knowledgeBaseId", Handler: routes.Directory.KnowledgeBase},
		{Method: "GET", Path: "/v1/knowledge-bases/:knowledgeBaseId/members", Handler: routes.Directory.KnowledgeBaseMembers},
		{Method: "GET", Path: "/v1/documents/:documentId/collaborators", Handler: routes.Directory.DocumentCollaborators},
		{Method: "GET", Path: "/v1/documents/:documentId/mention-candidates", Handler: routes.Directory.DocumentMentionCandidates},
		{Method: "GET", Path: "/v1/spaces/:spaceId/documents", Handler: routes.Directory.Documents},
		{Method: "GET", Path: "/v1/knowledge-bases/:knowledgeBaseId/documents", Handler: routes.Directory.Documents},
		{Method: "GET", Path: "/v1/documents/:documentId/editable-units", Handler: routes.EditableUnits.List},
		{Method: "POST", Path: "/v1/knowledge-bases/:knowledgeBaseId/roles", Handler: routes.KnowledgeBases.Role},
		{Method: "POST", Path: "/v1/knowledge-bases/:knowledgeBaseId/creation-grants", Handler: routes.KnowledgeBases.CreationGrant},
		{Method: "PATCH", Path: "/v1/knowledge-bases/:knowledgeBaseId", Handler: routes.KnowledgeBases.Rename},
		{Method: "POST", Path: "/v1/documents/:documentId/authorization-changes", Handler: routes.Authorizations.Change},
		{Method: "GET", Path: "/v1/authorization-changes/:operationId", Handler: routes.Authorizations.Progress},
		{Method: "GET", Path: "/v1/me/spaces", Handler: routes.Spaces.List},
		{Method: "POST", Path: "/v1/spaces/:spaceId/knowledge-bases", Handler: routes.KnowledgeBases.Create},
		{Method: "POST", Path: "/v1/documents", Handler: routes.Documents.Create},
		{Method: "GET", Path: "/v1/documents/:documentId", Handler: routes.Readings.Document},
		{Method: "GET", Path: "/v1/content-versions/:contentVersionId", Handler: routes.ContentVersions.Read},
		{Method: "GET", Path: "/v1/documents/:documentId/main", Handler: routes.Readings.Main},
		{Method: "GET", Path: "/v1/documents/:documentId/main-revision", Handler: routes.Readings.Revision},
		{Method: "GET", Path: "/v1/documents/:documentId/manifests/:manifestId", Handler: routes.Readings.Manifest},
		{Method: "GET", Path: "/v1/documents/:documentId/read-views/:viewId/units/:unitId", Handler: routes.Readings.Unit},
	})
	// go-zero默认1MiB会在handler校验前拒绝合法修复正文。仅正文路由提升预算，
	// 其余控制入口继续使用框架默认值及16KiB严格解码。
````

## 第 4 步：RegisterDocument（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 185 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

<!-- write-step: 4 bytes=15873:16074 -->
````go
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/conflict-contexts/:conflictRef/resolutions", Handler: routes.ConflictResolutions.Create},
	}, rest.WithMaxBytes(httpx.ContentBodyLimit))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/routes.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
