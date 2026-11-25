package platform

import "encoding/hex"

// 期望集合来自受校验的发布配置；数据库不能自行声明一个新摘要就让服务认可。
// 001仍独立比较，附加迁移既不能缺失，也不能跳过版本或忽略未知记录。
func matchesSchemaMigrations(baseline string, additional, actual map[int]string) bool {
	if len(actual) != 1+len(additional) {
		return false
	}
	for version := 1; version <= 1+len(additional); version++ {
		expected := baseline
		if version > 1 {
			expected = additional[version]
		}
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != 32 || actual[version] != expected {
			return false
		}
	}
	return true
}
