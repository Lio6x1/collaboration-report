package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// 只覆盖实际保存私有内容/标题/固定输入的Doc任务。通知、原授权屏障、
// 房间编号与上传恢复凭据另有职责，不能为了清正文把原恢复依据全部删除。
var accountBodyTaskKinds = []string{
	"SUBMISSION_RECOVER", "CAPTURE_FINALIZE", "MERGE_RECOVER", "MAIN_COMMIT_FIXED", "MAIN_RECONCILE",
	"MAIN_CONTROL_CONTEXT", "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_MAIN_FIXED", "MAIN_CONTROL_RECONCILE",
	"PUBLICATION_CONTEXT", "PUBLICATION_RECOVER", "PULL_MAIN_CONTEXT", "PULL_MAIN_RECOVER", "PULL_MAIN_COMMITTED", "PULL_MAIN_RECONCILE",
	"STRUCTURE_PREVIEW_CONTEXT", "STRUCTURE_PREVIEW_RECOVER", "STRUCTURE_PREVIEW_FIXED",
	"DISCARD_PREVIEW_CONTEXT", "DISCARD_PREVIEW_RECOVER", "DISCARD_PREVIEW_FIXED",
	"DISCARD_DRAFT_CONTEXT", "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_COMMITTED", "DISCARD_DRAFT_RECONCILE",
	"REVERSE_CONTEXT", "REVERSE_RECOVER", "REVERSE_FIXED", "REVERSE_COMMITTED", "REVERSE_RECONCILE", "REVERSE_REVIEW_SOURCE",
	"RESOLUTION_CONTEXT", "RESOLUTION_RECOVER", "RESOLUTION_FIXED", "RESOLUTION_COMMITTED", "RESOLUTION_RECONCILE",
	"MERGE_ATTEMPT_CONTEXT", "MERGE_ATTEMPT_FIXED", "MERGE_ATTEMPT_RANGE_FIXED", "CONFLICT_CONTEXT_VERSIONS",
	"REVIEW_ROUND_CONTEXT", "REVIEW_CAPTURE_CUT", "MAIN_READ_CONTEXT",
}

type accountPayloadTask struct {
	id               uint64
	operation, owner sql.NullString
	state            string
}

func AccountPayloadCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var operation, state string
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_PAYLOAD_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if operation != request.Operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.AccountDocumentCleanupRequest != request {
		return result, false, errors.New("ACCOUNT_PAYLOAD_CLEANUP_BINDING_CHANGED")
	}
	return result, true, nil
}

func accountPayloadTasks(ctx context.Context, tx *sql.Tx, predicate string, args []any) ([]accountPayloadTask, error) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(accountBodyTaskKinds)), ",")
	all := make([]any, 0, len(accountBodyTaskKinds)+len(args))
	for _, kind := range accountBodyTaskKinds {
		all = append(all, kind)
	}
	all = append(all, args...)
	// 007迁移从原payload派生归属并建索引，正文更新时由MySQL同步维护；
	// 避免每次注销逐行解析全部历史任务JSON，仍使用同一事务的真实绑定。
	rows, err := tx.QueryContext(ctx, "SELECT id,operation_id,state,cleanup_document FROM durable_task WHERE kind IN ("+marks+") AND ("+predicate+") ORDER BY id", all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []accountPayloadTask{}
	for rows.Next() {
		var row accountPayloadTask
		if err = rows.Scan(&row.id, &row.operation, &row.state, &row.owner); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// 先用原context确定文档，再沿精确operation_id扩展无doc字段的子固定任务。
// 不读取/输出正文，不用business_key前缀猜归属；跨文档或未终态必须拒绝。
func RemoveAccountPayloads(ctx context.Context, tx *sql.Tx, authority platform.AccountDocumentCleanupContext) error {
	roots, err := accountPayloadTasks(ctx, tx, "cleanup_document=?", []any{authority.Document})
	if err != nil {
		return err
	}
	operations := map[string]bool{}
	tasks := map[uint64]accountPayloadTask{}
	for _, root := range roots {
		tasks[root.id] = root
		if root.operation.Valid && root.operation.String != "" {
			operations[root.operation.String] = true
		}
	}
	for operation := range operations {
		children, err := accountPayloadTasks(ctx, tx, "operation_id=?", []any{operation})
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.owner.Valid && child.owner.String != authority.Document {
				return errors.New("ACCOUNT_PAYLOAD_DOCUMENT_CHANGED")
			}
			tasks[child.id] = child
		}
	}
	for _, task := range tasks {
		if task.state != "SUCCEEDED" && task.state != "REJECTED" {
			return domain.Violation(domain.StateConflict, "ACCOUNT_PAYLOAD_RESULT_UNKNOWN")
		}
	}
	// 原任务终态/原号保留，原正文不可再提供给迟到安装器；Node永久墓碑
	// 已阻止物化恢复。保留规范doc context，后续排空核验仍可精确定位。
	raw, err := json.Marshal(map[string]any{
		"body_removed": true, "cleanup_context": authority,
		"context": map[string]any{"target": map[string]string{"document_id": authority.Document}},
	})
	if err != nil {
		return err
	}
	for _, task := range tasks {
		result, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE id=? AND state=?", string(raw), task.id, task.state)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return domain.Violation(domain.StateConflict, "ACCOUNT_PAYLOAD_TASK_CHANGED")
		}
	}
	return nil
}
