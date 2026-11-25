# 分步编写 `services/document/internal/repository/document/account_cleanup_queries.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/140-account-cleanup-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按固定账号计划清理关系、历史依赖与草稿资源，并保存原清理回执。

用户回执成功仍可能有Node安装UNKNOWN，必须等所有正文driver收尾才能删依赖。

写之前先按这条执行路线理解各段的连接：

1. AccountDocumentWritesPending沿真实关联检查任务
2. LockAccountCleanupDocument核父级
3. RemoveAccountHistoryGraph按叶删除
4. ReleaseAccountDraftResources先建立正式留存pin再释放草稿pin。

本文件需要守住的边界：

- 不能关闭FOREIGN_KEY_CHECKS
- 不能删除全部PURGE任务来绕过待处理判断
- 父对象缺失不是成功证明。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/account_cleanup_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/account_cleanup_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/account_cleanup_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/account_cleanup_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:419 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	allocations "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：LockAccountCleanupDocument

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。

本步声明：`func LockAccountCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch 		FROM document WHERE id=? FOR UPDATE`, item.Document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(account_owner_id,0),COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Binding</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>item.Space</code>；<code>item.KnowledgeBase</code> |
| <code>scopes</code> | <code>&#91;&#93;domain.AccountRetentionScope</code> | 使用 <code>domain.AccountRetentionScope</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_METADATA_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_PARENT_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_DOCUMENT_SPACE_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch 		FROM document WHERE id=? FOR UPDATE`, item.Document).Scan(&amp;space, &amp;kb, &amp;state, &amp;acl, &amp;publication)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch 		FROM document WHERE id=? FOR UPDATE`, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>space != item.Space &#124;&#124; kb != item.KnowledgeBase &#124;&#124; state != &quot;CLOSING&quot; &#124;&#124; acl != epochs&#91;&quot;doc-acl:&quot;+domain.TextID(item.Document)&#93; &#124;&#124; publication != epochs&#91;&quot;publication:&quot;+domain.TextID(item.Document)&#93;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_METADATA_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>kb != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb).Scan(&amp;parent, &amp;state, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>。
- 检查 <code>parent != space &#124;&#124; state != &quot;CLOSING&quot; &#124;&#124; revision != epochs&#91;&quot;kb-acl:&quot;+domain.TextID(kb)&#93;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(account_owner_id,0),COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space).Scan(&amp;kind, &amp;state, &amp;owner, &amp;organization)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(account_owner_id,0),COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space)</code>。
- 检查 <code>state != &quot;CLOSING&quot; &#124;&#124; (!owned &amp;&amp; !closingOrganization)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=419:2476 -->
````go
// 来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
// 不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。
func LockAccountCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error {
	epochs := map[string]uint64{}
	for _, scope := range scopes {
		epochs[scope.Key] = scope.Epoch
	}
	var space, kb, acl, publication uint64
	var state string
	err := tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch
		FROM document WHERE id=? FOR UPDATE`, item.Document).Scan(&space, &kb, &state, &acl, &publication)
	if err != nil {
		return err
	}
	if space != item.Space || kb != item.KnowledgeBase || state != "CLOSING" || acl != epochs["doc-acl:"+domain.TextID(item.Document)] || publication != epochs["publication:"+domain.TextID(item.Document)] {
		return errors.New("ACCOUNT_DOCUMENT_METADATA_CHANGED")
	}
	if kb != 0 {
		var parent, revision uint64
		if err = tx.QueryRowContext(ctx, "SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE", kb).Scan(&parent, &state, &revision); err != nil {
			return err
		}
		if parent != space || state != "CLOSING" || revision != epochs["kb-acl:"+domain.TextID(kb)] {
			return errors.New("ACCOUNT_DOCUMENT_PARENT_CHANGED")
		}
	}
	var kind string
	var owner, organization uint64
	if err = tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(account_owner_id,0),COALESCE(organization_id,0)
		FROM space WHERE id=? FOR UPDATE`, space).Scan(&kind, &state, &owner, &organization); err != nil {
		return err
	}
	owned := kind == "PERSONAL" && owner == plan.Binding.User
	closingOrganization := kind == "ORGANIZATION" && slices.Contains(plan.Binding.ClosingOrganizations, organization)
	if state != "CLOSING" || (!owned && !closingOrganization) {
		return errors.New("ACCOUNT_DOCUMENT_SPACE_CHANGED")
	}
	return nil
}

````

## 第 3 步：AccountDocumentWritesPending

插入位置：第 2 步代码之后，紧接追加；完成稿约第 57 行。

单文档资格以原持久关联定位任务：两类非文档路由（MERGE、CONFLICT_RESOLUTION）
用原admission/业务记录；payload保留历史大小写。任何非终态driver都等待原恢复器，
包括用户receipt已SUCCEEDED而Node安装仍UNKNOWN的RECONCILE，绝不使用TTL或lease。

本步声明：`func AccountDocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return DocumentWritesPending(ctx, tx, document, &quot;&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2476:2948 -->
````go
// 单文档资格以原持久关联定位任务：两类非文档路由（MERGE、CONFLICT_RESOLUTION）
// 用原admission/业务记录；payload保留历史大小写。任何非终态driver都等待原恢复器，
// 包括用户receipt已SUCCEEDED而Node安装仍UNKNOWN的RECONCILE，绝不使用TTL或lease。
func AccountDocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64) (bool, error) {
	return DocumentWritesPending(ctx, tx, document, "")
}

````

## 第 4 步：DocumentWritesPending

插入位置：第 3 步代码之后，紧接追加；完成稿约第 64 行。

仅排除清理流程自身的原receipt；正文driver/安装UNKNOWN及任何其他原号仍然保护。
Account沿旧入口传空串，保持原资格检查。不能把整个PURGE kind都排除掉。

本步声明：`func DocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64, excludingOperation string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN submission_operation s ON s.oper …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM submission_operation WHERE document_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;)) 		OR EXISTS(SELECT 1 FROM merge_operation …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code>；<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN submission_operation s ON s.oper …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>excludingOperation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>pending</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN submission_operation s ON s.oper …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM durable_task t 		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id 		LEFT JOIN submission_operation s ON s.oper …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil &#124;&#124; pending</code>。条件成立时执行 <code>return pending, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil &#124;&#124; pending</code>。条件成立时执行 <code>return pending, err</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=2948:6113 -->
````go
// 仅排除清理流程自身的原receipt；正文driver/安装UNKNOWN及任何其他原号仍然保护。
// Account沿旧入口传空串，保持原资格检查。不能把整个PURGE kind都排除掉。
func DocumentWritesPending(ctx context.Context, tx *sql.Tx, document uint64, excludingOperation string) (bool, error) {
	var pending bool
	id := domain.TextID(document)
	path := "/v1/documents/" + id + "/%"
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM durable_task t
		LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id
		LEFT JOIN submission_operation s ON s.operation_id=t.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=t.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		WHERE t.kind IN ('SUBMISSION_RECOVER','CAPTURE_FINALIZE','MERGE_RECOVER','MAIN_RECONCILE',
		'MAIN_CONTROL_RECOVER','MAIN_CONTROL_RECONCILE','PUBLICATION_RECOVER','PULL_MAIN_RECOVER','PULL_MAIN_RECONCILE',
		'STRUCTURE_PREVIEW_RECOVER','DISCARD_PREVIEW_RECOVER','DISCARD_DRAFT_RECOVER','DISCARD_DRAFT_RECONCILE',
		'REVERSE_RECOVER','REVERSE_RECONCILE','RESOLUTION_RECOVER','RESOLUTION_RECONCILE')
		AND t.state NOT IN ('SUCCEEDED','REJECTED') AND (
		 s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=t.operation_id AND a.document_id=?)
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.capture.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.fixed.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.main.context.target.document_id'))=?
		 OR JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Context.target.document_id'))=?
		))`, document, document, path, document, id, id, id, id, id).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// task可能尚未领取，或原终态提交结果未知。还要看原操作而不是仅数任务；
	// 原admission与receipt在同一事务产生，来源关闭后不会再合法创建新的写入操作。
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM operation_receipt o
		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=o.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		WHERE o.operation_id<>? AND o.state NOT IN ('SUCCEEDED','REJECTED','CANCELLED') AND (
		 s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=o.operation_id AND a.document_id=?)
		))`, excludingOperation, document, document, path, document).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// 不能把“查不到未终结任务”当成安装成功。每个已接纳正文操作必须有原恢复器
	// 的真实终态；成功送审还需Finalize，成功MAIN和所有草稿控制还需真实安装收尾。
	// MERGE早期明确拒绝可以没有Node准备；它的原RECOVER终态与两侧控制屏障一起证明。
````

## 第 5 步：DocumentWritesPending（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 109 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS( 		SELECT 1 FROM operation_receipt o 		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id 		LEFT JOIN merge_operation m ON m.o …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil &#124;&#124; pending</code>。条件成立时执行 <code>return pending, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM submission_operation WHERE document_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;)) 		OR EXISTS(SELECT 1 FROM merge_operation …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM submission_operation WHERE document_id=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;)) 		OR EXISTS(SELECT 1 FROM merge_operation …</code>（长表达式，按本步完整代码继续）。

<!-- write-step: 5 bytes=6113:9821 -->
````go
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM operation_receipt o
		LEFT JOIN submission_operation s ON s.operation_id=o.operation_id
		LEFT JOIN merge_operation m ON m.operation_id=o.operation_id
		LEFT JOIN review_request r ON r.id=m.request_id
		LEFT JOIN durable_task original ON original.business_key=o.operation_id AND original.operation_id=o.operation_id
		 AND original.kind=CASE o.kind
		 WHEN 'SUBMISSION' THEN 'SUBMISSION_RECOVER' WHEN 'MERGE' THEN 'MERGE_RECOVER'
		 WHEN 'MAIN_LOCK' THEN 'MAIN_CONTROL_RECOVER' WHEN 'STYLE_CHANGE' THEN 'MAIN_CONTROL_RECOVER'
		 WHEN 'CHAPTER_CREATE' THEN 'MAIN_CONTROL_RECOVER' WHEN 'STRUCTURE_CHANGE' THEN 'MAIN_CONTROL_RECOVER'
		 WHEN 'RELEASE_CREATE' THEN 'PUBLICATION_RECOVER' WHEN 'PULL_MAIN' THEN 'PULL_MAIN_RECOVER'
		 WHEN 'STRUCTURE_PREVIEW' THEN 'STRUCTURE_PREVIEW_RECOVER' WHEN 'DISCARD_PREVIEW' THEN 'DISCARD_PREVIEW_RECOVER'
		 WHEN 'DISCARD_DRAFT' THEN 'DISCARD_DRAFT_RECOVER' WHEN 'REVERSE' THEN 'REVERSE_RECOVER'
		 WHEN 'CONFLICT_RESOLUTION' THEN 'RESOLUTION_RECOVER' END
		LEFT JOIN durable_task installed ON installed.business_key=o.operation_id AND installed.operation_id=o.operation_id
		 AND installed.kind=CASE
		 WHEN o.kind='SUBMISSION' AND o.state='SUCCEEDED' THEN 'CAPTURE_FINALIZE'
		 WHEN o.kind='MERGE' AND o.state='SUCCEEDED' THEN 'MAIN_RECONCILE'
		 WHEN o.kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE') THEN 'MAIN_CONTROL_RECONCILE'
		 WHEN o.kind='PULL_MAIN' THEN 'PULL_MAIN_RECONCILE' WHEN o.kind='DISCARD_DRAFT' THEN 'DISCARD_DRAFT_RECONCILE'
		 WHEN o.kind='REVERSE' THEN 'REVERSE_RECONCILE' WHEN o.kind='CONFLICT_RESOLUTION' THEN 'RESOLUTION_RECONCILE' END
		WHERE o.kind IN ('SUBMISSION','MERGE','MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE',
		'RELEASE_CREATE','PULL_MAIN','STRUCTURE_PREVIEW','DISCARD_PREVIEW','DISCARD_DRAFT','REVERSE','CONFLICT_RESOLUTION')
		AND (s.document_id=? OR r.document_id=? OR JSON_UNQUOTE(JSON_EXTRACT(o.target_json,'$.path')) LIKE ?
		 OR EXISTS(SELECT 1 FROM document_admission a WHERE a.operation_id=o.operation_id AND a.document_id=?))
		AND (original.id IS NULL OR original.state NOT IN ('SUCCEEDED','REJECTED') OR
		 ((o.kind IN ('MAIN_LOCK','STYLE_CHANGE','CHAPTER_CREATE','STRUCTURE_CHANGE','PULL_MAIN','DISCARD_DRAFT','REVERSE','CONFLICT_RESOLUTION')
		 OR (o.kind IN ('SUBMISSION','MERGE') AND o.state='SUCCEEDED')) AND (installed.id IS NULL OR installed.state<>'SUCCEEDED')))
		)`, document, document, path, document).Scan(&pending)
	if err != nil || pending {
		return pending, err
	}
	// 真实业务记录和控制占位仍未释放时，即使某条任务误终结/丢失也不能发证。
	// BRANCH_CONTROL_GATE本身一直是SUCCEEDED，必须看其payload中的原操作号。
	err = tx.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM submission_operation WHERE document_id=? AND state NOT IN ('SUCCEEDED','REJECTED'))
		OR EXISTS(SELECT 1 FROM merge_operation m JOIN review_request r ON r.id=m.request_id
		 WHERE r.document_id=? AND m.state NOT IN ('SUCCEEDED','REJECTED','CANCELLED'))
		OR EXISTS(SELECT 1 FROM main_commit_barrier WHERE document_id=? AND (phase<>'OPEN' OR operation_id IS NOT NULL))
		OR EXISTS(SELECT 1 FROM branch WHERE document_id=? AND lifecycle='RESTORING')
		OR EXISTS(SELECT 1 FROM durable_task t JOIN branch b ON t.business_key=CAST(b.id AS CHAR)
		 WHERE t.kind='BRANCH_CONTROL_GATE' AND b.document_id=?
		 AND (t.state<>'SUCCEEDED' OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.operation_id')),'')<>''))`,
		document, document, document, document, document).Scan(&pending)
	return pending, err
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

````

## 第 6 步：AccountHistoryCleanupReceipt

插入位置：第 5 步代码之后，紧接追加；完成稿约第 158 行。

本步声明：`func AccountHistoryCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_HISTORY_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Document</code>；<code>request.Operation</code>；<code>result.AccountDocumentCleanupRequest != request</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_CLEANUP_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_HISTORY_CLEANUP&#x27; AND business_key=?&quot;, request.Document).Scan(&amp;operation, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_HISTORY_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, false, err</code>。当前函数在这里结束。
- 检查 <code>operation != request.Operation &#124;&#124; state != &quot;SUCCEEDED&quot; &#124;&#124; json.Unmarshal(raw, &amp;result) != nil &#124;&#124; !result.Valid() &#124;&#124; result.AccountDocumentCleanupRequest != request</code>。条件成立时执行 <code>return result, false, errors.New(&quot;ACCOUNT_HISTORY_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=9821:10673 -->
````go
func AccountHistoryCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var result platform.AccountDocumentCleanupContext
	var operation, state string
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_HISTORY_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if operation != request.Operation || state != "SUCCEEDED" || json.Unmarshal(raw, &result) != nil || !result.Valid() || result.AccountDocumentCleanupRequest != request {
		return result, false, errors.New("ACCOUNT_HISTORY_CLEANUP_BINDING_CHANGED")
	}
	return result, true, nil
}

````

## 第 7 步：RemoveAccountHistoryDependents

插入位置：第 6 步代码之后，紧接追加；完成稿约第 175 行。

只改可变指针，历史表只DELETE。关闭来源与原安装终态已证明；仍有未知输出
或导出时整个事务回滚，不能为了绕外键关掉FOREIGN_KEY_CHECKS。

本步声明：`func RemoveAccountHistoryDependents(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;) 	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state I …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, statement, document)</code>；<code>deleteHistoryLeaves(ctx, tx, `DELETE l FROM branch_lineage l LEFT JOIN branch_lineage c ON c.previous_id=l.id 	 WHERE l.document_id=? AND c.id IS NULL`, &quot;SELECT COUNT(*) FROM branc …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;DELETE FROM branch WHERE document_id=?&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code>；<code>deleteHistoryLeaves(ctx, tx, `DELETE l FROM branch_lineage l LEFT JOIN branch_lineage c ON c.previous_id=l.id 	 WHERE l.document_id=? AND c.id IS NULL`, &quot;SELECT COUNT(*) FROM branc …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;) 	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state I …</code>（长表达式，按本步完整代码继续）；<code>domain.TextID(document)</code>；<code>tx.ExecContext(ctx, statement, document)</code>；<code>deleteHistoryLeaves(ctx, tx, `DELETE l FROM branch_lineage l LEFT JOIN branch_lineage c ON c.previous_id=l.id 	 WHERE l.document_id=? AND c.id IS NULL`, &quot;SELECT COUNT(*) FROM branc …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_HISTORY_DEPENDENT_PENDING&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;) 	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state I …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;) 	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state I …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>pending</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_HISTORY_DEPENDENT_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=10673:14824 -->
````go
// 只改可变指针，历史表只DELETE。关闭来源与原安装终态已证明；仍有未知输出
// 或导出时整个事务回滚，不能为了绕外键关掉FOREIGN_KEY_CHECKS。
func RemoveAccountHistoryDependents(ctx context.Context, tx *sql.Tx, document uint64) error {
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_session WHERE document_id=? AND state<>'SETTLED')
	 OR EXISTS(SELECT 1 FROM export_job WHERE document_id=? AND state IN ('QUEUED','RUNNING'))
	 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id=? AND state<>'RELEASED')
	 OR EXISTS(SELECT 1 FROM durable_task WHERE kind IN ('COMMENT_NOTIFICATION','REVIEW_NOTIFICATION','REVIEW_ACTION_NOTIFICATION','MERGE_NOTIFICATION')
	 AND state NOT IN ('SUCCEEDED','REJECTED') AND
	 (JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document_id'))=? OR JSON_UNQUOTE(JSON_EXTRACT(payload,'$.document'))=?))`, document, document, document, domain.TextID(document), domain.TextID(document)).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "ACCOUNT_HISTORY_DEPENDENT_PENDING")
	}
	statements := []string{
		"UPDATE collaboration_unit SET active_branch_id=NULL WHERE document_id=?",
		"UPDATE branch SET current_lineage_id=NULL WHERE document_id=?",
		"UPDATE review_request SET current_round_id=NULL,active_merge_operation_id=NULL WHERE document_id=?",
		"UPDATE submission_operation SET expected_round_id=NULL,result_round_id=NULL WHERE document_id=?",
		"UPDATE merge_operation m JOIN review_request r ON r.id=m.request_id SET m.result_merge_id=NULL WHERE r.document_id=?",
		"UPDATE merge_record m JOIN review_request r ON r.id=m.request_id SET m.approval_action_id=NULL WHERE r.document_id=?",
		"DELETE g FROM unit_review_guard g JOIN collaboration_unit u ON u.id=g.unit_id WHERE u.document_id=?",
		// 先拆可变resolved_action指针，删反向引用change的解析动作；创建动作本来
		// 不带target_change_id。随后删change与剩余动作，无须UPDATE不可变review_action。
		"UPDATE change_request c JOIN review_request r ON r.id=c.request_id SET c.status='OPEN',c.resolved_action_id=NULL WHERE r.document_id=?",
		"DELETE a FROM review_action a JOIN review_request r ON r.id=a.request_id WHERE r.document_id=? AND a.target_change_id IS NOT NULL",
		"DELETE c FROM change_request c JOIN review_request r ON r.id=c.request_id WHERE r.document_id=?",
		"DELETE a FROM review_action a JOIN review_request r ON r.id=a.request_id WHERE r.document_id=?",
		"DELETE m FROM merge_record m JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE a FROM merge_attempt a JOIN merge_operation m ON m.operation_id=a.operation_id JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE m FROM merge_operation m JOIN review_request r ON r.id=m.request_id WHERE r.document_id=?",
		"DELETE s FROM request_submitter s JOIN review_request r ON r.id=s.request_id WHERE r.document_id=?",
		"DELETE FROM review_round WHERE document_id=?",
		"DELETE FROM submission_operation WHERE document_id=?",
		"DELETE FROM review_request WHERE document_id=?",
		"UPDATE comment c JOIN comment_thread t ON t.id=c.thread_id SET c.reply_to_id=NULL WHERE t.document_id=?",
		"DELETE c FROM comment c JOIN comment_thread t ON t.id=c.thread_id WHERE t.document_id=?",
		"DELETE FROM comment_thread WHERE document_id=?",
		"DELETE FROM read_view WHERE document_id=?",
		"DELETE FROM delivery_session WHERE document_id=?",
		"DELETE t FROM collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id WHERE a.document_id=?",
		"DELETE s FROM admission_source s JOIN document_admission a ON a.admission_id=s.admission_id WHERE a.document_id=?",
		"DELETE FROM document_admission WHERE document_id=?",
		"DELETE FROM export_job WHERE document_id=?",
		"DELETE FROM main_commit_barrier WHERE document_id=?",
		"DELETE FROM unit_lineage WHERE source_document_id=?",
		"DELETE FROM document_application WHERE document_id=?",
		"DELETE FROM document_invitation WHERE document_id=?",
		"DELETE FROM content_grant WHERE document_id=?",
	}
````

## 第 8 步：RemoveAccountHistoryDependents（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 226 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, statement, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM branch WHERE document_id=?&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 8 bytes=14824:15415 -->
````go
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, document); err != nil {
			return err
		}
	}
	// previous_id保留原事实；真正从叶节点释放，而不是靠自增ID猜插入顺序。
	if err := deleteHistoryLeaves(ctx, tx, `DELETE l FROM branch_lineage l LEFT JOIN branch_lineage c ON c.previous_id=l.id
	 WHERE l.document_id=? AND c.id IS NULL`, "SELECT COUNT(*) FROM branch_lineage WHERE document_id=?", document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM branch WHERE document_id=?", document)
	return err
}

````

## 第 9 步：deleteHistoryLeaves

插入位置：第 8 步代码之后，紧接追加；完成稿约第 240 行。

本步声明：`func deleteHistoryLeaves(ctx context.Context, tx *sql.Tx, statement, remaining string, args ...any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, remaining, args...)</code>；<code>tx.ExecContext(ctx, statement, args...)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>statement</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, args...)</code> |
| <code>remaining</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, remaining, args...)</code> |
| <code>args</code> | <code>...any</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, remaining, args...)</code>；<code>tx.ExecContext(ctx, statement, args...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_CYCLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, remaining, args...).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, remaining, args...)</code>。
- 检查 <code>count == 0</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, statement, args...)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>removed == 0</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_CYCLE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=15415:15942 -->
````go
func deleteHistoryLeaves(ctx context.Context, tx *sql.Tx, statement, remaining string, args ...any) error {
	for {
		var count uint64
		if err := tx.QueryRowContext(ctx, remaining, args...).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		result, err := tx.ExecContext(ctx, statement, args...)
		if err != nil {
			return err
		}
		removed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if removed == 0 {
			return errors.New("ACCOUNT_HISTORY_DEPENDENCY_CYCLE")
		}
	}
}

````

## 第 10 步：RemoveAccountHistoryGraph

插入位置：第 9 步代码之后，紧接追加；完成稿约第 263 行。

本步声明：`func RemoveAccountHistoryGraph(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, g AccountHistoryGraph) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT manifest_id,title FROM `release` WHERE id=? AND document_id=?&quot;, item.Release, item.Document)</code>；<code>tx.ExecContext(ctx, `UPDATE document SET title=?,responsible_user_id=NULL,current_structure_id=?,current_style_id=?, 		 current_manifest_id=?,current_release_id=?,publication_state …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE document SET title=&#x27;已清理文档&#x27;,responsible_user_id=NULL,current_structure_id=NULL, 		 current_style_id=NULL,current_manifest_id=NULL,current_release_id=NULL …</code>（长表达式，按本步完整代码继续）<br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code>；<code>deleteHistoryLeaves(ctx, tx, `DELETE e FROM structure_entry e LEFT JOIN structure_entry c ON c.structure_id=e.structure_id AND c.parent_node_id=e.node_id 	 WHERE e.document_id=? AN …</code>（长表达式，按本步完整代码继续）<br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>item.Release</code>；<code>item.Manifest</code>；<code>item.Structure</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>g</code> | <code>AccountHistoryGraph</code> | 在此声明中的实际使用：<code>g.Manifests</code>；<code>g.Versions</code>；<code>g.KeepVersions</code>；<code>g.KeepManifests</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_RELEASE_CHANGED&quot;)</code>；<code>ReconcileCleanedResourceAllocations(ctx, tx, item.Document, item.Space, allocationResources)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>item.Release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT manifest_id,title FROM `release` WHERE id=? AND document_id=?&quot;, item.Release, item.Document).Scan(&amp;manifest, &amp;title)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT manifest_id,title FROM `release` WHERE id=? AND document_id=?&quot;, item.Release, item.Document)</code>。
- 检查 <code>manifest != item.Manifest</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_HISTORY_RELEASE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET title=?,responsible_user_id=NULL,current_structure_id=?,current_style_id=?, 		 current_manifest_id=?,current_release_id=?,publication_state …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET title=&#x27;已清理文档&#x27;,responsible_user_id=NULL,current_structure_id=NULL, 		 current_style_id=NULL,current_manifest_id=NULL,current_release_id=NULL …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM `release` WHERE document_id=? AND id&lt;&gt;?&quot;, item.Document, item.Release)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>manifest.ID == item.Manifest</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM manifest_entry WHERE manifest_id=? AND document_id=?&quot;, manifest.ID, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>g.KeepVersions&#91;version&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM resource_reference WHERE document_id=? AND owner_kind=&#x27;VERSION&#x27; AND owner_ref=?&quot;, item.Document, domain.TextID(version))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM content_version WHERE id=? AND document_id=?&quot;, version, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 10 bytes=15942:18212 -->
````go
func RemoveAccountHistoryGraph(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, g AccountHistoryGraph) error {
	allocationResources, err := LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)
	if err != nil {
		return err
	}
	// 标题必须跟保留release一致，不能把关闭前私有MAIN标题留在公开metadata。
	if item.Release != 0 {
		var manifest uint64
		var title string
		if err := tx.QueryRowContext(ctx, "SELECT manifest_id,title FROM `release` WHERE id=? AND document_id=?", item.Release, item.Document).Scan(&manifest, &title); err != nil {
			return err
		}
		if manifest != item.Manifest {
			return errors.New("ACCOUNT_HISTORY_RELEASE_CHANGED")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE document SET title=?,responsible_user_id=NULL,current_structure_id=?,current_style_id=?,
		 current_manifest_id=?,current_release_id=?,publication_state='PUBLISHED' WHERE id=?`, title, item.Structure, item.Style, item.Manifest, item.Release, item.Document); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE document SET title='已清理文档',responsible_user_id=NULL,current_structure_id=NULL,
		 current_style_id=NULL,current_manifest_id=NULL,current_release_id=NULL,publication_state='UNPUBLISHED' WHERE id=?`, item.Document); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM `release` WHERE document_id=? AND id<>?", item.Document, item.Release); err != nil {
		return err
	}
	for _, manifest := range g.Manifests {
		if manifest.ID == item.Manifest {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM manifest_entry WHERE manifest_id=? AND document_id=?", manifest.ID, item.Document); err != nil {
			return err
		}
	}
	// 非保留版本的pin与正文同事务移除，保留版本pin及RETENTION独立保留。
	for _, version := range g.Versions {
		if g.KeepVersions[version] {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM resource_reference WHERE document_id=? AND owner_kind='VERSION' AND owner_ref=?", item.Document, domain.TextID(version)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM content_version WHERE id=? AND document_id=?", version, item.Document); err != nil {
````

## 第 11 步：RemoveAccountHistoryGraph（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 308 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM main_manifest WHERE id=? AND document_id=?&quot;, id, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!g.KeepUnits&#91;id&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM collaboration_unit WHERE id=? AND document_id=?&quot;, id, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>!g.KeepChapters&#91;id&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM chapter WHERE id=? AND document_id=?&quot;, id, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM &quot;+family.table+&quot; WHERE id=? AND document_id=?&quot;, id, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 11 bytes=18212:20130 -->
````go
			return err
		}
	}
	order, err := domain.HistoryDeletionOrder(g.Manifests, g.KeepManifests)
	if err != nil {
		return err
	}
	for _, id := range order {
		if _, err = tx.ExecContext(ctx, "DELETE FROM main_manifest WHERE id=? AND document_id=?", id, item.Document); err != nil {
			return err
		}
	}
	if err = deleteHistoryLeaves(ctx, tx, `DELETE e FROM structure_entry e LEFT JOIN structure_entry c ON c.structure_id=e.structure_id AND c.parent_node_id=e.node_id
	 WHERE e.document_id=? AND e.structure_id<>? AND c.node_id IS NULL`, "SELECT COUNT(*) FROM structure_entry WHERE document_id=? AND structure_id<>?", item.Document, item.Structure); err != nil {
		return err
	}
	for _, id := range g.Units {
		if !g.KeepUnits[id] {
			if _, err = tx.ExecContext(ctx, "DELETE FROM collaboration_unit WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	for _, id := range g.Chapters {
		if !g.KeepChapters[id] {
			if _, err = tx.ExecContext(ctx, "DELETE FROM chapter WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	for _, family := range []struct {
		table string
		rows  []domain.AccountHistoryRevision
		keep  map[uint64]bool
	}{{"structure_revision", g.Structures, g.KeepStructures}, {"style_revision", g.Styles, g.KeepStyles}} {
		order, err := domain.HistoryDeletionOrder(family.rows, family.keep)
		if err != nil {
			return err
		}
		// table只有上面的两个编译期常量；编号全部参数绑定。
		for _, id := range order {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+family.table+" WHERE id=? AND document_id=?", id, item.Document); err != nil {
				return err
			}
		}
	}
	// 上传原号及原document绑定属于资源恢复协议，保留关闭的文档metadata；其他历史资源pin只保留实际
	// 存在的固定VERSION/CONTENT及当前正式图片RETENTION。外文档共享引用不受影响。
````

## 第 12 步：RemoveAccountHistoryGraph（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 356 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND owner_kind NOT IN (&#x27;VERSION&#x27;,&#x27;RETENTION&#x27;) 	 AND NOT(owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;UPLOAD&#x27;)`, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return ReconcileCleanedResourceAllocations(ctx, tx, item.Document, item.Space, allocationResources)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=20130:20638 -->
````go
	if _, err = tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND owner_kind NOT IN ('VERSION','RETENTION')
	 AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')`, item.Document); err != nil {
		return err
	}
	if err = ReconcileCleanedDocumentCapacity(ctx, tx, item.Document); err != nil {
		return err
	}
	return ReconcileCleanedResourceAllocations(ctx, tx, item.Document, item.Space, allocationResources)
}

// account_history_graph.go：保留原实现，按业务合并文件。

````

## 第 13 步：AccountHistoryGraph

插入位置：第 12 步代码之后，紧接追加；完成稿约第 368 行。

本步声明：`type AccountHistoryGraph struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Manifests</code> | <code>&#91;&#93;domain.AccountHistoryRevision</code> | 使用 <code>domain.AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。 |
| <code>Structures</code> | <code>&#91;&#93;domain.AccountHistoryRevision</code> | 使用 <code>domain.AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。 |
| <code>Styles</code> | <code>&#91;&#93;domain.AccountHistoryRevision</code> | 使用 <code>domain.AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。 |
| <code>Units</code> | <code>&#91;&#93;uint64</code> | 该文档历史中的协作单元编号集合。 |
| <code>Chapters</code> | <code>&#91;&#93;uint64</code> | 历史章节编号集合。 |
| <code>Versions</code> | <code>&#91;&#93;uint64</code> | 历史固定正文版本集合。 |
| <code>KeepManifests</code> | <code>map&#91;uint64&#93;bool</code> | 正式留存所需清单的集合，用布尔map做成员判断。 |
| <code>KeepStructures</code> | <code>map&#91;uint64&#93;bool</code> | 留存正文仍引用的结构版本集合。 |
| <code>KeepStyles</code> | <code>map&#91;uint64&#93;bool</code> | 留存正文仍引用的样式版本集合。 |
| <code>KeepUnits</code> | <code>map&#91;uint64&#93;bool</code> | 留存范围内仍需要的协作单元集合。 |
| <code>KeepChapters</code> | <code>map&#91;uint64&#93;bool</code> | 留存范围内仍需要的章节集合。 |
| <code>KeepVersions</code> | <code>map&#91;uint64&#93;bool</code> | 必须保留的固定正文版本集合；清理时不能仅看是否当前版本。 |

<!-- write-step: 13 bytes=20638:21060 -->
````go
type AccountHistoryGraph struct {
	Manifests, Structures, Styles []domain.AccountHistoryRevision
	Units, Chapters, Versions     []uint64
	KeepManifests                 map[uint64]bool
	KeepStructures                map[uint64]bool
	KeepStyles                    map[uint64]bool
	KeepUnits                     map[uint64]bool
	KeepChapters                  map[uint64]bool
	KeepVersions                  map[uint64]bool
}

````

## 第 14 步：historyRevisions

插入位置：第 13 步代码之后，紧接追加；完成稿约第 379 行。

本步声明：`func historyRevisions(ctx context.Context, tx *sql.Tx, statement string, document uint64) ([]domain.AccountHistoryRevision, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, statement, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>statement</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, statement, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, statement, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.AccountHistoryRevision</code> | 使用 <code>domain.AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, statement, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;row.ID, &amp;row.Parent, &amp;row.Structure, &amp;row.Style)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=21060:21583 -->
````go
func historyRevisions(ctx context.Context, tx *sql.Tx, statement string, document uint64) ([]domain.AccountHistoryRevision, error) {
	rows, err := tx.QueryContext(ctx, statement, document)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.AccountHistoryRevision{}
	for rows.Next() {
		var row domain.AccountHistoryRevision
		if err = rows.Scan(&row.ID, &row.Parent, &row.Structure, &row.Style); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

````

## 第 15 步：AccountHistoryGraphRow

插入位置：第 14 步代码之后，紧接追加；完成稿约第 396 行。

文档已封门并持锁。版本通过unit前导索引读取，不按无索引document_id扫描全文表。

本步声明：`func AccountHistoryGraphRow(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument) (AccountHistoryGraph, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),structure_id,style_id FROM main_manifest WHERE document_id=?&quot;, item.Document)</code>；<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),0,0 FROM structure_revision WHERE document_id=?&quot;, item.Document)</code>；<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),0,0 FROM style_revision WHERE document_id=?&quot;, item.Document)</code>；<code>tx.QueryContext(ctx, &quot;SELECT chapter_id,COALESCE(unit_id,0) FROM structure_entry WHERE structure_id=? AND document_id=?&quot;, item.Structure, item.Document)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),structure_id,style_id FROM main_manifest WHERE document_id=?&quot;, item.Document)</code>；<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),0,0 FROM structure_revision WHERE document_id=?&quot;, item.Document)</code>；<code>historyRevisions(ctx, tx, &quot;SELECT id,COALESCE(parent_id,0),0,0 FROM style_revision WHERE document_id=?&quot;, item.Document)</code>；<code>tx.QueryContext</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>item.Release</code>；<code>item.Manifest</code>；<code>item.Structure</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountHistoryGraph</code> | 实际返回表达式：<code>g</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_HISTORY_FIXED_VERSION_MISSING&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>item.Release != 0</code>。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT chapter_id,COALESCE(unit_id,0) FROM structure_entry WHERE structure_id=? AND document_id=?&quot;, item.Structure, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;chapter, &amp;unit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>unit != 0</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 15 bytes=21583:23300 -->
````go
// 文档已封门并持锁。版本通过unit前导索引读取，不按无索引document_id扫描全文表。
func AccountHistoryGraphRow(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument) (AccountHistoryGraph, error) {
	g := AccountHistoryGraph{
		KeepManifests:  map[uint64]bool{},
		KeepStructures: map[uint64]bool{},
		KeepStyles:     map[uint64]bool{},
		KeepUnits:      map[uint64]bool{},
		KeepChapters:   map[uint64]bool{},
		KeepVersions:   map[uint64]bool{},
	}
	var err error
	g.Manifests, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),structure_id,style_id FROM main_manifest WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	g.Structures, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),0,0 FROM structure_revision WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	g.Styles, err = historyRevisions(ctx, tx, "SELECT id,COALESCE(parent_id,0),0,0 FROM style_revision WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	if item.Release != 0 {
		g.KeepManifests[item.Manifest], g.KeepStructures[item.Structure], g.KeepStyles[item.Style] = true, true, true
		for _, id := range item.Versions {
			g.KeepVersions[id] = true
		}
		rows, err := tx.QueryContext(ctx, "SELECT chapter_id,COALESCE(unit_id,0) FROM structure_entry WHERE structure_id=? AND document_id=?", item.Structure, item.Document)
		if err != nil {
			return g, err
		}
		for rows.Next() {
			var chapter, unit uint64
			if err = rows.Scan(&chapter, &unit); err != nil {
				break
			}
			g.KeepChapters[chapter] = true
			if unit != 0 {
				g.KeepUnits[unit] = true
			}
		}
		if err == nil {
			err = rows.Err()
		}
````

## 第 16 步：AccountHistoryGraphRow（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 441 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>g.KeepManifests&#91;manifest.ID&#93;</code>。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,chapter_id,created_structure_id,COALESCE(retired_structure_id,0) FROM collaboration_unit WHERE document_id=? ORDER BY id&quot;, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;chapter, &amp;created, &amp;retired)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>g.KeepUnits&#91;id&#93;</code>。
- 检查 <code>retired != 0</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,structure_id FROM content_version WHERE unit_id=? AND document_id=? ORDER BY id&quot;, unit, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>versions.Scan(&amp;id, &amp;structure)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。

<!-- write-step: 16 bytes=23300:24604 -->
````go
		rows.Close()
		if err != nil {
			return g, err
		}
	}
	if err = domain.KeepHistoryAncestors(g.Manifests, g.KeepManifests); err != nil {
		return g, err
	}
	for _, manifest := range g.Manifests {
		if g.KeepManifests[manifest.ID] {
			g.KeepStructures[manifest.Structure], g.KeepStyles[manifest.Style] = true, true
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,chapter_id,created_structure_id,COALESCE(retired_structure_id,0) FROM collaboration_unit WHERE document_id=? ORDER BY id", item.Document)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var id, chapter, created, retired uint64
		if err = rows.Scan(&id, &chapter, &created, &retired); err != nil {
			break
		}
		g.Units = append(g.Units, id)
		if g.KeepUnits[id] {
			g.KeepChapters[chapter], g.KeepStructures[created] = true, true
			if retired != 0 {
				g.KeepStructures[retired] = true
			}
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return g, err
	}
	for _, unit := range g.Units {
		versions, err := tx.QueryContext(ctx, "SELECT id,structure_id FROM content_version WHERE unit_id=? AND document_id=? ORDER BY id", unit, item.Document)
		if err != nil {
			return g, err
		}
		for versions.Next() {
			var id, structure uint64
			if err = versions.Scan(&id, &structure); err != nil {
````

## 第 17 步：AccountHistoryGraphRow（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 486 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>g.KeepVersions&#91;id&#93;</code>。
- 检查 <code>!g.KeepUnits&#91;unit&#93; &#124;&#124; structure != item.Structure</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>!foundVersions&#91;id&#93;</code>。条件成立时执行 <code>return g, errors.New(&quot;ACCOUNT_HISTORY_FIXED_VERSION_MISSING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,created_structure_id,COALESCE(retired_structure_id,0) FROM chapter WHERE document_id=?&quot;, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;created, &amp;retired)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>g.KeepChapters&#91;id&#93;</code>。
- 检查 <code>retired != 0</code>。

<!-- write-step: 17 bytes=24604:25693 -->
````go
				break
			}
			g.Versions = append(g.Versions, id)
			if g.KeepVersions[id] {
				if !g.KeepUnits[unit] || structure != item.Structure {
					err = errors.New("ACCOUNT_HISTORY_FIXED_VERSION_CHANGED")
					break
				}
				g.KeepStructures[structure] = true
			}
		}
		if err == nil {
			err = versions.Err()
		}
		versions.Close()
		if err != nil {
			return g, err
		}
	}
	foundVersions := map[uint64]bool{}
	for _, id := range g.Versions {
		foundVersions[id] = true
	}
	for _, id := range item.Versions {
		if !foundVersions[id] {
			return g, errors.New("ACCOUNT_HISTORY_FIXED_VERSION_MISSING")
		}
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,created_structure_id,COALESCE(retired_structure_id,0) FROM chapter WHERE document_id=?", item.Document)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		var id, created, retired uint64
		if err = rows.Scan(&id, &created, &retired); err != nil {
			break
		}
		g.Chapters = append(g.Chapters, id)
		if g.KeepChapters[id] {
			g.KeepStructures[created] = true
			if retired != 0 {
				g.KeepStructures[retired] = true
			}
		}
	}
````

## 第 18 步：AccountHistoryGraphRow（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 531 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return g, err</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=25693:26032 -->
````go
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return g, err
	}
	if err = domain.KeepHistoryAncestors(g.Structures, g.KeepStructures); err != nil {
		return g, err
	}
	err = domain.KeepHistoryAncestors(g.Styles, g.KeepStyles)
	return g, err
}

// account_relations.go：保留原实现，按业务合并文件。

````

## 第 19 步：DiscoverAccountRelations

插入位置：第 18 步代码之后，紧接追加；完成稿约第 547 行。

身份来源已经封闭，候选只能收缩；仍在全部来源与业务锁下重新发现，
不把最初的普通SELECT当作关闭当前授权或清空负责人关系的依据。

本步声明：`func DiscoverAccountRelations(ctx context.Context, q Queryer, user uint64) (AccountRetentionCandidates, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT k.id,k.space_id,k.acl_revision FROM knowledge_base k WHERE k.id IN ( 		SELECT knowledge_base_id FROM knowledge_base_member WHERE user_id=? AND state&lt;&gt;&#x27;C …</code>（长表达式，按本步完整代码继续）；<code>q.QueryContext(ctx, `SELECT id,space_id,COALESCE(knowledge_base_id,0),acl_revision FROM document WHERE id IN ( 		SELECT id FROM document WHERE responsible_user_id=? 		UNION SELECT  …</code>（长表达式，按本步完整代码继续）；<code>q.QueryRowContext(ctx, &quot;SELECT space_id,acl_revision FROM knowledge_base WHERE id=?&quot;, kb)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code>；<code>q.QueryRowContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, `SELECT k.id,k.space_id,k.acl_revision FROM knowledge_base k WHERE k.id IN ( 		SELECT knowledge_base_id FROM knowledge_base_member WHERE user_id=? AND state&lt;&gt;&#x27;C …</code>（长表达式，按本步完整代码继续）；<code>q.QueryContext(ctx, `SELECT id,space_id,COALESCE(knowledge_base_id,0),acl_revision FROM document WHERE id IN ( 		SELECT id FROM document WHERE responsible_user_id=? 		UNION SELECT  …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionCandidates</code> | 使用 <code>AccountRetentionCandidates</code> 类型；其定义处的职责：固定账号关闭后的Doc/KB/Space留存范围并核真实排空与完成。<br>实际返回表达式：<code>c</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, `SELECT k.id,k.space_id,k.acl_revision FROM knowledge_base k WHERE k.id IN ( 		SELECT knowledge_base_id FROM knowledge_base_member WHERE user_id=? AND state&lt;&gt;&#x27;C …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;space, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryContext(ctx, `SELECT id,space_id,COALESCE(knowledge_base_id,0),acl_revision FROM document WHERE id IN ( 		SELECT id FROM document WHERE responsible_user_id=? 		UNION SELECT  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;space, &amp;kb, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>kb != 0 &amp;&amp; !slices.Contains(c.KnowledgeBases, kb)</code>。
- 检查 <code>err == nil</code>。

<!-- write-step: 19 bytes=26032:28034 -->
````go
// 身份来源已经封闭，候选只能收缩；仍在全部来源与业务锁下重新发现，
// 不把最初的普通SELECT当作关闭当前授权或清空负责人关系的依据。
func DiscoverAccountRelations(ctx context.Context, q Queryer, user uint64) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	rows, err := q.QueryContext(ctx, `SELECT k.id,k.space_id,k.acl_revision FROM knowledge_base k WHERE k.id IN (
		SELECT knowledge_base_id FROM knowledge_base_member WHERE user_id=? AND state<>'CLOSED'
		UNION SELECT knowledge_base_id FROM knowledge_base_creation_grant WHERE user_id=? AND state<>'CLOSED') ORDER BY k.id`, user, user)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, space, epoch uint64
		if err = rows.Scan(&id, &space, &epoch); err != nil {
			break
		}
		c.KnowledgeBases = append(c.KnowledgeBases, id)
		c.Spaces = append(c.Spaces, space)
		c.Sources = append(c.Sources, domain.LeaseSource{Key: "kb-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	rows, err = q.QueryContext(ctx, `SELECT id,space_id,COALESCE(knowledge_base_id,0),acl_revision FROM document WHERE id IN (
		SELECT id FROM document WHERE responsible_user_id=?
		UNION SELECT document_id FROM content_grant WHERE user_id=? AND state<>'CLOSED') ORDER BY id`, user, user)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, space, kb, epoch uint64
		if err = rows.Scan(&id, &space, &kb, &epoch); err != nil {
			break
		}
		c.Documents = append(c.Documents, id)
		c.Spaces = append(c.Spaces, space)
		if kb != 0 && !slices.Contains(c.KnowledgeBases, kb) {
			c.KnowledgeBases = append(c.KnowledgeBases, kb)
		}
		c.Sources = append(c.Sources, domain.LeaseSource{Key: "doc-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
	}
	if err == nil {
````

## 第 20 步：DiscoverAccountRelations（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 592 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>slices.ContainsFunc(c.Sources, func(s domain.LeaseSource) bool { return s.Key == key })</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：此处通过 <code>return s.Key == key</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT space_id,acl_revision FROM knowledge_base WHERE id=?&quot;, kb).Scan(&amp;space, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT space_id,acl_revision FROM knowledge_base WHERE id=?&quot;, kb)</code>。
- 在本段的匿名函数内：检查 <code>a.Key &lt; b.Key</code>。条件成立时执行 <code>return -1</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>a.Key &gt; b.Key</code>。条件成立时执行 <code>return 1</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return c, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=28034:28971 -->
````go
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	// 负责人文档所在KB也必须持相同来源锁；它未必在用户的KB成员集合中。
	for _, kb := range c.KnowledgeBases {
		key := "kb-acl:" + domain.TextID(kb)
		if slices.ContainsFunc(c.Sources, func(s domain.LeaseSource) bool { return s.Key == key }) {
			continue
		}
		var space, epoch uint64
		if err = q.QueryRowContext(ctx, "SELECT space_id,acl_revision FROM knowledge_base WHERE id=?", kb).Scan(&space, &epoch); err != nil {
			return c, err
		}
		c.Spaces = append(c.Spaces, space)
		c.Sources = append(c.Sources, domain.LeaseSource{Key: key, Epoch: domain.TextID(epoch)})
	}
	slices.Sort(c.Spaces)
	c.Spaces = slices.Compact(c.Spaces)
	slices.Sort(c.KnowledgeBases)
	slices.SortFunc(c.Sources, func(a, b domain.LeaseSource) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return c, nil
}

````

## 第 21 步：ApplyAccountRelations

插入位置：第 20 步代码之后，紧接追加；完成稿约第 626 行。

本步声明：`func ApplyAccountRelations(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, c AccountRetentionCandidates) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT operation_id,state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, binding.User)</code>；<code>AvailableKBAdministrators(ctx, tx, kb)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE admin_retirement SET state=&#x27;APPLIED&#x27; WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state=&#x27;RESERVED&#x27;&quot;, kb, binding.User, binding.Operation)</code>；<code>tx.QueryRowContext(ctx, `SELECT s.kind,COALESCE(s.organization_id,0),k.lifecycle,s.lifecycle FROM knowledge_base k JOIN space s ON s.id=k.space_id WHERE k.id=?`, kb)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>AvailableKBAdministrators(ctx, tx, kb)</code>；<code>tx.ExecContext</code>；<code>queueKnowledgeBaseDepartures(ctx, tx, domain.DepartureBinding{ID: binding.Operation, Org: organization, User: binding.User, Epoch: binding.IdentityEpoch}, kb)</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Binding</code>；<code>plan.ReservedKnowledgeBases</code> |
| <code>c</code> | <code>AccountRetentionCandidates</code> | 使用 <code>AccountRetentionCandidates</code> 类型；其定义处的职责：固定账号关闭后的Doc/KB/Space留存范围并核真实排空与完成。<br>在此声明中的实际使用：<code>c.KnowledgeBases</code>；<code>c.Documents</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETIREMENT_RESERVATION_CHANGED&quot;)</code>；<code>errors.New(&quot;LAST_KB_ADMIN_REQUIRED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT operation_id,state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, binding.User).Scan(&amp;operation, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT operation_id,state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE&quot;, kb, binding.User)</code>。
- 检查 <code>operation != binding.Operation &#124;&#124; state != &quot;RESERVED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETIREMENT_RESERVATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>available &lt; 1</code>。条件成立时执行 <code>return errors.New(&quot;LAST_KB_ADMIN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE admin_retirement SET state=&#x27;APPLIED&#x27; WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state=&#x27;RESERVED&#x27;&quot;, kb, binding.User, binding.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT s.kind,COALESCE(s.organization_id,0),k.lifecycle,s.lifecycle FROM knowledge_base k JOIN space s ON s.id=k.space_id WHERE k.id=?`, kb).Scan(&amp;kind, &amp;organization, &amp;kbState, &amp;spaceState)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT s.kind,COALESCE(s.organization_id,0),k.lifecycle,s.lifecycle FROM knowledge_base k JOIN space s ON s.id=k.space_id WHERE k.id=?`, kb)</code>。
- 检查 <code>kind == &quot;ORGANIZATION&quot; &amp;&amp; !slices.Contains(binding.ClosingOrganizations, organization) &amp;&amp; kbState == &quot;ACTIVE&quot; &amp;&amp; spaceState == &quot;ACTIVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, statement, kb, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE content_grant SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE document_id=? AND user_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, id, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 21 bytes=28971:31505 -->
````go
func ApplyAccountRelations(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan, c AccountRetentionCandidates) error {
	binding := plan.Binding
	for _, kb := range plan.ReservedKnowledgeBases {
		var operation, state string
		if err := tx.QueryRowContext(ctx, "SELECT operation_id,state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", kb, binding.User).Scan(&operation, &state); err != nil {
			return err
		}
		if operation != binding.Operation || state != "RESERVED" {
			return errors.New("ACCOUNT_RETIREMENT_RESERVATION_CHANGED")
		}
		available, err := AvailableKBAdministrators(ctx, tx, kb)
		if err != nil {
			return err
		}
		if available < 1 {
			return errors.New("LAST_KB_ADMIN_REQUIRED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE admin_retirement SET state='APPLIED' WHERE knowledge_base_id=? AND user_id=? AND operation_id=? AND state='RESERVED'", kb, binding.User, binding.Operation); err != nil {
			return err
		}
	}
	for _, kb := range c.KnowledgeBases {
		var organization uint64
		var kind, kbState, spaceState string
		if err := tx.QueryRowContext(ctx, `SELECT s.kind,COALESCE(s.organization_id,0),k.lifecycle,s.lifecycle FROM knowledge_base k JOIN space s ON s.id=k.space_id WHERE k.id=?`, kb).Scan(&kind, &organization, &kbState, &spaceState); err != nil {
			return err
		}
		if kind == "ORGANIZATION" && !slices.Contains(binding.ClosingOrganizations, organization) && kbState == "ACTIVE" && spaceState == "ACTIVE" {
			// 原Account身份关闭证明已验证；这里只复用管理员快照/可靠通知登记。
			// 不伪造离组门禁，也不调用普通Departure.Close绕过管理员退休检查。
			if err := queueKnowledgeBaseDepartures(ctx, tx, domain.DepartureBinding{ID: binding.Operation, Org: organization, User: binding.User, Epoch: binding.IdentityEpoch}, kb); err != nil {
				return err
			}
		}
		for _, statement := range []string{
			"UPDATE knowledge_base_member SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND user_id=? AND state<>'CLOSED'",
			"UPDATE knowledge_base_creation_grant SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND user_id=? AND state<>'CLOSED'",
		} {
			if _, err := tx.ExecContext(ctx, statement, kb, binding.User); err != nil {
				return err
			}
		}
	}
	for _, id := range c.Documents {
		if _, err := tx.ExecContext(ctx, "UPDATE content_grant SET state='CLOSED',revision=revision+1 WHERE document_id=? AND user_id=? AND state<>'CLOSED'", id, binding.User); err != nil {
````

## 第 22 步：ApplyAccountRelations（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 671 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET responsible_user_id=NULL,control_revision=control_revision+1 WHERE id=? AND responsible_user_id=?&quot;, id, binding.User)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;PLANNED&#x27;&quot;, binding.Operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=31505:32202 -->
````go
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE document SET responsible_user_id=NULL,control_revision=control_revision+1 WHERE id=? AND responsible_user_id=?", id, binding.User); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING',generation=generation+1 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='PLANNED'", binding.Operation)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("ACCOUNT_RETENTION_STATE_CHANGED")
	}
	return nil
}

// account_resource_cleanup.go：保留原实现，按业务合并文件。

````

## 第 23 步：AccountResourceCleanupReceipt

插入位置：第 22 步代码之后，紧接追加；完成稿约第 693 行。

本步声明：`func AccountResourceCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_RESOURCE_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>request</code> | <code>platform.AccountDocumentCleanupRequest</code> | 使用 <code>platform.AccountDocumentCleanupRequest</code> 类型；其定义处的职责：Node边界的编号必须是规范十进制字符串，避免JS Number改变原清理对象或代次。<br>在此声明中的实际使用：<code>request.Document</code>；<code>context.AccountDocumentCleanupRequest != request</code>；<code>request.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>context</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_RESOURCE_CLEANUP_BINDING_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_RESOURCE_CLEANUP&#x27; AND business_key=?&quot;, request.Document).Scan(&amp;operation, &amp;state, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT operation_id,state,payload FROM durable_task WHERE kind=&#x27;ACCOUNT_RESOURCE_CLEANUP&#x27; AND business_key=?&quot;, request.Document)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return context, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return context, false, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;context) != nil &#124;&#124; !context.Valid() &#124;&#124; context.AccountDocumentCleanupRequest != request &#124;&#124; operation != request.Operation &#124;&#124; state != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return context, false, errors.New(&quot;ACCOUNT_RESOURCE_CLEANUP_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return context, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=32202:33065 -->
````go
func AccountResourceCleanupReceipt(ctx context.Context, q Queryer, request platform.AccountDocumentCleanupRequest) (platform.AccountDocumentCleanupContext, bool, error) {
	var context platform.AccountDocumentCleanupContext
	var raw []byte
	var operation, state string
	err := q.QueryRowContext(ctx, "SELECT operation_id,state,payload FROM durable_task WHERE kind='ACCOUNT_RESOURCE_CLEANUP' AND business_key=?", request.Document).Scan(&operation, &state, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return context, false, nil
	}
	if err != nil {
		return context, false, err
	}
	if json.Unmarshal(raw, &context) != nil || !context.Valid() || context.AccountDocumentCleanupRequest != request || operation != request.Operation || state != "SUCCEEDED" {
		return context, false, errors.New("ACCOUNT_RESOURCE_CLEANUP_BINDING_CHANGED")
	}
	return context, true, nil
}

````

## 第 24 步：ReleaseAccountDraftResources

插入位置：第 23 步代码之后，紧接追加；完成稿约第 710 行。

Doc→export→protection→resource。来源已关闭，Finalize/Protect不能越过原来源锁；
GC持资源行锁并重查引用。先建立留存pin，再释放草稿pin，始终不存在无引用窗口。

本步声明：`func ReleaseAccountDraftResources(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ReleaseDocumentDraftResourcePins(ctx, tx, item, authority)</code>；<code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_RESOURCE_CLEANUP&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))`, autho …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ReleaseDocumentDraftResourcePins(ctx, tx, item, authority)</code>；<code>tx.ExecContext</code> |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>ReleaseDocumentDraftResourcePins(ctx, tx, item, authority)</code> |
| <code>authority</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>ReleaseDocumentDraftResourcePins(ctx, tx, item, authority)</code>；<code>json.Marshal(authority)</code>；<code>authority.Document</code>；<code>authority.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) 		VALUES(&#x27;ACCOUNT_RESOURCE_CLEANUP&#x27;,?,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))`, autho …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 24 bytes=33065:33870 -->
````go
// Doc→export→protection→resource。来源已关闭，Finalize/Protect不能越过原来源锁；
// GC持资源行锁并重查引用。先建立留存pin，再释放草稿pin，始终不存在无引用窗口。
func ReleaseAccountDraftResources(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error {
	if err := ReleaseDocumentDraftResourcePins(ctx, tx, item, authority); err != nil {
		return err
	}
	raw, err := json.Marshal(authority)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('ACCOUNT_RESOURCE_CLEANUP',?,?,?,'SUCCEEDED',UTC_TIMESTAMP(6))`, authority.Document, authority.Operation, string(raw))
	return err
}

````

## 第 25 步：ReleaseDocumentDraftResourcePins

插入位置：第 24 步代码之后，紧接追加；完成稿约第 725 行。

低层只操作原事务的引用与保护，不插入任何业务清理回执。Account/PURGE分别
核验自己的固定来源及Node CLEANED，再使用各自原号登记回执，避免复用doc键。

本步声明：`func ReleaseDocumentDraftResourcePins(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, item.Space)</code>；<code>allocations.LockQuota(ctx, tx, item.Space, limit)</code>；<code>allocations.ReleaseCleanedDraftBodies(ctx, tx, q, item.Document)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockDocumentResourceAllocationCleanup(ctx, tx, item.Document)</code>；<code>tx.QueryRowContext</code>；<code>allocations.LockQuota(ctx, tx, item.Space, limit)</code>；<code>allocations.ReleaseCleanedDraftBodies(ctx, tx, q, item.Document)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>item</code> | <code>domain.AccountRetentionDocument</code> | 使用 <code>domain.AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>在此声明中的实际使用：<code>item.Document</code>；<code>item.Space</code>；<code>item.Resources</code> |
| <code>authority</code> | <code>platform.AccountDocumentCleanupContext</code> | 使用 <code>platform.AccountDocumentCleanupContext</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>authority.Document</code>；<code>authority.Operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_RESOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, item.Space).Scan(&amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, item.Space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT id,state FROM export_job WHERE document_id=? ORDER BY id FOR UPDATE&quot;, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>state == &quot;QUEUED&quot; &#124;&#124; state == &quot;RUNNING&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT operation_id,state FROM resource_protection WHERE document_id=? ORDER BY operation_id FOR UPDATE&quot;, item.Document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=33870:35464 -->
````go
// 低层只操作原事务的引用与保护，不插入任何业务清理回执。Account/PURGE分别
// 核验自己的固定来源及Node CLEANED，再使用各自原号登记回执，避免复用doc键。
func ReleaseDocumentDraftResourcePins(ctx context.Context, tx *sql.Tx, item domain.AccountRetentionDocument, authority platform.AccountDocumentCleanupContext) error {
	if _, err := LockDocumentResourceAllocationCleanup(ctx, tx, item.Document); err != nil {
		return err
	}
	var limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", item.Space).Scan(&limit); err != nil {
		return err
	}
	q, err := allocations.LockQuota(ctx, tx, item.Space, limit)
	if err != nil {
		return err
	}
	if err = allocations.ReleaseCleanedDraftBodies(ctx, tx, q, item.Document); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,state FROM export_job WHERE document_id=? ORDER BY id FOR UPDATE", item.Document)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uint64
		var state string
		if err = rows.Scan(&id, &state); err != nil {
			break
		}
		if state == "QUEUED" || state == "RUNNING" {
			err = domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_EXPORT_PENDING")
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, "SELECT operation_id,state FROM resource_protection WHERE document_id=? ORDER BY operation_id FOR UPDATE", item.Document)
	if err != nil {
		return err
	}
	for rows.Next() {
		var operation, state string
````

## 第 26 步：ReleaseDocumentDraftResourcePins（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 770 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;operation, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>state == &quot;UNKNOWN&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!allowed &#124;&#124; state != &quot;READY&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_RESOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation) 			VALUES(?,?,&#x27;RETENTION&#x27;,?,&#x27;PUBLIC_RELEASE&#x27;,1)`, id, item.Docu …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE resource_protection SET state=&#x27;RELEASED&#x27; WHERE document_id=? AND state IN (&#x27;PROTECTED&#x27;,&#x27;CONFIRMED&#x27;)&quot;, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND 		(owner_kind IN (&#x27;DRAFT&#x27;,&#x27;CAPTURE&#x27;) OR (owner_kind=&#x27;OPERATION&#x27; AND purpose=&#x27;DRAFT_UPDATE&#x27;))`, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=35464:37090 -->
````go
		if err = rows.Scan(&operation, &state); err != nil {
			break
		}
		if state == "UNKNOWN" {
			err = domain.Violation(domain.StateConflict, "ACCOUNT_DOCUMENT_RESOURCE_UNKNOWN")
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range item.Resources {
		_, state, err := LockedResourceSpace(ctx, tx, id)
		if err != nil {
			return err
		}
		allowed, err := allocations.ResourceAllowedForDocument(ctx, tx, id, item.Document, item.Space)
		if err != nil {
			return err
		}
		if !allowed || state != "READY" {
			return errors.New("ACCOUNT_RETENTION_RESOURCE_CHANGED")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose,generation)
			VALUES(?,?,'RETENTION',?,'PUBLIC_RELEASE',1)`, id, item.Document, authority.Document+":"+authority.Operation)
		if err != nil {
			return err
		}
	}
	// Node原CLEANED回执证明这些房间和原安装均已关闭、私有正文已移除。
	// UNKNOWN不由TTL推成终态。VERSION/MANIFEST/RELEASE/EXPORT引用仍随各自历史清理，
	// 此阶段不能因草稿已清就让仍存在的版本图片被物理删除。
	if _, err = tx.ExecContext(ctx, "UPDATE resource_protection SET state='RELEASED' WHERE document_id=? AND state IN ('PROTECTED','CONFIRMED')", item.Document); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE document_id=? AND
		(owner_kind IN ('DRAFT','CAPTURE') OR (owner_kind='OPERATION' AND purpose='DRAFT_UPDATE'))`, item.Document); err != nil {
		return err
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/account_cleanup_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
