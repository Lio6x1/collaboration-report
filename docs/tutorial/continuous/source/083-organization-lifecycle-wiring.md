# services/account/internal/svc/organization_lifecycle_wiring.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/083-organization-lifecycle-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewClosure`、`NewLogout`、`NewInvitations`、`NewApplications`、`NewOrganizations`、`NewOrganizationTransitions`、`NewOrganizationActions`、`NewOrganizationClosures`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"collaboration/account/internal/client"
	domain "collaboration/account/internal/domain/account"
	logic "collaboration/account/internal/logic/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"database/sql"
	"net/http"
)

// closure.go：保留原实现，按业务合并文件。

func NewClosure(store *platform.Store, rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *logic.Closure {
	peer := client.NewDocument(rpc, httpClient, endpoint)
	return logic.NewClosure(store.DB, repository.NewClosure(store.DB), peer)
}

// logout.go：保留原实现，按业务合并文件。

func NewLogout(store *platform.Store, authentication *logic.Authentication) *logic.Logout {
	return logic.NewLogout(
		store.DB,
		authentication,
		repository.NewOperationReceipts(store.DB),
		repository.NewMutation(),
		repository.NewClosure(store.DB),
	)
}

// membership.go：保留原实现，按业务合并文件。

func NewInvitations(store *platform.Store, mutations *logic.Mutations, key []byte, capture logic.CaptureSource) *logic.Invitations {
	return logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)
}

func NewApplications(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.Applications {
	return logic.NewApplications(
		store.DB,
		mutations,
		repository.NewApplications(store.DB),
		repository.NewOrganizations(store.DB),
		repository.NewInvitations(store.DB),
		capture,
	)
}

// organization.go：保留原实现，按业务合并文件。

func NewOrganizations(store *platform.Store, capture logic.CaptureSource) *logic.Organizations {
	return logic.NewOrganizations(store.DB, repository.NewOrganizations(store.DB), capture)
}

func NewOrganizationTransitions(store *platform.Store, mutations *logic.Mutations, httpClient *http.Client, endpoint string, capture logic.CaptureSource) *logic.OrganizationTransitions {
	return logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)
}

func NewOrganizationActions(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.OrganizationActions {
	return logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)
}

// organization_close.go：保留原实现，按业务合并文件。

func NewOrganizationClosures(db *sql.DB, mutations *logic.Mutations, fence logic.AccountCloseFencePeer, httpClient *http.Client, documentEndpoint string) *logic.OrganizationClosures {
	return logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))
}
````

UTF-8/LF全文SHA-256：`e50948d1cb871b642e81a42288153d5177267059c4ecd601b19c6615c3893b29`。[参考实现](../../../../services/account/internal/svc/organization_lifecycle_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
