# services/document/internal/logic/document/control_manifest_validation.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/200-control-manifest-validation.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`validEmptyControlledManifest`、`structureManifestUnitSetMatches`、`controlManifestUnitSetMatches`、`validateControlInstallation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

func validEmptyControlledManifest(fixed mainInstallContext) bool {
	if len(fixed.Units) != 0 || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.Target.DocumentId == 0 ||
		fixed.Context.Target.Purpose != "MAIN_CONTROL" || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 {
		return false
	}
	digest, err := domain.MainManifestDigest(fixed.Manifest, []*pb.ManifestUnit{})
	return err == nil && bytes.Equal(digest[:], fixed.Digest)
}

func structureManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string) bool {
	if len(rows) == 0 && len(structure.Entries) != 0 {
		return false
	}
	expected := map[string]bool{}
	for _, entry := range structure.Entries {
		if entry.UnitId == nil {
			continue
		}
		if _, err := domain.CanonicalID(*entry.UnitId); err != nil || expected[*entry.UnitId] {
			return false
		}
		expected[*entry.UnitId] = true
	}
	if len(expected) != len(rows) {
		return false
	}
	for _, row := range rows {
		if !expected[row["unit_id"]] {
			return false
		}
		if _, err := domain.CanonicalID(row["content_version_id"]); err != nil {
			return false
		}
		delete(expected, row["unit_id"])
	}
	return len(expected) == 0
}

func controlManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string, units []*pb.ManifestUnit) bool {
	if !structureManifestUnitSetMatches(structure, rows) || len(rows) != len(units) {
		return false
	}
	expected := map[string]string{}
	for _, unit := range units {
		if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || expected[domain.TextID(unit.UnitId)] != "" {
			return false
		}
		expected[domain.TextID(unit.UnitId)] = domain.TextID(unit.ContentVersionId)
	}
	for _, row := range rows {
		if expected[row["unit_id"]] != row["content_version_id"] {
			return false
		}
	}
	return true
}

// 空清单只能来自真实空目录与同号实际manifest；摘要一致仍不足以证明没有漏掉正文。
// 这里是系统收尾读取，沿私有库同一只读TX核验，不重新取得已撤销的原用户准入。
func (s *MainControls) validateControlInstallation(ctx context.Context, fixed mainInstallContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_MANIFEST_UNKNOWN")
	if fixed.Context == nil || fixed.Context.Target == nil {
		return invalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	document := fixed.Context.Target.DocumentId
	valid, err := repository.MainControlManifestBinding(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId)
	if err != nil {
		return err
	}
	if !valid {
		return invalid
	}
	structure, err := repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(document))
	if err != nil {
		return err
	}
	rows, err := repository.ManifestUnits(ctx, tx, domain.TextID(fixed.Manifest), document)
	if err != nil {
		return err
	}
	if !controlManifestUnitSetMatches(structure, rows, fixed.Units) {
		return invalid
	}
	return tx.Commit()
}
````

UTF-8/LF全文SHA-256：`4bd19ca86a397de6f09cd983f279aab0bd10ff8c87e4b0b7653730e4a6ea680d`。[参考实现](../../../../services/document/internal/logic/document/control_manifest_validation.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
