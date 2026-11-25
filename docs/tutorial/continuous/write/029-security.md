# 分步编写 `packages/platform/security.go`

先按 [F03 · 配置、连接池、就绪与安全出口](../lessons/foundation/03-runtime-and-security.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/029-security.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证RPC调用服务身份和方法白名单，并输出安全HTTP错误。

mTLS说明谁在调用，rpc-policy说明能调用什么；缺一项都会把内部接口变成无边界入口。

写之前先按这条执行路线理解各段的连接：

1. AuthorizeRPC要求证书VerifiedChains与唯一SPIFFE角色，再要求0至5秒有限deadline。
2. RPCInterceptor通过后才执行handler；WriteSafeError只映射取消/超时与稳定503码。
3. WriteJSON统一no-store、nosniff和JSON Content-Type。

本文件需要守住的边界：

- 不能用浏览器头或证书CN自报服务角色；驱动原错误可能含SQL/凭据，禁止原文输出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/security.go') -and (Get-Item -LiteralPath 'packages/platform/security.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/security.go')) { New-Item -ItemType File -Path 'packages/platform/security.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:291 -->
````go
package platform

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

````

## 第 2 步：policyJSON

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`var policyJSON []byte`。

<!-- write-step: 2 bytes=291:341 -->
````go
//go:embed rpc-policy.json
var policyJSON []byte

````

## 第 3 步：rpcPolicy

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`var rpcPolicy = func() map[string][]string {`。

<!-- write-step: 3 bytes=341:529 -->
````go
var rpcPolicy = func() map[string][]string {
	var result map[string][]string
	if json.Unmarshal(policyJSON, &result) != nil {
		panic("invalid embedded RPC policy")
	}
	return result
}()

````

## 第 4 步：AuthorizeRPC

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func AuthorizeRPC(ctx context.Context, method string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>peer.FromContext(ctx)</code>；<code>ctx.Deadline</code> |
| <code>method</code> | <code>string</code> | grpc完整方法路径，用作内嵌rpc-policy.json的allowlist键。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unauthenticated, &quot;SERVICE_IDENTITY_REQUIRED&quot;)</code>；<code>status.Error(codes.Unauthenticated, &quot;AMBIGUOUS_SERVICE_IDENTITY&quot;)</code>；<code>status.Error(codes.InvalidArgument, &quot;FINITE_DEADLINE_REQUIRED&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return status.Error(codes.Unauthenticated, &quot;SERVICE_IDENTITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; len(info.State.VerifiedChains) == 0 &#124;&#124; len(info.State.PeerCertificates) == 0</code>。条件成立时执行 <code>return status.Error(codes.Unauthenticated, &quot;SERVICE_IDENTITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>uri.String() == &quot;spiffe://collaboration-report/&quot;+candidate</code>。
- 检查 <code>role != &quot;&quot; &amp;&amp; role != candidate</code>。条件成立时执行 <code>return status.Error(codes.Unauthenticated, &quot;AMBIGUOUS_SERVICE_IDENTITY&quot;)</code>。当前函数在这里结束。
- 检查 <code>role == &quot;&quot;</code>。条件成立时执行 <code>return status.Error(codes.Unauthenticated, &quot;SERVICE_IDENTITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; time.Until(deadline) &lt;= 0 &#124;&#124; time.Until(deadline) &gt; 5*time.Second</code>。条件成立时执行 <code>return status.Error(codes.InvalidArgument, &quot;FINITE_DEADLINE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>role == allowed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(codes.PermissionDenied, &quot;SERVICE_METHOD_FORBIDDEN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=529:1961 -->
````go
func AuthorizeRPC(ctx context.Context, method string) error {
	// 身份来自握手验证过的证书URI，禁止把浏览器头或证书CN当成服务能力。
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 || len(info.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	role := ""
	for _, uri := range info.State.PeerCertificates[0].URIs {
		for _, candidate := range []string{"account", "document", "search", "collaboration"} {
			if uri.String() == "spiffe://collaboration-report/"+candidate {
				if role != "" && role != candidate {
					return status.Error(codes.Unauthenticated, "AMBIGUOUS_SERVICE_IDENTITY")
				}
				role = candidate
			}
		}
	}
	if role == "" {
		return status.Error(codes.Unauthenticated, "SERVICE_IDENTITY_REQUIRED")
	}
	deadline, ok := ctx.Deadline()
	// 调用方必须明确给出期限；超长期限也拒绝，避免后台资源被未知调用占住。
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
		return status.Error(codes.InvalidArgument, "FINITE_DEADLINE_REQUIRED")
	}
	for _, allowed := range rpcPolicy[method] {
		if role == allowed {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "SERVICE_METHOD_FORBIDDEN")
}

````

## 第 5 步：RPCInterceptor

插入位置：第 4 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func RPCInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AuthorizeRPC(ctx, info.FullMethod)</code>；<code>handler(ctx, req)</code> |
| <code>req</code> | <code>any</code> | 在此声明中的实际使用：<code>handler(ctx, req)</code> |
| <code>info</code> | <code>*grpc.UnaryServerInfo</code> | 在此声明中的实际使用：<code>info.FullMethod</code> |
| <code>handler</code> | <code>grpc.UnaryHandler</code> | 在此声明中的实际使用：<code>handler(ctx, req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 由 <code>handler(ctx, req)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>handler(ctx, req)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return handler(ctx, req)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1961:2191 -->
````go
func RPCInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := AuthorizeRPC(ctx, info.FullMethod); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

````

## 第 6 步：ErrorResponse

插入位置：第 5 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`type ErrorResponse struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Code</code> | <code>string</code> | 可对外稳定错误码，如DEPENDENCY_TIMEOUT，绝非驱动原文。<br>序列化/校验标记：<code>`json:&quot;code&quot;`</code> |
| <code>Message</code> | <code>string</code> | 安全的人类可读提示，不含SQL/路径/凭据。<br>序列化/校验标记：<code>`json:&quot;message&quot;`</code> |
| <code>TraceID</code> | <code>string</code> | 随机追踪号用于关联请求；不授予查询权限。<br>序列化/校验标记：<code>`json:&quot;trace_id&quot;`</code> |
| <code>RetryAction</code> | <code>string</code> | 客户端下一步策略，如RETRY_SAME_OPERATION或RELOGIN。<br>序列化/校验标记：<code>`json:&quot;retry_action&quot;`</code> |

<!-- write-step: 6 bytes=2191:2373 -->
````go
type ErrorResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	TraceID     string `json:"trace_id"`
	RetryAction string `json:"retry_action"`
}

````

## 第 7 步：NewTraceID

插入位置：第 6 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func NewTraceID() string`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;trace-unavailable&quot;</code>；<code>hex.EncodeToString(bytes)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;trace-unavailable&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return hex.EncodeToString(bytes)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2373:2541 -->
````go
func NewTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "trace-unavailable"
	}
	return hex.EncodeToString(bytes)
}

````

## 第 8 步：WriteSafeError

插入位置：第 7 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func WriteSafeError(w http.ResponseWriter, trace string, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Code: code, Message: message, TraceID: trace, RetryAction: &quot;RETRY_SAME_OPERATION&quot;})</code> |
| <code>trace</code> | <code>string</code> | 在此声明中的实际使用：<code>TraceID: trace</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, context.DeadlineExceeded)</code>；<code>errors.Is(err, context.Canceled)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, context.DeadlineExceeded)</code>。
- 检查 <code>errors.Is(err, context.Canceled)</code>。

<!-- write-step: 8 bytes=2541:3092 -->
````go
func WriteSafeError(w http.ResponseWriter, trace string, err error) {
	// 原始驱动错误可能含SQL、凭据和私有路径，只交付稳定错误码与追踪号。
	code, message := "SERVICE_UNAVAILABLE", "服务暂时不可用，请稍后重试"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "DEPENDENCY_TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		code = "REQUEST_CANCELLED"
	}
	WriteJSON(w, http.StatusServiceUnavailable, ErrorResponse{Code: code, Message: message, TraceID: trace, RetryAction: "RETRY_SAME_OPERATION"})
}

````

## 第 9 步：WriteJSON

插入位置：第 8 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func WriteJSON(w http.ResponseWriter, code int, value any)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>w.Header</code>；<code>w.WriteHeader</code>；<code>json.NewEncoder(w)</code> |
| <code>code</code> | <code>int</code> | 在此声明中的实际使用：<code>w.WriteHeader(code)</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>json.NewEncoder(w).Encode(value)</code> |

<!-- write-step: 9 bytes=3092:3379 -->
````go
func WriteJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/security.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/03-runtime-and-security.md)。
