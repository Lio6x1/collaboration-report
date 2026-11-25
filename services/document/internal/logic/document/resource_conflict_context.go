package document

import (
	domain "collaboration/document/internal/domain/document"
	documents "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

// conflictContains 在 REVIEW 原轮次权限已核验后扩展原冲突输入的资源集合。
// 只读取冲突页面已物化的版本，不隐式创建快照；候选索引不能代替原固定事实证明。
func (s *ResourceReading) conflictContains(ctx context.Context, tx *sql.Tx, target ResourceTarget) (bool, error) {
	operation, err := resources.ConflictResourceOperation(ctx, tx, target.Document, target.Context, target.Resource)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fact, err := documents.MergeRow(ctx, tx, operation)
	if err != nil {
		return false, err
	}
	if fact.Document != target.Document || fact.Round != target.Context {
		return false, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	}
	attempt, fixed, _, err := s.reviews.conflictOriginal(ctx, tx, fact)
	if err != nil {
		return false, err
	}
	raw, err := documents.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(fact.Operation, attempt.ID))
	if err != nil {
		return false, err
	}
	versions, err := validateConflictVersions(ctx, tx, raw, fixed)
	if err != nil {
		return false, err
	}
	// 三份版本已逐项比对原固定输入，仍以真实 CONTENT 引用确认目标资源。
	for _, version := range []uint64{versions.Base, versions.Main, versions.Submitted} {
		found, err := resources.VersionContains(ctx, tx, target.Document, version, target.Resource)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}
