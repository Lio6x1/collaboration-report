package httpx

import (
	domain "collaboration/account/internal/domain/account"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

// 保留用户能采取行动的已知故障类别；未知驱动错误仍走安全兜底，不能从message猜测业务冲突。
func TestKnownAccountFailuresKeepTheirCodeWithoutLeakingDriverDetails(t *testing.T) {
	for _, sample := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrSMTPDeliveryFailed, 503, "SMTP_DELIVERY_FAILED"},
		{fmt.Errorf("challenge read: %w", domain.ErrChallengeInvalid), 400, "CHALLENGE_INVALID"},
		{fmt.Errorf("challenge issue: %w", domain.ErrChallengeRateLimited), 429, "CHALLENGE_RATE_LIMITED"},
		{fmt.Errorf("delivery: %w", domain.ErrChallengeStoreUnavailable), 503, "CHALLENGE_STORE_UNAVAILABLE"},
		{domain.RuleViolation{Kind: domain.StateConflict, Code: "EMAIL_ALREADY_REGISTERED"}, 409, "EMAIL_ALREADY_REGISTERED"},
		{errors.New("SELECT password_hash; SMTP_DELIVERY_FAILED private-credential"), 503, "SERVICE_UNAVAILABLE"},
	} {
		w := httptest.NewRecorder()
		WriteError(w, sample.err)
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != sample.status || body.Code != sample.code {
			t.Fatalf("wrong classification: %d %s", w.Code, body.Code)
		}
		if strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), "private-credential") {
			t.Fatal("private details leaked")
		}
	}
}
