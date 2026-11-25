# 分步编写 `services/document/internal/repository/document/organization_lifecycle_queries.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/162-organization-lifecycle-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久独立组织关闭计划并发现原组织空间范围。

关闭组织不应冻结负责人本人身份或个人空间，不能复用账号范围假装同一操作。

写之前先按这条执行路线理解各段的连接：

1. DiscoverOrganizationClose限定原组织
2. SaveOrganizationClose固定保留依赖
3. OrganizationCloseGateMatches核Account真实组织屏障
4. OrganizationCloseDrained核两端ACK
5. CompleteOrganizationClose收尾。

本文件需要守住的边界：

- 外部协作者仍可能完成旧写入，候选需锁后复核
- 不伪造identity墓碑。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/organization_lifecycle_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/organization_lifecycle_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/organization_lifecycle_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/organization_lifecycle_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:329 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"slices"
)

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 2 步：DiscoverOrganizationClose

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

候选只读取原组织空间；Account组织来源ACK不冻结本人身份或个人空间。
外部KB协作者仍可能完成旧写入，必须在来源锁和业务锁后重新核对候选。

本步声明：`func DiscoverOrganizationClose(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding) (AccountRetentionCandidates, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, statement+&quot; ORDER BY id&quot;, args...)</code>；<code>q.QueryContext(ctx, &quot;SELECT id,acl_revision,lifecycle FROM knowledge_base WHERE space_id=? ORDER BY id&quot;, space)</code>；<code>q.QueryContext(ctx, &quot;SELECT id,acl_revision,publication_epoch FROM document WHERE space_id=? AND lifecycle&lt;&gt;&#x27;PURGED&#x27; ORDER BY id&quot;, space)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>binding</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>binding.Organization</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionCandidates</code> | 使用 <code>AccountRetentionCandidates</code> 类型；其定义处的职责：固定账号关闭后的Doc/KB/Space留存范围并核真实排空与完成。<br>实际返回表达式：<code>c</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, statement+&quot; ORDER BY id&quot;, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;kind, &amp;owner, &amp;org, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>kind != &quot;ORGANIZATION&quot; &#124;&#124; org != binding.Organization</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryContext(ctx, &quot;SELECT id,acl_revision,lifecycle FROM knowledge_base WHERE space_id=? ORDER BY id&quot;, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;epoch, &amp;kbState)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>kbState != &quot;ACTIVE&quot; &amp;&amp; kbState != &quot;CLOSED&quot;</code>。

<!-- write-step: 2 bytes=329:2027 -->
````go
// 候选只读取原组织空间；Account组织来源ACK不冻结本人身份或个人空间。
// 外部KB协作者仍可能完成旧写入，必须在来源锁和业务锁后重新核对候选。
func DiscoverOrganizationClose(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	statement := `SELECT id,kind,COALESCE(account_owner_id,0),COALESCE(organization_id,0),lifecycle FROM space WHERE kind='ORGANIZATION' AND organization_id=?`
	args := []any{binding.Organization}
	rows, err := q.QueryContext(ctx, statement+" ORDER BY id", args...)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, owner, org uint64
		var kind, state string
		if err = rows.Scan(&id, &kind, &owner, &org, &state); err != nil {
			break
		}
		if kind != "ORGANIZATION" || org != binding.Organization {
			continue
		}
		if state != "ACTIVE" {
			err = errors.New("ACCOUNT_RETENTION_SPACE_TRANSITION_PENDING")
			break
		}
		c.Spaces = append(c.Spaces, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	for _, space := range c.Spaces {
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision,lifecycle FROM knowledge_base WHERE space_id=? ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var id, epoch uint64
			var kbState string
			if err = rows.Scan(&id, &epoch, &kbState); err != nil {
				break
			}
			if kbState != "ACTIVE" && kbState != "CLOSED" {
				err = errors.New("ORGANIZATION_CLOSE_KB_TRANSITION_PENDING")
````

## 第 3 步：DiscoverOrganizationClose（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 60 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryContext(ctx, &quot;SELECT id,acl_revision,publication_epoch FROM document WHERE space_id=? AND lifecycle&lt;&gt;&#x27;PURGED&#x27; ORDER BY id&quot;, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;acl, &amp;publication)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>a.Key &lt; b.Key</code>。条件成立时执行 <code>return -1</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>a.Key &gt; b.Key</code>。条件成立时执行 <code>return 1</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return c, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2027:3219 -->
````go
				break
			}
			c.KnowledgeBases = append(c.KnowledgeBases, id)
			c.Sources = append(c.Sources, domain.LeaseSource{Key: "kb-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return c, err
		}
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision,publication_epoch FROM document WHERE space_id=? AND lifecycle<>'PURGED' ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var id, acl, publication uint64
			if err = rows.Scan(&id, &acl, &publication); err != nil {
				break
			}
			c.Documents = append(c.Documents, id)
			c.Sources = append(c.Sources, domain.LeaseSource{Key: "doc-acl:" + domain.TextID(id), Epoch: domain.TextID(acl)}, domain.LeaseSource{Key: "publication:" + domain.TextID(id), Epoch: domain.TextID(publication)})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return c, err
		}
	}
	slices.Sort(c.KnowledgeBases)
	slices.Sort(c.Documents)
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

## 第 4 步：SaveOrganizationClose

插入位置：第 3 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`func SaveOrganizationClose(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |
| <code>plan</code> | <code>domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ORGANIZATION_CLOSE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=3219:3513 -->
````go
func SaveOrganizationClose(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error {
	return saveRetentionPlan(ctx, tx, operation, raw, plan, "ORGANIZATION_CLOSE")
}

// organization_close_complete.go：保留原实现，按业务合并文件。

````

## 第 5 步：CompleteOrganizationClose

插入位置：第 4 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3513:4075 -->
````go
func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='ORGANIZATION_CLOSE' AND state='CLEANING'", operation)
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

// organization_close_document.go：保留原实现，按业务合并文件。

````

## 第 6 步：LockOrganizationCleanupDocument

插入位置：第 5 步代码之后，紧接追加；完成稿约第 129 行。

来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。

本步声明：`func LockOrganizationCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch 		FROM document WHERE id=? FOR UPDATE`, item.Document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>；<code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>plan</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>在此声明中的实际使用：<code>plan.Binding</code> |
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
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space).Scan(&amp;kind, &amp;state, &amp;organization)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(organization_id,0) 		FROM space WHERE id=? FOR UPDATE`, space)</code>。
- 检查 <code>state != &quot;CLOSING&quot; &#124;&#124; !closingOrganization</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_DOCUMENT_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4075:6077 -->
````go
// 来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
// 不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。
func LockOrganizationCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error {
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
	var organization uint64
	if err = tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(organization_id,0)
		FROM space WHERE id=? FOR UPDATE`, space).Scan(&kind, &state, &organization); err != nil {
		return err
	}
	closingOrganization := kind == "ORGANIZATION" && organization == plan.Binding.Organization
	if state != "CLOSING" || !closingOrganization {
		return errors.New("ACCOUNT_DOCUMENT_SPACE_CHANGED")
	}
	return nil
}

// organization_close_drains.go：保留原实现，按业务合并文件。

````

## 第 7 步：OrganizationCloseDrained

插入位置：第 6 步代码之后，紧接追加；完成稿约第 170 行。

不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。

本步声明：`func OrganizationCloseDrained(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ORGANIZATION_RETENTION&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ORGANIZATION_RETENTION&quot;)</code> |
| <code>plan</code> | <code>domain.OrganizationClosePlan</code> | 使用 <code>domain.OrganizationClosePlan</code> 类型；其定义处的职责：仅复用留存值类型；Binding 永远为组织绑定，没有身份墓碑和个人空间范围。<br>在此声明中的实际使用：<code>plan.Retention</code>；<code>plan.Binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ORGANIZATION_RETENTION&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ORGANIZATION_RETENTION&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ORGANIZATION_RETENTION&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=6077:6574 -->
````go
// 不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
// 调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。
func OrganizationCloseDrained(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) (bool, error) {
	return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, "ORGANIZATION_RETENTION")
}

// organization_close_ready.go：保留原实现，按业务合并文件。

````

## 第 8 步：OrganizationCloseReady

插入位置：第 7 步代码之后，紧接追加；完成稿约第 178 行。

Ready只检查本流程的非终态记录；新原号仍由Account恢复器推动，Doc已有通用来源worker排空。

本步声明：`func OrganizationCloseReady(ctx context.Context, db *sql.DB) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryContext(ctx, `SELECT operation_id,COALESCE(actor_id,0),state,expected_revision,fixed_plan FROM lifecycle_operation 		 WHERE kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state NOT IN (&#x27;SUC …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryContext(ctx, `SELECT operation_id,COALESCE(actor_id,0),state,expected_revision,fixed_plan FROM lifecycle_operation 		 WHERE kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state NOT IN (&#x27;SUC …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;operation, &amp;actor, &amp;state, &amp;epoch, &amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>decodeErr != nil &#124;&#124; plan.Binding.Operation != operation &#124;&#124; plan.Binding.User != actor &#124;&#124; plan.Binding.Epoch != epoch &#124;&#124; (state != &quot;PLANNED&quot; &amp;&amp; state != &quot;CLEANING&quot;)</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count &lt; 100</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=6574:7865 -->
````go
// Ready只检查本流程的非终态记录；新原号仍由Account恢复器推动，Doc已有通用来源worker排空。
func OrganizationCloseReady(ctx context.Context, db *sql.DB) error {
	var after string
	for {
		rows, err := db.QueryContext(ctx, `SELECT operation_id,COALESCE(actor_id,0),state,expected_revision,fixed_plan FROM lifecycle_operation
		 WHERE kind='ORGANIZATION_CLOSE' AND state NOT IN ('SUCCEEDED','REJECTED') AND operation_id>? ORDER BY operation_id LIMIT 100`, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var operation, state string
			var actor, epoch uint64
			var raw []byte
			if err = rows.Scan(&operation, &actor, &state, &epoch, &raw); err != nil {
				break
			}
			plan, decodeErr := domain.DecodeOrganizationClosePlan(raw)
			if decodeErr != nil || plan.Binding.Operation != operation || plan.Binding.User != actor || plan.Binding.Epoch != epoch || (state != "PLANNED" && state != "CLEANING") {
				err = errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
				break
			}
			after, count = operation, count+1
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if count < 100 {
			return nil
		}
	}
}

// organization_close_source.go：保留原实现，按业务合并文件。

````

## 第 9 步：OrganizationCloseGateMatches

插入位置：第 8 步代码之后，紧接追加；完成稿约第 217 行。

原组织来源由 Account 的真实 FenceAccess 安装，不写入身份gate或伪造墓碑。

本步声明：`func OrganizationCloseGateMatches(ctx context.Context, q Queryer, b platform.OrganizationClosureBinding) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? FROM access_gate WHERE scope_key=?`, b.Epoch, b.Operation, b.Scope())</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Epoch</code>；<code>b.Operation</code>；<code>b.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>matches</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? FROM access_gate WHERE scope_key=?`, b.Epoch, b.Operation, b.Scope()).Scan(&amp;matches)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id=? FROM access_gate WHERE scope_key=?`, b.Epoch, b.Operation, b.Scope())</code>。
- 此处通过 <code>return matches, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7865:8308 -->
````go
// 原组织来源由 Account 的真实 FenceAccess 安装，不写入身份gate或伪造墓碑。
func OrganizationCloseGateMatches(ctx context.Context, q Queryer, b platform.OrganizationClosureBinding) (bool, error) {
	var matches bool
	err := q.QueryRowContext(ctx, `SELECT max_epoch=? AND state='BLOCKED' AND last_revocation_id=? FROM access_gate WHERE scope_key=?`, b.Epoch, b.Operation, b.Scope()).Scan(&matches)
	return matches, err
}

````

## 第 10 步：RecordOrganizationClose

插入位置：第 9 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func RecordOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, raw []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 	 VALUES(?,?,&#x27;ORGANIZATION_CLOSE&#x27;,&#x27;FROZEN&#x27;,?,?)`, b.Operation, b.User, b.Epoch, string(raw))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Operation</code>；<code>b.User</code>；<code>b.Epoch</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan) 	 VALUES(?,?,&#x27;ORGANIZATION_CLOSE&#x27;,&#x27;FROZEN&#x27;,?,?)`, b.Operation, b.User, b.Epoch, string(raw))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=8308:8664 -->
````go
func RecordOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, raw []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
	 VALUES(?,?,'ORGANIZATION_CLOSE','FROZEN',?,?)`, b.Operation, b.User, b.Epoch, string(raw))
	return err
}

````

## 第 11 步：StartOrganizationCleaning

插入位置：第 10 步代码之后，紧接追加；完成稿约第 230 行。

本步声明：`func StartOrganizationCleaning(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;PLANNED&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;PLANNED&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;CLEANING&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ORGANIZATION_CLOSE&#x27; AND state=&#x27;PLANNED&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8664:8956 -->
````go
func StartOrganizationCleaning(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING',generation=generation+1 WHERE operation_id=? AND kind='ORGANIZATION_CLOSE' AND state='PLANNED'", operation)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/organization_lifecycle_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
