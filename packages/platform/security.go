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
