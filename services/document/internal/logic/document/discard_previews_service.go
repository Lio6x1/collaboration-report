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
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// discard_preview_commit.go：保留原实现，按业务合并文件。

func (s *DraftControls) commitPreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext, snapshot discardPreviewSnapshot, reply *pb.DraftPreviewReply, raw []byte, proofs []*pb.IdentityAdmission) error {
	// 网络与正文校验在SQL锁外；最终落库仍在来源锁下复核原权限、实际谱系和领取代次。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validatePreview(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	version, err := repository.MainUnitVersion(ctx, tx, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Manifest)
	if err != nil || version != snapshot.MainVersion {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_MAIN_VERSION_UNKNOWN")
	}
	_, _, prior, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	lineage, err := codec.Decode(prior)
	if err != nil {
		return err
	}
	canonical, err := codec.Canonical(lineage)
	if err != nil || !bytes.Equal(canonical, snapshot.LineageProvenance) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_LINEAGE_UNKNOWN")
	}
	for _, content := range []*pb.FixedContent{snapshot.Draft, snapshot.Candidate} {
		value, _ := codec.Decode(content.ProvenanceJsonUtf8)
		accepted, _ := codec.Canonical(value.(map[string]any)["accepted_captures"])
		expected, _ := codec.Canonical(lineage.(map[string]any)["accepted_captures"])
		if !bytes.Equal(accepted, expected) {
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_ACCEPTED_CAPTURES_CHANGED")
		}
	}
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resources := map[uint64]bool{}
	for _, content := range []*pb.FixedContent{snapshot.Draft, snapshot.Main} {
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
		state, err := repository.LockedDocumentResource(ctx, tx, id, fixed.Scope.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "DISCARD_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, fixed.Scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	blocks, err := repository.DiscardPreviewBlockingChanges(ctx, tx, fixed.Scope.Unit)
	if err != nil {
		return err
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, fixed.Scope.Unit)
	if err != nil {
		return err
	}
	review := discardPreviewReview{Guard: guard}
	request := guard.Ongoing
	if request == 0 {
		request = guard.Resumable
	}
	if request != 0 {
		review.Pointer, err = repository.LockedReviewPointer(ctx, tx, request)
		if err != nil {
			return err
		}
	}
	reviewRaw, err := json.Marshal(review)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_REVIEW", task.Operation, task.Operation, reviewRaw, "SUCCEEDED"); err != nil {
		return err
	}
	oldDraft, err := repository.InsertMergeVersion(ctx, tx, snapshot.Draft, "RESTORE", task.Operation)
	if err != nil {
		return err
	}
	scope, workspace := fixed.Scope, fixed.Scope.Workspace
	public := map[string]any{"preview_cut_ref": task.Operation, "document_id": domain.TextID(scope.Document), "unit_id": domain.TextID(scope.Unit), "branch_id": domain.TextID(workspace.Workspace), "workspace_epoch": domain.TextID(workspace.Epoch), "lineage_id": domain.TextID(workspace.Lineage), "base_manifest_id": domain.TextID(workspace.BaseManifest), "draft_content_version_id": domain.TextID(oldDraft), "main_manifest_id": domain.TextID(scope.Manifest), "structure_id": domain.TextID(scope.Structure), "cut_digest": hex.EncodeToString(snapshot.Cut.DependencyDigest), "expires_at": time.UnixMilli(reply.ExpiresUnixMs).UTC().Format(time.RFC3339Nano), "blocking_changes": blocks}
	result, err := json.Marshal(public)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "DISCARD_PREVIEW_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_preview_readiness.go：保留原实现，按业务合并文件。

func validatePendingDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error) {
	var original, progress discardPreviewContext
	invalid := errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
	if fact.State != "PREPARING" || json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &progress) != nil {
		return original, invalid
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(progress)
	if !bytes.Equal(a, b) {
		return original, invalid
	}
	return validateOriginalDiscardPreviewBinding(fact)
}

// 启动检查只检查可恢复的原绑定；证明自然过期不等于原会话过期，也不能用重启生成新目标。
func validateOriginalDiscardPreviewBinding(fact repository.PendingDiscardPreviewBinding) (discardPreviewContext, error) {
	var original discardPreviewContext
	invalid := errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &original) != nil {
		return original, invalid
	}
	ctx, scope := original.Context, original.Scope
	w := scope.Workspace
	if ctx == nil || ctx.Target == nil || original.Actor == 0 || scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Structure == 0 || scope.Manifest == 0 || scope.Style == 0 || scope.Control == 0 || original.SessionExpires <= 0 ||
		w.Workspace == 0 || w.Epoch == 0 || w.Lineage == 0 || w.BaseManifest == 0 || w.BaseVersion == 0 || original.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation ||
		original.Intent.Operation != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || !original.Access.Permissions.Manage ||
		original.Intent.Branch != w.Workspace || original.Intent.Structure != scope.Structure || original.Access.Document.Id != domain.TextID(scope.Document) || original.Access.Document.CurrentManifestId != domain.TextID(scope.Manifest) ||
		original.Access.Document.CurrentStructureId != domain.TextID(scope.Structure) || original.Access.Document.CurrentStyleId != domain.TextID(scope.Style) || original.Access.Document.ControlRevision != domain.TextID(scope.Control) ||
		len(ctx.Sources) == 0 || !proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: original.Access.Sources}) {
		return original, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: w.Workspace, StructureId: scope.Structure, WorkspaceEpoch: w.Epoch, Purpose: "DISCARD_PREVIEW"}
	if !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || len(original.Access.Proofs) == 0 || original.Access.Proofs[0] == nil {
		return original, invalid
	}
	parsed, err := domain.ParseDiscardPreview(original.Intent.Input)
	if err != nil || parsed.Operation != fact.Operation || parsed.Branch != w.Workspace || parsed.Structure != scope.Structure {
		return original, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/units/" + domain.TextID(scope.Unit) + "/discard-previews"
	var httpTarget struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Target, &httpTarget) != nil || httpTarget.Path != path {
		return original, invalid
	}
	fingerprint, err := domain.OperationFingerprint(path, original.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return original, invalid
	}
	available := map[string]uint64{}
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return original, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range original.Access.Proofs {
		if proof == nil || proof.ActorId != original.Actor || proof.OperationId != fact.Operation || proof.Purpose != "DISCARD_PREVIEW" || !proto.Equal(proof.Target, expected) ||
			len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, original.Access.Proofs[0].SessionRef) {
			return original, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return original, invalid
			}
		}
	}
	if original.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef {
		return original, invalid
	}
	return original, nil
}

func (s *DraftControls) Ready(ctx context.Context) error {
	facts, err := repository.PendingDiscardPreviewBindings(ctx, s.db)
	if err != nil {
		return errors.New("DISCARD_PREVIEW_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingDiscardPreviewBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateDiscardPreviewAdmission(fixed, admission) != nil {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("DISCARD_PREVIEW_BINDING_UNKNOWN")
		}
	}
	return s.readyDiscards(ctx)
}

// discard_preview_readings.go：保留原实现，按业务合并文件。

func (s *DraftControls) WaitPreview(ctx context.Context, operation string) error {
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		fact, err := repository.DiscardPreviewResult(ctx, s.db, operation)
		if err != nil {
			return err
		}
		if fact.State != "PREPARING" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_PREPARING")
		case <-ticker.C:
		}
	}
}

func (s *DraftControls) previewResultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_PREVIEW_RESULT")
	if err == nil && !access.Permissions.Manage {
		err = domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	return access, err
}

func (s *DraftControls) visiblePreview(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return repository.MainControlReceipt{}, err
	}
	fact, err := repository.DiscardPreviewResult(ctx, tx, operation)
	if err != nil {
		return fact, err
	}
	if fact.Actor != actor.ID {
		return fact, domain.Violation(domain.EntityNotFound, "DISCARD_PREVIEW_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return fact, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	return fact, nil
}

func discardPreviewRejection(fact repository.MainControlReceipt) error {
	var rejected struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
	}
	return domain.Violation(domain.StateConflict, rejected.Error.Code)
}

func (s *DraftControls) ReadPreview(ctx context.Context, actor Actor, document, unit uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupPreview(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Scope.Document != document || fixed.Scope.Unit != unit || fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "DISCARD_PREVIEW_NOT_FOUND")
	}
	access, err := s.previewResultAccess(ctx, actor, document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visiblePreview(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		if fact.State == "PREPARING" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_PREPARING")
		}
		if fact.State != "SUCCEEDED" {
			return nil, discardPreviewRejection(fact)
		}
		var value dto.DiscardPreview
		if json.Unmarshal(fact.Result, &value) != nil || value.PreviewCutRef != operation || value.DocumentId != domain.TextID(document) || value.UnitId != domain.TextID(unit) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		return value, nil
	}, nil
}

// 公共Operation严格投影冻结字段。完整预览仅从原POST回放取得，私有RPC另核验完整原目标与指纹。
func (s *DraftControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.LookupPreview(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.previewResultAccess(ctx, actor, fixed.Scope.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visiblePreview(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"operation_id": operation, "state": fact.State}
		if fact.State == "PREPARING" {
			result["poll_url"] = "/v1/operations/" + operation
			return result, nil
		}
		if fact.State == "SUCCEEDED" {
			var preview dto.DiscardPreview
			if json.Unmarshal(fact.Result, &preview) != nil || preview.DraftContentVersionId == "" {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
			}
			result["result"] = map[string]any{"entity_id": preview.DraftContentVersionId}
			return result, nil
		}
		if err = discardPreviewRejection(fact); err != nil {
			var rejected map[string]any
			if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected["error"] == nil {
				return nil, err
			}
			result["error"] = rejected["error"]
		}
		return result, nil
	}, nil
}

func (s *DraftControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "DISCARD_PREVIEW_QUERY_INVALID")
	}
	fact, err := repository.DiscardPreviewResult(ctx, s.db, query.OperationId)
	if err == sql.ErrNoRows {
		return s.DiscardOperationResult(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	fixed, err := s.LookupPreview(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if fact.Actor != fixed.Actor {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	if fact.State == "SUCCEEDED" {
		var preview dto.DiscardPreview
		if json.Unmarshal(fact.Result, &preview) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		version, err := domain.CanonicalID(preview.DraftContentVersionId)
		if err != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_RESULT_UNKNOWN")
		}
		result.State, result.ResultIds = pb.OperationResult_SUCCEEDED, []uint64{version}
		return result, nil
	}
	if err = discardPreviewRejection(fact); err != nil {
		violation, ok := err.(domain.RuleViolation)
		if !ok || violation.Kind != domain.StateConflict {
			return nil, err
		}
		result.State, result.BusinessCode = pb.OperationResult_REJECTED, violation.Code
	}
	return result, nil
}

// discard_preview_recovery.go：保留原实现，按业务合并文件。

func (s *DraftControls) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimDiscardPreviewTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed discardPreviewContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	} else {
		original, readErr := s.LookupPreview(ctx, task.Operation)
		err = readErr
		if err == nil {
			a, _ := json.Marshal(original)
			b, _ := json.Marshal(fixed)
			if !bytes.Equal(a, b) {
				err = domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
			}
		}
		if err == nil {
			err = s.capturePreview(ctx, task, fixed)
		}
	}
	var violation domain.RuleViolation
	if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
		err = rejectPreviewTask(ctx, s.db, task, violation.Code)
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

func (s *DraftControls) capturePreview(ctx context.Context, task domain.SubmissionTask, fixed discardPreviewContext) error {
	proofs, err := s.resumePreview(ctx, fixed)
	if err != nil {
		return err
	}
	if err = s.validateBeforePreviewCapture(ctx, fixed, proofs); err != nil {
		return err
	}
	input := discardPreviewRequest(fixed)
	reply, err := s.client.Preview(ctx, input)
	if err != nil {
		return discardPreviewCaptureError(err)
	}
	raw, err := s.client.FixedDraft(ctx, input)
	if err != nil {
		return err
	}
	snapshot, err := validateDiscardPreviewSnapshot(s.validator, raw, fixed, reply)
	if err != nil {
		return err
	}
	if reply.ExpiresUnixMs <= time.Now().UnixMilli() {
		return domain.Violation(domain.StateConflict, "DISCARD_PREVIEW_EXPIRED")
	}
	return s.commitPreview(ctx, task, fixed, snapshot, reply, raw, proofs)
}

func discardPreviewCaptureError(err error) error {
	rpc, ok := status.FromError(err)
	if !ok {
		return err
	}
	if rpc.Code() == codes.FailedPrecondition && (rpc.Message() == "WORKSPACE_CHANGED" || rpc.Message() == "LINEAGE_CHANGED") {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	if rpc.Code() == codes.ResourceExhausted && rpc.Message() == "DISCARD_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	return err
}

// 拒绝结果和任务终态在同一事务落定；先复核原任务代次，避免旧 worker 拒绝新的预约。
func rejectPreviewTask(ctx context.Context, db *sql.DB, task domain.SubmissionTask, code string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// discard_preview_register.go：保留原实现，按业务合并文件。

func (s *DraftControls) RegisterPreview(ctx context.Context, actor Actor, document, unit uint64, intent domain.DiscardPreviewIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "DISCARD_PREVIEW")
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
	actor.Sources = access.Sources
	if fact, err := repository.DiscardPreviewResult(ctx, s.db, intent.Operation); err == nil {
		if fact.Actor != actor.ID || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if err != sql.ErrNoRows {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	if access.Document.CurrentStructureId != domain.TextID(intent.Structure) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: workspace.Epoch, Purpose: "DISCARD_PREVIEW"}
	if len(access.Proofs) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_IDENTITY_UNKNOWN")
	}
	var boundProofs []*pb.IdentityAdmission
	boundProofs, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = boundProofs
	actor.Proof = access.Proofs[0]
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
	if fact, err := repository.LockedOperation(ctx, tx, intent.Operation); err == nil {
		if fact.ActorID != actor.ID || fact.Kind != "DISCARD_PREVIEW" || !bytes.Equal(fact.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if err != sql.ErrNoRows {
		return actor, err
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, unit, document)
	if err != nil || active != intent.Branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	actual, err := repository.LockedEditWorkspace(ctx, tx, intent.Branch, document, unit)
	if err != nil {
		return actor, err
	}
	if actual != workspace || current.CurrentStructureId != domain.TextID(intent.Structure) {
		return actor, domain.Violation(domain.StateConflict, "DRAFT_CONTEXT_CHANGED")
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", workspace.Workspace, unit, workspace.Epoch)
	if err != nil {
		return actor, err
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalID(current.ControlRevision)
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	// 授权到锁定文档之间MAIN可合法推进。权限已按ACL/结构复核，原上下文须保存同一最终文档快照。
	access.Document = current
	fixed := discardPreviewContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Scope: draftSourceScope{Document: document, Unit: unit, Room: room, Manifest: manifest, Structure: intent.Structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: workspace}}
	command := OperationCommand{ID: intent.Operation, Kind: "DISCARD_PREVIEW", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, map[string]any{"operation_id": intent.Operation, "state": "PREPARING"}); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "DISCARD_PREVIEW", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterDiscardPreview(ctx, tx, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// discard_preview_snapshot.go：保留原实现，按业务合并文件。

type discardPreviewSnapshot struct {
	Draft, Main, Candidate *pb.FixedContent
	Cut                    *pb.CutVector
	MainVersion            uint64
	LineageProvenance      []byte
}

func validateDiscardPreviewSnapshot(validator *codec.Validator, raw []byte, fixed discardPreviewContext, reply *pb.DraftPreviewReply) (discardPreviewSnapshot, error) {
	var snapshot discardPreviewSnapshot
	invalid := domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_FIXED_UNKNOWN")
	scope, operation := fixed.Scope, fixed.Intent.Operation
	if epoch := draftMainEpoch(scope); epoch == 0 || epoch > scope.Control {
		return snapshot, invalid
	}
	if validator == nil || reply == nil || reply.State != pb.CaptureReply_FIXED || reply.PreviewCutRef != operation || reply.MainManifestId != scope.Manifest || reply.ObservedCut == nil || reply.ExpiresUnixMs <= 0 || len(reply.InputDigest) != 32 || len(raw) > 24<<20 {
		return snapshot, invalid
	}
	var envelope struct {
		Binding                json.RawMessage   `json:"binding"`
		Room                   string            `json:"room_id"`
		DraftCut               json.RawMessage   `json:"draft_cut"`
		MainCut                []json.RawMessage `json:"main_cut"`
		Draft, Main, Candidate json.RawMessage
		Conflicts              []json.RawMessage `json:"conflicts"`
		Digest                 []byte            `json:"input_digest"`
		Preview                string            `json:"preview_cut_ref"`
		Expires                string            `json:"expires_unix_ms"`
		Source                 struct {
			Purpose   string `json:"purpose"`
			Room      string `json:"room_id"`
			Manifest  string `json:"main_manifest_id"`
			Version   string `json:"main_version_id"`
			Structure string `json:"structure_id"`
			Style     string `json:"style_id"`
			Control   string `json:"control_revision"`
			MainEpoch string `json:"workspace_epoch"`
			Lineage   struct {
				ID         string          `json:"lineage_id"`
				Manifest   string          `json:"base_manifest_id"`
				Version    string          `json:"base_version_id"`
				Provenance json.RawMessage `json:"provenance"`
			} `json:"lineage"`
		} `json:"source"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Room != domain.TextID(scope.Room) || envelope.Preview != operation || envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs)) || !bytes.Equal(envelope.Digest, reply.InputDigest) || len(envelope.Conflicts) != 0 || len(envelope.MainCut) > 1 {
		return snapshot, invalid
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(envelope.Binding, binding) != nil || !proto.Equal(binding, discardPreviewRequest(fixed)) {
		return snapshot, invalid
	}
	value, err := codec.Decode(raw)
	if err != nil {
		return snapshot, invalid
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return snapshot, invalid
	}
	delete(fields, "input_digest")
	canonical, err := codec.Canonical(fields)
	if err != nil {
		return snapshot, invalid
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(digest[:], reply.InputDigest) {
		return snapshot, invalid
	}
	source := envelope.Source
	// 旧固定包缺省workspace_epoch仍按其原control解释；新增原包必须保存实际物理来源。
	if source.MainEpoch != domain.TextID(draftMainEpoch(scope)) && !(scope.MainEpoch == 0 && source.MainEpoch == "") {
		return snapshot, invalid
	}
	if source.Purpose != "DISCARD_PREVIEW" || source.Room != domain.TextID(scope.Room) || source.Manifest != domain.TextID(scope.Manifest) || source.Structure != domain.TextID(scope.Structure) || source.Style != domain.TextID(scope.Style) || source.Control != domain.TextID(scope.Control) ||
		source.Lineage.ID != domain.TextID(scope.Workspace.Lineage) || source.Lineage.Manifest != domain.TextID(scope.Workspace.BaseManifest) || source.Lineage.Version != domain.TextID(scope.Workspace.BaseVersion) {
		return snapshot, invalid
	}
	if snapshot.MainVersion, err = domain.CanonicalID(source.Version); err != nil {
		return snapshot, invalid
	}
	lineage, err := codec.Decode(source.Lineage.Provenance)
	if err != nil || validator.Provenance(lineage) != nil {
		return snapshot, invalid
	}
	snapshot.LineageProvenance, err = codec.Canonical(lineage)
	if err != nil {
		return snapshot, invalid
	}
	draftCut := &pb.RoomCut{}
	if protojson.Unmarshal(envelope.DraftCut, draftCut) != nil || draftCut.RoomId != scope.Room || draftCut.WorkspaceEpoch != scope.Workspace.Epoch {
		return snapshot, invalid
	}
	snapshot.Cut = &pb.CutVector{DocumentId: scope.Document, StructureId: scope.Structure, StyleId: scope.Style, ControlRevision: scope.Control, Rooms: []*pb.RoomCut{draftCut}}
	mainCut := &pb.RoomCut{WorkspaceEpoch: draftMainEpoch(scope)}
	if len(envelope.MainCut) == 1 {
		if protojson.Unmarshal(envelope.MainCut[0], mainCut) != nil || mainCut.RoomId == 0 || mainCut.RoomId == scope.Room || mainCut.WorkspaceEpoch != draftMainEpoch(scope) {
			return snapshot, invalid
		}
		snapshot.Cut.Rooms = append(snapshot.Cut.Rooms, mainCut)
	}
	rooms, err := codec.Canonical(domain.CutBinding(snapshot.Cut)["rooms"])
	if err != nil {
		return snapshot, invalid
	}
	roomDigest := sha256.Sum256(rooms)
	snapshot.Cut.DependencyDigest = roomDigest[:]
	if !proto.Equal(snapshot.Cut, reply.ObservedCut) {
		return snapshot, invalid
	}
	decode := func(raw json.RawMessage, ref string, branch, epoch uint64, cut *pb.RoomCut, main bool) (*pb.FixedContent, error) {
		content := &pb.FixedContent{}
		if protojson.Unmarshal(raw, content) != nil || content.OperationId != operation || content.CaptureId != ref || content.BranchId != branch || content.WorkspaceEpoch != epoch {
			return nil, invalid
		}
		if err := validateFixedMergeContent(validator, content, scope.Document, scope.Unit, scope.Structure); err != nil {
			return nil, err
		}
		projected, err := domain.BranchBaselineBody(content.BodyJsonUtf8)
		if err != nil || !bytes.Equal(projected, content.BodyJsonUtf8) {
			return nil, invalid
		}
		cutBytes, err := codec.Canonical(map[string]any{"room_id": domain.TextID(cut.RoomId), "workspace_epoch": domain.TextID(cut.WorkspaceEpoch), "durable_seq": domain.TextID(cut.DurableSeq)})
		if err != nil || !bytes.Equal(cutBytes, content.CutJsonUtf8) {
			return nil, invalid
		}
		provenanceValue, _ := codec.Decode(content.ProvenanceJsonUtf8)
		provenance, ok := provenanceValue.(map[string]any)
		if !ok || provenance["source_operation_id"] != operation || provenance["lineage_id"] != content.LineageRef {
			return nil, invalid
		}
		baseManifest, baseVersion := scope.Workspace.BaseManifest, scope.Workspace.BaseVersion
		if main {
			baseManifest, baseVersion = scope.Manifest, snapshot.MainVersion
		} else if content.LineageRef != domain.TextID(scope.Workspace.Lineage) {
			return nil, invalid
		}
		if provenance["base_manifest_id"] != domain.TextID(baseManifest) || provenance["base_content_version_id"] != domain.TextID(baseVersion) {
			return nil, invalid
		}
		return content, nil
	}
	if snapshot.Draft, err = decode(envelope.Draft, operation, scope.Workspace.Workspace, scope.Workspace.Epoch, draftCut, false); err != nil {
		return snapshot, err
	}
	if !proto.Equal(snapshot.Draft, reply.Draft) {
		return snapshot, invalid
	}
	if snapshot.Main, err = decode(envelope.Main, operation+":main", 0, scope.Control, mainCut, true); err != nil {
		return snapshot, err
	}
	if snapshot.Candidate, err = decode(envelope.Candidate, operation+":candidate", scope.Workspace.Workspace, scope.Workspace.Epoch, draftCut, false); err != nil {
		return snapshot, err
	}
	// 丢弃候选只能来自这一份可靠MAIN；即使整个包重新计算了摘要，也不能夹带另一份有效正文。
	if !bytes.Equal(snapshot.Candidate.BodyJsonUtf8, snapshot.Main.BodyJsonUtf8) || !bytes.Equal(snapshot.Candidate.BodyHash, snapshot.Main.BodyHash) {
		return snapshot, invalid
	}
	return snapshot, nil
}

// discard_preview_source.go：保留原实现，按业务合并文件。

func (s *DraftControls) LookupPreview(ctx context.Context, operation string) (discardPreviewContext, error) {
	var fixed discardPreviewContext
	raw, err := repository.MergeArtifact(ctx, s.db, "DISCARD_PREVIEW_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return fixed, nil
}

func (s *DraftControls) resumePreview(ctx context.Context, fixed discardPreviewContext) ([]*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

func validateDiscardPreviewAdmission(fixed discardPreviewContext, actual repository.OriginalAdmissionBinding) error {
	ctx := fixed.Context
	if ctx == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit ||
		actual.Identity != ctx.IdentityAdmissionRef || actual.Operation != ctx.OperationId || actual.Purpose != "DISCARD_PREVIEW" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, ctx.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return nil
}

func (s *DraftControls) validatePreview(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error {
	return s.validatePreviewForControl(ctx, tx, fixed, proofs, "")
}

// 确认只可在自己的业务占位下复核原预览；普通预览必须看到OPEN，不能借用其他原号。
func (s *DraftControls) validatePreviewForControl(ctx context.Context, tx *sql.Tx, fixed discardPreviewContext, proofs []*pb.IdentityAdmission, owner string) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	scope := fixed.Scope
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	// 丢弃预览只读取可靠主线；主线锁定是允许状态，但原结构/清单/control不可被换掉。
	if current.CurrentManifestId != domain.TextID(scope.Manifest) || current.CurrentStructureId != domain.TextID(scope.Structure) || current.CurrentStyleId != domain.TextID(scope.Style) || current.ControlRevision != domain.TextID(scope.Control) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, scope.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)
	if err != nil || active != scope.Workspace.Workspace {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, active, scope.Document, scope.Unit)
	if err != nil {
		return err
	}
	if workspace != scope.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	control, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return err
	}
	if control != owner {
		return domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	if err = validateDiscardPreviewAdmission(fixed, admission); err != nil {
		return err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "DISCARD_PREVIEW_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "DISCARD_PREVIEW_BINDING_UNKNOWN")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

func (s *DraftControls) PreviewSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.LookupPreview(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, discardPreviewRequest(fixed)) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resumePreview(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.validatePreview(ctx, tx, fixed, proofs); err != nil {
		return nil, err
	}
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err == nil {
		value["purpose"] = "DISCARD_PREVIEW"
	}
	return value, err
}

func (s *DraftControls) validateBeforePreviewCapture(ctx context.Context, fixed discardPreviewContext, proofs []*pb.IdentityAdmission) error {
	// Go已能确定的失效不依赖Node在线。这里只短期核验后回滚释放锁，RPC和最终提交仍各自复核。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.validatePreview(ctx, tx, fixed, proofs)
}
