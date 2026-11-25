package cursor

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func codecForTest(t *testing.T) *Codec {
	t.Helper()
	codec, err := NewCodec(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func batchForTest() Batch {
	return Batch{Chain: [32]byte{1}, Digest: [32]byte{2}, Count: 100, More: true,
		Before: json.RawMessage(`[1.2500e+2,"9007199254740993",9007199254740993]`),
		After:  json.RawMessage(`[1.2500e+2,"18446744073709551615",18446744073709551615]`)}
}

func TestCodecRejectsPublicZeroMasterAndMalformedKeyLengths(t *testing.T) {
	// 全零master虽有32字节，却是任何人都已知的签名密钥；不能把配置缺失当合法密钥。
	for _, test := range []struct {
		name string
		key  []byte
	}{
		{name: "全零32字节", key: make([]byte, 32)},
		{name: "nil", key: nil},
		{name: "31字节", key: bytes.Repeat([]byte{7}, 31)},
		{name: "33字节", key: bytes.Repeat([]byte{7}, 33)},
	} {
		t.Run(test.name, func(t *testing.T) {
			codec, err := NewCodec(test.key)
			if err == nil || codec != nil {
				t.Fatal("无效master仍可创建签名Codec")
			}
			if err.Error() != "SEARCH_CURSOR_KEY_INVALID" {
				t.Fatal("无效master必须返回固定错误")
			}
		})
	}
}

func TestBatchAnchorRoundTripsInitialTerminalAndEmptyWithoutRewritingSort(t *testing.T) {
	codec := codecForTest(t)
	initial := batchForTest()
	initial.Before = nil
	terminal := batchForTest()
	terminal.Count, terminal.More = 17, false
	empty := batchForTest()
	empty.Count, empty.More, empty.After = 0, false, append(json.RawMessage(nil), empty.Before...)
	initialEmpty := empty
	initialEmpty.Before, initialEmpty.After = nil, nil
	// 初批没有输入锚点，终批仍可回放；整数和score原词法不经float64重新序列化。
	for _, batch := range []Batch{initial, terminal, empty, initialEmpty, batchForTest()} {
		sealed, err := codec.Seal(batch)
		if err != nil {
			t.Fatal(err)
		}
		opened, err := codec.Open(sealed)
		if err != nil || opened.Chain != batch.Chain || opened.Digest != batch.Digest ||
			opened.Count != batch.Count || opened.More != batch.More ||
			!bytes.Equal(opened.Before, batch.Before) || !bytes.Equal(opened.After, batch.After) {
			t.Fatalf("批锚点或原sort改变: %v", err)
		}
		for _, action := range []Action{ActionNext, ActionReplay} {
			envelope, err := Envelope(action, sealed)
			if err != nil || len(envelope) > 768 {
				t.Fatalf("请求envelope超过边界: %v", err)
			}
			actual, anchor, err := ParseEnvelope(envelope)
			if err != nil || actual != action || !bytes.Equal(anchor, sealed) {
				t.Fatalf("动作与已完成批锚点不一致: %v", err)
			}
		}
		again, err := codec.Seal(batch)
		if err != nil || !bytes.Equal(sealed, again) {
			t.Fatal("同一已完成批必须产生确定锚点")
		}
	}
}

func TestBatchAnchorRejectsInvalidCombinations(t *testing.T) {
	codec := codecForTest(t)
	changes := map[string]func(*Batch){
		"零chain":   func(b *Batch) { b.Chain = [32]byte{} },
		"零摘要":      func(b *Batch) { b.Digest = [32]byte{} },
		"超过100":    func(b *Batch) { b.Count = 101 },
		"非满批More":  func(b *Batch) { b.Count = 99 },
		"非空缺尾":     func(b *Batch) { b.After = nil },
		"空批改变位置":   func(b *Batch) { b.Count, b.More = 0, false },
		"空批More":   func(b *Batch) { b.Count, b.After = 0, b.Before },
		"无效before": func(b *Batch) { b.Before = json.RawMessage(`null`) },
		"无效after":  func(b *Batch) { b.After = json.RawMessage(`[1,"0",0]`) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			batch := batchForTest()
			change(&batch)
			if _, err := codec.Seal(batch); err == nil {
				t.Fatal("无效批次被签发")
			}
		})
	}
}

func TestAnchorRejectsEveryByteMutationTruncationExtraBytesAndOtherPurpose(t *testing.T) {
	codec := codecForTest(t)
	sealed, err := codec.Seal(batchForTest())
	if err != nil {
		t.Fatal(err)
	}
	// 每个字段与MAC都受签名覆盖；位置、版本或边界的修改不能借合法原签名通过。
	for index := range sealed {
		changed := append([]byte(nil), sealed...)
		changed[index] ^= 1
		if _, err := codec.Open(changed); err == nil {
			t.Fatalf("第%d字节可篡改", index)
		}
	}
	for length := 0; length < len(sealed); length++ {
		if _, err := codec.Open(sealed[:length]); err == nil {
			t.Fatalf("长度%d的截断被接受", length)
		}
	}
	if _, err := codec.Open(append(sealed, 0)); err == nil {
		t.Fatal("尾随字节被接受")
	}
	if _, err := codec.Open(make([]byte, 769)); err == nil {
		t.Fatal("超预算锚点被接受")
	}
	foreign := append([]byte(nil), sealed...)
	mac := hmac.New(sha256.New, bytes.Repeat([]byte{7}, 32))
	mac.Write([]byte("workbench-cursor-v1\n"))
	mac.Write(foreign[:len(foreign)-32])
	copy(foreign[len(foreign)-32:], mac.Sum(nil))
	if _, err := codec.Open(foreign); err == nil {
		t.Fatal("其他用途HMAC被接受")
	}
}

func TestEnvelopeHasExact768ByteBoundaryAndStrictFraming(t *testing.T) {
	// envelope只框定合法动作与不透明字节，签名验证由Codec.Open完成，不能用framing当验签。
	probe, err := Envelope(ActionNext, []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	overhead := len(probe) - 1
	anchor := bytes.Repeat([]byte{9}, 768-overhead)
	envelope, err := Envelope(ActionReplay, anchor)
	if err != nil || len(envelope) != 768 {
		t.Fatalf("768字节边界不正确: %v", err)
	}
	if _, _, err = ParseEnvelope(envelope); err != nil {
		t.Fatal(err)
	}
	if _, err = Envelope(ActionReplay, append(anchor, 0)); err == nil {
		t.Fatal("769字节envelope被生成")
	}
	for _, action := range []Action{0, 3, 255} {
		if _, err = Envelope(action, []byte{1}); err == nil {
			t.Fatal("非法请求动作被生成")
		}
	}
	if _, err = Envelope(ActionNext, nil); err == nil {
		t.Fatal("续接请求缺少锚点仍被生成")
	}
	for _, invalid := range [][]byte{nil, probe[:len(probe)-1], append(probe, 0), append(envelope, 0)} {
		if _, _, err = ParseEnvelope(invalid); err == nil {
			t.Fatal("不完整或额外请求字节被接受")
		}
	}
	for _, index := range []int{0, 3, 4, 5, 6} {
		changed := append([]byte(nil), probe...)
		changed[index] = 255
		if _, _, err = ParseEnvelope(changed); err == nil {
			t.Fatalf("非法版本/动作/长度(%d)被接受", index)
		}
	}
}

func TestCodecAndDecodedSlicesDoNotAliasInputs(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	codec, err := NewCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := codec.Seal(batchForTest())
	if err != nil {
		t.Fatal(err)
	}
	key[0] ^= 1
	if _, err = codec.Open(sealed); err != nil {
		t.Fatal("调用方修改key改变Codec")
	}
	opened, err := codec.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	opened.Before[0], opened.After[0] = '!', '!'
	if _, err = codec.Open(sealed); err != nil {
		t.Fatal("解码sort别名到锚点")
	}
	envelope, err := Envelope(ActionNext, sealed)
	if err != nil {
		t.Fatal(err)
	}
	_, parsed, err := ParseEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	parsed[0] ^= 1
	_, intact, err := ParseEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.Open(intact); err != nil {
		t.Fatal("解析请求返回原envelope别名")
	}
	sealed[0] ^= 1
	_, intact, err = ParseEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.Open(intact); err != nil {
		t.Fatal("构造envelope复用了输入slice")
	}
	for _, length := range []int{0, 1, 31, 33} {
		if _, err = NewCodec(make([]byte, length)); err == nil {
			t.Fatal("非法签名key长度被接受")
		}
	}
	var absent *Codec
	if _, err = absent.Open(intact); err == nil {
		t.Fatal("无Codec实例仍可解码")
	}
}
