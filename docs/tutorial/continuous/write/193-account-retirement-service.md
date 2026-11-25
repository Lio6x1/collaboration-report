# 分步编写 `services/document/internal/logic/document/account_retirement_service.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/193-account-retirement-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为Account原注销操作准备管理员预留和冻结，并处理有证明的取消。

此流程是跨服务准备而非用户直接注销完成；组织关闭范围必须来自Account权威。

写之前先按这条执行路线理解各段的连接：

1. Prepare核AccountRetirementBinding和管理员候选，固定计划
2. Cancel仅接受Account持久REJECTED且未进入实际排空的原操作。

本文件需要守住的边界：

- 超时、缺行、租约到期都不代表回滚
- 浏览器不能自行声明ClosingOrganizations。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/account_retirement_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/account_retirement_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/account_retirement_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/account_retirement_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:412 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// account_retirement.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetirementAuthority

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type AccountRetirementAuthority interface {`。

**这一段为什么这样写：** 恢复管理员退役预约时区分“拒绝”和“暂时查不到”，避免未知操作被当作取消。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Rejected</code> | <code>(context.Context, platform.AccountClosureBinding) (bool, error)</code> | 传入ctx及原关闭绑定，查询Account是否明确拒绝这次关闭；仅明确拒绝才释放对应预约。 |

<!-- write-step: 2 bytes=412:532 -->
````go
type AccountRetirementAuthority interface {
	Rejected(context.Context, platform.AccountClosureBinding) (bool, error)
}

````

## 第 3 步：AccountRetirements

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type AccountRetirements struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authority</code> | <code>AccountRetirementAuthority</code> | 使用 <code>AccountRetirementAuthority</code> 类型；其定义处的职责：恢复管理员退役预约时区分“拒绝”和“暂时查不到”，避免未知操作被当作取消。 |

<!-- write-step: 3 bytes=532:625 -->
````go
type AccountRetirements struct {
	db        *sql.DB
	authority AccountRetirementAuthority
}

````

## 第 4 步：NewAccountRetirements

插入位置：第 3 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func NewAccountRetirements(db *sql.DB, authority AccountRetirementAuthority) *AccountRetirements`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>authority</code> | <code>AccountRetirementAuthority</code> | 使用 <code>AccountRetirementAuthority</code> 类型；其定义处的职责：恢复管理员退役预约时区分“拒绝”和“暂时查不到”，避免未知操作被当作取消。<br>在此声明中的实际使用：<code>authority: authority</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountRetirements</code> | 实际返回表达式：<code>&amp;AccountRetirements{db: db, authority: authority}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountRetirements{db: db, authority: authority}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=625:785 -->
````go
func NewAccountRetirements(db *sql.DB, authority AccountRetirementAuthority) *AccountRetirements {
	return &AccountRetirements{db: db, authority: authority}
}

````

## 第 5 步：AccountRetirements.Prepare

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

Prepare是跨服务注销准备的一部分，不是注销完成。调用者必须是已验证原操作的
Account控制通道；当前不作为用户HTTP入口，不能接收用户自行声明的关闭组织集合。

本步声明：`func (s *AccountRetirements) Prepare(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.LockSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(binding.User), binding.IdentityEpoch)</code>；<code>repository.LockedAccountRetirement(ctx, tx, binding.Operation)</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, binding)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>binding.Operation</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetirementResult</code> | 使用 <code>domain.AccountRetirementResult</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETIREMENT_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_BINDING_CHANGED&quot;)</code>；<code>nil</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid()</code>。条件成立时执行 <code>return result, errors.New(&quot;ACCOUNT_RETIREMENT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>decodeErr != nil &#124;&#124; record.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; record.Actor != binding.User &#124;&#124; record.Epoch != binding.IdentityEpoch &#124;&#124; !plan.Binding.Same(binding)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;REJECTED&quot;</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; binding.IdentityEpoch &#124;&#124; (gate.Epoch == binding.IdentityEpoch &amp;&amp; gate.State == &quot;BLOCKED&quot;)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=785:3119 -->
````go
// Prepare是跨服务注销准备的一部分，不是注销完成。调用者必须是已验证原操作的
// Account控制通道；当前不作为用户HTTP入口，不能接收用户自行声明的关闭组织集合。
func (s *AccountRetirements) Prepare(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementResult, error) {
	result := domain.AccountRetirementResult{RequiredKnowledgeBases: []uint64{}, ReservedKnowledgeBases: []uint64{}}
	if !binding.Valid() {
		return result, errors.New("ACCOUNT_RETIREMENT_BINDING_INVALID")
	}
	// 不存在的原操作不需要范围锁：RR的缺行间隙锁会与随后知识库锁反向等待。
	// RC下仍显式锁定身份来源、知识库、成员和退休事实；操作唯一键处理跨用户撞号。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// 与任命目标身份、本人建库使用同一来源锁，预留后不能迟到获得新的管理员职责。
	gate, err := repository.LockSourceGate(ctx, tx, "identity:"+domain.TextID(binding.User), binding.IdentityEpoch)
	if err != nil {
		return result, err
	}
	record, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err == nil {
		plan, decodeErr := domain.DecodeAccountRetirementPlan(record.Plan)
		if decodeErr != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
		}
		if record.State == "REJECTED" {
			result.State = "REJECTED"
			return result, nil
		}
		matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
		if err != nil {
			return result, err
		}
		if !matches {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
		}
		result.State, result.ReservedKnowledgeBases = record.State, plan.ReservedKnowledgeBases
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if gate.Epoch > binding.IdentityEpoch || (gate.Epoch == binding.IdentityEpoch && gate.State == "BLOCKED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
````

## 第 6 步：AccountRetirements.Prepare（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>candidate.PersonalOwner == binding.User &#124;&#124; slices.Contains(binding.ClosingOrganizations, candidate.Organization)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>candidate.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; candidate.MemberState != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;KB_ADMIN_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>candidate.ReservationState == &quot;RESERVED&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;KB_ADMIN_RETIREMENT_RESERVED&quot;)</code>。当前函数在这里结束。
- 检查 <code>candidate.Available &lt;= 1</code>。
- 检查 <code>len(result.RequiredKnowledgeBases) &gt; 0</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;ACCOUNT_RETIREMENT_PREPARE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3119:5097 -->
````go
	candidates, err := repository.LockAdministratorRetirementCandidates(ctx, tx, binding.User)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		// 自有个人空间及权威明确将关闭的组织不属于继续存续的库。
		if candidate.PersonalOwner == binding.User || slices.Contains(binding.ClosingOrganizations, candidate.Organization) {
			continue
		}
		if candidate.Lifecycle != "ACTIVE" || candidate.MemberState != "ACTIVE" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_TRANSITION_PENDING")
		}
		if candidate.ReservationState == "RESERVED" {
			return result, domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
		}
		if candidate.Available <= 1 {
			result.RequiredKnowledgeBases = append(result.RequiredKnowledgeBases, candidate.KnowledgeBase)
		}
		result.ReservedKnowledgeBases = append(result.ReservedKnowledgeBases, candidate.KnowledgeBase)
	}
	if len(result.RequiredKnowledgeBases) > 0 {
		// 不提交任何来源或部分预留，用户仍可正常交接后重新发起。
		result.State, result.ReservedKnowledgeBases = "HANDOFF_REQUIRED", []uint64{}
		return result, nil
	}
	for _, kb := range result.ReservedKnowledgeBases {
		if err = repository.ReserveAdministratorRetirement(ctx, tx, kb, binding.User, binding.Operation); err != nil {
			return result, err
		}
	}
	plan, err := json.Marshal(domain.AccountRetirementPlan{Binding: binding, ReservedKnowledgeBases: result.ReservedKnowledgeBases})
	if err != nil {
		return result, err
	}
	if err = repository.RecordAccountRetirement(ctx, tx, binding, plan); err != nil {
		return result, err
	}
	if err = repository.FreezeAccountRetirement(ctx, tx, binding); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, errors.New("ACCOUNT_RETIREMENT_PREPARE_UNKNOWN")
	}
	result.State = "FROZEN"
	return result, nil
}

// account_retirement_cancel.go：保留原实现，按业务合并文件。

````

## 第 7 步：AccountRetirements.Cancel

插入位置：第 6 步代码之后，紧接追加；完成稿约第 124 行。

Cancel仅接受Account对原固定计划的持久REJECTED证明。超时、查无记录和租约到期
都不是回滚证明；已开始实际排空的操作也不能通过这个准备阶段接口撤销。

本步声明：`func (s *AccountRetirements) Cancel(ctx context.Context, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authority.Rejected(ctx, platform.AccountClosureBinding(binding))</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.LockSourceGate(ctx, tx, &quot;identity:&quot;+domain.TextID(binding.User), binding.IdentityEpoch)</code>；<code>repository.LockedAccountRetirement(ctx, tx, binding.Operation)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>platform.AccountClosureBinding(binding)</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETIREMENT_AUTHORITY_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_CANCEL_UNCONFIRMED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid() &#124;&#124; s.authority == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_AUTHORITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!rejected</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_CANCEL_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_CANCEL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; record.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; record.Actor != binding.User &#124;&#124; record.Epoch != binding.IdentityEpoch &#124;&#124; !plan.Binding.Same(binding)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=5097:7143 -->
````go
// Cancel仅接受Account对原固定计划的持久REJECTED证明。超时、查无记录和租约到期
// 都不是回滚证明；已开始实际排空的操作也不能通过这个准备阶段接口撤销。
func (s *AccountRetirements) Cancel(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.authority == nil {
		return errors.New("ACCOUNT_RETIREMENT_AUTHORITY_REQUIRED")
	}
	rejected, err := s.authority.Rejected(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return err
	}
	if !rejected {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_CANCEL_UNCONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = repository.LockSourceGate(ctx, tx, "identity:"+domain.TextID(binding.User), binding.IdentityEpoch); err != nil {
		return err
	}
	record, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if errors.Is(err, sql.ErrNoRows) {
		// Prepare可能尚未到达。权威拒绝仍须留下同号墓碑，禁止迟到请求重新封门；
		// 不触碰其他操作的来源状态或预留，也不把已开始的排空伪装为未发生。
		if _, err = repository.LockedFence(ctx, tx, binding.Operation); err == nil {
			return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = repository.RecordRejectedAccountRetirement(ctx, tx, binding); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return errors.New("ACCOUNT_RETIREMENT_CANCEL_UNKNOWN")
		}
		return nil
	}
	if err != nil {
		return err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
	}
````

## 第 8 步：AccountRetirements.Cancel（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 169 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;FROZEN&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_ALREADY_ADVANCED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_CANCEL_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=7143:7954 -->
````go
	if record.State == "REJECTED" {
		return nil
	}
	if record.State != "FROZEN" {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_ALREADY_ADVANCED")
	}
	matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
	if err != nil {
		return err
	}
	if !matches {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	if _, err = repository.LockedFence(ctx, tx, binding.Operation); err == nil {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_FENCE_ALREADY_STARTED")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = repository.CancelAccountRetirement(ctx, tx, plan); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETIREMENT_CANCEL_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/account_retirement_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
