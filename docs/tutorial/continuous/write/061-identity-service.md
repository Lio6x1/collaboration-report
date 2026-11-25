# 分步编写 `services/account/internal/logic/account/identity_service.go`

先按 [A02 · 短期身份准入、重验与幂等写入](../lessons/account/02-identity-admission.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/061-identity-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。

会话有效只说明账号当前可用，具体组织资格仍要在权威行锁内检查，并绑定操作、用途、目标与来源。

写之前先按这条执行路线理解各段的连接：

1. AcquireIdentityAdmission 摘要化会话原始字节，按固定顺序锁权威行和gate，保存证明及所有来源后提交。
2. RevalidateIdentityAdmission 比较原目标和来源，锁原admission检查OPEN与期限；RenewOperationAdmission只对用途白名单生成新的准入号。
3. Mutations.Run 锁user/session/receipt后核对actor/kind/fingerprint，首次执行body，再同事务写结果和审计。
4. AuthorizationResults 反向核原关闭范围与实际Doc ACK；Cursors 把绑定、末ID、5分钟期限签入HMAC游标。

本文件需要守住的边界：

- 不得把过期准入原地延期；续期必须保持原会话、用途、目标和来源完全一致。
- 数据库故障不能解释成权限已撤销；未知提交必须按原号恢复。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/identity_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/identity_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/identity_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/identity_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:620 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// authorization_result.go：保留原实现，按业务合并文件。

````

## 第 2 步：AuthorizationResults

插入位置：第 1 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type AuthorizationResults struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>repository</code> | <code>*repository.AuthorizationResults</code> | 使用 <code>repository.AuthorizationResults</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。 |

<!-- write-step: 2 bytes=620:703 -->
````go
type AuthorizationResults struct {
	repository *repository.AuthorizationResults
}

````

## 第 3 步：NewAuthorizationResults

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func NewAuthorizationResults(db *sql.DB) *AuthorizationResults`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>repository.NewAuthorizationResults(db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AuthorizationResults</code> | 实际返回表达式：<code>&amp;AuthorizationResults{repository: repository.NewAuthorizationResults(db)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AuthorizationResults{repository: repository.NewAuthorizationResults(db)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=703:853 -->
````go
func NewAuthorizationResults(db *sql.DB) *AuthorizationResults {
	return &AuthorizationResults{repository: repository.NewAuthorizationResults(db)}
}

````

## 第 4 步：authorizationInvalid

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func authorizationInvalid() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;AUTHORIZATION_REQUEST_BINDING_INVALID&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;AUTHORIZATION_REQUEST_BINDING_INVALID&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=853:997 -->
````go
func authorizationInvalid() error {
	return domain.RuleViolation{Kind: domain.InvalidRequest, Code: "AUTHORIZATION_REQUEST_BINDING_INVALID"}
}

````

## 第 5 步：authorizationUnavailable

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func authorizationUnavailable() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;AUTHORIZATION_PROOF_UNAVAILABLE&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;AUTHORIZATION_PROOF_UNAVAILABLE&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=997:1147 -->
````go
func authorizationUnavailable() error {
	return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "AUTHORIZATION_PROOF_UNAVAILABLE"}
}

````

## 第 6 步：AuthorizationResults.facts

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (s *AuthorizationResults) facts(ctx context.Context, operation string) (repository.AuthorizationResultFacts, *pb.FenceRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Read(ctx, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>s.repository.Read(ctx, operation)</code>；<code>authorizationProof(operation, f)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.AuthorizationResultFacts</code> | 使用 <code>repository.AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>实际返回表达式：<code>repository.AuthorizationResultFacts{}</code>；<code>f</code> |
| <code>(匿名/未命名)</code> | <code>*pb.FenceRequest</code> | 实际返回表达式：<code>nil</code>；<code>request</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationInvalid()</code>；<code>domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;AUTHORIZATION_CHANGE_NOT_FOUND&quot;}</code>；<code>authorizationUnavailable()</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。条件成立时执行 <code>return repository.AuthorizationResultFacts{}, nil, authorizationInvalid()</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return f, nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;AUTHORIZATION_CHANGE_NOT_FOUND&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return f, nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 此处通过 <code>return f, request, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1147:1741 -->
````go
func (s *AuthorizationResults) facts(ctx context.Context, operation string) (repository.AuthorizationResultFacts, *pb.FenceRequest, error) {
	if !domain.ValidOperationID(operation) {
		return repository.AuthorizationResultFacts{}, nil, authorizationInvalid()
	}
	f, err := s.repository.Read(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return f, nil, domain.RuleViolation{Kind: domain.EntityNotFound, Code: "AUTHORIZATION_CHANGE_NOT_FOUND"}
	}
	if err != nil {
		return f, nil, authorizationUnavailable()
	}
	request, err := authorizationProof(operation, f)
	return f, request, err
}

````

## 第 7 步：AuthorizationResults.Fence

插入位置：第 6 步代码之后，紧接追加；完成稿约第 59 行。

Fence只反向证明Account原关闭范围，绝不登记新范围或再调用Doc Fence。
真实Doc ACK未落库时返回PROCESSING，由原Account持久协调器继续恢复。

本步声明：`func (s *AuthorizationResults) Fence(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.facts(ctx, input.RevocationId)</code> |
| <code>input</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>platform.ValidBootstrapFence(input)</code>；<code>input.RevocationId</code>；<code>proto.Equal(original, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.FenceResult{State: pb.FenceResult_PROCESSING, RevocationId: original.RevocationId, ClosedScopes: original.ClosedScopes, Generation: original.Generation}</code>；<code>platform.BootstrapAck(original, &quot;account&quot;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationInvalid()</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidBootstrapFence(input)</code>。条件成立时执行 <code>return nil, authorizationInvalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(original, input)</code>。条件成立时执行 <code>return nil, authorizationInvalid()</code>。当前函数在这里结束。
- 检查 <code>f.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return &amp;pb.FenceResult{State: pb.FenceResult_PROCESSING, RevocationId: original.RevocationId, ClosedScopes: original.ClosedScopes, Generation: original.Generation}, nil</code>。当前函数在这里结束。
- 此处通过 <code>return platform.BootstrapAck(original, &quot;account&quot;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1741:2659 -->
````go
// Fence只反向证明Account原关闭范围，绝不登记新范围或再调用Doc Fence。
// 真实Doc ACK未落库时返回PROCESSING，由原Account持久协调器继续恢复。
func (s *AuthorizationResults) Fence(ctx context.Context, input *pb.FenceRequest) (*pb.FenceResult, error) {
	if !platform.ValidBootstrapFence(input) {
		return nil, authorizationInvalid()
	}
	f, original, err := s.facts(ctx, input.RevocationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(original, input) {
		return nil, authorizationInvalid()
	}
	if f.State != "SUCCEEDED" {
		return &pb.FenceResult{State: pb.FenceResult_PROCESSING, RevocationId: original.RevocationId, ClosedScopes: original.ClosedScopes, Generation: original.Generation}, nil
	}
	// Account摘要只在原持久关闭和实际参与者ACK已核验后签出，不把空参与者当成功。
	return platform.BootstrapAck(original, "account"), nil
}

````

## 第 8 步：AuthorizationResults.Get

插入位置：第 7 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (s *AuthorizationResults) Get(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.facts(ctx, input.OperationId)</code> |
| <code>input</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Target</code>；<code>input.RequestFingerprint</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationInvalid()</code>；<code>err</code>；<code>authorizationUnavailable()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Target == nil &#124;&#124; len(input.RequestFingerprint) != sha256.Size</code>。条件成立时执行 <code>return nil, authorizationInvalid()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(input.Target, target) &#124;&#124; !bytes.Equal(input.RequestFingerprint, f.Fingerprint)</code>。条件成立时执行 <code>return nil, authorizationInvalid()</code>。当前函数在这里结束。
- 检查 <code>f.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>result.State == pb.OperationResult_PROCESSING</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2659:4237 -->
````go
func (s *AuthorizationResults) Get(ctx context.Context, input *pb.OperationQuery) (*pb.OperationResult, error) {
	if input == nil || input.Target == nil || len(input.RequestFingerprint) != sha256.Size {
		return nil, authorizationInvalid()
	}
	f, _, err := s.facts(ctx, input.OperationId)
	if err != nil {
		return nil, err
	}
	target, err := authorizationQueryTarget(f)
	if err != nil {
		return nil, authorizationUnavailable()
	}
	if !proto.Equal(input.Target, target) || !bytes.Equal(input.RequestFingerprint, f.Fingerprint) {
		return nil, authorizationInvalid()
	}
	state := "PROCESSING"
	result := &pb.OperationResult{OperationId: input.OperationId, State: pb.OperationResult_PROCESSING, Generation: 1}
	if f.State == "SUCCEEDED" {
		state = "SUCCEEDED"
		result.State = pb.OperationResult_SUCCEEDED
	}
	raw, err := json.Marshal(struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}{input.OperationId, state, f.Scope, f.Epoch})
	if err != nil {
		return nil, authorizationUnavailable()
	}
	if result.State == pb.OperationResult_PROCESSING {
		result.CurrentStateJsonUtf8 = raw
		return result, nil
	}
	// 原业务协议generation恒为1，不读可变worker租约代次。授权表无独立revision：
	// terminal_revision=1定义为这个不可变范围的OUTPUT_STOPPED固定版本，仅真实持久终态赋值。
	result.TerminalRevision, result.FixedResultJsonUtf8 = 1, raw
	digest := sha256.Sum256(raw)
	result.ResultDigest = digest[:]
	return result, nil
}

````

## 第 9 步：authorizationQueryTarget

插入位置：第 8 步代码之后，紧接追加；完成稿约第 121 行。

冻结Target只有文档字段，Account采用明确固定映射：所有文档ID为0，workspace_epoch
是原来源epoch，purpose为前缀+scope+两个原完整JSON目标的规范摘要。不能猜文档ID。
子撤权没有自己的receipt，指纹和operation_target明确取原ACCOUNT_CLOSE父回执。

本步声明：`func authorizationQueryTarget(f repository.AuthorizationResultFacts) (*pb.Target, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>f</code> | <code>repository.AuthorizationResultFacts</code> | 使用 <code>repository.AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>在此声明中的实际使用：<code>f.Target</code>；<code>f.ReceiptTarget</code>；<code>f.TaskOperation</code>；<code>f.Epoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.Target</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.Target{WorkspaceEpoch: f.Epoch, Purpose: &quot;ACCOUNT_AUTHORIZATION_CHANGE:&quot; + f.Scope + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;pb.Target{WorkspaceEpoch: f.Epoch, Purpose: &quot;ACCOUNT_AUTHORIZATION_CHANGE:&quot; + f.Scope + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4237:5318 -->
````go
// 冻结Target只有文档字段，Account采用明确固定映射：所有文档ID为0，workspace_epoch
// 是原来源epoch，purpose为前缀+scope+两个原完整JSON目标的规范摘要。不能猜文档ID。
// 子撤权没有自己的receipt，指纹和operation_target明确取原ACCOUNT_CLOSE父回执。
func authorizationQueryTarget(f repository.AuthorizationResultFacts) (*pb.Target, error) {
	authorization, err := canonicalAuthorizationJSON(f.Target)
	if err != nil {
		return nil, err
	}
	operation, err := canonicalAuthorizationJSON(f.ReceiptTarget)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(struct {
		Operation     string          `json:"receipt_operation_id"`
		Authorization json.RawMessage `json:"authorization_target"`
		Target        json.RawMessage `json:"operation_target"`
	}{f.TaskOperation, authorization, operation})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &pb.Target{WorkspaceEpoch: f.Epoch, Purpose: "ACCOUNT_AUTHORIZATION_CHANGE:" + f.Scope + ":" + hex.EncodeToString(digest[:])}, nil
}

````

## 第 10 步：canonicalAuthorizationJSON

插入位置：第 9 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func canonicalAuthorizationJSON(raw []byte) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>json.Marshal(value)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>json.Marshal(value)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;AUTHORIZATION_TARGET_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>d.Decode(&amp;value) != nil &#124;&#124; d.Decode(new(any)) != io.EOF &#124;&#124; value == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;AUTHORIZATION_TARGET_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.Marshal(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5318:5623 -->
````go
func canonicalAuthorizationJSON(raw []byte) ([]byte, error) {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF || value == nil {
		return nil, errors.New("AUTHORIZATION_TARGET_INVALID")
	}
	return json.Marshal(value)
}

````

## 第 11 步：sameAuthorizationJSON

插入位置：第 10 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func sameAuthorizationJSON(first, second []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>first</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>canonicalAuthorizationJSON(first)</code> |
| <code>second</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>canonicalAuthorizationJSON(second)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; bytes.Equal(a, b)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(a, b)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5623:5848 -->
````go
func sameAuthorizationJSON(first, second []byte) bool {
	a, err := canonicalAuthorizationJSON(first)
	if err != nil {
		return false
	}
	b, err := canonicalAuthorizationJSON(second)
	return err == nil && bytes.Equal(a, b)
}

````

## 第 12 步：authorizationProof

插入位置：第 11 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func authorizationProof(operation string, f repository.AuthorizationResultFacts) (*pb.FenceRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>validAuthorizationOriginal(operation, f)</code>；<code>RevocationId: operation</code>；<code>f.GateRevocation != operation</code> |
| <code>f</code> | <code>repository.AuthorizationResultFacts</code> | 使用 <code>repository.AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>在此声明中的实际使用：<code>f.Scope</code>；<code>f.Epoch</code>；<code>f.Actor</code>；<code>f.ReceiptActor</code><br>另有 15 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceRequest</code> | 实际返回表达式：<code>nil</code>；<code>request</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>authorizationUnavailable()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidAccountOutputScope(f.Scope) &#124;&#124; f.Epoch == 0 &#124;&#124; f.Actor != f.ReceiptActor &#124;&#124; len(f.Fingerprint) != sha256.Size &#124;&#124; 		(f.State != &quot;FENCING&quot; &amp;&amp; f.State != &quot;SUCCEEDED&quot;) &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>!validAuthorizationOriginal(operation, f)</code>。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>(f.State == &quot;FENCING&quot; &amp;&amp; f.ReceiptState != &quot;PREPARING&quot;) &#124;&#124; 		(f.State == &quot;SUCCEEDED&quot; &amp;&amp; f.TaskKind != domain.AccountCloseFenceKind &amp;&amp; f.TaskKind != repository.OrganizationCloseReco …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>strings.HasPrefix(f.Scope, &quot;session:&quot;)</code>。
- 检查 <code>!platform.ValidBootstrapFence(request)</code>。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>f.GateEpoch &lt; f.Epoch &#124;&#124; (f.GateEpoch == f.Epoch &amp;&amp; (f.GateState != &quot;BLOCKED&quot; &#124;&#124; f.GateRevocation != operation)) &#124;&#124; 		(f.GateEpoch &gt; f.Epoch &amp;&amp; (f.State != &quot;SUCCEEDED&quot; &#124;&#124; (f.GateSt …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>f.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>f.AckEpoch != f.Epoch &#124;&#124; f.AckPhase != &quot;OUTPUT_STOPPED&quot; &#124;&#124; !f.AckCompleted &#124;&#124; !bytes.Equal(f.AckDigest, platform.BootstrapDigest(request, &quot;document&quot;)) &#124;&#124; 			(f.TaskKind != reposito …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 检查 <code>len(f.AckDigest) != 0 &#124;&#124; f.TaskState == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil, authorizationUnavailable()</code>。当前函数在这里结束。
- 此处通过 <code>return request, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5848:8378 -->
````go
func authorizationProof(operation string, f repository.AuthorizationResultFacts) (*pb.FenceRequest, error) {
	var participants []string
	if !platform.ValidAccountOutputScope(f.Scope) || f.Epoch == 0 || f.Actor != f.ReceiptActor || len(f.Fingerprint) != sha256.Size ||
		(f.State != "FENCING" && f.State != "SUCCEEDED") || (f.ReceiptState != "PREPARING" && f.ReceiptState != "SUCCEEDED") ||
		json.Unmarshal(f.Participants, &participants) != nil || len(participants) != 1 || participants[0] != "document" || !validAuthorizationTaskState(f.TaskState) {
		return nil, authorizationUnavailable()
	}
	if !validAuthorizationOriginal(operation, f) {
		return nil, authorizationUnavailable()
	}
	// 普通关闭在同一事务结束receipt；账号/组织关闭的来源ACK先于全局业务终态，
	// 不能把它们仍在清理的父回执误当缺少来源证明。
	if (f.State == "FENCING" && f.ReceiptState != "PREPARING") ||
		(f.State == "SUCCEEDED" && f.TaskKind != domain.AccountCloseFenceKind && f.TaskKind != repository.OrganizationCloseRecoveryKind && (f.ReceiptState != "SUCCEEDED" || !f.ReceiptTerminal)) {
		return nil, authorizationUnavailable()
	}
	request := &pb.FenceRequest{RevocationId: operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: f.Scope, Epoch: f.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(f.Scope, f.Epoch)}
	if strings.HasPrefix(f.Scope, "session:") {
		request.TargetDigest = platform.LogoutTargetDigest(strings.TrimPrefix(f.Scope, "session:"))
	}
	if !platform.ValidBootstrapFence(request) {
		return nil, authorizationUnavailable()
	}
	// 同代gate须仍是原关闭点；更高代次只允许带真实终态ACK的旧范围重读。
	// 新身份来源可重新OPEN，但不能抹掉已完成的旧epoch或替旧未知范围补证明。
	if f.GateEpoch < f.Epoch || (f.GateEpoch == f.Epoch && (f.GateState != "BLOCKED" || f.GateRevocation != operation)) ||
		(f.GateEpoch > f.Epoch && (f.State != "SUCCEEDED" || (f.GateState != "OPEN" && f.GateState != "BLOCKED"))) {
		return nil, authorizationUnavailable()
	}
	if f.State == "SUCCEEDED" {
		if f.AckEpoch != f.Epoch || f.AckPhase != "OUTPUT_STOPPED" || !f.AckCompleted || !bytes.Equal(f.AckDigest, platform.BootstrapDigest(request, "document")) ||
			(f.TaskKind != repository.OrganizationCloseRecoveryKind && f.TaskState != "SUCCEEDED") {
			return nil, authorizationUnavailable()
		}
	} else if len(f.AckDigest) != 0 || f.TaskState == "SUCCEEDED" {
		return nil, authorizationUnavailable()
	}
	return request, nil
}

````

## 第 13 步：validAuthorizationTaskState

插入位置：第 12 步代码之后，紧接追加；完成稿约第 204 行。

本步声明：`func validAuthorizationTaskState(state string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>state</code> | <code>string</code> | 持久授权任务的状态；这里只接受QUEUED、RUNNING、RETRY、UNKNOWN、SUCCEEDED，其他状态不能作为可恢复授权事实。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8378:8542 -->
````go
func validAuthorizationTaskState(state string) bool {
	switch state {
	case "QUEUED", "RUNNING", "RETRY", "UNKNOWN", "SUCCEEDED":
		return true
	}
	return false
}

````

## 第 14 步：validAuthorizationOriginal

插入位置：第 13 步代码之后，紧接追加；完成稿约第 212 行。

本步声明：`func validAuthorizationOriginal(operation string, f repository.AuthorizationResultFacts) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>validAccountCloseAuthorization(operation, f)</code>；<code>f.TaskOperation != operation</code>；<code>b.Operation == operation</code>；<code>target.Operation == operation</code> |
| <code>f</code> | <code>repository.AuthorizationResultFacts</code> | 使用 <code>repository.AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>在此声明中的实际使用：<code>f.TaskKind</code>；<code>validAccountCloseAuthorization(operation, f)</code>；<code>f.TaskOperation</code>；<code>f.Target</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>validAccountCloseAuthorization(operation, f)</code>；<code>false</code>；<code>err == nil &amp;&amp; payloadErr == nil &amp;&amp; currentErr == nil &amp;&amp; payload.Binding == b &amp;&amp; current.Binding == b &amp;&amp; sameAuthorizationJSON(f.Payload, f.Target) &amp;&amp; 			b.Operation == operation &amp;&amp; …</code>（长表达式，按本步完整代码继续）；<code>f.Actor &gt; 0 &amp;&amp; f.ReceiptKind == &quot;SESSION_LOGOUT&quot; &amp;&amp; payload.Kind == &quot;&quot; &amp;&amp; payload.UserID == &quot;&quot; &amp;&amp; payload.OrganizationID == &quot;&quot; &amp;&amp; payload.Role == &quot;&quot; &amp;&amp; f.Scope == &quot;session:&quot;+payloa …</code>（长表达式，按本步完整代码继续）<br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>f.TaskKind == domain.AccountCloseFenceKind</code>。条件成立时执行 <code>return validAccountCloseAuthorization(operation, f)</code>。当前函数在这里结束。
- 检查 <code>f.TaskOperation != operation</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>f.TaskKind == repository.OrganizationCloseRecoveryKind</code>。
- 此处通过 <code>return err == nil &amp;&amp; payloadErr == nil &amp;&amp; currentErr == nil &amp;&amp; payload.Binding == b &amp;&amp; current.Binding == b &amp;&amp; sameAuthorizationJSON(f.Payload, f.Target) &amp;&amp; 			b.Operation == opera …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;payload) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; payload.Scope != f.Scope &#124;&#124; payload.Epoch != strconv.FormatUint(f.Epoch, 10)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>f.TaskKind == &quot;SESSION_LOGOUT&quot;</code>。
- 此处通过 <code>return f.Actor &gt; 0 &amp;&amp; f.ReceiptKind == &quot;SESSION_LOGOUT&quot; &amp;&amp; payload.Kind == &quot;&quot; &amp;&amp; payload.UserID == &quot;&quot; &amp;&amp; payload.OrganizationID == &quot;&quot; &amp;&amp; payload.Role == &quot;&quot; &amp;&amp; f.Scope == &quot;session:&quot; …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidEntityID(payload.UserID) &#124;&#124; payload.SessionHash != &quot;&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>f.TaskKind == &quot;CREDENTIAL_CHANGE&quot;</code>。
- 此处通过 <code>return f.Actor &gt; 0 &amp;&amp; payload.UserID == strconv.FormatUint(f.Actor, 10) &amp;&amp; f.Scope == &quot;credential:&quot;+payload.UserID &amp;&amp; f.ReceiptKind == payload.Kind &amp;&amp; 			(payload.Kind == &quot;CHANGE_P …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!sameAuthorizationJSON(f.Target, f.Payload)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>f.TaskKind == &quot;PLATFORM_GOVERNANCE_CHANGE&quot;</code>。
- 此处通过 <code>return f.Actor == 0 &amp;&amp; payload.Kind == &quot;PLATFORM_GOVERNANCE_REVOKE&quot; &amp;&amp; payload.OrganizationID == &quot;&quot; &amp;&amp; payload.Role == &quot;&quot; &amp;&amp; f.Scope == &quot;platform-admin:&quot;+payload.UserID &amp;&amp; f.Receip …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>f.TaskKind != &quot;ORGANIZATION_CHANGE&quot; &#124;&#124; f.Actor == 0 &#124;&#124; !domain.ValidEntityID(payload.OrganizationID)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=8542:11733 -->
````go
func validAuthorizationOriginal(operation string, f repository.AuthorizationResultFacts) bool {
	if f.TaskKind == domain.AccountCloseFenceKind {
		return validAccountCloseAuthorization(operation, f)
	}
	if f.TaskOperation != operation {
		return false
	}
	if f.TaskKind == repository.OrganizationCloseRecoveryKind {
		original, err := domain.DecodeOrganizationClosePlan(f.Target)
		payload, payloadErr := domain.DecodeOrganizationClosePlan(f.Payload)
		current, currentErr := domain.DecodeOrganizationClosePlan(f.OrganizationPlan)
		b := original.Binding
		return err == nil && payloadErr == nil && currentErr == nil && payload.Binding == b && current.Binding == b && sameAuthorizationJSON(f.Payload, f.Target) &&
			b.Operation == operation && b.User == f.Actor && b.Organization == f.Organization && f.OrganizationActor == f.Actor && b.Scope() == f.Scope && b.Epoch == f.Epoch && f.ReceiptKind == "ORG_CLOSE" &&
			((f.State == "FENCING" && f.OrganizationState == "FENCING") || (f.State == "SUCCEEDED" && (f.OrganizationState == "APPLIED" || f.OrganizationState == "SUCCEEDED")))
	}
	var payload domain.SessionClosure
	decoder := json.NewDecoder(bytes.NewReader(f.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || payload.Scope != f.Scope || payload.Epoch != strconv.FormatUint(f.Epoch, 10) {
		return false
	}
	if f.TaskKind == "SESSION_LOGOUT" {
		var target map[string]string
		return f.Actor > 0 && f.ReceiptKind == "SESSION_LOGOUT" && payload.Kind == "" && payload.UserID == "" && payload.OrganizationID == "" && payload.Role == "" && f.Scope == "session:"+payload.SessionHash &&
			sameAuthorizationJSON(f.Target, f.ReceiptTarget) && json.Unmarshal(f.Target, &target) == nil && len(target) == 2 && target["scope_key"] == f.Scope && target["session_hash"] == payload.SessionHash
	}
	if !domain.ValidEntityID(payload.UserID) || payload.SessionHash != "" {
		return false
	}
	if f.TaskKind == "CREDENTIAL_CHANGE" {
		var target map[string]string
		return f.Actor > 0 && payload.UserID == strconv.FormatUint(f.Actor, 10) && f.Scope == "credential:"+payload.UserID && f.ReceiptKind == payload.Kind &&
			(payload.Kind == "CHANGE_PASSWORD" || payload.Kind == "RESET_PASSWORD" || payload.Kind == "CHANGE_EMAIL") && payload.OrganizationID == "" && payload.Role == "" && sameAuthorizationJSON(f.Target, f.ReceiptTarget) &&
			json.Unmarshal(f.Target, &target) == nil && len(target) == 3 && target["user_id"] == payload.UserID && target["scope_key"] == f.Scope
	}
	if !sameAuthorizationJSON(f.Target, f.Payload) {
		return false
	}
	if f.TaskKind == "PLATFORM_GOVERNANCE_CHANGE" {
		var target GovernanceOperatorInput
		return f.Actor == 0 && payload.Kind == "PLATFORM_GOVERNANCE_REVOKE" && payload.OrganizationID == "" && payload.Role == "" && f.Scope == "platform-admin:"+payload.UserID && f.ReceiptKind == "PLATFORM_GOVERNANCE_OPERATOR" &&
			json.Unmarshal(f.ReceiptTarget, &target) == nil && target.Operation == operation && strconv.FormatUint(target.User, 10) == payload.UserID && target.Action == "REVOKE"
	}
	if f.TaskKind != "ORGANIZATION_CHANGE" || f.Actor == 0 || !domain.ValidEntityID(payload.OrganizationID) {
````

## 第 15 步：validAuthorizationOriginal（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 257 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return f.ReceiptKind == kind &amp;&amp; f.Scope == prefix+payload.OrganizationID+&quot;:&quot;+payload.UserID</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11733:12122 -->
````go
		return false
	}
	prefix, kind := "org-member:", payload.Kind
	switch payload.Kind {
	case "ORG_ROLE", "ORG_REMOVE", "ORG_LEAVE":
	case "ORG_OWNER_TRANSFER":
		prefix = "org-owner:"
	case "ORG_CREATE_REVOKE":
		prefix = "org-create:"
		kind = "ORG_CREATION_GRANT"
	default:
		return false
	}
	return f.ReceiptKind == kind && f.Scope == prefix+payload.OrganizationID+":"+payload.UserID
}

````

## 第 16 步：validAccountCloseAuthorization

插入位置：第 15 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`func validAccountCloseAuthorization(operation string, f repository.AuthorizationResultFacts) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>p.Revocation != operation</code> |
| <code>f</code> | <code>repository.AuthorizationResultFacts</code> | 使用 <code>repository.AuthorizationResultFacts</code> 类型；其定义处的职责：AuthorizationResultFacts 是原关闭记录的一致快照；任务代次只用于租约，不能充当协议代次。<br>在此声明中的实际使用：<code>f.Payload</code>；<code>f.Capture</code>；<code>f.TaskOperation</code>；<code>f.Actor</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>matched == 1</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; captureErr != nil &#124;&#124; c.Binding == nil &#124;&#124; p.Operation != f.TaskOperation &#124;&#124; p.Revocation != operation &#124;&#124; p.User != f.Actor &#124;&#124; f.CloseUser != f.Actor &#124;&#124; p.Scope != f.Sc …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>scope.Scope == p.Scope &#124;&#124; scope.Revocation == p.Revocation</code>。
- 检查 <code>scope.Scope != p.Scope &#124;&#124; scope.Revocation != p.Revocation &#124;&#124; scope.Epoch != p.Epoch</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return matched == 1</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=12122:13666 -->
````go
func validAccountCloseAuthorization(operation string, f repository.AuthorizationResultFacts) bool {
	p, err := domain.DecodeAccountCloseFence(f.Payload)
	c, captureErr := domain.DecodeAccountCloseCapture(f.Capture)
	var target struct {
		Operation string `json:"account_close_operation"`
		Scope     string `json:"scope_key"`
		Epoch     uint64 `json:"epoch"`
	}
	var receipt map[string]string
	if err != nil || captureErr != nil || c.Binding == nil || p.Operation != f.TaskOperation || p.Revocation != operation || p.User != f.Actor || f.CloseUser != f.Actor || p.Scope != f.Scope || p.Epoch != f.Epoch ||
		f.ReceiptKind != "ACCOUNT_CLOSE" || c.Binding.Operation != p.Operation || c.Binding.User != p.User ||
		(f.CloseState != "FROZEN" && f.CloseState != "PLANNED" && f.CloseState != "CLEANING" && f.CloseState != "SUCCEEDED") || (f.State == "FENCING" && f.CloseState != "FROZEN") || json.Unmarshal(f.Target, &target) != nil || target.Operation != p.Operation || target.Scope != p.Scope || target.Epoch != p.Epoch ||
		json.Unmarshal(f.ReceiptTarget, &receipt) != nil || len(receipt) != 2 || receipt["user_id"] != strconv.FormatUint(p.User, 10) || receipt["session_hash"] != c.SessionHash {
		return false
	}
	matched := 0
	for _, scope := range c.Scopes {
		if scope.Scope == p.Scope || scope.Revocation == p.Revocation {
			if scope.Scope != p.Scope || scope.Revocation != p.Revocation || scope.Epoch != p.Epoch {
				return false
			}
			matched++
		}
	}
	return matched == 1
}

// cursor.go：保留原实现，按业务合并文件。

````

## 第 17 步：Cursors

插入位置：第 16 步代码之后，紧接追加；完成稿约第 302 行。

本步声明：`type Cursors struct{ secrets *domain.Secrets }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。 |

<!-- write-step: 17 bytes=13666:13714 -->
````go
type Cursors struct{ secrets *domain.Secrets }

````

## 第 18 步：NewCursors

插入位置：第 17 步代码之后，紧接追加；完成稿约第 304 行。

本步声明：`func NewCursors(secrets *domain.Secrets) *Cursors`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>secrets</code> | <code>*domain.Secrets</code> | 使用 <code>domain.Secrets</code> 类型；其定义处的职责：集中持有用于摘要和请求指纹的HMAC密钥；构造时复制字节，避免调用方后续修改切片改变摘要行为，回执也不必保存秘密原文。<br>在此声明中的实际使用：<code>secrets: secrets</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Cursors</code> | 实际返回表达式：<code>&amp;Cursors{secrets: secrets}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Cursors{secrets: secrets}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13714:13803 -->
````go
func NewCursors(secrets *domain.Secrets) *Cursors { return &Cursors{secrets: secrets} }

````

## 第 19 步：Cursors.Decode

插入位置：第 18 步代码之后，紧接追加；完成稿约第 306 行。

本步声明：`func (s *Cursors) Decode(token, binding string) (string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>string</code> | 在此声明中的实际使用：<code>token == &quot;&quot;</code>；<code>len(token)</code>；<code>strings.Split(token, &quot;.&quot;)</code> |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>cursor.Binding != binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;0&quot;</code>；<code>&quot;&quot;</code>；<code>cursor.Last</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 检查 <code>token == &quot;&quot;</code>。条件成立时执行 <code>return &quot;0&quot;, true</code>。当前函数在这里结束。
- 检查 <code>len(token) &gt; 2048</code>。条件成立时执行 <code>return &quot;&quot;, false</code>。当前函数在这里结束。
- 检查 <code>len(parts) != 2</code>。条件成立时执行 <code>return &quot;&quot;, false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; s.secrets.Digest(&quot;cursor\n&quot;+parts&#91;0&#93;) != parts&#91;1&#93; &#124;&#124; json.Unmarshal(raw, &amp;cursor) != nil &#124;&#124; cursor.Binding != binding &#124;&#124; cursor.Expires &lt; time.Now().Unix() &#124;&#124; !domain.ValidEntityID(cursor.Last)</code>。条件成立时执行 <code>return &quot;&quot;, false</code>。当前函数在这里结束。
- 此处通过 <code>return cursor.Last, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=13803:14504 -->
````go
func (s *Cursors) Decode(token, binding string) (string, bool) {
	if token == "" {
		return "0", true
	}
	if len(token) > 2048 {
		return "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	var cursor domain.CursorPayload
	// 原签名、所属账号/组织及目录修订号一起核验，不能把游标用于另一个权限范围。
	if err != nil || s.secrets.Digest("cursor\n"+parts[0]) != parts[1] || json.Unmarshal(raw, &cursor) != nil || cursor.Binding != binding || cursor.Expires < time.Now().Unix() || !domain.ValidEntityID(cursor.Last) {
		return "", false
	}
	return cursor.Last, true
}

````

## 第 20 步：Cursors.Encode

插入位置：第 19 步代码之后，紧接追加；完成稿约第 326 行。

本步声明：`func (s *Cursors) Encode(binding, last string) *string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>Binding: binding</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>Last: last</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*string</code> | 实际返回表达式：<code>&amp;token</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;token</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=14504:14873 -->
````go
func (s *Cursors) Encode(binding, last string) *string {
	raw, _ := json.Marshal(domain.CursorPayload{Binding: binding, Last: last, Expires: time.Now().Add(5 * time.Minute).Unix()})
	body := base64.RawURLEncoding.EncodeToString(raw)
	token := body + "." + s.secrets.Digest("cursor\n"+body)
	return &token
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 21 步：Identity

插入位置：第 20 步代码之后，紧接追加；完成稿约第 335 行。

本步声明：`type Identity struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Identity</code> | 使用 <code>repository.Identity</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。 |

<!-- write-step: 21 bytes=14873:14952 -->
````go
type Identity struct {
	db         *sql.DB
	repository *repository.Identity
}

````

## 第 22 步：NewIdentity

插入位置：第 21 步代码之后，紧接追加；完成稿约第 340 行。

本步声明：`func NewIdentity(db *sql.DB, repository *repository.Identity) *Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repository</code> | <code>*repository.Identity</code> | 使用 <code>repository.Identity</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。<br>在此声明中的实际使用：<code>repository: repository</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Identity</code> | 实际返回表达式：<code>&amp;Identity{db: db, repository: repository}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Identity{db: db, repository: repository}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=14952:15079 -->
````go
func NewIdentity(db *sql.DB, repository *repository.Identity) *Identity {
	return &Identity{db: db, repository: repository}
}

````

## 第 23 步：admissionBinding

插入位置：第 22 步代码之后，紧接追加；完成稿约第 344 行。

本步声明：`type admissionBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Proof</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;proof&quot;`</code> |

<!-- write-step: 23 bytes=15079:15142 -->
````go
type admissionBinding struct {
	Proof []byte `json:"proof"`
}

````

## 第 24 步：identityDenied

插入位置：第 23 步代码之后，紧接追加；完成稿约第 348 行。

本步声明：`func identityDenied() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityDenied</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.ErrIdentityDenied</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=15142:15211 -->
````go
func identityDenied() error      { return domain.ErrIdentityDenied }
````

## 第 25 步：identityUnavailable

插入位置：第 24 步代码之后，紧接追加；完成稿约第 349 行。

本步声明：`func identityUnavailable() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityUnavailable</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.ErrIdentityUnavailable</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=15211:15286 -->
````go
func identityUnavailable() error { return domain.ErrIdentityUnavailable }

````

## 第 26 步：Identity.AcquireIdentityAdmission

插入位置：第 25 步代码之后，紧接追加；完成稿约第 351 行。

本步声明：`func (s *Identity) AcquireIdentityAdmission(ctx context.Context, req *pb.IdentityRequest) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.SessionActor(ctx, tx, hash&#91;:&#93;, &amp;actor)</code>；<code>s.admissionSources(ctx, tx, actor, hash&#91;:&#93;, req.RequiredIdentityCapability, req.IdentityTarget, req.Purpose)</code>；<code>s.lockIdentityGates(ctx, tx, sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>req</code> | <code>*pb.IdentityRequest</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.OperationId</code>；<code>req.SessionToken</code>；<code>req.Purpose</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityBinding</code>；<code>identityUnavailable()</code>；<code>identityDenied()</code>；<code>err</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; !domain.ValidOperationID(req.OperationId) &#124;&#124; len(req.SessionToken) != 32 &#124;&#124; !domain.ValidIdentityPurpose(req.Purpose) &#124;&#124; req.Target == nil &#124;&#124; req.Target.Purpose != req.Purpose &#124;&#124; req.IdentityTarget == nil</code>。条件成立时执行 <code>return nil, domain.ErrIdentityBinding</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>s.repository.SessionActor(ctx, tx, hash&#91;:&#93;, &amp;actor) != nil</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.repository.SessionExpiry(ctx, tx, hash&#91;:&#93;, &amp;sessionExpiry) != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>sessionExpiry.Before(expires)</code>。

<!-- write-step: 26 bytes=15286:17143 -->
````go
func (s *Identity) AcquireIdentityAdmission(ctx context.Context, req *pb.IdentityRequest) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.OperationId) || len(req.SessionToken) != 32 || !domain.ValidIdentityPurpose(req.Purpose) || req.Target == nil || req.Target.Purpose != req.Purpose || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	hash := sha256.Sum256(req.SessionToken)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	// 先只定位actor，真正资格在固定锁顺序user→session→组织→member→gate中核验。
	var actor uint64
	if s.repository.SessionActor(ctx, tx, hash[:], &actor) != nil {
		return nil, identityDenied()
	}
	sources, err := s.admissionSources(ctx, tx, actor, hash[:], req.RequiredIdentityCapability, req.IdentityTarget, req.Purpose)
	if err != nil {
		return nil, err
	}
	if err = s.lockIdentityGates(ctx, tx, sources); err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(30 * time.Second)
	// 准入寿命取实际会话与协议上限的较小值，不能在会话自然到期后继续发出旧资格。
	var sessionExpiry time.Time
	if s.repository.SessionExpiry(ctx, tx, hash[:], &sessionExpiry) != nil {
		return nil, identityUnavailable()
	}
	if sessionExpiry.Before(expires) {
		expires = sessionExpiry
	}
	proof := &pb.IdentityAdmission{
		AdmissionId:    platform.NewTraceID(),
		ActorId:        actor,
		SessionRef:     hash[:],
		Sources:        sources,
		ExpiresUnixMs:  expires.UnixMilli(),
		Purpose:        req.Purpose,
		Target:         req.Target,
		IdentityTarget: req.IdentityTarget,
		Capability:     req.RequiredIdentityCapability,
		OperationId:    req.OperationId,
	}
	raw, _ := proto.Marshal(proof)
	binding, _ := json.Marshal(admissionBinding{Proof: raw})
````

## 第 27 步：Identity.AcquireIdentityAdmission（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 396 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=17143:17608 -->
````go
	if err = s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, actor, hash[:], req.OperationId, req.Purpose, string(binding), expires); err != nil {
		return nil, identityUnavailable()
	}
	for _, source := range sources {
		if err = s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch); err != nil {
			return nil, identityUnavailable()
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
````

## 第 28 步：Identity.RevalidateIdentityAdmission

插入位置：第 27 步代码之后，紧接追加；完成稿约第 409 行。

本步声明：`func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.revalidateIdentityAdmission(ctx, req, nil)</code> |
| <code>req</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>s.revalidateIdentityAdmission(ctx, req, nil)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 由 <code>s.revalidateIdentityAdmission(ctx, req, nil)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.revalidateIdentityAdmission(ctx, req, nil)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.revalidateIdentityAdmission(ctx, req, nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=17608:17795 -->
````go
func (s *Identity) RevalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	return s.revalidateIdentityAdmission(ctx, req, nil)
}

````

## 第 29 步：Identity.DocumentCollaborator

插入位置：第 28 步代码之后，紧接追加；完成稿约第 413 行。

DocumentCollaborator只解析Doc已选定的协作者、提及候选或申请人，不提供任意用户目录。
原证明绑定真实会话、文档、用途和目标；姓名与重验共用已锁定的账号事务，邮箱不参与查询。

本步声明：`func (s *Identity) DocumentCollaborator(ctx context.Context, req *pb.IdentityRevalidation) (dto.DocumentCollaborator, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.revalidateIdentityAdmission(ctx, req, func(ctx context.Context, tx *sql.Tx, proof *pb.IdentityAdmission) error { 		if proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT { 	 …</code>（长表达式，按本步完整代码继续） |
| <code>req</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.Purpose</code>；<code>req.Target</code>；<code>req.IdentityTarget</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.DocumentCollaborator</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityBinding</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; (req.Purpose != &quot;DOCUMENT_COLLABORATORS&quot; &amp;&amp; req.Purpose != &quot;DOCUMENT_MENTION_CANDIDATES&quot; &amp;&amp; req.Purpose != &quot;DOCUMENT_APPLICATION_NAMES&quot;) &#124;&#124; req.Target == nil &#124;&#124; req.T …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, domain.ErrIdentityBinding</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT</code>。条件成立时执行 <code>return identityDenied()</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; strings.TrimSpace(name) == &quot;&quot;</code>。条件成立时执行 <code>return identityUnavailable()</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=17795:19471 -->
````go
// DocumentCollaborator只解析Doc已选定的协作者、提及候选或申请人，不提供任意用户目录。
// 原证明绑定真实会话、文档、用途和目标；姓名与重验共用已锁定的账号事务，邮箱不参与查询。
func (s *Identity) DocumentCollaborator(ctx context.Context, req *pb.IdentityRevalidation) (dto.DocumentCollaborator, error) {
	var result dto.DocumentCollaborator
	// 明确限定这三个读取用途，不能让普通活跃账号证明变成任意账号姓名查询。
	if req == nil || (req.Purpose != "DOCUMENT_COLLABORATORS" && req.Purpose != "DOCUMENT_MENTION_CANDIDATES" && req.Purpose != "DOCUMENT_APPLICATION_NAMES") || req.Target == nil || req.Target.Purpose != req.Purpose || req.Target.DocumentId == 0 ||
		req.Target.UnitId != 0 || req.Target.BranchId != 0 || req.Target.StructureId != 0 || req.Target.WorkspaceEpoch != 0 ||
		req.IdentityTarget == nil || req.IdentityTarget.Kind != pb.IdentityTarget_USER || req.IdentityTarget.UserId == 0 || req.IdentityTarget.OrganizationId != 0 || req.IdentityTarget.SpaceId != 0 {
		return result, domain.ErrIdentityBinding
	}
	_, err := s.revalidateIdentityAdmission(ctx, req, func(ctx context.Context, tx *sql.Tx, proof *pb.IdentityAdmission) error {
		if proof.Capability != pb.IdentityCapability_ACTIVE_ACCOUNT {
			return identityDenied()
		}
		name, err := s.repository.TargetDisplayName(ctx, tx, proof.IdentityTarget.UserId)
		if err != nil || strings.TrimSpace(name) == "" {
			return identityUnavailable()
		}
		result = dto.DocumentCollaborator{UserId: strconv.FormatUint(proof.IdentityTarget.UserId, 10), DisplayName: name}
		return nil
	})
	return result, err
}

````

## 第 30 步：Identity.revalidateIdentityAdmission

插入位置：第 29 步代码之后，紧接追加；完成稿约第 437 行。

本步声明：`func (s *Identity) revalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation, read func(context.Context, *sql.Tx, *pb.IdentityAdmission) error) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Admission(ctx, s.db, req.AdmissionId, &amp;raw, &amp;state, &amp;expires)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.admissionSources(ctx, tx, proof.ActorId, proof.SessionRef, proof.Capability, proof.IdentityTarget, proof.Purpose)</code>；<code>s.lockIdentityGates(ctx, tx, current)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>req</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.AdmissionId</code>；<code>req.Target</code>；<code>req.IdentityTarget</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>read</code> | <code>func(context.Context, *sql.Tx, *pb.IdentityAdmission) error</code> | 在此声明中的实际使用：<code>read != nil</code>；<code>read(ctx, tx, proof)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityBinding</code>；<code>identityDenied()</code>；<code>identityUnavailable()</code>；<code>err</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; !domain.ValidOperationID(req.AdmissionId) &#124;&#124; req.Target == nil &#124;&#124; req.IdentityTarget == nil</code>。条件成立时执行 <code>return nil, domain.ErrIdentityBinding</code>。当前函数在这里结束。
- 检查 <code>s.repository.Admission(ctx, s.db, req.AdmissionId, &amp;raw, &amp;state, &amp;expires) != nil</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;binding) != nil &#124;&#124; proto.Unmarshal(binding.Proof, proof) != nil &#124;&#124; state != &quot;OPEN&quot; &#124;&#124; !expires.After(time.Now()) &#124;&#124; proof.OperationId != req.OperationId &#124;&#124; pro …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!sameSources(proof.Sources, current)</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockAdmission(ctx, tx, req.AdmissionId, &amp;state, &amp;expires) != nil &#124;&#124; state != &quot;OPEN&quot; &#124;&#124; !expires.After(time.Now())</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>read != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=19471:21292 -->
````go
func (s *Identity) revalidateIdentityAdmission(ctx context.Context, req *pb.IdentityRevalidation, read func(context.Context, *sql.Tx, *pb.IdentityAdmission) error) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.AdmissionId) || req.Target == nil || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	var raw []byte
	var state string
	var expires time.Time
	if s.repository.Admission(ctx, s.db, req.AdmissionId, &raw, &state, &expires) != nil {
		return nil, identityDenied()
	}
	var binding admissionBinding
	proof := &pb.IdentityAdmission{}
	if json.Unmarshal(raw, &binding) != nil || proto.Unmarshal(binding.Proof, proof) != nil || state != "OPEN" || !expires.After(time.Now()) || proof.OperationId != req.OperationId || proof.Purpose != req.Purpose || !proto.Equal(proof.Target, req.Target) || !proto.Equal(proof.IdentityTarget, req.IdentityTarget) {
		return nil, identityDenied()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	current, err := s.admissionSources(ctx, tx, proof.ActorId, proof.SessionRef, proof.Capability, proof.IdentityTarget, proof.Purpose)
	if err != nil {
		return nil, err
	}
	if !sameSources(proof.Sources, current) {
		return nil, identityDenied()
	}
	if err = s.lockIdentityGates(ctx, tx, current); err != nil {
		return nil, err
	}
	// 关闭事务也先锁相同权威行；此处锁admission防止查询后被并发关闭。
	if s.repository.LockAdmission(ctx, tx, req.AdmissionId, &state, &expires) != nil || state != "OPEN" || !expires.After(time.Now()) {
		return nil, identityDenied()
	}
	if read != nil {
		if err = read(ctx, tx, proof); err != nil {
			return nil, err
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}
````

## 第 31 步：Identity.currentSources

插入位置：第 30 步代码之后，紧接追加；完成稿约第 481 行。

本步声明：`func (s *Identity) currentSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, actor, &amp;state, &amp;credential, &amp;identity)</code>；<code>s.repository.LockSession(ctx, tx, session, &amp;sessionActor, &amp;state, &amp;sessionCredential, &amp;sessionEpoch, &amp;expiry)</code>；<code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetEpoch)</code>；<code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetIdentityEpoch)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, actor, &amp;state, &amp;credential, &amp;identity)</code>；<code>s.repository.LockSession(ctx, tx, session, &amp;sessionActor, &amp;state, &amp;sessionCredential, &amp;sessionEpoch, &amp;expiry)</code>；<code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetEpoch)</code>；<code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetIdentityEpoch)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, actor, &amp;state, &amp;credential, &amp;identity)</code>；<code>sessionActor != actor</code>；<code>strconv.FormatUint(actor, 10)</code>；<code>target.UserId != actor</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.repository.LockSession(ctx, tx, session, &amp;sessionActor, &amp;state, &amp;sessionCredential, &amp;sessionEpoch, &amp;expiry)</code>；<code>hex.EncodeToString(session)</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>capability == pb.IdentityCapability_ACTIVE_ACCOUNT</code>；<code>capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER</code>；<code>capability != pb.IdentityCapability_ORGANIZATION_ADMIN</code>；<code>capability != pb.IdentityCapability_ORGANIZATION_OWNER</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>*pb.IdentityTarget</code> | 在此声明中的实际使用：<code>target.Kind</code>；<code>target.UserId</code>；<code>target.OrganizationId</code>；<code>target.SpaceId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>identityDenied()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.repository.LockUser(ctx, tx, actor, &amp;state, &amp;credential, &amp;identity) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockSession(ctx, tx, session, &amp;sessionActor, &amp;state, &amp;sessionCredential, &amp;sessionEpoch, &amp;expiry) != nil &#124;&#124; sessionActor != actor &#124;&#124; state != &quot;ACTIVE&quot; &#124;&#124; sessionCredent …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>capability == pb.IdentityCapability_ACTIVE_ACCOUNT &#124;&#124; capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER</code>。
- 检查 <code>target.Kind != pb.IdentityTarget_USER &#124;&#124; target.UserId == 0 &#124;&#124; target.OrganizationId != 0 &#124;&#124; target.SpaceId != 0 &#124;&#124; (capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER &amp;&amp; target.UserId != actor)</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>target.UserId != actor</code>。
- 检查 <code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetEpoch) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 此处通过 <code>return sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>target.Kind != pb.IdentityTarget_ORGANIZATION &#124;&#124; target.OrganizationId == 0 &#124;&#124; target.SpaceId != 0</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>target.UserId != 0 &amp;&amp; target.UserId != actor</code>。
- 检查 <code>capability != pb.IdentityCapability_ORGANIZATION_ADMIN &amp;&amp; capability != pb.IdentityCapability_ORGANIZATION_OWNER &amp;&amp; capability != pb.IdentityCapability_ORGANIZATION_MEMBER</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockTargetIdentity(ctx, tx, target.UserId, &amp;state, &amp;targetIdentityEpoch) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockOrganization(ctx, tx, target.OrganizationId, &amp;owner, &amp;state) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。

<!-- write-step: 31 bytes=21292:24159 -->
````go
func (s *Identity) currentSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget) ([]*pb.SourceEpoch, error) {
	var state string
	var credential, identity uint64
	if s.repository.LockUser(ctx, tx, actor, &state, &credential, &identity) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	var sessionActor, sessionCredential, sessionEpoch uint64
	var expiry time.Time
	if s.repository.LockSession(ctx, tx, session, &sessionActor, &state, &sessionCredential, &sessionEpoch, &expiry) != nil || sessionActor != actor || state != "ACTIVE" || sessionCredential != credential || !expiry.After(time.Now()) {
		return nil, identityDenied()
	}
	user := strconv.FormatUint(actor, 10)
	sources := []*pb.SourceEpoch{{ScopeKey: "identity:" + user, Epoch: identity}, {ScopeKey: "credential:" + user, Epoch: credential}, {ScopeKey: "session:" + hex.EncodeToString(session), Epoch: sessionEpoch}}
	if capability == pb.IdentityCapability_ACTIVE_ACCOUNT || capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER {
		if target.Kind != pb.IdentityTarget_USER || target.UserId == 0 || target.OrganizationId != 0 || target.SpaceId != 0 || (capability == pb.IdentityCapability_PERSONAL_SPACE_OWNER && target.UserId != actor) {
			return nil, identityDenied()
		}
		// 文档管理者的个人协作授权需要核验实际目标账号；这里只证明活跃资格，
		// 文档管理权限仍由文档服务负责。证明携带目标身份来源，关闭账号会使它失效。
		if target.UserId != actor {
			var targetEpoch uint64
			if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		}
		return sources, nil
	}
	if target.Kind != pb.IdentityTarget_ORGANIZATION || target.OrganizationId == 0 || target.SpaceId != 0 {
		return nil, identityDenied()
	}
	var targetIdentityEpoch uint64
	if target.UserId != 0 && target.UserId != actor {
		if capability != pb.IdentityCapability_ORGANIZATION_ADMIN && capability != pb.IdentityCapability_ORGANIZATION_OWNER && capability != pb.IdentityCapability_ORGANIZATION_MEMBER {
			return nil, identityDenied()
		}
		// 目标user先于组织锁，与注销固定组织集合使用同一顺序，
		// 避免目标注销持user等组织、目标证明持组织等user的反向等待。
		if s.repository.LockTargetIdentity(ctx, tx, target.UserId, &state, &targetIdentityEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
	}
	org := strconv.FormatUint(target.OrganizationId, 10)
	var owner uint64
	if s.repository.LockOrganization(ctx, tx, target.OrganizationId, &owner, &state) != nil || state != "ACTIVE" {
````

## 第 32 步：Identity.currentSources（接着实现）

插入位置：第 31 步代码之后，紧接追加；完成稿约第 526 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, identityDenied()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>s.repository.LockMember(ctx, tx, target.OrganizationId, actor, &amp;role, &amp;state, &amp;memberEpoch) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>owner != actor &amp;&amp; role != &quot;ADMIN&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>owner != actor</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>owner == actor &#124;&#124; role == &quot;ADMIN&quot;</code>。
- 检查 <code>s.repository.LockCreationGrant(ctx, tx, target.OrganizationId, actor, &amp;grantState, &amp;revision) != nil &#124;&#124; grantState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>target.UserId != 0 &amp;&amp; target.UserId != actor</code>。
- 检查 <code>s.repository.LockTargetMember(ctx, tx, target.OrganizationId, target.UserId, &amp;state, &amp;targetEpoch) != nil &#124;&#124; state != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。

<!-- write-step: 32 bytes=24159:26423 -->
````go
		return nil, identityDenied()
	}
	var role string
	var memberEpoch uint64
	if s.repository.LockMember(ctx, tx, target.OrganizationId, actor, &role, &state, &memberEpoch) != nil || state != "ACTIVE" {
		return nil, identityDenied()
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + user, Epoch: memberEpoch})
	switch capability {
	case pb.IdentityCapability_ORGANIZATION_MEMBER:
	case pb.IdentityCapability_ORGANIZATION_ADMIN:
		if owner != actor && role != "ADMIN" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_OWNER:
		if owner != actor {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-owner:" + org + ":" + user, Epoch: memberEpoch})
	case pb.IdentityCapability_ORGANIZATION_CREATE_KNOWLEDGE_BASE:
		if owner == actor || role == "ADMIN" {
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-admin:" + org + ":" + user, Epoch: memberEpoch})
		} else {
			var grantState string
			var revision uint64
			if s.repository.LockCreationGrant(ctx, tx, target.OrganizationId, actor, &grantState, &revision) != nil || grantState != "ACTIVE" {
				return nil, identityDenied()
			}
			sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-create:" + org + ":" + user, Epoch: revision})
		}
	default:
		return nil, identityDenied()
	}
	// 任免目标也必须是当前成员；目标来源写入依赖，防止检查后被移除。
	if target.UserId != 0 && target.UserId != actor {
		var targetEpoch uint64
		if s.repository.LockTargetMember(ctx, tx, target.OrganizationId, target.UserId, &state, &targetEpoch) != nil || state != "ACTIVE" {
			return nil, identityDenied()
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "org-member:" + org + ":" + strconv.FormatUint(target.UserId, 10), Epoch: targetEpoch})
		// 目标必须活跃本身也是权限依赖；否则注销预留封住identity后，
		// 另一管理员仍可能凭旧目标证明把新的KB职责授给待注销账号。
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "identity:" + strconv.FormatUint(target.UserId, 10), Epoch: targetIdentityEpoch})
	}
````

## 第 33 步：Identity.currentSources（接着实现）

插入位置：第 32 步代码之后，紧接追加；完成稿约第 571 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=26423:26446 -->
````go
	return sources, nil
}
````

## 第 34 步：Identity.lockIdentityGates

插入位置：第 33 步代码之后，紧接追加；完成稿约第 573 行。

本步声明：`func (s *Identity) lockIdentityGates(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.EnsureGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>s.repository.LockGate(ctx, tx, source.ScopeKey, &amp;epoch, &amp;state)</code>；<code>s.repository.AdvanceGate(ctx, tx, source.ScopeKey, source.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.repository.EnsureGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>s.repository.LockGate(ctx, tx, source.ScopeKey, &amp;epoch, &amp;state)</code>；<code>s.repository.AdvanceGate(ctx, tx, source.ScopeKey, source.Epoch)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>sort.Slice(sources, func(i, j int) bool { return sources&#91;i&#93;.ScopeKey &lt; sources&#91;j&#93;.ScopeKey })</code>；<code>sources&#91;i&#93;.ScopeKey</code>；<code>sources&#91;j&#93;.ScopeKey</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>identityUnavailable()</code>；<code>identityDenied()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.ScopeKey &lt; sources&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockGate(ctx, tx, source.ScopeKey, &amp;epoch, &amp;state) != nil</code>。条件成立时执行 <code>return identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>source.Epoch &lt; epoch &#124;&#124; (source.Epoch == epoch &amp;&amp; state != &quot;OPEN&quot;)</code>。条件成立时执行 <code>return identityDenied()</code>。当前函数在这里结束。
- 检查 <code>source.Epoch &gt; epoch</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return identityUnavailable()</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=26446:27350 -->
````go
func (s *Identity) lockIdentityGates(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) error {
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	for _, source := range sources {
		if err := s.repository.EnsureGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
			return identityUnavailable()
		}
		var epoch uint64
		var state string
		if s.repository.LockGate(ctx, tx, source.ScopeKey, &epoch, &state) != nil {
			return identityUnavailable()
		}
		if source.Epoch < epoch || (source.Epoch == epoch && state != "OPEN") {
			return identityDenied()
		}
		// 只有重新核验权威来源后才可导入更高代次；旧关闭水位留存，不靠TTL删除。
		if source.Epoch > epoch {
			if err := s.repository.AdvanceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return identityUnavailable()
			}
		}
	}
	return nil
}
````

## 第 35 步：sameSources

插入位置：第 34 步代码之后，紧接追加；完成稿约第 596 行。

本步声明：`func sameSources(a, b []*pb.SourceEpoch) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(a)</code>；<code>sort.Slice(a, func(i, j int) bool { return a&#91;i&#93;.ScopeKey &lt; a&#91;j&#93;.ScopeKey })</code>；<code>a&#91;i&#93;.ScopeKey</code>；<code>a&#91;j&#93;.ScopeKey</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(b)</code>；<code>sort.Slice(b, func(i, j int) bool { return b&#91;i&#93;.ScopeKey &lt; b&#91;j&#93;.ScopeKey })</code>；<code>b&#91;i&#93;.ScopeKey</code>；<code>b&#91;j&#93;.ScopeKey</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(a) != len(b)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return a&#91;i&#93;.ScopeKey &lt; a&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return b&#91;i&#93;.ScopeKey &lt; b&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>a&#91;i&#93;.ScopeKey != b&#91;i&#93;.ScopeKey &#124;&#124; a&#91;i&#93;.Epoch != b&#91;i&#93;.Epoch</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=27350:27787 -->
````go
func sameSources(a, b []*pb.SourceEpoch) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i].ScopeKey < a[j].ScopeKey })
	sort.Slice(b, func(i, j int) bool { return b[i].ScopeKey < b[j].ScopeKey })
	for i := range a {
		if a[i].ScopeKey != b[i].ScopeKey || a[i].Epoch != b[i].Epoch {
			return false
		}
	}
	return true
}

// identity_renewal.go：保留原实现，按业务合并文件。

````

## 第 36 步：Identity.RenewOperationAdmission

插入位置：第 35 步代码之后，紧接追加；完成稿约第 612 行。

RenewOperationAdmission只用于白名单内已登记操作的后台续接。它签发新准入，绝不让旧过期证明重新有效。
调用方的document服务身份由独立HTTP入口核验；真实会话和原来源仍在本事务中重查。

本步声明：`func (s *Identity) RenewOperationAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Admission(ctx, s.db, req.AdmissionId, &amp;raw, &amp;state, &amp;oldExpiry)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.admissionSources(ctx, tx, original.ActorId, original.SessionRef, original.Capability, original.IdentityTarget, original.Purpose)</code>；<code>s.lockIdentityGates(ctx, tx, current)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>req</code> | <code>*pb.IdentityRevalidation</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.AdmissionId</code>；<code>req.OperationId</code>；<code>req.Target</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrIdentityBinding</code>；<code>identityDenied()</code>；<code>identityUnavailable()</code>；<code>err</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; !domain.ValidOperationID(req.AdmissionId) &#124;&#124; !domain.ValidOperationID(req.OperationId) &#124;&#124; req.Target == nil &#124;&#124; req.IdentityTarget == nil</code>。条件成立时执行 <code>return nil, domain.ErrIdentityBinding</code>。当前函数在这里结束。
- 此处通过 <code>return nil, identityDenied()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>req.Target.Purpose != req.Purpose</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>s.repository.Admission(ctx, s.db, req.AdmissionId, &amp;raw, &amp;state, &amp;oldExpiry) != nil</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;binding) != nil &#124;&#124; proto.Unmarshal(binding.Proof, original) != nil &#124;&#124; state != &quot;OPEN&quot; &#124;&#124; 		original.OperationId != req.OperationId &#124;&#124; original.Purpose != req.P …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!sameSources(original.Sources, current)</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.repository.LockAdmission(ctx, tx, req.AdmissionId, &amp;state, &amp;oldExpiry) != nil &#124;&#124; state != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。

<!-- write-step: 36 bytes=27787:30115 -->
````go
// RenewOperationAdmission只用于白名单内已登记操作的后台续接。它签发新准入，绝不让旧过期证明重新有效。
// 调用方的document服务身份由独立HTTP入口核验；真实会话和原来源仍在本事务中重查。
func (s *Identity) RenewOperationAdmission(ctx context.Context, req *pb.IdentityRevalidation) (*pb.IdentityAdmission, error) {
	if req == nil || !domain.ValidOperationID(req.AdmissionId) || !domain.ValidOperationID(req.OperationId) || req.Target == nil || req.IdentityTarget == nil {
		return nil, domain.ErrIdentityBinding
	}
	switch req.Purpose {
	case "SUBMISSION", "MERGE", "MAIN_CONTROL", "PUBLICATION", "PULL_MAIN", "STRUCTURE_PREVIEW", "DISCARD_PREVIEW", "DISCARD_DRAFT", "REVERSE", "CONFLICT_RESOLUTION":
		// 人工修复同样依赖原号证明续期；下方仍逐项核验原会话、目标和来源，不能借续期换权限。
	default:
		return nil, identityDenied()
	}
	if req.Target.Purpose != req.Purpose {
		return nil, identityDenied()
	}
	var raw []byte
	var state string
	var oldExpiry time.Time
	if s.repository.Admission(ctx, s.db, req.AdmissionId, &raw, &state, &oldExpiry) != nil {
		return nil, identityDenied()
	}
	var binding admissionBinding
	original := &pb.IdentityAdmission{}
	if json.Unmarshal(raw, &binding) != nil || proto.Unmarshal(binding.Proof, original) != nil || state != "OPEN" ||
		original.OperationId != req.OperationId || original.Purpose != req.Purpose ||
		!proto.Equal(original.Target, req.Target) || !proto.Equal(original.IdentityTarget, req.IdentityTarget) || len(original.SessionRef) != 32 {
		return nil, identityDenied()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, identityUnavailable()
	}
	defer tx.Rollback()
	// 与登出/凭据/角色变化共用user→session→组织→成员→gate锁顺序，不能把旧来源换成新权限。
	current, err := s.admissionSources(ctx, tx, original.ActorId, original.SessionRef, original.Capability, original.IdentityTarget, original.Purpose)
	if err != nil {
		return nil, err
	}
	if !sameSources(original.Sources, current) {
		return nil, identityDenied()
	}
	if err = s.lockIdentityGates(ctx, tx, current); err != nil {
		return nil, err
	}
	if s.repository.LockAdmission(ctx, tx, req.AdmissionId, &state, &oldExpiry) != nil || state != "OPEN" {
````

## 第 37 步：Identity.RenewOperationAdmission（接着实现）

插入位置：第 36 步代码之后，紧接追加；完成稿约第 657 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, identityDenied()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>s.repository.SessionExpiry(ctx, tx, original.SessionRef, &amp;sessionExpiry) != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>sessionExpiry.Before(expires)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, proof.ActorId, proof.SessionRef, proof.OperationId, proof.Purpose, string(newBinding), expires) != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch) != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, identityUnavailable()</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=30115:31331 -->
````go
		return nil, identityDenied()
	}
	var sessionExpiry time.Time
	if s.repository.SessionExpiry(ctx, tx, original.SessionRef, &sessionExpiry) != nil {
		return nil, identityUnavailable()
	}
	expires := time.Now().UTC().Add(30 * time.Second)
	if sessionExpiry.Before(expires) {
		expires = sessionExpiry
	}
	proof := proto.Clone(original).(*pb.IdentityAdmission)
	proof.AdmissionId, proof.ExpiresUnixMs = platform.NewTraceID(), expires.UnixMilli()
	proof.Sources = current
	encoded, err := proto.Marshal(proof)
	if err != nil {
		return nil, identityUnavailable()
	}
	newBinding, err := json.Marshal(admissionBinding{Proof: encoded})
	if err != nil {
		return nil, identityUnavailable()
	}
	if s.repository.CreateAdmission(ctx, tx, proof.AdmissionId, proof.ActorId, proof.SessionRef, proof.OperationId, proof.Purpose, string(newBinding), expires) != nil {
		return nil, identityUnavailable()
	}
	for _, source := range current {
		if s.repository.AttachSource(ctx, tx, proof.AdmissionId, source.ScopeKey, source.Epoch) != nil {
			return nil, identityUnavailable()
		}
	}
	if tx.Commit() != nil {
		return nil, identityUnavailable()
	}
	return proof, nil
}

// mutation.go：保留原实现，按业务合并文件。

````

## 第 38 步：MutationBody

插入位置：第 37 步代码之后，紧接追加；完成稿约第 694 行。

MutationBody使用同一私有库事务，不能另开池。成员离开等明确需要跨服务互斥的
用例仍在来源锁内调用有deadline的内部检查；调用本身不能代替本地持久关闭事实。

本步声明：`type MutationBody func(context.Context, *sql.Tx, domain.Session) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |
| <code>(匿名/未命名)</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 业务事务产生的结果对象，将写入原操作回执；只有外层事务提交确认后才可返回成功。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 38 bytes=31331:31644 -->
````go
// MutationBody使用同一私有库事务，不能另开池。成员离开等明确需要跨服务互斥的
// 用例仍在来源锁内调用有deadline的内部检查；调用本身不能代替本地持久关闭事实。
type MutationBody func(context.Context, *sql.Tx, domain.Session) (map[string]any, error)

````

## 第 39 步：Mutations

插入位置：第 38 步代码之后，紧接追加；完成稿约第 698 行。

本步声明：`type Mutations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Mutation</code> | 使用 <code>repository.Mutation</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。 |

<!-- write-step: 39 bytes=31644:31724 -->
````go
type Mutations struct {
	db         *sql.DB
	repository *repository.Mutation
}

````

## 第 40 步：NewMutations

插入位置：第 39 步代码之后，紧接追加；完成稿约第 703 行。

本步声明：`func NewMutations(db *sql.DB, repo *repository.Mutation) *Mutations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>*repository.Mutation</code> | 使用 <code>repository.Mutation</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。<br>在此声明中的实际使用：<code>repository: repo</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Mutations</code> | 实际返回表达式：<code>&amp;Mutations{db: db, repository: repo}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Mutations{db: db, repository: repo}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=31724:31842 -->
````go
func NewMutations(db *sql.DB, repo *repository.Mutation) *Mutations {
	return &Mutations{db: db, repository: repo}
}

````

## 第 41 步：Mutations.CheckSession

插入位置：第 40 步代码之后，紧接追加；完成稿约第 707 行。

本步声明：`func (s *Mutations) CheckSession(ctx context.Context, tx *sql.Tx, session domain.Session) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, session.UserID)</code>；<code>s.repository.LockSession(ctx, tx, session.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.repository.LockUser(ctx, tx, session.UserID)</code>；<code>s.repository.LockSession(ctx, tx, session.ID)</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.CredentialEpoch</code>；<code>session.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; user.State != &quot;ACTIVE&quot; &#124;&#124; user.Epoch != session.CredentialEpoch</code>。条件成立时执行 <code>return domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;ACTIVE&quot; &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=31842:32423 -->
````go
func (s *Mutations) CheckSession(ctx context.Context, tx *sql.Tx, session domain.Session) error {
	user, err := s.repository.LockUser(ctx, tx, session.UserID)
	if err != nil || user.State != "ACTIVE" || user.Epoch != session.CredentialEpoch {
		return domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	current, err := s.repository.LockSession(ctx, tx, session.ID)
	if err != nil || current.State != "ACTIVE" || !current.Expires.After(time.Now()) {
		return domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	return nil
}

````

## 第 42 步：Mutations.Run

插入位置：第 41 步代码之后，紧接追加；完成稿约第 719 行。

本步声明：`func (s *Mutations) Run(ctx context.Context, input domain.Mutation, body MutationBody) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.CheckSession(ctx, tx, input.Session)</code>；<code>s.repository.Receipt(ctx, tx, input.OperationID)</code>；<code>body(ctx, tx, input.Session)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>domain.Mutation</code> | 使用 <code>domain.Mutation</code> 类型；其定义处的职责：固定一笔账号写入的真实身份与请求内容。<br>在此声明中的实际使用：<code>input.OperationID</code>；<code>input.Session</code>；<code>input.Kind</code>；<code>input.Fingerprint</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>body</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：MutationBody使用同一私有库事务，不能另开池。成员离开等明确需要跨服务互斥的 用例仍在来源锁内调用有deadline的内部检查；调用本身不能代替本地持久关闭事实。<br>在此声明中的实际使用：<code>body(ctx, tx, input.Session)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>receipt.Result</code>；<code>result.JSON</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>；<code>err</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>；<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;OPERATION_PENDING&quot;}</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(input.OperationID)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>receipt.ActorID != input.Session.UserID &#124;&#124; receipt.Kind != input.Kind &#124;&#124; !bytes.Equal(receipt.Fingerprint, input.Fingerprint)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 检查 <code>len(receipt.Result) == 0</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;OPERATION_PENDING&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return receipt.Result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>s.repository.InsertReceipt(ctx, tx, input, result.State, result.JSON, target) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OPERATION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OPERATION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 42 bytes=32423:34062 -->
````go
func (s *Mutations) Run(ctx context.Context, input domain.Mutation, body MutationBody) (json.RawMessage, error) {
	if !domain.ValidOperationID(input.OperationID) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 原user→session→receipt锁顺序和提交边界不变；同号重试也核验当前会话。
	if err := s.CheckSession(ctx, tx, input.Session); err != nil {
		return nil, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, input.OperationID)
	if err == nil {
		if receipt.ActorID != input.Session.UserID || receipt.Kind != input.Kind || !bytes.Equal(receipt.Fingerprint, input.Fingerprint) {
			return nil, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		if len(receipt.Result) == 0 {
			return nil, domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "OPERATION_PENDING"}
		}
		return receipt.Result, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	value, err := body(ctx, tx, input.Session)
	if err != nil {
		return nil, err
	}
	result, err := platform.EncodeOperationReceipt(input.OperationID, value)
	if err != nil {
		return nil, err
	}
	target, err := json.Marshal(map[string]string{"path": input.Path})
	if err != nil {
		return nil, err
	}
	if s.repository.InsertReceipt(ctx, tx, input, result.State, result.JSON, target) != nil {
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	if err := s.repository.InsertAudit(ctx, tx, input, target); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
````

## 第 43 步：Mutations.Run（接着实现）

插入位置：第 42 步代码之后，紧接追加；完成稿约第 764 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, errors.New(&quot;OPERATION_COMMIT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result.JSON, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=34062:34145 -->
````go
		return nil, errors.New("OPERATION_COMMIT_UNKNOWN")
	}
	return result.JSON, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/identity_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/02-identity-admission.md)。
