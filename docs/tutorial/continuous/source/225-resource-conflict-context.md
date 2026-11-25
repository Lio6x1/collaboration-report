# services/document/internal/logic/document/resource_conflict_context.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/225-resource-conflict-context.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`conflictContains`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`774e45b63f52e5157dc8451fa2fdf5dda1cd144a891fd5006d391383b979ef94`。[参考实现](../../../../services/document/internal/logic/document/resource_conflict_context.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
