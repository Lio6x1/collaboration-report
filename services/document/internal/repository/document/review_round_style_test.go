package document

import "testing"

func TestFrozenRoundStyleExactID(t *testing.T) {
	for _, test := range []struct {
		raw, want string
		legacy    bool
	}{
		{`{"style_id":"1"}`, "1", false},
		{`{"style_id":1}`, "1", true},
		{`{"style_id":18446744073709551615}`, "18446744073709551615", true},
	} {
		style, legacy, err := frozenRoundStyle([]byte(test.raw))
		if err != nil || style != test.want || legacy != test.legacy {
			t.Fatalf("%s: %s %v %v", test.raw, style, legacy, err)
		}
	}
	// 数字兼容不能变成float64/宽松ID入口；尚须调用方核验实际原REVERSE来源。
	for _, raw := range []string{`{}`, `{"style_id":null}`, `{"style_id":0}`, `{"style_id":-1}`, `{"style_id":1.0}`, `{"style_id":1e1}`, `{"style_id":true}`, `{"style_id":"01"}`, `{"style_id":18446744073709551616}`} {
		if _, _, err := frozenRoundStyle([]byte(raw)); err == nil {
			t.Fatalf("invalid style accepted: %s", raw)
		}
	}
}
