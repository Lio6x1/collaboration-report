# 分步编写 `services/document/internal/repository/document/account_retention_queries.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/142-account-retention-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

固定账号关闭后的Doc/KB/Space留存范围并核真实排空与完成。

账号身份排空不会自动冻结外部协作者，所以候选须在来源与实体锁后当前读复核。

写之前先按这条执行路线理解各段的连接：

1. DiscoverAccountRetention找候选
2. LockAccountRetentionEntities先全部来源后Doc/KB/Space
3. SaveAccountRetention存计划
4. AccountRetentionDrained核task/gate/两端ACK
5. CompleteAccountRetentionEntities只开放固定公开版。

本文件需要守住的边界：

- 不能只数ACK
- 私有来源继续BLOCKED
- 更高公开代次不意味着文档恢复可编辑。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/account_retention_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/account_retention_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/account_retention_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/account_retention_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:430 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
)

// account_retention.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetentionCandidates

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type AccountRetentionCandidates struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Spaces</code> | <code>&#91;&#93;uint64</code> | 注销计划涉及的候选空间编号。 |
| <code>KnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 计划涉及的候选知识库编号。 |
| <code>Documents</code> | <code>&#91;&#93;uint64</code> | 计划涉及的候选文档编号，进入锁内后还需重新核验。 |
| <code>Sources</code> | <code>&#91;&#93;domain.LeaseSource</code> | 使用 <code>domain.LeaseSource</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。 |

<!-- write-step: 2 bytes=430:586 -->
````go
type AccountRetentionCandidates struct {
	Spaces         []uint64
	KnowledgeBases []uint64
	Documents      []uint64
	Sources        []domain.LeaseSource
}

````

## 第 3 步：DiscoverAccountRetention

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

候选读取不授予清理资格。Account已确认身份排空，因此自有个人空间和唯一成员
组织不能再合法建库/建文档；现有协作者的修改仍须在来源锁后当前读复核。

本步声明：`func DiscoverAccountRetention(ctx context.Context, q Queryer, binding domain.AccountRetirementBinding) (AccountRetentionCandidates, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, statement+&quot; ORDER BY id&quot;, args...)</code>；<code>q.QueryContext(ctx, &quot;SELECT id,acl_revision FROM knowledge_base WHERE space_id=? ORDER BY id&quot;, space)</code>；<code>q.QueryContext(ctx, &quot;SELECT id,acl_revision,publication_epoch FROM document WHERE space_id=? AND lifecycle&lt;&gt;&#x27;PURGED&#x27; ORDER BY id&quot;, space)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>binding</code> | <code>domain.AccountRetirementBinding</code> | 使用 <code>domain.AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>binding.User</code>；<code>binding.ClosingOrganizations</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionCandidates</code> | 实际返回表达式：<code>c</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(binding.ClosingOrganizations) &gt; 0</code>。
- 执行 <code>q.QueryContext(ctx, statement+&quot; ORDER BY id&quot;, args...)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;kind, &amp;owner, &amp;org, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>kind == &quot;ORGANIZATION&quot; &amp;&amp; !slices.Contains(binding.ClosingOrganizations, org)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>state != &quot;ACTIVE&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。
- 执行 <code>q.QueryContext(ctx, &quot;SELECT id,acl_revision FROM knowledge_base WHERE space_id=? ORDER BY id&quot;, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return c, err</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=586:2356 -->
````go
// 候选读取不授予清理资格。Account已确认身份排空，因此自有个人空间和唯一成员
// 组织不能再合法建库/建文档；现有协作者的修改仍须在来源锁后当前读复核。
func DiscoverAccountRetention(ctx context.Context, q Queryer, binding domain.AccountRetirementBinding) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	statement := `SELECT id,kind,COALESCE(account_owner_id,0),COALESCE(organization_id,0),lifecycle FROM space
		WHERE (kind='PERSONAL' AND account_owner_id=?)`
	args := []any{binding.User}
	if len(binding.ClosingOrganizations) > 0 {
		statement += " OR (kind='ORGANIZATION' AND organization_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(binding.ClosingOrganizations)), ",") + "))"
		for _, org := range binding.ClosingOrganizations {
			args = append(args, org)
		}
	}
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
		if kind == "ORGANIZATION" && !slices.Contains(binding.ClosingOrganizations, org) {
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
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision FROM knowledge_base WHERE space_id=? ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
````

## 第 4 步：DiscoverAccountRetention（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 73 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 执行 <code>rows.Scan(&amp;id, &amp;epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
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

<!-- write-step: 4 bytes=2356:3603 -->
````go
			var id, epoch uint64
			if err = rows.Scan(&id, &epoch); err != nil {
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
````

## 第 5 步：DiscoverAccountRetention（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 118 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return c, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3603:3621 -->
````go
	return c, nil
}

````

## 第 6 步：LockAccountRetentionEntities

插入位置：第 5 步代码之后，紧接追加；完成稿约第 121 行。

所有来源锁先于所有Doc、KB、Space锁。避免逐文档读取父对象导致跨文档锁序倒置。

本步声明：`func LockAccountRetentionEntities(ctx context.Context, tx *sql.Tx, c AccountRetentionCandidates) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+group.table+&quot; WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>c</code> | <code>AccountRetentionCandidates</code> | 在此声明中的实际使用：<code>c.Documents</code>；<code>c.KnowledgeBases</code>；<code>c.Spaces</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+group.table+&quot; WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;actual)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+group.table+&quot; WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3621:4217 -->
````go
// 所有来源锁先于所有Doc、KB、Space锁。避免逐文档读取父对象导致跨文档锁序倒置。
func LockAccountRetentionEntities(ctx context.Context, tx *sql.Tx, c AccountRetentionCandidates) error {
	for _, group := range []struct {
		table string
		ids   []uint64
	}{{"document", c.Documents}, {"knowledge_base", c.KnowledgeBases}, {"space", c.Spaces}} {
		for _, id := range group.ids {
			var actual uint64
			if err := tx.QueryRowContext(ctx, "SELECT id FROM "+group.table+" WHERE id=? FOR UPDATE", id).Scan(&actual); err != nil {
				return err
			}
		}
	}
	return nil
}

````

## 第 7 步：SaveAccountRetention

插入位置：第 6 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func SaveAccountRetention(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |
| <code>plan</code> | <code>domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return saveRetentionPlan(ctx, tx, operation, raw, plan, &quot;ACCOUNT_CLOSE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4217:4425 -->
````go
func SaveAccountRetention(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error {
	return saveRetentionPlan(ctx, tx, operation, raw, plan, "ACCOUNT_CLOSE")
}

````

## 第 8 步：saveRetentionPlan

插入位置：第 7 步代码之后，紧接追加；完成稿约第 141 行。

调用方持有原来源/实体锁；这里只复用写入，不能另开事务或隐式提交。

本步声明：`func saveRetentionPlan(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan, kind string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,main_locked=TRUE,acl_revision=acl_revision+1, 			publication_epoch=publication_epoch+1,control_revision=control_revision+1 WHERE id=?`, d.Document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSING&#x27;,acl_revision=acl_revision+1 WHERE id=?&quot;, kb)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE space SET lifecycle=&#x27;CLOSING&#x27; WHERE id=?&quot;, space)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;PLANNED&#x27;,fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state=&#x27;FROZEN&#x27;&quot;, string(raw), operation, kind)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;PLANNED&#x27;,fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state=&#x27;FROZEN&#x27;&quot;, string(raw), operation, kind)</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(raw)</code> |
| <code>plan</code> | <code>domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Documents</code>；<code>plan.KnowledgeBases</code>；<code>plan.Spaces</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;PLANNED&#x27;,fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state=&#x27;FROZEN&#x27;&quot;, string(raw), operation, kind)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE document SET lifecycle=&#x27;CLOSING&#x27;,main_locked=TRUE,acl_revision=acl_revision+1, 			publication_epoch=publication_epoch+1,control_revision=control_revision+1 WHERE id=?`, d.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSING&#x27;,acl_revision=acl_revision+1 WHERE id=?&quot;, kb)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE space SET lifecycle=&#x27;CLOSING&#x27; WHERE id=?&quot;, space)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;PLANNED&#x27;,fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state=&#x27;FROZEN&#x27;&quot;, string(raw), operation, kind)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4425:5811 -->
````go
// 调用方持有原来源/实体锁；这里只复用写入，不能另开事务或隐式提交。
func saveRetentionPlan(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan, kind string) error {
	for _, d := range plan.Documents {
		if _, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',main_locked=TRUE,acl_revision=acl_revision+1,
			publication_epoch=publication_epoch+1,control_revision=control_revision+1 WHERE id=?`, d.Document); err != nil {
			return err
		}
	}
	for _, kb := range plan.KnowledgeBases {
		if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSING',acl_revision=acl_revision+1 WHERE id=?", kb); err != nil {
			return err
		}
	}
	for _, space := range plan.Spaces {
		if _, err := tx.ExecContext(ctx, "UPDATE space SET lifecycle='CLOSING' WHERE id=?", space); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='PLANNED',fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state='FROZEN'", string(raw), operation, kind)
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

// account_retention_complete.go：保留原实现，按业务合并文件。

````

## 第 9 步：LockAccountRetentionCompletion

插入位置：第 8 步代码之后，紧接追加；完成稿约第 175 行。

所有来源和原lifecycle行均已锁定，先锁全部Doc→KB→Space；读取不会自行
选择新文档或新的公开版本。保留原metadata行，上传恢复仍使用原document绑定。

本步声明：`func LockAccountRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockAccountRetentionEntities(ctx, tx, candidates)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM knowledge_base WHERE id=?&quot;, kb)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM space WHERE id=?&quot;, space)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockAccountRetentionEntities(ctx, tx, candidates)</code>；<code>tx.QueryRowContext</code> |
| <code>plan</code> | <code>domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Spaces</code>；<code>plan.KnowledgeBases</code>；<code>plan.Documents</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_PARENT_CHANGED&quot;)</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_SPACE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM knowledge_base WHERE id=?&quot;, kb).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM knowledge_base WHERE id=?&quot;, kb)</code>。
- 检查 <code>state != &quot;CLOSING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_PARENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM space WHERE id=?&quot;, space).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle FROM space WHERE id=?&quot;, space)</code>。
- 检查 <code>state != &quot;CLOSING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_SPACE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5811:7006 -->
````go
// 所有来源和原lifecycle行均已锁定，先锁全部Doc→KB→Space；读取不会自行
// 选择新文档或新的公开版本。保留原metadata行，上传恢复仍使用原document绑定。
func LockAccountRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error {
	candidates := AccountRetentionCandidates{Spaces: plan.Spaces, KnowledgeBases: plan.KnowledgeBases}
	for _, item := range plan.Documents {
		candidates.Documents = append(candidates.Documents, item.Document)
	}
	if err := LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	for _, kb := range plan.KnowledgeBases {
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM knowledge_base WHERE id=?", kb).Scan(&state); err != nil {
			return err
		}
		if state != "CLOSING" {
			return errors.New("ACCOUNT_RETENTION_PARENT_CHANGED")
		}
	}
	for _, space := range plan.Spaces {
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM space WHERE id=?", space).Scan(&state); err != nil {
			return err
		}
		if state != "CLOSING" {
			return errors.New("ACCOUNT_RETENTION_SPACE_CHANGED")
		}
	}
	return nil
}

````

## 第 10 步：CompleteAccountRetentionEntities

插入位置：第 9 步代码之后，紧接追加；完成稿约第 206 行。

只在全部原清理回执及固定依赖核验之后调用。更高代次只开放已固定公开
正式版的输出；原身份/私有文档来源保持BLOCKED，内部写入仍被lifecycle拒绝。

本步声明：`func CompleteAccountRetentionEntities(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)),purge_after=NULL WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, item.Document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT acl_revision,publication_epoch FROM document WHERE id=?&quot;, item.Document)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PUBLIC_RETAINED&#x27;,acl_revision=?,publication_epoch=?,main_locked=TRUE,deleted_at=NULL,purge_after=NULL WHERE id=? AND lifecycle=&#x27; …</code>（长表达式，按本步完整代码继续）；<code>ReopenSourceGate(ctx, tx, source.key, source.epoch)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code>；<code>ReopenSourceGate(ctx, tx, source.key, source.epoch)</code>；<code>ReopenSourceGate(ctx, tx, &quot;kb-acl:&quot;+domain.TextID(kb), acl+1)</code> |
| <code>plan</code> | <code>domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Documents</code>；<code>plan.KnowledgeBases</code>；<code>plan.Spaces</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_EPOCH_EXHAUSTED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>item.Release == 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PURGED&#x27;,deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)),purge_after=NULL WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, item.Document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>item.KnowledgeBase != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT acl_revision,publication_epoch FROM document WHERE id=?&quot;, item.Document).Scan(&amp;acl, &amp;publication)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT acl_revision,publication_epoch FROM document WHERE id=?&quot;, item.Document)</code>。
- 检查 <code>acl == math.MaxUint64 &#124;&#124; publication == math.MaxUint64</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;PUBLIC_RETAINED&#x27;,acl_revision=?,publication_epoch=?,main_locked=TRUE,deleted_at=NULL,purge_after=NULL WHERE id=? AND lifecycle=&#x27; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>retainedKB&#91;kb&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, kb).Scan(&amp;acl)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT acl_revision FROM knowledge_base WHERE id=?&quot;, kb)</code>。
- 检查 <code>acl == math.MaxUint64</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;ACCOUNT_RETENTION_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=7006:9137 -->
````go
// 只在全部原清理回执及固定依赖核验之后调用。更高代次只开放已固定公开
// 正式版的输出；原身份/私有文档来源保持BLOCKED，内部写入仍被lifecycle拒绝。
func CompleteAccountRetentionEntities(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error {
	retainedKB, retainedSpace := map[uint64]bool{}, map[uint64]bool{}
	for _, item := range plan.Documents {
		if item.Release == 0 {
			if _, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)),purge_after=NULL WHERE id=? AND lifecycle='CLOSING'", item.Document); err != nil {
				return err
			}
			continue
		}
		retainedSpace[item.Space] = true
		if item.KnowledgeBase != 0 {
			retainedKB[item.KnowledgeBase] = true
		}
		var acl, publication uint64
		if err := tx.QueryRowContext(ctx, "SELECT acl_revision,publication_epoch FROM document WHERE id=?", item.Document).Scan(&acl, &publication); err != nil {
			return err
		}
		if acl == math.MaxUint64 || publication == math.MaxUint64 {
			return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PUBLIC_RETAINED',acl_revision=?,publication_epoch=?,main_locked=TRUE,deleted_at=NULL,purge_after=NULL WHERE id=? AND lifecycle='CLOSING'", acl+1, publication+1, item.Document); err != nil {
			return err
		}
		for _, source := range []struct {
			key   string
			epoch uint64
		}{{"doc-acl:" + domain.TextID(item.Document), acl + 1}, {"publication:" + domain.TextID(item.Document), publication + 1}} {
			if err := ReopenSourceGate(ctx, tx, source.key, source.epoch); err != nil {
				return err
			}
		}
	}
	for _, kb := range plan.KnowledgeBases {
		state := "CLOSED"
		if retainedKB[kb] {
			state = "PUBLIC_RETAINED"
			var acl uint64
			if err := tx.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=?", kb).Scan(&acl); err != nil {
				return err
			}
			if acl == math.MaxUint64 {
				return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
			}
````

## 第 11 步：CompleteAccountRetentionEntities（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 251 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET acl_revision=? WHERE id=?&quot;, acl+1, kb)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=?,name=IF(?=&#x27;CLOSED&#x27;,&#x27;已关闭知识库&#x27;,name) WHERE id=?&quot;, state, state, kb)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE &quot;+table+&quot; SET state=&#x27;CLOSED&#x27;,revision=revision+1 WHERE knowledge_base_id=? AND state&lt;&gt;&#x27;CLOSED&#x27;&quot;, kb)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>retainedSpace&#91;space&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE space SET lifecycle=? WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, state, space)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=9137:10234 -->
````go
			if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET acl_revision=? WHERE id=?", acl+1, kb); err != nil {
				return err
			}
			if err := ReopenSourceGate(ctx, tx, "kb-acl:"+domain.TextID(kb), acl+1); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle=?,name=IF(?='CLOSED','已关闭知识库',name) WHERE id=?", state, state, kb); err != nil {
			return err
		}
		// 关闭父库的其他外部成员也不能继续持有可变管理关系；不清除存续库。
		for _, table := range []string{"knowledge_base_member", "knowledge_base_creation_grant"} {
			if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND state<>'CLOSED'", kb); err != nil {
				return err
			}
		}
	}
	for _, space := range plan.Spaces {
		state := "CLOSED"
		if retainedSpace[space] {
			state = "PUBLIC_RETAINED"
		}
		if _, err := tx.ExecContext(ctx, "UPDATE space SET lifecycle=? WHERE id=? AND lifecycle='CLOSING'", state, space); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 12 步：CompleteAccountRetention

插入位置：第 11 步代码之后，紧接追加；完成稿约第 280 行。

本步声明：`func CompleteAccountRetention(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE lifecycle_operation SET state=&#x27;SUCCEEDED&#x27;,generation=generation+1 WHERE operation_id=? AND kind=&#x27;ACCOUNT_CLOSE&#x27; AND state=&#x27;CLEANING&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>changed != 1</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_RETENTION_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=10234:10787 -->
````go
func CompleteAccountRetention(ctx context.Context, tx *sql.Tx, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='CLEANING'", operation)
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

// account_retention_drains.go：保留原实现，按业务合并文件。

````

## 第 13 步：AccountRetentionDrained

插入位置：第 12 步代码之后，紧接追加；完成稿约第 297 行。

不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。

本步声明：`func AccountRetentionDrained(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ACCOUNT_RETENTION&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ACCOUNT_RETENTION&quot;)</code> |
| <code>plan</code> | <code>domain.AccountRetirementPlan</code> | 使用 <code>domain.AccountRetirementPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>plan.Retention</code>；<code>plan.Binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ACCOUNT_RETENTION&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ACCOUNT_RETENTION&quot;)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, &quot;ACCOUNT_RETENTION&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=10787:11201 -->
````go
// 不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
// 调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。
func AccountRetentionDrained(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) (bool, error) {
	return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, "ACCOUNT_RETENTION")
}

````

## 第 14 步：retentionDrained

插入位置：第 13 步代码之后，紧接追加；完成稿约第 303 行。

本步声明：`func retentionDrained(ctx context.Context, tx *sql.Tx, retention *domain.AccountRetentionPlan, user uint64, action string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.ac …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operation, participant, scope.Key, scope.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>retention</code> | <code>*domain.AccountRetentionPlan</code> | 使用 <code>domain.AccountRetentionPlan</code> 类型；其定义处的职责：定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。<br>在此声明中的实际使用：<code>retention == nil</code>；<code>retention.Scopes</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>actor == user</code> |
| <code>action</code> | <code>string</code> | 在此声明中的实际使用：<code>closure.Action == action</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>；<code>nil</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>retention == nil</code>。条件成立时执行 <code>return false, errors.New(&quot;ACCOUNT_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.ac …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,g.max_epoch,g.state,COALESCE(g.last_revocation_id,&#x27;&#x27;),COALESCE(c.ac …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(payload, &amp;closure) != nil &#124;&#124; json.Unmarshal(target, &amp;binding) != nil &#124;&#124; json.Unmarshal(participants, &amp;participantPlan) != nil</code>。条件成立时执行 <code>return false, errors.New(&quot;ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!taskMatches &#124;&#124; !authorizationMatches &#124;&#124; !gateMatches &#124;&#124; !digestMatches</code>。条件成立时执行 <code>return false, errors.New(&quot;ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>taskState != &quot;SUCCEEDED&quot; &#124;&#124; authState != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operatio …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, scope.Operation, participant, scope.Key, scope.Epoch)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(digest, platform.BootstrapDigest(request, participant))</code>。条件成立时执行 <code>return false, errors.New(&quot;ACCOUNT_RETENTION_ACK_CHANGED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=11201:14211 -->
````go
func retentionDrained(ctx context.Context, tx *sql.Tx, retention *domain.AccountRetentionPlan, user uint64, action string) (bool, error) {
	if retention == nil {
		return false, errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	for _, scope := range retention.Scopes {
		var taskOperation, taskState, authState, key, gateState, revocation string
		var epoch, gateEpoch, actor uint64
		var payload, target, participants []byte
		err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
			FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
			WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, scope.Operation).Scan(&taskOperation, &taskState, &payload, &authState, &key, &epoch, &target, &gateEpoch, &gateState, &revocation, &actor, &participants)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var closure domain.Closure
		var binding map[string]string
		var participantPlan []string
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		if json.Unmarshal(payload, &closure) != nil || json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &participantPlan) != nil {
			return false, errors.New("ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED")
		}
		taskMatches := taskOperation == scope.Operation && closure.Scope == scope.Key && closure.Epoch == scope.Epoch && closure.Action == action && closure.Entity == 0
		authorizationMatches := actor == user && key == scope.Key && epoch == scope.Epoch && slices.Equal(participantPlan, []string{"edge", "collaboration"})
		gateMatches := gateEpoch == scope.Epoch && gateState == "BLOCKED" && revocation == scope.Operation
		digestMatches := binding["generation"] == "1" && binding["bootstrap_target_digest"] == hex.EncodeToString(request.TargetDigest)
		if !taskMatches || !authorizationMatches || !gateMatches || !digestMatches {
			return false, errors.New("ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED")
		}
		if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
			return false, nil
		}
		for _, participant := range []string{"edge", "collaboration"} {
			var digest []byte
			err := tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", scope.Operation, participant, scope.Key, scope.Epoch).Scan(&digest)
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
				return false, errors.New("ACCOUNT_RETENTION_ACK_CHANGED")
			}
		}
	}
````

## 第 15 步：retentionDrained（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 351 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=14211:14231 -->
````go
	return true, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/account_retention_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
