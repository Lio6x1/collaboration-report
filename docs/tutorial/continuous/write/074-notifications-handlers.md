# 分步编写 `services/account/internal/handler/account/notifications_handlers.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/074-notifications-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将通知候选、收件核验、事件拉取/确认和离开状态开放给真实Document服务。

内部队列含成员身份，服务证书是唯一入口身份，普通浏览器Cookie不授予权限。

写之前先按这条执行路线理解各段的连接：

1. 每个内部handler先HTTPCaller(document)，严格限8KiB单份JSON。
2. notificationEventInput排除null和未知字段；Events读取空对象，Acknowledge读取ID集合。
3. 领域非法输入映射400，存储未知映射503，并设置no-store。

本文件需要守住的边界：

- 不能把HTTP 200当作已确认除非业务Acknowledge已提交；不得将内部端点开放为公共成员枚举。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/notifications_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/notifications_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/notifications_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/notifications_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:433 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// departure_status.go：保留原实现，按业务合并文件。

````

## 第 2 步：Organizations.DepartureStatus

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func (h *Organizations) DepartureStatus(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.Failure(w, 400, &quot;DEPARTURE_BINDING_INVALID&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 200, map&#91;string&#93;string{&quot;state&quot;: state, &quot;operation_id&quot;: operation})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.URL</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!domain.ValidEntityID(organization) &#124;&#124; !domain.ValidEntityID(user) &#124;&#124; !domain.ValidOperationID(operation)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=433:1267 -->
````go
func (h *Organizations) DepartureStatus(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	query := r.URL.Query()
	organization, user, operation := query.Get("organization_id"), query.Get("user_id"), query.Get("operation_id")
	if !domain.ValidEntityID(organization) || !domain.ValidEntityID(user) || !domain.ValidOperationID(operation) {
		httpx.Failure(w, 400, "DEPARTURE_BINDING_INVALID")
		return
	}
	state, err := h.logic.DepartureStatus(r.Context(), organization, user, operation, query.Get("epoch"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": state, "operation_id": operation})
}

// notification_administrators.go：保留原实现，按业务合并文件。

````

## 第 3 步：NotificationRecipients.Administrators

插入位置：第 2 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func (h *NotificationRecipients) Administrators(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>http.MaxBytesReader(w, r.Body, 8192)</code>；<code>httpx.Failure(w, 400, &quot;NOTIFICATION_ADMINISTRATORS_INVALID&quot;)</code>；<code>httpx.Failure(w, 503, &quot;NOTIFICATION_ADMINISTRATORS_UNAVAILABLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;input) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, domain.ErrNotificationAdministratorsInput)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1267:2328 -->
````go
func (h *NotificationRecipients) Administrators(w http.ResponseWriter, r *http.Request) {
	// 仅实际Document服务证书可枚举候选人，浏览器Cookie和自报身份不能进入目录。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.NotificationAdministratorsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_ADMINISTRATORS_INVALID")
		return
	}
	result, err := h.logic.Administrators(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationAdministratorsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_ADMINISTRATORS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_ADMINISTRATORS_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

// notification_events.go：保留原实现，按业务合并文件。

````

## 第 4 步：NotificationEvents

插入位置：第 3 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`type NotificationEvents struct{ logic *logic.NotificationEvents }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.NotificationEvents</code> | 使用 <code>logic.NotificationEvents</code> 类型；其定义处的职责：编排通知队列读取及整批原子确认。 |

<!-- write-step: 4 bytes=2328:2395 -->
````go
type NotificationEvents struct{ logic *logic.NotificationEvents }

````

## 第 5 步：NewNotificationEvents

插入位置：第 4 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func NewNotificationEvents(service *logic.NotificationEvents) *NotificationEvents`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.NotificationEvents</code> | 使用 <code>logic.NotificationEvents</code> 类型；其定义处的职责：编排通知队列读取及整批原子确认。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationEvents</code> | 实际返回表达式：<code>&amp;NotificationEvents{logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationEvents{logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2395:2526 -->
````go
func NewNotificationEvents(service *logic.NotificationEvents) *NotificationEvents {
	return &NotificationEvents{logic: service}
}

````

## 第 6 步：notificationEventInput

插入位置：第 5 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func notificationEventInput(w http.ResponseWriter, r *http.Request, input any) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>http.MaxBytesReader(w, r.Body, 8192)</code>；<code>httpx.Failure(w, 400, &quot;NOTIFICATION_EVENTS_INVALID&quot;)</code>；<code>w.Header</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.Body</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>strict.Decode(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>decoder.Decode(&amp;raw) != nil &#124;&#124; len(raw) == 0 &#124;&#124; raw&#91;0&#93; != &#x27;{&#x27; &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>strict.Decode(input) != nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2526:3485 -->
````go
func notificationEventInput(w http.ResponseWriter, r *http.Request, input any) bool {
	// 证书角色是唯一身份来源；即便携带有效用户Cookie也不能访问内部投递队列。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if decoder.Decode(&raw) != nil || len(raw) == 0 || raw[0] != '{' || decoder.Decode(new(any)) != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		return false
	}
	// 第二次严格解码保留字段检查，同时排除JSON null当作空对象的隐式接受。
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if strict.Decode(input) != nil {
		httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}

````

## 第 7 步：NotificationEvents.Events

插入位置：第 6 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func (h *NotificationEvents) Events(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>notificationEventInput(w, r, &amp;struct{}{})</code>；<code>httpx.Failure(w, 503, &quot;NOTIFICATION_EVENTS_UNAVAILABLE&quot;)</code>；<code>platform.WriteJSON(w, 200, result)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>notificationEventInput(w, r, &amp;struct{}{})</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!notificationEventInput(w, r, &amp;struct{}{})</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3485:3795 -->
````go
func (h *NotificationEvents) Events(w http.ResponseWriter, r *http.Request) {
	if !notificationEventInput(w, r, &struct{}{}) {
		return
	}
	result, err := h.logic.Events(r.Context())
	if err != nil {
		httpx.Failure(w, 503, "NOTIFICATION_EVENTS_UNAVAILABLE")
		return
	}
	platform.WriteJSON(w, 200, result)
}

````

## 第 8 步：NotificationEvents.Acknowledge

插入位置：第 7 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func (h *NotificationEvents) Acknowledge(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>notificationEventInput(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;NOTIFICATION_EVENTS_INVALID&quot;)</code>；<code>httpx.Failure(w, 503, &quot;NOTIFICATION_EVENTS_UNAVAILABLE&quot;)</code>；<code>platform.WriteJSON(w, 200, platform.AccountNotificationAckResponse{Acknowledged: true})</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>notificationEventInput(w, r, &amp;input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!notificationEventInput(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, logic.ErrNotificationEventsInput)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3795:4416 -->
````go
func (h *NotificationEvents) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var input platform.AccountNotificationAckRequest
	if !notificationEventInput(w, r, &input) {
		return
	}
	if err := h.logic.Acknowledge(r.Context(), input); err != nil {
		if errors.Is(err, logic.ErrNotificationEventsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_EVENTS_UNAVAILABLE")
		}
		return
	}
	platform.WriteJSON(w, 200, platform.AccountNotificationAckResponse{Acknowledged: true})
}

// notification_recipients.go：保留原实现，按业务合并文件。

````

## 第 9 步：NotificationRecipients

插入位置：第 8 步代码之后，紧接追加；完成稿约第 127 行。

本步声明：`type NotificationRecipients struct{ logic *logic.NotificationRecipients }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.NotificationRecipients</code> | 使用 <code>logic.NotificationRecipients</code> 类型；其定义处的职责：判断通知收件资格并返回5秒有效的来源快照，解释离开原事务的实际状态。 |

<!-- write-step: 9 bytes=4416:4491 -->
````go
type NotificationRecipients struct{ logic *logic.NotificationRecipients }

````

## 第 10 步：NewNotificationRecipients

插入位置：第 9 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func NewNotificationRecipients(service *logic.NotificationRecipients) *NotificationRecipients`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>service</code> | <code>*logic.NotificationRecipients</code> | 使用 <code>logic.NotificationRecipients</code> 类型；其定义处的职责：判断通知收件资格并返回5秒有效的来源快照，解释离开原事务的实际状态。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationRecipients</code> | 实际返回表达式：<code>&amp;NotificationRecipients{logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationRecipients{logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4491:4638 -->
````go
func NewNotificationRecipients(service *logic.NotificationRecipients) *NotificationRecipients {
	return &NotificationRecipients{logic: service}
}

````

## 第 11 步：NotificationRecipients.Check

插入位置：第 10 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`func (h *NotificationRecipients) Check(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>http.MaxBytesReader(w, r.Body, 8192)</code>；<code>httpx.Failure(w, 400, &quot;NOTIFICATION_RECIPIENTS_INVALID&quot;)</code>；<code>httpx.Failure(w, 503, &quot;NOTIFICATION_RECIPIENTS_UNAVAILABLE&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>r.Body</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != io.EOF</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.Is(err, domain.ErrNotificationRecipientsInput)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4638:5682 -->
````go
func (h *NotificationRecipients) Check(w http.ResponseWriter, r *http.Request) {
	// 只接受实际document mTLS证书；不使用Cookie，不向浏览器开放后台成员枚举。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.NotificationRecipientsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		return
	}
	result, err := h.logic.Check(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationRecipientsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_RECIPIENTS_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/notifications_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
