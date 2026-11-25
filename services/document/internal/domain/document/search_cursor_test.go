package document

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func searchCursorFixture(t *testing.T) (*SearchCursorCodec, SearchPageCursor, time.Time) {
	t.Helper()
	codec, err := NewSearchCursorCodec(bytes.Repeat([]byte{0x5a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 12, 34, 56, 789000000, time.UTC)
	cursor := SearchPageCursor{
		Anchor: []byte(`{"sort":[1.234567890123456789,"18446744073709551615",18446744073709551615],"document":"18446744073709551615"}`),
		Prefix: 99, Binding: [32]byte{1, 2, 3}, Until: now.Add(2 * time.Minute).UnixMilli(),
	}
	return codec, cursor, now
}

func assertSearchCursorFailure(t *testing.T, err error, kind FailureKind, code string) {
	t.Helper()
	var violation RuleViolation
	if !errors.As(err, &violation) || violation.Kind != kind || violation.Code != code || err.Error() != code {
		t.Fatalf("want fixed domain failure %v/%s, got %v", kind, code, err)
	}
}

func openSearchCursorFailure(t *testing.T, codec *SearchCursorCodec, token string, binding [32]byte, now time.Time, kind FailureKind, code string) {
	t.Helper()
	cursor, err := codec.Open(token, binding, now)
	assertSearchCursorFailure(t, err, kind, code)
	if cursor.Anchor != nil || cursor.Prefix != 0 || cursor.Binding != [32]byte{} || cursor.Until != 0 {
		t.Fatal("rejected cursor exposed partially decoded fields")
	}
}

func TestSearchCursorRoundTripOpaqueMaximumAnchor(t *testing.T) {
	codec, cursor, now := searchCursorFixture(t)
	for _, size := range []int{1, 768} {
		for _, prefix := range []uint8{0, 99} {
			original := cursor
			original.Anchor = bytes.Repeat([]byte{0xff}, size)
			copy(original.Anchor, cursor.Anchor)
			original.Prefix = prefix
			original.Until = math.MaxInt64
			encoded, err := codec.Seal(original)
			if err != nil || len(encoded) > 2048 {
				t.Fatalf("maximum cursor not encodable: size=%d prefix=%d, %v", size, prefix, err)
			}
			opened, err := codec.Open(encoded, original.Binding, now)
			if err != nil || !bytes.Equal(opened.Anchor, original.Anchor) || opened.Prefix != prefix || opened.Binding != original.Binding || opened.Until != original.Until {
				t.Fatalf("opaque bytes or integer binding changed: size=%d prefix=%d, %v", size, prefix, err)
			}
		}
	}
}

func TestSearchCursorRandomNonceHidesCandidateSort(t *testing.T) {
	codec, cursor, now := searchCursorFixture(t)
	first, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("identical plaintext reused a deterministic nonce")
	}
	// HTTP游标允许用户base64解码；拒绝候选的完整sort/文档ID必须仍是密文。
	for _, token := range []string{first, second} {
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatal(err)
		}
		if json.Valid(raw) || bytes.Contains(raw, cursor.Anchor) || bytes.Contains(raw, []byte("18446744073709551615")) {
			t.Fatal("base64 decoding exposed candidate sort or document")
		}
		opened, err := codec.Open(token, cursor.Binding, now)
		if err != nil || !bytes.Equal(opened.Anchor, cursor.Anchor) {
			t.Fatal("random nonce prevented roundtrip")
		}
	}
	openSearchCursorFailure(t, codec, base64.RawURLEncoding.EncodeToString(cursor.Anchor), cursor.Binding, now, InvalidRequest, "CURSOR_INVALID")
}

func TestSearchCursorTamperingAndWrongMasterKeyFailClosed(t *testing.T) {
	codec, cursor, now := searchCursorFixture(t)
	token, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	// 每个字节都篡改一次，覆盖版本、随机nonce、密文及外层认证，拒绝结果不得泄露字段。
	for offset := range raw {
		changed := bytes.Clone(raw)
		changed[offset] ^= 1
		openSearchCursorFailure(t, codec, base64.RawURLEncoding.EncodeToString(changed), cursor.Binding, now, InvalidRequest, "CURSOR_INVALID")
	}
	other, err := NewSearchCursorCodec(bytes.Repeat([]byte{0x6b}, 32))
	if err != nil {
		t.Fatal(err)
	}
	openSearchCursorFailure(t, other, token, cursor.Binding, now, InvalidRequest, "CURSOR_INVALID")
}

func TestSearchCursorRejectsNonCanonicalOrOversizedEncoding(t *testing.T) {
	codec, cursor, now := searchCursorFixture(t)
	cursor.Anchor = []byte{1, 2}
	token, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", strings.Repeat("A", 2049), strings.Repeat("A", 2048), token + "=", token + "\n", token + "\r", " " + token, token[:len(token)-1], "+/" + token} {
		openSearchCursorFailure(t, codec, bad, cursor.Binding, now, InvalidRequest, "CURSOR_INVALID")
	}
	// 当末字节含未用pad bits时，宽松解码会接受另一拼法；codec必须坚持唯一raw base64url。
	if len(token)%4 != 2 && len(token)%4 != 3 {
		t.Fatal("fixture must exercise unused base64 bits")
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, token[len(token)-1])
	alias := token[:len(token)-1] + string(alphabet[last+1])
	openSearchCursorFailure(t, codec, alias, cursor.Binding, now, InvalidRequest, "CURSOR_INVALID")
}

func TestSearchCursorBindingAndAbsoluteExpiryNeverRenew(t *testing.T) {
	codec, cursor, now := searchCursorFixture(t)
	token, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	wrong := cursor.Binding
	wrong[31] ^= 1
	openSearchCursorFailure(t, codec, token, wrong, now, StateConflict, "CURSOR_STALE")
	openSearchCursorFailure(t, codec, token, [32]byte{}, now, InvalidRequest, "CURSOR_INVALID")
	// 使用传入的真实时钟；本地时区和连续Open均不能把绝对期限延长。
	for _, clock := range []time.Time{now, time.UnixMilli(cursor.Until - 1).In(time.FixedZone("CST", 8*60*60))} {
		opened, err := codec.Open(token, cursor.Binding, clock)
		if err != nil || opened.Until != cursor.Until {
			t.Fatalf("absolute expiry renewed: %v", err)
		}
	}
	for _, clock := range []time.Time{time.UnixMilli(cursor.Until), time.UnixMilli(cursor.Until + 1)} {
		openSearchCursorFailure(t, codec, token, cursor.Binding, clock, StateConflict, "CURSOR_EXPIRED")
	}
}

func TestSearchCursorRejectsInvalidKeyAndFields(t *testing.T) {
	for _, key := range [][]byte{nil, make([]byte, 32), bytes.Repeat([]byte{1}, 31), bytes.Repeat([]byte{1}, 33)} {
		codec, err := NewSearchCursorCodec(key)
		assertSearchCursorFailure(t, err, InvalidRequest, "CURSOR_INVALID")
		if codec != nil {
			t.Fatal("invalid key returned usable codec")
		}
	}
	codec, valid, _ := searchCursorFixture(t)
	for _, change := range []func(*SearchPageCursor){
		func(c *SearchPageCursor) { c.Anchor = nil },
		func(c *SearchPageCursor) { c.Anchor = make([]byte, 769) },
		func(c *SearchPageCursor) { c.Prefix = 100 },
		func(c *SearchPageCursor) { c.Prefix = 255 },
		func(c *SearchPageCursor) { c.Binding = [32]byte{} },
		func(c *SearchPageCursor) { c.Until = 0 },
		func(c *SearchPageCursor) { c.Until = -1 },
	} {
		cursor := valid
		change(&cursor)
		token, err := codec.Seal(cursor)
		assertSearchCursorFailure(t, err, InvalidRequest, "CURSOR_INVALID")
		if token != "" {
			t.Fatal("invalid fields returned a usable cursor")
		}
	}
	var missing *SearchCursorCodec
	_, err := missing.Seal(valid)
	assertSearchCursorFailure(t, err, InvalidRequest, "CURSOR_INVALID")
	openSearchCursorFailure(t, missing, "invalid", valid.Binding, time.Now(), InvalidRequest, "CURSOR_INVALID")
}

func TestSearchCursorOwnsMasterKeyAndReturnedAnchor(t *testing.T) {
	key := bytes.Repeat([]byte{0x5a}, 32)
	codec, err := NewSearchCursorCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	_, cursor, now := searchCursorFixture(t)
	original := bytes.Clone(cursor.Anchor)
	token, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	// 调用方可复用输入缓冲；codec不得持有该key/anchor别名。
	clear(key)
	clear(cursor.Anchor)
	first, err := codec.Open(token, cursor.Binding, now)
	if err != nil || !bytes.Equal(first.Anchor, original) {
		t.Fatal("caller mutation changed authenticated cursor")
	}
	clear(first.Anchor)
	second, err := codec.Open(token, cursor.Binding, now)
	if err != nil || !bytes.Equal(second.Anchor, original) {
		t.Fatal("returned anchor aliases another Open")
	}
	third, err := codec.Seal(SearchPageCursor{Anchor: original, Prefix: 99, Binding: cursor.Binding, Until: cursor.Until})
	if err != nil {
		t.Fatal("caller key mutation broke codec")
	}
	if _, err = codec.Open(third, cursor.Binding, now); err != nil {
		t.Fatal(err)
	}
}
