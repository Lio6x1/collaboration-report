# services/document/internal/logic/document/retained_governance_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/232-retained-governance-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`RetainedGovernanceAuthority`、`RetainedResourceCleaner`、`RetainedExportDrainer`、`RetainedGovernance`、`NewRetainedGovernance`、`Authorize`、`Prepare`、`lockRetainedRegistrationSources`、`retainedGovernanceProgress`、`Lookup`、`Progress`、`Metadata`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

// retained_governance.go：保留原实现，按业务合并文件。

// 实现方必须调用Account真实治理能力，不得根据Doc旧grant、组织角色或
// 进程配置名单发证；标准事务还会向Account再次Revalidate这个证明。
type RetainedGovernanceAuthority interface {
	AcquireGovernance(context.Context, Actor, string, uint64) (*pb.IdentityAdmission, error)
}

type RetainedResourceCleaner interface {
	CleanRetainedResource(context.Context, uint64, uint64, uint64) error
}

type RetainedExportDrainer interface {
	DrainDocument(context.Context, uint64) error
}

type RetainedGovernance struct {
	db        *sql.DB
	authority RetainedGovernanceAuthority
	cleaner   AccountDocumentCleaner
	resources RetainedResourceCleaner
	exports   RetainedExportDrainer
	instance  string
}

func NewRetainedGovernance(db *sql.DB, authority RetainedGovernanceAuthority, cleaner AccountDocumentCleaner, resources RetainedResourceCleaner, exports RetainedExportDrainer, instance string) *RetainedGovernance {
	return &RetainedGovernance{db: db, authority: authority, cleaner: cleaner, resources: resources, exports: exports, instance: instance}
}

func (s *RetainedGovernance) Authorize(ctx context.Context, actor Actor, operation string, document uint64) (Actor, error) {
	if s.authority == nil || actor.Proof == nil || !domain.ValidOperationID(operation) || document == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "RETAINED_GOVERNANCE_AUTHORITY_REQUIRED")
	}
	proof, err := s.authority.AcquireGovernance(ctx, actor, operation, document)
	if err != nil {
		return actor, err
	}
	if proof == nil || proof.ActorId != actor.ID || proof.OperationId != operation || proof.Purpose != domain.RetainedGovernancePurpose ||
		proof.Target == nil || proof.Target.DocumentId != document || proof.Target.Purpose != domain.RetainedGovernancePurpose ||
		proof.IdentityTarget == nil || proof.IdentityTarget.Kind != pb.IdentityTarget_USER || proof.IdentityTarget.UserId != actor.ID || proof.IdentityTarget.OrganizationId != 0 || proof.IdentityTarget.SpaceId != 0 ||
		proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT || proof.Target.UnitId != 0 || proof.Target.BranchId != 0 || proof.Target.StructureId != 0 || proof.Target.WorkspaceEpoch != 0 ||
		proof.ExpiresUnixMs <= time.Now().UnixMilli() || !bytes.Equal(proof.SessionRef, actor.Proof.SessionRef) {
		return actor, errors.New("RETAINED_GOVERNANCE_PROOF_INVALID")
	}
	governance := false
	for _, source := range proof.Sources {
		if source != nil && source.ScopeKey == "platform-admin:"+domain.TextID(actor.ID) && source.Epoch != 0 {
			governance = true
		}
	}
	if !governance {
		return actor, errors.New("RETAINED_GOVERNANCE_SOURCE_REQUIRED")
	}
	merged, err := MergeSources(proof, actor.Proof)
	if err != nil {
		return actor, err
	}
	if len(merged) != len(proof.Sources) {
		return actor, errors.New("RETAINED_GOVERNANCE_IDENTITY_SOURCES_REQUIRED")
	}
	actor.Proof, actor.Sources = proof, proof.Sources
	return actor, nil
}

// handler先Authorize并标准Replay；Prepare只生成新号的不可变计划。来源锁
// 在run中一次排序，避免先持session再倒序申请Doc/KB形成反向锁顺序。
func (s *RetainedGovernance) Prepare(ctx context.Context, actor Actor, id uint64, action string, input dto.VersionedAction) (Actor, MutationBody, MutationPresentation, error) {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, nil, err
	}
	space, err := repository.TrashSpaceRow(ctx, s.db, spaceID, false)
	if err != nil {
		return actor, nil, nil, err
	}
	var kb *dto.KnowledgeBase
	var kbID uint64
	if facts.Document.KnowledgeBaseId != nil {
		kbID, err = domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return actor, nil, nil, err
		}
		value, err := repository.TrashKnowledgeBaseRow(ctx, s.db, kbID, false)
		if err != nil {
			return actor, nil, nil, err
		}
		kb = &value
	}
	if err = domain.ValidateRetainedGovernance(facts, space, kb, action, input.ExpectedRevision); err != nil {
		return actor, nil, nil, err
	}
	acl, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+domain.TextID(id))
	if err != nil {
		return actor, nil, nil, err
	}
	publication, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+domain.TextID(id))
	if err != nil {
		return actor, nil, nil, err
	}
	purge, err := domain.NewPurgePlan(input.OperationId, actor.ID, false, facts, kbID, acl, publication)
	if err != nil {
		return actor, nil, nil, err
	}
	plan := domain.RetainedGovernancePlan{Purge: purge, Action: action, Resources: []uint64{}}
	if facts.Document.CurrentReleaseId != nil {
		plan.Release, err = domain.CanonicalID(*facts.Document.CurrentReleaseId)
		if err != nil {
			return actor, nil, nil, err
		}
	}
	plan.Resources, err = repository.RetainedDocumentResources(ctx, s.db, id)
	if err != nil {
		return actor, nil, nil, err
	}
	sources := append([]*pb.SourceEpoch(nil), actor.Proof.Sources...)
	for _, source := range []struct{ key, revision string }{{"doc-acl:" + domain.TextID(id), facts.Document.AclRevision}, {"publication:" + domain.TextID(id), facts.Document.PublicationEpoch}} {
		epoch, err := domain.CanonicalRevision(source.revision)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: source.key, Epoch: epoch})
	}
	if kb != nil {
		epoch, err := domain.CanonicalRevision(kb.AclRevision)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kbID), Epoch: epoch})
		plan.KnowledgeBaseACL = epoch
	}
	actor.Sources = []*pb.SourceEpoch{}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := lockRetainedRegistrationSources(ctx, tx, sources, id); err != nil {
			return nil, err
		}
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		var currentKB *dto.KnowledgeBase
		if kb != nil {
			value, err := repository.TrashKnowledgeBaseRow(ctx, tx, kbID, true)
			if err != nil {
				return nil, err
			}
			currentKB = &value
		}
		currentSpace, err := repository.TrashSpaceRow(ctx, tx, spaceID, true)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateRetainedGovernance(current, currentSpace, currentKB, action, input.ExpectedRevision); err != nil {
			return nil, err
		}
		if current.Sequence != facts.Sequence || !reflect.DeepEqual(current.Document, facts.Document) || !reflect.DeepEqual(currentKB, kb) {
			return nil, domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
		}
		if err = repository.BeginRetainedGovernance(ctx, tx, plan); err != nil {
			return nil, err
		}
		for _, source := range purge.Scopes {
			if err = QueueClosure(ctx, tx, source.Operation, actor.ID, domain.Closure{Scope: source.Key, Epoch: source.Epoch, Entity: id, Action: domain.RetainedGovernancePurpose}); err != nil {
				return nil, err
			}
		}
		payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: input.OperationId, Type: "DOCUMENT_UNPUBLISHED", Document: id, Sequence: purge.Sequence, PublicationEpoch: purge.Publication, OccurredAt: time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		if err = repository.InsertIndexEvent(ctx, tx, id, purge.Sequence, input.OperationId, "DOCUMENT_UNPUBLISHED", payload); err != nil {
			return nil, err
		}
		return retainedGovernanceProgress(input.OperationId, "PREPARING", id), nil
	}
	present := func(_ context.Context, _ *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		// 只发送最小固定receipt；正文和已关闭Doc来源不被治理身份重新开放。
		return value, actor.Proof.Sources, nil
	}
	return actor, run, present, nil
}

func lockRetainedRegistrationSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64) error {
	ordered := append([]*pb.SourceEpoch(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ScopeKey < ordered[j].ScopeKey })
	for _, source := range ordered {
		if source.ScopeKey != "doc-acl:"+domain.TextID(document) && source.ScopeKey != "publication:"+domain.TextID(document) {
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{source}); err != nil {
				return err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		if gate.Epoch != source.Epoch {
			return domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
		pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
		if err != nil {
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
		}
		if gate.State == "BLOCKED" {
			drained, err := repository.RetainedClosedSourceDrained(ctx, tx, source.ScopeKey, source.Epoch, document)
			if err != nil {
				return err
			}
			if !drained {
				return domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
			}
		} else if gate.State != "OPEN" {
			return domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
	}
	return nil
}

func retainedGovernanceProgress(operation, state string, document uint64) map[string]any {
	value := OperationResult(operation, state, document)
	if state != "SUCCEEDED" {
		value["poll_url"] = "/v1/operations/" + operation
	}
	return value
}

func (s *RetainedGovernance) Lookup(ctx context.Context, operation string) (repository.RetainedGovernanceRecord, error) {
	return repository.RetainedGovernanceRow(ctx, s.db, operation, false)
}

func (s *RetainedGovernance) Progress(actor Actor, operation string, document uint64) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		row, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
		if err != nil {
			return nil, err
		}
		if row.Plan.Purge.Actor != actor.ID || row.Plan.Purge.Document != document {
			return nil, domain.Violation(domain.PermissionDenied, "RETAINED_ORIGINAL_ACTOR_REQUIRED")
		}
		receipt, err := repository.LockedOperation(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if receipt.ActorID != actor.ID || receipt.Kind != domain.RetainedGovernanceKind(row.Plan.Action) || json.Unmarshal(receipt.Result, &result) != nil {
			return nil, domain.Violation(domain.StateConflict, "RETAINED_RECEIPT_CHANGED")
		}
		return result, nil
	}
}

// retained_governance_read.go：保留原实现，按业务合并文件。

// 治理页面只取得孤立留存的元数据和当前控制修订。使用真实平台资格登记输出，
// 不为已关闭的Doc/KB来源创建正文read admission，也不重新开放协作能力。
func (s *RetainedGovernance) Metadata(document uint64) ReadBody {
	return func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		facts, err := repository.TrashDocumentRow(ctx, tx, document, false)
		if err != nil {
			return nil, err
		}
		spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
		if err != nil {
			return nil, err
		}
		space, err := repository.TrashSpaceRow(ctx, tx, spaceID, false)
		if err != nil {
			return nil, err
		}
		var kb *dto.KnowledgeBase
		if facts.Document.KnowledgeBaseId != nil {
			id, err := domain.CanonicalID(*facts.Document.KnowledgeBaseId)
			if err != nil {
				return nil, err
			}
			value, err := repository.TrashKnowledgeBaseRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			kb = &value
		}
		// PURGE的前置事实包含仍发布及已撤下两种合法留存；ACTIVE/CLOSING/PURGED
		// 均不由这个入口返回。实际写入还会在事务锁下重新比较这份控制修订。
		if err = domain.ValidateRetainedGovernance(facts, space, kb, "PURGE", facts.Document.ControlRevision); err != nil {
			return nil, err
		}
		return facts.Document, nil
	}
}
````

UTF-8/LF全文SHA-256：`cb0db53b973b1a6a98f5ee9938e2cfd109a3788cf05e07e6a5235745e1e48ba4`。[参考实现](../../../../services/document/internal/logic/document/retained_governance_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
