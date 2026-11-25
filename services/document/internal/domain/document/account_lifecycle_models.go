// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	"collaboration/platform"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

// CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
// 拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。
func (p AccountRetirementPlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error) {
	return cleanupRetentionDocument(p.Retention, document, "ACCOUNT")
}

func cleanupRetentionDocument(retention *AccountRetentionPlan, document uint64, kind string) (AccountRetentionDocument, []AccountRetentionScope, error) {
	var item AccountRetentionDocument
	if retention == nil {
		return item, nil, errors.New(kind + "_RETENTION_PLAN_REQUIRED")
	}
	found := false
	for _, candidate := range retention.Documents {
		if candidate.Document == document {
			item, found = candidate, true
			break
		}
	}
	if !found {
		return item, nil, Violation(StateConflict, kind+"_DOCUMENT_NOT_IN_PLAN")
	}
	keys := map[string]bool{"doc-acl:" + TextID(document): true, "publication:" + TextID(document): true}
	if item.KnowledgeBase != 0 {
		keys["kb-acl:"+TextID(item.KnowledgeBase)] = true
	}
	var scopes []AccountRetentionScope
	for _, scope := range retention.Scopes {
		if keys[scope.Key] {
			scopes = append(scopes, scope)
			delete(keys, scope.Key)
		}
	}
	if len(keys) != 0 {
		return item, nil, errors.New(kind + "_DOCUMENT_SCOPE_MISSING")
	}
	return item, scopes, nil
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

// 留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。
type AccountHistoryRevision struct {
	ID, Parent, Structure, Style uint64
}

func KeepHistoryAncestors(rows []AccountHistoryRevision, keep map[uint64]bool) error {
	byID := map[uint64]AccountHistoryRevision{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for id := range keep {
		seen := map[uint64]bool{}
		for id != 0 {
			row, found := byID[id]
			if !found || seen[id] {
				return errors.New("ACCOUNT_HISTORY_DEPENDENCY_INVALID")
			}
			seen[id], keep[id] = true, true
			id = row.Parent
		}
	}
	return nil
}

// 不改不可变parent_id：从真实叶节点删除，不因ID大小猜历史先后。
func HistoryDeletionOrder(rows []AccountHistoryRevision, keep map[uint64]bool) ([]uint64, error) {
	remaining := map[uint64]AccountHistoryRevision{}
	for _, row := range rows {
		remaining[row.ID] = row
	}
	order := []uint64{}
	for len(remaining) > 0 {
		parents := map[uint64]bool{}
		for _, row := range remaining {
			if row.Parent != 0 {
				parents[row.Parent] = true
			}
		}
		progress := false
		for id := range remaining {
			if parents[id] {
				continue
			}
			if !keep[id] {
				order = append(order, id)
			}
			delete(remaining, id)
			progress = true
		}
		if !progress {
			return nil, errors.New("ACCOUNT_HISTORY_DEPENDENCY_CYCLE")
		}
	}
	return order, nil
}

// account_retention.go：保留原实现，按业务合并文件。

// 保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。
// Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。
type AccountRetentionDocument struct {
	Document      uint64   `json:"document_id"`
	Space         uint64   `json:"space_id"`
	KnowledgeBase uint64   `json:"knowledge_base_id"`
	Release       uint64   `json:"release_id"`
	Manifest      uint64   `json:"manifest_id"`
	Structure     uint64   `json:"structure_id"`
	Style         uint64   `json:"style_id"`
	Versions      []uint64 `json:"version_ids"`
	Resources     []uint64 `json:"resource_ids"`
}

type AccountRetentionScope struct {
	Key       string `json:"scope_key"`
	Epoch     uint64 `json:"epoch"`
	Operation string `json:"revocation_id"`
}

type AccountRetentionPlan struct {
	Spaces         []uint64                   `json:"space_ids"`
	KnowledgeBases []uint64                   `json:"knowledge_base_ids"`
	Documents      []AccountRetentionDocument `json:"documents"`
	Scopes         []AccountRetentionScope    `json:"scopes"`
}

// 子屏障使用固定原号派生，和Account身份屏障及用户操作回执互不混用。
func AccountRetentionRevocation(operation, scope string) string {
	digest := sha256.Sum256([]byte("ACCOUNT_RETENTION\n" + operation + "\n" + scope))
	return "retention-" + hex.EncodeToString(digest[:])
}

func orderedPositive(ids []uint64) bool {
	if ids == nil {
		return false
	}
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] >= id) {
			return false
		}
	}
	return true
}

func (p AccountRetentionPlan) Valid(operation string) bool {
	if !orderedPositive(p.Spaces) || !orderedPositive(p.KnowledgeBases) || p.Documents == nil || p.Scopes == nil {
		return false
	}
	expected := map[string]bool{}
	for _, kb := range p.KnowledgeBases {
		expected["kb-acl:"+TextID(kb)] = true
	}
	for i, d := range p.Documents {
		if d.Document == 0 || !slices.Contains(p.Spaces, d.Space) || (d.KnowledgeBase != 0 && !slices.Contains(p.KnowledgeBases, d.KnowledgeBase)) || (i > 0 && p.Documents[i-1].Document >= d.Document) || !orderedPositive(d.Versions) || !orderedPositive(d.Resources) {
			return false
		}
		if d.Release == 0 {
			if d.Manifest != 0 || d.Structure != 0 || d.Style != 0 || len(d.Versions) != 0 || len(d.Resources) != 0 {
				return false
			}
		} else if d.Manifest == 0 || d.Structure == 0 || d.Style == 0 {
			return false
		}
		expected["doc-acl:"+TextID(d.Document)] = true
		expected["publication:"+TextID(d.Document)] = true
	}
	for i, scope := range p.Scopes {
		if !expected[scope.Key] || !platform.ValidDocumentScope(scope.Key) || scope.Epoch == 0 || scope.Operation != AccountRetentionRevocation(operation, scope.Key) || (i > 0 && p.Scopes[i-1].Key >= scope.Key) {
			return false
		}
		delete(expected, scope.Key)
	}
	return len(expected) == 0
}

// account_retirement.go：保留原实现，按业务合并文件。

// AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。
// ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。
type AccountRetirementBinding platform.AccountClosureBinding

type AccountRetirementPlan struct {
	Binding                AccountRetirementBinding `json:"binding"`
	ReservedKnowledgeBases []uint64                 `json:"reserved_knowledge_bases"`
	Retention              *AccountRetentionPlan    `json:"retention,omitempty"`
}

type AccountRetirementResult struct {
	State                  string
	RequiredKnowledgeBases []uint64
	ReservedKnowledgeBases []uint64
}

func (b AccountRetirementBinding) Valid() bool {
	return platform.ValidateAccountClosureBinding(platform.AccountClosureBinding(b))
}

func (b AccountRetirementBinding) Same(other AccountRetirementBinding) bool {
	return platform.EqualAccountClosureBinding(platform.AccountClosureBinding(b), platform.AccountClosureBinding(other))
}

func DecodeAccountRetirementPlan(raw []byte) (AccountRetirementPlan, error) {
	var plan AccountRetirementPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || plan.ReservedKnowledgeBases == nil {
		return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
	}
	previous := uint64(0)
	for _, id := range plan.ReservedKnowledgeBases {
		if id <= previous {
			return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
		}
		previous = id
	}
	if plan.Retention != nil && !plan.Retention.Valid(plan.Binding.Operation) {
		return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
	}
	return plan, nil
}
