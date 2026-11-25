// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"time"
)

// visibility.go：保留原实现，按业务合并文件。

type Visibilities struct {
	knowledgeBases *KnowledgeBases
	access         *Access
}

func NewVisibilities(knowledgeBases *KnowledgeBases, access *Access) *Visibilities {
	return &Visibilities{knowledgeBases: knowledgeBases, access: access}
}
func (s *Visibilities) Prepare(ctx context.Context, actor Actor, id uint64, isKnowledgeBase bool, input dto.VisibilityChange) (Actor, MutationBody, error) {
	var doc DocumentAccess
	var kb KnowledgeBaseAccess
	var err error
	scope := "doc-acl:" + domain.TextID(id)
	if isKnowledgeBase {
		kb, err = s.knowledgeBases.Manage(ctx, actor, id, "CHANGE_VISIBILITY", false)
		actor.Sources = kb.Sources
		scope = "kb-acl:" + domain.TextID(id)
	} else {
		doc, err = s.access.Authorize(ctx, actor, id, "CHANGE_VISIBILITY")
		if err == nil && !doc.Permissions.Manage {
			err = domain.Violation(domain.PermissionDenied, "DOCUMENT_MANAGEMENT_REQUIRED")
		}
		actor.Sources = doc.Sources
	}
	if err != nil {
		return actor, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		var revision, epoch string
		if isKnowledgeBase {
			current, err := ValidateKnowledgeBase(ctx, tx, kb, kb.KB.AclRevision)
			if err != nil {
				return nil, err
			}
			revision = current.VisibilityRevision
			epoch = current.AclRevision
		} else {
			current, err := ValidateAccess(ctx, tx, doc)
			if err != nil {
				return nil, err
			}
			revision = current.VisibilityRevision
			epoch = current.AclRevision
		}
		if revision != input.ExpectedVisibilityRevision {
			return nil, domain.Violation(domain.StateConflict, "VISIBILITY_REVISION_MISMATCH")
		}
		documents := []uint64{id}
		knowledgeBase := uint64(0)
		if isKnowledgeBase {
			knowledgeBase = id
			documents, err = repository.VisibilityDocumentIDs(ctx, tx, id)
			if err != nil {
				return nil, err
			}
		}
		if err := repository.UpdateVisibility(ctx, tx, id, isKnowledgeBase, input.Policy); err != nil {
			return nil, err
		}
		if err := queueVisibilityIndexEvents(ctx, tx, input.OperationId, documents, knowledgeBase); err != nil {
			return nil, err
		}
		closed, _ := domain.CanonicalID(epoch)
		if err := QueueClosure(ctx, tx, input.OperationId, actor.ID, domain.Closure{Scope: scope, Epoch: closed, Entity: id, Action: "VISIBILITY"}); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "PREPARING", id), nil
	}
	return actor, run, nil
}

// 保留可见范围动作原有固定投影：关闭未确认时显示固定两参与者，重试仍走完整原号投影。
func (s *Authorizations) VisibilityPresentation(actor Actor, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		facts, err := repository.AuthorizationProjection(ctx, tx, operation, actor.ID)
		if err != nil {
			return nil, nil, err
		}
		value := dto.AuthorizationResult{OperationId: operation, State: facts.State, ScopeKey: facts.Scope, Epoch: facts.Epoch, MissingParticipants: []string{}}
		if value.State != "SUCCEEDED" {
			value.MissingParticipants = []string{"edge", "collaboration"}
		}
		return value, actor.Proof.Sources, nil
	}
}

// visibility_indexing.go：保留原实现，按业务合并文件。

// 一个KB动作的全部事件与政策、关闭任务和receipt共同提交。原号重放由外层receipt
// 跳过业务run；不另起事务，也不在KB政策已成功后才异步选择可能变化的子文档范围。
func queueVisibilityIndexEvents(ctx context.Context, tx *sql.Tx, operation string, documents []uint64, knowledgeBase uint64) error {
	occurred := time.Now().UTC()
	for _, document := range documents {
		facts, err := repository.LockedVisibilityIndexFacts(ctx, tx, document, knowledgeBase)
		if err != nil {
			return err
		}
		next, err := domain.NextIndexEventSequence(facts.Sequence)
		if err != nil {
			return err
		}
		eventID, err := domain.VisibilityIndexEventID(operation, document)
		if err != nil {
			return err
		}
		payload, err := domain.EncodeIndexEvent(domain.IndexEvent{
			ID: eventID, Type: "DOCUMENT_VISIBILITY_CHANGED", Document: document,
			Sequence: next, Release: facts.Release, PublicationEpoch: facts.PublicationEpoch,
			OccurredAt: occurred,
		})
		if err != nil {
			return err
		}
		if err = repository.AdvanceVisibilitySequence(ctx, tx, document, facts.Sequence, next); err != nil {
			return err
		}
		if err = repository.InsertIndexEvent(ctx, tx, document, next, eventID, "DOCUMENT_VISIBILITY_CHANGED", payload); err != nil {
			return err
		}
	}
	return nil
}
