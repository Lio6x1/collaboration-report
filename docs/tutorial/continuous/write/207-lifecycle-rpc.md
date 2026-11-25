# 分步编写 `services/document/internal/logic/document/lifecycle_rpc.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/207-lifecycle-rpc.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将Reserve、Prepare、Apply协议阶段绑定到同一账号生命周期计划。

协议代次独立于identity epoch与worker租约；扩充Retention后仍要允许原输入摘要重试。

写之前先按这条执行路线理解各段的连接：

1. register核原完整proto与阶段
2. Reserve预留
3. Prepare固定留存
4. Apply推进原计划
5. lifecycleRPCReply从真实lifecycle事实构造响应。

本文件需要守住的边界：

- 不能把上一阶段原请求因计划扩充误判冲突
- task登记成功不是清理终态。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/lifecycle_rpc.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/lifecycle_rpc.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/lifecycle_rpc.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/lifecycle_rpc.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:348 -->
````go
package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"google.golang.org/protobuf/proto"
)

````

## 第 2 步：LifecycleBindingAuthority

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type LifecycleBindingAuthority interface {`。

**这一段为什么这样写：** 生命周期RPC的身份入口，防止拿一个任意操作号清理其他账号内容。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>(context.Context, string) (platform.AccountClosureBinding, error)</code> | ctx和原操作号查询Account关闭绑定，返回权威身份/代次，不能从请求自行拼出可信绑定。 |

<!-- write-step: 2 bytes=348:468 -->
````go
type LifecycleBindingAuthority interface {
	Binding(context.Context, string) (platform.AccountClosureBinding, error)
}

````

## 第 3 步：LifecycleReservation

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type LifecycleReservation interface {`。

**这一段为什么这样写：** 管理员退役准备能力，固定之后供留存和确认阶段复核。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Prepare</code> | <code>(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementResult, error)</code> | ctx与账号退役绑定输入，返回固定准备结果；准备阶段保留管理职责约束，尚未执行最终清理。 |

<!-- write-step: 3 bytes=468:608 -->
````go
type LifecycleReservation interface {
	Prepare(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementResult, error)
}

````

## 第 4 步：LifecycleRetention

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type LifecycleRetention interface {`。

**这一段为什么这样写：** 把原计划获取、关系推进和最终确认分开，避免一次跨服务调用被误当成原子事务。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Plan</code> | <code>(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error)</code> | ctx与原退役绑定取得固定留存计划。 |
| <code>ApplyRelations</code> | <code>(context.Context, domain.AccountRetirementBinding) error</code> | 按同一绑定推进文档/知识库关系留存，失败从原步骤恢复。 |
| <code>Complete</code> | <code>(context.Context, domain.AccountRetirementBinding) error</code> | 按原绑定核验关系及清理结果全部完成。 |

<!-- write-step: 4 bytes=608:879 -->
````go
type LifecycleRetention interface {
	Plan(context.Context, domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error)
	ApplyRelations(context.Context, domain.AccountRetirementBinding) error
	Complete(context.Context, domain.AccountRetirementBinding) error
}

````

## 第 5 步：LifecyclePlans

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type LifecyclePlans struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authority</code> | <code>LifecycleBindingAuthority</code> | 使用 <code>LifecycleBindingAuthority</code> 类型；其定义处的职责：生命周期RPC的身份入口，防止拿一个任意操作号清理其他账号内容。 |
| <code>reservation</code> | <code>LifecycleReservation</code> | 使用 <code>LifecycleReservation</code> 类型；其定义处的职责：管理员退役准备能力，固定之后供留存和确认阶段复核。 |
| <code>retention</code> | <code>LifecycleRetention</code> | 使用 <code>LifecycleRetention</code> 类型；其定义处的职责：把原计划获取、关系推进和最终确认分开，避免一次跨服务调用被误当成原子事务。 |

<!-- write-step: 5 bytes=879:1037 -->
````go
type LifecyclePlans struct {
	db          *sql.DB
	authority   LifecycleBindingAuthority
	reservation LifecycleReservation
	retention   LifecycleRetention
}

````

## 第 6 步：NewLifecyclePlans

插入位置：第 5 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func NewLifecyclePlans(db *sql.DB, authority LifecycleBindingAuthority, reservation LifecycleReservation, retention LifecycleRetention) *LifecyclePlans`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>authority</code> | <code>LifecycleBindingAuthority</code> | 使用 <code>LifecycleBindingAuthority</code> 类型；其定义处的职责：生命周期RPC的身份入口，防止拿一个任意操作号清理其他账号内容。<br>在此声明中的实际使用：<code>authority: authority</code> |
| <code>reservation</code> | <code>LifecycleReservation</code> | 使用 <code>LifecycleReservation</code> 类型；其定义处的职责：管理员退役准备能力，固定之后供留存和确认阶段复核。<br>在此声明中的实际使用：<code>reservation: reservation</code> |
| <code>retention</code> | <code>LifecycleRetention</code> | 使用 <code>LifecycleRetention</code> 类型；其定义处的职责：把原计划获取、关系推进和最终确认分开，避免一次跨服务调用被误当成原子事务。<br>在此声明中的实际使用：<code>retention: retention</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*LifecyclePlans</code> | 实际返回表达式：<code>&amp;LifecyclePlans{db: db, authority: authority, reservation: reservation, retention: retention}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;LifecyclePlans{db: db, authority: authority, reservation: reservation, retention: retention}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1037:1296 -->
````go
func NewLifecyclePlans(db *sql.DB, authority LifecycleBindingAuthority, reservation LifecycleReservation, retention LifecycleRetention) *LifecyclePlans {
	return &LifecyclePlans{db: db, authority: authority, reservation: reservation, retention: retention}
}

````

## 第 7 步：lifecycleRPCStep

插入位置：第 6 步代码之后，紧接追加；完成稿约第 44 行。

协议代次由首次Reserve固定，独立于身份epoch、lifecycle业务推进和后台租约代次。
两个推进请求保存原完整proto及输入阶段的完整计划，回复丢失后允许原摘要重试，
不能因为Prepare已扩充Retention就把原输入摘要误判为冲突。

本步声明：`type lifecycleRPCStep struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Request</code> | <code>*pb.LifecycleRequest</code> | 序列化/校验标记：<code>`json:&quot;request&quot;`</code> |
| <code>InputPlan</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;input_plan&quot;`</code> |

<!-- write-step: 7 bytes=1296:1734 -->
````go
// 协议代次由首次Reserve固定，独立于身份epoch、lifecycle业务推进和后台租约代次。
// 两个推进请求保存原完整proto及输入阶段的完整计划，回复丢失后允许原摘要重试，
// 不能因为Prepare已扩充Retention就把原输入摘要误判为冲突。
type lifecycleRPCStep struct {
	Request   *pb.LifecycleRequest `json:"request"`
	InputPlan []byte               `json:"input_plan"`
}

````

## 第 8 步：lifecycleRPCRegistration

插入位置：第 7 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`type lifecycleRPCRegistration struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Reserve</code> | <code>*pb.LifecycleRequest</code> | 序列化/校验标记：<code>`json:&quot;reserve&quot;`</code> |
| <code>Prepare</code> | <code>*lifecycleRPCStep</code> | 序列化/校验标记：<code>`json:&quot;prepare,omitempty&quot;`</code> |
| <code>Apply</code> | <code>*lifecycleRPCStep</code> | 序列化/校验标记：<code>`json:&quot;apply,omitempty&quot;`</code> |

<!-- write-step: 8 bytes=1734:2026 -->
````go
type lifecycleRPCRegistration struct {
	Binding domain.AccountRetirementBinding `json:"binding"`
	Reserve *pb.LifecycleRequest            `json:"reserve"`
	Prepare *lifecycleRPCStep               `json:"prepare,omitempty"`
	Apply   *lifecycleRPCStep               `json:"apply,omitempty"`
}

````

## 第 9 步：lifecycleRPCConflict

插入位置：第 8 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func lifecycleRPCConflict(code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.Violation(domain.StateConflict, code)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, code)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2026:2128 -->
````go
func lifecycleRPCConflict(code string) error { return domain.Violation(domain.StateConflict, code) }

````

## 第 10 步：validateLifecycleRPCRequest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func validateLifecycleRPCRequest(request *pb.LifecycleRequest, phase string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>request == nil</code>；<code>request.OperationId</code>；<code>request.UserId</code>；<code>request.Generation</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>phase</code> | <code>string</code> | 在此声明中的实际使用：<code>phase == &quot;RESERVE&quot;</code>；<code>phase != &quot;RESERVE&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_REQUEST_INVALID&quot;)</code>；<code>domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_ORGANIZATIONS_INVALID&quot;)</code>；<code>domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_DIGEST_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>request == nil &#124;&#124; !domain.ValidOperationID(request.OperationId) &#124;&#124; request.UserId == 0 &#124;&#124; request.Generation == 0</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>organization &lt;= previous</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_ORGANIZATIONS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>(phase == &quot;RESERVE&quot; &amp;&amp; len(request.FixedPlanDigest) != 0) &#124;&#124; (phase != &quot;RESERVE&quot; &amp;&amp; len(request.FixedPlanDigest) != sha256.Size)</code>。条件成立时执行 <code>return domain.Violation(domain.InvalidRequest, &quot;LIFECYCLE_RPC_DIGEST_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2128:2890 -->
````go
func validateLifecycleRPCRequest(request *pb.LifecycleRequest, phase string) error {
	if request == nil || !domain.ValidOperationID(request.OperationId) || request.UserId == 0 || request.Generation == 0 {
		return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_REQUEST_INVALID")
	}
	var previous uint64
	for _, organization := range request.ClosingOrganizationIds {
		if organization <= previous {
			return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_ORGANIZATIONS_INVALID")
		}
		previous = organization
	}
	if (phase == "RESERVE" && len(request.FixedPlanDigest) != 0) || (phase != "RESERVE" && len(request.FixedPlanDigest) != sha256.Size) {
		return domain.Violation(domain.InvalidRequest, "LIFECYCLE_RPC_DIGEST_INVALID")
	}
	return nil
}

````

## 第 11 步：bindingMatchesLifecycleRequest

插入位置：第 10 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func bindingMatchesLifecycleRequest(binding domain.AccountRetirementBinding, request *pb.LifecycleRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>binding.Operation</code>；<code>binding.User</code>；<code>binding.ClosingOrganizations</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>request.OperationId</code>；<code>request.UserId</code>；<code>request.ClosingOrganizationIds</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>binding.Valid() &amp;&amp; binding.Operation == request.OperationId &amp;&amp; binding.User == request.UserId &amp;&amp; 		slices.Equal(binding.ClosingOrganizations, request.ClosingOrganizationIds)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return binding.Valid() &amp;&amp; binding.Operation == request.OperationId &amp;&amp; binding.User == request.UserId &amp;&amp; 		slices.Equal(binding.ClosingOrganizations, request.ClosingOrganizationIds)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2890:3189 -->
````go
func bindingMatchesLifecycleRequest(binding domain.AccountRetirementBinding, request *pb.LifecycleRequest) bool {
	return binding.Valid() && binding.Operation == request.OperationId && binding.User == request.UserId &&
		slices.Equal(binding.ClosingOrganizations, request.ClosingOrganizationIds)
}

````

## 第 12 步：decodeLifecycleRPCRegistration

插入位置：第 11 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func decodeLifecycleRPCRegistration(raw []byte, operation, state string) (lifecycleRPCRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;record)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>operation != record.Reserve.OperationId</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>state != &quot;SUCCEEDED&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>lifecycleRPCRegistration</code> | 实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;LIFECYCLE_RPC_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;LIFECYCLE_RPC_INPUT_PLAN_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>state != &quot;SUCCEEDED&quot; &#124;&#124; json.Unmarshal(raw, &amp;record) != nil &#124;&#124; record.Reserve == nil &#124;&#124; 		validateLifecycleRPCRequest(record.Reserve, &quot;RESERVE&quot;) != nil &#124;&#124; operation != record.Reser …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return record, errors.New(&quot;LIFECYCLE_RPC_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>step != nil &amp;&amp; (step.Request == nil &#124;&#124; validateLifecycleRPCRequest(step.Request, &quot;STEP&quot;) != nil &#124;&#124; 			!bindingMatchesLifecycleRequest(record.Binding, step.Request) &#124;&#124; step.Request.Generation != record.Reserve.Generation)</code>。条件成立时执行 <code>return record, errors.New(&quot;LIFECYCLE_RPC_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>step != nil</code>。
- 检查 <code>err != nil &#124;&#124; !plan.Binding.Same(record.Binding) &#124;&#124; !bytes.Equal(digest&#91;:&#93;, step.Request.FixedPlanDigest)</code>。条件成立时执行 <code>return record, errors.New(&quot;LIFECYCLE_RPC_INPUT_PLAN_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return record, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3189:4381 -->
````go
func decodeLifecycleRPCRegistration(raw []byte, operation, state string) (lifecycleRPCRegistration, error) {
	var record lifecycleRPCRegistration
	if state != "SUCCEEDED" || json.Unmarshal(raw, &record) != nil || record.Reserve == nil ||
		validateLifecycleRPCRequest(record.Reserve, "RESERVE") != nil || operation != record.Reserve.OperationId ||
		!bindingMatchesLifecycleRequest(record.Binding, record.Reserve) {
		return record, errors.New("LIFECYCLE_RPC_BINDING_UNKNOWN")
	}
	for _, step := range []*lifecycleRPCStep{record.Prepare, record.Apply} {
		if step != nil && (step.Request == nil || validateLifecycleRPCRequest(step.Request, "STEP") != nil ||
			!bindingMatchesLifecycleRequest(record.Binding, step.Request) || step.Request.Generation != record.Reserve.Generation) {
			return record, errors.New("LIFECYCLE_RPC_BINDING_UNKNOWN")
		}
		if step != nil {
			digest := sha256.Sum256(step.InputPlan)
			plan, err := domain.DecodeAccountRetirementPlan(step.InputPlan)
			if err != nil || !plan.Binding.Same(record.Binding) || !bytes.Equal(digest[:], step.Request.FixedPlanDigest) {
				return record, errors.New("LIFECYCLE_RPC_INPUT_PLAN_UNKNOWN")
			}
		}
	}
	return record, nil
}

````

## 第 13 步：readLifecycleRPCOriginal

插入位置：第 12 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func readLifecycleRPCOriginal(ctx context.Context, q repository.Queryer, record lifecycleRPCRegistration) (repository.AccountRetirementRecord, domain.AccountRetirementPlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, q, record.Binding.Operation)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, q, record.Binding.Operation)</code> |
| <code>record</code> | <code>lifecycleRPCRegistration</code> | 在此声明中的实际使用：<code>record.Binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.AccountRetirementRecord</code> | 使用 <code>repository.AccountRetirementRecord</code> 类型；其定义处的职责：保存账号注销准备、冻结、拒绝及显式取消的持久事实。<br>实际返回表达式：<code>original</code> |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>实际返回表达式：<code>domain.AccountRetirementPlan{}</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_PLAN_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;LIFECYCLE_RPC_RETENTION_UNKNOWN&quot;)</code>；<code>errors.New(&quot;LIFECYCLE_RPC_PHASE_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return original, domain.AccountRetirementPlan{}, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; original.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; original.Actor != record.Binding.User &#124;&#124; 		original.Epoch != record.Binding.IdentityEpoch &#124;&#124; !plan.Binding.Same(record.Binding)</code>。条件成立时执行 <code>return original, plan, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_PLAN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil</code>。条件成立时执行 <code>return original, plan, errors.New(&quot;LIFECYCLE_RPC_RETENTION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return original, plan, errors.New(&quot;LIFECYCLE_RPC_PHASE_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return original, plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4381:5373 -->
````go
func readLifecycleRPCOriginal(ctx context.Context, q repository.Queryer, record lifecycleRPCRegistration) (repository.AccountRetirementRecord, domain.AccountRetirementPlan, error) {
	original, err := repository.AccountRetirementSnapshot(ctx, q, record.Binding.Operation)
	if err != nil {
		return original, domain.AccountRetirementPlan{}, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil || original.Kind != "ACCOUNT_CLOSE" || original.Actor != record.Binding.User ||
		original.Epoch != record.Binding.IdentityEpoch || !plan.Binding.Same(record.Binding) {
		return original, plan, lifecycleRPCConflict("LIFECYCLE_RPC_PLAN_BINDING_CHANGED")
	}
	switch original.State {
	case "FROZEN", "REJECTED":
	case "PLANNED", "CLEANING", "SUCCEEDED":
		if plan.Retention == nil {
			return original, plan, errors.New("LIFECYCLE_RPC_RETENTION_UNKNOWN")
		}
	default:
		return original, plan, errors.New("LIFECYCLE_RPC_PHASE_UNKNOWN")
	}
	return original, plan, nil
}

````

## 第 14 步：lifecycleRPCCompatibleInput

插入位置：第 13 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func lifecycleRPCCompatibleInput(step *lifecycleRPCStep, original repository.AccountRetirementRecord) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>step</code> | <code>*lifecycleRPCStep</code> | 在此声明中的实际使用：<code>step.InputPlan</code> |
| <code>original</code> | <code>repository.AccountRetirementRecord</code> | 使用 <code>repository.AccountRetirementRecord</code> 类型；其定义处的职责：保存账号注销准备、冻结、拒绝及显式取消的持久事实。<br>在此声明中的实际使用：<code>original.Plan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_FIXED_PLAN_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !input.Binding.Same(current.Binding) &#124;&#124; !slices.Equal(input.ReservedKnowledgeBases, current.ReservedKnowledgeBases) &#124;&#124; 		(input.Retention != nil &amp;&amp; !bytes.Equal(step.InputPlan, original.Plan))</code>。条件成立时执行 <code>return lifecycleRPCConflict(&quot;LIFECYCLE_RPC_FIXED_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5373:5946 -->
````go
func lifecycleRPCCompatibleInput(step *lifecycleRPCStep, original repository.AccountRetirementRecord) error {
	input, err := domain.DecodeAccountRetirementPlan(step.InputPlan)
	if err != nil {
		return err
	}
	current, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil || !input.Binding.Same(current.Binding) || !slices.Equal(input.ReservedKnowledgeBases, current.ReservedKnowledgeBases) ||
		(input.Retention != nil && !bytes.Equal(step.InputPlan, original.Plan)) {
		return lifecycleRPCConflict("LIFECYCLE_RPC_FIXED_PLAN_CHANGED")
	}
	return nil
}

````

## 第 15 步：LifecyclePlans.register

插入位置：第 14 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`func (s *LifecyclePlans) register(ctx context.Context, request *pb.LifecycleRequest, phase string) (lifecycleRPCRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReadLifecycleRPCBinding(ctx, s.db, request.OperationId, false)</code>；<code>s.authority.Binding(ctx, request.OperationId)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.InsertLifecycleRPCBinding(ctx, tx, request.OperationId, raw)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>validateLifecycleRPCRequest(request, phase)</code>；<code>request.OperationId</code>；<code>bindingMatchesLifecycleRequest(empty.Binding, request)</code>；<code>proto.Clone(request)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>phase</code> | <code>string</code> | 在此声明中的实际使用：<code>validateLifecycleRPCRequest(request, phase)</code>；<code>phase != &quot;RESERVE&quot;</code>；<code>phase == &quot;RESERVE&quot;</code>；<code>phase == &quot;APPLY&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>lifecycleRPCRegistration</code> | 实际返回表达式：<code>empty</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;LIFECYCLE_RPC_DEPENDENCIES_REQUIRED&quot;)</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_RESERVATION_REQUIRED&quot;)</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>s == nil &#124;&#124; s.db == nil &#124;&#124; s.authority == nil &#124;&#124; s.reservation == nil &#124;&#124; s.retention == nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;LIFECYCLE_RPC_DEPENDENCIES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>phase != &quot;RESERVE&quot;</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_RESERVATION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!bindingMatchesLifecycleRequest(empty.Binding, request)</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>phase == &quot;RESERVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=5946:7937 -->
````go
func (s *LifecyclePlans) register(ctx context.Context, request *pb.LifecycleRequest, phase string) (lifecycleRPCRegistration, error) {
	var empty lifecycleRPCRegistration
	if err := validateLifecycleRPCRequest(request, phase); err != nil {
		return empty, err
	}
	if s == nil || s.db == nil || s.authority == nil || s.reservation == nil || s.retention == nil {
		return empty, errors.New("LIFECYCLE_RPC_DEPENDENCIES_REQUIRED")
	}
	raw, operation, state, err := repository.ReadLifecycleRPCBinding(ctx, s.db, request.OperationId, false)
	if errors.Is(err, sql.ErrNoRows) {
		if phase != "RESERVE" {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_RESERVATION_REQUIRED")
		}
		// Account原持久绑定在SQL锁外读取；浏览器或RPC输入不能自行挑选identity_epoch。
		binding, err := s.authority.Binding(ctx, request.OperationId)
		if err != nil {
			return empty, err
		}
		empty.Binding = domain.AccountRetirementBinding(binding)
		if !bindingMatchesLifecycleRequest(empty.Binding, request) {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED")
		}
		empty.Reserve = proto.Clone(request).(*pb.LifecycleRequest)
		raw, err = json.Marshal(empty)
		if err != nil {
			return empty, err
		}
	} else if err != nil {
		return empty, err
	} else if _, err = decodeLifecycleRPCRegistration(raw, operation, state); err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	// 只持专用登记行锁，不持来源/lifecycle业务锁，更不在事务中跨服务调用。
	// 唯一键把两个首次Reserve串行化，后来的请求只能读原payload，不能覆盖原代次。
	if phase == "RESERVE" {
		if err = repository.InsertLifecycleRPCBinding(ctx, tx, request.OperationId, raw); err != nil {
			return empty, err
		}
	}
	raw, operation, state, err = repository.ReadLifecycleRPCBinding(ctx, tx, request.OperationId, true)
````

## 第 16 步：LifecyclePlans.register（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 186 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!bindingMatchesLifecycleRequest(record.Binding, request) &#124;&#124; request.Generation != record.Reserve.Generation</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_REQUEST_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>phase == &quot;RESERVE&quot;</code>。
- 检查 <code>!proto.Equal(request, record.Reserve)</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_REQUEST_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>phase == &quot;APPLY&quot;</code>。
- 检查 <code>plan.Retention == nil &amp;&amp; original.State != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>step != nil</code>。
- 检查 <code>!proto.Equal(step.Request, request)</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_REQUEST_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest&#91;:&#93;, request.FixedPlanDigest)</code>。条件成立时执行 <code>return empty, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_FIXED_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>phase == &quot;APPLY&quot;</code>。

<!-- write-step: 16 bytes=7937:9461 -->
````go
	if err != nil {
		return empty, err
	}
	record, err := decodeLifecycleRPCRegistration(raw, operation, state)
	if err != nil {
		return empty, err
	}
	if !bindingMatchesLifecycleRequest(record.Binding, request) || request.Generation != record.Reserve.Generation {
		return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
	}
	if phase == "RESERVE" {
		if !proto.Equal(request, record.Reserve) {
			return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
		}
	} else {
		original, plan, err := readLifecycleRPCOriginal(ctx, tx, record)
		if err != nil {
			return empty, err
		}
		step := record.Prepare
		if phase == "APPLY" {
			step = record.Apply
			if plan.Retention == nil && original.State != "REJECTED" {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED")
			}
		}
		if step != nil {
			if !proto.Equal(step.Request, request) {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_REQUEST_BINDING_CHANGED")
			}
			if err = lifecycleRPCCompatibleInput(step, original); err != nil {
				return empty, err
			}
		} else {
			digest := sha256.Sum256(original.Plan)
			if !bytes.Equal(digest[:], request.FixedPlanDigest) {
				return empty, lifecycleRPCConflict("LIFECYCLE_RPC_FIXED_PLAN_CHANGED")
			}
			step = &lifecycleRPCStep{Request: proto.Clone(request).(*pb.LifecycleRequest), InputPlan: slices.Clone(original.Plan)}
			if phase == "APPLY" {
				record.Apply = step
			} else {
				record.Prepare = step
			}
			raw, err = json.Marshal(record)
````

## 第 17 步：LifecyclePlans.register（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 231 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;LIFECYCLE_RPC_REGISTRATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return record, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9461:9766 -->
````go
			if err != nil {
				return empty, err
			}
			if err = repository.UpdateLifecycleRPCBinding(ctx, tx, request.OperationId, raw); err != nil {
				return empty, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("LIFECYCLE_RPC_REGISTRATION_UNKNOWN")
	}
	return record, nil
}

````

## 第 18 步：lifecycleRPCReply

插入位置：第 17 步代码之后，紧接追加；完成稿约第 245 行。

本步声明：`func lifecycleRPCReply(record lifecycleRPCRegistration, original repository.AccountRetirementRecord) *pb.LifecyclePlan`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>record</code> | <code>lifecycleRPCRegistration</code> | 在此声明中的实际使用：<code>record.Binding</code>；<code>record.Reserve</code> |
| <code>original</code> | <code>repository.AccountRetirementRecord</code> | 使用 <code>repository.AccountRetirementRecord</code> 类型；其定义处的职责：保存账号注销准备、冻结、拒绝及显式取消的持久事实。<br>在此声明中的实际使用：<code>original.Plan</code>；<code>original.State</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>&amp;pb.LifecyclePlan{OperationId: record.Binding.Operation, Generation: record.Reserve.Generation, 		PlanJsonUtf8: slices.Clone(original.Plan), PlanDigest: digest&#91;:&#93;, Phase: original.State}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.LifecyclePlan{OperationId: record.Binding.Operation, Generation: record.Reserve.Generation, 		PlanJsonUtf8: slices.Clone(original.Plan), PlanDigest: digest&#91;:&#93;, Phase: original.State}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9766:10125 -->
````go
func lifecycleRPCReply(record lifecycleRPCRegistration, original repository.AccountRetirementRecord) *pb.LifecyclePlan {
	digest := sha256.Sum256(original.Plan)
	return &pb.LifecyclePlan{OperationId: record.Binding.Operation, Generation: record.Reserve.Generation,
		PlanJsonUtf8: slices.Clone(original.Plan), PlanDigest: digest[:], Phase: original.State}
}

````

## 第 19 步：LifecyclePlans.Reserve

插入位置：第 18 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func (s *LifecyclePlans) Reserve(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.register(ctx, request, &quot;RESERVE&quot;)</code>；<code>readLifecycleRPCOriginal(ctx, s.db, record)</code>；<code>s.authority.Binding(ctx, request.OperationId)</code>；<code>s.reservation.Prepare(ctx, record.Binding)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.register(ctx, request, &quot;RESERVE&quot;)</code>；<code>request.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>nil</code>；<code>lifecycleRPCReply(record, original)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>bindingErr</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED&quot;)</code>；<code>preparationErr</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>bindingErr != nil</code>。条件成立时执行 <code>return nil, bindingErr</code>。当前函数在这里结束。
- 检查 <code>!record.Binding.Same(domain.AccountRetirementBinding(binding))</code>。条件成立时执行 <code>return nil, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>preparationErr != nil</code>。条件成立时执行 <code>return nil, preparationErr</code>。当前函数在这里结束。
- 检查 <code>result.State == &quot;HANDOFF_REQUIRED&quot;</code>。条件成立时执行 <code>return nil, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_HANDOFF_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return lifecycleRPCReply(record, original), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=10125:11301 -->
````go
func (s *LifecyclePlans) Reserve(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "RESERVE")
	if err != nil {
		return nil, err
	}
	original, _, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if errors.Is(err, sql.ErrNoRows) {
		// 登记回复未知或进程退出后可能尚未实际预留；重查原Account决定，不能把
		// 已拒绝但取消消息尚未到达的旧登记提升为新的封门资格。
		binding, bindingErr := s.authority.Binding(ctx, request.OperationId)
		if bindingErr != nil {
			return nil, bindingErr
		}
		if !record.Binding.Same(domain.AccountRetirementBinding(binding)) {
			return nil, lifecycleRPCConflict("LIFECYCLE_RPC_ACCOUNT_BINDING_CHANGED")
		}
		result, preparationErr := s.reservation.Prepare(ctx, record.Binding)
		if preparationErr != nil {
			return nil, preparationErr
		}
		if result.State == "HANDOFF_REQUIRED" {
			return nil, lifecycleRPCConflict("LIFECYCLE_RPC_HANDOFF_REQUIRED")
		}
		original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
	}
	if err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}

````

## 第 20 步：LifecyclePlans.Prepare

插入位置：第 19 步代码之后，紧接追加；完成稿约第 282 行。

本步声明：`func (s *LifecyclePlans) Prepare(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.register(ctx, request, &quot;PREPARE&quot;)</code>；<code>readLifecycleRPCOriginal(ctx, s.db, record)</code>；<code>s.retention.Plan(ctx, record.Binding)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.register(ctx, request, &quot;PREPARE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>nil</code>；<code>lifecycleRPCReply(record, original)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil &amp;&amp; original.State != &quot;REJECTED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return lifecycleRPCReply(record, original), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11301:12128 -->
````go
func (s *LifecyclePlans) Prepare(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "PREPARE")
	if err != nil {
		return nil, err
	}
	original, plan, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	// 先读原固定计划/终态，Account后来去标识或身份变代不能破坏已登记同号重放。
	if plan.Retention == nil && original.State != "REJECTED" {
		if _, err = s.retention.Plan(ctx, record.Binding); err != nil {
			return nil, err
		}
		original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
		if err != nil {
			return nil, err
		}
	}
	if err = lifecycleRPCCompatibleInput(record.Prepare, original); err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}

````

## 第 21 步：lifecycleRPCWaiting

插入位置：第 20 步代码之后，紧接追加；完成稿约第 307 行。

本步声明：`func lifecycleRPCWaiting(err error) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>errors.As(err, &amp;violation) &amp;&amp; violation.Kind == domain.StateConflict &amp;&amp; 		(violation.Code == &quot;ACCOUNT_RETENTION_DRAIN_PENDING&quot; &#124;&#124; violation.Code == &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.As(err, &amp;violation) &amp;&amp; violation.Kind == domain.StateConflict &amp;&amp; 		(violation.Code == &quot;ACCOUNT_RETENTION_DRAIN_PENDING&quot; &#124;&#124; violation.Code == &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=12128:12402 -->
````go
func lifecycleRPCWaiting(err error) bool {
	var violation domain.RuleViolation
	return errors.As(err, &violation) && violation.Kind == domain.StateConflict &&
		(violation.Code == "ACCOUNT_RETENTION_DRAIN_PENDING" || violation.Code == "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
}

````

## 第 22 步：LifecyclePlans.Apply

插入位置：第 21 步代码之后，紧接追加；完成稿约第 313 行。

本步声明：`func (s *LifecyclePlans) Apply(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.register(ctx, request, &quot;APPLY&quot;)</code>；<code>readLifecycleRPCOriginal(ctx, s.db, record)</code>；<code>s.retention.ApplyRelations(ctx, record.Binding)</code>；<code>s.retention.Complete(ctx, record.Binding)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.register(ctx, request, &quot;APPLY&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>nil</code>；<code>lifecycleRPCReply(record, original)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>lifecycleRPCConflict(&quot;LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return nil, lifecycleRPCConflict(&quot;LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &amp;&amp; !lifecycleRPCWaiting(err)</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return lifecycleRPCReply(record, original), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=12402:13608 -->
````go
func (s *LifecyclePlans) Apply(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	record, err := s.register(ctx, request, "APPLY")
	if err != nil {
		return nil, err
	}
	original, _, err := readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	if err = lifecycleRPCCompatibleInput(record.Apply, original); err != nil {
		return nil, err
	}
	// 一次调用只推进一个既有阶段；不另起同步清理循环。CLEANING须待Account原
	// 持久协调器逐文档拿到清理回执，Complete也绝不替Account做最后去标识事务。
	switch original.State {
	case "PLANNED":
		err = s.retention.ApplyRelations(ctx, record.Binding)
	case "CLEANING":
		err = s.retention.Complete(ctx, record.Binding)
	case "SUCCEEDED", "REJECTED":
	default:
		return nil, lifecycleRPCConflict("LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED")
	}
	if err != nil && !lifecycleRPCWaiting(err) {
		return nil, err
	}
	original, _, err = readLifecycleRPCOriginal(ctx, s.db, record)
	if err != nil {
		return nil, err
	}
	if err = lifecycleRPCCompatibleInput(record.Apply, original); err != nil {
		return nil, err
	}
	return lifecycleRPCReply(record, original), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/lifecycle_rpc.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
