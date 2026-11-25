# 分步编写 `services/document/internal/logic/document/retained_governance_recovery.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/230-retained-governance-recovery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

收尾已审计留存治理并向Node提供原清理上下文。

管理员离线后可以完成原计划，但不能重新选范围或借Account旧号发新证。

写之前先按这条执行路线理解各段的连接：

1. lockedCurrent按来源→Doc→KB→Space→任务锁
2. DocumentCleanupContext核原ACK
3. removeHistory等CLEANED及导出排空
4. finish核资源终态。

本文件需要守住的边界：

- 恢复记录一致不替代真实清理回执
- 成功收尾不要求原管理员继续在线。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_recovery.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/retained_governance_recovery.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/retained_governance_recovery.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/retained_governance_recovery.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:261 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

````

## 第 2 步：ErrRetainedGovernanceIdle

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`var ErrRetainedGovernanceIdle = errors.New("RETAINED_GOVERNANCE_IDLE")`。

<!-- write-step: 2 bytes=261:333 -->
````go
var ErrRetainedGovernanceIdle = errors.New("RETAINED_GOVERNANCE_IDLE")

````

## 第 3 步：RetainedGovernance.Recover

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func (s *RetainedGovernance) Recover(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.ClaimRetainedGovernance(ctx, tx, s.instance)</code>；<code>s.recover(ctx, task)</code>；<code>context.WithoutCancel(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;RETAINED_GOVERNANCE_DEPENDENCIES_REQUIRED&quot;)</code>；<code>err</code>；<code>ErrRetainedGovernanceIdle</code>；<code>errors.New(&quot;RETAINED_CLAIM_COMMIT_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.cleaner == nil &#124;&#124; s.resources == nil &#124;&#124; s.exports == nil &#124;&#124; s.instance == &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_GOVERNANCE_DEPENDENCIES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return ErrRetainedGovernanceIdle</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_CLAIM_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(context.WithoutCancel(ctx), time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=333:1186 -->
````go
func (s *RetainedGovernance) Recover(ctx context.Context) error {
	if s.cleaner == nil || s.resources == nil || s.exports == nil || s.instance == "" {
		return errors.New("RETAINED_GOVERNANCE_DEPENDENCIES_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	task, err := repository.ClaimRetainedGovernance(ctx, tx, s.instance)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRetainedGovernanceIdle
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_CLAIM_COMMIT_UNKNOWN")
	}
	if err = s.recover(ctx, task); err != nil {
		retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = repository.RetryRetainedGovernance(retry, s.db, task, s.instance)
	}
	return err
}

````

## 第 4 步：RetainedGovernance.recover

插入位置：第 3 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func (s *RetainedGovernance) recover(ctx context.Context, task repository.PurgeTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, task.Operation)</code>；<code>s.DocumentCleanupContext(ctx, request)</code>；<code>s.exports.DrainDocument(ctx, plan.Purge.Document)</code>；<code>s.cleaner.Clean(ctx, expected)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>s.removeHistory(ctx, task, expected)</code>；<code>s.finish(ctx, task, plan)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.finish(ctx, task, plan)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.Action == &quot;PURGE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.finish(ctx, task, plan)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1186:2288 -->
````go
func (s *RetainedGovernance) recover(ctx context.Context, task repository.PurgeTask) error {
	original, err := s.Lookup(ctx, task.Operation)
	if err != nil {
		return err
	}
	plan := original.Plan
	if plan.Action == "PURGE" {
		request := platform.AccountDocumentCleanupRequest{Operation: task.Operation, Document: domain.TextID(plan.Purge.Document)}
		expected, err := s.DocumentCleanupContext(ctx, request)
		if err != nil {
			return err
		}
		// 先等来源真实ACK，然后在锁外排空导出和取得Node原清理回执。
		// 这些事实的结果未知时重送原号，不把进程租约当成正文/文件已删除。
		if err = s.exports.DrainDocument(ctx, plan.Purge.Document); err != nil {
			return err
		}
		if err = s.cleaner.Clean(ctx, expected); err != nil {
			return err
		}
		if err = s.removeHistory(ctx, task, expected); err != nil {
			return err
		}
		for _, resource := range plan.Resources {
			if err = s.resources.CleanRetainedResource(ctx, resource, plan.Purge.Space, plan.Purge.Document); err != nil {
				return err
			}
		}
	}
	return s.finish(ctx, task, plan)
}

````

## 第 5 步：RetainedGovernance.DocumentCleanupContext

插入位置：第 4 步代码之后，紧接追加；完成稿约第 77 行。

Node向Doc反查此context；它与Account的单文档协议相同，但发证资格只来自
本治理任务、精确原关闭范围及ACK，绝不借用旧Account操作重新发永久清理。

本步声明：`func (s *RetainedGovernance) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedCurrent(ctx, tx, request.Operation)</code>；<code>repository.DocumentWritesPending(ctx, tx, plan.Purge.Document, plan.Purge.Operation)</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>request.Operation</code>；<code>request.Document</code>；<code>platform.NewAccountDocumentCleanupContext(request, plan.Purge.ACL, plan.Purge.Scopes&#91;0&#93;.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>platform.NewAccountDocumentCleanupContext(request, plan.Purge.ACL, plan.Purge.Scopes&#91;0&#93;.Operation)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;RETAINED_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;RETAINED_PURGE_PLAN_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_WRITES_PENDING&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, domain.Violation(domain.InvalidRequest, &quot;RETAINED_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.Action != &quot;PURGE&quot; &#124;&#124; request.Document != domain.TextID(plan.Purge.Document)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.PermissionDenied, &quot;RETAINED_PURGE_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;RETAINED_WRITES_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;RETAINED_CONTEXT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return platform.NewAccountDocumentCleanupContext(request, plan.Purge.ACL, plan.Purge.Scopes&#91;0&#93;.Operation), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2288:3705 -->
````go
// Node向Doc反查此context；它与Account的单文档协议相同，但发证资格只来自
// 本治理任务、精确原关闭范围及ACK，绝不借用旧Account操作重新发永久清理。
func (s *RetainedGovernance) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "RETAINED_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	if plan.Action != "PURGE" || request.Document != domain.TextID(plan.Purge.Document) {
		return empty, domain.Violation(domain.PermissionDenied, "RETAINED_PURGE_PLAN_REQUIRED")
	}
	pending, err := repository.DocumentWritesPending(ctx, tx, plan.Purge.Document, plan.Purge.Operation)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "RETAINED_WRITES_PENDING")
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("RETAINED_CONTEXT_COMMIT_UNKNOWN")
	}
	return platform.NewAccountDocumentCleanupContext(request, plan.Purge.ACL, plan.Purge.Scopes[0].Operation), nil
}

````

## 第 6 步：RetainedGovernance.lockedCurrent

插入位置：第 5 步代码之后，紧接追加；完成稿约第 109 行。

恢复不要求平台管理员继续在线；只能完成已经带审计登记的原关闭计划。
来源→Doc→KB→Space→任务，其他worker不能持任务锁等待来源。

本步声明：`func (s *RetainedGovernance) lockedCurrent(ctx context.Context, tx *sql.Tx, operation string) (domain.RetainedGovernancePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, tx, operation, false)</code>；<code>repository.LockSourceGate(ctx, tx, source.Key, source.Epoch)</code>；<code>repository.TrashDocumentRow(ctx, tx, p.Document, true)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, tx, p.KnowledgeBase, true)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, tx, operation, false)</code>；<code>repository.LockSourceGate(ctx, tx, source.Key, source.Epoch)</code>；<code>repository.TrashDocumentRow(ctx, tx, p.Document, true)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, tx, p.KnowledgeBase, true)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.RetainedGovernanceRow(ctx, tx, operation, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>实际返回表达式：<code>original.Plan</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_PARENT_CHANGED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return original.Plan, err</code>。当前函数在这里结束。
- 检查 <code>p.KnowledgeBase != 0</code>。
- 在本段的匿名函数内：此处通过 <code>return scopes&#91;i&#93;.Key &lt; scopes&#91;j&#93;.Key</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>source.Operation == &quot;&quot;</code>。
- 检查 <code>gate.Epoch != source.Epoch &#124;&#124; gate.State != state</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>plan.Action == &quot;PURGE&quot;</code>。
- 检查 <code>d.Lifecycle != expectedLifecycle &#124;&#124; d.SpaceId != domain.TextID(p.Space) &#124;&#124; d.ControlRevision != domain.TextID(p.ClosingControl) &#124;&#124; 		d.AclRevision != domain.TextID(p.ACL) &#124;&#124; d.Publ …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>p.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=3705:5705 -->
````go
// 恢复不要求平台管理员继续在线；只能完成已经带审计登记的原关闭计划。
// 来源→Doc→KB→Space→任务，其他worker不能持任务锁等待来源。
func (s *RetainedGovernance) lockedCurrent(ctx context.Context, tx *sql.Tx, operation string) (domain.RetainedGovernancePlan, error) {
	original, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
	if err != nil {
		return original.Plan, err
	}
	plan := original.Plan
	p := plan.Purge
	scopes := append([]domain.PurgeScope(nil), p.Scopes...)
	if p.KnowledgeBase != 0 {
		scopes = append(scopes, domain.PurgeScope{Key: "kb-acl:" + domain.TextID(p.KnowledgeBase), Epoch: plan.KnowledgeBaseACL})
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Key < scopes[j].Key })
	for _, source := range scopes {
		gate, err := repository.LockSourceGate(ctx, tx, source.Key, source.Epoch)
		if err != nil {
			return plan, err
		}
		state := "BLOCKED"
		if source.Operation == "" {
			state = "OPEN"
		}
		if gate.Epoch != source.Epoch || gate.State != state {
			return plan, domain.Violation(domain.StateConflict, "RETAINED_SOURCE_CHANGED")
		}
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, p.Document, true)
	if err != nil {
		return plan, err
	}
	d := facts.Document
	expectedLifecycle := "PUBLIC_RETAINED"
	if plan.Action == "PURGE" {
		expectedLifecycle = "CLOSING"
	}
	if d.Lifecycle != expectedLifecycle || d.SpaceId != domain.TextID(p.Space) || d.ControlRevision != domain.TextID(p.ClosingControl) ||
		d.AclRevision != domain.TextID(p.ACL) || d.PublicationEpoch != domain.TextID(p.Publication) || d.CurrentReleaseId != nil ||
		d.PublicationState != "UNPUBLISHED" || d.ResponsibleUserId != nil || !d.MainLocked || facts.Sequence != p.Sequence {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_DOCUMENT_CHANGED")
	}
	if p.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, p.KnowledgeBase, true)
		if err != nil {
			return plan, err
		}
````

## 第 7 步：RetainedGovernance.lockedCurrent（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 155 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>d.KnowledgeBaseId == nil &#124;&#124; *d.KnowledgeBaseId != kb.Id &#124;&#124; kb.SpaceId != d.SpaceId &#124;&#124; kb.Lifecycle != &quot;PUBLIC_RETAINED&quot; &#124;&#124; kb.AclRevision != domain.TextID(plan.KnowledgeBaseACL)</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>d.KnowledgeBaseId != nil</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>space.Lifecycle != &quot;PUBLIC_RETAINED&quot;</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot; &#124;&#124; !reflect.DeepEqual(locked.Plan, plan)</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_ORIGINAL_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return plan, domain.Violation(domain.StateConflict, &quot;RETAINED_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5705:6831 -->
````go
		if d.KnowledgeBaseId == nil || *d.KnowledgeBaseId != kb.Id || kb.SpaceId != d.SpaceId || kb.Lifecycle != "PUBLIC_RETAINED" || kb.AclRevision != domain.TextID(plan.KnowledgeBaseACL) {
			return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
		}
	} else if d.KnowledgeBaseId != nil {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, p.Space, true)
	if err != nil {
		return plan, err
	}
	if space.Lifecycle != "PUBLIC_RETAINED" {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_PARENT_CHANGED")
	}
	locked, err := repository.RetainedGovernanceRow(ctx, tx, operation, false)
	if err != nil {
		return plan, err
	}
	if locked.State == "SUCCEEDED" || !reflect.DeepEqual(locked.Plan, plan) {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_ORIGINAL_CHANGED")
	}
	drained, err := repository.RetainedGovernanceDrained(ctx, tx, plan)
	if err != nil {
		return plan, err
	}
	if !drained {
		return plan, domain.Violation(domain.StateConflict, "RETAINED_DRAIN_PENDING")
	}
	return plan, nil
}

````

## 第 8 步：RetainedGovernance.removeHistory

插入位置：第 7 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`func (s *RetainedGovernance) removeHistory(ctx context.Context, task repository.PurgeTask, expected platform.AccountDocumentCleanupContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedCurrent(ctx, tx, task.Operation)</code>；<code>repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance)</code>；<code>repository.RetainedGovernanceCleanupReceipt(ctx, tx, p.Operation)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance)</code> |
| <code>expected</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>expected != platform.NewAccountDocumentCleanupContext(platform.AccountDocumentCleanupRequest{Operation: p.Operation, Document: domain.TextID(p.Document)}, p.ACL, p.Scopes&#91;0&#93;.Operation)</code>；<code>prior != expected</code>；<code>repository.ReleaseDocumentDraftResourcePins(ctx, tx, item, expected)</code>；<code>repository.RemoveAccountPayloads(ctx, tx, expected)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;RETAINED_NODE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;RETAINED_CLEANUP_BINDING_CHANGED&quot;)</code>；<code>nil</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>expected != platform.NewAccountDocumentCleanupContext(platform.AccountDocumentCleanupRequest{Operation: p.Operation, Document: domain.TextID(p.Document)}, p.ACL, p.Scopes&#91;0&#93;.Operation)</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>found</code>。
- 检查 <code>prior != expected</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_EXPORT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=6831:8537 -->
````go
func (s *RetainedGovernance) removeHistory(ctx context.Context, task repository.PurgeTask, expected platform.AccountDocumentCleanupContext) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	p := plan.Purge
	if expected != platform.NewAccountDocumentCleanupContext(platform.AccountDocumentCleanupRequest{Operation: p.Operation, Document: domain.TextID(p.Document)}, p.ACL, p.Scopes[0].Operation) {
		return errors.New("RETAINED_NODE_BINDING_CHANGED")
	}
	if err = repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if prior, found, err := repository.RetainedGovernanceCleanupReceipt(ctx, tx, p.Operation); err != nil {
		return err
	} else if found {
		if prior != expected {
			return errors.New("RETAINED_CLEANUP_BINDING_CHANGED")
		}
		return nil
	}
	drained, err := repository.RetainedExportArtifactsDrained(ctx, tx, p.Document)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "RETAINED_EXPORT_DRAIN_PENDING")
	}
	item := domain.AccountRetentionDocument{Document: p.Document, Space: p.Space, KnowledgeBase: p.KnowledgeBase}
	if err = repository.ReleaseDocumentDraftResourcePins(ctx, tx, item, expected); err != nil {
		return err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return err
	}
	if err = repository.RemoveAccountPayloads(ctx, tx, expected); err != nil {
		return err
	}
	if err = repository.RemoveRetainedExportControls(ctx, tx, p.Document); err != nil {
		return err
	}
````

## 第 9 步：RetainedGovernance.removeHistory（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 231 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_HISTORY_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=8537:9173 -->
````go
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, p.Document); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return err
	}
	if err = repository.ReleaseRetainedResourcePins(ctx, tx, p.Document); err != nil {
		return err
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "RETAINED_GOVERNANCE_CLEANED", p.Operation, p.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_HISTORY_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 10 步：RetainedGovernance.finish

插入位置：第 9 步代码之后，紧接追加；完成稿约第 253 行。

本步声明：`func (s *RetainedGovernance) finish(ctx context.Context, task repository.PurgeTask, expected domain.RetainedGovernancePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedCurrent(ctx, tx, task.Operation)</code>；<code>repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance)</code>；<code>repository.RetainedGovernanceCleanupReceipt(ctx, tx, task.Operation)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance)</code>；<code>repository.FinishPurgeTask(ctx, tx, task, s.instance)</code> |
| <code>expected</code> | <code>domain.RetainedGovernancePlan</code> | 使用 <code>domain.RetainedGovernancePlan</code> 类型；其定义处的职责：平台治理只能关闭孤立的留存对象，不能从这个计划恢复可写空间或选择新正文。 PurgePlan仅复用原号、代次及关闭范围格式；不登记伪造的账号注销计划。<br>在此声明中的实际使用：<code>reflect.DeepEqual(plan, expected)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_PLAN_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RETAINED_HISTORY_CLEANUP_PENDING&quot;)</code>；<code>errors.New(&quot;RETAINED_FINAL_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!reflect.DeepEqual(plan, expected)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.Action == &quot;PURGE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RETAINED_HISTORY_CLEANUP_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;RETAINED_FINAL_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=9173:10555 -->
````go
func (s *RetainedGovernance) finish(ctx context.Context, task repository.PurgeTask, expected domain.RetainedGovernancePlan) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err := s.lockedCurrent(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, expected) {
		return domain.Violation(domain.StateConflict, "RETAINED_PLAN_CHANGED")
	}
	if err = repository.LockRetainedGovernanceOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if plan.Action == "PURGE" {
		if _, found, err := repository.RetainedGovernanceCleanupReceipt(ctx, tx, task.Operation); err != nil {
			return err
		} else if !found {
			return domain.Violation(domain.StateConflict, "RETAINED_HISTORY_CLEANUP_PENDING")
		}
		if err = repository.ValidateRetainedResourceCompletion(ctx, tx, plan); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(retainedGovernanceProgress(task.Operation, "SUCCEEDED", plan.Purge.Document))
	if err != nil {
		return err
	}
	if err = repository.CompleteRetainedGovernance(ctx, tx, plan, raw); err != nil {
		return err
	}
	if err = repository.FinishPurgeTask(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_FINAL_COMMIT_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/retained_governance_recovery.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
