# services/document/internal/logic/document/merges_preparation_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/211-merges-preparation-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`projectMergeBase`、`projectMergeSubmitted`、`projectStoredMergeBase`、`mergeAttemptContext`、`mergeFixedInputs`、`attemptKey`、`reserveAttempt`、`fixInputs`、`validateMainCaptureDigest`、`validateFixedMergeContent`、`validateMergeCandidateProvenance`、`acceptMergeLineage`、`fixedContext`、`resume`、`validateMergeProofs`、`MainSource`、`canonicalVersion`、`fixedWire`、`Prepare`、`mergePending`、`validateMergeReview`、`validProjectedSubmissionCut`、`requireStableReviewRange`、`stableMergeRange`、`projectMergeVersion`、`Ready`、`validatePendingMergeBinding`、`validMergeRecoveryContext`、`validMainInstallBinding`、`Lookup`、`Progress`、`mergePresentation`、`readMergeResult`、`OperationResult`、`readSubmissionCut`、`ensureSubmissionCut`、`MergeClient`、`Merges`、`mergeContext`、`NewMerges`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"time"
)

// merge_base_range.go：保留原实现，按业务合并文件。

// 这里只投影计算上下文，不更新旧content_version；拆/合导致的范围变化不能走这个稳定身份快捷路径。
func projectMergeBase(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext) (*pb.FixedContent, error) {
	if attempt.Main == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BASE_UNKNOWN")
	}
	return projectMergeVersion(ctx, tx, attempt.Base, attempt.Round.BaseVersion, attempt.Main.ExpectedStructureId, nil)
}

func projectMergeSubmitted(ctx context.Context, tx *sql.Tx, attempt mergeAttemptContext, round uint64) (*pb.FixedContent, error) {
	if attempt.Main == nil || attempt.Submitted == nil || attempt.Main.Context == nil || attempt.Main.Context.Target == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMITTED_UNKNOWN")
	}
	target := attempt.Main.Context.Target
	if attempt.Submitted.BranchId != target.BranchId || attempt.Submitted.WorkspaceEpoch != target.WorkspaceEpoch || attempt.Submitted.LineageRef != domain.TextID(attempt.Round.Lineage) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_SUBMITTED_BINDING_UNKNOWN")
	}
	if attempt.Submitted.StructureId == attempt.Main.ExpectedStructureId {
		return attempt.Submitted, nil
	}
	cut, err := readSubmissionCut(ctx, tx, round)
	if err != nil {
		return nil, err
	}
	return projectMergeVersion(ctx, tx, attempt.Submitted, attempt.Round.SubmittedVersion, attempt.Main.ExpectedStructureId, cut)
}

func (s *Merges) projectStoredMergeBase(ctx context.Context, task domain.SubmissionTask, fixed mergeFixedInputs, key string) (mergeFixedInputs, error) {
	if fixed.Input == nil || fixed.Input.Base == nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
	}
	if fixed.Input.Base.StructureId == fixed.Input.Context.GetTarget().GetStructureId() {
		return fixed, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fixed, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return fixed, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return fixed, err
	}
	const kind = "MERGE_ATTEMPT_RANGE_FIXED"
	if raw, readErr := repository.MergeArtifact(ctx, tx, kind, key); readErr == nil {
		var result mergeFixedInputs
		if json.Unmarshal(raw, &result) != nil {
			return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_INPUTS_UNKNOWN")
		}
		return result, nil
	} else if readErr != sql.ErrNoRows {
		return fixed, readErr
	}
	if !proto.Equal(fixed.Input.Base, fixed.Attempt.Base) {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BASE_BINDING_UNKNOWN")
	}
	original := append([]byte(nil), fixed.Input.InputDigest...)
	projected, err := projectMergeBase(ctx, tx, fixed.Attempt)
	if err != nil {
		return fixed, err
	}
	fixed.Input = proto.Clone(fixed.Input).(*pb.MergeCandidateRequest)
	fixed.Input.Base = projected
	digest, err := domain.MergeInputDigest(fixed.Input)
	if err != nil {
		return fixed, err
	}
	fixed.Input.InputDigest = digest[:]
	raw, err := json.Marshal(fixed)
	if err != nil {
		return fixed, err
	}
	// 旧生产者的混结构输入在Node注册候选前必然InvalidArgument；只追加带范围证据的派生输入。
	// 原MAIN切点、轮次、字节及MERGE_ATTEMPT_FIXED保持，更新仍须原worker代次和准确旧摘要。
	if bytes.Equal(digest[:], original) {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_RANGE_DIGEST_UNKNOWN")
	}
	if err = repository.FixProjectedMergeAttempt(ctx, tx, fixed.Attempt.ID, original, digest[:], raw, key, task.Operation); err != nil {
		return fixed, err
	}
	return fixed, tx.Commit()
}

// merge_compute.go：保留原实现，按业务合并文件。

type mergeAttemptContext struct {
	ID, Generation  uint64
	Round           repository.MergeRoundInput
	Main            *pb.CaptureMainRequest
	Base, Submitted *pb.FixedContent
	SubmissionCut   json.RawMessage
}

type mergeFixedInputs struct {
	Attempt mergeAttemptContext
	Main    *pb.MainCaptureReply
	Input   *pb.MergeCandidateRequest
}

func attemptKey(operation string, id uint64) string { return operation + ":" + domain.TextID(id) }

func (s *Merges) reserveAttempt(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, proofs []*pb.IdentityAdmission) (mergeAttemptContext, error) {
	var attempt mergeAttemptContext
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return attempt, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return attempt, err
	}
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return attempt, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return attempt, err
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if barrier.Phase != "OPEN" && barrier.Operation != task.Operation {
		return attempt, domain.Violation(domain.TemporarilyUnavailable, "MAIN_COMMIT_PENDING")
	}
	if _, err = validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true); err != nil {
		return attempt, err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return attempt, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return attempt, err
	}
	prior, err := repository.LatestMergeAttempt(ctx, tx, task.Operation)
	if err != nil && err != sql.ErrNoRows {
		return attempt, err
	}
	if err == nil && prior.State != "SUPERSEDED" {
		raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_CONTEXT", attemptKey(task.Operation, prior.ID))
		if err != nil {
			return attempt, err
		}
		if json.Unmarshal(raw, &attempt) != nil {
			return attempt, domain.Violation(domain.TemporarilyUnavailable, "MERGE_ATTEMPT_UNKNOWN")
		}
		if err = tx.Commit(); err != nil {
			return attempt, err
		}
		return attempt, nil
	}
	round, err := repository.MergeRoundInputs(ctx, tx, fixed.Pointer.ID, fixed.Intent.Round)
	if err != nil {
		return attempt, err
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	structure, _ := domain.CanonicalID(current.CurrentStructureId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalID(current.ControlRevision)
	if round.SubmittedVersion != fixed.Intent.Version {
		return attempt, domain.Violation(domain.StateConflict, "REVIEW_CONTENT_CHANGED")
	}
	if round.Structure != structure {
		if err = requireStableReviewRange(ctx, tx, fixed.Pointer.Document, fixed.Pointer.Unit, round.Structure, structure); err != nil {
			return attempt, err
		}
	}
	base, err := repository.VersionContent(ctx, tx, round.BaseVersion, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if err = canonicalVersion(base, round.BaseVersion); err != nil {
		return attempt, err
	}
	submitted, err := repository.VersionContent(ctx, tx, round.SubmittedVersion, fixed.Pointer.Document)
	if err != nil {
		return attempt, err
	}
	if err = canonicalVersion(submitted, round.SubmittedVersion); err != nil {
		return attempt, err
	}
	submitted.BranchId, submitted.WorkspaceEpoch, submitted.LineageRef = fixed.Pointer.Branch, fixed.Context.Target.WorkspaceEpoch, domain.TextID(round.Lineage)
	cut, err := readSubmissionCut(ctx, tx, fixed.Intent.Round)
	if err != nil {
		return attempt, err
	}
	// submitted的切点是原送审确认；不能把S2当前尾部或合并后的MAIN切点替代它。
	submitted.CutJsonUtf8 = cut
	generation := barrier.Generation + 1
	id, err := repository.ReserveMergeAttempt(ctx, tx, fixed.Intent, fixed.Pointer.Document, prior.Number+1, manifest, structure, style, round.Lineage, round.BaseVersion, generation)
	if err != nil {
		return attempt, err
	}
	attempt = mergeAttemptContext{ID: id, Generation: generation, Round: round, Base: base, Submitted: submitted, SubmissionCut: cut, Main: &pb.CaptureMainRequest{Context: fixed.Context, ExpectedManifestId: manifest, ExpectedControlRevision: control, ExpectedStructureId: structure, ExpectedStyleId: style, Usage: "MERGE_ATTEMPT:" + domain.TextID(id)}}
	payload, err := json.Marshal(attempt)
	if err != nil {
		return attempt, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "MERGE_ATTEMPT_CONTEXT", attemptKey(task.Operation, id), task.Operation, payload, "SUCCEEDED"); err != nil {
		return attempt, err
	}
	return attempt, tx.Commit()
}

func (s *Merges) fixInputs(ctx context.Context, task domain.SubmissionTask, fixed mergeContext, attempt mergeAttemptContext) (mergeFixedInputs, error) {
	var result mergeFixedInputs
	key := attemptKey(task.Operation, attempt.ID)
	raw, err := repository.MergeArtifact(ctx, s.db, "MERGE_ATTEMPT_FIXED", key)
	if err == nil {
		if json.Unmarshal(raw, &result) != nil {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
		}
		return s.projectStoredMergeBase(ctx, task, result, key)
	}
	if err != sql.ErrNoRows {
		return result, err
	}
	main, err := s.client.CaptureMain(ctx, attempt.Main)
	if err != nil {
		return result, err
	}
	if main == nil || main.State != pb.CaptureReply_FIXED || main.Cut == nil || main.Cut.DocumentId != fixed.Pointer.Document || main.Cut.StructureId != attempt.Main.ExpectedStructureId || main.Cut.StyleId != attempt.Main.ExpectedStyleId || main.Cut.ControlRevision != attempt.Main.ExpectedControlRevision {
		return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNKNOWN")
	}
	if err = validateMainCaptureDigest(main); err != nil {
		return result, err
	}
	var selected *pb.FixedContent
	seen := map[uint64]bool{}
	for _, content := range main.Units {
		if content == nil || seen[content.UnitId] {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_INVALID")
		}
		seen[content.UnitId] = true
		if content.BranchId != 0 || content.WorkspaceEpoch != attempt.Main.ExpectedControlRevision || content.OperationId != task.Operation {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_BINDING_INVALID")
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Pointer.Document, content.UnitId, attempt.Main.ExpectedStructureId); err != nil {
			return result, err
		}
		if content.UnitId == fixed.Pointer.Unit {
			selected = content
		}
	}
	if selected == nil {
		return result, domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_UNIT_MISSING")
	}
	input := &pb.MergeCandidateRequest{Context: fixed.Context, AttemptId: domain.TextID(attempt.ID), Base: attempt.Base, Submitted: attempt.Submitted, Main: selected, MainCut: main.Cut}
	digest, err := domain.MergeInputDigest(input)
	if err != nil {
		return result, err
	}
	input.InputDigest = digest[:]
	result = mergeFixedInputs{Attempt: attempt, Main: main, Input: input}
	payload, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return result, err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return result, err
	}
	base, err := projectMergeBase(ctx, tx, attempt)
	if err != nil {
		return result, err
	}
	input.Base = base
	input.Submitted, err = projectMergeSubmitted(ctx, tx, attempt, fixed.Intent.Round)
	if err != nil {
		return result, err
	}
	digest, err = domain.MergeInputDigest(input)
	if err != nil {
		return result, err
	}
	input.InputDigest = digest[:]
	result.Input = input
	payload, err = json.Marshal(result)
	if err != nil {
		return result, err
	}
	if known, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key); err == nil {
		if json.Unmarshal(known, &result) != nil {
			return result, domain.Violation(domain.TemporarilyUnavailable, "MERGE_INPUTS_UNKNOWN")
		}
	} else if err != sql.ErrNoRows {
		return result, err
	} else {
		if err = repository.FixMergeAttempt(ctx, tx, attempt.ID, digest[:], payload, key, task.Operation); err != nil {
			return result, err
		}
	}
	return result, tx.Commit()
}

// merge_content_validation.go：保留原实现，按业务合并文件。

// 跨服务固定字节在事务外校验，未知摘要或不完整正文不能进入正式主线写入。
func validateMainCaptureDigest(main *pb.MainCaptureReply) error {
	cut := domain.CutBinding(main.Cut)
	rooms, err := codec.Canonical(cut["rooms"])
	if err != nil {
		return err
	}
	dependencies := sha256.Sum256(rooms)
	if !bytes.Equal(dependencies[:], main.Cut.DependencyDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_DEPENDENCY_DIGEST_INVALID")
	}
	units := make([]any, 0, len(main.Units))
	for _, unit := range main.Units {
		if unit == nil {
			return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_INVALID")
		}
		wire, err := fixedWire(unit)
		if err != nil {
			return err
		}
		units = append(units, wire)
	}
	raw, err := codec.Canonical(map[string]any{"cut": cut, "units": units})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if !bytes.Equal(digest[:], main.ResultDigest) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_CAPTURE_DIGEST_INVALID")
	}
	return nil
}

func validateFixedMergeContent(validator *codec.Validator, content *pb.FixedContent, document, unit, structure uint64) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MERGE_FIXED_CONTENT_INVALID")
	if content == nil || content.DocumentId != document || content.UnitId != unit || content.StructureId != structure || content.SchemaVersion != "content-v1" || content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" || content.ProvenanceSchema != "provenance-v1" || len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) {
		return invalid
	}
	body, err := codec.Decode(content.BodyJsonUtf8)
	if err != nil || validator.Body(body) != nil {
		return invalid
	}
	canonical, err := codec.Canonical(body)
	if err != nil || !bytes.Equal(canonical, content.BodyJsonUtf8) {
		return invalid
	}
	provenance, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil || validator.Provenance(provenance) != nil {
		return invalid
	}
	canonical, err = codec.Canonical(provenance)
	if err != nil || !bytes.Equal(canonical, content.ProvenanceJsonUtf8) {
		return invalid
	}
	if err = domain.ValidateFixedContentResources(body, content); err != nil {
		return err
	}
	return nil
}

// merge_lineage.go：保留原实现，按业务合并文件。

func validateMergeCandidateProvenance(content *pb.FixedContent, attempt mergeAttemptContext) error {
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	original, err := codec.Decode(attempt.Submitted.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := value.(map[string]any)
	if provenance["lineage_id"] != domain.TextID(attempt.Round.Lineage) || provenance["base_manifest_id"] != domain.TextID(attempt.Round.BaseManifest) || provenance["base_content_version_id"] != domain.TextID(attempt.Round.BaseVersion) || provenance["source_operation_id"] != content.OperationId {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_PROVENANCE_BINDING_INVALID")
	}
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return err
	}
	before, err := codec.Canonical(original.(map[string]any)["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, before) {
		return domain.Violation(domain.TemporarilyUnavailable, "MERGE_ACCEPTED_CAPTURES_CHANGED")
	}
	return nil
}

func (s *Merges) acceptMergeLineage(ctx context.Context, tx *sql.Tx, fixed mergeContext, inputs mergeFixedInputs, workspace domain.EditWorkspace, record, manifest uint64) (map[string]any, error) {
	id, err := NewEntityID()
	if err != nil {
		return nil, err
	}
	value, err := codec.Decode(inputs.Attempt.Submitted.ProvenanceJsonUtf8)
	if err != nil {
		return nil, err
	}
	provenance := value.(map[string]any)
	_, _, previousRaw, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Pointer.Branch)
	if err != nil {
		return nil, err
	}
	previous, err := codec.Decode(previousRaw)
	if err != nil {
		return nil, err
	}
	accepted, ok := previous.(map[string]any)["accepted_captures"].([]any)
	if !ok {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LINEAGE_ACCEPTED_UNKNOWN")
	}
	var cut map[string]string
	if json.Unmarshal(inputs.Attempt.SubmissionCut, &cut) != nil || cut["room_id"] == "" || cut["workspace_epoch"] == "" || cut["durable_seq"] == "" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	provenance["lineage_id"] = domain.TextID(id)
	provenance["base_manifest_id"] = domain.TextID(manifest)
	// S1是已接纳的共同草稿基线；未拉入的MAIN兼容新增不能在下一次S2送审时被误判成删除。
	provenance["base_content_version_id"] = domain.TextID(fixed.Intent.Version)
	provenance["source_operation_id"] = fixed.Intent.Operation
	provenance["accepted_captures"] = append(append([]any{}, accepted...), map[string]any{"capture_id": inputs.Attempt.Round.Capture, "submitted_content_version_id": domain.TextID(fixed.Intent.Version), "merge_record_id": domain.TextID(record), "cut": []any{map[string]any{"room_id": cut["room_id"], "epoch": cut["workspace_epoch"], "durable_seq": cut["durable_seq"]}}})
	if err = s.validator.Provenance(provenance); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LINEAGE_ACCEPTED_INVALID")
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return nil, err
	}
	if err = repository.InsertAcceptedLineage(ctx, tx, id, fixed.Pointer, workspace.Lineage, manifest, fixed.Intent.Version, fixed.Intent.Operation, inputs.Attempt.Round.Capture, raw); err != nil {
		return nil, err
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", fixed.Pointer.Branch, fixed.Pointer.Unit, workspace.Epoch)
	if err != nil {
		return nil, err
	}
	body, err := codec.Decode(inputs.Attempt.Submitted.BodyJsonUtf8)
	if err != nil {
		return nil, err
	}
	return map[string]any{"room_id": domain.TextID(room), "epoch": domain.TextID(workspace.Epoch), "previous_lineage_id": domain.TextID(workspace.Lineage), "context": map[string]any{"lineage_id": domain.TextID(id), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(fixed.Intent.Version), "body": body, "provenance": provenance}}, nil
}

// merge_main_source.go：保留原实现，按业务合并文件。

func (s *Merges) fixedContext(ctx context.Context, operation string) (mergeContext, error) {
	var fixed mergeContext
	payload, err := repository.MergePayload(ctx, s.db, operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(payload, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || len(fixed.Access.Proofs) == 0 {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
	}
	return fixed, nil
}

// 短期证明续接在SQL锁外执行；所有最终写入仍须检查原会话期限、来源和当前文档事实。
func (s *Merges) resume(ctx context.Context, fixed mergeContext) ([]*pb.IdentityAdmission, error) {
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

func validateMergeProofs(fixed mergeContext, proofs []*pb.IdentityAdmission) error {
	if len(proofs) != len(fixed.Access.Proofs) || fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

// 仅向协作服务提供已登记MERGE原号的全篇固定清单；不能让mTLS调用方任意指定正文或版本。
func (s *Merges) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
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
	if current.CurrentManifestId != domain.TextID(input.ExpectedManifestId) || current.CurrentStructureId != domain.TextID(input.ExpectedStructureId) || current.CurrentStyleId != domain.TextID(input.ExpectedStyleId) || current.ControlRevision != domain.TextID(input.ExpectedControlRevision) {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	if _, err = validateMergeReview(ctx, tx, fixed.Actor, fixed.Access, fixed.Pointer, fixed.Intent, true); err != nil {
		return nil, err
	}
	if err = validateMergeProofs(fixed, proofs); err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Pointer.Document, current.CurrentManifestId, input.ExpectedControlRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

func canonicalVersion(content *pb.FixedContent, version uint64) error {
	digest := sha256.Sum256(content.BodyJsonUtf8)
	if !bytes.Equal(digest[:], content.BodyHash) || content.ByteSize != uint64(len(content.BodyJsonUtf8)) || len(content.BodyJsonUtf8) > 16<<20 {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_VERSION_INVALID")
	}
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	content.ProvenanceJsonUtf8, err = codec.Canonical(value)
	if err != nil {
		return err
	}
	provenance, ok := value.(map[string]any)
	if !ok {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_PROVENANCE_INVALID")
	}
	content.LineageRef, _ = provenance["lineage_id"].(string)
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"content_version_id": domain.TextID(version), "source_ref": content.CaptureId})
	return err
}

func fixedWire(content *pb.FixedContent) (map[string]any, error) {
	raw, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(content)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	err = json.Unmarshal(raw, &value)
	return value, err
}

// merge_prepare.go：保留原实现，按业务合并文件。

func (s *Merges) Prepare(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.ApproveMerge, path string) (Actor, MutationBody, MutationPresentation, error) {
	intent, err := domain.ParseMerge(input)
	if err != nil {
		return actor, nil, nil, err
	}
	access, err := s.access.Authorize(ctx, actor, pointer.Document, "MERGE")
	if err != nil {
		return actor, nil, nil, err
	}
	if !reviewContextAllowed(access, pointer) {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	present := s.mergePresentation(actor, access, pointer, intent.Operation)
	if _, err = repository.MergeRow(ctx, s.db, intent.Operation); err == nil {
		// 回执复用由通用事务执行，不能因现在的轮次/资格改变再次写一次批准。
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, pointer.Branch, pointer.Document, pointer.Unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, nil, nil, err
	}
	target := &pb.Target{DocumentId: pointer.Document, UnitId: pointer.Unit, BranchId: pointer.Branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "MERGE"}
	proofs := access.Proofs
	access.Proofs = make([]*pb.IdentityAdmission, 0, len(proofs))
	for _, proof := range proofs {
		bound, err := s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, intent.Operation, target)
		if err != nil {
			return actor, nil, nil, err
		}
		if !proto.Equal(&pb.OperationContext{Sources: proof.Sources}, &pb.OperationContext{Sources: bound.Sources}) {
			return actor, nil, nil, domain.Violation(domain.PermissionDenied, "IDENTITY_CHANGED")
		}
		access.Proofs = append(access.Proofs, bound)
	}
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, err := validateMergeReview(ctx, tx, actor.ID, access, pointer, intent, false)
		if err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, pointer.Document, "MERGE", admission); err != nil {
			return nil, err
		}
		targetJSON, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, pointer.Unit, intent.Operation, targetJSON, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		binding := &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
		payload, err := json.Marshal(mergeContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Pointer: current, Intent: intent, Access: access, Context: binding})
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterMerge(ctx, tx, actor.ID, current.ID, intent, payload); err != nil {
			return nil, err
		}
		return mergePending(intent.Operation), nil
	}
	return actor, run, present, nil
}

func mergePending(operation string) map[string]any {
	return map[string]any{"operation_id": operation, "state": "PREPARING", "poll_url": "/v1/merge-operations/" + operation}
}

func validateMergeReview(ctx context.Context, tx *sql.Tx, actor uint64, access DocumentAccess, pointer domain.ReviewPointer, intent domain.MergeIntent, reserved bool) (domain.ReviewPointer, error) {
	guard, err := repository.LockedReviewGuard(ctx, tx, pointer.Unit)
	if err != nil {
		return pointer, err
	}
	current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
	if err != nil {
		return current, err
	}
	active, err := repository.LockedActiveMerge(ctx, tx, current.ID)
	if err != nil {
		return current, err
	}
	version, err := repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)
	if err != nil {
		return current, err
	}
	if err = domain.ValidateMergePosition(current, guard, active, intent, version); err != nil {
		return current, err
	}
	if reserved && active != intent.Operation {
		return current, domain.Violation(domain.StateConflict, "MERGE_RESERVATION_CHANGED")
	}
	ever, err := repository.EverSubmitted(ctx, tx, current.ID, actor)
	if err != nil {
		return current, err
	}
	if !domain.CanApproveMerge(access.Permissions, ever) {
		return current, domain.Violation(domain.PermissionDenied, "MERGE_APPROVAL_NOT_ALLOWED")
	}
	changes, err := repository.LockedReviewChanges(ctx, tx, current.ID)
	if err != nil {
		return current, err
	}
	return current, domain.ValidateMergeChanges(actor, changes, intent.Resolutions)
}

// merge_range_cut.go：保留原实现，按业务合并文件。

func validProjectedSubmissionCut(projected []byte, attempt mergeAttemptContext) bool {
	if attempt.Submitted == nil || attempt.Main == nil {
		return false
	}
	original := attempt.Submitted.CutJsonUtf8
	if bytes.Equal(projected, original) {
		return true
	}
	var cut map[string]json.RawMessage
	if json.Unmarshal(projected, &cut) != nil {
		return false
	}
	var proof struct {
		Source dto.StructureRevision `json:"source_structure"`
		Target dto.StructureRevision `json:"target_structure"`
		Unit   string                `json:"unit_id"`
	}
	if json.Unmarshal(cut["range_projection"], &proof) != nil || proof.Source.Id != domain.TextID(attempt.Submitted.StructureId) || proof.Target.Id != domain.TextID(attempt.Main.ExpectedStructureId) ||
		proof.Source.DocumentId != domain.TextID(attempt.Submitted.DocumentId) || proof.Unit != domain.TextID(attempt.Submitted.UnitId) || !domain.SameStableUnitRange(proof.Source, proof.Target, proof.Unit) {
		return false
	}
	delete(cut, "range_projection")
	raw, err := json.Marshal(cut)
	if err != nil {
		return false
	}
	left, err := codec.Decode(raw)
	if err != nil {
		return false
	}
	right, err := codec.Decode(original)
	if err != nil {
		return false
	}
	a, err := codec.Canonical(left)
	if err != nil {
		return false
	}
	b, err := codec.Canonical(right)
	// 仅允许已证明的范围注解；房间、epoch和durable_seq及任何其它原字段必须完全相同。
	return err == nil && bytes.Equal(a, b)
}

// merge_range_version.go：保留原实现，按业务合并文件。

func requireStableReviewRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) error {
	_, _, err := stableMergeRange(ctx, tx, document, unit, source, target)
	return err
}

func stableMergeRange(ctx context.Context, tx *sql.Tx, document, unit, source, target uint64) (any, any, error) {
	old, err := repository.StructureRow(ctx, tx, domain.TextID(source), domain.TextID(document))
	if err != nil {
		return nil, nil, err
	}
	current, err := repository.StructureRow(ctx, tx, domain.TextID(target), domain.TextID(document))
	if err != nil {
		return nil, nil, err
	}
	if !domain.SameStableUnitRange(old, current, domain.TextID(unit)) {
		return nil, nil, domain.Violation(domain.StateConflict, "MERGE_BASE_RANGE_CHANGED")
	}
	return old, current, nil
}

// 基线和送审正文分别绑定各自的实际版本；结构投影不修改正文/来源，也不读取S2当前草稿。
func projectMergeVersion(ctx context.Context, tx *sql.Tx, content *pb.FixedContent, version, target uint64, submittedCut []byte) (*pb.FixedContent, error) {
	if content == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_UNKNOWN")
	}
	if content.StructureId == target {
		return content, nil
	}
	old, current, err := stableMergeRange(ctx, tx, content.DocumentId, content.UnitId, content.StructureId, target)
	if err != nil {
		return nil, err
	}
	stored, err := repository.VersionContent(ctx, tx, version, content.DocumentId)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(stored, version); err != nil {
		return nil, err
	}
	if submittedCut != nil {
		// 送审的BRANCH绑定来自原审核上下文，切点重新取Doc不可变原SUBMISSION事实。
		stored.BranchId, stored.WorkspaceEpoch, stored.LineageRef = content.BranchId, content.WorkspaceEpoch, content.LineageRef
		stored.CutJsonUtf8 = submittedCut
	}
	if !proto.Equal(stored, content) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_BINDING_UNKNOWN")
	}
	cut, err := codec.Decode(content.CutJsonUtf8)
	if err != nil {
		return nil, err
	}
	value, ok := cut.(map[string]any)
	if !ok {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_VERSION_CUT_UNKNOWN")
	}
	raw, err := json.Marshal(map[string]any{"source_structure": old, "target_structure": current, "unit_id": domain.TextID(content.UnitId)})
	if err != nil {
		return nil, err
	}
	evidence, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	value["range_projection"] = evidence
	projected := proto.Clone(content).(*pb.FixedContent)
	projected.StructureId = target
	projected.CutJsonUtf8, err = codec.Canonical(value)
	return projected, err
}

// merge_readiness.go：保留原实现，按业务合并文件。

func (s *Merges) Ready(ctx context.Context) error {
	facts, err := repository.PendingMergeBindings(ctx, s.db)
	if err != nil {
		return errors.New("MERGE_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingMergeBinding(fact); err != nil {
			return err
		}
	}
	return nil
}

func validatePendingMergeBinding(fact repository.PendingMergeBinding) error {
	f := fact.Fact
	var original mergeContext
	if f.Actor == 0 || f.Document == 0 || f.Unit == 0 || f.Branch == 0 || f.Request == 0 || f.Round == 0 || f.Version == 0 ||
		fact.ReceiptActor != f.Actor || len(fact.Fingerprint) != 32 || json.Unmarshal(fact.Original, &original) != nil ||
		!validMergeRecoveryContext(original, fact) {
		return errors.New("MERGE_RECOVERY_BINDING_UNKNOWN")
	}
	switch fact.Kind {
	case "MERGE_RECOVER":
		var fixed mergeContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || !validMergeRecoveryContext(fixed, fact) ||
			fact.ReceiptState != "PREPARING" || (f.State != "PREPARING" && f.State != "COMPUTING" && f.State != "PREPARED") {
			return errors.New("MERGE_RECOVERY_BINDING_UNKNOWN")
		}
	case "MAIN_RECONCILE":
		var fixed mainInstallContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || !proto.Equal(fixed.Context, original.Context) || fixed.Generation == 0 ||
			(fact.ReceiptState != "SUCCEEDED" && fact.ReceiptState != "REJECTED" && fact.ReceiptState != "CANCELLED") {
			return errors.New("MAIN_RECONCILE_BINDING_UNKNOWN")
		}
		if fact.ReceiptState == "SUCCEEDED" && !validMainInstallBinding(fixed, fact) {
			return errors.New("MAIN_RECONCILE_BINDING_UNKNOWN")
		}
	case "MERGE_NOTIFICATION":
		var notice map[string]string
		if f.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Record == 0 ||
			json.Unmarshal(fact.Payload, &notice) != nil || notice["event"] != "APPROVE_AND_MERGE" ||
			notice["document"] != domain.TextID(f.Document) || notice["request"] != domain.TextID(f.Request) ||
			notice["round"] != domain.TextID(f.Round) || notice["record"] != domain.TextID(fact.Record) || notice["actor"] != domain.TextID(f.Actor) {
			return errors.New("MERGE_NOTIFICATION_FACT_UNKNOWN")
		}
	default:
		return errors.New("MERGE_RECOVERY_KIND_UNKNOWN")
	}
	return nil
}

// 这里校验固定身份而非重新授予权限。原短期证明过期不冻结服务，工作者会在SQL锁外续接。
func validMergeRecoveryContext(fixed mergeContext, fact repository.PendingMergeBinding) bool {
	f, binding := fact.Fact, fixed.Context
	if binding == nil || binding.Target == nil || binding.OperationId != fact.Operation || binding.ActorId != f.Actor ||
		fixed.Actor != f.Actor || fixed.Intent.Operation != fact.Operation || fixed.Intent.Round != f.Round || fixed.Intent.Version != f.Version ||
		fixed.Pointer.ID != f.Request || fixed.Pointer.Document != f.Document || fixed.Pointer.Unit != f.Unit || fixed.Pointer.Branch != f.Branch ||
		fixed.Pointer.Round != f.Round || fixed.SessionExpires <= 0 || fixed.Access.Document.Id != domain.TextID(f.Document) ||
		!bytes.Equal(binding.RequestFingerprint, fact.Fingerprint) || binding.IdentityAdmissionRef == "" || binding.DocumentAdmissionRef == "" {
		return false
	}
	target := binding.Target
	if target.DocumentId != f.Document || target.UnitId != f.Unit || target.BranchId != f.Branch ||
		target.StructureId == 0 || target.WorkspaceEpoch == 0 || target.Purpose != "MERGE" || len(binding.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: binding.Sources}, &pb.OperationContext{Sources: fixed.Access.Sources}) || len(fixed.Access.Proofs) == 0 {
		return false
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.OperationId != fact.Operation || proof.ActorId != f.Actor || proof.Purpose != "MERGE" || !proto.Equal(proof.Target, target) {
			return false
		}
	}
	return fixed.Access.Proofs[0].AdmissionId == binding.IdentityAdmissionRef
}

func validMainInstallBinding(fixed mainInstallContext, fact repository.PendingMergeBinding) bool {
	if fact.Fact.State != "SUCCEEDED" || fact.Record == 0 || fixed.Manifest == 0 || fixed.Manifest != fact.Manifest ||
		fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 || len(fixed.Units) == 0 || len(fixed.Digest) != 32 {
		return false
	}
	seen := map[uint64]bool{}
	for _, unit := range fixed.Units {
		if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || seen[unit.UnitId] || unit.Content == nil ||
			unit.Content.UnitId != unit.UnitId || unit.Content.DocumentId != fact.Fact.Document || unit.Content.StructureId != fixed.Structure ||
			unit.Content.BranchId != 0 || unit.Content.WorkspaceEpoch != fixed.Control {
			return false
		}
		seen[unit.UnitId] = true
	}
	digest, err := domain.MainManifestDigest(fixed.Manifest, fixed.Units)
	return err == nil && bytes.Equal(digest[:], fixed.Digest)
}

// merge_readings.go：保留原实现，按业务合并文件。

func (s *Merges) Lookup(ctx context.Context, operation string) (domain.MergeFact, error) {
	return repository.MergeRow(ctx, s.db, operation)
}

func (s *Merges) Progress(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error) {
	if actor.ID != fact.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "MERGE_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fact.Document, "MERGE_RESULT")
	if err != nil {
		return nil, nil, err
	}
	pointer := domain.ReviewPointer{ID: fact.Request, Document: fact.Document, Unit: fact.Unit}
	if !reviewContextAllowed(access, pointer) {
		return nil, nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readMergeResult(ctx, tx, actor, access, pointer, fact.Operation)
	}
	return access.Sources, run, nil
}

func (s *Merges) mergePresentation(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readMergeResult(ctx, tx, actor, access, pointer, operation)
		return value, access.Sources, err
	}
}

func (s *Merges) readMergeResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, pointer domain.ReviewPointer, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	if !reviewContextAllowed(access, pointer) {
		return nil, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	fact, err := repository.MergeRow(ctx, tx, operation)
	if err != nil || fact.Actor != actor.ID || fact.Request != pointer.ID {
		return nil, domain.Violation(domain.EntityNotFound, "MERGE_NOT_FOUND")
	}
	result, err := repository.MergeResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !result.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if err = json.Unmarshal(result.Result, &value); err != nil {
		return nil, err
	}
	current, err := repository.ReviewPointerRow(ctx, tx, pointer.ID)
	if err != nil {
		return nil, err
	}
	value["current_state"] = map[string]string{"review_state": current.Status, "current_round_id": domain.TextID(current.Round), "revision": domain.TextID(current.Revision)}
	if result.State == "SUCCEEDED" {
		ready, err := repository.MainInstallationCompleted(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		sync := "SYNCING"
		if ready {
			sync = "READY"
		}
		// 契约把安装状态放在result；这里只更新HTTP投影，原receipt/RPC终态摘要和固定版本编号不变。
		value["result"].(map[string]any)["main_sync_state"] = sync
	}
	return value, nil
}

// 系统安装只核对原号/原目标/原摘要对应的权威事实；用户读取仍走上面的当前授权与输出租约。
func (s *Merges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	payload, err := repository.MergePayload(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed mergeContext
	if json.Unmarshal(payload, &fixed) != nil || fixed.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "MERGE_BINDING_UNKNOWN")
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.MergeResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
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
	if err = json.Unmarshal(fact.Result, &value); err != nil {
		return nil, err
	}
	for _, key := range []string{"merge_record_id", "manifest_id", "content_version_id"} {
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

// merge_submission_cut.go：保留原实现，按业务合并文件。

func readSubmissionCut(ctx context.Context, q repository.Queryer, round uint64) (json.RawMessage, error) {
	raw, err := repository.MergeArtifact(ctx, q, "REVIEW_ROUND_CONTEXT", domain.TextID(round))
	if err != nil {
		return nil, err
	}
	var original struct {
		Cut json.RawMessage `json:"cut"`
	}
	if json.Unmarshal(raw, &original) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	if len(original.Cut) != 0 && !bytes.Equal(original.Cut, []byte("null")) {
		return original.Cut, nil
	}
	raw, err = repository.MergeArtifact(ctx, q, "REVIEW_CAPTURE_CUT", domain.TextID(round))
	if err != nil {
		return nil, err
	}
	var fact domain.CommittedSubmissionCut
	if json.Unmarshal(raw, &fact) != nil || len(fact.Cut) == 0 {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	return fact.Cut, nil
}

func (s *Merges) ensureSubmissionCut(ctx context.Context, task domain.SubmissionTask, fixed mergeContext) error {
	if _, err := readSubmissionCut(ctx, s.db, fixed.Intent.Round); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}
	round, err := repository.MergeRoundInputs(ctx, s.db, fixed.Pointer.ID, fixed.Intent.Round)
	if err != nil {
		return err
	}
	raw, err := repository.SubmissionPayload(ctx, s.db, round.Submission)
	if err != nil {
		return err
	}
	var original submissionContext
	if json.Unmarshal(raw, &original) != nil || original.Capture == nil || original.Capture.Context == nil || original.Capture.Context.Target == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	binding := original.Capture.Context
	// RPC在SQL锁外，只按已成功送审的原target/fingerprint读固定系统事实；不续接旧作者的权限。
	fact, err := s.client.CommittedCut(ctx, &pb.OperationQuery{OperationId: round.Submission, Target: binding.Target, RequestFingerprint: binding.RequestFingerprint})
	if err != nil {
		return err
	}
	content, err := repository.VersionContent(ctx, s.db, round.SubmittedVersion, fixed.Pointer.Document)
	if err != nil {
		return err
	}
	var cut struct{ Room, Epoch, Seq string }
	var shape map[string]string
	if json.Unmarshal(fact.Cut, &shape) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	cut.Room, cut.Epoch, cut.Seq = shape["room_id"], shape["workspace_epoch"], shape["durable_seq"]
	if fact.Operation != round.Submission || fact.Version != round.SubmittedVersion || !bytes.Equal(fact.BodyHash, content.BodyHash) ||
		binding.Target.DocumentId != fixed.Pointer.Document || binding.Target.UnitId != fixed.Pointer.Unit || binding.Target.BranchId != fixed.Pointer.Branch ||
		cut.Epoch != domain.TextID(binding.Target.WorkspaceEpoch) || cut.Room == "" || cut.Seq == "" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	payload, err := json.Marshal(fact)
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
	if _, err = readSubmissionCut(ctx, tx, fixed.Intent.Round); err == nil {
		return tx.Commit()
	} else if err != sql.ErrNoRows {
		return err
	}
	// 追加单独的固定证明，不改旧REVIEW_ROUND_CONTEXT，更不重采当前支线尾部。
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_CAPTURE_CUT", domain.TextID(fixed.Intent.Round), round.Submission, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// merges.go：保留原实现，按业务合并文件。

type MergeClient interface {
	CommittedCut(context.Context, *pb.OperationQuery) (domain.CommittedSubmissionCut, error)
	CaptureMain(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)
	Compute(context.Context, *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error)
	PrepareMain(context.Context, *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error)
	ReconcileMain(context.Context, *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error)
}

type Merges struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

type mergeContext struct {
	Actor          uint64               `json:"actor"`
	SessionExpires int64                `json:"session_expires"`
	Pointer        domain.ReviewPointer `json:"pointer"`
	Intent         domain.MergeIntent   `json:"intent"`
	Access         DocumentAccess       `json:"access"`
	Context        *pb.OperationContext `json:"context"`
}

func NewMerges(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Merges {
	return &Merges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
````

UTF-8/LF全文SHA-256：`a8f2fbc83c1182086f04f6117d4119a476d60159ba84eaee060e6c0e59eb2f75`。[参考实现](../../../../services/document/internal/logic/document/merges_preparation_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
