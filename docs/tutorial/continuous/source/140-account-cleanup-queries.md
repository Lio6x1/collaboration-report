# services/document/internal/repository/document/account_cleanup_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/140-account-cleanup-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`LockAccountCleanupDocument`、`AccountDocumentWritesPending`、`DocumentWritesPending`、`AccountHistoryCleanupReceipt`、`RemoveAccountHistoryDependents`、`deleteHistoryLeaves`、`RemoveAccountHistoryGraph`、`AccountHistoryGraph`、`historyRevisions`、`AccountHistoryGraphRow`、`DiscoverAccountRelations`、`ApplyAccountRelations`、`AccountResourceCleanupReceipt`、`ReleaseAccountDraftResources`、`ReleaseDocumentDraftResourcePins`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	allocations "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

// 来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
// 不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。
func LockAccountCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error {
	epochs := map[string]uint64{}
	for _, scope := range scopes {
		epochs[scope.Key] = scope.Epoch
	}
	var space, kb, acl, publication uint64
	var state string
	err := tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch
		FROM document WHERE id=? FOR UPDATE`, item.Document).Scan(&space, &kb, &state, &acl, &publication)
	if err != nil {
		return err
	}
	if space != item.Space || kb != item.KnowledgeBase || state != "CLOSING" || acl != epochs["doc-acl:"+domain.TextID(item.Document)] || publication != epochs["publication:"+domain.TextID(item.Document)] {
		return errors.New("ACCOUNT_DOCUMENT_METADATA_CHANGED")
	}
	if kb != 0 {
		var parent, revision uint64
		if err = tx.QueryRowContext(ctx, "SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE", kb).Scan(&parent, &state, &revision); err != nil {
			return err
		}
		if parent != space || state != "CLOSING" || revision != epochs["kb-acl:"+domain.TextID(kb)] {
			return errors.New("ACCOUNT_DOCUMENT_PARENT_CHANGED")
		}
	}
	var kind string
	var owner, organization uint64
	if err = tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(account_owner_id,0),COALESCE(organization_id,0)
		FROM space WHERE id=? FOR UPDATE`, space).Scan(&kind, &state, &owner, &organization); err != nil {
		return err
	}
	owned := kind == "PERSONAL" && owner == plan.Binding.User
	closingOrganization := kind == "ORGANIZATION" && slices.Contains(plan.Binding.ClosingOrganizations, organization)
	if state != "CLOSING" || (!owned && !closingOrganization) {
		return errors.New("ACCOUNT_DOCUMENT_SPACE_CHANGED")
	}
	return nil
}

// 单文档资格以原持久关联定位任务：两类非文档路由（MERGE、CONFLICT_RESOLUTION）
// 用原admission/业务记录；payload保留历史大小写。任何非终态driver都等待原恢复器，
// 包括用户receipt已SUCCEEDED而Node安装仍UNKNOWN的RECONCILE，绝不使用TTL或lease。
func AccountDocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64) (bool, error) {
	return DocumentWritesPending(ctx, tx, document, "")
}

// 仅排除清理流程自身的原receipt；正文driver/安装UNKNOWN及任何其他原号仍然保护。
// Account沿旧入口传空串，保持原资格检查。不能把整个PURGE kind都排除掉。
func DocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64, excludingOperation string) (bool, error) {
	var pending bool
	id := domain.TextID(document)
	path := "/v1/documents/" + id + "/%"
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM durable_task t
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		LEFT JOIN submission_operation s ON s.operation_id=t.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		WHERE t.kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','MERGE_RECOVER','MAIN_RECONCILE',
		'MAIN_CONTROL_RECOVER','MAIN_CONTROL_RECONCILE','PUBLICATION_RECOVER','PULL_MAIN_RECOVER','PULL_MAIN_RECONCILE',
		'STRUCTURE_PREVIEW_RECOVER','DISCARD_PREVIEW_RECOVER','DISCARD_DRAFT_RECOVER','DISCARD_DRAFT_RECONCILE',
		'REVERSE_RECOVER','REVERSE_RECONCILE','RESOLUTION_RECOVER','RESOLUTION_RECONCILE')
		AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
		 s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=t.operation_id AND a.document_id=?)
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.capture.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.fixed.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.main.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Context.target.document_id'))=?
		))`, document, document, path, document, id, id, id, id, id).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// task可能尚未领取，或原终态提交结果未知。还要看原操作而不是仅数任务；
	// 原admission与receipt在同一事务产生，来源关闭后不会再合法创建新的写入操作。
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM operation_receipt o
		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=o.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		WHERE o.operation_id<>? AND o.state NOT IN ('SUCCEEDED','REJECTED','CANCELLED') AND (
		 s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=o.operation_id AND a.document_id=?)
		))`, excludingOperation, document, document, path, document).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// 不能把“查不到未终结任务”当成安装成功。每个已接纳正文操作必须有原恢复器
	// 的真实终态；成功送审还需Finalize，成功MAIN和所有草稿控制还需真实安装收尾。
	// MERGE早期明确拒绝可以没有Node准备；它的原RECOVER终态与两侧控制屏障一起证明。
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM operation_receipt o
		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=o.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		LEFT JOIN durable_task original ON original.business_key=o.operation_id AND original.operation_id=o.operation_id
		 AND original.kind=CASE o.kind
		 WHEN 'SUBMISSION' THEN 'SUBMISSION_RECOVER' WHEN 'MERGE' THEN 'MERGE_RECOVER'
		 WHEN 'MAIN_LOCK' THEN 'MAIN_CONTROL_RECOVER' WHEN 'STYLE_CHANGE' THEN 'MAIN_CONTROL_RECOVER'
		 WHEN 'CHAPTER_CREATE' THEN 'MAIN_CONTROL_RECOVER' WHEN 'STRUCTURE_CHANGE' THEN 'MAIN_CONTROL_RECOVER'
		 WHEN 'RELEASE_CREATE' THEN 'PUBLICATION_RECOVER' WHEN 'PULL_MAIN' THEN 'PULL_MAIN_RECOVER'
		 WHEN 'STRUCTURE_PREVIEW' THEN 'STRUCTURE_PREVIEW_RECOVER' WHEN 'DISCARD_PREVIEW' THEN 'DISCARD_PREVIEW_RECOVER'
		 WHEN 'DISCARD_DRAFT' THEN 'DISCARD_DRAFT_RECOVER' WHEN 'REVERSE' THEN 'REVERSE_RECOVER'
		 WHEN 'CONFLICT_RESOLUTION' THEN 'RESOLUTION_RECOVER' END
		LEFT JOIN durable_task installed ON installed.business_key=o.operation_id AND installed.operation_id=o.operation_id
		 AND installed.kind=CASE
		 WHEN o.kind='SUBMISSION' AND o.state='SUCCEEDED' THEN 'CAPTURE_FINALIZE'
		 WHEN o.kind='MERGE' AND o.state='SUCCEEDED' THEN 'MAIN_RECONCILE'
		 WHEN o.kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE') THEN 'MAIN_CONTROL_RECONCILE'
		 WHEN o.kind='PULL_MAIN' THEN 'PULL_MAIN_RECONCILE' WHEN o.kind='DISCARD_DRAFT' THEN 'DISCARD_DRAFT_RECONCILE'
		 WHEN o.kind='REVERSE' THEN 'REVERSE_RECONCILE' WHEN o.kind='CONFLICT_RESOLUTION' THEN 'RESOLUTION_RECONCILE' END
		WHERE o.kind IN ('SUBMISSION','MERGE','MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE',
		'RELEASE_CREATE','PULL_MAIN','STRUCTURE_PREVIEW','DISCARD_PREVIEW','DISCARD_DRAFT','REVERSE','CONFLICT_RESOLUTION')
		AND (s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=o.operation_id AND a.document_id=?))
		AND (original.id IS NULL OR original.state NOT IN ('SUCCEEDED','REJECTED') OR
		 ((o.kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE','PULL_MAIN','DISCARD_DRAFT','REVERSE','CONFLICT_RESOLUTION')
		 OR (o.kind IN ('SUBMISSION','MERGE') AND o.state='SUCCEEDED')) AND (installed.id IS NULL OR installed.state<>'SUCCEEDED')))
		)`, document, document, path, document).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// 真实业务记录和控制占位仍未释放时，即使某条任务误终结/丢失也不能发证。
	// BRANCH_CONTROL_GATE本身一直是SUCCEEDED，必须看其payload中的原操作号。
	err = tx.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM submission_operation WHERE document_id=? AND state NOT IN ('SUCCEEDED','REJECTED'))
		OR EXISTS(SELECT 1 FROM merge_operation m JOIN review_request r ON r.id=m.request_id
		 WHERE r.document_id=? AND m.state NOT IN ('SUCCEEDED','REJECTED','CANCELLED'))
		OR EXISTS(SELECT 1 FROM main_commit_barrier WHERE document_id=? AND (phase<>'OPEN' OR operation_id IS NOT NULL))
		OR EXISTS(SELECT 1 FROM branch WHERE document_id=? AND lifecycle='RESTORING')
		OR EXISTS(SELECT 1 FROM durable_task t JOIN branch b ON t.business_key=CAST(b.id AS CHAR)
		 WHERE t.kind='BRANCH_CONTROL_GATE' AND b.document_id=?
		 AND (t.state<>'SUCCEEDED' OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.operation_id')),'')<>''))`,
		document, document, document, document, document).Scan(&pending)
	return pending, err
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

func AccountHistoryCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var operation, state string
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_HISTORY_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if operation != request.Operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.AccountDocumentCleanupRequest != request {
		return result, false, errors.New("ACCOUNT_HISTORY_CLEANUP_BINDING_CHANGED")
	}
	return result, true, nil
}

// 只改可变指针，历史表只DELETE。关闭来源与原安装终态已证明；仍有未知输出
// 或导出时整个事务回滚，不能为了绕外键关掉FOREIGN_KEY_CHECKS。
func RemoveAccountHistoryDependents(ctx context.Context, tx *sql.Tx, document uint64) error {
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state<>'SETTLED')
	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN ('QUEUED','RUNNING'))
	 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state<>'RELEASED')
	 OR EXISTS(SELECT 1 FROM durable_task WHERE kind IN ('COMMENT_NOTIFICATION','REVIEW_NOTIFICATION','REVIEW_ACTION_NOTIFICATION','MERGE_NOTIFICATION')
	 AND state NOT IN ('SUCCEEDED','REJECTED') AND
	 (JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document_id'))=? OR JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document'))=?))`, document, document, document, domain.TextID(document), domain.TextID(document)).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "ACCOUNT_HISTORY_DEPENDENT_PENDING")
	}
	statements := []string{
		"UPDATE collaboration_unit SET active_branch_id=NULL WHERE document_id=?",
		"UPDATE branch SET current_lineage_id=NULL WHERE document_id=?",
		"UPDATE review_request SET current_round_id=NULL,active_merge_operation_id=NULL WHERE document_id=?",
		"UPDATE submission_operation SET expected_round_id=NULL,result_round_id=NULL WHERE document_id=?",
		"UPDATE merge_operation m JOIN review_request r ON r.id=m.request_id SET m.result_merge_id=NULL WHERE r.document_id=?",
		"UPDATE merge_record m JOIN review_request r ON r.id=m.request_id SET m.approval_action_id=NULL WHERE r.document_id=?",
		"DELETE g FROM unit_review_guard g JOIN collaboration_unit u ON u.id=g.unit_id WHERE u.document_id=?",
		// 先拆可变resolved_action指针，删反向引用change的解析动作；创建动作本来
		// 不带target_change_id。随后删change与剩余动作，无须UPDATE不可变review_action。
		"UPDATE change_request c JOIN review_request r ON r.id=c.request_id SET c.status='OPEN',c.resolved_action_id=NULL WHERE r.document_id=?",
		"DELETE a FROM review_action a JOIN review_request r ON r.id=a.request_id WHERE r.document_id=? AND a.target_change_id IS NOT NULL",
		"DELETE c FROM change_request c JOIN review_request r ON r.id=c.request_id WHERE r.document_id=?",
		"DELETE a FROM review_action a JOIN review_request r ON r.id=a.request_id WHERE r.document_id=?",
		"DELETE m FROM merge_record m JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE a FROM merge_attempt a JOIN merge_operation m ON m.operation_id=a.operation_id JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE m FROM merge_operation m JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE s FROM request_submitter s JOIN review_request r ON r.id=s.request_id WHERE r.document_id=?",
		"DELETE FROM review_round WHERE document_id=?",
		"DELETE FROM submission_operation WHERE document_id=?",
		"DELETE FROM review_request WHERE document_id=?",
		"UPDATE comment c JOIN comment_thread t ON t.id=c.thread_id SET c.reply_to_id=NULL WHERE t.document_id=?",
		"DELETE c FROM comment c JOIN comment_thread t ON t.id=c.thread_id WHERE t.document_id=?",
		"DELETE FROM comment_thread WHERE document_id=?",
		"DELETE FROM read_view WHERE document_id=?",
		"DELETE FROM delivery_session WHERE document_id=?",
		"DELETE t FROM collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id WHERE a.document_id=?",
		"DELETE s FROM admission_source s JOIN document_admission a ON a.admission_id=s.admission_id WHERE a.document_id=?",
		"DELETE FROM document_admission WHERE document_id=?",
		"DELETE FROM export_job WHERE document_id=?",
		"DELETE FROM main_commit_barrier WHERE document_id=?",
		"DELETE FROM unit_lineage WHERE source_document_id=?",
		"DELETE FROM document_application WHERE document_id=?",
		"DELETE FROM document_invitation WHERE document_id=?",
		"DELETE FROM content_grant WHERE document_id=?",
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, document); err != nil {
			return err
		}
	}
	// previous_id保留原事实；真正从叶节点释放，而不是靠自增ID猜插入顺序。
	if err := deleteHistoryLeaves(ctx, tx, `DELETE l FROM branch_lineage l LEFT JOIN branch_lineage c ON c.previous_id=l.id
	 WHERE l.document_id=? AND c.id IS NULL`, "SELECT COUNT(*) FROM branch_lineage WHERE document_id=?", document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM branch WHERE document_id=?", document)
	return err
}

func deleteHistoryLeaves(ctx context.Context, tx *sql.Tx, statement, remaining string, args ...any) error {
	for {
		var count uint64
		if err := tx.QueryRowContext(ctx, remaining, args...).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		result, err := tx.ExecContext(ctx, statement, args...)
		if err != nil {
			return err
		}
		removed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if removed == 0 {
			return errors.New("ACCOUNT_HISTORY_DEPENDENCY_CYCLE")
		}
	}
}

func RemoveAccountHistoryGraph(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, g AccountHistoryGraph) error {
	allocationResources, err := LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)
	if err != nil {
		return err
	}
	// 标题必须跟保留release一致，不能把关闭前私有MAIN标题留在公开metadata。
	if item.Release != 0 {
		var manifest uint64
		var title string
		if err := tx.QueryRowContext(ctx, "SELECT manifest_id,title FROM `release` WHERE id=? AND document_id=?", item.Release, item.Document).Scan(&manifest, &title); err != nil {
			return err
		}
		if manifest != item.Manifest {
			return errors.New("ACCOUNT_HISTORY_RELEASE_CHANGED")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE document SET title=?,responsible_user_id=NULL,current_structure_id=?,current_style_id=?,
		 current_manifest_id=?,current_release_id=?,publication_state='PUBLISHED' WHERE id=?`, title, item.Structure, item.Style, item.Manifest, item.Release, item.Document); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE document SET title='已清理文档',responsible_user_id=NULL,current_structure_id=NULL,
		 current_style_id=NULL,current_manifest_id=NULL,current_release_id=NULL,publication_state='UNPUBLISHED' WHERE id=?`, item.Document); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM `release` WHERE document_id=? AND id<>?", item.Document, item.Release); err != nil {
		return err
	}
	for _, manifest := range g.Manifests {
		if manifest.ID == item.Manifest {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM manifest_entry WHERE manifest_id=? AND document_id=?", manifest.ID, item.Document); err != nil {
			return err
		}
	}
	// 非保留版本的pin与正文同事务移除，保留版本pin及RETENTION独立保留。
	for _, version := range g.Versions {
		if g.KeepVersions[version] {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=?", item.Document, domain.TextID(version)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM content_version WHERE id=? AND document_id=?", version, item.Document); err != nil {
			return err
		}
	}
	order, err := domain.HistoryDeletionOrder(g.Manifests, g.KeepManifests)
	if err != nil {
		return err
	}
	for _, id := range order {
		if _, err = tx.ExecContext(ctx, "DELETE FROM main_manifest WHERE id=? AND document_id=?", id, item.Document); err != nil {
			return err
		}
	}
	if err = deleteHistoryLeaves(ctx, tx, `DELETE e FROM structure_entry e LEFT JOIN structure_entry c ON c.structure_id=e.structure_id AND c.parent_node_id=e.node_id
	 WHERE e.document_id=? AND e.structure_id<>? AND c.node_id IS NULL`, "SELECT COUNT(*) FROM structure_entry WHERE document_id=? AND structure_id<>?", item.Document, item.Structure); err != nil {
		return err
	}
	for _, id := range g.Units {
		if !g.KeepUnits[id] {
			if _, err = tx.ExecContext(ctx, "DELETE FROM collaboration_unit WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	for _, id := range g.Chapters {
		if !g.KeepChapters[id] {
			if _, err = tx.ExecContext(ctx, "DELETE FROM chapter WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	for _, family := range []struct {
		table string
		rows  []domain.AccountHistoryRevision
		keep  map[uint64]bool
	}{{"structure_revision", g.Structures, g.KeepStructures}, {"style_revision", g.Styles, g.KeepStyles}} {
		order, err := domain.HistoryDeletionOrder(family.rows, family.keep)
		if err != nil {
			return err
		}
		// table只有上面的两个编译期常量；编号全部参数绑定。
		for _, id := range order {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+family.table+" WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	// 上传原号及原document绑定属于资源恢复协议，保留关闭的文档metadata；其他历史资源pin只保留实际
	// 存在的固定VERSION/CONTENT及当前正式图片RETENTION。外文档共享引用不受影响。
	if _, err = tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND owner_kind NOT IN ('VERSION','RETENTION')
	 AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')`, item.Document); err != nil {
		return err
	}
	if err = ReconcileCleanedDocumentCapacity(ctx, tx, item.Document); err != nil {
		return err
	}
	return ReconcileCleanedResourceAllocations(ctx, tx, item.Document, item.Space, allocationResources)
}

// account_history_graph.go：保留原实现，按业务合并文件。

type AccountHistoryGraph struct {
	Manifests, Structures, Styles []domain.AccountHistoryRevision
	Units, Chapters, Versions     []uint64
	KeepManifests                 map[uint64]bool
	KeepStructures                map[uint64]bool
	KeepStyles                    map[uint64]bool
	KeepUnits                     map[uint64]bool
	KeepChapters                  map[uint64]bool
	KeepVersions                  map[uint64]bool
}

func historyRevisions(ctx context.Context, tx *sql.Tx, statement string, document uint64) ([]domain.AccountHistoryRevision, error) {
	rows, err := tx.QueryContext(ctx, statement, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.AccountHistoryRevision{}
	for rows.Next() {
		var row domain.AccountHistoryRevision
		if err = rows.Scan(&row.ID, &row.Parent, &row.Structure, &row.Style); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// 文档已封门并持锁。版本通过unit前导索引读取，不按无索引document_id扫描全文表。
func AccountHistoryGraphRow(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument) (AccountHistoryGraph, error) {
	g := AccountHistoryGraph{
		KeepManifests:  map[uint64]bool{},
		KeepStructures: map[uint64]bool{},
		KeepStyles:     map[uint64]bool{},
		KeepUnits:      map[uint64]bool{},
		KeepChapters:   map[uint64]bool{},
		KeepVersions:   map[uint64]bool{},
	}
	var err error
	g.Manifests, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),structure_id,style_id FROM main_manifest WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	g.Structures, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),0,0 FROM structure_revision WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	g.Styles, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),0,0 FROM style_revision WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	if item.Release != 0 {
		g.KeepManifests[item.Manifest], g.KeepStructures[item.Structure], g.KeepStyles[item.Style] = true, true, true
		for _, id := range item.Versions {
			g.KeepVersions[id] = true
		}
		rows, err := tx.QueryContext(ctx, "SELECT chapter_id,COALESCE(unit_id,0) FROM structure_entry WHERE structure_id=? AND document_id=?", item.Structure, item.Document)
		if err != nil {
			return g, err
		}
		for rows.Next() {
			var chapter, unit uint64
			if err = rows.Scan(&chapter, &unit); err != nil {
				break
			}
			g.KeepChapters[chapter] = true
			if unit != 0 {
				g.KeepUnits[unit] = true
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return g, err
		}
	}
	if err = domain.KeepHistoryAncestors(g.Manifests, g.KeepManifests); err != nil {
		return g, err
	}
	for _, manifest := range g.Manifests {
		if g.KeepManifests[manifest.ID] {
			g.KeepStructures[manifest.Structure], g.KeepStyles[manifest.Style] = true, true
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,chapter_id,created_structure_id,COALESCE(retired_structure_id,0) FROM collaboration_unit WHERE document_id=? ORDER BY id", item.Document)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var id, chapter, created, retired uint64
		if err = rows.Scan(&id, &chapter, &created, &retired); err != nil {
			break
		}
		g.Units = append(g.Units, id)
		if g.KeepUnits[id] {
			g.KeepChapters[chapter], g.KeepStructures[created] = true, true
			if retired != 0 {
				g.KeepStructures[retired] = true
			}
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return g, err
	}
	for _, unit := range g.Units {
		versions, err := tx.QueryContext(ctx, "SELECT id,structure_id FROM content_version WHERE unit_id=? AND document_id=? ORDER BY id", unit, item.Document)
		if err != nil {
			return g, err
		}
		for versions.Next() {
			var id, structure uint64
			if err = versions.Scan(&id, &structure); err != nil {
				break
			}
			g.Versions = append(g.Versions, id)
			if g.KeepVersions[id] {
				if !g.KeepUnits[unit] || structure != item.Structure {
					err = errors.New("ACCOUNT_HISTORY_FIXED_VERSION_CHANGED")
					break
				}
				g.KeepStructures[structure] = true
			}
		}
		if err == nil {
			err = versions.Err()
		}
		versions.Close()
		if err != nil {
			return g, err
		}
	}
	foundVersions := map[uint64]bool{}
	for _, id := range g.Versions {
		foundVersions[id] = true
	}
	for _, id := range item.Versions {
		if !foundVersions[id] {
			return g, errors.New("ACCOUNT_HISTORY_FIXED_VERSION_MISSING")
		}
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,created_structure_id,COALESCE(retired_structure_id,0) FROM chapter WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var id, created, retired uint64
		if err = rows.Scan(&id, &created, &retired); err != nil {
			break
		}
		g.Chapters = append(g.Chapters, id)
		if g.KeepChapters[id] {
			g.KeepStructures[created] = true
			if retired != 0 {
				g.KeepStructures[retired] = true
			}
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return g, err
	}
	if err = domain.KeepHistoryAncestors(g.Structures, g.KeepStructures); err != nil {
		return g, err
	}
	err = domain.KeepHistoryAncestors(g.Styles, g.KeepStyles)
	return g, err
}

// account_relations.go：保留原实现，按业务合并文件。

// 身份来源已经封闭，候选只能收缩；仍在全部来源与业务锁下重新发现，
// 不把最初的普通SELECT当作关闭当前授权或清空负责人关系的依据。
func DiscoverAccountRelations(ctx context.Context, q Queryer, user uint64) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	rows, err := q.QueryContext(ctx, `SELECT k.id,k.space_id,k.acl_revision FROM knowledge_base k WHERE k.id IN (
		SELECT knowledge_base_id FROM knowledge_base_member WHERE user_id=? AND state<>'CLOSED'
		UNION SELECT knowledge_base_id FROM knowledge_base_creation_grant WHERE user_id=? AND state<>'CLOSED') ORDER BY k.id`, user, user)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, space, epoch uint64
		if err = rows.Scan(&id, &space, &epoch); err != nil {
			break
		}
		c.KnowledgeBases = append(c.KnowledgeBases, id)
		c.Spaces = append(c.Spaces, space)
		c.Sources = append(c.Sources, domain.LeaseSource{Key: "kb-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	rows, err = q.QueryContext(ctx, `SELECT id,space_id,COALESCE(knowledge_base_id,0),acl_revision FROM document WHERE id IN (
		SELECT id FROM document WHERE responsible_user_id=?
		UNION SELECT document_id FROM content_grant WHERE user_id=? AND state<>'CLOSED') ORDER BY id`, user, user)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, space, kb, epoch uint64
		if err = rows.Scan(&id, &space, &kb, &epoch); err != nil {
			break
		}
		c.Documents = append(c.Documents, id)
		c.Spaces = append(c.Spaces, space)
		if kb != 0 && !slices.Contains(c.KnowledgeBases, kb) {
			c.KnowledgeBases = append(c.KnowledgeBases, kb)
		}
		c.Sources = append(c.Sources, domain.LeaseSource{Key: "doc-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	// 负责人文档所在KB也必须持相同来源锁；它未必在用户的KB成员集合中。
	for _, kb := range c.KnowledgeBases {
		key := "kb-acl:" + domain.TextID(kb)
		if slices.ContainsFunc(c.Sources, func(s domain.LeaseSource) bool { return s.Key == key }) {
			continue
		}
		var space, epoch uint64
		if err = q.QueryRowContext(ctx, "SELECT space_id,acl_revision FROM knowledge_base WHERE id=?", kb).Scan(&space, &epoch); err != nil {
			return c, err
		}
		c.Spaces = append(c.Spaces, space)
		c.Sources = append(c.Sources, domain.LeaseSource{Key: key, Epoch: domain.TextID(epoch)})
	}
	slices.Sort(c.Spaces)
	c.Spaces = slices.Compact(c.Spaces)
	slices.Sort(c.KnowledgeBases)
	slices.SortFunc(c.Sources, func(a, b domain.LeaseSource) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return c, nil
}

func ApplyAccountRelations(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, c AccountRetentionCandidates) error {
	binding := plan.Binding
	for _, kb := range plan.ReservedKnowledgeBases {
		var operation, state string
		if err := tx.QueryRowContext(ctx, "SELECT operation_id,state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", kb, binding.User).Scan(&operation, &state); err != nil {
			return err
		}
		if operation != binding.Operation || state != "RESERVED" {
			return errors.New("ACCOUNT_RETIREMENT_RESERVATION_CHANGED")
		}
		available, err := AvailableKBAdministrators(ctx, tx, kb)
		if err != nil {
			return err
		}
		if available < 1 {
			return errors.New("LAST_KB_ADMIN_REQUIRED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE admin_retirement SET state='APPLIED' WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state='RESERVED'", kb, binding.User, binding.Operation); err != nil {
			return err
		}
	}
	for _, kb := range c.KnowledgeBases {
		var organization uint64
		var kind, kbState, spaceState string
		if err := tx.QueryRowContext(ctx, `SELECT s.kind,COALESCE(s.organization_id,0),k.lifecycle,s.lifecycle FROM knowledge_base k JOIN space s ON s.id=k.space_id WHERE k.id=?`, kb).Scan(&kind, &organization, &kbState, &spaceState); err != nil {
			return err
		}
		if kind == "ORGANIZATION" && !slices.Contains(binding.ClosingOrganizations, organization) && kbState == "ACTIVE" && spaceState == "ACTIVE" {
			// 原Account身份关闭证明已验证；这里只复用管理员快照/可靠通知登记。
			// 不伪造离组门禁，也不调用普通Departure.Close绕过管理员退休检查。
			if err := queueKnowledgeBaseDepartures(ctx, tx, domain.DepartureBinding{ID: binding.Operation, Org: organization, User: binding.User, Epoch: binding.IdentityEpoch}, kb); err != nil {
				return err
			}
		}
		for _, statement := range []string{
			"UPDATE knowledge_base_member SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND user_id=? AND state<>'CLOSED'",
			"UPDATE knowledge_base_creation_grant SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND user_id=? AND state<>'CLOSED'",
		} {
			if _, err := tx.ExecContext(ctx, statement, kb, binding.User); err != nil {
				return err
			}
		}
	}
	for _, id := range c.Documents {
		if _, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND user_id=? AND state<>'CLOSED'", id, binding.User); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE document SET responsible_user_id=NULL,control_revision=control_revision+1 WHERE id=? AND responsible_user_id=?", id, binding.User); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING',generation=generation+1 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PLANNED'", binding.Operation)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("ACCOUNT_RETENTION_STATE_CHANGED")
	}
	return nil
}

// account_resource_cleanup.go：保留原实现，按业务合并文件。

func AccountResourceCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var context platform.AccountDocumentCleanupContext
	var raw []byte
	var operation, state string
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_RESOURCE_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return context, false, nil
	}
	if err != nil {
		return context, false, err
	}
	if json.Unmarshal(raw, &context) != nil || !context.Valid() || context.AccountDocumentCleanupRequest != request || operation != request.Operation || state != "SUCCEEDED" {
		return context, false, errors.New("ACCOUNT_RESOURCE_CLEANUP_BINDING_CHANGED")
	}
	return context, true, nil
}

// Doc→export→protection→resource。来源已关闭，Finalize/Protect不能越过原来源锁；
// GC持资源行锁并重查引用。先建立留存pin，再释放草稿pin，始终不存在无引用窗口。
func ReleaseAccountDraftResources(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error {
	if err := ReleaseDocumentDraftResourcePins(ctx, tx, item, authority); err != nil {
		return err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_RESOURCE_CLEANUP',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))`, authority.Document, authority.Operation, string(raw))
	return err
}

// 低层只操作原事务的引用与保护，不插入任何业务清理回执。Account/PURGE分别
// 核验自己的固定来源及Node CLEANED，再使用各自原号登记回执，避免复用doc键。
func ReleaseDocumentDraftResourcePins(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error {
	if _, err := LockDocumentResourceAllocationCleanup(ctx, tx, item.Document); err != nil {
		return err
	}
	var limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", item.Space).Scan(&limit); err != nil {
		return err
	}
	q, err := allocations.LockQuota(ctx, tx, item.Space, limit)
	if err != nil {
		return err
	}
	if err = allocations.ReleaseCleanedDraftBodies(ctx, tx, q, item.Document); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,state FROM export_job WHERE document_id=? ORDER BY id FOR UPDATE", item.Document)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uint64
		var state string
		if err = rows.Scan(&id, &state); err != nil {
			break
		}
		if state == "QUEUED" || state == "RUNNING" {
			err = domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_EXPORT_PENDING")
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, "SELECT operation_id,state FROM resource_protection WHERE document_id=? ORDER BY operation_id FOR UPDATE", item.Document)
	if err != nil {
		return err
	}
	for rows.Next() {
		var operation, state string
		if err = rows.Scan(&operation, &state); err != nil {
			break
		}
		if state == "UNKNOWN" {
			err = domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_RESOURCE_UNKNOWN")
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range item.Resources {
		_, state, err := LockedResourceSpace(ctx, tx, id)
		if err != nil {
			return err
		}
		allowed, err := allocations.ResourceAllowedForDocument(ctx, tx, id, item.Document, item.Space)
		if err != nil {
			return err
		}
		if !allowed || state != "READY" {
			return errors.New("ACCOUNT_RETENTION_RESOURCE_CHANGED")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation)
			VALUES(?,?,'RETENTION',?,'PUBLIC_RELEASE',1)`, id, item.Document, authority.Document+":"+authority.Operation)
		if err != nil {
			return err
		}
	}
	// Node原CLEANED回执证明这些房间和原安装均已关闭、私有正文已移除。
	// UNKNOWN不由TTL推成终态。VERSION/MANIFEST/RELEASE/EXPORT引用仍随各自历史清理，
	// 此阶段不能因草稿已清就让仍存在的版本图片被物理删除。
	if _, err = tx.ExecContext(ctx, "UPDATE resource_protection SET state='RELEASED' WHERE document_id=? AND state IN ('PROTECTED','CONFIRMED')", item.Document); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND
		(owner_kind IN ('DRAFT','CAPTURE') OR (owner_kind='OPERATION' AND purpose='DRAFT_UPDATE'))`, item.Document); err != nil {
		return err
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`195933ea5de2c3101b1682700401fae4667d7c57115772d916eaefa6669afdfa`。[参考实现](../../../../services/document/internal/repository/document/account_cleanup_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
