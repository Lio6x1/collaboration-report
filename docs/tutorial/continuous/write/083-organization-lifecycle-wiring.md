# 分步编写 `services/account/internal/svc/organization_lifecycle_wiring.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/083-organization-lifecycle-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

将组织、邀请申请、退出与关闭用例接到同一个Store及Document客户端。

构造层决定依赖，不让handler或logic悄悄另开库和HTTP客户端。

写之前先按这条执行路线理解各段的连接：

1. NewLogout注入Authentication与同池receipt/mutation/closure仓库。
2. NewInvitations注入加密key，NewOrganizationTransitions注入Departure客户端，NewOrganizationClosures注入Fence和独立关闭控制接口。

本文件需要守住的边界：

- capture必须沿请求原context使用；加密key必须稳定，否则原邀请密文无法续取。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/svc/organization_lifecycle_wiring.go') -and (Get-Item -LiteralPath 'services/account/internal/svc/organization_lifecycle_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/svc/organization_lifecycle_wiring.go')) { New-Item -ItemType File -Path 'services/account/internal/svc/organization_lifecycle_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:492 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"collaboration/account/internal/client"
	domain "collaboration/account/internal/domain/account"
	logic "collaboration/account/internal/logic/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"database/sql"
	"net/http"
)

// closure.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewClosure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func NewClosure(store *platform.Store, rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *logic.Closure`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>rpc</code> | <code>pb.DocumentInternalClient</code> | 在此声明中的实际使用：<code>client.NewDocument(rpc, httpClient, endpoint)</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewDocument(rpc, httpClient, endpoint)</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>client.NewDocument(rpc, httpClient, endpoint)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Closure</code> | 使用 <code>logic.Closure</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。<br>实际返回表达式：<code>logic.NewClosure(store.DB, repository.NewClosure(store.DB), peer)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewClosure(store.DB, repository.NewClosure(store.DB), peer)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=492:812 -->
````go
func NewClosure(store *platform.Store, rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *logic.Closure {
	peer := client.NewDocument(rpc, httpClient, endpoint)
	return logic.NewClosure(store.DB, repository.NewClosure(store.DB), peer)
}

// logout.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewLogout

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func NewLogout(store *platform.Store, authentication *logic.Authentication) *logic.Logout`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>authentication</code> | <code>*logic.Authentication</code> | 使用 <code>logic.Authentication</code> 类型；其定义处的职责：把SQL事务、只读/锁定查询和限流能力组合成业务用例，HTTP Cookie留给handler。<br>在此声明中的实际使用：<code>logic.NewLogout( 		store.DB, 		authentication, 		repository.NewOperationReceipts(store.DB), 		repository.NewMutation(), 		repository.NewClosure(store.DB), 	)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Logout</code> | 使用 <code>logic.Logout</code> 类型；其定义处的职责：编排退出、自然到期与来源关闭恢复，接收Doc确认后执行本地收尾。<br>实际返回表达式：<code>logic.NewLogout( 		store.DB, 		authentication, 		repository.NewOperationReceipts(store.DB), 		repository.NewMutation(), 		repository.NewClosure(store.DB), 	)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewLogout( 		store.DB, 		authentication, 		repository.NewOperationReceipts(store.DB), 		repository.NewMutation(), 		repository.NewClosure(store.DB), 	)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=812:1136 -->
````go
func NewLogout(store *platform.Store, authentication *logic.Authentication) *logic.Logout {
	return logic.NewLogout(
		store.DB,
		authentication,
		repository.NewOperationReceipts(store.DB),
		repository.NewMutation(),
		repository.NewClosure(store.DB),
	)
}

// membership.go：保留原实现，按业务合并文件。

````

## 第 4 步：NewInvitations

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func NewInvitations(store *platform.Store, mutations *logic.Mutations, key []byte, capture logic.CaptureSource) *logic.Invitations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>mutations</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)</code> |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>domain.NewInvitationCodec(key)</code> |
| <code>capture</code> | <code>logic.CaptureSource</code> | 使用 <code>logic.CaptureSource</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Invitations</code> | 使用 <code>logic.Invitations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1136:1400 -->
````go
func NewInvitations(store *platform.Store, mutations *logic.Mutations, key []byte, capture logic.CaptureSource) *logic.Invitations {
	return logic.NewInvitations(store.DB, mutations, repository.NewInvitations(store.DB), domain.NewInvitationCodec(key), capture)
}

````

## 第 5 步：NewApplications

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`func NewApplications(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.Applications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>mutations</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic.NewApplications( 		store.DB, 		mutations, 		repository.NewApplications(store.DB), 		repository.NewOrganizations(store.DB), 		repository.NewInvitations(store.DB), 		capture, 	)</code> |
| <code>capture</code> | <code>logic.CaptureSource</code> | 使用 <code>logic.CaptureSource</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic.NewApplications( 		store.DB, 		mutations, 		repository.NewApplications(store.DB), 		repository.NewOrganizations(store.DB), 		repository.NewInvitations(store.DB), 		capture, 	)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Applications</code> | 使用 <code>logic.Applications</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewApplications( 		store.DB, 		mutations, 		repository.NewApplications(store.DB), 		repository.NewOrganizations(store.DB), 		repository.NewInvitations(store.DB), 		capture, 	)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewApplications( 		store.DB, 		mutations, 		repository.NewApplications(store.DB), 		repository.NewOrganizations(store.DB), 		repository.NewInvitations(store.DB), 		capture, 	)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1400:1781 -->
````go
func NewApplications(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.Applications {
	return logic.NewApplications(
		store.DB,
		mutations,
		repository.NewApplications(store.DB),
		repository.NewOrganizations(store.DB),
		repository.NewInvitations(store.DB),
		capture,
	)
}

// organization.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewOrganizations

插入位置：第 5 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func NewOrganizations(store *platform.Store, capture logic.CaptureSource) *logic.Organizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>capture</code> | <code>logic.CaptureSource</code> | 使用 <code>logic.CaptureSource</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic.NewOrganizations(store.DB, repository.NewOrganizations(store.DB), capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Organizations</code> | 使用 <code>logic.Organizations</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewOrganizations(store.DB, repository.NewOrganizations(store.DB), capture)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewOrganizations(store.DB, repository.NewOrganizations(store.DB), capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1781:1970 -->
````go
func NewOrganizations(store *platform.Store, capture logic.CaptureSource) *logic.Organizations {
	return logic.NewOrganizations(store.DB, repository.NewOrganizations(store.DB), capture)
}

````

## 第 7 步：NewOrganizationTransitions

插入位置：第 6 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func NewOrganizationTransitions(store *platform.Store, mutations *logic.Mutations, httpClient *http.Client, endpoint string, capture logic.CaptureSource) *logic.OrganizationTransitions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>mutations</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewDeparture(httpClient, endpoint)</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>client.NewDeparture(httpClient, endpoint)</code> |
| <code>capture</code> | <code>logic.CaptureSource</code> | 使用 <code>logic.CaptureSource</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.OrganizationTransitions</code> | 使用 <code>logic.OrganizationTransitions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1970:2303 -->
````go
func NewOrganizationTransitions(store *platform.Store, mutations *logic.Mutations, httpClient *http.Client, endpoint string, capture logic.CaptureSource) *logic.OrganizationTransitions {
	return logic.NewOrganizationTransitions(mutations, repository.NewOrganizations(store.DB), client.NewDeparture(httpClient, endpoint), capture)
}

````

## 第 8 步：NewOrganizationActions

插入位置：第 7 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func NewOrganizationActions(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.OrganizationActions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>mutations</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)</code> |
| <code>capture</code> | <code>logic.CaptureSource</code> | 使用 <code>logic.CaptureSource</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>在此声明中的实际使用：<code>logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.OrganizationActions</code> | 使用 <code>logic.OrganizationActions</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2303:2610 -->
````go
func NewOrganizationActions(store *platform.Store, mutations *logic.Mutations, capture logic.CaptureSource) *logic.OrganizationActions {
	return logic.NewOrganizationActions(mutations, repository.NewOrganizations(store.DB), capture)
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 9 步：NewOrganizationClosures

插入位置：第 8 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func NewOrganizationClosures(db *sql.DB, mutations *logic.Mutations, fence logic.AccountCloseFencePeer, httpClient *http.Client, documentEndpoint string) *logic.OrganizationClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))</code> |
| <code>mutations</code> | <code>*logic.Mutations</code> | 使用 <code>logic.Mutations</code> 类型；其定义处的职责：签发并重验30秒身份准入，恢复已登记操作准入，统一账号幂等事务与签名游标。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))</code> |
| <code>fence</code> | <code>logic.AccountCloseFencePeer</code> | 使用 <code>logic.AccountCloseFencePeer</code> 类型；其定义处的职责：执行注销密码确认、职责交接检查、下游范围固定和来源冻结。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewOrganizationCloseControl(httpClient, documentEndpoint)</code> |
| <code>documentEndpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>client.NewOrganizationCloseControl(httpClient, documentEndpoint)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：编排申请审核、邀请分享、组织读写、成员离开和所有者转移以及独立组织关闭。<br>实际返回表达式：<code>logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2610:2922 -->
````go
func NewOrganizationClosures(db *sql.DB, mutations *logic.Mutations, fence logic.AccountCloseFencePeer, httpClient *http.Client, documentEndpoint string) *logic.OrganizationClosures {
	return logic.NewOrganizationClosures(db, mutations, fence, client.NewOrganizationCloseControl(httpClient, documentEndpoint))
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/svc/organization_lifecycle_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
