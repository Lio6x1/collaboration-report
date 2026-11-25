# services/document/internal/handler/httpx/errors.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/267-errors.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Reject`、`Fail`、`Failure`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`684ecd68505a5f4ce4820cca42f4b0158d62c30c54d2e86991848160a0da6adc`。[参考实现](../../../../services/document/internal/handler/httpx/errors.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
