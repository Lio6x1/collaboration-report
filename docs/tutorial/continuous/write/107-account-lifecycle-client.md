# 分步编写 `services/document/internal/client/account_lifecycle_client.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/107-account-lifecycle-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

向Account查询关闭/拒绝/回滚权威事实并调用Node单文档清理。

跨服务超时属于未知，不能替代持久拒绝或清理回执。

写之前先按这条执行路线理解各段的连接：

1. AccountClosure.Confirmed/Rejected核原绑定
2. Binding读取完整组织集合
3. AccountDeparture.RolledBack取串行回滚证明
4. AccountDocumentCleanup.Clean按原号调用
5. OrganizationCloseAuthority核组织来源。

本文件需要守住的边界：

- 大集合不能套1KiB布尔响应上限
- HTTP失败或缺行不是回滚
- Node提交丢回复仍续原号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/account_lifecycle_client.go') -and (Get-Item -LiteralPath 'services/document/internal/client/account_lifecycle_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/account_lifecycle_client.go')) { New-Item -ItemType File -Path 'services/document/internal/client/account_lifecycle_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:414 -->
````go
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

````

## 第 2 步：AccountClosure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type AccountClosure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 2 bytes=414:486 -->
````go
type AccountClosure struct {
	endpoint string
	http     *http.Client
}

````

## 第 3 步：NewAccountClosure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func NewAccountClosure(endpoint string, client *http.Client) *AccountClosure`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |
| <code>client</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountClosure</code> | 实际返回表达式：<code>&amp;AccountClosure{endpoint: endpoint, http: client}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountClosure{endpoint: endpoint, http: client}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=486:626 -->
````go
func NewAccountClosure(endpoint string, client *http.Client) *AccountClosure {
	return &AccountClosure{endpoint: endpoint, http: client}
}

````

## 第 4 步：AccountClosure.Rejected

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (c *AccountClosure) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-decision&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-decision&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>result.Rejected</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return result.Rejected, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=626:934 -->
````go
func (c *AccountClosure) Rejected(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	var result platform.AccountClosureDecision
	if err := c.call(ctx, "/infra/lifecycle/account-close-decision", binding, &result); err != nil {
		return false, err
	}
	return result.Rejected, nil
}

````

## 第 5 步：AccountClosure.Confirmed

插入位置：第 4 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func (c *AccountClosure) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-confirmation&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-confirmation&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>result.Confirmed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return result.Confirmed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=934:1252 -->
````go
func (c *AccountClosure) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	var result platform.AccountClosureConfirmation
	if err := c.call(ctx, "/infra/lifecycle/account-close-confirmation", binding, &result); err != nil {
		return false, err
	}
	return result.Confirmed, nil
}

````

## 第 6 步：AccountClosure.call

插入位置：第 5 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (c *AccountClosure) call(ctx context.Context, path string, binding platform.AccountClosureBinding, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+path, bytes.NewReader(raw))</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.endpoint+path</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>platform.ValidateAccountClosureBinding(binding)</code>；<code>json.Marshal(binding)</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>decoder.Decode(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_DECISION_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(binding)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_DECISION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; decoder.Decode(result) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_DECISION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1252:2238 -->
````go
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

````

## 第 7 步：AccountDeparture

插入位置：第 6 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`type AccountDeparture struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseURL</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 7 bytes=2238:2310 -->
````go
type AccountDeparture struct {
	baseURL string
	http    *http.Client
}

````

## 第 8 步：NewAccountDeparture

插入位置：第 7 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func NewAccountDeparture(baseURL string, httpClient *http.Client) *AccountDeparture`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseURL</code> | <code>string</code> | 在此声明中的实际使用：<code>baseURL: baseURL</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountDeparture</code> | 实际返回表达式：<code>&amp;AccountDeparture{baseURL: baseURL, http: httpClient}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountDeparture{baseURL: baseURL, http: httpClient}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2310:2461 -->
````go
func NewAccountDeparture(baseURL string, httpClient *http.Client) *AccountDeparture {
	return &AccountDeparture{baseURL: baseURL, http: httpClient}
}

````

## 第 9 步：AccountDeparture.RolledBack

插入位置：第 8 步代码之后，紧接追加；完成稿约第 83 行。

账号服务提供串行回滚证明；HTTP失败、缺行和超时均不能证明已回滚。

本步声明：`func (c *AccountDeparture) RolledBack(ctx context.Context, observed domain.DepartureObservation) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;GET&quot;, path, nil)</code> |
| <code>observed</code> | <code>domain.DepartureObservation</code> | 使用 <code>domain.DepartureObservation</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>observed.Scope</code>；<code>observed.Operation</code>；<code>observed.Epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>value.ID == observed.Operation &amp;&amp; value.State == &quot;ROLLED_BACK&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DEPARTURE_SCOPE_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;DEPARTURE_STATUS_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(parts) != 3</code>。条件成立时执行 <code>return false, errors.New(&quot;DEPARTURE_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&amp;value) != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;DEPARTURE_STATUS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value.ID == observed.Operation &amp;&amp; value.State == &quot;ROLLED_BACK&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2461:3655 -->
````go
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

````

## 第 10 步：AccountDocumentCleanup

插入位置：第 9 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`type AccountDocumentCleanup struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | Node协作内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 10 bytes=3655:3735 -->
````go
type AccountDocumentCleanup struct {
	endpoint string
	http     *http.Client
}

````

## 第 11 步：NewAccountDocumentCleanup

插入位置：第 10 步代码之后，紧接追加；完成稿约第 116 行。

本步声明：`func NewAccountDocumentCleanup(endpoint string, client *http.Client) *AccountDocumentCleanup`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |
| <code>client</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountDocumentCleanup</code> | 实际返回表达式：<code>&amp;AccountDocumentCleanup{endpoint: endpoint, http: client}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountDocumentCleanup{endpoint: endpoint, http: client}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3735:3899 -->
````go
func NewAccountDocumentCleanup(endpoint string, client *http.Client) *AccountDocumentCleanup {
	return &AccountDocumentCleanup{endpoint: endpoint, http: client}
}

````

## 第 12 步：AccountDocumentCleanup.Clean

插入位置：第 11 步代码之后，紧接追加；完成稿约第 120 行。

Node成功提交但HTTP回复丢失时，调用者保持原号重试；不从超时推断清理失败。

本步声明：`func (c *AccountDocumentCleanup) Clean(ctx context.Context, expected platform.AccountDocumentCleanupContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+&quot;/infra/lifecycle/document-cleanup&quot;, bytes.NewReader(raw))</code> |
| <code>expected</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>expected.Valid</code>；<code>expected.AccountDocumentCleanupRequest</code>；<code>result.AccountDocumentCleanupContext != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CONTEXT_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_NODE_CLEANUP_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_NODE_CLEANUP_UNCONFIRMED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!expected.Valid()</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_NODE_CLEANUP_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; decoder.Decode(&amp;result) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; result.State != &quot;CLEANED&quot; &#124;&#124; result.AccountDocumentCleanupContext != expected</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_NODE_CLEANUP_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3899:5169 -->
````go
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

````

## 第 13 步：AccountClosure.Binding

插入位置：第 12 步代码之后，紧接追加；完成稿约第 150 行。

Binding读取Account原持久关闭事实，HTTP客户端仍由bootstrap装配精确Account mTLS。
大组织集合不能沿用布尔confirmation的1KiB响应预算，也不接受截断的成功JSON。

本步声明：`func (c *AccountClosure) Binding(ctx context.Context, operation string) (platform.AccountClosureBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+&quot;/infra/lifecycle/account-close-binding&quot;, bytes.NewReader(raw))</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationId: operation</code>；<code>result.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>实际返回表达式：<code>result</code>；<code>platform.AccountClosureBinding{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_LIFECYCLE_BINDING_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;ACCOUNT_LIFECYCLE_BINDING_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;ACCOUNT_LIFECYCLE_BINDING_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; decoder.Decode(&amp;result) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; 		!platform.ValidateAccountClosureBinding(result) &#124;&#124; result.Operation != operation</code>。条件成立时执行 <code>return platform.AccountClosureBinding{}, errors.New(&quot;ACCOUNT_LIFECYCLE_BINDING_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5169:6503 -->
````go
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

````

## 第 14 步：OrganizationCloseAuthority

插入位置：第 13 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`type OrganizationCloseAuthority struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 14 bytes=6503:6587 -->
````go
type OrganizationCloseAuthority struct {
	endpoint string
	http     *http.Client
}

````

## 第 15 步：NewOrganizationCloseAuthority

插入位置：第 14 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func NewOrganizationCloseAuthority(endpoint string, client *http.Client) *OrganizationCloseAuthority`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |
| <code>client</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationCloseAuthority</code> | 实际返回表达式：<code>&amp;OrganizationCloseAuthority{endpoint: endpoint, http: client}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationCloseAuthority{endpoint: endpoint, http: client}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6587:6762 -->
````go
func NewOrganizationCloseAuthority(endpoint string, client *http.Client) *OrganizationCloseAuthority {
	return &OrganizationCloseAuthority{endpoint: endpoint, http: client}
}
````

## 第 16 步：OrganizationCloseAuthority.Confirmed

插入位置：第 15 步代码之后，紧接追加；完成稿约第 187 行。

本步声明：`func (c *OrganizationCloseAuthority) Confirmed(ctx context.Context, b platform.OrganizationClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+&quot;/infra/lifecycle/organization-close-confirmation&quot;, bytes.NewReader(raw))</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Valid</code>；<code>json.Marshal(b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>result.Confirmed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_UNCONFIRMED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!b.Valid()</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; d.Decode(&amp;result) != nil &#124;&#124; d.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return false, errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result.Confirmed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6762:7772 -->
````go
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
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/account_lifecycle_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
