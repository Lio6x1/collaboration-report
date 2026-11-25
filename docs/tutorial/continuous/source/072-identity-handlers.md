# services/account/internal/handler/account/identity_handlers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/072-identity-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`IdentityRenewal`、`NewIdentityRenewal`、`Renew`、`DocumentCollaborator`、`Fingerprint`、`MutationRequests`、`NewMutationRequests`、`Bind`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
)

// identity_renewal.go：保留原实现，按业务合并文件。

type IdentityRenewal struct{ logic *logic.Identity }

func NewIdentityRenewal(service *logic.Identity) *IdentityRenewal {
	return &IdentityRenewal{logic: service}
}

func (h *IdentityRenewal) Renew(w http.ResponseWriter, r *http.Request) {
	// 没有浏览器入口或Cookie鉴权；只接收实际document证书，正文不包含原始会话令牌。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	input := &pb.IdentityRevalidation{}
	if err != nil || len(raw) > 8192 || protojson.Unmarshal(raw, input) != nil {
		httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		return
	}
	proof, err := h.logic.RenewOperationAdmission(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdentityBinding):
			httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		case errors.Is(err, domain.ErrIdentityDenied):
			httpx.Failure(w, 403, "IDENTITY_NOT_CURRENT")
		default:
			httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		}
		return
	}
	body, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(proof)
	if err != nil {
		httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

func (h *IdentityRenewal) DocumentCollaborator(w http.ResponseWriter, r *http.Request) {
	// 只信任CA核验后的Doc服务身份；浏览器Cookie或自报角色都不能查询目标姓名。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	input := &pb.IdentityRevalidation{}
	if err != nil || len(raw) > 8192 || protojson.Unmarshal(raw, input) != nil {
		httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		return
	}
	result, err := h.logic.DocumentCollaborator(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdentityBinding):
			httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		case errors.Is(err, domain.ErrIdentityDenied):
			httpx.Failure(w, 403, "IDENTITY_NOT_CURRENT")
		default:
			httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

// mutation.go：保留原实现，按业务合并文件。

type Fingerprint func(any) []byte

// MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。
type MutationRequests struct {
	authentication *Authentication
	fingerprint    Fingerprint
}

func NewMutationRequests(authentication *Authentication, fingerprint Fingerprint) *MutationRequests {
	return &MutationRequests{authentication: authentication, fingerprint: fingerprint}
}

func (h *MutationRequests) Bind(w http.ResponseWriter, r *http.Request, id string, input any) (domain.Mutation, bool) {
	if !domain.ValidOperationID(id) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return domain.Mutation{}, false
	}
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return domain.Mutation{}, false
	}
	// 路径和规范化后的输入仍使用原HMAC编码，已提交的幂等记录无需重写。
	fingerprint := h.fingerprint(struct {
		Path  string `json:"path"`
		Input any    `json:"input"`
	}{r.URL.Path, input})
	return domain.Mutation{
		OperationID: id,
		Path:        r.URL.Path,
		Fingerprint: fingerprint,
		Session:     session,
	}, true
}
````

UTF-8/LF全文SHA-256：`ee5dad8d0ab5e658f3e3fef0611cfac2eeb822d1eab72cb70464d3ff3a5f2b06`。[参考实现](../../../../services/account/internal/handler/account/identity_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
