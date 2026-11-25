# 分步编写 `services/account/internal/server/rpc_server.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/080-rpc-server.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现IdentityInternal RPC适配，将已装配的身份/操作/授权用例转换为protobuf响应。

RPC方法不能临时创建依赖或发起新的关闭；调用者身份与deadline由平台拦截器统一把关。

写之前先按这条执行路线理解各段的连接：

1. Acquire/Revalidate调用logic并把领域绑定/拒绝/故障映射gRPC错误。
2. BindOperationResults和BindAuthorizationResults在Serve前完成；GetAccountOperation读取原回执。
3. FenceAccess只反向验证原关闭范围，GetAuthorizationChange返回固定授权结果。

本文件需要守住的边界：

- 未绑定依赖必须报不可用；查询不能变成隐式创建或扩大关闭范围。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/server' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/server/rpc_server.go') -and (Get-Item -LiteralPath 'services/account/internal/server/rpc_server.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/server/rpc_server.go')) { New-Item -ItemType File -Path 'services/account/internal/server/rpc_server.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:435 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	domain "collaboration/account/internal/domain/account"
	logic "collaboration/account/internal/logic/account"
	pb "collaboration/contracts/internalv1"
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authorization_result.go：保留原实现，按业务合并文件。

````

## 第 2 步：Identity.FenceAccess

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

调用者最小权限仍由平台冻结策略限定为Doc mTLS身份；这里不建立新的会话或关闭操作。

本步声明：`func (s *Identity) FenceAccess(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorizations.Fence(ctx, input)</code> |
| <code>input</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>s.authorizations.Fence(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationResultError(nil)</code>；<code>authorizationResultError(err)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.authorizations == nil</code>。条件成立时执行 <code>return nil, authorizationResultError(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, authorizationResultError(err)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=435:873 -->
````go
// 调用者最小权限仍由平台冻结策略限定为Doc mTLS身份；这里不建立新的会话或关闭操作。
func (s *Identity) FenceAccess(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error) {
	if s.authorizations == nil {
		return nil, authorizationResultError(nil)
	}
	result, err := s.authorizations.Fence(ctx, input)
	if err != nil {
		return nil, authorizationResultError(err)
	}
	return result, nil
}

````

## 第 3 步：Identity.GetAuthorizationChange

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func (s *Identity) GetAuthorizationChange(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorizations.Get(ctx, input)</code> |
| <code>input</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.authorizations.Get(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationResultError(nil)</code>；<code>authorizationResultError(err)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.authorizations == nil</code>。条件成立时执行 <code>return nil, authorizationResultError(nil)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, authorizationResultError(err)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=873:1206 -->
````go
func (s *Identity) GetAuthorizationChange(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error) {
	if s.authorizations == nil {
		return nil, authorizationResultError(nil)
	}
	result, err := s.authorizations.Get(ctx, input)
	if err != nil {
		return nil, authorizationResultError(err)
	}
	return result, nil
}

````

## 第 4 步：authorizationResultError

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func authorizationResultError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;rule)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(code, rule.Code)</code>；<code>status.Error(codes.Unavailable, &quot;AUTHORIZATION_PROOF_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;rule)</code>。
- 检查 <code>rule.Kind == domain.InvalidRequest</code>。
- 检查 <code>rule.Kind == domain.EntityNotFound</code>。
- 此处通过 <code>return status.Error(code, rule.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.Unavailable, &quot;AUTHORIZATION_PROOF_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1206:1672 -->
````go
func authorizationResultError(err error) error {
	var rule domain.RuleViolation
	if errors.As(err, &rule) {
		code := codes.Unavailable
		if rule.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		}
		if rule.Kind == domain.EntityNotFound {
			code = codes.NotFound
		}
		return status.Error(code, rule.Code)
	}
	return status.Error(codes.Unavailable, "AUTHORIZATION_PROOF_UNAVAILABLE")
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 5 步：Identity

插入位置：第 4 步代码之后，紧接追加；完成稿约第 56 行。

RPC只适配已装配用例；服务身份、冻结调用者策略和deadline继续由平台拦截器核验。

本步声明：`type Identity struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pb.UnimplementedIdentityInternalServer</code> | <code>pb.UnimplementedIdentityInternalServer</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>logic</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |
| <code>operations</code> | <code>*logic.OperationResults</code> | 使用 <code>logic.OperationResults</code> 类型；其定义处的职责：授权原退出/凭据操作的进度查询，并将账号操作映射为RPC最小固定结果。 |
| <code>authorizations</code> | <code>*logic.AuthorizationResults</code> | 使用 <code>logic.AuthorizationResults</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。 |

<!-- write-step: 5 bytes=1672:1968 -->
````go
// RPC只适配已装配用例；服务身份、冻结调用者策略和deadline继续由平台拦截器核验。
type Identity struct {
	pb.UnimplementedIdentityInternalServer
	logic          *logic.Identity
	operations     *logic.OperationResults
	authorizations *logic.AuthorizationResults
}

````

## 第 6 步：NewIdentity

插入位置：第 5 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func NewIdentity(service *logic.Identity) *Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：RPC只适配已装配用例；服务身份、冻结调用者策略和deadline继续由平台拦截器核验。<br>实际返回表达式：<code>&amp;Identity{logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Identity{logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1968:2058 -->
````go
func NewIdentity(service *logic.Identity) *Identity { return &Identity{logic: service} }

````

## 第 7 步：Identity.BindOperationResults

插入位置：第 6 步代码之后，紧接追加；完成稿约第 66 行。

与HTTP共享已装配的权限用例，平台装配完成后才Serve，不在查询时创建依赖。

本步声明：`func (s *Identity) BindOperationResults(operations *logic.OperationResults)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operations</code> | <code>*logic.OperationResults</code> | 使用 <code>logic.OperationResults</code> 类型；其定义处的职责：授权原退出/凭据操作的进度查询，并将账号操作映射为RPC最小固定结果。<br>在此声明中的实际使用：<code>s.operations = operations</code> |

<!-- write-step: 7 bytes=2058:2272 -->
````go
// 与HTTP共享已装配的权限用例，平台装配完成后才Serve，不在查询时创建依赖。
func (s *Identity) BindOperationResults(operations *logic.OperationResults) {
	s.operations = operations
}

````

## 第 8 步：Identity.BindAuthorizationResults

插入位置：第 7 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (s *Identity) BindAuthorizationResults(authorizations *logic.AuthorizationResults)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authorizations</code> | <code>*logic.AuthorizationResults</code> | 使用 <code>logic.AuthorizationResults</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>s.authorizations = authorizations</code> |

<!-- write-step: 8 bytes=2272:2400 -->
````go
func (s *Identity) BindAuthorizationResults(authorizations *logic.AuthorizationResults) {
	s.authorizations = authorizations
}

````

## 第 9 步：Identity.AcquireIdentityAdmission

插入位置：第 8 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func (s *Identity) AcquireIdentityAdmission(ctx context.Context, input *pb.IdentityRequest) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.logic.AcquireIdentityAdmission(ctx, input)</code> |
| <code>input</code> | <code>*pb.IdentityRequest</code> | 在此声明中的实际使用：<code>s.logic.AcquireIdentityAdmission(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>identityError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return proof, identityError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2400:2622 -->
````go
func (s *Identity) AcquireIdentityAdmission(ctx context.Context, input *pb.IdentityRequest) (*pb.IdentityAdmission, error) {
	proof, err := s.logic.AcquireIdentityAdmission(ctx, input)
	return proof, identityError(err)
}

````

## 第 10 步：Identity.RevalidateIdentityAdmission

插入位置：第 9 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, input *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.logic.RevalidateIdentityAdmission(ctx, input)</code> |
| <code>input</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>s.logic.RevalidateIdentityAdmission(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>identityError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return proof, identityError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2622:2855 -->
````go
func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, input *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	proof, err := s.logic.RevalidateIdentityAdmission(ctx, input)
	return proof, identityError(err)
}

````

## 第 11 步：identityError

插入位置：第 10 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`func identityError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>errors.Is(err, domain.ErrIdentityBinding)</code>；<code>errors.Is(err, domain.ErrIdentityDenied)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.InvalidArgument, &quot;IDENTITY_BINDING_INVALID&quot;)</code>；<code>status.Error(codes.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code>；<code>status.Error(codes.Unavailable, &quot;IDENTITY_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.InvalidArgument, &quot;IDENTITY_BINDING_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.PermissionDenied, &quot;IDENTITY_NOT_CURRENT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.Unavailable, &quot;IDENTITY_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2855:3324 -->
````go
func identityError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrIdentityBinding):
		return status.Error(codes.InvalidArgument, "IDENTITY_BINDING_INVALID")
	case errors.Is(err, domain.ErrIdentityDenied):
		return status.Error(codes.PermissionDenied, "IDENTITY_NOT_CURRENT")
	default:
		return status.Error(codes.Unavailable, "IDENTITY_UNAVAILABLE")
	}
}

// operation_result.go：保留原实现，按业务合并文件。

````

## 第 12 步：Identity.GetAccountOperation

插入位置：第 11 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func (s *Identity) GetAccountOperation(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.operations.Get(ctx, query)</code> |
| <code>query</code> | <code>*pb.AccountOperationQuery</code> | 在此声明中的实际使用：<code>s.operations.Get(ctx, query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>result</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(code, rule.Code)</code>；<code>status.Error(codes.Unavailable, &quot;ACCOUNT_OPERATION_RESULT_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;rule)</code>。
- 检查 <code>rule.Kind == domain.InvalidRequest</code>。
- 检查 <code>rule.Kind == domain.EntityNotFound</code>。
- 此处通过 <code>return nil, status.Error(code, rule.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, status.Error(codes.Unavailable, &quot;ACCOUNT_OPERATION_RESULT_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3324:3907 -->
````go
func (s *Identity) GetAccountOperation(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error) {
	result, err := s.operations.Get(ctx, query)
	if err == nil {
		return result, nil
	}
	var rule domain.RuleViolation
	if errors.As(err, &rule) {
		code := codes.Unavailable
		if rule.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		} else if rule.Kind == domain.EntityNotFound {
			code = codes.NotFound
		}
		return nil, status.Error(code, rule.Code)
	}
	return nil, status.Error(codes.Unavailable, "ACCOUNT_OPERATION_RESULT_UNAVAILABLE")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/server/rpc_server.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
