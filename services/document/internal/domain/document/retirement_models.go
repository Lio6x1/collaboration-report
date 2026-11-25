// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"time"
)

// departure.go：保留原实现，按业务合并文件。

// DepartureBinding绑定实际组织、成员、操作编号与原身份代次。
type DepartureBinding struct {
	Org, User uint64
	ID        string
	Epoch     uint64
}

func (b DepartureBinding) Scope() string { return "org-member:" + TextID(b.Org) + ":" + TextID(b.User) }

type DepartureGate struct {
	Epoch            uint64
	State, Operation string
}
type DepartureObservation struct {
	Scope, Operation string
	Epoch            uint64
}

// departure_notification.go：保留原实现，按业务合并文件。

const DepartureNotificationKind = "RESPONSIBLE_DEPARTURE_NOTIFICATION"

// DepartureNotification 固定离组时的文档范围和真实知识库管理员；后续补任负责人、
// 成员重新入组或者门禁代次推进，都不能改写这次历史事件的收件集合。
type DepartureNotification struct {
	Document      uint64   `json:"document_id"`
	KnowledgeBase uint64   `json:"knowledge_base_id"`
	Organization  uint64   `json:"organization_id"`
	Departed      uint64   `json:"departed_user_id"`
	Epoch         uint64   `json:"epoch"`
	Operation     string   `json:"operation_id"`
	Event         string   `json:"event_id"`
	Recipients    []uint64 `json:"recipients"`
	DeliveryAfter uint64   `json:"delivery_after"`
}

func DepartureNotificationEvent(operation string, document uint64) string {
	digest := sha256.Sum256([]byte(operation + ":" + TextID(document)))
	return "departure-" + hex.EncodeToString(digest[:])
}

// DecodeDepartureNotification 严格拒绝未知字段、非升序集合及不在原集合中的游标。
// 数字直接解码为uint64，避免通过浮点数丢失身份精度。
func DecodeDepartureNotification(raw []byte) (DepartureNotification, error) {
	var event DepartureNotification
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	invalid := errors.New("DEPARTURE_NOTIFICATION_BINDING_INVALID")
	if decoder.Decode(&event) != nil || decoder.Decode(new(any)) != io.EOF {
		return event, invalid
	}
	if event.Document == 0 || event.KnowledgeBase == 0 || event.Organization == 0 || event.Departed == 0 || event.Epoch == 0 || !ValidOperationID(event.Operation) || event.Event != DepartureNotificationEvent(event.Operation, event.Document) || event.Recipients == nil {
		return event, invalid
	}
	previous, cursorFound := uint64(0), event.DeliveryAfter == 0
	for _, recipient := range event.Recipients {
		if recipient <= previous || recipient == event.Departed {
			return event, invalid
		}
		previous = recipient
		cursorFound = cursorFound || recipient == event.DeliveryAfter
	}
	if !cursorFound {
		return event, invalid
	}
	return event, nil
}

func SameDepartureNotification(left, right DepartureNotification) bool {
	left.DeliveryAfter, right.DeliveryAfter = 0, 0
	return reflect.DeepEqual(left, right)
}

// organization_close.go：保留原实现，按业务合并文件。

// 仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。
type OrganizationClosePlan struct {
	Binding   platform.OrganizationClosureBinding `json:"binding"`
	Retention *AccountRetentionPlan               `json:"retention"`
}

func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error) {
	var plan OrganizationClosePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&plan) != nil || d.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || plan.Retention == nil || !plan.Retention.Valid(plan.Binding.Operation) {
		return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
	}
	return plan, nil
}

// organization_close_document.go：保留原实现，按业务合并文件。

// CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
// 拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。
func (p OrganizationClosePlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error) {
	return cleanupRetentionDocument(p.Retention, document, "ORGANIZATION")
}

// personal_transfer.go：保留原实现，按业务合并文件。

type PersonalTransfer struct {
	ID, Document, SourceUser, TargetUser, SourceSpace, ExpectedControl, Revision uint64
	Title, State, OfferOperation, AcceptOperation                                string
	Expires                                                                      time.Time
	Now                                                                          time.Time
	Plan                                                                         PersonalTransferPlan
}

// 固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。
type PersonalTransferPlan struct {
	Operation                                                                                    string `json:"operation_id"`
	Transfer, Document, SourceUser, TargetUser, SourceSpace, TargetSpace, TargetKB               uint64
	ExpectedControl, ClosingControl, ACL, Publication, BodyBytes, SourceBodyBytes, ReservedBytes uint64
	Resources                                                                                    []TransferResource
	Scopes                                                                                       []PurgeScope
	SourceIdentity, ReceiverIdentity, ReceiverSession                                            []byte
	NodeBytes                                                                                    uint64
	NodeMeasurement                                                                              []byte
}

type TransferResource struct {
	ID, Size     uint64
	TargetCharge bool
}

func PersonalTransferFence(operation, scope string) string {
	sum := sha256.Sum256([]byte(operation + "\n" + scope))
	return "personal_fence_" + hex.EncodeToString(sum[:])
}

func ValidatePersonalTransferAction(offer PersonalTransfer, actor, expected uint64, action string) error {
	if offer.Revision != expected {
		return Violation(StateConflict, "TRANSFER_REVISION_CHANGED")
	}
	if offer.State != "PENDING" {
		return Violation(StateConflict, "TRANSFER_NOT_PENDING")
	}
	if !offer.Expires.After(offer.Now) {
		return Violation(StateConflict, "TRANSFER_EXPIRED")
	}
	if (action == "CANCEL" && actor != offer.SourceUser) || (action != "CANCEL" && actor != offer.TargetUser) {
		return Violation(PermissionDenied, "TRANSFER_PARTICIPANT_REQUIRED")
	}
	return nil
}

func TransferCapacity(body, resources uint64) (uint64, error) {
	if resources > ^uint64(0)-body {
		return 0, Violation(StateConflict, "TRANSFER_CAPACITY_OVERFLOW")
	}
	return body + resources, nil
}

func (p PersonalTransferPlan) Valid() bool {
	if !ValidOperationID(p.Operation) || p.Transfer == 0 || p.Document == 0 || p.SourceUser == 0 || p.TargetUser == 0 || p.SourceUser == p.TargetUser || p.SourceSpace == 0 || p.TargetSpace == 0 || p.SourceSpace == p.TargetSpace || p.TargetKB == 0 || p.ExpectedControl == ^uint64(0) || p.ClosingControl != p.ExpectedControl+1 || p.ClosingControl == ^uint64(0) || p.ACL == 0 || p.ACL == ^uint64(0) || p.Publication == 0 || p.Publication == ^uint64(0) || len(p.Scopes) != 2 || len(p.ReceiverSession) != 32 || len(p.SourceIdentity) == 0 || len(p.ReceiverIdentity) == 0 {
		return false
	}
	for i, s := range p.Scopes {
		key, epoch := "doc-acl:"+TextID(p.Document), p.ACL
		if i == 1 {
			key, epoch = "publication:"+TextID(p.Document), p.Publication
		}
		if s.Key != key || s.Epoch != epoch || s.Operation != PersonalTransferFence(p.Operation, key) {
			return false
		}
	}
	var previous, total uint64
	for _, r := range p.Resources {
		if r.ID == 0 || r.ID <= previous {
			return false
		}
		previous = r.ID
		if r.TargetCharge {
			var err error
			total, err = TransferCapacity(total, r.Size)
			if err != nil {
				return false
			}
		}
	}
	total, err := TransferCapacity(total, p.BodyBytes)
	if err != nil {
		return false
	}
	total, err = TransferCapacity(total, p.NodeBytes)
	return err == nil && total == p.ReservedBytes && (len(p.NodeMeasurement) == 0 || len(p.NodeMeasurement) == 32)
}

// retained_governance.go：保留原实现，按业务合并文件。

const RetainedGovernancePurpose = "RETAINED_GOVERNANCE"

// 平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。
// PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。
type RetainedGovernancePlan struct {
	Purge            PurgePlan `json:"purge"`
	Action           string    `json:"action"`
	Release          uint64    `json:"release_id"`
	Resources        []uint64  `json:"resources"`
	KnowledgeBaseACL uint64    `json:"knowledge_base_acl"`
}

func ValidateRetainedGovernance(facts TrashFacts, space dto.Space, kb *dto.KnowledgeBase, action, expected string) error {
	d := facts.Document
	if action != "UNPUBLISH" && action != "PURGE" {
		return Violation(InvalidRequest, "RETAINED_ACTION_INVALID")
	}
	// 原负责人已解除、父空间已关闭协作能力才是孤立留存；普通ACTIVE文档即使
	// 对外公开，也绝不因平台管理身份获得代替负责人操作的入口。
	if d.Lifecycle != "PUBLIC_RETAINED" || d.ResponsibleUserId != nil || !d.MainLocked ||
		space.Id != d.SpaceId || space.Lifecycle != "PUBLIC_RETAINED" {
		return Violation(PermissionDenied, "ISOLATED_PUBLIC_RETAINED_REQUIRED")
	}
	if d.KnowledgeBaseId != nil && (kb == nil || kb.Id != *d.KnowledgeBaseId || kb.SpaceId != space.Id || kb.Lifecycle != "PUBLIC_RETAINED") {
		return Violation(PermissionDenied, "ISOLATED_PUBLIC_RETAINED_REQUIRED")
	}
	if d.ControlRevision != expected {
		return Violation(StateConflict, "CONTROL_REVISION_MISMATCH")
	}
	if d.PublicationState == "PUBLISHED" && d.CurrentReleaseId != nil {
		policy, err := PublicPolicy(PublicationFacts{Document: d, Space: space, KnowledgeBase: kb})
		if err != nil {
			return err
		}
		if policy != "PUBLIC" {
			return Violation(StateConflict, "RETAINED_PUBLIC_BINDING_CHANGED")
		}
		return nil
	}
	// 已撤下的孤立内容仍可永久删除，但不能重新撤下以制造一个新关闭成功。
	if action == "PURGE" && d.PublicationState == "UNPUBLISHED" && d.CurrentReleaseId == nil {
		return nil
	}
	return Violation(StateConflict, "RETAINED_PUBLICATION_CHANGED")
}

func (p RetainedGovernancePlan) Valid() bool {
	if !p.Purge.Valid() || p.Purge.Automatic || (p.Action != "UNPUBLISH" && p.Action != "PURGE") ||
		(p.Action == "UNPUBLISH" && p.Release == 0) || p.Resources == nil || !slices.IsSorted(p.Resources) || (p.Purge.KnowledgeBase != 0 && p.KnowledgeBaseACL == 0) {
		return false
	}
	for i, id := range p.Resources {
		if id == 0 || (i > 0 && p.Resources[i-1] == id) {
			return false
		}
	}
	return true
}

func DecodeRetainedGovernancePlan(raw []byte) (RetainedGovernancePlan, error) {
	var p RetainedGovernancePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 1<<20 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || !p.Valid() {
		return p, Violation(TemporarilyUnavailable, "RETAINED_PLAN_INVALID")
	}
	return p, nil
}

func RetainedGovernanceKind(action string) string {
	return "RETAINED_PUBLIC_" + action
}
