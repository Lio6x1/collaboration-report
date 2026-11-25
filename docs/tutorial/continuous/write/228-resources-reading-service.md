# 分步编写 `services/document/internal/logic/document/resources_reading_service.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/228-resources-reading-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。

同文档上传过图片不代表任一上下文都能下载，必须证明当前阅读范围确实引用它。

写之前先按这条执行路线理解各段的连接：

1. context/publicContext计算来源及复核闭包
2. branchCommentContains/mainCommentContains核历史讨论原版本
3. Open锁资源并登记输出后开验证文件
4. ResourceDownload.Close释放文件和锁。

本文件需要守住的边界：

- 旧房间引用不能授权新epoch
- 公开资源随当前release和受众复核
- 失败应释放读锁与输出登记。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/resources_reading_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/resources_reading_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/resources_reading_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/resources_reading_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:606 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	repository "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
)

// resource_capacity.go：保留原实现，按业务合并文件。

````

## 第 2 步：Resources.InitializeQuotas

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (s *Resources) InitializeQuotas(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InitializeQuotaAccounts(ctx, s.db, s.limits.SpaceBytes)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>repository.InitializeQuotaAccounts(ctx, s.db, s.limits.SpaceBytes)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.InitializeQuotaAccounts(ctx, s.db, s.limits.SpaceBytes)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=606:827 -->
````go
func (s *Resources) InitializeQuotas(ctx context.Context) error {
	return repository.InitializeQuotaAccounts(ctx, s.db, s.limits.SpaceBytes)
}

// resource_comment_context.go：保留原实现，按业务合并文件。

````

## 第 3 步：ResourceReading.branchCommentContains

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (s *ResourceReading) branchCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.BranchCommentResourceVersions(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>comments.ContextVersion(ctx, tx, target.Document, &quot;BRANCH&quot;, target.Context, version)</code>；<code>comments.BranchScope(ctx, tx, target.Document, target.Context)</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.BranchCommentResourceVersions(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>comments.ContextVersion(ctx, tx, target.Document, &quot;BRANCH&quot;, target.Context, version)</code>；<code>comments.BranchScope(ctx, tx, target.Document, target.Context)</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>target</code> | <code>ResourceTarget</code> | 在此声明中的实际使用：<code>target.Document</code>；<code>target.Context</code>；<code>target.Resource</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(versions) == 0</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>bound</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 检查 <code>!bound</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>access.Permissions.Comment &#124;&#124; access.Permissions.Manage &#124;&#124; access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 此处通过 <code>return historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=827:1962 -->
````go
func (s *ResourceReading) branchCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error) {
	versions, err := repository.BranchCommentResourceVersions(ctx, tx, target.Document, target.Context, target.Resource)
	if err != nil || len(versions) == 0 {
		return false, err
	}
	// 与持久批注读取保持相同的真实分支归属证明，不依赖可变草稿或当前谱系。
	bound := false
	for _, version := range versions {
		bound, err = comments.ContextVersion(ctx, tx, target.Document, "BRANCH", target.Context, version)
		if err != nil {
			return false, err
		}
		if bound {
			break
		}
	}
	if !bound {
		return false, nil
	}
	unit, _, err := comments.BranchScope(ctx, tx, target.Document, target.Context)
	if err != nil {
		return false, err
	}
	// COMMENT 可读取已保存讨论的固定原版本，但不能据此访问任何当前房间草稿。
	if access.Permissions.Comment || access.Permissions.Manage || access.Permissions.Edit[unit] {
		return true, nil
	}
	return historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)
}

````

## 第 4 步：ResourceReading.mainCommentContains

插入位置：第 3 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func (s *ResourceReading) mainCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MainCommentResourceUnits(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MainCommentResourceUnits(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code>；<code>historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)</code> |
| <code>target</code> | <code>ResourceTarget</code> | 在此声明中的实际使用：<code>target.Document</code>；<code>target.Context</code>；<code>target.Resource</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>access.Permissions.Comment &#124;&#124; access.Permissions.Manage &#124;&#124; access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>covered</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 此处通过 <code>return false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1962:2688 -->
````go
func (s *ResourceReading) mainCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error) {
	units, err := repository.MainCommentResourceUnits(ctx, tx, target.Document, target.Context, target.Resource)
	if err != nil {
		return false, err
	}
	for _, unit := range units {
		if access.Permissions.Comment || access.Permissions.Manage || access.Permissions.Edit[unit] {
			return true, nil
		}
		covered, err := historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)
		if err != nil {
			return false, err
		}
		if covered {
			return true, nil
		}
	}
	return false, nil
}

// resource_context.go：保留原实现，按业务合并文件。

````

## 第 5 步：resourceContextCheck

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`type resourceContextCheck func(context.Context, *sql.Tx) (resourceReadFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。 |
| <code>(匿名/未命名)</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>resourceReadFacts</code> | 经原事务核验的资源空间、来源代次与正式版事实；之后登记输出/读取租约仍用这份绑定。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。 |

<!-- write-step: 5 bytes=2688:2773 -->
````go
type resourceContextCheck func(context.Context, *sql.Tx) (resourceReadFacts, error)

````

## 第 6 步：ResourceReading.context

插入位置：第 5 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`func (s *ResourceReading) context(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.publicContext(ctx, actor, target)</code>；<code>s.resources.access.Authorize(ctx, *actor, target.Document, &quot;READ_RESOURCE&quot;)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.publicContext(ctx, actor, target)</code>；<code>actor == nil</code>；<code>s.resources.access.Authorize(ctx, *actor, target.Document, &quot;READ_RESOURCE&quot;)</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>ResourceTarget</code> | 在此声明中的实际使用：<code>target.Resource</code>；<code>target.Document</code>；<code>target.Context</code>；<code>target.Kind</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 由 <code>s.publicContext(ctx, actor, target)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>resourceContextCheck</code> | 由 <code>s.publicContext(ctx, actor, target)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>check</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.publicContext(ctx, actor, target)</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;RESOURCE_CONTEXT_INVALID&quot;)</code>；<code>domain.Violation(domain.SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>target.Resource == 0 &#124;&#124; target.Document == 0 &#124;&#124; target.Context == 0</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;RESOURCE_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>target.Kind == &quot;PUBLIC&quot;</code>。条件成立时执行 <code>return s.publicContext(ctx, actor, target)</code>。当前函数在这里结束。
- 检查 <code>target.Kind != &quot;MAIN&quot; &amp;&amp; target.Kind != &quot;BRANCH&quot; &amp;&amp; target.Kind != &quot;REVIEW&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;RESOURCE_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>actor == nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.SessionRequired, &quot;SESSION_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>doc.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return facts, domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; allowed</code>。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; !allowed</code>。

<!-- write-step: 6 bytes=2773:4790 -->
````go
func (s *ResourceReading) context(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error) {
	if target.Resource == 0 || target.Document == 0 || target.Context == 0 {
		return nil, nil, domain.Violation(domain.InvalidRequest, "RESOURCE_CONTEXT_INVALID")
	}
	if target.Kind == "PUBLIC" {
		return s.publicContext(ctx, actor, target)
	}
	if target.Kind != "MAIN" && target.Kind != "BRANCH" && target.Kind != "REVIEW" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "RESOURCE_CONTEXT_INVALID")
	}
	if actor == nil {
		return nil, nil, domain.Violation(domain.SessionRequired, "SESSION_REQUIRED")
	}
	access, err := s.resources.access.Authorize(ctx, *actor, target.Document, "READ_RESOURCE")
	if err != nil {
		return nil, nil, err
	}
	check := func(ctx context.Context, tx *sql.Tx) (resourceReadFacts, error) {
		var facts resourceReadFacts
		doc, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return facts, err
		}
		if doc.Lifecycle != "ACTIVE" {
			return facts, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		facts.space, _ = domain.CanonicalID(doc.SpaceId)
		allowed := false
		switch target.Kind {
		case "MAIN":
			// 当前锁定清单或本人会话的有效阅读快照，不能用任意历史manifest取得文件。
			allowed = doc.MainLocked && doc.CurrentManifestId == domain.TextID(target.Context)
			if !allowed {
				allowed, err = repository.MainManifestAccessible(ctx, tx, target.Document, target.Context, actor.ID, actor.Proof.SessionRef)
			}
			if err == nil && allowed {
				allowed, err = repository.ManifestContains(ctx, tx, target.Document, target.Context, target.Resource)
			}
			if err == nil && !allowed {
				// 阅读快照到期不删除持久讨论；只放行当前仍可读讨论原版本内的图片。
				allowed, err = s.mainCommentContains(ctx, tx, access, target)
			}
		case "BRANCH":
			branch, e := repository.BranchContext(ctx, tx, target.Document, target.Context)
			err = e
````

## 第 7 步：ResourceReading.context（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 126 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; access.Permissions.Edit&#91;branch.Unit&#93;</code>。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; !allowed &amp;&amp; branch.Room != 0</code>。
- 在本段的匿名函数内：检查 <code>!allowed &amp;&amp; (err == nil &#124;&#124; err == sql.ErrNoRows)</code>。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; !allowed</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 在本段的匿名函数内：检查 <code>err == nil &amp;&amp; !allowed</code>。
- 在本段的匿名函数内：检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return facts, domain.Violation(domain.EntityNotFound, &quot;RESOURCE_CONTEXT_NOT_FOUND&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=4790:6594 -->
````go
			if err != nil && err != sql.ErrNoRows {
				break
			}
			if err == nil && access.Permissions.Edit[branch.Unit] {
				allowed, err = repository.VersionContains(ctx, tx, target.Document, branch.BaseVersion, target.Resource)
				if err == nil && !allowed && branch.Room != 0 {
					allowed, err = repository.DraftContains(ctx, tx, target.Document, branch.Room, branch.Epoch, target.Resource)
				}
			}
			if !allowed && (err == nil || err == sql.ErrNoRows) {
				// 历史分支或草稿已移除图片时，只以可读的未删除讨论原版本补足引用。
				// 补足失败仍保留原归属缺失，不能把跨文档的不存在上下文改成普通无权。
				original := err
				allowed, err = s.branchCommentContains(ctx, tx, access, target)
				if err == nil && !allowed {
					err = original
				}
			}
		case "REVIEW":
			request, version, base, e := repository.ReviewContext(ctx, tx, target.Document, target.Context)
			err = e
			if err != nil {
				break
			}
			pointer, e := documents.ReviewPointerRow(ctx, tx, request)
			err = e
			if err != nil {
				break
			}
			if err = s.reviews.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
				break
			}
			allowed, err = repository.VersionContains(ctx, tx, target.Document, version, target.Resource)
			if err == nil && !allowed {
				allowed, err = repository.VersionContains(ctx, tx, target.Document, base, target.Resource)
			}
			if err == nil && !allowed {
				// 同一审核轮次可读取的原冲突输入也属于其固定上下文，
				// 但必须仍有有效原冲突收据，不能借现在的主线引用放行。
				allowed, err = s.conflictContains(ctx, tx, target)
			}
		}
		if err == sql.ErrNoRows {
			return facts, domain.Violation(domain.EntityNotFound, "RESOURCE_CONTEXT_NOT_FOUND")
		}
````

## 第 8 步：ResourceReading.context（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 171 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return facts, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。条件成立时执行 <code>return facts, domain.Violation(domain.PermissionDenied, &quot;RESOURCE_REFERENCE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, check, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6594:6808 -->
````go
		if err != nil {
			return facts, err
		}
		if !allowed {
			return facts, domain.Violation(domain.PermissionDenied, "RESOURCE_REFERENCE_REQUIRED")
		}
		return facts, nil
	}
	return access.Sources, check, nil
}

````

## 第 9 步：ResourceReading.publicContext

插入位置：第 8 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func (s *ResourceReading) publicContext(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.public.facts(ctx, target.Document)</code>；<code>s.public.audience(ctx, actor, facts, policy)</code> |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.public.audience(ctx, actor, facts, policy)</code> |
| <code>target</code> | <code>ResourceTarget</code> | 在此声明中的实际使用：<code>target.Document</code>；<code>target.Context</code>；<code>target.Resource</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>resourceContextCheck</code> | 实际返回表达式：<code>nil</code>；<code>check</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PUBLIC_RESOURCE_NOT_CURRENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!domain.HasCurrentPublication(facts, policy) &#124;&#124; facts.Document.CurrentReleaseId == nil &#124;&#124; *facts.Document.CurrentReleaseId != domain.TextID(target.Context)</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;PUBLIC_RESOURCE_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>publicFactsBinding(current) != publicFactsBinding(facts) &#124;&#124; !domain.HasCurrentPublication(current, policy)</code>。条件成立时执行 <code>return result, domain.Violation(domain.StateConflict, &quot;PUBLIC_DOCUMENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。条件成立时执行 <code>return result, domain.Violation(domain.PermissionDenied, &quot;RESOURCE_REFERENCE_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6808:8614 -->
````go
func (s *ResourceReading) publicContext(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error) {
	facts, err := s.public.facts(ctx, target.Document)
	if err != nil {
		return nil, nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, nil, err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(target.Context) {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PUBLIC_RESOURCE_NOT_CURRENT")
	}
	_, sources, err := s.public.audience(ctx, actor, facts, policy)
	if err != nil {
		return nil, nil, err
	}
	check := func(ctx context.Context, tx *sql.Tx) (resourceReadFacts, error) {
		var result resourceReadFacts
		current, err := documents.PublicationFactsRow(ctx, tx, target.Document, true)
		if err != nil {
			return result, err
		}
		if publicFactsBinding(current) != publicFactsBinding(facts) || !domain.HasCurrentPublication(current, policy) {
			return result, domain.Violation(domain.StateConflict, "PUBLIC_DOCUMENT_CHANGED")
		}
		fixed, err := documents.FixedReleaseRow(ctx, tx, target.Document, target.Context)
		if err != nil {
			return result, err
		}
		manifest, err := domain.CanonicalID(fixed.Release.ManifestId)
		if err != nil {
			return result, err
		}
		allowed, err := repository.ManifestContains(ctx, tx, target.Document, manifest, target.Resource)
		if err != nil {
			return result, err
		}
		if !allowed {
			return result, domain.Violation(domain.PermissionDenied, "RESOURCE_REFERENCE_REQUIRED")
		}
		result.space, _ = domain.CanonicalID(current.Space.Id)
		result.epoch, _ = domain.CanonicalID(current.Document.PublicationEpoch)
		result.release = target.Context
		return result, nil
	}
````

## 第 10 步：ResourceReading.publicContext（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 227 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return sources, check, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=8614:8712 -->
````go
	return sources, check, nil
}

// resource_quota.go：保留原实现，按业务合并文件。

````

## 第 11 步：Resources.PrepareQuota

插入位置：第 10 步代码之后，紧接追加；完成稿约第 232 行。

本步声明：`func (s *Resources) PrepareQuota(ctx context.Context, actor Actor, space uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_RESOURCE_QUOTA&quot;, 0)</code>；<code>s.access.identity.Revalidate(ctx, proof)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_RESOURCE_QUOTA&quot;, 0)</code>；<code>actor.Proof</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, &quot;READ_RESOURCE_QUOTA&quot;, 0)</code>；<code>documents.SpaceRow(ctx, tx, space, true)</code>；<code>repository.LockQuota(ctx, tx, space, s.limits.SpaceBytes)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return dto.Quota{SpaceId: domain.TextID(q.Space), LimitBytes: domain.TextID(q.Limit), UsedBytes: domain.TextID(q.Used), ReservedBytes: domain.TextID(q.Reserved), Revision: domain.TextID(q.Revision)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8712:9942 -->
````go
func (s *Resources) PrepareQuota(ctx context.Context, actor Actor, space uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	_, proof, err := s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_RESOURCE_QUOTA", 0)
	if err != nil {
		return nil, nil, err
	}
	if err := s.access.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 首次读取才物化配额账户，身份来源锁由统一Read事务持有。
		// 返回账户本身的revision，上传预留必须比较同一份余额版本。
		if _, err := documents.SpaceRow(ctx, tx, space, true); err != nil {
			return nil, err
		}
		q, err := repository.LockQuota(ctx, tx, space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		return dto.Quota{SpaceId: domain.TextID(q.Space), LimitBytes: domain.TextID(q.Limit), UsedBytes: domain.TextID(q.Used), ReservedBytes: domain.TextID(q.Reserved), Revision: domain.TextID(q.Revision)}, nil
	}
	return sources, run, nil
}

// resource_reading.go：保留原实现，按业务合并文件。

````

## 第 12 步：ResourceTarget

插入位置：第 11 步代码之后，紧接追加；完成稿约第 261 行。

本步声明：`type ResourceTarget struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Resource</code> | <code>uint64</code> | 请求读取的图片/资源编号。 |
| <code>Document</code> | <code>uint64</code> | 资源读取的文档范围。 |
| <code>Context</code> | <code>uint64</code> | 对应读取类别的上下文编号（清单、轮次、支线等），不能跨类别复用。 |
| <code>Kind</code> | <code>string</code> | 资源读取上下文类别，决定验证哪条引用关系。 |

<!-- write-step: 12 bytes=9942:10046 -->
````go
type ResourceTarget struct {
	Resource, Document, Context uint64
	Kind                        string
}

````

## 第 13 步：ResourceReading

插入位置：第 12 步代码之后，紧接追加；完成稿约第 266 行。

本步声明：`type ResourceReading struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>resources</code> | <code>*Resources</code> | 使用 <code>Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。 |
| <code>public</code> | <code>*PublicReading</code> | 使用 <code>PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。 |
| <code>reviews</code> | <code>*Reviews</code> | 使用 <code>Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。 |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。 |

<!-- write-step: 13 bytes=10046:10167 -->
````go
type ResourceReading struct {
	resources *Resources
	public    *PublicReading
	reviews   *Reviews
	outputs   *Outputs
}

````

## 第 14 步：NewResourceReading

插入位置：第 13 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`func NewResourceReading(resources *Resources, public *PublicReading, reviews *Reviews, outputs *Outputs) *ResourceReading`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>resources</code> | <code>*Resources</code> | 使用 <code>Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>在此声明中的实际使用：<code>resources: resources</code> |
| <code>public</code> | <code>*PublicReading</code> | 使用 <code>PublicReading</code> 类型；其定义处的职责：按当前正式指针与当前受众输出公开文档、受控历史和搜索快照。<br>在此声明中的实际使用：<code>public: public</code> |
| <code>reviews</code> | <code>*Reviews</code> | 使用 <code>Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>在此声明中的实际使用：<code>reviews: reviews</code> |
| <code>outputs</code> | <code>*Outputs</code> | 使用 <code>Outputs</code> 类型；其定义处的职责：Outputs协调持久租约与真实参与者；ResponseWriter和SQL细节分别留在adapter及repository。<br>在此声明中的实际使用：<code>outputs: outputs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResourceReading</code> | 实际返回表达式：<code>&amp;ResourceReading{resources: resources, public: public, reviews: reviews, outputs: outputs}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ResourceReading{resources: resources, public: public, reviews: reviews, outputs: outputs}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10167:10393 -->
````go
func NewResourceReading(resources *Resources, public *PublicReading, reviews *Reviews, outputs *Outputs) *ResourceReading {
	return &ResourceReading{resources: resources, public: public, reviews: reviews, outputs: outputs}
}

````

## 第 15 步：ResourceDownload

插入位置：第 14 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`type ResourceDownload struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 已核验的资源编号。 |
| <code>Size</code> | <code>uint64</code> | 已核验资源字节数，用于响应及完整性判断。 |
| <code>MediaType</code> | <code>string</code> | 响应使用的经过验证的媒体类型。 |
| <code>Reader</code> | <code>io.ReadCloser</code> | 需要关闭的对象读流；读结束/取消都要释放。 |
| <code>release</code> | <code>func()</code> | 释放本次读取保护的回调。 |
| <code>once</code> | <code>sync.Once</code> | 保证Close/错误路径重复收尾也只释放一次。 |

<!-- write-step: 15 bytes=10393:10527 -->
````go
type ResourceDownload struct {
	ID, Size  uint64
	MediaType string
	Reader    io.ReadCloser
	release   func()
	once      sync.Once
}

````

## 第 16 步：ResourceDownload.Close

插入位置：第 15 步代码之后，紧接追加；完成稿约第 285 行。

本步声明：`func (d *ResourceDownload) Close()`。

<!-- write-step: 16 bytes=10527:10620 -->
````go
func (d *ResourceDownload) Close() {
	d.once.Do(func() { d.Reader.Close(); d.release() })
}

````

## 第 17 步：ResourceReading.Open

插入位置：第 16 步代码之后，紧接追加；完成稿约第 289 行。

本步声明：`func (s *ResourceReading) Open(ctx context.Context, actor *Actor, target ResourceTarget, handler string, bind BindOutput) (*ResourceDownload, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.context(ctx, actor, target)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.resources.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, preflight, sources)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>*Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.context(ctx, actor, target)</code>；<code>actor != nil</code>；<code>actor.Proof</code>；<code>InsertAdmission(ctx, tx, *actor, sources, target.Document, &quot;READ_RESOURCE&quot;, admission)</code> |
| <code>target</code> | <code>ResourceTarget</code> | 在此声明中的实际使用：<code>s.context(ctx, actor, target)</code>；<code>target.Resource</code>；<code>target.Document</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.outputs.RegisterPublication(ctx, tx, sources, target.Document, facts.epoch, facts.release, handler, edge)</code>；<code>s.outputs.Register(ctx, tx, sources, target.Document, admission, handler, edge)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ResourceDownload</code> | 实际返回表达式：<code>nil</code>；<code>download</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;RESOURCE_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; edge == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.resources.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 执行 <code>preflight.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;RESOURCE_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>object.State != &quot;READY&quot; &#124;&#124; object.Size &gt; 32&lt;&lt;20</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 登记 <code>func() { 		if !accepted { 			download.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 17 bytes=10620:12366 -->
````go
func (s *ResourceReading) Open(ctx context.Context, actor *Actor, target ResourceTarget, handler string, bind BindOutput) (*ResourceDownload, error) {
	sources, check, err := s.context(ctx, actor, target)
	if err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	// 先确认真实上下文引用再读取文件，避免无权请求触发大文件校验。
	// 该短事务不跨文件IO持锁；下面登记输出前仍须重新核对，关闭撤权窗口。
	preflight, err := s.resources.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = LockSources(ctx, preflight, sources); err == nil {
		_, err = check(ctx, preflight)
	}
	preflight.Rollback()
	if err != nil {
		return nil, err
	}
	object, err := repository.ObjectRow(ctx, s.resources.db, target.Resource, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.Violation(domain.EntityNotFound, "RESOURCE_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if object.State != "READY" || object.Size > 32<<20 {
		return nil, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
	}
	release, err := s.resources.files.AcquireRead(ctx, object.Key)
	if err != nil {
		return nil, err
	}
	// 不可变文件完整校验放在数据库事务外，共享文件锁阻止并发删除，但允许多个读者。
	file, err := s.resources.files.Open(ctx, object.Key, object.Size, object.Checksum)
	if err != nil {
		release()
		return nil, err
	}
	download := &ResourceDownload{ID: object.ID, Size: object.Size, MediaType: object.MediaType, Reader: file, release: release}
	accepted := false
	defer func() {
````

## 第 18 步：ResourceReading.Open（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 334 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>!accepted</code>。
- 检查 <code>actor != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.resources.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>current != object &#124;&#124; !allowed &#124;&#124; current.State != &quot;READY&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;RESOURCE_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>facts.release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=12366:13817 -->
````go
		if !accepted {
			download.Close()
		}
	}()
	if actor != nil {
		if err := s.resources.access.identity.Revalidate(ctx, actor.Proof); err != nil {
			return nil, err
		}
	}
	tx, err := s.resources.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	facts, err := check(ctx, tx)
	if err != nil {
		return nil, err
	}
	current, err := repository.ObjectRow(ctx, tx, target.Resource, true)
	if err != nil {
		return nil, err
	}
	allowed, err := repository.ResourceAllowedForDocument(ctx, tx, target.Resource, target.Document, facts.space)
	if err != nil {
		return nil, err
	}
	if current != object || !allowed || current.State != "READY" {
		return nil, domain.Violation(domain.StateConflict, "RESOURCE_CONTEXT_CHANGED")
	}
	var registration *ResponseRegistration
	if facts.release != 0 {
		registration, err = s.outputs.RegisterPublication(ctx, tx, sources, target.Document, facts.epoch, facts.release, handler, edge)
	} else {
		// 私有文档输出必须有真实准入记录；资源上传者或裸文档ID不能代替准入。
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, *actor, sources, target.Document, "READ_RESOURCE", admission); err != nil {
			return nil, err
		}
		registration, err = s.outputs.Register(ctx, tx, sources, target.Document, admission, handler, edge)
	}
	if err != nil {
		return nil, err
	}
````

## 第 19 步：ResourceReading.Open（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 380 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return download, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=13817:13973 -->
````go
	output := bind(registration, sources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	accepted = true
	return download, nil
}

````

## 第 20 步：resourceReadFacts

插入位置：第 19 步代码之后，紧接追加；完成稿约第 389 行。

本步声明：`type resourceReadFacts struct{ space, epoch, release uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>space</code> | <code>uint64</code> | 资源读取记账/归属空间编号。 |
| <code>epoch</code> | <code>uint64</code> | 资源读取对应的权限或发布来源代次，用于后续绑定。 |
| <code>release</code> | <code>uint64</code> | 读取正式版资源时对应的正式版编号。 |

<!-- write-step: 20 bytes=13973:14035 -->
````go
type resourceReadFacts struct{ space, epoch, release uint64 }
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/resources_reading_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
