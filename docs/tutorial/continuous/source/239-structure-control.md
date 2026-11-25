# services/document/internal/logic/document/structure_control.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/239-structure-control.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`stableControlledEntries`、`prepareStableControlledLayout`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

func stableControlledEntries(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.StructureChange) ([]dto.StructureEntry, error) {
	if current.CurrentStructureId != input.ExpectedStructureId {
		return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	old, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return nil, err
	}
	return domain.ApplyStableStructureChanges(old.Entries, input)
}

func prepareStableControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error) {
	layout := mainControlLayout{Structure: fixed.Structure}
	if fixed.Intent.Structure == nil {
		return layout, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_INTENT_UNKNOWN")
	}
	entries, err := stableControlledEntries(ctx, tx, current, *fixed.Intent.Structure)
	if err != nil {
		return layout, err
	}
	// 原可靠MAIN已经准备；目录与各单元版本/manifest在调用方同一最终事务追加。
	// rename/move保持unit与branch身份，不改写历史，也不重新采集已固定的审核正文。
	layout.Structure, err = repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)
	if err != nil {
		return layout, err
	}
	return layout, repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)
}
````

UTF-8/LF全文SHA-256：`68314efef4113c55b74317c1553246bb2d2a7b6ef649b210b5ae5f0e9f819262`。[参考实现](../../../../services/document/internal/logic/document/structure_control.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
