# 分步编写 `services/document/internal/logic/document/account_cleanup_service.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/190-account-cleanup-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排注销的关系解除、Node清理资格、资源接续、历史与payload删除。

各阶段有不同事实边界，CLEANING只说明关系阶段已提交，不代表全部私有副本已删除。

写之前先按这条执行路线理解各段的连接：

1. DocumentCleanupContext从原冻结计划发证
2. ApplyRelations关用户关系
3. ReleaseDraftResources锁外取CLEANED后同事务接正式pin
4. CleanHistory删关系图
5. CleanPayloads删剩余私有副本。

本文件需要守住的边界：

- 不在SQL锁内再次查询Account
- 跨库成功本库提交未知必须原号恢复。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/account_cleanup_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/account_cleanup_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/account_cleanup_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/account_cleanup_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:418 -->
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

// account_document_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetentions.DocumentCleanupContext

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
的Account冻结，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。

本步声明：`func (s *AccountRetentions) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=418:1402 -->
````go
// DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
// 的Account冻结，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。
func (s *AccountRetentions) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	result, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	// 返回给Node之前先结束只读事务，SQL错误不能被成功HTTP掩盖。
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}

````

## 第 3 步：AccountRetentions.lockedDocumentCleanupContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 40 行。

可被后续资源/历史清理复用；调用者拥有事务，不在此处提交或跨服务通信。

本步声明：`func (s *AccountRetentions) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, tx, request.Operation)</code>；<code>lockAccountRetentionSources(ctx, tx, sources, plan.Binding)</code>；<code>repository.LockedAccountRetirement(ctx, tx, request.Operation)</code>；<code>repository.AccountRetentionDrained(ctx, tx, current)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, tx, request.Operation)</code>；<code>lockAccountRetentionSources(ctx, tx, sources, plan.Binding)</code>；<code>repository.LockedAccountRetirement(ctx, tx, request.Operation)</code>；<code>repository.AccountRetentionDrained(ctx, tx, current)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Document</code>；<code>request.Operation</code>；<code>platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANING_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_DRAIN_PENDING&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.Binding.Operation != request.Operation</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; original.State != &quot;CLEANING&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 此处通过 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANING_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>locked.State != &quot;CLEANING&quot; &#124;&#124; string(locked.Plan) != string(original.Plan)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=1402:3141 -->
````go
// 可被后续资源/历史清理复用；调用者拥有事务，不在此处提交或跨服务通信。
func (s *AccountRetentions) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	document, err := domain.CanonicalID(request.Document)
	if err != nil {
		return empty, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(original.Plan)
	if err != nil {
		return empty, err
	}
	if plan.Binding.Operation != request.Operation {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	plan, err = boundRetentionPlan(original, plan.Binding)
	if err != nil || original.State != "CLEANING" {
		if err != nil {
			return empty, err
		}
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANING_REQUIRED")
	}
	item, scopes, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	sources := make([]domain.LeaseSource, 0, len(scopes))
	for _, scope := range scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	if err = lockAccountRetentionSources(ctx, tx, sources, plan.Binding); err != nil {
		return empty, err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	current, err := boundRetentionPlan(locked, plan.Binding)
	if err != nil {
		return empty, err
	}
	if locked.State != "CLEANING" || string(locked.Plan) != string(original.Plan) {
````

## 第 4 步：AccountRetentions.lockedDocumentCleanupContext（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 85 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_WRITES_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Key == &quot;doc-acl:&quot;+request.Document</code>。
- 检查 <code>!result.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_SCOPE_MISSING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3141:4527 -->
````go
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	// 只核验这个文档的doc-acl/publication/父KB屏障，其他文档的未知操作不能
	// 使本次单文档资格变成全局排队。每个屏障仍验证原任务、gate及双方真实ACK摘要。
	retention := *current.Retention
	retention.Scopes = scopes
	current.Retention = &retention
	drained, err := repository.AccountRetentionDrained(ctx, tx, current)
	if err != nil {
		return empty, err
	}
	if !drained {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_DRAIN_PENDING")
	}
	if err = repository.LockAccountCleanupDocument(ctx, tx, current, item, scopes); err != nil {
		return empty, err
	}
	pending, err := repository.AccountDocumentWritesPending(ctx, tx, document)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_WRITES_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document {
			result := platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)
			if !result.Valid() {
				return empty, errors.New("ACCOUNT_DOCUMENT_CONTEXT_INVALID")
			}
			return result, nil
		}
	}
	return empty, errors.New("ACCOUNT_DOCUMENT_SCOPE_MISSING")
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

````

## 第 5 步：AccountRetentions.CleanHistory

插入位置：第 4 步代码之后，紧接追加；完成稿约第 123 行。

此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。

本步声明：`func (s *AccountRetentions) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountHistoryCleanupReceipt(ctx, s.db, request)</code>；<code>s.ReleaseDraftResources(ctx, request)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>repository.AccountHistoryCleanupReceipt(ctx, s.db, request)</code>；<code>s.ReleaseDraftResources(ctx, request)</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_NODE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_HISTORY_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=4527:6223 -->
````go
// 此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
// 原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。
func (s *AccountRetentions) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.ReleaseDraftResources(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_HISTORY_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil {
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return empty, err
	}
````

## 第 6 步：AccountRetentions.CleanHistory（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 169 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=6223:6894 -->
````go
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, document); err != nil {
		return empty, err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_HISTORY_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// account_payload_cleanup.go：保留原实现，按业务合并文件。

````

## 第 7 步：AccountRetentions.CleanPayloads

插入位置：第 6 步代码之后，紧接追加；完成稿约第 190 行。

原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。

本步声明：`func (s *AccountRetentions) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountPayloadCleanupReceipt(ctx, s.db, request)</code>；<code>s.CleanHistory(ctx, request)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, s.db, request)</code>；<code>s.CleanHistory(ctx, request)</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=6894:8664 -->
````go
// 原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
// 原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。
func (s *AccountRetentions) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.CleanHistory(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	if err = repository.RemoveAccountPayloads(ctx, tx, authority); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_PAYLOAD_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// account_relations.go：保留原实现，按业务合并文件。

````

## 第 8 步：AccountRetentions.ApplyRelations

插入位置：第 7 步代码之后，紧接追加；完成稿约第 237 行。

ApplyRelations只关闭这个已冻结用户的关系，存续文档的正文、主线、审核、正式
历史全部保留。CLEANING表示关系阶段已提交，仍不表示Node/正文/引用清理完成。

本步声明：`func (s *AccountRetentions) ApplyRelations(ctx context.Context, binding domain.AccountRetirementBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)</code>；<code>repository.DiscoverAccountRelations(ctx, tx, binding.User)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>platform.AccountClosureBinding(binding)</code>；<code>binding.Operation</code>；<code>boundRetentionPlan(original, binding)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETENTION_AUTHORITY_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_FREEZE_UNCONFIRMED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid() &#124;&#124; s.authority == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_AUTHORITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!confirmed</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_FREEZE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;CLEANING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;PLANNED&quot; &#124;&#124; plan.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!slices.ContainsFunc(sources, func(s domain.LeaseSource) bool { return s.Key == scope.Key })</code>。
- 在本段的匿名函数内：此处通过 <code>return s.Key == scope.Key</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=8664:10441 -->
````go
// ApplyRelations只关闭这个已冻结用户的关系，存续文档的正文、主线、审核、正式
// 历史全部保留。CLEANING表示关系阶段已提交，仍不表示Node/正文/引用清理完成。
func (s *AccountRetentions) ApplyRelations(ctx context.Context, binding domain.AccountRetirementBinding) error {
	if !binding.Valid() || s.authority == nil {
		return errors.New("ACCOUNT_RETENTION_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, platform.AccountClosureBinding(binding))
	if err != nil {
		return err
	}
	if !confirmed {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_FREEZE_UNCONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	original, err := repository.AccountRetirementSnapshot(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundRetentionPlan(original, binding)
	if err != nil {
		return err
	}
	if plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	if original.State == "CLEANING" {
		return nil
	}
	if original.State != "PLANNED" || plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	candidates, err := repository.DiscoverAccountRelations(ctx, tx, binding.User)
	if err != nil {
		return err
	}
	// 包含固定保留来源，确保核验旧排空时其他调用不能更换gate代次或原任务绑定。
	sources := slices.Clone(candidates.Sources)
	for _, scope := range plan.Retention.Scopes {
		if !slices.ContainsFunc(sources, func(s domain.LeaseSource) bool { return s.Key == scope.Key }) {
			sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
		}
	}
````

## 第 9 步：AccountRetentions.ApplyRelations（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 283 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;CLEANING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;PLANNED&quot; &#124;&#124; plan.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!slices.Equal(candidates.Spaces, current.Spaces) &#124;&#124; !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) &#124;&#124; !slices.Equal(candidates.Documents, current.Documents) &#124;&#124; !s …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RELATIONS_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RELATIONS_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=10441:11997 -->
````go
	if err = lockAccountRetentionSources(ctx, tx, sources, binding); err != nil {
		return err
	}
	original, err = repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	plan, err = boundRetentionPlan(original, binding)
	if err != nil {
		return err
	}
	if plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	if original.State == "CLEANING" {
		return nil
	}
	if original.State != "PLANNED" || plan.Retention == nil {
		return errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	drained, err := repository.AccountRetentionDrained(ctx, tx, plan)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_DRAIN_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	current, err := repository.DiscoverAccountRelations(ctx, tx, binding.User)
	if err != nil {
		return err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RELATIONS_CHANGED")
	}
	if err = repository.ApplyAccountRelations(ctx, tx, plan, current); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RELATIONS_COMMIT_UNKNOWN")
	}
	return nil
}

// account_resource_cleanup.go：保留原实现，按业务合并文件。

````

## 第 10 步：AccountRetentions.ReleaseDraftResources

插入位置：第 9 步代码之后，紧接追加；完成稿约第 331 行。

ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。

本步声明：`func (s *AccountRetentions) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountResourceCleanupReceipt(ctx, s.db, request)</code>；<code>s.DocumentCleanupContext(ctx, request)</code>；<code>s.cleaner.Clean(ctx, expected)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>repository.AccountResourceCleanupReceipt(ctx, s.db, request)</code>；<code>s.DocumentCleanupContext(ctx, request)</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANER_REQUIRED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid() &#124;&#124; s.cleaner == nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。条件成立时执行 <code>return prior, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=11997:13693 -->
````go
// ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
// 接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。
func (s *AccountRetentions) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() || s.cleaner == nil {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANER_REQUIRED")
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, s.db, request); err != nil || found {
		return prior, err
	}
	expected, err := s.DocumentCleanupContext(ctx, request)
	if err != nil {
		return empty, err
	}
	if err = s.cleaner.Clean(ctx, expected); err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, tx, request); err != nil || found {
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeAccountRetirementPlan(record.Plan)
	if err != nil {
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
````

## 第 11 步：AccountRetentions.ReleaseDraftResources（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 376 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=13693:13940 -->
````go
	if err = repository.ReleaseAccountDraftResources(ctx, tx, item, authority); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/account_cleanup_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
