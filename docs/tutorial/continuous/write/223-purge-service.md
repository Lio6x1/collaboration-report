# 分步编写 `services/document/internal/logic/document/purge_service.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/223-purge-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

驱动到期回收、真实关闭、Node清理和最终永久删除。

租约仅授予提交者资格，Node CLEANED与输出ACK才是实际停止证明。

写之前先按这条执行路线理解各段的连接：

1. RegisterExpired每轮至多16篇按DB时间登记
2. Recover领取后锁外清理
3. DocumentCleanupContext从原plan与ACK发证
4. 最终核Node回执释放容量。

本文件需要守住的边界：

- CLOSED父KB不重开来源
- 单个竞争候选不能阻塞批次
- 未知提交续稳定原号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/purge_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/purge_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/purge_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/purge_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:504 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
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

// purge_context.go：保留原实现，按业务合并文件。

````

## 第 2 步：Purges.Handles

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (s *Purges) Handles(ctx context.Context, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LifecycleKind(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.LifecycleKind(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>kind == &quot;PURGE&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return kind == &quot;PURGE&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=504:736 -->
````go
func (s *Purges) Handles(ctx context.Context, operation string) (bool, error) {
	kind, err := repository.LifecycleKind(ctx, s.db, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return kind == "PURGE", err
}

````

## 第 3 步：Purges.DocumentCleanupContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

与Account使用同一真实单Doc清理协议，但资格只来自本PURGE固定计划/原屏障。
authority HTTP在只读事务提交成功后才返回，且不拿TTL当清理或停止输出的证明。

本步声明：`func (s *Purges) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedCleanupContext(ctx, tx, request)</code>；<code>repository.SetPurgeCleaning(ctx, tx, request.Operation)</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.lockedCleanupContext(ctx, tx, request)</code>；<code>request.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;PURGE_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;PURGE_CONTEXT_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, domain.Violation(domain.InvalidRequest, &quot;PURGE_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;PURGE_CONTEXT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=736:1744 -->
````go
// 与Account使用同一真实单Doc清理协议，但资格只来自本PURGE固定计划/原屏障。
// authority HTTP在只读事务提交成功后才返回，且不拿TTL当清理或停止输出的证明。
func (s *Purges) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "PURGE_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	_, authority, err := s.lockedCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if err = repository.SetPurgeCleaning(ctx, tx, request.Operation); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("PURGE_CONTEXT_COMMIT_UNKNOWN")
	}
	return authority, nil
}

````

## 第 4 步：Purges.lockedCleanupContext

插入位置：第 3 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func (s *Purges) lockedCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (domain.PurgePlan, platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PurgeRow(ctx, tx, request.Operation, false)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, false)</code>；<code>lockPurgeSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source}, closedKB)</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.PurgeRow(ctx, tx, request.Operation, false)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, false)</code>；<code>lockPurgeSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source}, closedKB)</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Operation</code>；<code>request.Document</code>；<code>platform.NewAccountDocumentCleanupContext(request, plan.ACL, plan.Scopes&#91;0&#93;.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>实际返回表达式：<code>domain.PurgePlan{}</code>；<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>platform.NewAccountDocumentCleanupContext(request, plan.ACL, plan.Scopes&#91;0&#93;.Operation)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_CLEANING_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_BINDING_CHANGED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PurgePlan{}, empty, err</code>。当前函数在这里结束。
- 检查 <code>domain.TextID(plan.Document) != request.Document &#124;&#124; (original.State != &quot;PREPARING&quot; &amp;&amp; original.State != &quot;CLEANING&quot;)</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_CLEANING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.ScopeKey &lt; sources&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>plan.KnowledgeBase != 0 &amp;&amp; source.ScopeKey == &quot;kb-acl:&quot;+domain.TextID(plan.KnowledgeBase)</code>。
- 检查 <code>kbLifecycle == &quot;CLOSED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=1744:3812 -->
````go
func (s *Purges) lockedCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (domain.PurgePlan, platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	original, err := repository.PurgeRow(ctx, tx, request.Operation, false)
	if err != nil {
		return domain.PurgePlan{}, empty, err
	}
	plan := original.Plan
	if domain.TextID(plan.Document) != request.Document || (original.State != "PREPARING" && original.State != "CLEANING") {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_CLEANING_REQUIRED")
	}
	// 父KB不被这个动作关闭。它可合法换管理员或可见性，每次取现今KB ACL后
	// 同gate串行再核当前读，不把旧父revision固定成永远无法恢复的清理资格。
	var kbRevision uint64
	var kbLifecycle string
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, false)
		if err != nil {
			return plan, empty, err
		}
		kbRevision, err = domain.CanonicalID(kb.AclRevision)
		if err != nil {
			return plan, empty, err
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return plan, empty, err
		}
		kbLifecycle = kb.Lifecycle
	}
	sources := []*pb.SourceEpoch{{ScopeKey: plan.Scopes[0].Key, Epoch: plan.ACL}, {ScopeKey: plan.Scopes[1].Key, Epoch: plan.Publication}}
	if plan.KnowledgeBase != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(plan.KnowledgeBase), Epoch: kbRevision})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
	for _, source := range sources {
		if plan.KnowledgeBase != 0 && source.ScopeKey == "kb-acl:"+domain.TextID(plan.KnowledgeBase) {
			var closedKB uint64
			if kbLifecycle == "CLOSED" {
				closedKB = plan.KnowledgeBase
			}
			if err := lockPurgeSources(ctx, tx, []*pb.SourceEpoch{source}, closedKB); err != nil {
				return plan, empty, err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
````

## 第 5 步：Purges.lockedCleanupContext（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 99 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch &gt; source.Epoch</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>locked.State != &quot;PREPARING&quot; &amp;&amp; locked.State != &quot;CLEANING&quot;</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_CLEANING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!reflect.DeepEqual(locked.Plan, plan)</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>kb.SpaceId != domain.TextID(plan.Space) &#124;&#124; kb.AclRevision != domain.TextID(kbRevision) &#124;&#124; kb.Lifecycle != kbLifecycle</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>d.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>d.Lifecycle != &quot;CLOSING&quot; &#124;&#124; d.SpaceId != domain.TextID(plan.Space) &#124;&#124; currentKB != plan.KnowledgeBase &#124;&#124; 		d.ControlRevision != domain.TextID(plan.ClosingControl) &#124;&#124; d.AclRevision  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3812:5609 -->
````go
		if err != nil {
			return plan, empty, err
		}
		if gate.Epoch > source.Epoch {
			return plan, empty, domain.Violation(domain.StateConflict, "PURGE_SOURCE_CHANGED")
		}
	}
	locked, err := repository.PurgeRow(ctx, tx, request.Operation, true)
	if err != nil {
		return plan, empty, err
	}
	if locked.State != "PREPARING" && locked.State != "CLEANING" {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_CLEANING_REQUIRED")
	}
	if !reflect.DeepEqual(locked.Plan, plan) {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_BINDING_CHANGED")
	}
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, true)
		if err != nil {
			return plan, empty, err
		}
		if kb.SpaceId != domain.TextID(plan.Space) || kb.AclRevision != domain.TextID(kbRevision) || kb.Lifecycle != kbLifecycle {
			return plan, empty, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, plan.Document, true)
	if err != nil {
		return plan, empty, err
	}
	d := facts.Document
	var currentKB uint64
	if d.KnowledgeBaseId != nil {
		currentKB, err = domain.CanonicalID(*d.KnowledgeBaseId)
		if err != nil {
			return plan, empty, err
		}
	}
	if d.Lifecycle != "CLOSING" || d.SpaceId != domain.TextID(plan.Space) || currentKB != plan.KnowledgeBase ||
		d.ControlRevision != domain.TextID(plan.ClosingControl) || d.AclRevision != domain.TextID(plan.ACL) || d.PublicationEpoch != domain.TextID(plan.Publication) || d.CurrentReleaseId != nil || facts.Sequence != plan.Sequence {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, plan.Space, true)
	if err != nil {
		return plan, empty, err
	}
````

## 第 6 步：Purges.lockedCleanupContext（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 145 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>space.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, empty, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return plan, empty, domain.Violation(domain.StateConflict, &quot;PURGE_WRITES_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, platform.NewAccountDocumentCleanupContext(request, plan.ACL, plan.Scopes&#91;0&#93;.Operation), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=5609:6347 -->
````go
	if space.Lifecycle != "ACTIVE" {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
	}
	drained, err := repository.PurgeDrained(ctx, tx, plan)
	if err != nil {
		return plan, empty, err
	}
	if !drained {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_DRAIN_PENDING")
	}
	pending, err := repository.DocumentWritesPending(ctx, tx, plan.Document, plan.Operation)
	if err != nil {
		return plan, empty, err
	}
	if pending {
		return plan, empty, domain.Violation(domain.StateConflict, "PURGE_WRITES_PENDING")
	}
	return plan, platform.NewAccountDocumentCleanupContext(request, plan.ACL, plan.Scopes[0].Operation), nil
}

// purge_expiry.go：保留原实现，按业务合并文件。

````

## 第 7 步：Purges.RegisterExpired

插入位置：第 6 步代码之后，紧接追加；完成稿约第 167 行。

每轮固定最多16篇，分别用原事务登记；一个刚恢复/父级关闭的候选不能堵住
同轮其它到期项。竞争失败保留TRASHED，下一轮重新读DB时钟及同一稳定原号。

本步声明：`func (s *Purges) RegisterExpired(ctx context.Context) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ExpiredTrashCandidates(ctx, s.db, s.expiryCursor.Load())</code>；<code>ctx.Err</code>；<code>s.registerExpired(ctx, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>0</code>；<code>registered</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>ctx.Err()</code>；<code>first</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>len(ids) == 0</code>。
- 此处通过 <code>return 0, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return registered, ctx.Err()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>first == nil</code>。
- 此处通过 <code>return registered, first</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=6347:7329 -->
````go
// 每轮固定最多16篇，分别用原事务登记；一个刚恢复/父级关闭的候选不能堵住
// 同轮其它到期项。竞争失败保留TRASHED，下一轮重新读DB时钟及同一稳定原号。
func (s *Purges) RegisterExpired(ctx context.Context) (int, error) {
	ids, err := repository.ExpiredTrashCandidates(ctx, s.db, s.expiryCursor.Load())
	if err != nil {
		return 0, err
	}
	// 仅保留一个扫描编号，不保存无限候选/正文；永久等待的前16项不能饿死
	// 后续到期项。走到尾部后下一轮从头核验，重启也从头读真实到期事实。
	if len(ids) == 0 {
		s.expiryCursor.Store(0)
		return 0, nil
	}
	s.expiryCursor.Store(ids[len(ids)-1])
	registered := 0
	var first error
	for _, id := range ids {
		if ctx.Err() != nil {
			return registered, ctx.Err()
		}
		if err := s.registerExpired(ctx, id); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		registered++
	}
	return registered, first
}

````

## 第 8 步：Purges.registerExpired

插入位置：第 7 步代码之后，紧接追加；完成稿约第 198 行。

本步声明：`func (s *Purges) registerExpired(ctx context.Context, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.plan(ctx, facts, operation, 0, true)</code>；<code>repository.TrashKnowledgeBaseRow(ctx, s.db, plan.KnowledgeBase, false)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>original.Plan.Document != id</code>；<code>repository.TrashDocumentRow(ctx, tx, id, true)</code>；<code>domain.TextID(id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>；<code>errors.New(&quot;PURGE_EXPIRY_BINDING_CHANGED&quot;)</code>；<code>nil</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>kb.SpaceId != facts.Document.SpaceId</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>kb.Lifecycle == &quot;CLOSED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>!original.Plan.Automatic &#124;&#124; original.Plan.Document != id</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_EXPIRY_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=7329:8783 -->
````go
func (s *Purges) registerExpired(ctx context.Context, id uint64) error {
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return err
	}
	if err = domain.ValidatePurgeIntent(facts, true, facts.Document.ControlRevision, true); err != nil {
		return err
	}
	operation := domain.PurgeExpiryOperation(facts)
	plan, err := s.plan(ctx, facts, operation, 0, true)
	if err != nil {
		return err
	}
	var kbRevision uint64
	var closedKB uint64
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, s.db, plan.KnowledgeBase, false)
		if err != nil {
			return err
		}
		if kb.SpaceId != facts.Document.SpaceId {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return err
		}
		if kb.Lifecycle == "CLOSED" {
			closedKB = plan.KnowledgeBase
		}
		kbRevision, err = domain.CanonicalRevision(kb.AclRevision)
		if err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, operation); err != nil {
		return err
	}
	// 必须在任何预期next-epoch重开之前认原号，防止重试产生未发生的高水位。
	if original, err := repository.PurgeRow(ctx, tx, operation, false); err == nil {
		if !original.Plan.Automatic || original.Plan.Document != id {
````

## 第 9 步：Purges.registerExpired（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 243 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;PURGE_EXPIRY_BINDING_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>kb.SpaceId != facts.Document.SpaceId &#124;&#124; kb.AclRevision != domain.TextID(kbRevision) &#124;&#124; (kb.Lifecycle == &quot;CLOSED&quot;) != (closedKB != 0)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.Document.SpaceId != facts.Document.SpaceId &#124;&#124; !sameTrashID(current.Document.KnowledgeBaseId, facts.Document.KnowledgeBaseId) &#124;&#124; 		current.Document.AclRevision != facts.Docu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>space.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SPACE_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=8783:10725 -->
````go
			return errors.New("PURGE_EXPIRY_BINDING_CHANGED")
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sources := []*pb.SourceEpoch{{ScopeKey: plan.Scopes[0].Key, Epoch: plan.ACL}, {ScopeKey: plan.Scopes[1].Key, Epoch: plan.Publication}}
	if plan.KnowledgeBase != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + domain.TextID(plan.KnowledgeBase), Epoch: kbRevision})
	}
	if err = lockPurgeSources(ctx, tx, sources, closedKB); err != nil {
		return err
	}
	if plan.KnowledgeBase != 0 {
		kb, err := repository.TrashKnowledgeBaseRow(ctx, tx, plan.KnowledgeBase, true)
		if err != nil {
			return err
		}
		if kb.SpaceId != facts.Document.SpaceId || kb.AclRevision != domain.TextID(kbRevision) || (kb.Lifecycle == "CLOSED") != (closedKB != 0) {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeParent(kb); err != nil {
			return err
		}
	}
	current, err := repository.TrashDocumentRow(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if err = domain.ValidatePurgeIntent(current, true, facts.Document.ControlRevision, true); err != nil {
		return err
	}
	if current.Document.SpaceId != facts.Document.SpaceId || !sameTrashID(current.Document.KnowledgeBaseId, facts.Document.KnowledgeBaseId) ||
		current.Document.AclRevision != facts.Document.AclRevision || current.Document.PublicationEpoch != facts.Document.PublicationEpoch ||
		current.Document.VisibilityRevision != facts.Document.VisibilityRevision || current.Sequence != facts.Sequence || domain.PurgeExpiryOperation(current) != operation {
		return domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
	}
	space, err := repository.TrashSpaceRow(ctx, tx, plan.Space, true)
	if err != nil {
		return err
	}
	if space.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "SPACE_NOT_ACTIVE")
	}
	if err = registerPurge(ctx, tx, plan); err != nil {
````

## 第 10 步：Purges.registerExpired（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 288 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_EXPIRY_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=10725:11390 -->
````go
		return err
	}
	input := dto.PurgeAction{OperationId: operation, ExpectedControlRevision: facts.Document.ControlRevision, Confirmed: true}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+domain.TextID(id)+"/purge", input)
	if err != nil {
		return err
	}
	result, err := json.Marshal(purgeProgress(operation, "PREPARING", id))
	if err != nil {
		return err
	}
	if err = repository.RegisterPurgeExpiryReceipt(ctx, tx, plan, fingerprint, result); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_EXPIRY_COMMIT_UNKNOWN")
	}
	return nil
}

// purge_recovery.go：保留原实现，按业务合并文件。

````

## 第 11 步：Purges.Recover

插入位置：第 10 步代码之后，紧接追加；完成稿约第 310 行。

每次只领取一个原任务。网络调用在事务外；原Node回执和Doc最终提交都可重送，
lease只决定谁能提交，不能证明正文、输出或安装已经停止。

本步声明：`func (s *Purges) Recover(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.ClaimPurge(ctx, tx, s.instance)</code>；<code>s.recoverTask(ctx, task)</code>；<code>context.WithoutCancel(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PURGE_DEPENDENCIES_REQUIRED&quot;)</code>；<code>err</code>；<code>ErrPurgeIdle</code>；<code>errors.New(&quot;PURGE_CLAIM_COMMIT_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.cleaner == nil &#124;&#124; s.instance == &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_DEPENDENCIES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return ErrPurgeIdle</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_CLAIM_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(context.WithoutCancel(ctx), time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=11390:12428 -->
````go
// 每次只领取一个原任务。网络调用在事务外；原Node回执和Doc最终提交都可重送，
// lease只决定谁能提交，不能证明正文、输出或安装已经停止。
func (s *Purges) Recover(ctx context.Context) error {
	if s.cleaner == nil || s.instance == "" {
		return errors.New("PURGE_DEPENDENCIES_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	task, err := repository.ClaimPurge(ctx, tx, s.instance)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPurgeIdle
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_CLAIM_COMMIT_UNKNOWN")
	}
	if err = s.recoverTask(ctx, task); err != nil {
		// 原提交可能已经成功：只CAS仍属本worker的RUNNING，绝不覆盖已SUCCEEDED。
		retry, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = repository.RetryPurge(retry, s.db, task, s.instance)
	}
	return err
}

````

## 第 12 步：Purges.recoverTask

插入位置：第 11 步代码之后，紧接追加；完成稿约第 340 行。

本步声明：`func (s *Purges) recoverTask(ctx context.Context, task repository.PurgeTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PurgeRow(ctx, s.db, task.Operation, false)</code>；<code>s.DocumentCleanupContext(ctx, request)</code>；<code>s.cleaner.Clean(ctx, expected)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 11 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.LockPurgeOwnership(ctx, tx, task, s.instance)</code>；<code>repository.FinishPurgeTask(ctx, tx, task, s.instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PURGE_NODE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;PURGE_FINAL_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=12428:14058 -->
````go
func (s *Purges) recoverTask(ctx context.Context, task repository.PurgeTask) error {
	original, err := repository.PurgeRow(ctx, s.db, task.Operation, false)
	if err != nil {
		return err
	}
	request := platform.AccountDocumentCleanupRequest{Operation: task.Operation, Document: domain.TextID(original.Plan.Document)}
	expected, err := s.DocumentCleanupContext(ctx, request)
	if err != nil {
		return err
	}
	if err = s.cleaner.Clean(ctx, expected); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, authority, err := s.lockedCleanupContext(ctx, tx, request)
	if err != nil {
		return err
	}
	if authority != expected {
		return errors.New("PURGE_NODE_BINDING_CHANGED")
	}
	// 所有恢复器都先来源/lifecycle/业务事实，再任务ownership，不能持任务锁等gate。
	if err = repository.LockPurgeOwnership(ctx, tx, task, s.instance); err != nil {
		return err
	}
	item := domain.AccountRetentionDocument{Document: plan.Document, Space: plan.Space, KnowledgeBase: plan.KnowledgeBase}
	if err = repository.ReleaseDocumentDraftResourcePins(ctx, tx, item, authority); err != nil {
		return err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return err
	}
	// 先清固定任务副本再删其关联图，避免graph删admission等归属后漏掉正文副本。
	if err = repository.RemoveAccountPayloads(ctx, tx, authority); err != nil {
		return err
	}
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, plan.Document); err != nil {
		return err
	}
````

## 第 13 步：Purges.recoverTask（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 385 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_FINAL_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=14058:14985 -->
````go
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "PURGE_DOCUMENT_CLEANUP", authority.Document+":"+authority.Operation, authority.Operation, raw, "SUCCEEDED"); err != nil {
		return err
	}
	if err = repository.CompletePurgeDocument(ctx, tx, plan); err != nil {
		return err
	}
	result, err := json.Marshal(purgeProgress(plan.Operation, "SUCCEEDED", plan.Document))
	if err != nil {
		return err
	}
	if err = repository.CompletePurgeReceipt(ctx, tx, plan, result); err != nil {
		return err
	}
	if err = repository.FinishPurgeTask(ctx, tx, task, s.instance); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("PURGE_FINAL_COMMIT_UNKNOWN")
	}
	return nil
}

// purge_sources.go：保留原实现，按业务合并文件。

````

## 第 14 步：lockPurgeSources

插入位置：第 13 步代码之后，紧接追加；完成稿约第 416 行。

永久清理不为CLOSED父KB创建新正文输出资格。闭合父来源保持原BLOCKED，
只有其精确原关闭/ACK已经终结才能接受；其他来源仍采用普通准入规则。
合并真实身份证明后一次字典序锁门，不能先持session再倒序拿doc/KB。

本步声明：`func lockPurgeSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, closedKB uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source})</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.ClosedPurgeParentDrained(ctx, tx, closedKB, source.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, &#91;&#93;*pb.SourceEpoch{source})</code>；<code>repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)</code>；<code>repository.PendingSourceTransition(ctx, tx, source.ScopeKey)</code>；<code>repository.ClosedPurgeParentDrained(ctx, tx, closedKB, source.Epoch)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>append(&#91;&#93;*pb.SourceEpoch(nil), sources...)</code> |
| <code>closedKB</code> | <code>uint64</code> | 在此声明中的实际使用：<code>closedKB == 0</code>；<code>domain.TextID(closedKB)</code>；<code>repository.ClosedPurgeParentDrained(ctx, tx, closedKB, source.Epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PURGE_SOURCES_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_DRAIN_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>source == nil &#124;&#124; source.Epoch == 0 &#124;&#124; seen&#91;source.ScopeKey&#93;</code>。条件成立时执行 <code>return errors.New(&quot;PURGE_SOURCES_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93;.ScopeKey &lt; ordered&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>closedKB == 0 &#124;&#124; source.ScopeKey != &quot;kb-acl:&quot;+domain.TextID(closedKB)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>gate.Epoch != source.Epoch &#124;&#124; gate.State != &quot;BLOCKED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14985:16683 -->
````go
// 永久清理不为CLOSED父KB创建新正文输出资格。闭合父来源保持原BLOCKED，
// 只有其精确原关闭/ACK已经终结才能接受；其他来源仍采用普通准入规则。
// 合并真实身份证明后一次字典序锁门，不能先持session再倒序拿doc/KB。
func lockPurgeSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, closedKB uint64) error {
	ordered := append([]*pb.SourceEpoch(nil), sources...)
	seen := map[string]bool{}
	for _, source := range ordered {
		if source == nil || source.Epoch == 0 || seen[source.ScopeKey] {
			return errors.New("PURGE_SOURCES_INVALID")
		}
		seen[source.ScopeKey] = true
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ScopeKey < ordered[j].ScopeKey })
	for _, source := range ordered {
		if closedKB == 0 || source.ScopeKey != "kb-acl:"+domain.TextID(closedKB) {
			if err := LockSources(ctx, tx, []*pb.SourceEpoch{source}); err != nil {
				return err
			}
			continue
		}
		gate, err := repository.LockSourceGate(ctx, tx, source.ScopeKey, source.Epoch)
		if err != nil {
			return err
		}
		if gate.Epoch != source.Epoch || gate.State != "BLOCKED" {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_SOURCE_CHANGED")
		}
		pending, err := repository.PendingSourceTransition(ctx, tx, source.ScopeKey)
		if err != nil {
			return err
		}
		if pending {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_PENDING")
		}
		drained, err := repository.ClosedPurgeParentDrained(ctx, tx, closedKB, source.Epoch)
		if err != nil {
			return err
		}
		if !drained {
			return domain.Violation(domain.StateConflict, "PURGE_PARENT_DRAIN_PENDING")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/purge_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
