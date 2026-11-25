package document

import (
	dto "collaboration/contracts/httpdto"
	"context"
	"database/sql"
)

func InsertControlledStructure(ctx context.Context, tx *sql.Tx, document, parent uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO structure_revision(document_id,parent_id,source_operation_id,schema_version) VALUES(?,?,?,'structure-v1')", document, parent, operation)
}

func InsertStructureEntries(ctx context.Context, tx *sql.Tx, document, structure uint64, entries []dto.StructureEntry) error {
	for _, entry := range entries {
		_, err := tx.ExecContext(ctx, `INSERT INTO structure_entry(structure_id,document_id,node_id,
 parent_node_id,chapter_id,kind,title,sort_order,unit_id,split_mode) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			structure, document, entry.NodeId, entry.ParentNodeId, entry.ChapterId, entry.Kind,
			entry.Title, entry.SortOrder, entry.UnitId, entry.SplitLevel2)
		if err != nil {
			return err
		}
	}
	return nil
}

func PointToControlledStructure(ctx context.Context, tx *sql.Tx, document, structure uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET current_structure_id=? WHERE id=?", structure, document)
	return err
}
