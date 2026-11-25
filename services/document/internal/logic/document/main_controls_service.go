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
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// main_control_commit.go：保留原实现，按业务合并文件。

func (s *MainControls) commit(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery, main *pb.MainCaptureReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	fixed := recovery.Fixed
	cut, err := codec.Canonical(domain.CutBinding(main.Cut))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(cut)
	if prepared == nil ||
		prepared.State != pb.MainBarrierReply_PREPARED ||
		prepared.OperationId != task.Operation ||
		prepared.BarrierGeneration != fixed.Generation ||
		!proto.Equal(prepared.PreparedCut, main.Cut) ||
		!bytes.Equal(prepared.Digest, digest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
	// RPC已在锁外完成。最终事务按operation→来源→Doc→屏障→worker复核，任何一步失败整体回滚。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if err = domain.ValidateMainControl(fixed.Access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, fixed.Intent); err != nil {
		return err
	}
	if current.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Structure) ||
		current.CurrentStyleId != domain.TextID(fixed.Style) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != fixed.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMainControlProofs(fixed, proofs); err != nil {
		return err
	}
	entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, fixed.Document)
	if err != nil {
		return err
	}
	if len(entries) != len(main.Units) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	sourceStructure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return err
	}
	if !structureManifestUnitSetMatches(sourceStructure, entries) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	captured := map[uint64]*pb.FixedContent{}
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, content := range main.Units {
		captured[content.UnitId] = content
	}
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, main.Units, "MERGE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	style := fixed.Style
	locked := current.MainLocked
	// 样式与清单均追加新修订；历史轮次保留原指针，可靠MAIN尾部与新控制指针在同一事务切换。
	if fixed.Intent.Kind == "STYLE_CHANGE" {
		style, err = repository.InsertControlledStyle(ctx, tx, fixed.Document, fixed.Style, task.Operation, fixed.Intent.StyleJSON)
		if err != nil {
			return err
		}
	} else if fixed.Intent.Kind == "MAIN_LOCK" {
		locked = fixed.Intent.Locked
	}
	layout, err := prepareControlledLayout(ctx, tx, current, fixed)
	if err != nil {
		return err
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Manifest, layout.Structure, style, task.Operation, fixed.Intent.Kind, controlAttemptKey(task.Operation, recovery.Attempt))
	if err != nil {
		return err
	}
	control := fixed.Control + 1
	units, err := materializeControlledUnits(ctx, tx, fixed, layout, entries, captured, manifest, control)
	if err != nil {
		return err
	}
	entity := style
	if layout.Chapter != 0 {
		entity = layout.Chapter
	}
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" {
		entity = layout.Structure
	}
	manifestDigest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{
		Context: fixed.Context, Generation: fixed.Generation,
		Manifest: manifest, Units: units, Digest: manifestDigest[:],
		Structure: layout.Structure, Style: style, Control: control, Entity: entity,
	})
	if err != nil {
		return err
	}
	result := map[string]any{
		"operation_id": task.Operation, "state": "SUCCEEDED",
		"result": map[string]string{
			"manifest_id": domain.TextID(manifest), "entity_id": domain.TextID(entity),
			"main_sync_state": "SYNCING",
		},
	}
	if err = repository.PointToControlledStructure(ctx, tx, fixed.Document, layout.Structure); err != nil {
		return err
	}
	if err = repository.CompleteMainControl(ctx, tx, task, fixed.Document, fixed.Generation, manifest, style, locked, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_compute.go：保留原实现，按业务合并文件。

func controlAttemptKey(operation string, attempt uint64) string {
	return operation + ":" + domain.TextID(attempt)
}

func (s *MainControls) captureMain(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) (*pb.MainCaptureReply, error) {
	fixed := recovery.Fixed
	key := controlAttemptKey(task.Operation, recovery.Attempt)
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_CONTROL_MAIN_FIXED", key)
	if err == nil {
		var main pb.MainCaptureReply
		if json.Unmarshal(raw, &main) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
		}
		return &main, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	input := &pb.CaptureMainRequest{Context: fixed.Context, ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure, ExpectedStyleId: fixed.Style, ExpectedControlRevision: fixed.Control, Usage: "MAIN_CONTROL:" + domain.TextID(recovery.Attempt)}
	main, err := s.client.CaptureMain(ctx, input)
	if err != nil {
		return nil, err
	}
	if main == nil ||
		main.State != pb.CaptureReply_FIXED ||
		main.Cut == nil ||
		main.Cut.DocumentId != fixed.Document ||
		main.Cut.StructureId != fixed.Structure ||
		main.Cut.StyleId != fixed.Style ||
		main.Cut.ControlRevision != fixed.Control {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
	}
	if err = validateMainCaptureDigest(main); err != nil {
		return nil, err
	}
	seen := map[uint64]bool{}
	for _, content := range main.Units {
		if content == nil ||
			content.UnitId == 0 ||
			seen[content.UnitId] ||
			content.BranchId != 0 ||
			content.WorkspaceEpoch != fixed.Control ||
			content.OperationId != task.Operation {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		seen[content.UnitId] = true
		if err = validateFixedMergeContent(s.validator, content, fixed.Document, content.UnitId, fixed.Structure); err != nil {
			return nil, err
		}
	}
	payload, err := json.Marshal(main)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return nil, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return nil, err
	}
	if prior, err := repository.MergeArtifact(ctx, tx, "MAIN_CONTROL_MAIN_FIXED", key); err == nil {
		// 另一领取代次若已固定，则不能以这次RPC结果覆盖；必须取得完全相同的原输入。
		var known pb.MainCaptureReply
		if json.Unmarshal(prior, &known) != nil || !bytes.Equal(known.ResultDigest, main.ResultDigest) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		main = &known
	} else if err != sql.ErrNoRows {
		return nil, err
	} else if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_MAIN_FIXED", key, task.Operation, payload, "SUCCEEDED"); err != nil {
		return nil, err
	}
	return main, tx.Commit()
}

func (s *MainControls) advanceAttempt(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error {
	tx, err := s.db.BeginTx(ctx, nil)
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
	recovery.Attempt++
	raw, err := json.Marshal(recovery)
	if err != nil {
		return err
	}
	if err = repository.AdvanceMainControlAttempt(ctx, tx, task, raw); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CUT_CHANGED_RETRY")
}

// main_control_install.go：保留原实现，按业务合并文件。

func (s *MainControls) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	result, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.State != pb.OperationResult_SUCCEEDED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_NOT_TERMINAL")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_COMMIT_FIXED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed mainInstallContext
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_BINDING_UNKNOWN")
	}
	units := make([]any, 0, len(fixed.Units))
	for _, unit := range fixed.Units {
		content, err := fixedWire(unit.Content)
		if err != nil {
			return nil, err
		}
		units = append(units, map[string]any{"unit_id": domain.TextID(unit.UnitId), "content_version_id": domain.TextID(unit.ContentVersionId), "content": content})
	}
	transition, err := structureInstallationWire(fixed.Transition)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"generation": domain.TextID(fixed.Generation), "manifest_id": domain.TextID(fixed.Manifest),
		"structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style),
		"control_revision": domain.TextID(fixed.Control), "manifest_digest": base64.StdEncoding.EncodeToString(fixed.Digest),
		"units": units, "lineage": nil, "structure_transition": transition,
	}, nil
}

func (s *MainControls) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	// 这是已固定终态的系统收尾，不依赖原用户在线或旧短期准入；Node须再次查询Go权威结果。
	var fixed mainInstallContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RECONCILE_BINDING_UNKNOWN")
	}
	result, err := s.OperationResult(ctx, &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Context.Target, RequestFingerprint: fixed.Context.RequestFingerprint})
	if err != nil {
		return err
	}
	if result.State != pb.OperationResult_SUCCEEDED && result.State != pb.OperationResult_REJECTED && result.State != pb.OperationResult_CANCELLED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RESULT_UNKNOWN")
	}
	if result.State == pb.OperationResult_SUCCEEDED {
		// 在调用Node前先验证实际清单完整性，避免错误的空候选使Node退役仍存在的MAIN房间。
		if err = s.validateControlInstallation(ctx, fixed); err != nil {
			return err
		}
	}
	reply, err := s.client.ReconcileMain(ctx, &pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, ManifestDigest: fixed.Digest, Units: fixed.Units})
	if err != nil {
		return err
	}
	if fixed.Context.OperationId != task.Operation || !validMainInstallReply(fixed, result.State == pb.OperationResult_SUCCEEDED, reply) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_INSTALL_RESULT_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
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
	if fixed.Transition != nil {
		scope := "doc-acl:" + domain.TextID(fixed.Context.Target.DocumentId)
		fence := &pb.FenceRequest{RevocationId: task.Operation, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope, fixed.Transition.ACLEpoch), ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope, Epoch: fixed.Transition.ACLEpoch}}}
		done, err := repository.StructureAuthorizationComplete(ctx, tx, task.Operation, fixed.Context.Target.DocumentId, fixed.Transition.ACLEpoch, platform.BootstrapDigest(fence, "edge"), platform.BootstrapDigest(fence, "collaboration"))
		if err != nil {
			return err
		}
		if !done {
			return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_OLD_OUTPUTS_PENDING")
		}
		branches := make([]uint64, 0, len(fixed.Transition.Branches))
		for _, branch := range fixed.Transition.Branches {
			branches = append(branches, branch.Branch)
		}
		if err = repository.CompleteStructureBranches(ctx, tx, fixed.Context.Target.DocumentId, branches); err != nil {
			return err
		}
	}
	if err = repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_prepare.go：保留原实现，按业务合并文件。

func mainControlPending(operation string) map[string]any {
	return map[string]any{"operation_id": operation, "state": "PREPARING", "poll_url": "/v1/operations/" + operation}
}

func (s *MainControls) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.MainControlIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "MAIN_CONTROL")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "MAIN_CONTROL_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	present := s.presentation(actor, access, intent.Operation)
	if _, err = repository.MainControlResult(ctx, s.db, intent.Operation); err == nil {
		// 同号回执读取仍受当前授权，但不会因旧期待修订再次运行已经完成的业务。
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	structure, _ := domain.CanonicalID(access.Document.CurrentStructureId)
	control, _ := domain.CanonicalRevision(access.Document.ControlRevision)
	if intent.Kind == "CHAPTER_CREATE" {
		intent.Control = control
		if intent.Chapter == nil || intent.Chapter.ExpectedStructureId != access.Document.CurrentStructureId {
			return actor, nil, nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
	}
	if intent.Kind == "STRUCTURE_CHANGE" && (intent.Structure == nil || intent.Structure.ExpectedStructureId != access.Document.CurrentStructureId) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, StructureId: structure, WorkspaceEpoch: control, Purpose: "MAIN_CONTROL"}
	proofs := access.Proofs
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateMainControl(access.Permissions, current.ControlRevision, current.CurrentStyleId, current.MainLocked, intent); err != nil {
			return nil, err
		}
		if intent.Chapter != nil {
			if _, err = validateChapterAnchor(ctx, tx, current, *intent.Chapter); err != nil {
				return nil, err
			}
		}
		var previewBinding *structureConfirmationBinding
		if intent.Structure != nil {
			if domain.HasStructureRangeChange(*intent.Structure) {
				confirmation, err := s.readStructureConfirmation(ctx, tx, actor, access, *intent.Structure)
				if err != nil {
					return nil, err
				}
				previewBinding = &structureConfirmationBinding{Reference: confirmation.Preview.Intent.Operation, Digest: append([]byte(nil), confirmation.Digest...), Expires: confirmation.Expires}
			} else {
				if _, err = stableControlledEntries(ctx, tx, current, *intent.Structure); err != nil {
					return nil, err
				}
			}
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if barrier.Phase != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "MAIN_CONTROL", admission); err != nil {
			return nil, err
		}
		rawTarget, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		fixed := mainControlContext{
			Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs,
			Document: document, Generation: barrier.Generation + 1,
			Intent: intent, Access: access,
			Manifest: manifest, Structure: structure, Style: style, Control: control,
			StructurePreview: previewBinding,
			Context: &pb.OperationContext{
				OperationId: intent.Operation, ActorId: actor.ID,
				IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission,
				Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:],
			},
		}
		original, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		recovery, err := json.Marshal(mainControlRecovery{Fixed: fixed, Attempt: 1})
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterMainControl(ctx, tx, document, fixed.Generation, intent.Operation, original, recovery); err != nil {
			return nil, err
		}
		return mainControlPending(intent.Operation), nil
	}
	return actor, run, present, nil
}

// main_control_readiness.go：保留原实现，按业务合并文件。

func (s *MainControls) Ready(ctx context.Context) error {
	facts, err := repository.PendingMainControlBindings(ctx, s.db)
	if err != nil {
		return errors.New("MAIN_CONTROL_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingMainControlBinding(fact); err != nil {
			return err
		}
		if fact.State == "SUCCEEDED" {
			var fixed mainInstallContext
			if json.Unmarshal(fact.Payload, &fixed) != nil || s.validateControlInstallation(ctx, fixed) != nil {
				return errors.New("MAIN_CONTROL_MANIFEST_UNKNOWN")
			}
		}
	}
	return nil
}

func validatePendingMainControlBinding(fact repository.PendingMainControlBinding) error {
	var original mainControlContext
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Target, &target) != nil || !validMainControlContext(original, fact) {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	path := "/v1/documents/" + domain.TextID(original.Document) + "/main-lock"
	if original.Intent.Kind == "STYLE_CHANGE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/styles"
	}
	if original.Intent.Kind == "CHAPTER_CREATE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/chapters"
	}
	if original.Intent.Kind == "STRUCTURE_CHANGE" {
		path = "/v1/documents/" + domain.TextID(original.Document) + "/structure-changes"
	}
	if target.Path != path {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	// 既检查领取载荷与原事实完全一致，也从原意图重建HTTP摘要；两份意图一起被改仍不能就绪。
	fingerprint, err := mainControlIntentFingerprint(original.Intent, path)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return errors.New("MAIN_CONTROL_BINDING_UNKNOWN")
	}
	if fact.Kind == "MAIN_CONTROL_RECOVER" {
		var recovery mainControlRecovery
		if fact.State != "PREPARING" || json.Unmarshal(fact.Payload, &recovery) != nil || recovery.Attempt == 0 || !sameMainControlContext(recovery.Fixed, original) {
			return errors.New("MAIN_CONTROL_RECOVERY_UNKNOWN")
		}
	} else if fact.Kind == "MAIN_CONTROL_RECONCILE" {
		var install mainInstallContext
		if json.Unmarshal(fact.Payload, &install) != nil ||
			!proto.Equal(install.Context, original.Context) ||
			install.Generation != original.Generation ||
			install.Lineage != nil ||
			(fact.State != "SUCCEEDED" && fact.State != "REJECTED" && fact.State != "CANCELLED") {
			return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
		}
		if fact.State == "SUCCEEDED" {
			if !validStructureInstallation(original, install) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
			var result struct {
				Result map[string]string `json:"result"`
			}
			if json.Unmarshal(fact.Result, &result) != nil ||
				install.Manifest == 0 ||
				result.Result["manifest_id"] != domain.TextID(install.Manifest) ||
				result.Result["entity_id"] != domain.TextID(mainControlResultEntity(install)) ||
				install.Style == 0 ||
				!validControlledStructure(original, install) ||
				install.Control != original.Control+1 ||
				(len(install.Units) == 0 && (!validEmptyControlledManifest(install) || original.Intent.Kind == "CHAPTER_CREATE")) ||
				(original.Intent.Kind == "MAIN_LOCK" && install.Style != original.Style) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
			seen := map[uint64]bool{}
			for _, unit := range install.Units {
				content := unit.GetContent()
				if unit == nil ||
					unit.UnitId == 0 ||
					unit.ContentVersionId == 0 ||
					seen[unit.UnitId] ||
					content == nil ||
					content.DocumentId != original.Document ||
					content.UnitId != unit.UnitId ||
					content.StructureId != install.Structure ||
					content.BranchId != 0 ||
					content.WorkspaceEpoch != install.Control {
					return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
				}
				seen[unit.UnitId] = true
			}
			digest, err := domain.MainManifestDigest(install.Manifest, install.Units)
			if err != nil || !bytes.Equal(digest[:], install.Digest) {
				return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
			}
		} else if install.Manifest != 0 || len(install.Units) != 0 || len(install.Digest) != 0 {
			return errors.New("MAIN_CONTROL_INSTALL_UNKNOWN")
		}
	} else {
		return errors.New("MAIN_CONTROL_TASK_UNKNOWN")
	}
	return nil
}

func sameMainControlContext(a, b mainControlContext) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func mainControlIntentFingerprint(intent domain.MainControlIntent, path string) ([32]byte, error) {
	if intent.Kind == "STRUCTURE_CHANGE" {
		if intent.Structure == nil || intent.Chapter != nil || intent.Style != 0 || len(intent.StyleJSON) != 0 || intent.Locked {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		parsed, err := domain.ParseStructureChange(*intent.Structure)
		if err != nil || parsed.Operation != intent.Operation || parsed.Control != intent.Control {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		return domain.OperationFingerprint(path, *intent.Structure)
	}
	if intent.Structure != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	if intent.Kind == "CHAPTER_CREATE" {
		if intent.Chapter == nil || intent.Chapter.OperationId != intent.Operation || intent.Style != 0 || len(intent.StyleJSON) != 0 || intent.Locked {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		if _, err := domain.ParseChapterCreate(*intent.Chapter); err != nil {
			return [32]byte{}, err
		}
		return domain.OperationFingerprint(path, *intent.Chapter)
	}
	if intent.Chapter != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	if intent.Kind == "MAIN_LOCK" {
		if intent.Style != 0 || len(intent.StyleJSON) != 0 {
			return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
		}
		return domain.OperationFingerprint(path, dto.MainLock{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), Locked: intent.Locked})
	}
	input := dto.StyleChange{OperationId: intent.Operation, ExpectedControlRevision: domain.TextID(intent.Control), ExpectedStyleId: domain.TextID(intent.Style)}
	if json.Unmarshal(intent.StyleJSON, &input.Style) != nil {
		return [32]byte{}, errors.New("MAIN_CONTROL_INTENT_INVALID")
	}
	return domain.OperationFingerprint(path, input)
}

func validMainControlContext(fixed mainControlContext, fact repository.PendingMainControlBinding) bool {
	ctx := fixed.Context
	if ctx == nil ||
		ctx.Target == nil ||
		fixed.Document == 0 ||
		fixed.Generation == 0 ||
		fixed.SessionExpires <= 0 ||
		fixed.Actor != fact.Actor ||
		ctx.ActorId != fact.Actor ||
		ctx.OperationId != fact.Operation ||
		fixed.Intent.Operation != fact.Operation ||
		fixed.Intent.Kind != fact.ReceiptKind ||
		(fixed.Intent.Kind != "MAIN_LOCK" && fixed.Intent.Kind != "STYLE_CHANGE" && fixed.Intent.Kind != "CHAPTER_CREATE" && fixed.Intent.Kind != "STRUCTURE_CHANGE") ||
		!bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) ||
		len(fact.Fingerprint) != 32 {
		return false
	}
	if fixed.Intent.Kind == "CHAPTER_CREATE" && (fixed.Intent.Chapter == nil || fixed.Intent.Chapter.ExpectedStructureId != domain.TextID(fixed.Structure)) {
		return false
	}
	if fixed.Intent.Kind == "STRUCTURE_CHANGE" && (fixed.Intent.Structure == nil || fixed.Intent.Structure.ExpectedStructureId != domain.TextID(fixed.Structure) || fixed.Intent.Structure.ExpectedControlRevision != domain.TextID(fixed.Control)) {
		return false
	}
	if fixed.Intent.Structure != nil && domain.HasStructureRangeChange(*fixed.Intent.Structure) {
		reference, err := domain.StructureConfirmationReference(*fixed.Intent.Structure)
		if err != nil || fixed.StructurePreview == nil || fixed.StructurePreview.Reference != reference || len(fixed.StructurePreview.Digest) != 32 || fixed.StructurePreview.Expires <= 0 {
			return false
		}
	} else if fixed.StructurePreview != nil {
		return false
	}
	if ctx.Target.Purpose != "MAIN_CONTROL" ||
		ctx.Target.DocumentId != fixed.Document ||
		ctx.Target.UnitId != 0 ||
		ctx.Target.BranchId != 0 ||
		ctx.Target.StructureId != fixed.Structure ||
		ctx.Target.WorkspaceEpoch != fixed.Control ||
		fixed.Control != fixed.Intent.Control ||
		fixed.Manifest == 0 ||
		fixed.Style == 0 ||
		fixed.Structure == 0 ||
		fixed.Access.Document.Id != domain.TextID(fixed.Document) ||
		fixed.Access.Document.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		fixed.Access.Document.CurrentStructureId != domain.TextID(fixed.Structure) ||
		fixed.Access.Document.CurrentStyleId != domain.TextID(fixed.Style) ||
		fixed.Access.Document.ControlRevision != domain.TextID(fixed.Control) ||
		!fixed.Access.Permissions.Manage ||
		(fixed.Intent.Kind == "STYLE_CHANGE" && fixed.Intent.Style != fixed.Style) {
		return false
	}
	if ctx.IdentityAdmissionRef == "" ||
		ctx.DocumentAdmissionRef == "" ||
		len(ctx.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: fixed.Access.Sources}) ||
		len(fixed.Access.Proofs) == 0 {
		return false
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil ||
			proof.ActorId != fixed.Actor ||
			proof.OperationId != fact.Operation ||
			proof.Purpose != "MAIN_CONTROL" ||
			!proto.Equal(proof.Target, ctx.Target) {
			return false
		}
	}
	return fixed.Access.Proofs[0].AdmissionId == ctx.IdentityAdmissionRef
}

func mainControlResultEntity(install mainInstallContext) uint64 {
	if install.Entity != 0 {
		return install.Entity
	}
	// 既有锁/样式回执没有此字段，保留不可变结果与原样式身份。
	return install.Style
}

func validControlledStructure(original mainControlContext, install mainInstallContext) bool {
	if original.Intent.Kind == "STRUCTURE_CHANGE" {
		return install.Structure != 0 && install.Structure != original.Structure && install.Entity == install.Structure && install.Style == original.Style
	}
	if original.Intent.Kind == "CHAPTER_CREATE" {
		return install.Structure != 0 && install.Structure != original.Structure && install.Entity != 0 && install.Style == original.Style
	}
	return install.Structure == original.Structure
}

// main_control_readings.go：保留原实现，按业务合并文件。

func (s *MainControls) fixedContext(ctx context.Context, operation string) (mainControlContext, error) {
	var fixed mainControlContext
	raw, err := repository.MergeArtifact(ctx, s.db, "MAIN_CONTROL_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
	}
	return fixed, nil
}

func (s *MainControls) Lookup(ctx context.Context, operation string) (mainControlContext, error) {
	return s.fixedContext(ctx, operation)
}

func (s *MainControls) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}

func (s *MainControls) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.fixedContext(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "MAIN_CONTROL_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "MAIN_CONTROL_RESULT")
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}

func (s *MainControls) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	fact, err := repository.MainControlResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if fact.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "MAIN_CONTROL_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_RESULT_UNKNOWN")
	}
	if fact.State == "SUCCEEDED" {
		ready, err := repository.MainInstallationCompleted(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		sync := "SYNCING"
		if ready {
			sync = "READY"
		}
		// 与MERGE一致：HTTP只投影安装状态，持久系统终态字节与摘要保持原值。
		value["result"].(map[string]any)["main_sync_state"] = sync
	}
	return value, nil
}

func (s *MainControls) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.MainControlResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
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
		Result map[string]string `json:"result"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_RESULT_UNKNOWN")
	}
	for _, key := range []string{"manifest_id", "entity_id"} {
		if text := value.Result[key]; text != "" {
			id, err := domain.CanonicalID(text)
			if err != nil {
				return nil, err
			}
			result.ResultIds = append(result.ResultIds, id)
		}
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// main_control_recovery.go：保留原实现，按业务合并文件。

func (s *MainControls) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimMainControlTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "MAIN_CONTROL_RECONCILE" {
		err = s.reconcile(ctx, task)
	} else {
		var recovery mainControlRecovery
		if json.Unmarshal(task.Payload, &recovery) != nil || recovery.Fixed.Context == nil || recovery.Attempt == 0 {
			err = domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_BINDING_UNKNOWN")
		} else {
			err = s.recoverControl(ctx, task, recovery)
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied ||
				violation.Kind == domain.EntityNotFound ||
				violation.Kind == domain.StateConflict) {
				err = s.reject(ctx, task, recovery.Fixed, violation.Code)
			}
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

// 任务只记录稳定错误类别；底层语句和远端详情可能带有私有数据，不进入持久状态。
func mainControlRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("MAIN_CONTROL_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "MAIN_CONTROL_RPC_" + rpc.Code().String()
	}
	return "MAIN_CONTROL_RECOVERY_UNKNOWN"
}

func (s *MainControls) recoverControl(ctx context.Context, task domain.SubmissionTask, recovery mainControlRecovery) error {
	proofs, err := s.resume(ctx, recovery.Fixed)
	if err != nil {
		return err
	}
	if recovery.Fixed.StructurePreview != nil {
		return s.recoverStructure(ctx, task, recovery, proofs)
	}
	main, err := s.captureMain(ctx, task, recovery)
	if err != nil {
		return err
	}
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: recovery.Fixed.Context, BarrierGeneration: recovery.Fixed.Generation, ExpectedCut: main.Cut})
	if err != nil {
		return err
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return s.advanceAttempt(ctx, task, recovery)
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commit(ctx, task, recovery, main, prepared, proofs)
}

func (s *MainControls) reject(ctx context.Context, task domain.SubmissionTask, fixed mainControlContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = repository.DocumentRow(ctx, tx, fixed.Document, true); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if barrier.Operation == task.Operation && barrier.Generation == fixed.Generation {
		payload, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: fixed.Generation})
		if err != nil {
			return err
		}
		// 即使拒绝也要以原号查Go终态并解除自己的Node准备；未知RPC结果不直接开放旧MAIN。
		if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_CONTROL_RECONCILE", task.Operation, task.Operation, payload, "QUEUED"); err != nil {
			return err
		}
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// main_control_source.go：保留原实现，按业务合并文件。

func (s *MainControls) resume(ctx context.Context, fixed mainControlContext) ([]*pb.IdentityAdmission, error) {
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

func validateMainControlProofs(fixed mainControlContext, proofs []*pb.IdentityAdmission) error {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(proofs) == 0 || len(proofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
	}
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

func (s *MainControls) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) {
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
	if current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) ||
		current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) ||
		current.CurrentStyleId != domain.TextID(input.ExpectedStyleId) ||
		current.ControlRevision != domain.TextID(input.ExpectedControlRevision) {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
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
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, current.CurrentManifestId, input.ExpectedControlRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

// main_control_units.go：保留原实现，按业务合并文件。

func materializeControlledUnits(ctx context.Context, tx *sql.Tx, fixed mainControlContext, layout mainControlLayout, entries []map[string]string, captured map[uint64]*pb.FixedContent, manifest, control uint64) ([]*pb.ManifestUnit, error) {
	units := make([]*pb.ManifestUnit, 0, len(entries)+1)
	for _, entry := range entries {
		unit, _ := domain.CanonicalID(entry["unit_id"])
		version, _ := domain.CanonicalID(entry["content_version_id"])
		content := captured[unit]
		if content == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
		}
		old, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return nil, err
		}
		if layout.Structure == fixed.Structure && bytes.Equal(old.BodyHash, content.BodyHash) {
			if err = canonicalVersion(old, version); err != nil {
				return nil, err
			}
			content = old
		} else {
			// 即使正文相同，结构上下文变化也须创建新版本；不改写旧版本的structure_id。
			content = proto.Clone(content).(*pb.FixedContent)
			content.StructureId = layout.Structure
			version, err = repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId)
			if err != nil {
				return nil, err
			}
		}
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, unit, version, control, content)
		if err != nil {
			return nil, err
		}
		units = append(units, installed)
	}
	if layout.Unit != 0 {
		version, err := initialUnitVersion(ctx, tx, fixed.Document, layout.Unit, layout.Structure, manifest, fixed.Intent.Operation)
		if err != nil {
			return nil, err
		}
		content, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return nil, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return nil, err
		}
		installed, err := controlledMainUnit(ctx, tx, fixed.Document, manifest, layout.Unit, version, control, content)
		if err != nil {
			return nil, err
		}
		units = append(units, installed)
	}
	return units, nil
}

func controlledMainUnit(ctx context.Context, tx *sql.Tx, document, manifest, unit, version, control uint64, content *pb.FixedContent) (*pb.ManifestUnit, error) {
	if err := repository.InsertInitialManifestEntry(ctx, tx, document, manifest, unit, version); err != nil {
		return nil, err
	}
	room, err := collaborationRoom(ctx, tx, "MAIN", unit, unit, control)
	if err != nil {
		return nil, err
	}
	content.BranchId, content.WorkspaceEpoch = 0, control
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(control), "durable_seq": "0"})
	if err != nil {
		return nil, err
	}
	return &pb.ManifestUnit{UnitId: unit, ContentVersionId: version, Content: content}, nil
}

// main_controls.go：保留原实现，按业务合并文件。

type MainControls struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

type mainControlContext struct {
	Actor                               uint64                   `json:"actor"`
	SessionExpires                      int64                    `json:"session_expires"`
	Document                            uint64                   `json:"document"`
	Generation                          uint64                   `json:"generation"`
	Intent                              domain.MainControlIntent `json:"intent"`
	Access                              DocumentAccess           `json:"access"`
	Context                             *pb.OperationContext     `json:"context"`
	Manifest, Structure, Style, Control uint64
	StructurePreview                    *structureConfirmationBinding `json:"structure_preview,omitempty"`
}

type mainControlRecovery struct {
	Fixed   mainControlContext `json:"fixed"`
	Attempt uint64             `json:"attempt"`
}

func NewMainControls(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *MainControls {
	return &MainControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
