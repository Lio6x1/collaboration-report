package document

import (
	domain "collaboration/document/internal/domain/document"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"
)

type CursorSigner interface{ Sign([]byte) []byte }
type Pagination struct{ signer CursorSigner }

func NewPagination(signer CursorSigner) *Pagination { return &Pagination{signer: signer} }
func (s *Pagination) Parse(limitText, encoded, binding string) (int, string, error) {
	limit := 20
	if limitText != "" {
		value, err := strconv.Atoi(limitText)
		if err != nil || value < 1 || value > 100 {
			return 0, "", domain.Violation(domain.InvalidRequest, "LIMIT_INVALID")
		}
		limit = value
	}
	if encoded == "" {
		return limit, "", nil
	}
	if len(encoded) > 2048 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 33 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	data, signature := raw[:len(raw)-32], raw[len(raw)-32:]
	if !hmac.Equal(signature, s.signer.Sign(data)) {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	var cursor domain.Cursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Binding != binding || cursor.Until <= time.Now().Unix() {
		return 0, "", domain.Violation(domain.StateConflict, "CURSOR_STALE")
	}
	return limit, cursor.Last, nil
}
func (s *Pagination) Next(binding, last string) *string {
	raw, _ := json.Marshal(domain.Cursor{Binding: binding, Last: last, Until: time.Now().Add(5 * time.Minute).Unix()})
	encoded := base64.RawURLEncoding.EncodeToString(append(raw, s.signer.Sign(raw)...))
	return &encoded
}
