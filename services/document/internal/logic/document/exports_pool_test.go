package document

import "testing"

// 配置遗漏沿用单worker；非法配置应在启动时拒绝，不能静默启动无限转换。
func TestExportPoolConfiguration(t *testing.T) {
	for _, input := range []struct{ configured, want int }{{0, 1}, {1, 1}, {2, 2}, {8, 8}} {
		service, err := NewExports(nil, nil, nil, nil, nil, nil, nil, input.configured)
		if err != nil || service.WorkerCount() != input.want {
			t.Fatalf("workers %d: service=%v, err=%v", input.configured, service, err)
		}
		if service.temporaryLimit != uint64(input.want)*256<<20 {
			t.Fatal("aggregate budget must scale with bounded concurrency")
		}
	}
	for _, input := range []int{-1, 9} {
		if _, err := NewExports(nil, nil, nil, nil, nil, nil, nil, input); err == nil {
			t.Fatalf("accepted invalid workers: %d", input)
		}
	}
}
