# services/document/internal/repository/document/publication.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/166-publication.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`PublicationReceipt`、`PublicationResult`、`RegisterPublication`、`ClaimPublicationTask`、`InsertRelease`、`ReleaseManifest`、`PublicationEventSequence`、`SetPublication`、`PendingPublicationBinding`、`PendingPublicationBindings`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"time"
)

type PublicationReceipt struct {
	Actor               uint64
	Generation          uint64
	Kind, State         string
	Fingerprint, Result []byte
	Expires             time.Time
}

func PublicationResult(ctx context.Context, q Queryer, operation string) (PublicationReceipt, error) {
	var receipt PublicationReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,generation,kind,state,request_fingerprint,result_json,expires_at FROM operation_receipt WHERE operation_id=? AND kind IN ('RELEASE_CREATE','SWITCH','UNPUBLISH')", operation).
		Scan(&receipt.Actor, &receipt.Generation, &receipt.Kind, &receipt.State, &receipt.Fingerprint, &receipt.Result, &receipt.Expires)
	return receipt, err
}

func RegisterPublication(ctx context.Context, tx *sql.Tx, operation string, original []byte, capture bool) error {
	if err := InsertSubmissionTask(ctx, tx, "PUBLICATION_CONTEXT", operation, operation, original, "SUCCEEDED"); err != nil {
		return err
	}
	if capture {
		return InsertSubmissionTask(ctx, tx, "PUBLICATION_RECOVER", operation, operation, original, "QUEUED")
	}
	return nil
}

func ClaimPublicationTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "PUBLICATION_RECOVER", "PUBLICATION_RECOVER")
}

func InsertRelease(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, title, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO `release`(document_id,manifest_id,title,publisher_id,operation_id) VALUES(?,?,?,?,?)", document, manifest, title, actor, operation)
}

func ReleaseManifest(ctx context.Context, q Queryer, document, release uint64) (uint64, error) {
	var manifest uint64
	err := q.QueryRowContext(ctx, "SELECT manifest_id FROM `release` WHERE document_id=? AND id=?", document, release).Scan(&manifest)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "RELEASE_NOT_FOUND")
	}
	return manifest, err
}

func PublicationEventSequence(ctx context.Context, tx *sql.Tx, document uint64) (uint64, error) {
	var sequence uint64
	err := tx.QueryRowContext(ctx, "SELECT event_seq FROM document WHERE id=? FOR UPDATE", document).Scan(&sequence)
	return sequence, err
}

// 业务指针、单调水位和唯一outbox在调用方同一事务写入；这里不自行提交或发送Kafka。
func SetPublication(ctx context.Context, tx *sql.Tx, document, release, epoch, sequence uint64, operation, eventType string, payload []byte) error {
	var current any
	state := "UNPUBLISHED"
	if release != 0 {
		current, state = release, "PUBLISHED"
	}
	if _, err := tx.ExecContext(ctx, "UPDATE document SET current_release_id=?,publication_state=?,publication_epoch=?,event_seq=? WHERE id=?", current, state, epoch, sequence, document); err != nil {
		return err
	}
	return InsertIndexEvent(ctx, tx, document, sequence, operation, eventType, payload)
}

type PendingPublicationBinding struct {
	Operation, State, Kind                 string
	Actor                                  uint64
	Payload, Original, Fingerprint, Target []byte
}

func PendingPublicationBindings(ctx context.Context, db *sql.DB) ([]PendingPublicationBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.operation_id,t.payload,original.payload,
		o.actor_id,o.kind,o.state,o.request_fingerprint,o.target_json
		FROM durable_task t
		LEFT JOIN durable_task original ON original.kind='PUBLICATION_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id AND original.state='SUCCEEDED'
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		WHERE t.kind='PUBLICATION_RECOVER' AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []PendingPublicationBinding
	for rows.Next() {
		var fact PendingPublicationBinding
		if err = rows.Scan(&fact.Operation, &fact.Payload, &fact.Original, &fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Target); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
````

UTF-8/LF全文SHA-256：`5e06f443f052032d9a7326af61e723509fefb078bf99aa0b7fd22b5b240f3644`。[参考实现](../../../../services/document/internal/repository/document/publication.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
