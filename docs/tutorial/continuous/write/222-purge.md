# 分步编写 `services/document/internal/logic/document/purge.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/222-purge.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将用户确认永久删除固定成异步清理计划。

跨服务清理不应挤进短HTTP期限，入口只登记不可逆范围。

写之前先按这条执行路线理解各段的连接：

1. Prepare核TRASHED/confirmed/revision
2. plan固定Doc及两条关闭来源
3. registerPurge写生命周期、任务、回执和撤发布事件。

本文件需要守住的边界：

- 不关闭父KB或同库其他Doc
- 原号使用标准Replay/PresentedMutation。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/purge.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/purge.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/purge.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/purge.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:294 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"
)

````

## 第 2 步：ErrPurgeIdle

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`var ErrPurgeIdle = errors.New("PURGE_IDLE")`。

<!-- write-step: 2 bytes=294:339 -->
````go
var ErrPurgeIdle = errors.New("PURGE_IDLE")

````

## 第 3 步：Purges

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type Purges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>trash</code> | <code>*Trash</code> | 使用 <code>Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。 |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |
| <code>expiryCursor</code> | <code>atomic.Uint64</code> | 到期回收扫描的原子游标，用于分批推进扫描，不作为永久清理结果。 |

<!-- write-step: 3 bytes=339:492 -->
````go
type Purges struct {
	db           *sql.DB
	trash        *Trash
	cleaner      AccountDocumentCleaner
	instance     string
	expiryCursor atomic.Uint64
}

````

## 第 4 步：NewPurges

插入位置：第 3 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func NewPurges(db *sql.DB, trash *Trash, cleaner AccountDocumentCleaner, instance string) *Purges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>trash</code> | <code>*Trash</code> | 使用 <code>Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。<br>在此声明中的实际使用：<code>trash: trash</code> |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>cleaner: cleaner</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Purges</code> | 实际返回表达式：<code>&amp;Purges{db: db, trash: trash, cleaner: cleaner, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Purges{db: db, trash: trash, cleaner: cleaner, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=492:671 -->
````go
func NewPurges(db *sql.DB, trash *Trash, cleaner AccountDocumentCleaner, instance string) *Purges {
	return &Purges{db: db, trash: trash, cleaner: cleaner, instance: instance}
}

````

## 第 5 步：Purges.Prepare

插入位置：第 4 步代码之后，紧接追加；完成稿约第 29 行。

POST只登记不可逆固定清理任务，不在三秒HTTP内执行Node/物理清理。handler必须
使用标准Replay/PresentedMutation，原号fingerprint与输出生命周期仍由它负责。

本步声明：`func (s *Purges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.PurgeAction) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PurgeRow(ctx, s.db, input.OperationId, false)</code>；<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.trash.spaceAuthority(ctx, actor, spaceID, &quot;PURGE_DOCUMENT&quot;, id)</code>；<code>s.trash.documentAuthority(ctx, s.db, actor, space, facts, false)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>actor.Sources</code>；<code>actor.Proof</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>original.Plan.Document != id</code>；<code>repository.TrashDocumentRow(ctx, s.db, id, false)</code>；<code>s.trash.spaceAuthority(ctx, actor, spaceID, &quot;PURGE_DOCUMENT&quot;, id)</code>；<code>purgeProgress(plan.Operation, &quot;PREPARING&quot;, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.PurgeAction</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Confirmed</code>；<code>input.ExpectedControlRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>original.Plan.Actor != actor.ID &#124;&#124; original.Plan.Document != id &#124;&#124; original.Plan.Automatic</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, nil, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>access.KB != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=671:2631 -->
````go
// POST只登记不可逆固定清理任务，不在三秒HTTP内执行Node/物理清理。handler必须
// 使用标准Replay/PresentedMutation，原号fingerprint与输出生命周期仍由它负责。
func (s *Purges) Prepare(ctx context.Context, actor Actor, id uint64, input dto.PurgeAction) (Actor, MutationBody, MutationPresentation, error) {
	if original, err := repository.PurgeRow(ctx, s.db, input.OperationId, false); err == nil {
		if original.Plan.Actor != actor.ID || original.Plan.Document != id || original.Plan.Automatic {
			return actor, nil, nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		actor.Sources = actor.Proof.Sources
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, nil, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return actor, nil, nil, err
	}
	facts, err := repository.TrashDocumentRow(ctx, s.db, id, false)
	if err != nil {
		return actor, nil, nil, err
	}
	spaceID, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return actor, nil, nil, err
	}
	space, err := s.trash.spaceAuthority(ctx, actor, spaceID, "PURGE_DOCUMENT", id)
	if err != nil {
		return actor, nil, nil, err
	}
	access, err := s.trash.documentAuthority(ctx, s.db, actor, space, facts, false)
	if err != nil {
		return actor, nil, nil, err
	}
	if err := domain.ValidatePurgeIntent(facts, input.Confirmed, input.ExpectedControlRevision, false); err != nil {
		return actor, nil, nil, err
	}
	plan, err := s.plan(ctx, facts, input.OperationId, actor.ID, false)
	if err != nil {
		return actor, nil, nil, err
	}
	sources, err := trashSources(actor, space, access, plan.ACL, plan.Publication)
	if err != nil {
		return actor, nil, nil, err
	}
	var closedKB uint64
	if access.KB != nil {
		if err = domain.ValidatePurgeParent(*access.KB); err != nil {
			return actor, nil, nil, err
		}
````

## 第 6 步：Purges.Prepare（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 75 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>access.KB.Lifecycle == &quot;CLOSED&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>currentKB != nil &amp;&amp; currentKB.Lifecycle != access.KB.Lifecycle</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PURGE_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Document.PublicationEpoch != facts.Document.PublicationEpoch &#124;&#124; current.Document.VisibilityRevision != facts.Document.VisibilityRevision &#124;&#124; current.Sequence != facts.Sequence</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;PURGE_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return purgeProgress(plan.Operation, &quot;PREPARING&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!executed</code>。条件成立时执行 <code>return value, actor.Proof.Sources, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Document.AclRevision != domain.TextID(plan.ACL) &#124;&#124; current.Document.PublicationEpoch != domain.TextID(plan.Publication)</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;PURGE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, actor.Proof.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2631:4672 -->
````go
		if access.KB.Lifecycle == "CLOSED" {
			closedKB = plan.KnowledgeBase
		}
	}
	// 明确非nil空列表让标准Mutation不提前持identity/session锁；run负责一次
	// 排序全部真实来源。最小receipt输出仍由标准Outputs.Register锁身份来源。
	actor.Sources = []*pb.SourceEpoch{}
	executed := false
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := lockPurgeSources(ctx, tx, sources, closedKB); err != nil {
			return nil, err
		}
		current, currentKB, err := s.trash.validate(ctx, tx, actor, space, access)
		if err != nil {
			return nil, err
		}
		if currentKB != nil && currentKB.Lifecycle != access.KB.Lifecycle {
			return nil, domain.Violation(domain.StateConflict, "PURGE_PARENT_CHANGED")
		}
		if err = domain.ValidatePurgeIntent(current, input.Confirmed, input.ExpectedControlRevision, false); err != nil {
			return nil, err
		}
		if current.Document.PublicationEpoch != facts.Document.PublicationEpoch || current.Document.VisibilityRevision != facts.Document.VisibilityRevision || current.Sequence != facts.Sequence {
			return nil, domain.Violation(domain.StateConflict, "PURGE_DOCUMENT_CHANGED")
		}
		if err = registerPurge(ctx, tx, plan); err != nil {
			return nil, err
		}
		executed = true
		return purgeProgress(plan.Operation, "PREPARING", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		// 同号竞态跳过run时仅输出标准fingerprint核验过的原最小receipt，
		// 没有拿next-epoch开门，也不把原动作重新当成新权限。
		if !executed {
			return value, actor.Proof.Sources, nil
		}
		current, err := repository.TrashDocumentRow(ctx, tx, id, true)
		if err != nil {
			return nil, nil, err
		}
		if current.Document.AclRevision != domain.TextID(plan.ACL) || current.Document.PublicationEpoch != domain.TextID(plan.Publication) {
			return nil, nil, domain.Violation(domain.StateConflict, "PURGE_SOURCE_CHANGED")
		}
		return value, actor.Proof.Sources, nil
	}
````

## 第 7 步：Purges.Prepare（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 121 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4672:4708 -->
````go
	return actor, run, present, nil
}

````

## 第 8 步：Purges.plan

插入位置：第 7 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`func (s *Purges) plan(ctx context.Context, facts domain.TrashFacts, operation string, actor uint64, automatic bool) (domain.PurgePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TrashSourceHighWater(ctx, s.db, &quot;doc-acl:&quot;+facts.Document.Id)</code>；<code>repository.TrashSourceHighWater(ctx, s.db, &quot;publication:&quot;+facts.Document.Id)</code> |
| <code>facts</code> | <code>domain.TrashFacts</code> | 使用 <code>domain.TrashFacts</code> 类型；其定义处的职责：TrashFacts保留实际生命周期及数据库时间；普通DocumentRow仍只接受ACTIVE。 恢复判断不使用进程时钟猜测清理期限，也不读取或重写固定正文。<br>在此声明中的实际使用：<code>facts.Document</code>；<code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> |
| <code>automatic</code> | <code>bool</code> | 在此声明中的实际使用：<code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>由 <code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>domain.PurgePlan{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PurgePlan{}, err</code>。当前函数在这里结束。
- 检查 <code>facts.Document.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.PurgePlan{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4708:5413 -->
````go
func (s *Purges) plan(ctx context.Context, facts domain.TrashFacts, operation string, actor uint64, automatic bool) (domain.PurgePlan, error) {
	acl, err := repository.TrashSourceHighWater(ctx, s.db, "doc-acl:"+facts.Document.Id)
	if err != nil {
		return domain.PurgePlan{}, err
	}
	publication, err := repository.TrashSourceHighWater(ctx, s.db, "publication:"+facts.Document.Id)
	if err != nil {
		return domain.PurgePlan{}, err
	}
	var kb uint64
	if facts.Document.KnowledgeBaseId != nil {
		kb, err = domain.CanonicalID(*facts.Document.KnowledgeBaseId)
		if err != nil {
			return domain.PurgePlan{}, err
		}
	}
	return domain.NewPurgePlan(operation, actor, automatic, facts, kb, acl, publication)
}

````

## 第 9 步：registerPurge

插入位置：第 8 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`func registerPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.RegisterPurge(ctx, tx, plan)</code>；<code>repository.BeginPurgeDocument(ctx, tx, plan)</code>；<code>QueueClosure(ctx, tx, scope.Operation, plan.Actor, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: plan.Document, Action: &quot;PURGE_DOCUMENT&quot;})</code>；<code>repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, &quot;DOCUMENT_UNPUBLISHED&quot;, payload)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.RegisterPurge(ctx, tx, plan)</code>；<code>repository.BeginPurgeDocument(ctx, tx, plan)</code>；<code>QueueClosure(ctx, tx, scope.Operation, plan.Actor, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: plan.Document, Action: &quot;PURGE_DOCUMENT&quot;})</code>；<code>repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, &quot;DOCUMENT_UNPUBLISHED&quot;, payload)</code> |
| <code>plan</code> | <code>domain.PurgePlan</code> | 使用 <code>domain.PurgePlan</code> 类型；其定义处的职责：普通永久删除只固定一个Doc和两条自己的关闭来源，不关闭父KB/空间，也不借用 Account的冻结/保留计划。计划不保存正文，后续阶段只能恢复这个原号。<br>在此声明中的实际使用：<code>repository.RegisterPurge(ctx, tx, plan)</code>；<code>repository.BeginPurgeDocument(ctx, tx, plan)</code>；<code>plan.Scopes</code>；<code>plan.Actor</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, &quot;DOCUMENT_UNPUBLISHED&quot;, payload)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, &quot;DOCUMENT_UNPUBLISHED&quot;, payload)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5413:6287 -->
````go
func registerPurge(ctx context.Context, tx *sql.Tx, plan domain.PurgePlan) error {
	if err := repository.RegisterPurge(ctx, tx, plan); err != nil {
		return err
	}
	if err := repository.BeginPurgeDocument(ctx, tx, plan); err != nil {
		return err
	}
	for _, scope := range plan.Scopes {
		if err := QueueClosure(ctx, tx, scope.Operation, plan.Actor, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Entity: plan.Document, Action: "PURGE_DOCUMENT"}); err != nil {
			return err
		}
	}
	payload, err := domain.EncodeIndexEvent(domain.IndexEvent{ID: plan.Operation, Type: "DOCUMENT_UNPUBLISHED", Document: plan.Document, Sequence: plan.Sequence, PublicationEpoch: plan.Publication, OccurredAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return repository.InsertIndexEvent(ctx, tx, plan.Document, plan.Sequence, plan.Operation, "DOCUMENT_UNPUBLISHED", payload)
}

````

## 第 10 步：purgeProgress

插入位置：第 9 步代码之后，紧接追加；完成稿约第 162 行。

本步声明：`func purgeProgress(operation, state string, document uint64) map[string]any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code>；<code>&quot;/v1/operations/&quot; + operation</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code>；<code>state != &quot;SUCCEEDED&quot;</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>OperationResult(operation, state, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>value</code> |

**跟着本段代码走一遍：**

- 检查 <code>state != &quot;SUCCEEDED&quot;</code>。
- 此处通过 <code>return value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6287:6517 -->
````go
func purgeProgress(operation, state string, document uint64) map[string]any {
	value := OperationResult(operation, state, document)
	if state != "SUCCEEDED" {
		value["poll_url"] = "/v1/operations/" + operation
	}
	return value
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/purge.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
