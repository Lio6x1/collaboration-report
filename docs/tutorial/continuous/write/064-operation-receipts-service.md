# 分步编写 `services/account/internal/logic/account/operation_receipts_service.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/064-operation-receipts-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

授权原退出/凭据操作的进度查询，并将账号操作映射为RPC最小固定结果。

关闭后的原秘密仅允许续取该操作；多次读取间状态变化不能被拼成固定终态。

写之前先按这条执行路线理解各段的连接：

1. Logout.Progress 用主Cookie或恢复Cookie核原actor/session绑定，仅返回状态。
2. OperationResults.Get 先读持久快照、调用现有Progress授权、再完整比较第二次快照。
3. 终态必须有terminal与固定结果，摘要仅覆盖operation_id/state的最小JSON投影。

本文件需要守住的边界：

- 不能声称最小投影摘要是整个result_json摘要。
- 两次快照不同要报不可用并重查，不把一阶段权限配给另一阶段结果。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/operation_receipts_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/operation_receipts_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/operation_receipts_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/operation_receipts_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:468 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// account_operation_result.go：保留原实现，按业务合并文件。

````

## 第 2 步：OperationResults

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type OperationResults struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>logout</code> | <code>*Logout</code> | 使用 <code>Logout</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。 |
| <code>closures</code> | <code>*AccountClosures</code> | 使用 <code>AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。 |

<!-- write-step: 2 bytes=468:565 -->
````go
type OperationResults struct {
	db       *sql.DB
	logout   *Logout
	closures *AccountClosures
}

````

## 第 3 步：NewOperationResults

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func NewOperationResults(db *sql.DB, logout *Logout, closures *AccountClosures) *OperationResults`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>logout</code> | <code>*Logout</code> | 使用 <code>Logout</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。<br>在此声明中的实际使用：<code>logout: logout</code> |
| <code>closures</code> | <code>*AccountClosures</code> | 使用 <code>AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>closures: closures</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OperationResults</code> | 实际返回表达式：<code>&amp;OperationResults{db: db, logout: logout, closures: closures}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OperationResults{db: db, logout: logout, closures: closures}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=565:738 -->
````go
func NewOperationResults(db *sql.DB, logout *Logout, closures *AccountClosures) *OperationResults {
	return &OperationResults{db: db, logout: logout, closures: closures}
}

````

## 第 4 步：minimalAccountResult

插入位置：第 3 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type minimalAccountResult struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |

<!-- write-step: 4 bytes=738:849 -->
````go
type minimalAccountResult struct {
	Operation string `json:"operation_id"`
	State     string `json:"state"`
}

````

## 第 5 步：operationResultNotFound

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func operationResultNotFound() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;OPERATION_NOT_FOUND&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;OPERATION_NOT_FOUND&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=849:978 -->
````go
func operationResultNotFound() error {
	return domain.RuleViolation{Kind: domain.EntityNotFound, Code: "OPERATION_NOT_FOUND"}
}

````

## 第 6 步：operationResultUnavailable

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func operationResultUnavailable() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;ACCOUNT_OPERATION_RESULT_UNAVAILABLE&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: &quot;ACCOUNT_OPERATION_RESULT_UNAVAILABLE&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=978:1135 -->
````go
func operationResultUnavailable() error {
	return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "ACCOUNT_OPERATION_RESULT_UNAVAILABLE"}
}

````

## 第 7 步：sameOperationResult

插入位置：第 6 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func sameOperationResult(left, right repository.OperationResultSnapshot) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>left</code> | <code>repository.OperationResultSnapshot</code> | 使用 <code>repository.OperationResultSnapshot</code> 类型；其定义处的职责：OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的 state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。<br>在此声明中的实际使用：<code>left.Actor</code>；<code>left.Kind</code>；<code>left.State</code>；<code>left.SessionHash</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>right</code> | <code>repository.OperationResultSnapshot</code> | 使用 <code>repository.OperationResultSnapshot</code> 类型；其定义处的职责：OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的 state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。<br>在此声明中的实际使用：<code>right.Actor</code>；<code>right.Kind</code>；<code>right.State</code>；<code>right.SessionHash</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>left.Actor == right.Actor &amp;&amp; left.Kind == right.Kind &amp;&amp; left.State == right.State &amp;&amp; 		left.SessionHash == right.SessionHash &amp;&amp; left.Generation == right.Generation &amp;&amp; left.Terminal …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return left.Actor == right.Actor &amp;&amp; left.Kind == right.Kind &amp;&amp; left.State == right.State &amp;&amp; 		left.SessionHash == right.SessionHash &amp;&amp; left.Generation == right.Generation &amp;&amp; left.T …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1135:1521 -->
````go
func sameOperationResult(left, right repository.OperationResultSnapshot) bool {
	return left.Actor == right.Actor && left.Kind == right.Kind && left.State == right.State &&
		left.SessionHash == right.SessionHash && left.Generation == right.Generation && left.Terminal == right.Terminal &&
		bytes.Equal(left.Fingerprint, right.Fingerprint) && bytes.Equal(left.Result, right.Result)
}

````

## 第 8 步：OperationResults.Get

插入位置：第 7 步代码之后，紧接追加；完成稿约第 49 行。

Get仅查询已存在的原操作。recovery_receipt沿HTTP恢复Cookie的真实语义处理：
它是原会话秘密，不是新发明的bearer；权限全部复用原Progress，不调用Resolve/Start。

本步声明：`func (s *OperationResults) Get(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReadOperationResult(ctx, s.db, query.OperationId)</code>；<code>s.closures.Progress(ctx, query.OperationId, primary, recovery)</code>；<code>s.logout.Progress(ctx, query.OperationId, primary, recovery)</code> |
| <code>query</code> | <code>*pb.AccountOperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.OperationId</code>；<code>query.SessionToken</code>；<code>query.RecoveryReceipt</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>；<code>operationResultUnavailable()</code>；<code>operationResultNotFound()</code>；<code>err</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; !domain.ValidOperationID(query.OperationId)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>s == nil &#124;&#124; s.db == nil &#124;&#124; s.logout == nil &#124;&#124; s.closures == nil</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, operationResultNotFound()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>before.Kind == &quot;ACCOUNT_CLOSE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, normalizeOperationResultError(err)</code>。当前函数在这里结束。
- 检查 <code>progress.State != &quot;&quot;</code>。
- 检查 <code>json.Unmarshal(progress.Result, &amp;authorized) != nil</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !sameOperationResult(before, after)</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>authorized.Operation != query.OperationId &#124;&#124; after.Generation == 0 &#124;&#124; len(after.Fingerprint) != 32</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>len(after.Result) &gt; 0</code>。

<!-- write-step: 8 bytes=1521:3670 -->
````go
// Get仅查询已存在的原操作。recovery_receipt沿HTTP恢复Cookie的真实语义处理：
// 它是原会话秘密，不是新发明的bearer；权限全部复用原Progress，不调用Resolve/Start。
func (s *OperationResults) Get(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error) {
	if query == nil || !domain.ValidOperationID(query.OperationId) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	if s == nil || s.db == nil || s.logout == nil || s.closures == nil {
		return nil, operationResultUnavailable()
	}
	before, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, operationResultNotFound()
	}
	if err != nil {
		return nil, err
	}
	primary, recovery := string(query.SessionToken), string(query.RecoveryReceipt)
	var authorized minimalAccountResult
	if before.Kind == "ACCOUNT_CLOSE" {
		plan, err := s.closures.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		authorized = minimalAccountResult{Operation: plan.OperationId, State: plan.State}
	} else {
		progress, err := s.logout.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		if progress.State != "" {
			authorized = minimalAccountResult{Operation: query.OperationId, State: progress.State}
		} else if json.Unmarshal(progress.Result, &authorized) != nil {
			return nil, operationResultUnavailable()
		}
	}
	// 原权限返回和本次RPC必须属于同一操作/会话绑定。读后比较整份持久快照，
	// 连同结果字节核验：即使关闭器没有递增generation也不能混用两阶段结果。
	after, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if err != nil || !sameOperationResult(before, after) {
		return nil, operationResultUnavailable()
	}
	if authorized.Operation != query.OperationId || after.Generation == 0 || len(after.Fingerprint) != 32 {
		return nil, operationResultUnavailable()
	}
	if len(after.Result) > 0 {
````

## 第 9 步：OperationResults.Get（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 94 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(after.Result, &amp;stored) != nil &#124;&#124; stored != authorized</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>authorized.State != after.State &#124;&#124; after.Kind == &quot;ACCOUNT_CLOSE&quot;</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!terminal</code>。
- 检查 <code>after.Terminal</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!after.Terminal &#124;&#124; len(after.Result) == 0 &#124;&#124; authorized.State != after.State</code>。条件成立时执行 <code>return nil, operationResultUnavailable()</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3670:5219 -->
````go
		var stored minimalAccountResult
		if json.Unmarshal(after.Result, &stored) != nil || stored != authorized {
			return nil, operationResultUnavailable()
		}
	} else if authorized.State != after.State || after.Kind == "ACCOUNT_CLOSE" {
		return nil, operationResultUnavailable()
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: after.Generation, State: pb.OperationResult_PROCESSING}
	terminal := true
	switch after.State {
	case "PREPARING":
		terminal = false
	case "SUCCEEDED":
		result.State = pb.OperationResult_SUCCEEDED
	case "REJECTED":
		result.State = pb.OperationResult_REJECTED
	case "CANCELLED":
		result.State = pb.OperationResult_CANCELLED
	default:
		return nil, operationResultUnavailable()
	}
	raw, err := json.Marshal(authorized)
	if err != nil {
		return nil, err
	}
	if !terminal {
		if after.Terminal {
			return nil, operationResultUnavailable()
		}
		result.CurrentStateJsonUtf8 = raw
		return result, nil
	}
	if !after.Terminal || len(after.Result) == 0 || authorized.State != after.State {
		return nil, operationResultUnavailable()
	}
	// generation来自原receipt；terminal_revision定义为持久终态的receipt代次，
	// 不采用后台租约代次或当前身份epoch。摘要只覆盖最小固定投影的JSON字节，
	// 不冒充完整result_json摘要，也不向已关闭会话交付实体/交接/保留清单。
	result.TerminalRevision = after.Generation
	result.FixedResultJsonUtf8 = raw
	digest := sha256.Sum256(raw)
	result.ResultDigest = digest[:]
	return result, nil
}

````

## 第 10 步：normalizeOperationResultError

插入位置：第 9 步代码之后，紧接追加；完成稿约第 139 行。

本步声明：`func normalizeOperationResultError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.Is(err, sql.ErrNoRows)</code>；<code>errors.As(err, &amp;rule)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>operationResultNotFound()</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows) &#124;&#124; (errors.As(err, &amp;rule) &amp;&amp; rule.Kind == domain.EntityNotFound)</code>。条件成立时执行 <code>return operationResultNotFound()</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5219:5529 -->
````go
func normalizeOperationResultError(err error) error {
	var rule domain.RuleViolation
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &rule) && rule.Kind == domain.EntityNotFound) {
		return operationResultNotFound()
	}
	return err
}

// operation_progress.go：保留原实现，按业务合并文件。

````

## 第 11 步：Logout.Progress

插入位置：第 10 步代码之后，紧接追加；完成稿约第 149 行。

Progress对已关闭的原会话只返回该操作的最小进度，不交付目标或其他业务投影。

本步声明：`func (s *Logout) Progress(ctx context.Context, id, primaryToken, recoveryToken string) (domain.OperationProgress, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.receipts.Lookup(ctx, id)</code>；<code>s.authentication.Session(ctx, token)</code>；<code>s.authentication.Session(ctx, primaryToken)</code>；<code>s.receipts.CurrentCredential(ctx, session.UserID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.receipts.Lookup(ctx, id)</code>；<code>s.receipts.Result(ctx, id)</code> |
| <code>primaryToken</code> | <code>string</code> | 在此声明中的实际使用：<code>s.authentication.Session(ctx, primaryToken)</code> |
| <code>recoveryToken</code> | <code>string</code> | 主会话已关闭后用于核对原退出回执的恢复令牌；必须匹配回执会话，不是重新登录。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OperationProgress</code> | 使用 <code>domain.OperationProgress</code> 类型；其定义处的职责：OperationProgress只携带持久结果或原关闭操作的状态，URL由HTTP适配层生成。<br>实际返回表达式：<code>domain.OperationProgress{}</code>；<code>domain.OperationProgress{State: receipt.State}</code>；<code>domain.OperationProgress{Result: raw}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>notFound</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return domain.OperationProgress{}, notFound</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.OperationProgress{}, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; boundOperation &amp;&amp; receipt.ActorID == session.UserID &amp;&amp; receipt.SessionHash == hex.EncodeToString(session.ID)</code>。条件成立时执行 <code>return domain.OperationProgress{State: receipt.State}, nil</code>。当前函数在这里结束。
- 检查 <code>!boundOperation</code>。
- 检查 <code>err == nil &amp;&amp; session.State == &quot;ACTIVE&quot; &amp;&amp; session.Expires.After(time.Now()) &amp;&amp; receipt.ActorID == session.UserID</code>。
- 检查 <code>err == nil &amp;&amp; credential.State == &quot;ACTIVE&quot; &amp;&amp; credential.Epoch == session.CredentialEpoch</code>。
- 检查 <code>err == nil &amp;&amp; len(raw) &gt; 0</code>。条件成立时执行 <code>return domain.OperationProgress{Result: raw}, nil</code>。当前函数在这里结束。
- 此处通过 <code>return domain.OperationProgress{}, notFound</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5529:7129 -->
````go
// Progress对已关闭的原会话只返回该操作的最小进度，不交付目标或其他业务投影。
func (s *Logout) Progress(ctx context.Context, id, primaryToken, recoveryToken string) (domain.OperationProgress, error) {
	receipt, err := s.receipts.Lookup(ctx, id)
	notFound := domain.RuleViolation{Kind: domain.EntityNotFound, Code: "OPERATION_NOT_FOUND"}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperationProgress{}, notFound
	}
	if err != nil {
		return domain.OperationProgress{}, err
	}
	boundOperation := receipt.Kind == "SESSION_LOGOUT" || receipt.Kind == "CHANGE_PASSWORD" || receipt.Kind == "CHANGE_EMAIL"
	for _, token := range []string{primaryToken, recoveryToken} {
		session, err := s.authentication.Session(ctx, token)
		if err == nil && boundOperation && receipt.ActorID == session.UserID && receipt.SessionHash == hex.EncodeToString(session.ID) {
			return domain.OperationProgress{State: receipt.State}, nil
		}
	}
	if !boundOperation {
		session, err := s.authentication.Session(ctx, primaryToken)
		if err == nil && session.State == "ACTIVE" && session.Expires.After(time.Now()) && receipt.ActorID == session.UserID {
			credential, err := s.receipts.CurrentCredential(ctx, session.UserID)
			if err == nil && credential.State == "ACTIVE" && credential.Epoch == session.CredentialEpoch {
				raw, err := s.receipts.Result(ctx, id)
				if err == nil && len(raw) > 0 {
					return domain.OperationProgress{Result: raw}, nil
				}
			}
		}
	}
	return domain.OperationProgress{}, notFound
}

// operation_result.go：保留原实现，按业务合并文件。

````

## 第 12 步：OperationResult

插入位置：第 11 步代码之后，紧接追加；完成稿约第 183 行。

本步声明：`func OperationResult(id, state, entity string) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: id</code>；<code>&quot;/v1/account/operations/&quot; + id</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;state&quot;: state</code>；<code>state == &quot;PREPARING&quot;</code> |
| <code>entity</code> | <code>string</code> | 在此声明中的实际使用：<code>entity != &quot;&quot;</code>；<code>&quot;entity_id&quot;: entity</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>value</code> |

**跟着本段代码走一遍：**

- 检查 <code>entity != &quot;&quot;</code>。
- 检查 <code>state == &quot;PREPARING&quot;</code>。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7129:7434 -->
````go
func OperationResult(id, state, entity string) map[string]any {
	value := map[string]any{"operation_id": id, "state": state}
	if entity != "" {
		value["result"] = map[string]string{"entity_id": entity}
	}
	if state == "PREPARING" {
		value["poll_url"] = "/v1/account/operations/" + id
	}
	return value
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/operation_receipts_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
