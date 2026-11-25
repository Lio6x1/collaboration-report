# 分步编写 `services/document/internal/repository/document/authorization_queries.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/146-authorization-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。

授权变化必须与关闭计划在同一调用方事务提交，才能在进程中断后继续关闭旧输出。

写之前先按这条执行路线理解各段的连接：

1. Access从活动grant和当前structure取得权限输入
2. InsertAuthorization与InsertAuthorizationTask登记关闭
3. LockedTaskOwnership校验执行者
4. OriginalAdmission读取原actor/target/source绑定。

本文件需要守住的边界：

- LockedSourceGate在恢复时不能为缺失来源凭空创建OPEN门
- OUTPUT_STOPPED必须属于原操作与原代次。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/authorization_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/authorization_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/authorization_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/authorization_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:369 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
	"time"
)

// access.go：保留原实现，按业务合并文件。

````

## 第 2 步：Access

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type Access struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=369:403 -->
````go
type Access struct{ db *sql.DB }

````

## 第 3 步：NewAccess

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func NewAccess(db *sql.DB) *Access`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Access</code> | 实际返回表达式：<code>&amp;Access{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Access{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=403:465 -->
````go
func NewAccess(db *sql.DB) *Access { return &Access{db: db} }
````

## 第 4 步：Access.KnowledgeBase

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func (r *Access) KnowledgeBase(ctx context.Context, id, space uint64) (string, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?&quot;, id, space)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?&quot;, id, space)</code> |
| <code>space</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?&quot;, id, space)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?&quot;, id, space).Scan(&amp;state, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?&quot;, id, space)</code>。
- 此处通过 <code>return state, revision, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=465:779 -->
````go
func (r *Access) KnowledgeBase(ctx context.Context, id, space uint64) (string, uint64, error) {
	var state string
	var revision uint64
	err := r.db.QueryRowContext(ctx, "SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? AND space_id=?", id, space).Scan(&state, &revision)
	return state, revision, err
}
````

## 第 5 步：Access.KnowledgeBaseRole

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`func (r *Access) KnowledgeBaseRole(ctx context.Context, id, actor uint64) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>role</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor).Scan(&amp;role)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code>。
- 此处通过 <code>return role, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=779:1066 -->
````go
func (r *Access) KnowledgeBaseRole(ctx context.Context, id, actor uint64) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx, "SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE'", id, actor).Scan(&role)
	return role, err
}
````

## 第 6 步：Access.Grants

插入位置：第 5 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func (r *Access) Grants(ctx context.Context, id, actor uint64) ([]domain.GrantFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.GrantFact</code> | 使用 <code>domain.GrantFact</code> 类型；其定义处的职责：权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。<br>实际返回表达式：<code>nil</code>；<code>grants</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;GRANT_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state=&#x27;ACTIVE&#x27;&quot;, id, actor)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;grant.User, &amp;grant.Kind, &amp;grant.Scope, &amp;grant.Capability) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;GRANT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;grant.User, &amp;grant.Kind, &amp;grant.Scope, &amp;grant.Capability)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return grants, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1066:1664 -->
````go
func (r *Access) Grants(ctx context.Context, id, actor uint64) ([]domain.GrantFact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT user_id,scope_kind,scope_id,capability FROM content_grant WHERE document_id=? AND user_id=? AND state='ACTIVE'", id, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []domain.GrantFact
	for rows.Next() {
		var grant domain.GrantFact
		if rows.Scan(&grant.User, &grant.Kind, &grant.Scope, &grant.Capability) != nil {
			return nil, errors.New("GRANT_UNAVAILABLE")
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
````

## 第 7 步：Access.Units

插入位置：第 6 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (r *Access) Units(ctx context.Context, structure string, id uint64) ([]domain.UnitFact, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.doc …</code>（长表达式，按本步完整代码继续） |
| <code>structure</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.doc …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.doc …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.UnitFact</code> | 使用 <code>domain.UnitFact</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>nil</code>；<code>units</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;UNIT_UNAVAILABLE&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.doc …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;unit.ID, &amp;unit.Chapter) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;UNIT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;unit.ID, &amp;unit.Chapter)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return units, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1664:2330 -->
````go
func (r *Access) Units(ctx context.Context, structure string, id uint64) ([]domain.UnitFact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT u.id,u.chapter_id FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=? WHERE u.document_id=? AND u.retired_structure_id IS NULL ORDER BY u.id", structure, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var units []domain.UnitFact
	for rows.Next() {
		var unit domain.UnitFact
		if rows.Scan(&unit.ID, &unit.Chapter) != nil {
			return nil, errors.New("UNIT_UNAVAILABLE")
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}
````

## 第 8 步：LockedKnowledgeBase

插入位置：第 7 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func LockedKnowledgeBase(ctx context.Context, tx *sql.Tx, id string) (string, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>state</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;state, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return state, revision, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2330:2631 -->
````go
func LockedKnowledgeBase(ctx context.Context, tx *sql.Tx, id string) (string, uint64, error) {
	var state string
	var revision uint64
	err := tx.QueryRowContext(ctx, "SELECT lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE", id).Scan(&state, &revision)
	return state, revision, err
}

````

## 第 9 步：Access.Document

插入位置：第 8 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func (r *Access) Document(ctx context.Context, id uint64) (dto.Document, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>DocumentRow(ctx, r.db, id, false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentRow(ctx, r.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 由 <code>DocumentRow(ctx, r.db, id, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>DocumentRow(ctx, r.db, id, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return DocumentRow(ctx, r.db, id, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2631:2757 -->
````go
func (r *Access) Document(ctx context.Context, id uint64) (dto.Document, error) {
	return DocumentRow(ctx, r.db, id, false)
}
````

## 第 10 步：Access.Space

插入位置：第 9 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func (r *Access) Space(ctx context.Context, id uint64) (dto.Space, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>SpaceRow(ctx, r.db, id, false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>SpaceRow(ctx, r.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Space</code> | 由 <code>SpaceRow(ctx, r.db, id, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>SpaceRow(ctx, r.db, id, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return SpaceRow(ctx, r.db, id, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2757:2937 -->
````go
func (r *Access) Space(ctx context.Context, id uint64) (dto.Space, error) {
	return SpaceRow(ctx, r.db, id, false)
}

// admission.go：保留原实现，按业务合并文件。

````

## 第 11 步：AdmissionSources

插入位置：第 10 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`func AdmissionSources(ctx context.Context, q Queryer, admission string) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key&quot;, admission)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryContext(ctx, &quot;SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key&quot;, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.ErrSourcesRequired</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>q.QueryContext(ctx, &quot;SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key&quot;, admission)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;source.ScopeKey, &amp;source.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>len(result) == 0</code>。条件成立时执行 <code>return nil, domain.ErrSourcesRequired</code>。当前函数在这里结束。
- 此处通过 <code>return result, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=2937:3531 -->
````go
func AdmissionSources(ctx context.Context, q Queryer, admission string) ([]*pb.SourceEpoch, error) {
	rows, err := q.QueryContext(ctx, "SELECT scope_key,epoch FROM admission_source WHERE admission_id=? ORDER BY scope_key", admission)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*pb.SourceEpoch
	for rows.Next() {
		var source pb.SourceEpoch
		if err := rows.Scan(&source.ScopeKey, &source.Epoch); err != nil {
			return nil, err
		}
		result = append(result, &source)
	}
	if len(result) == 0 {
		return nil, domain.ErrSourcesRequired
	}
	return result, rows.Err()
}
````

## 第 12 步：Outputs.AdmissionSources

插入位置：第 11 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`func (r *Outputs) AdmissionSources(ctx context.Context, admission string) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>AdmissionSources(ctx, r.db, admission)</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>AdmissionSources(ctx, r.db, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 由 <code>AdmissionSources(ctx, r.db, admission)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>AdmissionSources(ctx, r.db, admission)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return AdmissionSources(ctx, r.db, admission)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3531:3684 -->
````go
func (r *Outputs) AdmissionSources(ctx context.Context, admission string) ([]*pb.SourceEpoch, error) {
	return AdmissionSources(ctx, r.db, admission)
}

````

## 第 13 步：AdmissionState

插入位置：第 12 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`type AdmissionState struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>TargetJSON</code> | <code>&#91;&#93;byte</code> | 原准入绑定的目标JSON；恢复时核对，不能换目标。 |
| <code>State</code> | <code>string</code> | document_admission的状态，决定此准入是否仍可使用。 |
| <code>Expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。 |

<!-- write-step: 13 bytes=3684:3776 -->
````go
type AdmissionState struct {
	TargetJSON []byte
	State      string
	Expires    time.Time
}

````

## 第 14 步：LockedAdmission

插入位置：第 13 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`func LockedAdmission(ctx context.Context, tx *sql.Tx, admission string) (AdmissionState, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AdmissionState</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission).Scan(&amp;current.TargetJSON, &amp;current.State, &amp;current.Expires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE&quot;, admission)</code>。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=3776:4123 -->
````go
func LockedAdmission(ctx context.Context, tx *sql.Tx, admission string) (AdmissionState, error) {
	var current AdmissionState
	err := tx.QueryRowContext(ctx, "SELECT target_json,state,expires_at FROM document_admission WHERE admission_id=? FOR UPDATE", admission).Scan(&current.TargetJSON, &current.State, &current.Expires)
	return current, err
}
````

## 第 15 步：MarkWebSocketDelivery

插入位置：第 14 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func MarkWebSocketDelivery(ctx context.Context, tx *sql.Tx, lease string, expires time.Time) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET target_kind=&#x27;WSS&#x27;,expires_at=? WHERE delivery_id=?&quot;, expires, lease)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>lease</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET target_kind=&#x27;WSS&#x27;,expires_at=? WHERE delivery_id=?&quot;, expires, lease)</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET target_kind=&#x27;WSS&#x27;,expires_at=? WHERE delivery_id=?&quot;, expires, lease)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET target_kind=&#x27;WSS&#x27;,expires_at=? WHERE delivery_id=?&quot;, expires, lease)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=4123:4434 -->
````go
func MarkWebSocketDelivery(ctx context.Context, tx *sql.Tx, lease string, expires time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE delivery_session SET target_kind='WSS',expires_at=? WHERE delivery_id=?", expires, lease)
	return err
}

// authorization.go：保留原实现，按业务合并文件。

````

## 第 16 步：Authorizations

插入位置：第 15 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`type Authorizations struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 16 bytes=4434:4476 -->
````go
type Authorizations struct{ db *sql.DB }

````

## 第 17 步：NewAuthorizations

插入位置：第 16 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func NewAuthorizations(db *sql.DB) *Authorizations`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Authorizations</code> | 实际返回表达式：<code>&amp;Authorizations{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Authorizations{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=4476:4563 -->
````go
func NewAuthorizations(db *sql.DB) *Authorizations { return &Authorizations{db: db} }

````

## 第 18 步：InsertAuthorization

插入位置：第 17 步代码之后，紧接追加；完成稿约第 123 行。

所有写入接受调用方事务；关闭计划与业务变更不可拆成两次提交。

本步声明：`func InsertAuthorization(ctx context.Context, tx *sql.Tx, operation string, actor uint64, scope string, epoch uint64, binding []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续） |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续） |
| <code>binding</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;\&quot;edge\&quot;,\&quot;colla …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=4563:5069 -->
````go
// 所有写入接受调用方事务；关闭计划与业务变更不可拆成两次提交。
func InsertAuthorization(ctx context.Context, tx *sql.Tx, operation string, actor uint64, scope string, epoch uint64, binding []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO authorization_change(revocation_id,actor_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,?,'FENCING','[\"edge\",\"collaboration\"]')", operation, actor, scope, epoch, string(binding))
	return err
}
````

## 第 19 步：InsertAuthorizationTask

插入位置：第 18 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func InsertAuthorizationTask(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;AUTHORIZATION_CHANGE&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, operation, string(payload))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;AUTHORIZATION_CHANGE&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, operation, string(payload))</code> |
| <code>payload</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(payload)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES(&#x27;AUTHORIZATION_CHANGE&#x27;,?,?,?,&#x27;QUEUED&#x27;,UTC_TIMESTAMP(6))&quot;, operation, operation, string(payload))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=5069:5403 -->
````go
func InsertAuthorizationTask(ctx context.Context, tx *sql.Tx, operation string, payload []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at) VALUES('AUTHORIZATION_CHANGE',?,?,?,'QUEUED',UTC_TIMESTAMP(6))", operation, operation, string(payload))
	return err
}
````

## 第 20 步：AuthorizationProjection

插入位置：第 19 步代码之后，紧接追加；完成稿约第 132 行。

本步声明：`func AuthorizationProjection(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?&quot;, operation, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?&quot;, operation, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?&quot;, operation, actor)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AuthorizationStatus</code> | 使用 <code>domain.AuthorizationStatus</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?&quot;, operation, actor).Scan(&amp;result.State, &amp;result.Scope, &amp;result.Epoch)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?&quot;, operation, actor)</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=5403:5805 -->
````go
func AuthorizationProjection(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error) {
	var result domain.AuthorizationStatus
	err := tx.QueryRowContext(ctx, "SELECT state,scope_key,CAST(epoch AS CHAR) FROM authorization_change WHERE revocation_id=? AND actor_id=?", operation, actor).Scan(&result.State, &result.Scope, &result.Epoch)
	return result, err
}
````

## 第 21 步：AuthorizationProgress

插入位置：第 20 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func AuthorizationProgress(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.ope …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.ope …</code>（长表达式，按本步完整代码继续） |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.ope …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AuthorizationStatus</code> | 使用 <code>domain.AuthorizationStatus</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.ope …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.ope …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=5805:6305 -->
````go
func AuthorizationProgress(ctx context.Context, tx *sql.Tx, operation string, actor uint64) (domain.AuthorizationStatus, error) {
	var result domain.AuthorizationStatus
	err := tx.QueryRowContext(ctx, "SELECT c.state,c.scope_key,CAST(c.epoch AS CHAR),r.state FROM operation_receipt r JOIN authorization_change c ON c.revocation_id=r.operation_id WHERE r.operation_id=? AND r.actor_id=?", operation, actor).Scan(&result.State, &result.Scope, &result.Epoch, &result.ReceiptState)
	return result, err
}
````

## 第 22 步：ProjectionAcknowledgment

插入位置：第 21 步代码之后，紧接追加；完成稿约第 142 行。

本步声明：`func ProjectionAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant string) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant)</code> |
| <code>participant</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>count</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant)</code>。
- 此处通过 <code>return count, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=6305:6628 -->
````go
func ProjectionAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND phase='OUTPUT_STOPPED'", operation, participant).Scan(&count)
	return count, err
}
````

## 第 23 步：ProgressAcknowledgment

插入位置：第 22 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`func ProgressAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant, scope, epoch string) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |
| <code>participant</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |
| <code>epoch</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>count</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch).Scan(&amp;count)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase=&#x27;OUTPUT_STOPPED&#x27;&quot;, operation, participant, scope, epoch)</code>。
- 此处通过 <code>return count, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=6628:7005 -->
````go
func ProgressAcknowledgment(ctx context.Context, tx *sql.Tx, operation, participant, scope, epoch string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", operation, participant, scope, epoch).Scan(&count)
	return count, err
}
````

## 第 24 步：ClaimAuthorization

插入位置：第 23 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func ClaimAuthorization(ctx context.Context, tx *sql.Tx) (domain.AuthorizationTask, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>lockDueTask(ctx, tx, authorizationTaskDue)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>lockDueTask(ctx, tx, authorizationTaskDue)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.AuthorizationTask</code> | 使用 <code>domain.AuthorizationTask</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>domain.AuthorizationTask{ID: task.ID, Operation: task.Operation, Payload: task.Payload}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.AuthorizationTask{ID: task.ID, Operation: task.Operation, Payload: task.Payload}, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=7005:7381 -->
````go
func ClaimAuthorization(ctx context.Context, tx *sql.Tx) (domain.AuthorizationTask, error) {
	// UNKNOWN仍按原撤权协议续接；原5秒执行租约由RunAuthorizationTask登记，不套用控制任务代次。
	task, err := lockDueTask(ctx, tx, authorizationTaskDue)
	return domain.AuthorizationTask{ID: task.ID, Operation: task.Operation, Payload: task.Payload}, err
}
````

## 第 25 步：RunAuthorizationTask

插入位置：第 24 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`func RunAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RUNNING&#x27;,instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?&quot;, instance, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=7381:7680 -->
````go
func RunAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='RUNNING',instance_id=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 5 SECOND),attempts=attempts+1 WHERE id=?", instance, id)
	return err
}
````

## 第 26 步：LockedTaskOwnership

插入位置：第 25 步代码之后，紧接追加；完成稿约第 161 行。

本步声明：`func LockedTaskOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.TaskOwnership</code> | 使用 <code>domain.TaskOwnership</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>实际返回表达式：<code>owner</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id).Scan(&amp;owner.Instance, &amp;owner.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return owner, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=7680:7974 -->
````go
func LockedTaskOwnership(ctx context.Context, tx *sql.Tx, id uint64) (domain.TaskOwnership, error) {
	var owner domain.TaskOwnership
	err := tx.QueryRowContext(ctx, "SELECT instance_id,state FROM durable_task WHERE id=? FOR UPDATE", id).Scan(&owner.Instance, &owner.State)
	return owner, err
}
````

## 第 27 步：TrashClosingDocument

插入位置：第 26 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func TrashClosingDocument(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=7974:8262 -->
````go
func TrashClosingDocument(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='TRASHED',deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE id=? AND lifecycle='CLOSING'", id)
	return err
}
````

## 第 28 步：TrashClosingKnowledgeBaseDocuments

插入位置：第 27 步代码之后，紧接追加；完成稿约第 170 行。

本步声明：`func TrashClosingKnowledgeBaseDocuments(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE knowledge_base_id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE knowledge_base_id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET lifecycle=&#x27;TRASHED&#x27;,deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE knowledge_base_id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=8262:8579 -->
````go
func TrashClosingKnowledgeBaseDocuments(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='TRASHED',deleted_at=UTC_TIMESTAMP(6),purge_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 DAY) WHERE knowledge_base_id=? AND lifecycle='CLOSING'", id)
	return err
}
````

## 第 29 步：CloseKnowledgeBase

插入位置：第 28 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func CloseKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSED&#x27; WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSED&#x27; WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE knowledge_base SET lifecycle=&#x27;CLOSED&#x27; WHERE id=? AND lifecycle=&#x27;CLOSING&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=8579:8787 -->
````go
func CloseKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSED' WHERE id=? AND lifecycle='CLOSING'", id)
	return err
}
````

## 第 30 步：CompleteAuthorizationReceipt

插入位置：第 29 步代码之后，紧接追加；完成稿约第 178 行。

本步声明：`func CompleteAuthorizationReceipt(ctx context.Context, tx *sql.Tx, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,terminal_at=UTC_TIMESTAMP(6),result_json=JSON_SET(result_json,&#x27;$.state&#x27;,&#x27;SUCCEEDED&#x27;) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,terminal_at=UTC_TIMESTAMP(6),result_json=JSON_SET(result_json,&#x27;$.state&#x27;,&#x27;SUCCEEDED&#x27;) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE operation_receipt SET state=&#x27;SUCCEEDED&#x27;,terminal_at=UTC_TIMESTAMP(6),result_json=JSON_SET(result_json,&#x27;$.state&#x27;,&#x27;SUCCEEDED&#x27;) WHERE operation_id=? AND state=&#x27;PREPARING&#x27;&quot;, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=8787:9114 -->
````go
func CompleteAuthorizationReceipt(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE operation_receipt SET state='SUCCEEDED',terminal_at=UTC_TIMESTAMP(6),result_json=JSON_SET(result_json,'$.state','SUCCEEDED') WHERE operation_id=? AND state='PREPARING'", operation)
	return err
}
````

## 第 31 步：CompleteAuthorizationTask

插入位置：第 30 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func CompleteAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?&quot;, id, instance)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?&quot;, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?&quot;, id, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?&quot;, id, instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=9114:9380 -->
````go
func CompleteAuthorizationTask(ctx context.Context, tx *sql.Tx, id uint64, instance string) error {
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',lease_until=NULL,last_error=NULL WHERE id=? AND instance_id=?", id, instance)
	return err
}
````

## 第 32 步：Authorizations.Retry

插入位置：第 31 步代码之后，紧接追加；完成稿约第 186 行。

本步声明：`func (r *Authorizations) Retry(ctx context.Context, id uint64, instance string)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;FENCE_NOT_CONFIRMED&#x27; WHERE id=? AND  …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;FENCE_NOT_CONFIRMED&#x27; WHERE id=? AND  …</code>（长表达式，按本步完整代码继续） |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;FENCE_NOT_CONFIRMED&#x27; WHERE id=? AND  …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;RETRY&#x27;,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error=&#x27;FENCE_NOT_CONFIRMED&#x27; WHERE id=? AND  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 32 bytes=9380:9776 -->
````go
func (r *Authorizations) Retry(ctx context.Context, id uint64, instance string) {
	_, _ = r.db.ExecContext(ctx, "UPDATE durable_task SET state='RETRY',retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),lease_until=NULL,last_error='FENCE_NOT_CONFIRMED' WHERE id=? AND instance_id=? AND state='RUNNING'", id, instance)
}

// authorization_change.go：保留原实现，按业务合并文件。

````

## 第 33 步：EditGrantScope

插入位置：第 32 步代码之后，紧接追加；完成稿约第 192 行。

表名来自已验证的内部scope枚举，不接受外部SQL片段。

本步声明：`func EditGrantScope(ctx context.Context, tx *sql.Tx, kind string, scope, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind == &quot;CHAPTER&quot;</code> |
| <code>scope</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document).Scan(&amp;found)</code> |

**跟着本段代码走一遍：**

- 检查 <code>kind == &quot;CHAPTER&quot;</code>。
- 此处通过 <code>return tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document).Scan(&amp;found)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document).Scan(&amp;found)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM &quot;+table+&quot; WHERE id=? AND document_id=? AND retired_structure_id IS NULL&quot;, scope, document)</code>。

<!-- write-step: 33 bytes=9776:10199 -->
````go
// 表名来自已验证的内部scope枚举，不接受外部SQL片段。
func EditGrantScope(ctx context.Context, tx *sql.Tx, kind string, scope, document uint64) error {
	table := "collaboration_unit"
	if kind == "CHAPTER" {
		table = "chapter"
	}
	var found uint64
	return tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE id=? AND document_id=? AND retired_structure_id IS NULL", scope, document).Scan(&found)
}
````

## 第 34 步：SetContentGrant

插入位置：第 33 步代码之后，紧接追加；完成稿约第 201 行。

本步声明：`func SetContentGrant(ctx context.Context, tx *sql.Tx, document uint64, grant domain.GrantFact, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=re …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=re …</code>（长表达式，按本步完整代码继续） |
| <code>grant</code> | <code>domain.GrantFact</code> | 使用 <code>domain.GrantFact</code> 类型；其定义处的职责：权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。<br>在此声明中的实际使用：<code>grant.User</code>；<code>grant.Kind</code>；<code>grant.Scope</code>；<code>grant.Capability</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=re …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=re …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=10199:10604 -->
````go
func SetContentGrant(ctx context.Context, tx *sql.Tx, document uint64, grant domain.GrantFact, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_grant(document_id,user_id,scope_kind,scope_id,capability,state) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", document, grant.User, grant.Kind, grant.Scope, grant.Capability, state)
	return err
}
````

## 第 35 步：AdvanceDocumentACL

插入位置：第 34 步代码之后，紧接追加；完成稿约第 205 行。

本步声明：`func AdvanceDocumentACL(ctx context.Context, tx *sql.Tx, document uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET acl_revision=acl_revision+1 WHERE id=?&quot;, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document SET acl_revision=acl_revision+1 WHERE id=?&quot;, document)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document SET acl_revision=acl_revision+1 WHERE id=?&quot;, document)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=10604:10866 -->
````go
func AdvanceDocumentACL(ctx context.Context, tx *sql.Tx, document uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET acl_revision=acl_revision+1 WHERE id=?", document)
	return err
}

// departure.go：保留原实现，按业务合并文件。

````

## 第 36 步：Departures

插入位置：第 35 步代码之后，紧接追加；完成稿约第 212 行。

本步声明：`type Departures struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 36 bytes=10866:10904 -->
````go
type Departures struct{ db *sql.DB }

````

## 第 37 步：NewDepartures

插入位置：第 36 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`func NewDepartures(db *sql.DB) *Departures`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Departures</code> | 实际返回表达式：<code>&amp;Departures{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Departures{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 37 bytes=10904:10979 -->
````go
func NewDepartures(db *sql.DB) *Departures { return &Departures{db: db} }

````

## 第 38 步：departureAdminSQL

插入位置：第 37 步代码之后，紧接追加；完成稿约第 216 行。

本步声明：`const departureAdminSQL = "SELECT EXISTS(SELECT 1 FROM knowledge_base_member m JOIN knowledge_base k ON k.id=m.knowledge_base_id JOIN space s ON s.id=k.space_id WHERE s.kind='ORGANIZATION' AND s.organization_id=? AND k.lifecycle IN ('ACTIVE','CLOSING') AND m.user_id=? AND m.role='ADMIN' AND m.state IN ('ACTIVE','REVOKING'))"`。

<!-- write-step: 38 bytes=10979:11307 -->
````go
const departureAdminSQL = "SELECT EXISTS(SELECT 1 FROM knowledge_base_member m JOIN knowledge_base k ON k.id=m.knowledge_base_id JOIN space s ON s.id=k.space_id WHERE s.kind='ORGANIZATION' AND s.organization_id=? AND k.lifecycle IN ('ACTIVE','CLOSING') AND m.user_id=? AND m.role='ADMIN' AND m.state IN ('ACTIVE','REVOKING'))"

````

## 第 39 步：EnsureDepartureGate

插入位置：第 38 步代码之后，紧接追加；完成稿约第 218 行。

本步声明：`func EnsureDepartureGate(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, binding.Scope(), binding.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>binding.Scope</code>；<code>binding.Epoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, binding.Scope(), binding.Epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 39 bytes=11307:11608 -->
````go
func EnsureDepartureGate(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", binding.Scope(), binding.Epoch)
	return err
}
````

## 第 40 步：LockedDepartureGate

插入位置：第 39 步代码之后，紧接追加；完成稿约第 222 行。

本步声明：`func LockedDepartureGate(ctx context.Context, tx *sql.Tx, scope string) (domain.DepartureGate, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.DepartureGate</code> | 使用 <code>domain.DepartureGate</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>gate</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope).Scan(&amp;gate.Epoch, &amp;gate.State, &amp;operation)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, scope)</code>。
- 此处通过 <code>return gate, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 40 bytes=11608:12001 -->
````go
func LockedDepartureGate(ctx context.Context, tx *sql.Tx, scope string) (domain.DepartureGate, error) {
	var gate domain.DepartureGate
	var operation sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT max_epoch,state,last_revocation_id FROM access_gate WHERE scope_key=? FOR UPDATE", scope).Scan(&gate.Epoch, &gate.State, &operation)
	gate.Operation = operation.String
	return gate, err
}
````

## 第 41 步：HasKnowledgeBaseAdministration

插入位置：第 40 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func HasKnowledgeBaseAdministration(ctx context.Context, tx *sql.Tx, organization, user uint64) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, departureAdminSQL, organization, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, departureAdminSQL, organization, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, departureAdminSQL, organization, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>admin</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, departureAdminSQL, organization, user).Scan(&amp;admin)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, departureAdminSQL, organization, user)</code>。
- 此处通过 <code>return admin, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=12001:12234 -->
````go
func HasKnowledgeBaseAdministration(ctx context.Context, tx *sql.Tx, organization, user uint64) (bool, error) {
	var admin bool
	err := tx.QueryRowContext(ctx, departureAdminSQL, organization, user).Scan(&admin)
	return admin, err
}
````

## 第 42 步：BlockMemberDeparture

插入位置：第 41 步代码之后，紧接追加；完成稿约第 234 行。

本步声明：`func BlockMemberDeparture(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 WHERE scope_key=?&quot;, binding.Epoch, binding.ID, binding.Scope())</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>binding</code> | <code>domain.DepartureBinding</code> | 使用 <code>domain.DepartureBinding</code> 类型；其定义处的职责：DepartureBinding绑定实际组织、成员、操作编号与原身份代次。<br>在此声明中的实际使用：<code>binding.Epoch</code>；<code>binding.ID</code>；<code>binding.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;BLOCKED&#x27;,last_revocation_id=?,revision=revision+1 WHERE scope_key=?&quot;, binding.Epoch, binding.ID, binding.Scope())</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 42 bytes=12234:12536 -->
````go
func BlockMemberDeparture(ctx context.Context, tx *sql.Tx, binding domain.DepartureBinding) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='BLOCKED',last_revocation_id=?,revision=revision+1 WHERE scope_key=?", binding.Epoch, binding.ID, binding.Scope())
	return err
}
````

## 第 43 步：CloseOrganizationMemberFacts

插入位置：第 42 步代码之后，紧接追加；完成稿约第 238 行。

本步声明：`func CloseOrganizationMemberFacts(ctx context.Context, tx *sql.Tx, organization, user uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, organization, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>organization</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, organization, user)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, statement, organization, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, statement, organization, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 43 bytes=12536:13666 -->
````go
func CloseOrganizationMemberFacts(ctx context.Context, tx *sql.Tx, organization, user uint64) error {
	commands := []string{
		"UPDATE knowledge_base_member m JOIN knowledge_base k ON k.id=m.knowledge_base_id JOIN space s ON s.id=k.space_id SET m.state='CLOSED',m.revision=m.revision+1 WHERE s.organization_id=? AND m.user_id=? AND m.state<>'CLOSED'",
		"UPDATE knowledge_base_creation_grant g JOIN knowledge_base k ON k.id=g.knowledge_base_id JOIN space s ON s.id=k.space_id SET g.state='CLOSED',g.revision=g.revision+1 WHERE s.organization_id=? AND g.user_id=? AND g.state<>'CLOSED'",
		"UPDATE content_grant g JOIN document d ON d.id=g.document_id JOIN space s ON s.id=d.space_id SET g.state='CLOSED',g.revision=g.revision+1 WHERE s.organization_id=? AND g.user_id=? AND g.state<>'CLOSED'",
		"UPDATE document d JOIN space s ON s.id=d.space_id SET d.responsible_user_id=NULL,d.control_revision=d.control_revision+1 WHERE s.organization_id=? AND d.responsible_user_id=?",
	}
	for _, statement := range commands {
		if _, err := tx.ExecContext(ctx, statement, organization, user); err != nil {
			return err
		}
	}
	return nil
}
````

## 第 44 步：Departures.Pending

插入位置：第 43 步代码之后，紧接追加；完成稿约第 252 行。

本步声明：`func (r *Departures) Pending(ctx context.Context) (domain.DepartureObservation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT g.scope_key,g.last_revocation_id,g.max_epoch FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id WHERE g.s …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.DepartureObservation</code> | 使用 <code>domain.DepartureObservation</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>实际返回表达式：<code>observed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT g.scope_key,g.last_revocation_id,g.max_epoch FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id WHERE g.s …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT g.scope_key,g.last_revocation_id,g.max_epoch FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id WHERE g.s …</code>（长表达式，按本步完整代码继续）。
- 此处通过 <code>return observed, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 44 bytes=13666:14216 -->
````go
func (r *Departures) Pending(ctx context.Context) (domain.DepartureObservation, error) {
	var observed domain.DepartureObservation
	err := r.db.QueryRowContext(ctx, "SELECT g.scope_key,g.last_revocation_id,g.max_epoch FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id WHERE g.scope_key LIKE 'org-member:%' AND g.state='BLOCKED' AND g.last_revocation_id IS NOT NULL AND c.revocation_id IS NULL ORDER BY g.scope_key LIMIT 1").Scan(&observed.Scope, &observed.Operation, &observed.Epoch)
	return observed, err
}
````

## 第 45 步：Departures.Reopen

插入位置：第 44 步代码之后，紧接追加；完成稿约第 257 行。

本步声明：`func (r *Departures) Reopen(ctx context.Context, observed domain.DepartureObservation)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;UPDATE access_gate SET state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=? AND max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id= …</code>（长表达式，按本步完整代码继续） |
| <code>observed</code> | <code>domain.DepartureObservation</code> | 使用 <code>domain.DepartureObservation</code> 类型；其定义处的职责：定义离组通知、组织关闭、个人转移和孤立公开留存治理的固定计划。<br>在此声明中的实际使用：<code>observed.Scope</code>；<code>observed.Epoch</code>；<code>observed.Operation</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, &quot;UPDATE access_gate SET state=&#x27;OPEN&#x27;,last_revocation_id=NULL,revision=revision+1 WHERE scope_key=? AND max_epoch=? AND state=&#x27;BLOCKED&#x27; AND last_revocation_id= …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 45 bytes=14216:14623 -->
````go
func (r *Departures) Reopen(ctx context.Context, observed domain.DepartureObservation) {
	_, _ = r.db.ExecContext(ctx, "UPDATE access_gate SET state='OPEN',last_revocation_id=NULL,revision=revision+1 WHERE scope_key=? AND max_epoch=? AND state='BLOCKED' AND last_revocation_id=?", observed.Scope, observed.Epoch, observed.Operation)
}

// original_admission.go：保留原实现，按业务合并文件。

````

## 第 46 步：OriginalAdmissionBinding

插入位置：第 45 步代码之后，紧接追加；完成稿约第 263 行。

本步声明：`type OriginalAdmissionBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 原准入的账号。 |
| <code>Document</code> | <code>uint64</code> | 原准入的文档。 |
| <code>Unit</code> | <code>uint64</code> | 原准入的协作单元范围。 |
| <code>Identity</code> | <code>string</code> | 账号服务发出的身份准入引用。 |
| <code>Operation</code> | <code>string</code> | 原业务操作号。 |
| <code>Purpose</code> | <code>string</code> | 原准入用途，不能跨用途复用。 |
| <code>Target</code> | <code>&#91;&#93;byte</code> | 原准入的目标JSON字节。 |
| <code>Session</code> | <code>&#91;&#93;byte</code> | 原会话引用字节，绑定同一次会话来源。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |

<!-- write-step: 46 bytes=14623:14812 -->
````go
type OriginalAdmissionBinding struct {
	Actor, Document, Unit        uint64
	Identity, Operation, Purpose string
	Target, Session              []byte
	SessionExpires               int64
}

````

## 第 47 步：OriginalAdmission

插入位置：第 46 步代码之后，紧接追加；完成稿约第 270 行。

原准入证明所有持久操作的归属，不能另取新准入替换固定目标。所有字段在同一调用方事务读取。

本步声明：`func OriginalAdmission(ctx context.Context, q Queryer, admission string, locked bool) (OriginalAdmissionBinding, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>q.QueryRowContext(ctx, statement, admission)</code> |
| <code>q</code> | <code>Queryer</code> | SQL 查询能力接口；调用者传入连接池、连接或事务中的哪一种，决定本次查询所属的提交与锁边界。<br>在此声明中的实际使用：<code>q.QueryRowContext</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>q.QueryRowContext(ctx, statement, admission)</code> |
| <code>locked</code> | <code>bool</code> | true时追加FOR UPDATE锁定读取，供随后同一事务内的核验/写入使用；false仅查询。调用者须传入原事务，不能读完在另一连接更新。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>OriginalAdmissionBinding</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>locked</code>。
- 执行 <code>q.QueryRowContext(ctx, statement, admission).Scan(&amp;value.Actor, &amp;value.Document, &amp;value.Unit, &amp;value.Identity, &amp;value.Operation, &amp;value.Purpose, &amp;value.Target, &amp;value.Session, &amp;value.SessionExpires)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>q.QueryRowContext(ctx, statement, admission)</code>。
- 此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 47 bytes=14812:15674 -->
````go
// 原准入证明所有持久操作的归属，不能另取新准入替换固定目标。所有字段在同一调用方事务读取。
func OriginalAdmission(ctx context.Context, q Queryer, admission string, locked bool) (OriginalAdmissionBinding, error) {
	var value OriginalAdmissionBinding
	statement := `SELECT actor_id,document_id,COALESCE(unit_id,0),identity_admission_ref,
 operation_id,purpose,target_json,session_ref,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED)
 FROM document_admission WHERE admission_id=?`
	if locked {
		statement += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, statement, admission).Scan(&value.Actor, &value.Document, &value.Unit, &value.Identity, &value.Operation, &value.Purpose, &value.Target, &value.Session, &value.SessionExpires)
	return value, err
}

// source_gate.go：保留原实现，按业务合并文件。

````

## 第 48 步：SourceGate

插入位置：第 47 步代码之后，紧接追加；完成稿约第 285 行。

本步声明：`type SourceGate struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Epoch</code> | <code>uint64</code> | 来源门保存的最高代次，用于阻止旧权限证明。 |
| <code>State</code> | <code>string</code> | 来源门开放或阻断等状态；代次一致也不能忽略门状态。 |

<!-- write-step: 48 bytes=15674:15730 -->
````go
type SourceGate struct {
	Epoch uint64
	State string
}

````

## 第 49 步：LockSourceGate

插入位置：第 48 步代码之后，紧接追加；完成稿约第 290 行。

本步声明：`func LockSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) (SourceGate, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key, epoch)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key, epoch)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key, epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>SourceGate</code> | 实际返回表达式：<code>gate</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;GATE_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,&#x27;OPEN&#x27;) ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)&quot;, key, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return gate, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key).Scan(&amp;gate.Epoch, &amp;gate.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return gate, errors.New(&quot;GATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return gate, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 49 bytes=15730:16389 -->
````go
func LockSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) (SourceGate, error) {
	var gate SourceGate
	// INSERT IGNORE会取得共享锁并造成升级死锁；无值变化的UPSERT直接取得排他锁。
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,?,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", key, epoch)
	if err != nil {
		return gate, err
	}
	err = tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&gate.Epoch, &gate.State)
	if err != nil {
		return gate, errors.New("GATE_UNAVAILABLE")
	}
	return gate, nil
}
````

## 第 50 步：PendingSourceTransition

插入位置：第 49 步代码之后，紧接追加；完成稿约第 303 行。

本步声明：`func PendingSourceTransition(ctx context.Context, tx *sql.Tx, key string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) LIMIT 1 FOR UPDATE&quot;, key)</code>；<code>PersonalTransferSourcePending(ctx, tx, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>PersonalTransferSourcePending(ctx, tx, key)</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) LIMIT 1 FOR UPDATE&quot;, key)</code>；<code>PersonalTransferSourcePending(ctx, tx, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>PersonalTransferSourcePending(ctx, tx, key)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>err == nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>PersonalTransferSourcePending(ctx, tx, key)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) LIMIT 1 FOR UPDATE&quot;, key).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN (&#x27;SUCCEEDED&#x27;,&#x27;REJECTED&#x27;) LIMIT 1 FOR UPDATE&quot;, key)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return PersonalTransferSourcePending(ctx, tx, key)</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 50 bytes=16389:16777 -->
````go
func PendingSourceTransition(ctx context.Context, tx *sql.Tx, key string) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM authorization_change WHERE scope_key=? AND state NOT IN ('SUCCEEDED','REJECTED') LIMIT 1 FOR UPDATE", key).Scan(&state)
	if err == sql.ErrNoRows {
		return PersonalTransferSourcePending(ctx, tx, key)
	}
	return err == nil, err
}
````

## 第 51 步：ReopenSourceGate

插入位置：第 50 步代码之后，紧接追加；完成稿约第 311 行。

本步声明：`func ReopenSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;, epoch, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;, epoch, key)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;, epoch, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE access_gate SET max_epoch=?,state=&#x27;OPEN&#x27;,revision=revision+1 WHERE scope_key=?&quot;, epoch, key)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 51 bytes=16777:17013 -->
````go
func ReopenSourceGate(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE access_gate SET max_epoch=?,state='OPEN',revision=revision+1 WHERE scope_key=?", epoch, key)
	return err
}

````

## 第 52 步：LockedSourceGate

插入位置：第 51 步代码之后，紧接追加；完成稿约第 316 行。

LockedSourceGate 只锁已有来源；缺失记录保留为未知，不能在恢复时补出开放来源。

本步声明：`func LockedSourceGate(ctx context.Context, tx *sql.Tx, key string) (SourceGate, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>SourceGate</code> | 实际返回表达式：<code>gate</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key).Scan(&amp;gate.Epoch, &amp;gate.State)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE&quot;, key)</code>。
- 此处通过 <code>return gate, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 52 bytes=17013:17394 -->
````go
// LockedSourceGate 只锁已有来源；缺失记录保留为未知，不能在恢复时补出开放来源。
func LockedSourceGate(ctx context.Context, tx *sql.Tx, key string) (SourceGate, error) {
	var gate SourceGate
	err := tx.QueryRowContext(ctx, "SELECT max_epoch,state FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&gate.Epoch, &gate.State)
	return gate, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/authorization_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
