# packages/platform/security.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/029-security.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`AuthorizeRPC`、`RPCInterceptor`、`ErrorResponse`、`NewTraceID`、`WriteSafeError`、`WriteJSON`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package platform

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

//go:embed rpc-policy.json
var policyJSON []byte

var rpcPolicy = func() map[string][]string {
	var result map[string][]string
	if json.Unmarshal(policyJSON, &result) != nil {
		panic("invalid embedded RPC policy")
	}
	return result
}()

func AuthorizeRPC(ctx context.Context, method string) error {
	// 身份来自握手验证过的证书URI，禁止把浏览器头或证书CN当成服务能力。
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 || len(info.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	role := ""
	for _, uri := range info.State.PeerCertificates[0].URIs {
		for _, candidate := range []string{"account", "document", "search", "collaboration"} {
			if uri.String() == "spiffe://collaboration-report/"+candidate {
				if role != "" && role != candidate {
					return status.Error(codes.Unauthenticated, "AMBIGUOUS_SERVICE_IDENTITY")
				}
				role = candidate
			}
		}
	}
	if role == "" {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	deadline, ok := ctx.Deadline()
	// 调用方必须明确给出期限；超长期限也拒绝，避免后台资源被未知调用占住。
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
		return status.Error(codes.InvalidArgument, "FINITE_DEADLINE_REQUIRED")
	}
	for _, allowed := range rpcPolicy[method] {
		if role == allowed {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "SERVICE_METHOD_FORBIDDEN")
}

func RPCInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := AuthorizeRPC(ctx, info.FullMethod); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

type ErrorResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	TraceID     string `json:"trace_id"`
	RetryAction string `json:"retry_action"`
}

func NewTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "trace-unavailable"
	}
	return hex.EncodeToString(bytes)
}

func WriteSafeError(w http.ResponseWriter, trace string, err error) {
	// 原始驱动错误可能含SQL、凭据和私有路径，只交付稳定错误码与追踪号。
	code, message := "SERVICE_UNAVAILABLE", "服务暂时不可用，请稍后重试"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "DEPENDENCY_TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		code = "REQUEST_CANCELLED"
	}
	WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Code: code, Message: message, TraceID: trace, RetryAction: "RETRY_SAME_OPERATION"})
}

func WriteJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
````

UTF-8/LF全文SHA-256：`36de4ad113898e0e1d38a7e77cd7c9ec817f5f86a3f6c1acf57cb6bcac5958aa`。[参考实现](../../../../packages/platform/security.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
