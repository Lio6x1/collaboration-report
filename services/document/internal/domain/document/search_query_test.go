package document

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math"
	"net/url"
	"strings"
	"testing"
	"time"
)

func assertSearchQueryFailure(t *testing.T, err error, code string) {
	t.Helper()
	var violation RuleViolation
	if !errors.As(err, &violation) || violation.Kind != InvalidRequest || violation.Code != code || err.Error() != code {
		t.Fatalf("want fixed invalid-input failure %s, got %v", code, err)
	}
}

func TestSearchQueryKeepsOriginalUnicodeAndDefaults(t *testing.T) {
	for _, text := range []string{"学", " 研发 😀 \n", strings.Repeat("😀", 500), "e\u0301", "\x00"} {
		query, err := ParseSearchQuery(url.Values{"q": {text}})
		if err != nil || query.Text != text || query.Limit != 20 || query.SpaceID != 0 || query.Cursor != "" {
			t.Fatalf("valid Unicode/defaults changed: %v", err)
		}
	}
	// 解析规则按Unicode码点计数，不按UTF8字节；不trim、NFC或大小写改写搜索词法。
	query, err := ParseSearchQuery(url.Values{"q": {" 研发 "}, "limit": {"100"}, "space_id": {"18446744073709551615"}})
	if err != nil || query.Text != " 研发 " || query.Limit != 100 || query.SpaceID != math.MaxUint64 {
		t.Fatalf("maximum uint64 or raw query lost: %v", err)
	}
}

func TestSearchQueryRejectsInvalidText(t *testing.T) {
	for _, text := range []string{"", " \t\n\r\u3000", strings.Repeat("中", 501), strings.Repeat("a", 501), string([]byte{0xff}), "ok" + string([]byte{0xc0, 0xaf})} {
		query, err := ParseSearchQuery(url.Values{"q": {text}})
		assertSearchQueryFailure(t, err, "SEARCH_QUERY_INVALID")
		if query != (SearchQuery{}) {
			t.Fatal("invalid text exposed usable parsed query")
		}
	}
	_, err := ParseSearchQuery(nil)
	assertSearchQueryFailure(t, err, "SEARCH_QUERY_INVALID")
}

func TestSearchQueryRejectsDuplicateUnknownAndValuelessParameters(t *testing.T) {
	// URL.Query().Get会悄悄选第一个值；即使两个值一样，也不能让另一层解释成不同查询。
	for _, values := range []url.Values{
		{"q": {"a", "a"}},
		{"q": {"a"}, "limit": {"20", "100"}},
		{"q": {"a"}, "space_id": {"", "1"}},
		{"q": {"a"}, "cursor": {"", ""}},
		{"q": {"a"}, "session_ref": {"pretend-identity"}},
		{"q": {"a"}, "scope_hash": {"pretend-scope"}},
		{"q": {"a"}, "Q": {"b"}},
		{"q": {"a"}, "": {"bad"}},
		{"q": nil},
		{"q": {"a"}, "limit": nil},
	} {
		query, err := ParseSearchQuery(values)
		assertSearchQueryFailure(t, err, "SEARCH_PARAMETERS_INVALID")
		if query != (SearchQuery{}) {
			t.Fatal("ambiguous parameter returned usable query")
		}
	}
}

func TestSearchQueryLimitAndSpaceUseCanonicalIntegers(t *testing.T) {
	for _, limit := range []string{"", "0", "101", "-1", "+20", "020", "20.0", " 20", "２０", "18446744073709551615"} {
		_, err := ParseSearchQuery(url.Values{"q": {"a"}, "limit": {limit}})
		assertSearchQueryFailure(t, err, "LIMIT_INVALID")
	}
	for _, limit := range []string{"1", "20", "100"} {
		if _, err := ParseSearchQuery(url.Values{"q": {"a"}, "limit": {limit}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, space := range []string{"0", "01", "-1", "+1", " 1", "1.0", "１", "18446744073709551616"} {
		_, err := ParseSearchQuery(url.Values{"q": {"a"}, "space_id": {space}})
		assertSearchQueryFailure(t, err, "ID_INVALID")
	}
	query, err := ParseSearchQuery(url.Values{"q": {"a"}, "space_id": {""}, "cursor": {""}})
	if err != nil || query.SpaceID != 0 || query.Cursor != "" {
		t.Fatal("explicit empty optional fields changed default scope")
	}
}

func TestSearchQueryCursorChecksEncodingButDefersAuthentication(t *testing.T) {
	maximum := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, 1536))
	if len(maximum) != 2048 {
		t.Fatal("test fixture did not reach HTTP cursor bound")
	}
	query, err := ParseSearchQuery(url.Values{"q": {"a"}, "cursor": {maximum}})
	if err != nil || query.Cursor != maximum {
		t.Fatal("canonical 2048-byte boundary rejected")
	}
	for _, cursor := range []string{maximum + "A", "AA=", "AA\n", "AA\r", " AA", "AB", "+/", "a", "游标"} {
		_, err := ParseSearchQuery(url.Values{"q": {"a"}, "cursor": {cursor}})
		assertSearchQueryFailure(t, err, "CURSOR_INVALID")
	}
	// 规范base64不等于可信游标；这里只检查词法，codec仍必须拒绝无有效MAC的内容。
	query, err = ParseSearchQuery(url.Values{"q": {"a"}, "cursor": {"AA"}})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := query.Binding(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	codec, _, now := searchCursorFixture(t)
	_, err = codec.Open(query.Cursor, binding, now)
	assertSearchCursorFailure(t, err, InvalidRequest, "CURSOR_INVALID")
}

func TestSearchQueryBindingStableAndCoversEveryAuthorityParameter(t *testing.T) {
	// 独立固定向量钉住v1编码：q="a"、默认20、全部空间、匿名、空session。
	// 不能在未换用途版本时静默改序列化，导致已发游标无故失效。
	pinned := SearchQuery{Text: "a", Limit: 20}
	vector, err := pinned.Binding(true, nil)
	if err != nil || hex.EncodeToString(vector[:]) != "506746f0bcf75b8e2cf000cbae06f98cd6d5f942bcc38fdf7030a31e613cae68" {
		t.Fatalf("v1 stable SHA256 binding changed: %v", err)
	}
	query, err := ParseSearchQuery(url.Values{"q": {"研发 😀"}})
	if err != nil {
		t.Fatal(err)
	}
	session := bytes.Repeat([]byte{1}, 32)
	first, err := query.Binding(false, session)
	if err != nil || first == [32]byte{} {
		t.Fatal("valid real session did not bind")
	}
	equivalent, err := ParseSearchQuery(url.Values{"space_id": {""}, "cursor": {"AA"}, "limit": {"20"}, "q": {"研发 😀"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := equivalent.Binding(false, bytes.Clone(session))
	if err != nil || first != second {
		t.Fatal("query order/default spelling/page token changed binding")
	}
	for _, change := range []func(*SearchQuery){
		func(q *SearchQuery) { q.Text += " " },
		func(q *SearchQuery) { q.Text = "研发 😁" },
		func(q *SearchQuery) { q.Limit = 1 },
		func(q *SearchQuery) { q.SpaceID = math.MaxUint64 },
	} {
		changed := query
		change(&changed)
		binding, err := changed.Binding(false, session)
		if err != nil || binding == first {
			t.Fatal("changed request retained old binding")
		}
	}
	anonymous, err := query.Binding(true, nil)
	if err != nil || anonymous == first {
		t.Fatal("anonymous and real session share cursor binding")
	}
	session[31] ^= 1
	other, err := query.Binding(false, session)
	if err != nil || other == first {
		t.Fatal("session switching retained old binding")
	}
}

func TestSearchQueryBindingRejectsForgedOrMissingSession(t *testing.T) {
	query, err := ParseSearchQuery(url.Values{"q": {"a"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct {
		anonymous bool
		session   []byte
	}{
		{true, []byte{1}}, {true, bytes.Repeat([]byte{1}, 32)},
		{false, nil}, {false, make([]byte, 32)}, {false, bytes.Repeat([]byte{1}, 31)}, {false, bytes.Repeat([]byte{1}, 33)},
	} {
		binding, err := query.Binding(value.anonymous, value.session)
		assertSearchQueryFailure(t, err, "SEARCH_BINDING_INVALID")
		if binding != [32]byte{} {
			t.Fatal("invalid identity returned usable binding")
		}
	}
	// 公开domain值也须复核，不能绕过Parse直接构造非法limit或未验证cursor。
	for _, invalid := range []SearchQuery{{Text: "a", Limit: 0}, {Text: "", Limit: 20}, {Text: "a", Limit: 20, Cursor: "AA="}} {
		if binding, err := invalid.Binding(true, nil); err == nil || binding != [32]byte{} {
			t.Fatal("direct invalid query acquired a binding")
		}
	}
}

func TestSearchQueryBindingWorksWithEncryptedPageCursor(t *testing.T) {
	query, err := ParseSearchQuery(url.Values{"q": {"中文"}, "limit": {"100"}, "space_id": {"18446744073709551615"}})
	if err != nil {
		t.Fatal(err)
	}
	session := bytes.Repeat([]byte{9}, 32)
	binding, err := query.Binding(false, session)
	if err != nil {
		t.Fatal(err)
	}
	codec, cursor, now := searchCursorFixture(t)
	cursor.Binding = binding
	cursor.Until = now.Add(30 * time.Second).UnixMilli()
	token, err := codec.Seal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	query.Cursor = token
	same, err := query.Binding(false, session)
	if err != nil || same != binding {
		t.Fatal("next page token changed query binding")
	}
	if _, err := codec.Open(token, same, now); err != nil {
		t.Fatal(err)
	}
	query.SpaceID--
	changed, err := query.Binding(false, session)
	if err != nil {
		t.Fatal(err)
	}
	_, err = codec.Open(token, changed, now)
	assertSearchCursorFailure(t, err, StateConflict, "CURSOR_STALE")
}
