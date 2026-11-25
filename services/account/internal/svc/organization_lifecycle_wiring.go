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
