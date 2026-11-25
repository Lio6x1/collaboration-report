# 分步编写 `services/document/internal/svc/content_workflows_wiring.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/276-content-workflows-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

装配送审、合并、结构预览、草稿控制、修复和可靠MAIN阅读依赖。

用例共享Doc私有池与规范正文validator，远端协作客户端由本服务配置创建。

写之前先按这条执行路线理解各段的连接：

1. 各New函数构造validator和对应client，再注入Identity/Access及Outputs.Self实例身份
2. NewDraftResources注入Node持久事实读取器。

本文件需要守住的边界：

- 初始化validator或客户端失败必须上报，不能启动一个缺恢复依赖的半成品服务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/svc/content_workflows_wiring.go') -and (Get-Item -LiteralPath 'services/document/internal/svc/content_workflows_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/svc/content_workflows_wiring.go')) { New-Item -ItemType File -Path 'services/document/internal/svc/content_workflows_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:372 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	"collaboration/document/internal/client"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"net/http"
)

// conflict_resolutions.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewResolutions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func NewResolutions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Resolutions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewDraftTransitions(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。<br>实际返回表达式：<code>nil</code>；<code>logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=372:909 -->
````go
func NewResolutions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Resolutions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// draft_controls.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewDraftControls

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func NewDraftControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.DraftControls, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewDraftTransitions(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。<br>实际返回表达式：<code>nil</code>；<code>logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=909:1453 -->
````go
func NewDraftControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.DraftControls, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// draft_resources.go：保留原实现，按业务合并文件。

````

## 第 4 步：NewDraftResources

插入位置：第 3 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func NewDraftResources(store *platform.Store, cfg platform.Config, peer *http.Client) *logic.DraftResources`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>cfg.Peers</code> |
| <code>peer</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>client.NewNodePersistence(peer, cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.DraftResources</code> | 使用 <code>logic.DraftResources</code> 类型；其定义处的职责：为协作UPDATE预保护资源并原子预留正文容量。<br>实际返回表达式：<code>logic.NewDraftResources(store.DB, client.NewNodePersistence(peer, cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewDraftResources(store.DB, client.NewNodePersistence(peer, cfg.Peers&#91;&quot;collaborationHttp&quot;&#93;))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1453:1739 -->
````go
func NewDraftResources(store *platform.Store, cfg platform.Config, peer *http.Client) *logic.DraftResources {
	return logic.NewDraftResources(store.DB, client.NewNodePersistence(peer, cfg.Peers["collaborationHttp"]))
}

// main_controls.go：保留原实现，按业务合并文件。

````

## 第 5 步：NewMainControls

插入位置：第 4 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func NewMainControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.MainControls, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewMerges(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。<br>实际返回表达式：<code>nil</code>；<code>logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1739:2264 -->
````go
func NewMainControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.MainControls, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// main_read.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewMainReadViews

插入位置：第 5 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func NewMainReadViews(store *platform.Store, cfg platform.Config, identity *logic.Identity) (*logic.MainReadViews, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewMerges(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewMainReadViews(store.DB, identity, client.NewMerges(cfg, peer), validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.MainReadViews</code> | 使用 <code>logic.MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。<br>实际返回表达式：<code>nil</code>；<code>logic.NewMainReadViews(store.DB, identity, client.NewMerges(cfg, peer), validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewMainReadViews(store.DB, identity, client.NewMerges(cfg, peer), validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2264:2716 -->
````go
func NewMainReadViews(store *platform.Store, cfg platform.Config, identity *logic.Identity) (*logic.MainReadViews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMainReadViews(store.DB, identity, client.NewMerges(cfg, peer), validator), nil
}

// merges.go：保留原实现，按业务合并文件。

````

## 第 7 步：NewMerges

插入位置：第 6 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func NewMerges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Merges, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewMerges(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。<br>实际返回表达式：<code>nil</code>；<code>logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2716:3223 -->
````go
func NewMerges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Merges, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// pull_main.go：保留原实现，按业务合并文件。

````

## 第 8 步：NewPullMain

插入位置：第 7 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func NewPullMain(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.PullMain, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewDraftTransitions(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。<br>实际返回表达式：<code>nil</code>；<code>logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3223:3752 -->
````go
func NewPullMain(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.PullMain, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// reverse_changes.go：保留原实现，按业务合并文件。

````

## 第 9 步：NewReverseChanges

插入位置：第 8 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func NewReverseChanges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.ReverseChanges, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewDraftTransitions(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。<br>实际返回表达式：<code>nil</code>；<code>logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3752:4291 -->
````go
func NewReverseChanges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.ReverseChanges, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// reviews.go：保留原实现，按业务合并文件。

````

## 第 10 步：NewReviews

插入位置：第 9 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func NewReviews(store *platform.Store, access *logic.Access, pagination *logic.Pagination, directory logic.DirectoryReader) (*logic.Reviews, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewReviews(store.DB, access, pagination, directory, validator)</code> |
| <code>pagination</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>logic.NewReviews(store.DB, access, pagination, directory, validator)</code> |
| <code>directory</code> | <code>logic.DirectoryReader</code> | 使用 <code>logic.DirectoryReader</code> 类型；其定义处的职责：空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。<br>在此声明中的实际使用：<code>logic.NewReviews(store.DB, access, pagination, directory, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Reviews</code> | 使用 <code>logic.Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>实际返回表达式：<code>nil</code>；<code>logic.NewReviews(store.DB, access, pagination, directory, validator)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewReviews(store.DB, access, pagination, directory, validator), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4291:4675 -->
````go
func NewReviews(store *platform.Store, access *logic.Access, pagination *logic.Pagination, directory logic.DirectoryReader) (*logic.Reviews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewReviews(store.DB, access, pagination, directory, validator), nil
}

// structure_previews.go：保留原实现，按业务合并文件。

````

## 第 11 步：NewStructurePreviews

插入位置：第 10 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func NewStructurePreviews(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.StructurePreviews, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>platform.HTTPPeer(cfg, &quot;collaboration&quot;)</code>；<code>client.NewDraftTransitions(cfg, peer)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。<br>实际返回表达式：<code>nil</code>；<code>logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4675:5227 -->
````go
func NewStructurePreviews(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.StructurePreviews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// submissions.go：保留原实现，按业务合并文件。

````

## 第 12 步：NewSubmissions

插入位置：第 11 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`func NewSubmissions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Submissions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>cfg</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>client.NewSubmissionCaptures(cfg)</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID)</code> |
| <code>outputs</code> | <code>*logic.Outputs</code> | 使用 <code>logic.Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs.Self</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。<br>实际返回表达式：<code>nil</code>；<code>logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5227:5600 -->
````go
func NewSubmissions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Submissions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID), nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/svc/content_workflows_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
