# services/document/internal/logic/document/conflict_resolutions_preparation_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/198-conflict-resolutions-preparation-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`fixedPreview`、`persistFixed`、`compute`、`resolutionPreviewError`、`validateResolutionContext`、`validatePendingResolutionBinding`、`Ready`、`Progress`、`OperationResult`、`validateResolutionAdmission`、`resume`、`validate`、`resolutionSourceFields`、`Source`、`Resolutions`、`resolutionContext`、`resolutionInstall`、`NewResolutions`、`decodeResolutionContext`、`Lookup`、`resolutionPreviewRequest`、`sameResolutionContext`、`decodeResolutionInstall`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"reflect"
	"time"
)

// conflict_resolution_compute.go：保留原实现，按业务合并文件。

func (s *Resolutions) fixedPreview(ctx context.Context, tx *sql.Tx, raw []byte, fixed resolutionContext) (draftPrepared, error) {
	var expected draftPrepared
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
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
	bindingRaw, _ := json.Marshal(value["binding"])
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(bindingRaw, binding) != nil || !proto.Equal(binding, resolutionPreviewRequest(fixed)) || value["preview_cut_ref"] != fixed.Intent.Operation {
		return expected, unknown
	}
	source, ok := value["source"].(map[string]any)
	if !ok || (source["workspace_epoch"] != domain.TextID(draftMainEpoch(fixed.Scope)) && !(fixed.Scope.MainEpoch == 0 && source["workspace_epoch"] == nil)) {
		return expected, unknown
	}
	if !ok || source["main_manifest_id"] != domain.TextID(fixed.Scope.Manifest) || source["structure_id"] != domain.TextID(fixed.Scope.Structure) || source["style_id"] != domain.TextID(fixed.Scope.Style) || source["control_revision"] != domain.TextID(fixed.Scope.Control) || source["room_id"] != domain.TextID(fixed.Scope.Room) {
		return expected, unknown
	}
	original, versions, err := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
	if err != nil {
		return expected, err
	}
	if versions.Main != fixed.Versions.Main || versions.Submitted != fixed.Versions.Submitted || versions.Base != fixed.Versions.Base {
		return expected, unknown
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return expected, err
	}
	wanted := map[string]any{}
	if err = resolutionSourceFields(wanted, fixed, original, baseline); err != nil {
		return expected, err
	}
	for key, expectedValue := range wanted {
		left, e1 := codec.Canonical(source[key])
		right, e2 := codec.Canonical(expectedValue)
		if e1 != nil || e2 != nil || !bytes.Equal(left, right) {
			return expected, unknown
		}
	}
	expected.Preview, expected.Digest = fixed.Intent.Operation, digest
	cutRaw, _ := json.Marshal(value["draft_cut"])
	if json.Unmarshal(cutRaw, &expected.Cut) != nil || expected.Cut["room_id"] != domain.TextID(fixed.Scope.Room) || expected.Cut["workspace_epoch"] != domain.TextID(fixed.Scope.Workspace.Epoch) {
		return expected, unknown
	}
	if _, err = domain.CanonicalRevision(expected.Cut["durable_seq"]); err != nil {
		return expected, unknown
	}
	if value["candidate"] == nil {
		return expected, domain.Violation(domain.StateConflict, "RESOLUTION_CONFLICT")
	}
	for _, key := range []string{"candidate", "main", "draft"} {
		part, _ := json.Marshal(value[key])
		content, e := decodePullFixedContent(part)
		if e != nil {
			return expected, e
		}
		if e = validateFixedMergeContent(s.validator, content, fixed.Scope.Document, fixed.Scope.Unit, fixed.Scope.Structure); e != nil {
			return expected, e
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
			if _, e = resolutionBody(s.validator, content.BodyJsonUtf8); e != nil {
				return expected, unknown
			}
			if key == "candidate" {
				if content.CaptureId != fixed.Intent.Operation+":candidate" {
					return expected, unknown
				}
				expected.Candidate = content
			}
		}
	}
	return expected, nil
}

func (s *Resolutions) persistFixed(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, raw []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	if prior, e := repository.MergeArtifact(ctx, tx, "RESOLUTION_FIXED", task.Operation); e == nil {
		left, e1 := codec.Decode(prior)
		right, e2 := codec.Decode(raw)
		if e1 != nil || e2 != nil {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
		}
		a, e1 := codec.Canonical(left)
		b, e2 := codec.Canonical(right)
		if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_CHANGED")
		}
	} else if e == sql.ErrNoRows {
		if err = repository.InsertSubmissionTask(ctx, tx, "RESOLUTION_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
			return err
		}
	} else {
		return e
	}
	return tx.Commit()
}

func (s *Resolutions) compute(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext) (draftPrepared, error) {
	request := resolutionPreviewRequest(fixed)
	preview, err := s.client.Preview(ctx, request)
	if err != nil {
		return draftPrepared{}, resolutionPreviewError(err)
	}
	if preview == nil || preview.State != pb.CaptureReply_FIXED || preview.PreviewCutRef != fixed.Intent.Operation || preview.MainManifestId != fixed.Scope.Manifest {
		return draftPrepared{}, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREVIEW_UNKNOWN")
	}
	raw, err := s.client.FixedDraft(ctx, request)
	if err != nil {
		return draftPrepared{}, err
	}
	if err = s.persistFixed(ctx, task, fixed, raw); err != nil {
		return draftPrepared{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return draftPrepared{}, err
	}
	expected, err := s.fixedPreview(ctx, tx, raw, fixed)
	_ = tx.Rollback()
	if err != nil {
		return expected, err
	}
	var parts map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	}
	draft, err := decodePullFixedContent(parts["draft"])
	if err != nil || !proto.Equal(draft, preview.Draft) || !bytes.Equal(preview.InputDigest, expected.Digest) {
		return expected, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREVIEW_UNKNOWN")
	}
	wire, err := contextWire(fixed.Context)
	if err != nil {
		return expected, err
	}
	raw, err = s.client.Prepare(ctx, map[string]any{"context": wire, "expected_attempt_id": expected.Preview, "expected_input_digest": expected.Digest, "expected_round_id": domain.TextID(fixed.Intent.Round)})
	if err != nil {
		return expected, err
	}
	var prepared draftPrepared
	if json.Unmarshal(raw, &parts) != nil || json.Unmarshal(parts["state"], &prepared.State) != nil {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
	}
	if prepared.State == "CHANGED" {
		if json.Unmarshal(parts["code"], &prepared.Code) != nil || prepared.Code == "" {
			return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
		}
		return prepared, domain.Violation(domain.StateConflict, prepared.Code)
	}
	if prepared.State == "CONFLICTED" {
		return prepared, domain.Violation(domain.StateConflict, "RESOLUTION_CONFLICT")
	}
	if prepared.State != "PREPARED" {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARE_UNKNOWN")
	}
	prepared.Candidate, err = decodePullFixedContent(parts["candidate"])
	if err != nil {
		return prepared, err
	}
	prepared.Main, err = decodePullFixedContent(parts["main"])
	if err != nil {
		return prepared, err
	}
	if json.Unmarshal(parts["preview_cut_ref"], &prepared.Preview) != nil || json.Unmarshal(parts["input_digest"], &prepared.Digest) != nil || json.Unmarshal(parts["draft_cut"], &prepared.Cut) != nil || prepared.Preview != expected.Preview || !bytes.Equal(prepared.Digest, expected.Digest) || !proto.Equal(prepared.Candidate, expected.Candidate) || !proto.Equal(prepared.Main, expected.Main) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARED_BINDING_UNKNOWN")
	}
	a, _ := codec.Canonical(prepared.Cut)
	b, _ := codec.Canonical(expected.Cut)
	if !bytes.Equal(a, b) {
		return prepared, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CUT_CHANGED")
	}
	return prepared, nil
}

// conflict_resolution_errors.go：保留原实现，按业务合并文件。

// 只接纳已约定的确定拒绝。连接/超时、坏固定来源或未认识的RPC错误继续未知恢复，不能释放控制。
func resolutionPreviewError(err error) error {
	value, ok := status.FromError(err)
	if !ok {
		return err
	}
	if value.Code() == codes.InvalidArgument && value.Message() == "RESOLUTION_CONTENT_INVALID" {
		return domain.Violation(domain.StateConflict, "RESOLUTION_CONTENT_INVALID")
	}
	if value.Code() == codes.ResourceExhausted && value.Message() == "DISCARD_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, "RESOLUTION_TRANSFER_LIMIT")
	}
	return err
}

// conflict_resolution_readiness.go：保留原实现，按业务合并文件。

func validateResolutionContext(fixed resolutionContext) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	c, scope := fixed.Context, fixed.Scope
	if c == nil || c.Target == nil || c.OperationId != fixed.Intent.Operation || c.ActorId != fixed.Actor || fixed.Actor == 0 || fixed.SessionExpires <= 0 || c.IdentityAdmissionRef == "" || c.DocumentAdmissionRef == "" || len(c.RequestFingerprint) != 32 || len(fixed.Access.Proofs) == 0 || !fixed.Access.Permissions.Edit[scope.Unit] {
		return unknown
	}
	target := &pb.Target{DocumentId: scope.Document, UnitId: scope.Unit, BranchId: scope.Workspace.Workspace, StructureId: scope.Structure, WorkspaceEpoch: scope.Workspace.Epoch, Purpose: "CONFLICT_RESOLUTION"}
	if scope.Document == 0 || scope.Unit == 0 || scope.Room == 0 || scope.Manifest == 0 || scope.Structure == 0 || scope.Style == 0 || scope.Control == 0 || scope.Workspace.Lineage == 0 || scope.Workspace.Workspace == 0 || scope.Workspace.Epoch == 0 || fixed.BaselineVersion == 0 || !proto.Equal(target, c.Target) || !equalPullSources(c.Sources, fixed.Access.Sources) {
		return unknown
	}
	if fixed.Origin.Operation != fixed.Intent.Conflict || fixed.Origin.Document != scope.Document || fixed.Origin.Unit != scope.Unit || fixed.Origin.Branch != scope.Workspace.Workspace || fixed.Origin.Round != fixed.Intent.Round || fixed.Versions.Attempt != fixed.Intent.Attempt || fixed.Versions.Base == 0 || fixed.Versions.Submitted == 0 || fixed.Versions.Main == 0 || !bytes.Equal(fixed.Versions.Digest, fixed.Intent.Digest) {
		return unknown
	}
	if fixed.Review.ID != fixed.Origin.Request || fixed.Review.Round != fixed.Intent.Round || fixed.Review.Document != scope.Document || fixed.Review.Unit != scope.Unit || fixed.Review.Branch != scope.Workspace.Workspace || fixed.Review.Status != "WITHDRAWN" || fixed.Review.Archived || fixed.Guard.Ongoing != 0 || fixed.Guard.Resumable != fixed.Review.ID || fixed.Guard.Submission != "" {
		return unknown
	}
	d := fixed.Access.Document
	if d.Id != domain.TextID(scope.Document) || d.CurrentManifestId != domain.TextID(scope.Manifest) || d.CurrentStructureId != domain.TextID(scope.Structure) || d.CurrentStyleId != domain.TextID(scope.Style) || d.ControlRevision != domain.TextID(scope.Control) {
		return unknown
	}
	path := "/v1/conflict-contexts/" + fixed.Intent.Conflict + "/resolutions"
	fingerprint, err := domain.ConflictResolutionFingerprint(path, fixed.Intent)
	if err != nil || !bytes.Equal(fingerprint[:], c.RequestFingerprint) {
		return unknown
	}
	available := map[string]uint64{}
	for _, source := range c.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return unknown
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ActorId != fixed.Actor || proof.OperationId != fixed.Intent.Operation || proof.Purpose != "CONFLICT_RESOLUTION" || !proto.Equal(proof.Target, target) || len(proof.SessionRef) != 32 || len(proof.Sources) == 0 || proof.ExpiresUnixMs <= 0 || !domain.ValidOperationID(proof.AdmissionId) || !bytes.Equal(proof.SessionRef, fixed.Access.Proofs[0].SessionRef) {
			return unknown
		}
		for _, source := range proof.Sources {
			if source == nil || source.Epoch == 0 || available[source.ScopeKey] != source.Epoch {
				return unknown
			}
		}
	}
	if fixed.Access.Proofs[0].AdmissionId != c.IdentityAdmissionRef {
		return unknown
	}
	return nil
}

func validatePendingResolutionBinding(fact repository.PendingResolutionBinding) (resolutionContext, error) {
	fixed, err := decodeResolutionContext(fact.Original)
	if err != nil {
		return fixed, err
	}
	if err = validateResolutionContext(fixed); err != nil {
		return fixed, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	var target struct {
		Path string `json:"path"`
	}
	if fixed.Intent.Operation != fact.Operation || fixed.Actor != fact.Actor || fact.Owner != fact.Operation || !bytes.Equal(fixed.Context.RequestFingerprint, fact.Fingerprint) || json.Unmarshal(fact.Target, &target) != nil || target.Path != "/v1/conflict-contexts/"+fixed.Intent.Conflict+"/resolutions" {
		return fixed, unknown
	}
	if fact.Kind == "RESOLUTION_RECOVER" {
		payload, e := decodeResolutionContext(fact.Payload)
		if e != nil || fact.State != "PREPARING" || !sameResolutionContext(payload, fixed) {
			return fixed, unknown
		}
	} else if fact.Kind == "RESOLUTION_RECONCILE" {
		if (fact.State != "SUCCEEDED" && fact.State != "REJECTED") || !bytes.Equal(fact.Payload, fact.Committed) {
			return fixed, unknown
		}
		install, decodeError := decodeResolutionInstall(fact.Payload)
		if decodeError != nil || !proto.Equal(install.Context, fixed.Context) || install.Branch != fixed.Scope.Workspace.Workspace || install.OldRoom != fixed.Scope.Room || install.Preview != fact.Operation {
			return fixed, unknown
		}
		raw, _ := json.Marshal(install.Fixed)
		original, e := decodeResolutionContext(raw)
		if e != nil || !sameResolutionContext(original, fixed) {
			return fixed, unknown
		}
		if fact.State == "SUCCEEDED" && (install.NewRoom == 0 || install.NewRoom == fixed.Scope.Room || install.Epoch != fixed.Scope.Workspace.Epoch+1 || install.Lineage == 0 || install.Version == 0 || install.Content == nil || install.Content.OperationId != fact.Operation || install.Content.CaptureId != fact.Operation+":candidate") {
			return fixed, unknown
		}
	} else {
		return fixed, unknown
	}
	return fixed, nil
}

func (s *Resolutions) Ready(ctx context.Context) error {
	facts, err := repository.PendingResolutionBindings(ctx, s.db, "")
	if err != nil {
		return err
	}
	for _, fact := range facts {
		fixed, e := validatePendingResolutionBinding(fact)
		if e != nil {
			return e
		}
		actual, e := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if e != nil {
			return e
		}
		if e = validateResolutionAdmission(fixed, actual); e != nil {
			return e
		}
		sources, e := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if e != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
		}
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		_, versions, e := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
		if e == nil && !reflect.DeepEqual(versions, fixed.Versions) {
			e = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSIONS_UNKNOWN")
		}
		if e == nil && fact.Kind == "RESOLUTION_RECONCILE" && fact.State == "SUCCEEDED" {
			install, decodeError := decodeResolutionInstall(fact.Payload)
			if decodeError != nil {
				e = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
			} else {
				e = s.validateInstallSource(ctx, tx, install)
			}
		}
		_ = tx.Rollback()
		if e != nil {
			return e
		}
	}
	return nil
}

// conflict_resolution_readings.go：保留原实现，按业务合并文件。

func (s *Resolutions) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if actor.ID != fixed.Actor {
		return nil, nil, domain.Violation(domain.EntityNotFound, "RESOLUTION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Scope.Document, "CONFLICT_RESOLUTION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Edit[fixed.Scope.Unit] {
		return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		receipt, err := repository.ResolutionResult(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if receipt.Actor != actor.ID || !receipt.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.EntityNotFound, "RESOLUTION_NOT_FOUND")
		}
		var value map[string]any
		if json.Unmarshal(receipt.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
		}
		installed, err := repository.ResolutionInstalled(ctx, tx, operation)
		if err != nil {
			return nil, err
		}
		if receipt.State != "PREPARING" && !installed {
			return map[string]any{"operation_id": operation, "state": "WAITING", "poll_url": "/v1/operations/" + operation}, nil
		}
		return value, nil
	}, nil
}

func (s *Resolutions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "RESOLUTION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.ResolutionResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	if fact.Actor != fixed.Actor || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	result := &pb.OperationResult{OperationId: query.OperationId, State: pb.OperationResult_PROCESSING, Generation: fact.Generation}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
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
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
	}
	result.BusinessCode = value.Error.Code
	return result, nil
}

// conflict_resolution_source.go：保留原实现，按业务合并文件。

func validateResolutionAdmission(fixed resolutionContext, actual repository.OriginalAdmissionBinding) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	if len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || fixed.Context == nil {
		return unknown
	}
	target := &pb.Target{}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Scope.Document || actual.Unit != fixed.Scope.Unit || actual.Identity != fixed.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation || actual.Purpose != "CONFLICT_RESOLUTION" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, fixed.Context.Target) {
		return unknown
	}
	return nil
}

func (s *Resolutions) resume(ctx context.Context, fixed resolutionContext) ([]*pb.IdentityAdmission, error) {
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

func (s *Resolutions) validate(ctx context.Context, tx *sql.Tx, fixed resolutionContext, proofs []*pb.IdentityAdmission) (mergeFixedInputs, error) {
	var empty mergeFixedInputs
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return empty, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return empty, err
	}
	scope := fixed.Scope
	if !fixed.Access.Permissions.Edit[scope.Unit] {
		return empty, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	if current.CurrentManifestId != domain.TextID(scope.Manifest) || current.CurrentStructureId != domain.TextID(scope.Structure) || current.CurrentStyleId != domain.TextID(scope.Style) || current.ControlRevision != domain.TextID(scope.Control) {
		return empty, domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, scope.Document)
	if err != nil {
		return empty, err
	}
	if barrier.Phase != "OPEN" {
		return empty, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, scope.Unit, scope.Document)
	if err != nil || active != scope.Workspace.Workspace {
		return empty, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedControlledWorkspace(ctx, tx, active, scope.Document, scope.Unit)
	if err != nil || workspace != scope.Workspace {
		return empty, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	owner, err := repository.LockedBranchControl(ctx, tx, active)
	if err != nil {
		return empty, err
	}
	if owner != fixed.Intent.Operation {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	pointer, guard, err := resolutionReview(ctx, tx, fixed.Origin, fixed.Intent.Round)
	if err != nil {
		return empty, err
	}
	if pointer != fixed.Review || guard != fixed.Guard {
		return empty, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return empty, err
	}
	if err = validateResolutionAdmission(fixed, actual); err != nil {
		return empty, err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return empty, err
	}
	if state.State == "CLOSED" {
		return empty, domain.Violation(domain.PermissionDenied, "RESOLUTION_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ADMISSION_UNKNOWN")
	}
	if err = validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs); err != nil {
		return empty, err
	}
	original, versions, err := s.original(ctx, tx, fixed.Origin, fixed.Intent, false)
	if err == nil && !reflect.DeepEqual(versions, fixed.Versions) {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSIONS_UNKNOWN")
	}
	return original, err
}

func resolutionSourceFields(value map[string]any, fixed resolutionContext, original mergeFixedInputs, baseline *pb.FixedContent) error {
	submitted, err := fixedWire(original.Input.Submitted)
	if err != nil {
		return err
	}
	base, err := fixedWire(baseline)
	if err != nil {
		return err
	}
	body, err := codec.Decode(fixed.Intent.Body)
	if err != nil {
		return err
	}
	value["purpose"] = "CONFLICT_RESOLUTION"
	value["conflict_ref"], value["original_attempt_id"], value["original_input_digest"], value["expected_round_id"] = fixed.Intent.Conflict, domain.ConflictAttemptRef(fixed.Intent.Attempt), fixed.Intent.Input.ExpectedInputDigest, domain.TextID(fixed.Intent.Round)
	value["resolution_content"], value["resolution_submitted"], value["resolution_baseline"] = body, submitted, base
	value["resolution_base_version_id"], value["resolution_base_manifest_id"] = domain.TextID(fixed.BaselineVersion), domain.TextID(original.Attempt.Main.ExpectedManifestId)
	return nil
}

func (s *Resolutions) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "RESOLUTION_SOURCE_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input, resolutionPreviewRequest(fixed)) {
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
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return nil, err
	}
	original, err := s.validate(ctx, tx, fixed, proofs)
	if err != nil {
		return nil, err
	}
	value, err := readDraftSource(ctx, tx, fixed.Scope)
	if err != nil {
		return nil, err
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return nil, err
	}
	if err = resolutionSourceFields(value, fixed, original, baseline); err != nil {
		return nil, err
	}
	return value, nil
}

// conflict_resolutions.go：保留原实现，按业务合并文件。

type Resolutions struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

type resolutionContext struct {
	Actor           uint64
	SessionExpires  int64
	Intent          domain.ConflictResolutionIntent
	Origin          domain.MergeFact
	Versions        conflictVersions
	BaselineVersion uint64
	Access          DocumentAccess
	Scope           draftSourceScope
	Review          domain.ReviewPointer
	Guard           domain.ReviewGuard
	Context         *pb.OperationContext
}

type resolutionInstall struct {
	draftInstall
	Fixed   resolutionContext
	Version uint64
}

func NewResolutions(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *Resolutions {
	return &Resolutions{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

func decodeResolutionContext(raw []byte) (resolutionContext, error) {
	var fixed resolutionContext
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.Target.Purpose != "CONFLICT_RESOLUTION" || fixed.Intent.Operation != fixed.Context.OperationId || fixed.Origin.Operation != fixed.Intent.Conflict {
		return fixed, unknown
	}
	intent, err := domain.ParseConflictResolution(fixed.Intent.Conflict, fixed.Intent.Input)
	if err != nil || intent.Operation != fixed.Intent.Operation || intent.Attempt != fixed.Intent.Attempt || intent.Round != fixed.Intent.Round || !bytes.Equal(intent.Digest, fixed.Intent.Digest) || !reflect.DeepEqual(intent.Conflicts, fixed.Intent.Conflicts) {
		return fixed, unknown
	}
	// 正文只持久化在原HTTP意图里，避免另存一份可独立漂移的大正文；读取时从同一意图规范派生。
	fixed.Intent = intent
	return fixed, validateResolutionContext(fixed)
}

func (s *Resolutions) Lookup(ctx context.Context, operation string) (resolutionContext, error) {
	raw, err := repository.MergeArtifact(ctx, s.db, "RESOLUTION_CONTEXT", operation)
	if err != nil {
		return resolutionContext{}, err
	}
	fixed, err := decodeResolutionContext(raw)
	if err == nil && fixed.Context.OperationId != operation {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	return fixed, err
}

func resolutionPreviewRequest(fixed resolutionContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
}

func sameResolutionContext(left, right resolutionContext) bool {
	// protobuf比较会初始化消息内部缓存；这些字段不参与已持久化协议。
	// 比较完整协议编码，保留所有业务/来源字段，避免reflect把缓存状态当成篡改。
	a, first := json.Marshal(left)
	b, second := json.Marshal(right)
	return first == nil && second == nil && bytes.Equal(a, b)
}

func decodeResolutionInstall(raw []byte) (resolutionInstall, error) {
	var install resolutionInstall
	// 跨库JSON中的source_range和标题级别是规范整数；默认float64解码无法再进入canonical-v1。
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&install)
	return install, err
}
````

UTF-8/LF全文SHA-256：`935120846f0c615d1bc2383e53af3be8840089b771602ca7a973e6153e423133`。[参考实现](../../../../services/document/internal/logic/document/conflict_resolutions_preparation_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
