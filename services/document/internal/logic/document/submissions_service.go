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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
)

// submission_commit.go：保留原实现，按业务合并文件。

func (s *Submissions) commitSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, content *pb.FixedContent, currentProofs []*pb.IdentityAdmission) error {
	resources, err := domain.ValidateSubmissionCapture(s.validator, content, domain.CaptureExpectation{Request: fixed.Capture, Workspace: fixed.Workspace, Room: fixed.Room, LineageProvenance: fixed.LineageProvenance})
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
	if err = LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return err
	}
	target := fixed.Capture.Context.Target
	branch, err := repository.LockedActiveBranch(ctx, tx, target.UnitId, target.DocumentId)
	if err != nil || branch != target.BranchId {
		return domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
	}
	workspace, err := repository.LockedEditWorkspace(ctx, tx, branch, target.DocumentId, target.UnitId)
	if err != nil || workspace != fixed.Workspace {
		return domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, target.UnitId)
	if err != nil {
		return err
	}
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if guard.Submission != task.Operation || guard.Generation != fixed.Capture.GuardGeneration {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_GUARD_SUPERSEDED")
	}
	// RPC 与等锁可能跨过自然期限，最终业务事务仍须检查本次实际续接证明和原会话的期限。
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(currentProofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_RESUME_PROOF_INVALID")
	}
	for _, proof := range currentProofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_ADMISSION_RETRY_REQUIRED")
		}
	}
	fact, err := repository.SubmissionRow(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if fact.State != "PREPARING" && fact.State != "CAPTURED" {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_ALREADY_TERMINAL")
	}
	var pointer *domain.ReviewPointer
	if fixed.Intent.Mode == "RESUBMIT" {
		value, err := repository.LockedReviewPointer(ctx, tx, fixed.Intent.Request)
		if err != nil {
			return domain.Violation(domain.StateConflict, "RESUBMIT_POSITION_CHANGED")
		}
		pointer = &value
	}
	position := guard
	position.Submission = ""
	if err = domain.ValidateSubmissionPosition(fixed.Intent, position, pointer, target.DocumentId, target.UnitId); err != nil {
		return err
	}
	if fixed.Intent.Mode == "NEW" {
		baseline, err := repository.VersionContent(ctx, tx, workspace.BaseVersion, target.DocumentId)
		if err != nil {
			return err
		}
		if err = canonicalVersion(baseline, workspace.BaseVersion); err != nil {
			return err
		}
		if baseline.UnitId != target.UnitId {
			return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_BASE_UNKNOWN")
		}
		// MAIN独立字体不属于可送审的支线变化；与同一真实基线的BRANCH投影比较，不能虚构NEW。
		projected, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
		if err != nil {
			return err
		}
		baseHash := sha256.Sum256(projected)
		if bytes.Equal(baseHash[:], content.BodyHash) {
			return domain.Violation(domain.StateConflict, "SUBMISSION_NO_CHANGES")
		}
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	for _, id := range resources {
		state, err := repository.LockedDocumentResource(ctx, tx, id, target.DocumentId, space)
		if err != nil {
			return err
		}
		if state != "READY" {
			return domain.Violation(domain.StateConflict, "SUBMISSION_RESOURCE_NOT_READY")
		}
	}
	version, err := repository.InsertSubmissionVersion(ctx, tx, content)
	if err != nil {
		return err
	}
	for _, id := range resources {
		if err = repository.InsertSubmissionResource(ctx, tx, id, target.DocumentId, version); err != nil {
			return err
		}
	}
	request := fixed.Intent.Request
	if fixed.Intent.Mode == "NEW" {
		request, err = repository.CreateReviewRequest(ctx, tx, target.DocumentId, target.UnitId, branch)
		if err != nil {
			return err
		}
	}
	round, err := repository.CreateSubmissionRound(ctx, tx, request, fixed.Actor, version, content, workspace)
	if err != nil {
		return err
	}
	// 轮次目录由 structure_id 固定，样式另外存不可变上下文，避免读取时偷换成文档当前样式。
	// 同时保留原可靠切点，后续接纳来源无需依赖原提交者仍在线或重新读取可变房间。
	roundContext, err := json.Marshal(map[string]any{"style_id": fixed.Style, "document_title": fixed.Access.Document.Title, "cut": json.RawMessage(content.CutJsonUtf8)})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_ROUND_CONTEXT", domain.TextID(round), task.Operation, roundContext, "SUCCEEDED"); err != nil {
		return err
	}
	result := map[string]any{"operation_id": task.Operation, "state": "SUCCEEDED", "result": map[string]string{"request_id": domain.TextID(request), "round_id": domain.TextID(round), "content_version_id": domain.TextID(version)}}
	if err = repository.CompleteSubmission(ctx, tx, task.Operation, fixed.Actor, target.UnitId, request, round, result); err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "CAPTURE_FINALIZE", task.Operation, task.Operation, task.Payload, "QUEUED"); err != nil {
		return err
	}
	notice, err := json.Marshal(map[string]string{"document_id": domain.TextID(target.DocumentId), "request_id": domain.TextID(request), "round_id": domain.TextID(round), "kind": "SUBMISSION"})
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "REVIEW_NOTIFICATION", task.Operation, task.Operation, notice, "QUEUED"); err != nil {
		return err
	}
	if err = repository.FinishSubmissionTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Submissions) rejectSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	guard, err := repository.LockedReviewGuard(ctx, tx, fixed.Capture.Context.Target.UnitId)
	if err != nil {
		return err
	}
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if guard.Submission != task.Operation || guard.Generation != fixed.Capture.GuardGeneration {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_GUARD_SUPERSEDED")
	}
	result := map[string]any{"operation_id": task.Operation, "state": "REJECTED", "error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}}
	if err = repository.RejectSubmission(ctx, tx, task, fixed.Capture.Context.Target.UnitId, result, code); err != nil {
		return err
	}
	return tx.Commit()
}

// submission_prepare.go：保留原实现，按业务合并文件。

func (s *Submissions) Prepare(ctx context.Context, actor Actor, document, unit uint64, intent domain.SubmissionIntent, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "SUBMISSION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	present := s.submissionPresentation(actor, access, document, unit, intent.Operation)
	// 原号结果先读原回执，不能因后来的工作区变化改成一次新提交。
	if _, err = repository.SubmissionRow(ctx, s.db, intent.Operation); err == nil {
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	workspace, err := repository.SubmissionWorkspace(ctx, s.db, intent.Branch, document, unit)
	if err != nil {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
	}
	target := &pb.Target{DocumentId: document, UnitId: unit, BranchId: intent.Branch, StructureId: intent.Structure, WorkspaceEpoch: workspace.Epoch, Purpose: "SUBMISSION"}
	original := access.Proofs
	access.Proofs = make([]*pb.IdentityAdmission, 0, len(original))
	for _, proof := range original {
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
	fingerprint, err := domain.OperationFingerprint(path, json.RawMessage(intent.Canonical))
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.CurrentStructureId != domain.TextID(intent.Structure) {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		branch, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil || branch != intent.Branch {
			return nil, domain.Violation(domain.StateConflict, "BRANCH_CHANGED")
		}
		locked, err := repository.LockedEditWorkspace(ctx, tx, branch, document, unit)
		if err != nil || locked != workspace {
			return nil, domain.Violation(domain.StateConflict, "LINEAGE_CHANGED")
		}
		guard, err := repository.LockedReviewGuard(ctx, tx, unit)
		if err != nil {
			return nil, err
		}
		var pointer *domain.ReviewPointer
		if intent.Mode == "RESUBMIT" {
			value, err := repository.LockedReviewPointer(ctx, tx, intent.Request)
			if err != nil {
				return nil, domain.Violation(domain.StateConflict, "RESUBMIT_POSITION_CHANGED")
			}
			pointer = &value
		}
		if err := domain.ValidateSubmissionPosition(intent, guard, pointer, document, unit); err != nil {
			return nil, err
		}
		_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, branch)
		if err != nil {
			return nil, err
		}
		room, err := collaborationRoom(ctx, tx, "BRANCH", branch, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		admission := platform.NewTraceID()
		if err := InsertAdmission(ctx, tx, actor, access.Sources, document, "SUBMISSION", admission); err != nil {
			return nil, err
		}
		targetJSON, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err := repository.ConfigureSubmissionAdmission(ctx, tx, admission, unit, intent.Operation, targetJSON, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		capture := &pb.CaptureSubmissionRequest{Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}, FlushReceiptId: intent.Flush, GuardGeneration: guard.Generation + 1, LineageRef: domain.TextID(workspace.Lineage)}
		fixed := submissionContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Capture: capture, Workspace: workspace, Room: room, LineageProvenance: provenance, Style: current.CurrentStyleId}
		raw, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err := repository.RegisterSubmission(ctx, tx, actor.ID, document, unit, intent, workspace, capture.GuardGeneration, raw); err != nil {
			return nil, err
		}
		return map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}, nil
	}
	return actor, run, present, nil
}

func (s *Submissions) submissionPresentation(actor Actor, access DocumentAccess, document, unit uint64, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		if !access.Permissions.Edit[unit] {
			return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
		}
		fact, err := repository.SubmissionRow(ctx, tx, operation)
		if err != nil || fact.Actor != actor.ID || fact.Document != document || fact.Unit != unit {
			return nil, nil, domain.Violation(domain.EntityNotFound, "SUBMISSION_NOT_FOUND")
		}
		result, err := repository.SubmissionResult(ctx, tx, operation)
		if err != nil {
			return nil, nil, err
		}
		if !result.Expires.After(time.Now()) {
			return nil, nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
		}
		if err := json.Unmarshal(result.Result, &value); err != nil {
			return nil, nil, err
		}
		return value, access.Sources, nil
	}
}

// submission_readiness.go：保留原实现，按业务合并文件。

func (s *Submissions) Ready(ctx context.Context) error {
	raw, err := repository.PendingSubmissionContexts(ctx, s.db)
	if err != nil {
		return errors.New("SUBMISSION_RECOVERY_UNAVAILABLE")
	}
	var facts []struct {
		Kind                          string          `json:"kind"`
		Operation                     string          `json:"operation"`
		Payload                       json.RawMessage `json:"payload"`
		Actor, Document, Unit, Branch uint64
		GuardGeneration               uint64  `json:"guard_generation"`
		State                         string  `json:"state"`
		ReceiptState                  string  `json:"receipt_state"`
		HeldOperation                 *string `json:"held_operation"`
		HeldGeneration                uint64  `json:"held_generation"`
		Round                         uint64  `json:"round"`
	}
	if json.Unmarshal(raw, &facts) != nil {
		return errors.New("SUBMISSION_RECOVERY_INVALID")
	}
	for _, fact := range facts {
		if fact.Actor == 0 || fact.Document == 0 || fact.Unit == 0 || fact.Branch == 0 || fact.Operation == "" {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		if fact.Kind == "REVIEW_NOTIFICATION" {
			// 通知意图是已提交轮次的后续投递；Task 9 接管投递，存在合法积压不冻结整个文档服务。
			if fact.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Round == 0 {
				return errors.New("REVIEW_NOTIFICATION_FACT_UNKNOWN")
			}
			continue
		}
		var fixed submissionContext
		if json.Unmarshal(fact.Payload, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil || fixed.Capture.Context.Target == nil {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		binding := fixed.Capture.Context
		target := binding.Target
		if binding.OperationId != fact.Operation || binding.ActorId != fact.Actor || target.DocumentId != fact.Document || target.UnitId != fact.Unit || target.BranchId != fact.Branch || fixed.Capture.GuardGeneration != fact.GuardGeneration {
			return errors.New("SUBMISSION_RECOVERY_BINDING_UNKNOWN")
		}
		if fact.Kind == "SUBMISSION_RECOVER" {
			if fact.ReceiptState != "PREPARING" || (fact.State != "PREPARING" && fact.State != "CAPTURED") || fact.HeldOperation == nil || *fact.HeldOperation != fact.Operation || fact.HeldGeneration != fact.GuardGeneration {
				return errors.New("SUBMISSION_GUARD_RECOVERY_REQUIRED")
			}
		} else if fact.State != "SUCCEEDED" || fact.ReceiptState != "SUCCEEDED" || fact.Round == 0 {
			return errors.New("CAPTURE_FINALIZE_FACT_UNKNOWN")
		}
	}
	return nil
}

// submission_recovery.go：保留原实现，按业务合并文件。

func (s *Submissions) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimSubmissionTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if tx.Commit() != nil {
		return
	}
	var fixed submissionContext
	if json.Unmarshal(task.Payload, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil || fixed.Capture.Context.Target == nil || len(fixed.Access.Proofs) == 0 {
		s.retrySubmission(parent, task, domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_CONTEXT_INVALID"))
		return
	}
	if task.Kind == "CAPTURE_FINALIZE" {
		err = s.finalizeSubmission(ctx, task, fixed)
	} else {
		currentProofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
		// 证明续接发生在 SQL 锁外；只接受原会话、原操作、原来源，不重新选择现在的权限。
		for _, original := range fixed.Access.Proofs {
			var current *pb.IdentityAdmission
			if current, err = s.identity.Resume(ctx, original); err != nil {
				break
			}
			currentProofs = append(currentProofs, current)
		}
		if err == nil && fixed.SessionExpires <= time.Now().UnixMilli() {
			err = domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
		}
		var reply *pb.CaptureReply
		if err == nil {
			reply, err = s.captures.Capture(ctx, fixed.Capture)
		}
		if err == nil {
			if reply == nil || reply.State != pb.CaptureReply_FIXED {
				err = domain.Violation(domain.TemporarilyUnavailable, "CAPTURE_RESULT_UNKNOWN")
			} else {
				err = s.commitSubmission(ctx, task, fixed, reply.Content, currentProofs)
			}
		}
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || (violation.Kind == domain.StateConflict && violation.Code != "SCOPE_TRANSITION_PENDING")) {
			err = s.rejectSubmission(ctx, task, fixed, violation.Code)
		}
	}
	if err != nil {
		s.retrySubmission(parent, task, err)
	}
}

func (s *Submissions) retrySubmission(parent context.Context, task domain.SubmissionTask, cause error) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	_ = repository.RetrySubmissionTask(ctx, s.db, task, submissionRecoveryCode(cause))
}

func submissionRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("SUBMISSION_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "SUBMISSION_RPC_" + rpc.Code().String()
	}
	return "SUBMISSION_RECOVERY_UNKNOWN"
}

func (s *Submissions) finalizeSubmission(ctx context.Context, task domain.SubmissionTask, fixed submissionContext) error {
	// 轮次已提交。收尾是系统事实核对，不再要求当事人的浏览器或旧短期证明保持在线。
	query := &pb.OperationQuery{OperationId: task.Operation, Target: fixed.Capture.Context.Target, RequestFingerprint: fixed.Capture.Context.RequestFingerprint}
	authority, err := s.OperationResult(ctx, query)
	if err != nil {
		return err
	}
	if authority.State != pb.OperationResult_SUCCEEDED {
		return domain.Violation(domain.TemporarilyUnavailable, "SUBMISSION_RESULT_UNKNOWN")
	}
	result, err := s.captures.Finalize(ctx, &pb.FinalizeCaptureRequest{Context: fixed.Capture.Context, CaptureId: task.Operation, AuthoritativeResult: authority})
	if err != nil {
		return err
	}
	if result == nil || result.State != pb.OperationResult_SUCCEEDED || result.OperationId != task.Operation {
		return domain.Violation(domain.TemporarilyUnavailable, "CAPTURE_FINALIZE_UNKNOWN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.ValidateSubmissionTask(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishSubmissionTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

// submission_result.go：保留原实现，按业务合并文件。

func (s *Submissions) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	raw, err := repository.SubmissionPayload(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	var fixed submissionContext
	if json.Unmarshal(raw, &fixed) != nil || fixed.Capture == nil || fixed.Capture.Context == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_BINDING_UNKNOWN")
	}
	if !proto.Equal(query.Target, fixed.Capture.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Capture.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.SubmissionResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	result.FixedResultJsonUtf8 = fact.Result
	digest := sha256.Sum256(fact.Result)
	result.ResultDigest = digest[:]
	result.TerminalRevision = fact.Generation
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
	} else {
		result.State = pb.OperationResult_REJECTED
	}
	var body struct {
		Result map[string]string `json:"result"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &body) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_RESULT_UNKNOWN")
	}
	for _, key := range []string{"request_id", "round_id", "content_version_id"} {
		if text := body.Result[key]; text != "" {
			id, err := domain.CanonicalID(text)
			if err != nil {
				return nil, err
			}
			result.ResultIds = append(result.ResultIds, id)
		}
	}
	result.BusinessCode = body.Error.Code
	return result, nil
}

func (s *Submissions) Lookup(ctx context.Context, id string) (domain.SubmissionFact, error) {
	return repository.UserSubmissionRow(ctx, s.db, id)
}

func (s *Submissions) Progress(ctx context.Context, actor Actor, fact domain.SubmissionFact) ([]*pb.SourceEpoch, ReadBody, error) {
	if fact.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "SUBMISSION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fact.Document, "SUBMISSION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Edit[fact.Unit] {
		return nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		result, err := repository.SubmissionResult(ctx, tx, fact.Operation)
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
		return value, nil
	}
	return access.Sources, run, nil
}

// submissions.go：保留原实现，按业务合并文件。

type SubmissionCaptureClient interface {
	Capture(context.Context, *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error)
	Finalize(context.Context, *pb.FinalizeCaptureRequest) (*pb.OperationResult, error)
}

type Submissions struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	captures  SubmissionCaptureClient
	validator *codec.Validator
	instance  string
}

// 持久上下文只含已核验事实；浏览器 token 不进入 JSON，也不能供后台重新选择权限。
type submissionContext struct {
	Actor             uint64                       `json:"actor"`
	SessionExpires    int64                        `json:"session_expires"`
	Intent            domain.SubmissionIntent      `json:"intent"`
	Access            DocumentAccess               `json:"access"`
	Capture           *pb.CaptureSubmissionRequest `json:"capture"`
	Workspace         domain.EditWorkspace         `json:"workspace"`
	Room              uint64                       `json:"room"`
	LineageProvenance []byte                       `json:"lineage_provenance"`
	Style             string                       `json:"style"`
}

func NewSubmissions(db *sql.DB, identity *Identity, access *Access, captures SubmissionCaptureClient, validator *codec.Validator, instance string) *Submissions {
	return &Submissions{db: db, identity: identity, access: access, captures: captures, validator: validator, instance: instance}
}
