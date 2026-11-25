# 分步编写 `services/document/internal/client/account_identity_client.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/106-account-identity-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现账号服务的会话验证、准入续期、人员目录、通知资格以及邀请加密适配。

Document不信任浏览器自报身份，也不把持久操作恢复成一个新浏览器会话；短期证明只能由Account权威返回。

写之前先按这条执行路线理解各段的连接：

1. VerifySession转送Cookie/Origin/CSRF
2. RenewAdmission提交原绑定
3. InvitationTokens用独立派生密钥及binding加解密
4. Resolve验证每个收件人的来源与期限。

本文件需要守住的边界：

- 存在Cookie的变更必须验证CSRF
- 通知候选名单不等于当前可投递资格
- 管理员分页游标必须严格推进。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/account_identity_client.go') -and (Get-Item -LiteralPath 'services/document/internal/client/account_identity_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/account_identity_client.go')) { New-Item -ItemType File -Path 'services/document/internal/client/account_identity_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:587 -->
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

````

## 第 2 步：AccountSession.RenewAdmission

插入位置：第 1 步代码之后，紧接追加；完成稿约第 29 行。

RenewAdmission复用账号mTLS客户端，仅交付持久证明的原绑定，不能恢复或保存浏览器会话令牌。

本步声明：`func (c *AccountSession) RenewAdmission(ctx context.Context, input *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.base+&quot;/infra/identity/renew-operation&quot;, bytes.NewReader(raw))</code> |
| <code>input</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>(protojson.MarshalOptions{UseProtoNames: true}).Marshal(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;IDENTITY_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == 403</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(body) &gt; 65536 &#124;&#124; protojson.Unmarshal(body, proof) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;IDENTITY_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=587:1855 -->
````go
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

````

## 第 3 步：AccountDirectory

插入位置：第 2 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`type AccountDirectory struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseURL</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 3 bytes=1855:1927 -->
````go
type AccountDirectory struct {
	baseURL string
	http    *http.Client
}

````

## 第 4 步：NewAccountDirectory

插入位置：第 3 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func NewAccountDirectory(baseURL string, httpClient *http.Client) *AccountDirectory`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseURL</code> | <code>string</code> | 在此声明中的实际使用：<code>baseURL: baseURL</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountDirectory</code> | 实际返回表达式：<code>&amp;AccountDirectory{baseURL: baseURL, http: httpClient}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountDirectory{baseURL: baseURL, http: httpClient}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1927:2077 -->
````go
func NewAccountDirectory(baseURL string, httpClient *http.Client) *AccountDirectory {
	return &AccountDirectory{baseURL: baseURL, http: httpClient}
}
````

## 第 5 步：AccountDirectory.Read

插入位置：第 4 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func (c *AccountDirectory) Read(ctx context.Context, cookie, path string, value any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;GET&quot;, c.baseURL+path, nil)</code> |
| <code>cookie</code> | <code>string</code> | 在此声明中的实际使用：<code>request.Header.Set(&quot;Cookie&quot;, cookie)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.baseURL+path</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>json.NewDecoder(io.LimitReader(response.Body, 2&lt;&lt;20)).Decode(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.SessionRequired, &quot;DIRECTORY_NOT_CURRENT&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;DIRECTORY_NOT_CURRENT&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == 401</code>。条件成立时执行 <code>return domain.Violation(domain.SessionRequired, &quot;DIRECTORY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode == 403</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;DIRECTORY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.NewDecoder(io.LimitReader(response.Body, 2&lt;&lt;20)).Decode(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2077:2832 -->
````go
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

````

## 第 6 步：AccountDirectory.Collaborator

插入位置：第 5 步代码之后，紧接追加；完成稿约第 92 行。

Collaborator只提交原目标证明的绑定；内部mTLS入口重验后返回姓名，不能接受浏览器自报昵称。

本步声明：`func (c *AccountDirectory) Collaborator(ctx context.Context, proof *pb.IdentityRevalidation) (dto.DocumentCollaborator, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, time.Second)</code>；<code>context.WithTimeout(ctx, time.Second)</code>；<code>http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+&quot;/infra/identity/document-collaborator&quot;, bytes.NewReader(raw))</code> |
| <code>proof</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>proof == nil</code>；<code>proof.IdentityTarget</code>；<code>(protojson.MarshalOptions{UseProtoNames: true}).Marshal(proof)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.DocumentCollaborator</code> | 实际返回表达式：<code>value</code>；<code>dto.DocumentCollaborator{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;COLLABORATOR_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>proof == nil &#124;&#124; proof.IdentityTarget == nil</code>。条件成立时执行 <code>return value, errors.New(&quot;COLLABORATOR_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, errors.New(&quot;COLLABORATOR_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == http.StatusForbidden</code>。条件成立时执行 <code>return value, domain.Violation(domain.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != http.StatusOK</code>。条件成立时执行 <code>return value, errors.New(&quot;DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; decoder.Decode(&amp;struct{}{}) != io.EOF &#124;&#124; value.UserId != strconv.FormatUint(proof.IdentityTarget.UserId, 10) &#124;&#124; strings.TrimSpace(value.DisplayName) == &quot;&quot;</code>。条件成立时执行 <code>return dto.DocumentCollaborator{}, errors.New(&quot;COLLABORATOR_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2832:4534 -->
````go
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

````

## 第 7 步：AccountSession

插入位置：第 6 步代码之后，紧接追加；完成稿约第 130 行。

本步声明：`type AccountSession struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 7 bytes=4534:4598 -->
````go
type AccountSession struct {
	base string
	http *http.Client
}

````

## 第 8 步：NewAccountSession

插入位置：第 7 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func NewAccountSession(base string, httpClient *http.Client) *AccountSession`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>string</code> | 在此声明中的实际使用：<code>base: base</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountSession</code> | 实际返回表达式：<code>&amp;AccountSession{base: base, http: httpClient}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountSession{base: base, http: httpClient}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4598:4733 -->
````go
func NewAccountSession(base string, httpClient *http.Client) *AccountSession {
	return &AccountSession{base: base, http: httpClient}
}
````

## 第 9 步：AccountSession.VerifySession

插入位置：第 8 步代码之后，紧接追加；完成稿约第 138 行。

本步声明：`func (c *AccountSession) VerifySession(ctx context.Context, request domain.SessionVerification) (domain.VerifiedSession, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.base+&quot;/infra/identity/verify-session&quot;, nil)</code> |
| <code>request</code> | <code>domain.SessionVerification</code> | 使用 <code>domain.SessionVerification</code> 类型；其定义处的职责：SessionVerification只携带账号权威验证所需元数据，业务层不保存HTTP请求。<br>在此声明中的实际使用：<code>request.Cookie</code>；<code>request.Origin</code>；<code>request.CSRF</code>；<code>request.Mutation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.VerifiedSession</code> | 使用 <code>domain.VerifiedSession</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>verified</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.SessionRequired, &quot;SESSION_OR_CSRF_INVALID&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_OR_CSRF_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return verified, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return verified, errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == 401</code>。条件成立时执行 <code>return verified, domain.Violation(domain.SessionRequired, &quot;SESSION_OR_CSRF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode == 403</code>。条件成立时执行 <code>return verified, domain.Violation(domain.PermissionDenied, &quot;SESSION_OR_CSRF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return verified, errors.New(&quot;IDENTITY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&amp;result) != nil</code>。条件成立时执行 <code>return verified, errors.New(&quot;IDENTITY_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; verified.ActorID == 0</code>。条件成立时执行 <code>return verified, errors.New(&quot;IDENTITY_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return verified, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4733:6420 -->
````go
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

````

## 第 10 步：CursorSigner

插入位置：第 9 步代码之后，紧接追加；完成稿约第 180 行。

本步声明：`type CursorSigner struct{ keyFile string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>keyFile</code> | <code>string</code> | 游标签名密钥文件路径，密钥留在服务端，不复制进教程或前端。 |

<!-- write-step: 10 bytes=6420:6464 -->
````go
type CursorSigner struct{ keyFile string }

````

## 第 11 步：NewCursorSigner

插入位置：第 10 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func NewCursorSigner(keyFile string) *CursorSigner`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>keyFile</code> | <code>string</code> | 在此声明中的实际使用：<code>keyFile: keyFile</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*CursorSigner</code> | 实际返回表达式：<code>&amp;CursorSigner{keyFile: keyFile}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;CursorSigner{keyFile: keyFile}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6464:6559 -->
````go
func NewCursorSigner(keyFile string) *CursorSigner { return &CursorSigner{keyFile: keyFile} }

````

## 第 12 步：CursorSigner.Sign

插入位置：第 11 步代码之后，紧接追加；完成稿约第 184 行。

服务私钥只用于派生独立游标签名密钥；读取失败仍按原边界拒绝已签游标。

本步声明：`func (c *CursorSigner) Sign(data []byte) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>data</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>mac.Write(data)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>mac.Sum(nil)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return mac.Sum(nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6559:7003 -->
````go
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

````

## 第 13 步：InvitationTokens

插入位置：第 12 步代码之后，紧接追加；完成稿约第 198 行。

InvitationTokens隔离文件密钥与加密适配；每次读取保持原密钥变更及不可用边界。

本步声明：`type InvitationTokens struct{ keyFile string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>keyFile</code> | <code>string</code> | 邀请令牌加密密钥文件路径，解密绑定原文档，不能与原文令牌一同公开。 |

<!-- write-step: 13 bytes=7003:7161 -->
````go
// InvitationTokens隔离文件密钥与加密适配；每次读取保持原密钥变更及不可用边界。
type InvitationTokens struct{ keyFile string }

````

## 第 14 步：NewInvitationTokens

插入位置：第 13 步代码之后，紧接追加；完成稿约第 201 行。

本步声明：`func NewInvitationTokens(keyFile string) *InvitationTokens`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>keyFile</code> | <code>string</code> | 在此声明中的实际使用：<code>keyFile: keyFile</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*InvitationTokens</code> | 使用 <code>InvitationTokens</code> 类型；其定义处的职责：InvitationTokens隔离文件密钥与加密适配；每次读取保持原密钥变更及不可用边界。<br>实际返回表达式：<code>&amp;InvitationTokens{keyFile: keyFile}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;InvitationTokens{keyFile: keyFile}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7161:7268 -->
````go
func NewInvitationTokens(keyFile string) *InvitationTokens {
	return &InvitationTokens{keyFile: keyFile}
}
````

## 第 15 步：InvitationTokens.aead

插入位置：第 14 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`func (s *InvitationTokens) aead() (cipher.AEAD, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>cipher.AEAD</code> | 由 <code>cipher.NewGCM(block)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>cipher.NewGCM(block)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return cipher.NewGCM(block)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7268:7579 -->
````go
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
````

## 第 16 步：InvitationTokens.Seal

插入位置：第 15 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`func (s *InvitationTokens) Seal(token, binding string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(token)</code> |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>a.Seal(nonce, nonce, &#91;&#93;byte(token), &#91;&#93;byte(binding))</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return a.Seal(nonce, nonce, &#91;&#93;byte(token), &#91;&#93;byte(binding)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7579:7994 -->
````go
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
````

## 第 17 步：InvitationTokens.Open

插入位置：第 16 步代码之后，紧接追加；完成稿约第 228 行。

本步声明：`func (s *InvitationTokens) Open(raw []byte, binding string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>a.Open(nil, raw&#91;:a.NonceSize()&#93;, raw&#91;a.NonceSize():&#93;, &#91;&#93;byte(binding))</code> |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>string(value)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;INVITATION_KEY_UNAVAILABLE&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(raw) &lt; a.NonceSize()</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;INVITATION_KEY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return string(value), err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=7994:8390 -->
````go
func (s *InvitationTokens) Open(raw []byte, binding string) (string, error) {
	a, err := s.aead()
	if err != nil || len(raw) < a.NonceSize() {
		return "", errors.New("INVITATION_KEY_UNAVAILABLE")
	}
	value, err := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], []byte(binding))
	return string(value), err
}

// notification_administrators.go：保留原实现，按业务合并文件。

````

## 第 18 步：NotificationRecipients.Administrators

插入位置：第 17 步代码之后，紧接追加；完成稿约第 239 行。

Administrators只分页发现候选人；后台投递必须再调用Resolve核验当前角色和短期撤权来源。

本步声明：`func (c *NotificationRecipients) Administrators(ctx context.Context, organization, after uint64) (platform.NotificationAdministratorsResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+&quot;/infra/notification/administrators&quot;, bytes.NewReader(raw))</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>organization == 0</code>；<code>OrganizationID: organization</code>；<code>value.OrganizationID != organization</code> |
| <code>after</code> | <code>uint64</code> | 在此声明中的实际使用：<code>After: after</code>；<code>previous := after</code>；<code>value.NextAfter &lt;= after</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.NotificationAdministratorsResponse</code> | 使用 <code>platform.NotificationAdministratorsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_ADMINISTRATORS_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;NOTIFICATION_ADMINISTRATORS_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>organization == 0</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_ADMINISTRATORS_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != http.StatusOK</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_ADMINISTRATORS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; decoder.Decode(&amp;struct{}{}) != io.EOF &#124;&#124; value.OrganizationID != organization &#124;&#124; value.UserIDs == nil &#124;&#124; len(value.UserIDs) &gt; 100</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id &lt;= previous</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.NextAfter != 0 &amp;&amp; (len(value.UserIDs) != 100 &#124;&#124; value.NextAfter != previous &#124;&#124; value.NextAfter &lt;= after)</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_ADMINISTRATORS_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8390:10344 -->
````go
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

````

## 第 19 步：NotificationRecipients

插入位置：第 18 步代码之后，紧接追加；完成稿约第 283 行。

本步声明：`type NotificationRecipients struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseURL</code> | <code>string</code> | Account内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 19 bytes=10344:10422 -->
````go
type NotificationRecipients struct {
	baseURL string
	http    *http.Client
}

````

## 第 20 步：NewNotificationRecipients

插入位置：第 19 步代码之后，紧接追加；完成稿约第 288 行。

本步声明：`func NewNotificationRecipients(base string, peer *http.Client) *NotificationRecipients`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>string</code> | 在此声明中的实际使用：<code>baseURL: base</code> |
| <code>peer</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationRecipients</code> | 实际返回表达式：<code>&amp;NotificationRecipients{baseURL: base, http: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationRecipients{baseURL: base, http: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10422:10573 -->
````go
func NewNotificationRecipients(base string, peer *http.Client) *NotificationRecipients {
	return &NotificationRecipients{baseURL: base, http: peer}
}

````

## 第 21 步：NotificationRecipients.Resolve

插入位置：第 20 步代码之后，紧接追加；完成稿约第 292 行。

仅接收权威Account经mTLS返回的短期事实；它不是会话，不能用于普通用户请求。

本步声明：`func (c *NotificationRecipients) Resolve(ctx context.Context, organization uint64, ids []uint64) (platform.NotificationRecipientsResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.baseURL+&quot;/infra/notification/recipients&quot;, bytes.NewReader(raw))</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>OrganizationID: organization</code>；<code>value.OrganizationID != organization</code>；<code>organization != 0</code>；<code>strconv.FormatUint(organization, 10)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>UserIDs: ids</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.NotificationRecipientsResponse</code> | 使用 <code>platform.NotificationRecipientsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;NOTIFICATION_RECIPIENT_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENTS_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>id == 0 &#124;&#124; want&#91;id&#93;</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(ids) == 0 &#124;&#124; len(ids) &gt; 100</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != http.StatusOK</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENTS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil &#124;&#124; decoder.Decode(&amp;struct{}{}) != io.EOF &#124;&#124; value.OrganizationID != organization &#124;&#124; len(value.Recipients) != len(want) &#124;&#124; value.ExpiresUnixMS &lt;= time. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!want&#91;recipient.UserID&#93;</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!recipient.Eligible</code>。
- 检查 <code>len(recipient.Sources) != 0</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=10573:12517 -->
````go
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
````

## 第 22 步：NotificationRecipients.Resolve（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 338 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>organization != 0</code>。
- 检查 <code>recipient.Role == &quot;ADMIN&quot;</code>。
- 检查 <code>recipient.OrgOwner</code>。
- 检查 <code>len(recipient.Sources) != len(expected)</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>source.Epoch == 0 &#124;&#124; expected&#91;source.ScopeKey&#93; != source.Epoch</code>。条件成立时执行 <code>return value, errors.New(&quot;NOTIFICATION_RECIPIENT_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=12517:13407 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/account_identity_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
