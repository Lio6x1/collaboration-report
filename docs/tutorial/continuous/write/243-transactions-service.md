# 分步编写 `services/document/internal/logic/document/transactions_service.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/243-transactions-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。

原号重试只重放固定业务结果，敏感内容仍依据当前权限生成；防止成功回执变成永久读权限。

写之前先按这条执行路线理解各段的连接：

1. Mutation锁原号并比较actor/kind/fingerprint
2. 无回执才执行run
3. present在同一事务复核资格
4. bind在COMMIT前连接真实输出
5. TryReplay按入口策略处理原号。

本文件需要守住的边界：

- 不存在回执返回未处理后仍要完整鉴权
- 依赖未知不能重复业务
- 输出绑定失败不能照样提交。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/transactions_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/transactions_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/transactions_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/transactions_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:444 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
)

// mutation_transaction.go：保留原实现，按业务合并文件。

````

## 第 2 步：Transactions.Mutation

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

通用receipt只存固定结果；敏感投影在原事务内重新鉴权后构造，并登记它实际依赖的来源。

本步声明：`func (s *Transactions) Mutation(ctx context.Context, actor Actor, command OperationCommand, handler string, run MutationBody, present MutationPresentation, bind BindOutput) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Revalidate(ctx, actor.Proof)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, command.ID)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code>；<code>actor.Sources</code>；<code>actor.ID</code>；<code>insertMutationReceipt(ctx, tx, actor, command, fingerprint, command.InitialResult)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>command</code> | <code>OperationCommand</code> | 在此声明中的实际使用：<code>command.ID</code>；<code>command.Path</code>；<code>command.Input</code>；<code>command.Kind</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.outputs.Register(ctx, tx, responseSources, 0, &quot;&quot;, handler, edge)</code> |
| <code>run</code> | <code>MutationBody</code> | 在此声明中的实际使用：<code>run(ctx, tx)</code> |
| <code>present</code> | <code>MutationPresentation</code> | 在此声明中的实际使用：<code>present != nil</code>；<code>present(ctx, tx, value)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, responseSources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>response</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(command.ID)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>sources == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>receipt.ActorID != actor.ID &#124;&#124; receipt.Kind != command.Kind &#124;&#124; !bytes.Equal(receipt.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(receipt.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;OPERATION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>command.InitialResult != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=444:2181 -->
````go
// 通用receipt只存固定结果；敏感投影在原事务内重新鉴权后构造，并登记它实际依赖的来源。
func (s *Transactions) Mutation(ctx context.Context, actor Actor, command OperationCommand, handler string, run MutationBody, present MutationPresentation, bind BindOutput) (any, error) {
	if !domain.ValidOperationID(command.ID) {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_ID_INVALID")
	}
	if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil {
		return nil, err
	}
	fingerprint, err := domain.OperationFingerprint(command.Path, command.Input)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := repository.LockOperationGate(ctx, tx, command.ID); err != nil {
		return nil, err
	}
	sources := actor.Sources
	if sources == nil {
		sources = actor.Proof.Sources
	}
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	receipt, err := repository.LockedOperation(ctx, tx, command.ID)
	var value map[string]any
	if err == nil {
		if receipt.ActorID != actor.ID || receipt.Kind != command.Kind || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		if json.Unmarshal(receipt.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_PENDING")
		}
	} else if err == sql.ErrNoRows {
		if command.InitialResult != nil {
			if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, command.InitialResult); err != nil {
				return nil, err
			}
		}
````

## 第 3 步：Transactions.Mutation（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 63 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>command.InitialResult != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>present != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return response, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2181:3149 -->
````go
		value, err = run(ctx, tx)
		if err != nil {
			return nil, err
		}
		if command.InitialResult != nil {
			result, err := platform.EncodeOperationReceipt(command.ID, value)
			if err != nil {
				return nil, err
			}
			if err = repository.UpdateOperationResult(ctx, tx, command.ID, result.State, result.JSON); err != nil {
				return nil, err
			}
		} else if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, value); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	var response any = value
	responseSources := actor.Proof.Sources
	if present != nil {
		response, responseSources, err = present(ctx, tx, value)
		if err != nil {
			return nil, err
		}
	}
	registration, err := s.outputs.Register(ctx, tx, responseSources, 0, "", handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, responseSources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return response, nil
}

````

## 第 4 步：insertMutationReceipt

插入位置：第 3 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func insertMutationReceipt(ctx context.Context, tx *sql.Tx, actor Actor, command OperationCommand, fingerprint [32]byte, value map[string]any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertOperation(ctx, tx, record)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertOperation(ctx, tx, record)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>command</code> | <code>OperationCommand</code> | 在此声明中的实际使用：<code>command.ID</code>；<code>command.Path</code>；<code>command.Kind</code> |
| <code>fingerprint</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>Fingerprint: fingerprint&#91;:&#93;</code> |
| <code>value</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>platform.EncodeOperationReceipt(command.ID, value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>repository.InsertOperation(ctx, tx, record)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return repository.InsertOperation(ctx, tx, record)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3149:3788 -->
````go
func insertMutationReceipt(ctx context.Context, tx *sql.Tx, actor Actor, command OperationCommand, fingerprint [32]byte, value map[string]any) error {
	result, err := platform.EncodeOperationReceipt(command.ID, value)
	if err != nil {
		return err
	}
	target, err := json.Marshal(map[string]string{"path": command.Path})
	if err != nil {
		return err
	}
	record := domain.OperationRecord{ID: command.ID, ActorID: actor.ID, Kind: command.Kind, TargetJSON: target, Fingerprint: fingerprint[:], State: result.State, IdentityAdmissionRef: actor.Proof.AdmissionId, ResultJSON: result.JSON}
	return repository.InsertOperation(ctx, tx, record)
}
````

## 第 5 步：OperationResult

插入位置：第 4 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func OperationResult(id, state string, entity uint64) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;operation_id&quot;: id</code>；<code>&quot;/v1/authorization-changes/&quot; + id</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;state&quot;: state</code>；<code>state == &quot;PREPARING&quot;</code> |
| <code>entity</code> | <code>uint64</code> | 在此声明中的实际使用：<code>entity != 0</code>；<code>domain.TextID(entity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>value</code> |

**跟着本段代码走一遍：**

- 检查 <code>entity != 0</code>。
- 检查 <code>state == &quot;PREPARING&quot;</code>。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3788:4187 -->
````go
func OperationResult(id, state string, entity uint64) map[string]any {
	value := map[string]any{"operation_id": id, "state": state}
	if entity != 0 {
		value["result"] = map[string]string{"entity_id": domain.TextID(entity)}
	}
	if state == "PREPARING" {
		value["poll_url"] = "/v1/authorization-changes/" + id
	}
	return value
}

// operation_replay.go：保留原实现，按业务合并文件。

````

## 第 6 步：ReplayPolicy

插入位置：第 5 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`type ReplayPolicy uint8`。

<!-- write-step: 6 bytes=4187:4212 -->
````go
type ReplayPolicy uint8

````

## 第 7 步：ValidateOperationFirst、LookupReceiptFirst

插入位置：第 6 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`const (`。

<!-- write-step: 7 bytes=4212:4287 -->
````go
const (
	ValidateOperationFirst ReplayPolicy = iota
	LookupReceiptFirst
)

````

## 第 8 步：Transactions.TryReplay

插入位置：第 7 步代码之后，紧接追加；完成稿约第 133 行。

TryReplay保留两类旧入口的验证顺序；存在原号时只用当前账号证明读取固定回执。
清除业务来源不会绕过新动作资格：不存在原号时返回未处理，调用者仍需执行完整新用例。

本步声明：`func (s *Transactions) TryReplay(ctx context.Context, actor Actor, command OperationCommand, handler string, policy ReplayPolicy, present MutationPresentation, bind BindOutput) (bool, any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.operations.Exists(ctx, command.ID)</code>；<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Sources</code>；<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |
| <code>command</code> | <code>OperationCommand</code> | 在此声明中的实际使用：<code>command.ID</code>；<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |
| <code>policy</code> | <code>ReplayPolicy</code> | 在此声明中的实际使用：<code>policy == ValidateOperationFirst</code> |
| <code>present</code> | <code>MutationPresentation</code> | 在此声明中的实际使用：<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 		return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 	}, present, bind)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>policy == ValidateOperationFirst &amp;&amp; !domain.ValidOperationID(command.ID)</code>。条件成立时执行 <code>return true, nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_ID_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return true, nil, err</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return false, nil, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true, result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4287:5373 -->
````go
// TryReplay保留两类旧入口的验证顺序；存在原号时只用当前账号证明读取固定回执。
// 清除业务来源不会绕过新动作资格：不存在原号时返回未处理，调用者仍需执行完整新用例。
func (s *Transactions) TryReplay(ctx context.Context, actor Actor, command OperationCommand, handler string, policy ReplayPolicy, present MutationPresentation, bind BindOutput) (bool, any, error) {
	if policy == ValidateOperationFirst && !domain.ValidOperationID(command.ID) {
		return true, nil, domain.Violation(domain.InvalidRequest, "OPERATION_ID_INVALID")
	}
	exists, err := s.operations.Exists(ctx, command.ID)
	if err != nil {
		return true, nil, err
	}
	if !exists {
		return false, nil, nil
	}
	actor.Sources = nil
	result, err := s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map[string]any, error) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}, present, bind)
	return true, result, err
}

// transactions.go：保留原实现，按业务合并文件。

````

## 第 9 步：ReadBody

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`type ReadBody func(context.Context, *sql.Tx, string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |
| <code>(匿名/未命名)</code> | <code>string</code> | 在原事务创建/核验的文档准入引用，正文输出与该准入绑定。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 此次受保护读取的业务值；外层还须登记输出并确认事务才发给用户。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 9 bytes=5373:5439 -->
````go
type ReadBody func(context.Context, *sql.Tx, string) (any, error)
````

## 第 10 步：MutationBody

插入位置：第 9 步代码之后，紧接追加；完成稿约第 156 行。

本步声明：`type MutationBody func(context.Context, *sql.Tx) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 要写入原操作回执的业务结果map；错误时外层不提交成功回执。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 10 bytes=5439:5512 -->
````go
type MutationBody func(context.Context, *sql.Tx) (map[string]any, error)
````

## 第 11 步：MutationPresentation

插入位置：第 10 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`type MutationPresentation func(context.Context, *sql.Tx, map[string]any) (any, []*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 原操作已保存的结果map，展示层从中恢复用户可读结果。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 面向本次响应的展示对象。 |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 展示此结果实际依赖的权限来源列表，随后统一登记输出。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 11 bytes=5512:5617 -->
````go
type MutationPresentation func(context.Context, *sql.Tx, map[string]any) (any, []*pb.SourceEpoch, error)
````

## 第 12 步：OutputAborter

插入位置：第 11 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`type OutputAborter interface{ Abort() }`。

**这一段为什么这样写：** 业务失败时撤销待发送输出的最小接口。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Abort</code> | <code>()</code> | 终止尚未放行的输出登记；事务/核验失败时调用，不把未确认内容先发出去。 |

<!-- write-step: 12 bytes=5617:5658 -->
````go
type OutputAborter interface{ Abort() }

````

## 第 13 步：BindOutput

插入位置：第 12 步代码之后，紧接追加；完成稿约第 160 行。

BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。

本步声明：`type BindOutput func(*ResponseRegistration, []*pb.SourceEpoch) OutputAborter`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 使用 <code>ResponseRegistration</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。 |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 已经核验的输出来源及代次，绑定到当前响应的停止追踪。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OutputAborter</code> | 使用 <code>OutputAborter</code> 类型；其定义处的职责：业务失败时撤销待发送输出的最小接口。 |

<!-- write-step: 13 bytes=5658:5852 -->
````go
// BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。
type BindOutput func(*ResponseRegistration, []*pb.SourceEpoch) OutputAborter

````

## 第 14 步：OperationCommand

插入位置：第 13 步代码之后，紧接追加；完成稿约第 163 行。

本步声明：`type OperationCommand struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 客户端本次业务意图的原操作号。 |
| <code>Kind</code> | <code>string</code> | 明确业务操作种类，和ID及指纹共同绑定回执。 |
| <code>Path</code> | <code>string</code> | 本次操作请求路径，参与请求指纹/目标绑定。 |
| <code>Input</code> | <code>any</code> | 经过协议解码的请求值，用于规范指纹和业务处理；不能跳过后续权限检查。 |
| <code>InitialResult</code> | <code>map&#91;string&#93;any</code> | 有外键依赖原号的流程先在同一事务登记准备态，再执行业务写入；失败仍整体回滚。 |

<!-- write-step: 14 bytes=5852:6077 -->
````go
type OperationCommand struct {
	ID, Kind, Path string
	Input          any
	// 有外键依赖原号的流程先在同一事务登记准备态，再执行业务写入；失败仍整体回滚。
	InitialResult map[string]any
}
````

## 第 15 步：Transactions

插入位置：第 14 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`type Transactions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>operations</code> | <code>*repository.Operations</code> | 使用 <code>repository.Operations</code> 类型；其定义处的职责：实现幂等操作门闩、固定回执、准入来源索引及事务内搜索outbox。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |

<!-- write-step: 15 bytes=6077:6205 -->
````go
type Transactions struct {
	db         *sql.DB
	operations *repository.Operations
	identity   *Identity
	outputs    *Outputs
}

````

## 第 16 步：NewTransactions

插入位置：第 15 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func NewTransactions(db *sql.DB, operations *repository.Operations, identity *Identity, outputs *Outputs) *Transactions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>operations</code> | <code>*repository.Operations</code> | 使用 <code>repository.Operations</code> 类型；其定义处的职责：实现幂等操作门闩、固定回执、准入来源索引及事务内搜索outbox。<br>在此声明中的实际使用：<code>operations: operations</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Transactions</code> | 实际返回表达式：<code>&amp;Transactions{db: db, operations: operations, identity: identity, outputs: outputs}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Transactions{db: db, operations: operations, identity: identity, outputs: outputs}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6205:6421 -->
````go
func NewTransactions(db *sql.DB, operations *repository.Operations, identity *Identity, outputs *Outputs) *Transactions {
	return &Transactions{db: db, operations: operations, identity: identity, outputs: outputs}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/transactions_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
