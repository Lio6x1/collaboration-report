// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	allocations "collaboration/document/internal/repository/resource"
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

// account_retention.go：保留原实现，按业务合并文件。

type AccountRetentionAuthority interface {
	Confirmed(context.Context, platform.AccountClosureBinding) (bool, error)
}

type AccountDocumentCleaner interface {
	Clean(context.Context, platform.AccountDocumentCleanupContext) error
}

type AccountRetentions struct {
	db        *sql.DB
	authority AccountRetentionAuthority
	validator *codec.Validator
	cleaner   AccountDocumentCleaner
}

func NewAccountRetentions(db *sql.DB, authority AccountRetentionAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *AccountRetentions {
	return &AccountRetentions{db: db, authority: authority, validator: validator, cleaner: cleaner}
}

// Plan先取得Account原固定身份屏障的权威确认，再固定当前公开正式版。
// 这里只进入PLANNED和排空，不能因为计划已保存就释放草稿保护或结束全局注销。
func (s *AccountRetentions) Plan(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error) {
	var empty domain.AccountRetirementPlan
	if !binding.Valid() || s.authority == nil || s.validator == nil {
		return empty, errors.New("ACCOUNT_RETENTION_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return empty, err
	}
	if !confirmed {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_FREEZE_UNCONFIRMED")
	}
	// 跨服务查询必须在SQL锁外。确认事实不可逆；冻结后Account不再允许取消。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	original, err := repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return empty, err
	}
	if plan.Retention != nil {
		return plan, nil
	}
	if original.State != "FROZEN" {
		return empty, errors.New("ACCOUNT_RETENTION_STATE_INVALID")
	}
	candidates, err := repository.DiscoverAccountRetention(ctx, tx, binding)
	if err != nil {
		return empty, err
	}
	if err = lockAccountRetentionSources(ctx, tx, candidates.Sources, binding); err != nil {
		return empty, err
	}
	original, err = repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return empty, err
	}
	plan, err = boundRetentionPlan(original, binding)
	if err != nil {
		return empty, err
	}
	if plan.Retention != nil {
		return plan, nil
	}
	if original.State != "FROZEN" {
		return empty, errors.New("ACCOUNT_RETENTION_STATE_INVALID")
	}
	// Account 已冻结身份，但已接受的跨空间原迁移不能同时被固定进两份清理计划。
	pending, err := repository.PersonalTransferAccountPending(ctx, tx, binding.User)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return empty, err
	}
	current, err := repository.DiscoverAccountRetention(ctx, tx, binding)
	if err != nil {
		return empty, err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_CANDIDATES_CHANGED")
	}
	retention := domain.AccountRetentionPlan{Spaces: current.Spaces, KnowledgeBases: current.KnowledgeBases, Documents: []domain.AccountRetentionDocument{}, Scopes: []domain.AccountRetentionScope{}}
	for _, id := range current.Documents {
		item, err := s.retainedDocument(ctx, tx, id)
		if err != nil {
			return empty, err
		}
		retention.Documents = append(retention.Documents, item)
	}
	for _, source := range current.Sources {
		epoch, err := domain.CanonicalID(source.Epoch)
		if err != nil || epoch == math.MaxUint64 {
			return empty, errors.New("ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
		}
		retention.Scopes = append(retention.Scopes, domain.AccountRetentionScope{Key: source.Key, Epoch: epoch + 1, Operation: domain.AccountRetentionRevocation(binding.Operation, source.Key)})
	}
	plan.Retention = &retention
	if !retention.Valid(binding.Operation) {
		return empty, errors.New("ACCOUNT_RETENTION_PLAN_INVALID")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return empty, err
	}
	if err = repository.SaveAccountRetention(ctx, tx, binding.Operation, raw, retention); err != nil {
		return empty, err
	}
	for _, scope := range retention.Scopes {
		// 复用真实Doc/Edge/Node排空worker。子操作只终结自己的屏障任务，
		// 没有用户operation_receipt，不会把原ACCOUNT_CLOSE提前写成成功。
		closure := domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Action: "ACCOUNT_RETENTION"}
		if err = QueueClosure(ctx, tx, scope.Operation, binding.User, closure); err != nil {
			return empty, err
		}
	}
	if err = queueVisibilityIndexEvents(ctx, tx, binding.Operation, current.Documents, 0); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_RETENTION_PLAN_UNKNOWN")
	}
	return plan, nil
}

func boundRetentionPlan(record repository.AccountRetirementRecord, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error) {
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
		return plan, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
	}
	if record.State == "REJECTED" {
		return plan, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_REJECTED")
	}
	return plan, nil
}

func lockAccountRetentionSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, binding domain.AccountRetirementBinding) error {
	all := slices.Clone(sources)
	identity := "identity:" + domain.TextID(binding.User)
	all = append(all, domain.LeaseSource{Key: identity, Epoch: domain.TextID(binding.IdentityEpoch)})
	slices.SortFunc(all, func(a, b domain.LeaseSource) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	// 100项只限制一次SQL大小，不能截断整份关闭计划。
	for start := 0; start < len(all); start += 100 {
		end := min(start+100, len(all))
		page := all[start:end]
		gates, err := repository.LockSourceGatesBatch(ctx, tx, page)
		if err != nil {
			return err
		}
		keys := []string{}
		for _, source := range page {
			epoch, err := domain.CanonicalID(source.Epoch)
			if err != nil {
				return err
			}
			gate := gates[source.Key]
			if source.Key == identity {
				matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
				if err != nil {
					return err
				}
				if !matches {
					return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
				}
			} else {
				if gate.Epoch > epoch {
					return errors.New("ACCOUNT_RETENTION_SOURCE_CHANGED")
				}
				keys = append(keys, source.Key)
			}
		}
		pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, keys)
		if err != nil {
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
		}
	}
	return nil
}

// account_retention_complete.go：保留原实现，按业务合并文件。

// Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
// 循环同步清理整个账号。Account恢复器应逐文档推进后调用本关口，原号重放。
// Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须自己的最终去标识事务。
func (s *AccountRetentions) Complete(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.validator == nil {
		return errors.New("ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundRetentionPlan(original, binding)
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
	if err = lockRetentionCompletionSources(ctx, tx, plan); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	current, err := boundRetentionPlan(locked, binding)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.State != "CLEANING" || string(original.Plan) != string(locked.Plan) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_CHANGED")
	}
	drained, err := repository.AccountRetentionDrained(ctx, tx, current)
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
	digest := sha256.Sum256([]byte("ACCOUNT_RETENTION_COMPLETE\n" + binding.Operation))
	indexOperation := "retained-index-" + hex.EncodeToString(digest[:])
	documents := make([]uint64, 0, len(current.Retention.Documents))
	for _, item := range current.Retention.Documents {
		documents = append(documents, item.Document)
	}
	if err = queueVisibilityIndexEvents(ctx, tx, indexOperation, documents, 0); err != nil {
		return err
	}
	if err = repository.CompleteAccountRetention(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN")
	}
	return nil
}

// 来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
// 提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。
func lockRetentionCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error {
	sources := []domain.LeaseSource{{Key: "identity:" + domain.TextID(plan.Binding.User), Epoch: domain.TextID(plan.Binding.IdentityEpoch)}}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	slices.SortFunc(sources, func(a, b domain.LeaseSource) int { return strings.Compare(a.Key, b.Key) })
	for start := 0; start < len(sources); start += 100 {
		if _, err := repository.LockSourceGatesBatch(ctx, tx, sources[start:min(start+100, len(sources))]); err != nil {
			return err
		}
	}
	matches, err := repository.AccountRetirementGateMatches(ctx, tx, plan.Binding)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	return nil
}

func (s *AccountRetentions) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument) error {
	_, scopes, err := plan.CleanupDocument(item.Document)
	if err != nil {
		return err
	}
	if err = repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes); err != nil {
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

// account_retention_dependencies.go：保留原实现，按业务合并文件。

func (s *AccountRetentions) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error) {
	item := domain.AccountRetentionDocument{Document: id, Versions: []uint64{}, Resources: []uint64{}}
	facts, err := repository.PublicationFactsRow(ctx, tx, id, false)
	if err != nil {
		return item, err
	}
	item.Space, err = domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return item, err
	}
	if facts.KnowledgeBase != nil {
		item.KnowledgeBase, err = domain.CanonicalID(facts.KnowledgeBase.Id)
		if err != nil {
			return item, err
		}
	}
	if facts.Document.Lifecycle != "ACTIVE" && facts.Document.Lifecycle != "TRASHED" {
		return item, errors.New("ACCOUNT_RETENTION_DOCUMENT_TRANSITION_PENDING")
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return item, err
	}
	if policy != "PUBLIC" || !domain.HasCurrentPublication(facts, policy) {
		return item, nil
	}
	item.Release, err = domain.CanonicalID(*facts.Document.CurrentReleaseId)
	if err != nil {
		return item, err
	}
	return s.retainedDependencies(ctx, tx, facts.Document, item)
}

// 准备期与最终留存启用共享同一依赖核验；启用期对象仍CLOSING，不能临时
// 改成ACTIVE绕过状态检查。这里只解析固定release，不选择新的保留范围。
func (s *AccountRetentions) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error) {
	id := item.Document
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, item.Release)
	if err != nil {
		return item, err
	}
	item.Manifest, err = domain.CanonicalID(fixed.Release.ManifestId)
	if err != nil {
		return item, err
	}
	item.Structure, err = domain.CanonicalID(fixed.Structure)
	if err != nil {
		return item, err
	}
	item.Style, err = domain.CanonicalID(fixed.Style)
	if err != nil {
		return item, err
	}
	structure, err := repository.StructureRow(ctx, tx, fixed.Structure, document.Id)
	if err != nil {
		return item, err
	}
	if _, err = repository.StyleRow(ctx, tx, fixed.Style, document.Id); err != nil {
		return item, err
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return item, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return item, err
	}
	resources := map[uint64]bool{}
	for _, unit := range units {
		unitID, err := domain.CanonicalID(unit["unit_id"])
		if err != nil {
			return item, err
		}
		version, err := domain.CanonicalID(unit["content_version_id"])
		if err != nil {
			return item, err
		}
		content, err := repository.VersionContent(ctx, tx, version, id)
		if err != nil {
			return item, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return item, err
		}
		if err = validateFixedMergeContent(s.validator, content, id, unitID, item.Structure); err != nil {
			return item, err
		}
		item.Versions = append(item.Versions, version)
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	slices.Sort(item.Versions)
	item.Versions = slices.Compact(item.Versions)
	for resource := range resources {
		item.Resources = append(item.Resources, resource)
	}
	slices.Sort(item.Resources)
	// 依赖必须仍属于本空间并READY。持有文档锁后检查引用，GC无法在引用存在时删除。
	for _, resource := range item.Resources {
		state, err := repository.ResourceState(ctx, tx, resource)
		if err != nil {
			return item, err
		}
		allowed, err := allocations.ResourceAllowedForDocument(ctx, tx, resource, item.Document, item.Space)
		if err != nil {
			return item, err
		}
		if !allowed || state != "READY" {
			return item, errors.New("ACCOUNT_RETENTION_RESOURCE_UNKNOWN")
		}
	}
	return item, nil
}

// account_retention_progress.go：保留原实现，按业务合并文件。

// 原固定计划只读分页，不扫描可变空间/文档集合，也不等待清理或执行网络调用。
// 完成证明来自同一Doc事务写入的SUCCEEDED，Account还须核对自己的冻结/ACK事实。
func (s *AccountRetentions) Progress(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error) {
	result := platform.AccountClosurePage{Documents: []platform.AccountClosureDocument{}}
	binding := domain.AccountRetirementBinding(request.AccountClosureBinding)
	if !binding.Valid() {
		return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return result, err
	}
	plan, err := boundRetentionPlan(original, binding)
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
	result.Binding, result.State, result.NextAfter, result.Done = request.AccountClosureBinding, original.State, request.AfterDocument, true
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
