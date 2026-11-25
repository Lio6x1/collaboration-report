# 分步编写 `services/account/internal/handler/account/organization_handlers.go`

先按 [A03 · 组织、邀请申请、成员变更与组织关闭](../lessons/account/03-organizations.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/076-organization-handlers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把组织申请、邀请、成员目录、角色任免和组织关闭暴露为精确HTTP操作。

DTO、路径ID和分页归handler，管理资格和事务归logic；返回码区分即时成功与异步PREPARING。 新增current_member投影和只读q适配仍集中在既有handler，不新增第二套成员身份或目录接口。

写之前先按这条执行路线理解各段的连接：

1. Apply/Decide 校验理由长度和ACCEPT/REJECT枚举，再 Bind 原操作。
2. 邀请创建校验到期，成功后Share获取可复制链接；目录按账号/组织修订绑定cursor。
3. Role只接受MEMBER或ADMIN；Leave从真实session取本人ID；Confirmation仅开放document证书。
4. OrganizationDTO将CurrentMember投影为公开OrganizationMembership；组织详情可独立返回本人user_id/role/state/revision，前端不再翻完整成员目录寻找本人。
5. Members从URL读取q，trim/lower且按Unicode码点限制100字符；把q经QueryEscape与actor/organization/member_revision共同绑定签名cursor，再调用directory.List。

本文件需要守住的边界：

- 不能相信请求体自报离开者身份。
- 分页+1探测下一页后必须去掉多取项，不能让目录丢人或重复。
- 过滤条件变化时旧cursor必须拒绝；不能只在SQL中加q却漏掉游标绑定。
- 当前成员角色不从请求体或所选成员行提取；GET身份仍来自Authenticate，后续角色写入仍走原Mutation。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/handler/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/handler/account/organization_handlers.go') -and (Get-Item -LiteralPath 'services/account/internal/handler/account/organization_handlers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/handler/account/organization_handlers.go')) { New-Item -ItemType File -Path 'services/account/internal/handler/account/organization_handlers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:472 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：Applications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type Applications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。 |
| <code>logic</code> | <code>*logic.Applications</code> | 使用 <code>logic.Applications</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |
| <code>page</code> | <code>ParsePage</code> | 解析页大小和绑定当前查询范围的游标，返回(limit,after,ok)；ok=false时已写错误响应。 |
| <code>nextCursor</code> | <code>NextCursor</code> | 用查询范围和下一边界生成可验证游标；没有下一页时返回nil。 |

<!-- write-step: 2 bytes=472:657 -->
````go
type Applications struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.Applications
	page           ParsePage
	nextCursor     NextCursor
}

````

## 第 3 步：NewApplications

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func NewApplications(authentication *Authentication, requests *MutationRequests, service *logic.Applications, page ParsePage, cursor NextCursor) *Applications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>在此声明中的实际使用：<code>requests:       requests</code> |
| <code>service</code> | <code>*logic.Applications</code> | 使用 <code>logic.Applications</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic:          service</code> |
| <code>page</code> | <code>ParsePage</code> | 在此声明中的实际使用：<code>page:           page</code> |
| <code>cursor</code> | <code>NextCursor</code> | 在此声明中的实际使用：<code>nextCursor:     cursor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Applications</code> | 实际返回表达式：<code>&amp;Applications{ 		authentication: authentication, 		requests:       requests, 		logic:          service, 		page:           page, 		nextCursor:     cursor, 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Applications{ 		authentication: authentication, 		requests:       requests, 		logic:          service, 		page:           page, 		nextCursor:     cursor, 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=657:986 -->
````go
func NewApplications(authentication *Authentication, requests *MutationRequests, service *logic.Applications, page ParsePage, cursor NextCursor) *Applications {
	return &Applications{
		authentication: authentication,
		requests:       requests,
		logic:          service,
		page:           page,
		nextCursor:     cursor,
	}
}

````

## 第 4 步：Applications.Apply

插入位置：第 3 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func (h *Applications) Apply(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;APPLICATION_REASON_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>len(&#91;&#93;rune(input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=986:1521 -->
````go
func (h *Applications) Apply(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.ApplicationCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "APPLICATION_REASON_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Apply(r.Context(), mutation, organization, input)
	writeApplication(w, result, err)
}

````

## 第 5 步：Applications.AcceptInvitation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func (h *Applications) AcceptInvitation(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;INVITATION_INPUT_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>writeApplication(w, result, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(&#91;&#93;rune(input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=1521:2055 -->
````go
func (h *Applications) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input dto.InvitationAccept
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	hash, err := domain.TokenHash(input.InvitationToken)
	if err != nil || len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "INVITATION_INPUT_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.AcceptInvitation(r.Context(), mutation, hash, input)
	writeApplication(w, result, err)
}

````

## 第 6 步：Applications.Decide

插入位置：第 5 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func (h *Applications) Decide(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;applicationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;APPLICATION_DECISION_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;applicationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>(input.Decision != &quot;ACCEPT&quot; &amp;&amp; input.Decision != &quot;REJECT&quot;) &#124;&#124; len(&#91;&#93;rune(input.Reason)) &gt; 2000</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2055:2635 -->
````go
func (h *Applications) Decide(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "applicationId")
	if !ok {
		return
	}
	var input dto.ApplicationReview
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if (input.Decision != "ACCEPT" && input.Decision != "REJECT") || len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "APPLICATION_DECISION_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Decide(r.Context(), mutation, id, input)
	writeApplication(w, result, err)
}

````

## 第 7 步：writeApplication

插入位置：第 6 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func writeApplication(w http.ResponseWriter, result json.RawMessage, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 200, result)</code> |
| <code>result</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>platform.WriteJSON(w, 200, result)</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err != nil</code>；<code>httpx.WriteError(w, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2635:2812 -->
````go
func writeApplication(w http.ResponseWriter, result json.RawMessage, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

````

## 第 8 步：Applications.List

插入位置：第 7 步代码之后，紧接追加；完成稿约第 105 行。

本步声明：`func (h *Applications) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>h.page(w, r, binding)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>r.URL</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>r.URL.Path != &quot;/v1/account/applications&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>organization != &quot;&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>more</code>。

<!-- write-step: 8 bytes=2812:3902 -->
````go
func (h *Applications) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	organization := ""
	if r.URL.Path != "/v1/account/applications" {
		organization, ok = httpx.PathID(w, r, "organizationId")
		if !ok {
			return
		}
	}
	directory, err := h.logic.OpenDirectory(r.Context(), session.UserID, organization)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	binding := "applications:own:" + session.UserID
	if organization != "" {
		binding = "applications:" + session.UserID + ":" + organization + ":" + directory.Revision()
	}
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.ApplicationPage{Items: []dto.Application{}, HasMore: more}
	for _, application := range items {
		page.Items = append(page.Items, applicationDTO(application))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

````

## 第 9 步：applicationDTO

插入位置：第 8 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func applicationDTO(value domain.OrganizationApplication) dto.Application`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.OrganizationApplication</code> | 使用 <code>domain.OrganizationApplication</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>在此声明中的实际使用：<code>value.ID</code>；<code>value.OrganizationID</code>；<code>value.ApplicantID</code>；<code>value.Reason</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Application</code> | 实际返回表达式：<code>application</code> |

**跟着本段代码走一遍：**

- 检查 <code>value.InvitationValidatedAt != nil</code>。
- 此处通过 <code>return application</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3902:4541 -->
````go
func applicationDTO(value domain.OrganizationApplication) dto.Application {
	application := dto.Application{
		Id:                 value.ID,
		OrganizationId:     value.OrganizationID,
		ApplicantId:        value.ApplicantID,
		Reason:             value.Reason,
		State:              value.State,
		Revision:           value.Revision,
		SourceInvitationId: value.SourceInvitationID,
	}
	if value.InvitationValidatedAt != nil {
		formatted := value.InvitationValidatedAt.UTC().Format(time.RFC3339Nano)
		application.InvitationValidatedAt = &formatted
	}
	return application
}

// invitation.go：保留原实现，按业务合并文件。

````

## 第 10 步：Invitations

插入位置：第 9 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`type Invitations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。 |
| <code>logic</code> | <code>*logic.Invitations</code> | 使用 <code>logic.Invitations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |
| <code>origin</code> | <code>string</code> | 配置的用户站点源地址，用于核对浏览器请求来源/构建本源链接；不能信任任意Host或输入URL。 |
| <code>page</code> | <code>ParsePage</code> | 解析页大小和绑定当前查询范围的游标，返回(limit,after,ok)；ok=false时已写错误响应。 |
| <code>nextCursor</code> | <code>NextCursor</code> | 用查询范围和下一边界生成可验证游标；没有下一页时返回nil。 |

<!-- write-step: 10 bytes=4541:4747 -->
````go
type Invitations struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.Invitations
	origin         string
	page           ParsePage
	nextCursor     NextCursor
}

````

## 第 11 步：NewInvitations

插入位置：第 10 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func NewInvitations(authentication *Authentication, requests *MutationRequests, service *logic.Invitations, origin string, page ParsePage, cursor NextCursor) *Invitations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>在此声明中的实际使用：<code>requests:       requests</code> |
| <code>service</code> | <code>*logic.Invitations</code> | 使用 <code>logic.Invitations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic:          service</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin:         origin</code> |
| <code>page</code> | <code>ParsePage</code> | 在此声明中的实际使用：<code>page:           page</code> |
| <code>cursor</code> | <code>NextCursor</code> | 在此声明中的实际使用：<code>nextCursor:     cursor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Invitations</code> | 实际返回表达式：<code>&amp;Invitations{ 		authentication: authentication, 		requests:       requests, 		logic:          service, 		origin:         origin, 		page:           page, 		nextCursor:     cursor, 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Invitations{ 		authentication: authentication, 		requests:       requests, 		logic:          service, 		origin:         origin, 		page:           page, 		nextCursor:     cursor, 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4747:5113 -->
````go
func NewInvitations(authentication *Authentication, requests *MutationRequests, service *logic.Invitations, origin string, page ParsePage, cursor NextCursor) *Invitations {
	return &Invitations{
		authentication: authentication,
		requests:       requests,
		logic:          service,
		origin:         origin,
		page:           page,
		nextCursor:     cursor,
	}
}

````

## 第 12 步：Invitations.Create

插入位置：第 11 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func (h *Invitations) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;INVITATION_EXPIRY_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>input.ExpiresInHours &lt; 1 &#124;&#124; input.ExpiresInHours &gt; 720</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.Unmarshal(raw, &amp;operation) != nil &#124;&#124; operation.Result == nil &#124;&#124; operation.Result.EntityId == nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5113:6610 -->
````go
func (h *Invitations) Create(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.InvitationCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if input.ExpiresInHours < 1 || input.ExpiresInHours > 720 {
		httpx.Failure(w, 400, "INVITATION_EXPIRY_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	raw, err := h.logic.Create(r.Context(), mutation, organization, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var operation dto.Operation
	if json.Unmarshal(raw, &operation) != nil || operation.Result == nil || operation.Result.EntityId == nil {
		httpx.Failure(w, 503, "INVITATION_RESULT_UNAVAILABLE")
		return
	}
	// 通用receipt的成功不授予分享链接读取权；本次输出前重新核验会话及管理员来源。
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	share, err := h.logic.Share(r.Context(), session.UserID, organization, *operation.Result.EntityId)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, dto.InvitationCreated{
		OperationId: input.OperationId,
		ShareUrl:    h.origin + "/join?invitation_token=" + share.Token,
		Invitation: dto.Invitation{
			Id:             share.ID,
			OrganizationId: organization,
			State:          "OPEN",
			ExpiresAt:      share.Expires.UTC().Format(time.RFC3339),
			Revision:       "1",
		},
	})
}

````

## 第 13 步：Invitations.Cancel

插入位置：第 12 步代码之后，紧接追加；完成稿约第 234 行。

本步声明：`func (h *Invitations) Cancel(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;invitationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>httpx.WriteError(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;invitationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6610:7075 -->
````go
func (h *Invitations) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "invitationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Cancel(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

````

## 第 14 步：Invitations.List

插入位置：第 13 步代码之后，紧接追加；完成稿约第 255 行。

本步声明：`func (h *Invitations) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>h.page(w, r, binding)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>r.Context</code>；<code>h.page(w, r, binding)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>more</code>。

<!-- write-step: 14 bytes=7075:8008 -->
````go
func (h *Invitations) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	directory, err := h.logic.OpenDirectory(r.Context(), session.UserID, organization)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	binding := "invitations:" + session.UserID + ":" + organization + ":" + directory.Revision()
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.InvitationPage{Items: []dto.Invitation{}, HasMore: more}
	for _, invitation := range items {
		page.Items = append(page.Items, invitationDTO(invitation))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

````

## 第 15 步：invitationDTO

插入位置：第 14 步代码之后，紧接追加；完成稿约第 289 行。

本步声明：`func invitationDTO(value domain.OrganizationInvitation) dto.Invitation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.OrganizationInvitation</code> | 使用 <code>domain.OrganizationInvitation</code> 类型；其定义处的职责：定义组织、成员、申请、邀请及关闭固定计划，并实现邀请加密与组织关闭前置规则。<br>在此声明中的实际使用：<code>value.ID</code>；<code>value.OrganizationID</code>；<code>value.State</code>；<code>value.Expires</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Invitation</code> | 实际返回表达式：<code>dto.Invitation{ 		Id:             value.ID, 		OrganizationId: value.OrganizationID, 		State:          value.State, 		ExpiresAt:      value.Expires.UTC().Format(time.RFC3339), 		Revision:       value.Revision, 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return dto.Invitation{ 		Id:             value.ID, 		OrganizationId: value.OrganizationID, 		State:          value.State, 		ExpiresAt:      value.Expires.UTC().Format(time.RFC3339), 		Revision:       value.Revision, 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8008:8375 -->
````go
func invitationDTO(value domain.OrganizationInvitation) dto.Invitation {
	return dto.Invitation{
		Id:             value.ID,
		OrganizationId: value.OrganizationID,
		State:          value.State,
		ExpiresAt:      value.Expires.UTC().Format(time.RFC3339),
		Revision:       value.Revision,
	}
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 16 步：OrganizationClosures

插入位置：第 15 步代码之后，紧接追加；完成稿约第 301 行。

本步声明：`type OrganizationClosures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。 |
| <code>logic</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |

<!-- write-step: 16 bytes=8375:8479 -->
````go
type OrganizationClosures struct {
	requests *MutationRequests
	logic    *logic.OrganizationClosures
}

````

## 第 17 步：NewOrganizationClosures

插入位置：第 16 步代码之后，紧接追加；完成稿约第 306 行。

本步声明：`func NewOrganizationClosures(requests *MutationRequests, service *logic.OrganizationClosures) *OrganizationClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>service</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationClosures</code> | 实际返回表达式：<code>&amp;OrganizationClosures{requests: requests, logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationClosures{requests: requests, logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=8479:8665 -->
````go
func NewOrganizationClosures(requests *MutationRequests, service *logic.OrganizationClosures) *OrganizationClosures {
	return &OrganizationClosures{requests: requests, logic: service}
}
````

## 第 18 步：OrganizationClosures.Close

插入位置：第 17 步代码之后，紧接追加；完成稿约第 309 行。

本步声明：`func (h *OrganizationClosures) Close(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>writeTransition(w, result, err)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=8665:9116 -->
````go
func (h *OrganizationClosures) Close(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Begin(r.Context(), mutation, organization, input.ExpectedRevision)
	writeTransition(w, result, err)
}
````

## 第 19 步：OrganizationClosures.Confirmation

插入位置：第 18 步代码之后，紧接追加；完成稿约第 325 行。

本步声明：`func (h *OrganizationClosures) Confirmation(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.Failure(w, 403, &quot;CONTROL_IDENTITY_REQUIRED&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>httpx.WriteError(w, err)</code>；<code>w.Header</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>platform.HTTPCaller(r, &quot;document&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;binding)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.HTTPCaller(r, &quot;document&quot;)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;binding)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=9116:9733 -->
````go
func (h *OrganizationClosures) Confirmation(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.OrganizationClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	confirmed, err := h.logic.Confirmed(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.OrganizationClosureConfirmation{Confirmed: confirmed})
}

// organization_read.go：保留原实现，按业务合并文件。

````

## 第 20 步：ParsePage

插入位置：第 19 步代码之后，紧接追加；完成稿约第 345 行。

本步声明：`type ParsePage func(http.ResponseWriter, *http.Request, string) (int, string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。 |
| <code>(匿名/未命名)</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。 |
| <code>(匿名/未命名)</code> | <code>string</code> | 本页查询的固定范围绑定字符串，核验游标不得跨用户/组织复用。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 校验后的页大小。 |
| <code>(匿名/未命名)</code> | <code>string</code> | 解析出的上一页最后一项编号（after边界）。 |
| <code>(匿名/未命名)</code> | <code>bool</code> | 解析是否成功；false表示已经写错误响应，handler直接return。 |

<!-- write-step: 20 bytes=9733:9817 -->
````go
type ParsePage func(http.ResponseWriter, *http.Request, string) (int, string, bool)
````

## 第 21 步：NextCursor

插入位置：第 20 步代码之后，紧接追加；完成稿约第 346 行。

本步声明：`type NextCursor func(string, string) *string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 当前查询范围绑定字符串。 |
| <code>(匿名/未命名)</code> | <code>string</code> | 本页最后一项编号，作为下一页after边界。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*string</code> | 编码并签名的下一页游标指针，列表有更多项时才调用生成。 |

<!-- write-step: 21 bytes=9817:9863 -->
````go
type NextCursor func(string, string) *string

````

## 第 22 步：Organizations

插入位置：第 21 步代码之后，紧接追加；完成稿约第 348 行。

本步声明：`type Organizations struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>logic</code> | <code>*logic.Organizations</code> | 使用 <code>logic.Organizations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |
| <code>page</code> | <code>ParsePage</code> | 解析页大小和绑定当前查询范围的游标，返回(limit,after,ok)；ok=false时已写错误响应。 |
| <code>nextCursor</code> | <code>NextCursor</code> | 用查询范围和下一边界生成可验证游标；没有下一页时返回nil。 |

<!-- write-step: 22 bytes=9863:10016 -->
````go
type Organizations struct {
	authentication *Authentication
	logic          *logic.Organizations
	page           ParsePage
	nextCursor     NextCursor
}

````

## 第 23 步：NewOrganizations

插入位置：第 22 步代码之后，紧接追加；完成稿约第 355 行。

本步声明：`func NewOrganizations(authentication *Authentication, service *logic.Organizations, page ParsePage, cursor NextCursor) *Organizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>service</code> | <code>*logic.Organizations</code> | 使用 <code>logic.Organizations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic: service</code> |
| <code>page</code> | <code>ParsePage</code> | 在此声明中的实际使用：<code>page: page</code> |
| <code>cursor</code> | <code>NextCursor</code> | 在此声明中的实际使用：<code>nextCursor: cursor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Organizations</code> | 实际返回表达式：<code>&amp;Organizations{authentication: authentication, logic: service, page: page, nextCursor: cursor}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Organizations{authentication: authentication, logic: service, page: page, nextCursor: cursor}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=10016:10258 -->
````go
func NewOrganizations(authentication *Authentication, service *logic.Organizations, page ParsePage, cursor NextCursor) *Organizations {
	return &Organizations{authentication: authentication, logic: service, page: page, nextCursor: cursor}
}

````

## 第 24 步：OrganizationDTO

插入位置：第 23 步代码之后，紧接追加；完成稿约第 359 行。

本步声明：`func OrganizationDTO(value domain.Organization) dto.Organization`。

**这一段为什么这样写：** 把领域组织和仅在详情准入中填入的当前成员关系投影为生成契约；只公布本人身份所需字段。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>domain.Organization</code> | AccessOrganization返回的领域组织；CurrentMember=nil时不编造本人角色。<br>在此声明中的实际使用：<code>value.CurrentMember</code>；<code>value.ID</code>；<code>value.Name</code>；<code>value.OwnerID</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Organization</code> | 生成dto.Organization，current_member只含user_id、role、state、revision。<br>实际返回表达式：<code>dto.Organization{ 		CurrentMember:  current, 		Id:             value.ID, 		Name:           value.Name, 		OwnerId:        value.OwnerID, 		Lifecycle:      value.Lifecycle, 		Revisio …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>m != nil</code>。
- 此处通过 <code>return dto.Organization{ 		CurrentMember:  current, 		Id:             value.ID, 		Name:           value.Name, 		OwnerId:        value.OwnerID, 		Lifecycle:      value.Lifecycle, 		 …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=10258:10779 -->
````go
func OrganizationDTO(value domain.Organization) dto.Organization {
	var current *dto.OrganizationMembership
	if m := value.CurrentMember; m != nil {
		current = &dto.OrganizationMembership{UserId: m.UserID, Role: m.Role, State: m.State, Revision: m.Revision}
	}
	return dto.Organization{
		CurrentMember:  current,
		Id:             value.ID,
		Name:           value.Name,
		OwnerId:        value.OwnerID,
		Lifecycle:      value.Lifecycle,
		Revision:       value.Revision,
		MemberRevision: value.MemberRevision,
	}
}

````

## 第 25 步：Organizations.Get

插入位置：第 24 步代码之后，紧接追加；完成稿约第 375 行。

本步声明：`func (h *Organizations) Get(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 200, OrganizationDTO(org))</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=10779:11167 -->
````go
func (h *Organizations) Get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	org, err := h.logic.Get(r.Context(), session.UserID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, OrganizationDTO(org))
}

````

## 第 26 步：Organizations.List

插入位置：第 25 步代码之后，紧接追加；完成稿约第 392 行。

本步声明：`func (h *Organizations) List(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>h.page(w, r, binding)</code>；<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 200, page)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>h.page(w, r, binding)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>more</code>。

<!-- write-step: 26 bytes=11167:11842 -->
````go
func (h *Organizations) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	binding := "organizations:" + session.UserID
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := h.logic.List(r.Context(), session.UserID, last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.OrganizationPage{Items: []dto.Organization{}, HasMore: more}
	for _, item := range items {
		page.Items = append(page.Items, OrganizationDTO(item))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

````

## 第 27 步：Organizations.Members

插入位置：第 26 步代码之后，紧接追加；完成稿约第 417 行。

本步声明：`func (h *Organizations) Members(w http.ResponseWriter, r *http.Request)`。

**这一段为什么这样写：** 把搜索文字纳入目录的当前成员版本和签名游标范围，在服务端筛选后分页。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 输出MemberPage或明确的参数/权限错误。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.WriteError(w, err)</code>；<code>httpx.Failure(w, 400, &quot;DIRECTORY_QUERY_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 路径organizationId；URL limit/cursor/q；q为空不筛选，trim/lower后最多100个Unicode字符。<br>在此声明中的实际使用：<code>h.authentication.Authenticate(w, r, false)</code>；<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>r.Context</code>；<code>r.URL</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>len(&#91;&#93;rune(query)) &gt; 100</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>more</code>。

<!-- write-step: 27 bytes=11842:13255 -->
````go
func (h *Organizations) Members(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	directory, err := h.logic.OpenMembers(r.Context(), session.UserID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len([]rune(query)) > 100 {
		httpx.Failure(w, 400, "DIRECTORY_QUERY_INVALID")
		return
	}
	binding := "members:" + session.UserID + ":" + id + ":" + directory.Revision() + ":" + url.QueryEscape(query)
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit, query)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.MemberPage{Items: []dto.Member{}, HasMore: more}
	for _, item := range items {
		page.Items = append(page.Items, dto.Member{
			UserId:                item.UserID,
			DisplayName:           item.DisplayName,
			Role:                  item.Role,
			State:                 item.State,
			Revision:              item.Revision,
			KnowledgeBaseCreation: &item.KnowledgeBaseCreation,
		})
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].UserID)
	}
	platform.WriteJSON(w, 200, page)
}

// organization_transition.go：保留原实现，按业务合并文件。

````

## 第 28 步：OrganizationTransitions

插入位置：第 27 步代码之后，紧接追加；完成稿约第 465 行。

本步声明：`type OrganizationTransitions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。 |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。 |
| <code>logic</code> | <code>*logic.OrganizationTransitions</code> | 使用 <code>logic.OrganizationTransitions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |

<!-- write-step: 28 bytes=13255:13409 -->
````go
type OrganizationTransitions struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.OrganizationTransitions
}

````

## 第 29 步：NewOrganizationTransitions

插入位置：第 28 步代码之后，紧接追加；完成稿约第 471 行。

本步声明：`func NewOrganizationTransitions(authentication *Authentication, requests *MutationRequests, service *logic.OrganizationTransitions) *OrganizationTransitions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>authentication</code> | <code>*Authentication</code> | 使用 <code>Authentication</code> 类型；其定义处的职责：只适配HTTP输入输出，复用logic认证并记录响应来源。<br>在此声明中的实际使用：<code>authentication: authentication</code> |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>service</code> | <code>*logic.OrganizationTransitions</code> | 使用 <code>logic.OrganizationTransitions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationTransitions</code> | 实际返回表达式：<code>&amp;OrganizationTransitions{authentication: authentication, requests: requests, logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationTransitions{authentication: authentication, requests: requests, logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=13409:13672 -->
````go
func NewOrganizationTransitions(authentication *Authentication, requests *MutationRequests, service *logic.OrganizationTransitions) *OrganizationTransitions {
	return &OrganizationTransitions{authentication: authentication, requests: requests, logic: service}
}

````

## 第 30 步：OrganizationTransitions.Role

插入位置：第 29 步代码之后，紧接追加；完成稿约第 475 行。

本步声明：`func (h *OrganizationTransitions) Role(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.PathID(w, r, &quot;userId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;ROLE_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.PathID(w, r, &quot;userId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>input.Role != &quot;MEMBER&quot; &amp;&amp; input.Role != &quot;ADMIN&quot;</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 30 bytes=13672:14328 -->
````go
func (h *OrganizationTransitions) Role(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	user, ok := httpx.PathID(w, r, "userId")
	if !ok {
		return
	}
	var input dto.MemberRole
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if input.Role != "MEMBER" && input.Role != "ADMIN" {
		httpx.Failure(w, 400, "ROLE_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, user, input.ExpectedRevision, "ORG_ROLE", input.Role)
	writeTransition(w, result, err)
}

````

## 第 31 步：OrganizationTransitions.Remove

插入位置：第 30 步代码之后，紧接追加；完成稿约第 500 行。

本步声明：`func (h *OrganizationTransitions) Remove(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.PathID(w, r, &quot;userId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.PathID(w, r, &quot;userId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 31 bytes=14328:14879 -->
````go
func (h *OrganizationTransitions) Remove(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	user, ok := httpx.PathID(w, r, "userId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, user, input.ExpectedRevision, "ORG_REMOVE", "")
	writeTransition(w, result, err)
}

````

## 第 32 步：OrganizationTransitions.Leave

插入位置：第 31 步代码之后，紧接追加；完成稿约第 521 行。

本步声明：`func (h *OrganizationTransitions) Leave(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.authentication.Authenticate(w, r, true)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.authentication.Authenticate(w, r, true)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 32 bytes=14879:15558 -->
````go
func (h *OrganizationTransitions) Leave(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	// 目标只能取自真实会话，保持原先在通用操作号核验之前的身份检查顺序。
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, session.UserID, input.ExpectedRevision, "ORG_LEAVE", "")
	writeTransition(w, result, err)
}

````

## 第 33 步：OrganizationTransitions.TransferOwner

插入位置：第 32 步代码之后，紧接追加；完成稿约第 543 行。

本步声明：`func (h *OrganizationTransitions) TransferOwner(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;ENTITY_ID_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEntityID(input.TargetUserId)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。

<!-- write-step: 33 bytes=15558:16113 -->
````go
func (h *OrganizationTransitions) TransferOwner(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.OwnerChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidEntityID(input.TargetUserId) {
		httpx.Failure(w, 400, "ENTITY_ID_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.TransferOwner(r.Context(), mutation, organization, input)
	writeTransition(w, result, err)
}

````

## 第 34 步：writeTransition

插入位置：第 33 步代码之后，紧接追加；完成稿约第 564 行。

本步声明：`func writeTransition(w http.ResponseWriter, result json.RawMessage, err error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.WriteError(w, err)</code>；<code>platform.WriteJSON(w, 202, result)</code> |
| <code>result</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>在此声明中的实际使用：<code>platform.WriteJSON(w, 202, result)</code> |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err != nil</code>；<code>httpx.WriteError(w, err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=16113:16360 -->
````go
func writeTransition(w http.ResponseWriter, result json.RawMessage, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 202, result)
}

// organization_write.go：保留原实现，按业务合并文件。

````

## 第 35 步：OrganizationActions

插入位置：第 34 步代码之后，紧接追加；完成稿约第 574 行。

本步声明：`type OrganizationActions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。 |
| <code>logic</code> | <code>*logic.OrganizationActions</code> | 使用 <code>logic.OrganizationActions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。 |

<!-- write-step: 35 bytes=16360:16462 -->
````go
type OrganizationActions struct {
	requests *MutationRequests
	logic    *logic.OrganizationActions
}

````

## 第 36 步：NewOrganizationActions

插入位置：第 35 步代码之后，紧接追加；完成稿约第 579 行。

本步声明：`func NewOrganizationActions(requests *MutationRequests, service *logic.OrganizationActions) *OrganizationActions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>requests</code> | <code>*MutationRequests</code> | 使用 <code>MutationRequests</code> 类型；其定义处的职责：MutationRequests绑定真实会话和原请求指纹；事务与业务动作留给logic。<br>在此声明中的实际使用：<code>requests: requests</code> |
| <code>service</code> | <code>*logic.OrganizationActions</code> | 使用 <code>logic.OrganizationActions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic: service</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationActions</code> | 实际返回表达式：<code>&amp;OrganizationActions{requests: requests, logic: service}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationActions{requests: requests, logic: service}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 36 bytes=16462:16645 -->
````go
func NewOrganizationActions(requests *MutationRequests, service *logic.OrganizationActions) *OrganizationActions {
	return &OrganizationActions{requests: requests, logic: service}
}

````

## 第 37 步：OrganizationActions.Create

插入位置：第 36 步代码之后，紧接追加；完成稿约第 583 行。

本步声明：`func (h *OrganizationActions) Create(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;ORGANIZATION_NAME_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>httpx.WriteError(w, err)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=16645:17200 -->
````go
func (h *OrganizationActions) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.NamedCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	name, valid := domain.NormalizeOrganizationName(input.Name)
	if !valid {
		httpx.Failure(w, 400, "ORGANIZATION_NAME_INVALID")
		return
	}
	input.Name = name
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Create(r.Context(), mutation, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 201, result)
}

````

## 第 38 步：OrganizationActions.Edit

插入位置：第 37 步代码之后，紧接追加；完成稿约第 606 行。

本步声明：`func (h *OrganizationActions) Edit(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;ORGANIZATION_NAME_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 38 bytes=17200:17830 -->
````go
func (h *OrganizationActions) Edit(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.OrganizationEdit
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	name, valid := domain.NormalizeOrganizationName(input.Name)
	if !valid {
		httpx.Failure(w, 400, "ORGANIZATION_NAME_INVALID")
		return
	}
	input.Name = name
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Edit(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

````

## 第 39 步：OrganizationActions.ChangeCreationGrant

插入位置：第 38 步代码之后，紧接追加；完成稿约第 633 行。

本步声明：`func (h *OrganizationActions) ChangeCreationGrant(w http.ResponseWriter, r *http.Request)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>httpx.Failure(w, 400, &quot;ENTITY_ID_INVALID&quot;)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>httpx.PathID(w, r, &quot;organizationId&quot;)</code>；<code>httpx.ReadJSON(w, r, &amp;input)</code>；<code>h.requests.Bind(w, r, input.OperationId, input)</code>；<code>r.Context</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!httpx.ReadJSON(w, r, &amp;input)</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidEntityID(input.UserId)</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=17830:18427 -->
````go
func (h *OrganizationActions) ChangeCreationGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.CreationGrant
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidEntityID(input.UserId) {
		httpx.Failure(w, 400, "ENTITY_ID_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeCreationGrant(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/handler/account/organization_handlers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/03-organizations.md)。
