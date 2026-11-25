# 分步编写 `services/document/internal/client/collaboration_workflows_client.go`

先按 [D06 · 审核合并、冲突修复与反向更改](../lessons/document/06-merge-conflict.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/109-collaboration-workflows-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

适配Document到协作服务的捕获、合并、草稿准备、安装和持久事实查询。

Go只处理规范正文与固定协议，Yjs及可靠存储由Node负责，服务之间不能越库读取。

写之前先按这条执行路线理解各段的连接：

1. SubmissionCaptures先取得绑定FLUSH再捕获
2. Merges提供CaptureMain/Compute/PrepareMain/ReconcileMain
3. DraftTransitions.Prepare只送上下文摘要
4. NodePersistence查询更新结果
5. 转移计量核原计划。

本文件需要守住的边界：

- MainRevision只是可靠尾部摘要，不授予正文访问
- Node结果不能自行决定Document资源解除保护
- 严格限制响应尺寸。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/collaboration_workflows_client.go') -and (Get-Item -LiteralPath 'services/document/internal/client/collaboration_workflows_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/collaboration_workflows_client.go')) { New-Item -ItemType File -Path 'services/document/internal/client/collaboration_workflows_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:626 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// collaboration_fence.go：保留原实现，按业务合并文件。

````

## 第 2 步：CollaborationFence

插入位置：第 1 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`type CollaborationFence struct{ cfg platform.Config }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。 |

<!-- write-step: 2 bytes=626:681 -->
````go
type CollaborationFence struct{ cfg platform.Config }

````

## 第 3 步：NewCollaborationFence

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewCollaborationFence(cfg platform.Config) *CollaborationFence`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg: cfg</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*CollaborationFence</code> | 实际返回表达式：<code>&amp;CollaborationFence{cfg: cfg}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;CollaborationFence{cfg: cfg}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=681:791 -->
````go
func NewCollaborationFence(cfg platform.Config) *CollaborationFence {
	return &CollaborationFence{cfg: cfg}
}
````

## 第 4 步：CollaborationFence.Close

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func (c *CollaborationFence) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(ctx, 1500*time.Millisecond)</code> |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 由 <code>pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 1500*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=791:1256 -->
````go
func (c *CollaborationFence) Close(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	callCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	return pb.NewCollaborationInternalClient(peer).FenceAccess(callCtx, request)
}

// committed_submission_cut.go：保留原实现，按业务合并文件。

````

## 第 5 步：Merges.CommittedCut

插入位置：第 4 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func (c *Merges) CommittedCut(ctx context.Context, query *pb.OperationQuery) (domain.CommittedSubmissionCut, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;+&quot;/infra/persistence/submission-cut&quot;, bytes.NewReader(raw))</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(query)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.CommittedSubmissionCut</code> | 使用 <code>domain.CommittedSubmissionCut</code> 类型；其定义处的职责：CommittedSubmissionCut只保存既有成功送审的切点证明，不携带或延长原用户的权限。<br>实际返回表达式：<code>fact</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fact, domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != http.StatusOK &#124;&#124; json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&amp;fact) != nil</code>。条件成立时执行 <code>return fact, domain.Violation(domain.TemporarilyUnavailable, &quot;REVIEW_CAPTURE_CUT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fact, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1256:2351 -->
````go
func (c *Merges) CommittedCut(ctx context.Context, query *pb.OperationQuery) (domain.CommittedSubmissionCut, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var fact domain.CommittedSubmissionCut
	raw, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(query)
	if err != nil {
		return fact, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/persistence/submission-cut", bytes.NewReader(raw))
	if err != nil {
		return fact, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fact, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&fact) != nil {
		return fact, domain.Violation(domain.TemporarilyUnavailable, "REVIEW_CAPTURE_CUT_UNKNOWN")
	}
	return fact, nil
}

// discard_previews.go：保留原实现，按业务合并文件。

````

## 第 6 步：DraftTransitions.FixedDraft

插入位置：第 5 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (c *DraftTransitions) FixedDraft(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;+&quot;/infra/draft/fixed&quot;, bytes.NewReader(raw))</code> |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>(protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DISCARD_TRANSFER_LIMIT&quot;)</code>；<code>errors.New(&quot;DISCARD_FIXED_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == 413</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DISCARD_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return nil, errors.New(&quot;DISCARD_FIXED_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(value) &gt; 24&lt;&lt;20</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DISCARD_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2351:3627 -->
````go
func (c *DraftTransitions) FixedDraft(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	// protobuf的uint64/bytes用官方JSON编码；普通encoding/json会使HTTP与RPC的原指纹绑定不一致。
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/draft/fixed", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == 413 {
		return nil, domain.Violation(domain.StateConflict, "DISCARD_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("DISCARD_FIXED_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (24<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 24<<20 {
		return nil, domain.Violation(domain.StateConflict, "DISCARD_TRANSFER_LIMIT")
	}
	return value, nil
}

// draft_transitions.go：保留原实现，按业务合并文件。

````

## 第 7 步：DraftTransitions

插入位置：第 6 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`type DraftTransitions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 7 bytes=3627:3702 -->
````go
type DraftTransitions struct {
	cfg  platform.Config
	http *http.Client
}

````

## 第 8 步：NewDraftTransitions

插入位置：第 7 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func NewDraftTransitions(cfg platform.Config, peer *http.Client) *DraftTransitions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg: cfg</code> |
| <code>peer</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftTransitions</code> | 实际返回表达式：<code>&amp;DraftTransitions{cfg: cfg, http: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftTransitions{cfg: cfg, http: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3702:3837 -->
````go
func NewDraftTransitions(cfg platform.Config, peer *http.Client) *DraftTransitions {
	return &DraftTransitions{cfg: cfg, http: peer}
}
````

## 第 9 步：DraftTransitions.Preview

插入位置：第 8 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`func (c *DraftTransitions) Preview(ctx context.Context, input *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftPreviewReply</code> | 由 <code>pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3837:4258 -->
````go
func (c *DraftTransitions) Preview(ctx context.Context, input *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).CaptureDraftPreview(ctx, input, grpc.MaxCallRecvMsgSize(32<<20))
}
````

## 第 10 步：DraftTransitions.Apply

插入位置：第 9 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func (c *DraftTransitions) Apply(ctx context.Context, input *pb.DraftResolutionRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20))</code> |
| <code>input</code> | <code>*pb.DraftResolutionRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4258:4680 -->
````go
func (c *DraftTransitions) Apply(ctx context.Context, input *pb.DraftResolutionRequest) (*pb.OperationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).ApplyDraftResolution(ctx, input, grpc.MaxCallSendMsgSize(32<<20))
}

````

## 第 11 步：DraftTransitions.Prepare

插入位置：第 10 步代码之后，紧接追加；完成稿约第 136 行。

准备入口仅传原上下文和摘要，正文从Node不可变预览读取，避免把浏览器或调用方正文当可靠草稿。

本步声明：`func (c *DraftTransitions) Prepare(ctx context.Context, input any) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;+&quot;/infra/draft/prepare&quot;, bytes.NewReader(raw))</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DRAFT_TRANSFER_LIMIT&quot;)</code>；<code>errors.New(&quot;DRAFT_PREPARE_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;DRAFT_PREPARE_TOO_LARGE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == http.StatusRequestEntityTooLarge</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;DRAFT_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return nil, errors.New(&quot;DRAFT_PREPARE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(value) &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, errors.New(&quot;DRAFT_PREPARE_TOO_LARGE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4680:5868 -->
````go
// 准备入口仅传原上下文和摘要，正文从Node不可变预览读取，避免把浏览器或调用方正文当可靠草稿。
func (c *DraftTransitions) Prepare(ctx context.Context, input any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/draft/prepare", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusRequestEntityTooLarge {
		return nil, domain.Violation(domain.StateConflict, "DRAFT_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("DRAFT_PREPARE_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 32<<20 {
		return nil, errors.New("DRAFT_PREPARE_TOO_LARGE")
	}
	return value, nil
}

// merges.go：保留原实现，按业务合并文件。

````

## 第 12 步：Merges

插入位置：第 11 步代码之后，紧接追加；完成稿约第 172 行。

本步声明：`type Merges struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 12 bytes=5868:5933 -->
````go
type Merges struct {
	cfg  platform.Config
	http *http.Client
}

````

## 第 13 步：NewMerges

插入位置：第 12 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`func NewMerges(cfg platform.Config, peer *http.Client) *Merges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg: cfg</code> |
| <code>peer</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Merges</code> | 实际返回表达式：<code>&amp;Merges{cfg: cfg, http: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Merges{cfg: cfg, http: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5933:6038 -->
````go
func NewMerges(cfg platform.Config, peer *http.Client) *Merges { return &Merges{cfg: cfg, http: peer} }

````

## 第 14 步：Merges.MainRevision

插入位置：第 13 步代码之后，紧接追加；完成稿约第 179 行。

只读MAIN的可靠尾部摘要，不做Capture；正文仍必须走原读取准入和固定版本接口。

本步声明：`func (c *Merges) MainRevision(ctx context.Context, document uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;+&quot;/infra/main/revision&quot;, bytes.NewReader(raw))</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_REVISION_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_REVISION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != http.StatusOK &#124;&#124; json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&amp;revision) != nil</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_REVISION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32</code>。条件成立时执行 <code>return &quot;&quot;, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_REVISION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return revision, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6038:7280 -->
````go
// 只读MAIN的可靠尾部摘要，不做Capture；正文仍必须走原读取准入和固定版本接口。
func (c *Merges) MainRevision(ctx context.Context, document uint64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := json.Marshal(map[string]string{"document_id": domain.TextID(document)})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Peers["collaborationHttp"]+"/infra/main/revision", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	defer response.Body.Close()
	var revision string
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&revision) != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	digest, err := hex.DecodeString(revision)
	if err != nil || len(digest) != 32 {
		return "", domain.Violation(domain.TemporarilyUnavailable, "MAIN_REVISION_UNAVAILABLE")
	}
	return revision, nil
}

````

## 第 15 步：Merges.CaptureMain

插入位置：第 14 步代码之后，紧接追加；完成稿约第 208 行。

本步声明：`func (c *Merges) CaptureMain(ctx context.Context, input *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>input</code> | <code>*pb.CaptureMainRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.MainCaptureReply</code> | 由 <code>pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7280:7824 -->
````go
func (c *Merges) CaptureMain(ctx context.Context, input *pb.CaptureMainRequest) (*pb.MainCaptureReply, error) {
	// worker租约覆盖多个步骤；单次跨服务调用仍受既定五秒边界约束，不能直接转发十五秒任务期限。
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).CaptureMainView(ctx, input, grpc.MaxCallRecvMsgSize(32<<20))
}

````

## 第 16 步：Merges.Compute

插入位置：第 15 步代码之后，紧接追加；完成稿约第 220 行。

本步声明：`func (c *Merges) Compute(ctx context.Context, input *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>input</code> | <code>*pb.MergeCandidateRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.MergeCandidateReply</code> | 由 <code>pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7824:8372 -->
````go
func (c *Merges) Compute(ctx context.Context, input *pb.MergeCandidateRequest) (*pb.MergeCandidateReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	// 三份各16MiB固定正文；仅本调用提高发送预算，不改变单份正文上限。
	return pb.NewCollaborationInternalClient(peer).ComputeMergeCandidate(ctx, input, grpc.MaxCallSendMsgSize(64<<20), grpc.MaxCallRecvMsgSize(32<<20))
}

````

## 第 17 步：Merges.PrepareMain

插入位置：第 16 步代码之后，紧接追加；完成稿约第 232 行。

本步声明：`func (c *Merges) PrepareMain(ctx context.Context, input *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)</code> |
| <code>input</code> | <code>*pb.PrepareMainCommitRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.MainBarrierReply</code> | 由 <code>pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=8372:8757 -->
````go
func (c *Merges) PrepareMain(ctx context.Context, input *pb.PrepareMainCommitRequest) (*pb.MainBarrierReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).PrepareMainCommit(ctx, input)
}

````

## 第 18 步：Merges.ReconcileMain

插入位置：第 17 步代码之后，紧接追加；完成稿约第 243 行。

本步声明：`func (c *Merges) ReconcileMain(ctx context.Context, input *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>input</code> | <code>*pb.ReconcileMainCommitRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.InstallReply</code> | 由 <code>pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32&lt;&lt;20), grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=8757:9279 -->
````go
func (c *Merges) ReconcileMain(ctx context.Context, input *pb.ReconcileMainCommitRequest) (*pb.InstallReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).ReconcileMainCommit(ctx, input, grpc.MaxCallSendMsgSize(32<<20), grpc.MaxCallRecvMsgSize(32<<20))
}

// node_persistence.go：保留原实现，按业务合并文件。

````

## 第 19 步：NodePersistence

插入位置：第 18 步代码之后，紧接追加；完成稿约第 256 行。

NodePersistence只读取Node的持久提交事实，不替Doc决定资源能否解除保护。

本步声明：`type NodePersistence struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |
| <code>url</code> | <code>string</code> | Node协作内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |

<!-- write-step: 19 bytes=9279:9442 -->
````go
// NodePersistence只读取Node的持久提交事实，不替Doc决定资源能否解除保护。
type NodePersistence struct {
	http *http.Client
	url  string
}

````

## 第 20 步：NewNodePersistence

插入位置：第 19 步代码之后，紧接追加；完成稿约第 262 行。

本步声明：`func NewNodePersistence(httpClient *http.Client, url string) *NodePersistence`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |
| <code>url</code> | <code>string</code> | 在此声明中的实际使用：<code>url: url</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NodePersistence</code> | 使用 <code>NodePersistence</code> 类型；其定义处的职责：NodePersistence只读取Node的持久提交事实，不替Doc决定资源能否解除保护。<br>实际返回表达式：<code>&amp;NodePersistence{http: httpClient, url: url}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NodePersistence{http: httpClient, url: url}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=9442:9577 -->
````go
func NewNodePersistence(httpClient *http.Client, url string) *NodePersistence {
	return &NodePersistence{http: httpClient, url: url}
}
````

## 第 21 步：NodePersistence.UpdateOutcome

插入位置：第 20 步代码之后，紧接追加；完成稿约第 265 行。

本步声明：`func (s *NodePersistence) UpdateOutcome(ctx context.Context, req *pb.DraftResourceRequest) (domain.NodeUpdateProof, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, s.url+&quot;/infra/persistence/update&quot;, bytes.NewReader(raw))</code> |
| <code>req</code> | <code>*pb.DraftResourceRequest</code> | 在此声明中的实际使用：<code>req.RoomId</code>；<code>req.WorkspaceEpoch</code>；<code>req.ClientSessionId</code>；<code>req.ClientUpdateId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.NodeUpdateProof</code> | 使用 <code>domain.NodeUpdateProof</code> 类型；其定义处的职责：表达草稿正文计量、资源保护绑定及Node提交证明。<br>实际返回表达式：<code>proof</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RejectProtection(domain.ProtectionUnknown, &quot;NODE_OUTCOME_UNKNOWN&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.http == nil</code>。条件成立时执行 <code>return proof, domain.RejectProtection(domain.ProtectionUnknown, &quot;NODE_OUTCOME_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return proof, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return proof, domain.RejectProtection(domain.ProtectionUnknown, &quot;NODE_OUTCOME_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 登记 <code>res.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>res.StatusCode != 200 &#124;&#124; json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&amp;proof) != nil</code>。条件成立时执行 <code>return proof, domain.RejectProtection(domain.ProtectionUnknown, &quot;NODE_OUTCOME_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return proof, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=9577:10736 -->
````go
func (s *NodePersistence) UpdateOutcome(ctx context.Context, req *pb.DraftResourceRequest) (domain.NodeUpdateProof, error) {
	var proof domain.NodeUpdateProof
	if s.http == nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	raw, _ := json.Marshal(map[string]string{"room_id": domain.TextID(req.RoomId), "workspace_epoch": domain.TextID(req.WorkspaceEpoch), "client_session_id": req.ClientSessionId, "client_update_id": req.ClientUpdateId})
	request, err := http.NewRequestWithContext(ctx, "POST", s.url+"/infra/persistence/update", bytes.NewReader(raw))
	if err != nil {
		return proof, err
	}
	request.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(request)
	if err != nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&proof) != nil {
		return proof, domain.RejectProtection(domain.ProtectionUnknown, "NODE_OUTCOME_UNKNOWN")
	}
	return proof, nil
}

// personal_transfer_storage.go：保留原实现，按业务合并文件。

````

## 第 22 步：PersonalTransferStorage

插入位置：第 21 步代码之后，紧接追加；完成稿约第 289 行。

本步声明：`type PersonalTransferStorage struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | Node协作内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 22 bytes=10736:10817 -->
````go
type PersonalTransferStorage struct {
	endpoint string
	http     *http.Client
}

````

## 第 23 步：NewPersonalTransferStorage

插入位置：第 22 步代码之后，紧接追加；完成稿约第 294 行。

本步声明：`func NewPersonalTransferStorage(endpoint string, client *http.Client) *PersonalTransferStorage`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |
| <code>client</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: client</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PersonalTransferStorage</code> | 实际返回表达式：<code>&amp;PersonalTransferStorage{endpoint: endpoint, http: client}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PersonalTransferStorage{endpoint: endpoint, http: client}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=10817:10984 -->
````go
func NewPersonalTransferStorage(endpoint string, client *http.Client) *PersonalTransferStorage {
	return &PersonalTransferStorage{endpoint: endpoint, http: client}
}

````

## 第 24 步：PersonalTransferStorage.MeasurePersonalTransfer

插入位置：第 23 步代码之后，紧接追加；完成稿约第 298 行。

本步声明：`func (c *PersonalTransferStorage) MeasurePersonalTransfer(ctx context.Context, plan domain.PersonalTransferPlan) (logic.PersonalTransferStorage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+&quot;/infra/lifecycle/personal-transfer-storage&quot;, bytes.NewReader(raw))</code> |
| <code>plan</code> | <code>domain.PersonalTransferPlan</code> | 使用 <code>domain.PersonalTransferPlan</code> 类型；其定义处的职责：固定计划不保存可替换正文；仅把原资源与计费事实带到原接受操作的恢复阶段。<br>在此声明中的实际使用：<code>plan.Document</code>；<code>plan.Operation</code>；<code>plan.Scopes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>logic.PersonalTransferStorage</code> | 使用 <code>logic.PersonalTransferStorage</code> 类型；其定义处的职责：Node 只读计量须由真实旧来源关闭事实固定，不能由用户声称保存/关闭成功。<br>实际返回表达式：<code>result</code>；<code>logic.PersonalTransferStorage{Operation: plan.Operation, Document: plan.Document, Bytes: count, Digest: digest}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PERSONAL_TRANSFER_STORAGE_PLAN_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNCONFIRMED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>plan.Document == 0 &#124;&#124; !domain.ValidOperationID(plan.Operation) &#124;&#124; len(plan.Scopes) != 2</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Epoch == 0 &#124;&#124; !domain.ValidOperationID(scope.Operation) &#124;&#124; 			(scope.Key != &quot;doc-acl:&quot;+input.Document &amp;&amp; scope.Key != &quot;publication:&quot;+input.Document)</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return input.Closures&#91;i&#93;.Scope &lt; input.Closures&#91;j&#93;.Scope</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.Closures&#91;0&#93;.Scope == input.Closures&#91;1&#93;.Scope</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; decoder.Decode(&amp;measured) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; 		measured.Operation != input.Operation &#124;&#124; measured.Document != input.Docume …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; strconv.FormatUint(count, 10) != measured.Bytes</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(digest) != 32 &#124;&#124; hex.EncodeToString(digest) != measured.Digest</code>。条件成立时执行 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=10984:13558 -->
````go
func (c *PersonalTransferStorage) MeasurePersonalTransfer(ctx context.Context, plan domain.PersonalTransferPlan) (logic.PersonalTransferStorage, error) {
	var result logic.PersonalTransferStorage
	if plan.Document == 0 || !domain.ValidOperationID(plan.Operation) || len(plan.Scopes) != 2 {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
	}
	input := platform.PersonalTransferStorageRequest{Operation: plan.Operation, Document: domain.TextID(plan.Document), Generation: "1"}
	for _, scope := range plan.Scopes {
		if scope.Epoch == 0 || !domain.ValidOperationID(scope.Operation) ||
			(scope.Key != "doc-acl:"+input.Document && scope.Key != "publication:"+input.Document) {
			return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
		}
		input.Closures = append(input.Closures, platform.PersonalTransferStorageClosure{Revocation: scope.Operation, Scope: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	sort.Slice(input.Closures, func(i, j int) bool { return input.Closures[i].Scope < input.Closures[j].Scope })
	if input.Closures[0].Scope == input.Closures[1].Scope {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_PLAN_INVALID")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/lifecycle/personal-transfer-storage", bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNAVAILABLE")
	}
	defer response.Body.Close()
	// 丢回复只能再次查询相同原号。完整严格解码及目标核对防止另一篇的字节/摘要冒充本篇。
	var measured platform.PersonalTransferStorageMeasurement
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2048))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(&measured) != nil || decoder.Decode(new(any)) != io.EOF ||
		measured.Operation != input.Operation || measured.Document != input.Document || measured.Generation != "1" || !measured.SourceClosed {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	count, err := strconv.ParseUint(measured.Bytes, 10, 64)
	if err != nil || strconv.FormatUint(count, 10) != measured.Bytes {
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	digest, err := hex.DecodeString(measured.Digest)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != measured.Digest {
````

## 第 25 步：PersonalTransferStorage.MeasurePersonalTransfer（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 343 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, errors.New(&quot;PERSONAL_TRANSFER_STORAGE_UNCONFIRMED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return logic.PersonalTransferStorage{Operation: plan.Operation, Document: plan.Document, Bytes: count, Digest: digest}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=13558:13829 -->
````go
		return result, errors.New("PERSONAL_TRANSFER_STORAGE_UNCONFIRMED")
	}
	return logic.PersonalTransferStorage{Operation: plan.Operation, Document: plan.Document, Bytes: count, Digest: digest}, nil
}

// structure_previews.go：保留原实现，按业务合并文件。

````

## 第 26 步：DraftTransitions.FixedStructure

插入位置：第 25 步代码之后，紧接追加；完成稿约第 350 行。

本步声明：`func (c *DraftTransitions) FixedStructure(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 4*time.Second)</code>；<code>context.WithTimeout(ctx, 4*time.Second)</code>；<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;+&quot;/infra/structure/fixed&quot;, bytes.NewReader(raw))</code> |
| <code>input</code> | <code>*pb.DraftPreviewRequest</code> | 在此声明中的实际使用：<code>(protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_TRANSFER_LIMIT&quot;)</code>；<code>errors.New(&quot;STRUCTURE_FIXED_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;STRUCTURE_FIXED_TOO_LARGE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 4*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode == 413</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return nil, errors.New(&quot;STRUCTURE_FIXED_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>len(value) &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, errors.New(&quot;STRUCTURE_FIXED_TOO_LARGE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=13829:14979 -->
````go
func (c *DraftTransitions) FixedStructure(ctx context.Context, input *pb.DraftPreviewRequest) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(input)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", c.cfg.Peers["collaborationHttp"]+"/infra/structure/fixed", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == 413 {
		return nil, domain.Violation(domain.StateConflict, "STRUCTURE_TRANSFER_LIMIT")
	}
	if response.StatusCode != 200 {
		return nil, errors.New("STRUCTURE_FIXED_UNAVAILABLE")
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(value) > 32<<20 {
		return nil, errors.New("STRUCTURE_FIXED_TOO_LARGE")
	}
	return value, nil
}

// submission_captures.go：保留原实现，按业务合并文件。

````

## 第 27 步：SubmissionCaptures

插入位置：第 26 步代码之后，紧接追加；完成稿约第 385 行。

本步声明：`type SubmissionCaptures struct{ cfg platform.Config }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。 |

<!-- write-step: 27 bytes=14979:15034 -->
````go
type SubmissionCaptures struct{ cfg platform.Config }

````

## 第 28 步：NewSubmissionCaptures

插入位置：第 27 步代码之后，紧接追加；完成稿约第 387 行。

本步声明：`func NewSubmissionCaptures(cfg platform.Config) *SubmissionCaptures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg: cfg</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*SubmissionCaptures</code> | 实际返回表达式：<code>&amp;SubmissionCaptures{cfg: cfg}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;SubmissionCaptures{cfg: cfg}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=15034:15145 -->
````go
func NewSubmissionCaptures(cfg platform.Config) *SubmissionCaptures {
	return &SubmissionCaptures{cfg: cfg}
}

````

## 第 29 步：SubmissionCaptures.Capture

插入位置：第 28 步代码之后，紧接追加；完成稿约第 391 行。

本步声明：`func (c *SubmissionCaptures) Capture(ctx context.Context, request *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>client.GetCapture(ctx, query, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code>；<code>client.CaptureSubmission(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>request</code> | <code>*pb.CaptureSubmissionRequest</code> | 在此声明中的实际使用：<code>request.Context</code>；<code>client.CaptureSubmission(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CaptureReply</code> | 实际返回表达式：<code>nil</code>；<code>known</code>；<code>reply</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;SUBMISSION_FLUSH_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>known.State == pb.CaptureReply_FIXED</code>。条件成立时执行 <code>return known, nil</code>。当前函数在这里结束。
- 检查 <code>status.Code(err) == codes.PermissionDenied &amp;&amp; status.Convert(err).Message() == &quot;FLUSH_BINDING_INVALID&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;SUBMISSION_FLUSH_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return reply, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=15145:16307 -->
````go
func (c *SubmissionCaptures) Capture(ctx context.Context, request *pb.CaptureSubmissionRequest) (*pb.CaptureReply, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	client := pb.NewCollaborationInternalClient(peer)
	// 固定正文允许 16 MiB；逐调用提高接收上限，不能意外沿用 gRPC 默认 4 MiB。
	query := &pb.OperationQuery{OperationId: request.Context.OperationId, Target: request.Context.Target, RequestFingerprint: request.Context.RequestFingerprint}
	known, err := client.GetCapture(ctx, query, grpc.MaxCallRecvMsgSize(32<<20))
	if err != nil {
		return nil, err
	}
	if known.State == pb.CaptureReply_FIXED {
		return known, nil
	}
	reply, err := client.CaptureSubmission(ctx, request, grpc.MaxCallRecvMsgSize(32<<20))
	// 仅这个明确的原回执绑定拒绝属于非法用户输入；来源关闭、超时或候选冲突仍由原号恢复。
	if status.Code(err) == codes.PermissionDenied && status.Convert(err).Message() == "FLUSH_BINDING_INVALID" {
		return nil, domain.Violation(domain.StateConflict, "SUBMISSION_FLUSH_INVALID")
	}
	return reply, err
}

````

## 第 30 步：SubmissionCaptures.Finalize

插入位置：第 29 步代码之后，紧接追加；完成稿约第 415 行。

本步声明：`func (c *SubmissionCaptures) Finalize(ctx context.Context, request *pb.FinalizeCaptureRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>request</code> | <code>*pb.FinalizeCaptureRequest</code> | 在此声明中的实际使用：<code>pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=16307:16660 -->
````go
func (c *SubmissionCaptures) Finalize(ctx context.Context, request *pb.FinalizeCaptureRequest) (*pb.OperationResult, error) {
	peer, err := platform.DialPeer(c.cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	defer peer.Close()
	return pb.NewCollaborationInternalClient(peer).FinalizeCapture(ctx, request, grpc.MaxCallRecvMsgSize(32<<20))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/collaboration_workflows_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/06-merge-conflict.md)。
