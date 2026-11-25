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
