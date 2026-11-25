# services/document/internal/svc/lifecycle_wiring.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/277-lifecycle-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewAccountRetentions`、`NewOrganizationClosures`、`NewPersonalTransfers`、`NewPurges`、`NewRetainedGovernance`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`47adc6e81567d43e5f0bec7f93865c206c8679221493ff02aa04d2e47f4b53ee`。[参考实现](../../../../services/document/internal/svc/lifecycle_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
