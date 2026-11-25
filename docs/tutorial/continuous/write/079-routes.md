# 分步编写 `services/account/internal/handler/routes.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/079-routes.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

集中注册Account公开业务与服务间控制路由。

明确方法和路径使前端、DTO契约与handler一一对应，也防止内部控制功能意外变成公共任免入口。

写之前先按这条执行路线理解各段的连接：

1. RegisterAuthentication映射login/me；RegisterCredentials映射reset/password/email操作。
2. 组织读写、申请邀请、退出和注销路由分别绑定具体handler；内部续期和通知路由依赖handler的document证书检查。

本文件需要守住的边界：

- 注册路由本身不等于完成鉴权，不能删掉内部handler的HTTPCaller检查。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/routes.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/routes.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/routes.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/routes.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:125 -->
````go
package handler

import (
	account "collaboration/account/internal/handler/account"

	"github.com/zeromicro/go-zero/rest"
)

````

## 第 2 步：RegisterAuthentication

插入位置：第 1 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`func RegisterAuthentication(server *rest.Server, authentication *account.Authentication)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>authentication</code> | <code>*account.Authentication</code> | 使用 <code>account.Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication.Login</code>；<code>authentication.Me</code>；<code>authentication.VerifySession</code> |

<!-- write-step: 2 bytes=125:503 -->
````go
func RegisterAuthentication(server *rest.Server, authentication *account.Authentication) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/login", Handler: authentication.Login},
		{Method: "GET", Path: "/v1/account/me", Handler: authentication.Me},
		{Method: "POST", Path: "/infra/identity/verify-session", Handler: authentication.VerifySession},
	})
}

````

## 第 3 步：RegisterIdentityRenewal

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func RegisterIdentityRenewal(server *rest.Server, renewal *account.IdentityRenewal)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoute</code> |
| <code>renewal</code> | <code>*account.IdentityRenewal</code> | 使用 <code>account.IdentityRenewal</code> 类型；其定义处的职责：接入仅供 Document 的身份续期/协作者解析端点，并把浏览器写入绑定为 Mutation。<br>在此声明中的实际使用：<code>renewal.Renew</code>；<code>renewal.DocumentCollaborator</code> |

<!-- write-step: 3 bytes=503:833 -->
````go
func RegisterIdentityRenewal(server *rest.Server, renewal *account.IdentityRenewal) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/identity/renew-operation", Handler: renewal.Renew})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/identity/document-collaborator", Handler: renewal.DocumentCollaborator})
}

````

## 第 4 步：RegisterNotificationRecipients

插入位置：第 3 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func RegisterNotificationRecipients(server *rest.Server, recipients *account.NotificationRecipients)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoute</code> |
| <code>recipients</code> | <code>*account.NotificationRecipients</code> | 使用 <code>account.NotificationRecipients</code> 类型；其定义处的职责：将通知候选、收件核验、事件拉取/确认和离开状态开放给真实Document服务。<br>在此声明中的实际使用：<code>recipients.Check</code>；<code>recipients.Administrators</code> |

<!-- write-step: 4 bytes=833:1176 -->
````go
func RegisterNotificationRecipients(server *rest.Server, recipients *account.NotificationRecipients) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/recipients", Handler: recipients.Check})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/administrators", Handler: recipients.Administrators})
}

````

## 第 5 步：RegisterNotificationEvents

插入位置：第 4 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func RegisterNotificationEvents(server *rest.Server, events *account.NotificationEvents)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoute</code> |
| <code>events</code> | <code>*account.NotificationEvents</code> | 使用 <code>account.NotificationEvents</code> 类型；其定义处的职责：将通知候选、收件核验、事件拉取/确认和离开状态开放给真实Document服务。<br>在此声明中的实际使用：<code>events.Events</code>；<code>events.Acknowledge</code> |

<!-- write-step: 5 bytes=1176:1489 -->
````go
func RegisterNotificationEvents(server *rest.Server, events *account.NotificationEvents) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/events", Handler: events.Events})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/notification/events/ack", Handler: events.Acknowledge})
}

````

## 第 6 步：RegisterAccountCloseDecisions

插入位置：第 5 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func RegisterAccountCloseDecisions(server *rest.Server, decisions *account.AccountCloseDecisions)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoute</code> |
| <code>decisions</code> | <code>*account.AccountCloseDecisions</code> | 使用 <code>account.AccountCloseDecisions</code> 类型；其定义处的职责：接入Document读取/确认注销的内部接口，并严格识别可公开续取的最小注销结果。<br>在此声明中的实际使用：<code>decisions.Binding</code>；<code>decisions.Read</code>；<code>decisions.Confirm</code> |

<!-- write-step: 6 bytes=1489:1958 -->
````go
func RegisterAccountCloseDecisions(server *rest.Server, decisions *account.AccountCloseDecisions) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-binding", Handler: decisions.Binding})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-decision", Handler: decisions.Read})
	server.AddRoute(rest.Route{Method: "POST", Path: "/infra/lifecycle/account-close-confirmation", Handler: decisions.Confirm})
}

````

## 第 7 步：RegisterLogout

插入位置：第 6 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func RegisterLogout(server *rest.Server, logout *account.Logout)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>logout</code> | <code>*account.Logout</code> | 使用 <code>account.Logout</code> 类型；其定义处的职责：处理用户退出HTTP与主/恢复Cookie交替。<br>在此声明中的实际使用：<code>logout.Logout</code>；<code>logout.Progress</code> |

<!-- write-step: 7 bytes=1958:2226 -->
````go
func RegisterLogout(server *rest.Server, logout *account.Logout) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/logout", Handler: logout.Logout},
		{Method: "GET", Path: "/v1/account/operations/:operationId", Handler: logout.Progress},
	})
}

````

## 第 8 步：RegisterAccountClosures

插入位置：第 7 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func RegisterAccountClosures(server *rest.Server, closures *account.AccountClosures)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>closures</code> | <code>*account.AccountClosures</code> | 使用 <code>account.AccountClosures</code> 类型；其定义处的职责：处理用户账号注销发起/进度，以及向Document交付原绑定。<br>在此声明中的实际使用：<code>closures.Start</code>；<code>closures.Progress</code> |

<!-- write-step: 8 bytes=2226:2517 -->
````go
func RegisterAccountClosures(server *rest.Server, closures *account.AccountClosures) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/closures", Handler: closures.Start},
		{Method: "GET", Path: "/v1/account/closures/:operationId", Handler: closures.Progress},
	})
}

````

## 第 9 步：RegisterChallenges

插入位置：第 8 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func RegisterChallenges(server *rest.Server, challenges *account.Challenges)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>challenges</code> | <code>*account.Challenges</code> | 使用 <code>account.Challenges</code> 类型；其定义处的职责：将登录、验证码、注册和凭据变更用例接入浏览器 HTTP。<br>在此声明中的实际使用：<code>challenges.Issue</code>；<code>challenges.Signup</code> |

<!-- write-step: 9 bytes=2517:2790 -->
````go
func RegisterChallenges(server *rest.Server, challenges *account.Challenges) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/challenges", Handler: challenges.Issue},
		{Method: "POST", Path: "/v1/account/signup", Handler: challenges.Signup},
	})
}

````

## 第 10 步：RegisterCredentials

插入位置：第 9 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func RegisterCredentials(server *rest.Server, credentials *account.Credentials)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>credentials</code> | <code>*account.Credentials</code> | 使用 <code>account.Credentials</code> 类型；其定义处的职责：将登录、验证码、注册和凭据变更用例接入浏览器 HTTP。<br>在此声明中的实际使用：<code>credentials.ResetPassword</code>；<code>credentials.ChangePassword</code>；<code>credentials.ChangeEmail</code> |

<!-- write-step: 10 bytes=2790:3172 -->
````go
func RegisterCredentials(server *rest.Server, credentials *account.Credentials) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/account/password-resets", Handler: credentials.ResetPassword},
		{Method: "POST", Path: "/v1/account/password", Handler: credentials.ChangePassword},
		{Method: "POST", Path: "/v1/account/email", Handler: credentials.ChangeEmail},
	})
}

````

## 第 11 步：RegisterOrganizationReads

插入位置：第 10 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func RegisterOrganizationReads(server *rest.Server, organizations *account.Organizations)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>organizations</code> | <code>*account.Organizations</code> | 使用 <code>account.Organizations</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>organizations.List</code>；<code>organizations.Get</code>；<code>organizations.Members</code>；<code>organizations.DepartureStatus</code> |

<!-- write-step: 11 bytes=3172:3679 -->
````go
func RegisterOrganizationReads(server *rest.Server, organizations *account.Organizations) {
	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/v1/organizations", Handler: organizations.List},
		{Method: "GET", Path: "/v1/organizations/:organizationId", Handler: organizations.Get},
		{Method: "GET", Path: "/v1/organizations/:organizationId/members", Handler: organizations.Members},
		{Method: "GET", Path: "/infra/lifecycle/member-departure-status", Handler: organizations.DepartureStatus},
	})
}

````

## 第 12 步：RegisterOrganizationTransitions

插入位置：第 11 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func RegisterOrganizationTransitions(server *rest.Server, transitions *account.OrganizationTransitions)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>transitions</code> | <code>*account.OrganizationTransitions</code> | 使用 <code>account.OrganizationTransitions</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>transitions.Role</code>；<code>transitions.Remove</code>；<code>transitions.TransferOwner</code>；<code>transitions.Leave</code> |

<!-- write-step: 12 bytes=3679:4257 -->
````go
func RegisterOrganizationTransitions(server *rest.Server, transitions *account.OrganizationTransitions) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations/:organizationId/members/:userId/role", Handler: transitions.Role},
		{Method: "DELETE", Path: "/v1/organizations/:organizationId/members/:userId", Handler: transitions.Remove},
		{Method: "POST", Path: "/v1/organizations/:organizationId/owner-transfers", Handler: transitions.TransferOwner},
		{Method: "POST", Path: "/v1/organizations/:organizationId/leave", Handler: transitions.Leave},
	})
}

````

## 第 13 步：RegisterOrganizationClosures

插入位置：第 12 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func RegisterOrganizationClosures(server *rest.Server, closures *account.OrganizationClosures)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>closures</code> | <code>*account.OrganizationClosures</code> | 使用 <code>account.OrganizationClosures</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>closures.Close</code>；<code>closures.Confirmation</code> |

<!-- write-step: 13 bytes=4257:4600 -->
````go
func RegisterOrganizationClosures(server *rest.Server, closures *account.OrganizationClosures) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations/:organizationId/closures", Handler: closures.Close},
		{Method: "POST", Path: "/infra/lifecycle/organization-close-confirmation", Handler: closures.Confirmation},
	})
}

````

## 第 14 步：RegisterOrganizationActions

插入位置：第 13 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func RegisterOrganizationActions(server *rest.Server, actions *account.OrganizationActions)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>actions</code> | <code>*account.OrganizationActions</code> | 使用 <code>account.OrganizationActions</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>actions.Create</code>；<code>actions.Edit</code>；<code>actions.ChangeCreationGrant</code> |

<!-- write-step: 14 bytes=4600:5009 -->
````go
func RegisterOrganizationActions(server *rest.Server, actions *account.OrganizationActions) {
	server.AddRoutes([]rest.Route{
		{Method: "POST", Path: "/v1/organizations", Handler: actions.Create},
		{Method: "PATCH", Path: "/v1/organizations/:organizationId", Handler: actions.Edit},
		{Method: "POST", Path: "/v1/organizations/:organizationId/creation-grants", Handler: actions.ChangeCreationGrant},
	})
}

````

## 第 15 步：RegisterMembership

插入位置：第 14 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func RegisterMembership(server *rest.Server, invitations *account.Invitations, applications *account.Applications)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>server</code> | <code>*rest.Server</code> | 在此声明中的实际使用：<code>server.AddRoutes</code> |
| <code>invitations</code> | <code>*account.Invitations</code> | 使用 <code>account.Invitations</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>invitations.Create</code>；<code>invitations.List</code>；<code>invitations.Cancel</code> |
| <code>applications</code> | <code>*account.Applications</code> | 使用 <code>account.Applications</code> 类型；其定义处的职责：把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。<br>在此声明中的实际使用：<code>applications.AcceptInvitation</code>；<code>applications.Apply</code>；<code>applications.List</code>；<code>applications.Decide</code> |

<!-- write-step: 15 bytes=5009:5946 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/routes.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
