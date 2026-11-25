# services/document/internal/svc/workspace_wiring.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/279-workspace-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewAuthorizations`、`NewAuthorizationChanges`、`NewContentVersions`、`NewDepartures`、`NewDocuments`、`NewEditSessions`、`NewIdentity`、`NewAccess`、`NewKnowledgeBases`、`NewManagement`、`NewReadings`、`NewSharing`、`NewPagination`、`NewDirectory`、`NewSpaces`、`NewTransactions`、`NewVisibilities`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`f1754585b957eda8b715aea96fdd88643e190220734874845f1fa7dd78fa1d52`。[参考实现](../../../../services/document/internal/svc/workspace_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
