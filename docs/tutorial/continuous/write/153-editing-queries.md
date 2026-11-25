# 分步编写 `services/document/internal/repository/document/editing_queries.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/153-editing-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存编辑票据、协作房间、支线基线，并核算持久正文容量。

票据兑换、空间容量与版本写入必须在调用方同一事务内，否则会超额或发出无正文的编辑资格。

写之前先按这条执行路线理解各段的连接：

1. LockDocumentCapacity按空间额度后资源顺序锁定
2. ChargeStoredVersion随真实版本INSERT收费
3. LockedEditingTicket锁一次票据
4. SharedContentVersion依据业务引用证明共享范围。

本文件需要守住的边界：

- 不可凭可猜版本ID或source_kind开放草稿
- 恢复先确认原版本存在，避免重复收费。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/editing_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/editing_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/editing_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/editing_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:364 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
	"errors"
	"time"
)

// content_capacity.go：保留原实现，按业务合并文件。

````

## 第 2 步：LockDocumentCapacity

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

调用方已锁 Doc/KB。先锁空间与额度，再锁资源；不可在资源之后首次取得额度锁。
runtime 配置只在创建/首次空间物化传入，历史写入使用空间实际持久上限。

本步声明：`func LockDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id FROM document WHERE id=?&quot;, document)</code>；<code>SpaceRow(ctx, tx, space, true)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space)</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>SpaceRow(ctx, tx, space, true)</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT space_id FROM document WHERE id=?&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DOCUMENT_QUOTA_NOT_INITIALIZED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id FROM document WHERE id=?&quot;, document).Scan(&amp;space)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT space_id FROM document WHERE id=?&quot;, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DOCUMENT_QUOTA_NOT_INITIALIZED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space).Scan(&amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT limit_bytes FROM quota_account WHERE space_id=?&quot;, space)</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=364:1149 -->
````go
// 调用方已锁 Doc/KB。先锁空间与额度，再锁资源；不可在资源之后首次取得额度锁。
// runtime 配置只在创建/首次空间物化传入，历史写入使用空间实际持久上限。
func LockDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT space_id FROM document WHERE id=?", document).Scan(&space); err != nil {
		return err
	}
	if _, err := SpaceRow(ctx, tx, space, true); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", space).Scan(&limit); err != nil {
		return errors.New("DOCUMENT_QUOTA_NOT_INITIALIZED")
	}
	_, err := resources.LockQuota(ctx, tx, space, limit)
	return err
}

````

## 第 3 步：ChargeStoredVersion

插入位置：第 2 步代码之后，紧接追加；完成稿约第 32 行。

与真实 INSERT 使用同一事务；结果未知的原恢复器先核对原版本，不生成第二笔费用。

本步声明：`func ChargeStoredVersion(ctx context.Context, tx *sql.Tx, document, bytes uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>；<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>；<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |
| <code>bytes</code> | <code>uint64</code> | 在此声明中的实际使用：<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document).Scan(&amp;space, &amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>。
- 此处通过 <code>return resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1149:1700 -->
````go
// 与真实 INSERT 使用同一事务；结果未知的原恢复器先核对原版本，不生成第二笔费用。
func ChargeStoredVersion(ctx context.Context, tx *sql.Tx, document, bytes uint64, operation string) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?", document).Scan(&space, &limit); err != nil {
		return err
	}
	return resources.ChargeDocumentCapacity(ctx, tx, document, space, bytes, limit, operation)
}

````

## 第 4 步：ReconcileCleanedDocumentCapacity

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

Node 的真实原 CLEANED 已由上层确认。保留公开版本继续计费，删除的历史和
已排空的 Node 存储才退款；整份删除时同事务移除容量行。

本步声明：`func ReconcileCleanedDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE&quot;, document, space)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL&quot;, document)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>resources.LockQuota(ctx, tx, space, limit)</code>；<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE&quot;, document, space)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL&quot;, document)</code>；<code>tx.ExecContext(ctx, &quot;DELETE FROM document_capacity WHERE document_id=?&quot;, document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DOCUMENT_CAPACITY_INCONSISTENT&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;DOCUMENT_CAPACITY_REVISION_CHANGED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document).Scan(&amp;space, &amp;limit)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?&quot;, document)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE&quot;, document, space).Scan(&amp;paid)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE&quot;, document, space)</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL&quot;, document).Scan(&amp;remaining)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL&quot;, document)</code>。
- 检查 <code>remaining &gt; paid &#124;&#124; paid-remaining &gt; q.Used</code>。条件成立时执行 <code>return errors.New(&quot;DOCUMENT_CAPACITY_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>paid == 0 &amp;&amp; remaining == 0</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE quota_account SET used_bytes=used_bytes-?,revision=revision+1 WHERE space_id=? AND revision=? AND revision&lt;18446744073709551615&quot;, paid-remaining, space, q.Revision)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;DOCUMENT_CAPACITY_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>remaining == 0</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM document_capacity WHERE document_id=?&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_capacity SET bytes=?,node_bytes=0 WHERE document_id=?&quot;, remaining, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1700:3653 -->
````go
// Node 的真实原 CLEANED 已由上层确认。保留公开版本继续计费，删除的历史和
// 已排空的 Node 存储才退款；整份删除时同事务移除容量行。
func ReconcileCleanedDocumentCapacity(ctx context.Context, tx *sql.Tx, document uint64) error {
	var space, limit uint64
	if err := tx.QueryRowContext(ctx, "SELECT d.space_id,q.limit_bytes FROM document d JOIN quota_account q ON q.space_id=d.space_id WHERE d.id=?", document).Scan(&space, &limit); err != nil {
		return err
	}
	q, err := resources.LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var paid uint64
	err = tx.QueryRowContext(ctx, "SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE", document, space).Scan(&paid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	var remaining uint64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL", document).Scan(&remaining); err != nil {
		return err
	}
	if remaining > paid || paid-remaining > q.Used {
		return errors.New("DOCUMENT_CAPACITY_INCONSISTENT")
	}
	if paid == 0 && remaining == 0 {
		return nil
	}
	changed, err := tx.ExecContext(ctx, "UPDATE quota_account SET used_bytes=used_bytes-?,revision=revision+1 WHERE space_id=? AND revision=? AND revision<18446744073709551615", paid-remaining, space, q.Revision)
	if err != nil {
		return err
	}
	if n, err := changed.RowsAffected(); err != nil || n != 1 {
		return errors.New("DOCUMENT_CAPACITY_REVISION_CHANGED")
	}
	if remaining == 0 {
		_, err = tx.ExecContext(ctx, "DELETE FROM document_capacity WHERE document_id=?", document)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE document_capacity SET bytes=?,node_bytes=0 WHERE document_id=?", remaining, document)
	}
	return err
}

// content_versions.go：保留原实现，按业务合并文件。

````

## 第 5 步：ContentVersionScope

插入位置：第 4 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`func ContentVersionScope(ctx context.Context, q Queryer, id uint64) (domain.ContentVersionScope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?&quot;, id)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ContentVersionScope</code> | 使用 <code>domain.ContentVersionScope</code> 类型；其定义处的职责：版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。<br>实际返回表达式：<code>scope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?&quot;, id).Scan(&amp;scope.Version, &amp;scope.Document, &amp;scope.Unit, &amp;scope.Structure)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?&quot;, id)</code>。
- 此处通过 <code>return scope, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=3653:4001 -->
````go
func ContentVersionScope(ctx context.Context, q Queryer, id uint64) (domain.ContentVersionScope, error) {
	var scope domain.ContentVersionScope
	err := q.QueryRowContext(ctx, "SELECT id,document_id,unit_id,structure_id FROM content_version WHERE id=?", id).Scan(&scope.Version, &scope.Document, &scope.Unit, &scope.Structure)
	return scope, err
}

````

## 第 6 步：SharedContentVersion

插入位置：第 5 步代码之后，紧接追加；完成稿约第 90 行。

只从实际业务引用证明这是共享审核/主线上下文，source_kind或可猜到的ID不授予草稿访问。

本步声明：`func SharedContentVersion(ctx context.Context, q Queryer, scope domain.ContentVersionScope) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM review_round WHERE document_id=? AND unit_id=? AND content_version_id=?) 		OR EXISTS(SELECT 1 FROM manifest_entry WHERE docume …</code>（长表达式，按本步完整代码继续） |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>scope</code> | <code>domain.ContentVersionScope</code> | 使用 <code>domain.ContentVersionScope</code> 类型；其定义处的职责：版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。<br>在此声明中的实际使用：<code>scope.Document</code>；<code>scope.Unit</code>；<code>scope.Version</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>shared</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM review_round WHERE document_id=? AND unit_id=? AND content_version_id=?) 		OR EXISTS(SELECT 1 FROM manifest_entry WHERE docume …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, `SELECT 		EXISTS(SELECT 1 FROM review_round WHERE document_id=? AND unit_id=? AND content_version_id=?) 		OR EXISTS(SELECT 1 FROM manifest_entry WHERE docume …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return shared, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4001:4679 -->
````go
// 只从实际业务引用证明这是共享审核/主线上下文，source_kind或可猜到的ID不授予草稿访问。
func SharedContentVersion(ctx context.Context, q Queryer, scope domain.ContentVersionScope) (bool, error) {
	var shared bool
	err := q.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM review_round WHERE document_id=? AND unit_id=? AND content_version_id=?)
		OR EXISTS(SELECT 1 FROM manifest_entry WHERE document_id=? AND unit_id=? AND content_version_id=?)`,
		scope.Document, scope.Unit, scope.Version, scope.Document, scope.Unit, scope.Version).Scan(&shared)
	return shared, err
}

// edit_bootstrap.go：保留原实现，按业务合并文件。

````

## 第 7 步：LockedEditingStructure

插入位置：第 6 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`func LockedEditingStructure(ctx context.Context, tx *sql.Tx, document uint64) (string, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>lifecycle</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>structure</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE&quot;, document).Scan(&amp;lifecycle, &amp;structure)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE&quot;, document)</code>。
- 此处通过 <code>return lifecycle, structure, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4679:5011 -->
````go
func LockedEditingStructure(ctx context.Context, tx *sql.Tx, document uint64) (string, uint64, error) {
	var lifecycle string
	var structure uint64
	err := tx.QueryRowContext(ctx, "SELECT lifecycle,current_structure_id FROM document WHERE id=? FOR UPDATE", document).Scan(&lifecycle, &structure)
	return lifecycle, structure, err
}
````

## 第 8 步：BranchBootstrapBody

插入位置：第 7 步代码之后，紧接追加；完成稿约第 108 行。

本步声明：`func BranchBootstrapBody(ctx context.Context, tx *sql.Tx, workspace, document, unit, epoch uint64) ([]byte, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>workspace</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>body</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_ve …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return body, version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5011:5694 -->
````go
func BranchBootstrapBody(ctx context.Context, tx *sql.Tx, workspace, document, unit, epoch uint64) ([]byte, uint64, error) {
	var body []byte
	var version uint64
	err := tx.QueryRowContext(ctx, "SELECT v.canonical_body,v.id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id JOIN content_version v ON v.id=l.base_version_id AND v.document_id=b.document_id JOIN collaboration_unit u ON u.active_branch_id=b.id AND u.id=b.unit_id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.workspace_epoch=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL", workspace, document, unit, epoch).Scan(&body, &version)
	return body, version, err
}
````

## 第 9 步：MainBootstrapBody

插入位置：第 8 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func MainBootstrapBody(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]byte, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e. …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e. …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e. …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>body</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>version</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e. …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e. …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return body, version, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5694:6146 -->
````go
func MainBootstrapBody(ctx context.Context, tx *sql.Tx, document, unit uint64) ([]byte, uint64, error) {
	var body []byte
	var version uint64
	err := tx.QueryRowContext(ctx, "SELECT v.canonical_body,v.id FROM document d JOIN manifest_entry e ON e.manifest_id=d.current_manifest_id AND e.unit_id=? JOIN content_version v ON v.id=e.content_version_id WHERE d.id=? AND d.main_locked=0", unit, document).Scan(&body, &version)
	return body, version, err
}

````

## 第 10 步：BranchBootstrapLineage

插入位置：第 9 步代码之后，紧接追加；完成稿约第 121 行。

来源上下文随当前编辑基线交付，Node不能自行编造base或已接纳来源。

本步声明：`func BranchBootstrapLineage(ctx context.Context, tx *sql.Tx, branch uint64) (uint64, uint64, []byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.lifecycle=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.lifecycle=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, branch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>lineage</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>manifest</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>provenance</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.l …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.lifecycle=&#x27;ACTIVE&#x27; FOR UPDATE&quot;, branch)</code>。
- 此处通过 <code>return lineage, manifest, provenance, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6146:6782 -->
````go
// 来源上下文随当前编辑基线交付，Node不能自行编造base或已接纳来源。
func BranchBootstrapLineage(ctx context.Context, tx *sql.Tx, branch uint64) (uint64, uint64, []byte, error) {
	var lineage, manifest uint64
	var provenance []byte
	err := tx.QueryRowContext(ctx, "SELECT l.id,l.base_manifest_id,l.provenance_json FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.lifecycle='ACTIVE' FOR UPDATE", branch).Scan(&lineage, &manifest, &provenance)
	return lineage, manifest, provenance, err
}

// edit_session.go：保留原实现，按业务合并文件。

````

## 第 11 步：EnsureCollaborationRoom

插入位置：第 10 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func EnsureCollaborationRoom(ctx context.Context, tx *sql.Tx, key string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;COLLABORATION_ROOM&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE busine …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;COLLABORATION_ROOM&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE busine …</code>（长表达式，按本步完整代码继续） |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;COLLABORATION_ROOM&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE busine …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6782:7137 -->
````go
func EnsureCollaborationRoom(ctx context.Context, tx *sql.Tx, key string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('COLLABORATION_ROOM',?,?,'SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)", key, string(payload))
	return err
}
````

## 第 12 步：LockedCollaborationRoom

插入位置：第 11 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`func LockedCollaborationRoom(ctx context.Context, tx *sql.Tx, key string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? FOR UPDATE&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? FOR UPDATE&quot;, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? FOR UPDATE&quot;, key).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;COLLABORATION_ROOM&#x27; AND business_key=? FOR UPDATE&quot;, key)</code>。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7137:7410 -->
````go
func LockedCollaborationRoom(ctx context.Context, tx *sql.Tx, key string) ([]byte, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='COLLABORATION_ROOM' AND business_key=? FOR UPDATE", key).Scan(&raw)
	return raw, err
}
````

## 第 13 步：LockedActiveBranch

插入位置：第 12 步代码之后，紧接追加；完成稿约第 140 行。

本步声明：`func LockedActiveBranch(ctx context.Context, tx *sql.Tx, unit, document uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE&quot;, unit, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE&quot;, unit, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE&quot;, unit, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>branch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE&quot;, unit, document).Scan(&amp;branch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE&quot;, unit, document)</code>。
- 此处通过 <code>return branch, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7410:7735 -->
````go
func LockedActiveBranch(ctx context.Context, tx *sql.Tx, unit, document uint64) (uint64, error) {
	var branch uint64
	err := tx.QueryRowContext(ctx, "SELECT active_branch_id FROM collaboration_unit WHERE id=? AND document_id=? AND retired_structure_id IS NULL FOR UPDATE", unit, document).Scan(&branch)
	return branch, err
}
````

## 第 14 步：LockedEditWorkspace

插入位置：第 13 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func LockedEditWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockedControlledWorkspace(ctx, tx, branch, document, unit)</code>；<code>RequireBranchControlOpen(ctx, tx, branch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockedControlledWorkspace(ctx, tx, branch, document, unit)</code>；<code>RequireBranchControlOpen(ctx, tx, branch)</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockedControlledWorkspace(ctx, tx, branch, document, unit)</code>；<code>RequireBranchControlOpen(ctx, tx, branch)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockedControlledWorkspace(ctx, tx, branch, document, unit)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>LockedControlledWorkspace(ctx, tx, branch, document, unit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>RequireBranchControlOpen(ctx, tx, branch)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 此处通过 <code>return current, RequireBranchControlOpen(ctx, tx, branch)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=7735:8037 -->
````go
func LockedEditWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error) {

	current, err := LockedControlledWorkspace(ctx, tx, branch, document, unit)
	if err != nil {
		return current, err
	}
	return current, RequireBranchControlOpen(ctx, tx, branch)
}

````

## 第 15 步：LockedControlledWorkspace

插入位置：第 14 步代码之后，紧接追加；完成稿约第 154 行。

已登记控制用例读取自己的旧工作区；普通编辑/送审/合并必须再检查控制占位。

本步声明：`func LockedControlledWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>branch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续） |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b. …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=8037:8738 -->
````go
// 已登记控制用例读取自己的旧工作区；普通编辑/送审/合并必须再检查控制占位。
func LockedControlledWorkspace(ctx context.Context, tx *sql.Tx, branch, document, unit uint64) (domain.EditWorkspace, error) {
	var current domain.EditWorkspace
	err := tx.QueryRowContext(ctx, "SELECT b.id,b.workspace_epoch,l.id,l.base_manifest_id,l.base_version_id FROM branch b JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id WHERE b.id=? AND b.document_id=? AND b.unit_id=? AND b.lifecycle='ACTIVE' FOR UPDATE", branch, document, unit).Scan(&current.Workspace, &current.Epoch, &current.Lineage, &current.BaseManifest, &current.BaseVersion)
	return current, err
}
````

## 第 16 步：ConfigureEditAdmission

插入位置：第 15 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`func ConfigureEditAdmission(ctx context.Context, tx *sql.Tx, unit uint64, admission string, target []byte, expires time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,target_json=?,capabilities=&#x27;&#91;\&quot;EDIT\&quot;,\&quot;READ\&quot;&#93;&#x27;,expires_at=? WHERE admission_id=?&quot;, unit, string(target), expires, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,target_json=?,capabilities=&#x27;&#91;\&quot;EDIT\&quot;,\&quot;READ\&quot;&#93;&#x27;,expires_at=? WHERE admission_id=?&quot;, unit, string(target), expires, admission)</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,target_json=?,capabilities=&#x27;&#91;\&quot;EDIT\&quot;,\&quot;READ\&quot;&#93;&#x27;,expires_at=? WHERE admission_id=?&quot;, unit, string(target), expires, admission)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,target_json=?,capabilities=&#x27;&#91;\&quot;EDIT\&quot;,\&quot;READ\&quot;&#93;&#x27;,expires_at=? WHERE admission_id=?&quot;, unit, string(target), expires, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET unit_id=?,target_json=?,capabilities=&#x27;&#91;\&quot;EDIT\&quot;,\&quot;READ\&quot;&#93;&#x27;,expires_at=? WHERE admission_id=?&quot;, unit, string(target), expires, admission)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8738:9085 -->
````go
func ConfigureEditAdmission(ctx context.Context, tx *sql.Tx, unit uint64, admission string, target []byte, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission SET unit_id=?,target_json=?,capabilities='[\"EDIT\",\"READ\"]',expires_at=? WHERE admission_id=?", unit, string(target), expires, admission)
	return err
}
````

## 第 17 步：InsertEditTicket

插入位置：第 16 步代码之后，紧接追加；完成稿约第 164 行。

本步声明：`func InsertEditTicket(ctx context.Context, tx *sql.Tx, hash []byte, admission, client, kind string, workspace, epoch uint64, expires time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>client</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>workspace</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=9085:9509 -->
````go
func InsertEditTicket(ctx context.Context, tx *sql.Tx, hash []byte, admission, client, kind string, workspace, epoch uint64, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO collaboration_ticket(token_hash,admission_id,client_session_id,workspace_kind,workspace_id,workspace_epoch,state,expires_at) VALUES(?,?,?,?,?,?,'OPEN',?)", hash, admission, client, kind, workspace, epoch, expires)
	return err
}
````

## 第 18 步：InsertEditSessionResult

插入位置：第 17 步代码之后，紧接追加；完成稿约第 168 行。

本步声明：`func InsertEditSessionResult(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;EDIT_SESSION&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(payload))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;EDIT_SESSION&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(payload))</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES(&#x27;EDIT_SESSION&#x27;,?,?,&#x27;SUCCEEDED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, string(payload))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=9509:9812 -->
````go
func InsertEditSessionResult(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('EDIT_SESSION',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))", operation, string(payload))
	return err
}
````

## 第 19 步：EditSessionResult

插入位置：第 18 步代码之后，紧接追加；完成稿约第 172 行。

本步声明：`func EditSessionResult(ctx context.Context, tx *sql.Tx, operation string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;EDIT_SESSION&#x27; AND business_key=?&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;EDIT_SESSION&#x27; AND business_key=?&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>raw</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;EDIT_SESSION&#x27; AND business_key=?&quot;, operation).Scan(&amp;raw)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT payload FROM durable_task WHERE kind=&#x27;EDIT_SESSION&#x27; AND business_key=?&quot;, operation)</code>。
- 此处通过 <code>return raw, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=9812:10074 -->
````go
func EditSessionResult(ctx context.Context, tx *sql.Tx, operation string) ([]byte, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='EDIT_SESSION' AND business_key=?", operation).Scan(&raw)
	return raw, err
}
````

## 第 20 步：TicketAdmission

插入位置：第 19 步代码之后，紧接追加；完成稿约第 177 行。

本步声明：`func TicketAdmission(ctx context.Context, q Queryer, hash []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT admission_id FROM collaboration_ticket WHERE token_hash=?&quot;, hash)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, &quot;SELECT admission_id FROM collaboration_ticket WHERE token_hash=?&quot;, hash)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>admission</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT admission_id FROM collaboration_ticket WHERE token_hash=?&quot;, hash).Scan(&amp;admission)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, &quot;SELECT admission_id FROM collaboration_ticket WHERE token_hash=?&quot;, hash)</code>。
- 此处通过 <code>return admission, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=10074:10327 -->
````go
func TicketAdmission(ctx context.Context, q Queryer, hash []byte) (string, error) {
	var admission string
	err := q.QueryRowContext(ctx, "SELECT admission_id FROM collaboration_ticket WHERE token_hash=?", hash).Scan(&admission)
	return admission, err
}
````

## 第 21 步：LockedEditingTicket

插入位置：第 20 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func LockedEditingTicket(ctx context.Context, tx *sql.Tx, hash []byte) (domain.EditingTicket, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collab …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collab …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditingTicket</code> | 使用 <code>domain.EditingTicket</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>实际返回表达式：<code>ticket</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collab …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collab …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return ticket, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=10327:10919 -->
````go
func LockedEditingTicket(ctx context.Context, tx *sql.Tx, hash []byte) (domain.EditingTicket, error) {
	var ticket domain.EditingTicket
	a := &ticket.Admission
	err := tx.QueryRowContext(ctx, "SELECT a.actor_id,a.session_ref,a.target_json,a.state,a.purpose,a.expires_at,t.client_session_id,t.state,t.expires_at FROM document_admission a JOIN collaboration_ticket t ON t.admission_id=a.admission_id WHERE t.token_hash=? FOR UPDATE", hash).Scan(&a.Actor, &a.SessionRef, &a.TargetJSON, &a.State, &ticket.Purpose, &a.Expires, &ticket.Client, &ticket.State, &ticket.Expires)
	return ticket, err
}
````

## 第 22 步：BindEditingGrant

插入位置：第 21 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`func BindEditingGrant(ctx context.Context, tx *sql.Tx, admission, grant string, hash, target []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=?&quot;, string(target), admission)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket SET state=&#x27;REDEEMED&#x27;,redeemed_grant_ref=? WHERE token_hash=?&quot;, grant, hash)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=?&quot;, string(target), admission)</code> |
| <code>grant</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket SET state=&#x27;REDEEMED&#x27;,redeemed_grant_ref=? WHERE token_hash=?&quot;, grant, hash)</code> |
| <code>hash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket SET state=&#x27;REDEEMED&#x27;,redeemed_grant_ref=? WHERE token_hash=?&quot;, grant, hash)</code> |
| <code>target</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(target)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission SET target_json=? WHERE admission_id=?&quot;, string(target), admission)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket SET state=&#x27;REDEEMED&#x27;,redeemed_grant_ref=? WHERE token_hash=?&quot;, grant, hash)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 22 bytes=10919:11337 -->
````go
func BindEditingGrant(ctx context.Context, tx *sql.Tx, admission, grant string, hash, target []byte) error {
	if _, err := tx.ExecContext(ctx, "UPDATE document_admission SET target_json=? WHERE admission_id=?", string(target), admission); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE collaboration_ticket SET state='REDEEMED',redeemed_grant_ref=? WHERE token_hash=?", grant, hash)
	return err
}
````

## 第 23 步：LockedEditingAdmission

插入位置：第 22 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func LockedEditingAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.EditingAdmission</code> | 使用 <code>domain.EditingAdmission</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission).Scan(&amp;current.Actor, &amp;current …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code>。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=11337:11829 -->
````go
func LockedEditingAdmission(ctx context.Context, tx *sql.Tx, admission string) (domain.EditingAdmission, error) {
	var current domain.EditingAdmission
	err := tx.QueryRowContext(ctx, "SELECT actor_id,session_ref,target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.Actor, &current.SessionRef, &current.TargetJSON, &current.State, &current.Expires)
	return current, err
}

// editable.go：保留原实现，按业务合并文件。

````

## 第 24 步：VisitEditableUnits

插入位置：第 23 步代码之后，紧接追加；完成稿约第 203 行。

按原顺序逐条投影，业务达到页上限即结束；不为分层提前扫描整个可编辑目录。

本步声明：`func VisitEditableUnits(ctx context.Context, tx *sql.Tx, structure string, document uint64, last string, visit func(domain.EditableUnitFact) bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u  …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>structure</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u  …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u  …</code>（长表达式，按本步完整代码继续） |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u  …</code>（长表达式，按本步完整代码继续） |
| <code>visit</code> | <code>func(domain.EditableUnitFact) bool</code> | 在此声明中的实际使用：<code>visit(fact)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EDITABLE_UNITS_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, &quot;SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;fact.ID, &amp;fact.Title, &amp;fact.Branch, &amp;fact.Lifecycle, &amp;fact.Epoch, &amp;fact.Lineage) != nil</code>。条件成立时执行 <code>return errors.New(&quot;EDITABLE_UNITS_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;fact.ID, &amp;fact.Title, &amp;fact.Branch, &amp;fact.Lifecycle, &amp;fact.Epoch, &amp;fact.Lineage)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>!visit(fact)</code>。条件成立时执行 <code>break</code>。退出对应循环或switch，按代码中的标签/层级确定目标。
- 此处通过 <code>return rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=11829:12917 -->
````go
// 按原顺序逐条投影，业务达到页上限即结束；不为分层提前扫描整个可编辑目录。
func VisitEditableUnits(ctx context.Context, tx *sql.Tx, structure string, document uint64, last string, visit func(domain.EditableUnitFact) bool) error {
	rows, err := tx.QueryContext(ctx, "SELECT CAST(u.id AS CHAR),e.title,CAST(b.id AS CHAR),b.lifecycle,CAST(b.workspace_epoch AS CHAR),CAST(b.current_lineage_id AS CHAR) FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? LEFT JOIN branch b ON b.id=u.active_branch_id AND b.unit_id=u.id AND b.document_id=u.document_id WHERE u.document_id=? AND u.id>? AND u.retired_structure_id IS NULL ORDER BY u.id", structure, document, last)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var fact domain.EditableUnitFact
		if rows.Scan(&fact.ID, &fact.Title, &fact.Branch, &fact.Lifecycle, &fact.Epoch, &fact.Lineage) != nil {
			return errors.New("EDITABLE_UNITS_UNAVAILABLE")
		}
		if !visit(fact) {
			break
		}
	}
	return rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/editing_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
