# services/document/internal/logic/document/merges_execution_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/210-merges-execution-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`mainInstallContext`、`commitMerge`、`CommittedSource`、`reconcileMain`、`validMainInstallReply`、`RecoverOne`、`recoverMerge`、`supersedeAttempt`、`persistMergeConflicts`、`rejectMerge`、`mergeRecoveryCode`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// merge_commit.go：保留原实现，按业务合并文件。

type mainInstallContext struct {
	Context                   *pb.OperationContext `json:"context"`
	Generation                uint64               `json:"generation"`
	Manifest                  uint64               `json:"manifest"`
	Units                     []*pb.ManifestUnit   `json:"units"`
	Digest                    []byte               `json:"digest"`
	Lineage                   map[string]any       `json:"lineage"`
	Structure, Style, Control uint64
	Entity                    uint64                 `json:"entity,omitempty"`
	Transition                *structureInstallation `json:"structure_transition,omitempty"`
}

func (s *Merges) commitMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, inputs mergeFixedInputs, candidate *pb.MergeCandidateReply, prepared *pb.MainBarrierReply, proofs []*pb.IdentityAdmission) error {
	attempt := inputs.Attempt
	content := candidate.GetCandidate()
	if candidate == nil || candidate.State != pb.CaptureReply_FIXED || len(candidate.Conflicts) != 0 || !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) || content == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_UNKNOWN")
	}
	if err := validateFixedMergeContent(s.validator, content, fixed.Pointer.Document, fixed.Pointer.Unit, attempt.Main.ExpectedStructureId); err != nil {
		return err
	}
	if content.OperationId != task.Operation || content.CaptureId != task.Operation+"-merge-"+domain.TextID(attempt.ID) || content.BranchId != fixed.Pointer.Branch || content.WorkspaceEpoch != fixed.Context.Target.WorkspaceEpoch || content.LineageRef != domain.TextID(attempt.Round.Lineage) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_BINDING_INVALID")
	}
	if !bytes.Equal(content.CutJsonUtf8, inputs.Input.Submitted.CutJsonUtf8) || !validProjectedSubmissionCut(content.CutJsonUtf8, attempt) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMISSION_CUT_CHANGED")
	}
	if err := validateMergeCandidateProvenance(content, attempt); err != nil {
		return err
	}
	cutBytes, err := codec.Canonical(domain.CutBinding(inputs.Main.Cut))
	if err != nil {
		return err
	}
	cutDigest := sha256.Sum256(cutBytes)
	if prepared == nil || prepared.State != pb.MainBarrierReply_PREPARED || prepared.OperationId != task.Operation || prepared.BarrierGeneration != attempt.Generation || !proto.Equal(prepared.PreparedCut, inputs.Main.Cut) || !bytes.Equal(prepared.Digest, cutDigest[:]) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARED_UNKNOWN")
	}
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
	if current.CurrentManifestId != domain.TextID(attempt.Main.ExpectedManifestId) || current.CurrentStructureId != domain.TextID(attempt.Main.ExpectedStructureId) || current.CurrentStyleId != domain.TextID(attempt.Main.ExpectedStyleId) || current.ControlRevision != domain.TextID(attempt.Main.ExpectedControlRevision) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != attempt.Generation || barrier.Phase != "PREPARING" {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	branch, err := repository.LockedActiveBranch(ctx, tx, fixed.Pointer.Unit, fixed.Pointer.Document)
	if err != nil || branch != fixed.Pointer.Branch {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedEditWorkspace(ctx, tx, branch, fixed.Pointer.Document, fixed.Pointer.Unit)
	if err != nil || workspace.Epoch != fixed.Context.Target.WorkspaceEpoch {
		return domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	pointer, err := validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return err
	}
	latest, err := repository.LatestMergeAttempt(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if latest.ID != attempt.ID || latest.State != "FIXED" {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_ATTEMPT_SUPERSEDED")
	}
	entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, pointer.Document)
	if err != nil {
		return err
	}
	if len(entries) != len(inputs.Main.Units) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
	}
	main := map[uint64]*pb.FixedContent{}
	for _, unit := range inputs.Main.Units {
		main[unit.UnitId] = unit
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	// candidate 和 MAIN 可靠尾部引用精确当前 Doc 的计费分配，保留原 resourceId。
	contents := append(append([]*pb.FixedContent(nil), inputs.Main.Units...), content)
	if err = lockFixedContentResources(ctx, tx, fixed.Pointer.Document, space, contents, "MERGE_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	version, err := repository.InsertMergeVersion(ctx, tx, content, "MERGE", content.CaptureId)
	if err != nil {
		return err
	}
	manifest, err := repository.InsertMergedManifest(ctx, tx, pointer.Document, attempt.Main.ExpectedManifestId, attempt.Main.ExpectedStructureId, attempt.Main.ExpectedStyleId, task.Operation, attemptKey(task.Operation, attempt.ID))
	if err != nil {
		return err
	}
	control := attempt.Main.ExpectedControlRevision + 1
	units := make([]*pb.ManifestUnit, 0, len(entries))
	for _, entry := range entries {
		unit, _ := domain.CanonicalID(entry["unit_id"])
		oldVersion, _ := domain.CanonicalID(entry["content_version_id"])
		captured := main[unit]
		if captured == nil {
			return domain.Violation(domain.TemporarilyUnavailable, "MAIN_MANIFEST_INCOMPLETE")
		}
		chosen := oldVersion
		var body *pb.FixedContent
		if unit == pointer.Unit {
			chosen, body = version, proto.Clone(content).(*pb.FixedContent)
		} else {
			old, err := repository.VersionContent(ctx, tx, oldVersion, pointer.Document)
			if err != nil {
				return err
			}
			if bytes.Equal(old.BodyHash, captured.BodyHash) {
				if err = canonicalVersion(old, oldVersion); err != nil {
					return err
				}
				body = old
			} else {
				chosen, err = repository.InsertMergeVersion(ctx, tx, captured, "MAIN_CAPTURE", captured.CaptureId)
				if err != nil {
					return err
				}
				body = proto.Clone(captured).(*pb.FixedContent)
			}
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, pointer.Document, manifest, unit, chosen); err != nil {
			return err
		}
		room, err := collaborationRoom(ctx, tx, "MAIN", unit, unit, control)
		if err != nil {
			return err
		}
		body.BranchId, body.WorkspaceEpoch = 0, control
		body.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(control), "durable_seq": "0"})
		if err != nil {
			return err
		}
		units = append(units, &pb.ManifestUnit{UnitId: unit, ContentVersionId: chosen, Content: body})
	}
	record, err := repository.InsertMergeRecord(ctx, tx, task.Operation, pointer, attempt.ID, attempt.Main.ExpectedManifestId, manifest, version, attempt.Round.Lineage, attempt.Round.Capture)
	if err != nil {
		return err
	}
	if _, err = repository.InsertMergeApproval(ctx, tx, task.Operation, pointer, fixed.Actor, fixed.Intent.Version, record); err != nil {
		return err
	}
	for i, resolution := range fixed.Intent.Resolutions {
		if err = repository.InsertMergedResolution(ctx, tx, task.Operation, pointer, fixed.Actor, resolution.ID, i+1); err != nil {
			return err
		}
	}
	lineage, err := s.acceptMergeLineage(ctx, tx, fixed, inputs, workspace, record, manifest)
	if err != nil {
		return err
	}
	digest, err := domain.MainManifestDigest(manifest, units)
	if err != nil {
		return err
	}
	install, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: attempt.Generation, Manifest: manifest, Units: units, Digest: digest[:], Lineage: lineage, Structure: attempt.Main.ExpectedStructureId, Style: attempt.Main.ExpectedStyleId, Control: control})
	if err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"merge_record_id": domain.TextID(record), "manifest_id": domain.TextID(manifest), "content_version_id": domain.TextID(version), "request_id": domain.TextID(pointer.ID), "round_id": domain.TextID(pointer.Round), "main_sync_state": "SYNCING"}}
	if err = repository.CompleteMerge(ctx, tx, task, pointer, fixed.Actor, record, manifest, attempt.ID, attempt.Generation, result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// merge_install.go：保留原实现，按业务合并文件。

func (s *Merges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
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
	return map[string]any{"generation": domain.TextID(fixed.Generation), "manifest_id": domain.TextID(fixed.Manifest), "structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style), "control_revision": domain.TextID(fixed.Control), "manifest_digest": base64.StdEncoding.EncodeToString(fixed.Digest), "units": units, "lineage": fixed.Lineage}, nil
}

func (s *Merges) reconcileMain(ctx context.Context, task domain.SubmissionTask) error {
	var fixed mainInstallContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RECONCILE_BINDING_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Context.Target, RequestFingerprint: fixed.Context.RequestFingerprint}
	result, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if result.State != pb.OperationResult_SUCCEEDED && result.State != pb.OperationResult_REJECTED && result.State != pb.OperationResult_CANCELLED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_RESULT_UNKNOWN")
	}
	reply, err := s.client.ReconcileMain(ctx, &pb.ReconcileMainCommitRequest{Context: fixed.Context, BarrierGeneration: fixed.Generation, AuthoritativeResult: result, ManifestId: fixed.Manifest, Units: fixed.Units, ManifestDigest: fixed.Digest})
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
	if err = repository.CompleteMainInstallation(ctx, tx, task, fixed.Context.Target.DocumentId, fixed.Generation, reply); err != nil {
		return err
	}
	return tx.Commit()
}

// merge_install_ack.go：保留原实现，按业务合并文件。

// 安装成功枚举不足以解除屏障：必须逐项确认原完整清单安装到原房间代次与零切点。
func validMainInstallReply(fixed mainInstallContext, succeeded bool, reply *pb.InstallReply) bool {
	if fixed.Context == nil || reply == nil || reply.OperationId != fixed.Context.OperationId || reply.BarrierGeneration != fixed.Generation {
		return false
	}
	if !succeeded {
		return reply.State == pb.InstallReply_ABORTED && reply.ManifestId == 0 && len(reply.InstalledEpochs) == 0 && len(reply.ManifestDigest) == 0
	}
	expectedCount := len(fixed.Units)
	if fixed.Transition != nil {
		expectedCount += len(fixed.Transition.Branches)
	}
	if reply.State != pb.InstallReply_READY || reply.ManifestId != fixed.Manifest || len(fixed.Digest) != 32 ||
		!bytes.Equal(reply.ManifestDigest, fixed.Digest) || len(reply.InstalledEpochs) != expectedCount || (len(fixed.Units) == 0 && !validEmptyControlledManifest(fixed)) {
		return false
	}
	expected := map[string]map[string]string{}
	for _, unit := range fixed.Units {
		var cut map[string]string
		if unit == nil || unit.Content == nil || json.Unmarshal(unit.Content.CutJsonUtf8, &cut) != nil ||
			cut["room_id"] == "" || cut["workspace_epoch"] == "" || cut["durable_seq"] != "0" || expected[cut["room_id"]] != nil {
			return false
		}
		expected[cut["room_id"]] = cut
	}
	if fixed.Transition != nil {
		for _, branch := range fixed.Transition.Branches {
			var cut map[string]string
			if branch.Content == nil || json.Unmarshal(branch.Content.CutJsonUtf8, &cut) != nil || cut["room_id"] != domain.TextID(branch.Room) || cut["workspace_epoch"] != domain.TextID(branch.Epoch) || cut["durable_seq"] != "0" || branch.Room == 0 || branch.Epoch == 0 || expected[cut["room_id"]] != nil {
				return false
			}
			expected[cut["room_id"]] = cut
		}
	}
	for _, installed := range reply.InstalledEpochs {
		if installed == nil {
			return false
		}
		key := domain.TextID(installed.RoomId)
		cut, exists := expected[key]
		if !exists || cut["workspace_epoch"] != domain.TextID(installed.WorkspaceEpoch) || installed.DurableSeq != 0 {
			return false
		}
		delete(expected, key)
	}
	return len(expected) == 0
}

// merge_recovery.go：保留原实现，按业务合并文件。

func (s *Merges) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimMergeTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "MAIN_RECONCILE" {
		err = s.reconcileMain(ctx, task)
	} else {
		var fixed mergeContext
		if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
			err = domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
		} else {
			err = s.recoverMerge(ctx, task, fixed)
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				err = s.rejectMerge(ctx, task, fixed, violation.Code)
			}
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mergeRecoveryCode(err))
	}
}

func (s *Merges) recoverMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	if err = s.ensureSubmissionCut(ctx, task, fixed); err != nil {
		return err
	}
	attempt, err := s.reserveAttempt(ctx, task, fixed, proofs)
	if err != nil {
		return err
	}
	inputs, err := s.fixInputs(ctx, task, fixed, attempt)
	if err != nil {
		return err
	}
	candidate, err := s.client.Compute(ctx, inputs.Input)
	if err != nil {
		return err
	}
	if candidate == nil || candidate.State != pb.CaptureReply_FIXED || !bytes.Equal(candidate.InputDigest, inputs.Input.InputDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_CANDIDATE_UNKNOWN")
	}
	if len(candidate.Conflicts) != 0 {
		if err = s.persistMergeConflicts(ctx, task, attempt, candidate); err != nil {
			return err
		}
		return domain.Violation(domain.StateConflict, "MERGE_CONFLICT_REQUIRES_RESOLUTION")
	}
	prepared, err := s.client.PrepareMain(ctx, &pb.PrepareMainCommitRequest{Context: fixed.Context, BarrierGeneration: attempt.Generation, ExpectedCut: inputs.Main.Cut})
	if err != nil {
		return err
	}
	if prepared == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_UNKNOWN")
	}
	if prepared.State == pb.MainBarrierReply_CHANGED {
		return s.supersedeAttempt(ctx, task, fixed, attempt)
	}
	if prepared.State != pb.MainBarrierReply_PREPARED {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_PREPARE_PENDING")
	}
	return s.commitMerge(ctx, task, fixed, inputs, candidate, prepared, proofs)
}

func (s *Merges) supersedeAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) error {
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
	if _, err = ValidateAccess(ctx, tx, fixed.Access); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if barrier.Operation != task.Operation || barrier.Generation != attempt.Generation {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_BARRIER_SUPERSEDED")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 只有Node明确CHANGED（未准备）才允许下一次尝试重新采尾；超时保持原attempt与屏障。
	if err = repository.SupersedeMergeAttempt(ctx, tx, task.Operation, attempt.ID, fixed.Pointer.Document, attempt.Generation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CUT_CHANGED_RETRY")
}

func (s *Merges) persistMergeConflicts(ctx context.Context, task domain.SubmissionTask, attempt mergeAttemptContext, candidate *pb.MergeCandidateReply) error {
	raw, err := json.Marshal(candidate.Conflicts)
	if err != nil {
		return err
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
	if err = repository.StoreMergeConflict(ctx, tx, attempt.ID, raw); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Merges) rejectMerge(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	// 原来源已关闭时也必须完成原号拒绝；这条事务只撤销自己的预约，不写正文/意见/批准。
	if _, err = repository.DocumentRow(ctx, tx, fixed.Pointer.Document, true); err != nil {
		return err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	if _, err = repository.LockedReviewGuard(ctx, tx, fixed.Pointer.Unit); err != nil {
		return err
	}
	if _, err = repository.LockedReviewPointer(ctx, tx, fixed.Pointer.ID); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 持久终态也使用公开Error契约。原操作号同时作为追踪标识，重试读取不会产生另一份错误事实。
	result := map[string]any{
		"operation_id": task.Operation,
		"state":        "REJECTED",
		"error": map[string]string{
			"code": code, "message": code,
			"trace_id": task.Operation, "retry_action": "NONE",
		},
	}
	if attempt, err := repository.LatestMergeAttempt(ctx, tx, task.Operation); err == nil && attempt.State == "CONFLICT" {
		result["result"] = map[string]string{"conflict_context_ref": task.Operation}
	} else if err != nil && err != sql.ErrNoRows {
		return err
	}
	if barrier.Operation == task.Operation && barrier.Phase != "OPEN" {
		payload, err := json.Marshal(mainInstallContext{Context: fixed.Context, Generation: barrier.Generation})
		if err != nil {
			return err
		}
		if err = repository.InsertSubmissionTask(ctx, tx, "MAIN_RECONCILE", task.Operation, task.Operation, payload, "QUEUED"); err != nil {
			return err
		}
	}
	if err = repository.RejectMerge(ctx, tx, task, fixed.Pointer.ID, code, result); err != nil {
		return err
	}
	return tx.Commit()
}

func mergeRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	if rpc, ok := status.FromError(err); ok {
		return "MERGE_RPC_" + rpc.Code().String()
	}
	return "MERGE_RECOVERY_UNAVAILABLE"
}
````

UTF-8/LF全文SHA-256：`d6cb2b5e5f1017b0450df73388491ce7a3bd72d2e0db971b2883bb0d70d7eaa4`。[参考实现](../../../../services/document/internal/logic/document/merges_execution_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
