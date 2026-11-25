# 分步编写 `services/account/internal/handler/account/identity_handlers.go`

先按 [A02 · 短期身份准入、重验与幂等写入](../lessons/account/02-identity-admission.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/072-identity-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

接入仅供 Document 的身份续期/协作者解析端点，并把浏览器写入绑定为 Mutation。

内部证明入口使用真实服务证书，浏览器写入使用真实会话；两类身份不能混用。

写之前先按这条执行路线理解各段的连接：

1. IdentityRenewal 先 HTTPCaller(document)，限8KiB protobuf JSON，按绑定无效/资格消失/不可用映射400/403/503。
2. MutationRequests.Bind 检查原号、Authenticate(mutation=true)，把URL路径和规范输入一起HMAC。

本文件需要守住的边界：

- 路径必须参与指纹，否则同号可能在两个目标接口间复用。
- DocumentCollaborator 不应成为任意用户目录，目标姓名需原文档用途证明。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/identity_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/identity_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/identity_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/identity_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:514 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
)

// identity_renewal.go：保留原实现，按业务合并文件。

````

## 第 2 步：IdentityRenewal

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type IdentityRenewal struct{ logic *logic.Identity }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |

<!-- write-step: 2 bytes=514:568 -->
````go
type IdentityRenewal struct{ logic *logic.Identity }

````

## 第 3 步：NewIdentityRenewal

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func NewIdentityRenewal(service *logic.Identity) *IdentityRenewal`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*IdentityRenewal</code> | 实际返回表达式：<code>&amp;IdentityRenewal{logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;IdentityRenewal{logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=568:680 -->
````go
func NewIdentityRenewal(service *logic.Identity) *IdentityRenewal {
	return &IdentityRenewal{logic: service}
}

````

## 第 4 步：IdentityRenewal.Renew

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (h *IdentityRenewal) Renew(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Failure(w, 400, &quot;IDENTITY_BINDING_INVALID&quot;)</code>；<code>httpx.Failure(w, 403, &quot;IDENTITY_NOT_CURRENT&quot;)</code>；<code>httpx.Failure(w, 503, &quot;IDENTITY_UNAVAILABLE&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; len(raw) &gt; 8192 &#124;&#124; protojson.Unmarshal(raw, input) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=680:1889 -->
````go
func (h *IdentityRenewal) Renew(w http.ResponseWriter, r *http.Request) {
	// 没有浏览器入口或Cookie鉴权；只接收实际document证书，正文不包含原始会话令牌。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	input := &pb.IdentityRevalidation{}
	if err != nil || len(raw) > 8192 || protojson.Unmarshal(raw, input) != nil {
		httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		return
	}
	proof, err := h.logic.RenewOperationAdmission(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdentityBinding):
			httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		case errors.Is(err, domain.ErrIdentityDenied):
			httpx.Failure(w, 403, "IDENTITY_NOT_CURRENT")
		default:
			httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		}
		return
	}
	body, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(proof)
	if err != nil {
		httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

````

## 第 5 步：IdentityRenewal.DocumentCollaborator

插入位置：第 4 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func (h *IdentityRenewal) DocumentCollaborator(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Failure(w, 400, &quot;IDENTITY_BINDING_INVALID&quot;)</code>；<code>httpx.Failure(w, 403, &quot;IDENTITY_NOT_CURRENT&quot;)</code>；<code>httpx.Failure(w, 503, &quot;IDENTITY_UNAVAILABLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; len(raw) &gt; 8192 &#124;&#124; protojson.Unmarshal(raw, input) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1889:3009 -->
````go
func (h *IdentityRenewal) DocumentCollaborator(w http.ResponseWriter, r *http.Request) {
	// 只信任CA核验后的Doc服务身份；浏览器Cookie或自报角色都不能查询目标姓名。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	input := &pb.IdentityRevalidation{}
	if err != nil || len(raw) > 8192 || protojson.Unmarshal(raw, input) != nil {
		httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		return
	}
	result, err := h.logic.DocumentCollaborator(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrIdentityBinding):
			httpx.Failure(w, 400, "IDENTITY_BINDING_INVALID")
		case errors.Is(err, domain.ErrIdentityDenied):
			httpx.Failure(w, 403, "IDENTITY_NOT_CURRENT")
		default:
			httpx.Failure(w, 503, "IDENTITY_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

// mutation.go：保留原实现，按业务合并文件。

````

## 第 6 步：Fingerprint

插入位置：第 5 步代码之后，紧接追加；完成稿约第 91 行。

本步声明：`type Fingerprint func(any) []byte`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 已经解码的本次请求值，用规范编码计算摘要。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 请求指纹字节，同操作号重试与原指纹比较，不能当权限证明。 |

<!-- write-step: 6 bytes=3009:3044 -->
````go
type Fingerprint func(any) []byte

````

## 第 7 步：MutationRequests

插入位置：第 6 步代码之后，紧接追加；完成稿约第 93 行。

MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。

本步声明：`type MutationRequests struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>fingerprint</code> | <code>Fingerprint</code> | 把规范请求值计算成摘要的函数依赖；同操作号重试需比对同一请求内容。 |

<!-- write-step: 7 bytes=3044:3232 -->
````go
// MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。
type MutationRequests struct {
	authentication *Authentication
	fingerprint    Fingerprint
}

````

## 第 8 步：NewMutationRequests

插入位置：第 7 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`func NewMutationRequests(authentication *Authentication, fingerprint Fingerprint) *MutationRequests`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>fingerprint</code> | <code>Fingerprint</code> | 在此声明中的实际使用：<code>fingerprint: fingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>实际返回表达式：<code>&amp;MutationRequests{authentication: authentication, fingerprint: fingerprint}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;MutationRequests{authentication: authentication, fingerprint: fingerprint}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3232:3421 -->
````go
func NewMutationRequests(authentication *Authentication, fingerprint Fingerprint) *MutationRequests {
	return &MutationRequests{authentication: authentication, fingerprint: fingerprint}
}

````

## 第 9 步：MutationRequests.Bind

插入位置：第 8 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func (h *MutationRequests) Bind(w http.ResponseWriter, r *http.Request, id string, input any) (domain.Mutation, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.authentication.Authenticate(w, r, true)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, true)</code>；<code>r.URL</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(id)</code>；<code>OperationID: id</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>h.fingerprint(struct { 		Path  string `json:&quot;path&quot;` 		Input any    `json:&quot;input&quot;` 	}{r.URL.Path, input})</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>实际返回表达式：<code>domain.Mutation{}</code>；<code>domain.Mutation{ 		OperationID: id, 		Path:        r.URL.Path, 		Fingerprint: fingerprint, 		Session:     session, 	}</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(id)</code>。
- 此处通过 <code>return domain.Mutation{}, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return domain.Mutation{}, false</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Mutation{ 		OperationID: id, 		Path:        r.URL.Path, 		Fingerprint: fingerprint, 		Session:     session, 	}, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3421:4120 -->
````go
func (h *MutationRequests) Bind(w http.ResponseWriter, r *http.Request, id string, input any) (domain.Mutation, bool) {
	if !domain.ValidOperationID(id) {
		httpx.Failure(w, 400, "OPERATION_ID_INVALID")
		return domain.Mutation{}, false
	}
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return domain.Mutation{}, false
	}
	// 路径和规范化后的输入仍使用原HMAC编码，已提交的幂等记录无需重写。
	fingerprint := h.fingerprint(struct {
		Path  string `json:"path"`
		Input any    `json:"input"`
	}{r.URL.Path, input})
	return domain.Mutation{
		OperationID: id,
		Path:        r.URL.Path,
		Fingerprint: fingerprint,
		Session:     session,
	}, true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/identity_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/02-identity-admission.md)。
