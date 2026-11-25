# 分步编写 `services/document/internal/logic/document/account_retention_service.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/192-account-retention-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定账号原公开正式版依赖，并在各文档清理完成后启用留存。

保存计划只进入PLANNED和排空，不能直接释放草稿或宣称注销完成。

写之前先按这条执行路线理解各段的连接：

1. Plan先取Account原身份屏障确认
2. retainedDocument/Dependencies验证固定release
3. Progress只分页原集合
4. Complete消费真实单文档清理回执并启用更高公开来源。

本文件需要守住的边界：

- 对象CLOSING时不能临时改ACTIVE绕过校验
- Doc成功后Account仍要最终去标识事务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/account_retention_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/account_retention_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/account_retention_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/account_retention_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:610 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	allocations "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
)

// account_retention.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetentionAuthority

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type AccountRetentionAuthority interface {`。

**这一段为什么这样写：** Doc清理账号内容前核对Account权威关闭事实，不把收到RPC等同用户已确认。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Confirmed</code> | <code>(context.Context, platform.AccountClosureBinding) (bool, error)</code> | 传入ctx及原账号关闭绑定，向Account核验该关闭是否已经确认；bool是核验结果，error表示无法确定。 |

<!-- write-step: 2 bytes=610:730 -->
````go
type AccountRetentionAuthority interface {
	Confirmed(context.Context, platform.AccountClosureBinding) (bool, error)
}

````

## 第 3 步：AccountDocumentCleaner

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type AccountDocumentCleaner interface {`。

**这一段为什么这样写：** 跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Clean</code> | <code>(context.Context, platform.AccountDocumentCleanupContext) error</code> | 传入ctx及固定文档清理上下文，让协作服务清理对应持久正文；错误继续原绑定恢复。 |

<!-- write-step: 3 bytes=730:843 -->
````go
type AccountDocumentCleaner interface {
	Clean(context.Context, platform.AccountDocumentCleanupContext) error
}

````

## 第 4 步：AccountRetentions

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`type AccountRetentions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authority</code> | <code>AccountRetentionAuthority</code> | 使用 <code>AccountRetentionAuthority</code> 类型；其定义处的职责：Doc清理账号内容前核对Account权威关闭事实，不把收到RPC等同用户已确认。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。 |

<!-- write-step: 4 bytes=843:996 -->
````go
type AccountRetentions struct {
	db        *sql.DB
	authority AccountRetentionAuthority
	validator *codec.Validator
	cleaner   AccountDocumentCleaner
}

````

## 第 5 步：NewAccountRetentions

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewAccountRetentions(db *sql.DB, authority AccountRetentionAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *AccountRetentions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>authority</code> | <code>AccountRetentionAuthority</code> | 使用 <code>AccountRetentionAuthority</code> 类型；其定义处的职责：Doc清理账号内容前核对Account权威关闭事实，不把收到RPC等同用户已确认。<br>在此声明中的实际使用：<code>authority: authority</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>cleaner: cleaner</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountRetentions</code> | 实际返回表达式：<code>&amp;AccountRetentions{db: db, authority: authority, validator: validator, cleaner: cleaner}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountRetentions{db: db, authority: authority, validator: validator, cleaner: cleaner}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=996:1252 -->
````go
func NewAccountRetentions(db *sql.DB, authority AccountRetentionAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *AccountRetentions {
	return &AccountRetentions{db: db, authority: authority, validator: validator, cleaner: cleaner}
}

````

## 第 6 步：AccountRetentions.Plan

插入位置：第 5 步代码之后，紧接追加；完成稿约第 43 行。

Plan先取得Account原固定身份屏障的权威确认，再固定当前公开正式版。
这里只进入PLANNED和排空，不能因为计划已保存就释放草稿保护或结束全局注销。

本步声明：`func (s *AccountRetentions) Plan(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)</code>；<code>repository.DiscoverAccountRetention(ctx, tx, binding)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>platform.AccountClosureBinding(binding)</code>；<code>binding.Operation</code>；<code>boundRetentionPlan(original, binding)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>实际返回表达式：<code>empty</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETENTION_AUTHORITY_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_FREEZE_UNCONFIRMED&quot;)</code>；<code>nil</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid() &#124;&#124; s.authority == nil &#124;&#124; s.validator == nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_AUTHORITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!confirmed</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_FREEZE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention != nil</code>。条件成立时执行 <code>return plan, nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;FROZEN&quot;</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=1252:2966 -->
````go
// Plan先取得Account原固定身份屏障的权威确认，再固定当前公开正式版。
// 这里只进入PLANNED和排空，不能因为计划已保存就释放草稿保护或结束全局注销。
func (s *AccountRetentions) Plan(ctx context.Context, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error) {
	var empty domain.AccountRetirementPlan
	if !binding.Valid() || s.authority == nil || s.validator == nil {
		return empty, errors.New("ACCOUNT_RETENTION_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return empty, err
	}
	if !confirmed {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_FREEZE_UNCONFIRMED")
	}
	// 跨服务查询必须在SQL锁外。确认事实不可逆；冻结后Account不再允许取消。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	original, err := repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return empty, err
	}
	if plan.Retention != nil {
		return plan, nil
	}
	if original.State != "FROZEN" {
		return empty, errors.New("ACCOUNT_RETENTION_STATE_INVALID")
	}
	candidates, err := repository.DiscoverAccountRetention(ctx, tx, binding)
	if err != nil {
		return empty, err
	}
	if err = lockAccountRetentionSources(ctx, tx, candidates.Sources, binding); err != nil {
		return empty, err
	}
	original, err = repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return empty, err
	}
````

## 第 7 步：AccountRetentions.Plan（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 88 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention != nil</code>。条件成立时执行 <code>return plan, nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;FROZEN&quot;</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;PERSONAL_TRANSFER_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!slices.Equal(candidates.Spaces, current.Spaces) &#124;&#124; !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) &#124;&#124; !slices.Equal(candidates.Documents, current.Documents) &#124;&#124; !s …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_CANDIDATES_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; epoch == math.MaxUint64</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!retention.Valid(binding.Operation)</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_PLAN_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=2966:4943 -->
````go
	plan, err = boundRetentionPlan(original, binding)
	if err != nil {
		return empty, err
	}
	if plan.Retention != nil {
		return plan, nil
	}
	if original.State != "FROZEN" {
		return empty, errors.New("ACCOUNT_RETENTION_STATE_INVALID")
	}
	// Account 已冻结身份，但已接受的跨空间原迁移不能同时被固定进两份清理计划。
	pending, err := repository.PersonalTransferAccountPending(ctx, tx, binding.User)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "PERSONAL_TRANSFER_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return empty, err
	}
	current, err := repository.DiscoverAccountRetention(ctx, tx, binding)
	if err != nil {
		return empty, err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_CANDIDATES_CHANGED")
	}
	retention := domain.AccountRetentionPlan{Spaces: current.Spaces, KnowledgeBases: current.KnowledgeBases, Documents: []domain.AccountRetentionDocument{}, Scopes: []domain.AccountRetentionScope{}}
	for _, id := range current.Documents {
		item, err := s.retainedDocument(ctx, tx, id)
		if err != nil {
			return empty, err
		}
		retention.Documents = append(retention.Documents, item)
	}
	for _, source := range current.Sources {
		epoch, err := domain.CanonicalID(source.Epoch)
		if err != nil || epoch == math.MaxUint64 {
			return empty, errors.New("ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
		}
		retention.Scopes = append(retention.Scopes, domain.AccountRetentionScope{Key: source.Key, Epoch: epoch + 1, Operation: domain.AccountRetentionRevocation(binding.Operation, source.Key)})
	}
	plan.Retention = &retention
	if !retention.Valid(binding.Operation) {
````

## 第 8 步：AccountRetentions.Plan（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 133 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_PLAN_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RETENTION_PLAN_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4943:5871 -->
````go
		return empty, errors.New("ACCOUNT_RETENTION_PLAN_INVALID")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return empty, err
	}
	if err = repository.SaveAccountRetention(ctx, tx, binding.Operation, raw, retention); err != nil {
		return empty, err
	}
	for _, scope := range retention.Scopes {
		// 复用真实Doc/Edge/Node排空worker。子操作只终结自己的屏障任务，
		// 没有用户operation_receipt，不会把原ACCOUNT_CLOSE提前写成成功。
		closure := domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Action: "ACCOUNT_RETENTION"}
		if err = QueueClosure(ctx, tx, scope.Operation, binding.User, closure); err != nil {
			return empty, err
		}
	}
	if err = queueVisibilityIndexEvents(ctx, tx, binding.Operation, current.Documents, 0); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_RETENTION_PLAN_UNKNOWN")
	}
	return plan, nil
}

````

## 第 9 步：boundRetentionPlan

插入位置：第 8 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func boundRetentionPlan(record repository.AccountRetirementRecord, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>record</code> | <code>repository.AccountRetirementRecord</code> | 使用 <code>repository.AccountRetirementRecord</code> 类型；其定义处的职责：保存账号注销准备、冻结、拒绝及显式取消的持久事实。<br>在此声明中的实际使用：<code>record.Plan</code>；<code>record.Kind</code>；<code>record.Actor</code>；<code>record.Epoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>plan.Binding.Same(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_BINDING_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_REJECTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; record.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; record.Actor != binding.User &#124;&#124; record.Epoch != binding.IdentityEpoch &#124;&#124; !plan.Binding.Same(binding)</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_REJECTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5871:6473 -->
````go
func boundRetentionPlan(record repository.AccountRetirementRecord, binding domain.AccountRetirementBinding) (domain.AccountRetirementPlan, error) {
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil || record.Kind != "ACCOUNT_CLOSE" || record.Actor != binding.User || record.Epoch != binding.IdentityEpoch || !plan.Binding.Same(binding) {
		return plan, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_BINDING_CHANGED")
	}
	if record.State == "REJECTED" {
		return plan, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_REJECTED")
	}
	return plan, nil
}

````

## 第 10 步：lockAccountRetentionSources

插入位置：第 9 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func lockAccountRetentionSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, page)</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, binding)</code>；<code>repository.PendingSourceTransitionsBatch(ctx, tx, keys)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, page)</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, binding)</code>；<code>repository.PendingSourceTransitionsBatch(ctx, tx, keys)</code> |
| <code>sources</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>slices.Clone(sources)</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>a.Key &lt; b.Key</code>。条件成立时执行 <code>return -1</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>a.Key &gt; b.Key</code>。条件成立时执行 <code>return 1</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>source.Key == identity</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; epoch</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=6473:7877 -->
````go
func lockAccountRetentionSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, binding domain.AccountRetirementBinding) error {
	all := slices.Clone(sources)
	identity := "identity:" + domain.TextID(binding.User)
	all = append(all, domain.LeaseSource{Key: identity, Epoch: domain.TextID(binding.IdentityEpoch)})
	slices.SortFunc(all, func(a, b domain.LeaseSource) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	// 100项只限制一次SQL大小，不能截断整份关闭计划。
	for start := 0; start < len(all); start += 100 {
		end := min(start+100, len(all))
		page := all[start:end]
		gates, err := repository.LockSourceGatesBatch(ctx, tx, page)
		if err != nil {
			return err
		}
		keys := []string{}
		for _, source := range page {
			epoch, err := domain.CanonicalID(source.Epoch)
			if err != nil {
				return err
			}
			gate := gates[source.Key]
			if source.Key == identity {
				matches, err := repository.AccountRetirementGateMatches(ctx, tx, binding)
				if err != nil {
					return err
				}
				if !matches {
					return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
				}
			} else {
				if gate.Epoch > epoch {
					return errors.New("ACCOUNT_RETENTION_SOURCE_CHANGED")
				}
				keys = append(keys, source.Key)
			}
		}
		pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, keys)
		if err != nil {
````

## 第 11 步：lockAccountRetentionSources（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 215 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7877:8088 -->
````go
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
		}
	}
	return nil
}

// account_retention_complete.go：保留原实现，按业务合并文件。

````

## 第 12 步：AccountRetentions.Complete

插入位置：第 11 步代码之后，紧接追加；完成稿约第 226 行。

Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
循环同步清理整个账号。Account恢复器应逐文档推进后调用本关口，原号重放。
Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须自己的最终去标识事务。

本步声明：`func (s *AccountRetentions) Complete(ctx context.Context, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>lockRetentionCompletionSources(ctx, tx, plan)</code>；<code>repository.LockedAccountRetirement(ctx, tx, binding.Operation)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>binding.Operation</code>；<code>boundRetentionPlan(original, binding)</code>；<code>boundRetentionPlan(locked, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_CLEANING_REQUIRED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid() &#124;&#124; s.validator == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;CLEANING&quot; &#124;&#124; plan.Retention == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_CLEANING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>locked.State != &quot;CLEANING&quot; &#124;&#124; string(original.Plan) != string(locked.Plan)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=8088:9733 -->
````go
// Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
// 循环同步清理整个账号。Account恢复器应逐文档推进后调用本关口，原号重放。
// Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须自己的最终去标识事务。
func (s *AccountRetentions) Complete(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.validator == nil {
		return errors.New("ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return err
	}
	if original.State == "SUCCEEDED" {
		return nil
	}
	if original.State != "CLEANING" || plan.Retention == nil {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_CLEANING_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockRetentionCompletionSources(ctx, tx, plan); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	current, err := boundRetentionPlan(locked, binding)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.State != "CLEANING" || string(original.Plan) != string(locked.Plan) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_CHANGED")
	}
	drained, err := repository.AccountRetentionDrained(ctx, tx, current)
	if err != nil {
````

## 第 13 步：AccountRetentions.Complete（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 271 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9733:10978 -->
````go
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_DRAIN_PENDING")
	}
	if err = repository.LockAccountRetentionCompletion(ctx, tx, *current.Retention); err != nil {
		return err
	}
	for _, item := range current.Retention.Documents {
		if err = s.validateRetentionCompletion(ctx, tx, current, item); err != nil {
			return err
		}
	}
	if err = repository.CompleteAccountRetentionEntities(ctx, tx, *current.Retention); err != nil {
		return err
	}
	// 与准备期隐藏事件使用不同固定阶段号，不冲突、不创建新的正式历史。
	digest := sha256.Sum256([]byte("ACCOUNT_RETENTION_COMPLETE\n" + binding.Operation))
	indexOperation := "retained-index-" + hex.EncodeToString(digest[:])
	documents := make([]uint64, 0, len(current.Retention.Documents))
	for _, item := range current.Retention.Documents {
		documents = append(documents, item.Document)
	}
	if err = queueVisibilityIndexEvents(ctx, tx, indexOperation, documents, 0); err != nil {
		return err
	}
	if err = repository.CompleteAccountRetention(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 14 步：lockRetentionCompletionSources

插入位置：第 13 步代码之后，紧接追加；完成稿约第 306 行。

来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。

本步声明：`func lockRetentionCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, sources&#91;start:min(start+100, len(sources))&#93;)</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, plan.Binding)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, sources&#91;start:min(start+100, len(sources))&#93;)</code>；<code>repository.AccountRetirementGateMatches(ctx, tx, plan.Binding)</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Binding</code>；<code>plan.Retention</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return strings.Compare(a.Key, b.Key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10978:12084 -->
````go
// 来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
// 提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。
func lockRetentionCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) error {
	sources := []domain.LeaseSource{{Key: "identity:" + domain.TextID(plan.Binding.User), Epoch: domain.TextID(plan.Binding.IdentityEpoch)}}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	slices.SortFunc(sources, func(a, b domain.LeaseSource) int { return strings.Compare(a.Key, b.Key) })
	for start := 0; start < len(sources); start += 100 {
		if _, err := repository.LockSourceGatesBatch(ctx, tx, sources[start:min(start+100, len(sources))]); err != nil {
			return err
		}
	}
	matches, err := repository.AccountRetirementGateMatches(ctx, tx, plan.Binding)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	return nil
}

````

## 第 15 步：AccountRetentions.validateRetentionCompletion

插入位置：第 14 步代码之后，紧接追加；完成稿约第 329 行。

本步声明：`func (s *AccountRetentions) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, tx, request)</code>；<code>repository.PublicationFactsRow(ctx, tx, item.Document, false)</code>；<code>s.retainedDependencies(ctx, tx, doc, actual)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, tx, request)</code>；<code>repository.PublicationFactsRow(ctx, tx, item.Document, false)</code>；<code>s.retainedDependencies(ctx, tx, doc, actual)</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.CleanupDocument</code>；<code>repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>plan.Binding</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>item.Release</code>；<code>item.Manifest</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Key == &quot;doc-acl:&quot;+request.Document &amp;&amp; proof != platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>item.Release == 0</code>。
- 检查 <code>doc.PublicationState != &quot;UNPUBLISHED&quot; &#124;&#124; doc.CurrentReleaseId != nil &#124;&#124; doc.CurrentManifestId != &quot;&quot; &#124;&#124; doc.CurrentStructureId != &quot;&quot; &#124;&#124; doc.CurrentStyleId != &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>policy != &quot;PUBLIC&quot; &#124;&#124; doc.CurrentReleaseId == nil &#124;&#124; *doc.CurrentReleaseId != domain.TextID(item.Release) &#124;&#124; doc.CurrentManifestId != domain.TextID(item.Manifest) &#124;&#124; doc.CurrentStr …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PUBLIC_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=12084:14035 -->
````go
func (s *AccountRetentions) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument) error {
	_, scopes, err := plan.CleanupDocument(item.Document)
	if err != nil {
		return err
	}
	if err = repository.LockAccountCleanupDocument(ctx, tx, plan, item, scopes); err != nil {
		return err
	}
	request := platform.AccountDocumentCleanupRequest{Operation: plan.Binding.Operation, Document: domain.TextID(item.Document)}
	proof, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request)
	if err != nil {
		return err
	}
	if !found {
		return domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document && proof != platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation) {
			return errors.New("ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED")
		}
	}
	facts, err := repository.PublicationFactsRow(ctx, tx, item.Document, false)
	if err != nil {
		return err
	}
	doc := facts.Document
	if item.Release == 0 {
		if doc.PublicationState != "UNPUBLISHED" || doc.CurrentReleaseId != nil || doc.CurrentManifestId != "" || doc.CurrentStructureId != "" || doc.CurrentStyleId != "" {
			return errors.New("ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS")
		}
		return nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return err
	}
	if policy != "PUBLIC" || doc.CurrentReleaseId == nil || *doc.CurrentReleaseId != domain.TextID(item.Release) || doc.CurrentManifestId != domain.TextID(item.Manifest) || doc.CurrentStructureId != domain.TextID(item.Structure) || doc.CurrentStyleId != domain.TextID(item.Style) {
		return errors.New("ACCOUNT_RETENTION_PUBLIC_BINDING_CHANGED")
	}
	actual := item
	actual.Versions, actual.Resources = []uint64{}, []uint64{}
	actual, err = s.retainedDependencies(ctx, tx, doc, actual)
	if err != nil {
		return err
	}
````

## 第 16 步：AccountRetentions.validateRetentionCompletion（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 374 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>actual.Manifest != item.Manifest &#124;&#124; actual.Structure != item.Structure &#124;&#124; actual.Style != item.Style &#124;&#124; !slices.Equal(actual.Versions, item.Versions) &#124;&#124; !slices.Equal(actual.Resources, item.Resources)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_DEPENDENCIES_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=14035:14405 -->
````go
	if actual.Manifest != item.Manifest || actual.Structure != item.Structure || actual.Style != item.Style || !slices.Equal(actual.Versions, item.Versions) || !slices.Equal(actual.Resources, item.Resources) {
		return errors.New("ACCOUNT_RETENTION_DEPENDENCIES_CHANGED")
	}
	return nil
}

// account_retention_dependencies.go：保留原实现，按业务合并文件。

````

## 第 17 步：AccountRetentions.retainedDocument

插入位置：第 16 步代码之后，紧接追加；完成稿约第 382 行。

本步声明：`func (s *AccountRetentions) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PublicationFactsRow(ctx, tx, id, false)</code>；<code>s.retainedDependencies(ctx, tx, facts.Document, item)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.PublicationFactsRow(ctx, tx, id, false)</code>；<code>s.retainedDependencies(ctx, tx, facts.Document, item)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Document: id</code>；<code>repository.PublicationFactsRow(ctx, tx, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>由 <code>s.retainedDependencies(ctx, tx, facts.Document, item)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>item</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.retainedDependencies(ctx, tx, facts.Document, item)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_DOCUMENT_TRANSITION_PENDING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。
- 检查 <code>facts.KnowledgeBase != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。
- 检查 <code>facts.Document.Lifecycle != &quot;ACTIVE&quot; &amp;&amp; facts.Document.Lifecycle != &quot;TRASHED&quot;</code>。条件成立时执行 <code>return item, errors.New(&quot;ACCOUNT_RETENTION_DOCUMENT_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。
- 检查 <code>policy != &quot;PUBLIC&quot; &#124;&#124; !domain.HasCurrentPublication(facts, policy)</code>。条件成立时执行 <code>return item, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。
- 此处通过 <code>return s.retainedDependencies(ctx, tx, facts.Document, item)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=14405:15509 -->
````go
func (s *AccountRetentions) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error) {
	item := domain.AccountRetentionDocument{Document: id, Versions: []uint64{}, Resources: []uint64{}}
	facts, err := repository.PublicationFactsRow(ctx, tx, id, false)
	if err != nil {
		return item, err
	}
	item.Space, err = domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return item, err
	}
	if facts.KnowledgeBase != nil {
		item.KnowledgeBase, err = domain.CanonicalID(facts.KnowledgeBase.Id)
		if err != nil {
			return item, err
		}
	}
	if facts.Document.Lifecycle != "ACTIVE" && facts.Document.Lifecycle != "TRASHED" {
		return item, errors.New("ACCOUNT_RETENTION_DOCUMENT_TRANSITION_PENDING")
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return item, err
	}
	if policy != "PUBLIC" || !domain.HasCurrentPublication(facts, policy) {
		return item, nil
	}
	item.Release, err = domain.CanonicalID(*facts.Document.CurrentReleaseId)
	if err != nil {
		return item, err
	}
	return s.retainedDependencies(ctx, tx, facts.Document, item)
}

````

## 第 18 步：AccountRetentions.retainedDependencies

插入位置：第 17 步代码之后，紧接追加；完成稿约第 415 行。

准备期与最终留存启用共享同一依赖核验；启用期对象仍CLOSING，不能临时
改成ACTIVE绕过状态检查。这里只解析固定release，不选择新的保留范围。

本步声明：`func (s *AccountRetentions) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, item.Release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.FixedReleaseRow(ctx, tx, id, item.Release)</code>；<code>repository.StructureRow(ctx, tx, fixed.Structure, document.Id)</code>；<code>repository.StyleRow(ctx, tx, fixed.Style, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>document.Id</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>item.Release</code>；<code>return item, err</code>；<code>item.Manifest</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>实际返回表达式：<code>item</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_RESOURCE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=15509:17075 -->
````go
// 准备期与最终留存启用共享同一依赖核验；启用期对象仍CLOSING，不能临时
// 改成ACTIVE绕过状态检查。这里只解析固定release，不选择新的保留范围。
func (s *AccountRetentions) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error) {
	id := item.Document
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, item.Release)
	if err != nil {
		return item, err
	}
	item.Manifest, err = domain.CanonicalID(fixed.Release.ManifestId)
	if err != nil {
		return item, err
	}
	item.Structure, err = domain.CanonicalID(fixed.Structure)
	if err != nil {
		return item, err
	}
	item.Style, err = domain.CanonicalID(fixed.Style)
	if err != nil {
		return item, err
	}
	structure, err := repository.StructureRow(ctx, tx, fixed.Structure, document.Id)
	if err != nil {
		return item, err
	}
	if _, err = repository.StyleRow(ctx, tx, fixed.Style, document.Id); err != nil {
		return item, err
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return item, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return item, err
	}
	resources := map[uint64]bool{}
	for _, unit := range units {
		unitID, err := domain.CanonicalID(unit["unit_id"])
		if err != nil {
			return item, err
		}
		version, err := domain.CanonicalID(unit["content_version_id"])
		if err != nil {
			return item, err
		}
		content, err := repository.VersionContent(ctx, tx, version, id)
````

## 第 19 步：AccountRetentions.retainedDependencies（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 460 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return item, err</code>。当前函数在这里结束。
- 检查 <code>!allowed &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>return item, errors.New(&quot;ACCOUNT_RETENTION_RESOURCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return item, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=17075:18267 -->
````go
		if err != nil {
			return item, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return item, err
		}
		if err = validateFixedMergeContent(s.validator, content, id, unitID, item.Structure); err != nil {
			return item, err
		}
		item.Versions = append(item.Versions, version)
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	slices.Sort(item.Versions)
	item.Versions = slices.Compact(item.Versions)
	for resource := range resources {
		item.Resources = append(item.Resources, resource)
	}
	slices.Sort(item.Resources)
	// 依赖必须仍属于本空间并READY。持有文档锁后检查引用，GC无法在引用存在时删除。
	for _, resource := range item.Resources {
		state, err := repository.ResourceState(ctx, tx, resource)
		if err != nil {
			return item, err
		}
		allowed, err := allocations.ResourceAllowedForDocument(ctx, tx, resource, item.Document, item.Space)
		if err != nil {
			return item, err
		}
		if !allowed || state != "READY" {
			return item, errors.New("ACCOUNT_RETENTION_RESOURCE_UNKNOWN")
		}
	}
	return item, nil
}

// account_retention_progress.go：保留原实现，按业务合并文件。

````

## 第 20 步：AccountRetentions.Progress

插入位置：第 19 步代码之后，紧接追加；完成稿约第 499 行。

原固定计划只读分页，不扫描可变空间/文档集合，也不等待清理或执行网络调用。
完成证明来自同一Doc事务写入的SUCCEEDED，Account还须核对自己的冻结/ACK事实。

本步声明：`func (s *AccountRetentions) Progress(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)</code> |
| <code>request</code> | <code>platform.AccountClosurePageRequest</code> | 使用 <code>platform.AccountClosurePageRequest</code> 类型；其定义处的职责：控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要， Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。<br>在此声明中的实际使用：<code>request.AccountClosureBinding</code>；<code>request.PlanDigest</code>；<code>request.AfterDocument</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosurePage</code> | 使用 <code>platform.AccountClosurePage</code> 类型；其定义处的职责：定义Account与Document之间的固定注销绑定和有界清理分页协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid()</code>。条件成立时执行 <code>return result, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil &#124;&#124; (original.State != &quot;PLANNED&quot; &amp;&amp; original.State != &quot;CLEANING&quot; &amp;&amp; original.State != &quot;SUCCEEDED&quot;)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>request.PlanDigest != &quot;&quot; &amp;&amp; request.PlanDigest != result.PlanDigest</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>request.AfterDocument != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; request.PlanDigest == &quot;&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>item.Document &lt;= after</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>len(result.Documents) == 100</code>。

<!-- write-step: 20 bytes=18267:20376 -->
````go
// 原固定计划只读分页，不扫描可变空间/文档集合，也不等待清理或执行网络调用。
// 完成证明来自同一Doc事务写入的SUCCEEDED，Account还须核对自己的冻结/ACK事实。
func (s *AccountRetentions) Progress(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error) {
	result := platform.AccountClosurePage{Documents: []platform.AccountClosureDocument{}}
	binding := domain.AccountRetirementBinding(request.AccountClosureBinding)
	if !binding.Valid() {
		return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return result, err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return result, err
	}
	if plan.Retention == nil || (original.State != "PLANNED" && original.State != "CLEANING" && original.State != "SUCCEEDED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	digest := sha256.Sum256(original.Plan)
	result.PlanDigest = hex.EncodeToString(digest[:])
	if request.PlanDigest != "" && request.PlanDigest != result.PlanDigest {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
	}
	var after uint64
	if request.AfterDocument != "" {
		after, err = domain.CanonicalID(request.AfterDocument)
		if err != nil || request.PlanDigest == "" {
			return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_CURSOR_INVALID")
		}
		found := false
		for _, item := range plan.Retention.Documents {
			found = found || item.Document == after
		}
		if !found {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
		}
	}
	result.Binding, result.State, result.NextAfter, result.Done = request.AccountClosureBinding, original.State, request.AfterDocument, true
	for _, item := range plan.Retention.Documents {
		if item.Document <= after {
			continue
		}
		if len(result.Documents) == 100 {
			result.Done = false
````

## 第 21 步：AccountRetentions.Progress（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 544 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=20376:20621 -->
````go
			break
		}
		result.Documents = append(result.Documents, platform.AccountClosureDocument{Document: domain.TextID(item.Document), Release: domain.TextID(item.Release)})
		result.NextAfter = domain.TextID(item.Document)
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/account_retention_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
