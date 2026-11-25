package platform

import (
	"strings"
	"testing"
)

func TestOrganizationClosurePageProtectsOriginalCursor(t *testing.T) {
	// 回复丢失后的原号重放必须保持摘要和递增文档游标；非法页不能推进持久进度。
	b := OrganizationClosureBinding{Operation: "org-close-boundary", Organization: 7, User: 3, Revision: 2, MemberRevision: 4, Epoch: 5}
	r := OrganizationClosurePageRequest{Binding: b, PlanDigest: strings.Repeat("a", 64), AfterDocument: "10"}
	p := OrganizationClosurePage{Binding: b, State: "CLEANING", PlanDigest: r.PlanDigest, Documents: []AccountClosureDocument{{Document: "11", Release: "0"}}, NextAfter: "11", Done: true}
	if !p.Matches(r) {
		t.Fatal("固定递增页面应可推进")
	}
	p.Documents[0].Document = "10"
	if p.Matches(r) {
		t.Fatal("重叠旧游标不得再次推进")
	}
	p.Documents[0].Document = "11"
	p.PlanDigest = strings.Repeat("b", 64)
	if p.Matches(r) {
		t.Fatal("更换原计划摘要不得推进")
	}
	p.PlanDigest = r.PlanDigest
	p.Done = false
	if p.Matches(r) {
		t.Fatal("未完成短页可能截断，必须拒绝")
	}
}
