package account

import (
	"strings"
	"testing"
)

func TestPasswordLengthIsUTF8Bytes(t *testing.T) {
	if ValidPassword(strings.Repeat("中", 25)) {
		t.Fatal("75-byte password would be silently outside bcrypt limit")
	}
	if ValidPassword("") {
		t.Fatal("empty password accepted")
	}
	if !ValidPassword(strings.Repeat("中", 24)) {
		t.Fatal("72-byte password should be accepted")
	}
}
func TestEmailRejectsDisplayNameAndOversize(t *testing.T) {
	if ValidEmail("User <x@example.test>") || ValidEmail(strings.Repeat("a", 250)+"@example.test") {
		t.Fatal("non-address or oversized identifier accepted")
	}
	if !ValidEmail("learner@example.test") {
		t.Fatal("valid address rejected")
	}
}

func TestNewPasswordPolicyKeepsLegacyInputSeparate(t *testing.T) {
	// 覆盖三种两类组合、八字符边界以及Unicode不能冒充英文/特殊字符。
	for _, item := range []struct {
		password string
		valid    bool
	}{
		{"Abcdef12", true}, {"abcd!efg", true}, {"1234567!", true},
		{"abcD12!x", true}, {"abc123!", false}, {"abcdefgh", false},
		{"ABCDEFGH", false}, {"abcdEFGH", false}, {"12345678", false}, {"!!!!!!!!", false},
		{"abcd    ", false}, {"中文文字1234", false}, {"中文文字ab12", true},
		{"中文a1!", false}, {strings.Repeat("a", 71) + "1", true},
		{strings.Repeat("a", 72) + "1", false}, {strings.Repeat("中", 23) + "a1!", true},
		{strings.Repeat("中", 24) + "a1!", false}, {"abc123!\xff", false},
	} {
		if ValidNewPassword(item.password) != item.valid {
			t.Errorf("新密码规则判断不符，字符数=%d、字节数=%d", len([]rune(item.password)), len(item.password))
		}
	}
	if !ValidPassword("old") || ValidNewPassword("old") {
		t.Fatal("历史密码仍可参加原哈希验证，但不能作为新设置的密码")
	}
}
