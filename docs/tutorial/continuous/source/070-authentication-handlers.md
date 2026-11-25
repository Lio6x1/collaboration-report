# services/account/internal/handler/account/authentication_handlers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/070-authentication-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`SessionSourceCapture`、`Authentication`、`NewAuthentication`、`Login`、`Authenticate`、`Me`、`writeError`、`Challenges`、`NewChallenges`、`Issue`、`Signup`、`Credentials`、`NewCredentials`、`ResetPassword`、`ChangePassword`、`ChangeEmail`、`retry`、`writeCredential`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// authentication.go：保留原实现，按业务合并文件。

type SessionSourceCapture func(context.Context, domain.Session, uint64)

type Authentication struct {
	logic   *logic.Authentication
	origin  string
	capture SessionSourceCapture
}

func NewAuthentication(service *logic.Authentication, origin string, capture SessionSourceCapture) *Authentication {
	return &Authentication{logic: service, origin: origin, capture: capture}
}

func (h *Authentication) Login(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Login
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	result, err := h.logic.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	// logic已提交会话；只有此时才交付令牌。CSRF Cookie用于刷新后恢复请求头。
	httpx.SetCookie(w, httpx.SessionCookie, result.SessionToken, result.Expires, true)
	httpx.SetCookie(w, httpx.CSRFCookie, result.CSRFToken, result.Expires, false)
	platform.WriteJSON(w, 200, dto.LoginResult{UserId: result.UserID, DisplayName: result.DisplayName, CsrfToken: result.CSRFToken, ExpiresAt: result.Expires.Format(time.RFC3339)})
}

// Authenticate核验账号会话；组织角色仍由对应业务事务锁定并重检。
func (h *Authentication) Authenticate(w http.ResponseWriter, r *http.Request, mutation bool) (domain.Session, bool) {
	value, err := h.logic.Authenticate(r.Context(), httpx.CookieValue(r, httpx.SessionCookie))
	if err != nil {
		writeError(w, err)
		return value.Session, false
	}
	// 保留真实输出来源绑定；拆层后不能遗漏身份、凭据或会话的关闭代次。
	h.capture(r.Context(), value.Session, value.IdentityEpoch)
	if mutation {
		if !httpx.Origin(w, r, h.origin) {
			return value.Session, false
		}
		if !httpx.ValidCSRF(r, value.Session) {
			httpx.Failure(w, 403, "CSRF_INVALID")
			return value.Session, false
		}
	}
	return value.Session, true
}

func (h *Authentication) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := h.Authenticate(w, r, false)
	if !ok {
		return
	}
	user, err := h.logic.CurrentUser(r.Context(), session)
	if err != nil {
		writeError(w, err)
		return
	}
	platform.WriteJSON(w, 200, dto.User{Id: user.ID, Email: &user.Email, DisplayName: user.DisplayName, State: user.State, CredentialEpoch: user.CredentialEpoch, IdentityEpoch: user.IdentityEpoch})
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrLoginInput):
		httpx.Failure(w, 400, "LOGIN_INPUT_INVALID")
	case errors.Is(err, domain.ErrLoginLimited):
		httpx.Failure(w, 429, "LOGIN_RATE_LIMITED")
	case errors.Is(err, domain.ErrLoginInvalid):
		httpx.Failure(w, 401, "LOGIN_INVALID")
	case errors.Is(err, domain.ErrSessionInvalid):
		httpx.Failure(w, 401, "SESSION_INVALID")
	default:
		platform.WriteSafeError(w, platform.NewTraceID(), err)
	}
}

// challenge.go：保留原实现，按业务合并文件。

type Challenges struct {
	authentication *Authentication
	logic          *logic.Challenges
	signups        *logic.Signups
	secrets        *domain.Secrets
	origin         string
}

func NewChallenges(authentication *Authentication, service *logic.Challenges, signups *logic.Signups, secrets *domain.Secrets, origin string) *Challenges {
	return &Challenges{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}
}

func (h *Challenges) Issue(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.ChallengeCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !domain.ValidEmail(input.Email) || (input.Purpose != "SIGNUP" && input.Purpose != "RESET_PASSWORD" && input.Purpose != "CHANGE_EMAIL") {
		httpx.Failure(w, 400, "CHALLENGE_INPUT_INVALID")
		return
	}
	var session *domain.Session
	if input.Purpose == "CHANGE_EMAIL" {
		current, ok := h.authentication.Authenticate(w, r, true)
		if !ok {
			return
		}
		session = &current
	}
	issued, err := h.logic.Issue(r.Context(), input, session)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	// 成功仅表示挑战已受理/邮件服务器已确认DATA；头不含SMTP主机、凭据或邮箱存在性。
	if mode := h.logic.DeliveryMode(); mode != "" {
		w.Header().Set("X-Collaboration-Mail-Delivery", mode)
	}
	platform.WriteJSON(w, 200, dto.Challenge{ChallengeId: issued.ID, ExpiresAt: issued.Expires.Format(time.RFC3339), RetryAfterSeconds: 60})
}

func (h *Challenges) Signup(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.Signup
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidEmail(input.Email) || !domain.ValidPassword(input.Password) || len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 100 {
		httpx.Failure(w, 400, "SIGNUP_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	// 公开重取仅返回原事实；指纹含密码/验证码的HMAC，不能靠操作号读取他人结果。
	raw, found, err := h.signups.PublicResult(r.Context(), input.OperationId, "SIGNUP", fingerprint)
	if found {
		if err != nil {
			httpx.WriteError(w, err)
		} else {
			platform.WriteJSON(w, 200, raw)
		}
		return
	}
	raw, err = h.signups.Register(r.Context(), input, fingerprint)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, raw)
}

// credential.go：保留原实现，按业务合并文件。

type Credentials struct {
	authentication *Authentication
	logic          *logic.Credentials
	signups        *logic.Signups
	secrets        *domain.Secrets
	origin         string
}

func NewCredentials(authentication *Authentication, service *logic.Credentials, signups *logic.Signups, secrets *domain.Secrets, origin string) *Credentials {
	return &Credentials{authentication: authentication, logic: service, signups: signups, secrets: secrets, origin: origin}
}

func (h *Credentials) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.ResetPassword
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidPassword(input.NewPassword) {
		httpx.Failure(w, 400, "RESET_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	raw, found, err := h.signups.PublicResult(r.Context(), input.OperationId, "RESET_PASSWORD", fingerprint)
	if found {
		if err != nil {
			httpx.WriteError(w, err)
		} else {
			platform.WriteJSON(w, 200, raw)
		}
		return
	}
	state, err := h.logic.ResetPassword(r.Context(), input, fingerprint)
	writeCredential(w, 200, input.OperationId, state, err)
}

func (h *Credentials) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.PasswordChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidPassword(input.NewPassword) || !domain.ValidPassword(input.CurrentPassword) {
		httpx.Failure(w, 400, "PASSWORD_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
		return
	}
	capture := httpx.NewResponseCapture()
	session, ok := h.authentication.Authenticate(capture, r, true)
	if !ok {
		// 等待另一个同号请求提交后，失效会话仍能续取原结果；暂存身份拒绝不能抢先输出。
		if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
			return
		}
		capture.CopyTo(w)
		return
	}
	state, err := h.logic.ChangePassword(r.Context(), session, input, fingerprint)
	var rejected domain.RuleViolation
	if errors.As(err, &rejected) && rejected.Code == "CURRENT_PASSWORD_INVALID" {
		if h.retry(w, r, input.OperationId, "CHANGE_PASSWORD", fingerprint) {
			return
		}
	}
	writeCredential(w, 202, input.OperationId, state, err)
}

func (h *Credentials) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	if !httpx.Origin(w, r, h.origin) {
		return
	}
	var input dto.EmailChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	input.NewEmail = strings.ToLower(strings.TrimSpace(input.NewEmail))
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidEmail(input.NewEmail) {
		httpx.Failure(w, 400, "EMAIL_INPUT_INVALID")
		return
	}
	fingerprint := h.secrets.Fingerprint(input)
	if h.retry(w, r, input.OperationId, "CHANGE_EMAIL", fingerprint) {
		return
	}
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return
	}
	state, err := h.logic.ChangeEmail(r.Context(), session, input, fingerprint)
	writeCredential(w, 202, input.OperationId, state, err)
}

func (h *Credentials) retry(w http.ResponseWriter, r *http.Request, id, kind string, fingerprint []byte) bool {
	value, found, err := h.logic.Retry(r.Context(), id, kind, fingerprint, httpx.CookieValue(r, httpx.SessionCookie))
	if !found {
		return false
	}
	if err != nil {
		httpx.WriteError(w, err)
		return true
	}
	if !httpx.ValidCSRF(r, value.Session) {
		httpx.Failure(w, 403, "CSRF_INVALID")
		return true
	}
	platform.WriteJSON(w, 202, ProgressDTO(id, value.State))
	return true
}

func writeCredential(w http.ResponseWriter, status int, id, state string, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, status, ProgressDTO(id, state))
}
````

UTF-8/LF全文SHA-256：`3a87cae0e70a0ced38457967ea49e03fcb16b5d9d29766cbf8a256324b62449c`。[参考实现](../../../../services/account/internal/handler/account/authentication_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
