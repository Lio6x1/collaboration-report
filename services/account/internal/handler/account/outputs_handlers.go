// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// output.go：保留原实现，按业务合并文件。

type Outputs struct{ logic *logic.Outputs }

func NewOutputs(service *logic.Outputs) *Outputs { return &Outputs{logic: service} }

// MinimalReceipt只放行原操作的最小结果，不能借操作号直接返回分享链接或敏感投影。
func MinimalReceipt(raw []byte) bool {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || len(value) < 2 || len(value) > 4 || value["operation_id"] == nil || value["state"] == nil {
		return false
	}
	for key := range value {
		if key != "operation_id" && key != "state" && key != "poll_url" && key != "result" {
			return false
		}
	}
	var id, state string
	if json.Unmarshal(value["operation_id"], &id) != nil || !domain.ValidOperationID(id) || json.Unmarshal(value["state"], &state) != nil {
		return false
	}
	switch state {
	case "PREPARING", "FENCING", "SUCCEEDED", "REJECTED", "CANCELLED":
	default:
		return false
	}
	if result := value["result"]; result != nil {
		var fields map[string]string
		if json.Unmarshal(result, &fields) != nil || len(fields) != 1 || !domain.ValidEntityID(fields["entity_id"]) {
			return false
		}
	}
	return true
}

func anonymousOutput(path string) bool {
	return path == "/v1/account/login" || path == "/v1/account/signup" || path == "/v1/account/challenges" || path == "/v1/account/password-resets"
}

func (h *Outputs) Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") || anonymousOutput(r.URL.Path) || platform.HTTPCaller(r, "document") {
			next(w, r)
			return
		}
		binding := &domain.OutputBinding{Sources: map[string]string{}}
		r = r.WithContext(context.WithValue(r.Context(), outputKey{}, binding))
		capture := httpx.NewResponseCapture()
		next(capture, r)
		if capture.Status() >= 400 || MinimalReceipt(capture.Body()) || MinimalClosureReceipt(r.URL.Path, capture.Body()) {
			capture.CopyTo(w)
			return
		}
		// 短JSON完全构造后才登记入口实际handler，私有直连没有可控制输出资格。
		if r.Header.Get("X-Edge-Handler") == "" || binding.Invalid || len(binding.Sources) == 0 || capture.Size() > 2<<20 {
			httpx.Failure(w, 503, "OUTPUT_LEASE_REQUIRED")
			return
		}
		if err := h.logic.Register(r.Context(), r.Header.Get("X-Edge-Handler"), binding); err != nil {
			reason := "OUTPUT_REGISTER_UNKNOWN"
			if err.Error() == "SOURCE_CHANGED" || err.Error() == "SOURCE_INVALID" {
				reason = err.Error()
			}
			slog.Warn("account_output_pending", "reason", reason)
			httpx.Failure(w, 503, "ACCOUNT_OUTPUT_NOT_CURRENT")
			return
		}
		capture.CopyTo(w)
		_ = http.NewResponseController(w).Flush()
	}
}

// output_binding.go：保留原实现，按业务合并文件。

type outputKey struct{}

func CaptureSource(ctx context.Context, key string, epoch uint64) {
	binding, _ := ctx.Value(outputKey{}).(*domain.OutputBinding)
	if binding == nil {
		return
	}
	value := strconv.FormatUint(epoch, 10)
	if epoch == 0 || (binding.Sources[key] != "" && binding.Sources[key] != value) {
		binding.Invalid = true
		return
	}
	binding.Sources[key] = value
}

func CaptureSessionSources(ctx context.Context, session domain.Session, identity uint64) {
	credential, _ := strconv.ParseUint(session.CredentialEpoch, 10, 64)
	CaptureSource(ctx, "identity:"+session.UserID, identity)
	CaptureSource(ctx, "credential:"+session.UserID, credential)
	CaptureSource(ctx, "session:"+hex.EncodeToString(session.ID), session.Epoch)
}
