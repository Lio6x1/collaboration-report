# 分步编写 `services/document/internal/svc/lifecycle_wiring.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/277-lifecycle-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把账号留存、组织关闭、个人转移、永久清理及治理依赖组合为用例。

公开留存依赖校验需要统一content validator，物理清理与权威身份由接口注入。

写之前先按这条执行路线理解各段的连接：

1. NewAccountRetentions/NewOrganizationClosures创建validator
2. 其余构造传Store.DB、原instance、额度与cleaner/drainer。

本文件需要守住的边界：

- 复用私有池而非新建数据库
- 不要把组织关闭装成账号注销资格。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/svc/lifecycle_wiring.go') -and (Get-Item -LiteralPath 'services/document/internal/svc/lifecycle_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/svc/lifecycle_wiring.go')) { New-Item -ItemType File -Path 'services/document/internal/svc/lifecycle_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:331 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"database/sql"
)

// account_retention.go：保留原实现，按业务合并文件。

````

## 第 2 步：NewAccountRetentions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func NewAccountRetentions(db *sql.DB, authority logic.AccountRetentionAuthority, cleaner logic.AccountDocumentCleaner) (*logic.AccountRetentions, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>logic.NewAccountRetentions(db, authority, validator, cleaner)</code> |
| <code>authority</code> | <code>logic.AccountRetentionAuthority</code> | 使用 <code>logic.AccountRetentionAuthority</code> 类型；其定义处的职责：Doc清理账号内容前核对Account权威关闭事实，不把收到RPC等同用户已确认。<br>在此声明中的实际使用：<code>logic.NewAccountRetentions(db, authority, validator, cleaner)</code> |
| <code>cleaner</code> | <code>logic.AccountDocumentCleaner</code> | 使用 <code>logic.AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>logic.NewAccountRetentions(db, authority, validator, cleaner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.AccountRetentions</code> | 使用 <code>logic.AccountRetentions</code> 类型；其定义处的职责：固定账号原公开正式版依赖，并在各文档清理完成后启用留存。<br>实际返回表达式：<code>nil</code>；<code>logic.NewAccountRetentions(db, authority, validator, cleaner)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewAccountRetentions(db, authority, validator, cleaner), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=331:713 -->
````go
func NewAccountRetentions(db *sql.DB, authority logic.AccountRetentionAuthority, cleaner logic.AccountDocumentCleaner) (*logic.AccountRetentions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewAccountRetentions(db, authority, validator, cleaner), nil
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 3 步：NewOrganizationClosures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func NewOrganizationClosures(db *sql.DB, authority logic.OrganizationCloseAuthority, cleaner logic.AccountDocumentCleaner) (*logic.OrganizationClosures, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, authority, validator, cleaner)</code> |
| <code>authority</code> | <code>logic.OrganizationCloseAuthority</code> | 使用 <code>logic.OrganizationCloseAuthority</code> 类型；其定义处的职责：组织关闭的权威确认接口，与账号注销绑定分别核验。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, authority, validator, cleaner)</code> |
| <code>cleaner</code> | <code>logic.AccountDocumentCleaner</code> | 使用 <code>logic.AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>logic.NewOrganizationClosures(db, authority, validator, cleaner)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.OrganizationClosures</code> | 使用 <code>logic.OrganizationClosures</code> 类型；其定义处的职责：仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。<br>实际返回表达式：<code>nil</code>；<code>logic.NewOrganizationClosures(db, authority, validator, cleaner)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return logic.NewOrganizationClosures(db, authority, validator, cleaner), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=713:1104 -->
````go
func NewOrganizationClosures(db *sql.DB, authority logic.OrganizationCloseAuthority, cleaner logic.AccountDocumentCleaner) (*logic.OrganizationClosures, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewOrganizationClosures(db, authority, validator, cleaner), nil
}

// personal_transfer.go：保留原实现，按业务合并文件。

````

## 第 4 步：NewPersonalTransfers

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func NewPersonalTransfers(store *platform.Store, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, fences *logic.Fences, storage logic.PersonalTransferStorageReader, quotaLimit uint64, instance string) *logic.PersonalTransfers`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>identity</code> | <code>*logic.Identity</code> | 使用 <code>logic.Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>access</code> | <code>*logic.Access</code> | 使用 <code>logic.Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>pagination</code> | <code>*logic.Pagination</code> | 使用 <code>logic.Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>fences</code> | <code>*logic.Fences</code> | 使用 <code>logic.Fences</code> 类型；其定义处的职责：协调Document本地流、Edge和Node的来源撤权与真实排空。<br>在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>storage</code> | <code>logic.PersonalTransferStorageReader</code> | 使用 <code>logic.PersonalTransferStorageReader</code> 类型；其定义处的职责：跨空间迁移前从协作存储核对字节数及绑定，避免只移动文档元数据却漏计正文容量。<br>在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>quotaLimit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.PersonalTransfers</code> | 使用 <code>logic.PersonalTransfers</code> 类型；其定义处的职责：提供个人文档邀约、参与者列表、取消拒绝和接受登记。<br>实际返回表达式：<code>logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1104:1536 -->
````go
func NewPersonalTransfers(store *platform.Store, identity *logic.Identity, access *logic.Access, pagination *logic.Pagination, fences *logic.Fences, storage logic.PersonalTransferStorageReader, quotaLimit uint64, instance string) *logic.PersonalTransfers {
	return logic.NewPersonalTransfers(store.DB, identity, access, pagination, fences, storage, quotaLimit, instance)
}

// purge.go：保留原实现，按业务合并文件。

````

## 第 5 步：NewPurges

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func NewPurges(store *platform.Store, trash *logic.Trash, cleaner logic.AccountDocumentCleaner, instance string) *logic.Purges`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>trash</code> | <code>*logic.Trash</code> | 使用 <code>logic.Trash</code> 类型；其定义处的职责：Trash独立于普通Access的ACTIVE入口，但共用真实身份、签名分页和事务输出协议。 不持有自己的事务协调器，handler须通过现有Transactions提交与登记输出。<br>在此声明中的实际使用：<code>logic.NewPurges(store.DB, trash, cleaner, instance)</code> |
| <code>cleaner</code> | <code>logic.AccountDocumentCleaner</code> | 使用 <code>logic.AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>logic.NewPurges(store.DB, trash, cleaner, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>logic.NewPurges(store.DB, trash, cleaner, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.Purges</code> | 使用 <code>logic.Purges</code> 类型；其定义处的职责：将用户确认永久删除固定成异步清理计划。<br>实际返回表达式：<code>logic.NewPurges(store.DB, trash, cleaner, instance)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewPurges(store.DB, trash, cleaner, instance)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1536:1800 -->
````go
func NewPurges(store *platform.Store, trash *logic.Trash, cleaner logic.AccountDocumentCleaner, instance string) *logic.Purges {
	return logic.NewPurges(store.DB, trash, cleaner, instance)
}

// retained_governance.go：保留原实现，按业务合并文件。

````

## 第 6 步：NewRetainedGovernance

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func NewRetainedGovernance(store *platform.Store, authority logic.RetainedGovernanceAuthority, cleaner logic.AccountDocumentCleaner, resources logic.RetainedResourceCleaner, exports logic.RetainedExportDrainer, instance string) *logic.RetainedGovernance`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store.DB</code> |
| <code>authority</code> | <code>logic.RetainedGovernanceAuthority</code> | 使用 <code>logic.RetainedGovernanceAuthority</code> 类型；其定义处的职责：留存内容治理独立于原账号普通权限，必须重新取得平台治理权威准入。<br>在此声明中的实际使用：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |
| <code>cleaner</code> | <code>logic.AccountDocumentCleaner</code> | 使用 <code>logic.AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |
| <code>resources</code> | <code>logic.RetainedResourceCleaner</code> | 使用 <code>logic.RetainedResourceCleaner</code> 类型；其定义处的职责：留存治理的资源清理边界，与普通删除共用引用及容量核验。<br>在此声明中的实际使用：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |
| <code>exports</code> | <code>logic.RetainedExportDrainer</code> | 使用 <code>logic.RetainedExportDrainer</code> 类型；其定义处的职责：治理清理前先停止依赖旧正文的导出，避免转换器仍在读取被删除的文件。<br>在此声明中的实际使用：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*logic.RetainedGovernance</code> | 使用 <code>logic.RetainedGovernance</code> 类型；其定义处的职责：取得Account平台治理资格并登记孤立留存撤下/删除。<br>实际返回表达式：<code>logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1800:2154 -->
````go
func NewRetainedGovernance(store *platform.Store, authority logic.RetainedGovernanceAuthority, cleaner logic.AccountDocumentCleaner, resources logic.RetainedResourceCleaner, exports logic.RetainedExportDrainer, instance string) *logic.RetainedGovernance {
	return logic.NewRetainedGovernance(store.DB, authority, cleaner, resources, exports, instance)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/svc/lifecycle_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
