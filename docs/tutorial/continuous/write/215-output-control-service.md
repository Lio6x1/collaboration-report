# 分步编写 `services/document/internal/logic/document/output_control_service.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/215-output-control-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

协调Document本地流、Edge和Node的来源撤权与真实排空。

cancel只发出请求，缓冲写入仍可能继续；Fence成功必须等待所有真实done和原参与者ACK。

写之前先按这条执行路线理解各段的连接：

1. Close先prepare关准入，再stopEdgeAndSettle以及Node关闭，finish落定
2. LocalOutputs.Stop取消匹配来源并等done
3. Register在原事务写租约
4. recoverExited核PID归属。

本文件需要守住的边界：

- 即使没有在线handler仍要给Edge关闭水位以拦截迟到open
- 不能用心跳或TTL猜旧进程已退出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/output_control_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/output_control_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/output_control_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/output_control_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:499 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// collaboration_output.go：保留原实现，按业务合并文件。

````

## 第 2 步：Outputs.RegisterCollaboration

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

长连接来源从已兑换准入读取，不能接受Node自报更少的来源绕过关闭。

本步声明：`func (m *Outputs) RegisterCollaboration(ctx context.Context, request domain.CollaborationOutput) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.repository.AdmissionSources(ctx, request.Admission)</code>；<code>m.EdgeInstance(ctx, request.Handler)</code>；<code>m.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, sources)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>domain.CollaborationOutput</code> | 使用 <code>domain.CollaborationOutput</code> 类型；其定义处的职责：CollaborationOutput引用已兑换准入，调用方只提交固定绑定，不能自定权限来源。<br>在此声明中的实际使用：<code>request.Admission</code>；<code>request.Handler</code>；<code>request.Grant</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>lease.ID</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;ADMISSION_CLOSED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;GRANT_BINDING_INVALID&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>m.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;OPEN&quot; &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.PermissionDenied, &quot;ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(current.TargetJSON, &amp;binding) != nil &#124;&#124; binding.GrantID != request.Grant &#124;&#124; binding.Target == nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.PermissionDenied, &quot;GRANT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;OUTPUT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>m.edge.Call(ctx, &quot;/infra/output/open&quot;, lease, &amp;ack) != nil &#124;&#124; ack.State != &quot;OPEN&quot; &#124;&#124; ack.ID != edge.ID</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=499:2553 -->
````go
// 长连接来源从已兑换准入读取，不能接受Node自报更少的来源绕过关闭。
func (m *Outputs) RegisterCollaboration(ctx context.Context, request domain.CollaborationOutput) (string, error) {
	sources, err := m.repository.AdmissionSources(ctx, request.Admission)
	if err != nil {
		return "", err
	}
	edge, err := m.EdgeInstance(ctx, request.Handler)
	if err != nil || edge == nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return "", err
	}
	current, err := repository.LockedAdmission(ctx, tx, request.Admission)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return "", domain.Violation(domain.PermissionDenied, "ADMISSION_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != request.Grant || binding.Target == nil {
		return "", domain.Violation(domain.PermissionDenied, "GRANT_BINDING_INVALID")
	}
	if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
		return "", err
	}
	lease := domain.OutputLease{ID: platform.NewTraceID(), Instance: edge.ID, Handler: request.Handler, Sources: SourcesWire(sources), Document: binding.Target.DocumentId, Admission: request.Admission, Authority: "collaboration"}
	if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
		return "", err
	}
	if err := repository.MarkWebSocketDelivery(ctx, tx, lease.ID, current.Expires); err != nil {
		return "", err
	}
	if tx.Commit() != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	// Node自己的socket停止不足以证明Edge缓冲排空；开放也必须取得真实Edge确认。
	var ack struct {
		State string `json:"state"`
		ID    string `json:"instance_id"`
	}
	if m.edge.Call(ctx, "/infra/output/open", lease, &ack) != nil || ack.State != "OPEN" || ack.ID != edge.ID {
````

## 第 3 步：Outputs.RegisterCollaboration（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 67 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return lease.ID, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2553:2729 -->
````go
		return "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return lease.ID, nil
}

// fence.go：保留原实现，按业务合并文件。

````

## 第 4 步：FencePeer

插入位置：第 3 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`type FencePeer interface {`。

**这一段为什么这样写：** Doc在来源撤销时协调Node停止旧权限输出，确保确认发生在停止之后。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Close</code> | <code>(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)</code> | ctx与原来源关闭请求发送给Node，返回真实停止输出的确认；网络失败不能当作连接已关。 |

<!-- write-step: 4 bytes=2729:2825 -->
````go
type FencePeer interface {
	Close(context.Context, *pb.FenceRequest) (*pb.FenceResult, error)
}
````

## 第 5 步：Fences

插入位置：第 4 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`type Fences struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Fences</code> | 使用 <code>repository.Fences</code> 类型；其定义处的职责：保存关闭门闩、准入关闭、两参与者ACK及进程/输出结算事实。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |
| <code>node</code> | <code>FencePeer</code> | 使用 <code>FencePeer</code> 类型；其定义处的职责：Doc在来源撤销时协调Node停止旧权限输出，确保确认发生在停止之后。 |

<!-- write-step: 5 bytes=2825:2943 -->
````go
type Fences struct {
	db         *sql.DB
	repository *repository.Fences
	outputs    *Outputs
	node       FencePeer
}

````

## 第 6 步：NewFences

插入位置：第 5 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func NewFences(db *sql.DB, repository *repository.Fences, outputs *Outputs, node FencePeer) *Fences`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repository</code> | <code>*repository.Fences</code> | 使用 <code>repository.Fences</code> 类型；其定义处的职责：保存关闭门闩、准入关闭、两参与者ACK及进程/输出结算事实。<br>在此声明中的实际使用：<code>repository: repository</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |
| <code>node</code> | <code>FencePeer</code> | 使用 <code>FencePeer</code> 类型；其定义处的职责：Doc在来源撤销时协调Node停止旧权限输出，确保确认发生在停止之后。<br>在此声明中的实际使用：<code>node: node</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Fences</code> | 实际返回表达式：<code>&amp;Fences{db: db, repository: repository, outputs: outputs, node: node}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Fences{db: db, repository: repository, outputs: outputs, node: node}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2943:3126 -->
````go
func NewFences(db *sql.DB, repository *repository.Fences, outputs *Outputs, node FencePeer) *Fences {
	return &Fences{db: db, repository: repository, outputs: outputs, node: node}
}

````

## 第 7 步：Fences.Close

插入位置：第 6 步代码之后，紧接追加；完成稿约第 88 行。

Close先关闭准入，再等待真实两段输出与Node ACK，持久确认之前不返回成功。

本步声明：`func (s *Fences) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.prepare(ctx, request)</code>；<code>s.outputs.Local().Stop(ctx, request.ClosedScopes&#91;0&#93;)</code>；<code>s.stopEdgeAndSettle(ctx, request)</code>；<code>s.node.Close(ctx, request)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>s.prepare(ctx, request)</code>；<code>platform.BootstrapAck(request, &quot;document&quot;)</code>；<code>request.ClosedScopes</code>；<code>s.stopEdgeAndSettle(ctx, request)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 实际返回表达式：<code>nil</code>；<code>platform.BootstrapAck(request, &quot;document&quot;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;LOCAL_OUTPUT_STOP_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_STOP_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>done</code>。条件成立时执行 <code>return platform.BootstrapAck(request, &quot;document&quot;), nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;LOCAL_OUTPUT_STOP_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_STOP_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;NODE_FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return platform.BootstrapAck(request, &quot;document&quot;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3126:4078 -->
````go
// Close先关闭准入，再等待真实两段输出与Node ACK，持久确认之前不返回成功。
func (s *Fences) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	done, err := s.prepare(ctx, request)
	if err != nil {
		return nil, err
	}
	if done {
		return platform.BootstrapAck(request, "document"), nil
	}
	if err := s.outputs.Local().Stop(ctx, request.ClosedScopes[0]); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "LOCAL_OUTPUT_STOP_UNKNOWN")
	}
	if err := s.stopEdgeAndSettle(ctx, request); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_STOP_UNKNOWN")
	}
	ack, err := s.node.Close(ctx, request)
	if err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "NODE_FENCE_UNAVAILABLE")
	}
	if err := s.finish(ctx, request, ack); err != nil {
		return nil, err
	}
	return platform.BootstrapAck(request, "document"), nil
}
````

## 第 8 步：Fences.prepare

插入位置：第 7 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func (s *Fences) prepare(ctx context.Context, request *pb.FenceRequest) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.BlockFenceScope(ctx, tx, closed.ScopeKey, closed.Epoch, request.RevocationId)</code>；<code>repository.AccountRetirementFenceCancelled(ctx, tx, request.RevocationId, closed.ScopeKey, closed.Epoch)</code>；<code>repository.LockedFence(ctx, tx, request.RevocationId)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>request.ClosedScopes</code>；<code>request.RevocationId</code>；<code>request.TargetDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_CANCELLED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;FENCE_ID_CONFLICT&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>cancelled</code>。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETIREMENT_CANCELLED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>json.Unmarshal(current.TargetPermission, &amp;prior) != nil &#124;&#124; prior&#91;&quot;bootstrap_target_digest&quot;&#93; != binding&#91;&quot;bootstrap_target_digest&quot;&#93; &#124;&#124; prior&#91;&quot;generation&quot;&#93; != &quot;1&quot; &#124;&#124; current.Scope !=  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false, domain.Violation(domain.StateConflict, &quot;FENCE_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return false, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 8 bytes=4078:6006 -->
````go
func (s *Fences) prepare(ctx context.Context, request *pb.FenceRequest) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	closed := request.ClosedScopes[0]
	// 新准入与关闭器都遵循gate→authorization_change，不能交叉持锁。
	if err := repository.BlockFenceScope(ctx, tx, closed.ScopeKey, closed.Epoch, request.RevocationId); err != nil {
		return false, err
	}
	// 原Account操作已权威拒绝并撤回预留时，迟到的同号Fence必须整体回滚，
	// 不能再次封住已恢复的身份来源。常规会话/凭据关闭不受该专项终态影响。
	cancelled, err := repository.AccountRetirementFenceCancelled(ctx, tx, request.RevocationId, closed.ScopeKey, closed.Epoch)
	if err != nil {
		return false, err
	}
	if cancelled {
		return false, domain.Violation(domain.StateConflict, "ACCOUNT_RETIREMENT_CANCELLED")
	}
	binding := map[string]string{"bootstrap_target_digest": hex.EncodeToString(request.TargetDigest), "generation": "1"}
	current, err := repository.LockedFence(ctx, tx, request.RevocationId)
	if err == nil {
		var prior map[string]string
		if json.Unmarshal(current.TargetPermission, &prior) != nil || prior["bootstrap_target_digest"] != binding["bootstrap_target_digest"] || prior["generation"] != "1" || current.Scope != closed.ScopeKey || current.Epoch != closed.Epoch {
			return false, domain.Violation(domain.StateConflict, "FENCE_ID_CONFLICT")
		}
		if current.State == "SUCCEEDED" {
			return true, nil
		}
	} else if err != sql.ErrNoRows {
		return false, err
	} else {
		encoded, _ := json.Marshal(binding)
		if err := repository.InsertFence(ctx, tx, request.RevocationId, closed.ScopeKey, closed.Epoch, encoded); err != nil {
			return false, err
		}
	}
	if err := repository.CloseSourceAdmissions(ctx, tx, closed.ScopeKey, closed.Epoch); err != nil {
		return false, err
	}
	return false, tx.Commit()
}
````

## 第 9 步：Fences.finish

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func (s *Fences) finish(ctx context.Context, request *pb.FenceRequest, node *pb.FenceResult) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.RecordNodeAck(ctx, tx, request.RevocationId, scope.ScopeKey, scope.Epoch, node.ResultDigest)</code>；<code>repository.CompleteFence(ctx, tx, request.RevocationId)</code> |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>platform.MatchesBootstrapAck(request, node, &quot;collaboration&quot;)</code>；<code>request.ClosedScopes</code>；<code>request.RevocationId</code> |
| <code>node</code> | <code>*pb.FenceResult</code> | 在此声明中的实际使用：<code>platform.MatchesBootstrapAck(request, node, &quot;collaboration&quot;)</code>；<code>node.ResultDigest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;NODE_FENCE_NOT_CONFIRMED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.MatchesBootstrapAck(request, node, &quot;collaboration&quot;)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;NODE_FENCE_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FENCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6006:7016 -->
````go
func (s *Fences) finish(ctx context.Context, request *pb.FenceRequest, node *pb.FenceResult) error {
	if !platform.MatchesBootstrapAck(request, node, "collaboration") {
		return domain.Violation(domain.TemporarilyUnavailable, "NODE_FENCE_NOT_CONFIRMED")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := request.ClosedScopes[0]
	if err := repository.RecordNodeAck(ctx, tx, request.RevocationId, scope.ScopeKey, scope.Epoch, node.ResultDigest); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_UNAVAILABLE")
	}
	if err := repository.CompleteFence(ctx, tx, request.RevocationId); err != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_COMMIT_UNKNOWN")
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return nil
}

// fence_outputs.go：保留原实现，按业务合并文件。

````

## 第 10 步：Fences.stopEdgeAndSettle

插入位置：第 9 步代码之后，紧接追加；完成稿约第 179 行。

即使没有在线handler，固定Edge参与者也接收关闭水位，阻止提交前登记而迟到的open。

本步声明：`func (s *Fences) stopEdgeAndSettle(ctx context.Context, req *pb.FenceRequest) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.repository.Unsettled(ctx)</code>；<code>s.outputs.EdgeCall(ctx, &quot;/infra/output/fence&quot;, input, &amp;ack)</code>；<code>s.outputs.Settle(ctx, lease.ID, lease.Instance)</code>；<code>s.repository.RecordEdgeAck(ctx, req.RevocationId, closed.ScopeKey, closed.Epoch, digest)</code> |
| <code>req</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>req.ClosedScopes</code>；<code>req.RevocationId</code>；<code>req.TargetDigest</code>；<code>platform.BootstrapDigest(req, &quot;edge&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>；<code>errors.New(&quot;OUTPUT_INSTANCE_UNKNOWN&quot;)</code>；<code>errors.New(&quot;EDGE_ACK_BINDING_INVALID&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(record.Payload, &amp;lease) != nil &#124;&#124; lease.ID != record.ID</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>source.Key == closed.ScopeKey &amp;&amp; epoch &lt;= closed.Epoch</code>。
- 检查 <code>lease.Instance != edgeID &amp;&amp; (lease.Instance != s.outputs.Self().ID &#124;&#124; !s.outputs.Local().Has(lease.ID))</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_INSTANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>ack.State != &quot;OUTPUT_STOPPED&quot; &#124;&#124; ack.ID != edgeID &#124;&#124; ack.Scope != closed.ScopeKey &#124;&#124; ack.Epoch != input&#91;&quot;epoch&quot;&#93; &#124;&#124; ack.Revocation != req.RevocationId &#124;&#124; ack.Digest != input&#91;&quot;target_digest&quot;&#93;</code>。条件成立时执行 <code>return errors.New(&quot;EDGE_ACK_BINDING_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=7016:9032 -->
````go
// 即使没有在线handler，固定Edge参与者也接收关闭水位，阻止提交前登记而迟到的open。
func (s *Fences) stopEdgeAndSettle(ctx context.Context, req *pb.FenceRequest) error {
	closed := req.ClosedScopes[0]
	records, err := s.repository.Unsettled(ctx)
	if err != nil {
		return err
	}
	var matched []domain.OutputLease
	for _, record := range records {
		var lease domain.OutputLease
		if json.Unmarshal(record.Payload, &lease) != nil || lease.ID != record.ID {
			return errors.New("OUTPUT_RECORD_INVALID")
		}
		for _, source := range lease.Sources {
			epoch, err := strconv.ParseUint(source.Epoch, 10, 64)
			if err != nil {
				return err
			}
			if source.Key == closed.ScopeKey && epoch <= closed.Epoch {
				matched = append(matched, lease)
				break
			}
		}
	}
	edgeID := strings.TrimSuffix(s.outputs.Self().ID, "-document") + "-edge"
	for _, lease := range matched {
		if lease.Instance != edgeID && (lease.Instance != s.outputs.Self().ID || !s.outputs.Local().Has(lease.ID)) {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
	}
	// 固定入口参与者即使当前没有在线handler也接收关闭水位，阻止commit前已登记但迟到的open。
	var ack struct {
		State      string `json:"state"`
		ID         string `json:"instance_id"`
		Scope      string `json:"scope_key"`
		Epoch      string `json:"epoch"`
		Revocation string `json:"revocation_id"`
		Digest     string `json:"target_digest"`
	}
	input := map[string]string{"instance_id": edgeID, "scope_key": closed.ScopeKey, "epoch": strconv.FormatUint(closed.Epoch, 10), "revocation_id": req.RevocationId, "target_digest": hex.EncodeToString(req.TargetDigest)}
	if err = s.outputs.EdgeCall(ctx, "/infra/output/fence", input, &ack); err != nil {
		return err
	}
	if ack.State != "OUTPUT_STOPPED" || ack.ID != edgeID || ack.Scope != closed.ScopeKey || ack.Epoch != input["epoch"] || ack.Revocation != req.RevocationId || ack.Digest != input["target_digest"] {
		return errors.New("EDGE_ACK_BINDING_INVALID")
	}
````

## 第 11 步：Fences.stopEdgeAndSettle（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 225 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>lease.Instance == s.outputs.Self().ID</code>。
- 此处通过 <code>return s.repository.RecordEdgeAck(ctx, req.RevocationId, closed.ScopeKey, closed.Epoch, digest)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=9032:9461 -->
````go
	for _, lease := range matched {
		if err = s.outputs.Settle(ctx, lease.ID, lease.Instance); err != nil {
			return err
		}
		if lease.Instance == s.outputs.Self().ID {
			s.outputs.Local().Remove(lease.ID)
		}
	}
	digest := platform.BootstrapDigest(req, "edge")
	return s.repository.RecordEdgeAck(ctx, req.RevocationId, closed.ScopeKey, closed.Epoch, digest)
}

// local_outputs.go：保留原实现，按业务合并文件。

````

## 第 12 步：localFlow

插入位置：第 11 步代码之后，紧接追加；完成稿约第 239 行。

Stop必须等待真实done；发出取消信号以后，输出仍可能持有缓冲或正阻塞写入。

本步声明：`type localFlow struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 本次本地输出依赖的权限来源和代次。 |
| <code>cancel</code> | <code>func()</code> | 请求停止正在发送的输出；调用后仍需等待done。 |
| <code>done</code> | <code>&lt;-chan struct{}</code> | 输出实际结束的只读信号，关闭来源时据此确认已停止。 |

<!-- write-step: 12 bytes=9461:9663 -->
````go
// Stop必须等待真实done；发出取消信号以后，输出仍可能持有缓冲或正阻塞写入。
type localFlow struct {
	sources []*pb.SourceEpoch
	cancel  func()
	done    <-chan struct{}
}
````

## 第 13 步：LocalOutputs

插入位置：第 12 步代码之后，紧接追加；完成稿约第 245 行。

本步声明：`type LocalOutputs struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mu</code> | <code>sync.Mutex</code> | 保护共享内存的互斥锁；加锁与解锁必须覆盖同一临界区，不能代替数据库事务或跨进程锁。 |
| <code>flows</code> | <code>map&#91;string&#93;localFlow</code> | 以输出编号索引本进程活动输出；由mu保护，不能当作跨进程永久结果。 |

<!-- write-step: 13 bytes=9663:9739 -->
````go
type LocalOutputs struct {
	mu    sync.Mutex
	flows map[string]localFlow
}

````

## 第 14 步：NewLocalOutputs

插入位置：第 13 步代码之后，紧接追加；完成稿约第 250 行。

本步声明：`func NewLocalOutputs() *LocalOutputs`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*LocalOutputs</code> | 实际返回表达式：<code>&amp;LocalOutputs{flows: map&#91;string&#93;localFlow{}}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;LocalOutputs{flows: map&#91;string&#93;localFlow{}}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9739:9832 -->
````go
func NewLocalOutputs() *LocalOutputs { return &LocalOutputs{flows: map[string]localFlow{}} }
````

## 第 15 步：LocalOutputs.Add

插入位置：第 14 步代码之后，紧接追加；完成稿约第 251 行。

本步声明：`func (m *LocalOutputs) Add(id string, sources []*pb.SourceEpoch, cancel func(), done <-chan struct{})`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>m.flows&#91;id&#93; = localFlow{sources, cancel, done}</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>m.flows&#91;id&#93; = localFlow{sources, cancel, done}</code> |
| <code>cancel</code> | <code>func()</code> | 在此声明中的实际使用：<code>m.flows&#91;id&#93; = localFlow{sources, cancel, done}</code> |
| <code>done</code> | <code>&lt;-chan struct{}</code> | 在此声明中的实际使用：<code>m.flows&#91;id&#93; = localFlow{sources, cancel, done}</code> |

**跟着本段代码走一遍：**

- 执行 <code>m.mu.Lock()</code>。
- 登记 <code>m.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>m.mu.Unlock()</code>。

<!-- write-step: 15 bytes=9832:10020 -->
````go
func (m *LocalOutputs) Add(id string, sources []*pb.SourceEpoch, cancel func(), done <-chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flows[id] = localFlow{sources, cancel, done}
}
````

## 第 16 步：LocalOutputs.Remove

插入位置：第 15 步代码之后，紧接追加；完成稿约第 256 行。

本步声明：`func (m *LocalOutputs) Remove(id string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>delete(m.flows, id)</code> |

**跟着本段代码走一遍：**

- 执行 <code>m.mu.Lock()</code>。
- 登记 <code>m.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>m.mu.Unlock()</code>。

<!-- write-step: 16 bytes=10020:10119 -->
````go
func (m *LocalOutputs) Remove(id string) { m.mu.Lock(); defer m.mu.Unlock(); delete(m.flows, id) }
````

## 第 17 步：LocalOutputs.Has

插入位置：第 16 步代码之后，紧接追加；完成稿约第 257 行。

本步声明：`func (m *LocalOutputs) Has(id string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>_, ok := m.flows&#91;id&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>ok</code> |

**跟着本段代码走一遍：**

- 执行 <code>m.mu.Lock()</code>。
- 登记 <code>m.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>m.mu.Unlock()</code>。
- 此处通过 <code>return ok</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=10119:10233 -->
````go
func (m *LocalOutputs) Has(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.flows[id]
	return ok
}
````

## 第 18 步：LocalOutputs.Stop

插入位置：第 17 步代码之后，紧接追加；完成稿约第 263 行。

本步声明：`func (m *LocalOutputs) Stop(ctx context.Context, closed *pb.SourceEpoch) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code> |
| <code>closed</code> | <code>*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>closed.ScopeKey</code>；<code>closed.Epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_STOP_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>m.mu.Lock()</code>。
- 检查 <code>s.ScopeKey == closed.ScopeKey &amp;&amp; s.Epoch &lt;= closed.Epoch</code>。
- 执行 <code>m.mu.Unlock()</code>。
- 此处通过 <code>return errors.New(&quot;OUTPUT_STOP_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=10233:10805 -->
````go
func (m *LocalOutputs) Stop(ctx context.Context, closed *pb.SourceEpoch) error {
	m.mu.Lock()
	var pending []localFlow
	for _, f := range m.flows {
		for _, s := range f.sources {
			if s.ScopeKey == closed.ScopeKey && s.Epoch <= closed.Epoch {
				pending = append(pending, f)
				break
			}
		}
	}
	m.mu.Unlock()
	for _, f := range pending {
		f.cancel()
	}
	for _, f := range pending {
		select {
		case <-f.done:
		case <-ctx.Done():
			return errors.New("OUTPUT_STOP_UNKNOWN")
		}
	}
	return nil
}

// output_recovery.go：保留原实现，按业务合并文件。

````

## 第 19 步：Outputs.recoverExited

插入位置：第 18 步代码之后，紧接追加；完成稿约第 290 行。

旧实例只能凭指定PID归属证据完成恢复；不以心跳、租约或TTL推测停止。

本步声明：`func (m *Outputs) recoverExited(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.repository.ActiveInstances(ctx)</code>；<code>m.process.Verify(ctx, instance.PID, role, owner)</code>；<code>m.db.BeginTx(ctx, nil)</code>；<code>m.repository.ConfirmExit(ctx, tx, instance.ID, state)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_INSTANCE_UNKNOWN&quot;)</code>；<code>errors.New(&quot;OUTPUT_RECOVERY_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>instance.ID == m.self.ID</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>strings.HasSuffix(instance.ID, &quot;-&quot;+candidate)</code>。
- 检查 <code>role == &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_INSTANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>m.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Rollback()</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_RECOVERY_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=10805:11807 -->
````go
// 旧实例只能凭指定PID归属证据完成恢复；不以心跳、租约或TTL推测停止。
func (m *Outputs) recoverExited(ctx context.Context) error {
	instances, err := m.repository.ActiveInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if instance.ID == m.self.ID {
			continue
		}
		role := ""
		for _, candidate := range []string{"document", "edge"} {
			if strings.HasSuffix(instance.ID, "-"+candidate) {
				role = candidate
			}
		}
		if role == "" {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
		owner := strings.TrimSuffix(instance.ID, "-"+role)
		state, err := m.process.Verify(ctx, instance.PID, role, owner)
		if err != nil {
			return err
		}
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := m.repository.ConfirmExit(ctx, tx, instance.ID, state); err != nil {
			tx.Rollback()
			return err
		}
		if tx.Commit() != nil {
			return errors.New("OUTPUT_RECOVERY_COMMIT_UNKNOWN")
		}
	}
	return nil
}
````

## 第 20 步：Outputs.Ready

插入位置：第 19 步代码之后，紧接追加；完成稿约第 328 行。

本步声明：`func (m *Outputs) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.repository.UnrecoveredCount(ctx)</code>；<code>m.repository.Pending(ctx)</code>；<code>m.repository.Settled(ctx, output.ID, output.Instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_RECOVERY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;OUTPUT_RECOVERY_REQUIRED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;OUTPUT_HANDLER_UNKNOWN&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>count != 0</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>output.Instance == m.self.ID</code>。
- 检查 <code>!m.local.Has(output.ID)</code>。
- 检查 <code>err != nil &#124;&#124; !settled</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_HANDLER_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>output.Instance != edgeID</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_INSTANCE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=11807:12762 -->
````go
func (m *Outputs) Ready(ctx context.Context) error {
	count, err := m.repository.UnrecoveredCount(ctx)
	if err != nil {
		return errors.New("OUTPUT_RECOVERY_UNAVAILABLE")
	}
	if count != 0 {
		return errors.New("OUTPUT_RECOVERY_REQUIRED")
	}
	pending, err := m.repository.Pending(ctx)
	if err != nil {
		return err
	}
	edgeID := strings.TrimSuffix(m.self.ID, "-document") + "-edge"
	for _, output := range pending {
		if output.Instance == m.self.ID {
			if !m.local.Has(output.ID) {
				// Pending之后可能完成持久结算并移除handler。旧快照不能把正常结束
				// 当成丢失处理器；只有原记录的提交成功事实才允许通过此竞争窗口。
				settled, err := m.repository.Settled(ctx, output.ID, output.Instance)
				if err != nil || !settled {
					return errors.New("OUTPUT_HANDLER_UNKNOWN")
				}
			}
		} else if output.Instance != edgeID {
			return errors.New("OUTPUT_INSTANCE_UNKNOWN")
		}
	}
	return nil
}

````

## 第 21 步：OutputRecoveryCode

插入位置：第 20 步代码之后，紧接追加；完成稿约第 358 行。

恢复日志只输出白名单边界码，避免驱动错误带出私有配置或SQL。

本步声明：`func OutputRecoveryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err.Error</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>err.Error()</code>；<code>&quot;OUTPUT_RECOVERY_UNKNOWN&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return err.Error()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &quot;OUTPUT_RECOVERY_UNKNOWN&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=12762:13256 -->
````go
// 恢复日志只输出白名单边界码，避免驱动错误带出私有配置或SQL。
func OutputRecoveryCode(err error) string {
	switch err.Error() {
	case "OUTPUT_EXIT_PROOF_UNAVAILABLE", "OUTPUT_EXIT_PROOF_DEADLINE", "OLD_OUTPUT_PROCESS_STILL_RUNNING", "OUTPUT_INSTANCE_UNKNOWN", "OUTPUT_RECOVERY_INVALID", "OUTPUT_RECOVERY_COMMIT_UNKNOWN":
		return err.Error()
	default:
		return "OUTPUT_RECOVERY_UNKNOWN"
	}
}

// output_registration.go：保留原实现，按业务合并文件。

````

## 第 22 步：ResponseRegistration

插入位置：第 21 步代码之后，紧接追加；完成稿约第 370 行。

本步声明：`type ResponseRegistration struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Local</code> | <code>domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。 |
| <code>Edge</code> | <code>*domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。 |

<!-- write-step: 22 bytes=13256:13347 -->
````go
type ResponseRegistration struct {
	Local domain.OutputLease
	Edge  *domain.OutputLease
}

````

## 第 23 步：Outputs.EdgeInstance

插入位置：第 22 步代码之后，紧接追加；完成稿约第 375 行。

本步声明：`func (m *Outputs) EdgeInstance(ctx context.Context, handler string) (*domain.OutputInstance, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.edge.Call(ctx, &quot;/infra/output/info&quot;, nil, &amp;instance)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>handler == &quot;&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>实际返回表达式：<code>nil</code>；<code>&amp;instance</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;EDGE_OUTPUT_INSTANCE_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>handler == &quot;&quot;</code>。条件成立时执行 <code>return nil, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>instance.ID != strings.TrimSuffix(m.self.ID, &quot;-document&quot;)+&quot;-edge&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;EDGE_OUTPUT_INSTANCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;instance, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=13347:13784 -->
````go
func (m *Outputs) EdgeInstance(ctx context.Context, handler string) (*domain.OutputInstance, error) {
	if handler == "" {
		return nil, nil
	}
	var instance domain.OutputInstance
	if err := m.edge.Call(ctx, "/infra/output/info", nil, &instance); err != nil {
		return nil, err
	}
	if instance.ID != strings.TrimSuffix(m.self.ID, "-document")+"-edge" {
		return nil, errors.New("EDGE_OUTPUT_INSTANCE_CHANGED")
	}
	return &instance, nil
}
````

## 第 24 步：SourcesWire

插入位置：第 23 步代码之后，紧接追加；完成稿约第 388 行。

本步声明：`func SourcesWire(sources []*pb.SourceEpoch) []domain.LeaseSource`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>len(sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>实际返回表达式：<code>list</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return list</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=13784:14051 -->
````go
func SourcesWire(sources []*pb.SourceEpoch) []domain.LeaseSource {
	list := make([]domain.LeaseSource, 0, len(sources))
	for _, s := range sources {
		list = append(list, domain.LeaseSource{Key: s.ScopeKey, Epoch: strconv.FormatUint(s.Epoch, 10)})
	}
	return list
}

````

## 第 25 步：Outputs.Register

插入位置：第 24 步代码之后，紧接追加；完成稿约第 396 行。

租约与业务读取/变更共用调用方的原tx；commit之前不开放入口输出。

本步声明：`func (m *Outputs) Register(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64, admission string, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Document: document</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>Admission: admission</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 由 <code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=14051:14470 -->
````go
// 租约与业务读取/变更共用调用方的原tx；commit之前不开放入口输出。
func (m *Outputs) Register(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document uint64, admission string, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, Admission: admission}, handler, edge)
}

````

## 第 26 步：Outputs.RegisterPublication

插入位置：第 25 步代码之后，紧接追加；完成稿约第 401 行。

本步声明：`func (m *Outputs) RegisterPublication(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document, epoch, release uint64, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>domain.TextID(document)</code>；<code>Document: document</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>epoch == 0</code>；<code>source.Epoch == epoch</code>；<code>domain.TextID(epoch)</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>release == 0</code>；<code>domain.TextID(release)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>edge == nil</code>；<code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 由 <code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_OUTPUT_BINDING_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>document == 0 &#124;&#124; epoch == 0 &#124;&#124; release == 0 &#124;&#124; edge == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_OUTPUT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>source != nil &amp;&amp; source.ScopeKey == &quot;publication:&quot;+domain.TextID(document) &amp;&amp; source.Epoch == epoch</code>。
- 检查 <code>!matched</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLIC_OUTPUT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=14470:15315 -->
````go
func (m *Outputs) RegisterPublication(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, document, epoch, release uint64, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if document == 0 || epoch == 0 || release == 0 || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_OUTPUT_BINDING_INVALID")
	}
	matched := false
	for _, source := range sources {
		if source != nil && source.ScopeKey == "publication:"+domain.TextID(document) && source.Epoch == epoch {
			matched = true
		}
	}
	if !matched {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_OUTPUT_BINDING_INVALID")
	}
	return m.registerResponse(ctx, tx, sources, domain.OutputLease{Document: document, PublicationEpoch: domain.TextID(epoch), Release: domain.TextID(release)}, handler, edge)
}

````

## 第 27 步：Outputs.registerResponse

插入位置：第 26 步代码之后，紧接追加；完成稿约第 417 行。

本步声明：`func (m *Outputs) registerResponse(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, sources)</code>；<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, sources)</code>；<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>LockSources(ctx, tx, sources)</code>；<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |
| <code>template</code> | <code>domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 由 <code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=15315:15675 -->
````go
func (m *Outputs) registerResponse(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	return m.writeResponseRegistration(ctx, tx, sources, template, handler, edge)
}

````

## 第 28 步：Outputs.writeResponseRegistration

插入位置：第 27 步代码之后，紧接追加；完成稿约第 424 行。

只登记原事务内的租约。普通路径自行锁来源；搜索路径必须持有私有锁证明。

本步声明：`func (m *Outputs) writeResponseRegistration(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertOutputLease(ctx, tx, o.Local)</code>；<code>repository.EnsureInstance(ctx, tx, *edge)</code>；<code>repository.InsertOutputLease(ctx, tx, lease)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertOutputLease(ctx, tx, o.Local)</code>；<code>repository.EnsureInstance(ctx, tx, *edge)</code>；<code>repository.InsertOutputLease(ctx, tx, lease)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>SourcesWire(sources)</code> |
| <code>template</code> | <code>domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>local := template</code>；<code>lease := template</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>lease.ID, lease.Instance, lease.Handler, lease.Sources = platform.NewTraceID(), edge.ID, handler, SourcesWire(sources)</code> |
| <code>edge</code> | <code>*domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>edge != nil</code>；<code>repository.EnsureInstance(ctx, tx, *edge)</code>；<code>edge.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseRegistration</code> | 实际返回表达式：<code>nil</code>；<code>o</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>edge != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return o, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=15675:16659 -->
````go
// 只登记原事务内的租约。普通路径自行锁来源；搜索路径必须持有私有锁证明。
func (m *Outputs) writeResponseRegistration(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch, template domain.OutputLease, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	id := platform.NewTraceID()
	local := template
	local.ID, local.Instance, local.Handler, local.Sources = id, m.self.ID, id, SourcesWire(sources)
	o := &ResponseRegistration{Local: local}
	if err := repository.InsertOutputLease(ctx, tx, o.Local); err != nil {
		return nil, err
	}
	if edge != nil {
		if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
			return nil, err
		}
		lease := template
		lease.ID, lease.Instance, lease.Handler, lease.Sources = platform.NewTraceID(), edge.ID, handler, SourcesWire(sources)
		if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
			return nil, err
		}
		o.Edge = &lease
	}
	return o, nil
}

````

## 第 29 步：Outputs.RegisterAccount

插入位置：第 28 步代码之后，紧接追加；完成稿约第 447 行。

本步声明：`func (m *Outputs) RegisterAccount(ctx context.Context, handler string, sources []*pb.SourceEpoch) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.EdgeInstance(ctx, handler)</code>；<code>m.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, sources)</code>；<code>repository.EnsureInstance(ctx, tx, *edge)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>m.EdgeInstance(ctx, handler)</code>；<code>Handler: handler</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>LockSources(ctx, tx, sources)</code>；<code>SourcesWire(sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;OUTPUT_COMMIT_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>m.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;OUTPUT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>m.edge.Call(ctx, &quot;/infra/output/open&quot;, lease, &amp;ack) != nil &#124;&#124; ack.State != &quot;OPEN&quot; &#124;&#124; ack.ID != edge.ID</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=16659:17872 -->
````go
func (m *Outputs) RegisterAccount(ctx context.Context, handler string, sources []*pb.SourceEpoch) error {
	edge, err := m.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return err
	}
	if err := repository.EnsureInstance(ctx, tx, *edge); err != nil {
		return err
	}
	lease := domain.OutputLease{ID: platform.NewTraceID(), Instance: edge.ID, Handler: handler, Sources: SourcesWire(sources), Authority: "account"}
	if err := repository.InsertOutputLease(ctx, tx, lease); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	var ack struct {
		State string `json:"state"`
		ID    string `json:"instance_id"`
	}
	if m.edge.Call(ctx, "/infra/output/open", lease, &ack) != nil || ack.State != "OPEN" || ack.ID != edge.ID {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return nil
}

// outputs.go：保留原实现，按业务合并文件。

````

## 第 30 步：EdgeOutput

插入位置：第 29 步代码之后，紧接追加；完成稿约第 482 行。

本步声明：`type EdgeOutput interface {`。

**这一段为什么这样写：** Edge输出控制接口，配合Doc本地输出停止形成完整下游关闭。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Call</code> | <code>(context.Context, string, any, any) error</code> | 参数依次为ctx、内部相对路径、输入对象、接收响应的对象；调用Edge登记/停止输出，处理失败才能决定后续步骤。 |

<!-- write-step: 30 bytes=17872:17949 -->
````go
type EdgeOutput interface {
	Call(context.Context, string, any, any) error
}
````

## 第 31 步：ProcessVerifier

插入位置：第 30 步代码之后，紧接追加；完成稿约第 485 行。

本步声明：`type ProcessVerifier interface {`。

**这一段为什么这样写：** 原输出实例无法响应时验证真实进程归属和状态，避免杀错进程或猜测输出已停止。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Verify</code> | <code>(context.Context, uint64, string, string) (string, error)</code> | 参数顺序为ctx、PID、预期服务角色、实例owner标识，返回核验的进程状态与错误；PID可能复用，不能单凭PID判断退出。 |

<!-- write-step: 31 bytes=17949:18050 -->
````go
type ProcessVerifier interface {
	Verify(context.Context, uint64, string, string) (string, error)
}

````

## 第 32 步：Outputs

插入位置：第 31 步代码之后，紧接追加；完成稿约第 489 行。

Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。

本步声明：`type Outputs struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>repository</code> | <code>*repository.Outputs</code> | 使用 <code>repository.Outputs</code> 类型；其定义处的职责：保存关闭门闩、准入关闭、两参与者ACK及进程/输出结算事实。 |
| <code>local</code> | <code>*LocalOutputs</code> | 本进程活动输出注册表，撤权时取消并等待实际结束；跨进程仍要Edge/Node确认。 |
| <code>self</code> | <code>domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。 |
| <code>edge</code> | <code>EdgeOutput</code> | 使用 <code>EdgeOutput</code> 类型；其定义处的职责：Edge输出控制接口，配合Doc本地输出停止形成完整下游关闭。 |
| <code>process</code> | <code>ProcessVerifier</code> | 使用 <code>ProcessVerifier</code> 类型；其定义处的职责：原输出实例无法响应时验证真实进程归属和状态，避免杀错进程或猜测输出已停止。 |

<!-- write-step: 32 bytes=18050:18349 -->
````go
// Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。
type Outputs struct {
	db         *sql.DB
	repository *repository.Outputs
	local      *LocalOutputs
	self       domain.OutputInstance
	edge       EdgeOutput
	process    ProcessVerifier
}

````

## 第 33 步：NewOutputs

插入位置：第 32 步代码之后，紧接追加；完成稿约第 499 行。

本步声明：`func NewOutputs(db *sql.DB, repository *repository.Outputs, self domain.OutputInstance, edge EdgeOutput, process ProcessVerifier) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>repository</code> | <code>*repository.Outputs</code> | 使用 <code>repository.Outputs</code> 类型；其定义处的职责：保存关闭门闩、准入关闭、两参与者ACK及进程/输出结算事实。<br>在此声明中的实际使用：<code>repository: repository</code> |
| <code>self</code> | <code>domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>self: self</code> |
| <code>edge</code> | <code>EdgeOutput</code> | 使用 <code>EdgeOutput</code> 类型；其定义处的职责：Edge输出控制接口，配合Doc本地输出停止形成完整下游关闭。<br>在此声明中的实际使用：<code>edge: edge</code> |
| <code>process</code> | <code>ProcessVerifier</code> | 使用 <code>ProcessVerifier</code> 类型；其定义处的职责：原输出实例无法响应时验证真实进程归属和状态，避免杀错进程或猜测输出已停止。<br>在此声明中的实际使用：<code>process: process</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>实际返回表达式：<code>&amp;Outputs{db: db, repository: repository, local: NewLocalOutputs(), self: self, edge: edge, process: process}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{db: db, repository: repository, local: NewLocalOutputs(), self: self, edge: edge, process: process}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=18349:18609 -->
````go
func NewOutputs(db *sql.DB, repository *repository.Outputs, self domain.OutputInstance, edge EdgeOutput, process ProcessVerifier) *Outputs {
	return &Outputs{db: db, repository: repository, local: NewLocalOutputs(), self: self, edge: edge, process: process}
}
````

## 第 34 步：Outputs.Self

插入位置：第 33 步代码之后，紧接追加；完成稿约第 502 行。

本步声明：`func (m *Outputs) Self() domain.OutputInstance`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>实际返回表达式：<code>m.self</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return m.self</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=18609:18674 -->
````go
func (m *Outputs) Self() domain.OutputInstance { return m.self }
````

## 第 35 步：Outputs.Local

插入位置：第 34 步代码之后，紧接追加；完成稿约第 503 行。

本步声明：`func (m *Outputs) Local() *LocalOutputs`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*LocalOutputs</code> | 实际返回表达式：<code>m.local</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return m.local</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=18674:18740 -->
````go
func (m *Outputs) Local() *LocalOutputs        { return m.local }
````

## 第 36 步：Outputs.EdgeCall

插入位置：第 35 步代码之后，紧接追加；完成稿约第 504 行。

本步声明：`func (m *Outputs) EdgeCall(ctx context.Context, path string, input, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.edge.Call(ctx, path, input, result)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>m.edge.Call(ctx, path, input, result)</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>m.edge.Call(ctx, path, input, result)</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>m.edge.Call(ctx, path, input, result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>m.edge.Call(ctx, path, input, result)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return m.edge.Call(ctx, path, input, result)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=18740:18876 -->
````go
func (m *Outputs) EdgeCall(ctx context.Context, path string, input, result any) error {
	return m.edge.Call(ctx, path, input, result)
}
````

## 第 37 步：Outputs.Initialize

插入位置：第 36 步代码之后，紧接追加；完成稿约第 507 行。

本步声明：`func (m *Outputs) Initialize(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.recoverExited(ctx)</code>；<code>m.db.BeginTx(ctx, nil)</code>；<code>repository.EnsureInstance(ctx, tx, m.self)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>m.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 37 bytes=18876:19191 -->
````go
func (m *Outputs) Initialize(ctx context.Context) error {
	if err := m.recoverExited(ctx); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := repository.EnsureInstance(ctx, tx, m.self); err != nil {
		return err
	}
	return tx.Commit()
}
````

## 第 38 步：Outputs.Settle

插入位置：第 37 步代码之后，紧接追加；完成稿约第 521 行。

本步声明：`func (m *Outputs) Settle(ctx context.Context, id, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>m.db.BeginTx(ctx, nil)</code>；<code>m.repository.Settle(ctx, tx, id, instance)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>m.repository.Settle(ctx, tx, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>m.repository.Settle(ctx, tx, id, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>m.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 38 bytes=19191:19461 -->
````go
func (m *Outputs) Settle(ctx context.Context, id, instance string) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := m.repository.Settle(ctx, tx, id, instance); err != nil {
		return err
	}
	return tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/output_control_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
