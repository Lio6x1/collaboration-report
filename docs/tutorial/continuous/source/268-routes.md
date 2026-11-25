# services/document/internal/handler/routes.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/268-routes.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Routes`、`RegisterDocument`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package handler

import (
	document "collaboration/document/internal/handler/document"
	"collaboration/document/internal/handler/httpx"
	"github.com/zeromicro/go-zero/rest"
)

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
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/conflict-contexts/:conflictRef/resolutions", Handler: routes.ConflictResolutions.Create},
	}, rest.WithMaxBytes(httpx.ContentBodyLimit))
}
````

UTF-8/LF全文SHA-256：`ce70d2ec74d7ecffaa0946072bee0700a7e8377b853440056b9f47a20a8f34b7`。[参考实现](../../../../services/document/internal/handler/routes.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
