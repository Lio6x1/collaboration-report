package document

import (
	dto "collaboration/contracts/httpdto"
	"testing"
)

// 期待修订是非负数；管理资格与当前control分别检查，不能把过期页面升级成新控制授权。
func TestMainControlIntentAndPosition(t *testing.T) {
	intent, err := ParseMainLock(dto.MainLock{OperationId: "main-control-operation", ExpectedControlRevision: "0", Locked: false})
	if err != nil {
		t.Fatal(err)
	}
	if intent.Kind != "MAIN_LOCK" || intent.Control != 0 || intent.Locked {
		t.Fatal("fixed lock intent changed")
	}
	if ValidateMainControl(Permissions{Manage: true}, "1", "3", true, intent) == nil {
		t.Fatal("stale control accepted")
	}
	intent.Control = 1
	if ValidateMainControl(Permissions{Review: true}, "1", "3", true, intent) == nil {
		t.Fatal("reviewer controls MAIN")
	}
	if err = ValidateMainControl(Permissions{Manage: true}, "1", "3", true, intent); err != nil {
		t.Fatal(err)
	}
	if _, err = ParseMainLock(dto.MainLock{OperationId: "main-control-operation", ExpectedControlRevision: "01"}); err == nil {
		t.Fatal("non-canonical control accepted")
	}
}
