package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	dto "collaboration/contracts/httpdto"
)

func TestMissingOrNullBooleanCannotBecomeARevoke(t *testing.T) {
	for _, raw := range []string{`{"operation_id":"grant-0123456789abcdef","expected_revision":"1","user_id":"1"}`, `{"operation_id":"grant-0123456789abcdef","expected_revision":"1","user_id":"1","allowed":null}`} {
		r := httptest.NewRequest("POST", "/v1/organizations/1/creation-grants", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var input dto.CreationGrant
		if ReadJSON(w, r, &input) {
			t.Fatal("missing/null allowed became false revoke")
		}
		if w.Code != 400 {
			t.Fatalf("status %d", w.Code)
		}
	}
}
