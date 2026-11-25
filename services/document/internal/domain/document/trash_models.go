// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"time"
)

// purge.go：保留原实现，按业务合并文件。

// 普通已关闭KB中的TRASHED正文仍须按期清理；恢复另有ACTIVE限制。
// CLOSED资格还必须由调用方核原BLOCKED关闭与真实ACK，不能单凭枚举发证。
func ValidatePurgeParent(kb dto.KnowledgeBase) error {
	if kb.Lifecycle != "ACTIVE" && kb.Lifecycle != "CLOSED" {
		return Violation(StateConflict, "PURGE_PARENT_CHANGED")
	}
	return nil
}

// 普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用
// Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。
type PurgePlan struct {
	Version         uint8        `json:"version"`
	Operation       string       `json:"operation_id"`
	Actor           uint64       `json:"actor_id"`
	Automatic       bool         `json:"automatic"`
	Document        uint64       `json:"document_id"`
	Space           uint64       `json:"space_id"`
	KnowledgeBase   uint64       `json:"knowledge_base_id"`
	ExpectedControl uint64       `json:"expected_control_revision"`
	ClosingControl  uint64       `json:"closing_control_revision"`
	ACL             uint64       `json:"acl_revision"`
	Publication     uint64       `json:"publication_epoch"`
	Sequence        uint64       `json:"event_seq"`
	Scopes          []PurgeScope `json:"scopes"`
}

type PurgeScope struct {
	Key       string `json:"scope_key"`
	Epoch     uint64 `json:"epoch"`
	Operation string `json:"revocation_id"`
}

func purgeDerivedID(kind, input string) string {
	digest := sha256.Sum256([]byte(kind + "\n" + input))
	return kind + "_" + hex.EncodeToString(digest[:])
}

func PurgeRevocation(operation, scope string) string {
	return purgeDerivedID("purge_fence", operation+"\n"+scope)
}

func PurgeExpiryOperation(facts TrashFacts) string {
	deadline := ""
	if facts.PurgeAfter != nil {
		deadline = facts.PurgeAfter.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return purgeDerivedID("purge_expiry", facts.Document.Id+"\n"+facts.Document.ControlRevision+"\n"+deadline)
}

func ValidatePurgeIntent(facts TrashFacts, confirmed bool, expected string, automatic bool) error {
	if !confirmed {
		return Violation(InvalidRequest, "PURGE_CONFIRMATION_REQUIRED")
	}
	if facts.Document.Lifecycle != "TRASHED" {
		return Violation(StateConflict, "DOCUMENT_NOT_TRASHED")
	}
	if facts.Document.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if automatic && (facts.Now.IsZero() || facts.PurgeAfter == nil || facts.PurgeAfter.After(facts.Now)) {
		return Violation(StateConflict, "PURGE_NOT_EXPIRED")
	}
	return nil
}

func NewPurgePlan(operation string, actor uint64, automatic bool, facts TrashFacts, kb, aclHighWater, publicationHighWater uint64) (PurgePlan, error) {
	var p PurgePlan
	next, err := PlanTrashRestore(facts, aclHighWater, publicationHighWater)
	if err != nil {
		return p, err
	}
	doc, err := CanonicalID(facts.Document.Id)
	if err != nil {
		return p, err
	}
	space, err := CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return p, err
	}
	control, err := CanonicalRevision(facts.Document.ControlRevision)
	if err != nil {
		return p, err
	}
	p = PurgePlan{Version: 1, Operation: operation, Actor: actor, Automatic: automatic,
		Document: doc, Space: space, KnowledgeBase: kb, ExpectedControl: control,
		ClosingControl: next.Control, ACL: next.ACL, Publication: next.Publication, Sequence: next.Sequence}
	for _, scope := range []struct {
		key   string
		epoch uint64
	}{{"doc-acl:" + TextID(doc), p.ACL}, {"publication:" + TextID(doc), p.Publication}} {
		p.Scopes = append(p.Scopes, PurgeScope{Key: scope.key, Epoch: scope.epoch, Operation: PurgeRevocation(operation, scope.key)})
	}
	if !p.Valid() {
		return PurgePlan{}, Violation(InvalidRequest, "PURGE_PLAN_INVALID")
	}
	return p, nil
}

func (p PurgePlan) Valid() bool {
	if p.Version != 1 || !ValidOperationID(p.Operation) || p.Document == 0 || p.Space == 0 ||
		(p.Automatic && p.Actor != 0) || (!p.Automatic && p.Actor == 0) || p.ClosingControl == 0 ||
		p.ClosingControl != p.ExpectedControl+1 || p.ACL == 0 || p.Publication == 0 || p.Sequence == 0 || len(p.Scopes) != 2 {
		return false
	}
	for i, source := range p.Scopes {
		key, epoch := "doc-acl:"+TextID(p.Document), p.ACL
		if i == 1 {
			key, epoch = "publication:"+TextID(p.Document), p.Publication
		}
		if source.Key != key || source.Epoch != epoch || source.Operation != PurgeRevocation(p.Operation, key) {
			return false
		}
	}
	return true
}

func DecodePurgePlan(raw []byte) (PurgePlan, error) {
	var p PurgePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 8192 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || !p.Valid() {
		return PurgePlan{}, Violation(TemporarilyUnavailable, "PURGE_PLAN_INVALID")
	}
	return p, nil
}

// trash.go：保留原实现，按业务合并文件。

// TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。
// 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。
type TrashFacts struct {
	Document   dto.Document
	Sequence   uint64
	PurgeAfter *time.Time
	Now        time.Time
}

type TrashRestoreRevisions struct {
	Control, ACL, Publication, Visibility, Sequence uint64
}

// 预读只决定使用当前来源准备原号回放；不包含另一份fingerprint/结果解析，
// 真正重放必须交给标准Mutation持原operation gate核验并读取固定receipt。
type TrashRestoreReceipt struct {
	Operation, Kind, State, Path string
	Actor                        uint64
}

func ValidateTrashRestoreReceipt(receipt TrashRestoreReceipt, operation string, actor, document uint64) error {
	if receipt.Operation != operation || receipt.Actor != actor || receipt.Kind != "DOCUMENT_RESTORE" ||
		receipt.Path != "/v1/documents/"+TextID(document)+"/restore" {
		return Violation(StateConflict, "OPERATION_ID_CONFLICT")
	}
	if receipt.State != "SUCCEEDED" {
		return Violation(TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}
	return nil
}

// 回收资格来自现今仍有效的责任关系/上级管理来源，旧文档直接授权不用于复活。
// Responsible由调用方在真实组织成员proof下核验当前Doc负责关系；
// KnowledgeBaseAdmin还要求当前ACTIVE库，不能把离组或已关闭角色当作管理资格。
func CanManageTrash(facts PermissionFacts) bool {
	return facts.Responsible || facts.PersonalOwner || facts.OrganizationAdmin || facts.KnowledgeBaseAdmin
}

func ValidateTrashRestore(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, expected string) error {
	d := facts.Document
	if d.Lifecycle != "TRASHED" {
		return Violation(StateConflict, "DOCUMENT_NOT_TRASHED")
	}
	if space.Id != d.SpaceId || space.Lifecycle != "ACTIVE" {
		return Violation(StateConflict, "SPACE_NOT_ACTIVE")
	}
	if d.KnowledgeBaseId != nil {
		if kb == nil || kb.Id != *d.KnowledgeBaseId || kb.SpaceId != d.SpaceId {
			return Violation(TemporarilyUnavailable, "TRASH_KNOWLEDGE_BASE_UNKNOWN")
		}
		// 冻结接口没有迁移目标，不能把关闭库中的文档私自移入其他库或空间。
		if kb.Lifecycle != "ACTIVE" {
			return Violation(StateConflict, "KB_NOT_ACTIVE")
		}
	}
	if d.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if facts.Now.IsZero() || facts.PurgeAfter == nil {
		return Violation(StateConflict, "TRASH_RETENTION_UNKNOWN")
	}
	if !facts.PurgeAfter.After(facts.Now) {
		return Violation(StateConflict, "TRASH_RETENTION_EXPIRED")
	}
	for _, id := range []string{d.CurrentStructureId, d.CurrentStyleId, d.CurrentManifestId} {
		if _, err := CanonicalID(id); err != nil {
			return Violation(TemporarilyUnavailable, "TRASH_CONTENT_BINDING_UNKNOWN")
		}
	}
	return nil
}

// 关闭高水位可能已超出旧metadata，恢复必须同时越过两者。耗尽拒绝整笔事务，
// 不通过回绕或降低gate.max_epoch让迟到的旧输出/协作请求复活。
func PlanTrashRestore(facts TrashFacts, aclHighWater, publicationHighWater uint64) (TrashRestoreRevisions, error) {
	var next TrashRestoreRevisions
	values := []string{facts.Document.ControlRevision, facts.Document.AclRevision, facts.Document.PublicationEpoch, facts.Document.VisibilityRevision}
	current := make([]uint64, len(values))
	for i, text := range values {
		value, err := CanonicalID(text)
		if err != nil {
			return next, Violation(TemporarilyUnavailable, "TRASH_REVISION_INVALID")
		}
		current[i] = value
	}
	current[1] = max(current[1], aclHighWater)
	current[2] = max(current[2], publicationHighWater)
	for _, value := range append(current, facts.Sequence) {
		if value == math.MaxUint64 {
			return next, Violation(StateConflict, "TRASH_REVISION_EXHAUSTED")
		}
	}
	next = TrashRestoreRevisions{
		Control:     current[0] + 1,
		ACL:         current[1] + 1,
		Publication: current[2] + 1,
		Visibility:  current[3] + 1,
		Sequence:    facts.Sequence + 1,
	}
	return next, nil
}
