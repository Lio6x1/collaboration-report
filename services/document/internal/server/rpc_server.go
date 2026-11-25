// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// candidate_cleanup.go：保留原实现，按业务合并文件。

func (s *Document) BindCandidateCleanup(cleanup *logic.CandidateCleanup) { s.cleanup = cleanup }

func (s *Document) AuthorizeCandidateCleanup(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error) {
	permit, err := s.cleanup.Authorize(ctx, request)
	return permit, lifecycleRPCError(err)
}

// document.go：保留原实现，按业务合并文件。

// Document聚合已有RPC适配器，未实施的方法仍沿生成契约返回UNIMPLEMENTED。
type Document struct {
	pb.UnimplementedDocumentInternalServer
	fences     *Fence
	tickets    *EditSessions
	resources  *DraftResources
	operations *OperationResults
	public     *PublicSnapshot
	lifecycle  *logic.LifecyclePlans
	cleanup    *logic.CandidateCleanup
}

func NewDocument() *Document { return &Document{} }

// 平台先登记RPC再装配HTTP，且装配成功后才Serve；Bind仅在这一启动阶段调用一次。
// 这里不查询SQL或初始化业务，依赖由bootstrap统一创建。
func (s *Document) Bind(fences *Fence, tickets *EditSessions, resources *DraftResources, operations *OperationResults, public *PublicSnapshot) {
	s.fences = fences
	s.tickets = tickets
	s.resources = resources
	s.operations = operations
	s.public = public
}

func (s *Document) GetPublicSnapshot(ctx context.Context, in *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	return s.public.GetPublicSnapshot(ctx, in)
}

func (s *Document) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error) {
	return s.operations.GetOperationResult(ctx, in)
}

func (s *Document) FenceAccess(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.fences.FenceAccess(ctx, in)
}

func (s *Document) GetFenceResult(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.fences.GetFenceResult(ctx, in)
}

func (s *Document) RedeemCollaborationTicket(ctx context.Context, in *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	return s.tickets.RedeemCollaborationTicket(ctx, in)
}

func (s *Document) ProtectDraftResources(ctx context.Context, in *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	return s.resources.ProtectDraftResources(ctx, in)
}

func (s *Document) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	return s.resources.GetDraftResourceProtection(ctx, in)
}

func (s *Document) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	return s.resources.ConfirmDraftResources(ctx, in)
}

func (s *Document) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	return s.resources.ReleaseDraftResources(ctx, in)
}

// draft_resources.go：保留原实现，按业务合并文件。

type DraftResources struct{ usecase *logic.DraftResources }

func NewDraftResources(usecase *logic.DraftResources) *DraftResources {
	return &DraftResources{usecase: usecase}
}
func (s *DraftResources) ProtectDraftResources(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Protect(ctx, req)
	return result, ProtectionError(err)
}
func (s *DraftResources) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Get(ctx, in)
	return result, ProtectionError(err)
}
func (s *DraftResources) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Finalize(ctx, in, false)
	return result, ProtectionError(err)
}
func (s *DraftResources) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Finalize(ctx, in, true)
	return result, ProtectionError(err)
}
func ProtectionError(err error) error {
	var failure domain.ProtectionFailure
	if !errors.As(err, &failure) {
		return dependencyError(err)
	}
	code := codes.Unavailable
	switch failure.Kind {
	case domain.ProtectionInvalid:
		code = codes.InvalidArgument
	case domain.ProtectionForbidden:
		code = codes.PermissionDenied
	case domain.ProtectionMissing:
		code = codes.NotFound
	case domain.ProtectionConflict:
		code = codes.AlreadyExists
	case domain.ProtectionPrecondition:
		code = codes.FailedPrecondition
	}
	return status.Error(code, failure.Code)
}

// edit_sessions.go：保留原实现，按业务合并文件。

type EditSessions struct{ usecase *logic.EditSessions }

func NewEditSessions(usecase *logic.EditSessions) *EditSessions {
	return &EditSessions{usecase: usecase}
}
func (s *EditSessions) RedeemCollaborationTicket(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	if req == nil || !domain.ValidOperationID(req.ClientSessionId) || !domain.ValidOperationID(req.ConnectionId) {
		return nil, status.Error(codes.InvalidArgument, "CONNECTION_BINDING_INVALID")
	}
	result, err := s.usecase.Redeem(ctx, req)
	return result, EditingError(err)
}
func EditingError(err error) error {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		return dependencyError(err)
	}
	code := codes.Unavailable
	switch violation.Kind {
	case domain.InvalidRequest:
		code = codes.InvalidArgument
	case domain.PermissionDenied:
		code = codes.PermissionDenied
	case domain.EntityNotFound:
		code = codes.NotFound
	case domain.StateConflict:
		code = codes.FailedPrecondition
	}
	return status.Error(code, violation.Code)
}

// fence.go：保留原实现，按业务合并文件。

type Fence struct{ usecase *logic.Fences }

func NewFence(usecase *logic.Fences) *Fence { return &Fence{usecase: usecase} }
func (s *Fence) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	if !platform.ValidBootstrapFence(request) {
		return nil, status.Error(codes.InvalidArgument, "FENCE_BINDING_INVALID")
	}
	result, err := s.usecase.Close(ctx, request)
	return result, FenceError(err)
}
func (s *Fence) GetFenceResult(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.FenceAccess(ctx, request)
}
func FenceError(err error) error {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		return err
	}
	code := codes.Unavailable
	if violation.Kind == domain.StateConflict {
		code = codes.AlreadyExists
	}
	return status.Error(code, violation.Code)
}

// lifecycle.go：保留原实现，按业务合并文件。

// 平台在bootstrap完成后才Serve；这三个入口继续受既定account mTLS allowlist/deadline保护。
func (s *Document) BindLifecycle(plans *logic.LifecyclePlans) { s.lifecycle = plans }

func (s *Document) ReserveAdminRetirement(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Reserve(ctx, request)
	return plan, lifecycleRPCError(err)
}

func (s *Document) PrepareLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Prepare(ctx, request)
	return plan, lifecycleRPCError(err)
}

func (s *Document) ApplyLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Apply(ctx, request)
	return plan, lifecycleRPCError(err)
}

func lifecycleRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return status.Error(codes.NotFound, "LIFECYCLE_RPC_ORIGINAL_NOT_FOUND")
	}
	var rule domain.RuleViolation
	if !errors.As(err, &rule) {
		return status.Error(codes.Unavailable, "LIFECYCLE_RPC_UNAVAILABLE")
	}
	code := codes.Unavailable
	switch rule.Kind {
	case domain.InvalidRequest:
		code = codes.InvalidArgument
	case domain.PermissionDenied:
		code = codes.PermissionDenied
	case domain.EntityNotFound:
		code = codes.NotFound
	case domain.StateConflict:
		code = codes.AlreadyExists
		if rule.Code == "LIFECYCLE_RPC_HANDOFF_REQUIRED" || rule.Code == "LIFECYCLE_RPC_RESERVATION_REQUIRED" || rule.Code == "LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED" {
			code = codes.FailedPrecondition
		}
	}
	return status.Error(code, rule.Code)
}

// operation_results.go：保留原实现，按业务合并文件。

type OperationResults struct {
	submissions  *logic.Submissions
	merges       *logic.Merges
	controls     *logic.MainControls
	pulls        *logic.PullMain
	previews     *logic.StructurePreviews
	drafts       *logic.DraftControls
	reverse      *logic.ReverseChanges
	resolutions  *logic.Resolutions
	publications *logic.Publications
}

func NewOperationResults(submissions *logic.Submissions, merges *logic.Merges, controls *logic.MainControls, pulls *logic.PullMain, previews *logic.StructurePreviews, drafts *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions, publications *logic.Publications) *OperationResults {
	return &OperationResults{submissions: submissions, merges: merges, controls: controls, pulls: pulls, previews: previews, drafts: drafts, reverse: reverse, resolutions: resolutions, publications: publications}
}

func (s *OperationResults) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error) {
	result, err := s.submissions.OperationResult(ctx, in)
	if err == sql.ErrNoRows {
		result, err = s.merges.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.controls != nil {
		result, err = s.controls.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.pulls != nil {
		result, err = s.pulls.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.previews != nil {
		result, err = s.previews.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.drafts != nil {
		result, err = s.drafts.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.reverse != nil {
		result, err = s.reverse.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.resolutions != nil {
		result, err = s.resolutions.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.publications != nil {
		result, err = s.publications.OperationResult(ctx, in)
	}
	if err == nil {
		return result, nil
	}
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "OPERATION_NOT_FOUND")
	}
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		code := codes.Unavailable
		if violation.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		}
		if violation.Kind == domain.StateConflict {
			code = codes.AlreadyExists
		}
		return nil, status.Error(code, violation.Code)
	}
	return nil, status.Error(codes.Unavailable, "OPERATION_RESULT_UNKNOWN")
}

// public_snapshot.go：保留原实现，按业务合并文件。

type PublicSnapshot struct{ usecase *logic.PublicReading }

func NewPublicSnapshot(usecase *logic.PublicReading) *PublicSnapshot {
	return &PublicSnapshot{usecase: usecase}
}

// 调用方资格仍由冻结mTLS方法白名单核验；适配器不替Search伪造用户身份。
func (s *PublicSnapshot) GetPublicSnapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	value, err := s.usecase.Snapshot(ctx, input)
	return value, EditingError(err)
}
