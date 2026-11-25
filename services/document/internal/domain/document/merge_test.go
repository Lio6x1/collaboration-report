package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"testing"
)

// 自审限制跨整个请求保留；当前管理者的显式例外不能被后来取得的 REVIEW 权限替代。
func TestMergeEligibilityPreservesEverySubmitter(t *testing.T) {
	pointer := ReviewPointer{ID: 3, Unit: 5, Round: 7, Status: "REVIEWING"}
	intent := MergeIntent{Operation: "merge-operation-0001", Round: 7, Version: 11}
	guard := ReviewGuard{Ongoing: 3}
	if err := ValidateMergePosition(pointer, guard, "", intent, 11); err != nil {
		t.Fatal(err)
	}
	if CanApproveMerge(Permissions{Review: true}, true) || !CanApproveMerge(Permissions{Manage: true}, true) || !CanApproveMerge(Permissions{Review: true}, false) {
		t.Fatal("self-review eligibility changed")
	}
	for _, state := range []string{"WITHDRAWN", "MERGED"} {
		changed := pointer
		changed.Status = state
		if ValidateMergePosition(changed, guard, "", intent, 11) == nil {
			t.Fatal("inactive review was accepted")
		}
	}
	if ValidateMergePosition(pointer, guard, "other-operation-0001", intent, 11) == nil || ValidateMergePosition(pointer, guard, "", intent, 12) == nil {
		t.Fatal("foreign reservation or changed fixed version accepted")
	}
}

// Revision契约允许零，但数据库现有意见从1开始；合法旧预期应走CAS冲突，而不是被当作非法ID。
func TestMergeResolutionZeroRevisionIsAStaleExpectation(t *testing.T) {
	var input dto.ApproveMerge
	if err := json.Unmarshal([]byte(`{"operation_id":"revision-merge-operation","round_id":"7","content_version_id":"11","own_changes_to_resolve":[{"id":"3","revision":"0"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	intent, err := ParseMerge(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(intent.Resolutions) != 1 || intent.Resolutions[0].Revision != 0 {
		t.Fatal("zero revision expectation changed")
	}
	if ValidateMergeChanges(9, []ChangeFact{{ID: 3, Author: 9, Revision: 1, Status: "OPEN"}}, intent.Resolutions) == nil {
		t.Fatal("stale revision accepted")
	}
}

// 拟解除只是合并意图：缺一条、异作者或异revision都拒绝，输入意见保持OPEN直到成功事务。
func TestMergeResolutionsRequireAllCurrentOwnChanges(t *testing.T) {
	changes := []ChangeFact{{ID: 1, Author: 9, Revision: 2, Status: "OPEN"}, {ID: 2, Author: 9, Revision: 4, Status: "OPEN"}}
	wanted := []MergeResolution{{ID: 1, Revision: 2}, {ID: 2, Revision: 4}}
	if err := ValidateMergeChanges(9, changes, wanted); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]MergeResolution{wanted[:1], {{ID: 1, Revision: 3}, {ID: 2, Revision: 4}}, {{ID: 1, Revision: 2}, {ID: 1, Revision: 2}}, {{ID: 3, Revision: 1}}} {
		if ValidateMergeChanges(9, changes, invalid) == nil {
			t.Fatal("invalid proposed resolutions accepted")
		}
	}
	if ValidateMergeChanges(10, changes, wanted) == nil || changes[0].Status != "OPEN" || changes[1].Status != "OPEN" {
		t.Fatal("other author's changes resolved before commit")
	}
}
