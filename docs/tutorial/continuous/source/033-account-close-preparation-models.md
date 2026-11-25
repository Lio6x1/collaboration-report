# services/account/internal/domain/account/account_close_preparation_models.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/033-account-close-preparation-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`AccountCloseCapture`、`AccountCloseRetention`、`AccountCloseOrganization`、`AccountCloseScope`、`AccountCloseRecord`、`AccountCloseReceipt`、`DecodeAccountCloseCapture`、`AccountCloseFencePayload`、`AccountCloseFenceTask`、`DecodeAccountCloseFence`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	"collaboration/platform"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

// account_close.go：保留原实现，按业务合并文件。

// AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。
// Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。
type AccountCloseCapture struct {
	SessionHash     string                          `json:"session_hash"`
	CredentialEpoch uint64                          `json:"credential_epoch"`
	IdentityEpoch   uint64                          `json:"identity_epoch"`
	Binding         *platform.AccountClosureBinding `json:"binding"`
	Organizations   []AccountCloseOrganization      `json:"organizations,omitempty"`
	Scopes          []AccountCloseScope             `json:"scopes,omitempty"`
	Retention       *AccountCloseRetention          `json:"retention,omitempty"`
}

// 文档游标只在原PAYLOADS_REMOVED确认后推进；摘要固定Doc原计划，不存正文。
type AccountCloseRetention struct {
	PlanDigest    string `json:"plan_digest"`
	AfterDocument string `json:"after_document"`
	DocCompleted  bool   `json:"doc_completed"`
}

// MemberRevision是组织成员集合水位，MembershipRevision是本人那一行的水位，
// 两者都固定才能在关闭前发现其他成员加入与本人角色变化。
type AccountCloseOrganization struct {
	ID                      uint64 `json:"id"`
	Owner                   uint64 `json:"owner"`
	Revision                uint64 `json:"revision"`
	MemberRevision          uint64 `json:"member_revision"`
	MembershipRevision      uint64 `json:"membership_revision"`
	Epoch                   uint64 `json:"epoch"`
	Role                    string `json:"role"`
	MemberState             string `json:"member_state"`
	Lifecycle               string `json:"lifecycle"`
	LiveMembers             int    `json:"live_members"`
	AvailableAdministrators int    `json:"available_administrators"`
}

type AccountCloseScope struct {
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      uint64 `json:"epoch"`
}

type AccountCloseRecord struct {
	User    uint64
	State   string
	Capture []byte
}

type AccountCloseReceipt struct {
	User                     uint64
	Kind, State, SessionHash string
	Fingerprint, Result      []byte
}

func DecodeAccountCloseCapture(raw []byte) (AccountCloseCapture, error) {
	var capture AccountCloseCapture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	invalid := errors.New("ACCOUNT_CLOSE_CAPTURE_INVALID")
	if decoder.Decode(&capture) != nil || decoder.Decode(new(any)) != io.EOF {
		return capture, invalid
	}
	digest, err := hex.DecodeString(capture.SessionHash)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != capture.SessionHash || capture.CredentialEpoch == 0 || capture.IdentityEpoch == 0 {
		return capture, invalid
	}
	if capture.Binding != nil && (!platform.ValidateAccountClosureBinding(*capture.Binding) || capture.Binding.IdentityEpoch != capture.IdentityEpoch) {
		return capture, invalid
	}
	if capture.Retention != nil {
		digest, err := hex.DecodeString(capture.Retention.PlanDigest)
		if capture.Binding == nil || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != capture.Retention.PlanDigest {
			return capture, invalid
		}
		if capture.Retention.AfterDocument != "" {
			id, err := strconv.ParseUint(capture.Retention.AfterDocument, 10, 64)
			if err != nil || id == 0 || strconv.FormatUint(id, 10) != capture.Retention.AfterDocument {
				return capture, invalid
			}
		}
	}
	return capture, nil
}

// account_close_fence.go：保留原实现，按业务合并文件。

const AccountCloseFenceKind = "ACCOUNT_CLOSE_FENCE"

type AccountCloseFencePayload struct {
	Operation  string `json:"operation_id"`
	User       uint64 `json:"user_id"`
	Revocation string `json:"revocation_id"`
	Scope      string `json:"scope_key"`
	Epoch      uint64 `json:"epoch"`
}

type AccountCloseFenceTask struct {
	ID, Generation                         uint64
	Operation, Revocation, State, Instance string
	Payload                                []byte
	LeaseValid                             bool
}

func DecodeAccountCloseFence(raw []byte) (AccountCloseFencePayload, error) {
	var payload AccountCloseFencePayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || !ValidOperationID(payload.Operation) || !ValidOperationID(payload.Revocation) || payload.User == 0 || payload.Epoch == 0 || !platform.ValidAccountScope(payload.Scope) {
		return payload, errors.New("ACCOUNT_CLOSE_FENCE_PAYLOAD_INVALID")
	}
	return payload, nil
}
````

UTF-8/LF全文SHA-256：`7fbc58b3f26d036a2f4cb1284cb5e9bd1a09e4b20a8027ab5dad00aec6cfa22a`。[参考实现](../../../../services/account/internal/domain/account/account_close_preparation_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
