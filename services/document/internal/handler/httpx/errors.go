package httpx

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-sql-driver/mysql"
)

// Reject兼容正在迁移的HTTP入口；新业务用例直接返回领域类别。
func Reject(status int, code string) error {
	kind := domain.TemporarilyUnavailable
	switch status {
	case 400:
		kind = domain.InvalidRequest
	case 401:
		kind = domain.SessionRequired
	case 403:
		kind = domain.PermissionDenied
	case 404:
		kind = domain.EntityNotFound
	case 409:
		kind = domain.StateConflict
	}
	return domain.Violation(kind, code)
}

func Fail(w http.ResponseWriter, status int, code string) {
	platform.WriteJSON(w, status, platform.ErrorResponse{Code: code, Message: "请求未满足当前操作条件", TraceID: platform.NewTraceID(), RetryAction: "NONE"})
}

func Failure(w http.ResponseWriter, err error) {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		// 公共响应不泄露依赖细节；日志只保留固定错误码或数据库编号，便于定位真实故障。
		code := "DEPENDENCY_FAILURE"
		if safeFailureCode.MatchString(err.Error()) {
			code = err.Error()
		}
		var databaseError *mysql.MySQLError
		if errors.As(err, &databaseError) {
			slog.Warn("document_request_failed", "code", code, "database_number", databaseError.Number)
		} else {
			slog.Warn("document_request_failed", "code", code)
		}
		platform.WriteSafeError(w, platform.NewTraceID(), err)
		return
	}
	status := 503
	switch violation.Kind {
	case domain.InvalidRequest:
		status = 400
	case domain.SessionRequired:
		status = 401
	case domain.PermissionDenied:
		status = 403
	case domain.EntityNotFound:
		status = 404
	case domain.StateConflict:
		status = 409
	}
	Fail(w, status, violation.Code)
}

var safeFailureCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
