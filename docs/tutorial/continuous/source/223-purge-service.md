# services/document/internal/logic/document/purge_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/223-purge-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Handles`、`DocumentCleanupContext`、`lockedCleanupContext`、`RegisterExpired`、`registerExpired`、`Recover`、`recoverTask`、`lockPurgeSources`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

// purge_context.go：保留原实现，按业务合并文件。

func (s *Purges) Handles(ctx context.Context, operation string) (bool, error) {
	kind, err := repository.LifecycleKind(ctx, s.db, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return kind == "PURGE", err
}

// 与Account使用同一真实单Doc清理协议，但资格只来自本PURGE固定计划/原屏障。
// authority HTTP在只读事务提交成功后才返回，且不拿TTL当清理或停止输出的证明。
func (s *Purges) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "PURGE_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	_, authority, err := s.lockedCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if err = repository.SetPurgeCleaning(ctx, tx, request.Operation); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("PURGE_CONTEXT_COMMIT_UNKNOWN")
	}
	return authority, nil
}

func (s *Purges) lockedCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (domain.PurgePlan, platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	original, err := repository.PurgeRow(ctx, tx, request.Operation, false)
	if err != nil {
		return domain.PurgePlan{}, empty, err
	}
	plan := original.Plan
	if domain.TextID(plan.Document) != request.Document || (original.State != "PREPARING" && original.State != "CLEANING") {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_CLEANING_REQUIRED")
	}
	// 父KB不被这个动作关闭。它可合法换管理员或可见性，每次取现今KB ACL后
	// 同gate串行再核当前读，不把旧父revision固定成永远无法恢复的清理资格。
	var kbRevision uint64
	var kbLifecycle string
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, false)
		if err != nil {
			return plan, empty, err
		}
		kbRevision, err = domain.CanonicalID(kb.AclRevision)
		if err != nil {
			return plan, empty, err
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return plan, empty, err
		}
		kbLifecycle = kb.Lifecycle
	}
	sources := []*pb.SourceEpoch{{ScopeKey: plan.Scopes[0].Key, Epoch: plan.ACL}, {ScopeKey: plan.Scopes[1].Key, Epoch: plan.Publication}}
	if plan.KnowledgeBase != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(plan.KnowledgeBase), Epoch: kbRevision})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	for _, source := range sources {
		if plan.KnowledgeBase != 0 && source.ScopeKey == "kb-acl:"+domain.TextID(plan.KnowledgeBase) {
			var closedKB uint64
			if kbLifecycle == "CLOSED" {
				closedKB = plan.KnowledgeBase
			}
			if err := lockPurgeSources(ctx, tx, []*pb.SourceEpoch{source}, closedKB); err != nil {
				return plan, empty, err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return plan, empty, err
		}
		if gate.Epoch > source.Epoch {
			return plan, empty, domain.Violation(domain.StateConflict, "PURGE_SOURCE_CHANGED")
		}
	}
	locked, err := repository.PurgeRow(ctx, tx, request.Operation, true)
	if err != nil {
		return plan, empty, err
	}
	if locked.State != "PREPARING" && locked.State != "CLEANING" {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_CLEANING_REQUIRED")
	}
	if !reflect.DeepEqual(locked.Plan, plan) {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_BINDING_CHANGED")
	}
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, true)
		if err != nil {
			return plan, empty, err
		}
		if kb.SpaceId != domain.TextID(plan.Space) || kb.AclRevision != domain.TextID(kbRevision) || kb.Lifecycle != kbLifecycle {
			return plan, empty, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, plan.Document, true)
	if err != nil {
		return plan, empty, err
	}
	d := facts.Document
	var currentKB uint64
	if d.KnowledgeBaseId != nil {
		currentKB, err = domain.CanonicalID(*d.KnowledgeBaseId)
		if err != nil {
			return plan, empty, err
		}
	}
	if d.Lifecycle != "CLOSING" || d.SpaceId != domain.TextID(plan.Space) || currentKB != plan.KnowledgeBase ||
		d.ControlRevision != domain.TextID(plan.ClosingControl) || d.AclRevision != domain.TextID(plan.ACL) || d.PublicationEpoch != domain.TextID(plan.Publication) || d.CurrentReleaseId != nil || facts.Sequence != plan.Sequence {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, plan.Space, true)
	if err != nil {
		return plan, empty, err
	}
	if space.Lifecycle != "ACTIVE" {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
	}
	drained, err := repository.PurgeDrained(ctx, tx, plan)
	if err != nil {
		return plan, empty, err
	}
	if !drained {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_DRAIN_PENDING")
	}
	pending, err := repository.DocumentWritesPending(ctx, tx, plan.Document, plan.Operation)
	if err != nil {
		return plan, empty, err
	}
	if pending {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_WRITES_PENDING")
	}
	return plan, platform.NewAccountDocumentCleanupContext(request, plan.ACL, plan.Scopes[0].Operation), nil
}

// purge_expiry.go：保留原实现，按业务合并文件。

// 每轮固定最多16篇，分别用原事务登记；一个刚恢复/父级关闭的候选不能堵住
// 同轮其它到期项。竞争失败保留TRASHED，下一轮重新读DB时钟及同一稳定原号。
func (s *Purges) RegisterExpired(ctx context.Context) (int, error) {
	ids, err := repository.ExpiredTrashCandidates(ctx, s.db, s.expiryCursor.Load())
	if err != nil {
		return 0, err
	}
	// 仅保留一个扫描编号，不保存无限候选/正文；永久等待的前16项不能饿死
	// 后续到期项。走到尾部后下一轮从头核验，重启也从头读真实到期事实。
	if len(ids) == 0 {
		s.expiryCursor.Store(0)
		return 0, nil
	}
	s.expiryCursor.Store(ids[len(ids)-1])
	registered := 0
	var first error
	for _, id := range ids {
		if ctx.Err() != nil {
			return registered, ctx.Err()
		}
		if err := s.registerExpired(ctx, id); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		registered++
	}
	return registered, first
}

func (s *Purges) registerExpired(ctx context.Context, id uint64) error {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return err
	}
	if err = domain.ValidatePurgeIntent(facts, true, facts.Document.ControlRevision, true); err != nil {
		return err
	}
	operation := domain.PurgeExpiryOperation(facts)
	plan, err := s.plan(ctx, facts, operation, 0, true)
	if err != nil {
		return err
	}
	var kbRevision uint64
	var closedKB uint64
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, s.db, plan.KnowledgeBase, false)
		if err != nil {
			return err
		}
		if kb.SpaceId != facts.Document.SpaceId {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return err
		}
		if kb.Lifecycle == "CLOSED" {
			closedKB = plan.KnowledgeBase
		}
		kbRevision, err = domain.CanonicalRevision(kb.AclRevision)
		if err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, operation); err != nil {
		return err
	}
	// 必须在任何预期next-epoch重开之前认原号，防止重试产生未发生的高水位。
	if original, err := repository.PurgeRow(ctx, tx, operation, false); err == nil {
		if !original.Plan.Automatic || original.Plan.Document != id {
			return errors.New("PURGE_EXPIRY_BINDING_CHANGED")
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sources := []*pb.SourceEpoch{{ScopeKey: plan.Scopes[0].Key, Epoch: plan.ACL}, {ScopeKey: plan.Scopes[1].Key, Epoch: plan.Publication}}
	if plan.KnowledgeBase != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(plan.KnowledgeBase), Epoch: kbRevision})
	}
	if err = lockPurgeSources(ctx, tx, sources, closedKB); err != nil {
		return err
	}
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, true)
		if err != nil {
			return err
		}
		if kb.SpaceId != facts.Document.SpaceId || kb.AclRevision != domain.TextID(kbRevision) || (kb.Lifecycle == "CLOSED") != (closedKB != 0) {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return err
		}
	}
	current, err := repository.TrashDocumentRow(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if err = domain.ValidatePurgeIntent(current, true, facts.Document.ControlRevision, true); err != nil {
		return err
	}
	if current.Document.SpaceId != facts.Document.SpaceId || !sameTrashID(current.Document.KnowledgeBaseId, facts.Document.KnowledgeBaseId) ||
		current.Document.AclRevision != facts.Document.AclRevision || current.Document.PublicationEpoch != facts.Document.PublicationEpoch ||
		current.Document.VisibilityRevision != facts.Document.VisibilityRevision || current.Sequence != facts.Sequence || domain.PurgeExpiryOperation(current) != operation {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, plan.Space, true)
	if err != nil {
		return err
	}
	if space.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	if err = registerPurge(ctx, tx, plan); err != nil {
		return err
	}
	input := dto.PurgeAction{OperationId: operation, ExpectedControlRevision: facts.Document.ControlRevision, Confirmed: true}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+domain.TextID(id)+"/purge", input)
	if err != nil {
		return err
	}
	result, err := json.Marshal(purgeProgress(operation, "PREPARING", id))
	if err != nil {
		return err
	}
	if err = repository.RegisterPurgeExpiryReceipt(ctx, tx, plan, fingerprint, result); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_EXPIRY_COMMIT_UNKNOWN")
	}
	return nil
}

// purge_recovery.go：保留原实现，按业务合并文件。

// 每次只领取一个原任务。网络调用在事务外；原Node回执和Doc最终提交都可重送，
// lease只决定谁能提交，不能证明正文、输出或安装已经停止。
func (s *Purges) Recover(ctx context.Context) error {
	if s.cleaner == nil || s.instance == "" {
		return errors.New("PURGE_DEPENDENCIES_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	task, err := repository.ClaimPurge(ctx, tx, s.instance)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPurgeIdle
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_CLAIM_COMMIT_UNKNOWN")
	}
	if err = s.recoverTask(ctx, task); err != nil {
		// 原提交可能已经成功：只CAS仍属本worker的RUNNING，绝不覆盖已SUCCEEDED。
		retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = repository.RetryPurge(retry, s.db, task, s.instance)
	}
	return err
}

func (s *Purges) recoverTask(ctx context.Context, task repository.PurgeTask) error {
	original, err := repository.PurgeRow(ctx, s.db, task.Operation, false)
	if err != nil {
		return err
	}
	request := platform.AccountDocumentCleanupRequest{Operation: task.Operation, Document: domain.TextID(original.Plan.Document)}
	expected, err := s.DocumentCleanupContext(ctx, request)
	if err != nil {
		return err
	}
	if err = s.cleaner.Clean(ctx, expected); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, authority, err := s.lockedCleanupContext(ctx, tx, request)
	if err != nil {
		return err
	}
	if authority != expected {
		return errors.New("PURGE_NODE_BINDING_CHANGED")
	}
	// 所有恢复器都先来源/lifecycle/业务事实，再任务ownership，不能持任务锁等gate。
	if err = repository.LockPurgeOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	item := domain.AccountRetentionDocument{Document: plan.Document, Space: plan.Space, KnowledgeBase: plan.KnowledgeBase}
	if err = repository.ReleaseDocumentDraftResourcePins(ctx, tx, item, authority); err != nil {
		return err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return err
	}
	// 先清固定任务副本再删其关联图，避免graph删admission等归属后漏掉正文副本。
	if err = repository.RemoveAccountPayloads(ctx, tx, authority); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, plan.Document); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "PURGE_DOCUMENT_CLEANUP", authority.Document+":"+authority.Operation, authority.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.CompletePurgeDocument(ctx, tx, plan); err != nil {
		return err
	}
	result, err := json.Marshal(purgeProgress(plan.Operation, "SUCCEEDED", plan.Document))
	if err != nil {
		return err
	}
	if err = repository.CompletePurgeReceipt(ctx, tx, plan, result); err != nil {
		return err
	}
	if err = repository.FinishPurgeTask(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_FINAL_COMMIT_UNKNOWN")
	}
	return nil
}

// purge_sources.go：保留原实现，按业务合并文件。

// 永久清理不为CLOSED父KB创建新正文输出资格。闭合父来源保持原BLOCKED，
// 只有其精确原关闭/ACK已经终结才能接受；其他来源仍采用普通准入规则。
// 合并真实身份证明后一次字典序锁门，不能先持session再倒序拿doc/KB。
func lockPurgeSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, closedKB uint64) error {
	ordered := append([]*pb.SourceEpoch(nil), sources...)
	seen := map[string]bool{}
	for _, source := range ordered {
		if source == nil || source.Epoch == 0 || seen[source.ScopeKey] {
			return errors.New("PURGE_SOURCES_INVALID")
		}
		seen[source.ScopeKey] = true
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ScopeKey < ordered[j].ScopeKey })
	for _, source := range ordered {
		if closedKB == 0 || source.ScopeKey != "kb-acl:"+domain.TextID(closedKB) {
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{source}); err != nil {
				return err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		if gate.Epoch != source.Epoch || gate.State != "BLOCKED" {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_SOURCE_CHANGED")
		}
		pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
		if err != nil {
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_PENDING")
		}
		drained, err := repository.ClosedPurgeParentDrained(ctx, tx, closedKB, source.Epoch)
		if err != nil {
			return err
		}
		if !drained {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_PENDING")
		}
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`80108cef760917c392f6ca1e8b26d6e77fe4782d9421e7d7b426b6237535792c`。[参考实现](../../../../services/document/internal/logic/document/purge_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
