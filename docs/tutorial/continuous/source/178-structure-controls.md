# services/document/internal/repository/document/structure_controls.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/178-structure-controls.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`InsertControlledStructure`、`InsertStructureEntries`、`PointToControlledStructure`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`67eb14c4d11ea0ca3125b4183728ca54ecc1a604d46bd557dba203a29783026d`。[参考实现](../../../../services/document/internal/repository/document/structure_controls.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
