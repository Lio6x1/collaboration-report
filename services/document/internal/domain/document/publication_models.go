// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"math"
)

// public_reading.go：保留原实现，按业务合并文件。

// PublicationFacts是Doc私有库的一致快照。公开受众和内部READ是独立资格，
// PUBLIC不会给匿名或普通空间成员增加草稿、MAIN及历史权限。
type PublicationFacts struct {
	Document      dto.Document
	Space         dto.Space
	KnowledgeBase *dto.KnowledgeBase
	Sequence      uint64
}

func PublicPolicy(facts PublicationFacts) (string, error) {
	valid := func(value string) bool {
		return value == "PUBLIC" || value == "SPACE_MEMBERS" || value == "COLLABORATORS"
	}
	policy := facts.Document.VisibilityPolicy
	if !valid(policy) {
		return "", Violation(TemporarilyUnavailable, "PUBLIC_POLICY_UNKNOWN")
	}
	if facts.KnowledgeBase != nil {
		if !valid(facts.KnowledgeBase.VisibilityPolicy) {
			return "", Violation(TemporarilyUnavailable, "PUBLIC_POLICY_UNKNOWN")
		}
		policy = EffectiveVisibility(facts.KnowledgeBase.VisibilityPolicy, policy)
	}
	return policy, nil
}

// 留存对象只可能继续展示已经固定的公开版；其他关闭、回收站或未知状态都不回退历史。
func HasCurrentPublication(facts PublicationFacts, policy string) bool {
	document := facts.Document
	if document.PublicationState != "PUBLISHED" || document.CurrentReleaseId == nil {
		return false
	}
	retained := false
	states := []string{document.Lifecycle, facts.Space.Lifecycle}
	if facts.KnowledgeBase != nil {
		states = append(states, facts.KnowledgeBase.Lifecycle)
	}
	for _, state := range states {
		if state == "PUBLIC_RETAINED" {
			retained = true
		} else if state != "ACTIVE" {
			return false
		}
	}
	return !retained || policy == "PUBLIC"
}

func ValidatePublicAudience(policy string, authenticated, member, internal bool) error {
	if policy == "PUBLIC" {
		return nil
	}
	if !authenticated {
		return Violation(SessionRequired, "SESSION_REQUIRED")
	}
	if (policy == "SPACE_MEMBERS" && member) || (policy == "COLLABORATORS" && internal) {
		return nil
	}
	return Violation(PermissionDenied, "PUBLIC_DOCUMENT_NOT_ALLOWED")
}

type FixedRelease struct {
	Release                 dto.Release
	Title, Structure, Style string
}

// publication.go：保留原实现，按业务合并文件。

// PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖，
// 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。
type PublicationIntent struct {
	Operation   string `json:"operation"`
	Kind        string `json:"kind"`
	Publication uint64 `json:"publication"`
	Control     uint64 `json:"control"`
	Release     uint64 `json:"release"`
}

func ParseReleaseCreate(input dto.ReleaseCreate) (PublicationIntent, error) {
	intent := PublicationIntent{Operation: input.OperationId, Kind: "RELEASE_CREATE"}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	var err error
	if intent.Publication, err = CanonicalRevision(input.ExpectedPublicationRevision); err != nil {
		return intent, err
	}
	intent.Control, err = CanonicalRevision(input.ExpectedControlRevision)
	return intent, err
}

func ParsePublication(input dto.Publication) (PublicationIntent, error) {
	intent := PublicationIntent{Operation: input.OperationId, Kind: input.Action}
	if !ValidOperationID(intent.Operation) ||
		(input.Action != "SWITCH" && input.Action != "UNPUBLISH") ||
		(input.Action == "SWITCH" && input.ReleaseId == nil) ||
		(input.Action == "UNPUBLISH" && input.ReleaseId != nil) {
		return intent, Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	var err error
	if intent.Publication, err = CanonicalRevision(input.ExpectedPublicationRevision); err != nil {
		return intent, err
	}
	if input.Action == "SWITCH" {
		intent.Release, err = CanonicalID(*input.ReleaseId)
	}
	return intent, err
}

func ValidatePublication(permissions Permissions, current dto.Document, intent PublicationIntent) error {
	// 先检查管理资格，不能借一个过期发布页面探知当前版本或推进新水位。
	if !permissions.Manage {
		return Violation(PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	if current.Lifecycle != "ACTIVE" {
		return Violation(StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	if current.PublicationEpoch != TextID(intent.Publication) {
		return Violation(StateConflict, "PUBLICATION_CHANGED")
	}
	if intent.Publication == math.MaxUint64 {
		return Violation(StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
	switch intent.Kind {
	case "RELEASE_CREATE":
		if current.ControlRevision != TextID(intent.Control) {
			return Violation(StateConflict, "MAIN_CONTROL_CHANGED")
		}
	case "SWITCH":
		if intent.Release == 0 {
			return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
		}
		if current.PublicationState == "PUBLISHED" && current.CurrentReleaseId != nil && *current.CurrentReleaseId == TextID(intent.Release) {
			return Violation(StateConflict, "PUBLICATION_ALREADY_CURRENT")
		}
	case "UNPUBLISH":
		if current.PublicationState != "PUBLISHED" || current.CurrentReleaseId == nil {
			return Violation(StateConflict, "PUBLICATION_NOT_PUBLISHED")
		}
	default:
		return Violation(InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	return nil
}

// 正式指针和outbox序号在同一事务推进；任何一个耗尽都使整个动作失败，
// 不能先创建正式版，再因序号溢出留下没有对应事件的业务结果。
func NextPublicationSequences(epoch, event uint64) (uint64, uint64, error) {
	if epoch == math.MaxUint64 || event == math.MaxUint64 {
		return 0, 0, Violation(StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
	return epoch + 1, event + 1, nil
}

// visibility.go：保留原实现，按业务合并文件。

// 可见范围只限制正式版受众，不产生主线/草稿资格；设置PUBLIC也不能自动发布。
func EffectiveVisibility(knowledgeBase, document string) string {
	rank := map[string]int{"PUBLIC": 0, "SPACE_MEMBERS": 1, "COLLABORATORS": 2}
	if rank[knowledgeBase] > rank[document] {
		return knowledgeBase
	}
	return document
}

type KnowledgeBaseChild struct {
	ID, Epoch  uint64
	HasRelease bool
}
