package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"
)

var ErrPurgeIdle = errors.New("PURGE_IDLE")

type Purges struct {
	db           *sql.DB
	trash        *Trash
	cleaner      AccountDocumentCleaner
	instance     string
	expiryCursor atomic.Uint64
}

func NewPurges(db *sql.DB, trash *Trash, cleaner AccountDocumentCleaner, instance string) *Purges {
	return &Purges{db: db, trash: trash, cleaner: cleaner, instance: instance}
}

// POST只登记不可逆固定清理任务，不在三秒HTTP内执行Node/物理清理。handler必须
// 使用标准Replay/PresentedMutation，原号fingerprint与输出生命周期仍由它负责。
func (s *Purges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.PurgeAction) (Actor, MutationBody, MutationPresentation, error) {
	if original, err := repository.PurgeRow(ctx, s.db, input.OperationId, false); err == nil {
		if original.Plan.Actor != actor.ID || original.Plan.Document != id || original.Plan.Automatic {
			return actor, nil, nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		actor.Sources = actor.Proof.Sources
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, nil, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return actor, nil, nil, err
	}
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, nil, err
	}
	space, err := s.trash.spaceAuthority(ctx, actor, spaceID, "PURGE_DOCUMENT", id)
	if err != nil {
		return actor, nil, nil, err
	}
	access, err := s.trash.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, nil, err
	}
	if err := domain.ValidatePurgeIntent(facts, input.Confirmed, input.ExpectedControlRevision, false); err != nil {
		return actor, nil, nil, err
	}
	plan, err := s.plan(ctx, facts, input.OperationId, actor.ID, false)
	if err != nil {
		return actor, nil, nil, err
	}
	sources, err := trashSources(actor, space, access, plan.ACL, plan.Publication)
	if err != nil {
		return actor, nil, nil, err
	}
	var closedKB uint64
	if access.KB != nil {
		if err = domain.ValidatePurgeParent(*access.KB); err != nil {
			return actor, nil, nil, err
		}
		if access.KB.Lifecycle == "CLOSED" {
			closedKB = plan.KnowledgeBase
		}
	}
	// 明确非nil空列表让标准Mutation不提前持identity/session锁；run负责一次
	// 排序全部真实来源。最小receipt输出仍由标准Outputs.Register锁身份来源。
	actor.Sources = []*pb.SourceEpoch{}
	executed := false
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := lockPurgeSources(ctx, tx, sources, closedKB); err != nil {
			return nil, err
		}
		current, currentKB, err := s.trash.validate(ctx, tx, actor, space, access)
		if err != nil {
			return nil, err
		}
		if currentKB != nil && currentKB.Lifecycle != access.KB.Lifecycle {
			return nil, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeIntent(current, input.Confirmed, input.ExpectedControlRevision, false); err != nil {
			return nil, err
		}
		if current.Document.PublicationEpoch != facts.Document.PublicationEpoch || current.Document.VisibilityRevision != facts.Document.VisibilityRevision || current.Sequence != facts.Sequence {
			return nil, domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
		}
		if err = registerPurge(ctx, tx, plan); err != nil {
			return nil, err
		}
		executed = true
		return purgeProgress(plan.Operation, "PREPARING", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		// 同号竞态跳过run时仅输出标准fingerprint核验过的原最小receipt，
		// 没有拿next-epoch开门，也不把原动作重新当成新权限。
		if !executed {
			return value, actor.Proof.Sources, nil
		}
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, nil, err
		}
		if current.Document.AclRevision != domain.TextID(plan.ACL) || current.Document.PublicationEpoch != domain.TextID(plan.Publication) {
			return nil, nil, domain.Violation(domain.StateConflict, "PURGE_SOURCE_CHANGED")
		}
		return value, actor.Proof.Sources, nil
	}
	return actor, run, present, nil
}

func (s *Purges) plan(ctx context.Context, facts domain.TrashFacts, operation string, actor uint64, automatic bool) (domain.PurgePlan, error) {
	acl, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+facts.Document.Id)
	if err != nil {
		return domain.PurgePlan{}, err
	}
	publication, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+facts.Document.Id)
	if err != nil {
		return domain.PurgePlan{}, err
	}
	var kb uint64
	if facts.Document.KnowledgeBaseId != nil {
		kb, err = domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return domain.PurgePlan{}, err
		}
	}
	return domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)
}

func registerPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	if err := repository.RegisterPurge(ctx, tx, plan); err != nil {
		return err
	}
	if err := repository.BeginPurgeDocument(ctx, tx, plan); err != nil {
		return err
	}
	for _, scope := range plan.Scopes {
		if err := QueueClosure(ctx, tx, scope.Operation, plan.Actor, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: plan.Document, Action: "PURGE_DOCUMENT"}); err != nil {
			return err
		}
	}
	payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: plan.Operation, Type: "DOCUMENT_UNPUBLISHED", Document: plan.Document, Sequence: plan.Sequence, PublicationEpoch: plan.Publication, OccurredAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, "DOCUMENT_UNPUBLISHED", payload)
}

func purgeProgress(operation, state string, document uint64) map[string]any {
	value := OperationResult(operation, state, document)
	if state != "SUCCEEDED" {
		value["poll_url"] = "/v1/operations/" + operation
	}
	return value
}
