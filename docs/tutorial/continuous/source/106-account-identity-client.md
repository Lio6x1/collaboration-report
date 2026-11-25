# services/document/internal/client/account_identity_client.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/106-account-identity-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`RenewAdmission`、`AccountDirectory`、`NewAccountDirectory`、`Read`、`Collaborator`、`AccountSession`、`NewAccountSession`、`VerifySession`、`CursorSigner`、`NewCursorSigner`、`Sign`、`InvitationTokens`、`NewInvitationTokens`、`aead`、`Seal`、`Open`、`Administrators`、`NotificationRecipients`、`NewNotificationRecipients`、`Resolve`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// account_admission.go：保留原实现，按业务合并文件。

// RenewAdmission复用账号mTLS客户端，仅交付持久证明的原绑定，不能恢复或保存浏览器会话令牌。
func (c *AccountSession) RenewAdmission(ctx context.Context, input *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(input)
	if err != nil {
		return nil, errors.New("IDENTITY_BINDING_INVALID")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/infra/identity/renew-operation", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("IDENTITY_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode == 403 {
		return nil, domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("IDENTITY_UNAVAILABLE")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	proof := &pb.IdentityAdmission{}
	if err != nil || len(body) > 65536 || protojson.Unmarshal(body, proof) != nil {
		return nil, errors.New("IDENTITY_RESPONSE_INVALID")
	}
	return proof, nil
}

// account_directory.go：保留原实现，按业务合并文件。

type AccountDirectory struct {
	baseURL string
	http    *http.Client
}

func NewAccountDirectory(baseURL string, httpClient *http.Client) *AccountDirectory {
	return &AccountDirectory{baseURL: baseURL, http: httpClient}
}
func (c *AccountDirectory) Read(ctx context.Context, cookie, path string, value any) error {
	request, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Cookie", cookie)
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("DIRECTORY_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 {
		return domain.Violation(domain.SessionRequired, "DIRECTORY_NOT_CURRENT")
	}
	if response.StatusCode == 403 {
		return domain.Violation(domain.PermissionDenied, "DIRECTORY_NOT_CURRENT")
	}
	if response.StatusCode != 200 {
		return errors.New("DIRECTORY_UNAVAILABLE")
	}
	return json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(value)
}

// Collaborator只提交原目标证明的绑定；内部mTLS入口重验后返回姓名，不能接受浏览器自报昵称。
func (c *AccountDirectory) Collaborator(ctx context.Context, proof *pb.IdentityRevalidation) (dto.DocumentCollaborator, error) {
	var value dto.DocumentCollaborator
	if proof == nil || proof.IdentityTarget == nil {
		return value, errors.New("COLLABORATOR_BINDING_INVALID")
	}
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(proof)
	if err != nil {
		return value, errors.New("COLLABORATOR_BINDING_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/infra/identity/document-collaborator", bytes.NewReader(raw))
	if err != nil {
		return value, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return value, errors.New("DIRECTORY_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden {
		return value, domain.Violation(domain.PermissionDenied, "IDENTITY_NOT_CURRENT")
	}
	if response.StatusCode != http.StatusOK {
		return value, errors.New("DIRECTORY_UNAVAILABLE")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 8193))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || value.UserId != strconv.FormatUint(proof.IdentityTarget.UserId, 10) || strings.TrimSpace(value.DisplayName) == "" {
		return dto.DocumentCollaborator{}, errors.New("COLLABORATOR_RESPONSE_INVALID")
	}
	return value, nil
}

// account_session.go：保留原实现，按业务合并文件。

type AccountSession struct {
	base string
	http *http.Client
}

func NewAccountSession(base string, httpClient *http.Client) *AccountSession {
	return &AccountSession{base: base, http: httpClient}
}
func (c *AccountSession) VerifySession(ctx context.Context, request domain.SessionVerification) (domain.VerifiedSession, error) {
	var verified domain.VerifiedSession
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/infra/identity/verify-session", nil)
	if err != nil {
		return verified, err
	}
	// CSRF由账号权威验证与真实会话的绑定；Doc不复制账号私有表或接受本地猜测。
	req.Header.Set("Cookie", request.Cookie)
	req.Header.Set("Origin", request.Origin)
	req.Header.Set("X-CSRF-Token", request.CSRF)
	req.Header.Set("X-Internal-Mutation", strconv.FormatBool(request.Mutation))
	response, err := c.http.Do(req)
	if err != nil {
		return verified, errors.New("IDENTITY_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 {
		return verified, domain.Violation(domain.SessionRequired, "SESSION_OR_CSRF_INVALID")
	}
	if response.StatusCode == 403 {
		return verified, domain.Violation(domain.PermissionDenied, "SESSION_OR_CSRF_INVALID")
	}
	if response.StatusCode != 200 {
		return verified, errors.New("IDENTITY_UNAVAILABLE")
	}
	var result struct {
		ID                   string `json:"actor_id"`
		SessionExpiresUnixMs int64  `json:"session_expires_unix_ms"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil {
		return verified, errors.New("IDENTITY_RESPONSE_INVALID")
	}
	verified.ActorID, err = strconv.ParseUint(result.ID, 10, 64)
	if err != nil || verified.ActorID == 0 {
		return verified, errors.New("IDENTITY_RESPONSE_INVALID")
	}
	verified.ExpiresUnixMs = result.SessionExpiresUnixMs
	return verified, nil
}

// cursor_signer.go：保留原实现，按业务合并文件。

type CursorSigner struct{ keyFile string }

func NewCursorSigner(keyFile string) *CursorSigner { return &CursorSigner{keyFile: keyFile} }

// 服务私钥只用于派生独立游标签名密钥；读取失败仍按原边界拒绝已签游标。
func (c *CursorSigner) Sign(data []byte) []byte {
	raw, err := os.ReadFile(c.keyFile)
	if err != nil {
		return nil
	}
	key := sha256.Sum256(append([]byte("workbench-cursor-v1\n"), raw...))
	mac := hmac.New(sha256.New, key[:])
	mac.Write(data)
	return mac.Sum(nil)
}

// invitation_tokens.go：保留原实现，按业务合并文件。

// InvitationTokens隔离文件密钥与加密适配；每次读取保持原密钥变更及不可用边界。
type InvitationTokens struct{ keyFile string }

func NewInvitationTokens(keyFile string) *InvitationTokens {
	return &InvitationTokens{keyFile: keyFile}
}
func (s *InvitationTokens) aead() (cipher.AEAD, error) {
	raw, err := os.ReadFile(s.keyFile)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(append([]byte("document-invitation-v1\n"), raw...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *InvitationTokens) Seal(token, binding string) ([]byte, error) {
	a, err := s.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	// 附加数据绑定真实文档或编辑操作，不能把一个实体的密文移到另一个实体重放。
	return a.Seal(nonce, nonce, []byte(token), []byte(binding)), nil
}
func (s *InvitationTokens) Open(raw []byte, binding string) (string, error) {
	a, err := s.aead()
	if err != nil || len(raw) < a.NonceSize() {
		return "", errors.New("INVITATION_KEY_UNAVAILABLE")
	}
	value, err := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], []byte(binding))
	return string(value), err
}

// notification_administrators.go：保留原实现，按业务合并文件。

// Administrators只分页发现候选人；后台投递必须再调用Resolve核验当前角色和短期撤权来源。
func (c *NotificationRecipients) Administrators(ctx context.Context, organization, after uint64) (platform.NotificationAdministratorsResponse, error) {
	var value platform.NotificationAdministratorsResponse
	if organization == 0 {
		return value, errors.New("NOTIFICATION_ADMINISTRATORS_REQUEST_INVALID")
	}
	raw, err := json.Marshal(platform.NotificationAdministratorsRequest{OrganizationID: organization, After: after})
	if err != nil {
		return value, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/infra/notification/administrators", bytes.NewReader(raw))
	if err != nil {
		return value, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return value, errors.New("NOTIFICATION_ADMINISTRATORS_UNAVAILABLE")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || value.OrganizationID != organization || value.UserIDs == nil || len(value.UserIDs) > 100 {
		return value, errors.New("NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID")
	}
	previous := after
	for _, id := range value.UserIDs {
		if id <= previous {
			return value, errors.New("NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID")
		}
		previous = id
	}
	// 非零游标必须严格推进且指向满页末尾，禁止错误分页把同一批重复投递或跳过。
	if value.NextAfter != 0 && (len(value.UserIDs) != 100 || value.NextAfter != previous || value.NextAfter <= after) {
		return value, errors.New("NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID")
	}
	return value, nil
}

// notification_recipients.go：保留原实现，按业务合并文件。

type NotificationRecipients struct {
	baseURL string
	http    *http.Client
}

func NewNotificationRecipients(base string, peer *http.Client) *NotificationRecipients {
	return &NotificationRecipients{baseURL: base, http: peer}
}

// 仅接收权威Account经mTLS返回的短期事实；它不是会话，不能用于普通用户请求。
func (c *NotificationRecipients) Resolve(ctx context.Context, organization uint64, ids []uint64) (platform.NotificationRecipientsResponse, error) {
	var value platform.NotificationRecipientsResponse
	want := map[uint64]bool{}
	for _, id := range ids {
		if id == 0 || want[id] {
			return value, errors.New("NOTIFICATION_RECIPIENT_REQUEST_INVALID")
		}
		want[id] = true
	}
	if len(ids) == 0 || len(ids) > 100 {
		return value, errors.New("NOTIFICATION_RECIPIENT_REQUEST_INVALID")
	}
	raw, err := json.Marshal(platform.NotificationRecipientsRequest{OrganizationID: organization, UserIDs: ids})
	if err != nil {
		return value, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/infra/notification/recipients", bytes.NewReader(raw))
	if err != nil {
		return value, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return value, errors.New("NOTIFICATION_RECIPIENTS_UNAVAILABLE")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || value.OrganizationID != organization || len(value.Recipients) != len(want) || value.ExpiresUnixMS <= time.Now().UnixMilli() || value.ExpiresUnixMS > time.Now().Add(5*time.Second).UnixMilli() {
		return value, errors.New("NOTIFICATION_RECIPIENT_PROOF_INVALID")
	}
	for _, recipient := range value.Recipients {
		if !want[recipient.UserID] {
			return value, errors.New("NOTIFICATION_RECIPIENT_PROOF_INVALID")
		}
		delete(want, recipient.UserID)
		if !recipient.Eligible {
			if len(recipient.Sources) != 0 {
				return value, errors.New("NOTIFICATION_RECIPIENT_PROOF_INVALID")
			}
			continue
		}
		user := strconv.FormatUint(recipient.UserID, 10)
		expected := map[string]uint64{"identity:" + user: recipient.IdentityEpoch, "credential:" + user: recipient.CredentialEpoch}
		if organization != 0 {
			org := strconv.FormatUint(organization, 10)
			expected["org-member:"+org+":"+user] = recipient.MemberEpoch
			if recipient.Role == "ADMIN" {
				expected["org-admin:"+org+":"+user] = recipient.MemberEpoch
			}
			if recipient.OrgOwner {
				expected["org-owner:"+org+":"+user] = recipient.MemberEpoch
			}
		}
		if len(recipient.Sources) != len(expected) {
			return value, errors.New("NOTIFICATION_RECIPIENT_PROOF_INVALID")
		}
		for _, source := range recipient.Sources {
			if source.Epoch == 0 || expected[source.ScopeKey] != source.Epoch {
				return value, errors.New("NOTIFICATION_RECIPIENT_PROOF_INVALID")
			}
			delete(expected, source.ScopeKey)
		}
	}
	return value, nil
}
````

UTF-8/LF全文SHA-256：`d6e60d00c71dc11e3f7a8050923c4431e457cc9f34a00c2b2599abd1ac218f17`。[参考实现](../../../../services/document/internal/client/account_identity_client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
