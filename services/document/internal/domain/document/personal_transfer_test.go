package document

import (
	"testing"
	"time"
)

func TestPersonalTransferActionRequiresPendingParticipantAndRevision(t *testing.T) {
	// 容量/身份由真实事务验证；这里只覆盖必须拒绝的核心状态机反例。
	now := time.Now()
	p := PersonalTransfer{SourceUser: 7, TargetUser: 9, Revision: 2, State: "PENDING", Now: now, Expires: now.Add(time.Hour)}
	for _, test := range []struct {
		actor, revision uint64
		action          string
		state           string
		expired         bool
	}{
		{7, 2, "ACCEPT", "PENDING", false}, {9, 2, "CANCEL", "PENDING", false}, {9, 1, "ACCEPT", "PENDING", false},
		{9, 2, "ACCEPT", "ACCEPTING", false}, {7, 2, "CANCEL", "ACCEPTED", false}, {9, 2, "ACCEPT", "PENDING", true},
	} {
		candidate := p
		candidate.State = test.state
		if test.expired {
			candidate.Expires = now
		}
		if ValidatePersonalTransferAction(candidate, test.actor, test.revision, test.action) == nil {
			t.Fatalf("必须拒绝未满足本人/版本/待接受/期限的动作: %+v", test)
		}
	}
	if err := ValidatePersonalTransferAction(p, 9, 2, "ACCEPT"); err != nil {
		t.Fatal(err)
	}
}

func TestPersonalTransferCapacityCannotWrap(t *testing.T) {
	// 满额容量不能通过 uint64 回绕伪装成剩余容量。
	if _, err := TransferCapacity(^uint64(0), 1); err == nil {
		t.Fatal("容量溢出必须拒绝")
	}
	if amount, err := TransferCapacity(19, 23); err != nil || amount != 42 {
		t.Fatalf("合法正文和资源计量失败: %d %v", amount, err)
	}
}
