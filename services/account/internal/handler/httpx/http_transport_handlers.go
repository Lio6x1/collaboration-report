// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package httpx

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"reflect"
	"time"
)

// cookie.go：保留原实现，按业务合并文件。

const (
	SessionCookie  = "__Host-collabSession"
	CSRFCookie     = "__Host-collabCSRF"
	RecoveryCookie = "__Host-collabRecovery"
)

func SetCookie(w http.ResponseWriter, name, value string, expires time.Time, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Expires: expires, Secure: true, HttpOnly: httpOnly, SameSite: http.SameSiteStrictMode})
}

func ClearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", Secure: true, HttpOnly: name != CSRFCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func CookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func ValidCSRF(r *http.Request, session domain.Session) bool {
	hash, err := domain.TokenHash(r.Header.Get("X-CSRF-Token"))
	return err == nil && subtle.ConstantTimeCompare(hash, session.CSRF) == 1
}

func Origin(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Header.Get("Origin") != expected {
		Failure(w, 403, "ORIGIN_FORBIDDEN")
		return false
	}
	return true
}

// error.go：保留原实现，按业务合并文件。

func WriteError(w http.ResponseWriter, err error) {
	// 通过错误链判断稳定业务类别，外层补充诊断上下文不会把限流/无效验证码误报为503。
	switch {
	case errors.Is(err, domain.ErrChallengeInvalid):
		Failure(w, http.StatusBadRequest, domain.ErrChallengeInvalid.Error())
		return
	case errors.Is(err, domain.ErrChallengeRateLimited):
		Failure(w, http.StatusTooManyRequests, domain.ErrChallengeRateLimited.Error())
		return
	}
	// 仅透传已定义的故障类别，SMTP/Redis的原始报错可能含主机或凭据，日志同样不记录原文。
	for _, known := range []error{domain.ErrChallengeStoreUnavailable, domain.ErrSMTPDeliveryFailed} {
		if errors.Is(err, known) {
			slog.Warn("account_request_unavailable", "code", known.Error())
			Failure(w, http.StatusServiceUnavailable, known.Error())
			return
		}
	}
	var rejected domain.RuleViolation
	if errors.As(err, &rejected) {
		status := map[domain.RuleFailure]int{
			domain.InvalidRequest:         400,
			domain.SessionRequired:        401,
			domain.PermissionDenied:       403,
			domain.EntityNotFound:         404,
			domain.StateConflict:          409,
			domain.TemporarilyUnavailable: 503,
		}[rejected.Kind]
		if status != 0 {
			Failure(w, status, rejected.Code)
			return
		}
	}
	platform.WriteSafeError(w, platform.NewTraceID(), err)
}

// path.go：保留原实现，按业务合并文件。

func PathID(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	id := pathvar.Vars(r)[key]
	if !domain.ValidEntityID(id) {
		Failure(w, 400, "ENTITY_ID_INVALID")
		return "", false
	}
	return id, true
}

// request.go：保留原实现，按业务合并文件。

func ReadJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if decoder.Decode(&raw) != nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	// 生成契约是required/nullability的唯一来源，缺失布尔不能误解成撤销请求。
	model := reflect.Indirect(reflect.ValueOf(value)).Type().Name()
	for key, nullable := range dto.RequiredFields[model] {
		field, exists := fields[key]
		if !exists || (!nullable && bytes.Equal(bytes.TrimSpace(field), []byte("null"))) {
			Failure(w, 400, "REQUEST_INVALID")
			return false
		}
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if strict.Decode(value) != nil {
		Failure(w, 400, "REQUEST_INVALID")
		return false
	}
	return true
}

// response.go：保留原实现，按业务合并文件。

func Failure(w http.ResponseWriter, status int, code string) {
	retry := "NONE"
	if status == 401 {
		retry = "RELOGIN"
	}
	platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: "请求未满足当前操作条件", TraceID: platform.NewTraceID(), RetryAction: retry})
}

// response_capture.go：保留原实现，按业务合并文件。

// ResponseCapture暂存短JSON，输出资格核验成功后才交给真实ResponseWriter。
type ResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func NewResponseCapture() *ResponseCapture { return &ResponseCapture{header: make(http.Header)} }

func (c *ResponseCapture) Header() http.Header             { return c.header }
func (c *ResponseCapture) WriteHeader(status int)          { c.status = status }
func (c *ResponseCapture) Write(value []byte) (int, error) { return c.body.Write(value) }
func (c *ResponseCapture) Body() []byte                    { return c.body.Bytes() }
func (c *ResponseCapture) Size() int                       { return c.body.Len() }

func (c *ResponseCapture) Status() int {
	if c.status == 0 {
		return http.StatusOK
	}
	return c.status
}

func (c *ResponseCapture) CopyTo(w http.ResponseWriter) {
	for key, values := range c.header {
		w.Header()[key] = values
	}
	w.WriteHeader(c.Status())
	_, _ = w.Write(c.body.Bytes())
}
