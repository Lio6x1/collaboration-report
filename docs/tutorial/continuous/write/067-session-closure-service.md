# 分步编写 `services/account/internal/logic/account/session_closure_service.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/067-session-closure-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。

浏览器断电也必须停止到期会话的所有输出；worker凭持久任务恢复，不依赖用户再次点击。

写之前先按这条执行路线理解各段的连接：

1. Logout.Resolve从主/恢复秘密找原操作；Prepare在user→session→receipt锁内登记关闭。
2. Closure.RecoverOne领取并提交任务后才调用FenceAccess，验证Doc摘要再finish。
3. EnqueueExpired选到期ACTIVE会话，再在事务重锁复核，使用同样持久关闭协议。

本文件需要守住的边界：

- Commit未知不能清除唯一恢复依据；更换原号会破坏幂等。
- Doc失败只重试，不能由本地超时推断旧输出已经停止。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/session_closure_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/session_closure_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/session_closure_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/session_closure_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:530 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"
)

// closure.go：保留原实现，按业务合并文件。

````

## 第 2 步：ClosurePeer

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type ClosurePeer interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>FenceAccess</code> | <code>(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)</code> | 向Doc提交固定来源关闭请求，返回参与者停止输出的确认。 |
| <code>CloseDepartedGrants</code> | <code>(context.Context, domain.SessionClosure, string) error</code> | 顺序接收ctx、原会话关闭事实、任务实例标识，推进退出成员相关授权关闭。 |
| <code>FailureCode</code> | <code>(error) string</code> | 把下游error归一为可记录的失败码；不把原始敏感错误直接返回用户。 |

<!-- write-step: 2 bytes=530:737 -->
````go
type ClosurePeer interface {
	FenceAccess(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
	CloseDepartedGrants(context.Context, domain.SessionClosure, string) error
	FailureCode(error) string
}

````

## 第 3 步：Closure

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type Closure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Closure</code> | 使用 <code>repository.Closure</code> 类型；其定义处的职责：持久化来源关闭、领取恢复任务、保存真实Doc ACK并完成会话或成员收尾。 |
| <code>peer</code> | <code>ClosurePeer</code> | 跨服务关闭参与者接口；恢复仍依据持久任务及真实ACK，不凭进程内标志完成。 |

<!-- write-step: 3 bytes=737:838 -->
````go
type Closure struct {
	db         *sql.DB
	repository *repository.Closure
	peer       ClosurePeer
}

````

## 第 4 步：NewClosure

插入位置：第 3 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func NewClosure(db *sql.DB, repo *repository.Closure, peer ClosurePeer) *Closure`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repo</code> | <code>*repository.Closure</code> | 使用 <code>repository.Closure</code> 类型；其定义处的职责：持久化来源关闭、领取恢复任务、保存真实Doc ACK并完成会话或成员收尾。<br>在此声明中的实际使用：<code>repository: repo</code> |
| <code>peer</code> | <code>ClosurePeer</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Closure</code> | 实际返回表达式：<code>&amp;Closure{db: db, repository: repo, peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Closure{db: db, repository: repo, peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=838:979 -->
````go
func NewClosure(db *sql.DB, repo *repository.Closure, peer ClosurePeer) *Closure {
	return &Closure{db: db, repository: repo, peer: peer}
}

````

## 第 5 步：Closure.RecoverOne

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func (s *Closure) RecoverOne(parent context.Context, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 4*time.Second)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.MarkRunning(ctx, tx, task.ID, instance)</code>；<code>s.repository.Retry(ctx, task.ID, instance, &quot;LOGOUT_PAYLOAD_INVALID&quot;)</code>；<code>s.repository.Retry(ctx, task.ID, instance, &quot;DEPARTURE_GRANTS_UNKNOWN&quot;)</code>；<code>s.repository.Retry(ctx, task.ID, instance, &quot;FENCE_NOT_CONFIRMED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>s.repository.MarkRunning(ctx, tx, task.ID, instance) != nil &#124;&#124; tx.Commit() != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>json.Unmarshal(task.Payload, &amp;binding) == nil</code>。
- 检查 <code>epochError == nil &amp;&amp; ((sessionError == nil &amp;&amp; len(session) == 32) &#124;&#124; binding.UserID != &quot;&quot;)</code>。
- 检查 <code>binding.UserID != &quot;&quot;</code>。
- 检查 <code>request == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>binding.Kind == &quot;ORG_LEAVE&quot; &#124;&#124; binding.Kind == &quot;ORG_REMOVE&quot;</code>。
- 检查 <code>s.peer.CloseDepartedGrants(ctx, binding, task.OperationID) != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=979:2653 -->
````go
func (s *Closure) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	// 先提交固定任务的持久租约，网络调用在事务外；未知结果仍重取原任务。
	task, err := s.repository.Claim(ctx, tx)
	if err != nil {
		return
	}
	if s.repository.MarkRunning(ctx, tx, task.ID, instance) != nil || tx.Commit() != nil {
		return
	}
	var binding domain.SessionClosure
	var request *pb.FenceRequest
	if json.Unmarshal(task.Payload, &binding) == nil {
		epoch, epochError := strconv.ParseUint(binding.Epoch, 10, 64)
		session, sessionError := hex.DecodeString(binding.SessionHash)
		if epochError == nil && ((sessionError == nil && len(session) == 32) || binding.UserID != "") {
			request = &pb.FenceRequest{
				RevocationId: task.OperationID,
				ClosedScopes: []*pb.SourceEpoch{{ScopeKey: binding.Scope, Epoch: epoch}},
				Generation:   1,
			}
			if binding.UserID != "" {
				request.TargetDigest = platform.SourceClosureDigest(binding.Scope, epoch)
			} else {
				request.TargetDigest = platform.LogoutTargetDigest(binding.SessionHash)
			}
		}
	}
	if request == nil {
		s.repository.Retry(ctx, task.ID, instance, "LOGOUT_PAYLOAD_INVALID")
		return
	}
	// 离开关系才关闭组织下属授权；降级保留独立授权。持久授权关闭先于排空ACK。
	if binding.Kind == "ORG_LEAVE" || binding.Kind == "ORG_REMOVE" {
		if s.peer.CloseDepartedGrants(ctx, binding, task.OperationID) != nil {
			s.repository.Retry(ctx, task.ID, instance, "DEPARTURE_GRANTS_UNKNOWN")
			return
		}
	}
````

## 第 6 步：Closure.RecoverOne（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 85 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 检查 <code>err != nil &#124;&#124; !platform.MatchesBootstrapAck(request, ack, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2653:3185 -->
````go
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	ack, err := s.peer.FenceAccess(rpcCtx, request)
	done()
	if err != nil || !platform.MatchesBootstrapAck(request, ack, "document") {
		// 日志只记录稳定码，不输出私有payload、原始令牌和依赖驱动详情。
		slog.Warn("logout_fence_pending", "operation_id", task.OperationID, "rpc_code", s.peer.FailureCode(err))
		s.repository.Retry(ctx, task.ID, instance, "FENCE_NOT_CONFIRMED")
		return
	}
	s.finish(ctx, task, instance, binding, ack.ResultDigest)
}

````

## 第 7 步：Closure.finish

插入位置：第 6 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func (s *Closure) finish(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.finishGovernance(ctx, task, instance, binding, digest)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.LockOwnership(ctx, tx, task.ID)</code>；<code>s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.ClosureTask</code> | 使用 <code>domain.ClosureTask</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>在此声明中的实际使用：<code>s.finishGovernance(ctx, task, instance, binding, digest)</code>；<code>task.ID</code>；<code>task.OperationID</code>；<code>s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>s.finishGovernance(ctx, task, instance, binding, digest)</code>；<code>ownership.Instance != instance</code> |
| <code>binding</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>binding.Kind</code>；<code>s.finishGovernance(ctx, task, instance, binding, digest)</code>；<code>binding.OrganizationID</code>；<code>binding.Epoch</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.finishGovernance(ctx, task, instance, binding, digest)</code>；<code>s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal)</code> |

**跟着本段代码走一遍：**

- 检查 <code>binding.Kind == &quot;PLATFORM_GOVERNANCE_REVOKE&quot;</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; ownership.Instance != instance &#124;&#124; ownership.State != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>isCredential &#124;&#124; binding.OrganizationID != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; oldEpoch == ^uint64(0)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>isCredential &amp;&amp; s.repository.FinishCredential(ctx, tx, binding.UserID, oldEpoch) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>binding.OrganizationID != &quot;&quot;</code>。
- 检查 <code>s.repository.FinishMemberRole(ctx, tx, binding.OrganizationID, binding.UserID, binding.Role, oldEpoch+1) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>s.repository.FinishMemberDeparture(ctx, tx, binding.OrganizationID, binding.UserID, oldEpoch+1) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>s.repository.FinishOrganizationTransition(ctx, tx, task.OperationID) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=3185:4922 -->
````go
func (s *Closure) finish(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	if binding.Kind == "PLATFORM_GOVERNANCE_REVOKE" {
		s.finishGovernance(ctx, task, instance, binding, digest)
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	isCredential := binding.Kind == "CHANGE_PASSWORD" || binding.Kind == "RESET_PASSWORD" || binding.Kind == "CHANGE_EMAIL"
	var oldEpoch uint64
	if isCredential || binding.OrganizationID != "" {
		oldEpoch, err = strconv.ParseUint(binding.Epoch, 10, 64)
		if err != nil || oldEpoch == ^uint64(0) {
			return
		}
	}
	session, _ := hex.DecodeString(binding.SessionHash)
	poll := "/v1/account/operations/" + task.OperationID
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED", PollUrl: &poll})
	if s.repository.FinishBase(ctx, tx, task, binding, session, digest, terminal) != nil {
		return
	}
	if isCredential && s.repository.FinishCredential(ctx, tx, binding.UserID, oldEpoch) != nil {
		return
	}
	if binding.OrganizationID != "" {
		switch binding.Kind {
		case "ORG_ROLE", "ORG_OWNER_TRANSFER":
			if s.repository.FinishMemberRole(ctx, tx, binding.OrganizationID, binding.UserID, binding.Role, oldEpoch+1) != nil {
				return
			}
		case "ORG_REMOVE", "ORG_LEAVE":
			if s.repository.FinishMemberDeparture(ctx, tx, binding.OrganizationID, binding.UserID, oldEpoch+1) != nil {
				return
			}
		}
		if s.repository.FinishOrganizationTransition(ctx, tx, task.OperationID) != nil {
			return
		}
````

## 第 8 步：Closure.finish（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 142 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>binding.Kind == &quot;ORG_OWNER_TRANSFER&quot;</code>。
- 检查 <code>s.repository.CreateOwnerTransferNotice(ctx, tx, task.OperationID, binding.OrganizationID, binding.UserID) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 8 bytes=4922:5296 -->
````go
		if binding.Kind == "ORG_OWNER_TRANSFER" {
			if s.repository.CreateOwnerTransferNotice(ctx, tx, task.OperationID, binding.OrganizationID, binding.UserID) != nil {
				return
			}
		}
	}
	// commit响应丢失仍是未知事实；下轮恢复读原记录，不能补写假的终态。
	_ = tx.Commit()
}

// closure_enqueue.go：保留原实现，按业务合并文件。

````

## 第 9 步：EnqueueSourceClosure

插入位置：第 8 步代码之后，紧接追加；完成稿约第 154 行。

本步声明：`func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |
| <code>actor</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |
| <code>payload</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>json.Marshal(payload)</code>；<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5296:5587 -->
````go
func EnqueueSourceClosure(ctx context.Context, tx *sql.Tx, id, actor string, payload domain.SessionClosure) error {
	target, _ := json.Marshal(payload)
	return repository.EnqueueSourceClosure(ctx, tx, id, actor, payload, target)
}

// logout.go：保留原实现，按业务合并文件。

````

## 第 10 步：Logout

插入位置：第 9 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`type Logout struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。 |
| <code>receipts</code> | <code>*repository.OperationReceipts</code> | 使用 <code>repository.OperationReceipts</code> 类型；其定义处的职责：查询账号操作原回执与当前凭据，为退出/改密恢复和RPC固定结果提供持久事实。 |
| <code>mutations</code> | <code>*repository.Mutation</code> | 使用 <code>repository.Mutation</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。 |
| <code>closures</code> | <code>*repository.Closure</code> | 使用 <code>repository.Closure</code> 类型；其定义处的职责：持久化来源关闭、领取恢复任务、保存真实Doc ACK并完成会话或成员收尾。 |

<!-- write-step: 10 bytes=5587:5786 -->
````go
type Logout struct {
	db             *sql.DB
	authentication *Authentication
	receipts       *repository.OperationReceipts
	mutations      *repository.Mutation
	closures       *repository.Closure
}

````

## 第 11 步：NewLogout

插入位置：第 10 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func NewLogout(db *sql.DB, authentication *Authentication, receipts *repository.OperationReceipts, mutations *repository.Mutation, closures *repository.Closure) *Logout`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db:             db</code> |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>receipts</code> | <code>*repository.OperationReceipts</code> | 使用 <code>repository.OperationReceipts</code> 类型；其定义处的职责：查询账号操作原回执与当前凭据，为退出/改密恢复和RPC固定结果提供持久事实。<br>在此声明中的实际使用：<code>receipts:       receipts</code> |
| <code>mutations</code> | <code>*repository.Mutation</code> | 使用 <code>repository.Mutation</code> 类型；其定义处的职责：提供身份准入、来源门闩和幂等写入所需的权威 SQL，并读取授权关闭事实快照。<br>在此声明中的实际使用：<code>mutations:      mutations</code> |
| <code>closures</code> | <code>*repository.Closure</code> | 使用 <code>repository.Closure</code> 类型；其定义处的职责：持久化来源关闭、领取恢复任务、保存真实Doc ACK并完成会话或成员收尾。<br>在此声明中的实际使用：<code>closures:       closures</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Logout</code> | 实际返回表达式：<code>&amp;Logout{ 		db:             db, 		authentication: authentication, 		receipts:       receipts, 		mutations:      mutations, 		closures:       closures, 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Logout{ 		db:             db, 		authentication: authentication, 		receipts:       receipts, 		mutations:      mutations, 		closures:       closures, 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5786:6121 -->
````go
func NewLogout(db *sql.DB, authentication *Authentication, receipts *repository.OperationReceipts, mutations *repository.Mutation, closures *repository.Closure) *Logout {
	return &Logout{
		db:             db,
		authentication: authentication,
		receipts:       receipts,
		mutations:      mutations,
		closures:       closures,
	}
}

````

## 第 12 步：Logout.Resolve

插入位置：第 11 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func (s *Logout) Resolve(ctx context.Context, id, primaryToken, recoveryToken string) (domain.LogoutSelection, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authentication.Session(ctx, primaryToken)</code>；<code>s.receipts.Lookup(ctx, id)</code>；<code>s.authentication.Session(ctx, recoveryToken)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.receipts.Lookup(ctx, id)</code> |
| <code>primaryToken</code> | <code>string</code> | 在此声明中的实际使用：<code>s.authentication.Session(ctx, primaryToken)</code>；<code>Token: primaryToken</code> |
| <code>recoveryToken</code> | <code>string</code> | 在此声明中的实际使用：<code>selection.Token = recoveryToken</code>；<code>s.authentication.Session(ctx, recoveryToken)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.LogoutSelection</code> | 使用 <code>domain.LogoutSelection</code> 类型；其定义处的职责：表达原操作回执、退出会话选择和只含状态的最小进度。<br>实际返回表达式：<code>selection</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>；<code>nil</code>；<code>lookup</code> |

**跟着本段代码走一遍：**

- 检查 <code>lookup == nil</code>。
- 检查 <code>sessionError != nil &#124;&#124; hex.EncodeToString(session.ID) != receipt.SessionHash</code>。
- 检查 <code>sessionError != nil</code>。条件成立时执行 <code>return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>receipt.ActorID != session.UserID &#124;&#124; receipt.Kind != &quot;SESSION_LOGOUT&quot; &#124;&#124; receipt.SessionHash != hex.EncodeToString(session.ID)</code>。条件成立时执行 <code>return selection, domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;OPERATION_ID_CONFLICT&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return selection, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>lookup != sql.ErrNoRows</code>。条件成立时执行 <code>return selection, lookup</code>。当前函数在这里结束。
- 检查 <code>sessionError != nil &#124;&#124; session.State != &quot;ACTIVE&quot; &#124;&#124; !session.Expires.After(time.Now())</code>。条件成立时执行 <code>return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return selection, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6121:7528 -->
````go
func (s *Logout) Resolve(ctx context.Context, id, primaryToken, recoveryToken string) (domain.LogoutSelection, error) {
	session, sessionError := s.authentication.Session(ctx, primaryToken)
	selection := domain.LogoutSelection{Session: session, Token: primaryToken}
	receipt, lookup := s.receipts.Lookup(ctx, id)
	if lookup == nil {
		// 只选择原已登记操作的会话证明；关闭的会话不能通过此分支发起新修改。
		if sessionError != nil || hex.EncodeToString(session.ID) != receipt.SessionHash {
			selection.Token = recoveryToken
			session, sessionError = s.authentication.Session(ctx, recoveryToken)
			selection.Session = session
		}
		if sessionError != nil {
			return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
		}
		if receipt.ActorID != session.UserID || receipt.Kind != "SESSION_LOGOUT" || receipt.SessionHash != hex.EncodeToString(session.ID) {
			return selection, domain.RuleViolation{Kind: domain.StateConflict, Code: "OPERATION_ID_CONFLICT"}
		}
		selection.Registered = true
		selection.ReceiptState = receipt.State
		return selection, nil
	}
	if lookup != sql.ErrNoRows {
		return selection, lookup
	}
	if sessionError != nil || session.State != "ACTIVE" || !session.Expires.After(time.Now()) {
		return selection, domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	return selection, nil
}

````

## 第 13 步：Logout.Prepare

插入位置：第 12 步代码之后，紧接追加；完成稿约第 209 行。

本步声明：`func (s *Logout) Prepare(ctx context.Context, id string, session domain.Session) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.mutations.LockUser(ctx, tx, session.UserID)</code>；<code>s.closures.LockExpirySession(ctx, tx, session.ID)</code>；<code>s.receipts.ConcurrentLogout(ctx, tx, id)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>s.receipts.ConcurrentLogout(ctx, tx, id)</code>；<code>OperationID: id</code> |
| <code>session</code> | <code>domain.Session</code> | 使用 <code>domain.Session</code> 类型；其定义处的职责：把浏览器秘密与持久会话事实分离。<br>在此声明中的实际使用：<code>session.UserID</code>；<code>session.ID</code>；<code>session.CredentialEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>state</code>；<code>&quot;PREPARING&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;LOGOUT_RECHECK_FAILED&quot;)</code>；<code>domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;LOGOUT_RECHECK_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>credential.State != &quot;ACTIVE&quot; &#124;&#124; credential.Epoch != session.CredentialEpoch &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code>。当前函数在这里结束。
- 检查 <code>current.State != &quot;ACTIVE&quot;</code>。
- 检查 <code>err == nil &amp;&amp; bound == hex.EncodeToString(session.ID)</code>。条件成立时执行 <code>return state, nil</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;&quot;, domain.RuleViolation{Kind: domain.SessionRequired, Code: &quot;SESSION_INVALID&quot;}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7528:9331 -->
````go
func (s *Logout) Prepare(ctx context.Context, id string, session domain.Session) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// 与登录/凭据修改相同，先锁用户再锁会话，防止相反锁顺序导致死锁。
	credential, err := s.mutations.LockUser(ctx, tx, session.UserID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	current, err := s.closures.LockExpirySession(ctx, tx, session.ID)
	if err != nil {
		return "", errors.New("LOGOUT_RECHECK_FAILED")
	}
	if credential.State != "ACTIVE" || credential.Epoch != session.CredentialEpoch || !current.Expires.After(time.Now()) {
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	if current.State != "ACTIVE" {
		// 初次查询没有receipt的同号请求，等锁后必须重读持久绑定。
		bound, state, err := s.receipts.ConcurrentLogout(ctx, tx, id)
		if err == nil && bound == hex.EncodeToString(session.ID) {
			return state, nil
		}
		return "", domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
	}
	digest := hex.EncodeToString(session.ID)
	scope := "session:" + digest
	target, _ := json.Marshal(map[string]string{"session_hash": digest, "scope_key": scope})
	fingerprint := sha256.Sum256([]byte("SESSION_LOGOUT\n" + session.UserID + "\n" + digest))
	payload, _ := json.Marshal(map[string]string{
		"session_hash": digest,
		"scope_key":    scope,
		"closed_epoch": strconv.FormatUint(current.Epoch, 10),
	})
	preparation := domain.SessionClosePreparation{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Epoch:       current.Epoch,
		OperationID: id,
		Scope:       scope,
		Target:      target,
		Payload:     payload,
		Fingerprint: fingerprint[:],
	}
````

## 第 14 步：Logout.Prepare（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 254 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>s.closures.PrepareSessionClose(ctx, tx, preparation) != nil &#124;&#124; tx.Commit() != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;LOGOUT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return &quot;PREPARING&quot;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9331:9566 -->
````go
	if s.closures.PrepareSessionClose(ctx, tx, preparation) != nil || tx.Commit() != nil {
		return "", errors.New("LOGOUT_COMMIT_UNKNOWN")
	}
	return "PREPARING", nil
}

// session_expiry.go：保留原实现，按业务合并文件。

````

## 第 15 步：Closure.EnqueueExpired

插入位置：第 14 步代码之后，紧接追加；完成稿约第 262 行。

浏览器可能断电；会话自然到期也必须形成持久关闭，不能等待用户点击退出。

本步声明：`func (s *Closure) EnqueueExpired(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, time.Second)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>s.repository.LockExpiryUser(ctx, tx, session.UserID) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;ACTIVE&quot; &#124;&#124; current.Expires.After(time.Now()) &#124;&#124; current.Epoch == ^uint64(0)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>s.repository.PrepareSessionClose(ctx, tx, value) != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 15 bytes=9566:11098 -->
````go
// 浏览器可能断电；会话自然到期也必须形成持久关闭，不能等待用户点击退出。
func (s *Closure) EnqueueExpired(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	session, err := s.repository.ExpiredSession(ctx)
	if err != nil {
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	// 与显式退出一样先锁用户再锁会话；池外发现过期不代表事务内仍可关闭。
	if s.repository.LockExpiryUser(ctx, tx, session.UserID) != nil {
		return
	}
	current, err := s.repository.LockExpirySession(ctx, tx, session.ID)
	if err != nil || current.State != "ACTIVE" || current.Expires.After(time.Now()) || current.Epoch == ^uint64(0) {
		return
	}
	digest := hex.EncodeToString(session.ID)
	scope := "session:" + digest
	operationID := "expiry-" + digest
	target, _ := json.Marshal(map[string]string{"session_hash": digest, "scope_key": scope})
	payload, _ := json.Marshal(domain.SessionClosure{SessionHash: digest, Scope: scope, Epoch: strconv.FormatUint(current.Epoch, 10)})
	fingerprint := sha256.Sum256([]byte("SESSION_EXPIRY\n" + digest))
	value := domain.SessionClosePreparation{
		SessionID:   session.ID,
		UserID:      session.UserID,
		Epoch:       current.Epoch,
		OperationID: operationID,
		Scope:       scope,
		Target:      target,
		Payload:     payload,
		Fingerprint: fingerprint[:],
	}
	if s.repository.PrepareSessionClose(ctx, tx, value) != nil {
		return
	}
	_ = tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/session_closure_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
