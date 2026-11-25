# services/document/internal/logic/document/pull_main_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/221-pull-main-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`commit`、`compute`、`decodePullFixedContent`、`contextWire`、`validateInputs`、`validateBeforeCompute`、`CommittedSource`、`validatePullCommittedBinding`、`reconcile`、`PullClient`、`PullMain`、`pullContext`、`NewPullMain`、`Prepare`、`validateGuard`、`Ready`、`validatePendingPullBinding`、`validateOriginalAdmission`、`equalPullSources`、`Lookup`、`presentation`、`Progress`、`readResult`、`OperationResult`、`RecoverOne`、`reject`、`resume`、`validate`、`DraftSource`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// pull_commit.go：保留原实现，按业务合并文件。

func (s *PullMain) commit(ctx context.Context, task domain.SubmissionTask, fixed pullContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = s.validate(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	current, err := repository.DocumentRow(ctx, tx, fixed.Document, true)
	if err != nil {
		return err
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	if err = lockFixedContentResources(ctx, tx, fixed.Document, space, []*pb.FixedContent{prepared.Main, prepared.Candidate}, "PULL_RESOURCE_NOT_READY"); err != nil {
		return err
	}
	_, _, prior, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return err
	}
	original, err := codec.Decode(prior)
	if err != nil {
		return err
	}
	value, err := codec.Decode(prepared.Candidate.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance := value.(map[string]any)
	accepted, err := codec.Canonical(provenance["accepted_captures"])
	if err != nil {
		return err
	}
	expected, err := codec.Canonical(original.(map[string]any)["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, expected) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_ACCEPTED_CAPTURES_CHANGED")
	}
	// 可靠MAIN采样成为这条支线的固定基线；不切换全篇正式manifest，也不把未拉入的MAIN新增解释成删除。
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	version, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
	if err != nil {
		return err
	}
	lineage, err := NewEntityID()
	if err != nil {
		return err
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(fixed.Manifest), domain.TextID(version)
	provenance["source_operation_id"] = task.Operation
	if err = s.validator.Provenance(provenance); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_PROVENANCE_INVALID")
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertPullLineage(ctx, tx, lineage, fixed.Workspace.Workspace, fixed.Document, fixed.Unit, fixed.Workspace.Lineage, fixed.Manifest, version, task.Operation, prepared.Preview, raw); err != nil {
		return err
	}
	epoch := fixed.Workspace.Epoch + 1
	room, err := collaborationRoom(ctx, tx, "BRANCH", fixed.Workspace.Workspace, fixed.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch = epoch
	content.LineageRef = domain.TextID(lineage)
	content.ProvenanceJsonUtf8 = raw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := draftInstall{Context: fixed.Context, Branch: fixed.Workspace.Workspace, OldRoom: fixed.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(fixed.Manifest), "base_version_id": domain.TextID(version), "body": body, "provenance": provenance}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
		return err
	}
	if err = repository.AdvancePulledBranch(ctx, tx, fixed.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompletePull(ctx, tx, task, result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_compute.go：保留原实现，按业务合并文件。

func (s *PullMain) compute(ctx context.Context, fixed pullContext) (*pb.DraftPreviewReply, draftPrepared, error) {
	preview, err := s.client.Preview(ctx, &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Workspace.Workspace, ExpectedStructureId: fixed.Structure, ExpectedLineageId: fixed.Workspace.Lineage})
	var prepared draftPrepared
	if err != nil {
		return nil, prepared, err
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.MainManifestId != fixed.Manifest || len(preview.InputDigest) != 32 || preview.PreviewCutRef != "draft-preview:"+fixed.Input.OperationId {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREVIEW_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return nil, prepared, err
	}
	raw, err := s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": preview.PreviewCutRef, "expected_input_digest": preview.InputDigest})
	if err != nil {
		return nil, prepared, err
	}
	var parts map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARE_UNKNOWN")
	}
	if err = json.Unmarshal(parts["state"], &prepared.State); err != nil {
		return nil, prepared, err
	}
	if prepared.State == "CONFLICTED" {
		return nil, prepared, domain.Violation(domain.StateConflict, "PULL_MAIN_CONFLICT")
	}
	if prepared.State == "CHANGED" {
		_ = json.Unmarshal(parts["code"], &prepared.Code)
		return nil, prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State != "PREPARED" {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARE_UNKNOWN")
	}
	if prepared.Candidate, err = decodePullFixedContent(parts["candidate"]); err != nil {
		return nil, prepared, err
	}
	if prepared.Main, err = decodePullFixedContent(parts["main"]); err != nil {
		return nil, prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != preview.PreviewCutRef || !bytes.Equal(prepared.Digest, preview.InputDigest) {
		return nil, prepared, domain.Violation(domain.TemporarilyUnavailable, "DRAFT_PREPARED_BINDING_UNKNOWN")
	}
	if err = s.validateInputs(fixed, preview, prepared); err != nil {
		return nil, prepared, err
	}
	return preview, prepared, nil
}

func decodePullFixedContent(raw json.RawMessage) (*pb.FixedContent, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if envelope, exists := fields["purpose"]; exists {
		var purpose string
		if json.Unmarshal(envelope, &purpose) != nil || purpose != "PULL_MAIN" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_CONTENT_ENVELOPE_UNKNOWN")
		}
		// 只兼容首轮已经固定的这一项生产者封装，不使用DiscardUnknown，也不改写Node原候选与摘要。
		delete(fields, "purpose")
		var err error
		raw, err = json.Marshal(fields)
		if err != nil {
			return nil, err
		}
	}
	content := &pb.FixedContent{}
	if err := protojson.Unmarshal(raw, content); err != nil {
		return nil, err
	}
	return content, nil
}
func contextWire(context *pb.OperationContext) (map[string]any, error) {
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(context)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	err = json.Unmarshal(raw, &value)
	return value, err
}
func (s *PullMain) validateInputs(fixed pullContext, preview *pb.DraftPreviewReply, prepared draftPrepared) error {
	if prepared.Cut["room_id"] != domain.TextID(fixed.Room) || prepared.Cut["workspace_epoch"] != domain.TextID(fixed.Workspace.Epoch) {
		return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CUT_UNKNOWN")
	}
	for _, content := range []*pb.FixedContent{preview.Draft, prepared.Candidate, prepared.Main} {
		if err := validateFixedMergeContent(s.validator, content, fixed.Document, fixed.Unit, fixed.Structure); err != nil {
			return err
		}
		if content.OperationId != fixed.Input.OperationId {
			return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CONTENT_BINDING_UNKNOWN")
		}
	}
	for _, content := range []*pb.FixedContent{preview.Draft, prepared.Candidate} {
		if content.BranchId != fixed.Workspace.Workspace || content.WorkspaceEpoch != fixed.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Workspace.Lineage) {
			return domain.Violation(domain.TemporarilyUnavailable, "DRAFT_CONTENT_BINDING_UNKNOWN")
		}
	}
	return nil
}

func (s *PullMain) validateBeforeCompute(ctx context.Context, fixed pullContext, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.validate(ctx, tx, fixed, proofs)
}

// pull_install.go：保留原实现，按业务合并文件。

func (s *PullMain) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "PULL_MAIN_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install draftInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install, query) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"] = content, install.LineageContext
		value["new_room_id"], value["new_epoch"] = domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

func validatePullCommittedBinding(install draftInstall, query *pb.OperationQuery) error {
	if install.Context == nil || query == nil || install.Context.Target == nil ||
		install.Context.OperationId != query.OperationId || !proto.Equal(install.Context.Target, query.Target) ||
		!bytes.Equal(install.Context.RequestFingerprint, query.RequestFingerprint) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	return nil
}
func (s *PullMain) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	var install draftInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	preview, digest := install.Preview, install.Digest
	if authority.State != pb.OperationResult_SUCCEEDED {
		// 拒绝收尾只使用原号系统终态；原来源关闭或会话失效也必须能解除自己的旧屏障。
		preview = "draft-preview:" + task.Operation
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: preview, ExpectedInputDigest: digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.State != authority.State || reply.OperationId != task.Operation || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_INSTALL_ACK_UNKNOWN")
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
	operation, err := repository.LockedBranchControl(ctx, tx, install.Branch)
	if err != nil {
		return err
	}
	if operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.CompletePullInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_main.go：保留原实现，按业务合并文件。

type PullClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	Prepare(context.Context, any) ([]byte, error)
	Apply(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)
}
type PullMain struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    PullClient
	validator *codec.Validator
	instance  string
}
type pullContext struct {
	Actor                               uint64               `json:"actor"`
	SessionExpires                      int64                `json:"session_expires"`
	Document                            uint64               `json:"document"`
	Unit                                uint64               `json:"unit"`
	Room                                uint64               `json:"room"`
	Input                               dto.PullMain         `json:"input"`
	Workspace                           domain.EditWorkspace `json:"workspace"`
	Access                              DocumentAccess       `json:"access"`
	Context                             *pb.OperationContext `json:"context"`
	Manifest, Structure, Style, Control uint64
	MainEpoch                           uint64 `json:"main_epoch,omitempty"`
}

func NewPullMain(db *sql.DB, identity *Identity, access *Access, client PullClient, validator *codec.Validator, instance string) *PullMain {
	return &PullMain{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

// pull_prepare.go：保留原实现，按业务合并文件。

func (s *PullMain) Prepare(ctx context.Context, actor Actor, document, unit uint64, input dto.PullMain, path string) (Actor, MutationBody, MutationPresentation, error) {
	branch, err := domain.CanonicalID(input.ExpectedBranchId)
	if err != nil {
		return actor, nil, nil, err
	}
	lineage, err := domain.CanonicalID(input.ExpectedLineageId)
	if err != nil {
		return actor, nil, nil, err
	}
	structure, err := domain.CanonicalID(input.ExpectedStructureId)
	if err != nil || !domain.ValidOperationID(input.OperationId) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "PULL_MAIN_INPUT_INVALID")
	}
	access, err := s.access.Authorize(ctx, actor, document, "PULL_MAIN")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	present := s.presentation(actor, access, input.OperationId)
	if _, err = repository.PullResult(ctx, s.db, input.OperationId); err == nil {
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, branch, document, unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "PULL_MAIN"}
	original := access.Proofs
	access.Proofs = nil
	for _, proof := range original {
		bound, err := s.identity.AcquireBound(ctx, actor, proof.Capability, proof.IdentityTarget, input.OperationId, target)
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
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.CurrentStructureId != input.ExpectedStructureId {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if barrier.Phase != "OPEN" {
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		active, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil || active != branch {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
		}
		actual, err := repository.LockedControlledWorkspace(ctx, tx, branch, document, unit)
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
		}
		if actual.Lineage != lineage {
			return nil, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
		}
		if actual.Epoch != workspace.Epoch {
			return nil, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
		}
		if err = s.validateGuard(ctx, tx, unit); err != nil {
			return nil, err
		}
		if err = repository.RequireBranchControlOpen(ctx, tx, branch); err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "PULL_MAIN", admission); err != nil {
			return nil, err
		}
		rawTarget, _ := json.Marshal(target)
		if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, unit, input.OperationId, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		control, _ := domain.CanonicalRevision(current.ControlRevision)
		mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		fixed := pullContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Document: document, Unit: unit, Room: room, Input: input, Workspace: actual, Access: access, Manifest: manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Context: &pb.OperationContext{OperationId: input.OperationId, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}}
		payload, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterPull(ctx, tx, branch, input.OperationId, payload); err != nil {
			return nil, err
		}
		return mainControlPending(input.OperationId), nil
	}
	return actor, run, present, nil
}

// 固定旧审核轮次可以继续存在；未固定送审/正在合并则不能与改变支线代次互相越过。
func (s *PullMain) validateGuard(ctx context.Context, tx *sql.Tx, unit uint64) error {
	guard, err := repository.LockedReviewGuard(ctx, tx, unit)
	if err != nil {
		return err
	}
	if guard.Submission != "" {
		return domain.Violation(domain.StateConflict, "SUBMISSION_PENDING")
	}
	for _, request := range []uint64{guard.Ongoing, guard.Resumable} {
		if request == 0 {
			continue
		}
		active, err := repository.LockedActiveMerge(ctx, tx, request)
		if err != nil {
			return err
		}
		if active != "" {
			return domain.Violation(domain.StateConflict, "MERGE_PENDING")
		}
	}
	return nil
}

// pull_readiness.go：保留原实现，按业务合并文件。

func (s *PullMain) Ready(ctx context.Context) error {
	facts, err := repository.PendingPullBindings(ctx, s.db)
	if err != nil {
		return errors.New("PULL_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err = validatePendingPullBinding(fact); err != nil {
			return err
		}
		var fixed pullContext
		if json.Unmarshal(fact.Original, &fixed) != nil {
			return errors.New("PULL_BINDING_UNKNOWN")
		}
		sources, readErr := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if readErr != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("PULL_BINDING_UNKNOWN")
		}
	}
	return nil
}
func validatePendingPullBinding(fact repository.PendingPullBinding) error {
	invalid := errors.New("PULL_BINDING_UNKNOWN")
	var fixed pullContext
	var gate struct {
		Operation string `json:"operation_id"`
	}
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &fixed) != nil ||
		json.Unmarshal(fact.Gate, &gate) != nil ||
		json.Unmarshal(fact.Target, &target) != nil ||
		fixed.Context == nil ||
		fixed.Context.Target == nil ||
		fixed.Actor == 0 ||
		fixed.Actor != fact.Actor ||
		fixed.Context.ActorId != fixed.Actor ||
		fixed.SessionExpires <= 0 ||
		fixed.Document == 0 ||
		fixed.Unit == 0 ||
		fixed.Room == 0 ||
		fixed.Workspace.Workspace == 0 ||
		fixed.Workspace.Lineage == 0 ||
		fixed.Workspace.Epoch == 0 ||
		fixed.Workspace.BaseManifest == 0 ||
		fixed.Workspace.BaseVersion == 0 ||
		fixed.Input.OperationId != fact.Operation ||
		fixed.Context.OperationId != fact.Operation ||
		!domain.ValidOperationID(fact.Operation) ||
		gate.Operation != fact.Operation {
		return invalid
	}
	businessTarget := fixed.Context.Target
	if !domain.ValidOperationID(fixed.Context.DocumentAdmissionRef) || !domain.ValidOperationID(fixed.Context.IdentityAdmissionRef) {
		return invalid
	}
	if businessTarget.Purpose != "PULL_MAIN" ||
		businessTarget.DocumentId != fixed.Document ||
		businessTarget.UnitId != fixed.Unit ||
		businessTarget.BranchId != fixed.Workspace.Workspace ||
		businessTarget.StructureId != fixed.Structure ||
		businessTarget.WorkspaceEpoch != fixed.Workspace.Epoch ||
		fixed.Input.ExpectedBranchId != domain.TextID(fixed.Workspace.Workspace) ||
		fixed.Input.ExpectedLineageId != domain.TextID(fixed.Workspace.Lineage) ||
		fixed.Input.ExpectedStructureId != domain.TextID(fixed.Structure) ||
		!fixed.Access.Permissions.Edit[fixed.Unit] ||
		len(fixed.Access.Proofs) == 0 ||
		len(fixed.Access.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: fixed.Access.Sources}, &pb.OperationContext{Sources: fixed.Context.Sources}) {
		return invalid
	}
	for i, proof := range fixed.Access.Proofs {
		if proof == nil ||
			proof.ActorId != fixed.Actor ||
			proof.OperationId != fact.Operation ||
			proof.Purpose != "PULL_MAIN" ||
			len(proof.SessionRef) != 32 ||
			!proto.Equal(proof.Target, businessTarget) ||
			(i == 0 && proof.AdmissionId != fixed.Context.IdentityAdmissionRef) {
			return invalid
		}
		if i > 0 && !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return invalid
		}
	}
	if validateOriginalAdmission(fixed, fact.Admission) != nil {
		return invalid
	}
	path := "/v1/documents/" + domain.TextID(fixed.Document) + "/units/" + domain.TextID(fixed.Unit) + "/pull-main"
	fingerprint, err := domain.OperationFingerprint(path, fixed.Input)
	if err != nil || target.Path != path || !bytes.Equal(fingerprint[:], fact.Fingerprint) || !bytes.Equal(fingerprint[:], fixed.Context.RequestFingerprint) {
		return invalid
	}
	if fact.Kind == "PULL_MAIN_RECOVER" {
		var progress pullContext
		if fact.State != "PREPARING" ||
			fact.Lifecycle != "ACTIVE" ||
			fact.Epoch != fixed.Workspace.Epoch ||
			fact.Lineage != fixed.Workspace.Lineage ||
			json.Unmarshal(fact.Payload, &progress) != nil {
			return invalid
		}
		left, _ := json.Marshal(progress)
		right, _ := json.Marshal(fixed)
		if !bytes.Equal(left, right) {
			return invalid
		}
	} else if fact.Kind == "PULL_MAIN_RECONCILE" {
		var install draftInstall
		if json.Unmarshal(fact.Payload, &install) != nil || !proto.Equal(install.Context, fixed.Context) || install.Branch != fixed.Workspace.Workspace || install.OldRoom != fixed.Room {
			return invalid
		}
		if fact.State == "SUCCEEDED" {
			var result struct {
				Result map[string]string `json:"result"`
			}
			if json.Unmarshal(fact.Result, &result) != nil ||
				install.NewRoom == 0 ||
				install.Lineage == 0 ||
				install.Epoch != fixed.Workspace.Epoch+1 ||
				install.Content == nil ||
				install.Content.DocumentId != fixed.Document ||
				install.Content.UnitId != fixed.Unit ||
				install.Content.BranchId != fixed.Workspace.Workspace ||
				install.Content.WorkspaceEpoch != install.Epoch ||
				install.Content.LineageRef != domain.TextID(install.Lineage) ||
				len(install.Digest) != 32 ||
				install.Preview != "draft-preview:"+fact.Operation ||
				fact.Lifecycle != "RESTORING" ||
				fact.Epoch != install.Epoch ||
				fact.Lineage != install.Lineage ||
				result.Result["entity_id"] != domain.TextID(install.Lineage) {
				return invalid
			}
		} else if fact.State == "REJECTED" {
			if install.NewRoom != 0 || install.Content != nil || fact.Lifecycle != "ACTIVE" || fact.Epoch != fixed.Workspace.Epoch || fact.Lineage != fixed.Workspace.Lineage {
				return invalid
			}
		} else {
			return invalid
		}
	} else {
		return invalid
	}
	return nil
}

func validateOriginalAdmission(fixed pullContext, actual repository.OriginalAdmissionBinding) error {
	invalid := errors.New("PULL_BINDING_UNKNOWN")
	if fixed.Context == nil || fixed.Context.Target == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return invalid
	}
	var target pb.Target
	if actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != fixed.Unit ||
		actual.Operation != fixed.Input.OperationId || actual.Purpose != "PULL_MAIN" ||
		actual.Identity != fixed.Context.IdentityAdmissionRef ||
		actual.SessionExpires != fixed.SessionExpires || len(actual.Session) != 32 ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) ||
		json.Unmarshal(actual.Target, &target) != nil || !proto.Equal(&target, fixed.Context.Target) {
		return invalid
	}
	// 会话/来源到期属于业务拒绝或系统收尾；就绪检查只判断原记录有没有被换绑。
	return nil
}

func equalPullSources(left, right []*pb.SourceEpoch) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	ordered := func(values []*pb.SourceEpoch) ([]*pb.SourceEpoch, bool) {
		result := append([]*pb.SourceEpoch(nil), values...)
		for _, value := range result {
			if value == nil || value.ScopeKey == "" || value.Epoch == 0 {
				return nil, false
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].ScopeKey < result[j].ScopeKey })
		for i := 1; i < len(result); i++ {
			if result[i-1].ScopeKey == result[i].ScopeKey {
				return nil, false
			}
		}
		return result, true
	}
	a, ok := ordered(left)
	if !ok {
		return false
	}
	b, ok := ordered(right)
	return ok && proto.Equal(&pb.OperationContext{Sources: a}, &pb.OperationContext{Sources: b})
}

// pull_readings.go：保留原实现，按业务合并文件。

func (s *PullMain) Lookup(ctx context.Context, operation string) (pullContext, error) {
	var fixed pullContext
	raw, err := repository.MergeArtifact(ctx, s.db, "PULL_MAIN_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Input.OperationId != operation || fixed.Context.OperationId != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
	return fixed, nil
}
func (s *PullMain) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}
func (s *PullMain) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PULL_MAIN_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "PULL_MAIN_RESULT")
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}
func (s *PullMain) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	fact, err := repository.PullResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if fact.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "PULL_MAIN_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(fact.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	if fact.State == "SUCCEEDED" {
		ready, err := repository.PullInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		result, ok := value["result"].(map[string]any)
		if !ok {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
		}
		// 冻结Operation没有草稿同步字段。HTTP投影WAITING直至系统完成；RPC仍交付不可变SUCCEEDED事实。
		// 首轮已固定回执多带了支线字段，只投影契约已有ID，不改原回执字节或系统摘要。
		ids := map[string]any{"entity_id": result["entity_id"]}
		if version, exists := result["content_version_id"]; exists {
			ids["content_version_id"] = version
		}
		value["result"] = ids
		if !ready {
			value["state"] = "WAITING"
			value["poll_url"] = "/v1/operations/" + operation
		}
	}
	return value, nil
}
func (s *PullMain) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.PullResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PULL_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// pull_recovery.go：保留原实现，按业务合并文件。

func (s *PullMain) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimPullTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	if task.Kind == "PULL_MAIN_RECONCILE" {
		err = s.reconcile(ctx, task)
	} else {
		var fixed pullContext
		if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
			err = domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
		} else {
			proofs, resumeError := s.resume(ctx, fixed)
			err = resumeError
			if err == nil {
				err = s.validateBeforeCompute(ctx, fixed, proofs)
			}
			if err == nil {
				_, prepared, computeError := s.compute(ctx, fixed)
				err = computeError
				if err == nil {
					err = s.commit(ctx, task, fixed, prepared, proofs)
				}
			}
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				err = s.reject(ctx, task, fixed, violation.Code)
			}
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, mainControlRecoveryCode(err))
	}
}

func (s *PullMain) reject(ctx context.Context, task domain.SubmissionTask, fixed pullContext, code string) error {
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
	operation, err := repository.LockedBranchControl(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return err
	}
	if operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	// 拒绝也要原号系统确认后解锁自己的Node屏障；不能把RPC超时当作已恢复OPEN。
	install, _ := json.Marshal(draftInstall{Context: fixed.Context, Branch: fixed.Workspace.Workspace, OldRoom: fixed.Room, Epoch: fixed.Workspace.Epoch})
	if err = repository.InsertSubmissionTask(ctx, tx, "PULL_MAIN_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

// pull_source.go：保留原实现，按业务合并文件。

func (s *PullMain) resume(ctx context.Context, fixed pullContext) ([]*pb.IdentityAdmission, error) {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	var result []*pb.IdentityAdmission
	for _, proof := range fixed.Access.Proofs {
		current, err := s.identity.Resume(ctx, proof)
		if err != nil {
			return nil, err
		}
		result = append(result, current)
	}
	return result, nil
}
func (s *PullMain) validate(ctx context.Context, tx *sql.Tx, fixed pullContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Edit[fixed.Unit] {
		return domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	if current.CurrentManifestId != domain.TextID(fixed.Manifest) || current.CurrentStructureId != domain.TextID(fixed.Structure) || current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, fixed.Workspace.Workspace, fixed.Document, fixed.Unit)
	if err != nil {
		return domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	if workspace != fixed.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	if err = s.validateGuard(ctx, tx, fixed.Unit); err != nil {
		return err
	}
	operation, err := repository.LockedBranchControl(ctx, tx, workspace.Workspace)
	if err != nil {
		return err
	}
	if operation != fixed.Input.OperationId {
		return domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil || validateOriginalAdmission(fixed, admission) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "PULL_BINDING_UNKNOWN")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

func (s *PullMain) DraftSource(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(fixed.Context, input.Context) || input.ExpectedBranchId != fixed.Workspace.Workspace || input.ExpectedLineageId != fixed.Workspace.Lineage || input.ExpectedStructureId != fixed.Structure {
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
	if err = s.validate(ctx, tx, fixed, proofs); err != nil {
		return nil, err
	}
	return readDraftSource(ctx, tx, draftSourceScope{Document: fixed.Document, Unit: fixed.Unit, Room: fixed.Room, Manifest: fixed.Manifest, Structure: fixed.Structure, Style: fixed.Style, Control: fixed.Control, MainEpoch: fixed.MainEpoch, Workspace: fixed.Workspace})
}
````

UTF-8/LF全文SHA-256：`71503863d0248d9e04bf7a9159f807e669a084ea0d314419807bd5de33af484b`。[参考实现](../../../../services/document/internal/logic/document/pull_main_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
