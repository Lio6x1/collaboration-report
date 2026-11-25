# 分步编写 `services/document/internal/handler/document/operations_handlers.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/255-operations-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。

取消信号不证明字节已停止，必须等实际写入和Flush结束才能报告排空。

写之前先按这条执行路线理解各段的连接：

1. ResponseOutput先编码和检查budget，再打开Edge登记并写入
2. finish结束真实输出
3. Transactions适配Read/Mutation/Replay
4. DraftSources按purpose分发固定来源。

本文件需要守住的边界：

- done不能在写入前提前关闭
- 202响应同样必须登记输出
- 超限应在成功头与敏感字节发出前失败。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/operations_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/operations_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/operations_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/operations_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:538 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
	"sync"
	"time"
)

// draft_sources.go：保留原实现，按业务合并文件。

````

## 第 2 步：DraftSources

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type DraftSources struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pulls</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。 |
| <code>controls</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。 |
| <code>reverse</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。 |
| <code>resolutions</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。 |

<!-- write-step: 2 bytes=538:698 -->
````go
type DraftSources struct {
	pulls       *logic.PullMain
	controls    *logic.DraftControls
	reverse     *logic.ReverseChanges
	resolutions *logic.Resolutions
}

````

## 第 3 步：NewDraftSources

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func NewDraftSources(pulls *logic.PullMain, controls *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions) *DraftSources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pulls</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。<br>在此声明中的实际使用：<code>pulls: pulls</code> |
| <code>controls</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。<br>在此声明中的实际使用：<code>controls: controls</code> |
| <code>reverse</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。<br>在此声明中的实际使用：<code>reverse: reverse</code> |
| <code>resolutions</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。<br>在此声明中的实际使用：<code>resolutions: resolutions</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftSources</code> | 实际返回表达式：<code>&amp;DraftSources{pulls: pulls, controls: controls, reverse: reverse, resolutions: resolutions}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftSources{pulls: pulls, controls: controls, reverse: reverse, resolutions: resolutions}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=698:955 -->
````go
func NewDraftSources(pulls *logic.PullMain, controls *logic.DraftControls, reverse *logic.ReverseChanges, resolutions *logic.Resolutions) *DraftSources {
	return &DraftSources{pulls: pulls, controls: controls, reverse: reverse, resolutions: resolutions}
}

````

## 第 4 步：DraftSources.Source

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func (h *DraftSources) Source(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_SOURCE_INVALID&quot;)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_SOURCE_PURPOSE_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(raw, input) != nil &#124;&#124; input.Context == nil &#124;&#124; input.Context.Target == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=955:2055 -->
````go
func (h *DraftSources) Source(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	input := &pb.DraftPreviewRequest{}
	if protojson.Unmarshal(raw, input) != nil || input.Context == nil || input.Context.Target == nil {
		httpx.Fail(w, 400, "DRAFT_SOURCE_INVALID")
		return
	}
	// 证书只识别协作服务；具体正文资格必须由原操作用途分派并重新核验，不能靠客户端指定候选。
	var value map[string]any
	var err error
	switch input.Context.Target.Purpose {
	case "PULL_MAIN":
		value, err = h.pulls.DraftSource(r.Context(), input)
	case "DISCARD_PREVIEW":
		value, err = h.controls.PreviewSource(r.Context(), input)
	case "DISCARD_DRAFT":
		value, err = h.controls.DiscardSource(r.Context(), input)
	case "REVERSE":
		value, err = h.reverse.Source(r.Context(), input)
	case "CONFLICT_RESOLUTION":
		value, err = h.resolutions.Source(r.Context(), input)
	default:
		httpx.Fail(w, 400, "DRAFT_SOURCE_PURPOSE_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

````

## 第 5 步：DraftSources.Committed

插入位置：第 4 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func (h *DraftSources) Committed(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_COMMIT_QUERY_INVALID&quot;)</code>；<code>httpx.Fail(w, 400, &quot;DRAFT_COMMIT_PURPOSE_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>mainSourceInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>raw == nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>protojson.Unmarshal(raw, query) != nil &#124;&#124; query.Target == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2055:2999 -->
````go
func (h *DraftSources) Committed(w http.ResponseWriter, r *http.Request) {
	raw := mainSourceInput(w, r)
	if raw == nil {
		return
	}
	query := &pb.OperationQuery{}
	if protojson.Unmarshal(raw, query) != nil || query.Target == nil {
		httpx.Fail(w, 400, "DRAFT_COMMIT_QUERY_INVALID")
		return
	}
	var value map[string]any
	var err error
	switch query.Target.Purpose {
	case "PULL_MAIN":
		value, err = h.pulls.CommittedSource(r.Context(), query)
	case "DISCARD_DRAFT":
		value, err = h.controls.DiscardCommittedSource(r.Context(), query)
	case "REVERSE":
		value, err = h.reverse.CommittedSource(r.Context(), query)
	case "CONFLICT_RESOLUTION":
		value, err = h.resolutions.CommittedSource(r.Context(), query)
	default:
		httpx.Fail(w, 400, "DRAFT_COMMIT_PURPOSE_INVALID")
		return
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, value)
}

// response_output.go：保留原实现，按业务合并文件。

````

## 第 6 步：ResponseOutput

插入位置：第 5 步代码之后，紧接追加；完成稿约第 101 行。

done只能在实际写入与Flush结束后关闭，取消信号本身不构成排空ACK。

本步声明：`type ResponseOutput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>m</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |
| <code>local</code> | <code>domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。 |
| <code>edge</code> | <code>*domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。 |
| <code>done</code> | <code>chan struct{}</code> | 实际响应写入和Flush结束后关闭的信号；取消请求本身不能关闭它冒充排空。 |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。 |
| <code>stopped</code> | <code>chan struct{}</code> | 权限撤销要求停止发送的信号，写出敏感字节前重复检查。 |
| <code>once</code> | <code>sync.Once</code> | 确保停止路径的信号/状态只执行一次，避免并发重复close。 |

<!-- write-step: 6 bytes=2999:3300 -->
````go
// done只能在实际写入与Flush结束后关闭，取消信号本身不构成排空ACK。
type ResponseOutput struct {
	m       *logic.Outputs
	local   domain.OutputLease
	edge    *domain.OutputLease
	done    chan struct{}
	w       http.ResponseWriter
	stopped chan struct{}
	once    sync.Once
}

````

## 第 7 步：ResponseOutput.Abort

插入位置：第 6 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func (o *ResponseOutput) Abort()`。

<!-- write-step: 7 bytes=3300:3383 -->
````go
func (o *ResponseOutput) Abort() { close(o.done); o.m.Local().Remove(o.local.ID) }
````

## 第 8 步：ResponseOutput.Send

插入位置：第 7 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func (o *ResponseOutput) Send(ctx context.Context, status int, value any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.send(ctx, status, value, 0)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>o.send(ctx, status, value, 0)</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>o.send(ctx, status, value, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>o.send(ctx, status, value, 0)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return o.send(ctx, status, value, 0)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3383:3506 -->
````go
func (o *ResponseOutput) Send(ctx context.Context, status int, value any) error {
	return o.send(ctx, status, value, 0)
}

````

## 第 9 步：ResponseOutput.SendBounded

插入位置：第 8 步代码之后，紧接追加；完成稿约第 117 行。

固定整篇正文使用独立有限预算；编码失败或超限时尚未写出成功头/敏感字节。

本步声明：`func (o *ResponseOutput) SendBounded(ctx context.Context, status int, value any, budget int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.send(ctx, status, value, budget)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>o.send(ctx, status, value, budget)</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>o.send(ctx, status, value, budget)</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>o.send(ctx, status, value, budget)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>o.send(ctx, status, value, budget)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return o.send(ctx, status, value, budget)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3506:3763 -->
````go
// 固定整篇正文使用独立有限预算；编码失败或超限时尚未写出成功头/敏感字节。
func (o *ResponseOutput) SendBounded(ctx context.Context, status int, value any, budget int) error {
	return o.send(ctx, status, value, budget)
}

````

## 第 10 步：ResponseOutput.send

插入位置：第 9 步代码之后，紧接追加；完成稿约第 122 行。

本步声明：`func (o *ResponseOutput) send(ctx context.Context, status int, value any, budget int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.open(ctx)</code>；<code>o.checkStopped(ctx)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>o.w.WriteHeader(status)</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(value)</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>budget &gt; 0</code>；<code>len(raw) &gt; budget</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_RESPONSE_TRANSFER_LIMIT&quot;)</code>；<code>http.NewResponseController(o.w).Flush()</code> |

**跟着本段代码走一遍：**

- 登记 <code>o.finish()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>budget &gt; 0 &amp;&amp; len(raw) &gt; budget</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;FIXED_RESPONSE_TRANSFER_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return http.NewResponseController(o.w).Flush()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3763:4629 -->
````go
func (o *ResponseOutput) send(ctx context.Context, status int, value any, budget int) error {
	defer o.finish()
	if err := o.open(ctx); err != nil {
		return err
	}
	_ = http.NewResponseController(o.w).SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if budget > 0 && len(raw) > budget {
		return domain.Violation(domain.TemporarilyUnavailable, "FIXED_RESPONSE_TRANSFER_LIMIT")
	}
	// 设置期限与构造正文后再检查，避免覆盖撤权时设置的立即停止期限。
	if err := o.checkStopped(ctx); err != nil {
		return err
	}
	o.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	o.w.Header().Set("Cache-Control", "no-store")
	o.w.WriteHeader(status)
	if _, err = o.w.Write(raw); err != nil {
		return err
	}
	return http.NewResponseController(o.w).Flush()
}

````

## 第 11 步：ResponseOutput.open

插入位置：第 10 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func (o *ResponseOutput) open(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>o.m.EdgeCall(ctx, &quot;/infra/output/open&quot;, o.edge, &amp;result)</code>；<code>o.checkStopped(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_BINDING_MISMATCH&quot;)</code>；<code>o.checkStopped(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>o.edge != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != &quot;OPEN&quot; &#124;&#124; result.ID != o.edge.Instance</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_BINDING_MISMATCH&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return o.checkStopped(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4629:5051 -->
````go
func (o *ResponseOutput) open(ctx context.Context) error {
	if o.edge != nil {
		var result struct {
			State string `json:"state"`
			ID    string `json:"instance_id"`
		}
		if err := o.m.EdgeCall(ctx, "/infra/output/open", o.edge, &result); err != nil {
			return err
		}
		if result.State != "OPEN" || result.ID != o.edge.Instance {
			return errors.New("OUTPUT_BINDING_MISMATCH")
		}
	}
	return o.checkStopped(ctx)
}

````

## 第 12 步：ResponseOutput.checkStopped

插入位置：第 11 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func (o *ResponseOutput) checkStopped(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>ctx.Err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_STOPPED&quot;)</code>；<code>ctx.Err()</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;OUTPUT_STOPPED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ctx.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5051:5252 -->
````go
func (o *ResponseOutput) checkStopped(ctx context.Context) error {
	select {
	case <-o.stopped:
		return errors.New("OUTPUT_STOPPED")
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
````

## 第 13 步：ResponseOutput.finish

插入位置：第 12 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func (o *ResponseOutput) finish()`。

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(context.Background(), time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>o.m.Settle(ctx, o.local.ID, o.local.Instance) == nil</code>。

<!-- write-step: 13 bytes=5252:5553 -->
````go
func (o *ResponseOutput) finish() {
	close(o.done)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if o.m.Settle(ctx, o.local.ID, o.local.Instance) == nil {
		o.m.Local().Remove(o.local.ID)
	}
}

// transactions.go：保留原实现，按业务合并文件。

````

## 第 14 步：Transactions

插入位置：第 13 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`type Transactions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Transactions</code> | 使用 <code>logic.Transactions</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。 |

<!-- write-step: 14 bytes=5553:5630 -->
````go
type Transactions struct {
	usecase *logic.Transactions
	outputs *Outputs
}

````

## 第 15 步：NewTransactions

插入位置：第 14 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func NewTransactions(usecase *logic.Transactions, outputs *Outputs) *Transactions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Transactions</code> | 使用 <code>logic.Transactions</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。<br>在此声明中的实际使用：<code>outputs: outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Transactions</code> | 实际返回表达式：<code>&amp;Transactions{usecase: usecase, outputs: outputs}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Transactions{usecase: usecase, outputs: outputs}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5630:5774 -->
````go
func NewTransactions(usecase *logic.Transactions, outputs *Outputs) *Transactions {
	return &Transactions{usecase: usecase, outputs: outputs}
}
````

## 第 16 步：Transactions.Read

插入位置：第 15 步代码之后，紧接追加；完成稿约第 193 行。

本步声明：`func (h *Transactions) Read(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)</code> |

<!-- write-step: 16 bytes=5774:6014 -->
````go
func (h *Transactions) Read(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody) {
	h.ReadProjected(w, r, actor, sources, document, purpose, run, nil)
}

````

## 第 17 步：Transactions.ReadProjected

插入位置：第 16 步代码之后，紧接追加；完成稿约第 197 行。

业务提交之后才编码传输投影；HTTP头与响应对象不进入用例事务。

本步声明：`func (h *Transactions) ReadProjected(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |
| <code>present</code> | <code>func(any) any</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)</code> |

<!-- write-step: 17 bytes=6014:6394 -->
````go
// 业务提交之后才编码传输投影；HTTP头与响应对象不进入用例事务。
func (h *Transactions) ReadProjected(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any) {
	h.readStatus(w, r, actor, sources, document, purpose, run, present, http.StatusOK)
}

````

## 第 18 步：Transactions.ReadAccepted

插入位置：第 17 步代码之后，紧接追加；完成稿约第 202 行。

已登记的异步操作也通过真实输出租约发送202；状态码适配不能绕过撤权与输出生命周期。

本步声明：`func (h *Transactions) ReadAccepted(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)</code> |

<!-- write-step: 18 bytes=6394:6784 -->
````go
// 已登记的异步操作也通过真实输出租约发送202；状态码适配不能绕过撤权与输出生命周期。
func (h *Transactions) ReadAccepted(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody) {
	h.readStatus(w, r, actor, sources, document, purpose, run, nil, http.StatusAccepted)
}

````

## 第 19 步：Transactions.readStatus

插入位置：第 18 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`func (h *Transactions) readStatus(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status int)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>present</code> | <code>func(any) any</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)</code> |

<!-- write-step: 19 bytes=6784:7084 -->
````go
func (h *Transactions) readStatus(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status int) {
	h.readStatusBounded(w, r, actor, sources, document, purpose, run, present, status, 0)
}

````

## 第 20 步：Transactions.ReadBounded

插入位置：第 19 步代码之后，紧接追加；完成稿约第 211 行。

本步声明：`func (h *Transactions) ReadBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, budget int)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)</code> |

<!-- write-step: 20 bytes=7084:7370 -->
````go
func (h *Transactions) ReadBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, budget int) {
	h.readStatusBounded(w, r, actor, sources, document, purpose, run, nil, http.StatusOK, budget)
}

````

## 第 21 步：Transactions.readStatusBounded

插入位置：第 20 步代码之后，紧接追加；完成稿约第 215 行。

本步声明：`func (h *Transactions) readStatusBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status, budget int)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.outputs.Bind(w, registration, sources)</code>；<code>httpx.Failure(w, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code>；<code>r.Header</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get(&quot;X-Edge-Handler&quot;), run, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic. …</code>（长表达式，按本步完整代码继续） |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get(&quot;X-Edge-Handler&quot;), run, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic. …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get(&quot;X-Edge-Handler&quot;), run, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic. …</code>（长表达式，按本步完整代码继续） |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get(&quot;X-Edge-Handler&quot;), run, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic. …</code>（长表达式，按本步完整代码继续） |
| <code>run</code> | <code>logic.ReadBody</code> | 使用 <code>logic.ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get(&quot;X-Edge-Handler&quot;), run, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic. …</code>（长表达式，按本步完整代码继续） |
| <code>present</code> | <code>func(any) any</code> | 在此声明中的实际使用：<code>present != nil</code>；<code>present(value)</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>output.SendBounded(r.Context(), status, value, budget)</code> |
| <code>budget</code> | <code>int</code> | 在此声明中的实际使用：<code>output.SendBounded(r.Context(), status, value, budget)</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>present != nil</code>。
- 检查 <code>err != nil</code>。

<!-- write-step: 21 bytes=7370:8117 -->
````go
func (h *Transactions) readStatusBounded(w http.ResponseWriter, r *http.Request, actor logic.Actor, sources []*pb.SourceEpoch, document uint64, purpose string, run logic.ReadBody, present func(any) any, status, budget int) {
	var output *ResponseOutput
	value, err := h.usecase.Read(r.Context(), actor, sources, document, purpose, r.Header.Get("X-Edge-Handler"), run, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if present != nil {
		value = present(value)
	}
	if err := output.SendBounded(r.Context(), status, value, budget); err != nil {
		httpx.Failure(w, err)
	}
}
````

## 第 22 步：Transactions.PresentedMutation

插入位置：第 21 步代码之后，紧接追加；完成稿约第 232 行。

本步声明：`func (h *Transactions) PresentedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, run logic.MutationBody, present logic.MutationPresentation)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code>；<code>r.URL</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>ID: id</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>Kind: kind</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>Input: input</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code> |
| <code>run</code> | <code>logic.MutationBody</code> | 使用 <code>logic.MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code> |
| <code>present</code> | <code>logic.MutationPresentation</code> | 使用 <code>logic.MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)</code> |

<!-- write-step: 22 bytes=8117:8452 -->
````go
func (h *Transactions) PresentedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, run logic.MutationBody, present logic.MutationPresentation) {
	h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}, status, run, present)
}

````

## 第 23 步：Transactions.PreparedMutation

插入位置：第 22 步代码之后，紧接追加；完成稿约第 236 行。

本步声明：`func (h *Transactions) PreparedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, initial map[string]any, run logic.MutationBody, present logic.MutationPresentation)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code>；<code>r.URL</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>ID: id</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>Kind: kind</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>Input: input</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code> |
| <code>initial</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>InitialResult: initial</code> |
| <code>run</code> | <code>logic.MutationBody</code> | 使用 <code>logic.MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code> |
| <code>present</code> | <code>logic.MutationPresentation</code> | 使用 <code>logic.MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)</code> |

<!-- write-step: 23 bytes=8452:8834 -->
````go
func (h *Transactions) PreparedMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, initial map[string]any, run logic.MutationBody, present logic.MutationPresentation) {
	h.presentMutation(w, r, actor, logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input, InitialResult: initial}, status, run, present)
}

````

## 第 24 步：Transactions.presentMutation

插入位置：第 23 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func (h *Transactions) presentMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, command logic.OperationCommand, status int, run logic.MutationBody, present logic.MutationPresentation)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.outputs.Bind(w, registration, sources)</code>；<code>httpx.Failure(w, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code>；<code>r.Header</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.usecase.Mutation(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), run, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Output …</code>（长表达式，按本步完整代码继续） |
| <code>command</code> | <code>logic.OperationCommand</code> | 使用 <code>logic.OperationCommand</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.Mutation(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), run, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Output …</code>（长表达式，按本步完整代码继续） |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>output.Send(r.Context(), status, value)</code> |
| <code>run</code> | <code>logic.MutationBody</code> | 使用 <code>logic.MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.Mutation(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), run, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Output …</code>（长表达式，按本步完整代码继续） |
| <code>present</code> | <code>logic.MutationPresentation</code> | 使用 <code>logic.MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.Mutation(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), run, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Output …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。

<!-- write-step: 24 bytes=8834:9490 -->
````go
func (h *Transactions) presentMutation(w http.ResponseWriter, r *http.Request, actor logic.Actor, command logic.OperationCommand, status int, run logic.MutationBody, present logic.MutationPresentation) {
	var output *ResponseOutput
	value, err := h.usecase.Mutation(r.Context(), actor, command, r.Header.Get("X-Edge-Handler"), run, present, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if err := output.Send(r.Context(), status, value); err != nil {
		httpx.Failure(w, err)
	}
}
````

## 第 25 步：Transactions.Replay

插入位置：第 24 步代码之后，紧接追加；完成稿约第 254 行。

本步声明：`func (h *Transactions) Replay(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, policy logic.ReplayPolicy, present logic.MutationPresentation) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.outputs.Bind(w, registration, sources)</code>；<code>httpx.Failure(w, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.URL</code>；<code>r.Context</code>；<code>r.Header</code> |
| <code>actor</code> | <code>logic.Actor</code> | 使用 <code>logic.Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>h.usecase.TryReplay(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), policy, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Ou …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>ID: id</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>Kind: kind</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>Input: input</code> |
| <code>status</code> | <code>int</code> | 在此声明中的实际使用：<code>output.Send(r.Context(), status, value)</code> |
| <code>policy</code> | <code>logic.ReplayPolicy</code> | 使用 <code>logic.ReplayPolicy</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.TryReplay(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), policy, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Ou …</code>（长表达式，按本步完整代码继续） |
| <code>present</code> | <code>logic.MutationPresentation</code> | 使用 <code>logic.MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>h.usecase.TryReplay(r.Context(), actor, command, r.Header.Get(&quot;X-Edge-Handler&quot;), policy, present, func(registration *logic.ResponseRegistration, sources &#91;&#93;*pb.SourceEpoch) logic.Ou …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!handled</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=9490:10292 -->
````go
func (h *Transactions) Replay(w http.ResponseWriter, r *http.Request, actor logic.Actor, id, kind string, input any, status int, policy logic.ReplayPolicy, present logic.MutationPresentation) bool {
	var output *ResponseOutput
	command := logic.OperationCommand{ID: id, Kind: kind, Path: r.URL.Path, Input: input}
	handled, value, err := h.usecase.TryReplay(r.Context(), actor, command, r.Header.Get("X-Edge-Handler"), policy, present, func(registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) logic.OutputAborter {
		output = h.outputs.Bind(w, registration, sources)
		return output
	})
	if !handled {
		return false
	}
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	if err := output.Send(r.Context(), status, value); err != nil {
		httpx.Failure(w, err)
	}
	return true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/operations_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
