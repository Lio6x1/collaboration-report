# 分步编写 `services/document/internal/handler/document/retirement_handlers.go`

先按 [D12 · 回收恢复、永久清理与个人转移](../lessons/document/12-trash-transfer.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/260-retirement-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

适配成员离组、组织关闭、个人转移和留存治理HTTP协议。

相同Node清理形状可以复用，发证资格必须属于各生命周期原计划。

写之前先按这条执行路线理解各段的连接：

1. 严格解析departure/organization绑定
2. DispatchCleanupContext按kind分派
3. PersonalTransfers处理参与者动作
4. RetainedGovernance先授权再Replay。

本文件需要守住的边界：

- 未知查询失败不能回退为账号证明
- 关闭文档不因此得到正文读取能力。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/document/retirement_handlers.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/document/retirement_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/document/retirement_handlers.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/document/retirement_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:532 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/document/internal/handler/httpx"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	"net/http"
)

// departure.go：保留原实现，按业务合并文件。

````

## 第 2 步：Departures

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type Departures struct{ usecase *logic.Departures }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Departures</code> | 使用 <code>logic.Departures</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |

<!-- write-step: 2 bytes=532:585 -->
````go
type Departures struct{ usecase *logic.Departures }

````

## 第 3 步：NewDepartures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewDepartures(usecase *logic.Departures) *Departures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Departures</code> | 使用 <code>logic.Departures</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Departures</code> | 实际返回表达式：<code>&amp;Departures{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Departures{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=585:684 -->
````go
func NewDepartures(usecase *logic.Departures) *Departures { return &Departures{usecase: usecase} }
````

## 第 4 步：departureInput

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func departureInput(w http.ResponseWriter, r *http.Request) (domain.DepartureBinding, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;DEPARTURE_BINDING_INVALID&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>r.URL</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>实际返回表达式：<code>binding</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; !domain.ValidOperationID(binding.ID)</code>。
- 此处通过 <code>return binding, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return binding, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=684:1506 -->
````go
func departureInput(w http.ResponseWriter, r *http.Request) (domain.DepartureBinding, bool) {
	var binding domain.DepartureBinding
	var err error
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return binding, false
	}
	binding.Org, err = domain.CanonicalID(r.URL.Query().Get("organization_id"))
	if err != nil {
		httpx.Failure(w, err)
		return binding, false
	}
	binding.User, err = domain.CanonicalID(r.URL.Query().Get("user_id"))
	if err != nil {
		httpx.Failure(w, err)
		return binding, false
	}
	binding.Epoch, err = domain.CanonicalID(r.URL.Query().Get("epoch"))
	binding.ID = r.URL.Query().Get("operation_id")
	if err != nil || !domain.ValidOperationID(binding.ID) {
		httpx.Fail(w, 400, "DEPARTURE_BINDING_INVALID")
		return binding, false
	}
	return binding, true
}
````

## 第 5 步：Departures.Eligibility

插入位置：第 4 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (h *Departures) Eligibility(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>departureInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;bool{&quot;requires_handoff&quot;: admin})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>departureInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1506:1826 -->
````go
func (h *Departures) Eligibility(w http.ResponseWriter, r *http.Request) {
	binding, ok := departureInput(w, r)
	if !ok {
		return
	}
	admin, err := h.usecase.Eligibility(r.Context(), binding)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]bool{"requires_handoff": admin})
}
````

## 第 6 步：Departures.Close

插入位置：第 5 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func (h *Departures) Close(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>departureInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;string{&quot;state&quot;: &quot;CLOSED&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>departureInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1826:2193 -->
````go
func (h *Departures) Close(w http.ResponseWriter, r *http.Request) {
	binding, ok := departureInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Close(r.Context(), binding); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": "CLOSED"})
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 7 步：OrganizationClosures

插入位置：第 6 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`type OrganizationClosures struct{ usecase *logic.OrganizationClosures }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。 |

<!-- write-step: 7 bytes=2193:2266 -->
````go
type OrganizationClosures struct{ usecase *logic.OrganizationClosures }

````

## 第 8 步：NewOrganizationClosures

插入位置：第 7 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`func NewOrganizationClosures(usecase *logic.OrganizationClosures) *OrganizationClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationClosures</code> | 实际返回表达式：<code>&amp;OrganizationClosures{usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationClosures{usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2266:2406 -->
````go
func NewOrganizationClosures(usecase *logic.OrganizationClosures) *OrganizationClosures {
	return &OrganizationClosures{usecase: usecase}
}
````

## 第 9 步：organizationCloseInput

插入位置：第 8 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`func organizationCloseInput(w http.ResponseWriter, r *http.Request) (platform.OrganizationClosureBinding, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;b)</code>；<code>httpx.Fail(w, 400, &quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>w.Header</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>实际返回表达式：<code>b</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return b, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;b)</code>。条件成立时执行 <code>return b, false</code>。当前函数在这里结束。
- 检查 <code>!b.Valid()</code>。
- 此处通过 <code>return b, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return b, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2406:2890 -->
````go
func organizationCloseInput(w http.ResponseWriter, r *http.Request) (platform.OrganizationClosureBinding, bool) {
	var b platform.OrganizationClosureBinding
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return b, false
	}
	if !httpx.DecodeBody(w, r, &b) {
		return b, false
	}
	if !b.Valid() {
		httpx.Fail(w, 400, "ORGANIZATION_CLOSE_BINDING_INVALID")
		return b, false
	}
	w.Header().Set("Cache-Control", "no-store")
	return b, true
}
````

## 第 10 步：OrganizationClosures.Plan

插入位置：第 9 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`func (h *OrganizationClosures) Plan(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>organizationCloseInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: state})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>organizationCloseInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2890:3396 -->
````go
func (h *OrganizationClosures) Plan(w http.ResponseWriter, r *http.Request) {
	b, ok := organizationCloseInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Plan(r.Context(), b); err != nil {
		httpx.Failure(w, err)
		return
	}
	// 已有原号返回真实持久阶段，不把成功HTTP解释成新的规划或清理ACK。
	state, err := h.usecase.State(r.Context(), b)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: state})
}
````

## 第 11 步：OrganizationClosures.Progress

插入位置：第 10 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func (h *OrganizationClosures) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Failure(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3396:3879 -->
````go
func (h *OrganizationClosures) Progress(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.OrganizationClosurePageRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.Progress(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}
````

## 第 12 步：OrganizationClosures.Complete

插入位置：第 11 步代码之后，紧接追加；完成稿约第 130 行。

本步声明：`func (h *OrganizationClosures) Complete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>organizationCloseInput(w, r)</code>；<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: &quot;SUCCEEDED&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>organizationCloseInput(w, r)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3879:4203 -->
````go
func (h *OrganizationClosures) Complete(w http.ResponseWriter, r *http.Request) {
	b, ok := organizationCloseInput(w, r)
	if !ok {
		return
	}
	if err := h.usecase.Complete(r.Context(), b); err != nil {
		httpx.Failure(w, err)
		return
	}
	platform.WriteJSON(w, 200, platform.OrganizationClosureState{State: "SUCCEEDED"})
}
````

## 第 13 步：OrganizationClosures.CleanPayloads

插入位置：第 12 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`func (h *OrganizationClosures) CleanPayloads(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Failure(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;account&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;account&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4203:4794 -->
````go
func (h *OrganizationClosures) CleanPayloads(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "account") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.CleanPayloads(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.AccountDocumentCleanupReceipt{AccountDocumentCleanupContext: result, State: "PAYLOADS_REMOVED"})
}

````

## 第 14 步：OrganizationClosures.DocumentCleanupContext

插入位置：第 13 步代码之后，紧接追加；完成稿约第 159 行。

根在现有Node context入口按lifecycle kind分派此用例，不改变Node原协议。

本步声明：`func (h *OrganizationClosures) DocumentCleanupContext(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>httpx.Failure(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;collaboration&quot;)</code>；<code>httpx.DecodeBody(w, r, &amp;request)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4794:5485 -->
````go
// 根在现有Node context入口按lifecycle kind分派此用例，不改变Node原协议。
func (h *OrganizationClosures) DocumentCleanupContext(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "collaboration") {
		httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var request platform.AccountDocumentCleanupRequest
	if !httpx.DecodeBody(w, r, &request) {
		return
	}
	result, err := h.usecase.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

// organization_close_dispatch.go：保留原实现，按业务合并文件。

````

## 第 15 步：OrganizationClosures.DispatchCleanupContext

插入位置：第 14 步代码之后，紧接追加；完成稿约第 180 行。

路由仅包裹旧context入口：原Node协议完全不变，组织原号由自己的计划发证。
其他清理生命周期继续交给现有完整分派器；未知查询失败绝不回退账号证明。

本步声明：`func (h *OrganizationClosures) DispatchCleanupContext(fallback http.HandlerFunc) http.HandlerFunc`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fallback</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>fallback(w, r)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>实际返回表达式：<code>func(w http.ResponseWriter, r *http.Request) { 		if !platform.HTTPCaller(r, &quot;collaboration&quot;) { 			httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;) 			return 		} 		var request platfo …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(w http.ResponseWriter, r *http.Request) { 		if !platform.HTTPCaller(r, &quot;collaboration&quot;) { 			httpx.Fail(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;) 			return 		} 		var request …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!platform.HTTPCaller(r, &quot;collaboration&quot;)</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!httpx.DecodeBody(w, r, &amp;request)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!request.Valid()</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>handles</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5485:6791 -->
````go
// 路由仅包裹旧context入口：原Node协议完全不变，组织原号由自己的计划发证。
// 其他清理生命周期继续交给现有完整分派器；未知查询失败绝不回退账号证明。
func (h *OrganizationClosures) DispatchCleanupContext(fallback http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.HTTPCaller(r, "collaboration") {
			httpx.Fail(w, 403, "CONTROL_IDENTITY_REQUIRED")
			return
		}
		var request platform.AccountDocumentCleanupRequest
		if !httpx.DecodeBody(w, r, &request) {
			return
		}
		if !request.Valid() {
			httpx.Fail(w, 400, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
			return
		}
		handles, err := h.usecase.HandlesCleanup(r.Context(), request.Operation)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		if handles {
			result, err := h.usecase.DocumentCleanupContext(r.Context(), request)
			if err != nil {
				httpx.Failure(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			platform.WriteJSON(w, 200, result)
			return
		}
		raw, err := json.Marshal(request)
		if err != nil {
			httpx.Failure(w, err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		fallback(w, r)
	}
}

// personal_transfer.go：保留原实现，按业务合并文件。

````

## 第 16 步：PersonalTransfers

插入位置：第 15 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`type PersonalTransfers struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.PersonalTransfers</code> | 使用 <code>logic.PersonalTransfers</code> 类型；其定义处的职责：提供个人文档邀约、参与者列表、取消拒绝和接受登记。 |

<!-- write-step: 16 bytes=6791:6905 -->
````go
type PersonalTransfers struct {
	identity *Identity
	requests *Transactions
	usecase  *logic.PersonalTransfers
}

````

## 第 17 步：NewPersonalTransfers

插入位置：第 16 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func NewPersonalTransfers(identity *Identity, requests *Transactions, usecase *logic.PersonalTransfers) *PersonalTransfers`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.PersonalTransfers</code> | 使用 <code>logic.PersonalTransfers</code> 类型；其定义处的职责：提供个人文档邀约、参与者列表、取消拒绝和接受登记。<br>在此声明中的实际使用：<code>usecase: usecase</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*PersonalTransfers</code> | 实际返回表达式：<code>&amp;PersonalTransfers{identity: identity, requests: requests, usecase: usecase}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;PersonalTransfers{identity: identity, requests: requests, usecase: usecase}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6905:7118 -->
````go
func NewPersonalTransfers(identity *Identity, requests *Transactions, usecase *logic.PersonalTransfers) *PersonalTransfers {
	return &PersonalTransfers{identity: identity, requests: requests, usecase: usecase}
}

````

## 第 18 步：PersonalTransfers.Offer

插入位置：第 17 步代码之后，紧接追加；完成稿约第 233 行。

本步声明：`func (h *PersonalTransfers) Offer(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_OFFER&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_OFFER&quot;, input, 202, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;PERSONAL_TRANSFER_OFFER&quot;, id)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_OFFER&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_OFFER&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7118:7887 -->
````go
func (h *PersonalTransfers) Offer(w http.ResponseWriter, r *http.Request) {
	var input dto.OwnerChange
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PERSONAL_TRANSFER_OFFER", id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_OFFER", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Offer(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_OFFER", input, 202, run, nil)
}

````

## 第 19 步：PersonalTransfers.Detail

插入位置：第 18 步代码之后，紧接追加；完成稿约第 259 行。

本步声明：`func (h *PersonalTransfers) Detail(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;PERSONAL_TRANSFER_DETAIL&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, &quot;PERSONAL_TRANSFER_DETAIL&quot;, 0)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;PERSONAL_TRANSFER_DETAIL&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=7887:8399 -->
````go
func (h *PersonalTransfers) Detail(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, "PERSONAL_TRANSFER_DETAIL", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	sources, run, err := h.usecase.Detail(r.Context(), actor, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "PERSONAL_TRANSFER_DETAIL", run)
}

````

## 第 20 步：PersonalTransfers.List

插入位置：第 19 步代码之后，紧接追加；完成稿约第 278 行。

本步声明：`func (h *PersonalTransfers) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;PERSONAL_TRANSFER_LIST&quot;, run)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.identity.Actor(r, false, &quot;PERSONAL_TRANSFER_LIST&quot;, 0)</code>；<code>r.URL</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, sources, 0, &quot;PERSONAL_TRANSFER_LIST&quot;, run)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8399:8857 -->
````go
func (h *PersonalTransfers) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.identity.Actor(r, false, "PERSONAL_TRANSFER_LIST", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	q := r.URL.Query()
	sources, run, err := h.usecase.List(r.Context(), actor, q.Get("direction"), q.Get("limit"), q.Get("cursor"))
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, sources, 0, "PERSONAL_TRANSFER_LIST", run)
}

````

## 第 21 步：PersonalTransfers.Accept

插入位置：第 20 步代码之后，紧接追加；完成稿约第 293 行。

本步声明：`func (h *PersonalTransfers) Accept(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, input, 202, run, present)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, &quot;PERSONAL_TRANSFER_ACCEPT&quot;, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=8857:9752 -->
````go
func (h *PersonalTransfers) Accept(w http.ResponseWriter, r *http.Request) {
	var input dto.PersonalTransferAccept
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, true, "PERSONAL_TRANSFER_ACCEPT", 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 未知接受只能读取标准 fingerprint 核验后的原 receipt，不再次预留容量。
	if h.requests.Replay(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_ACCEPT", input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Accept(r.Context(), actor, id, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, "PERSONAL_TRANSFER_ACCEPT", input, 202, run, present)
}

````

## 第 22 步：PersonalTransfers.Cancel

插入位置：第 21 步代码之后，紧接追加；完成稿约第 320 行。

本步声明：`func (h *PersonalTransfers) Cancel(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.decide(w, r, &quot;CANCEL&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.decide(w, r, &quot;CANCEL&quot;)</code> |

<!-- write-step: 22 bytes=9752:9856 -->
````go
func (h *PersonalTransfers) Cancel(w http.ResponseWriter, r *http.Request) { h.decide(w, r, "CANCEL") }
````

## 第 23 步：PersonalTransfers.Reject

插入位置：第 22 步代码之后，紧接追加；完成稿约第 321 行。

本步声明：`func (h *PersonalTransfers) Reject(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.decide(w, r, &quot;REJECT&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.decide(w, r, &quot;REJECT&quot;)</code> |

<!-- write-step: 23 bytes=9856:9961 -->
````go
func (h *PersonalTransfers) Reject(w http.ResponseWriter, r *http.Request) { h.decide(w, r, "REJECT") }

````

## 第 24 步：PersonalTransfers.decide

插入位置：第 23 步代码之后，紧接追加；完成稿约第 323 行。

本步声明：`func (h *PersonalTransfers) decide(w http.ResponseWriter, r *http.Request, action string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.ValidateOperationFirst, nil)</code>；<code>h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, 202, run, nil)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, kind, 0)</code>；<code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.ValidateOperationFirst, nil)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;PERSONAL_TRANSFER_&quot; + action</code>；<code>h.usecase.Decide(r.Context(), actor, id, input, action)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=9961:10811 -->
````go
func (h *PersonalTransfers) decide(w http.ResponseWriter, r *http.Request, action string) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["transferId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	kind := "PERSONAL_TRANSFER_" + action
	actor, err := h.identity.Actor(r, true, kind, 0)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, 202, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, err := h.usecase.Decide(r.Context(), actor, id, input, action)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, 202, run, nil)
}

// retained_cleanup_context.go：保留原实现，按业务合并文件。

````

## 第 25 步：AccountRetirements.SetRetainedGovernance

插入位置：第 24 步代码之后，紧接追加；完成稿约第 352 行。

本步声明：`func (h *AccountRetirements) SetRetainedGovernance(usecase *logic.RetainedGovernance)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。<br>在此声明中的实际使用：<code>h.retainedGovernance = usecase</code> |

<!-- write-step: 25 bytes=10811:10934 -->
````go
func (h *AccountRetirements) SetRetainedGovernance(usecase *logic.RetainedGovernance) {
	h.retainedGovernance = usecase
}

````

## 第 26 步：AccountRetirements.retainedGovernanceCleanupContext

插入位置：第 25 步代码之后，紧接追加；完成稿约第 356 行。

本步声明：`func (h *AccountRetirements) retainedGovernanceCleanupContext(w http.ResponseWriter, r *http.Request, request platform.AccountDocumentCleanupRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>platform.WriteJSON(w, http.StatusOK, struct { 		platform.AccountDocumentCleanupContext 		CleanupKind string `json:&quot;cleanup_kind&quot;` 	}{AccountDocumentCleanupContext: result, CleanupKind: &quot;PURGE&quot;})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Context</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Operation</code>；<code>h.retainedGovernance.DocumentCleanupContext(r.Context(), request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>h.retainedGovernance == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=10934:11850 -->
````go
func (h *AccountRetirements) retainedGovernanceCleanupContext(w http.ResponseWriter, r *http.Request, request platform.AccountDocumentCleanupRequest) bool {
	if h.retainedGovernance == nil {
		return false
	}
	_, err := h.retainedGovernance.Lookup(r.Context(), request.Operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	result, err := h.retainedGovernance.DocumentCleanupContext(r.Context(), request)
	if err != nil {
		httpx.Failure(w, err)
		return true
	}
	// 复用真实PURGE清理墓碑，回执仍沿冻结context；内部kind不进入CLEANED摘要。
	platform.WriteJSON(w, http.StatusOK, struct {
		platform.AccountDocumentCleanupContext
		CleanupKind string `json:"cleanup_kind"`
	}{AccountDocumentCleanupContext: result, CleanupKind: "PURGE"})
	return true
}

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 27 步：RetainedGovernance

插入位置：第 26 步代码之后，紧接追加；完成稿约第 383 行。

本步声明：`type RetainedGovernance struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。 |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。 |
| <code>usecase</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。 |
| <code>otherOperations</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。 |

<!-- write-step: 27 bytes=11850:12021 -->
````go
type RetainedGovernance struct {
	identity        *Identity
	requests        *Transactions
	usecase         *logic.RetainedGovernance
	otherOperations http.HandlerFunc
}

````

## 第 28 步：NewRetainedGovernance

插入位置：第 27 步代码之后，紧接追加；完成稿约第 390 行。

本步声明：`func NewRetainedGovernance(identity *Identity, requests *Transactions, usecase *logic.RetainedGovernance, other http.HandlerFunc) *RetainedGovernance`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：把授权、邀请和申请HTTP请求转换成已鉴权业务调用及受控输出。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>requests</code> | <code>*Transactions</code> | 使用 <code>Transactions</code> 类型；其定义处的职责：将事务结果接入真实HTTP输出生命周期，并路由草稿私有source请求。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>usecase</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。<br>在此声明中的实际使用：<code>usecase: usecase</code> |
| <code>other</code> | <code>http.HandlerFunc</code> | 处理当前HTTP请求的函数依赖，负责写响应；共享handler实例不保存某一次请求的可变状态。<br>在此声明中的实际使用：<code>otherOperations: other</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*RetainedGovernance</code> | 实际返回表达式：<code>&amp;RetainedGovernance{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;RetainedGovernance{identity: identity, requests: requests, usecase: usecase, otherOperations: other}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=12021:12286 -->
````go
func NewRetainedGovernance(identity *Identity, requests *Transactions, usecase *logic.RetainedGovernance, other http.HandlerFunc) *RetainedGovernance {
	return &RetainedGovernance{identity: identity, requests: requests, usecase: usecase, otherOperations: other}
}

````

## 第 29 步：RetainedGovernance.Unpublish

插入位置：第 28 步代码之后，紧接追加；完成稿约第 394 行。

本步声明：`func (h *RetainedGovernance) Unpublish(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.mutate(w, r, &quot;UNPUBLISH&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.mutate(w, r, &quot;UNPUBLISH&quot;)</code> |

<!-- write-step: 29 bytes=12286:12399 -->
````go
func (h *RetainedGovernance) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "UNPUBLISH")
}

````

## 第 30 步：RetainedGovernance.Delete

插入位置：第 29 步代码之后，紧接追加；完成稿约第 398 行。

本步声明：`func (h *RetainedGovernance) Delete(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.mutate(w, r, &quot;PURGE&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.mutate(w, r, &quot;PURGE&quot;)</code> |

<!-- write-step: 30 bytes=12399:12505 -->
````go
func (h *RetainedGovernance) Delete(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "PURGE")
}

````

## 第 31 步：RetainedGovernance.Metadata

插入位置：第 30 步代码之后，紧接追加；完成稿约第 402 行。

本步声明：`func (h *RetainedGovernance) Metadata(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, err)</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Metadata(id))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, false, domain.RetainedGovernancePurpose, id)</code>；<code>r.Context</code>；<code>h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Metadata(id))</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=12505:13050 -->
````go
func (h *RetainedGovernance) Metadata(w http.ResponseWriter, r *http.Request) {
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err := h.identity.Actor(r, false, domain.RetainedGovernancePurpose, id)
	if err == nil {
		actor, err = h.usecase.Authorize(r.Context(), actor, platform.NewTraceID(), id)
	}
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Metadata(id))
}

````

## 第 32 步：RetainedGovernance.mutate

插入位置：第 31 步代码之后，紧接追加；完成稿约第 419 行。

本步声明：`func (h *RetainedGovernance) mutate(w http.ResponseWriter, r *http.Request, action string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>httpx.Failure(w, err)</code>；<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>httpx.Fail(w, 400, &quot;CONTROL_REVISION_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.DecodeBody(w, r, &amp;input)</code>；<code>pathvar.Vars(r)</code>；<code>h.identity.Actor(r, true, domain.RetainedGovernancePurpose, id)</code>；<code>r.Context</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.RetainedGovernanceKind(action)</code>；<code>h.usecase.Prepare(r.Context(), actor, id, action, input)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.DecodeBody(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidOperationID(input.OperationId)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>h.requests.Replay(w, r, actor, input.OperationId, kind, input, http.StatusAccepted, logic.ValidateOperationFirst, nil)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=13050:14473 -->
````go
func (h *RetainedGovernance) mutate(w http.ResponseWriter, r *http.Request, action string) {
	var input dto.VersionedAction
	if !httpx.DecodeBody(w, r, &input) {
		return
	}
	id, err := domain.CanonicalID(pathvar.Vars(r)["documentId"])
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	if !domain.ValidOperationID(input.OperationId) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	if _, err = domain.CanonicalRevision(input.ExpectedRevision); err != nil {
		httpx.Fail(w, 400, "CONTROL_REVISION_INVALID")
		return
	}
	actor, err := h.identity.Actor(r, true, domain.RetainedGovernancePurpose, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Authorize(r.Context(), actor, input.OperationId, id)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	kind := domain.RetainedGovernanceKind(action)
	// 原号在最终删除后仍可查询精确最小receipt，但每次HTTP都核验当前
	// 平台角色；普通活跃账号证明、旧组织管理员或注销恢复票都不能代替。
	if h.requests.Replay(w, r, actor, input.OperationId, kind, input, http.StatusAccepted, logic.ValidateOperationFirst, nil) {
		return
	}
	actor, run, present, err := h.usecase.Prepare(r.Context(), actor, id, action, input)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	h.requests.PresentedMutation(w, r, actor, input.OperationId, kind, input, http.StatusAccepted, run, present)
}

````

## 第 33 步：RetainedGovernance.Progress

插入位置：第 32 步代码之后，紧接追加；完成稿约第 461 行。

本步声明：`func (h *RetainedGovernance) Progress(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Fail(w, 400, &quot;OPERATION_ID_INVALID&quot;)</code>；<code>h.otherOperations(w, r)</code>；<code>httpx.Fail(w, 404, &quot;OPERATION_NOT_FOUND&quot;)</code>；<code>httpx.Failure(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>pathvar.Vars(r)</code>；<code>r.Context</code>；<code>h.otherOperations(w, r)</code>；<code>h.identity.Actor(r, false, domain.RetainedGovernancePurpose, row.Plan.Purge.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!domain.ValidOperationID(operation)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>errors.Is(err, sql.ErrNoRows) &amp;&amp; h.otherOperations != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=14473:15551 -->
````go
func (h *RetainedGovernance) Progress(w http.ResponseWriter, r *http.Request) {
	operation := pathvar.Vars(r)["operationId"]
	if !domain.ValidOperationID(operation) {
		httpx.Fail(w, 400, "OPERATION_ID_INVALID")
		return
	}
	row, err := h.usecase.Lookup(r.Context(), operation)
	if errors.Is(err, sql.ErrNoRows) && h.otherOperations != nil {
		h.otherOperations(w, r)
		return
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Fail(w, 404, "OPERATION_NOT_FOUND")
		} else {
			httpx.Failure(w, err)
		}
		return
	}
	actor, err := h.identity.Actor(r, false, domain.RetainedGovernancePurpose, row.Plan.Purge.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	actor, err = h.usecase.Authorize(r.Context(), actor, operation, row.Plan.Purge.Document)
	if err != nil {
		httpx.Failure(w, err)
		return
	}
	// 查询只输出原操作者的最小进度，不为已关闭Doc创建正文read admission。
	h.requests.Read(w, r, actor, actor.Proof.Sources, 0, domain.RetainedGovernancePurpose, h.usecase.Progress(actor, operation, row.Plan.Purge.Document))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/document/retirement_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/12-trash-transfer.md)。
