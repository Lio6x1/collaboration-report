# 分步编写 `services/document/internal/handler/document/outputs_handlers.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/256-outputs-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将真实HTTP取消/完成回调绑定输出登记，并提供编辑会话与内部结算入口。

业务层在COMMIT前登记的输出需要与实际ResponseWriter连接，才能在撤权时取消并等待完成。

写之前先按这条执行路线理解各段的连接：

1. Outputs.Bind建立ResponseOutput和本地done
2. EditSessions.Create发票据
3. Bootstrap按准入提供基线
4. RegisterCollaboration登记WSS
5. Settled查询原流终态。

本文件需要守住的边界：

- 不能先提交再绑定回调
- 内部控制身份仍须检查，调用者不能任意修改固定来源。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/outputs_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/outputs_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/outputs_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/outputs_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:568 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"net/http"
	"strings"
	"time"
)

// collaboration_output.go：保留原实现，按业务合并文件。

````

## 第 2 步：Outputs.RegisterCollaboration

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (h *Outputs) RegisterCollaboration(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Fail(w, 400, &quot;OUTPUT_BINDING_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.Handler)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=568:1419 -->
````go
func (h *Outputs) RegisterCollaboration(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Admission string `json:"admission_id"`
		Grant     string `json:"grant_id"`
		Handler   string `json:"handler_id"`
	}
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.Handler) {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	lease, err := h.usecase.RegisterCollaboration(r.Context(), domain.CollaborationOutput{Admission: input.Admission, Grant: input.Grant, Handler: input.Handler})
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "OPEN", "lease_id": lease})
}

// edit_sessions.go：保留原实现，按业务合并文件。

````

## 第 3 步：EditSessions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`type EditSessions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.EditSessions</code> | 使用 <code>logic.EditSessions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。 |

<!-- write-step: 3 bytes=1419:1523 -->
````go
type EditSessions struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.EditSessions
}

````

## 第 4 步：NewEditSessions

插入位置：第 3 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`func NewEditSessions(identity *Identity, requests *Transactions, usecase *logic.EditSessions) *EditSessions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.EditSessions</code> | 使用 <code>logic.EditSessions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EditSessions</code> | 实际返回表达式：<code>&amp;EditSessions{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EditSessions{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1523:1715 -->
````go
func NewEditSessions(identity *Identity, requests *Transactions, usecase *logic.EditSessions) *EditSessions {
	return &EditSessions{identity: identity, requests: requests, usecase: usecase}
}
````

## 第 5 步：EditSessions.Create

插入位置：第 4 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (h *EditSessions) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Fail(w, 400, &quot;EDIT_SESSION_INVALID&quot;)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;EDIT_SESSION&quot;, input, 200, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>h.identity.Actor(r, true, &quot;EDIT_SESSION&quot;, document)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidOperationID(input.OperationId) &#124;&#124; !domain.ValidOperationID(input.ClientSessionId) &#124;&#124; (input.WorkspaceKind != &quot;BRANCH&quot; &amp;&amp; input.WorkspaceKind != &quot;MAIN&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1715:2700 -->
````go
func (h *EditSessions) Create(w http.ResponseWriter, r *http.Request) {
	document, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	unit, err := domain.CanonicalID(pathvar.Vars(r)["unitId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	var input dto.EditSessionCreate
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	if !domain.ValidOperationID(input.OperationId) || !domain.ValidOperationID(input.ClientSessionId) || (input.WorkspaceKind != "BRANCH" && input.WorkspaceKind != "MAIN") {
		httpx.Fail(w, 400, "EDIT_SESSION_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, "EDIT_SESSION", document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, run, present, err := h.usecase.Create(r.Context(), actor, document, unit, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "EDIT_SESSION", input, 200, run, present)
}
````

## 第 6 步：EditSessions.Bootstrap

插入位置：第 5 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func (h *EditSessions) Bootstrap(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, result)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2700:3375 -->
````go
func (h *EditSessions) Bootstrap(w http.ResponseWriter, r *http.Request) {
	// 服务证书只授权这个内部入口，实际准入/来源/工作区仍由业务用例持锁核验。
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Admission string `json:"admission_id"`
		Grant     string `json:"grant_id"`
	}
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	result, err := h.usecase.Bootstrap(r.Context(), input.Admission, input.Grant)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

// outputs.go：保留原实现，按业务合并文件。

````

## 第 7 步：Outputs

插入位置：第 6 步代码之后，紧接追加；完成稿约第 111 行。

本步声明：`type Outputs struct{ usecase *logic.Outputs }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |

<!-- write-step: 7 bytes=3375:3422 -->
````go
type Outputs struct{ usecase *logic.Outputs }

````

## 第 8 步：NewOutputs

插入位置：第 7 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func NewOutputs(usecase *logic.Outputs) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 实际返回表达式：<code>&amp;Outputs{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3422:3510 -->
````go
func NewOutputs(usecase *logic.Outputs) *Outputs { return &Outputs{usecase: usecase} }

````

## 第 9 步：Outputs.Bind

插入位置：第 8 步代码之后，紧接追加；完成稿约第 115 行。

Bind登记真实取消/完成回调。用例在COMMIT前调用，传输层随后才发送正文。

本步声明：`func (h *Outputs) Bind(w http.ResponseWriter, registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) *ResponseOutput`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>w: w</code>；<code>http.NewResponseController(w)</code> |
| <code>registration</code> | <code>*logic.ResponseRegistration</code> | 使用 <code>logic.ResponseRegistration</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>registration.Local</code>；<code>registration.Edge</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>h.usecase.Local().Add(output.local.ID, sources, func() { 		output.once.Do(func() { close(output.stopped); _ = http.NewResponseController(w).SetWriteDeadline(time.Now()) }) 	}, output.done)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResponseOutput</code> | 使用 <code>ResponseOutput</code> 类型；其定义处的职责：done只能在实际写入与Flush结束后关闭，取消信号本身不构成排空ACK。<br>实际返回表达式：<code>output</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return output</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3510:4108 -->
````go
// Bind登记真实取消/完成回调。用例在COMMIT前调用，传输层随后才发送正文。
func (h *Outputs) Bind(w http.ResponseWriter, registration *logic.ResponseRegistration, sources []*pb.SourceEpoch) *ResponseOutput {
	output := &ResponseOutput{m: h.usecase, local: registration.Local, edge: registration.Edge, done: make(chan struct{}), w: w, stopped: make(chan struct{})}
	h.usecase.Local().Add(output.local.ID, sources, func() {
		output.once.Do(func() { close(output.stopped); _ = http.NewResponseController(w).SetWriteDeadline(time.Now()) })
	}, output.done)
	return output
}
````

## 第 10 步：Outputs.RegisterAccount

插入位置：第 9 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`func (h *Outputs) RegisterAccount(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Fail(w, 400, &quot;OUTPUT_BINDING_INVALID&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;string{&quot;state&quot;: &quot;OPEN&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;input) != nil &#124;&#124; !domain.ValidOperationID(input.Handler) &#124;&#124; len(input.Sources) == 0 &#124;&#124; len(input.Sources) &gt; 512</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; !platform.ValidAccountOutputScope(wire.Key)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4108:5237 -->
````go
func (h *Outputs) RegisterAccount(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		Handler string               `json:"handler_id"`
		Sources []domain.LeaseSource `json:"sources"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 65536))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || !domain.ValidOperationID(input.Handler) || len(input.Sources) == 0 || len(input.Sources) > 512 {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	sources := make([]*pb.SourceEpoch, 0, len(input.Sources))
	for _, wire := range input.Sources {
		epoch, err := domain.CanonicalID(wire.Epoch)
		if err != nil || !platform.ValidAccountOutputScope(wire.Key) {
			httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
			return
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: wire.Key, Epoch: epoch})
	}
	if err := h.usecase.RegisterAccount(r.Context(), input.Handler, sources); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "OPEN"})
}
````

## 第 11 步：Outputs.Settled

插入位置：第 10 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func (h *Outputs) Settled(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Fail(w, 400, &quot;OUTPUT_BINDING_INVALID&quot;)</code>；<code>httpx.Fail(w, 503, &quot;OUTPUT_SETTLEMENT_UNKNOWN&quot;)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;string{&quot;state&quot;: &quot;SETTLED&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;edge&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;edge&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&amp;input) != nil &#124;&#124; input.Instance != strings.TrimSuffix(h.usecase.Self().ID, &quot;-document&quot;)+&quot;-edge&quot;</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5237:5930 -->
````go
func (h *Outputs) Settled(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "edge") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input struct {
		ID       string `json:"lease_id"`
		Instance string `json:"instance_id"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&input) != nil || input.Instance != strings.TrimSuffix(h.usecase.Self().ID, "-document")+"-edge" {
		httpx.Fail(w, 400, "OUTPUT_BINDING_INVALID")
		return
	}
	if err := h.usecase.Settle(r.Context(), input.ID, input.Instance); err != nil {
		httpx.Fail(w, 503, "OUTPUT_SETTLEMENT_UNKNOWN")
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "SETTLED"})
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/outputs_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
