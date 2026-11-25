# services/account/internal/handler/routes.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/079-routes.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`RegisterAuthentication`、`RegisterIdentityRenewal`、`RegisterNotificationRecipients`、`RegisterNotificationEvents`、`RegisterAccountCloseDecisions`、`RegisterLogout`、`RegisterAccountClosures`、`RegisterChallenges`、`RegisterCredentials`、`RegisterOrganizationReads`、`RegisterOrganizationTransitions`、`RegisterOrganizationClosures`、`RegisterOrganizationActions`、`RegisterMembership`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package handler

import (
	account "collaboration/account/internal/handler/account"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterAuthentication(server *rest.Server, authentication *account.Authentication) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/login", Handler: authentication.Login},
		{Method: "GET", Path: "/v1/account/me", Handler: authentication.Me},
		{Method: "POST", Path: "/infra/identity/verify-session", Handler: authentication.VerifySession},
	})
}

func RegisterIdentityRenewal(server *rest.Server, renewal *account.IdentityRenewal) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/identity/renew-operation", Handler: renewal.Renew})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/identity/document-collaborator", Handler: renewal.DocumentCollaborator})
}

func RegisterNotificationRecipients(server *rest.Server, recipients *account.NotificationRecipients) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/recipients", Handler: recipients.Check})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/administrators", Handler: recipients.Administrators})
}

func RegisterNotificationEvents(server *rest.Server, events *account.NotificationEvents) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/events", Handler: events.Events})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/events/ack", Handler: events.Acknowledge})
}

func RegisterAccountCloseDecisions(server *rest.Server, decisions *account.AccountCloseDecisions) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-binding", Handler: decisions.Binding})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-decision", Handler: decisions.Read})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-confirmation", Handler: decisions.Confirm})
}

func RegisterLogout(server *rest.Server, logout *account.Logout) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/logout", Handler: logout.Logout},
		{Method: "GET", Path: "/v1/account/operations/:operationId", Handler: logout.Progress},
	})
}

func RegisterAccountClosures(server *rest.Server, closures *account.AccountClosures) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/closures", Handler: closures.Start},
		{Method: "GET", Path: "/v1/account/closures/:operationId", Handler: closures.Progress},
	})
}

func RegisterChallenges(server *rest.Server, challenges *account.Challenges) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/challenges", Handler: challenges.Issue},
		{Method: "POST", Path: "/v1/account/signup", Handler: challenges.Signup},
	})
}

func RegisterCredentials(server *rest.Server, credentials *account.Credentials) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/password-resets", Handler: credentials.ResetPassword},
		{Method: "POST", Path: "/v1/account/password", Handler: credentials.ChangePassword},
		{Method: "POST", Path: "/v1/account/email", Handler: credentials.ChangeEmail},
	})
}

func RegisterOrganizationReads(server *rest.Server, organizations *account.Organizations) {
	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/v1/organizations", Handler: organizations.List},
		{Method: "GET", Path: "/v1/organizations/:organizationId", Handler: organizations.Get},
		{Method: "GET", Path: "/v1/organizations/:organizationId/members", Handler: organizations.Members},
		{Method: "GET", Path: "/infra/lifecycle/member-departure-status", Handler: organizations.DepartureStatus},
	})
}

func RegisterOrganizationTransitions(server *rest.Server, transitions *account.OrganizationTransitions) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations/:organizationId/members/:userId/role", Handler: transitions.Role},
		{Method: "DELETE", Path: "/v1/organizations/:organizationId/members/:userId", Handler: transitions.Remove},
		{Method: "POST", Path: "/v1/organizations/:organizationId/owner-transfers", Handler: transitions.TransferOwner},
		{Method: "POST", Path: "/v1/organizations/:organizationId/leave", Handler: transitions.Leave},
	})
}

func RegisterOrganizationClosures(server *rest.Server, closures *account.OrganizationClosures) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations/:organizationId/closures", Handler: closures.Close},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-confirmation", Handler: closures.Confirmation},
	})
}

func RegisterOrganizationActions(server *rest.Server, actions *account.OrganizationActions) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations", Handler: actions.Create},
		{Method: "PATCH", Path: "/v1/organizations/:organizationId", Handler: actions.Edit},
		{Method: "POST", Path: "/v1/organizations/:organizationId/creation-grants", Handler: actions.ChangeCreationGrant},
	})
}

func RegisterMembership(server *rest.Server, invitations *account.Invitations, applications *account.Applications) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations/:organizationId/invitations", Handler: invitations.Create},
		{Method: "GET", Path: "/v1/organizations/:organizationId/invitations", Handler: invitations.List},
		{Method: "POST", Path: "/v1/invitations/:invitationId/cancel", Handler: invitations.Cancel},
		{Method: "POST", Path: "/v1/invitations/accept", Handler: applications.AcceptInvitation},
		{Method: "POST", Path: "/v1/organizations/:organizationId/applications", Handler: applications.Apply},
		{Method: "GET", Path: "/v1/organizations/:organizationId/applications", Handler: applications.List},
		{Method: "GET", Path: "/v1/account/applications", Handler: applications.List},
		{Method: "POST", Path: "/v1/applications/:applicationId/decisions", Handler: applications.Decide},
	})
}
````

UTF-8/LF全文SHA-256：`c5704230a9b0ad5afde857d45df857f8bafe9a609809d145be1c860d2c92e109`。[参考实现](../../../../services/account/internal/handler/routes.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
