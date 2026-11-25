package cursor

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSortRequiresExactThreeTypesAndFullCanonicalUint64(t *testing.T) {
	for _, raw := range []string{
		`[0,"1",0]`, `[-0,"1",0]`, `[-1.25e+2,"9007199254740993",9007199254740993]`,
		`[1.2500e+2,"18446744073709551615",18446744073709551615]`,
	} {
		if err := ValidateSort(json.RawMessage(raw)); err != nil {
			t.Fatalf("合法sort被拒绝: %s: %v", raw, err)
		}
	}
	// JSON number合法还不足够；shard必须是规范uint64整数，Doc必须是规范非零字符串。
	for _, raw := range []string{
		`null`, `{}`, `[]`, `[1,"1"]`, `[1,"1",0,1]`, `[1,"1",0] false`,
		`["1","1",0]`, `[null,"1",0]`, `[true,"1",0]`, `[[1],"1",0]`,
		`[NaN,"1",0]`, `[1e309,"1",0]`, `[-1e309,"1",0]`, `[01,"1",0]`,
		`[1,1,0]`, `[1,"0",0]`, `[1,"01",0]`, `[1,"+1",0]`,
		`[1,"\u0031",0]`, `[1,"18446744073709551616",0]`,
		`[1,"1","0"]`, `[1,"1",-0]`, `[1,"1",-1]`, `[1,"1",1.0]`,
		`[1,"1",1e0]`, `[1,"1",18446744073709551616]`,
	} {
		if err := ValidateSort(json.RawMessage(raw)); err == nil {
			t.Fatalf("非法sort被接受: %s", raw)
		}
	}
}

func TestSort128ByteLimitPreservesAllOriginalBytes(t *testing.T) {
	base := []byte(`[1.2500e+2,"18446744073709551615",18446744073709551615]`)
	full := append(bytes.Repeat([]byte{' '}, 128-len(base)), base...)
	if err := ValidateSort(full); err != nil {
		t.Fatal(err)
	}
	codec := codecForTest(t)
	batch := batchForTest()
	batch.Before, batch.After = full, append(json.RawMessage(nil), full...)
	sealed, err := codec.Seal(batch)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := codec.Open(sealed)
	if err != nil || !bytes.Equal(full, opened.Before) || !bytes.Equal(full, opened.After) {
		t.Fatalf("128字节原sort被裁剪/改写: %v", err)
	}
	if err := ValidateSort(append(full, ' ')); err == nil {
		t.Fatal("129字节sort被接受")
	}
}
