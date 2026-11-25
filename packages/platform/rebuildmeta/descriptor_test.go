package rebuildmeta

import (
	"bytes"
	"strings"
	"testing"
)

func TestDescriptorBindsOriginalStartRequest(t *testing.T) {
	d := Descriptor{Version: 1, PrepareRef: "rebuild-prepare-v1:" + strings.Repeat("a", 64),
		SnapshotRef: "rebuild-snapshot-v1:" + strings.Repeat("b", 64), Cluster: "cluster", Topic: "document-events",
		TopicID: strings.Repeat("c", 32), Starts: [3]string{"0", "42", "9223372036854775807"}}
	raw, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	first, err := RequestFingerprint("rebuild-one", 2, d.SnapshotRef, raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RequestFingerprint("rebuild-one", 2, d.SnapshotRef, raw)
	if err != nil || first != again {
		t.Fatal("原请求必须得到相同摘要")
	}
	d.Starts[0] = "1"
	changed, _ := d.Bytes()
	other, err := RequestFingerprint("rebuild-one", 2, d.SnapshotRef, changed)
	if err != nil || other == first {
		t.Fatal("原号不能掩盖起点变化")
	}
	// 规范编码同时拒绝重复键、未知字段和数字offset，不接受JSON解码器的覆盖语义。
	for _, invalid := range [][]byte{
		bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"extra":true`), 1),
		bytes.Replace(raw, []byte(`"0"`), []byte(`0`), 1),
	} {
		if _, err := ParseDescriptor(invalid); err == nil {
			t.Fatal("非规范描述符被接受")
		}
	}
}
