package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

// 可写正文依赖先持 Space/quota，再锁不可变资源；后续 canonical 版本 INSERT 的
// 同事务计费不会从资源反向新取 quota。资格必须精确到当前 Doc/Space 的已接受转让。
func LockedDocumentResource(ctx context.Context, tx *sql.Tx, resource, document, space uint64) (string, error) {
	if err := LockDocumentCapacity(ctx, tx, document); err != nil {
		return "", err
	}
	_, state, err := LockedResourceSpace(ctx, tx, resource)
	if err != nil {
		return state, err
	}
	allowed, err := resources.ResourceAllowedForDocument(ctx, tx, resource, document, space)
	if err != nil {
		return state, err
	}
	if !allowed {
		return state, domain.Violation(domain.StateConflict, "RESOURCE_SCOPE_CHANGED")
	}
	return state, nil
}
