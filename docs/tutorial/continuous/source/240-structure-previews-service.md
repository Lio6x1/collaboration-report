# services/document/internal/logic/document/structure_previews_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/240-structure-previews-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`validateStructurePreviewUnits`、`structurePreviewBlockingUnits`、`validatePendingStructurePreviewBinding`、`validateOriginalStructurePreviewBinding`、`Ready`、`Wait`、`Read`、`resultAccess`、`visibleResult`、`Progress`、`OperationResult`、`RecoverOne`、`captureAndCommit`、`structurePreviewCaptureError`、`Register`、`structurePreviewSnapshot`、`validateStructurePreviewSnapshot`、`sameStructurePreviewRef`、`Lookup`、`resume`、`validateStructurePreviewAdmission`、`validate`、`Source`、`StructurePreviewClient`、`StructurePreviews`、`structurePreviewUnit`、`structurePreviewContext`、`NewStructurePreviews`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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

// structure_preview_guards.go：保留原实现，按业务合并文件。

func validateStructurePreviewUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) error {
	currentUnits, err := repository.CurrentStructurePreviewUnits(ctx, tx, fixed.Document, fixed.Structure)
	if err != nil {
		return err
	}
	if len(currentUnits) != len(fixed.Units) {
		return domain.Violation(domain.StateConflict, "STRUCTURE_SCOPE_CHANGED")
	}
	for i, unit := range fixed.Units {
		actual := currentUnits[i]
		if actual.Unit != unit.Unit || actual.Chapter != unit.Chapter || actual.Branch != unit.Workspace.Workspace {
			return domain.Violation(domain.StateConflict, "STRUCTURE_SCOPE_CHANGED")
		}
		workspace, err := repository.SubmissionWorkspace(ctx, tx, actual.Branch, fixed.Document, actual.Unit)
		if err != nil || workspace != unit.Workspace {
			return domain.Violation(domain.StateConflict, "STRUCTURE_BRANCH_CHANGED")
		}
	}
	return nil
}

// 预览报告阻止单元，确认/最终提交则须拒绝。两者使用同一当前guard规则，不撤回或归档原请求。
func structurePreviewBlockingUnits(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext) ([]map[string]string, error) {
	blocks := []map[string]string{}
	affected := map[uint64]bool{}
	for _, change := range fixed.Intent.Input.Changes {
		if change.Type == "SPLIT_LEVEL2" || change.Type == "JOIN_CHAPTER" || change.Type == "REMOVE" {
			id, _ := domain.CanonicalID(change.ChapterId)
			affected[id] = true
		}
	}
	for _, unit := range fixed.Units {
		if !affected[unit.Chapter] {
			continue
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, unit.Unit)
		if err != nil {
			return nil, err
		}
		reason := ""
		if guard.Submission != "" {
			reason = "SUBMISSION_PENDING"
		} else if guard.Ongoing != 0 {
			reason = "ACTIVE_REVIEW"
		} else if guard.Resumable != 0 {
			changes, err := repository.LockedReviewChanges(ctx, tx, guard.Resumable)
			if err != nil {
				return nil, err
			}
			for _, change := range changes {
				if change.Status == "OPEN" {
					reason = "OPEN_CHANGE_REQUEST"
					break
				}
			}
		}
		if reason != "" {
			blocks = append(blocks, map[string]string{"unit_id": domain.TextID(unit.Unit), "reason": reason})
		}
	}
	return blocks, nil
}

// structure_preview_readiness.go：保留原实现，按业务合并文件。

func validatePendingStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error) {
	var original, progress structurePreviewContext
	invalid := errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	if fact.State != "PREPARING" || json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &progress) != nil {
		return original, invalid
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(progress)
	if !bytes.Equal(a, b) {
		return original, invalid
	}
	return validateOriginalStructurePreviewBinding(fact)
}

// 原事实核验与可变恢复状态分开：确认只能读取真实SUCCEEDED事实，不能伪造PREPARING复用就绪门槛。
func validateOriginalStructurePreviewBinding(fact repository.PendingStructurePreviewBinding) (structurePreviewContext, error) {
	var original structurePreviewContext
	invalid := errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	if json.Unmarshal(fact.Original, &original) != nil {
		return original, invalid
	}
	ctx := original.Context
	if ctx == nil || ctx.Target == nil || original.Actor == 0 || original.Document == 0 || original.Structure == 0 || original.Manifest == 0 || original.Style == 0 || original.SessionExpires <= 0 ||
		original.Actor != fact.Actor || ctx.ActorId != fact.Actor || ctx.OperationId != fact.Operation || original.Intent.Operation != fact.Operation || len(fact.Fingerprint) != 32 || !bytes.Equal(ctx.RequestFingerprint, fact.Fingerprint) ||
		original.Structure != original.Intent.Structure || original.Control != original.Intent.Control || len(original.Units) == 0 || !original.Access.Permissions.Manage || original.Access.Document.MainLocked ||
		original.Access.Document.Id != domain.TextID(original.Document) || original.Access.Document.CurrentManifestId != domain.TextID(original.Manifest) || original.Access.Document.CurrentStructureId != domain.TextID(original.Structure) ||
		original.Access.Document.CurrentStyleId != domain.TextID(original.Style) || original.Access.Document.ControlRevision != domain.TextID(original.Control) || len(ctx.Sources) == 0 ||
		!proto.Equal(&pb.OperationContext{Sources: ctx.Sources}, &pb.OperationContext{Sources: original.Access.Sources}) {
		return original, invalid
	}
	expected := &pb.Target{DocumentId: original.Document, StructureId: original.Structure, WorkspaceEpoch: original.Control, Purpose: "STRUCTURE_PREVIEW"}
	if !proto.Equal(ctx.Target, expected) || ctx.IdentityAdmissionRef == "" || ctx.DocumentAdmissionRef == "" || len(original.Access.Proofs) == 0 {
		return original, invalid
	}
	parsed, err := domain.ParseStructurePreview(original.Intent.Input)
	if err != nil || parsed.Operation != original.Intent.Operation || parsed.Structure != original.Structure || parsed.Control != original.Control {
		return original, invalid
	}
	path := "/v1/documents/" + domain.TextID(original.Document) + "/structure-previews"
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Target, &target) != nil || target.Path != path {
		return original, invalid
	}
	fingerprint, err := domain.OperationFingerprint(path, original.Intent.Input)
	if err != nil || !bytes.Equal(fingerprint[:], fact.Fingerprint) {
		return original, invalid
	}
	available := make(map[string]uint64, len(ctx.Sources))
	for _, source := range ctx.Sources {
		if source == nil || source.ScopeKey == "" || source.Epoch == 0 || available[source.ScopeKey] != 0 {
			return original, invalid
		}
		available[source.ScopeKey] = source.Epoch
	}
	for _, proof := range original.Access.Proofs {
		if proof == nil || proof.ActorId != original.Actor || proof.OperationId != fact.Operation || proof.Purpose != "STRUCTURE_PREVIEW" || !proto.Equal(proof.Target, expected) || len(proof.SessionRef) != 32 || proof.ExpiresUnixMs <= 0 || len(proof.Sources) == 0 || !domain.ValidOperationID(proof.AdmissionId) {
			return original, invalid
		}
		// 自然过期可续接；缺会话或换来源则不是原操作，启动屏障不能把它当成可恢复任务。
		if !bytes.Equal(proof.SessionRef, original.Access.Proofs[0].SessionRef) {
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
	seen := map[uint64]bool{}
	for _, unit := range original.Units {
		if unit.Unit == 0 || unit.Chapter == 0 || unit.Room == 0 || unit.Workspace.Workspace == 0 || unit.Workspace.Epoch == 0 || unit.Workspace.Lineage == 0 || unit.Workspace.BaseManifest == 0 || unit.Workspace.BaseVersion == 0 || seen[unit.Unit] {
			return original, invalid
		}
		seen[unit.Unit] = true
	}
	return original, nil
}

func (s *StructurePreviews) Ready(ctx context.Context) error {
	facts, err := repository.PendingStructurePreviewBindings(ctx, s.db)
	if err != nil {
		return errors.New("STRUCTURE_PREVIEW_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		fixed, err := validatePendingStructurePreviewBinding(fact)
		if err != nil {
			return err
		}
		admission, err := repository.OriginalAdmission(ctx, s.db, fixed.Context.DocumentAdmissionRef, false)
		if err != nil || validateStructurePreviewAdmission(fixed, admission) != nil {
			return errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
		}
		sources, err := repository.AdmissionSources(ctx, s.db, fixed.Context.DocumentAdmissionRef)
		if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
			return errors.New("STRUCTURE_PREVIEW_BINDING_UNKNOWN")
		}
	}
	return nil
}

// structure_preview_readings.go：保留原实现，按业务合并文件。

func (s *StructurePreviews) Wait(ctx context.Context, operation string) error {
	// 预览依靠持久任务续接，入口只短等；留出边缘4秒预算中的身份核验与错误响应时间。
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		fact, err := repository.StructurePreviewResult(ctx, s.db, operation)
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
			return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_PREPARING")
		case <-ticker.C:
		}
	}
}

func (s *StructurePreviews) Read(ctx context.Context, actor Actor, document uint64, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Document != document || fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	access, err := s.resultAccess(ctx, actor, document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visibleResult(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		if fact.State == "PREPARING" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_PREPARING")
		}
		if fact.State != "SUCCEEDED" {
			var rejected struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
			}
			return nil, domain.Violation(domain.StateConflict, rejected.Error.Code)
		}
		var value dto.StructurePreview
		if json.Unmarshal(fact.Result, &value) != nil || !sameStructurePreviewRef(value.PreviewCutRef, operation) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		value.PreviewCutRef = operation
		return value, nil
	}, nil
}

func (s *StructurePreviews) resultAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, document, "STRUCTURE_PREVIEW_RESULT")
	if err == nil && !access.Permissions.Manage {
		err = domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	return access, err
}

func (s *StructurePreviews) visibleResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (repository.MainControlReceipt, error) {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return repository.MainControlReceipt{}, err
	}
	fact, err := repository.StructurePreviewResult(ctx, tx, operation)
	if err != nil {
		return fact, err
	}
	if fact.Actor != actor.ID {
		return fact, domain.Violation(domain.EntityNotFound, "STRUCTURE_PREVIEW_NOT_FOUND")
	}
	if !fact.Expires.After(time.Now()) {
		return fact, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	return fact, nil
}

// POST返回固定预览；通用GET按冻结Operation仅投影状态和已有实体ID，不把私有切点塞入未知字段。
func (s *StructurePreviews) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	access, err := s.resultAccess(ctx, actor, fixed.Document)
	if err != nil {
		return nil, nil, err
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		fact, err := s.visibleResult(ctx, tx, actor, access, operation)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"operation_id": operation, "state": fact.State}
		if fact.State == "PREPARING" {
			result["poll_url"] = "/v1/operations/" + operation
			return result, nil
		}
		if fact.State == "SUCCEEDED" {
			result["result"] = map[string]any{"entity_id": domain.TextID(fixed.Structure)}
			return result, nil
		}
		var rejected map[string]any
		if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected["error"] == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		result["error"] = rejected["error"]
		return result, nil
	}, nil
}

func (s *StructurePreviews) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_PREVIEW_QUERY_INVALID")
	}
	fact, err := repository.StructurePreviewResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Context.Target) || !bytes.Equal(query.RequestFingerprint, fact.Fingerprint) || !bytes.Equal(query.RequestFingerprint, fixed.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if fact.Actor != fixed.Actor {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
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
		result.ResultIds = []uint64{fixed.Structure}
	} else {
		var rejected struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if fact.State != "REJECTED" || json.Unmarshal(fact.Result, &rejected) != nil || rejected.Error.Code == "" {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_RESULT_UNKNOWN")
		}
		result.BusinessCode = rejected.Error.Code
	}
	return result, nil
}

// structure_preview_recovery.go：保留原实现，按业务合并文件。

func (s *StructurePreviews) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimStructurePreviewTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed structurePreviewContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Context == nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	} else {
		original, readErr := s.Lookup(ctx, task.Operation)
		err = readErr
		if err == nil {
			a, _ := json.Marshal(original)
			b, _ := json.Marshal(fixed)
			if string(a) != string(b) {
				err = domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
			}
		}
		if err == nil {
			err = s.captureAndCommit(ctx, task, fixed)
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

func (s *StructurePreviews) captureAndCommit(ctx context.Context, task domain.SubmissionTask, fixed structurePreviewContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	input := &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedStructureId: fixed.Structure}
	reply, err := s.client.Preview(ctx, input)
	if err != nil {
		return structurePreviewCaptureError(err)
	}
	raw, err := s.client.FixedStructure(ctx, input)
	if err != nil {
		return err
	}
	snapshot, err := validateStructurePreviewSnapshot(s.validator, raw, fixed, reply)
	if err != nil {
		return err
	}
	if reply.ExpiresUnixMs <= time.Now().UnixMilli() {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_EXPIRED")
	}
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
	resources := map[uint64]bool{}
	for _, content := range append(snapshot.Main, snapshot.Drafts...) {
		for _, id := range content.ResourceIds {
			resources[id] = true
		}
	}
	resourceIDs := make([]uint64, 0, len(resources))
	for id := range resources {
		resourceIDs = append(resourceIDs, id)
	}
	sort.Slice(resourceIDs, func(i, j int) bool { return resourceIDs[i] < resourceIDs[j] })
	for _, id := range resourceIDs {
		state, err := repository.LockedDocumentResource(ctx, tx, id, fixed.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "STRUCTURE_RESOURCE_NOT_READY")
		}
		if err = repository.InsertOperationResourceRef(ctx, tx, id, fixed.Document, task.Operation, 1); err != nil {
			return err
		}
	}
	blocks, err := structurePreviewBlockingUnits(ctx, tx, fixed)
	if err != nil {
		return err
	}
	public := map[string]any{"preview_cut_ref": task.Operation, "structure_id": domain.TextID(fixed.Structure), "cut_digest": hex.EncodeToString(snapshot.Cut.DependencyDigest), "expires_at": time.UnixMilli(reply.ExpiresUnixMs).UTC().Format(time.RFC3339Nano), "blocking_units": blocks}
	result, err := json.Marshal(public)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "STRUCTURE_PREVIEW_FIXED", task.Operation, task.Operation, raw, "SUCCEEDED"); err != nil {
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

// 远端只有这些确定的正文前置条件可以终结原号；断连、未知范围和内部异常仍沿持久任务恢复。
func structurePreviewCaptureError(err error) error {
	rpc, ok := status.FromError(err)
	if !ok {
		return err
	}
	if rpc.Code() == codes.FailedPrecondition {
		switch rpc.Message() {
		case "DRAFT_STRUCTURE_CHANGED", "STRUCTURE_DRAFT_CONFLICT", "STRUCTURE_LEVEL2_MISSING", "STRUCTURE_TITLE_INVALID", "STRUCTURE_TITLE_CHANGED", "STRUCTURE_MODE_CHANGED", "STRUCTURE_RANGE_OVERLAP", "STRUCTURE_BLOCK_ID_COLLISION", "STRUCTURE_CONTENT_LIMIT":
			return domain.Violation(domain.StateConflict, rpc.Message())
		}
	}
	if rpc.Code() == codes.ResourceExhausted && rpc.Message() == "STRUCTURE_TRANSFER_LIMIT" {
		return domain.Violation(domain.StateConflict, rpc.Message())
	}
	return err
}

// structure_preview_register.go：保留原实现，按业务合并文件。

func (s *StructurePreviews) Register(ctx context.Context, actor Actor, document uint64, intent domain.StructurePreviewIntent, path string) (Actor, error) {
	access, err := s.access.Authorize(ctx, actor, document, "STRUCTURE_PREVIEW")
	if err != nil {
		return actor, err
	}
	if !access.Permissions.Manage {
		return actor, domain.Violation(domain.PermissionDenied, "STRUCTURE_MANAGE_REQUIRED")
	}
	fingerprint, err := domain.OperationFingerprint(path, intent.Input)
	if err != nil {
		return actor, err
	}
	actor.Sources = access.Sources
	if result, lookupErr := repository.StructurePreviewResult(ctx, s.db, intent.Operation); lookupErr == nil {
		if result.Actor != actor.ID || !bytes.Equal(result.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		// 当前MANAGE仍要求成立；原预览的结构/control可以已经过时，重复请求只读取原事实。
		return actor, nil
	} else if lookupErr != sql.ErrNoRows {
		return actor, lookupErr
	}
	// 明确无资格的页面不先占用账号原号；事务内仍再次复核，防止授权后到预约前发生改变。
	if access.Document.MainLocked {
		return actor, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
	}
	if access.Document.CurrentStructureId != domain.TextID(intent.Structure) || access.Document.ControlRevision != domain.TextID(intent.Control) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	target := &pb.Target{DocumentId: document, StructureId: intent.Structure, WorkspaceEpoch: intent.Control, Purpose: "STRUCTURE_PREVIEW"}
	proofs := access.Proofs
	if len(proofs) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_IDENTITY_UNKNOWN")
	}
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, err
	}
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
	if existing, lookupErr := repository.LockedOperation(ctx, tx, intent.Operation); lookupErr == nil {
		if existing.ActorID != actor.ID || existing.Kind != "STRUCTURE_PREVIEW" || !bytes.Equal(existing.Fingerprint, fingerprint[:]) {
			return actor, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		return actor, tx.Commit()
	} else if lookupErr != sql.ErrNoRows {
		return actor, lookupErr
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return actor, err
	}
	if current.CurrentStructureId != domain.TextID(intent.Structure) || current.ControlRevision != domain.TextID(intent.Control) {
		return actor, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	if current.MainLocked {
		return actor, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return actor, err
	}
	if barrier.Phase != "OPEN" {
		return actor, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return actor, err
	}
	for _, change := range intent.Input.Changes {
		found := false
		anchorFound := change.AfterChapterId == nil
		for _, entry := range structure.Entries {
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && change.AfterChapterId != nil && entry.ChapterId == *change.AfterChapterId {
				anchorFound = true
			}
			if entry.Kind == "CHAPTER" && entry.ParentNodeId == nil && entry.ChapterId == change.ChapterId {
				found = true
				if change.Type == "SPLIT_LEVEL2" && entry.SplitLevel2 || change.Type == "JOIN_CHAPTER" && !entry.SplitLevel2 {
					return actor, domain.Violation(domain.StateConflict, "STRUCTURE_MODE_CHANGED")
				}
			}
		}
		if !found {
			return actor, domain.Violation(domain.StateConflict, "CHAPTER_CHANGED")
		}
		if !anchorFound {
			return actor, domain.Violation(domain.StateConflict, "CHAPTER_ANCHOR_CHANGED")
		}
	}
	manifest, _ := domain.CanonicalID(current.CurrentManifestId)
	style, _ := domain.CanonicalID(current.CurrentStyleId)
	fixed := structurePreviewContext{Actor: actor.ID, Document: document, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Manifest: manifest, Structure: intent.Structure, Style: style, Control: intent.Control}
	units, err := repository.CurrentStructurePreviewUnits(ctx, tx, document, intent.Structure)
	if err != nil {
		return actor, err
	}
	if len(units) == 0 {
		return actor, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_SCOPE_UNKNOWN")
	}
	for _, unit := range units {
		workspace, err := repository.SubmissionWorkspace(ctx, tx, unit.Branch, document, unit.Unit)
		if err != nil {
			return actor, domain.Violation(domain.StateConflict, "STRUCTURE_BRANCH_CHANGED")
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", workspace.Workspace, unit.Unit, workspace.Epoch)
		if err != nil {
			return actor, err
		}
		fixed.Units = append(fixed.Units, structurePreviewUnit{Unit: unit.Unit, Chapter: unit.Chapter, Room: room, Workspace: workspace})
	}
	pending := map[string]any{"operation_id": intent.Operation, "state": "PREPARING"}
	command := OperationCommand{ID: intent.Operation, Kind: "STRUCTURE_PREVIEW", Path: path, Input: intent.Input}
	if err = insertMutationReceipt(ctx, tx, actor, command, fingerprint, pending); err != nil {
		return actor, err
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "STRUCTURE_PREVIEW", admission); err != nil {
		return actor, err
	}
	rawTarget, _ := json.Marshal(target)
	if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
		return actor, err
	}
	fixed.Context = &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}
	raw, err := json.Marshal(fixed)
	if err != nil {
		return actor, err
	}
	if err = repository.RegisterStructurePreview(ctx, tx, intent.Operation, raw); err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

// structure_preview_snapshot.go：保留原实现，按业务合并文件。

type structurePreviewSnapshot struct {
	Main, Drafts []*pb.FixedContent
	Cut          *pb.CutVector
}

func validateStructurePreviewSnapshot(validator *codec.Validator, raw []byte, fixed structurePreviewContext, reply *pb.DraftPreviewReply) (structurePreviewSnapshot, error) {
	var snapshot structurePreviewSnapshot
	invalid := domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_FIXED_UNKNOWN")
	if reply == nil || reply.State != pb.CaptureReply_FIXED || !sameStructurePreviewRef(reply.PreviewCutRef, fixed.Intent.Operation) || reply.MainManifestId != fixed.Manifest || reply.ObservedCut == nil || len(reply.InputDigest) != 32 || len(raw) > 24<<20 {
		return snapshot, invalid
	}
	var envelope struct {
		Binding json.RawMessage   `json:"binding"`
		Main    []json.RawMessage `json:"main_units"`
		Drafts  []json.RawMessage `json:"drafts"`
		Cut     json.RawMessage   `json:"observed_cut"`
		Digest  []byte            `json:"input_digest"`
		Preview string            `json:"preview_cut_ref"`
		Expires string            `json:"expires_unix_ms"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Preview != reply.PreviewCutRef || envelope.Expires != domain.TextID(uint64(reply.ExpiresUnixMs)) || !bytes.Equal(envelope.Digest, reply.InputDigest) {
		return snapshot, invalid
	}
	binding := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(envelope.Binding, binding) != nil || !proto.Equal(binding.Context, fixed.Context) || binding.ExpectedBranchId != 0 || binding.ExpectedLineageId != 0 || binding.ExpectedStructureId != fixed.Structure {
		return snapshot, invalid
	}
	value, err := codec.Decode(raw)
	if err != nil {
		return snapshot, err
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return snapshot, invalid
	}
	delete(fields, "input_digest")
	canonical, err := codec.Canonical(fields)
	if err != nil {
		return snapshot, err
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(digest[:], reply.InputDigest) {
		return snapshot, invalid
	}
	snapshot.Cut = &pb.CutVector{}
	if protojson.Unmarshal(envelope.Cut, snapshot.Cut) != nil || !proto.Equal(snapshot.Cut, reply.ObservedCut) || snapshot.Cut.DocumentId != fixed.Document || snapshot.Cut.StructureId != fixed.Structure || snapshot.Cut.StyleId != fixed.Style || snapshot.Cut.ControlRevision != fixed.Control || len(snapshot.Cut.Rooms) > 10000 {
		return snapshot, invalid
	}
	seenRooms := map[uint64]bool{}
	for _, room := range snapshot.Cut.Rooms {
		if room == nil || room.RoomId == 0 || room.WorkspaceEpoch == 0 || seenRooms[room.RoomId] {
			return snapshot, invalid
		}
		seenRooms[room.RoomId] = true
	}
	rooms, err := codec.Canonical(domain.CutBinding(snapshot.Cut)["rooms"])
	if err != nil {
		return snapshot, err
	}
	roomDigest := sha256.Sum256(rooms)
	if !bytes.Equal(roomDigest[:], snapshot.Cut.DependencyDigest) {
		return snapshot, invalid
	}
	if len(envelope.Main) != len(fixed.Units) || len(envelope.Drafts) != len(fixed.Units) {
		return snapshot, invalid
	}
	expected := map[uint64]structurePreviewUnit{}
	for _, unit := range fixed.Units {
		expected[unit.Unit] = unit
	}
	decode := func(items []json.RawMessage, draft bool) ([]*pb.FixedContent, error) {
		result := make([]*pb.FixedContent, 0, len(items))
		seen := map[uint64]bool{}
		for _, item := range items {
			content := &pb.FixedContent{}
			if protojson.Unmarshal(item, content) != nil || seen[content.UnitId] {
				return nil, invalid
			}
			unit, exists := expected[content.UnitId]
			if !exists || content.OperationId != fixed.Intent.Operation {
				return nil, invalid
			}
			if draft {
				if content.BranchId != unit.Workspace.Workspace || content.WorkspaceEpoch != unit.Workspace.Epoch || content.LineageRef != domain.TextID(unit.Workspace.Lineage) {
					return nil, invalid
				}
			} else if content.BranchId != 0 || content.WorkspaceEpoch != fixed.Control {
				return nil, invalid
			}
			if err := validateFixedMergeContent(validator, content, fixed.Document, unit.Unit, fixed.Structure); err != nil {
				return nil, err
			}
			hash := sha256.Sum256(content.BodyJsonUtf8)
			if !bytes.Equal(hash[:], content.BodyHash) {
				return nil, invalid
			}
			seen[unit.Unit] = true
			result = append(result, content)
		}
		return result, nil
	}
	if snapshot.Main, err = decode(envelope.Main, false); err != nil {
		return snapshot, err
	}
	snapshot.Drafts, err = decode(envelope.Drafts, true)
	return snapshot, err
}

// 只兼容首轮实际固定事实的准确旧前缀，不改其不可变字节/摘要；新公共引用统一为原操作号。
func sameStructurePreviewRef(reference, operation string) bool {
	return reference == operation || reference == "structure-preview:"+operation
}

// structure_preview_source.go：保留原实现，按业务合并文件。

func (s *StructurePreviews) Lookup(ctx context.Context, operation string) (structurePreviewContext, error) {
	var fixed structurePreviewContext
	raw, err := repository.MergeArtifact(ctx, s.db, "STRUCTURE_PREVIEW_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Context == nil || fixed.Context.Target == nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return fixed, nil
}

func (s *StructurePreviews) resume(ctx context.Context, fixed structurePreviewContext) ([]*pb.IdentityAdmission, error) {
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

func validateStructurePreviewAdmission(fixed structurePreviewContext, actual repository.OriginalAdmissionBinding) error {
	context := fixed.Context
	if context == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil || actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != 0 || actual.Identity != context.IdentityAdmissionRef ||
		actual.Operation != context.OperationId || actual.Purpose != "STRUCTURE_PREVIEW" || actual.SessionExpires != fixed.SessionExpires || !bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return nil
}

func (s *StructurePreviews) validate(ctx context.Context, tx *sql.Tx, fixed structurePreviewContext, proofs []*pb.IdentityAdmission) error {
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	if !fixed.Access.Permissions.Manage || current.MainLocked || current.CurrentManifestId != domain.TextID(fixed.Manifest) || current.CurrentStructureId != domain.TextID(fixed.Structure) || current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return domain.Violation(domain.StateConflict, "STRUCTURE_PREVIEW_CONTEXT_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return err
	}
	if barrier.Phase != "OPEN" {
		return domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	if err = validateStructurePreviewUnits(ctx, tx, fixed); err != nil {
		return err
	}
	admission, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	if err = validateStructurePreviewAdmission(fixed, admission); err != nil {
		return err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	// 固定引用只证明原事实归属，不能使已经关闭的原Doc准入重新开放；后台保存明确拒绝再收尾。
	if state.State == "CLOSED" {
		return domain.Violation(domain.PermissionDenied, "STRUCTURE_PREVIEW_ADMISSION_CLOSED")
	}
	if state.State != "OPEN" {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_ADMISSION_UNKNOWN")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	return validateMainControlProofs(mainControlContext{SessionExpires: fixed.SessionExpires, Access: fixed.Access}, proofs)
}

func (s *StructurePreviews) Source(ctx context.Context, input *pb.DraftPreviewRequest) (map[string]any, error) {
	if input == nil || input.Context == nil {
		return nil, domain.Violation(domain.InvalidRequest, "STRUCTURE_PREVIEW_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(input.Context, fixed.Context) || input.ExpectedBranchId != 0 || input.ExpectedLineageId != 0 || input.ExpectedStructureId != fixed.Structure {
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
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, domain.TextID(fixed.Manifest), fixed.Control)
	if err != nil {
		return nil, err
	}
	structure, err := repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(fixed.Document))
	if err != nil {
		return nil, err
	}
	boundaries, err := readStructureUnitBoundaries(ctx, tx, fixed.Document, structure.Entries)
	if err != nil {
		return nil, err
	}
	drafts := make([]any, 0, len(fixed.Units))
	for _, unit := range fixed.Units {
		baseline, err := repository.VersionContent(ctx, tx, unit.Workspace.BaseVersion, fixed.Document)
		if err != nil {
			return nil, err
		}
		if err = canonicalVersion(baseline, unit.Workspace.BaseVersion); err != nil {
			return nil, err
		}
		_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, unit.Workspace.Workspace)
		if err != nil {
			return nil, err
		}
		branchBaseline, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
		if err != nil {
			return nil, err
		}
		body, err := codec.Decode(branchBaseline)
		if err != nil {
			return nil, err
		}
		sources, err := codec.Decode(provenance)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, map[string]any{"unit_id": domain.TextID(unit.Unit), "chapter_id": domain.TextID(unit.Chapter), "branch_id": domain.TextID(unit.Workspace.Workspace), "workspace_epoch": domain.TextID(unit.Workspace.Epoch), "room_id": domain.TextID(unit.Room),
			"lineage": map[string]any{"lineage_id": domain.TextID(unit.Workspace.Lineage), "base_manifest_id": domain.TextID(unit.Workspace.BaseManifest), "base_version_id": domain.TextID(unit.Workspace.BaseVersion), "body": body, "provenance": sources}})
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": domain.TextID(fixed.Manifest), "structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style), "control_revision": domain.TextID(fixed.Control), "workspace_epoch": domain.TextID(epoch), "structure": structure, "changes": fixed.Intent.Input.Changes, "units": units, "drafts": drafts, "unit_boundaries": boundaries}, nil
}

// structure_previews.go：保留原实现，按业务合并文件。

type StructurePreviewClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	FixedStructure(context.Context, *pb.DraftPreviewRequest) ([]byte, error)
}

type StructurePreviews struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    StructurePreviewClient
	validator *codec.Validator
	instance  string
}

type structurePreviewUnit struct {
	Unit, Chapter, Room uint64
	Workspace           domain.EditWorkspace
}

// 原操作包含真实单元/支线代次及生成DTO；领取次数、HTTP超时都不能重写这些固定绑定。
type structurePreviewContext struct {
	Actor, Document                     uint64
	SessionExpires                      int64
	Intent                              domain.StructurePreviewIntent
	Access                              DocumentAccess
	Context                             *pb.OperationContext
	Manifest, Structure, Style, Control uint64
	Units                               []structurePreviewUnit
}

func NewStructurePreviews(db *sql.DB, identity *Identity, access *Access, client StructurePreviewClient, validator *codec.Validator, instance string) *StructurePreviews {
	return &StructurePreviews{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}
````

UTF-8/LF全文SHA-256：`11207121eb0504d1f83c653333b0adca712d60440be379cf7924d4cff78d7033`。[参考实现](../../../../services/document/internal/logic/document/structure_previews_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
