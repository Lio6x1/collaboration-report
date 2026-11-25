// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// account_closure.go：保留原实现，按业务合并文件。

type AccountClosure struct {
	endpoint string
	http     *http.Client
}

func NewAccountClosure(endpoint string, client *http.Client) *AccountClosure {
	return &AccountClosure{endpoint: endpoint, http: client}
}

func (c *AccountClosure) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	var result platform.AccountClosureDecision
	if err := c.call(ctx, "/infra/lifecycle/account-close-decision", binding, &result); err != nil {
		return false, err
	}
	return result.Rejected, nil
}

func (c *AccountClosure) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	var result platform.AccountClosureConfirmation
	if err := c.call(ctx, "/infra/lifecycle/account-close-confirmation", binding, &result); err != nil {
		return false, err
	}
	return result.Confirmed, nil
}

func (c *AccountClosure) call(ctx context.Context, path string, binding platform.AccountClosureBinding, result any) error {
	if !platform.ValidateAccountClosureBinding(binding) {
		return errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("ACCOUNT_CLOSURE_DECISION_UNAVAILABLE")
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(result) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("ACCOUNT_CLOSURE_DECISION_UNAVAILABLE")
	}
	return nil
}

// account_departure.go：保留原实现，按业务合并文件。

type AccountDeparture struct {
	baseURL string
	http    *http.Client
}

func NewAccountDeparture(baseURL string, httpClient *http.Client) *AccountDeparture {
	return &AccountDeparture{baseURL: baseURL, http: httpClient}
}

// 账号服务提供串行回滚证明；HTTP失败、缺行和超时均不能证明已回滚。
func (c *AccountDeparture) RolledBack(ctx context.Context, observed domain.DepartureObservation) (bool, error) {
	parts := strings.Split(observed.Scope, ":")
	if len(parts) != 3 {
		return false, errors.New("DEPARTURE_SCOPE_INVALID")
	}
	path := c.baseURL + "/infra/lifecycle/member-departure-status?organization_id=" + parts[1] + "&user_id=" + parts[2] + "&operation_id=" + url.QueryEscape(observed.Operation) + "&epoch=" + strconv.FormatUint(observed.Epoch, 10)
	request, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return false, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	var value struct {
		State string `json:"state"`
		ID    string `json:"operation_id"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&value) != nil {
		return false, errors.New("DEPARTURE_STATUS_UNAVAILABLE")
	}
	return value.ID == observed.Operation && value.State == "ROLLED_BACK", nil
}

// account_document_cleanup.go：保留原实现，按业务合并文件。

type AccountDocumentCleanup struct {
	endpoint string
	http     *http.Client
}

func NewAccountDocumentCleanup(endpoint string, client *http.Client) *AccountDocumentCleanup {
	return &AccountDocumentCleanup{endpoint: endpoint, http: client}
}

// Node成功提交但HTTP回复丢失时，调用者保持原号重试；不从超时推断清理失败。
func (c *AccountDocumentCleanup) Clean(ctx context.Context, expected platform.AccountDocumentCleanupContext) error {
	if !expected.Valid() {
		return errors.New("ACCOUNT_DOCUMENT_CONTEXT_INVALID")
	}
	raw, err := json.Marshal(expected.AccountDocumentCleanupRequest)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/lifecycle/document-cleanup", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("ACCOUNT_DOCUMENT_NODE_CLEANUP_UNAVAILABLE")
	}
	defer response.Body.Close()
	var result platform.AccountDocumentCleanupReceipt
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2048))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || result.State != "CLEANED" || result.AccountDocumentCleanupContext != expected {
		return errors.New("ACCOUNT_DOCUMENT_NODE_CLEANUP_UNCONFIRMED")
	}
	return nil
}

// account_lifecycle_binding.go：保留原实现，按业务合并文件。

// Binding读取Account原持久关闭事实，HTTP客户端仍由bootstrap装配精确Account mTLS。
// 大组织集合不能沿用布尔confirmation的1KiB响应预算，也不接受截断的成功JSON。
func (c *AccountClosure) Binding(ctx context.Context, operation string) (platform.AccountClosureBinding, error) {
	var result platform.AccountClosureBinding
	raw, err := json.Marshal(dto.Action{OperationId: operation})
	if err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/lifecycle/account-close-binding", bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return result, errors.New("ACCOUNT_LIFECYCLE_BINDING_UNAVAILABLE")
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF ||
		!platform.ValidateAccountClosureBinding(result) || result.Operation != operation {
		return platform.AccountClosureBinding{}, errors.New("ACCOUNT_LIFECYCLE_BINDING_UNCONFIRMED")
	}
	return result, nil
}

// organization_close.go：保留原实现，按业务合并文件。

type OrganizationCloseAuthority struct {
	endpoint string
	http     *http.Client
}

func NewOrganizationCloseAuthority(endpoint string, client *http.Client) *OrganizationCloseAuthority {
	return &OrganizationCloseAuthority{endpoint: endpoint, http: client}
}
func (c *OrganizationCloseAuthority) Confirmed(ctx context.Context, b platform.OrganizationClosureBinding) (bool, error) {
	if !b.Valid() {
		return false, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/lifecycle/organization-close-confirmation", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return false, errors.New("ORGANIZATION_CLOSE_AUTHORITY_UNAVAILABLE")
	}
	defer response.Body.Close()
	var result platform.OrganizationClosureConfirmation
	d := json.NewDecoder(io.LimitReader(response.Body, 1024))
	d.DisallowUnknownFields()
	if response.StatusCode != 200 || d.Decode(&result) != nil || d.Decode(new(any)) != io.EOF {
		return false, errors.New("ORGANIZATION_CLOSE_AUTHORITY_UNCONFIRMED")
	}
	return result.Confirmed, nil
}
