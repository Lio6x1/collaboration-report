# services/document/internal/repository/document/releases.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/170-releases.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`FixedReleaseRow`、`ReleaseRows`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
	"time"
)

func FixedReleaseRow(ctx context.Context, tx *sql.Tx, document, release uint64) (domain.FixedRelease, error) {
	var fixed domain.FixedRelease
	var created time.Time
	err := tx.QueryRowContext(ctx, "SELECT CAST(r.id AS CHAR),CAST(r.document_id AS CHAR),CAST(r.manifest_id AS CHAR),CAST(r.publisher_id AS CHAR),r.created_at,r.title,CAST(m.structure_id AS CHAR),CAST(m.style_id AS CHAR) FROM `release` r JOIN main_manifest m ON m.id=r.manifest_id AND m.document_id=r.document_id WHERE r.document_id=? AND r.id=?", document, release).Scan(&fixed.Release.Id, &fixed.Release.DocumentId, &fixed.Release.ManifestId, &fixed.Release.PublisherId, &created, &fixed.Title, &fixed.Structure, &fixed.Style)
	if errors.Is(err, sql.ErrNoRows) {
		return fixed, domain.Violation(domain.EntityNotFound, "RELEASE_NOT_FOUND")
	}
	if err == nil {
		fixed.Release.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	}
	return fixed, err
}

// 不可变历史按数值ID倒序；游标边界和页大小由用例先验签，SQL只在原输出事务中取一页。
func ReleaseRows(ctx context.Context, tx *sql.Tx, document, before uint64, limit int) ([]dto.Release, error) {
	statement := "SELECT CAST(id AS CHAR),CAST(document_id AS CHAR),CAST(manifest_id AS CHAR),CAST(publisher_id AS CHAR),created_at FROM `release` WHERE document_id=?"
	args := []any{document}
	if before != 0 {
		statement += " AND id<?"
		args = append(args, before)
	}
	statement += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []dto.Release{}
	for rows.Next() {
		var value dto.Release
		var created time.Time
		if err = rows.Scan(&value.Id, &value.DocumentId, &value.ManifestId, &value.PublisherId, &created); err != nil {
			return nil, err
		}
		value.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		values = append(values, value)
	}
	return values, rows.Err()
}
````

UTF-8/LF全文SHA-256：`5cf8b4ffa13f4edeecc34ed30d2655927e6f1418d04635604816795654de4144`。[参考实现](../../../../services/document/internal/repository/document/releases.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
