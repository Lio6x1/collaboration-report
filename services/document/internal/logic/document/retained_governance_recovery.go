package document

import (
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

var ErrRetainedGovernanceIdle = errors.New("RETAINED_GOVERNANCE_IDLE")

func (s *RetainedGovernance) Recover(ctx context.Context) error {
	if s.cleaner == nil || s.resources == nil || s.exports == nil || s.instance == "" {
		return errors.New("RETAINED_GOVERNANCE_DEPENDENCIES_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	task, err := repository.ClaimRetainedGovernance(ctx, tx, s.instance)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRetainedGovernanceIdle
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_CLAIM_COMMIT_UNKNOWN")
	}
	if err = s.recover(ctx, task); err != nil {
		retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = repository.RetryRetainedGovernance(retry, s.db, task, s.instance)
	}
	return err
}

func (s *RetainedGovernance) recover(ctx context.Context, task repository.PurgeTask) error {
	original, err := s.Lookup(ctx, task.Operation)
	if err != nil {
		return err
	}
	plan := original.Plan
	if plan.Action == "PURGE" {
		request := platform.AccountDocumentCleanupRequest{Operation: task.Operation, Document: domain.TextID(plan.Purge.Document)}
		expected, err := s.DocumentCleanupContext(ctx, request)
		if err != nil {
			return err
		}
		// 先等来源真实ACK，然后在锁外排空导出和取得Node原清理回执。
		// 这些事实的结果未知时重送原号，不把进程租约当成正文/文件已删除。
		if err = s.exports.DrainDocument(ctx, plan.Purge.Document); err != nil {
			return err
		}
		if err = s.cleaner.Clean(ctx, expected); err != nil {
			return err
		}
		if err = s.removeHistory(ctx, task, expected); err != nil {
			return err
		}
		for _, resource := range plan.Resources {
			if err = s.resources.CleanRetainedResource(ctx, resource, plan.Purge.Space, plan.Purge.Document); err != nil {
				return err
			}
		}
	}
	return s.finish(ctx, task, plan)
}

// Node向Doc反查此context；它与Account的单文档协议相同，但发证资格只来自
// 本治理任务、精确原关闭范围及ACK，绝不借用旧Account操作重新发永久清理。
func (s *RetainedGovernance) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "RETAINED_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	if plan.Action != "PURGE" || request.Document != domain.TextID(plan.Purge.Document) {
		return empty, domain.Violation(domain.PermissionDenied, "RETAINED_PURGE_PLAN_REQUIRED")
	}
	pending, err := repository.DocumentWritesPending(ctx, tx, plan.Purge.Document, plan.Purge.Operation)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "RETAINED_WRITES_PENDING")
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("RETAINED_CONTEXT_COMMIT_UNKNOWN")
	}
	return platform.NewAccountDocumentCleanupContext(request, plan.Purge.ACL, plan.Purge.Scopes[0].Operation), nil
}

// 恢复不要求平台管理员继续在线；只能完成已经带审计登记的原关闭计划。
// 来源→Doc→KB→Space→任务，其他worker不能持任务锁等待来源。
func (s *RetainedGovernance) lockedCurrent(ctx context.Context, tx *sql.Tx, operation string) (domain.RetainedGovernancePlan, error) {
	original, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
	if err != nil {
		return original.Plan, err
	}
	plan := original.Plan
	p := plan.Purge
	scopes := append([]domain.PurgeScope(nil), p.Scopes...)
	if p.KnowledgeBase != 0 {
		scopes = append(scopes, domain.PurgeScope{Key: "kb-acl:" + domain.TextID(p.KnowledgeBase), Epoch: plan.KnowledgeBaseACL})
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Key < scopes[j].Key })
	for _, source := range scopes {
		gate, err := repository.LockSourceGate(ctx, tx, source.Key, source.Epoch)
		if err != nil {
			return plan, err
		}
		state := "BLOCKED"
		if source.Operation == "" {
			state = "OPEN"
		}
		if gate.Epoch != source.Epoch || gate.State != state {
			return plan, domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, p.Document, true)
	if err != nil {
		return plan, err
	}
	d := facts.Document
	expectedLifecycle := "PUBLIC_RETAINED"
	if plan.Action == "PURGE" {
		expectedLifecycle = "CLOSING"
	}
	if d.Lifecycle != expectedLifecycle || d.SpaceId != domain.TextID(p.Space) || d.ControlRevision != domain.TextID(p.ClosingControl) ||
		d.AclRevision != domain.TextID(p.ACL) || d.PublicationEpoch != domain.TextID(p.Publication) || d.CurrentReleaseId != nil ||
		d.PublicationState != "UNPUBLISHED" || d.ResponsibleUserId != nil || !d.MainLocked || facts.Sequence != p.Sequence {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
	}
	if p.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, p.KnowledgeBase, true)
		if err != nil {
			return plan, err
		}
		if d.KnowledgeBaseId == nil || *d.KnowledgeBaseId != kb.Id || kb.SpaceId != d.SpaceId || kb.Lifecycle != "PUBLIC_RETAINED" || kb.AclRevision != domain.TextID(plan.KnowledgeBaseACL) {
			return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
		}
	} else if d.KnowledgeBaseId != nil {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, p.Space, true)
	if err != nil {
		return plan, err
	}
	if space.Lifecycle != "PUBLIC_RETAINED" {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
	}
	locked, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
	if err != nil {
		return plan, err
	}
	if locked.State == "SUCCEEDED" || !reflect.DeepEqual(locked.Plan, plan) {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_ORIGINAL_CHANGED")
	}
	drained, err := repository.RetainedGovernanceDrained(ctx, tx, plan)
	if err != nil {
		return plan, err
	}
	if !drained {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
	}
	return plan, nil
}

func (s *RetainedGovernance) removeHistory(ctx context.Context, task repository.PurgeTask, expected platform.AccountDocumentCleanupContext) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	p := plan.Purge
	if expected != platform.NewAccountDocumentCleanupContext(platform.AccountDocumentCleanupRequest{Operation: p.Operation, Document: domain.TextID(p.Document)}, p.ACL, p.Scopes[0].Operation) {
		return errors.New("RETAINED_NODE_BINDING_CHANGED")
	}
	if err = repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if prior, found, err := repository.RetainedGovernanceCleanupReceipt(ctx, tx, p.Operation); err != nil {
		return err
	} else if found {
		if prior != expected {
			return errors.New("RETAINED_CLEANUP_BINDING_CHANGED")
		}
		return nil
	}
	drained, err := repository.RetainedExportArtifactsDrained(ctx, tx, p.Document)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "RETAINED_EXPORT_DRAIN_PENDING")
	}
	item := domain.AccountRetentionDocument{Document: p.Document, Space: p.Space, KnowledgeBase: p.KnowledgeBase}
	if err = repository.ReleaseDocumentDraftResourcePins(ctx, tx, item, expected); err != nil {
		return err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return err
	}
	if err = repository.RemoveAccountPayloads(ctx, tx, expected); err != nil {
		return err
	}
	if err = repository.RemoveRetainedExportControls(ctx, tx, p.Document); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, p.Document); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return err
	}
	if err = repository.ReleaseRetainedResourcePins(ctx, tx, p.Document); err != nil {
		return err
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "RETAINED_GOVERNANCE_CLEANED", p.Operation, p.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_HISTORY_COMMIT_UNKNOWN")
	}
	return nil
}

func (s *RetainedGovernance) finish(ctx context.Context, task repository.PurgeTask, expected domain.RetainedGovernancePlan) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, expected) {
		return domain.Violation(domain.StateConflict, "RETAINED_PLAN_CHANGED")
	}
	if err = repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if plan.Action == "PURGE" {
		if _, found, err := repository.RetainedGovernanceCleanupReceipt(ctx, tx, task.Operation); err != nil {
			return err
		} else if !found {
			return domain.Violation(domain.StateConflict, "RETAINED_HISTORY_CLEANUP_PENDING")
		}
		if err = repository.ValidateRetainedResourceCompletion(ctx, tx, plan); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(retainedGovernanceProgress(task.Operation, "SUCCEEDED", plan.Purge.Document))
	if err != nil {
		return err
	}
	if err = repository.CompleteRetainedGovernance(ctx, tx, plan, raw); err != nil {
		return err
	}
	if err = repository.FinishPurgeTask(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_FINAL_COMMIT_UNKNOWN")
	}
	return nil
}
