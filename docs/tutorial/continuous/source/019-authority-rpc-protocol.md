# packages/platform/authority_rpc_protocol.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/019-authority-rpc-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`AccountDocumentCleanupRequest`、`AccountDocumentCleanupContext`、`AccountDocumentCleanupReceipt`、`canonicalCleanupID`、`Valid`、`NewAccountDocumentCleanupContext`、`AccountNotificationEvent`、`AccountNotificationEventsResponse`、`AccountNotificationAckRequest`、`AccountNotificationAckResponse`、`DraftBodyProtection`、`NotificationAdministratorsRequest`、`NotificationAdministratorsResponse`、`NotificationRecipientsRequest`、`NotificationRecipientSource`、`NotificationRecipient`、`NotificationRecipientsResponse`、`OrganizationClosureBinding`、`Scope`、`OrganizationClosureConfirmation`、`OrganizationClosureState`、`OrganizationClosurePageRequest`、`OrganizationClosurePage`、`Matches`、`PersonalTransferStorageClosure`、`PersonalTransferStorageRequest`、`PersonalTransferStorageMeasurement`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

// Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。
type AccountDocumentCleanupRequest struct {
	Operation string `json:"operation_id"`
	Document  string `json:"document_id"`
}

type AccountDocumentCleanupContext struct {
	AccountDocumentCleanupRequest
	Scope      string `json:"scope_key"`
	Epoch      string `json:"closed_epoch"`
	Revocation string `json:"revocation_id"`
	Digest     string `json:"plan_digest"`
}

// CLEANED是Node已持久清除私有工作区的回执，不等同整个账号注销完成。
type AccountDocumentCleanupReceipt struct {
	AccountDocumentCleanupContext
	State string `json:"state"`
}

func canonicalCleanupID(value string) bool {
	id, err := strconv.ParseUint(value, 10, 64)
	return err == nil && id != 0 && strconv.FormatUint(id, 10) == value
}

func (r AccountDocumentCleanupRequest) Valid() bool {
	return accountClosureOperation.MatchString(r.Operation) && canonicalCleanupID(r.Document)
}

func NewAccountDocumentCleanupContext(r AccountDocumentCleanupRequest, epoch uint64, revocation string) AccountDocumentCleanupContext {
	c := AccountDocumentCleanupContext{AccountDocumentCleanupRequest: r, Scope: "doc-acl:" + r.Document, Epoch: strconv.FormatUint(epoch, 10), Revocation: revocation}
	digest := sha256.Sum256([]byte("ACCOUNT_DOCUMENT_CLEANUP\n" + c.Operation + "\n" + c.Document + "\n" + c.Scope + "\n" + c.Epoch + "\n" + c.Revocation))
	c.Digest = hex.EncodeToString(digest[:])
	return c
}

func (c AccountDocumentCleanupContext) Valid() bool {
	if !c.AccountDocumentCleanupRequest.Valid() || !canonicalCleanupID(c.Epoch) || !accountClosureOperation.MatchString(c.Revocation) {
		return false
	}
	epoch, _ := strconv.ParseUint(c.Epoch, 10, 64)
	expected := NewAccountDocumentCleanupContext(c.AccountDocumentCleanupRequest, epoch, c.Revocation)
	return c == expected
}

// account_notification.go：保留原实现，按业务合并文件。

// Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。
type AccountNotificationEvent struct {
	ID             uint64 `json:"id"`
	RecipientID    uint64 `json:"recipient_id"`
	OrganizationID uint64 `json:"organization_id"`
	EventID        string `json:"event_id"`
	Kind           string `json:"kind"`
}

type AccountNotificationEventsResponse struct {
	Events []AccountNotificationEvent `json:"events"`
}

type AccountNotificationAckRequest struct {
	IDs []uint64 `json:"ids"`
}

type AccountNotificationAckResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// draft_body_capacity.go：保留原实现，按业务合并文件。

// DraftBodyProtection 是内部实施扩展；原 protobuf 请求仍由 protojson 解码，
// 不复制 EDIT 授权契约，也不改动冻结的 proto 字节。计量只由 Node 隔离候选产生。
type DraftBodyProtection struct {
	Protection json.RawMessage `json:"protection"`
	BodyBytes  string          `json:"body_bytes"`
	BodyHash   string          `json:"body_hash"`
}

// notification_administrators.go：保留原实现，按业务合并文件。

// 管理员目录仅产生候选人；投递前必须重新调用接收资格核验，不能把目录当授权凭据。
type NotificationAdministratorsRequest struct {
	OrganizationID uint64 `json:"organization_id"`
	After          uint64 `json:"after"`
}

type NotificationAdministratorsResponse struct {
	OrganizationID uint64   `json:"organization_id"`
	UserIDs        []uint64 `json:"user_ids"`
	NextAfter      uint64   `json:"next_after"`
}

// notification_recipient.go：保留原实现，按业务合并文件。

// 通知收件资格只在服务间传递，不是用户会话，也不能用于发起用户修改。
type NotificationRecipientsRequest struct {
	OrganizationID uint64   `json:"organization_id"`
	UserIDs        []uint64 `json:"user_ids"`
}

type NotificationRecipientSource struct {
	ScopeKey string `json:"scope_key"`
	Epoch    uint64 `json:"epoch"`
}

type NotificationRecipient struct {
	UserID          uint64                        `json:"user_id"`
	Eligible        bool                          `json:"eligible"`
	IdentityEpoch   uint64                        `json:"identity_epoch"`
	CredentialEpoch uint64                        `json:"credential_epoch"`
	MemberEpoch     uint64                        `json:"member_epoch"`
	Role            string                        `json:"role"`
	OrgOwner        bool                          `json:"org_owner"`
	Sources         []NotificationRecipientSource `json:"sources"`
}

type NotificationRecipientsResponse struct {
	OrganizationID uint64                  `json:"organization_id"`
	Recipients     []NotificationRecipient `json:"recipients"`
	ExpiresUnixMS  int64                   `json:"expires_unix_ms"`
}

// organization_close.go：保留原实现，按业务合并文件。

// OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。
// Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。
type OrganizationClosureBinding struct {
	Operation      string `json:"operation_id"`
	Organization   uint64 `json:"organization_id"`
	User           uint64 `json:"user_id"`
	Revision       uint64 `json:"revision"`
	MemberRevision uint64 `json:"member_revision"`
	Epoch          uint64 `json:"member_epoch"`
}

func (b OrganizationClosureBinding) Valid() bool {
	return accountClosureOperation.MatchString(b.Operation) && b.Organization > 0 && b.User > 0 && b.Revision > 0 && b.Revision < ^uint64(0) && b.MemberRevision > 0 && b.MemberRevision < ^uint64(0) && b.Epoch > 0 && b.Epoch < ^uint64(0)
}

func (b OrganizationClosureBinding) Scope() string {
	return "org-member:" + strconv.FormatUint(b.Organization, 10) + ":" + strconv.FormatUint(b.User, 10)
}

type OrganizationClosureConfirmation struct {
	Confirmed bool `json:"confirmed"`
}
type OrganizationClosureState struct {
	State string `json:"state"`
}
type OrganizationClosurePageRequest struct {
	Binding       OrganizationClosureBinding `json:"binding"`
	PlanDigest    string                     `json:"plan_digest"`
	AfterDocument string                     `json:"after_document"`
}
type OrganizationClosurePage struct {
	Binding    OrganizationClosureBinding `json:"binding"`
	State      string                     `json:"state"`
	PlanDigest string                     `json:"plan_digest"`
	Documents  []AccountClosureDocument   `json:"documents"`
	NextAfter  string                     `json:"next_after"`
	Done       bool                       `json:"done"`
}

// 固定摘要、严格递增文档号和已保存游标必须同时匹配；分页不接受截断集合。
func (p OrganizationClosurePage) Matches(r OrganizationClosurePageRequest) bool {
	digest, err := hex.DecodeString(p.PlanDigest)
	if !r.Binding.Valid() || p.Binding != r.Binding || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != p.PlanDigest || (r.PlanDigest != "" && r.PlanDigest != p.PlanDigest) || p.Documents == nil || len(p.Documents) > 100 || (p.State != "CLEANING" && p.State != "SUCCEEDED") {
		return false
	}
	var after uint64
	if r.AfterDocument != "" {
		if !canonicalCleanupID(r.AfterDocument) || r.PlanDigest == "" {
			return false
		}
		after, _ = strconv.ParseUint(r.AfterDocument, 10, 64)
	}
	next := r.AfterDocument
	for _, item := range p.Documents {
		if !canonicalCleanupID(item.Document) {
			return false
		}
		id, _ := strconv.ParseUint(item.Document, 10, 64)
		if id <= after || (item.Release != "0" && !canonicalCleanupID(item.Release)) {
			return false
		}
		after, next = id, item.Document
	}
	return p.NextAfter == next && (p.Done || len(p.Documents) == 100)
}

// personal_transfer_storage.go：保留原实现，按业务合并文件。

// 专用内部计量只跨mTLS传固定原号、关闭来源和字节事实；不会转移正文或发出写入权限。
type PersonalTransferStorageClosure struct {
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      string `json:"epoch"`
}

type PersonalTransferStorageRequest struct {
	Operation  string                           `json:"operation_id"`
	Document   string                           `json:"document_id"`
	Generation string                           `json:"generation"`
	Closures   []PersonalTransferStorageClosure `json:"closures"`
}

type PersonalTransferStorageMeasurement struct {
	Operation    string `json:"operation_id"`
	Document     string `json:"document_id"`
	Generation   string `json:"generation"`
	Bytes        string `json:"storage_bytes"`
	Digest       string `json:"snapshot_digest"`
	SourceClosed bool   `json:"source_closed"`
}
````

UTF-8/LF全文SHA-256：`4548186d17923f2071857315b26aa1e13a8644e0762071cfe8d3e6c03ad38e8a`。[参考实现](../../../../packages/platform/authority_rpc_protocol.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
