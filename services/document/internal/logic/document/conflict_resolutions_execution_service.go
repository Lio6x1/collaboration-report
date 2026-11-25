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
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sort"
	"time"
)

// conflict_resolution_commit.go：保留原实现，按业务合并文件。

func (s *Resolutions) commit(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, prepared draftPrepared, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return err
	}
	original, err := s.validate(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	raw, err := repository.MergeArtifact(ctx, tx, "RESOLUTION_FIXED", task.Operation)
	if err != nil {
		return err
	}
	expected, err := s.fixedPreview(ctx, tx, raw, fixed)
	if err != nil {
		return err
	}
	if !bytes.Equal(expected.Digest, prepared.Digest) || !proto.Equal(expected.Candidate, prepared.Candidate) || !proto.Equal(expected.Main, prepared.Main) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PREPARED_BINDING_UNKNOWN")
	}
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return err
	}
	stored, err := repository.VersionContent(ctx, tx, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	matched, err := conflictVersionMatches(stored, baseline, fixed.BaselineVersion)
	if err != nil || !matched {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BASELINE_UNKNOWN")
	}
	scope := fixed.Scope
	space, _ := domain.CanonicalID(fixed.Access.Document.SpaceId)
	resourceSet := map[uint64]bool{}
	for _, part := range []*pb.FixedContent{baseline, prepared.Candidate} {
		for _, id := range part.ResourceIds {
			resourceSet[id] = true
		}
	}
	ids := make([]uint64, 0, len(resourceSet))
	for id := range resourceSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, e := repository.LockedDocumentResource(ctx, tx, id, scope.Document, space)
		if e != nil {
			return e
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "RESOLUTION_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, scope.Document, task.Operation, 1); err != nil {
			return err
		}
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
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PROVENANCE_UNKNOWN")
	}
	manifest := original.Attempt.Main.ExpectedManifestId
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(lineage), domain.TextID(manifest), domain.TextID(fixed.BaselineVersion)
	if err = s.validator.Provenance(provenance); err != nil {
		return err
	}
	provenanceRaw, err := codec.Canonical(provenance)
	if err != nil {
		return err
	}
	if err = repository.InsertResolvedLineage(ctx, tx, lineage, scope.Workspace.Workspace, scope.Document, scope.Unit, scope.Workspace.Lineage, manifest, fixed.BaselineVersion, task.Operation, prepared.Preview, provenanceRaw); err != nil {
		return err
	}
	epoch := scope.Workspace.Epoch + 1
	if epoch == 0 {
		return domain.Violation(domain.StateConflict, "WORKSPACE_EPOCH_EXHAUSTED")
	}
	room, err := collaborationRoom(ctx, tx, "BRANCH", scope.Workspace.Workspace, scope.Unit, epoch)
	if err != nil {
		return err
	}
	content := proto.Clone(prepared.Candidate).(*pb.FixedContent)
	content.WorkspaceEpoch, content.LineageRef, content.ProvenanceJsonUtf8 = epoch, domain.TextID(lineage), provenanceRaw
	content.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": domain.TextID(epoch), "durable_seq": "0"})
	if err != nil {
		return err
	}
	version, err := repository.InsertMergeVersion(ctx, tx, content, "RESTORE", content.CaptureId)
	if err != nil {
		return err
	}
	branchBody, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
	if err != nil {
		return err
	}
	body, err := codec.Decode(branchBody)
	if err != nil {
		return err
	}
	install := resolutionInstall{Fixed: fixed, Version: version, draftInstall: draftInstall{Context: fixed.Context, Branch: scope.Workspace.Workspace, OldRoom: scope.Room, NewRoom: room, Epoch: epoch, Lineage: lineage, Preview: prepared.Preview, Digest: prepared.Digest, Content: content, LineageContext: map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(fixed.BaselineVersion), "body": body, "provenance": provenance}}}
	encoded, err := json.Marshal(install)
	if err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(lineage), "content_version_id": domain.TextID(version)}})
	if err != nil {
		return err
	}
	// 不改变正式MAIN、原轮次或意见。新谱系、可靠修复版本、RESTORING及系统安装责任同事务提交。
	if err = repository.AdvancePulledBranch(ctx, tx, scope.Workspace.Workspace, lineage, epoch); err != nil {
		return err
	}
	if err = repository.CompleteResolution(ctx, tx, task, "SUCCEEDED", result, encoded); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_install.go：保留原实现，按业务合并文件。

// 系统安装读取实际版本、谱系与原固定候选交叉核验；两份可变恢复payload相互一致不等于可信。
func (s *Resolutions) validateInstallSource(ctx context.Context, q *sql.Tx, install resolutionInstall) error {
	unknown := domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	if install.Context == nil || install.Context.Target == nil {
		return unknown
	}
	raw, err := repository.MergeArtifact(ctx, q, "RESOLUTION_CONTEXT", install.Context.OperationId)
	if err != nil {
		return err
	}
	fixed, err := decodeResolutionContext(raw)
	if err != nil {
		return err
	}
	givenRaw, _ := json.Marshal(install.Fixed)
	given, err := decodeResolutionContext(givenRaw)
	if err != nil || !sameResolutionContext(given, fixed) || !proto.Equal(install.Context, fixed.Context) || install.Content == nil || install.Version == 0 {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ORIGINAL_COPY_UNKNOWN")
	}
	// 固定成功结果也参与交叉核验；恢复payload保存正确正文不能掩盖结果指向另一真实旧版本。
	receipt, err := repository.ResolutionResult(ctx, q, fixed.Intent.Operation)
	if err != nil {
		return err
	}
	var result struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Result    struct {
			Lineage string `json:"entity_id"`
			Version string `json:"content_version_id"`
		} `json:"result"`
	}
	if receipt.State != "SUCCEEDED" || receipt.Actor != fixed.Actor || !bytes.Equal(receipt.Fingerprint, fixed.Context.RequestFingerprint) || json.Unmarshal(receipt.Result, &result) != nil || result.Operation != fixed.Intent.Operation || result.State != "SUCCEEDED" || result.Result.Lineage != domain.TextID(install.Lineage) || result.Result.Version != domain.TextID(install.Version) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_BINDING_UNKNOWN")
	}
	previewRaw, err := repository.MergeArtifact(ctx, q, "RESOLUTION_FIXED", fixed.Intent.Operation)
	if err != nil {
		return err
	}
	preview, err := s.fixedPreview(ctx, q, previewRaw, fixed)
	if err != nil {
		return err
	}
	if !bytes.Equal(preview.Digest, install.Digest) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_FIXED_UNKNOWN")
	}
	baseline, err := resolutionBaseline(ctx, q, fixed)
	if err != nil {
		return err
	}
	baseVersion, err := repository.VersionContent(ctx, q, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	baseMatches, err := conflictVersionMatches(baseVersion, baseline, fixed.BaselineVersion)
	if err != nil || !baseMatches {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BASELINE_UNKNOWN")
	}
	expected := proto.Clone(preview.Candidate).(*pb.FixedContent)
	provenanceValue, err := codec.Decode(expected.ProvenanceJsonUtf8)
	if err != nil {
		return err
	}
	provenance, ok := provenanceValue.(map[string]any)
	if !ok {
		return unknown
	}
	provenance["lineage_id"], provenance["base_manifest_id"], provenance["base_content_version_id"] = domain.TextID(install.Lineage), install.LineageContext["base_manifest_id"], domain.TextID(fixed.BaselineVersion)
	expected.WorkspaceEpoch, expected.LineageRef = install.Epoch, domain.TextID(install.Lineage)
	expected.ProvenanceJsonUtf8, err = codec.Canonical(provenance)
	if err != nil {
		return err
	}
	expected.CutJsonUtf8, err = codec.Canonical(map[string]any{"room_id": domain.TextID(install.NewRoom), "workspace_epoch": domain.TextID(install.Epoch), "durable_seq": "0"})
	if err != nil || !proto.Equal(expected, install.Content) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CANDIDATE_UNKNOWN")
	}
	stored, err := repository.VersionContent(ctx, q, install.Version, fixed.Scope.Document)
	if err != nil {
		return err
	}
	matched, err := conflictVersionMatches(stored, install.Content, install.Version)
	if err != nil || !matched {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_VERSION_UNKNOWN")
	}
	lineage, err := repository.ResolutionLineage(ctx, q, install.Lineage)
	if err != nil {
		return err
	}
	if lineage.Branch != fixed.Scope.Workspace.Workspace || lineage.Document != fixed.Scope.Document || lineage.Unit != fixed.Scope.Unit || lineage.Previous != fixed.Scope.Workspace.Lineage || lineage.Version != fixed.BaselineVersion || lineage.Operation != fixed.Intent.Operation || lineage.Capture != fixed.Intent.Operation || lineage.Kind != "RESTORED" {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_LINEAGE_UNKNOWN")
	}
	original, _, err := s.original(ctx, q, fixed.Origin, fixed.Intent, false)
	if err != nil || lineage.Manifest != original.Attempt.Main.ExpectedManifestId {
		return unknown
	}
	provenanceDecoded, err := codec.Decode(lineage.Provenance)
	if err != nil {
		return unknown
	}
	p, ok := provenanceDecoded.(map[string]any)
	if !ok {
		return unknown
	}
	canonical, err := codec.Canonical(p)
	if err != nil || !bytes.Equal(canonical, install.Content.ProvenanceJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_PROVENANCE_UNKNOWN")
	}
	body, err := repository.VersionContent(ctx, q, fixed.BaselineVersion, fixed.Scope.Document)
	if err != nil {
		return err
	}
	projected, err := domain.BranchBaselineBody(body.BodyJsonUtf8)
	if err != nil {
		return err
	}
	expectedBody, err := codec.Decode(projected)
	if err != nil {
		return err
	}
	expectedLineage := map[string]any{"lineage_id": domain.TextID(install.Lineage), "base_manifest_id": domain.TextID(lineage.Manifest), "base_version_id": domain.TextID(fixed.BaselineVersion), "body": expectedBody, "provenance": p}
	a, e1 := codec.Canonical(expectedLineage)
	b, e2 := codec.Canonical(install.LineageContext)
	if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_LINEAGE_CONTEXT_UNKNOWN")
	}
	roomRaw, err := repository.MergeArtifact(ctx, q, "COLLABORATION_ROOM", "BRANCH:"+domain.TextID(install.Branch)+":"+domain.TextID(fixed.Scope.Unit)+":"+domain.TextID(install.Epoch))
	if err != nil {
		return err
	}
	var room map[string]string
	if json.Unmarshal(roomRaw, &room) != nil || room["room_id"] != domain.TextID(install.NewRoom) || install.Branch != fixed.Scope.Workspace.Workspace || install.OldRoom != fixed.Scope.Room || install.Epoch != fixed.Scope.Workspace.Epoch+1 {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_ROOM_UNKNOWN")
	}
	return nil
}

func (s *Resolutions) CommittedSource(ctx context.Context, query *pb.OperationQuery) (map[string]any, error) {
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return nil, err
	}
	if authority.State != pb.OperationResult_SUCCEEDED && authority.State != pb.OperationResult_REJECTED {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_RESULT_UNKNOWN")
	}
	raw, err := repository.MergeArtifact(ctx, s.db, "RESOLUTION_COMMITTED", query.OperationId)
	if err != nil {
		return nil, err
	}
	install, decodeError := decodeResolutionInstall(raw)
	if decodeError != nil || validatePullCommittedBinding(install.draftInstall, query) != nil || install.Fixed.Intent.Operation != query.OperationId {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	}
	value := map[string]any{"old_room_id": domain.TextID(install.OldRoom), "result_digest": base64.StdEncoding.EncodeToString(authority.ResultDigest), "expected_round_id": domain.TextID(install.Fixed.Intent.Round)}
	if authority.State == pb.OperationResult_SUCCEEDED {
		tx, beginError := s.db.BeginTx(ctx, nil)
		if beginError != nil {
			return nil, beginError
		}
		err = s.validateInstallSource(ctx, tx, install)
		_ = tx.Rollback()
		if err != nil {
			return nil, err
		}
		content, e := fixedWire(install.Content)
		if e != nil {
			return nil, e
		}
		value["content"], value["lineage"], value["new_room_id"], value["new_epoch"] = content, install.LineageContext, domain.TextID(install.NewRoom), domain.TextID(install.Epoch)
	}
	return value, nil
}

func (s *Resolutions) reconcile(ctx context.Context, task domain.SubmissionTask) error {
	install, decodeError := decodeResolutionInstall(task.Payload)
	if decodeError != nil || install.Context == nil || install.Fixed.Intent.Operation != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_UNKNOWN")
	}
	query := &pb.OperationQuery{OperationId: task.Operation, Target: install.Context.Target, RequestFingerprint: install.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if _, err = s.CommittedSource(ctx, query); err != nil {
		return err
	}
	reply, err := s.client.Apply(ctx, &pb.DraftResolutionRequest{Context: install.Context, ExpectedRoundId: install.Fixed.Intent.Round, ExpectedAttemptId: install.Preview, ExpectedInputDigest: install.Digest, Resolution: install.Content})
	if err != nil {
		return err
	}
	if reply == nil || reply.OperationId != task.Operation || reply.State != authority.State || !bytes.Equal(reply.ResultDigest, authority.ResultDigest) || !bytes.Equal(reply.FixedResultJsonUtf8, authority.FixedResultJsonUtf8) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	var current struct {
		Sync    string `json:"draft_sync_state"`
		Room    string `json:"new_room_id"`
		Epoch   string `json:"new_epoch"`
		Lineage string `json:"lineage_id"`
		Digest  []byte `json:"content_digest"`
	}
	if json.Unmarshal(reply.CurrentStateJsonUtf8, &current) != nil || current.Sync != "READY" {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	if authority.State == pb.OperationResult_SUCCEEDED && (current.Room != domain.TextID(install.NewRoom) || current.Epoch != domain.TextID(install.Epoch) || current.Lineage != domain.TextID(install.Lineage) || !bytes.Equal(current.Digest, install.Content.BodyHash)) {
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_INSTALL_ACK_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, install.Fixed); err != nil {
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
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	// 这里只完成成功时登记的系统责任；注销不能取消已经提交的修复，也不触发意见解除或请求归档。
	if authority.State == pb.OperationResult_SUCCEEDED {
		if err = s.validateInstallSource(ctx, tx, install); err != nil {
			return err
		}
		pointer, guard, e := resolutionReview(ctx, tx, install.Fixed.Origin, install.Fixed.Intent.Round)
		if e != nil {
			return e
		}
		if pointer != install.Fixed.Review || guard != install.Fixed.Guard {
			return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_REVIEW_CHANGED")
		}
	}
	if err = repository.CompleteResolutionInstallation(ctx, tx, task, install.Branch, install.Epoch, authority.State == pb.OperationResult_SUCCEEDED); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_original.go：保留原实现，按业务合并文件。

// 登记时要求公共冲突仍有效；已经登记的系统恢复继续核验原固定事实，不依赖公共页面的TTL。
func (s *Resolutions) original(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, intent domain.ConflictResolutionIntent, fresh bool) (mergeFixedInputs, conflictVersions, error) {
	var empty mergeFixedInputs
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	actual, err := repository.MergeRow(ctx, tx, origin.Operation)
	if err != nil || !reflect.DeepEqual(actual, origin) {
		return empty, conflictVersions{}, unknown
	}
	receipt, err := repository.MergeResult(ctx, tx, origin.Operation)
	if err != nil {
		return empty, conflictVersions{}, err
	}
	if receipt.State != "REJECTED" || (fresh && !receipt.Expires.After(time.Now())) {
		return empty, conflictVersions{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	attempt, err := repository.ConflictAttemptRow(ctx, tx, origin.Operation)
	if err != nil {
		return empty, conflictVersions{}, err
	}
	if attempt.State != "CONFLICT" || attempt.ID != intent.Attempt || !bytes.Equal(attempt.Digest, intent.Digest) || origin.Round != intent.Round {
		return empty, conflictVersions{}, domain.Violation(domain.StateConflict, "CONFLICT_ATTEMPT_CHANGED")
	}
	var conflicts []dto.Conflict
	if json.Unmarshal(attempt.Conflicts, &conflicts) != nil {
		return empty, conflictVersions{}, unknown
	}
	if err = intent.ValidateConflictSet(len(conflicts)); err != nil {
		return empty, conflictVersions{}, err
	}
	fixed, err := readConflictFixed(ctx, tx, s.validator, origin, attempt)
	if err != nil {
		return fixed, conflictVersions{}, err
	}
	if !bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint) {
		return fixed, conflictVersions{}, unknown
	}
	var versions conflictVersions
	if fresh {
		versions, err = ensureConflictVersions(ctx, tx, origin, fixed)
	} else {
		raw, e := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(origin.Operation, attempt.ID))
		if e != nil {
			return fixed, versions, e
		}
		versions, err = validateConflictVersions(ctx, tx, raw, fixed)
	}
	return fixed, versions, err
}

func lockResolutionOperations(ctx context.Context, tx *sql.Tx, fixed resolutionContext) error {
	// 两个操作互斥都先于来源与文档锁；GET物化旧冲突也采用这个方向，避免交叉等待。
	operations := []string{fixed.Intent.Operation, fixed.Intent.Conflict}
	sort.Strings(operations)
	for _, operation := range operations {
		if err := repository.LockOperationGate(ctx, tx, operation); err != nil {
			return err
		}
	}
	return nil
}

func resolutionReview(ctx context.Context, tx *sql.Tx, origin domain.MergeFact, round uint64) (domain.ReviewPointer, domain.ReviewGuard, error) {
	guard, err := repository.LockedReviewGuard(ctx, tx, origin.Unit)
	if err != nil {
		return domain.ReviewPointer{}, guard, err
	}
	if guard.Ongoing != 0 {
		return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, "REVIEW_WITHDRAW_REQUIRED")
	}
	if guard.Submission != "" || guard.Resumable != origin.Request {
		return domain.ReviewPointer{}, guard, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
	pointer, err := repository.LockedReviewPointer(ctx, tx, origin.Request)
	if err != nil {
		return pointer, guard, err
	}
	if pointer.Archived || pointer.Status != "WITHDRAWN" || pointer.Round != round || pointer.Document != origin.Document || pointer.Unit != origin.Unit || pointer.Branch != origin.Branch {
		return pointer, guard, domain.Violation(domain.StateConflict, "RESOLUTION_REVIEW_CHANGED")
	}
	active, err := repository.LockedActiveMerge(ctx, tx, origin.Request)
	if err == nil && active != "" {
		err = domain.Violation(domain.StateConflict, "MERGE_PENDING")
	}
	return pointer, guard, err
}

func resolutionBody(validator *codec.Validator, raw []byte) ([]uint64, error) {
	invalid := domain.Violation(domain.InvalidRequest, "RESOLUTION_CONTENT_INVALID")
	body, err := codec.Decode(raw)
	if err != nil || validator.Body(body) != nil {
		return nil, invalid
	}
	projected, err := domain.BranchBaselineBody(raw)
	if err != nil || !bytes.Equal(projected, raw) {
		return nil, invalid
	}
	seen := map[uint64]bool{}
	blocks := map[string]bool{}
	var walk func(any) error
	walk = func(value any) error {
		node, ok := value.(map[string]any)
		if !ok {
			return invalid
		}
		// Schema只验证单节点属性。稳定块ID在整篇唯一，行内图片可重复引用同一资源。
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return invalid
				}
				blocks[id] = true
			}
		}
		if node["type"] == "image" {
			attrs, ok := node["attrs"].(map[string]any)
			if !ok {
				return invalid
			}
			text, ok := attrs["resourceId"].(string)
			if !ok {
				return invalid
			}
			id, e := domain.CanonicalID(text)
			if e != nil {
				return invalid
			}
			seen[id] = true
		}
		children, _ := node["content"].([]any)
		for _, child := range children {
			if e := walk(child); e != nil {
				return e
			}
		}
		return nil
	}
	if err = walk(body); err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// 从原冲突真实MAIN版本投影当前仍完整的稳定单元；不会把后来MAIN尾部提升成已接纳基线。
func resolutionBaseline(ctx context.Context, tx *sql.Tx, fixed resolutionContext) (*pb.FixedContent, error) {
	baseline, err := repository.VersionContent(ctx, tx, fixed.Versions.Main, fixed.Scope.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(baseline, fixed.Versions.Main); err != nil {
		return nil, err
	}
	return projectMergeVersion(ctx, tx, baseline, fixed.Versions.Main, fixed.Scope.Structure, nil)
}

// conflict_resolution_recovery.go：保留原实现，按业务合并文件。

func (s *Resolutions) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimResolutionTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	fact, bindErr := repository.ClaimedResolutionBinding(ctx, s.db, task.Operation)
	fixed, err := validatePendingResolutionBinding(fact)
	if bindErr != nil || fact.Kind != task.Kind || !bytes.Equal(fact.Payload, task.Payload) {
		err = domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_BINDING_UNKNOWN")
	}
	if err == nil {
		if task.Kind == "RESOLUTION_RECONCILE" {
			err = s.reconcile(ctx, task)
		} else {
			proofs, resumeErr := s.resume(ctx, fixed)
			err = resumeErr
			if err == nil {
				check, e := s.db.BeginTx(ctx, nil)
				err = e
				if err == nil {
					err = lockResolutionOperations(ctx, check, fixed)
					if err == nil {
						_, err = s.validate(ctx, check, fixed, proofs)
					}
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

func (s *Resolutions) reject(ctx context.Context, task domain.SubmissionTask, fixed resolutionContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
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
		return domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_CONTROL_UNKNOWN")
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	install, _ := json.Marshal(resolutionInstall{Fixed: fixed, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, Epoch: fixed.Scope.Workspace.Epoch, Preview: fixed.Intent.Operation}})
	// 无论是否已收到prepare响应，都由原号权威终态驱动Node收尾，不通过超时猜测屏障未建立。
	if err = repository.CompleteResolution(ctx, tx, task, "REJECTED", result, install); err != nil {
		return err
	}
	return tx.Commit()
}

// conflict_resolution_register.go：保留原实现，按业务合并文件。

func (s *Resolutions) Register(ctx context.Context, actor Actor, origin domain.MergeFact, intent domain.ConflictResolutionIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, origin.Document, "CONFLICT_RESOLUTION")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Edit[origin.Unit] {
		return actor, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	fingerprint, err := domain.ConflictResolutionFingerprint(path, intent)
	if err != nil {
		return actor, err
	}
	if prior, e := repository.ResolutionResult(ctx, s.db, intent.Operation); e == nil {
		if prior.Actor != actor.ID || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, nil
	} else if e != sql.ErrNoRows {
		return actor, e
	}
	resources, err := resolutionBody(s.validator, intent.Body)
	if err != nil {
		return actor, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, origin.Branch, origin.Document, origin.Unit)
	if err != nil {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return actor, err
	}
	target := &pb.Target{DocumentId: origin.Document, UnitId: origin.Unit, BranchId: origin.Branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "CONFLICT_RESOLUTION"}
	var bound []*pb.IdentityAdmission
	bound, err = s.identity.BindProofs(ctx, actor, access.Proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
	if len(bound) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "RESOLUTION_IDENTITY_UNKNOWN")
	}
	access.Proofs = bound
	actor.Proof = bound[0]
	actor.Sources = access.Sources
	fixed := resolutionContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Origin: origin, Access: access}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = lockResolutionOperations(ctx, tx, fixed); err != nil {
		return actor, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return actor, err
	}
	if prior, e := repository.LockedOperation(ctx, tx, intent.Operation); e == nil {
		if prior.ActorID != actor.ID || prior.Kind != "CONFLICT_RESOLUTION" || !bytes.Equal(prior.Fingerprint, fingerprint[:]) {
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
	if current.CurrentStructureId != domain.TextID(structure) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, origin.Document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	active, err := repository.LockedActiveBranch(ctx, tx, origin.Unit, origin.Document)
	if err != nil || active != origin.Branch {
		return actor, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	actual, err := repository.LockedControlledWorkspace(ctx, tx, active, origin.Document, origin.Unit)
	if err != nil || actual != workspace {
		return actor, domain.Violation(domain.StateConflict, "WORKSPACE_CHANGED")
	}
	if err = repository.RequireBranchControlOpen(ctx, tx, active); err != nil {
		return actor, err
	}
	fixed.Review, fixed.Guard, err = resolutionReview(ctx, tx, origin, intent.Round)
	if err != nil {
		return actor, err
	}
	original, versions, err := s.original(ctx, tx, origin, intent, true)
	if err != nil {
		return actor, err
	}
	if err = requireStableReviewRange(ctx, tx, origin.Document, origin.Unit, original.Input.Main.StructureId, structure); err != nil {
		return actor, err
	}
	fixed.Versions = versions
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	control, _ := domain.CanonicalRevision(current.ControlRevision)
	room, err := collaborationRoom(ctx, tx, "BRANCH", active, origin.Unit, actual.Epoch)
	if err != nil {
		return actor, err
	}
	mainEpoch, err := repository.MainWorkspaceEpoch(ctx, tx, origin.Document)
	if err != nil {
		return actor, err
	}
	fixed.Scope = draftSourceScope{Document: origin.Document, Unit: origin.Unit, Room: room, Manifest: manifest, Structure: structure, Style: style, Control: control, MainEpoch: mainEpoch, Workspace: actual}
	fixed.Access.Document = current
	baseline, err := resolutionBaseline(ctx, tx, fixed)
	if err != nil {
		return actor, err
	}
	fixed.BaselineVersion = versions.Main
	if baseline.StructureId != original.Input.Main.StructureId {
		fixed.BaselineVersion, err = repository.InsertMergeVersion(ctx, tx, baseline, "RESTORE", baseline.CaptureId)
		if err != nil {
			return actor, err
		}
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, resource := range resources {
		state, e := repository.LockedDocumentResource(ctx, tx, resource, origin.Document, space)
		if e != nil {
			return actor, e
		}
		if state != "READY" {
			return actor, domain.Violation(domain.StateConflict, "RESOLUTION_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, resource, origin.Document, intent.Operation, 1); err != nil {
			return actor, err
		}
	}
	command := OperationCommand{ID: intent.Operation, Kind: "CONFLICT_RESOLUTION", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, mainControlPending(intent.Operation)); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, origin.Document, "CONFLICT_RESOLUTION", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureDiscardPreviewAdmission(ctx, tx, admission, intent.Operation, origin.Unit, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, Target: target, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterResolution(ctx, tx, active, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}
