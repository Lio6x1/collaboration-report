// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

// DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
// 的Account冻结，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。
func (s *AccountRetentions) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	result, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	// 返回给Node之前先结束只读事务，SQL错误不能被成功HTTP掩盖。
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}

// 可被后续资源/历史清理复用；调用者拥有事务，不在此处提交或跨服务通信。
func (s *AccountRetentions) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	document, err := domain.CanonicalID(request.Document)
	if err != nil {
		return empty, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil {
		return empty, err
	}
	if plan.Binding.Operation != request.Operation {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	plan, err = boundRetentionPlan(original, plan.Binding)
	if err != nil || original.State != "CLEANING" {
		if err != nil {
			return empty, err
		}
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANING_REQUIRED")
	}
	item, scopes, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	sources := make([]domain.LeaseSource, 0, len(scopes))
	for _, scope := range scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	if err = lockAccountRetentionSources(ctx, tx, sources, plan.Binding); err != nil {
		return empty, err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	current, err := boundRetentionPlan(locked, plan.Binding)
	if err != nil {
		return empty, err
	}
	if locked.State != "CLEANING" || string(locked.Plan) != string(original.Plan) {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	// 只核验这个文档的doc-acl/publication/父KB屏障，其他文档的未知操作不能
	// 使本次单文档资格变成全局排队。每个屏障仍验证原任务、gate及双方真实ACK摘要。
	retention := *current.Retention
	retention.Scopes = scopes
	current.Retention = &retention
	drained, err := repository.AccountRetentionDrained(ctx, tx, current)
	if err != nil {
		return empty, err
	}
	if !drained {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_DRAIN_PENDING")
	}
	if err = repository.LockAccountCleanupDocument(ctx, tx, current, item, scopes); err != nil {
		return empty, err
	}
	pending, err := repository.AccountDocumentWritesPending(ctx, tx, document)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_WRITES_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document {
			result := platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)
			if !result.Valid() {
				return empty, errors.New("ACCOUNT_DOCUMENT_CONTEXT_INVALID")
			}
			return result, nil
		}
	}
	return empty, errors.New("ACCOUNT_DOCUMENT_SCOPE_MISSING")
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

// 此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
// 原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。
func (s *AccountRetentions) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.ReleaseDraftResources(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_HISTORY_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil {
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return empty, err
	}
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, document); err != nil {
		return empty, err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_HISTORY_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// account_payload_cleanup.go：保留原实现，按业务合并文件。

// 原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
// 原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。
func (s *AccountRetentions) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.CleanHistory(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	if err = repository.RemoveAccountPayloads(ctx, tx, authority); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_PAYLOAD_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// account_relations.go：保留原实现，按业务合并文件。

// ApplyRelations只关闭这个已冻结用户的关系，存续文档的正文、主线、审核、正式
// 历史全部保留。CLEANING表示关系阶段已提交，仍不表示Node/正文/引用清理完成。
func (s *AccountRetentions) ApplyRelations(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.authority == nil {
		return errors.New("ACCOUNT_RETENTION_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return err
	}
	if !confirmed {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_FREEZE_UNCONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	original, err := repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return err
	}
	if plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	if original.State == "CLEANING" {
		return nil
	}
	if original.State != "PLANNED" || plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	candidates, err := repository.DiscoverAccountRelations(ctx, tx, binding.User)
	if err != nil {
		return err
	}
	// 包含固定保留来源，确保核验旧排空时其他调用不能更换gate代次或原任务绑定。
	sources := slices.Clone(candidates.Sources)
	for _, scope := range plan.Retention.Scopes {
		if !slices.ContainsFunc(sources, func(s domain.LeaseSource) bool { return s.Key == scope.Key }) {
			sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
		}
	}
	if err = lockAccountRetentionSources(ctx, tx, sources, binding); err != nil {
		return err
	}
	original, err = repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	plan, err = boundRetentionPlan(original, binding)
	if err != nil {
		return err
	}
	if plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	if original.State == "CLEANING" {
		return nil
	}
	if original.State != "PLANNED" || plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	drained, err := repository.AccountRetentionDrained(ctx, tx, plan)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_DRAIN_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	current, err := repository.DiscoverAccountRelations(ctx, tx, binding.User)
	if err != nil {
		return err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RELATIONS_CHANGED")
	}
	if err = repository.ApplyAccountRelations(ctx, tx, plan, current); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RELATIONS_COMMIT_UNKNOWN")
	}
	return nil
}

// account_resource_cleanup.go：保留原实现，按业务合并文件。

// ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
// 接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。
func (s *AccountRetentions) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() || s.cleaner == nil {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANER_REQUIRED")
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.DocumentCleanupContext(ctx, request)
	if err != nil {
		return empty, err
	}
	if err = s.cleaner.Clean(ctx, expected); err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil {
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	if err = repository.ReleaseAccountDraftResources(ctx, tx, item, authority); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}
