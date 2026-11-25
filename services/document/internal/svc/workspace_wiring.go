// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	logic "collaboration/document/internal/logic/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"database/sql"
	"net/http"
)

// authorization.go：保留原实现，按业务合并文件。

func NewAuthorizations(store *platform.Store, fences *logic.Fences, outputs *logic.Outputs) *logic.Authorizations {
	return logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)
}

// authorization_change.go：保留原实现，按业务合并文件。

func NewAuthorizationChanges(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.AuthorizationChanges {
	return logic.NewAuthorizationChanges(store.DB, identity, access)
}

// content_versions.go：保留原实现，按业务合并文件。

func NewContentVersions(store *platform.Store, access *logic.Access) (*logic.ContentVersions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewContentVersions(store.DB, access, validator), nil
}

// departure.go：保留原实现，按业务合并文件。

func NewDepartures(store *platform.Store, cfg platform.Config, accountHTTP *http.Client) *logic.Departures {
	return logic.NewDepartures(store.DB, repository.NewDepartures(store.DB), client.NewAccountDeparture(cfg.Peers["accountHttp"], accountHTTP))
}

// document.go：保留原实现，按业务合并文件。

func NewDocuments(store *platform.Store, identity *logic.Identity, access *logic.Access, quotaLimit uint64) *logic.Documents {
	return logic.NewDocuments(store.DB, identity, access, quotaLimit)
}

// edit_sessions.go：保留原实现，按业务合并文件。

func NewEditSessions(store *platform.Store, cfg platform.Config, access *logic.Access, tokens logic.InvitationCipher) *logic.EditSessions {
	return logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)
}

// identity.go：保留原实现，按业务合并文件。

func NewIdentity(peer pb.IdentityInternalClient, base string, httpClient *http.Client) *logic.Identity {
	account := client.NewAccountSession(base, httpClient)
	return logic.NewIdentity(peer, account, account)
}
func NewAccess(db *sql.DB, identity *logic.Identity) *logic.Access {
	return logic.NewAccess(repository.NewAccess(db), identity)
}

// knowledge_base.go：保留原实现，按业务合并文件。

func NewKnowledgeBases(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.KnowledgeBases {
	return logic.NewKnowledgeBases(store.DB, identity, access)
}

// management.go：保留原实现，按业务合并文件。

func NewManagement(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.Management {
	return logic.NewManagement(store.DB, identity, access)
}

// readings.go：保留原实现，按业务合并文件。

func NewReadings(access *logic.Access, mainViews *logic.MainReadViews) (*logic.Readings, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewReadings(access, mainViews, validator), nil
}

// sharing.go：保留原实现，按业务合并文件。

func NewSharing(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, tokens logic.InvitationCipher, directory *client.AccountDirectory) *logic.Sharing {
	return logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)
}

// space.go：保留原实现，按业务合并文件。

func NewPagination(cfg platform.Config) *logic.Pagination {
	return logic.NewPagination(client.NewCursorSigner(cfg.KeyFile))
}
func NewDirectory(cfg platform.Config, accountHTTP *http.Client) *client.AccountDirectory {
	return client.NewAccountDirectory(cfg.Peers["accountHttp"], accountHTTP)
}
func NewSpaces(identity *logic.Identity, directory *client.AccountDirectory, pagination *logic.Pagination) *logic.Spaces {
	return logic.NewSpaces(identity, directory, pagination)
}

// transactions.go：保留原实现，按业务合并文件。

func NewTransactions(store *platform.Store, identity *logic.Identity, outputs *logic.Outputs) *logic.Transactions {
	return logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)
}

// visibility.go：保留原实现，按业务合并文件。

func NewVisibilities(knowledgeBases *logic.KnowledgeBases, access *logic.Access) *logic.Visibilities {
	return logic.NewVisibilities(knowledgeBases, access)
}
