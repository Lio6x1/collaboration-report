# 分步编写 `services/account/internal/logic/account/account_close_completion_service.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/057-account-close-completion-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

逐阶段恢复账号注销、撤销Doc预留、读取最小进度并最终收尾。

一个大账号可能有大量文档，每次领取只推进一阶段或一篇，避免同步HTTP和任务预算被占满。

写之前先按这条执行路线理解各段的连接：

1. RecoverOne重读原user/receipt/account_close与任务ownership；advance按PREPARING/FROZEN/PLANNED/CLEANING阶段操作Doc。
2. 只有原PAYLOADS_REMOVED才推进AfterDocument，最后一页SUCCEEDED且全部来源ACK完整才能finalize。
3. Resolve/Progress对冻结原会话仅返回该操作最小计划。

本文件需要守住的边界：

- 网络未知维持原号重试；不得跳页或从新计划继续旧游标。
- 取消任务只释放匹配预留，不可把REJECTED父操作改成新成功操作。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/account_close_completion_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/account_close_completion_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/account_close_completion_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/account_close_completion_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:488 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"
)

// account_close_cancel.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountCloseCancelPeer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type AccountCloseCancelPeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Cancel</code> | <code>(context.Context, platform.AccountClosureBinding) error</code> | 接收ctx和原账号关闭绑定，通知Doc取消尚可取消的准备；只有收到明确结果才能落定取消。 |

<!-- write-step: 2 bytes=488:594 -->
````go
type AccountCloseCancelPeer interface {
	Cancel(context.Context, platform.AccountClosureBinding) error
}

````

## 第 3 步：AccountCloseCancellations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`type AccountCloseCancellations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.AccountCloseCancellations</code> | 使用 <code>repository.AccountCloseCancellations</code> 类型；其定义处的职责：保存注销取消、来源ACK汇总、最终个人资料清理和自动恢复任务。 |
| <code>peer</code> | <code>AccountCloseCancelPeer</code> | Doc取消准备接口，失败继续原号恢复，不能直接宣称已取消。 |

<!-- write-step: 3 bytes=594:742 -->
````go
type AccountCloseCancellations struct {
	db         *sql.DB
	repository *repository.AccountCloseCancellations
	peer       AccountCloseCancelPeer
}

````

## 第 4 步：NewAccountCloseCancellations

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func NewAccountCloseCancellations(db *sql.DB, peer AccountCloseCancelPeer) *AccountCloseCancellations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code>；<code>repository.NewAccountCloseCancellations(db)</code> |
| <code>peer</code> | <code>AccountCloseCancelPeer</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseCancellations</code> | 实际返回表达式：<code>&amp;AccountCloseCancellations{db: db, repository: repository.NewAccountCloseCancellations(db), peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseCancellations{db: db, repository: repository.NewAccountCloseCancellations(db), peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=742:961 -->
````go
func NewAccountCloseCancellations(db *sql.DB, peer AccountCloseCancelPeer) *AccountCloseCancellations {
	return &AccountCloseCancellations{db: db, repository: repository.NewAccountCloseCancellations(db), peer: peer}
}

````

## 第 5 步：AccountCloseCancellations.Ready

插入位置：第 4 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func (s *AccountCloseCancellations) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Ready(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>s.repository.Ready(ctx)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.repository.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=961:1066 -->
````go
func (s *AccountCloseCancellations) Ready(ctx context.Context) error { return s.repository.Ready(ctx) }

````

## 第 6 步：AccountCloseCancellations.RecoverOne

插入位置：第 5 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`func (s *AccountCloseCancellations) RecoverOne(parent context.Context, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.Claim(ctx, tx, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>finished.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>finished.Rollback()</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>finished.Rollback()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>finished.Commit() != nil</code>。
- 执行 <code>finished.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 6 bytes=1066:2319 -->
````go
func (s *AccountCloseCancellations) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := s.repository.Claim(ctx, tx, instance)
	if err != nil || tx.Commit() != nil {
		return
	}
	binding, err := s.repository.Binding(ctx, s.db, task)
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	// 只有Account同事务保存的REJECTED绑定能调用取消；成功响应来自Doc再次
	// 查询权威证明并完成本地提交。未知网络回复重试原号，不自造释放确认。
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	err = s.peer.Cancel(rpcCtx, binding)
	done()
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	finished, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	defer finished.Rollback()
	if err = s.repository.Finish(ctx, finished, task, binding); err != nil {
		_ = finished.Rollback()
		s.repository.Retry(ctx, task)
		return
	}
	if finished.Commit() != nil {
		s.repository.Retry(ctx, task)
	}
}

// account_close_confirmation.go：保留原实现，按业务合并文件。

````

## 第 7 步：AccountClosures.Confirmed

插入位置：第 6 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`func (s *AccountClosures) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.lockBinding(ctx, tx, binding)</code>；<code>s.repository.LockUser(ctx, tx, binding.User)</code>；<code>s.repository.ConfirmedScopes(ctx, tx, binding, capture)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>s.lockBinding(ctx, tx, binding)</code>；<code>binding.User</code>；<code>binding.IdentityEpoch</code>；<code>s.repository.ConfirmedScopes(ctx, tx, binding, capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>s.repository.ConfirmedScopes(ctx, tx, binding, capture)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>false</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.repository.ConfirmedScopes(ctx, tx, binding, capture)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_FROZEN_IDENTITY_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil &#124;&#124; (record.State != &quot;FROZEN&quot; &amp;&amp; record.State != &quot;PLANNED&quot; &amp;&amp; record.State != &quot;CLEANING&quot;)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>binding.IdentityEpoch == ^uint64(0) &#124;&#124; user.State != &quot;REVOKING&quot; &#124;&#124; user.IdentityEpoch != binding.IdentityEpoch+1 &#124;&#124; user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10)</code>。条件成立时执行 <code>return false, accountCloseConflict(&quot;ACCOUNT_CLOSE_FROZEN_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.repository.ConfirmedScopes(ctx, tx, binding, capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2319:3334 -->
````go
func (s *AccountClosures) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if capture.Binding == nil || (record.State != "FROZEN" && record.State != "PLANNED" && record.State != "CLEANING") {
		return false, nil
	}
	user, err := s.repository.LockUser(ctx, tx, binding.User)
	if err != nil {
		return false, err
	}
	if binding.IdentityEpoch == ^uint64(0) || user.State != "REVOKING" || user.IdentityEpoch != binding.IdentityEpoch+1 || user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10) {
		return false, accountCloseConflict("ACCOUNT_CLOSE_FROZEN_IDENTITY_CHANGED")
	}
	return s.repository.ConfirmedScopes(ctx, tx, binding, capture)
}

// account_close_finalize.go：保留原实现，按业务合并文件。

````

## 第 8 步：AccountCloseRecoveries.finalize

插入位置：第 7 步代码之后，紧接追加；完成稿约第 110 行。

仅接受原计划最后一页的真实Doc SUCCEEDED；没有页面/清理资格、网络未知或
本人冻结/任一ACK不完整都继续等待。最后个人信息、组织关系、回执和原任务同事务。

本步声明：`func (s *AccountCloseRecoveries) finalize(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan, proof platform.AccountClosurePage) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.closures.lockBinding(ctx, tx, *capture.Binding)</code>；<code>s.closures.repository.LockUser(ctx, tx, capture.Binding.User)</code>；<code>s.closures.repository.ConfirmedScopes(ctx, tx, *capture.Binding, original)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code>；<code>task.Operation</code>；<code>repository.FinishAccountCloseRecovery(ctx, tx, task, true)</code> |
| <code>expected</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>expected.State</code>；<code>expected.Capture</code> |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>capture.Binding</code>；<code>capture.Retention</code>；<code>capture.IdentityEpoch</code>；<code>capture.CredentialEpoch</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>plan</code> | <code>dto.AccountClosePlan</code> | 在此声明中的实际使用：<code>plan.State</code>；<code>json.Marshal(plan)</code> |
| <code>proof</code> | <code>platform.AccountClosurePage</code> | 使用 <code>platform.AccountClosurePage</code> 类型；其定义处的职责：定义Account与Document之间的固定注销绑定和有界清理分页协议。<br>在此声明中的实际使用：<code>proof.Matches</code>；<code>proof.State</code>；<code>proof.Done</code>；<code>proof.Documents</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_PROOF_REQUIRED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_PROOF_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_FINAL_STATE_CHANGED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>capture.Binding == nil &#124;&#124; capture.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_PROOF_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!proof.Matches(request) &#124;&#124; proof.State != &quot;SUCCEEDED&quot; &#124;&#124; !proof.Done &#124;&#124; len(proof.Documents) != 0</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.State != &quot;CLEANING&quot; &#124;&#124; current.State != expected.State &#124;&#124; !bytes.Equal(current.Capture, expected.Capture)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>capture.IdentityEpoch == ^uint64(0) &#124;&#124; user.State != &quot;REVOKING&quot; &#124;&#124; user.IdentityEpoch != capture.IdentityEpoch+1 &#124;&#124; user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_FINAL_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!confirmed</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_FINAL_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=3334:5567 -->
````go
// 仅接受原计划最后一页的真实Doc SUCCEEDED；没有页面/清理资格、网络未知或
// 本人冻结/任一ACK不完整都继续等待。最后个人信息、组织关系、回执和原任务同事务。
func (s *AccountCloseRecoveries) finalize(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan, proof platform.AccountClosurePage) error {
	if capture.Binding == nil || capture.Retention == nil {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_REQUIRED")
	}
	request := platform.AccountClosurePageRequest{AccountClosureBinding: *capture.Binding, PlanDigest: capture.Retention.PlanDigest, AfterDocument: capture.Retention.AfterDocument}
	if !proof.Matches(request) || proof.State != "SUCCEEDED" || !proof.Done || len(proof.Documents) != 0 {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, original, _, err := s.closures.lockBinding(ctx, tx, *capture.Binding)
	if err != nil {
		return err
	}
	if current.State != "CLEANING" || current.State != expected.State || !bytes.Equal(current.Capture, expected.Capture) {
		return errors.New("ACCOUNT_CLOSE_FINAL_STATE_CHANGED")
	}
	user, err := s.closures.repository.LockUser(ctx, tx, capture.Binding.User)
	if err != nil {
		return err
	}
	if capture.IdentityEpoch == ^uint64(0) || user.State != "REVOKING" || user.IdentityEpoch != capture.IdentityEpoch+1 || user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10) {
		return errors.New("ACCOUNT_CLOSE_FINAL_IDENTITY_CHANGED")
	}
	confirmed, err := s.closures.repository.ConfirmedScopes(ctx, tx, *capture.Binding, original)
	if err != nil {
		return err
	}
	if !confirmed {
		return accountCloseConflict("ACCOUNT_CLOSE_FINAL_DRAIN_PENDING")
	}
	if err = repository.FinalizeAccountCloseData(ctx, tx, *capture.Binding, capture); err != nil {
		return err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	plan.State = "SUCCEEDED"
	capture.Retention.DocCompleted = true
	rawCapture, err := json.Marshal(capture)
````

## 第 9 步：AccountCloseRecoveries.finalize（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 9 bytes=5567:6149 -->
````go
	if err != nil {
		return err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan); err != nil {
		return err
	}
	if err = repository.CompleteAccountCloseReceipt(ctx, tx, *capture.Binding, proof.PlanDigest, rawPlan); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, true); err != nil {
		return err
	}
	return tx.Commit()
}

// account_close_progress.go：保留原实现，按业务合并文件。

````

## 第 10 步：AccountCloseSession

插入位置：第 9 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`type AccountCloseSession struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。 |
| <code>Token</code> | <code>string</code> | 选中用于核对原关闭操作的会话令牌；可能来自专用恢复Cookie，不授予新的普通会话权限。 |

<!-- write-step: 10 bytes=6149:6226 -->
````go
type AccountCloseSession struct {
	Session domain.Session
	Token   string
}

````

## 第 11 步：accountCloseNotFound

插入位置：第 10 步代码之后，紧接追加；完成稿约第 181 行。

本步声明：`func accountCloseNotFound() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ACCOUNT_CLOSE_NOT_FOUND&quot;}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.RuleViolation{Kind: domain.EntityNotFound, Code: &quot;ACCOUNT_CLOSE_NOT_FOUND&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6226:6356 -->
````go
func accountCloseNotFound() error {
	return domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ACCOUNT_CLOSE_NOT_FOUND"}
}

````

## 第 12 步：AccountClosures.Resolve

插入位置：第 11 步代码之后，紧接追加；完成稿约第 185 行。

冻结后只能用原会话秘密恢复这一条最小进度，不能把恢复Cookie提升成登录权限。
新操作只能用当前有效主会话；恢复Cookie不能发起另一次注销。

本步声明：`func (s *AccountClosures) Resolve(ctx context.Context, operation, primary, recovery string) (AccountCloseSession, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.ProgressBinding(ctx, operation)</code>；<code>s.authentication.Authenticate(ctx, primary)</code>；<code>s.authentication.Session(ctx, token)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>s.repository.ProgressBinding(ctx, operation)</code> |
| <code>primary</code> | <code>string</code> | 在此声明中的实际使用：<code>s.authentication.Authenticate(ctx, primary)</code>；<code>Token: primary</code> |
| <code>recovery</code> | <code>string</code> | 恢复Cookie携带的原会话证明；主会话失效后仅用于原关闭操作的查询核验，不能发起新修改。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountCloseSession</code> | 实际返回表达式：<code>AccountCloseSession{}</code>；<code>AccountCloseSession{Session: authenticated.Session, Token: primary}</code>；<code>AccountCloseSession{Session: session, Token: token}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>；<code>err</code>；<code>nil</code>；<code>accountCloseNotFound()</code> |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。条件成立时执行 <code>return AccountCloseSession{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;OPERATION_ID_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>errors.Is(err, domain.ErrSessionInvalid)</code>。
- 此处通过 <code>return AccountCloseSession{Session: authenticated.Session, Token: primary}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return AccountCloseSession{}, err</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; receipt.Kind == &quot;ACCOUNT_CLOSE&quot; &amp;&amp; strconv.FormatUint(receipt.User, 10) == session.UserID &amp;&amp; receipt.SessionHash == hex.EncodeToString(session.ID)</code>。条件成立时执行 <code>return AccountCloseSession{Session: session, Token: token}, nil</code>。当前函数在这里结束。
- 此处通过 <code>return AccountCloseSession{}, accountCloseNotFound()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6356:7687 -->
````go
// 冻结后只能用原会话秘密恢复这一条最小进度，不能把恢复Cookie提升成登录权限。
// 新操作只能用当前有效主会话；恢复Cookie不能发起另一次注销。
func (s *AccountClosures) Resolve(ctx context.Context, operation, primary, recovery string) (AccountCloseSession, error) {
	if !domain.ValidOperationID(operation) {
		return AccountCloseSession{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	receipt, err := s.repository.ProgressBinding(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		authenticated, err := s.authentication.Authenticate(ctx, primary)
		if errors.Is(err, domain.ErrSessionInvalid) {
			err = domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
		}
		return AccountCloseSession{Session: authenticated.Session, Token: primary}, err
	}
	if err != nil {
		return AccountCloseSession{}, err
	}
	for _, token := range []string{primary, recovery} {
		session, err := s.authentication.Session(ctx, token)
		if err == nil && receipt.Kind == "ACCOUNT_CLOSE" && strconv.FormatUint(receipt.User, 10) == session.UserID && receipt.SessionHash == hex.EncodeToString(session.ID) {
			return AccountCloseSession{Session: session, Token: token}, nil
		}
	}
	return AccountCloseSession{}, accountCloseNotFound()
}

````

## 第 13 步：AccountClosures.Progress

插入位置：第 12 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`func (s *AccountClosures) Progress(ctx context.Context, operation, primary, recovery string) (dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Resolve(ctx, operation, primary, recovery)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.repository.LockUser(ctx, tx, user)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Resolve(ctx, operation, primary, recovery)</code>；<code>s.repository.Receipt(ctx, tx, operation)</code>；<code>s.repository.Record(ctx, tx, operation)</code>；<code>accountCloseSavedPlan(operation, record, receipt)</code> |
| <code>primary</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Resolve(ctx, operation, primary, recovery)</code> |
| <code>recovery</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Resolve(ctx, operation, primary, recovery)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>accountCloseNotFound()</code>；<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;rule) &#124;&#124; errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return plan, accountCloseNotFound()</code>。当前函数在这里结束。
- 此处通过 <code>return plan, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, accountCloseNotFound()</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return plan, accountCloseNotFound()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>receipt.User != user &#124;&#124; receipt.Kind != &quot;ACCOUNT_CLOSE&quot; &#124;&#124; receipt.SessionHash != hex.EncodeToString(selection.Session.ID)</code>。条件成立时执行 <code>return plan, accountCloseNotFound()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 检查 <code>capture.SessionHash != receipt.SessionHash &#124;&#124; strconv.FormatUint(capture.CredentialEpoch, 10) != selection.Session.CredentialEpoch</code>。条件成立时执行 <code>return plan, accountCloseNotFound()</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=7687:9300 -->
````go
func (s *AccountClosures) Progress(ctx context.Context, operation, primary, recovery string) (dto.AccountClosePlan, error) {
	var plan dto.AccountClosePlan
	selection, err := s.Resolve(ctx, operation, primary, recovery)
	if err != nil {
		// 匿名、其他会话和不存在的操作都不暴露注销是否存在。
		var rule domain.RuleViolation
		if errors.As(err, &rule) || errors.Is(err, sql.ErrNoRows) {
			return plan, accountCloseNotFound()
		}
		return plan, err
	}
	user, err := strconv.ParseUint(selection.Session.UserID, 10, 64)
	if err != nil {
		return plan, accountCloseNotFound()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return plan, err
	}
	defer tx.Rollback()
	if _, err = s.repository.LockUser(ctx, tx, user); err != nil {
		return plan, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return plan, accountCloseNotFound()
	}
	if err != nil {
		return plan, err
	}
	if receipt.User != user || receipt.Kind != "ACCOUNT_CLOSE" || receipt.SessionHash != hex.EncodeToString(selection.Session.ID) {
		return plan, accountCloseNotFound()
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return plan, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return plan, err
	}
	if capture.SessionHash != receipt.SessionHash || strconv.FormatUint(capture.CredentialEpoch, 10) != selection.Session.CredentialEpoch {
		return plan, accountCloseNotFound()
	}
	plan, err = accountCloseSavedPlan(operation, record, receipt)
````

## 第 14 步：AccountClosures.Progress（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 256 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return plan, err</code>。当前函数在这里结束。
- 此处通过 <code>return plan, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 14 bytes=9300:9443 -->
````go
	if err != nil {
		return plan, err
	}
	return plan, tx.Commit()
}

// account_close_recovery.go：保留原实现，按业务合并文件。

````

## 第 15 步：AccountCloseRecoveryPeer

插入位置：第 14 步代码之后，紧接追加；完成稿约第 264 行。

本步声明：`type AccountCloseRecoveryPeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>AccountClosurePreparer</code> | <code>AccountClosurePreparer</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>Plan</code> | <code>(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)</code> | 以原关闭绑定取得固定留存计划，返回计划事实与错误。 |
| <code>Relations</code> | <code>(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)</code> | 按原绑定推进关系留存，返回已固定的留存事实，不换计划。 |
| <code>Page</code> | <code>(context.Context, platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)</code> | 接收绑定及游标，返回本轮文档清理页和下一游标；逐页恢复避免无限一次性工作。 |
| <code>CleanDocument</code> | <code>(context.Context, platform.AccountClosureBinding, string) error</code> | 参数依次为ctx、原关闭绑定、文档编号；请求Doc清理该计划内文档。 |
| <code>Complete</code> | <code>(context.Context, platform.AccountClosureBinding) error</code> | 以原绑定确认Doc留存/清理完成；调用成功不替代Account自己事务中的终态提交。 |

<!-- write-step: 15 bytes=9443:9950 -->
````go
type AccountCloseRecoveryPeer interface {
	AccountClosurePreparer
	Plan(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Relations(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Page(context.Context, platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)
	CleanDocument(context.Context, platform.AccountClosureBinding, string) error
	Complete(context.Context, platform.AccountClosureBinding) error
}

````

## 第 16 步：AccountCloseRecoveries

插入位置：第 15 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`type AccountCloseRecoveries struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>closures</code> | <code>*AccountClosures</code> | 使用 <code>AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。 |
| <code>peer</code> | <code>AccountCloseRecoveryPeer</code> | Doc留存与清理参与者，Account仅通过接口推进，不跨库删除文档。 |

<!-- write-step: 16 bytes=9950:10070 -->
````go
type AccountCloseRecoveries struct {
	db       *sql.DB
	closures *AccountClosures
	peer     AccountCloseRecoveryPeer
}

````

## 第 17 步：NewAccountCloseRecoveries

插入位置：第 16 步代码之后，紧接追加；完成稿约第 279 行。

本步声明：`func NewAccountCloseRecoveries(db *sql.DB, closures *AccountClosures, peer AccountCloseRecoveryPeer) *AccountCloseRecoveries`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>closures</code> | <code>*AccountClosures</code> | 使用 <code>AccountClosures</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>closures: closures</code> |
| <code>peer</code> | <code>AccountCloseRecoveryPeer</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountCloseRecoveries</code> | 实际返回表达式：<code>&amp;AccountCloseRecoveries{db: db, closures: closures, peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountCloseRecoveries{db: db, closures: closures, peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=10070:10272 -->
````go
func NewAccountCloseRecoveries(db *sql.DB, closures *AccountClosures, peer AccountCloseRecoveryPeer) *AccountCloseRecoveries {
	return &AccountCloseRecoveries{db: db, closures: closures, peer: peer}
}

````

## 第 18 步：accountCloseRecoveryUser

插入位置：第 17 步代码之后，紧接追加；完成稿约第 283 行。

本步声明：`func accountCloseRecoveryUser(task domain.AccountCloseFenceTask) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Payload</code>；<code>task.Operation</code>；<code>task.Revocation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>payload.User</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;payload) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; !domain.ValidOperationID(task.Operation) &#124;&#124; payload.Operation != task.Operation &#124;&#124; task.Revocation != task.Operation &#124;&#124; payload.User == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return payload.User, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=10272:10813 -->
````go
func accountCloseRecoveryUser(task domain.AccountCloseFenceTask) (uint64, error) {
	var payload repository.AccountCloseRecoveryPayload
	decoder := json.NewDecoder(bytes.NewReader(task.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || !domain.ValidOperationID(task.Operation) || payload.Operation != task.Operation || task.Revocation != task.Operation || payload.User == 0 {
		return 0, errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	return payload.User, nil
}

````

## 第 19 步：AccountCloseRecoveries.RecoverOne

插入位置：第 18 步代码之后，紧接追加；完成稿约第 293 行。

一次领取只推进一个阶段或一个文档，避免大账号占满同步HTTP/任务预算。
跨服务调用不持SQL锁；每步本地进度和原任务重新排队同事务，回复丢失原号续接。

本步声明：`func (s *AccountCloseRecoveries) RecoverOne(parent context.Context, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code>；<code>context.WithTimeout(parent, time.Second)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ClaimAccountCloseRecovery(ctx, tx, instance)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>done()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 19 bytes=10813:11767 -->
````go
// 一次领取只推进一个阶段或一个文档，避免大账号占满同步HTTP/任务预算。
// 跨服务调用不持SQL锁；每步本地进度和原任务重新排队同事务，回复丢失原号续接。
func (s *AccountCloseRecoveries) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return
	}
	task, err := repository.ClaimAccountCloseRecovery(ctx, tx, instance)
	if err != nil {
		tx.Rollback()
		return
	}
	if tx.Commit() != nil {
		return
	}
	if err = s.advance(ctx, task); err != nil {
		// 本地提交未知不能改写已成功任务；条件重试只作用原领取代次。
		retry, done := context.WithTimeout(parent, time.Second)
		defer done()
		_ = repository.RetryAccountCloseRecovery(retry, s.db, task, "ACCOUNT_CLOSE_STEP_UNCONFIRMED")
	}
}

````

## 第 20 步：AccountCloseRecoveries.read

插入位置：第 19 步代码之后，紧接追加；完成稿约第 318 行。

本步声明：`func (s *AccountCloseRecoveries) read(ctx context.Context, task domain.AccountCloseFenceTask) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.closures.repository.LockUser(ctx, tx, user)</code>；<code>s.closures.repository.Receipt(ctx, tx, task.Operation)</code>；<code>s.closures.repository.Record(ctx, tx, task.Operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>accountCloseRecoveryUser(task)</code>；<code>task.Operation</code>；<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>实际返回表达式：<code>capture</code> |
| <code>(匿名/未命名)</code> | <code>dto.AccountClosePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, capture, plan, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, capture, plan, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, capture, plan, err</code>。当前函数在这里结束。
- 检查 <code>record.User != user &#124;&#124; receipt.User != user &#124;&#124; receipt.SessionHash != capture.SessionHash</code>。条件成立时执行 <code>return record, capture, plan, errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, capture, plan, err</code>。当前函数在这里结束。
- 此处通过 <code>return record, capture, plan, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 20 bytes=11767:13274 -->
````go
func (s *AccountCloseRecoveries) read(ctx context.Context, task domain.AccountCloseFenceTask) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error) {
	var record domain.AccountCloseRecord
	var capture domain.AccountCloseCapture
	var plan dto.AccountClosePlan
	user, err := accountCloseRecoveryUser(task)
	if err != nil {
		return record, capture, plan, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return record, capture, plan, err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return record, capture, plan, err
	}
	receipt, err := s.closures.repository.Receipt(ctx, tx, task.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	record, err = s.closures.repository.Record(ctx, tx, task.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	capture, err = domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return record, capture, plan, err
	}
	if record.User != user || receipt.User != user || receipt.SessionHash != capture.SessionHash {
		return record, capture, plan, errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	plan, err = accountCloseSavedPlan(task.Operation, record, receipt)
	if err != nil {
		return record, capture, plan, err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return record, capture, plan, err
	}
	return record, capture, plan, tx.Commit()
}

````

## 第 21 步：AccountCloseRecoveries.advance

插入位置：第 20 步代码之后，紧接追加；完成稿约第 359 行。

本步声明：`func (s *AccountCloseRecoveries) advance(ctx context.Context, task domain.AccountCloseFenceTask) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.read(ctx, task)</code>；<code>s.finishTerminal(ctx, task, record.User)</code>；<code>s.closures.PrepareClose(ctx, task.Operation, s.peer)</code>；<code>s.requeueCurrent(ctx, task, record.User)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>s.read(ctx, task)</code>；<code>s.finishTerminal(ctx, task, record.User)</code>；<code>task.Operation</code>；<code>s.requeueCurrent(ctx, task, record.User)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>s.finishTerminal(ctx, task, record.User)</code>；<code>accountCloseConflict(&quot;ACCOUNT_CLOSE_HANDOFF_REQUIRED&quot;)</code>；<code>s.requeueCurrent(ctx, task, record.User)</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;SUCCEEDED&quot; &#124;&#124; record.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return s.finishTerminal(ctx, task, record.User)</code>。当前函数在这里结束。
- 检查 <code>record.State == &quot;PREPARING&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State == &quot;PREPARING&quot; &amp;&amp; len(result.RequiredSuccessions) &gt; 0</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_HANDOFF_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.requeueCurrent(ctx, task, record.User)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!confirmed</code>。条件成立时执行 <code>return accountCloseConflict(&quot;ACCOUNT_CLOSE_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!page.Matches(request)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 21 bytes=13274:14688 -->
````go
func (s *AccountCloseRecoveries) advance(ctx context.Context, task domain.AccountCloseFenceTask) error {
	record, capture, plan, err := s.read(ctx, task)
	if err != nil {
		return err
	}
	if record.State == "SUCCEEDED" || record.State == "REJECTED" {
		return s.finishTerminal(ctx, task, record.User)
	}
	if record.State == "PREPARING" {
		result, err := s.closures.PrepareClose(ctx, task.Operation, s.peer)
		if err != nil {
			return err
		}
		if result.State == "PREPARING" && len(result.RequiredSuccessions) > 0 {
			return accountCloseConflict("ACCOUNT_CLOSE_HANDOFF_REQUIRED")
		}
		return s.requeueCurrent(ctx, task, record.User)
	}
	if capture.Binding == nil {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	binding := *capture.Binding
	switch record.State {
	case "FROZEN":
		confirmed, err := s.closures.Confirmed(ctx, binding)
		if err != nil {
			return err
		}
		if !confirmed {
			return accountCloseConflict("ACCOUNT_CLOSE_DRAIN_PENDING")
		}
		if _, err = s.peer.Plan(ctx, binding); err != nil {
			return err
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		capture.Retention = &domain.AccountCloseRetention{PlanDigest: page.PlanDigest}
		plan.State = "PLANNED"
	case "PLANNED":
````

## 第 22 步：AccountCloseRecoveries.advance（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 404 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>capture.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RETENTION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>capture.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RETENTION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!page.Matches(request)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(page.Documents) &gt; 0</code>。
- 检查 <code>capture.Retention.DocCompleted</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_COMPLETION_CURSOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>document.Release != &quot;0&quot;</code>。
- 检查 <code>!page.Done</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_PAGE_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>page.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return s.finalize(ctx, task, record, capture, plan, page)</code>。当前函数在这里结束。
- 检查 <code>capture.Retention.DocCompleted</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_DOC_COMPLETION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 22 bytes=14688:16190 -->
````go
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		if _, err = s.peer.Relations(ctx, binding); err != nil {
			return err
		}
		plan.State = "CLEANING"
	case "CLEANING":
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding, AfterDocument: capture.Retention.AfterDocument, PlanDigest: capture.Retention.PlanDigest}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		if len(page.Documents) > 0 {
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_COMPLETION_CURSOR_CHANGED")
			}
			document := page.Documents[0]
			if err = s.peer.CleanDocument(ctx, binding, document.Document); err != nil {
				return err
			}
			capture.Retention.AfterDocument = document.Document
			if document.Release != "0" {
				plan.RetainedReleaseIds = append(plan.RetainedReleaseIds, document.Release)
			}
		} else {
			if !page.Done {
				return errors.New("ACCOUNT_CLOSE_PAGE_INCOMPLETE")
			}
			if page.State == "SUCCEEDED" {
				return s.finalize(ctx, task, record, capture, plan, page)
			}
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_DOC_COMPLETION_CHANGED")
			}
			if err = s.peer.Complete(ctx, binding); err != nil {
				return err
			}
			capture.Retention.DocCompleted = true
		}
````

## 第 23 步：AccountCloseRecoveries.advance（接着实现）

插入位置：第 22 步代码之后，紧接追加；完成稿约第 450 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;ACCOUNT_CLOSE_STATE_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return s.persist(ctx, task, record, capture, plan)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=16190:16309 -->
````go
	default:
		return errors.New("ACCOUNT_CLOSE_STATE_INVALID")
	}
	return s.persist(ctx, task, record, capture, plan)
}

````

## 第 24 步：AccountCloseRecoveries.persist

插入位置：第 23 步代码之后，紧接追加；完成稿约第 456 行。

本步声明：`func (s *AccountCloseRecoveries) persist(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.closures.lockBinding(ctx, tx, *capture.Binding)</code>；<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code>；<code>repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code>；<code>task.Operation</code>；<code>repository.FinishAccountCloseRecovery(ctx, tx, task, false)</code> |
| <code>expected</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>expected.State</code>；<code>expected.Capture</code> |
| <code>capture</code> | <code>domain.AccountCloseCapture</code> | 使用 <code>domain.AccountCloseCapture</code> 类型；其定义处的职责：AccountCloseCapture 只保存原会话摘要及代次；密码从不入计划，请求指纹只在操作回执保存。 Binding在下游关闭范围固定之前必须为nil，不能用空组织集合冒充已完成规划。<br>在此声明中的实际使用：<code>capture.Binding</code>；<code>json.Marshal(capture)</code> |
| <code>plan</code> | <code>dto.AccountClosePlan</code> | 在此声明中的实际使用：<code>json.Marshal(plan)</code>；<code>plan.State</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_STEP_CHANGED&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.State != expected.State &#124;&#124; !bytes.Equal(current.Capture, expected.Capture)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_STEP_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 24 bytes=16309:17408 -->
````go
func (s *AccountCloseRecoveries) persist(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, _, _, err := s.closures.lockBinding(ctx, tx, *capture.Binding)
	if err != nil {
		return err
	}
	if current.State != expected.State || !bytes.Equal(current.Capture, expected.Capture) {
		return errors.New("ACCOUNT_CLOSE_STEP_CHANGED")
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, false); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 25 步：AccountCloseRecoveries.requeueCurrent

插入位置：第 24 步代码之后，紧接追加；完成稿约第 489 行。

本步声明：`func (s *AccountCloseRecoveries) requeueCurrent(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.closures.repository.LockUser(ctx, tx, user)</code>；<code>s.closures.repository.Receipt(ctx, tx, task.Operation)</code>；<code>s.closures.repository.Record(ctx, tx, task.Operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code>；<code>repository.FinishAccountCloseRecovery(ctx, tx, task, false)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.closures.repository.LockUser(ctx, tx, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 25 bytes=17408:18170 -->
````go
func (s *AccountCloseRecoveries) requeueCurrent(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err = s.closures.repository.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = s.closures.repository.Record(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, false); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 26 步：AccountCloseRecoveries.finishTerminal

插入位置：第 25 步代码之后，紧接追加；完成稿约第 513 行。

本步声明：`func (s *AccountCloseRecoveries) finishTerminal(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.closures.repository.LockUser(ctx, tx, user)</code>；<code>s.closures.repository.Receipt(ctx, tx, task.Operation)</code>；<code>s.closures.repository.Record(ctx, tx, task.Operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateAccountCloseRecovery(ctx, tx, task)</code>；<code>repository.FinishAccountCloseRecovery(ctx, tx, task, true)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.closures.repository.LockUser(ctx, tx, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_NOT_TERMINAL&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;SUCCEEDED&quot; &amp;&amp; record.State != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_NOT_TERMINAL&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 26 bytes=18170:19135 -->
````go
func (s *AccountCloseRecoveries) finishTerminal(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err = s.closures.repository.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	record, err := s.closures.repository.Record(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if record.State != "SUCCEEDED" && record.State != "REJECTED" {
		return errors.New("ACCOUNT_CLOSE_NOT_TERMINAL")
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, true); err != nil {
		return err
	}
	return tx.Commit()
}

// account_close_recovery_ready.go：保留原实现，按业务合并文件。

````

## 第 27 步：AccountCloseRecoveries.Ready

插入位置：第 26 步代码之后，紧接追加；完成稿约第 543 行。

待恢复任务必须仍绑定原账号、原回执及固定计划。逐页核验持久事实，
READY不能用忽略孤立任务的INNER JOIN或把损坏任务改成功来掩盖恢复缺口。

本步声明：`func (s *AccountCloseRecoveries) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountCloseRecoveryPage(ctx, s.db, after)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(page) &lt; 100</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=19135:19740 -->
````go
// 待恢复任务必须仍绑定原账号、原回执及固定计划。逐页核验持久事实，
// READY不能用忽略孤立任务的INNER JOIN或把损坏任务改成功来掩盖恢复缺口。
func (s *AccountCloseRecoveries) Ready(ctx context.Context) error {
	after := uint64(0)
	for {
		page, err := repository.AccountCloseRecoveryPage(ctx, s.db, after)
		if err != nil {
			return err
		}
		for _, row := range page {
			if err = validateRecoveryPlan(row.Task, row.Record, row.Receipt); err != nil {
				return err
			}
			after = row.Task.ID
		}
		if len(page) < 100 {
			return nil
		}
	}
}

````

## 第 28 步：validateRecoveryPlan

插入位置：第 27 步代码之后，紧接追加；完成稿约第 564 行。

本步声明：`func validateRecoveryPlan(task domain.AccountCloseFenceTask, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>task</code> | <code>domain.AccountCloseFenceTask</code> | 使用 <code>domain.AccountCloseFenceTask</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>accountCloseRecoveryUser(task)</code>；<code>task.Operation</code> |
| <code>record</code> | <code>domain.AccountCloseRecord</code> | 使用 <code>domain.AccountCloseRecord</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>record.Capture</code>；<code>record.User</code>；<code>accountCloseSavedPlan(task.Operation, record, receipt)</code>；<code>record.State</code> |
| <code>receipt</code> | <code>domain.AccountCloseReceipt</code> | 使用 <code>domain.AccountCloseReceipt</code> 类型；其定义处的职责：定义账号注销时固定的原会话、组织图、来源屏障与文档清理游标。<br>在此声明中的实际使用：<code>receipt.User</code>；<code>receipt.SessionHash</code>；<code>receipt.Result</code>；<code>accountCloseSavedPlan(task.Operation, record, receipt)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_RETENTION_REQUIRED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSE_STATE_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>record.User != user &#124;&#124; receipt.User != user &#124;&#124; receipt.SessionHash != capture.SessionHash &#124;&#124; !json.Valid(receipt.Result)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>capture.Binding == nil &#124;&#124; capture.Retention == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RETENTION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;ACCOUNT_CLOSE_STATE_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>capture.Binding != nil &amp;&amp; (capture.Binding.User != user &#124;&#124; capture.Binding.Operation != task.Operation)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=19740:20927 -->
````go
func validateRecoveryPlan(task domain.AccountCloseFenceTask, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) error {
	user, err := accountCloseRecoveryUser(task)
	if err != nil {
		return err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return err
	}
	if record.User != user || receipt.User != user || receipt.SessionHash != capture.SessionHash || !json.Valid(receipt.Result) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	if _, err = accountCloseSavedPlan(task.Operation, record, receipt); err != nil {
		return err
	}
	switch record.State {
	case "PREPARING", "REJECTED":
	case "FROZEN":
		if capture.Binding == nil {
			return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
		}
	case "PLANNED", "CLEANING", "SUCCEEDED":
		if capture.Binding == nil || capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
	default:
		return errors.New("ACCOUNT_CLOSE_STATE_INVALID")
	}
	if capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != task.Operation) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/account_close_completion_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
