package cursor

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func validControl(phase string) Control {
	control := Control{Version: 1, Phase: phase, RetryAt: math.MaxInt64, Uncertain: true}
	if phase != PhaseIdle {
		control.Nonce = strings.Repeat("a", 64)
		control.Deadline = math.MaxInt64
	}
	if phase != PhaseOpening {
		control.Tail = json.RawMessage(`[1.2500e+2, "18446744073709551615",18446744073709551615]`)
	}
	return control
}

func TestControlRoundTripPrivateTailAndUnknownFacts(t *testing.T) {
	for _, phase := range []string{PhaseOpening, PhaseIdle, PhaseInflight} {
		control := validControl(phase)
		encoded, err := EncodeControl(control)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(encoded, []byte(`"version":1`)) || !bytes.Contains(encoded, []byte(`"retry_at":9223372036854775807`)) {
			t.Fatal("版本/完整int64词法丢失")
		}
		decoded, err := DecodeControl(encoded)
		if err != nil || decoded.Version != 1 || decoded.Phase != phase || decoded.Nonce != control.Nonce || decoded.Deadline != control.Deadline ||
			decoded.RetryAt != control.RetryAt || !decoded.Uncertain || !bytes.Equal(decoded.Tail, control.Tail) {
			t.Fatal("控制事实改变", err)
		}
		if len(control.Tail) != 0 {
			control.Tail[0] = 'x'
			if decoded.Tail[0] != '[' {
				t.Fatal("解码Tail引用源Control")
			}
			decoded.Tail[0] = 'x'
			if bytes.Contains(encoded, []byte(`x1.2500`)) {
				t.Fatal("解码Tail引用传输输入")
			}
		}
	}
}

func TestControlRejectsInvalidPhaseCombinations(t *testing.T) {
	cases := map[string]func(*Control){
		"version":           func(c *Control) { c.Version = 2 },
		"phase":             func(c *Control) { c.Phase = "CLOSING" },
		"opening tail":      func(c *Control) { c.Phase = PhaseOpening },
		"missing nonce":     func(c *Control) { c.Nonce = "" },
		"zero nonce":        func(c *Control) { c.Nonce = strings.Repeat("0", 64) },
		"uppercase nonce":   func(c *Control) { c.Nonce = strings.Repeat("A", 64) },
		"short nonce":       func(c *Control) { c.Nonce = strings.Repeat("a", 63) },
		"nonhex nonce":      func(c *Control) { c.Nonce = strings.Repeat("z", 64) },
		"deadline zero":     func(c *Control) { c.Deadline = 0 },
		"deadline negative": func(c *Control) { c.Deadline = -1 },
		"retry negative":    func(c *Control) { c.RetryAt = -1 },
		"idle nonce":        func(c *Control) { c.Phase = PhaseIdle; c.Deadline = 0 },
		"idle deadline":     func(c *Control) { c.Phase = PhaseIdle; c.Nonce = "" },
		"null literal tail": func(c *Control) { c.Tail = json.RawMessage(`null`) },
		"bad sort":          func(c *Control) { c.Tail = json.RawMessage(`[1,"1",1.0]`) },
		"sort limit":        func(c *Control) { c.Tail = json.RawMessage(strings.Repeat(" ", 129)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			control := validControl(PhaseInflight)
			mutate(&control)
			if _, err := EncodeControl(control); err == nil || err.Error() != "SEARCH_CURSOR_CONTROL_INVALID" {
				t.Fatal("无效组合被写入", err)
			}
		})
	}
}

func TestControlStrictRootDuplicatesUnknownNullTypesAndTrailing(t *testing.T) {
	// 重复键会让不同SQL JSON读取与Go解码看到不同控制事实，必须在持久读取入口拒绝。
	for _, raw := range []string{
		`{"version":1,"version":1,"phase":"IDLE"}`, `{"version":1,"vers\u0069on":1,"phase":"IDLE"}`,
		`{"version":1,"phase":"IDLE","unexpected":false}`, `{"version":1,"phase":"IDLE"}{}`,
		`{"version":1,"phase":"IDLE"}null`, `[]`, `null`, `{"version":1}`, `{"phase":"IDLE"}`,
		`{"version":1,"phase":"IDLE","deadline":null}`, `{"version":1,"phase":"IDLE","nonce":null}`,
		`{"version":1,"phase":"IDLE","retry_at":null}`, `{"version":1,"phase":"IDLE","uncertain":null}`,
		`{"version":1,"phase":"IDLE","retry_at":1e2}`, `{"version":1,"phase":"IDLE","retry_at":9223372036854775808}`,
		`{"version":1,"phase":"IDLE","uncertain":1}`, `{"version":1,"phase":"IDLE","tail":{}}`,
		`{"version":1,"phase":"IDLE","tail":""}`, `{"version":1,"phase":"IDLE","tail":"null"}`,
	} {
		if _, err := DecodeControl([]byte(raw)); err == nil || err.Error() != "SEARCH_CURSOR_CONTROL_INVALID" {
			t.Fatal("非严格JSON被接收", raw, err)
		}
	}
	control, err := DecodeControl([]byte(`{"version":1,"phase":"IDLE","tail":null}`))
	if err != nil || control.Tail != nil || control.Nonce != "" || control.Deadline != 0 {
		t.Fatal("合法空闲缺省失败", err)
	}
	if _, err := DecodeControl([]byte{'{', '"', 0xff, '"', ':', '1', '}'}); err == nil {
		t.Fatal("非法UTF8仍解析")
	}
}

func TestControlExact2048ByteReadBound(t *testing.T) {
	raw, err := EncodeControl(validControl(PhaseOpening))
	if err != nil {
		t.Fatal(err)
	}
	bounded := append(append([]byte(nil), raw...), bytes.Repeat([]byte{' '}, 2048-len(raw))...)
	if _, err := DecodeControl(bounded); err != nil {
		t.Fatal("2048边界错误", err)
	}
	if _, err := DecodeControl(append(bounded, ' ')); err == nil {
		t.Fatal("2049未拒绝")
	}
}

func TestControlTailStringSurvivesOuterSQLJSONReorderAnd128ByteBound(t *testing.T) {
	// SQL JSON会正规化数字/对象键。把Tail保存为字符串，外层重排不能改内层sort原字节。
	tail := []byte(` [1.2500e+2, "18446744073709551615",18446744073709551615] `)
	tail = append(tail, bytes.Repeat([]byte{' '}, 128-len(tail))...)
	control := validControl(PhaseInflight)
	control.Tail = tail
	encoded, err := EncodeControl(control)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(encoded, &fields) != nil {
		t.Fatal("无效控制JSON")
	}
	var stored string
	if json.Unmarshal(fields["tail"], &stored) != nil || stored != string(tail) {
		t.Fatal("Tail仍是会被SQL正规化的数组")
	}
	// RawMessage只模拟外层key重排/空白正规化，保留int64及string，不用float64伪造SQL读取。
	reordered, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeControl(reordered)
	if err != nil || !bytes.Equal(decoded.Tail, tail) {
		t.Fatal("外层重排改变原Tail", err)
	}
	for i := range encoded {
		encoded[i] = 'x'
	}
	for i := range reordered {
		reordered[i] = 'x'
	}
	if !bytes.Equal(decoded.Tail, tail) {
		t.Fatal("返回Tail引用输入JSON")
	}
	control.Tail = append(append([]byte(nil), tail...), ' ')
	if _, err := EncodeControl(control); err == nil {
		t.Fatal("129-byte原Tail仍可编码")
	}
	encodedTail, _ := json.Marshal(string(control.Tail))
	fields["tail"] = encodedTail
	oversized, _ := json.Marshal(fields)
	if _, err := DecodeControl(oversized); err == nil {
		t.Fatal("string解码后超过128仍接收")
	}
	fields["tail"] = json.RawMessage(`[1,"1",0]`)
	legacy, _ := json.Marshal(fields)
	if _, err := DecodeControl(legacy); err == nil {
		t.Fatal("旧数组Tail仍可读取")
	}
}
