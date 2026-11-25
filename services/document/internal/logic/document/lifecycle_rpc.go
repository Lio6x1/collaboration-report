package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"google.golang.org/protobuf/proto"
)

type LifecycleBindingAuthority interface {
	Binding(context.Context, string) (platform.AccountClosureBinding, error)
}

type LifecycleReservation interface {
	Prepare(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementResult, error)
}

type LifecycleRetention interface {
	Plan(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error)
	ApplyRelations(context.Context, domain.AccountRetirementBinding) error
	Complete(context.Context, domain.AccountRetirementBinding) error
}

type LifecyclePlans struct {
	db          *sql.DB
	authority   LifecycleBindingAuthority
	reservation LifecycleReservation
	retention   LifecycleRetention
}

func NewLifecyclePlans(db *sql.DB, authority LifecycleBindingAuthority, reservation LifecycleReservation, retention LifecycleRetention) *LifecyclePlans {
	return &LifecyclePlans{db: db, authority: authority, reservation: reservation, retention: retention}
}

// 协议代次由首次Reserve固定，独立于身份epoch、lifecycle业务推进和后台租约代次。
// 两个推进请求保存原完整proto及输入阶段的完整计划，回复丢失后允许原摘要重试，
// 不能因为Prepare已扩充Retention就把原输入摘要误判为冲突。
type lifecycleRPCStep struct {
	Request   *pb.LifecycleRequest `json:"request"`
	InputPlan []byte               `json:"input_plan"`
}

type lifecycleRPCRegistration struct {
	Binding domain.AccountRetirementBinding `json:"binding"`
	Reserve *pb.LifecycleRequest            `json:"reserve"`
	Prepare *lifecycleRPCStep               `json:"prepare,omitempty"`
	Apply   *lifecycleRPCStep               `json:"apply,omitempty"`
}

func lifecycleRPCConflict(code string) error { return domain.Violation(domain.StateConflict, code) }

func validateLifecycleRPCRequest(request *pb.LifecycleRequest, phase string) error {
	if request == nil || !domain.ValidOperationID(request.OperationId) || request.UserId == 0 || request.Generation == 0 {
		return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_REQUEST_INVALID")
	}
	var previous uint64
	for _, organization := range request.ClosingOrganizationIds {
		if organization <= previous {
			return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_ORGANIZATIONS_INVALID")
		}
		previous = organization
	}
	if (phase == "RESERVE" && len(request.FixedPlanDigest) != 0) || (phase != "RESERVE" && len(request.FixedPlanDigest) != sha256.Size) {
		return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_DIGEST_INVALID")
	}
	return nil
}

func bindingMatchesLifecycleRequest(binding domain.AccountRetirementBinding, request *pb.LifecycleRequest) bool {
	return binding.Valid() && binding.Operation == request.OperationId && binding.User == request.UserId &&
		slices.Equal(binding.ClosingOrganizations, request.ClosingOrganizationIds)
}

func decodeLifecycleRPCRegistration(raw []byte, operation, state string) (lifecycleRPCRegistration, error) {
	var record lifecycleRPCRegistration
	if state != "SUCCEEDED" || json.Unmarshal(raw, &record) != nil || record.Reserve == nil ||
		validateLifecycleRPCRequest(record.Reserve, "RESERVE") != nil || operation != record.Reserve.OperationId ||
		!bindingMatchesLifecycleRequest(record.Binding, record.Reserve) {
		return record, errors.New("LIFECYCLE_RPC_BINDING_UNKNOWN")
	}
	for _, step := range []*lifecycleRPCStep{record.Prepare, record.Apply} {
		if step != nil && (step.Request == nil || validateLifecycleRPCRequest(step.Request, "STEP") != nil ||
			!bindingMatchesLifecycleRequest(record.Binding, step.Request) || step.Request.Generation != record.Reserve.Generation) {
			return record, errors.New("LIFECYCLE_RPC_BINDING_UNKNOWN")
		}
		if step != nil {
			digest := sha256.Sum256(step.InputPlan)
			plan, err := domain.DecodeAccountRetirementPlan(step.InputPlan)
			if err != nil || !plan.Binding.Same(record.Binding) || !bytes.Equal(digest[:], step.Request.FixedPlanDigest) {
				return record, errors.New("LIFECYCLE_RPC_INPUT_PLAN_UNKNOWN")
			}
		}
	}
	return record, nil
}

func readLifecycleRPCOriginal(ctx context.Context, q repository.Queryer, record lifecycleRPCRegistration) (repository.AccountRetirementRecord, domain.AccountRetirementPlan, error) {
	original, err := repository.AccountRetirementSnapshot(ctx, q, record.Binding.Operation)
	if err != nil {
		return original, domain.AccountRetirementPlan{}, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil || original.Kind != "ACCOUNT_CLOSE" || original.Actor != record.Binding.User ||
		original.Epoch != record.Binding.IdentityEpoch || !plan.Binding.Same(record.Binding) {
		return original, plan, lifecycleRPCConflict("LIFECYCLE_RPC_PLAN_BINDING_CHANGED")
	}
	switch original.State {
	case "FROZEN", "REJECTED":
	case "PLANNED", "CLEANING", "SUCCEEDED":
		if plan.Retention == nil {
			return original, plan, errors.New("LIFECYCLE_RPC_RETENTION_UNKNOWN")
		}
	default:
		return original, plan, errors.New("LIFECYCLE_RPC_PHASE_UNKNOWN")
	}
	return original, plan, nil
}

func lifecycleRPCCompatibleInput(step *lifecycleRPCStep, original repository.AccountRetirementRecord) error {
	input, err := domain.DecodeAccountRetirementPlan(step.InputPlan)
	if err != nil {
		return err
	}
	current, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil || !input.Binding.Same(current.Binding) || !slices.Equal(input.ReservedKnowledgeBases, current.ReservedKnowledgeBases) ||
		(input.Retention != nil && !bytes.Equal(step.InputPlan, original.Plan)) {
		return lifecycleRPCConflict("LIFECYCLE_RPC_FIXED_PLAN_CHANGED")
	}
	return nil
}

func (s *LifecyclePlans) register(ctx context.Context, request *pb.LifecycleRequest, phase string) (lifecycleRPCRegistration, error) {
	var empty lifecycleRPCRegistration
	if err := validateLifecycleRPCRequest(request, phase); err != nil {
		return empty, err
	}
	if s == nil || s.db == nil || s.authority == nil || s.reservation == nil || s.retention == nil {
		return empty, errors.New("LIFECYCLE_RPC_DEPENDENCIES_REQUIRED")
	}
	raw, operation, state, err := repository.ReadLifecycleRPCBinding(ctx, s.db, request.OperationId, false)
	if errors.Is(err, sql.ErrNoRows) {
		if phase != "RESERVE" {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_RESERVATION_REQUIRED")
		}
		// Account原持久绑定在SQL锁外读取；浏览器或RPC输入不能自行挑选identity_epoch。
		binding, err := s.authority.Binding(ctx, request.OperationId)
		if err != nil {
			return empty, err
		}
		empty.Binding = domain.AccountRetirementBinding(binding)
		if !bindingMatchesLifecycleRequest(empty.Binding, request) {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED")
		}
		empty.Reserve = proto.Clone(request).(*pb.LifecycleRequest)
		raw, err = json.Marshal(empty)
		if err != nil {
			return empty, err
		}
	} else if err != nil {
		return empty, err
	} else if _, err = decodeLifecycleRPCRegistration(raw, operation, state); err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	// 只持专用登记行锁，不持来源/lifecycle业务锁，更不在事务中跨服务调用。
	// 唯一键把两个首次Reserve串行化，后来的请求只能读原payload，不能覆盖原代次。
	if phase == "RESERVE" {
		if err = repository.InsertLifecycleRPCBinding(ctx, tx, request.OperationId, raw); err != nil {
			return empty, err
		}
	}
	raw, operation, state, err = repository.ReadLifecycleRPCBinding(ctx, tx, request.OperationId, true)
	if err != nil {
		return empty, err
	}
	record, err := decodeLifecycleRPCRegistration(raw, operation, state)
	if err != nil {
		return empty, err
	}
	if !bindingMatchesLifecycleRequest(record.Binding, request) || request.Generation != record.Reserve.Generation {
		return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
	}
	if phase == "RESERVE" {
		if !proto.Equal(request, record.Reserve) {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
		}
	} else {
		original, plan, err := readLifecycleRPCOriginal(ctx, tx, record)
		if err != nil {
			return empty, err
		}
		step := record.Prepare
		if phase == "APPLY" {
			step = record.Apply
			if plan.Retention == nil && original.State != "REJECTED" {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED")
			}
		}
		if step != nil {
			if !proto.Equal(step.Request, request) {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
			}
			if err = lifecycleRPCCompatibleInput(step, original); err != nil {
				return empty, err
			}
		} else {
			digest := sha256.Sum256(original.Plan)
			if !bytes.Equal(digest[:], request.FixedPlanDigest) {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_FIXED_PLAN_CHANGED")
			}
			step = &lifecycleRPCStep{Request: proto.Clone(request).(*pb.LifecycleRequest), InputPlan: slices.Clone(original.Plan)}
			if phase == "APPLY" {
				record.Apply = step
			} else {
				record.Prepare = step
			}
			raw, err = json.Marshal(record)
			if err != nil {
				return empty, err
			}
			if err = repository.UpdateLifecycleRPCBinding(ctx, tx, request.OperationId, raw); err != nil {
				return empty, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("LIFECYCLE_RPC_REGISTRATION_UNKNOWN")
	}
	return record, nil
}

func lifecycleRPCReply(record lifecycleRPCRegistration, original repository.AccountRetirementRecord) *pb.LifecyclePlan {
	digest := sha256.Sum256(original.Plan)
	return &pb.LifecyclePlan{OperationId: record.Binding.Operation, Generation: record.Reserve.Generation,
		PlanJsonUtf8: slices.Clone(original.Plan), PlanDigest: digest[:], Phase: original.State}
}

func (s *LifecyclePlans) Reserve(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "RESERVE")
	if err != nil {
		return nil, err
	}
	original, _, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if errors.Is(err, sql.ErrNoRows) {
		// 登记回复未知或进程退出后可能尚未实际预留；重查原Account决定，不能把
		// 已拒绝但取消消息尚未到达的旧登记提升为新的封门资格。
		binding, bindingErr := s.authority.Binding(ctx, request.OperationId)
		if bindingErr != nil {
			return nil, bindingErr
		}
		if !record.Binding.Same(domain.AccountRetirementBinding(binding)) {
			return nil, lifecycleRPCConflict("LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED")
		}
		result, preparationErr := s.reservation.Prepare(ctx, record.Binding)
		if preparationErr != nil {
			return nil, preparationErr
		}
		if result.State == "HANDOFF_REQUIRED" {
			return nil, lifecycleRPCConflict("LIFECYCLE_RPC_HANDOFF_REQUIRED")
		}
		original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
	}
	if err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}

func (s *LifecyclePlans) Prepare(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "PREPARE")
	if err != nil {
		return nil, err
	}
	original, plan, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	// 先读原固定计划/终态，Account后来去标识或身份变代不能破坏已登记同号重放。
	if plan.Retention == nil && original.State != "REJECTED" {
		if _, err = s.retention.Plan(ctx, record.Binding); err != nil {
			return nil, err
		}
		original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
		if err != nil {
			return nil, err
		}
	}
	if err = lifecycleRPCCompatibleInput(record.Prepare, original); err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}

func lifecycleRPCWaiting(err error) bool {
	var violation domain.RuleViolation
	return errors.As(err, &violation) && violation.Kind == domain.StateConflict &&
		(violation.Code == "ACCOUNT_RETENTION_DRAIN_PENDING" || violation.Code == "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
}

func (s *LifecyclePlans) Apply(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "APPLY")
	if err != nil {
		return nil, err
	}
	original, _, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	if err = lifecycleRPCCompatibleInput(record.Apply, original); err != nil {
		return nil, err
	}
	// 一次调用只推进一个既有阶段；不另起同步清理循环。CLEANING须待Account原
	// 持久协调器逐文档拿到清理回执，Complete也绝不替Account做最后去标识事务。
	switch original.State {
	case "PLANNED":
		err = s.retention.ApplyRelations(ctx, record.Binding)
	case "CLEANING":
		err = s.retention.Complete(ctx, record.Binding)
	case "SUCCEEDED", "REJECTED":
	default:
		return nil, lifecycleRPCConflict("LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED")
	}
	if err != nil && !lifecycleRPCWaiting(err) {
		return nil, err
	}
	original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	if err = lifecycleRPCCompatibleInput(record.Apply, original); err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}
