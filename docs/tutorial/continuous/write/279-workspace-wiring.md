# 分步编写 `services/document/internal/svc/workspace_wiring.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/279-workspace-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

组合工作区、身份、ACL、事务、目录、邀请和分页基础用例。

把构造集中让handler只关心业务接口，且所有数据库操作共享同一Document私有Store。

写之前先按这条执行路线理解各段的连接：

1. NewIdentity接Account RPC与会话HTTP
2. NewAccess接ACL仓储
3. NewTransactions注入输出
4. NewSharing接加密令牌与AccountDirectory姓名解析器
5. NewReadings接可靠MAIN
6. NewPagination接独立签名器。

本文件需要守住的边界：

- 邀请密钥与游标签名用途不能混同
- 构造函数不应隐式提交业务事务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/svc/workspace_wiring.go') -and (Get-Item -LiteralPath 'services/document/internal/svc/workspace_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/svc/workspace_wiring.go')) { New-Item -ItemType File -Path 'services/document/internal/svc/workspace_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:488 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	logic "collaboration/document/internal/logic/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"database/sql"
	"net/http"
)

// authorization.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewAuthorizations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`func NewAuthorizations(store *platform.Store, fences *logic.Fences, outputs *logic.Outputs) *logic.Authorizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>fences</code> | <code>*logic.Fences</code> | 使用 <code>logic.Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Authorizations</code> | 使用 <code>logic.Authorizations</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=488:779 -->
````go
func NewAuthorizations(store *platform.Store, fences *logic.Fences, outputs *logic.Outputs) *logic.Authorizations {
	return logic.NewAuthorizations(store.DB, repository.NewAuthorizations(store.DB), fences, outputs)
}

// authorization_change.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewAuthorizationChanges

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewAuthorizationChanges(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.AuthorizationChanges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewAuthorizationChanges(store.DB, identity, access)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewAuthorizationChanges(store.DB, identity, access)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.AuthorizationChanges</code> | 使用 <code>logic.AuthorizationChanges</code> 类型；其定义处的职责：先确认操作者可管理文档并绑定受影响身份，再准备授权变更的事务执行体；权限撤销和来源关闭需沿用同一操作的恢复边界。<br>实际返回表达式：<code>logic.NewAuthorizationChanges(store.DB, identity, access)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewAuthorizationChanges(store.DB, identity, access)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=779:1047 -->
````go
func NewAuthorizationChanges(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.AuthorizationChanges {
	return logic.NewAuthorizationChanges(store.DB, identity, access)
}

// content_versions.go：保留原实现，按业务合并文件。

````

## 第 4 步：NewContentVersions

插入位置：第 3 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`func NewContentVersions(store *platform.Store, access *logic.Access) (*logic.ContentVersions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewContentVersions(store.DB, access, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.ContentVersions</code> | 使用 <code>logic.ContentVersions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>实际返回表达式：<code>nil</code>；<code>logic.NewContentVersions(store.DB, access, validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewContentVersions(store.DB, access, validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1047:1360 -->
````go
func NewContentVersions(store *platform.Store, access *logic.Access) (*logic.ContentVersions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewContentVersions(store.DB, access, validator), nil
}

// departure.go：保留原实现，按业务合并文件。

````

## 第 5 步：NewDepartures

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewDepartures(store *platform.Store, cfg platform.Config, accountHTTP *http.Client) *logic.Departures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Peers</code> |
| <code>accountHTTP</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewAccountDeparture(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Departures</code> | 使用 <code>logic.Departures</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>logic.NewDepartures(store.DB, repository.NewDepartures(store.DB), client.NewAccountDeparture(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewDepartures(store.DB, repository.NewDepartures(store.DB), client.NewAccountDeparture(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1360:1674 -->
````go
func NewDepartures(store *platform.Store, cfg platform.Config, accountHTTP *http.Client) *logic.Departures {
	return logic.NewDepartures(store.DB, repository.NewDepartures(store.DB), client.NewAccountDeparture(cfg.Peers["accountHttp"], accountHTTP))
}

// document.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewDocuments

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func NewDocuments(store *platform.Store, identity *logic.Identity, access *logic.Access, quotaLimit uint64) *logic.Documents`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewDocuments(store.DB, identity, access, quotaLimit)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewDocuments(store.DB, identity, access, quotaLimit)</code> |
| <code>quotaLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>logic.NewDocuments(store.DB, identity, access, quotaLimit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Documents</code> | 使用 <code>logic.Documents</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>实际返回表达式：<code>logic.NewDocuments(store.DB, identity, access, quotaLimit)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewDocuments(store.DB, identity, access, quotaLimit)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1674:1937 -->
````go
func NewDocuments(store *platform.Store, identity *logic.Identity, access *logic.Access, quotaLimit uint64) *logic.Documents {
	return logic.NewDocuments(store.DB, identity, access, quotaLimit)
}

// edit_sessions.go：保留原实现，按业务合并文件。

````

## 第 7 步：NewEditSessions

插入位置：第 6 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func NewEditSessions(store *platform.Store, cfg platform.Config, access *logic.Access, tokens logic.InvitationCipher) *logic.EditSessions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Origin</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)</code> |
| <code>tokens</code> | <code>logic.InvitationCipher</code> | 使用 <code>logic.InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。<br>在此声明中的实际使用：<code>logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.EditSessions</code> | 使用 <code>logic.EditSessions</code> 类型；其定义处的职责：发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。<br>实际返回表达式：<code>logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1937:2209 -->
````go
func NewEditSessions(store *platform.Store, cfg platform.Config, access *logic.Access, tokens logic.InvitationCipher) *logic.EditSessions {
	return logic.NewEditSessions(store.DB, access, tokens, cfg.Origin)
}

// identity.go：保留原实现，按业务合并文件。

````

## 第 8 步：NewIdentity

插入位置：第 7 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func NewIdentity(peer pb.IdentityInternalClient, base string, httpClient *http.Client) *logic.Identity`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peer</code> | <code>pb.IdentityInternalClient</code> | 在此声明中的实际使用：<code>logic.NewIdentity(peer, account, account)</code> |
| <code>base</code> | <code>string</code> | 在此声明中的实际使用：<code>client.NewAccountSession(base, httpClient)</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewAccountSession(base, httpClient)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>实际返回表达式：<code>logic.NewIdentity(peer, account, account)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewIdentity(peer, account, account)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2209:2421 -->
````go
func NewIdentity(peer pb.IdentityInternalClient, base string, httpClient *http.Client) *logic.Identity {
	account := client.NewAccountSession(base, httpClient)
	return logic.NewIdentity(peer, account, account)
}
````

## 第 9 步：NewAccess

插入位置：第 8 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func NewAccess(db *sql.DB, identity *logic.Identity) *logic.Access`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>repository.NewAccess(db)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewAccess(repository.NewAccess(db), identity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>实际返回表达式：<code>logic.NewAccess(repository.NewAccess(db), identity)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewAccess(repository.NewAccess(db), identity)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2421:2620 -->
````go
func NewAccess(db *sql.DB, identity *logic.Identity) *logic.Access {
	return logic.NewAccess(repository.NewAccess(db), identity)
}

// knowledge_base.go：保留原实现，按业务合并文件。

````

## 第 10 步：NewKnowledgeBases

插入位置：第 9 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func NewKnowledgeBases(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.KnowledgeBases`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewKnowledgeBases(store.DB, identity, access)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewKnowledgeBases(store.DB, identity, access)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.KnowledgeBases</code> | 使用 <code>logic.KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>实际返回表达式：<code>logic.NewKnowledgeBases(store.DB, identity, access)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewKnowledgeBases(store.DB, identity, access)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2620:2864 -->
````go
func NewKnowledgeBases(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.KnowledgeBases {
	return logic.NewKnowledgeBases(store.DB, identity, access)
}

// management.go：保留原实现，按业务合并文件。

````

## 第 11 步：NewManagement

插入位置：第 10 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func NewManagement(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.Management`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewManagement(store.DB, identity, access)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewManagement(store.DB, identity, access)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Management</code> | 使用 <code>logic.Management</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>实际返回表达式：<code>logic.NewManagement(store.DB, identity, access)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewManagement(store.DB, identity, access)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2864:3094 -->
````go
func NewManagement(store *platform.Store, identity *logic.Identity, access *logic.Access) *logic.Management {
	return logic.NewManagement(store.DB, identity, access)
}

// readings.go：保留原实现，按业务合并文件。

````

## 第 12 步：NewReadings

插入位置：第 11 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func NewReadings(access *logic.Access, mainViews *logic.MainReadViews) (*logic.Readings, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewReadings(access, mainViews, validator)</code> |
| <code>mainViews</code> | <code>*logic.MainReadViews</code> | 使用 <code>logic.MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。<br>在此声明中的实际使用：<code>logic.NewReadings(access, mainViews, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Readings</code> | 使用 <code>logic.Readings</code> 类型；其定义处的职责：协调文档详情、MAIN全文、变化摘要和固定视图单元读取。<br>实际返回表达式：<code>nil</code>；<code>logic.NewReadings(access, mainViews, validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewReadings(access, mainViews, validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3094:3394 -->
````go
func NewReadings(access *logic.Access, mainViews *logic.MainReadViews) (*logic.Readings, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewReadings(access, mainViews, validator), nil
}

// sharing.go：保留原实现，按业务合并文件。

````

## 第 13 步：NewSharing

插入位置：第 12 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func NewSharing(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, tokens logic.InvitationCipher, directory *client.AccountDirectory) *logic.Sharing`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Origin</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |
| <code>pagination</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |
| <code>tokens</code> | <code>logic.InvitationCipher</code> | 使用 <code>logic.InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。<br>在此声明中的实际使用：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |
| <code>directory</code> | <code>*client.AccountDirectory</code> | 使用 <code>client.AccountDirectory</code> 类型；其定义处的职责：实现账号服务的会话验证、准入续期、人员目录、通知资格以及邀请加密适配。<br>在此声明中的实际使用：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Sharing</code> | 使用 <code>logic.Sharing</code> 类型；其定义处的职责：实现个人文档邀请、申请、审批、状态分页与安全归档。<br>实际返回表达式：<code>logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=3394:3773 -->
````go
func NewSharing(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, tokens logic.InvitationCipher, directory *client.AccountDirectory) *logic.Sharing {
	return logic.NewSharing(store.DB, identity, access, pagination, tokens, cfg.Origin, directory)
}

// space.go：保留原实现，按业务合并文件。

````

## 第 14 步：NewPagination

插入位置：第 13 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`func NewPagination(cfg platform.Config) *logic.Pagination`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.KeyFile</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>实际返回表达式：<code>logic.NewPagination(client.NewCursorSigner(cfg.KeyFile))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewPagination(client.NewCursorSigner(cfg.KeyFile))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=3773:3900 -->
````go
func NewPagination(cfg platform.Config) *logic.Pagination {
	return logic.NewPagination(client.NewCursorSigner(cfg.KeyFile))
}
````

## 第 15 步：NewDirectory

插入位置：第 14 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`func NewDirectory(cfg platform.Config, accountHTTP *http.Client) *client.AccountDirectory`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Peers</code> |
| <code>accountHTTP</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewAccountDirectory(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*client.AccountDirectory</code> | 使用 <code>client.AccountDirectory</code> 类型；其定义处的职责：实现账号服务的会话验证、准入续期、人员目录、通知资格以及邀请加密适配。<br>实际返回表达式：<code>client.NewAccountDirectory(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return client.NewAccountDirectory(cfg.Peers&#91;&quot;accountHttp&quot;&#93;, accountHTTP)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=3900:4068 -->
````go
func NewDirectory(cfg platform.Config, accountHTTP *http.Client) *client.AccountDirectory {
	return client.NewAccountDirectory(cfg.Peers["accountHttp"], accountHTTP)
}
````

## 第 16 步：NewSpaces

插入位置：第 15 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func NewSpaces(identity *logic.Identity, directory *client.AccountDirectory, pagination *logic.Pagination) *logic.Spaces`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewSpaces(identity, directory, pagination)</code> |
| <code>directory</code> | <code>*client.AccountDirectory</code> | 使用 <code>client.AccountDirectory</code> 类型；其定义处的职责：实现账号服务的会话验证、准入续期、人员目录、通知资格以及邀请加密适配。<br>在此声明中的实际使用：<code>logic.NewSpaces(identity, directory, pagination)</code> |
| <code>pagination</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>logic.NewSpaces(identity, directory, pagination)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Spaces</code> | 使用 <code>logic.Spaces</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>实际返回表达式：<code>logic.NewSpaces(identity, directory, pagination)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewSpaces(identity, directory, pagination)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=4068:4316 -->
````go
func NewSpaces(identity *logic.Identity, directory *client.AccountDirectory, pagination *logic.Pagination) *logic.Spaces {
	return logic.NewSpaces(identity, directory, pagination)
}

// transactions.go：保留原实现，按业务合并文件。

````

## 第 17 步：NewTransactions

插入位置：第 16 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`func NewTransactions(store *platform.Store, identity *logic.Identity, outputs *logic.Outputs) *logic.Transactions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Transactions</code> | 使用 <code>logic.Transactions</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=4316:4593 -->
````go
func NewTransactions(store *platform.Store, identity *logic.Identity, outputs *logic.Outputs) *logic.Transactions {
	return logic.NewTransactions(store.DB, repository.NewOperations(store.DB), identity, outputs)
}

// visibility.go：保留原实现，按业务合并文件。

````

## 第 18 步：NewVisibilities

插入位置：第 17 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`func NewVisibilities(knowledgeBases *logic.KnowledgeBases, access *logic.Access) *logic.Visibilities`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>knowledgeBases</code> | <code>*logic.KnowledgeBases</code> | 使用 <code>logic.KnowledgeBases</code> 类型；其定义处的职责：编排文档/知识库创建、管理、章节布局与个人/组织空间物化。<br>在此声明中的实际使用：<code>logic.NewVisibilities(knowledgeBases, access)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewVisibilities(knowledgeBases, access)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Visibilities</code> | 使用 <code>logic.Visibilities</code> 类型；其定义处的职责：协调可见政策、撤权关闭任务与搜索索引事件的一次业务提交。<br>实际返回表达式：<code>logic.NewVisibilities(knowledgeBases, access)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewVisibilities(knowledgeBases, access)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=4593:4752 -->
````go
func NewVisibilities(knowledgeBases *logic.KnowledgeBases, access *logic.Access) *logic.Visibilities {
	return logic.NewVisibilities(knowledgeBases, access)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/svc/workspace_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
