package document

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSearchRPCEnvelopeOpaqueAnchor(t *testing.T) {
	// Doc只选择NEXT/REPLAY，故意用非SCB内容证明没有窥探或重签Search的opaque锚点。
	anchor := []byte{0xff, 0, 'o', 'p', 'a', 'q', 'u', 'e'}
	for _, replay := range []bool{false, true} {
		envelope, err := EnvelopeSearchAnchor(anchor, replay)
		action := byte(1)
		if replay {
			action = 2
		}
		if err != nil || !bytes.Equal(envelope[:4], []byte{'S', 'C', 'E', 1}) || envelope[4] != action || binary.BigEndian.Uint16(envelope[5:7]) != uint16(len(anchor)) || !bytes.Equal(envelope[7:], anchor) {
			t.Fatalf("opaque envelope failed: error=%v", err)
		}
		envelope[7] = 7
		if anchor[0] != 0xff {
			t.Fatal("envelope aliases source anchor")
		}
	}
}

func TestSearchRPCEnvelopeBound(t *testing.T) {
	for _, size := range []int{0, 761, 762, 768, 769} {
		envelope, err := EnvelopeSearchAnchor(bytes.Repeat([]byte{1}, size), false)
		if size == 761 {
			if err != nil || len(envelope) != 768 {
				t.Fatalf("maximum frame rejected: %v", err)
			}
		} else if err == nil || envelope != nil {
			t.Fatalf("invalid frame size accepted: %d", size)
		}
	}
}
