# 分步编写 `services/document/internal/logic/document/conflicts_service.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/199-conflicts-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

安全展示原合并冲突，幂等物化当时三份正文版本。

冲突页面必须读真实不可变版本，不能只信可修改恢复记录中的摘要。

写之前先按这条执行路线理解各段的连接：

1. conflictOriginal取得原attempt
2. readConflictFixed验固定输入
3. validateConflictVersions核库中实际版本
4. ensureConflictVersions按原操作锁追加必要副本和VERSION资源引用。

本文件需要守住的边界：

- 不得读取当前房间替换原冲突正文，也不得因查看冲突推进主线或审核轮次。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/conflicts_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/conflicts_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/conflicts_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/conflicts_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:574 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sort"
	"time"
)

// conflict_contexts.go：保留原实现，按业务合并文件。

````

## 第 2 步：Reviews.LookupConflict

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func (s *Reviews) LookupConflict(ctx context.Context, operation string) (domain.MergeFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeRow(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>repository.MergeRow(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>实际返回表达式：<code>domain.MergeFact{}</code>；<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;CONFLICT_REF_INVALID&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。条件成立时执行 <code>return domain.MergeFact{}, domain.Violation(domain.InvalidRequest, &quot;CONFLICT_REF_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return fact, domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=574:1004 -->
````go
func (s *Reviews) LookupConflict(ctx context.Context, operation string) (domain.MergeFact, error) {
	if !domain.ValidOperationID(operation) {
		return domain.MergeFact{}, domain.Violation(domain.InvalidRequest, "CONFLICT_REF_INVALID")
	}
	fact, err := repository.MergeRow(ctx, s.db, operation)
	if err == sql.ErrNoRows {
		return fact, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	return fact, err
}

````

## 第 3 步：Reviews.Conflict

插入位置：第 2 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (s *Reviews) Conflict(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_HISTORY&quot;)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, fact.Operation)</code>；<code>LockSources(ctx, tx, access.Sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_HISTORY&quot;)</code> |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Request</code>；<code>fact.Document</code>；<code>fact.Unit</code>；<code>fact.Branch</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>original.ID != attempt.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;CONFLICT_ATTEMPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return readConflictView(ctx, tx, fact, original, fixed, expires)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1004:2755 -->
````go
func (s *Reviews) Conflict(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error) {
	pointer := domain.ReviewPointer{ID: fact.Request, Document: fact.Document, Unit: fact.Unit, Branch: fact.Branch}
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	// 旧冲突尚无HTTP版本引用时，只根据原固定事实建立幂等快照。互斥先于来源锁，与写用例锁序一致。
	// 这一步不输出正文；后面的独立读取事务仍须再次核验当前来源并持有真实输出租约。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, fact.Operation); err != nil {
		return nil, nil, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return nil, nil, err
	}
	if err = s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
		return nil, nil, err
	}
	attempt, fixed, _, err := s.conflictOriginal(ctx, tx, fact)
	if err != nil {
		return nil, nil, err
	}
	if _, err = ensureConflictVersions(ctx, tx, fact, fixed); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		original, fixed, expires, err := s.conflictOriginal(ctx, tx, fact)
		if err != nil {
			return nil, err
		}
		if original.ID != attempt.ID {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_ATTEMPT_CHANGED")
		}
		return readConflictView(ctx, tx, fact, original, fixed, expires)
	}
````

## 第 4 步：Reviews.Conflict（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 79 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2755:2791 -->
````go
	return access.Sources, run, nil
}

````

## 第 5 步：Reviews.conflictOriginal

插入位置：第 4 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func (s *Reviews) conflictOriginal(ctx context.Context, tx *sql.Tx, fact domain.MergeFact) (repository.ConflictAttempt, mergeFixedInputs, time.Time, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeResult(ctx, tx, fact.Operation)</code>；<code>repository.ConflictAttemptRow(ctx, tx, fact.Operation)</code>；<code>readConflictFixed(ctx, tx, s.validator, fact, attempt)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeResult(ctx, tx, fact.Operation)</code>；<code>repository.ConflictAttemptRow(ctx, tx, fact.Operation)</code>；<code>readConflictFixed(ctx, tx, s.validator, fact, attempt)</code> |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Operation</code>；<code>readConflictFixed(ctx, tx, s.validator, fact, attempt)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.ConflictAttempt</code> | 使用 <code>repository.ConflictAttempt</code> 类型；其定义处的职责：读取原合并操作最终冲突attempt及其固定输入。<br>实际返回表达式：<code>repository.ConflictAttempt{}</code>；<code>attempt</code> |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 实际返回表达式：<code>empty</code>；<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>实际返回表达式：<code>time.Time{}</code>；<code>receipt.Expires</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.ConflictAttempt{}, empty, time.Time{}, err</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;REJECTED&quot; &#124;&#124; !receipt.Expires.After(time.Now())</code>。条件成立时执行 <code>return repository.ConflictAttempt{}, empty, time.Time{}, domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return attempt, empty, time.Time{}, err</code>。当前函数在这里结束。
- 检查 <code>attempt.State != &quot;CONFLICT&quot; &#124;&#124; len(attempt.Conflicts) == 0</code>。条件成立时执行 <code>return attempt, empty, time.Time{}, domain.Violation(domain.EntityNotFound, &quot;CONFLICT_CONTEXT_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; !bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint)</code>。
- 此处通过 <code>return attempt, fixed, receipt.Expires, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2791:3937 -->
````go
func (s *Reviews) conflictOriginal(ctx context.Context, tx *sql.Tx, fact domain.MergeFact) (repository.ConflictAttempt, mergeFixedInputs, time.Time, error) {
	var empty mergeFixedInputs
	receipt, err := repository.MergeResult(ctx, tx, fact.Operation)
	if err != nil {
		return repository.ConflictAttempt{}, empty, time.Time{}, err
	}
	if receipt.State != "REJECTED" || !receipt.Expires.After(time.Now()) {
		return repository.ConflictAttempt{}, empty, time.Time{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	attempt, err := repository.ConflictAttemptRow(ctx, tx, fact.Operation)
	if err != nil {
		return attempt, empty, time.Time{}, err
	}
	if attempt.State != "CONFLICT" || len(attempt.Conflicts) == 0 {
		return attempt, empty, time.Time{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	fixed, err := readConflictFixed(ctx, tx, s.validator, fact, attempt)
	if err == nil && !bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint) {
		err = domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	}
	return attempt, fixed, receipt.Expires, err
}

````

## 第 6 步：readConflictView

插入位置：第 5 步代码之后，紧接追加；完成稿约第 105 行。

本步声明：`func readConflictView(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, attempt repository.ConflictAttempt, fixed mergeFixedInputs, expires time.Time) (dto.ConflictContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, attemptKey(fact.Operation, attempt.ID))</code>；<code>validateConflictVersions(ctx, tx, raw, fixed)</code>；<code>repository.ContentRow(ctx, tx, domain.TextID(versions.Base), value.DocumentId)</code>；<code>repository.ContentRow(ctx, tx, domain.TextID(versions.Submitted), value.DocumentId)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, attemptKey(fact.Operation, attempt.ID))</code>；<code>validateConflictVersions(ctx, tx, raw, fixed)</code>；<code>repository.ContentRow(ctx, tx, domain.TextID(versions.Base), value.DocumentId)</code>；<code>repository.ContentRow(ctx, tx, domain.TextID(versions.Submitted), value.DocumentId)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Operation</code>；<code>fact.Document</code>；<code>fact.Unit</code>；<code>fact.Request</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>attempt</code> | <code>repository.ConflictAttempt</code> | 使用 <code>repository.ConflictAttempt</code> 类型；其定义处的职责：读取原合并操作最终冲突attempt及其固定输入。<br>在此声明中的实际使用：<code>attempt.ID</code>；<code>attempt.Digest</code>；<code>attempt.Conflicts</code> |
| <code>fixed</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>validateConflictVersions(ctx, tx, raw, fixed)</code>；<code>fixed.Attempt</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>expires.UTC</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ConflictContext</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;CONFLICT_LIST_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(value.Conflicts) == 0</code>。条件成立时执行 <code>return value, domain.Violation(domain.TemporarilyUnavailable, &quot;CONFLICT_LIST_UNKNOWN&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=3937:6348 -->
````go
func readConflictView(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, attempt repository.ConflictAttempt, fixed mergeFixedInputs, expires time.Time) (dto.ConflictContext, error) {
	// 公开契约使用不透明操作引用；命名空间加完整uint64定宽编码与真实attempt一一对应。
	// 私有固定输入仍使用原数据库主键，不生成新尝试，也不把公开引用写回旧固定事实。
	attemptRef := domain.ConflictAttemptRef(attempt.ID)
	value := dto.ConflictContext{
		ConflictRef: fact.Operation,
		OperationId: fact.Operation,
		AttemptId:   attemptRef,
		DocumentId:  domain.TextID(fact.Document),
		UnitId:      domain.TextID(fact.Unit),
		RequestId:   domain.TextID(fact.Request),
		RoundId:     domain.TextID(fact.Round),
		InputDigest: hex.EncodeToString(attempt.Digest),
		ExpiresAt:   expires.UTC().Format(time.RFC3339Nano),
		Conflicts:   []dto.Conflict{},
	}
	raw, err := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(fact.Operation, attempt.ID))
	if err != nil {
		return value, err
	}
	versions, err := validateConflictVersions(ctx, tx, raw, fixed)
	if err != nil {
		return value, err
	}
	if value.Base, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Base), value.DocumentId); err != nil {
		return value, err
	}
	if value.Submitted, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Submitted), value.DocumentId); err != nil {
		return value, err
	}
	if value.Main, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Main), value.DocumentId); err != nil {
		return value, err
	}
	if value.Structure, err = repository.StructureRow(ctx, tx, domain.TextID(fixed.Attempt.Main.ExpectedStructureId), value.DocumentId); err != nil {
		return value, err
	}
	if value.Style, err = repository.StyleRow(ctx, tx, domain.TextID(fixed.Attempt.Main.ExpectedStyleId), value.DocumentId); err != nil {
		return value, err
	}
	if err = json.Unmarshal(attempt.Conflicts, &value.Conflicts); err != nil || len(value.Conflicts) == 0 {
		return value, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_LIST_UNKNOWN")
	}
	// 私有计算结果按固定数组保存，尚无公共冲突引用。用真实attempt与原数组序号建立稳定引用；
	// 后续修复必须在同一attempt/digest下核对整组引用，不能跨尝试沿用序号或改变旧计算事实。
	for index := range value.Conflicts {
````

## 第 7 步：readConflictView（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 150 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=6348:6514 -->
````go
		value.Conflicts[index].Id = domain.ConflictItemRef(attempt.ID, index+1)
	}
	return value, nil
}

// conflict_fixed.go：保留原实现，按业务合并文件。

````

## 第 8 步：readConflictFixed

插入位置：第 7 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func readConflictFixed(ctx context.Context, tx *sql.Tx, validator *codec.Validator, fact domain.MergeFact, attempt repository.ConflictAttempt) (mergeFixedInputs, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key)</code>；<code>repository.MergeRoundInputs(ctx, tx, fact.Request, fact.Round)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_RANGE_FIXED&quot;, key)</code>；<code>repository.MergeArtifact(ctx, tx, &quot;MERGE_ATTEMPT_FIXED&quot;, key)</code>；<code>repository.MergeRoundInputs(ctx, tx, fact.Request, fact.Round)</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validateFixedMergeContent(validator, content, fact.Document, fact.Unit, target.StructureId)</code> |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Operation</code>；<code>fact.Actor</code>；<code>fact.Document</code>；<code>fact.Unit</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>attempt</code> | <code>repository.ConflictAttempt</code> | 使用 <code>repository.ConflictAttempt</code> 类型；其定义处的职责：读取原合并操作最终冲突attempt及其固定输入。<br>在此声明中的实际使用：<code>attempt.ID</code>；<code>attempt.Digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mergeFixedInputs</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Input == nil &#124;&#124; fixed.Input.Context == nil &#124;&#124; fixed.Input.Context.Target == nil &#124;&#124; fixed.Attempt.Main == nil &#124;&#124; fixed.Main == nil &#124;&#124; fixed.Main.Cut == nil</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>fixed.Attempt.ID != attempt.ID &#124;&#124; input.AttemptId != domain.TextID(attempt.ID) &#124;&#124; input.Context.OperationId != fact.Operation &#124;&#124; input.Context.ActorId != fact.Actor &#124;&#124; target.Docum …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(digest&#91;:&#93;, input.InputDigest) &#124;&#124; !bytes.Equal(digest&#91;:&#93;, attempt.Digest)</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !reflect.DeepEqual(round, fixed.Attempt.Round) &#124;&#124; round.SubmittedVersion != fact.Version</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>main != nil &amp;&amp; main.UnitId == fact.Unit &amp;&amp; proto.Equal(main, input.Main)</code>。
- 检查 <code>!matched</code>。条件成立时执行 <code>return fixed, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6514:8755 -->
````go
func readConflictFixed(ctx context.Context, tx *sql.Tx, validator *codec.Validator, fact domain.MergeFact, attempt repository.ConflictAttempt) (mergeFixedInputs, error) {
	var fixed mergeFixedInputs
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	key := attemptKey(fact.Operation, attempt.ID)
	raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key)
	if err == sql.ErrNoRows {
		raw, err = repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key)
	}
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Input == nil || fixed.Input.Context == nil || fixed.Input.Context.Target == nil || fixed.Attempt.Main == nil || fixed.Main == nil || fixed.Main.Cut == nil {
		return fixed, unknown
	}
	input, target := fixed.Input, fixed.Input.Context.Target
	if fixed.Attempt.ID != attempt.ID || input.AttemptId != domain.TextID(attempt.ID) || input.Context.OperationId != fact.Operation || input.Context.ActorId != fact.Actor || target.DocumentId != fact.Document || target.UnitId != fact.Unit || target.BranchId != fact.Branch || target.Purpose != "MERGE" || !proto.Equal(input.MainCut, fixed.Main.Cut) {
		return fixed, unknown
	}
	digest, err := domain.MergeInputDigest(input)
	if err != nil || !bytes.Equal(digest[:], input.InputDigest) || !bytes.Equal(digest[:], attempt.Digest) {
		return fixed, unknown
	}
	round, err := repository.MergeRoundInputs(ctx, tx, fact.Request, fact.Round)
	if err != nil || !reflect.DeepEqual(round, fixed.Attempt.Round) || round.SubmittedVersion != fact.Version {
		return fixed, unknown
	}
	if err = validateMainCaptureDigest(fixed.Main); err != nil {
		return fixed, err
	}
	matched := false
	for _, main := range fixed.Main.Units {
		if main != nil && main.UnitId == fact.Unit && proto.Equal(main, input.Main) {
			matched = true
		}
	}
	if !matched {
		return fixed, unknown
	}
	for _, content := range []*pb.FixedContent{input.Base, input.Submitted, input.Main} {
		if err = validateFixedMergeContent(validator, content, fact.Document, fact.Unit, target.StructureId); err != nil {
			return fixed, err
		}
	}
	return fixed, nil
}

// conflict_version_binding.go：保留原实现，按业务合并文件。

````

## 第 9 步：validateConflictVersions

插入位置：第 8 步代码之后，紧接追加；完成稿约第 205 行。

恢复记录只是索引，真实不可变版本才是正文来源。不能仅凭两个可写记录中的摘要相同放行。
调用方传入当前事务；三份原型来自已经验证真实attempt/digest/轮次的固定计算输入。

本步声明：`func validateConflictVersions(ctx context.Context, q repository.Queryer, raw []byte, fixed mergeFixedInputs) (conflictVersions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, q, part.version, part.fixed.DocumentId)</code> |
| <code>q</code> | <code>repository.Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, q, part.version, part.fixed.DocumentId)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(raw, &amp;value)</code> |
| <code>fixed</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>fixed.Attempt</code>；<code>fixed.Input</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>conflictVersions</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(raw, &amp;value) != nil &#124;&#124; value.Attempt != fixed.Attempt.ID &#124;&#124; !bytes.Equal(value.Digest, fixed.Input.InputDigest) &#124;&#124; value.Base == 0 &#124;&#124; value.Submitted == 0 &#124;&#124; value.Main == 0</code>。条件成立时执行 <code>return value, unknown</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return value, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !matched</code>。条件成立时执行 <code>return value, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=8755:10021 -->
````go
// 恢复记录只是索引，真实不可变版本才是正文来源。不能仅凭两个可写记录中的摘要相同放行。
// 调用方传入当前事务；三份原型来自已经验证真实attempt/digest/轮次的固定计算输入。
func validateConflictVersions(ctx context.Context, q repository.Queryer, raw []byte, fixed mergeFixedInputs) (conflictVersions, error) {
	var value conflictVersions
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_VERSIONS_UNKNOWN")
	if json.Unmarshal(raw, &value) != nil || value.Attempt != fixed.Attempt.ID || !bytes.Equal(value.Digest, fixed.Input.InputDigest) || value.Base == 0 || value.Submitted == 0 || value.Main == 0 {
		return value, unknown
	}
	for _, part := range []struct {
		version uint64
		fixed   *pb.FixedContent
	}{
		{value.Base, fixed.Input.Base},
		{value.Submitted, fixed.Input.Submitted},
		{value.Main, fixed.Input.Main},
	} {
		stored, err := repository.VersionContent(ctx, q, part.version, part.fixed.DocumentId)
		if err == sql.ErrNoRows {
			return value, unknown
		}
		if err != nil {
			return value, err
		}
		matched, err := conflictVersionMatches(stored, part.fixed, part.version)
		if err != nil || !matched {
			return value, unknown
		}
	}
	return value, nil
}

````

## 第 10 步：conflictVersionMatches

插入位置：第 9 步代码之后，紧接追加；完成稿约第 236 行。

本步声明：`func conflictVersionMatches(stored, fixed *pb.FixedContent, version uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>stored</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>canonicalVersion(stored, version)</code>；<code>proto.Equal(stored, expected)</code> |
| <code>fixed</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>proto.Clone(fixed)</code>；<code>fixed.CaptureId</code> |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>canonicalVersion(stored, version)</code>；<code>domain.TextID(version)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>proto.Equal(stored, expected)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return proto.Equal(stored, expected), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=10021:10817 -->
````go
func conflictVersionMatches(stored, fixed *pb.FixedContent, version uint64) (bool, error) {
	if err := canonicalVersion(stored, version); err != nil {
		return false, err
	}
	// VERSION的引用切点与房间采样切点属于不同传输表示。只规范这三个字段，
	// 原正文/hash/字节数/schema/谱系/来源操作与引用/结构/完整资源集仍逐项精确比较。
	expected := proto.Clone(fixed).(*pb.FixedContent)
	expected.BranchId, expected.WorkspaceEpoch = 0, 0
	cut, err := codec.Canonical(map[string]any{"content_version_id": domain.TextID(version), "source_ref": fixed.CaptureId})
	if err != nil {
		return false, err
	}
	expected.CutJsonUtf8 = cut
	return proto.Equal(stored, expected), nil
}

// conflict_versions.go：保留原实现，按业务合并文件。

````

## 第 11 步：conflictVersions

插入位置：第 10 步代码之后，紧接追加；完成稿约第 254 行。

本步声明：`type conflictVersions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Attempt</code> | <code>uint64</code> | 发生冲突的合并尝试编号。 |
| <code>Base</code> | <code>uint64</code> | 三方合并的共同基线版本。 |
| <code>Submitted</code> | <code>uint64</code> | 本次送审固定版本。 |
| <code>Main</code> | <code>uint64</code> | 该尝试采样的主线固定版本。 |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 上述固定合并输入的摘要，用来绑定解决方案。 |

<!-- write-step: 11 bytes=10817:10929 -->
````go
type conflictVersions struct {
	Attempt, Base, Submitted, Main uint64
	Digest                         []byte
}

````

## 第 12 步：ensureConflictVersions

插入位置：第 11 步代码之后，紧接追加；完成稿约第 259 行。

把旧冲突的原固定事实幂等物化；不读取当前房间，不改主线、轮次或共同草稿。
调用方先持有原操作互斥及当前来源/文档锁，同一事务保存VERSION引用和版本绑定。

本步声明：`func ensureConflictVersions(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, fixed mergeFixedInputs) (conflictVersions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, key)</code>；<code>validateConflictVersions(ctx, tx, raw, fixed)</code>；<code>repository.DocumentRow(ctx, tx, fact.Document, true)</code>；<code>repository.LockedDocumentResource(ctx, tx, id, fact.Document, space)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, key)</code>；<code>validateConflictVersions(ctx, tx, raw, fixed)</code>；<code>repository.DocumentRow(ctx, tx, fact.Document, true)</code>；<code>repository.LockedDocumentResource(ctx, tx, id, fact.Document, space)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>fact</code> | <code>domain.MergeFact</code> | 使用 <code>domain.MergeFact</code> 类型；其定义处的职责：表达固定合并、冲突修复、丢弃与反向变更意图及跨语言摘要。<br>在此声明中的实际使用：<code>fact.Operation</code>；<code>fact.Document</code> |
| <code>fixed</code> | <code>mergeFixedInputs</code> | 在此声明中的实际使用：<code>fixed.Attempt</code>；<code>fixed.Input</code>；<code>validateConflictVersions(ctx, tx, raw, fixed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>conflictVersions</code> | 由 <code>validateConflictVersions(ctx, tx, raw, fixed)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>validateConflictVersions(ctx, tx, raw, fixed)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>repository.InsertSubmissionTask(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, key, fact.Operation, raw, &quot;SUCCEEDED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return validateConflictVersions(ctx, tx, raw, fixed)</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return value, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。

<!-- write-step: 12 bytes=10929:12796 -->
````go
// 把旧冲突的原固定事实幂等物化；不读取当前房间，不改主线、轮次或共同草稿。
// 调用方先持有原操作互斥及当前来源/文档锁，同一事务保存VERSION引用和版本绑定。
func ensureConflictVersions(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, fixed mergeFixedInputs) (conflictVersions, error) {
	value := conflictVersions{Attempt: fixed.Attempt.ID, Digest: fixed.Input.InputDigest}
	key := attemptKey(fact.Operation, value.Attempt)
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_VERSIONS_UNKNOWN")
	if raw, err := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", key); err == nil {
		return validateConflictVersions(ctx, tx, raw, fixed)
	} else if err != sql.ErrNoRows {
		return value, err
	}
	document, err := repository.DocumentRow(ctx, tx, fact.Document, true)
	if err != nil {
		return value, err
	}
	space, err := domain.CanonicalID(document.SpaceId)
	if err != nil {
		return value, err
	}
	set := map[uint64]bool{}
	for _, part := range []*pb.FixedContent{fixed.Input.Base, fixed.Input.Submitted, fixed.Input.Main} {
		for _, resource := range part.ResourceIds {
			set[resource] = true
		}
	}
	ids := make([]uint64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, err := repository.LockedDocumentResource(ctx, tx, id, fact.Document, space)
		if err != nil {
			return value, err
		}
		if state != "READY" {
			return value, unknown
		}
	}
	value.Base, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Base, fixed.Attempt.Round.BaseVersion, "RESTORE")
	if err != nil {
		return value, err
	}
	value.Submitted, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Submitted, fixed.Attempt.Round.SubmittedVersion, "RESTORE")
	if err != nil {
````

## 第 13 步：ensureConflictVersions（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 304 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, repository.InsertSubmissionTask(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, key, fact.Operation, raw, &quot;SUCCEEDED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=12796:13148 -->
````go
		return value, err
	}
	value.Main, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Main, 0, "MAIN_CAPTURE")
	if err != nil {
		return value, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	return value, repository.InsertSubmissionTask(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", key, fact.Operation, raw, "SUCCEEDED")
}

````

## 第 14 步：conflictSnapshotVersion

插入位置：第 13 步代码之后，紧接追加；完成稿约第 317 行。

本步声明：`func conflictSnapshotVersion(ctx context.Context, tx *sql.Tx, fixed *pb.FixedContent, original uint64, kind string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, original, fixed.DocumentId)</code>；<code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, original, fixed.DocumentId)</code>；<code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> |
| <code>fixed</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>fixed.DocumentId</code>；<code>conflictVersionMatches(stored, fixed, original)</code>；<code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code>；<code>fixed.CaptureId</code> |
| <code>original</code> | <code>uint64</code> | 在此声明中的实际使用：<code>original != 0</code>；<code>repository.VersionContent(ctx, tx, original, fixed.DocumentId)</code>；<code>conflictVersionMatches(stored, fixed, original)</code>；<code>return original, nil</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code>；<code>original</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>original != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>matched</code>。条件成立时执行 <code>return original, nil</code>。当前函数在这里结束。
- 此处通过 <code>return repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=13148:13783 -->
````go
func conflictSnapshotVersion(ctx context.Context, tx *sql.Tx, fixed *pb.FixedContent, original uint64, kind string) (uint64, error) {
	if original != 0 {
		stored, err := repository.VersionContent(ctx, tx, original, fixed.DocumentId)
		if err != nil {
			return 0, err
		}
		matched, err := conflictVersionMatches(stored, fixed, original)
		if err != nil {
			return 0, err
		}
		if matched {
			return original, nil
		}
	}
	// 稳定范围的结构投影或可靠MAIN尾部必须保存独立真实快照，不能把旧正式版本号贴到新正文上。
	return repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/conflicts_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
