package platform

import "testing"

func TestSchemaReadinessRequiresExactConfiguredMigrationSet(t *testing.T) {
	base := "9c9ea8ea0a1ef726f4fa205af99402ce6a88ee25a7d3049842eb8187eaac6d8d"
	index := "1111111111111111111111111111111111111111111111111111111111111111"
	// 新索引不能因只核对第一行而漏验；缺版本、改摘要及未知额外版本都拒绝。
	for _, tc := range []struct {
		name               string
		additional, actual map[int]string
		want               bool
	}{
		{"legacy exact", nil, map[int]string{1: base}, true},
		{"additional exact", map[int]string{2: index}, map[int]string{1: base, 2: index}, true},
		{"missing addition", map[int]string{2: index}, map[int]string{1: base}, false},
		{"changed addition", map[int]string{2: index}, map[int]string{1: base, 2: base}, false},
		{"unknown addition", nil, map[int]string{1: base, 2: index}, false},
		{"changed baseline", map[int]string{2: index}, map[int]string{1: index, 2: index}, false},
		{"noncontiguous config", map[int]string{3: index}, map[int]string{1: base, 3: index}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesSchemaMigrations(base, tc.additional, tc.actual); got != tc.want {
				t.Fatalf("migration expectation mismatch: got %v want %v", got, tc.want)
			}
		})
	}
}
