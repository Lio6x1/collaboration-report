// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// discard_draft.go：保留原实现，按业务合并文件。

type discardPreviewReview struct {
	Guard   domain.ReviewGuard
	Pointer domain.ReviewPointer
}

type discardDraftContext struct {
	Actor          uint64
	SessionExpires int64
	Intent         domain.DiscardDraftIntent
	Access         DocumentAccess
	Context        *pb.OperationContext
	Scope          draftSourceScope
	Preview        discardPreviewContext
	PreviewResult  dto.DiscardPreview
	PreviewDigest  []byte
	Review         discardPreviewReview
}

type discardDraftInstall struct {
	draftInstall
	Fixed discardDraftContext
}

func (s *DraftControls) LookupDiscard(ctx context.Context, operation string) (discardDraftContext, error) {
	var fixed discardDraftContext
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_DRAFT_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation || fixed.Context.Target.Purpose != "DISCARD_DRAFT" {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	return fixed, nil
}

// discard_draft_commit.go：保留原实现，按业务合并文件。

func (s *DraftControls) commitDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, prepared draftPrepared, proofs, original []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validateDiscard(ctx, tx, fixed, proofs, original); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	scope := fixed.Scope
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resources := map[uint64]bool{}
	for _, content := range []*pb.FixedContent{prepared.Main, prepared.Candidate} {
		for _, id := range content.ResourceIds {
			resources[id] = true
		}
	}
	ids := make([]uint64, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, e := repository.LockedDocumentResource(ctx, tx, id, scope.Document, space)
		if e != nil {
			return e
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "DISCARD_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	version, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
	if err != nil {
		return err
	}
	lineage, err := NewEntityID()
	if err != nil {
		return err
	}
	decoded, err := codec.Decode(prepared.Candidate.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := decoded.(map[string]any)
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(scope.Manifest), domain.TextID(version)
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertDiscardLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, scope.Manifest, version, task.Operation, prepared.Preview, raw); err != nil {
		return err
	}
	epoch := scope.Workspace.Epoch + 1
	room, err := collaborationRoom(ctx, tx, "BRANCH", scope.Workspace.Workspace, scope.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch, content.LineageRef, content.ProvenanceJsonUtf8 = epoch, domain.TextID(lineage), raw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := discardDraftInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(scope.Manifest), "base_version_id": domain.TextID(version), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
		return err
	}
	// MAIN正式指针保持原值，意见和resumable也保持；安装责任与成功事实在同一事务登记。
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompleteDiscardDraft(ctx, tx, task, "SUCCEEDED", result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_compute.go：保留原实现，按业务合并文件。

// 派生规则与Node相同且可逐字节核验：只重绑本次操作来源，不采样、不改原预览、不改旧接纳来源。
func deriveDiscardContent(value map[string]any, old, operation, suffix string) (*pb.FixedContent, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	encodedProvenance, ok := value["provenance_json_utf8"].(string)
	if !ok {
		return nil, invalid
	}
	raw, err := base64.StdEncoding.DecodeString(encodedProvenance)
	if err != nil {
		return nil, err
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	provenance, ok := decoded.(map[string]any)
	if !ok {
		return nil, invalid
	}
	provenance["source_operation_id"] = operation
	sources, ok := provenance["block_sources"].([]any)
	if !ok {
		return nil, invalid
	}
	for _, item := range sources {
		source, ok := item.(map[string]any)
		if !ok {
			return nil, invalid
		}
		if source["origin_capture_ref"] == old {
			source["origin_capture_ref"] = operation
		}
	}
	encoded, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	value["operation_id"], value["capture_id"], value["provenance_json_utf8"] = operation, operation+suffix, base64.StdEncoding.EncodeToString(encoded)
	contentRaw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return decodePullFixedContent(contentRaw)
}

func (s *DraftControls) expectedDiscard(ctx context.Context, fixed discardDraftContext) (map[string]any, draftPrepared, error) {
	var expected draftPrepared
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_PREVIEW_FIXED", fixed.Intent.Preview)
	if err != nil {
		return nil, expected, err
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		return nil, expected, err
	}
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	value, ok := decoded.(map[string]any)
	if !ok {
		return nil, expected, invalid
	}
	digestText, ok := value["input_digest"].(string)
	if !ok {
		return nil, expected, invalid
	}
	digest, err := base64.StdEncoding.DecodeString(digestText)
	if err != nil || !bytes.Equal(digest, fixed.PreviewDigest) {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	delete(value, "input_digest")
	original, err := codec.Canonical(value)
	if err != nil {
		return nil, expected, err
	}
	hash := sha256.Sum256(original)
	if !bytes.Equal(hash[:], digest) {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return nil, expected, err
	}
	value["preview_binding"] = value["binding"]
	value["binding"] = map[string]any{"context": wire, "expected_branch_id": domain.TextID(fixed.Scope.Workspace.Workspace), "expected_structure_id": domain.TextID(fixed.Scope.Structure), "expected_lineage_id": domain.TextID(fixed.Scope.Workspace.Lineage)}
	value["preview_input_digest"] = base64.StdEncoding.EncodeToString(digest)
	value["expected_round_id"] = domain.TextID(fixed.Intent.Round)
	for key, suffix := range map[string]string{"draft": "", "main": ":main", "candidate": ":candidate"} {
		part, ok := value[key].(map[string]any)
		if !ok {
			return nil, expected, invalid
		}
		content, err := deriveDiscardContent(part, fixed.Intent.Preview, fixed.Intent.Operation, suffix)
		if err != nil {
			return nil, expected, err
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); err != nil {
			return nil, expected, err
		}
		if key == "main" {
			expected.Main = content
		}
		if key == "candidate" {
			expected.Candidate = content
		}
	}
	encoded, err := codec.Canonical(value)
	if err != nil {
		return nil, expected, err
	}
	derived := sha256.Sum256(encoded)
	expected.Digest = derived[:]
	expected.Preview = fixed.Intent.Preview
	cutRaw, _ := json.Marshal(value["draft_cut"])
	if json.Unmarshal(cutRaw, &expected.Cut) != nil {
		return nil, expected, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_CUT_UNKNOWN")
	}
	return value, expected, nil
}

func (s *DraftControls) prepareDiscard(ctx context.Context, fixed discardDraftContext) (draftPrepared, error) {
	_, expected, err := s.expectedDiscard(ctx, fixed)
	if err != nil {
		return expected, err
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err := s.client.Prepare(ctx, map[string]any{"context": wire, "expected_lineage_id": domain.TextID(fixed.Scope.Workspace.Lineage), "expected_round_id": domain.TextID(fixed.Intent.Round), "expected_attempt_id": fixed.Intent.Preview, "expected_input_digest": fixed.PreviewDigest})
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	var result draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &result.State) != nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
	}
	if result.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &result.Code) != nil || result.Code == "" {
			return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
		}
		return result, domain.Violation(domain.StateConflict, result.Code)
	}
	if result.State != "PREPARED" {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARE_UNKNOWN")
	}
	result.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return result, err
	}
	result.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return result, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &result.Preview) != nil || json.Unmarshal(parts["input_digest"], &result.Digest) != nil || json.Unmarshal(parts["draft_cut"], &result.Cut) != nil || result.Preview != expected.Preview || !bytes.Equal(result.Digest, expected.Digest) || !proto.Equal(result.Candidate, expected.Candidate) || !proto.Equal(result.Main, expected.Main) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARED_BINDING_UNKNOWN")
	}
	observed, _ := codec.Canonical(result.Cut)
	wanted, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(observed, wanted) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREPARED_BINDING_UNKNOWN")
	}
	return result, nil
}

// discard_draft_install.go：保留原实现，按业务合并文件。

func (s *DraftControls) DiscardCommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.DiscardOperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_DRAFT_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install discardDraftInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest), "expected_round_id": domain.TextID(install.Fixed.Intent.Round)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

func (s *DraftControls) reconcileDiscard(ctx context.Context, task domain.SubmissionTask) error {
	var install discardDraftInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.DiscardOperationResult(ctx, query)
	if err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_INSTALL_ACK_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, query.Target.DocumentId, true); err != nil {
		return err
	}
	owner, err := repository.LockedBranchControl(ctx, tx, install.Branch)
	if err != nil {
		return err
	}
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if authority.State == pb.OperationResult_SUCCEEDED {
		guard, err := repository.LockedReviewGuard(ctx, tx, install.Fixed.Scope.Unit)
		if err != nil {
			return err
		}
		if guard != install.Fixed.Review.Guard {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
		}
		if install.Fixed.Review.Pointer.ID != 0 {
			pointer, err := repository.LockedReviewPointer(ctx, tx, install.Fixed.Review.Pointer.ID)
			if err != nil {
				return err
			}
			if pointer != install.Fixed.Review.Pointer {
				return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ARCHIVE_FACT_CHANGED")
			}
		}
		// 此时只核验成功时固定事实，不恢复旧用户证明；注销不能取消已经成功的系统安装责任。
		if err = repository.ArchiveDiscardedReview(ctx, tx, task.Operation, install.Fixed.Actor, install.Fixed.Review.Pointer, install.Fixed.Intent.Changes); err != nil {
			return err
		}
		if err = repository.ActivateDiscardedBranch(ctx, tx, install.Branch, install.Epoch); err != nil {
			return err
		}
	}
	if err = repository.SetBranchControl(ctx, tx, install.Branch, ""); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_DRAFT_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_readiness.go：保留原实现，按业务合并文件。

func validatePendingDiscardDraftBinding(fact repository.PendingDiscardDraftBinding) (discardDraftContext, error) {
	var fixed discardDraftContext
	invalid := errors.New("DISCARD_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &fixed) != nil {
		return fixed, invalid
	}
	ctx, scope := fixed.Context, fixed.Scope
	if ctx == nil || ctx.Target == nil || fixed.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || fixed.Intent.Operation != fact.Operation || fact.Owner != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Manage || len(fixed.PreviewDigest) != 32 {
		return fixed, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "DISCARD_DRAFT"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || scope.Workspace.Lineage == 0 || !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" {
		return fixed, invalid
	}
	parsed, err := domain.ParseDiscardDraft(fixed.Intent.Input)
	if err != nil || parsed.Operation != fact.Operation || parsed.Preview != fixed.Intent.Preview || parsed.Branch != scope.Workspace.Workspace || parsed.Structure != scope.Structure || parsed.Round != fixed.Intent.Round {
		return fixed, invalid
	}
	a, _ := json.Marshal(parsed.Changes)
	b, _ := json.Marshal(fixed.Intent.Changes)
	if !bytes.Equal(a, b) {
		return fixed, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-draft"
	var httpTarget struct {
		Path string `json:"path"`
	}
	fingerprint, err := domain.OperationFingerprint(path, fixed.Intent.Input)
	if err != nil || json.Unmarshal(fact.Target, &httpTarget) != nil || httpTarget.Path != path || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return fixed, invalid
	}
	if fixed.Access.Document.Id != domain.TextID(scope.Document) || fixed.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) || fixed.Access.Document.CurrentStructureId != domain.TextID(scope.Structure) || fixed.Access.Document.CurrentStyleId != domain.TextID(scope.Style) || fixed.Access.Document.ControlRevision != domain.TextID(scope.Control) || !equalPullSources(ctx.Sources, fixed.Access.Sources) {
		return fixed, invalid
	}
	available := map[string]uint64{}
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return fixed, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fact.Operation || proof.Purpose != "DISCARD_DRAFT" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) {
			return fixed, invalid
		}
		if !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return fixed, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || available[source.ScopeKey] != source.Epoch || source.Epoch == 0 {
				return fixed, invalid
			}
		}
	}
	if fixed.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef || fixed.Preview.Context == nil || len(fixed.Preview.Access.Proofs) == 0 || fixed.Preview.Access.Proofs[0] == nil || !bytes.Equal(fixed.Access.Proofs[0].SessionRef, fixed.Preview.Access.Proofs[0].SessionRef) || fixed.Preview.Scope != scope || fixed.Preview.Actor != fixed.Actor || fixed.Preview.Intent.Operation != fixed.Intent.Preview {
		return fixed, invalid
	}
	previewRaw, _ := json.Marshal(fixed.Preview)
	previewTarget, _ := json.Marshal(map[string]string{"path": "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-previews"})
	if _, err = validateOriginalDiscardPreviewBinding(repository.PendingDiscardPreviewBinding{Operation: fixed.Intent.Preview, Actor: fixed.Actor, Original: previewRaw, Target: previewTarget, Fingerprint: fixed.Preview.Context.RequestFingerprint}); err != nil {
		return fixed, invalid
	}
	public := fixed.PreviewResult
	if public.PreviewCutRef != fixed.Intent.Preview || public.DocumentId != domain.TextID(scope.Document) || public.UnitId != domain.TextID(scope.Unit) || public.BranchId != domain.TextID(scope.Workspace.Workspace) || public.WorkspaceEpoch != domain.TextID(scope.Workspace.Epoch) || public.LineageId != domain.TextID(scope.Workspace.Lineage) || public.MainManifestId != domain.TextID(scope.Manifest) || public.StructureId != domain.TextID(scope.Structure) {
		return fixed, invalid
	}
	if _, err = time.Parse(time.RFC3339Nano, public.ExpiresAt); err != nil {
		return fixed, invalid
	}
	if fact.Kind == "DISCARD_DRAFT_RECOVER" {
		if fact.State != "PREPARING" || !bytes.Equal(fact.Original, fact.Payload) {
			return fixed, invalid
		}
	} else if fact.Kind == "DISCARD_DRAFT_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, invalid
		}
		var install discardDraftInstall
		if json.Unmarshal(fact.Payload, &install) != nil || install.Context == nil || install.Branch != scope.Workspace.Workspace || install.OldRoom != scope.Room || !proto.Equal(install.Context, ctx) {
			return fixed, invalid
		}
		raw, _ := json.Marshal(install.Fixed)
		originalFields, _ := json.Marshal(fixed)
		// MySQL JSON会调整字段顺序；核验完整已解析绑定，不把原字节的合法数据库编码误判为篡改。
		if !bytes.Equal(raw, originalFields) {
			return fixed, invalid
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == scope.Room || install.Epoch != scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Content == nil || install.Content.OperationId != fact.Operation) {
			return fixed, invalid
		}
	} else {
		return fixed, invalid
	}
	return fixed, nil
}

func validateDiscardAdmission(fixed discardDraftContext, actual repository.OriginalAdmissionBinding) error {
	if fixed.Context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "DISCARD_DRAFT" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return errors.New("DISCARD_ADMISSION_UNKNOWN")
	}
	return nil
}

func (s *DraftControls) readyDiscards(ctx context.Context) error {
	facts, err := repository.PendingDiscardDraftBindings(ctx, s.db)
	if err != nil {
		return errors.New("DISCARD_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingDiscardDraftBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardAdmission(fixed, admission) != nil {
			return errors.New("DISCARD_ADMISSION_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("DISCARD_ADMISSION_UNKNOWN")
		}
		original, err := s.LookupPreview(ctx, fixed.Intent.Preview)
		if err != nil {
			return err
		}
		a, _ := json.Marshal(original)
		b, _ := json.Marshal(fixed.Preview)
		if !bytes.Equal(a, b) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		previewAdmission, err := repository.OriginalAdmission(ctx, s.db, original.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardPreviewAdmission(original, previewAdmission) != nil {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		previewSources, err := repository.AdmissionSources(ctx, s.db, original.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(previewSources, original.Access.Sources) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		if _, _, err = s.expectedDiscard(ctx, fixed); err != nil {
			return err
		}
	}
	return nil
}

// discard_draft_readings.go：保留原实现，按业务合并文件。

func (s *DraftControls) DiscardProgress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupDiscard(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.previewResultAccess(ctx, actor, fixed.Scope.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fact, err := repository.DiscardDraftResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.Actor != actor.ID {
			return nil, domain.Violation(domain.EntityNotFound, "DISCARD_NOT_FOUND")
		}
		if !fact.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
		}
		var value map[string]any
		if json.Unmarshal(fact.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
		}
		installed, err := repository.DiscardDraftInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.State != "PREPARING" && !installed {
			// 公共结果到真实安装/归档完成后才终态；私有系统原结果保持不变供可靠收尾。
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

func (s *DraftControls) DiscardOperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_QUERY_INVALID")
	}
	fixed, err := s.LookupDiscard(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.DiscardDraftResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, State: pb.OperationResult_PROCESSING, Generation: fact.Generation}
	if fact.State == "PREPARING" {
		return result, nil
	}
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	result.State = pb.OperationResult_REJECTED
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
	}
	var value struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// discard_draft_recovery.go：保留原实现，按业务合并文件。

func (s *DraftControls) RecoverDiscardOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimDiscardDraftTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindingError := repository.ClaimedDiscardDraftBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingDiscardDraftBinding(fact)
	if bindingError != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_BINDING_UNKNOWN")
	}
	if err != nil {
		_ = repository.RetryMergeTask(ctx, s.db, task, "DISCARD_BINDING_UNKNOWN")
		return
	}
	if task.Kind == "DISCARD_DRAFT_RECONCILE" {
		err = s.reconcileDiscard(ctx, task)
	} else {
		proofs, original, resumeError := s.resumeDiscard(ctx, fixed)
		err = resumeError
		if err == nil {
			check, e := s.db.BeginTx(ctx, nil)
			err = e
			if err == nil {
				err = s.validateDiscard(ctx, check, fixed, proofs, original)
				_ = check.Rollback()
			}
		}
		if err == nil {
			prepared, e := s.prepareDiscard(ctx, fixed)
			err = e
			if err == nil {
				err = s.commitDiscard(ctx, task, fixed, prepared, proofs, original)
			}
		}
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
			err = s.rejectDiscard(ctx, task, fixed, violation.Code)
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

func (s *DraftControls) rejectDiscard(ctx context.Context, task domain.SubmissionTask, fixed discardDraftContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, fixed.Scope.Document, true); err != nil {
		return err
	}
	owner, err := repository.LockedBranchControl(ctx, tx, fixed.Scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	if owner != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	// 未完成prepare也登记原号系统收尾；不能因网络超时猜测Node从未BLOCKED。
	install, _ := json.Marshal(discardDraftInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: fixed.Intent.Preview}})
	if err = repository.CompleteDiscardDraft(ctx, tx, task, "REJECTED", result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_draft_register.go：保留原实现，按业务合并文件。

func (s *DraftControls) RegisterDiscard(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardDraftIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_DRAFT")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Manage {
		return actor, domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	fingerprint, err := domain.OperationFingerprint(path, intent.Input)
	if err != nil {
		return actor, err
	}
	if fact, e := repository.DiscardDraftResult(ctx, s.db, intent.Operation); e == nil {
		if fact.Actor != actor.ID || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	preview, err := s.LookupPreview(ctx, intent.Preview)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_REQUIRED")
	}
	if preview.Actor != actor.ID || preview.Scope.Document != document || preview.Scope.Unit != unit || preview.Scope.Workspace.Workspace != intent.Branch || preview.Scope.Structure != intent.Structure || len(access.Proofs) == 0 || len(preview.Access.Proofs) == 0 || !bytes.Equal(access.Proofs[0].SessionRef, preview.Access.Proofs[0].SessionRef) {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_BINDING_CHANGED")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: preview.Scope.Workspace.Epoch, Purpose: "DISCARD_DRAFT"}
	var bound []*pb.IdentityAdmission
	bound, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = bound
	actor.Proof = bound[0]
	actor.Sources = access.Sources
	originalProofs, err := s.resumePreview(ctx, preview)
	if err != nil {
		return actor, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, intent.Operation); err != nil {
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if prior, e := repository.LockedOperation(ctx, tx, intent.Operation); e == nil {
		if prior.ActorID != actor.ID || prior.Kind != "DISCARD_DRAFT" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	if err = s.validatePreview(ctx, tx, preview, originalProofs); err != nil {
		return actor, err
	}
	fact, err := repository.DiscardPreviewResult(ctx, tx, intent.Preview)
	if err != nil {
		return actor, err
	}
	var public dto.DiscardPreview
	if fact.State != "SUCCEEDED" || json.Unmarshal(fact.Result, &public) != nil || public.PreviewCutRef != intent.Preview {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_REQUIRED")
	}
	expires, err := time.Parse(time.RFC3339Nano, public.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return actor, domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	raw, err := repository.MergeArtifact(ctx, tx, "DISCARD_PREVIEW_FIXED", intent.Preview)
	if err != nil {
		return actor, err
	}
	var snapshot struct {
		Digest []byte `json:"input_digest"`
	}
	if json.Unmarshal(raw, &snapshot) != nil || len(snapshot.Digest) != 32 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	}
	access.Document = current
	fixed := discardDraftContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Scope: preview.Scope, Preview: preview, PreviewResult: public, PreviewDigest: snapshot.Digest}
	fixed.Review, err = s.readDiscardReview(ctx, tx, unit, intent, fixed)
	if err != nil {
		return actor, err
	}
	command := OperationCommand{ID: intent.Operation, Kind: "DISCARD_DRAFT", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "DISCARD_DRAFT", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	encoded, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterDiscardDraft(ctx, tx, intent.Branch, intent.Operation, encoded); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// discard_draft_review.go：保留原实现，按业务合并文件。

func (s *DraftControls) readDiscardReview(ctx context.Context, tx *sql.Tx, unit uint64, intent domain.DiscardDraftIntent, preview discardDraftContext) (discardPreviewReview, error) {
	var result discardPreviewReview
	raw, err := repository.MergeArtifact(ctx, tx, "DISCARD_PREVIEW_REVIEW", intent.Preview)
	if err != nil {
		return result, err
	}
	var original discardPreviewReview
	if json.Unmarshal(raw, &original) != nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_REVIEW_UNKNOWN")
	}
	result.Guard, err = repository.LockedReviewGuard(ctx, tx, unit)
	if err != nil {
		return result, err
	}
	if result.Guard.Submission != "" {
		return result, domain.Violation(domain.StateConflict, "SUBMISSION_PENDING")
	}
	if result.Guard.Ongoing != 0 {
		return result, domain.Violation(domain.StateConflict, "REVIEW_WITHDRAW_REQUIRED")
	}
	if result.Guard.Resumable != original.Pointer.ID {
		return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	if result.Guard.Resumable == 0 {
		if intent.Round != 0 || len(intent.Changes) != 0 || len(preview.PreviewResult.BlockingChanges) != 0 {
			return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
		}
		return result, nil
	}
	result.Pointer, err = repository.LockedReviewPointer(ctx, tx, result.Guard.Resumable)
	if err != nil {
		return result, err
	}
	p := result.Pointer
	if p.Archived || p.Status != "WITHDRAWN" || p.Round != intent.Round || p.Round != original.Pointer.Round || p.Branch != intent.Branch || p.Unit != unit || p.Document != preview.Scope.Document {
		return result, domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	active, err := repository.LockedActiveMerge(ctx, tx, p.ID)
	if err != nil {
		return result, err
	}
	if active != "" {
		return result, domain.Violation(domain.StateConflict, "MERGE_PENDING")
	}
	blocks, err := repository.DiscardPreviewBlockingChanges(ctx, tx, unit)
	if err != nil {
		return result, err
	}
	expected := map[string]string{}
	for _, item := range preview.PreviewResult.BlockingChanges {
		expected[item.Id] = item.Revision
	}
	changes := map[string]string{}
	for _, item := range intent.Changes {
		changes[domain.TextID(item.ID)] = domain.TextID(item.Revision)
	}
	if len(blocks) != len(expected) || len(changes) != len(expected) {
		return result, domain.Violation(domain.StateConflict, "DISCARD_CHANGES_CHANGED")
	}
	for _, item := range blocks {
		if expected[item["id"]] != item["revision"] || changes[item["id"]] != item["revision"] {
			return result, domain.Violation(domain.StateConflict, "DISCARD_CHANGES_CHANGED")
		}
	}
	return result, nil
}

func (s *DraftControls) validateDiscardReview(ctx context.Context, tx *sql.Tx, fixed discardDraftContext) error {
	current, err := s.readDiscardReview(ctx, tx, fixed.Scope.Unit, fixed.Intent, fixed)
	if err != nil {
		return err
	}
	if current != fixed.Review {
		return domain.Violation(domain.StateConflict, "DISCARD_REVIEW_CHANGED")
	}
	expires, err := time.Parse(time.RFC3339Nano, fixed.PreviewResult.ExpiresAt)
	if err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_EXPIRY_UNKNOWN")
	}
	if !expires.After(time.Now()) {
		return domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	return nil
}

// discard_draft_source.go：保留原实现，按业务合并文件。

func (s *DraftControls) resumeDiscard(ctx context.Context, fixed discardDraftContext) ([]*pb.IdentityAdmission, []*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, nil, err
		}
		proofs = append(proofs, proof)
	}
	original, err := s.resumePreview(ctx, fixed.Preview)
	return proofs, original, err
}

func (s *DraftControls) validateDiscard(ctx context.Context, tx *sql.Tx, fixed discardDraftContext, proofs, original []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	if _, err := ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	if err := s.validatePreviewForControl(ctx, tx, fixed.Preview, original, fixed.Intent.Operation); err != nil {
		return err
	}
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	if len(fixed.Access.Proofs) == 0 || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "DISCARD_DRAFT" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "DISCARD_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ADMISSION_UNKNOWN")
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs); err != nil {
		return err
	}
	return s.validateDiscardReview(ctx, tx, fixed)
}

// Node仅得到已登记确认的原预览引用/摘要；正文仍从Node自己的不可变原预览取，Go不让调用方传新候选。
func (s *DraftControls) DiscardSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_SOURCE_INVALID")
	}
	fixed, err := s.LookupDiscard(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	expected := &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
	if !proto.Equal(input, expected) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, original, err := s.resumeDiscard(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.validateDiscard(ctx, tx, fixed, proofs, original); err != nil {
		return nil, err
	}
	previewBinding, err := contextWire(fixed.Preview.Context)
	if err != nil {
		return nil, err
	}
	return map[string]any{"purpose": "DISCARD_DRAFT", "preview_cut_ref": fixed.Intent.Preview, "preview_input_digest": fixed.PreviewDigest, "preview_context": previewBinding, "expected_round_id": domain.TextID(fixed.Intent.Round)}, nil
}
