package account

import "testing"

func TestOrganizationCloseRequiresSoleActiveOwner(t *testing.T) {
	// 组织关闭不得清除其他存续/撤权未知成员的数据，owner也必须先交接。
	org := Organization{ID: "7", OwnerID: "3", Lifecycle: "ACTIVE", Revision: "2"}
	member := OrganizationMember{State: "ACTIVE", Epoch: 4}
	for _, tc := range []struct {
		name, actor, expected, state string
		members                      int
		allowed                      bool
	}{
		{"本人唯一owner", "3", "2", "ACTIVE", 1, true},
		{"其他成员仍存续", "3", "2", "ACTIVE", 2, false},
		{"非owner不能关闭", "4", "2", "ACTIVE", 1, false},
		{"撤权未知不能关闭", "3", "2", "REVOKING", 1, false},
		{"旧组织版本拒绝", "3", "1", "ACTIVE", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			member.State = tc.state
			err := OrganizationCloseEligible(org, tc.actor, tc.expected, member, tc.members)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v err=%v", tc.allowed, err)
			}
		})
	}
}
