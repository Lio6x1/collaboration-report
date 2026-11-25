# 分步编写 `services/document/internal/logic/document/personal_transfer_recovery.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/218-personal-transfer-recovery.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

恢复已接受转移的关闭、Node计量、额度补偿和最终迁移。

断线后必须续原计划，真实字节要在旧来源关闭后计量。

写之前先按这条执行路线理解各段的连接：

1. claimRecovery短事务领取
2. measureRecovery锁外关闭与计量
3. lockRecoveryInputs按来源→消费者Doc→任务→父级
4. commitRecovery复核并迁移或补偿拒绝。

本文件需要守住的边界：

- 未知结果只重试原号
- 不持SQL锁等Node
- 目标额度不足的补偿与状态同事务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/personal_transfer_recovery.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/personal_transfer_recovery.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/personal_transfer_recovery.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/personal_transfer_recovery.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:349 -->
````go
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"slices"
	"sort"
	"time"
)

````

## 第 2 步：PersonalTransfers.Recover

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

Recover 串起认领、远端关闭/计量、事务落定三个阶段。未知结果始终重试原任务。

本步声明：`func (s *PersonalTransfers) Recover(parent context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code>；<code>context.WithTimeout(parent, time.Second)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PERSONAL_TRANSFER_NOT_READY&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!s.Ready()</code>。条件成立时执行 <code>return errors.New(&quot;PERSONAL_TRANSFER_NOT_READY&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>func() { 		if !complete { 			retryCtx, retryCancel := context.WithTimeout(parent, time.Second) 			defer retryCancel() 			_ = repository.RetryPersonalTransfer(retryCtx, s.db, task, s.instance) 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!complete</code>。
- 在本段的匿名函数内：执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 在本段的匿名函数内：登记 <code>retryCancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=349:1185 -->
````go
// Recover 串起认领、远端关闭/计量、事务落定三个阶段。未知结果始终重试原任务。
func (s *PersonalTransfers) Recover(parent context.Context) error {
	if !s.Ready() {
		return errors.New("PERSONAL_TRANSFER_NOT_READY")
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	task, offer, err := s.claimRecovery(ctx)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			retryCtx, retryCancel := context.WithTimeout(parent, time.Second)
			defer retryCancel()
			_ = repository.RetryPersonalTransfer(retryCtx, s.db, task, s.instance)
		}
	}()
	measured, err := s.measureRecovery(ctx, offer.Plan)
	if err != nil {
		return err
	}
	if err = s.commitRecovery(ctx, task, offer.Plan, measured); err != nil {
		return err
	}
	complete = true
	return nil
}

````

## 第 3 步：PersonalTransfers.claimRecovery

插入位置：第 2 步代码之后，紧接追加；完成稿约第 47 行。

认领单独提交；远端调用不持有这笔短事务的锁，最终落定时再验证原任务代次。

本步声明：`func (s *PersonalTransfers) claimRecovery(ctx context.Context) (repository.PurgeTask, domain.PersonalTransfer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.ClaimPersonalTransfer(ctx, tx, s.instance)</code>；<code>repository.LockPersonalTransferTask(ctx, tx, task, s.instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>实际返回表达式：<code>repository.PurgeTask{}</code>；<code>task</code> |
| <code>(匿名/未命名)</code> | <code>domain.PersonalTransfer</code> | 使用 <code>domain.PersonalTransfer</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>domain.PersonalTransfer{}</code>；<code>offer</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>ErrPersonalTransferIdle</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.PurgeTask{}, domain.PersonalTransfer{}, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return repository.PurgeTask{}, domain.PersonalTransfer{}, ErrPersonalTransferIdle</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.PurgeTask{}, domain.PersonalTransfer{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return task, offer, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1185:2125 -->
````go
// 认领单独提交；远端调用不持有这笔短事务的锁，最终落定时再验证原任务代次。
func (s *PersonalTransfers) claimRecovery(ctx context.Context) (repository.PurgeTask, domain.PersonalTransfer, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	defer tx.Rollback()
	task, err := repository.ClaimPersonalTransfer(ctx, tx, s.instance)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, ErrPersonalTransferIdle
	}
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	offer, err := repository.LockPersonalTransferTask(ctx, tx, task, s.instance)
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	if err = tx.Commit(); err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	return task, offer, nil
}

````

## 第 4 步：PersonalTransfers.measureRecovery

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (s *PersonalTransfers) measureRecovery(ctx context.Context, plan domain.PersonalTransferPlan) (PersonalTransferStorage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fences.Close(ctx, request)</code>；<code>s.storage.MeasurePersonalTransfer(ctx, plan)</code> |
| <code>plan</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>plan.Scopes</code>；<code>s.storage.MeasurePersonalTransfer(ctx, plan)</code>；<code>plan.Operation</code>；<code>plan.Document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>PersonalTransferStorage</code> | 使用 <code>PersonalTransferStorage</code> 类型；其定义处的职责：Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。<br>实际返回表达式：<code>PersonalTransferStorage{}</code>；<code>measured</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_ACK_UNKNOWN&quot;)</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_STORAGE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return PersonalTransferStorage{}, err</code>。当前函数在这里结束。
- 检查 <code>!platform.MatchesBootstrapAck(request, ack, &quot;document&quot;)</code>。条件成立时执行 <code>return PersonalTransferStorage{}, errors.New(&quot;PERSONAL_TRANSFER_ACK_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return PersonalTransferStorage{}, err</code>。当前函数在这里结束。
- 检查 <code>measured.Operation != plan.Operation &#124;&#124; measured.Document != plan.Document &#124;&#124; len(measured.Digest) != 32</code>。条件成立时执行 <code>return PersonalTransferStorage{}, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return measured, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2125:3298 -->
````go
func (s *PersonalTransfers) measureRecovery(ctx context.Context, plan domain.PersonalTransferPlan) (PersonalTransferStorage, error) {
	for _, scope := range plan.Scopes {
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		ack, err := s.fences.Close(ctx, request)
		if err != nil {
			return PersonalTransferStorage{}, err
		}
		if !platform.MatchesBootstrapAck(request, ack, "document") {
			return PersonalTransferStorage{}, errors.New("PERSONAL_TRANSFER_ACK_UNKNOWN")
		}
	}
	// 真实 ACK 后固定 Node 的 checkpoint/update_log 字节。未知计量只能重试原号，
	// 不猜零字节；确定容量不足才执行下面的业务拒绝补偿。
	measured, err := s.storage.MeasurePersonalTransfer(ctx, plan)
	if err != nil {
		return PersonalTransferStorage{}, err
	}
	if measured.Operation != plan.Operation || measured.Document != plan.Document || len(measured.Digest) != 32 {
		return PersonalTransferStorage{}, errors.New("PERSONAL_TRANSFER_STORAGE_INVALID")
	}
	return measured, nil
}

````

## 第 5 步：PersonalTransfers.commitRecovery

插入位置：第 4 步代码之后，紧接追加；完成稿约第 94 行。

下面的复核、配额补偿与最终迁移只由本函数拥有的同一事务落定。

本步声明：`func (s *PersonalTransfers) commitRecovery(ctx context.Context, task repository.PurgeTask, plan domain.PersonalTransferPlan, measured PersonalTransferStorage) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>resources.TransferResourceConsumerDocuments(ctx, s.db, plan.Resources, false)</code>；<code>repository.ActiveTransferTargetACL(ctx, s.db, plan.TargetKB, plan.TargetSpace)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockRecoveryInputs(ctx, tx, task, plan, consumers, targetACL)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>s.lockRecoveryInputs(ctx, tx, task, plan, consumers, targetACL)</code> |
| <code>plan</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>plan.Resources</code>；<code>plan.TargetKB</code>；<code>plan.TargetSpace</code>；<code>plan, err = s.lockRecoveryInputs(ctx, tx, task, plan, consumers, targetACL)</code><br>另有 13 处使用，继续按下方完整代码的顺序处理。 |
| <code>measured</code> | <code>PersonalTransferStorage</code> | 使用 <code>PersonalTransferStorage</code> 类型；其定义处的职责：Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。<br>在此声明中的实际使用：<code>measured.Bytes</code>；<code>measured.Digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_STORAGE_CHANGED&quot;)</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_CONSUMERS_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(plan.NodeMeasurement) &gt; 0</code>。
- 检查 <code>plan.NodeBytes != measured.Bytes &#124;&#124; string(plan.NodeMeasurement) != string(measured.Digest)</code>。条件成立时执行 <code>return errors.New(&quot;PERSONAL_TRANSFER_STORAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!slices.Equal(consumers, currentConsumers)</code>。条件成立时执行 <code>return errors.New(&quot;PERSONAL_TRANSFER_CONSUMERS_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=3298:5028 -->
````go
// 下面的复核、配额补偿与最终迁移只由本函数拥有的同一事务落定。
func (s *PersonalTransfers) commitRecovery(ctx context.Context, task repository.PurgeTask, plan domain.PersonalTransferPlan, measured PersonalTransferStorage) error {
	consumers, err := resources.TransferResourceConsumerDocuments(ctx, s.db, plan.Resources, false)
	if err != nil {
		return err
	}
	// gate 必须先于 KB 行锁。先取候选当前代次，再在下面的 KB 排他锁下核对，
	// 这样已完成的 KB 变更无需依赖某次外部读取才开放新的安全代次。
	targetACL, err := repository.ActiveTransferTargetACL(ctx, s.db, plan.TargetKB, plan.TargetSpace)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err = s.lockRecoveryInputs(ctx, tx, task, plan, consumers, targetACL)
	if err != nil {
		return err
	}
	source, target, err := resources.LockTransferQuotas(ctx, tx, plan.SourceSpace, plan.TargetSpace, s.quotaLimit)
	if err != nil {
		return err
	}
	if len(plan.NodeMeasurement) > 0 {
		if plan.NodeBytes != measured.Bytes || string(plan.NodeMeasurement) != string(measured.Digest) {
			return errors.New("PERSONAL_TRANSFER_STORAGE_CHANGED")
		}
	}
	plan, resourceBytes, err := resources.RefreshTransferResourceCharges(ctx, tx, plan)
	if err != nil {
		return err
	}
	currentConsumers, err := resources.TransferResourceConsumerDocuments(ctx, tx, plan.Resources, true)
	if err != nil {
		return err
	}
	if !slices.Equal(consumers, currentConsumers) {
		return errors.New("PERSONAL_TRANSFER_CONSUMERS_CHANGED")
	}
	required, err := domain.TransferCapacity(plan.BodyBytes, resourceBytes)
	if err != nil {
		return err
	}
````

## 第 6 步：PersonalTransfers.commitRecovery（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 139 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>required &gt; plan.ReservedBytes</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; violation.Code == &quot;QUOTA_EXCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>required &lt; plan.ReservedBytes</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=5028:6572 -->
````go
	required, err = domain.TransferCapacity(required, measured.Bytes)
	if err != nil {
		return err
	}
	if required > plan.ReservedBytes {
		err = resources.ReserveTransferCapacity(ctx, tx, target, required-plan.ReservedBytes, target.Revision)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "QUOTA_EXCEEDED" {
				if err = resources.ReleaseTransferCapacity(ctx, tx, target, plan.ReservedBytes); err != nil {
					return err
				}
				if err = repository.RejectPersonalTransfer(ctx, tx, plan, "QUOTA_EXCEEDED"); err != nil {
					return err
				}
				if err = tx.Commit(); err != nil {
					return err
				}
				return nil
			}
			return err
		}
	} else if required < plan.ReservedBytes {
		if err = resources.ReleaseTransferCapacity(ctx, tx, target, plan.ReservedBytes-required); err != nil {
			return err
		}
	}
	plan.NodeBytes = measured.Bytes
	plan.NodeMeasurement = measured.Digest
	plan.ReservedBytes = required
	// 计量/账目与最终变更使用同一事务；未提交的调整不会泄漏到任务恢复。
	target, err = resources.LockQuota(ctx, tx, plan.TargetSpace, s.quotaLimit)
	if err != nil {
		return err
	}
	released, redundant, err := resources.ApplyTransferResources(ctx, tx, plan)
	if err != nil {
		return err
	}
	released, err = domain.TransferCapacity(released, plan.SourceBodyBytes)
	if err != nil {
		return err
	}
	if err = resources.CommitTransferCapacity(ctx, tx, source, target, plan.ReservedBytes, released, redundant); err != nil {
		return err
	}
````

## 第 7 步：PersonalTransfers.commitRecovery（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 185 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=6572:6828 -->
````go
	if err = repository.CompletePersonalTransfer(ctx, tx, plan); err != nil {
		return err
	}
	if err = repository.SavePersonalTransferMeasurement(ctx, tx, plan); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

````

## 第 8 步：PersonalTransfers.lockRecoveryInputs

插入位置：第 7 步代码之后，紧接追加；完成稿约第 197 行。

锁顺序仍为来源 gate → 所有消费者文档 → 原任务 → KB/父空间；此函数不提交。

本步声明：`func (s *PersonalTransfers) lockRecoveryInputs(ctx context.Context, tx *sql.Tx, task repository.PurgeTask, plan domain.PersonalTransferPlan, consumers []uint64, targetACL uint64) (domain.PersonalTransferPlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockedSourceGate(ctx, tx, scope.Key)</code>；<code>repository.PendingSourceTransition(ctx, tx, targetKey)</code>；<code>repository.ReopenSourceGate(ctx, tx, targetKey, targetACL)</code>；<code>repository.LockTransferConsumerDocuments(ctx, tx, append(append(&#91;&#93;uint64{}, consumers...), plan.Document))</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockedSourceGate(ctx, tx, scope.Key)</code>；<code>repository.PendingSourceTransition(ctx, tx, targetKey)</code>；<code>repository.ReopenSourceGate(ctx, tx, targetKey, targetACL)</code>；<code>repository.LockTransferConsumerDocuments(ctx, tx, append(append(&#91;&#93;uint64{}, consumers...), plan.Document))</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>repository.PurgeTask</code> | 使用 <code>repository.PurgeTask</code> 类型；其定义处的职责：持久永久删除计划、关闭证据、清理阶段与系统到期任务。<br>在此声明中的实际使用：<code>repository.LockPersonalTransferTask(ctx, tx, task, s.instance)</code> |
| <code>plan</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>plan.Scopes</code>；<code>plan.TargetKB</code>；<code>return plan, err</code>；<code>return plan, errors.New(&quot;PERSONAL_TRANSFER_TARGET_SOURCE_PENDING&quot;)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |
| <code>consumers</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>append(&#91;&#93;uint64{}, consumers...)</code> |
| <code>targetACL</code> | <code>uint64</code> | 在此声明中的实际使用：<code>epoch &gt; targetACL</code>；<code>epoch == targetACL</code>；<code>epoch &lt; targetACL</code>；<code>repository.ReopenSourceGate(ctx, tx, targetKey, targetACL)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_TARGET_SOURCE_PENDING&quot;)</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_SOURCE_CHANGED&quot;)</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_DOCUMENT_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return lockScopes&#91;i&#93;.Key &lt; lockScopes&#91;j&#93;.Key</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>scope.Key == targetKey</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>pending &#124;&#124; epoch &gt; targetACL &#124;&#124; (epoch == targetACL &amp;&amp; state != &quot;OPEN&quot;)</code>。条件成立时执行 <code>return plan, errors.New(&quot;PERSONAL_TRANSFER_TARGET_SOURCE_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>epoch &lt; targetACL</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;BLOCKED&quot; &#124;&#124; epoch &lt; scope.Epoch</code>。条件成立时执行 <code>return plan, errors.New(&quot;PERSONAL_TRANSFER_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>doc.Lifecycle != &quot;CLOSING&quot; &#124;&#124; doc.ControlRevision != domain.TextID(plan.ClosingControl) &#124;&#124; doc.SpaceId != domain.TextID(plan.SourceSpace)</code>。条件成立时执行 <code>return plan, errors.New(&quot;PERSONAL_TRANSFER_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=6828:8948 -->
````go
// 锁顺序仍为来源 gate → 所有消费者文档 → 原任务 → KB/父空间；此函数不提交。
func (s *PersonalTransfers) lockRecoveryInputs(ctx context.Context, tx *sql.Tx, task repository.PurgeTask, plan domain.PersonalTransferPlan, consumers []uint64, targetACL uint64) (domain.PersonalTransferPlan, error) {
	// 来源 gate 在 Doc/任务之前排序锁定。后台不重验原 owner 的会话，避免其注销
	// 和已登记迁移互相等待；Account 固定父空间范围必须先等待本任务终态。
	var err error
	scopes := append([]domain.PurgeScope{}, plan.Scopes...)
	targetKey := "kb-acl:" + domain.TextID(plan.TargetKB)
	lockScopes := append(append([]domain.PurgeScope{}, scopes...), domain.PurgeScope{Key: targetKey})
	sort.Slice(lockScopes, func(i, j int) bool { return lockScopes[i].Key < lockScopes[j].Key })
	for _, scope := range lockScopes {
		gate, err := repository.LockedSourceGate(ctx, tx, scope.Key)
		if err != nil {
			return plan, err
		}
		epoch, state := gate.Epoch, gate.State
		if scope.Key == targetKey {
			pending, err := repository.PendingSourceTransition(ctx, tx, targetKey)
			if err != nil {
				return plan, err
			}
			if pending || epoch > targetACL || (epoch == targetACL && state != "OPEN") {
				return plan, errors.New("PERSONAL_TRANSFER_TARGET_SOURCE_PENDING")
			}
			if epoch < targetACL {
				if err = repository.ReopenSourceGate(ctx, tx, targetKey, targetACL); err != nil {
					return plan, err
				}
			}
			continue
		}
		if state != "BLOCKED" || epoch < scope.Epoch {
			return plan, errors.New("PERSONAL_TRANSFER_SOURCE_CHANGED")
		}
	}
	if err := repository.LockTransferConsumerDocuments(ctx, tx, append(append([]uint64{}, consumers...), plan.Document)); err != nil {
		return plan, err
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, plan.Document, true)
	if err != nil {
		return plan, err
	}
	doc := facts.Document
	if doc.Lifecycle != "CLOSING" || doc.ControlRevision != domain.TextID(plan.ClosingControl) || doc.SpaceId != domain.TextID(plan.SourceSpace) {
		return plan, errors.New("PERSONAL_TRANSFER_DOCUMENT_CHANGED")
	}
````

## 第 9 步：PersonalTransfers.lockRecoveryInputs（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 242 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>targetKB.Revision != targetACL</code>。条件成立时执行 <code>return plan, errors.New(&quot;PERSONAL_TRANSFER_TARGET_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=8948:9610 -->
````go
	offer, err := repository.LockPersonalTransferTask(ctx, tx, task, s.instance)
	if err != nil {
		return plan, err
	}
	plan = offer.Plan
	for _, scope := range scopes {
		if err = repository.TransferFenceConfirmed(ctx, tx, scope); err != nil {
			return plan, err
		}
	}
	if err = lockTransferParents(ctx, tx, plan.SourceSpace, plan.TargetSpace, plan.TargetKB, plan.TargetUser); err != nil {
		return plan, err
	}
	targetKB, err := repository.LockedCreationKnowledgeBase(ctx, tx, plan.TargetKB)
	if err != nil {
		return plan, err
	}
	if targetKB.Revision != targetACL {
		return plan, errors.New("PERSONAL_TRANSFER_TARGET_SOURCE_CHANGED")
	}
	return plan, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/personal_transfer_recovery.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
