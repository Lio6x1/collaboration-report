// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
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
	"reflect"
	"sort"
	"time"
)

// reverse_changes.go：保留原实现，按业务合并文件。

type ReverseChanges struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

type reverseContext struct {
	Actor           uint64
	SessionExpires  int64
	Intent          domain.ReverseChangeIntent
	Access          DocumentAccess
	Context         *pb.OperationContext
	Scope           draftSourceScope
	Record          repository.ReverseMergeRecord
	Before, After   *pb.FixedContent
	GuardGeneration uint64
}

type reverseInstall struct {
	draftInstall
	Fixed                   reverseContext `json:"fixed"`
	Request, Round, Version uint64
}

func NewReverseChanges(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *ReverseChanges {
	return &ReverseChanges{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

func (s *ReverseChanges) lookup(ctx context.Context, operation string) (reverseContext, error) {
	var fixed reverseContext
	raw, err := repository.MergeArtifact(ctx, s.db, "REVERSE_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.OperationId != operation || fixed.Intent.Operation != operation || fixed.Context.Target.Purpose != "REVERSE" {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	return fixed, nil
}

func reversePreviewRequest(fixed reverseContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedStructureId: fixed.Scope.Structure, ExpectedLineageId: fixed.Scope.Workspace.Lineage}
}

// reverse_commit.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) commit(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
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
			return domain.Violation(domain.StateConflict, "REVERSE_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	baseline := proto.Clone(prepared.Main).(*pb.FixedContent)
	baseVersion, err := repository.InsertMergeVersion(ctx, tx, baseline, "MAIN_CAPTURE", baseline.CaptureId)
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
	provenance, ok := decoded.(map[string]any)
	if !ok {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PROVENANCE_UNKNOWN")
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"], provenance["source_operation_id"] = domain.TextID(lineage), domain.TextID(scope.Manifest), domain.TextID(baseVersion), task.Operation
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	raw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertReverseLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, scope.Manifest, baseVersion, task.Operation, prepared.Preview, raw); err != nil {
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
	version, err := repository.InsertMergeVersion(ctx, tx, content, "REVERSE", content.CaptureId)
	if err != nil {
		return err
	}
	request, err := repository.CreateReviewRequest(ctx, tx, scope.Document, scope.Unit, scope.Workspace.Workspace)
	if err != nil {
		return err
	}
	workspace := scope.Workspace
	workspace.Epoch, workspace.Lineage, workspace.BaseManifest, workspace.BaseVersion = epoch, lineage, scope.Manifest, baseVersion
	round, err := repository.CreateSubmissionRound(ctx, tx, request, fixed.Actor, version, content, workspace)
	if err != nil {
		return err
	}
	roundContext, err := json.Marshal(map[string]any{"style_id": domain.TextID(scope.Style), "document_title": fixed.Access.Document.Title, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_ROUND_CONTEXT", domain.TextID(round), task.Operation, roundContext, "SUCCEEDED"); err != nil {
		return err
	}
	// 服务器固定来源、真实submission_operation和review_round同TX；后续普通批准读取这份新代次零尾cut。
	reviewSource, err := json.Marshal(map[string]any{"operation_id": task.Operation, "merge_record_id": domain.TextID(fixed.Record.ID), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version), "capture_ref": content.CaptureId, "input_digest": prepared.Digest, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_REVIEW_SOURCE", task.Operation, task.Operation, reviewSource, "SUCCEEDED"); err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)}}
	if err = repository.CompleteSubmission(ctx, tx, task.Operation, fixed.Actor, scope.Unit, request, round, result); err != nil {
		return err
	}
	body, err := codec.Decode(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	install := reverseInstall{Fixed: fixed, Request: request, Round: round, Version: version, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content,
		LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(scope.Manifest), "base_version_id": domain.TextID(baseVersion), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.InsertReverseAudit(ctx, tx, task.Operation, fixed.Actor, fixed.Record.ID, request, round, version, fixed.Intent.Reason); err != nil {
		return err
	}
	notice, err := json.Marshal(map[string]string{"document_id": domain.TextID(scope.Document), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "kind": "SUBMISSION"})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
	if err = repository.CompleteReverseTask(ctx, tx, task, "SUCCEEDED", encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_compute.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) fixedPreview(raw []byte, fixed reverseContext) (draftPrepared, error) {
	var expected draftPrepared
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_UNKNOWN")
	decoded, err := codec.Decode(raw)
	if err != nil {
		return expected, err
	}
	value, ok := decoded.(map[string]any)
	if !ok {
		return expected, unknown
	}
	digestText, ok := value["input_digest"].(string)
	if !ok {
		return expected, unknown
	}
	digest, err := base64.StdEncoding.DecodeString(digestText)
	if err != nil || len(digest) != 32 {
		return expected, unknown
	}
	delete(value, "input_digest")
	canonical, err := codec.Canonical(value)
	if err != nil {
		return expected, err
	}
	hash := sha256.Sum256(canonical)
	if !bytes.Equal(hash[:], digest) {
		return expected, unknown
	}
	bindingRaw, err := json.Marshal(value["binding"])
	if err != nil {
		return expected, err
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(bindingRaw, binding) != nil || !proto.Equal(binding, reversePreviewRequest(fixed)) {
		return expected, unknown
	}
	source, ok := value["source"].(map[string]any)
	if !ok || (source["workspace_epoch"] != domain.TextID(draftMainEpoch(fixed.Scope)) && !(fixed.Scope.MainEpoch == 0 && source["workspace_epoch"] == nil)) {
		return expected, unknown
	}
	if !ok || source["purpose"] != "REVERSE" || source["merge_record_id"] != domain.TextID(fixed.Record.ID) || source["reason"] != fixed.Intent.Reason || source["main_manifest_id"] != domain.TextID(fixed.Scope.Manifest) || source["structure_id"] != domain.TextID(fixed.Scope.Structure) || source["control_revision"] != domain.TextID(fixed.Scope.Control) {
		return expected, unknown
	}
	for key, wanted := range map[string]*pb.FixedContent{"reverse_before": fixed.Before, "reverse_after": fixed.After} {
		part, err := json.Marshal(source[key])
		if err != nil {
			return expected, err
		}
		content, err := decodePullFixedContent(part)
		if err != nil || !proto.Equal(content, wanted) {
			return expected, unknown
		}
	}
	if value["preview_cut_ref"] != fixed.Intent.Operation {
		return expected, unknown
	}
	expected.Preview, expected.Digest = fixed.Intent.Operation, digest
	cutRaw, err := json.Marshal(value["draft_cut"])
	if err != nil || json.Unmarshal(cutRaw, &expected.Cut) != nil || expected.Cut["room_id"] != domain.TextID(fixed.Scope.Room) || expected.Cut["workspace_epoch"] != domain.TextID(fixed.Scope.Workspace.Epoch) {
		return expected, unknown
	}
	if value["candidate"] == nil {
		return expected, domain.Violation(domain.StateConflict, "REVERSE_CONFLICT")
	}
	for key := range map[string]bool{"candidate": true, "main": true, "draft": true} {
		part, err := json.Marshal(value[key])
		if err != nil {
			return expected, err
		}
		content, err := decodePullFixedContent(part)
		if err != nil {
			return expected, err
		}
		if err = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); err != nil {
			return expected, err
		}
		if content.OperationId != fixed.Intent.Operation {
			return expected, unknown
		}
		if key == "main" {
			if content.BranchId != 0 || content.WorkspaceEpoch != fixed.Scope.Control {
				return expected, unknown
			}
			expected.Main = content
		} else {
			if content.BranchId != fixed.Scope.Workspace.Workspace || content.WorkspaceEpoch != fixed.Scope.Workspace.Epoch || content.LineageRef != domain.TextID(fixed.Scope.Workspace.Lineage) {
				return expected, unknown
			}
			if key == "candidate" {
				if content.CaptureId != fixed.Intent.Operation {
					return expected, unknown
				}
				expected.Candidate = content
			}
		}
	}
	if bytes.Equal(expected.Candidate.BodyHash, expected.Main.BodyHash) {
		return expected, domain.Violation(domain.StateConflict, "REVERSE_NO_CHANGES")
	}
	return expected, nil
}

func (s *ReverseChanges) compute(ctx context.Context, task domain.SubmissionTask, fixed reverseContext) (draftPrepared, error) {
	request := reversePreviewRequest(fixed)
	preview, err := s.client.Preview(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.MainManifestId != fixed.Scope.Manifest || preview.PreviewCutRef != fixed.Intent.Operation {
		return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_UNKNOWN")
	}
	raw, err := s.client.FixedDraft(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	// Go保留Node完整原固定输入；即使冲突也不抹掉两份正文和真实切点。
	if err = s.persistFixed(ctx, task, raw); err != nil {
		return draftPrepared{}, err
	}
	expected, err := s.fixedPreview(raw, fixed)
	if err != nil {
		return expected, err
	}
	var originalParts map[string]json.RawMessage
	if json.Unmarshal(raw, &originalParts) != nil {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_BINDING_UNKNOWN")
	}
	draft, err := decodePullFixedContent(originalParts["draft"])
	if err != nil {
		return expected, err
	}
	if !bytes.Equal(preview.InputDigest, expected.Digest) || !proto.Equal(preview.Draft, draft) {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREVIEW_BINDING_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err = s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": expected.Preview, "expected_input_digest": expected.Digest})
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	var prepared draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &prepared.State) != nil {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
	}
	if prepared.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &prepared.Code) != nil || prepared.Code == "" {
			return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
		}
		return prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State != "PREPARED" {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARE_UNKNOWN")
	}
	prepared.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return prepared, err
	}
	prepared.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != expected.Preview || !bytes.Equal(prepared.Digest, expected.Digest) || !proto.Equal(prepared.Main, expected.Main) || !proto.Equal(prepared.Candidate, expected.Candidate) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_PREPARED_BINDING_UNKNOWN")
	}
	actual, _ := codec.Canonical(prepared.Cut)
	wanted, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(actual, wanted) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CUT_CHANGED")
	}
	return prepared, nil
}

func (s *ReverseChanges) persistFixed(ctx context.Context, task domain.SubmissionTask, raw []byte) error {
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
	if prior, e := repository.MergeArtifact(ctx, tx, "REVERSE_FIXED", task.Operation); e == nil {
		// MySQL JSON只改变对象键的排列；比较完整规范事实，不改已有payload。
		left, e1 := codec.Decode(prior)
		right, e2 := codec.Decode(raw)
		if e1 != nil || e2 != nil {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_UNKNOWN")
		}
		a, _ := codec.Canonical(left)
		b, _ := codec.Canonical(right)
		if !bytes.Equal(a, b) {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_FIXED_CHANGED")
		}
	} else if e == sql.ErrNoRows {
		if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	} else {
		return e
	}
	return tx.Commit()
}

// reverse_install.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "REVERSE_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	var install reverseInstall
	if json.Unmarshal(raw, &install) != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		if err = s.validateReviewSource(ctx, s.db, install, false); err != nil {
			return nil, err
		}
		content, err := fixedWire(install.Content)
		if err != nil {
			return nil, err
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

func (s *ReverseChanges) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	var install reverseInstall
	if json.Unmarshal(task.Payload, &install) != nil || install.Context == nil || install.Context.Target == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (install.Content == nil || current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_INSTALL_ACK_UNKNOWN")
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
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if authority.State == pb.OperationResult_SUCCEEDED {
		pointer, err := repository.LockedReviewPointer(ctx, tx, install.Request)
		if err != nil {
			return err
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, query.Target.UnitId)
		if err != nil {
			return err
		}
		round, err := repository.MergeRoundInputs(ctx, tx, install.Request, install.Round)
		if err != nil {
			return err
		}
		if pointer.Document != query.Target.DocumentId || pointer.Unit != query.Target.UnitId || pointer.Branch != install.Branch || pointer.Round != install.Round || pointer.Status != "REVIEWING" || pointer.Archived || guard.Ongoing != install.Request || guard.Submission != "" || round.Submission != task.Operation || round.SubmittedVersion != install.Version || round.Lineage != install.Lineage || round.Capture != task.Operation {
			return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
		}
		// 成功后的恢复属于系统责任，注销不重写旧成功，也不重新借用作者权限。
		if err = repository.ActivateDiscardedBranch(ctx, tx, install.Branch, install.Epoch); err != nil {
			return err
		}
	}
	if err = repository.SetBranchControl(ctx, tx, install.Branch, ""); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVERSE_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_original.go：保留原实现，按业务合并文件。

// 合并前正文来自原可靠MAIN采样，而非input_manifest里的更旧正式版本；否则会反向删除当时兼容的MAIN尾部。
func (s *ReverseChanges) original(ctx context.Context, tx *sql.Tx, document, record, structure uint64) (repository.ReverseMergeRecord, *pb.FixedContent, *pb.FixedContent, error) {
	value, err := repository.ReverseMerge(ctx, tx, document, record)
	if err == sql.ErrNoRows {
		return value, nil, nil, domain.Violation(domain.StateConflict, "REVERSE_MERGE_NOT_FOUND")
	}
	if err != nil {
		return value, nil, nil, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_SOURCE_UNKNOWN")
	if value.State != "SUCCEEDED" || len(value.Digest) != 32 {
		return value, nil, nil, unknown
	}
	key := attemptKey(value.Operation, value.Attempt)
	raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key)
	if err == sql.ErrNoRows {
		raw, err = repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key)
	}
	if err != nil {
		return value, nil, nil, err
	}
	var fixed mergeFixedInputs
	if json.Unmarshal(raw, &fixed) != nil || fixed.Input == nil || fixed.Input.Context == nil || fixed.Input.Context.Target == nil || fixed.Input.Main == nil || fixed.Attempt.Main == nil {
		return value, nil, nil, unknown
	}
	target := fixed.Input.Context.Target
	digest, err := domain.MergeInputDigest(fixed.Input)
	if err != nil || !bytes.Equal(digest[:], value.Digest) || !bytes.Equal(fixed.Input.InputDigest, value.Digest) || fixed.Attempt.ID != value.Attempt || fixed.Input.AttemptId != domain.TextID(value.Attempt) || fixed.Input.Context.OperationId != value.Operation || target.DocumentId != document || target.UnitId != value.Unit || target.BranchId != value.Branch || fixed.Attempt.Main.ExpectedManifestId != value.InputManifest || fixed.Attempt.Round.Capture != value.Capture {
		return value, nil, nil, unknown
	}
	before := proto.Clone(fixed.Input.Main).(*pb.FixedContent)
	if err = validateFixedMergeContent(s.validator, before, document, value.Unit, value.Structure); err != nil {
		return value, nil, nil, err
	}
	after, err := repository.VersionContent(ctx, tx, value.Version, document)
	if err != nil {
		return value, nil, nil, err
	}
	if err = canonicalVersion(after, value.Version); err != nil {
		return value, nil, nil, err
	}
	if after.UnitId != value.Unit || after.StructureId != value.Structure || after.OperationId != value.Operation {
		return value, nil, nil, unknown
	}
	if value.Structure != structure {
		if err = requireStableReviewRange(ctx, tx, document, value.Unit, value.Structure, structure); err != nil {
			return value, nil, nil, err
		}
	}
	return value, before, after, nil
}

// reverse_readiness.go：保留原实现，按业务合并文件。

func validatePendingReverseBinding(fact repository.PendingReverseBinding) (reverseContext, error) {
	var fixed reverseContext
	invalid := errors.New("REVERSE_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &fixed) != nil {
		return fixed, invalid
	}
	ctx, scope := fixed.Context, fixed.Scope
	if ctx == nil || ctx.Target == nil || fixed.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || fixed.Intent.Operation != fact.Operation || fact.Owner != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) || fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Manage {
		return fixed, invalid
	}
	expected := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "REVERSE"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || scope.Workspace.Lineage == 0 || !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || fixed.GuardGeneration == 0 {
		return fixed, invalid
	}
	parsed, err := domain.ParseReverseChange(fixed.Intent.Input)
	if err != nil || !reflect.DeepEqual(parsed, fixed.Intent) || parsed.Manifest != scope.Manifest || parsed.Merge != fixed.Record.ID {
		return fixed, invalid
	}
	path := "/v1/documents/" + domain.TextID(scope.Document) + "/reverse-changes"
	var target struct {
		Path string `json:"path"`
	}
	fingerprint, err := domain.OperationFingerprint(path, fixed.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) || json.Unmarshal(fact.Target, &target) != nil || target.Path != path {
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
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fact.Operation || proof.Purpose != "REVERSE" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return fixed, invalid
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return fixed, invalid
			}
		}
	}
	if fixed.Access.Proofs[0].AdmissionId != ctx.IdentityAdmissionRef {
		return fixed, invalid
	}
	record := fixed.Record
	if record.ID == 0 || record.Document != scope.Document || record.Unit != scope.Unit || record.Request == 0 || record.Round == 0 || record.Attempt == 0 || record.InputManifest == 0 || record.OutputManifest == 0 || record.Version == 0 || record.Structure == 0 || record.State != "SUCCEEDED" || !domain.ValidOperationID(record.Operation) || len(record.Digest) != 32 || fixed.Before == nil || fixed.After == nil {
		return fixed, invalid
	}
	for _, content := range []*pb.FixedContent{fixed.Before, fixed.After} {
		if content.DocumentId != scope.Document || content.UnitId != scope.Unit || content.StructureId != record.Structure || content.OperationId != record.Operation || len(content.BodyHash) != 32 {
			return fixed, invalid
		}
	}
	if fact.SourceActor != fixed.Actor || fact.SourceDocument != scope.Document || fact.SourceUnit != scope.Unit || fact.SourceBranch != scope.Workspace.Workspace || fact.SourceLineage != scope.Workspace.Lineage || fact.SourceGeneration != fixed.GuardGeneration {
		return fixed, invalid
	}
	if fact.Kind == "REVERSE_RECOVER" {
		if fact.State != "PREPARING" || fact.SourceState != "PREPARING" || fact.HeldOperation != fact.Operation || fact.HeldGeneration != fixed.GuardGeneration || !bytes.Equal(fact.Payload, fact.Original) {
			return fixed, invalid
		}
	} else if fact.Kind == "REVERSE_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || fact.SourceState != fact.State || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, invalid
		}
		var install reverseInstall
		if json.Unmarshal(fact.Payload, &install) != nil || !proto.Equal(install.Context, ctx) || install.Branch != scope.Workspace.Workspace || install.OldRoom != scope.Room || install.Preview != fact.Operation {
			return fixed, invalid
		}
		a, _ := json.Marshal(install.Fixed)
		b, _ := json.Marshal(fixed)
		if !bytes.Equal(a, b) {
			return fixed, invalid
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == scope.Room || install.Epoch != scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Request == 0 || install.Round == 0 || install.Version == 0 || install.Round != fact.Round || install.Content == nil || install.Content.OperationId != fact.Operation || install.Content.CaptureId != fact.Operation) {
			return fixed, invalid
		}
	} else {
		return fixed, invalid
	}
	return fixed, nil
}

func validateReverseAdmission(fixed reverseContext, actual repository.OriginalAdmissionBinding) error {
	invalid := errors.New("REVERSE_ADMISSION_UNKNOWN")
	if fixed.Context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "REVERSE" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return invalid
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return invalid
	}
	return nil
}

func (s *ReverseChanges) Ready(ctx context.Context) error {
	facts, err := repository.PendingReverseBindings(ctx, s.db)
	if err != nil {
		return errors.New("REVERSE_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingReverseBinding(fact)
		if err != nil {
			return err
		}
		actual, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateReverseAdmission(fixed, actual) != nil {
			return errors.New("REVERSE_ADMISSION_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("REVERSE_ADMISSION_UNKNOWN")
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		record, before, after, err := s.original(ctx, tx, fixed.Scope.Document, fixed.Intent.Merge, fixed.Scope.Structure)
		if err == nil {
			err = s.validatePendingReviewSource(ctx, tx, fact)
		}
		_ = tx.Rollback()
		if err != nil || !reflect.DeepEqual(record, fixed.Record) || !proto.Equal(before, fixed.Before) || !proto.Equal(after, fixed.After) {
			return errors.New("REVERSE_SOURCE_UNKNOWN")
		}
	}
	return nil
}

// reverse_readings.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) OperationDocument(ctx context.Context, operation string) (uint64, error) {
	fixed, err := s.lookup(ctx, operation)
	return fixed.Scope.Document, err
}

func (s *ReverseChanges) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if actor.ID != fixed.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "REVERSE_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Scope.Document, "REVERSE_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Manage {
		return nil, nil, domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fact, err := repository.ReverseResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.Actor != actor.ID || !fact.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "REVERSE_NOT_FOUND")
		}
		var value map[string]any
		if json.Unmarshal(fact.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
		}
		installed, err := repository.ReverseInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if fact.State != "PREPARING" && !installed {
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

func (s *ReverseChanges) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "REVERSE_QUERY_INVALID")
	}
	fixed, err := s.lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.ReverseResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REVERSE_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// reverse_recovery.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimReverseTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindingError := repository.ClaimedReverseBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingReverseBinding(fact)
	if bindingError != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	if err != nil {
		_ = repository.RetryMergeTask(ctx, s.db, task, "REVERSE_BINDING_UNKNOWN")
		return
	}
	if task.Kind == "REVERSE_RECONCILE" {
		err = s.validatePendingReviewSource(ctx, s.db, fact)
		if err == nil {
			err = s.reconcile(ctx, task)
		}
	} else {
		if err == nil {
			proofs, e := s.resume(ctx, fixed)
			err = e
			if err == nil {
				check, e := s.db.BeginTx(ctx, nil)
				err = e
				if err == nil {
					err = s.validate(ctx, check, fixed, proofs)
					_ = check.Rollback()
				}
			}
			if err == nil {
				prepared, e := s.compute(ctx, task, fixed)
				err = e
				if err == nil {
					err = s.commit(ctx, task, fixed, prepared, proofs)
				}
			}
			var violation domain.RuleViolation
			if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.StateConflict || violation.Kind == domain.EntityNotFound) {
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

func (s *ReverseChanges) reject(ctx context.Context, task domain.SubmissionTask, fixed reverseContext, code string) error {
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
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.RejectReverseSource(ctx, tx, task.Operation, fixed.Scope.Unit, code); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	install, err := json.Marshal(reverseInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: task.Operation}})
	if err != nil {
		return err
	}
	if err = repository.CompleteReverseTask(ctx, tx, task, "REJECTED", install); err != nil {
		return err
	}
	return tx.Commit()
}

// reverse_register.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) Register(ctx context.Context, actor Actor, document uint64, intent domain.ReverseChangeIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "REVERSE")
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
	if prior, e := repository.ReverseResult(ctx, s.db, intent.Operation); e == nil {
		if prior.Actor != actor.ID || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	if access.Document.CurrentManifestId != domain.TextID(intent.Manifest) {
		return actor, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	record, err := repository.ReverseMerge(ctx, s.db, document, intent.Merge)
	if err == sql.ErrNoRows {
		return actor, domain.Violation(domain.StateConflict, "REVERSE_MERGE_NOT_FOUND")
	}
	if err != nil {
		return actor, err
	}
	branch, err := repository.ReverseActiveBranch(ctx, s.db, record.Unit, document)
	if err == sql.ErrNoRows {
		// 原单元已退役或删除，不能猜测其现存后代作为可写范围；真实数据库故障仍保留重试语义。
		return actor, domain.Violation(domain.StateConflict, "REVERSE_RANGE_CHANGED")
	}
	if err != nil {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, branch, document, record.Unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, err
	}
	target := &pb.Target{DocumentId: document, UnitId: record.Unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "REVERSE"}
	var proofs []*pb.IdentityAdmission
	proofs, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	access.Proofs = proofs
	actor.Proof, actor.Sources = proofs[0], access.Sources
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
		if prior.ActorID != actor.ID || prior.Kind != "REVERSE" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
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
	if current.CurrentManifestId != domain.TextID(intent.Manifest) || current.CurrentStructureId != domain.TextID(structure) {
		return actor, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, record.Unit, document)
	if err != nil || active != branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	actual, err := repository.LockedControlledWorkspace(ctx, tx, branch, document, record.Unit)
	if err != nil || actual != workspace {
		return actor, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	owner, err := repository.LockedBranchControl(ctx, tx, branch)
	if err != nil {
		return actor, err
	}
	if owner != "" {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, record.Unit)
	if err != nil {
		return actor, err
	}
	// 反向是新的审核，不会覆盖另一个进行中或尚可续提的请求；必须先明确结束那项工作。
	if guard.Ongoing != 0 || guard.Resumable != 0 || guard.Submission != "" {
		return actor, domain.Violation(domain.StateConflict, "REVERSE_REVIEW_PENDING")
	}
	record, before, after, err := s.original(ctx, tx, document, intent.Merge, structure)
	if err != nil {
		return actor, err
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: actor.SessionExpiresUnixMs, Access: access}, proofs); err != nil {
		return actor, err
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", branch, record.Unit, workspace.Epoch)
	if err != nil {
		return actor, err
	}
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalRevision(current.ControlRevision)
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	access.Document = current
	fixed := reverseContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Record: record, Before: before, After: after, GuardGeneration: guard.Generation + 1,
		Scope: draftSourceScope{Document: document, Unit: record.Unit, Room: room, Manifest: intent.Manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: workspace}}
	command := OperationCommand{ID: intent.Operation, Kind: "REVERSE", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "REVERSE", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureSubmissionAdmission(ctx, tx, admission, record.Unit, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	payload, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterReverse(ctx, tx, record, actor.ID, workspace, fixed.GuardGeneration, intent.Operation, payload); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// reverse_review_source.go：保留原实现，按业务合并文件。

// 恢复元数据与真实不可变轮次交叉核验，不能只因两个任务里的版本号一致就开放Ready或安装。
// 调用方可传同一事务保持一致读；成功后的系统收尾不重新借用作者登录资格。
func (s *ReverseChanges) validateReviewSource(ctx context.Context, q repository.Queryer, install reverseInstall, pending bool) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
	if install.Context == nil || install.Context.Target == nil || install.Content == nil {
		return unknown
	}
	target := install.Context.Target
	pointer, err := repository.ReviewPointerRow(ctx, q, install.Request)
	if err != nil {
		return err
	}
	round, err := repository.MergeRoundInputs(ctx, q, install.Request, install.Round)
	if err != nil {
		return err
	}
	if pointer.Document != target.DocumentId || pointer.Unit != target.UnitId || pointer.Branch != install.Branch || round.Submission != install.Context.OperationId || round.SubmittedVersion != install.Version || round.Lineage != install.Lineage || round.Capture != install.Context.OperationId || round.Structure != target.StructureId {
		return unknown
	}
	// 未完成安装时当前轮次仍须独占；已完成后的私有原号查询允许请求后来已合并或续提。
	if pending && (pointer.Round != install.Round || pointer.Status != "REVIEWING" || pointer.Archived) {
		return unknown
	}
	stored, err := repository.VersionContent(ctx, q, install.Version, target.DocumentId)
	if err != nil {
		return err
	}
	if err = canonicalVersion(stored, install.Version); err != nil {
		return err
	}
	if err = validateFixedMergeContent(s.validator, stored, target.DocumentId, target.UnitId, target.StructureId); err != nil {
		return err
	}
	// 正文版本用版本引用切点；安装候选用新房间起始切点。只规范这三个传输字段，正文/谱系/资源仍精确比较。
	expected := proto.Clone(install.Content).(*pb.FixedContent)
	expected.BranchId, expected.WorkspaceEpoch = 0, 0
	expected.CutJsonUtf8, err = codec.Canonical(map[string]any{"content_version_id": domain.TextID(install.Version), "source_ref": install.Context.OperationId})
	if err != nil || !proto.Equal(stored, expected) {
		return unknown
	}
	return nil
}

func (s *ReverseChanges) validatePendingReviewSource(ctx context.Context, q repository.Queryer, fact repository.PendingReverseBinding) error {
	if fact.Kind != "REVERSE_RECONCILE" || fact.State != "SUCCEEDED" {
		return nil
	}
	var install reverseInstall
	if json.Unmarshal(fact.Payload, &install) != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_REVIEW_SOURCE_UNKNOWN")
	}
	return s.validateReviewSource(ctx, q, install, true)
}

// reverse_source.go：保留原实现，按业务合并文件。

func (s *ReverseChanges) resume(ctx context.Context, fixed reverseContext) ([]*pb.IdentityAdmission, error) {
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

func (s *ReverseChanges) validate(ctx context.Context, tx *sql.Tx, fixed reverseContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage {
		return domain.Violation(domain.PermissionDenied, "DRAFT_MANAGE_REQUIRED")
	}
	scope := fixed.Scope
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
	owner, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return err
	}
	if owner != fixed.Intent.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_CONTROL_UNKNOWN")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, scope.Unit)
	if err != nil {
		return err
	}
	if guard.Ongoing != 0 || guard.Resumable != 0 || guard.Submission != fixed.Intent.Operation || guard.Generation != fixed.GuardGeneration {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_GUARD_UNKNOWN")
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	target := &pb.Target{}
	if admission.Actor != fixed.Actor || admission.Document != scope.Document || admission.Unit != scope.Unit || admission.Operation != fixed.Intent.Operation || admission.Purpose != "REVERSE" || admission.Identity != fixed.Context.IdentityAdmissionRef || admission.SessionExpires != fixed.SessionExpires || len(fixed.Access.Proofs) == 0 || !bytes.Equal(admission.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(admission.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "REVERSE_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_BINDING_UNKNOWN")
	}
	record, before, after, err := s.original(ctx, tx, scope.Document, fixed.Intent.Merge, scope.Structure)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(record, fixed.Record) || !proto.Equal(before, fixed.Before) || !proto.Equal(after, fixed.After) {
		return domain.Violation(domain.TemporarilyUnavailable, "REVERSE_SOURCE_CHANGED")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

func (s *ReverseChanges) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "DRAFT_SOURCE_INVALID")
	}
	fixed, err := s.lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, reversePreviewRequest(fixed)) {
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
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err != nil {
		return nil, err
	}
	before, err := fixedWire(fixed.Before)
	if err != nil {
		return nil, err
	}
	after, err := fixedWire(fixed.After)
	if err != nil {
		return nil, err
	}
	value["purpose"], value["reverse_before"], value["reverse_after"], value["merge_record_id"], value["reason"] = "REVERSE", before, after, domain.TextID(fixed.Record.ID), fixed.Intent.Reason
	return value, nil
}
