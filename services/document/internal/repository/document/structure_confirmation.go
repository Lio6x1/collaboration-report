package document

import (
	"context"
	"database/sql"
	"time"
)

type StructureConfirmationFact struct {
	Binding        PendingStructurePreviewBinding
	Snapshot       []byte
	ReceiptExpires time.Time
}

// 确认读取实际原回执、原上下文与不可变快照；没有任何一步换池或用调用者伪造的PREPARING状态。
func LockedStructureConfirmation(ctx context.Context, tx *sql.Tx, operation string) (StructureConfirmationFact, error) {
	var fact StructureConfirmationFact
	fact.Binding.Operation = operation
	err := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.state,o.request_fingerprint,o.target_json,o.expires_at,
 original.payload,fixed.payload FROM operation_receipt o
 LEFT JOIN durable_task original ON original.kind='STRUCTURE_PREVIEW_CONTEXT' AND original.business_key=o.operation_id AND original.operation_id=o.operation_id
 LEFT JOIN durable_task fixed ON fixed.kind='STRUCTURE_PREVIEW_FIXED' AND fixed.business_key=o.operation_id AND fixed.operation_id=o.operation_id
 WHERE o.operation_id=? AND o.kind='STRUCTURE_PREVIEW' FOR UPDATE`, operation).Scan(
		&fact.Binding.Actor, &fact.Binding.State, &fact.Binding.Fingerprint, &fact.Binding.Target,
		&fact.ReceiptExpires, &fact.Binding.Original, &fact.Snapshot,
	)
	return fact, err
}
