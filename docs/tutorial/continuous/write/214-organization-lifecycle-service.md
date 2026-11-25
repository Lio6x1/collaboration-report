# 分步编写 `services/document/internal/logic/document/organization_lifecycle_service.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/214-organization-lifecycle-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

独立执行组织关闭的固定公开留存、单文档清理与完成。

只组合复用不可变依赖解析，不继承账号注销的个人空间或身份关闭资格。

写之前先按这条执行路线理解各段的连接：

1. Plan核组织权威屏障并存原集合
2. Progress确认排空进入CLEANING
3. DocumentCleanupContext发单Doc证
4. ReleaseDraftResources/CleanHistory/CleanPayloads分阶段
5. Complete核回执。

本文件需要守住的边界：

- 本人identity完全不参与关闭
- 原Account清理回执不能从组织端点冒用返回
- 完成仍需Account结束组织操作。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/organization_lifecycle_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/organization_lifecycle_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/organization_lifecycle_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/organization_lifecycle_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:544 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
)

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：OrganizationCloseAuthority

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type OrganizationCloseAuthority interface {`。

**这一段为什么这样写：** 组织关闭的权威确认接口，与账号注销绑定分别核验。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Confirmed</code> | <code>(context.Context, platform.OrganizationClosureBinding) (bool, error)</code> | ctx与原组织关闭绑定交给Account核验确认结果；error表示未知，不能继续不可逆清理。 |

<!-- write-step: 2 bytes=544:670 -->
````go
type OrganizationCloseAuthority interface {
	Confirmed(context.Context, platform.OrganizationClosureBinding) (bool, error)
}

````

## 第 3 步：OrganizationClosures

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。

本步声明：`type OrganizationClosures struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>authority</code> | <code>OrganizationCloseAuthority</code> | 使用 <code>OrganizationCloseAuthority</code> 类型；其定义处的职责：组织关闭的权威确认接口，与账号注销绑定分别核验。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。 |
| <code>dependencies</code> | <code>*AccountRetentions</code> | 使用 <code>AccountRetentions</code> 类型；其定义处的职责：固定账号原公开正式版依赖，并在各文档清理完成后启用留存。 |

<!-- write-step: 3 bytes=670:975 -->
````go
// 仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。
type OrganizationClosures struct {
	db           *sql.DB
	authority    OrganizationCloseAuthority
	validator    *codec.Validator
	cleaner      AccountDocumentCleaner
	dependencies *AccountRetentions
}

````

## 第 4 步：NewOrganizationClosures

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func NewOrganizationClosures(db *sql.DB, authority OrganizationCloseAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *OrganizationClosures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code>；<code>NewAccountRetentions(db, nil, validator, nil)</code> |
| <code>authority</code> | <code>OrganizationCloseAuthority</code> | 使用 <code>OrganizationCloseAuthority</code> 类型；其定义处的职责：组织关闭的权威确认接口，与账号注销绑定分别核验。<br>在此声明中的实际使用：<code>authority: authority</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code>；<code>NewAccountRetentions(db, nil, validator, nil)</code> |
| <code>cleaner</code> | <code>AccountDocumentCleaner</code> | 使用 <code>AccountDocumentCleaner</code> 类型；其定义处的职责：跨服务正文清理能力；保留任务/绑定，只有参与者确认后才推进后续清理。<br>在此声明中的实际使用：<code>cleaner: cleaner</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationClosures</code> | 使用 <code>OrganizationClosures</code> 类型；其定义处的职责：仅复用不可变正式版依赖解析器；所有入口及证明均由独立组织计划约束。<br>实际返回表达式：<code>&amp;OrganizationClosures{db: db, authority: authority, validator: validator, cleaner: cleaner, dependencies: NewAccountRetentions(db, nil, validator, nil)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationClosures{db: db, authority: authority, validator: validator, cleaner: cleaner, dependencies: NewAccountRetentions(db, nil, validator, nil)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=975:1302 -->
````go
func NewOrganizationClosures(db *sql.DB, authority OrganizationCloseAuthority, validator *codec.Validator, cleaner AccountDocumentCleaner) *OrganizationClosures {
	return &OrganizationClosures{db: db, authority: authority, validator: validator, cleaner: cleaner, dependencies: NewAccountRetentions(db, nil, validator, nil)}
}

````

## 第 5 步：OrganizationClosures.retainedDocument

插入位置：第 4 步代码之后，紧接追加；完成稿约第 40 行。

组合复用纯依赖解析，避免继承账号注销的权限入口或身份来源检查。

本步声明：`func (s *OrganizationClosures) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.dependencies.retainedDocument(ctx, tx, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.dependencies.retainedDocument(ctx, tx, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.dependencies.retainedDocument(ctx, tx, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>由 <code>s.dependencies.retainedDocument(ctx, tx, id)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.dependencies.retainedDocument(ctx, tx, id)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.dependencies.retainedDocument(ctx, tx, id)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1302:1589 -->
````go
// 组合复用纯依赖解析，避免继承账号注销的权限入口或身份来源检查。
func (s *OrganizationClosures) retainedDocument(ctx context.Context, tx *sql.Tx, id uint64) (domain.AccountRetentionDocument, error) {
	return s.dependencies.retainedDocument(ctx, tx, id)
}
````

## 第 6 步：OrganizationClosures.retainedDependencies

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (s *OrganizationClosures) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>由 <code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.dependencies.retainedDependencies(ctx, tx, document, item)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.dependencies.retainedDependencies(ctx, tx, document, item)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1589:1850 -->
````go
func (s *OrganizationClosures) retainedDependencies(ctx context.Context, tx *sql.Tx, document dto.Document, item domain.AccountRetentionDocument) (domain.AccountRetentionDocument, error) {
	return s.dependencies.retainedDependencies(ctx, tx, document, item)
}

````

## 第 7 步：OrganizationClosures.HandlesCleanup

插入位置：第 6 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func (s *OrganizationClosures) HandlesCleanup(ctx context.Context, operation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LifecycleKind(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.LifecycleKind(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>kind == &quot;ORGANIZATION_CLOSE&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return kind == &quot;ORGANIZATION_CLOSE&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1850:2116 -->
````go
func (s *OrganizationClosures) HandlesCleanup(ctx context.Context, operation string) (bool, error) {
	kind, err := repository.LifecycleKind(ctx, s.db, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return kind == "ORGANIZATION_CLOSE", err
}

````

## 第 8 步：boundOrganizationClosePlan

插入位置：第 7 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func boundOrganizationClosePlan(record repository.AccountRetirementRecord, b platform.OrganizationClosureBinding) (domain.OrganizationClosePlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>record</code> | <code>repository.AccountRetirementRecord</code> | 使用 <code>repository.AccountRetirementRecord</code> 类型；其定义处的职责：保存账号注销准备、冻结、拒绝及显式取消的持久事实。<br>在此声明中的实际使用：<code>record.Plan</code>；<code>record.Kind</code>；<code>record.Actor</code>；<code>record.Epoch</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.User</code>；<code>b.Epoch</code>；<code>plan.Binding != b</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_STATE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; record.Kind != &quot;ORGANIZATION_CLOSE&quot; &#124;&#124; record.Actor != b.User &#124;&#124; record.Epoch != b.Epoch &#124;&#124; plan.Binding != b</code>。条件成立时执行 <code>return plan, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;PLANNED&quot; &amp;&amp; record.State != &quot;CLEANING&quot; &amp;&amp; record.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return plan, errors.New(&quot;ORGANIZATION_CLOSE_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2116:2708 -->
````go
func boundOrganizationClosePlan(record repository.AccountRetirementRecord, b platform.OrganizationClosureBinding) (domain.OrganizationClosePlan, error) {
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil || record.Kind != "ORGANIZATION_CLOSE" || record.Actor != b.User || record.Epoch != b.Epoch || plan.Binding != b {
		return plan, errors.New("ORGANIZATION_CLOSE_BINDING_CHANGED")
	}
	if record.State != "PLANNED" && record.State != "CLEANING" && record.State != "SUCCEEDED" {
		return plan, errors.New("ORGANIZATION_CLOSE_STATE_INVALID")
	}
	return plan, nil
}

````

## 第 9 步：OrganizationClosures.Plan

插入位置：第 8 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`func (s *OrganizationClosures) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authority.Confirmed(ctx, b)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.DiscoverOrganizationClose(ctx, tx, b)</code>；<code>repository.AccountRetirementSnapshot(ctx, tx, b.Operation)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Valid</code>；<code>s.authority.Confirmed(ctx, b)</code>；<code>repository.DiscoverOrganizationClose(ctx, tx, b)</code>；<code>b.Operation</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_REQUIRED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ORGANIZATION_CLOSE_SOURCE_UNCONFIRMED&quot;)</code>；<code>readErr</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!b.Valid() &#124;&#124; s.authority == nil &#124;&#124; s.validator == nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_AUTHORITY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!confirmed</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ORGANIZATION_CLOSE_SOURCE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>readErr != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return readErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=2708:4285 -->
````go
func (s *OrganizationClosures) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error {
	if !b.Valid() || s.authority == nil || s.validator == nil {
		return errors.New("ORGANIZATION_CLOSE_AUTHORITY_REQUIRED")
	}
	confirmed, err := s.authority.Confirmed(ctx, b)
	if err != nil {
		return err
	}
	if !confirmed {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_SOURCE_UNCONFIRMED")
	}
	// Account不可逆组织关闭证明先取得；SQL事务不等待任何跨服务调用。
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	candidates, err := repository.DiscoverOrganizationClose(ctx, tx, b)
	if err != nil {
		// 已规划重放时父空间已经CLOSING；先尝试读取同号，绝不重新选计划。
		record, readErr := repository.AccountRetirementSnapshot(ctx, tx, b.Operation)
		if readErr != nil {
			return err
		}
		_, readErr = boundOrganizationClosePlan(record, b)
		return readErr
	}
	if err = lockOrganizationCloseSources(ctx, tx, candidates.Sources, b); err != nil {
		return err
	}
	original, err := repository.LockedAccountRetirement(ctx, tx, b.Operation)
	if err == nil {
		_, err = boundOrganizationClosePlan(original, b)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	keys := make([]string, 0, len(candidates.Sources))
	for _, source := range candidates.Sources {
		keys = append(keys, source.Key)
	}
	pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, keys)
	if err != nil {
		return err
	}
````

## 第 10 步：OrganizationClosures.Plan（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 113 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;SCOPE_TRANSITION_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!slices.Equal(candidates.Spaces, current.Spaces) &#124;&#124; !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) &#124;&#124; !slices.Equal(candidates.Documents, current.Documents) &#124;&#124; !s …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ORGANIZATION_CLOSE_CANDIDATES_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; epoch == math.MaxUint64</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!retention.Valid(b.Operation)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_RETENTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=4285:6332 -->
````go
	if pending {
		return domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
	}
	if err = repository.LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	current, err := repository.DiscoverOrganizationClose(ctx, tx, b)
	if err != nil {
		return err
	}
	if !slices.Equal(candidates.Spaces, current.Spaces) || !slices.Equal(candidates.KnowledgeBases, current.KnowledgeBases) || !slices.Equal(candidates.Documents, current.Documents) || !slices.Equal(candidates.Sources, current.Sources) {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_CANDIDATES_CHANGED")
	}
	retention := domain.AccountRetentionPlan{Spaces: current.Spaces, KnowledgeBases: current.KnowledgeBases, Documents: []domain.AccountRetentionDocument{}, Scopes: []domain.AccountRetentionScope{}}
	for _, id := range current.Documents {
		item, err := s.retainedDocument(ctx, tx, id)
		if err != nil {
			return err
		}
		retention.Documents = append(retention.Documents, item)
	}
	for _, source := range current.Sources {
		epoch, err := domain.CanonicalID(source.Epoch)
		if err != nil || epoch == math.MaxUint64 {
			return errors.New("ORGANIZATION_CLOSE_EPOCH_EXHAUSTED")
		}
		retention.Scopes = append(retention.Scopes, domain.AccountRetentionScope{Key: source.Key, Epoch: epoch + 1, Operation: domain.AccountRetentionRevocation(b.Operation, source.Key)})
	}
	if !retention.Valid(b.Operation) {
		return errors.New("ORGANIZATION_CLOSE_RETENTION_INVALID")
	}
	plan := domain.OrganizationClosePlan{Binding: b, Retention: &retention}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.RecordOrganizationClose(ctx, tx, b, raw); err != nil {
		return err
	}
	if err = repository.SaveOrganizationClose(ctx, tx, b.Operation, raw, retention); err != nil {
		return err
	}
	for _, scope := range retention.Scopes {
		if err = QueueClosure(ctx, tx, scope.Operation, b.User, domain.Closure{Scope: scope.Key, Epoch: scope.Epoch, Action: "ORGANIZATION_RETENTION"}); err != nil {
			return err
		}
	}
````

## 第 11 步：OrganizationClosures.Plan（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 160 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PLAN_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6332:6548 -->
````go
	if err = queueVisibilityIndexEvents(ctx, tx, b.Operation, current.Documents, 0); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_PLAN_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 12 步：OrganizationClosures.State

插入位置：第 11 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`func (s *OrganizationClosures) State(ctx context.Context, b platform.OrganizationClosureBinding) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Operation</code>；<code>boundOrganizationClosePlan(record, b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>record.State</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 此处通过 <code>return record.State, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6548:6891 -->
````go
func (s *OrganizationClosures) State(ctx context.Context, b platform.OrganizationClosureBinding) (string, error) {
	record, err := repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)
	if err != nil {
		return "", err
	}
	if _, err = boundOrganizationClosePlan(record, b); err != nil {
		return "", err
	}
	return record.State, nil
}

````

## 第 13 步：lockOrganizationCloseSources

插入位置：第 12 步代码之后，紧接追加；完成稿约第 180 行。

来源按字符串排序统一锁定，原组织来源及所有Doc/KB来源先于业务对象。
本人identity完全不参与关闭，个人空间的来源继续可用。

本步声明：`func lockOrganizationCloseSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, b platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, page)</code>；<code>repository.OrganizationCloseGateMatches(ctx, tx, b)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, page)</code>；<code>repository.OrganizationCloseGateMatches(ctx, tx, b)</code> |
| <code>sources</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>slices.Clone(sources)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Scope</code>；<code>b.Epoch</code>；<code>repository.OrganizationCloseGateMatches(ctx, tx, b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_SOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return strings.Compare(a.Key, c.Key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>source.Key != b.Scope() &amp;&amp; gates&#91;source.Key&#93;.Epoch &gt; epoch</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6891:8073 -->
````go
// 来源按字符串排序统一锁定，原组织来源及所有Doc/KB来源先于业务对象。
// 本人identity完全不参与关闭，个人空间的来源继续可用。
func lockOrganizationCloseSources(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource, b platform.OrganizationClosureBinding) error {
	all := slices.Clone(sources)
	all = append(all, domain.LeaseSource{Key: b.Scope(), Epoch: domain.TextID(b.Epoch)})
	slices.SortFunc(all, func(a, c domain.LeaseSource) int { return strings.Compare(a.Key, c.Key) })
	for start := 0; start < len(all); start += 100 {
		page := all[start:min(start+100, len(all))]
		gates, err := repository.LockSourceGatesBatch(ctx, tx, page)
		if err != nil {
			return err
		}
		for _, source := range page {
			epoch, err := domain.CanonicalID(source.Epoch)
			if err != nil {
				return err
			}
			if source.Key != b.Scope() && gates[source.Key].Epoch > epoch {
				return errors.New("ORGANIZATION_CLOSE_SOURCE_CHANGED")
			}
		}
	}
	matches, err := repository.OrganizationCloseGateMatches(ctx, tx, b)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ORGANIZATION_CLOSE_SOURCE_CHANGED")
	}
	return nil
}

````

## 第 14 步：OrganizationClosures.ensureCleaning

插入位置：第 13 步代码之后，紧接追加；完成稿约第 212 行。

本步声明：`func (s *OrganizationClosures) ensureCleaning(ctx context.Context, b platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>lockOrganizationCloseSources(ctx, tx, sources, b)</code>；<code>repository.LockedAccountRetirement(ctx, tx, b.Operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Operation</code>；<code>boundOrganizationClosePlan(original, b)</code>；<code>lockOrganizationCloseSources(ctx, tx, sources, b)</code>；<code>boundOrganizationClosePlan(locked, b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_PLAN_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ORGANIZATION_CLOSE_DRAIN_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;CLEANING&quot; &#124;&#124; original.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>string(original.Plan) != string(locked.Plan)</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;CLEANING&quot; &#124;&#124; locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ORGANIZATION_CLOSE_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=8073:9468 -->
````go
func (s *OrganizationClosures) ensureCleaning(ctx context.Context, b platform.OrganizationClosureBinding) error {
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, b.Operation)
	if err != nil {
		return err
	}
	plan, err := boundOrganizationClosePlan(original, b)
	if err != nil {
		return err
	}
	if original.State == "CLEANING" || original.State == "SUCCEEDED" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sources := []domain.LeaseSource{}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	if err = lockOrganizationCloseSources(ctx, tx, sources, b); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, b.Operation)
	if err != nil {
		return err
	}
	current, err := boundOrganizationClosePlan(locked, b)
	if err != nil {
		return err
	}
	if string(original.Plan) != string(locked.Plan) {
		return errors.New("ORGANIZATION_CLOSE_PLAN_CHANGED")
	}
	if locked.State == "CLEANING" || locked.State == "SUCCEEDED" {
		return nil
	}
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ORGANIZATION_CLOSE_DRAIN_PENDING")
	}
````

## 第 15 步：OrganizationClosures.ensureCleaning（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 257 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_CLEANING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=9468:9756 -->
````go
	if err = repository.StartOrganizationCleaning(ctx, tx, b.Operation); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("ORGANIZATION_CLOSE_CLEANING_UNKNOWN")
	}
	return nil
}

// organization_close_cleanup_context.go：保留原实现，按业务合并文件。

````

## 第 16 步：OrganizationClosures.cleanupExpected

插入位置：第 15 步代码之后，紧接追加；完成稿约第 268 行。

原资源/历史回执允许终态重放，但先绑定本流程固定计划，不能把旧账号清理回执
从组织端点返回，也不能因节点旧回执存在而跳过原文档/代次核验。

本步声明：`func (s *OrganizationClosures) cleanupExpected(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, request.Operation)</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>request.Operation</code>；<code>request.Document</code>；<code>platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_DOCUMENT_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_DOCUMENT_CLEANING_REQUIRED&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>record.State != &quot;CLEANING&quot; &amp;&amp; record.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_CLEANING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>scope.Key == &quot;doc-acl:&quot;+request.Document</code>。条件成立时执行 <code>return platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation), nil</code>。当前函数在这里结束。
- 此处通过 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_SCOPE_MISSING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=9756:11129 -->
````go
// 原资源/历史回执允许终态重放，但先绑定本流程固定计划，不能把旧账号清理回执
// 从组织端点返回，也不能因节点旧回执存在而跳过原文档/代次核验。
func (s *OrganizationClosures) cleanupExpected(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ORGANIZATION_DOCUMENT_REQUEST_INVALID")
	}
	record, err := repository.AccountRetirementSnapshot(ctx, s.db, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
		return empty, err
	}
	if _, err = boundOrganizationClosePlan(record, plan.Binding); err != nil {
		return empty, err
	}
	if record.State != "CLEANING" && record.State != "SUCCEEDED" {
		return empty, errors.New("ORGANIZATION_DOCUMENT_CLEANING_REQUIRED")
	}
	document, _ := domain.CanonicalID(request.Document)
	_, scopes, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document {
			return platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation), nil
		}
	}
	return empty, errors.New("ORGANIZATION_DOCUMENT_SCOPE_MISSING")
}

````

## 第 17 步：OrganizationClosures.DocumentCleanupContext

插入位置：第 16 步代码之后，紧接追加；完成稿约第 302 行。

DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
的Account组织来源关闭，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。

本步声明：`func (s *OrganizationClosures) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.lockedDocumentCleanupContext(ctx, tx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=11129:12128 -->
````go
// DocumentCleanupContext只给Node证明一个固定文档可清理。CLEANING来自已确认且不可逆
// 的Account组织来源关闭，不在SQL锁内再次网络查询Account。来源→lifecycle→Doc锁序与原计划相同。
func (s *OrganizationClosures) DocumentCleanupContext(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, domain.Violation(domain.InvalidRequest, "ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	result, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	// 返回给Node之前先结束只读事务，SQL错误不能被成功HTTP掩盖。
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}

````

## 第 18 步：OrganizationClosures.lockedDocumentCleanupContext

插入位置：第 17 步代码之后，紧接追加；完成稿约第 325 行。

可被后续资源/历史清理复用；调用者拥有事务，不在此处提交或跨服务通信。

本步声明：`func (s *OrganizationClosures) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, tx, request.Operation)</code>；<code>lockOrganizationCloseSources(ctx, tx, sources, plan.Binding)</code>；<code>repository.LockedAccountRetirement(ctx, tx, request.Operation)</code>；<code>repository.OrganizationCloseDrained(ctx, tx, current)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, tx, request.Operation)</code>；<code>lockOrganizationCloseSources(ctx, tx, sources, plan.Binding)</code>；<code>repository.LockedAccountRetirement(ctx, tx, request.Operation)</code>；<code>repository.OrganizationCloseDrained(ctx, tx, current)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Document</code>；<code>request.Operation</code>；<code>platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANING_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_DRAIN_PENDING&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>plan.Binding.Operation != request.Operation</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; original.State != &quot;CLEANING&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 此处通过 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANING_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>locked.State != &quot;CLEANING&quot; &#124;&#124; string(locked.Plan) != string(original.Plan)</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=12128:13887 -->
````go
// 可被后续资源/历史清理复用；调用者拥有事务，不在此处提交或跨服务通信。
func (s *OrganizationClosures) lockedDocumentCleanupContext(ctx context.Context, tx *sql.Tx, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	document, err := domain.CanonicalID(request.Document)
	if err != nil {
		return empty, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(original.Plan)
	if err != nil {
		return empty, err
	}
	if plan.Binding.Operation != request.Operation {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	plan, err = boundOrganizationClosePlan(original, plan.Binding)
	if err != nil || original.State != "CLEANING" {
		if err != nil {
			return empty, err
		}
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANING_REQUIRED")
	}
	item, scopes, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	sources := make([]domain.LeaseSource, 0, len(scopes))
	for _, scope := range scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	if err = lockOrganizationCloseSources(ctx, tx, sources, plan.Binding); err != nil {
		return empty, err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	current, err := boundOrganizationClosePlan(locked, plan.Binding)
	if err != nil {
		return empty, err
	}
	if locked.State != "CLEANING" || string(locked.Plan) != string(original.Plan) {
````

## 第 19 步：OrganizationClosures.lockedDocumentCleanupContext（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 370 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_PLAN_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>!drained</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>pending</code>。条件成立时执行 <code>return empty, domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_WRITES_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Key == &quot;doc-acl:&quot;+request.Document</code>。
- 检查 <code>!result.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_SCOPE_MISSING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=13887:15283 -->
````go
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_PLAN_CHANGED")
	}
	// 只核验这个文档的doc-acl/publication/父KB屏障，其他文档的未知操作不能
	// 使本次单文档资格变成全局排队。每个屏障仍验证原任务、gate及双方真实ACK摘要。
	retention := *current.Retention
	retention.Scopes = scopes
	current.Retention = &retention
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
		return empty, err
	}
	if !drained {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_DRAIN_PENDING")
	}
	if err = repository.LockOrganizationCleanupDocument(ctx, tx, current, item, scopes); err != nil {
		return empty, err
	}
	pending, err := repository.AccountDocumentWritesPending(ctx, tx, document)
	if err != nil {
		return empty, err
	}
	if pending {
		return empty, domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_WRITES_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document {
			result := platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)
			if !result.Valid() {
				return empty, errors.New("ACCOUNT_DOCUMENT_CONTEXT_INVALID")
			}
			return result, nil
		}
	}
	return empty, errors.New("ACCOUNT_DOCUMENT_SCOPE_MISSING")
}

// organization_close_complete.go：保留原实现，按业务合并文件。

````

## 第 20 步：OrganizationClosures.Complete

插入位置：第 19 步代码之后，紧接追加；完成稿约第 408 行。

Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
循环同步清理整个组织。Account恢复器应逐文档推进后调用本关口，原号重放。
Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须结束组织原操作的事务。

本步声明：`func (s *OrganizationClosures) Complete(ctx context.Context, binding platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>lockOrganizationCompletionSources(ctx, tx, plan)</code>；<code>repository.LockedAccountRetirement(ctx, tx, binding.Operation)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>binding</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>binding.Valid</code>；<code>binding.Operation</code>；<code>boundOrganizationClosePlan(original, binding)</code>；<code>boundOrganizationClosePlan(locked, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_CLEANING_REQUIRED&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid() &#124;&#124; s.validator == nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;CLEANING&quot; &#124;&#124; plan.Retention == nil</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_CLEANING_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>locked.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>locked.State != &quot;CLEANING&quot; &#124;&#124; string(original.Plan) != string(locked.Plan)</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=15283:16955 -->
````go
// Complete只消费已持久成功的单文档清理回执，不在SQL锁内跨服务调用，也不
// 循环同步清理整个组织。Account恢复器应逐文档推进后调用本关口，原号重放。
// Doc SUCCEEDED仅表示Doc保留/清理完成，Account尚须结束组织原操作的事务。
func (s *OrganizationClosures) Complete(ctx context.Context, binding platform.OrganizationClosureBinding) error {
	if !binding.Valid() || s.validator == nil {
		return errors.New("ACCOUNT_RETENTION_BINDING_INVALID")
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return err
	}
	plan, err := boundOrganizationClosePlan(original, binding)
	if err != nil {
		return err
	}
	if original.State == "SUCCEEDED" {
		return nil
	}
	if original.State != "CLEANING" || plan.Retention == nil {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_CLEANING_REQUIRED")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockOrganizationCompletionSources(ctx, tx, plan); err != nil {
		return err
	}
	locked, err := repository.LockedAccountRetirement(ctx, tx, binding.Operation)
	if err != nil {
		return err
	}
	current, err := boundOrganizationClosePlan(locked, binding)
	if err != nil {
		return err
	}
	if locked.State == "SUCCEEDED" {
		return nil
	}
	if locked.State != "CLEANING" || string(original.Plan) != string(locked.Plan) {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_CHANGED")
	}
	drained, err := repository.OrganizationCloseDrained(ctx, tx, current)
	if err != nil {
````

## 第 21 步：OrganizationClosures.Complete（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 453 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!drained</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_DRAIN_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=16955:18206 -->
````go
		return err
	}
	if !drained {
		return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_DRAIN_PENDING")
	}
	if err = repository.LockAccountRetentionCompletion(ctx, tx, *current.Retention); err != nil {
		return err
	}
	for _, item := range current.Retention.Documents {
		if err = s.validateRetentionCompletion(ctx, tx, current, item); err != nil {
			return err
		}
	}
	if err = repository.CompleteAccountRetentionEntities(ctx, tx, *current.Retention); err != nil {
		return err
	}
	// 与准备期隐藏事件使用不同固定阶段号，不冲突、不创建新的正式历史。
	digest := sha256.Sum256([]byte("ORGANIZATION_RETENTION_COMPLETE\n" + binding.Operation))
	indexOperation := "retained-index-" + hex.EncodeToString(digest[:])
	documents := make([]uint64, 0, len(current.Retention.Documents))
	for _, item := range current.Retention.Documents {
		documents = append(documents, item.Document)
	}
	if err = queueVisibilityIndexEvents(ctx, tx, indexOperation, documents, 0); err != nil {
		return err
	}
	if err = repository.CompleteOrganizationClose(ctx, tx, binding.Operation); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("ACCOUNT_RETENTION_COMPLETE_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 22 步：lockOrganizationCompletionSources

插入位置：第 21 步代码之后，紧接追加；完成稿约第 488 行。

来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。

本步声明：`func lockOrganizationCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, sources&#91;start:min(start+100, len(sources))&#93;)</code>；<code>repository.OrganizationCloseGateMatches(ctx, tx, plan.Binding)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockSourceGatesBatch(ctx, tx, sources&#91;start:min(start+100, len(sources))&#93;)</code>；<code>repository.OrganizationCloseGateMatches(ctx, tx, plan.Binding)</code> |
| <code>plan</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>在此声明中的实际使用：<code>plan.Binding</code>；<code>plan.Retention</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return strings.Compare(a.Key, b.Key)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!matches</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=18206:19281 -->
````go
// 来源锁先于lifecycle。此处只取得锁、不提前拒绝更高代次：并发原号可能已
// 提交并重开公开来源，必须先读原SUCCEEDED才能判断，避免把成功重放误判冲突。
func lockOrganizationCompletionSources(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) error {
	sources := []domain.LeaseSource{{Key: plan.Binding.Scope(), Epoch: domain.TextID(plan.Binding.Epoch)}}
	for _, scope := range plan.Retention.Scopes {
		sources = append(sources, domain.LeaseSource{Key: scope.Key, Epoch: domain.TextID(scope.Epoch)})
	}
	slices.SortFunc(sources, func(a, b domain.LeaseSource) int { return strings.Compare(a.Key, b.Key) })
	for start := 0; start < len(sources); start += 100 {
		if _, err := repository.LockSourceGatesBatch(ctx, tx, sources[start:min(start+100, len(sources))]); err != nil {
			return err
		}
	}
	matches, err := repository.OrganizationCloseGateMatches(ctx, tx, plan.Binding)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("ACCOUNT_RETIREMENT_SOURCE_CHANGED")
	}
	return nil
}

````

## 第 23 步：OrganizationClosures.validateRetentionCompletion

插入位置：第 22 步代码之后，紧接追加；完成稿约第 511 行。

本步声明：`func (s *OrganizationClosures) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, tx, request)</code>；<code>repository.PublicationFactsRow(ctx, tx, item.Document, false)</code>；<code>s.retainedDependencies(ctx, tx, doc, actual)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, tx, request)</code>；<code>repository.PublicationFactsRow(ctx, tx, item.Document, false)</code>；<code>s.retainedDependencies(ctx, tx, doc, actual)</code> |
| <code>plan</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>在此声明中的实际使用：<code>plan.CleanupDocument</code>；<code>repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>plan.Binding</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes)</code>；<code>item.Release</code>；<code>item.Manifest</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_DOCUMENT_CLEANUP_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>scope.Key == &quot;doc-acl:&quot;+request.Document &amp;&amp; proof != platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>item.Release == 0</code>。
- 检查 <code>doc.PublicationState != &quot;UNPUBLISHED&quot; &#124;&#124; doc.CurrentReleaseId != nil &#124;&#124; doc.CurrentManifestId != &quot;&quot; &#124;&#124; doc.CurrentStructureId != &quot;&quot; &#124;&#124; doc.CurrentStyleId != &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>policy != &quot;PUBLIC&quot; &#124;&#124; doc.CurrentReleaseId == nil &#124;&#124; *doc.CurrentReleaseId != domain.TextID(item.Release) &#124;&#124; doc.CurrentManifestId != domain.TextID(item.Manifest) &#124;&#124; doc.CurrentStr …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PUBLIC_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=19281:21240 -->
````go
func (s *OrganizationClosures) validateRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument) error {
	_, scopes, err := plan.CleanupDocument(item.Document)
	if err != nil {
		return err
	}
	if err = repository.LockOrganizationCleanupDocument(ctx, tx, plan, item, scopes); err != nil {
		return err
	}
	request := platform.AccountDocumentCleanupRequest{Operation: plan.Binding.Operation, Document: domain.TextID(item.Document)}
	proof, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request)
	if err != nil {
		return err
	}
	if !found {
		return domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_CLEANUP_PENDING")
	}
	for _, scope := range scopes {
		if scope.Key == "doc-acl:"+request.Document && proof != platform.NewAccountDocumentCleanupContext(request, scope.Epoch, scope.Operation) {
			return errors.New("ACCOUNT_RETENTION_CLEANUP_BINDING_CHANGED")
		}
	}
	facts, err := repository.PublicationFactsRow(ctx, tx, item.Document, false)
	if err != nil {
		return err
	}
	doc := facts.Document
	if item.Release == 0 {
		if doc.PublicationState != "UNPUBLISHED" || doc.CurrentReleaseId != nil || doc.CurrentManifestId != "" || doc.CurrentStructureId != "" || doc.CurrentStyleId != "" {
			return errors.New("ACCOUNT_RETENTION_PRIVATE_CONTENT_REMAINS")
		}
		return nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return err
	}
	if policy != "PUBLIC" || doc.CurrentReleaseId == nil || *doc.CurrentReleaseId != domain.TextID(item.Release) || doc.CurrentManifestId != domain.TextID(item.Manifest) || doc.CurrentStructureId != domain.TextID(item.Structure) || doc.CurrentStyleId != domain.TextID(item.Style) {
		return errors.New("ACCOUNT_RETENTION_PUBLIC_BINDING_CHANGED")
	}
	actual := item
	actual.Versions, actual.Resources = []uint64{}, []uint64{}
	actual, err = s.retainedDependencies(ctx, tx, doc, actual)
	if err != nil {
		return err
	}
````

## 第 24 步：OrganizationClosures.validateRetentionCompletion（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 556 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>actual.Manifest != item.Manifest &#124;&#124; actual.Structure != item.Structure &#124;&#124; actual.Style != item.Style &#124;&#124; !slices.Equal(actual.Versions, item.Versions) &#124;&#124; !slices.Equal(actual.Resources, item.Resources)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_DEPENDENCIES_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=21240:21606 -->
````go
	if actual.Manifest != item.Manifest || actual.Structure != item.Structure || actual.Style != item.Style || !slices.Equal(actual.Versions, item.Versions) || !slices.Equal(actual.Resources, item.Resources) {
		return errors.New("ACCOUNT_RETENTION_DEPENDENCIES_CHANGED")
	}
	return nil
}

// organization_close_history.go：保留原实现，按业务合并文件。

````

## 第 25 步：OrganizationClosures.CleanHistory

插入位置：第 24 步代码之后，紧接追加；完成稿约第 564 行。

此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。

本步声明：`func (s *OrganizationClosures) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountHistoryCleanupReceipt(ctx, s.db, request)</code>；<code>s.ReleaseDraftResources(ctx, request)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountHistoryCleanupReceipt(ctx, s.db, request)</code>；<code>s.ReleaseDraftResources(ctx, request)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_NODE_BINDING_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_HISTORY_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=21606:23353 -->
````go
// 此步骤移除关系型历史，不把它当作所有操作正文副本/公开留存的最终完成。
// 原Node及草稿资源回执在锁外取得；删除历史与对应版本引用、本地回执同事务。
func (s *OrganizationClosures) CleanHistory(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	expected, err := s.ReleaseDraftResources(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_HISTORY_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountHistoryCleanupReceipt(ctx, tx, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
````

## 第 26 步：OrganizationClosures.CleanHistory（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 609 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=23353:24296 -->
````go
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	graph, err := repository.AccountHistoryGraphRow(ctx, tx, item)
	if err != nil {
		return empty, err
	}
	if err = repository.RemoveAccountHistoryDependents(ctx, tx, document); err != nil {
		return empty, err
	}
	if err = repository.RemoveAccountHistoryGraph(ctx, tx, item, graph); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_HISTORY_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_HISTORY_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// organization_close_payloads.go：保留原实现，按业务合并文件。

````

## 第 27 步：OrganizationClosures.CleanPayloads

插入位置：第 26 步代码之后，紧接追加；完成稿约第 641 行。

原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。

本步声明：`func (s *OrganizationClosures) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, s.db, request)</code>；<code>s.CleanHistory(ctx, request)</code>；<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountPayloadCleanupReceipt(ctx, s.db, request)</code>；<code>s.CleanHistory(ctx, request)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid()</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=24296:26011 -->
````go
// 原history回执在锁外取得，正文副本清除和本地回执同事务；未知提交不能重开
// 原操作或伪造安装成功，必须原号查询。公开留存启用需读取这里的真实终态。
func (s *OrganizationClosures) CleanPayloads(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANUP_REQUEST_INVALID")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	expected, err := s.CleanHistory(ctx, request)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_PAYLOAD_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountPayloadCleanupReceipt(ctx, tx, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	if err = repository.RemoveAccountPayloads(ctx, tx, authority); err != nil {
		return empty, err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return empty, err
	}
````

## 第 28 步：OrganizationClosures.CleanPayloads（接着实现）

插入位置：第 27 步代码之后，紧接追加；完成稿约第 687 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=26011:26395 -->
````go
	if err = repository.InsertSubmissionTask(ctx, tx, "ACCOUNT_PAYLOAD_CLEANUP", request.Document, request.Operation, raw, "SUCCEEDED"); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_PAYLOAD_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}

// organization_close_progress.go：保留原实现，按业务合并文件。

````

## 第 29 步：OrganizationClosures.Progress

插入位置：第 28 步代码之后，紧接追加；完成稿约第 698 行。

分页只读原固定文档集合；第一次读取先核验真实Doc/KB来源ACK并进入CLEANING。
尚未排空立即返回待重试，不等待网络；SUCCEEDED来自同一Doc完成事务。

本步声明：`func (s *OrganizationClosures) Progress(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.ensureCleaning(ctx, binding)</code>；<code>repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)</code> |
| <code>request</code> | <code>platform.OrganizationClosurePageRequest</code> | 使用 <code>platform.OrganizationClosurePageRequest</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>request.Binding</code>；<code>request.PlanDigest</code>；<code>request.AfterDocument</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.OrganizationClosurePage</code> | 使用 <code>platform.OrganizationClosurePage</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!binding.Valid()</code>。条件成立时执行 <code>return result, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>plan.Retention == nil &#124;&#124; (original.State != &quot;PLANNED&quot; &amp;&amp; original.State != &quot;CLEANING&quot; &amp;&amp; original.State != &quot;SUCCEEDED&quot;)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>request.PlanDigest != &quot;&quot; &amp;&amp; request.PlanDigest != result.PlanDigest</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>request.AfterDocument != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; request.PlanDigest == &quot;&quot;</code>。条件成立时执行 <code>return result, domain.Violation(domain.InvalidRequest, &quot;ACCOUNT_RETENTION_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_PAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>item.Document &lt;= after</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 29 bytes=26395:28474 -->
````go
// 分页只读原固定文档集合；第一次读取先核验真实Doc/KB来源ACK并进入CLEANING。
// 尚未排空立即返回待重试，不等待网络；SUCCEEDED来自同一Doc完成事务。
func (s *OrganizationClosures) Progress(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error) {
	result := platform.OrganizationClosurePage{Documents: []platform.AccountClosureDocument{}}
	binding := request.Binding
	if !binding.Valid() {
		return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_BINDING_INVALID")
	}
	if err := s.ensureCleaning(ctx, binding); err != nil {
		return result, err
	}
	original, err := repository.AccountRetirementSnapshot(ctx, s.db, binding.Operation)
	if err != nil {
		return result, err
	}
	plan, err := boundOrganizationClosePlan(original, binding)
	if err != nil {
		return result, err
	}
	if plan.Retention == nil || (original.State != "PLANNED" && original.State != "CLEANING" && original.State != "SUCCEEDED") {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	digest := sha256.Sum256(original.Plan)
	result.PlanDigest = hex.EncodeToString(digest[:])
	if request.PlanDigest != "" && request.PlanDigest != result.PlanDigest {
		return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
	}
	var after uint64
	if request.AfterDocument != "" {
		after, err = domain.CanonicalID(request.AfterDocument)
		if err != nil || request.PlanDigest == "" {
			return result, domain.Violation(domain.InvalidRequest, "ACCOUNT_RETENTION_CURSOR_INVALID")
		}
		found := false
		for _, item := range plan.Retention.Documents {
			found = found || item.Document == after
		}
		if !found {
			return result, domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_PAGE_CHANGED")
		}
	}
	result.Binding, result.State, result.NextAfter, result.Done = request.Binding, original.State, request.AfterDocument, true
	for _, item := range plan.Retention.Documents {
		if item.Document <= after {
			continue
		}
````

## 第 30 步：OrganizationClosures.Progress（接着实现）

插入位置：第 29 步代码之后，紧接追加；完成稿约第 744 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>len(result.Documents) == 100</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=28474:28856 -->
````go
		if len(result.Documents) == 100 {
			result.Done = false
			break
		}
		result.Documents = append(result.Documents, platform.AccountClosureDocument{Document: domain.TextID(item.Document), Release: domain.TextID(item.Release)})
		result.NextAfter = domain.TextID(item.Document)
	}
	return result, nil
}

// organization_close_ready.go：保留原实现，按业务合并文件。

````

## 第 31 步：OrganizationClosures.Ready

插入位置：第 30 步代码之后，紧接追加；完成稿约第 756 行。

本步声明：`func (s *OrganizationClosures) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.OrganizationCloseReady(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED&quot;)</code>；<code>repository.OrganizationCloseReady(ctx, s.db)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.authority == nil &#124;&#124; s.validator == nil &#124;&#124; s.cleaner == nil</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return repository.OrganizationCloseReady(ctx, s.db)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=28856:29193 -->
````go
func (s *OrganizationClosures) Ready(ctx context.Context) error {
	if s.authority == nil || s.validator == nil || s.cleaner == nil {
		return errors.New("ORGANIZATION_CLOSE_DEPENDENCIES_REQUIRED")
	}
	return repository.OrganizationCloseReady(ctx, s.db)
}

// organization_close_resources.go：保留原实现，按业务合并文件。

````

## 第 32 步：OrganizationClosures.ReleaseDraftResources

插入位置：第 31 步代码之后，紧接追加；完成稿约第 765 行。

ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。
底层ACCOUNT_*清理任务是单文档引擎协议；资格仍只来自ORGANIZATION_CLOSE计划。

本步声明：`func (s *OrganizationClosures) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountResourceCleanupReceipt(ctx, s.db, request)</code>；<code>s.DocumentCleanupContext(ctx, request)</code>；<code>s.cleaner.Clean(ctx, expected)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Valid</code>；<code>s.cleanupExpected(ctx, request)</code>；<code>repository.AccountResourceCleanupReceipt(ctx, s.db, request)</code>；<code>s.DocumentCleanupContext(ctx, request)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>empty</code>；<code>prior</code>；<code>authority</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_DOCUMENT_CLEANER_REQUIRED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!request.Valid() &#124;&#124; s.cleaner == nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_CLEANER_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>authority != expected</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; found</code>。
- 检查 <code>found &amp;&amp; prior != expectedPlan</code>。条件成立时执行 <code>return empty, errors.New(&quot;ORGANIZATION_DOCUMENT_RECEIPT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return prior, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。

<!-- write-step: 32 bytes=29193:31045 -->
````go
// ReleaseDraftResources先核验原计划，锁外取得Node真实清理回执，再在同一Doc事务
// 接住正式资源、释放草稿保护、保存回执。跨库成功/本库提交未知都按原号恢复。
// 底层ACCOUNT_*清理任务是单文档引擎协议；资格仍只来自ORGANIZATION_CLOSE计划。
func (s *OrganizationClosures) ReleaseDraftResources(ctx context.Context, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, error) {
	var empty platform.AccountDocumentCleanupContext
	if !request.Valid() || s.cleaner == nil {
		return empty, errors.New("ACCOUNT_DOCUMENT_CLEANER_REQUIRED")
	}
	expectedPlan, err := s.cleanupExpected(ctx, request)
	if err != nil {
		return empty, err
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, s.db, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	expected, err := s.DocumentCleanupContext(ctx, request)
	if err != nil {
		return empty, err
	}
	if err = s.cleaner.Clean(ctx, expected); err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	authority, err := s.lockedDocumentCleanupContext(ctx, tx, request)
	if err != nil {
		return empty, err
	}
	if authority != expected {
		return empty, errors.New("ACCOUNT_DOCUMENT_NODE_BINDING_CHANGED")
	}
	if prior, found, err := repository.AccountResourceCleanupReceipt(ctx, tx, request); err != nil || found {
		if found && prior != expectedPlan {
			return empty, errors.New("ORGANIZATION_DOCUMENT_RECEIPT_CHANGED")
		}
		return prior, err
	}
	record, err := repository.AccountRetirementSnapshot(ctx, tx, request.Operation)
	if err != nil {
````

## 第 33 步：OrganizationClosures.ReleaseDraftResources（接着实现）

插入位置：第 32 步代码之后，紧接追加；完成稿约第 810 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return empty, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return empty, errors.New(&quot;ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return authority, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=31045:31558 -->
````go
		return empty, err
	}
	plan, err := domain.DecodeOrganizationClosePlan(record.Plan)
	if err != nil {
		return empty, err
	}
	document, _ := domain.CanonicalID(request.Document)
	item, _, err := plan.CleanupDocument(document)
	if err != nil {
		return empty, err
	}
	if err = repository.ReleaseAccountDraftResources(ctx, tx, item, authority); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, errors.New("ACCOUNT_RESOURCE_CLEANUP_COMMIT_UNKNOWN")
	}
	return authority, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/organization_lifecycle_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
