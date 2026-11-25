package cursor

import (
	pb "collaboration/contracts/internalv1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
)

func digestCandidate(document uint64) *pb.SearchCandidate {
	return &pb.SearchCandidate{DocumentId: document, ReleaseId: ^uint64(0), AggregateSeq: ^uint64(0), PhysicalIndex: "owned-g1",
		SortJsonUtf8: []byte(fmt.Sprintf(`[1.2500e+2,"%d",18446744073709551615]`, document))}
}

func TestDigestCandidatesStableEmptyTerminalAndFullBatch(t *testing.T) {
	for _, count := range []int{0, 1, 100} {
		candidates := []*pb.SearchCandidate{}
		for i := 0; i < count; i++ {
			candidates = append(candidates, digestCandidate(uint64(i+1)))
		}
		var after json.RawMessage
		if count != 0 {
			after = candidates[count-1].SortJsonUtf8
		}
		first, err := DigestCandidates(nil, after, candidates, count == 100)
		second, secondErr := DigestCandidates(nil, after, candidates, count == 100)
		if err != nil || secondErr != nil || first == ([32]byte{}) || first != second {
			t.Fatal("稳定完整摘要失败", err, secondErr)
		}
	}
	before := json.RawMessage(`[1,"18446744073709551615",18446744073709551615]`)
	if _, err := DigestCandidates(before, before, nil, false); err != nil {
		t.Fatal("空EOF必须保留原位置", err)
	}
}

func TestDigestCandidatesBindsEveryOrderedFrozenFieldAndRawSort(t *testing.T) {
	before := json.RawMessage(`[2,"1",0]`)
	candidates := []*pb.SearchCandidate{digestCandidate(2), digestCandidate(^uint64(0))}
	after := candidates[1].SortJsonUtf8
	base, err := DigestCandidates(before, after, candidates, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]*pb.SearchCandidate){
		"physical": func(c []*pb.SearchCandidate) {
			for _, v := range c {
				v.PhysicalIndex = "owned-g2"
			}
		},
		"release":  func(c []*pb.SearchCandidate) { c[0].ReleaseId-- },
		"sequence": func(c []*pb.SearchCandidate) { c[0].AggregateSeq-- },
		"document": func(c []*pb.SearchCandidate) {
			c[0].DocumentId = 3
			c[0].SortJsonUtf8 = digestCandidate(3).SortJsonUtf8
		},
		"original lexeme": func(c []*pb.SearchCandidate) { c[0].SortJsonUtf8 = []byte(`[125.00,"2",18446744073709551615]`) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := []*pb.SearchCandidate{proto.Clone(candidates[0]).(*pb.SearchCandidate), proto.Clone(candidates[1]).(*pb.SearchCandidate)}
			mutate(changed)
			digest, err := DigestCandidates(before, changed[1].SortJsonUtf8, changed, false)
			if err != nil || digest == base {
				t.Fatal("候选字段未绑定", err)
			}
		})
	}
	changedStart, err := DigestCandidates(json.RawMessage(`[3,"1",0]`), after, candidates, false)
	if err != nil || changedStart == base {
		t.Fatal("起点未绑定", err)
	}
	reversed := []*pb.SearchCandidate{candidates[1], candidates[0]}
	ordered, err := DigestCandidates(before, reversed[1].SortJsonUtf8, reversed, false)
	if err != nil || ordered == base {
		t.Fatal("稳定顺序未绑定", err)
	}
	full := []*pb.SearchCandidate{}
	for i := uint64(1); i <= 100; i++ {
		full = append(full, digestCandidate(i))
	}
	terminal, _ := DigestCandidates(nil, full[99].SortJsonUtf8, full, false)
	more, _ := DigestCandidates(nil, full[99].SortJsonUtf8, full, true)
	if terminal == more {
		t.Fatal("More未绑定")
	}
}

func TestDigestCandidatesRejectsIncompleteOrUnboundBatch(t *testing.T) {
	for name, mutate := range map[string]func(*pb.SearchCandidate){
		"zero doc":      func(c *pb.SearchCandidate) { c.DocumentId = 0 },
		"zero release":  func(c *pb.SearchCandidate) { c.ReleaseId = 0 },
		"zero sequence": func(c *pb.SearchCandidate) { c.AggregateSeq = 0 },
		"bad physical":  func(c *pb.SearchCandidate) { c.PhysicalIndex = "bad/index" },
		"sort doc":      func(c *pb.SearchCandidate) { c.SortJsonUtf8 = []byte(`[1,"9",0]`) },
		"sort types":    func(c *pb.SearchCandidate) { c.SortJsonUtf8 = []byte(`[1,"1",1e0]`) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := digestCandidate(1)
			mutate(candidate)
			if digest, err := DigestCandidates(nil, candidate.SortJsonUtf8, []*pb.SearchCandidate{candidate}, false); err == nil || digest != ([32]byte{}) {
				t.Fatal("不完整摘要被计算")
			}
		})
	}
	valid := digestCandidate(1)
	for _, candidates := range [][]*pb.SearchCandidate{{nil}, {valid, valid}, {valid, digestCandidate(2)}} {
		if _, err := DigestCandidates(nil, valid.SortJsonUtf8, candidates, false); err == nil {
			t.Fatal("nil/重复/非尾after未拒绝")
		}
	}
	other := digestCandidate(2)
	other.PhysicalIndex = "owned-g2"
	if _, err := DigestCandidates(nil, other.SortJsonUtf8, []*pb.SearchCandidate{valid, other}, false); err == nil {
		t.Fatal("跨物理索引被接受")
	}
	if _, err := DigestCandidates(nil, valid.SortJsonUtf8, []*pb.SearchCandidate{valid}, true); err == nil {
		t.Fatal("不满100仍More")
	}
	if _, err := DigestCandidates(nil, valid.SortJsonUtf8, nil, false); err == nil {
		t.Fatal("空批改变位置")
	}
	if _, err := DigestCandidates(json.RawMessage(`null`), nil, nil, false); err == nil {
		t.Fatal("非法起点")
	}
	oversized := []*pb.SearchCandidate{}
	for i := uint64(1); i <= 101; i++ {
		oversized = append(oversized, digestCandidate(i))
	}
	if _, err := DigestCandidates(nil, oversized[100].SortJsonUtf8, oversized, true); err == nil {
		t.Fatal("超100仍生成摘要")
	}
}

func TestDigestCandidatesPurposeGoldenEmptyVector(t *testing.T) {
	// 固定向量锁住独立用途标签与有长度字段的编码，避免以后误换成anchor签名用途。
	digest, err := DigestCandidates(nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(digest[:]) != "9927e63acf90edb262d0fd0202d7d8e5ead48e8b7af7fe397ed9f60e24eecf4f" {
		t.Fatal("批摘要用途或版本编码改变")
	}
}
