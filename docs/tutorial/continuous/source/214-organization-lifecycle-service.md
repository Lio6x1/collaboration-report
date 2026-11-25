# services/document/internal/logic/document/organization_lifecycle_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/214-organization-lifecycle-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`OrganizationCloseAuthority`、`OrganizationClosures`、`NewOrganizationClosures`、`retainedDocument`、`retainedDependencies`、`HandlesCleanup`、`boundOrganizationClosePlan`、`Plan`、`State`、`lockOrganizationCloseSources`、`ensureCleaning`、`cleanupExpected`、`DocumentCleanupContext`、`lockedDocumentCleanupContext`、`Complete`、`lockOrganizationCompletionSources`、`validateRetentionCompletion`、`CleanHistory`、`CleanPayloads`、`Progress`、`Ready`、`ReleaseDraftResources`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
)

// organization_close.go：保留原实现，按业务合并文件。

type OrganizationCloseAuthority interface {
	Confirmed(context.Context, platform.OrganizationClosureBinding) (bool, error)
}

// 仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。
type OrganizationClosures struct {
	db           *sql.DB
	authority    OrganizationCloseAuthority
	validator    *codec.Validator
	cleaner      AccountDocumentCleaner
	dependencies *AccountRetentions
}

func NewOrganizationClosures(db *sql.DB, authority OrganizationCloseAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *OrganizationClosures {
	return &OrganizationClosures{db: db, authority: authority, validator: validator, cleaner: cleaner, dependencies: NewAccountRetentions(db, nil, validator, nil)}
}

// 组合复用纯依赖解析，避免继承账号注销的权限入口或身份来源检查。
func (s *OrganizationClosures) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error) {
	return s.dependencies.retainedDocument(ctx, tx, id)
}
func (s *OrganizationClosures) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error) {
	return s.dependencies.retainedDependencies(ctx, tx, document, item)
}

func (s *OrganizationClosures) HandlesCleanup(ctx context.Context, operation string) (bool, error) {
	kind, err := repository.LifecycleKind(ctx, s.db, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return kind == "ORGANIZATION_CLOSE", err
}

func boundOrganizationClosePlan(record repository.AccountRetirementRecord, b platform.OrganizationClosureBinding) (domain.OrganizationClosePlan, error) {
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil || record.Kind != "ORGANIZATION_CLOSE" || record.Actor != b.User || record.Epoch != b.Epoch || plan.Binding != b {
		return plan, errors.New("ORGANIZATION_CLOSE_BINDING_CHANGED")
	}
	if record.State != "PLANNED" && record.State != "CLEANING" && record.State != "SUCCEEDED" {
		return plan, errors.New("ORGANIZATION_CLOSE_STATE_INVALID")
	}
	return plan, nil
}

func (s *OrganizationClosures) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error {
	if !b.Valid() || s.authority == nil || s.validator == nil {
		return errors.New("ORGANIZATION_CLOSE_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, b)
	if err != nil {
		return err
	}
	if !confirmed {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_SOURCE_UNCONFIRMED")
	}
	// Account不可逆组织关闭证明先取得；SQL事务不等待任何跨服务调用。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	candidates, err := repository.DiscoverOrganizationClose(ctx, tx, b)
	if err != nil {
		// 已规划重放时父空间已经CLOSING；先尝试读取同号，绝不重新选计划。
		record, readErr := repository.AccountRetirementSnapshot(ctx, tx, b.Operation)
		if readErr != nil {
			return err
		}
		_, readErr = boundOrganizationClosePlan(record, b)
		return readErr
	}
	if err = lockOrganizationCloseSources(ctx, tx, candidates.Sources, b); err != nil {
		return err
	}
	original, err := repository.LockedAccountRetirement(ctx, tx, b.Operation)
	if err == nil {
		_, err = boundOrganizationClosePlan(original, b)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	keys := make([]string, 0, len(candidates.Sources))
	for _, source := range candidates.Sources {
		keys = append(keys, source.Key)
	}
	pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, keys)
	if err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	current, err := repository.DiscoverOrganizationClose(ctx, tx, b)
	if err != nil {
		return err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_CANDIDATES_CHANGED")
	}
	retention := domain.AccountRetentionPlan{Spaces: current.Spaces, KnowledgeBases: current.KnowledgeBases, Documents: []domain.AccountRetentionDocument{}, Scopes: []domain.AccountRetentionScope{}}
	for _, id := range current.Documents {
		item, err := s.retainedDocument(ctx, tx, id)
		if err != nil {
			return err
		}
		retention.Documents = append(retention.Documents, item)
	}
	for _, source := range current.Sources {
		epoch, err := domain.CanonicalID(source.Epoch)
		if err != nil || epoch == math.MaxUint64 {
			return errors.New("ORGANIZATION_CLOSE_EPOCH_EXHAUSTED")
		}
		retention.Scopes = append(retention.Scopes, domain.AccountRetentionScope{Key: source.Key, Epoch: epoch + 1, Operation: domain.AccountRetentionRevocation(b.Operation, source.Key)})
	}
	if !retention.Valid(b.Operation) {
		return errors.New("ORGANIZATION_CLOSE_RETENTION_INVALID")
	}
	plan := domain.OrganizationClosePlan{Binding: b, Retention: &retention}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.RecordOrganizationClose(ctx, tx, b, raw); err != nil {
		return err
	}
	if err = repository.SaveOrganizationClose(ctx, tx, b.Operation, raw, retention); err != nil {
		return err
	}
	for _, scope := range retention.Scopes {
		if err = QueueClosure(ctx, tx, scope.Operation, b.User, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Action: "ORGANIZATION_RETENTION"}); err != nil {
			return err
		}
	}
	if err = queueVisibilityIndexEvents(ctx, tx, b.Operation, current.Documents, 0); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_PLAN_COMMIT_UNKNOWN")
	}
	return nil
}

func (s *OrganizationClosures) State(ctx context.Context, b platform.OrganizationClosureBinding) (string, error) {
	record, err := repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)
	if err != nil {
		return "", err
	}
	if _, err = boundOrganizationClosePlan(record, b); err != nil {
		return "", err
	}
	return record.State, nil
}

// 来源按字符串排序统一锁定，原组织来源及所有Doc/KB来源先于业务对象。
// 本人identity完全不参与关闭，个人空间的来源继续可用。
func lockOrganizationCloseSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, b platform.OrganizationClosureBinding) error {
	all := slices.Clone(sources)
	all = append(all, domain.LeaseSource{Key: b.Scope(), Epoch: domain.TextID(b.Epoch)})
	slices.SortFunc(all, func(a, c domain.LeaseSource) int { return strings.Compare(a.Key, c.Key) })
	for start := 0; start < len(all); start += 100 {
		page := all[start:min(start+100, len(all))]
		gates, err := repository.LockSourceGatesBatch(ctx, tx, page)
		if err != nil {
			return err
		}
		for _, source := range page {
			epoch, err := domain.CanonicalID(source.Epoch)
			if err != nil {
				return err
			}
			if source.Key != b.Scope() && gates[source.Key].Epoch > epoch {
				return errors.New("ORGANIZATION_CLOSE_SOURCE_CHANGED")
			}
		}
	}
	matches, err := repository.OrganizationCloseGateMatches(ctx, tx, b)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ORGANIZATION_CLOSE_SOURCE_CHANGED")
	}
	return nil
}

func (s *OrganizationClosures) ensureCleaning(ctx context.Context, b platform.OrganizationClosureBinding) error {
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)
	if err != nil {
		return err
	}
	plan, err := boundOrganizationClosePlan(original, b)
	if err != nil {
		return err
	}
	if original.State == "CLEANING" || original.State == "SUCCEEDED" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sources := []domain.LeaseSource{}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	if err = lockOrganizationCloseSources(ctx, tx, sources, b); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, b.Operation)
	if err != nil {
		return err
	}
	current, err := boundOrganizationClosePlan(locked, b)
	if err != nil {
		return err
	}
	if string(original.Plan) != string(locked.Plan) {
		return errors.New("ORGANIZATION_CLOSE_PLAN_CHANGED")
	}
	if locked.State == "CLEANING" || locked.State == "SUCCEEDED" {
		return nil
	}
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_DRAIN_PENDING")
	}
	if err = repository.StartOrganizationCleaning(ctx, tx, b.Operation); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_CLEANING_UNKNOWN")
	}
	return nil
}

// organization_close_cleanup_context.go：保留原实现，按业务合并文件。

// 原资源/历史回执允许终态重放，但先绑定本流程固定计划，不能把旧账号清理回执
// 从组织端点返回，也不能因节点旧回执存在而跳过原文档/代次核验。
func (s *OrganizationClosures) cleanupExpected(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ORGANIZATION_DOCUMENT_REQUEST_INVALID")
	}
	record, err := repository.AccountRetirementSnapshot(ctx, s.db, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
		return empty, err
	}
	if _, err = boundOrganizationClosePlan(record, plan.Binding); err != nil {
		return empty, err
	}
	if record.State != "CLEANING" && record.State != "SUCCEEDED" {
		return empty, errors.New("ORGANIZATION_DOCUMENT_CLEANING_REQUIRED")
	}
	document, _ := domain.CanonicalID(request.Document)
	_, scopes, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document {
			return platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation), nil
		}
	}
	return empty, errors.New("ORGANIZATION_DOCUMENT_SCOPE_MISSING")
}

// DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
// 的Account组织来源关闭，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。
func (s *OrganizationClosures) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
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
func (s *OrganizationClosures) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	document, err := domain.CanonicalID(request.Document)
	if err != nil {
		return empty, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(original.Plan)
	if err != nil {
		return empty, err
	}
	if plan.Binding.Operation != request.Operation {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	plan, err = boundOrganizationClosePlan(original, plan.Binding)
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
	if err = lockOrganizationCloseSources(ctx, tx, sources, plan.Binding); err != nil {
		return empty, err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	current, err := boundOrganizationClosePlan(locked, plan.Binding)
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
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
		return empty, err
	}
	if !drained {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_DRAIN_PENDING")
	}
	if err = repository.LockOrganizationCleanupDocument(ctx, tx, current, item, scopes); err != nil {
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

// organization_close_complete.go：保留原实现，按业务合并文件。

// Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
// 循环同步清理整个组织。Account恢复器应逐文档推进后调用本关口，原号重放。
// Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须结束组织原操作的事务。
func (s *OrganizationClosures) Complete(ctx context.Context, binding platform.OrganizationClosureBinding) error {
	if !binding.Valid() || s.validator == nil {
		return errors.New("ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundOrganizationClosePlan(original, binding)
	if err != nil {
		return err
	}
	if original.State == "SUCCEEDED" {
		return nil
	}
	if original.State != "CLEANING" || plan.Retention == nil {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_CLEANING_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockOrganizationCompletionSources(ctx, tx, plan); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	current, err := boundOrganizationClosePlan(locked, binding)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.State != "CLEANING" || string(original.Plan) != string(locked.Plan) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_CHANGED")
	}
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_DRAIN_PENDING")
	}
	if err = repository.LockAccountRetentionCompletion(ctx, tx, *current.Retention); err != nil {
		return err
	}
	for _, item := range current.Retention.Documents {
		if err = s.validateRetentionCompletion(ctx, tx, current, item); err != nil {
			return err
		}
	}
	if err = repository.CompleteAccountRetentionEntities(ctx, tx, *current.Retention); err != nil {
		return err
	}
	// 与准备期隐藏事件使用不同固定阶段号，不冲突、不创建新的正式历史。
	digest := sha256.Sum256([]byte("ORGANIZATION_RETENTION_COMPLETE\n" + binding.Operation))
	indexOperation := "retained-index-" + hex.EncodeToString(digest[:])
	documents := make([]uint64, 0, len(current.Retention.Documents))
	for _, item := range current.Retention.Documents {
		documents = append(documents, item.Document)
	}
	if err = queueVisibilityIndexEvents(ctx, tx, indexOperation, documents, 0); err != nil {
		return err
	}
	if err = repository.CompleteOrganizationClose(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN")
	}
	return nil
}

// 来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
// 提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。
func lockOrganizationCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error {
	sources := []domain.LeaseSource{{Key: plan.Binding.Scope(), Epoch: domain.TextID(plan.Binding.Epoch)}}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	slices.SortFunc(sources, func(a, b domain.LeaseSource) int { return strings.Compare(a.Key, b.Key) })
	for start := 0; start < len(sources); start += 100 {
		if _, err := repository.LockSourceGatesBatch(ctx, tx, sources[start:min(start+100, len(sources))]); err != nil {
			return err
		}
	}
	matches, err := repository.OrganizationCloseGateMatches(ctx, tx, plan.Binding)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	return nil
}

func (s *OrganizationClosures) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument) error {
	_, scopes, err := plan.CleanupDocument(item.Document)
	if err != nil {
		return err
	}
	if err = repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes); err != nil {
		return err
	}
	request := platform.AccountDocumentCleanupRequest{Operation: plan.Binding.Operation, Document: domain.TextID(item.Document)}
	proof, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request)
	if err != nil {
		return err
	}
	if !found {
		return domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document && proof != platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation) {
			return errors.New("ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED")
		}
	}
	facts, err := repository.PublicationFactsRow(ctx, tx, item.Document, false)
	if err != nil {
		return err
	}
	doc := facts.Document
	if item.Release == 0 {
		if doc.PublicationState != "UNPUBLISHED" || doc.CurrentReleaseId != nil || doc.CurrentManifestId != "" || doc.CurrentStructureId != "" || doc.CurrentStyleId != "" {
			return errors.New("ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS")
		}
		return nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return err
	}
	if policy != "PUBLIC" || doc.CurrentReleaseId == nil || *doc.CurrentReleaseId != domain.TextID(item.Release) || doc.CurrentManifestId != domain.TextID(item.Manifest) || doc.CurrentStructureId != domain.TextID(item.Structure) || doc.CurrentStyleId != domain.TextID(item.Style) {
		return errors.New("ACCOUNT_RETENTION_PUBLIC_BINDING_CHANGED")
	}
	actual := item
	actual.Versions, actual.Resources = []uint64{}, []uint64{}
	actual, err = s.retainedDependencies(ctx, tx, doc, actual)
	if err != nil {
		return err
	}
	if actual.Manifest != item.Manifest || actual.Structure != item.Structure || actual.Style != item.Style || !slices.Equal(actual.Versions, item.Versions) || !slices.Equal(actual.Resources, item.Resources) {
		return errors.New("ACCOUNT_RETENTION_DEPENDENCIES_CHANGED")
	}
	return nil
}

// organization_close_history.go：保留原实现，按业务合并文件。

// 此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
// 原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。
func (s *OrganizationClosures) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
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
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
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

// organization_close_payloads.go：保留原实现，按业务合并文件。

// 原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
// 原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。
func (s *OrganizationClosures) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
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
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
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

// organization_close_progress.go：保留原实现，按业务合并文件。

// 分页只读原固定文档集合；第一次读取先核验真实Doc/KB来源ACK并进入CLEANING。
// 尚未排空立即返回待重试，不等待网络；SUCCEEDED来自同一Doc完成事务。
func (s *OrganizationClosures) Progress(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error) {
	result := platform.OrganizationClosurePage{Documents: []platform.AccountClosureDocument{}}
	binding := request.Binding
	if !binding.Valid() {
		return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_BINDING_INVALID")
	}
	if err := s.ensureCleaning(ctx, binding); err != nil {
		return result, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return result, err
	}
	plan, err := boundOrganizationClosePlan(original, binding)
	if err != nil {
		return result, err
	}
	if plan.Retention == nil || (original.State != "PLANNED" && original.State != "CLEANING" && original.State != "SUCCEEDED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	digest := sha256.Sum256(original.Plan)
	result.PlanDigest = hex.EncodeToString(digest[:])
	if request.PlanDigest != "" && request.PlanDigest != result.PlanDigest {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
	}
	var after uint64
	if request.AfterDocument != "" {
		after, err = domain.CanonicalID(request.AfterDocument)
		if err != nil || request.PlanDigest == "" {
			return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_CURSOR_INVALID")
		}
		found := false
		for _, item := range plan.Retention.Documents {
			found = found || item.Document == after
		}
		if !found {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
		}
	}
	result.Binding, result.State, result.NextAfter, result.Done = request.Binding, original.State, request.AfterDocument, true
	for _, item := range plan.Retention.Documents {
		if item.Document <= after {
			continue
		}
		if len(result.Documents) == 100 {
			result.Done = false
			break
		}
		result.Documents = append(result.Documents, platform.AccountClosureDocument{Document: domain.TextID(item.Document), Release: domain.TextID(item.Release)})
		result.NextAfter = domain.TextID(item.Document)
	}
	return result, nil
}

// organization_close_ready.go：保留原实现，按业务合并文件。

func (s *OrganizationClosures) Ready(ctx context.Context) error {
	if s.authority == nil || s.validator == nil || s.cleaner == nil {
		return errors.New("ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED")
	}
	return repository.OrganizationCloseReady(ctx, s.db)
}

// organization_close_resources.go：保留原实现，按业务合并文件。

// ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
// 接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。
// 底层ACCOUNT_*清理任务是单文档引擎协议；资格仍只来自ORGANIZATION_CLOSE计划。
func (s *OrganizationClosures) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() || s.cleaner == nil {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANER_REQUIRED")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
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
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
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
````

UTF-8/LF全文SHA-256：`b80e14678643851653b7cb22b2e2438710a57bcdbfad8d6081f66502079b55f0`。[参考实现](../../../../services/document/internal/logic/document/organization_lifecycle_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
