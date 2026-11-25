# services/account/internal/server/rpc_server.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/080-rpc-server.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`FenceAccess`、`GetAuthorizationChange`、`authorizationResultError`、`Identity`、`NewIdentity`、`BindOperationResults`、`BindAuthorizationResults`、`AcquireIdentityAdmission`、`RevalidateIdentityAdmission`、`identityError`、`GetAccountOperation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	domain "collaboration/account/internal/domain/account"
	logic "collaboration/account/internal/logic/account"
	pb "collaboration/contracts/internalv1"
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authorization_result.go：保留原实现，按业务合并文件。

// 调用者最小权限仍由平台冻结策略限定为Doc mTLS身份；这里不建立新的会话或关闭操作。
func (s *Identity) FenceAccess(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error) {
	if s.authorizations == nil {
		return nil, authorizationResultError(nil)
	}
	result, err := s.authorizations.Fence(ctx, input)
	if err != nil {
		return nil, authorizationResultError(err)
	}
	return result, nil
}

func (s *Identity) GetAuthorizationChange(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error) {
	if s.authorizations == nil {
		return nil, authorizationResultError(nil)
	}
	result, err := s.authorizations.Get(ctx, input)
	if err != nil {
		return nil, authorizationResultError(err)
	}
	return result, nil
}

func authorizationResultError(err error) error {
	var rule domain.RuleViolation
	if errors.As(err, &rule) {
		code := codes.Unavailable
		if rule.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		}
		if rule.Kind == domain.EntityNotFound {
			code = codes.NotFound
		}
		return status.Error(code, rule.Code)
	}
	return status.Error(codes.Unavailable, "AUTHORIZATION_PROOF_UNAVAILABLE")
}

// identity.go：保留原实现，按业务合并文件。

// RPC只适配已装配用例；服务身份、冻结调用者策略和deadline继续由平台拦截器核验。
type Identity struct {
	pb.UnimplementedIdentityInternalServer
	logic          *logic.Identity
	operations     *logic.OperationResults
	authorizations *logic.AuthorizationResults
}

func NewIdentity(service *logic.Identity) *Identity { return &Identity{logic: service} }

// 与HTTP共享已装配的权限用例，平台装配完成后才Serve，不在查询时创建依赖。
func (s *Identity) BindOperationResults(operations *logic.OperationResults) {
	s.operations = operations
}

func (s *Identity) BindAuthorizationResults(authorizations *logic.AuthorizationResults) {
	s.authorizations = authorizations
}

func (s *Identity) AcquireIdentityAdmission(ctx context.Context, input *pb.IdentityRequest) (*pb.IdentityAdmission, error) {
	proof, err := s.logic.AcquireIdentityAdmission(ctx, input)
	return proof, identityError(err)
}

func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, input *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	proof, err := s.logic.RevalidateIdentityAdmission(ctx, input)
	return proof, identityError(err)
}

func identityError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrIdentityBinding):
		return status.Error(codes.InvalidArgument, "IDENTITY_BINDING_INVALID")
	case errors.Is(err, domain.ErrIdentityDenied):
		return status.Error(codes.PermissionDenied, "IDENTITY_NOT_CURRENT")
	default:
		return status.Error(codes.Unavailable, "IDENTITY_UNAVAILABLE")
	}
}

// operation_result.go：保留原实现，按业务合并文件。

func (s *Identity) GetAccountOperation(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error) {
	result, err := s.operations.Get(ctx, query)
	if err == nil {
		return result, nil
	}
	var rule domain.RuleViolation
	if errors.As(err, &rule) {
		code := codes.Unavailable
		if rule.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		} else if rule.Kind == domain.EntityNotFound {
			code = codes.NotFound
		}
		return nil, status.Error(code, rule.Code)
	}
	return nil, status.Error(codes.Unavailable, "ACCOUNT_OPERATION_RESULT_UNAVAILABLE")
}
````

UTF-8/LF全文SHA-256：`a42402e8866757a253a0488a7c4ffe26565ef917afb7a97681acbfd9fd241da4`。[参考实现](../../../../services/account/internal/server/rpc_server.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
