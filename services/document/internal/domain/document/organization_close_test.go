package document

import (
	"collaboration/platform"
	"encoding/json"
	"testing"
)

func TestOrganizationCloseCleanupRequiresFixedDocument(t *testing.T) {
	// Node发证只认原组织留存计划；不能追加个人文档或把旧账号身份绑定当证明。
	b := platform.OrganizationClosureBinding{Operation: "org-close-document", Organization: 7, User: 3, Revision: 2, MemberRevision: 4, Epoch: 5}
	retention := AccountRetentionPlan{Spaces: []uint64{8}, KnowledgeBases: []uint64{}, Documents: []AccountRetentionDocument{{Document: 11, Space: 8, Versions: []uint64{}, Resources: []uint64{}}}, Scopes: []AccountRetentionScope{}}
	for _, key := range []string{"doc-acl:11", "publication:11"} {
		retention.Scopes = append(retention.Scopes, AccountRetentionScope{Key: key, Epoch: 2, Operation: AccountRetentionRevocation(b.Operation, key)})
	}
	raw, _ := json.Marshal(OrganizationClosePlan{Binding: b, Retention: &retention})
	plan, err := DecodeOrganizationClosePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = plan.CleanupDocument(11); err != nil {
		t.Fatal(err)
	}
	if _, _, err = plan.CleanupDocument(12); err == nil {
		t.Fatal("原计划外文档不得发清理证明")
	}
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	value["binding"].(map[string]any)["identity_epoch"] = 1
	mutated, _ := json.Marshal(value)
	if _, err = DecodeOrganizationClosePlan(mutated); err == nil {
		t.Fatal("账号身份绑定不得混入独立组织关闭")
	}
}
