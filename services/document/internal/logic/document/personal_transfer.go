package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。
type PersonalTransferStorage struct {
	Operation       string
	Document, Bytes uint64
	Digest          []byte
}
type PersonalTransferStorageReader interface {
	MeasurePersonalTransfer(context.Context, domain.PersonalTransferPlan) (PersonalTransferStorage, error)
}

type PersonalTransfers struct {
	db         *sql.DB
	identity   *Identity
	access     *Access
	pagination *Pagination
	fences     *Fences
	storage    PersonalTransferStorageReader
	quotaLimit uint64
	instance   string
}

func NewPersonalTransfers(db *sql.DB, identity *Identity, access *Access, pagination *Pagination, fences *Fences, storage PersonalTransferStorageReader, quotaLimit uint64, instance string) *PersonalTransfers {
	return &PersonalTransfers{db: db, identity: identity, access: access, pagination: pagination, fences: fences, storage: storage, quotaLimit: quotaLimit, instance: instance}
}

func (s *PersonalTransfers) Ready() bool {
	return s != nil && s.db != nil && s.identity != nil && s.access != nil && s.pagination != nil && s.fences != nil && s.storage != nil && s.quotaLimit > 0 && s.instance != ""
}

func (s *PersonalTransfers) Offer(ctx context.Context, actor Actor, id uint64, input dto.OwnerChange) (Actor, MutationBody, error) {
	target, err := domain.CanonicalID(input.TargetUserId)
	if err != nil {
		return actor, nil, err
	}
	if target == actor.ID {
		return actor, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_TO_SELF")
	}
	access, err := s.access.Authorize(ctx, actor, id, "PERSONAL_TRANSFER_OFFER")
	if err != nil {
		return actor, nil, err
	}
	spaceID, _ := domain.CanonicalID(access.Document.SpaceId)
	space, proof, err := s.access.SpaceIdentity(ctx, actor, spaceID, pb.IdentityCapability_PERSONAL_SPACE_OWNER, "PERSONAL_TRANSFER_OFFER", id)
	if err != nil {
		return actor, nil, err
	}
	if space.Kind != "PERSONAL" || space.Lifecycle != "ACTIVE" || access.Document.Lifecycle != "ACTIVE" || access.Document.ResponsibleUserId == nil || *access.Document.ResponsibleUserId != domain.TextID(actor.ID) {
		return actor, nil, domain.Violation(domain.PermissionDenied, "PERSONAL_DOCUMENT_OWNER_REQUIRED")
	}
	receiver, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: target}, input.OperationId, &pb.Target{DocumentId: id, Purpose: "PERSONAL_TRANSFER_OFFER"})
	if err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, proof); err != nil {
		return actor, nil, err
	}
	if err = s.identity.Revalidate(ctx, receiver); err != nil {
		return actor, nil, err
	}
	actor.Sources, err = MergeSources(append(access.Proofs, proof, receiver)...)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = append(actor.Sources, documentSources(access)...)
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.Lifecycle != "ACTIVE" || current.ControlRevision != input.ExpectedRevision {
			return nil, domain.Violation(domain.StateConflict, "CONTROL_REVISION_MISMATCH")
		}
		space, err := repository.SpaceRow(ctx, tx, spaceID, true)
		if err != nil {
			return nil, err
		}
		if space.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
		}
		if err = repository.TransferInterlock(ctx, tx, id); err != nil {
			return nil, err
		}
		revision, _ := domain.CanonicalRevision(input.ExpectedRevision)
		offer, err := repository.InsertPersonalTransfer(ctx, tx, domain.PersonalTransfer{Document: id, Title: current.Title, SourceUser: actor.ID, TargetUser: target, SourceSpace: spaceID, ExpectedControl: revision, OfferOperation: input.OperationId})
		if err != nil {
			return nil, err
		}
		// 申请由受本人签名的 incoming 列表读取；成功 receipt 的 entity 是 offer。
		// 既有负责人通知只能在完成迁移后发送，不能提前宣称接收方已经成为 owner。
		return OperationResult(input.OperationId, "SUCCEEDED", offer), nil
	}
	return actor, run, nil
}

func documentSources(access DocumentAccess) []*pb.SourceEpoch {
	acl, _ := domain.CanonicalID(access.Document.AclRevision)
	result := []*pb.SourceEpoch{{ScopeKey: "doc-acl:" + access.Document.Id, Epoch: acl}}
	if access.Document.KnowledgeBaseId != nil {
		result = append(result, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *access.Document.KnowledgeBaseId, Epoch: access.KBRevision})
	}
	return result
}

func transferMetadata(p domain.PersonalTransfer) dto.PersonalTransferMetadata {
	state := p.State
	if state == "PENDING" && !p.Expires.After(p.Now) {
		state = "EXPIRED"
	}
	result := dto.PersonalTransferMetadata{Id: domain.TextID(p.ID), DocumentId: domain.TextID(p.Document), DocumentTitle: p.Title, SourceUserId: domain.TextID(p.SourceUser), TargetUserId: domain.TextID(p.TargetUser), SourceSpaceId: domain.TextID(p.SourceSpace), ExpectedControlRevision: domain.TextID(p.ExpectedControl), Revision: domain.TextID(p.Revision), State: state, ExpiresAt: p.Expires.UTC().Format(time.RFC3339Nano)}
	if p.AcceptOperation != "" {
		result.AcceptOperationId = &p.AcceptOperation
		space, kb := domain.TextID(p.Plan.TargetSpace), domain.TextID(p.Plan.TargetKB)
		result.TargetSpaceId = &space
		result.TargetKnowledgeBaseId = &kb
	}
	return result
}

func transferParticipant(p domain.PersonalTransfer, actor uint64) error {
	if actor != p.SourceUser && actor != p.TargetUser {
		return domain.Violation(domain.PermissionDenied, "TRANSFER_PARTICIPANT_REQUIRED")
	}
	return nil
}

func (s *PersonalTransfers) Detail(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	p, err := repository.PersonalTransferRow(ctx, s.db, id, false)
	if err != nil {
		return nil, nil, err
	}
	if err = transferParticipant(p, actor.ID); err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		p, err := repository.PersonalTransferRow(ctx, tx, id, false)
		if err != nil {
			return nil, err
		}
		if err = transferParticipant(p, actor.ID); err != nil {
			return nil, err
		}
		return transferMetadata(p), nil
	}
	return actor.Proof.Sources, run, nil
}

func (s *PersonalTransfers) List(ctx context.Context, actor Actor, direction, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	if direction != "incoming" && direction != "outgoing" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_DIRECTION_INVALID")
	}
	binding := "personal-transfers:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef) + ":" + direction
	limit, lastText, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var last uint64
	if lastText != "" {
		last, err = domain.CanonicalID(lastText)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		ids, err := repository.PersonalTransferIDs(ctx, tx, actor.ID, direction, last, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.PersonalTransferPage{Items: []dto.PersonalTransferMetadata{}}
		more := len(ids) > limit
		if more {
			ids = ids[:limit]
		}
		for _, id := range ids {
			p, err := repository.PersonalTransferRow(ctx, tx, id, false)
			if err != nil {
				return nil, err
			}
			if err = transferParticipant(p, actor.ID); err != nil {
				return nil, err
			}
			page.Items = append(page.Items, transferMetadata(p))
		}
		if more {
			page.NextCursor = s.pagination.Next(binding, domain.TextID(ids[len(ids)-1]))
		}
		return page, nil
	}
	return actor.Proof.Sources, run, nil
}

func (s *PersonalTransfers) Decide(ctx context.Context, actor Actor, id uint64, input dto.VersionedAction, action string) (Actor, MutationBody, error) {
	revision, err := domain.CanonicalRevision(input.ExpectedRevision)
	if err != nil {
		return actor, nil, err
	}
	if action != "CANCEL" && action != "REJECT" {
		return actor, nil, domain.Violation(domain.InvalidRequest, "TRANSFER_ACTION_INVALID")
	}
	actor.Sources = actor.Proof.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		p, err := repository.PersonalTransferRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePersonalTransferAction(p, actor.ID, revision, action); err != nil {
			return nil, err
		}
		state := "CANCELLED"
		if action == "REJECT" {
			state = "REJECTED"
		}
		if err = repository.ClosePersonalTransfer(ctx, tx, id, revision, state); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	return actor, run, nil
}

func (s *PersonalTransfers) Accept(ctx context.Context, actor Actor, id uint64, input dto.PersonalTransferAccept) (Actor, MutationBody, MutationPresentation, error) {
	if !s.Ready() {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "PERSONAL_TRANSFER_NOT_READY")
	}
	offer, err := repository.PersonalTransferRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	revision, err := domain.CanonicalRevision(input.ExpectedRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	if err = domain.ValidatePersonalTransferAction(offer, actor.ID, revision, "ACCEPT"); err != nil {
		return actor, nil, nil, err
	}
	kb, err := domain.CanonicalID(input.TargetKnowledgeBaseId)
	if err != nil {
		return actor, nil, nil, err
	}
	expectedQuota, err := domain.CanonicalRevision(input.ExpectedQuotaRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	current, err := repository.DocumentRow(ctx, s.db, offer.Document, false)
	if err != nil {
		return actor, nil, nil, err
	}
	targetKB, err := repository.CreationKnowledgeBase(ctx, s.db, kb, false)
	if err != nil {
		return actor, nil, nil, err
	}
	targetSpace, targetACL, kbState := targetKB.Space, targetKB.Revision, targetKB.Lifecycle
	targetFacts, _, err := s.access.SpaceIdentity(ctx, actor, targetSpace, pb.IdentityCapability_PERSONAL_SPACE_OWNER, "PERSONAL_TRANSFER_ACCEPT", offer.Document)
	if err != nil {
		return actor, nil, nil, err
	}
	if targetFacts.Kind != "PERSONAL" || targetSpace == offer.SourceSpace || kbState != "ACTIVE" {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "TRANSFER_TARGET_PERSONAL_SPACE_REQUIRED")
	}
	targetProof, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_PERSONAL_SPACE_OWNER, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: actor.ID}, input.OperationId, &pb.Target{DocumentId: offer.Document, Purpose: "PERSONAL_TRANSFER_ACCEPT"})
	if err != nil {
		return actor, nil, nil, err
	}
	sourceProof, err := s.identity.AcquireBound(ctx, actor, pb.IdentityCapability_ACTIVE_ACCOUNT, &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: offer.SourceUser}, input.OperationId, &pb.Target{DocumentId: offer.Document, Purpose: "PERSONAL_TRANSFER_ACCEPT"})
	if err != nil {
		return actor, nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, targetProof); err != nil {
		return actor, nil, nil, err
	}
	if err = s.identity.Revalidate(ctx, sourceProof); err != nil {
		return actor, nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, targetProof, sourceProof)
	if err != nil {
		return actor, nil, nil, err
	}
	acl, _ := domain.CanonicalRevision(current.AclRevision)
	publication, _ := domain.CanonicalRevision(current.PublicationEpoch)
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + current.Id, Epoch: acl}, &pb.SourceEpoch{ScopeKey: "publication:" + current.Id, Epoch: publication}, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(kb), Epoch: targetACL})
	if current.KnowledgeBaseId != nil {
		sourceKB, err := domain.CanonicalID(*current.KnowledgeBaseId)
		if err != nil {
			return actor, nil, nil, err
		}
		epoch, err := repository.KnowledgeBaseACL(ctx, s.db, sourceKB)
		if err != nil {
			return actor, nil, nil, err
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *current.KnowledgeBaseId, Epoch: epoch})
	}
	actor.Sources = sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		doc, err := repository.DocumentRow(ctx, tx, offer.Document, true)
		if err != nil {
			return nil, err
		}
		p, err := repository.PersonalTransferRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePersonalTransferAction(p, actor.ID, revision, "ACCEPT"); err != nil {
			return nil, err
		}
		if doc.Lifecycle != "ACTIVE" || doc.ControlRevision != domain.TextID(p.ExpectedControl) || doc.SpaceId != domain.TextID(p.SourceSpace) || doc.ResponsibleUserId == nil || *doc.ResponsibleUserId != domain.TextID(p.SourceUser) || doc.AclRevision != current.AclRevision || doc.PublicationEpoch != current.PublicationEpoch {
			return nil, domain.Violation(domain.StateConflict, "TRANSFER_DOCUMENT_CHANGED")
		}
		if err = repository.TransferInterlock(ctx, tx, p.Document); err != nil {
			return nil, err
		}
		if p.ExpectedControl == ^uint64(0) || acl == ^uint64(0) || publication == ^uint64(0) {
			return nil, domain.Violation(domain.StateConflict, "TRANSFER_REVISION_EXHAUSTED")
		}
		if err = lockTransferParents(ctx, tx, p.SourceSpace, targetSpace, kb, actor.ID); err != nil {
			return nil, err
		}
		_, target, err := resources.LockTransferQuotas(ctx, tx, p.SourceSpace, targetSpace, s.quotaLimit)
		if err != nil {
			return nil, err
		}
		fixed, amount, err := resources.PrepareTransferResources(ctx, tx, p.Document, p.SourceSpace, targetSpace)
		if err != nil {
			return nil, err
		}
		body, paid, err := repository.TransferBodyCapacity(ctx, tx, p.Document)
		if err != nil {
			return nil, err
		}
		amount, err = domain.TransferCapacity(amount, body)
		if err != nil {
			return nil, err
		}
		if err = resources.ReserveTransferCapacity(ctx, tx, target, amount, expectedQuota); err != nil {
			return nil, err
		}
		sourceIdentity, _ := proto.Marshal(sourceProof)
		receiverIdentity, _ := proto.Marshal(targetProof)
		plan := domain.PersonalTransferPlan{Operation: input.OperationId, Transfer: p.ID, Document: p.Document, SourceUser: p.SourceUser, TargetUser: p.TargetUser, SourceSpace: p.SourceSpace, TargetSpace: targetSpace, TargetKB: kb, ExpectedControl: p.ExpectedControl, ClosingControl: p.ExpectedControl + 1, ACL: acl, Publication: publication, BodyBytes: body, SourceBodyBytes: paid, ReservedBytes: amount, Resources: fixed, SourceIdentity: sourceIdentity, ReceiverIdentity: receiverIdentity, ReceiverSession: actor.Proof.SessionRef}
		for _, v := range []struct {
			scope string
			epoch uint64
		}{{"doc-acl:" + doc.Id, acl}, {"publication:" + doc.Id, publication}} {
			plan.Scopes = append(plan.Scopes, domain.PurgeScope{Key: v.scope, Epoch: v.epoch, Operation: domain.PersonalTransferFence(input.OperationId, v.scope)})
		}
		if err = repository.BeginPersonalTransfer(ctx, tx, plan); err != nil {
			return nil, err
		}
		for _, scope := range plan.Scopes {
			if err = QueueClosure(ctx, tx, scope.Operation, actor.ID, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: p.Document, Action: "PERSONAL_TRANSFER"}); err != nil {
				return nil, err
			}
		}
		if err = queueVisibilityIndexEvents(ctx, tx, input.OperationId, []uint64{p.Document}, 0); err != nil {
			return nil, err
		}
		return purgeProgress(input.OperationId, "PREPARING", p.Document), nil
	}
	// 旧 Doc 来源已经被本事务关闭，最小原号输出只绑定真实 receiver session。
	present := func(_ context.Context, _ *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		return value, actor.Proof.Sources, nil
	}
	return actor, run, present, nil
}

func lockTransferParents(ctx context.Context, tx *sql.Tx, source, target, kb, user uint64) error {
	facts, err := repository.LockedCreationKnowledgeBase(ctx, tx, kb)
	if err != nil {
		return err
	}
	if facts.Space != target || facts.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "TRANSFER_TARGET_KB_CHANGED")
	}
	ids := []uint64{source, target}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		s, err := repository.SpaceRow(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if s.Kind != "PERSONAL" || s.Lifecycle != "ACTIVE" {
			return domain.Violation(domain.StateConflict, "TRANSFER_SPACE_CHANGED")
		}
		if id == target && (s.OwnerUserId == nil || *s.OwnerUserId != domain.TextID(user)) {
			return domain.Violation(domain.PermissionDenied, "PERSONAL_SPACE_NOT_OWNED")
		}
	}
	return nil
}

var ErrPersonalTransferIdle = errors.New("PERSONAL_TRANSFER_IDLE")
