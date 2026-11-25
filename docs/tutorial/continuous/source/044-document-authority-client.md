# services/account/internal/client/document_authority_client.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/044-document-authority-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`AccountClosureControl`、`NewAccountClosureControl`、`call`、`send`、`Prepare`、`Cancel`、`Plan`、`Relations`、`Complete`、`Page`、`CleanDocument`、`AccountOutput`、`NewAccountOutput`、`Register`、`Departure`、`NewDeparture`、`Check`、`Document`、`NewDocument`、`FenceAccess`、`FailureCode`、`CloseDepartedGrants`、`OrganizationCloseControl`、`NewOrganizationCloseControl`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/status"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// account_closure.go：保留原实现，按业务合并文件。

type AccountClosureControl struct {
	http     *http.Client
	endpoint string
}

func NewAccountClosureControl(httpClient *http.Client, endpoint string) *AccountClosureControl {
	return &AccountClosureControl{http: httpClient, endpoint: endpoint}
}

func (c *AccountClosureControl) call(ctx context.Context, path string, binding platform.AccountClosureBinding, result any) error {
	if !platform.ValidateAccountClosureBinding(binding) {
		return errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	return c.send(ctx, path, raw, result)
}

// 内部绑定和有界分页共用mTLS/完整JSON核验，不接受成功HTTP中的截断回执。
func (c *AccountClosureControl) send(ctx context.Context, path string, raw []byte, result any) error {
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("ACCOUNT_CLOSURE_CONTROL_UNAVAILABLE")
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(result) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("ACCOUNT_CLOSURE_CONTROL_UNCONFIRMED")
	}
	return nil
}

func (c *AccountClosureControl) Prepare(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosurePreparation, error) {
	var result platform.AccountClosurePreparation
	err := c.call(ctx, "/infra/lifecycle/account-close-prepare", binding, &result)
	if err == nil && (result.RequiredKnowledgeBases == nil || result.ReservedKnowledgeBases == nil || (result.State != "FROZEN" && result.State != "HANDOFF_REQUIRED" && result.State != "REJECTED")) {
		err = errors.New("ACCOUNT_CLOSURE_PREPARATION_INVALID")
	}
	return result, err
}

func (c *AccountClosureControl) Cancel(ctx context.Context, binding platform.AccountClosureBinding) error {
	var result platform.AccountClosureDecision
	if err := c.call(ctx, "/infra/lifecycle/account-close-cancel", binding, &result); err != nil {
		return err
	}
	if !result.Rejected {
		return errors.New("ACCOUNT_CLOSURE_CANCEL_UNCONFIRMED")
	}
	return nil
}

func (c *AccountClosureControl) Plan(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error) {
	var result platform.AccountClosureRetention
	err := c.call(ctx, "/infra/lifecycle/account-close-plan", binding, &result)
	if err == nil && result.State != "PLANNED" {
		err = errors.New("ACCOUNT_CLOSURE_RETENTION_UNCONFIRMED")
	}
	return result, err
}

func (c *AccountClosureControl) Relations(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error) {
	var result platform.AccountClosureRetention
	err := c.call(ctx, "/infra/lifecycle/account-close-relations", binding, &result)
	if err == nil && result.State != "CLEANING" {
		err = errors.New("ACCOUNT_CLOSURE_RELATIONS_UNCONFIRMED")
	}
	return result, err
}

func (c *AccountClosureControl) Complete(ctx context.Context, binding platform.AccountClosureBinding) error {
	var result platform.AccountClosureRetention
	if err := c.call(ctx, "/infra/lifecycle/account-close-complete", binding, &result); err != nil {
		return err
	}
	if result.State != "SUCCEEDED" {
		return errors.New("ACCOUNT_CLOSURE_COMPLETION_UNCONFIRMED")
	}
	return nil
}

func (c *AccountClosureControl) Page(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error) {
	var result platform.AccountClosurePage
	if !platform.ValidateAccountClosureBinding(request.AccountClosureBinding) {
		return result, errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if err = c.send(ctx, "/infra/lifecycle/account-close-progress", raw, &result); err != nil {
		return result, err
	}
	if !result.Matches(request) {
		return result, errors.New("ACCOUNT_CLOSURE_PAGE_INVALID")
	}
	return result, nil
}

func (c *AccountClosureControl) CleanDocument(ctx context.Context, binding platform.AccountClosureBinding, document string) error {
	request := platform.AccountDocumentCleanupRequest{Operation: binding.Operation, Document: document}
	if !platform.ValidateAccountClosureBinding(binding) || !request.Valid() {
		return errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var result platform.AccountDocumentCleanupReceipt
	if err = c.send(ctx, "/infra/lifecycle/document-clean-payloads", raw, &result); err != nil {
		return err
	}
	if result.AccountDocumentCleanupRequest != request || !result.Valid() || result.State != "PAYLOADS_REMOVED" {
		return errors.New("ACCOUNT_CLOSURE_DOCUMENT_UNCONFIRMED")
	}
	return nil
}

// account_output.go：保留原实现，按业务合并文件。

type AccountOutput struct {
	http     *http.Client
	endpoint string
}

func NewAccountOutput(httpClient *http.Client, endpoint string) *AccountOutput {
	return &AccountOutput{http: httpClient, endpoint: endpoint}
}

func (c *AccountOutput) Register(ctx context.Context, handler string, sources []domain.OutputSource) error {
	raw, _ := json.Marshal(map[string]any{"handler_id": handler, "sources": sources})
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/output/register-account", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("OUTPUT_REGISTER_UNKNOWN")
	}
	defer response.Body.Close()
	var ack struct {
		State string `json:"state"`
		Code  string `json:"code"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&ack) != nil || response.StatusCode != 200 || ack.State != "OPEN" {
		slog.Warn("account_output_registration_pending", "status", response.StatusCode, "code", ack.Code)
		return errors.New("OUTPUT_REGISTER_UNKNOWN")
	}
	return nil
}

// departure.go：保留原实现，按业务合并文件。

type Departure struct {
	http     *http.Client
	endpoint string
}

func NewDeparture(httpClient *http.Client, endpoint string) *Departure {
	return &Departure{http: httpClient, endpoint: endpoint}
}

func (c *Departure) Check(ctx context.Context, organization, user, operation string, epoch uint64) error {
	// 调用方仍持有组织锁；Doc须锁住同一member来源，不能用两次普通读取代替互斥。
	ctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	endpoint := c.endpoint + "/infra/lifecycle/member-departure-eligibility?organization_id=" + url.QueryEscape(organization) + "&user_id=" + url.QueryEscape(user) + "&operation_id=" + url.QueryEscape(operation) + "&epoch=" + strconv.FormatUint(epoch, 10)
	request, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	var value struct {
		RequiresHandoff bool `json:"requires_handoff"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&value) != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	if value.RequiresHandoff {
		return domain.RuleViolation{Kind: domain.StateConflict, Code: "ADMIN_RETIREMENT_REQUIRED"}
	}
	return nil
}

// document.go：保留原实现，按业务合并文件。

type Document struct {
	rpc      pb.DocumentInternalClient
	http     *http.Client
	endpoint string
}

func NewDocument(rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *Document {
	return &Document{rpc: rpc, http: httpClient, endpoint: endpoint}
}

func (c *Document) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	return c.rpc.FenceAccess(ctx, request)
}

func (c *Document) FailureCode(err error) string { return status.Code(err).String() }

func (c *Document) CloseDepartedGrants(ctx context.Context, binding domain.SessionClosure, operation string) error {
	// 客户端继续使用平台装配的mTLS连接；浏览器声明不能代替账号服务证书。
	endpoint := c.endpoint + "/infra/lifecycle/member-departure-close?organization_id=" + url.QueryEscape(binding.OrganizationID) + "&user_id=" + url.QueryEscape(binding.UserID) + "&operation_id=" + url.QueryEscape(operation) + "&epoch=" + binding.Epoch
	request, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("DEPARTURE_GRANTS_UNKNOWN")
	}
	return nil
}

// organization_close.go：保留原实现，按业务合并文件。

// 传输复用同一mTLS与严格完整JSON读取器；绑定协议和路由属于独立组织关闭。
type OrganizationCloseControl struct{ transport *AccountClosureControl }

func NewOrganizationCloseControl(httpClient *http.Client, endpoint string) *OrganizationCloseControl {
	return &OrganizationCloseControl{transport: NewAccountClosureControl(httpClient, endpoint)}
}
func (c *OrganizationCloseControl) call(ctx context.Context, path string, b platform.OrganizationClosureBinding, result any) error {
	if !b.Valid() {
		return errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return c.transport.send(ctx, path, raw, result)
}
func (c *OrganizationCloseControl) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error {
	var result platform.OrganizationClosureState
	if err := c.call(ctx, "/infra/lifecycle/organization-close-plan", b, &result); err != nil {
		return err
	}
	if result.State != "PLANNED" && result.State != "CLEANING" && result.State != "SUCCEEDED" {
		return errors.New("ORGANIZATION_CLOSE_PLAN_UNCONFIRMED")
	}
	return nil
}
func (c *OrganizationCloseControl) Complete(ctx context.Context, b platform.OrganizationClosureBinding) error {
	var result platform.OrganizationClosureState
	if err := c.call(ctx, "/infra/lifecycle/organization-close-complete", b, &result); err != nil {
		return err
	}
	if result.State != "SUCCEEDED" {
		return errors.New("ORGANIZATION_CLOSE_COMPLETE_UNCONFIRMED")
	}
	return nil
}
func (c *OrganizationCloseControl) Page(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error) {
	var result platform.OrganizationClosurePage
	if !request.Binding.Valid() {
		return result, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if err = c.transport.send(ctx, "/infra/lifecycle/organization-close-progress", raw, &result); err != nil {
		return result, err
	}
	if !result.Matches(request) {
		return result, errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
	}
	return result, nil
}
func (c *OrganizationCloseControl) CleanDocument(ctx context.Context, b platform.OrganizationClosureBinding, document string) error {
	request := platform.AccountDocumentCleanupRequest{Operation: b.Operation, Document: document}
	if !b.Valid() || !request.Valid() {
		return errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var result platform.AccountDocumentCleanupReceipt
	if err = c.transport.send(ctx, "/infra/lifecycle/organization-close-clean-payloads", raw, &result); err != nil {
		return err
	}
	if result.AccountDocumentCleanupRequest != request || !result.Valid() || result.State != "PAYLOADS_REMOVED" {
		return errors.New("ORGANIZATION_CLOSE_CLEANUP_UNCONFIRMED")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`d4993da86e610d1cbf74f02c0eed282094d1c04df72aa8e7a1748340112a5d8f`。[参考实现](../../../../services/account/internal/client/document_authority_client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
