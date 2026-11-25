# services/account/internal/domain/account/organization_models.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/042-organization-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`OrganizationApplication`、`OrganizationInvitation`、`InvitationShare`、`InvitationIdentity`、`InvitationCodec`、`NewInvitationCodec`、`cipher`、`Encrypt`、`Decrypt`、`Organization`、`OrganizationEntry`、`OrganizationMember`、`ValidEntityID`、`OrganizationClosePlan`、`DecodeOrganizationClosePlan`、`OrganizationCloseEligible`、`NormalizeOrganizationName`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	"collaboration/platform"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

type OrganizationApplication struct {
	ID                    string
	OrganizationID        string
	ApplicantID           string
	Reason                string
	State                 string
	Revision              string
	SourceInvitationID    *string
	InvitationValidatedAt *time.Time
}

// invitation.go：保留原实现，按业务合并文件。

type OrganizationInvitation struct {
	ID             string
	OrganizationID string
	State          string
	Expires        time.Time
	Revision       string
}

type InvitationShare struct {
	ID      string
	Token   string
	Expires time.Time
}

type InvitationIdentity struct {
	ID             string
	OrganizationID string
}

// invitation_codec.go：保留原实现，按业务合并文件。

// InvitationCodec使用组织ID作附加认证数据，密文不能挪到另一组织解密。
type InvitationCodec struct{ key []byte }

func NewInvitationCodec(key []byte) *InvitationCodec {
	return &InvitationCodec{key: append([]byte(nil), key...)}
}

func (c *InvitationCodec) cipher() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (c *InvitationCodec) Encrypt(token, organization string) ([]byte, error) {
	aead, err := c.cipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(token), []byte(organization)), nil
}

func (c *InvitationCodec) Decrypt(value []byte, organization string) (string, error) {
	aead, err := c.cipher()
	if err != nil || len(value) < aead.NonceSize() {
		return "", RuleViolation{Kind: TemporarilyUnavailable, Code: "INVITATION_KEY_UNAVAILABLE"}
	}
	plain, err := aead.Open(nil, value[:aead.NonceSize()], value[aead.NonceSize():], []byte(organization))
	return string(plain), err
}

// organization.go：保留原实现，按业务合并文件。

type Organization struct {
	CurrentMember  *OrganizationMember
	ID             string
	Name           string
	OwnerID        string
	Lifecycle      string
	Revision       string
	MemberRevision string
}

type OrganizationEntry struct {
	Organization Organization
	MemberEpoch  uint64
}

type OrganizationMember struct {
	UserID        string
	DisplayName   string
	Role          string
	State         string
	Revision      string
	Epoch         uint64
	IdentityEpoch uint64
	// 仅用于成员目录的状态说明，实际建库仍由Identity重新准入。
	KnowledgeBaseCreation string
}

func ValidEntityID(value string) bool {
	number, err := strconv.ParseUint(value, 10, 64)
	return err == nil && number > 0 && strconv.FormatUint(number, 10) == value
}

// organization_close.go：保留原实现，按业务合并文件。

// Plan 保存在原 transition；游标只有在对应 Doc 清理证明返回后推进。
type OrganizationClosePlan struct {
	Binding       platform.OrganizationClosureBinding `json:"binding"`
	PlanDigest    string                              `json:"plan_digest"`
	AfterDocument string                              `json:"after_document"`
}

func DecodeOrganizationClosePlan(raw []byte) (OrganizationClosePlan, error) {
	var plan OrganizationClosePlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&plan) != nil || d.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || (plan.AfterDocument != "" && (!ValidEntityID(plan.AfterDocument) || plan.PlanDigest == "")) {
		return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
	}
	if plan.PlanDigest != "" {
		digest, err := hex.DecodeString(plan.PlanDigest)
		if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != plan.PlanDigest {
			return plan, errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
		}
	}
	return plan, nil
}

// 唯一存续成员包括撤权中的成员，不能趁撤权未知期间清除他人数据。
func OrganizationCloseEligible(org Organization, actor, expected string, member OrganizationMember, liveMembers int) error {
	if org.Lifecycle != "ACTIVE" || member.State != "ACTIVE" {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_CLOSE_PENDING"}
	}
	if org.OwnerID != actor {
		return RuleViolation{Kind: PermissionDenied, Code: "ORGANIZATION_OWNER_REQUIRED"}
	}
	if org.Revision != expected {
		return RuleViolation{Kind: StateConflict, Code: "REVISION_CHANGED"}
	}
	if liveMembers != 1 {
		return RuleViolation{Kind: StateConflict, Code: "ORGANIZATION_OWNER_HANDOFF_REQUIRED"}
	}
	if member.Epoch == 0 || member.Epoch == ^uint64(0) {
		return RuleViolation{Kind: StateConflict, Code: "EPOCH_EXHAUSTED"}
	}
	return nil
}

// organization_name.go：保留原实现，按业务合并文件。

func NormalizeOrganizationName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	length := len([]rune(name))
	return name, length >= 1 && length <= 160
}
````

UTF-8/LF全文SHA-256：`ef673e7c5bea31356a5d3a62aad700eb85e318c9acd799049d6a5783c7185a01`。[参考实现](../../../../services/account/internal/domain/account/organization_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
