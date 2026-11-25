# 分步编写 `services/document/internal/server/rpc_server.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/270-rpc-server.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

聚合文档内部RPC适配器并映射领域失败到协议状态。

RPC注册早于HTTP业务装配，依赖只在启动Bind后可用，不能在请求中临时初始化数据库。

写之前先按这条执行路线理解各段的连接：

1. Document.Bind注入fence/ticket/resource/operation/public适配器
2. BindLifecycle与BindCandidateCleanup接生命周期
3. OperationResults按purpose分发原号查询
4. ProtectionError等翻译失败。

本文件需要守住的边界：

- 未实现方法沿生成契约返回UNIMPLEMENTED
- mTLS白名单不代替具体actor/target绑定校验。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/server' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/server/rpc_server.go') -and (Get-Item -LiteralPath 'services/document/internal/server/rpc_server.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/server/rpc_server.go')) { New-Item -ItemType File -Path 'services/document/internal/server/rpc_server.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:478 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// candidate_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：Document.BindCandidateCleanup

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func (s *Document) BindCandidateCleanup(cleanup *logic.CandidateCleanup)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cleanup</code> | <code>*logic.CandidateCleanup</code> | 使用 <code>logic.CandidateCleanup</code> 类型；其定义处的职责：为Node清理送审/合并临时候选签发绑定原工件的许可。<br>在此声明中的实际使用：<code>s.cleanup = cleanup</code> |

<!-- write-step: 2 bytes=478:576 -->
````go
func (s *Document) BindCandidateCleanup(cleanup *logic.CandidateCleanup) { s.cleanup = cleanup }

````

## 第 3 步：Document.AuthorizeCandidateCleanup

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func (s *Document) AuthorizeCandidateCleanup(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.cleanup.Authorize(ctx, request)</code> |
| <code>request</code> | <code>*pb.CleanupAuthorization</code> | 在此声明中的实际使用：<code>s.cleanup.Authorize(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CleanupPermit</code> | 实际返回表达式：<code>permit</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>lifecycleRPCError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return permit, lifecycleRPCError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=576:858 -->
````go
func (s *Document) AuthorizeCandidateCleanup(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error) {
	permit, err := s.cleanup.Authorize(ctx, request)
	return permit, lifecycleRPCError(err)
}

// document.go：保留原实现，按业务合并文件。

````

## 第 4 步：Document

插入位置：第 3 步代码之后，紧接追加；完成稿约第 27 行。

Document聚合已有RPC适配器，未实施的方法仍沿生成契约返回UNIMPLEMENTED。

本步声明：`type Document struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pb.UnimplementedDocumentInternalServer</code> | <code>pb.UnimplementedDocumentInternalServer</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>fences</code> | <code>*Fence</code> | 内部来源关闭RPC适配器。 |
| <code>tickets</code> | <code>*EditSessions</code> | 内部编辑会话/准入RPC适配器。 |
| <code>resources</code> | <code>*DraftResources</code> | 内部草稿资源保护RPC适配器。 |
| <code>operations</code> | <code>*OperationResults</code> | 内部原操作结果查询适配器。 |
| <code>public</code> | <code>*PublicSnapshot</code> | 内部公开快照RPC适配器。 |
| <code>lifecycle</code> | <code>*logic.LifecyclePlans</code> | 使用 <code>logic.LifecyclePlans</code> 类型；其定义处的职责：将Reserve、Prepare、Apply协议阶段绑定到同一账号生命周期计划。 |
| <code>cleanup</code> | <code>*logic.CandidateCleanup</code> | 使用 <code>logic.CandidateCleanup</code> 类型；其定义处的职责：为Node清理送审/合并临时候选签发绑定原工件的许可。 |

<!-- write-step: 4 bytes=858:1222 -->
````go
// Document聚合已有RPC适配器，未实施的方法仍沿生成契约返回UNIMPLEMENTED。
type Document struct {
	pb.UnimplementedDocumentInternalServer
	fences     *Fence
	tickets    *EditSessions
	resources  *DraftResources
	operations *OperationResults
	public     *PublicSnapshot
	lifecycle  *logic.LifecyclePlans
	cleanup    *logic.CandidateCleanup
}

````

## 第 5 步：NewDocument

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewDocument() *Document`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Document</code> | 使用 <code>Document</code> 类型；其定义处的职责：Document聚合已有RPC适配器，未实施的方法仍沿生成契约返回UNIMPLEMENTED。<br>实际返回表达式：<code>&amp;Document{}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Document{}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1222:1275 -->
````go
func NewDocument() *Document { return &Document{} }

````

## 第 6 步：Document.Bind

插入位置：第 5 步代码之后，紧接追加；完成稿约第 41 行。

平台先登记RPC再装配HTTP，且装配成功后才Serve；Bind仅在这一启动阶段调用一次。
这里不查询SQL或初始化业务，依赖由bootstrap统一创建。

本步声明：`func (s *Document) Bind(fences *Fence, tickets *EditSessions, resources *DraftResources, operations *OperationResults, public *PublicSnapshot)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fences</code> | <code>*Fence</code> | 在此声明中的实际使用：<code>s.fences = fences</code> |
| <code>tickets</code> | <code>*EditSessions</code> | 在此声明中的实际使用：<code>s.tickets = tickets</code> |
| <code>resources</code> | <code>*DraftResources</code> | 在此声明中的实际使用：<code>s.resources = resources</code> |
| <code>operations</code> | <code>*OperationResults</code> | 在此声明中的实际使用：<code>s.operations = operations</code> |
| <code>public</code> | <code>*PublicSnapshot</code> | 在此声明中的实际使用：<code>s.public = public</code> |

<!-- write-step: 6 bytes=1275:1720 -->
````go
// 平台先登记RPC再装配HTTP，且装配成功后才Serve；Bind仅在这一启动阶段调用一次。
// 这里不查询SQL或初始化业务，依赖由bootstrap统一创建。
func (s *Document) Bind(fences *Fence, tickets *EditSessions, resources *DraftResources, operations *OperationResults, public *PublicSnapshot) {
	s.fences = fences
	s.tickets = tickets
	s.resources = resources
	s.operations = operations
	s.public = public
}

````

## 第 7 步：Document.GetPublicSnapshot

插入位置：第 6 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func (s *Document) GetPublicSnapshot(ctx context.Context, in *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.public.GetPublicSnapshot(ctx, in)</code> |
| <code>in</code> | <code>*pb.PublicSnapshotRequest</code> | 在此声明中的实际使用：<code>s.public.GetPublicSnapshot(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.PublicSnapshot</code> | 由 <code>s.public.GetPublicSnapshot(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.public.GetPublicSnapshot(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.public.GetPublicSnapshot(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1720:1885 -->
````go
func (s *Document) GetPublicSnapshot(ctx context.Context, in *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	return s.public.GetPublicSnapshot(ctx, in)
}

````

## 第 8 步：Document.GetOperationResult

插入位置：第 7 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`func (s *Document) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.operations.GetOperationResult(ctx, in)</code> |
| <code>in</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.operations.GetOperationResult(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>s.operations.GetOperationResult(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.operations.GetOperationResult(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.operations.GetOperationResult(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1885:2050 -->
````go
func (s *Document) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error) {
	return s.operations.GetOperationResult(ctx, in)
}

````

## 第 9 步：Document.FenceAccess

插入位置：第 8 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func (s *Document) FenceAccess(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fences.FenceAccess(ctx, in)</code> |
| <code>in</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>s.fences.FenceAccess(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 由 <code>s.fences.FenceAccess(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.fences.FenceAccess(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.fences.FenceAccess(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2050:2191 -->
````go
func (s *Document) FenceAccess(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.fences.FenceAccess(ctx, in)
}

````

## 第 10 步：Document.GetFenceResult

插入位置：第 9 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (s *Document) GetFenceResult(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fences.GetFenceResult(ctx, in)</code> |
| <code>in</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>s.fences.GetFenceResult(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 由 <code>s.fences.GetFenceResult(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.fences.GetFenceResult(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.fences.GetFenceResult(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2191:2338 -->
````go
func (s *Document) GetFenceResult(ctx context.Context, in *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.fences.GetFenceResult(ctx, in)
}

````

## 第 11 步：Document.RedeemCollaborationTicket

插入位置：第 10 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func (s *Document) RedeemCollaborationTicket(ctx context.Context, in *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.tickets.RedeemCollaborationTicket(ctx, in)</code> |
| <code>in</code> | <code>*pb.RedeemTicketRequest</code> | 在此声明中的实际使用：<code>s.tickets.RedeemCollaborationTicket(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CollaborationGrant</code> | 由 <code>s.tickets.RedeemCollaborationTicket(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.tickets.RedeemCollaborationTicket(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.tickets.RedeemCollaborationTicket(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2338:2522 -->
````go
func (s *Document) RedeemCollaborationTicket(ctx context.Context, in *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	return s.tickets.RedeemCollaborationTicket(ctx, in)
}

````

## 第 12 步：Document.ProtectDraftResources

插入位置：第 11 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (s *Document) ProtectDraftResources(ctx context.Context, in *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resources.ProtectDraftResources(ctx, in)</code> |
| <code>in</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>s.resources.ProtectDraftResources(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.resources.ProtectDraftResources(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.resources.ProtectDraftResources(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.resources.ProtectDraftResources(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=2522:2703 -->
````go
func (s *Document) ProtectDraftResources(ctx context.Context, in *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	return s.resources.ProtectDraftResources(ctx, in)
}

````

## 第 13 步：Document.GetDraftResourceProtection

插入位置：第 12 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func (s *Document) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resources.GetDraftResourceProtection(ctx, in)</code> |
| <code>in</code> | <code>*pb.DraftResourceQuery</code> | 在此声明中的实际使用：<code>s.resources.GetDraftResourceProtection(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.resources.GetDraftResourceProtection(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.resources.GetDraftResourceProtection(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.resources.GetDraftResourceProtection(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=2703:2892 -->
````go
func (s *Document) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	return s.resources.GetDraftResourceProtection(ctx, in)
}

````

## 第 14 步：Document.ConfirmDraftResources

插入位置：第 13 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func (s *Document) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resources.ConfirmDraftResources(ctx, in)</code> |
| <code>in</code> | <code>*pb.DraftResourceFinalize</code> | 在此声明中的实际使用：<code>s.resources.ConfirmDraftResources(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.resources.ConfirmDraftResources(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.resources.ConfirmDraftResources(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.resources.ConfirmDraftResources(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=2892:3074 -->
````go
func (s *Document) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	return s.resources.ConfirmDraftResources(ctx, in)
}

````

## 第 15 步：Document.ReleaseDraftResources

插入位置：第 14 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func (s *Document) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resources.ReleaseDraftResources(ctx, in)</code> |
| <code>in</code> | <code>*pb.DraftResourceFinalize</code> | 在此声明中的实际使用：<code>s.resources.ReleaseDraftResources(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 由 <code>s.resources.ReleaseDraftResources(ctx, in)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.resources.ReleaseDraftResources(ctx, in)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.resources.ReleaseDraftResources(ctx, in)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=3074:3324 -->
````go
func (s *Document) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	return s.resources.ReleaseDraftResources(ctx, in)
}

// draft_resources.go：保留原实现，按业务合并文件。

````

## 第 16 步：DraftResources

插入位置：第 15 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`type DraftResources struct{ usecase *logic.DraftResources }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.DraftResources</code> | 使用 <code>logic.DraftResources</code> 类型；其定义处的职责：为协作UPDATE预保护资源并原子预留正文容量。 |

<!-- write-step: 16 bytes=3324:3385 -->
````go
type DraftResources struct{ usecase *logic.DraftResources }

````

## 第 17 步：NewDraftResources

插入位置：第 16 步代码之后，紧接追加；完成稿约第 91 行。

本步声明：`func NewDraftResources(usecase *logic.DraftResources) *DraftResources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.DraftResources</code> | 使用 <code>logic.DraftResources</code> 类型；其定义处的职责：为协作UPDATE预保护资源并原子预留正文容量。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftResources</code> | 实际返回表达式：<code>&amp;DraftResources{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftResources{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=3385:3501 -->
````go
func NewDraftResources(usecase *logic.DraftResources) *DraftResources {
	return &DraftResources{usecase: usecase}
}
````

## 第 18 步：DraftResources.ProtectDraftResources

插入位置：第 17 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`func (s *DraftResources) ProtectDraftResources(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Protect(ctx, req)</code> |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>s.usecase.Protect(ctx, req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ProtectionError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return result, ProtectionError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=3501:3718 -->
````go
func (s *DraftResources) ProtectDraftResources(ctx context.Context, req *pb.DraftResourceRequest) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Protect(ctx, req)
	return result, ProtectionError(err)
}
````

## 第 19 步：DraftResources.GetDraftResourceProtection

插入位置：第 18 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func (s *DraftResources) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Get(ctx, in)</code> |
| <code>in</code> | <code>*pb.DraftResourceQuery</code> | 在此声明中的实际使用：<code>s.usecase.Get(ctx, in)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ProtectionError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return result, ProtectionError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=3718:3932 -->
````go
func (s *DraftResources) GetDraftResourceProtection(ctx context.Context, in *pb.DraftResourceQuery) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Get(ctx, in)
	return result, ProtectionError(err)
}
````

## 第 20 步：DraftResources.ConfirmDraftResources

插入位置：第 19 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func (s *DraftResources) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Finalize(ctx, in, false)</code> |
| <code>in</code> | <code>*pb.DraftResourceFinalize</code> | 在此声明中的实际使用：<code>s.usecase.Finalize(ctx, in, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ProtectionError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return result, ProtectionError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=3932:4156 -->
````go
func (s *DraftResources) ConfirmDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Finalize(ctx, in, false)
	return result, ProtectionError(err)
}
````

## 第 21 步：DraftResources.ReleaseDraftResources

插入位置：第 20 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func (s *DraftResources) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Finalize(ctx, in, true)</code> |
| <code>in</code> | <code>*pb.DraftResourceFinalize</code> | 在此声明中的实际使用：<code>s.usecase.Finalize(ctx, in, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftResourceReceipt</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ProtectionError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return result, ProtectionError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=4156:4379 -->
````go
func (s *DraftResources) ReleaseDraftResources(ctx context.Context, in *pb.DraftResourceFinalize) (*pb.DraftResourceReceipt, error) {
	result, err := s.usecase.Finalize(ctx, in, true)
	return result, ProtectionError(err)
}
````

## 第 22 步：ProtectionError

插入位置：第 21 步代码之后，紧接追加；完成稿约第 110 行。

本步声明：`func ProtectionError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;failure)</code>；<code>dependencyError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>dependencyError(err)</code>；<code>status.Error(code, failure.Code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!errors.As(err, &amp;failure)</code>。条件成立时执行 <code>return dependencyError(err)</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(code, failure.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=4379:5003 -->
````go
func ProtectionError(err error) error {
	var failure domain.ProtectionFailure
	if !errors.As(err, &failure) {
		return dependencyError(err)
	}
	code := codes.Unavailable
	switch failure.Kind {
	case domain.ProtectionInvalid:
		code = codes.InvalidArgument
	case domain.ProtectionForbidden:
		code = codes.PermissionDenied
	case domain.ProtectionMissing:
		code = codes.NotFound
	case domain.ProtectionConflict:
		code = codes.AlreadyExists
	case domain.ProtectionPrecondition:
		code = codes.FailedPrecondition
	}
	return status.Error(code, failure.Code)
}

// edit_sessions.go：保留原实现，按业务合并文件。

````

## 第 23 步：EditSessions

插入位置：第 22 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`type EditSessions struct{ usecase *logic.EditSessions }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.EditSessions</code> | 使用 <code>logic.EditSessions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。 |

<!-- write-step: 23 bytes=5003:5060 -->
````go
type EditSessions struct{ usecase *logic.EditSessions }

````

## 第 24 步：NewEditSessions

插入位置：第 23 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func NewEditSessions(usecase *logic.EditSessions) *EditSessions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.EditSessions</code> | 使用 <code>logic.EditSessions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EditSessions</code> | 实际返回表达式：<code>&amp;EditSessions{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EditSessions{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=5060:5168 -->
````go
func NewEditSessions(usecase *logic.EditSessions) *EditSessions {
	return &EditSessions{usecase: usecase}
}
````

## 第 25 步：EditSessions.RedeemCollaborationTicket

插入位置：第 24 步代码之后，紧接追加；完成稿约第 138 行。

本步声明：`func (s *EditSessions) RedeemCollaborationTicket(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Redeem(ctx, req)</code> |
| <code>req</code> | <code>*pb.RedeemTicketRequest</code> | 在此声明中的实际使用：<code>req == nil</code>；<code>req.ClientSessionId</code>；<code>req.ConnectionId</code>；<code>s.usecase.Redeem(ctx, req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CollaborationGrant</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.InvalidArgument, &quot;CONNECTION_BINDING_INVALID&quot;)</code>；<code>EditingError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>req == nil &#124;&#124; !domain.ValidOperationID(req.ClientSessionId) &#124;&#124; !domain.ValidOperationID(req.ConnectionId)</code>。条件成立时执行 <code>return nil, status.Error(codes.InvalidArgument, &quot;CONNECTION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, EditingError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=5168:5575 -->
````go
func (s *EditSessions) RedeemCollaborationTicket(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	if req == nil || !domain.ValidOperationID(req.ClientSessionId) || !domain.ValidOperationID(req.ConnectionId) {
		return nil, status.Error(codes.InvalidArgument, "CONNECTION_BINDING_INVALID")
	}
	result, err := s.usecase.Redeem(ctx, req)
	return result, EditingError(err)
}
````

## 第 26 步：EditingError

插入位置：第 25 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func EditingError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>dependencyError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>dependencyError(err)</code>；<code>status.Error(code, violation.Code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return dependencyError(err)</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(code, violation.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=5575:6112 -->
````go
func EditingError(err error) error {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		return dependencyError(err)
	}
	code := codes.Unavailable
	switch violation.Kind {
	case domain.InvalidRequest:
		code = codes.InvalidArgument
	case domain.PermissionDenied:
		code = codes.PermissionDenied
	case domain.EntityNotFound:
		code = codes.NotFound
	case domain.StateConflict:
		code = codes.FailedPrecondition
	}
	return status.Error(code, violation.Code)
}

// fence.go：保留原实现，按业务合并文件。

````

## 第 27 步：Fence

插入位置：第 26 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`type Fence struct{ usecase *logic.Fences }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Fences</code> | 使用 <code>logic.Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。 |

<!-- write-step: 27 bytes=6112:6156 -->
````go
type Fence struct{ usecase *logic.Fences }

````

## 第 28 步：NewFence

插入位置：第 27 步代码之后，紧接追加；完成稿约第 168 行。

本步声明：`func NewFence(usecase *logic.Fences) *Fence`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Fences</code> | 使用 <code>logic.Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Fence</code> | 实际返回表达式：<code>&amp;Fence{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Fence{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=6156:6236 -->
````go
func NewFence(usecase *logic.Fences) *Fence { return &Fence{usecase: usecase} }
````

## 第 29 步：Fence.FenceAccess

插入位置：第 28 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func (s *Fence) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Close(ctx, request)</code> |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>platform.ValidBootstrapFence(request)</code>；<code>s.usecase.Close(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.InvalidArgument, &quot;FENCE_BINDING_INVALID&quot;)</code>；<code>FenceError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidBootstrapFence(request)</code>。条件成立时执行 <code>return nil, status.Error(codes.InvalidArgument, &quot;FENCE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, FenceError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=6236:6541 -->
````go
func (s *Fence) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	if !platform.ValidBootstrapFence(request) {
		return nil, status.Error(codes.InvalidArgument, "FENCE_BINDING_INVALID")
	}
	result, err := s.usecase.Close(ctx, request)
	return result, FenceError(err)
}
````

## 第 30 步：Fence.GetFenceResult

插入位置：第 29 步代码之后，紧接追加；完成稿约第 176 行。

本步声明：`func (s *Fence) GetFenceResult(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.FenceAccess(ctx, request)</code> |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>s.FenceAccess(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 由 <code>s.FenceAccess(ctx, request)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.FenceAccess(ctx, request)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.FenceAccess(ctx, request)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=6541:6684 -->
````go
func (s *Fence) GetFenceResult(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	return s.FenceAccess(ctx, request)
}
````

## 第 31 步：FenceError

插入位置：第 30 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func FenceError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>status.Error(code, violation.Code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>violation.Kind == domain.StateConflict</code>。
- 此处通过 <code>return status.Error(code, violation.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=6684:7017 -->
````go
func FenceError(err error) error {
	var violation domain.RuleViolation
	if !errors.As(err, &violation) {
		return err
	}
	code := codes.Unavailable
	if violation.Kind == domain.StateConflict {
		code = codes.AlreadyExists
	}
	return status.Error(code, violation.Code)
}

// lifecycle.go：保留原实现，按业务合并文件。

````

## 第 32 步：Document.BindLifecycle

插入位置：第 31 步代码之后，紧接追加；完成稿约第 193 行。

平台在bootstrap完成后才Serve；这三个入口继续受既定account mTLS allowlist/deadline保护。

本步声明：`func (s *Document) BindLifecycle(plans *logic.LifecyclePlans)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>plans</code> | <code>*logic.LifecyclePlans</code> | 使用 <code>logic.LifecyclePlans</code> 类型；其定义处的职责：将Reserve、Prepare、Apply协议阶段绑定到同一账号生命周期计划。<br>在此声明中的实际使用：<code>s.lifecycle = plans</code> |

<!-- write-step: 32 bytes=7017:7216 -->
````go
// 平台在bootstrap完成后才Serve；这三个入口继续受既定account mTLS allowlist/deadline保护。
func (s *Document) BindLifecycle(plans *logic.LifecyclePlans) { s.lifecycle = plans }

````

## 第 33 步：Document.ReserveAdminRetirement

插入位置：第 32 步代码之后，紧接追加；完成稿约第 196 行。

本步声明：`func (s *Document) ReserveAdminRetirement(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lifecycle.Reserve(ctx, request)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.lifecycle.Reserve(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>lifecycleRPCError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return plan, lifecycleRPCError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=7216:7426 -->
````go
func (s *Document) ReserveAdminRetirement(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Reserve(ctx, request)
	return plan, lifecycleRPCError(err)
}

````

## 第 34 步：Document.PrepareLifecyclePlan

插入位置：第 33 步代码之后，紧接追加；完成稿约第 201 行。

本步声明：`func (s *Document) PrepareLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lifecycle.Prepare(ctx, request)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.lifecycle.Prepare(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>lifecycleRPCError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return plan, lifecycleRPCError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=7426:7634 -->
````go
func (s *Document) PrepareLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Prepare(ctx, request)
	return plan, lifecycleRPCError(err)
}

````

## 第 35 步：Document.ApplyLifecyclePlan

插入位置：第 34 步代码之后，紧接追加；完成稿约第 206 行。

本步声明：`func (s *Document) ApplyLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.lifecycle.Apply(ctx, request)</code> |
| <code>request</code> | <code>*pb.LifecycleRequest</code> | 在此声明中的实际使用：<code>s.lifecycle.Apply(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.LifecyclePlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>lifecycleRPCError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return plan, lifecycleRPCError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=7634:7838 -->
````go
func (s *Document) ApplyLifecyclePlan(ctx context.Context, request *pb.LifecycleRequest) (*pb.LifecyclePlan, error) {
	plan, err := s.lifecycle.Apply(ctx, request)
	return plan, lifecycleRPCError(err)
}

````

## 第 36 步：lifecycleRPCError

插入位置：第 35 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`func lifecycleRPCError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>errors.Is(err, sql.ErrNoRows)</code>；<code>errors.As(err, &amp;rule)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.NotFound, &quot;LIFECYCLE_RPC_ORIGINAL_NOT_FOUND&quot;)</code>；<code>status.Error(codes.Unavailable, &quot;LIFECYCLE_RPC_UNAVAILABLE&quot;)</code>；<code>status.Error(code, rule.Code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return status.Error(codes.NotFound, &quot;LIFECYCLE_RPC_ORIGINAL_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!errors.As(err, &amp;rule)</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;LIFECYCLE_RPC_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>rule.Code == &quot;LIFECYCLE_RPC_HANDOFF_REQUIRED&quot; &#124;&#124; rule.Code == &quot;LIFECYCLE_RPC_RESERVATION_REQUIRED&quot; &#124;&#124; rule.Code == &quot;LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED&quot;</code>。
- 此处通过 <code>return status.Error(code, rule.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=7838:8753 -->
````go
func lifecycleRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return status.Error(codes.NotFound, "LIFECYCLE_RPC_ORIGINAL_NOT_FOUND")
	}
	var rule domain.RuleViolation
	if !errors.As(err, &rule) {
		return status.Error(codes.Unavailable, "LIFECYCLE_RPC_UNAVAILABLE")
	}
	code := codes.Unavailable
	switch rule.Kind {
	case domain.InvalidRequest:
		code = codes.InvalidArgument
	case domain.PermissionDenied:
		code = codes.PermissionDenied
	case domain.EntityNotFound:
		code = codes.NotFound
	case domain.StateConflict:
		code = codes.AlreadyExists
		if rule.Code == "LIFECYCLE_RPC_HANDOFF_REQUIRED" || rule.Code == "LIFECYCLE_RPC_RESERVATION_REQUIRED" || rule.Code == "LIFECYCLE_RPC_PREPARED_PLAN_REQUIRED" {
			code = codes.FailedPrecondition
		}
	}
	return status.Error(code, rule.Code)
}

// operation_results.go：保留原实现，按业务合并文件。

````

## 第 37 步：OperationResults

插入位置：第 36 步代码之后，紧接追加；完成稿约第 241 行。

本步声明：`type OperationResults struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>submissions</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。 |
| <code>merges</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。 |
| <code>controls</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。 |
| <code>pulls</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。 |
| <code>previews</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。 |
| <code>drafts</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。 |
| <code>reverse</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。 |
| <code>resolutions</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。 |
| <code>publications</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。 |

<!-- write-step: 37 bytes=8753:9089 -->
````go
type OperationResults struct {
	submissions  *logic.Submissions
	merges       *logic.Merges
	controls     *logic.MainControls
	pulls        *logic.PullMain
	previews     *logic.StructurePreviews
	drafts       *logic.DraftControls
	reverse      *logic.ReverseChanges
	resolutions  *logic.Resolutions
	publications *logic.Publications
}

````

## 第 38 步：NewOperationResults

插入位置：第 37 步代码之后，紧接追加；完成稿约第 253 行。

本步声明：`func NewOperationResults(submissions *logic.Submissions, merges *logic.Merges, controls *logic.MainControls, pulls *logic.PullMain, previews *logic.StructurePreviews, drafts *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions, publications *logic.Publications) *OperationResults`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>submissions</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。<br>在此声明中的实际使用：<code>submissions: submissions</code> |
| <code>merges</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。<br>在此声明中的实际使用：<code>merges: merges</code> |
| <code>controls</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。<br>在此声明中的实际使用：<code>controls: controls</code> |
| <code>pulls</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。<br>在此声明中的实际使用：<code>pulls: pulls</code> |
| <code>previews</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。<br>在此声明中的实际使用：<code>previews: previews</code> |
| <code>drafts</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。<br>在此声明中的实际使用：<code>drafts: drafts</code> |
| <code>reverse</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。<br>在此声明中的实际使用：<code>reverse: reverse</code> |
| <code>resolutions</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。<br>在此声明中的实际使用：<code>resolutions: resolutions</code> |
| <code>publications</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。<br>在此声明中的实际使用：<code>publications: publications</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OperationResults</code> | 实际返回表达式：<code>&amp;OperationResults{submissions: submissions, merges: merges, controls: controls, pulls: pulls, previews: previews, drafts: drafts, reverse: reverse, resolutions: resolutions, publications: publications}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OperationResults{submissions: submissions, merges: merges, controls: controls, pulls: pulls, previews: previews, drafts: drafts, reverse: reverse, resolutions: resolutions, publications: publications}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=9089:9615 -->
````go
func NewOperationResults(submissions *logic.Submissions, merges *logic.Merges, controls *logic.MainControls, pulls *logic.PullMain, previews *logic.StructurePreviews, drafts *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions, publications *logic.Publications) *OperationResults {
	return &OperationResults{submissions: submissions, merges: merges, controls: controls, pulls: pulls, previews: previews, drafts: drafts, reverse: reverse, resolutions: resolutions, publications: publications}
}

````

## 第 39 步：OperationResults.GetOperationResult

插入位置：第 38 步代码之后，紧接追加；完成稿约第 257 行。

本步声明：`func (s *OperationResults) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.submissions.OperationResult(ctx, in)</code>；<code>s.merges.OperationResult(ctx, in)</code>；<code>s.controls.OperationResult(ctx, in)</code>；<code>s.pulls.OperationResult(ctx, in)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>in</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>s.submissions.OperationResult(ctx, in)</code>；<code>s.merges.OperationResult(ctx, in)</code>；<code>s.controls.OperationResult(ctx, in)</code>；<code>s.pulls.OperationResult(ctx, in)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>result</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.NotFound, &quot;OPERATION_NOT_FOUND&quot;)</code>；<code>status.Error(code, violation.Code)</code>；<code>status.Error(codes.Unavailable, &quot;OPERATION_RESULT_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.controls != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.pulls != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.previews != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.drafts != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.reverse != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.resolutions != nil</code>。
- 检查 <code>err == sql.ErrNoRows &amp;&amp; s.publications != nil</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil, status.Error(codes.NotFound, &quot;OPERATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;violation)</code>。
- 检查 <code>violation.Kind == domain.InvalidRequest</code>。
- 检查 <code>violation.Kind == domain.StateConflict</code>。
- 此处通过 <code>return nil, status.Error(code, violation.Code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, status.Error(codes.Unavailable, &quot;OPERATION_RESULT_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=9615:11187 -->
````go
func (s *OperationResults) GetOperationResult(ctx context.Context, in *pb.OperationQuery) (*pb.OperationResult, error) {
	result, err := s.submissions.OperationResult(ctx, in)
	if err == sql.ErrNoRows {
		result, err = s.merges.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.controls != nil {
		result, err = s.controls.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.pulls != nil {
		result, err = s.pulls.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.previews != nil {
		result, err = s.previews.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.drafts != nil {
		result, err = s.drafts.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.reverse != nil {
		result, err = s.reverse.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.resolutions != nil {
		result, err = s.resolutions.OperationResult(ctx, in)
	}
	if err == sql.ErrNoRows && s.publications != nil {
		result, err = s.publications.OperationResult(ctx, in)
	}
	if err == nil {
		return result, nil
	}
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "OPERATION_NOT_FOUND")
	}
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		code := codes.Unavailable
		if violation.Kind == domain.InvalidRequest {
			code = codes.InvalidArgument
		}
		if violation.Kind == domain.StateConflict {
			code = codes.AlreadyExists
		}
		return nil, status.Error(code, violation.Code)
	}
	return nil, status.Error(codes.Unavailable, "OPERATION_RESULT_UNKNOWN")
}

// public_snapshot.go：保留原实现，按业务合并文件。

````

## 第 40 步：PublicSnapshot

插入位置：第 39 步代码之后，紧接追加；完成稿约第 305 行。

本步声明：`type PublicSnapshot struct{ usecase *logic.PublicReading }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。 |

<!-- write-step: 40 bytes=11187:11247 -->
````go
type PublicSnapshot struct{ usecase *logic.PublicReading }

````

## 第 41 步：NewPublicSnapshot

插入位置：第 40 步代码之后，紧接追加；完成稿约第 307 行。

本步声明：`func NewPublicSnapshot(usecase *logic.PublicReading) *PublicSnapshot`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.PublicReading</code> | 使用 <code>logic.PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PublicSnapshot</code> | 实际返回表达式：<code>&amp;PublicSnapshot{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PublicSnapshot{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=11247:11363 -->
````go
func NewPublicSnapshot(usecase *logic.PublicReading) *PublicSnapshot {
	return &PublicSnapshot{usecase: usecase}
}

````

## 第 42 步：PublicSnapshot.GetPublicSnapshot

插入位置：第 41 步代码之后，紧接追加；完成稿约第 311 行。

调用方资格仍由冻结mTLS方法白名单核验；适配器不替Search伪造用户身份。

本步声明：`func (s *PublicSnapshot) GetPublicSnapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.usecase.Snapshot(ctx, input)</code> |
| <code>input</code> | <code>*pb.PublicSnapshotRequest</code> | 在此声明中的实际使用：<code>s.usecase.Snapshot(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.PublicSnapshot</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>EditingError(err)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return value, EditingError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=11363:11672 -->
````go
// 调用方资格仍由冻结mTLS方法白名单核验；适配器不替Search伪造用户身份。
func (s *PublicSnapshot) GetPublicSnapshot(ctx context.Context, input *pb.PublicSnapshotRequest) (*pb.PublicSnapshot, error) {
	value, err := s.usecase.Snapshot(ctx, input)
	return value, EditingError(err)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/server/rpc_server.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
