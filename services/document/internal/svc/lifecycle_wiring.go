// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
)

// account_retention.go：保留原实现，按业务合并文件。

func NewAccountRetentions(db *sql.DB, authority logic.AccountRetentionAuthority, cleaner logic.AccountDocumentCleaner) (*logic.AccountRetentions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewAccountRetentions(db, authority, validator, cleaner), nil
}

// organization_close.go：保留原实现，按业务合并文件。

func NewOrganizationClosures(db *sql.DB, authority logic.OrganizationCloseAuthority, cleaner logic.AccountDocumentCleaner) (*logic.OrganizationClosures, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewOrganizationClosures(db, authority, validator, cleaner), nil
}

// personal_transfer.go：保留原实现，按业务合并文件。

func NewPersonalTransfers(store *platform.Store, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, fences *logic.Fences, storage logic.PersonalTransferStorageReader, quotaLimit uint64, instance string) *logic.PersonalTransfers {
	return logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)
}

// purge.go：保留原实现，按业务合并文件。

func NewPurges(store *platform.Store, trash *logic.Trash, cleaner logic.AccountDocumentCleaner, instance string) *logic.Purges {
	return logic.NewPurges(store.DB, trash, cleaner, instance)
}

// retained_governance.go：保留原实现，按业务合并文件。

func NewRetainedGovernance(store *platform.Store, authority logic.RetainedGovernanceAuthority, cleaner logic.AccountDocumentCleaner, resources logic.RetainedResourceCleaner, exports logic.RetainedExportDrainer, instance string) *logic.RetainedGovernance {
	return logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)
}
