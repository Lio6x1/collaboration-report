package document

import (
	"regexp"
	"strconv"
)

var operationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)

func ValidOperationID(value string) bool { return operationPattern.MatchString(value) }

// CanonicalID拒绝零值、符号及前导零，确保同一事实只有一个绑定字符串。
func CanonicalID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != value {
		return 0, Violation(InvalidRequest, "ID_INVALID")
	}
	return id, nil
}

func TextID(id uint64) string { return strconv.FormatUint(id, 10) }
