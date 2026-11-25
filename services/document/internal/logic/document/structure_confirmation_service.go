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
	"encoding/base64"
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"strconv"
	"time"
)

// structure_confirmation.go：保留原实现，按业务合并文件。

type structureConfirmationBinding struct {
	Reference string `json:"reference"`
	Digest    []byte `json:"digest"`
	Expires   int64  `json:"expires"`
}

func validateStructureConfirmationActor(actor Actor, access DocumentAccess, input dto.StructureChange, fact repository.PendingStructurePreviewBinding, now time.Time) (structurePreviewContext, error) {
	var preview structurePreviewContext
	if fact.State != "SUCCEEDED" {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_NOT_FIXED")
	}
	preview, err := validateOriginalStructurePreviewBinding(fact)
	if err != nil {
		return preview, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	if err = domain.ValidateStructureConfirmation(input, preview.Intent.Input); err != nil {
		return preview, err
	}
	if actor.ID != preview.Actor || access.Document.Id != domain.TextID(preview.Document) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ACTOR_MISMATCH")
	}
	if !access.Permissions.Manage {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	if actor.SessionExpiresUnixMs <= now.UnixMilli() || preview.SessionExpires <= now.UnixMilli() {
		return preview, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if actor.SessionExpiresUnixMs != preview.SessionExpires || len(access.Proofs) == 0 {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
	}
	expected := &pb.Target{DocumentId: preview.Document, StructureId: preview.Structure, WorkspaceEpoch: preview.Control, Purpose: "MAIN_CONTROL"}
	for _, proof := range access.Proofs {
		// 原短期证明可以已经过期；当前确认必须取得同一会话的新真实证明，不能用另一登录会话接管预览。
		if proof == nil || proof.ActorId != actor.ID || proof.OperationId != input.OperationId || proof.Purpose != "MAIN_CONTROL" || !proto.Equal(proof.Target, expected) || proof.ExpiresUnixMs <= now.UnixMilli() || len(proof.SessionRef) != 32 || !bytes.Equal(proof.SessionRef, preview.Access.Proofs[0].SessionRef) {
			return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SESSION_MISMATCH")
		}
	}
	if !equalPullSources(access.Sources, preview.Access.Sources) {
		return preview, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_SOURCE_CHANGED")
	}
	if access.Document.MainLocked || access.Document.CurrentManifestId != domain.TextID(preview.Manifest) || access.Document.CurrentStructureId != domain.TextID(preview.Structure) || access.Document.CurrentStyleId != domain.TextID(preview.Style) || access.Document.ControlRevision != domain.TextID(preview.Control) {
		return preview, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CONTEXT_CHANGED")
	}
	return preview, nil
}

// structure_confirmation_read.go：保留原实现，按业务合并文件。

type structureConfirmation struct {
	Preview   structurePreviewContext
	Snapshot  structurePreviewSnapshot
	Raw       []byte
	Digest    []byte
	Expires   int64
	Migration domain.StructureMigration
}

func (s *MainControls) readStructureConfirmation(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, input dto.StructureChange) (structureConfirmation, error) {
	var result structureConfirmation
	reference, err := domain.StructureConfirmationReference(input)
	if err != nil {
		return result, err
	}
	fact, err := repository.LockedStructureConfirmation(ctx, tx, reference)
	if err == sql.ErrNoRows {
		return result, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	if err != nil {
		return result, err
	}
	now := time.Now()
	if !fact.ReceiptExpires.After(now) {
		return result, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	result.Preview, err = validateStructureConfirmationActor(actor, access, input, fact.Binding, now)
	if err != nil {
		return result, err
	}
	if err = validateStructurePreviewUnits(ctx, tx, result.Preview); err != nil {
		return result, err
	}
	blocks, err := structurePreviewBlockingUnits(ctx, tx, result.Preview)
	if err != nil {
		return result, err
	}
	if len(blocks) != 0 {
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGE_BLOCKED")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef, true)
	if err != nil {
		return result, err
	}
	if err = validateStructurePreviewAdmission(result.Preview, admission); err != nil {
		return result, err
	}
	state, err := repository.LockedAdmission(ctx, tx, result.Preview.Context.DocumentAdmissionRef)
	if err != nil {
		return result, err
	}
	if state.State == "CLOSED" {
		return result, domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return result, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, result.Preview.Context.DocumentAdmissionRef)
	if err != nil {
		return result, err
	}
	if !equalPullSources(sources, result.Preview.Access.Sources) {
		return result, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	// 同一原快照先校验完整digest/cut/body，再读取候选；引用和到期时间都不能只相信浏览器参数。
	var envelope struct {
		Preview   string          `json:"preview_cut_ref"`
		Expires   string          `json:"expires_unix_ms"`
		Digest    []byte          `json:"input_digest"`
		Cut       json.RawMessage `json:"observed_cut"`
		Migration json.RawMessage `json:"migration"`
		Source    json.RawMessage `json:"source"`
	}
	invalid := domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	if json.Unmarshal(fact.Snapshot, &envelope) != nil {
		return result, invalid
	}
	result.Expires, err = strconv.ParseInt(envelope.Expires, 10, 64)
	if err != nil || result.Expires <= 0 || strconv.FormatInt(result.Expires, 10) != envelope.Expires {
		return result, invalid
	}
	if result.Expires <= now.UnixMilli() {
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_EXPIRED")
	}
	cut := &pb.CutVector{}
	if protojson.Unmarshal(envelope.Cut, cut) != nil {
		return result, invalid
	}
	reply := &pb.DraftPreviewReply{State: pb.CaptureReply_FIXED, PreviewCutRef: envelope.Preview,
		MainManifestId: result.Preview.Manifest, ObservedCut: cut, ExpiresUnixMs: result.Expires, InputDigest: envelope.Digest}
	result.Snapshot, err = validateStructurePreviewSnapshot(s.validator, fact.Snapshot, result.Preview, reply)
	if err != nil {
		return result, err
	}
	if len(envelope.Migration) == 0 {
		// 首轮旧事实只固定正文，没有迁移候选。保留原字节，明确拒绝确认，不能原号内重新算候选。
		return result, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CANDIDATE_MISSING")
	}
	var source struct {
		Manifest    string                `json:"manifest_id"`
		StructureID string                `json:"structure_id"`
		Style       string                `json:"style_id"`
		Control     string                `json:"control_revision"`
		Structure   dto.StructureRevision `json:"structure"`
	}
	if json.Unmarshal(envelope.Source, &source) != nil || source.Manifest != domain.TextID(result.Preview.Manifest) || source.StructureID != domain.TextID(result.Preview.Structure) || source.Style != domain.TextID(result.Preview.Style) || source.Control != domain.TextID(result.Preview.Control) {
		return result, invalid
	}
	actualStructure, err := repository.StructureRow(ctx, tx, source.StructureID, domain.TextID(result.Preview.Document))
	if err != nil {
		return result, err
	}
	actualJSON, _ := json.Marshal(actualStructure)
	sourceJSON, _ := json.Marshal(source.Structure)
	if !bytes.Equal(actualJSON, sourceJSON) {
		return result, invalid
	}
	// 系统source里的changes也必须是实际原HTTP意图，不能仅验证候选自己声称的动作种类。
	var sourceIntent dto.StructurePreviewCreate
	if json.Unmarshal(envelope.Source, &sourceIntent) != nil {
		return result, invalid
	}
	sourceIntent.OperationId = result.Preview.Intent.Operation
	sourceIntent.ExpectedStructureId = source.StructureID
	sourceIntent.ExpectedControlRevision = source.Control
	declared, _ := json.Marshal(sourceIntent)
	original, _ := json.Marshal(result.Preview.Intent.Input)
	if !bytes.Equal(declared, original) {
		return result, invalid
	}
	scopes := make([]domain.StructureMigrationSource, 0, len(result.Preview.Units))
	for _, unit := range result.Preview.Units {
		scopes = append(scopes, domain.StructureMigrationSource{Unit: unit.Unit, Chapter: unit.Chapter})
	}
	result.Migration, err = domain.ParseStructureMigration(envelope.Migration, result.Preview.Intent.Input, scopes, s.validator)
	if err != nil {
		return result, err
	}
	result.Raw, result.Digest = fact.Snapshot, envelope.Digest
	return result, nil
}

// structure_confirmation_source.go：保留原实现，按业务合并文件。

func (s *MainControls) loadStructureConfirmation(ctx context.Context, tx *sql.Tx, fixed mainControlContext, proofs []*pb.IdentityAdmission) (structureConfirmation, error) {
	var empty structureConfirmation
	if fixed.StructurePreview == nil || fixed.Intent.Structure == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_CONFIRMATION_BINDING_UNKNOWN")
	}
	access := fixed.Access
	access.Proofs = proofs
	result, err := s.readStructureConfirmation(ctx, tx, Actor{ID: fixed.Actor, SessionExpiresUnixMs: fixed.SessionExpires}, access, *fixed.Intent.Structure)
	if err != nil {
		return empty, err
	}
	if fixed.StructurePreview.Reference != result.Preview.Intent.Operation || fixed.StructurePreview.Expires != result.Expires || !bytes.Equal(fixed.StructurePreview.Digest, result.Digest) {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_CONFIRMATION_BINDING_UNKNOWN")
	}
	return result, nil
}

func structureConfirmationCut(fixed structureConfirmation) (*pb.CutVector, error) {
	affected := map[string]bool{}
	for _, change := range fixed.Migration.Changes {
		for _, source := range change.Sources {
			affected[source] = true
		}
	}
	all, selected := []uint64{}, []uint64{}
	for _, unit := range fixed.Preview.Units {
		all = append(all, unit.Room)
		if affected[domain.TextID(unit.Unit)] {
			selected = append(selected, unit.Room)
		}
	}
	return domain.StructurePreparationCut(fixed.Snapshot.Cut, all, selected)
}

// 此私有查询只返回原确认中的预览绑定。候选留在固定artifact，调用方不能另提交候选或新cut。
func (s *MainControls) ConfirmationSource(ctx context.Context, input *pb.PrepareMainCommitRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.ExpectedCut == nil {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_CONFIRMATION_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) || fixed.Generation != input.BarrierGeneration {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return nil, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return nil, err
	}
	if err = domain.ValidateMainControl(fixed.Access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, fixed.Intent); err != nil {
		return nil, err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	if barrier.Operation != fixed.Intent.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return nil, err
	}
	value := map[string]any{"structure_preview": nil}
	if fixed.StructurePreview != nil {
		confirmation, err := s.loadStructureConfirmation(ctx, tx, fixed, proofs)
		if err != nil {
			return nil, err
		}
		cut, err := structureConfirmationCut(confirmation)
		if err != nil {
			return nil, err
		}
		if !proto.Equal(cut, input.ExpectedCut) {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CUT_CONFLICT")
		}
		value["structure_preview"] = map[string]string{"reference": fixed.StructurePreview.Reference, "digest": base64.StdEncoding.EncodeToString(fixed.StructurePreview.Digest), "expires": domain.TextID(uint64(fixed.StructurePreview.Expires))}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return value, nil
}
